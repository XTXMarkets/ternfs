// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "UDPSocketPair.hpp"

#include "Common.hpp"

#include <arpa/inet.h>
#include <cstddef>

UDPSocketPair::UDPSocketPair(Env& env, const AddrsInfo& addr_, int32_t sockBufSize) : _addr(addr_) {
    sockaddr_in saddr;
    for (int i = 0; i < 2; i++) {
        bool hasIp = _addr[i].ip != Ip({0,0,0,0});
        ALWAYS_ASSERT(i > 0 || hasIp, "The first IP address must be specified");
        if (!hasIp) { continue; }
        _addr[i].toSockAddrIn(saddr);
        _initSock(i, saddr, sockBufSize);
        _addr[i].port = ntohs(saddr.sin_port);
    }
    LOG_INFO(env, "Bound to addresses %s", _addr);
}

void UDPSocketPair::_initSock(uint8_t sockIdx, sockaddr_in& addr, int32_t sockBufSize) {
    auto sock = Sock::UDPSock();
    if (sock.error()) {
        throw SYSCALL_EXCEPTION("cannot create socket");
    }

    if (bind(sock.get(), (sockaddr*)&addr, sizeof(addr)) != 0) {
        char ip[INET_ADDRSTRLEN];
        throw SYSCALL_EXCEPTION("cannot bind socket to addr %s:%s", inet_ntop(AF_INET, &addr.sin_addr, ip, INET_ADDRSTRLEN), ntohs(addr.sin_port));
    }
    {
        socklen_t addrLen = sizeof(addr);
        if (getsockname(sock.get(), (sockaddr*)&addr, &addrLen) < 0) {
            throw SYSCALL_EXCEPTION("getsockname");
        }
    }
    {
        if (setsockopt(sock.get(), SOL_SOCKET, SO_RCVBUF, (void*)&sockBufSize, sizeof(sockBufSize)) < 0) {
            throw SYSCALL_EXCEPTION("setsockopt");
        }
    }
    _socks[sockIdx] = std::move(sock);
}


void UDPSender::sendMessages(Env& env, const UDPSocketPair& socks) {
    for (size_t i = 0; i < _sendAddrs.size(); ++i) {
        if (_sendAddrs[i].size() == 0) { continue; }
        LOG_TRACE(env, "sending %s messages to socket (%s)[%s]", _sendAddrs[i].size(), socks.addr(), i);
        for (size_t j = 0; j < _sendAddrs[i].size(); ++j) {
            auto& vec = _sendVecs[i][j];
            vec.iov_base = &_sendBuf[(size_t)vec.iov_base];
            auto& hdr = _sendHdrs[i].emplace_back();
            ALWAYS_ASSERT(_sendAddrs[i][j].sin_port != 0);
            hdr.msg_hdr = {
                .msg_name = (sockaddr_in*)&_sendAddrs[i][j],
                .msg_namelen = sizeof(_sendAddrs[i][j]),
                .msg_iov = &vec,
                .msg_iovlen = 1,
                .msg_control = nullptr,
                .msg_controllen = 0,
                .msg_flags = 0,
            };
            hdr.msg_len = vec.iov_len;
        }
        size_t sentMessages{0};
        int ret{1};
        while (sentMessages < _sendHdrs[i].size() && ret > 0) {
            ret = sendmmsg(socks.socks()[i].get(), &_sendHdrs[i][sentMessages], _sendHdrs[i].size() - sentMessages, 0);
            sentMessages += ret;
        }
        if (unlikely(ret < 0)) {
            switch (errno) {
            case EPERM: // we get this when nf drops packets
                LOG_INFO(env, "dropping %s messages because of EPERM", _sendHdrs[i].size());
                break;
            case ENETUNREACH: // we get this when nic flaps
                LOG_INFO(env, "dropping %s messages because of ENETUNREACH", _sendHdrs[i].size());
                break;
            default:
                throw SYSCALL_EXCEPTION("sendmmsg");
            }
        } else if (unlikely(sentMessages < _sendHdrs[i].size())) {
            LOG_INFO(env, "dropping %s out of %s messages since `sendmmsg` could not send them all", _sendHdrs[i].size()-sentMessages, _sendHdrs[i].size());
        }
    }

    _sendBuf.clear();
    for(size_t i = 0; i < _sendHdrs.size(); ++i) {
        _sendAddrs[i].clear();
        _sendHdrs[i].clear();
        _sendVecs[i].clear();
    }
}

int UDPSocketPair::registerEpoll(int epollFd) {
    for (auto& sock : _socks) {
        if (sock.error()) continue;
        int err = sock.registerEpoll(epollFd);
        if (err) {
            return err;
        }
    }
    return 0;
}
