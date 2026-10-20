// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Package sessionstore is the on-disk format of NFSv4.1 session files: one
// fixed-layout file per session, overwritten in place, whose slots each hold
// their last request's record. Why it is shaped this way and how the server
// uses it is in ../server/README.md#session-files-on-disk.
//
// The slot count and area size are fixed in the header, and every offset is
// computed from it. Each slot has two record areas, and a write goes to the
// area that does not hold the slot's newest record. Load keeps, per slot,
// the valid record (CRC32C) with the later sequence number in serial order
// (SeqAfter), and at an equal sequence number a completed request's record
// over its intent, which is written first. Nothing is fsynced: a pwrite
// survives the death of the process, not necessarily a host crash.
//
// File layout (big-endian):
//
//	header area, headerAreaSize bytes:
//	  [4] "NFSS"  [2] version  [4] slot count  [4] area size
//	  [4] session record length  [N] session record  [4] CRC32C
//	slot areas: slotCount * 2 areas of areaSize bytes, slot s area a at
//	  headerAreaSize + (2*s + a) * areaSize
//	record, within an area:
//	  [4] "NFSR"  [4] seq  [1] kind  [1] flags (bit 0 cachethis)
//	  [2] user length  [32] digest  [4] payload length
//	  [N] user  [M] payload  [4] CRC32C over everything before it
package sessionstore

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	// IDSize is the size of a session identifier, as on the wire.
	IDSize = 16

	// headerAreaSize is the space reserved for the file header. The
	// session record must fit in it with the fixed fields and CRC.
	headerAreaSize = 4096

	// MaxSessionRecord is the largest session record a header can hold.
	MaxSessionRecord = headerAreaSize - headerFixedSize - 4

	// recordFixedSize is a record's fields before user and payload.
	recordFixedSize = 4 + 4 + 1 + 1 + 2 + 32 + 4

	// RecordOverhead is what a record adds to its user and payload: the
	// fixed fields and the CRC. A record needs RecordOverhead + len(user)
	// + len(payload) bytes of area.
	RecordOverhead = recordFixedSize + 4

	headerFixedSize = 4 + 2 + 4 + 4 + 4

	// formatVersion changes whenever what a file means changes, the
	// server's payloads included. Load takes only files of this version: a
	// reader of another would misread its records. Version 2: an intent
	// names the index of the operation it protects. Version 3: areas are
	// written in turn, not chosen by sequence number, and a completion
	// beats its intent at the same sequence number. Version 4: no file is
	// left of a session that sent a callback or changed its callback
	// program, which a version 3 file may be. Version 5: no file is left
	// of a session that ran a compound with more than one mutating
	// operation, which a version 4 file may be.
	formatVersion = 5

	areasPerSlot = 2
)

var (
	fileMagic   = [4]byte{'N', 'F', 'S', 'S'}
	recordMagic = [4]byte{'N', 'F', 'S', 'R'}
	crcTable    = crc32.MakeTable(crc32.Castagnoli)

	// ErrTooLarge is returned when a record does not fit its slot area.
	ErrTooLarge = errors.New("record larger than the slot area")
	// ErrBadSlot is returned for a slot index beyond the header's count.
	ErrBadSlot = errors.New("slot index out of range")
	// ErrCorrupt is returned by Load for a file whose header is unusable.
	ErrCorrupt = errors.New("corrupt session file")
)

// ID is a session identifier.
type ID [IDSize]byte

func (id ID) String() string { return hex.EncodeToString(id[:]) }

// Header is a session file's fixed layout and the opaque session record
// the server stores with it.
type Header struct {
	SlotCount uint32
	// AreaSize is the size of one record area. The largest record a slot
	// can hold is AreaSize - RecordOverhead bytes of user plus payload.
	AreaSize uint32
	// Session is the server's description of the session: identity,
	// channel attributes, whatever it needs to rebuild the session. At
	// most MaxSessionRecord bytes.
	Session []byte
}

// Kind tells what a record describes.
type Kind uint8

const (
	// KindSlot is a completed request: the slot's sequence number, the
	// request's identity and, when cached, its reply.
	KindSlot Kind = 1
	// KindIntent is a request about to perform a non-idempotent effect,
	// written before the effect. Its payload lets a later process decide
	// whether the effect happened.
	KindIntent Kind = 2
)

// Record is one slot's state for one request.
type Record struct {
	Seq       uint32
	Kind      Kind
	CacheThis bool
	// Digest identifies the request's arguments; User its effective user.
	// Together they detect a false retry.
	Digest [32]byte
	User   []byte
	// Payload is the cached reply body for KindSlot (empty when not
	// cached) and the effect's identity for KindIntent.
	Payload []byte
}

// Store is a directory of session files.
type Store struct {
	dir string
}

// Open prepares dir, creating it if needed.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir is the store's directory.
func (s *Store) Dir() string { return s.dir }

// Path is the file for a session.
func (s *Store) Path(id ID) string { return filepath.Join(s.dir, id.String()) }

