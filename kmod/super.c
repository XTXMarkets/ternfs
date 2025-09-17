// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "super.h"

#include <linux/backing-dev.h>
#include <linux/inet.h>
#include <linux/kernel.h>
#include <linux/module.h>
#include <linux/statfs.h>
#include <linux/workqueue.h>
#include <linux/parser.h>

#include "block_services.h"
#include "log.h"
#include "inode.h"
#include "export.h"
#include "metadata.h"
#include "dentry.h"
#include "net.h"
#include "registry.h"
#include "bincode.h"
#include "sysfs.h"
#include "err.h"
#include "rs.h"

#define MSECS_TO_JIFFIES(_ms) ((_ms * HZ) / 1000)
int ternfs_registry_refresh_time_jiffies = MSECS_TO_JIFFIES(60000);
unsigned int ternfs_readahead_pages = 10000;

static void ternfs_free_fs_info(struct ternfs_fs_info* info) {
    ternfs_debug("info=%p", info);
    cancel_delayed_work_sync(&info->registry_refresh_work);
    ternfs_net_shard_free_socket(&info->sock);
    put_net(info->registry_addr.net);
    kfree(info->registry_addr.addr);
    kfree(info);
}

static int ternfs_refresh_fs_info(struct ternfs_fs_info* info) {
    int err;

    struct socket* registry_sock;
    err = ternfs_create_registry_socket(&info->registry_addr, &registry_sock);
    if (err < 0) { return err; }

    struct kvec iov;
    struct msghdr msg = {NULL};
    {
        static_assert(TERNFS_LOCAL_SHARDS_REQ_SIZE == 0);
        char registry_req[TERNFS_REGISTRY_REQ_HEADER_SIZE];
        ternfs_write_registry_req_header(registry_req, TERNFS_LOCAL_SHARDS_REQ_SIZE, TERNFS_REGISTRY_LOCAL_SHARDS);
        int written_so_far;
        for (written_so_far = 0; written_so_far < sizeof(registry_req);) {
            iov.iov_base = registry_req + written_so_far;
            iov.iov_len = sizeof(registry_req) - written_so_far;
            int written = kernel_sendmsg(registry_sock, &msg, &iov, 1, iov.iov_len);
            if (written < 0) { err = written; goto out_sock; }
            written_so_far += written;
        }

        char shards_resp_header[TERNFS_REGISTRY_RESP_HEADER_SIZE + 2]; // + 2 = list len
        int read_so_far;
        for (read_so_far = 0; read_so_far < sizeof(shards_resp_header);) {
            iov.iov_base = shards_resp_header + read_so_far;
            iov.iov_len = sizeof(shards_resp_header) - read_so_far;
            int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
            if (read == 0) { err = -ECONNRESET; goto out_sock; }
            if (read < 0) { err = read; goto out_sock; }
            read_so_far += read;
        }
        u32 registry_resp_len;
        u8 registry_resp_kind;
        err = ternfs_read_registry_resp_header(shards_resp_header, &registry_resp_len, &registry_resp_kind);
        if (err < 0) { goto out_sock; }
        u16 shard_info_len = get_unaligned_le16(shards_resp_header + sizeof(shards_resp_header) - 2);
        if (shard_info_len != 256) {
            ternfs_info("expected 256 shard infos, got %d", shard_info_len);
            err = -EIO; goto out_sock;
        }
        if (registry_resp_len != 2 + TERNFS_SHARD_INFO_SIZE*256) {
            ternfs_info("expected size of %d, got %d", 2 + TERNFS_SHARD_INFO_SIZE*256, registry_resp_len);
            err = -EIO; goto out_sock;
        }

        int shid;
        for (shid = 0; shid < 256; shid++) {
            char shard_info_resp[TERNFS_SHARD_INFO_SIZE];
            for (read_so_far = 0; read_so_far < TERNFS_SHARD_INFO_SIZE;) {
                iov.iov_base = shard_info_resp + read_so_far;
                iov.iov_len = sizeof(shard_info_resp) - read_so_far;
                int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
                if (read == 0) { err = -ECONNRESET; goto out_sock; }
                if (read < 0) { err = read; goto out_sock; }
                read_so_far += read;
            }
            struct ternfs_bincode_get_ctx ctx = {
                .buf = shard_info_resp,
                .end = shard_info_resp + sizeof(shard_info_resp),
                .err = 0,
            };
            ternfs_shard_info_get_start(&ctx, start);
            ternfs_shard_info_get_addrs(&ctx, start, addr_start);
            ternfs_addrs_info_get_addr1(&ctx, addr_start, ipport1_start);
            ternfs_ip_port_get_addrs(&ctx, ipport1_start, shard_ip1);
            ternfs_ip_port_get_port(&ctx, shard_ip1, shard_port1);
            ternfs_ip_port_get_end(&ctx, shard_port1, ipport1_end);
            ternfs_addrs_info_get_addr2(&ctx, ipport1_end, ipport2_start);
            ternfs_ip_port_get_addrs(&ctx, ipport2_start, shard_ip2);
            ternfs_ip_port_get_port(&ctx, shard_ip2, shard_port2);
            ternfs_ip_port_get_end(&ctx, shard_port2, ipport2_end);
            ternfs_addrs_info_get_end(&ctx, ipport2_end, addr_end);
            ternfs_shard_info_get_last_seen(&ctx, addr_end, last_seen);
            ternfs_shard_info_get_end(&ctx, last_seen, end);
            ternfs_shard_info_get_finish(&ctx, end);
            if (ctx.err != 0) { err = ternfs_error_to_linux(ctx.err); goto out_sock; }

            atomic64_set(&info->shard_addrs1[shid], ternfs_mk_addr(shard_ip1.x, shard_port1.x));
            atomic64_set(&info->shard_addrs2[shid], ternfs_mk_addr(shard_ip2.x, shard_port2.x));
        }
    }
    {
        static_assert(TERNFS_LOCAL_CDC_REQ_SIZE == 0);
        char cdc_req[TERNFS_REGISTRY_REQ_HEADER_SIZE];
        ternfs_write_registry_req_header(cdc_req, TERNFS_LOCAL_CDC_REQ_SIZE, TERNFS_REGISTRY_LOCAL_CDC);
        int written_so_far;
        for (written_so_far = 0; written_so_far < sizeof(cdc_req);) {
            iov.iov_base = cdc_req + written_so_far;
            iov.iov_len = sizeof(cdc_req) - written_so_far;
            int written = kernel_sendmsg(registry_sock, &msg, &iov, 1, iov.iov_len);
            if (written < 0) { err = written; goto out_sock; }
            written_so_far += written;
        }

        char cdc_resp_header[TERNFS_REGISTRY_RESP_HEADER_SIZE];
        int read_so_far;
        for (read_so_far = 0; read_so_far < sizeof(cdc_resp_header);) {
            iov.iov_base = cdc_resp_header + read_so_far;
            iov.iov_len = sizeof(cdc_resp_header) - read_so_far;
            int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
            if (read == 0) { err = -ECONNRESET; goto out_sock; }
            if (read < 0) { err = read; goto out_sock; }
            read_so_far += read;
        }
        u32 registry_resp_len;
        u8 registry_resp_kind;
        err = ternfs_read_registry_resp_header(cdc_resp_header, &registry_resp_len, &registry_resp_kind);
        if (err < 0) { goto out_sock; }
        if (registry_resp_len != TERNFS_LOCAL_CDC_RESP_SIZE) {
            ternfs_debug("expected size of %d, got %d", TERNFS_LOCAL_CDC_RESP_SIZE, registry_resp_len);
            err = -EINVAL; goto out_sock;
        }
        {
            char cdc_resp[TERNFS_LOCAL_CDC_RESP_SIZE];
            for (read_so_far = 0; read_so_far < sizeof(cdc_resp);) {
                iov.iov_base = (char*)&cdc_resp + read_so_far;
                iov.iov_len = sizeof(cdc_resp) - read_so_far;
                int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
                if (read == 0) { err = -ECONNRESET; goto out_sock; }
                if (read < 0) { err = read; goto out_sock; }
                read_so_far += read;
            }
            struct ternfs_bincode_get_ctx ctx = {
                .buf = cdc_resp,
                .end = cdc_resp + sizeof(cdc_resp),
                .err = 0,
            };
            ternfs_local_cdc_resp_get_start(&ctx, start);
            ternfs_local_cdc_resp_get_addrs(&ctx, start, addr_start);
            ternfs_addrs_info_get_addr1(&ctx, addr_start, ipport1_start);
            ternfs_ip_port_get_addrs(&ctx, ipport1_start, cdc_ip1);
            ternfs_ip_port_get_port(&ctx, cdc_ip1, cdc_port1);
            ternfs_ip_port_get_end(&ctx, cdc_port1, ipport1_end);
            ternfs_addrs_info_get_addr2(&ctx, ipport1_end, ipport2_start);
            ternfs_ip_port_get_addrs(&ctx, ipport2_start, cdc_ip2);
            ternfs_ip_port_get_port(&ctx, cdc_ip2, cdc_port2);
            ternfs_ip_port_get_end(&ctx, cdc_port2, ipport2_end);
            ternfs_addrs_info_get_end(&ctx, ipport2_end, addr_end);
            ternfs_local_cdc_resp_get_last_seen(&ctx, addr_end, last_seen);
            ternfs_local_cdc_resp_get_end(&ctx, last_seen, end);
            ternfs_local_cdc_resp_get_finish(&ctx, end);
            if (ctx.err != 0) { err = ternfs_error_to_linux(ctx.err); goto out_sock; }

            atomic64_set(&info->cdc_addr1, ternfs_mk_addr(cdc_ip1.x, cdc_port1.x));
            atomic64_set(&info->cdc_addr2, ternfs_mk_addr(cdc_ip2.x, cdc_port2.x));
        }
    }

    {
        {
            char changed_block_services_req[TERNFS_REGISTRY_REQ_HEADER_SIZE + TERNFS_LOCAL_CHANGED_BLOCK_SERVICES_REQ_SIZE];
            struct ternfs_bincode_put_ctx ctx = {
                .start = changed_block_services_req + TERNFS_REGISTRY_REQ_HEADER_SIZE,
                .cursor = changed_block_services_req + TERNFS_REGISTRY_REQ_HEADER_SIZE,
                .end = changed_block_services_req + sizeof(changed_block_services_req),
            };
            ternfs_local_changed_block_services_req_put_start(&ctx, start);
            ternfs_local_changed_block_services_req_put_changed_since(&ctx, start, changed_since, info->block_services_last_changed_time);
            ternfs_local_changed_block_services_req_put_end(&ctx, changed_since, end);
            ternfs_write_registry_req_header(changed_block_services_req, TERNFS_LOCAL_CHANGED_BLOCK_SERVICES_REQ_SIZE, TERNFS_REGISTRY_LOCAL_CHANGED_BLOCK_SERVICES);
            int written_so_far;
            for (written_so_far = 0; written_so_far < sizeof(changed_block_services_req);) {
                iov.iov_base = changed_block_services_req + written_so_far;
                iov.iov_len = sizeof(changed_block_services_req) - written_so_far;
                int written = kernel_sendmsg(registry_sock, &msg, &iov, 1, iov.iov_len);
                if (written < 0) { err = written; goto out_sock; }
                written_so_far += written;
            }
        }
        u32 registry_resp_len;
        u8 registry_resp_kind;
        {
            char block_services_resp_header[TERNFS_REGISTRY_RESP_HEADER_SIZE];
            int read_so_far;
            for (read_so_far = 0; read_so_far < sizeof(block_services_resp_header);) {
                iov.iov_base = block_services_resp_header + read_so_far;
                iov.iov_len = sizeof(block_services_resp_header) - read_so_far;
                int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
                if (read == 0) { err = -ECONNRESET; goto out_sock; }
                if (read < 0) { err = read; goto out_sock; }
                read_so_far += read;
            }
            err = ternfs_read_registry_resp_header(block_services_resp_header, &registry_resp_len, &registry_resp_kind);
            if (err < 0) { goto out_sock; }
        }
        u64 last_changed;
        u16 block_services_len;
        {
            char last_changed_and_len[sizeof(last_changed) + sizeof(block_services_len)];
            if (registry_resp_len < sizeof(last_changed_and_len)) {
                ternfs_debug("expected size of at least %ld for BlockServicesWithFlagChangeResp, got %d", sizeof(last_changed_and_len), registry_resp_len);
                err = -EINVAL;
                goto out_sock;
            }
            int read_so_far;
            for (read_so_far = 0; read_so_far < sizeof(last_changed_and_len);) {
                iov.iov_base = last_changed_and_len + read_so_far;
                iov.iov_len = sizeof(last_changed_and_len) - read_so_far;
                int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
                if (read == 0) { err = -ECONNRESET; goto out_sock; }
                if (read < 0) { err = read; goto out_sock; }
                read_so_far += read;
            }
            last_changed = get_unaligned_le64(last_changed_and_len);
            block_services_len = get_unaligned_le16(last_changed_and_len + sizeof(last_changed));
            registry_resp_len -= sizeof(last_changed_and_len);
        }
        {
            if (registry_resp_len != TERNFS_BLOCK_SERVICE_SIZE * block_services_len) {
                ternfs_debug("expected size of at least %d for %d BlockServices in BlockServicesWithFlagChangeResp, got %d",
                    TERNFS_BLOCK_SERVICE_SIZE * block_services_len, block_services_len, registry_resp_len);
                err = -EINVAL;
                goto out_sock;
            }
            u16 block_service_idx;
            int read_so_far;
            for (block_service_idx = 0; block_service_idx < block_services_len; block_service_idx++) {
                char block_service_buf[TERNFS_BLOCK_SERVICE_SIZE];
                for (read_so_far = 0; read_so_far < sizeof(block_service_buf);) {
                    iov.iov_base = block_service_buf + read_so_far;
                    iov.iov_len = sizeof(block_service_buf) - read_so_far;
                    int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
                    if (read == 0) { err = -ECONNRESET; goto out_sock; }
                    if (read < 0) { err = read; goto out_sock; }
                    read_so_far += read;
                }
                struct ternfs_bincode_get_ctx bs_ctx = {
                    .buf = block_service_buf,
                    .end = block_service_buf + sizeof(block_service_buf),
                    .err = 0,
                };
                ternfs_block_service_get_start(&bs_ctx, start);
                ternfs_block_service_get_addrs(&bs_ctx, start, addr_start);
                ternfs_addrs_info_get_addr1(&bs_ctx, addr_start, ipport1_start);
                ternfs_ip_port_get_addrs(&bs_ctx, ipport1_start, ip1);
                ternfs_ip_port_get_port(&bs_ctx, ip1, port1);
                ternfs_ip_port_get_end(&bs_ctx, port1, ipport1_end);
                ternfs_addrs_info_get_addr2(&bs_ctx, ipport1_end, ipport2_start);
                ternfs_ip_port_get_addrs(&bs_ctx, ipport2_start, ip2);
                ternfs_ip_port_get_port(&bs_ctx, ip2, port2);
                ternfs_ip_port_get_end(&bs_ctx, port2, ipport2_end);
                ternfs_addrs_info_get_end(&bs_ctx, ipport2_end, addr_end);
                ternfs_block_service_get_id(&bs_ctx, addr_end, bs_id);
                ternfs_block_service_get_flags(&bs_ctx, bs_id, bs_flags);
                ternfs_block_service_get_end(&bs_ctx, bs_flags, end);
                ternfs_block_service_get_finish(&bs_ctx, end);
                if (bs_ctx.err != 0) { err = ternfs_error_to_linux(bs_ctx.err); goto out_sock; }

                struct ternfs_block_service bs;
                bs.id = bs_id.x;
                bs.ip1 = ip1.x;
                bs.port1 = port1.x;
                bs.ip2 = ip2.x;
                bs.port2 = port2.x;
                bs.flags = bs_flags.x;
                struct ternfs_stored_block_service* sbs = ternfs_upsert_block_service(&bs);
                if (IS_ERR(sbs)) {
                    err = PTR_ERR(sbs);
                    goto out_sock;
                }
            }
        }
        info->block_services_last_changed_time = last_changed;
    }
    {
        static_assert(TERNFS_INFO_REQ_SIZE == 0);
        char info_req[TERNFS_REGISTRY_REQ_HEADER_SIZE];
        ternfs_write_registry_req_header(info_req, 0, TERNFS_REGISTRY_INFO);
        int written_so_far;
        for (written_so_far = 0; written_so_far < sizeof(info_req);) {
            iov.iov_base = info_req + written_so_far;
            iov.iov_len = sizeof(info_req) - written_so_far;
            int written = kernel_sendmsg(registry_sock, &msg, &iov, 1, iov.iov_len);
            if (written < 0) { err = written; goto out_sock; }
            written_so_far += written;
        }

        char info_resp_header[TERNFS_REGISTRY_RESP_HEADER_SIZE];
        int read_so_far;
        for (read_so_far = 0; read_so_far < sizeof(info_resp_header);) {
            iov.iov_base = info_resp_header + read_so_far;
            iov.iov_len = sizeof(info_resp_header) - read_so_far;
            int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
            if (read == 0) { err = -ECONNRESET; goto out_sock; }
            if (read < 0) { err = read; goto out_sock; }
            read_so_far += read;
        }
        u32 info_resp_len;
        u8 info_resp_kind;
        err = ternfs_read_registry_resp_header(info_resp_header, &info_resp_len, &info_resp_kind);
        if (err < 0) {
            goto out_sock;
        }
        if (info_resp_len != TERNFS_INFO_RESP_SIZE) {
            ternfs_info("expected size of %d, got %d", TERNFS_INFO_RESP_SIZE, info_resp_len);
            err = -EIO; goto out_sock;
        }
        char info_resp[TERNFS_INFO_RESP_SIZE];
        for (read_so_far = 0; read_so_far < sizeof(info_resp);) {
            iov.iov_base = (char*)&info_resp + read_so_far;
            iov.iov_len = sizeof(info_resp) - read_so_far;
            int read = kernel_recvmsg(registry_sock, &msg, &iov, 1, iov.iov_len, 0);
            if (read == 0) { err = -ECONNRESET; goto out_sock; }
            if (read < 0) { err = read; goto out_sock; }
            read_so_far += read;
        }
        struct ternfs_bincode_get_ctx ctx = {
            .buf = info_resp,
            .end = info_resp + sizeof(info_resp),
            .err = 0,
        };
        ternfs_info_resp_get_start(&ctx, start);
        ternfs_info_resp_get_num_block_services(&ctx, start, num_block_services);
        ternfs_info_resp_get_num_failure_domains(&ctx, num_block_services, num_failure_domains);
        ternfs_info_resp_get_capacity(&ctx, num_failure_domains, capacity);
        ternfs_info_resp_get_available(&ctx, capacity, available);
        ternfs_info_resp_get_blocks(&ctx, available, blocks);
        ternfs_info_resp_get_end(&ctx, blocks, end);
        ternfs_info_resp_get_finish(&ctx, end);
        if (ctx.err != 0) {
            ternfs_info("response error %s (%d)", ternfs_err_str(ctx.err), ctx.err);
            err = ternfs_error_to_linux(ctx.err);
            goto out_sock;
        }
        atomic64_set(&(info->available), available.x);
        atomic64_set(&(info->capacity), capacity.x);
    }

    sock_release(registry_sock);
    return 0;

out_sock:
    sock_release(registry_sock);
    return err;
}

