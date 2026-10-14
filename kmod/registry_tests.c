// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "registry_dns.h"

#define CHECK(x) \
    do { \
        if (!(x)) { \
            fprintf(stderr, "check failed at line %d: %s\n", __LINE__, #x); \
            exit(1); \
        } \
    } while (0)

static void check_token(
    const struct ternfs_registry_dns_token* token,
    const char* expected
) {
    CHECK(token->len == strlen(expected));
    CHECK(memcmp(token->data, expected, token->len) == 0);
}

int main(void) {
    struct ternfs_registry_dns_token tokens[TERNFS_MAX_REGISTRY_ADDRS];
    size_t count;

    count = ternfs_registry_dns_tokens(
        "192.0.2.1",
        strlen("192.0.2.1"),
        tokens,
        TERNFS_MAX_REGISTRY_ADDRS
    );
    CHECK(count == 1);
    check_token(&tokens[0], "192.0.2.1");

    count = ternfs_registry_dns_tokens(
        "192.0.2.1,198.51.100.2",
        strlen("192.0.2.1,198.51.100.2"),
        tokens,
        TERNFS_MAX_REGISTRY_ADDRS
    );
    CHECK(count == 2);
    check_token(&tokens[0], "192.0.2.1");
    check_token(&tokens[1], "198.51.100.2");

    count = ternfs_registry_dns_tokens(
        "192.0.2.1,198.51.100.2,203.0.113.3",
        strlen("192.0.2.1,198.51.100.2,203.0.113.3"),
        tokens,
        TERNFS_MAX_REGISTRY_ADDRS
    );
    CHECK(count == 2);
    check_token(&tokens[0], "192.0.2.1");
    check_token(&tokens[1], "198.51.100.2");

    CHECK(ternfs_registry_dns_tokens("", 0, tokens, TERNFS_MAX_REGISTRY_ADDRS) == 0);
    CHECK(ternfs_registry_dns_tokens(",192.0.2.1", strlen(",192.0.2.1"), tokens, TERNFS_MAX_REGISTRY_ADDRS) == 0);
    CHECK(ternfs_registry_dns_tokens("192.0.2.1,", strlen("192.0.2.1,"), tokens, TERNFS_MAX_REGISTRY_ADDRS) == 0);
    CHECK(ternfs_registry_dns_tokens("192.0.2.1,,198.51.100.2", strlen("192.0.2.1,,198.51.100.2"), tokens, TERNFS_MAX_REGISTRY_ADDRS) == 0);

    printf("All registry tests pass.\n");
    return 0;
}
