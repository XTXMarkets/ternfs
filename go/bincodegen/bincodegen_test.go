// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/XTXMarkets/ternfs/go/core/bincode"
)

type len32TestStruct struct {
	Narrow []uint64
	Wide   []uint64     `bincode:"len32"`
	Data   bincode.Blob `bincode:"len32"`
}

func expectContains(t *testing.T, what string, out string, subs ...string) {
	for _, s := range subs {
		if !strings.Contains(out, s) {
			t.Errorf("%s output missing %q:\n%s", what, s, out)
		}
	}
}

func TestLen32Codegen(t *testing.T) {
	typ := reflect.TypeOf(len32TestStruct{})

	goOut := new(bytes.Buffer)
	generateGoSingle(goOut, typ)
	expectContains(t, "go", goOut.String(),
		"bincode.PackLength(w, len1)", "bincode.UnpackLength(r, &len1)",
		"bincode.PackLength32(w, len2)", "bincode.UnpackLength32(r, &len2)",
		"bincode.PackBlob32(w, v.Data)", "bincode.UnpackBlob32(r, &v.Data)")

	hpp, cpp := new(bytes.Buffer), new(bytes.Buffer)
	generateCppSingle(hpp, cpp, typ)
	expectContains(t, "c++", hpp.String(),
		"BincodeList<uint64_t> narrow;", "BincodeList<uint64_t, uint32_t> wide;", "BincodeList<uint8_t, uint32_t> data;",
		"BincodeList<uint64_t, uint32_t>::STATIC_SIZE")

	// kmod doesn't support tags
	kmod := new(bytes.Buffer)
	generateKmodGet(map[string]int{}, kmod, reflect.TypeOf(struct{ Narrow []uint64 }{}))
	generateKmodPut(map[string]int{}, kmod, reflect.TypeOf(struct{ Narrow []uint64 }{}))
	expectContains(t, "kmod", kmod.String(), "u16 len", "get_unaligned_le16", "put_unaligned_le16")
}

func expectPanic(t *testing.T, what string, typ reflect.Type, f func()) {
	defer func() {
		if recover() == nil {
			t.Errorf("%s %v: expected panic", what, typ)
		}
	}()
	f()
}

func TestLen32RejectsInvalidFields(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeOf(struct {
			X []byte `bincode:"len32"`
		}{}),
		reflect.TypeOf(struct {
			X uint32 `bincode:"len32"`
		}{}),
		reflect.TypeOf(struct {
			X []struct{} `bincode:"len32"`
		}{}),
		reflect.TypeOf(struct {
			X []struct{}
		}{}),
		reflect.TypeOf(struct {
			X []uint64 `bincode:"len64"`
		}{}),
	} {
		expectPanic(t, "go", typ, func() { generateGoSingle(new(bytes.Buffer), typ) })
		expectPanic(t, "c++", typ, func() { generateCppSingle(new(bytes.Buffer), new(bytes.Buffer), typ) })
		expectPanic(t, "kmod get", typ, func() { generateKmodGet(map[string]int{}, new(bytes.Buffer), typ) })
		expectPanic(t, "kmod put", typ, func() { generateKmodPut(map[string]int{}, new(bytes.Buffer), typ) })
	}
}

func TestKmodRejectsLen32(t *testing.T) {
	typ := reflect.TypeOf(struct {
		X []uint64 `bincode:"len32"`
	}{})
	expectPanic(t, "kmod get", typ, func() { generateKmodGet(map[string]int{}, new(bytes.Buffer), typ) })
	expectPanic(t, "kmod put", typ, func() { generateKmodPut(map[string]int{}, new(bytes.Buffer), typ) })
}
