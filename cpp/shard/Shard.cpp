// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "Shard.hpp"

#include <atomic>
#include <cstdint>
#include <memory>
#include <ostream>
#include <string>
#include <unordered_map>
#include <vector>
#include <fcntl.h>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "BlockServicesCacheDB.hpp"
#include "CDCKey.hpp"
#include "Common.hpp"
#include "Crypto.hpp"
#include "Env.hpp"
#include "ErrorCount.hpp"
#include "Exception.hpp"
#include "LogsDB.hpp"
#include "Loop.hpp"
#include "Metrics.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "MultiplexedChannel.hpp"
#include "PeriodicLoop.hpp"
#include "Protocol.hpp"
#include "Random.hpp"
#include "ShardDB.hpp"
#include "ShardKey.hpp"
#include "SharedRocksDB.hpp"
#include "RegistryClient.hpp"
#include "SPSC.hpp"
#include "Time.hpp"
#include "Timings.hpp"
#include "UDPSocketPair.hpp"
#include "Xmon.hpp"

struct ShardReq {
    uint32_t protocol;
    ShardReqMsg msg;
    TernTime receivedAt;
    IpPort clientAddr;
    int sockIx; // which sock to use to reply
};

struct ShardResp {
    ShardRespMsg msg;
    IpPort clientAddr;
};

struct ProxyLogsDBRequest {
    IpPort clientAddr;
    int sockIx; // which sock to use to reply
    LogsDBRequest request;
};

struct ProxyLogsDBResponse {
    LogsDBResponse response;
};

// TODO make options
const int WRITER_QUEUE_SIZE = 8192;
const int READER_QUEUE_SIZE = 1024;
const int MAX_RECV_MSGS = 100;

std::ostream& operator<<(std::ostream& out, const std::vector<FullShardInfo>& shards) {
    out << "[";
    for (const auto& s : shards) {
        out << s << ", ";
    }
    return out << "]";

}

struct ShardShared {
    const ShardOptions& options;

    // network
    std::array<UDPSocketPair, 1> socks; // in an array to play with UDPReceiver<>

    // server to -> reader/writer communication
    MultiSPSCWaiter writeQueuesWaiter;
    SPSC<LogsDBRequest, true> logsDBRequestQueue;
    SPSC<LogsDBResponse, true> logsDBResponseQueue;
    SPSC<ProxyLogsDBRequest, true> proxyLogsDBRequestQueue;
    SPSC<ProxyLogsDBResponse, true> proxyLogsDBResponseQueue;
    SPSC<ShardReq, true> writerShardReqQueue;
    SPSC<ShardResp, true> writerShardRespQueue;
    SPSC<ProxyShardRespMsg, true> writerProxyShardRespQueue;
    std::vector<std::unique_ptr<SPSC<ShardReq>>> readerRequestsQueues;

    // databases and caches
    SharedRocksDB& sharedDB;
    LogsDB& logsDB;
    ShardDB& shardDB;
    BlockServicesCacheDB& blockServicesCache;

    // statistics
    std::array<Timings, maxShardMessageKind+1> timings;
    std::array<ErrorCount, maxShardMessageKind+1> errors;
    std::atomic<uint64_t> receivedLogsDBReqs;
    std::atomic<uint64_t> receivedLogsDBResps;
    std::atomic<uint64_t> receivedProxyLogsDBReqs;
    std::atomic<uint64_t> receivedProxyLogsDBResps;
    std::atomic<uint64_t> receivedWriterReqs;
    std::atomic<uint64_t> receivedWriterProxyResps;
    std::atomic<uint64_t> receivedReaderReqs;
    std::array<std::atomic<uint64_t>, 2> receivedRequests; // how many requests we got at once from each socket

    // how many requests we dropped
    std::atomic<uint64_t> droppedLogsDBReqs;
    std::atomic<uint64_t> droppedLogsDBResps;
    std::atomic<uint64_t> droppedProxyLogsDBReqs;
    std::atomic<uint64_t> droppedProxyLogsDBResps;
    std::atomic<uint64_t> droppedWriteReqs;
    std::atomic<uint64_t> droppedProxyWriteResps;
    std::atomic<uint64_t> droppedReadReqs;

    // we should get up to date information from registry before we start serving any requests
    // this is populated by ShardRegisterer
    std::atomic<bool> isInitiated;
    std::atomic<bool> isBlockServiceCacheInitiated;

    RegistryClient registryClient;

    ShardShared() = delete;
    ShardShared(const ShardOptions& options_, SharedRocksDB& sharedDB_, BlockServicesCacheDB& blockServicesCache_, ShardDB& shardDB_, LogsDB& logsDB_, UDPSocketPair&& sock, Logger& logger, std::shared_ptr<XmonAgent>& xmon) :
        options(options_),
        socks({std::move(sock)}),
        logsDBRequestQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        logsDBResponseQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        proxyLogsDBRequestQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        proxyLogsDBResponseQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        writerShardReqQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        writerShardRespQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        writerProxyShardRespQueue(WRITER_QUEUE_SIZE, writeQueuesWaiter),
        sharedDB(sharedDB_),
        logsDB(logsDB_),
        shardDB(shardDB_),
        blockServicesCache(blockServicesCache_),
        receivedLogsDBReqs(0),
        receivedLogsDBResps(0),
        receivedProxyLogsDBReqs(0),
        receivedProxyLogsDBResps(0),
        receivedWriterReqs(0),
        receivedWriterProxyResps(0),
        receivedReaderReqs(0),
        droppedLogsDBReqs(0),
        droppedLogsDBResps(0),
        droppedProxyLogsDBReqs(0),
        droppedProxyLogsDBResps(0),
        droppedWriteReqs(0),
        droppedProxyWriteResps(0),
        droppedReadReqs(0),
        isInitiated(false),
        isBlockServiceCacheInitiated(false),
        registryClient(logger, xmon, options_.registryClientOptions.host, options_.registryClientOptions.port, options_.registryClientOptions.timeout),
        _replicas(nullptr),
        _leadersAtOtherLocations(std::make_shared<const std::vector<FullShardInfo>>())
    {
        for (uint16_t i = 0; i < options_.numReaders; ++i) {
            readerRequestsQueues.emplace_back(std::make_unique<SPSC<ShardReq>>(READER_QUEUE_SIZE));
        }
        for (ShardMessageKind kind : allShardMessageKind) {
            timings[(int)kind] = Timings::Standard();
        }
        for (auto& x: receivedRequests) {
            x = 0;
        }
    }

    const UDPSocketPair& sock() const {
        return socks[0];
    }

    inline std::shared_ptr<const std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> getReplicas() const {
        return _replicas.load();
    }

    inline void setReplicas(const std::array<AddrsInfo, LogsDB::REPLICA_COUNT>&& replicas) {
        _replicas.store(std::make_shared<const std::array<AddrsInfo, LogsDB::REPLICA_COUNT>>(replicas));
    }

    inline std::shared_ptr<const std::vector<FullShardInfo>> getLeadersAtOtherLocations() const {
        return _leadersAtOtherLocations.load();
    }

    inline void setLeadersAtOtherLocations(const std::vector<FullShardInfo>&& leadersAtOtherLocations) {
        _leadersAtOtherLocations.store(std::make_shared<const std::vector<FullShardInfo>>(leadersAtOtherLocations));
    }

private:
    // replication information
    std::atomic<std::shared_ptr<const std::array<AddrsInfo, LogsDB::REPLICA_COUNT>>> _replicas; // used for replicating shard within location
    std::atomic<std::shared_ptr<const std::vector<FullShardInfo>>> _leadersAtOtherLocations; // used for cross location replication
};

static bool bigRequest(ShardMessageKind kind) {
    return unlikely(
        kind == ShardMessageKind::ADD_SPAN_INITIATE ||
        kind == ShardMessageKind::ADD_SPAN_CERTIFY
    );
}

static bool bigResponse(ShardMessageKind kind) {
    return unlikely(
        kind == ShardMessageKind::READ_DIR ||
        kind == ShardMessageKind::ADD_SPAN_INITIATE ||
        kind == ShardMessageKind::LOCAL_FILE_SPANS ||
        kind == ShardMessageKind::FILE_SPANS ||
        kind == ShardMessageKind::VISIT_DIRECTORIES ||
        kind == ShardMessageKind::VISIT_FILES ||
        kind == ShardMessageKind::VISIT_TRANSIENT_FILES ||
        kind == ShardMessageKind::BLOCK_SERVICE_FILES ||
        kind == ShardMessageKind::FULL_READ_DIR
    );
}

static void packShardResponse(
    Env& env,
    ShardShared& shared,
    const AddrsInfo& srcAddr,
    UDPSender& sender,
    bool dropArtificially,
    const ShardReq& req,
    const ShardRespMsg& msg
) {
    auto respKind = msg.body.kind();
    auto reqKind = req.msg.body.kind();
    auto elapsed = ternNow() - req.receivedAt;
    shared.timings[(int)reqKind].add(elapsed);
    shared.errors[(int)reqKind].add( respKind != ShardMessageKind::ERROR ? TernError::NO_ERROR : msg.body.getError());
    if (unlikely(dropArtificially)) {
        LOG_DEBUG(env, "artificially dropping response %s", msg.id);
        return;
    }
    ALWAYS_ASSERT(req.clientAddr.port != 0);

    if (respKind != ShardMessageKind::ERROR) {
        LOG_DEBUG(env, "successfully processed request %s with kind %s in %s", msg.id, reqKind, elapsed);
        if (bigResponse(respKind)) {
            if (unlikely(env.shouldLog(LogLevel::LOG_TRACE))) {
                LOG_TRACE(env, "resp: %s", msg);
            } else {
                LOG_DEBUG(env, "resp: <omitted>");
            }
        } else {
            LOG_DEBUG(env, "resp: %s", msg);
        }
    } else {
        LOG_DEBUG(env, "request %s failed with error %s in %s", reqKind, msg.body.getError(), elapsed);
    }
    sender.prepareOutgoingMessage(env, srcAddr, req.sockIx, req.clientAddr,
        [&msg](BincodeBuf& buf) {
            msg.pack(buf);
        });
    LOG_DEBUG(env, "will send response for req id %s kind %s to %s", msg.id, reqKind, req.clientAddr);
}

template<typename T>
static void packCheckPointedShardResponse(
    Env& env,
    ShardShared& shared,
    const AddrsInfo& srcAddr,
    UDPSender& sender,
    bool dropArtificially,
    const ShardReq& req,
    const T& msg,
    const AES128Key& key
) {
    auto respKind = msg.body.resp.kind();
    auto reqKind = req.msg.body.kind();
    auto elapsed = ternNow() - req.receivedAt;
    shared.timings[(int)reqKind].add(elapsed);
    shared.errors[(int)reqKind].add( respKind != ShardMessageKind::ERROR ? TernError::NO_ERROR : msg.body.resp.getError());
    if (unlikely(dropArtificially)) {
        LOG_DEBUG(env, "artificially dropping response %s", msg.id);
        return;
    }
    ALWAYS_ASSERT(req.clientAddr.port != 0);

    if (respKind != ShardMessageKind::ERROR) {
        LOG_DEBUG(env, "successfully processed request %s with kind %s in %s", msg.id, reqKind, elapsed);
        if (bigResponse(respKind)) {
            if (unlikely(env.shouldLog(LogLevel::LOG_TRACE))) {
                LOG_TRACE(env, "resp: %s", msg);
            } else {
                LOG_DEBUG(env, "resp: <omitted>");
            }
        } else {
            LOG_DEBUG(env, "resp: %s", msg);
        }
    } else {
        LOG_DEBUG(env, "request %s failed with error %s in %s", reqKind, msg.body.resp.getError(), elapsed);
    }
    sender.prepareOutgoingMessage(env, srcAddr, req.sockIx, req.clientAddr,
        [&msg, &key](BincodeBuf& buf) {
            msg.pack(buf, key);
        });
    LOG_DEBUG(env, "will send response for req id %s kind %s to %s", msg.id, reqKind, req.clientAddr);
}

static constexpr std::array<uint32_t, 7> ShardProtocols = {
        SHARD_REQ_PROTOCOL_VERSION,
        SHARD_RESP_PROTOCOL_VERSION,
        CDC_TO_SHARD_REQ_PROTOCOL_VERSION,
        LOG_REQ_PROTOCOL_VERSION,
        LOG_RESP_PROTOCOL_VERSION,
        PROXY_SHARD_RESP_PROTOCOL_VERSION,
        PROXY_SHARD_REQ_PROTOCOL_VERSION};

using ShardChannel = MultiplexedChannel<ShardProtocols.size(), ShardProtocols>;

struct ShardServer : Loop {
private:
    // init data
    ShardShared& _shared;

    // run data
    AES128Key _expandedCDCKey;
    AES128Key _expandedShardKey;

    // log entries buffers
    std::vector<LogsDBRequest> _logsDBRequests;
    std::vector<LogsDBResponse> _logsDBResponses;
    std::vector<ProxyLogsDBRequest> _proxyLogsDBRequests;
    std::vector<ProxyLogsDBResponse> _proxyLogsDBResponses;
    std::vector<ShardReq> _writeReqs;
    std::vector<ProxyShardRespMsg> _proxyResponses;
    std::vector<ShardResp> _waitResponses;

