// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cerrno>
#include <ctime>
#include <vector>
#include <stdint.h>
#include <atomic>
#include <linux/futex.h>

#include "Assert.hpp"
#include "Common.hpp"
#include "Exception.hpp"
#include "Time.hpp"

// This queue is designed for batching writes. The intended
// usage is to set _maxSize to be the maximum write queue we want
// to have before dropping requests.
//
// Won't work unless it's a single thread pushing serially and a
// single thread pulling serially.

//There are often cases where consumer wants to wait for work on multiple queues.
//For that usecase a MultiSPSCWaiter implementation exists.

class MultiSPSCWaiter {
public:
    MultiSPSCWaiter() {}
    ~MultiSPSCWaiter() = default;
    bool wait(Duration timeout) {
        uint32_t queuesWithWork = _queuesWithWork.load(std::memory_order_acquire);
        for (; unlikely(queuesWithWork == 0); queuesWithWork = _queuesWithWork.load(std::memory_order_acquire)) {
            if (timeout == 0) {
                return false;
            }
            timespec spec = timeout.timespec();
            long ret = syscall(SYS_futex, &_queuesWithWork, FUTEX_WAIT_PRIVATE, 0, timeout < 0 ? nullptr : &spec,  nullptr, 0);
            if (unlikely(ret != 0)) {
                if (errno == ETIMEDOUT) {
                    return false;
                }
                if (errno != EAGAIN) {
                    throw SYSCALL_EXCEPTION("futex");
                }
            }
        }
        return true;
    }
private:
    alignas(64) std::atomic<uint32_t> _queuesWithWork{0};
    void addWork() {
        uint32_t prev = _queuesWithWork.fetch_add(1, std::memory_order_relaxed);
        if (unlikely(prev == 0)) {
            long ret = syscall(SYS_futex, &_queuesWithWork, FUTEX_WAKE_PRIVATE, 1, nullptr, nullptr, 0);
            if (unlikely(ret < 0)) {
                throw SYSCALL_EXCEPTION("futex");
            }
        }
    }
    void removeWork() {
        uint32_t prev = _queuesWithWork.fetch_sub(1, std::memory_order_relaxed);
        ALWAYS_ASSERT(prev > 0);
    }
    template<typename A, bool MultiWaiter>
    friend class SPSC;
};

template<typename A, bool MultiWaiter = false>
class SPSC {
public:
    SPSC(uint32_t maxSize) requires(!MultiWaiter):
        _maxSize(maxSize),
        _sizeMask(maxSize-1),
        _head(0),
        _tail(0),
        _elements(maxSize)
    {
        ALWAYS_ASSERT(_maxSize > 0);
        ALWAYS_ASSERT((_maxSize&_sizeMask) == 0);
        ALWAYS_ASSERT(_maxSize < (1ull<<31));
    }

    SPSC(uint32_t maxSize, MultiSPSCWaiter& waiter) requires(MultiWaiter):
        _maxSize(maxSize),
        _sizeMask(maxSize-1),
        _head(0),
        _tail(0),
        _elements(maxSize),
        _waiter(&waiter)
    {
        ALWAYS_ASSERT(_maxSize > 0);
        ALWAYS_ASSERT((_maxSize&_sizeMask) == 0);
        ALWAYS_ASSERT(_maxSize < (1ull<<31));
    }

    // This will interrupt pullers.
    void close() {
        auto prev = _size.fetch_or(1ull<<31, std::memory_order_relaxed);
        if (prev & (1ull<<31)) {
            // already closed nothing to do
            return;
        }
        if (MultiWaiter) {
            if (prev > 0) {
                _waiter->removeWork();
            }
        } else {
            if (prev == 0) {
                long ret = syscall(SYS_futex, &_size, FUTEX_WAKE_PRIVATE, 1, nullptr, nullptr, 0);
                if (unlikely(ret < 0)) {
                    throw SYSCALL_EXCEPTION("futex");
                }
            }
        }
    }

    bool isClosed() {
        return _size.load(std::memory_order_relaxed) & (1ull<<31);
    }

