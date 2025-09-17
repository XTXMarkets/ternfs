// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_FILE_H
#define _TERNFS_FILE_H

#include "inode.h"

extern unsigned ternfs_atime_update_interval_sec;
extern unsigned ternfs_max_write_span_attempts;
extern unsigned ternfs_file_io_timeout_sec;
extern unsigned ternfs_file_io_retry_refresh_span_interval_sec;

extern int ternfs_file_getattr_refresh_time_jiffies; // this is only relevant for mtime/atime updates

ssize_t ternfs_file_write(struct ternfs_inode* enode, int flags, loff_t* ppos, struct iov_iter* from);
int ternfs_file_flush(struct ternfs_inode* enode, struct dentry* dentry);

// Also used in ternfs_do_ftruncate to fill the end of the file.
ssize_t ternfs_file_write_internal(struct ternfs_inode* enode, int flags, loff_t* ppos, struct iov_iter* from, size_t count);

void ternfs_link_destructor(void*);
char* ternfs_read_link(struct ternfs_inode* enode);

extern const struct file_operations ternfs_file_operations;
extern const struct address_space_operations ternfs_mmap_operations;

int __init ternfs_file_init(void);
void __cold ternfs_file_exit(void);

#endif