    // read requests buffer
    std::vector<ShardReq> _readRequests;

    std::vector<std::unique_ptr<SPSC<ShardReq>>>::iterator _readQueueIt;

    std::unique_ptr<UDPReceiver<1>> _receiver;
    std::unique_ptr<ShardChannel> _channel;
public:
    ShardServer(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared) :
        Loop(logger, xmon, "server"),
        _shared(shared),
        _readQueueIt(_shared.readerRequestsQueues.begin())
    {
        auto convertProb = [this](const std::string& what, double prob, uint64_t& iprob) {
            if (prob != 0.0) {
                LOG_INFO(_env, "Will drop %s%% of %s packets", prob*100.0, what);
                iprob = prob * 10'000.0;
                ALWAYS_ASSERT(iprob > 0 && iprob < 10'000);
            }
        };
        expandKey(ShardKey, _expandedShardKey);
        expandKey(CDCKey, _expandedCDCKey);
        _receiver = std::make_unique<UDPReceiver<1>>(UDPReceiverConfig{.perSockMaxRecvMsg = MAX_RECV_MSGS, .maxMsgSize = MAX_UDP_MTU});
        _channel = std::make_unique<ShardChannel>();
    }

    virtual ~ShardServer() = default;

private:
    void _handleLogsDBResponse(UDPMessage& msg) {
        LOG_DEBUG(_env, "received LogsDBResponse from %s", msg.clientAddr);

        LogRespMsg* respMsg = nullptr;

        auto replicaId = _getReplicaId(msg.clientAddr);
        bool proxyLocation = true;

        if (replicaId != LogsDB::REPLICA_COUNT) {
            auto& resp = _logsDBResponses.emplace_back();
            resp.replicaId = replicaId;
            respMsg = &resp.msg;
            proxyLocation = false;
        } else {
            // try matching to other locations
            auto leadersPtr = _shared.getLeadersAtOtherLocations();
            for (auto& leader : *leadersPtr) {
                if (leader.locationId == 0 && leader.addrs.contains(msg.clientAddr)) {
                    auto& resp = _proxyLogsDBResponses.emplace_back();
                    respMsg = &resp.response.msg;
                    break;
                }
            }
        }

        if (respMsg == nullptr) {
            LOG_DEBUG(_env, "We can't match this address to replica or other leader. Dropping");
            return;
        }

        try {
            respMsg->unpack(msg.buf, _expandedShardKey);
        } catch (const BincodeException& err) {
            LOG_ERROR(_env, "Could not parse LogsDBResponse: %s", err.what());
            if (proxyLocation) {
                _proxyLogsDBResponses.pop_back();
            } else {
                _logsDBResponses.pop_back();
            }
            return;
        }
        LOG_DEBUG(_env, "Received response %s for requests id %s from replica id %s", respMsg->body.kind(), respMsg->id, replicaId);
    }

    void _handleLogsDBRequest(UDPMessage& msg) {
        LOG_DEBUG(_env, "received LogsDBRequest from %s", msg.clientAddr);
        LogReqMsg* reqMsg = nullptr;
        bool proxyLocation = true;

        auto replicaId = _getReplicaId(msg.clientAddr);

        if (replicaId != LogsDB::REPLICA_COUNT) {
            auto& req = _logsDBRequests.emplace_back();
            req.replicaId = replicaId;
            reqMsg = &req.msg;
            proxyLocation = false;
        } else {
            // try matching to other locations
            auto leadersPtr = _shared.getLeadersAtOtherLocations();
            for (auto& leader : *leadersPtr) {
                if (leader.addrs.contains(msg.clientAddr)) {
                    auto& req = _proxyLogsDBRequests.emplace_back();
                    req.clientAddr = msg.clientAddr;
                    req.sockIx = msg.socketIx;
                    reqMsg = &req.request.msg;
                    break;
                }
            }
        }

        if (reqMsg == nullptr) {
            LOG_DEBUG(_env, "We can't match this address to replica or other leader. Dropping");
            return;
        }

        try {
            reqMsg->unpack(msg.buf, _expandedShardKey);
        } catch (const BincodeException& err) {
            LOG_ERROR(_env, "Could not parse LogsDBRequest: %s", err.what());
            if (proxyLocation) {
                _proxyLogsDBRequests.pop_back();
            } else {
                _logsDBRequests.pop_back();
            }
            return;
        }
        LOG_DEBUG(_env, "Received request %s with requests id %s from replica id %s", reqMsg->body.kind(), reqMsg->id, replicaId);
    }

    uint8_t _getReplicaId(const IpPort& clientAddress) {
        auto replicasPtr = _shared.getReplicas();
        if (!replicasPtr) {
            return LogsDB::REPLICA_COUNT;
        }

        for (ReplicaId replicaId = 0; replicaId.u8 < replicasPtr->size(); ++replicaId.u8) {
            if (replicasPtr->at(replicaId.u8).contains(clientAddress)) {
                return replicaId.u8;
            }
        }

        return LogsDB::REPLICA_COUNT;
    }

    void _handleShardRequest(UDPMessage& msg, uint32_t protocol) {
        LOG_DEBUG(_env, "received message from %s", msg.clientAddr);
        if (unlikely(!_shared.options.isLeader())) {
            LOG_DEBUG(_env, "not leader, dropping request %s", msg.clientAddr);
            return;
        }

        ShardReqMsg req;
        try {
            switch (protocol) {
            case CDC_TO_SHARD_REQ_PROTOCOL_VERSION:
                {
                    CdcToShardReqMsg signedReq;
                    signedReq.unpack(msg.buf, _expandedCDCKey);
                    req.id = signedReq.id;
                    req.body = std::move(signedReq.body);
                }
                break;
            case SHARD_REQ_PROTOCOL_VERSION:
                req.unpack(msg.buf);
                if (isPrivilegedRequestKind((uint8_t)req.body.kind())) {
                    LOG_ERROR(_env, "Received unauthenticated request %s from %s", req.body.kind(), msg.clientAddr);
                    return;
                }
                break;
            case PROXY_SHARD_REQ_PROTOCOL_VERSION:
                {
                    ProxyShardReqMsg signedReq;
                    signedReq.unpack(msg.buf, _expandedShardKey);
                    req.id = signedReq.id;
                    req.body = std::move(signedReq.body);
                }
                break;
            default:
                ALWAYS_ASSERT(false, "Unknown protocol version");
            }
        } catch (const BincodeException& err) {
            LOG_ERROR(_env, "Could not parse: %s", err.what());
            RAISE_ALERT(_env, "could not parse request from %s, dropping it.", msg.clientAddr);
            return;
        }

        auto t0 = ternNow();

        LOG_DEBUG(_env, "received request id %s, kind %s, from %s", req.id, req.body.kind(), msg.clientAddr);

        if (bigRequest(req.body.kind())) {
                if (unlikely(_env.shouldLog(LogLevel::LOG_TRACE))) {
                    LOG_TRACE(_env, "parsed request: %s", req);
                } else {
                    LOG_DEBUG(_env, "parsed request: <omitted>");
                }
        } else {
            LOG_DEBUG(_env, "parsed request: %s", req);
        }

        auto& entry = _requestNeedsConsistency(req.body.kind(), protocol) ? _writeReqs.emplace_back() : _readRequests.emplace_back();
        entry.sockIx = msg.socketIx;
        entry.clientAddr = msg.clientAddr;
        entry.receivedAt = t0;
        entry.protocol = protocol;
        entry.msg = std::move(req);
    }

    // All write requests fall into this category. Some read requests issues by CDC also need cross regional consistency
    // which is handled only on write thread
    bool _requestNeedsConsistency(ShardMessageKind kind, uint32_t protocol) {
        if (protocol == CDC_TO_SHARD_REQ_PROTOCOL_VERSION && kind == ShardMessageKind::LOOKUP) {
            return true;
        }
        return !readOnlyShardReq(kind);
    }

    void _handleShardResponse(UDPMessage& msg, uint32_t protocol) {
        LOG_DEBUG(_env, "received message from %s", msg.clientAddr);
        if (unlikely(!_shared.options.isLeader())) {
            LOG_DEBUG(_env, "not leader, dropping response %s", msg.clientAddr);
            return;
        }
        switch (protocol) {
        case SHARD_RESP_PROTOCOL_VERSION: {
            ShardResp& resp = _waitResponses.emplace_back();
            try {
                resp.msg.unpack(msg.buf);
                ALWAYS_ASSERT(resp.msg.body.kind() == ShardMessageKind::WAIT_STATE_APPLIED);
                resp.clientAddr = msg.clientAddr;
            } catch (const BincodeException& err) {
                LOG_ERROR(_env, "Could not parse: %s", err.what());
                RAISE_ALERT(_env, "could not parse request from %s, dropping it.", msg.clientAddr);
                _waitResponses.pop_back();
                return;
            }
            LOG_DEBUG(_env, "parsed shard response from %s: %s", msg.clientAddr, resp.msg);
            break;
        }
        case PROXY_SHARD_RESP_PROTOCOL_VERSION: {
            ProxyShardRespMsg& resp = _proxyResponses.emplace_back();
            try {
                resp.unpack(msg.buf, _expandedShardKey);
            } catch (const BincodeException& err) {
                LOG_ERROR(_env, "Could not parse: %s", err.what());
                RAISE_ALERT(_env, "could not parse request from %s, dropping it.", msg.clientAddr);
                _proxyResponses.pop_back();
                return;
            }
            LOG_DEBUG(_env, "parsed shard response from %s: %s", msg.clientAddr, resp);
            break;
        }
        default:
            ALWAYS_ASSERT(false, "Unknown protocol version");
        }

    }

public:
    virtual void step() override {
        if (unlikely(!_shared.isInitiated.load(std::memory_order_acquire) || !_shared.isBlockServiceCacheInitiated.load(std::memory_order_acquire))) {
            (100_ms).sleepRetry();
            return;
        }

        _logsDBRequests.clear();
        _logsDBResponses.clear();
        _proxyLogsDBRequests.clear();
        _proxyLogsDBResponses.clear();
        _writeReqs.clear();
        _proxyResponses.clear();
        _readRequests.clear();
        _waitResponses.clear();

        if (unlikely(!_channel->receiveMessages(_env, _shared.socks, *_receiver))) {
            return;
        }

        for (auto& msg : _channel->protocolMessages(LOG_RESP_PROTOCOL_VERSION)) {
            _handleLogsDBResponse(msg);
        }

        for (auto& msg : _channel->protocolMessages(LOG_REQ_PROTOCOL_VERSION)) {
            _handleLogsDBRequest(msg);
        }

        for (auto& msg: _channel->protocolMessages(PROXY_SHARD_RESP_PROTOCOL_VERSION)) {
            _handleShardResponse(msg, PROXY_SHARD_RESP_PROTOCOL_VERSION);
        }

        for (auto& msg: _channel->protocolMessages(SHARD_RESP_PROTOCOL_VERSION)) {
            _handleShardResponse(msg, SHARD_RESP_PROTOCOL_VERSION);
        }

        std::array<size_t, 2> shardMsgCount{0, 0};
        for (auto& msg: _channel->protocolMessages(PROXY_SHARD_REQ_PROTOCOL_VERSION)) {
            _handleShardRequest(msg, PROXY_SHARD_REQ_PROTOCOL_VERSION);
            ++shardMsgCount[msg.socketIx];
        }

        for (auto& msg : _channel->protocolMessages(CDC_TO_SHARD_REQ_PROTOCOL_VERSION)) {
            _handleShardRequest(msg, CDC_TO_SHARD_REQ_PROTOCOL_VERSION);
        }

        for (auto& msg : _channel->protocolMessages(SHARD_REQ_PROTOCOL_VERSION)) {
            _handleShardRequest(msg, SHARD_REQ_PROTOCOL_VERSION);
            ++shardMsgCount[msg.socketIx];
        }

        for (size_t i = 0; i < _shared.receivedRequests.size(); ++i) {
            _shared.receivedRequests[i] = _shared.receivedRequests[i]*0.95 + ((double)shardMsgCount[i])*0.05;
        }

        // write out write requests to queues
        _shared.writerShardRespQueue.push(_waitResponses);
        auto pushed = _shared.logsDBRequestQueue.push(_logsDBRequests);
        _shared.receivedLogsDBReqs.fetch_add(_logsDBRequests.size(), std::memory_order_relaxed);
        _shared.droppedLogsDBReqs.fetch_add(_logsDBRequests.size() - pushed, std::memory_order_relaxed);

        pushed = _shared.logsDBResponseQueue.push(_logsDBResponses);
        _shared.receivedLogsDBResps.fetch_add(_logsDBResponses.size(), std::memory_order_relaxed);
        _shared.droppedLogsDBResps.fetch_add(_logsDBResponses.size() - pushed, std::memory_order_relaxed);

        pushed = _shared.proxyLogsDBRequestQueue.push(_proxyLogsDBRequests);
        _shared.receivedProxyLogsDBReqs.fetch_add(_proxyLogsDBRequests.size(), std::memory_order_relaxed);
        _shared.droppedProxyLogsDBReqs.fetch_add(_proxyLogsDBRequests.size() - pushed, std::memory_order_relaxed);

        pushed = _shared.proxyLogsDBResponseQueue.push(_proxyLogsDBResponses);
        _shared.receivedProxyLogsDBResps.fetch_add(_proxyLogsDBResponses.size(), std::memory_order_relaxed);
        _shared.droppedProxyLogsDBResps.fetch_add(_proxyLogsDBResponses.size() - pushed, std::memory_order_relaxed);

        pushed = _shared.writerProxyShardRespQueue.push(_proxyResponses);
        _shared.receivedWriterProxyResps.fetch_add(_proxyResponses.size(), std::memory_order_relaxed);
        _shared.droppedProxyWriteResps.fetch_add(_proxyResponses.size() - pushed, std::memory_order_relaxed);

        pushed = _shared.writerShardReqQueue.push(_writeReqs);
        _shared.receivedWriterReqs.fetch_add(_writeReqs.size(), std::memory_order_relaxed);
        _shared.droppedWriteReqs.fetch_add(_writeReqs.size() - pushed, std::memory_order_relaxed);

        // write out read requests to queue
        auto readIt = _readRequests.begin();
        auto readQueueIt = _readQueueIt;
        for(;;) {
            readIt = (*readQueueIt)->push(readIt, _readRequests.end());
            if (++readQueueIt == _shared.readerRequestsQueues.end()) {
                readQueueIt = _shared.readerRequestsQueues.begin();
            }
            if (readIt == _readRequests.end()) {
                break;
            }
            if (readQueueIt == _readQueueIt) {
                break;
            }
        }
        _readQueueIt = readQueueIt;

        _shared.receivedReaderReqs.fetch_add(_readRequests.size(), std::memory_order_relaxed);
        _shared.droppedReadReqs.fetch_add(std::distance(readIt, _readRequests.end()), std::memory_order_relaxed);
    }
};

struct InFlightRequestKey {
    uint64_t requestId;
    uint64_t portIp;

