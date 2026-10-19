// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#include <cstring>
#include <filesystem>
#include <memory>
#include <filesystem>
#include <rocksdb/db.h>
#include <sstream>
#include <unistd.h>


#include "Bincode.hpp"
#include "Protocol.hpp"
#include "ShardDB.hpp"
#include "ShardDBData.hpp"
#include "Crypto.hpp"
#include "crc32c.h"
#include "RocksDBUtils.hpp"
#include "SharedRocksDB.hpp"
#include "Time.hpp"
#include "CDCKey.hpp"
#include "Random.hpp"

#define DOCTEST_CONFIG_IMPLEMENT_WITH_MAIN
#include "doctest.h"

REGISTER_EXCEPTION_TRANSLATOR(AbstractException& ex) {
    std::stringstream ss;
    // Before, we had stack traces and this was useful, now a bit less
    ss << std::endl << ex.what() << std::endl;
    return doctest::String(ss.str().c_str());
}


template<typename A>
static void bincodeTestScalar() {
    int bits = sizeof(A) * 8;
    char buf[8];
    for (int i = 0; i < bits; i++) {
        A x = (A)1 << i;
        BincodeBuf bbuf(buf, sizeof(buf));
        bbuf.packScalar<A>(x);
        bbuf = BincodeBuf(buf, sizeof(buf));
        CHECK(x == bbuf.unpackScalar<A>());
    }
}

TEST_CASE("bincode u8") { bincodeTestScalar<uint8_t>(); }
TEST_CASE("bincode u16") { bincodeTestScalar<uint16_t>(); }
TEST_CASE("bincode u32") { bincodeTestScalar<uint32_t>(); }
TEST_CASE("bincode u64") { bincodeTestScalar<uint64_t>(); }

struct SizedBincodeValue {
    static constexpr size_t STATIC_SIZE = sizeof(uint32_t);
    static inline size_t constructed = 0;

    uint32_t value;

    SizedBincodeValue() {
        constructed++;
    }

    void unpack(BincodeBuf& buf) {
        value = buf.unpackScalar<uint32_t>();
    }
};

static_assert(bincodeMinSize<uint64_t>() == 8);
static_assert(bincodeMinSize<InodeId>() == 8);
static_assert(bincodeMinSize<Edge>() == Edge::STATIC_SIZE);
static_assert(bincodeMinSize<RegistryReqContainer>() == sizeof(RegistryMessageKind));
static_assert(bincodeMinSize<ShardLogEntryContainer>() == sizeof(ShardLogEntryKind));
static_assert(bincodeMinSize<SizedBincodeValue>() == 4);

TEST_CASE("bincode rejects trailing bytes") {
    char buf[] = {'x'};
    BincodeBuf bbuf(buf, sizeof(buf));

    CHECK_THROWS_AS(bbuf.ensureFinished(), BincodeException);
}

TEST_CASE("bincode checks list count before allocating") {
    // count = 3, but only two 4-byte elements follow
    char buf[] = {3, 0, 1, 0, 0, 0, 2, 0, 0, 0};
    BincodeBuf bbuf(buf, sizeof(buf));
    BincodeList<SizedBincodeValue> values;
    SizedBincodeValue::constructed = 0;

    CHECK_THROWS_AS(bbuf.unpackList(values), BincodeException);
    CHECK(values.els.empty());
    CHECK(SizedBincodeValue::constructed == 0);
}

TEST_CASE("bincode checks container list count before allocating") {
    char buf[] = {-1, -1};
    BincodeBuf bbuf(buf, sizeof(buf));
    BincodeList<RegistryReqContainer> values;

    CHECK_THROWS_AS(bbuf.unpackList(values), BincodeException);
    CHECK(values.els.empty());
}

TEST_CASE("bincode decodes lists") {
    char buf[] = {2, 0, 1, 0, 0, 0, 2, 0, 0, 0};
    BincodeBuf bbuf(buf, sizeof(buf));
    BincodeList<SizedBincodeValue> values;

    bbuf.unpackList(values);
    bbuf.ensureFinished();
    REQUIRE(values.els.size() == 2);
    CHECK(values.els[0].value == 1);
    CHECK(values.els[1].value == 2);
}

TEST_CASE("bincode list length prefix") {
    std::vector<char> buf(4 + 70000*8);

    BincodeList<uint64_t> narrow;
    narrow.els = {1, 2, 3};
    BincodeBuf nbuf(buf.data(), buf.size());
    nbuf.packList(narrow);
    CHECK(nbuf.len() == narrow.packedSize());
    CHECK(nbuf.len() == 2 + 3*8);

    BincodeList<uint64_t, uint32_t> wide;
    for (uint64_t i = 0; i < 70000; i++) { wide.els.push_back(i); }
    BincodeBuf wbuf(buf.data(), buf.size());
    wbuf.packList(wide);
    CHECK(wbuf.len() == wide.packedSize());
    CHECK(wbuf.len() == buf.size());
    BincodeList<uint64_t, uint32_t> wideOut;
    BincodeBuf rbuf(buf.data(), buf.size());
    rbuf.unpackList(wideOut);
    CHECK(wideOut == wide);
    CHECK(rbuf.remaining() == 0);
}

TEST_CASE("LeaderToken epoch encoding") {
    LeaderToken token(ReplicaId(3), Epoch(42));

    CHECK(token.u64 == (42ull << 3 | 3));
    CHECK(token.replica() == ReplicaId(3));
    CHECK(token.epoch() == Epoch(42));
}

TEST_CASE("BincodeBytes") {
    BincodeBytes bytes;
    CHECK(bytes.size() == 0);
    CHECK(strncmp(bytes.data(), "", bytes.size()) == 0);
    char buf[255];
    RandomGenerator rand(0);
    for (int i = 0; i < 10; i++) {
        rand.generateBytes(buf, i);
        bytes = BincodeBytes(buf, i);
        CHECK(bytes.size() == i);
        CHECK(strncmp(bytes.data(), buf, bytes.size()) == 0);
    }
}

struct TempRocksDB {
    rocksdb::DB* db;
    std::string dbDir;

    TempRocksDB() {
        dbDir = std::string("temp-rocks-db.XXXXXX");
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
        rocksdb::Options options;
        options.create_if_missing = true;
        ROCKS_DB_CHECKED(rocksdb::DB::Open(options, dbDir, &db));
    }

    rocksdb::DB* operator->() {
        return db;
    }

