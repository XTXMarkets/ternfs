// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Package clientrecords keeps one record per confirmed NFS client identity
// in the staging directory, under clients/. A record holds what the server
// needs to decide registration, confirmation, client reboot, expiry and
// writer eligibility after a restart: the EXCHANGE_ID owner, the RPC
// principal, the confirmed incarnation and its boot verifier, and whether
// the client expired. An unconfirmed registration is not recorded.
//
// The staging directory is the server's identity, so the records go with it:
// a restart or a copy of the whole directory keeps them, a wiped directory
// has none. They are only on local disk, written by one process, so every
// update has a known outcome.
//
// Layout:
//
//	<staging>/clients/
//	    <identity-hash>                    one record, JSON
//	    .<identity-hash>.tmp-<random>      record being written
//
// The identity hash is the SHA-256 of the owner, as 64 hex digits. A record
// is replaced by writing a temporary, fsyncing it, renaming it over the
// record and fsyncing clients/; it is removed by unlinking it and fsyncing
// clients/. Every transition returns only once its record is durable, so a
// caller acts on it afterwards: a confirmation is answered after its record,
// state is purged or retired after the record says so. Open removes
// temporaries a crash left.
//
// A reboot (Confirm over a confirmed record) is one write. The caller then
// purges the replaced incarnation's sessions and staged writers. A crash
// before it has done so leaves nothing to finish: no record confirms that
// incarnation, so a restart neither restores its sessions nor resumes its
// writers.
//
// Expiry has two steps. It sets expired, the caller retires the
// incarnation's sessions, then calls Remove. A crash between them leaves
// the record expired, and Finish completes the expiry at startup.
// Destruction retires the incarnation's sessions and opens first and then
// calls Remove, the order the server uses for DESTROY_CLIENTID.
//
// Nothing else accumulates: a record goes at expiry or destruction.
package clientrecords

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/keyedlock"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
)

var (
	// ErrNotCurrent is returned by a transition naming an incarnation that
	// no record has confirmed.
	ErrNotCurrent = errors.New("client records: incarnation is not confirmed")
	// ErrUnfinished is returned by Confirm over a record that is expired,
	// which would drop the evidence of an expiry still in progress.
	ErrUnfinished = errors.New("client records: earlier transition unfinished")
	// ErrCorrupt is returned by Open for clients/ content it did not write.
	ErrCorrupt = errors.New("client records: corrupt")
	// ErrInUse is returned by Confirm for an incarnation a record already
	// confirms.
	ErrInUse = errors.New("client records: incarnation in use")
)

// Incarnation is one lifetime of a client identity: 64 random bits, never
// zero, chosen by the server when the client registers and recorded when it
// is confirmed. A sidecar's ClientID holds it.
type Incarnation uint64

// NewIncarnation returns a random incarnation. The caller checks it is not
// in use.
func NewIncarnation() Incarnation {
	for {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			panic(err) // crypto/rand does not fail on Linux
		}
		if id := Incarnation(binary.BigEndian.Uint64(b[:])); id != 0 {
			return id
		}
	}
}

func (i Incarnation) String() string { return fmt.Sprintf("%016x", uint64(i)) }

func (i Incarnation) MarshalText() ([]byte, error) { return []byte(i.String()), nil }

func (i *Incarnation) UnmarshalText(text []byte) error {
	if len(text) != 16 {
		return fmt.Errorf("incarnation %q: not 16 hex digits", text)
	}
	v, err := strconv.ParseUint(string(text), 16, 64)
	if err != nil {
		return fmt.Errorf("incarnation %q: %w", text, err)
	}
	*i = Incarnation(v)
	return nil
}

// IdentityHash names a client identity: the SHA-256 of its EXCHANGE_ID
// owner.
type IdentityHash [32]byte

func HashOwner(owner []byte) IdentityHash { return sha256.Sum256(owner) }

func (h IdentityHash) String() string { return hex.EncodeToString(h[:]) }

// Principal identifies the caller of an RPC by its credential flavor and
// body. It is compared, not interpreted.
type Principal struct {
	Flavor uint32
	Body   string
}

