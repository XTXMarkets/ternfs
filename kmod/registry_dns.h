// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_REGISTRY_DNS_H
#define _TERNFS_REGISTRY_DNS_H

#ifdef __KERNEL__
#include <linux/string.h>
#include <linux/types.h>
#else
#include <stddef.h>
#include <string.h>
#endif

#define TERNFS_MAX_REGISTRY_ADDRS 2

struct ternfs_registry_dns_token {
    const char* data;
    size_t len;
};

/*
 * Split and validate a comma-separated DNS resolver payload.  The resolver
 * may return more addresses than the connection code can use; retain the
 * first max_tokens while still validating the complete payload.
 */
static inline size_t ternfs_registry_dns_tokens(
    const char* result,
    size_t result_len,
    struct ternfs_registry_dns_token* tokens,
    size_t max_tokens
) {
    const char* cursor;
    const char* end;
    size_t num_tokens = 0;

    if (!result || result_len == 0 || !tokens || max_tokens == 0) {
        return 0;
    }

    cursor = result;
    end = result + result_len;
    while (cursor < end) {
        const char* comma = memchr(cursor, ',', end - cursor);
        const char* token_end = comma ? comma : end;

        if (token_end == cursor) {
            return 0;
        }
        if (num_tokens < max_tokens) {
            tokens[num_tokens].data = cursor;
            tokens[num_tokens].len = token_end - cursor;
        }
        num_tokens++;

        if (!comma) {
            break;
        }
        cursor = comma + 1;
        if (cursor == end) {
            return 0;
        }
    }

    return num_tokens < max_tokens ? num_tokens : max_tokens;
}

#endif
