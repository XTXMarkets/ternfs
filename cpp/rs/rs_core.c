// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#ifndef __KERNEL__

typedef uint8_t u8;
typedef uint32_t u32;
typedef uint64_t u64;

#endif

extern const u8 rs_gf_inv_table[256];
extern const u8 rs_gf_log_table[256];
extern const u8 rs_gf_exp_table[256];

static inline u8 gf_inv(u8 x) {
    return rs_gf_inv_table[x];
}

static inline u8 gf_mul(u8 x, u8 y) {
    if (x == 0 || y == 0) {
        return 0;
    }
    int i = rs_gf_log_table[x] + rs_gf_log_table[y];
    return rs_gf_exp_table[i > 254 ? i - 255 : i];
}

static inline void gf_mul_expand_factor(u8 x, u8* expanded_x) {
    int i;
    for (i = 0; i < 16; i++) {
        expanded_x[i] = gf_mul(i, x);
    }
    for (i = 0; i < 16; i++) {
        expanded_x[16 + i] = gf_mul(i << 4, x);
    }
}

static inline u8 gf_mul_expanded(u8 x, const u8* expanded_y) {
    return expanded_y[x & 0x0f] ^ expanded_y[16 + ((x & 0xf0) >> 4)];
}

__attribute__((target("avx2")))
static inline __m256i gf_mul_expanded_avx2(__m256i x, __m256i expanded_y, __m256i low_nibble_mask) {
    __m256i expanded_y_lo = _mm256_permute2x128_si256(expanded_y, expanded_y, 0x00);
    __m256i expanded_y_hi = _mm256_permute2x128_si256(expanded_y, expanded_y, 0x11);

    __m256i x_lo = _mm256_and_si256(x, low_nibble_mask);
    __m256i x_hi = _mm256_and_si256(_mm256_srli_epi16(x, 4), low_nibble_mask);

    return _mm256_xor_si256(
        _mm256_shuffle_epi8(expanded_y_lo, x_lo),
        _mm256_shuffle_epi8(expanded_y_hi, x_hi)
    );
}

static inline u8 rs_data_blocks_core(u8 parity) {
    return parity & 0x0F;
}

static inline u8 rs_parity_blocks_core(u8 parity) {
    return parity >> 4;
}

static inline u8 rs_blocks_core(u8 parity) {
    return rs_data_blocks_core(parity) + rs_parity_blocks_core(parity);
}

// From <https://github.com/intel/isa-l/blob/33a2d9484595c2d6516c920ce39a694c144ddf69/erasure_code/ec_base.c#L110>,
// just Gaussian elimination.
//
// TODO this is in row-major, it'd be nice to have it in column-major
// to save have the final operation to be more natural in rs_recover.
static bool rs_gf_invert_matrix(u8* in_mat, u8* out_mat, const int n) {
    int i, j, k;
    u8 temp;

    // Set out_mat[] to the identity matrix
    memset(out_mat, 0, n*n);
    for (i = 0; i < n; i++) {
        out_mat[i * n + i] = 1;
    }

    // Inverse
    for (i = 0; i < n; i++) {
        // Check for 0 in pivot element
        if (in_mat[i * n + i] == 0) {
            // Find a row with non-zero in current column and swap
            for (j = i + 1; j < n; j++) {
                if (in_mat[j * n + i]) {
                    break;
                }
            }

            if (j == n) {    // Couldn't find means it's singular
                return false;
            }

            for (k = 0; k < n; k++) {    // Swap rows i,j
                temp = in_mat[i * n + k];
                in_mat[i * n + k] = in_mat[j * n + k];
                in_mat[j * n + k] = temp;

                temp = out_mat[i * n + k];
                out_mat[i * n + k] = out_mat[j * n + k];
                out_mat[j * n + k] = temp;
            }
        }

        temp = gf_inv(in_mat[i * n + i]);    // 1/pivot
        for (j = 0; j < n; j++) {    // Scale row i by 1/pivot
            in_mat[i * n + j] = gf_mul(in_mat[i * n + j], temp);
            out_mat[i * n + j] = gf_mul(out_mat[i * n + j], temp);
        }

        for (j = 0; j < n; j++) {
            if (j == i) {
                continue;
            }

            temp = in_mat[j * n + i];
            for (k = 0; k < n; k++) {
                out_mat[j * n + k] ^= gf_mul(temp, out_mat[i * n + k]);
                in_mat[j * n + k] ^= gf_mul(temp, in_mat[i * n + k]);
            }
        }
    }
    return true;
}

