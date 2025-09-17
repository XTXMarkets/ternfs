// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#undef TRACE_SYSTEM
#define TRACE_SYSTEM eggsfs

#if !defined(_TRACE_EGGFS_H) || defined(TRACE_HEADER_MULTI_READ)
#define _TRACE_TERNFS_H

#include <linux/version.h>
#include <linux/tracepoint.h>
#include <linux/namei.h>
#include <linux/net.h>
#include <net/udp.h>
#include <net/sock.h>
#include <linux/socket.h>
#include <linux/pagemap.h>
#include <net/inet_common.h>

#include "inode.h"
#include "rs.h"

TRACE_DEFINE_ENUM(LOOKUP_FOLLOW);
TRACE_DEFINE_ENUM(LOOKUP_DIRECTORY);
TRACE_DEFINE_ENUM(LOOKUP_AUTOMOUNT);
TRACE_DEFINE_ENUM(LOOKUP_PARENT);
TRACE_DEFINE_ENUM(LOOKUP_REVAL);
TRACE_DEFINE_ENUM(LOOKUP_RCU);
TRACE_DEFINE_ENUM(LOOKUP_OPEN);
TRACE_DEFINE_ENUM(LOOKUP_CREATE);
TRACE_DEFINE_ENUM(LOOKUP_EXCL);
TRACE_DEFINE_ENUM(LOOKUP_RENAME_TARGET);
TRACE_DEFINE_ENUM(LOOKUP_EMPTY);
TRACE_DEFINE_ENUM(LOOKUP_DOWN);

#if (LINUX_VERSION_CODE < KERNEL_VERSION(5, 14, 0))
TRACE_DEFINE_ENUM(LOOKUP_JUMPED);
TRACE_DEFINE_ENUM(LOOKUP_ROOT);

#define show_lookup_flags(flags) \
	__print_flags(flags, "|", \
			{ LOOKUP_FOLLOW, "FOLLOW" }, \
			{ LOOKUP_DIRECTORY, "DIRECTORY" }, \
			{ LOOKUP_AUTOMOUNT, "AUTOMOUNT" }, \
			{ LOOKUP_PARENT, "PARENT" }, \
			{ LOOKUP_REVAL, "REVAL" }, \
			{ LOOKUP_RCU, "RCU" }, \
			{ LOOKUP_OPEN, "OPEN" }, \
			{ LOOKUP_CREATE, "CREATE" }, \
			{ LOOKUP_EXCL, "EXCL" }, \
			{ LOOKUP_RENAME_TARGET, "RENAME_TARGET" }, \
			{ LOOKUP_JUMPED, "JUMPED" }, \
			{ LOOKUP_ROOT, "ROOT" }, \
			{ LOOKUP_EMPTY, "EMPTY" }, \
			{ LOOKUP_DOWN, "DOWN" })
#else

#define show_lookup_flags(flags) \
	__print_flags(flags, "|", \
			{ LOOKUP_FOLLOW, "FOLLOW" }, \
			{ LOOKUP_DIRECTORY, "DIRECTORY" }, \
			{ LOOKUP_AUTOMOUNT, "AUTOMOUNT" }, \
			{ LOOKUP_PARENT, "PARENT" }, \
			{ LOOKUP_REVAL, "REVAL" }, \
			{ LOOKUP_RCU, "RCU" }, \
			{ LOOKUP_OPEN, "OPEN" }, \
			{ LOOKUP_CREATE, "CREATE" }, \
			{ LOOKUP_EXCL, "EXCL" }, \
			{ LOOKUP_RENAME_TARGET, "RENAME_TARGET" }, \
			{ LOOKUP_EMPTY, "EMPTY" }, \
			{ LOOKUP_DOWN, "DOWN" })
#endif

#define TERNFS_TRACE_EVENT_inode(_name) \
    TRACE_EVENT(eggsfs_##_name, \
        TP_PROTO(struct inode* inode), \
        TP_ARGS(inode), \
        TP_STRUCT__entry( \
            __field(u64, ino) \
        ), \
        TP_fast_assign( \
            __entry->ino = inode->i_ino; \
        ), \
        TP_printk("enode=%#llx", __entry->ino) \
    )