static void ternfs_registry_refresh_work(struct work_struct* work) {
    struct ternfs_fs_info* info = container_of(container_of(work, struct delayed_work, work), struct ternfs_fs_info, registry_refresh_work);
    int err = ternfs_refresh_fs_info(info);
    if (err != 0 && err != -EAGAIN) {
        ternfs_warn("failed to refresh registry data: %d", err);
    }
    ternfs_debug("scheduling registry data refresh after %dms", jiffies_to_msecs(ternfs_registry_refresh_time_jiffies));
    queue_delayed_work(system_long_wq, &info->registry_refresh_work, ternfs_registry_refresh_time_jiffies);
}

enum {
    Opt_err, Opt_uid, Opt_gid, Opt_umask, Opt_dmask, Opt_fmask
};

static const match_table_t tokens = {
    {Opt_uid, "uid=%u"},
    {Opt_gid, "gid=%u"},
    {Opt_umask, "umask=%u"},
    {Opt_dmask, "dmask=%u"},
    {Opt_fmask, "fmask=%u"},
    {Opt_err, NULL}
};

static int ternfs_parse_options(char* options, struct ternfs_fs_info* ternfs_info) {
    char *p;
    int option;
    substring_t args[MAX_OPT_ARGS];

    // defaults
    ternfs_info->uid = make_kuid(&init_user_ns, 1000);
    ternfs_info->gid = make_kgid(&init_user_ns, 1000);
    ternfs_info->dmask = 0000;
    ternfs_info->fmask = 0111;

    if (!options)
        return 0;

    while ((p = strsep(&options, ",")) != NULL) {
        int token;
        if (!*p)
            continue;

        token = match_token(p, tokens, args);
        switch (token) {
        case Opt_uid:
            if (match_int(&args[0], &option))
                return -EINVAL;
            ternfs_info->uid = make_kuid(&init_user_ns, option);
            if (!uid_valid(ternfs_info->uid))
                return -EINVAL;
            break;
        case Opt_gid:
            if (match_int(&args[0], &option))
                return -EINVAL;
            ternfs_info->gid = make_kgid(&init_user_ns, option);
            if (!gid_valid(ternfs_info->gid))
                return -EINVAL;
            break;
        case Opt_umask:
            if (match_octal(&args[0], &option))
                return -EINVAL;
            ternfs_info->fmask = ternfs_info->dmask = option;
            break;
        case Opt_dmask:
            if (match_octal(&args[0], &option))
                return -EINVAL;
            ternfs_info->dmask = option;
            break;
        case Opt_fmask:
            if (match_octal(&args[0], &option))
                return -EINVAL;
            ternfs_info->fmask = option;
            break;
        default:
            ternfs_error("Invalid mount option \"%s\"", p);
            return -EINVAL;
        }
    }

    return 0;
}

