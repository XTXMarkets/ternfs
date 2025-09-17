// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <cstdint>
#include <netinet/in.h>
#include <poll.h>
#include <vector>

#include "Bincode.hpp"
#include "Connect.hpp"
#include "Env.hpp"
#include "Msgs.hpp"
#include "Loop.hpp"

struct UDPSocketPair {
    UDPSocketPair(Env& env, const AddrsInfo& addr, int32_t sockBufSize = 1 << 20);
    UDPSocketPair(const UDPSocketPair&) = delete;
    UDPSocketPair(UDPSocketPair&& s) : _addr(s._addr), _socks(std::move(s._socks)) {}

    const AddrsInfo& addr() const { return _addr; }
    const std::array<Sock, 2>& socks() const { return _socks; }

    int registerEpoll(int epollFd);

    bool containsFd(int fd) const {
        for (const auto& sock : _socks) {
            if (sock.error()) {
                continue;
            }
            if (sock.get() == fd) {
                return true;
            }
        }
        return false;
    }

private:
    void _initSock(uint8_t sockIdx, sockaddr_in& addr, int32_t sockBufSize);
    AddrsInfo _addr;
    std::array<Sock, 2> _socks;
};

struct UDPMessage {
    BincodeBuf buf;
    IpPort clientAddr;
    uint8_t socketIx;
};

struct UDPReceiverConfig {
    size_t perSockMaxRecvMsg = 127;
    size_t maxMsgSize = DEFAULT_UDP_MTU;
};

// Receives UDP messages from a set of UDP sockets.
template<size_t N>
struct UDPReceiver {
    UDPReceiver(const UDPReceiverConfig& config) : _perSockMaxRecvMsg(config.perSockMaxRecvMsg) {
        _recvBuf.resize(2*N * config.maxMsgSize * config.perSockMaxRecvMsg);
        _recvHdrs.resize(2*N * config.perSockMaxRecvMsg);
        memset(_recvHdrs.data(), 0, sizeof(_recvHdrs[0]) * 2*N * config.perSockMaxRecvMsg);
        _recvAddrs.resize(2*N * config.perSockMaxRecvMsg);
        _recvVecs.resize(2*N * config.perSockMaxRecvMsg);
        for (int i = 0; i < _recvVecs.size(); ++i) {
            _recvVecs[i].iov_base = &_recvBuf[i * config.maxMsgSize];
            _recvVecs[i].iov_len = config.maxMsgSize;
            _recvHdrs[i].msg_hdr.msg_iov = &_recvVecs[i];
            _recvHdrs[i].msg_hdr.msg_iovlen = 1;
            _recvHdrs[i].msg_hdr.msg_namelen = sizeof(_recvAddrs[i]);
            _recvHdrs[i].msg_hdr.msg_name = &_recvAddrs[i];
        }
    }

    UDPReceiver() : UDPReceiver(UDPReceiverConfig()) {}

    // Waits until any socket ready (or up to timeout) and receives up to `socketCount * perSockMaxRecvMsg` messages.
    // Returns false on signal interrupt.
    //
    // If maxMsgCount<0, the maximum available is chosen based on perSockMaxRecvMsg.
    // If maxMsgCount==0, true is immediately returned.
    bool receiveMessages(Env& env, const std::array<UDPSocketPair, N>& socks, ssize_t maxMsgCount = -1, Duration timeout = -1, bool noPoll = false) {
        for (auto& msg: _recvMsgs) {
            msg.clear();
        }
        if (maxMsgCount == 0) {
            return true;
        }
        if (maxMsgCount < 0) {
            maxMsgCount = N*2 * _perSockMaxRecvMsg;
        }
        // fill in FDs
        std::array<pollfd, N*2> fds;
        std::array<std::pair<uint8_t, uint8_t>, N*2> fdToSockIx;
        int numFds = 0;
        for (int sockIx1 = 0; sockIx1 < N; sockIx1++) {
            for (int sockIx2 = 0; sockIx2 < 2; sockIx2++) {
                if (socks[sockIx1].addr()[sockIx2].port == 0) { continue; }
                fds[numFds].fd = socks[sockIx1].socks()[sockIx2].get();
                fds[numFds].events = POLL_IN;
                fdToSockIx[numFds] = {sockIx1, sockIx2};
                numFds++;
            }
        }
        if (!noPoll) {
            // poll
            int err = Loop::poll(fds.data(), numFds, timeout);
            if (unlikely( err < 0)) {
                if (errno == EINTR) { return false; }
                throw SYSCALL_EXCEPTION("poll");
            }
            LOG_TRACE(env, "Poll returned, reading messages");
        }
        // read
        size_t messagesSoFar = 0;
        for (int i = 0; i < numFds; i++) {
            const pollfd& pfd = fds[i];
            if ((!noPoll) && (!(pfd.revents & POLLIN))) { continue; }
            const auto [sockIx1, sockIx2] = fdToSockIx[i];
            size_t maxMsgs = std::min(maxMsgCount-messagesSoFar, _perSockMaxRecvMsg);
            LOG_TRACE(env, "data on address %s, reading up to %s messages", socks[sockIx1].addr()[sockIx2], maxMsgs);
            int ret = recvmmsg(pfd.fd, &_recvHdrs[messagesSoFar], maxMsgs, MSG_DONTWAIT, nullptr);
            if (unlikely(ret < 0)) { 
                if (noPoll) {
                    continue;
                }
                // we know we have data from poll, we won't get EAGAIN
                throw SYSCALL_EXCEPTION("recvmmsgs");
            }
            for (int j = 0; j < ret; j++) {
                const auto& hdr = _recvHdrs[messagesSoFar+j];
                _recvMsgs[sockIx1].emplace_back(UDPMessage{
                    .buf = {(char*)hdr.msg_hdr.msg_iov[0].iov_base, hdr.msg_len},
                    .clientAddr = IpPort::fromSockAddrIn(_recvAddrs[messagesSoFar+j]),
                    .socketIx = sockIx2,
                });
            }
            messagesSoFar += ret;
            if (messagesSoFar >= maxMsgCount) {
                break;
            }
        }
        return true;
    }

