// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The alias log records publication aliases, so that a client holding a
// handle to a version it has just replaced keeps seeing its publication
// across an nfsd restart. It lives in its own subdirectory of the staging
// directory, which startup recovery and the inspector skip, as a file of
// JSON aliasRecord lines; the sweep rewrites it with the live ones.
const aliasLogName = "published.jsonl"

type aliasRecord struct {
	ClientID uint64    `json:"client"`
	FileID   uint64    `json:"file"`
	DataID   uint64    `json:"data"`
	Expires  time.Time `json:"expires"`
}

// aliasLog appends alias records as they are made and rewrites the file at
// each sweep. Writes are not synced: losing them on a crash leaves what
// there was before this file existed, old handles serving old content.
type aliasLog struct {
	path   string
	rename func(oldpath, newpath string) error
}

func newAliasLog(stagingDir string) *aliasLog {
	if stagingDir == "" {
		return nil
	}
	return &aliasLog{path: filepath.Join(stagingDir, staging.AliasesDirName, aliasLogName), rename: os.Rename}
}

func (l *aliasLog) append(rec aliasRecord) error {
	if l == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// load returns the records whose alias has not expired at now. A malformed
// line ends the read: what came before it is kept, the rest is lost.
func (l *aliasLog) load(now time.Time) ([]aliasRecord, error) {
	if l == nil {
		return nil, nil
	}
	f, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var live []aliasRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var rec aliasRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			return live, fmt.Errorf("%s: %w", l.path, err)
		}
		if rec.Expires.After(now) {
			live = append(live, rec)
		}
	}
	return live, sc.Err()
}

// rewrite replaces the file with the given records, through a rename.
func (l *aliasLog) rewrite(recs []aliasRecord) error {
	if l == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	tmp := l.path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, rec := range recs {
		line, err := json.Marshal(rec)
		if err != nil {
			f.Close()
			return err
		}
		w.Write(append(line, '\n'))
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return l.rename(tmp, l.path)
}

// loadAliases installs the persisted aliases that are still live.
func (s *Server) loadAliases() {
	recs, err := s.aliases.load(s.clients.Now())
	if err != nil {
		s.log.Warn("publication aliases: load", "err", err)
	}
	for _, rec := range recs {
		s.views.Set(openstate.ViewKey{ClientID: rec.ClientID, FileID: vfs.InodeID(rec.FileID)}, vfs.InodeID(rec.DataID), rec.Expires)
	}
	if len(recs) > 0 {
		s.log.Info("publication aliases recovered", "count", len(recs))
	}
}

// compactAliases rewrites the log with the aliases in memory. An alias
// whose handle its client still has open outlives its expiry in memory;
// the log gives it a lease from now, so that a process starting before
// the next sweep keeps it too. A publication waits for the rewrite, since
// its append to the file being replaced would be lost.
func (s *Server) compactAliases() {
	if s.aliases == nil {
		return
	}
	s.aliasMu.Lock()
	defer s.aliasMu.Unlock()
	heldUntil := s.clients.Now().Add(clientstore.LeaseTime)
	var recs []aliasRecord
	for _, a := range s.views.Entries() {
		expires := a.Expires
		if heldUntil.After(expires) && s.viewHeld(a.Key) {
			expires = heldUntil
		}
		recs = append(recs, aliasRecord{ClientID: a.Key.ClientID, FileID: uint64(a.Key.FileID), DataID: uint64(a.DataID), Expires: expires})
	}
	if err := s.aliases.rewrite(recs); err != nil {
		s.log.Warn("publication aliases: rewrite", "err", err)
	}
}
