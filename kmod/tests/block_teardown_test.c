// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/interrupt.h>
#include <linux/module.h>
#include <linux/net.h>
#include <linux/slab.h>
#include <linux/workqueue.h>
#include <net/sock.h>

#include "../block.h"
#include "../err.h"
#include "../file.h"
#include "../log.h"
#include "../net_compat.h"
#include "../page_compat.h"
#include "../super.h"
#include "../wq.h"

static unsigned cache_create_count;
static unsigned fail_cache_create;
static unsigned cache_destroy_count;
static void* tracked_socket;
static atomic_t tracked_socket_frees = ATOMIC_INIT(0);
static atomic_t socket_releases = ATOMIC_INIT(0);
static atomic_t request_completions = ATOMIC_INIT(0);
static atomic_t teardown_order_bad = ATOMIC_INIT(0);
static atomic_t release_context_bad = ATOMIC_INIT(0);
static bool check_teardown_order;

static struct kmem_cache* test_kmem_cache_create(
    const char* name, unsigned int size, unsigned int align,
    slab_flags_t flags, void (*ctor)(void*)
) {
    cache_create_count++;
    if (cache_create_count == fail_cache_create) { return NULL; }
    return kmem_cache_create(name, size, align, flags, ctor);
}

static void test_kmem_cache_destroy(struct kmem_cache* cache) {
    if (check_teardown_order &&
        (atomic_read(&tracked_socket_frees) != 1 ||
         atomic_read(&socket_releases) != 1 ||
         atomic_read(&request_completions) != 1)) {
        pr_err(
            "ternfs-block-teardown-test: destroy order socket_free=%d socket_release=%d completion=%d\n",
            atomic_read(&tracked_socket_frees),
            atomic_read(&socket_releases),
            atomic_read(&request_completions)
        );
        atomic_set(&teardown_order_bad, 1);
    }
    cache_destroy_count++;
    kmem_cache_destroy(cache);
}

static void test_kfree(const void* ptr) {
    if (ptr == tracked_socket) {
        atomic_inc(&tracked_socket_frees);
    }
    kfree(ptr);
}

static void test_sock_release(struct socket* sock) {
    if (in_interrupt() || irqs_disabled()) {
        atomic_set(&release_context_bad, 1);
    }
    atomic_inc(&socket_releases);
    sock_release(sock);
}

// Only the block-write trace boundary is used by block.c. Avoid defining a
// second tracepoint provider in this standalone test module.
#define _TRACE_EGGFS_H
#define TERNFS_BLOCK_WRITE_QUEUED 0
#define TERNFS_BLOCK_WRITE_RECV_ENTER 1
#define TERNFS_BLOCK_WRITE_RECV_EXIT 2
#define TERNFS_BLOCK_WRITE_WRITE_ENTER 3
#define TERNFS_BLOCK_WRITE_WRITE_EXIT 4
#define TERNFS_BLOCK_WRITE_WRITE_QUEUED 5
#define TERNFS_BLOCK_WRITE_DONE 6
#define TERNFS_BLOCK_WRITE_START 7
#define TERNFS_BLOCK_WRITE_COMPLETE_QUEUED 8
#define trace_eggsfs_block_write(...) do {} while (0)

// Scope lifecycle observation and allocation failure injection to this copy
// of production block.c. The wrappers still call the real kernel operations.
#undef kmem_cache_create
#define kmem_cache_create(name, size, align, flags, ctor) \
    test_kmem_cache_create(name, size, align, flags, ctor)
#undef kmem_cache_destroy
#define kmem_cache_destroy(cache) test_kmem_cache_destroy(cache)
#undef kfree
#define kfree(ptr) test_kfree(ptr)
#undef sock_release
#define sock_release(sock) test_sock_release(sock)
#include "../block.c"
#undef sock_release
#undef kfree
#undef kmem_cache_destroy
#undef kmem_cache_create

MODULE_LICENSE("GPL");

int ternfs_debug_output;
struct workqueue_struct* ternfs_wq;
struct workqueue_struct* ternfs_fast_wq;

int ternfs_error_to_linux(int err) {
    return err <= 0 ? err : -EIO;
}

struct test_request {
    struct block_request breq;
};

static struct test_request pending_request;

