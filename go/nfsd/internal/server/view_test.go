// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// quarantineBetweenChecks quarantines the entry the moment anyone asks
// whether it failed, after reading the answer: the interleaving in which a
// prune that asks Failed and then Get sees neither.
type quarantineBetweenChecks struct {
	staging.Store
	id vfs.InodeID
}

func (s quarantineBetweenChecks) Failed(id vfs.InodeID) bool {
	failed := s.Store.Failed(id)
	if id == s.id {
		s.Store.Quarantine(id)
	}
	return failed
}

// A writer index entry survives a sweep while its staging is live or
// failed, even when the quarantine lands during the sweep. Losing it lets
// a later COMMIT through the handle report success for a failed writer.
func TestWriterPruneKeepsWriterQuarantinedDuringSweep(t *testing.T) {
	local, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	id := vfs.MakeInodeID(vfs.InodeTypeFile, 100)
	if _, err := local.Create(id, staging.Meta{DirID: vfs.MakeInodeID(vfs.InodeTypeDir, 1), FileName: "f"}); err != nil {
		t.Fatal(err)
	}
	s := &Server{stagingStore: quarantineBetweenChecks{Store: local, id: id}}
	key := openstate.ViewKey{ClientID: 7, FileID: vfs.MakeInodeID(vfs.InodeTypeFile, 9)}
	s.views.AddWriter(key, id)
	s.views.PruneWriters(s.writerKept)
	if got := s.views.WritersOf(key); len(got) != 1 || got[0] != id {
		t.Fatalf("writer index after the sweep = %v, want [%v]", got, id)
	}
	// Whichever way the sweep asked, the writer is failed by now; COMMIT
	// through the handle must find it in the index.
	if err := local.Quarantine(id); err != nil {
		t.Fatal(err)
	}
	s.stagingStore = local
	if !s.failedWriter(key.ClientID, key.FileID) {
		t.Fatal("COMMIT through the handle would not see the failed writer")
	}
}
