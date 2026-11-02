// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Sidecars from the NFSv4.0 server (NFS5 on main, NFS6 and NFS7 on its
// branches) and from the NFSv4.1 server before publication candidates
// (NFS8) stop startup instead of being quarantined, so an upgrade with
// undrained staging is noticed.
func TestOlderStagingFormatFailsStartup(t *testing.T) {
	for _, format := range []string{"NFS5", "NFS6", "NFS7", "NFS8"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			store, _, sf := CreateOverlayStage(t, dir, vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
			if err := sf.Sync(); err != nil {
				t.Fatal(err)
			}
			metaPath := sf.(*LocalFile).MetaPath()
			data, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			CloseFiles(t, store)
			copy(data[30+int(binary.BigEndian.Uint16(data[28:30]))+8:], format)
			if err := os.WriteFile(metaPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			var formatErr FormatError
			if _, err := NewLocalStore(dir, nil); !errors.As(err, &formatErr) {
				t.Fatalf("startup with %s staging = %v", format, err)
			}
			if _, err := os.Stat(metaPath); err != nil {
				t.Fatalf("older sidecar was moved: %v", err)
			}
		})
	}
}

func TestInvalidStagingIsQuarantined(t *testing.T) {
	for _, format := range []string{"missing", "truncated", "XFS8"} {
		t.Run(format, func(t *testing.T) {
			dir := t.TempDir()
			store, id, sf := CreateOverlayStage(t, dir, vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
			if err := sf.Write(0, []byte("keep")); err != nil {
				t.Fatal(err)
			}
			if err := sf.Sync(); err != nil {
				t.Fatal(err)
			}
			metaPath := sf.(*LocalFile).MetaPath()
			data, err := os.ReadFile(metaPath)
			if err != nil {
				t.Fatal(err)
			}
			CloseFiles(t, store)
			switch format {
			case "missing":
				err = os.Remove(metaPath)
			case "truncated":
				err = os.WriteFile(metaPath, data[:8], 0600)
			default:
				off := 30 + int(binary.BigEndian.Uint16(data[28:30])) + 8
				copy(data[off:], format)
				err = os.WriteFile(metaPath, data, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			recovered, err := NewLocalStore(dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			if recovered.Get(id) != nil || len(recovered.Entries()) != 0 {
				t.Fatal("invalid staging was registered")
			}
			files, err := filepath.Glob(filepath.Join(dir, "quarantine", "*", "*.staging"))
			if err != nil || len(files) != 1 {
				t.Fatalf("quarantine files = %v, err = %v", files, err)
			}
			got, err := os.ReadFile(files[0])
			if err != nil || string(got) != "keep" {
				t.Fatalf("quarantined data = %q, err = %v", got, err)
			}
		})
	}
}

// A namespace or bind save records the current candidate beside the older
// checkpointed data version. After a restart the data version starts above
// the candidate's, so later writes cannot make that candidate look current.
func TestLoadedDataVersionStartsAboveCandidate(t *testing.T) {
	dir := t.TempDir()
	store, id, sf := CreateOverlayStage(t, dir, vfs.MakeInodeID(vfs.InodeTypeFile, 1), []byte("base"))
	for range 3 {
		if err := sf.Write(0, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	c := Candidate{ID: id, Version: sf.DataVersion()}
	if err := sf.SetCandidate(c, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Rebind(id, 7, [12]byte{1}); err != nil {
		t.Fatal(err)
	}
	CloseFiles(t, store)
	loaded, err := NewLocalStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseFiles(t, loaded)
	got := loaded.Get(id)
	if got.Candidate() != c || got.DataVersion() <= c.Version {
		t.Fatalf("loaded candidate %+v, data version %d; want %+v below the data version", got.Candidate(), got.DataVersion(), c)
	}
}

// Retained answers for live and failed entries alike, in one check, and no
// for a removed one. Writer-index pruning depends on that: Failed then Get
// both say no when a quarantine lands between them.
func TestRetainedCoversLiveAndFailedEntries(t *testing.T) {
	store, err := NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	id := vfs.MakeInodeID(vfs.InodeTypeFile, 100)
	if store.Retained(id) {
		t.Fatal("retained before create")
	}
	if _, err := store.Create(id, Meta{DirID: vfs.MakeInodeID(vfs.InodeTypeDir, 1), FileName: "f"}); err != nil {
		t.Fatal(err)
	}
	if !store.Retained(id) {
		t.Fatal("live entry not retained")
	}
	if err := store.Quarantine(id); err != nil {
		t.Fatal(err)
	}
	if !store.Retained(id) || store.Get(id) != nil || !store.Failed(id) {
		t.Fatal("quarantined entry must be retained, failed and not live")
	}
	other := vfs.MakeInodeID(vfs.InodeTypeFile, 101)
	if _, err := store.Create(other, Meta{DirID: vfs.MakeInodeID(vfs.InodeTypeDir, 1), FileName: "g"}); err != nil {
		t.Fatal(err)
	}
	store.Remove(other)
	if store.Retained(other) {
		t.Fatal("removed entry retained")
	}
}
