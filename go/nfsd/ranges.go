// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import "sort"

type byteRange struct {
	start uint64
	end   uint64
}

// byteRangeSet stores sorted, non-overlapping half-open byte ranges.
type byteRangeSet []byteRange

func (rs *byteRangeSet) add(start, end uint64) {
	if start >= end {
		return
	}

	ranges := *rs
	first := sort.Search(len(ranges), func(i int) bool {
		return ranges[i].end >= start
	})

	last := first
	for last < len(ranges) && ranges[last].start <= end {
		if ranges[last].start < start {
			start = ranges[last].start
		}
		if ranges[last].end > end {
			end = ranges[last].end
		}
		last++
	}

	merged := byteRange{start: start, end: end}
	if first == last {
		ranges = append(ranges, byteRange{})
		copy(ranges[first+1:], ranges[first:])
		ranges[first] = merged
		*rs = ranges
		return
	}

	ranges[first] = merged
	copy(ranges[first+1:], ranges[last:])
	*rs = ranges[:len(ranges)-(last-first)+1]
}

func (rs byteRangeSet) union(other byteRangeSet) byteRangeSet {
	merged := make(byteRangeSet, 0, len(rs)+len(other))
	appendRange := func(r byteRange) {
		if len(merged) != 0 && merged[len(merged)-1].end >= r.start {
			if r.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = r.end
			}
			return
		}
		merged = append(merged, r)
	}

	i, j := 0, 0
	for i < len(rs) && j < len(other) {
		if rs[i].start <= other[j].start {
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

func (rs *byteRangeSet) remove(start, end uint64) {
	if start >= end {
		return
	}

	ranges := *rs
	for _, r := range ranges {
		if r.start < start && r.end > end {
			out := make(byteRangeSet, 0, len(ranges)+1)
			for _, split := range ranges {
				if split != r {
					out = append(out, split)
					continue
				}
				out = append(out,
					byteRange{start: r.start, end: start},
					byteRange{start: end, end: r.end},
				)
			}
			*rs = out
			return
		}
	}

	out := ranges[:0]
	for _, r := range ranges {
		if r.end <= start || r.start >= end {
			out = append(out, r)
			continue
		}
		if r.start < start {
			out = append(out, byteRange{start: r.start, end: start})
		}
		if r.end > end {
			out = append(out, byteRange{start: end, end: r.end})
		}
	}
	*rs = out
}

func (rs byteRangeSet) contains(start, end uint64) bool {
	if start >= end {
		return true
	}
	for _, r := range rs {
		if r.start > start {
			return false
		}
		if r.end > start {
			return r.end >= end
		}
	}
	return false
}

// gaps returns portions of [start, end) not covered by the set.
func (rs byteRangeSet) gaps(start, end uint64) []byteRange {
	if start >= end {
		return nil
	}
	var gaps []byteRange
	pos := start
	for _, r := range rs {
		if r.end <= pos {
			continue
		}
		if r.start >= end {
			break
		}
		if r.start > pos {
			gaps = append(gaps, byteRange{start: pos, end: min(r.start, end)})
		}
		if r.end > pos {
			pos = r.end
		}
		if pos >= end {
			return gaps
		}
	}
	if pos < end {
		gaps = append(gaps, byteRange{start: pos, end: end})
	}
	return gaps
}