struct rs {
    u8 parity;
    // u8[D*B], in column-major.
    u8* matrix;
    // u8[D*P][32], in column-major. These are the lookup tables
    // to perform multiplication quickly.
    u8* expanded_matrix;
};

// Note that we pervasively assume that the first column of the parity columns
// is 1s, which causes the first parity to be the XORs of the data. So you can't
// really change how the matrix is generated.
static void rs_cauchy_matrix(struct rs* r) {
    int D = rs_data_blocks_core(r->parity);
    int B = rs_blocks_core(r->parity);
    u8* matrix = r->matrix;
    memset(matrix, 0, D*B);
    // Identity in the d*d upper half
    int i;
    for (i = 0; i < D; i++) {
        matrix[D*i + i] = 1;
    }
    // Fill in the rest using cauchy
    int col, row;
    for (col = D; col < B; col++) {
        for (row = 0; row < D; row++) {
            matrix[col*D + row] = gf_inv(col ^ row);
        }
    }
    // Scale the columns
    for (col = D; col < B; col++) {
        u8 factor = gf_inv(matrix[col*D]);
        for (row = 0; row < D; row++) {
            matrix[col*D + row] = gf_mul(matrix[col*D + row], factor);
        }
    }
    // Scale the rows
    for (row = 1; row < D; row++) {
        u8 factor = gf_inv(matrix[D*D + row]);
        for (col = D; col < B; col++) {
            matrix[col*D + row] = gf_mul(matrix[col*D + row], factor);
        }
    }
}

// out must be at leas size 4
static void rs_cpuidex(u32 function_id, u32 subfunction_id, u32* out) {
    u32 a, b, c, d;
    __asm("cpuid":"=a"(a),"=b"(b),"=c"(c),"=d"(d):"0"(function_id),"2"(subfunction_id));
    out[0] = a; out[1] = b; out[2] = c; out[3] = d;
}

static bool rs_has_cpu_level_core(enum rs_cpu_level level) {
    u32 _0_0[4];
    rs_cpuidex(0, 0, _0_0);
    u32 _7_0[4];
    if (_0_0[0] >= 7) {
        rs_cpuidex(7, 0, _7_0);
    } else {
        memset(_7_0, 0, sizeof(_7_0));
    }
    switch (level) {
    case RS_CPU_SCALAR:
        return true;
    case RS_CPU_AVX2:
        return _7_0[1] & (1<<5);
    case RS_CPU_GFNI:
        return _7_0[2] & (1<<8) && !rs_detect_valgrind();
    default:
        rs_warn("bad CPU level %d", level);
        return false;
    }
}

#define rs_compute_parity_single(D, P, r, i, data, parity) do { \
        parity[0][i] = 0; \
        int d; \
        int p; \
        for (d = 0; d < D; d++) { \
            parity[0][i] ^= data[d][i]; \
        } \
        for (p = 1; p < P; p++) { \
            const u8* factor = &r->expanded_matrix[D*32*p]; \
            parity[p][i] = 0; \
            for (d = 0; d < D; d++, factor += 32) { \
                parity[p][i] ^= gf_mul_expanded(data[d][i], factor); \
            } \
        } \
    } while (0)

#define rs_compute_parity_scalar(D, P, r, size, data, parity) do { \
        /* parity = r->matrix * data */ \
        u64 i; \
        for (i = 0; i < size; i++) { \
            rs_compute_parity_single(D, P, r, i, data, parity); \
        } \
    } while (0)

