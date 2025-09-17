// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package bincode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Packable interface {
	Pack(w io.Writer) error
}

func PackScalar[V bool | uint8 | uint16 | uint32 | uint64](w io.Writer, x V) error {
	return binary.Write(w, binary.LittleEndian, x)
}

func PackBytes(w io.Writer, bs []byte) error {
	if len(bs) > 255 {
		panic(fmt.Sprintf("bytes length exceed 255: %v", len(bs)))
	}
	if err := PackScalar(w, uint8(len(bs))); err != nil {
		return err
	}
	if _, err := w.Write(bs); err != nil {
		return err
	}
	return nil
}

func PackFixedBytes(w io.Writer, l int, bs []byte) error {
	if len(bs) != l {
		panic(fmt.Sprintf("expecting fixed bytes of len %v, got %v instead", l, len(bs)))
	}
	_, err := w.Write(bs)
	return err
}

func PackLength(w io.Writer, l int) error {
	if l > math.MaxUint16 {
		panic(fmt.Sprintf("len %d exceeds max length %d", l, math.MaxUint16))
	}
	return PackScalar(w, uint16(l))
}

type Blob []byte

func PackBlob(w io.Writer, bs Blob) error {
	if len(bs) > int(^uint16(0)) {
		panic(fmt.Sprintf("bytes length exceed %v: %v", ^uint16(0), len(bs)))
	}
	if err := PackScalar(w, uint16(len(bs))); err != nil {
		return err
	}
	if _, err := w.Write(bs); err != nil {
		return err
	}
	return nil
}

func UnpackScalar[V bool | uint8 | uint16 | uint32 | uint64](r io.Reader, x *V) error {
	return binary.Read(r, binary.LittleEndian, x)
}

// This function will discard what's in `data`, and just
// set the pointer to a slice of the backing `buf`.
func UnpackBytes(r io.Reader, data *[]byte) error {
	var l uint8
	if err := UnpackScalar(r, &l); err != nil {
		return err
	}
	*data = make([]byte, l)
	if _, err := io.ReadFull(r, *data); err != nil {
		return err
	}
	return nil
}

// The intent with this one (as opposed with `UnpackBytes`) is
// to use it with a fixedsized array, e.g.
//
//	var x [4]byte
//	buf.UnpackFixedBytes(4, x[:])
//
// Which is why it does not take a pointer like `UnpackBytes`,
// and copies the data.
func UnpackFixedBytes(r io.Reader, l int, data []byte) error {
	if len(data) != l {
		panic(fmt.Sprintf("expecting fixed bytes of len %v, got %v instead", l, len(data)))
	}
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}
	return nil
}

func UnpackString(r io.Reader, data *string) error {
	var bs []byte
	if err := UnpackBytes(r, &bs); err != nil {
		return err
	}
	*data = string(bs)
	return nil
}

// This function will discard what's in `data`, and just
// set the pointer to a slice of the backing `buf`.
func UnpackBlob(r io.Reader, data *Blob) error {
	var l uint16
	if err := UnpackScalar(r, &l); err != nil {
		return err
	}
	*data = make([]byte, l)
	if _, err := io.ReadFull(r, *data); err != nil {
		return err
	}
	return nil
}

func UnpackLength(r io.Reader, l *int) error {
	var l16 uint16
	if err := UnpackScalar(r, &l16); err != nil {
		return err
	}
	*l = int(l16)
	return nil
}

type Unpackable interface {
	Unpack(r io.Reader) error
}

// Useful in codegen, weirdly go does not include something like vector::resize.
//
// This does not resize exponentially, which is annoying, but better than allocating
// every time.
func EnsureLength[T any](xs *[]T, l int) {
	if *xs == nil || cap(*xs) < l {
		*xs = make([]T, l)
	} else {
		*xs = (*xs)[:l]
	}
}

func Pack(v Packable) []byte {
	buf := bytes.NewBuffer([]byte{})
	if err := v.Pack(buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func Unpack(data []byte, v Unpackable) error {
	buf := bytes.NewReader(data)
	if err := v.Unpack(buf); err != nil {
		return err
	}
	if buf.Len() != 0 {
		return fmt.Errorf("%v leftover bytes remaining after unpacking", buf.Len())
	}
	return nil
}
