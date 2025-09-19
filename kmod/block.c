// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <asm-generic/errno.h>
#include <linux/inet.h>
#include <net/tcp.h>
#include <net/sock.h>
#include <linux/atomic.h>
#include <linux/workqueue.h>
#include <linux/completion.h>
#include <linux/prandom.h>

#include "block.h"
#include "log.h"
#include "err.h"
#include "super.h"
#include "crc.h"
#include "trace.h"
#include "wq.h"
#include "file.h"
#include "net_compat.h"

// Static data
// --------------------------------------------------------------------

#define TERNFS_BLOCKS_REQ_HEADER_SIZE (4 + 8 + 1) // protocol + block service id + kind
#define TERNFS_BLOCKS_RESP_HEADER_SIZE (4 + 1) // protocol + kind

#define MSECS_TO_JIFFIES(_ms) ((_ms * HZ) / 1000)

#define PAGE_CRC_BYTES 4

int ternfs_fetch_block_timeout_jiffies = MSECS_TO_JIFFIES(60 * 1000);
int ternfs_write_block_timeout_jiffies = MSECS_TO_JIFFIES(60 * 1000);
int ternfs_block_service_connect_timeout_jiffies = MSECS_TO_JIFFIES(4 * 1000);

static u64 WHICH_BLOCK_IP = 0;

static struct kmem_cache* fetch_request_cachep;
static struct kmem_cache* write_request_cachep;

// two 256 elements arrays (we have ~100 disks per block service, so
// with one bucket per element we can hold ~25k disks comfortably)
#define BLOCK_SOCKET_BITS 8
#define BLOCK_SOCKET_BUCKETS (1<<BLOCK_SOCKET_BITS)

static u64 block_socket_key(struct sockaddr_in* addr) {
    return ((__force u64)addr->sin_addr.s_addr << 16) | (__force u64)addr->sin_port;
}

struct block_socket;
struct block_request;

struct block_ops {
    DECLARE_HASHTABLE(sockets, BLOCK_SOCKET_BITS);
    spinlock_t locks[BLOCK_SOCKET_BUCKETS];
    // these two are just to detect when a socket is gone
    wait_queue_head_t wqs[BLOCK_SOCKET_BUCKETS];
    atomic_t len[BLOCK_SOCKET_BUCKETS];

    int* timeout_jiffies;

    // functions
    void (*sk_write_space)(struct sock *sk);
    void (*sk_data_ready)(struct sock *sk);
    void (*write_work)(struct work_struct* work);
    void (*cleanup_work)(struct work_struct* work);
    int (*write_req)(struct block_socket* socket, struct block_request* req);
    int (*receive_req)(read_descriptor_t* rd_desc, struct sk_buff* skb, unsigned int offset, size_t len);
};

static void block_socket_sk_write_space(struct sock *sk);

static void fetch_block_pages_with_crc_sk_data_ready(struct sock *sk);
static void fetch_block_pages_with_crc_write_work(struct work_struct* work);
static void fetch_block_pages_with_crc_cleanup_work(struct work_struct* work);
static int fetch_block_pages_with_crc_write_req(struct block_socket* socket, struct block_request* breq);
static int fetch_block_pages_with_crc_receive_req(read_descriptor_t* rd_desc, struct sk_buff* skb, unsigned int offset, size_t len);

static struct block_ops fetch_block_pages_witch_crc_ops = {
    .sk_write_space = block_socket_sk_write_space,
    .sk_data_ready = fetch_block_pages_with_crc_sk_data_ready,
    .write_work = fetch_block_pages_with_crc_write_work,
    .cleanup_work = fetch_block_pages_with_crc_cleanup_work,
    .write_req = fetch_block_pages_with_crc_write_req,
    .receive_req = fetch_block_pages_with_crc_receive_req,
    .timeout_jiffies = &ternfs_fetch_block_timeout_jiffies,
};

static void write_block_sk_data_ready(struct sock *sk);
static void write_block_write_work(struct work_struct* work);
static void write_block_cleanup_work(struct work_struct* work);
static int write_block_write_req(struct block_socket* socket, struct block_request* breq);
static int write_block_receive_req(read_descriptor_t* rd_desc, struct sk_buff* skb, unsigned int offset, size_t len);

static struct block_ops write_block_ops = {
    .sk_write_space = block_socket_sk_write_space,
    .sk_data_ready = write_block_sk_data_ready,
    .write_work = write_block_write_work,
    .cleanup_work = write_block_cleanup_work,
    .write_req = write_block_write_req,
    .receive_req = write_block_receive_req,
    .timeout_jiffies = &ternfs_write_block_timeout_jiffies,
};

static void do_timeout_sockets(struct work_struct* w);
static DECLARE_DELAYED_WORK(timeout_work, do_timeout_sockets);

// Utils
// --------------------------------------------------------------------

static u32 ternfs_skb_copy(void* dstp, struct sk_buff* skb, u32 offset, u32 len) {
    struct skb_seq_state seq;
    u32 consumed = 0;
    char* dst = dstp;

    skb_prepare_seq_read(skb, offset, min(offset + len, skb->len), &seq);
    for (;;) {
        const u8* ptr;
        int avail = skb_seq_read(consumed, &ptr, &seq);
        if (avail == 0) { return consumed; }
        BUG_ON(avail < 0);
        // avail _can_ exceed the upper bound here
        int actual_avail = min((u32)avail, len-consumed);
        memcpy(dst + consumed, ptr, actual_avail);
        consumed += actual_avail;
        if (actual_avail < avail) {
            skb_abort_seq_read(&seq);
            return consumed;
        }
    }
    BUG();
}

// Generic socket/request infra
// --------------------------------------------------------------------

static void block_ops_init(struct block_ops* ops) {
    int i;
    hash_init(ops->sockets);
    for (i = 0; i < BLOCK_SOCKET_BUCKETS; i++) {
        spin_lock_init(&ops->locks[i]);
        atomic_set(&ops->len[i], 0);
        init_waitqueue_head(&ops->wqs[i]);
    }
}

struct block_socket {
    struct socket* sock;
    struct sockaddr_in addr;
    // We'll check that not more than `block_ops.timeout_jiffies` has passed since this.
    // For this reason, we set this timeout when:
    // * We write anything
    // * We read anything
    // The counters are separate as we can keep writing a lot of small requests while still not getting any response
    // in which case we still need to timeout
    u64 last_write_activity;
    u64 last_read_activity;
    // Error can be set either by write/read issues, error response which requires termination of socket
    // or timeout. If error is already set when enqueing work we wait for sock_wait and then try to get new socket.
    // This is to avoid enqueuing work on already errored out socket and a race with timing out inactive sockets.
    atomic_t err;
    // To store in the hashmap.
    struct hlist_node hnode;
    // Write end. write_work does all the work by peeking at the head, new requests
    // get added to the tail and try to write immediately if they get queued as head
    struct list_head write;
    spinlock_t list_lock;
    // We schedule write work as kernel is unhappy if we write from sk_write_space callback
    // write work is scheduled on ternfs_fast_wq and we yield after at most 10ms of writing
    struct work_struct write_work;
    // We schedule cleanup work on ternfs_fast_wq when we error out socket or if there was no activity for block_ops->timeout_jiffies
    struct work_struct cleanup_work;
    // There could be multiple cleanup items scheduled, we only want first one to remove socket from hash, wake up waiters, remove requests
    bool terminal;
    // Read end. "sk_data_ready" callback does all the work.
    struct list_head read;
    // used for waiting on connect or waiting for socket do be fully removed
    struct completion sock_wait;
    // we need number of waiters as last waiter needs to schedule work to free the socket (it will be terminal by then)
    int waiters;
    // Saved callbacks
    void (*saved_state_change)(struct sock *sk);
    void (*saved_data_ready)(struct sock *sk);
    void (*saved_write_space)(struct sock *sk);
};

