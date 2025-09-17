// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "net.h"

#include <linux/version.h>

#include "log.h"
#include "super.h"
#include "trace.h"
#include "err.h"
#include "debugfs.h"
#include "metadata.h"
#include "inode.h"
#include "net_compat.h"

#define MSECS_TO_JIFFIES(_ms) ((_ms * HZ) / 1000)

unsigned ternfs_initial_shard_timeout_jiffies = MSECS_TO_JIFFIES(250);
unsigned ternfs_max_shard_timeout_jiffies = MSECS_TO_JIFFIES(2000);
unsigned ternfs_overall_shard_timeout_jiffies = MSECS_TO_JIFFIES(10000);
unsigned ternfs_initial_cdc_timeout_jiffies = MSECS_TO_JIFFIES(1000);
unsigned ternfs_max_cdc_timeout_jiffies = MSECS_TO_JIFFIES(10000);
unsigned ternfs_overall_cdc_timeout_jiffies = MSECS_TO_JIFFIES(120000);

static u64 WHICH_SHARD_IP = 0;

static struct ternfs_metadata_request* get_metadata_request(struct ternfs_metadata_socket* s, u64 request_id) __must_hold(s->lock) {
    struct rb_node* node = s->requests.rb_node;
    while (node) {
        struct ternfs_metadata_request* req = container_of(node, struct ternfs_metadata_request, node);
        if (request_id < req->request_id) { node = node->rb_left; }
        else if (request_id > req->request_id) { node = node->rb_right; }
        else { return req; }
    }
    return NULL;
}

static void insert_metadata_request(struct ternfs_metadata_socket* s, struct ternfs_metadata_request* req) __must_hold(s->lock) {
    u64 request_id = req->request_id;
    struct rb_node** new = &s->requests.rb_node;
    struct rb_node* parent = NULL;
    while (*new) {
        struct ternfs_metadata_request* this = container_of(*new, struct ternfs_metadata_request, node);
        parent = *new;
        if (request_id < this->request_id) { new = &((*new)->rb_left); }
        else if (request_id > this->request_id) { new = &((*new)->rb_right); }
        else { ternfs_error("dup id %lld vs. %lld %p vs. %p\n", request_id, this->request_id, req, this); BUG(); return; }
    }

    rb_link_node(&req->node, parent, new);
    rb_insert_color(&req->node, &s->requests);
}

void ternfs_metadata_request_init(
    struct ternfs_metadata_socket* sock,
    struct ternfs_metadata_request* req,
    struct ternfs_metadata_request_state* state
) {
    state->start_t = get_jiffies_64();
    state->next_timeout = 0;
    state->attempts = 0;
    state->which_addr = WHICH_SHARD_IP++;
    req->skb = NULL;

    spin_lock_bh(&sock->lock);
    insert_metadata_request(sock, req);
    spin_unlock_bh(&sock->lock);
}

