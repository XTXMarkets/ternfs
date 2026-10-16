// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <linux/fs.h>
#include <linux/module.h>
#include <linux/mount.h>
#include <linux/string.h>
#include <linux/timekeeping.h>

#include "../file.h"
#include "../inode_compat.h"
#include "../metadata.h"

MODULE_LICENSE("GPL");

static unsigned trace_calls;
static unsigned traces_while_locked;
static struct ternfs_inode test_enode;
static struct super_block test_sb;
unsigned ternfs_atime_update_interval_sec;

static u64 now_ns;
static u64 refreshed_atime_ns;
static int refresh_err;
static int set_time_err;
static unsigned refresh_calls;
static unsigned set_time_calls;
static bool bad_request;

static u64 test_now(void) { return now_ns; }

static int test_getattr(struct ternfs_inode* enode, int cache_timeout_type) {
    refresh_calls++;
    if (cache_timeout_type != ATTR_CACHE_NO_TIMEOUT ||
        !inode_is_locked(&enode->inode)) {
        bad_request = true;
    }
    if (refresh_err) { return refresh_err; }
    inode_set_atime_to_ts(&enode->inode, ns_to_timespec64(refreshed_atime_ns));
    return 0;
}

static int test_set_time(struct ternfs_fs_info* info, u64 ino, u64 mtime, u64 atime) {
    set_time_calls++;
    if (info != test_sb.s_fs_info || ino != test_enode.inode.i_ino ||
        mtime != 0 || atime != (now_ns | (1ull << 63)) ||
        !inode_is_locked(&test_enode.inode)) {
        bad_request = true;
    }
    return set_time_err;
}

static void test_trace_inode_lock(struct inode* inode, const char* operation) {
    trace_calls++;
    if (inode_is_locked(inode)) { traces_while_locked++; }
}

// Avoid pulling trace definitions into this standalone test. The production
// callback still invokes the boundary at the same points.
#define _TRACE_EGGFS_H
#define trace_eggsfs_inode_lock(inode, event, operation) \
    test_trace_inode_lock(inode, operation)
#define ktime_get_real_ns test_now
#define ternfs_do_getattr test_getattr
#define ternfs_shard_set_time test_set_time
#include "../open.c"
#undef ternfs_shard_set_time
#undef ternfs_do_getattr
#undef ktime_get_real_ns
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

