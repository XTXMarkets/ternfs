// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package inspect

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"
)

// ErrBadRequest marks a query the source could not accept; the response is
// 400 with the error text.
var ErrBadRequest = errors.New("bad request")

// StoreQuery selects the durable-store report. At most one of the four
// selectors is set; the HTTP layer enforces that. Staging asks for the
// server's own staging directory to be read and joined; the directory
// itself is never part of the query.
type StoreQuery struct {
	Identity     string
	IdentityHash string // lowercase hex, the identity directory's name
	Incarnation  uint64
	Stateid      *[12]byte
	Staging      bool
}

// StoreSource serves the durable-store report, whose shape the server
// defines; this package passes it through as JSON.
// Envelope identifies the process without a running server: in store-only
// mode it is what every response carries.
type StoreSource interface {
	Envelope() Envelope
	Store(StoreQuery) (any, error)
}

type storeResponse struct {
	Envelope
	Report any `json:"report"`
}

type resource struct {
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Filters     map[string]string `json:"filters,omitempty"`
	// Memory resources describe the running server and are unavailable in
	// store-only mode.
	Memory bool `json:"needs_running_server"`
	handle func(h *handler, q url.Values) (any, error)
}

type indexResponse struct {
	Envelope
	Resources []resource          `json:"resources"`
	Enums     map[string][]string `json:"enums"`
	Notes     []string            `json:"notes"`
}

type errorResponse struct {
	Envelope
	Error  string `json:"error"`
	Status int    `json:"status"`
}

type handler struct {
	src   Source
	store StoreSource
}

// Handler serves the diagnostics API. src describes the running server;
// store serves the durable-store report. Either may be nil: a nil src is
// the store-only mode, and a nil store means /v1/store is unavailable.
func Handler(src Source, store StoreSource) http.Handler {
	h := &handler{src: src, store: store}
	mux := http.NewServeMux()
	for _, r := range resources {
		r := r
		mux.HandleFunc(r.Path, func(w http.ResponseWriter, req *http.Request) {
			if r.Path == "/" && req.URL.Path != "/" {
				h.writeError(w, http.StatusNotFound, fmt.Errorf("no resource at %s; GET / lists them", req.URL.Path))
				return
			}
			if req.Method != http.MethodGet && req.Method != http.MethodHead {
				h.writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("%s not allowed; the API is read-only", req.Method))
				return
			}
			if r.Memory && h.src == nil {
				h.writeError(w, http.StatusServiceUnavailable, ErrUnavailable)
				return
			}
			if r.handle == nil {
				h.writeJSON(w, http.StatusOK, h.index())
				return
			}
			out, err := r.handle(h, req.URL.Query())
			if err != nil {
				h.writeError(w, statusFor(err), err)
				return
			}
			h.writeJSON(w, http.StatusOK, out)
		})
	}
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

func (h *handler) index() indexResponse {
	out := indexResponse{Envelope: h.envelope(), Resources: make([]resource, 0, len(resources)), Enums: enums, Notes: notes}
	for _, r := range resources {
		r.handle = nil
		out.Resources = append(out.Resources, r)
	}
	return out
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

// envelope is what every response carries. Without a running server it is
// the store's; without either it still names the mode, so a reader knows
// why memory resources are absent.
func (h *handler) envelope() Envelope {
	switch {
	case h.src != nil:
		return h.src.Envelope()
	case h.store != nil:
		return h.store.Envelope()
	}
	return Envelope{Mode: ModeStoreOnly}
}

func (h *handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (h *handler) writeError(w http.ResponseWriter, status int, err error) {
	h.writeJSON(w, status, errorResponse{Envelope: h.envelope(), Error: err.Error(), Status: status})
}

// Query helpers. An id parameter is parsed for the one kind the filter
// names. An omitted parameter is no filter; a present one, even an empty
// string, is parsed and fails like any other bad value; a zero eight-byte
// id is rejected rather than treated as absent.

func queryID(q url.Values, name, kind string) ([]byte, error) {
	if !q.Has(name) {
		return nil, nil
	}
	raw, err := ParseID(kind, q.Get(name))
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrBadRequest, name, err)
	}
	return raw, nil
}

func queryScalar(q url.Values, name, kind string) (uint64, error) {
	if !q.Has(name) {
		return 0, nil
	}
	v, err := ParseScalar(kind, q.Get(name))
	if err != nil {
		return 0, fmt.Errorf("%w: %s: %v", ErrBadRequest, name, err)
	}
	return v, nil
}