    ~TempRocksDB() {
        std::error_code err;
        if (std::filesystem::remove_all(std::filesystem::path(dbDir), err) < 0) {
            std::cerr << "Could not remove " << dbDir << ": " << err << std::endl;
        }
        auto closeStatus = db->Close();
        if (!closeStatus.ok()) {
            std::cerr << "Could not close db: " << closeStatus.ToString() << std::endl;
        }
        delete db;
    }
};

static bool sliceEq(const rocksdb::Slice& lhs, const rocksdb::Slice& rhs) {
    return lhs.size() == rhs.size() && (strncmp(lhs.data(), rhs.data(), lhs.size()) == 0);
}

TEST_CASE("ShardDB data") {
    TempRocksDB db;

    SUBCASE("TransientFile") {
        auto transientFileId = InodeIdKey::Static({InodeType::FILE, ShardId(), 0});
        BincodeBytes transientFileBodyNote("hello world");
        StaticValue<TransientFileBody> transientFileBody;
        transientFileBody().setVersion(0);
        transientFileBody().setFileSize(123);
        transientFileBody().setMtime({456});
        transientFileBody().setDeadline({789});
        transientFileBody().setLastSpanState(SpanState::CLEAN);
        transientFileBody().setNoteDangerous(transientFileBodyNote.ref());

        ROCKS_DB_CHECKED(
            db.db->Put({}, transientFileId.toSlice(), transientFileBody.toSlice())
        );
        std::string transientFileValue;
        ROCKS_DB_CHECKED(
            db.db->Get({}, transientFileId.toSlice(), &transientFileValue)
        );
        CHECK(sliceEq(transientFileBody.toSlice(), {transientFileValue}));

        ExternalValue<TransientFileBody> readTransientFileBody(transientFileValue);

        CHECK(transientFileBody().mtime() == readTransientFileBody().mtime());
        CHECK(transientFileBody().fileSize() == readTransientFileBody().fileSize());
        CHECK(transientFileBody().deadline() == readTransientFileBody().deadline());
        CHECK(transientFileBody().note() == readTransientFileBody().note());
    }

    SUBCASE("File") {
        auto fileId = InodeIdKey::Static({InodeType::FILE, ShardId(), 0});
        StaticValue<FileBody> fileBody;
        fileBody().setVersion(0);
        fileBody().setMtime(123);
        fileBody().setAtime(123);
        fileBody().setFileSize(456);

        ROCKS_DB_CHECKED(db.db->Put({}, fileId.toSlice(), fileBody.toSlice()));
        std::string fileValue;
        ROCKS_DB_CHECKED(db.db->Get({}, fileId.toSlice(), &fileValue));
        CHECK(sliceEq(fileBody.toSlice(), {fileValue}));

        ExternalValue<FileBody> readFileBody(fileValue);

        CHECK(fileBody().mtime() == readFileBody().mtime());
        CHECK(fileBody().fileSize() == readFileBody().fileSize());
    }

    /*
    SUBCASE("Span (ZERO)") {
        SpanKey key(InodeId(InodeType::FILE, ShardId(), 0), 123);

        auto span = SpanBody::NewZero();
        span->size = 456;
        span->blockSize = 789;
        span->crc32 = 012;
        span->parity = Parity(0);

        ROCKS_DB_CHECKED(
            db.db->Put({}, key.toSlice(), span->toSlice())
        );
        std::string spanValue;
        ROCKS_DB_CHECKED(
            db.db->Get({}, key.toSlice(), &spanValue)
        );
        CHECK(sliceEq(span->toSlice(), {spanValue}));

        auto readSpan = SpanBody::FromSlice({spanValue});

        CHECK(readSpan->storageClass == ZERO_FILL_STORAGE);
        CHECK(readSpan->size == span->size);
        CHECK(readSpan->blockSize == span->blockSize);
        CHECK(readSpan->crc32 == span->crc32);
        CHECK(readSpan->parity == span->parity);
    }

    SUBCASE("Span (INLINE)") {
        SpanKey key(InodeId(InodeType::FILE, ShardId(), 0), 123);

        BincodeBytes body("body");
        auto span = SpanBody::NewInline(body.length);
        span->size = 456;
        span->blockSize = 789;
        span->crc32 = 012;
        span->parity = Parity(0);
        span->setInlineBody(body);

        ROCKS_DB_CHECKED(
            db.db->Put({}, key.toSlice(), span->toSlice())
        );
        std::string spanValue;
        ROCKS_DB_CHECKED(
            db.db->Get({}, key.toSlice(), &spanValue)
        );
        CHECK(sliceEq(span->toSlice(), {spanValue}));

        auto readSpan = SpanBody::FromSlice({spanValue});

        CHECK(readSpan->storageClass == INLINE_STORAGE);
        CHECK(readSpan->size == span->size);
        CHECK(readSpan->blockSize == span->blockSize);
        CHECK(readSpan->crc32 == span->crc32);
        CHECK(readSpan->parity == span->parity);
        CHECK(readSpan->inlineBody() == span->inlineBody());
    }

    SUBCASE("Span (BLOCKS)") {
        SpanKey key(InodeId(InodeType::FILE, ShardId(), 0), 456);

        Parity parity(10, 4);
        uint8_t storageClass = 2;
        BincodeBytes body("body");
        auto span = SpanBody::New(storageClass, parity);
        span->size = 456;
        span->blockSize = 789;
        span->crc32 = 012;
        for (uint64_t i = 0; i < span->parity.blocks(); i++) {
            auto& block = span->block(i);
            block.blockServiceId = i * 2;
            block.crc32 = i * 2 + 1;
        }

        ROCKS_DB_CHECKED(
            db.db->Put({}, key.toSlice(), span->toSlice())
        );
        std::string spanValue;
        ROCKS_DB_CHECKED(
            db.db->Get({}, key.toSlice(), &spanValue)
        );
        CHECK(sliceEq(span->toSlice(), {spanValue}));

        auto readSpan = SpanBody::FromSlice({spanValue});

        CHECK(readSpan->storageClass == storageClass);
        CHECK(readSpan->size == span->size);
        CHECK(readSpan->blockSize == span->blockSize);
        CHECK(readSpan->crc32 == span->crc32);
        CHECK(readSpan->parity == span->parity);
        for (uint64_t i = 0; i < span->parity.blocks(); i++) {
            const auto& lhs = readSpan->block(i);
            const auto& rhs = span->block(i);
            CHECK(lhs.blockServiceId == rhs.blockServiceId);
            CHECK(lhs.crc32 == rhs.crc32);
        }
    }

    SUBCASE("Directory") {
        InodeId id(InodeType::DIRECTORY, ShardId(3), 5);

        StaticValue<DirectoryBody> dir;
        dir().setOwnerId(ROOT_DIR_INODE_ID);
        dir().setMtime(123);
        dir().setHashMode(HashMode::XXH3_63);
        dir().setInfoInherited(true);
        dir().setInfo({});

        auto slice = dir.toSlice();

        auto dirKey = InodeIdKey::Static(id);

        ROCKS_DB_CHECKED(db.db->Put({}, dirKey.toSlice(), dir.toSlice()));
        std::string dirValue;
        ROCKS_DB_CHECKED(db.db->Get({}, dirKey.toSlice(), &dirValue));
        CHECK(sliceEq(dir.toSlice(), {dirValue}));

        ExternalValue<DirectoryBody> readDir(dirValue);

        CHECK(readDir().mtime() == dir().mtime());
        CHECK(readDir().ownerId() == dir().ownerId());
        CHECK(readDir().hashMode() == dir().hashMode());
        CHECK(readDir().infoInherited() == dir().infoInherited());
    }

    SUBCASE("Edges") {
        InodeId dirId(InodeType::DIRECTORY, ShardId(3), 5);

        BincodeBytes snapshotName("foo");
        auto snapshotKey = EdgeKey::NewSnapshot(snapshotName.length);
        snapshotKey->setDirId(dirId);
        snapshotKey->nameHash = 123;
        snapshotKey->setName(snapshotName);
        snapshotKey->setCreationTime(456);

        SnapshotEdgeBody snapshot;
        snapshot.setTargetId(InodeIdExtra(InodeId(InodeType::FILE, ShardId(4), 32), false));

        ROCKS_DB_CHECKED(
            db.db->Put({}, snapshotKey->toSlice(), snapshot.toSlice())
        );

        BincodeBytes currentName("barrrr");
        auto currentKey = EdgeKey::NewCurrent(currentName.length);
        currentKey->setDirId(dirId);
        currentKey->nameHash = 789;
        currentKey->setName(currentName);

        CurrentEdgeBody current;
        current.setTargetId(InodeIdExtra(InodeId(InodeType::FILE, ShardId(5), 33), true));
        current.setCreationTime(012);

        ROCKS_DB_CHECKED(
            db.db->Put({}, currentKey->toSlice(), current.toSlice())
        );

        std::unique_ptr<rocksdb::Iterator> it(db.db->NewIterator({}));
        // we first go to the current edge, to emulate what readdir does
        {
            auto key = EdgeKey::NewCurrent(0);
            key->setDirId(dirId);
            key->nameHash = currentKey->nameHash;
            it->Seek(key->toSlice());
        }
        CHECK(it->Valid());
        CHECK(sliceEq(it->key(), currentKey->toSlice()));
        auto key = EdgeKey::FromSlice(it->key());
        CHECK(key->current == currentKey->current);
        CHECK(key->dirId() == currentKey->dirId());
        CHECK(key->nameHash == currentKey->nameHash);
        CHECK(key->name() == currentKey->name());
        CHECK(sliceEq(it->value(), current.toSlice()));
        CurrentEdgeBody currentValue(it->value());
        CHECK(currentValue.targetId() == current.targetId());
        CHECK(currentValue.creationTime() == current.creationTime());
        // then we go backwards to the snapshot edge
        it->Prev();
        CHECK(it->Valid());
        CHECK(sliceEq(it->key(), snapshotKey->toSlice()));
        key = EdgeKey::FromSlice(it->key());
        CHECK(key->current == snapshotKey->current);
        CHECK(key->dirId() == snapshotKey->dirId());
        CHECK(key->nameHash == snapshotKey->nameHash);
        CHECK(key->name() == snapshotKey->name());
        CHECK(key->creationTime() == snapshotKey->creationTime());
        CHECK(sliceEq(it->value(), snapshot.toSlice()));
        SnapshotEdgeBody snapshotValue(it->value());
        CHECK(snapshotValue.targetId() == snapshot.targetId());
    }
    */
}

