// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <signal.h>
#include <immintrin.h>
#include <array>

#include "rs.h"

#define rs_warn(fmt, ...) fprintf(stderr, "rs: " fmt "\n" __VA_OPT__(,) __VA_ARGS__); raise(SIGABRT);
#define die(...) do { rs_warn(__VA_ARGS__); raise(SIGABRT); } while (0)

// See `valgrind.h`
static uint64_t rs_valgrind_client_request(uint64_t defaultResult, uint64_t reqID, uint64_t arg1, uint64_t arg2, uint64_t arg3, uint64_t arg4, uint64_t arg5) {
    uint64_t args[6] = {reqID, arg1, arg2, arg3, arg4, arg5};
    uint64_t result = defaultResult;
    asm volatile ("rol $3, %%rdi\n\t"
                  "rol $13, %%rdi\n\t"
                  "rol $61, %%rdi\n\t"
                  "rol $51, %%rdi\n\t"
                  "xchg %%rbx, %%rbx\n\t"
                  : "+d" (result)
                  : "a" (args)
                  : "cc", "memory"
                  );
    return result;
}

static bool rs_detect_valgrind() {
    return rs_valgrind_client_request(0, 0x1001, 0, 0, 0, 0, 0);
}

// This will emit vbroadcastb
__attribute__((no_sanitize("integer")))
static inline __m256i broadcast_u8(uint8_t x) {
    return _mm256_set_epi8(
        x, x, x, x, x, x, x, x,
        x, x, x, x, x, x, x, x,
        x, x, x, x, x, x, x, x,
        x, x, x, x, x, x, x, x
    );
}

#include "rs_core.c"

uint8_t rs_parity(struct rs* r) {
    return r->parity;
}

static struct rs* rs_cached[256] = {
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
    nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr, nullptr,
};

bool rs_has_cpu_level(rs_cpu_level level) {
    return rs_has_cpu_level_core(level);
}

static uint8_t rs_chosen_cpu_level = RS_CPU_SCALAR;

__attribute__((constructor))
void rs_detect_cpu_level() {
    if (rs_has_cpu_level(RS_CPU_GFNI)) {
        rs_chosen_cpu_level = RS_CPU_GFNI;
        return;
    }
    if (rs_has_cpu_level(RS_CPU_AVX2)) {
        rs_chosen_cpu_level = RS_CPU_AVX2;
        return;
    }
    fprintf(stderr, "Picking scalar execution for RS library -- this will be slow.\n");
    rs_chosen_cpu_level = RS_CPU_SCALAR;
    return;
}

void rs_set_cpu_level(rs_cpu_level level) {
    __atomic_store_n(&rs_chosen_cpu_level, level, __ATOMIC_RELAXED);
}

rs_cpu_level rs_get_cpu_level() {
    return (rs_cpu_level)__atomic_load_n(&rs_chosen_cpu_level, __ATOMIC_RELAXED);
}

struct rs* rs_get(uint8_t parity) {
    if (rs_data_blocks(parity) < 2 || rs_parity_blocks(parity) < 1) {
        die("bad parity (%d,%d), expected at least 2 data blocks and 1 parity block.\n", rs_data_blocks(parity), rs_parity_blocks(parity));
    }
    struct rs* r = __atomic_load_n(&rs_cached[parity], __ATOMIC_RELAXED);
    if (__builtin_expect(r == nullptr, 0)) {
        r = (struct rs*)malloc(RS_SIZE(rs_data_blocks(parity), rs_parity_blocks(parity)));
        rs_new_core(parity, r);
        if (r == nullptr) {
            die("could not allocate RS data");
        }
        struct rs* expected = nullptr;
        if (!__atomic_compare_exchange_n(&rs_cached[parity], &expected, r, false, __ATOMIC_RELAXED, __ATOMIC_RELAXED)) {
            // somebody else got to it first
            free(r);
            r = __atomic_load_n(&rs_cached[parity], __ATOMIC_RELAXED);
        }
    }
    return r;
}

template<int D, int P> __attribute__((noinline))
static void rs_compute_parity_scalar_tmpl(struct rs* r, uint64_t size, const uint8_t** data, uint8_t** parity) {
    rs_compute_parity_scalar(D, P, r, size, data, parity);
}

template<int D, int P> __attribute__((noinline))
static void rs_compute_parity_avx2_tmpl(struct rs* r, uint64_t size, const uint8_t** data, uint8_t** parity) {
    rs_compute_parity_avx2(D, P, r, size, data, parity);
}

template<int D, int P> __attribute__((noinline))
static void rs_compute_parity_gfni_tmpl(struct rs* r, uint64_t size, const uint8_t** data, uint8_t** parity) {
    rs_compute_parity_gfni(D, P, r, size, data, parity);
}

