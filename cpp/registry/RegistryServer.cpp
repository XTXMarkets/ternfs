// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "RegistryServer.hpp"
#include "Assert.hpp"
#include "Env.hpp"
#include "MsgsGen.hpp"

bool RegistryServer::init() {
    _epollFd = epoll_create1(0);
    if (_epollFd == -1) {
      LOG_ERROR(_env, "Failed to create epoll instance: %s",
                strerror(errno));
      return false;
    }
    for (int i = 0; i < _options.addrs.size(); ++i) {
        auto& addr = _options.addrs[i];
        if (addr.ip.data[0] == 0) {
            continue;
        }
        int fd = socket(AF_INET, SOCK_STREAM | SOCK_NONBLOCK, 0);
        if (fd == -1) {
            LOG_ERROR(_env, "Failed to create socket: %s", strerror(errno));
            return false;
        }

        int opt = 1;
        setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

        sockaddr_in sockAddr{};
        addr.toSockAddrIn(sockAddr);

        if (bind(fd, (sockaddr *)&sockAddr, sizeof(sockAddr)) == -1) {
            LOG_ERROR(_env, "Failed to bind socket: %s", strerror(errno));
            return false;
        }

        if (listen(fd, SOMAXCONN) == -1) {
            LOG_ERROR(_env, "Failed to listen on socket: %s", strerror(errno));
            return false;
        }

        epoll_event event{};
        event.events = EPOLLIN;
        event.data.fd = fd;
        if (epoll_ctl(_epollFd, EPOLL_CTL_ADD, fd, &event) == -1){
            LOG_ERROR(_env, "Failed to register for epoll fd %s", fd);
            return false;
        }
        _sockFds[i] = fd;
    }
    if (_socks[0].registerEpoll(_epollFd) == -1) {
        LOG_ERROR(_env, "Failed to register udp socks for epoll");
        return false;
    }
    LOG_INFO(_env, "initialized sockets");
    return true;
}

bool RegistryServer::receiveMessages(Duration timeout){
    ALWAYS_ASSERT(_receivedRequests.empty());
    ALWAYS_ASSERT(_logsDBRequests.empty());
    ALWAYS_ASSERT(_logsDBResponses.empty());    

    int numEvents = Loop::epollWait(_epollFd, &_events[0], _events.size(), timeout);
    LOG_TRACE(_env, "epoll returned %s events", numEvents);
    
    if (numEvents == -1) {
        if (errno != EINTR) {
            LOG_ERROR(_env, "epoll_wait error: %s", strerror(errno));
        }
        return false;
    }

    bool haveUdpMessages = false;
    for (int i = 0; i < numEvents; ++i) {
        if (_events[i].data.fd == _sockFds[0]) {
            _acceptConnection(_sockFds[0]); 
        } else if (_events[i].data.fd == _sockFds[1]) {
            _acceptConnection(_sockFds[1]);
        } else if (_socks[0].containsFd(_events[i].data.fd)) {
            haveUdpMessages = true;
        } else if (_events[i].events & (EPOLLHUP | EPOLLRDHUP | EPOLLERR)) {
            _removeClient(_events[i].data.fd);
        } else if (_events[i].events & EPOLLIN) {
            _readClient(_events[i].data.fd);
        } else if (_events[i].events & EPOLLOUT) {
            _writeClient(_events[i].data.fd);
        }
    }
    if (haveUdpMessages) {
        if (!_channel.receiveMessages(_env, _socks, *_receiver, MAX_RECV_MSGS, -1, true)) {
            LOG_ERROR(_env, "failed receiving UDP messages");
        };
        for (auto &msg : _channel.protocolMessages(LOG_RESP_PROTOCOL_VERSION)) {
            _handleLogsDBResponse(msg);
        }
        for (auto &msg : _channel.protocolMessages(LOG_REQ_PROTOCOL_VERSION)) {
            _handleLogsDBRequest(msg);
        }
    }
    return true;
}

void RegistryServer::sendLogsDBMessages(std::vector<LogsDBRequest *>& requests, std::vector<LogsDBResponse>& responses) {
    for (auto request : requests) {
        _packLogsDBRequest(*request);
    }
    for (auto& response : responses) {
        _packLogsDBResponse(response);
    }
    requests.clear();
    responses.clear();
    _sender.sendMessages(_env, _socks[0]);
}

