// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package clientrecords

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
)

var (
	owner     = []byte("Linux NFSv4.2 client-a")
	principal = Principal{Flavor: 1, Body: "uid 1234"}
	boot1     = [8]byte{1}
	boot2     = [8]byte{2}
	errCrash  = errors.New("crash")
)

// crashNone is a crash before the transition starts.
const crashNone crashPoint = "before"

func openRecords(t *testing.T, dir string) *Records {
	t.Helper()
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func confirm(t *testing.T, r *Records, boot [8]byte) Record {
	t.Helper()
	rec, err := r.Confirm(owner, boot, principal, NewIncarnation())
	check(t, err)
	return rec
}

func clientFiles(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, staging.ClientsDirName))
	check(t, err)
	return entries
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// outcome is an identity's state after a restart.
type outcome struct {
	present bool    // a confirmed, live record
	boot    [8]byte // its verifier
}

// Every update is a crash point before its rename, with the temporary
// synced, and after its rename or unlink, before the directory is synced.
// A crash at either, or before the update starts, leaves a directory Open
// loads as the state before or after the update. The process that failed
// to sync still sees the state before the update, so it never acts on one
// it reported failed.
func TestCrashPoints(t *testing.T) {
	cases := []struct {
		name          string
		remove        bool
		setup         func(*testing.T, *Records) Incarnation
		do            func(*Records, Incarnation) error
		before, after outcome
	}{{
		name:  "confirm new client",
		setup: func(*testing.T, *Records) Incarnation { return 0 },
		do: func(r *Records, _ Incarnation) error {
			_, err := r.Confirm(owner, boot1, principal, NewIncarnation())
			return err
		},
		before: outcome{},
		after:  outcome{present: true, boot: boot1},
	}, {
		name: "confirm reboot",
		setup: func(t *testing.T, r *Records) Incarnation {
			return confirm(t, r, boot1).Incarnation
		},
		do: func(r *Records, _ Incarnation) error {
			_, err := r.Confirm(owner, boot2, principal, NewIncarnation())
			return err
		},
		before: outcome{present: true, boot: boot1},
		after:  outcome{present: true, boot: boot2},
	}, {
		name:   "remove",
		remove: true,
		setup: func(t *testing.T, r *Records) Incarnation {
			return confirm(t, r, boot1).Incarnation
		},
		do:     func(r *Records, id Incarnation) error { return r.Remove(id) },
		before: outcome{present: true, boot: boot1},
		after:  outcome{},
	}}
	for _, tc := range cases {
		points := []crashPoint{crashNone, crashSynced, crashRenamed}
		if tc.remove {
			points = []crashPoint{crashNone, crashRemoved}
		}
		for _, point := range points {
			t.Run(tc.name+"/"+string(point), func(t *testing.T) {
				dir := t.TempDir()
				r := openRecords(t, dir)
				id := tc.setup(t, r)
				before := r.List()
				fired := false
				r.crash = func(p crashPoint) error {
					if p != point {
						return nil
					}
					fired = true
					return errCrash
				}
				if point != crashNone {
					if err := tc.do(r, id); !fired || !errors.Is(err, errCrash) {
						t.Fatalf("got %v, want the crash", err)
					}
				}
				if point == crashRenamed || point == crashRemoved {
					// The update was not reported durable, so the
					// process still sees the state before it.
					if got := r.List(); !reflect.DeepEqual(got, before) {
						t.Fatalf("in-memory records after an unsynced update %+v, want %+v", got, before)
					}
					// The directory may or may not hold the change, so
					// the process trusts nothing more.
					if _, err := r.Confirm([]byte("other"), boot1, principal, NewIncarnation()); !errors.Is(err, errCrash) {
						t.Fatalf("update after an unsynced rename: got %v", err)
					}
				}

				r = openRecords(t, dir)
				entries := clientFiles(t, dir)
				for _, e := range entries {
					if strings.Contains(e.Name(), ".tmp-") {
						t.Fatalf("temporary %s left after Open", e.Name())
					}
				}
				var got outcome
				if rec, ok := r.Lookup(HashOwner(owner)); ok {
					if _, live := r.Live(rec.Incarnation); !live {
						t.Fatalf("record not live: %+v", rec)
					}
					got.present, got.boot = true, rec.Verifier
				}
				want := tc.after
				if point == crashNone || point == crashSynced {
					want = tc.before
				}
				if got != want {
					t.Fatalf("got %+v, want %+v", got, want)
				}
				if len(r.List()) != len(clientFiles(t, dir)) {
					t.Fatal("records and files differ after a restart")
				}
			})
		}
	}
}

