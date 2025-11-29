// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "ShardSyncServer.hpp"

#include <cerrno>
#include <cstring>
#include <netinet/in.h>
#include <sys/socket.h>
#include <unistd.h>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "Loop.hpp"

ShardSyncServer::~ShardSyncServer() {
    for (auto& [fd, client] : _clients) {
        close(fd);
    }
    if (_listenFd != -1) {
        close(_listenFd);
    }
    if (_epollFd != -1) {
        close(_epollFd);
    }
}

bool ShardSyncServer::init() {
    _epollFd = epoll_create1(0);
    if (_epollFd == -1) {
        LOG_ERROR(_env, "Failed to create epoll instance: %s", strerror(errno));
        return false;
    }

    if (_options.bindAddress.ip.data[0] == 0) {
        LOG_INFO(_env, "Sync server not configured (no bind address)");
        return true;
    }

    _listenFd = socket(AF_INET, SOCK_STREAM | SOCK_NONBLOCK, 0);
    if (_listenFd == -1) {
        LOG_ERROR(_env, "Failed to create sync server socket: %s", strerror(errno));
        return false;
    }

    int opt = 1;
    setsockopt(_listenFd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));

    sockaddr_in sockAddr{};
    _options.bindAddress.toSockAddrIn(sockAddr);

    if (bind(_listenFd, (sockaddr*)&sockAddr, sizeof(sockAddr)) == -1) {
        LOG_ERROR(_env, "Failed to bind sync server socket: %s", strerror(errno));
        return false;
    }

    if (listen(_listenFd, SOMAXCONN) == -1) {
        LOG_ERROR(_env, "Failed to listen on sync server socket: %s", strerror(errno));
        return false;
    }

    epoll_event event{};
    event.events = EPOLLIN;
    event.data.fd = _listenFd;
    if (epoll_ctl(_epollFd, EPOLL_CTL_ADD, _listenFd, &event) == -1) {
        LOG_ERROR(_env, "Failed to register sync server listen socket for epoll: %s", strerror(errno));
        return false;
    }

    LOG_INFO(_env, "Sync server initialized on %s", _options.bindAddress);
    return true;
}

bool ShardSyncServer::receiveMessages(Duration timeout) {
    ALWAYS_ASSERT(_receivedRequests.empty());

    if (_epollFd == -1) {
        return false;
    }

    int numEvents = Loop::epollWait(_epollFd, &_events[0], _events.size(), timeout);
    LOG_TRACE(_env, "Sync server epoll returned %s events", numEvents);

    if (numEvents == -1) {
        if (errno != EINTR) {
            LOG_ERROR(_env, "Sync server epoll_wait error: %s", strerror(errno));
        }
        return false;
    }

    for (int i = 0; i < numEvents; ++i) {
        if (_events[i].data.fd == _listenFd) {
            _acceptConnection();
        } else if (_events[i].events & (EPOLLHUP | EPOLLRDHUP | EPOLLERR)) {
            _removeClient(_events[i].data.fd);
        } else if (_events[i].events & EPOLLIN) {
            _readClient(_events[i].data.fd);
        } else if (_events[i].events & EPOLLOUT) {
            _writeClient(_events[i].data.fd);
        }
    }

    return true;
}

void ShardSyncServer::sendSyncResponses(std::vector<SyncResponse>& responses) {
    for (auto& response : responses) {
        auto inFlightIt = _inFlightRequests.find(response.requestId);
        if (inFlightIt == _inFlightRequests.end()) {
            LOG_TRACE(_env, "Dropping sync response for requestId %s as request was dropped", response.requestId);
            continue;
        }
        int fd = inFlightIt->second;
        _inFlightRequests.erase(inFlightIt);
        if (response.resp.kind() == SyncMessageKind::EMPTY) {
            LOG_TRACE(_env, "Dropping sync connection with fd %s due to empty response", fd);
            _removeClient(fd);
            continue;
        }
        _sendResponse(fd, response.resp);
    }
}

void ShardSyncServer::_acceptConnection() {
    sockaddr_in clientAddr{};
    socklen_t clientAddrLen = sizeof(clientAddr);

    int clientFd = accept4(_listenFd, (sockaddr*)&clientAddr, &clientAddrLen, SOCK_NONBLOCK);

    if (clientFd == -1) {
        LOG_ERROR(_env, "Failed to accept sync connection: %s", strerror(errno));
        return;
    }

    if (_clients.size() >= _options.maxConnections) {
        LOG_DEBUG(_env, "Dropping sync connection as we reached connection limit");
        close(clientFd);
        return;
    }

    auto client_it = _clients.emplace(clientFd, Client{clientFd, {}, {}, ternNow(), 0, 0}).first;
    client_it->second.readBuffer.resize(MESSAGE_HEADER_SIZE);

    epoll_event event{};
    event.events = EPOLLIN | EPOLLHUP | EPOLLERR | EPOLLRDHUP;
    event.data.fd = clientFd;

    if (epoll_ctl(_epollFd, EPOLL_CTL_ADD, clientFd, &event) == -1) {
        LOG_ERROR(_env, "Failed to add sync client to epoll: %s", strerror(errno));
        _removeClient(clientFd);
        return;
    }

    LOG_TRACE(_env, "Accepted sync connection on fd %s", clientFd);
}

