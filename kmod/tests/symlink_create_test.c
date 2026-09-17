// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/fs.h>
#include <linux/module.h>
#include <linux/string.h>
#include <linux/uio.h>

#include "../file.h"

MODULE_LICENSE("GPL");

enum test_event {
    EVENT_TRACE_BEFORE,
    EVENT_WRITE,
    EVENT_TRACE_AFTER,
    EVENT_FLUSH,
    EVENT_IPUT,
    EVENT_PUBLISH,
};

static struct ternfs_inode test_enode;
static struct super_block test_super;
static struct dentry test_dentry;
static const char test_path[] = "target/path";
static enum test_event events[8];
static unsigned event_count;
static unsigned trace_calls;
static unsigned traces_while_locked;
static unsigned write_calls;
static unsigned flush_calls;
static unsigned iput_calls;
static unsigned publish_calls;
static int write_error;
static int flush_error;
static bool write_contract_ok;
static bool flush_contract_ok;
static bool iput_contract_ok;
static bool publish_contract_ok;

static void add_event(enum test_event event) {
    if (event_count < ARRAY_SIZE(events)) { events[event_count++] = event; }
}

static void test_trace_inode_lock(struct inode* inode, const char* operation) {
    add_event(trace_calls ? EVENT_TRACE_AFTER : EVENT_TRACE_BEFORE);
    trace_calls++;
    if (inode_is_locked(inode)) { traces_while_locked++; }
}

static ssize_t test_file_write(
    struct ternfs_inode* enode, int flags, loff_t* ppos,
    struct iov_iter* from
) {
    char observed[sizeof(test_path)] = {};
    size_t expected = sizeof(test_path) - 1;
    size_t copied;

    add_event(EVENT_WRITE);
    write_calls++;
    copied = copy_from_iter(observed, expected, from);
    write_contract_ok =
        inode_is_locked(&enode->inode) &&
        enode->file.status == TERNFS_FILE_STATUS_WRITING &&
        flags == 0 && *ppos == 0 && copied == expected &&
        iov_iter_count(from) == 0 &&
        memcmp(observed, test_path, expected) == 0;
    if (write_error) { return write_error; }
    *ppos += copied;
    return copied;
}

static int test_file_flush(struct ternfs_inode* enode, struct dentry* dentry) {
    add_event(EVENT_FLUSH);
    flush_calls++;
    flush_contract_ok =
        !inode_is_locked(&enode->inode) && dentry == &test_dentry;
    if (flush_error) { return flush_error; }
    enode->file.status = TERNFS_FILE_STATUS_READING;
    return 0;
}

static void test_iput(struct inode* inode) {
    add_event(EVENT_IPUT);
    iput_calls++;
    iput_contract_ok =
        inode == &test_enode.inode && !inode_is_locked(inode) &&
        inode->i_nlink == 0 && atomic_read(&inode->i_count) == 2;
    iput(inode);
}

static void test_d_instantiate(struct dentry* dentry, struct inode* inode) {
    add_event(EVENT_PUBLISH);
    publish_calls++;
    publish_contract_ok =
        dentry == &test_dentry && inode == &test_enode.inode &&
        !inode_is_locked(inode) && inode->i_nlink == 1 &&
        atomic_read(&inode->i_count) == 2;
}

// Keep trace definitions out of this standalone module while preserving calls
// at the production boundary.
#define _TRACE_EGGFS_H
#define trace_eggsfs_inode_lock(inode, event, operation) \
    test_trace_inode_lock(inode, operation)
#define ternfs_file_write test_file_write
#define ternfs_file_flush test_file_flush
#define iput test_iput
#define d_instantiate test_d_instantiate
#include "../symlink_create.c"
#undef d_instantiate
#undef iput
#undef ternfs_file_flush
#undef ternfs_file_write
#undef trace_eggsfs_inode_lock
#undef _TRACE_EGGFS_H

static void prepare_case(int new_write_error, int new_flush_error) {
    memset(&test_enode.file, 0, sizeof(test_enode.file));
    test_enode.file.status = TERNFS_FILE_STATUS_NONE;
    test_enode.inode.i_mode = S_IFLNK | 0777;
    test_enode.inode.i_state = 0;
    test_enode.inode.__i_nlink = 1;
    atomic_long_set(&test_super.s_remove_count, 0);
    atomic_set(&test_enode.inode.i_count, 1);
    ihold(&test_enode.inode);

    memset(events, 0, sizeof(events));
    event_count = 0;
    trace_calls = 0;
    traces_while_locked = 0;
    write_calls = 0;
    flush_calls = 0;
    iput_calls = 0;
    publish_calls = 0;
    write_error = new_write_error;
    flush_error = new_flush_error;
    write_contract_ok = false;
    flush_contract_ok = false;
    iput_contract_ok = false;
    publish_contract_ok = false;
}