#define TERNFS_TRACE_EVENT_inode_ret(_name) \
    TRACE_EVENT(eggsfs_##_name, \
        TP_PROTO(struct inode* inode, int err), \
        TP_ARGS(inode, err), \
        TP_STRUCT__entry( \
            __field(u64, ino) \
            __field(int, err) \
        ), \
        TP_fast_assign( \
            __entry->ino = inode->i_ino; \
            __entry->err = err; \
        ), \
        TP_printk("enode=%#llx err=%d", __entry->ino, __entry->err) \
    )

#define TERNFS_TRACE_EVENT_dir_dentry(_name) \
    TRACE_EVENT(eggsfs_##_name, \
        TP_PROTO(struct inode* dir, struct dentry* dentry), \
        TP_ARGS(dir, dentry), \
        TP_STRUCT__entry( \
            __field(u64, dir) \
            __string(name, dentry->d_name.name) \
        ), \
        TP_fast_assign( \
            __entry->dir = dir->i_ino; \
            __assign_str(name, dentry->d_name.name); \
        ), \
        TP_printk("dir=%#llx name=%s", __entry->dir, __get_str(name)) \
    )

#define TERNFS_TRACE_EVENT_dir_dentry_ret(_name) \
    TRACE_EVENT(eggsfs_##_name, \
        TP_PROTO(struct inode* dir, struct dentry* dentry, int err), \
        TP_ARGS(dir, dentry, err), \
        TP_STRUCT__entry( \
            __field(u64, dir) \
            __field(int, err) \
            __string(name, dentry->d_name.name) \
        ), \
        TP_fast_assign( \
            __entry->dir = dir->i_ino; \
            __entry->err = err; \
            __assign_str(name, dentry->d_name.name); \
        ), \
        TP_printk("dir=%llx name=%s err=%d", __entry->dir, __get_str(name), __entry->err) \
    )

#define TERNFS_TRACE_EVENT_dir_dentry_inode(_name) \
    TRACE_EVENT(eggsfs_##_name, \
        TP_PROTO(struct inode* dir, struct dentry* dentry, struct inode* inode), \
        TP_ARGS(dir, dentry, inode), \
        TP_STRUCT__entry( \
            __field(u64, dir) \
            __field(u64, ino) \
            __string(name, dentry->d_name.name) \
        ), \
        TP_fast_assign( \
            __entry->dir = dir->i_ino; \
            __entry->ino = inode ? inode->i_ino : 0; \
            __assign_str(name, dentry->d_name.name); \
        ), \
        TP_printk("dir=%llx name=%s ino=%llx", __entry->dir, __get_str(name), __entry->ino) \
    )


#define TERNFS_TRACE_EVENT_dir_dentry_inode_ret(_name) \
    TRACE_EVENT(eggsfs_##_name, \
        TP_PROTO(struct inode* dir, struct dentry* dentry, struct inode* inode, int err), \
        TP_ARGS(dir, dentry, inode, err), \
        TP_STRUCT__entry( \
            __field(u64, dir) \
            __field(u64, ino) \
            __field(int, err) \
            __string(name, dentry->d_name.name) \
        ), \
        TP_fast_assign( \
            __entry->dir = dir->i_ino; \
            __entry->ino = inode ? inode->i_ino : 0; \
            __entry->err = err; \
            __assign_str(name, dentry->d_name.name); \
        ), \
        TP_printk("dir=%llx name=%s ino=%llx err=%d", __entry->dir, __get_str(name), __entry->ino, __entry->err) \
    )

// dcache
TERNFS_TRACE_EVENT_inode(dcache_delete_inode);
TERNFS_TRACE_EVENT_inode(dcache_invalidate_dir);
TERNFS_TRACE_EVENT_dir_dentry(dcache_invalidate_neg_entry);
TERNFS_TRACE_EVENT_dir_dentry_inode(dcache_delete_entry);
TERNFS_TRACE_EVENT_dir_dentry_inode(dcache_invalidate_entry);

