// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <atomic>
#include <cstring>
#include <fcntl.h>
#include <memory>
#include <netinet/in.h>
#include <sys/epoll.h>
#include <sys/socket.h>
#include <unistd.h>
#include <unordered_map>
#include <unordered_set>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "Env.hpp"
#include "ErrorCount.hpp"
#include "LogsDB.hpp"
#include "Loop.hpp"
#include "Msgs.hpp"
#include "MsgsGen.hpp"

#include "Registerer.hpp"
#include "Registry.hpp"
#include "RegistryDB.hpp"
#include "RegistryReader.hpp"
#include "RegistryServer.hpp"
#include "RegistryWriter.hpp"
#include "BlockServiceFileTracker.hpp"

#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "Timings.hpp"

#include "XmonAgent.hpp"

struct RegistryState {
    explicit RegistryState() :
        logEntriesQueueSize(0), readingRequests(0), writingRequests(0),
        activeConnections(0), requestsInProgress(0)
    {
        for (RegistryMessageKind kind : allRegistryMessageKind) {
        timings[(int)kind] = Timings::Standard();
        }
        for (auto &x : receivedRequests) {
        x = 0;
        }
    }

    std::unique_ptr<RegistryServer> server;

    // databases
    std::unique_ptr<SharedRocksDB> sharedDB;
    std::unique_ptr<LogsDB> logsDB;
    std::unique_ptr<RegistryDB> registryDB;

    // statistics
    std::array<Timings, maxRegistryMessageKind + 1> timings;
    std::array<ErrorCount, maxRegistryMessageKind + 1> errors;
    std::atomic<double> logEntriesQueueSize;
    std::array<std::atomic<double>, 2> receivedRequests; // how many requests we got at once from each socket
    std::atomic<double> readingRequests; // how many requests we got from write queue
    std::atomic<double> writingRequests; // how many requests we got from read queue
    std::atomic<double> activeConnections;
    std::atomic<double> requestsInProgress;
};

class RegistryLoop : public Loop {
public:
    RegistryLoop(
        Logger &logger, std::shared_ptr<XmonAgent> xmon, const RegistryOptions& options,
        Registerer& registerer, RegistryServer& server, LogsDB& logsDB, RegistryDB& registryDB,
        std::vector<RegistryReader*> readers, RegistryWriter& writer, BlockServiceFileTracker& tracker) :
        Loop(logger, xmon, "server"),
        _options(options),
        _registerer(registerer),
        _server(server),
        _logsDB(logsDB),
        _registryDB(registryDB),
        _writer(writer),
        _tracker(tracker),
        _replicaFinishedBootstrap({}),
        _boostrapFinished(false),
        _readers(std::move(readers))
        {}

    virtual ~RegistryLoop() {}