void ShardSyncServer::_readClient(int fd) {
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());

    Client& client = it->second;
    client.lastActive = ternNow();
    size_t bytesToRead = client.readBuffer.size() - client.messageBytesProcessed;
    ssize_t bytesRead;

    while (bytesToRead > 0 &&
           (bytesRead = read(fd, &client.readBuffer[client.messageBytesProcessed], bytesToRead)) > 0) {

        LOG_TRACE(_env, "Received %s bytes from sync client", bytesRead);
        bytesToRead -= bytesRead;
        client.messageBytesProcessed += bytesRead;

        if (bytesToRead > 0) {
            continue;
        }

        if (client.messageBytesProcessed == MESSAGE_HEADER_SIZE) {
            BincodeBuf buf{&client.readBuffer[0], MESSAGE_HEADER_SIZE};
            uint32_t protocol = buf.unpackScalar<uint32_t>();
            if (protocol != SYNC_REQ_PROTOCOL_VERSION) {
                LOG_ERROR(_env, "Invalid sync protocol version: %s", protocol);
                _removeClient(fd);
                return;
            }
            uint32_t len = buf.unpackScalar<uint32_t>();
            buf.ensureFinished();
            LOG_TRACE(_env, "Received sync message of length %s", len);
            bytesToRead = len;
            client.readBuffer.resize(len + MESSAGE_HEADER_SIZE);
        } else {
            LOG_TRACE(_env, "Unpacking sync ReadBuffer size %s", client.readBuffer.size());
            BincodeBuf buf{&client.readBuffer[MESSAGE_HEADER_SIZE], client.readBuffer.size() - MESSAGE_HEADER_SIZE};
            auto& req = _receivedRequests.emplace_back();

            try {
                req.req.unpack(buf);
                buf.ensureFinished();
                LOG_TRACE(_env, "Received sync request on fd %s, kind %s", fd, req.req.kind());

                // Remove read event from epoll after receiving complete request
                epoll_event event{};
                event.events = EPOLLHUP | EPOLLERR | EPOLLRDHUP;
                event.data.fd = fd;
                if (epoll_ctl(_epollFd, EPOLL_CTL_MOD, fd, &event) == -1) {
                    LOG_ERROR(_env, "Failed to modify sync client epoll event: %s", strerror(errno));
                    _receivedRequests.pop_back();
                    _removeClient(fd);
                    return;
                }
            } catch (const BincodeException& err) {
                LOG_ERROR(_env, "Could not parse SyncReq: %s", err.what());
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
        LOG_DEBUG(_env, "Error reading from sync client: %s", strerror(errno));
        _removeClient(fd);
    }

    if (bytesRead == 0) {
        _removeClient(fd);
    }
}

void ShardSyncServer::_removeClient(int fd) {
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());

    epoll_ctl(_epollFd, EPOLL_CTL_DEL, fd, nullptr);
    close(fd);
    if (it->second.inFlightRequestId != 0) {
        _inFlightRequests.erase(it->second.inFlightRequestId);
    }
    _clients.erase(it);
    LOG_TRACE(_env, "Removed sync client %s", fd);
}

void ShardSyncServer::_sendResponse(int fd, SyncRespContainer& resp) {
    LOG_TRACE(_env, "Sending sync response to client %s, kind %s", fd, resp.kind());
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());
    auto& client = it->second;
    ALWAYS_ASSERT(client.writeBuffer.empty());
    ALWAYS_ASSERT(client.readBuffer.empty());
    ALWAYS_ASSERT(client.messageBytesProcessed == 0);

    uint32_t len = resp.packedSize();
    client.writeBuffer.resize(len + MESSAGE_HEADER_SIZE);
    BincodeBuf buf(client.writeBuffer);
    buf.packScalar(SYNC_RESP_PROTOCOL_VERSION);
    buf.packScalar(len);
    resp.pack(buf);
    buf.ensureFinished();
    client.inFlightRequestId = 0;
    _writeClient(fd, true);
}

void ShardSyncServer::_writeClient(int fd, bool registerEpoll) {
    auto it = _clients.find(fd);
    ALWAYS_ASSERT(it != _clients.end());
    auto& client = it->second;
    client.lastActive = ternNow();
    ssize_t bytesToWrite = client.writeBuffer.size() - client.messageBytesProcessed;
    ssize_t bytesWritten = 0;
    LOG_TRACE(_env, "Writing to sync client %s, %s bytes left", fd, bytesToWrite);

    while (bytesToWrite > 0 &&
           (bytesWritten = write(fd, &client.writeBuffer[client.messageBytesProcessed], bytesToWrite)) > 0) {
        LOG_TRACE(_env, "Sent %s bytes to sync client", bytesWritten);
        client.messageBytesProcessed += bytesWritten;
        bytesToWrite -= bytesWritten;
    }

    LOG_TRACE(_env, "Finished writing to sync client %s, %s bytes left", fd, bytesToWrite);

    if (bytesToWrite > 0 && registerEpoll) {
        struct epoll_event ev;
        ev.events = EPOLLOUT | EPOLLHUP | EPOLLERR | EPOLLRDHUP;
        ev.data.fd = fd;
        if (epoll_ctl(_epollFd, EPOLL_CTL_MOD, fd, &ev) == -1) {
            LOG_ERROR(_env, "Failed to modify epoll for sync client %s", fd);
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
            LOG_ERROR(_env, "Failed to modify epoll for sync client %s", fd);
            _removeClient(fd);
            return;
        }
    }
}