// inode.c
TERNFS_TRACE_EVENT_inode(vfs_getattr_enter);
TERNFS_TRACE_EVENT_inode(vfs_getattr_lock);
TERNFS_TRACE_EVENT_inode_ret(vfs_getattr_exit);

// dir.c
TERNFS_TRACE_EVENT_inode(vfs_opendir_enter);
TERNFS_TRACE_EVENT_inode_ret(vfs_opendir_exit);

TERNFS_TRACE_EVENT_inode(vfs_closedir_enter);
TERNFS_TRACE_EVENT_inode_ret(vfs_closedir_exit);

// dentry.c
TERNFS_TRACE_EVENT_dir_dentry(vfs_lookup_enter);
TERNFS_TRACE_EVENT_dir_dentry_inode_ret(vfs_lookup_exit);

TERNFS_TRACE_EVENT_dir_dentry(vfs_mkdir_enter);
TERNFS_TRACE_EVENT_dir_dentry_inode_ret(vfs_mkdir_exit);

TERNFS_TRACE_EVENT_dir_dentry_inode(vfs_rmdir_enter);
TERNFS_TRACE_EVENT_dir_dentry_ret(vfs_rmdir_exit);

TERNFS_TRACE_EVENT_dir_dentry_inode(vfs_unlink_enter);
TERNFS_TRACE_EVENT_dir_dentry_ret(vfs_unlink_exit);

TRACE_EVENT(eggsfs_vfs_rename_enter,
    TP_PROTO(struct inode* old_dir, struct dentry* old_dentry, struct inode* new_dir, struct dentry* new_dentry),
    TP_ARGS(old_dir, old_dentry, new_dir, new_dentry),

    TP_STRUCT__entry(
        __field(u64, old_dir_ino)
        __field(u64, new_dir_ino)
        __field(u64, ino)
        __string(old_parent, old_dentry->d_parent->d_name.name)
        __string(old_name, old_dentry->d_name.name)
        __string(new_parent, new_dentry->d_parent->d_name.name)
        __string(new_name, new_dentry->d_name.name)
    ),
    TP_fast_assign(
        __entry->old_dir_ino = old_dir->i_ino;
        __entry->new_dir_ino = new_dir->i_ino;
        __entry->ino = old_dentry->d_inode->i_ino;
        __assign_str(old_parent, old_dentry->d_parent->d_name.name);
        __assign_str(old_name, old_dentry->d_name.name)
        __assign_str(new_parent, new_dentry->d_parent->d_name.name);
        __assign_str(new_name, new_dentry->d_name.name)
    ),
    TP_printk("ino=%lld old_dir=%lld old_name=%s/%s -> new_dir=%lld new_name=%s/%s", __entry->ino, __entry->old_dir_ino, __get_str(old_parent), __get_str(old_name), __entry->new_dir_ino, __get_str(new_parent), __get_str(new_name))
);

TRACE_EVENT(eggsfs_vfs_rename_exit,
    TP_PROTO(struct inode* old_dir, struct dentry* old_dentry, struct inode* new_dir, struct dentry* new_dentry, int err),
    TP_ARGS(old_dir, old_dentry, new_dir, new_dentry, err),

    TP_STRUCT__entry(
        __field(u64, old_dir_ino)
        __field(u64, new_dir_ino)
        __field(u64, ino)
        __field(int, err)
        __string(old_parent, old_dentry->d_parent->d_name.name)
        __string(old_name, old_dentry->d_name.name)
        __string(new_parent, new_dentry->d_parent->d_name.name)
        __string(new_name, new_dentry->d_name.name)
    ),
    TP_fast_assign(
        __entry->old_dir_ino = old_dir->i_ino;
        __entry->new_dir_ino = new_dir->i_ino;
        __entry->ino = old_dentry->d_inode->i_ino;
        __entry->err = err;
        __assign_str(old_parent, old_dentry->d_parent->d_name.name);
        __assign_str(old_name, old_dentry->d_name.name)
        __assign_str(new_parent, new_dentry->d_parent->d_name.name);
        __assign_str(new_name, new_dentry->d_name.name)
    ),
    TP_printk("ino=%lld old_dir=%lld old_name=%s/%s -> new_dir=%lld new_name=%s/%s err=%d", __entry->ino, __entry->old_dir_ino, __get_str(old_parent), __get_str(old_name), __entry->new_dir_ino, __get_str(new_parent), __get_str(new_name), __entry->err)
);

