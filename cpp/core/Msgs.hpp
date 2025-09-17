// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cstdint>
#include <ostream>

#include "Assert.hpp"
#include "Common.hpp"
#include "Bincode.hpp"

enum class InodeType : uint8_t {
    RESERVED = 0,
    DIRECTORY = 1,
    FILE = 2,
    SYMLINK = 3,
};

using Ip = BincodeFixedBytes<4>;

struct IpPort {
    Ip ip;
    uint16_t port;

    static constexpr size_t STATIC_SIZE = Ip::STATIC_SIZE + sizeof(port);

    IpPort() : ip(), port(0) {}

    bool operator==(const IpPort& rhs) const {
        return port == rhs.port && ip == rhs.ip;
    }

    void clear() {
        ip.clear();
        port = 0;
    }

    void pack(BincodeBuf& buf) const {
        buf.packFixedBytes<4>(ip);
        buf.packScalar(port);
    }

    void unpack(BincodeBuf& buf) {
        buf.unpackFixedBytes<4>(ip);
        port = buf.unpackScalar<uint16_t>();
    }

    constexpr size_t packedSize() const {
        return STATIC_SIZE;
    }

    void toSockAddrIn(struct sockaddr_in& out) const;

    static IpPort fromSockAddrIn(const struct sockaddr_in& in);
};

std::ostream& operator<<(std::ostream& out, const IpPort& addr);

struct AddrsInfo {
    std::array<IpPort, 2> addrs;

    static constexpr size_t STATIC_SIZE = IpPort::STATIC_SIZE + IpPort::STATIC_SIZE;

    AddrsInfo() {}

    bool operator==(const AddrsInfo& rhs) const {
        return addrs[0] == rhs.addrs[0] && addrs[1] == rhs.addrs[1];
    }

    bool contains(const IpPort& addr) const {
        return addrs[0] == addr || addrs[1] == addr;
    }

    void clear() {
        addrs[0].clear();
        addrs[1].clear();
    }

    void pack(BincodeBuf& buf) const {
        addrs[0].pack(buf);
        addrs[1].pack(buf);
    }

    void unpack(BincodeBuf& buf) {
        addrs[0].unpack(buf);
        addrs[1].unpack(buf);
    }

    constexpr size_t packedSize() const {
        return STATIC_SIZE;
    }

    constexpr IpPort& operator[](size_t i) { return addrs[i]; }
    constexpr const IpPort& operator[](size_t i) const { return addrs[i]; }
    constexpr size_t size() const { return addrs.size(); }
};

std::ostream& operator<<(std::ostream& out, const AddrsInfo& addrs);

struct ShardId {
    uint8_t u8;

    constexpr ShardId(): u8(0) {}

    constexpr ShardId(uint8_t id): u8(id) {}

    bool operator==(ShardId rhs) const {
        return u8 == rhs.u8;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint8_t>(u8);
    }

    void unpack(BincodeBuf& buf) {
        u8 = buf.unpackScalar<uint8_t>();
    }
    static constexpr size_t SHARD_COUNT = 256;
};

std::ostream& operator<<(std::ostream& out, ShardId shard);

struct ReplicaId {
    uint8_t u8;

    constexpr ReplicaId(): u8(0) {}

    constexpr ReplicaId(uint8_t id): u8(id) {
        ALWAYS_ASSERT(valid(), "Invalid replica id %s", int(u8));
    }

    bool operator==(ReplicaId rhs) const {
        return u8 == rhs.u8;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint8_t>(u8);
    }

    void unpack(BincodeBuf& buf) {
        u8 = buf.unpackScalar<uint8_t>();
    }

    constexpr bool valid() const {
        return u8 < 5;
    }
};

std::ostream& operator<<(std::ostream& out, ReplicaId replica);

struct ShardReplicaId {
    uint16_t u16;

    constexpr ShardReplicaId() : u16(0) {}

    constexpr ShardReplicaId(ShardId shid, ReplicaId rid): u16(((uint16_t)rid.u8 << 8) | shid.u8) {}

    bool operator==(ShardReplicaId rhs) const {
        return u16 == rhs.u16;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint16_t>(u16);
    }

    void unpack(BincodeBuf& buf) {
        u16 = buf.unpackScalar<uint16_t>();
        if (unlikely(!valid())) {
            throw BINCODE_EXCEPTION("bad ShardReplicaId %s", u16);
        }
    }

