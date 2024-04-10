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
#include <unordered_set>

#include "Bincode.hpp"
#include "Msgs.hpp"
#include "Shuckle.hpp"
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

static std::pair<Sock, std::string> shuckleSock(const std::string& host, uint16_t port, Duration timeout) {
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

static std::pair<int, std::string> writeShuckleRequest(int fd, const ShuckleReqContainer& req, Duration timeout) {
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
    WRITE_OUT(&SHUCKLE_REQ_PROTOCOL_VERSION, sizeof(SHUCKLE_REQ_PROTOCOL_VERSION));
    uint32_t len = 1 + bbuf.len();
    WRITE_OUT(&len, sizeof(len));
    auto kind = req.kind();
    WRITE_OUT(&kind, sizeof(kind));
    WRITE_OUT(bbuf.data, bbuf.len());
#undef WRITE_OUT
    return {};
}

static std::pair<int, std::string> readShuckleResponse(int fd, ShuckleRespContainer& resp, Duration timeout) {
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
    if (protocol != SHUCKLE_RESP_PROTOCOL_VERSION) {
        std::ostringstream ss;
        ss << "bad shuckle protocol (expected " << SHUCKLE_RESP_PROTOCOL_VERSION << ", got " << protocol << ")";
        return {EIO, ss.str()};
    }
    uint32_t len;
    READ_IN(&len, sizeof(len));
    ShuckleMessageKind kind;
    READ_IN(&kind, sizeof(kind));
    std::vector<char> buf(len-1);
    READ_IN(buf.data(), buf.size());
#undef READ_IN
    BincodeBuf bbuf(buf.data(), buf.size());
    if (kind == ShuckleMessageKind::ERROR) {
        EggsError error = (EggsError)bbuf.unpackScalar<uint16_t>();
        std::stringstream ss;
        ss << "got error " << error;
        return {EIO, ss.str()};
    }
    resp.unpack(bbuf, kind);
    return {};
}

std::pair<int, std::string> fetchBlockServices(const std::string& addr, uint16_t port, Duration timeout, ShardId shid, std::vector<BlockServiceInfo>& blockServices, std::vector<BlockServiceId>& currentBlockServices) {
    blockServices.clear();
    currentBlockServices.clear();

#define FAIL(err, errStr) do { blockServices.clear(); currentBlockServices.clear(); return {err, errStr}; } while (0)

    auto [sock, err] = shuckleSock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), err};
    }

    // all block services
    {
        ShuckleReqContainer reqContainer;
        auto& req = reqContainer.setAllBlockServices();
        {
            const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        ShuckleRespContainer respContainer;
        {
            const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        blockServices = respContainer.getAllBlockServices().blockServices.els;
    }

    // current block services
    {
        ShuckleReqContainer reqContainer;
        auto& req = reqContainer.setShardBlockServices();
        req.shardId = shid;
        {
            const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        ShuckleRespContainer respContainer;
        {
            const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
            if (err) { FAIL(err, errStr); }
        }

        currentBlockServices = respContainer.getShardBlockServices().blockServices.els;
    }

    // check that all current block services are known -- there's a small race here
    // the caller should just retry in these cases.
    {
        std::unordered_set<uint64_t> knownBlockServices;
        for (const auto& bs : blockServices) {
            knownBlockServices.insert(bs.info.id.u64);
        }
        for (BlockServiceId bsId : currentBlockServices) {
            if (!knownBlockServices.contains(bsId.u64)) {
                std::stringstream ss;
                ss << "got unknown block service " << bsId << " in current block services, was probably added in the meantime, please retry";
                FAIL(EIO, ss.str());
            }
        }
    }

    return {};

#undef FAIL
}

std::pair<int, std::string> registerShardReplica(
    const std::string& addr, uint16_t port, Duration timeout, ShardReplicaId shrid, bool isLeader,
    const AddrsInfo& info
) {
    const auto [sock, errStr] = shuckleSock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    ShuckleReqContainer reqContainer;
    auto& req = reqContainer.setRegisterShardReplica();
    req.shrid = shrid;
    req.isLeader = isLeader;
    req.info = info;
    {
        const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    ShuckleRespContainer respContainer;
    {
        const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterShardReplica();

    return {};
}

std::pair<int, std::string> fetchShardReplicas(
    const std::string& addr, uint16_t port, Duration timeout, ShardReplicaId shrid, std::array<AddrsInfo, 5>& replicas
) {
    const auto [sock, errStr] = shuckleSock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    ShuckleReqContainer reqContainer;
    auto& req = reqContainer.setShardReplicas();
    req.id = shrid.shardId();

    {
        const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    ShuckleRespContainer respContainer;
    {
        const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    if (respContainer.getShardReplicas().replicas.els.size() != replicas.size()) {
        throw EGGS_EXCEPTION("expecting %s replicas, got %s", replicas.size(), respContainer.getShardReplicas().replicas.els.size());
    }
    for (int i = 0; i < replicas.size(); i++) {
        replicas[i] = respContainer.getShardReplicas().replicas.els[i];
    }

    return {};
}

std::pair<int, std::string> registerCDCReplica(const std::string& host, uint16_t port, Duration timeout, ReplicaId replicaId, bool isLeader, const AddrsInfo& info) {
    const auto [sock, errStr] = shuckleSock(host, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    ShuckleReqContainer reqContainer;
    auto& req = reqContainer.setRegisterCdcReplica();
    req.replica = replicaId;
    req.isLeader = isLeader;
    req.info = info;
    {
        const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    ShuckleRespContainer respContainer;
    {
        const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    respContainer.getRegisterCdcReplica();

    return {};
}

std::pair<int, std::string> fetchCDCReplicas(
    const std::string& addr, uint16_t port, Duration timeout, std::array<AddrsInfo, 5>& replicas
) {
    const auto [sock, errStr] = shuckleSock(addr, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    ShuckleReqContainer reqContainer;
    auto& req = reqContainer.setCdcReplicas();

    {
        const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    ShuckleRespContainer respContainer;
    {
        const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    if (respContainer.getCdcReplicas().replicas.els.size() != replicas.size()) {
        throw EGGS_EXCEPTION("expecting %s replicas, got %s", replicas.size(), respContainer.getCdcReplicas().replicas.els.size());
    }
    for (int i = 0; i < replicas.size(); i++) {
        replicas[i] = respContainer.getCdcReplicas().replicas.els[i];
    }

    return {};
}

std::pair<int, std::string> fetchShards(const std::string& host, uint16_t port, Duration timeout, std::array<ShardInfo, 256>& shards) {
    const auto [sock, errStr] = shuckleSock(host, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    ShuckleReqContainer reqContainer;
    reqContainer.setShards();
    {
        const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    ShuckleRespContainer respContainer;
    {
        const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }
    if (respContainer.getShards().shards.els.size() != shards.size()) {
        throw EGGS_EXCEPTION("expecting %s shards, got %s", shards.size(), respContainer.getShards().shards.els.size());
    }
    for (int i = 0; i < shards.size(); i++) {
        shards[i] = respContainer.getShards().shards.els[i];
    }

    return {};
}

bool parseShuckleAddress(const std::string& fullShuckleAddress, std::string& shuckleHost, uint16_t& shucklePort) {
    // split host:port
    auto colon = fullShuckleAddress.find(":");
    if (colon == fullShuckleAddress.size()) {
        std::cerr << "Could not find colon" << std::endl;
        return false;
    }
    // parse port
    if (fullShuckleAddress.size() - colon > 6) {
        return false; // port too long
    }
    uint32_t port = 0;
    for (int i = colon + 1; i < fullShuckleAddress.size(); i++) {
        char ch = fullShuckleAddress[i];
        if ((i == colon + 1 && ch == '0') || ch < '0' || ch > '9') {
            return false;
        }
        port = port*10 + (ch-'0');
    }
    if (port == 0 || port > 65535) {
        return false;
    }
    shucklePort = port;
    shuckleHost = {fullShuckleAddress.begin(), fullShuckleAddress.begin()+colon};
    return true;
}

std::pair<int, std::string> insertStats(
    const std::string& host,
    uint16_t port,
    Duration timeout,
    const std::vector<Stat>& stats
) {
    const auto [sock, errStr] = shuckleSock(host, port, timeout);
    if (sock.error()) {
        return {sock.getErrno(), errStr};
    }

    ShuckleReqContainer reqContainer;
    auto& req = reqContainer.setInsertStats();
    req.stats.els.insert(req.stats.els.end(), std::make_move_iterator(stats.begin()), std::make_move_iterator(stats.end()));
    {
        const auto [err, errStr] = writeShuckleRequest(sock.get(), reqContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    ShuckleRespContainer respContainer;
    {
        const auto [err, errStr] = readShuckleResponse(sock.get(), respContainer, timeout);
        if (err) { return {err, errStr}; }
    }

    return {};

}
