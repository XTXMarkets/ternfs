# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

LIBNFS_REPOSITORY := https://github.com/sahlberg/libnfs.git
LIBNFS_VERSION := 7.0.2
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
TEST_ARGS ?=
PYNFS_PYTHON ?= python3
PYNFS_TESTS ?= all noblock nochar nofifo nosocket nogss noacl nomode000
PYNFS_ARGS ?=
PYNFS_SKIP_FILE ?= $(CURDIR)/pynfs/pynfs_unsupported.txt
PYNFS_OUTPUT ?= $(CURDIR)/pynfs.out
PYNFS_TIMEOUT ?= 1h
LIBNFS_TIMEOUT ?= 15m
FUNCTIONAL_TEST_ROOT ?=

.DEFAULT_GOAL := help

.PHONY: help fetch-libnfs build-libnfs \
	test-libnfs test-libnfs-cluster clean-libnfs fetch-pynfs build-pynfs \
	test-pynfs test-pynfs-cluster clean-pynfs test-functional

help:
	@printf '%s\n' \
		'Client test harnesses (the package tests are in ../Makefile):' \
		'  fetch-libnfs     Fetch the pinned libnfs source' \
		'  build-libnfs     Build and install libnfs under .deps' \
		'  test-libnfs      Run libnfs tests against an existing backend' \
		'  test-libnfs-cluster  Run libnfs tests with a temporary ternrun cluster' \
		'  clean-libnfs     Remove downloaded and built libnfs files' \
		'  fetch-pynfs      Fetch the pinned pynfs source under .deps' \
		'  build-pynfs      Generate the pynfs NFSv4.0 modules' \
		'  test-pynfs       Run pynfs against an existing backend' \
		'  test-pynfs-cluster  Run pynfs with a temporary ternrun cluster' \
		'  clean-pynfs      Remove downloaded and built pynfs files' \
		'  test-functional  Run common workflows on an existing NFS mount' \
		'' \
		'Targets ending in -cluster start their own backend.' \
		'Other client targets require TEST_ARGS="-registry HOST:PORT".' \
		'Use TEST_ARGS="-binaries-dir DIR" with -cluster targets for prebuilt backend binaries.' \
		'Set GO_TEST_FLAGS to pass additional arguments to Go test invocations.' \
		'Set PYNFS_TESTS, PYNFS_ARGS, PYNFS_SKIP_FILE, PYNFS_OUTPUT and PYNFS_TIMEOUT to configure pynfs.' \
		'Set FUNCTIONAL_TEST_ROOT to an existing mounted test directory for test-functional.'

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

test-libnfs-cluster test-pynfs-cluster: CLUSTER_ARG = -cluster

test-libnfs test-libnfs-cluster: $(LIBNFS_BUILT)
	go test -count=1 -v -timeout "$(LIBNFS_TIMEOUT)" -tags libnfs \
		-run '^TestLibnfs$$' $(GO_TEST_FLAGS) ./libnfs $(TEST_ARGS) $(CLUSTER_ARG)

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

test-pynfs test-pynfs-cluster: $(PYNFS_BUILT)
	PYNFS_SOURCE="$(PYNFS_SOURCE)" \
	PYNFS_PYTHON="$(PYNFS_PYTHON)" \
	PYNFS_TESTS="$(PYNFS_TESTS)" \
	PYNFS_ARGS="$(PYNFS_ARGS)" \
	PYNFS_SKIP_FILE="$(PYNFS_SKIP_FILE)" \
	go run ./pynfs -timeout "$(PYNFS_TIMEOUT)" $(TEST_ARGS) $(CLUSTER_ARG) \
		2>&1 | tee "$(PYNFS_OUTPUT)"

clean-pynfs:
	rm -rf "$(PYNFS_SOURCE)"

test-functional:
	@test -n "$(FUNCTIONAL_TEST_ROOT)" || { \
		echo "error: set FUNCTIONAL_TEST_ROOT to an existing mounted test directory" >&2; \
		exit 1; \
	}
	./functional/run-all.sh "$(FUNCTIONAL_TEST_ROOT)"
