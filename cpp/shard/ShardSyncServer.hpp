// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include <array>
#include <cstdint>
#include <string>
#include <sys/epoll.h>
#include <unordered_map>
#include <vector>

#include "Env.hpp"
#include "MsgsGen.hpp"
#include "Protocol.hpp"
#include "Time.hpp"

struct SyncRequest {
    uint64_t requestId;
    SyncReqContainer req;
};

struct SyncResponse {
    uint64_t requestId;
    SyncRespContainer resp;
};

struct ShardSyncServerOptions {
    IpPort bindAddress;
    uint32_t maxConnections = 16;
};

class ShardSyncServer {
public:
    ShardSyncServer(const ShardSyncServerOptions& options, Env& env) :
        _options(options),
        _env(env),
        _lastRequestId(0)
    {}

    virtual ~ShardSyncServer();

    bool init();

    inline const IpPort& boundAddress() const { return _options.bindAddress; }

    // Returns true if poll returned events, false on error/timeout
    bool receiveMessages(Duration timeout);

    inline std::vector<SyncRequest>& receivedSyncRequests() { return _receivedRequests; }

    void sendSyncResponses(std::vector<SyncResponse>& responses);

private:
    static constexpr size_t MESSAGE_HEADER_SIZE = 8;
    static constexpr size_t MESSAGE_HEADER_LENGTH_OFFSET = 4;
    static constexpr int MAX_EVENTS = 64;

    const ShardSyncServerOptions _options;
    Env& _env;

    int _listenFd = -1;
    int _epollFd = -1;
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
    std::vector<SyncRequest> _receivedRequests;

    void _acceptConnection();
    void _removeClient(int fd);
    void _readClient(int fd);
    void _writeClient(int fd, bool registerEpoll = false);

    void _sendResponse(int fd, SyncRespContainer& resp);
};
