// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include "Msgs.hpp"
#include <cstddef>
#include <limits>
#include <string>

class ShardDBTools {
public:
    static void verifyEqual(const std::string& db1Path, const std::string& db2Path);
    static void outputUnreleasedState(const std::string& dbPath);
    static void fsck(const std::string& dbPath);
    static void outputLogEntries(const std::string& dbPath, LogIdx startIdx = 0, size_t count = std::numeric_limits<size_t>::max());
    static void sampleFiles(const std::string& dbPath, const std::string& outputFilePath);
    static void outputFilesWithDuplicateFailureDomains(const std::string& dbPath, const std::string& outputFilePath);
    static void outputBlockServiceUsage(const std::string& dbPath, const std::string& outputFilePath);
};