// Needs to be called with read lock on socket->sock->sock->sk_callback_lock
// or from workqueue context (ternfs_fast_wq)
// Errors socket if not already in error state and schedules cleanup
inline void error_socket(struct block_socket* socket, int err) {
    if (atomic_cmpxchg(&socket->err, 0, err) == 0) {
        queue_work(ternfs_fast_wq, &socket->cleanup_work);
    }
}

static void block_ops_exit(struct block_ops* ops) {
    ternfs_debug("waiting for all sockets to be done");

    struct block_socket* sock;
    int bucket;
    rcu_read_lock();
    hash_for_each_rcu(ops->sockets, bucket, sock, hnode) {
        ternfs_debug("scheduling winddown for %d", ntohs(sock->addr.sin_port));
        error_socket(sock, -ECONNABORTED);
    }
    rcu_read_unlock();

    // wait for all of them to be freed by work
    for (bucket = 0; bucket < BLOCK_SOCKET_BUCKETS; bucket++) {
        ternfs_debug("waiting for bucket %d (len %d)", bucket, atomic_read(&ops->len[bucket]));
        wait_event(ops->wqs[bucket], atomic_read(&ops->len[bucket]) == 0);
    }
}

// used exclusively to track time spent in state for requests
enum block_requests_state {
    BR_ST_QUEUEING = 0,
    BR_ST_WRITING = 1,
    BR_ST_WAITING_RESPONSE = 2,
    BR_ST_READING = 3,
    BR_ST_MAX = 4
};

struct block_request {
    // To store the request in the read/write lists of the socket
    // Access should be protected by holding respectively block_socket->list_lock
    // There is no guarantee requests will be removed from write list before it gets removed form read list
    // as response can be quick and handled by a different thread. We only want to schedule complete_work once
    // removed from all lists.
    struct list_head read_list;
    struct list_head write_list;

    // There is no other reason for these to be atomic appart from preventing write req/read resp race
    atomic_t err;
    atomic_t state;
    atomic64_t timings[BR_ST_MAX];

    // work that gets scheduled on ternfs_wq to call complete callback on request
    struct work_struct complete_work;

    // How much is left to write (including the block body if writing)
    u32 left_to_write;
    // How much is left to read (including block body if reading)
    u32 left_to_read;
};

// We log requests which error out or take more than 10s to complete
static void block_socket_log_req_completion(struct block_socket* socket, struct block_request* req) {
    u64 completed = get_jiffies_64();
    u64 created = (u64)atomic64_read(&req->timings[BR_ST_QUEUEING]);
    u64 total = completed - created;
    int err = atomic_read(&socket->err);
    if (err == 0 && total < MSECS_TO_JIFFIES(10000)) {
        return;
    }
    int state = atomic_read(&req->state);
    if (state != BR_ST_READING) {
        // in case request didn't get to last state set now as end time of last operation
        atomic64_set(&req->timings[state + 1], completed);
    }
    u64 writing_started = max(created, (u64)atomic64_read(&req->timings[BR_ST_WRITING]));
    u64 waiting_started = max(writing_started, (u64)atomic64_read(&req->timings[BR_ST_WAITING_RESPONSE]));
    u64 reading_started = max(waiting_started, (u64)atomic64_read(&req->timings[BR_ST_READING]));
    u64 queued = writing_started - created;
    u64 writing = waiting_started - writing_started;
    u64 waiting = reading_started - waiting_started;
    u64 reading = completed - reading_started;

    ternfs_warn("dropped block request to %pI4:%d (err=%d) last_state=%d left_to_read=%d left_to_write=%d elapsed=%llums [queued=%llums, writing=%llums, waiting=%llums, reading=%llums]",
        &socket->addr.sin_addr, ntohs(socket->addr.sin_port), err, state, req->left_to_read, req->left_to_write, jiffies64_to_msecs(total),
        jiffies64_to_msecs(queued), jiffies64_to_msecs(writing), jiffies64_to_msecs(waiting), jiffies64_to_msecs(reading));
}

static void block_socket_state_check_locked(struct sock* sk) {
    struct block_socket* socket = sk->sk_user_data;

    if (sk->sk_state == TCP_ESTABLISHED) { return; } // the only good one

    // Right now we connect beforehand, so every change is trouble. Just
    // kill the socket.
    ternfs_debug("socket state check triggered: %d, will fail reqs", sk->sk_state);
    error_socket(socket, -ECONNABORTED); // TODO we could have nicer errors
}

static void block_socket_sk_state_change(struct sock* sk) {
    void (*saved_state_change)(struct sock *);
    read_lock_bh(&sk->sk_callback_lock);
    struct block_socket* socket = sk->sk_user_data;
    if (socket != NULL) {
        saved_state_change = socket->saved_state_change;
        block_socket_state_check_locked(sk);
    } else {
        saved_state_change = sk->sk_state_change;
    }
    read_unlock_bh(&sk->sk_callback_lock);
    saved_state_change(sk);
}

static void block_socket_connect_sk_state_change(struct sock* sk) {
    void (*saved_state_change)(struct sock *);
    read_lock_bh(&sk->sk_callback_lock);
    struct block_socket* socket = sk->sk_user_data;
    if (socket != NULL) {
        if (sk->sk_state == TCP_ESTABLISHED) {
           complete_all(&socket->sock_wait);
        }
        saved_state_change = socket->saved_state_change;
    } else {
        saved_state_change = sk->sk_state_change;
    }
    read_unlock_bh(&sk->sk_callback_lock);

    saved_state_change(sk);
}

static void block_socket_write_work(
    struct block_ops* ops,
    struct block_socket* socket
) {

    ternfs_debug("%pI4:%d", &socket->addr.sin_addr, ntohs(socket->addr.sin_port));
    // if socked failed, exit immediately
    if (atomic_read(&socket->err)) {
        if (socket->terminal) {
            queue_work(ternfs_fast_wq, &socket->cleanup_work);
        }
        return;
    }

    struct block_request* req;
    u64 start = get_jiffies_64();
    spin_lock_bh(&socket->list_lock);
    req = list_first_entry_or_null(&socket->write, struct block_request, write_list);
    socket->last_write_activity = get_jiffies_64();
    spin_unlock_bh(&socket->list_lock);

    while (req != NULL) {
        if (get_jiffies_64() - start > MSECS_TO_JIFFIES(10)) {
            queue_work(ternfs_fast_wq, &socket->write_work);
            //yield to other sockets
            break;
        }
        // Can't be done already, we just got the req
        BUG_ON(req->left_to_write == 0);

        if (atomic_read(&req->state) == BR_ST_QUEUEING) {
            atomic_set(&req->state, BR_ST_WRITING);
            atomic64_set(&req->timings[BR_ST_WRITING], get_jiffies_64());
        }

        int sent = ops->write_req(socket, req);
        if (sent < 0) {
            error_socket(socket, sent);
            return;
        }
        BUG_ON(sent > req->left_to_write);
        req->left_to_write -= sent;
        if (req->left_to_write > 0) {
            // we didn't manage to write out request, wait for more space
            break;
        }

        // we are done with this requests update timers and get a new one
        atomic64_set(&req->timings[BR_ST_WAITING_RESPONSE], get_jiffies_64());
        // we need to compare as we could be in reading state already
        atomic_cmpxchg(&req->state, BR_ST_WRITING, BR_ST_WAITING_RESPONSE);
        struct block_request* completed_req = req;
        spin_lock_bh(&socket->list_lock);
        list_del(&req->write_list);
        INIT_LIST_HEAD(&req->write_list); // mark it empty for cleanup function

        // check if this request is first and update read activity time so we don't get timed out
        if (req == list_first_entry_or_null(&socket->read, struct block_request, read_list)) {
            socket->last_read_activity = get_jiffies_64();
        }
        bool scheduledCompletion = list_empty(&completed_req->read_list);
        req = list_first_entry_or_null(&socket->write, struct block_request, write_list);
        socket->last_write_activity = get_jiffies_64();
        spin_unlock_bh(&socket->list_lock);
        if (scheduledCompletion) {
            block_socket_log_req_completion(socket, completed_req);
            queue_work(ternfs_wq, &completed_req->complete_work);
        }
    }
}

