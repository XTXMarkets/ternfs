// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"xtx/ternfs/msgs"
)

const oneRuleJSON = `[
  {
    "name": "example_delete",
    "tag": "DELETE",
    "applies_to": "file",
    "include_path_match": ["^/test/.*x/.*"],
    "exclude_path_match": [".*/skip/.*"],
    "file_suffix_match": ["\\.dat$"],
    "atime_days": 30,
    "mtime_days": 30,
    "size_bytes": 1048576,
    "extended_retention_bitfield_match": null
  }
]`

func mustLoad(t *testing.T, data string) []*Rule {
	t.Helper()
	rules, err := LoadRules([]byte(data))
	if err != nil {
		t.Fatalf("LoadRules: %v", err)
	}
	return rules
}

func TestLoadRules_RequiresTag(t *testing.T) {
	const j = `[{"name":"x","tag":"","include_path_match":[],"exclude_path_match":[],"file_suffix_match":[],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":null}]`
	if _, err := LoadRules([]byte(j)); err == nil {
		t.Fatal("expected empty-tag rule to be rejected")
	}
}

func TestLoadRules_AppliesToDefaultsToFile(t *testing.T) {
	const j = `[{"name":"x","tag":"DELETE","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":null}]`
	rules := mustLoad(t, j)
	if rules[0].AppliesTo != "file" {
		t.Fatalf("default applies_to = %q, want %q", rules[0].AppliesTo, "file")
	}
}

func TestLoadRules_AppliesToInvalid(t *testing.T) {
	const j = `[{"name":"x","tag":"DELETE","applies_to":"sock","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":null}]`
	if _, err := LoadRules([]byte(j)); err == nil {
		t.Fatal("expected invalid applies_to to be rejected")
	}
}

func TestRulePredicate_HappyPath(t *testing.T) {
	rules := mustLoad(t, oneRuleJSON)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	older := now - msgs.TernTime(60*24*time.Hour)
	if !rules[0].Matches("/test/foo/x/bar/file.dat", 2<<20, older, older, now) {
		t.Fatal("expected match")
	}
}

func TestRulePredicate_RejectsOnEachDimension(t *testing.T) {
	rules := mustLoad(t, oneRuleJSON)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	older := now - msgs.TernTime(60*24*time.Hour)
	tooFresh := now - msgs.TernTime(1*24*time.Hour)
	good := "/test/foo/x/bar/file.dat"
	cases := []struct {
		name         string
		path         string
		size         uint64
		atime, mtime msgs.TernTime
		wantFire     bool
	}{
		{"happy", good, 2 << 20, older, older, true},
		{"include_miss", "/etc/passwd", 2 << 20, older, older, false},
		{"exclude_hit", "/test/foo/x/skip/file.dat", 2 << 20, older, older, false},
		{"suffix_miss", "/test/foo/x/bar/file.txt", 2 << 20, older, older, false},
		{"too_small", good, 1024, older, older, false},
		{"too_fresh_atime", good, 2 << 20, tooFresh, older, false},
		{"too_fresh_mtime", good, 2 << 20, older, tooFresh, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rules[0].Matches(tc.path, tc.size, tc.atime, tc.mtime, now)
			if got != tc.wantFire {
				t.Fatalf("Matches=%v want %v", got, tc.wantFire)
			}
		})
	}
}

func TestRulePredicate_ExtendedRetentionBitfield(t *testing.T) {
	const j = `[
      {"name":"er_required","tag":"NOOP","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":true},
      {"name":"er_forbidden","tag":"NOOP","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":false}
    ]`
	rules := mustLoad(t, j)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	// Pick a seconds-aligned mtime whose low 10 bits match the sentinel.
	baseSec := int64((uint64(1_600_000_000) & ^extendedRetentionBitmask) | extendedRetentionBitfield)
	withER := msgs.MakeTernTime(time.Unix(baseSec, 0))
	noER := msgs.MakeTernTime(time.Unix(baseSec+1, 0))

	if !rules[0].Matches("x", 0, now, withER, now) {
		t.Fatal("er_required should match when bitfield is present")
	}
	if rules[0].Matches("x", 0, now, noER, now) {
		t.Fatal("er_required should not match without bitfield")
	}
	if rules[1].Matches("x", 0, now, withER, now) {
		t.Fatal("er_forbidden should not match when bitfield is present")
	}
	if !rules[1].Matches("x", 0, now, noER, now) {
		t.Fatal("er_forbidden should match when bitfield is absent")
	}
}

