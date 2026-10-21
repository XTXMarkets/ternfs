// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Confirmation of a pending incarnation N over the identity's confirmed
// predecessor P (README.md#writes-and-fencing). Three replace-by-rename writes:
//
//	A  N/confirming.<nfsdID>       a claim that keeps N from GC
//	B  N/reboot -> P               evidence for purging P afterwards
//	D  identity/confirmed -> N     the commit
//
// Each write renames one pinned temp edge; a rename error is fenced by
// removing that edge, and the result is decided by reading the confirmed
// pointer back (README.md#writes-and-fencing).
//
// Ownership: a ConfirmAttempt's mutex serializes SettleConfirmation,
// FinishReplacement and Result on it, and is taken
// before the identity lock. CommitConfirmation takes the identity lock first
// and then the mutex of the attempt it creates, which is not yet published
// to anyone else, so the two orders cannot meet. A settled result is never
// recomputed. The caller still owns the attempt: it must act on each result
// once, and keep at most one attempt per identity.

var (
	// errPredecessorChanged: the confirmed pointer is no longer the P and
	// Sp that PrepareConfirmation read. Nothing was written, which does not
	// prove that P is unchanged; the read-back classifies the attempt.
	errPredecessorChanged = errors.New("client store: confirmed incarnation changed since preparation")
	// errOutcomeUnknown marks a backend call whose result was never
	// returned: a rename that may have applied, or a fence that may have.
	errOutcomeUnknown = errors.New("client store: backend call outcome unknown")
	// ErrPurgeProtected: a resumed purge was deferred because a
	// confirmation in this process protects n or its selected target.
	// Nothing was removed.
	ErrPurgeProtected = errors.New("client store: replacement purge deferred by a confirmation")
)

// confirmTarget is what PrepareConfirmation read, without writing.
type confirmTarget struct {
	Identity, N vfs.InodeID
	// p is the confirmed incarnation and sp its confirmed symlink's inode,
	// both zero when the identity has no confirmed incarnation.
	P, sp vfs.InodeID
}

type WriteID int

const (
	writeA WriteID = iota
	writeB
	WriteD
)

type fenceState uint8

const (
	fenceNone       fenceState = iota // not needed or not yet tried
	fenceNotApplied                   // RemoveEdge took the temp: never applies
	fenceEdgeGone                     // the temp edge is gone: may have applied
	fenceFailed                       // RemoveEdge failed or never returned: unknown
)

// edgeWrite is the attempt record of one write. It is part of its attempt
// before any backend call, so a panic cannot lose a submitted write.
type edgeWrite struct {
	dir        vfs.InodeID
	temp, name string
	edge       vfs.Edge
	inode      vfs.InodeID // the temp's inode
	submitted  bool        // RenameEdge was called
	err        error       // the pre-send or RenameEdge error, or errOutcomeUnknown
	fence      fenceState
	fenceErr   error
}

type writeOutcome uint8

const (
	writeNever     writeOutcome = iota // not attempted, failed before sending, or fenced
	writeApplied                       // RenameEdge succeeded
	writeMaybe                         // fenced as edge gone: may have applied
	writeUnsettled                     // the fence has not succeeded
)

func (w *edgeWrite) outcome() writeOutcome {
	switch {
	case w == nil || !w.submitted || w.fence == fenceNotApplied:
		return writeNever
	case w.err == nil:
		return writeApplied
	case w.fence == fenceEdgeGone:
		return writeMaybe
	}
	return writeUnsettled
}

type ConfirmResult uint8

const (
	// confirmUnresolved: a fence or the read-back failed. All evidence is
	// kept; nothing about N or P may be destroyed until a later settle.
	confirmUnresolved ConfirmResult = iota
	// ConfirmCommitted: confirmed names N. Only now may P be replaced.
	ConfirmCommitted
	// ConfirmRolledBack: D never applied and confirmed is still P with
	// symlink Sp. P is unchanged.
	ConfirmRolledBack
	// ConfirmConservative: neither committed nor provably unchanged. P is
	// treated as replaced locally and N's record is dropped; nothing durable
	// is deleted.
	ConfirmConservative
)

// ConfirmAttempt holds the evidence of one CommitConfirmation until it is
// settled. Its methods are called with no session-store lock held.
type ConfirmAttempt struct {
	mu       sync.Mutex
	target   confirmTarget
	writes   [3]*edgeWrite
	observed struct {
		id, file vfs.InodeID
		found    bool
	}
	result ConfirmResult
	// panicked is the last recovered panic, for the caller to log. The
	// attempt is then unresolved with all its evidence.
	Panicked any
	// preWrite is why no write was made, if none was: N no longer pending,
	// a changed predecessor, or a read error. The attempt is still
	// classified only by its read-back.
	PreWrite error
}