// Record is one client identity's durable state.
type Record struct {
	Owner     []byte
	Principal Principal
	// Incarnation is the confirmed incarnation, Verifier its client boot
	// verifier and ConfirmedAt when the server confirmed it.
	Incarnation Incarnation
	Verifier    [8]byte
	ConfirmedAt time.Time
	// Expired says the server expired Incarnation and is retiring its
	// state. The record is removed once it has.
	Expired bool
}

func (rec Record) Hash() IdentityHash { return HashOwner(rec.Owner) }

// RecoveryKey identifies one client boot independently of its incarnation:
// the identity, the boot verifier and the principal. A staged writer keeps
// it so that the same boot can recover its data after its incarnation
// expired, and no other principal can.
func (rec Record) RecoveryKey() [32]byte {
	h := sha256.New()
	h.Write([]byte("ternfs-nfsd recovery key\x00"))
	id := rec.Hash()
	h.Write(id[:])
	h.Write(rec.Verifier[:])
	var n [8]byte
	binary.BigEndian.PutUint32(n[:4], rec.Principal.Flavor)
	binary.BigEndian.PutUint32(n[4:], uint32(len(rec.Principal.Body)))
	h.Write(n[:])
	h.Write([]byte(rec.Principal.Body))
	var key [32]byte
	h.Sum(key[:0])
	return key
}

func (rec Record) clone() Record {
	rec.Owner = bytes.Clone(rec.Owner)
	return rec
}

// Records is the set of client records of one staging directory. Its
// methods are safe for concurrent use; transitions of one identity are
// serialised.
type Records struct {
	dir   string
	locks keyedlock.Locker[IdentityHash]

	mu      sync.Mutex
	records map[IdentityHash]Record
	byID    map[Incarnation]IdentityHash
	// failed is set when an update may have reached the directory without
	// being made durable. Nothing is known about the records after it.
	failed error

	// crash, set by tests, stops an update at a crash point when it
	// returns an error, leaving the directory as a crash there would.
	crash func(crashPoint) error
}

type crashPoint string

const (
	crashSynced  crashPoint = "temporary synced"
	crashRenamed crashPoint = "renamed"
	crashRemoved crashPoint = "removed"
)

// Open loads the records of stagingDir, creating clients/ if it is absent
// and removing temporaries. Call it after the staging directory's server
// identity has been checked, which refuses a directory with records but no
// server-id.
func Open(stagingDir string) (*Records, error) {
	dir := filepath.Join(stagingDir, staging.ClientsDirName)
	switch err := os.Mkdir(dir, 0700); {
	case err == nil:
		if err := syncDir(stagingDir); err != nil {
			return nil, err
		}
	case !errors.Is(err, fs.ErrExist):
		return nil, err
	}
	r := &Records{
		dir:     dir,
		records: make(map[IdentityHash]Record),
		byID:    make(map[Incarnation]IdentityHash),
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			return nil, fmt.Errorf("%w: unexpected directory %s", ErrCorrupt, filepath.Join(dir, name))
		}
		if strings.HasPrefix(name, ".") && strings.Contains(name, ".tmp-") {
			if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
			continue
		}
		rec, err := r.load(name)
		if err != nil {
			return nil, err
		}
		h := rec.Hash()
		if _, dup := r.byID[rec.Incarnation]; dup {
			return nil, fmt.Errorf("%w: %s: incarnation %s used twice", ErrCorrupt, filepath.Join(dir, name), rec.Incarnation)
		}
		r.records[h] = rec
		r.byID[rec.Incarnation] = h
	}
	return r, nil
}

func (r *Records) load(name string) (Record, error) {
	path := filepath.Join(r.dir, name)
	var want IdentityHash
	if n, err := hex.Decode(want[:], []byte(name)); err != nil || n != len(want) || len(name) != 2*len(want) || strings.ToLower(name) != name {
		return Record{}, fmt.Errorf("%w: unexpected entry %s", ErrCorrupt, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var j recordJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Record{}, fmt.Errorf("%w: %s: %v", ErrCorrupt, path, err)
	}
	rec := j.record()
	switch {
	case len(rec.Owner) == 0:
		return Record{}, fmt.Errorf("%w: %s: no owner", ErrCorrupt, path)
	case rec.Hash() != want:
		return Record{}, fmt.Errorf("%w: %s: owner does not match the name", ErrCorrupt, path)
	case rec.Incarnation == 0:
		return Record{}, fmt.Errorf("%w: %s: bad incarnation", ErrCorrupt, path)
	}
	return rec, nil
}

// InUse says whether a record confirms id.
func (r *Records) InUse(id Incarnation) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.byID[id]
	return ok
}