    virtual void step() override {
        _server.setReplicas(_registerer.replicas());
        if (!_server.receiveMessages(_logsDB.getNextTimeout())){
            return;
        }

        auto now = ternNow();

        _tracker.pushShardResponses(_server.receivedShardResponses());
        _server.receivedShardResponses().clear();

        _writer.pushLogsDBResponses(_server.receivedLogsDBResponses());
        _server.receivedLogsDBResponses().clear();

        _writer.pushLogsDBRequests(_server.receivedLogsDBRequests());
        _server.receivedLogsDBRequests().clear();

        auto& receivedRequests = _server.receivedRegistryRequests();

        for (auto &req : receivedRequests) {
            // Always serve registry replicas from in-memory state regardless of
            // bootstrap phase, so there is no data gap during the transition.
            if (req.req.kind() == RegistryMessageKind::ALL_REGISTRY_REPLICAS_DE_PR_EC_AT_ED ||
                req.req.kind() == RegistryMessageKind::ALL_REGISTRY_REPLICAS) {
                _readRequests.emplace_back(std::move(req));
                continue;
            }

            if (unlikely(!_boostrapFinished)) {
                _processBootstrapRequest(req);
                continue;
            }

            if (unlikely(!_logsDB.isLeader())) {
                LOG_DEBUG(_env, "not leader. dropping request from client %s", req.requestId);
                continue;
            }
            switch (req.req.kind()) {
            case RegistryMessageKind::REGISTER_REGISTRY: {
                const auto &regiReq = req.req.getRegisterRegistry();
                if (regiReq.location == _options.logsDBOptions.location) {
                    _registryDB.updateReplicaInfo(regiReq);
                }
                _writeRequests.emplace_back(std::move(req));
                break;
            }
            case RegistryMessageKind::LOCAL_SHARDS:
            case RegistryMessageKind::LOCAL_CDC:
            case RegistryMessageKind::INFO:
            case RegistryMessageKind::REGISTRY:
            case RegistryMessageKind::LOCATIONS:
            case RegistryMessageKind::LOCAL_CHANGED_BLOCK_SERVICES:
            case RegistryMessageKind::CHANGED_BLOCK_SERVICES_AT_LOCATION:
            case RegistryMessageKind::SHARDS_AT_LOCATION:
            case RegistryMessageKind::CDC_AT_LOCATION:
            case RegistryMessageKind::SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
            case RegistryMessageKind::CDC_REPLICAS_DE_PR_EC_AT_ED:
            case RegistryMessageKind::ALL_SHARDS_DE_PR_EC_AT_ED:
            case RegistryMessageKind::SHARD_BLOCK_SERVICES:
            case RegistryMessageKind::ALL_CDC_DE_PR_EC_AT_ED:
            case RegistryMessageKind::ERASE_DECOMMISSIONED_BLOCK:
            case RegistryMessageKind::ALL_BLOCK_SERVICES_DEPRECATED:
            case RegistryMessageKind::ALL_BLOCK_SERVICES:
            case RegistryMessageKind::ALL_SHARDS:
            case RegistryMessageKind::ALL_CDC:
            case RegistryMessageKind::BLOCK_SERVICES_NEEDING_MIGRATION:
            case RegistryMessageKind::CHANGED_BLOCK_SERVICES:
            case RegistryMessageKind::BLOCK_SERVICE_AVAILABLE_SPACE: {
                _readRequests.emplace_back(std::move(req));
                break;
            }
            case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE: {
                const auto& decomReq = req.req.getDecommissionBlockService();
                BlockServiceId targetId = decomReq.id;

                std::vector<FullBlockServiceInfo> blockServices;
                _registryDB.blockServices(blockServices);

                const FullBlockServiceInfo* targetBS = nullptr;

                struct GroupKey {
                    uint8_t loc;
                    uint8_t sc;
                    bool operator==(const GroupKey& o) const { return loc == o.loc && sc == o.sc; }
                };
                struct GroupStats {
                    GroupKey key;
                    uint32_t unreadableCount = 0;
                };

                struct UnreadableFDKey {
                    uint8_t loc;
                    uint8_t sc;
                    FailureDomain fd;

                    bool operator==(const UnreadableFDKey& other) const {
                        return loc == other.loc && sc == other.sc && fd == other.fd;
                    }
                };

                struct UnreadableFDKeyHash {
                    std::size_t operator()(const UnreadableFDKey& k) const {
                        size_t hash = 2166136261u;
                        auto combine = [&](uint8_t b) {
                            hash ^= b;
                            hash *= 16777619u;
                        };

                        combine(k.loc);
                        combine(k.sc);

                        for (uint8_t b : k.fd.name.data) {
                            combine(b);
                        }
                        return hash;
                    }
                };

                std::vector<GroupStats> stats;
                std::unordered_set<UnreadableFDKey, UnreadableFDKeyHash> unreadableFDs;

                for (const auto& bs : blockServices) {
                    if (bs.id == targetId) {
                        targetBS = &bs;
                    }

                    if(!bs.hasFiles || blockServiceFlagsReadable(bs.flags)) {
                        continue;
                    }

                    GroupKey key{bs.locationId, bs.storageClass};
                    auto it = std::find_if(stats.begin(), stats.end(), [&](const GroupStats& g){ return g.key == key; });
                    if (it == stats.end()) {
                        stats.push_back({key, 1});
                        unreadableFDs.insert({bs.locationId, bs.storageClass, bs.failureDomain});
                    } else {
                        if (unreadableFDs.find({bs.locationId, bs.storageClass, bs.failureDomain}) == unreadableFDs.end()) {
                            unreadableFDs.insert({bs.locationId, bs.storageClass, bs.failureDomain});
                            it->unreadableCount++;
                        }
                    }
                }

                if (!targetBS) {
                    auto& resp = _registryResponses.emplace_back();
                    resp.requestId = req.requestId;
                    resp.resp.setError() = TernError::BLOCK_SERVICE_NOT_FOUND;
                    break;
                }

                if (targetBS->flags == BlockServiceFlags::DECOMMISSIONED) {
                    auto& resp = _registryResponses.emplace_back();
                    resp.requestId = req.requestId;
                    resp.resp.setDecommissionBlockService();
                    break;
                }

                auto now = ternNow();
                auto lastIt = std::find_if(_lastDecommissionTimes.begin(), _lastDecommissionTimes.end(),
                    [&](const auto& p){ return p.first == targetBS->failureDomain; });

                if (lastIt != _lastDecommissionTimes.end()) {
                    if (now - lastIt->second < _options.minDecomInterval) {
                        auto& resp = _registryResponses.emplace_back();
                        resp.requestId = req.requestId;
                        resp.resp.setError() = TernError::AUTO_DECOMMISSION_RATE_LIMITED;
                        break;
                    }
                }

                GroupKey targetKey{targetBS->locationId, targetBS->storageClass};
                auto it = std::find_if(stats.begin(), stats.end(), [&](const GroupStats& g){ return g.key == targetKey; });

                if (it != stats.end() && it->unreadableCount >= _options.alertAfterUnavailableFailureDomains) {
                    auto& resp = _registryResponses.emplace_back();
                    resp.requestId = req.requestId;
                    resp.resp.setError() = TernError::AUTO_DECOMMISSION_FORBIDDEN;
                    break;
                }

                if (lastIt != _lastDecommissionTimes.end()) {
                    lastIt->second = now;
                } else {
                    _lastDecommissionTimes.emplace_back(targetBS->failureDomain, now);
                }

                _writeRequests.emplace_back(std::move(req));
                break;
            }
            case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
            case RegistryMessageKind::CREATE_LOCATION:
            case RegistryMessageKind::RENAME_LOCATION:
            case RegistryMessageKind::REGISTER_SHARD:
            case RegistryMessageKind::REGISTER_CDC:
            case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
            case RegistryMessageKind::MOVE_SHARD_LEADER:
            case RegistryMessageKind::CLEAR_SHARD_INFO:
            case RegistryMessageKind::MOVE_CDC_LEADER:
            case RegistryMessageKind::CLEAR_CDC_INFO:
            case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
            case RegistryMessageKind::SET_BLOCK_SERVICE_HAS_FILES: {
                _writeRequests.emplace_back(std::move(req));
                break;
            }
            case RegistryMessageKind::ALL_REGISTRY_REPLICAS_DE_PR_EC_AT_ED:
            case RegistryMessageKind::ALL_REGISTRY_REPLICAS:
                ALWAYS_ASSERT(false, "ALL_REGISTRY_REPLICAS should be handled before bootstrap check");
            case RegistryMessageKind::EMPTY:
            case RegistryMessageKind::ERROR:
                auto& resp = _registryResponses.emplace_back();
                resp.requestId = req.requestId;
                break;
            }
        }
        receivedRequests.clear();


        auto writeReqBegin = _writer.pushRegistryRequests(_writeRequests.begin(), _writeRequests.end());
        for (;writeReqBegin != _writeRequests.end(); ++writeReqBegin) {
            auto& resp = _registryResponses.emplace_back();
            resp.requestId = writeReqBegin->requestId;
        }
        _writeRequests.clear();



        auto readReqBegin = _readRequests.begin();
        auto readReqEnd = _readRequests.end();
        for (size_t i = 0; i < _readers.size() && readReqBegin != readReqEnd; ++i) {
            auto& reader = *_readers[i];
            readReqBegin = reader.pushRequests(readReqBegin,readReqEnd);
        }
        for (;readReqBegin != readReqEnd; ++readReqBegin) {
            auto& resp = _registryResponses.emplace_back();
            resp.requestId = readReqBegin->requestId;
        }
        _readRequests.clear();

        _server.sendRegistryResponses(_registryResponses);
        _registryResponses.clear();
    }
private:
    const RegistryOptions& _options;
    Registerer& _registerer;
    RegistryServer& _server;
    LogsDB& _logsDB;
    RegistryDB& _registryDB;
    RegistryWriter& _writer;
    BlockServiceFileTracker& _tracker;

