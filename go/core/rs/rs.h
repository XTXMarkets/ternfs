// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Reed-Solomon implementation in the style of
// <https://www.corsix.org/content/reed-solomon-for-software-raid>
//
// Specifically:
//
// * We use the GF(2^8) as our word -- using the GNFI/Rijndael
//     polynomial x^8 + x^4 + x^3 + x + 1.
// * We use a matrix so that the encoding is systematic -- the data
//     blocks are preserved. Or in other words, the first part of
//     the encoding matrix is the identity matrix.
// * We rescale the encoding matrix so that the first parity block
//     is the XOR of the data blocks, in line with RAID5.
//
// In the comments below, we use D to mean the number of Data blocks,
// P to mean the number of Parity blocks, and B for the total number
// of Blocks.
//
// We also store the number of parity/data blocks in the two nibbles
// of a uint8_t, but this is a fairly irrelevant quirk of TernFS,
// although it does nicely enforce that we do not go beyond what's
// resonable for data storage purposes (rather than for error correction).
#ifndef TERN_RS
#define TERN_RS

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

struct rs;

inline uint8_t rs_data_blocks(uint8_t parity) {
    return parity & 0x0F;
}

inline uint8_t rs_parity_blocks(uint8_t parity) {
    return parity >> 4;
}

inline uint8_t rs_blocks(uint8_t parity) {
    return rs_data_blocks(parity) + rs_parity_blocks(parity);
}

inline uint8_t rs_mk_parity(uint8_t data, uint8_t parity) {
    return (parity << 4) | data;
}

enum rs_cpu_level {
    RS_CPU_SCALAR = 1,
    RS_CPU_AVX2 = 2,
    RS_CPU_GFNI = 3,
};

bool rs_has_cpu_level(enum rs_cpu_level level);
void rs_set_cpu_level(enum rs_cpu_level cpu_level);
enum rs_cpu_level rs_get_cpu_level();

// Will crash (SIGABRT) if:
//
// * If `D < 2` (just use mirroring);
// * If `P < 1` (can't do RS with no parity blocks);
//
// There's no `rs_delete` -- once allocated the objects are kept around
// forever, since they are immutable.
struct rs* rs_get(uint8_t parity);

uint8_t rs_parity(struct rs* rs);

// Computes all parity blocks at once. This is what you use when you store
// something for the first time.
void rs_compute_parity(
    struct rs* rs,
    uint64_t size,
    const uint8_t** data, // input, uint8_t[D][size]
    uint8_t** parity      // output, uint8_t[P][size]
);

// Computes an arbitrary block given at least `D` other blocks.
// This is what you use to recover a lost block.
void rs_recover(
    struct rs* rs,
    uint64_t size,
    uint32_t have_blocks,       // [0, B)[D], in bitmask (lowest bit = lowest index)
    const uint8_t** have,       // uint8_t[D][size]
    uint32_t want_block,        // bit set = block we want (in the future this might also be a mask, if we want to recover multiple at once)
    uint8_t* want               // uint8_t[size]
);

#ifdef __cplusplus
}
#endif

#endif
