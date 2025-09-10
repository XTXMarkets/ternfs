// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

// To be used in conjunction with ternfuse to make sure that only
// files which have been close()d explicitly get linked to the filesystem.
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

// This variable will be set by the userspace loader
volatile const uint32_t target_dev = 0;

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    // We don't really have strong guarantees on the interval between a user calling
    // close() and flush being invoked at the VFS layer. This is essentially a buffer
    // that needs to be big enough to remember the file long enough in that interval.
    // Since forgetting a file is pretty bad, make this big.
    __uint(max_entries, 1<<20);
    __type(key, uint64_t);
    __type(value, uint64_t);
} closed_inodes_map SEC(".maps");

SEC("tp/syscalls/sys_enter_close")
int handle_close(struct trace_event_raw_sys_enter* ctx) {
    // Exit early if no target device is set
    if (target_dev == 0) {
        return 0;
    }

    int fd = (int)ctx->args[0];

    struct task_struct* task = (struct task_struct *)bpf_get_current_task();
    if (!task) {
        return 0;
    }

    struct files_struct* files = BPF_CORE_READ(task, files);
    if (!files) {
        return 0;
    }

    struct fdtable* fdt = BPF_CORE_READ(files, fdt);
    if (!fdt) {
        return 0;
    }

    if (fd >= BPF_CORE_READ(fdt, max_fds)) {
        return 0;
    }

    struct file** fd_array = BPF_CORE_READ(fdt, fd);
    if (!fd_array) {
        return 0;
    }
    
    struct file* file;
    bpf_probe_read_kernel(&file, sizeof(file), &fd_array[fd]);
    if (!file) {
        return 0;
    }

    struct inode* inode = BPF_CORE_READ(file, f_inode);
    if (!inode) {
        return 0;
    }

    struct super_block* sb = BPF_CORE_READ(inode, i_sb);
    if (!sb) {
        return 0;
    }

    // Filter by the device ID provided by userspace
    dev_t dev = BPF_CORE_READ(sb, s_dev);
    if (dev != target_dev) {
        return 0;
    }

    uint64_t ino = BPF_CORE_READ(inode, i_ino);
    uint64_t* count = bpf_map_lookup_elem(&closed_inodes_map, &ino);
    if (count) {
        __sync_fetch_and_add(count, 1);
    } else {
        uint64_t init_val = 1;
        bpf_map_update_elem(&closed_inodes_map, &ino, &init_val, BPF_ANY);
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
