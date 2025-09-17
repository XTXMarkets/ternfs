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
#include <unordered_map>
#include <unordered_set>

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

static std::pair<Sock, std::string> registrySock(const std::string& host, uint16_t port, Duration timeout) {
    auto [sock, err] = connectToHost(host, port, timeout);
    if (sock.error()) {
        return {std::move(sock), err};
    }

    struct timeval tv;
    tv.tv_sec = timeout.ns/1'000'000'000ull;
    tv.tv_usec = (timeout.ns%1'000'000'000ull)/1'000;

    if (setsockopt(sock.get(), SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv)) < 0) {
        throw SYSCALL_EXCEPTION("setsockopt");
    }

    return {std::move(sock), ""};
}

static std::pair<int, std::string> writeRegistryRequest(int fd, const RegistryReqContainer& req, Duration timeout) {
    static_assert(std::endian::native == std::endian::little);
    // Serialize
    std::vector<char> buf(req.packedSize());
    BincodeBuf bbuf(buf.data(), buf.size());
    req.pack(bbuf);
    struct pollfd pfd{.fd = fd, .events = POLLOUT};
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
    struct pollfd pfd{.fd = fd, .events = POLLIN};
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

std::pair<int, std::string> fetchBlockServices(const std::string& addr, uint16_t port, Duration timeout, ShardId shid, std::vector<BlockServiceDeprecatedInfo>& blockServices, std::vector<BlockServiceInfoShort>& currentBlockServices) {
    blockServices.clear();
    currentBlockServices.clear();

#define FAIL(err, errStr) do { blockServices.clear(); currentBlockServices.clear(); return {err, errStr}; } while (0)

    auto [sock, err] = registrySock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), err};
    }

    // all block services
    {
        RegistryReqContainer reqContainer;
        auto& req = reqContainer.setAllBlockServicesDeprecated();
        {
            const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        RegistryRespContainer respContainer;
        {
            const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        blockServices = respContainer.getAllBlockServicesDeprecated().blockServices.els;
    }

    // current block services
    {
        RegistryReqContainer reqContainer;
        auto& req = reqContainer.setShardBlockServices();
        req.shardId = shid;
        {
            const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        RegistryRespContainer respContainer;
        {
            const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        currentBlockServices = respContainer.getShardBlockServices().blockServices.els;
    }

    // check that all current block services are known -- there's a small race here
    // the caller should just retry in these cases.
    // check that all current block services are from different failure domains
    // registry should guarantee that when sending response but verify the invariant
    {
        std::unordered_set<uint64_t> knownBlockServices;
        std::unordered_map<uint64_t, const BlockServiceDeprecatedInfo* > bsIdToBlockService;
        std::unordered_set<std::string> fdSet;
        for (const auto& bs : blockServices) {
            knownBlockServices.insert(bs.id.u64);
            bsIdToBlockService[bs.id.u64] = &bs;
        }

        for (auto storageClass : {HDD_STORAGE, FLASH_STORAGE}) {
            fdSet.clear();
            for (BlockServiceInfoShort bs : currentBlockServices) {
                if (bs.storageClass != storageClass) { continue; }
                if (!knownBlockServices.contains(bs.id.u64)) {
                    std::stringstream ss;
                    ss << "got unknown block service " << bs.id << " in current block services, was probably added in the meantime, please retry";
                    FAIL(EIO, ss.str());
                }
                auto fdName = std::string((const char*)bs.failureDomain.name.data.data(), bs.failureDomain.name.data.size());
                if (!fdSet.insert(fdName).second) {
                    std::stringstream ss;
                    ss << "got multiple block services in the same failure domain: " << fdName;
                    FAIL(EIO, ss.str());
                }
            }
            if (fdSet.size() < 14) {
                std::stringstream ss;
                ss << "we need at least 14 block services per storage class but we got " << storageClass << ": " << fdSet.size();
                FAIL(EIO, ss.str());
            }
        }
    }

    return {};

#undef FAIL
}

std::pair<int, std::string> registerRegistry(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    ReplicaId replicaId,
    uint8_t location,
    bool isLeader,
    const AddrsInfo& addrs,
    bool bootstrap
) {
    const auto [sock, errStr] = registrySock(registryHost, registryPort, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setRegisterRegistry();
    req.replicaId = replicaId;
    req.location = location;
    req.isLeader = isLeader;
    req.addrs = addrs;
    req.bootstrap = bootstrap;
    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterRegistry(); // check that the response is of the right type

    return {};
}

std::pair<int, std::string> fetchRegistryReplicas(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    std::vector<FullRegistryInfo>& replicas
) {
    const auto [sock, errStr] = registrySock(registryHost, registryPort, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllRegistryReplicas();
    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    replicas = respContainer.getAllRegistryReplicas().replicas.els;

    return {};
}

std::pair<int, std::string> registerShard(
    const std::string& addr, uint16_t port, Duration timeout, ShardReplicaId shrid, uint8_t location, bool isLeader,
    const AddrsInfo& addrs
) {
    const auto [sock, errStr] = registrySock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setRegisterShard();
    req.shrid = shrid;
    req.location = location;
    req.isLeader = isLeader;
    req.addrs = addrs;
    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterShard(); // check that the response is of the right type

    return {};
}

std::pair<int, std::string> fetchShardReplicas(
    const std::string& addr, uint16_t port, Duration timeout, ShardId shid, std::vector<FullShardInfo>& replicas
) {
    const auto [sock, errStr] = registrySock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setAllShards();
    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    for(auto& shard : respContainer.getAllShards().shards.els) {
        if (shard.id.shardId() == shid) {
            replicas.emplace_back(std::move(shard));
        }
    }

    return {};
}

std::pair<int, std::string> registerCDCReplica(const std::string& host, uint16_t port, Duration timeout, ReplicaId replicaId, uint8_t location, bool isLeader, const AddrsInfo& addrs) {
    const auto [sock, errStr] = registrySock(host, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setRegisterCdc();
    req.replica = replicaId;
    req.location = location;
    req.isLeader = isLeader;
    req.addrs = addrs;
    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterCdc();

    return {};
}

std::pair<int, std::string> fetchCDCReplicas(
    const std::string& addr, uint16_t port, Duration timeout, std::array<AddrsInfo, 5>& replicas
) {
    const auto [sock, errStr] = registrySock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    auto& req = reqContainer.setCdcReplicasDEPRECATED();

    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
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

std::pair<int, std::string> fetchLocalShards(const std::string& host, uint16_t port, Duration timeout, std::array<ShardInfo, 256>& shards) {
    const auto [sock, errStr] = registrySock(host, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    RegistryReqContainer reqContainer;
    reqContainer.setLocalShards();
    {
        const auto [err, errStr] = writeRegistryRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    RegistryRespContainer respContainer;
    {
        const auto [err, errStr] = readRegistryResponse(sock.get(), respContainer, timeout);
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
