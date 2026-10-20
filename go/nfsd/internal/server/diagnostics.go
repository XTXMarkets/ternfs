// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/XTXMarkets/ternfs/go/nfsd/inspect"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/clientstore"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Server implements inspect.Source (../../inspect/README.md#snapshot-consistency).
// This file takes no lock and reads no guarded field: each store's snapshot
// methods, in diagnostics_snapshots.go and in the diagnostics.go of the
// clientstore, openstate and staging packages, take only their owner's locks
// and return copies, which this file composes, filters and sorts.
var _ inspect.Source = (*Server)(nil)
var _ inspect.StoreSource = (*Server)(nil)

func (s *Server) Envelope() inspect.Envelope {
	epoch, _ := s.sessions.wireIDs.current()
	return inspect.Envelope{
		Now:          s.clients.Now(),
		NfsdID:       s.clients.NfsdID,
		StateidEpoch: fmt.Sprintf("%08x", s.opens.Epoch),
		WireEpoch:    epoch,
		Mode:         inspect.ModeServing,
	}
}

func (s *Server) Status() (inspect.Status, error) {
	acc := s.sessions.accounting()
	epoch, last := s.sessions.wireIDs.current()
	return inspect.Status{
		Envelope:  s.Envelope(),
		StartedAt: s.startedAt,
		NFSAddr:   s.NFSAddr,
		Limits: inspect.Limits{
			MaxInFlightPerConnection: s.MaxInFlightPerConn,
			AllowMinorVersion2:       s.AllowMinorVersion2,
			LeaseTime:                clientstore.LeaseTime.String(),
		},
		Accounting: inspect.Accounting{
			CacheBytes:          acc.CacheBytes,
			CacheBuffers:        acc.CacheBuffers,
			CacheLimit:          acc.CacheLimit,
			WorkBytes:           acc.WorkBytes,
			WorkReplies:         acc.WorkReplies,
			WorkLimit:           acc.WorkLimit,
			Sessions:            acc.Sessions,
			Records:             acc.Records,
			RecordLimit:         acc.RecordLimit,
			PendingSessions:     acc.PendingSessions,
			Groups:              acc.Groups,
			GroupCredits:        acc.GroupCredits,
			GroupLimit:          acc.GroupLimit,
			ActiveRequests:      acc.ActiveRequests,
			ProvisionalRequests: acc.ProvisionalRequests,
		},
		Wire:               inspect.Wire{Epoch: epoch, LastCounter: last, Limit: acc.WireLimit},
		ForegroundReads:    s.foregroundReads.Load(),
		HydrationsInFlight: len(s.hydrationSlots),
		StrayCallbacks:     s.backchannel.strayReplies.Load(),
		ClientGC:           s.clientGCSnapshot(),
		Connections:        len(s.liveConns()),
	}, nil
}

func (s *Server) Connections(f inspect.ConnectionFilter) (inspect.Connections, error) {
	out := inspect.Connections{Envelope: s.Envelope(), Records: []inspect.Connection{}}
	for _, c := range s.liveConns() {
		if f.Session != nil && !c.carries(SessionID(*f.Session)) {
			continue
		}
		out.Records = append(out.Records, c.snapshot())
	}
	sort.Slice(out.Records, func(i, j int) bool {
		if !out.Records[i].ConnectedAt.Equal(out.Records[j].ConnectedAt) {
			return out.Records[i].ConnectedAt.Before(out.Records[j].ConnectedAt)
		}
		return out.Records[i].Remote < out.Records[j].Remote
	})
	return out, nil
}

func (s *Server) Clients(f inspect.ClientFilter) (inspect.Clients, error) {
	out := inspect.Clients{Envelope: s.Envelope()}
	out.Records, out.Owners, out.WireLimit = s.sessions.snapshotClients(f)
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].WireClientID < out.Records[j].WireClientID })
	sort.Slice(out.Owners, func(i, j int) bool { return out.Owners[i].OwnerHex < out.Owners[j].OwnerHex })
	return out, nil
}

