// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <iostream>
#include <limits>
#include <time.h>
#include <fcntl.h>
#include <unistd.h>
#include <bit>
#include <sys/socket.h>
#include <unordered_set>
#include <sys/timerfd.h>
#include <charconv>

#include "Common.hpp"
#include "Env.hpp"
#include "Exception.hpp"
#include "Time.hpp"
#include "Xmon.hpp"
#include "Connect.hpp"
#include "XmonAgent.hpp"
#include "strerror.h"

static const std::vector<XmonAppType> appTypes = {XmonAppType::NEVER, XmonAppType::DAYTIME, XmonAppType::CRITICAL};

static const char* appTypeString(XmonAppType appType) {
    switch (appType) {
    case XmonAppType::NEVER:
        return "restech_eggsfs.never";
    case XmonAppType::DAYTIME:
        return "restech_eggsfs.daytime";
    case XmonAppType::CRITICAL:
        return "restech_eggsfs.critical";
    default:
        throw TERN_EXCEPTION("Bad xmon app type %s", (int)appType);
    }
}

enum struct XmonMood : int32_t {
    Happy = 0,
    Perturbed = 1,
    Upset = 2,
};

static std::string explicitGenerateErrString(const std::string& what, int err, const char* str) {
    std::stringstream ss;
    ss << "could not " << what << ": " << err << "/" << str;
    return ss.str();
}

static std::string generateErrString(const std::string& what, int err) {
    return explicitGenerateErrString(what, err, (std::string(translateErrno(err)) + "=" + safe_strerror(err)).c_str());
}

Xmon::Xmon(
    Logger& logger,
    std::shared_ptr<XmonAgent>& agent,
    const XmonConfig& config
) :
    Loop(logger, agent, "xmon"),
    _agent(agent),
    _appInstance(config.appInstance),
    _parent(config.appType)
{
    if (_appInstance.empty()) {
        throw TERN_EXCEPTION("empty app name");
    }

    {
        size_t colon = config.addr.find_last_of(':');
        if (colon == std::string_view::npos || colon == 0 || colon == config.addr.size()-1) {
            throw TERN_EXCEPTION("invalid xmon addr %s", config.addr);
        }

        std::string host = config.addr.substr(0, colon);
        std::string portString = config.addr.substr(colon + 1);

        uint64_t port64;
        auto [ptr, ec] = std::from_chars(portString.data(), portString.data() + portString.size(), port64);

        if (ec != std::errc() || ptr != portString.data() + portString.size()) {
            throw TERN_EXCEPTION("invalid xmon addr %s", config.addr);
        }

        if (port64 > ~(uint16_t)0) {
            throw TERN_EXCEPTION("invalid port %s", port64);
        }

        _xmonHost = host;
        _xmonPort = port64;
    }

    {
        char buf[HOST_NAME_MAX];
        int res = gethostname(buf, HOST_NAME_MAX);
        if (res != 0) {
            throw SYSCALL_EXCEPTION("gethostname");
        }
        _hostname = buf;
        auto end = _hostname.find(".");
        if (end != -1) {
            _hostname.resize(end);
        }
    }
    _appInstance = _appInstance + "@" + _hostname;

    {
        bool found = false;
        for (const auto& appType: appTypes) {
            if (appType == _parent) {
                found = true;
                continue;
            }
            _children.emplace_back(appType);
        }
        ALWAYS_ASSERT(found, "Did not find app type %s", (uint8_t)_parent);
    }

    // xmon socket
    _fds[SOCK_FD].fd = -1;
    _fds[SOCK_FD].events = POLLIN;
    // xmon agent read end
    _fds[PIPE_FD].fd = agent->readFd();
    _fds[PIPE_FD].events = POLLIN;
    // timer fd
    _fds[TIMER_FD].fd = timerfd_create(CLOCK_MONOTONIC, 0);
    if (_fds[TIMER_FD].fd < 0) {
        throw SYSCALL_EXCEPTION("timerfd_create");
    }
    _fds[TIMER_FD].events = POLLIN;

    // arm initial timer
    {
        auto now = ternNow();
        _timerExpiresAt = std::numeric_limits<uint64_t>::max();
        _ensureTimer(now, now);
    }
}

