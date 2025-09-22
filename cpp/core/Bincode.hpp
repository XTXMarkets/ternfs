// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <array>
#include <bit>
#include <cstdint>
#include <exception>
#include <iomanip>
#include <rocksdb/slice.h>
#include <string>
#include <type_traits>
#include <vector>

#include "Assert.hpp"
#include "Common.hpp"
#include "Crypto.hpp"

struct BincodeBytesRef {
private:
    const char* _data;
    const uint8_t _length;

public:
    BincodeBytesRef(): _data(""), _length(0) {}
    BincodeBytesRef(const char* data, size_t length): _data(data), _length(length) {
        ALWAYS_ASSERT(length < 256);
    }
    BincodeBytesRef(const char* str): BincodeBytesRef(str, strlen(str)) {}

    const char* data() const {
        return _data;
    }

    uint8_t size() const {
        return _length;
    }

    bool operator==(const BincodeBytesRef& rhs) const {
        if (size() != rhs.size()) {
            return false;
        }
        return strncmp(data(), rhs.data(), size()) == 0;
    }
};

std::ostream& operator<<(std::ostream& out, const BincodeBytesRef& x);

// Owned strings of at most 255 length. We could get away with references
// to existing strings most of the times, but considering how much allocation
// happens anyway it's not worth the hassle.
struct BincodeBytes {
private:
    // We store the length in the most significant byte of the pointer.
    // If the length is < 8, then the string is in the remaining 7 bytes.
    // Otherwise, the rest of the bytes are to be interpreted as a pointer
    // to the data.
    uintptr_t _data;
    static_assert(sizeof(uintptr_t) == 8);
    static_assert(std::endian::native == std::endian::little);
public:
    static constexpr uint16_t STATIC_SIZE = 1; // length

    BincodeBytes(): _data(0) {}
    BincodeBytes(const char* data, size_t length): _data(0) { copy(data, length); }
    BincodeBytes(const rocksdb::Slice& slice): BincodeBytes(slice.data(), slice.size()) {}
    BincodeBytes(const std::string& str): BincodeBytes(str.data(), str.size()) {}
    BincodeBytes(const BincodeBytesRef& ref): BincodeBytes(ref.data(), ref.size()) {}
    BincodeBytes(const char* str): BincodeBytes(str, strlen(str)) {}

    void clear() {
        if (size() >= 8) {
            ::free(data());
        }
        _data = 0;
    }

    void copy(const char* data, size_t length) {
        clear();
        ALWAYS_ASSERT(length < 256);
        _data = (uintptr_t)length << (64-8);
        if (length < 8) {
            memcpy(&_data, data, length);
        } else {
            char* buf = (char*)malloc(length);
            ALWAYS_ASSERT(buf != nullptr);
            ALWAYS_ASSERT(((uintptr_t)buf >> (64-8)) == 0);
            memcpy(buf, data, length);
            _data |= (uintptr_t)buf;
        }
    }

    ~BincodeBytes() {
        clear();
    }

    uint8_t size() const {
        return _data >> (64-8);
    }

    char* data() {
        if (size() < 8) {
            return (char*)&_data;
        } else {
            return (char*)(_data & ~(0xFFull << (64-8)));
        }
    }

    const char* data() const {
        return (const char*)((BincodeBytes*)this)->data();
    }

    BincodeBytes(const BincodeBytes& bs): BincodeBytes(bs.data(), bs.size()) {}

    BincodeBytes& operator=(const BincodeBytes& other) {
        if (this == &other) {
            return *this;
        }
        copy(other.data(), other.size());
        return *this;
    }

    BincodeBytes(BincodeBytes&& other): _data(other._data) {
        other._data = 0;
    }

    size_t packedSize() const {
        return 1 + size();
    }

    bool operator==(const BincodeBytes& rhs) const {
        if (size() != rhs.size()) {
            return false;
        }
        return strncmp(data(), rhs.data(), size()) == 0;
    }

    BincodeBytesRef ref() const {
        return BincodeBytesRef(data(), size());
    }
};

std::ostream& operator<<(std::ostream& out, const BincodeBytes& x);

template<typename A>
struct BincodeList {
    std::vector<A> els;

    static constexpr uint16_t STATIC_SIZE = 2; // length

    void clear() { els.resize(0); }

    size_t packedSize() const {
        size_t sz = 2;
        if constexpr (std::is_integral_v<A> || std::is_enum_v<A>) {
            sz += els.size()*sizeof(A);
        } else {
            for (const auto& el: els) {
                sz += el.packedSize();
            }
        }
        return sz;
    }

