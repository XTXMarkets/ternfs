const NFS4_FHSIZE = 128;
const NFS4_VERIFIER_SIZE = 8;
const NFS4_OTHER_SIZE = 12;
const NFS4_OPAQUE_LIMIT = 1024;
const NFS4_SESSIONID_SIZE = 16;
const NFS4_INT64_MAX = 0x7fffffffffffffff;
const NFS4_UINT64_MAX = 0xffffffffffffffff;
const NFS4_INT32_MAX = 0x7fffffff;
const NFS4_UINT32_MAX = 0xffffffff;
const NFS4_MAXFILELEN = 0xffffffffffffffff;
const NFS4_MAXFILEOFF = 0xfffffffffffffffe;
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
const ACE4_INHERITED_ACE = 0x00000080;
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
const ACE4_WRITE_RETENTION = 0x00000200;
const ACE4_WRITE_RETENTION_HOLD = 0x00000400;
const ACE4_DELETE = 0x00010000;
const ACE4_READ_ACL = 0x00020000;
const ACE4_WRITE_ACL = 0x00040000;
const ACE4_WRITE_OWNER = 0x00080000;
const ACE4_SYNCHRONIZE = 0x00100000;
const ACE4_GENERIC_READ = 0x00120081;
const ACE4_GENERIC_WRITE = 0x00160106;
const ACE4_GENERIC_EXECUTE = 0x001200A0;
const ACL4_AUTO_INHERIT = 0x00000001;
const ACL4_PROTECTED = 0x00000002;
const ACL4_DEFAULTED = 0x00000004;
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
const NFS4_DEVICEID4_SIZE = 16;
const LAYOUT4_RET_REC_FILE = 1;
const LAYOUT4_RET_REC_FSID = 2;
const LAYOUT4_RET_REC_ALL = 3;
const TH4_READ_SIZE = 0;
const TH4_WRITE_SIZE = 1;
const TH4_READ_IOSIZE = 2;
const TH4_WRITE_IOSIZE = 3;
const RET4_DURATION_INFINITE = 0xffffffffffffffff;
const FSCHARSET_CAP4_CONTAINS_NON_UTF8 = 0x1;
const FSCHARSET_CAP4_ALLOWS_ONLY_UTF8 = 0x2;
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
const FATTR4_SUPPATTR_EXCLCREAT = 75;
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
const FATTR4_DIR_NOTIF_DELAY = 56;
const FATTR4_DIRENT_NOTIF_DELAY = 57;
const FATTR4_DACL = 58;
const FATTR4_SACL = 59;
const FATTR4_CHANGE_POLICY = 60;
const FATTR4_FS_STATUS = 61;
const FATTR4_FS_LAYOUT_TYPES = 62;
const FATTR4_LAYOUT_HINT = 63;
const FATTR4_LAYOUT_TYPES = 64;
const FATTR4_LAYOUT_BLKSIZE = 65;
const FATTR4_LAYOUT_ALIGNMENT = 66;
const FATTR4_FS_LOCATIONS_INFO = 67;
const FATTR4_MDSTHRESHOLD = 68;
const FATTR4_RETENTION_GET = 69;
const FATTR4_RETENTION_SET = 70;
const FATTR4_RETENTEVT_GET = 71;
const FATTR4_RETENTEVT_SET = 72;
const FATTR4_RETENTION_HOLD = 73;
const FATTR4_MODE_SET_MASKED = 74;
const FATTR4_FS_CHARSET_CAP = 76;
const FATTR4_CLONE_BLKSIZE = 77;
const FATTR4_SPACE_FREED = 78;
const FATTR4_CHANGE_ATTR_TYPE = 79;
const FATTR4_SEC_LABEL = 80;
const FATTR4_XATTR_SUPPORT = 82;
const FSLI4BX_GFLAGS = 0;
const FSLI4BX_TFLAGS = 1;
const FSLI4BX_CLSIMUL = 2;
const FSLI4BX_CLHANDLE = 3;
const FSLI4BX_CLFILEID = 4;
const FSLI4BX_CLWRITEVER = 5;
const FSLI4BX_CLCHANGE = 6;
const FSLI4BX_CLREADDIR = 7;
const FSLI4BX_READRANK = 8;
const FSLI4BX_WRITERANK = 9;
const FSLI4BX_READORDER = 10;
const FSLI4BX_WRITEORDER = 11;
const FSLI4GF_WRITABLE = 0x01;
const FSLI4GF_CUR_REQ = 0x02;
const FSLI4GF_ABSENT = 0x04;
const FSLI4GF_GOING = 0x08;
const FSLI4GF_SPLIT = 0x10;
const FSLI4TF_RDMA = 0x01;
const FSLI4IF_VAR_SUB = 0x00000001;
const NFL4_UFLG_MASK = 0x0000003F;
const NFL4_UFLG_DENSE = 0x00000001;
const NFL4_UFLG_COMMIT_THRU_MDS = 0x00000002;
const NFL42_UFLG_IO_ADVISE_THRU_MDS = 0x00000004;
const NFL4_UFLG_STRIPE_UNIT_SIZE_MASK = 0xFFFFFFC0;
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
const OPEN4_SHARE_ACCESS_WANT_DELEG_MASK = 0xFF00;
const OPEN4_SHARE_ACCESS_WANT_NO_PREFERENCE = 0x0000;
const OPEN4_SHARE_ACCESS_WANT_READ_DELEG = 0x0100;
const OPEN4_SHARE_ACCESS_WANT_WRITE_DELEG = 0x0200;
const OPEN4_SHARE_ACCESS_WANT_ANY_DELEG = 0x0300;
const OPEN4_SHARE_ACCESS_WANT_NO_DELEG = 0x0400;
const OPEN4_SHARE_ACCESS_WANT_CANCEL = 0x0500;
const OPEN4_SHARE_ACCESS_WANT_SIGNAL_DELEG_WHEN_RESRC_AVAIL = 0x10000;
const OPEN4_SHARE_ACCESS_WANT_PUSH_DELEG_WHEN_UNCONTENDED = 0x20000;
const OPEN4_RESULT_CONFIRM = 0x00000002;
const OPEN4_RESULT_LOCKTYPE_POSIX = 0x00000004;
const OPEN4_RESULT_PRESERVE_UNLINKED = 0x00000008;
const OPEN4_RESULT_MAY_NOTIFY_LOCK = 0x00000020;
const EXCHGID4_FLAG_SUPP_MOVED_REFER = 0x00000001;
const EXCHGID4_FLAG_SUPP_MOVED_MIGR = 0x00000002;
const EXCHGID4_FLAG_SUPP_FENCE_OPS = 0x00000004;
const EXCHGID4_FLAG_BIND_PRINC_STATEID = 0x00000100;
const EXCHGID4_FLAG_USE_NON_PNFS = 0x00010000;
const EXCHGID4_FLAG_USE_PNFS_MDS = 0x00020000;
const EXCHGID4_FLAG_USE_PNFS_DS = 0x00040000;
const EXCHGID4_FLAG_MASK_PNFS = 0x00070000;
const EXCHGID4_FLAG_UPD_CONFIRMED_REC_A = 0x40000000;
const EXCHGID4_FLAG_CONFIRMED_R = 0x80000000;
const CREATE_SESSION4_FLAG_PERSIST = 0x00000001;
const CREATE_SESSION4_FLAG_CONN_BACK_CHAN = 0x00000002;
const CREATE_SESSION4_FLAG_CONN_RDMA = 0x00000004;
const SEQ4_STATUS_CB_PATH_DOWN = 0x00000001;
const SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRING = 0x00000002;
const SEQ4_STATUS_CB_GSS_CONTEXTS_EXPIRED = 0x00000004;
const SEQ4_STATUS_EXPIRED_ALL_STATE_REVOKED = 0x00000008;
const SEQ4_STATUS_EXPIRED_SOME_STATE_REVOKED = 0x00000010;
const SEQ4_STATUS_ADMIN_STATE_REVOKED = 0x00000020;
const SEQ4_STATUS_RECALLABLE_STATE_REVOKED = 0x00000040;
const SEQ4_STATUS_LEASE_MOVED = 0x00000080;
const SEQ4_STATUS_RESTART_RECLAIM_NEEDED = 0x00000100;
const SEQ4_STATUS_CB_PATH_DOWN_SESSION = 0x00000200;
const SEQ4_STATUS_BACKCHANNEL_FAULT = 0x00000400;
const SEQ4_STATUS_DEVID_CHANGED = 0x00000800;
const SEQ4_STATUS_DEVID_DELETED = 0x00001000;
const FF_FLAGS_NO_LAYOUTCOMMIT = 0x00000001;
const FF_FLAGS_NO_IO_THRU_MDS = 0x00000002;
const FF_FLAGS_NO_READ_IO = 0x00000004;
const RCA4_TYPE_MASK_RDATA_DLG = 0;
const RCA4_TYPE_MASK_WDATA_DLG = 1;
const RCA4_TYPE_MASK_DIR_DLG = 2;
const RCA4_TYPE_MASK_FILE_LAYOUT = 3;
const RCA4_TYPE_MASK_BLK_LAYOUT = 4;
const RCA4_TYPE_MASK_OBJ_LAYOUT_MIN = 8;
const RCA4_TYPE_MASK_OBJ_LAYOUT_MAX = 9;
const RCA4_TYPE_MASK_OTHER_LAYOUT_MIN = 12;
const RCA4_TYPE_MASK_OTHER_LAYOUT_MAX = 15;
const FATTR4_OFFLINE = 83;
const FATTR4_OPEN_ARGUMENTS = 86;
const OPEN4_SHARE_ACCESS_WANT_OPEN_XOR_DELEGATION = 0x200000;
const OPEN4_RESULT_NO_OPEN_STATEID = 0x00000010;
const FATTR4_TIME_DELEG_ACCESS = 84;
const FATTR4_TIME_DELEG_MODIFY = 85;
const OPEN4_SHARE_ACCESS_WANT_DELEG_TIMESTAMPS = 0x100000;

message authsys_parms_gids {
    beu32 count;
    beu32 data[count];
}

