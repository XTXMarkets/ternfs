// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <iostream>
#include <cstdint>
#include <fcntl.h>
#include <sstream>
#include <string>
#include <sys/socket.h>
#include <netdb.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <sys/syscall.h>
#include <unistd.h>
#include <sys/time.h>
#include <array>
#include <netinet/tcp.h>

#include "Bincode.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "Protocol.hpp"
#include "RegistryClient.hpp"
#include "Exception.hpp"
#include "Connect.hpp"
#include "Loop.hpp"

static std::string explicitGenerateErrString(const std::string& what, int err, const char* str) {
    std::stringstream ss;
    ss << "could not " << what << ": " << err << "/" << str;
    return ss.str();
}

static std::string generateErrString(const std::string& what, int err) {
    return explicitGenerateErrString(what, err, (std::string(translateErrno(err)) + "=" + safe_strerror(err)).c_str());
}

static std::pair<int, std::string> writeRegistryRequest(int fd, const RegistryReqContainer& req, Duration timeout) {
    static_assert(std::endian::native == std::endian::little);
    // Serialize
    std::vector<char> buf(req.packedSize());
    BincodeBuf bbuf(buf.data(), buf.size());
    req.pack(bbuf);
    struct pollfd pfd{.fd = fd, .events = POLLOUT, .revents = 0};
    // Write out
#define WRITE_OUT(buf, len) \
    do { \
        if (unlikely(Loop::poll(&pfd, 1, timeout) < 0)) { \
            return {errno, generateErrString("poll socket", errno)}; \
        } \
        ssize_t written = write(fd, buf, len); \
        if (written < 0) { \
            return {errno, generateErrString("write request", errno)}; \
        } \
        if (written != len) { \
            return {EIO, "couldn't write full request"}; \
        } \
    } while (false)
    WRITE_OUT(&REGISTRY_REQ_PROTOCOL_VERSION, sizeof(REGISTRY_REQ_PROTOCOL_VERSION));
    uint32_t len = bbuf.len();
    WRITE_OUT(&len, sizeof(len));
    WRITE_OUT(bbuf.data, bbuf.len());
#undef WRITE_OUT
    return {};
}

static std::pair<int, std::string> readRegistryResponse(int fd, RegistryRespContainer& resp, Duration timeout) {
    static_assert(std::endian::native == std::endian::little);
    struct pollfd pfd{.fd = fd, .events = POLLIN, .revents = 0};
#define READ_IN(buf, count) \
    do { \
        ssize_t readSoFar = 0; \
        while (readSoFar < count) { \
            if (unlikely(Loop::poll(&pfd, 1, timeout) < 0)) { \
                return {errno, generateErrString("read request", errno)}; \
            } \
            ssize_t r = read(fd, buf+readSoFar, count-readSoFar); \
            if (r < 0) { \
                return {errno, generateErrString("read response", errno)}; \
            } \
            if (r == 0) { \
                return {EIO, "unexpected EOF"}; \
            } \
            readSoFar += r; \
        } \
    } while (0)
    uint32_t protocol;
    READ_IN(&protocol, sizeof(protocol));
    if (protocol != REGISTRY_RESP_PROTOCOL_VERSION) {
        std::ostringstream ss;
        ss << "bad registry protocol (expected " << REGISTRY_RESP_PROTOCOL_VERSION << ", got " << protocol << ")";
        return {EIO, ss.str()};
    }
    uint32_t len;
    READ_IN(&len, sizeof(len));
    std::vector<char> buf(len);
    READ_IN(buf.data(), buf.size());
#undef READ_IN
    BincodeBuf bbuf(buf.data(), buf.size());
    resp.unpack(bbuf);
    if (resp.kind() == RegistryMessageKind::ERROR) {
        std::stringstream ss;
        ss << "got error " << resp.getError();
        return {EIO, ss.str()};
    }
    return {};
}

// RegistryClient implementation

RegistryClient::RegistryClient(Logger& logger, std::shared_ptr<XmonAgent>& xmon,
                               const std::string& host, uint16_t port,
                               Duration timeout)
    : _host(host), _port(port), _timeout(timeout),
      _env(logger, xmon, "registry_client") {}