#define TERNFS_METADATA_REQUEST_ATTEMPT 0
#define TERNFS_METADATA_REQUEST_DONE 1

TRACE_EVENT(eggsfs_metadata_request,
    TP_PROTO(struct sockaddr_in* addr, u64 req_id, u32 len, s16 shard_id, u8 kind, u32 n_attempts, u32 resp_len, u8 event, int error),
    TP_ARGS(addr, req_id, len, shard_id, kind, n_attempts, resp_len, event, error),

    TP_STRUCT__entry(
        __array(u8, addr, sizeof(struct sockaddr_in))
        __field(u64, req_id)
        __field(u32, len)
        __field(u32, n_attempts)
        __field(u32, resp_len)
        __field(int, error)
        __field(s16, shard_id) // -1 is used for CDC
        __field(u8, event)
        __field(u8, kind)
    ),
    TP_fast_assign(
        memcpy(__entry->addr, addr, sizeof(struct sockaddr_in));
        __entry->req_id = req_id;
        __entry->len = len;
        __entry->shard_id = shard_id;
        __entry->kind = kind;
        __entry->n_attempts = n_attempts;
        __entry->resp_len = resp_len;
        __entry->error = error;
        __entry->event = event;
    ),
    TP_printk(
        "dst=%pISp req_id=%llu event=%s shard_id=%d kind=%s len=%u n_attempts=%u resp_len=%u error=%d",
        __entry->addr, __entry->req_id,
        __print_symbolic(
            __entry->event,
            { 0, "attempt" },
            { 1, "done" }
        ),
        __entry->shard_id,
        __entry->shard_id >= 0 ?
            __print_ternfs_shard_kind(__entry->kind) :
            __print_ternfs_cdc_kind(__entry->kind),
        __entry->len, __entry->n_attempts, __entry->resp_len,
        __entry->error
    )
);

TRACE_EVENT(eggsfs_get_inode_enter,
    TP_PROTO(u64 ino),
    TP_ARGS(ino),

    TP_STRUCT__entry(
        __field(u64, ino)
    ),
    TP_fast_assign(
        __entry->ino = ino;
    ),
    TP_printk("ino=%lld", __entry->ino)
);

TRACE_EVENT(eggsfs_get_inode_exit,
    TP_PROTO(u64 ino, struct inode* inode, bool new, int error),
    TP_ARGS(ino, inode, new, error),

    TP_STRUCT__entry(
        __field(u64, ino)
        __field(struct inode*, inode)
        __field(bool, new)
        __field(int, error)
    ),
    TP_fast_assign(
        __entry->ino = ino;
        __entry->inode = inode;
        __entry->new = new;
        __entry->error = error;
    ),
    TP_printk("ino=%lld inode=%p new=%d error=%d", __entry->ino, __entry->inode, __entry->new, __entry->error)
);

TRACE_EVENT(eggsfs_dentry_handle_enoent,
    TP_PROTO(struct dentry* dentry),
    TP_ARGS(dentry),

    TP_STRUCT__entry(
        __field(u64, ino)
        __string(parent, dentry->d_parent->d_name.name)
        __string(name, dentry->d_name.name)
    ),
    TP_fast_assign(
        __entry->ino = dentry->d_inode ? dentry->d_inode->i_ino : 0;
        __assign_str(name, dentry->d_parent->d_name.name);
        __assign_str(name, dentry->d_name.name);
    ),
    TP_printk("ino=%lld name=%s/%s", __entry->ino, __get_str(parent), __get_str(name))
);