    InFlightRequestKey(uint64_t requestId_, IpPort clientAddr) :
        requestId(requestId_)
    {
        sockaddr_in addr;
        clientAddr.toSockAddrIn(addr);
        portIp = ((uint64_t) addr.sin_port << 32) | ((uint64_t) addr.sin_addr.s_addr);
    }

    bool operator==(const InFlightRequestKey& other) const {
        return requestId == other.requestId && portIp == other.portIp;
    }
};

template <>
struct std::hash<InFlightRequestKey> {
    std::size_t operator()(const InFlightRequestKey& key) const {
        return std::hash<uint64_t>{}(key.requestId ^ key.portIp);
    }
};

struct ProxyShardReq {
    ShardReq req;
    TernTime lastSent;
    TernTime created;
    TernTime gotLogIdx;
    TernTime finished;
};

struct CrossRegionLocationState {
    TernTime lastSent;
    uint64_t waitReqId;
};

struct CrossRegionWaitInfo {
    LogIdx idx;
    ShardReq originalRequest;
    CdcToShardRespMsg responseToSend;
    std::unordered_map<uint8_t, CrossRegionLocationState> locationStates; // location ID -> state
    TernTime createdAt;
};

struct ShardWriter : Loop {
private:
    const std::string _basePath;
    ShardShared& _shared;
    AES128Key _expandedShardKey;
    AES128Key _expandedCDCKey;

    // outgoing network
    UDPSender _sender;
    RandomGenerator _packetDropRand;
    uint64_t _outgoingPacketDropProbability; // probability * 10,000



    // work queue
    const size_t _maxWorkItemsAtOnce;

    LogsDB& _logsDB;
    bool _isLogsDBLeader;
    uint64_t _currentLogIndex;
    LogIdx _knownLastReleased; // used by leaders in proxy location to determine if they need to catch up

    Duration _nextTimeout;
    // buffers for separated items from work queue
    std::vector<LogsDBRequest> _logsDBRequests; // requests from other replicas
    std::vector<LogsDBResponse> _logsDBResponses; // responses from other replicas
    std::vector<ShardReq>       _shardRequests; // requests from clients or proxy locations
    std::vector<ShardReq>       _strongConsistencyReadRequests; // read requests that need strong cross region consistency
    std::vector<ProxyShardRespMsg> _shardResponses; // responses from shard leader in primary location
    std::vector<ProxyLogsDBRequest> _proxyLogsDBRequests; // catchup requests from logs db leader in proxy location
                                                          // or write requests from logs db leader in primary location
    std::vector<ProxyLogsDBResponse> _proxyLogsDBResponses; // responses from logs db leader in primary location
    std::vector<ShardResp> _waitResponses; // responses to wait state applied messages

    // buffers for outgoing local losgsdb requests/responses
    std::vector<LogsDBRequest *> _logsDBOutRequests;
    std::vector<LogsDBResponse> _logsDBOutResponses;

    std::vector<LogsDBLogEntry> _logsDBEntries; // used for batching writes/reads to logsdb
    std::vector<ShardLogEntry> _shardEntries; // used for batching logEntries

    std::unordered_map<uint64_t, ShardLogEntry> _inFlightEntries; // used while primary leader to track in flight entries we produced

    static constexpr Duration PROXIED_REUQEST_TIMEOUT = 100_ms;
    std::unordered_map<uint64_t, ProxyShardReq> _proxyShardRequests; // outstanding proxied shard requests
    std::unordered_map<uint64_t, std::pair<LogIdx, TernTime>> _proxyCatchupRequests; // outstanding logsdb catchup requests to primary leader
    std::unordered_map<uint64_t, std::pair<ShardRespContainer, ProxyShardReq>> _proxiedResponses; // responses from primary location that we need to send back to client

    std::vector<ProxyLogsDBRequest> _proxyReadRequests; // currently processing proxied read requests
    std::vector<LogIdx>  _proxyReadRequestsIndices;  // indices from above reuqests as LogsDB api needs them

    std::vector<ShardReq> _linkReadRequests; //TODO: make LogsDB support reads from different thread

    static constexpr size_t CATCHUP_BUFFER_SIZE = 1 << 17;
    std::array<std::pair<uint64_t,LogsDBLogEntry>,CATCHUP_BUFFER_SIZE> _catchupWindow;
    uint64_t _catchupWindowIndex; // index into the catchup window

    // The enqueued requests, but indexed by req id + ip + port. We
    // store this so that we can drop repeated requests which are
    // still queued, and which will therefore be processed in due
    // time anyway. This relies on clients having unique req ids. It's
    // kinda unsafe anyway (if clients get restarted), but it's such
    // a useful optimization for now that we live with it.
    std::unordered_set<InFlightRequestKey> _inFlightRequestKeys;

    std::map<std::pair<LogIdx, uint64_t>, ShardReq> _waitStateRequests; // log idx + request id for requests waiting for logsdb to advance

    // Cross-region wait tracking for CDC writes
    static constexpr Duration CROSS_REGION_WAIT_RETRY_INTERVAL = 200_ms;
    Duration _shardDownThreshold;
    std::unordered_map<uint64_t, CrossRegionWaitInfo> _crossRegionWaitResponses; // wait req id -> info
    std::unordered_map<uint64_t, uint64_t> _waitReqIdToResponseId;

    uint64_t _requestIdCounter;

    std::unordered_map<uint64_t, ShardReq> _logIdToShardRequest; // used to track which log entry was generated by which request
    std::shared_ptr<const std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> _replicaInfo;

    virtual void sendStop() override {
        _shared.logsDBRequestQueue.close();
        _shared.logsDBResponseQueue.close();
        _shared.proxyLogsDBRequestQueue.close();
        _shared.proxyLogsDBResponseQueue.close();
        _shared.writerShardReqQueue.close();
        _shared.writerShardRespQueue.close();
        _shared.writerProxyShardRespQueue.close();
    }

public:
    ShardWriter(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared) :
        Loop(logger, xmon, "writer"),
        _basePath(shared.options.logsDBOptions.dbDir),
        _shared(shared),
        _sender(UDPSenderConfig{.maxMsgSize = MAX_UDP_MTU}),
        _packetDropRand(ternNow().ns),
        _outgoingPacketDropProbability(0),
        _maxWorkItemsAtOnce(LogsDB::IN_FLIGHT_APPEND_WINDOW * 10),
        _logsDB(shared.logsDB),
        _isLogsDBLeader(false),
        _currentLogIndex(_shared.shardDB.lastAppliedLogEntry()),
        _knownLastReleased(_logsDB.getLastReleased()),
        _nextTimeout(0),
        _catchupWindowIndex(0),
        _shardDownThreshold(shared.options.regionStalenessThreshold),
        _requestIdCounter(RandomGenerator().generate64())
    {
        expandKey(ShardKey, _expandedShardKey);
        expandKey(CDCKey, _expandedCDCKey);
        auto convertProb = [this](const std::string& what, double prob, uint64_t& iprob) {
            if (prob != 0.0) {
                LOG_INFO(_env, "Will drop %s%% of %s packets", prob*100.0, what);
                iprob = prob * 10'000.0;
                ALWAYS_ASSERT(iprob > 0 && iprob < 10'000);
            }
        };
        _catchupWindow = {};
        convertProb("outgoing", _shared.options.serverOptions.simulateOutgoingPacketDrop, _outgoingPacketDropProbability);
        _shardEntries.reserve(LogsDB::IN_FLIGHT_APPEND_WINDOW);
    }

    virtual ~ShardWriter() = default;

    void _sendProxyAndCatchupRequests() {
        if (!_shared.options.isProxyLocation()) {
            return;
        }
        auto leadersAtOtherLocations = _shared.getLeadersAtOtherLocations();
        const AddrsInfo* primaryLeaderAddress = nullptr;
        for(auto& leader : *leadersAtOtherLocations ) {
            if (leader.locationId == 0) {
                primaryLeaderAddress = &leader.addrs;
                break;
            }
        }
        if (primaryLeaderAddress == nullptr) {
            LOG_DEBUG(_env, "don't have primary leader address yet");
            return;
        }
        // catchup requests first as progressing state is more important then sending new requests
        auto now = ternNow();
        for(auto& req : _proxyCatchupRequests) {
            if (now - req.second.second < PROXIED_REUQEST_TIMEOUT) {
                continue;
            }
            req.second.second = now;
            LogReqMsg reqMsg;
            reqMsg.id = req.first;
            reqMsg.body.setLogRead().idx = req.second.first;
            _sender.prepareOutgoingMessage(
                _env,
                _shared.sock().addr(),
                *primaryLeaderAddress,
                [&reqMsg, this](BincodeBuf& buf) {
                    reqMsg.pack(buf, _expandedShardKey);
            });
        }
        for(auto& req : _proxyShardRequests) {
            if(now - req.second.lastSent < PROXIED_REUQEST_TIMEOUT) {
                continue;
            }
            req.second.lastSent = now;
            ProxyShardReqMsg reqMsg;
            reqMsg.id = req.first;
            switch (req.second.req.msg.body.kind()) {
                case ShardMessageKind::ADD_SPAN_INITIATE:
                {
                    auto& addSpanReq = reqMsg.body.setAddSpanAtLocationInitiate();
                    addSpanReq.locationId = _shared.options.logsDBOptions.location;
                    addSpanReq.req.reference = NULL_INODE_ID;
                    addSpanReq.req.req = req.second.req.msg.body.getAddSpanInitiate();
                    break;
                }
                case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
                {
                    auto& addSpanReq = reqMsg.body.setAddSpanAtLocationInitiate();
                    addSpanReq.locationId = _shared.options.logsDBOptions.location;
                    addSpanReq.req = req.second.req.msg.body.getAddSpanInitiateWithReference();
                    break;
                }
                default:
                    reqMsg.body = req.second.req.msg.body;
            }

            _sender.prepareOutgoingMessage(
                _env,
                _shared.sock().addr(),
                *primaryLeaderAddress,
                [&reqMsg, this](BincodeBuf& buf) {
                    reqMsg.pack(buf, _expandedShardKey);
            });
        }
    }

    void _sendWaitStateAppliedResponses() {
        auto lastApplied = _logsDB.getLastReleased();
        for(auto it = _waitStateRequests.begin(); it != _waitStateRequests.end(); ) {
            if (lastApplied < it->first.first.u64) {
                break;
            }
            auto& req = it->second;

            bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
            ShardRespMsg resp;
            resp.id = req.msg.id;
            resp.body.setWaitStateApplied();

            _inFlightRequestKeys.erase(InFlightRequestKey{req.msg.id, req.clientAddr});

            packShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp);
            it = _waitStateRequests.erase(it);
        }
    }