TEST_CASE("CBC MAC") {
    std::array<uint8_t, 16> userKey;
    memcpy(userKey.data(), "\x2b\x7e\x15\x16\x28\xae\xd2\xa6\xab\xf7\x15\x88\x09\xcf\x4f\x3c", sizeof(userKey));
    AES128Key key;
    expandKey(userKey, key);
    const auto test = [&key](const std::vector<uint8_t>& plaintext, const std::array<uint8_t, 8>& expectedMac) {
        CHECK(cbcmac(key, plaintext.data(), plaintext.size()) == expectedMac);
    };
    test(
        {68,235,81,75,124,255,47,151,1,176},
        {25,112,35,208,238,64,6,13}
    );
    test(
        {44,48,69,112,99,225,155,9,121,48,26,199,170,9,87,117},
        {35,130,149,17,102,117,44,174}
    );
    test(
        {174,202,179,110,252,61,96,177,72,45,146,68,134,235,251,99,104,219,39,83},
        {25,197,134,127,44,151,241,223}
    );
    test(
        {30,150,143,64,165,93,232,5,86,160,21,239,228,49,121,199,214,95,153,152,37,35,188,167,111,6,253,215,180,215,85,201},
        {85,212,198,154,164,248,20,252}
    );
    AES128Key expandedCDCKey;
    expandKey(CDCKey, expandedCDCKey);
    const auto testCDC = [&expandedCDCKey](const std::vector<uint8_t>& plaintext, const std::array<uint8_t, 8>& expectedMac) {
        CHECK(cbcmac(expandedCDCKey, plaintext.data(), plaintext.size()) == expectedMac);
    };
    testCDC(
        {30,150,143,64,165,93,232,5,86,160,21,239,228,49,121,199,214,95,153,152,37,35,188,167,111,6,253,215,180,215,85,201},
        {78, 250, 182, 234, 239, 64, 212, 140}
    );
    testCDC(
        {83,72,65,0,202,51,170,123,110,151,72,1,128,7,0,0,0,0,0,0,32,0,0,0,0,0,0,0,32,1,0,0},
        {7,78,66,131,192,124,19,0}
    );
}

struct TempShardDB {
    std::string dbDir;
    Logger logger;
    std::unique_ptr<Env> env;
    ShardId shid;
    std::unique_ptr<SharedRocksDB> sharedDB;
    std::unique_ptr<BlockServicesCacheDB> blockServicesCacheDB;
    std::unique_ptr<ShardDB> db;

