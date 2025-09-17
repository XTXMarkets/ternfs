// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "CommonOptions.hpp"

#include <unordered_map>

static const std::unordered_map<std::string, uint64_t> durationUnitMap = {
	{"ns", 1ull},
	{"us", 1'000ull},
	{"µs", 1'000ull},  // U+00B5 = micro symbol
	{"μs", 1'000ull},  // U+03BC = Greek letter mu
	{"ms", 1'000'000ull},
	{"s",  1'000'000'000ull},
	{"m",  1'000'000'000ull*60},
	{"h",  1'000'000'000ull*60*60},
};

Duration parseDuration(CommandLineArgs& args) {
    size_t idx;
    auto arg = args.getArg();
    uint64_t x = std::stoull(arg, &idx);
	auto unit = arg.substr(idx);
	if (!durationUnitMap.contains(unit)) {
		fprintf(stderr, "Invalid duration unit: %s\n", unit.c_str());
		args.dieWithUsage();
	}
    return x * durationUnitMap.at(arg.substr(idx));
}