message authsys_parms {
    beu32              stamp;
    beu32              machinename_len;
    u8                 machinename[machinename_len];
    align(4);
    beu32              uid;
    beu32              gid;
    authsys_parms_gids gids;
}

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
    NFS4_OK                           = 0;
    NFS4ERR_PERM                      = 1;
    NFS4ERR_NOENT                     = 2;
    NFS4ERR_IO                        = 5;
    NFS4ERR_NXIO                      = 6;
    NFS4ERR_ACCESS                    = 13;
    NFS4ERR_EXIST                     = 17;
    NFS4ERR_XDEV                      = 18;
    NFS4ERR_NOTDIR                    = 20;
    NFS4ERR_ISDIR                     = 21;
    NFS4ERR_INVAL                     = 22;
    NFS4ERR_FBIG                      = 27;
    NFS4ERR_NOSPC                     = 28;
    NFS4ERR_ROFS                      = 30;
    NFS4ERR_MLINK                     = 31;
    NFS4ERR_NAMETOOLONG               = 63;
    NFS4ERR_NOTEMPTY                  = 66;
    NFS4ERR_DQUOT                     = 69;
    NFS4ERR_STALE                     = 70;
    NFS4ERR_BADHANDLE                 = 10001;
    NFS4ERR_BAD_COOKIE                = 10003;
    NFS4ERR_NOTSUPP                   = 10004;
    NFS4ERR_TOOSMALL                  = 10005;
    NFS4ERR_SERVERFAULT               = 10006;
    NFS4ERR_BADTYPE                   = 10007;
    NFS4ERR_DELAY                     = 10008;
    NFS4ERR_SAME                      = 10009;
    NFS4ERR_DENIED                    = 10010;
    NFS4ERR_EXPIRED                   = 10011;
    NFS4ERR_LOCKED                    = 10012;
    NFS4ERR_GRACE                     = 10013;
    NFS4ERR_FHEXPIRED                 = 10014;
    NFS4ERR_SHARE_DENIED              = 10015;
    NFS4ERR_WRONGSEC                  = 10016;
    NFS4ERR_CLID_INUSE                = 10017;
    NFS4ERR_RESOURCE                  = 10018;
    NFS4ERR_MOVED                     = 10019;
    NFS4ERR_NOFILEHANDLE              = 10020;
    NFS4ERR_MINOR_VERS_MISMATCH       = 10021;
    NFS4ERR_STALE_CLIENTID            = 10022;
    NFS4ERR_STALE_STATEID             = 10023;
    NFS4ERR_OLD_STATEID               = 10024;
    NFS4ERR_BAD_STATEID               = 10025;
    NFS4ERR_BAD_SEQID                 = 10026;
    NFS4ERR_NOT_SAME                  = 10027;
    NFS4ERR_LOCK_RANGE                = 10028;
    NFS4ERR_SYMLINK                   = 10029;
    NFS4ERR_RESTOREFH                 = 10030;
    NFS4ERR_LEASE_MOVED               = 10031;
    NFS4ERR_ATTRNOTSUPP               = 10032;
    NFS4ERR_NO_GRACE                  = 10033;
    NFS4ERR_RECLAIM_BAD               = 10034;
    NFS4ERR_RECLAIM_CONFLICT          = 10035;
    NFS4ERR_BADXDR                    = 10036;
    NFS4ERR_LOCKS_HELD                = 10037;
    NFS4ERR_OPENMODE                  = 10038;
    NFS4ERR_BADOWNER                  = 10039;
    NFS4ERR_BADCHAR                   = 10040;
    NFS4ERR_BADNAME                   = 10041;
    NFS4ERR_BAD_RANGE                 = 10042;
    NFS4ERR_LOCK_NOTSUPP              = 10043;
    NFS4ERR_OP_ILLEGAL                = 10044;
    NFS4ERR_DEADLOCK                  = 10045;
    NFS4ERR_FILE_OPEN                 = 10046;
    NFS4ERR_ADMIN_REVOKED             = 10047;
    NFS4ERR_CB_PATH_DOWN              = 10048;
    NFS4ERR_BADIOMODE                 = 10049;
    NFS4ERR_BADLAYOUT                 = 10050;
    NFS4ERR_BAD_SESSION_DIGEST        = 10051;
    NFS4ERR_BADSESSION                = 10052;
    NFS4ERR_BADSLOT                   = 10053;
    NFS4ERR_COMPLETE_ALREADY          = 10054;
    NFS4ERR_CONN_NOT_BOUND_TO_SESSION = 10055;
    NFS4ERR_DELEG_ALREADY_WANTED      = 10056;
    NFS4ERR_BACK_CHAN_BUSY            = 10057;
    NFS4ERR_LAYOUTTRYLATER            = 10058;
    NFS4ERR_LAYOUTUNAVAILABLE         = 10059;
    NFS4ERR_NOMATCHING_LAYOUT         = 10060;
    NFS4ERR_RECALLCONFLICT            = 10061;
    NFS4ERR_UNKNOWN_LAYOUTTYPE        = 10062;
    NFS4ERR_SEQ_MISORDERED            = 10063;
    NFS4ERR_SEQUENCE_POS              = 10064;
    NFS4ERR_REQ_TOO_BIG               = 10065;
    NFS4ERR_REP_TOO_BIG               = 10066;
    NFS4ERR_REP_TOO_BIG_TO_CACHE      = 10067;
    NFS4ERR_RETRY_UNCACHED_REP        = 10068;
    NFS4ERR_UNSAFE_COMPOUND           = 10069;
    NFS4ERR_TOO_MANY_OPS              = 10070;
    NFS4ERR_OP_NOT_IN_SESSION         = 10071;
    NFS4ERR_HASH_ALG_UNSUPP           = 10072;
    NFS4ERR_CLIENTID_BUSY             = 10074;
    NFS4ERR_PNFS_IO_HOLE              = 10075;
    NFS4ERR_SEQ_FALSE_RETRY           = 10076;
    NFS4ERR_BAD_HIGH_SLOT             = 10077;
    NFS4ERR_DEADSESSION               = 10078;
    NFS4ERR_ENCR_ALG_UNSUPP           = 10079;
    NFS4ERR_PNFS_NO_LAYOUT            = 10080;
    NFS4ERR_NOT_ONLY_OP               = 10081;
    NFS4ERR_WRONG_CRED                = 10082;
    NFS4ERR_WRONG_TYPE                = 10083;
    NFS4ERR_DIRDELEG_UNAVAIL          = 10084;
    NFS4ERR_REJECT_DELEG              = 10085;
    NFS4ERR_RETURNCONFLICT            = 10086;
    NFS4ERR_DELEG_REVOKED             = 10087;
    NFS4ERR_PARTNER_NOTSUPP           = 10088;
    NFS4ERR_PARTNER_NO_AUTH           = 10089;
    NFS4ERR_UNION_NOTSUPP             = 10090;
    NFS4ERR_OFFLOAD_DENIED            = 10091;
    NFS4ERR_WRONG_LFS                 = 10092;
    NFS4ERR_BADLABEL                  = 10093;
    NFS4ERR_OFFLOAD_NO_REQS           = 10094;
    NFS4ERR_NOXATTR                   = 10095;
    NFS4ERR_XATTR2BIG                 = 10096;
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