    std::array<bool, LogsDB::REPLICA_COUNT> _replicaFinishedBootstrap;
    bool _boostrapFinished;

    std::vector<RegistryReader*> _readers;
    std::vector<RegistryRequest> _readRequests;
    std::vector<RegistryRequest> _writeRequests;


    std::vector<RegistryResponse> _registryResponses;

    std::vector<std::pair<FailureDomain, TernTime>> _lastDecommissionTimes;


    void _processBootstrapRequest(RegistryRequest &req) {
        auto& resp = _registryResponses.emplace_back();
        resp.requestId = req.requestId;
        switch (req.req.kind()) {
        case RegistryMessageKind::REGISTER_REGISTRY: {
            const auto &regiReq = req.req.getRegisterRegistry();
            LOG_TRACE(_env, "Received register request in bootstrap %s", regiReq);
            if (regiReq.location != _options.logsDBOptions.location) {
                LOG_TRACE(_env, "Dropping register request in bootstrap for other location");
                break;
            }
            _registryDB.updateReplicaInfo(regiReq);
            _replicaFinishedBootstrap[regiReq.replicaId.u8] = !regiReq.bootstrap;
            resp.resp.setRegisterRegistry();
            size_t quorumSize = _replicaFinishedBootstrap.size() / 2 + 1;
            if (_options.logsDBOptions.noReplication) {
                quorumSize = 1;
            }

            _boostrapFinished = std::count(
                std::begin(_replicaFinishedBootstrap), std::end(_replicaFinishedBootstrap), true) >= quorumSize;
            if (_boostrapFinished) {
                LOG_TRACE(_env, "Bootstrap finished!");
                _server.setReplicas(_registerer.replicas());
            }
            break;
        }
        default:
            // empty response removes client
            break;
        }
    }
};