Xmon::~Xmon() {
    if (_fds[SOCK_FD].fd >= 0) {
        if (close(_fds[0].fd) < 0) {
            std::cerr << "Could not close xmon socket (" << errno << ")" << std::endl;
        }
    }
    if (close(_fds[TIMER_FD].fd) < 0) {
        std::cerr << "Could not close timer fd (" << errno << ")" << std::endl;
    }
}

void Xmon::_ensureTimer(TernTime now, TernTime t) {
    if (_timerExpiresAt <= t) { return; }
    Duration d = std::max<Duration>(1, t - now);
    LOG_DEBUG(_env, "arming timer in %s", d);
    struct itimerspec utmr;
    memset(&utmr, 0, sizeof(utmr));
    utmr.it_value = d.timespec();
    if (timerfd_settime(_fds[TIMER_FD].fd, 0, &utmr, nullptr) < 0) {
        throw SYSCALL_EXCEPTION("timerfd_settime");
    }
    _timerExpiresAt = t;
}

const Duration HEARTBEAT_INTERVAL = 10_sec;
const uint8_t HEARTBEAT_INTERVAL_SECS = HEARTBEAT_INTERVAL.ns/1'000'000'000ull;

void Xmon::_packLogon(XmonBuf& buf) {
    buf.reset();

    // Name 	Type 	Description
    // Magic 	int16 	always 'T'
    // Version 	int32 	version number (latest version is 4)
    // Message type 	int32 	0x0
    // (unused) 	int32 	must be 0
    // Hostname 	string 	your hostname
    // Heartbeat interval 	byte 	in seconds, 0 for default (5s))
    // (unused) 	byte 	must be 0
    // (unused) 	int16 	must be 0
    // App type 	string 	defines rota and schedule
    // App inst 	string 	arbitrary identifier
    // Heap size 	int64 	(deprecated)
    // Num threads 	int32 	(deprecated)
    // Num errors 	int32 	(deprecated)
    // Mood 	int32 	current mood
    // Details JSON 	string 	(unused)
    buf.packScalar<int16_t>('T'); // magic
    buf.packScalar<int32_t>(4);   // version
    buf.packScalar<int32_t>(0x0); // message type
    buf.packScalar<int32_t>(0x0); // unused
    buf.packString(_hostname); // hostname
    buf.packScalar<uint8_t>(HEARTBEAT_INTERVAL_SECS); // default interval
    buf.packScalar<uint8_t>(0); // unused
    buf.packScalar<int16_t>(0); // unused
    buf.packString(appTypeString(_parent)); // app type
    buf.packString(_appInstance); // app instance
    buf.packScalar<int64_t>(0); // heap size
    buf.packScalar<int32_t>(0); // num threads
    buf.packScalar<int32_t>(0); // num errors
    buf.packScalar<XmonMood>(XmonMood::Happy); // mood
    std::string details;
    buf.packString(details); // details

    for (const auto& child: _children) {
        buf.packScalar<int32_t>(0x100); // msg type
        buf.packScalar<int32_t>(0x0); // must be 0
        buf.packString(_hostname); // hostname
        buf.packScalar<uint8_t>(0); // unused
        buf.packScalar<uint8_t>(0); // unused
        buf.packScalar<int16_t>(0); // unused
        buf.packString(appTypeString(child)); // app type
        buf.packString(_appInstance); // app instance
        buf.packScalar<int64_t>(0); // heap size
        buf.packScalar<int32_t>(0); // num threads
        buf.packScalar<int32_t>(0); // num errors
        buf.packScalar<XmonMood>(XmonMood::Happy); // mood
        std::string details;
        buf.packString(details); // details
    }
}

void Xmon::_packUpdate(XmonBuf& buf) {
    buf.reset();
    // Message type 	int32 	0x1
    // (unused) 	int32 	must be 0
    // Heap size 	int64 	(deprecated)
    // Num threads 	int32 	(deprecated)
    // Num errors 	int32 	(deprecated)
    // Mood 	int32 	updated mood
    buf.packScalar<int32_t>(0x1);
    buf.packScalar<int32_t>(0);
    buf.packScalar<int64_t>(0);
    buf.packScalar<int32_t>(0);
    buf.packScalar<int32_t>(0);
    buf.packScalar<XmonMood>(XmonMood::Happy);
}

