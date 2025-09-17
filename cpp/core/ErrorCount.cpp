// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "ErrorCount.hpp"

void ErrorCount::reset() {
    for (int i = 0; i < count.size(); i++) {
        count[i].store(0);
    }
}