// Lookup returns the record of an identity.
func (r *Records) Lookup(h IdentityHash) (Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[h]
	return rec.clone(), ok
}

// Live returns the record whose confirmed incarnation is id, unless it
// expired. A session file or staged writer of any other incarnation is no
// longer its client's: the client rebooted, expired or was destroyed.
func (r *Records) Live(id Incarnation) (Record, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.byID[id]
	if !ok || r.records[h].Expired {
		return Record{}, false
	}
	return r.records[h].clone(), true
}

// List returns every record, ordered by identity hash.
func (r *Records) List() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Record, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, rec.clone())
	}
	slices.SortFunc(out, func(a, b Record) int {
		ha, hb := a.Hash(), b.Hash()
		return bytes.Compare(ha[:], hb[:])
	})
	return out
}

// Confirm records id as owner's confirmed incarnation, with its boot
// verifier and principal, and returns the record once it is durable. If
// the identity already had a confirmed incarnation, a reboot or a takeover
// by another principal replaced it, and the caller purges its state.
func (r *Records) Confirm(owner []byte, verifier [8]byte, principal Principal, id Incarnation) (Record, error) {
	if len(owner) == 0 {
		return Record{}, errors.New("client records: empty owner")
	}
	if id == 0 {
		return Record{}, errors.New("client records: zero incarnation")
	}
	h := HashOwner(owner)
	unlock := r.locks.Lock(h)
	defer unlock()
	r.mu.Lock()
	old, exists := r.records[h]
	_, used := r.byID[id]
	err := r.failed
	switch {
	case err != nil:
	case exists && old.Expired:
		err = ErrUnfinished
	case used:
		err = ErrInUse
	}
	r.mu.Unlock()
	if err != nil {
		return Record{}, err
	}
	rec := Record{
		Owner:       bytes.Clone(owner),
		Principal:   principal,
		Incarnation: id,
		Verifier:    verifier,
		ConfirmedAt: time.Now().UTC(),
	}
	if err := r.write(rec, old.Incarnation); err != nil {
		return Record{}, err
	}
	return rec.clone(), nil
}

// Expire records that the server expired incarnation id. The caller then
// retires its state and calls Remove.
func (r *Records) Expire(id Incarnation) error {
	return r.update(id, func(rec *Record) bool {
		changed := !rec.Expired
		rec.Expired = true
		return changed
	})
}

// Remove removes the record whose confirmed incarnation is id: the end of
// an expiry, or DESTROY_CLIENTID.
func (r *Records) Remove(id Incarnation) error {
	h, unlock, err := r.lockCurrent(id)
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(r.dir, h.String())
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := r.settle(crashRemoved); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.records, h)
	delete(r.byID, id)
	r.mu.Unlock()
	return nil
}

// Finish completes the expiries a crash interrupted. It runs at startup,
// before the server serves: retire is called for an expired record, which
// is then removed. retire must tolerate state already partly gone.
func (r *Records) Finish(retire func(Record) error) error {
	for _, rec := range r.List() {
		if rec.Expired {
			if err := retire(rec); err != nil {
				return err
			}
			if err := r.Remove(rec.Incarnation); err != nil {
				return err
			}
		}
	}
	return nil
}

// update rewrites the record confirming id with change applied, if change
// reports a difference.
func (r *Records) update(id Incarnation, change func(*Record) bool) error {
	h, unlock, err := r.lockCurrent(id)
	if err != nil {
		return err
	}
	defer unlock()
	r.mu.Lock()
	rec := r.records[h].clone()
	r.mu.Unlock()
	if !change(&rec) {
		return nil
	}
	return r.write(rec, id)
}

