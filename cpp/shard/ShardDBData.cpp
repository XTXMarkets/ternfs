// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include "ShardDBData.hpp"

#include <xxhash.h>

std::ostream& operator<<(std::ostream& out, SpanState state) {
    switch (state) {
    case SpanState::CLEAN:
        out << "CLEAN";
        break;
    case SpanState::DIRTY:
        out << "DIRTY";
        break;
    case SpanState::CONDEMNED:
        out << "CONDEMNED";
        break;
    default:
        out << "SpanState(" << ((int)state) << ")";
        break;
    }
    return out;
}

std::ostream& operator<<(std::ostream& out, const EdgeKey& edgeKey) {
    out << "EdgeKey(dirId=" << edgeKey.dirId() << ", current=" << (int)edgeKey.current() << ", nameHash=" << edgeKey.nameHash() << ", name=" << edgeKey.name();
    if (!edgeKey.current()) {
        out << ", creationTime=" << edgeKey.creationTime();
    }
    out << ")";
    return out;
}

uint64_t EdgeKey::computeNameHash(HashMode mode, const BincodeBytesRef& bytes) {
    switch (mode) {
    case HashMode::XXH3_63:
        return XXH3_64bits(bytes.data(), bytes.size()) & ~(1ull<<63);
    default:
        throw TERN_EXCEPTION("bad hash mode %s", (int)mode);
    }
}
