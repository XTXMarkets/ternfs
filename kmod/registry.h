// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_REGISTRY_H
#define _TERNFS_REGISTRY_H

#include <linux/kernel.h>
#include <linux/inet.h>

#define TERNFS_REGISTRY_REQ_HEADER_SIZE (4 + 4 + 1) // protocol + len + kind
void ternfs_write_registry_req_header(char* buf, u32 req_len, u8 req_kind);

#define TERNFS_REGISTRY_RESP_HEADER_SIZE (4 + 4 + 1) // protocol + len + kind
int ternfs_read_registry_resp_header(char* buf, u32* resp_len, u8* resp_kind);

struct ternfs_registry_addr {
    struct net* net;
    char* addr;
};

int ternfs_process_registry_addr(struct ternfs_registry_addr* addr, struct sockaddr_in* addr1, struct sockaddr_in* addr2);

int ternfs_create_registry_socket(struct ternfs_registry_addr* addr, struct socket** sock);

#endif
