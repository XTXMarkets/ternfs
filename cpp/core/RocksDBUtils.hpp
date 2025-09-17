// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <memory>
#include <rocksdb/db.h>
#include <rocksdb/statistics.h>
#include <rocksdb/utilities/transaction.h>

#include "Assert.hpp"
#include "Bincode.hpp"
#include "Msgs.hpp"
#include "Env.hpp"

#define ROCKS_DB_CHECKED_MSG(status, ...) \
    do { \
        if (unlikely(!status.ok())) { \
            throw RocksDBException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), status, VALIDATE_FORMAT(__VA_ARGS__)); \
        } \
    } while (false)

#define ROCKS_DB_CHECKED(status) \
    do { \
        if (unlikely(!status.ok())) { \
            throw RocksDBException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), status); \
        } \
    } while (false)

class RocksDBException : public AbstractException {
public:
    template <typename ... Args>
    RocksDBException(int line, const char *file, const char *function, rocksdb::Status status, const char *fmt, Args ... args) {
        std::stringstream ss;
        ss << "RocksDBException(" << file << "@" << line << ", " << status.ToString() << "):\n";
        format_pack(ss, fmt, args...);
        _msg = ss.str();
    }

    RocksDBException(int line, const char *file, const char *function, rocksdb::Status status) {
        std::stringstream ss;
        ss << "RocksDBException(" << file << "@" << line << ", " << status.ToString() << ")";
        _msg = ss.str();
    }

    virtual const char *what() const throw() override {
        return _msg.c_str();
    };
private:
    std::string _msg;
};

template<typename T>
struct StaticValue {
private:
    static_assert(sizeof(T) == sizeof(char*));
    static_assert(std::is_same_v<decltype(((T*)nullptr)->_data), char*>);
    std::array<char, T::MAX_SIZE> _data;
public:
    StaticValue(const StaticValue& other) {
        memcpy(_data.data(), other._data.data(), T::MAX_SIZE);
    }

    StaticValue(const T other) {
        memset(_data.data(), 0, T::MAX_SIZE);
        memcpy(_data.data(), other._data, other.size());
    }

    StaticValue& operator=(const StaticValue& other) {
        if (this == &other) { return *this; }
        memcpy(_data.data(), other._data.data(), T::MAX_SIZE);
        return *this;
    }

    StaticValue() = default;

    rocksdb::Slice toSlice() const {
        return rocksdb::Slice(&_data[0], operator()().size());
    }

    T operator()() {
        T val;
        val._data = &_data[0];
        return val;
    }

    const T operator()() const {
        T val;
        val._data = (char*)&_data[0];
        return val;
    }
};

template<typename T>
struct ExternalValue {
private:
    static_assert(sizeof(T) == sizeof(char*));
    static_assert(std::is_same_v<decltype(((T*)nullptr)->_data), char*>);
    T _val;
public:
    ExternalValue() {
        _val._data = nullptr;
    }
    ExternalValue(char* data, size_t size) {
        _val._data = data;
        _val.checkSize(size);
    }
    ExternalValue(std::string& s): ExternalValue(s.data(), s.size()) {}

    static const ExternalValue<T> FromSlice(const rocksdb::Slice& slice) {
        return ExternalValue((char*)slice.data(), slice.size());
    }

    // TODO make this have some dangerous name, since it's easy to make mistakes
    // when manipulating external values
    T operator()() {
        return _val;
    }

    const T operator()() const {
        return _val;
    }

    rocksdb::Slice toSlice() {
        return rocksdb::Slice(_val._data, _val.size());
    }
};

template<typename T>
struct OwnedValue {
    static_assert(sizeof(T) == sizeof(char*));
    static_assert(std::is_same_v<decltype(((T*)nullptr)->_data), char*>);
    T _val;
public:
    OwnedValue() = delete;
    OwnedValue<T>& operator=(const OwnedValue<T>&) = delete;

    template<typename ...Args>
    OwnedValue(Args&&... args) {
        size_t sz = T::calcSize(std::forward<Args>(args)...);
        _val._data = (char*)malloc(sz);
        ALWAYS_ASSERT(_val._data);
        _val.afterAlloc(std::forward<Args>(args)...);
        ALWAYS_ASSERT(_val.size() == sz);
    }

