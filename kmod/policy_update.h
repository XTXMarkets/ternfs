// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_POLICY_UPDATE_H
#define _TERNFS_POLICY_UPDATE_H

struct ternfs_inode;
struct ternfs_policy_body;

int ternfs_update_dir_policies(
    struct ternfs_inode* enode,
    struct ternfs_policy_body* block,
    struct ternfs_policy_body* span,
    struct ternfs_policy_body* stripe,
    struct ternfs_policy_body* snapshot
);

#endif
