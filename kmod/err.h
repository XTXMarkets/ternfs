// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_ERR_H
#define _TERNFS_ERR_H

#include <linux/errno.h>

#include "bincode.h"

bool ternfs_unexpected_error(int err);
int ternfs_error_to_linux(int err);

#endif

