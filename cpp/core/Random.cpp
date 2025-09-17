// Copyright 2023 Peter Cawley <corsix@corsix.org>
// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "Random.hpp"
#include "Exception.hpp"

#include <unistd.h>
#include <cstring>

RandomGenerator::RandomGenerator() {
   int ret = getentropy(_state, 16);
   if (ret != 0) {
       throw SYSCALL_EXCEPTION("getentropy");
   }
}

// this will only return zero if the input is zero
// (it's an invertible function)
#ifdef __clang__
__attribute__((no_sanitize("integer")))
#endif
static inline uint64_t mix(uint64_t x) {
    // this is the 'moremur' 64-bit xorshift-mul-xorshift hash
    // it's similar to splitmix64
    x ^= x >> 27;
    x *= 0x3c79ac492ba7b653ul;
    x ^= x >> 33;
    x *= 0x1c69b3f74ac4ae35ul;
    x ^= x >> 27;
    return x;
}

#ifdef __clang__
__attribute__((no_sanitize("integer")))
#endif
RandomGenerator::RandomGenerator(uint64_t seed) {
    // we can never end up with a zero state, because the inputs to mix cannot both be zero
    seed += 0x9e3779b97f4a7c15ul;
    _state[0] = mix(seed);
    seed += 0x9e3779b97f4a7c15ul;
    _state[1] = mix(seed);
}

#ifdef __clang__
__attribute__((no_sanitize("integer")))
#endif
uint64_t RandomGenerator::generate64() {
    // Lehmer generator, but inside out to increase ILP
    uint64_t lo = _state[0];
    uint64_t hi = _state[1];
    __uint128_t tmp = (__uint128_t)hi << 64 | lo;
    tmp *= 0xda942042e4dd58b5ul;
    _state[0] = tmp;
    _state[1] = tmp >> 64;
    return hi;
}

#ifdef __clang__
__attribute__((no_sanitize("integer")))
#endif
double RandomGenerator::generateDouble() {
    // taken from corsix.org
    double d;
    uint64_t x = generate64();
    uint64_t e = __builtin_ctzll(x) - 11ul;
    if ((int64_t)e >= 0) e = __builtin_ctzll(generate64());
    x = (((x >> 11) + 1) >> 1) - ((e - 1011ul) << 52);
    memcpy(&d, &x, 8);
    return d;
}

void RandomGenerator::generateBytes(char *buf, size_t len) {
    while (len >= 8) {
        uint64_t x = generate64();
        memcpy(buf, (char*)&x, 8);
        len -= 8;
        buf += 8;
    }
    uint64_t x = generate64();
    if (len & 4) {
        memcpy(buf, (char*)&x + 4, 4);
        buf += 4;
    }
    if (len & 2) {
        memcpy(buf, (char*)&x + 2, 2);
        buf += 2;
    }
    if (len & 1) {
        memcpy(buf, (char*)&x, 1);
        buf += 1;
    }
    return;
}