func (a *ConfirmAttempt) Result() ConfirmResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.result
}

// PrepareConfirmation checks that n is the identity's pending incarnation
// with this confirmation verifier, and reads its predecessor and the
// predecessor's confirmed symlink inode. It does not write. The caller then
// closes the predecessor; CommitConfirmation revalidates both afterwards.
func (cs *Store) PrepareConfirmation(n uint64, confirm [8]byte) (confirmTarget, error) {
	incarnationID := vfs.InodeID(n)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil {
		return confirmTarget{}, err
	}
	if !valid {
		return confirmTarget{}, nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	unlock := cs.readLockIdentity(identityID)
	defer unlock()
	if err := cs.checkPendingLocked(identityID, incarnationID, confirm); err != nil {
		return confirmTarget{}, err
	}
	target := confirmTarget{Identity: identityID, N: incarnationID}
	if target.P, target.sp, err = cs.readPredecessorLocked(identityID); err != nil {
		return confirmTarget{}, err
	}
	return target, nil
}

func (cs *Store) readPredecessorLocked(identityID vfs.InodeID) (vfs.InodeID, vfs.InodeID, error) {
	p, sp, found, err := cs.readIdentitySymlinkWithFile(identityID, ConfirmedName)
	if err != nil || !found {
		return 0, 0, err
	}
	return p, sp, nil
}

// checkPendingLocked requires n to be pending with the given verifier.
func (cs *Store) checkPendingLocked(identityID, n vfs.InodeID, confirm [8]byte) error {
	record, found, err := cs.readRecord(n, ClientRecordName)
	if err != nil {
		return err
	}
	if !found || !bytes.Equal(record.Confirm, confirm[:]) {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	pending, isPending, err := cs.ReadIdentitySymlink(identityID, pendingName)
	if err != nil {
		return err
	}
	if !isPending || pending != n {
		return nfs4.Error(nfs4.NFS4ERR_STALE_CLIENTID)
	}
	return nil
}

// CommitConfirmation makes writes A, B and D, stopping at the first that
// does not plainly succeed, then fences and observes. The caller has closed
// the target's P in this process and registered protection for N and P.
//
// Under the exclusive identity lock it first rechecks that N is pending and
// that the confirmed pointer is still the target's P through Sp. If either
// check fails or cannot be read, nothing is written and the attempt is
// classified by the read-back alone, with D never applied:
//   - confirmed still P through Sp proves P unchanged (rolled back), for
//     example when only N's pending state moved;
//   - confirmed naming N is committed elsewhere, with no B and so no purge;
//   - anything else is conservative;
//   - a read error leaves the attempt unresolved until SettleConfirmation
//     can read.
//
// It always returns an attempt, even after a panic: the attempt is then
// unresolved and keeps every write record.
func (cs *Store) CommitConfirmation(t confirmTarget, confirm [8]byte) (a *ConfirmAttempt) {
	unlock := cs.LockIdentity(t.Identity)
	defer unlock()
	a = &ConfirmAttempt{target: t}
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			a.Panicked, a.result = recovered, confirmUnresolved
		}
	}()
	a.PreWrite = cs.checkPendingLocked(t.Identity, t.N, confirm)
	if a.PreWrite == nil {
		p, sp, err := cs.readPredecessorLocked(t.Identity)
		switch {
		case err != nil:
			a.PreWrite = err
		case p != t.P || sp != t.sp:
			a.PreWrite = errPredecessorChanged
		}
	}
	var claim []byte
	if a.PreWrite == nil {
		claim, a.PreWrite = json.Marshal(DurableLease{ExpiresUnixNano: cs.Now().Add(ClientGCGrace).UnixNano()})
	}
	if a.PreWrite != nil {
		cs.settleLocked(a)
		return a
	}
	ok := func(w *edgeWrite) bool { return w.submitted && w.err == nil }
	a.writes[writeA] = &edgeWrite{dir: t.N, name: cs.ConfirmingName}
	cs.edgeReplace(a.writes[writeA], func(temp string) (vfs.InodeID, error) {
		return cs.fs.CreateFile(t.N, temp, bytes.NewReader(claim))
	})
	if ok(a.writes[writeA]) && t.P != 0 && t.P != t.N {
		a.writes[writeB] = &edgeWrite{dir: t.N, name: RebootName}
		cs.writePointer(a.writes[writeB], t.Identity, t.P, "../")
	}
	if ok(a.writes[writeA]) && (a.writes[writeB] == nil || ok(a.writes[writeB])) && t.P != t.N {
		a.writes[WriteD] = &edgeWrite{dir: t.Identity, name: ConfirmedName}
		cs.writePointer(a.writes[WriteD], t.Identity, t.N, "")
	}
	cs.settleLocked(a)
	return a
}

