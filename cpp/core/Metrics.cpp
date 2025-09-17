// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// See <https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/>
// for docs.
#include "Metrics.hpp"

#include "Assert.hpp"
#define CPPHTTPLIB_USE_POLL
#include "httplib.h"

// We're being overly conservative here, see
// <https://docs.influxdata.com/influxdb/v2/reference/syntax/line-protocol/#special-characters>,
// but better being safe than sorry.
static void validMetricsName(const std::string& name) {
    for (auto ch : name) {
        ALWAYS_ASSERT(
            (ch >= 'a' && ch <= 'z') ||
            (ch >= 'A' && ch <= 'Z') ||
            (ch >= '0' && ch <= '9') ||
            ch == '-' || ch == '_' || ch == '.' || ch == ':'
        );
    }
}

void MetricsBuilder::measurement(const std::string& name) {
    ALWAYS_ASSERT(_state == State::INIT || _state == State::TIMESTAMP);
    validMetricsName(name);
    if (_state == State::TIMESTAMP) {
        _payload += '\n';
    }
    _payload += name;
    _state = State::MEASUREMENT;
}

void MetricsBuilder::tagRaw(const std::string& name, const std::string& value) {
    ALWAYS_ASSERT(_state == State::MEASUREMENT || _state == State::TAGS);
    validMetricsName(name);
    validMetricsName(value);
    _payload += ',';
    _payload += name;
    _payload += '=';
    _payload += value;
    _state = State::TAGS;
}

void MetricsBuilder::fieldRaw(const std::string& name, const std::string& value) {
    ALWAYS_ASSERT(_state == State::MEASUREMENT || _state == State::TAGS || _state == State::FIELDS);
    validMetricsName(name);
    if (_state == State::MEASUREMENT || _state == State::TAGS) {
        _payload += ' ';
    } else {
        _payload += ',';
    }
    _payload += name;
    _payload += '=';
    _payload += value;
    _state = State::FIELDS;
}

void MetricsBuilder::timestamp(TernTime t) {
    ALWAYS_ASSERT(_state == State::FIELDS);
    _payload += ' ';
    static char buf[21];
    std::sprintf(buf, "%lu", t.ns);
    _payload += buf;
    _state = State::TIMESTAMP;
}

std::string sendMetrics(const InfluxDB& idb, Duration timeout, const std::string& payload) {
    httplib::Client cli(idb.origin);
    time_t ds = timeout.ns / 1'000'000'000ull;
    time_t dus = (timeout.ns % 1'000'000'000ull) / 1'000ull;
    cli.set_connection_timeout(ds, dus);
    cli.set_read_timeout(ds, dus);
    cli.set_write_timeout(ds, dus);

    std::string path = "/api/v2/write?org=" + idb.org + "&bucket=" + idb.bucket + "&precision=ns";
    const auto res = cli.Post(path, payload, "text/plain");
    if (res.error() != httplib::Error::Success) {
        std::ostringstream ss;
        ss << "Could not insert metrics to " << idb.origin << path << ": " << res.error();
        return ss.str();
    }
    return {};
}
