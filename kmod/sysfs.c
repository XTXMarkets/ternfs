// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "sysfs.h"

#include <linux/kobject.h>
#include <linux/fs.h>

#include "dentry.h"
#include "span.h"
#include "log.h"

static struct kset* ternfs_kset;

struct ternfs_sysfs_counter {
    struct kobj_attribute kobj_attr;
    u64 __percpu * ptr;
};

static ssize_t ternfs_sysfs_counter_show(struct kobject* kobj, struct kobj_attribute* attr, char* buf) {
    struct ternfs_sysfs_counter* ettr = container_of(attr, struct ternfs_sysfs_counter, kobj_attr);
    u64 val = ternfs_counter_get(ettr->ptr);
    return snprintf(buf, PAGE_SIZE, "%llu\n", val);
}

static ssize_t ternfs_sysfs_counter_store(struct kobject *kobj, struct kobj_attribute *a, const char *buf, size_t count) {
    return -EPERM;
}

struct ternfs_sysfs_atomic64 {
    struct kobj_attribute kobj_attr;
    atomic64_t* v;
};

static umode_t ternfs_stat_visible(struct kobject *kobj, struct attribute *attr, int unused) {
    return attr->mode;
}

#define __INIT_KOBJ_ATTR(_name, _mode, _show, _store)			\
{									\
	.attr	= { .name = __stringify(_name), .mode = _mode },	\
	.show	= _show,						\
	.store	= _store,						\
}

#define TERNFS_SYSFS_COUNTER(_name) \
    static struct ternfs_sysfs_counter ternfs_sysfs_counter_##_name = { \
        .kobj_attr = __INIT_KOBJ_ATTR(_name, S_IRUGO, ternfs_sysfs_counter_show, ternfs_sysfs_counter_store), \
        .ptr = &ternfs_stat_##_name, \
    }

#define TERNFS_SYSFS_COUNTER_PTR(_name) (&ternfs_sysfs_counter_##_name.kobj_attr.attr)

TERNFS_SYSFS_COUNTER(dir_revalidations);

static struct attribute* ternfs_stat_attrs[] = {
    TERNFS_SYSFS_COUNTER_PTR(dir_revalidations),
    NULL
};

static const struct attribute_group ternfs_stat_attr_group = {
    .name = "stats",
    .is_visible = ternfs_stat_visible,
    .attrs = ternfs_stat_attrs,
};

static ssize_t ternfs_revision_show(struct kobject* kobj, struct kobj_attribute* attr, char* buf) {
    return sprintf(buf, "%s\n", ternfs_revision);
}

static struct kobj_attribute ternfs_revision_attr = __ATTR(revision, 0444, ternfs_revision_show, NULL);

int __init ternfs_sysfs_init(void) {
    int err;

    ternfs_kset = kset_create_and_add("eggsfs", NULL, fs_kobj);
    if (!ternfs_kset) { return -ENOMEM; }

    err = sysfs_create_group(&ternfs_kset->kobj, &ternfs_stat_attr_group);
    if (err) { goto out_unreg; }

    err = sysfs_create_file(&ternfs_kset->kobj, &ternfs_revision_attr.attr);
    if (err) { goto out_unreg; }

    return 0;

out_unreg:
    kset_unregister(ternfs_kset);
    return err;
}

void __cold ternfs_sysfs_exit(void) {
    ternfs_debug("sysfs exit");
    kset_unregister(ternfs_kset);
}