struct sessionid4 {
    u8 data[16];
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

message secret4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
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

struct change_policy4 {
    beu64 cp_major;
    beu64 cp_minor;
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

message nfsacl41 {
    beu32   na41_flag;
    beu32   na41_aces_count;
    nfsace4 na41_aces[na41_aces_count];
}

struct mode_masked4 {
    beu32 mm_value_to_set;
    beu32 mm_mask_bits;
}

struct specdata4 {
    beu32 specdata1;
    beu32 specdata2;
}

message xdr_opaque {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message netaddr4 {
    xdr_opaque na_r_netid;
    xdr_opaque na_r_addr;
}

message nfs_impl_id4 {
    utf8str_cis nii_domain;
    utf8str_cs  nii_name;
    nfstime4    nii_date;
}

struct stateid4 {
    beu32 seqid;
    u8    other[12];
}

enum layouttype4 : beu32 {
    LAYOUT4_NFSV4_1_FILES = 0x1;
    LAYOUT4_OSD2_OBJECTS  = 0x2;
    LAYOUT4_BLOCK_VOLUME  = 0x3;
    LAYOUT4_FLEX_FILES    = 0x4;
}

message layout_content4 {
    layouttype4 loc_type;
    beu32       loc_body_len;
    u8          loc_body[loc_body_len];
    align(4);
}

message layouthint4 {
    layouttype4 loh_type;
    beu32       loh_body_len;
    u8          loh_body[loh_body_len];
    align(4);
}

enum layoutiomode4 : beu32 {
    LAYOUTIOMODE4_READ = 1;
    LAYOUTIOMODE4_RW   = 2;
    LAYOUTIOMODE4_ANY  = 3;
}

message layout4 {
    beu64           lo_offset;
    beu64           lo_length;
    layoutiomode4   lo_iomode;
    layout_content4 lo_content;
}

struct deviceid4 {
    u8 data[16];
}

message device_addr4 {
    layouttype4 da_layout_type;
    beu32       da_addr_body_len;
    u8          da_addr_body[da_addr_body_len];
    align(4);
}

message layoutupdate4 {
    layouttype4 lou_type;
    beu32       lou_body_len;
    u8          lou_body[lou_body_len];
    align(4);
}

enum layoutreturn_type4 : beu32 {
    LAYOUTRETURN4_FILE = 1;
    LAYOUTRETURN4_FSID = 2;
    LAYOUTRETURN4_ALL  = 3;
}

message layoutreturn_file4 {
    beu64    lrf_offset;
    beu64    lrf_length;
    stateid4 lrf_stateid;
    beu32    lrf_body_len;
    u8       lrf_body[lrf_body_len];
    align(4);
}

union layoutreturn4 (layoutreturn_type4) {
    LAYOUTRETURN4_FILE: layoutreturn_file4;
    default:            void;
}

enum fs4_status_type : beu32 {
    STATUS4_FIXED     = 1;
    STATUS4_UPDATED   = 2;
    STATUS4_VERSIONED = 3;
    STATUS4_WRITABLE  = 4;
    STATUS4_REFERRAL  = 5;
}

message fs4_status {
    beu32           fss_absent;
    fs4_status_type fss_type;
    utf8str_cs      fss_source;
    utf8str_cs      fss_current;
    bei32           fss_age;
    nfstime4        fss_version;
}

message threshold_item4 {
    layouttype4 thi_layout_type;
    bitmap4     thi_hintset;
    beu32       thi_hintlist_len;
    u8          thi_hintlist[thi_hintlist_len];
    align(4);
}

message mdsthreshold4 {
    beu32           mth_hints_count;
    threshold_item4 mth_hints[mth_hints_count];
}

message retention_get4 {
    beu64    rg_duration;
    beu32    rg_begin_time_count;
    nfstime4 rg_begin_time[rg_begin_time_count];
}

message retention_set4 {
    beu32 rs_enable;
    beu32 rs_duration_count;
    beu64 rs_duration[rs_duration_count];
}

enum netloc_type4 : beu32 {
    NL4_NAME    = 1;
    NL4_URL     = 2;
    NL4_NETADDR = 3;
}

union netloc4 (netloc_type4) {
    NL4_NAME:    utf8str_cis;
    NL4_URL:     utf8str_cis;
    NL4_NETADDR: netaddr4;
}

enum change_attr_type4 : beu32 {
    NFS4_CHANGE_TYPE_IS_MONOTONIC_INCR         = 0;
    NFS4_CHANGE_TYPE_IS_VERSION_COUNTER        = 1;
    NFS4_CHANGE_TYPE_IS_VERSION_COUNTER_NOPNFS = 2;
    NFS4_CHANGE_TYPE_IS_TIME_METADATA          = 3;
    NFS4_CHANGE_TYPE_IS_UNDEFINED              = 4;
}

struct labelformat_spec4 {
    beu32 lfs_lfs;
    beu32 lfs_pi;
}

message sec_label4 {
    labelformat_spec4 slai_lfs;
    beu32             slai_data_len;
    u8                slai_data[slai_data_len];
    align(4);
}

message copy_from_auth_priv {
    secret4       cfap_shared_secret;
    netloc_type4  cfap_destination_type;
    netloc4       cfap_destination(cfap_destination_type);
    utf8str_mixed cfap_username;
}

message netloc4_entry {
    netloc_type4 disc;
    netloc4      value(disc);
}

message copy_to_auth_priv {
    secret4       ctap_shared_secret;
    beu32         ctap_source_count;
    netloc4_entry ctap_source[ctap_source_count];
    utf8str_mixed ctap_username;
}

message copy_confirm_auth_priv {
    beu32         ccap_shared_secret_mic_len;
    u8            ccap_shared_secret_mic[ccap_shared_secret_mic_len];
    align(4);
    utf8str_mixed ccap_username;
}

message app_data_block4 {
    beu64 adb_offset;
    beu64 adb_block_size;
    beu64 adb_block_count;
    beu64 adb_reloff_blocknum;
    beu32 adb_block_num;
    beu64 adb_reloff_pattern;
    beu32 adb_pattern_len;
    u8    adb_pattern[adb_pattern_len];
    align(4);
}

message data4 {
    beu64 d_offset;
    beu32 d_data_len;
    u8    d_data[d_data_len];
    align(4);
}

struct data_info4 {
    beu64 di_offset;
    beu64 di_length;
}

enum data_content4 : beu32 {
    NFS4_CONTENT_DATA = 0;
    NFS4_CONTENT_HOLE = 1;
}

enum stable_how4 : beu32 {
    UNSTABLE4  = 0;
    DATA_SYNC4 = 1;
    FILE_SYNC4 = 2;
}

message write_response4 {
    beu32       wr_callback_id_count;
    stateid4    wr_callback_id[wr_callback_id_count];
    beu64       wr_count;
    stable_how4 wr_committed;
    verifier4   wr_writeverf;
}

message xattrkey4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message xattrvalue4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message fattr4_owner {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message fattr4_owner_group {
    beu32 data_len;
    u8    data[data_len];
    align(4);
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

message cb_client4 {
    beu32    cb_program;
    netaddr4 cb_location;
}

message nfs_client_id4 {
    verifier4 verifier;
    beu32     id_len;
    u8        id[id_len];
    align(4);
}

message client_owner4 {
    verifier4 co_verifier;
    beu32     co_ownerid_len;
    u8        co_ownerid[co_ownerid_len];
    align(4);
}

message server_owner4 {
    beu64 so_minor_id;
    beu32 so_major_id_len;
    u8    so_major_id[so_major_id_len];
    align(4);
}

message state_owner4 {
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

enum ssv_subkey4 : beu32 {
    SSV4_SUBKEY_MIC_I2T  = 1;
    SSV4_SUBKEY_MIC_T2I  = 2;
    SSV4_SUBKEY_SEAL_I2T = 3;
    SSV4_SUBKEY_SEAL_T2I = 4;
}

message ssv_mic_plain_tkn4 {
    beu32 smpt_ssv_seq;
    beu32 smpt_orig_plain_len;
    u8    smpt_orig_plain[smpt_orig_plain_len];
    align(4);
}

message ssv_mic_tkn4 {
    beu32 smt_ssv_seq;
    beu32 smt_hmac_len;
    u8    smt_hmac[smt_hmac_len];
    align(4);
}

message ssv_seal_plain_tkn4 {
    xdr_opaque sspt_confounder;
    beu32      sspt_ssv_seq;
    xdr_opaque sspt_orig_plain;
    xdr_opaque sspt_pad;
}

message ssv_seal_cipher_tkn4 {
    beu32      ssct_ssv_seq;
    xdr_opaque ssct_iv;
    xdr_opaque ssct_encr_data;
    xdr_opaque ssct_hmac;
}

message fs_locations_server4 {
    bei32       fls_currency;
    beu32       fls_info_len;
    u8          fls_info[fls_info_len];
    align(4);
    utf8str_cis fls_server;
}

message fs_locations_item4 {
    beu32                fli_entries_count;
    fs_locations_server4 fli_entries[fli_entries_count];
    pathname4            fli_rootpath;
}

message fs_locations_info4 {
    beu32              fli_flags;
    bei32              fli_valid_for;
    pathname4          fli_fs_root;
    beu32              fli_items_count;
    fs_locations_item4 fli_items[fli_items_count];
}

enum filelayout_hint_care4 : beu32 {
    NFLH4_CARE_DENSE              = 0x00000001;
    NFLH4_CARE_COMMIT_THRU_MDS    = 0x00000002;
    NFL42_CARE_IO_ADVISE_THRU_MDS = 0x00000004;
    NFLH4_CARE_STRIPE_UNIT_SIZE   = 0x00000040;
    NFLH4_CARE_STRIPE_COUNT       = 0x00000080;
}

struct nfsv4_1_file_layouthint4 {
    beu32 nflh_care;
    beu32 nflh_util;
    beu32 nflh_stripe_count;
}

message multipath_list4 {
    beu32    count;
    netaddr4 data[count];
}

message nfsv4_1_file_layout_ds_addr4_nflda_multipath_ds_list {
    beu32           count;
    multipath_list4 data[count];
}

message nfsv4_1_file_layout_ds_addr4 {
    beu32                                                nflda_stripe_indices_count;
    beu32                                                nflda_stripe_indices[nflda_stripe_indices_count];
    nfsv4_1_file_layout_ds_addr4_nflda_multipath_ds_list nflda_multipath_ds_list;
}

message nfsv4_1_file_layout4 {
    deviceid4 nfl_deviceid;
    beu32     nfl_util;
    beu32     nfl_first_stripe_index;
    beu64     nfl_pattern_offset;
    beu32     nfl_fh_list_count;
    nfs_fh4   nfl_fh_list[nfl_fh_list_count];
}

enum nfs_opnum4 : beu32 {
    OP_ACCESS               = 3;
    OP_CLOSE                = 4;
    OP_COMMIT               = 5;
    OP_CREATE               = 6;
    OP_DELEGPURGE           = 7;
    OP_DELEGRETURN          = 8;
    OP_GETATTR              = 9;
    OP_GETFH                = 10;
    OP_LINK                 = 11;
    OP_LOCK                 = 12;
    OP_LOCKT                = 13;
    OP_LOCKU                = 14;
    OP_LOOKUP               = 15;
    OP_LOOKUPP              = 16;
    OP_NVERIFY              = 17;
    OP_OPEN                 = 18;
    OP_OPENATTR             = 19;
    OP_OPEN_CONFIRM         = 20;
    OP_OPEN_DOWNGRADE       = 21;
    OP_PUTFH                = 22;
    OP_PUTPUBFH             = 23;
    OP_PUTROOTFH            = 24;
    OP_READ                 = 25;
    OP_READDIR              = 26;
    OP_READLINK             = 27;
    OP_REMOVE               = 28;
    OP_RENAME               = 29;
    OP_RENEW                = 30;
    OP_RESTOREFH            = 31;
    OP_SAVEFH               = 32;
    OP_SECINFO              = 33;
    OP_SETATTR              = 34;
    OP_SETCLIENTID          = 35;
    OP_SETCLIENTID_CONFIRM  = 36;
    OP_VERIFY               = 37;
    OP_WRITE                = 38;
    OP_RELEASE_LOCKOWNER    = 39;
    OP_BACKCHANNEL_CTL      = 40;
    OP_BIND_CONN_TO_SESSION = 41;
    OP_EXCHANGE_ID          = 42;
    OP_CREATE_SESSION       = 43;
    OP_DESTROY_SESSION      = 44;
    OP_FREE_STATEID         = 45;
    OP_GET_DIR_DELEGATION   = 46;
    OP_GETDEVICEINFO        = 47;
    OP_GETDEVICELIST        = 48;
    OP_LAYOUTCOMMIT         = 49;
    OP_LAYOUTGET            = 50;
    OP_LAYOUTRETURN         = 51;
    OP_SECINFO_NO_NAME      = 52;
    OP_SEQUENCE             = 53;
    OP_SET_SSV              = 54;
    OP_TEST_STATEID         = 55;
    OP_WANT_DELEGATION      = 56;
    OP_DESTROY_CLIENTID     = 57;
    OP_RECLAIM_COMPLETE     = 58;
    OP_ALLOCATE             = 59;
    OP_COPY                 = 60;
    OP_COPY_NOTIFY          = 61;
    OP_DEALLOCATE           = 62;
    OP_IO_ADVISE            = 63;
    OP_LAYOUTERROR          = 64;
    OP_LAYOUTSTATS          = 65;
    OP_OFFLOAD_CANCEL       = 66;
    OP_OFFLOAD_STATUS       = 67;
    OP_READ_PLUS            = 68;
    OP_SEEK                 = 69;
    OP_WRITE_SAME           = 70;
    OP_CLONE                = 71;
    OP_GETXATTR             = 72;
    OP_SETXATTR             = 73;
    OP_LISTXATTRS           = 74;
    OP_REMOVEXATTR          = 75;
    OP_ILLEGAL              = 10044;
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

struct CLONE4args {
    stateid4 cl_src_stateid;
    stateid4 cl_dst_stateid;
    beu64    cl_src_offset;
    beu64    cl_dst_offset;
    beu64    cl_count;
}

struct CLONE4res {
    nfsstat4 cl_status;
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
    beu32        open_seqid;
    stateid4     open_stateid;
    beu32        lock_seqid;
    state_owner4 lock_owner;
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
    state_owner4   owner;
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
    state_owner4   owner;
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
    UNCHECKED4   = 0;
    GUARDED4     = 1;
    EXCLUSIVE4   = 2;
    EXCLUSIVE4_1 = 3;
}

message creatverfattr {
    verifier4 cva_verf;
    fattr4    cva_attrs;
}

union createhow4 (createmode4) {
    UNCHECKED4:   fattr4;
    GUARDED4:     fattr4;
    EXCLUSIVE4:   verifier4;
    EXCLUSIVE4_1: creatverfattr;
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
    OPEN_DELEGATE_NONE              = 0;
    OPEN_DELEGATE_READ              = 1;
    OPEN_DELEGATE_WRITE             = 2;
    OPEN_DELEGATE_NONE_EXT          = 3;
    OPEN_DELEGATE_READ_ATTRS_DELEG  = 4;
    OPEN_DELEGATE_WRITE_ATTRS_DELEG = 5;
}

enum open_claim_type4 : beu32 {
    CLAIM_NULL          = 0;
    CLAIM_PREVIOUS      = 1;
    CLAIM_DELEGATE_CUR  = 2;
    CLAIM_DELEGATE_PREV = 3;
    CLAIM_FH            = 4;
    CLAIM_DELEG_CUR_FH  = 5;
    CLAIM_DELEG_PREV_FH = 6;
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
    CLAIM_FH:            void;
    CLAIM_DELEG_PREV_FH: void;
    CLAIM_DELEG_CUR_FH:  stateid4;
}

message OPEN4args {
    beu32            seqid;
    beu32            share_access;
    beu32            share_deny;
    state_owner4     owner;
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

enum why_no_delegation4 : beu32 {
    WND4_NOT_WANTED                 = 0;
    WND4_CONTENTION                 = 1;
    WND4_RESOURCE                   = 2;
    WND4_NOT_SUPP_FTYPE             = 3;
    WND4_WRITE_DELEG_NOT_SUPP_FTYPE = 4;
    WND4_NOT_SUPP_UPGRADE           = 5;
    WND4_NOT_SUPP_DOWNGRADE         = 6;
    WND4_CANCELLED                  = 7;
    WND4_IS_DIR                     = 8;
}

struct open_none_delegation4_WND4_CONTENTION {
    beu32 value;
}

struct open_none_delegation4_WND4_RESOURCE {
    beu32 value;
}

union open_none_delegation4 (why_no_delegation4) {
    WND4_CONTENTION: open_none_delegation4_WND4_CONTENTION;
    WND4_RESOURCE:   open_none_delegation4_WND4_RESOURCE;
    default:         void;
}

message open_none_delegation4_entry {
    why_no_delegation4    disc;
    open_none_delegation4 value(disc);
}

union open_delegation4 (open_delegation_type4) {
    OPEN_DELEGATE_NONE:              void;
    OPEN_DELEGATE_READ:              open_read_delegation4;
    OPEN_DELEGATE_READ_ATTRS_DELEG:  open_read_delegation4;
    OPEN_DELEGATE_WRITE:             open_write_delegation4;
    OPEN_DELEGATE_WRITE_ATTRS_DELEG: open_write_delegation4;
    OPEN_DELEGATE_NONE_EXT:          open_none_delegation4_entry;
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
    NFS4ERR_CLID_INUSE: netaddr4;
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
    state_owner4 lock_owner;
}

struct RELEASE_LOCKOWNER4res {
    nfsstat4 status;
}

struct ILLEGAL4res {
    nfsstat4 status;
}

message gsshandle4_t {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message gss_cb_handles4 {
    rpc_gss_svc_t gcbp_service;
    gsshandle4_t  gcbp_handle_from_server;
    gsshandle4_t  gcbp_handle_from_client;
}

enum disc_callback_sec_parms4 : beu32 {
    AUTH_NONE = 0;
    AUTH_SYS  = 1;
}

union callback_sec_parms4 (disc_callback_sec_parms4) {
    AUTH_NONE:  void;
    AUTH_SYS:   authsys_parms;
    RPCSEC_GSS: gss_cb_handles4;
}

message callback_sec_parms4_entry {
    disc_callback_sec_parms4 disc;
    callback_sec_parms4      value(disc);
}

message BACKCHANNEL_CTL4args {
    beu32                     bca_cb_program;
    beu32                     bca_sec_parms_count;
    callback_sec_parms4_entry bca_sec_parms[bca_sec_parms_count];
}

struct BACKCHANNEL_CTL4res {
    nfsstat4 bcr_status;
}

enum channel_dir_from_client4 : beu32 {
    CDFC4_FORE         = 0x1;
    CDFC4_BACK         = 0x2;
    CDFC4_FORE_OR_BOTH = 0x3;
    CDFC4_BACK_OR_BOTH = 0x7;
}

struct BIND_CONN_TO_SESSION4args {
    sessionid4               bctsa_sessid;
    channel_dir_from_client4 bctsa_dir;
    beu32                    bctsa_use_conn_in_rdma_mode;
}

enum channel_dir_from_server4 : beu32 {
    CDFS4_FORE = 0x1;
    CDFS4_BACK = 0x2;
    CDFS4_BOTH = 0x3;
}

struct BIND_CONN_TO_SESSION4resok {
    sessionid4               bctsr_sessid;
    channel_dir_from_server4 bctsr_dir;
    beu32                    bctsr_use_conn_in_rdma_mode;
}

union BIND_CONN_TO_SESSION4res (nfsstat4) {
    NFS4_OK: BIND_CONN_TO_SESSION4resok;
    default: void;
}

message state_protect_ops4 {
    bitmap4 spo_must_enforce;
    bitmap4 spo_must_allow;
}

message ssv_sp_parms4_ssp_encr_algs {
    beu32    count;
    sec_oid4 data[count];
}

message ssv_sp_parms4 {
    state_protect_ops4          ssp_ops;
    beu32                       ssp_hash_algs_count;
    sec_oid4                    ssp_hash_algs[ssp_hash_algs_count];
    ssv_sp_parms4_ssp_encr_algs ssp_encr_algs;
    beu32                       ssp_window;
    beu32                       ssp_num_gss_handles;
}

enum state_protect_how4 : beu32 {
    SP4_NONE      = 0;
    SP4_MACH_CRED = 1;
    SP4_SSV       = 2;
}

union state_protect4_a (state_protect_how4) {
    SP4_NONE:      void;
    SP4_MACH_CRED: state_protect_ops4;
    SP4_SSV:       ssv_sp_parms4;
}

message EXCHANGE_ID4args {
    client_owner4      eia_clientowner;
    beu32              eia_flags;
    state_protect_how4 eia_state_protect_type;
    state_protect4_a   eia_state_protect(eia_state_protect_type);
    beu32              eia_client_impl_id_count;
    nfs_impl_id4       eia_client_impl_id[eia_client_impl_id_count];
}

message ssv_prot_info4 {
    state_protect_ops4 spi_ops;
    beu32              spi_hash_alg;
    beu32              spi_encr_alg;
    beu32              spi_ssv_len;
    beu32              spi_window;
    beu32              spi_handles_count;
    gsshandle4_t       spi_handles[spi_handles_count];
}

union state_protect4_r (state_protect_how4) {
    SP4_NONE:      void;
    SP4_MACH_CRED: state_protect_ops4;
    SP4_SSV:       ssv_prot_info4;
}

message EXCHANGE_ID4resok_eir_server_impl_id {
    beu32        count;
    nfs_impl_id4 data[count];
}

message EXCHANGE_ID4resok {
    beu64                                eir_clientid;
    beu32                                eir_sequenceid;
    beu32                                eir_flags;
    state_protect_how4                   eir_state_protect_type;
    state_protect4_r                     eir_state_protect(eir_state_protect_type);
    server_owner4                        eir_server_owner;
    beu32                                eir_server_scope_len;
    u8                                   eir_server_scope[eir_server_scope_len];
    align(4);
    EXCHANGE_ID4resok_eir_server_impl_id eir_server_impl_id;
}

union EXCHANGE_ID4res (nfsstat4) {
    NFS4_OK: EXCHANGE_ID4resok;
    default: void;
}

message channel_attrs4 {
    beu32 ca_headerpadsize;
    beu32 ca_maxrequestsize;
    beu32 ca_maxresponsesize;
    beu32 ca_maxresponsesize_cached;
    beu32 ca_maxoperations;
    beu32 ca_maxrequests;
    beu32 ca_rdma_ird_count;
    beu32 ca_rdma_ird[ca_rdma_ird_count];
}

message CREATE_SESSION4args {
    beu64                     csa_clientid;
    beu32                     csa_sequence;
    beu32                     csa_flags;
    channel_attrs4            csa_fore_chan_attrs;
    channel_attrs4            csa_back_chan_attrs;
    beu32                     csa_cb_program;
    beu32                     csa_sec_parms_count;
    callback_sec_parms4_entry csa_sec_parms[csa_sec_parms_count];
}

message CREATE_SESSION4resok {
    sessionid4     csr_sessionid;
    beu32          csr_sequence;
    beu32          csr_flags;
    channel_attrs4 csr_fore_chan_attrs;
    channel_attrs4 csr_back_chan_attrs;
}

union CREATE_SESSION4res (nfsstat4) {
    NFS4_OK: CREATE_SESSION4resok;
    default: void;
}

struct DESTROY_SESSION4args {
    sessionid4 dsa_sessionid;
}

struct DESTROY_SESSION4res {
    nfsstat4 dsr_status;
}

struct FREE_STATEID4args {
    stateid4 fsa_stateid;
}

struct FREE_STATEID4res {
    nfsstat4 fsr_status;
}

message GET_DIR_DELEGATION4args {
    beu32    gdda_signal_deleg_avail;
    bitmap4  gdda_notification_types;
    nfstime4 gdda_child_attr_delay;
    nfstime4 gdda_dir_attr_delay;
    bitmap4  gdda_child_attributes;
    bitmap4  gdda_dir_attributes;
}

message GET_DIR_DELEGATION4resok {
    verifier4 gddr_cookieverf;
    stateid4  gddr_stateid;
    bitmap4   gddr_notification;
    bitmap4   gddr_child_attributes;
    bitmap4   gddr_dir_attributes;
}

enum gddrnf4_status : beu32 {
    GDD4_OK      = 0;
    GDD4_UNAVAIL = 1;
}

struct GET_DIR_DELEGATION4res_non_fatal_GDD4_UNAVAIL {
    beu32 value;
}

union GET_DIR_DELEGATION4res_non_fatal (gddrnf4_status) {
    GDD4_OK:      GET_DIR_DELEGATION4resok;
    GDD4_UNAVAIL: GET_DIR_DELEGATION4res_non_fatal_GDD4_UNAVAIL;
}

message GET_DIR_DELEGATION4res_non_fatal_entry {
    gddrnf4_status                   disc;
    GET_DIR_DELEGATION4res_non_fatal value(disc);
}

union GET_DIR_DELEGATION4res (nfsstat4) {
    NFS4_OK: GET_DIR_DELEGATION4res_non_fatal_entry;
    default: void;
}

message GETDEVICEINFO4args {
    deviceid4   gdia_device_id;
    layouttype4 gdia_layout_type;
    beu32       gdia_maxcount;
    bitmap4     gdia_notify_types;
}

message GETDEVICEINFO4resok {
    device_addr4 gdir_device_addr;
    bitmap4      gdir_notification;
}

struct GETDEVICEINFO4res_NFS4ERR_TOOSMALL {
    beu32 value;
}

union GETDEVICEINFO4res (nfsstat4) {
    NFS4_OK:          GETDEVICEINFO4resok;
    NFS4ERR_TOOSMALL: GETDEVICEINFO4res_NFS4ERR_TOOSMALL;
    default:          void;
}

struct GETDEVICELIST4args {
    layouttype4 gdla_layout_type;
    beu32       gdla_maxdevices;
    beu64       gdla_cookie;
    verifier4   gdla_cookieverf;
}

message GETDEVICELIST4resok {
    beu64     gdlr_cookie;
    verifier4 gdlr_cookieverf;
    beu32     gdlr_deviceid_list_count;
    deviceid4 gdlr_deviceid_list[gdlr_deviceid_list_count];
    beu32     gdlr_eof;
}

union GETDEVICELIST4res (nfsstat4) {
    NFS4_OK: GETDEVICELIST4resok;
    default: void;
}

union newtime4 (xdr_bool) {
    TRUE:  nfstime4;
    FALSE: void;
}

struct newoffset4_TRUE {
    beu64 value;
}

union newoffset4 (xdr_bool) {
    TRUE:  newoffset4_TRUE;
    FALSE: void;
}

message LAYOUTCOMMIT4args {
    beu64         loca_offset;
    beu64         loca_length;
    beu32         loca_reclaim;
    stateid4      loca_stateid;
    xdr_bool      loca_last_write_offset_type;
    newoffset4    loca_last_write_offset(loca_last_write_offset_type);
    xdr_bool      loca_time_modify_type;
    newtime4      loca_time_modify(loca_time_modify_type);
    layoutupdate4 loca_layoutupdate;
}

struct newsize4_TRUE {
    beu64 value;
}

union newsize4 (xdr_bool) {
    TRUE:  newsize4_TRUE;
    FALSE: void;
}

message LAYOUTCOMMIT4resok {
    xdr_bool locr_newsize_type;
    newsize4 locr_newsize(locr_newsize_type);
}

union LAYOUTCOMMIT4res (nfsstat4) {
    NFS4_OK: LAYOUTCOMMIT4resok;
    default: void;
}

struct LAYOUTGET4args {
    beu32         loga_signal_layout_avail;
    layouttype4   loga_layout_type;
    layoutiomode4 loga_iomode;
    beu64         loga_offset;
    beu64         loga_length;
    beu64         loga_minlength;
    stateid4      loga_stateid;
    beu32         loga_maxcount;
}

message LAYOUTGET4resok {
    beu32    logr_return_on_close;
    stateid4 logr_stateid;
    beu32    logr_layout_count;
    layout4  logr_layout[logr_layout_count];
}

struct LAYOUTGET4res_NFS4ERR_LAYOUTTRYLATER {
    beu32 value;
}

union LAYOUTGET4res (nfsstat4) {
    NFS4_OK:                LAYOUTGET4resok;
    NFS4ERR_LAYOUTTRYLATER: LAYOUTGET4res_NFS4ERR_LAYOUTTRYLATER;
    default:                void;
}

message LAYOUTRETURN4args {
    beu32              lora_reclaim;
    layouttype4        lora_layout_type;
    layoutiomode4      lora_iomode;
    layoutreturn_type4 lora_layoutreturn_type;
    layoutreturn4      lora_layoutreturn(lora_layoutreturn_type);
}

union layoutreturn_stateid (xdr_bool) {
    TRUE:  stateid4;
    FALSE: void;
}

message layoutreturn_stateid_entry {
    xdr_bool             disc;
    layoutreturn_stateid value(disc);
}

union LAYOUTRETURN4res (nfsstat4) {
    NFS4_OK: layoutreturn_stateid_entry;
    default: void;
}

enum secinfo_style4 : beu32 {
    SECINFO_STYLE4_CURRENT_FH = 0;
    SECINFO_STYLE4_PARENT     = 1;
}

struct SEQUENCE4args {
    sessionid4 sa_sessionid;
    beu32      sa_sequenceid;
    beu32      sa_slotid;
    beu32      sa_highest_slotid;
    beu32      sa_cachethis;
}

struct SEQUENCE4resok {
    sessionid4 sr_sessionid;
    beu32      sr_sequenceid;
    beu32      sr_slotid;
    beu32      sr_highest_slotid;
    beu32      sr_target_highest_slotid;
    beu32      sr_status_flags;
}

union SEQUENCE4res (nfsstat4) {
    NFS4_OK: SEQUENCE4resok;
    default: void;
}

struct ssa_digest_input4 {
    SEQUENCE4args sdi_seqargs;
}

message SET_SSV4args {
    xdr_opaque ssa_ssv;
    xdr_opaque ssa_digest;
}

message ssr_digest_input4 {
    nfsstat4     sdi_seqres_type;
    SEQUENCE4res sdi_seqres(sdi_seqres_type);
}

message SET_SSV4resok {
    beu32 ssr_digest_len;
    u8    ssr_digest[ssr_digest_len];
    align(4);
}

union SET_SSV4res (nfsstat4) {
    NFS4_OK: SET_SSV4resok;
    default: void;
}

message TEST_STATEID4args {
    beu32    ts_stateids_count;
    stateid4 ts_stateids[ts_stateids_count];
}

message TEST_STATEID4resok {
    beu32    tsr_status_codes_count;
    nfsstat4 tsr_status_codes[tsr_status_codes_count];
}

union TEST_STATEID4res (nfsstat4) {
    NFS4_OK: TEST_STATEID4resok;
    default: void;
}

struct deleg_claim4_CLAIM_PREVIOUS {
    open_delegation_type4 value;
}

union deleg_claim4 (open_claim_type4) {
    CLAIM_FH:            void;
    CLAIM_DELEG_PREV_FH: void;
    CLAIM_PREVIOUS:      deleg_claim4_CLAIM_PREVIOUS;
}

message WANT_DELEGATION4args {
    beu32            wda_want;
    open_claim_type4 wda_claim_type;
    deleg_claim4     wda_claim(wda_claim_type);
}

message open_delegation4_entry {
    open_delegation_type4 disc;
    open_delegation4      value(disc);
}

union WANT_DELEGATION4res (nfsstat4) {
    NFS4_OK: open_delegation4_entry;
    default: void;
}

struct DESTROY_CLIENTID4args {
    beu64 dca_clientid;
}

struct DESTROY_CLIENTID4res {
    nfsstat4 dcr_status;
}

struct RECLAIM_COMPLETE4args {
    beu32 rca_one_fs;
}

struct RECLAIM_COMPLETE4res {
    nfsstat4 rcr_status;
}

message COPY4args {
    stateid4      ca_src_stateid;
    stateid4      ca_dst_stateid;
    beu64         ca_src_offset;
    beu64         ca_dst_offset;
    beu64         ca_count;
    beu32         ca_consecutive;
    beu32         ca_synchronous;
    beu32         ca_source_server_count;
    netloc4_entry ca_source_server[ca_source_server_count];
}

struct copy_requirements4 {
    beu32 cr_consecutive;
    beu32 cr_synchronous;
}

message COPY4resok {
    write_response4    cr_response;
    copy_requirements4 cr_requirements;
}

union COPY4res (nfsstat4) {
    NFS4_OK:                 COPY4resok;
    NFS4ERR_OFFLOAD_NO_REQS: copy_requirements4;
    default:                 void;
}

message COPY_NOTIFY4args {
    stateid4     cna_src_stateid;
    netloc_type4 cna_destination_server_type;
    netloc4      cna_destination_server(cna_destination_server_type);
}

message COPY_NOTIFY4resok {
    nfstime4      cnr_lease_time;
    stateid4      cnr_stateid;
    beu32         cnr_source_server_count;
    netloc4_entry cnr_source_server[cnr_source_server_count];
}

union COPY_NOTIFY4res (nfsstat4) {
    NFS4_OK: COPY_NOTIFY4resok;
    default: void;
}

struct OFFLOAD_CANCEL4args {
    stateid4 oca_stateid;
}

struct OFFLOAD_CANCEL4res {
    nfsstat4 ocr_status;
}

struct OFFLOAD_STATUS4args {
    stateid4 osa_stateid;
}

message OFFLOAD_STATUS4resok {
    beu64    osr_count;
    beu32    osr_complete_count;
    nfsstat4 osr_complete[osr_complete_count];
}

union OFFLOAD_STATUS4res (nfsstat4) {
    NFS4_OK: OFFLOAD_STATUS4resok;
    default: void;
}

struct ALLOCATE4args {
    stateid4 aa_stateid;
    beu64    aa_offset;
    beu64    aa_length;
}

struct ALLOCATE4res {
    nfsstat4 ar_status;
}

struct DEALLOCATE4args {
    stateid4 da_stateid;
    beu64    da_offset;
    beu64    da_length;
}

struct DEALLOCATE4res {
    nfsstat4 dr_status;
}

enum IO_ADVISE_type4 : beu32 {
    IO_ADVISE4_NORMAL                 = 0;
    IO_ADVISE4_SEQUENTIAL             = 1;
    IO_ADVISE4_SEQUENTIAL_BACKWARDS   = 2;
    IO_ADVISE4_RANDOM                 = 3;
    IO_ADVISE4_WILLNEED               = 4;
    IO_ADVISE4_WILLNEED_OPPORTUNISTIC = 5;
    IO_ADVISE4_DONTNEED               = 6;
    IO_ADVISE4_NOREUSE                = 7;
    IO_ADVISE4_READ                   = 8;
    IO_ADVISE4_WRITE                  = 9;
    IO_ADVISE4_INIT_PROXIMITY         = 10;
}

message IO_ADVISE4args {
    stateid4 iaa_stateid;
    beu64    iaa_offset;
    beu64    iaa_count;
    bitmap4  iaa_hints;
}

message IO_ADVISE4resok {
    bitmap4 ior_hints;
}

union IO_ADVISE4res (nfsstat4) {
    NFS4_OK: IO_ADVISE4resok;
    default: void;
}

struct device_error4 {
    deviceid4  de_deviceid;
    nfsstat4   de_status;
    nfs_opnum4 de_opnum;
}

message LAYOUTERROR4args {
    beu64         lea_offset;
    beu64         lea_length;
    stateid4      lea_stateid;
    beu32         lea_errors_count;
    device_error4 lea_errors[lea_errors_count];
}

struct LAYOUTERROR4res {
    nfsstat4 ler_status;
}

struct io_info4 {
    beu64 ii_count;
    beu64 ii_bytes;
}

message LAYOUTSTATS4args {
    beu64         lsa_offset;
    beu64         lsa_length;
    stateid4      lsa_stateid;
    io_info4      lsa_read;
    io_info4      lsa_write;
    deviceid4     lsa_deviceid;
    layoutupdate4 lsa_layoutupdate;
}

struct LAYOUTSTATS4res {
    nfsstat4 lsr_status;
}

struct READ_PLUS4args {
    stateid4 rpa_stateid;
    beu64    rpa_offset;
    beu32    rpa_count;
}

union read_plus_content (data_content4) {
    NFS4_CONTENT_DATA: data4;
    NFS4_CONTENT_HOLE: data_info4;
    default:           void;
}

message read_plus_content_entry {
    data_content4     disc;
    read_plus_content value(disc);
}

message read_plus_res4 {
    beu32                   rpr_eof;
    beu32                   rpr_contents_count;
    read_plus_content_entry rpr_contents[rpr_contents_count];
}

union READ_PLUS4res (nfsstat4) {
    NFS4_OK: read_plus_res4;
    default: void;
}

struct SEEK4args {
    stateid4      sa_stateid;
    beu64         sa_offset;
    data_content4 sa_what;
}

struct seek_res4 {
    beu32 sr_eof;
    beu64 sr_offset;
}

union SEEK4res (nfsstat4) {
    NFS4_OK: seek_res4;
    default: void;
}

message WRITE_SAME4args {
    stateid4        wsa_stateid;
    stable_how4     wsa_stable;
    app_data_block4 wsa_adb;
}

union WRITE_SAME4res (nfsstat4) {
    NFS4_OK: write_response4;
    default: void;
}

struct ff_device_versions4 {
    beu32 ffdv_version;
    beu32 ffdv_minorversion;
    beu32 ffdv_rsize;
    beu32 ffdv_wsize;
    beu32 ffdv_tightly_coupled;
}

message ff_device_addr4 {
    multipath_list4     ffda_netaddrs;
    beu32               ffda_versions_count;
    ff_device_versions4 ffda_versions[ffda_versions_count];
}

message ff_data_server4 {
    deviceid4          ffds_deviceid;
    beu32              ffds_efficiency;
    stateid4           ffds_stateid;
    beu32              ffds_fh_vers_count;
    nfs_fh4            ffds_fh_vers[ffds_fh_vers_count];
    fattr4_owner       ffds_user;
    fattr4_owner_group ffds_group;
}

message ff_mirror4 {
    beu32           ffm_data_servers_count;
    ff_data_server4 ffm_data_servers[ffm_data_servers_count];
}

message ff_layout4 {
    beu64      ffl_stripe_unit;
    beu32      ffl_mirrors_count;
    ff_mirror4 ffl_mirrors[ffl_mirrors_count];
    beu32      ffl_flags;
    beu32      ffl_stats_collect_hint;
}

message ff_ioerr4 {
    beu64         ffie_offset;
    beu64         ffie_length;
    stateid4      ffie_stateid;
    beu32         ffie_errors_count;
    device_error4 ffie_errors[ffie_errors_count];
}

struct ff_io_latency4 {
    beu64    ffil_ops_requested;
    beu64    ffil_bytes_requested;
    beu64    ffil_ops_completed;
    beu64    ffil_bytes_completed;
    beu64    ffil_bytes_not_delivered;
    nfstime4 ffil_total_busy_time;
    nfstime4 ffil_aggregate_completion_time;
}

message ff_layoutupdate4 {
    netaddr4       ffl_addr;
    nfs_fh4        ffl_fhandle;
    ff_io_latency4 ffl_read;
    ff_io_latency4 ffl_write;
    nfstime4       ffl_duration;
    beu32          ffl_local;
}

message ff_iostats4 {
    beu64            ffis_offset;
    beu64            ffis_length;
    stateid4         ffis_stateid;
    io_info4         ffis_read;
    io_info4         ffis_write;
    deviceid4        ffis_deviceid;
    ff_layoutupdate4 ffis_layoutupdate;
}

message ff_layoutreturn4_fflr_iostats_report {
    beu32       count;
    ff_iostats4 data[count];
}

message ff_layoutreturn4 {
    beu32                                fflr_ioerr_report_count;
    ff_ioerr4                            fflr_ioerr_report[fflr_ioerr_report_count];
    ff_layoutreturn4_fflr_iostats_report fflr_iostats_report;
}

struct ff_mirrors_hint_TRUE {
    beu32 value;
}

union ff_mirrors_hint (xdr_bool) {
    TRUE:  ff_mirrors_hint_TRUE;
    FALSE: void;
}

message ff_layouthint4 {
    xdr_bool        fflh_mirrors_hint_type;
    ff_mirrors_hint fflh_mirrors_hint(fflh_mirrors_hint_type);
}

enum ff_cb_recall_any_mask : beu32 {
    FF_RCA4_TYPE_MASK_READ = 2;
    FF_RCA4_TYPE_MASK_RW   = 1;
}

message GETXATTR4args {
    xattrkey4 gxa_name;
}

union GETXATTR4res (nfsstat4) {
    NFS4_OK: xattrvalue4;
    default: void;
}

enum setxattr_option4 : beu32 {
    SETXATTR4_EITHER  = 0;
    SETXATTR4_CREATE  = 1;
    SETXATTR4_REPLACE = 2;
}

message SETXATTR4args {
    setxattr_option4 sxa_option;
    xattrkey4        sxa_key;
    xattrvalue4      sxa_value;
}

union SETXATTR4res (nfsstat4) {
    NFS4_OK: change_info4;
    default: void;
}

struct LISTXATTRS4args {
    beu64 lxa_cookie;
    beu32 lxa_maxcount;
}

message LISTXATTRS4resok {
    beu64     lxr_cookie;
    beu32     lxr_names_count;
    xattrkey4 lxr_names[lxr_names_count];
    beu32     lxr_eof;
}

union LISTXATTRS4res (nfsstat4) {
    NFS4_OK: LISTXATTRS4resok;
    default: void;
}

message REMOVEXATTR4args {
    xattrkey4 rxa_name;
}

union REMOVEXATTR4res (nfsstat4) {
    NFS4_OK: change_info4;
    default: void;
}

struct nfs_argop4_OP_SECINFO_NO_NAME {
    secinfo_style4 value;
}

union nfs_argop4 (nfs_opnum4) {
    OP_ACCESS:               ACCESS4args;
    OP_CLOSE:                CLOSE4args;
    OP_COMMIT:               COMMIT4args;
    OP_CREATE:               CREATE4args;
    OP_DELEGPURGE:           DELEGPURGE4args;
    OP_DELEGRETURN:          DELEGRETURN4args;
    OP_GETATTR:              GETATTR4args;
    OP_GETFH:                void;
    OP_LINK:                 LINK4args;
    OP_LOCK:                 LOCK4args;
    OP_LOCKT:                LOCKT4args;
    OP_LOCKU:                LOCKU4args;
    OP_LOOKUP:               LOOKUP4args;
    OP_LOOKUPP:              void;
    OP_NVERIFY:              NVERIFY4args;
    OP_OPEN:                 OPEN4args;
    OP_OPENATTR:             OPENATTR4args;
    OP_OPEN_CONFIRM:         OPEN_CONFIRM4args;
    OP_OPEN_DOWNGRADE:       OPEN_DOWNGRADE4args;
    OP_PUTFH:                PUTFH4args;
    OP_PUTPUBFH:             void;
    OP_PUTROOTFH:            void;
    OP_READ:                 READ4args;
    OP_READDIR:              READDIR4args;
    OP_READLINK:             void;
    OP_REMOVE:               REMOVE4args;
    OP_RENAME:               RENAME4args;
    OP_RENEW:                RENEW4args;
    OP_RESTOREFH:            void;
    OP_SAVEFH:               void;
    OP_SECINFO:              SECINFO4args;
    OP_SETATTR:              SETATTR4args;
    OP_SETCLIENTID:          SETCLIENTID4args;
    OP_SETCLIENTID_CONFIRM:  SETCLIENTID_CONFIRM4args;
    OP_VERIFY:               VERIFY4args;
    OP_WRITE:                WRITE4args;
    OP_RELEASE_LOCKOWNER:    RELEASE_LOCKOWNER4args;
    OP_BACKCHANNEL_CTL:      BACKCHANNEL_CTL4args;
    OP_BIND_CONN_TO_SESSION: BIND_CONN_TO_SESSION4args;
    OP_EXCHANGE_ID:          EXCHANGE_ID4args;
    OP_CREATE_SESSION:       CREATE_SESSION4args;
    OP_DESTROY_SESSION:      DESTROY_SESSION4args;
    OP_FREE_STATEID:         FREE_STATEID4args;
    OP_GET_DIR_DELEGATION:   GET_DIR_DELEGATION4args;
    OP_GETDEVICEINFO:        GETDEVICEINFO4args;
    OP_GETDEVICELIST:        GETDEVICELIST4args;
    OP_LAYOUTCOMMIT:         LAYOUTCOMMIT4args;
    OP_LAYOUTGET:            LAYOUTGET4args;
    OP_LAYOUTRETURN:         LAYOUTRETURN4args;
    OP_SECINFO_NO_NAME:      nfs_argop4_OP_SECINFO_NO_NAME;
    OP_SEQUENCE:             SEQUENCE4args;
    OP_SET_SSV:              SET_SSV4args;
    OP_TEST_STATEID:         TEST_STATEID4args;
    OP_WANT_DELEGATION:      WANT_DELEGATION4args;
    OP_DESTROY_CLIENTID:     DESTROY_CLIENTID4args;
    OP_RECLAIM_COMPLETE:     RECLAIM_COMPLETE4args;
    OP_ALLOCATE:             ALLOCATE4args;
    OP_COPY:                 COPY4args;
    OP_COPY_NOTIFY:          COPY_NOTIFY4args;
    OP_DEALLOCATE:           DEALLOCATE4args;
    OP_IO_ADVISE:            IO_ADVISE4args;
    OP_LAYOUTERROR:          LAYOUTERROR4args;
    OP_LAYOUTSTATS:          LAYOUTSTATS4args;
    OP_OFFLOAD_CANCEL:       OFFLOAD_CANCEL4args;
    OP_OFFLOAD_STATUS:       OFFLOAD_STATUS4args;
    OP_READ_PLUS:            READ_PLUS4args;
    OP_SEEK:                 SEEK4args;
    OP_WRITE_SAME:           WRITE_SAME4args;
    OP_CLONE:                CLONE4args;
    OP_GETXATTR:             GETXATTR4args;
    OP_SETXATTR:             SETXATTR4args;
    OP_LISTXATTRS:           LISTXATTRS4args;
    OP_REMOVEXATTR:          REMOVEXATTR4args;
    OP_ILLEGAL:              void;
    default:                 void;
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

message BIND_CONN_TO_SESSION4res_entry {
    nfsstat4                 disc;
    BIND_CONN_TO_SESSION4res value(disc);
}

message EXCHANGE_ID4res_entry {
    nfsstat4        disc;
    EXCHANGE_ID4res value(disc);
}

message CREATE_SESSION4res_entry {
    nfsstat4           disc;
    CREATE_SESSION4res value(disc);
}

message GET_DIR_DELEGATION4res_entry {
    nfsstat4               disc;
    GET_DIR_DELEGATION4res value(disc);
}

message GETDEVICEINFO4res_entry {
    nfsstat4          disc;
    GETDEVICEINFO4res value(disc);
}

message GETDEVICELIST4res_entry {
    nfsstat4          disc;
    GETDEVICELIST4res value(disc);
}

message LAYOUTCOMMIT4res_entry {
    nfsstat4         disc;
    LAYOUTCOMMIT4res value(disc);
}

message LAYOUTGET4res_entry {
    nfsstat4      disc;
    LAYOUTGET4res value(disc);
}

message LAYOUTRETURN4res_entry {
    nfsstat4         disc;
    LAYOUTRETURN4res value(disc);
}

message SEQUENCE4res_entry {
    nfsstat4     disc;
    SEQUENCE4res value(disc);
}

message SET_SSV4res_entry {
    nfsstat4    disc;
    SET_SSV4res value(disc);
}

message TEST_STATEID4res_entry {
    nfsstat4         disc;
    TEST_STATEID4res value(disc);
}

message WANT_DELEGATION4res_entry {
    nfsstat4            disc;
    WANT_DELEGATION4res value(disc);
}

message COPY4res_entry {
    nfsstat4 disc;
    COPY4res value(disc);
}

message COPY_NOTIFY4res_entry {
    nfsstat4        disc;
    COPY_NOTIFY4res value(disc);
}

message IO_ADVISE4res_entry {
    nfsstat4      disc;
    IO_ADVISE4res value(disc);
}

message OFFLOAD_STATUS4res_entry {
    nfsstat4           disc;
    OFFLOAD_STATUS4res value(disc);
}

message READ_PLUS4res_entry {
    nfsstat4      disc;
    READ_PLUS4res value(disc);
}

message SEEK4res_entry {
    nfsstat4 disc;
    SEEK4res value(disc);
}

message WRITE_SAME4res_entry {
    nfsstat4       disc;
    WRITE_SAME4res value(disc);
}

message GETXATTR4res_entry {
    nfsstat4     disc;
    GETXATTR4res value(disc);
}

message SETXATTR4res_entry {
    nfsstat4     disc;
    SETXATTR4res value(disc);
}

message LISTXATTRS4res_entry {
    nfsstat4       disc;
    LISTXATTRS4res value(disc);
}

message REMOVEXATTR4res_entry {
    nfsstat4        disc;
    REMOVEXATTR4res value(disc);
}

union nfs_resop4 (nfs_opnum4) {
    OP_ACCESS:               ACCESS4res_entry;
    OP_CLOSE:                CLOSE4res_entry;
    OP_COMMIT:               COMMIT4res_entry;
    OP_CREATE:               CREATE4res_entry;
    OP_DELEGPURGE:           DELEGPURGE4res;
    OP_DELEGRETURN:          DELEGRETURN4res;
    OP_GETATTR:              GETATTR4res_entry;
    OP_GETFH:                GETFH4res_entry;
    OP_LINK:                 LINK4res_entry;
    OP_LOCK:                 LOCK4res_entry;
    OP_LOCKT:                LOCKT4res_entry;
    OP_LOCKU:                LOCKU4res_entry;
    OP_LOOKUP:               LOOKUP4res;
    OP_LOOKUPP:              LOOKUPP4res;
    OP_NVERIFY:              NVERIFY4res;
    OP_OPEN:                 OPEN4res_entry;
    OP_OPENATTR:             OPENATTR4res;
    OP_OPEN_CONFIRM:         OPEN_CONFIRM4res_entry;
    OP_OPEN_DOWNGRADE:       OPEN_DOWNGRADE4res_entry;
    OP_PUTFH:                PUTFH4res;
    OP_PUTPUBFH:             PUTPUBFH4res;
    OP_PUTROOTFH:            PUTROOTFH4res;
    OP_READ:                 READ4res_entry;
    OP_READDIR:              READDIR4res_entry;
    OP_READLINK:             READLINK4res_entry;
    OP_REMOVE:               REMOVE4res_entry;
    OP_RENAME:               RENAME4res_entry;
    OP_RENEW:                RENEW4res;
    OP_RESTOREFH:            RESTOREFH4res;
    OP_SAVEFH:               SAVEFH4res;
    OP_SECINFO:              SECINFO4res_entry;
    OP_SETATTR:              SETATTR4res;
    OP_SETCLIENTID:          SETCLIENTID4res_entry;
    OP_SETCLIENTID_CONFIRM:  SETCLIENTID_CONFIRM4res;
    OP_VERIFY:               VERIFY4res;
    OP_WRITE:                WRITE4res_entry;
    OP_RELEASE_LOCKOWNER:    RELEASE_LOCKOWNER4res;
    OP_BACKCHANNEL_CTL:      BACKCHANNEL_CTL4res;
    OP_BIND_CONN_TO_SESSION: BIND_CONN_TO_SESSION4res_entry;
    OP_EXCHANGE_ID:          EXCHANGE_ID4res_entry;
    OP_CREATE_SESSION:       CREATE_SESSION4res_entry;
    OP_DESTROY_SESSION:      DESTROY_SESSION4res;
    OP_FREE_STATEID:         FREE_STATEID4res;
    OP_GET_DIR_DELEGATION:   GET_DIR_DELEGATION4res_entry;
    OP_GETDEVICEINFO:        GETDEVICEINFO4res_entry;
    OP_GETDEVICELIST:        GETDEVICELIST4res_entry;
    OP_LAYOUTCOMMIT:         LAYOUTCOMMIT4res_entry;
    OP_LAYOUTGET:            LAYOUTGET4res_entry;
    OP_LAYOUTRETURN:         LAYOUTRETURN4res_entry;
    OP_SECINFO_NO_NAME:      SECINFO4res_entry;
    OP_SEQUENCE:             SEQUENCE4res_entry;
    OP_SET_SSV:              SET_SSV4res_entry;
    OP_TEST_STATEID:         TEST_STATEID4res_entry;
    OP_WANT_DELEGATION:      WANT_DELEGATION4res_entry;
    OP_DESTROY_CLIENTID:     DESTROY_CLIENTID4res;
    OP_RECLAIM_COMPLETE:     RECLAIM_COMPLETE4res;
    OP_ALLOCATE:             ALLOCATE4res;
    OP_COPY:                 COPY4res_entry;
    OP_COPY_NOTIFY:          COPY_NOTIFY4res_entry;
    OP_DEALLOCATE:           DEALLOCATE4res;
    OP_IO_ADVISE:            IO_ADVISE4res_entry;
    OP_LAYOUTERROR:          LAYOUTERROR4res;
    OP_LAYOUTSTATS:          LAYOUTSTATS4res;
    OP_OFFLOAD_CANCEL:       OFFLOAD_CANCEL4res;
    OP_OFFLOAD_STATUS:       OFFLOAD_STATUS4res_entry;
    OP_READ_PLUS:            READ_PLUS4res_entry;
    OP_SEEK:                 SEEK4res_entry;
    OP_WRITE_SAME:           WRITE_SAME4res_entry;
    OP_CLONE:                CLONE4res;
    OP_GETXATTR:             GETXATTR4res_entry;
    OP_SETXATTR:             SETXATTR4res_entry;
    OP_LISTXATTRS:           LISTXATTRS4res_entry;
    OP_REMOVEXATTR:          REMOVEXATTR4res_entry;
    OP_ILLEGAL:              ILLEGAL4res;
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

enum layoutrecall_type4 : beu32 {
    LAYOUTRECALL4_FILE = 1;
    LAYOUTRECALL4_FSID = 2;
    LAYOUTRECALL4_ALL  = 3;
}

message layoutrecall_file4 {
    nfs_fh4  lor_fh;
    beu64    lor_offset;
    beu64    lor_length;
    stateid4 lor_stateid;
}

union layoutrecall4 (layoutrecall_type4) {
    LAYOUTRECALL4_FILE: layoutrecall_file4;
    LAYOUTRECALL4_FSID: fsid4;
    LAYOUTRECALL4_ALL:  void;
}

message CB_LAYOUTRECALL4args {
    layouttype4        clora_type;
    layoutiomode4      clora_iomode;
    beu32              clora_changed;
    layoutrecall_type4 clora_recall_type;
    layoutrecall4      clora_recall(clora_recall_type);
}

struct CB_LAYOUTRECALL4res {
    nfsstat4 clorr_status;
}

enum notify_type4 : beu32 {
    NOTIFY4_CHANGE_CHILD_ATTRS     = 0;
    NOTIFY4_CHANGE_DIR_ATTRS       = 1;
    NOTIFY4_REMOVE_ENTRY           = 2;
    NOTIFY4_ADD_ENTRY              = 3;
    NOTIFY4_RENAME_ENTRY           = 4;
    NOTIFY4_CHANGE_COOKIE_VERIFIER = 5;
}

message notify_entry4 {
    component4 ne_file;
    fattr4     ne_attrs;
}

message prev_entry4 {
    notify_entry4 pe_prev_entry;
    beu64         pe_prev_entry_cookie;
}

message notify_remove4 {
    notify_entry4 nrm_old_entry;
    beu64         nrm_old_entry_cookie;
}

message notify_add4_nad_new_entry_cookie {
    beu32 count;
    beu64 data[count];
}

message notify_add4_nad_prev_entry {
    beu32       count;
    prev_entry4 data[count];
}

message notify_add4 {
    beu32                            nad_old_entry_count;
    notify_remove4                   nad_old_entry[nad_old_entry_count];
    notify_entry4                    nad_new_entry;
    notify_add4_nad_new_entry_cookie nad_new_entry_cookie;
    notify_add4_nad_prev_entry       nad_prev_entry;
    beu32                            nad_last_entry;
}

message notify_attr4 {
    notify_entry4 na_changed_entry;
}

message notify_rename4 {
    notify_remove4 nrn_old_entry;
    notify_add4    nrn_new_entry;
}

struct notify_verifier4 {
    verifier4 nv_old_cookieverf;
    verifier4 nv_new_cookieverf;
}

message notifylist4 {
    beu32 data_len;
    u8    data[data_len];
    align(4);
}

message notify4 {
    bitmap4     notify_mask;
    notifylist4 notify_vals;
}

message CB_NOTIFY4args {
    stateid4 cna_stateid;
    nfs_fh4  cna_fh;
    beu32    cna_changes_count;
    notify4  cna_changes[cna_changes_count];
}

struct CB_NOTIFY4res {
    nfsstat4 cnr_status;
}

message CB_PUSH_DELEG4args {
    nfs_fh4               cpda_fh;
    open_delegation_type4 cpda_delegation_type;
    open_delegation4      cpda_delegation(cpda_delegation_type);
}

struct CB_PUSH_DELEG4res {
    nfsstat4 cpdr_status;
}

message CB_RECALL_ANY4args {
    beu32   craa_objects_to_keep;
    bitmap4 craa_type_mask;
}

struct CB_RECALL_ANY4res {
    nfsstat4 crar_status;
}

struct CB_RECALLABLE_OBJ_AVAIL4res {
    nfsstat4 croa_status;
}

struct CB_RECALL_SLOT4args {
    beu32 rsa_target_highest_slotid;
}

struct CB_RECALL_SLOT4res {
    nfsstat4 rsr_status;
}

struct referring_call4 {
    beu32 rc_sequenceid;
    beu32 rc_slotid;
}

message referring_call_list4 {
    sessionid4      rcl_sessionid;
    beu32           rcl_referring_calls_count;
    referring_call4 rcl_referring_calls[rcl_referring_calls_count];
}

message CB_SEQUENCE4args {
    sessionid4           csa_sessionid;
    beu32                csa_sequenceid;
    beu32                csa_slotid;
    beu32                csa_highest_slotid;
    beu32                csa_cachethis;
    beu32                csa_referring_call_lists_count;
    referring_call_list4 csa_referring_call_lists[csa_referring_call_lists_count];
}

struct CB_SEQUENCE4resok {
    sessionid4 csr_sessionid;
    beu32      csr_sequenceid;
    beu32      csr_slotid;
    beu32      csr_highest_slotid;
    beu32      csr_target_highest_slotid;
}

union CB_SEQUENCE4res (nfsstat4) {
    NFS4_OK: CB_SEQUENCE4resok;
    default: void;
}

struct CB_WANTS_CANCELLED4args {
    beu32 cwca_contended_wants_cancelled;
    beu32 cwca_resourced_wants_cancelled;
}

struct CB_WANTS_CANCELLED4res {
    nfsstat4 cwcr_status;
}

message CB_NOTIFY_LOCK4args {
    nfs_fh4      cnla_fh;
    state_owner4 cnla_lock_owner;
}

struct CB_NOTIFY_LOCK4res {
    nfsstat4 cnlr_status;
}

enum notify_deviceid_type4 : beu32 {
    NOTIFY_DEVICEID4_CHANGE = 1;
    NOTIFY_DEVICEID4_DELETE = 2;
}

struct notify_deviceid_delete4 {
    layouttype4 ndd_layouttype;
    deviceid4   ndd_deviceid;
}

struct notify_deviceid_change4 {
    layouttype4 ndc_layouttype;
    deviceid4   ndc_deviceid;
    beu32       ndc_immediate;
}

message CB_NOTIFY_DEVICEID4args {
    beu32   cnda_changes_count;
    notify4 cnda_changes[cnda_changes_count];
}

struct CB_NOTIFY_DEVICEID4res {
    nfsstat4 cndr_status;
}

struct offload_info4_default {
    beu64 value;
}

union offload_info4 (nfsstat4) {
    NFS4_OK: write_response4;
    default: offload_info4_default;
}

message CB_OFFLOAD4args {
    nfs_fh4       coa_fh;
    stateid4      coa_stateid;
    nfsstat4      coa_offload_info_type;
    offload_info4 coa_offload_info(coa_offload_info_type);
}

struct CB_OFFLOAD4res {
    nfsstat4 cor_status;
}

enum nfs_cb_opnum4 : beu32 {
    OP_CB_GETATTR              = 3;
    OP_CB_RECALL               = 4;
    OP_CB_LAYOUTRECALL         = 5;
    OP_CB_NOTIFY               = 6;
    OP_CB_PUSH_DELEG           = 7;
    OP_CB_RECALL_ANY           = 8;
    OP_CB_RECALLABLE_OBJ_AVAIL = 9;
    OP_CB_RECALL_SLOT          = 10;
    OP_CB_SEQUENCE             = 11;
    OP_CB_WANTS_CANCELLED      = 12;
    OP_CB_NOTIFY_LOCK          = 13;
    OP_CB_NOTIFY_DEVICEID      = 14;
    OP_CB_OFFLOAD              = 15;
    OP_CB_ILLEGAL              = 10044;
}

union nfs_cb_argop4 (nfs_cb_opnum4) {
    OP_CB_GETATTR:              CB_GETATTR4args;
    OP_CB_RECALL:               CB_RECALL4args;
    OP_CB_LAYOUTRECALL:         CB_LAYOUTRECALL4args;
    OP_CB_NOTIFY:               CB_NOTIFY4args;
    OP_CB_PUSH_DELEG:           CB_PUSH_DELEG4args;
    OP_CB_RECALL_ANY:           CB_RECALL_ANY4args;
    OP_CB_RECALLABLE_OBJ_AVAIL: CB_RECALL_ANY4args;
    OP_CB_RECALL_SLOT:          CB_RECALL_SLOT4args;
    OP_CB_SEQUENCE:             CB_SEQUENCE4args;
    OP_CB_WANTS_CANCELLED:      CB_WANTS_CANCELLED4args;
    OP_CB_NOTIFY_LOCK:          CB_NOTIFY_LOCK4args;
    OP_CB_NOTIFY_DEVICEID:      CB_NOTIFY_DEVICEID4args;
    OP_CB_OFFLOAD:              CB_OFFLOAD4args;
    OP_CB_ILLEGAL:              void;
}

message CB_GETATTR4res_entry {
    nfsstat4       disc;
    CB_GETATTR4res value(disc);
}

message CB_SEQUENCE4res_entry {
    nfsstat4        disc;
    CB_SEQUENCE4res value(disc);
}

union nfs_cb_resop4 (nfs_cb_opnum4) {
    OP_CB_GETATTR:              CB_GETATTR4res_entry;
    OP_CB_RECALL:               CB_RECALL4res;
    OP_CB_LAYOUTRECALL:         CB_LAYOUTRECALL4res;
    OP_CB_NOTIFY:               CB_NOTIFY4res;
    OP_CB_PUSH_DELEG:           CB_PUSH_DELEG4res;
    OP_CB_RECALL_ANY:           CB_RECALL_ANY4res;
    OP_CB_RECALLABLE_OBJ_AVAIL: CB_RECALLABLE_OBJ_AVAIL4res;
    OP_CB_RECALL_SLOT:          CB_RECALL_SLOT4res;
    OP_CB_SEQUENCE:             CB_SEQUENCE4res_entry;
    OP_CB_WANTS_CANCELLED:      CB_WANTS_CANCELLED4res;
    OP_CB_NOTIFY_LOCK:          CB_NOTIFY_LOCK4res;
    OP_CB_NOTIFY_DEVICEID:      CB_NOTIFY_DEVICEID4res;
    OP_CB_OFFLOAD:              CB_OFFLOAD4res;
    OP_CB_ILLEGAL:              CB_ILLEGAL4res;
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

message open_arguments4 {
    bitmap4 oa_share_access;
    bitmap4 oa_share_deny;
    bitmap4 oa_share_access_want;
    bitmap4 oa_open_claim;
    bitmap4 oa_create_mode;
}

enum open_args_share_access4 : beu32 {
    OPEN_ARGS_SHARE_ACCESS_READ  = 1;
    OPEN_ARGS_SHARE_ACCESS_WRITE = 2;
    OPEN_ARGS_SHARE_ACCESS_BOTH  = 3;
}

enum open_args_share_deny4 : beu32 {
    OPEN_ARGS_SHARE_DENY_NONE  = 0;
    OPEN_ARGS_SHARE_DENY_READ  = 1;
    OPEN_ARGS_SHARE_DENY_WRITE = 2;
    OPEN_ARGS_SHARE_DENY_BOTH  = 3;
}

enum open_args_share_access_want4 : beu32 {
    OPEN_ARGS_SHARE_ACCESS_WANT_ANY_DELEG                     = 3;
    OPEN_ARGS_SHARE_ACCESS_WANT_NO_DELEG                      = 4;
    OPEN_ARGS_SHARE_ACCESS_WANT_CANCEL                        = 5;
    OPEN_ARGS_SHARE_ACCESS_WANT_SIGNAL_DELEG_WHEN_RESRC_AVAIL = 17;
    OPEN_ARGS_SHARE_ACCESS_WANT_PUSH_DELEG_WHEN_UNCONTENDED   = 18;
    OPEN_ARGS_SHARE_ACCESS_WANT_DELEG_TIMESTAMPS              = 20;
    OPEN_ARGS_SHARE_ACCESS_WANT_OPEN_XOR_DELEGATION           = 21;
}

enum open_args_open_claim4 : beu32 {
    OPEN_ARGS_OPEN_CLAIM_NULL          = 0;
    OPEN_ARGS_OPEN_CLAIM_PREVIOUS      = 1;
    OPEN_ARGS_OPEN_CLAIM_DELEGATE_CUR  = 2;
    OPEN_ARGS_OPEN_CLAIM_DELEGATE_PREV = 3;
    OPEN_ARGS_OPEN_CLAIM_FH            = 4;
    OPEN_ARGS_OPEN_CLAIM_DELEG_CUR_FH  = 5;
    OPEN_ARGS_OPEN_CLAIM_DELEG_PREV_FH = 6;
}

enum open_args_createmode4 : beu32 {
    OPEN_ARGS_CREATEMODE_UNCHECKED4    = 0;
    OPEN_ARGS_CREATE_MODE_GUARDED      = 1;
    OPEN_ARGS_CREATEMODE_EXCLUSIVE4    = 2;
    OPEN_ARGS_CREATE_MODE_EXCLUSIVE4_1 = 3;
}