    void _sendCrossRegionWaitResponses() {
        if (_crossRegionWaitResponses.empty()) {
            return;
        }

        auto now = ternNow();
        auto leadersAtOtherLocations = _shared.getLeadersAtOtherLocations();

        for (auto it = _crossRegionWaitResponses.begin(); it != _crossRegionWaitResponses.end(); ) {
            auto& waitInfo = it->second;
            bool shouldSendResponse = false;

            // Check if any locations need retry or are down
            for (auto locIt = waitInfo.locationStates.begin(); locIt != waitInfo.locationStates.end(); ) {
                uint8_t locationId = locIt->first;
                auto& locState = locIt->second;

                bool locationFound = false;
                bool locationDown = false;
                AddrsInfo locationAddrs;

                for (const auto& leader : *leadersAtOtherLocations) {
                    if (leader.locationId == locationId) {
                        locationFound = true;
                        locationAddrs = leader.addrs;
                        auto timeSinceLastSeen = now - leader.lastSeen;
                        if (timeSinceLastSeen >= _shardDownThreshold) {
                            locationDown = true;
                            LOG_INFO(_env, "Location %s detected as down (no response for %s), removing from wait list for LogIdx %s",
                                    locationId, timeSinceLastSeen, waitInfo.idx.u64);
                        }
                        break;
                    }
                }

                if (!locationFound || locationDown) {
                    _waitReqIdToResponseId.erase(locState.waitReqId);
                    locIt = waitInfo.locationStates.erase(locIt);
                    continue;
                }

                auto timeSinceLastSent = now - locState.lastSent;
                if (timeSinceLastSent >= CROSS_REGION_WAIT_RETRY_INTERVAL) {
                    uint64_t waitReqId = locState.waitReqId;

                    ShardReqMsg waitReq;
                    waitReq.id = waitReqId;
                    auto& waitBody = waitReq.body.setWaitStateApplied();
                    waitBody.idx.u64 = waitInfo.idx.u64;

                    _sender.prepareOutgoingMessage(
                        _env,
                        _shared.sock().addr(),
                        locationAddrs,
                        [&waitReq](BincodeBuf& buf) {
                            waitReq.pack(buf);
                        });

                    LOG_DEBUG(_env, "Retrying WaitStateAppliedReq (id=%s) for LogIdx %s to location %s",
                             waitReqId, waitInfo.idx.u64, locationId);
                    locState.lastSent = now;
                }

                ++locIt;
            }

            if (waitInfo.locationStates.empty()) {
                LOG_DEBUG(_env, "All secondary locations have applied LogIdx %s, sending response for request %s",
                         waitInfo.idx.u64, waitInfo.originalRequest.msg.id);

                bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender,
                                             dropArtificially, waitInfo.originalRequest,
                                             waitInfo.responseToSend, _expandedCDCKey);

                ALWAYS_ASSERT(_inFlightRequestKeys.erase(InFlightRequestKey{waitInfo.originalRequest.msg.id,
                                                                            waitInfo.originalRequest.clientAddr}) == 1);
                it = _crossRegionWaitResponses.erase(it);
            } else {
                ++it;
            }
        }
    }

    void _tryReplicateToOtherLocations() {
        // if we are not leader or in proxy location already we will not try to replicate
        if (!_isLogsDBLeader || _shared.options.isProxyLocation()) {
            return;
        }
        auto leadersAtOtherLocations = _shared.getLeadersAtOtherLocations();
        if (leadersAtOtherLocations->empty()) {
            return;
        }
        auto lastReleased = _logsDB.getLastReleased();
        // for each entry we fire off a no response best effort write to other locations
        // if some of the leaders are down or messages is lost they will catch up through othe means
        for (auto& logsDBEntry : _logsDBEntries) {
            LogReqMsg msg;
            msg.id = 0;
            auto& write = msg.body.setLogWrite();
            write.idx = logsDBEntry.idx;
            write.value.els = std::move(logsDBEntry.value);
            write.lastReleased  = lastReleased;

            for (const auto& receiver : *leadersAtOtherLocations) {
                bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                if (unlikely(dropArtificially)) {
                    LOG_DEBUG(_env, "artificially dropping replication write %s to shard %s at location %s", write.idx, receiver.id, receiver.locationId);
                    return;
                }
                _sender.prepareOutgoingMessage(
                    _env,
                    _shared.sock().addr(),
                    receiver.addrs,
                    [&msg, this](BincodeBuf& buf) {
                        msg.pack(buf, _expandedShardKey);
                });
                LOG_DEBUG(_env, "sending replication write %s to shard %s at location %s", write.idx, receiver.id, receiver.locationId);
            }
        }
    }

