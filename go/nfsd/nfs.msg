const NFS4_FHSIZE = 128;
const NFS4_VERIFIER_SIZE = 8;
const NFS4_OTHER_SIZE = 12;
const NFS4_OPAQUE_LIMIT = 1024;
const NFS4_INT64_MAX = 0x7fffffffffffffff;
const NFS4_UINT64_MAX = 0xffffffffffffffff;
const NFS4_INT32_MAX = 0x7fffffff;
const NFS4_UINT32_MAX = 0xffffffff;
const ACL4_SUPPORT_ALLOW_ACL = 0x00000001;
const ACL4_SUPPORT_DENY_ACL = 0x00000002;
const ACL4_SUPPORT_AUDIT_ACL = 0x00000004;
const ACL4_SUPPORT_ALARM_ACL = 0x00000008;
const ACE4_ACCESS_ALLOWED_ACE_TYPE = 0x00000000;
const ACE4_ACCESS_DENIED_ACE_TYPE = 0x00000001;
const ACE4_SYSTEM_AUDIT_ACE_TYPE = 0x00000002;
const ACE4_SYSTEM_ALARM_ACE_TYPE = 0x00000003;
const ACE4_FILE_INHERIT_ACE = 0x00000001;
const ACE4_DIRECTORY_INHERIT_ACE = 0x00000002;
const ACE4_NO_PROPAGATE_INHERIT_ACE = 0x00000004;
const ACE4_INHERIT_ONLY_ACE = 0x00000008;
const ACE4_SUCCESSFUL_ACCESS_ACE_FLAG = 0x00000010;
const ACE4_FAILED_ACCESS_ACE_FLAG = 0x00000020;
const ACE4_IDENTIFIER_GROUP = 0x00000040;
const ACE4_READ_DATA = 0x00000001;
const ACE4_LIST_DIRECTORY = 0x00000001;
const ACE4_WRITE_DATA = 0x00000002;
const ACE4_ADD_FILE = 0x00000002;
const ACE4_APPEND_DATA = 0x00000004;
const ACE4_ADD_SUBDIRECTORY = 0x00000004;
const ACE4_READ_NAMED_ATTRS = 0x00000008;
const ACE4_WRITE_NAMED_ATTRS = 0x00000010;
const ACE4_EXECUTE = 0x00000020;
const ACE4_DELETE_CHILD = 0x00000040;
const ACE4_READ_ATTRIBUTES = 0x00000080;
const ACE4_WRITE_ATTRIBUTES = 0x00000100;
const ACE4_DELETE = 0x00010000;
const ACE4_READ_ACL = 0x00020000;
const ACE4_WRITE_ACL = 0x00040000;
const ACE4_WRITE_OWNER = 0x00080000;
const ACE4_SYNCHRONIZE = 0x00100000;
const ACE4_GENERIC_READ = 0x00120081;
const ACE4_GENERIC_WRITE = 0x00160106;
const ACE4_GENERIC_EXECUTE = 0x001200A0;
const MODE4_SUID = 0x800;
const MODE4_SGID = 0x400;
const MODE4_SVTX = 0x200;
const MODE4_RUSR = 0x100;
const MODE4_WUSR = 0x080;
const MODE4_XUSR = 0x040;
const MODE4_RGRP = 0x020;
const MODE4_WGRP = 0x010;
const MODE4_XGRP = 0x008;
const MODE4_ROTH = 0x004;
const MODE4_WOTH = 0x002;
const MODE4_XOTH = 0x001;
const FH4_PERSISTENT = 0x00000000;
const FH4_NOEXPIRE_WITH_OPEN = 0x00000001;
const FH4_VOLATILE_ANY = 0x00000002;
const FH4_VOL_MIGRATION = 0x00000004;
const FH4_VOL_RENAME = 0x00000008;
const FATTR4_SUPPORTED_ATTRS = 0;
const FATTR4_TYPE = 1;
const FATTR4_FH_EXPIRE_TYPE = 2;
const FATTR4_CHANGE = 3;
const FATTR4_SIZE = 4;
const FATTR4_LINK_SUPPORT = 5;
const FATTR4_SYMLINK_SUPPORT = 6;
const FATTR4_NAMED_ATTR = 7;
const FATTR4_FSID = 8;
const FATTR4_UNIQUE_HANDLES = 9;
const FATTR4_LEASE_TIME = 10;
const FATTR4_RDATTR_ERROR = 11;
const FATTR4_FILEHANDLE = 19;
const FATTR4_ACL = 12;
const FATTR4_ACLSUPPORT = 13;
const FATTR4_ARCHIVE = 14;
const FATTR4_CANSETTIME = 15;
const FATTR4_CASE_INSENSITIVE = 16;
const FATTR4_CASE_PRESERVING = 17;
const FATTR4_CHOWN_RESTRICTED = 18;
const FATTR4_FILEID = 20;
const FATTR4_FILES_AVAIL = 21;
const FATTR4_FILES_FREE = 22;
const FATTR4_FILES_TOTAL = 23;
const FATTR4_FS_LOCATIONS = 24;
const FATTR4_HIDDEN = 25;
const FATTR4_HOMOGENEOUS = 26;
const FATTR4_MAXFILESIZE = 27;
const FATTR4_MAXLINK = 28;
const FATTR4_MAXNAME = 29;
const FATTR4_MAXREAD = 30;
const FATTR4_MAXWRITE = 31;
const FATTR4_MIMETYPE = 32;
const FATTR4_MODE = 33;
const FATTR4_NO_TRUNC = 34;
const FATTR4_NUMLINKS = 35;
const FATTR4_OWNER = 36;
const FATTR4_OWNER_GROUP = 37;
const FATTR4_QUOTA_AVAIL_HARD = 38;
const FATTR4_QUOTA_AVAIL_SOFT = 39;
const FATTR4_QUOTA_USED = 40;
const FATTR4_RAWDEV = 41;
const FATTR4_SPACE_AVAIL = 42;
const FATTR4_SPACE_FREE = 43;
const FATTR4_SPACE_TOTAL = 44;
const FATTR4_SPACE_USED = 45;
const FATTR4_SYSTEM = 46;
const FATTR4_TIME_ACCESS = 47;
const FATTR4_TIME_ACCESS_SET = 48;
const FATTR4_TIME_BACKUP = 49;
const FATTR4_TIME_CREATE = 50;
const FATTR4_TIME_DELTA = 51;
const FATTR4_TIME_METADATA = 52;
const FATTR4_TIME_MODIFY = 53;
const FATTR4_TIME_MODIFY_SET = 54;
const FATTR4_MOUNTED_ON_FILEID = 55;
const ACCESS4_READ = 0x00000001;
const ACCESS4_LOOKUP = 0x00000002;
const ACCESS4_MODIFY = 0x00000004;
const ACCESS4_EXTEND = 0x00000008;
const ACCESS4_DELETE = 0x00000010;
const ACCESS4_EXECUTE = 0x00000020;
const OPEN4_SHARE_ACCESS_READ = 0x00000001;
const OPEN4_SHARE_ACCESS_WRITE = 0x00000002;
const OPEN4_SHARE_ACCESS_BOTH = 0x00000003;
const OPEN4_SHARE_DENY_NONE = 0x00000000;
const OPEN4_SHARE_DENY_READ = 0x00000001;
const OPEN4_SHARE_DENY_WRITE = 0x00000002;
const OPEN4_SHARE_DENY_BOTH = 0x00000003;
const OPEN4_RESULT_CONFIRM = 0x00000002;
const OPEN4_RESULT_LOCKTYPE_POSIX = 0x00000004;

