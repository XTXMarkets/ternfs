# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: GPL-2.0-or-later

KDIR ?= /lib/modules/$(shell uname -r)/build
LLVM ?= 0

MINOR_VERSION ?= 0
# Only populate GIT_REVISION when building inside a git checkout.
# DKMS builds run on the target host from an extracted source tree with
# revision.c already shipped — git will not be available there.
GIT_REVISION := $(shell git describe --always 2>/dev/null)
FULL_MINOR_VERSION := "$(MINOR_VERSION)-$(GIT_REVISION)"

obj-m += ternfs-client.o

ternfs-client-objs += \
	dir.o \
	err.o \
	export.o \
	debugfs.o \
	file.o \
	inode.o \
	kmod.o \
	metadata.o \
	dentry.o \
	net.o \
	super.o \
	sysctl.o \
	sysfs.o \
	trace.o \
	registry.o \
	block.o \
	file.o \
	rs.o \
	crc.o \
	crc_fpu.o \
	span.o \
	bincode.o \
	revision.o \
	policy.o

HAVE_INODE_ATIME_ACCESSORS := $(shell grep -q inode_get_atime_sec \
    $(srctree)/include/linux/fs.h 2>/dev/null && echo 1)

EXTRA_CFLAGS = -I$(src) -g -DDEBUG -fdiagnostics-color=always -Wno-declaration-after-statement
ifeq ($(HAVE_INODE_ATIME_ACCESSORS),1)
  EXTRA_CFLAGS += -DHAVE_INODE_ATIME_ACCESSORS
endif

export CF = -Wbitwise -Wcontext -Wcast_truncate -Wsparse-all -Wno-shadow -Wnopointer-arith -Wnosizeof-bool -DCONFIG_SPARSE_RCU_POINTER

ifneq ($(strip $(GIT_REVISION)),)
# In a git checkout: regenerate revision.c on every build so the embedded
# revision always matches HEAD.
.PHONY: revision.c
revision.c:
	printf "#include \"sysfs.h\"\nconst char* ternfs_revision = \"$(GIT_REVISION)\";\n" > revision.c
else
# Outside a git checkout (e.g. DKMS): revision.c must already be shipped in
# the source tree. Never overwrite it, and fail loudly if it's missing.
revision.c:
	@test -f $@ || { echo "revision.c missing and not a git checkout" >&2; exit 1; }
endif

EXTRA_FILES := ../cpp/crc32c/crc32c_body.c ../cpp/rs/gf_tables.c ../cpp/rs/rs_core.c

extra-files: $(EXTRA_FILES)
	$(foreach file,$(EXTRA_FILES),cp $(file) .;)

LOCAL_FILES := $(notdir $(EXTRA_FILES))
extra-files-clean:
	rm -f $(LOCAL_FILES) revision.c *.o

# C=1 builds with sparse
ternfs-client-tests: revision.c extra-files
	$(MAKE) -C $(KDIR) M=$(CURDIR) C=1 modules

ternfs-client-clean:
	$(MAKE) -C $(KDIR) M=$(CURDIR) clean
	rm -f *.o *.ko


bincode_tests: bincode_tests.c bincodegen.h bincode.h
	gcc -Wall -g -O2 -fsanitize=undefined,address -fno-sanitize-recover=all bincode_tests.c -o bincode_tests

bincode_tests-clean:
	rm -f bincode_tests rs_tests ubuntu.img test-out trace vm-out dmesg
	rm -rf tmp

dkms-version:
	cp debian_tmpl/* debian/. && find debian -maxdepth 1 -type f -exec sed -i "s/##VERSION##/$(FULL_MINOR_VERSION)/g" {} \;


deb-package: revision.c extra-files dkms-version
	dpkg-buildpackage -us -uc

# Targets to build kmod from dkms package,
# revision.c and extra files are already included
ternfs-client:
	$(MAKE) -C $(KDIR) M=$(CURDIR) modules

# Targets to build/clean the kmod with local linux
# We don't want sparse as a dependency.
ternfs-client-local: revision.c extra-files
	$(MAKE) -C $(KDIR) M=$(CURDIR) modules

clean: ternfs-client-clean bincode_tests-clean
