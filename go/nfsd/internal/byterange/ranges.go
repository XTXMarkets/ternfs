// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package byterange

import (
	"sort"
)

type Range struct {
	Start uint64
	End   uint64
}

// Set stores sorted, non-overlapping half-open byte ranges.
type Set []Range

func (rs *Set) Add(start, end uint64) {
	if start >= end {
		return
	}

	ranges := *rs
	first := sort.Search(len(ranges), func(i int) bool {
		return ranges[i].End >= start
	})

	last := first
	for last < len(ranges) && ranges[last].Start <= end {
		if ranges[last].Start < start {
			start = ranges[last].Start
		}
		if ranges[last].End > end {
			end = ranges[last].End
		}
		last++
	}

	merged := Range{Start: start, End: end}
	if first == last {
		ranges = append(ranges, Range{})
		copy(ranges[first+1:], ranges[first:])
		ranges[first] = merged
		*rs = ranges
		return
	}

	ranges[first] = merged
	copy(ranges[first+1:], ranges[last:])
	*rs = ranges[:len(ranges)-(last-first)+1]
}

func (rs Set) Union(other Set) Set {
	merged := make(Set, 0, len(rs)+len(other))
	appendRange := func(r Range) {
		if len(merged) != 0 && merged[len(merged)-1].End >= r.Start {
			if r.End > merged[len(merged)-1].End {
				merged[len(merged)-1].End = r.End
			}
			return
		}
		merged = append(merged, r)
	}

	i, j := 0, 0
	for i < len(rs) && j < len(other) {
		if rs[i].Start <= other[j].Start {
			appendRange(rs[i])
			i++
		} else {
			appendRange(other[j])
			j++
		}
	}
	for ; i < len(rs); i++ {
		appendRange(rs[i])
	}
	for ; j < len(other); j++ {
		appendRange(other[j])
	}
	return merged
}

func (rs *Set) Remove(start, end uint64) {
	if start >= end {
		return
	}

	ranges := *rs
	for _, r := range ranges {
		if r.Start < start && r.End > end {
			out := make(Set, 0, len(ranges)+1)
			for _, split := range ranges {
				if split != r {
					out = append(out, split)
					continue
				}
				out = append(out,
					Range{Start: r.Start, End: start},
					Range{Start: end, End: r.End},
				)
			}
			*rs = out
			return
		}
	}

	out := ranges[:0]
	for _, r := range ranges {
		if r.End <= start || r.Start >= end {
			out = append(out, r)
			continue
		}
		if r.Start < start {
			out = append(out, Range{Start: r.Start, End: start})
		}
		if r.End > end {
			out = append(out, Range{Start: end, End: r.End})
		}
	}
	*rs = out
}

func (rs Set) Contains(start, end uint64) bool {
	if start >= end {
		return true
	}
	for _, r := range rs {
		if r.Start > start {
			return false
		}
		if r.End > start {
			return r.End >= end
		}
	}
	return false
}

// Gaps returns portions of [start, end) not covered by the set.
func (rs Set) Gaps(start, end uint64) []Range {
	if start >= end {
		return nil
	}
	var gaps []Range
	pos := start
	for _, r := range rs {
		if r.End <= pos {
			continue
		}
		if r.Start >= end {
			break
		}
		if r.Start > pos {
			gaps = append(gaps, Range{Start: pos, End: min(r.Start, end)})
		}
		if r.End > pos {
			pos = r.End
		}
		if pos >= end {
			return gaps
		}
	}
	if pos < end {
		gaps = append(gaps, Range{Start: pos, End: end})
	}
	return gaps
}