// Incarnations joins the partial records of the session store, the client
// store, the open state and the confirmation registry. The join key is the
// incarnation's formatted id, which every partial record already carries.
func (s *Server) Incarnations(f inspect.IncarnationFilter) (inspect.Incarnations, error) {
	out := inspect.Incarnations{Envelope: s.Envelope(), Records: []inspect.Incarnation{}}
	recs := make(map[string]*inspect.Incarnation)
	get := func(id string) *inspect.Incarnation {
		if recs[id] == nil {
			recs[id] = &inspect.Incarnation{ID: id, Sessions: []string{}, Links: inspect.IncarnationLinks(id)}
		}
		return recs[id]
	}

	groups, limits := s.sessions.snapshotIncarnations(f.ID)
	out.Limits.Groups, out.Limits.GroupCredits, out.Limits.GroupLimit = limits.Groups, limits.GroupCredits, limits.GroupLimit
	for _, p := range groups {
		r := get(p.ID)
		r.Group, r.WireClientID, r.Sessions = p.Group, p.WireClientID, p.Sessions
	}

	leases, limits := s.clients.SnapshotIncarnations(vfs.InodeID(f.ID))
	out.Limits.Refreshers, out.Limits.RefresherLimit = limits.Refreshers, limits.RefresherLimit
	out.Limits.Orphans, out.Limits.OrphanLimit = limits.Orphans, limits.OrphanLimit
	for _, p := range leases {
		r := get(p.ID)
		r.Refresher, r.Cached = p.Refresher, p.Cached
	}

	for id, n := range s.opens.OpensByIncarnation(f.ID) {
		get(incarnationText(id)).Opens = n
	}

	for _, p := range s.confirmHolds(f.ID) {
		r := get(p.ID)
		r.Confirming, r.DestroyHolds, r.PurgePending = p.Confirming, p.DestroyHolds, p.PurgePending
	}

	for _, r := range recs {
		out.Records = append(out.Records, *r)
	}
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].ID < out.Records[j].ID })
	return out, nil
}

func (s *Server) Sessions(f inspect.SessionFilter) (inspect.Sessions, error) {
	out := inspect.Sessions{Envelope: s.Envelope(), Records: s.sessions.snapshotSessions(f)}
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].ID < out.Records[j].ID })
	return out, nil
}

func (s *Server) Opens(f inspect.OpenFilter) (inspect.Opens, error) {
	out := inspect.Opens{Envelope: s.Envelope()}
	out.Records, out.Expired = s.opens.Snapshot(f)
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].Stateid < out.Records[j].Stateid })
	sort.Slice(out.Expired, func(i, j int) bool { return out.Expired[i].Stateid < out.Expired[j].Stateid })
	out.Views = s.views.Snapshot()
	if f.Incarnation != 0 || f.File != 0 || f.Stateid != nil {
		incarnation, file := incarnationText(f.Incarnation), inodeText(vfs.InodeID(f.File))
		aliases := out.Views.Aliases[:0]
		for _, a := range out.Views.Aliases {
			if f.Incarnation != 0 && a.Incarnation != incarnation || f.File != 0 && a.FileID != file && a.DataID != file || f.Stateid != nil {
				continue
			}
			aliases = append(aliases, a)
		}
		out.Views.Aliases = aliases
	}
	sort.Slice(out.Views.Aliases, func(i, j int) bool {
		a, b := out.Views.Aliases[i], out.Views.Aliases[j]
		return a.Incarnation < b.Incarnation || a.Incarnation == b.Incarnation && a.FileID < b.FileID
	})
	out.Views.NameHints = s.names.size()
	return out, nil
}

func (s *Server) Confirmations() (inspect.Confirmations, error) {
	out := s.snapshotConfirmations()
	out.Envelope = s.Envelope()
	return out, nil
}

func (s *Server) Staging(f inspect.StagingFilter) (inspect.Staging, error) {
	out := inspect.Staging{Envelope: s.Envelope(), Records: []inspect.StagingEntry{}, Failed: []string{}}
	local, ok := s.stagingStore.(*staging.LocalStore)
	if !ok {
		return out, nil
	}
	out.Configured = true
	file, incarnation := inodeText(vfs.InodeID(f.File)), incarnationText(f.Incarnation)
	records, failed := local.Snapshot()
	for _, id := range failed {
		// A tombstone records no owner, so only the file filter applies.
		if f.File == 0 || id == file {
			out.Failed = append(out.Failed, id)
		}
	}
	for _, r := range records {
		if f.File != 0 && r.FileID != file {
			continue
		}
		// A locked file has no owner metadata yet. It remains a possible
		// match rather than disappearing from the client's diagnostics.
		ownerUnknown := r.Locked && r.Incarnation == ""
		if f.Incarnation != 0 && r.Incarnation != incarnation && !ownerUnknown {
			continue
		}
		out.Incomplete = out.Incomplete || r.Locked
		out.Records = append(out.Records, r)
	}
	sort.Slice(out.Records, func(i, j int) bool { return out.Records[i].FileID < out.Records[j].FileID })
	sort.Strings(out.Failed)
	return out, nil
}