static int run_atime_cases(struct ternfs_inode* enode) {
    const u64 old = 100ull * NSEC_PER_SEC;
    const u64 now = 200ull * NSEC_PER_SEC + 500;
    const struct {
        const char* name;
        bool disabled;
        bool transient;
        bool unthrottled;
        unsigned flags;
        u64 cached;
        u64 refreshed;
        int refresh_error;
        int set_error;
        unsigned refreshes;
        unsigned writes;
        u64 expected_atime;
    } cases[] = {
        {.name = "default off", .disabled = true, .expected_atime = old},
        {.name = "read-only noatime mount", .refreshes = 1, .writes = 1, .expected_atime = now},
        {.name = "O_NOATIME", .flags = O_NOATIME, .expected_atime = old},
        {.name = "transient writer", .transient = true, .expected_atime = old},
        {.name = "cached throttle", .cached = now - 10 * NSEC_PER_SEC, .expected_atime = now - 10 * NSEC_PER_SEC},
        {.name = "peer already updated", .refreshed = now - 10 * NSEC_PER_SEC, .refreshes = 1, .expected_atime = now - 10 * NSEC_PER_SEC},
        {.name = "zero interval", .unthrottled = true, .refreshes = 1, .writes = 1, .expected_atime = now},
        {.name = "atime equal to now", .unthrottled = true, .refreshed = now, .refreshes = 1, .expected_atime = now},
        {.name = "atime one nanosecond ahead", .unthrottled = true, .refreshed = now + 1, .refreshes = 1, .expected_atime = now + 1},
        {.name = "cached future timestamp", .cached = now + NSEC_PER_SEC, .expected_atime = now + NSEC_PER_SEC},
        {.name = "getattr failure", .refresh_error = -EIO, .refreshes = 1, .expected_atime = old},
        {.name = "set-time failure", .set_error = -EIO, .refreshes = 1, .writes = 1, .expected_atime = old},
    };
    struct vfsmount mount = {.mnt_flags = MNT_READONLY | MNT_NOATIME};
    unsigned i;

    test_sb.s_flags = SB_RDONLY;
    test_sb.s_fs_info = &test_sb;
    enode->inode.i_sb = &test_sb;
    enode->inode.i_ino = 0x4000000000000123UL;
    for (i = 0; i < ARRAY_SIZE(cases); i++) {
        struct file filp = {};
        struct ternfs_inode_file expected;
        struct timespec64 atime;
        int err;

        prepare_file_state(enode, cases[i].transient ?
            TERNFS_FILE_STATUS_WRITING : TERNFS_FILE_STATUS_READING);
        expected = enode->file;
        filp.f_mode = cases[i].transient ? FMODE_WRITE : FMODE_READ;
        filp.f_flags = cases[i].flags;
        filp.f_path.mnt = &mount;
        ternfs_atime_on_open = !cases[i].disabled;
        ternfs_atime_update_interval_sec = cases[i].unthrottled ? 0 : 60;
        now_ns = now;
        refreshed_atime_ns = cases[i].refreshed ? cases[i].refreshed : old;
        inode_set_atime_to_ts(&enode->inode, ns_to_timespec64(cases[i].cached ? cases[i].cached : old));
        inode_set_mtime(&enode->inode, 50, 123);
        enode->getattr_expiry = 999;
        refresh_err = cases[i].refresh_error;
        set_time_err = cases[i].set_error;
        refresh_calls = set_time_calls = trace_calls = traces_while_locked = 0;
        bad_request = false;

        err = ternfs_file_open(&enode->inode, &filp);
        atime = inode_get_atime(&enode->inode);
        if (err || refresh_calls != cases[i].refreshes ||
            set_time_calls != cases[i].writes || bad_request ||
            timespec64_to_ns(&atime) != cases[i].expected_atime ||
            inode_get_mtime_sec(&enode->inode) != 50 ||
            inode_get_mtime_nsec(&enode->inode) != 123 ||
            enode->getattr_expiry != (set_time_calls && !set_time_err ? 0 : 999) ||
            memcmp(&enode->file, &expected, sizeof(expected)) ||
            trace_calls != 2 || traces_while_locked != 0 ||
            inode_is_locked(&enode->inode)) {
            pr_err("ternfs-open-test: atime case %s failed (err=%d refresh=%u write=%u atime=%lld)\n",
                cases[i].name, err, refresh_calls, set_time_calls, timespec64_to_ns(&atime));
            return -EINVAL;
        }
    }

    // Several opens inside the throttle window must leave cached atime
    // alone, so an open at the deadline eventually persists it.
    {
        struct file filp = {.f_mode = FMODE_READ, .f_path.mnt = &mount};
        struct timespec64 atime;
        ternfs_atime_on_open = 1;
        ternfs_atime_update_interval_sec = 60;
        refreshed_atime_ns = old;
        inode_set_atime_to_ts(&enode->inode, ns_to_timespec64(old));
        refresh_err = set_time_err = 0;
        refresh_calls = set_time_calls = 0;
        for (i = 1; i <= 6; i++) {
            now_ns = old + i * 10ull * NSEC_PER_SEC;
            if (ternfs_file_open(&enode->inode, &filp)) { return -EINVAL; }
        }
        atime = inode_get_atime(&enode->inode);
        if (refresh_calls != 1 || set_time_calls != 1 || timespec64_to_ns(&atime) != now_ns) {
            pr_err("ternfs-open-test: repeated throttled opens postponed persistence\n");
            return -EINVAL;
        }
    }
    ternfs_atime_on_open = 0;
    return 0;
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
    err = run_atime_cases(enode);
    if (err) { return err; }

    pr_info(
        "ternfs-open-test: PASS (states, modes, repeated reopen, ownership, locks, atime on RO mounts, throttle, failures)\n"
    );
    return 0;
}

static void __exit open_test_exit(void) {}

module_init(open_test_init);
module_exit(open_test_exit);
