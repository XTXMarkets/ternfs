# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

# Rijndael's poly, see <https://en.wikipedia.org/wiki/Finite_field_arithmetic#Rijndael's_(AES)_finite_field>
POLY = 0x11B # x^8 + x^4 + x^3 + x + 1, or 100011011
# See <https://en.wikipedia.org/wiki/Finite_field_arithmetic#Generator_based_tables>
GENERATOR = 0x03 # x + 1, or 00000011

# REUSE-IgnoreStart
print('// Copyright 2025 XTX Markets Technologies Limited')
print('//')
print('// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception')
# REUSE-IgnoreEnd
print()
print('// generated with gf_tables.py')
print('#ifndef __KERNEL__')
print('#include <stdint.h>')
print('#endif')
print()

def gf_mul(x, y):
    out = 0
    # Multiply and add the factor
    for i in range(8):
        if (y>>i)&1 != 0:
            out ^= x<<i
    # Compute the modulo -- shift and subtract the polynomial
    for i in range(14,7,-1):
        if (out>>i)&1 != 0:
            out ^= POLY<<(i-8)
    return out

# Exp table
print('const uint8_t rs_gf_exp_table[256] = {', end='')
exp_table = [0]*256
x = 1
for i in range(256):
    exp_table[i] = x
    if i % 16 == 0:
        print('\n    ', end='')
    print(f'{x:#04x}, ', end='')
    x = gf_mul(x, GENERATOR)
print('\n};\n')

# Log table
log_table = {}
log_table[0] = 0
for a, b in enumerate(exp_table):
    log_table[b] = a
print('const uint8_t rs_gf_log_table[256] = {', end='')
for i in range(256):
    if i % 16 == 0:
        print('\n    ', end='')
    print(f'{log_table[i]:#04x}, ', end='')
print('\n};\n')

# Inv table
inv_table = [0]*256
print('const uint8_t rs_gf_inv_table[256] = {', end='')
for i in range(256):
    if i == 0:
        x = 0
    else:
        x = exp_table[255 - log_table[i]]
    inv_table[i] = x
    if i % 16 == 0:
        print('\n    ', end='')
    print(f'{x:#04x}, ', end='')
print('\n};\n')

# Verify
for i in range(1, 256):
    assert gf_mul(i,inv_table[i]) == 1
    for j in range(1, 256):
        tmp = log_table[i] + log_table[j]
        ixj = exp_table[(tmp - 255) if tmp > 254 else tmp]
        assert gf_mul(i, j) == ixj