#define TERNFS_BLOCK_WRITE_START 7
#define TERNFS_BLOCK_WRITE_QUEUED 0
#define TERNFS_BLOCK_WRITE_RECV_ENTER 1
#define TERNFS_BLOCK_WRITE_RECV_EXIT 2
#define TERNFS_BLOCK_WRITE_WRITE_ENTER 3
#define TERNFS_BLOCK_WRITE_WRITE_EXIT 4
#define TERNFS_BLOCK_WRITE_WRITE_QUEUED 5
#define TERNFS_BLOCK_WRITE_DONE 6
#define TERNFS_BLOCK_WRITE_COMPLETE_QUEUED 8

TRACE_EVENT(eggsfs_block_write,
    TP_PROTO(u64 block_service_id, u64 block_id, u8 event, u32 block_size, u8 req_written, u32 block_written, u8 resp_read, int err),
    TP_ARGS(block_service_id, block_id, event, block_size, req_written, block_written, resp_read, err),

    TP_STRUCT__entry(
        __field(u64, block_service_id)
        __field(u64, block_id)
        __field(u32, block_size)
        __field(u32, block_written)
        __field(int, err)
        __field(u8, req_written)
        __field(u8, resp_read)
        __field(u8, event)
    ),
    TP_fast_assign(
        __entry->block_service_id = block_service_id;
        __entry->block_id = block_id;
        __entry->block_size = block_size;
        __entry->block_written = block_written;
        __entry->err = err;
        __entry->req_written = req_written;
        __entry->resp_read = resp_read;
        __entry->event = event;
    ),
    TP_printk(
        "block_service=%016llx block_id=%016llx event=%s block_size=%u req_written=%u block_written=%u resp_read=%u err=%d",
        __entry->block_service_id, __entry->block_id,
        __print_symbolic(
            __entry->event,
            { 7, "start" },
            { 0, "queued" },
            { 1, "recv_enter" },
            { 2, "recv_exit" },
            { 3, "write_enter" },
            { 4, "write_exit" },
            { 5, "write_queued" },
            { 6, "done" },
            { 8, "complete_queued" }
        ),
        __entry->block_size, __entry->req_written, __entry->block_written, __entry->resp_read, __entry->err
    )
);

TRACE_EVENT(eggsfs_span_flush_enter,
    TP_PROTO(u64 file_id, u64 offset, u32 size, u8 parity, u8 stripes, u8 storage_class),
    TP_ARGS(     file_id,     offset,     size,    parity,    stripes,    storage_class),

    TP_STRUCT__entry(
        __field(u64, file_id)
        __field(u64, offset)
        __field(u32, size)
        __field(u8, parity)
        __field(u8, stripes)
        __field(u8, storage_class)
    ),
    TP_fast_assign(
        __entry->file_id = file_id;
        __entry->offset = offset;
        __entry->size = size;
        __entry->parity = parity;
        __entry->stripes = stripes;
        __entry->storage_class = storage_class;
    ),
    TP_printk("file_id=%016llx offset=%llu size=%u parity=RS(%u,%u) stripes=%u, storage_class=%u", __entry->file_id, __entry->offset, __entry->size, ternfs_data_blocks(__entry->parity), ternfs_parity_blocks(__entry->parity), (int)__entry->stripes, (int)__entry->storage_class)
);

TRACE_EVENT(eggsfs_span_flush_exit,
    TP_PROTO(u64 file_id, u64 offset, u32 size, int err),
    TP_ARGS(     file_id,     offset,     size,     err),

    TP_STRUCT__entry(
        __field(u64, file_id)
        __field(u64, offset)
        __field(u32, size)
        __field(int, err)
    ),
    TP_fast_assign(
        __entry->file_id = file_id;
        __entry->offset = offset;
        __entry->size = size;
        __entry->err = err;
    ),
    TP_printk("file_id=%016llx offset=%llu size=%u err=%d", __entry->file_id, __entry->offset, __entry->size, __entry->err)
);

