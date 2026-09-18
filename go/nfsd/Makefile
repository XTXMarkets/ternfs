# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

GO_TEST_FLAGS ?=
SS ?= ss
TERNFS_REGISTRY_HOST ?= 127.0.0.1
TERNFS_REGISTRY_PORT ?= 55556

.DEFAULT_GOAL := help

.PHONY: help test check-cluster-port test-cluster

help:
	@printf '%s\n' \
		'Available targets:' \
		'  test             Run the Go unit tests' \
		'  test-cluster     Run the Go tests which need a TernFS cluster' \
		'' \
		'Set GO_TEST_FLAGS to pass additional arguments to go test.' \
		'The client test harnesses (libnfs, pynfs, kernel client, workflows) are in test/Makefile.'

test:
	go test $(GO_TEST_FLAGS) .

check-cluster-port:
	@command -v "$(SS)" >/dev/null 2>&1 || { \
		echo "error: $(SS) is required to check for an existing TernFS test cluster" >&2; \
		exit 1; \
	}
	@if [ -n "$$("$(SS)" -H -lun "sport = :$(TERNFS_REGISTRY_PORT)")" ]; then \
		echo "error: UDP port $(TERNFS_REGISTRY_HOST):$(TERNFS_REGISTRY_PORT) is already in use; stop the existing TernFS test cluster" >&2; \
		exit 1; \
	fi

test-cluster: check-cluster-port
	go test -tags ternnfs $(GO_TEST_FLAGS) .
