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
        blockServicesCacheDB = std::make_unique<BlockServicesCacheDB>(logger, xmon, *sharedDB);
        db = std::make_unique<ShardDB>(logger, xmon, shid, 0, DEFAULT_DEADLINE_INTERVAL, *sharedDB, *blockServicesCacheDB);
    }

    // useful to test recovery
    void restart() {
        std::shared_ptr<XmonAgent> xmon;
        db->close();
        sharedDB = std::make_unique<SharedRocksDB>(logger, xmon, dbDir + "/db", dbDir + "/db-statistics.txt");
        initSharedDB();
        blockServicesCacheDB = std::make_unique<BlockServicesCacheDB>(logger, xmon, *sharedDB);
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
