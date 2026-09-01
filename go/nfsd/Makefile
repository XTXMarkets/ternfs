# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

LIBNFS_REPOSITORY := https://github.com/sahlberg/libnfs.git
LIBNFS_VERSION := 5.0.2
LIBNFS_REVISION := libnfs-$(LIBNFS_VERSION)

DEPS_DIR := $(CURDIR)/.deps
LIBNFS_SOURCE := $(DEPS_DIR)/libnfs-$(LIBNFS_VERSION)
LIBNFS_BUILD := $(DEPS_DIR)/libnfs-build-$(LIBNFS_VERSION)
LIBNFS_INSTALL := $(DEPS_DIR)/libnfs-install
LIBNFS_FETCHED := $(LIBNFS_SOURCE)/.ternfs-fetched
LIBNFS_BUILT := $(LIBNFS_INSTALL)/.ternfs-built-$(LIBNFS_VERSION)

GO_TEST_FLAGS ?=

.DEFAULT_GOAL := help

.PHONY: help test test-cluster fetch-libnfs build-libnfs test-libnfs clean-libnfs

help:
	@printf '%s\n' \
		'Available targets:' \
		'  test             Run the default Go tests' \
		'  test-cluster     Run the TernFS-backed tests' \
		'  fetch-libnfs     Fetch the pinned libnfs source' \
		'  build-libnfs     Build and install libnfs under .deps' \
		'  test-libnfs      Build libnfs and run its client tests verbosely' \
		'  clean-libnfs     Remove downloaded and built libnfs files' \
		'' \
		'Set GO_TEST_FLAGS to pass additional arguments to Go test targets.'

test:
	go test $(GO_TEST_FLAGS) .

test-cluster:
	go test -tags ternnfs $(GO_TEST_FLAGS) .

fetch-libnfs: $(LIBNFS_FETCHED)

$(LIBNFS_FETCHED):
	mkdir -p "$(DEPS_DIR)"
	git clone --branch "$(LIBNFS_REVISION)" --depth 1 \
		"$(LIBNFS_REPOSITORY)" "$(LIBNFS_SOURCE)"
	touch "$@"

build-libnfs: $(LIBNFS_BUILT)

$(LIBNFS_BUILT): $(LIBNFS_FETCHED)
	cmake -S "$(LIBNFS_SOURCE)" -B "$(LIBNFS_BUILD)" \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX="$(LIBNFS_INSTALL)" \
		-DBUILD_SHARED_LIBS=ON
	cmake --build "$(LIBNFS_BUILD)" --parallel
	cmake --install "$(LIBNFS_BUILD)"
	touch "$@"

test-libnfs: $(LIBNFS_BUILT)
	go test -count=1 -v -tags libnfs -run '^TestLibnfs_' $(GO_TEST_FLAGS) .

clean-libnfs:
	rm -rf "$(DEPS_DIR)"