    constexpr ShardId shardId() const {
        return ShardId(u16 & 0xFF);
    }

    constexpr ReplicaId replicaId() const {
        return ReplicaId(u16 >> 8);
    }

    constexpr bool valid() const {
        return replicaId().valid();
    }
};

std::ostream& operator<<(std::ostream& out, ShardReplicaId shrid);

using LocationId = uint8_t;

struct ShardReplicaLocationKey {
    uint32_t u32;
    
    ShardReplicaLocationKey(): u32(0) {}
    explicit ShardReplicaLocationKey(uint32_t u32_) : u32(u32_) {}
    ShardReplicaLocationKey(ShardReplicaId shrid, LocationId locationId_) {
        shardReplicaId(shrid);
        locationId(locationId_);
    }

    bool operator==(const ShardReplicaLocationKey&) const = default;

    void shardReplicaId(ShardReplicaId shrid) {
        u32 = (u32 & ~SHRID_MASK) | (static_cast<uint32_t>(shrid.u16));
    }

    void locationId(LocationId locationId_) {
        u32 = (u32 & SHRID_MASK) | (static_cast<uint32_t>(locationId_) << LOCATION_SHIFT);
    }

    ShardReplicaId shardReplicaId() const {
        ShardReplicaId shrid;
        shrid.u16 = static_cast<uint16_t>(u32 & SHRID_MASK);
        return shrid;
    }

    LocationId locationId() const {
        return static_cast<LocationId>((u32 >> LOCATION_SHIFT));
    }
private:    
    static constexpr uint32_t LOCATION_SHIFT = sizeof(ShardReplicaId) * 8;
    static constexpr uint32_t SHRID_MASK = ~0 >> (sizeof(uint32_t)*8 - LOCATION_SHIFT);
};

std::ostream& operator<<(std::ostream& out, ShardReplicaLocationKey shardReplicaLocation);

template <>
struct std::hash<ShardReplicaLocationKey> {
    std::size_t operator()(const ShardReplicaLocationKey& k) const {
        return hash<uint32_t>()(k.u32);
    }
};


// 63-bit:
// TTIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIISSSSSSSS
struct InodeId {
    uint64_t u64;

    constexpr InodeId(): u64(0) {}

    static constexpr InodeId FromU64(uint64_t data) {
        InodeId x;
        x.u64 = data;
        ALWAYS_ASSERT(x.valid());
        return x;
    }

    static constexpr InodeId FromU64Unchecked(uint64_t data) {
        InodeId x;
        x.u64 = data;
        return x;
    }

    constexpr InodeId(InodeType type, ShardId shard, uint64_t id): u64(((uint64_t)type << 61) | (id << 8) | (uint64_t)shard.u8) {
        ALWAYS_ASSERT(valid());
    }

    constexpr bool valid() const {
        return null() || ((id() < (1ull<<53)-1) && (type() != InodeType::RESERVED));
    }

    constexpr InodeType type() const {
        return (InodeType)((u64 >> 61) & 0x03);
    }

    constexpr uint64_t id() const {
        return (u64 & ~(7ull << 61)) >> 8;
    }

    constexpr ShardId shard() const {
        return ShardId(u64 & 0xFF);
    }

    constexpr bool null() const {
        return u64 == 0;
    }

    bool operator==(InodeId rhs) const {
        return u64 == rhs.u64;
    }

    bool operator!=(InodeId rhs) const {
        return u64 != rhs.u64;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint64_t>(u64);
    }

    void unpack(BincodeBuf& buf) {
        u64 = buf.unpackScalar<uint64_t>();
        if (unlikely(!valid())) {
            throw BINCODE_EXCEPTION("bad InodeId %s", u64);
        }
    }

    size_t packedSize() const {
        return sizeof(*this);
    }
};

template<>
struct std::hash<InodeId> {
    std::size_t operator()(InodeId id) const noexcept {
        return std::hash<uint64_t>{}(id.u64);
    }
};

std::ostream& operator<<(std::ostream& out, InodeId id);

constexpr InodeId NULL_INODE_ID = InodeId::FromU64(0);
constexpr InodeId ROOT_DIR_INODE_ID(InodeType::DIRECTORY, ShardId(0), 0);

