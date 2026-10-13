// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
)

func TestDiagnosticsStagingFilterKeepsUnknownOwners(t *testing.T) {
	srv, store := diagnosticsStagingStore(t)
	const first = InodeID(0x4000000000000001)
	const second = InodeID(0x4000000000000002)
	for i, id := range []InodeID{first, second} {
		if _, err := store.Create(id, StagingMeta{
			DirID: srv.fs.RootID(), FileName: fmt.Sprint(i), ClientID: uint64(7 + i),
		}); err != nil {
			t.Fatal(err)
		}
	}
	sf := store.Get(first).(*localStagingFile)
	sf.mu.Lock()
	locked := true
	defer func() {
		if locked {
			sf.mu.Unlock()
		}
	}()

	for _, tc := range []struct {
		name       string
		filter     inspect.StagingFilter
		files      []InodeID
		incomplete bool
	}{
		{"all", inspect.StagingFilter{}, []InodeID{first, second}, true},
		{"owner with busy writer", inspect.StagingFilter{Incarnation: 7}, []InodeID{first}, true},
		{"other owner with possible match", inspect.StagingFilter{Incarnation: 8}, []InodeID{first, second}, true},
		{"known file excludes busy writer", inspect.StagingFilter{File: uint64(second), Incarnation: 8}, []InodeID{second}, false},
		{"known negative match", inspect.StagingFilter{File: uint64(second), Incarnation: 7}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := srv.Staging(tc.filter)
			if err != nil || out.Incomplete != tc.incomplete || len(out.Records) != len(tc.files) {
				t.Fatalf("staging = %+v, %v", out, err)
			}
			for i, id := range tc.files {
				if out.Records[i].FileID != inodeText(id) {
					t.Fatalf("record %d = %+v, want file %s", i, out.Records[i], inodeText(id))
				}
			}
		})
	}

	// The partial-result contract is also visible on the wire.
	httpServer := httptest.NewServer(inspect.Handler(srv, srv))
	defer func() {
		// A regressed lookup may still hold an HTTP request on this lock
		// after the test client's deadline. Release it before Close waits.
		if locked {
			sf.mu.Unlock()
			locked = false
		}
		httpServer.Close()
	}()
	out := getJSON(t, httpServer.URL, "/v1/staging?incarnation="+incarnationText(7), http.StatusOK)
	records := out["records"].([]any)
	if out["incomplete"] != true || len(records) != 1 || records[0].(map[string]any)["locked"] != true {
		t.Fatalf("HTTP staging result = %+v", out)
	}
	id := diagnosticStateid(srv.opens.epoch, 99)
	out = getJSON(t, httpServer.URL, "/v1/lookup?id="+stateidText(id), http.StatusOK)
	if out["incomplete"] != true || out["known"] != false {
		t.Fatalf("HTTP lookup result = %+v", out)
	}

	sf.mu.Unlock()
	locked = false
	for _, owner := range []uint64{7, 8} {
		out, err := srv.Staging(inspect.StagingFilter{Incarnation: owner})
		if err != nil || out.Incomplete || len(out.Records) != 1 ||
			out.Records[0].Incarnation != incarnationText(owner) {
			t.Fatalf("complete owner %d snapshot = %+v, %v", owner, out, err)
		}
	}
}

func TestDiagnosticsHydrationReportsWorkerActivity(t *testing.T) {
	for _, stop := range []string{"complete", "cancel"} {
		t.Run(stop, func(t *testing.T) {
			srv, store := diagnosticsStagingStore(t)
			file, err := store.Create(InodeID(0x4000000000000001), StagingMeta{
				DirID: srv.fs.RootID(), FileName: "hydrate", ClientID: 7,
				BaseID: InodeID(0x4000000000000002), BaseSize: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			sf := file.(*localStagingFile)
			check := func(want bool) {
				t.Helper()
				out, err := srv.Staging(inspect.StagingFilter{})
				if err != nil || len(out.Records) != 1 || out.Records[0].Hydrating != want {
					t.Fatalf("hydrating want %t: %+v, %v", want, out, err)
				}
			}
			check(false)
			entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			defer closeSignal(release)
			sf.StartHydration(func(cancel <-chan struct{}, _ InodeID, _ uint64, dest []byte) (int, bool, error) {
				close(entered)
				if stop == "cancel" {
					<-cancel
					close(cancelled)
				}
				<-release
				dest[0] = 'x'
				return 1, true, nil
			})
			awaitSignal(t, entered, "hydration reader")
			check(true)
			sf.hydrateMu.Lock()
			done := sf.hydrateDone
			if stop == "cancel" {
				sf.cancelHydrationLocked()
			}
			sf.hydrateMu.Unlock()
			if stop == "cancel" {
				awaitSignal(t, cancelled, "hydration cancellation")
				check(true) // Cancellation requested, but the reader has not exited.
			}
			closeSignal(release)
			awaitSignal(t, done, "hydration completion")
			check(false)
		})
	}
}

// A failed tombstone records no owner, so the file filter applies to the
// tombstones and the incarnation filter does not.
func TestDiagnosticsStagingTombstoneFilter(t *testing.T) {
	srv, store := diagnosticsStagingStore(t)
	const failed = InodeID(0x4000000000000001)
	const live = InodeID(0x4000000000000002)
	for i, id := range []InodeID{failed, live} {
		if _, err := store.Create(id, StagingMeta{DirID: srv.fs.RootID(), FileName: fmt.Sprint(i), ClientID: 7}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Quarantine(failed); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		filter  inspect.StagingFilter
		records int
		failed  []string
	}{
		{"all", inspect.StagingFilter{}, 1, []string{inodeText(failed)}},
		{"live file", inspect.StagingFilter{File: uint64(live)}, 1, []string{}},
		{"failed file", inspect.StagingFilter{File: uint64(failed)}, 0, []string{inodeText(failed)}},
		{"owner", inspect.StagingFilter{Incarnation: 7}, 1, []string{inodeText(failed)}},
	} {
		out, err := srv.Staging(tc.filter)
		if err != nil || len(out.Records) != tc.records || fmt.Sprint(out.Failed) != fmt.Sprint(tc.failed) {
			t.Fatalf("%s: staging = %+v, %v; want %d records, failed %v", tc.name, out, err, tc.records, tc.failed)
		}
	}
}
