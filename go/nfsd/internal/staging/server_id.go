// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ServerIDFileName is the file in the staging directory holding the server
// identity (README.md#server-identity).
const ServerIDFileName = "server-id"

// ClientsDirName is the staging subdirectory reserved for the server's
// client records. A staging directory holding any is a server's state.
const ClientsDirName = "clients"

// ServerID identifies the NFS server a staging directory belongs to. The
// server owner and scope a client sees at EXCHANGE_ID are derived from it.
type ServerID [16]byte

func (id ServerID) String() string { return hex.EncodeToString(id[:]) }

// ErrPartialCopy stops startup on a staging directory that holds a server's
// state but no server-id: neither the same server nor a new one.
var ErrPartialCopy = errors.New("staging directory holds server state but no " + ServerIDFileName)

// loadServerID reads dir's server identity, or writes a new one if dir holds
// no server state. The new identity is durable before it is returned.
func loadServerID(dir string) (ServerID, error) {
	path := filepath.Join(dir, ServerIDFileName)
	data, err := os.ReadFile(path)
	if err == nil {
		var id ServerID
		text := strings.TrimSuffix(string(data), "\n")
		if n, err := hex.Decode(id[:], []byte(text)); err != nil || n != len(id) || len(text) != 2*len(id) {
			return ServerID{}, fmt.Errorf("%s: not %d hex digits", path, 2*len(id))
		}
		return id, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return ServerID{}, err
	}
	state, err := serverState(dir)
	if err != nil {
		return ServerID{}, err
	}
	if state != "" {
		return ServerID{}, fmt.Errorf("%w: %s has %s; restore the whole directory or start on an empty one", ErrPartialCopy, dir, state)
	}
	var id ServerID
	if _, err := rand.Read(id[:]); err != nil {
		return ServerID{}, err
	}
	tmpPath := filepath.Join(dir, "."+ServerIDFileName+".tmp")
	tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return ServerID{}, err
	}
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(id.String() + "\n"); err != nil {
		tmp.Close()
		return ServerID{}, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return ServerID{}, err
	}
	if err := tmp.Close(); err != nil {
		return ServerID{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return ServerID{}, err
	}
	return id, syncDir(dir)
}

// serverState names the first kind of server state found in dir, or "" for
// none: staged files or sidecars, session files, client records.
func serverState(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if name := e.Name(); !e.IsDir() && (strings.HasSuffix(name, ".staging") || strings.HasSuffix(name, ".meta")) {
			return "staged file " + name, nil
		}
	}
	for _, sub := range []struct{ dir, what string }{
		{SessionsDirName, "session files"},
		{ClientsDirName, "client records"},
	} {
		entries, err := os.ReadDir(filepath.Join(dir, sub.dir))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if len(entries) > 0 {
			return sub.what + " in " + sub.dir + "/", nil
		}
	}
	return "", nil
}

// processServerID is the identity of a server without a staging directory:
// one per process, since it keeps no state for a later one.
var processServerID = sync.OnceValue(func() ServerID {
	var id ServerID
	if _, err := rand.Read(id[:]); err != nil {
		panic("generate nfsd server id: " + err.Error())
	}
	return id
})
