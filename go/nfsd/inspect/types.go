// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Package inspect is nfsd's diagnostics API: the JSON types a running
// server reports about its in-memory NFSv4.1 state, and the HTTP handlers
// that serve them. The server implements Source; this package never reaches
// into server internals.
//
// Every value is rendered for a reader who has a packet capture or a log
// line and wants to know what the server thinks: ids are typed kind:hex
// strings (ids.go), enums are lowercase words, and times are RFC 3339.
// Field names are stable.
package inspect

import (
	"errors"
	"time"
)

// ErrUnavailable is returned by a Source that has no running server, which
// is the store-only mode: the durable store can be read but there is no
// in-memory state.
var ErrUnavailable = errors.New("not available: no NFS server is running in this process")

// Mode values for Envelope.Mode.
const (
	ModeServing   = "serving"
	ModeStoreOnly = "store-only"
)

// Envelope is carried by every response. The three epochs identify the
// process: nfsd_id names its lease and confirming slots in the client
// store, stateid_epoch is the first four bytes of every stateid and session
// id it issued, and wire_epoch is the high 32 bits of every wire clientid.
// wire_epoch is zero until the NFS endpoint is bound.
type Envelope struct {
	Now          time.Time `json:"now"`
	NfsdID       string    `json:"nfsd_id"`
	StateidEpoch string    `json:"stateid_epoch"`
	WireEpoch    uint32    `json:"wire_epoch"`
	Mode         string    `json:"mode"`
}

// Source is what the server exposes to the API. Each method takes a
// consistent snapshot under the relevant locks and returns copies; none of
// them blocks on a lock that is held across I/O.
type Source interface {
	Envelope() Envelope
	Status() (Status, error)
	Clients(ClientFilter) (Clients, error)
	Incarnations(IncarnationFilter) (Incarnations, error)
	Sessions(SessionFilter) (Sessions, error)
	Connections(ConnectionFilter) (Connections, error)
	Opens(OpenFilter) (Opens, error)
	Confirmations() (Confirmations, error)
	Staging(StagingFilter) (Staging, error)

	// Resolvers for /v1/lookup, one per kind the server has state for. Each
	// says whether this process knows the id. Resolve fills Envelope, ID
	// and Kind.
	ResolveWireClientID(id uint64) (Resolution, error)
	ResolveIncarnation(id uint64) (Resolution, error)
	ResolveInode(id uint64) (Resolution, error)
	ResolveStateid(other [12]byte) (Resolution, error)
	ResolveSession(id [16]byte) (Resolution, error)
}

// Status is the instance summary.
type Status struct {
	Envelope
	StartedAt          time.Time  `json:"started_at"`
	NFSAddr            string     `json:"nfs_addr,omitempty"`
	Limits             Limits     `json:"limits"`
	Accounting         Accounting `json:"accounting"`
	Wire               Wire       `json:"wire"`
	ForegroundReads    int64      `json:"foreground_reads"`
	HydrationsInFlight int        `json:"hydrations_in_flight"`
	StrayCallbacks     uint64     `json:"stray_callback_replies"`
	ClientGC           ClientGC   `json:"client_gc"`
	Connections        int        `json:"connections"`
}

type Limits struct {
	MaxInFlightPerConnection int    `json:"max_in_flight_per_connection"`
	AllowMinorVersion2       bool   `json:"allow_minor_version_2"`
	LeaseTime                string `json:"lease_time"`
}

// Accounting is the session store's reply-cache and request figures.
type Accounting struct {
	CacheBytes          int `json:"cache_bytes"`
	CacheBuffers        int `json:"cache_buffers"`
	CacheLimit          int `json:"cache_limit"`
	WorkBytes           int `json:"work_bytes"`
	WorkReplies         int `json:"work_replies"`
	WorkLimit           int `json:"work_limit"`
	Sessions            int `json:"sessions"`
	Records             int `json:"records"`
	RecordLimit         int `json:"record_limit"`
	PendingSessions     int `json:"pending_sessions"`
	Groups              int `json:"groups"`
	GroupCredits        int `json:"group_credits"`
	GroupLimit          int `json:"group_limit"`
	ActiveRequests      int `json:"active_requests"`
	ProvisionalRequests int `json:"provisional_requests"`
}