Registry::Registry(Logger &logger, std::shared_ptr<XmonAgent> xmon)
    : _env(logger, xmon, "registry") {

      };

Registry::~Registry() {}

void Registry::start(const RegistryOptions& options, LoopThreads& threads) {
    {
        LOG_INFO(_env, "Running registry %s at location %s, base directory %s:", options.logsDBOptions.replicaId, (int)options.logsDBOptions.location, options.logsDBOptions.dbDir);
        LOG_INFO(_env, "  Logging options:");
        LOG_INFO(_env, "    logFile = '%s'", options.logOptions.logFile);
        LOG_INFO(_env, "    xmon = %s", options.xmonOptions.addr);
        LOG_INFO(_env, "    metrics = '%s'", (int)!options.metricsOptions.origin.empty());
        LOG_INFO(_env, "  LogsDB options:");
        LOG_INFO(_env, "    noReplication = '%s'", (int)options.logsDBOptions.noReplication);
        LOG_INFO(_env, "    avoidBeingLeader = '%s'", (int)options.logsDBOptions.avoidBeingLeader);
        LOG_INFO(_env, "  Registry options:");
        LOG_INFO(_env, "    registryHost = '%s'", options.registryClientOptions.host);
        LOG_INFO(_env, "    registryPort = %s", options.registryClientOptions.port);
        LOG_INFO(_env, "    ownAddres = %s", options.serverOptions.addrs);
        LOG_INFO(_env, "    enforceStableIp = '%s'", (int)options.enforceStableIp);
        LOG_INFO(_env, "    enforceStableLeader = '%s'", (int)options.enforceStableLeader);
        LOG_INFO(_env, "    maxConnections = '%s'", options.maxConnections);
        LOG_INFO(_env, "    minAutoDecomInterval = '%s'", options.minDecomInterval);
        LOG_INFO(_env, "    alertAtUnavailableFailureDomains = '%s'", (int)options.alertAfterUnavailableFailureDomains);
        LOG_INFO(_env, "    stalenessDelay = '%s'", options.staleDelay);
        LOG_INFO(_env, "    blockServiceUseDelay = '%s'", options.blockServiceUsageDelay);
        LOG_INFO(_env, "    maxWritableBlockServicePerShard = '%s'", options.maxFailureDomainsPerShard);
        LOG_INFO(_env, "    writableBlockServiceUpdateInterval = '%s'", options.writableBlockServiceUpdateInterval);
    }

    _state = std::make_unique<RegistryState>();
    std::string dbDir = options.logsDBOptions.dbDir;
    XmonNCAlert dbInitAlert{1_mins};
    _env.updateAlert(dbInitAlert, "initializing database %s", options.xmonOptions.appInstance);
    auto xmon = _env.xmon();
    auto& logger = _env.logger();
    _state->sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
    _state->sharedDB->registerCFDescriptors(LogsDB::getColumnFamilyDescriptors());
    _state->sharedDB->registerCFDescriptors(RegistryDB::getColumnFamilyDescriptors());

    rocksdb::Options rocksDBOptions;
    rocksDBOptions.create_if_missing = true;
    rocksDBOptions.create_missing_column_families = true;
    rocksDBOptions.compression = rocksdb::kLZ4Compression;
    rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
    rocksDBOptions.max_open_files = 1000;
    // We batch writes and flush manually.
    rocksDBOptions.manual_wal_flush = true;
    _state->sharedDB->open(rocksDBOptions);
    _state->registryDB = std::make_unique<RegistryDB>(logger, xmon, options, *_state->sharedDB);
    _state->logsDB = std::make_unique<LogsDB>(
        logger, xmon, *_state->sharedDB, options.logsDBOptions.replicaId,
        _state->registryDB->lastAppliedLogEntry(), options.logsDBOptions.noReplication,
        !options.logsDBOptions.leaderElection,
        options.logsDBOptions.avoidBeingLeader);
    _env.clearAlert(dbInitAlert);

    _state->server.reset(new RegistryServer(options, _env));
    ALWAYS_ASSERT(_state->server->init());
    std::vector<FullRegistryInfo> cachedRegistries;
    _state->registryDB->registries(cachedRegistries);

    std::vector<RegistryReader*> readers;
    for(uint8_t i = 0; i < options.numReaders; ++i) {
        auto reader = std::make_unique<RegistryReader>(logger, xmon, options, *_state->registryDB, *_state->server);
        readers.emplace_back(reader.get());
        threads.emplace_back(LoopThread::Spawn(std::move(reader)));
    }

    auto writer = std::make_unique<RegistryWriter>(
        logger, xmon, *_state->registryDB, *_state->server, *_state->logsDB);

    auto registerer = std::make_unique<Registerer>(
        logger, xmon, options, _state->server->boundAddresses(), cachedRegistries);

    auto tracker = std::make_unique<BlockServiceFileTracker>(
        logger, xmon, options, *_state->registryDB, *_state->server, *writer);

    auto registryLoop = std::make_unique<RegistryLoop>(
        logger, xmon, options, *registerer, *_state->server,
        *_state->logsDB, *_state->registryDB, std::move(readers), *writer, *tracker);

    threads.emplace_back(LoopThread::Spawn(std::move(registerer)));
    threads.emplace_back(LoopThread::Spawn(std::move(writer)));
    threads.emplace_back(LoopThread::Spawn(std::move(tracker)));
    threads.emplace_back(LoopThread::Spawn(std::move(registryLoop)));
}

void Registry::close() {
  if (_state) {
    if (_state->registryDB) {
      _state->registryDB->close();
      _state->registryDB.reset();
    }
    if (_state->logsDB) {
      _state->logsDB->close();
      _state->logsDB.reset();
    }
    if (_state->sharedDB) {
      _state->sharedDB->close();
      _state->sharedDB.reset();
    }

    LOG_INFO(_env, "registry terminating gracefully, bye.");
  }
  _state.reset();
}
