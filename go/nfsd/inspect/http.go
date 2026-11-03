// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package inspect

import (
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

type resource struct {
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Filters     map[string]string `json:"filters,omitempty"`
	handle      func(h *handler, q url.Values) (any, error)
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
	src Source
}

// Handler serves the diagnostics API of the running server src.
func Handler(src Source) http.Handler {
	h := &handler{src: src}
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
	out := indexResponse{Envelope: h.src.Envelope(), Resources: make([]resource, 0, len(resources)), Enums: enums, Notes: notes}
	for _, r := range resources {
		r.handle = nil
		out.Resources = append(out.Resources, r)
	}
	return out
}

func statusFor(err error) int {
	if errors.Is(err, ErrBadRequest) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
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
	h.writeJSON(w, status, errorResponse{Envelope: h.src.Envelope(), Error: err.Error(), Status: status})
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
		Description: "Instance summary: the three process identifiers, limits, reply-cache and request accounting, and hydration activity.",
		handle: func(h *handler, q url.Values) (any, error) {
			return h.src.Status()
		},
	},
	{
		Path:        "/v1/lookup",
		Description: "Resolve one typed id: whether this process issued it, whether it has state for it, and links to the resources describing it.",
		Filters:     map[string]string{"id": "required; one of " + strings.Join(Kinds, ", ") + ", as kind:hex"},
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
		Description: "The client records in the staging directory, one per client identity, and the EXCHANGE_ID wire clientids this process issued, each bound to an incarnation, with the CREATE_SESSION replay slot.",
		Filters: map[string]string{
			"wire":        "one wire-clientid, and the client record of its incarnation",
			"incarnation": "wire clientids bound to this incarnation, and the client record that confirms or is purging it",
			"owner":       "wire clientids and the client record of this EXCHANGE_ID owner, hex",
		},
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
		Description: "Per client incarnation: session group and logical lease gate, sessions and opens.",
		Filters:     map[string]string{"id": "one incarnation"},
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
		Path:        "/v1/staging",
		Description: "The local staging store in memory: every staged writer with its target, owner, size, dirty and hydrated ranges and flags, plus the failed tombstones. Locked entries remain possible matches for an incarnation filter and make the response incomplete. A file filter applies to the tombstones too; an incarnation filter does not, since a tombstone records no owner.",
		Filters: map[string]string{
			"file":        "the staging (transient) inode",
			"incarnation": "writers of this incarnation",
		},
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
}

// enums documents every state word the API uses, for the index.
var enums = map[string][]string{
	"client.state": {
		ClientUnconfirmed + ": EXCHANGE_ID done, no CREATE_SESSION yet",
		ClientConfirmed + ": the first CREATE_SESSION confirmed the incarnation",
		ClientAdopted + ": EXCHANGE_ID found the incarnation already confirmed by its client record",
		ClientRetired + ": dropped; the clientid gets STALE_CLIENTID",
	},
	"group.state": {
		GroupOpen + ": accepting requests",
		GroupClosing + ": a newer incarnation of the same identity is being confirmed; new requests wait",
		GroupReplaced + ": replaced; its sessions are gone for good",
	},
	"group.lease_gate": {
		GateLive + ": the logical lease is live or being held",
		GateChecking + ": the expiry decision is writing the client record expired",
		GateExpired + ": this process has expired the client",
	},
	"connection.path, backchannel.connections[].path": {
		PathProbing + ": CB_NULL sent, not yet answered; counts as up for status flags",
		PathUp + ": a callback reply was validated on this connection",
		PathUnresponsive + ": the probe timed out or the connection closed",
	},
	"id kinds": {
		KindWireClientID + ":<16 hex>: the clientid EXCHANGE_ID returned, epoch<<32 | counter, process-local",
		KindIncarnation + ":<16 hex>: a client incarnation, chosen at random when it registers and kept in its client record once confirmed",
		KindInode + ":<16 hex>: any other inode: a filehandle, a staging data inode, a directory, a base",
		KindStateid + ":<24 hex>: the stateid other field; the first 4 bytes are the stateid epoch",
		KindSession + ":<32 hex>: a session id: stateid epoch, incarnation, counter",
		KindIdentityHash + ":<64 hex>: SHA-256 of the EXCHANGE_ID owner, naming its client record file",
	},
}

var notes = []string{
	"Every response carries now, server_id, stateid_epoch and wire_epoch. server_id is the staging directory's server-id, the server owner clients see; stateid_epoch is the first four bytes of every stateid and session id it issued; wire_epoch is the high 32 bits of every wire clientid.",
	"Every id is a typed kind:hex string, as listed under id kinds, and a filter or lookup takes exactly the kind it names: a value copied from any response, or from the session-created log line, pastes back unchanged. Uppercase hex is accepted. A zero wire-clientid, incarnation or inode is rejected rather than treated as no filter.",
	"Snapshots are taken one lock at a time, so counts across resources may not agree exactly. A slot or staging file reported locked was held by a request or a disk operation at that instant.",
	"A stateid lookup with incomplete=true could not rule out recoverable staging without waiting. Staging responses are incomplete when they include locked entries; those entries have unknown owner metadata and remain possible matches for an incarnation filter. Retry for a complete result.",
	"The API is read-only and has no authentication. Bind it to a loopback or management address.",
}
