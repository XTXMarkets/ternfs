// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <cstdint>

#include "Common.hpp"

struct AES128Key {
    ALIGNED(16) uint8_t key[16*11];
};

void generateSecretKey(std::array<uint8_t, 16>& userKey);

void expandKey(const std::array<uint8_t, 16>& userKey, AES128Key& key);

// generates an AES-128 CBC MAC
std::array<uint8_t, 8> cbcmac(const AES128Key& key, const uint8_t* data, size_t len);