static struct ternfs_fs_info* ternfs_init_fs_info(struct net* net, const char* dev_name, char* options) {
    int err;

    struct ternfs_fs_info* ternfs_info = kzalloc(sizeof(struct ternfs_fs_info), GFP_KERNEL);
    if (!ternfs_info) { err = -ENOMEM; goto out; }

    ternfs_info->registry_addr.net = get_net(net);
    ternfs_info->registry_addr.addr = kmalloc(strlen(dev_name)+1, GFP_KERNEL);
    if (!ternfs_info->registry_addr.addr) {
        err = -ENOMEM;
        goto out_info;
    }
    memcpy(ternfs_info->registry_addr.addr, dev_name, strlen(dev_name)+1);

    err = ternfs_parse_options(options, ternfs_info);
    if (err) { goto out_addr; }

    err = ternfs_init_shard_socket(&ternfs_info->sock);
    if (err) { goto out_addr; }

    // for the first update we will ask for everything that changed in last day.
    // this is more than enough time for any older changed to be visible to shards and propagated through block info
    u64 atime_ns = ktime_get_real_ns();
    atime_ns -= min(atime_ns, 86400000000000ull);
    ternfs_info->block_services_last_changed_time = atime_ns;

    err = ternfs_refresh_fs_info(ternfs_info);
    if (err != 0) { goto out_socket; }

