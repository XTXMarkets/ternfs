// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package sessionstore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testID(b byte) ID {
	var id ID
	for i := range id {
		id[i] = b
	}
	return id
}

func record(seq uint32, kind Kind, payload string) Record {
	rec := Record{Seq: seq, Kind: kind, CacheThis: kind == KindSlot, User: []byte("uid:1000"), Payload: []byte(payload)}
	for i := range rec.Digest {
		rec.Digest[i] = byte(seq)
	}
	return rec
}

func sameRecord(a, b Record) bool {
	return a.Seq == b.Seq && a.Kind == b.Kind && a.CacheThis == b.CacheThis && a.Digest == b.Digest &&
		bytes.Equal(a.User, b.User) && bytes.Equal(a.Payload, b.Payload)
}

func openStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func reload(t *testing.T, s *Store, id ID) *File {
	t.Helper()
	files, problems, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	for _, f := range files {
		if f.ID() == id {
			t.Cleanup(func() { f.Close() })
			return f
		}
	}
	t.Fatalf("session %s not loaded", id)
	return nil
}

// A session's header and its slots' newest records come back after a
// reload, and the layout comes from the header, not from any constant.
func TestRoundTripFollowsHeader(t *testing.T) {
	s := openStore(t)
	id := testID(1)
	h := Header{SlotCount: 7, AreaSize: 300, Session: []byte("incarnation 42, 7 slots")}
	f, err := s.Create(id, h)
	if err != nil {
		t.Fatal(err)
	}
	for seq := uint32(1); seq <= 3; seq++ {
		if err := f.Write(6, record(seq, KindSlot, "reply for 6")); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Write(0, record(9, KindIntent, "rename a b")); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(7, record(1, KindSlot, "")); !errors.Is(err, ErrBadSlot) {
		t.Fatalf("slot 7 of 7: %v", err)
	}
	f.Close()

	g := reload(t, s, id)
	if gh := g.Header(); gh.SlotCount != 7 || gh.AreaSize != 300 || !bytes.Equal(gh.Session, h.Session) {
		t.Fatalf("header %+v", gh)
	}
	if rec, ok := g.Slot(6); !ok || !sameRecord(rec, record(3, KindSlot, "reply for 6")) {
		t.Fatalf("slot 6: %+v %v", rec, ok)
	}
	if rec, ok := g.Slot(0); !ok || !sameRecord(rec, record(9, KindIntent, "rename a b")) {
		t.Fatalf("slot 0: %+v %v", rec, ok)
	}
	for _, slot := range []uint32{1, 5, 7} {
		if _, ok := g.Slot(slot); ok {
			t.Fatalf("slot %d has a record", slot)
		}
	}
	if st, err := os.Stat(s.Path(id)); err != nil || st.Size() != int64(headerAreaSize+7*2*300) {
		t.Fatalf("file size %v %v", st, err)
	}
}

// A torn write damages only the area it writes, and the slot falls back to
// the record in the other area; two bad areas leave the slot empty.
func TestTornRecordFallsBackToPrevious(t *testing.T) {
	s := openStore(t)
	id := testID(2)
	f, err := s.Create(id, Header{SlotCount: 2, AreaSize: 256, Session: []byte("s")})
	if err != nil {
		t.Fatal(err)
	}
	for seq := uint32(1); seq <= 2; seq++ {
		if err := f.Write(1, record(seq, KindSlot, "reply")); err != nil {
			t.Fatal(err)
		}
	}
	f.Close()
	tear(t, f, 1, 2, KindSlot)
	g := reload(t, s, id)
	if rec, ok := g.Slot(1); !ok || rec.Seq != 1 {
		t.Fatalf("after a torn seq 2, slot 1 = %+v %v, want seq 1", rec, ok)
	}
	g.Close()
	tear(t, f, 1, 1, KindSlot)
	g = reload(t, s, id)
	if _, ok := g.Slot(1); ok {
		t.Fatal("slot with two bad areas has a record")
	}
}

// tear damages the slot area of f holding the record of seq and kind, as
// a write cut short leaves it.
func tear(t *testing.T, f *File, slot, seq uint32, kind Kind) {
	t.Helper()
	fd, err := os.OpenFile(f.Path(), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	for area := 0; area < areasPerSlot; area++ {
		off := f.areaOffset(slot, area)
		fixed := make([]byte, recordFixedSize)
		if _, err := fd.ReadAt(fixed, off); err != nil {
			t.Fatal(err)
		}
		if [4]byte(fixed[0:4]) == recordMagic && binary.BigEndian.Uint32(fixed[4:8]) == seq && Kind(fixed[8]) == kind {
			flip(t, f.Path(), off+int64(recordFixedSize))
			return
		}
	}
	t.Fatalf("no record of seq %d kind %d on slot %d", seq, kind, slot)
}

// Every write goes to the area that does not hold the slot's newest record,
// and at equal sequence numbers a completed request's record beats its
// intent. So a write torn at any point leaves the record a retransmission
// needs: the previous reply when a new request's record is torn, and the
// intent when the record of its completion is.
func TestTornWriteKeepsNewestRecord(t *testing.T) {
	type step struct {
		seq      uint32
		kind     Kind
		reload   bool // close and load the file again
		tooLarge bool // a write the area refuses
	}
	r := func(seq uint32) step { return step{seq: seq, kind: KindSlot} }
	i := func(seq uint32) step { return step{seq: seq, kind: KindIntent} }
	reloadStep := step{reload: true}
	for _, c := range []struct {
		name  string
		steps []step
		torn  step // the newest record when the last write is torn
	}{
		{"reply after reply", []step{r(1), r(2)}, r(1)},
		{"intent after reply", []step{r(1), i(2)}, r(1)},
		{"reply after intent", []step{r(1), i(2), r(2)}, i(2)},
		{"reply after intent and reply", []step{r(1), i(2), r(2), r(3)}, r(2)},
		{"intent after intent and reply", []step{r(1), i(2), r(2), i(3)}, r(2)},
		{"reply after intent, reloaded between", []step{r(1), i(2), reloadStep, r(2)}, i(2)},
		{"reply after reply, reloaded between", []step{r(1), r(2), reloadStep, r(3)}, r(2)},
		{"reply after intent across the wrap", []step{r(0xffffffff), i(0), r(0)}, i(0)},
		{"reply after a refused write", []step{r(1), {seq: 2, kind: KindSlot, tooLarge: true}, r(2)}, r(1)},
	} {
		for _, torn := range []bool{false, true} {
			name := c.name
			if torn {
				name += ", torn"
			}
			t.Run(name, func(t *testing.T) {
				s := openStore(t)
				id := testID(11)
				f, err := s.Create(id, Header{SlotCount: 1, AreaSize: 256})
				if err != nil {
					t.Fatal(err)
				}
				for _, st := range c.steps {
					switch {
					case st.reload:
						f.Close()
						f = reload(t, s, id)
					case st.tooLarge:
						if err := f.Write(0, record(st.seq, st.kind, string(make([]byte, 256)))); !errors.Is(err, ErrTooLarge) {
							t.Fatalf("oversized write: %v", err)
						}
					default:
						if err := f.Write(0, record(st.seq, st.kind, "payload")); err != nil {
							t.Fatal(err)
						}
					}
				}
				f.Close()
				last := c.steps[len(c.steps)-1]
				want := last
				if torn {
					tear(t, f, 0, last.seq, last.kind)
					want = c.torn
				}
				if rec, ok := reload(t, s, id).Slot(0); !ok || rec.Seq != want.seq || rec.Kind != want.kind {
					t.Fatalf("slot 0 = seq %d kind %d %v, want seq %d kind %d", rec.Seq, rec.Kind, ok, want.seq, want.kind)
				}
			})
		}
	}
}

func flip(t *testing.T, path string, off int64) {
	t.Helper()
	fd, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer fd.Close()
	b := make([]byte, 1)
	if _, err := fd.ReadAt(b, off); err != nil {
		t.Fatal(err)
	}
	b[0] ^= 0xff
	if _, err := fd.WriteAt(b, off); err != nil {
		t.Fatal(err)
	}
}

// Sequence numbers wrap; the newest record is the one that comes after.
func TestSequenceWrap(t *testing.T) {
	s := openStore(t)
	id := testID(3)
	f, err := s.Create(id, Header{SlotCount: 1, AreaSize: 256})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Write(0, record(0xffffffff, KindSlot, "last")); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(0, record(0, KindSlot, "wrapped")); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if rec, ok := reload(t, s, id).Slot(0); !ok || rec.Seq != 0 || string(rec.Payload) != "wrapped" {
		t.Fatalf("slot 0 after wrap: %+v %v", rec, ok)
	}
}

// A record larger than its area is refused before anything is written.
func TestTooLargeRecordIsRefused(t *testing.T) {
	s := openStore(t)
	f, err := s.Create(testID(4), Header{SlotCount: 1, AreaSize: 128})
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Write(0, record(1, KindSlot, string(make([]byte, 128-RecordOverhead-len("uid:1000")+1)))); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized record: %v", err)
	}
	if _, ok := f.Slot(0); ok {
		t.Fatal("refused record was kept")
	}
	if err := f.Write(0, record(1, KindSlot, string(make([]byte, 128-RecordOverhead-len("uid:1000"))))); err != nil {
		t.Fatalf("record filling the area exactly: %v", err)
	}
}

// A file whose header does not verify is reported and left in place; the
// other files still load. Names that are not session IDs are ignored.
func TestCorruptHeaderIsAProblem(t *testing.T) {
	s := openStore(t)
	good, err := s.Create(testID(5), Header{SlotCount: 1, AreaSize: 256, Session: []byte("good")})
	if err != nil {
		t.Fatal(err)
	}
	good.Close()
	bad, err := s.Create(testID(6), Header{SlotCount: 1, AreaSize: 256, Session: []byte("bad")})
	if err != nil {
		t.Fatal(err)
	}
	bad.Close()
	flip(t, s.Path(testID(6)), 20)
	if err := os.WriteFile(filepath.Join(s.Dir(), "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, problems, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].ID() != testID(5) {
		t.Fatalf("loaded %d files", len(files))
	}
	files[0].Close()
	if len(problems) != 1 || !errors.Is(problems[0].Err, ErrCorrupt) || problems[0].Path != s.Path(testID(6)) {
		t.Fatalf("problems %v", problems)
	}
	if _, err := os.Stat(s.Path(testID(6))); err != nil {
		t.Fatalf("corrupt file was not left in place: %v", err)
	}
}

// Create refuses a second file for the same session, an empty layout and
// a session record that does not fit the header.
func TestCreateValidation(t *testing.T) {
	s := openStore(t)
	f, err := s.Create(testID(7), Header{SlotCount: 1, AreaSize: 256})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := s.Create(testID(7), Header{SlotCount: 1, AreaSize: 256}); !errors.Is(err, os.ErrExist) {
		t.Fatalf("duplicate: %v", err)
	}
	if _, err := s.Create(testID(8), Header{SlotCount: 0, AreaSize: 256}); err == nil {
		t.Fatal("zero slots accepted")
	}
	if _, err := s.Create(testID(8), Header{SlotCount: 1, AreaSize: RecordOverhead - 1}); err == nil {
		t.Fatal("area too small for any record accepted")
	}
	if _, err := s.Create(testID(8), Header{SlotCount: 1, AreaSize: 256, Session: make([]byte, MaxSessionRecord+1)}); err == nil {
		t.Fatal("oversized session record accepted")
	}
	if _, err := os.Stat(s.Path(testID(8))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused create left a file: %v", err)
	}
}

// Remove deletes the file; removing twice is fine; a removed session does
// not load.
func TestRemove(t *testing.T) {
	s := openStore(t)
	f, err := s.Create(testID(9), Header{SlotCount: 1, AreaSize: 256})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(testID(9)); err != nil {
		t.Fatal(err)
	}
	files, _, err := s.Load()
	if err != nil || len(files) != 0 {
		t.Fatalf("after remove: %d files %v", len(files), err)
	}
}

// Different slots take concurrent writes; each slot's newest record is
// what reloads.
func TestConcurrentSlots(t *testing.T) {
	s := openStore(t)
	id := testID(10)
	f, err := s.Create(id, Header{SlotCount: 32, AreaSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for slot := uint32(0); slot < 32; slot++ {
		wg.Add(1)
		go func(slot uint32) {
			defer wg.Done()
			for seq := uint32(1); seq <= 50; seq++ {
				if err := f.Write(slot, record(seq+slot, KindSlot, "r")); err != nil {
					t.Error(err)
					return
				}
			}
		}(slot)
	}
	wg.Wait()
	f.Close()
	g := reload(t, s, id)
	for slot := uint32(0); slot < 32; slot++ {
		if rec, ok := g.Slot(slot); !ok || rec.Seq != 50+slot {
			t.Fatalf("slot %d: %+v %v", slot, rec, ok)
		}
	}
}

// A file of another format version is a problem, left in place: its records
// may mean something else.
func TestOtherVersionIsAProblem(t *testing.T) {
	s := openStore(t)
	f, err := s.Create(testID(9), Header{SlotCount: 1, AreaSize: 256, Session: []byte("s")})
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	path := s.Path(testID(9))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(data[4:6], formatVersion-1)
	off := headerFixedSize + 1
	binary.BigEndian.PutUint32(data[off:], crc32.Checksum(data[:off], crcTable))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	files, problems, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 || len(problems) != 1 || !errors.Is(problems[0].Err, ErrCorrupt) {
		t.Fatalf("files %d, problems %v", len(files), problems)
	}
}
