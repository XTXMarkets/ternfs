// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <atomic>
#include <stdio.h>
#include <time.h>
#include <chrono>
#include <thread>

#include "Exception.hpp"
#include "Time.hpp"
#include "Assert.hpp"

std::ostream& operator<<(std::ostream& out, Duration d) {
    if (d.ns < 1'000) {
        out << d.ns << "ns";
    } else if (d.ns < 1'000'000) {
        out << d.ns/1'000 << "." << d.ns%1'000 << "us";
    } else if (d.ns < 1'000'000'000) {
        out << d.ns/1'000'000 << "." << d.ns%1'000'000 << "ms";
    } else if (d.ns < 1'000'000'000ull*60) {
        out << d.ns/1'000'000'000 << "." << d.ns%1'000'000'000 << "s";
    } else if (d.ns < 1'000'000'000ull*60*60) {
        out << d.ns/(1'000'000'000ull*60) << "." << d.ns%(1'000'000'000ull*60) << "m";
    } else {
        out << d.ns/(1'000'000'000ull*60*60) << "." << d.ns%(1'000'000'000ull*60*60) << "h";
    }
    return out;
}

void Duration::sleepRetry() const {
    struct timespec ts = timespec();
    for (;;) {
        int ret = nanosleep(&ts, &ts);
        if (likely(ret == 0)) {
            return;
        }
        if (likely(errno == EINTR)) {
            continue;
        }
        throw SYSCALL_EXCEPTION("nanosleep");
    }
}

Duration Duration::sleep() const {
    struct timespec ts = timespec();
    int ret = nanosleep(&ts, &ts);
    if (likely(ret == 0)) {
        return 0;
    }
    if (likely(errno == EINTR)) {
        return Duration(ts);
    }
    throw SYSCALL_EXCEPTION("nanosleep");
}

__attribute__((constructor))
static void checkClockRes() {
    struct timespec ts;
    if (clock_getres(CLOCK_REALTIME, &ts) != 0) {
        throw SYSCALL_EXCEPTION("clock_getres");
    }
    if (ts.tv_sec != 0 || ts.tv_nsec != 1) {
        throw TERN_EXCEPTION("expected nanosecond precisions, got %s,%s", ts.tv_sec, ts.tv_nsec);
    }
}

static std::atomic<TernTime> _currentTimeInTest = TernTime(0);

void _setCurrentTime(TernTime time) {
    _currentTimeInTest.store(time, std::memory_order_relaxed);
}

TernTime ternNow() {
    auto timeInTest = _currentTimeInTest.load(std::memory_order_relaxed);
    if (unlikely( timeInTest != 0)) {
        return timeInTest;
    }
    struct timespec now;

    if (clock_gettime(CLOCK_REALTIME, &now) != 0) {
        throw SYSCALL_EXCEPTION("clock_gettime");
    }

    return TernTime(((uint64_t)now.tv_nsec + ((uint64_t)now.tv_sec * 1'000'000'000ull)));
}

std::ostream& operator<<(std::ostream& out, TernTime ternt) {
    time_t secs = ternt.ns / 1'000'000'000ull;
    uint64_t nsecs = ternt.ns % 1'000'000'000ull;
    struct tm tm;
    if (gmtime_r(&secs, &tm) == nullptr) {
        throw SYSCALL_EXCEPTION("gmtime_r");
    }
    // "2006-01-02T15:04:05.999999999Z"
    char buf[31];
    ALWAYS_ASSERT(strftime(buf, 21, "%Y-%m-%dT%H:%M:%S.", &tm) == 20);
    ALWAYS_ASSERT(snprintf(buf + 20, 10, "%09lu", nsecs) == 9);
    ALWAYS_ASSERT(snprintf(buf + 29, 2, "Z") == 1);
    buf[30] = '\0';
    out << buf;
    return out;
}

void sleepFor(Duration dt) {
    std::this_thread::sleep_for(std::chrono::nanoseconds(dt.ns));
}