void RegistryServer::sendRegistryResponses(std::vector<RegistryResponse>& responses) {
    for (auto& response : responses) {
        auto inFlightIt = _inFlightRequests.find(response.requestId);
        if (inFlightIt == _inFlightRequests.end()) {
            LOG_TRACE(_env, "Dropping response %s for requestId %s as request was dropped ", response.resp, response.requestId);
            continue;
        }
        int fd = inFlightIt->second;
        _inFlightRequests.erase(inFlightIt);
        if (response.resp.kind() == RegistryMessageKind::EMPTY) {
            // drop connection on empty response
            LOG_TRACE(_env, "Dropping connection with fd %s due to empty response", fd);
            _removeClient(fd);
            continue;
        }
        _sendResponse(fd, response.resp);
    }
}

static constexpr size_t MESSAGE_HEADER_SIZE = 8;
static constexpr size_t MESSAGE_HEADER_LENGTH_OFFSET = 4;

void RegistryServer::_acceptConnection(int fd) {
    sockaddr_in clientAddr{};
    socklen_t clientAddrLen = sizeof(clientAddr);

    int clientFd = accept4(fd, (sockaddr *)&clientAddr, &clientAddrLen, SOCK_NONBLOCK);

    if (clientFd == -1) {
        LOG_ERROR(_env, "Failed to accept connection: %s", strerror(errno));
        return;
    }

    if (_clients.size() == _maxConnections) {
        LOG_DEBUG(_env, "dropping connection as we reached connection limit");
        close(fd);
        return;
    }

    auto client_it = _clients.emplace(clientFd, Client{clientFd, {}, {}, ternNow(),0,0}).first;
    client_it->second.readBuffer.resize(MESSAGE_HEADER_SIZE);

    epoll_event event{};
    event.events = EPOLLIN | EPOLLHUP | EPOLLERR | EPOLLRDHUP;
    event.data.fd = clientFd;

    if (epoll_ctl(_epollFd, EPOLL_CTL_ADD, clientFd, &event) == -1) {
        LOG_ERROR(_env, "Failed to add client to epoll: %s", strerror(errno));
        _removeClient(clientFd);
        return;
    }
}

  
void RegistryServer::_readClient(int fd) {
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());

    Client &client = it->second;
    client.lastActive = ternNow();
    size_t bytesToRead = client.readBuffer.size() - client.messageBytesProcessed;
    ssize_t bytesRead;

    while (bytesToRead > 0 &&
        (bytesRead = read(fd, &client.readBuffer[client.messageBytesProcessed], bytesToRead)) > 0) {
        
        LOG_TRACE(_env, "Received %s bytes from client", bytesRead);
        bytesToRead -= bytesRead;
        client.messageBytesProcessed += bytesRead;
        if (bytesToRead > 0) {
            continue;
        }
        if (client.messageBytesProcessed == MESSAGE_HEADER_SIZE) {
            BincodeBuf buf{&client.readBuffer[0], MESSAGE_HEADER_SIZE};
            uint32_t protocol = buf.unpackScalar<uint32_t>();
            if (protocol != REGISTRY_REQ_PROTOCOL_VERSION) {
                LOG_ERROR(_env, "Invalid protocol version: %s", protocol);
                _removeClient(fd);
                return;
            }
            uint32_t len = buf.unpackScalar<uint32_t>();
            buf.ensureFinished();
            LOG_TRACE(_env, "Received message of length %s", len);
            bytesToRead = len;
            client.readBuffer.resize(len + MESSAGE_HEADER_SIZE);
            LOG_TRACE(_env, "ReadBuffer new size %s", client.readBuffer.size());
        } else {
            LOG_TRACE(_env, "Unpacking ReadBuffer size %s", client.readBuffer.size());
            BincodeBuf buf{&client.readBuffer[MESSAGE_HEADER_SIZE], client.readBuffer.size() - MESSAGE_HEADER_SIZE};
            auto &req = _receivedRequests.emplace_back();
            
            try {
                LOG_TRACE(_env, "buf len %s", buf.remaining());
                req.req.unpack(buf);
                buf.ensureFinished();
                LOG_TRACE(_env, "Recieved request on fd %s, %s", fd, req.req);
                // Remove read event from epoll after receiving complete request
                epoll_event event{};
                event.events =
                    EPOLLHUP | EPOLLERR | EPOLLRDHUP; // Keep only hangup detection
                event.data.fd = fd;
                if (epoll_ctl(_epollFd, EPOLL_CTL_MOD, fd, &event) == -1) {
                    LOG_ERROR(_env, "Failed to modify client epoll event: %s", strerror(errno));
                    _receivedRequests.pop_back();
                    _removeClient(fd);
                    return;
                }
            } catch (const BincodeException &err) {
                LOG_ERROR(_env, "Could not parse RegistryReq: %s", err.what());
                _receivedRequests.pop_back();
                _removeClient(fd);
                return;
            }
            req.requestId = ++_lastRequestId;
            client.readBuffer.clear();
            client.messageBytesProcessed = 0;
            client.inFlightRequestId = req.requestId;
            _inFlightRequests.emplace(req.requestId, fd);
        }
    }

    if (bytesRead == -1 && errno != EAGAIN && errno != EWOULDBLOCK) {
        LOG_DEBUG(_env, "Error reading from client: %s", strerror(errno));
        _removeClient(fd);
    }

    if (bytesRead == 0) {
         _removeClient(fd);
    }
}

