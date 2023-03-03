#pragma once

#include <stdint.h>

constexpr uint32_t ISCSI_POLY = 0x82F63B78u;

// Return a(x) multiplied by b(x) modulo ISCSI_POLY, For speed, this requires
// that a(x) not be zero.
static uint32_t crc32c_mult_mod_p(uint32_t a, uint32_t b) {
    // m goes from x^0 to x^31
    uint32_t m = 1u << 31;
    uint32_t p = 0;
    for (;;) {
        // If a(x) contains x^n, add b(x)*x^n
        if (a & m) {
            p ^= b;
            // Exit when there are no higher bits in a(x)
            if ((a & (m - 1)) == 0) {
                break;
            }
        }
        // Go from x^n to x^(n+1)
        m >>= 1;
        // Go from b(x)*x^n to b(x)*x^(n+1)
        b = (b & 1) ? ((b >> 1) ^ ISCSI_POLY) : b >> 1;
    }
    return p;
}