static void block_socket_sk_write_space(struct sock* sk) {
    void (*old_write_space)(struct sock *);
    read_lock_bh(&sk->sk_callback_lock);
    struct block_socket* socket = (struct block_socket*)sk->sk_user_data;
    if (socket != NULL) {
        queue_work(ternfs_fast_wq, &socket->write_work);
        old_write_space = socket->saved_write_space;
    } else {
        old_write_space = sk->sk_write_space;
    }
    read_unlock_bh(&sk->sk_callback_lock);
    old_write_space(sk);
}

// Gets the socket, and acquires a reference to it.
//
// If successful, returns with RCU read lock taken.
static struct block_socket* __acquires(RCU) get_block_socket(
    struct block_ops* ops,
    struct sockaddr_in* addr
) {
    u64 key = block_socket_key(addr);
    int bucket = hash_min(key, BLOCK_SOCKET_BITS);

    rcu_read_lock();
    struct block_socket* sock;
    hlist_for_each_entry_rcu(sock, &ops->sockets[bucket], hnode) {
        if (block_socket_key(&sock->addr) == key) {
            return sock;
        }
    }
    rcu_read_unlock();

    ternfs_debug("socket to %pI4:%d not found, will create", &addr->sin_addr, ntohs(addr->sin_port));

    sock = kmalloc(sizeof(struct block_socket), GFP_KERNEL);
    int err = 0;
    if (sock == NULL) {
        err = -ENOMEM;
        goto out_err;
    }

    memcpy(&sock->addr, addr, sizeof(struct sockaddr_in));

    atomic_set(&sock->err, 0);

    err = sock_create_kern(&init_net, PF_INET, SOCK_STREAM, IPPROTO_TCP, &sock->sock);
    if (err != 0) { goto out_err_sock; }

    // very important to set it so we can use kernel_sendmsg from sk_write_space callback
    // otherwsi we might try to allocate and sleep
    sock->sock->sk->sk_allocation = GFP_ATOMIC;

    init_completion(&sock->sock_wait);

    // Put the minimal state_change callback in before connecting because it is
    // needed to finalise the completion when connection becomes established.
    //
    // Note that
    // 1. We save the callbacks just for hygiene -- I'm pretty sure this is actually
    //     not needed since we call the original callbacks in the state change callback
    //     anyway. Moreover it's nice to be able to remove the custom callbacks
    //     when disposing of a socket, since then we know that we're really done with
    //     that socket (see)
    // 2. We could use TFO, but we currently don't. It shouldn't really matter anyway
    //     since we aggressively reuse connection, and also given that we have our own
    //     callbacks which does not play well at connection opening stage.
    sock->saved_state_change = sock->sock->sk->sk_state_change;
    sock->sock->sk->sk_user_data = sock;
    sock->sock->sk->sk_state_change = block_socket_connect_sk_state_change;

    err = kernel_connect(sock->sock, (struct sockaddr*)&sock->addr, sizeof(sock->addr), O_NONBLOCK);
    if (err < 0) {
        if (unlikely(err != -EINPROGRESS)) {
            ternfs_warn("could not connect to block service at %pI4:%d: %d", &sock->addr.sin_addr, ntohs(sock->addr.sin_port), err);
            sock_release(sock->sock);
            goto out_err_sock;
        } else {
            if (likely(sock->sock->sk->sk_state != TCP_ESTABLISHED)) {
                ternfs_debug("waiting for connection to %pI4:%d to be established", &sock->addr.sin_addr, ntohs(sock->addr.sin_port));
                err = wait_for_completion_timeout(&sock->sock_wait, ternfs_block_service_connect_timeout_jiffies);
                if (err <= 0) {
                    ternfs_warn("timed out waiting for connection to %pI4:%d to be established", &sock->addr.sin_addr, ntohs(sock->addr.sin_port));
                    sock_release(sock->sock);
                    err = -ETIMEDOUT;
                    goto out_err_sock;
                }
                // we need to re-init for removal waiters
                reinit_completion(&sock->sock_wait);
            }
        }
    }

    spin_lock_init(&sock->list_lock);
    INIT_LIST_HEAD(&sock->write);
    INIT_WORK(&sock->write_work, ops->write_work);
    INIT_WORK(&sock->cleanup_work, ops->cleanup_work);
    INIT_LIST_HEAD(&sock->read);


    sock->terminal = false;
    sock->last_read_activity = sock->last_write_activity = get_jiffies_64();
    sock->waiters = 0;

    // now insert
    struct block_socket* other_sock;

    // Important for the callbacks to happen after the connect, see
    // comment about fastopen above.
    //
    // We cannot setup the callbacks before we take the read lock.
    // They might delete the socket itself. We also cannot just
    // add them afterwards, otherwise multiple processes racing
    // to add the same socket might exit this function. So we take
    // the callback locks now, add the socket, and then change the
    // callbacks contextually to adding the socket to the hash
    // map.

    write_lock(&sock->sock->sk->sk_callback_lock);

    spin_lock(&ops->locks[bucket]);
    // Take the RCU lock while holding the socket lock so that
    // we know it won't get removed by timeouts or such in the
    // span of time between unlocking the socket lock and acquiring
    // the RCU lock.
    rcu_read_lock();
    // first check if somebody else got there first
    hlist_for_each_entry_rcu(other_sock, &ops->sockets[bucket], hnode) { // first we check if somebody didn't get to it first
        if (block_socket_key(&other_sock->addr) == key) {
            // somebody got here before us
            rcu_read_unlock();
            spin_unlock(&ops->locks[bucket]);
            write_unlock(&sock->sock->sk->sk_callback_lock);
            ternfs_debug("multiple callers tried to get socket to %pI4:%d, dropping one", &other_sock->addr.sin_addr, ntohs(other_sock->addr.sin_port));
            // call again rather than trying to `sock_release` with the
            // RCU read lock held, this might not be safe in atomic context.
            sock_release(sock->sock);
            kfree(sock);
            return get_block_socket(ops, addr);
        }
    }

    // Put the new callbacks in

    sock->saved_data_ready = sock->sock->sk->sk_data_ready;
    sock->saved_write_space = sock->sock->sk->sk_write_space;
    sock->sock->sk->sk_data_ready = ops->sk_data_ready;
    sock->sock->sk->sk_state_change = block_socket_sk_state_change;
    sock->sock->sk->sk_write_space = ops->sk_write_space;

    // Insert the socket into the hash map -- anyone else which
    // will find it will be good to do.
    hlist_add_head_rcu(&sock->hnode, &ops->sockets[bucket]);

    spin_unlock(&ops->locks[bucket]);

    write_unlock(&sock->sock->sk->sk_callback_lock);

    atomic_inc(&ops->len[bucket]);

    // We are holding RCU, let's verify that we also have a socket.
    BUG_ON(IS_ERR(sock));
    return sock;
out_err_sock:
    kfree(sock);
out_err:
    // we grab rcu to keep sparse happy as we promised to acquire RCU
    rcu_read_lock();
    return ERR_PTR(err);
}

inline bool block_socket_check_timeout(struct block_socket* socket, u64 now, u64 timeout_jiffies, bool set_error) {
    // if something is locked activity in progress, nothing to do
    if (!spin_trylock_bh(&socket->list_lock)) {
        return false;
    }
    u64 last_activity = max(socket->last_read_activity, socket->last_write_activity);
    bool timed_out = now - min(now, last_activity) > timeout_jiffies;
    if (set_error && timed_out) {
        atomic_cmpxchg(&socket->err, 0, -ETIMEDOUT);
    }
    spin_unlock_bh(&socket->list_lock);
    return timed_out;
}

