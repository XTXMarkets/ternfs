// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package byterange

import (
	"reflect"
	"testing"
)

func TestByteRangeSetAdd(t *testing.T) {
	var ranges Set
	ranges.Add(10, 20)
	ranges.Add(30, 40)
	ranges.Add(20, 30)
	ranges.Add(5, 8)
	ranges.Add(6, 12)
	ranges.Add(50, 50)

	want := Set{
		{Start: 5, End: 40},
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
}

func TestByteRangeSetUnion(t *testing.T) {
	left := Set{
		{Start: 0, End: 10},
		{Start: 30, End: 40},
		{Start: 60, End: 70},
	}
	right := Set{
		{Start: 10, End: 20},
		{Start: 25, End: 35},
		{Start: 80, End: 90},
	}
	got := left.Union(right)
	want := Set{
		{Start: 0, End: 20},
		{Start: 25, End: 40},
		{Start: 60, End: 70},
		{Start: 80, End: 90},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("union = %#v, want %#v", got, want)
	}
}

func TestByteRangeSetRemove(t *testing.T) {
	ranges := Set{
		{Start: 0, End: 10},
		{Start: 20, End: 30},
	}
	ranges.Remove(5, 25)

	want := Set{
		{Start: 0, End: 5},
		{Start: 25, End: 30},
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
}

func TestByteRangeSetRemoveSplitsWithoutCorruptingFollowingRanges(t *testing.T) {
	ranges := Set{
		{Start: 0, End: 100},
		{Start: 200, End: 300},
	}
	ranges.Remove(10, 20)

	want := Set{
		{Start: 0, End: 10},
		{Start: 20, End: 100},
		{Start: 200, End: 300},
	}
	if !reflect.DeepEqual(ranges, want) {
		t.Fatalf("ranges = %#v, want %#v", ranges, want)
	}
}

func TestByteRangeSetContains(t *testing.T) {
	ranges := Set{
		{Start: 5, End: 15},
		{Start: 20, End: 30},
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
		if got := ranges.Contains(tt.start, tt.end); got != tt.want {
			t.Errorf("contains(%d, %d) = %t, want %t",
				tt.start, tt.end, got, tt.want)
		}
	}
}

func TestByteRangeSetGaps(t *testing.T) {
	ranges := Set{
		{Start: 5, End: 10},
		{Start: 15, End: 20},
		{Start: 25, End: 30},
	}
	got := ranges.Gaps(7, 28)
	want := []Range{
		{Start: 10, End: 15},
		{Start: 20, End: 25},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gaps = %#v, want %#v", got, want)
	}
}