template<int D, int P>
static void rs_compute_parity_tmpl(struct rs* r, uint64_t size, const uint8_t** data, uint8_t** parity) {
    switch (rs_cpu_level l = rs_get_cpu_level()) {
    case RS_CPU_SCALAR:
        rs_compute_parity_scalar_tmpl<D, P>(r, size, data, parity);
        break;
    case RS_CPU_AVX2:
        rs_compute_parity_avx2_tmpl<D, P>(r, size, data, parity);
        break;
    case RS_CPU_GFNI:
        rs_compute_parity_gfni_tmpl<D, P>(r, size, data, parity);
        break;
    default:
        die("bad cpu_level %d\n", l);
    }
}

static void (*rs_compute_parity_funcs[256])(struct rs* r, uint64_t size, const uint8_t** data, uint8_t** parity);
void rs_compute_parity(struct rs* r, uint64_t size, const uint8_t** data, uint8_t** parity) {
    rs_compute_parity_funcs[r->parity](r, size, data, parity);
}

template<int D> __attribute__((noinline))
static void rs_recover_matmul_scalar_tmpl(uint64_t size, const uint8_t** have, uint8_t* want, const uint8_t* mat) {
    rs_recover_matmul_scalar(D, size, have, want, mat);
}

template<int D> __attribute__((noinline))
static void rs_recover_matmul_avx2_tmpl(uint64_t size, const uint8_t** have, uint8_t* want, const uint8_t* mat) {
    rs_recover_matmul_avx2(D, size, have, want, mat);
}

template<int D> __attribute__((noinline))
static void rs_recover_matmul_gfni_tmpl(uint64_t size, const uint8_t** have, uint8_t* want, const uint8_t* mat) {
    rs_recover_matmul_gfni(D, size, have, want, mat);
}

template<int D>
static void rs_recover_matmul_tmpl(uint64_t size, const uint8_t** have, uint8_t* want, const uint8_t* mat) {
    switch (rs_cpu_level l = rs_get_cpu_level()) {
    case RS_CPU_SCALAR:
        rs_recover_matmul_scalar_tmpl<D>(size, have, want, mat);
        break;
    case RS_CPU_AVX2:
        rs_recover_matmul_avx2_tmpl<D>(size, have, want, mat);
        break;
    case RS_CPU_GFNI:
        rs_recover_matmul_gfni_tmpl<D>(size, have, want, mat);
        break;
    default:
        die("bad cpu_level %d\n", l);
    }
}

static void (*rs_recover_matmul_funcs[16])(uint64_t size, const uint8_t** have, uint8_t* want, const uint8_t* mat);
void rs_recover(
    struct rs* r,
    uint64_t size,
    uint32_t have_blocks,
    const uint8_t** have,
    uint32_t want_block,
    uint8_t* want
) {
    int D = rs_data_blocks(r->parity);
    u8* mat = (u8*)malloc(RS_RECOVER_MAT_SIZE(D));
    if (mat == NULL) {
        free(mat);
        die("could not allocate mat");
    }
    if (!rs_recover_mat(r, have_blocks, want_block, mat)) {
        free(mat);
        die("could not get recover matrix");
    }
    rs_recover_matmul_funcs[D](size, have, want, mat);
    free(mat);
}