    bool operator==(const BincodeList<A>& rhs) const {
        if (els.size() != rhs.els.size()) {
            return false;
        }
        for (int i = 0; i < els.size(); i++) {
            if (els[i] != rhs.els[i]) {
                return false;
            }
        }
        return true;
    }
};

template<typename A>
static std::ostream& operator<<(std::ostream& out, const BincodeList<A>& x) {
    out << "[";
    for (int i = 0; i < x.els.size(); i++) {
        if (i > 0) {
            out << ", ";
        }
        out << x.els[i];
    }
    out << "]";
    return out;
}

template<uint16_t SZ>
struct BincodeFixedBytes {
    std::array<uint8_t, SZ> data;
    static constexpr uint16_t STATIC_SIZE = SZ;

    BincodeFixedBytes() { clear(); }
    BincodeFixedBytes(const char* data, size_t length) { copy(data, length); }
    BincodeFixedBytes(const rocksdb::Slice& slice): BincodeFixedBytes(slice.data(), slice.size()) {}
    BincodeFixedBytes(const BincodeBytesRef& ref): BincodeFixedBytes(ref.data(), ref.size()) {}
    BincodeFixedBytes(const char* str): BincodeFixedBytes(str, strlen(str)) {}
    BincodeFixedBytes(const std::array<uint8_t, SZ>& data_): data(data_) {}

    void clear() { memset(data.data(), 0, SZ); }

    void copy(const char* data_, size_t length) {
        clear();
        ALWAYS_ASSERT(length < SZ);
        memcpy(data.data(), data_, length);
    }

    constexpr size_t packedSize() const {
        return SZ;
    }

    bool operator==(const BincodeFixedBytes<SZ>& rhs) const {
        return data == rhs.data;
    }
};

template<uint16_t SZ>
static std::ostream& operator<<(std::ostream& out, const BincodeFixedBytes<SZ>& x) {
    out << SZ << "[";
    const uint8_t* data = (const uint8_t*)x.data.data();
    for (int i = 0; i < SZ; i++) {
        if (i > 0) {
            out << " ";
        }
        out << (int)data[i];
    }
    out << "]";
    return out;

    /*
    // Unlike operator<< for BincodeBytes, we always escape here,
    // since these are basically never ASCII strings.
    out << "b" << SZ << "\"";
    const char cfill = out.fill();
    out << std::hex << std::setfill('0');
    for (int i = 0; i < SZ; i++) {
        out << "\\x" << std::setw(2) << ((int)x.data[i]);
    }
    out << std::setfill(cfill) << std::dec;
    out << "\"";
    return out;
    */
}

#define BINCODE_EXCEPTION(...) BincodeException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), VALIDATE_FORMAT(__VA_ARGS__))

class BincodeException : public AbstractException {
public:
    template <typename TFmt, typename ... Args>
    BincodeException(int line, const char *file, const char *function, TFmt fmt, Args ... args) {
        std::stringstream ss;
        ss << "BincodeException(" << file << "@" << line << " in " << function << "):\n";
        format_pack(ss, fmt, args...);
        _msg = ss.str();
    }
    virtual const char *what() const noexcept override;
private:
    std::string _msg;
};

inline uint8_t varU61Size(uint64_t x) {
    uint64_t bits = 64 - std::countl_zero(x);
    ALWAYS_ASSERT(bits < 62, "uint64_t too large for packVarU61: %s", x);
    uint64_t neededBytes = ((bits + 3) + 8 - 1) / 8;
    return neededBytes;
}

struct BincodeBuf {
    uint8_t* data;
    uint8_t* cursor;
    uint8_t* end;

    BincodeBuf() = delete;
    BincodeBuf(char* buf, size_t len): data((uint8_t*)buf), cursor((uint8_t*)buf), end((uint8_t*)buf+len) {}
    BincodeBuf(std::string& str): BincodeBuf(str.data(), str.size()) {}

    size_t len() const {
        return cursor - data;
    }

    size_t remaining() const {
        return end - cursor;
    }

    void ensureFinished() const {
        ALWAYS_ASSERT(remaining() == 0);
    }

    void ensureSizeOrPanic(size_t sz) const {
        ALWAYS_ASSERT(remaining() >= sz, "expecting at least %s remaining bytes, got %s", sz, remaining());
    }

