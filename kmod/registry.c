// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/version.h>
#if (LINUX_VERSION_CODE < KERNEL_VERSION(6, 12, 0))
#include <asm/unaligned.h>
#else
#include <linux/unaligned.h>
#endif
#include <linux/module.h>
#include <linux/dns_resolver.h>
#include <linux/sunrpc/addr.h>

#include "bincode.h"
#include "registry.h"
#include "log.h"
#include "err.h"
#include "super.h"
#include "net_compat.h"

void ternfs_write_registry_req_header(char* buf, u32 req_len, u8 req_kind) {
    put_unaligned_le32(TERNFS_REGISTRY_REQ_PROTOCOL_VERSION, buf); buf += 4;
    // automatically include the kind, much nicer for the caller
    put_unaligned_le32(req_len+1, buf); buf += 4;
    *(u8*)buf = req_kind; buf++;
}

int ternfs_read_registry_resp_header(char* buf, u32* resp_len, u8* resp_kind) {
    u32 protocol = get_unaligned_le32(buf); buf += 4;
    if (protocol != TERNFS_REGISTRY_RESP_PROTOCOL_VERSION) {
        ternfs_warn("bad registry protocol, expected 0x%08x, got 0x%08x", TERNFS_REGISTRY_RESP_PROTOCOL_VERSION, protocol);
        return -EINVAL;
    }
    *resp_len = get_unaligned_le32(buf); buf += 4;
    if (*resp_len == 0) {
        ternfs_warn("unexpected zero-length registry response (the kind should at least be there)");
        return -EINVAL;
    }
    *resp_kind = *(u8*)buf; buf++;
    ternfs_debug("resp_len=%d, resp_kind=0x%02x", *resp_len, (int)*resp_kind);
    (*resp_len)--; // exclude the kind, it's much nicer for the caller
    return 0;
}

static int ternfs_parse_ipv4_addr(const char* str, int len, struct sockaddr_in* addr) {
    int err;
    const char* addr_end;

    ternfs_debug("parsing registry address %*pE", len, str);

    // parse device, which is the registry address in 0.0.0.0:0 form (ipv4, port)

    if (in4_pton(str, len, (__force u8*)&addr->sin_addr, ':', &addr_end) != 1) {
        return -EINVAL;
    }

    u16 port;
    {
        char port_str[6]; // max port + terminating byte
        int port_len = len - (addr_end-str) - 1;
        if (port_len <= 0 || port_len >= sizeof(port_str)) {
            ternfs_warn("bad port in address %*pE (port_len=%d)", len, str, port_len);
            return -EINVAL;
        }
        memcpy(port_str, addr_end+1, port_len);
        port_str[port_len] = '\0';
        err = kstrtou16(port_str, 10, &port);
        if (err < 0) {
            ternfs_warn("bad port in address %*pE", len, str);
            return err;
        }
        if (port == 0) {
            ternfs_warn("zero port in address %*pE", len, str);
            return -EINVAL;
        }
        addr->sin_port = htons(port);
    }

    addr->sin_family = AF_INET;
    ternfs_debug("parsed registry addr %pI4:%d", &addr->sin_addr, port);

    return 0;
}

static int ternfs_resolve_domain_name_addr(struct ternfs_registry_addr* saddr, struct sockaddr_in* addr) {
    const char* domain_name_end = strchr(saddr->addr, ':');
    if (domain_name_end == NULL) {
        ternfs_info("registry address %s does not have port", saddr->addr);
        return -EINVAL;
    }

    ternfs_debug("resolving address %s", saddr->addr);

    char* ip_addr = NULL;
    int ip_len = dns_query(
        saddr->net,
        NULL,
        saddr->addr,
        domain_name_end - saddr->addr,
        NULL,
        &ip_addr,
        NULL,
        false
    );
    if (ip_len < 0) {
        ternfs_info("could not resolve registry address %s: %d", saddr->addr, ip_len);
        return ip_len;
    }
    size_t addrsz = rpc_pton(saddr->net, ip_addr, ip_len, (struct sockaddr*)addr, sizeof(*addr));
    kfree(ip_addr);
    if (addrsz == 0) {
        ternfs_info("could not parse DNS query result for registry address %s: %d", saddr->addr, ip_len);
        return -EINVAL;
    }

    u16 port;
    {
        int err = kstrtou16(domain_name_end+1, 10, &port);
        if (err < 0) {
            return err;
        }
    }
    addr->sin_port = htons(port);

    ternfs_debug("resolved address %s to %pI4:%d", saddr->addr, &addr->sin_addr, ntohs(addr->sin_port));

    return 0;
}

