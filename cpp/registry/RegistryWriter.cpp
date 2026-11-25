// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "RegistryWriter.hpp"
#include "Assert.hpp"
#include "LogsDB.hpp"
#include "Msgs.hpp"
#include "RegistryDB.hpp"
#include "RegistryDBLogEntry.hpp"
#include "RegistryServer.hpp"
#include <unistd.h>

const int REGISTRY_WRITER_QUEUE_SIZE = 16384;
const int MAX_WORK_ITEMS_AT_ONCE = 512;

RegistryWriter::RegistryWriter(Logger &logger, std::shared_ptr<XmonAgent> xmon, RegistryDB& registryDB,RegistryServer& server, LogsDB& logsDB) :
    Loop(logger, xmon, "registry_writer"),
    _registryDB(registryDB),
    _server(server),
    _logsDB(logsDB),
    _logsDBRequestsQueue(REGISTRY_WRITER_QUEUE_SIZE, _waiter),
    _logsDBResponsesQueue(REGISTRY_WRITER_QUEUE_SIZE, _waiter),
    _registryRequestsQueue(REGISTRY_WRITER_QUEUE_SIZE, _waiter),
    _blockServiceStateRequestsQueue(REGISTRY_WRITER_QUEUE_SIZE, _waiter)
{

}

void RegistryWriter::sendStop() {
    _registryRequestsQueue.close();
    _logsDBRequestsQueue.close();
    _logsDBResponsesQueue.close();
}

void RegistryWriter::step() {
    // Clear all buffers
    _registryRequests.clear();
    _registryResponses.clear();
    _logsDBOutRequests.clear();
    _logsDBOutResponses.clear();
    _writeResults.clear();
    _logEntries.clear();

    _waiter.wait(_logsDB.getNextTimeout());

    if (unlikely(_registryRequestsQueue.isClosed())) {

        stop();
        return;
    }

    auto remainingPullBudget = MAX_WORK_ITEMS_AT_ONCE;

    remainingPullBudget -= _logsDBResponsesQueue.pull(_logsDBResponses, remainingPullBudget);
    remainingPullBudget -= _logsDBRequestsQueue.pull(_logsDBRequests, remainingPullBudget);
    remainingPullBudget -= _blockServiceStateRequestsQueue.pull(_registryRequests, remainingPullBudget);
    remainingPullBudget -= _registryRequestsQueue.pull(_registryRequests, remainingPullBudget);

    _logsDB.processIncomingMessages(_logsDBRequests, _logsDBResponses);

    // Append entries to logs DB
    if (_logsDB.isLeader()) {
        RegistryDBLogEntry::requestsToLogEntries(
            _registryRequests, LogsDB::DEFAULT_UDP_ENTRY_SIZE, ternNow(), _logEntries, _entriesRequestIds);


        _logsDB.appendEntries(_logEntries);
        size_t requestIdx = 0;
        for (int i = 0; i < _logEntries.size(); ++i) {
            auto &entry = _logEntries[i];
            while (requestIdx < _entriesRequestIds.size() && _entriesRequestIds[requestIdx].logIdx == i) {
                auto entryIdx = _entriesRequestIds[requestIdx];
                auto requestId = _registryRequests[requestIdx].requestId;
                ++requestIdx;
                if (requestId == 0) {
                    continue;
                }
                if (entry.idx == 0) {
                    LOG_DEBUG(_env, "could not append entry for request %s", requestId);
                    // empty response drops request
                    auto& resp = _registryResponses.emplace_back();
                    resp.requestId = requestId;
                } else {
                    entryIdx.logIdx = entry.idx;
                    _logIdxToRequestId.emplace(entryIdx, requestId);
                }
            }
        }
        _logEntries.clear();
        _entriesRequestIds.clear();
    } else {
        for (auto &req : _registryRequests) {
            auto& resp = _registryResponses.emplace_back();
            resp.requestId = req.requestId;
        }
    }

    // Apply log entries
    _applyLogEntries();

    _logsDB.getOutgoingMessages(_logsDBOutRequests, _logsDBOutResponses);
    _server.sendLogsDBMessages(_logsDBOutRequests, _logsDBOutResponses);

    // Send responses
    _server.sendRegistryResponses(_registryResponses);
    _registryResponses.clear();
}

void RegistryWriter::_applyLogEntries() {
    do {
        _logEntries.clear();
        _logsDB.readEntries(_logEntries);
        _registryDB.processLogEntries(_logEntries, _writeResults);
    } while (!_logEntries.empty());

    _logsDB.flush(true);

    // Convert write results to registry responses
    for (auto &writeResult : _writeResults) {
        auto requestIdIt = _logIdxToRequestId.find(writeResult.idx);
        if (requestIdIt == _logIdxToRequestId.end()) {
            continue;
        }
        auto& regiResp = _registryResponses.emplace_back();

        regiResp.requestId = requestIdIt->second;
        RegistryRespContainer& resp = regiResp.resp;

        if (writeResult.err != TernError::NO_ERROR) {
            resp.setError() = writeResult.err;
            continue;
        }

        switch (writeResult.kind) {
        case RegistryMessageKind::ERROR:
            resp.setError() = writeResult.err;
            break;
        case RegistryMessageKind::CREATE_LOCATION:
            resp.setCreateLocation();
            break;
        case RegistryMessageKind::RENAME_LOCATION:
            resp.setRenameLocation();
            break;
        case RegistryMessageKind::REGISTER_SHARD:
            resp.setRegisterShard();
            break;
        case RegistryMessageKind::REGISTER_CDC:
            resp.setRegisterCdc();
            break;
        case RegistryMessageKind::SET_BLOCK_SERVICE_FLAGS:
            resp.setSetBlockServiceFlags();
            break;
        case RegistryMessageKind::REGISTER_BLOCK_SERVICES:
            resp.setRegisterBlockServices();
            break;
        case RegistryMessageKind::REGISTER_REGISTRY:
            resp.setRegisterRegistry();
            break;
        case RegistryMessageKind::DECOMMISSION_BLOCK_SERVICE:
            resp.setDecommissionBlockService();
            break;
        case RegistryMessageKind::MOVE_SHARD_LEADER:
            resp.setMoveShardLeader();
            break;
        case RegistryMessageKind::CLEAR_SHARD_INFO:
            resp.setClearShardInfo();
            break;
        case RegistryMessageKind::MOVE_CDC_LEADER:
            resp.setMoveCdcLeader();
            break;
        case RegistryMessageKind::CLEAR_CDC_INFO:
            resp.setClearCdcInfo();
            break;
        case RegistryMessageKind::UPDATE_BLOCK_SERVICE_PATH:
            resp.setUpdateBlockServicePath();
            break;
        default:
            ALWAYS_ASSERT(false);
            break;
        }
    }
    _writeResults.clear();
}