    TempShardDB(LogLevel level, ShardId shid_): logger(level, STDERR_FILENO, false, false), shid(shid_) {
        dbDir = std::string("temp-shard-db.XXXXXX");
        if (mkdtemp(dbDir.data()) == nullptr) {
            throw SYSCALL_EXCEPTION("mkdtemp");
        }
        std::shared_ptr<XmonAgent> xmon;
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        initSharedDB();
        blockServicesCacheDB = std::make_unique<BlockServicesCacheDB>(logger, xmon, *sharedDB, 0_sec, 0, 0, 0);
        db = std::make_unique<ShardDB>(logger, xmon, shid, 0, DEFAULT_DEADLINE_INTERVAL, *sharedDB, *blockServicesCacheDB);
    }

    // useful to test recovery
    void restart() {
        std::shared_ptr<XmonAgent> xmon;
        db->close();
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        initSharedDB();
        blockServicesCacheDB = std::make_unique<BlockServicesCacheDB>(logger, xmon, *sharedDB, 0_sec, 0, 0, 0);
        db = std::make_unique<ShardDB>(logger, xmon, shid, 0, DEFAULT_DEADLINE_INTERVAL, *sharedDB, *blockServicesCacheDB);
    }

    ~TempShardDB() {
        std::error_code err;
        if (std::filesystem::remove_all(std::filesystem::path(dbDir), err) < 0) {
            std::cerr << "Could not remove " << dbDir << ": " << err << std::endl;
        }
    }

    std::unique_ptr<ShardDB>& operator->() {
        return db;
    }

    void initSharedDB() {
        sharedDB->registerCFDescriptors(BlockServicesCacheDB::getColumnFamilyDescriptors());
        sharedDB->registerCFDescriptors(ShardDB::getColumnFamilyDescriptors());
        rocksdb::Options rocksDBOptions;
        rocksDBOptions.create_if_missing = true;
        rocksDBOptions.create_missing_column_families = true;
        rocksDBOptions.compression = rocksdb::kLZ4Compression;
        rocksDBOptions.bottommost_compression = rocksdb::kZSTD;
        // 1000*256 = 256k open files at once, given that we currently run on a
        // single machine this is appropriate.
        rocksDBOptions.max_open_files = 1000;
        // We batch writes and flush manually.
        rocksDBOptions.manual_wal_flush = true;
        sharedDB->open(rocksDBOptions);
    }
};