// Wire is the wire clientid issuer: clientids are epoch<<32 | counter.
type Wire struct {
	Epoch       uint32 `json:"epoch"`
	LastCounter uint32 `json:"last_counter"`
	Limit       int    `json:"record_limit"`
}

type ClientGC struct {
	Pending int  `json:"pending"`
	Recheck int  `json:"recheck"`
	Running bool `json:"running"`
}

// ClientFilter selects wire client records. Zero values select everything;
// the HTTP layer rejects an explicit zero id.
type ClientFilter struct {
	WireClientID uint64
	Incarnation  uint64
	OwnerHex     string
}

// Clients lists the EXCHANGE_ID records this process has issued.
type Clients struct {
	Envelope
	Records   []Client `json:"records"`
	Owners    []Owner  `json:"owners"`
	WireLimit int      `json:"wire_limit"`
}

// Client is one wire clientid: the value EXCHANGE_ID returned. It is
// process-local and never stored; incarnation is the durable client it is
// bound to, which owns sessions and state.
type Client struct {
	WireClientID string    `json:"wire_clientid"`
	Incarnation  string    `json:"incarnation"`
	OwnerHex     string    `json:"owner_hex"`
	Verifier     string    `json:"verifier"`
	Principal    Principal `json:"principal"`
	CreatedAt    time.Time `json:"created_at"`
	// State: unconfirmed until the first CREATE_SESSION; confirmed after
	// it; adopted when the incarnation was already durably confirmed when
	// this process first saw it; retired once dropped.
	State        string       `json:"state"`
	HasConfirm   bool         `json:"has_confirm"`
	CreationSlot CreationSlot `json:"creation_slot"`
	Confirming   bool         `json:"confirming"`
	Links        []string     `json:"links"`
}

// Client states.
const (
	ClientUnconfirmed = "unconfirmed"
	ClientConfirmed   = "confirmed"
	ClientAdopted     = "adopted"
	ClientRetired     = "retired"
)

type Principal struct {
	Flavor  string `json:"flavor"`
	Text    string `json:"text"`
	BodyHex string `json:"body_hex,omitempty"`
}

// CreationSlot is the CREATE_SESSION replay slot (RFC 8881 section 18.36.4).
type CreationSlot struct {
	Seq     uint32 `json:"seq"`
	Cached  bool   `json:"cached"`
	Status  string `json:"status,omitempty"`
	Busy    bool   `json:"busy"`
	BusySeq uint32 `json:"busy_seq,omitempty"`
}

// Owner is an EXCHANGE_ID owner's wire records: the confirmed one and at
// most one waiting for its first CREATE_SESSION.
type Owner struct {
	OwnerHex    string `json:"owner_hex"`
	Confirmed   string `json:"confirmed,omitempty"`
	Unconfirmed string `json:"unconfirmed,omitempty"`
}

// IncarnationFilter selects one incarnation; zero selects all.
type IncarnationFilter struct {
	ID uint64
}

// Incarnations joins everything this process keeps per durable client
// incarnation: the session group with its logical lease gate, the durable
// lease refresher record, the cached store location, confirmation
// protection and destruction holds.
type Incarnations struct {
	Envelope
	Records []Incarnation     `json:"records"`
	Limits  IncarnationLimits `json:"limits"`
}

type IncarnationLimits struct {
	Refreshers     int `json:"refreshers"`
	RefresherLimit int `json:"refresher_limit"`
	Orphans        int `json:"orphans"`
	OrphanLimit    int `json:"orphan_limit"`
	Groups         int `json:"groups"`
	GroupCredits   int `json:"group_credits"`
	GroupLimit     int `json:"group_limit"`
}