func TestRulePredicate_InfThresholdNeverFires(t *testing.T) {
	const j = `[
      {"name":"disabled","tag":"NOOP","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":"inf","mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":null}
    ]`
	rules := mustLoad(t, j)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	ancient := msgs.MakeTernTime(time.Unix(0, 0))
	if rules[0].Matches("x", 0, ancient, ancient, now) {
		t.Fatal("atime_days=inf rule should never match")
	}
}

func TestMaybeMatchesByCreationTime_RejectsTooYoung(t *testing.T) {
	rules := mustLoad(t, oneRuleJSON)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	young := now - msgs.TernTime(10*24*time.Hour)
	if rules[0].MaybeMatchesByCreationTime("/test/foo/x/bar/file.dat", young, now) {
		t.Fatal("creation-time prefilter should reject when creationTime is too young for the rule")
	}
}

func TestMaybeMatchesByCreationTime_AcceptsOldEnough(t *testing.T) {
	rules := mustLoad(t, oneRuleJSON)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	old := now - msgs.TernTime(60*24*time.Hour)
	if !rules[0].MaybeMatchesByCreationTime("/test/foo/x/bar/file.dat", old, now) {
		t.Fatal("creation-time prefilter should keep the candidate when creationTime is old enough")
	}
}

func TestMaybeMatchesByCreationTime_RegexStillFilters(t *testing.T) {
	rules := mustLoad(t, oneRuleJSON)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	old := now - msgs.TernTime(60*24*time.Hour)
	if rules[0].MaybeMatchesByCreationTime("/etc/passwd", old, now) {
		t.Fatal("creation-time prefilter should still apply the include regex")
	}
}

func TestAnyMaybeMatches_OneOldEnoughIsEnough(t *testing.T) {
	const j = `[
      {"name":"slow","tag":"NOOP","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":365,"mtime_days":365,"size_bytes":0,"extended_retention_bitfield_match":null},
      {"name":"fast","tag":"NOOP","include_path_match":[".*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":1,"mtime_days":1,"size_bytes":0,"extended_retention_bitfield_match":null}
    ]`
	rules := mustLoad(t, j)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	young := now - msgs.TernTime(7*24*time.Hour)

	if !AnyMaybeMatches(rules, "/x", young, now) {
		t.Fatal("AnyMaybeMatches should be true when at least one rule is satisfiable")
	}

	veryYoung := now - msgs.TernTime(1*time.Hour)
	if AnyMaybeMatches(rules, "/x", veryYoung, now) {
		t.Fatal("AnyMaybeMatches should be false when no rule can possibly fire")
	}
}

func TestFirstMatch_FirstWins(t *testing.T) {
	const j = `[
      {"name":"a","tag":"DELETE","include_path_match":["^/a/.*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":null},
      {"name":"b","tag":"COMPRESS","include_path_match":["^/a/b/.*"],"exclude_path_match":[],"file_suffix_match":[".*"],"atime_days":0,"mtime_days":0,"size_bytes":0,"extended_retention_bitfield_match":null}
    ]`
	rules := mustLoad(t, j)
	now := msgs.MakeTernTime(time.Unix(1_700_000_000, 0))
	got := FirstMatch(rules, "/a/b/x.dat", 0, now, now, now)
	if got == nil || got.Name != "a" {
		t.Fatalf("want rule a, got %+v", got)
	}
}

func TestFileRulesApplyOnlyToRegularFiles(t *testing.T) {
	for _, tc := range []struct {
		inodeType msgs.InodeType
		want      bool
	}{
		{msgs.FILE, true},
		{msgs.DIRECTORY, false},
		{msgs.SYMLINK, false},
	} {
		id := msgs.MakeInodeId(tc.inodeType, 0, 1)
		if got := fileRulesApplyTo(id); got != tc.want {
			t.Errorf("fileRulesApplyTo(%s) = %v, want %v", tc.inodeType, got, tc.want)
		}
	}
}

