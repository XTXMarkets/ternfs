// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#define ISCSI_POLY 0x82F63B78u

// Return a(x) multiplied by b(x) modulo ISCSI_POLY, For speed, this requires
// that a(x) not be zero.
static u32 crc32c_mult_mod_p(u32 a, u32 b) {
    // m goes from x^0 to x^31
    u32 m = 1u << 31;
    u32 p = 0;
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