type Incarnation struct {
	ID           string        `json:"incarnation"`
	WireClientID string        `json:"wire_clientid,omitempty"`
	Group        *Group        `json:"group,omitempty"`
	Refresher    *Refresher    `json:"lease_refresher,omitempty"`
	Cached       *CachedClient `json:"cached_store_location,omitempty"`
	Sessions     []string      `json:"sessions"`
	Opens        int           `json:"opens"`
	Confirming   bool          `json:"confirming"`
	DestroyHolds int           `json:"destroy_holds"`
	PurgePending bool          `json:"purge_pending"`
	Links        []string      `json:"links"`
}

// Group is the session group and logical lease gate of an incarnation.
// The logical lease is held while provisional or active requests exist or
// quiet_until is in the future; the process never expires the client while
// it is held.
type Group struct {
	Members            int        `json:"members"`
	Provisional        int        `json:"provisional_requests"`
	Active             int        `json:"active_requests"`
	Creating           int        `json:"create_sessions_in_progress"`
	State              string     `json:"state"`
	QuietUntil         *time.Time `json:"quiet_until,omitempty"`
	LeaseGate          string     `json:"lease_gate"`
	MarkerPhase        string     `json:"expired_marker_phase"`
	RenewedDuringCheck bool       `json:"renewed_during_check"`
	Dropped            bool       `json:"dropped"`
	ReclaimComplete    bool       `json:"reclaim_complete"`
}

// Group states, lease gate states and marker phases.
const (
	GroupOpen     = "open"
	GroupClosing  = "closing"
	GroupReplaced = "replaced"

	GateLive     = "live"
	GateChecking = "checking"
	GateExpired  = "expired"

	MarkerNone      = "none"
	MarkerDeciding  = "deciding"
	MarkerSubmitted = "submitted"
)