// Create makes the file for a new session. The file must not exist.
func (s *Store) Create(id ID, h Header) (*File, error) {
	if h.SlotCount == 0 || h.AreaSize < RecordOverhead {
		return nil, fmt.Errorf("session %s: invalid layout %d slots of %d bytes", id, h.SlotCount, h.AreaSize)
	}
	if len(h.Session) > MaxSessionRecord {
		return nil, fmt.Errorf("session %s: session record of %d bytes exceeds %d", id, len(h.Session), MaxSessionRecord)
	}
	path := s.Path(id)
	fd, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	f := &File{id: id, header: h, fd: fd, path: path, slots: make([]slotState, h.SlotCount)}
	if err := f.writeHeader(); err != nil {
		fd.Close()
		os.Remove(path)
		return nil, err
	}
	// Sparse: the areas take disk only as they are written.
	if err := fd.Truncate(f.size()); err != nil {
		fd.Close()
		os.Remove(path)
		return nil, err
	}
	return f, nil
}

// Remove deletes a session's file. A missing file is not an error.
func (s *Store) Remove(id ID) error {
	err := os.Remove(s.Path(id))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Problem is a file Load could not use, and the session it is named for.
type Problem struct {
	ID   ID
	Path string
	Err  error
}

// Load opens every session file in the store and reads its slots. Files
// whose header is unusable are reported as problems and left in place;
// a slot area whose record does not verify is ignored.
func (s *Store) Load() ([]*File, []Problem, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, nil, err
	}
	var files []*File
	var problems []Problem
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		raw, err := hex.DecodeString(e.Name())
		if err != nil || len(raw) != IDSize {
			continue // not a session file
		}
		var id ID
		copy(id[:], raw)
		f, err := s.open(id)
		if err != nil {
			problems = append(problems, Problem{ID: id, Path: s.Path(id), Err: err})
			continue
		}
		files = append(files, f)
	}
	return files, problems, nil
}

// open reads an existing session file.
func (s *Store) open(id ID) (*File, error) {
	path := s.Path(id)
	fd, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	f := &File{id: id, fd: fd, path: path}
	if err := f.readHeader(); err != nil {
		fd.Close()
		return nil, err
	}
	f.slots = make([]slotState, f.header.SlotCount)
	for slot := range f.slots {
		st := &f.slots[slot]
		for area := 0; area < areasPerSlot; area++ {
			rec, ok := f.readArea(uint32(slot), area)
			if ok && (!st.present || newer(rec, st.record)) {
				*st = slotState{record: rec, present: true, area: area}
			}
		}
	}
	return f, nil
}

// newer says whether record a replaces b as a slot's newest: a later
// sequence number, or the same one completed where b is its intent.
func newer(a, b Record) bool {
	return SeqAfter(a.Seq, b.Seq) || a.Seq == b.Seq && a.Kind == KindSlot && b.Kind == KindIntent
}

// SeqAfter says whether sequence number a comes after b, allowing for
// wrap-around.
func SeqAfter(a, b uint32) bool { return int32(a-b) > 0 }

// File is one session's file.
type File struct {
	id     ID
	header Header
	fd     *os.File
	path   string

	mu    sync.RWMutex // slots; the file itself takes concurrent pwrites
	slots []slotState
}

type slotState struct {
	record  Record
	present bool
	area    int // the area holding record
}

// ID is the session this file belongs to.
func (f *File) ID() ID { return f.id }

// Header is the file's layout and session record.
func (f *File) Header() Header { return f.header }

// Path is the file's path.
func (f *File) Path() string { return f.path }

// Slot is the newest valid record for a slot, if any.
func (f *File) Slot(slot uint32) (Record, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if int(slot) >= len(f.slots) || !f.slots[slot].present {
		return Record{}, false
	}
	return f.slots[slot].record, true
}

// Write records a request on its slot, in the area that does not hold the
// slot's newest record, so that record survives until this one is
// complete. The caller serialises writes to one slot. A write that fails
// leaves the newest record as it was.
func (f *File) Write(slot uint32, rec Record) error {
	if slot >= f.header.SlotCount {
		return ErrBadSlot
	}
	n := RecordOverhead + len(rec.User) + len(rec.Payload)
	if n > int(f.header.AreaSize) || len(rec.User) > 0xffff {
		return ErrTooLarge
	}
	buf := make([]byte, n)
	copy(buf[0:4], recordMagic[:])
	binary.BigEndian.PutUint32(buf[4:8], rec.Seq)
	buf[8] = byte(rec.Kind)
	if rec.CacheThis {
		buf[9] = 1
	}
	binary.BigEndian.PutUint16(buf[10:12], uint16(len(rec.User)))
	copy(buf[12:44], rec.Digest[:])
	binary.BigEndian.PutUint32(buf[44:48], uint32(len(rec.Payload)))
	off := recordFixedSize
	copy(buf[off:], rec.User)
	off += len(rec.User)
	copy(buf[off:], rec.Payload)
	off += len(rec.Payload)
	binary.BigEndian.PutUint32(buf[off:off+4], crc32.Checksum(buf[:off], crcTable))
	f.mu.RLock()
	area := 0
	if newest := f.slots[slot]; newest.present {
		area = 1 - newest.area
	}
	f.mu.RUnlock()
	if _, err := f.fd.WriteAt(buf, f.areaOffset(slot, area)); err != nil {
		return err
	}
	f.mu.Lock()
	f.slots[slot] = slotState{record: rec, present: true, area: area}
	f.mu.Unlock()
	return nil
}

