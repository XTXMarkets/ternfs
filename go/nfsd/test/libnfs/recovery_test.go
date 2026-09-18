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

func requireRecoveryError(t *testing.T, r libnfsReply, status string) {
	t.Helper()
	if r.Error == "" || !strings.Contains(r.Error, status) {
		t.Fatalf("got error %q (errno %d), want %s", r.Error, r.Errno, status)
	}
	t.Logf("libnfs reported %s (errno %d)", r.Error, r.Errno)
}

func runLibnfsRecovery(t *testing.T, suite *nfsTestSuite) {
	t.Log("libnfs=7.0.2 timeout=5s reconnects=2")
	t.Run("OpenCloseAcrossRestart", func(t *testing.T) {
		s := suite.server(t)
		c := s.client(t)
		for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
			h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
			c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("published")})
			c.must(t, libnfsRequest{Op: "close", Handle: h})
			s.published(t, "file", "published")
			s.restart(t, sig)
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
		for _, tc := range []struct{ name, base, want string }{
			{"new", "", "new"},
			{"replacement", "old tail", "new tail"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				s := suite.server(t)
				c := s.client(t)
				if tc.base != "" {
					h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
					c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte(tc.base)})
					c.must(t, libnfsRequest{Op: "close", Handle: h})
				}
				h := c.open(t, s, "file", os.O_CREATE|os.O_RDWR)
				c.must(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("new")})
				c.must(t, libnfsRequest{Op: "sync", Handle: h})
				s.published(t, "file", tc.base)
				before := recoveryConfirmed(t, s.inspect(t, c))
				if !before.Usable || len(before.Opens) != 1 || before.Opens[0].Staging == nil {
					t.Fatalf("writer not durably registered: %+v", before)
				}
				s.restart(t, syscall.SIGKILL)
				c.must(t, libnfsRequest{Op: "close", Handle: h})
				s.published(t, "file", tc.want)
				after := recoveryConfirmed(t, s.inspect(t, c))
				if before.ClientID != after.ClientID {
					t.Fatalf("client re-registered during close recovery: %s -> %s", before.ClientID, after.ClientID)
				}
				if len(after.Opens) != 0 {
					t.Fatalf("open markers remain after recovered close: %+v", after.Opens)
				}
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
		requireRecoveryError(t, c.call(t, libnfsRequest{Op: "read", Handle: h}), "NFS4ERR_BAD_STATEID")
		requireRecoveryError(t, c.call(t, libnfsRequest{Op: "close", Handle: h}), "NFS4ERR_BAD_STATEID")
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
		abandoned := dead.open(t, s, "dead-client", os.O_CREATE|os.O_RDWR)
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
		waitForNFSTest(t, "lease expiration and staging retirement", 3*lease, func() bool {
			c.process.alive(t)
			s.process.alive(t)
			inc := recoveryConfirmed(t, s.inspect(t, c))
			deadInc := recoveryConfirmed(t, s.inspect(t, dead))
			return !inc.Usable && len(inc.Opens) == 0 && retiredStaging(inc) &&
				!deadInc.Usable && len(deadInc.Opens) == 0 && retiredStaging(deadInc) &&
				retainedStaging()
		})
		requireRecoveryError(t, c.call(t, libnfsRequest{Op: "write", Handle: h, Data: []byte("late")}), "NFS4ERR_EXPIRED")
		// libnfs commits before it closes. COMMIT carries no stateid, and on
		// retired staging it fails with IO because the descriptor was released
		// at expiry; a CLOSE which reaches the server fails with EXPIRED.
		if late := c.call(t, libnfsRequest{Op: "close", Handle: h}); late.Error == "" ||
			!strings.Contains(late.Error, "NFS4ERR_EXPIRED") && !strings.Contains(late.Error, "NFS4ERR_IO") {
			t.Fatalf("late close: got error %q (errno %d), want NFS4ERR_EXPIRED or NFS4ERR_IO", late.Error, late.Errno)
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