    INIT_DELAYED_WORK(&ternfs_info->registry_refresh_work, ternfs_registry_refresh_work);
    queue_delayed_work(system_long_wq, &ternfs_info->registry_refresh_work, ternfs_registry_refresh_time_jiffies);

    return ternfs_info;

out_socket:
    ternfs_net_shard_free_socket(&ternfs_info->sock);
out_addr:
    kfree(ternfs_info->registry_addr.addr);
out_info:
    put_net(ternfs_info->registry_addr.net);
    kfree(ternfs_info);
out:
    return ERR_PTR(err);
}

static void ternfs_put_super(struct super_block* sb) {
    ternfs_debug("sb=%p", sb);
    ternfs_free_fs_info(sb->s_fs_info);
    sb->s_fs_info = NULL;
}

#define TERNFS_SUPER_MAGIC 0x45474753 // EGGS

static int ternfs_statfs(struct dentry* dentry, struct kstatfs* stats) {
    struct ternfs_fs_info* info = (struct ternfs_fs_info*)dentry->d_sb->s_fs_info;

    stats->f_type = TERNFS_SUPER_MAGIC;
    stats->f_bsize = PAGE_SIZE;
    stats->f_frsize = PAGE_SIZE;
    stats->f_blocks = atomic64_read(&info->capacity) / PAGE_SIZE;
    stats->f_bfree = atomic64_read(&info->available) / PAGE_SIZE;
    stats->f_bavail = atomic64_read(&info->available) / PAGE_SIZE;
    stats->f_files = -1;
    stats->f_ffree = -1;
    stats->f_namelen = TERNFS_MAX_FILENAME;
    return 0;
}