void RegistryServer::_removeClient(int fd) {
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());

    epoll_ctl(_epollFd, EPOLL_CTL_DEL, fd, nullptr);
    close(fd);
    if (it->second.inFlightRequestId != 0) {
        _inFlightRequests.erase(it->second.inFlightRequestId);
    }
    _clients.erase(it);
    LOG_TRACE(_env, "removing client %s", fd);
  }

  void RegistryServer::_handleLogsDBResponse(UDPMessage &msg) {
    LOG_TRACE(_env, "received LogsDBResponse from %s", msg.clientAddr);

    LogRespMsg *respMsg = nullptr;

    auto replicaId = _getReplicaId(msg.clientAddr);

    if (replicaId != LogsDB::REPLICA_COUNT) {
        auto &resp = _logsDBResponses.emplace_back();
        resp.replicaId = replicaId;
        respMsg = &resp.msg;
    }

    if (respMsg == nullptr) {
        LOG_DEBUG(_env, "We can't match this address to replica or other leader. Dropping");
        return;
    }

    try {
        respMsg->unpack(msg.buf, _expandedRegistryKey);
    } catch (const BincodeException &err) {
        LOG_ERROR(_env, "Could not parse LogsDBResponse: %s", err.what());
        _logsDBResponses.pop_back();
        return;
    }
    LOG_TRACE(_env, "Received response %s for requests id %s from replica id %s", respMsg->body.kind(), respMsg->id, replicaId);
}

void RegistryServer::_handleLogsDBRequest(UDPMessage &msg) {
    LOG_TRACE(_env, "received LogsDBRequest from %s", msg.clientAddr);
    LogReqMsg *reqMsg = nullptr;

    auto replicaId = _getReplicaId(msg.clientAddr);

    if (replicaId != LogsDB::REPLICA_COUNT) {
        auto &req = _logsDBRequests.emplace_back();
        req.replicaId = replicaId;
        reqMsg = &req.msg;
    }

    if (reqMsg == nullptr) {
        LOG_DEBUG(_env, "We can't match this address to replica or other leader. Dropping");
        return;
    }

    try {
        reqMsg->unpack(msg.buf, _expandedRegistryKey);
    } catch (const BincodeException &err) {
        LOG_ERROR(_env, "Could not parse LogsDBRequest: %s", err.what());
        _logsDBRequests.pop_back();
        return;
    }
    LOG_TRACE(_env, "Received request %s with requests id %s from replica id %s", reqMsg->body.kind(), reqMsg->id, replicaId);
}

uint8_t RegistryServer::_getReplicaId(const IpPort &clientAddress) {
    auto replicasPtr = _replicaInfo;
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

void RegistryServer::_sendResponse(int fd, RegistryRespContainer &resp) {
    LOG_TRACE(_env, "Sending response to client %s, resp %s", fd, resp);
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());
    auto &client = it->second;
    ALWAYS_ASSERT(client.writeBuffer.empty());
    ALWAYS_ASSERT(client.readBuffer.empty());
    ALWAYS_ASSERT(client.messageBytesProcessed == 0);
    uint32_t len = resp.packedSize();
    client.writeBuffer.resize(len + MESSAGE_HEADER_SIZE);
    BincodeBuf buf(client.writeBuffer);
    buf.packScalar(REGISTRY_RESP_PROTOCOL_VERSION);
    buf.packScalar(len);
    resp.pack(buf);
    buf.ensureFinished();
    client.inFlightRequestId = 0;
    _writeClient(fd, true);
}

