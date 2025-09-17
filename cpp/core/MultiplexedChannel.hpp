// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <cstdint>
#include <vector>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "Common.hpp"
#include "Env.hpp"
#include "UDPSocketPair.hpp"

template<size_t ProtocolCount, std::array<uint32_t, ProtocolCount> Protocols>
class MultiplexedChannel {
public:
    // Returns false on signal interrupt
    template<size_t N>
    bool receiveMessages(Env& env, std::array<UDPSocketPair, N>& socks, UDPReceiver<N>& receiver, ssize_t maxMsgCount = -1, Duration timeout = -1, bool noPoll = false) {
        for (auto& v : _messages) {
            v.clear();
        }
        if (unlikely(!receiver.receiveMessages(env, socks, maxMsgCount, timeout, noPoll))) {
            return false;
        }
        for (int i = 0; i < N; i++) {
            for (UDPMessage& msg: receiver.messages()[i]) {
                uint32_t protocol = msg.buf.unpackScalar<uint32_t>();
                msg.buf.cursor = msg.buf.data;
                auto protocolIndex = _lookupProtocol(protocol);
                if (unlikely(protocolIndex == ProtocolCount)) {
                    RAISE_ALERT(env, "Could not parse request header. Unexpected protocol %s from %s, dropping it.", protocol, msg.clientAddr);
                } else {
                    _messages[protocolIndex].emplace_back(std::move(msg));
                }
            }
        }
        return true;
    }

    std::vector<UDPMessage>& protocolMessages(uint32_t protocol) {
        auto protocolIdx = _lookupProtocol(protocol);
        ALWAYS_ASSERT(protocolIdx != ProtocolCount);
        return _messages[protocolIdx];
    }
private:
    std::array<std::vector<UDPMessage>, ProtocolCount> _messages;

    static constexpr size_t _lookupProtocol(uint32_t prot) {
        for (size_t i = 0 ; i < Protocols.size(); ++i) {
            if (Protocols[i] == prot) {
                return i;
            }
        }
        return ProtocolCount;
    }
};