    ~OwnedValue() {
        free(_val._data);
    }

    rocksdb::Slice toSlice() const {
        return rocksdb::Slice(_val._data, _val.size());
    }

    T operator()() {
        return _val;
    }
};

// We use byteswap to go from LE to BE, and vice-versa.
static_assert(std::endian::native == std::endian::little);

#define LE_VAL(type, name, setName, offset) \
    type name() const { \
        type x; \
        memcpy(&x, _data+offset, sizeof(x)); \
        return x; \
    } \
    void setName(type x) { \
        memcpy(_data+offset, &x, sizeof(x)); \
    }

#define BYTES_VAL(name, setName, offset) \
    BincodeBytesRef name() const { \
        return BincodeBytesRef((const char*)(_data+offset+1), (uint8_t)(int)*(_data+offset)); \
    } \
    void setName(const BincodeBytesRef& bytes) { \
        *(_data+offset) = (char)(int)bytes.size(); \
        memcpy(_data+offset+1, bytes.data(), bytes.size()); \
    }

// Actually needed to avoid confusing cpp with commas 🤪
template<size_t sz>
using FBytesArr = std::array<uint8_t, sz>;

#define FBYTES_VAL(sz, getName, setName, offset) \
    FBytesArr<sz> getName() const { \
        FBytesArr<sz> bs; \
        memcpy(bs.data(), _data+offset, sz); \
        return bs; \
    } \
    void setName(const FBytesArr<sz>& bs) { \
        memcpy(_data+offset, bs.data(), sz); \
    }

template<size_t sz>
struct FieldToBE;

template<>
struct FieldToBE<8> {
    using Scalar = uint64_t;
    static inline Scalar bswap(Scalar x) {
        return __builtin_bswap64(x);
    }
};

template<>
struct FieldToBE<4> {
    using Scalar = uint32_t;
    static inline Scalar bswap(Scalar x) {
        return __builtin_bswap32(x);
    }
};

template<>
struct FieldToBE<2> {
    using Scalar = uint16_t;
    static inline Scalar bswap(Scalar x) {
        return __builtin_bswap16(x);
    }
};

template<>
struct FieldToBE<1> {
    using Scalar = uint8_t;
    static inline Scalar bswap(Scalar x) { return x; }
};


#define BE_VAL(type, name, setName, offset) \
    type name() const { \
        FieldToBE<sizeof(type)>::Scalar x; \
        memcpy(&x, _data+offset, sizeof(x)); \
        x = FieldToBE<sizeof(type)>::bswap(x); /* BE -> LE */ \
        type v; \
        memcpy(&v, (char*)&x, sizeof(x)); \
        return v; \
    } \
    void setName(type v) { \
        FieldToBE<sizeof(type)>::Scalar x; \
        memcpy(&x, (char*)&v, sizeof(type)); \
        x = FieldToBE<sizeof(type)>::bswap(x); /* LE -> BE */ \
        memcpy(_data+offset, &x, sizeof(x)); \
    }

// See <https://www.scs.stanford.edu/~dm/blog/va-opt.html>
// for the reasoning behind this horror.

#define PARENS ()

#define EXPAND(arg) EXPAND1(EXPAND1(EXPAND1(EXPAND1(arg))))
#define EXPAND1(arg) EXPAND2(EXPAND2(EXPAND2(EXPAND2(arg))))
#define EXPAND2(arg) EXPAND3(EXPAND3(EXPAND3(EXPAND3(arg))))
#define EXPAND3(arg) arg

#define FIELDS_HELPER(offset, which, ...) \
  which##_FIELDS_AGAIN PARENS (offset __VA_OPT__(,) __VA_ARGS__)
#define FIELDS_AGAIN() FIELDS_HELPER

#define FIELDS(...) \
    char* _data; \
    EXPAND(FIELDS_HELPER(0, __VA_ARGS__))

#define END_FIELDS(offset)
#define END_FIELDS_AGAIN() END_FIELDS

#define LE_FIELDS(offset, type, name, setName, ...) \
    LE_VAL(type, name, setName, offset) \
    FIELDS_AGAIN PARENS ((offset)+sizeof(type), __VA_ARGS__)
#define LE_FIELDS_AGAIN() LE_FIELDS

