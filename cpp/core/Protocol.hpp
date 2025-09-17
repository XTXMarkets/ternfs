// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "MsgsGen.hpp"

// >>> format(struct.unpack('<I', b'SHA\0')[0], 'x')
// '414853'
constexpr uint32_t SHARD_REQ_PROTOCOL_VERSION = 0x414853;

// >>> format(struct.unpack('<I', b'SHA\1')[0], 'x')
// '1414853'
constexpr uint32_t SHARD_RESP_PROTOCOL_VERSION = 0x1414853;

// >>> format(struct.unpack('<I', b'SHA\2')[0], 'x')
// '2414853'
constexpr uint32_t SHARD_LOG_PROTOCOL_VERSION = 0x2414853;

// >>> format(struct.unpack('<I', b'SHA\3')[0], 'x')
// '3414853'
constexpr uint32_t CDC_TO_SHARD_REQ_PROTOCOL_VERSION = 0x3414853;

// >>> format(struct.unpack('<I', b'SHA\4')[0], 'x')
// '4414853'
constexpr uint32_t CDC_TO_SHARD_RESP_PROTOCOL_VERSION = 0x4414853;

// >>> format(struct.unpack('<I', b'SHA\5')[0], 'x')
// '5414853'
constexpr uint32_t PROXY_SHARD_REQ_PROTOCOL_VERSION = 0x5414853;

// >>> format(struct.unpack('<I', b'SHA\6')[0], 'x')
// '6414853'
constexpr uint32_t PROXY_SHARD_RESP_PROTOCOL_VERSION = 0x6414853;

// >>> format(struct.unpack('<I', b'CDC\0')[0], 'x')
// '434443'
constexpr uint32_t CDC_REQ_PROTOCOL_VERSION = 0x434443;

// >>> format(struct.unpack('<I', b'CDC\1')[0], 'x')
// '1434443'
constexpr uint32_t CDC_RESP_PROTOCOL_VERSION = 0x1434443;

// >>> format(struct.unpack('<I', b'SHU\0')[0], 'x')
// '554853'
constexpr uint32_t REGISTRY_REQ_PROTOCOL_VERSION = 0x554853;

// >>> format(struct.unpack('<I', b'SHU\1')[0], 'x')
// '1554853'
constexpr uint32_t REGISTRY_RESP_PROTOCOL_VERSION = 0x1554853;

// >>> format(struct.unpack('<I', b'LOG\0')[0], 'x')
// '474f4c'
constexpr uint32_t  LOG_REQ_PROTOCOL_VERSION = 0x474f4c;

// >>> format(struct.unpack('<I', b'LOG\1')[0], 'x')
// '1474f4c'
constexpr uint32_t LOG_RESP_PROTOCOL_VERSION = 0x1474f4c;

struct ShardCheckPointedResp {
public:
    static constexpr size_t STATIC_SIZE = LogIdx::STATIC_SIZE + ShardRespContainer::STATIC_SIZE;
    size_t packedSize() const {
        return checkPointIdx.packedSize() + resp.packedSize();
    }
    void pack(BincodeBuf& buf) const {
        checkPointIdx.pack(buf);
        resp.pack(buf);
    }
    void unpack(BincodeBuf& buf) {
        checkPointIdx.unpack(buf);
        resp.unpack(buf);
    }
    bool operator==(const ShardCheckPointedResp& other) const {
        return checkPointIdx == other.checkPointIdx && resp == other.resp;
    }

    ShardMessageKind kind() const {
        return resp.kind();
    }

    void clear() {
        checkPointIdx = 0;
        resp.clear();
    }

    LogIdx  checkPointIdx;
    ShardRespContainer resp;
};

inline std::ostream& operator<<(std::ostream& out, const ShardCheckPointedResp& resp) {
    return out << "checkPointIdx: " << resp.checkPointIdx << " resp: " << resp.resp;
}

using ShardReqMsg = ProtocolMessage<SHARD_REQ_PROTOCOL_VERSION, ShardReqContainer>;
using ShardRespMsg = ProtocolMessage<SHARD_RESP_PROTOCOL_VERSION, ShardRespContainer>;
using CdcToShardReqMsg = SignedProtocolMessage<CDC_TO_SHARD_REQ_PROTOCOL_VERSION, ShardReqContainer>;
using CdcToShardRespMsg = SignedProtocolMessage<CDC_TO_SHARD_RESP_PROTOCOL_VERSION, ShardCheckPointedResp>;
using CDCReqMsg = ProtocolMessage<CDC_REQ_PROTOCOL_VERSION, CDCReqContainer>;
using CDCRespMsg = ProtocolMessage<CDC_RESP_PROTOCOL_VERSION, CDCRespContainer>;
using LogReqMsg = SignedProtocolMessage<LOG_REQ_PROTOCOL_VERSION, LogReqContainer>;
using LogRespMsg = SignedProtocolMessage<LOG_RESP_PROTOCOL_VERSION, LogRespContainer>;
using ProxyShardReqMsg = SignedProtocolMessage<PROXY_SHARD_REQ_PROTOCOL_VERSION, ShardReqContainer>;
using ProxyShardRespMsg = SignedProtocolMessage<PROXY_SHARD_RESP_PROTOCOL_VERSION, ShardCheckPointedResp>;
