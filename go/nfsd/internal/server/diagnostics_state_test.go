// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/openstate"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Race detector test: incarnation snapshots read cached store locations
// under their own lock while MarkOpen renews them.
func TestDiagnosticsCachedClientDuringRenewal(t *testing.T) {
	f := newDiagnosticsFixture(t)
	done := make(chan error, 1)
	go func() {
		for range 500 {
			if err := f.srv.clients.MarkOpen(f.incarnID, nfs4.StateID(f.stateid)); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			return
		default:
			out, err := f.srv.Incarnations(inspect.IncarnationFilter{ID: f.incarnID})
			if err != nil || len(out.Records) != 1 {
				t.Fatalf("incarnation snapshot: %+v, %v", out, err)
			}
		}
	}
}

// Race detector test: incarnation snapshots read a refresher's published
// expiry under its mutex while the worker publishes.
func TestDiagnosticsLeaseDuringPublication(t *testing.T) {
	cs, id, r, _ := clientstore.RefresherForTest(t, vfs.NewLocalTernVFS(t.TempDir()))
	srv := &Server{clients: cs, sessions: newSessionStore(1), opens: openstate.NewStore()}
	done := make(chan error, 1)
	go func() {
		// The worker owns leaseLocks during every refresh. Force each
		// publication due so snapshots overlap real lastExpiry writes.
		release := cs.LeaseLocks.Lock(vfs.InodeID(id))
		defer release()
		for range 100 {
			r.Mu.Lock()
			r.Proven = 0
			r.Mu.Unlock()
			if !r.Refresh() {
				done <- fmt.Errorf("lease publication failed")
				return
			}
		}
		done <- nil
	}()
	for {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
			r.Mu.Lock()
			published := r.LastExpiry
			r.Mu.Unlock()
			if published == 0 {
				t.Fatal("no lease was published")
			}
			return
		default:
			if _, err := srv.Incarnations(inspect.IncarnationFilter{ID: id}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func diagnosticsStagingStore(t *testing.T) (*Server, *staging.LocalStore) {
	t.Helper()
	store, err := staging.NewLocalStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.CloseAll)
	srv, err := NewServer(vfs.NewLocalTernVFS(t.TempDir()), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, store
}

func diagnosticStateid(epoch uint32, counter uint64) nfs4.StateID {
	var id nfs4.StateID
	binary.BigEndian.PutUint32(id[:4], epoch)
	binary.BigEndian.PutUint64(id[4:], counter)
	return id
}

// A stateid lookup tries a busy staging file's lock rather than waiting:
// the answer is incomplete, and complete once the file is free.
func TestDiagnosticsStateidLookupDoesNotWaitForStaging(t *testing.T) {
	srv, store := diagnosticsStagingStore(t)
	id := diagnosticStateid(srv.opens.Epoch, 1)
	file, err := store.Create(vfs.InodeID(0x4000000000000001), staging.Meta{
		DirID: srv.fs.RootID(), FileName: "busy", ClientID: 7, NFSStateID: id,
	})
	if err != nil {
		t.Fatal(err)
	}
	sf := file.(*staging.LocalFile)
	sf.LockForTest()
	locked := true
	defer func() {
		if locked {
			sf.UnlockForTest()
		}
	}()
	for _, query := range []nfs4.StateID{id, diagnosticStateid(srv.opens.Epoch, 2)} {
		done := make(chan inspect.Resolution, 1)
		go func() {
			result, _ := srv.ResolveStateid([12]byte(query))
			done <- result
		}()
		select {
		case result := <-done:
			if result.Known || !result.Incomplete {
				t.Fatalf("busy staging must leave lookup incomplete: %+v", result)
			}
		case <-time.After(testutil.ChannelTimeout):
			// Unblock and join a regressed reader before cleanup releases
			// the store, so the failure does not introduce a test-only race.
			sf.UnlockForTest()
			locked = false
			select {
			case <-done:
			case <-time.After(testutil.ChannelTimeout):
				t.Fatal("stateid lookup did not finish after releasing the file")
			}
			t.Fatal("stateid lookup waited on the staging file")
		}
	}
	sf.UnlockForTest()
	locked = false
	result, _ := srv.ResolveStateid([12]byte(id))
	if !result.Known || result.Incomplete || result.State != "staging" {
		t.Fatalf("unlocked staging not resolved: %+v", result)
	}
	missing, _ := srv.ResolveStateid([12]byte(diagnosticStateid(srv.opens.Epoch, 2)))
	if missing.Known || missing.Incomplete {
		t.Fatalf("complete negative lookup: %+v", missing)
	}
}

// A stateid from an earlier process resolves through its recovered staging,
// and not once that staging is quarantined.
func TestDiagnosticsStateidLookupFindsRecoveredEpoch(t *testing.T) {
	srv, store := diagnosticsStagingStore(t)
	id := diagnosticStateid(srv.opens.Epoch+1, 1)
	const fileID = vfs.InodeID(0x4000000000000001)
	if _, err := store.Create(fileID, staging.Meta{
		DirID: srv.fs.RootID(), FileName: "recovered", ClientID: 7, NFSStateID: id,
	}); err != nil {
		t.Fatal(err)
	}
	// Reload the sidecar as a restarted daemon would. Its old stateid is
	// deliberately not in this server's process-local open-state map.
	store.CloseAll()
	recovered, err := staging.NewLocalStore(store.Dir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(recovered.CloseAll)
	srv.stagingStore = recovered
	result, err := srv.ResolveStateid([12]byte(id))
	if err != nil || !result.Known || result.Incomplete || result.State != "staging" ||
		result.Incarnation != incarnationText(7) || result.IssuedByThisProcess == nil ||
		*result.IssuedByThisProcess {
		t.Fatalf("recovered stateid: %+v, %v", result, err)
	}
	if err := recovered.Quarantine(fileID); err != nil {
		t.Fatal(err)
	}
	result, _ = srv.ResolveStateid([12]byte(id))
	if result.Known || result.Incomplete {
		t.Fatalf("quarantined staging must not be advertised as recoverable: %+v", result)
	}
}