    void _applyLogEntries() {
        ALWAYS_ASSERT(_logsDBEntries.empty());
        auto lastContinuousIdx = _logsDB.getLastContinuous();
        while (_currentLogIndex < lastContinuousIdx.u64) {
            _logsDB.readEntries(_logsDBEntries);
            ALWAYS_ASSERT(!_logsDBEntries.empty());
            for (auto& logsDBEntry : _logsDBEntries) {
                ++_currentLogIndex;
                ALWAYS_ASSERT(_currentLogIndex == logsDBEntry.idx);
                ALWAYS_ASSERT(logsDBEntry.value.size() > 0);
                BincodeBuf buf((char*)&logsDBEntry.value.front(), logsDBEntry.value.size());
                ShardLogEntry shardEntry;
                shardEntry.unpack(buf);
                ALWAYS_ASSERT(_currentLogIndex == shardEntry.idx);

                if (_isLogsDBLeader) {
                    {
                        //this is sanity check confirming what we got through the log and deserialized
                        //exactly matches what we serialized and pushed to log
                        //this is only ever true in primary location
                        auto it = _inFlightEntries.find(shardEntry.idx.u64);
                        ALWAYS_ASSERT(it != _inFlightEntries.end());
                        ALWAYS_ASSERT(shardEntry == it->second);
                        ALWAYS_ASSERT(shardEntry.idx == it->second.idx);
                        _inFlightEntries.erase(it);
                    }
                    auto it = _logIdToShardRequest.find(shardEntry.idx.u64);
                    if (it == _logIdToShardRequest.end()) {
                        // if we are primary location, all writes should be triggered by requests
                        ALWAYS_ASSERT(_shared.options.isProxyLocation());
                        // we are proxy location there are writes not initiated by us and we behave like follower
                        // we are not leader, we can not do any checks and there is no response to send
                        ShardRespContainer _;
                        _shared.shardDB.applyLogEntry(logsDBEntry.idx.u64, shardEntry,  _);
                        continue;
                    }
                    auto& request = it->second;
                    if (likely(request.msg.id)) {
                        LOG_DEBUG(_env, "applying log entry for request %s kind %s from %s", request.msg.id, request.msg.body.kind(), request.clientAddr);
                    } else {
                        LOG_DEBUG(_env, "applying request-less log entry");
                    }
                    // first handle case where client does not care about response
                    if (request.msg.id == 0) {
                        ShardRespContainer resp;
                        _shared.shardDB.applyLogEntry(logsDBEntry.idx.u64, shardEntry,  resp);
                        if (unlikely(resp.kind() == ShardMessageKind::ERROR)) {
                            RAISE_ALERT(_env, "could not apply request-less log entry: %s", resp.getError());
                        }
                        _logIdToShardRequest.erase(it);
                        continue;
                    }

                    // depending on protocol we need different kind of responses
                    bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                    bool shouldWait = false;
                    switch(request.protocol){
                        case SHARD_REQ_PROTOCOL_VERSION:
                            {
                                ShardRespMsg resp;
                                _shared.shardDB.applyLogEntry(logsDBEntry.idx.u64, shardEntry,  resp.body);
                                resp.id = request.msg.id;
                                auto it = _proxiedResponses.find(logsDBEntry.idx.u64);
                                if (it != _proxiedResponses.end()) {
                                    ALWAYS_ASSERT(_shared.options.isProxyLocation());
                                    it->second.second.finished = ternNow();
                                    logSlowProxyReq(it->second.second);
                                    resp.body = std::move(it->second.first);
                                    _proxiedResponses.erase(it);
                                }
                                if (resp.body.kind() == ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE) {
                                    ShardRespContainer tmpResp;
                                    switch (request.msg.body.kind()) {
                                        case ShardMessageKind::ADD_SPAN_INITIATE:
                                        {
                                            auto& addResp = tmpResp.setAddSpanInitiate();
                                            addResp.blocks = std::move(resp.body.getAddSpanAtLocationInitiate().resp.blocks);
                                            resp.body.setAddSpanInitiate().blocks = std::move(addResp.blocks);
                                            break;
                                        }
                                        case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
                                        {
                                            auto& addResp = tmpResp.setAddSpanInitiateWithReference();
                                            addResp.resp.blocks = std::move(resp.body.getAddSpanAtLocationInitiate().resp.blocks);
                                            resp.body.setAddSpanInitiateWithReference().resp.blocks = std::move(addResp.resp.blocks);
                                            break;
                                        }
                                        case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
                                        {
                                            break;
                                        }
                                        default:
                                            ALWAYS_ASSERT(false, "Unexpected reponse kind %s for requests kind %s", resp.body.kind(), request.msg.body.kind() );
                                    }
                                }
                                packShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, request, resp);
                            }
                            break;
                        case CDC_TO_SHARD_REQ_PROTOCOL_VERSION:
                            {
                                CdcToShardRespMsg resp;
                                resp.body.checkPointIdx = shardEntry.idx;
                                resp.id = request.msg.id;
                                _shared.shardDB.applyLogEntry(logsDBEntry.idx.u64, shardEntry,  resp.body.resp);


                                // For CDC requests, wait for all secondary leaders to apply
                                // before sending response
                                auto now = ternNow();
                                auto leadersAtOtherLocations = _shared.getLeadersAtOtherLocations();
                                if ((!leadersAtOtherLocations->empty()) &&
                                    (resp.body.resp.kind() == ShardMessageKind::CREATE_DIRECTORY_INODE || resp.body.resp.kind() == ShardMessageKind::CREATE_LOCKED_CURRENT_EDGE)) {
                                    CrossRegionWaitInfo waitInfo;
                                    waitInfo.idx = shardEntry.idx;
                                    waitInfo.createdAt = now;

                                    uint64_t responseId = request.msg.id;

                                    for (const auto& leader : *leadersAtOtherLocations) {
                                         if (leader.locationId == 0) continue;
                                        auto timeSinceLastSeen = now - leader.lastSeen;
                                        if (timeSinceLastSeen >= _shardDownThreshold) {
                                            LOG_INFO(_env, "Skipping wait for secondary leader at location %s (down for %s)",
                                                    leader.locationId, timeSinceLastSeen);
                                            continue;
                                        }
                                        shouldWait = true;

                                        uint64_t waitReqId = ++_requestIdCounter;
                                        CrossRegionLocationState locState;
                                        locState.lastSent = now;
                                        locState.waitReqId = waitReqId;
                                        waitInfo.locationStates[leader.locationId] = locState;
                                        _waitReqIdToResponseId[waitReqId] = responseId;

                                        ShardReqMsg waitReq;
                                        waitReq.id = waitReqId;
                                        auto& waitBody = waitReq.body.setWaitStateApplied();
                                        waitBody.idx.u64 = shardEntry.idx.u64;

                                        _sender.prepareOutgoingMessage(
                                            _env,
                                            _shared.sock().addr(),
                                            leader.addrs,
                                            [&waitReq](BincodeBuf& buf) {
                                                waitReq.pack(buf);
                                            });

                                        LOG_DEBUG(_env, "Sent WaitStateAppliedReq (id=%s) for LogIdx %s to location %s",
                                                 waitReqId, shardEntry.idx.u64, leader.locationId);
                                    }
                                    if (shouldWait) {
                                        waitInfo.originalRequest = std::move(request);
                                        waitInfo.responseToSend = std::move(resp);
                                        _crossRegionWaitResponses[responseId] = std::move(waitInfo);
                                    }
                                }
                                if (!shouldWait) {
                                    packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, request, resp, _expandedCDCKey);
                                }
                            }
                            break;
                        case PROXY_SHARD_REQ_PROTOCOL_VERSION:
                            {
                                ProxyShardRespMsg resp;
                                resp.body.checkPointIdx = shardEntry.idx;
                                resp.id = request.msg.id;
                                _shared.shardDB.applyLogEntry(logsDBEntry.idx.u64, shardEntry,  resp.body.resp);
                                packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, request, resp, _expandedShardKey);
                            }
                            break;
                    }
                    if (!shouldWait) {
                        ALWAYS_ASSERT(_inFlightRequestKeys.erase(InFlightRequestKey{request.msg.id, request.clientAddr}) == 1);
                    }
                    _logIdToShardRequest.erase(it);
                } else {
                    // we are not leader, we can not do any checks and there is no response to send
                    ShardRespContainer _;
                    _shared.shardDB.applyLogEntry(logsDBEntry.idx.u64, shardEntry,  _);
                }
            }
            // we send new LogsDB entry to leaders in other locations
            _tryReplicateToOtherLocations();
            _logsDBEntries.clear();
        }
    }

    void _processCathupReads() {
        if (_proxyReadRequests.empty()) {
            return;
        }
        std::sort(
            _proxyReadRequests.begin(),
            _proxyReadRequests.end(),
            [](const auto& lhs, const auto& rhs) {
                return lhs.request.msg.body.getLogRead().idx < rhs.request.msg.body.getLogRead().idx;
            }
        );
        ALWAYS_ASSERT(_logsDBEntries.empty());
        ALWAYS_ASSERT(_proxyReadRequestsIndices.empty());
        _proxyReadRequestsIndices.reserve(_proxyReadRequests.size());
        for (auto& request : _proxyReadRequests) {
            _proxyReadRequestsIndices.emplace_back(request.request.msg.body.getLogRead().idx);
        }

        _logsDB.readIndexedEntries(_proxyReadRequestsIndices, _logsDBEntries);
        ALWAYS_ASSERT(_logsDBEntries.size() == _proxyReadRequestsIndices.size());
        for (size_t i = 0; i < _proxyReadRequestsIndices.size(); ++i) {
            auto& request = _proxyReadRequests[i];
            if (_logsDBEntries[i].idx == 0) {
                LOG_ERROR(_env, "We failed reading a catchup entry for index %s", request.request.msg.body.getLogRead().idx);
                continue;
            }
            LogRespMsg resp;
            resp.id = request.request.msg.id;
            auto& readResp = resp.body.setLogRead();
            readResp.result = TernError::NO_ERROR;
            readResp.value.els = _logsDBEntries[i].value;
            _sender.prepareOutgoingMessage(
                _env,
                _shared.sock().addr(),
                request.sockIx,
                request.clientAddr,
                [&resp,this](BincodeBuf& buf) {
                    resp.pack(buf, _expandedShardKey);
                });
        }

        _proxyReadRequests.clear();
        _proxyReadRequestsIndices.clear();
        _logsDBEntries.clear();
    }

    void _processLinkReadRequest(const ShardReq& req) {
        ShardRespMsg resp;
        resp.id = req.msg.id;
        auto& respBody = resp.body.setGetLinkEntries();
        auto& reqBody = req.msg.body.getGetLinkEntries();

        respBody.nextIdx = std::max(reqBody.fromIdx, _logsDB.getHeadIdx());
        std::vector<LogIdx> indexes;
        std::vector<LogsDBLogEntry> readEntries;
        indexes.reserve(128);
        readEntries.reserve(128);

        size_t processed = 0;
        const size_t maxProcess = 1024;

        auto mtu = std::min(reqBody.mtu == 0 ? (int)DEFAULT_UDP_MTU : (int)reqBody.mtu, (int)MAX_UDP_MTU);

        int budget = mtu - (int)ShardRespMsg::STATIC_SIZE - (int)GetLinkEntriesResp::STATIC_SIZE;

        const size_t maxEntriesByMtu = budget / (int)LinkEntry::STATIC_SIZE;
        while (respBody.nextIdx < _logsDB.getLastReleased() && respBody.entries.els.size() < maxEntriesByMtu && processed < maxProcess) {
            LogIdx startIdx = respBody.nextIdx;
            readEntries.clear();
            indexes.clear();
            for (int i = 0; i < 128 && processed < maxProcess; ++i) {
                if (_logsDB.getLastReleased() < startIdx) {
                    break;
                }
                indexes.push_back(startIdx);
                ++startIdx;
                ++processed;
            }
            if (indexes.empty()) {
                break;
            }
            _logsDB.readIndexedEntries(indexes, readEntries);
            for (auto& entry: readEntries) {
                if (entry.idx == 0) {
                    ++respBody.nextIdx;
                    continue;
                }
                respBody.nextIdx = entry.idx;
                ShardLogEntry logEntry;
                BincodeBuf buf((char*)entry.value.data(), entry.value.size());
                logEntry.unpack(buf);
                if (logEntry.body.kind() != ShardLogEntryKind::LINK_FILE) {
                    continue;
                }
                auto& link = logEntry.body.getLinkFile();
                LinkEntry newEntry;
                newEntry.id = link.fileId;
                newEntry.ownerId = link.ownerId;
                newEntry.time = logEntry.time;
                respBody.entries.els.emplace_back(newEntry);
                if (respBody.entries.els.size() >= maxEntriesByMtu) {
                    break;
                }
            }
        }

        packShardResponse(_env, _shared, _shared.sock().addr(), _sender, false, req, resp);
    }

    void _processCatchupWindow() {
        if (!_isLogsDBLeader || !_shared.options.isProxyLocation()) {
            return;
        }
        ALWAYS_ASSERT(_shardEntries.empty());
        ALWAYS_ASSERT(_knownLastReleased.u64 >= _currentLogIndex + _inFlightEntries.size());
        // first we move any continuous entries from cathup window and schedule them for replication
        auto maxToWrite = LogsDB::IN_FLIGHT_APPEND_WINDOW - _inFlightEntries.size();
        auto expectedLogIdx = _currentLogIndex + _inFlightEntries.size() + 1;
        while(maxToWrite > 0) {
            --maxToWrite;
            if (_catchupWindow[_catchupWindowIndex%_catchupWindow.size()].second.idx == 0) {
                // missing entry
                break;
            }
            auto& logsDBEntry = _catchupWindow[_catchupWindowIndex%_catchupWindow.size()].second;
            // LogIndex should match
            ALWAYS_ASSERT(expectedLogIdx == logsDBEntry.idx);
            // there should be no request in flight for it
            ALWAYS_ASSERT(_catchupWindow[_catchupWindowIndex%_catchupWindow.size()].first == 0);
            BincodeBuf buf((char*)&logsDBEntry.value.front(), logsDBEntry.value.size());
            auto& shardEntry = _shardEntries.emplace_back();
            shardEntry.unpack(buf);
            shardEntry.idx = expectedLogIdx;
            expectedLogIdx++;
            _catchupWindowIndex++;
            //clear catchup window element
            logsDBEntry.idx = 0;
            logsDBEntry.value.clear();
        }
        auto maxCatchupElements = std::min(_knownLastReleased.u64 - (_currentLogIndex + _inFlightEntries.size() + _shardEntries.size()), _catchupWindow.size());

        // schedule any catchup requests for missing entries
        for (uint64_t i = 0 ; i < maxCatchupElements; ++i) {
            auto &catchupEntry = _catchupWindow[(_catchupWindowIndex+i)%_catchupWindow.size()];
            if (catchupEntry.first != 0 || catchupEntry.second.idx != 0) {
                // already requested or already have it
                continue;
            }
            if (_proxyCatchupRequests.size() == LogsDB::CATCHUP_WINDOW * 4) {
                break;
            }
            catchupEntry.first = ++_requestIdCounter;
            _proxyCatchupRequests.insert({catchupEntry.first, {expectedLogIdx+i, 0}});
        }
    }

    void logSlowProxyReq(const ProxyShardReq& req) {
        auto elapsed = req.finished - req.created;
        if (elapsed > 5_sec) {
            LOG_ERROR(_env, "slow proxy requests, elapsed %s, waiting response %s, waiting log apply %s", elapsed, req.gotLogIdx - req.created, req.finished-req.gotLogIdx);
        }
    }

    void logsDBStep() {
        _logsDB.processIncomingMessages(_logsDBRequests,_logsDBResponses);
        _knownLastReleased = std::max(_knownLastReleased,_logsDB.getLastReleased());
        _nextTimeout = _logsDB.getNextTimeout();
        auto now = ternNow();

        // check for leadership state change and clean up internal state
        if (unlikely(_isLogsDBLeader != _logsDB.isLeader())) {
            if (_logsDB.isLeader()) {
                // we should not add anything to these if not leader
                ALWAYS_ASSERT(_inFlightEntries.empty());
                ALWAYS_ASSERT(_proxyCatchupRequests.empty());
                ALWAYS_ASSERT(_inFlightRequestKeys.empty());
                ALWAYS_ASSERT(_proxyShardRequests.empty());
                ALWAYS_ASSERT(_proxiedResponses.empty());

                // during leader election relase point might have moved and there could be entries we have not applied yet
                _applyLogEntries();
                _knownLastReleased = _logsDB.getLastReleased();
            } else {
                LOG_INFO(_env, "We are no longer leader in LogsDB");
                _inFlightEntries.clear();
                _proxyCatchupRequests.clear();
                _inFlightRequestKeys.clear();
                _proxyShardRequests.clear();
                _proxiedResponses.clear();
                for(auto& entry : _catchupWindow) {
                    entry.first = 0;
                    entry.second.idx = 0;
                }
                _catchupWindowIndex = 0;
            }
            _isLogsDBLeader = _logsDB.isLeader();
        }

        if (!_isLogsDBLeader) {
            // clear things we don't serve or expect as a follower
            _shardRequests.clear();
            _shardResponses.clear();
            _proxyLogsDBRequests.clear();
            _proxyLogsDBResponses.clear();
            _waitResponses.clear();
        }

        _linkReadRequests.clear();
        _strongConsistencyReadRequests.clear();

        if (!_shared.options.isProxyLocation()) {
            // we don't expect and should not process any shard  responses if in primary location
            _shardResponses.clear();
        }

        // there could be log entries just released which we should apply
        _applyLogEntries();

        // if we are leader we should alyways have latest state applied
        ALWAYS_ASSERT(!_isLogsDBLeader || _currentLogIndex == _logsDB.getLastReleased());

        ShardReq snapshotReq;
        // prepare log entries for requests or proxy them to primary leader
        ALWAYS_ASSERT(_shardEntries.empty());
        for (auto& req : _shardRequests) {
            if (req.msg.id != 0 && _inFlightRequestKeys.contains(InFlightRequestKey{req.msg.id, req.clientAddr})) {
                // we already have a request in flight with this id from this client
                continue;
            }
            if (readOnlyShardReq(req.msg.body.kind())) {
                if (req.msg.id != 0) {
                    _inFlightRequestKeys.insert(InFlightRequestKey{req.msg.id, req.clientAddr});
                }
                _strongConsistencyReadRequests.emplace_back(std::move(req));
                continue;
            }
            switch (req.msg.body.kind()) {
                case ShardMessageKind::SHARD_SNAPSHOT:
                    snapshotReq = std::move(req);
                    continue;
                case ShardMessageKind::GET_LINK_ENTRIES:
                    _linkReadRequests.emplace_back(std::move(req));
                    continue;
                case ShardMessageKind::WAIT_STATE_APPLIED:
                    {
                        auto& waitReq = req.msg.body.getWaitStateApplied();
                        auto logIdx = LogIdx{waitReq.idx.u64};
                        if (logIdx.u64 <= _logsDB.getLastReleased().u64) {
                            // already applied
                            bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                            ShardRespMsg resp;
                            resp.id = req.msg.id;
                            resp.body.setWaitStateApplied();
                            packShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp);
                        } else {
                            _inFlightRequestKeys.insert(InFlightRequestKey{req.msg.id, req.clientAddr});
                            _waitStateRequests.emplace(std::make_pair(logIdx, req.msg.id), std::move(req));
                        }
                    }
                    continue;
                default:
                    break;
            }

            if (_shared.options.isProxyLocation()) {
                if (req.msg.id != 0) {
                    _inFlightRequestKeys.insert(InFlightRequestKey{req.msg.id, req.clientAddr});
                }
                // we send them out later along with timed out ones
                _proxyShardRequests.insert({++_requestIdCounter, ProxyShardReq{std::move(req), 0, now, 0, 0}});
                continue;
            }

            if (_inFlightEntries.size() + _shardEntries.size() == LogsDB::IN_FLIGHT_APPEND_WINDOW) {
                // we have reached the limit of in flight entries anything else will be dropped anyway
                // clients will retry
                continue;
            }
            auto& entry = _shardEntries.emplace_back();

            auto err = _shared.shardDB.prepareLogEntry(req.msg.body, entry);
            if (unlikely(err != TernError::NO_ERROR)) {
                _shardEntries.pop_back(); // back out the log entry
                LOG_ERROR(_env, "error preparing log entry for request: %s from: %s err: %s", req.msg, req.clientAddr, err);
                // depending on protocol we need different kind of responses
                bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                switch(req.protocol){
                    case SHARD_REQ_PROTOCOL_VERSION:
                        {
                            ShardRespMsg resp;
                            resp.id = req.msg.id;
                            resp.body.setError() = err;
                            packShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp);
                        }
                        break;
                    case CDC_TO_SHARD_REQ_PROTOCOL_VERSION:
                        {
                            CdcToShardRespMsg resp;
                            resp.body.checkPointIdx = _logsDB.getLastReleased();
                            resp.id = req.msg.id;
                            resp.body.resp.setError() = err;
                            packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp, _expandedCDCKey);
                        }
                        break;
                    case PROXY_SHARD_REQ_PROTOCOL_VERSION:
                        {
                            ProxyShardRespMsg resp;
                            resp.body.checkPointIdx = _logsDB.getLastReleased();
                            resp.id = req.msg.id;
                            resp.body.resp.setError() = err;
                            packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp, _expandedShardKey);
                        }
                        break;
                }
            }
            entry.idx = _currentLogIndex + _inFlightEntries.size() + _shardEntries.size();
            // requests with id 0 are "one off, don't want response, will not retry" so we assume that if we receive it twice we need to execute it twice
            if (req.msg.id != 0) {
                _inFlightRequestKeys.insert(InFlightRequestKey{req.msg.id, req.clientAddr});
            }
            _logIdToShardRequest.insert({entry.idx.u64, std::move(req)});
        }

        for(auto& req : _strongConsistencyReadRequests) {
            ALWAYS_ASSERT(req.protocol == CDC_TO_SHARD_REQ_PROTOCOL_VERSION);
            CdcToShardRespMsg resp;
            resp.body.checkPointIdx = _shared.shardDB.read(req.msg.body, resp.body.resp);
            resp.id = req.msg.id;

            // For CDC requests, wait for all secondary leaders to apply
            // before sending response
            auto now = ternNow();
            auto leadersAtOtherLocations = _shared.getLeadersAtOtherLocations();
            bool shouldWait = false;
            if ((!leadersAtOtherLocations->empty()) && resp.body.resp.kind() != ShardMessageKind::ERROR) {

                CrossRegionWaitInfo waitInfo;
                waitInfo.idx = resp.body.checkPointIdx;
                waitInfo.createdAt = now;

                uint64_t responseId = resp.id;

                for (const auto& leader : *leadersAtOtherLocations) {
                    if (leader.locationId == 0) continue;
                    auto timeSinceLastSeen = now - leader.lastSeen;
                    if (timeSinceLastSeen >= _shardDownThreshold) {
                        LOG_INFO(_env, "Skipping wait for secondary leader at location %s (down for %s)",
                                leader.locationId, timeSinceLastSeen);
                        continue;
                    }
                    shouldWait = true;

                    uint64_t waitReqId = ++_requestIdCounter;
                    CrossRegionLocationState locState;
                    locState.lastSent = now;
                    locState.waitReqId = waitReqId;
                    waitInfo.locationStates[leader.locationId] = locState;
                    _waitReqIdToResponseId[waitReqId] = responseId;

                    ShardReqMsg waitReq;
                    waitReq.id = waitReqId;
                    auto& waitBody = waitReq.body.setWaitStateApplied();
                    waitBody.idx = waitInfo.idx;

                    _sender.prepareOutgoingMessage(
                        _env,
                        _shared.sock().addr(),
                        leader.addrs,
                        [&waitReq](BincodeBuf& buf) {
                            waitReq.pack(buf);
                        });

                    LOG_DEBUG(_env, "Sent WaitStateAppliedReq (id=%s) for LogIdx %s to location %s",
                                waitReqId,  waitInfo.idx, leader.locationId);
                }
                if (shouldWait) {
                    waitInfo.originalRequest = std::move(req);
                    waitInfo.responseToSend = std::move(resp);
                    _crossRegionWaitResponses[responseId] = std::move(waitInfo);
                }
            }
            if (!shouldWait) {
                bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp, _expandedCDCKey);
                ALWAYS_ASSERT(_inFlightRequestKeys.erase(InFlightRequestKey{req.msg.id, req.clientAddr}) == 1);
            }
        }

        for(auto& waitResp : _waitResponses) {
            ALWAYS_ASSERT(waitResp.msg.body.kind() == ShardMessageKind::WAIT_STATE_APPLIED);
            auto waitReqIt = _waitReqIdToResponseId.find(waitResp.msg.id);
            if (waitReqIt != _waitReqIdToResponseId.end()) {
                uint64_t responseId = waitReqIt->second;
                auto waitIt = _crossRegionWaitResponses.find(responseId);
                if (waitIt != _crossRegionWaitResponses.end()) {
                    // Match source address to location ID
                    auto leadersAtOtherLocations = _shared.getLeadersAtOtherLocations();
                    uint8_t respondingLocationId = 255; // Invalid location ID

                    for (const auto& leader : *leadersAtOtherLocations) {
                        if (leader.locationId == 0) {
                            continue;
                        }
                        if (leader.addrs.contains(waitResp.clientAddr)) {
                            respondingLocationId = leader.locationId;
                            break;
                        }
                    }
                    auto locIt = waitIt->second.locationStates.find(respondingLocationId);
                    if (locIt == waitIt->second.locationStates.end()) {
                        // it's possible leader address change and we could not match it
                        // we just ignore this response as we will retry later
                        continue;
                    }

                    auto& locState = locIt->second;
                    waitIt->second.locationStates.erase(locIt);
                    LOG_DEBUG(_env, "Received WaitStateAppliedResp for LogIdx %s from location %s, %s locations remaining",
                                waitIt->second.idx.u64, respondingLocationId,
                                waitIt->second.locationStates.size());
                    _waitReqIdToResponseId.erase(waitReqIt);
                }
            }
        }

        for(auto& resp: _shardResponses) {
            auto it = _proxyShardRequests.find(resp.id);
            if (it == _proxyShardRequests.end()) {
                // it is possible we already replied to this request and removed it from the map
                continue;
            }
            auto& req = it->second.req;
            it->second.gotLogIdx = now;

            // it is possible we already applied the log entry, forward the response
            if (resp.body.checkPointIdx <= _logsDB.getLastContinuous()) {
                if (likely(req.msg.id)) {
                    LOG_DEBUG(_env, "applying log entry for request %s kind %s from %s", req.msg.id, req.msg.body.kind(), req.clientAddr);
                } else {
                    LOG_DEBUG(_env, "applying request-less log entry");
                    // client does not care about response
                    _proxyShardRequests.erase(it);
                    continue;
                }
                it->second.finished = now;

                logSlowProxyReq(it->second);

                // depending on protocol we need different kind of responses
                bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
                ALWAYS_ASSERT(req.protocol == SHARD_REQ_PROTOCOL_VERSION);
                {
                    ShardRespMsg forwarded_resp;
                    forwarded_resp.id = req.msg.id;
                    forwarded_resp.body = std::move(resp.body.resp);
                    if (forwarded_resp.body.kind() == ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE) {
                        ShardRespContainer tmpResp;
                        switch (req.msg.body.kind()) {
                            case ShardMessageKind::ADD_SPAN_INITIATE:
                            {
                                auto& addResp = tmpResp.setAddSpanInitiate();
                                addResp.blocks = std::move(forwarded_resp.body.getAddSpanAtLocationInitiate().resp.blocks);
                                forwarded_resp.body.setAddSpanInitiate().blocks = std::move(addResp.blocks);
                                break;
                            }
                            case ShardMessageKind::ADD_SPAN_INITIATE_WITH_REFERENCE:
                            {
                                auto& addResp = tmpResp.setAddSpanInitiateWithReference();
                                addResp.resp.blocks = std::move(forwarded_resp.body.getAddSpanAtLocationInitiate().resp.blocks);
                                forwarded_resp.body.setAddSpanInitiateWithReference().resp.blocks = std::move(addResp.resp.blocks);
                                break;
                            }
                            case ShardMessageKind::ADD_SPAN_AT_LOCATION_INITIATE:
                            {
                                break;
                            }
                            default:
                                ALWAYS_ASSERT(false, "Unexpected reponse kind %s for requests kind %s", forwarded_resp.body.kind(), req.msg.body.kind() );
                        }
                    }
                    packShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, forwarded_resp);
                }
                ALWAYS_ASSERT(_inFlightRequestKeys.erase(InFlightRequestKey{req.msg.id, req.clientAddr}) == 1);
                _proxyShardRequests.erase(it);
                continue;
            }
            if (resp.body.checkPointIdx.u64 > _knownLastReleased.u64 + 1000) {
                    LOG_ERROR(_env, "new last known released on proxy response receive, lag of %s",resp.body.checkPointIdx.u64 - _knownLastReleased.u64 );

            }
            _knownLastReleased = std::max(_knownLastReleased, resp.body.checkPointIdx);
            if (resp.body.checkPointIdx.u64 - _logsDB.getLastReleased().u64 > 1000) {
                LOG_ERROR(_env, "large log lag on proxy response receive %s", resp.body.checkPointIdx.u64 - _logsDB.getLastReleased().u64);
            }

            // we have the response but we will not reply immediately as we need to wait for logsDB to apply the log entry to guarantee
            // read your own writes
            _logIdToShardRequest.insert({resp.body.checkPointIdx.u64, std::move(req)});
            _proxiedResponses.insert({resp.body.checkPointIdx.u64, std::pair<ShardRespContainer, ProxyShardReq>({std::move(resp.body.resp), std::move(it->second)})});
            _proxyShardRequests.erase(it);
        }

        for (auto &resp : _proxyLogsDBResponses) {
            auto it = _proxyCatchupRequests.find(resp.response.msg.id);
            if (it == _proxyCatchupRequests.end()) {
                continue;
            }
            if (resp.response.msg.body.kind() != LogMessageKind::LOG_READ) {
                LOG_ERROR(_env, "Unexpected LogsDB proxy response kind: %s", resp.response.msg.body.kind());
                continue;
            }
            auto logIdx = it->second.first;
            // if we had an outstanding request this log index should be something we have not already written
            // are not currently replicating within location (inFlightEntries)
            ALWAYS_ASSERT(logIdx.u64 > _currentLogIndex);
            ALWAYS_ASSERT(logIdx.u64 - _currentLogIndex > _inFlightEntries.size());
            _proxyCatchupRequests.erase(it);
            auto offsetFromLastReplication = logIdx.u64 - _currentLogIndex - _inFlightEntries.size() - 1;
            // we never request things out of catchup window
            ALWAYS_ASSERT(offsetFromLastReplication < _catchupWindow.size());
            auto catchupWindowOffset = (_catchupWindowIndex + offsetFromLastReplication) % _catchupWindow.size();
            // request should match and it should only be procesed once
            ALWAYS_ASSERT(_catchupWindow[catchupWindowOffset].first == resp.response.msg.id);
            ALWAYS_ASSERT(_catchupWindow[catchupWindowOffset].second.idx == 0);
            _catchupWindow[catchupWindowOffset].first = 0;
            _catchupWindow[catchupWindowOffset].second.idx = logIdx;
            _catchupWindow[catchupWindowOffset].second.value = std::move(resp.response.msg.body.getLogRead().value.els);
        }

        for (auto &req : _proxyLogsDBRequests) {
            switch(req.request.msg.body.kind()) {
            case LogMessageKind::LOG_WRITE:
                {
                    auto& write= req.request.msg.body.getLogWrite();
                    if (_knownLastReleased + 1000 < write.lastReleased) {
                        LOG_ERROR(_env, "received large update for _lastKnownRelaease %s", write.lastReleased.u64 - _knownLastReleased.u64);
                    }
                    _knownLastReleased = std::max(_knownLastReleased, write.lastReleased);
                    if (write.idx.u64 <= _currentLogIndex + _inFlightEntries.size()) {
                        LOG_ERROR(_env, "we have received write from leader but we are already replicating it. This should never happen");
                        break;
                    }
                    if (write.idx.u64 > _currentLogIndex + _inFlightEntries.size() + _catchupWindow.size()) {
                        // write outside of catchup window, we can't store it yet so we ignore. lastReleased was updated
                        break;
                    }
                    auto offsetFromLastReplication = write.idx.u64 - _currentLogIndex - _inFlightEntries.size() - 1;
                    auto catchupWindowOffset = (_catchupWindowIndex + offsetFromLastReplication) % _catchupWindow.size();
                    if (_catchupWindow[catchupWindowOffset].first != 0) {
                        // we have outstanding request for this entry
                        auto it = _proxyCatchupRequests.find(_catchupWindow[catchupWindowOffset].first);
                        ALWAYS_ASSERT(it != _proxyCatchupRequests.end());
                        ALWAYS_ASSERT(it->second.first == write.idx);
                        _proxyCatchupRequests.erase(it);
                        _catchupWindow[catchupWindowOffset].first = 0;
                    }
                    _catchupWindow[catchupWindowOffset].second.idx = write.idx;
                    _catchupWindow[catchupWindowOffset].second.value = std::move(write.value.els);
                    break;
                }
            case LogMessageKind::LOG_READ:
                _proxyReadRequests.emplace_back(std::move(req));
                break;
            default:
                LOG_ERROR(_env, "Unexpected LogsDB proxy request kind: %s", req.request.msg.body.kind());
                break;
            }
        }

        _processCathupReads();

        _processCatchupWindow();

        if (!_shardEntries.empty()) {
            ALWAYS_ASSERT(_isLogsDBLeader);
            ALWAYS_ASSERT(_logsDBEntries.empty());
            _logsDBEntries.reserve(_shardEntries.size());
            std::array<uint8_t, MAX_UDP_MTU> data;

            for(auto& entry : _shardEntries) {
                auto& logsDBEntry = _logsDBEntries.emplace_back();;
                BincodeBuf buf((char*)&data[0], MAX_UDP_MTU);
                entry.pack(buf);
                logsDBEntry.value.assign(buf.data, buf.cursor);
            }
            auto err = _logsDB.appendEntries(_logsDBEntries);
            ALWAYS_ASSERT(err == TernError::NO_ERROR);
            for (size_t i = 0; i < _shardEntries.size(); ++i) {
                ALWAYS_ASSERT(_logsDBEntries[i].idx == _shardEntries[i].idx);
                _inFlightEntries.emplace(_shardEntries[i].idx.u64, std::move(_shardEntries[i]));
            }
            _logsDBEntries.clear();
        }

        // Log if not active is not chaty but it's messages are higher priority as they make us progress state under high load.
        // We want to have priority when sending out
        _logsDB.getOutgoingMessages(_logsDBOutRequests, _logsDBOutResponses);

        for (auto& response : _logsDBOutResponses) {
            packLogsDBResponse(response);
        }

        for (auto request : _logsDBOutRequests) {
            packLogsDBRequest(*request);
        }

        _sendProxyAndCatchupRequests();

        _sendWaitStateAppliedResponses();

        _sendCrossRegionWaitResponses();

        _shared.shardDB.flush(true);

        for (auto& req : _linkReadRequests) {
            _processLinkReadRequest(req);
        }
        // not needed as we just flushed and apparently it does actually flush again
        // _logsDB.flush(true);

        // snapshot is processed once and last as it will initiate a flush
        if (unlikely(snapshotReq.msg.body.kind() == ShardMessageKind::SHARD_SNAPSHOT)) {
            ShardRespMsg resp;
            resp.id = snapshotReq.msg.id;
            auto err = _shared.sharedDB.snapshot(_basePath +"/snapshot-" + std::to_string(snapshotReq.msg.body.getShardSnapshot().snapshotId));

            if (err == TernError::NO_ERROR) {
                resp.body.setShardSnapshot();
            } else {
                resp.body.setError() = err;
            }
            packShardResponse(_env, _shared, _shared.sock().addr(), _sender, false, snapshotReq, resp);
        }

        _sender.sendMessages(_env, _shared.sock());
    }

    const AddrsInfo* addressFromReplicaId(ReplicaId id) {
        if (!_replicaInfo) {
            return nullptr;
        }

        auto& addr = (*_replicaInfo)[id.u8];
        if (addr[0].port == 0) {
            return nullptr;
        }
        return &addr;
    }

    void packLogsDBResponse(LogsDBResponse& response) {
        auto addrInfoPtr = addressFromReplicaId(response.replicaId);
        if (unlikely(addrInfoPtr == nullptr)) {
            LOG_DEBUG(_env, "No information for replica id %s. dropping response", response.replicaId);
            return;
        }
        auto& addrInfo = *addrInfoPtr;

        bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
        if (unlikely(dropArtificially)) {
            LOG_DEBUG(_env, "artificially dropping response %s", response.msg.id);
            return;
        }

        _sender.prepareOutgoingMessage(
            _env,
            _shared.sock().addr(),
            addrInfo,
            [&response,this](BincodeBuf& buf) {
                response.msg.pack(buf, _expandedShardKey);
            });

        LOG_DEBUG(_env, "will send response for req id %s kind %s to %s", response.msg.id, response.msg.body.kind(), addrInfo);
    }

    void packLogsDBRequest(LogsDBRequest& request) {
        auto addrInfoPtr = addressFromReplicaId(request.replicaId);
        if (unlikely(addrInfoPtr == nullptr)) {
            LOG_DEBUG(_env, "No information for replica id %s. dropping request", request.replicaId);
            return;
        }
        auto& addrInfo = *addrInfoPtr;

        bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
        if (unlikely(dropArtificially)) {
            LOG_DEBUG(_env, "artificially dropping request %s", request.msg.id);
            return;
        }

        _sender.prepareOutgoingMessage(
            _env,
            _shared.sock().addr(),
            addrInfo,
            [&request,this](BincodeBuf& buf) {
                request.msg.pack(buf, _expandedShardKey);
            });

        LOG_DEBUG(_env, "will send request for req id %s kind %s to %s", request.msg.id, request.msg.body.kind(), addrInfo);
    }

    virtual void step() override {
        _logsDBRequests.clear();
        _logsDBResponses.clear();
        _logsDBOutRequests.clear();
        _logsDBOutResponses.clear();
        _shardRequests.clear();
        _shardResponses.clear();
        _waitResponses.clear();
        _shardEntries.clear();
        _proxyLogsDBRequests.clear();
        _proxyLogsDBResponses.clear();
        _proxyReadRequests.clear();
        _proxyReadRequestsIndices.clear();

        _replicaInfo = _shared.getReplicas();
        auto hasWork = _shared.writeQueuesWaiter.wait(_nextTimeout);
        auto remainingPullBudget = _maxWorkItemsAtOnce;
        if (unlikely(!hasWork && _shared.logsDBRequestQueue.isClosed())) {
            // queue is closed, stop
            stop();
            return;
        }

        auto start = ternNow();

        // the order here determins priority of processing when there is more work than we can handle at once
        // we pull wait state responses without budget limit as they only do in memory modifications
        _shared.writerShardRespQueue.pull(_waitResponses, WRITER_QUEUE_SIZE);

        // we prioritize LogsDB requests and responses as they make us progress state
        remainingPullBudget -= _shared.logsDBResponseQueue.pull(_logsDBResponses, remainingPullBudget);
        remainingPullBudget -= _shared.logsDBRequestQueue.pull(_logsDBRequests, remainingPullBudget);

        // then ProxyLogsDB requests/responses as they are needed to make progress on secondary locations
        remainingPullBudget -= _shared.proxyLogsDBResponseQueue.pull(_proxyLogsDBResponses, remainingPullBudget);
        remainingPullBudget -= _shared.proxyLogsDBRequestQueue.pull(_proxyLogsDBRequests, remainingPullBudget);

        // then shard reponses as these are responses for requests we sent to primary location
        remainingPullBudget -= _shared.writerProxyShardRespQueue.pull(_shardResponses, remainingPullBudget);

        // last are new requests from local clients and then proxy locations
        remainingPullBudget -= _shared.writerShardReqQueue.pull(_shardRequests, remainingPullBudget);

        logsDBStep();
        auto loopTime = ternNow() - start;
    }
};

