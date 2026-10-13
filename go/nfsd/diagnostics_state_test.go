// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
)

func TestDiagnosticsCachedClientDuringRenewal(t *testing.T) {
	f := newDiagnosticsFixture(t)
	done := make(chan error, 1)
	go func() {
		for range 500 {
			if err := f.srv.clients.MarkOpen(f.incarnID, StateID(f.stateid)); err != nil {
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

func TestDiagnosticsLeaseDuringPublication(t *testing.T) {
	cs, id, r, _ := refresherForTest(t, NewLocalTernVFS(t.TempDir()))
	srv := &Server{clients: cs, sessions: newSessionStore(1), opens: newOpenStateStore()}
	done := make(chan error, 1)
	go func() {
		// The worker owns leaseLocks during every refresh. Force each
		// publication due so snapshots overlap real lastExpiry writes.
		release := cs.leaseLocks.lock(InodeID(id))
		defer release()
		for range 100 {
			r.mu.Lock()
			r.proven = 0
			r.mu.Unlock()
			if !r.refresh() {
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
			r.mu.Lock()
			published := r.lastExpiry
			r.mu.Unlock()
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

func diagnosticsStagingStore(t *testing.T) (*Server, *LocalStagingStore) {
	t.Helper()
	store, err := NewLocalStagingStore(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.closeAll)
	srv, err := NewServer(NewLocalTernVFS(t.TempDir()), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	return srv, store
}

func diagnosticStateid(epoch uint32, counter uint64) StateID {
	var id StateID
	binary.BigEndian.PutUint32(id[:4], epoch)
	binary.BigEndian.PutUint64(id[4:], counter)
	return id
}

func TestDiagnosticsStateidLookupDoesNotWaitForStaging(t *testing.T) {
	srv, store := diagnosticsStagingStore(t)
	id := diagnosticStateid(srv.opens.epoch, 1)
	file, err := store.Create(InodeID(0x4000000000000001), StagingMeta{
		DirID: srv.fs.RootID(), FileName: "busy", ClientID: 7, NFSStateID: id,
	})
	if err != nil {
		t.Fatal(err)
	}
	sf := file.(*localStagingFile)
	sf.mu.Lock()
	locked := true
	defer func() {
		if locked {
			sf.mu.Unlock()
		}
	}()
	for _, query := range []StateID{id, diagnosticStateid(srv.opens.epoch, 2)} {
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
		case <-time.After(testChannelTimeout):
			// Unblock and join a regressed reader before cleanup releases
			// the store, so the failure does not introduce a test-only race.
			sf.mu.Unlock()
			locked = false
			select {
			case <-done:
			case <-time.After(testChannelTimeout):
				t.Fatal("stateid lookup did not finish after releasing the file")
			}
			t.Fatal("stateid lookup waited on the staging file")
		}
	}
	sf.mu.Unlock()
	locked = false
	result, _ := srv.ResolveStateid([12]byte(id))
	if !result.Known || result.Incomplete || result.State != "staging" {
		t.Fatalf("unlocked staging not resolved: %+v", result)
	}
	missing, _ := srv.ResolveStateid([12]byte(diagnosticStateid(srv.opens.epoch, 2)))
	if missing.Known || missing.Incomplete {
		t.Fatalf("complete negative lookup: %+v", missing)
	}
}

func TestDiagnosticsStateidLookupFindsRecoveredEpoch(t *testing.T) {
	srv, store := diagnosticsStagingStore(t)
	id := diagnosticStateid(srv.opens.epoch+1, 1)
	const fileID = InodeID(0x4000000000000001)
	if _, err := store.Create(fileID, StagingMeta{
		DirID: srv.fs.RootID(), FileName: "recovered", ClientID: 7, NFSStateID: id,
	}); err != nil {
		t.Fatal(err)
	}
	// Reload the sidecar as a restarted daemon would. Its old stateid is
	// deliberately not in this server's process-local open-state map.
	store.closeAll()
	recovered, err := NewLocalStagingStore(store.dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(recovered.closeAll)
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
