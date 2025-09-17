// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "Time.hpp"

struct MetricsBuilder {
private:
    enum State : uint8_t {
        INIT,
        MEASUREMENT,
        TAGS,
        FIELDS,
        TIMESTAMP
    };

    State _state = State::INIT;
    std::string _payload;

public:
    void reset() {
        _state = State::INIT;
        _payload.clear();
    }

    const std::string& payload() const {
        ALWAYS_ASSERT(_state == State::INIT || _state == State::TIMESTAMP);
        return _payload;
    }

    void measurement(const std::string& name);

    void tagRaw(const std::string& name, const std::string& value);

    template<typename A>
    void tag(const std::string& name, const A& value) {
        std::ostringstream ss;
        ss << value;
        tagRaw(name, ss.str());
    }

    void fieldRaw(const std::string& name, const std::string& value);

    void fieldU64(const std::string& name, uint64_t x) {
        std::ostringstream ss;
        ss << x << "i";
        fieldRaw(name, ss.str());
    }

    void fieldFloat(const std::string& name, double x) {
        std::ostringstream ss;
        ss << x;
        fieldRaw(name, ss.str());
    }

    void timestamp(TernTime t);
};

struct InfluxDB {
    std::string origin;
    std::string org;
    std::string bucket;
};

// error string on error
std::string sendMetrics(const InfluxDB& idb, Duration timeout, const std::string& payload);