func queryBool(q url.Values, name string, def bool) bool {
	switch strings.ToLower(q.Get(name)) {
	case "":
		return def
	case "0", "false", "no", "off":
		return false
	}
	return true
}

var resources = []resource{
	{
		Path:        "/",
		Description: "This index: every resource, its filters, and the meaning of each enum value. Read it first.",
		// handle is nil: the index is served by Handler itself, since a
		// handler that lists resources cannot be part of their initializer.
	},
	{
		Path:        "/v1/status",
		Description: "Instance summary: the three process identifiers, limits, reply-cache and request accounting, hydration and GC activity.",
		Memory:      true,
		handle: func(h *handler, q url.Values) (any, error) {
			return h.src.Status()
		},
	},
	{
		Path:        "/v1/lookup",
		Description: "Resolve one typed id: whether this process issued it, whether it has state for it, and links to the resources describing it.",
		Filters:     map[string]string{"id": "required; one of " + strings.Join(Kinds, ", ") + ", as kind:hex"},
		Memory:      true,
		handle: func(h *handler, q url.Values) (any, error) {
			if q.Get("id") == "" {
				return nil, fmt.Errorf("%w: id is required", ErrBadRequest)
			}
			out, err := Resolve(h.src, q.Get("id"))
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
			}
			return out, nil
		},
	},
	{
		Path:        "/v1/clients",
		Description: "EXCHANGE_ID wire clientids this process issued, each bound to a durable incarnation, with the CREATE_SESSION replay slot.",
		Filters: map[string]string{
			"wire":        "one wire-clientid",
			"incarnation": "records bound to this incarnation",
			"owner":       "records of this EXCHANGE_ID owner, hex",
		},
		Memory: true,
		handle: func(h *handler, q url.Values) (any, error) {
			var f ClientFilter
			var err error
			if f.WireClientID, err = queryScalar(q, "wire", KindWireClientID); err != nil {
				return nil, err
			}
			if f.Incarnation, err = queryScalar(q, "incarnation", KindIncarnation); err != nil {
				return nil, err
			}
			f.OwnerHex = strings.ToLower(q.Get("owner"))
			return h.src.Clients(f)
		},
	},
	{
		Path:        "/v1/incarnations",
		Description: "Per durable client incarnation: session group and logical lease gate, durable lease refresher record, cached store location, confirmation protection and destruction holds, sessions and opens.",
		Filters:     map[string]string{"id": "one incarnation"},
		Memory:      true,
		handle: func(h *handler, q url.Values) (any, error) {
			id, err := queryScalar(q, "id", KindIncarnation)
			if err != nil {
				return nil, err
			}
			return h.src.Incarnations(IncarnationFilter{ID: id})
		},
	},
	{
		Path:        "/v1/sessions",
		Description: "NFSv4.1 sessions: channel limits, SEQUENCE status flags, backchannel sender and connections, and each slot. A slot a request holds right now is reported as locked.",
		Filters: map[string]string{
			"id":          "one session",
			"incarnation": "sessions of this incarnation",
			"slots":       "0 omits the per-slot detail",
		},
		Memory: true,
		handle: func(h *handler, q url.Values) (any, error) {
			var f SessionFilter
			raw, err := queryID(q, "id", KindSession)
			if err != nil {
				return nil, err
			}
			if raw != nil {
				id := [16]byte(raw)
				f.ID = &id
			}
			if f.Incarnation, err = queryScalar(q, "incarnation", KindIncarnation); err != nil {
				return nil, err
			}
			f.Slots = queryBool(q, "slots", true)
			return h.src.Sessions(f)
		},
	},
	{
		Path:        "/v1/connections",
		Description: "Live client TCP connections: requests in flight, the sessions each may carry requests for, the sessions whose callbacks it carries, and its callback path state.",
		Filters:     map[string]string{"session": "connections associated with this session, in either direction"},
		Memory:      true,
		handle: func(h *handler, q url.Values) (any, error) {
			var f ConnectionFilter
			raw, err := queryID(q, "session", KindSession)
			if err != nil {
				return nil, err
			}
			if raw != nil {
				id := [16]byte(raw)
				f.Session = &id
			}
			return h.src.Connections(f)
		},
	},
	{
		Path:        "/v1/opens",
		Description: "Process-local open state: each open stateid with its file, data inode, owner and access; expired stateid tombstones; and the client views that decide what a handle shows.",
		Filters: map[string]string{
			"stateid":     "one stateid",
			"incarnation": "opens of this incarnation",
			"file":        "opens whose filehandle or data inode is this inode",
		},
		Memory: true,
		handle: func(h *handler, q url.Values) (any, error) {
			var f OpenFilter
			raw, err := queryID(q, "stateid", KindStateid)
			if err != nil {
				return nil, err
			}
			if raw != nil {
				id := [12]byte(raw)
				f.Stateid = &id
			}
			if f.Incarnation, err = queryScalar(q, "incarnation", KindIncarnation); err != nil {
				return nil, err
			}
			if f.File, err = queryScalar(q, "file", KindInode); err != nil {
				return nil, err
			}
			return h.src.Opens(f)
		},
	},
	{
		Path:        "/v1/confirmations",
		Description: "Client confirmations in progress or unresolved, with the evidence of their writes, and the destruction holds and purge hints around them. Normally empty.",
		Memory:      true,
		handle: func(h *handler, q url.Values) (any, error) {
			return h.src.Confirmations()
		},
	},
	{
		Path:        "/v1/staging",
		Description: "The local staging store in memory: every staged writer with its target, owner, size, dirty and hydrated ranges and flags, plus the failed tombstones. Locked entries remain possible matches for an incarnation filter and make the response incomplete. A file filter applies to the tombstones too; an incarnation filter does not, since a tombstone records no owner. /v1/store?staging=1 reads the same directory from disk.",
		Filters: map[string]string{
			"file":        "the staging (transient) inode",
			"incarnation": "writers of this incarnation",
		},
		Memory: true,
		handle: func(h *handler, q url.Values) (any, error) {
			var f StagingFilter
			var err error
			if f.File, err = queryScalar(q, "file", KindInode); err != nil {
				return nil, err
			}
			if f.Incarnation, err = queryScalar(q, "incarnation", KindIncarnation); err != nil {
				return nil, err
			}
			return h.src.Staging(f)
		},
	},
	{
		Path:        "/v1/store",
		Description: "The durable client store report: every identity, its incarnations, leases, open markers and, with staging=1, the staging sidecars joined to them. Reads the TernFS backend, so it is slower than the other resources and works without a running server.",
		Filters: map[string]string{
			"identity":      "the raw EXCHANGE_ID owner string",
			"identity-hash": "one identity-hash",
			"incarnation":   "one incarnation",
			"stateid":       "one stateid; selects the incarnation holding its open marker or staging",
			"staging":       "1 also reads the server's own staging sidecars",
		},
		handle: func(h *handler, q url.Values) (any, error) {
			if h.store == nil {
				return nil, fmt.Errorf("%w: no client store is open", ErrUnavailable)
			}
			sq := StoreQuery{Identity: q.Get("identity"), Staging: queryBool(q, "staging", false)}
			selectors := 0
			if sq.Identity != "" {
				selectors++
			}
			hash, err := queryID(q, "identity-hash", KindIdentityHash)
			if err != nil {
				return nil, err
			}
			if hash != nil {
				sq.IdentityHash = hex.EncodeToString(hash)
				selectors++
			}
			if sq.Incarnation, err = queryScalar(q, "incarnation", KindIncarnation); err != nil {
				return nil, err
			}
			if sq.Incarnation != 0 {
				selectors++
			}
			stateid, err := queryID(q, "stateid", KindStateid)
			if err != nil {
				return nil, err
			}
			if stateid != nil {
				id := [12]byte(stateid)
				sq.Stateid = &id
				selectors++
			}
			if selectors > 1 {
				return nil, fmt.Errorf("%w: use at most one of identity, identity-hash, incarnation and stateid", ErrBadRequest)
			}
			report, err := h.store.Store(sq)
			if err != nil {
				return nil, err
			}
			return storeResponse{Envelope: h.envelope(), Report: report}, nil
		},
	},
}

