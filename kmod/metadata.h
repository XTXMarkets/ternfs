// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_SHARD_H
#define _TERNFS_SHARD_H

#include <linux/kernel.h>

#include "net.h"
#include "super.h"
#include "inode.h"

#define TERNFS_DEFAULT_MTU 1472
#define TERNFS_MAX_MTU 8972

extern int ternfs_mtu;
extern int ternfs_default_mtu;
extern int ternfs_max_mtu;

int ternfs_shard_lookup(struct ternfs_fs_info* info, u64 dir, const char* name, int name_len, u64* ino, u64* creation_time);
int ternfs_shard_readdir(struct ternfs_fs_info* info, u64 dir, u64 start_pos, void* data, u64* next_hash);
int ternfs_shard_soft_unlink_file(
    struct ternfs_fs_info* info, u64 dir, u64 file, const char* name, int name_len, u64 creation_time, u64* delete_creation_time
);
int ternfs_shard_hard_unlink_file(struct ternfs_fs_info* info, u64 dir, u64 file, const char* name, int name_len, u64 creation_time);
int ternfs_shard_rename(struct ternfs_fs_info* info, u64 dir, u64 target_id, const char* old_name, int old_name_len, u64 old_creation_time, const char* new_name, int new_name_len, u64* new_creation_time);
int ternfs_shard_link_file(struct ternfs_fs_info* info, u64 file, u64 cookie, u64 dir, const char* name, int name_len, u64* creation_time);
int ternfs_shard_getattr_file(struct ternfs_fs_info* info, u64 file, u64* mtime, u64* atime, u64* size);
int ternfs_shard_async_getattr_file(struct ternfs_fs_info* info, struct ternfs_metadata_request* metadata_req, u64 file);
int ternfs_shard_parse_getattr_file(struct sk_buff* skb, u64* mtime, u64* atime, u64* size);
int ternfs_shard_getattr_dir(
    struct ternfs_fs_info* info,
    u64 file,
    u64* mtime,
    u64* owner,
    struct ternfs_policy_body* block_policies,
    struct ternfs_policy_body* span_policies,
    struct ternfs_policy_body* stripe_policy,
    struct ternfs_policy_body* snapshot_policy
);
int ternfs_shard_async_getattr_dir(struct ternfs_fs_info* info, struct ternfs_metadata_request* metadata_req, u64 dir);
int ternfs_shard_parse_getattr_dir(
    struct sk_buff* skb,
    u64* mtime,
    u64* owner,
    struct ternfs_policy_body* block_policies,
    struct ternfs_policy_body* span_policies,
    struct ternfs_policy_body* stripe_policy,
    struct ternfs_policy_body* snapshot_policy
);
int ternfs_shard_create_file(struct ternfs_fs_info* info, u8 shid, int itype, const char* name, int name_len, u64* ino, u64* cookie);

typedef void (*ternfs_file_spans_cb_inline_span)(void* data, u64 offset, u32 size, u8 len, const char* body);
typedef void (*ternfs_file_spans_cb_span)(
    void* data, u64 offset, u32 size, u32 crc,
    u8 storage_class, u8 parity, u8 stripes, u32 cell_size
);
typedef void (*ternfs_file_spans_cb_block)(
    void* data, int block_ix,
    // block service stuff
    u64 bs_id, u32 ip1, u16 port1, u32 ip2, u16 port2, u8 flags,
    // block stuff
    u64 block_id, u32 crc
);

int ternfs_shard_file_spans(
    struct ternfs_fs_info* info,
    u64 file, u64 offset,
    u64* next_offset,
    ternfs_file_spans_cb_inline_span inline_span_cb,
    ternfs_file_spans_cb_span span_cb,
    ternfs_file_spans_cb_block block_cb,
    void* cb_data);
int ternfs_shard_add_inline_span(struct ternfs_fs_info* info, u64 file, u64 cookie, u64 offset, u32 size, const char* data, u8 len);
int ternfs_shard_set_time(struct ternfs_fs_info* info, u64 file, u64 mtime, u64 atime);
// Shoots a set_time request, does not wait/retry etc.
int ternfs_shard_set_atime_nowait(struct ternfs_fs_info* info, u64 file, u64 atime);

// just to have some static size
#define TERNFS_MAX_BLACKLIST_LENGTH 8

struct ternfs_add_span_initiate_block {
    u8 failure_domain[16];
    u64 block_service_id;
    u64 block_id;
    u64 certificate;
    u32 ip1;
    u32 ip2;
    u16 port2;
    u16 port1;
};

int ternfs_shard_add_span_initiate(
    struct ternfs_fs_info* info, void* data, u64 file, u64 cookie, u64 offset, u32 size, u32 crc, u8 storage_class, u8 parity,
    u8 stripes, u32 cell_size, u32* cell_crcs,
    // blacklisted failure domains
    u16 blacklist_length, char (*blacklist)[16],
    // B long
    struct ternfs_add_span_initiate_block* blocks
);
int ternfs_shard_add_span_certify(
    struct ternfs_fs_info* info, u64 file, u64 cookie, u64 offset, u8 parity, const u64* block_ids, const u64* block_proofs
);
int ternfs_shard_move_span(
    struct ternfs_fs_info* info, u64 file1, u64 offset1, u64 cookie1, u64 file2, u64 offset2, u64 cookie2, u32 span_size
);

int ternfs_cdc_mkdir(struct ternfs_fs_info* info, u64 dir, const char* name, int name_len, u64* ino, u64* creation_time);
int ternfs_cdc_rmdir(struct ternfs_fs_info* info, u64 owner_dir, u64 target, u64 creation_time, const char* name, int name_len);
int ternfs_cdc_cross_shard_hard_unlink_file(struct ternfs_fs_info* info, u64 dir, u64 file, const char* name, int name_len, u64 creation_time);

int ternfs_cdc_rename_directory(
    struct ternfs_fs_info* info,
    u64 target, u64 old_parent, u64 new_parent,
    const char* old_name, int old_name_len, u64 old_creation_time,
    const char* new_name, int new_name_len,
    u64* new_creation_time
);
int ternfs_cdc_rename_file(
    struct ternfs_fs_info* info,
    u64 target, u64 old_parent, u64 new_parent,
    const char* old_name, int old_name_len, u64 old_creation_time,
    const char* new_name, int new_name_len,
    u64* new_creation_time
);

void __init ternfs_shard_init(void);

#endif
