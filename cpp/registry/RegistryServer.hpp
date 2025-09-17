// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include "LogsDB.hpp"
#include "MultiplexedChannel.hpp"
#include "Random.hpp"
#include "RegistryCommon.hpp"
#include "RegistryKey.hpp"

static constexpr std::array<uint32_t, 2> RegistryProtocols = {
    LOG_REQ_PROTOCOL_VERSION,
    LOG_RESP_PROTOCOL_VERSION,
};

using RegistryChannel = MultiplexedChannel<RegistryProtocols.size(), RegistryProtocols>;

constexpr int MAX_RECV_MSGS = 500;

struct RegistryRequest {
  uint64_t requestId;
  RegistryReqContainer req;
};

struct RegistryResponse {
    uint64_t requestId;
    RegistryRespContainer resp;
};



class RegistryServer {
public:
    RegistryServer(const RegistryOptions &options, Env& env) :
        _options(options.serverOptions),
        _maxConnections(options.maxConnections),
        _env(env),
        _socks({UDPSocketPair(_env, _options.addrs)}),
        _boundAddresses(_socks[0].addr()),
        _lastRequestId(0),
        _sender(UDPSenderConfig{.maxMsgSize = MAX_UDP_MTU}),        
        _packetDropRand(ternNow().ns),
        _outgoingPacketDropProbability(_options.simulateOutgoingPacketDrop)
    {
        _receiver = std::make_unique<UDPReceiver<1>>(UDPReceiverConfig{
            .perSockMaxRecvMsg = MAX_RECV_MSGS, .maxMsgSize = MAX_UDP_MTU});
        expandKey(RegistryKey, _expandedRegistryKey);
    }

    virtual ~RegistryServer() = default;

    bool init();

    inline const AddrsInfo& boundAddresses() const { return _boundAddresses; }

    inline void setReplicas(std::shared_ptr<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> replicaInfo) {
        _replicaInfo = replicaInfo;
    }

    bool receiveMessages(Duration timeout);

    inline std::vector<LogsDBRequest>& receivedLogsDBRequests() { return _logsDBRequests; }
    inline std::vector<LogsDBResponse>& receivedLogsDBResponses() { return _logsDBResponses; }
    inline std::vector<RegistryRequest>& receivedRegistryRequests() { return _receivedRequests; }

    void sendLogsDBMessages(std::vector<LogsDBRequest *>& requests, std::vector<LogsDBResponse>& responses);
    void sendRegistryResponses(std::vector<RegistryResponse>& responses);

private:
    const ServerOptions _options;
    uint32_t _maxConnections;

    Env& _env;

    std::array<UDPSocketPair, 1> _socks; // in an array to play with UDPReceiver<>
    const AddrsInfo _boundAddresses;
    std::unique_ptr<UDPReceiver<1>> _receiver;
    RegistryChannel _channel;
    AES128Key _expandedRegistryKey;

    std::array<int, 2> _sockFds{-1, -1};
    int _epollFd = -1;
    static constexpr int MAX_EVENTS = 1024;
    std::array<epoll_event, MAX_EVENTS> _events;

    struct Client {
        int fd;
        std::string readBuffer;
        std::string writeBuffer;
        TernTime lastActive;
        size_t messageBytesProcessed;
        uint64_t inFlightRequestId;
    };

    std::unordered_map<int, Client> _clients;
    
    uint64_t _lastRequestId;
    std::unordered_map<uint64_t, int> _inFlightRequests; // request to fd mapping
    std::vector<RegistryRequest> _receivedRequests;


    std::shared_ptr<std::array<AddrsInfo, LogsDB::REPLICA_COUNT>> _replicaInfo;
    
    // log entries buffers
    std::vector<LogsDBRequest> _logsDBRequests;   // requests from other replicas
    std::vector<LogsDBResponse> _logsDBResponses; // responses from other replicas

    // outgoing network
    UDPSender _sender;
    RandomGenerator _packetDropRand;
    uint64_t _outgoingPacketDropProbability; // probability * 10,000

    void _acceptConnection(int fd);
    void _removeClient(int fd);
    void _readClient(int fd);
    void _writeClient(int fd, bool registerEpoll = false);

    uint8_t _getReplicaId(const IpPort &clientAddress);
    AddrsInfo* _addressFromReplicaId(ReplicaId id);

    void _handleLogsDBResponse(UDPMessage &msg);
    void _handleLogsDBRequest(UDPMessage &msg);

    void _sendResponse(int fd, RegistryRespContainer &resp);

    void _packLogsDBResponse(LogsDBResponse &response);
    void _packLogsDBRequest(LogsDBRequest &request);
};