// writePointer writes w.dir/w.name as a symlink to prefix+(target's name
// below the identity).
func (cs *Store) writePointer(w *edgeWrite, identityID, target vfs.InodeID, prefix string) {
	targetName, found, err := cs.IncarnationName(identityID, target)
	if err == nil && !found {
		err = fmt.Errorf("client store: incarnation %d is not below identity %d", target, identityID)
	}
	if err != nil {
		w.err = err
		return
	}
	cs.edgeReplace(w, func(temp string) (vfs.InodeID, error) {
		return cs.fs.Symlink(w.dir, temp, prefix+targetName)
	})
}

// edgeReplace replaces w.dir/w.name with a new temp through its exact edge,
// recording each step in w as it goes.
func (cs *Store) edgeReplace(w *edgeWrite, create func(string) (vfs.InodeID, error)) {
	suffix, err := randomHex(8)
	if err != nil {
		w.err = err
		return
	}
	w.temp = TempPrefix + suffix
	if w.inode, w.err = create(w.temp); w.err != nil {
		return
	}
	if w.edge, w.err = cs.fs.LookupEdge(w.dir, w.temp); w.err != nil {
		return
	}
	if w.edge.ID != w.inode {
		w.err = fmt.Errorf("client store: temp %s names inode %d, created %d", w.temp, w.edge.ID, w.inode)
		return
	}
	// Until RenameEdge returns, its outcome is unknown.
	w.submitted, w.err = true, errOutcomeUnknown
	w.err = cs.fs.RenameEdge(w.dir, w.temp, w.edge, w.dir, w.name)
}

// SettleConfirmation retries the fences and the read-back of an unresolved
// attempt. Settled attempts keep their result. It never writes again.
func (cs *Store) SettleConfirmation(a *ConfirmAttempt) (result ConfirmResult) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.result != confirmUnresolved {
		return a.result
	}
	unlock := cs.LockIdentity(a.target.Identity)
	defer unlock()
	defer func() {
		if recovered := recover(); recovered != nil {
			a.Panicked, a.result = recovered, confirmUnresolved
			result = confirmUnresolved
		}
	}()
	cs.settleLocked(a)
	return a.result
}

// settleLocked fences and observes. The caller holds a.mu and the identity
// lock. A panic leaves the attempt unresolved: every unfinished fence is
// recorded as failed before it is sent.
func (cs *Store) settleLocked(a *ConfirmAttempt) {
	a.result = confirmUnresolved
	settled := true
	for _, w := range a.writes {
		if w == nil || !w.submitted || w.err == nil || w.fence == fenceNotApplied || w.fence == fenceEdgeGone {
			continue
		}
		w.fence, w.fenceErr = fenceFailed, errOutcomeUnknown
		err := cs.fs.RemoveEdge(w.dir, w.temp, w.edge)
		switch {
		case err == nil:
			w.fence, w.fenceErr = fenceNotApplied, nil
		case isEdgeGone(err):
			w.fence, w.fenceErr = fenceEdgeGone, nil
		default:
			w.fenceErr = err
			settled = false
		}
	}
	if !settled {
		return
	}
	id, file, found, err := cs.readIdentitySymlinkWithFile(a.target.Identity, ConfirmedName)
	if err != nil {
		return
	}
	a.observed.id, a.observed.file, a.observed.found = id, file, found
	a.result = classifyConfirmation(a.target, a.writes[WriteD], id, file, found)
}

// classifyConfirmation is the observation table (README.md#observation-table).
// Missing data never proves a rollback: only D never applied and confirmed
// still P through Sp does.
func classifyConfirmation(t confirmTarget, d *edgeWrite, id, file vfs.InodeID, found bool) ConfirmResult {
	outcome := d.outcome()
	switch {
	case found && id == t.N && d != nil && file == d.inode && (outcome == writeApplied || outcome == writeMaybe):
		return ConfirmCommitted
	case found && id == t.N && (d == nil || file != d.inode):
		return ConfirmCommitted // committed elsewhere, or already
	case outcome == writeNever && t.P != 0 && found && id == t.P && file == t.sp:
		return ConfirmRolledBack
	}
	// P with another symlink, another incarnation M, or no confirmed
	// symlink, including a first confirmation that never applied.
	return ConfirmConservative
}

