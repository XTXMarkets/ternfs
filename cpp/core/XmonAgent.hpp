// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "Time.hpp"

#include <stdint.h>
#include <atomic>
#include <vector>

enum struct XmonRequestType : int32_t {
    CREATE = 0x4,
    UPDATE = 0x5,
    CLEAR  = 0x3,
};

enum struct XmonAppType : uint8_t {
    DEFAULT = 0, // same as the parent app type
    NEVER = 1,
    DAYTIME = 2,
    CRITICAL = 3,
};

struct XmonRequest {
    int64_t alertId;
    Duration quietPeriod;
    std::string message;
    XmonRequestType msgType;
    XmonAppType appType;
    bool binnable;

    // multiple writers are fine.
    void write(int fd) const;

    // Only one reader at a time, please. Return false
    // if we got EAGAIN immediately.
    bool read(int fd);
};

struct XmonNCAlert {
    int64_t alertId;
    Duration quietPeriod;
    XmonAppType appType;

    XmonNCAlert() : alertId(-1), quietPeriod(0), appType(XmonAppType::DEFAULT) {}
    XmonNCAlert(XmonAppType appType_) : alertId(-1), quietPeriod(0), appType(appType_) {}
    XmonNCAlert(XmonAppType appType_, Duration quietPeriod_) : alertId(-1), quietPeriod(quietPeriod_), appType(appType_) {}
    XmonNCAlert(Duration quietPeriod_) : alertId(-1), quietPeriod(quietPeriod_), appType(XmonAppType::DEFAULT) {}
};

struct XmonAgent {
private:
    int _pipe[2]; // {readfd, writefd}
    std::atomic<int64_t> _alertId;

    int64_t _createAlert(XmonAppType appType, bool binnable, Duration quietPeriod, const std::string& message);

public:
    static constexpr int64_t TOO_MANY_ALERTS_ALERT_ID = 0;

    XmonAgent();
    ~XmonAgent();

    void raiseAlert(XmonAppType appType, const std::string& message);
    void updateAlert(XmonNCAlert& aid, const std::string& message);
    void clearAlert(XmonNCAlert& aid);

    int readFd() const {
        return _pipe[0];
    }
};
