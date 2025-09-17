// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <stdio.h>
#include <string.h>
#include <vector>
#include <immintrin.h>

#include "crc32c.h"
#include "Random.hpp"

#define ASSERT(expr) do { \
        if (!(expr)) { \
            fprintf(stderr, "Assertion failed: " #expr "\n"); \
            exit(1); \
        } \
    } while (false)

int main() {
    uint32_t expectedCrc = 0x6c0ec068u;
    const char* str = "bazzer\n";
    ASSERT(expectedCrc == crc32c(0, str, strlen(str)));

    RandomGenerator rand(0);

    const auto randString = [&rand](size_t l) -> std::vector<char> {
        std::vector<char> s(l, 0);
        rand.generateBytes(s.data(), s.size());
        return s;
    };

    // Test append
    for (int i = 0; i < 1000; i++) {
        auto s1 = randString(1 + rand.generate64()%100);
        uint32_t crc1 = crc32c(0, s1.data(), s1.size());
        ASSERT(crc1 == crc32c_append(0, crc1, s1.size()));
        auto s2 = randString(1 + rand.generate64()%100);
        uint32_t crc2 = crc32c(0, s2.data(), s2.size());
        std::vector<char> s = s1;
        s.insert(s.end(), s2.begin(), s2.end());
        uint32_t crc = crc32c(0, s.data(), s.size());
        ASSERT(crc == crc32c_append(crc1, crc2, s2.size()));
    }

    // Test XOR
    for (int i = 0; i < 1000; i++) {
        size_t l = 1 + rand.generate64()%100;
        auto s1 = randString(l);
        uint32_t crc1 = crc32c(0, s1.data(), s1.size());
        auto s2 = randString(l);
        uint32_t crc2 = crc32c(0, s2.data(), s2.size());
        std::vector<char> s = s1;
        for (int i = 0; i < l; i++) {
            s[i] ^= s2[i];
        }
        uint32_t crc = crc32c(0, s.data(), s.size());
        ASSERT(crc == crc32c_xor(crc1, crc2, l));
    }

    // Test zero extend
    for (int i = 0; i < 100; i++) {
        size_t l = 1 + rand.generate64()%100;
        size_t lzeros = rand.generate64()%100;
        auto s = randString(l);
        std::vector<char> szeros(l + lzeros, 0);
        memcpy(&szeros[0], s.data(), l);
        uint32_t crc = crc32c(0, s.data(), l);
        ASSERT(
            crc32c_zero_extend(crc, lzeros) ==
            crc32c(0, szeros.data(), szeros.size())
        );
    }

    // Test zero contract
    for (int i = 0; i < 100; i++) {
        size_t l = 1 + rand.generate64()%100;
        ssize_t lzeros = rand.generate64()%100;
        auto s = randString(l);
        std::vector<char> szeros(l + lzeros, 0);
        memcpy(&szeros[0], s.data(), l);
        uint32_t crc = crc32c(0, szeros.data(), szeros.size());
        ASSERT(
            crc32c_zero_extend(crc, -lzeros) ==
            crc32c(0, s.data(), s.size())
        );
    }

    for (int i = 0; i < 100; i++) {
        size_t l = 1 + rand.generate64()%100;
        auto s = randString(l);
        ASSERT(
            crc32c(0, s.data(), s.size()) ==
            crc32c(crc32c(0, s.data(), s.size()), "", 0)
        );
    }

    return 0;
}
