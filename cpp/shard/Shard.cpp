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

enum class WriterQueueEntryKind :uint8_t {
    LOGSDB_REQUEST = 1,
    LOGSDB_RESPONSE = 2,
    SHARD_REQUEST = 3,
    SHARD_RESPONSE = 4,
    PROXY_LOGSDB_REQUEST = 5,
    PROXY_LOGSDB_RESPONSE = 6,
};

std::ostream& operator<<(std::ostream& out, WriterQueueEntryKind kind) {
    switch (kind) {
    case WriterQueueEntryKind::LOGSDB_REQUEST:
        out << "LOGSDB_REQUEST";
        break;
    case WriterQueueEntryKind::LOGSDB_RESPONSE:
        out << "LOGSDB_RESPONSE";
        break;
    case WriterQueueEntryKind::SHARD_REQUEST:
        out << "SHARD_REQUEST";
        break;
    case WriterQueueEntryKind::SHARD_RESPONSE:
        out << "SHARD_RESPONSE";
        break;
    case WriterQueueEntryKind::PROXY_LOGSDB_REQUEST:
        out << "PROXY_LOGSDB_REQUEST";
        break;
    case WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE:
        out << "PROXY_LOGSDB_RESPONSE";
        break;
    default:
        out << "Unknown WriterQueueEntryKind(" << (uint8_t)kind << ")";
      break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, const std::vector<FullShardInfo>& shards) {
    out << "[";
    for (const auto& s : shards) {
        out << s << ", ";
    }
    return out << "]";

}

class ShardWriterRequest {
public:
    ShardWriterRequest() { clear(); }

    ShardWriterRequest(ShardWriterRequest&& other) {
        *this = std::move(other);
    }

    ShardWriterRequest& operator=(ShardWriterRequest&& other) {
        _kind = other._kind;
        _data = std::move(other._data);
        other.clear();
        return *this;
    }

    void clear() { _kind = (WriterQueueEntryKind)0; }

    WriterQueueEntryKind kind() const { return _kind; }

    LogsDBRequest& setLogsDBRequest() {
        _kind = WriterQueueEntryKind::LOGSDB_REQUEST;
        auto& x = _data.emplace<0>();
        return x;
    }

    const LogsDBRequest& getLogsDBRequest() const {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::LOGSDB_REQUEST, "%s != %s", _kind, WriterQueueEntryKind::LOGSDB_REQUEST);
        return std::get<0>(_data);
    }

    LogsDBRequest&& moveLogsDBRequest() {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::LOGSDB_REQUEST, "%s != %s", _kind, WriterQueueEntryKind::LOGSDB_REQUEST);
        clear();
        return std::move(std::get<0>(_data));
    }

    LogsDBResponse& setLogsDBResponse() {
        _kind = WriterQueueEntryKind::LOGSDB_RESPONSE;
        auto& x = _data.emplace<1>();
        return x;
    }

    const LogsDBResponse& getLogsDBResponse() const {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::LOGSDB_RESPONSE, "%s != %s", _kind, WriterQueueEntryKind::LOGSDB_RESPONSE);
        return std::get<1>(_data);
    }

    LogsDBResponse&& moveLogsDBResponse() {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::LOGSDB_RESPONSE, "%s != %s", _kind, WriterQueueEntryKind::LOGSDB_RESPONSE);
        clear();
        return std::move(std::get<1>(_data));
    }

    ShardReq& setShardReq() {
        _kind = WriterQueueEntryKind::SHARD_REQUEST;
        auto& x = _data.emplace<2>();
        return x;
    }

    const ShardReq& getShardReq() const {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::SHARD_REQUEST, "%s != %s", _kind, WriterQueueEntryKind::SHARD_REQUEST);
        return std::get<2>(_data);
    }

    ShardReq&& moveShardReq() {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::SHARD_REQUEST, "%s != %s", _kind, WriterQueueEntryKind::SHARD_REQUEST);
        clear();
        return std::move(std::get<2>(_data));
    }

    ProxyShardRespMsg& setShardResp() {
        _kind = WriterQueueEntryKind::SHARD_RESPONSE;
        auto& x = _data.emplace<3>();
        return x;
    }

    const ProxyShardRespMsg& getShardResp() const {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::SHARD_RESPONSE, "%s != %s", _kind, WriterQueueEntryKind::SHARD_RESPONSE);
        return std::get<3>(_data);
    }

    ProxyShardRespMsg&& moveShardResp() {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::SHARD_RESPONSE, "%s != %s", _kind, WriterQueueEntryKind::SHARD_RESPONSE);
        clear();
        return std::move(std::get<3>(_data));
    }

    ProxyLogsDBRequest& setProxyLogsDBRequest() {
        _kind = WriterQueueEntryKind::PROXY_LOGSDB_REQUEST;
        auto& x = _data.emplace<4>();
        return x;
    }

    const ProxyLogsDBRequest& getProxyLogsDBRequest() const {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::PROXY_LOGSDB_REQUEST, "%s != %s", _kind, WriterQueueEntryKind::PROXY_LOGSDB_REQUEST);
        return std::get<4>(_data);
    }

    ProxyLogsDBRequest&& moveProxyLogsDBRequest() {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::PROXY_LOGSDB_REQUEST, "%s != %s", _kind, WriterQueueEntryKind::PROXY_LOGSDB_REQUEST);
        clear();
        return std::move(std::get<4>(_data));
    }

    ProxyLogsDBResponse& setProxyLogsDBResponse() {
        _kind = WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE;
        auto& x = _data.emplace<5>();
        return x;
    }

    const ProxyLogsDBResponse& getProxyLogsDBResponse() const {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE, "%s != %s", _kind, WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE);
        return std::get<5>(_data);
    }

    ProxyLogsDBResponse&& moveProxyLogsDBResponse() {
        ALWAYS_ASSERT(_kind == WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE, "%s != %s", _kind, WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE);
        clear();
        return std::move(std::get<5>(_data));
    }

