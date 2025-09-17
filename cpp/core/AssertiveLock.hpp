// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <atomic>

#include "Common.hpp"
#include "Exception.hpp"

struct AssertiveLocked {
private:
    std::atomic<bool>& _held;
public:
    AssertiveLocked(std::atomic<bool>& held): _held(held) {
        bool expected = false;
        if (!_held.compare_exchange_strong(expected, true)) {
            throw TERN_EXCEPTION("could not aquire lock, are you using this function concurrently?");
        }
    }

    AssertiveLocked() = delete;
    AssertiveLocked& operator=(const AssertiveLocked&) = delete;

    ~AssertiveLocked() {
        _held.store(false);
    }
};

struct AssertiveLock {
private:
    std::atomic<bool> _held;
public:
    AssertiveLock(): _held(false) {}

    AssertiveLock(const AssertiveLock&) = delete;
    AssertiveLock& operator=(const AssertiveLock&) = delete;

    AssertiveLocked lock() {
        return AssertiveLocked(_held);
    }
};
