// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_BINCODE_H
#define _TERNFS_BINCODE_H

#ifdef __KERNEL__
#include <linux/kernel.h>
#include <linux/string.h>
#include <linux/bug.h>
#include <linux/version.h>
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6, 12, 0))
#include <asm/unaligned.h>
#else
#include <linux/unaligned.h>
#endif
#endif

struct ternfs_bincode_put_ctx { char* start; char* cursor; char* end; };

// If owned is not null, we need to free this buf when done.
struct ternfs_bincode_get_ctx { char* owned; char* buf; char* end; u16 err;};

struct ternfs_bincode_bytes { char* buf; u8 len; };

#define ternfs_bincode_get_finish_list_el(end) ((void)(end))

struct ternfs_failure_domain_start;
struct ternfs_failure_domain_end;

static inline void _ternfs_failure_domain_get(struct ternfs_bincode_get_ctx* ctx, struct ternfs_failure_domain_start* start, uint8_t* name) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < 16)) {
            ctx->err = 14; // 14 == TERNFS_ERR_MALFORMED_RESPONSE, see static assert below
        } else {
            memcpy(name, ctx->buf, 16);
            ctx->buf += 16;
        }
    }
}
#define ternfs_failure_domain_get(ctx, prev, next, name) \
    struct ternfs_failure_domain_end* next; \
    uint8_t name[16]; \
    _ternfs_failure_domain_get(ctx, prev, name)

#include "bincodegen.h"

#ifdef __KERNEL__
static_assert(14 == TERNFS_ERR_MALFORMED_RESPONSE);
#endif

inline static void ternfs_bincode_put_u64(struct ternfs_bincode_put_ctx* ctx, u64 x) {
    BUG_ON(ctx->end - ctx->cursor < sizeof(x));
    put_unaligned_le64(x, ctx->cursor);
    ctx->cursor += sizeof(x);
}
inline static void ternfs_bincode_put_u32(struct ternfs_bincode_put_ctx* ctx, u32 x) {
    BUG_ON(ctx->end - ctx->cursor < sizeof(x));
    put_unaligned_le32(x, ctx->cursor);
    ctx->cursor += sizeof(x);
}
inline static void ternfs_bincode_put_u16(struct ternfs_bincode_put_ctx* ctx, u16 x) {
    BUG_ON(ctx->end - ctx->cursor < sizeof(x));
    put_unaligned_le16(x, ctx->cursor);
    ctx->cursor += sizeof(x);
}
inline static void ternfs_bincode_put_u8(struct ternfs_bincode_put_ctx* ctx, u8 x) {
    BUG_ON(ctx->end - ctx->cursor < sizeof(x));
    *(u8*)(ctx->cursor) = x;
    ctx->cursor += sizeof(x);
}

inline static u64 ternfs_bincode_get_u64(struct ternfs_bincode_get_ctx* ctx) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < sizeof(u64))) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            return 0;
        } else {
            u64 x = get_unaligned_le64(ctx->buf);
            ctx->buf += sizeof(x);
            return x;
        }
    } else {
        return 0;
    }
}
inline static u32 ternfs_bincode_get_u32(struct ternfs_bincode_get_ctx* ctx) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < sizeof(u32))) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            return 0;
        } else {
            u32 x = get_unaligned_le32(ctx->buf);
            ctx->buf += sizeof(x);
            return x;
        }
    } else {
        return 0;
    }
}
inline static u16 ternfs_bincode_get_u16(struct ternfs_bincode_get_ctx* ctx) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < sizeof(u16))) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            return 0;
        } else {
            u16 x = get_unaligned_le16(ctx->buf);
            ctx->buf += sizeof(x);
            return x;
        }
    } else {
        return 0;
    }
}
inline static u8 ternfs_bincode_get_u8(struct ternfs_bincode_get_ctx* ctx) {
    if (likely(ctx->err == 0)) {
        if (unlikely(ctx->end - ctx->buf < sizeof(u8))) {
            ctx->err = TERNFS_ERR_MALFORMED_RESPONSE;
            return 0;
        } else {
            u8 x = *(u8*)(ctx->buf);
            ctx->buf += sizeof(x);
            return x;
        }
    } else {
        return 0;
    }
}