static void test_request_complete(struct work_struct* work) {
    struct test_request* req = container_of(
        work, struct test_request, breq.complete_work
    );
    if (atomic_inc_return(&request_completions) != 1 ||
        atomic_read(&req->breq.err) != -ECONNABORTED ||
        in_interrupt()) {
        atomic_set(&teardown_order_bad, 1);
    }
}

static void init_pending_request(struct block_socket* socket) {
    int i;
    memset(&pending_request, 0, sizeof(pending_request));
    INIT_LIST_HEAD(&pending_request.breq.read_list);
    INIT_LIST_HEAD(&pending_request.breq.write_list);
    INIT_WORK(&pending_request.breq.complete_work, test_request_complete);
    atomic_set(&pending_request.breq.err, 0);
    atomic_set(&pending_request.breq.state, BR_ST_QUEUEING);
    for (i = 0; i < BR_ST_MAX; i++) {
        atomic64_set(&pending_request.breq.timings[i], get_jiffies_64());
    }
    pending_request.breq.left_to_read = 1;
    pending_request.breq.left_to_write = 1;
    list_add_tail(&pending_request.breq.read_list, &socket->read);
    list_add_tail(&pending_request.breq.write_list, &socket->write);
}

static struct block_socket* add_test_socket(struct block_ops* ops) {
    struct block_socket* socket;
    int bucket;
    int err;

    socket = kzalloc(sizeof(*socket), GFP_KERNEL);
    if (!socket) { return ERR_PTR(-ENOMEM); }
    err = sock_create_kern(
        &init_net, PF_INET, SOCK_STREAM, IPPROTO_TCP, &socket->sock
    );
    if (err) {
        kfree(socket);
        return ERR_PTR(err);
    }

    socket->addr.sin_family = AF_INET;
    socket->addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    socket->addr.sin_port = htons(49151);
    socket->last_read_activity = get_jiffies_64();
    socket->last_write_activity = get_jiffies_64();
    atomic_set(&socket->err, 0);
    init_rcu_head(&socket->rcu_list);
    INIT_WORK(&socket->release_work, block_socket_release_work);
    spin_lock_init(&socket->list_lock);
    INIT_LIST_HEAD(&socket->write);
    INIT_WORK(&socket->write_work, ops->write_work);
    atomic_set(&socket->write_work_active, 0);
    atomic_set(&socket->refcount, 1);
    socket->terminal = false;
    INIT_LIST_HEAD(&socket->read);
    init_completion(&socket->sock_wait);
    init_pending_request(socket);

    write_lock_bh(&socket->sock->sk->sk_callback_lock);
    socket->saved_state_change = socket->sock->sk->sk_state_change;
    socket->saved_data_ready = socket->sock->sk->sk_data_ready;
    socket->saved_write_space = socket->sock->sk->sk_write_space;
    socket->sock->sk->sk_user_data = socket;
    socket->sock->sk->sk_state_change = block_socket_sk_state_change;
    socket->sock->sk->sk_data_ready = ops->sk_data_ready;
    socket->sock->sk->sk_write_space = ops->sk_write_space;
    write_unlock_bh(&socket->sock->sk->sk_callback_lock);

    bucket = hash_min(block_socket_key(&socket->addr), BLOCK_SOCKET_BITS);
    spin_lock(&ops->locks[bucket]);
    hlist_add_head_rcu(&socket->hnode, &ops->sockets[bucket]);
    atomic_inc(&ops->len[bucket]);
    spin_unlock(&ops->locks[bucket]);
    return socket;
}

static bool tables_empty(struct block_ops* ops) {
    int bucket;
    for (bucket = 0; bucket < BLOCK_SOCKET_BUCKETS; bucket++) {
        if (atomic_read(&ops->len[bucket]) != 0 ||
            !hlist_empty(&ops->sockets[bucket])) {
            return false;
        }
    }
    return true;
}

#define CHECK(condition, description) do { \
    if (!(condition)) { \
        pr_err("ternfs-block-teardown-test: FAIL: %s\n", description); \
        err = -EINVAL; \
        goto out; \
    } \
} while (0)