void Xmon::_packRequest(XmonBuf& buf, const XmonRequest& req) {
    buf.reset();

    bool hasMessage = req.msgType == XmonRequestType::CREATE || req.msgType == XmonRequestType::UPDATE;

    if (_parent == req.appType) {
        buf.packScalar(req.msgType);
        buf.packScalar(req.alertId);
        if (hasMessage) {
            buf.packScalar(req.binnable);
            buf.packString(req.message);
        }
    } else {
        buf.packScalar<int32_t>((int32_t)req.msgType | 0x100);
        buf.packString(appTypeString(req.appType));
        buf.packString(_appInstance);
        buf.packString(std::to_string(req.alertId));
        if (hasMessage) {
            buf.packScalar((int32_t)req.binnable);
            buf.packString(req.message);
            std::string json; // unused
            buf.packString(json);
        }
    }
}

std::string XmonBuf::writeOut(int fd) {
    const char* curr = buf + cur;
    const char* end = buf + len;
    while (curr < end) {
        ssize_t written = write(fd, curr, end-curr);
        if (written < 0) {
            return generateErrString("writing out xmon buf", errno);
        }
        if (written == 0) {
            return "unexpected EOF";
        }
        curr += written;
    }
    return {};
}

void XmonBuf::readIn(int fd, size_t sz, std::string& errString) {
    errString.clear();
    reset();
    ensureUnpackSizeOrPanic(sz);
    size_t readSoFar = 0;
    while (readSoFar < sz) {
        ssize_t r = read(fd, buf+cur+readSoFar, sz-readSoFar);
        if (unlikely(r < 0)) {
            errString = generateErrString("read in xmon buf", errno);
            return;
        }
        if (unlikely(r == 0)) {
            errString = "unexpected EOF";
            return;
        }
        readSoFar += r;
    }
    len += sz;
}

constexpr int MAX_BINNABLE_ALERTS = 20;

