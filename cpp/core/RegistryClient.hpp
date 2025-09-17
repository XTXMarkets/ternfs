// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "Msgs.hpp"
#include "MsgsGen.hpp"

// The host here is the scheme + host + port, e.g. `http://localhost:5000`.
//
// If the first number is non-zero, we errored out, and the element contains
// an error. We only returns error strings for errors that might be transient
// (e.g. we cannot connect to the server, or the connection dies), and crash
// on things that are almost certainly not transient (e.g. bad data on
// the wire).
//
// This function does double duty -- it both gets all the block services
// (the shard needs to know which ones exist to fill in addrs), but it also
// fills in the block services for the shard specifically.
std::pair<int, std::string> fetchBlockServices(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    ShardId shid,
    std::vector<BlockServiceDeprecatedInfo>& blockServices,
    std::vector<BlockServiceInfoShort>& currentBlockServices
);

std::pair<int, std::string> registerRegistry(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    ReplicaId replicaId,
    uint8_t location,
    bool isLeader,
    const AddrsInfo& addrs,
    bool bootstrap
);

std::pair<int, std::string> fetchRegistryReplicas(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    std::vector<FullRegistryInfo>& replicas
);

std::pair<int, std::string> registerShard(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    ShardReplicaId shrid,
    uint8_t location,
    bool isLeader,
    const AddrsInfo& addrs
);

std::pair<int, std::string> fetchShardReplicas(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    ShardId shid,
    std::vector<FullShardInfo>& replicas
);

std::pair<int, std::string> registerCDCReplica(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    ReplicaId replicaId,
    uint8_t location,
    bool isLeader,
    const AddrsInfo& addrs
);

std::pair<int, std::string> fetchCDCReplicas(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    std::array<AddrsInfo, 5>& replicas
);

std::pair<int, std::string> fetchLocalShards(
    const std::string& registryHost,
    uint16_t registryPort,
    Duration timeout,
    std::array<ShardInfo, 256>& shards
);

bool parseRegistryAddress(const std::string& fullRegistryAddress, std::string& registryHost, uint16_t& registryPort);
