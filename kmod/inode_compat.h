// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_INODE_COMPAT_H
#define _TERNFS_INODE_COMPAT_H
#include <linux/version.h>
#include <linux/fs.h>

// Compatibility for user_namespace struct
#if LINUX_VERSION_CODE < KERNEL_VERSION(5, 12, 0)
  #define COMPAT_FUNC_UNS(name, ...) name(__VA_ARGS__);
  #define COMPAT_FUNC_UNS_IMP(name, ...) name(__VA_ARGS__)
  #define COMPAT_FUNC_UNS_CALL(name, ...) name(__VA_ARGS__);
#elif LINUX_VERSION_CODE < KERNEL_VERSION(6,3,0)
  #define COMPAT_FUNC_UNS(name, ...) name(struct user_namespace*, __VA_ARGS__);
  #define COMPAT_FUNC_UNS_IMP(name, ...) name(struct user_namespace* uns, __VA_ARGS__)
  #define COMPAT_FUNC_UNS_CALL(name, ...) name(uns, __VA_ARGS__);
# else
  #define COMPAT_FUNC_UNS(name, ...) name(struct mnt_idmap *, __VA_ARGS__);
  #define COMPAT_FUNC_UNS_IMP(name, ...) name(struct mnt_idmap * uns, __VA_ARGS__)
  #define COMPAT_FUNC_UNS_CALL(name, ...) name(uns, __VA_ARGS__);
#endif


#if LINUX_VERSION_CODE < KERNEL_VERSION(6,6,0)

static inline time64_t inode_get_ctime_sec(const struct inode *inode)
{
	return inode->i_ctime.tv_sec;
}

static inline long inode_get_ctime_nsec(const struct inode *inode)
{
	return inode->i_ctime.tv_nsec;
}

static inline struct timespec64 inode_get_ctime(const struct inode *inode)
{
	return inode->i_ctime;
}

static inline struct timespec64 inode_set_ctime_to_ts(struct inode *inode,
						      struct timespec64 ts)
{
	inode->i_ctime = ts;
	return ts;
}

static inline struct timespec64 inode_set_ctime(struct inode *inode,
						time64_t sec, long nsec)
{
	struct timespec64 ts = { .tv_sec  = sec,
				 .tv_nsec = nsec };

	return inode_set_ctime_to_ts(inode, ts);
}

#endif

#if LINUX_VERSION_CODE < KERNEL_VERSION(6, 7, 0)

static inline time64_t inode_get_atime_sec(const struct inode *inode)
{
	return inode->i_atime.tv_sec;
}

static inline long inode_get_atime_nsec(const struct inode *inode)
{
	return inode->i_atime.tv_nsec;
}

static inline struct timespec64 inode_get_atime(const struct inode *inode)
{
	return inode->i_atime;
}

static inline struct timespec64 inode_set_atime_to_ts(struct inode *inode,
						      struct timespec64 ts)
{
	inode->i_atime = ts;
	return ts;
}

static inline struct timespec64 inode_set_atime(struct inode *inode,
						time64_t sec, long nsec)
{
	struct timespec64 ts = { .tv_sec  = sec,
				 .tv_nsec = nsec };
	return inode_set_atime_to_ts(inode, ts);
}

static inline time64_t inode_get_mtime_sec(const struct inode *inode)
{
	return inode->i_mtime.tv_sec;
}

static inline long inode_get_mtime_nsec(const struct inode *inode)
{
	return inode->i_mtime.tv_nsec;
}

static inline struct timespec64 inode_get_mtime(const struct inode *inode)
{
	return inode->i_mtime;
}

static inline struct timespec64 inode_set_mtime_to_ts(struct inode *inode,
						      struct timespec64 ts)
{
	inode->i_mtime = ts;
	return ts;
}

static inline struct timespec64 inode_set_mtime(struct inode *inode,
						time64_t sec, long nsec)
{
	struct timespec64 ts = { .tv_sec  = sec,
				 .tv_nsec = nsec };
	return inode_set_mtime_to_ts(inode, ts);
}

#endif

#endif /* _TERNFS_INODE_COMPAT_H */
