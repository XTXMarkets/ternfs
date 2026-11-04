// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientrecords"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/testutil"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The startup lease protects only incarnations a live record confirms. An
// incarnation that expired before the restart has no record and counts as
// expired at once, so the same client boot coming back straight after the
// restart recovers the writer it left.
func TestStartupLeaseDoesNotProtectAbsentRecord(t *testing.T) {
	local, dir := vfs.NewLocalTernVFS(t.TempDir()), t.TempDir()
	first := identityServerAt(t, local, dir)
	principal := clientrecords.Principal{Flavor: nfs4.AUTH_SYS, Body: "owner"}
	verifier := [8]byte{7}
	p := confirmForTest(t, first, "client", verifier, principal)
	rec, _ := first.records.Live(clientrecords.Incarnation(p))
	id, cookie, err := local.ConstructFile(local.RootID())
	if err != nil {
		t.Fatal(err)
	}
	sid := first.opens.NewStateID()
	sf, err := first.stagingStore.Create(id, staging.Meta{DirID: local.RootID(), FileName: "f",
		TernCookie: cookie, NFSStateID: sid, ClientID: p, OwnerKnown: true, OpenOwner: "o",
		RecoveryKey: rec.RecoveryKey()})
	if err != nil {
		t.Fatal(err)
	}
	// P expires in the first process: its record is removed and its writer
	// retained for recovery.
	if err := first.records.Remove(clientrecords.Incarnation(p)); err != nil {
		t.Fatal(err)
	}
	if err := sf.Retire(p, sid); err != nil {
		t.Fatal(err)
	}

	srv := identityServerAt(t, local, dir)
	if !srv.inStartupLease() {
		t.Fatal("not in the startup lease")
	}
	n := confirmForTest(t, srv, "client", verifier, principal)
	if expired, err := srv.decideExpired(p, false); err != nil || !expired {
		t.Fatalf("incarnation with no record in the startup lease: expired=%v err=%v", expired, err)
	}
	if expired, err := srv.decideExpired(n, false); err != nil || expired {
		t.Fatalf("confirmed incarnation in the startup lease: expired=%v err=%v", expired, err)
	}
	if _, _, found, err := srv.recoveredStagingTarget(local.RootID(), "f", n, "o", true); err != nil || !found {
		t.Fatalf("same boot's writer in the startup lease: found=%v err=%v", found, err)
	}
}

// No CREATE_SESSION enters while expiry is being decided. Expiry checks
// for creations before it sets the gate checking, so one admitted during
// the record removal could insert a session the expiry then retires: OK,
// then BADSESSION on its first SEQUENCE. It gets DELAY instead, and its
// retry sees the outcome.
func TestCreateSessionWaitsForExpiryDecision(t *testing.T) {
	srv := identityServerAt(t, vfs.NewLocalTernVFS(t.TempDir()), t.TempDir())
	wire, _ := registerForTest(t, srv, "client", 1)
	incarnation := incarnationForTest(t, srv, wire)
	awaitRequestHoldsReleased(t, srv)
	afterLeaseForTest(srv)

	entered, release := make(chan struct{}), make(chan struct{})
	remove := srv.removeClientRecord
	srv.removeClientRecord = func(id clientrecords.Incarnation) error {
		testutil.CloseSignal(entered)
		<-release
		return remove(id)
	}
	t.Cleanup(func() { testutil.CloseSignal(release) })
	decided := make(chan error, 1)
	go func() {
		expired, err := srv.decideExpired(incarnation, true)
		if err == nil && !expired {
			err = errors.New("lease that ran out not expired")
		}
		decided <- err
	}()
	testutil.AwaitSignal(t, entered, "expiry's record removal")

	status, id := createResult(t, srv, 1, wire, 2)
	if status != nfs4.NFS4ERR_DELAY {
		testutil.CloseSignal(release)
		if err := testutil.AwaitValue(t, decided, "expiry"); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("CREATE_SESSION while expiry was decided: %s, then SEQUENCE %s",
			nfs4.Nfsstat4Name(status), nfs4.Nfsstat4Name(sequenceStatusForTest(t, srv, id)))
	}
	testutil.CloseSignal(release)
	if err := testutil.AwaitValue(t, decided, "expiry"); err != nil {
		t.Fatal(err)
	}
	if status, _ := createResult(t, srv, 1, wire, 2); status != nfs4.NFS4ERR_STALE_CLIENTID {
		t.Fatalf("CREATE_SESSION retried after expiry: %s", nfs4.Nfsstat4Name(status))
	}
}