func (s *Server) ResolveWireClientID(id uint64) (inspect.Resolution, error) {
	epoch, counter := uint32(id>>32), uint32(id)
	ours, last := s.sessions.wireIDs.current()
	r := inspect.Resolution{
		Epoch:               fmt.Sprint(epoch),
		IssuedByThisProcess: inspect.Bool(ours != 0 && epoch == ours),
		Links:               []string{"/v1/clients?wire=" + wireText(id)},
	}
	incarnation, state, known := s.sessions.wireClientState(id)
	switch {
	case known:
		r.Known = true
		r.State = state
		r.Incarnation = incarnationText(incarnation)
		r.Links = append(r.Links, inspect.IncarnationLinks(r.Incarnation)...)
		r.Note = "an EXCHANGE_ID clientid issued by this process and bound to the incarnation shown"
	case ours == 0:
		r.Note = "this process has not bound its NFS endpoint, so it has issued no wire clientids"
	case epoch != ours:
		r.Note = "a wire clientid from another nfsd process or an earlier epoch of this endpoint; it cannot be resolved here, and the client gets STALE_CLIENTID"
	case counter > last:
		r.Note = "carries this process's epoch but a counter it has not issued yet"
	default:
		r.Note = "issued by this process but no longer known: retired, destroyed or replaced"
	}
	return r, nil
}

func (s *Server) ResolveIncarnation(id uint64) (inspect.Resolution, error) {
	r := inspect.Resolution{Incarnation: incarnationText(id)}
	if !clientstore.IsDirectoryInodeID(vfs.InodeID(id)) {
		r.Note = "not a directory inode, so not an incarnation directory"
		return r, nil
	}
	wire, grouped := s.sessions.incarnationKnown(id)
	r.WireClientID = wire
	r.Known = grouped || s.clients.HasIncarnationState(vfs.InodeID(id))
	r.Links = inspect.IncarnationLinks(r.Incarnation)
	if r.Known {
		r.Note = "a client incarnation directory inode this process has state for"
	} else {
		r.Note = "a directory inode; if it is an incarnation, this process has no state for it and only the store report can describe it"
	}
	return r, nil
}

func (s *Server) ResolveInode(id uint64) (inspect.Resolution, error) {
	inode := vfs.InodeID(id)
	r := inspect.Resolution{InodeType: inodeTypeName(inode), Links: []string{
		"/v1/opens?file=" + inodeText(inode),
		"/v1/staging?file=" + inodeText(inode),
	}}
	opens := s.opens.FileOpens(inode)
	staged := s.stagingStore.Get(inode) != nil
	r.Known = opens > 0 || staged
	switch {
	case staged && opens > 0:
		r.Note = fmt.Sprintf("a filehandle is the published inode; %d open(s) use it and it is a staged writer's transient inode", opens)
	case staged:
		r.Note = "a staged writer's transient inode with no live open: recoverable staging"
	case opens > 0:
		r.Note = fmt.Sprintf("a filehandle is the published inode; %d open(s) refer to it", opens)
	default:
		r.Note = "a filehandle is the published inode; this process has no open or staging for it"
	}
	return r, nil
}

