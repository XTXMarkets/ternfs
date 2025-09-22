// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package bufpool

import (
	"sync"
	"sync/atomic"
)

const (
	minSize    = 4096
	numBuckets = 20
)

type BufPool struct {
	buckets        [numBuckets]sync.Pool
	bucketsMaxSize [numBuckets-1]int
}

func NewBufPool() *BufPool {
	bp := &BufPool{}
	sz := minSize
	for i := range numBuckets-1 {
		sz = (sz * 3) / 2
		sz = ((sz + 4096 - 1) / 4096) * 4096 // page-align all sizes
		bp.bucketsMaxSize[i] = sz
		bucketIx := i
		bp.buckets[i] = sync.Pool{
			New: func() any {
				buf := make([]byte, bp.bucketsMaxSize[bucketIx])
				return &buf
			},
		}
	}
	bp.buckets[numBuckets-1] = sync.Pool{
		New: func() any {
			buf := []byte{}
			return &buf
		},
	}
	return bp
}

// Use with caution, if you then put the resulting buf in a bufpool: the buffer
// will be reused and therefore overwritten with stuff.
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

func (b *Buf) Acquire() *Buf {
	res := b.b.Swap(nil)
	if res == nil {
		panic("accessing freed buffer")
	}
	return NewBuf(res)
}

func (bpool *BufPool) bucket(l int) int {
	var i int
	for i = 0; i < numBuckets-1 && l > bpool.bucketsMaxSize[i]; i++ {
	}
	return i
}

// This does _not_ zero the memory in the bufs -- i.e. there might
// be garbage in it. The user shouldn't extend the backing buffer,
// although things will keep working if he does. TODO possibly
// enforce this?
func (bpool *BufPool) Get(l int) *Buf {
	res := &Buf{}
	if l == 0 {
		b := []byte{}
		res.b.Store(&b)
		return res
	}
	poolIx := bpool.bucket(l)
	pool := &bpool.buckets[poolIx]
	buf := pool.Get().(*[]byte)
	if cap(*buf) >= l {
		*buf = (*buf)[:l]
	} else { // happens for last bucket, and if users change the buffer
		*buf = (*buf)[:cap(*buf)]
		if poolIx == numBuckets-1 { // just make it exactly l
			*buf = append(*buf, make([]byte, l-len(*buf))...)
		} else {
			if l > bpool.bucketsMaxSize[poolIx] {
				panic("impossible")
			}
			*buf = append(*buf, make([]byte, bpool.bucketsMaxSize[poolIx]-len(*buf))...)
		}
		*buf = (*buf)[:l]
	}
	res.b.Store(buf)
	return res
}

func (bpool *BufPool) Put(buf *Buf) {
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
	poolIx := bpool.bucket(cap(*ptr))
	bpool.buckets[poolIx].Put(ptr)
}