    template<typename A>
    void packScalar(A x) {
        static_assert(std::is_integral_v<A> || std::is_enum_v<A>);
        static_assert(std::endian::native == std::endian::little);
        ensureSizeOrPanic(sizeof(A));
        memcpy(cursor, &x, sizeof(x));
        cursor += sizeof(A);
    }

    template<size_t SZ>
    void packFixedBytes(const BincodeFixedBytes<SZ>& x) {
        ensureSizeOrPanic(SZ);
        memcpy(cursor, &x.data[0], SZ);
        cursor += SZ;
    }

    void packBytes(const BincodeBytes& x) {
        ensureSizeOrPanic(1+x.size());
        packScalar<uint8_t>(x.size());
        memcpy(cursor, x.data(), x.size());
        cursor += x.size();
    }

    template<typename A>
    void packList(const BincodeList<A>& xs) {
        ALWAYS_ASSERT(xs.els.size() < (1<<16));
        packScalar<uint16_t>(xs.els.size());
        if (unlikely(xs.els.size() == 0)) {
            return;
        }
        // If it's a number of some sorts, just memcpy it
        if constexpr (std::is_integral_v<A> || std::is_enum_v<A>) {
            static_assert(std::endian::native == std::endian::little);
            size_t sz = sizeof(A)*xs.els.size();
            ensureSizeOrPanic(sz);
            memcpy(cursor, xs.els.data(), sz);
            cursor += sz;
        } else {
            for (const auto& x: xs.els) {
                x.pack(*this);
            }
        }
    }

    template<typename A>
    A unpackScalar() {
        static_assert(std::is_integral_v<A> || std::is_enum_v<A>);
        static_assert(std::endian::native == std::endian::little);
        if (unlikely(remaining() < sizeof(A))) {
            throw BINCODE_EXCEPTION("not enough bytes to unpack scalar (need %s, got %s)", sizeof(A), remaining());
        }
        A x;
        memcpy(&x, cursor, sizeof(A));
        cursor += sizeof(A);
        return x;
    }

    template<size_t SZ>
    void unpackFixedBytes(BincodeFixedBytes<SZ>& x) {
        if (unlikely(remaining() < SZ)) {
            throw BINCODE_EXCEPTION("not enough bytes to unpack fixed bytes (need %s, got %s)", SZ, remaining());
        }
        memcpy(&x.data[0], cursor, SZ);
        cursor += SZ;
    }

    void unpackBytes(BincodeBytes& x) {
        size_t len = unpackScalar<uint8_t>();
        if (unlikely(remaining() < len)) {
            throw BINCODE_EXCEPTION("not enough bytes to unpack bytes (need %s, got %s)", (int)len, remaining());
        }
        x.copy((char*)cursor, len);
        cursor += len;
    }

    template<typename A>
    void unpackList(BincodeList<A>& xs) {
        xs.els.resize(unpackScalar<uint16_t>());
        if (unlikely(xs.els.size() == 0)) {
            return;
        }
        // If it's a number of some sorts, just memcpy it
        if constexpr (std::is_integral_v<A> || std::is_enum_v<A>) {
            static_assert(std::endian::native == std::endian::little);
            size_t sz = sizeof(A)*xs.els.size();
            if (unlikely(remaining() < sz)) {
                throw BINCODE_EXCEPTION("not enough bytes to unpack scalars (need %s, got %s)", sz, remaining());
            }
            memcpy(xs.els.data(), cursor, sz);
            cursor += sz;
        } else {
            for (int i = 0; i < xs.els.size(); i++) {
                xs.els[i].unpack(*this);
            }
        }
    }

    rocksdb::Slice slice() const {
        return rocksdb::Slice((const char*)data, len());
    }
};

// Utility class for serialization of Requests/Responses in all protocols
template<uint32_t P, typename R, bool CheckBuffFullyUsed = true>
class ProtocolMessage {
public:
    uint64_t id;
    R body;

    ProtocolMessage(): id(0) { body.clear(); }
    ProtocolMessage(const ProtocolMessage&) = delete;
    ProtocolMessage(ProtocolMessage&&) = default;
    ProtocolMessage& operator=(ProtocolMessage&&) = default;

    static constexpr uint16_t STATIC_SIZE = R::STATIC_SIZE + sizeof(P) + sizeof(id);

    size_t packedSize() const {
        return sizeof(P) + sizeof(id) + body.packedSize();
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar(P);
        buf.packScalar(id);
        body.pack(buf);
    }