struct ShardReader : Loop {
private:

    ShardShared& _shared;
    SPSC<ShardReq>& _queue;
    AES128Key _expandedShardKey;
    AES128Key _expandedCDCKey;
    ShardRespMsg _respContainer;
    CdcToShardRespMsg _checkPointedrespContainer;
    std::vector<ShardReq> _requests;

    UDPSender _sender;
    RandomGenerator _packetDropRand;
    uint64_t _outgoingPacketDropProbability; // probability * 10,000

    virtual void sendStop() override {
        _queue.close();
    }

public:
    ShardReader(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared, SPSC<ShardReq>& queue) :
        Loop(logger, xmon, "reader"),
        _shared(shared),
        _queue(queue),
        _sender(UDPSenderConfig{.maxMsgSize = MAX_UDP_MTU}),
        _packetDropRand(ternNow().ns),
        _outgoingPacketDropProbability(0)
    {
        expandKey(ShardKey, _expandedShardKey);
        expandKey(CDCKey, _expandedCDCKey);
        auto convertProb = [this](const std::string& what, double prob, uint64_t& iprob) {
            if (prob != 0.0) {
                LOG_INFO(_env, "Will drop %s%% of %s packets", prob*100.0, what);
                iprob = prob * 10'000.0;
                ALWAYS_ASSERT(iprob > 0 && iprob < 10'000);
            }
        };
        convertProb("outgoing", _shared.options.serverOptions.simulateOutgoingPacketDrop, _outgoingPacketDropProbability);
        _requests.reserve(MAX_RECV_MSGS * 2);
    }

    virtual ~ShardReader() = default;

    virtual void step() override {
        _requests.clear();
        uint32_t pulled = _queue.pull(_requests, MAX_RECV_MSGS * 2);
        auto start = ternNow();
        if (unlikely(_queue.isClosed())) {
            // queue is closed, stop
            stop();
            return;
        }

        LOG_DEBUG(_env, "pulled %s requests from read queue", pulled);
        for(auto& req : _requests) {
            ALWAYS_ASSERT(readOnlyShardReq(req.msg.body.kind()));
            bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
            switch(req.protocol){
                case SHARD_REQ_PROTOCOL_VERSION:
                {
                    ShardRespMsg resp;
                    resp.id = req.msg.id;
                    _shared.shardDB.read(req.msg.body, resp.body);
                    packShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp);
                    break;
                }
                case CDC_TO_SHARD_REQ_PROTOCOL_VERSION:
                {
                    CdcToShardRespMsg resp;
                    resp.id = req.msg.id;
                    resp.body.checkPointIdx = _shared.shardDB.read(req.msg.body, resp.body.resp);
                    packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, req, resp, _expandedCDCKey);
                    break;
                }
                default:
                LOG_ERROR(_env, "Unknown protocol version %s", req.protocol);
            }
        }