__attribute__((constructor))
static void rs_initialize_compute_parity_funcs() {
    rs_compute_parity_funcs[rs_mk_parity(0, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 1)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 2)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 3)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 4)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 5)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 6)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 7)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 8)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 9)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 10)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 11)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 12)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 13)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 14)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(0, 15)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 1)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 2)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 3)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 4)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 5)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 6)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 7)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 8)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 9)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 10)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 11)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 12)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 13)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 14)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(1, 15)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(2, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(2, 1)] = &rs_compute_parity_tmpl<2, 1>;
    rs_compute_parity_funcs[rs_mk_parity(2, 2)] = &rs_compute_parity_tmpl<2, 2>;
    rs_compute_parity_funcs[rs_mk_parity(2, 3)] = &rs_compute_parity_tmpl<2, 3>;
    rs_compute_parity_funcs[rs_mk_parity(2, 4)] = &rs_compute_parity_tmpl<2, 4>;
    rs_compute_parity_funcs[rs_mk_parity(2, 5)] = &rs_compute_parity_tmpl<2, 5>;
    rs_compute_parity_funcs[rs_mk_parity(2, 6)] = &rs_compute_parity_tmpl<2, 6>;
    rs_compute_parity_funcs[rs_mk_parity(2, 7)] = &rs_compute_parity_tmpl<2, 7>;
    rs_compute_parity_funcs[rs_mk_parity(2, 8)] = &rs_compute_parity_tmpl<2, 8>;
    rs_compute_parity_funcs[rs_mk_parity(2, 9)] = &rs_compute_parity_tmpl<2, 9>;
    rs_compute_parity_funcs[rs_mk_parity(2, 10)] = &rs_compute_parity_tmpl<2, 10>;
    rs_compute_parity_funcs[rs_mk_parity(2, 11)] = &rs_compute_parity_tmpl<2, 11>;
    rs_compute_parity_funcs[rs_mk_parity(2, 12)] = &rs_compute_parity_tmpl<2, 12>;
    rs_compute_parity_funcs[rs_mk_parity(2, 13)] = &rs_compute_parity_tmpl<2, 13>;
    rs_compute_parity_funcs[rs_mk_parity(2, 14)] = &rs_compute_parity_tmpl<2, 14>;
    rs_compute_parity_funcs[rs_mk_parity(2, 15)] = &rs_compute_parity_tmpl<2, 15>;
    rs_compute_parity_funcs[rs_mk_parity(3, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(3, 1)] = &rs_compute_parity_tmpl<3, 1>;
    rs_compute_parity_funcs[rs_mk_parity(3, 2)] = &rs_compute_parity_tmpl<3, 2>;
    rs_compute_parity_funcs[rs_mk_parity(3, 3)] = &rs_compute_parity_tmpl<3, 3>;
    rs_compute_parity_funcs[rs_mk_parity(3, 4)] = &rs_compute_parity_tmpl<3, 4>;
    rs_compute_parity_funcs[rs_mk_parity(3, 5)] = &rs_compute_parity_tmpl<3, 5>;
    rs_compute_parity_funcs[rs_mk_parity(3, 6)] = &rs_compute_parity_tmpl<3, 6>;
    rs_compute_parity_funcs[rs_mk_parity(3, 7)] = &rs_compute_parity_tmpl<3, 7>;
    rs_compute_parity_funcs[rs_mk_parity(3, 8)] = &rs_compute_parity_tmpl<3, 8>;
    rs_compute_parity_funcs[rs_mk_parity(3, 9)] = &rs_compute_parity_tmpl<3, 9>;
    rs_compute_parity_funcs[rs_mk_parity(3, 10)] = &rs_compute_parity_tmpl<3, 10>;
    rs_compute_parity_funcs[rs_mk_parity(3, 11)] = &rs_compute_parity_tmpl<3, 11>;
    rs_compute_parity_funcs[rs_mk_parity(3, 12)] = &rs_compute_parity_tmpl<3, 12>;
    rs_compute_parity_funcs[rs_mk_parity(3, 13)] = &rs_compute_parity_tmpl<3, 13>;
    rs_compute_parity_funcs[rs_mk_parity(3, 14)] = &rs_compute_parity_tmpl<3, 14>;
    rs_compute_parity_funcs[rs_mk_parity(3, 15)] = &rs_compute_parity_tmpl<3, 15>;
    rs_compute_parity_funcs[rs_mk_parity(4, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(4, 1)] = &rs_compute_parity_tmpl<4, 1>;
    rs_compute_parity_funcs[rs_mk_parity(4, 2)] = &rs_compute_parity_tmpl<4, 2>;
    rs_compute_parity_funcs[rs_mk_parity(4, 3)] = &rs_compute_parity_tmpl<4, 3>;
    rs_compute_parity_funcs[rs_mk_parity(4, 4)] = &rs_compute_parity_tmpl<4, 4>;
    rs_compute_parity_funcs[rs_mk_parity(4, 5)] = &rs_compute_parity_tmpl<4, 5>;
    rs_compute_parity_funcs[rs_mk_parity(4, 6)] = &rs_compute_parity_tmpl<4, 6>;
    rs_compute_parity_funcs[rs_mk_parity(4, 7)] = &rs_compute_parity_tmpl<4, 7>;
    rs_compute_parity_funcs[rs_mk_parity(4, 8)] = &rs_compute_parity_tmpl<4, 8>;
    rs_compute_parity_funcs[rs_mk_parity(4, 9)] = &rs_compute_parity_tmpl<4, 9>;
    rs_compute_parity_funcs[rs_mk_parity(4, 10)] = &rs_compute_parity_tmpl<4, 10>;
    rs_compute_parity_funcs[rs_mk_parity(4, 11)] = &rs_compute_parity_tmpl<4, 11>;
    rs_compute_parity_funcs[rs_mk_parity(4, 12)] = &rs_compute_parity_tmpl<4, 12>;
    rs_compute_parity_funcs[rs_mk_parity(4, 13)] = &rs_compute_parity_tmpl<4, 13>;
    rs_compute_parity_funcs[rs_mk_parity(4, 14)] = &rs_compute_parity_tmpl<4, 14>;
    rs_compute_parity_funcs[rs_mk_parity(4, 15)] = &rs_compute_parity_tmpl<4, 15>;
    rs_compute_parity_funcs[rs_mk_parity(5, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(5, 1)] = &rs_compute_parity_tmpl<5, 1>;
    rs_compute_parity_funcs[rs_mk_parity(5, 2)] = &rs_compute_parity_tmpl<5, 2>;
    rs_compute_parity_funcs[rs_mk_parity(5, 3)] = &rs_compute_parity_tmpl<5, 3>;
    rs_compute_parity_funcs[rs_mk_parity(5, 4)] = &rs_compute_parity_tmpl<5, 4>;
    rs_compute_parity_funcs[rs_mk_parity(5, 5)] = &rs_compute_parity_tmpl<5, 5>;
    rs_compute_parity_funcs[rs_mk_parity(5, 6)] = &rs_compute_parity_tmpl<5, 6>;
    rs_compute_parity_funcs[rs_mk_parity(5, 7)] = &rs_compute_parity_tmpl<5, 7>;
    rs_compute_parity_funcs[rs_mk_parity(5, 8)] = &rs_compute_parity_tmpl<5, 8>;
    rs_compute_parity_funcs[rs_mk_parity(5, 9)] = &rs_compute_parity_tmpl<5, 9>;
    rs_compute_parity_funcs[rs_mk_parity(5, 10)] = &rs_compute_parity_tmpl<5, 10>;
    rs_compute_parity_funcs[rs_mk_parity(5, 11)] = &rs_compute_parity_tmpl<5, 11>;
    rs_compute_parity_funcs[rs_mk_parity(5, 12)] = &rs_compute_parity_tmpl<5, 12>;
    rs_compute_parity_funcs[rs_mk_parity(5, 13)] = &rs_compute_parity_tmpl<5, 13>;
    rs_compute_parity_funcs[rs_mk_parity(5, 14)] = &rs_compute_parity_tmpl<5, 14>;
    rs_compute_parity_funcs[rs_mk_parity(5, 15)] = &rs_compute_parity_tmpl<5, 15>;
    rs_compute_parity_funcs[rs_mk_parity(6, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(6, 1)] = &rs_compute_parity_tmpl<6, 1>;
    rs_compute_parity_funcs[rs_mk_parity(6, 2)] = &rs_compute_parity_tmpl<6, 2>;
    rs_compute_parity_funcs[rs_mk_parity(6, 3)] = &rs_compute_parity_tmpl<6, 3>;
    rs_compute_parity_funcs[rs_mk_parity(6, 4)] = &rs_compute_parity_tmpl<6, 4>;
    rs_compute_parity_funcs[rs_mk_parity(6, 5)] = &rs_compute_parity_tmpl<6, 5>;
    rs_compute_parity_funcs[rs_mk_parity(6, 6)] = &rs_compute_parity_tmpl<6, 6>;
    rs_compute_parity_funcs[rs_mk_parity(6, 7)] = &rs_compute_parity_tmpl<6, 7>;
    rs_compute_parity_funcs[rs_mk_parity(6, 8)] = &rs_compute_parity_tmpl<6, 8>;
    rs_compute_parity_funcs[rs_mk_parity(6, 9)] = &rs_compute_parity_tmpl<6, 9>;
    rs_compute_parity_funcs[rs_mk_parity(6, 10)] = &rs_compute_parity_tmpl<6, 10>;
    rs_compute_parity_funcs[rs_mk_parity(6, 11)] = &rs_compute_parity_tmpl<6, 11>;
    rs_compute_parity_funcs[rs_mk_parity(6, 12)] = &rs_compute_parity_tmpl<6, 12>;
    rs_compute_parity_funcs[rs_mk_parity(6, 13)] = &rs_compute_parity_tmpl<6, 13>;
    rs_compute_parity_funcs[rs_mk_parity(6, 14)] = &rs_compute_parity_tmpl<6, 14>;
    rs_compute_parity_funcs[rs_mk_parity(6, 15)] = &rs_compute_parity_tmpl<6, 15>;
    rs_compute_parity_funcs[rs_mk_parity(7, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(7, 1)] = &rs_compute_parity_tmpl<7, 1>;
    rs_compute_parity_funcs[rs_mk_parity(7, 2)] = &rs_compute_parity_tmpl<7, 2>;
    rs_compute_parity_funcs[rs_mk_parity(7, 3)] = &rs_compute_parity_tmpl<7, 3>;
    rs_compute_parity_funcs[rs_mk_parity(7, 4)] = &rs_compute_parity_tmpl<7, 4>;
    rs_compute_parity_funcs[rs_mk_parity(7, 5)] = &rs_compute_parity_tmpl<7, 5>;
    rs_compute_parity_funcs[rs_mk_parity(7, 6)] = &rs_compute_parity_tmpl<7, 6>;
    rs_compute_parity_funcs[rs_mk_parity(7, 7)] = &rs_compute_parity_tmpl<7, 7>;
    rs_compute_parity_funcs[rs_mk_parity(7, 8)] = &rs_compute_parity_tmpl<7, 8>;
    rs_compute_parity_funcs[rs_mk_parity(7, 9)] = &rs_compute_parity_tmpl<7, 9>;
    rs_compute_parity_funcs[rs_mk_parity(7, 10)] = &rs_compute_parity_tmpl<7, 10>;
    rs_compute_parity_funcs[rs_mk_parity(7, 11)] = &rs_compute_parity_tmpl<7, 11>;
    rs_compute_parity_funcs[rs_mk_parity(7, 12)] = &rs_compute_parity_tmpl<7, 12>;
    rs_compute_parity_funcs[rs_mk_parity(7, 13)] = &rs_compute_parity_tmpl<7, 13>;
    rs_compute_parity_funcs[rs_mk_parity(7, 14)] = &rs_compute_parity_tmpl<7, 14>;
    rs_compute_parity_funcs[rs_mk_parity(7, 15)] = &rs_compute_parity_tmpl<7, 15>;
    rs_compute_parity_funcs[rs_mk_parity(8, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(8, 1)] = &rs_compute_parity_tmpl<8, 1>;
    rs_compute_parity_funcs[rs_mk_parity(8, 2)] = &rs_compute_parity_tmpl<8, 2>;
    rs_compute_parity_funcs[rs_mk_parity(8, 3)] = &rs_compute_parity_tmpl<8, 3>;
    rs_compute_parity_funcs[rs_mk_parity(8, 4)] = &rs_compute_parity_tmpl<8, 4>;
    rs_compute_parity_funcs[rs_mk_parity(8, 5)] = &rs_compute_parity_tmpl<8, 5>;
    rs_compute_parity_funcs[rs_mk_parity(8, 6)] = &rs_compute_parity_tmpl<8, 6>;
    rs_compute_parity_funcs[rs_mk_parity(8, 7)] = &rs_compute_parity_tmpl<8, 7>;
    rs_compute_parity_funcs[rs_mk_parity(8, 8)] = &rs_compute_parity_tmpl<8, 8>;
    rs_compute_parity_funcs[rs_mk_parity(8, 9)] = &rs_compute_parity_tmpl<8, 9>;
    rs_compute_parity_funcs[rs_mk_parity(8, 10)] = &rs_compute_parity_tmpl<8, 10>;
    rs_compute_parity_funcs[rs_mk_parity(8, 11)] = &rs_compute_parity_tmpl<8, 11>;
    rs_compute_parity_funcs[rs_mk_parity(8, 12)] = &rs_compute_parity_tmpl<8, 12>;
    rs_compute_parity_funcs[rs_mk_parity(8, 13)] = &rs_compute_parity_tmpl<8, 13>;
    rs_compute_parity_funcs[rs_mk_parity(8, 14)] = &rs_compute_parity_tmpl<8, 14>;
    rs_compute_parity_funcs[rs_mk_parity(8, 15)] = &rs_compute_parity_tmpl<8, 15>;
    rs_compute_parity_funcs[rs_mk_parity(9, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(9, 1)] = &rs_compute_parity_tmpl<9, 1>;
    rs_compute_parity_funcs[rs_mk_parity(9, 2)] = &rs_compute_parity_tmpl<9, 2>;
    rs_compute_parity_funcs[rs_mk_parity(9, 3)] = &rs_compute_parity_tmpl<9, 3>;
    rs_compute_parity_funcs[rs_mk_parity(9, 4)] = &rs_compute_parity_tmpl<9, 4>;
    rs_compute_parity_funcs[rs_mk_parity(9, 5)] = &rs_compute_parity_tmpl<9, 5>;
    rs_compute_parity_funcs[rs_mk_parity(9, 6)] = &rs_compute_parity_tmpl<9, 6>;
    rs_compute_parity_funcs[rs_mk_parity(9, 7)] = &rs_compute_parity_tmpl<9, 7>;
    rs_compute_parity_funcs[rs_mk_parity(9, 8)] = &rs_compute_parity_tmpl<9, 8>;
    rs_compute_parity_funcs[rs_mk_parity(9, 9)] = &rs_compute_parity_tmpl<9, 9>;
    rs_compute_parity_funcs[rs_mk_parity(9, 10)] = &rs_compute_parity_tmpl<9, 10>;
    rs_compute_parity_funcs[rs_mk_parity(9, 11)] = &rs_compute_parity_tmpl<9, 11>;
    rs_compute_parity_funcs[rs_mk_parity(9, 12)] = &rs_compute_parity_tmpl<9, 12>;
    rs_compute_parity_funcs[rs_mk_parity(9, 13)] = &rs_compute_parity_tmpl<9, 13>;
    rs_compute_parity_funcs[rs_mk_parity(9, 14)] = &rs_compute_parity_tmpl<9, 14>;
    rs_compute_parity_funcs[rs_mk_parity(9, 15)] = &rs_compute_parity_tmpl<9, 15>;
    rs_compute_parity_funcs[rs_mk_parity(10, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(10, 1)] = &rs_compute_parity_tmpl<10, 1>;
    rs_compute_parity_funcs[rs_mk_parity(10, 2)] = &rs_compute_parity_tmpl<10, 2>;
    rs_compute_parity_funcs[rs_mk_parity(10, 3)] = &rs_compute_parity_tmpl<10, 3>;
    rs_compute_parity_funcs[rs_mk_parity(10, 4)] = &rs_compute_parity_tmpl<10, 4>;
    rs_compute_parity_funcs[rs_mk_parity(10, 5)] = &rs_compute_parity_tmpl<10, 5>;
    rs_compute_parity_funcs[rs_mk_parity(10, 6)] = &rs_compute_parity_tmpl<10, 6>;
    rs_compute_parity_funcs[rs_mk_parity(10, 7)] = &rs_compute_parity_tmpl<10, 7>;
    rs_compute_parity_funcs[rs_mk_parity(10, 8)] = &rs_compute_parity_tmpl<10, 8>;
    rs_compute_parity_funcs[rs_mk_parity(10, 9)] = &rs_compute_parity_tmpl<10, 9>;
    rs_compute_parity_funcs[rs_mk_parity(10, 10)] = &rs_compute_parity_tmpl<10, 10>;
    rs_compute_parity_funcs[rs_mk_parity(10, 11)] = &rs_compute_parity_tmpl<10, 11>;
    rs_compute_parity_funcs[rs_mk_parity(10, 12)] = &rs_compute_parity_tmpl<10, 12>;
    rs_compute_parity_funcs[rs_mk_parity(10, 13)] = &rs_compute_parity_tmpl<10, 13>;
    rs_compute_parity_funcs[rs_mk_parity(10, 14)] = &rs_compute_parity_tmpl<10, 14>;
    rs_compute_parity_funcs[rs_mk_parity(10, 15)] = &rs_compute_parity_tmpl<10, 15>;
    rs_compute_parity_funcs[rs_mk_parity(11, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(11, 1)] = &rs_compute_parity_tmpl<11, 1>;
    rs_compute_parity_funcs[rs_mk_parity(11, 2)] = &rs_compute_parity_tmpl<11, 2>;
    rs_compute_parity_funcs[rs_mk_parity(11, 3)] = &rs_compute_parity_tmpl<11, 3>;
    rs_compute_parity_funcs[rs_mk_parity(11, 4)] = &rs_compute_parity_tmpl<11, 4>;
    rs_compute_parity_funcs[rs_mk_parity(11, 5)] = &rs_compute_parity_tmpl<11, 5>;
    rs_compute_parity_funcs[rs_mk_parity(11, 6)] = &rs_compute_parity_tmpl<11, 6>;
    rs_compute_parity_funcs[rs_mk_parity(11, 7)] = &rs_compute_parity_tmpl<11, 7>;
    rs_compute_parity_funcs[rs_mk_parity(11, 8)] = &rs_compute_parity_tmpl<11, 8>;
    rs_compute_parity_funcs[rs_mk_parity(11, 9)] = &rs_compute_parity_tmpl<11, 9>;
    rs_compute_parity_funcs[rs_mk_parity(11, 10)] = &rs_compute_parity_tmpl<11, 10>;
    rs_compute_parity_funcs[rs_mk_parity(11, 11)] = &rs_compute_parity_tmpl<11, 11>;
    rs_compute_parity_funcs[rs_mk_parity(11, 12)] = &rs_compute_parity_tmpl<11, 12>;
    rs_compute_parity_funcs[rs_mk_parity(11, 13)] = &rs_compute_parity_tmpl<11, 13>;
    rs_compute_parity_funcs[rs_mk_parity(11, 14)] = &rs_compute_parity_tmpl<11, 14>;
    rs_compute_parity_funcs[rs_mk_parity(11, 15)] = &rs_compute_parity_tmpl<11, 15>;
    rs_compute_parity_funcs[rs_mk_parity(12, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(12, 1)] = &rs_compute_parity_tmpl<12, 1>;
    rs_compute_parity_funcs[rs_mk_parity(12, 2)] = &rs_compute_parity_tmpl<12, 2>;
    rs_compute_parity_funcs[rs_mk_parity(12, 3)] = &rs_compute_parity_tmpl<12, 3>;
    rs_compute_parity_funcs[rs_mk_parity(12, 4)] = &rs_compute_parity_tmpl<12, 4>;
    rs_compute_parity_funcs[rs_mk_parity(12, 5)] = &rs_compute_parity_tmpl<12, 5>;
    rs_compute_parity_funcs[rs_mk_parity(12, 6)] = &rs_compute_parity_tmpl<12, 6>;
    rs_compute_parity_funcs[rs_mk_parity(12, 7)] = &rs_compute_parity_tmpl<12, 7>;
    rs_compute_parity_funcs[rs_mk_parity(12, 8)] = &rs_compute_parity_tmpl<12, 8>;
    rs_compute_parity_funcs[rs_mk_parity(12, 9)] = &rs_compute_parity_tmpl<12, 9>;
    rs_compute_parity_funcs[rs_mk_parity(12, 10)] = &rs_compute_parity_tmpl<12, 10>;
    rs_compute_parity_funcs[rs_mk_parity(12, 11)] = &rs_compute_parity_tmpl<12, 11>;
    rs_compute_parity_funcs[rs_mk_parity(12, 12)] = &rs_compute_parity_tmpl<12, 12>;
    rs_compute_parity_funcs[rs_mk_parity(12, 13)] = &rs_compute_parity_tmpl<12, 13>;
    rs_compute_parity_funcs[rs_mk_parity(12, 14)] = &rs_compute_parity_tmpl<12, 14>;
    rs_compute_parity_funcs[rs_mk_parity(12, 15)] = &rs_compute_parity_tmpl<12, 15>;
    rs_compute_parity_funcs[rs_mk_parity(13, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(13, 1)] = &rs_compute_parity_tmpl<13, 1>;
    rs_compute_parity_funcs[rs_mk_parity(13, 2)] = &rs_compute_parity_tmpl<13, 2>;
    rs_compute_parity_funcs[rs_mk_parity(13, 3)] = &rs_compute_parity_tmpl<13, 3>;
    rs_compute_parity_funcs[rs_mk_parity(13, 4)] = &rs_compute_parity_tmpl<13, 4>;
    rs_compute_parity_funcs[rs_mk_parity(13, 5)] = &rs_compute_parity_tmpl<13, 5>;
    rs_compute_parity_funcs[rs_mk_parity(13, 6)] = &rs_compute_parity_tmpl<13, 6>;
    rs_compute_parity_funcs[rs_mk_parity(13, 7)] = &rs_compute_parity_tmpl<13, 7>;
    rs_compute_parity_funcs[rs_mk_parity(13, 8)] = &rs_compute_parity_tmpl<13, 8>;
    rs_compute_parity_funcs[rs_mk_parity(13, 9)] = &rs_compute_parity_tmpl<13, 9>;
    rs_compute_parity_funcs[rs_mk_parity(13, 10)] = &rs_compute_parity_tmpl<13, 10>;
    rs_compute_parity_funcs[rs_mk_parity(13, 11)] = &rs_compute_parity_tmpl<13, 11>;
    rs_compute_parity_funcs[rs_mk_parity(13, 12)] = &rs_compute_parity_tmpl<13, 12>;
    rs_compute_parity_funcs[rs_mk_parity(13, 13)] = &rs_compute_parity_tmpl<13, 13>;
    rs_compute_parity_funcs[rs_mk_parity(13, 14)] = &rs_compute_parity_tmpl<13, 14>;
    rs_compute_parity_funcs[rs_mk_parity(13, 15)] = &rs_compute_parity_tmpl<13, 15>;
    rs_compute_parity_funcs[rs_mk_parity(14, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(14, 1)] = &rs_compute_parity_tmpl<14, 1>;
    rs_compute_parity_funcs[rs_mk_parity(14, 2)] = &rs_compute_parity_tmpl<14, 2>;
    rs_compute_parity_funcs[rs_mk_parity(14, 3)] = &rs_compute_parity_tmpl<14, 3>;
    rs_compute_parity_funcs[rs_mk_parity(14, 4)] = &rs_compute_parity_tmpl<14, 4>;
    rs_compute_parity_funcs[rs_mk_parity(14, 5)] = &rs_compute_parity_tmpl<14, 5>;
    rs_compute_parity_funcs[rs_mk_parity(14, 6)] = &rs_compute_parity_tmpl<14, 6>;
    rs_compute_parity_funcs[rs_mk_parity(14, 7)] = &rs_compute_parity_tmpl<14, 7>;
    rs_compute_parity_funcs[rs_mk_parity(14, 8)] = &rs_compute_parity_tmpl<14, 8>;
    rs_compute_parity_funcs[rs_mk_parity(14, 9)] = &rs_compute_parity_tmpl<14, 9>;
    rs_compute_parity_funcs[rs_mk_parity(14, 10)] = &rs_compute_parity_tmpl<14, 10>;
    rs_compute_parity_funcs[rs_mk_parity(14, 11)] = &rs_compute_parity_tmpl<14, 11>;
    rs_compute_parity_funcs[rs_mk_parity(14, 12)] = &rs_compute_parity_tmpl<14, 12>;
    rs_compute_parity_funcs[rs_mk_parity(14, 13)] = &rs_compute_parity_tmpl<14, 13>;
    rs_compute_parity_funcs[rs_mk_parity(14, 14)] = &rs_compute_parity_tmpl<14, 14>;
    rs_compute_parity_funcs[rs_mk_parity(14, 15)] = &rs_compute_parity_tmpl<14, 15>;
    rs_compute_parity_funcs[rs_mk_parity(15, 0)] = nullptr;
    rs_compute_parity_funcs[rs_mk_parity(15, 1)] = &rs_compute_parity_tmpl<15, 1>;
    rs_compute_parity_funcs[rs_mk_parity(15, 2)] = &rs_compute_parity_tmpl<15, 2>;
    rs_compute_parity_funcs[rs_mk_parity(15, 3)] = &rs_compute_parity_tmpl<15, 3>;
    rs_compute_parity_funcs[rs_mk_parity(15, 4)] = &rs_compute_parity_tmpl<15, 4>;
    rs_compute_parity_funcs[rs_mk_parity(15, 5)] = &rs_compute_parity_tmpl<15, 5>;
    rs_compute_parity_funcs[rs_mk_parity(15, 6)] = &rs_compute_parity_tmpl<15, 6>;
    rs_compute_parity_funcs[rs_mk_parity(15, 7)] = &rs_compute_parity_tmpl<15, 7>;
    rs_compute_parity_funcs[rs_mk_parity(15, 8)] = &rs_compute_parity_tmpl<15, 8>;
    rs_compute_parity_funcs[rs_mk_parity(15, 9)] = &rs_compute_parity_tmpl<15, 9>;
    rs_compute_parity_funcs[rs_mk_parity(15, 10)] = &rs_compute_parity_tmpl<15, 10>;
    rs_compute_parity_funcs[rs_mk_parity(15, 11)] = &rs_compute_parity_tmpl<15, 11>;
    rs_compute_parity_funcs[rs_mk_parity(15, 12)] = &rs_compute_parity_tmpl<15, 12>;
    rs_compute_parity_funcs[rs_mk_parity(15, 13)] = &rs_compute_parity_tmpl<15, 13>;
    rs_compute_parity_funcs[rs_mk_parity(15, 14)] = &rs_compute_parity_tmpl<15, 14>;
    rs_compute_parity_funcs[rs_mk_parity(15, 15)] = &rs_compute_parity_tmpl<15, 15>;
}

__attribute__((constructor))
static void rs_initialize_recover_matmul_funcs() {
    rs_recover_matmul_funcs[0] = nullptr;
    rs_recover_matmul_funcs[1] = nullptr;
    rs_recover_matmul_funcs[2] = &rs_recover_matmul_tmpl<2>;
    rs_recover_matmul_funcs[3] = &rs_recover_matmul_tmpl<3>;
    rs_recover_matmul_funcs[4] = &rs_recover_matmul_tmpl<4>;
    rs_recover_matmul_funcs[5] = &rs_recover_matmul_tmpl<5>;
    rs_recover_matmul_funcs[6] = &rs_recover_matmul_tmpl<6>;
    rs_recover_matmul_funcs[7] = &rs_recover_matmul_tmpl<7>;
    rs_recover_matmul_funcs[8] = &rs_recover_matmul_tmpl<8>;
    rs_recover_matmul_funcs[9] = &rs_recover_matmul_tmpl<9>;
    rs_recover_matmul_funcs[10] = &rs_recover_matmul_tmpl<10>;
    rs_recover_matmul_funcs[11] = &rs_recover_matmul_tmpl<11>;
    rs_recover_matmul_funcs[12] = &rs_recover_matmul_tmpl<12>;
    rs_recover_matmul_funcs[13] = &rs_recover_matmul_tmpl<13>;
    rs_recover_matmul_funcs[14] = &rs_recover_matmul_tmpl<14>;
    rs_recover_matmul_funcs[15] = &rs_recover_matmul_tmpl<15>;
}