// A replaced or expired incarnation is never live again. The same client
// boot returning after its expiry gets a new incarnation with the same
// recovery key.
func TestIncarnationsAreNotRevived(t *testing.T) {
	r := openRecords(t, t.TempDir())
	first := confirm(t, r, boot1)
	rebooted := confirm(t, r, boot2)
	if rebooted.Incarnation == first.Incarnation || r.InUse(first.Incarnation) {
		t.Fatalf("reboot: %+v after %+v", rebooted, first)
	}
	if _, live := r.Live(first.Incarnation); live {
		t.Fatal("replaced incarnation live")
	}
	if err := r.Remove(first.Incarnation); !errors.Is(err, ErrNotCurrent) {
		t.Fatalf("expire replaced incarnation: %v", err)
	}
	check(t, r.Remove(rebooted.Incarnation))
	if _, live := r.Live(rebooted.Incarnation); live {
		t.Fatal("expired incarnation live")
	}
	back := confirm(t, r, boot2)
	if back.Incarnation == rebooted.Incarnation {
		t.Fatalf("return after expiry: %+v", back)
	}
	if back.RecoveryKey() != rebooted.RecoveryKey() {
		t.Fatal("same boot, different recovery key")
	}
	if back.RecoveryKey() == first.RecoveryKey() {
		t.Fatal("different boots, same recovery key")
	}
	other := back
	other.Principal.Body = "uid 4321"
	if other.RecoveryKey() == back.RecoveryKey() {
		t.Fatal("different principals, same recovery key")
	}
}

// An earlier build wrote a record expired before removing it, so a crash
// could leave one. Open removes it rather than load it as live.
func TestOpenRemovesExpiredRecords(t *testing.T) {
	dir := t.TempDir()
	r := openRecords(t, dir)
	rec := confirm(t, r, boot1)
	path := filepath.Join(dir, staging.ClientsDirName, rec.Hash().String())
	data, err := os.ReadFile(path)
	check(t, err)
	data = append(data[:len(data)-1], []byte(`,"expired":true}`)...)
	check(t, os.WriteFile(path, data, 0600))
	r = openRecords(t, dir)
	if _, live := r.Live(rec.Incarnation); live {
		t.Fatal("expired record loaded as live")
	}
	if n := len(clientFiles(t, dir)); n != 0 {
		t.Fatalf("%d files left in clients/", n)
	}
}

// Open refuses clients/ content it did not write rather than guess what a
// client's state was.
func TestOpenRefusesForeignContent(t *testing.T) {
	valid := func(t *testing.T) (string, string) {
		dir := t.TempDir()
		r := openRecords(t, dir)
		rec := confirm(t, r, boot1)
		return dir, filepath.Join(dir, staging.ClientsDirName, rec.Hash().String())
	}
	cases := map[string]func(t *testing.T, dir, path string){
		"undecodable record": func(t *testing.T, _, path string) {
			check(t, os.WriteFile(path, []byte("{"), 0600))
		},
		"record under another identity's name": func(t *testing.T, dir, path string) {
			data, err := os.ReadFile(path)
			check(t, err)
			other := HashOwner([]byte("other")).String()
			check(t, os.WriteFile(filepath.Join(dir, staging.ClientsDirName, other), data, 0600))
		},
		"unexpected file": func(t *testing.T, dir, _ string) {
			check(t, os.WriteFile(filepath.Join(dir, staging.ClientsDirName, "notes"), nil, 0600))
		},
		"directory": func(t *testing.T, dir, _ string) {
			check(t, os.Mkdir(filepath.Join(dir, staging.ClientsDirName, HashOwner([]byte("d")).String()), 0700))
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			dir, path := valid(t)
			corrupt(t, dir, path)
			if _, err := Open(dir); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("got %v, want ErrCorrupt", err)
			}
		})
	}
}