#define BYTES_FIELDS(offset, name, setName, ...) \
    BYTES_VAL(name, setName, offset) \
    FIELDS_AGAIN PARENS ((offset)+1+name().size(), __VA_ARGS__)
#define BYTES_FIELDS_AGAIN() BYTES_FIELDS

#define FBYTES_FIELDS(offset, sz, name, setName, ...) \
    FBYTES_VAL(sz, name, setName, offset) \
    FIELDS_AGAIN PARENS ((offset)+sz, __VA_ARGS__)
#define FBYTES_FIELDS_AGAIN() FBYTES_FIELDS

#define BE_FIELDS(offset, type, name, setName, ...) \
    BE_VAL(type, name, setName, offset) \
    FIELDS_AGAIN PARENS ((offset)+sizeof(type), __VA_ARGS__)
#define BE_FIELDS_AGAIN() BE_FIELDS

#define EMIT_OFFSET_FIELDS(offset, name, ...) \
    static constexpr size_t name = offset; \
    FIELDS_AGAIN PARENS ((offset), __VA_ARGS__)
#define EMIT_OFFSET_FIELDS_AGAIN() EMIT_OFFSET_FIELDS

#define EMIT_SIZE_FIELDS(offset, name, ...) \
    inline size_t name() const { \
        return (offset); \
    }
#define EMIT_SIZE_FIELDS_AGAIN() EMIT_SIZE_FIELDS

// Useful for simple datatypes with a fixed, static size.
#define END_STATIC_FIELDS(offset) \
    static constexpr size_t MAX_SIZE = offset; \
    size_t size() const { return MAX_SIZE; } \
    void checkSize(size_t size) { ALWAYS_ASSERT(size == MAX_SIZE); }
#define END_STATIC_FIELDS_AGAIN() END_STATIC_FIELDS

// When we need a simple u64 value (e.g. log index)
struct U64Value {
    FIELDS(
        LE, uint64_t, u64, setU64,
        END_STATIC
    )

    static StaticValue<U64Value> Static(uint64_t x) {
        auto v = StaticValue<U64Value>();
        v().setU64(x);
        return v;
    }
};

struct I64Value {
    FIELDS(
        LE, int64_t, i64, setI64,
        END_STATIC,
    )

    static StaticValue<I64Value> Static(int64_t x) {
        auto v = StaticValue<I64Value>();
        v().setI64(x);
        return v;
    }
};

// When we need a simple u64 key (e.g. log index)
struct U64Key {
    FIELDS(
        BE, uint64_t, u64, setU64,
        END_STATIC
    )

    static StaticValue<U64Key> Static(uint64_t x) {
        auto v = StaticValue<U64Key>();
        v().setU64(x);
        return v;
    }
};

// When we need an InodeId key. We mostly do not need them to be ordered
// (and therefore BE), but it does make it a bit nicer to be able to traverse
// them like that.
struct InodeIdKey {
    FIELDS(
        BE, InodeId, id, setId,
        END_STATIC
    )

    static StaticValue<InodeIdKey> Static(InodeId id) {
        auto x = StaticValue<InodeIdKey>();
        x().setId(id);
        return x;
    }
};

template<typename A>
std::string bincodeToRocksValue(const A& v) {
    std::string buf;
    buf.resize(v.packedSize());
    BincodeBuf bbuf(buf);
    v.pack(bbuf);
    ALWAYS_ASSERT(bbuf.remaining() == 0, "expected no remaining bytes, got %s", bbuf.remaining());
    return buf;
}

template<typename A>
void bincodeFromRocksValue(const rocksdb::Slice& value, A& v) {
    BincodeBuf bbuf((char*)value.data(), value.size());
    v.unpack(bbuf);
    ALWAYS_ASSERT(bbuf.remaining() == 0);
}

struct InodeIdValue {
    FIELDS(
        LE, InodeId, id, setId,
        END_STATIC
    )

    static StaticValue<InodeIdValue> Static(InodeId id) {
        auto x = StaticValue<InodeIdValue>();
        x().setId(id);
        return x;
    }
};

std::shared_ptr<rocksdb::MergeOperator> CreateInt64AddOperator();

void rocksDBMetrics(Env& env, rocksdb::DB* db, const rocksdb::Statistics& statistics, std::unordered_map<std::string, uint64_t>& stats);