TRACE_EVENT(eggsfs_span_add,
    TP_PROTO(u64 file_id, u64 offset),
    TP_ARGS(     file_id,     offset),

    TP_STRUCT__entry(
        __field(u64, file_id)
        __field(u64, offset)
    ),
    TP_fast_assign(
        __entry->file_id = file_id;
        __entry->offset = offset;
    ),
    TP_printk("file_id=%016llx offset=%llu", __entry->file_id, __entry->offset)
);

TRACE_EVENT(eggsfs_get_span_enter,
    TP_PROTO(u64 file_id, u64 offset),
    TP_ARGS(     file_id,     offset),

    TP_STRUCT__entry(
        __field(u64, file_id)
        __field(u64, offset)
    ),
    TP_fast_assign(
        __entry->file_id = file_id;
        __entry->offset = offset;
    ),
    TP_printk("file_id=%016llx offset=%llu", __entry->file_id, __entry->offset)
);

TRACE_EVENT(eggsfs_get_span_exit,
    TP_PROTO(u64 file_id, u64 offset, int err),
    TP_ARGS(     file_id,     offset,     err),

    TP_STRUCT__entry(
        __field(u64, file_id)
        __field(u64, offset)
        __field(int, err)
    ),
    TP_fast_assign(
        __entry->file_id = file_id;
        __entry->offset = offset;
    ),
    TP_printk("file_id=%016llx offset=%llu, err=%d", __entry->file_id, __entry->offset, __entry->err)
);

#define TERNFS_UPSERT_BLOCKSERVICE_MATCH 0
#define TERNFS_UPSERT_BLOCKSERVICE_NOMATCH 1
#define TERNFS_UPSERT_BLOCKSERVICE_NEW 2

TRACE_EVENT(eggsfs_upsert_block_service,
    TP_PROTO(u64 bs_id, u8 event),
    TP_ARGS(     bs_id,    event),
    TP_STRUCT__entry(
        __field(u64, bs_id)
        __field(u8, event)
    ),
    TP_fast_assign(
        __entry->bs_id = bs_id;
        __entry->event = event;
    ),
    TP_printk(
        "bs_id=%016llx event=%s", __entry->bs_id,
        __print_symbolic(
            __entry->event,
            { TERNFS_UPSERT_BLOCKSERVICE_MATCH, "match" },
            { TERNFS_UPSERT_BLOCKSERVICE_NOMATCH, "nomatch" },
            { TERNFS_UPSERT_BLOCKSERVICE_NEW, "new" }
        )
    )
);

#if 0
#define TERNFS_FETCH_BLOCK_SOCKET_START 0
#define TERNFS_FETCH_BLOCK_SOCKET_BLOCK_START 1
#define TERNFS_FETCH_BLOCK_SOCKET_BLOCK_DONE 2
#define TERNFS_FETCH_BLOCK_SOCKET_END 3
#define TERNFS_FETCH_BLOCK_SOCKET_FREE 4

TRACE_EVENT(eggsfs_fetch_block_socket,
    TP_PROTO(u32 ip, u16 port, u8 event, int err),
    TP_ARGS(     ip,     port,    event,     err),

    TP_STRUCT__entry(
        __field(u32, ip)
        __field(int, err)
        __field(u16, port)
        __field(u8, event)
    ),
    TP_fast_assign(
        __entry->ip = ip;
        __entry->err = err;
        __entry->port = port;
        __entry->event = event;
    ),
    TP_printk(
        "ip=%08x port=%u event=%s err=%d",
        __entry->ip, __entry->port,
        __print_symbolic(
            __entry->event,
            { TERNFS_FETCH_STRIPE_START, "start" },
            { TERNFS_FETCH_STRIPE_BLOCK_START, "block_start" },
            { TERNFS_FETCH_STRIPE_BLOCK_DONE, "block_done" },
            { TERNFS_FETCH_STRIPE_END, "end" },
            { TERNFS_FETCH_STRIPE_FREE, "free" }
        ),
        __entry->block, __entry->err
    )
)
#endif

#endif /* _TRACE_EGGFS_H */

#undef TRACE_INCLUDE_PATH
#define TRACE_INCLUDE_PATH .
#define TRACE_INCLUDE_FILE trace

#include <trace/define_trace.h>

