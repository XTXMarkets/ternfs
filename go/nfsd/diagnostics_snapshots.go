// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
)

// The diagnostics snapshots of every store, kept out of the files that
// implement the stores. The owner-facing entry points here are methods of
// the type that owns the state they read: each acquires that type's own
// locks, in the order the type's own writers use, copies into the API's
// types and returns, so the aggregator in diagnostics.go holds no lock and
// reads no guarded field. The private record builders (clientRecord,
// groupRecord, refresherRecord, openState.snapshot, confirmation.snapshot
// and the enum-name helpers) run under a lock their owner method has
// already taken. Ordinary registry and record mutexes are acquired
// normally. Three locks are only tried, because a request path tries them
// or they are held across I/O: session slots, staging files and
// confirmation attempts. A snapshot never waits for their holders; it
// reports the entry as locked instead.

func decodeSequenceFlags(flags uint32) []string {
	names := []string{}
	for _, f := range sequenceFlagNames {
		if flags&f.flag != 0 {
			names = append(names, f.name)
			flags &^= f.flag
		}
	}
	if flags != 0 {
		names = append(names, fmt.Sprintf("0x%08x", flags))
	}
	return names
}

var sequenceFlagNames = []struct {
	flag uint32
	name string
}{
	{SEQ4_STATUS_CB_PATH_DOWN, "cb_path_down"},
	{SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRING, "cb_gss_contexts_expiring"},
	{SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRED, "cb_gss_contexts_expired"},
	{SEQ4_STATUS_EXPIRED_ALL_STATE_REVOKED, "expired_all_state_revoked"},
	{SEQ4_STATUS_EXPIRED_SOME_STATE_REVOKED, "expired_some_state_revoked"},
	{SEQ4_STATUS_ADMIN_STATE_REVOKED, "admin_state_revoked"},
	{SEQ4_STATUS_RECALLABLE_STATE_REVOKED, "recallable_state_revoked"},
	{SEQ4_STATUS_LEASE_MOVED, "lease_moved"},
	{SEQ4_STATUS_RESTART_RECLAIM_NEEDED, "restart_reclaim_needed"},
	{SEQ4_STATUS_CB_PATH_DOWN_SESSION, "cb_path_down_session"},
	{SEQ4_STATUS_BACKCHANNEL_FAULT, "backchannel_fault"},
	{SEQ4_STATUS_DEVID_CHANGED, "devid_changed"},
	{SEQ4_STATUS_DEVID_DELETED, "devid_deleted"},
}

func cbPathName(p cbPath) string {
	switch p {
	case cbPathProbing:
		return inspect.PathProbing
	case cbPathUp:
		return inspect.PathUp
	case cbPathUnresponsive:
		return inspect.PathUnresponsive
	}
	return fmt.Sprintf("unknown_%d", p)
}

// snapshot copies the sender's state for diagnostics, with the back
// connections to describe outside cb.mu.
func (cb *cbSender) snapshot() (inspect.Backchannel, []*rpcConnection) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return inspect.Backchannel{
		Program:          cb.program,
		Credential:       authFlavorName(cb.sec.flavor),
		NextSeq:          cb.seq,
		Fault:            cb.fault,
		RecallPending:    cb.recall,
		RecallTarget:     cb.target,
		InFlight:         cb.inFlight,
		LastRecallStatus: Nfsstat4Name(cb.lastRecallStatus),
		Running:          cb.running,
		Cancelled:        cb.cancelled,
		Connections:      []inspect.BackConn{},
	}, append([]*rpcConnection(nil), cb.conns...)
}

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

func confirmResultName(r confirmResult) string {
	switch r {
	case confirmUnresolved:
		return inspect.ConfirmUnresolved
	case confirmCommitted:
		return inspect.ConfirmCommitted
	case confirmRolledBack:
		return inspect.ConfirmRolledBack
	case confirmConservative:
		return inspect.ConfirmConservative
	}
	return fmt.Sprintf("unknown_%d", r)
}