// listFinal returns sorted dir entries excluding *.tmp.
func listFinal(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func TestBatchWriter_RotatesByRows_LocalFs(t *testing.T) {
	dir := t.TempDir()
	bw, err := newBatchWriters(dir, 3, time.Hour, false, []string{"DELETE"})
	if err != nil {
		t.Fatalf("newBatchWriters: %v", err)
	}
	for i := 0; i < 7; i++ {
		if err := bw.AppendRow("DELETE", 0x2a, "row-x"); err != nil {
			t.Fatalf("AppendRow: %v", err)
		}
	}
	if err := bw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := listFinal(t, dir)
	want := []string{
		"DELETE-2a-000000.tsv",
		"DELETE-2a-000001.tsv",
		"DELETE-2a-000002.tsv",
	}
	if !equalStringSlice(got, want) {
		t.Fatalf("listFinal = %v\nwant            %v", got, want)
	}
	for _, name := range got {
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		if len(body) == 0 || body[len(body)-1] != '\n' {
			t.Fatalf("file %s does not end with newline (body=%q)", name, body)
		}
	}
}

func TestBatchWriter_RotatesByAge_LocalFs(t *testing.T) {
	dir := t.TempDir()
	bw, err := newBatchWriters(dir, 1_000_000, 10*time.Millisecond, false, []string{"DELETE"})
	if err != nil {
		t.Fatalf("newBatchWriters: %v", err)
	}
	if err := bw.AppendRow("DELETE", 1, "row1"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := bw.AppendRow("DELETE", 1, "row2"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := bw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := listFinal(t, dir)
	want := []string{
		"DELETE-01-000000.tsv",
		"DELETE-01-000001.tsv",
	}
	if !equalStringSlice(got, want) {
		t.Fatalf("listFinal = %v\nwant            %v", got, want)
	}
}

func TestBatchWriter_PerTagShardIndependent_LocalFs(t *testing.T) {
	dir := t.TempDir()
	bw, err := newBatchWriters(dir, 1_000_000, time.Hour, false, []string{"DELETE", "COMPRESS"})
	if err != nil {
		t.Fatalf("newBatchWriters: %v", err)
	}
	if err := bw.AppendRow("DELETE", 0, "a"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := bw.AppendRow("COMPRESS", 0, "b"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := bw.AppendRow("DELETE", 1, "c"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := bw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := listFinal(t, dir)
	want := []string{
		"COMPRESS-00-000000.tsv",
		"DELETE-00-000000.tsv",
		"DELETE-01-000000.tsv",
	}
	if !equalStringSlice(got, want) {
		t.Fatalf("listFinal = %v\nwant            %v", got, want)
	}
}

func TestBatchWriter_LocalFsHidesTmp(t *testing.T) {
	dir := t.TempDir()
	bw, err := newBatchWriters(dir, 1_000_000, time.Hour, false, []string{"DELETE"})
	if err != nil {
		t.Fatalf("newBatchWriters: %v", err)
	}
	if err := bw.AppendRow("DELETE", 0, "a"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}

	// AppendRow is async; wait for the .tmp to appear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hasTmpFile(t, dir) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if names := listFinal(t, dir); len(names) != 0 {
		t.Fatalf("expected no final batches while open, got %v", names)
	}
	if !hasTmpFile(t, dir) {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("expected a .tmp file while open, dir=%v", entries)
	}

	if err := bw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got := listFinal(t, dir)
	want := []string{"DELETE-00-000000.tsv"}
	if !equalStringSlice(got, want) {
		t.Fatalf("after close: %v want %v", got, want)
	}
}

func hasTmpFile(t *testing.T, dir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			return true
		}
	}
	return false
}

func TestBatchWriter_TernfsTransient_NoTmpSuffix(t *testing.T) {
	dir := t.TempDir()
	bw, err := newBatchWriters(dir, 1, time.Hour, true, []string{"DELETE"})
	if err != nil {
		t.Fatalf("newBatchWriters: %v", err)
	}
	if err := bw.AppendRow("DELETE", 0, "a"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := bw.AppendRow("DELETE", 0, "b"); err != nil {
		t.Fatalf("AppendRow: %v", err)
	}
	if err := bw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("ternfs-transient mode should not create .tmp files, found %q", e.Name())
		}
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