void RegistryServer::_writeClient(int fd, bool registerEpoll) {
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());
    auto &client = it->second;
    client.lastActive = ternNow();
    ssize_t bytesToWrite = client.writeBuffer.size() - client.messageBytesProcessed;
    ssize_t bytesWritten = 0;
    LOG_TRACE(_env, "writing to client %s, %s bytes left", fd, bytesToWrite);

    while (bytesToWrite > 0 && (bytesWritten = write(fd, &client.writeBuffer[client.messageBytesProcessed], bytesToWrite)) > 0) {
        LOG_TRACE(_env, "Sent %s bytes to client", bytesWritten);
        client.messageBytesProcessed += bytesWritten;
        bytesToWrite -= bytesWritten;
    }
    LOG_TRACE(_env, "finished writing to client %s, %s bytes left", fd, bytesToWrite);
    if (bytesToWrite > 0 && registerEpoll) {
        struct epoll_event ev;
        ev.events = EPOLLOUT | EPOLLHUP | EPOLLERR | EPOLLRDHUP;
        ev.data.fd = fd;
        if (epoll_ctl(_epollFd, EPOLL_CTL_MOD, fd, &ev) == -1) {
            LOG_ERROR(_env, "Failed to modify epoll for client %s", fd);
            _removeClient(fd);
            return;
        }
    }
    if (bytesToWrite == 0) {
        struct epoll_event ev;
        ev.events = EPOLLIN | EPOLLHUP | EPOLLERR | EPOLLRDHUP;
        ev.data.fd = fd;
        client.messageBytesProcessed = 0;
        client.readBuffer.resize(MESSAGE_HEADER_SIZE);
        client.writeBuffer.clear();
        if (epoll_ctl(_epollFd, EPOLL_CTL_MOD, fd, &ev) == -1) {
            LOG_ERROR(_env, "Failed to modify epoll for client %s", fd);
            _removeClient(fd);
            return;
        }
    }
}

AddrsInfo* RegistryServer::_addressFromReplicaId(ReplicaId id) {
    if (!_replicaInfo) {
        return nullptr;
    }
    auto &addr = (*_replicaInfo)[id.u8];
    if (addr[0].port == 0) {
        return nullptr;
    }
    return &addr;
}

void RegistryServer::_packLogsDBResponse(LogsDBResponse &response) {
    auto addrInfoPtr = _addressFromReplicaId(response.replicaId);
    if (unlikely(addrInfoPtr == nullptr)) {
        LOG_TRACE(_env, "No information for replica id %s. dropping response", response.replicaId);
        return;
    }
    auto &addrInfo = *addrInfoPtr;

    bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
    if (unlikely(dropArtificially)) {
        LOG_TRACE(_env, "artificially dropping response %s", response.msg.id);
        return;
    }

    _sender.prepareOutgoingMessage(_env, _socks[0].addr(), addrInfo, 
        [&response, this](BincodeBuf &buf) {
            response.msg.pack(buf, _expandedRegistryKey);
    });

    LOG_TRACE(_env, "will send response for req id %s kind %s to %s", response.msg.id, response.msg.body.kind(), addrInfo);
}

void RegistryServer::_packLogsDBRequest(LogsDBRequest &request) {
    auto addrInfoPtr = _addressFromReplicaId(request.replicaId);
    if (unlikely(addrInfoPtr == nullptr)) {
        LOG_TRACE(_env, "No information for replica id %s. dropping request", request.replicaId);
        return;
    }
    auto &addrInfo = *addrInfoPtr;

    bool dropArtificially = _packetDropRand.generate64() % 10'000 < _outgoingPacketDropProbability;
    if (unlikely(dropArtificially)) {
        LOG_TRACE(_env, "artificially dropping request %s", request.msg.id);
        return;
    }

    _sender.prepareOutgoingMessage(_env, _socks[0].addr(), addrInfo, 
        [&request, this](BincodeBuf &buf) {
            request.msg.pack(buf, _expandedRegistryKey);
    });

    LOG_TRACE(_env, "will send request for req id %s kind %s to %s", request.msg.id, request.msg.body.kind(), addrInfo);
}
