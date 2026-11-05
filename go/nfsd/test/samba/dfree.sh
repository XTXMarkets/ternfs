#!/bin/sh
# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later
#
# Samba "dfree command": total and available 1 KiB blocks for $1.
df -k --output=size,avail "$1" | tail -1