enum nfs_ftype4 : beu32 {
    NF4REG       = 1;
    NF4DIR       = 2;
    NF4BLK       = 3;
    NF4CHR       = 4;
    NF4LNK       = 5;
    NF4SOCK      = 6;
    NF4FIFO      = 7;
    NF4ATTRDIR   = 8;
    NF4NAMEDATTR = 9;
}

enum nfsstat4 : beu32 {
    NFS4_OK                     = 0;
    NFS4ERR_PERM                = 1;
    NFS4ERR_NOENT               = 2;
    NFS4ERR_IO                  = 5;
    NFS4ERR_NXIO                = 6;
    NFS4ERR_ACCESS              = 13;
    NFS4ERR_EXIST               = 17;
    NFS4ERR_XDEV                = 18;
    NFS4ERR_NOTDIR              = 20;
    NFS4ERR_ISDIR               = 21;
    NFS4ERR_INVAL               = 22;
    NFS4ERR_FBIG                = 27;
    NFS4ERR_NOSPC               = 28;
    NFS4ERR_ROFS                = 30;
    NFS4ERR_MLINK               = 31;
    NFS4ERR_NAMETOOLONG         = 63;
    NFS4ERR_NOTEMPTY            = 66;
    NFS4ERR_DQUOT               = 69;
    NFS4ERR_STALE               = 70;
    NFS4ERR_BADHANDLE           = 10001;
    NFS4ERR_BAD_COOKIE          = 10003;
    NFS4ERR_NOTSUPP             = 10004;
    NFS4ERR_TOOSMALL            = 10005;
    NFS4ERR_SERVERFAULT         = 10006;
    NFS4ERR_BADTYPE             = 10007;
    NFS4ERR_DELAY               = 10008;
    NFS4ERR_SAME                = 10009;
    NFS4ERR_DENIED              = 10010;
    NFS4ERR_EXPIRED             = 10011;
    NFS4ERR_LOCKED              = 10012;
    NFS4ERR_GRACE               = 10013;
    NFS4ERR_FHEXPIRED           = 10014;
    NFS4ERR_SHARE_DENIED        = 10015;
    NFS4ERR_WRONGSEC            = 10016;
    NFS4ERR_CLID_INUSE          = 10017;
    NFS4ERR_RESOURCE            = 10018;
    NFS4ERR_MOVED               = 10019;
    NFS4ERR_NOFILEHANDLE        = 10020;
    NFS4ERR_MINOR_VERS_MISMATCH = 10021;
    NFS4ERR_STALE_CLIENTID      = 10022;
    NFS4ERR_STALE_STATEID       = 10023;
    NFS4ERR_OLD_STATEID         = 10024;
    NFS4ERR_BAD_STATEID         = 10025;
    NFS4ERR_BAD_SEQID           = 10026;
    NFS4ERR_NOT_SAME            = 10027;
    NFS4ERR_LOCK_RANGE          = 10028;
    NFS4ERR_SYMLINK             = 10029;
    NFS4ERR_RESTOREFH           = 10030;
    NFS4ERR_LEASE_MOVED         = 10031;
    NFS4ERR_ATTRNOTSUPP         = 10032;
    NFS4ERR_NO_GRACE            = 10033;
    NFS4ERR_RECLAIM_BAD         = 10034;
    NFS4ERR_RECLAIM_CONFLICT    = 10035;
    NFS4ERR_BADXDR              = 10036;
    NFS4ERR_LOCKS_HELD          = 10037;
    NFS4ERR_OPENMODE            = 10038;
    NFS4ERR_BADOWNER            = 10039;
    NFS4ERR_BADCHAR             = 10040;
    NFS4ERR_BADNAME             = 10041;
    NFS4ERR_BAD_RANGE           = 10042;
    NFS4ERR_LOCK_NOTSUPP        = 10043;
    NFS4ERR_OP_ILLEGAL          = 10044;
    NFS4ERR_DEADLOCK            = 10045;
    NFS4ERR_FILE_OPEN           = 10046;
    NFS4ERR_ADMIN_REVOKED       = 10047;
    NFS4ERR_CB_PATH_DOWN        = 10048;
}

