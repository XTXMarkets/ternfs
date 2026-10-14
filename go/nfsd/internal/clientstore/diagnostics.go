// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientstore

import (
	"encoding/hex"
	"fmt"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Diagnostics snapshots of the client store, its confirmation attempts and
// lease refreshers, in the inspect API's types. Entry points take only the
// store's own locks and return copies; the server composes them (see
// server/diagnostics_snapshots.go for the locking rules).

// The API's text for ids; see inspect.Scalar and inspect.ID.
func inodeText(id vfs.InodeID) string     { return inspect.Scalar(inspect.KindInode, uint64(id)) }
func incarnationText(id uint64) string    { return inspect.Scalar(inspect.KindIncarnation, id) }
func stateidText(id nfs4.StateID) string  { return inspect.ID(inspect.KindStateid, id[:]) }
func identityHashText(name string) string { return inspect.IdentityHash(name) }

func fenceStateName(f fenceState) string {
	switch f {
	case fenceNone:
		return inspect.FenceNone
	case fenceNotApplied:
		return inspect.FenceNotApplied
	case fenceEdgeGone:
		return inspect.FenceEdgeGone
	case fenceFailed:
		return inspect.FenceFailed
	}
	return fmt.Sprintf("unknown_%d", f)
}

func writeOutcomeName(o writeOutcome) string {
	switch o {
	case writeNever:
		return inspect.WriteNever
	case writeApplied:
		return inspect.WriteApplied
	case writeMaybe:
		return inspect.WriteMaybe
	case writeUnsettled:
		return inspect.WriteUnsettled
	}
	return fmt.Sprintf("unknown_%d", o)
}

func confirmResultName(r ConfirmResult) string {
	switch r {
	case confirmUnresolved:
		return inspect.ConfirmUnresolved
	case ConfirmCommitted:
		return inspect.ConfirmCommitted
	case ConfirmRolledBack:
		return inspect.ConfirmRolledBack
	case ConfirmConservative:
		return inspect.ConfirmConservative
	}
	return fmt.Sprintf("unknown_%d", r)
}

// Diagnostics: the attempt as /v1/confirmations reports it. The mutex is
// held across backend I/O by a settle, so it is only tried; a locked
// attempt reports nothing else.
func (a *ConfirmAttempt) Snapshot() *inspect.Attempt {
	if !a.mu.TryLock() {
		return &inspect.Attempt{Locked: true}
	}
	defer a.mu.Unlock()
	out := &inspect.Attempt{Result: confirmResultName(a.result), Panicked: a.Panicked != nil}
	if a.PreWrite != nil {
		out.PreWrite = a.PreWrite.Error()
	}
	for i, w := range a.writes {
		if w == nil {
			continue
		}
		rec := inspect.ConfirmWrite{
			Name:      [...]string{"claim", "reboot", "confirmed"}[i],
			Dir:       inodeText(w.dir),
			Temp:      w.temp,
			Submitted: w.submitted,
			Fence:     fenceStateName(w.fence),
			Outcome:   writeOutcomeName(w.outcome()),
		}
		if w.err != nil {
			rec.Error = w.err.Error()
		}
		if w.fenceErr != nil {
			rec.FenceError = w.fenceErr.Error()
		}
		out.Writes = append(out.Writes, rec)
	}
	if a.observed.found || a.observed.id != 0 {
		out.Observed = &inspect.Observed{Found: a.observed.found}
		if a.observed.found {
			out.Observed.Confirmed = incarnationText(uint64(a.observed.id))
			out.Observed.File = inodeText(a.observed.file)
		}
	}
	return out
}

// refresherRecord copies a lease record. The caller holds r.mu.
func refresherRecord(r *LeaseRefresher) *inspect.Refresher {
	out := &inspect.Refresher{
		State:       leaseRecordName(r.State),
		Proven:      inspect.UnixNanos(r.Proven),
		VerifiedAt:  inspect.UnixNanos(r.VerifiedAt),
		LastExpiry:  inspect.UnixNanos(r.LastExpiry),
		HasAttempt:  r.Attempt != nil,
		Orphans:     []inspect.Orphan{},
		Bootstrap:   r.bootstrap,
		InCall:      r.InCall,
		Running:     r.running,
		Verify:      r.Verify,
		LastPass:    inspect.Time(r.lastPass),
		Refs:        r.refs,
		Finished:    r.Finished,
		Pinned:      r.pinned,
		NotBefore:   inspect.Time(r.NotBefore),
		MaintainAt:  inspect.Time(r.MaintainAt),
		IdleWaiting: r.IdleWaiting,
		Reclaim:     r.reclaim,
	}
	if r.identity != 0 {
		out.Identity = inodeText(r.identity)
	}
	if r.Backoff != 0 {
		out.Backoff = r.Backoff.String()
	}
	for _, o := range r.orphans {
		rec := inspect.Orphan{Temp: o.Temp, Phase: orphanPhaseName(o.Phase)}
		if o.Inode != 0 {
			rec.Inode = inodeText(o.Inode)
		}
		out.Orphans = append(out.Orphans, rec)
	}
	return out
}

func orphanPhaseName(ph orphanPhase) string {
	switch ph {
	case orphanConstructing:
		return inspect.OrphanConstructing
	case orphanLinking:
		return inspect.OrphanLinking
	case orphanLinked:
		return inspect.OrphanLinked
	case OrphanUnresolved:
		return inspect.OrphanUnresolved
	}
	return fmt.Sprintf("unknown_%d", ph)
}

func leaseRecordName(st leaseRecordState) string {
	switch st {
	case LeaseRecordLive:
		return inspect.LeaseLive
	case LeaseRecordStale:
		return inspect.LeaseStale
	case LeaseRecordFleetExpired:
		return inspect.LeaseFleetExpired
	}
	return fmt.Sprintf("unknown_%d", st)
}

// HasIncarnationState reports a lease record or a cached store location for
// id, for /v1/lookup.
func (cs *Store) HasIncarnationState(id vfs.InodeID) bool {
	cs.RefreshMu.Lock()
	_, hasRecord := cs.Refreshers[id]
	cs.RefreshMu.Unlock()
	if hasRecord {
		return true
	}
	_, cached := cs.LocalClients.Load(id)
	return cached
}

// SnapshotIncarnations copies the lease records and cached store locations,
// one partial record per incarnation that has either, for id or for all
// when id is zero. The record pointers are listed under refreshMu, which is
// released before each record's mu is taken; each cached location is read
// under its own mu.
func (cs *Store) SnapshotIncarnations(id vfs.InodeID) ([]inspect.Incarnation, inspect.IncarnationLimits) {
	cs.RefreshMu.Lock()
	limits := inspect.IncarnationLimits{
		Refreshers: len(cs.Refreshers), RefresherLimit: cs.RefresherLimit,
		Orphans: cs.orphans, OrphanLimit: cs.orphanLimit,
	}
	refreshers := make([]*LeaseRefresher, 0, len(cs.Refreshers))
	for inc, r := range cs.Refreshers {
		if id == 0 || inc == id {
			refreshers = append(refreshers, r)
		}
	}
	cs.RefreshMu.Unlock()
	recs := make(map[vfs.InodeID]*inspect.Incarnation)
	get := func(inc vfs.InodeID) *inspect.Incarnation {
		if recs[inc] == nil {
			recs[inc] = inspect.NewIncarnation(uint64(inc))
		}
		return recs[inc]
	}
	for _, r := range refreshers {
		r.Mu.Lock()
		rec := refresherRecord(r)
		r.Mu.Unlock()
		get(r.Incarnation).Refresher = rec
	}
	cs.LocalClients.Range(func(key, value any) bool {
		inc, ok := key.(vfs.InodeID)
		state, ok2 := value.(*localClientState)
		if !ok || !ok2 || id != 0 && inc != id {
			return true
		}
		state.mu.Lock()
		cached := &inspect.CachedClient{
			IdentityID:       inodeText(state.identityID),
			ConfirmedPointer: inodeText(state.confirmedPointer),
			Opens:            len(state.opens),
		}
		state.mu.Unlock()
		get(inc).Cached = cached
		return true
	})
	out := make([]inspect.Incarnation, 0, len(recs))
	for _, r := range recs {
		out = append(out, *r)
	}
	return out, limits
}

// AuthFlavorName names an RPC auth flavor for the diagnostics API.
func AuthFlavorName(flavor uint32) string {
	switch flavor {
	case nfs4.AUTH_NONE:
		return "auth_none"
	case nfs4.AUTH_SYS:
		return "auth_sys"
	case nfs4.RPCSEC_GSS:
		return "rpcsec_gss"
	}
	return fmt.Sprintf("flavor_%d", flavor)
}

// PrincipalRecord formats a principal for the diagnostics API.
func PrincipalRecord(p RPCPrincipal) inspect.Principal {
	out := inspect.Principal{Flavor: AuthFlavorName(p.Flavor), Text: formatPrincipal(p)}
	if p.Flavor != nfs4.AUTH_NONE {
		out.BodyHex = hex.EncodeToString([]byte(p.Body))
	}
	return out
}
