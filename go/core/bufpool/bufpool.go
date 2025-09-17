// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package bufpool

import (
	"sync"
	"sync/atomic"
)

// An extremely dumb buffer pool. The assumption is that this is to be used in
// cases where there is a fixed upper bound for the buffers, and most requests
// are going to be for that upper bound.
type BufPool sync.Pool

func NewBufPool() *BufPool {
	pool := BufPool(sync.Pool{
		New: func() any {
			buf := []byte{}
			return &buf
		},
	})
	return &pool
}

func NewBuf(b *[]byte) *Buf {
	res := &Buf{}
	res.b.Store(b)
	return res
}

type Buf struct {
	b atomic.Pointer[[]byte]
}

func (b *Buf) Bytes() []byte {
	return *b.BytesPtr()
}

func (b *Buf) BytesPtr() *[]byte {
	res := b.b.Load()
	if res == nil {
		panic("accessing freed buffer")
	}
	return res
}

// This does _not_ zero the memory in the bufs -- i.e. there might
// be garbage in it.
func (pool *BufPool) Get(l int) *Buf {
	res := &Buf{}
	if l == 0 {
		b := []byte{}
		res.b.Store(&b)
		return res
	}
	buf := (*sync.Pool)(pool).Get().(*[]byte)
	if cap(*buf) >= l {
		*buf = (*buf)[:l]
	} else {
		*buf = (*buf)[:cap(*buf)]
		*buf = append(*buf, make([]byte, l-len(*buf))...)
	}
	res.b.Store(buf)
	return res
}

func (pool *BufPool) Put(buf *Buf) {
	if buf == nil {
		return
	}
	ptr := buf.b.Swap(nil)
	if ptr == nil {
		panic("double BufPool put")
	}
	if cap(*ptr) == 0 {
		return
	}
	(*sync.Pool)(pool).Put(ptr)
}
