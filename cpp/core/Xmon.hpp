// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <atomic>
#include <queue>
#include <mutex>
#include <unordered_map>
#include <semaphore>
#include <memory>
#include <unordered_set>
#include <poll.h>

#include "Common.hpp"
#include "Env.hpp"
#include "XmonAgent.hpp"
#include "Loop.hpp"

struct XmonConfig {
    std::string addr;
    XmonAppType appType = XmonAppType::NEVER;
    std::string appInstance = "";
};

struct XmonBuf {
    size_t cur = 0;
    size_t len = 0;
    char buf[4096];

    void reset() {
        cur = 0;
        len = 0;
    }

    void ensurePackSizeOrPanic(size_t sz) const {
        ALWAYS_ASSERT((sizeof(buf) - len) >= sz);
    }

    template<typename A>
    void packScalar(A x) {
        static_assert(std::is_integral_v<A> || std::is_enum_v<A>);
        static_assert(std::endian::native == std::endian::little);
        ensurePackSizeOrPanic(sizeof(A));
        if constexpr (sizeof(A) == 8) {
            x = (A)__builtin_bswap64((uint64_t)x);
        } else if constexpr (sizeof(A) == 4) {
            x = (A)__builtin_bswap32((uint32_t)x);
        } else if constexpr (sizeof(A) == 2) {
            x = (A)__builtin_bswap16((uint16_t)x);
        } else {
            static_assert(sizeof(A) == 1);
        }
        memcpy(buf+len, &x, sizeof(x));
        len += sizeof(A);
    }

    void packString(const std::string& str) {
        ALWAYS_ASSERT(str.size() < (1ull<<16)-1);
        packScalar<uint16_t>(str.size());
        ensurePackSizeOrPanic(str.size());
        memcpy(buf+len, str.data(), str.size());
        len += str.size();
    }

    void ensureUnpackSizeOrPanic(size_t sz) const {
        ALWAYS_ASSERT((sizeof(buf) - cur) >= sz);
    }

    template<typename A>
    A unpackScalar() {
        static_assert(std::is_integral_v<A> || std::is_enum_v<A>);
        static_assert(std::endian::native == std::endian::little);
        ensureUnpackSizeOrPanic(sizeof(A));
        A x;
        memcpy(&x, buf+cur, sizeof(x));
        if constexpr (sizeof(A) == 8) {
            x = (A)__builtin_bswap64((uint64_t)x);
        } else if constexpr (sizeof(A) == 4) {
            x = (A)__builtin_bswap32((uint32_t)x);
        } else if constexpr (sizeof(A) == 2) {
            x = (A)__builtin_bswap16((uint16_t)x);
        } else {
            static_assert(sizeof(A) == 1);
        }
        cur += sizeof(x);
        return x;
    }

    std::string unpackString() {
        int16_t sz = unpackScalar<int16_t>();
        ensurePackSizeOrPanic(sz);
        std::string s(sz, '\0');
        memcpy(s.data(), buf+cur, sz);
        cur += sz;
        return s;
    }

    // returns an error string if it failed
    std::string writeOut(int fd);
    void readIn(int fd, size_t sz, std::string& errString);
};

struct Xmon : Loop {
private:
    std::shared_ptr<XmonAgent> _agent;
    std::string _hostname;
    std::string _appInstance;
    XmonAppType _parent;
    std::vector<XmonAppType> _children;
    std::string _xmonHost;
    uint16_t _xmonPort;

    // xmon socket, xmon agent pipe read end, timer fd
    static constexpr int SOCK_FD = 0;
    static constexpr int PIPE_FD = 1;
    static constexpr int TIMER_FD = 2;
    static constexpr int NUM_FDS = 3;
    struct pollfd _fds[NUM_FDS];
    // requests we got from the pipe
    std::deque<XmonRequest> _queuedRequests;
    // active binnable alerts
    std::unordered_set<int64_t> _binnableAlerts;
    // quiet alerts we're waiting to send out
    struct QuietAlert {
        TernTime quietUntil;
        XmonAppType appType;
        std::string message;
    };
    std::unordered_map<int64_t, QuietAlert> _quietAlerts;
    // last heartbeat from xmon
    TernTime _gotHeartbeatAt;
    // what the timer fd expiration is currently set to
    TernTime _timerExpiresAt;

    XmonBuf _buf;

    void _packLogon(XmonBuf& buf);
    void _packUpdate(XmonBuf& buf);
    void _packRequest(XmonBuf& buf, const XmonRequest& req);
    void _ensureTimer(TernTime now, TernTime t);

    TernTime _stepNextWakeup();
public:
    Xmon(
        Logger& logger,
        std::shared_ptr<XmonAgent>& agent,
        const XmonConfig& config
    );
    virtual ~Xmon();

    virtual void step() override;
};