    std::array<std::vector<UDPMessage>, N>& messages() {
        return _recvMsgs;
    }
private:
    size_t _perSockMaxRecvMsg;
    std::vector<char> _recvBuf;
    std::vector<mmsghdr> _recvHdrs;
    std::vector<sockaddr_in> _recvAddrs;
    std::vector<iovec> _recvVecs;
    std::array<std::vector<UDPMessage>, N> _recvMsgs;
};

struct UDPSenderConfig {
    uint16_t maxMsgSize = DEFAULT_UDP_MTU;
};

class UDPSender {
public:
    UDPSender(const UDPSenderConfig& config) : _maxMsgSize(config.maxMsgSize) {}
    UDPSender() : UDPSender(UDPSenderConfig()) {}

    template<typename Fill>
    void prepareOutgoingMessage(Env& env, const AddrsInfo& srcAddr, uint8_t srcSockIdx, const IpPort& dstAddr, Fill f) {
        ALWAYS_ASSERT(dstAddr.port != 0);

        size_t sendBufBegin = _sendBuf.size();
        _sendBuf.resize(sendBufBegin + _maxMsgSize);
        BincodeBuf respBbuf(&_sendBuf[sendBufBegin], _maxMsgSize);
        f(respBbuf);
        _sendBuf.resize(sendBufBegin + respBbuf.len());

        // Prepare sendmmsg stuff. The vectors might be resized by the
        // time we get to sending this, so store references when we must
        // -- we'll fix up the actual values later.
        auto& saddr = _sendAddrs[srcSockIdx].emplace_back();
        dstAddr.toSockAddrIn(saddr);
        auto& vec = _sendVecs[srcSockIdx].emplace_back();
        vec.iov_base = (void*)sendBufBegin;
        vec.iov_len = respBbuf.len();
        LOG_TRACE(env, "Prepared message of length(%s) from %s to %s", respBbuf.len(), srcAddr[srcSockIdx], dstAddr);
    }

    template<typename Fill>
    void prepareOutgoingMessage(Env& env, const AddrsInfo& srcAddr, const AddrsInfo& dstAddr, Fill f) {
        auto now = ternNow(); // randomly pick one of the dest addrs and one of our sockets
        uint8_t srcSockIdx = now.ns & (srcAddr[1].port != 0);
        uint8_t dstSockIdx = (now.ns>>1) & (dstAddr[1].port != 0);
        prepareOutgoingMessage(env, srcAddr, srcSockIdx, dstAddr[dstSockIdx], f);
    }

    void sendMessages(Env& env, const UDPSocketPair& socks);
private:
    uint16_t _maxMsgSize;

    // send buffers
    std::vector<char> _sendBuf;
    std::array<std::vector<sockaddr_in>, 2> _sendAddrs;
    std::array<std::vector<mmsghdr>, 2> _sendHdrs;
    std::array<std::vector<iovec>, 2> _sendVecs;
};