private:
    WriterQueueEntryKind _kind;
    std::variant<LogsDBRequest, LogsDBResponse, ShardReq, ProxyShardRespMsg, ProxyLogsDBRequest, ProxyLogsDBResponse> _data;
};

struct ShardShared {
    const ShardOptions& options;

    // network
    std::array<UDPSocketPair, 1> socks; // in an array to play with UDPReceiver<>

    // server to -> reader/writer communication
    SPSC<ShardWriterRequest> writerRequestsQueue;
    SPSC<ShardReq> readerRequestsQueue;

    // databases and caches
    SharedRocksDB& sharedDB;
    LogsDB& logsDB;
    ShardDB& shardDB;
    BlockServicesCacheDB& blockServicesCache;

    // replication information
    std::shared_ptr<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> replicas; // used for replicating shard within location
    std::shared_ptr<std::vector<FullShardInfo>> leadersAtOtherLocations; // used for cross location replication

    // statistics
    std::array<Timings, maxShardMessageKind+1> timings;
    std::array<ErrorCount, maxShardMessageKind+1> errors;
    std::atomic<double> logEntriesQueueSize;
    std::atomic<double> readerRequestQueueSize;
    std::array<std::atomic<double>, 2> receivedRequests; // how many requests we got at once from each socket
    std::atomic<double> pulledWriteRequests; // how many requests we got from write queue
    std::atomic<double> pulledReadRequests; // how many requests we got from read queue

    // we should get up to date information from registry before we start serving any requests
    // this is populated by ShardRegisterer
    std::atomic<bool> isInitiated;
    std::atomic<bool> isBlockServiceCacheInitiated;

