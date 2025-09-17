// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <stdint.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <immintrin.h>
#include <stdbool.h>

typedef uint8_t u8;
typedef uint16_t u16;
typedef uint32_t u32;
typedef uint64_t u64;

#define likely(x)       __builtin_expect(!!(x), 1)
#define unlikely(x)     __builtin_expect(!!(x), 0)

#define BUG_ON(x) \
    if (unlikely(x)) { \
        fprintf(stderr, "bug: " #x); \
        fprintf(stderr, "\n"); \
        exit(1); \
    }

#define TERNFS_UDP_MTU 1472

static inline u64 get_unaligned_le64(const void* p) {
    u64 x;
    memcpy(&x, p, sizeof(x));
    return x;
}
static inline u32 get_unaligned_le32(const void* p) {
    u32 x;
    memcpy(&x, p, sizeof(x));
    return x;
}
static inline u16 get_unaligned_le16(const void* p) {
    u16 x;
    memcpy(&x, p, sizeof(x));
    return x;
}

static inline u64 get_unaligned_be64(const void* p) {
    u64 x;
    memcpy(&x, p, sizeof(x));
    return __bswap_64(x);
}
static inline u32 get_unaligned_be32(const void* p) {
    u32 x;
    memcpy(&x, p, sizeof(x));
    return __bswap_32(x);
}

static inline void put_unaligned_le64(u64 x, void* p) {
    memcpy(p, &x, sizeof(x));
}
static inline void put_unaligned_le32(u32 x, void* p) {
    memcpy(p, &x, sizeof(x));
}
static inline void put_unaligned_le16(u16 x, void* p) {
    memcpy(p, &x, sizeof(x));
}

static inline void put_unaligned_be64(u64 x, void* p) {
    x = __bswap_64(x);
    memcpy(p, &x, sizeof(x));
}
static inline void put_unaligned_be32(u32 x, void* p) {
    x = __bswap_32(x);
    memcpy(p, &x, sizeof(x));
}

#include "bincode.h"

int main(void) {
    {
        char lookup_req[TERNFS_LOOKUP_REQ_MAX_SIZE];
        struct ternfs_bincode_put_ctx put_ctx = {
            .start = lookup_req,
            .cursor = lookup_req,
            .end = lookup_req + sizeof(lookup_req),
        };
        {
            ternfs_lookup_req_put_start(&put_ctx, start);
            ternfs_lookup_req_put_dir_id(&put_ctx, start, dir_id, 123);
            ternfs_lookup_req_put_name(&put_ctx, dir_id, name, "foo", 3);
            ternfs_lookup_req_put_end(&ctx, name, end);
        }
        struct ternfs_bincode_get_ctx get_ctx = {
            .buf = put_ctx.start,
            .end = put_ctx.cursor,
            .err = 0,
        };
        {
            ternfs_lookup_req_get_start(&get_ctx, start);
            ternfs_lookup_req_get_dir_id(&get_ctx, start, dir_id);
            ternfs_lookup_req_get_name(&get_ctx, dir_id, name);
            ternfs_lookup_req_get_end(&get_ctx, name, end);
            ternfs_lookup_req_get_finish(&get_ctx, end);
            BUG_ON(get_ctx.err != 0);
            BUG_ON(dir_id.x != 123);
            BUG_ON(name.str.len != 3);
            BUG_ON(strncmp("foo", name.str.buf, 3) != 0);
        }
    }

    {
        char read_dir_resp[TERNFS_UDP_MTU];
        struct ternfs_bincode_put_ctx put_ctx = {
            .start = read_dir_resp,
            .cursor = read_dir_resp,
            .end = read_dir_resp + sizeof(read_dir_resp),
        };
        {
            ternfs_read_dir_resp_put_start(&put_ctx, start);
            ternfs_read_dir_resp_put_next_hash(&put_ctx, start, next_hash, 123);
            ternfs_read_dir_resp_put_results(&put_ctx, next_hash, results, 3);
            for (int i = 0; i < 3; i++) {
                ternfs_current_edge_put_start(&put_ctx, edge_start);
                ternfs_current_edge_put_target_id(&put_ctx, edge_start, target_id, i);
                ternfs_current_edge_put_name_hash(&put_ctx, target_id, name_hash, i + 42);
                char name_str[2];
                snprintf(name_str, 2, "%d", i);
                ternfs_current_edge_put_name(&put_ctx, name_hash, name, name_str, 1);
                ternfs_current_edge_put_creation_time(&put_ctx, name, creation_time, i + 1000);
                ternfs_current_edge_put_end(&put_ctx, creation_time, end);
            }
            ternfs_read_dir_resp_put_end(&ctx, results, end);
        }
        struct ternfs_bincode_get_ctx get_ctx = {
            .buf = put_ctx.start,
            .end = put_ctx.cursor,
            .err = 0,
        };
        {
            ternfs_read_dir_resp_get_start(&get_ctx, start);
            ternfs_read_dir_resp_get_next_hash(&get_ctx, start, next_hash);
            ternfs_read_dir_resp_get_results(&get_ctx, next_hash, results);
            for (int i = 0; i < results.len; i++) {
                ternfs_current_edge_get_start(&get_ctx, edge_start);
                ternfs_current_edge_get_target_id(&get_ctx, edge_start, target_id);
                ternfs_current_edge_get_name_hash(&get_ctx, target_id, name_hash);
                ternfs_current_edge_get_name(&get_ctx, name_hash, name);
                ternfs_current_edge_get_creation_time(&get_ctx, name, creation_time);
                ternfs_current_edge_get_end(&put_ctx, creation_time, end);
                ternfs_bincode_get_finish_list_el(end);
                if (get_ctx.err == 0) {
                    BUG_ON(target_id.x != i);
                    BUG_ON(name_hash.x != i + 42);
                    BUG_ON(name.str.len != 1);
                    char name_str[11];
                    snprintf(name_str, 11, "%d", i);
                    BUG_ON(strlen(name_str) != 1);
                    BUG_ON(strncmp(name.str.buf, name_str, 1) != 0);
                    BUG_ON(creation_time.x != i + 1000);
                }
            }
            ternfs_read_dir_resp_get_end(&get_ctx, results, end);
            ternfs_read_dir_resp_get_finish(&get_ctx, end);
            BUG_ON(get_ctx.err != 0);
            BUG_ON(next_hash.x != 123);
        }
    }

    return 0;
}