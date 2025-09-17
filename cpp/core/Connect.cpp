// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <memory>
#include <netdb.h>
#include <netinet/tcp.h>
#include <sys/types.h>
#include <sys/socket.h>

#include "Connect.hpp"
#include "Assert.hpp"
#include "Exception.hpp"

static std::string explicitGenerateErrString(const std::string& what, int err, const char* str) {
    std::stringstream ss;
    ss << "could not " << what << ": " << err << "/" << str;
    return ss.str();
}

static std::string generateErrString(const std::string& what, int err) {
    return explicitGenerateErrString(what, err, (std::string(translateErrno(err)) + "=" + safe_strerror(err)).c_str());
}

std::pair<Sock, std::string> connectToHost(
    const std::string& host,
    uint16_t port,
    Duration timeout
) {
    int synRetries = 3;
    std::unique_ptr<struct addrinfo, decltype(&freeaddrinfo)> infos(nullptr, &freeaddrinfo);
    {
        char portStr[10];
        snprintf(portStr, sizeof(portStr), "%d", port);
        struct addrinfo hint;
        memset(&hint, 0, sizeof(hint));
        hint.ai_family = AF_INET;
        struct addrinfo* infosRaw;
        int res = getaddrinfo(host.c_str(), portStr, &hint, &infosRaw);
        if (res != 0) {
            if (res == EAI_SYSTEM) { // errno is filled in in this case
                throw SYSCALL_EXCEPTION("getaddrinfo");
            }
            std::string prefix = "resolve host " + host + ":" + std::to_string(port);
            if (res == EAI_ADDRFAMILY || res == EAI_AGAIN || res == EAI_NONAME) { // things that might be worth retrying
                return {Sock::SockError(EIO), explicitGenerateErrString(prefix, res, gai_strerror(res))};
            }
            throw TERN_EXCEPTION("%s: %s/%s", prefix, res, gai_strerror(res)); // we're probably hosed
        }
        infos.reset(infosRaw);
    }

    int err;
    std::string errStr;
    for (struct addrinfo* info = infos.get(); info != nullptr; info = info->ai_next) {
        auto sock = Sock::TCPSock();
        if (sock.error()) {
            throw SYSCALL_EXCEPTION("socket");
        }

        if (synRetries > 0) {
            if (setsockopt(sock.get(), IPPROTO_TCP, TCP_SYNCNT, &synRetries, sizeof(synRetries)) < 0) {
                err = errno;
                errStr = generateErrString("setsockopt", errno);
                continue;
            }
        }

        if (timeout > 0) {
            int userTimeout = timeout.ns / 1'000'000ull;
            if (setsockopt(sock.get(), IPPROTO_TCP, TCP_USER_TIMEOUT, &userTimeout, sizeof(userTimeout)) < 0) {
                err = errno;
                errStr = generateErrString("setsockopt", errno);
                continue;
            }
        }

        {
            // set keepalive as TCP_USER_TIMEOUT does not affect receive. TCP_KEEPIDLE + TCP_KEEPINTVL * TCP_KEEPCNT (4 + 2 * 3) = 10 sec
            // note that if TCP_USER_TIMEOUT is set to more than 10_sex TCP_KEEP_CNT is ignored until total time is > than TCP_USER_TIMEOUT
            // heartbeats are still sent every TCP_KEEPINTVL sec
            int optValue = 1;
            if (setsockopt(sock.get(), SOL_SOCKET, SO_KEEPALIVE, &optValue, sizeof(optValue)) < 0) {
                err = errno;
                errStr = generateErrString("setsockopt", errno);
                continue;
            }
            optValue = 4;
            if (setsockopt(sock.get(), IPPROTO_TCP, TCP_KEEPIDLE, &optValue, sizeof(optValue)) < 0) {
                err = errno;
                errStr = generateErrString("setsockopt", errno);
                continue;
            }
            optValue = 2;
            if (setsockopt(sock.get(), IPPROTO_TCP, TCP_KEEPINTVL, &optValue, sizeof(optValue)) < 0) {
                err = errno;
                errStr = generateErrString("setsockopt", errno);
                continue;
            }
            optValue = 3;
            if (setsockopt(sock.get(), IPPROTO_TCP, TCP_KEEPCNT, &optValue, sizeof(optValue)) < 0) {
                err = errno;
                errStr = generateErrString("setsockopt", errno);
                continue;
            }
        }

        if (connect(sock.get(), info->ai_addr, info->ai_addrlen) < 0) {
            err = errno;
            errStr = generateErrString("connect", errno);
        } else {
            return {std::move(sock), ""};
        }
    }

    ALWAYS_ASSERT(err > 0);
    return {Sock::SockError(err), errStr};
}