// enums documents every state word the API uses, for the index.
var enums = map[string][]string{
	"mode": {
		ModeServing + ": an NFS server is running in this process",
		ModeStoreOnly + ": only the durable store readers are running; memory resources return 503",
	},
	"client.state": {
		ClientUnconfirmed + ": EXCHANGE_ID done, no CREATE_SESSION yet",
		ClientConfirmed + ": the first CREATE_SESSION confirmed the incarnation",
		ClientAdopted + ": the incarnation was already durably confirmed when this process first saw it",
		ClientRetired + ": dropped; the clientid gets STALE_CLIENTID",
	},
	"group.state": {
		GroupOpen + ": accepting requests",
		GroupClosing + ": a newer incarnation of the same identity is being confirmed; new requests wait",
		GroupReplaced + ": replaced; its sessions are gone for good",
	},
	"group.lease_gate": {
		GateLive + ": the logical lease is live or being held",
		GateChecking + ": the expiry decision is reading the durable lease",
		GateExpired + ": this process has expired the client",
	},
	"group.expired_marker_phase": {
		MarkerNone + ": no expired marker work",
		MarkerDeciding + ": deciding whether to publish the expired marker",
		MarkerSubmitted + ": the marker write was submitted",
	},
	"lease_refresher.state": {
		LeaseLive + ": the durable lease can be refreshed",
		LeaseStale + ": the confirmed pointer no longer names this incarnation",
		LeaseFleetExpired + ": an expired marker was published or observed",
	},
	"lease_refresher.temp_obligations[].phase": {
		OrphanConstructing + ": ConstructFile in flight",
		OrphanLinking + ": LinkFile in flight; its outcome may be unknown",
		OrphanLinked + ": linked; waiting to be renamed over the lease or removed",
		OrphanUnresolved + ": a call never returned; the edge must be found before the credit is released",
	},
	"connection.path, backchannel.connections[].path": {
		PathProbing + ": CB_NULL sent, not yet answered; counts as up for status flags",
		PathUp + ": a callback reply was validated on this connection",
		PathUnresponsive + ": the probe timed out or the connection closed",
	},
	"confirmation.attempt.result": {
		ConfirmUnresolved + ": a fence or the read-back failed; nothing durable may be destroyed yet",
		ConfirmCommitted + ": the confirmed pointer names the new incarnation",
		ConfirmRolledBack + ": the confirmed write never applied and the predecessor is unchanged",
		ConfirmConservative + ": neither committed nor provably unchanged; nothing durable is deleted",
	},
	"confirmation.attempt.writes[].outcome": {
		WriteNever + ": not attempted, failed before sending, or fenced",
		WriteApplied + ": the rename succeeded",
		WriteMaybe + ": fenced as edge gone; it may have applied",
		WriteUnsettled + ": the fence has not succeeded yet",
	},
	"confirmation.attempt.writes[].fence": {
		FenceNone + ": not needed or not tried",
		FenceNotApplied + ": the temp edge was removed, so the write can never apply",
		FenceEdgeGone + ": the temp edge was already gone; the write may have applied",
		FenceFailed + ": the removal failed or never returned; fence_error says why",
	},
	"id kinds": {
		KindWireClientID + ":<16 hex>: the clientid EXCHANGE_ID returned, epoch<<32 | counter, process-local",
		KindIncarnation + ":<16 hex>: a client incarnation directory inode, the durable client id",
		KindInode + ":<16 hex>: any other inode: a filehandle, a staging data inode, a directory, a base",
		KindStateid + ":<24 hex>: the stateid other field; the first 4 bytes are the stateid epoch",
		KindSession + ":<32 hex>: a session id: stateid epoch, incarnation, counter",
		KindIdentityHash + ":<64 hex>: SHA-256 of the EXCHANGE_ID owner, naming the identity directory",
	},
}

var notes = []string{
	"Every response carries now, nfsd_id, stateid_epoch, wire_epoch and mode. nfsd_id names this process's lease and confirming slots in the client store; stateid_epoch is the first four bytes of every stateid and session id it issued; wire_epoch is the high 32 bits of every wire clientid.",
	"Every id is a typed kind:hex string, as listed under id kinds, and a filter or lookup takes exactly the kind it names: a value copied from any response, or from the session-created log line, pastes back unchanged. Uppercase hex is accepted. A zero wire-clientid, incarnation or inode is rejected rather than treated as no filter.",
	"Snapshots are taken one lock at a time, so counts across resources may not agree exactly. A slot, staging file or confirmation attempt reported locked was held by a request or a backend call at that instant.",
	"A stateid lookup with incomplete=true could not rule out recoverable staging without waiting. Staging responses are incomplete when they include locked entries; those entries have unknown owner metadata and remain possible matches for an incarnation filter. Retry for a complete result.",
	"The API is read-only and has no authentication. Bind it to a loopback or management address.",
}