// 63-bit:
// OTTIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIIISSSSSSSS
struct InodeIdExtra {
    uint64_t u64;

    InodeIdExtra(): u64(0) {}

    static InodeIdExtra FromU64(uint64_t data) {
        InodeIdExtra x;
        x.u64 = data;
        ALWAYS_ASSERT(x.id().valid());
        return x;
    }

    InodeIdExtra(InodeId id, bool extra) {
        u64 = id.u64 | (extra ? (1ull<<63) : 0);
    }

    InodeId id() const {
        return InodeId::FromU64(u64 & ~(1ull<<63));
    }

    bool extra() const {
        return u64 >> 63;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint64_t>(u64);
    }

    void unpack(BincodeBuf& buf) {
        u64 = buf.unpackScalar<uint64_t>();
        if (unlikely(!id().valid())) {
            throw BINCODE_EXCEPTION("bad OwnedInodeId %s", u64);
        }
    }

    bool operator==(InodeIdExtra rhs) const {
        return u64 == rhs.u64;
    }
};

std::ostream& operator<<(std::ostream& out, InodeIdExtra id);

struct Parity {
    uint8_t u8;

    constexpr Parity(): u8(0) {}

    explicit constexpr Parity(uint8_t data): u8(data) {}

    constexpr Parity(uint8_t dataBlocks, uint8_t parityBlocks): u8(dataBlocks | (parityBlocks << 4)) {
        ALWAYS_ASSERT(dataBlocks != 0 && dataBlocks < 16);
        ALWAYS_ASSERT(parityBlocks < 16);
    }

    constexpr uint8_t dataBlocks() const {
        return u8 & 0x0F;
    }

    constexpr uint8_t parityBlocks() const {
        return u8 >> 4;
    }

    constexpr uint8_t blocks() const {
        return dataBlocks()+parityBlocks();
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint8_t>(u8);
    }

    void unpack(BincodeBuf& buf) {
        u8 = buf.unpackScalar<uint8_t>();
    }

    bool operator==(Parity rhs) const {
        return u8 == rhs.u8;
    }
};

std::ostream& operator<<(std::ostream& out, Parity parity);

constexpr Parity NO_PARITY(0);

constexpr uint8_t EMPTY_STORAGE = 0;
constexpr uint8_t INLINE_STORAGE = 1;
constexpr uint8_t HDD_STORAGE = 2;
constexpr uint8_t FLASH_STORAGE = 3;

uint8_t storageClassByName(const char* name);

struct Crc {
    uint32_t u32;

    Crc() : u32(0) {}
    Crc(uint32_t x) : u32(x) {}

    void pack(BincodeBuf& buf) const {
        buf.packScalar(u32);
    }

    void unpack(BincodeBuf& buf) {
        u32 = buf.unpackScalar<uint32_t>();
    }

    size_t packedSize() const {
        return sizeof(u32);
    }

    bool operator==(Crc other) const {
        return u32 == other.u32;
    }
};

std::ostream& operator<<(std::ostream& out, Crc crc);

struct BlockServiceId {
    uint64_t u64;

    BlockServiceId(): u64(0) {}
    BlockServiceId(uint64_t x): u64(x) {}

    void pack(BincodeBuf& buf) const {
        buf.packScalar(u64);
    }

    void unpack(BincodeBuf& buf) {
        u64 = buf.unpackScalar<uint64_t>();
    }

    size_t packedSize() const {
        return sizeof(u64);
    }

    bool operator==(BlockServiceId other) const {
        return u64 == other.u64;
    }
};

template <>
struct std::hash<BlockServiceId> {
    std::size_t operator()(const BlockServiceId& k) const {
        return hash<uint64_t>()(k.u64);
    }
};

std::ostream& operator<<(std::ostream& out, BlockServiceId crc);

enum class BlockServiceFlags : uint8_t {
    EMPTY          = 0x0,
	STALE          = 0x1,
	NO_READ        = 0x2,
	NO_WRITE       = 0x4,
	DECOMMISSIONED = 0x8,
};