int ternfs_process_registry_addr(struct ternfs_registry_addr* saddr, struct sockaddr_in* addr1, struct sockaddr_in* addr2) {
    memset(addr1, 0, sizeof(*addr1));
    memset(addr2, 0, sizeof(*addr2));

    size_t addr_len = strlen(saddr->addr);
    const char* addr1_end = strchr(saddr->addr, ',');
    if (addr1_end == NULL) { // only one address
        int err = ternfs_parse_ipv4_addr(saddr->addr, addr_len, addr1);
        if (err == -EINVAL) {
            return ternfs_resolve_domain_name_addr(saddr, addr1);
        }
        return err;
    } else if (saddr->addr + addr_len == addr1_end) {
        return -EINVAL; // terminated with :
    } else {
        int err = ternfs_parse_ipv4_addr(saddr->addr, addr1_end - saddr->addr, addr1);
        if (err) {
            return err;
        }
        return ternfs_parse_ipv4_addr(addr1_end+1, addr_len - (addr1_end-saddr->addr) - 1, addr2);
    }
}

int ternfs_create_registry_socket(struct ternfs_registry_addr* saddr, struct socket** sock) {
    int err;

    struct sockaddr_in addrs[2];
    memset(&addrs, 0, sizeof(addrs));

    err = ternfs_process_registry_addr(saddr, &addrs[0], &addrs[1]);
    if (err) {
        return err;
    }

    // create socket
    err = sock_create_kern(&init_net, PF_INET, SOCK_STREAM, IPPROTO_TCP, sock);
    if (err < 0) {
        ternfs_warn("could not create registry socket: %d", err);
        return err;
    }

    struct __kernel_sock_timeval tv;
    tv.tv_sec = 10;
    tv.tv_usec = 0;
    err = COMPAT_SET_SOCKOPT(*sock, SOL_SOCKET, SO_RCVTIMEO_NEW, &tv, sizeof(tv));
    if (err < 0) {
        ternfs_warn("could not set receive timeout on registry socket: %d", err);
        goto out_sock;
    }
    err = COMPAT_SET_SOCKOPT(*sock, SOL_SOCKET, SO_SNDTIMEO_NEW, &tv, sizeof(tv));
    if (err < 0) {
        ternfs_warn("could not set send timeout on registry socket: %d", err);
        goto out_sock;
    }

    int syn_count = 3;
    err = COMPAT_SET_SOCKOPT(*sock, SOL_TCP, TCP_SYNCNT, &syn_count, sizeof(syn_count));
    if (err < 0) {
        ternfs_warn("could not set TCP_SYNCNT=%d on registry socket: %d", syn_count, err);
        goto out_sock;
    }

    u64 start = get_jiffies_64();
    u64 i;
    for (i = start; i < start+2; i++) {
        int ix = i%2;
        struct sockaddr_in* addr = &addrs[ix];
        if (addr->sin_port == 0) {
            ternfs_debug("skipping zero port address val %pI4:%d", &addr->sin_addr, ntohs(addr->sin_port));
            continue;
        }
        ternfs_debug("connecting to address %pI4:%d", &addr->sin_addr, ntohs(addr->sin_port));
        err = kernel_connect(*sock, (struct sockaddr*)addr, sizeof(*addr), 0);
        if (err < 0) {
            ternfs_warn("could not connect to registry addr %pI4:%d, might retry", &addr->sin_addr, ntohs(addr->sin_port));
            continue;
        }
        ternfs_debug("connected to registry");

        return 0;
    }

out_sock:
    // this would only happens with two zero addresses,
    // which we guard against elsewhere in the code.
    BUG_ON(err == 0);
    sock_release(*sock);
    return err;
}
