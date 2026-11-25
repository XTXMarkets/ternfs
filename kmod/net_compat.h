// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_NET_COMPAT_H
#define _TERNFS_NET_COMPAT_H

#include <linux/sockptr.h>

static inline ssize_t ternfs_sendpage(struct socket *sock, struct page *page,
                                     int offset, size_t size, int flags)
{
    struct msghdr msg = {};
    struct kvec iov = {};
    ssize_t result;

    void *page_addr = kmap(page);
    iov.iov_base = page_addr + offset;
    iov.iov_len = size;
    if (likely(sendpage_ok(page))) {
      msg.msg_flags = MSG_SPLICE_PAGES | flags;
    } else {
      msg.msg_flags = flags;
    }

    result = kernel_sendmsg(sock, &msg, &iov, 1, size);

    kunmap(page);

    return result;
}

#endif /* _TERNFS_NET_COMPAT_H */
