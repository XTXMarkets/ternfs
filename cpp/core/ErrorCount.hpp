// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <atomic>
#include <cstdint>
#include <vector>

#include "MsgsGen.hpp"
#include "Metrics.hpp"

struct ErrorCount {
    std::vector<std::atomic<uint64_t>> count;

    ErrorCount() : count(maxTernError) {
        for (int i = 0; i < count.size(); i++) {
            count[i].store(0);
        }
    }

    void add(TernError err) {
        count[(int)err]++;
    }

    // void toMetrics(const MetricsBuilder& builder); // will add tags and then fields
    void reset();
};
