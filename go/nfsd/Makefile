# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

LIBNFS_REPOSITORY := https://github.com/sahlberg/libnfs.git
LIBNFS_VERSION := 5.0.2
LIBNFS_REVISION := libnfs-$(LIBNFS_VERSION)

DEPS_DIR := $(CURDIR)/.deps
LIBNFS_SOURCE := $(DEPS_DIR)/libnfs-$(LIBNFS_VERSION)
LIBNFS_BUILD := $(DEPS_DIR)/libnfs-build-$(LIBNFS_VERSION)
LIBNFS_INSTALL := $(DEPS_DIR)/libnfs-install
LIBNFS_FETCHED := $(LIBNFS_SOURCE)/.ternfs-fetched
LIBNFS_BUILT := $(LIBNFS_INSTALL)/.ternfs-built-$(LIBNFS_VERSION)

PYNFS_REPOSITORY := https://github.com/linux-nfs/pynfs.git
PYNFS_REVISION := pynfs-0.5
PYNFS_SOURCE := $(DEPS_DIR)/pynfs
PYNFS_FETCHED := $(PYNFS_SOURCE)/.ternfs-fetched
PYNFS_BUILT := $(PYNFS_SOURCE)/.ternfs-built

GO_TEST_FLAGS ?=
SS ?= ss
PYNFS_PYTHON ?= python3
PYNFS_TESTS ?= all
PYNFS_ARGS ?=
PYNFS_OUTPUT ?= $(CURDIR)/pynfs.out
PYNFS_TIMEOUT ?= 1h
TERNFS_REGISTRY_HOST ?= 127.0.0.1
TERNFS_REGISTRY_PORT ?= 55556

.DEFAULT_GOAL := help

.PHONY: help check-cluster-port test test-cluster fetch-libnfs build-libnfs \
	test-libnfs clean-libnfs fetch-pynfs build-pynfs test-pynfs-cluster clean-pynfs

help:
	@printf '%s\n' \
		'Available targets:' \
		'  test             Run the default Go tests' \
		'  test-cluster     Run the TernFS-backed tests' \
		'  fetch-libnfs     Fetch the pinned libnfs source' \
		'  build-libnfs     Build and install libnfs under .deps' \
		'  test-libnfs      Build libnfs and run its client tests verbosely' \
		'  clean-libnfs     Remove downloaded and built libnfs files' \
		'  fetch-pynfs      Fetch the pinned pynfs source under .deps' \
		'  build-pynfs      Generate the pynfs NFSv4.0 modules' \
		'  test-pynfs-cluster  Run pynfs against a temporary TernFS cluster' \
		'  clean-pynfs      Remove downloaded and built pynfs files' \
		'' \
		'Test targets ending in -cluster start a temporary TernFS cluster.' \
		'Set GO_TEST_FLAGS to pass additional arguments to Go test targets.' \
		'Set PYNFS_TESTS, PYNFS_ARGS, PYNFS_OUTPUT and PYNFS_TIMEOUT to configure pynfs.'

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
	rm -rf "$(LIBNFS_SOURCE)" "$(LIBNFS_BUILD)" "$(LIBNFS_INSTALL)"

fetch-pynfs: $(PYNFS_FETCHED)

$(PYNFS_FETCHED):
	mkdir -p "$(DEPS_DIR)"
	git clone --branch "$(PYNFS_REVISION)" --depth 1 \
		"$(PYNFS_REPOSITORY)" "$(PYNFS_SOURCE)"
	touch "$@"

build-pynfs: $(PYNFS_BUILT)

$(PYNFS_BUILT): $(PYNFS_FETCHED)
	cd "$(PYNFS_SOURCE)/nfs4.0" && \
		"$(PYNFS_PYTHON)" setup.py build_ext --inplace
	touch "$@"

test-pynfs-cluster: $(PYNFS_BUILT) check-cluster-port
	PYNFS_SOURCE="$(PYNFS_SOURCE)" \
	PYNFS_PYTHON="$(PYNFS_PYTHON)" \
	PYNFS_TESTS="$(PYNFS_TESTS)" \
	PYNFS_ARGS="$(PYNFS_ARGS)" \
	go test -count=1 -v -timeout "$(PYNFS_TIMEOUT)" \
		-tags 'ternnfs pynfs' -run '^TestPynfs$$' $(GO_TEST_FLAGS) . \
		2>&1 | tee "$(PYNFS_OUTPUT)"

clean-pynfs:
	rm -rf "$(PYNFS_SOURCE)"
