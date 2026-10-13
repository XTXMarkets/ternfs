// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux && libnfs

package libnfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func (s *nfsTestServer) published(t *testing.T, name, want string) {
	t.Helper()
	data, err := s.ReadFile(name)
	if err != nil || string(data) != want {
		t.Fatalf("published %s = %q, %v; want %q", name, data, err, want)
	}
}

// Inode IDs in the inspector's JSON are hexadecimal strings. Decode only the
// fields asserted here, leaving the complete report in the run artifacts.
type recoveryReport struct {
	// LeaseTime is a Go duration string such as "90s".
	LeaseTime  string   `json:"lease_time"`
	Problems   []string `json:"problems"`
	Identities []struct {
		Problems     []string              `json:"problems"`
		Incarnations []recoveryIncarnation `json:"incarnations"`
	} `json:"identities"`
}

type recoveryIncarnation struct {
	ClientID string   `json:"clientid"`
	Roles    []string `json:"roles"`
	Usable   bool     `json:"usable"`
	Problems []string `json:"problems"`
	Opens    []struct {
		StateID string          `json:"stateid"`
		Staging json.RawMessage `json:"staging"`
	} `json:"opens"`
	// Staging lists this clientid's sidecars in the staging directory.
	Staging []struct {
		FileID  string `json:"file_id"`
		Retired bool   `json:"retired"`
	} `json:"staging"`
}

func (s *nfsTestServer) inspect(t *testing.T, c *libnfsProcessClient) *recoveryReport {
	t.Helper()
	// Use the actual binary's inspector so its decoding matches the server.
	args := []string{"inspect", "-identity", c.identity, "-staging", s.Staging,
		"-registry", s.Suite.Registry, "-json"}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.Suite.Binary, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("inspect: %v\n%s", err, stderr.Bytes())
	}
	var report recoveryReport
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Problems) != 0 || len(report.Identities) != 1 {
		t.Fatalf("unexpected inspector report: %s", out)
	}
	for _, identity := range report.Identities {
		if len(identity.Problems) != 0 {
			t.Fatalf("identity problems: %v", identity.Problems)
		}
		for _, incarnation := range identity.Incarnations {
			if len(incarnation.Problems) != 0 {
				t.Fatalf("incarnation problems: %v", incarnation.Problems)
			}
		}
	}
	s.inspections++
	if err := os.WriteFile(filepath.Join(s.Dir, fmt.Sprintf("%s-inspect-%03d.json", c.identity, s.inspections)), out, 0600); err != nil {
		t.Fatal(err)
	}
	return &report
}

func recoveryConfirmed(t *testing.T, report *recoveryReport) recoveryIncarnation {
	t.Helper()
	for _, incarnation := range report.Identities[0].Incarnations {
		for _, role := range incarnation.Roles {
			if role == "confirmed" {
				return incarnation
			}
		}
	}
	t.Fatal("no confirmed client incarnation")
	return recoveryIncarnation{}
}

// requireRecoveryError requires r to have failed with one of statuses.
func requireRecoveryError(t *testing.T, r libnfsReply, statuses ...string) {
	t.Helper()
	for _, status := range statuses {
		if r.Error != "" && strings.Contains(r.Error, status) {
			t.Logf("libnfs reported %s (errno %d)", r.Error, r.Errno)
			return
		}
	}
	t.Fatalf("got error %q (errno %d), want one of %v", r.Error, r.Errno, statuses)
}

// stagedWriter is the inspector's report of one open's staging, without the
// fields a checkpoint may change (times, allocation).
type stagedWriter struct {
	FileID      string `json:"file_id"`
	DataPresent bool   `json:"data_present"`
	MetaPresent bool   `json:"meta_present"`
	Size        int64  `json:"size"`
	DirID       string `json:"dir_id"`
	FileName    string `json:"file_name"`
	RecoveryKey string `json:"recovery_key"`
	StateID     string `json:"stateid"`
	ClientID    string `json:"clientid"`
	OpenOwner   string `json:"open_owner"`
	OwnerKnown  bool   `json:"owner_known"`
	ReadOnly    bool   `json:"read_only"`
	Retired     bool   `json:"retired"`
	Unlinked    bool   `json:"unlinked"`
	Guarded     bool   `json:"guarded"`
	Exclusive   bool   `json:"exclusive"`
	Verifier    string `json:"verifier"`
	BaseID      string `json:"base_id"`
	BaseSize    uint64 `json:"base_size"`
	Dirty       []struct {
		Start uint64 `json:"start"`
		End   uint64 `json:"end"`
	} `json:"dirty"`
	Problems []string `json:"problems"`
}

