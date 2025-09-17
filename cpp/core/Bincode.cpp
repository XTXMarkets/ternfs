// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "Bincode.hpp"
#include "Common.hpp"

std::ostream& operator<<(std::ostream& out, const BincodeBytesRef& x) {
    return goLangBytesFmt(out, x.data(), x.size());
}

std::ostream& operator<<(std::ostream& out, const BincodeBytes& x) {
    return out << x.ref();
}

const char* BincodeException::what() const noexcept {
    return _msg.c_str();
}