static void block_socket_cleanup_work(
    struct block_ops* ops,
    struct block_socket* socket
) {
    block_socket_check_timeout(socket, get_jiffies_64(), *ops->timeout_jiffies, true);
    int err = atomic_read(&socket->err);

    if (!err) {
        return;
    }
    // wait for callbacks to complete
    write_lock_bh(&socket->sock->sk->sk_callback_lock);

    // We are killing the socket either due to error or timeout
    // We already have callback lock, no callback could be running except for
    // default linux callback which we don't care about.
    socket->sock->sk->sk_data_ready = socket->saved_data_ready;
    socket->sock->sk->sk_state_change = socket->saved_state_change;
    socket->sock->sk->sk_write_space = socket->saved_write_space;
    socket->sock->sk->sk_user_data = NULL;
    write_unlock_bh(&socket->sock->sk->sk_callback_lock);

    ternfs_debug("winding down socket to %pI4:%d due to %d", &socket->addr.sin_addr, ntohs(socket->addr.sin_port), err);

    u64 key = block_socket_key(&socket->addr);
    int bucket = hash_min(key, BLOCK_SOCKET_BITS);
    bool completion_waiters = false;

    if (!socket->terminal) {
        socket->terminal = true;

        // First, remove socket from hashmap. After we're done with this,
        // we know nobody's going to add new requests to this.
        spin_lock(&ops->locks[bucket]);
        hash_del_rcu(&socket->hnode); // tied to atomic_dec below
        spin_unlock(&ops->locks[bucket]);
        synchronize_rcu();

        // Now complete all the remaining requests with the error.
        // We are not the only one remaining, there could be waiters for cleanup completion
        // We need to take locks

        struct block_request* req;
        struct block_request* tmp;

        struct list_head all_reqs;
        INIT_LIST_HEAD(&all_reqs);

        spin_lock_bh(&socket->list_lock);

        list_for_each_entry_safe(req, tmp, &socket->read, read_list) {
            if (!list_empty(&req->write_list)) {
                list_del(&req->write_list);
            }
            list_del(&req->read_list);
            list_add(&req->write_list, &all_reqs);
        }
        list_for_each_entry_safe(req, tmp, &socket->write, write_list) {
            list_del(&req->write_list);
            list_add(&req->write_list, &all_reqs);
        }
        completion_waiters = socket->waiters != 0;
        spin_unlock_bh(&socket->list_lock);

        if (completion_waiters) {
            // wake them up, last one will schedule termination work
            complete_all(&socket->sock_wait);
        }


        list_for_each_entry_safe(req, tmp, &all_reqs, write_list) {
            atomic_cmpxchg(&req->err, 0, err);
            ternfs_debug("completing request because of a socket winddown");
            list_del(&req->write_list);
            block_socket_log_req_completion(socket, req);
            queue_work(ternfs_wq, &req->complete_work);
        }
    }

    // Finally, kill the socket, unless there were waiters or if there is another work item remaining, in which
    // case it'll kill the socket but won't execute the cleanup operation above.
    if (!(completion_waiters || test_bit(WORK_STRUCT_PENDING_BIT, work_data_bits(&socket->cleanup_work)) || test_bit(WORK_STRUCT_PENDING_BIT, work_data_bits(&socket->write_work)))) {
        sock_release(socket->sock);
        kfree(socket);

        // Adjust len, notify waiters
        smp_mb__before_atomic();
        atomic_dec(&ops->len[bucket]);
        wake_up_all(&ops->wqs[bucket]);
    }
}

static int block_socket_receive_req_locked(
    int (*receive_single_req)(struct block_request* req, struct sk_buff* skb, unsigned int offset, size_t len),
    read_descriptor_t* rd_desc,
    struct sk_buff* skb,
    unsigned int offset,
    size_t len
) {
    struct block_socket* socket = rd_desc->arg.data;

    ternfs_debug("%pI4:%d offset=%u len=%lu", &socket->addr.sin_addr, ntohs(socket->addr.sin_port), offset, len);

    size_t len0 = len;

    if (atomic_read(&socket->err)) {
        return len0;
    }

    // line up first req
    struct block_request* req;
    spin_lock_bh(&socket->list_lock);
    req = list_first_entry_or_null(&socket->read, struct block_request, read_list);
    socket->last_read_activity = get_jiffies_64();
    spin_unlock_bh(&socket->list_lock);

    while (len > 0) {
        if (atomic_read(&req->state) != BR_ST_READING) {
            atomic_set(&req->state, BR_ST_READING);
            atomic64_set(&req->timings[BR_ST_READING], get_jiffies_64());
        }
        int consumed = receive_single_req(req, skb, offset, len);
        if (consumed < 0) {
            error_socket(socket, consumed);
            return len0;
        }
        if (consumed == 0) {
            // we didn't make any progress, stop
            break;
        }
        BUG_ON(consumed > req->left_to_read);
        req->left_to_read -= consumed;
        ternfs_debug("left_to_read=%u consumed=%d", req->left_to_read, consumed);
        len -= consumed;
        offset += consumed;
        if (req->left_to_read == 0 || atomic_read(&req->err)) {
            // this request is done remove it from list and schedule completion
            struct block_request* completed_req = req;
            spin_lock_bh(&socket->list_lock);
            list_del(&req->read_list);
            INIT_LIST_HEAD(&req->read_list);

            req = list_first_entry_or_null(&socket->read, struct block_request, read_list);
            socket->last_read_activity = get_jiffies_64();
            bool scheduleCompletion = list_empty(&completed_req->write_list);
            spin_unlock_bh(&socket->list_lock);
            if (scheduleCompletion) {
                block_socket_log_req_completion(socket, completed_req);
                queue_work(ternfs_wq, &completed_req->complete_work);
            }
        }
    }

    // We have more data but no requests. This should not happen
    BUG_ON(req == NULL && len > 0);
    return len0 - len;
}

static void block_socket_sk_data_ready(
    struct block_ops* ops,
    struct sock* sk
) {
    read_lock_bh(&sk->sk_callback_lock);
    if (sk->sk_user_data != NULL) {
        read_descriptor_t rd_desc;
        // Taken from iscsi -- we set count to 1 because we want the network layer to
        // hand us all the skbs that are available.
        rd_desc.arg.data = sk->sk_user_data;
        rd_desc.count = 1;
        tcp_read_sock(sk, &rd_desc, ops->receive_req);
        block_socket_state_check_locked(sk);
    }
    read_unlock_bh(&sk->sk_callback_lock);
}

static struct block_socket* __acquires(RCU) get_blockservice_socket(
    struct block_ops* ops,
    struct ternfs_block_service* bs
)   {
    int i, block_ip;
    struct sockaddr_in addr;
    struct block_socket* sock;

    // Try both ips (if we have them) before giving up
    block_ip = WHICH_BLOCK_IP++;
    for (i = 0; i < 1 + (bs->port2 != 0); i++, block_ip++) { // we might not have a second address
        addr.sin_family = AF_INET;
        if (bs->port2 == 0 || block_ip&1) {
            addr.sin_addr.s_addr = htonl(bs->ip1);
            addr.sin_port = htons(bs->port1);
        } else {
            addr.sin_addr.s_addr = htonl(bs->ip2);
            addr.sin_port = htons(bs->port2);
        }

        sock = get_block_socket(ops, &addr);
        if (!IS_ERR(sock)) {
            goto out;
        }
        rcu_read_unlock();
    }
    rcu_read_lock();
out:
    return sock;
}

