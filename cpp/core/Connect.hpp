// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cstdint>
#include <string>
#include <sys/epoll.h>
#include <utility>

#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include "Assert.hpp"
#include "Time.hpp"

class Sock {
public:
    static Sock SockError(int errnum) { ALWAYS_ASSERT(errnum > 0); return {-errnum}; }
    static Sock TCPSock() { return {socket(AF_INET, SOCK_STREAM, 0)}; }
    static Sock UDPSock() { return {socket(AF_INET, SOCK_DGRAM, IPPROTO_UDP)}; }

    Sock(): _fd(-1) {}
    Sock(const Sock&) = delete;
    Sock(Sock&& s) : _fd(s._fd) { s._fd = -1; }
    ~Sock() { if (_fd > 0) { close(_fd); _fd = -1; } }
    Sock& operator=(Sock&& s) { this->~Sock(); this->_fd = s.release(); return *this; }

    bool error() const { return _fd < 0; }

    int getErrno() const { ALWAYS_ASSERT(_fd < 0); return -_fd; }
    int get() const { ALWAYS_ASSERT(_fd > 0); return _fd; }
    int release() { int fd = _fd; _fd = -1; return fd; }
    int registerEpoll(int epollFd) {
        epoll_event event{};
        event.events = EPOLLIN;
        event.data.fd = _fd;
        return epoll_ctl(epollFd, EPOLL_CTL_ADD, _fd, &event);
    }
    int unregisterEpoll(int epollFd) {
        return epoll_ctl(epollFd, EPOLL_CTL_DEL, _fd, nullptr);
    }
private:
    Sock(int fd) : _fd(fd) {}
    int _fd;
};

// For errors that we probably shouldn't retry, an exception is thrown. Otherwise
// -errno fd and error string.
std::pair<Sock, std::string> connectToHost(
    const std::string& host,
    uint16_t port,
    Duration timeout = 10_sec
);