// Refresher is the durable lease worker's record. proven is the lower bound
// on the lease expiry published in the client store; a request is admitted
// only while it is in the future.
type Refresher struct {
	Identity    string     `json:"identity,omitempty"`
	State       string     `json:"state"`
	Proven      *time.Time `json:"proven_until,omitempty"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
	LastExpiry  *time.Time `json:"last_written_expiry,omitempty"`
	HasAttempt  bool       `json:"unsettled_rename"`
	Orphans     []Orphan   `json:"temp_obligations"`
	Bootstrap   bool       `json:"bootstrap"`
	InCall      bool       `json:"in_call"`
	Running     bool       `json:"worker_running"`
	Verify      bool       `json:"verify_wanted"`
	LastPass    *time.Time `json:"last_pass,omitempty"`
	Backoff     string     `json:"backoff,omitempty"`
	Refs        int        `json:"reservations"`
	Finished    bool       `json:"finished"`
	Pinned      bool       `json:"pinned"`
	NotBefore   *time.Time `json:"not_before,omitempty"`
	MaintainAt  *time.Time `json:"maintain_at,omitempty"`
	IdleWaiting bool       `json:"idle_waiting"`
	Reclaim     bool       `json:"reclaim"`
}

// Refresher states and temp obligation phases.
const (
	LeaseLive         = "live"
	LeaseStale        = "stale"
	LeaseFleetExpired = "fleet_expired"

	OrphanConstructing = "constructing"
	OrphanLinking      = "linking"
	OrphanLinked       = "linked"
	OrphanUnresolved   = "unresolved"
)

// Orphan is a temporary lease file whose edge has not been consumed yet.
type Orphan struct {
	Temp  string `json:"temp"`
	Inode string `json:"inode,omitempty"`
	Phase string `json:"phase"`
}

// CachedClient is the store location this process cached for the
// incarnation and the open stateids it has marked.
type CachedClient struct {
	IdentityID       string `json:"identity_id"`
	ConfirmedPointer string `json:"confirmed_pointer"`
	Opens            int    `json:"open_markers"`
}

// SessionFilter selects sessions. Zero values select everything. Slots
// omits the per-slot detail when false.
type SessionFilter struct {
	ID          *[16]byte
	Incarnation uint64
	Slots       bool
}

type Sessions struct {
	Envelope
	Records []Session `json:"records"`
}

// Session is one NFSv4.1 session. Sessions live in memory and end with the
// process.
type Session struct {
	ID            string       `json:"session"`
	Incarnation   string       `json:"incarnation"`
	WireClientID  string       `json:"wire_clientid,omitempty"`
	Fore          Channel      `json:"fore_channel"`
	Back          Channel      `json:"back_channel"`
	BackRequested bool         `json:"back_channel_requested"`
	CBProgram     uint32       `json:"callback_program"`
	Users         int          `json:"users"`
	Recalled      bool         `json:"slots_recalled"`
	Destroyed     bool         `json:"destroyed"`
	Fatal         bool         `json:"fatal"`
	StatusFlags   []string     `json:"sequence_status_flags"`
	Backchannel   *Backchannel `json:"backchannel,omitempty"`
	Slots         []Slot       `json:"slots,omitempty"`
	Links         []string     `json:"links"`
}

type Channel struct {
	MaxRequestSize  uint32 `json:"max_request_size"`
	MaxResponseSize uint32 `json:"max_response_size"`
	MaxCachedSize   uint32 `json:"max_cached_size"`
	MaxOperations   uint32 `json:"max_operations"`
	Slots           uint32 `json:"slots"`
}

// Backchannel is the session's callback sender.
type Backchannel struct {
	Program          uint32     `json:"program"`
	Credential       string     `json:"credential"`
	NextSeq          uint32     `json:"next_sequence"`
	Fault            bool       `json:"fault"`
	RecallPending    bool       `json:"recall_pending"`
	RecallTarget     uint32     `json:"recall_target_slot"`
	InFlight         bool       `json:"callback_in_flight"`
	LastRecallStatus string     `json:"last_recall_status,omitempty"`
	Running          bool       `json:"sender_running"`
	Cancelled        bool       `json:"cancelled"`
	Connections      []BackConn `json:"connections"`
}

// BackConn is a connection carrying this session's callbacks, in BIND
// order, with its path state.
type BackConn struct {
	Remote string `json:"remote"`
	Path   string `json:"path"`
}

// Callback path states.
const (
	PathProbing      = "probing"
	PathUp           = "up"
	PathUnresponsive = "unresponsive"
)

// Slot is one session slot. A slot that a request holds at the instant of
// the snapshot is reported as locked with no other fields: the snapshot
// only tries the lock, so it never waits behind a request. A SEQUENCE that
// arrives during the snapshot's own short hold sees a busy slot and
// retries, as it would for any other holder.
type Slot struct {
	Index      int    `json:"index"`
	Locked     bool   `json:"locked,omitempty"`
	Seq        uint32 `json:"seq"`
	InUse      bool   `json:"in_use"`
	Cached     bool   `json:"cached"`
	Occupied   bool   `json:"occupied"`
	Reserved   int    `json:"reserved_bytes"`
	ReplyBytes int    `json:"cached_reply_bytes"`
	HasBuffer  bool   `json:"has_buffer"`
	User       *User  `json:"user,omitempty"`
}

type User struct {
	Flavor string `json:"flavor"`
	UID    uint32 `json:"uid"`
	GID    uint32 `json:"gid"`
}

// ConnectionFilter selects connections; a session id selects those
// associated with it in either direction.
type ConnectionFilter struct {
	Session *[16]byte
}

type Connections struct {
	Envelope
	Records []Connection `json:"records"`
}

// Connection is one live TCP connection from a client. Fore associations are
// the sessions whose requests it may carry; back associations are the
// sessions whose callbacks it carries.
type Connection struct {
	Remote                  string    `json:"remote"`
	ConnectedAt             time.Time `json:"connected_at"`
	InFlight                int32     `json:"requests_in_flight"`
	Requests                uint64    `json:"requests_total"`
	Sessions                []string  `json:"sessions"`
	PendingAssociations     int       `json:"pending_associations"`
	BackSessions            []string  `json:"back_sessions"`
	PendingBackAssociations int       `json:"pending_back_associations"`
	OutstandingCallbacks    int       `json:"outstanding_callbacks"`
	ProbeOutstanding        bool      `json:"cb_null_outstanding"`
	Path                    string    `json:"path"`
}

// OpenFilter selects open state. Zero values select everything.
type OpenFilter struct {
	Stateid     *[12]byte
	Incarnation uint64
	File        uint64
}

// Opens is the process-local open state: live opens, expired stateid
// tombstones, and the client views that decide what a handle shows.
type Opens struct {
	Envelope
	Records []Open           `json:"records"`
	Expired []ExpiredStateid `json:"expired"`
	Views   Views            `json:"views"`
}

// Open is one open-owner's open of one file. file is the filehandle the
// client holds; data is what its I/O uses, the writer's staging inode or the
// published version a reader opened.
type Open struct {
	Stateid     string   `json:"stateid"`
	FileID      string   `json:"file"`
	DataID      string   `json:"data"`
	Incarnation string   `json:"incarnation"`
	OwnerHex    string   `json:"owner_hex"`
	Access      []string `json:"access"`
	ShareModes  uint8    `json:"share_access_values"`
	DenyModes   uint8    `json:"share_deny_values"`
	Generation  uint32   `json:"generation"`
	Seq         uint64   `json:"seq"`
	Writer      bool     `json:"writer"`
	// Marked reports a durable open marker in the client store. Write
	// opens, creates and recovered writers are marked, and stay marked
	// after OPEN_DOWNGRADE; other read opens are process-local.
	Marked bool `json:"marked"`
}

// ExpiredStateid is a tombstone: the owner gets EXPIRED, others BAD_STATEID.
type ExpiredStateid struct {
	Stateid     string `json:"stateid"`
	Incarnation string `json:"incarnation"`
}

// Views are the per-client aliases that make a handle show what the client
// itself published, the writer records COMMIT uses to find a failed
// tombstone, and the CLAIM_FH name hints.
type Views struct {
	Aliases       []Alias `json:"aliases"`
	WriterEntries int     `json:"writer_entries"`
	NameHints     int     `json:"name_hints"`
}

type Alias struct {
	Incarnation string    `json:"incarnation"`
	FileID      string    `json:"file"`
	DataID      string    `json:"data"`
	Expires     time.Time `json:"expires"`
}

// Confirmations are the client confirmations this process has in progress
// or unresolved, with the protection and destruction holds around them.
type Confirmations struct {
	Envelope
	Records              []Confirmation `json:"records"`
	DestroyHolds         []Hold         `json:"destroy_holds"`
	IdentityDestroyHolds []Hold         `json:"identity_destroy_holds"`
	PurgePending         []string       `json:"purge_pending"`
	PurgeLimit           int            `json:"purge_limit"`
}

// Hold counts the destruction holds in flight on an incarnation or an
// identity directory.
type Hold struct {
	ID    string `json:"id"`
	Holds int    `json:"holds"`
}

// Confirmation is one identity's confirmation of incarnation new over
// predecessor. While it exists, neither may be expired, retired or
// collected.
type Confirmation struct {
	Identity    string   `json:"identity"`
	OwnerHex    string   `json:"owner_hex"`
	New         string   `json:"new"`
	Predecessor string   `json:"predecessor,omitempty"`
	Applying    bool     `json:"applying"`
	Done        bool     `json:"done"`
	Attempt     *Attempt `json:"attempt,omitempty"`
}

// Attempt is the evidence of the confirmation's writes. It is locked while
// a settle runs backend I/O, in which case only locked is reported.
type Attempt struct {
	Locked   bool           `json:"locked,omitempty"`
	Result   string         `json:"result,omitempty"`
	PreWrite string         `json:"no_write_reason,omitempty"`
	Panicked bool           `json:"panicked,omitempty"`
	Writes   []ConfirmWrite `json:"writes,omitempty"`
	Observed *Observed      `json:"observed,omitempty"`
}

type ConfirmWrite struct {
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Temp      string `json:"temp,omitempty"`
	Submitted bool   `json:"submitted"`
	Error     string `json:"error,omitempty"`
	Fence     string `json:"fence"`
	// FenceError is why the fence is failed: the RemoveEdge error, or the
	// unknown-outcome marker when it never returned. Empty otherwise.
	FenceError string `json:"fence_error,omitempty"`
	Outcome    string `json:"outcome"`
}

type Observed struct {
	Confirmed string `json:"confirmed,omitempty"`
	File      string `json:"file,omitempty"`
	Found     bool   `json:"found"`
}

// Confirmation results, write outcomes and fence states.
const (
	ConfirmUnresolved   = "unresolved"
	ConfirmCommitted    = "committed"
	ConfirmRolledBack   = "rolled_back"
	ConfirmConservative = "conservative"

	WriteNever     = "never"
	WriteApplied   = "applied"
	WriteMaybe     = "maybe"
	WriteUnsettled = "unsettled"

	FenceNone       = "none"
	FenceNotApplied = "not_applied"
	FenceEdgeGone   = "edge_gone"
	FenceFailed     = "failed"
)

// StagingFilter selects staging entries. Zero values select everything.
// Locked entries with unavailable owner metadata remain possible matches
// for Incarnation and make the response incomplete.
type StagingFilter struct {
	File        uint64
	Incarnation uint64
}

// Staging is the in-memory view of the local staging store: every staged
// writer and the failed tombstones that outlive their files.
type Staging struct {
	Envelope
	Configured bool           `json:"configured"`
	Incomplete bool           `json:"incomplete,omitempty"`
	Records    []StagingEntry `json:"records"`
	Failed     []string       `json:"failed_tombstones"`
}

// StagingEntry is one staged writer. A file whose lock a disk operation
// holds at the instant of the snapshot is reported as locked with only its
// identity, publication target and hydration activity. Its other fields are
// unavailable, not zero-valued observations. Hydrating is true only while
// the speculative hydration worker has not completed.
type StagingEntry struct {
	FileID          string `json:"file"`
	Locked          bool   `json:"locked,omitempty"`
	DirID           string `json:"dir,omitempty"`
	FileName        string `json:"name"`
	Incarnation     string `json:"incarnation,omitempty"`
	Stateid         string `json:"stateid,omitempty"`
	OwnerHex        string `json:"owner_hex,omitempty"`
	OwnerKnown      bool   `json:"owner_known"`
	Size            uint64 `json:"size"`
	BaseID          string `json:"base,omitempty"`
	BaseSize        uint64 `json:"base_size"`
	DirtyRanges     int    `json:"dirty_ranges"`
	DirtyBytes      uint64 `json:"dirty_bytes"`
	HydratedRanges  int    `json:"hydrated_ranges"`
	Unlinked        bool   `json:"unlinked"`
	Guarded         bool   `json:"guarded"`
	Retired         bool   `json:"retired"`
	ReadOnly        bool   `json:"read_only"`
	Exclusive       bool   `json:"exclusive"`
	MetadataChanged bool   `json:"metadata_changed"`
	CheckpointDirty bool   `json:"checkpoint_dirty"`
	Removed         bool   `json:"removed"`
	Hydrating       bool   `json:"hydrating"`
}

// Time returns nil for the zero time, so optional timestamps are omitted.
func Time(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// UnixNanos converts a unix nanosecond timestamp, zero meaning unset.
func UnixNanos(ns int64) *time.Time {
	if ns == 0 {
		return nil
	}
	t := time.Unix(0, ns)
	return &t
}

// Bool returns a pointer for an optional boolean.
func Bool(b bool) *bool { return &b }
