// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_FANALYZER_H
#define _TERNFS_FANALYZER_H

#ifndef TERNFS_FANALYZER
#error "fanalyzer.h is only for the GCC -fanalyzer build"
#endif

#if !defined(__GNUC__) || defined(__clang__) || __GNUC__ < 11
#error "TernFS allocator analysis requires GCC 11 or newer"
#endif

#include <linux/slab.h>
#include <linux/vmalloc.h>

void ternfs_fanalyzer_escape(const void *ptr);

#define TERNFS_MALLOC_DEALLOC(_deallocator, _pointer_arg) \
    __attribute__(( \
        malloc(_deallocator, _pointer_arg), \
        malloc(ternfs_fanalyzer_escape, 1) \
    ))

/*
 * The kernel's __malloc annotation has no deallocator, so GCC's analyzer
 * does not otherwise know which calls release these allocations.
 */
void *__kmalloc_noprof(size_t size, gfp_t flags)
    TERNFS_MALLOC_DEALLOC(kfree, 1);
void *__kmalloc_node_noprof(DECL_BUCKET_PARAMS(size, b), gfp_t flags, int node)
    TERNFS_MALLOC_DEALLOC(kfree, 1);
void *__kmalloc_cache_noprof(struct kmem_cache *s, gfp_t flags, size_t size)
    TERNFS_MALLOC_DEALLOC(kfree, 1);
void *__kmalloc_cache_node_noprof(
    struct kmem_cache *s, gfp_t gfpflags, int node, size_t size
) TERNFS_MALLOC_DEALLOC(kfree, 1);
void *__kmalloc_large_noprof(size_t size, gfp_t flags)
    TERNFS_MALLOC_DEALLOC(kfree, 1);
void *__kmalloc_large_node_noprof(size_t size, gfp_t flags, int node)
    TERNFS_MALLOC_DEALLOC(kfree, 1);

void *kmem_cache_alloc_noprof(struct kmem_cache *cachep, gfp_t flags)
    TERNFS_MALLOC_DEALLOC(kmem_cache_free, 2);

void *vmalloc_noprof(unsigned long size)
    TERNFS_MALLOC_DEALLOC(vfree, 1);
void *vzalloc_noprof(unsigned long size)
    TERNFS_MALLOC_DEALLOC(vfree, 1);

struct kmem_cache *__kmem_cache_create_args(
    const char *name,
    unsigned int object_size,
    struct kmem_cache_args *args,
    slab_flags_t flags
) TERNFS_MALLOC_DEALLOC(kmem_cache_destroy, 1);

#undef TERNFS_MALLOC_DEALLOC

#endif
