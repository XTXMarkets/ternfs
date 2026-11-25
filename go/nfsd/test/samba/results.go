// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Package samba judges an smbtorture run through Samba over nfsd. Each suite
// runs against the share on the nfsd mount and against a tmpfs control share
// with the same options. A case that fails on both is Samba or configuration;
// a case that fails only on nfsd fails the run unless the manifest of known
// differences lists it.
package samba

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultSuites are the SMB2 suites run by default. With oplocks = no,
// smb2.lease, smb2.oplock and smb2.durable-open fail by configuration on both
// shares.
const DefaultSuites = "smb2.create,smb2.read,smb2.rename,smb2.dir,smb2.mkdir,smb2.setinfo," +
	"smb2.getinfo,smb2.lock,smb2.timestamps,smb2.delete-on-close-perms,smb2.sharemode,smb2.notify"

// guestPath is the guest's PATH. The guest runs the host's binaries over its
// 9p root, so these host directories are where it finds Samba.
var guestPath = []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin"}

// CheckTools reports the Samba programs guest.sh needs that the guest will
// not find.
func CheckTools() error {
	var missing []string
	for _, tool := range []string{"smbd", "smbtorture", "smbpasswd", "smbcontrol"} {
		found := false
		for _, dir := range guestPath {
			if info, err := os.Stat(filepath.Join(dir, tool)); err == nil && info.Mode()&0111 != 0 {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, tool)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s not installed in %s; on Debian and Ubuntu install samba and samba-testsuite",
			strings.Join(missing, ", "), strings.Join(guestPath, ":"))
	}
	return nil
}

// Results maps "suite/case" to smbtorture's status for it: success, failure,
// error or skip.
type Results map[string]string

// ParseOutput reads the status lines smbtorture prints for each case of a
// suite, such as "success: find" and "failure: CREATE Existing [".
func ParseOutput(suite string, r io.Reader) (Results, error) {
	results := Results{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		status, name, ok := strings.Cut(scanner.Text(), ": ")
		if !ok {
			continue
		}
		switch status {
		case "success", "failure", "error", "skip":
		default:
			continue
		}
		name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), "["))
		if name != "" {
			results[suite+"/"+name] = status
		}
	}
	return results, scanner.Err()
}

func failed(status string) bool { return status == "failure" || status == "error" }

// Report sorts the cases of one run.
type Report struct {
	Passed int
	// BothFailed fail on the control too.
	BothFailed []string
	// NfsdOnly fail, or did not run, only on nfsd and are not known.
	NfsdOnly []string
	// Known fail only on nfsd and are listed in the manifest.
	Known []string
	// Stale are manifest entries for suites that ran which no longer differ.
	Stale []string
	// ControlOnly fail only on the control.
	ControlOnly []string
}

// Compare sorts every case either share ran. known holds manifest entries.
func Compare(nfs, control Results, known map[string]bool, suites []string) Report {
	var r Report
	names := map[string]bool{}
	for name := range nfs {
		names[name] = true
	}
	for name := range control {
		names[name] = true
	}
	differing := map[string]bool{}
	for name := range names {
		n, ranOnNfs := nfs[name]
		c := control[name]
		switch {
		case (failed(n) || !ranOnNfs) && !failed(c):
			differing[name] = true
			if known[name] {
				r.Known = append(r.Known, name)
			} else {
				r.NfsdOnly = append(r.NfsdOnly, name)
			}
		case failed(n) && failed(c):
			r.BothFailed = append(r.BothFailed, name)
		case failed(c):
			r.ControlOnly = append(r.ControlOnly, name)
		default:
			r.Passed++
		}
	}
	ran := map[string]bool{}
	for _, suite := range suites {
		ran[suite] = true
	}
	for name := range known {
		suite, _, _ := strings.Cut(name, "/")
		if ran[suite] && !differing[name] {
			r.Stale = append(r.Stale, name)
		}
	}
	for _, list := range [][]string{r.BothFailed, r.NfsdOnly, r.Known, r.Stale, r.ControlOnly} {
		sort.Strings(list)
	}
	return r
}

// LoadManifest reads known differences: one "suite/case" per line, with the
// reason in comments above it.
func LoadManifest(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "/") {
			return nil, fmt.Errorf("%s: %q is not suite/case", path, line)
		}
		known[line] = true
	}
	return known, nil
}

// loadShare reads each suite's output on one share. incomplete lists the
// suites that did not finish normally: smbtorture exits 0 when every case
// passes and 1 when one fails, so any other exit status, such as 137 from
// the suite timeout's kill, or no cases at all, means the suite did not run
// to its end.
func loadShare(dir, share string, suites []string) (all Results, incomplete []string, err error) {
	all = Results{}
	for _, suite := range suites {
		rc, err := os.ReadFile(filepath.Join(dir, share, suite+".rc"))
		if err != nil {
			incomplete = append(incomplete, fmt.Sprintf("%s on the %s share has no exit status", suite, share))
			continue
		}
		if status := strings.TrimSpace(string(rc)); status != "0" && status != "1" {
			incomplete = append(incomplete, fmt.Sprintf("%s on the %s share exited %s: timed out or crashed", suite, share, status))
		}
		f, err := os.Open(filepath.Join(dir, share, suite+".txt"))
		if err != nil {
			return nil, nil, err
		}
		results, err := ParseOutput(suite, f)
		f.Close()
		if err != nil {
			return nil, nil, err
		}
		if len(results) == 0 {
			incomplete = append(incomplete, fmt.Sprintf("%s on the %s share ran no cases", suite, share))
		}
		for name, status := range results {
			all[name] = status
		}
	}
	return all, incomplete, nil
}

// Evaluate reads the raw output guest.sh wrote under dir, prints the
// comparison and fails on any unknown difference.
func Evaluate(dir, manifest string, suites []string, w io.Writer) error {
	known, err := LoadManifest(manifest)
	if err != nil {
		return err
	}
	nfs, nfsIncomplete, err := loadShare(dir, "nfs", suites)
	if err != nil {
		return err
	}
	control, controlIncomplete, err := loadShare(dir, "control", suites)
	if err != nil {
		return err
	}
	incomplete := append(nfsIncomplete, controlIncomplete...)
	r := Compare(nfs, control, known, suites)
	print := func(label string, names []string) {
		for _, name := range names {
			fmt.Fprintf(w, "SAMBA %s %s\n", label, name)
		}
	}
	print("both-failed", r.BothFailed)
	print("control-only-failure", r.ControlOnly)
	print("known-difference", r.Known)
	print("stale-manifest-entry", r.Stale)
	print("NFSD-ONLY-FAILURE", r.NfsdOnly)
	print("INCOMPLETE-SUITE", incomplete)
	fmt.Fprintf(w, "SAMBA passed=%d both-failed=%d known=%d nfsd-only=%d control-only=%d stale=%d incomplete=%d\n",
		r.Passed, len(r.BothFailed), len(r.Known), len(r.NfsdOnly), len(r.ControlOnly), len(r.Stale), len(incomplete))
	var errs []error
	if len(incomplete) > 0 {
		errs = append(errs, errors.New("smbtorture suites did not run to their end"))
	}
	if len(r.NfsdOnly) > 0 {
		errs = append(errs, errors.New("smbtorture cases fail on nfsd but not on the control share"))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%w; raw output in %s", errors.Join(errs...), dir)
	}
	return nil
}