#define NO_TERN_ERROR(expr) \
    do { \
        TernError err = (expr); \
        ALWAYS_ASSERT(err == TernError::NO_ERROR, #expr ", unexpected error %s", err); \
    } while(false)

#define NO_TERN_ERROR_IN_RESPONSE(resp, expr) \
    do { \
        (expr); \
        ALWAYS_ASSERT((int)((resp).kind()) != 0, #expr ", unexpected error %s", (resp).getError()); \
    } while(false)

TEST_CASE("touch file") {
    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    auto reqContainer = std::make_unique<ShardReqContainer>();
    auto respContainer = std::make_unique<ShardRespContainer>();
    auto logEntry = std::make_unique<ShardLogEntry>();
    uint64_t logEntryIndex = 0;

    InodeId id;
    BincodeFixedBytes<8> cookie;
    TernTime constructTime, linkTime;
    BincodeBytes name("filename");
    {
        auto& req = reqContainer->setConstructFile();
        req.type = (uint8_t)InodeType::FILE;
        req.note = "test note";
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        constructTime = logEntry->time;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
        db->flush(false);
        auto& resp = respContainer->getConstructFile();
        id = resp.id;
        cookie = resp.cookie;
    }
    {
        auto& req = reqContainer->setVisitTransientFiles();
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        auto& resp = respContainer->getVisitTransientFiles();
        REQUIRE(resp.nextId == NULL_INODE_ID);
        REQUIRE(resp.files.els.size() == 1);
        REQUIRE(resp.files.els[0].id == id);
        REQUIRE(resp.files.els[0].cookie == cookie);
    }
    {
        auto& req = reqContainer->setLinkFile();
        req.fileId = id;
        req.cookie = cookie;
        req.ownerId = ROOT_DIR_INODE_ID;
        req.name = name;
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        linkTime = logEntry->time;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
        db->flush(false);
    }
    {
        auto& req = reqContainer->setReadDir();
        req.dirId = ROOT_DIR_INODE_ID;
        req.startHash = 0;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        auto& resp = respContainer->getReadDir();
        REQUIRE(resp.nextHash == 0);
        REQUIRE(resp.results.els.size() == 1);
        auto& res = resp.results.els.at(0);
        REQUIRE(res.name == name);
        REQUIRE(res.targetId == id);
        REQUIRE(res.creationTime == linkTime);
    }
    {
        auto& req = reqContainer->setLookup();
        req.dirId = ROOT_DIR_INODE_ID;
        req.name = name;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        auto& resp = respContainer->getLookup();
        REQUIRE(resp.targetId == id);
    }
    {
        auto& req = reqContainer->setStatFile();
        req.id = id;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        auto& resp = respContainer->getStatFile();
        REQUIRE(resp.size == 0);
        REQUIRE(resp.mtime == linkTime);
    }
}

TEST_CASE("heartbeat entry is a no-op that advances applied time") {
    TernTime startTime(1234567890);
    _setCurrentTime(startTime);
    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    ShardRespContainer respContainer;
    ShardLogEntry logEntry;

    REQUIRE(db->lastAppliedEntryTime() == startTime);

    TernTime heartbeatTime(startTime.ns + 1000);
    logEntry.body.setHeartbeat();
    logEntry.time = heartbeatTime;
    NO_TERN_ERROR_IN_RESPONSE(respContainer, db->applyLogEntry(1, logEntry, respContainer));
    db->flush(false);

    REQUIRE(db->lastAppliedLogEntry() == 1);
    REQUIRE(db->lastAppliedEntryTime() == heartbeatTime);

    db.restart();
    REQUIRE(db->lastAppliedLogEntry() == 1);
    REQUIRE(db->lastAppliedEntryTime() == heartbeatTime);

    _setCurrentTime(0);
}

TEST_CASE("override") {
    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    auto reqContainer = std::make_unique<ShardReqContainer>();
    auto respContainer = std::make_unique<ShardRespContainer>();
    auto logEntry = std::make_unique<ShardLogEntry>();
    uint64_t logEntryIndex = 0;

    const auto createFile = [&](const char* name) -> std::tuple<InodeId, TernTime> {
        InodeId id;
        BincodeFixedBytes<8> cookie;
        {
            auto& req = reqContainer->setConstructFile();
            req.type = (uint8_t)InodeType::FILE;
            req.note = "test note";
            NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
            NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
            db->flush(false);
            auto& resp = respContainer->getConstructFile();
            id = resp.id;
            cookie = resp.cookie;
        }
        {
            auto& req = reqContainer->setVisitTransientFiles();
            NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        }
        TernTime creationTime;
        {
            auto& req = reqContainer->setLinkFile();
            req.fileId = id;
            req.cookie = cookie;
            req.ownerId = ROOT_DIR_INODE_ID;
            req.name = name;
            NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
            NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
            db->flush(false);
            creationTime = respContainer->getLinkFile().creationTime;
        }
        return {id, creationTime};
    };

    auto [foo, fooCreationTime] = createFile("foo");
    auto [bar, barCreationTime] = createFile("bar");

    {
        auto& req = reqContainer->setSameDirectoryRename();
        req.dirId = ROOT_DIR_INODE_ID;
        req.targetId = foo;
        req.oldName = "foo";
        req.oldCreationTime = fooCreationTime;
        req.newName = "bar";
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
        db->flush(false);
    }
    {
        auto& req = reqContainer->setFullReadDir();
        req.dirId = ROOT_DIR_INODE_ID;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        auto& resp = respContainer->getFullReadDir();
        REQUIRE(
            resp.results.els.size() ==
                1 + // the live "bar" edge
                1 + 1 + // the two snapshot "bar" edges
                1 // the snapshot "foo" edge
        );
    }
}

TEST_CASE("fullreaddir backwards without same_name") {
    // Regression: FullReadDirReq with BACKWARDS only (no SAME_NAME, no
    // CURRENT) used to crash the shard in _fullReadDirNormal because the
    // backwards boundary key is a *current* edge but the code still called
    // setCreationTime() on it, which asserts snapshot().
    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    auto reqContainer = std::make_unique<ShardReqContainer>();
    auto respContainer = std::make_unique<ShardRespContainer>();
    auto logEntry = std::make_unique<ShardLogEntry>();
    uint64_t logEntryIndex = 0;

    const auto createFile = [&](const char* name) -> std::tuple<InodeId, TernTime> {
        InodeId id;
        BincodeFixedBytes<8> cookie;
        {
            auto& req = reqContainer->setConstructFile();
            req.type = (uint8_t)InodeType::FILE;
            req.note = "test note";
            NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
            NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
            db->flush(false);
            auto& resp = respContainer->getConstructFile();
            id = resp.id;
            cookie = resp.cookie;
        }
        TernTime creationTime;
        {
            auto& req = reqContainer->setLinkFile();
            req.fileId = id;
            req.cookie = cookie;
            req.ownerId = ROOT_DIR_INODE_ID;
            req.name = name;
            NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
            NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
            db->flush(false);
            creationTime = respContainer->getLinkFile().creationTime;
        }
        return {id, creationTime};
    };

    // Create two files and rename "foo" -> "foo2" to leave a snapshot edge
    // behind for "foo". Enough structure that the backwards iterator has
    // something to return.
    auto [foo, fooCreationTime] = createFile("foo");
    createFile("bar");
    {
        auto& req = reqContainer->setSameDirectoryRename();
        req.dirId = ROOT_DIR_INODE_ID;
        req.targetId = foo;
        req.oldName = "foo";
        req.oldCreationTime = fooCreationTime;
        req.newName = "foo2";
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->applyLogEntry(++logEntryIndex, *logEntry, *respContainer));
        db->flush(false);
    }

    // Backwards iteration over snapshot edges only.
    {
        auto& req = reqContainer->setFullReadDir();
        req.dirId = ROOT_DIR_INODE_ID;
        req.flags = FULL_READ_DIR_BACKWARDS;
        NO_TERN_ERROR_IN_RESPONSE(*respContainer, db->read(*reqContainer, *respContainer));
        auto& resp = respContainer->getFullReadDir();
        REQUIRE(resp.results.els.size() >= 1);
        // The only snapshot edge is the now-historical "foo" rename source.
        REQUIRE(resp.results.els[0].name == "foo");
    }
}

TEST_CASE("wire request validation returns errors") {
    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    SUBCASE("fullreaddir same name requires a name") {
        ShardReqContainer reqContainer;
        ShardRespContainer respContainer;
        auto& req = reqContainer.setFullReadDir();
        req.dirId = ROOT_DIR_INODE_ID;
        req.flags = FULL_READ_DIR_SAME_NAME;

        db->read(reqContainer, respContainer);

        REQUIRE(respContainer.kind() == ShardMessageKind::ERROR);
        CHECK(respContainer.getError() == TernError::MALFORMED_REQUEST);
    }

    SUBCASE("fullreaddir current cursor cannot have a start time") {
        ShardReqContainer reqContainer;
        ShardRespContainer respContainer;
        auto& req = reqContainer.setFullReadDir();
        req.dirId = ROOT_DIR_INODE_ID;
        req.flags = FULL_READ_DIR_CURRENT;
        req.startTime = TernTime(1);

        db->read(reqContainer, respContainer);

        REQUIRE(respContainer.kind() == ShardMessageKind::ERROR);
        CHECK(respContainer.getError() == TernError::MALFORMED_REQUEST);
    }

    SUBCASE("null inode cannot be removed") {
        ShardReqContainer reqContainer;
        ShardLogEntry logEntry;
        auto& req = reqContainer.setRemoveInode();
        req.id = NULL_INODE_ID;

        CHECK(db->prepareLogEntry(reqContainer, logEntry) == TernError::MALFORMED_REQUEST);
    }

    SUBCASE("link file requires a valid name") {
        const std::vector<std::string> invalidNames{
            "",
            ".",
            "..",
            "bad/name",
            std::string("bad\0name", 8),
        };
        for (const auto& name : invalidNames) {
            ShardReqContainer reqContainer;
            ShardLogEntry logEntry;
            auto& req = reqContainer.setLinkFile();
            req.ownerId = ROOT_DIR_INODE_ID;
            req.fileId = InodeId(InodeType::FILE, ShardId(0), 1);
            req.name = name;

            CHECK(db->prepareLogEntry(reqContainer, logEntry) == TernError::BAD_NAME);
        }
    }

    SUBCASE("swap blocks requires distinct files") {
        ShardReqContainer reqContainer;
        ShardLogEntry logEntry;
        auto& req = reqContainer.setSwapBlocks();
        req.fileId1 = InodeId(InodeType::FILE, ShardId(0), 1);
        req.fileId2 = req.fileId1;

        CHECK(db->prepareLogEntry(reqContainer, logEntry) == TernError::SAME_SOURCE_AND_DESTINATION);
    }

    SUBCASE("swap spans requires distinct files") {
        ShardReqContainer reqContainer;
        ShardLogEntry logEntry;
        auto& req = reqContainer.setSwapSpans();
        req.fileId1 = InodeId(InodeType::FILE, ShardId(0), 1);
        req.fileId2 = req.fileId1;

        CHECK(db->prepareLogEntry(reqContainer, logEntry) == TernError::SAME_SOURCE_AND_DESTINATION);
    }

    SUBCASE("add span location requires distinct files") {
        ShardReqContainer reqContainer;
        ShardLogEntry logEntry;
        auto& req = reqContainer.setAddSpanLocation();
        req.fileId1 = InodeId(InodeType::FILE, ShardId(0), 1);
        req.fileId2 = req.fileId1;

        CHECK(db->prepareLogEntry(reqContainer, logEntry) == TernError::SAME_SOURCE_AND_DESTINATION);
    }
}

TEST_CASE("add span validates layout before preparing a log entry") {
    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    {
        ShardReqContainer reqContainer;
        ShardRespContainer respContainer;
        ShardLogEntry logEntry;
        auto& req = reqContainer.setConstructFile();
        req.type = (uint8_t)InodeType::FILE;

        NO_TERN_ERROR(db->prepareLogEntry(reqContainer, logEntry));
        NO_TERN_ERROR_IN_RESPONSE(respContainer, db->applyLogEntry(1, logEntry, respContainer));
        db->flush(false);
        fileId = respContainer.getConstructFile().id;
        cookie = respContainer.getConstructFile().cookie;
    }

    const auto prepareAddSpan = [&](Parity parity, uint8_t stripes, size_t crcCount) {
        ShardReqContainer reqContainer;
        ShardLogEntry logEntry;
        auto& req = reqContainer.setAddSpanInitiate();
        req.fileId = fileId;
        req.cookie = cookie;
        req.storageClass = HDD_STORAGE;
        req.parity = parity;
        req.stripes = stripes;
        req.crcs.els.resize(crcCount);
        return db->prepareLogEntry(reqContainer, logEntry);
    };

    SUBCASE("zero data and parity blocks") {
        CHECK(prepareAddSpan(Parity(0), 1, 0) == TernError::BAD_SPAN_BODY);
    }

    SUBCASE("multiple data blocks without parity") {
        CHECK(prepareAddSpan(Parity(2), 1, 2) == TernError::BAD_SPAN_BODY);
    }

    SUBCASE("too many data blocks") {
        CHECK(prepareAddSpan(Parity(0x1B), 1, 12) == TernError::BAD_SPAN_BODY);
    }

    SUBCASE("too many parity blocks") {
        CHECK(prepareAddSpan(Parity(0x71), 1, 8) == TernError::BAD_SPAN_BODY);
    }

    SUBCASE("zero stripes") {
        CHECK(prepareAddSpan(Parity(1, 1), 0, 0) == TernError::BAD_SPAN_BODY);
    }

    SUBCASE("too many stripes") {
        CHECK(prepareAddSpan(Parity(1, 1), 16, 32) == TernError::BAD_SPAN_BODY);
    }

    SUBCASE("one data block without parity is supported") {
        CHECK(prepareAddSpan(Parity(1), 1, 1) == TernError::COULD_NOT_PICK_BLOCK_SERVICES);
    }

    SUBCASE("production erasure coding configuration is supported") {
        CHECK(prepareAddSpan(Parity(10, 4), 1, 14) == TernError::COULD_NOT_PICK_BLOCK_SERVICES);
    }
}

struct BlockReplacementTest {
    struct ClockReset { ~ClockReset() { _setCurrentTime(0); } } clockReset;
    TempShardDB db{LogLevel::LOG_ERROR, ShardId(0)};
    std::unique_ptr<ShardReqContainer> req = std::make_unique<ShardReqContainer>();
    std::unique_ptr<ShardRespContainer> resp = std::make_unique<ShardRespContainer>();
    std::unique_ptr<ShardLogEntry> entry = std::make_unique<ShardLogEntry>();
    std::vector<FullBlockServiceInfo> services;
    std::array<AES128Key, 5> keys;
    std::array<uint64_t, 2> oldBlocks;
    uint64_t logIndex = 0;
    InodeId fileId;
    BincodeFixedBytes<8> cookie;
    static constexpr uint32_t BLOCK_SIZE = 4096;

    TernError prepare() {
        _setCurrentTime(ternNow() + 1_ms);
        return db->prepareLogEntry(*req, *entry);
    }

    void apply(TernError expected = TernError::NO_ERROR) {
        NO_TERN_ERROR(prepare());
        applyEntry(*entry, expected);
    }

    void applyEntry(const ShardLogEntry& logEntry, TernError expected = TernError::NO_ERROR) {
        db->applyLogEntry(++logIndex, logEntry, *resp);
        db->flush(false);
        auto actual = resp->kind() == ShardMessageKind::ERROR ? resp->getError() : TernError::NO_ERROR;
        REQUIRE(actual == expected);
    }

    BlockProof proof(char tag, BlockServiceId bs, uint64_t blockId) {
        char buf[32]{};
        BincodeBuf packed(buf, sizeof(buf));
        packed.packScalar<uint64_t>(bs.u64);
        packed.packScalar<char>(tag);
        packed.packScalar<uint64_t>(blockId);
        BlockProof result;
        result.blockId = blockId;
        result.proof.data = cbcmac(keys[bs.u64], (uint8_t*)buf, sizeof(buf));
        return result;
    }

    BlockReplacementTest(bool transient = false, bool dirtyLast = false, LocationId location = 0, bool decommission = true) {
        for (uint64_t id = 1; id <= 4; ++id) {
            auto& bs = services.emplace_back();
            bs.id = BlockServiceId(id);
            bs.locationId = location;
            bs.storageClass = HDD_STORAGE;
            bs.failureDomain.name.data[0] = id;
            bs.secretKey.data.fill(id);
            expandKey(bs.secretKey.data, keys[id]);
            bs.capacityBytes = 1ull << 30;
            bs.availableBytes = 1ull << 30;
        }
        db.blockServicesCacheDB->updateCache(services);
        req->setConstructFile().type = (uint8_t)InodeType::FILE;
        apply();
        fileId = resp->getConstructFile().id;
        cookie = resp->getConstructFile().cookie;
        const Crc crc(0x12345678);
        for (uint64_t i = 0; i < 2; ++i) {
            auto& atLocation = req->setAddSpanAtLocationInitiate();
            atLocation.locationId = location;
            auto& add = atLocation.req.req;
            add.fileId = fileId;
            add.cookie = cookie;
            add.byteOffset = i*BLOCK_SIZE;
            add.size = BLOCK_SIZE;
            add.crc = Crc(crc32c_append_pclmul(0, crc.u32, BLOCK_SIZE));
            add.storageClass = HDD_STORAGE;
            add.parity = Parity(1, 1);
            add.stripes = 1;
            add.cellSize = BLOCK_SIZE;
            add.crcs.els = {crc, crc};
            for (uint64_t bs = 3; bs <= 4; ++bs) {
                add.blacklist.els.emplace_back().blockService = BlockServiceId(bs);
            }
            apply();
            auto blocks = resp->getAddSpanAtLocationInitiate().resp.blocks.els;
            REQUIRE(blocks.size() == 2);
            for (const auto& block : blocks) {
                if (block.blockServiceId == BlockServiceId(1)) { oldBlocks[i] = block.blockId; }
            }
            if (dirtyLast && i == 1) { break; }
            auto& certify = req->setAddSpanCertify();
            certify.fileId = fileId;
            certify.cookie = cookie;
            certify.byteOffset = i*BLOCK_SIZE;
            for (const auto& block : blocks) {
                certify.proofs.els.push_back(proof('W', block.blockServiceId, block.blockId));
            }
            apply();
        }
        if (!transient) {
            auto& link = req->setLinkFile();
            link.fileId = fileId;
            link.cookie = cookie;
            link.ownerId = ROOT_DIR_INODE_ID;
            link.name = BincodeBytes("replacement");
            apply();
        }
        if (decommission) {
            services[0].flags = BlockServiceFlags::DECOMMISSIONED;
            db.blockServicesCacheDB->updateCache(services);
        }
    }

    ReplaceBlockInitiateReq& initiateRequest(size_t span = 0) {
        auto& init = req->setReplaceBlockInitiate();
        init.fileId = fileId;
        init.byteOffset = span*BLOCK_SIZE;
        init.oldBlockId = oldBlocks[span];
        return init;
    }

    ReplaceBlockInitiateResp initiate(size_t span = 0) {
        initiateRequest(span);
        apply();
        auto result = resp->getReplaceBlockInitiate();
        CHECK(result.newBlock.blockServiceId.u64 >= 3);
        return result;
    }

    ReplaceBlockCertifyReq& certifyRequest(const ReplaceBlockInitiateResp& init, size_t span = 0) {
        auto& certify = req->setReplaceBlockCertify();
        certify.fileId = fileId;
        certify.byteOffset = span*BLOCK_SIZE;
        certify.oldBlockId = oldBlocks[span];
        certify.holderId = init.holderId;
        certify.holderCookie = init.holderCookie;
        certify.newBlock = proof('W', init.newBlock.blockServiceId, init.newBlock.blockId);
        return certify;
    }

    void collect(const ReplaceBlockInitiateResp& init, uint64_t expectedBlockId = 0) {
        auto& scrap = req->setScrapTransientFile();
        scrap.id = init.holderId;
        scrap.cookie = init.holderCookie;
        apply();
        auto& remove = req->setRemoveSpanInitiate();
        remove.fileId = init.holderId;
        remove.cookie = init.holderCookie;
        apply();
        auto blocks = resp->getRemoveSpanInitiate().blocks.els;
        REQUIRE(resp->getRemoveSpanInitiate().byteOffset == 0);
        REQUIRE(blocks.size() == 1);
        CHECK(blocks[0].blockId == (expectedBlockId == 0 ? init.newBlock.blockId : expectedBlockId));
        auto& certify = req->setRemoveSpanCertify();
        certify.fileId = init.holderId;
        certify.cookie = init.holderCookie;
        certify.proofs.els.push_back(proof('E', blocks[0].blockServiceId, blocks[0].blockId));
        apply();
        auto& empty = req->setRemoveSpanInitiate();
        empty.fileId = init.holderId;
        empty.cookie = init.holderCookie;
        apply(TernError::FILE_EMPTY);
        req->setRemoveInode().id = init.holderId;
        apply();
    }
};

TEST_CASE("single block replacement and abandoned holder cleanup") {
    BlockReplacementTest f(false, false, LocationId(2));
    auto first = f.initiate();
    auto abandoned = f.initiate(1);
    f.certifyRequest(first);
    f.apply();
    CHECK(f.resp->getReplaceBlockCertify().replaced);
    f.req->setStatTransientFile().id = first.holderId;
    f.db->read(*f.req, *f.resp);
    REQUIRE(f.resp->kind() == ShardMessageKind::ERROR);
    CHECK(f.resp->getError() == TernError::FILE_NOT_FOUND);
    f.certifyRequest(first);
    CHECK(f.prepare() == TernError::BLOCK_NOT_FOUND);
    auto& spans = f.req->setFileSpans();
    spans.fileId = f.fileId;
    f.db->read(*f.req, *f.resp);
    REQUIRE(f.resp->kind() == ShardMessageKind::FILE_SPANS);
    const auto& spansResp = f.resp->getFileSpans();
    REQUIRE(spansResp.spans.els.size() == 2);
    const auto& location = spansResp.spans.els[0].getLocations().locations.els[0];
    CHECK(location.locationId == 2);
    bool found = false;
    for (const auto& block : location.blocks.els) {
        found |= block.blockId == first.newBlock.blockId;
    }
    CHECK(found);
    f.collect(abandoned);
}

TEST_CASE("single block replacement validates source and write proof") {
    BlockReplacementTest f;
    auto init = f.initiate();
    SUBCASE("bad proof leaves holder for GC") {
        f.certifyRequest(init).newBlock.proof.data[0] ^= 1;
        f.apply(TernError::BAD_BLOCK_PROOF);
        f.collect(init);
    }
    SUBCASE("wrong old block is rejected") {
        f.initiateRequest().oldBlockId++;
        CHECK(f.prepare() == TernError::BLOCK_NOT_FOUND);
    }
    SUBCASE("concurrent replacement leaves losing holder intact") {
        auto other = f.initiate();
        f.certifyRequest(init);
        NO_TERN_ERROR(f.prepare());
        auto pending = std::make_unique<ShardLogEntry>(*f.entry);
        f.certifyRequest(other);
        f.apply();
        CHECK(f.resp->getReplaceBlockCertify().replaced);
        f.applyEntry(*pending);
        CHECK_FALSE(f.resp->getReplaceBlockCertify().replaced);
        f.collect(init);
    }
}

TEST_CASE("block replacement retains the old block when an erase is still needed") {
    BlockReplacementTest f(false, false, LocationId(2), false);
    auto init = f.initiate();
    f.certifyRequest(init);
    NO_TERN_ERROR(f.prepare());
    CHECK_FALSE(f.entry->body.getReplaceBlockCertify().discardOldBlock);
    SUBCASE("healthy service") {}
    SUBCASE("service decommissioned after prepare") {
        f.services[0].flags = BlockServiceFlags::DECOMMISSIONED;
        f.db.blockServicesCacheDB->updateCache(f.services);
    }
    // Apply must honor the recorded decision even if its cache has changed.
    f.applyEntry(*f.entry);
    CHECK(f.resp->getReplaceBlockCertify().replaced);
    f.req->setFileSpans().fileId = init.holderId;
    f.db->read(*f.req, *f.resp);
    REQUIRE(f.resp->kind() == ShardMessageKind::FILE_SPANS);
    const auto& spans = f.resp->getFileSpans().spans.els;
    REQUIRE(spans.size() == 1);
    const auto& locations = spans[0].getLocations().locations.els;
    REQUIRE(locations.size() == 1);
    REQUIRE(locations[0].blocks.els.size() == 1);
    CHECK(locations[0].locationId == 2);
    CHECK(locations[0].blocks.els[0].blockId == f.oldBlocks[0]);

    auto& refs = f.req->setBlockServiceFiles();
    refs.blockServiceId = BlockServiceId(1);
    refs.startFrom = init.holderId;
    f.db->read(*f.req, *f.resp);
    REQUIRE(f.resp->kind() == ShardMessageKind::BLOCK_SERVICE_FILES);
    REQUIRE(f.resp->getBlockServiceFiles().fileIds.els.size() == 1);
    CHECK(f.resp->getBlockServiceFiles().fileIds.els[0] == init.holderId);
    f.collect(init, f.oldBlocks[0]);
    auto& after = f.req->setBlockServiceFiles();
    after.blockServiceId = BlockServiceId(1);
    after.startFrom = init.holderId;
    f.db->read(*f.req, *f.resp);
    CHECK(f.resp->getBlockServiceFiles().fileIds.els.empty());
}

TEST_CASE("block replacement discards an old block decommissioned before certify") {
    BlockReplacementTest f(false, false, 0, false);
    auto init = f.initiate();
    f.services[0].flags = BlockServiceFlags::DECOMMISSIONED;
    f.db.blockServicesCacheDB->updateCache(f.services);
    f.certifyRequest(init);
    NO_TERN_ERROR(f.prepare());
    CHECK(f.entry->body.getReplaceBlockCertify().discardOldBlock);
    // A replica can apply this log entry before learning the new registry flags.
    f.services[0].flags = BlockServiceFlags::EMPTY;
    f.db.blockServicesCacheDB->updateCache(f.services);
    f.applyEntry(*f.entry);
    CHECK(f.resp->getReplaceBlockCertify().replaced);
    f.req->setStatTransientFile().id = init.holderId;
    f.db->read(*f.req, *f.resp);
    REQUIRE(f.resp->kind() == ShardMessageKind::ERROR);
    CHECK(f.resp->getError() == TernError::FILE_NOT_FOUND);
}

TEST_CASE("single block replacement respects transient span ownership") {
    BlockReplacementTest f(true, true);
    f.initiateRequest(1);
    CHECK(f.prepare() == TernError::SPAN_NOT_FOUND);
    auto& scrap = f.req->setScrapTransientFile();
    scrap.id = f.fileId;
    scrap.cookie = f.cookie;
    f.apply();
    auto init = f.initiate(1);
    SUBCASE("expired dirty span can be replaced") {
        f.certifyRequest(init, 1);
        f.apply();
        CHECK(f.resp->getReplaceBlockCertify().replaced);
    }
    SUBCASE("GC condemns the span before certification is applied") {
        f.certifyRequest(init, 1);
        NO_TERN_ERROR(f.prepare());
        auto pending = std::make_unique<ShardLogEntry>(*f.entry);
        auto& remove = f.req->setRemoveSpanInitiate();
        remove.fileId = f.fileId;
        remove.cookie = f.cookie;
        f.apply();
        f.applyEntry(*pending);
        CHECK_FALSE(f.resp->getReplaceBlockCertify().replaced);
        f.collect(init);
        f.initiateRequest(1);
        CHECK(f.prepare() == TernError::SPAN_NOT_FOUND);
    }
}
TEST_CASE("test fmt") {
    {
        std::stringstream ss;
        ss << TernTime(0);
        REQUIRE(ss.str() == "1970-01-01T00:00:00.000000000Z");
    }
    {
        std::stringstream ss;
        ss << TernTime(1234567891ull);
        REQUIRE(ss.str() == "1970-01-01T00:00:01.234567891Z");
    }
}

/*
TEST_CASE("make/rm directory") {
    // not actually the full lifecycle, just some ad hoc tests

    TempShardDB db(LogLevel::LOG_ERROR, ShardId(0));

    auto reqContainer = std::make_unique<ShardReqContainer>();
    auto respContainer = std::make_unique<ShardRespContainer>();
    auto logEntry = std::make_unique<ShardLogEntry>();
    uint64_t logEntryIndex = 0;

    BincodeBytes defaultInfo = defaultDirectoryInfo();
    InodeId id(InodeType::DIRECTORY, ShardId(0), 1);

    {
        auto& req = reqContainer->setCreateDirectoryInode();
        req.id = id;
        req.info.inherited = true;
        req.ownerId = ROOT_DIR_INODE_ID;
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        NO_TERN_ERROR(db->applyLogEntry(true, ++logEntryIndex, *logEntry, *respContainer));
        respContainer->getCreateDirectoryInode();
    }
    {
        auto& req = reqContainer->setRemoveDirectoryOwner();
        req.dirId = id;
        req.info = defaultInfo;
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        NO_TERN_ERROR(db->applyLogEntry(true, ++logEntryIndex, *logEntry, *respContainer));
    }
    {
        auto& req = reqContainer->setStatDirectory();
        req.id = id;
        NO_TERN_ERROR(db->read(*reqContainer, *respContainer));
        const auto& resp = respContainer->getStatDirectory();
        CHECK(resp.info == defaultInfo);
    }
    {
        auto& req = reqContainer->setSetDirectoryOwner();
        req.dirId = id;
        req.ownerId = ROOT_DIR_INODE_ID;
        NO_TERN_ERROR(db->prepareLogEntry(*reqContainer, *logEntry));
        NO_TERN_ERROR(db->applyLogEntry(true, ++logEntryIndex, *logEntry, *respContainer));
    }
}
*/
