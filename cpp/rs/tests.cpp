// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <stdio.h>
#include <string.h>
#include <vector>
#include <stdlib.h>
#include <algorithm>

#include "rs.h"
#include "Random.hpp"

#define ASSERT(expr) do { \
        if (!(expr)) { \
            fprintf(stderr, "Assertion failed: " #expr "\n"); \
            exit(1); \
        } \
    } while (false)

int main() {
    RandomGenerator rand(0);
    constexpr int maxBlockSize = 1000;
    std::vector<uint8_t> buf(maxBlockSize*(16+16)); // all blocks
    std::vector<rs_cpu_level> cpuLevels = {RS_CPU_SCALAR, RS_CPU_AVX2, RS_CPU_GFNI};
    for (const auto level: cpuLevels) {
        if (!rs_has_cpu_level(level)) {
            continue;
        }
        rs_set_cpu_level(level);
        for (int i = 0; i < 16*16*100; i++) {
            int numData = 2 + rand.generate64()%(16-2);
            int numParity = 1 + rand.generate64()%(16-1);
            int blockSize;
            if (rs_get_cpu_level() == RS_CPU_SCALAR) {
                static_assert(100 < maxBlockSize);
                blockSize = 1 + rand.generate64()%100;
            } else {
                blockSize = 1 + rand.generate64()%maxBlockSize;
            }
            std::vector<uint8_t> data(buf.begin(), buf.begin() + blockSize*(numData+numParity));
            rand.generateBytes((char*)data.data(), numData*blockSize);
            auto rs = rs_get(rs_mk_parity(numData, numParity));
            std::vector<uint8_t*> blocksPtrs(numData+numParity);
            for (int i = 0; i < numData+numParity; i++) {
                blocksPtrs[i] = &data[i*blockSize];
            }
            rs_compute_parity(rs, blockSize, (const uint8_t**)&blocksPtrs[0], &blocksPtrs[numData]);
            // verify that the first parity block is the XOR of all the original data
            for (size_t i = 0; i < blockSize; i++) {
                uint8_t expectedParity = 0;
                for (int j = 0; j < numData; j++) {
                    expectedParity ^= data[j*blockSize + i];
                }
                ASSERT(expectedParity == data[numData*blockSize + i]);
            }
            // restore a random block, using random blocks.
            {
                std::vector<uint8_t> allBlocks(numData+numParity);
                for (int i = 0; i < allBlocks.size(); i++) {
                    allBlocks[i] = i;
                }
                for (int i = 0; i < std::min(numData+1, numData+numParity-1); i++) {
                    std::swap(allBlocks[i], allBlocks[i+1+ rand.generate64()%(numData+numParity-1-i)]);
                }
                std::sort(allBlocks.begin(), allBlocks.begin()+numData);
                uint32_t allBlocksBits = 0;
                for (int i = 0; i < numData; i++) {
                    allBlocksBits |= 1u << allBlocks[i];
                }
                std::vector<const uint8_t*> havePtrs(numData);
                for (int i = 0; i < numData; i++) {
                    havePtrs[i] = &data[allBlocks[i]*blockSize];
                }
                uint8_t wantBlock = allBlocks[numData];
                std::vector<uint8_t> recoveredBlock(blockSize);
                rs_recover(rs, blockSize, allBlocksBits, &havePtrs[0], 1u << wantBlock, &recoveredBlock[0]);
                std::vector<uint8_t> expectedBlock(data.begin() + wantBlock*blockSize, data.begin() + (wantBlock+1)*blockSize);
                ASSERT(expectedBlock == recoveredBlock);
            }
        }        
    }
    return 0;
}
