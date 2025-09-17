// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#ifndef _TERNFS_INTRSHIMS_H
#define _TERNFS_INTRSHIMS_H

#ifdef __KERNEL__
#include <linux/kernel.h>
#endif

#define _mm_crc32_u64(C, D) __builtin_ia32_crc32di(C, D)
#define _mm_crc32_u32(C, D) __builtin_ia32_crc32si(C, D)
#define _mm_crc32_u8(C, D) __builtin_ia32_crc32qi(C, D)

typedef long long __m128i __attribute__((vector_size(16)));
typedef long long __m128i_u __attribute__((vector_size(16), __aligned__(1)));

typedef char __v16qi __attribute__((vector_size(16)));
typedef short __v8hi __attribute__((vector_size(16)));
typedef int __v4si __attribute__((vector_size(16)));
typedef long long __v2di __attribute__((vector_size(16)));

#define _mm_setr_epi32(i0, i1, i2, i3) ((__m128i)(__v4si){i0,i1,i2,i3})
#define _mm_clmulepi64_si128(X, Y, I) ((__m128i)__builtin_ia32_pclmulqdq128((__v2di)(__m128i)(X), (__v2di)(__m128i)(Y), (char)(I)))
#define _mm_xor_si128(a, b) ((a) ^ (b))
#define _mm_cvtsi32_si128(a) ((__m128i)(__v4si){a,0,0,0})
#define _mm_cvtsi128_si64(a) ((a)[0])
#define _mm_extract_epi64(X, N) ((long long)__builtin_ia32_vec_ext_v2di((__v2di)(__m128i)(X), (int)(N)))
#define _mm_loadu_si128(__p) ({ \
        struct __loadu_si128 { \
            __m128i_u __v; \
        } __attribute__((__packed__, __may_alias__)); \
        ((const struct __loadu_si128 *)__p)->__v; \
    })

typedef long long __m256i __attribute__((vector_size(32)));
typedef long long __m256i_u __attribute__((vector_size(32), __aligned__(1)));

typedef char __v32qi __attribute__((vector_size(32)));
typedef short __v16hi __attribute__((vector_size(32)));
typedef int __v8si __attribute__((vector_size(32)));
typedef long long __v4di __attribute__((vector_size(32)));

#define _mm256_and_si256(a, b) ((a) & (b))
#define _mm256_xor_si256(a, b) ((a) ^ (b))
#define _mm256_permute2x128_si256(a, b, i) ((__m256i)__builtin_ia32_permti256((a), (b), (i)))
#define _mm256_srli_epi16(a, b) ((__m256i)__builtin_ia32_psrlwi256((__v16hi)(a), (b)))
#define _mm256_shuffle_epi8(a, b) ((__m256i)__builtin_ia32_pshufb256((__v32qi)(a), (__v32qi)(b)))
#define _mm256_setzero_si256() ((__m256i)(__v4di){0,0,0,0})
#define _mm256_loadu_si256(__p) ({ \
        struct __loadu_si256 { \
            __m256i_u __v; \
        } __attribute__((__packed__, __may_alias__)); \
        ((const struct __loadu_si256*)__p)->__v; \
    })
#define _mm256_storeu_si256(__p, __a) ({ \
        struct __storeu_si256 { \
            __m256i_u __v; \
        } __attribute__((__packed__, __may_alias__)); \
        ((struct __storeu_si256*)__p)->__v = __a; \
    })
#define _mm256_gf2p8mul_epi8(a, b) ((__m256i)__builtin_ia32_vgf2p8mulb_v32qi((__v32qi)(a), (__v32qi)(b)))

// gcc doesn't emit popcnt somehow <https://gcc.gnu.org/bugzilla/show_bug.cgi?id=105253>
#define __builtin_popcountll(__v) ({ \
        u64 __c; \
        u64 __v64 = (u64)__v; \
        __asm__("popcntq %1, %0" : "=r"(__c) : "r"(__v64) : "cc"); \
        __c; \
    })

#endif