// lockCurrent locks the identity whose confirmed incarnation is id.
func (r *Records) lockCurrent(id Incarnation) (IdentityHash, func(), error) {
	r.mu.Lock()
	h, ok := r.byID[id]
	err := r.failed
	r.mu.Unlock()
	if err != nil {
		return IdentityHash{}, nil, err
	}
	if !ok {
		return IdentityHash{}, nil, ErrNotCurrent
	}
	unlock := r.locks.Lock(h)
	r.mu.Lock()
	current := r.records[h].Incarnation == id
	r.mu.Unlock()
	if !current {
		unlock()
		return IdentityHash{}, nil, ErrNotCurrent
	}
	return h, unlock, nil
}

// write replaces rec's file and the in-memory copy, which confirmed old
// before (zero for none). The identity lock is held. The in-memory copy
// changes only once the file is durable: Lookup, Live and List never show
// an update the caller has not been told succeeded, which it would act on,
// such as a confirmation it answered with an error.
func (r *Records) write(rec Record, old Incarnation) error {
	data, err := json.Marshal(newRecordJSON(rec))
	if err != nil {
		return err
	}
	h := rec.Hash()
	tmp, err := os.CreateTemp(r.dir, "."+h.String()+".tmp-*")
	if err != nil {
		return err
	}
	if err := writeSynced(tmp, data); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := r.crashAt(crashSynced); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(r.dir, h.String())); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := r.settle(crashRenamed); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.byID, old)
	r.records[h] = rec.clone()
	r.byID[rec.Incarnation] = h
	r.mu.Unlock()
	return nil
}

// settle makes a rename or unlink in clients/ durable. If that fails the
// directory may hold the change or not, so every later transition fails,
// and the in-memory records keep the state before it.
func (r *Records) settle(point crashPoint) error {
	err := r.crashAt(point)
	if err == nil {
		err = syncDir(r.dir)
	}
	if err != nil {
		r.mu.Lock()
		if r.failed == nil {
			r.failed = fmt.Errorf("client records: %s not durable: %w", r.dir, err)
		}
		err = r.failed
		r.mu.Unlock()
	}
	return err
}

func (r *Records) crashAt(point crashPoint) error {
	if r.crash == nil {
		return nil
	}
	return r.crash(point)
}

func writeSynced(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

// recordJSON is a record file's body.
type recordJSON struct {
	Owner     []byte `json:"owner"`
	Principal struct {
		Flavor uint32 `json:"flavor"`
		Body   []byte `json:"body"`
	} `json:"principal"`
	Confirmed struct {
		ID          Incarnation `json:"id"`
		Verifier    hexVerifier `json:"verifier"`
		ConfirmedAt time.Time   `json:"confirmed_at"`
	} `json:"confirmed"`
	Expired bool `json:"expired"`
}

func newRecordJSON(rec Record) recordJSON {
	var j recordJSON
	j.Owner = rec.Owner
	j.Principal.Flavor = rec.Principal.Flavor
	j.Principal.Body = []byte(rec.Principal.Body)
	j.Confirmed.ID = rec.Incarnation
	j.Confirmed.Verifier = hexVerifier(rec.Verifier)
	j.Confirmed.ConfirmedAt = rec.ConfirmedAt
	j.Expired = rec.Expired
	return j
}

func (j recordJSON) record() Record {
	return Record{
		Owner:       j.Owner,
		Principal:   Principal{Flavor: j.Principal.Flavor, Body: string(j.Principal.Body)},
		Incarnation: j.Confirmed.ID,
		Verifier:    j.Confirmed.Verifier,
		ConfirmedAt: j.Confirmed.ConfirmedAt,
		Expired:     j.Expired,
	}
}

type hexVerifier [8]byte

func (v hexVerifier) MarshalText() ([]byte, error) { return []byte(hex.EncodeToString(v[:])), nil }

func (v *hexVerifier) UnmarshalText(text []byte) error {
	if n, err := hex.Decode(v[:], text); err != nil || n != len(v) || len(text) != 2*len(v) {
		return fmt.Errorf("verifier %q: not %d hex digits", text, 2*len(v))
	}
	return nil
}