static inline BlockServiceFlags operator&(BlockServiceFlags l, BlockServiceFlags r) {
    return (BlockServiceFlags) ((uint8_t)l & (uint8_t) r);
}
static inline BlockServiceFlags operator|(BlockServiceFlags l, BlockServiceFlags r) {
    return (BlockServiceFlags) ((uint8_t)l | (uint8_t) r);
}
static inline BlockServiceFlags operator^(BlockServiceFlags l, BlockServiceFlags r) {
    return (BlockServiceFlags) ((uint8_t)l ^ (uint8_t) r);
}
static inline BlockServiceFlags operator~(BlockServiceFlags b) {
    return (BlockServiceFlags) (~(uint8_t)b);
}

static inline bool isWritable(BlockServiceFlags flags) {
    return (flags & (BlockServiceFlags::STALE | BlockServiceFlags::NO_WRITE | BlockServiceFlags::DECOMMISSIONED)) == BlockServiceFlags::EMPTY;
}

static inline bool isReadable(BlockServiceFlags flags) {
    return (flags & (BlockServiceFlags::STALE | BlockServiceFlags::NO_READ | BlockServiceFlags::DECOMMISSIONED)) == BlockServiceFlags::EMPTY;
}

std::ostream& operator<<(std::ostream& out, BlockServiceFlags flags);


// we reserve 3 bits so that we can fit ReplicaId in LeaderToken
struct LogIdx {
    static constexpr size_t STATIC_SIZE = sizeof(uint64_t);
    uint64_t u64;

    constexpr LogIdx(): u64(0) {}

    constexpr LogIdx(uint64_t idx): u64(idx) {
        ALWAYS_ASSERT(valid());
    }

    LogIdx operator+(uint64_t offset) const {
        return u64 + offset;
    }

    LogIdx& operator++() {
        ++u64;
        return *this;
    }

    bool operator==(LogIdx rhs) const {
        return u64 == rhs.u64;
    }

    bool operator<(LogIdx other) const {
        return u64 < other.u64;
    }

    bool operator<=(LogIdx other) const {
        return u64 <= other.u64;
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint64_t>(u64);
    }

    void unpack(BincodeBuf& buf) {
        u64 = buf.unpackScalar<uint64_t>();
    }

    constexpr size_t packedSize() const {
        return STATIC_SIZE;
    }

    constexpr bool valid() const {
        return u64 < 0x2000000000000000ull;
    }
};

constexpr LogIdx MAX_LOG_IDX = LogIdx(0xffffffffffffffffull >> 3);

std::ostream& operator<<(std::ostream& out, LogIdx idx);

struct LeaderToken {
    uint64_t u64;


    constexpr LeaderToken(): u64(0) {}

    constexpr LeaderToken(ReplicaId replicaId, LogIdx idx): u64(idx.u64 << 3 | replicaId.u8) {
        ALWAYS_ASSERT(replicaId.valid() && idx.valid());
    }

    bool operator==(LeaderToken rhs) const {
        return u64 == rhs.u64;
    }

    constexpr bool operator<(LeaderToken rhs) const {
        return u64 < rhs.u64;
    }

    constexpr ReplicaId replica() const {
        return ReplicaId(u64 & 0x7);
    }

    constexpr LogIdx idx() const {
        return LogIdx(u64 >> 3);
    }

    constexpr bool valid() const {
        // we don't need to check LogIdx is valid as it any value is valid
        return replica().valid();
    }

    void pack(BincodeBuf& buf) const {
        buf.packScalar<uint64_t>(u64);
    }

    void unpack(BincodeBuf& buf) {
        u64 = buf.unpackScalar<uint64_t>();
    }
};

std::ostream& operator<<(std::ostream& out, LeaderToken token);

constexpr bool isPrivilegedRequestKind(uint8_t kind) {
    return kind & 0x80;
}

static constexpr uint8_t SNAPSHOT_POLICY_TAG = 1;
static constexpr uint8_t SPAN_POLICY_TAG = 2;
static constexpr uint8_t BLOCK_POLICY_TAG = 3;
static constexpr uint8_t STRIPE_POLICY_TAG = 4;
static std::array<uint8_t, 4> REQUIRED_DIR_INFO_TAGS = {SNAPSHOT_POLICY_TAG, SPAN_POLICY_TAG, BLOCK_POLICY_TAG, STRIPE_POLICY_TAG};

static constexpr uint8_t DEFAULT_LOCATION = 0;
static constexpr uint8_t INVALID_LOCATION = 255;