std::pair<int, std::string> RegistryClient::_ensureConnected() {
    if (!_sock.error()) {
        return {};
    }

    auto [sock, err] = connectToHost(_host, _port, _timeout);
    if (sock.error()) {
        return {sock.getErrno(), err};
    }

    struct timeval tv;
    tv.tv_sec = _timeout.ns/1'000'000'000ull;
    tv.tv_usec = (_timeout.ns%1'000'000'000ull)/1'000;

    if (setsockopt(sock.get(), SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv)) < 0) {
        throw SYSCALL_EXCEPTION("setsockopt SO_RCVTIMEO");
    }

    int keepalive = 1;
    if (setsockopt(sock.get(), SOL_SOCKET, SO_KEEPALIVE, &keepalive, sizeof(keepalive)) < 0) {
        throw SYSCALL_EXCEPTION("setsockopt SO_KEEPALIVE");
    }
    int keepidle = 30;
    if (setsockopt(sock.get(), IPPROTO_TCP, TCP_KEEPIDLE, &keepidle, sizeof(keepidle)) < 0) {
        throw SYSCALL_EXCEPTION("setsockopt TCP_KEEPIDLE");
    }
    int keepintvl = 10;
    if (setsockopt(sock.get(), IPPROTO_TCP, TCP_KEEPINTVL, &keepintvl, sizeof(keepintvl)) < 0) {
        throw SYSCALL_EXCEPTION("setsockopt TCP_KEEPINTVL");
    }
    int keepcnt = 3;
    if (setsockopt(sock.get(), IPPROTO_TCP, TCP_KEEPCNT, &keepcnt, sizeof(keepcnt)) < 0) {
        throw SYSCALL_EXCEPTION("setsockopt TCP_KEEPCNT");
    }

    LOG_DEBUG(_env, "connected to registry at %s:%s", _host, _port);
    _sock = std::move(sock);
    return {};
}

void RegistryClient::_closeConnection() {
    _sock = Sock();
}

std::pair<int, std::string> RegistryClient::_doRequest(RegistryReqContainer& req, RegistryRespContainer& resp) {
    return _doRequest(req, resp, _timeout);
}

std::pair<int, std::string> RegistryClient::_doRequest(RegistryReqContainer& req, RegistryRespContainer& resp, Duration timeout) {
    {
        const auto [err, errStr] = _ensureConnected();
        if (err) { return {err, errStr}; }
    }
    {
        const auto [err, errStr] = writeRegistryRequest(_sock.get(), req, timeout);
        if (err) { _closeConnection(); return {err, errStr}; }
    }
    {
        const auto [err, errStr] = readRegistryResponse(_sock.get(), resp, timeout);
        if (err) { _closeConnection(); return {err, errStr}; }
    }
    return {};
}

std::pair<int, std::string> RegistryClient::registerRegistry(
    ReplicaId replicaId, uint8_t location, bool isLeader,
    const AddrsInfo& addrs, bool bootstrap
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setRegisterRegistry();
    req.replicaId = replicaId;
    req.location = location;
    req.isLeader = isLeader;
    req.addrs = addrs;
    req.bootstrap = bootstrap;

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterRegistry(); // check that the response is of the right type
    return {};
}

std::pair<int, std::string> RegistryClient::fetchRegistryReplicas(
    std::vector<FullRegistryInfo>& replicas
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllRegistryReplicasDEPRECATED();

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }
    replicas = respContainer.getAllRegistryReplicasDEPRECATED().replicas.els;
    return {};
}

std::pair<int, std::string> RegistryClient::fetchRegistryReplicas(
    std::vector<FullRegistryInfo>& replicas,
    uint8_t minKnownReplicas,
    uint8_t location,
    Duration timeout
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllRegistryReplicas();
    req.minKnownReplicas = minKnownReplicas;
    req.location = location;

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    replicas = respContainer.getAllRegistryReplicas().replicas.els;
    return {};
}

std::pair<int, std::string> RegistryClient::registerShard(
    ShardReplicaId shrid, uint8_t location, bool isLeader,
    const AddrsInfo& addrs
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setRegisterShard();
    req.shrid = shrid;
    req.location = location;
    req.isLeader = isLeader;
    req.addrs = addrs;

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterShard(); // check that the response is of the right type
    return {};
}

std::pair<int, std::string> RegistryClient::fetchShardReplicas(
    ShardId shid, std::vector<FullShardInfo>& replicas
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllShardsDEPRECATED();

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }

    for(auto& shard : respContainer.getAllShardsDEPRECATED().shards.els) {
        if (shard.id.shardId() == shid) {
            replicas.emplace_back(std::move(shard));
        }
    }
    return {};
}

