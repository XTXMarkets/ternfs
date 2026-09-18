// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"reflect"
	"testing"
)

func TestByteRangeSetAdd(t *testing.T) {
	var ranges byteRangeSet
	ranges.add(10, 20)
	ranges.add(30, 40)
	ranges.add(20, 30)
	ranges.add(5, 8)
	ranges.add(6, 12)
	ranges.add(50, 50)

	want := byteRangeSet{
		{start: 5, end: 40},
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
}

func TestByteRangeSetUnion(t *testing.T) {
	left := byteRangeSet{
		{start: 0, end: 10},
		{start: 30, end: 40},
		{start: 60, end: 70},
	}
	right := byteRangeSet{
		{start: 10, end: 20},
		{start: 25, end: 35},
		{start: 80, end: 90},
	}
	got := left.union(right)
	want := byteRangeSet{
		{start: 0, end: 20},
		{start: 25, end: 40},
		{start: 60, end: 70},
		{start: 80, end: 90},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("union = %#v, want %#v", got, want)
	}
}

func TestByteRangeSetRemove(t *testing.T) {
	ranges := byteRangeSet{
		{start: 0, end: 10},
		{start: 20, end: 30},
	}
	ranges.remove(5, 25)

	want := byteRangeSet{
		{start: 0, end: 5},
		{start: 25, end: 30},
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
}

func TestByteRangeSetRemoveSplitsWithoutCorruptingFollowingRanges(t *testing.T) {
	ranges := byteRangeSet{
		{start: 0, end: 100},
		{start: 200, end: 300},
	}
	ranges.remove(10, 20)

	want := byteRangeSet{
		{start: 0, end: 10},
		{start: 20, end: 100},
		{start: 200, end: 300},
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
}

func TestByteRangeSetContains(t *testing.T) {
	ranges := byteRangeSet{
		{start: 5, end: 15},
		{start: 20, end: 30},
	}
	for _, tt := range []struct {
		start uint64
		end   uint64
		want  bool
	}{
		{start: 5, end: 15, want: true},
		{start: 7, end: 10, want: true},
		{start: 5, end: 16, want: false},
		{start: 15, end: 20, want: false},
		{start: 25, end: 25, want: true},
	} {
		if got := ranges.contains(tt.start, tt.end); got != tt.want {
			t.Errorf("contains(%d, %d) = %t, want %t",
				tt.start, tt.end, got, tt.want)
		}
	}
}

func TestByteRangeSetGaps(t *testing.T) {
	ranges := byteRangeSet{
		{start: 5, end: 10},
		{start: 15, end: 20},
		{start: 25, end: 30},
	}
	got := ranges.gaps(7, 28)
	want := []byteRange{
		{start: 10, end: 15},
		{start: 20, end: 25},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gaps = %#v, want %#v", got, want)
	}
}