static const struct super_operations ternfs_super_ops = {
    .alloc_inode = ternfs_inode_alloc,
    .evict_inode = ternfs_inode_evict,
    .free_inode = ternfs_inode_free,

    .put_super = ternfs_put_super,

    .statfs = ternfs_statfs,
};

static struct dentry* ternfs_mount(struct file_system_type* fs_type, int flags, const char* dev_name, void* data) {
    int err;

    ternfs_info("mounting at %s", dev_name);
    ternfs_debug("fs_type=%p flags=%d dev_name=%s data=%s", fs_type, flags, dev_name, data ? (char*)data : "");

    struct ternfs_fs_info* info = ternfs_init_fs_info(current->nsproxy->net_ns, dev_name, data);
    if (IS_ERR(info)) { err = PTR_ERR(info); goto out_err; }

    struct super_block* sb = sget(fs_type, NULL, set_anon_super, flags, NULL);
    if (IS_ERR(sb)) { err = PTR_ERR(sb); goto out_info; }

    sb->s_fs_info = info;

    sb->s_flags = SB_NOSUID | SB_NODEV | SB_NOEXEC | SB_NOATIME | SB_NODIRATIME;
    sb->s_iflags = SB_I_NOEXEC | SB_I_NODEV;

    sb->s_op = &ternfs_super_ops;
    sb->s_d_op = &ternfs_dentry_ops;
    sb->s_export_op = &ternfs_export_ops;

