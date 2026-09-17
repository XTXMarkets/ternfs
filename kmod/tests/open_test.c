// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/fs.h>
#include <linux/module.h>
#include <linux/string.h>

#include "../file.h"

MODULE_LICENSE("GPL");

static unsigned trace_calls;
static unsigned traces_while_locked;
static struct ternfs_inode test_enode;

static void test_trace_inode_lock(struct inode* inode, const char* operation) {
    trace_calls++;
    if (inode_is_locked(inode)) { traces_while_locked++; }
}

// Avoid pulling trace definitions into this standalone test. The production
// callback still invokes the boundary at the same points.
#define _TRACE_EGGFS_H
#define trace_eggsfs_inode_lock(inode, event, operation) \
    test_trace_inode_lock(inode, operation)
#include "../open.c"
#undef trace_eggsfs_inode_lock
#undef _TRACE_EGGFS_H

static void prepare_file_state(struct ternfs_inode* enode, int status) {
    memset(&enode->file, 0, sizeof(enode->file));
    enode->file.status = status;
    enode->file.spans.__ino = 0x1122334455667788ULL;
    init_rwsem(&enode->file.spans.__lock);
    enode->file.cookie = 0x8877665544332211ULL;
    atomic_set(&enode->file.transient_err, -EIO);
    enode->file.writing_span = (struct ternfs_transient_span*)0x12340000UL;
    sema_init(&enode->file.flushing_span_sema, 1);
    enode->file.owner = (struct task_struct*)0x56780000UL;
    enode->file.mm = (struct mm_struct*)0x9abc0000UL;
}

static int call_and_check(
    struct ternfs_inode* enode, fmode_t mode, int expected_err,
    const struct ternfs_inode_file* expected, const char* test
) {
    struct file filp;
    int err;

    memset(&filp, 0, sizeof(filp));
    filp.f_mode = mode;
    trace_calls = 0;
    traces_while_locked = 0;

    err = ternfs_file_open(&enode->inode, &filp);
    if (err != expected_err) {
        pr_err(
            "ternfs-open-test: %s returned %d, expected %d\n",
            test, err, expected_err
        );
        return -EINVAL;
    }
    if (memcmp(&enode->file, expected, sizeof(*expected)) != 0) {
        pr_err("ternfs-open-test: %s changed unexpected file state\n", test);
        return -EINVAL;
    }
    if (trace_calls != 2 || traces_while_locked != 0) {
        pr_err(
            "ternfs-open-test: %s trace/lock mismatch: calls=%u locked=%u\n",
            test, trace_calls, traces_while_locked
        );
        return -EINVAL;
    }
    if (!inode_trylock(&enode->inode)) {
        pr_err("ternfs-open-test: %s returned with inode locked\n", test);
        return -EINVAL;
    }
    inode_unlock(&enode->inode);
    return 0;
}

static int run_case(
    struct ternfs_inode* enode, int status, fmode_t mode,
    int expected_err, int expected_status, const char* test
) {
    struct ternfs_inode_file expected;

    prepare_file_state(enode, status);
    expected = enode->file;
    expected.status = expected_status;
    return call_and_check(enode, mode, expected_err, &expected, test);
}

static int run_reopen_sequence(struct ternfs_inode* enode) {
    struct ternfs_inode_file expected;
    unsigned i;
    int err;

    prepare_file_state(enode, TERNFS_FILE_STATUS_WRITING);
    expected = enode->file;
    for (i = 0; i < 3; i++) {
        err = call_and_check(
            enode, FMODE_READ, -EBUSY, &expected, "repeated read-only reopen"
        );
        if (err) { return err; }
    }
    return call_and_check(
        enode, FMODE_READ | FMODE_WRITE, 0, &expected,
        "writable reopen after rejection"
    );
}

static int __init open_test_init(void) {
    struct ternfs_inode* enode = &test_enode;
    int err;

    memset(enode, 0, sizeof(*enode));
    inode_init_once(&enode->inode);

    err = run_case(
        enode, TERNFS_FILE_STATUS_NONE, FMODE_READ,
        0, TERNFS_FILE_STATUS_READING, "NONE read-only"
    );
    if (err) { return err; }
    err = run_case(
        enode, TERNFS_FILE_STATUS_NONE, FMODE_WRITE,
        0, TERNFS_FILE_STATUS_READING, "NONE write-only"
    );
    if (err) { return err; }
    err = run_case(
        enode, TERNFS_FILE_STATUS_READING, FMODE_READ,
        0, TERNFS_FILE_STATUS_READING, "READING read-only"
    );
    if (err) { return err; }
    err = run_case(
        enode, TERNFS_FILE_STATUS_READING, FMODE_WRITE,
        0, TERNFS_FILE_STATUS_READING, "READING write-only"
    );
    if (err) { return err; }
    err = run_case(
        enode, TERNFS_FILE_STATUS_WRITING, FMODE_READ,
        -EBUSY, TERNFS_FILE_STATUS_WRITING, "WRITING read-only"
    );
    if (err) { return err; }
    err = run_case(
        enode, TERNFS_FILE_STATUS_WRITING, FMODE_WRITE,
        0, TERNFS_FILE_STATUS_WRITING, "WRITING write-only"
    );
    if (err) { return err; }
    err = run_case(
        enode, TERNFS_FILE_STATUS_WRITING, FMODE_READ | FMODE_WRITE,
        0, TERNFS_FILE_STATUS_WRITING, "WRITING read/write"
    );
    if (err) { return err; }
    err = run_reopen_sequence(enode);
    if (err) { return err; }

    pr_info(
        "ternfs-open-test: PASS (states, modes, repeated reopen, ownership, locks)\n"
    );
    return 0;
}

static void __exit open_test_exit(void) {}

module_init(open_test_init);
module_exit(open_test_exit);