func (s *Server) ResolveStateid(id [12]byte) (inspect.Resolution, error) {
	epoch := binary.BigEndian.Uint32(id[:4])
	sid := nfs4.StateID(id)
	r := inspect.Resolution{
		Epoch:               fmt.Sprintf("%08x", epoch),
		IssuedByThisProcess: inspect.Bool(epoch == s.opens.Epoch),
		Links:               []string{"/v1/opens?stateid=" + stateidText(sid), "/v1/store?stateid=" + stateidText(sid)},
	}
	owner, live, expired := s.opens.StateidOwner(sid)
	switch {
	case live:
		r.Known = true
		r.State = "active"
		r.Incarnation = incarnationText(owner)
		r.Links = append(r.Links, inspect.IncarnationLinks(r.Incarnation)...)
		r.Note = "a live open stateid of the incarnation shown"
	case expired:
		r.Known = true
		r.State = "expired"
		r.Incarnation = incarnationText(owner)
		r.Links = append(r.Links, inspect.IncarnationLinks(r.Incarnation)...)
		r.Note = "an expired stateid tombstone: the owner gets EXPIRED, anyone else BAD_STATEID"
	default:
		// Staging can outlive the process that issued its stateid. Check it
		// independently of the epoch, without waiting on a disk-I/O lock.
		if source, ok := s.stagingStore.(staging.StateidSource); ok {
			staged := source.SnapshotStateid(sid)
			if staged.Found {
				r.Known = true
				r.State = "staging"
				r.Incarnation = incarnationText(staged.ClientID)
				r.Links = append(r.Links, inspect.IncarnationLinks(r.Incarnation)...)
				r.Note = fmt.Sprintf("no live open, but staging %s still names this stateid: a recoverable writer", inodeText(staged.FileID))
				return r, nil
			}
			r.Incomplete = staged.Incomplete
		} else if !s.stagingStore.ReadOnly() {
			r.Incomplete = true
		}
		switch {
		case r.Incomplete:
			r.Note = "no live open or expired tombstone; staging could not be fully inspected without waiting, so this stateid may still name a recoverable writer"
		case epoch != s.opens.Epoch:
			r.Note = "a stateid from another nfsd process or an earlier run; no live open, expired tombstone or recoverable staging here"
		default:
			r.Note = "carries this process's epoch but has no live open, expired tombstone or recoverable staging here"
		}
	}
	return r, nil
}

func (s *Server) ResolveSession(id [16]byte) (inspect.Resolution, error) {
	epoch := binary.BigEndian.Uint32(id[:4])
	incarnation := incarnationText(binary.BigEndian.Uint64(id[4:12]))
	sid := SessionID(id)
	r := inspect.Resolution{
		Epoch:               fmt.Sprintf("%08x", epoch),
		IssuedByThisProcess: inspect.Bool(epoch == s.opens.Epoch),
		Incarnation:         incarnation,
		Links:               append([]string{"/v1/sessions?id=" + sessionText(sid)}, inspect.IncarnationLinks(incarnation)...),
	}
	sess := s.sessions.lookup(sid)
	switch {
	case sess != nil:
		r.Known = true
		r.State = "live"
		if sess.destroyed.Load() {
			r.State = "destroyed"
		} else if sess.fatal.Load() {
			r.State = "fatal"
		}
		r.Note = "a session of this process; bytes 4 to 12 are its incarnation"
	case epoch != s.opens.Epoch:
		r.Note = "a session from another nfsd process or an earlier run; SEQUENCE gets BADSESSION and the client registers again"
	default:
		r.Note = "issued by this process but gone: destroyed, or its client expired or was replaced"
	}
	return r, nil
}

// Store serves the durable-store report through the API. Staging sidecars
// are read only when asked, from the server's own staging directory.
func (s *Server) Store(q inspect.StoreQuery) (any, error) {
	stagingDir := ""
	if local, ok := s.stagingStore.(*staging.LocalStore); ok {
		stagingDir = local.Dir()
	}
	return storeReport(s.clients, stagingDir, q)
}

// storeReport runs the durable-store reader for the API. It checks the
// query before any backend read, reads the backend and, when asked, the
// staging directory, and never writes either.
func storeReport(cs *clientstore.Store, stagingDir string, q inspect.StoreQuery) (any, error) {
	if q.Incarnation != 0 && !clientstore.IsDirectoryInodeID(vfs.InodeID(q.Incarnation)) {
		return nil, fmt.Errorf("%w: %s is not a directory inode", inspect.ErrBadRequest, incarnationText(q.Incarnation))
	}
	if !q.Staging {
		stagingDir = ""
	} else if stagingDir == "" {
		return nil, fmt.Errorf("%w: no staging directory is configured", inspect.ErrBadRequest)
	}
	return cs.Inspect(q, stagingDir)
}

// StoreOnlySource is the diagnostics source of a process that runs no NFS
// server: a read-only client store opened with clientstore.OpenReader, and
// a staging directory that is only ever read by the store report. It has
// no nfsd id and no epochs, since it issues nothing.
type StoreOnlySource struct {
	ClientStore *clientstore.Store
	StagingDir  string
}

var _ inspect.StoreSource = StoreOnlySource{}

func (s StoreOnlySource) Envelope() inspect.Envelope {
	return inspect.Envelope{Now: s.ClientStore.Now(), Mode: inspect.ModeStoreOnly}
}

func (s StoreOnlySource) Store(q inspect.StoreQuery) (any, error) {
	return storeReport(s.ClientStore, s.StagingDir, q)
}
