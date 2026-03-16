// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <mutex>
#include <string>
#include <utility>
#include <vector>

#include "Connect.hpp"
#include "Env.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"
#include "Time.hpp"

class RegistryClient {
public:
    RegistryClient(Logger& logger, std::shared_ptr<XmonAgent>& xmon,
                   const std::string& host, uint16_t port,
                   Duration timeout);
    ~RegistryClient() = default;

    RegistryClient(const RegistryClient&) = delete;
    RegistryClient& operator=(const RegistryClient&) = delete;

    std::pair<int, std::string> fetchBlockServices(
        std::vector<FullBlockServiceInfo>& blockServices
    );

    std::pair<int, std::string> registerRegistry(
        ReplicaId replicaId,
        uint8_t location,
        bool isLeader,
        const AddrsInfo& addrs,
        bool bootstrap
    );

    std::pair<int, std::string> fetchRegistryReplicas(
        std::vector<FullRegistryInfo>& replicas
    );

    std::pair<int, std::string> registerShard(
        ShardReplicaId shrid,
        uint8_t location,
        bool isLeader,
        const AddrsInfo& addrs
    );

    std::pair<int, std::string> fetchShardReplicas(
        ShardId shid,
        std::vector<FullShardInfo>& replicas
    );

    std::pair<int, std::string> registerCDCReplica(
        ReplicaId replicaId,
        uint8_t location,
        bool isLeader,
        const AddrsInfo& addrs
    );

    std::pair<int, std::string> fetchCDCReplicas(
        std::array<AddrsInfo, 5>& replicas
    );

    std::pair<int, std::string> fetchLocalShards(
        std::array<ShardInfo, 256>& shards
    );

private:
    const std::string _host;
    const uint16_t _port;
    const Duration _timeout;
    Env _env;

    std::mutex _mutex;
    Sock _sock;

    std::pair<int, std::string> _ensureConnected();
    void _closeConnection();
    std::pair<int, std::string> _doRequest(RegistryReqContainer& req, RegistryRespContainer& resp);
};