static int check_events(
    const enum test_event* expected, unsigned count, const char* test
) {
    if (event_count != count ||
        memcmp(events, expected, count * sizeof(*expected)) != 0) {
        pr_err(
            "ternfs-symlink-create-test: %s event count/order mismatch\n",
            test
        );
        return -EINVAL;
    }
    return 0;
}

static int check_unlocked(const char* test) {
    if (!inode_trylock(&test_enode.inode)) {
        pr_err(
            "ternfs-symlink-create-test: %s returned with inode locked\n",
            test
        );
        return -EINVAL;
    }
    inode_unlock(&test_enode.inode);
    return 0;
}

static int test_write_failure(void) {
    static const enum test_event expected[] = {
        EVENT_TRACE_BEFORE, EVENT_WRITE, EVENT_TRACE_AFTER, EVENT_IPUT,
    };
    int err;

    prepare_case(-EFAULT, 0);
    err = ternfs_finish_symlink(&test_enode, &test_dentry, test_path);
    if (err != -EFAULT || write_calls != 1 || flush_calls != 0 ||
        publish_calls != 0 || iput_calls != 1 || !write_contract_ok ||
        !iput_contract_ok || atomic_read(&test_enode.inode.i_count) != 1 ||
        test_enode.inode.i_nlink != 0 ||
        atomic_long_read(&test_super.s_remove_count) != 1 ||
        trace_calls != 2 ||
        traces_while_locked != 0) {
        pr_err("ternfs-symlink-create-test: write failure mismatch\n");
        return -EINVAL;
    }
    err = check_events(expected, ARRAY_SIZE(expected), "write failure");
    return err ? err : check_unlocked("write failure");
}

static int test_flush_failure(void) {
    static const enum test_event expected[] = {
        EVENT_TRACE_BEFORE, EVENT_WRITE, EVENT_TRACE_AFTER,
        EVENT_FLUSH, EVENT_IPUT,
    };
    int err;

    prepare_case(0, -ENOSPC);
    err = ternfs_finish_symlink(&test_enode, &test_dentry, test_path);
    if (err != -ENOSPC || write_calls != 1 || flush_calls != 1 ||
        publish_calls != 0 || iput_calls != 1 || !write_contract_ok ||
        !flush_contract_ok || !iput_contract_ok ||
        atomic_read(&test_enode.inode.i_count) != 1 ||
        test_enode.inode.i_nlink != 0 ||
        atomic_long_read(&test_super.s_remove_count) != 1 ||
        trace_calls != 2 ||
        traces_while_locked != 0) {
        pr_err("ternfs-symlink-create-test: flush failure mismatch\n");
        return -EINVAL;
    }
    err = check_events(expected, ARRAY_SIZE(expected), "flush failure");
    return err ? err : check_unlocked("flush failure");
}

static int test_success(void) {
    static const enum test_event expected[] = {
        EVENT_TRACE_BEFORE, EVENT_WRITE, EVENT_TRACE_AFTER,
        EVENT_FLUSH, EVENT_PUBLISH,
    };
    int err;

    prepare_case(0, 0);
    err = ternfs_finish_symlink(&test_enode, &test_dentry, test_path);
    if (err != 0 || write_calls != 1 || flush_calls != 1 ||
        publish_calls != 1 || iput_calls != 0 || !write_contract_ok ||
        !flush_contract_ok || !publish_contract_ok ||
        atomic_read(&test_enode.inode.i_count) != 2 ||
        test_enode.inode.i_nlink != 1 ||
        atomic_long_read(&test_super.s_remove_count) != 0 ||
        test_enode.file.status != TERNFS_FILE_STATUS_READING ||
        trace_calls != 2 || traces_while_locked != 0) {
        pr_err("ternfs-symlink-create-test: success mismatch\n");
        return -EINVAL;
    }
    err = check_events(expected, ARRAY_SIZE(expected), "success");
    return err ? err : check_unlocked("success");
}

static int __init symlink_create_test_init(void) {
    int err;

    memset(&test_enode, 0, sizeof(test_enode));
    memset(&test_super, 0, sizeof(test_super));
    inode_init_once(&test_enode.inode);
    test_enode.inode.i_sb = &test_super;

    err = test_write_failure();
    if (err) { return err; }
    err = test_flush_failure();
    if (err) { return err; }
    err = test_success();
    if (err) { return err; }

    // The final successful case leaves the new reference notionally owned by
    // the substituted dentry and one observer reference held by this test.
    atomic_set(&test_enode.inode.i_count, 0);
    pr_info(
        "ternfs-symlink-create-test: PASS "
        "(write/flush errors, iput, lock, contents, publish order)\n"
    );
    return 0;
}

static void __exit symlink_create_test_exit(void) {}

module_init(symlink_create_test_init);
module_exit(symlink_create_test_exit);
