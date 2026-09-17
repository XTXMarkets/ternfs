# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

obj-m += ternfs-policy-test.o
ternfs-policy-test-y := policy_test.o
obj-m += ternfs-write-test.o
ternfs-write-test-y := write_test.o
obj-m += ternfs-inline-read-test.o
ternfs-inline-read-test-y := inline_read_test.o
obj-m += ternfs-symlink-test.o
ternfs-symlink-test-y := symlink_test.o
obj-m += ternfs-open-test.o
ternfs-open-test-y := open_test.o
ccflags-y += -I$(src)/.. -Wno-declaration-after-statement