    // Tries to push all the elements. Returns how many were actually
    // pushed. First element in `els` gets pushed first. Returns 0
    // if the queue is closed.
    uint32_t push(std::vector<A>& els) {
        uint32_t sz = _size.load(std::memory_order_relaxed);

        // queue is closed
        if (unlikely(sz & (1ull<<31))) { return 0; }

        // push as much as possible
        uint32_t toPush = std::min<uint64_t>(els.size(), _maxSize-sz);
        for (uint32_t i = 0; i < toPush; i++, _head++) {
            _elements[_head&_sizeMask] = std::move(els[i]);
        }

        // update size and wake up puller, if necessary
        uint32_t szBefore = _size.fetch_add(toPush, std::memory_order_release);
        // wake up puller only if we are not closed we were empty before and we actually pushed something
        if (unlikely(szBefore == 0 && (sz & (1ull<<31)) == 0 && toPush > 0)) {
            if (MultiWaiter) {
                _waiter->addWork();
                // we don't need to do futex wake here, since the waiter
                // is going to do it for us
            } else {
                long ret = syscall(SYS_futex, &_size, FUTEX_WAKE_PRIVATE, 1, nullptr, nullptr, 0);
                if (unlikely(ret < 0)) {
                    throw SYSCALL_EXCEPTION("futex");
                }
            }
        }

        return toPush;
    }

    // Drains at least one element, blocking if there are no elements,
    // unless the queue is closed or the operation times out, in which case it'll return 0.
    // Returns how many we've drained.
    uint32_t pull(std::vector<A>& els, uint32_t max, Duration timeout = -1) requires(!MultiWaiter) {
        uint32_t sz = _size.load(std::memory_order_acquire);
        for (; unlikely(sz == 0); sz = _size.load(std::memory_order_acquire)) {
            if (timeout == 0) {
                return 0;
            }
            timespec spec = timeout.timespec();
            long ret = syscall(SYS_futex, &_size, FUTEX_WAIT_PRIVATE, 0, timeout < 0 ? nullptr : &spec,  nullptr, 0);
            if (unlikely(ret != 0)) {
                if (errno == ETIMEDOUT) {
                    return 0;
                }
                if (errno != EAGAIN) {
                    throw SYSCALL_EXCEPTION("futex");
                }
            }
        }
        if (unlikely(sz & (1ull<<31))) { // queue is closed
            return 0;
        }
        // we've got something to drain
        uint32_t toDrain = std::min(sz, max);
        for (uint64_t i = _tail; i < _tail+toDrain; i++) {
            els.emplace_back(std::move(_elements[i&_sizeMask]));
        }
        _size.fetch_sub(toDrain, std::memory_order_release);
        _tail += toDrain;
        return toDrain;
    }

    // Drains up to max elements, returning immediately if there is nothing to drain.
    uint32_t pull(std::vector<A>& els, uint32_t max) requires(MultiWaiter) {
        uint32_t sz = _size.load(std::memory_order_acquire);
        if (unlikely(sz == 0 || sz & (1ull<<31))) { // queue is empty or closed
            return 0;
        }
        // we've got something to drain
        uint32_t toDrain = std::min(sz, max);
        for (uint64_t i = _tail; i < _tail+toDrain; i++) {
            els.emplace_back(std::move(_elements[i&_sizeMask]));
        }
        if (_size.fetch_sub(toDrain, std::memory_order_release) == toDrain) {
            _waiter->removeWork();
        }
        _tail += toDrain;
        return toDrain;
    }

    // don't return misleading numbers for a closed queue
    uint32_t size() const {
        return _size.load(std::memory_order_relaxed) & ~(1ull<<31);
    }

private:
    uint32_t _maxSize;
    uint32_t _sizeMask;
    // If the highest bit of size is set, then the queue is closed,
    // and push/pull will always return zero.
    alignas(64) std::atomic<uint32_t> _size;
    alignas(64) uint64_t _head;
    alignas(64) uint64_t _tail;
    std::vector<A> _elements;
    // Used only if MultiWaiter == true
    MultiSPSCWaiter* _waiter{nullptr};
};