message attrlist4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message bitmap4 {
    beu32 count;
    beu32 data[count];
}

message nfs_fh4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message sec_oid4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message utf8str_cis {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message utf8str_cs {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message utf8str_mixed {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message component4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message linktext4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message pathname4 {
    beu32      count;
    component4 data[count];
}

struct verifier4 {
    u8 data[8];
}

struct nfstime4 {
    bei64 seconds;
    beu32 nseconds;
}

enum time_how4 : beu32 {
    SET_TO_SERVER_TIME4 = 0;
    SET_TO_CLIENT_TIME4 = 1;
}

union settime4 (time_how4) {
    SET_TO_CLIENT_TIME4: nfstime4;
    default:             void;
}

struct fsid4 {
    beu64 major;
    beu64 minor;
}

message fs_location4 {
    beu32       server_count;
    utf8str_cis server[server_count];
    pathname4   rootpath;
}

message fs_locations4 {
    pathname4    fs_root;
    beu32        locations_count;
    fs_location4 locations[locations_count];
}

message nfsace4 {
    beu32         type_val;
    beu32         flag;
    beu32         access_mask;
    utf8str_mixed who;
}

struct specdata4 {
    beu32 specdata1;
    beu32 specdata2;
}

message fattr4 {
    bitmap4   attrmask;
    attrlist4 attr_vals;
}

struct change_info4 {
    beu32 atomic;
    beu64 before;
    beu64 after;
}

message xdr_opaque {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message clientaddr4 {
    xdr_opaque r_netid;
    xdr_opaque r_addr;
}

message cb_client4 {
    beu32       cb_program;
    clientaddr4 cb_location;
}

struct stateid4 {
    beu32 seqid;
    u8    other[12];
}

message nfs_client_id4 {
    verifier4 verifier;
    beu32     id_len;
    u8        id[id_len];
    align(4);
}

message open_owner4 {
    beu64 clientid;
    beu32 owner_len;
    u8    owner[owner_len];
    align(4);
}

message lock_owner4 {
    beu64 clientid;
    beu32 owner_len;
    u8    owner[owner_len];
    align(4);
}

enum nfs_lock_type4 : beu32 {
    READ_LT   = 1;
    WRITE_LT  = 2;
    READW_LT  = 3;
    WRITEW_LT = 4;
}

struct ACCESS4args {
    beu32 access;
}

struct ACCESS4resok {
    beu32 supported;
    beu32 access;
}

union ACCESS4res (nfsstat4) {
    NFS4_OK: ACCESS4resok;
    default: void;
}

struct CLOSE4args {
    beu32    seqid;
    stateid4 open_stateid;
}

union CLOSE4res (nfsstat4) {
    NFS4_OK: stateid4;
    default: void;
}

struct COMMIT4args {
    beu64 offset;
    beu32 count;
}

struct COMMIT4resok {
    verifier4 writeverf;
}

union COMMIT4res (nfsstat4) {
    NFS4_OK: COMMIT4resok;
    default: void;
}

union createtype4 (nfs_ftype4) {
    NF4LNK:  linktext4;
    NF4BLK:  specdata4;
    NF4CHR:  specdata4;
    NF4SOCK: void;
    NF4FIFO: void;
    NF4DIR:  void;
    default: void;
}

message CREATE4args {
    nfs_ftype4  objtype_type;
    createtype4 objtype(objtype_type);
    component4  objname;
    fattr4      createattrs;
}

message CREATE4resok {
    change_info4 cinfo;
    bitmap4      attrset;
}

union CREATE4res (nfsstat4) {
    NFS4_OK: CREATE4resok;
    default: void;
}

struct DELEGPURGE4args {
    beu64 clientid;
}

struct DELEGPURGE4res {
    nfsstat4 status;
}

struct DELEGRETURN4args {
    stateid4 deleg_stateid;
}

struct DELEGRETURN4res {
    nfsstat4 status;
}

message GETATTR4args {
    bitmap4 attr_request;
}

message GETATTR4resok {
    fattr4 obj_attributes;
}

union GETATTR4res (nfsstat4) {
    NFS4_OK: GETATTR4resok;
    default: void;
}

message GETFH4resok {
    nfs_fh4 object;
}

union GETFH4res (nfsstat4) {
    NFS4_OK: GETFH4resok;
    default: void;
}

message LINK4args {
    component4 newname;
}

struct LINK4resok {
    change_info4 cinfo;
}

union LINK4res (nfsstat4) {
    NFS4_OK: LINK4resok;
    default: void;
}

message open_to_lock_owner4 {
    beu32       open_seqid;
    stateid4    open_stateid;
    beu32       lock_seqid;
    lock_owner4 lock_owner;
}

struct exist_lock_owner4 {
    stateid4 lock_stateid;
    beu32    lock_seqid;
}

enum xdr_bool : beu32 {
    FALSE = 0;
    TRUE = 1;
}

union locker4 (xdr_bool) {
    TRUE:  open_to_lock_owner4;
    FALSE: exist_lock_owner4;
}

message LOCK4args {
    nfs_lock_type4 locktype;
    beu32          reclaim;
    beu64          offset;
    beu64          length;
    xdr_bool       locker_type;
    locker4        locker(locker_type);
}

message LOCK4denied {
    beu64          offset;
    beu64          length;
    nfs_lock_type4 locktype;
    lock_owner4    owner;
}

struct LOCK4resok {
    stateid4 lock_stateid;
}

union LOCK4res (nfsstat4) {
    NFS4_OK:        LOCK4resok;
    NFS4ERR_DENIED: LOCK4denied;
    default:        void;
}

message LOCKT4args {
    nfs_lock_type4 locktype;
    beu64          offset;
    beu64          length;
    lock_owner4    owner;
}

union LOCKT4res (nfsstat4) {
    NFS4ERR_DENIED: LOCK4denied;
    NFS4_OK:        void;
    default:        void;
}

struct LOCKU4args {
    nfs_lock_type4 locktype;
    beu32          seqid;
    stateid4       lock_stateid;
    beu64          offset;
    beu64          length;
}

union LOCKU4res (nfsstat4) {
    NFS4_OK: stateid4;
    default: void;
}

message LOOKUP4args {
    component4 objname;
}

struct LOOKUP4res {
    nfsstat4 status;
}

struct LOOKUPP4res {
    nfsstat4 status;
}

message NVERIFY4args {
    fattr4 obj_attributes;
}

struct NVERIFY4res {
    nfsstat4 status;
}

enum createmode4 : beu32 {
    UNCHECKED4 = 0;
    GUARDED4   = 1;
    EXCLUSIVE4 = 2;
}

union createhow4 (createmode4) {
    UNCHECKED4: fattr4;
    GUARDED4:   fattr4;
    EXCLUSIVE4: verifier4;
}

enum opentype4 : beu32 {
    OPEN4_NOCREATE = 0;
    OPEN4_CREATE   = 1;
}

message createhow4_entry {
    createmode4 disc;
    createhow4  value(disc);
}

union openflag4 (opentype4) {
    OPEN4_CREATE: createhow4_entry;
    default:      void;
}

enum limit_by4 : beu32 {
    NFS_LIMIT_SIZE   = 1;
    NFS_LIMIT_BLOCKS = 2;
}

struct nfs_modified_limit4 {
    beu32 num_blocks;
    beu32 bytes_per_block;
}

struct nfs_space_limit4_NFS_LIMIT_SIZE {
    beu64 value;
}

union nfs_space_limit4 (limit_by4) {
    NFS_LIMIT_SIZE:   nfs_space_limit4_NFS_LIMIT_SIZE;
    NFS_LIMIT_BLOCKS: nfs_modified_limit4;
}

enum open_delegation_type4 : beu32 {
    OPEN_DELEGATE_NONE  = 0;
    OPEN_DELEGATE_READ  = 1;
    OPEN_DELEGATE_WRITE = 2;
}

enum open_claim_type4 : beu32 {
    CLAIM_NULL          = 0;
    CLAIM_PREVIOUS      = 1;
    CLAIM_DELEGATE_CUR  = 2;
    CLAIM_DELEGATE_PREV = 3;
}

message open_claim_delegate_cur4 {
    stateid4   delegate_stateid;
    component4 file;
}

struct open_claim4_CLAIM_PREVIOUS {
    open_delegation_type4 value;
}

union open_claim4 (open_claim_type4) {
    CLAIM_NULL:          component4;
    CLAIM_PREVIOUS:      open_claim4_CLAIM_PREVIOUS;
    CLAIM_DELEGATE_CUR:  open_claim_delegate_cur4;
    CLAIM_DELEGATE_PREV: component4;
}

message OPEN4args {
    beu32            seqid;
    beu32            share_access;
    beu32            share_deny;
    open_owner4      owner;
    opentype4        openhow_type;
    openflag4        openhow(openhow_type);
    open_claim_type4 claim_type;
    open_claim4      claim(claim_type);
}

message open_read_delegation4 {
    stateid4 stateid;
    beu32    recall;
    nfsace4  permissions;
}

message open_write_delegation4 {
    stateid4         stateid;
    beu32            recall;
    limit_by4        space_limit_type;
    nfs_space_limit4 space_limit(space_limit_type);
    nfsace4          permissions;
}

union open_delegation4 (open_delegation_type4) {
    OPEN_DELEGATE_NONE:  void;
    OPEN_DELEGATE_READ:  open_read_delegation4;
    OPEN_DELEGATE_WRITE: open_write_delegation4;
}

message OPEN4resok {
    stateid4              stateid;
    change_info4          cinfo;
    beu32                 rflags;
    bitmap4               attrset;
    open_delegation_type4 delegation_type;
    open_delegation4      delegation(delegation_type);
}

union OPEN4res (nfsstat4) {
    NFS4_OK: OPEN4resok;
    default: void;
}

struct OPENATTR4args {
    beu32 createdir;
}

struct OPENATTR4res {
    nfsstat4 status;
}

struct OPEN_CONFIRM4args {
    stateid4 open_stateid;
    beu32    seqid;
}

struct OPEN_CONFIRM4resok {
    stateid4 open_stateid;
}

union OPEN_CONFIRM4res (nfsstat4) {
    NFS4_OK: OPEN_CONFIRM4resok;
    default: void;
}

struct OPEN_DOWNGRADE4args {
    stateid4 open_stateid;
    beu32    seqid;
    beu32    share_access;
    beu32    share_deny;
}

struct OPEN_DOWNGRADE4resok {
    stateid4 open_stateid;
}

union OPEN_DOWNGRADE4res (nfsstat4) {
    NFS4_OK: OPEN_DOWNGRADE4resok;
    default: void;
}

message PUTFH4args {
    nfs_fh4 object;
}

struct PUTFH4res {
    nfsstat4 status;
}

struct PUTPUBFH4res {
    nfsstat4 status;
}

struct PUTROOTFH4res {
    nfsstat4 status;
}

struct READ4args {
    stateid4 stateid;
    beu64    offset;
    beu32    count;
}

message READ4resok {
    beu32 eof;
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

union READ4res (nfsstat4) {
    NFS4_OK: READ4resok;
    default: void;
}

message READDIR4args {
    beu64     cookie;
    verifier4 cookieverf;
    beu32     dircount;
    beu32     maxcount;
    bitmap4   attr_request;
}

union entry4_opt (xdr_bool) {
    TRUE:    entry4;
    default: void;
}

message entry4 {
    beu64      cookie;
    component4 name;
    fattr4     attrs;
    xdr_bool   nextentry_present;
    entry4_opt nextentry(nextentry_present);
}

message dirlist4 {
    xdr_bool   entries_present;
    entry4_opt entries(entries_present);
    beu32      eof;
}

message READDIR4resok {
    verifier4 cookieverf;
    dirlist4  reply;
}

union READDIR4res (nfsstat4) {
    NFS4_OK: READDIR4resok;
    default: void;
}

message READLINK4resok {
    linktext4 link;
}

union READLINK4res (nfsstat4) {
    NFS4_OK: READLINK4resok;
    default: void;
}

message REMOVE4args {
    component4 target;
}

struct REMOVE4resok {
    change_info4 cinfo;
}

union REMOVE4res (nfsstat4) {
    NFS4_OK: REMOVE4resok;
    default: void;
}

message RENAME4args {
    component4 oldname;
    component4 newname;
}

struct RENAME4resok {
    change_info4 source_cinfo;
    change_info4 target_cinfo;
}

union RENAME4res (nfsstat4) {
    NFS4_OK: RENAME4resok;
    default: void;
}

struct RENEW4args {
    beu64 clientid;
}

struct RENEW4res {
    nfsstat4 status;
}

struct RESTOREFH4res {
    nfsstat4 status;
}

struct SAVEFH4res {
    nfsstat4 status;
}

message SECINFO4args {
    component4 name;
}

enum rpc_gss_svc_t : beu32 {
    RPC_GSS_SVC_NONE      = 1;
    RPC_GSS_SVC_INTEGRITY = 2;
    RPC_GSS_SVC_PRIVACY   = 3;
}

message rpcsec_gss_info {
    sec_oid4      oid;
    beu32         qop;
    rpc_gss_svc_t service;
}

enum disc_secinfo4 : beu32 {
    RPCSEC_GSS = 6;
}

union secinfo4 (disc_secinfo4) {
    RPCSEC_GSS: rpcsec_gss_info;
    default:    void;
}

message secinfo4_entry {
    disc_secinfo4 disc;
    secinfo4      value(disc);
}

message SECINFO4resok {
    beu32          count;
    secinfo4_entry data[count];
}

union SECINFO4res (nfsstat4) {
    NFS4_OK: SECINFO4resok;
    default: void;
}

message SETATTR4args {
    stateid4 stateid;
    fattr4   obj_attributes;
}

message SETATTR4res {
    nfsstat4 status;
    bitmap4  attrsset;
}

message SETCLIENTID4args {
    nfs_client_id4 client;
    cb_client4     callback;
    beu32          callback_ident;
}

struct SETCLIENTID4resok {
    beu64     clientid;
    verifier4 setclientid_confirm;
}

union SETCLIENTID4res (nfsstat4) {
    NFS4_OK:            SETCLIENTID4resok;
    NFS4ERR_CLID_INUSE: clientaddr4;
    default:            void;
}

struct SETCLIENTID_CONFIRM4args {
    beu64     clientid;
    verifier4 setclientid_confirm;
}

struct SETCLIENTID_CONFIRM4res {
    nfsstat4 status;
}

message VERIFY4args {
    fattr4 obj_attributes;
}

struct VERIFY4res {
    nfsstat4 status;
}

enum stable_how4 : beu32 {
    UNSTABLE4  = 0;
    DATA_SYNC4 = 1;
    FILE_SYNC4 = 2;
}

message WRITE4args {
    stateid4    stateid;
    beu64       offset;
    stable_how4 stable;
    beu32       data_len;
    u8          data[data_len];
    align(4);
}

struct WRITE4resok {
    beu32       count;
    stable_how4 committed;
    verifier4   writeverf;
}

union WRITE4res (nfsstat4) {
    NFS4_OK: WRITE4resok;
    default: void;
}

message RELEASE_LOCKOWNER4args {
    lock_owner4 lock_owner;
}

struct RELEASE_LOCKOWNER4res {
    nfsstat4 status;
}

struct ILLEGAL4res {
    nfsstat4 status;
}

enum nfs_opnum4 : beu32 {
    OP_ACCESS              = 3;
    OP_CLOSE               = 4;
    OP_COMMIT              = 5;
    OP_CREATE              = 6;
    OP_DELEGPURGE          = 7;
    OP_DELEGRETURN         = 8;
    OP_GETATTR             = 9;
    OP_GETFH               = 10;
    OP_LINK                = 11;
    OP_LOCK                = 12;
    OP_LOCKT               = 13;
    OP_LOCKU               = 14;
    OP_LOOKUP              = 15;
    OP_LOOKUPP             = 16;
    OP_NVERIFY             = 17;
    OP_OPEN                = 18;
    OP_OPENATTR            = 19;
    OP_OPEN_CONFIRM        = 20;
    OP_OPEN_DOWNGRADE      = 21;
    OP_PUTFH               = 22;
    OP_PUTPUBFH            = 23;
    OP_PUTROOTFH           = 24;
    OP_READ                = 25;
    OP_READDIR             = 26;
    OP_READLINK            = 27;
    OP_REMOVE              = 28;
    OP_RENAME              = 29;
    OP_RENEW               = 30;
    OP_RESTOREFH           = 31;
    OP_SAVEFH              = 32;
    OP_SECINFO             = 33;
    OP_SETATTR             = 34;
    OP_SETCLIENTID         = 35;
    OP_SETCLIENTID_CONFIRM = 36;
    OP_VERIFY              = 37;
    OP_WRITE               = 38;
    OP_RELEASE_LOCKOWNER   = 39;
    OP_ILLEGAL             = 10044;
}

union nfs_argop4 (nfs_opnum4) {
    OP_ACCESS:              ACCESS4args;
    OP_CLOSE:               CLOSE4args;
    OP_COMMIT:              COMMIT4args;
    OP_CREATE:              CREATE4args;
    OP_DELEGPURGE:          DELEGPURGE4args;
    OP_DELEGRETURN:         DELEGRETURN4args;
    OP_GETATTR:             GETATTR4args;
    OP_GETFH:               void;
    OP_LINK:                LINK4args;
    OP_LOCK:                LOCK4args;
    OP_LOCKT:               LOCKT4args;
    OP_LOCKU:               LOCKU4args;
    OP_LOOKUP:              LOOKUP4args;
    OP_LOOKUPP:             void;
    OP_NVERIFY:             NVERIFY4args;
    OP_OPEN:                OPEN4args;
    OP_OPENATTR:            OPENATTR4args;
    OP_OPEN_CONFIRM:        OPEN_CONFIRM4args;
    OP_OPEN_DOWNGRADE:      OPEN_DOWNGRADE4args;
    OP_PUTFH:               PUTFH4args;
    OP_PUTPUBFH:            void;
    OP_PUTROOTFH:           void;
    OP_READ:                READ4args;
    OP_READDIR:             READDIR4args;
    OP_READLINK:            void;
    OP_REMOVE:              REMOVE4args;
    OP_RENAME:              RENAME4args;
    OP_RENEW:               RENEW4args;
    OP_RESTOREFH:           void;
    OP_SAVEFH:              void;
    OP_SECINFO:             SECINFO4args;
    OP_SETATTR:             SETATTR4args;
    OP_SETCLIENTID:         SETCLIENTID4args;
    OP_SETCLIENTID_CONFIRM: SETCLIENTID_CONFIRM4args;
    OP_VERIFY:              VERIFY4args;
    OP_WRITE:               WRITE4args;
    OP_RELEASE_LOCKOWNER:   RELEASE_LOCKOWNER4args;
    OP_ILLEGAL:             void;
}

message ACCESS4res_entry {
    nfsstat4   disc;
    ACCESS4res value(disc);
}

message CLOSE4res_entry {
    nfsstat4  disc;
    CLOSE4res value(disc);
}

message COMMIT4res_entry {
    nfsstat4   disc;
    COMMIT4res value(disc);
}

message CREATE4res_entry {
    nfsstat4   disc;
    CREATE4res value(disc);
}

message GETATTR4res_entry {
    nfsstat4    disc;
    GETATTR4res value(disc);
}

message GETFH4res_entry {
    nfsstat4  disc;
    GETFH4res value(disc);
}

message LINK4res_entry {
    nfsstat4 disc;
    LINK4res value(disc);
}

message LOCK4res_entry {
    nfsstat4 disc;
    LOCK4res value(disc);
}

message LOCKT4res_entry {
    nfsstat4  disc;
    LOCKT4res value(disc);
}

message LOCKU4res_entry {
    nfsstat4  disc;
    LOCKU4res value(disc);
}

message OPEN4res_entry {
    nfsstat4 disc;
    OPEN4res value(disc);
}

message OPEN_CONFIRM4res_entry {
    nfsstat4         disc;
    OPEN_CONFIRM4res value(disc);
}

message OPEN_DOWNGRADE4res_entry {
    nfsstat4           disc;
    OPEN_DOWNGRADE4res value(disc);
}

message READ4res_entry {
    nfsstat4 disc;
    READ4res value(disc);
}

message READDIR4res_entry {
    nfsstat4    disc;
    READDIR4res value(disc);
}

message READLINK4res_entry {
    nfsstat4     disc;
    READLINK4res value(disc);
}

message REMOVE4res_entry {
    nfsstat4   disc;
    REMOVE4res value(disc);
}

message RENAME4res_entry {
    nfsstat4   disc;
    RENAME4res value(disc);
}

message SECINFO4res_entry {
    nfsstat4    disc;
    SECINFO4res value(disc);
}

message SETCLIENTID4res_entry {
    nfsstat4        disc;
    SETCLIENTID4res value(disc);
}

message WRITE4res_entry {
    nfsstat4  disc;
    WRITE4res value(disc);
}

union nfs_resop4 (nfs_opnum4) {
    OP_ACCESS:              ACCESS4res_entry;
    OP_CLOSE:               CLOSE4res_entry;
    OP_COMMIT:              COMMIT4res_entry;
    OP_CREATE:              CREATE4res_entry;
    OP_DELEGPURGE:          DELEGPURGE4res;
    OP_DELEGRETURN:         DELEGRETURN4res;
    OP_GETATTR:             GETATTR4res_entry;
    OP_GETFH:               GETFH4res_entry;
    OP_LINK:                LINK4res_entry;
    OP_LOCK:                LOCK4res_entry;
    OP_LOCKT:               LOCKT4res_entry;
    OP_LOCKU:               LOCKU4res_entry;
    OP_LOOKUP:              LOOKUP4res;
    OP_LOOKUPP:             LOOKUPP4res;
    OP_NVERIFY:             NVERIFY4res;
    OP_OPEN:                OPEN4res_entry;
    OP_OPENATTR:            OPENATTR4res;
    OP_OPEN_CONFIRM:        OPEN_CONFIRM4res_entry;
    OP_OPEN_DOWNGRADE:      OPEN_DOWNGRADE4res_entry;
    OP_PUTFH:               PUTFH4res;
    OP_PUTPUBFH:            PUTPUBFH4res;
    OP_PUTROOTFH:           PUTROOTFH4res;
    OP_READ:                READ4res_entry;
    OP_READDIR:             READDIR4res_entry;
    OP_READLINK:            READLINK4res_entry;
    OP_REMOVE:              REMOVE4res_entry;
    OP_RENAME:              RENAME4res_entry;
    OP_RENEW:               RENEW4res;
    OP_RESTOREFH:           RESTOREFH4res;
    OP_SAVEFH:              SAVEFH4res;
    OP_SECINFO:             SECINFO4res_entry;
    OP_SETATTR:             SETATTR4res;
    OP_SETCLIENTID:         SETCLIENTID4res_entry;
    OP_SETCLIENTID_CONFIRM: SETCLIENTID_CONFIRM4res;
    OP_VERIFY:              VERIFY4res;
    OP_WRITE:               WRITE4res_entry;
    OP_RELEASE_LOCKOWNER:   RELEASE_LOCKOWNER4res;
    OP_ILLEGAL:             ILLEGAL4res;
}

message nfs_argop4_entry {
    nfs_opnum4 disc;
    nfs_argop4 value(disc);
}

message COMPOUND4args {
    utf8str_cs       tag;
    beu32            minorversion;
    beu32            argarray_count;
    nfs_argop4_entry argarray[argarray_count];
}

message nfs_resop4_entry {
    nfs_opnum4 disc;
    nfs_resop4 value(disc);
}

message COMPOUND4res {
    nfsstat4         status;
    utf8str_cs       tag;
    beu32            resarray_count;
    nfs_resop4_entry resarray[resarray_count];
}

message CB_GETATTR4args {
    nfs_fh4 fh;
    bitmap4 attr_request;
}

message CB_GETATTR4resok {
    fattr4 obj_attributes;
}

union CB_GETATTR4res (nfsstat4) {
    NFS4_OK: CB_GETATTR4resok;
    default: void;
}

message CB_RECALL4args {
    stateid4 stateid;
    beu32    truncate;
    nfs_fh4  fh;
}

struct CB_RECALL4res {
    nfsstat4 status;
}

struct CB_ILLEGAL4res {
    nfsstat4 status;
}

enum nfs_cb_opnum4 : beu32 {
    OP_CB_GETATTR = 3;
    OP_CB_RECALL  = 4;
    OP_CB_ILLEGAL = 10044;
}

union nfs_cb_argop4 (nfs_cb_opnum4) {
    OP_CB_GETATTR: CB_GETATTR4args;
    OP_CB_RECALL:  CB_RECALL4args;
    OP_CB_ILLEGAL: void;
}

message CB_GETATTR4res_entry {
    nfsstat4       disc;
    CB_GETATTR4res value(disc);
}

union nfs_cb_resop4 (nfs_cb_opnum4) {
    OP_CB_GETATTR: CB_GETATTR4res_entry;
    OP_CB_RECALL:  CB_RECALL4res;
    OP_CB_ILLEGAL: CB_ILLEGAL4res;
}

message nfs_cb_argop4_entry {
    nfs_cb_opnum4 disc;
    nfs_cb_argop4 value(disc);
}

message CB_COMPOUND4args {
    utf8str_cs          tag;
    beu32               minorversion;
    beu32               callback_ident;
    beu32               argarray_count;
    nfs_cb_argop4_entry argarray[argarray_count];
}

message nfs_cb_resop4_entry {
    nfs_cb_opnum4 disc;
    nfs_cb_resop4 value(disc);
}

message CB_COMPOUND4res {
    nfsstat4            status;
    utf8str_cs          tag;
    beu32               resarray_count;
    nfs_cb_resop4_entry resarray[resarray_count];
}