static int block_socket_start_req(
    struct block_ops* ops,
    struct block_request* req,
    struct ternfs_block_service* bs,
    u32 left_to_write,
    u32 left_to_read
) {
    int err, i;
    bool is_first, socket_error;
    struct block_socket* sock;

    atomic_set(&req->err, 0);
    for (i = 0; i < BR_ST_MAX; ++i) {
        atomic64_set(&req->timings[i], 0);
    }
    atomic_set(&req->state, BR_ST_QUEUEING);
    atomic64_set(&req->timings[BR_ST_QUEUEING], get_jiffies_64());

    req->left_to_write = left_to_write;
    req->left_to_read = left_to_read;


retry_get_socket:
    // As the very last thing, try to get the socket (will get RCU lock).
    sock = get_blockservice_socket(ops, bs);
    if (IS_ERR(sock)) {
        err = PTR_ERR(sock);
        rcu_read_unlock();
        goto out_err;
    }
    err = 0;

    // We have a socket, and we also have the RCU lock. We need to hurry
    // and place the request in the queue, and schedule work. Everything
    // in the RCU section, since otherwise the socket might be cleared
    // under our feet. We need to put it in both write and read queue
    // to avoid a race where we get response before we move it from write
    // to read queue
    is_first = false;
    socket_error = false;
    spin_lock_bh(&sock->list_lock);
    socket_error = atomic_read(&sock->err) != 0;
    if (socket_error) {
        ++sock->waiters;
    } else {
        is_first = list_first_entry_or_null(&sock->write, struct block_request, write_list) == NULL;
        list_add_tail(&req->write_list, &sock->write);
        if (is_first) {
            // Prevent timeout
            sock->last_write_activity = get_jiffies_64();
        }
        list_add_tail(&req->read_list, &sock->read);
    }
    spin_unlock_bh(&sock->list_lock);

    // current socket is in errored state. Release rcu and wait for removal. Socket will not get destroyed
    // as we incremented waiters under a lock above.
    if (socket_error) {
        rcu_read_unlock();
        err = wait_for_completion_timeout(&sock->sock_wait, MSECS_TO_JIFFIES(100));
        // we can't use atomics if we want to have a timeout here otherwise we race with destruction
        spin_lock_bh(&sock->list_lock);
        bool last_waiter = --sock->waiters == 0;
        spin_unlock_bh(&sock->list_lock);
        if (last_waiter) {
            queue_work(ternfs_fast_wq, &sock->cleanup_work);
        }

        if (err <= 0) {
            ternfs_warn("timed out waiting for socked to bs=%016llx  to be cleaned up", bs->id);
            err = -ETIMEDOUT;
            goto out_err;
        }
        goto retry_get_socket;
    }

    // We are first request in write queue, schedule writing
    if (is_first) {
        queue_work(ternfs_fast_wq, &sock->write_work);
    }

    // we're done
    rcu_read_unlock();
    return 0;

out_err:
    ternfs_info("couldn't start block request, err=%d", err);
    return err;
}

// Call periodically to check for inactive ones and schedules work on them to potentially time them out
static void timeout_sockets(struct block_ops* ops) {
    int bucket;
    struct block_socket* sock;
    u64 now = get_jiffies_64();

    rcu_read_lock();
    hash_for_each_rcu(ops->sockets, bucket, sock, hnode) {
        if (block_socket_check_timeout(sock, now, *ops->timeout_jiffies, false)) {
            // We can't clean up here as timing out involves removing from hash and needs
            // synchronization through rcu_synchronize
            queue_work(ternfs_fast_wq, &sock->cleanup_work);
        }
    }
    rcu_read_unlock();
}

static int drop_sockets(struct block_ops* ops) {
    int dropped = 0;
    int bucket;
    struct block_socket* sock;

    rcu_read_lock();
    hash_for_each_rcu(ops->sockets, bucket, sock, hnode) {
        dropped++;
        // We put EAGAIN here so that the caller can restart the
        // request if it wants to (since we didn't really encounter
        // any error). We don't currently do this though.
        error_socket(sock, -EAGAIN);
    }
    rcu_read_unlock();

    ternfs_info("dropped %d sockets", dropped);

    return dropped;
}

// Fetch
// --------------------------------------------------------------------

struct fetch_request {
    struct block_request breq;

    void (*callback)(void* data, u64 block_id, struct list_head* pages, int err);
    void *data;
    struct list_head pages;

    // Req info
    u64 file_id;
    u64 block_service_id;
    u64 block_id;
    u32 block_crc;
    u32 offset;
    u32 count;
    u32 bytes_fetched;

    // When reading blocks with crcs we need to alternate between reading pages
    // and CRCs. next_read_size will be either PAGE_SIZE or PAGE_CRC_BYTES.
    u32 next_read_size;

    // The cursor inside the current page.
    u16 page_offset;
    // How much we've read of the header.
    u8 header_read;
    static_assert(TERNFS_FETCH_BLOCK_RESP_SIZE == 0);
    union {
        char buf[TERNFS_BLOCKS_RESP_HEADER_SIZE + 2];
        struct {
            __le32 protocol;
            u8 kind;
            __le16 error;
        } __attribute__((packed)) data;
    } header;
};

#define get_fetch_request(req) container_of(req, struct fetch_request, breq)

static void fetch_complete(struct block_request* breq) {
    struct fetch_request* req = get_fetch_request(breq);
    // When fetching pages with crc, the pages list needs to be returned in the
    // exact same order as it was received. We need to restore it back if
    // reading didn't finish and not all the items got rotated.
    struct page* first_page = list_first_entry(&req->pages, struct page, lru);
    struct page *first_seen = first_page;
    struct page* last_page = list_last_entry(&req->pages, struct page, lru);
    while(last_page->index < first_page->index || first_page->index == (unsigned long)-1) {
        list_rotate_left(&req->pages);
        ternfs_debug("rotating list left: %d %d, first_page:%ld, last_page:%ld", req->count, req->bytes_fetched, first_page->index, last_page->index);
        first_page = list_first_entry(&req->pages, struct page, lru);
        last_page = list_last_entry(&req->pages, struct page, lru);
        if (first_page == first_seen) {
            // Can only happen when page index is -1.
            // We've gone a full circle and we are still back at page out of range.
            break;
        }
    }
    ternfs_debug("block fetch complete block_id=%016llx err=%d", req->block_id, atomic_read(&req->breq.err));
    req->callback(req->data, req->block_id, &req->pages, atomic_read(&req->breq.err));

    ternfs_debug("block released socket block_id=%016llx err=%d", req->block_id, atomic_read(&req->breq.err));
    put_pages_list(&req->pages);
    kmem_cache_free(fetch_request_cachep, req);
}

static void fetch_complete_work(struct work_struct* work) {
    fetch_complete(container_of(work, struct block_request, complete_work));
}

static void fetch_request_constructor(void* ptr) {
    struct fetch_request* req = (struct fetch_request*)ptr;

    INIT_LIST_HEAD(&req->pages);
    INIT_WORK(&req->breq.complete_work, fetch_complete_work);
}

int ternfs_drop_fetch_block_sockets(void) {
    return drop_sockets(&fetch_block_pages_witch_crc_ops);
}

// Returns a negative result if the socket has to be considered corrupted.
// If a "recoverable" error occurs (e.g. the server just replies with an error)
// `req->err` will be filled in and receiving can continue. If an unrecoverable
// error occurs `req->complete.err` will be filled in also, but it's expected that the
// caller stops operating on this socket.
static int fetch_receive_single_req(
    struct block_request* breq,
    struct sk_buff* skb,
    unsigned int offset,
    size_t len
) {
    struct fetch_request* req = get_fetch_request(breq);
    size_t header_read;
    {
        size_t len0 = len;

        BUG_ON(req->breq.left_to_read == 0);

#define HEADER_COPY(buf, count) ({ \
        int read = count > 0 ? ternfs_skb_copy(buf, skb, offset, count) : 0; \
        offset += read; \
        len -= read; \
        req->header_read += read; \
        read; \
    })

        // Header
        {
            int header_left = TERNFS_BLOCKS_RESP_HEADER_SIZE - (int)req->header_read;
            int read = HEADER_COPY(req->header.buf + req->header_read, header_left);
            header_left -= read;
            if (header_left > 0) { return len0-len; }
        }

        // Protocol check
        if (unlikely(le32_to_cpu(req->header.data.protocol) != TERNFS_BLOCKS_RESP_PROTOCOL_VERSION)) {
            ternfs_info("bad blocks resp protocol, expected %*pE, got %*pE", 4, &TERNFS_BLOCKS_RESP_PROTOCOL_VERSION, 4, &req->header.data.protocol);
            return ternfs_error_to_linux(TERNFS_ERR_MALFORMED_RESPONSE);
        }

        // Error check
        if (unlikely(req->header.data.kind == 0)) {
            int error_left = (TERNFS_BLOCKS_RESP_HEADER_SIZE + 2) - (int)req->header_read;
            int read = HEADER_COPY(req->header.buf + req->header_read, error_left);
            error_left -= read;
            if (error_left > 0) { return len0-len; }
            // we got an error "nicely", the socket can carry on living.
            atomic_cmpxchg(&req->breq.err, 0, ternfs_error_to_linux(le16_to_cpu(req->header.data.error)));
            return len0-len;
        }

        header_read = len0-len;

        BUG_ON(header_read > req->breq.left_to_read);