// Close releases the file descriptor. The file stays on disk.
func (f *File) Close() error { return f.fd.Close() }

// Remove closes and deletes the file.
func (f *File) Remove() error {
	f.fd.Close()
	return f.Unlink()
}

// Unlink deletes the file and keeps the descriptor open. A missing file is
// not an error.
func (f *File) Unlink() error {
	err := os.Remove(f.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (f *File) size() int64 {
	return headerAreaSize + int64(f.header.SlotCount)*areasPerSlot*int64(f.header.AreaSize)
}

func (f *File) areaOffset(slot uint32, area int) int64 {
	return headerAreaSize + (int64(slot)*areasPerSlot+int64(area))*int64(f.header.AreaSize)
}

func (f *File) writeHeader() error {
	buf := make([]byte, headerFixedSize+len(f.header.Session)+4)
	copy(buf[0:4], fileMagic[:])
	binary.BigEndian.PutUint16(buf[4:6], formatVersion)
	binary.BigEndian.PutUint32(buf[6:10], f.header.SlotCount)
	binary.BigEndian.PutUint32(buf[10:14], f.header.AreaSize)
	binary.BigEndian.PutUint32(buf[14:18], uint32(len(f.header.Session)))
	copy(buf[18:], f.header.Session)
	off := headerFixedSize + len(f.header.Session)
	binary.BigEndian.PutUint32(buf[off:off+4], crc32.Checksum(buf[:off], crcTable))
	_, err := f.fd.WriteAt(buf, 0)
	return err
}

func (f *File) readHeader() error {
	buf := make([]byte, headerAreaSize)
	n, err := f.fd.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if n < headerFixedSize+4 || [4]byte(buf[0:4]) != fileMagic {
		return fmt.Errorf("%w: bad magic", ErrCorrupt)
	}
	if v := binary.BigEndian.Uint16(buf[4:6]); v != formatVersion {
		return fmt.Errorf("%w: version %d", ErrCorrupt, v)
	}
	h := Header{
		SlotCount: binary.BigEndian.Uint32(buf[6:10]),
		AreaSize:  binary.BigEndian.Uint32(buf[10:14]),
	}
	sessionLen := int(binary.BigEndian.Uint32(buf[14:18]))
	if sessionLen > MaxSessionRecord || headerFixedSize+sessionLen+4 > n {
		return fmt.Errorf("%w: session record length %d", ErrCorrupt, sessionLen)
	}
	off := headerFixedSize + sessionLen
	if crc32.Checksum(buf[:off], crcTable) != binary.BigEndian.Uint32(buf[off:off+4]) {
		return fmt.Errorf("%w: header checksum", ErrCorrupt)
	}
	if h.SlotCount == 0 || h.AreaSize < RecordOverhead {
		return fmt.Errorf("%w: layout %d slots of %d bytes", ErrCorrupt, h.SlotCount, h.AreaSize)
	}
	h.Session = append([]byte(nil), buf[headerFixedSize:off]...)
	f.header = h
	return nil
}

// readArea decodes one slot area. Anything that does not verify, including
// an area never written, is reported as absent.
func (f *File) readArea(slot uint32, area int) (Record, bool) {
	off := f.areaOffset(slot, area)
	fixed := make([]byte, recordFixedSize)
	if n, err := f.fd.ReadAt(fixed, off); n != recordFixedSize || (err != nil && !errors.Is(err, io.EOF)) {
		return Record{}, false
	}
	if [4]byte(fixed[0:4]) != recordMagic {
		return Record{}, false
	}
	userLen := int(binary.BigEndian.Uint16(fixed[10:12]))
	payloadLen := int(binary.BigEndian.Uint32(fixed[44:48]))
	total := RecordOverhead + userLen + payloadLen
	if total > int(f.header.AreaSize) {
		return Record{}, false
	}
	buf := make([]byte, total)
	copy(buf, fixed)
	if n, err := f.fd.ReadAt(buf[recordFixedSize:], off+int64(recordFixedSize)); n != total-recordFixedSize || (err != nil && !errors.Is(err, io.EOF)) {
		return Record{}, false
	}
	body := total - 4
	if crc32.Checksum(buf[:body], crcTable) != binary.BigEndian.Uint32(buf[body:]) {
		return Record{}, false
	}
	rec := Record{
		Seq:       binary.BigEndian.Uint32(buf[4:8]),
		Kind:      Kind(buf[8]),
		CacheThis: buf[9]&1 != 0,
	}
	copy(rec.Digest[:], buf[12:44])
	rec.User = append([]byte(nil), buf[recordFixedSize:recordFixedSize+userLen]...)
	rec.Payload = append([]byte(nil), buf[recordFixedSize+userLen:body]...)
	return rec, true
}
