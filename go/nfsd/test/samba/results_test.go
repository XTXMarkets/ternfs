// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package samba

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseOutput(t *testing.T) {
	out := `smbtorture 4.17.12-Debian
Using seed 1793810000
time: 2026-11-04 16:02:32.336423
test: OVERWRITE_IF
WARNING!: (../../source4/torture/smb2/delete-on-close.c:192) Incorrect status
failure: OVERWRITE_IF [
(../../source4/torture/smb2/delete-on-close.c:73) Incorrect status
]
test: CREATE Existing
success: CREATE Existing
skip: READONLY
error: FIND_and_set_DOC [
`
	got, err := ParseOutput("smb2.delete-on-close-perms", strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	want := Results{
		"smb2.delete-on-close-perms/OVERWRITE_IF":     "failure",
		"smb2.delete-on-close-perms/CREATE Existing":  "success",
		"smb2.delete-on-close-perms/READONLY":         "skip",
		"smb2.delete-on-close-perms/FIND_and_set_DOC": "error",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestCompare(t *testing.T) {
	nfs := Results{
		"s/pass":          "success",
		"s/both":          "failure",
		"s/nfsd-only":     "failure",
		"s/known":         "error",
		"s/control-only":  "success",
		"s/fixed":         "success",
		"other/skip-both": "skip",
	}
	control := Results{
		"s/pass":          "success",
		"s/both":          "failure",
		"s/nfsd-only":     "success",
		"s/known":         "success",
		"s/control-only":  "failure",
		"s/fixed":         "success",
		"s/not-run":       "success",
		"other/skip-both": "skip",
	}
	known := map[string]bool{"s/known": true, "s/fixed": true, "unrun/case": true}
	got := Compare(nfs, control, known, []string{"s", "other"})
	want := Report{
		Passed:      3,
		BothFailed:  []string{"s/both"},
		NfsdOnly:    []string{"s/nfsd-only", "s/not-run"},
		Known:       []string{"s/known"},
		Stale:       []string{"s/fixed"},
		ControlOnly: []string{"s/control-only"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

// writeRun lays out one share's output and exit status for each suite, as
// guest.sh does.
func writeRun(t *testing.T, dir, share string, suites map[string][2]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, share), 0755); err != nil {
		t.Fatal(err)
	}
	for suite, run := range suites {
		for name, data := range map[string]string{suite + ".txt": run[0], suite + ".rc": run[1]} {
			if err := os.WriteFile(filepath.Join(dir, share, name), []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// A suite that hangs or crashes on both shares compares equal, so it must
// fail the run on its own exit status, and so must one that ran nothing.
func TestEvaluateRejectsIncompleteSuites(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "known.txt")
	if err := os.WriteFile(manifest, nil, 0644); err != nil {
		t.Fatal(err)
	}
	passed := [2]string{"success: one\n", "0\n"}
	for _, test := range []struct {
		name  string
		other [2]string
		want  string
	}{
		{"complete", [2]string{"success: two\nfailure: three [\n", "1\n"}, ""},
		{"timed out", [2]string{"success: two\n", "137\n"}, "exited 137: timed out or crashed"},
		{"no cases", [2]string{"smbtorture 4.22.6\n", "1\n"}, "ran no cases"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, share := range []string{"nfs", "control"} {
				writeRun(t, dir, share, map[string][2]string{"a": passed, "b": test.other})
			}
			var out strings.Builder
			err := Evaluate(dir, manifest, []string{"a", "b"}, &out)
			switch {
			case test.want == "" && err != nil:
				t.Fatalf("complete run failed: %v\n%s", err, out.String())
			case test.want != "" && err == nil:
				t.Fatalf("run passed, want it to fail with %q\n%s", test.want, out.String())
			case test.want != "" && !strings.Contains(out.String(), test.want):
				t.Fatalf("report lacks %q:\n%s", test.want, out.String())
			}
		})
	}
}
