// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// Computes the CRC32C (C = "Castagnoli" polynomial, also used in iSCSI)
//
// As usual, the CRC is initialized with -1, and xor'd with -1 at the end.
// Since we provide `crc32c` so that the CRC can be computed incrementally,
// we just invert the crc at the beginning and at the end.
//
// See <https://en.wikipedia.org/wiki/Computation_of_cyclic_redundancy_checks#CRC_variants>.
//
// The pclmul versions use pclmul instructions, and are therefore generally faster. They're
// otherwise identical to the non-pclmul versions.
#ifndef TERN_CRC32C
#define TERN_CRC32C

#include <stdint.h>
#include <sys/types.h>

#ifdef __cplusplus
extern "C" {
#endif

// Initialize crc with 0.
uint32_t crc32c(uint32_t crc, const char* buf, size_t len);

// Computes the CRC32C of the XOR between two equally long
// strings, given their crcs and their length.
uint32_t crc32c_xor(uint32_t crc1, uint32_t crc2, size_t len);

// Computes the CRC32C of the concatenation of two strings, given
// their crcs and the length of the second string.
uint32_t crc32c_append(uint32_t crc1, uint32_t crc2, size_t len2);

// Computes the CRC32C that you'd get by doing
//
//     char* zero_bytes = (char*)malloc(zeros);
//     memset(zero_bytes, 0, zeros);
//     crc = crc32c(crc, zero_bytes, zeros);
//
// `zeros` can be negative, in which case this "undoes" `zeros`
// zeroes.
uint32_t crc32c_zero_extend(uint32_t crc, ssize_t zeros);

uint32_t crc32c_pclmul(uint32_t crc, const char* buf, size_t len);
uint32_t crc32c_xor_pclmul(uint32_t crc1, uint32_t crc2, size_t len);
uint32_t crc32c_append_pclmul(uint32_t crc1, uint32_t crc2, size_t len2);
uint32_t crc32c_zero_extend_pclmul(uint32_t crc, ssize_t zeros);

#ifdef __cplusplus
}
#endif

#endif
