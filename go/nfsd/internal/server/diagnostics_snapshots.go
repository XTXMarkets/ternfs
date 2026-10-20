// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// The diagnostics snapshots of the server's own state: sessions, groups,
// connections and confirmations, kept out of the files that implement them.
// The client store, open state and staging store snapshot themselves, in
// the diagnostics.go of their packages, on the same rules. The owner-facing
// entry points are methods of the type that owns the state they read: each
// acquires that type's own locks, in the order the type's own writers use,
// copies into the API's types and returns, so the aggregator in
// diagnostics.go holds no lock and reads no guarded field. The private
// record builders (clientRecord, groupRecord, confirmation.snapshot and the
// enum-name helpers) run under a lock their owner method has already taken.
// Ordinary registry and record mutexes are acquired normally. Three locks
// are only tried, because a request path tries them or they are held across
// I/O: session slots, staging files and confirmation attempts. A snapshot
// never waits for their holders; it reports the entry as locked instead
// (../../inspect/README.md#snapshot-consistency).

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
	{nfs4.SEQ4_STATUS_CB_PATH_DOWN, "cb_path_down"},
	{nfs4.SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRING, "cb_gss_contexts_expiring"},
	{nfs4.SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRED, "cb_gss_contexts_expired"},
	{nfs4.SEQ4_STATUS_EXPIRED_ALL_STATE_REVOKED, "expired_all_state_revoked"},
	{nfs4.SEQ4_STATUS_EXPIRED_SOME_STATE_REVOKED, "expired_some_state_revoked"},
	{nfs4.SEQ4_STATUS_ADMIN_STATE_REVOKED, "admin_state_revoked"},
	{nfs4.SEQ4_STATUS_RECALLABLE_STATE_REVOKED, "recallable_state_revoked"},
	{nfs4.SEQ4_STATUS_LEASE_MOVED, "lease_moved"},
	{nfs4.SEQ4_STATUS_RESTART_RECLAIM_NEEDED, "restart_reclaim_needed"},
	{nfs4.SEQ4_STATUS_CB_PATH_DOWN_SESSION, "cb_path_down_session"},
	{nfs4.SEQ4_STATUS_BACKCHANNEL_FAULT, "backchannel_fault"},
	{nfs4.SEQ4_STATUS_DEVID_CHANGED, "devid_changed"},
	{nfs4.SEQ4_STATUS_DEVID_DELETED, "devid_deleted"},
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
		Credential:       clientstore.AuthFlavorName(cb.sec.flavor),
		NextSeq:          cb.seq,
		Fault:            cb.fault,
		RecallPending:    cb.recall,
		RecallTarget:     cb.target,
		InFlight:         cb.inFlight,
		LastRecallStatus: nfs4.Nfsstat4Name(cb.lastRecallStatus),
		Running:          cb.running,
		Cancelled:        cb.cancelled,
		Connections:      []inspect.BackConn{},
	}, append([]*rpcConnection(nil), cb.conns...)
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
			recs[inc] = inspect.NewIncarnation(inc)
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
	var attempts []*clientstore.ConfirmAttempt
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
			out.Records[i].Attempt = a.Snapshot()
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
		Restored:   slot.restored,
		Intent:     slot.hasIntent,
	}
	if slot.occupied {
		out.User = &inspect.User{Flavor: clientstore.AuthFlavorName(slot.user.flavor), UID: slot.user.uid, GID: slot.user.gid}
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
		Persisted:     sess.file.onDisk.Load(),
		Restored:      sess.restored,
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
			recs[inc] = inspect.NewIncarnation(inc)
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

func inodeTypeName(id vfs.InodeID) string {
	switch id.Type() {
	case vfs.InodeTypeDir:
		return "directory"
	case vfs.InodeTypeFile:
		return "file"
	case vfs.InodeTypeSymlink:
		return "symlink"
	}
	return fmt.Sprintf("type_%d", id.Type())
}

func (nc *nameCache) size() int {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	return len(nc.entries)
}

// clientRecord copies a wire record. The caller holds sessions.mu.
func clientRecord(c *sessionClient) inspect.Client {
	slot := inspect.CreationSlot{Seq: c.csSeq, Cached: c.csCached, Busy: c.csBusy}
	if c.csCached {
		slot.Status = nfs4.Nfsstat4Name(c.csStatus)
	}
	if c.csBusy {
		slot.BusySeq = c.csBusySeq
	}
	return inspect.Client{
		WireClientID: wireText(c.clientID),
		Incarnation:  incarnationText(c.incarnation),
		OwnerHex:     hex.EncodeToString([]byte(c.owner)),
		Verifier:     hex.EncodeToString(c.verifier[:]),
		Principal:    clientstore.PrincipalRecord(c.principal),
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