// >>> format(struct.unpack('<I', b'SHA\0')[0], 'x')
// '414853'
static const u32 TERNFS_SHARD_REQ_PROTOCOL_VERSION = 0x414853;

// >>> format(struct.unpack('<I', b'SHA\1')[0], 'x')
// '1414853'
static const u32 TERNFS_SHARD_RESP_PROTOCOL_VERSION = 0x1414853;

// >>> format(struct.unpack('<I', b'CDC\0')[0], 'x')
// '434443'
static const u32 TERNFS_CDC_REQ_PROTOCOL_VERSION = 0x434443;

// >>> format(struct.unpack('<I', b'CDC\1')[0], 'x')
// '1434443'
static const u32 TERNFS_CDC_RESP_PROTOCOL_VERSION = 0x1434443;

// >>> format(struct.unpack('<I', b'SHU\0')[0], 'x')
// '554853'
static const u32 TERNFS_REGISTRY_REQ_PROTOCOL_VERSION = 0x554853;

// >>> format(struct.unpack('<I', b'SHU\1')[0], 'x')
// '1554853'
static const u32 TERNFS_REGISTRY_RESP_PROTOCOL_VERSION = 0x1554853;

// >>> format(struct.unpack('<I', b'BLO\0')[0], 'x')
// '4f4c42'
static const u32 TERNFS_BLOCKS_REQ_PROTOCOL_VERSION = 0x4f4c42;

// >>> format(struct.unpack('<I', b'BLO\1')[0], 'x')
// '14f4c42'
static const u32 TERNFS_BLOCKS_RESP_PROTOCOL_VERSION = 0x14f4c42;

static const u8 SNAPSHOT_POLICY_TAG = 1;
static const u8 SPAN_POLICY_TAG = 2;
static const u8 BLOCK_POLICY_TAG = 3;
static const u8 STRIPE_POLICY_TAG = 4;

static const u8 TERNFS_EMPTY_STORAGE  = 0;
static const u8 TERNFS_INLINE_STORAGE = 1;
static const u8 TERNFS_HDD_STORAGE    = 2;
static const u8 TERNFS_FLASH_STORAGE  = 3;

#define TERNFS_GET_EXTRA(x) (((x) & (1ull<<63)) >> 63)
#define TERNFS_GET_EXTRA_ID(x) ((x) & ~(1ull<<63))

#define TERNFS_MAX_FILENAME 255

#define TERNFS_BLOCK_SERVICE_STALE           1u
#define TERNFS_BLOCK_SERVICE_NO_READ        (1u<<1)
#define TERNFS_BLOCK_SERVICE_NO_WRITE       (1u<<2)
#define TERNFS_BLOCK_SERVICE_DECOMMISSIONED (1u<<3)

#define TERNFS_BLOCK_SERVICE_DONT_READ  (TERNFS_BLOCK_SERVICE_STALE | TERNFS_BLOCK_SERVICE_NO_READ  | TERNFS_BLOCK_SERVICE_DECOMMISSIONED)
#define TERNFS_BLOCK_SERVICE_DONT_WRITE (TERNFS_BLOCK_SERVICE_STALE | TERNFS_BLOCK_SERVICE_NO_WRITE | TERNFS_BLOCK_SERVICE_DECOMMISSIONED)

#define TERNFS_FULL_READ_DIR_CURRENT   (1u << 0)
#define TERNFS_FULL_READ_DIR_BACKWARDS (1u << 1)
#define TERNFS_FULL_READ_DIR_SAME_NAME (1u << 2)

#define TERNFS_MAX_STRIPES 15

#endif
