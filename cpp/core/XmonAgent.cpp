// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "XmonAgent.hpp"
#include "Assert.hpp"

#include <iostream>
#include <fcntl.h>

struct XmonRequestHeader {
    int64_t alertId;
    Duration quietPeriod;
    XmonRequestType msgType;
    XmonAppType appType;
    bool binnable;
    uint16_t messageLen;
};

void XmonRequest::write(int fd) const {
    ALWAYS_ASSERT(message.size() < 1<<16);
    XmonRequestHeader header = {
        .alertId = alertId,
        .quietPeriod = quietPeriod,
        .msgType = msgType,
        .appType = appType,
        .binnable = binnable,
        .messageLen = (uint16_t)message.size(),
    };
    static thread_local char buf[PIPE_BUF];
    ALWAYS_ASSERT(sizeof(header) + header.messageLen < PIPE_BUF);
    memcpy(buf, &header, sizeof(header));
    memcpy(buf+sizeof(header), message.data(), header.messageLen);
    {
        // pipe writes of < PIPE_BUF are guaranteed to be atomic, see pipe(7)
        int written = ::write(fd, buf, sizeof(header)+header.messageLen);
        if (written != sizeof(header)+header.messageLen) {
            throw SYSCALL_EXCEPTION("write");
        }
    }
}

bool XmonRequest::read(int fd) {
    static thread_local char buf[PIPE_BUF];
    XmonRequestHeader header;
    int read = ::read(fd, buf, sizeof(header));
    if (read != sizeof(header)) {
        throw SYSCALL_EXCEPTION("read");
    }
    memcpy(&header, buf, sizeof(header));
    appType = header.appType;
    msgType = header.msgType;
    alertId = header.alertId;
    quietPeriod = header.quietPeriod;
    binnable = header.binnable;
    read = ::read(fd, buf, header.messageLen);
    if (read != header.messageLen) {
        throw SYSCALL_EXCEPTION("read");
    }
    buf[header.messageLen] = '\0';
    message = std::string(buf);
    return true;
}

int64_t XmonAgent::_createAlert(XmonAppType appType, bool binnable, Duration quietPeriod, const std::string& message) {
    int64_t aid = _alertId.fetch_add(1);
    XmonRequest req;
    req.appType = appType;
    req.msgType = XmonRequestType::CREATE;
    req.alertId = aid;
    req.binnable = binnable;
    req.message = message;
    req.quietPeriod = quietPeriod;
    req.write(_pipe[1]);
    return aid;
}

XmonAgent::XmonAgent() : _alertId(1) {
    if (pipe(_pipe) < 0) {
        throw SYSCALL_EXCEPTION("pipe");
    }
    if (fcntl(readFd(), F_SETFL, O_NONBLOCK) < 0) {
        throw SYSCALL_EXCEPTION("fcntl");
    }
}

XmonAgent::~XmonAgent() {
    if (close(_pipe[0]) < 0) {
        std::cerr << "Could not close read end of pipe pipe (" << errno << ")" << std::endl;
    }
    if (close(_pipe[1]) < 0) {
        std::cerr << "Could not close write end of pipe pipe (" << errno << ")" << std::endl;
    }
}

void XmonAgent::raiseAlert(XmonAppType appType, const std::string& message) {
    _createAlert(appType, true, 0, message);
}

void XmonAgent::updateAlert(XmonNCAlert& aid, const std::string& message) {
    if (aid.alertId < 0) {
        aid.alertId = _createAlert(aid.appType, false, aid.quietPeriod, message);
    } else {
        XmonRequest req;
        req.appType = aid.appType;
        req.msgType = XmonRequestType::UPDATE;
        req.alertId = aid.alertId;
        req.binnable = false;
        req.message = message;
        req.quietPeriod = 0;
        req.write(_pipe[1]);
    }
}

void XmonAgent::clearAlert(XmonNCAlert& aid) {
    if (likely(aid.alertId < 0)) { return; }
    XmonRequest req;
    req.appType = aid.appType;
    req.msgType = XmonRequestType::CLEAR;
    req.alertId = aid.alertId;
    req.binnable = false;
    req.message = {};
    req.quietPeriod = 0;
    req.write(_pipe[1]);
    aid.alertId = -1;
}
