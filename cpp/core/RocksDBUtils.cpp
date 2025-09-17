// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <rocksdb/env.h>
#include <rocksdb/merge_operator.h>
#include <rocksdb/slice.h>
#include <rocksdb/statistics.h>

#include "RocksDBUtils.hpp"

namespace {

// A merge operator with int64 addition semantics
class Int64AddOperator : public rocksdb::AssociativeMergeOperator {
public:
    bool Merge(
        const rocksdb::Slice& key, const rocksdb::Slice* existing_value,
        const rocksdb::Slice& value, std::string* new_value,
        rocksdb::Logger* logger
    ) const override {
        int64_t orig_value = 0;
        if (existing_value) {
            orig_value = ExternalValue<I64Value>::FromSlice(*existing_value)().i64();
        }
        int64_t operand = ExternalValue<I64Value>::FromSlice(value)().i64();

        ALWAYS_ASSERT(new_value);
        new_value->resize(I64Value::MAX_SIZE);
        ExternalValue<I64Value>(*new_value)().setI64(orig_value + operand);

        return true;
    }

    const char* Name() const override { return "Int64AddOperator"; }
    const char* NickName() const override { return "int64add"; }
};

}

std::shared_ptr<rocksdb::MergeOperator> CreateInt64AddOperator() {
    return std::make_shared<Int64AddOperator>();
}

// These are low hanging fruits (int stats that are probably useful), eventually
// I want to parse level info, histograms, etc.
static const std::vector<std::pair<const std::string&, std::string>> rocksDBIntStats = {
    {rocksdb::DB::Properties::kBlockCacheCapacity, "block_cache_capacity"},
    {rocksdb::DB::Properties::kBlockCacheUsage, "block_cache_usage"},
    {rocksdb::DB::Properties::kBlockCachePinnedUsage, "block_cache_pinned_usage"},
    {rocksdb::DB::Properties::kBackgroundErrors, "background_errors"},
    {rocksdb::DB::Properties::kCurSizeAllMemTables, "cur_size_all_mem_tables"},
    {rocksdb::DB::Properties::kSizeAllMemTables, "size_all_mem_tables"},
    {rocksdb::DB::Properties::kEstimateNumKeys, "estimate_num_keys"},
    {rocksdb::DB::Properties::kEstimateTableReadersMem, "estimate_table_readers_mem"},
    {rocksdb::DB::Properties::kEstimateLiveDataSize, "estimate_live_data_size"},
    {rocksdb::DB::Properties::kEstimatePendingCompactionBytes, "estimate_pending_compaction_bytes"},
};

void rocksDBMetrics(Env& env, rocksdb::DB* db, const rocksdb::Statistics& statistics, std::unordered_map<std::string, uint64_t>& stats) {
    // properties
    for (const auto& [prop, name]: rocksDBIntStats) {
        uint64_t v;
        ALWAYS_ASSERT(db->GetIntProperty(prop, &v));
    }
    // statistics
    std::map<std::string, uint64_t> tickers;
    ALWAYS_ASSERT(statistics.getTickerMap(&tickers));
    for (const auto& [name, value]: tickers) {
        std::string prefix = "rocksdb.";
        ALWAYS_ASSERT(name.rfind(prefix, 0) == 0);
        std::string metric_name = name;
        metric_name.erase(0, prefix.length());
        std::replace(metric_name.begin(), metric_name.end(), '.', '_');
        stats.emplace(std::make_pair(metric_name, value));
    }
}