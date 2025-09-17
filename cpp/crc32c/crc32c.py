# Copyright 2025 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

# File to remind myself how to derive:
#
# * CRC32C(A ^ B) given CRC32C(A) and CRC32C(B)
# * CRC32C(A `append` B) given CRC32C(A) and CRC32C(B)
#
# Below I use 8-bit, but it doesn't matter. Also, I use
# "precond" for the CRC with only preconditioning, and
# "prepostcond" for the CRC with both pre- and postconditioning.
#
# With pre- and post-conditioning we mean the two measures
# here: <https://en.wikipedia.org/wiki/Computation_of_cyclic_redundancy_checks#CRC_variants>.
#
# Basically, the only operation we need to make everything fast
# is a fast "extend by N zeros" operation.
import random

# x^8 + x^2 + x + 1
POLY = [1,0,0,0,0,0,1,1,1]
BITS = len(POLY)-1

def xor(bs, *args):
    result = bs[:]
    for b in args:
        assert len(b) == len(result)
        for i in range(len(result)):
            result[i] ^= b[i]
    return result

# Computes the CRC of a bitstring, which needs to already
# be at least the size of the CRC register.
def crc(bitstring):
    assert len(bitstring) >= BITS # necessary if we want the output to be BITS long, without having to pad anyway
    assert len(bitstring)%BITS == 0 # not really necessary, actually
    result = bitstring[:]
    for i in range(len(result)-BITS):
        if result[i] == 1:
            for j in range(len(POLY)):
                result[i+j] ^= POLY[j]
    return result[-BITS:]

# CRC(A^B) == CRC(A)^CRC(B)
def crc_xor(crc1, crc2):
    xor(crc1, crc2)

def crc_append(crc1, crc2, l2):
    return xor(crc(crc1 + [0]*l2), crc2)

def random_string(l):
    return [random.randrange(2) for x in range((l+1)*BITS)]

for i in range(100):
    bs1 = random_string(random.randrange(20))
    bs2 = random_string(random.randrange(20))
    assert crc(bs1 + bs2) == crc_append(crc(bs1), crc(bs2), len(bs2))

def crc_precond(bs):
    return crc([1]*BITS + bs)

def crc_precond_append(crc1, crc2, l2):
    return xor(crc(xor(crc1, [1]*BITS) + [0]*len(bs2)), crc2)

for i in range(100):
    l = random.randrange(20)
    bs1 = random_string(l)
    bs2 = random_string(l)
    assert crc_precond(bs1 + bs2) == crc_precond_append(crc_precond(bs1), crc_precond(bs2), len(bs2))

def crc_prepostcond(bs):
    return xor(crc_precond(bs), [1]*BITS)

def crc_prepostcond_append(crc1, crc2, l2):
    return xor(crc(crc1 + [0]*len(bs2)), crc2)

for i in range(100):
    bs1 = random_string(random.randrange(20))
    bs2 = random_string(random.randrange(20))
    assert crc_prepostcond(bs1 + bs2) == crc_prepostcond_append(crc_prepostcond(bs1), crc_prepostcond(bs2), len(bs2))

def crc_precond_xor(crc1, crc2, l):
    return xor(crc1, crc2, crc_precond([0]*l))

for i in range(100):
    l = random.randrange(20)
    bs1 = random_string(l)
    bs2 = random_string(l)
    assert crc_precond(xor(bs1, bs2)) == crc_precond_xor(crc_precond(bs1), crc_precond(bs2), len(bs2))

def crc_prepostcond_xor(crc1, crc2, l):
    return xor(crc1, crc2, crc_prepostcond([0]*l))

for i in range(100):
    l = random.randrange(20)
    bs1 = random_string(l)
    bs2 = random_string(l)
    assert crc_prepostcond(xor(bs1, bs2)) == crc_prepostcond_xor(crc_prepostcond(bs1), crc_prepostcond(bs2), len(bs2))