#define rs_compute_parity_avx2(D, P, r, size, data, parity) do { \
        __m256i low_nibble_mask = broadcast_u8(0x0f); \
        size_t avx_leftover = size % 32; \
        size_t avx_size = size-avx_leftover; \
        u32 i; \
        int p; \
        int d; \
        for (i = 0; i < avx_size; i += 32) { \
            { \
                __m256i parity_0 = _mm256_setzero_si256(); \
                for (d = 0; d < D; d++) { \
                    parity_0 = _mm256_xor_si256(parity_0, _mm256_loadu_si256((const __m256i*)(data[d] + i))); \
                } \
                _mm256_storeu_si256((__m256i*)(parity[0] + i), parity_0); \
            } \
            for (p = 1; p < P; p++) { \
                __m256i parity_p = _mm256_setzero_si256(); \
                for (d = 0; d < D; d++) { \
                    __m256i data_d = _mm256_loadu_si256((const __m256i*)(data[d] + i)); \
                    __m256i factor = _mm256_loadu_si256((const __m256i*)&r->expanded_matrix[D*p*32 + 32*d]); \
                    parity_p = _mm256_xor_si256(parity_p, gf_mul_expanded_avx2(data_d, factor, low_nibble_mask)); \
                } \
                _mm256_storeu_si256((__m256i*)(parity[p] + i), parity_p); \
            } \
        } \
        for (i = avx_size; i < size; i++) { \
            rs_compute_parity_single(D, P, r, i, data, parity); \
        } \
    } while (0)

#define rs_compute_parity_gfni(D, P, r, size, data, parity) do { \
        size_t avx_leftover = size % 32; \
        size_t avx_size = size-avx_leftover; \
        u64 i; \
        int p; \
        int d; \
        for (i = 0; i < avx_size; i += 32) { \
            { \
                __m256i parity_0 = _mm256_setzero_si256(); \
                for (d = 0; d < D; d++) { \
                    parity_0 = _mm256_xor_si256(parity_0, _mm256_loadu_si256((const __m256i*)(data[d] + i))); \
                } \
                _mm256_storeu_si256((__m256i*)(parity[0] + i), parity_0); \
            } \
            for (p = 1; p < P; p++) { \
                __m256i parity_p = _mm256_setzero_si256(); \
                for (d = 0; d < D; d++) { \
                    __m256i data_d = _mm256_loadu_si256((const __m256i*)(data[d] + i)); \
                    __m256i factor = broadcast_u8(r->matrix[D*D + D*p + d]); \
                    parity_p = _mm256_xor_si256(parity_p, _mm256_gf2p8mul_epi8(data_d, factor)); \
                } \
                _mm256_storeu_si256((__m256i*)(parity[p] + i), parity_p); \
            } \
        } \
        for (i = avx_size; i < size; i++) { \
            rs_compute_parity_single(D, P, r, i, data, parity); \
        } \
    } while (0)

#define rs_recover_matmul_single(D, i, have, want, have_to_want) do { \
        want[i] = 0; \
        int j; \
        for (j = 0; j < D; j++) { \
            want[i] ^= gf_mul(have_to_want[j], have[j][i]); \
        } \
    } while (0)

#define rs_recover_matmul_single_expanded(D, i, have, want, have_to_want_expanded) do { \
        want[i] = 0; \
        int j; \
        for (j = 0; j < D; j++) { \
            want[i] ^= gf_mul_expanded(have[j][i], &have_to_want_expanded[j*32]); \
        } \
    } while(0)

#define rs_recover_matmul_scalar(D, size, have, want, have_to_want) do { \
        u8 have_to_want_expanded[D*32]; \
        int d; \
        for (d = 0; d < D; d++) { \
            gf_mul_expand_factor(have_to_want[d], &have_to_want_expanded[d*32]); \
        } \
        size_t i; \
        for (i = 0; i < size; i++) { \
            rs_recover_matmul_single_expanded(D, i, have, want, have_to_want_expanded); \
        } \
    } while (0) \

#define rs_recover_matmul_avx2(D, size, have, want, have_to_want) do { \
        __m256i have_to_want_expanded[D]; \
        int d; \
        for (d = 0; d < D; d++) { \
            gf_mul_expand_factor(have_to_want[d], ((u8*)&have_to_want_expanded[d])); \
        } \
        __m256i low_nibble_mask = broadcast_u8(0x0f); \
        size_t avx_leftover = size % 32; \
        size_t avx_size = size-avx_leftover; \
        u64 i; \
        for (i = 0; i < avx_size; i += 32) { \
            __m256i want_i = _mm256_setzero_si256(); \
            for (d = 0; d < D; d++) { \
                want_i = _mm256_xor_si256( \
                    want_i, \
                    gf_mul_expanded_avx2( \
                        _mm256_loadu_si256((const __m256i*)(have[d] + i)), \
                        have_to_want_expanded[d], \
                        low_nibble_mask \
                    ) \
                ); \
            } \
            _mm256_storeu_si256((__m256i*)(want + i), want_i); \
        } \
        for (i = avx_size; i < size; i++) { \
            rs_recover_matmul_single_expanded(D, i, have, want, ((const u8*)have_to_want_expanded)); \
        } \
    } while(0)

