// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// nfsDirName is the hidden directory under the VFS root used for NFS server
// state. Currently contains "clients/" for persistent client ID assignment.
const nfsDirName = ".nfs"

// ClientStore manages persistent NFS client ID assignment backed by TernVFS.
//
// Client identities are stored as files in /.nfs/clients/. The filename is
// the escaped client id string. The file contents are the 8-byte verifier.
// The file's InodeID serves as the client's unique 64-bit clientid.
//
// Unconfirmed clients have a pending file named <escaped_id>.<verifier_hex>.
// On confirmation, the pending file is renamed over the confirmed file,
// atomically replacing the old verifier and producing a new InodeID (clientid).
type ClientStore struct {
	mu        sync.Mutex
	fs        TernVFS
	dirID     InodeID
	pending   map[uint64]pendingClient // clientID → pending info
	confirmed map[uint64]bool          // set of known confirmed clientIDs
}

type pendingClient struct {
	name    string // escaped client id
	verfHex string // hex-encoded verifier
}

// NewClientStore creates a ClientStore, ensuring the /.nfs/clients/ directory
// hierarchy exists in the VFS.
func NewClientStore(fs TernVFS) (*ClientStore, error) {
	rootID := fs.RootID()

	// Ensure /.nfs/ directory exists.
	nfsID, err := fs.Lookup(rootID, nfsDirName)
	if errors.Is(err, os.ErrNotExist) {
		nfsID, err = fs.Mkdir(rootID, nfsDirName)
	}
	if err != nil {
		return nil, fmt.Errorf("client store: create %s dir: %w", nfsDirName, err)
	}

	// Ensure /.nfs/clients/ directory exists.
	clientsID, err := fs.Lookup(nfsID, "clients")
	if errors.Is(err, os.ErrNotExist) {
		clientsID, err = fs.Mkdir(nfsID, "clients")
	}
	if err != nil {
		return nil, fmt.Errorf("client store: create clients dir: %w", err)
	}

	return &ClientStore{
		fs:        fs,
		dirID:     clientsID,
		pending:   make(map[uint64]pendingClient),
		confirmed: make(map[uint64]bool),
	}, nil
}

// SetClientID assigns a persistent client ID for the given verifier and
// identity string. If a confirmed file with a matching verifier already
// exists, its InodeID is returned unchanged. Otherwise a pending file is
// created; the caller must follow up with ConfirmClientID.
func (cs *ClientStore) SetClientID(verifier [8]byte, id []byte) (uint64, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	name := escapeClientID(id)
	verfHex := hex.EncodeToString(verifier[:])

	// Check if a confirmed file exists for this client id.
	confirmedID, err := cs.fs.Lookup(cs.dirID, name)
	if err == nil {
		// File exists — read stored verifier.
		var stored [8]byte
		n, _, readErr := cs.fs.Read(confirmedID, 0, stored[:])
		if readErr == nil && n == 8 && stored == verifier {
			// Same verifier — return existing clientid.
			cs.confirmed[uint64(confirmedID)] = true
			return uint64(confirmedID), nil
		}
		// Different verifier — client reboot. Fall through to create pending.
	}

	// Check if a pending file already exists (retransmission).
	pendingName := name + "." + verfHex
	pendingID, err := cs.fs.Lookup(cs.dirID, pendingName)
	if err == nil {
		cs.pending[uint64(pendingID)] = pendingClient{name: name, verfHex: verfHex}
		return uint64(pendingID), nil
	}

	// Create pending file with verifier as content.
	newID, err := cs.fs.CreateFile(cs.dirID, pendingName, bytes.NewReader(verifier[:]))
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			// Race with another server — lookup the existing file.
			newID, err = cs.fs.Lookup(cs.dirID, pendingName)
			if err != nil {
				return 0, err
			}
		} else {
			return 0, err
		}
	}

	cs.pending[uint64(newID)] = pendingClient{name: name, verfHex: verfHex}
	return uint64(newID), nil
}

// ConfirmClientID confirms a pending client by renaming its pending file
// over the confirmed file. Returns the old clientid that was replaced
// (0 if none). The caller should purge any state associated with the old
// clientid.
func (cs *ClientStore) ConfirmClientID(clientID uint64) (uint64, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	p, ok := cs.pending[clientID]
	if !ok {
		// Not pending — might be already confirmed (no-op confirm).
		if cs.confirmed[clientID] {
			return 0, nil
		}
		return 0, nfsError(NFS4ERR_STALE_CLIENTID)
	}
	delete(cs.pending, clientID)

	// Check if there's an existing confirmed file (will be replaced).
	var oldClientID uint64
	oldID, err := cs.fs.Lookup(cs.dirID, p.name)
	if err == nil {
		oldClientID = uint64(oldID)
		delete(cs.confirmed, oldClientID)
	}

	// Rename pending file → confirmed file.
	pendingName := p.name + "." + p.verfHex
	if err := cs.fs.Rename(cs.dirID, pendingName, cs.dirID, p.name); err != nil {
		return 0, err
	}

	cs.confirmed[clientID] = true
	return oldClientID, nil
}

// escapeClientID produces a filesystem-safe filename from an NFS client id
// string. Uses Go-style escaping: normal printable ASCII passes through,
// control characters and non-printable bytes become \xNN or \t, \n, etc.
// Forward slashes are additionally escaped since they are path separators.
func escapeClientID(id []byte) string {
	q := strconv.Quote(string(id))
	// Strip surrounding double quotes added by Quote.
	s := q[1 : len(q)-1]
	// Escape / which is printable but invalid in filenames.
	s = strings.ReplaceAll(s, "/", `\x2f`)
	// Guard against pathological "." and ".." names.
	if s == "." || s == ".." {
		s = `\x2e` + s[1:]
	}
	return s
}