#undef HEADER_COPY
    }

    // Actual data copy

    struct page* page = list_first_entry(&req->pages, struct page, lru);
    BUG_ON(!page);
    char* page_ptr = kmap_atomic(page);

    struct skb_seq_state seq;
    u32 read_len = min(req->breq.left_to_read - (u32)header_read, (u32)len);
    skb_prepare_seq_read(skb, offset, offset + read_len, &seq);
    u32 block_bytes_read = 0;
    while (block_bytes_read < req->breq.left_to_read) {
        const u8* data;
        u32 avail = skb_seq_read(block_bytes_read, &data, &seq);
        if (avail == 0) { break; }
        avail = min(avail, read_len - block_bytes_read); // avail can exceed the len upper bound

        while (avail) {
            u32 this_len = min3(avail, (u32)req->next_read_size - req->page_offset, req->breq.left_to_read - block_bytes_read);
            ternfs_debug("read %d bytes of %d", this_len, req->next_read_size);
            if (req->next_read_size == PAGE_SIZE) {
                req->bytes_fetched += this_len;
                memcpy(page_ptr + req->page_offset, data, this_len);
            } else {
                memcpy((u8 *)&page->private+req->page_offset, data, this_len);
            }
            data += this_len;
            req->page_offset += this_len;
            avail -= this_len;
            block_bytes_read += this_len;

            if (req->page_offset >= req->next_read_size) {
                BUG_ON(req->page_offset > req->next_read_size);
                req->page_offset = 0;
                if (req->next_read_size == PAGE_SIZE) {
                    if (req->block_crc > 0) {
                        //  We are handling BlockWithCrc response, so CRC will follow this page
                        req->next_read_size = PAGE_CRC_BYTES;
                        // We only want to rotate the list and map new page
                        // when we are not expecting CRC after each page. When
                        // parsing BlockWithCrc response, the list will be
                        // rotated and new page mapped when we finish reading the CRC.
                        continue;
                    }
                } else {
                    // We have finished reading the CRC of the last page
                    // the crc itself was stored in page_crc->crc in the read loop.
                    req->next_read_size = PAGE_SIZE;
                }
                kunmap_atomic(page_ptr);
                list_rotate_left(&req->pages);
                page = list_first_entry(&req->pages, struct page, lru);
                page_ptr = kmap_atomic(page);
            }
        }
    }

    // We might have not terminated with avail == 0, but because we're done with this request
    skb_abort_seq_read(&seq);

    BUG_ON(header_read + block_bytes_read > req->breq.left_to_read);

    kunmap_atomic(page_ptr);

    return header_read + block_bytes_read;
}

// --------------------------------------------------------------------
// Fetch pages with crc
// --------------------------------------------------------------------

static int fetch_block_pages_with_crc_write_req(struct block_socket* socket, struct block_request* breq) {
    struct fetch_request* req = get_fetch_request(breq);

    char req_msg_buf[TERNFS_BLOCKS_REQ_HEADER_SIZE + TERNFS_FETCH_BLOCK_WITH_CRC_REQ_SIZE];
    BUG_ON(breq->left_to_write > sizeof(req_msg_buf));

    // we're not done, fill in the req
    char* req_msg = req_msg_buf;
    put_unaligned_le32(TERNFS_BLOCKS_REQ_PROTOCOL_VERSION, req_msg); req_msg += 4;
    put_unaligned_le64(req->block_service_id, req_msg); req_msg += 8;
    *(u8*)req_msg = TERNFS_BLOCKS_FETCH_BLOCK_WITH_CRC; req_msg += 1;
    {
        struct ternfs_bincode_put_ctx ctx = {
            .start = req_msg,
            .cursor = req_msg,
            .end = req_msg_buf + sizeof(req_msg_buf),
        };
        ternfs_fetch_block_with_crc_req_put_start(&ctx, start);
        ternfs_fetch_block_with_crc_req_put_file_id_unused(&ctx, start, req_file_id, 0);
        ternfs_fetch_block_with_crc_req_put_block_id(&ctx, req_file_id, req_block_id, req->block_id);
        ternfs_fetch_block_with_crc_req_put_block_crc_unused(&ctx, req_block_id, req_block_crc, 0);
        ternfs_fetch_block_with_crc_req_put_offset(&ctx, req_block_crc, req_offset, req->offset);
        ternfs_fetch_block_with_crc_req_put_count(&ctx, req_offset, req_count, req->count);
        ternfs_fetch_block_with_crc_req_put_end(ctx, req_count, end);
        BUG_ON(ctx.cursor != ctx.end);
    }

    // send message
    struct msghdr msg = { .msg_flags = MSG_DONTWAIT };
    ternfs_debug("sending fetch block req to %pI4:%d, bs=%016llx block_id=%016llx", &socket->addr.sin_addr, ntohs(socket->addr.sin_port), req->block_service_id, req->block_id);
    struct kvec iov = {
        .iov_base = req_msg_buf + (sizeof(req_msg_buf) - breq->left_to_write),
        .iov_len = breq->left_to_write,
    };
    int err = kernel_sendmsg(socket->sock, &msg, &iov, 1, iov.iov_len);
    return err == -EAGAIN ? 0 : err;
}

static void fetch_block_pages_with_crc_write_work(struct work_struct* work) {
    block_socket_write_work(&fetch_block_pages_witch_crc_ops, container_of(work, struct block_socket, write_work));
}

static void fetch_block_pages_with_crc_cleanup_work(struct work_struct* work) {
    struct block_socket* socket = container_of(work, struct block_socket, cleanup_work);
    block_socket_cleanup_work(&fetch_block_pages_witch_crc_ops, socket);
}

static int fetch_block_pages_with_crc_receive_req(read_descriptor_t* rd_desc, struct sk_buff* skb, unsigned int offset, size_t len) {
    return block_socket_receive_req_locked(fetch_receive_single_req, rd_desc, skb, offset, len);
}

static void fetch_block_pages_with_crc_sk_data_ready(struct sock* sk) {
    block_socket_sk_data_ready(&fetch_block_pages_witch_crc_ops, sk);
}