// retainedWriter checks that the incarnation's only open still has its
// staging, and reads its synced range [0, len(synced)) from the one staging
// data file in the server's staging directory. The file is tied to the
// inspector's entry by being the only one there and by its size.
func (s *nfsTestServer) retainedWriter(t *testing.T, inc recoveryIncarnation, synced string) (string, stagedWriter) {
	t.Helper()
	if len(inc.Opens) != 1 || inc.Opens[0].Staging == nil {
		t.Fatalf("expected one open with staging: %+v", inc)
	}
	var w stagedWriter
	if err := json.Unmarshal(inc.Opens[0].Staging, &w); err != nil {
		t.Fatal(err)
	}
	if !w.DataPresent || !w.MetaPresent || w.Retired || w.Unlinked || w.Guarded || len(w.Problems) != 0 ||
		w.ReadOnly || !w.OwnerKnown || w.RecoveryKey == "" || w.StateID != inc.Opens[0].StateID {
		t.Fatalf("staging not intact for its open %s: %+v", inc.Opens[0].StateID, w)
	}
	covered := false
	for _, r := range w.Dirty {
		covered = covered || r.Start == 0 && r.End >= uint64(len(synced))
	}
	if !covered {
		t.Fatalf("synced range not recorded as written: %+v", w.Dirty)
	}
	files, err := filepath.Glob(filepath.Join(s.Staging, "*.staging"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("staging data files %v, want exactly the writer's", files)
	}
	f, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.Size() != w.Size {
		t.Fatalf("staging data file %s does not match the inspector's entry: %v %v", files[0], info, err)
	}
	data := make([]byte, len(synced))
	if _, err := f.ReadAt(data, 0); err != nil || string(data) != synced {
		t.Fatalf("staged bytes %q, %v; want %q", data, err, synced)
	}
	return inc.Opens[0].StateID, w
}

// Sessions end with the nfsd process (canonical r3 §6). The libnfs build
// these tests were run with (the Makefile's libnfs-8.0.0 pin, whose source
// declares version 18.0.0) does not recover a session: after a restart it
// reconnects and sends SEQUENCE and BIND_CONN_TO_SESSION on its old session,
// gets NFS4ERR_BADSESSION and reports it, without EXCHANGE_ID or
// CREATE_SESSION. Each restart case below records that, and checks that
// nothing published or staged is lost and that a new libnfs context works. A
// client that re-registers recovers; these cases do not show that.
func runLibnfsRecovery(t *testing.T, suite *nfsTestSuite) {
	t.Log("libnfs=8.0.0 timeout=5s reconnects=2")
	t.Run("OpenCloseAcrossRestart", func(t *testing.T) {
		s := suite.server(t)
		c := s.client(t)
		for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
			h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
			c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("published")})
			c.must(t, libnfsRequest{Op: "close", Handle: h})
			s.published(t, "file", "published")
			s.restart(t, sig)
			requireRecoveryError(t, c.call(t, libnfsRequest{Op: "open", Path: "/" + s.Name + "/file", Flags: os.O_RDONLY}), "NFS4ERR_BADSESSION")
			c.process.stop(t, syscall.SIGKILL)
			c = s.client(t)
			h = c.open(t, s, "file", os.O_RDONLY)
			c.read(t, h, "published")
			c.must(t, libnfsRequest{Op: "close", Handle: h})
		}
	})
	t.Run("OperationsWhileServerDown", func(t *testing.T) {
		for _, operation := range []string{"open", "close"} {
			t.Run(operation, func(t *testing.T) {
				s := suite.server(t)
				c := s.client(t)
				h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
				c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("published")})
				c.must(t, libnfsRequest{Op: "close", Handle: h})
				req := libnfsRequest{Op: "open", Path: "/" + s.Name + "/file", Flags: os.O_RDONLY}
				if operation == "close" {
					h = c.open(t, s, "file", os.O_RDWR)
					c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("unpublished")})
					c.must(t, libnfsRequest{Op: "sync", Handle: h})
					req = libnfsRequest{Op: "close", Handle: h}
				}
				s.process.stop(t, syscall.SIGKILL)
				r := c.call(t, req)
				if r.Error == "" || r.Errno != int(syscall.EIO) {
					t.Fatalf("%s while server down: %+v; expected EIO", operation, r)
				}
				t.Logf("libnfs %s while down: %s (errno %d)", operation, r.Error, r.Errno)
				s.start(t)
				s.published(t, "file", "published")
				// Exhausting reconnect attempts leaves this libnfs context
				// unusable even when the server returns.
				r = c.call(t, libnfsRequest{Op: "open", Path: "/" + s.Name + "/file", Flags: os.O_RDONLY})
				if r.Error == "" || r.Errno != int(syscall.EIO) {
					t.Fatalf("exhausted libnfs context: %+v; expected EIO", r)
				}
				c.process.stop(t, syscall.SIGKILL)
				fresh := s.client(t)
				h = fresh.open(t, s, "file", os.O_RDONLY)
				fresh.read(t, h, "published")
				fresh.must(t, libnfsRequest{Op: "close", Handle: h})
			})
		}
	})
	t.Run("CloseSyncedWriterAfterRestart", func(t *testing.T) {
		for _, tc := range []struct {
			name, base string
			exclusive  bool
		}{
			{"new", "", false},
			{"replacement", "old tail", false},
			{"exclusive", "", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := suite.server(t)
				c := s.client(t)
				if tc.base != "" {
					h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
					c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte(tc.base)})
					c.must(t, libnfsRequest{Op: "close", Handle: h})
				}
				flags := os.O_CREATE | os.O_RDWR
				if tc.exclusive {
					flags |= os.O_EXCL
				}
				h := c.open(t, s, "file", flags)
				c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("new")})
				c.must(t, libnfsRequest{Op: "sync", Handle: h})
				s.published(t, "file", tc.base)
				before := recoveryConfirmed(t, s.inspect(t, c))
				if !before.Usable {
					t.Fatalf("writer not durably registered: %+v", before)
				}
				stateID, staged := s.retainedWriter(t, before, "new")
				if staged.Exclusive != tc.exclusive || (tc.base == "") != (staged.BaseSize == 0) {
					t.Fatalf("staging does not describe this writer: %+v", staged)
				}
				// retained checks that the same staging, tied to the original
				// open, still holds the synced bytes.
				retained := func(when string) recoveryIncarnation {
					t.Helper()
					inc := recoveryConfirmed(t, s.inspect(t, c))
					if inc.ClientID != before.ClientID {
						t.Fatalf("%s: client re-registered: %s -> %s", when, before.ClientID, inc.ClientID)
					}
					gotState, got := s.retainedWriter(t, inc, "new")
					if gotState != stateID || !reflect.DeepEqual(got, staged) {
						t.Fatalf("%s: staging changed:\n%s %+v\nwant\n%s %+v", when, gotState, got, stateID, staged)
					}
					return inc
				}
				s.restart(t, syscall.SIGKILL)
				retained("after restart")
				// The tested libnfs cannot close on its pre-restart session,
				// so the synced data is not published. It is not lost either:
				// the original open's staging stays, with its bytes, for a
				// client that recovers it (old stateid or CLAIM_FH) or for
				// lease expiry to retire.
				requireRecoveryError(t, c.call(t, libnfsRequest{Op: "close", Handle: h}), "NFS4ERR_BADSESSION")
				s.published(t, "file", tc.base)
				retained("after the rejected CLOSE")
				t.Logf("synced writer retained, unpublished: open %s, staging %s", stateID, staged.FileID)
				c.process.stop(t, syscall.SIGKILL)
				fresh := s.client(t)
				h = fresh.open(t, s, "file", os.O_RDONLY)
				fresh.read(t, h, tc.base)
				fresh.must(t, libnfsRequest{Op: "close", Handle: h})
			})
		}
	})
	t.Run("OldReadHandleAfterRestart", func(t *testing.T) {
		s := suite.server(t)
		c := s.client(t)
		h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
		c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("retained")})
		c.must(t, libnfsRequest{Op: "close", Handle: h})
		h = c.open(t, s, "file", os.O_RDONLY)
		s.restart(t, syscall.SIGKILL)
		// The SEQUENCE in front of the request fails before the read
		// stateid, which is not recovered, is examined.
		requireRecoveryError(t, c.call(t, libnfsRequest{Op: "read", Handle: h}), "NFS4ERR_BADSESSION")
		// The tested libnfs then sends CLOSE either on the old session, which
		// gets NFS4ERR_BADSESSION, or after dropping it, without SEQUENCE,
		// which gets NFS4ERR_OP_NOT_IN_SESSION. Both were observed. Neither
		// says anything about the read stateid.
		requireRecoveryError(t, c.call(t, libnfsRequest{Op: "close", Handle: h}),
			"NFS4ERR_BADSESSION", "NFS4ERR_OP_NOT_IN_SESSION")
		c.process.stop(t, syscall.SIGKILL)
		c = s.client(t)
		h = c.open(t, s, "file", os.O_RDONLY)
		c.read(t, h, "retained")
		c.must(t, libnfsRequest{Op: "close", Handle: h})
	})
	t.Run("LeaseExpiration", func(t *testing.T) {
		if testing.Short() {
			t.Skip("uses the server's real 90-second lease and periodic cleanup")
		}
		s := suite.server(t)
		c := s.client(t)
		h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
		c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("abandoned")})
		c.must(t, libnfsRequest{Op: "sync", Handle: h})
		before := recoveryConfirmed(t, s.inspect(t, c))
		if !before.Usable || len(before.Opens) != 1 {
			t.Fatalf("expected one live writer: %+v", before)
		}
		dead := s.client(t)
		abandoned := dead.open(t, s, "dead-client", os.O_CREATE|os.O_EXCL|os.O_RDWR)
		dead.must(t, libnfsRequest{Op: "write", Handle: abandoned, Data: []byte("client died")})
		dead.must(t, libnfsRequest{Op: "sync", Handle: abandoned})
		if inc := recoveryConfirmed(t, s.inspect(t, dead)); !inc.Usable || len(inc.Opens) != 1 {
			t.Fatalf("expected one live writer before killing client: %+v", inc)
		}
		dead.process.stop(t, syscall.SIGKILL)
		t.Logf("one libnfs client idle, one killed; waiting for lease expiry and server sweep")
		// The helper blocks on its command pipe: no calls service libnfs or renew
		// its lease. The inspector is read-only and cannot keep the lease alive.
		lease, err := time.ParseDuration(s.inspect(t, c).LeaseTime)
		if err != nil {
			t.Fatal(err)
		}
		if lease <= 0 {
			t.Fatalf("invalid lease duration: %s", lease)
		}
		// Expiry revokes the opens but retires the staged bytes in place:
		// both sidecars stay on disk, marked retired, until the original
		// client boot reclaims them or an administrator removes them.
		retiredStaging := func(inc recoveryIncarnation) bool {
			return len(inc.Staging) == 1 && inc.Staging[0].Retired
		}
		retainedStaging := func() bool {
			staging, err := filepath.Glob(filepath.Join(s.Staging, "*.staging"))
			if err != nil {
				t.Fatal(err)
			}
			sidecars, err := filepath.Glob(filepath.Join(s.Staging, "*.meta"))
			if err != nil {
				t.Fatal(err)
			}
			return len(staging) == 2 && len(sidecars) == 2
		}
		// The server's expiry horizon from the clients' last requests, which
		// precede this wait (canonical r3 §§8-9), for a healthy backend:
		//   lease      the logical lease's quiet period after the last reply;
		//   1.5*lease  the durable lease the refresher wrote last, during
		//              that period (clientstore.LeaseWriteTerm = lease + lease/2);
		//   0.5*lease  the sweeper's tick, which makes the decision and
		//              retires the staging;
		//   0.5*lease  margin for scheduling, the inspector's polling and
		//              the sweep's own run time.
		// It is not a bound under arbitrarily slow backend calls.
		expiryHorizon := lease + lease*3/2 + lease/2 + lease/2
		waitForNFSTest(t, "lease expiration and staging retirement", expiryHorizon, func() bool {
			c.process.alive(t)
			s.process.alive(t)
			inc := recoveryConfirmed(t, s.inspect(t, c))
			deadInc := recoveryConfirmed(t, s.inspect(t, dead))
			return !inc.Usable && len(inc.Opens) == 0 && retiredStaging(inc) &&
				!deadInc.Usable && len(deadInc.Opens) == 0 && retiredStaging(deadInc) &&
				retainedStaging()
		})
		// An expired clientid ends its sessions: the SEQUENCE in front of
		// any later request fails before the stateid is examined.
		requireRecoveryError(t, c.call(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("late")}), "NFS4ERR_BADSESSION")
		if late := c.call(t, libnfsRequest{Op: "close", Handle: h}); late.Error == "" {
			t.Fatal("late close succeeded after lease expiry")
		} else {
			t.Logf("libnfs reported %s (errno %d)", late.Error, late.Errno)
		}
		s.published(t, "file", "")
		s.published(t, "dead-client", "")
		fresh := s.client(t)
		h = fresh.open(t, s, "file", os.O_RDWR)
		fresh.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("new client")})
		fresh.must(t, libnfsRequest{Op: "close", Handle: h})
		s.published(t, "file", "new client")
		// Publishing through a new client neither reclaims nor removes the
		// retired staging at the same name.
		if !retainedStaging() {
			t.Fatal("retired staging was removed by an unrelated publication")
		}
	})
}
