// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"time"
	"xtx/ternfs/msgs"
)

// Rule is one entry in a tag-rules JSON file.
//
// Tag is an arbitrary string. When a rule fires, its tag selects the
// output bucket; tag values have no built-in meaning here.
//
// AppliesTo is "file" (default), "directory", or "symlink".
type Rule struct {
	Name                           string
	Tag                            string
	AppliesTo                      string
	SymlinkTarget                  string
	IncludePatterns                []*regexp.Regexp
	ExcludePatterns                []*regexp.Regexp
	SuffixPatterns                 []*regexp.Regexp
	AtimeDays                      float64
	MtimeDays                      float64
	SizeBytes                      uint64
	ExtendedRetentionBitfieldMatch *bool
}

// Sentinel checked against the seconds portion of mtime. When a rule sets
// ExtendedRetentionBitfieldMatch, the low 10 bits of (mtime / 1s) must
// match this pattern (true) or not (false).
const (
	extendedRetentionBitfield uint64 = 0b1010101010
	extendedRetentionBitmask  uint64 = 0b1111111111
)

// threshold accepts either a JSON number or the string "inf".
type threshold float64

func (t *threshold) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s == "inf" || s == "Infinity" {
			*t = threshold(math.Inf(+1))
			return nil
		}
		return fmt.Errorf("unexpected threshold string %q", s)
	}
	var f float64
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	*t = threshold(f)
	return nil
}

type ruleJSON struct {
	Name                           string    `json:"name"`
	Tag                            string    `json:"tag"`
	AppliesTo                      string    `json:"applies_to"`
	SymlinkTarget                  string    `json:"symlink_target"`
	IncludePathMatch               []string  `json:"include_path_match"`
	ExcludePathMatch               []string  `json:"exclude_path_match"`
	FileSuffixMatch                []string  `json:"file_suffix_match"`
	AtimeDays                      threshold `json:"atime_days"`
	MtimeDays                      threshold `json:"mtime_days"`
	SizeBytes                      uint64    `json:"size_bytes"`
	ExtendedRetentionBitfieldMatch *bool     `json:"extended_retention_bitfield_match"`
}

func LoadRules(data []byte) ([]*Rule, error) {
	var raw []ruleJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make([]*Rule, len(raw))
	for i, r := range raw {
		if r.Tag == "" {
			return nil, fmt.Errorf("rule %q has empty tag", r.Name)
		}
		appliesTo := r.AppliesTo
		if appliesTo == "" {
			appliesTo = "file"
		}
		if appliesTo != "file" && appliesTo != "directory" && appliesTo != "symlink" {
			return nil, fmt.Errorf("rule %q has invalid applies_to %q", r.Name, appliesTo)
		}
		if r.SymlinkTarget != "" && r.SymlinkTarget != "dangling" && r.SymlinkTarget != "not-dangling" {
			return nil, fmt.Errorf("rule %q has invalid symlink_target %q", r.Name, r.SymlinkTarget)
		}
		if r.SymlinkTarget != "" && appliesTo != "symlink" {
			return nil, fmt.Errorf("rule %q has symlink_target but applies_to is %q", r.Name, appliesTo)
		}
		inc, err := compileAll(r.IncludePathMatch)
		if err != nil {
			return nil, fmt.Errorf("rule %q include: %w", r.Name, err)
		}
		exc, err := compileAll(r.ExcludePathMatch)
		if err != nil {
			return nil, fmt.Errorf("rule %q exclude: %w", r.Name, err)
		}
		suf, err := compileAll(r.FileSuffixMatch)
		if err != nil {
			return nil, fmt.Errorf("rule %q suffix: %w", r.Name, err)
		}
		out[i] = &Rule{
			Name:                           r.Name,
			Tag:                            r.Tag,
			AppliesTo:                      appliesTo,
			SymlinkTarget:                  r.SymlinkTarget,
			IncludePatterns:                inc,
			ExcludePatterns:                exc,
			SuffixPatterns:                 suf,
			AtimeDays:                      float64(r.AtimeDays),
			MtimeDays:                      float64(r.MtimeDays),
			SizeBytes:                      r.SizeBytes,
			ExtendedRetentionBitfieldMatch: r.ExtendedRetentionBitfieldMatch,
		}
	}
	return out, nil
}

func compileAll(patterns []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		// \A anchors at start-of-string; the rest of the pattern is left
		// unanchored at end.
		re, err := regexp.Compile(`\A(?:` + p + `)`)
		if err != nil {
			return nil, fmt.Errorf("compile %q: %w", p, err)
		}
		out[i] = re
	}
	return out, nil
}

func matchAny(patterns []*regexp.Regexp, s string) bool {
	for _, re := range patterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// Matches reports whether the rule fires.
func (r *Rule) Matches(path string, size uint64, atime, mtime, now msgs.TernTime) bool {
	if time.Duration(now-atime) < daysToDuration(r.AtimeDays) {
		return false
	}
	if time.Duration(now-mtime) < daysToDuration(r.MtimeDays) {
		return false
	}
	if size < r.SizeBytes {
		return false
	}
	if r.ExtendedRetentionBitfieldMatch != nil {
		mtimeSec := uint64(mtime) / uint64(time.Second)
		hasBitfield := (mtimeSec & extendedRetentionBitmask) == extendedRetentionBitfield
		if hasBitfield != *r.ExtendedRetentionBitfieldMatch {
			return false
		}
	}
	return r.MatchesPath(path)
}

// MatchesPath reports whether the rule's path predicates hold.
func (r *Rule) MatchesPath(path string) bool {
	if !matchAny(r.IncludePatterns, path) {
		return false
	}
	if matchAny(r.ExcludePatterns, path) {
		return false
	}
	suffix := filepath.Ext(path)
	if !matchAny(r.SuffixPatterns, suffix) {
		return false
	}
	return true
}

// FirstMatch returns the first rule whose predicate holds, or nil.
func FirstMatch(rules []*Rule, path string, size uint64, atime, mtime, now msgs.TernTime) *Rule {
	for _, r := range rules {
		if r.Matches(path, size, atime, mtime, now) {
			return r
		}
	}
	return nil
}

func anyPathMatches(rules []*Rule, path string) bool {
	for _, r := range rules {
		if r.MatchesPath(path) {
			return true
		}
	}
	return false
}

// MaybeMatchesByCreationTime is a conservative prefilter: returns false
// only when atime/mtime are bounded below by creationTime tightly enough
// to rule out a match. Size and the bitfield are not checked.
func (r *Rule) MaybeMatchesByCreationTime(path string, creationTime, now msgs.TernTime) bool {
	creationAge := time.Duration(now - creationTime)
	if creationAge < daysToDuration(r.AtimeDays) {
		return false
	}
	if creationAge < daysToDuration(r.MtimeDays) {
		return false
	}
	if !matchAny(r.IncludePatterns, path) {
		return false
	}
	if matchAny(r.ExcludePatterns, path) {
		return false
	}
	suffix := filepath.Ext(path)
	if !matchAny(r.SuffixPatterns, suffix) {
		return false
	}
	return true
}

// AnyMaybeMatches returns true if any rule passes the prefilter.
func AnyMaybeMatches(rules []*Rule, path string, creationTime, now msgs.TernTime) bool {
	for _, r := range rules {
		if r.MaybeMatchesByCreationTime(path, creationTime, now) {
			return true
		}
	}
	return false
}

// daysToDuration treats +Inf as the largest representable duration.
func daysToDuration(days float64) time.Duration {
	if math.IsInf(days, +1) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(days * 24 * float64(time.Hour))
}