#define INC_STAT_COUNTER(req, kind, _NAME) do { \
        if ((req)->flags & TERNFS_METADATA_REQUEST_ASYNC_GETATTR) { break; } \
        u64 stats_idx = ((req)->shard < 0) ? \
            ((u64)__ternfs_cdc_kind_index_mappings[kind] * TERNFS_COUNTERS_NUM_COUNTERS) : \
            ((u64)__ternfs_shard_kind_index_mappings[kind] * TERNFS_COUNTERS_NUM_COUNTERS + (u64)(req)->shard * TERNFS_SHARD_KIND_MAX * TERNFS_COUNTERS_NUM_COUNTERS); \
        if ((req)->shard < 0) { \
            atomic64_add(1, (atomic64_t*)&cdc_counters[stats_idx + TERNFS_COUNTERS_IDX_##_NAME]); \
        } else { \
            atomic64_add(1, (atomic64_t*)&shard_counters[stats_idx + TERNFS_COUNTERS_IDX_##_NAME]); \
        } \
    } while (0)

static void sock_readable(struct sock* sk) {
    struct ternfs_metadata_socket* s;
    struct sk_buff* skb;
    int err;

    read_lock_bh(&sk->sk_callback_lock);
    s = (struct ternfs_metadata_socket*)sk->sk_user_data;
    BUG_ON(!s);
    for (;;) {
        skb = COMPAT_SKB_RECV_UDP(sk, MSG_DONTWAIT, &err);
        if (!skb) {
            read_unlock_bh(&sk->sk_callback_lock);
            return;
        }

        if (unlikely(skb_checksum_complete(skb))) {
            ternfs_warn("dropping request with bad checksum");
            goto drop_skb;
        }

        // u32 protocol + u64 req_id + u8 kind. Note that we check the protocol later on.
        // We just need the request id here, and the kind for logging.
        if (unlikely(skb->len < 13)) {
            ternfs_warn("dropping runt ternfs request");
            goto drop_skb;
        }
        if (unlikely(skb->len > TERNFS_MAX_MTU)) {
            ternfs_warn("dropping overlong ternfs request");
            goto drop_skb;
        }
        u32 resp_len = skb->len;
        __le64 request_id_le;
        BUG_ON(skb_copy_bits(skb, 4, &request_id_le, 8) != 0);
        u64 request_id = le64_to_cpu(request_id_le);
        u8 kind;
        BUG_ON(skb_copy_bits(skb, 12, &kind, 1) != 0);
        int bincode_err = 0;
        if (kind == 0 && skb->len >= (4 + 8 + 1 + 2)) {
            __le16 err;
            BUG_ON(skb_copy_bits(skb, 4 + 8 + 1, &err, 2) != 0);
            bincode_err = le16_to_cpu(err);
        }
        struct ternfs_metadata_request* req;
        s16 shard;

        // We want to remove the request quickly and not call completions while holding the lock
        // Completions functions also remove from the rb_tree but in a safe maner using ternfs_metadata_remove_request
        spin_lock_bh(&s->lock);
        req = get_metadata_request(s, request_id);
        struct ternfs_inode* getattr_enode = NULL;
        if (req) {
            BUG_ON(req->skb);
            shard = req->shard; // We save this here because I think `req` might be gone by the time we trace below.
            req->skb = skb;     // We could trace while holding the spinlock but I figured it's a bit better to not
            skb = NULL;         // do that.
            rb_erase(&req->node, &s->requests);
            if (req->flags & TERNFS_METADATA_REQUEST_ASYNC_GETATTR) {
                getattr_enode = container_of(req, struct ternfs_inode, getattr_async_req);
                // we must ihold it as it could timeout and be freed before we get to it.
                ihold(&getattr_enode->inode);
            }
        }
        spin_unlock_bh(&s->lock);
        if (req) {
            if (getattr_enode) {
                // We try to cancel the timeout. If we succeed we immediately need to schedule completion.
                // If we failed we timed out anyway. In this case skb will not leak as it's protected
                // in completion function by removing the request from the rb_tree
                bool was_pending = cancel_delayed_work_sync(&getattr_enode->getattr_async_work);
                if (was_pending) {
                    schedule_work(&getattr_enode->getattr_async_work.work);
                }
                iput(&getattr_enode->inode);
            } else {
                struct ternfs_metadata_sync_request* sreq = container_of(req, struct ternfs_metadata_sync_request, req);
                complete(&sreq->comp);
            }
            struct sockaddr_in addr = {0};
            trace_eggsfs_metadata_request(&addr, request_id, 0, shard, kind, 0, resp_len, TERNFS_METADATA_REQUEST_DONE, bincode_err);
        } else {
            ternfs_debug("could not find request id %llu (probably interrupted or late after multiple attempts)", request_id);
        }

        if (skb) {
drop_skb:
            kfree_skb(skb);
        }
    }
}

// timer async request expired
// if skb then retrun
// requeue
// whoever wants to remove request needs to handle timer and workqueue

void ternfs_net_shard_free_socket(struct ternfs_metadata_socket* s) {
    // TODO anything else?
    write_lock_bh(&s->sock->sk->sk_callback_lock);
    s->sock->sk->sk_data_ready = s->original_data_ready;
    s->original_data_ready = NULL;
    write_unlock_bh(&s->sock->sk->sk_callback_lock);
    sock_release(s->sock);
}

int ternfs_init_shard_socket(struct ternfs_metadata_socket* s) {
    struct sockaddr_in addr;
    int err;

    err = sock_create_kern(&init_net, PF_INET, SOCK_DGRAM, IPPROTO_UDP, &s->sock);
    if (err) { goto out_err; }

    int new_rcvbuf_size = 1024 * 1024;
    int optlen = sizeof(new_rcvbuf_size);
    err = COMPAT_SET_SOCKOPT(s->sock, SOL_SOCKET, SO_RCVBUF, &new_rcvbuf_size, optlen);
    if (err) { goto out_socket; }

    write_lock_bh(&s->sock->sk->sk_callback_lock);
    BUG_ON(s->sock->sk->sk_user_data);
    s->sock->sk->sk_user_data = s;
    s->original_data_ready = s->sock->sk->sk_data_ready;
    s->sock->sk->sk_data_ready = sock_readable;
    write_unlock_bh(&s->sock->sk->sk_callback_lock);

    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = 0;
    addr.sin_port = 0;
    err = s->sock->ops->bind(s->sock, (struct sockaddr*)&addr, sizeof(addr));
    if (err) { goto out_socket; }

    s->requests = RB_ROOT;
    spin_lock_init(&s->lock);

    return 0;

out_socket:
    sock_release(s->sock);
out_err:
    return err;
}

static bool ternfs_shard_fill_msghdr(struct msghdr *msg, struct sockaddr_in* addr, const atomic64_t* addr_data) {
    u64 v1 = atomic64_read(addr_data);

    __be32 ip = ternfs_get_addr_ip(v1);
    if (unlikely(ip == 0)) {
        return false;
    }
    __be16 port = ternfs_get_addr_port(v1);

    addr->sin_addr.s_addr = ip;
    addr->sin_port = port;
    addr->sin_family = AF_INET;

    msg->msg_name = addr;
    msg->msg_namelen = sizeof(struct sockaddr_in);
    msg->msg_control = NULL;
    msg->msg_controllen = 0;
    msg->msg_flags = 0;
    return true;
}

// jiffies only have millisecond precision, but we still use nanoseconds as the
// format is nicer and makes the buckets a bit more precise.
// |1 below drops the 0 value to the smallest bucket
static void inc_latency_bucket(s16 shard_id, u8 kind, u64 elapsed) {
    u64 latencies_idx = (shard_id < 0) ?
        ((u64)__ternfs_cdc_kind_index_mappings[kind] * TERNFS_LATENCIES_NUM_BUCKETS) :
        ((u64)__ternfs_shard_kind_index_mappings[kind] * TERNFS_LATENCIES_NUM_BUCKETS + (u64)shard_id * TERNFS_SHARD_KIND_MAX * TERNFS_LATENCIES_NUM_BUCKETS);
    u64 hist_bucket = 0;
    u64 elapsed_nsecs = jiffies_to_nsecs(elapsed) | 1;
    __asm__("lzcnt %1,%0" : "=r"(hist_bucket) : "r"(elapsed_nsecs));
    if (shard_id < 0) {
        atomic64_add(1, (atomic64_t*)&cdc_latencies[latencies_idx + hist_bucket]);
    } else {
        atomic64_add(1, (atomic64_t*)&shard_latencies[latencies_idx + hist_bucket]);
    }
}

int ternfs_metadata_send_request(
    struct ternfs_metadata_socket* sock,
    const atomic64_t* addr_data1,
    const atomic64_t* addr_data2,
    struct ternfs_metadata_request* req,
    void* data,
    u32 len,
    struct ternfs_metadata_request_state* state
) {
    state->attempts++;

    u8 kind = *((u8*)data + 12);

    int err;

    bool is_cdc = req->shard < 0;

    const atomic64_t* addr_data[2] = {addr_data1, addr_data2};

    unsigned overall_timeout = is_cdc ? ternfs_overall_cdc_timeout_jiffies : ternfs_overall_shard_timeout_jiffies;
    unsigned max_timeout = is_cdc ? ternfs_max_cdc_timeout_jiffies : ternfs_max_shard_timeout_jiffies;
    unsigned initial_timeout = is_cdc ? ternfs_initial_cdc_timeout_jiffies : ternfs_initial_shard_timeout_jiffies;
    if ((get_jiffies_64() - state->start_t) > overall_timeout) {
        err = -ETIMEDOUT;
        INC_STAT_COUNTER(req, kind, TIMEOUTS);
        goto out_err;
    }

    struct kvec vec;
    vec.iov_base = data;
    vec.iov_len = len;

    struct sockaddr_in addr;
    struct msghdr msg;

    if (unlikely(!ternfs_shard_fill_msghdr(&msg, &addr, addr_data[state->which_addr%2]))) {
        if (!ternfs_shard_fill_msghdr(&msg, &addr, addr_data[0])) {
            ternfs_warn("could not find any shard addresses! does everything look good in registry?");
            err = -EIO;
            goto out_err_no_trace; // we have no addr
        }
    }
    state->which_addr++;

    trace_eggsfs_metadata_request(&addr, req->request_id, len, req->shard, kind, state->attempts, 0, TERNFS_METADATA_REQUEST_ATTEMPT, 0);
    INC_STAT_COUNTER(req, kind, ATTEMPTED);

    ternfs_debug("sending: req_id=%llu shard_id=%d kind_str=%s kind=%d attempts=%d elapsed=%llums addr=%pI4:%d", req->request_id, req->shard,  is_cdc ? ternfs_cdc_kind_str(kind) : ternfs_shard_kind_str(kind), kind, state->attempts, jiffies64_to_msecs(get_jiffies_64() - state->start_t), &addr.sin_addr, ntohs(addr.sin_port));
    err = kernel_sendmsg(sock->sock, &msg, &vec, 1, len);
    if (unlikely(err < 0)) {
        if (err != -ERESTARTSYS) {
            ternfs_info("could not send: %d", err);
            INC_STAT_COUNTER(req, kind, NET_FAILURES);
        }
        // For ENETUNREACH, we pretend to have sent it, and then the timeout
        // mechanism will retry, since this might be fixed with time.
	// For EPERM, at the moment it is not entirely clear why it is returned
	// occasionally.
        if (err == -ENETUNREACH || err == -EPERM) {
            err = len;
        } else {
            goto out_err_no_latency; // we didn't really get a response here, so no sense increasing the latency bucket
        }
    }
    BUG_ON(err != len);
    err = 0;

    // update next timeout, 1.5 exponential backoff
    state->next_timeout = (state->next_timeout == 0) ? initial_timeout : min(max_timeout, (state->next_timeout*3)/2);

    BUG_ON(err != 0);
    return err;

out_err:
    inc_latency_bucket(req->shard, kind, get_jiffies_64() - state->start_t);
out_err_no_latency:
    {
        struct sockaddr_in addr = {0};
        trace_eggsfs_metadata_request(&addr, req->request_id, 0, -2, kind, 0, 0, TERNFS_METADATA_REQUEST_DONE, err);
    }
out_err_no_trace:
    // it is very important we search and remove request rather than just remove from the tree by node
    // this could be called after timeout and we might race with removal happening from socket callback
    ternfs_metadata_remove_request(sock, req->request_id);
    // Free the skb if it's set. It is possible request got completed and skb set between we gave up on completion and
    // came here to remove the request.
    if (req->skb) {
        // if the skb is set then completion is either set or will be set prompty. we need to wait for it or we risk
        // use after free for completion
        // we should not hit this code path for async requests. they dont retry the same way
        BUG_ON(req->flags & TERNFS_METADATA_REQUEST_ASYNC_GETATTR);
        struct ternfs_metadata_sync_request* sreq = container_of(req, struct ternfs_metadata_sync_request, req);
        wait_for_completion(&sreq->comp);
        kfree_skb(req->skb);
        req->skb = NULL;
    }
    return err;
}

void ternfs_metadata_remove_request(struct ternfs_metadata_socket* sock, u64 request_id) {
    spin_lock_bh(&sock->lock);
    struct ternfs_metadata_request* req = get_metadata_request(sock, request_id);
    if (req) {
        rb_erase(&req->node, &sock->requests);
    }
    spin_unlock_bh(&sock->lock);
}

// If this function returns succesfull, we're done, and the request
// is out of the tree.
// If it returns -ETIMEDOUT, we should call `ternfs_metadata_send_request`
// again.
// Any other error, we're done and the request is out of the tree.
static struct sk_buff* ternfs_metadata_wait_request(
    struct ternfs_metadata_socket* sock,
    struct ternfs_metadata_sync_request* req,
    s16 shard,
    u8 kind,
    struct ternfs_metadata_request_state* state
) {
    u64 timeout_s = state->next_timeout/HZ;
    if (timeout_s > 10) {
        ternfs_warn("about to wait for request for a long time (%llu seconds)", timeout_s);
    }
    int ret = wait_for_completion_timeout(&req->comp, state->next_timeout);
    // We got a response, this is the happy path.
    if (likely(ret > 0)) {
        BUG_ON(!req->req.skb);
        u64 elapsed = get_jiffies_64() - state->start_t;
        u64 late_threshold = (req->req.shard < 0) ? MSECS_TO_JIFFIES(60000) : MSECS_TO_JIFFIES(1000);
        const char* kind_str = (req->req.shard < 0) ? ternfs_cdc_kind_str(kind) : ternfs_shard_kind_str(kind);
        if (unlikely(elapsed > late_threshold)) {
            ternfs_info("late request: req_id=%llu shard_id=%d kind_str=%s kind=%d attempts=%d elapsed=%llums", req->req.request_id, req->req.shard, kind_str, kind, state->attempts, jiffies64_to_msecs(elapsed));
        } else {
            ternfs_debug("completed request req_id=%llu shard_id=%d kind_str=%s kind=%d attempts=%d elapsed=%llums", req->req.request_id, req->req.shard, kind_str, kind, state->attempts, jiffies64_to_msecs(elapsed));
        }
        inc_latency_bucket(shard, kind, elapsed);
        int bincode_err = 0;
        if (kind == 0 && req->req.skb->len >= (4 + 8 + 1 + 2)) {
            __le16 err;
            BUG_ON(skb_copy_bits(req->req.skb, 4 + 8 + 1, &err, 2) != 0);
            bincode_err = le16_to_cpu(err);
        }
        if (unlikely(bincode_err && ternfs_unexpected_error(bincode_err))) {
            INC_STAT_COUNTER(&req->req, kind, FAILURES);
            const char* kind_str = (req->req.shard < 0) ? ternfs_cdc_kind_str(kind) : ternfs_shard_kind_str(kind);
            ternfs_warn("unexpected error: req_id=%llu shard_id=%d kind_str=%s kind=%d err_str=%s err=%d", req->req.request_id, req->req.shard, kind_str, kind, ternfs_err_str(bincode_err), bincode_err);
        }
        INC_STAT_COUNTER(&req->req, kind, COMPLETED);
        return req->req.skb;
    }
    // Actual error, remove the request
    if (unlikely(ret)) {
        ternfs_warn("could not wait for request %llu: %d", req->req.request_id, ret);
        spin_lock_bh(&sock->lock);
        rb_erase(&req->req.node, &sock->requests);
        spin_unlock_bh(&sock->lock);
        return ERR_PTR(ret);
    }
    // Benign timeout
    return ERR_PTR(-ETIMEDOUT);
}

// shard_id is just used for debugging/event tracing, and should be -1 if we're going to the CDC
struct sk_buff* ternfs_metadata_request(
    struct ternfs_metadata_socket* sock,
    s16 shard_id,
    u64 req_id,
    void* p,
    u32 len,
    const atomic64_t* addr_data1,
    const atomic64_t* addr_data2,
    u32* attempts
) {
    struct ternfs_metadata_sync_request req;
    init_completion(&req.comp);
    req.req.request_id = req_id;
    req.req.flags = 0;
    req.req.shard = shard_id;

    struct ternfs_metadata_request_state state;
    struct sk_buff* skb = NULL;

    ternfs_metadata_request_init(sock, &req.req, &state);

    u8 kind = *((u8*)p + 12);

    for (;;) {
        int err = ternfs_metadata_send_request(sock, addr_data1, addr_data2, &req.req, p, len, &state);
        if (err < 0) {
            skb = ERR_PTR(err);
            goto out;
        }
        skb = ternfs_metadata_wait_request(sock, &req, shard_id, kind, &state);
        if (IS_ERR(skb)) {
            if (PTR_ERR(skb) == -ETIMEDOUT) { continue; }
            goto out;
        }
        break;
    }

out:
    *attempts = state.attempts;
    return skb;
}
