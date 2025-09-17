// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "Timings.hpp"

Timings::Timings(Duration firstUpperBound, double growth, int bins) :
    _growth(growth),
    _invLogGrowth(1.0/log(growth)),
    _firstUpperBound(firstUpperBound.ns),
    _growthDivUpperBound(growth / (double)firstUpperBound.ns),
    _startedAt(ternNow()),
    _bins(bins)
{
    if (firstUpperBound < 1) {
        throw TERN_EXCEPTION("non-positive first upper bound %s", firstUpperBound);
    }
    if (growth <= 1) {
        throw TERN_EXCEPTION("growth %s <= 1.0", growth);
    }
    for (auto& bin: _bins) {
        bin.store(0);
    }
}

void Timings::reset() {
    _startedAt = ternNow();
    for (auto& bin : _bins) {
        bin.store(0);
    }
}

Duration Timings::mean() const {
    double m = 0;
    double c = count();
    double upperBound = _firstUpperBound;
    for (const auto& bin : _bins) {
        m += (double)bin.load() * (upperBound / c);
        upperBound *= _growth;
    }
    return Duration(m);
}

Duration Timings::percentile(double p) const {
    uint64_t countSoFar = 0;
    uint64_t toReach = count()*p;
    double upperBound = _firstUpperBound;
    for (const auto& bin: _bins) {
        countSoFar += bin.load();
        if (countSoFar >= toReach) { break; }
        upperBound *= _growth;
    }
    return Duration(upperBound);
}

#if 0
void Timings::toMetrics(MetricsBuilder& builder, const std::string& name, const std::vector<std::pair<std::string, std::string>>& tags) {
    uint64_t sum = 0;
    {
        double upperBound = _firstUpperBound;
        for (int i = 0; i < _bins.size(); i++) {
            sum += _bins[i].load();
            builder.measurement(name + "_bucket");
            for (const auto& tag: tags) {
                builder.tag(tag.first, tag.second);
            }
            builder.tag("le", std::to_string((uint64_t)upperBound));
            builder.fieldU64("count", sum);
            upperBound *= _growth;
        }
    }
    builder.measurement(name + "_count");
    for (const auto& tag: tags) {
        builder.tag(tag.first, tag.second);
    }
    builder.fieldU64("count", sum);
}
#endif