    void unpack(BincodeBuf& buf) {
        uint32_t version = buf.unpackScalar<uint32_t>();
        if (version != P) {
            throw BINCODE_EXCEPTION("bad protocol version %s, expected %s", version, P);
        }
        id = buf.unpackScalar<uint64_t>();
        body.unpack(buf);
        if (CheckBuffFullyUsed) {
            if (unlikely(buf.remaining() != 0)) {
                throw BINCODE_EXCEPTION("Buf not empty after deserializing message with protocol %s, kind %s", P, body.kind());
            }
        }
    }
};

template<uint32_t P, typename R, bool CheckBuffFullyUsed = true>
class SignedProtocolMessage : public ProtocolMessage<P, R, false> {
public:

    static constexpr uint16_t STATIC_SIZE = BincodeFixedBytes<8>::STATIC_SIZE + ProtocolMessage<P, R, false>::STATIC_SIZE;
    size_t packedSize() const {
        return 8 + ProtocolMessage<P, R, false>::packedSize();
    }

    void pack(BincodeBuf& buf, const AES128Key& key) const {
        auto begin = buf.cursor;
        ProtocolMessage<P, R, false>::pack(buf);
        auto end = buf.cursor;
        BincodeFixedBytes<8> mac = cbcmac(key, begin, end - begin);
        buf.packFixedBytes<8>(mac);
    }

    void unpack(BincodeBuf& buf, const AES128Key& key) {
        auto begin = buf.cursor;
        ProtocolMessage<P, R, false>::unpack(buf);
        auto end = buf.cursor;
        BincodeFixedBytes<8> expectedMac = cbcmac(key, begin, end - begin);
        BincodeFixedBytes<8> receivedMac;
        buf.unpackFixedBytes<8>(receivedMac);
        if (expectedMac != receivedMac) {
            throw BINCODE_EXCEPTION("Signature not matching when deserializing signed message with protocol %s, kind %s", P, ProtocolMessage<P, R, false>::body.kind());
        }
        if (CheckBuffFullyUsed) {
            if (unlikely(buf.remaining() != 0)) {
                throw BINCODE_EXCEPTION("Buf not empty after deserializing signed message with protocol %s, kind %s", P, ProtocolMessage<P, R, false>::body.kind());
            }
        }
    }
private:
    void pack(BincodeBuf&) const { ALWAYS_ASSERT(false, "Called unsigned pack on signed message type"); }
    void unpack(BincodeBuf& buf) { ALWAYS_ASSERT(false, "Called unsigned unpack on signed message type"); }
};

template<uint32_t P, typename R, bool CheckBuffFullyUsed>
std::ostream& operator<<(std::ostream& out, const ProtocolMessage<P,R, CheckBuffFullyUsed>& msg) {
    return out << msg.id << " : " << msg.body;
}

template<uint32_t P, typename R, bool CheckBuffFullyUsed>
std::ostream& operator<<(std::ostream& out, const SignedProtocolMessage<P,R, CheckBuffFullyUsed>& msg) {
    return out << msg.id << " : " << msg.body;
}

constexpr size_t DEFAULT_UDP_MTU = 1472; // 1500 - IP header - ICMP header
constexpr size_t MAX_UDP_MTU = 8972;     // 9000 - IP header - ICMP header

constexpr uint8_t BLOCK_SERVICE_STALE          = 1u;
constexpr uint8_t BLOCK_SERVICE_NO_READ        = 1u<<1;
constexpr uint8_t BLOCK_SERVICE_NO_WRITE       = 1u<<2;
constexpr uint8_t BLOCK_SERVICE_DECOMMISSIONED = 1u<<3;

constexpr std::array<uint8_t, 4> BLOCK_SERVICE_FLAGS = {BLOCK_SERVICE_STALE, BLOCK_SERVICE_NO_READ, BLOCK_SERVICE_NO_WRITE, BLOCK_SERVICE_DECOMMISSIONED};

constexpr uint8_t BLOCK_SERVICE_DONT_READ  = BLOCK_SERVICE_STALE | BLOCK_SERVICE_NO_READ  | BLOCK_SERVICE_DECOMMISSIONED;
constexpr uint8_t BLOCK_SERVICE_DONT_WRITE = BLOCK_SERVICE_STALE | BLOCK_SERVICE_NO_WRITE | BLOCK_SERVICE_DECOMMISSIONED;

constexpr uint8_t FULL_READ_DIR_CURRENT = 1 << 0;
constexpr uint8_t FULL_READ_DIR_BACKWARDS = 1 << 1;
constexpr uint8_t FULL_READ_DIR_SAME_NAME = 1 << 2;