// isEdgeGone recognises a removal whose exact edge no longer exists: the
// remote backend's errors and the local VFS's inode-pinned not-found.
func isEdgeGone(err error) bool {
	return errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, msgs.EDGE_NOT_FOUND) ||
		errors.Is(err, msgs.NAME_NOT_FOUND) ||
		errors.Is(err, msgs.MISMATCHING_CREATION_TIME)
}

// FinishReplacement completes a committed confirmation. P is purged only if
// write B applied or may have, and N's reboot symlink, read through its
// exact edge, still names P. It returns the purged predecessor, or zero. A
// purge failure leaves the reboot symlink as evidence for
// ResumeReplacementPurge.
func (cs *Store) FinishReplacement(a *ConfirmAttempt) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.result != ConfirmCommitted {
		panic("FinishReplacement of an unconfirmed attempt")
	}
	t := a.target
	unlock := cs.LockIdentity(t.Identity)
	defer unlock()
	// Only this process writes its own confirming name, and only for an
	// attempt the caller serializes, so removing it by name is exact.
	defer func() { _ = cs.removeIfExists(t.N, cs.ConfirmingName) }()
	b := a.writes[writeB]
	if t.P == 0 || (b.outcome() != writeApplied && b.outcome() != writeMaybe) {
		return 0, nil
	}
	// The attempt's own confirmation protects N and P until this returns.
	return cs.purgeReplacedLocked(t.N, t.P, nil)
}

// ResumeReplacementPurge completes, from durable evidence alone, a purge that
// an earlier confirmation of n left unfinished: in this process, in another,
// or before a restart. It needs no attempt, so it survives wire retirement
// and restarts.
//
// Evidence, read under the exclusive identity lock: the confirmed pointer
// names n, and n's reboot symlink, through one exact edge, names an
// incarnation p other than n. Then p is purged and that exact edge removed.
//
// It acts on the same evidence the NFSv4.0 path's finishConfirmed purges on,
// adding only the confirmed check and the exact edge, and is not a proof that
// p can never be used again: the NFSv4.0 callback update can move confirmed
// back to it, and the identity lock is local (README.md#replacement-purge).
// It grants nothing on behalf of an attempt whose own B never applied.
//
// Locally, the purge is ordered with confirmation protection: once p is
// selected, beginPurge holds destruction holds on n and p through the purge
// and the edge removal, and a protected n or p defers it with
// ErrPurgeProtected, evidence unchanged.
//
// It returns the purged incarnation, or zero when there is nothing to do.
func (cs *Store) ResumeReplacementPurge(n uint64) (uint64, error) {
	incarnationID := vfs.InodeID(n)
	identityID, valid, err := cs.identityForIncarnation(incarnationID)
	if err != nil || !valid {
		return 0, err
	}
	unlock := cs.LockIdentity(identityID)
	defer unlock()
	confirmedID, confirmed, err := cs.ReadIdentitySymlink(identityID, ConfirmedName)
	if err != nil || !confirmed || confirmedID != incarnationID {
		return 0, err
	}
	return cs.purgeReplacedLocked(incarnationID, 0, cs.BeginPurge)
}

// purgeReplacedLocked purges p if n/reboot names p, then removes that exact
// reboot edge. The target stays the one the edge named when checked,
// whatever the symlink names later, and a reboot symlink replaced meanwhile,
// by another process, is left alone. With p zero, the edge's target is the
// one purged, for ResumeReplacementPurge's durable evidence. A non-nil begin
// admits the purge of the selected target, and its release runs when the
// purge returns or unwinds.
func (cs *Store) purgeReplacedLocked(n, p vfs.InodeID, begin func(n, p vfs.InodeID) (func(), bool)) (uint64, error) {
	edge, err := cs.fs.LookupEdge(n, RebootName)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	target, err := cs.resolvePointer(n, edge.ID, true)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil || target == n || p != 0 && target != p {
		return 0, err
	}
	p = target
	if begin != nil {
		end, ok := begin(n, p)
		if !ok {
			return 0, ErrPurgeProtected
		}
		defer end()
	}
	if err := cs.purgeState(p); err != nil {
		return 0, err
	}
	cs.LocalClients.Delete(p)
	if err := cs.fs.RemoveEdge(n, RebootName, edge); err != nil && !isEdgeGone(err) {
		return uint64(p), err
	}
	return uint64(p), nil
}