        _sender.sendMessages(_env, _shared.sock());
    }
};

struct ShardRegisterer : PeriodicLoop {
private:
    ShardShared& _shared;
    const ShardReplicaId _shrid;
    const uint8_t _location;
    const bool _noReplication;
    XmonNCAlert _alert;
public:
    ShardRegisterer(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared) :
        PeriodicLoop(logger, xmon, "registerer", {1_sec, 1, 2_mins, 1}),
        _shared(shared),
        _shrid(_shared.options.shrid()),
        _location(_shared.options.logsDBOptions.location),
        _noReplication(_shared.options.logsDBOptions.noReplication)
    {}

    virtual ~ShardRegisterer() = default;

    void init() {
        _env.updateAlert(_alert, "Waiting to register ourselves for the first time");
    }

    virtual bool periodicStep() {
        // Step 1: Register first so we're discoverable
        {
            const bool advertiseLeader = _shared.options.isLeader() && (_noReplication || _shared.isInitiated.load(std::memory_order_acquire));
            LOG_INFO(_env, "Registering ourselves (shard %s, location %s, %s, leader=%s) with registry", _shrid, (int)_location, _shared.sock().addr(), advertiseLeader);
            const auto [err, errStr] = _shared.registryClient.registerShard(_shrid, _location, advertiseLeader, _shared.sock().addr());
            if (err == EINTR) { return false; }
            if (err) {
                _env.updateAlert(_alert, "Couldn't register ourselves with registry: %s", errStr);
                return false;
            }
        }

        // Step 2: Fetch replicas, using wait filter during initial startup
        {
            std::vector<FullShardInfo> allReplicas;
            bool isInitiated = _shared.isInitiated.load(std::memory_order_acquire);
            bool needsWait = _shared.options.logsDBOptions.initialStart && !isInitiated && !_noReplication;
            uint8_t minKnown = 0;
            Duration timeout = 10_sec;
            if (needsWait) {
                minKnown = LogsDB::REPLICA_COUNT;
                timeout = 30_sec;
            }
            LOG_INFO(_env, "Fetching replicas for shardId %s from registry (minKnown=%s)", _shrid.shardId(), (int)minKnown);
            const auto [err, errStr] = _shared.registryClient.fetchShardReplicas(_shrid.shardId(), allReplicas, minKnown, _location, timeout);
            if (err == EINTR) { return false; }
            if (err) {
                _env.updateAlert(_alert, "Failed getting shard replicas from registry: %s", errStr);
                return false;
            }

            std::vector<FullShardInfo> leadersAtOtherLocations;
            std::array<AddrsInfo, LogsDB::REPLICA_COUNT> localReplicas;
            for (auto& replica : allReplicas) {
                if (replica.id.shardId() != _shrid.shardId()) {
                    continue;
                }
                if (replica.locationId == _location) {
                    localReplicas[replica.id.replicaId().u8] = replica.addrs;
                    continue;
                }
                if (replica.isLeader && replica.addrs.addrs[0].port != 0) {
                    leadersAtOtherLocations.emplace_back(std::move(replica));
                }
            }
            if (_shared.sock().addr() != localReplicas[_shrid.replicaId().u8]) {
                _env.updateAlert(_alert, "AddrsInfo in registry: %s , not matching local AddrsInfo: %s", localReplicas[_shrid.replicaId().u8], _shared.sock().addr());
                return false;
            }

            if (!isInitiated) {
                size_t emptyReplicas{0};
                for (auto& replica : localReplicas) {
                    if (replica.addrs[0].port == 0) {
                        ++emptyReplicas;
                    }
                }
                if (emptyReplicas > 0 && !_noReplication) {
                    _env.updateAlert(_alert, "Didn't get enough replicas with known addresses from registry");
                    return false;
                }
            }

            if (unlikely(!_shared.getReplicas() || *_shared.getReplicas() != localReplicas)) {
                LOG_DEBUG(_env, "Updating replicas to %s %s %s %s %s", localReplicas[0], localReplicas[1], localReplicas[2], localReplicas[3], localReplicas[4]);
                _shared.setReplicas(std::move(localReplicas));
            }

            LOG_DEBUG(_env, "Updating leaders at other locations to %s", leadersAtOtherLocations);
            _shared.setLeadersAtOtherLocations(std::move(leadersAtOtherLocations));
        }

        _env.clearAlert(_alert);
        return _shared.isInitiated.exchange(true, std::memory_order_release);
    }
};

struct ShardBlockServiceUpdater : PeriodicLoop {
private:
    ShardShared& _shared;
    ShardReplicaId _shrid;
    XmonNCAlert _alert;
    std::vector<FullBlockServiceInfo> _blockServices;
    bool _updatedOnce;
public:
    ShardBlockServiceUpdater(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared):
        PeriodicLoop(logger, xmon, "bs_updater", {1_sec, shared.options.isLeader() ? 30_sec : 2_mins}),
        _shared(shared),
        _shrid(_shared.options.shrid()),
        _updatedOnce(false)
    {
        _env.updateAlert(_alert, "Waiting to fetch block services for the first time");
    }

    virtual bool periodicStep() override {
        if (!_blockServices.empty()) {
            // We delayed applying cache update most likely we were leader. We should apply it now
            _shared.blockServicesCache.updateCache(_blockServices);
            _blockServices.clear();
        }

        LOG_INFO(_env, "about to fetch block services from registry");
        const auto [err, errStr] = _shared.registryClient.fetchBlockServices(_blockServices);
        if (err == EINTR) { return false; }
        if (err) {
            _env.updateAlert(_alert, "could not reach registry: %s", errStr);
            return false;
        }
        if (_blockServices.empty()) {
            _env.updateAlert(_alert, "got no block services");
            return false;
        }
        // We immediately update cache if we are leader and delay until next iteration on leader unless this is first update which we apply immediately
        if (!_shared.options.isLeader() || !_updatedOnce) {
            _updatedOnce = true;
            _shared.blockServicesCache.updateCache(_blockServices);
            _blockServices.clear();
            _shared.isBlockServiceCacheInitiated.store(true, std::memory_order_release);
            LOG_DEBUG(_env, "updated block services");
        }
        _env.clearAlert(_alert);

        return true;
    }
};

