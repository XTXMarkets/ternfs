// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package assert

import (
	"bytes"
	"testing"
)

func Equal[A comparable](t *testing.T, x A, y A) bool {
	if x != y {
		t.Errorf("%v != %v", x, y)
		return false
	}
	return true
}

func EqualBytes(t *testing.T, x []byte, y []byte) bool {
	if !bytes.Equal(x, y) {
		t.Errorf("%v != %v", x, y)
		return false
	}
	return true
}

func EqualSlice[A comparable](t *testing.T, x []A, y []A) bool {
	if len(x) != len(y) {
		t.Errorf("%v != %v", x, y)
		return false
	}
	for i := 0; i < len(x); i++ {
		if x[i] != y[i] {
			t.Errorf("%v != %v", x, y)
			return false
		}
	}
	return true
}

func True(t *testing.T, x bool, format string, args ...interface{}) bool {
	if !x {
		t.Errorf(format, args...)
		return false
	}
	return true
}

func NoError(t *testing.T, err error) bool {
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return false
	}
	return true
}