    ShardShared() = delete;
    ShardShared(const ShardOptions& options_, SharedRocksDB& sharedDB_, BlockServicesCacheDB& blockServicesCache_, ShardDB& shardDB_, LogsDB& logsDB_, UDPSocketPair&& sock) :
        options(options_),
        socks({std::move(sock)}),
        writerRequestsQueue(WRITER_QUEUE_SIZE),
        readerRequestsQueue(READER_QUEUE_SIZE),
        sharedDB(sharedDB_),
        logsDB(logsDB_),
        shardDB(shardDB_),
        blockServicesCache(blockServicesCache_),
        replicas(nullptr),
        leadersAtOtherLocations(std::make_shared<std::vector<FullShardInfo>>()),
        logEntriesQueueSize(0),
        readerRequestQueueSize(0),
        pulledWriteRequests(0),
        pulledReadRequests(0),
        isInitiated(false),
        isBlockServiceCacheInitiated(false)
    {
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
            if (unlikely(env._shouldLog(LogLevel::LOG_TRACE))) {
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
            if (unlikely(env._shouldLog(LogLevel::LOG_TRACE))) {
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

static constexpr std::array<uint32_t, 6> ShardProtocols = {
        SHARD_REQ_PROTOCOL_VERSION,
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
    std::vector<ShardWriterRequest> _writeEntries;

    // read requests buffer
    std::vector<ShardReq> _readRequests;

    std::unique_ptr<UDPReceiver<1>> _receiver;
    std::unique_ptr<ShardChannel> _channel;
public:
    ShardServer(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared) :
        Loop(logger, xmon, "server"),
        _shared(shared)
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

        if (replicaId != LogsDB::REPLICA_COUNT) {
            auto& resp = _writeEntries.emplace_back().setLogsDBResponse();
            resp.replicaId = replicaId;
            respMsg = &resp.msg;
        } else {
            // try matching to other locations
            auto leadersPtr = _shared.leadersAtOtherLocations;
            for (auto& leader : *leadersPtr) {
                if (leader.locationId == 0 && leader.addrs.contains(msg.clientAddr)) {
                    auto& resp = _writeEntries.emplace_back().setProxyLogsDBResponse();
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
            _writeEntries.pop_back();
            return;
        }
        LOG_DEBUG(_env, "Received response %s for requests id %s from replica id %s", respMsg->body.kind(), respMsg->id, replicaId);
    }

    void _handleLogsDBRequest(UDPMessage& msg) {
        LOG_DEBUG(_env, "received LogsDBRequest from %s", msg.clientAddr);
        LogReqMsg* reqMsg = nullptr;

        auto replicaId = _getReplicaId(msg.clientAddr);

        if (replicaId != LogsDB::REPLICA_COUNT) {
            auto& req = _writeEntries.emplace_back().setLogsDBRequest();
            req.replicaId = replicaId;
            reqMsg = &req.msg;
        } else {
            // try matching to other locations
            auto leadersPtr = _shared.leadersAtOtherLocations;
            for (auto& leader : *leadersPtr) {
                if (leader.addrs.contains(msg.clientAddr)) {
                    auto& req = _writeEntries.emplace_back().setProxyLogsDBRequest();
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
            _writeEntries.pop_back();
            return;
        }
        LOG_DEBUG(_env, "Received request %s with requests id %s from replica id %s", reqMsg->body.kind(), reqMsg->id, replicaId);
    }

    uint8_t _getReplicaId(const IpPort& clientAddress) {
        auto replicasPtr = _shared.replicas;
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
                if (unlikely(_env._shouldLog(LogLevel::LOG_TRACE))) {
                    LOG_TRACE(_env, "parsed request: %s", req);
                } else {
                    LOG_DEBUG(_env, "parsed request: <omitted>");
                }
        } else {
            LOG_DEBUG(_env, "parsed request: %s", req);
        }

        auto& entry = readOnlyShardReq(req.body.kind()) ? _readRequests.emplace_back() : _writeEntries.emplace_back().setShardReq();
        entry.sockIx = msg.socketIx;
        entry.clientAddr = msg.clientAddr;
        entry.receivedAt = t0;
        entry.protocol = protocol;
        entry.msg = std::move(req);
    }

    void _handleShardResponse(UDPMessage& msg, uint32_t protocol) {
        ALWAYS_ASSERT(protocol == PROXY_SHARD_RESP_PROTOCOL_VERSION);
        LOG_DEBUG(_env, "received message from %s", msg.clientAddr);
        if (unlikely(!_shared.options.isLeader())) {
            LOG_DEBUG(_env, "not leader, dropping response %s", msg.clientAddr);
            return;
        }

        ProxyShardRespMsg& resp = _writeEntries.emplace_back().setShardResp();
        try {
            resp.unpack(msg.buf, _expandedShardKey);
        } catch (const BincodeException& err) {
            LOG_ERROR(_env, "Could not parse: %s", err.what());
            RAISE_ALERT(_env, "could not parse request from %s, dropping it.", msg.clientAddr);
            _writeEntries.pop_back();
            return;
        }

        auto t0 = ternNow();
        LOG_DEBUG(_env, "parsed shard response from %s: %s", msg.clientAddr, resp);
    }

public:
    virtual void step() override {
        if (unlikely(!_shared.isInitiated.load(std::memory_order_acquire) || !_shared.isBlockServiceCacheInitiated.load(std::memory_order_acquire))) {
            (100_ms).sleepRetry();
            return;
        }

        _writeEntries.clear();
        _readRequests.clear();

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

        // write out write requests to queue
        {
            size_t numRequests = _writeEntries.size();
            if (numRequests > 0) {
                LOG_DEBUG(_env, "pushing %s requests to writer", numRequests);
                uint32_t pushed = _shared.writerRequestsQueue.push(_writeEntries);
                _shared.logEntriesQueueSize = _shared.logEntriesQueueSize*0.95 + _shared.writerRequestsQueue.size()*0.05;
                if (pushed < numRequests) {
                    LOG_INFO(_env, "tried to push %s requests to write queue, but pushed %s instead", numRequests, pushed);
                }
            }
        }
        // write out read requests to queue
        {
            size_t numReadRequests = _readRequests.size();
            if (numReadRequests > 0) {
                LOG_DEBUG(_env, "pushing %s read requests to reader", numReadRequests);
                uint32_t pushed = _shared.readerRequestsQueue.push(_readRequests);
                _shared.readerRequestQueueSize = _shared.readerRequestQueueSize*0.95 + _shared.readerRequestsQueue.size()*0.05;
                if (pushed < numReadRequests) {
                    LOG_INFO(_env, "tried to push %s elements to reader queue, but pushed %s instead", numReadRequests, pushed);
                }
            }
        }
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
    std::vector<ShardWriterRequest> _workItems;

    LogsDB& _logsDB;
    bool _isLogsDBLeader;
    uint64_t _currentLogIndex;
    LogIdx _knownLastReleased; // used by leaders in proxy location to determine if they need to catch up

    Duration _nextTimeout;
    // buffers for separated items from work queue
    std::vector<LogsDBRequest> _logsDBRequests; // requests from other replicas
    std::vector<LogsDBResponse> _logsDBResponses; // responses from other replicas
    std::vector<ShardReq>       _shardRequests; // requests from clients or proxy locations
    std::vector<ProxyShardRespMsg> _shardResponses; // responses from shard leader in primary location
    std::vector<ProxyLogsDBRequest> _proxyLogsDBRequests; // catchup requests from logs db leader in proxy location
                                                          // or write requests from logs db leader in primary location
    std::vector<ProxyLogsDBResponse> _proxyLogsDBResponses; // responses from logs db leader in primary location

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

    uint64_t _requestIdCounter;

    std::unordered_map<uint64_t, ShardReq> _logIdToShardRequest; // used to track which log entry was generated by which request
    std::shared_ptr<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> _replicaInfo;

    virtual void sendStop() override {
        _shared.writerRequestsQueue.close();
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
        memset(_catchupWindow.data(), 0, _catchupWindow.size()*sizeof(decltype(_catchupWindow)::value_type));
        convertProb("outgoing", _shared.options.serverOptions.simulateOutgoingPacketDrop, _outgoingPacketDropProbability);
        _workItems.reserve(_maxWorkItemsAtOnce);
        _shardEntries.reserve(LogsDB::IN_FLIGHT_APPEND_WINDOW);
    }

    virtual ~ShardWriter() = default;

    void _sendProxyAndCatchupRequests() {
        if (!_shared.options.isProxyLocation()) {
            return;
        }
        auto leadersAtOtherLocations = _shared.leadersAtOtherLocations;
        AddrsInfo* primaryLeaderAddress = nullptr;
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

    void _tryReplicateToOtherLocations() {
        // if we are not leader or in proxy location already we will not try to replicate
        if (!_isLogsDBLeader || _shared.options.isProxyLocation()) {
            return;
        }
        auto leadersAtOtherLocations = _shared.leadersAtOtherLocations;
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
                                packCheckPointedShardResponse(_env, _shared, _shared.sock().addr(), _sender, dropArtificially, request, resp, _expandedCDCKey);
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
                    ALWAYS_ASSERT(_inFlightRequestKeys.erase(InFlightRequestKey{request.msg.id, request.clientAddr}) == 1);
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
        }

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
            if (req.msg.body.kind() == ShardMessageKind::SHARD_SNAPSHOT) {
                snapshotReq = std::move(req);
                continue;
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

        _shared.shardDB.flush(true);
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

    AddrsInfo* addressFromReplicaId(ReplicaId id) {
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
        _workItems.clear();
        _logsDBRequests.clear();
        _logsDBResponses.clear();
        _logsDBOutRequests.clear();
        _logsDBOutResponses.clear();
        _shardRequests.clear();
        _shardResponses.clear();
        _shardEntries.clear();
        _proxyLogsDBRequests.clear();
        _proxyLogsDBResponses.clear();
        _proxyReadRequests.clear();
        _proxyReadRequestsIndices.clear();

        _replicaInfo = _shared.replicas;
        uint32_t pulled = _shared.writerRequestsQueue.pull(_workItems, _maxWorkItemsAtOnce, _nextTimeout);
        auto start = ternNow();
        if (likely(pulled > 0)) {
            LOG_DEBUG(_env, "pulled %s requests from write queue", pulled);
            _shared.pulledWriteRequests = _shared.pulledWriteRequests*0.95 + ((double)pulled)*0.05;
        }
        if (unlikely(_shared.writerRequestsQueue.isClosed())) {
            // queue is closed, stop
            stop();
            return;
        }

        for(auto& item : _workItems) {
            switch (item.kind()) {
            case WriterQueueEntryKind::LOGSDB_REQUEST:
                _logsDBRequests.emplace_back(item.moveLogsDBRequest());
                break;
            case WriterQueueEntryKind::LOGSDB_RESPONSE:
                _logsDBResponses.emplace_back(item.moveLogsDBResponse());
                break;
            case WriterQueueEntryKind::SHARD_REQUEST:
                _shardRequests.emplace_back(item.moveShardReq());
                break;
            case WriterQueueEntryKind::SHARD_RESPONSE:
                _shardResponses.emplace_back(item.moveShardResp());
                break;
            case WriterQueueEntryKind::PROXY_LOGSDB_REQUEST:
                _proxyLogsDBRequests.emplace_back(item.moveProxyLogsDBRequest());
                break;
            case WriterQueueEntryKind::PROXY_LOGSDB_RESPONSE:
                _proxyLogsDBResponses.emplace_back(item.moveProxyLogsDBResponse());
                break;
            }
        }
        logsDBStep();
        auto loopTime = ternNow() - start;
    }
};

struct ShardReader : Loop {
private:

    ShardShared& _shared;
    AES128Key _expandedShardKey;
    AES128Key _expandedCDCKey;
    ShardRespMsg _respContainer;
    CdcToShardRespMsg _checkPointedrespContainer;
    std::vector<ShardReq> _requests;

    UDPSender _sender;
    RandomGenerator _packetDropRand;
    uint64_t _outgoingPacketDropProbability; // probability * 10,000

    virtual void sendStop() override {
        _shared.readerRequestsQueue.close();
    }

public:
    ShardReader(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared) :
        Loop(logger, xmon, "reader"),
        _shared(shared),
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
        uint32_t pulled = _shared.readerRequestsQueue.pull(_requests, MAX_RECV_MSGS * 2);
        auto start = ternNow();
        if (likely(pulled > 0)) {
            LOG_DEBUG(_env, "pulled %s requests from read queue", pulled);
            _shared.pulledReadRequests = _shared.pulledReadRequests*0.95 + ((double)pulled)*0.05;
        }
        if (unlikely(_shared.readerRequestsQueue.isClosed())) {
            // queue is closed, stop
            stop();
            return;
        }

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
    const std::string _registryHost;
    const uint16_t _registryPort;
    XmonNCAlert _alert;
public:
    ShardRegisterer(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared) :
        PeriodicLoop(logger, xmon, "registerer", {1_sec, 1, 2_mins, 1}),
        _shared(shared),
        _shrid(_shared.options.shrid()),
        _location(_shared.options.logsDBOptions.location),
        _noReplication(_shared.options.logsDBOptions.noReplication),
        _registryHost(_shared.options.registryClientOptions.host),
        _registryPort(_shared.options.registryClientOptions.port)
    {}

    virtual ~ShardRegisterer() = default;

    void init() {
        _env.updateAlert(_alert, "Waiting to register ourselves for the first time");
    }

    virtual bool periodicStep() {
        {
            LOG_INFO(_env, "Registering ourselves (shard %s, location %s, %s) with registry", _shrid, (int)_location, _shared.sock().addr());
            // ToDO: once leader election is fully enabled report or leader status instead of value of flag passed on startup
            const auto [err, errStr] = registerShard(_registryHost, _registryPort, 10_sec, _shrid, _location, _shared.options.isLeader(), _shared.sock().addr());
            if (err == EINTR) { return false; }
            if (err) {
                _env.updateAlert(_alert, "Couldn't register ourselves with registry: %s", errStr);
                return false;
            }
        }

        {
            std::vector<FullShardInfo> allReplicas;
            LOG_INFO(_env, "Fetching replicas for shardId %s from registry", _shrid.shardId());
            const auto [err, errStr] = fetchShardReplicas(_registryHost, _registryPort, 10_sec, _shrid.shardId(), allReplicas);
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
            if (unlikely(!_shared.replicas)) {
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
            if (unlikely(!_shared.replicas || *_shared.replicas != localReplicas)) {
                LOG_DEBUG(_env, "Updating replicas to %s %s %s %s %s", localReplicas[0], localReplicas[1], localReplicas[2], localReplicas[3], localReplicas[4]);
                std::atomic_exchange(&_shared.replicas, std::make_shared<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>>(localReplicas));
            }
            if (unlikely(_shared.leadersAtOtherLocations->size() != leadersAtOtherLocations.size())) {
                LOG_DEBUG(_env, "Updating leaders at other locations to %s", leadersAtOtherLocations);
                std::atomic_exchange(&_shared.leadersAtOtherLocations, std::make_shared<std::vector<FullShardInfo>>(std::move(leadersAtOtherLocations)));
            } else {
                for (size_t i = 0; i < leadersAtOtherLocations.size(); ++i) {
                    if (_shared.leadersAtOtherLocations->at(i).locationId != leadersAtOtherLocations[i].locationId ||
                        _shared.leadersAtOtherLocations->at(i).id != leadersAtOtherLocations[i].id ||
                        _shared.leadersAtOtherLocations->at(i).addrs != leadersAtOtherLocations[i].addrs)
                    {
                        LOG_DEBUG(_env, "Updating leaders at other locations to %s", leadersAtOtherLocations);
                        std::atomic_exchange(&_shared.leadersAtOtherLocations, std::make_shared<std::vector<FullShardInfo>>(std::move(leadersAtOtherLocations)));
                        break;
                    }
                }
            }
        }
        _shared.isInitiated.store(true, std::memory_order_release);
        _env.clearAlert(_alert);
        return true;
    }
};

struct ShardBlockServiceUpdater : PeriodicLoop {
private:
    ShardShared& _shared;
    ShardReplicaId _shrid;
    std::string _registryHost;
    uint16_t _registryPort;
    XmonNCAlert _alert;
    std::vector<BlockServiceDeprecatedInfo> _blockServices;
    std::vector<BlockServiceInfoShort> _currentBlockServices;
    bool _updatedOnce;
public:
    ShardBlockServiceUpdater(Logger& logger, std::shared_ptr<XmonAgent>& xmon, ShardShared& shared):
        PeriodicLoop(logger, xmon, "bs_updater", {1_sec, shared.options.isLeader() ? 30_sec : 2_mins}),
        _shared(shared),
        _shrid(_shared.options.shrid()),
        _registryHost(_shared.options.registryClientOptions.host),
        _registryPort(_shared.options.registryClientOptions.port),
        _updatedOnce(false)
    {
        _env.updateAlert(_alert, "Waiting to fetch block services for the first time");
    }

    virtual bool periodicStep() override {
        if (!_blockServices.empty()) {
            // We delayed applying cache update most likely we were leader. We should apply it now
            _shared.blockServicesCache.updateCache(_blockServices, _currentBlockServices);
            _blockServices.clear();
            _currentBlockServices.clear();
        }

        LOG_INFO(_env, "about to fetch block services from %s:%s", _registryHost, _registryPort);
        const auto [err, errStr] = fetchBlockServices(_registryHost, _registryPort, 10_sec, _shrid.shardId(), _blockServices, _currentBlockServices);
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
            _shared.blockServicesCache.updateCache(_blockServices, _currentBlockServices);
            _blockServices.clear();
            _currentBlockServices.clear();
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

struct ShardMetricsInserter : PeriodicLoop {
private:
    InfluxDB _influxDB;
    ShardShared& _shared;
    ShardReplicaId _shrid;
    uint8_t _location;
    XmonNCAlert _sendMetricsAlert;
    MetricsBuilder _metricsBuilder;
    std::unordered_map<std::string, uint64_t> _rocksDBStats;
    std::array<XmonNCAlert, 2> _sockQueueAlerts;
    XmonNCAlert _writeQueueAlert;
public:
    ShardMetricsInserter(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const InfluxDB& influxDB, ShardShared& shared):
        PeriodicLoop(logger, xmon, "metrics", {1_sec, 1.0, 1_mins, 0.1}),
        _influxDB(influxDB),
        _shared(shared),
        _shrid(_shared.options.shrid()),
        _location(_shared.options.logsDBOptions.location),
        _sendMetricsAlert(XmonAppType::DAYTIME, 5_mins),
        _sockQueueAlerts({XmonAppType::NEVER, XmonAppType::NEVER}),
        _writeQueueAlert(XmonAppType::NEVER)
    {}

    virtual ~ShardMetricsInserter() = default;

    virtual bool periodicStep() {
        _shared.sharedDB.dumpRocksDBStatistics();
        for (int i = 0; i < 2; i++) {
            if (std::ceil(_shared.receivedRequests[i]) >= MAX_RECV_MSGS) {
                _env.updateAlert(_sockQueueAlerts[i], "recv queue for sock %s is full (%s)", i, _shared.receivedRequests[i]);
            } else {
                _env.clearAlert(_sockQueueAlerts[i]);
            }
        }
        if (std::ceil(_shared.logEntriesQueueSize) >= WRITER_QUEUE_SIZE) {
            _env.updateAlert(_writeQueueAlert, "write queue is full (%s)", _shared.logEntriesQueueSize);
        } else {
            _env.clearAlert(_writeQueueAlert);
        }
        auto now = ternNow();
        for (ShardMessageKind kind : allShardMessageKind) {
            const ErrorCount& errs = _shared.errors[(int)kind];
            for (int i = 0; i < errs.count.size(); i++) {
                uint64_t count = errs.count[i].load();
                if (count == 0) { continue; }
                _metricsBuilder.measurement("eggsfs_shard_requests");
                _metricsBuilder.tag("shard", _shrid);
                _metricsBuilder.tag("location", int(_location));
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
        {
            _metricsBuilder.measurement("eggsfs_shard_write_queue");
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.fieldFloat("size", _shared.logEntriesQueueSize);
            _metricsBuilder.timestamp(now);
        }
        {
            _metricsBuilder.measurement("eggsfs_shard_read_queue");
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.fieldFloat("size", _shared.readerRequestQueueSize);
            _metricsBuilder.timestamp(now);
        }
        for (int i = 0; i < _shared.receivedRequests.size(); i++) {
            _metricsBuilder.measurement("eggsfs_shard_received_requests");
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.tag("socket", i);
            _metricsBuilder.fieldFloat("count", _shared.receivedRequests[i]);
            _metricsBuilder.timestamp(now);
        }
        {
            _metricsBuilder.measurement("eggsfs_shard_pulled_write_requests");
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.fieldFloat("count", _shared.pulledWriteRequests);
            _metricsBuilder.timestamp(now);
        }
        {
            _metricsBuilder.measurement("eggsfs_shard_pulled_read_requests");
            _metricsBuilder.tag("shard", _shrid);
            _metricsBuilder.tag("location", int(_location));
            _metricsBuilder.fieldFloat("count", _shared.pulledReadRequests);
            _metricsBuilder.timestamp(now);
        }
        {
            _rocksDBStats.clear();
            _shared.sharedDB.rocksDBMetrics(_rocksDBStats);
            for (const auto& [name, value]: _rocksDBStats) {
                _metricsBuilder.measurement("eggsfs_shard_rocksdb");
                _metricsBuilder.tag("shard", _shrid);
                _metricsBuilder.tag("location", int(_location));
                _metricsBuilder.fieldU64(name, value);
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

    BlockServicesCacheDB blockServicesCache(logger, xmon, sharedDB);

    ShardDB shardDB(logger, xmon, options.shardId, options.logsDBOptions.location, options.transientDeadlineInterval, sharedDB, blockServicesCache);
    LogsDB logsDB(logger, xmon, sharedDB, options.logsDBOptions.replicaId, shardDB.lastAppliedLogEntry(), options.logsDBOptions.noReplication, options.logsDBOptions.avoidBeingLeader);
    env.clearAlert(dbInitAlert);

    ShardShared shared(options, sharedDB, blockServicesCache, shardDB, logsDB, UDPSocketPair(env, options.serverOptions.addrs));

    threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardServer>(logger, xmon, shared)));
    threads.emplace_back(LoopThread::Spawn(std::make_unique<ShardReader>(logger, xmon, shared)));
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