// Diagnostics: the attempt as /v1/confirmations reports it. The mutex is
// held across backend I/O by a settle, so it is only tried; a locked
// attempt reports nothing else.
func (a *confirmAttempt) snapshot() *inspect.Attempt {
	if !a.mu.TryLock() {
		return &inspect.Attempt{Locked: true}
	}
	defer a.mu.Unlock()
	out := &inspect.Attempt{Result: confirmResultName(a.result), Panicked: a.panicked != nil}
	if a.preWrite != nil {
		out.PreWrite = a.preWrite.Error()
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
func refresherRecord(r *leaseRefresher) *inspect.Refresher {
	out := &inspect.Refresher{
		State:       leaseRecordName(r.state),
		Proven:      inspect.UnixNanos(r.proven),
		VerifiedAt:  inspect.UnixNanos(r.verifiedAt),
		LastExpiry:  inspect.UnixNanos(r.lastExpiry),
		HasAttempt:  r.attempt != nil,
		Orphans:     []inspect.Orphan{},
		Bootstrap:   r.bootstrap,
		InCall:      r.inCall,
		Running:     r.running,
		Verify:      r.verify,
		LastPass:    inspect.Time(r.lastPass),
		Refs:        r.refs,
		Finished:    r.finished,
		Pinned:      r.pinned,
		NotBefore:   inspect.Time(r.notBefore),
		MaintainAt:  inspect.Time(r.maintainAt),
		IdleWaiting: r.idleWaiting,
		Reclaim:     r.reclaim,
	}
	if r.identity != 0 {
		out.Identity = inodeText(r.identity)
	}
	if r.backoff != 0 {
		out.Backoff = r.backoff.String()
	}
	for _, o := range r.orphans {
		rec := inspect.Orphan{Temp: o.temp, Phase: orphanPhaseName(o.phase)}
		if o.inode != 0 {
			rec.Inode = inodeText(o.inode)
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
	case orphanUnresolved:
		return inspect.OrphanUnresolved
	}
	return fmt.Sprintf("unknown_%d", ph)
}

func leaseRecordName(st leaseRecordState) string {
	switch st {
	case leaseRecordLive:
		return inspect.LeaseLive
	case leaseRecordStale:
		return inspect.LeaseStale
	case leaseRecordFleetExpired:
		return inspect.LeaseFleetExpired
	}
	return fmt.Sprintf("unknown_%d", st)
}

// hasIncarnationState reports a lease record or a cached store location for
// id, for /v1/lookup.
func (cs *ClientStore) hasIncarnationState(id InodeID) bool {
	cs.refreshMu.Lock()
	_, hasRecord := cs.refreshers[id]
	cs.refreshMu.Unlock()
	if hasRecord {
		return true
	}
	_, cached := cs.localClients.Load(id)
	return cached
}

// snapshotIncarnations copies the lease records and cached store locations,
// one partial record per incarnation that has either, for id or for all
// when id is zero. The record pointers are listed under refreshMu, which is
// released before each record's mu is taken; each cached location is read
// under its own mu.
func (cs *ClientStore) snapshotIncarnations(id InodeID) ([]inspect.Incarnation, inspect.IncarnationLimits) {
	cs.refreshMu.Lock()
	limits := inspect.IncarnationLimits{
		Refreshers: len(cs.refreshers), RefresherLimit: cs.refresherLimit,
		Orphans: cs.orphans, OrphanLimit: cs.orphanLimit,
	}
	refreshers := make([]*leaseRefresher, 0, len(cs.refreshers))
	for inc, r := range cs.refreshers {
		if id == 0 || inc == id {
			refreshers = append(refreshers, r)
		}
	}
	cs.refreshMu.Unlock()
	recs := make(map[InodeID]*inspect.Incarnation)
	get := func(inc InodeID) *inspect.Incarnation {
		if recs[inc] == nil {
			recs[inc] = newIncarnationRecord(uint64(inc))
		}
		return recs[inc]
	}
	for _, r := range refreshers {
		r.mu.Lock()
		rec := refresherRecord(r)
		r.mu.Unlock()
		get(r.incarnation).Refresher = rec
	}
	cs.localClients.Range(func(key, value any) bool {
		inc, ok := key.(InodeID)
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

// snapshot copies the open as /v1/opens reports it. The caller holds os.mu.
func (st *openState) snapshot() inspect.Open {
	out := inspect.Open{
		Stateid:     stateidText(st.id),
		FileID:      inodeText(st.fileID),
		DataID:      inodeText(st.dataID),
		Incarnation: incarnationText(st.owner.clientID),
		OwnerHex:    hex.EncodeToString([]byte(st.owner.owner)),
		Access:      []string{},
		ShareModes:  st.modes,
		DenyModes:   st.denyModes,
		Generation:  st.generation,
		Seq:         st.seq,
		Writer:      st.writes(),
	}
	if st.access&OPEN4_SHARE_ACCESS_READ != 0 {
		out.Access = append(out.Access, "read")
	}
	if st.access&OPEN4_SHARE_ACCESS_WRITE != 0 {
		out.Access = append(out.Access, "write")
	}
	return out
}

// fileOpens counts the live opens whose filehandle or data inode is inode.
func (os *openStateStore) fileOpens(inode InodeID) int {
	os.mu.Lock()
	defer os.mu.Unlock()
	n := 0
	for _, st := range os.states {
		if st.fileID == inode || st.dataID == inode {
			n++
		}
	}
	return n
}

// stateidOwner classifies one stateid in a single observation, so a stateid
// moving from live to expired is never seen as both or neither.
func (os *openStateStore) stateidOwner(id StateID) (owner uint64, live, expired bool) {
	os.mu.Lock()
	defer os.mu.Unlock()
	if st := os.states[id]; st != nil {
		return st.owner.clientID, true, false
	}
	owner, expired = os.expired[id]
	return owner, false, expired
}

// opensByIncarnation counts the live opens of each incarnation, or of id
// alone when it is not zero.
func (os *openStateStore) opensByIncarnation(id uint64) map[uint64]int {
	os.mu.Lock()
	defer os.mu.Unlock()
	counts := make(map[uint64]int)
	for _, st := range os.states {
		if id == 0 || st.owner.clientID == id {
			counts[st.owner.clientID]++
		}
	}
	return counts
}

// snapshot copies the opens and expired tombstones matching f for /v1/opens.
// A File filter selects by filehandle or data inode and excludes the
// tombstones, which record neither.
func (os *openStateStore) snapshot(f inspect.OpenFilter) (records []inspect.Open, expired []inspect.ExpiredStateid) {
	os.mu.Lock()
	defer os.mu.Unlock()
	records, expired = []inspect.Open{}, []inspect.ExpiredStateid{}
	for id, st := range os.states {
		if f.Stateid != nil && id != StateID(*f.Stateid) ||
			f.Incarnation != 0 && st.owner.clientID != f.Incarnation ||
			f.File != 0 && st.fileID != InodeID(f.File) && st.dataID != InodeID(f.File) {
			continue
		}
		records = append(records, st.snapshot())
	}
	for id, owner := range os.expired {
		if f.Stateid != nil && id != StateID(*f.Stateid) || f.Incarnation != 0 && owner != f.Incarnation || f.File != 0 {
			continue
		}
		expired = append(expired, inspect.ExpiredStateid{Stateid: stateidText(id), Incarnation: incarnationText(owner)})
	}
	return records, expired
}

// carries reports an association with id in either direction.
func (c *rpcConnection) carries(id SessionID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, fore := c.sessions[id]
	_, back := c.back[id]
	return fore || back
}

// snapshot copies the connection for /v1/connections. The path is read
// through backPath, which also reports a closed connection.
func (c *rpcConnection) snapshot() inspect.Connection {
	path := c.backPath()
	c.mu.Lock()
	defer c.mu.Unlock()
	out := inspect.Connection{
		Remote:               c.remote,
		ConnectedAt:          c.connectedAt,
		InFlight:             c.inFlight.Load(),
		Requests:             c.requests.Load(),
		Sessions:             []string{},
		BackSessions:         []string{},
		OutstandingCallbacks: len(c.calls),
		ProbeOutstanding:     c.nullCall,
		Path:                 cbPathName(path),
	}
	for id := range c.sessions {
		out.Sessions = append(out.Sessions, sessionText(id))
	}
	for _, n := range c.pending {
		out.PendingAssociations += n
	}
	for id := range c.back {
		out.BackSessions = append(out.BackSessions, sessionText(id))
	}
	for _, n := range c.backPending {
		out.PendingBackAssociations += n
	}
	sort.Strings(out.Sessions)
	sort.Strings(out.BackSessions)
	return out
}

func authFlavorName(flavor uint32) string {
	switch flavor {
	case authNone:
		return "auth_none"
	case authSys:
		return "auth_sys"
	case RPCSEC_GSS:
		return "rpcsec_gss"
	}
	return fmt.Sprintf("flavor_%d", flavor)
}

func principalRecord(p rpcPrincipal) inspect.Principal {
	out := inspect.Principal{Flavor: authFlavorName(p.flavor), Text: formatPrincipal(p)}
	if p.flavor != authNone {
		out.BodyHex = hex.EncodeToString([]byte(p.body))
	}
	return out
}

// liveConns copies the connection registry.
func (s *Server) liveConns() []*rpcConnection {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	out := make([]*rpcConnection, 0, len(s.conns))
	for c := range s.conns {
		out = append(out, c)
	}
	return out
}

// clientGCSnapshot copies the client GC queue state for /v1/status.
func (s *Server) clientGCSnapshot() inspect.ClientGC {
	s.clientGCMu.Lock()
	defer s.clientGCMu.Unlock()
	return inspect.ClientGC{
		Pending: len(s.clientGCPending),
		Recheck: len(s.clientGCRecheck),
		Running: s.clientGCRunning,
	}
}

// snapshot copies the confirmation as /v1/confirmations reports it. The
// caller holds confirmMu; the attempt is read afterwards, under its own
// lock.
func (c *confirmation) snapshot() inspect.Confirmation {
	out := inspect.Confirmation{
		Identity: inodeText(c.identity),
		OwnerHex: hex.EncodeToString([]byte(c.owner)),
		New:      incarnationText(c.n),
		Applying: c.applying,
		Done:     c.done,
	}
	if c.p != 0 {
		out.Predecessor = incarnationText(c.p)
	}
	return out
}

// confirmHolds copies, for /v1/incarnations, which incarnations a
// confirmation protects as new or predecessor, the destruction holds on
// them and the purge hints, one partial record each, for id or for all when
// id is zero. Under confirmMu only: attempts are not inspected.
func (s *Server) confirmHolds(id uint64) []inspect.Incarnation {
	s.confirmMu.Lock()
	defer s.confirmMu.Unlock()
	recs := make(map[uint64]*inspect.Incarnation)
	get := func(inc uint64) *inspect.Incarnation {
		if id != 0 && inc != id {
			return nil
		}
		if recs[inc] == nil {
			recs[inc] = newIncarnationRecord(inc)
		}
		return recs[inc]
	}
	for inc := range s.confirming {
		if r := get(inc); r != nil {
			r.Confirming = true
		}
	}
	for inc, holds := range s.destroying {
		if r := get(inc); r != nil {
			r.DestroyHolds = holds
		}
	}
	for inc := range s.purgePending {
		if r := get(inc); r != nil {
			r.PurgePending = true
		}
	}
	out := make([]inspect.Incarnation, 0, len(recs))
	for _, r := range recs {
		out = append(out, *r)
	}
	return out
}

// snapshotConfirmations copies the registry for /v1/confirmations: the
// confirmations, destruction holds and purge hints under confirmMu, and
// then, with it released, each attempt under its own tried lock, which a
// settle holds across backend I/O. The envelope is left to the caller.
func (s *Server) snapshotConfirmations() inspect.Confirmations {
	out := inspect.Confirmations{
		Records:              []inspect.Confirmation{},
		DestroyHolds:         []inspect.Hold{},
		IdentityDestroyHolds: []inspect.Hold{},
		PurgePending:         []string{},
	}
	var attempts []*confirmAttempt
	s.confirmMu.Lock()
	for _, c := range s.confirmingIdentity {
		out.Records = append(out.Records, c.snapshot())
		attempts = append(attempts, c.attempt)
	}
	for id, holds := range s.destroying {
		out.DestroyHolds = append(out.DestroyHolds, inspect.Hold{ID: incarnationText(id), Holds: holds})
	}
	for id, holds := range s.destroyingIdentity {
		out.IdentityDestroyHolds = append(out.IdentityDestroyHolds, inspect.Hold{ID: inodeText(id), Holds: holds})
	}
	for id := range s.purgePending {
		out.PurgePending = append(out.PurgePending, incarnationText(id))
	}
	out.PurgeLimit = s.purgeLimit
	s.confirmMu.Unlock()
	for i, a := range attempts {
		if a != nil {
			out.Records[i].Attempt = a.snapshot()
		}
	}
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].Identity < out.Records[j].Identity })
	sort.Slice(out.DestroyHolds, func(i, j int) bool { return out.DestroyHolds[i].ID < out.DestroyHolds[j].ID })
	sort.Slice(out.IdentityDestroyHolds, func(i, j int) bool { return out.IdentityDestroyHolds[i].ID < out.IdentityDestroyHolds[j].ID })
	sort.Strings(out.PurgePending)
	return out
}

// snapshot copies the slot. SEQUENCE tries the same lock and answers DELAY
// when it is held, so this only tries it too: a slot a request holds is
// reported locked with no detail, and a request that arrives during this
// short hold retries as for any other holder.
func (slot *sessionSlot) snapshot(index int) inspect.Slot {
	if !slot.mu.TryLock() {
		return inspect.Slot{Index: index, Locked: true}
	}
	defer slot.mu.Unlock()
	out := inspect.Slot{
		Index:      index,
		Seq:        slot.seq,
		InUse:      slot.inUse,
		Cached:     slot.cached,
		Occupied:   slot.occupied,
		Reserved:   slot.reserved,
		ReplyBytes: len(slot.reply),
		HasBuffer:  slot.buffer != nil,
	}
	if slot.occupied {
		out.User = &inspect.User{Flavor: authFlavorName(slot.user.flavor), UID: slot.user.uid, GID: slot.user.gid}
	}
	return out
}

// snapshot copies the session with no store lock held: the channel limits
// and sender were fixed before insertion, the lifecycle flags are atomic,
// and users, wire and flags were read by the caller.
func (sess *session) snapshot(users int, wire string, flags uint32, withSlots bool) inspect.Session {
	rec := inspect.Session{
		ID:           sessionText(sess.id),
		Incarnation:  incarnationText(sess.clientID),
		WireClientID: wire,
		Fore: inspect.Channel{
			MaxRequestSize:  sess.maxReq,
			MaxResponseSize: sess.maxResp,
			MaxCachedSize:   sess.maxCached,
			MaxOperations:   sess.maxOps,
			Slots:           uint32(len(sess.slots)),
		},
		Back: inspect.Channel{
			MaxRequestSize:  sess.backMaxReq,
			MaxResponseSize: sess.backMaxResp,
			MaxOperations:   sess.backMaxOps,
			Slots:           sess.backMaxReqs,
		},
		BackRequested: sess.backRequested,
		CBProgram:     sess.cbProgram,
		Users:         users,
		Recalled:      sess.recalled.Load(),
		Destroyed:     sess.destroyed.Load(),
		Fatal:         sess.fatal.Load(),
		StatusFlags:   decodeSequenceFlags(flags),
		Links:         inspect.IncarnationLinks(incarnationText(sess.clientID)),
	}
	if sess.cb != nil {
		bc, conns := sess.cb.snapshot()
		for _, c := range conns {
			bc.Connections = append(bc.Connections, inspect.BackConn{Remote: c.remote, Path: cbPathName(c.backPath())})
		}
		rec.Backchannel = &bc
	}
	if withSlots {
		rec.Slots = make([]inspect.Slot, 0, len(sess.slots))
		for i := range sess.slots {
			rec.Slots = append(rec.Slots, sess.slots[i].snapshot(i))
		}
	}
	return rec
}

// snapshotSessions copies the sessions matching f. The store is enumerated
// once under ss.mu. The sender of every session of a matched incarnation is
// then observed once with no store lock held, and the SEQUENCE status flags
// follow sequenceFlags: a session without a sender has none and does not
// count towards its client's path state; the client's path is down when a
// session with a sender asked for a backchannel and none has a path up or
// probing. A filter limits the records, not the sessions consulted for the
// client-wide flags.
func (ss *sessionStore) snapshotSessions(f inspect.SessionFilter) []inspect.Session {
	type picked struct {
		sess               *session
		users              int
		wire               string
		matched            bool
		fault, pending, up bool
	}
	type clientPath struct{ requested, up bool }
	ss.mu.Lock()
	list := make([]picked, 0, len(ss.sessions))
	clients := make(map[uint64]*clientPath) // incarnations with a matched session
	for id, sess := range ss.sessions {
		p := picked{sess: sess, users: sess.users}
		p.matched = !(f.ID != nil && id != SessionID(*f.ID) || f.Incarnation != 0 && sess.clientID != f.Incarnation)
		if p.matched {
			if g := ss.groups[sess.clientID]; g != nil && g.wire != nil {
				p.wire = wireText(g.wire.clientID)
			}
			if clients[sess.clientID] == nil {
				clients[sess.clientID] = &clientPath{}
			}
		}
		list = append(list, p)
	}
	ss.mu.Unlock()
	for i := range list {
		p := &list[i]
		client := clients[p.sess.clientID]
		if client == nil || p.sess.cb == nil {
			continue
		}
		p.fault, p.pending = p.sess.cb.flags()
		p.up = p.sess.cb.pathUp()
		client.requested = client.requested || p.sess.backRequested
		client.up = client.up || p.up
	}
	out := make([]inspect.Session, 0, len(list))
	for _, p := range list {
		if !p.matched {
			continue
		}
		var flags uint32
		if p.sess.cb != nil {
			client := clients[p.sess.clientID]
			flags = sequenceFlagsFrom(p.fault, p.pending, p.up, p.sess.backRequested, !p.up && client.requested && !client.up)
		}
		out = append(out, p.sess.snapshot(p.users, p.wire, flags, f.Slots))
	}
	return out
}

// groupRecord copies a group. The caller holds ss.mu.
func groupRecord(g *clientGroup) *inspect.Group {
	return &inspect.Group{
		Members:            g.members,
		Provisional:        g.provisional,
		Active:             g.active,
		Creating:           g.creating,
		State:              groupStateName(g.state),
		QuietUntil:         inspect.Time(g.quietUntil),
		LeaseGate:          leaseGateName(g.lease),
		MarkerPhase:        markerPhaseName(g.markerPhase),
		RenewedDuringCheck: g.renewedDuringCheck,
		Dropped:            g.dropped,
		ReclaimComplete:    g.reclaimComplete,
	}
}

func markerPhaseName(ph markerPhase) string {
	switch ph {
	case markerNone:
		return inspect.MarkerNone
	case markerDeciding:
		return inspect.MarkerDeciding
	case markerSubmitted:
		return inspect.MarkerSubmitted
	}
	return fmt.Sprintf("unknown_%d", ph)
}

func leaseGateName(st leaseGateState) string {
	switch st {
	case gateLive:
		return inspect.GateLive
	case gateChecking:
		return inspect.GateChecking
	case gateExpired:
		return inspect.GateExpired
	}
	return fmt.Sprintf("unknown_%d", st)
}

func groupStateName(st groupState) string {
	switch st {
	case groupOpen:
		return inspect.GroupOpen
	case groupClosing:
		return inspect.GroupClosing
	case groupReplaced:
		return inspect.GroupReplaced
	}
	return fmt.Sprintf("unknown_%d", st)
}

// incarnationKnown reports whether id has a group, and its wire clientid if
// one is bound, for /v1/lookup. Under ss.mu.
func (ss *sessionStore) incarnationKnown(id uint64) (wire string, known bool) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	g := ss.groups[id]
	if g == nil {
		return "", false
	}
	if g.wire != nil {
		wire = wireText(g.wire.clientID)
	}
	return wire, true
}

// snapshotIncarnations copies the groups and the session memberships, one
// partial record per incarnation, for id or for all when id is zero. The
// copies are made under ss.mu and sorted after it is released.
func (ss *sessionStore) snapshotIncarnations(id uint64) ([]inspect.Incarnation, inspect.IncarnationLimits) {
	ss.mu.Lock()
	limits := inspect.IncarnationLimits{Groups: len(ss.groups), GroupCredits: ss.groupCredits, GroupLimit: ss.groupLimit}
	recs := make(map[uint64]*inspect.Incarnation)
	get := func(inc uint64) *inspect.Incarnation {
		if id != 0 && inc != id {
			return nil
		}
		if recs[inc] == nil {
			recs[inc] = newIncarnationRecord(inc)
		}
		return recs[inc]
	}
	for inc, g := range ss.groups {
		if r := get(inc); r != nil {
			r.Group = groupRecord(g)
			if g.wire != nil {
				r.WireClientID = wireText(g.wire.clientID)
			}
		}
	}
	for sid, sess := range ss.sessions {
		if r := get(sess.clientID); r != nil {
			r.Sessions = append(r.Sessions, sessionText(sid))
		}
	}
	ss.mu.Unlock()
	out := make([]inspect.Incarnation, 0, len(recs))
	for _, r := range recs {
		sort.Strings(r.Sessions)
		out = append(out, *r)
	}
	return out, limits
}

func (sf *localStagingFile) snapshot(id InodeID, target stagingTarget) inspect.StagingEntry {
	out := inspect.StagingEntry{FileID: inodeText(id), DirID: inodeText(target.dirID), FileName: target.name}
	sf.hydrateMu.Lock()
	done := sf.hydrateDone
	sf.hydrateMu.Unlock()
	if done != nil {
		select {
		case <-done:
		default:
			out.Hydrating = true
		}
	}
	if !sf.mu.TryLock() {
		out.Locked = true
		return out
	}
	defer sf.mu.Unlock()
	meta := sf.meta
	out.Incarnation = incarnationText(meta.ClientID)
	out.Stateid = stateidText(meta.NFSStateID)
	out.OwnerHex = hex.EncodeToString([]byte(meta.OpenOwner))
	out.OwnerKnown = meta.OwnerKnown
	out.Size = sf.size
	if meta.BaseID != 0 {
		out.BaseID = inodeText(meta.BaseID)
	}
	out.BaseSize = meta.BaseSize
	out.DirtyRanges = len(sf.dirty)
	for _, r := range sf.dirty {
		out.DirtyBytes += r.end - r.start
	}
	out.HydratedRanges = len(sf.hydrated)
	out.Unlinked, out.Guarded, out.Retired = meta.Unlinked, meta.Guarded, meta.Retired
	out.ReadOnly, out.Exclusive = meta.ReadOnly, meta.Exclusive
	out.MetadataChanged = sf.metadataChanged
	out.CheckpointDirty = sf.checkpointDirty
	out.Removed = sf.removed
	return out
}

// Diagnostics: every entry as /v1/staging reports it, and the failed
// tombstones. Each file's lock is only tried, since it is held across disk
// I/O; a busy file reports its identity and target only.
func (s *LocalStagingStore) snapshot() ([]inspect.StagingEntry, []string) {
	type picked struct {
		id     InodeID
		entry  *localStagingEntry
		target stagingTarget
	}
	s.mu.Lock()
	var files []picked
	failed := []string{}
	for id, entry := range s.files {
		if entry.failed {
			failed = append(failed, inodeText(id))
			continue
		}
		files = append(files, picked{id, entry, entry.target})
	}
	s.mu.Unlock()
	records := make([]inspect.StagingEntry, 0, len(files))
	for _, p := range files {
		records = append(records, p.entry.file.snapshot(p.id, p.target))
	}
	return records, failed
}

func (s *LocalStagingStore) snapshotStateid(id StateID) stagingStateidSnapshot {
	s.mu.Lock()
	files := make(map[InodeID]*localStagingFile)
	for fileID, entry := range s.files {
		if !entry.failed {
			files[fileID] = entry.file
		}
	}
	s.mu.Unlock()
	var out stagingStateidSnapshot
	for fileID, file := range files {
		if !file.mu.TryLock() {
			out.Incomplete = true
			continue
		}
		matches := file.meta.NFSStateID == id
		clientID := file.meta.ClientID
		file.mu.Unlock()
		if matches {
			return stagingStateidSnapshot{FileID: fileID, ClientID: clientID, Found: true}
		}
	}
	return out
}

type stagingStateidSnapshot struct {
	FileID     InodeID
	ClientID   uint64
	Found      bool
	Incomplete bool
}

// stagingStateidSource is optional diagnostics support. It must not wait on
// a file lock held across I/O. A negative result is incomplete if any
// potentially matching entry could not be inspected.
type stagingStateidSource interface {
	snapshotStateid(StateID) stagingStateidSnapshot
}

func inodeTypeName(id InodeID) string {
	switch id.Type() {
	case InodeTypeDir:
		return "directory"
	case InodeTypeFile:
		return "file"
	case InodeTypeSymlink:
		return "symlink"
	}
	return fmt.Sprintf("type_%d", id.Type())
}

func (nc *nameCache) size() int {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return len(nc.entries)
}

// Diagnostics: the aliases and writer records as /v1/opens reports them.
func (vs *viewStore) snapshot() inspect.Views {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	out := inspect.Views{Aliases: []inspect.Alias{}, WriterEntries: len(vs.writers)}
	for key, alias := range vs.aliases {
		out.Aliases = append(out.Aliases, inspect.Alias{
			Incarnation: incarnationText(key.clientID),
			FileID:      inodeText(key.fileID),
			DataID:      inodeText(alias.dataID),
			Expires:     alias.expires,
		})
	}
	return out
}

// clientRecord copies a wire record. The caller holds sessions.mu.
func clientRecord(c *sessionClient) inspect.Client {
	slot := inspect.CreationSlot{Seq: c.csSeq, Cached: c.csCached, Busy: c.csBusy}
	if c.csCached {
		slot.Status = Nfsstat4Name(c.csStatus)
	}
	if c.csBusy {
		slot.BusySeq = c.csBusySeq
	}
	return inspect.Client{
		WireClientID: wireText(c.clientID),
		Incarnation:  incarnationText(c.incarnation),
		OwnerHex:     hex.EncodeToString([]byte(c.owner)),
		Verifier:     hex.EncodeToString(c.verifier[:]),
		Principal:    principalRecord(c.principal),
		CreatedAt:    c.createdAt,
		State:        clientState(c),
		HasConfirm:   c.hasConfirm,
		CreationSlot: slot,
		Confirming:   c.confirming != nil,
		Links:        inspect.IncarnationLinks(incarnationText(c.incarnation)),
	}
}

func clientState(c *sessionClient) string {
	switch {
	case c.retired:
		return inspect.ClientRetired
	case c.adopted:
		return inspect.ClientAdopted
	case c.confirmed:
		return inspect.ClientConfirmed
	}
	return inspect.ClientUnconfirmed
}

// wireClientState is the exact lookup of one wire clientid for
// /v1/lookup, under ss.mu.
func (ss *sessionStore) wireClientState(id uint64) (incarnation uint64, state string, ok bool) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	c := ss.clients[id]
	if c == nil {
		return 0, "", false
	}
	return c.incarnation, clientState(c), true
}

// snapshotClients copies the wire records matching f and their owners, under
// ss.mu. With an owner filter the owners are that one; with a wire or
// incarnation filter they are the owners of the matched records; otherwise
// every owner.
func (ss *sessionStore) snapshotClients(f inspect.ClientFilter) (records []inspect.Client, owners []inspect.Owner, wireLimit int) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	records, owners = []inspect.Client{}, []inspect.Owner{}
	matched := make(map[string]bool)
	for id, c := range ss.clients {
		if f.WireClientID != 0 && id != f.WireClientID ||
			f.Incarnation != 0 && c.incarnation != f.Incarnation ||
			f.OwnerHex != "" && hex.EncodeToString([]byte(c.owner)) != f.OwnerHex {
			continue
		}
		records = append(records, clientRecord(c))
		matched[c.owner] = true
	}
	byRecord := f.WireClientID != 0 || f.Incarnation != 0
	for owner, o := range ss.owners {
		ownerHex := hex.EncodeToString([]byte(owner))
		if f.OwnerHex != "" && ownerHex != f.OwnerHex || byRecord && !matched[owner] {
			continue
		}
		rec := inspect.Owner{OwnerHex: ownerHex}
		if o.confirmed != nil {
			rec.Confirmed = wireText(o.confirmed.clientID)
		}
		if o.unconfirmed != nil {
			rec.Unconfirmed = wireText(o.unconfirmed.clientID)
		}
		owners = append(owners, rec)
	}
	return records, owners, ss.wireLimit
}

// current reports the epoch and the counter of the last clientid issued.
// The epoch is 0 before start.
func (w *wireClientIDs) current() (epoch, last uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.epoch, w.last
}