#define rs_recover_matmul_gfni(D, size, have, want, have_to_want) do { \
        __m256i have_to_want_avx[D]; \
        int d; \
        for (d = 0; d < D; d++) { \
            have_to_want_avx[d] = broadcast_u8(have_to_want[d]); \
        } \
        size_t avx_leftover = size % 32; \
        size_t avx_size = size-avx_leftover; \
        u64 i; \
        for (i = 0; i < avx_size; i += 32) { \
            __m256i want_i = _mm256_setzero_si256(); \
            for (d = 0; d < D; d++) { \
                want_i = _mm256_xor_si256( \
                    want_i, \
                    _mm256_gf2p8mul_epi8( \
                        _mm256_loadu_si256((const __m256i*)(have[d] + i)), \
                        have_to_want_avx[d] \
                    ) \
                ); \
            } \
            _mm256_storeu_si256((__m256i*)(want + i), want_i); \
        } \
        for (i = avx_size; i < size; i++) { \
            rs_recover_matmul_single(D, i, have, want, have_to_want); \
        } \
    } while (0)

#define RS_SIZE(D, P) (sizeof(struct rs) + ((D)+(P))*(D) + (D)*(P)*32)

static void rs_new_core(u8 parity, struct rs* r) {
    int B = rs_blocks_core(parity);
    int D = rs_data_blocks_core(parity);
    int P = rs_parity_blocks_core(parity);
    r->parity = parity;
    r->matrix = (u8*)(r + 1);
    r->expanded_matrix = r->matrix + B*D;
    rs_cauchy_matrix(r);
    int p;
    int d;
    for (p = 0; p < P; p++) {
        for (d = 0; d < D; d++) {
            gf_mul_expand_factor(r->matrix[D*D + D*p + d], &r->expanded_matrix[D*32*p + 32*d]);
        }
    }
}

// This is actually enough space to hold two matrices,
// we need the space for the second temporarily.
#define RS_RECOVER_MAT_SIZE(D) (D*D + D*D)

static bool rs_recover_mat(
    struct rs* r,
    u32 have_blocks,
    u32 want_block,
    u8* scratch // out mat
) {
    int D = rs_data_blocks_core(r->parity);
    int B = rs_blocks_core(r->parity);
    u8* mat_1 = scratch;
    u8* mat_2 = scratch + D*D;
    // Preliminary checks
    int i, j, d, b;
    if ((have_blocks >> B) || (want_block >> B)) {
        rs_warn("have_blocks=%08x or want_block=%08x out of bounds wrt B=%d", have_blocks, want_block, B);
        return false;
    }
    if (have_blocks & want_block) {
        rs_warn("have_blocks=%08x contains want_block=%08x", have_blocks, want_block);
        return false;
    }
    // below in the dimensionality annotation we paper over transposes
    // [DxD] matrix going from the data blocks to the blocks we currently have
    u8* data_to_have = mat_1;
    u32 have_cursor;
    for (b = 0, d = 0, have_cursor = have_blocks; b < B; b++, have_cursor >>= 1) {
        if (!(have_cursor & 1u)) { continue; }
        memcpy(data_to_have + d*D, r->matrix + b*D, D);
        d++;
    }
    // [DxD] matrix going from what we have to the original data blocks
    u8* have_to_data = mat_2;
    if (!rs_gf_invert_matrix(data_to_have, have_to_data, D)) {
        rs_warn("unexpected singular matrix");
        return false;
    }
    data_to_have = NULL;
    // [Dx1] matrix going from the data blocks to the block we want
    u8* data_to_want = &r->matrix[__builtin_ctz(want_block)*D];
    // have_to_want = data_to_want * have_to_data
    // [Dx1] matrix going from `blocks` to the block we're into
    u8* have_to_want = mat_1;
    for (i = 0; i < D; i++) {
        have_to_want[i] = 0;
        for (j = 0; j < D; j++) {
            have_to_want[i] ^= gf_mul(data_to_want[j], have_to_data[j*D + i]);
        }
    }
    // want = have_to_want * have
    // We're done.
    return true;
}