    sb->s_time_gran = 1;
    sb->s_time_min = 0;
    sb->s_time_max = U64_MAX/1000000000ull;
    sb->s_maxbytes = MAX_LFS_FILESIZE;
    sb->s_blocksize = PAGE_SIZE;
    sb->s_magic = TERNFS_SUPER_MAGIC;
    sb->s_blocksize_bits = ffs(sb->s_blocksize) - 1;

    sb->s_bdi = &noop_backing_dev_info;
    sb->s_bdi->ra_pages = ternfs_readahead_pages;

    struct inode* root = ternfs_get_inode_normal(sb, NULL, TERNFS_ROOT_INODE);
    if (IS_ERR(root)) { err = PTR_ERR(root); goto out_sb; }

    struct ternfs_inode* root_enode = TERNFS_I(root);

    err = ternfs_do_getattr(root_enode, ATTR_CACHE_NORM_TIMEOUT);
    if (err) { goto out_sb; }

    if (!root_enode->block_policy || !root_enode->span_policy || !root_enode->stripe_policy) {
        ternfs_warn("no policies for root directory!");
        err = -EIO;
        goto out_sb;
    }

    sb->s_root = d_make_root(root);
    if (!sb->s_root) { err = -ENOMEM; goto out_sb; }

    sb->s_flags |= SB_ACTIVE;

    return dget(sb->s_root);

out_sb:
    deactivate_locked_super(sb);
out_info:
    ternfs_free_fs_info(info);
out_err:
    ternfs_debug("failed err=%d", err);
    return ERR_PTR(err);
}

static void ternfs_kill_sb(struct super_block* sb) {
    ternfs_debug("sb=%p", sb);
    kill_anon_super(sb);
}

static struct file_system_type ternfs_fs_type = {
    .owner = THIS_MODULE,
    .name = "eggsfs",
    .mount = ternfs_mount,
    .kill_sb = ternfs_kill_sb,
    // TODO is FS_RENAME_DOES_D_MOVE needed?
    .fs_flags = FS_RENAME_DOES_D_MOVE,
};
MODULE_ALIAS_FS("eggsfs");

int __init ternfs_fs_init(void) {
    return register_filesystem(&ternfs_fs_type);
}

void __cold ternfs_fs_exit(void) {
    ternfs_debug("fs exit");
    unregister_filesystem(&ternfs_fs_type);
}