TernTime Xmon::_stepNextWakeup() {
    std::string errString;
    TernTime nextWakeup = std::numeric_limits<uint64_t>::max();

#define CHECK_ERR_STRING(__what) \
    if (errString.size()) { \
        LOG_INFO(_env, "could not %s: %s, will reconnect", __what, errString); \
        if (_fds[SOCK_FD].fd >= 0 && close(_fds[SOCK_FD].fd) < 0) { \
            LOG_INFO(_env, "could not close xmon socket after error: %s (%s)", errno, safe_strerror(errno)); \
        } \
        _fds[SOCK_FD].fd = -1; \
        (1_sec).sleep(); \
        return nextWakeup; \
    }

    errString.clear();

    // Not connected, connect
    if (_fds[SOCK_FD].fd < 0) {
        {
            auto [sock, err] = connectToHost(_xmonHost, _xmonPort);
            _fds[SOCK_FD].fd = sock.release();
            errString = err;
        }
        CHECK_ERR_STRING(errString);
        LOG_INFO(_env, "connected to xmon %s:%s", _xmonHost, _xmonPort);

        // Send logon message(s)
        _packLogon(_buf);
        errString = _buf.writeOut(_fds[SOCK_FD].fd);
        CHECK_ERR_STRING(errString);
        LOG_INFO(_env, "sent logon to xmon, appType=%s appInstance=%s", appTypeString(_parent), _appInstance);

        _gotHeartbeatAt = 0;
        nextWakeup = std::min(nextWakeup, ternNow() + HEARTBEAT_INTERVAL*2);
    }

    if (poll(_fds, NUM_FDS, -1) < 0) {
        if (errno == EINTR) { return nextWakeup; }
        throw SYSCALL_EXCEPTION("poll");
    }

    auto now = ternNow();

    if (_fds[SOCK_FD].revents & (POLLIN|POLLHUP|POLLERR)) {
        LOG_DEBUG(_env, "got event in sock fd");
        // Receive everything
        _buf.readIn(_fds[SOCK_FD].fd, 4, errString);
        CHECK_ERR_STRING("read message type");
        int32_t msgType = _buf.unpackScalar<int32_t>();
        switch (msgType) {
        case 0x0: {
            if (_gotHeartbeatAt == 0) {
                LOG_INFO(_env, "got first xmon heartbeat, will start sending requests");
            } else {
                LOG_DEBUG(_env, "got xmon heartbeat");
            }
            _gotHeartbeatAt = now;
            nextWakeup = std::min(nextWakeup, now + HEARTBEAT_INTERVAL*2);
            _packUpdate(_buf);
            errString = _buf.writeOut(_fds[SOCK_FD].fd);
            CHECK_ERR_STRING("send heartbeat");
            break; }
        case 0x1: {
            _buf.readIn(_fds[SOCK_FD].fd, 8, errString);
            CHECK_ERR_STRING("reading alert binned id");
            int64_t alertId = _buf.unpackScalar<int64_t>();
            LOG_INFO(_env, "got alert %s binned from UI", alertId);
            _binnableAlerts.erase(alertId);
            break; }
        case 0x101: {
            // we don't care about which child this is, alert ids are unique anyway
            // also this really ain't the best code
            _buf.readIn(_fds[SOCK_FD].fd, 2, errString);
            CHECK_ERR_STRING("reading child app type length");
            {
                uint16_t appTypeLen = _buf.unpackScalar<uint16_t>();
                _buf.readIn(_fds[SOCK_FD].fd, appTypeLen, errString);
                CHECK_ERR_STRING("reading child app type");
            }
            _buf.readIn(_fds[SOCK_FD].fd, 2, errString);
            CHECK_ERR_STRING("reading child app instance length");
            {
                uint16_t appInstanceLen = _buf.unpackScalar<uint16_t>();
                _buf.readIn(_fds[SOCK_FD].fd, appInstanceLen, errString);
                CHECK_ERR_STRING("reading child app instance");
            }
            _buf.readIn(_fds[SOCK_FD].fd, 2, errString);
            CHECK_ERR_STRING("reading alert id length");
            int64_t alertId;
            {
                uint16_t alertIdLen = _buf.unpackScalar<uint16_t>();
                _buf.readIn(_fds[SOCK_FD].fd, alertIdLen, errString);
                CHECK_ERR_STRING("reading alert id");
                size_t idx;
                alertId = std::stoll(std::string(_buf.buf, alertIdLen), &idx);
                if (idx != alertIdLen) {
                    errString = "could not parse alert id";
                    CHECK_ERR_STRING("parsing alert id");
                }
            }
            LOG_INFO(_env, "got alert %s binned from UI", alertId);
            _binnableAlerts.erase(alertId);
            break; }
        default:
            throw TERN_EXCEPTION("unknown message type %s", msgType);
        }
    }

    if (_fds[TIMER_FD].revents & POLLIN) {
        LOG_DEBUG(_env, "got event in timer FD");
        // drain timer
        {
            char buf[8];
            if (read(_fds[TIMER_FD].fd, buf, sizeof(buf)) != sizeof(buf)) {
                throw SYSCALL_EXCEPTION("read");
            }
        }
        _timerExpiresAt = std::numeric_limits<uint64_t>::max(); // it just fired

        // check if we're past the deadline
        if (_gotHeartbeatAt > 0) {
            nextWakeup = std::min(nextWakeup, _gotHeartbeatAt + HEARTBEAT_INTERVAL*2);
            LOG_DEBUG(_env, "nextWakeup=%s", nextWakeup);
            if ((now-_gotHeartbeatAt > HEARTBEAT_INTERVAL*2)) {
                LOG_INFO(_env, "heartbeat deadline has passed, will reconnect");
                if (close(_fds[SOCK_FD].fd) < 0) {
                    throw SYSCALL_EXCEPTION("close");
                }
                _fds[SOCK_FD].fd = -1;
                return nextWakeup;
            }
        }

        // unquiet alerts that are due
        for (auto it = _quietAlerts.begin(); it != _quietAlerts.end();) {
            if (now >= it->second.quietUntil) {
                nextWakeup = std::min(nextWakeup, it->second.quietUntil);
                _queuedRequests.emplace_back(XmonRequest{
                    .alertId = it->first,
                    .quietPeriod = 0,
                    .message = std::move(it->second.message),
                    .msgType = XmonRequestType::CREATE,
                    .appType = it->second.appType,
                    .binnable = false,
                });
                it = _quietAlerts.erase(it);
            } else {
                it++;
            }
        }
    }

    if (_fds[PIPE_FD].revents & (POLLIN|POLLHUP|POLLERR)) {
        LOG_DEBUG(_env, "got event in pipe FD");
        XmonRequest req;
        req.read(_fds[PIPE_FD].fd);
        if (req.appType == XmonAppType::DEFAULT) {
            req.appType = _parent;
        }
        _queuedRequests.emplace_back(std::move(req));
    }

    // Write out all requests, if socket is alive anyway
    if (_gotHeartbeatAt > 0) {
        while (!_queuedRequests.empty()) {
            const auto& req = _queuedRequests.front();
            LOG_INFO(_env, "got req of type %s alertId=%s", (int)XmonRequestType::CREATE, req.alertId);
            int64_t alertIdToInsert = -1;
            if (req.msgType == XmonRequestType::CREATE) {
                if (req.quietPeriod > 0) {
                    ALWAYS_ASSERT(!req.binnable, "got alert with quietPeriod=%s, but it is binnable", req.quietPeriod);
                    LOG_INFO(_env, "got non-binnable alertId=%s message=%s quietPeriod=%s, will wait", req.alertId, req.message, req.quietPeriod);
                    TernTime quietUntil = now + req.quietPeriod;
                    nextWakeup = std::min(nextWakeup, quietUntil);
                    _quietAlerts[req.alertId] = QuietAlert{
                        .quietUntil = quietUntil,
                        .appType = req.appType,
                        .message = std::move(req.message),
                    };
                    goto skip_request;
                } else if (req.binnable) {
                    alertIdToInsert = req.alertId;
                    if (_binnableAlerts.size() > MAX_BINNABLE_ALERTS) {
                        LOG_ERROR(_env, "not creating alert, aid=%s binnable=%s message=%s, we're full", req.alertId, req.binnable, req.message);
                        if (_binnableAlerts.count(XmonAgent::TOO_MANY_ALERTS_ALERT_ID) == 0) {
                            XmonRequest req{
                                .alertId = XmonAgent::TOO_MANY_ALERTS_ALERT_ID,
                                .quietPeriod = 0,
                                .message = "too many alerts, alerts dropped",
                                .msgType = XmonRequestType::CREATE,
                                .appType = _parent,
                                .binnable = true,
                            };
                            _packRequest(_buf, req);
                            alertIdToInsert = XmonAgent::TOO_MANY_ALERTS_ALERT_ID;
                            goto write_request;
                        } else {
                            goto skip_request;
                        }
                    }
                }
                LOG_INFO(_env, "creating alert, aid=%s binnable=%s message=%s", req.alertId, req.binnable, req.message);
            } else if (req.msgType == XmonRequestType::UPDATE) {
                ALWAYS_ASSERT(!req.binnable);
                auto quiet = _quietAlerts.find(req.alertId);
                if (quiet != _quietAlerts.end()) {
                    LOG_INFO(_env, "skipping update alertId=%s message=%s since it's still quiet", req.alertId, req.message);
                    quiet->second.message = std::move(req.message);
                    goto skip_request;
                }
                LOG_INFO(_env, "updating alert, aid=%s binnable=%s message=%s", req.alertId, req.binnable, req.message);
            } else if (req.msgType == XmonRequestType::CLEAR) {
                ALWAYS_ASSERT(!req.binnable);
                auto quiet = _quietAlerts.find(req.alertId);
                if (quiet != _quietAlerts.end()) {
                    LOG_INFO(_env, "skipping clear alertId=%s since it's still quiet", req.alertId);
                    _quietAlerts.erase(quiet);
                    goto skip_request;
                }
                LOG_INFO(_env, "clearing alert, aid=%s", req.alertId);
            } else {
                throw TERN_EXCEPTION("bad req type %s", (int)req.msgType);
            }
            _packRequest(_buf, req);
        write_request:
            errString = _buf.writeOut(_fds[SOCK_FD].fd);
            CHECK_ERR_STRING(errString);
            LOG_DEBUG(_env, "sent request to xmon");
            if (alertIdToInsert >= 0) {
                _binnableAlerts.insert(alertIdToInsert);
            }
        skip_request:
            _queuedRequests.pop_front();
        }
    }

    return nextWakeup;
}

void Xmon::step() {
    TernTime nextTimer = _stepNextWakeup();
    _ensureTimer(ternNow(), nextTimer);
}