static int __init block_teardown_test_init(void) {
    struct block_socket* socket;
    struct block_request stopped_request;
    struct ternfs_block_service service = { .id = 0x1234 };
    unsigned destroys_before;
    bool block_active = false;
    int err = 0;

    ternfs_wq = alloc_workqueue("ternfs-block-test-wq", WQ_UNBOUND, 0);
    if (!ternfs_wq) { return -ENOMEM; }
    ternfs_fast_wq = alloc_workqueue(
        "ternfs-block-test-fast-wq", WQ_UNBOUND, 0
    );
    if (!ternfs_fast_wq) {
        destroy_workqueue(ternfs_wq);
        return -ENOMEM;
    }

    fail_cache_create = cache_create_count + 1;
    CHECK(ternfs_block_init() == -ENOMEM, "first cache failure");
    CHECK(cache_destroy_count == 0, "first cache failure destroyed a cache");

    fail_cache_create = cache_create_count + 2;
    destroys_before = cache_destroy_count;
    CHECK(ternfs_block_init() == -ENOMEM, "second cache failure");
    CHECK(cache_destroy_count == destroys_before + 1,
          "second cache failure did not unwind first cache");

    fail_cache_create = 0;
    CHECK(ternfs_block_init() == 0, "block init");
    block_active = true;
    flush_workqueue(ternfs_fast_wq);
    CHECK(delayed_work_pending(&timeout_work), "socket timer did not arm");
    CHECK(delayed_work_pending(&bad_addrs_reap_work),
          "bad-address timer did not arm");

    socket = add_test_socket(&fetch_block_pages_witch_crc_ops);
    CHECK(!IS_ERR(socket), "create real test socket");
    tracked_socket = socket;
    mark_bad_addr(htonl(INADDR_LOOPBACK), htons(49152));
    CHECK(atomic_read(&bad_addrs_live) == 1, "bad-address fixture");

    destroys_before = cache_destroy_count;
    check_teardown_order = true;
    ternfs_block_exit();
    block_active = false;
    check_teardown_order = false;

    CHECK(atomic_read(&tracked_socket_frees) == 1, "socket not freed exactly once");
    CHECK(atomic_read(&socket_releases) == 1, "kernel socket not released exactly once");
    CHECK(atomic_read(&request_completions) == 1,
          "pending request not completed exactly once");
    CHECK(!atomic_read(&release_context_bad), "socket released outside process context");
    CHECK(!atomic_read(&teardown_order_bad),
          "cache destroyed before socket/request completion");
    CHECK(cache_destroy_count == destroys_before + 2,
          "request caches not destroyed exactly once");
    CHECK(tables_empty(&fetch_block_pages_witch_crc_ops), "fetch table not empty");
    CHECK(tables_empty(&write_block_ops), "write table not empty");
    CHECK(!delayed_work_pending(&timeout_work), "socket timer still pending");
    CHECK(!delayed_work_pending(&bad_addrs_reap_work),
          "bad-address timer still pending");
    CHECK(atomic_read(&bad_addrs_live) == 0 && bad_addrs_count == 0,
          "bad-address RCU entries remain");

    memset(&stopped_request, 0, sizeof(stopped_request));
    INIT_LIST_HEAD(&stopped_request.read_list);
    INIT_LIST_HEAD(&stopped_request.write_list);
    CHECK(block_socket_start_req(
        &fetch_block_pages_witch_crc_ops, &stopped_request, &service, 1, 1
    ) == -ESHUTDOWN, "stopped retry was not rejected");

    destroys_before = cache_destroy_count;
    CHECK(ternfs_block_init() == 0, "second block init");
    block_active = true;
    flush_workqueue(ternfs_fast_wq);
    ternfs_block_exit();
    block_active = false;
    CHECK(cache_destroy_count == destroys_before + 2, "idle cache teardown");
    CHECK(tables_empty(&fetch_block_pages_witch_crc_ops), "idle fetch table");
    CHECK(tables_empty(&write_block_ops), "idle write table");
    CHECK(!delayed_work_pending(&timeout_work), "idle socket timer");
    CHECK(!delayed_work_pending(&bad_addrs_reap_work), "idle bad-address timer");

out:
    check_teardown_order = false;
    fail_cache_create = 0;
    if (block_active) { ternfs_block_exit(); }
    rcu_barrier();
    destroy_workqueue(ternfs_fast_wq);
    destroy_workqueue(ternfs_wq);
    if (!err) {
        pr_info("ternfs-block-teardown-test: PASS (RCU, socket, completion, timers, stop, unwind)\n");
    }
    return err;
}

static void __exit block_teardown_test_exit(void) {}

module_init(block_teardown_test_init);
module_exit(block_teardown_test_exit);