int ternfs_fetch_block_pages_with_crc(
    void (*callback)(void* data, u64 block_id, struct list_head* pages, int err),
    void* data,
    struct ternfs_block_service* bs,
    struct list_head* pages,
    u64 file_id,
    u64 block_id,
    u32 block_crc,
    u32 offset,
    u32 count
) {
    int err, i;

    if (count%PAGE_SIZE) {
        // we rely on the pages being filled neatly in the block fetch code
        // (we could change this easily, but not needed for now)
        ternfs_warn("cannot read not page-sized block segment: %u", count);
        return -EIO;
    }

    // can't read
    if (unlikely(bs->flags & TERNFS_BLOCK_SERVICE_DONT_READ)) {
        ternfs_debug("could not fetch block given block flags %02x", bs->flags);
        return -EIO;
    }

    struct fetch_request* req = kmem_cache_alloc(fetch_request_cachep, GFP_KERNEL);
    if (!req) { err = -ENOMEM; goto out_err; }

    req->callback = callback;
    req->data = data;
    req->file_id = file_id;
    req->block_service_id = bs->id;
    req->block_id = block_id;
    req->block_crc = block_crc;
    req->offset = offset;
    req->count = count;
    req->bytes_fetched = 0;
    req->page_offset = 0;
    req->header_read = 0;
    req->next_read_size = PAGE_SIZE;

    BUG_ON(!list_empty(&req->pages));
    for (i = 0; i < count/PAGE_SIZE; i++) {
        struct page *page = list_first_entry_or_null(pages, struct page, lru);
        BUG_ON(page == NULL);
        list_del(&page->lru);
        page->private = 0;
        list_add_tail(&page->lru, &req->pages);
    }
    BUG_ON(!list_empty(pages));
    err = block_socket_start_req(
        &fetch_block_pages_witch_crc_ops,
        &req->breq,
        bs,
        TERNFS_BLOCKS_REQ_HEADER_SIZE + TERNFS_FETCH_BLOCK_WITH_CRC_REQ_SIZE,
        TERNFS_BLOCKS_RESP_HEADER_SIZE + TERNFS_FETCH_BLOCK_WITH_CRC_RESP_SIZE + count + (count / PAGE_SIZE) * PAGE_CRC_BYTES
    );
    if (err) {
        goto out_err_pages;
    }

    return 0;

out_err_pages:
    // Transfer pages back to the caller.
    for (i = 0; i < count/PAGE_SIZE; i++) {
        struct page *page = list_first_entry_or_null(&req->pages, struct page, lru);
        BUG_ON(page == NULL);
        list_del(&page->lru);
        page->private = 0;
        list_add_tail(&page->lru, pages);
    }
    kmem_cache_free(fetch_request_cachep, req);
out_err:
    ternfs_info("couldn't start fetch block request, err=%d", err);
    return err;
}

// Write
// --------------------------------------------------------------------

struct write_request {
    struct block_request breq;

    // Called when request is done
    void (*callback)(void* data, struct list_head* pages, u64 block_id, u64 proof, int err);
    void* data;

    struct list_head pages;

    // Req info
    u64 block_service_id;
    u64 block_id;
    u32 crc;
    u32 size;
    u64 certificate;

    static_assert(TERNFS_WRITE_BLOCK_RESP_SIZE > 2);
    union {
        char buf[TERNFS_BLOCKS_RESP_HEADER_SIZE+TERNFS_WRITE_BLOCK_RESP_SIZE];
        struct {
            __le32 protocol;
            u8 kind;
            union {
                __le16 error;
                __be64 proof;
            };
        } __attribute__((packed)) data;
    } write_resp;
};

#define get_write_request(req) container_of(req, struct write_request, breq)

static void write_block_complete(struct block_request* breq) {
    struct write_request* req = get_write_request(breq);

    // might be that we didn't consume all the pages -- in which case we need
    // to keep rotating until we're there.
    while (list_first_entry(&req->pages, struct page, lru)->private == 0) {
        list_rotate_left(&req->pages);
    }
    req->callback(req->data, &req->pages, req->block_id, be64_to_cpu(req->write_resp.data.proof), atomic_read(&req->breq.err));

    BUG_ON(!list_empty(&req->pages)); // the callback must acquire this

    // What if the socket is being read right now? There's probably a race with
    // that and the free here.
    //
    // This function is called in three ways:
    // * `block_socket_receive_req_locked` enqueues the write complete work on ternfs_wq
    // * `block_socket_write_req_locked` enqueues the write complete work on ternfs_wq
    // * `cleanup_work` enqueues the write complete on ternfs_wq
    //
    // `cleanup_work` changes the callbacks, so that by the time
    // we get to call this function all the callbacks s must be done.
    //
    // Moreover `block_socket_write/receive_req_locked` removes the request from the queue as it
    // enqueues the work. So this should be safe.

    kmem_cache_free(write_request_cachep, req);
}

static void write_block_complete_work(struct work_struct* work) {
    write_block_complete(container_of(work, struct block_request, complete_work));
}

static void write_request_constructor(void* ptr) {
    struct write_request* req = (struct write_request*)ptr;

    INIT_LIST_HEAD(&req->pages);
    INIT_WORK(&req->breq.complete_work, write_block_complete_work);
}

int ternfs_drop_write_block_sockets(void) {
    return drop_sockets(&write_block_ops);
}

static int write_block_write_req(struct block_socket* socket, struct block_request* breq) {
    struct write_request* req = get_write_request(breq);
    int err = 0;

    u32 write_size = TERNFS_BLOCKS_REQ_HEADER_SIZE + TERNFS_WRITE_BLOCK_REQ_SIZE + req->size;
    u32 write_req_written0 = write_size - breq->left_to_write;
    u32 write_req_written = write_req_written0;

#define BLOCK_WRITE_EXIT return write_req_written - write_req_written0;

    // Still writing request -- we just serialize the buffer each time and send it out
    while (write_req_written < TERNFS_BLOCKS_REQ_HEADER_SIZE+TERNFS_WRITE_BLOCK_REQ_SIZE) {
        char req_msg_buf[TERNFS_BLOCKS_REQ_HEADER_SIZE+TERNFS_WRITE_BLOCK_REQ_SIZE];
        char* req_msg = req_msg_buf;
        put_unaligned_le32(TERNFS_BLOCKS_REQ_PROTOCOL_VERSION, req_msg); req_msg += 4;
        put_unaligned_le64(req->block_service_id, req_msg); req_msg += 8;
        *(u8*)req_msg = TERNFS_BLOCKS_WRITE_BLOCK; req_msg += 1;
        {
            struct ternfs_bincode_put_ctx ctx = {
                .start = req_msg,
                .cursor = req_msg,
                .end = req_msg_buf + sizeof(req_msg_buf),
            };
            ternfs_write_block_req_put_start(&ctx, start);
            ternfs_write_block_req_put_block_id(&ctx, start, req_block_id, req->block_id);
            ternfs_write_block_req_put_crc(&ctx, req_block_id, req_crc, req->crc);
            ternfs_write_block_req_put_size(&ctx, req_crc, req_size, req->size);
            ternfs_write_block_req_put_certificate(&ctx, req_size, req_certificate, req->certificate);
            ternfs_write_block_req_put_end(ctx, req_certificate, end);
            BUILD_BUG_ON(ctx.cursor != ctx.end);
        }

        // send message
        struct msghdr msg = {
            .msg_flags = MSG_MORE | MSG_DONTWAIT,
        };
        ternfs_debug("sending write block req to %pI4:%d", &socket->addr.sin_addr, ntohs(socket->addr.sin_port));
        struct kvec iov = {
            .iov_base = req_msg_buf + write_req_written,
            .iov_len = sizeof(req_msg_buf) - write_req_written,
        };
        int sent = kernel_sendmsg(socket->sock, &msg, &iov, 1, iov.iov_len);
        if (sent == -EAGAIN) { BLOCK_WRITE_EXIT } // done for now, will be rescheduled by `sk_write_space`
        if (sent < 0) { err = sent; goto out_err; }
        write_req_written += sent;
    }

    // Write the pages out
    while (write_req_written < write_size) {
        struct page* page = list_first_entry(&req->pages, struct page, lru);
        int sent = kernel_sendpage(
            socket->sock, page,
            page->index,
            min((u32)(PAGE_SIZE - page->index), write_size - write_req_written),
            MSG_MORE | MSG_DONTWAIT
        );
        if (sent == -EAGAIN) {
            // done for now, will be rescheduled by `sk_write_space`
            break;
        }
        if (sent < 0) { err = sent; goto out_err; }
        page->index += sent;
        write_req_written += sent;
        if (page->index >= PAGE_SIZE) {
            BUG_ON(page->index != PAGE_SIZE);
            list_rotate_left(&req->pages);
            if (write_size - write_req_written) { // otherwise this will be null
                list_first_entry(&req->pages, struct page, lru)->index = 0;
                list_first_entry(&req->pages, struct page, lru)->private = 0;
            }
        }
    }
    BLOCK_WRITE_EXIT;

out_err:
    ternfs_debug("block write failed err=%d", err);
    BUG_ON(err == 0);
    atomic_cmpxchg(&breq->err, 0, err);
    return err;

#undef BLOCK_WRITE_EXIT
}