std::pair<int, std::string> RegistryClient::fetchShardReplicas(
    ShardId shid, std::vector<FullShardInfo>& replicas,
    uint8_t minKnownReplicas,
    uint8_t location,
    Duration timeout
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllShards();
    req.minKnownReplicas = minKnownReplicas;
    req.location = location;
    req.shardId = shid.u8;

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    for(auto& shard : respContainer.getAllShards().shards.els) {
        if (shard.id.shardId() == shid) {
            replicas.emplace_back(std::move(shard));
        }
    }
    return {};
}

std::pair<int, std::string> RegistryClient::registerCDCReplica(
    ReplicaId replicaId, uint8_t location, bool isLeader,
    const AddrsInfo& addrs
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setRegisterCdc();
    req.replica = replicaId;
    req.location = location;
    req.isLeader = isLeader;
    req.addrs = addrs;

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterCdc();
    return {};
}

std::pair<int, std::string> RegistryClient::fetchCDCReplicas(
    std::array<AddrsInfo, 5>& replicas
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setCdcReplicasDEPRECATED();

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }

    if (respContainer.getCdcReplicasDEPRECATED().replicas.els.size() != replicas.size()) {
        throw TERN_EXCEPTION("expecting %s replicas, got %s", replicas.size(), respContainer.getCdcReplicasDEPRECATED().replicas.els.size());
    }
    for (int i = 0; i < replicas.size(); i++) {
        replicas[i] = respContainer.getCdcReplicasDEPRECATED().replicas.els[i];
    }
    return {};
}

std::pair<int, std::string> RegistryClient::fetchCDCReplicas(
    std::array<AddrsInfo, 5>& replicas,
    uint8_t minKnownReplicas,
    uint8_t location,
    Duration timeout
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllCdc();
    req.minKnownReplicas = minKnownReplicas;
    req.location = location;

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    for (auto& cdc : respContainer.getAllCdc().replicas.els) {
        if (cdc.locationId != location) {
            continue;
        }
        replicas[cdc.replicaId.u8] = cdc.addrs;
    }
    return {};
}

std::pair<int, std::string> RegistryClient::fetchLocalShards(
    std::array<ShardInfo, 256>& shards
) {
    std::scoped_lock lock(_mutex);

    RegistryReqContainer reqContainer;
    reqContainer.setLocalShards();

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { return {err, errStr}; }
    }
    if (respContainer.getLocalShards().shards.els.size() != shards.size()) {
        throw TERN_EXCEPTION("expecting %s shards, got %s", shards.size(), respContainer.getLocalShards().shards.els.size());
    }
    for (int i = 0; i < shards.size(); i++) {
        shards[i] = respContainer.getLocalShards().shards.els[i];
    }
    return {};
}

std::pair<int, std::string> RegistryClient::fetchChangedBlockServices(
    TernTime since,
    TernTime& lastChange,
    std::vector<FullBlockServiceInfo>& blockServices
) {
    std::scoped_lock lock(_mutex);
    blockServices.clear();

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setChangedBlockServices();
    req.since = since;
    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { blockServices.clear(); return {err, errStr}; }
    }
    auto& resp = respContainer.getChangedBlockServices();
    lastChange = resp.lastChange;
    blockServices = std::move(resp.blockServices.els);

    return {};
}

std::pair<int, std::string> RegistryClient::fetchBlockServiceAvailableSpace(
    std::vector<BlockServiceSpace>& blockServices
) {
    std::scoped_lock lock(_mutex);
    blockServices.clear();

    RegistryReqContainer reqContainer;
    reqContainer.setBlockServiceAvailableSpace();
    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = _doRequest(reqContainer, respContainer);
        if (err) { blockServices.clear(); return {err, errStr}; }
    }
    blockServices = std::move(respContainer.getBlockServiceAvailableSpace().blockServices.els);

    return {};
}

bool parseRegistryAddress(const std::string& fullRegistryAddress, std::string& registryHost, uint16_t& registryPort) {
    // split host:port
    auto colon = fullRegistryAddress.find(":");
    if (colon == fullRegistryAddress.size()) {
        std::cerr << "Could not find colon" << std::endl;
        return false;
    }
    // parse port
    if (fullRegistryAddress.size() - colon > 6) {
        return false; // port too long
    }
    uint32_t port = 0;
    for (int i = colon + 1; i < fullRegistryAddress.size(); i++) {
        char ch = fullRegistryAddress[i];
        if ((i == colon + 1 && ch == '0') || ch < '0' || ch > '9') {
            return false;
        }
        port = port*10 + (ch-'0');
    }
    if (port == 0 || port > 65535) {
        return false;
    }
    registryPort = port;
    registryHost = {fullRegistryAddress.begin(), fullRegistryAddress.begin()+colon};
    return true;
}
