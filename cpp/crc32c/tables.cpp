// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <stdio.h>
#include <stdlib.h>

#include "iscsi.hpp"

int main() {
    uint32_t crc_power_table[31];
    printf("static uint32_t CRC_POWER_TABLE[31] = {");
    uint32_t p = 1u << 30; // start with x^1
    for (int i = 0; i < 31; i++) {
        if (i % 5 == 0) {
            printf("\n    ");
        }
        crc_power_table[i] = p;
        printf("0x%08x, ", p);
        p = crc32c_mult_mod_p(p, p);
    }
    printf("\n};\n\n");

    // find x^-1
    for (uint64_t x = 1; x < (1ull<<32); x++) {
        if (crc32c_mult_mod_p(x, crc_power_table[0]) == (1u<<31)) {
            p = (uint32_t)x;
            break;
        }
    }

    printf("static uint32_t CRC_INVERSE_POWER_TABLE[31] = {");
    for (int i = 0; i < 31; i++) {
        if (i % 5 == 0) {
            printf("\n    ");
        }
        if (crc32c_mult_mod_p(p, crc_power_table[i]) != (1u<<31)) {
            fprintf(stderr, "not an inverse!\n");
            exit(1);
        }
        printf("0x%08x, ", p);
        p = crc32c_mult_mod_p(p, p);
    }
    printf("\n};\n\n");
}
