// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <stdint.h>
#include <atomic>
#include <math.h>

#include "Exception.hpp"
#include "Time.hpp"
#include "MsgsGen.hpp"
#include "Metrics.hpp"

struct Timings {
    // static stuff
    double _growth;
    double _invLogGrowth;
    uint64_t _firstUpperBound;
    double _growthDivUpperBound;

    // actual data
    TernTime _startedAt;
    std::vector<std::atomic<uint64_t>> _bins;
public:
    Timings(Duration firstUpperBound, double growth, int bins);
    Timings() = default;

    static Timings Standard() {
        // 100ns to ~20mins, 10% error
        return Timings(100_ns, 1.2, 128);
    }

    void add(Duration d) {
        int64_t inanos = d.ns;
        if (unlikely(inanos <= 0)) { return; }
        uint64_t nanos = inanos;
        int bin = std::min<int>(_bins.size()-1, std::max<int>(0, log((double)nanos * _growthDivUpperBound) * _invLogGrowth));
        _bins[bin]++;
    }

    uint64_t count() const {
        uint64_t total = 0;
        for (const auto& bin : _bins) {
            total += bin.load();
        }
        return total;
    }

    Duration mean() const;
    Duration percentile(double p) const;

    // void toMetrics(MetricsBuilder& builder, const std::string& name, const std::vector<std::pair<std::string, std::string>>& tags);
    void reset();
};
