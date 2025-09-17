// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_RS_H
#define _TERNFS_RS_H

#include <linux/kernel.h>

#define TERNFS_MAX_DATA 10
#define TERNFS_MAX_PARITY 6
#define TERNFS_MAX_BLOCKS (TERNFS_MAX_DATA+TERNFS_MAX_PARITY)

extern int ternfs_rs_cpu_level;
extern int ternfs_rs_cpu_level_min;
extern int ternfs_rs_cpu_level_max;

static inline u8 ternfs_data_blocks(u8 parity) {
    return parity & 0x0F;
}

static inline u8 ternfs_parity_blocks(u8 parity) {
    return parity >> 4;
}

static inline u8 ternfs_blocks(u8 parity) {
    return ternfs_data_blocks(parity) + ternfs_parity_blocks(parity);
}

static inline u8 ternfs_mk_parity(u8 data, u8 parity) {
    return (parity << 4) | data;
}

// len(data) = ternfs_data_blocks(parity)
// len(out)  = ternfs_parity_blocks(parity)
//
// You _must_ wrap this with kernel_fpu_begin()/kernel_fpu_end()!
int ternfs_compute_parity(u8 parity, ssize_t size, const char** data, char** out);

// This function does kernel_fpu_begin()/kernel_fpu_end() itself, since it already
// works with pages.
int ternfs_recover(
    u8 parity,
    u32 have_blocks,
    u32 want_block,
    u32 num_pages,
    // B long array of pages. We'll get the pages from the blocks we have and
    // put them in the block we want. The lists should all be of the same length.
    struct list_head* pages
);

int __init ternfs_rs_init(void);
void __cold ternfs_rs_exit(void);

#endif