static void write_block_write_work(struct work_struct* work) {
    struct block_socket* socket = container_of(work, struct block_socket, write_work);
    block_socket_write_work(&write_block_ops, socket);
}

static void write_block_cleanup_work(struct work_struct* work) {
    struct block_socket* socket = container_of(work, struct block_socket, cleanup_work);
    block_socket_cleanup_work(&write_block_ops, socket);
}

static int write_block_receive_single_req(
    struct block_request* breq,
    struct sk_buff* skb,
    unsigned int offset,
    size_t len0
) {
    struct write_request* req = get_write_request(breq);
    size_t len = len0;
    u32 write_resp_read = (TERNFS_BLOCKS_RESP_HEADER_SIZE + TERNFS_WRITE_BLOCK_RESP_SIZE) - req->breq.left_to_read;

#define BLOCK_WRITE_EXIT(i) do { \
        if (i < 0) { \
            atomic_cmpxchg(&req->breq.err, 0, i); \
        } \
        return i; \
    } while (0)

    // Write resp

#define HEADER_COPY(buf, count) ({ \
        int read = count > 0 ? ternfs_skb_copy(buf, skb, offset, count) : 0; \
        offset += read; \
        len -= read; \
        write_resp_read += read; \
        read; \
    })

    // Header
    {
        int header_left = TERNFS_BLOCKS_RESP_HEADER_SIZE - (int)write_resp_read;
        int read = HEADER_COPY(req->write_resp.buf + write_resp_read, header_left);
        header_left -= read;
        if (header_left > 0) { BLOCK_WRITE_EXIT(len0-len); }
    }

    // Protocol check
    if (unlikely(le32_to_cpu(req->write_resp.data.protocol) != TERNFS_BLOCKS_RESP_PROTOCOL_VERSION)) {
        ternfs_info("bad blocks resp protocol, expected %*pE, got %*pE", 4, &TERNFS_BLOCKS_RESP_PROTOCOL_VERSION, 4, &req->write_resp.data.protocol);
        BLOCK_WRITE_EXIT(ternfs_error_to_linux(TERNFS_ERR_MALFORMED_RESPONSE));
    }

    // Error check
    if (unlikely(req->write_resp.data.kind == 0)) {
        int error_left = (TERNFS_BLOCKS_RESP_HEADER_SIZE + 2) - (int)write_resp_read;
        int read = HEADER_COPY(req->write_resp.buf + write_resp_read, error_left);
        error_left -= read;
        if (error_left > 0) { BLOCK_WRITE_EXIT(len0-len); }
        ternfs_info("writing block to bs=%016llx block_id=%016llx failed=%u", req->block_service_id, req->block_id, req->write_resp.data.error);
        // We immediately start writing, so any error here means that the socket is kaput
        int err = ternfs_error_to_linux(le16_to_cpu(req->write_resp.data.error));
        BLOCK_WRITE_EXIT(err);
    }

    // We can finally get the proof
    {
        int proof_left = (TERNFS_BLOCKS_RESP_HEADER_SIZE + 8) - (int)write_resp_read;
        int read = HEADER_COPY(req->write_resp.buf + write_resp_read, proof_left);
        proof_left -= read;
        if (proof_left > 0) {
            BLOCK_WRITE_EXIT(len0-len);
        }
    }

#undef HEADER_COPY

    BLOCK_WRITE_EXIT(len0-len);

#undef BLOCK_WRITE_EXIT
}

static int write_block_receive_req(read_descriptor_t* rd_desc, struct sk_buff* skb, unsigned int offset, size_t len) {
    return block_socket_receive_req_locked(write_block_receive_single_req, rd_desc, skb, offset, len);
}

static void write_block_sk_data_ready(struct sock* sk) {
    block_socket_sk_data_ready(&write_block_ops, sk);
}

int ternfs_write_block(
    void (*callback)(void* data, struct list_head* pages, u64 block_id, u64 proof, int err),
    void* data,
    struct ternfs_block_service* bs,
    u64 block_id,
    u64 certificate,
    u32 size,
    u32 crc,
    struct list_head* pages
) {
    int err;

    BUG_ON(list_empty(pages));

    // can't write
    if (unlikely(bs->flags & TERNFS_BLOCK_SERVICE_DONT_WRITE)) {
        ternfs_debug("could not write block given block flags %02x", bs->flags);
        return -EIO;
    }

    struct write_request* req = kmem_cache_alloc(write_request_cachep, GFP_KERNEL);
    if (!req) { err = -ENOMEM; goto out_err; }

    req->callback = callback;
    req->data = data;
    req->block_service_id = bs->id;
    req->block_id = block_id;
    req->crc = crc;
    req->size = size;
    req->certificate = certificate;
    memset(&req->write_resp.buf, 0, sizeof(req->write_resp.buf));
    list_replace_init(pages, &req->pages);
    list_first_entry(&req->pages, struct page, lru)->index = 0;
    list_first_entry(&req->pages, struct page, lru)->private = 1; // we use this to mark the first page

    err = block_socket_start_req(
        &write_block_ops,
        &req->breq,
        bs,
        TERNFS_BLOCKS_REQ_HEADER_SIZE + TERNFS_WRITE_BLOCK_REQ_SIZE + size,
        TERNFS_BLOCKS_RESP_HEADER_SIZE + TERNFS_WRITE_BLOCK_RESP_SIZE
    );
    if (err) {
        goto out_err_req;
    }

    return 0;

out_err_req:
    list_replace_init(&req->pages, pages);
    kmem_cache_free(write_request_cachep, req);
out_err:
    ternfs_info("couldn't start write block request, err=%d", err);
    return err;
}

// Timeout
// --------------------------------------------------------------------

// Periodically checks all sockets and schedules timeouts
static void do_timeout_sockets(struct work_struct* w) {
    u64 start = get_jiffies_64();
    timeout_sockets(&fetch_block_pages_witch_crc_ops);
    timeout_sockets(&write_block_ops);
    u64 end = get_jiffies_64();
    u64 dt = end - start;
    if (dt > MSECS_TO_JIFFIES(100)) {
        ternfs_warn("SLOW checking for socket timeouts %llums", jiffies64_to_msecs(dt));
    }
    queue_delayed_work(ternfs_fast_wq, &timeout_work, MSECS_TO_JIFFIES(1000));
}

// init/exit
// --------------------------------------------------------------------

int __init ternfs_block_init(void) {
    int err;
    fetch_request_cachep = kmem_cache_create(
        "ternfs_fetch_block_request_cache",
        sizeof(struct fetch_request),
        0,
        SLAB_RECLAIM_ACCOUNT | SLAB_MEM_SPREAD,
        fetch_request_constructor
    );
    if (!fetch_request_cachep) { err = -ENOMEM; goto out_err; }

    write_request_cachep = kmem_cache_create(
        "ternfs_write_block_request_cache",
        sizeof(struct write_request),
        0,
        SLAB_RECLAIM_ACCOUNT | SLAB_MEM_SPREAD,
        write_request_constructor
    );
    if (!write_request_cachep) { err = -ENOMEM; goto out_fetch_request; }

    block_ops_init(&fetch_block_pages_witch_crc_ops);
    block_ops_init(&write_block_ops);

    queue_work(ternfs_fast_wq, &timeout_work.work);

    return 0;

out_fetch_request:
    kmem_cache_destroy(fetch_request_cachep);
out_err:
    return err;
}

void __cold ternfs_block_exit(void) {
    ternfs_debug("block exit");

    // stop timing out sockets
    cancel_delayed_work_sync(&timeout_work);

    // If we're here, it means that all requests
    // must have finished. However we might still
    // be getting data down the sockets. So we let
    // the normal cleanup procedure run its course
    // by erroring out each socket and waiting
    // for all sockets to be released.

    block_ops_exit(&fetch_block_pages_witch_crc_ops);
    block_ops_exit(&write_block_ops);

    // clear caches
    kmem_cache_destroy(fetch_request_cachep);
    kmem_cache_destroy(write_request_cachep);
}
