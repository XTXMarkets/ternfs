// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <time.h>

#include "Common.hpp"
#include "Bincode.hpp"

struct Duration {
    int64_t ns;

    constexpr Duration(): ns(0) {}
    constexpr Duration(int64_t ns_): ns(ns_) {}
    constexpr Duration(const struct timespec& ts): ns(ts.tv_sec*1'000'000'000ll + ts.tv_nsec) {}

    bool operator==(Duration rhs) const {
        return ns == rhs.ns;
    }

    bool operator>(Duration rhs) const {
        return ns > rhs.ns;
    }

    bool operator>=(Duration rhs) const {
        return ns >= rhs.ns;
    }

    bool operator<(Duration rhs) const {
        return ns < rhs.ns;
    }

    bool operator<=(Duration rhs) const {
        return ns <= rhs.ns;
    }

    Duration operator+(Duration d) const {
        return Duration(ns + d.ns);
    }

    Duration operator*(int64_t x) const {
        return Duration(ns * x);
    }

    Duration operator-(Duration d) const {
        return ns - d.ns;
    }

    struct timespec timespec() const {
        struct timespec ts;
        ts.tv_sec = ns / 1'000'000'000ll;
        ts.tv_nsec = ns % 1'000'000'000ll;
        return ts;
    }

    // sleeps, returns a non-zero duration if we were interrupted
    Duration sleep() const;

    // sleeps, retrying if we get EINTR
    void sleepRetry() const;
};

constexpr Duration operator "" _ns   (unsigned long long t) { return Duration(t); }
constexpr Duration operator "" _us   (unsigned long long t) { return Duration(t*1'000); }
constexpr Duration operator "" _ms   (unsigned long long t) { return Duration(t*1'000'000); }
constexpr Duration operator "" _sec  (unsigned long long t) { return Duration(t*1'000'000'000ull); }
constexpr Duration operator "" _mins (unsigned long long t) { return Duration(t*1'000'000'000ull*60); }
constexpr Duration operator "" _hours(unsigned long long t) { return Duration(t*1'000'000'000ull*60*60); }

std::ostream& operator<<(std::ostream& out, Duration d);

struct TernTime {
    uint64_t ns;

    TernTime(): ns(0) {}
    TernTime(uint64_t ns_): ns(ns_) {}

    bool operator==(TernTime rhs) const {
        return ns == rhs.ns;
    }

    bool operator>(TernTime rhs) const {
        return ns > rhs.ns;
    }

    bool operator>=(TernTime rhs) const {
        return ns >= rhs.ns;
    }

    bool operator<=(TernTime rhs) const {
        return ns <= rhs.ns;
    }

    bool operator<(TernTime rhs) const {
        return ns < rhs.ns;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint64_t>(ns);
    }

    void unpack(BincodeBuf& buf) {
        ns = buf.unpackScalar<uint64_t>();
    }

    TernTime operator+(Duration d) const {
        return TernTime(ns + d.ns);
    }

    TernTime operator-(Duration d) const {
        if (unlikely(d.ns > ns)) {
            return 0;
        }
        return TernTime(ns - d.ns);
    }

    Duration operator-(TernTime t) const {
        if (unlikely(t.ns > ns)) {
            return 0;
        }
        return Duration(ns - t.ns);
    }

    // Two positive times might give one negative
    // duration.
    #ifdef __clang__
    __attribute__((no_sanitize("integer")))
    #endif
    Duration operator-(TernTime d) {
        return Duration(ns - d.ns);
    }
};

std::ostream& operator<<(std::ostream& out, TernTime t);

// DO NOT USE UNLESS TESTING TIME SENSITIVE BEHAVIOR
void _setCurrentTime(TernTime time);

TernTime ternNow();