static void logsDBstatsToMetrics(struct MetricsBuilder& metricsBuilder, const LogsDBStats& stats, ShardReplicaId shrid, uint8_t location, TernTime now) {
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldU64( "idle_time", stats.idleTime.load(std::memory_order_relaxed).ns);
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldU64( "processing_time", stats.processingTime.load(std::memory_order_relaxed).ns);
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldU64( "leader_last_active", stats.leaderLastActive.load(std::memory_order_relaxed).ns);
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "append_window", stats.appendWindow.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "entries_released", stats.entriesReleased.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "follower_lag", stats.followerLag.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "reader_lag", stats.readerLag.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "catchup_window", stats.catchupWindow.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "entries_read", stats.entriesRead.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "requests_received", stats.requestsReceived.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "responses_received", stats.requestsReceived.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "requests_sent", stats.requestsSent.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "responses_sent", stats.responsesSent.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldFloat( "requests_timedout", stats.requestsTimedOut.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
    {
        metricsBuilder.measurement("eggsfs_shard_logsdb");
        metricsBuilder.tag("shard", shrid);
        metricsBuilder.tag("location", int(location));
        metricsBuilder.tag("leader", stats.isLeader.load(std::memory_order_relaxed));
        metricsBuilder.fieldU64( "current_epoch", stats.currentEpoch.load(std::memory_order_relaxed));
        metricsBuilder.timestamp(now);
    }
}

struct Metric {
    std::string name;
    std::atomic<uint64_t>* value;
};

struct ShardMetricsInserter : PeriodicLoop {
private:
    InfluxDB _influxDB;
    ShardShared& _shared;
    ShardReplicaId _shrid;
    uint8_t _location;
    XmonNCAlert _sendMetricsAlert;
    MetricsBuilder _metricsBuilder;
    std::unordered_map<std::string, uint64_t> _rocksDBStats;
    std::vector<Metric> _stats;
public:
    ShardMetricsInserter(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const InfluxDB& influxDB, ShardShared& shared):
        PeriodicLoop(logger, xmon, "metrics", {1_sec, 1.0, 1_mins, 0.1}),
        _influxDB(influxDB),
        _shared(shared),
        _shrid(_shared.options.shrid()),
        _location(_shared.options.logsDBOptions.location),
        _sendMetricsAlert(XmonAppType::DAYTIME, 5_mins)
    {
        _stats = {
            // received counts
            {"eggsfs_shard_logsdb_requests_received", &_shared.receivedLogsDBReqs},
            {"eggsfs_shard_logsdb_responses_received", &_shared.receivedLogsDBResps},
            {"eggsfs_shard_proxy_logsdb_requests_received", &_shared.receivedProxyLogsDBReqs},
            {"eggsfs_shard_proxy_logsdb_responses_received",&_shared.receivedProxyLogsDBResps},
            {"eggsfs_shard_write_requests_received", &_shared.receivedWriterReqs},
            {"eggsfs_shard_write_proxy_responses_received", &_shared.receivedWriterProxyResps},
            {"eggsfs_shard_read_requests_received", &_shared.receivedReaderReqs},
            // dropped counts
            {"eggsfs_shard_logsdb_requests_dropped", &_shared.droppedLogsDBReqs},
            {"eggsfs_shard_logsdb_responses_dropped", &_shared.droppedLogsDBResps},
            {"eggsfs_shard_proxy_logsdb_requests_dropped", &_shared.droppedProxyLogsDBReqs},
            {"eggsfs_shard_proxy_logsdb_responses_dropped",&_shared.droppedProxyLogsDBResps},
            {"eggsfs_shard_write_requests_dropped", &_shared.droppedWriteReqs},
            {"eggsfs_shard_write_proxy_responses_droppedd", &_shared.droppedProxyWriteResps},
            {"eggsfs_shard_read_requests_dropped", &_shared.droppedReadReqs},
        };
    }

    virtual ~ShardMetricsInserter() = default;

    virtual bool periodicStep() {
        _shared.sharedDB.dumpRocksDBStatistics();
        auto newMeassurement = [this](const std::string& name) {
            _metricsBuilder.measurement(name);
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.tag("leader", _shared.logsDB.isLeader());
        };
        auto now = ternNow();
        for (ShardMessageKind kind : allShardMessageKind) {
            const ErrorCount& errs = _shared.errors[(int)kind];
            for (int i = 0; i < errs.count.size(); i++) {
                uint64_t count = errs.count[i].load();
                if (count == 0) { continue; }
                newMeassurement("eggsfs_shard_requests");
                _metricsBuilder.tag("kind", kind);
                _metricsBuilder.tag("write", !readOnlyShardReq(kind));
                if (i == 0) {
                    _metricsBuilder.tag("error", "NO_ERROR");
                } else {
                    _metricsBuilder.tag("error", (TernError)i);
                }
                _metricsBuilder.fieldU64("count", count);
                _metricsBuilder.timestamp(now);
            }
        }

        for (int i = 0; i < _shared.receivedRequests.size(); i++) {
            _metricsBuilder.measurement("eggsfs_shard_received_requests");
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.tag("socket", i);
            _metricsBuilder.fieldFloat("count", _shared.receivedRequests[i]);
            _metricsBuilder.timestamp(now);
        }

        for (const auto& stat : _stats) {
            newMeassurement(stat.name);
            _metricsBuilder.fieldU64("count", *stat.value);
            _metricsBuilder.timestamp(now);
        }

        {
            _rocksDBStats.clear();
            _shared.sharedDB.rocksDBMetrics(_rocksDBStats);
            for (const auto& [name, value]: _rocksDBStats) {
                newMeassurement("eggsfs_shard_rocksdb");
                _metricsBuilder.fieldU64(name, value);
                _metricsBuilder.timestamp(now);
            }
        }

        {
            auto stats = _shared.blockServicesCache.getPickerStats();

            // Per-location/storage stats
            for (const auto& ls : stats.locStorage) {
                if (ls.totalPicks == 0) continue;

                uint8_t locationId = ls.key >> 8;
                uint8_t storageClass = ls.key & 0xFF;

                _metricsBuilder.measurement("eggsfs_picker_location_storage");
                _metricsBuilder.tag("shard", _shrid);
                _metricsBuilder.tag("location_id", (int)locationId);
                _metricsBuilder.tag("storage_class", (int)storageClass);
                _metricsBuilder.fieldU64("total_picks", ls.totalPicks);
                _metricsBuilder.fieldU64("writable_failure_domains", ls.writableFailureDomains);
                _metricsBuilder.fieldU64("writable_block_services", ls.writableBlockServices);
                _metricsBuilder.fieldU64("max_fd_weight", ls.maxWeight);
                _metricsBuilder.fieldU64("min_fd_weight", ls.minWeight);
                _metricsBuilder.timestamp(now);
            }

            if (stats.maxServicePicks > 0) {
                _metricsBuilder.measurement("eggsfs_picker_service_picks");
                _metricsBuilder.tag("shard", _shrid);
                _metricsBuilder.fieldU64("min_picks", stats.minServicePicks);
                _metricsBuilder.fieldU64("max_picks", stats.maxServicePicks);
                _metricsBuilder.timestamp(now);
            }

            if (stats.maxFdPicks > 0) {
                _metricsBuilder.measurement("eggsfs_picker_fd_picks");
                _metricsBuilder.tag("shard", _shrid);
                _metricsBuilder.fieldU64("min_picks", stats.minFdPicks);
                _metricsBuilder.fieldU64("max_picks", stats.maxFdPicks);
                _metricsBuilder.timestamp(now);
            }
        }

        logsDBstatsToMetrics(_metricsBuilder, _shared.logsDB.getStats(), _shrid, _location, now);
        std::string err = sendMetrics(_influxDB, 10_sec, _metricsBuilder.payload());
        _metricsBuilder.reset();
        if (err.empty()) {
            LOG_INFO(_env, "Sent metrics to influxdb");
            _env.clearAlert(_sendMetricsAlert);
            return true;
        } else {
            _env.updateAlert(_sendMetricsAlert, "Could not insert metrics: %s", err);
            return false;
        }
    }
};

void runShard(ShardOptions& options) {
    int logOutFd = STDOUT_FILENO;
    if (!options.logOptions.logFile.empty()) {
        logOutFd = open(options.logOptions.logFile.c_str(), O_WRONLY|O_CREAT|O_APPEND, 0644);
        if (logOutFd < 0) {
            throw SYSCALL_EXCEPTION("open");
        }
    }
    Logger logger(options.logOptions.logLevel, logOutFd, options.logOptions.syslog, true);

    std::shared_ptr<XmonAgent> xmon;
    if (!options.xmonOptions.addr.empty()) {
        xmon = std::make_shared<XmonAgent>();
    }

    Env env(logger, xmon, "startup");

    {
        LOG_INFO(env, "Running shard %s at location %s, with options:", options.shrid(), (int)options.logsDBOptions.location);
        LOG_INFO(env, "  level = %s", options.logOptions.logLevel);
        LOG_INFO(env, "  logFile = '%s'", options.logOptions.logFile);
        LOG_INFO(env, "  registryHost = '%s'", options.registryClientOptions.host);
        LOG_INFO(env, "  registryPort = %s", options.registryClientOptions.port);
        LOG_INFO(env, "  ownAddres = %s", options.serverOptions.addrs);
        LOG_INFO(env, "  simulateOutgoingPacketDrop = %s", options.serverOptions.simulateOutgoingPacketDrop);
        LOG_INFO(env, "  syslog = %s", (int)options.logOptions.syslog);
        LOG_INFO(env, "Using LogsDB with options:");
        LOG_INFO(env, "    noReplication = '%s'", (int)options.logsDBOptions.noReplication);
        LOG_INFO(env, "    avoidBeingLeader = '%s'", (int)options.logsDBOptions.avoidBeingLeader);
    }

    // Immediately start xmon: we want the database initializing update to
    // be there.
    LoopThreads threads;

    if (xmon) {
        {
            std::ostringstream ss;
            ss << "eggsshard_" << std::setfill('0') << std::setw(3) << options.shrid().shardId() << "_" << options.shrid().replicaId();
            options.xmonOptions.appInstance =  ss.str();
        }
        options.xmonOptions.appType = XmonAppType::CRITICAL;
        threads.emplace_back(LoopThread::Spawn(std::make_unique<Xmon>(logger, xmon, options.xmonOptions)));
    }

    // then everything else

    XmonNCAlert dbInitAlert;
    env.updateAlert(dbInitAlert, "initializing database");

    SharedRocksDB sharedDB(logger, xmon, options.logsDBOptions.dbDir + "/db", options.logsDBOptions.dbDir  + "/db-statistics.txt");
    sharedDB.registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
    sharedDB.registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    sharedDB.registerCFDescriptors(BlockServicesCacheDB::getColumnFamilyDescriptors());
    rocksdb::Options rocksDBOptions;
    rocksDBOptions.create_if_missing = true;
    rocksDBOptions.create_missing_column_families = true;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    // 1000*256 = 256k open files at once, given that we currently run on a
    // single machine this is appropriate.
    rocksDBOptions.max_open_files = 1000;
    // We batch writes and flush manually.
    rocksDBOptions.manual_wal_flush = true;
    sharedDB.open(rocksDBOptions);

    BlockServicesCacheDB blockServicesCache(logger, xmon, sharedDB, options.blockServiceWritableDelay);

    ShardDB shardDB(logger, xmon, options.shardId, options.logsDBOptions.location, options.transientDeadlineInterval, sharedDB, blockServicesCache);
    LogsDB logsDB(logger, xmon, sharedDB, options.logsDBOptions.replicaId, shardDB.lastAppliedLogEntry(), options.logsDBOptions.noReplication, !options.logsDBOptions.leaderElection, options.logsDBOptions.avoidBeingLeader);
    env.clearAlert(dbInitAlert);

    ShardShared shared(options, sharedDB, blockServicesCache, shardDB, logsDB, UDPSocketPair(env, options.serverOptions.addrs), logger, xmon);

    threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardServer>(logger, xmon, shared)));
    for (uint16_t i = 0; i < options.numReaders; ++i) {
        threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardReader>(logger, xmon, shared, *shared.readerRequestsQueues[i])));
    }
    threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardWriter>(logger, xmon, shared)));
    threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardRegisterer>(logger, xmon, shared)));
    threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardBlockServiceUpdater>(logger, xmon, shared)));
    if (!options.metricsOptions.origin.empty()) {
        threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardMetricsInserter>(logger, xmon, options.metricsOptions, shared)));
    }

    // from this point on termination on SIGINT/SIGTERM will be graceful
    LoopThread::waitUntilStopped(threads);
    threads.clear();

    shardDB.close();
    logsDB.close();
    sharedDB.close();

    LOG_INFO(env, "shard terminating gracefully, bye.");
}
