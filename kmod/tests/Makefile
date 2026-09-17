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
obj-m += ternfs-transient-test.o
ternfs-transient-test-y := transient_test.o
obj-m += ternfs-symlink-create-test.o
ternfs-symlink-create-test-y := symlink_create_test.o
obj-m += ternfs-fetch-state-test.o
ternfs-fetch-state-test-y := fetch_state_test.o
ccflags-y += -I$(src)/.. -Wno-declaration-after-statement
