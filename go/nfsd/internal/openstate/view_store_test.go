// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package openstate

import (
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The writer index forgets a writer only when its staging is gone: however
// many handles and writers a client has, one whose staging is still live or
// a failed tombstone stays, so COMMIT through its handle still reports it.
func TestWriterIndexFollowsStaging(t *testing.T) {
	var vs ViewStore
	failed := ViewKey{ClientID: 1, FileID: 7}
	vs.AddWriter(failed, 70)
	for i := range 2000 {
		key := ViewKey{ClientID: 2, FileID: vfs.InodeID(1000 + i%100)}
		vs.AddWriter(key, vfs.InodeID(10000+i))
	}
	if got := vs.WritersOf(failed); len(got) != 1 || got[0] != 70 {
		t.Fatalf("failed writer evicted by churn: %v", got)
	}
	if got := vs.WritersOf(ViewKey{ClientID: 2, FileID: 1000}); len(got) != 20 {
		t.Fatalf("handle 1000 has %d writers, want 20", len(got))
	}
	// Staging 70 failed, odd stagings are live, the rest are gone.
	kept := func(id vfs.InodeID) bool { return id == 70 || id%2 == 1 }
	vs.PruneWriters(kept)
	if got := vs.WritersOf(failed); len(got) != 1 {
		t.Fatalf("failed writer pruned: %v", got)
	}
	for _, id := range vs.WritersOf(ViewKey{ClientID: 2, FileID: 1001}) {
		if !kept(id) {
			t.Fatalf("gone staging %d kept", id)
		}
	}
	if got := vs.WritersOf(ViewKey{ClientID: 2, FileID: 1000}); len(got) != 0 {
		t.Fatalf("handle whose stagings are all gone kept %v", got)
	}
	vs.PruneWriters(func(vfs.InodeID) bool { return false })
	if len(vs.writers) != 0 {
		t.Fatalf("%d handles left after every staging went", len(vs.writers))
	}
}
