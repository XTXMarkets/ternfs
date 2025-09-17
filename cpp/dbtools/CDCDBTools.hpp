// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once

#include "Msgs.hpp"
#include <limits>
#include <string>

class CDCDBTools {
public:
    static void outputLogEntries(const std::string& dbPath, LogIdx startIdx = 0, size_t count = std::numeric_limits<size_t>::max());
};
