package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// -------------------------------------------------------
// Enum constants
// -------------------------------------------------------

const (
	NFS4_FHSIZE                     = 128
	NFS4_VERIFIER_SIZE              = 8
	NFS4_OTHER_SIZE                 = 12
	NFS4_OPAQUE_LIMIT               = 1024
	NFS4_INT64_MAX                  = 0x7fffffffffffffff
	NFS4_UINT64_MAX                 = 0xffffffffffffffff
	NFS4_INT32_MAX                  = 0x7fffffff
	NFS4_UINT32_MAX                 = 0xffffffff
	ACL4_SUPPORT_ALLOW_ACL          = 0x00000001
	ACL4_SUPPORT_DENY_ACL           = 0x00000002
	ACL4_SUPPORT_AUDIT_ACL          = 0x00000004
	ACL4_SUPPORT_ALARM_ACL          = 0x00000008
	ACE4_ACCESS_ALLOWED_ACE_TYPE    = 0x00000000
	ACE4_ACCESS_DENIED_ACE_TYPE     = 0x00000001
	ACE4_SYSTEM_AUDIT_ACE_TYPE      = 0x00000002
	ACE4_SYSTEM_ALARM_ACE_TYPE      = 0x00000003
	ACE4_FILE_INHERIT_ACE           = 0x00000001
	ACE4_DIRECTORY_INHERIT_ACE      = 0x00000002
	ACE4_NO_PROPAGATE_INHERIT_ACE   = 0x00000004
	ACE4_INHERIT_ONLY_ACE           = 0x00000008
	ACE4_SUCCESSFUL_ACCESS_ACE_FLAG = 0x00000010
	ACE4_FAILED_ACCESS_ACE_FLAG     = 0x00000020
	ACE4_IDENTIFIER_GROUP           = 0x00000040
	ACE4_READ_DATA                  = 0x00000001
	ACE4_LIST_DIRECTORY             = 0x00000001
	ACE4_WRITE_DATA                 = 0x00000002
	ACE4_ADD_FILE                   = 0x00000002
	ACE4_APPEND_DATA                = 0x00000004
	ACE4_ADD_SUBDIRECTORY           = 0x00000004
	ACE4_READ_NAMED_ATTRS           = 0x00000008
	ACE4_WRITE_NAMED_ATTRS          = 0x00000010
	ACE4_EXECUTE                    = 0x00000020
	ACE4_DELETE_CHILD               = 0x00000040
	ACE4_READ_ATTRIBUTES            = 0x00000080
	ACE4_WRITE_ATTRIBUTES           = 0x00000100
	ACE4_DELETE                     = 0x00010000
	ACE4_READ_ACL                   = 0x00020000
	ACE4_WRITE_ACL                  = 0x00040000
	ACE4_WRITE_OWNER                = 0x00080000
	ACE4_SYNCHRONIZE                = 0x00100000
	ACE4_GENERIC_READ               = 0x00120081
	ACE4_GENERIC_WRITE              = 0x00160106
	ACE4_GENERIC_EXECUTE            = 0x001200A0
	MODE4_SUID                      = 0x800
	MODE4_SGID                      = 0x400
	MODE4_SVTX                      = 0x200
	MODE4_RUSR                      = 0x100
	MODE4_WUSR                      = 0x080
	MODE4_XUSR                      = 0x040
	MODE4_RGRP                      = 0x020
	MODE4_WGRP                      = 0x010
	MODE4_XGRP                      = 0x008
	MODE4_ROTH                      = 0x004
	MODE4_WOTH                      = 0x002
	MODE4_XOTH                      = 0x001
	FH4_PERSISTENT                  = 0x00000000
	FH4_NOEXPIRE_WITH_OPEN          = 0x00000001
	FH4_VOLATILE_ANY                = 0x00000002
	FH4_VOL_MIGRATION               = 0x00000004
	FH4_VOL_RENAME                  = 0x00000008
	FATTR4_SUPPORTED_ATTRS          = 0
	FATTR4_TYPE                     = 1
	FATTR4_FH_EXPIRE_TYPE           = 2
	FATTR4_CHANGE                   = 3
	FATTR4_SIZE                     = 4
	FATTR4_LINK_SUPPORT             = 5
	FATTR4_SYMLINK_SUPPORT          = 6
	FATTR4_NAMED_ATTR               = 7
	FATTR4_FSID                     = 8
	FATTR4_UNIQUE_HANDLES           = 9
	FATTR4_LEASE_TIME               = 10
	FATTR4_RDATTR_ERROR             = 11
	FATTR4_FILEHANDLE               = 19
	FATTR4_ACL                      = 12
	FATTR4_ACLSUPPORT               = 13
	FATTR4_ARCHIVE                  = 14
	FATTR4_CANSETTIME               = 15
	FATTR4_CASE_INSENSITIVE         = 16
	FATTR4_CASE_PRESERVING          = 17
	FATTR4_CHOWN_RESTRICTED         = 18
	FATTR4_FILEID                   = 20
	FATTR4_FILES_AVAIL              = 21
	FATTR4_FILES_FREE               = 22
	FATTR4_FILES_TOTAL              = 23
	FATTR4_FS_LOCATIONS             = 24
	FATTR4_HIDDEN                   = 25
	FATTR4_HOMOGENEOUS              = 26
	FATTR4_MAXFILESIZE              = 27
	FATTR4_MAXLINK                  = 28
	FATTR4_MAXNAME                  = 29
	FATTR4_MAXREAD                  = 30
	FATTR4_MAXWRITE                 = 31
	FATTR4_MIMETYPE                 = 32
	FATTR4_MODE                     = 33
	FATTR4_NO_TRUNC                 = 34
	FATTR4_NUMLINKS                 = 35
	FATTR4_OWNER                    = 36
	FATTR4_OWNER_GROUP              = 37
	FATTR4_QUOTA_AVAIL_HARD         = 38
	FATTR4_QUOTA_AVAIL_SOFT         = 39
	FATTR4_QUOTA_USED               = 40
	FATTR4_RAWDEV                   = 41
	FATTR4_SPACE_AVAIL              = 42
	FATTR4_SPACE_FREE               = 43
	FATTR4_SPACE_TOTAL              = 44
	FATTR4_SPACE_USED               = 45
	FATTR4_SYSTEM                   = 46
	FATTR4_TIME_ACCESS              = 47
	FATTR4_TIME_ACCESS_SET          = 48
	FATTR4_TIME_BACKUP              = 49
	FATTR4_TIME_CREATE              = 50
	FATTR4_TIME_DELTA               = 51
	FATTR4_TIME_METADATA            = 52
	FATTR4_TIME_MODIFY              = 53
	FATTR4_TIME_MODIFY_SET          = 54
	FATTR4_MOUNTED_ON_FILEID        = 55
	ACCESS4_READ                    = 0x00000001
	ACCESS4_LOOKUP                  = 0x00000002
	ACCESS4_MODIFY                  = 0x00000004
	ACCESS4_EXTEND                  = 0x00000008
	ACCESS4_DELETE                  = 0x00000010
	ACCESS4_EXECUTE                 = 0x00000020
	OPEN4_SHARE_ACCESS_READ         = 0x00000001
	OPEN4_SHARE_ACCESS_WRITE        = 0x00000002
	OPEN4_SHARE_ACCESS_BOTH         = 0x00000003
	OPEN4_SHARE_DENY_NONE           = 0x00000000
	OPEN4_SHARE_DENY_READ           = 0x00000001
	OPEN4_SHARE_DENY_WRITE          = 0x00000002
	OPEN4_SHARE_DENY_BOTH           = 0x00000003
	OPEN4_RESULT_CONFIRM            = 0x00000002
	OPEN4_RESULT_LOCKTYPE_POSIX     = 0x00000004
)

// nfs_ftype4 constants (beu32)
const (
	NF4REG       uint32 = 1
	NF4DIR       uint32 = 2
	NF4BLK       uint32 = 3
	NF4CHR       uint32 = 4
	NF4LNK       uint32 = 5
	NF4SOCK      uint32 = 6
	NF4FIFO      uint32 = 7
	NF4ATTRDIR   uint32 = 8
	NF4NAMEDATTR uint32 = 9
)

// nfsstat4 constants (beu32)
const (
	NFS4_OK                     uint32 = 0
	NFS4ERR_PERM                uint32 = 1
	NFS4ERR_NOENT               uint32 = 2
	NFS4ERR_IO                  uint32 = 5
	NFS4ERR_NXIO                uint32 = 6
	NFS4ERR_ACCESS              uint32 = 13
	NFS4ERR_EXIST               uint32 = 17
	NFS4ERR_XDEV                uint32 = 18
	NFS4ERR_NOTDIR              uint32 = 20
	NFS4ERR_ISDIR               uint32 = 21
	NFS4ERR_INVAL               uint32 = 22
	NFS4ERR_FBIG                uint32 = 27
	NFS4ERR_NOSPC               uint32 = 28
	NFS4ERR_ROFS                uint32 = 30
	NFS4ERR_MLINK               uint32 = 31
	NFS4ERR_NAMETOOLONG         uint32 = 63
	NFS4ERR_NOTEMPTY            uint32 = 66
	NFS4ERR_DQUOT               uint32 = 69
	NFS4ERR_STALE               uint32 = 70
	NFS4ERR_BADHANDLE           uint32 = 10001
	NFS4ERR_BAD_COOKIE          uint32 = 10003
	NFS4ERR_NOTSUPP             uint32 = 10004
	NFS4ERR_TOOSMALL            uint32 = 10005
	NFS4ERR_SERVERFAULT         uint32 = 10006
	NFS4ERR_BADTYPE             uint32 = 10007
	NFS4ERR_DELAY               uint32 = 10008
	NFS4ERR_SAME                uint32 = 10009
	NFS4ERR_DENIED              uint32 = 10010
	NFS4ERR_EXPIRED             uint32 = 10011
	NFS4ERR_LOCKED              uint32 = 10012
	NFS4ERR_GRACE               uint32 = 10013
	NFS4ERR_FHEXPIRED           uint32 = 10014
	NFS4ERR_SHARE_DENIED        uint32 = 10015
	NFS4ERR_WRONGSEC            uint32 = 10016
	NFS4ERR_CLID_INUSE          uint32 = 10017
	NFS4ERR_RESOURCE            uint32 = 10018
	NFS4ERR_MOVED               uint32 = 10019
	NFS4ERR_NOFILEHANDLE        uint32 = 10020
	NFS4ERR_MINOR_VERS_MISMATCH uint32 = 10021
	NFS4ERR_STALE_CLIENTID      uint32 = 10022
	NFS4ERR_STALE_STATEID       uint32 = 10023
	NFS4ERR_OLD_STATEID         uint32 = 10024
	NFS4ERR_BAD_STATEID         uint32 = 10025
	NFS4ERR_BAD_SEQID           uint32 = 10026
	NFS4ERR_NOT_SAME            uint32 = 10027
	NFS4ERR_LOCK_RANGE          uint32 = 10028
	NFS4ERR_SYMLINK             uint32 = 10029
	NFS4ERR_RESTOREFH           uint32 = 10030
	NFS4ERR_LEASE_MOVED         uint32 = 10031
	NFS4ERR_ATTRNOTSUPP         uint32 = 10032
	NFS4ERR_NO_GRACE            uint32 = 10033
	NFS4ERR_RECLAIM_BAD         uint32 = 10034
	NFS4ERR_RECLAIM_CONFLICT    uint32 = 10035
	NFS4ERR_BADXDR              uint32 = 10036
	NFS4ERR_LOCKS_HELD          uint32 = 10037
	NFS4ERR_OPENMODE            uint32 = 10038
	NFS4ERR_BADOWNER            uint32 = 10039
	NFS4ERR_BADCHAR             uint32 = 10040
	NFS4ERR_BADNAME             uint32 = 10041
	NFS4ERR_BAD_RANGE           uint32 = 10042
	NFS4ERR_LOCK_NOTSUPP        uint32 = 10043
	NFS4ERR_OP_ILLEGAL          uint32 = 10044
	NFS4ERR_DEADLOCK            uint32 = 10045
	NFS4ERR_FILE_OPEN           uint32 = 10046
	NFS4ERR_ADMIN_REVOKED       uint32 = 10047
	NFS4ERR_CB_PATH_DOWN        uint32 = 10048
)

// time_how4 constants (beu32)
const (
	SET_TO_SERVER_TIME4 uint32 = 0
	SET_TO_CLIENT_TIME4 uint32 = 1
)

// nfs_lock_type4 constants (beu32)
const (
	READ_LT   uint32 = 1
	WRITE_LT  uint32 = 2
	READW_LT  uint32 = 3
	WRITEW_LT uint32 = 4
)

// xdr_bool constants (beu32)
const (
	FALSE uint32 = 0
	TRUE  uint32 = 1
)

// createmode4 constants (beu32)
const (
	UNCHECKED4 uint32 = 0
	GUARDED4   uint32 = 1
	EXCLUSIVE4 uint32 = 2
)

// opentype4 constants (beu32)
const (
	OPEN4_NOCREATE uint32 = 0
	OPEN4_CREATE   uint32 = 1
)

// limit_by4 constants (beu32)
const (
	NFS_LIMIT_SIZE   uint32 = 1
	NFS_LIMIT_BLOCKS uint32 = 2
)

// open_delegation_type4 constants (beu32)
const (
	OPEN_DELEGATE_NONE  uint32 = 0
	OPEN_DELEGATE_READ  uint32 = 1
	OPEN_DELEGATE_WRITE uint32 = 2
)

// open_claim_type4 constants (beu32)
const (
	CLAIM_NULL          uint32 = 0
	CLAIM_PREVIOUS      uint32 = 1
	CLAIM_DELEGATE_CUR  uint32 = 2
	CLAIM_DELEGATE_PREV uint32 = 3
)

// rpc_gss_svc_t constants (beu32)
const (
	RPC_GSS_SVC_NONE      uint32 = 1
	RPC_GSS_SVC_INTEGRITY uint32 = 2
	RPC_GSS_SVC_PRIVACY   uint32 = 3
)

// disc_secinfo4 constants (beu32)
const (
	RPCSEC_GSS uint32 = 6
)

// stable_how4 constants (beu32)
const (
	UNSTABLE4  uint32 = 0
	DATA_SYNC4 uint32 = 1
	FILE_SYNC4 uint32 = 2
)

// nfs_opnum4 constants (beu32)
const (
	OP_ACCESS              uint32 = 3
	OP_CLOSE               uint32 = 4
	OP_COMMIT              uint32 = 5
	OP_CREATE              uint32 = 6
	OP_DELEGPURGE          uint32 = 7
	OP_DELEGRETURN         uint32 = 8
	OP_GETATTR             uint32 = 9
	OP_GETFH               uint32 = 10
	OP_LINK                uint32 = 11
	OP_LOCK                uint32 = 12
	OP_LOCKT               uint32 = 13
	OP_LOCKU               uint32 = 14
	OP_LOOKUP              uint32 = 15
	OP_LOOKUPP             uint32 = 16
	OP_NVERIFY             uint32 = 17
	OP_OPEN                uint32 = 18
	OP_OPENATTR            uint32 = 19
	OP_OPEN_CONFIRM        uint32 = 20
	OP_OPEN_DOWNGRADE      uint32 = 21
	OP_PUTFH               uint32 = 22
	OP_PUTPUBFH            uint32 = 23
	OP_PUTROOTFH           uint32 = 24
	OP_READ                uint32 = 25
	OP_READDIR             uint32 = 26
	OP_READLINK            uint32 = 27
	OP_REMOVE              uint32 = 28
	OP_RENAME              uint32 = 29
	OP_RENEW               uint32 = 30
	OP_RESTOREFH           uint32 = 31
	OP_SAVEFH              uint32 = 32
	OP_SECINFO             uint32 = 33
	OP_SETATTR             uint32 = 34
	OP_SETCLIENTID         uint32 = 35
	OP_SETCLIENTID_CONFIRM uint32 = 36
	OP_VERIFY              uint32 = 37
	OP_WRITE               uint32 = 38
	OP_RELEASE_LOCKOWNER   uint32 = 39
	OP_ILLEGAL             uint32 = 10044
)

// nfs_cb_opnum4 constants (beu32)
const (
	OP_CB_GETATTR uint32 = 3
	OP_CB_RECALL  uint32 = 4
	OP_CB_ILLEGAL uint32 = 10044
)

// -------------------------------------------------------
// Attrlist4 — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type Attrlist4 []byte

func readAttrlist4(b *[]byte) (Attrlist4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := Attrlist4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadAttrlist4(b []byte) (Attrlist4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return Attrlist4(b[:total]), true
}

func (m Attrlist4) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Attrlist4) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// Attrlist4Writer writes a attrlist4:
//
//	data_len + u8 data[data_len] + align(4)
type Attrlist4Writer struct {
	buf []byte
	off int
}

func StartAttrlist4(buf []byte) Attrlist4Writer {
	off := len(buf)
	return Attrlist4Writer{buf: buf, off: off}
}

func (w Attrlist4Writer) SetData(data []byte) Attrlist4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w Attrlist4Writer) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Bitmap4 — variable: count(4) + beu32 data[count]
// -------------------------------------------------------

type Bitmap4 []byte

func readBitmap4(b *[]byte) (Bitmap4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	c_count := int(binary.BigEndian.Uint32((*b)[0:4]))
	*b = (*b)[4:]
	if len(*b) < c_count*4 {
		return nil, false
	}
	*b = (*b)[c_count*4:]
	total := startLen - len(*b)
	return Bitmap4(start[:total]), true
}

func ReadBitmap4(b []byte) (Bitmap4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	total := 4 + count*4
	if len(b) < total {
		return nil, false
	}
	return Bitmap4(b[:total]), true
}

func (m Bitmap4) Count() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Bitmap4) Data(i int) uint32 {
	off := 4 + i*4
	return binary.BigEndian.Uint32(m[off : off+4])
}

// Bitmap4Writer writes a bitmap4:
//
//	count + beu32 data[count]
type Bitmap4Writer struct {
	buf   []byte
	off   int
	count uint32
}

func StartBitmap4(buf []byte) Bitmap4Writer {
	off := len(buf)
	buf = binary.BigEndian.AppendUint32(buf, 0) // count placeholder
	return Bitmap4Writer{buf: buf, off: off}
}

func (w *Bitmap4Writer) AppendData(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
	w.count++
}

func (w *Bitmap4Writer) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], w.count)
	return w.buf
}

// -------------------------------------------------------
// NfsFh4 — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type NfsFh4 []byte

func readNfsFh4(b *[]byte) (NfsFh4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := NfsFh4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadNfsFh4(b []byte) (NfsFh4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return NfsFh4(b[:total]), true
}

func (m NfsFh4) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m NfsFh4) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// NfsFh4Writer writes a nfs_fh4:
//
//	data_len + u8 data[data_len] + align(4)
type NfsFh4Writer struct {
	buf []byte
	off int
}

func StartNfsFh4(buf []byte) NfsFh4Writer {
	off := len(buf)
	return NfsFh4Writer{buf: buf, off: off}
}

func (w NfsFh4Writer) SetData(data []byte) NfsFh4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w NfsFh4Writer) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// SecOid4 — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type SecOid4 []byte

func readSecOid4(b *[]byte) (SecOid4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := SecOid4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadSecOid4(b []byte) (SecOid4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return SecOid4(b[:total]), true
}

func (m SecOid4) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m SecOid4) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// SecOid4Writer writes a sec_oid4:
//
//	data_len + u8 data[data_len] + align(4)
type SecOid4Writer struct {
	buf []byte
	off int
}

func StartSecOid4(buf []byte) SecOid4Writer {
	off := len(buf)
	return SecOid4Writer{buf: buf, off: off}
}

func (w SecOid4Writer) SetData(data []byte) SecOid4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w SecOid4Writer) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Utf8strCis — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type Utf8strCis []byte

func readUtf8strCis(b *[]byte) (Utf8strCis, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := Utf8strCis((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadUtf8strCis(b []byte) (Utf8strCis, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return Utf8strCis(b[:total]), true
}

func (m Utf8strCis) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Utf8strCis) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// Utf8strCisWriter writes a utf8str_cis:
//
//	data_len + u8 data[data_len] + align(4)
type Utf8strCisWriter struct {
	buf []byte
	off int
}

func StartUtf8strCis(buf []byte) Utf8strCisWriter {
	off := len(buf)
	return Utf8strCisWriter{buf: buf, off: off}
}

func (w Utf8strCisWriter) SetData(data []byte) Utf8strCisWriter {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w Utf8strCisWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Utf8strCs — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type Utf8strCs []byte

func readUtf8strCs(b *[]byte) (Utf8strCs, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := Utf8strCs((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadUtf8strCs(b []byte) (Utf8strCs, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return Utf8strCs(b[:total]), true
}

func (m Utf8strCs) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Utf8strCs) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// Utf8strCsWriter writes a utf8str_cs:
//
//	data_len + u8 data[data_len] + align(4)
type Utf8strCsWriter struct {
	buf []byte
	off int
}

func StartUtf8strCs(buf []byte) Utf8strCsWriter {
	off := len(buf)
	return Utf8strCsWriter{buf: buf, off: off}
}

func (w Utf8strCsWriter) SetData(data []byte) Utf8strCsWriter {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w Utf8strCsWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Utf8strMixed — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type Utf8strMixed []byte

func readUtf8strMixed(b *[]byte) (Utf8strMixed, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := Utf8strMixed((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadUtf8strMixed(b []byte) (Utf8strMixed, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return Utf8strMixed(b[:total]), true
}

func (m Utf8strMixed) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Utf8strMixed) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// Utf8strMixedWriter writes a utf8str_mixed:
//
//	data_len + u8 data[data_len] + align(4)
type Utf8strMixedWriter struct {
	buf []byte
	off int
}

func StartUtf8strMixed(buf []byte) Utf8strMixedWriter {
	off := len(buf)
	return Utf8strMixedWriter{buf: buf, off: off}
}

func (w Utf8strMixedWriter) SetData(data []byte) Utf8strMixedWriter {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w Utf8strMixedWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Component4 — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type Component4 []byte

func readComponent4(b *[]byte) (Component4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := Component4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadComponent4(b []byte) (Component4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return Component4(b[:total]), true
}

func (m Component4) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Component4) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// Component4Writer writes a component4:
//
//	data_len + u8 data[data_len] + align(4)
type Component4Writer struct {
	buf []byte
	off int
}

func StartComponent4(buf []byte) Component4Writer {
	off := len(buf)
	return Component4Writer{buf: buf, off: off}
}

func (w Component4Writer) SetData(data []byte) Component4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w Component4Writer) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Linktext4 — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type Linktext4 []byte

func readLinktext4(b *[]byte) (Linktext4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := Linktext4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadLinktext4(b []byte) (Linktext4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return Linktext4(b[:total]), true
}

func (m Linktext4) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Linktext4) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// Linktext4Writer writes a linktext4:
//
//	data_len + u8 data[data_len] + align(4)
type Linktext4Writer struct {
	buf []byte
	off int
}

func StartLinktext4(buf []byte) Linktext4Writer {
	off := len(buf)
	return Linktext4Writer{buf: buf, off: off}
}

func (w Linktext4Writer) SetData(data []byte) Linktext4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w Linktext4Writer) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Pathname4 — variable: count(4) + component4 data[count]
// -------------------------------------------------------

type Pathname4 []byte

func readPathname4(b *[]byte) (Pathname4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	c_count := int(binary.BigEndian.Uint32((*b)[0:4]))
	*b = (*b)[4:]
	for i := 0; i < c_count; i++ {
		if _, ok := readComponent4(b); !ok {
			return nil, false
		}
	}
	total := startLen - len(*b)
	return Pathname4(start[:total]), true
}

func ReadPathname4(b []byte) (Pathname4, bool) {
	if len(b) < 4 {
		return nil, false
	}
	return Pathname4(b), true
}

func (m Pathname4) Count() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Pathname4) Data() Component4Iter {
	count := int(binary.BigEndian.Uint32(m[0:4]))
	return Component4Iter{
		b:     []byte(m[4:]),
		count: count,
	}
}

// Component4Iter iterates over variable-size Component4 entries.
type Component4Iter struct {
	b      []byte
	count  int
	i      int
	curLen int
}

func (it *Component4Iter) Next() bool {
	b := it.b
	if it.curLen > 0 {
		if len(b) < it.curLen {
			return false
		}
		b = b[it.curLen:]
		it.curLen = 0
	}
	if it.i >= it.count {
		it.b = b
		return false
	}
	if len(b) < 4 {
		return false
	}
	n := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return false
	}
	it.b = b
	it.curLen = total
	it.i++
	return true
}

func (it *Component4Iter) Data() Component4 {
	return Component4(it.b)
}

// Pathname4Writer writes a pathname4:
//
//	count + component4 data[count]
type Pathname4Writer struct {
	buf   []byte
	off   int
	count uint32
}

func StartPathname4(buf []byte) Pathname4Writer {
	off := len(buf)
	buf = binary.BigEndian.AppendUint32(buf, 0) // count placeholder
	return Pathname4Writer{buf: buf, off: off}
}

func (w *Pathname4Writer) AppendData() Component4Writer {
	_ = w.buf[:1]
	w.count++
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *Pathname4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *Pathname4Writer) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], w.count)
	return w.buf
}

// -------------------------------------------------------
// Verifier4 — fixed 8 bytes: data(u8[8])
// -------------------------------------------------------

type Verifier4 struct {
	m *[verifier4Size]byte
}

const verifier4Size = 8

func readVerifier4(b *[]byte) (Verifier4, bool) {
	if len(*b) < verifier4Size {
		return Verifier4{}, false
	}
	result := Verifier4{m: (*[verifier4Size]byte)(*b)}
	*b = (*b)[verifier4Size:]
	return result, true
}

func ReadVerifier4(b []byte) (Verifier4, bool) {
	return readVerifier4(&b)
}

func StartVerifier4(buf []byte) ([]byte, Verifier4) {
	buf = append(buf, make([]byte, verifier4Size)...)
	return buf, Verifier4{m: (*[verifier4Size]byte)(buf[len(buf)-verifier4Size:])}
}

func (m Verifier4) Data(i int) uint8 {
	off := 0 + i*1
	return m.m[off : off+1][0]
}

func (m Verifier4) SetData(i int, v uint8) {
	off := 0 + i*1
	m.m[off : off+1][0] = v
}

// -------------------------------------------------------
// Nfstime4 — fixed 12 bytes: seconds(8, bei64) + nseconds(4, beu32)
// -------------------------------------------------------

type Nfstime4 struct {
	m *[nfstime4Size]byte
}

const nfstime4Size = 12

func readNfstime4(b *[]byte) (Nfstime4, bool) {
	if len(*b) < nfstime4Size {
		return Nfstime4{}, false
	}
	result := Nfstime4{m: (*[nfstime4Size]byte)(*b)}
	*b = (*b)[nfstime4Size:]
	return result, true
}

func ReadNfstime4(b []byte) (Nfstime4, bool) {
	return readNfstime4(&b)
}

func StartNfstime4(buf []byte) ([]byte, Nfstime4) {
	buf = append(buf, make([]byte, nfstime4Size)...)
	return buf, Nfstime4{m: (*[nfstime4Size]byte)(buf[len(buf)-nfstime4Size:])}
}

func (m Nfstime4) Seconds() int64 {
	return int64(binary.BigEndian.Uint64(m.m[0:8]))
}

func (m Nfstime4) Nseconds() uint32 {
	return binary.BigEndian.Uint32(m.m[8:12])
}

func (m Nfstime4) SetSeconds(v int64) {
	binary.BigEndian.PutUint64(m.m[0:8], uint64(v))
}

func (m Nfstime4) SetNseconds(v uint32) {
	binary.BigEndian.PutUint32(m.m[8:12], v)
}

// -------------------------------------------------------
// Settime4 — union on time_how4 (external discriminant)
// -------------------------------------------------------

type Settime4 struct {
	b    []byte
	disc uint32
}

func readSettime4(b *[]byte, timeHow4 uint32) (Settime4, bool) {
	switch timeHow4 {
	case SET_TO_CLIENT_TIME4:
		r, ok := readNfstime4(b)
		if !ok {
			return Settime4{}, false
		}
		return Settime4{b: r.m[:], disc: timeHow4}, true
	default:
		return Settime4{b: (*b)[:0], disc: timeHow4}, true
	}
}

func (m Settime4) AsNfstime4() Nfstime4 {
	if m.disc != SET_TO_CLIENT_TIME4 {
		panic("wrong union discriminant")
	}
	return Nfstime4{m: (*[nfstime4Size]byte)(m.b)}
}

// -------------------------------------------------------
// Fsid4 — fixed 16 bytes: major(8, beu64) + minor(8, beu64)
// -------------------------------------------------------

type Fsid4 struct {
	m *[fsid4Size]byte
}

const fsid4Size = 16

func ReadFsid4(b []byte) (Fsid4, bool) {
	if len(b) < fsid4Size {
		return Fsid4{}, false
	}
	return Fsid4{m: (*[fsid4Size]byte)(b)}, true
}

func StartFsid4(buf []byte) ([]byte, Fsid4) {
	buf = append(buf, make([]byte, fsid4Size)...)
	return buf, Fsid4{m: (*[fsid4Size]byte)(buf[len(buf)-fsid4Size:])}
}

func (m Fsid4) Major() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m Fsid4) Minor() uint64 {
	return binary.BigEndian.Uint64(m.m[8:16])
}

func (m Fsid4) SetMajor(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

func (m Fsid4) SetMinor(v uint64) {
	binary.BigEndian.PutUint64(m.m[8:16], v)
}

// -------------------------------------------------------
// FsLocation4 — variable:
//   server_count(4) + utf8str_cis server[server_count] + rootpath
// -------------------------------------------------------

type FsLocation4 struct {
	data []byte
	off1 int // byte offset within data where rootpath starts
}

func readFsLocation4(b *[]byte) (FsLocation4, bool) {
	if len(*b) < 4 {
		return FsLocation4{}, false
	}
	start := *b
	startLen := len(start)
	c_server_count := int(binary.BigEndian.Uint32((*b)[0:4]))
	*b = (*b)[4:]
	for i := 0; i < c_server_count; i++ {
		if _, ok := readUtf8strCis(b); !ok {
			return FsLocation4{}, false
		}
	}
	off1 := startLen - len(*b)
	if _, ok := readPathname4(b); !ok {
		return FsLocation4{}, false
	}
	total := startLen - len(*b)
	return FsLocation4{data: start[:total], off1: off1}, true
}

func ReadFsLocation4(b []byte) (FsLocation4, bool) {
	return readFsLocation4(&b)
}

func (m FsLocation4) Server() Utf8strCisIter {
	server_count := int(binary.BigEndian.Uint32(m.data[0:4]))
	return Utf8strCisIter{b: m.data[4:], count: server_count}
}

func (m FsLocation4) ServerCount() uint32 {
	return binary.BigEndian.Uint32(m.data[0:4])
}

func (m FsLocation4) Rootpath() Pathname4 {
	return Pathname4(m.data[m.off1:])
}

// Utf8strCisIter iterates over variable-size Utf8strCis entries.
type Utf8strCisIter struct {
	b      []byte
	count  int
	i      int
	curLen int
}

func (it *Utf8strCisIter) Next() bool {
	b := it.b
	if it.curLen > 0 {
		if len(b) < it.curLen {
			return false
		}
		b = b[it.curLen:]
		it.curLen = 0
	}
	if it.i >= it.count {
		it.b = b
		return false
	}
	if len(b) < 4 {
		return false
	}
	n := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return false
	}
	it.b = b
	it.curLen = total
	it.i++
	return true
}

func (it *Utf8strCisIter) Server() Utf8strCis {
	return Utf8strCis(it.b)
}

// FsLocation4Writer writes a fs_location4:
//
//	server_count + utf8str_cis server[server_count] + rootpath
type FsLocation4Writer struct {
	buf         []byte
	off         int
	serverCount uint32
	phase       uint8
}

func StartFsLocation4(buf []byte) FsLocation4Writer {
	off := len(buf)
	buf = binary.BigEndian.AppendUint32(buf, 0) // server_count placeholder
	return FsLocation4Writer{buf: buf, off: off}
}

func (w *FsLocation4Writer) AppendServer() Utf8strCisWriter {
	_ = w.buf[:1]
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	w.serverCount++
	child := StartUtf8strCis(w.buf)
	w.buf = nil
	return child
}

func (w *FsLocation4Writer) StartRootpath() Pathname4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartPathname4(w.buf)
	w.buf = nil
	return child
}

func (w *FsLocation4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *FsLocation4Writer) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], w.serverCount)
	return w.buf
}

// -------------------------------------------------------
// FsLocations4 — variable:
//   fs_root + locations_count(4) + fs_location4 locations[locations_count]
// -------------------------------------------------------

type FsLocations4 struct {
	data []byte
	off1 int // byte offset within data where locations_count starts
}

func readFsLocations4(b *[]byte) (FsLocations4, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readPathname4(b); !ok {
		return FsLocations4{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return FsLocations4{}, false
	}
	c_locations_count := int(binary.BigEndian.Uint32((*b)[:4]))
	*b = (*b)[4:]
	for i := 0; i < c_locations_count; i++ {
		if _, ok := readFsLocation4(b); !ok {
			return FsLocations4{}, false
		}
	}
	total := startLen - len(*b)
	return FsLocations4{data: start[:total], off1: off1}, true
}

func ReadFsLocations4(b []byte) (FsLocations4, bool) {
	return readFsLocations4(&b)
}

func (m FsLocations4) FsRoot() Pathname4 {
	return Pathname4(m.data[0:m.off1])
}

func (m FsLocations4) Locations() FsLocation4Iter {
	locations_count := int(binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4]))
	return FsLocation4Iter{b: m.data[m.off1+4:], count: locations_count}
}

func (m FsLocations4) LocationsCount() uint32 {
	return binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4])
}

// FsLocation4Iter iterates over variable-size FsLocation4 entries.
type FsLocation4Iter struct {
	b     []byte
	count int
	i     int
	cur   FsLocation4
}

func (it *FsLocation4Iter) Next() bool {
	if it.i >= it.count {
		return false
	}
	var ok bool
	it.cur, ok = readFsLocation4(&it.b)
	if !ok {
		return false
	}
	it.i++
	return true
}

func (it *FsLocation4Iter) Location() FsLocation4 {
	return it.cur
}

// FsLocations4Writer writes a fs_locations4:
//
//	fs_root + locations_count + fs_location4 locations[locations_count]
type FsLocations4Writer struct {
	buf               []byte
	off               int
	locationsCount    uint32
	locationsCountOff int
	phase             uint8
}

func StartFsLocations4(buf []byte) FsLocations4Writer {
	off := len(buf)
	return FsLocations4Writer{buf: buf, off: off}
}

func (w *FsLocations4Writer) AppendLocations() FsLocation4Writer {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.locationsCountOff == 0 {
		w.locationsCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.locationsCount++
	child := StartFsLocation4(w.buf)
	w.buf = nil
	return child
}

func (w *FsLocations4Writer) StartFsRoot() Pathname4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartPathname4(w.buf)
	w.buf = nil
	return child
}

func (w *FsLocations4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *FsLocations4Writer) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.locationsCountOff:w.locationsCountOff+4], w.locationsCount)
	return w.buf
}

// -------------------------------------------------------
// Nfsace4 — variable: type_val(4) + flag(4) + access_mask(4) + who
// -------------------------------------------------------

type Nfsace4 []byte

func readNfsace4(b *[]byte) (Nfsace4, bool) {
	if len(*b) < 12 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[12:]
	if _, ok := readUtf8strMixed(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return Nfsace4(start[:total]), true
}

func ReadNfsace4(b []byte) (Nfsace4, bool) {
	return readNfsace4(&b)
}

func (m Nfsace4) TypeVal() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Nfsace4) Flag() uint32 {
	return binary.BigEndian.Uint32(m[4:8])
}

func (m Nfsace4) AccessMask() uint32 {
	return binary.BigEndian.Uint32(m[8:12])
}

func (m Nfsace4) Who() Utf8strMixed {
	return Utf8strMixed(m[12:])
}

// Nfsace4Writer writes a nfsace4:
//
//	type_val + flag + access_mask + who
type Nfsace4Writer struct {
	buf    []byte
	header *[12]byte
}

func StartNfsace4(buf []byte) Nfsace4Writer {
	buf = append(buf, make([]byte, 12)...) // type_val(4) + flag(4) + access_mask(4)
	return Nfsace4Writer{buf: buf, header: (*[12]byte)(buf[len(buf)-12:])}
}

func (w *Nfsace4Writer) SetTypeVal(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *Nfsace4Writer) SetFlag(v uint32) {
	binary.BigEndian.PutUint32(w.header[4:8], v)
}

func (w *Nfsace4Writer) SetAccessMask(v uint32) {
	binary.BigEndian.PutUint32(w.header[8:12], v)
}

func (w *Nfsace4Writer) StartWho() Utf8strMixedWriter {
	child := StartUtf8strMixed(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *Nfsace4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *Nfsace4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// Specdata4 — fixed 8 bytes: specdata1(4, beu32) + specdata2(4, beu32)
// -------------------------------------------------------

type Specdata4 struct {
	m *[specdata4Size]byte
}

const specdata4Size = 8

func readSpecdata4(b *[]byte) (Specdata4, bool) {
	if len(*b) < specdata4Size {
		return Specdata4{}, false
	}
	result := Specdata4{m: (*[specdata4Size]byte)(*b)}
	*b = (*b)[specdata4Size:]
	return result, true
}

func ReadSpecdata4(b []byte) (Specdata4, bool) {
	return readSpecdata4(&b)
}

func StartSpecdata4(buf []byte) ([]byte, Specdata4) {
	buf = append(buf, make([]byte, specdata4Size)...)
	return buf, Specdata4{m: (*[specdata4Size]byte)(buf[len(buf)-specdata4Size:])}
}

func (m Specdata4) Specdata1() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m Specdata4) Specdata2() uint32 {
	return binary.BigEndian.Uint32(m.m[4:8])
}

func (m Specdata4) SetSpecdata1(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m Specdata4) SetSpecdata2(v uint32) {
	binary.BigEndian.PutUint32(m.m[4:8], v)
}

// -------------------------------------------------------
// Fattr4 — variable: attrmask + attr_vals
// -------------------------------------------------------

type Fattr4 struct {
	data []byte
	off1 int // byte offset within data where attr_vals starts
}

func readFattr4(b *[]byte) (Fattr4, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readBitmap4(b); !ok {
		return Fattr4{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readAttrlist4(b); !ok {
		return Fattr4{}, false
	}
	total := startLen - len(*b)
	return Fattr4{data: start[:total], off1: off1}, true
}

func ReadFattr4(b []byte) (Fattr4, bool) {
	return readFattr4(&b)
}

func (m Fattr4) Attrmask() Bitmap4 {
	return Bitmap4(m.data[0:m.off1])
}

func (m Fattr4) AttrVals() Attrlist4 {
	return Attrlist4(m.data[m.off1:])
}

// Fattr4Writer writes a fattr4:
//
//	attrmask + attr_vals
type Fattr4Writer struct {
	buf   []byte
	off   int
	phase uint8
}

func StartFattr4(buf []byte) Fattr4Writer {
	off := len(buf)
	return Fattr4Writer{buf: buf, off: off}
}

func (w *Fattr4Writer) StartAttrmask() Bitmap4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartBitmap4(w.buf)
	w.buf = nil
	return child
}

func (w *Fattr4Writer) StartAttrVals() Attrlist4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartAttrlist4(w.buf)
	w.buf = nil
	return child
}

func (w *Fattr4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *Fattr4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// ChangeInfo4 — fixed 20 bytes:
//   atomic(4, beu32) + before(8, beu64) + after(8, beu64)
// -------------------------------------------------------

type ChangeInfo4 struct {
	m *[changeInfo4Size]byte
}

const changeInfo4Size = 20

func ReadChangeInfo4(b []byte) (ChangeInfo4, bool) {
	if len(b) < changeInfo4Size {
		return ChangeInfo4{}, false
	}
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(b)}, true
}

func StartChangeInfo4(buf []byte) ([]byte, ChangeInfo4) {
	buf = append(buf, make([]byte, changeInfo4Size)...)
	return buf, ChangeInfo4{m: (*[changeInfo4Size]byte)(buf[len(buf)-changeInfo4Size:])}
}

func (m ChangeInfo4) Atomic() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m ChangeInfo4) Before() uint64 {
	return binary.BigEndian.Uint64(m.m[4:12])
}

func (m ChangeInfo4) After() uint64 {
	return binary.BigEndian.Uint64(m.m[12:20])
}

func (m ChangeInfo4) SetAtomic(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m ChangeInfo4) SetBefore(v uint64) {
	binary.BigEndian.PutUint64(m.m[4:12], v)
}

func (m ChangeInfo4) SetAfter(v uint64) {
	binary.BigEndian.PutUint64(m.m[12:20], v)
}

// -------------------------------------------------------
// XdrOpaque — variable: data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type XdrOpaque []byte

func readXdrOpaque(b *[]byte) (XdrOpaque, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[0:4]))
	padded := (n + 3) &^ 3
	total := 4 + padded
	if len(*b) < total {
		return nil, false
	}
	result := XdrOpaque((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadXdrOpaque(b []byte) (XdrOpaque, bool) {
	if len(b) < 4 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[0:4]))
	padded := (count + 3) &^ 3
	total := 4 + padded
	if len(b) < total {
		return nil, false
	}
	return XdrOpaque(b[:total]), true
}

func (m XdrOpaque) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m XdrOpaque) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[0:4]))
	return m[4 : 4+n]
}

// XdrOpaqueWriter writes a xdr_opaque:
//
//	data_len + u8 data[data_len] + align(4)
type XdrOpaqueWriter struct {
	buf []byte
	off int
}

func StartXdrOpaque(buf []byte) XdrOpaqueWriter {
	off := len(buf)
	return XdrOpaqueWriter{buf: buf, off: off}
}

func (w XdrOpaqueWriter) SetData(data []byte) XdrOpaqueWriter {
	n := len(data)
	padded := (n + 3) &^ 3
	total := 4 + padded
	w.buf = append(w.buf, make([]byte, total)...)
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], uint32(n))
	copy(w.buf[w.off+4:], data)
	return w
}

func (w XdrOpaqueWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// Clientaddr4 — variable: r_netid + r_addr
// -------------------------------------------------------

type Clientaddr4 struct {
	data []byte
	off1 int // byte offset within data where r_addr starts
}

func readClientaddr4(b *[]byte) (Clientaddr4, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readXdrOpaque(b); !ok {
		return Clientaddr4{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readXdrOpaque(b); !ok {
		return Clientaddr4{}, false
	}
	total := startLen - len(*b)
	return Clientaddr4{data: start[:total], off1: off1}, true
}

func ReadClientaddr4(b []byte) (Clientaddr4, bool) {
	return readClientaddr4(&b)
}

func (m Clientaddr4) RNetid() XdrOpaque {
	return XdrOpaque(m.data[0:m.off1])
}

func (m Clientaddr4) RAddr() XdrOpaque {
	return XdrOpaque(m.data[m.off1:])
}

// Clientaddr4Writer writes a clientaddr4:
//
//	r_netid + r_addr
type Clientaddr4Writer struct {
	buf   []byte
	off   int
	phase uint8
}

func StartClientaddr4(buf []byte) Clientaddr4Writer {
	off := len(buf)
	return Clientaddr4Writer{buf: buf, off: off}
}

func (w *Clientaddr4Writer) StartRNetid() XdrOpaqueWriter {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartXdrOpaque(w.buf)
	w.buf = nil
	return child
}

func (w *Clientaddr4Writer) StartRAddr() XdrOpaqueWriter {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartXdrOpaque(w.buf)
	w.buf = nil
	return child
}

func (w *Clientaddr4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *Clientaddr4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CbClient4 — variable: cb_program(4) + cb_location
// -------------------------------------------------------

type CbClient4 []byte

func readCbClient4(b *[]byte) (CbClient4, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readClientaddr4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CbClient4(start[:total]), true
}

func ReadCbClient4(b []byte) (CbClient4, bool) {
	return readCbClient4(&b)
}

func (m CbClient4) CbProgram() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m CbClient4) CbLocation() Clientaddr4 {
	v, _ := ReadClientaddr4(m[4:])
	return v
}

// CbClient4Writer writes a cb_client4:
//
//	cb_program + cb_location
type CbClient4Writer struct {
	buf    []byte
	header *[4]byte
}

func StartCbClient4(buf []byte) CbClient4Writer {
	buf = append(buf, make([]byte, 4)...) // cb_program(4)
	return CbClient4Writer{buf: buf, header: (*[4]byte)(buf[len(buf)-4:])}
}

func (w *CbClient4Writer) SetCbProgram(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *CbClient4Writer) StartCbLocation() Clientaddr4Writer {
	child := StartClientaddr4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *CbClient4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *CbClient4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// Stateid4 — fixed 16 bytes: seqid(4, beu32) + other(u8[12])
// -------------------------------------------------------

type Stateid4 struct {
	m *[stateid4Size]byte
}

const stateid4Size = 16

func readStateid4(b *[]byte) (Stateid4, bool) {
	if len(*b) < stateid4Size {
		return Stateid4{}, false
	}
	result := Stateid4{m: (*[stateid4Size]byte)(*b)}
	*b = (*b)[stateid4Size:]
	return result, true
}

func ReadStateid4(b []byte) (Stateid4, bool) {
	return readStateid4(&b)
}

func StartStateid4(buf []byte) ([]byte, Stateid4) {
	buf = append(buf, make([]byte, stateid4Size)...)
	return buf, Stateid4{m: (*[stateid4Size]byte)(buf[len(buf)-stateid4Size:])}
}

func (m Stateid4) Seqid() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m Stateid4) Other(i int) uint8 {
	off := 4 + i*1
	return m.m[off : off+1][0]
}

func (m Stateid4) SetSeqid(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m Stateid4) SetOther(i int, v uint8) {
	off := 4 + i*1
	m.m[off : off+1][0] = v
}

// -------------------------------------------------------
// NfsClientId4 — variable: verifier(8) + id_len(4) + u8 id[id_len] + align(4)
// -------------------------------------------------------

type NfsClientId4 []byte

func readNfsClientId4(b *[]byte) (NfsClientId4, bool) {
	if len(*b) < 12 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[8:12]))
	padded := (n + 3) &^ 3
	total := 12 + padded
	if len(*b) < total {
		return nil, false
	}
	result := NfsClientId4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadNfsClientId4(b []byte) (NfsClientId4, bool) {
	if len(b) < 12 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[8:12]))
	padded := (count + 3) &^ 3
	total := 12 + padded
	if len(b) < total {
		return nil, false
	}
	return NfsClientId4(b[:total]), true
}

func (m NfsClientId4) Verifier() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m[0 : 0+verifier4Size])}
}

func (m NfsClientId4) IdLen() uint32 {
	return binary.BigEndian.Uint32(m[8:12])
}

func (m NfsClientId4) Id() []byte {
	n := int(binary.BigEndian.Uint32(m[8:12]))
	return m[12 : 12+n]
}

// NfsClientId4Writer writes a nfs_client_id4:
//
//	verifier + id_len + u8 id[id_len] + align(4)
type NfsClientId4Writer struct {
	buf   []byte
	off   int
	idLen uint32
}

func StartNfsClientId4(buf []byte) NfsClientId4Writer {
	off := len(buf)
	buf = append(buf, make([]byte, 12)...) // verifier(8) + id_len(4)
	return NfsClientId4Writer{buf: buf, off: off}
}

func (w *NfsClientId4Writer) Verifier() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(w.buf[w.off:])}
}

func (w NfsClientId4Writer) SetId(data []byte) NfsClientId4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	w.buf = append(w.buf, make([]byte, padded)...)
	copy(w.buf[len(w.buf)-padded:], data)
	w.idLen = uint32(n)
	return w
}

func (w NfsClientId4Writer) Finish() []byte {
	binary.BigEndian.PutUint32((*[12]byte)(w.buf[w.off:])[8:12], w.idLen)
	return w.buf
}

// -------------------------------------------------------
// OpenOwner4 — variable:
//   clientid(8) + owner_len(4) + u8 owner[owner_len] + align(4)
// -------------------------------------------------------

type OpenOwner4 []byte

func readOpenOwner4(b *[]byte) (OpenOwner4, bool) {
	if len(*b) < 12 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[8:12]))
	padded := (n + 3) &^ 3
	total := 12 + padded
	if len(*b) < total {
		return nil, false
	}
	result := OpenOwner4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadOpenOwner4(b []byte) (OpenOwner4, bool) {
	if len(b) < 12 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[8:12]))
	padded := (count + 3) &^ 3
	total := 12 + padded
	if len(b) < total {
		return nil, false
	}
	return OpenOwner4(b[:total]), true
}

func (m OpenOwner4) Clientid() uint64 {
	return binary.BigEndian.Uint64(m[0:8])
}

func (m OpenOwner4) OwnerLen() uint32 {
	return binary.BigEndian.Uint32(m[8:12])
}

func (m OpenOwner4) Owner() []byte {
	n := int(binary.BigEndian.Uint32(m[8:12]))
	return m[12 : 12+n]
}

// OpenOwner4Writer writes a open_owner4:
//
//	clientid + owner_len + u8 owner[owner_len] + align(4)
type OpenOwner4Writer struct {
	buf      []byte
	off      int
	ownerLen uint32
}

func StartOpenOwner4(buf []byte) OpenOwner4Writer {
	off := len(buf)
	buf = append(buf, make([]byte, 12)...) // clientid(8) + owner_len(4)
	return OpenOwner4Writer{buf: buf, off: off}
}

func (w OpenOwner4Writer) SetClientid(v uint64) OpenOwner4Writer {
	binary.BigEndian.PutUint64((*[12]byte)(w.buf[w.off:])[0:8], v)
	return w
}

func (w OpenOwner4Writer) SetOwner(data []byte) OpenOwner4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	w.buf = append(w.buf, make([]byte, padded)...)
	copy(w.buf[len(w.buf)-padded:], data)
	w.ownerLen = uint32(n)
	return w
}

func (w OpenOwner4Writer) Finish() []byte {
	binary.BigEndian.PutUint32((*[12]byte)(w.buf[w.off:])[8:12], w.ownerLen)
	return w.buf
}

// -------------------------------------------------------
// LockOwner4 — variable:
//   clientid(8) + owner_len(4) + u8 owner[owner_len] + align(4)
// -------------------------------------------------------

type LockOwner4 []byte

func readLockOwner4(b *[]byte) (LockOwner4, bool) {
	if len(*b) < 12 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[8:12]))
	padded := (n + 3) &^ 3
	total := 12 + padded
	if len(*b) < total {
		return nil, false
	}
	result := LockOwner4((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadLockOwner4(b []byte) (LockOwner4, bool) {
	if len(b) < 12 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[8:12]))
	padded := (count + 3) &^ 3
	total := 12 + padded
	if len(b) < total {
		return nil, false
	}
	return LockOwner4(b[:total]), true
}

func (m LockOwner4) Clientid() uint64 {
	return binary.BigEndian.Uint64(m[0:8])
}

func (m LockOwner4) OwnerLen() uint32 {
	return binary.BigEndian.Uint32(m[8:12])
}

func (m LockOwner4) Owner() []byte {
	n := int(binary.BigEndian.Uint32(m[8:12]))
	return m[12 : 12+n]
}

// LockOwner4Writer writes a lock_owner4:
//
//	clientid + owner_len + u8 owner[owner_len] + align(4)
type LockOwner4Writer struct {
	buf      []byte
	off      int
	ownerLen uint32
}

func StartLockOwner4(buf []byte) LockOwner4Writer {
	off := len(buf)
	buf = append(buf, make([]byte, 12)...) // clientid(8) + owner_len(4)
	return LockOwner4Writer{buf: buf, off: off}
}

func (w LockOwner4Writer) SetClientid(v uint64) LockOwner4Writer {
	binary.BigEndian.PutUint64((*[12]byte)(w.buf[w.off:])[0:8], v)
	return w
}

func (w LockOwner4Writer) SetOwner(data []byte) LockOwner4Writer {
	n := len(data)
	padded := (n + 3) &^ 3
	w.buf = append(w.buf, make([]byte, padded)...)
	copy(w.buf[len(w.buf)-padded:], data)
	w.ownerLen = uint32(n)
	return w
}

func (w LockOwner4Writer) Finish() []byte {
	binary.BigEndian.PutUint32((*[12]byte)(w.buf[w.off:])[8:12], w.ownerLen)
	return w.buf
}

// -------------------------------------------------------
// ACCESS4args — fixed 4 bytes: access(4, beu32)
// -------------------------------------------------------

type ACCESS4args struct {
	m *[aCCESS4argsSize]byte
}

const aCCESS4argsSize = 4

func readACCESS4args(b *[]byte) (ACCESS4args, bool) {
	if len(*b) < aCCESS4argsSize {
		return ACCESS4args{}, false
	}
	result := ACCESS4args{m: (*[aCCESS4argsSize]byte)(*b)}
	*b = (*b)[aCCESS4argsSize:]
	return result, true
}

func ReadACCESS4args(b []byte) (ACCESS4args, bool) {
	return readACCESS4args(&b)
}

func StartACCESS4args(buf []byte) ([]byte, ACCESS4args) {
	buf = append(buf, make([]byte, aCCESS4argsSize)...)
	return buf, ACCESS4args{m: (*[aCCESS4argsSize]byte)(buf[len(buf)-aCCESS4argsSize:])}
}

func (m ACCESS4args) Access() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m ACCESS4args) SetAccess(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// ACCESS4resok — fixed 8 bytes: supported(4, beu32) + access(4, beu32)
// -------------------------------------------------------

type ACCESS4resok struct {
	m *[aCCESS4resokSize]byte
}

const aCCESS4resokSize = 8

func readACCESS4resok(b *[]byte) (ACCESS4resok, bool) {
	if len(*b) < aCCESS4resokSize {
		return ACCESS4resok{}, false
	}
	result := ACCESS4resok{m: (*[aCCESS4resokSize]byte)(*b)}
	*b = (*b)[aCCESS4resokSize:]
	return result, true
}

func ReadACCESS4resok(b []byte) (ACCESS4resok, bool) {
	return readACCESS4resok(&b)
}

func StartACCESS4resok(buf []byte) ([]byte, ACCESS4resok) {
	buf = append(buf, make([]byte, aCCESS4resokSize)...)
	return buf, ACCESS4resok{m: (*[aCCESS4resokSize]byte)(buf[len(buf)-aCCESS4resokSize:])}
}

func (m ACCESS4resok) Supported() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m ACCESS4resok) Access() uint32 {
	return binary.BigEndian.Uint32(m.m[4:8])
}

func (m ACCESS4resok) SetSupported(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m ACCESS4resok) SetAccess(v uint32) {
	binary.BigEndian.PutUint32(m.m[4:8], v)
}

// -------------------------------------------------------
// ACCESS4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type ACCESS4res struct {
	b    []byte
	disc uint32
}

func readACCESS4res(b *[]byte, nfsstat4 uint32) (ACCESS4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readACCESS4resok(b)
		if !ok {
			return ACCESS4res{}, false
		}
		return ACCESS4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return ACCESS4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m ACCESS4res) AsACCESS4resok() ACCESS4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return ACCESS4resok{m: (*[aCCESS4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// CLOSE4args — fixed 20 bytes: seqid(4, beu32) + stateid4(16)
// -------------------------------------------------------

type CLOSE4args struct {
	m *[cLOSE4argsSize]byte
}

const cLOSE4argsSize = 20

func readCLOSE4args(b *[]byte) (CLOSE4args, bool) {
	if len(*b) < cLOSE4argsSize {
		return CLOSE4args{}, false
	}
	result := CLOSE4args{m: (*[cLOSE4argsSize]byte)(*b)}
	*b = (*b)[cLOSE4argsSize:]
	return result, true
}

func ReadCLOSE4args(b []byte) (CLOSE4args, bool) {
	return readCLOSE4args(&b)
}

func StartCLOSE4args(buf []byte) ([]byte, CLOSE4args) {
	buf = append(buf, make([]byte, cLOSE4argsSize)...)
	return buf, CLOSE4args{m: (*[cLOSE4argsSize]byte)(buf[len(buf)-cLOSE4argsSize:])}
}

func (m CLOSE4args) Seqid() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m CLOSE4args) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[4:20])}
}

func (m CLOSE4args) SetSeqid(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// CLOSE4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type CLOSE4res struct {
	b    []byte
	disc uint32
}

func readCLOSE4res(b *[]byte, nfsstat4 uint32) (CLOSE4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readStateid4(b)
		if !ok {
			return CLOSE4res{}, false
		}
		return CLOSE4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return CLOSE4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m CLOSE4res) AsStateid4() Stateid4 {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return Stateid4{m: (*[stateid4Size]byte)(m.b)}
}

// -------------------------------------------------------
// COMMIT4args — fixed 12 bytes: offset(8, beu64) + count(4, beu32)
// -------------------------------------------------------

type COMMIT4args struct {
	m *[cOMMIT4argsSize]byte
}

const cOMMIT4argsSize = 12

func readCOMMIT4args(b *[]byte) (COMMIT4args, bool) {
	if len(*b) < cOMMIT4argsSize {
		return COMMIT4args{}, false
	}
	result := COMMIT4args{m: (*[cOMMIT4argsSize]byte)(*b)}
	*b = (*b)[cOMMIT4argsSize:]
	return result, true
}

func ReadCOMMIT4args(b []byte) (COMMIT4args, bool) {
	return readCOMMIT4args(&b)
}

func StartCOMMIT4args(buf []byte) ([]byte, COMMIT4args) {
	buf = append(buf, make([]byte, cOMMIT4argsSize)...)
	return buf, COMMIT4args{m: (*[cOMMIT4argsSize]byte)(buf[len(buf)-cOMMIT4argsSize:])}
}

func (m COMMIT4args) Offset() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m COMMIT4args) Count() uint32 {
	return binary.BigEndian.Uint32(m.m[8:12])
}

func (m COMMIT4args) SetOffset(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

func (m COMMIT4args) SetCount(v uint32) {
	binary.BigEndian.PutUint32(m.m[8:12], v)
}

// -------------------------------------------------------
// COMMIT4resok — fixed 8 bytes: verifier4(8)
// -------------------------------------------------------

type COMMIT4resok struct {
	m *[cOMMIT4resokSize]byte
}

const cOMMIT4resokSize = 8

func readCOMMIT4resok(b *[]byte) (COMMIT4resok, bool) {
	if len(*b) < cOMMIT4resokSize {
		return COMMIT4resok{}, false
	}
	result := COMMIT4resok{m: (*[cOMMIT4resokSize]byte)(*b)}
	*b = (*b)[cOMMIT4resokSize:]
	return result, true
}

func ReadCOMMIT4resok(b []byte) (COMMIT4resok, bool) {
	return readCOMMIT4resok(&b)
}

func StartCOMMIT4resok(buf []byte) ([]byte, COMMIT4resok) {
	buf = append(buf, make([]byte, cOMMIT4resokSize)...)
	return buf, COMMIT4resok{m: (*[cOMMIT4resokSize]byte)(buf[len(buf)-cOMMIT4resokSize:])}
}

func (m COMMIT4resok) Writeverf() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m.m[0:8])}
}

// -------------------------------------------------------
// COMMIT4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type COMMIT4res struct {
	b    []byte
	disc uint32
}

func readCOMMIT4res(b *[]byte, nfsstat4 uint32) (COMMIT4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readCOMMIT4resok(b)
		if !ok {
			return COMMIT4res{}, false
		}
		return COMMIT4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return COMMIT4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m COMMIT4res) AsCOMMIT4resok() COMMIT4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return COMMIT4resok{m: (*[cOMMIT4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// Createtype4 — union on nfs_ftype4 (external discriminant)
// -------------------------------------------------------

type Createtype4 struct {
	b    []byte
	disc uint32
}

func readCreatetype4(b *[]byte, nfsFtype4 uint32) (Createtype4, bool) {
	switch nfsFtype4 {
	case NF4LNK:
		r, ok := readLinktext4(b)
		if !ok {
			return Createtype4{}, false
		}
		return Createtype4{b: []byte(r), disc: nfsFtype4}, true
	case NF4BLK:
		r, ok := readSpecdata4(b)
		if !ok {
			return Createtype4{}, false
		}
		return Createtype4{b: r.m[:], disc: nfsFtype4}, true
	case NF4CHR:
		r, ok := readSpecdata4(b)
		if !ok {
			return Createtype4{}, false
		}
		return Createtype4{b: r.m[:], disc: nfsFtype4}, true
	case NF4SOCK:
		return Createtype4{b: (*b)[:0], disc: nfsFtype4}, true
	case NF4FIFO:
		return Createtype4{b: (*b)[:0], disc: nfsFtype4}, true
	case NF4DIR:
		return Createtype4{b: (*b)[:0], disc: nfsFtype4}, true
	default:
		return Createtype4{b: (*b)[:0], disc: nfsFtype4}, true
	}
}

func (m Createtype4) AsLinktext4() Linktext4 {
	if m.disc != NF4LNK {
		panic("wrong union discriminant")
	}
	return Linktext4(m.b)
}

func (m Createtype4) AsNf4blk() Specdata4 {
	if m.disc != NF4BLK {
		panic("wrong union discriminant")
	}
	return Specdata4{m: (*[specdata4Size]byte)(m.b)}
}

func (m Createtype4) AsNf4chr() Specdata4 {
	if m.disc != NF4CHR {
		panic("wrong union discriminant")
	}
	return Specdata4{m: (*[specdata4Size]byte)(m.b)}
}

// -------------------------------------------------------
// CREATE4args — variable:
//   objtype_type(4) + createtype4 objtype + objname + createattrs
// -------------------------------------------------------

type CREATE4args struct {
	data []byte
	off1 int // byte offset within data where objname starts
	off2 int // byte offset within data where createattrs starts
}

func readCREATE4args(b *[]byte) (CREATE4args, bool) {
	if len(*b) < 4 {
		return CREATE4args{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readCreatetype4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return CREATE4args{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readComponent4(b); !ok {
		return CREATE4args{}, false
	}
	off2 := startLen - len(*b)
	if _, ok := readFattr4(b); !ok {
		return CREATE4args{}, false
	}
	total := startLen - len(*b)
	return CREATE4args{data: start[:total], off1: off1, off2: off2}, true
}

func ReadCREATE4args(b []byte) (CREATE4args, bool) {
	return readCREATE4args(&b)
}

func (m CREATE4args) ObjtypeType() uint32 {
	return binary.BigEndian.Uint32(m.data[0:4])
}

func (m CREATE4args) Objtype() Createtype4 {
	return Createtype4{b: m.data[4:m.off1], disc: binary.BigEndian.Uint32(m.data[0:4])}
}

func (m CREATE4args) Objname() Component4 {
	return Component4(m.data[m.off1:m.off2])
}

func (m CREATE4args) Createattrs() Fattr4 {
	v, _ := ReadFattr4(m.data[m.off2:])
	return v
}

// CREATE4argsWriter writes a CREATE4args:
//
//	objtype_type + objtype(objtype_type) + objname + createattrs
type CREATE4argsWriter struct {
	buf    []byte
	header *[4]byte
	phase  uint8
}

func StartCREATE4args(buf []byte) CREATE4argsWriter {
	buf = append(buf, make([]byte, 4)...) // objtype_type(4)
	return CREATE4argsWriter{buf: buf, header: (*[4]byte)(buf[len(buf)-4:])}
}

func (w *CREATE4argsWriter) SetObjtypeType(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *CREATE4argsWriter) SetObjtype_Nf4lnk() Linktext4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	binary.BigEndian.PutUint32(w.header[0:4], NF4LNK)
	child := StartLinktext4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *CREATE4argsWriter) SetObjtype_Nf4blk() Specdata4 {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	binary.BigEndian.PutUint32(w.header[0:4], NF4BLK)
	w.buf = append(w.buf, make([]byte, specdata4Size)...)
	return Specdata4{m: (*[specdata4Size]byte)(w.buf[len(w.buf)-specdata4Size:])}
}

func (w *CREATE4argsWriter) SetObjtype_Nf4chr() Specdata4 {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	binary.BigEndian.PutUint32(w.header[0:4], NF4CHR)
	w.buf = append(w.buf, make([]byte, specdata4Size)...)
	return Specdata4{m: (*[specdata4Size]byte)(w.buf[len(w.buf)-specdata4Size:])}
}

func (w *CREATE4argsWriter) SetObjtype_Nf4sock() {
	binary.BigEndian.PutUint32(w.header[0:4], NF4SOCK)
}

func (w *CREATE4argsWriter) SetObjtype_Nf4fifo() {
	binary.BigEndian.PutUint32(w.header[0:4], NF4FIFO)
}

func (w *CREATE4argsWriter) SetObjtype_Nf4dir() {
	binary.BigEndian.PutUint32(w.header[0:4], NF4DIR)
}

func (w *CREATE4argsWriter) SetObjtype_Default(nfsFtype4 uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], nfsFtype4)
}

func (w *CREATE4argsWriter) StartObjname() Component4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartComponent4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *CREATE4argsWriter) StartCreateattrs() Fattr4Writer {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	child := StartFattr4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *CREATE4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CREATE4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CREATE4resok — variable: cinfo(20) + attrset
// -------------------------------------------------------

type CREATE4resok []byte

func readCREATE4resok(b *[]byte) (CREATE4resok, bool) {
	if len(*b) < 20 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[20:]
	if _, ok := readBitmap4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CREATE4resok(start[:total]), true
}

func ReadCREATE4resok(b []byte) (CREATE4resok, bool) {
	return readCREATE4resok(&b)
}

func (m CREATE4resok) Cinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(m[0 : 0+changeInfo4Size])}
}

func (m CREATE4resok) Attrset() Bitmap4 {
	return Bitmap4(m[20:])
}

// CREATE4resokWriter writes a CREATE4resok:
//
//	cinfo + attrset
type CREATE4resokWriter struct {
	buf    []byte
	header *[20]byte
}

func StartCREATE4resok(buf []byte) CREATE4resokWriter {
	buf = append(buf, make([]byte, 20)...) // cinfo(20)
	return CREATE4resokWriter{buf: buf, header: (*[20]byte)(buf[len(buf)-20:])}
}

func (w *CREATE4resokWriter) Cinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(w.header[0:])}
}

func (w *CREATE4resokWriter) StartAttrset() Bitmap4Writer {
	child := StartBitmap4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *CREATE4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CREATE4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CREATE4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type CREATE4res struct {
	b    []byte
	disc uint32
}

func readCREATE4res(b *[]byte, nfsstat4 uint32) (CREATE4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readCREATE4resok(b)
		if !ok {
			return CREATE4res{}, false
		}
		return CREATE4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return CREATE4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m CREATE4res) AsCREATE4resok() CREATE4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return CREATE4resok(m.b)
}

// -------------------------------------------------------
// DELEGPURGE4args — fixed 8 bytes: clientid(8, beu64)
// -------------------------------------------------------

type DELEGPURGE4args struct {
	m *[dELEGPURGE4argsSize]byte
}

const dELEGPURGE4argsSize = 8

func readDELEGPURGE4args(b *[]byte) (DELEGPURGE4args, bool) {
	if len(*b) < dELEGPURGE4argsSize {
		return DELEGPURGE4args{}, false
	}
	result := DELEGPURGE4args{m: (*[dELEGPURGE4argsSize]byte)(*b)}
	*b = (*b)[dELEGPURGE4argsSize:]
	return result, true
}

func ReadDELEGPURGE4args(b []byte) (DELEGPURGE4args, bool) {
	return readDELEGPURGE4args(&b)
}

func StartDELEGPURGE4args(buf []byte) ([]byte, DELEGPURGE4args) {
	buf = append(buf, make([]byte, dELEGPURGE4argsSize)...)
	return buf, DELEGPURGE4args{m: (*[dELEGPURGE4argsSize]byte)(buf[len(buf)-dELEGPURGE4argsSize:])}
}

func (m DELEGPURGE4args) Clientid() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m DELEGPURGE4args) SetClientid(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

// -------------------------------------------------------
// DELEGPURGE4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type DELEGPURGE4res struct {
	m *[dELEGPURGE4resSize]byte
}

const dELEGPURGE4resSize = 4

func readDELEGPURGE4res(b *[]byte) (DELEGPURGE4res, bool) {
	if len(*b) < dELEGPURGE4resSize {
		return DELEGPURGE4res{}, false
	}
	result := DELEGPURGE4res{m: (*[dELEGPURGE4resSize]byte)(*b)}
	*b = (*b)[dELEGPURGE4resSize:]
	return result, true
}

func ReadDELEGPURGE4res(b []byte) (DELEGPURGE4res, bool) {
	return readDELEGPURGE4res(&b)
}

func StartDELEGPURGE4res(buf []byte) ([]byte, DELEGPURGE4res) {
	buf = append(buf, make([]byte, dELEGPURGE4resSize)...)
	return buf, DELEGPURGE4res{m: (*[dELEGPURGE4resSize]byte)(buf[len(buf)-dELEGPURGE4resSize:])}
}

func (m DELEGPURGE4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m DELEGPURGE4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// DELEGRETURN4args — fixed 16 bytes: stateid4(16)
// -------------------------------------------------------

type DELEGRETURN4args struct {
	m *[dELEGRETURN4argsSize]byte
}

const dELEGRETURN4argsSize = 16

func readDELEGRETURN4args(b *[]byte) (DELEGRETURN4args, bool) {
	if len(*b) < dELEGRETURN4argsSize {
		return DELEGRETURN4args{}, false
	}
	result := DELEGRETURN4args{m: (*[dELEGRETURN4argsSize]byte)(*b)}
	*b = (*b)[dELEGRETURN4argsSize:]
	return result, true
}

func ReadDELEGRETURN4args(b []byte) (DELEGRETURN4args, bool) {
	return readDELEGRETURN4args(&b)
}

func StartDELEGRETURN4args(buf []byte) ([]byte, DELEGRETURN4args) {
	buf = append(buf, make([]byte, dELEGRETURN4argsSize)...)
	return buf, DELEGRETURN4args{m: (*[dELEGRETURN4argsSize]byte)(buf[len(buf)-dELEGRETURN4argsSize:])}
}

func (m DELEGRETURN4args) DelegStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

// -------------------------------------------------------
// DELEGRETURN4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type DELEGRETURN4res struct {
	m *[dELEGRETURN4resSize]byte
}

const dELEGRETURN4resSize = 4

func readDELEGRETURN4res(b *[]byte) (DELEGRETURN4res, bool) {
	if len(*b) < dELEGRETURN4resSize {
		return DELEGRETURN4res{}, false
	}
	result := DELEGRETURN4res{m: (*[dELEGRETURN4resSize]byte)(*b)}
	*b = (*b)[dELEGRETURN4resSize:]
	return result, true
}

func ReadDELEGRETURN4res(b []byte) (DELEGRETURN4res, bool) {
	return readDELEGRETURN4res(&b)
}

func StartDELEGRETURN4res(buf []byte) ([]byte, DELEGRETURN4res) {
	buf = append(buf, make([]byte, dELEGRETURN4resSize)...)
	return buf, DELEGRETURN4res{m: (*[dELEGRETURN4resSize]byte)(buf[len(buf)-dELEGRETURN4resSize:])}
}

func (m DELEGRETURN4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m DELEGRETURN4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// GETATTR4args — variable: attr_request
// -------------------------------------------------------

type GETATTR4args []byte

func readGETATTR4args(b *[]byte) (GETATTR4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readBitmap4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return GETATTR4args(start[:total]), true
}

func ReadGETATTR4args(b []byte) (GETATTR4args, bool) {
	return readGETATTR4args(&b)
}

func (m GETATTR4args) AttrRequest() Bitmap4 {
	return Bitmap4(m[0:])
}

// GETATTR4argsWriter writes a GETATTR4args:
//
//	attr_request
type GETATTR4argsWriter struct {
	buf []byte
	off int
}

func StartGETATTR4args(buf []byte) GETATTR4argsWriter {
	off := len(buf)
	return GETATTR4argsWriter{buf: buf, off: off}
}

func (w *GETATTR4argsWriter) StartAttrRequest() Bitmap4Writer {
	child := StartBitmap4(w.buf)
	w.buf = nil
	return child
}

func (w *GETATTR4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *GETATTR4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// GETATTR4resok — variable: obj_attributes
// -------------------------------------------------------

type GETATTR4resok []byte

func readGETATTR4resok(b *[]byte) (GETATTR4resok, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readFattr4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return GETATTR4resok(start[:total]), true
}

func ReadGETATTR4resok(b []byte) (GETATTR4resok, bool) {
	return readGETATTR4resok(&b)
}

func (m GETATTR4resok) ObjAttributes() Fattr4 {
	v, _ := ReadFattr4(m[0:])
	return v
}

// GETATTR4resokWriter writes a GETATTR4resok:
//
//	obj_attributes
type GETATTR4resokWriter struct {
	buf []byte
	off int
}

func StartGETATTR4resok(buf []byte) GETATTR4resokWriter {
	off := len(buf)
	return GETATTR4resokWriter{buf: buf, off: off}
}

func (w *GETATTR4resokWriter) StartObjAttributes() Fattr4Writer {
	child := StartFattr4(w.buf)
	w.buf = nil
	return child
}

func (w *GETATTR4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *GETATTR4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// GETATTR4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type GETATTR4res struct {
	b    []byte
	disc uint32
}

func readGETATTR4res(b *[]byte, nfsstat4 uint32) (GETATTR4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readGETATTR4resok(b)
		if !ok {
			return GETATTR4res{}, false
		}
		return GETATTR4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return GETATTR4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m GETATTR4res) AsGETATTR4resok() GETATTR4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return GETATTR4resok(m.b)
}

// -------------------------------------------------------
// GETFH4resok — variable: object
// -------------------------------------------------------

type GETFH4resok []byte

func readGETFH4resok(b *[]byte) (GETFH4resok, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readNfsFh4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return GETFH4resok(start[:total]), true
}

func ReadGETFH4resok(b []byte) (GETFH4resok, bool) {
	return readGETFH4resok(&b)
}

func (m GETFH4resok) Object() NfsFh4 {
	return NfsFh4(m[0:])
}

// GETFH4resokWriter writes a GETFH4resok:
//
//	object
type GETFH4resokWriter struct {
	buf []byte
	off int
}

func StartGETFH4resok(buf []byte) GETFH4resokWriter {
	off := len(buf)
	return GETFH4resokWriter{buf: buf, off: off}
}

func (w *GETFH4resokWriter) StartObject() NfsFh4Writer {
	child := StartNfsFh4(w.buf)
	w.buf = nil
	return child
}

func (w *GETFH4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *GETFH4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// GETFH4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type GETFH4res struct {
	b    []byte
	disc uint32
}

func readGETFH4res(b *[]byte, nfsstat4 uint32) (GETFH4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readGETFH4resok(b)
		if !ok {
			return GETFH4res{}, false
		}
		return GETFH4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return GETFH4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m GETFH4res) AsGETFH4resok() GETFH4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return GETFH4resok(m.b)
}

// -------------------------------------------------------
// LINK4args — variable: newname
// -------------------------------------------------------

type LINK4args []byte

func readLINK4args(b *[]byte) (LINK4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readComponent4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LINK4args(start[:total]), true
}

func ReadLINK4args(b []byte) (LINK4args, bool) {
	return readLINK4args(&b)
}

func (m LINK4args) Newname() Component4 {
	return Component4(m[0:])
}

// LINK4argsWriter writes a LINK4args:
//
//	newname
type LINK4argsWriter struct {
	buf []byte
	off int
}

func StartLINK4args(buf []byte) LINK4argsWriter {
	off := len(buf)
	return LINK4argsWriter{buf: buf, off: off}
}

func (w *LINK4argsWriter) StartNewname() Component4Writer {
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *LINK4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LINK4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LINK4resok — fixed 20 bytes: change_info4(20)
// -------------------------------------------------------

type LINK4resok struct {
	m *[lINK4resokSize]byte
}

const lINK4resokSize = 20

func readLINK4resok(b *[]byte) (LINK4resok, bool) {
	if len(*b) < lINK4resokSize {
		return LINK4resok{}, false
	}
	result := LINK4resok{m: (*[lINK4resokSize]byte)(*b)}
	*b = (*b)[lINK4resokSize:]
	return result, true
}

func ReadLINK4resok(b []byte) (LINK4resok, bool) {
	return readLINK4resok(&b)
}

func StartLINK4resok(buf []byte) ([]byte, LINK4resok) {
	buf = append(buf, make([]byte, lINK4resokSize)...)
	return buf, LINK4resok{m: (*[lINK4resokSize]byte)(buf[len(buf)-lINK4resokSize:])}
}

func (m LINK4resok) Cinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(m.m[0:20])}
}

// -------------------------------------------------------
// LINK4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type LINK4res struct {
	b    []byte
	disc uint32
}

func readLINK4res(b *[]byte, nfsstat4 uint32) (LINK4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readLINK4resok(b)
		if !ok {
			return LINK4res{}, false
		}
		return LINK4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return LINK4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m LINK4res) AsLINK4resok() LINK4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return LINK4resok{m: (*[lINK4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// OpenToLockOwner4 — variable:
//   open_seqid(4) + open_stateid(16) + lock_seqid(4) + lock_owner
// -------------------------------------------------------

type OpenToLockOwner4 []byte

func readOpenToLockOwner4(b *[]byte) (OpenToLockOwner4, bool) {
	if len(*b) < 24 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[24:]
	if _, ok := readLockOwner4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return OpenToLockOwner4(start[:total]), true
}

func ReadOpenToLockOwner4(b []byte) (OpenToLockOwner4, bool) {
	return readOpenToLockOwner4(&b)
}

func (m OpenToLockOwner4) OpenSeqid() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m OpenToLockOwner4) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m[4 : 4+stateid4Size])}
}

func (m OpenToLockOwner4) LockSeqid() uint32 {
	return binary.BigEndian.Uint32(m[20:24])
}

func (m OpenToLockOwner4) LockOwner() LockOwner4 {
	return LockOwner4(m[24:])
}

// OpenToLockOwner4Writer writes a open_to_lock_owner4:
//
//	open_seqid + open_stateid + lock_seqid + lock_owner
type OpenToLockOwner4Writer struct {
	buf    []byte
	header *[24]byte
}

func StartOpenToLockOwner4(buf []byte) OpenToLockOwner4Writer {
	buf = append(buf, make([]byte, 24)...) // open_seqid(4) + open_stateid(16) + lock_seqid(4)
	return OpenToLockOwner4Writer{buf: buf, header: (*[24]byte)(buf[len(buf)-24:])}
}

func (w *OpenToLockOwner4Writer) SetOpenSeqid(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *OpenToLockOwner4Writer) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[4:])}
}

func (w *OpenToLockOwner4Writer) SetLockSeqid(v uint32) {
	binary.BigEndian.PutUint32(w.header[20:24], v)
}

func (w *OpenToLockOwner4Writer) StartLockOwner() LockOwner4Writer {
	child := StartLockOwner4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *OpenToLockOwner4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *OpenToLockOwner4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// ExistLockOwner4 — fixed 20 bytes: stateid4(16) + lock_seqid(4, beu32)
// -------------------------------------------------------

type ExistLockOwner4 struct {
	m *[existLockOwner4Size]byte
}

const existLockOwner4Size = 20

func readExistLockOwner4(b *[]byte) (ExistLockOwner4, bool) {
	if len(*b) < existLockOwner4Size {
		return ExistLockOwner4{}, false
	}
	result := ExistLockOwner4{m: (*[existLockOwner4Size]byte)(*b)}
	*b = (*b)[existLockOwner4Size:]
	return result, true
}

func ReadExistLockOwner4(b []byte) (ExistLockOwner4, bool) {
	return readExistLockOwner4(&b)
}

func StartExistLockOwner4(buf []byte) ([]byte, ExistLockOwner4) {
	buf = append(buf, make([]byte, existLockOwner4Size)...)
	return buf, ExistLockOwner4{m: (*[existLockOwner4Size]byte)(buf[len(buf)-existLockOwner4Size:])}
}

func (m ExistLockOwner4) LockStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

func (m ExistLockOwner4) LockSeqid() uint32 {
	return binary.BigEndian.Uint32(m.m[16:20])
}

func (m ExistLockOwner4) SetLockSeqid(v uint32) {
	binary.BigEndian.PutUint32(m.m[16:20], v)
}

// -------------------------------------------------------
// Locker4 — union on xdr_bool (external discriminant)
// -------------------------------------------------------

type Locker4 struct {
	b    []byte
	disc uint32
}

func readLocker4(b *[]byte, xdrBool uint32) (Locker4, bool) {
	switch xdrBool {
	case TRUE:
		r, ok := readOpenToLockOwner4(b)
		if !ok {
			return Locker4{}, false
		}
		return Locker4{b: []byte(r), disc: xdrBool}, true
	case FALSE:
		r, ok := readExistLockOwner4(b)
		if !ok {
			return Locker4{}, false
		}
		return Locker4{b: r.m[:], disc: xdrBool}, true
	default:
		return Locker4{}, false
	}
}

func (m Locker4) AsOpenToLockOwner4() OpenToLockOwner4 {
	if m.disc != TRUE {
		panic("wrong union discriminant")
	}
	return OpenToLockOwner4(m.b)
}

func (m Locker4) AsExistLockOwner4() ExistLockOwner4 {
	if m.disc != FALSE {
		panic("wrong union discriminant")
	}
	return ExistLockOwner4{m: (*[existLockOwner4Size]byte)(m.b)}
}

// -------------------------------------------------------
// LOCK4args — variable:
//   locktype(4) + reclaim(4) + offset(8) + length(8) + locker_type(4) + locker4 locker
// -------------------------------------------------------

type LOCK4args []byte

func readLOCK4args(b *[]byte) (LOCK4args, bool) {
	if len(*b) < 28 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[28:]
	if _, ok := readLocker4(b, binary.BigEndian.Uint32(start[24:28])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOCK4args(start[:total]), true
}

func ReadLOCK4args(b []byte) (LOCK4args, bool) {
	return readLOCK4args(&b)
}

func (m LOCK4args) Locktype() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m LOCK4args) Reclaim() uint32 {
	return binary.BigEndian.Uint32(m[4:8])
}

func (m LOCK4args) Offset() uint64 {
	return binary.BigEndian.Uint64(m[8:16])
}

func (m LOCK4args) Length() uint64 {
	return binary.BigEndian.Uint64(m[16:24])
}

func (m LOCK4args) LockerType() uint32 {
	return binary.BigEndian.Uint32(m[24:28])
}

func (m LOCK4args) Locker() Locker4 {
	return Locker4{b: m[28:], disc: binary.BigEndian.Uint32(m[24:28])}
}

// LOCK4argsWriter writes a LOCK4args:
//
//	locktype + reclaim + offset + length + locker_type + locker(locker_type)
type LOCK4argsWriter struct {
	buf    []byte
	header *[28]byte
}

func StartLOCK4args(buf []byte) LOCK4argsWriter {
	buf = append(buf, make([]byte, 28)...) // locktype(4) + reclaim(4) + offset(8) + length(8) + locker_type(4)
	return LOCK4argsWriter{buf: buf, header: (*[28]byte)(buf[len(buf)-28:])}
}

func (w *LOCK4argsWriter) SetLocktype(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *LOCK4argsWriter) SetReclaim(v uint32) {
	binary.BigEndian.PutUint32(w.header[4:8], v)
}

func (w *LOCK4argsWriter) SetOffset(v uint64) {
	binary.BigEndian.PutUint64(w.header[8:16], v)
}

func (w *LOCK4argsWriter) SetLength(v uint64) {
	binary.BigEndian.PutUint64(w.header[16:24], v)
}

func (w *LOCK4argsWriter) SetLockerType(v uint32) {
	binary.BigEndian.PutUint32(w.header[24:28], v)
}

func (w *LOCK4argsWriter) SetLocker_True() OpenToLockOwner4Writer {
	binary.BigEndian.PutUint32(w.header[24:28], TRUE)
	w.buf = append(w.buf, make([]byte, 24)...)
	buf := w.buf
	w.buf = nil
	return OpenToLockOwner4Writer{buf: buf, header: (*[24]byte)(buf[len(buf)-24:])}
}

func (w *LOCK4argsWriter) SetLocker_False() ExistLockOwner4 {
	binary.BigEndian.PutUint32(w.header[24:28], FALSE)
	w.buf = append(w.buf, make([]byte, existLockOwner4Size)...)
	return ExistLockOwner4{m: (*[existLockOwner4Size]byte)(w.buf[len(w.buf)-existLockOwner4Size:])}
}

func (w *LOCK4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LOCK4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LOCK4denied — variable: offset(8) + length(8) + locktype(4) + owner
// -------------------------------------------------------

type LOCK4denied []byte

func readLOCK4denied(b *[]byte) (LOCK4denied, bool) {
	if len(*b) < 20 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[20:]
	if _, ok := readLockOwner4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOCK4denied(start[:total]), true
}

func ReadLOCK4denied(b []byte) (LOCK4denied, bool) {
	return readLOCK4denied(&b)
}

func (m LOCK4denied) Offset() uint64 {
	return binary.BigEndian.Uint64(m[0:8])
}

func (m LOCK4denied) Length() uint64 {
	return binary.BigEndian.Uint64(m[8:16])
}

func (m LOCK4denied) Locktype() uint32 {
	return binary.BigEndian.Uint32(m[16:20])
}

func (m LOCK4denied) Owner() LockOwner4 {
	return LockOwner4(m[20:])
}

// LOCK4deniedWriter writes a LOCK4denied:
//
//	offset + length + locktype + owner
type LOCK4deniedWriter struct {
	buf    []byte
	header *[20]byte
}

func StartLOCK4denied(buf []byte) LOCK4deniedWriter {
	buf = append(buf, make([]byte, 20)...) // offset(8) + length(8) + locktype(4)
	return LOCK4deniedWriter{buf: buf, header: (*[20]byte)(buf[len(buf)-20:])}
}

func (w *LOCK4deniedWriter) SetOffset(v uint64) {
	binary.BigEndian.PutUint64(w.header[0:8], v)
}

func (w *LOCK4deniedWriter) SetLength(v uint64) {
	binary.BigEndian.PutUint64(w.header[8:16], v)
}

func (w *LOCK4deniedWriter) SetLocktype(v uint32) {
	binary.BigEndian.PutUint32(w.header[16:20], v)
}

func (w *LOCK4deniedWriter) StartOwner() LockOwner4Writer {
	child := StartLockOwner4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *LOCK4deniedWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LOCK4deniedWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LOCK4resok — fixed 16 bytes: stateid4(16)
// -------------------------------------------------------

type LOCK4resok struct {
	m *[lOCK4resokSize]byte
}

const lOCK4resokSize = 16

func readLOCK4resok(b *[]byte) (LOCK4resok, bool) {
	if len(*b) < lOCK4resokSize {
		return LOCK4resok{}, false
	}
	result := LOCK4resok{m: (*[lOCK4resokSize]byte)(*b)}
	*b = (*b)[lOCK4resokSize:]
	return result, true
}

func ReadLOCK4resok(b []byte) (LOCK4resok, bool) {
	return readLOCK4resok(&b)
}

func StartLOCK4resok(buf []byte) ([]byte, LOCK4resok) {
	buf = append(buf, make([]byte, lOCK4resokSize)...)
	return buf, LOCK4resok{m: (*[lOCK4resokSize]byte)(buf[len(buf)-lOCK4resokSize:])}
}

func (m LOCK4resok) LockStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

// -------------------------------------------------------
// LOCK4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type LOCK4res struct {
	b    []byte
	disc uint32
}

func readLOCK4res(b *[]byte, nfsstat4 uint32) (LOCK4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readLOCK4resok(b)
		if !ok {
			return LOCK4res{}, false
		}
		return LOCK4res{b: r.m[:], disc: nfsstat4}, true
	case NFS4ERR_DENIED:
		r, ok := readLOCK4denied(b)
		if !ok {
			return LOCK4res{}, false
		}
		return LOCK4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return LOCK4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m LOCK4res) AsLOCK4resok() LOCK4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return LOCK4resok{m: (*[lOCK4resokSize]byte)(m.b)}
}

func (m LOCK4res) AsLOCK4denied() LOCK4denied {
	if m.disc != NFS4ERR_DENIED {
		panic("wrong union discriminant")
	}
	return LOCK4denied(m.b)
}

// -------------------------------------------------------
// LOCKT4args — variable: locktype(4) + offset(8) + length(8) + owner
// -------------------------------------------------------

type LOCKT4args []byte

func readLOCKT4args(b *[]byte) (LOCKT4args, bool) {
	if len(*b) < 20 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[20:]
	if _, ok := readLockOwner4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOCKT4args(start[:total]), true
}

func ReadLOCKT4args(b []byte) (LOCKT4args, bool) {
	return readLOCKT4args(&b)
}

func (m LOCKT4args) Locktype() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m LOCKT4args) Offset() uint64 {
	return binary.BigEndian.Uint64(m[4:12])
}

func (m LOCKT4args) Length() uint64 {
	return binary.BigEndian.Uint64(m[12:20])
}

func (m LOCKT4args) Owner() LockOwner4 {
	return LockOwner4(m[20:])
}

// LOCKT4argsWriter writes a LOCKT4args:
//
//	locktype + offset + length + owner
type LOCKT4argsWriter struct {
	buf    []byte
	header *[20]byte
}

func StartLOCKT4args(buf []byte) LOCKT4argsWriter {
	buf = append(buf, make([]byte, 20)...) // locktype(4) + offset(8) + length(8)
	return LOCKT4argsWriter{buf: buf, header: (*[20]byte)(buf[len(buf)-20:])}
}

func (w *LOCKT4argsWriter) SetLocktype(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *LOCKT4argsWriter) SetOffset(v uint64) {
	binary.BigEndian.PutUint64(w.header[4:12], v)
}

func (w *LOCKT4argsWriter) SetLength(v uint64) {
	binary.BigEndian.PutUint64(w.header[12:20], v)
}

func (w *LOCKT4argsWriter) StartOwner() LockOwner4Writer {
	child := StartLockOwner4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *LOCKT4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LOCKT4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LOCKT4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type LOCKT4res struct {
	b    []byte
	disc uint32
}

func readLOCKT4res(b *[]byte, nfsstat4 uint32) (LOCKT4res, bool) {
	switch nfsstat4 {
	case NFS4ERR_DENIED:
		r, ok := readLOCK4denied(b)
		if !ok {
			return LOCKT4res{}, false
		}
		return LOCKT4res{b: []byte(r), disc: nfsstat4}, true
	case NFS4_OK:
		return LOCKT4res{b: (*b)[:0], disc: nfsstat4}, true
	default:
		return LOCKT4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m LOCKT4res) AsLOCK4denied() LOCK4denied {
	if m.disc != NFS4ERR_DENIED {
		panic("wrong union discriminant")
	}
	return LOCK4denied(m.b)
}

// -------------------------------------------------------
// LOCKU4args — fixed 40 bytes:
//   locktype(4, beu32) + seqid(4, beu32) + stateid4(16) + offset(8, beu64) + length(8, beu64)
// -------------------------------------------------------

type LOCKU4args struct {
	m *[lOCKU4argsSize]byte
}

const lOCKU4argsSize = 40

func readLOCKU4args(b *[]byte) (LOCKU4args, bool) {
	if len(*b) < lOCKU4argsSize {
		return LOCKU4args{}, false
	}
	result := LOCKU4args{m: (*[lOCKU4argsSize]byte)(*b)}
	*b = (*b)[lOCKU4argsSize:]
	return result, true
}

func ReadLOCKU4args(b []byte) (LOCKU4args, bool) {
	return readLOCKU4args(&b)
}

func StartLOCKU4args(buf []byte) ([]byte, LOCKU4args) {
	buf = append(buf, make([]byte, lOCKU4argsSize)...)
	return buf, LOCKU4args{m: (*[lOCKU4argsSize]byte)(buf[len(buf)-lOCKU4argsSize:])}
}

func (m LOCKU4args) Locktype() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m LOCKU4args) Seqid() uint32 {
	return binary.BigEndian.Uint32(m.m[4:8])
}

func (m LOCKU4args) LockStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[8:24])}
}

func (m LOCKU4args) Offset() uint64 {
	return binary.BigEndian.Uint64(m.m[24:32])
}

func (m LOCKU4args) Length() uint64 {
	return binary.BigEndian.Uint64(m.m[32:40])
}

func (m LOCKU4args) SetLocktype(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m LOCKU4args) SetSeqid(v uint32) {
	binary.BigEndian.PutUint32(m.m[4:8], v)
}

func (m LOCKU4args) SetOffset(v uint64) {
	binary.BigEndian.PutUint64(m.m[24:32], v)
}

func (m LOCKU4args) SetLength(v uint64) {
	binary.BigEndian.PutUint64(m.m[32:40], v)
}

// -------------------------------------------------------
// LOCKU4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type LOCKU4res struct {
	b    []byte
	disc uint32
}

func readLOCKU4res(b *[]byte, nfsstat4 uint32) (LOCKU4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readStateid4(b)
		if !ok {
			return LOCKU4res{}, false
		}
		return LOCKU4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return LOCKU4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m LOCKU4res) AsStateid4() Stateid4 {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return Stateid4{m: (*[stateid4Size]byte)(m.b)}
}

// -------------------------------------------------------
// LOOKUP4args — variable: objname
// -------------------------------------------------------

type LOOKUP4args []byte

func readLOOKUP4args(b *[]byte) (LOOKUP4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readComponent4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOOKUP4args(start[:total]), true
}

func ReadLOOKUP4args(b []byte) (LOOKUP4args, bool) {
	return readLOOKUP4args(&b)
}

func (m LOOKUP4args) Objname() Component4 {
	return Component4(m[0:])
}

// LOOKUP4argsWriter writes a LOOKUP4args:
//
//	objname
type LOOKUP4argsWriter struct {
	buf []byte
	off int
}

func StartLOOKUP4args(buf []byte) LOOKUP4argsWriter {
	off := len(buf)
	return LOOKUP4argsWriter{buf: buf, off: off}
}

func (w *LOOKUP4argsWriter) StartObjname() Component4Writer {
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *LOOKUP4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LOOKUP4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LOOKUP4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type LOOKUP4res struct {
	m *[lOOKUP4resSize]byte
}

const lOOKUP4resSize = 4

func readLOOKUP4res(b *[]byte) (LOOKUP4res, bool) {
	if len(*b) < lOOKUP4resSize {
		return LOOKUP4res{}, false
	}
	result := LOOKUP4res{m: (*[lOOKUP4resSize]byte)(*b)}
	*b = (*b)[lOOKUP4resSize:]
	return result, true
}

func ReadLOOKUP4res(b []byte) (LOOKUP4res, bool) {
	return readLOOKUP4res(&b)
}

func StartLOOKUP4res(buf []byte) ([]byte, LOOKUP4res) {
	buf = append(buf, make([]byte, lOOKUP4resSize)...)
	return buf, LOOKUP4res{m: (*[lOOKUP4resSize]byte)(buf[len(buf)-lOOKUP4resSize:])}
}

func (m LOOKUP4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m LOOKUP4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// LOOKUPP4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type LOOKUPP4res struct {
	m *[lOOKUPP4resSize]byte
}

const lOOKUPP4resSize = 4

func readLOOKUPP4res(b *[]byte) (LOOKUPP4res, bool) {
	if len(*b) < lOOKUPP4resSize {
		return LOOKUPP4res{}, false
	}
	result := LOOKUPP4res{m: (*[lOOKUPP4resSize]byte)(*b)}
	*b = (*b)[lOOKUPP4resSize:]
	return result, true
}

func ReadLOOKUPP4res(b []byte) (LOOKUPP4res, bool) {
	return readLOOKUPP4res(&b)
}

func StartLOOKUPP4res(buf []byte) ([]byte, LOOKUPP4res) {
	buf = append(buf, make([]byte, lOOKUPP4resSize)...)
	return buf, LOOKUPP4res{m: (*[lOOKUPP4resSize]byte)(buf[len(buf)-lOOKUPP4resSize:])}
}

func (m LOOKUPP4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m LOOKUPP4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// NVERIFY4args — variable: obj_attributes
// -------------------------------------------------------

type NVERIFY4args []byte

func readNVERIFY4args(b *[]byte) (NVERIFY4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readFattr4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return NVERIFY4args(start[:total]), true
}

func ReadNVERIFY4args(b []byte) (NVERIFY4args, bool) {
	return readNVERIFY4args(&b)
}

func (m NVERIFY4args) ObjAttributes() Fattr4 {
	v, _ := ReadFattr4(m[0:])
	return v
}

// NVERIFY4argsWriter writes a NVERIFY4args:
//
//	obj_attributes
type NVERIFY4argsWriter struct {
	buf []byte
	off int
}

func StartNVERIFY4args(buf []byte) NVERIFY4argsWriter {
	off := len(buf)
	return NVERIFY4argsWriter{buf: buf, off: off}
}

func (w *NVERIFY4argsWriter) StartObjAttributes() Fattr4Writer {
	child := StartFattr4(w.buf)
	w.buf = nil
	return child
}

func (w *NVERIFY4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *NVERIFY4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// NVERIFY4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type NVERIFY4res struct {
	m *[nVERIFY4resSize]byte
}

const nVERIFY4resSize = 4

func readNVERIFY4res(b *[]byte) (NVERIFY4res, bool) {
	if len(*b) < nVERIFY4resSize {
		return NVERIFY4res{}, false
	}
	result := NVERIFY4res{m: (*[nVERIFY4resSize]byte)(*b)}
	*b = (*b)[nVERIFY4resSize:]
	return result, true
}

func ReadNVERIFY4res(b []byte) (NVERIFY4res, bool) {
	return readNVERIFY4res(&b)
}

func StartNVERIFY4res(buf []byte) ([]byte, NVERIFY4res) {
	buf = append(buf, make([]byte, nVERIFY4resSize)...)
	return buf, NVERIFY4res{m: (*[nVERIFY4resSize]byte)(buf[len(buf)-nVERIFY4resSize:])}
}

func (m NVERIFY4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m NVERIFY4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// Createhow4 — union on createmode4 (external discriminant)
// -------------------------------------------------------

type Createhow4 struct {
	b    []byte
	disc uint32
}

func readCreatehow4(b *[]byte, createmode4 uint32) (Createhow4, bool) {
	switch createmode4 {
	case UNCHECKED4:
		r, ok := readFattr4(b)
		if !ok {
			return Createhow4{}, false
		}
		return Createhow4{b: r.data, disc: createmode4}, true
	case GUARDED4:
		r, ok := readFattr4(b)
		if !ok {
			return Createhow4{}, false
		}
		return Createhow4{b: r.data, disc: createmode4}, true
	case EXCLUSIVE4:
		r, ok := readVerifier4(b)
		if !ok {
			return Createhow4{}, false
		}
		return Createhow4{b: r.m[:], disc: createmode4}, true
	default:
		return Createhow4{}, false
	}
}

func (m Createhow4) AsUnchecked4() Fattr4 {
	if m.disc != UNCHECKED4 {
		panic("wrong union discriminant")
	}
	v, _ := ReadFattr4(m.b)
	return v
}

func (m Createhow4) AsGuarded4() Fattr4 {
	if m.disc != GUARDED4 {
		panic("wrong union discriminant")
	}
	v, _ := ReadFattr4(m.b)
	return v
}

func (m Createhow4) AsVerifier4() Verifier4 {
	if m.disc != EXCLUSIVE4 {
		panic("wrong union discriminant")
	}
	return Verifier4{m: (*[verifier4Size]byte)(m.b)}
}

// -------------------------------------------------------
// Createhow4Entry — variable: disc(4) + createhow4 value
// -------------------------------------------------------

type Createhow4Entry []byte

func readCreatehow4Entry(b *[]byte) (Createhow4Entry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readCreatehow4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return Createhow4Entry(start[:total]), true
}

func ReadCreatehow4Entry(b []byte) (Createhow4Entry, bool) {
	return readCreatehow4Entry(&b)
}

func (m Createhow4Entry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Createhow4Entry) Value() Createhow4 {
	return Createhow4{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// Createhow4EntryWriter writes a createhow4_entry:
//
//	disc + value(disc)
type Createhow4EntryWriter struct {
	buf []byte
	off int
}

func StartCreatehow4Entry(buf []byte) Createhow4EntryWriter {
	return Createhow4EntryWriter{buf: buf, off: len(buf)}
}

func (w *Createhow4EntryWriter) SetValue_Unchecked4() Fattr4Writer {
	w.buf = binary.BigEndian.AppendUint32(w.buf, UNCHECKED4)
	child := StartFattr4(w.buf)
	w.buf = nil
	return child
}

func (w *Createhow4EntryWriter) SetValue_Guarded4() Fattr4Writer {
	w.buf = binary.BigEndian.AppendUint32(w.buf, GUARDED4)
	child := StartFattr4(w.buf)
	w.buf = nil
	return child
}

func (w *Createhow4EntryWriter) SetValue_Exclusive4() Verifier4 {
	w.buf = append(w.buf, make([]byte, 4+verifier4Size)...)
	p := (*[4 + verifier4Size]byte)(w.buf[len(w.buf)-4-verifier4Size:])
	binary.BigEndian.PutUint32(p[:4], EXCLUSIVE4)
	return Verifier4{m: (*[verifier4Size]byte)(p[4:])}
}

func (w *Createhow4EntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *Createhow4EntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// Openflag4 — union on opentype4 (external discriminant)
// -------------------------------------------------------

type Openflag4 struct {
	b    []byte
	disc uint32
}

func readOpenflag4(b *[]byte, opentype4 uint32) (Openflag4, bool) {
	switch opentype4 {
	case OPEN4_CREATE:
		r, ok := readCreatehow4Entry(b)
		if !ok {
			return Openflag4{}, false
		}
		return Openflag4{b: []byte(r), disc: opentype4}, true
	default:
		return Openflag4{b: (*b)[:0], disc: opentype4}, true
	}
}

func (m Openflag4) AsCreatehow4Entry() Createhow4Entry {
	if m.disc != OPEN4_CREATE {
		panic("wrong union discriminant")
	}
	return Createhow4Entry(m.b)
}

// -------------------------------------------------------
// NfsModifiedLimit4 — fixed 8 bytes:
//   num_blocks(4, beu32) + bytes_per_block(4, beu32)
// -------------------------------------------------------

type NfsModifiedLimit4 struct {
	m *[nfsModifiedLimit4Size]byte
}

const nfsModifiedLimit4Size = 8

func readNfsModifiedLimit4(b *[]byte) (NfsModifiedLimit4, bool) {
	if len(*b) < nfsModifiedLimit4Size {
		return NfsModifiedLimit4{}, false
	}
	result := NfsModifiedLimit4{m: (*[nfsModifiedLimit4Size]byte)(*b)}
	*b = (*b)[nfsModifiedLimit4Size:]
	return result, true
}

func ReadNfsModifiedLimit4(b []byte) (NfsModifiedLimit4, bool) {
	return readNfsModifiedLimit4(&b)
}

func StartNfsModifiedLimit4(buf []byte) ([]byte, NfsModifiedLimit4) {
	buf = append(buf, make([]byte, nfsModifiedLimit4Size)...)
	return buf, NfsModifiedLimit4{m: (*[nfsModifiedLimit4Size]byte)(buf[len(buf)-nfsModifiedLimit4Size:])}
}

func (m NfsModifiedLimit4) NumBlocks() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m NfsModifiedLimit4) BytesPerBlock() uint32 {
	return binary.BigEndian.Uint32(m.m[4:8])
}

func (m NfsModifiedLimit4) SetNumBlocks(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m NfsModifiedLimit4) SetBytesPerBlock(v uint32) {
	binary.BigEndian.PutUint32(m.m[4:8], v)
}

// -------------------------------------------------------
// NfsSpaceLimit4NFSLIMITSIZE — fixed 8 bytes: value(8, beu64)
// -------------------------------------------------------

type NfsSpaceLimit4NFSLIMITSIZE struct {
	m *[nfsSpaceLimit4NFSLIMITSIZESize]byte
}

const nfsSpaceLimit4NFSLIMITSIZESize = 8

func readNfsSpaceLimit4NFSLIMITSIZE(b *[]byte) (NfsSpaceLimit4NFSLIMITSIZE, bool) {
	if len(*b) < nfsSpaceLimit4NFSLIMITSIZESize {
		return NfsSpaceLimit4NFSLIMITSIZE{}, false
	}
	result := NfsSpaceLimit4NFSLIMITSIZE{m: (*[nfsSpaceLimit4NFSLIMITSIZESize]byte)(*b)}
	*b = (*b)[nfsSpaceLimit4NFSLIMITSIZESize:]
	return result, true
}

func ReadNfsSpaceLimit4NFSLIMITSIZE(b []byte) (NfsSpaceLimit4NFSLIMITSIZE, bool) {
	return readNfsSpaceLimit4NFSLIMITSIZE(&b)
}

func StartNfsSpaceLimit4NFSLIMITSIZE(buf []byte) ([]byte, NfsSpaceLimit4NFSLIMITSIZE) {
	buf = append(buf, make([]byte, nfsSpaceLimit4NFSLIMITSIZESize)...)
	return buf, NfsSpaceLimit4NFSLIMITSIZE{m: (*[nfsSpaceLimit4NFSLIMITSIZESize]byte)(buf[len(buf)-nfsSpaceLimit4NFSLIMITSIZESize:])}
}

func (m NfsSpaceLimit4NFSLIMITSIZE) Value() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m NfsSpaceLimit4NFSLIMITSIZE) SetValue(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

// -------------------------------------------------------
// NfsSpaceLimit4 — union on limit_by4 (external discriminant)
// -------------------------------------------------------

type NfsSpaceLimit4 struct {
	b    []byte
	disc uint32
}

func readNfsSpaceLimit4(b *[]byte, limitBy4 uint32) (NfsSpaceLimit4, bool) {
	switch limitBy4 {
	case NFS_LIMIT_SIZE:
		r, ok := readNfsSpaceLimit4NFSLIMITSIZE(b)
		if !ok {
			return NfsSpaceLimit4{}, false
		}
		return NfsSpaceLimit4{b: r.m[:], disc: limitBy4}, true
	case NFS_LIMIT_BLOCKS:
		r, ok := readNfsModifiedLimit4(b)
		if !ok {
			return NfsSpaceLimit4{}, false
		}
		return NfsSpaceLimit4{b: r.m[:], disc: limitBy4}, true
	default:
		return NfsSpaceLimit4{}, false
	}
}

func (m NfsSpaceLimit4) AsNfsSpaceLimit4NFSLIMITSIZE() NfsSpaceLimit4NFSLIMITSIZE {
	if m.disc != NFS_LIMIT_SIZE {
		panic("wrong union discriminant")
	}
	return NfsSpaceLimit4NFSLIMITSIZE{m: (*[nfsSpaceLimit4NFSLIMITSIZESize]byte)(m.b)}
}

func (m NfsSpaceLimit4) AsNfsModifiedLimit4() NfsModifiedLimit4 {
	if m.disc != NFS_LIMIT_BLOCKS {
		panic("wrong union discriminant")
	}
	return NfsModifiedLimit4{m: (*[nfsModifiedLimit4Size]byte)(m.b)}
}

// -------------------------------------------------------
// OpenClaimDelegateCur4 — variable: delegate_stateid(16) + file
// -------------------------------------------------------

type OpenClaimDelegateCur4 []byte

func readOpenClaimDelegateCur4(b *[]byte) (OpenClaimDelegateCur4, bool) {
	if len(*b) < 16 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[16:]
	if _, ok := readComponent4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return OpenClaimDelegateCur4(start[:total]), true
}

func ReadOpenClaimDelegateCur4(b []byte) (OpenClaimDelegateCur4, bool) {
	return readOpenClaimDelegateCur4(&b)
}

func (m OpenClaimDelegateCur4) DelegateStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m[0 : 0+stateid4Size])}
}

func (m OpenClaimDelegateCur4) File() Component4 {
	return Component4(m[16:])
}

// OpenClaimDelegateCur4Writer writes a open_claim_delegate_cur4:
//
//	delegate_stateid + file
type OpenClaimDelegateCur4Writer struct {
	buf    []byte
	header *[16]byte
}

func StartOpenClaimDelegateCur4(buf []byte) OpenClaimDelegateCur4Writer {
	buf = append(buf, make([]byte, 16)...) // delegate_stateid(16)
	return OpenClaimDelegateCur4Writer{buf: buf, header: (*[16]byte)(buf[len(buf)-16:])}
}

func (w *OpenClaimDelegateCur4Writer) DelegateStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[0:])}
}

func (w *OpenClaimDelegateCur4Writer) StartFile() Component4Writer {
	child := StartComponent4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *OpenClaimDelegateCur4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *OpenClaimDelegateCur4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// OpenClaim4CLAIMPREVIOUS — fixed 4 bytes: value(4, beu32)
// -------------------------------------------------------

type OpenClaim4CLAIMPREVIOUS struct {
	m *[openClaim4CLAIMPREVIOUSSize]byte
}

const openClaim4CLAIMPREVIOUSSize = 4

func readOpenClaim4CLAIMPREVIOUS(b *[]byte) (OpenClaim4CLAIMPREVIOUS, bool) {
	if len(*b) < openClaim4CLAIMPREVIOUSSize {
		return OpenClaim4CLAIMPREVIOUS{}, false
	}
	result := OpenClaim4CLAIMPREVIOUS{m: (*[openClaim4CLAIMPREVIOUSSize]byte)(*b)}
	*b = (*b)[openClaim4CLAIMPREVIOUSSize:]
	return result, true
}

func ReadOpenClaim4CLAIMPREVIOUS(b []byte) (OpenClaim4CLAIMPREVIOUS, bool) {
	return readOpenClaim4CLAIMPREVIOUS(&b)
}

func StartOpenClaim4CLAIMPREVIOUS(buf []byte) ([]byte, OpenClaim4CLAIMPREVIOUS) {
	buf = append(buf, make([]byte, openClaim4CLAIMPREVIOUSSize)...)
	return buf, OpenClaim4CLAIMPREVIOUS{m: (*[openClaim4CLAIMPREVIOUSSize]byte)(buf[len(buf)-openClaim4CLAIMPREVIOUSSize:])}
}

func (m OpenClaim4CLAIMPREVIOUS) Value() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m OpenClaim4CLAIMPREVIOUS) SetValue(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// OpenClaim4 — union on open_claim_type4 (external discriminant)
// -------------------------------------------------------

type OpenClaim4 struct {
	b    []byte
	disc uint32
}

func readOpenClaim4(b *[]byte, openClaimType4 uint32) (OpenClaim4, bool) {
	switch openClaimType4 {
	case CLAIM_NULL:
		r, ok := readComponent4(b)
		if !ok {
			return OpenClaim4{}, false
		}
		return OpenClaim4{b: []byte(r), disc: openClaimType4}, true
	case CLAIM_PREVIOUS:
		r, ok := readOpenClaim4CLAIMPREVIOUS(b)
		if !ok {
			return OpenClaim4{}, false
		}
		return OpenClaim4{b: r.m[:], disc: openClaimType4}, true
	case CLAIM_DELEGATE_CUR:
		r, ok := readOpenClaimDelegateCur4(b)
		if !ok {
			return OpenClaim4{}, false
		}
		return OpenClaim4{b: []byte(r), disc: openClaimType4}, true
	case CLAIM_DELEGATE_PREV:
		r, ok := readComponent4(b)
		if !ok {
			return OpenClaim4{}, false
		}
		return OpenClaim4{b: []byte(r), disc: openClaimType4}, true
	default:
		return OpenClaim4{}, false
	}
}

func (m OpenClaim4) AsNull() Component4 {
	if m.disc != CLAIM_NULL {
		panic("wrong union discriminant")
	}
	return Component4(m.b)
}

func (m OpenClaim4) AsOpenClaim4CLAIMPREVIOUS() OpenClaim4CLAIMPREVIOUS {
	if m.disc != CLAIM_PREVIOUS {
		panic("wrong union discriminant")
	}
	return OpenClaim4CLAIMPREVIOUS{m: (*[openClaim4CLAIMPREVIOUSSize]byte)(m.b)}
}

func (m OpenClaim4) AsOpenClaimDelegateCur4() OpenClaimDelegateCur4 {
	if m.disc != CLAIM_DELEGATE_CUR {
		panic("wrong union discriminant")
	}
	return OpenClaimDelegateCur4(m.b)
}

func (m OpenClaim4) AsDelegatePrev() Component4 {
	if m.disc != CLAIM_DELEGATE_PREV {
		panic("wrong union discriminant")
	}
	return Component4(m.b)
}

// -------------------------------------------------------
// OPEN4args — variable:
//   seqid(4) + share_access(4) + share_deny(4) + owner + openhow_type(4) + openflag4 openhow + claim_type(4) + open_claim4 claim
// -------------------------------------------------------

type OPEN4args struct {
	data []byte
	off1 int // byte offset within data where openhow_type starts
	off2 int // byte offset within data where claim_type starts
}

func readOPEN4args(b *[]byte) (OPEN4args, bool) {
	if len(*b) < 12 {
		return OPEN4args{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[12:]
	if _, ok := readOpenOwner4(b); !ok {
		return OPEN4args{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return OPEN4args{}, false
	}
	c_openhow_type := binary.BigEndian.Uint32((*b)[:4])
	*b = (*b)[4:]
	if _, ok := readOpenflag4(b, c_openhow_type); !ok {
		return OPEN4args{}, false
	}
	off2 := startLen - len(*b)
	if len(*b) < 4 {
		return OPEN4args{}, false
	}
	c_claim_type := binary.BigEndian.Uint32((*b)[:4])
	*b = (*b)[4:]
	if _, ok := readOpenClaim4(b, c_claim_type); !ok {
		return OPEN4args{}, false
	}
	total := startLen - len(*b)
	return OPEN4args{data: start[:total], off1: off1, off2: off2}, true
}

func ReadOPEN4args(b []byte) (OPEN4args, bool) {
	return readOPEN4args(&b)
}

func (m OPEN4args) Seqid() uint32 {
	return binary.BigEndian.Uint32(m.data[0:4])
}

func (m OPEN4args) ShareAccess() uint32 {
	return binary.BigEndian.Uint32(m.data[4:8])
}

func (m OPEN4args) ShareDeny() uint32 {
	return binary.BigEndian.Uint32(m.data[8:12])
}

func (m OPEN4args) Owner() OpenOwner4 {
	return OpenOwner4(m.data[12:m.off1])
}

func (m OPEN4args) OpenhowType() uint32 {
	o := m.off1
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m OPEN4args) Openhow() Openflag4 {
	return Openflag4{b: m.data[m.off1+4 : m.off2], disc: binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4])}
}

func (m OPEN4args) ClaimType() uint32 {
	o := m.off2
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m OPEN4args) Claim() OpenClaim4 {
	return OpenClaim4{b: m.data[m.off2+4:], disc: binary.BigEndian.Uint32(m.data[m.off2 : m.off2+4])}
}

// OPEN4argsWriter writes a OPEN4args:
//
//	seqid + share_access + share_deny + owner + openhow_type + openhow(openhow_type) + claim_type + claim(claim_type)
type OPEN4argsWriter struct {
	buf    []byte
	header *[12]byte
	phase  uint8
}

func StartOPEN4args(buf []byte) OPEN4argsWriter {
	buf = append(buf, make([]byte, 12)...) // seqid(4) + share_access(4) + share_deny(4)
	return OPEN4argsWriter{buf: buf, header: (*[12]byte)(buf[len(buf)-12:])}
}

func (w *OPEN4argsWriter) SetSeqid(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *OPEN4argsWriter) SetShareAccess(v uint32) {
	binary.BigEndian.PutUint32(w.header[4:8], v)
}

func (w *OPEN4argsWriter) SetShareDeny(v uint32) {
	binary.BigEndian.PutUint32(w.header[8:12], v)
}

func (w *OPEN4argsWriter) SetOpenhow_Create() Createhow4EntryWriter {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = binary.BigEndian.AppendUint32(w.buf, OPEN4_CREATE)
	child := StartCreatehow4Entry(w.buf)
	w.buf = nil
	return child
}

func (w *OPEN4argsWriter) SetOpenhow_Default(opentype4 uint32) {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = binary.BigEndian.AppendUint32(w.buf, opentype4)
}

func (w *OPEN4argsWriter) SetClaim_Null() Component4Writer {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = binary.BigEndian.AppendUint32(w.buf, CLAIM_NULL)
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *OPEN4argsWriter) SetClaim_Previous() OpenClaim4CLAIMPREVIOUS {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = append(w.buf, make([]byte, 4+openClaim4CLAIMPREVIOUSSize)...)
	p := (*[4 + openClaim4CLAIMPREVIOUSSize]byte)(w.buf[len(w.buf)-4-openClaim4CLAIMPREVIOUSSize:])
	binary.BigEndian.PutUint32(p[:4], CLAIM_PREVIOUS)
	return OpenClaim4CLAIMPREVIOUS{m: (*[openClaim4CLAIMPREVIOUSSize]byte)(p[4:])}
}

func (w *OPEN4argsWriter) SetClaim_DelegateCur() OpenClaimDelegateCur4Writer {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = append(w.buf, make([]byte, 4+16)...)
	off := len(w.buf) - 4 - 16
	binary.BigEndian.PutUint32((*[4 + 16]byte)(w.buf[off:])[:4], CLAIM_DELEGATE_CUR)
	buf := w.buf
	w.buf = nil
	return OpenClaimDelegateCur4Writer{buf: buf, header: (*[16]byte)(buf[off+4:])}
}

func (w *OPEN4argsWriter) SetClaim_DelegatePrev() Component4Writer {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = binary.BigEndian.AppendUint32(w.buf, CLAIM_DELEGATE_PREV)
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *OPEN4argsWriter) StartOwner() OpenOwner4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartOpenOwner4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *OPEN4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *OPEN4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// OpenReadDelegation4 — variable: stateid(16) + recall(4) + permissions
// -------------------------------------------------------

type OpenReadDelegation4 []byte

func readOpenReadDelegation4(b *[]byte) (OpenReadDelegation4, bool) {
	if len(*b) < 20 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[20:]
	if _, ok := readNfsace4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return OpenReadDelegation4(start[:total]), true
}

func ReadOpenReadDelegation4(b []byte) (OpenReadDelegation4, bool) {
	return readOpenReadDelegation4(&b)
}

func (m OpenReadDelegation4) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m[0 : 0+stateid4Size])}
}

func (m OpenReadDelegation4) Recall() uint32 {
	return binary.BigEndian.Uint32(m[16:20])
}

func (m OpenReadDelegation4) Permissions() Nfsace4 {
	return Nfsace4(m[20:])
}

// OpenReadDelegation4Writer writes a open_read_delegation4:
//
//	stateid + recall + permissions
type OpenReadDelegation4Writer struct {
	buf    []byte
	header *[20]byte
}

func StartOpenReadDelegation4(buf []byte) OpenReadDelegation4Writer {
	buf = append(buf, make([]byte, 20)...) // stateid(16) + recall(4)
	return OpenReadDelegation4Writer{buf: buf, header: (*[20]byte)(buf[len(buf)-20:])}
}

func (w *OpenReadDelegation4Writer) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[0:])}
}

func (w *OpenReadDelegation4Writer) SetRecall(v uint32) {
	binary.BigEndian.PutUint32(w.header[16:20], v)
}

func (w *OpenReadDelegation4Writer) StartPermissions() Nfsace4Writer {
	child := StartNfsace4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *OpenReadDelegation4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *OpenReadDelegation4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// OpenWriteDelegation4 — variable:
//   stateid(16) + recall(4) + space_limit_type(4) + nfs_space_limit4 space_limit + permissions
// -------------------------------------------------------

type OpenWriteDelegation4 struct {
	data []byte
	off1 int // byte offset within data where permissions starts
}

func readOpenWriteDelegation4(b *[]byte) (OpenWriteDelegation4, bool) {
	if len(*b) < 24 {
		return OpenWriteDelegation4{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[24:]
	if _, ok := readNfsSpaceLimit4(b, binary.BigEndian.Uint32(start[20:24])); !ok {
		return OpenWriteDelegation4{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readNfsace4(b); !ok {
		return OpenWriteDelegation4{}, false
	}
	total := startLen - len(*b)
	return OpenWriteDelegation4{data: start[:total], off1: off1}, true
}

func ReadOpenWriteDelegation4(b []byte) (OpenWriteDelegation4, bool) {
	return readOpenWriteDelegation4(&b)
}

func (m OpenWriteDelegation4) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.data[0 : 0+stateid4Size])}
}

func (m OpenWriteDelegation4) Recall() uint32 {
	return binary.BigEndian.Uint32(m.data[16:20])
}

func (m OpenWriteDelegation4) SpaceLimitType() uint32 {
	return binary.BigEndian.Uint32(m.data[20:24])
}

func (m OpenWriteDelegation4) SpaceLimit() NfsSpaceLimit4 {
	return NfsSpaceLimit4{b: m.data[24:m.off1], disc: binary.BigEndian.Uint32(m.data[20:24])}
}

func (m OpenWriteDelegation4) Permissions() Nfsace4 {
	return Nfsace4(m.data[m.off1:])
}

// OpenWriteDelegation4Writer writes a open_write_delegation4:
//
//	stateid + recall + space_limit_type + space_limit(space_limit_type) + permissions
type OpenWriteDelegation4Writer struct {
	buf    []byte
	header *[24]byte
}

func StartOpenWriteDelegation4(buf []byte) OpenWriteDelegation4Writer {
	buf = append(buf, make([]byte, 24)...) // stateid(16) + recall(4) + space_limit_type(4)
	return OpenWriteDelegation4Writer{buf: buf, header: (*[24]byte)(buf[len(buf)-24:])}
}

func (w *OpenWriteDelegation4Writer) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[0:])}
}

func (w *OpenWriteDelegation4Writer) SetRecall(v uint32) {
	binary.BigEndian.PutUint32(w.header[16:20], v)
}

func (w *OpenWriteDelegation4Writer) SetSpaceLimitType(v uint32) {
	binary.BigEndian.PutUint32(w.header[20:24], v)
}

func (w *OpenWriteDelegation4Writer) SetSpaceLimit_Size() NfsSpaceLimit4NFSLIMITSIZE {
	binary.BigEndian.PutUint32(w.header[20:24], NFS_LIMIT_SIZE)
	w.buf = append(w.buf, make([]byte, nfsSpaceLimit4NFSLIMITSIZESize)...)
	return NfsSpaceLimit4NFSLIMITSIZE{m: (*[nfsSpaceLimit4NFSLIMITSIZESize]byte)(w.buf[len(w.buf)-nfsSpaceLimit4NFSLIMITSIZESize:])}
}

func (w *OpenWriteDelegation4Writer) SetSpaceLimit_Blocks() NfsModifiedLimit4 {
	binary.BigEndian.PutUint32(w.header[20:24], NFS_LIMIT_BLOCKS)
	w.buf = append(w.buf, make([]byte, nfsModifiedLimit4Size)...)
	return NfsModifiedLimit4{m: (*[nfsModifiedLimit4Size]byte)(w.buf[len(w.buf)-nfsModifiedLimit4Size:])}
}

func (w *OpenWriteDelegation4Writer) StartPermissions() Nfsace4Writer {
	child := StartNfsace4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *OpenWriteDelegation4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *OpenWriteDelegation4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// OpenDelegation4 — union on open_delegation_type4 (external discriminant)
// -------------------------------------------------------

type OpenDelegation4 struct {
	b    []byte
	disc uint32
}

func readOpenDelegation4(b *[]byte, openDelegationType4 uint32) (OpenDelegation4, bool) {
	switch openDelegationType4 {
	case OPEN_DELEGATE_NONE:
		return OpenDelegation4{b: (*b)[:0], disc: openDelegationType4}, true
	case OPEN_DELEGATE_READ:
		r, ok := readOpenReadDelegation4(b)
		if !ok {
			return OpenDelegation4{}, false
		}
		return OpenDelegation4{b: []byte(r), disc: openDelegationType4}, true
	case OPEN_DELEGATE_WRITE:
		r, ok := readOpenWriteDelegation4(b)
		if !ok {
			return OpenDelegation4{}, false
		}
		return OpenDelegation4{b: r.data, disc: openDelegationType4}, true
	default:
		return OpenDelegation4{}, false
	}
}

func (m OpenDelegation4) AsOpenReadDelegation4() OpenReadDelegation4 {
	if m.disc != OPEN_DELEGATE_READ {
		panic("wrong union discriminant")
	}
	return OpenReadDelegation4(m.b)
}

func (m OpenDelegation4) AsOpenWriteDelegation4() OpenWriteDelegation4 {
	if m.disc != OPEN_DELEGATE_WRITE {
		panic("wrong union discriminant")
	}
	v, _ := ReadOpenWriteDelegation4(m.b)
	return v
}

// -------------------------------------------------------
// OPEN4resok — variable:
//   stateid(16) + cinfo(20) + rflags(4) + attrset + delegation_type(4) + open_delegation4 delegation
// -------------------------------------------------------

type OPEN4resok struct {
	data []byte
	off1 int // byte offset within data where delegation_type starts
}

func readOPEN4resok(b *[]byte) (OPEN4resok, bool) {
	if len(*b) < 40 {
		return OPEN4resok{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[40:]
	if _, ok := readBitmap4(b); !ok {
		return OPEN4resok{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return OPEN4resok{}, false
	}
	c_delegation_type := binary.BigEndian.Uint32((*b)[:4])
	*b = (*b)[4:]
	if _, ok := readOpenDelegation4(b, c_delegation_type); !ok {
		return OPEN4resok{}, false
	}
	total := startLen - len(*b)
	return OPEN4resok{data: start[:total], off1: off1}, true
}

func ReadOPEN4resok(b []byte) (OPEN4resok, bool) {
	return readOPEN4resok(&b)
}

func (m OPEN4resok) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.data[0 : 0+stateid4Size])}
}

func (m OPEN4resok) Cinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(m.data[16 : 16+changeInfo4Size])}
}

func (m OPEN4resok) Rflags() uint32 {
	return binary.BigEndian.Uint32(m.data[36:40])
}

func (m OPEN4resok) Attrset() Bitmap4 {
	return Bitmap4(m.data[40:m.off1])
}

func (m OPEN4resok) DelegationType() uint32 {
	o := m.off1
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m OPEN4resok) Delegation() OpenDelegation4 {
	return OpenDelegation4{b: m.data[m.off1+4:], disc: binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4])}
}

// OPEN4resokWriter writes a OPEN4resok:
//
//	stateid + cinfo + rflags + attrset + delegation_type + delegation(delegation_type)
type OPEN4resokWriter struct {
	buf    []byte
	header *[40]byte
	phase  uint8
}

func StartOPEN4resok(buf []byte) OPEN4resokWriter {
	buf = append(buf, make([]byte, 40)...) // stateid(16) + cinfo(20) + rflags(4)
	return OPEN4resokWriter{buf: buf, header: (*[40]byte)(buf[len(buf)-40:])}
}

func (w *OPEN4resokWriter) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[0:])}
}

func (w *OPEN4resokWriter) Cinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(w.header[16:])}
}

func (w *OPEN4resokWriter) SetRflags(v uint32) {
	binary.BigEndian.PutUint32(w.header[36:40], v)
}

func (w *OPEN4resokWriter) SetDelegation_None() {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = binary.BigEndian.AppendUint32(w.buf, OPEN_DELEGATE_NONE)
}

func (w *OPEN4resokWriter) SetDelegation_Read() OpenReadDelegation4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], OPEN_DELEGATE_READ)
	buf := w.buf
	w.buf = nil
	return OpenReadDelegation4Writer{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *OPEN4resokWriter) SetDelegation_Write() OpenWriteDelegation4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = append(w.buf, make([]byte, 4+24)...)
	off := len(w.buf) - 4 - 24
	binary.BigEndian.PutUint32((*[4 + 24]byte)(w.buf[off:])[:4], OPEN_DELEGATE_WRITE)
	buf := w.buf
	w.buf = nil
	return OpenWriteDelegation4Writer{buf: buf, header: (*[24]byte)(buf[off+4:])}
}

func (w *OPEN4resokWriter) StartAttrset() Bitmap4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartBitmap4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *OPEN4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *OPEN4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// OPEN4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type OPEN4res struct {
	b    []byte
	disc uint32
}

func readOPEN4res(b *[]byte, nfsstat4 uint32) (OPEN4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readOPEN4resok(b)
		if !ok {
			return OPEN4res{}, false
		}
		return OPEN4res{b: r.data, disc: nfsstat4}, true
	default:
		return OPEN4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m OPEN4res) AsOPEN4resok() OPEN4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	v, _ := ReadOPEN4resok(m.b)
	return v
}

// -------------------------------------------------------
// OPENATTR4args — fixed 4 bytes: createdir(4, beu32)
// -------------------------------------------------------

type OPENATTR4args struct {
	m *[oPENATTR4argsSize]byte
}

const oPENATTR4argsSize = 4

func readOPENATTR4args(b *[]byte) (OPENATTR4args, bool) {
	if len(*b) < oPENATTR4argsSize {
		return OPENATTR4args{}, false
	}
	result := OPENATTR4args{m: (*[oPENATTR4argsSize]byte)(*b)}
	*b = (*b)[oPENATTR4argsSize:]
	return result, true
}

func ReadOPENATTR4args(b []byte) (OPENATTR4args, bool) {
	return readOPENATTR4args(&b)
}

func StartOPENATTR4args(buf []byte) ([]byte, OPENATTR4args) {
	buf = append(buf, make([]byte, oPENATTR4argsSize)...)
	return buf, OPENATTR4args{m: (*[oPENATTR4argsSize]byte)(buf[len(buf)-oPENATTR4argsSize:])}
}

func (m OPENATTR4args) Createdir() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m OPENATTR4args) SetCreatedir(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// OPENATTR4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type OPENATTR4res struct {
	m *[oPENATTR4resSize]byte
}

const oPENATTR4resSize = 4

func readOPENATTR4res(b *[]byte) (OPENATTR4res, bool) {
	if len(*b) < oPENATTR4resSize {
		return OPENATTR4res{}, false
	}
	result := OPENATTR4res{m: (*[oPENATTR4resSize]byte)(*b)}
	*b = (*b)[oPENATTR4resSize:]
	return result, true
}

func ReadOPENATTR4res(b []byte) (OPENATTR4res, bool) {
	return readOPENATTR4res(&b)
}

func StartOPENATTR4res(buf []byte) ([]byte, OPENATTR4res) {
	buf = append(buf, make([]byte, oPENATTR4resSize)...)
	return buf, OPENATTR4res{m: (*[oPENATTR4resSize]byte)(buf[len(buf)-oPENATTR4resSize:])}
}

func (m OPENATTR4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m OPENATTR4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// OPENCONFIRM4args — fixed 20 bytes: stateid4(16) + seqid(4, beu32)
// -------------------------------------------------------

type OPENCONFIRM4args struct {
	m *[oPENCONFIRM4argsSize]byte
}

const oPENCONFIRM4argsSize = 20

func readOPENCONFIRM4args(b *[]byte) (OPENCONFIRM4args, bool) {
	if len(*b) < oPENCONFIRM4argsSize {
		return OPENCONFIRM4args{}, false
	}
	result := OPENCONFIRM4args{m: (*[oPENCONFIRM4argsSize]byte)(*b)}
	*b = (*b)[oPENCONFIRM4argsSize:]
	return result, true
}

func ReadOPENCONFIRM4args(b []byte) (OPENCONFIRM4args, bool) {
	return readOPENCONFIRM4args(&b)
}

func StartOPENCONFIRM4args(buf []byte) ([]byte, OPENCONFIRM4args) {
	buf = append(buf, make([]byte, oPENCONFIRM4argsSize)...)
	return buf, OPENCONFIRM4args{m: (*[oPENCONFIRM4argsSize]byte)(buf[len(buf)-oPENCONFIRM4argsSize:])}
}

func (m OPENCONFIRM4args) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

func (m OPENCONFIRM4args) Seqid() uint32 {
	return binary.BigEndian.Uint32(m.m[16:20])
}

func (m OPENCONFIRM4args) SetSeqid(v uint32) {
	binary.BigEndian.PutUint32(m.m[16:20], v)
}

// -------------------------------------------------------
// OPENCONFIRM4resok — fixed 16 bytes: stateid4(16)
// -------------------------------------------------------

type OPENCONFIRM4resok struct {
	m *[oPENCONFIRM4resokSize]byte
}

const oPENCONFIRM4resokSize = 16

func readOPENCONFIRM4resok(b *[]byte) (OPENCONFIRM4resok, bool) {
	if len(*b) < oPENCONFIRM4resokSize {
		return OPENCONFIRM4resok{}, false
	}
	result := OPENCONFIRM4resok{m: (*[oPENCONFIRM4resokSize]byte)(*b)}
	*b = (*b)[oPENCONFIRM4resokSize:]
	return result, true
}

func ReadOPENCONFIRM4resok(b []byte) (OPENCONFIRM4resok, bool) {
	return readOPENCONFIRM4resok(&b)
}

func StartOPENCONFIRM4resok(buf []byte) ([]byte, OPENCONFIRM4resok) {
	buf = append(buf, make([]byte, oPENCONFIRM4resokSize)...)
	return buf, OPENCONFIRM4resok{m: (*[oPENCONFIRM4resokSize]byte)(buf[len(buf)-oPENCONFIRM4resokSize:])}
}

func (m OPENCONFIRM4resok) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

// -------------------------------------------------------
// OPENCONFIRM4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type OPENCONFIRM4res struct {
	b    []byte
	disc uint32
}

func readOPENCONFIRM4res(b *[]byte, nfsstat4 uint32) (OPENCONFIRM4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readOPENCONFIRM4resok(b)
		if !ok {
			return OPENCONFIRM4res{}, false
		}
		return OPENCONFIRM4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return OPENCONFIRM4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m OPENCONFIRM4res) AsOPENCONFIRM4resok() OPENCONFIRM4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return OPENCONFIRM4resok{m: (*[oPENCONFIRM4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// OPENDOWNGRADE4args — fixed 28 bytes:
//   stateid4(16) + seqid(4, beu32) + share_access(4, beu32) + share_deny(4, beu32)
// -------------------------------------------------------

type OPENDOWNGRADE4args struct {
	m *[oPENDOWNGRADE4argsSize]byte
}

const oPENDOWNGRADE4argsSize = 28

func readOPENDOWNGRADE4args(b *[]byte) (OPENDOWNGRADE4args, bool) {
	if len(*b) < oPENDOWNGRADE4argsSize {
		return OPENDOWNGRADE4args{}, false
	}
	result := OPENDOWNGRADE4args{m: (*[oPENDOWNGRADE4argsSize]byte)(*b)}
	*b = (*b)[oPENDOWNGRADE4argsSize:]
	return result, true
}

func ReadOPENDOWNGRADE4args(b []byte) (OPENDOWNGRADE4args, bool) {
	return readOPENDOWNGRADE4args(&b)
}

func StartOPENDOWNGRADE4args(buf []byte) ([]byte, OPENDOWNGRADE4args) {
	buf = append(buf, make([]byte, oPENDOWNGRADE4argsSize)...)
	return buf, OPENDOWNGRADE4args{m: (*[oPENDOWNGRADE4argsSize]byte)(buf[len(buf)-oPENDOWNGRADE4argsSize:])}
}

func (m OPENDOWNGRADE4args) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

func (m OPENDOWNGRADE4args) Seqid() uint32 {
	return binary.BigEndian.Uint32(m.m[16:20])
}

func (m OPENDOWNGRADE4args) ShareAccess() uint32 {
	return binary.BigEndian.Uint32(m.m[20:24])
}

func (m OPENDOWNGRADE4args) ShareDeny() uint32 {
	return binary.BigEndian.Uint32(m.m[24:28])
}

func (m OPENDOWNGRADE4args) SetSeqid(v uint32) {
	binary.BigEndian.PutUint32(m.m[16:20], v)
}

func (m OPENDOWNGRADE4args) SetShareAccess(v uint32) {
	binary.BigEndian.PutUint32(m.m[20:24], v)
}

func (m OPENDOWNGRADE4args) SetShareDeny(v uint32) {
	binary.BigEndian.PutUint32(m.m[24:28], v)
}

// -------------------------------------------------------
// OPENDOWNGRADE4resok — fixed 16 bytes: stateid4(16)
// -------------------------------------------------------

type OPENDOWNGRADE4resok struct {
	m *[oPENDOWNGRADE4resokSize]byte
}

const oPENDOWNGRADE4resokSize = 16

func readOPENDOWNGRADE4resok(b *[]byte) (OPENDOWNGRADE4resok, bool) {
	if len(*b) < oPENDOWNGRADE4resokSize {
		return OPENDOWNGRADE4resok{}, false
	}
	result := OPENDOWNGRADE4resok{m: (*[oPENDOWNGRADE4resokSize]byte)(*b)}
	*b = (*b)[oPENDOWNGRADE4resokSize:]
	return result, true
}

func ReadOPENDOWNGRADE4resok(b []byte) (OPENDOWNGRADE4resok, bool) {
	return readOPENDOWNGRADE4resok(&b)
}

func StartOPENDOWNGRADE4resok(buf []byte) ([]byte, OPENDOWNGRADE4resok) {
	buf = append(buf, make([]byte, oPENDOWNGRADE4resokSize)...)
	return buf, OPENDOWNGRADE4resok{m: (*[oPENDOWNGRADE4resokSize]byte)(buf[len(buf)-oPENDOWNGRADE4resokSize:])}
}

func (m OPENDOWNGRADE4resok) OpenStateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

// -------------------------------------------------------
// OPENDOWNGRADE4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type OPENDOWNGRADE4res struct {
	b    []byte
	disc uint32
}

func readOPENDOWNGRADE4res(b *[]byte, nfsstat4 uint32) (OPENDOWNGRADE4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readOPENDOWNGRADE4resok(b)
		if !ok {
			return OPENDOWNGRADE4res{}, false
		}
		return OPENDOWNGRADE4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return OPENDOWNGRADE4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m OPENDOWNGRADE4res) AsOPENDOWNGRADE4resok() OPENDOWNGRADE4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return OPENDOWNGRADE4resok{m: (*[oPENDOWNGRADE4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// PUTFH4args — variable: object
// -------------------------------------------------------

type PUTFH4args []byte

func readPUTFH4args(b *[]byte) (PUTFH4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readNfsFh4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return PUTFH4args(start[:total]), true
}

func ReadPUTFH4args(b []byte) (PUTFH4args, bool) {
	return readPUTFH4args(&b)
}

func (m PUTFH4args) Object() NfsFh4 {
	return NfsFh4(m[0:])
}

// PUTFH4argsWriter writes a PUTFH4args:
//
//	object
type PUTFH4argsWriter struct {
	buf []byte
	off int
}

func StartPUTFH4args(buf []byte) PUTFH4argsWriter {
	off := len(buf)
	return PUTFH4argsWriter{buf: buf, off: off}
}

func (w *PUTFH4argsWriter) StartObject() NfsFh4Writer {
	child := StartNfsFh4(w.buf)
	w.buf = nil
	return child
}

func (w *PUTFH4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *PUTFH4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// PUTFH4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type PUTFH4res struct {
	m *[pUTFH4resSize]byte
}

const pUTFH4resSize = 4

func readPUTFH4res(b *[]byte) (PUTFH4res, bool) {
	if len(*b) < pUTFH4resSize {
		return PUTFH4res{}, false
	}
	result := PUTFH4res{m: (*[pUTFH4resSize]byte)(*b)}
	*b = (*b)[pUTFH4resSize:]
	return result, true
}

func ReadPUTFH4res(b []byte) (PUTFH4res, bool) {
	return readPUTFH4res(&b)
}

func StartPUTFH4res(buf []byte) ([]byte, PUTFH4res) {
	buf = append(buf, make([]byte, pUTFH4resSize)...)
	return buf, PUTFH4res{m: (*[pUTFH4resSize]byte)(buf[len(buf)-pUTFH4resSize:])}
}

func (m PUTFH4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m PUTFH4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// PUTPUBFH4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type PUTPUBFH4res struct {
	m *[pUTPUBFH4resSize]byte
}

const pUTPUBFH4resSize = 4

func readPUTPUBFH4res(b *[]byte) (PUTPUBFH4res, bool) {
	if len(*b) < pUTPUBFH4resSize {
		return PUTPUBFH4res{}, false
	}
	result := PUTPUBFH4res{m: (*[pUTPUBFH4resSize]byte)(*b)}
	*b = (*b)[pUTPUBFH4resSize:]
	return result, true
}

func ReadPUTPUBFH4res(b []byte) (PUTPUBFH4res, bool) {
	return readPUTPUBFH4res(&b)
}

func StartPUTPUBFH4res(buf []byte) ([]byte, PUTPUBFH4res) {
	buf = append(buf, make([]byte, pUTPUBFH4resSize)...)
	return buf, PUTPUBFH4res{m: (*[pUTPUBFH4resSize]byte)(buf[len(buf)-pUTPUBFH4resSize:])}
}

func (m PUTPUBFH4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m PUTPUBFH4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// PUTROOTFH4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type PUTROOTFH4res struct {
	m *[pUTROOTFH4resSize]byte
}

const pUTROOTFH4resSize = 4

func readPUTROOTFH4res(b *[]byte) (PUTROOTFH4res, bool) {
	if len(*b) < pUTROOTFH4resSize {
		return PUTROOTFH4res{}, false
	}
	result := PUTROOTFH4res{m: (*[pUTROOTFH4resSize]byte)(*b)}
	*b = (*b)[pUTROOTFH4resSize:]
	return result, true
}

func ReadPUTROOTFH4res(b []byte) (PUTROOTFH4res, bool) {
	return readPUTROOTFH4res(&b)
}

func StartPUTROOTFH4res(buf []byte) ([]byte, PUTROOTFH4res) {
	buf = append(buf, make([]byte, pUTROOTFH4resSize)...)
	return buf, PUTROOTFH4res{m: (*[pUTROOTFH4resSize]byte)(buf[len(buf)-pUTROOTFH4resSize:])}
}

func (m PUTROOTFH4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m PUTROOTFH4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// READ4args — fixed 28 bytes:
//   stateid4(16) + offset(8, beu64) + count(4, beu32)
// -------------------------------------------------------

type READ4args struct {
	m *[rEAD4argsSize]byte
}

const rEAD4argsSize = 28

func readREAD4args(b *[]byte) (READ4args, bool) {
	if len(*b) < rEAD4argsSize {
		return READ4args{}, false
	}
	result := READ4args{m: (*[rEAD4argsSize]byte)(*b)}
	*b = (*b)[rEAD4argsSize:]
	return result, true
}

func ReadREAD4args(b []byte) (READ4args, bool) {
	return readREAD4args(&b)
}

func StartREAD4args(buf []byte) ([]byte, READ4args) {
	buf = append(buf, make([]byte, rEAD4argsSize)...)
	return buf, READ4args{m: (*[rEAD4argsSize]byte)(buf[len(buf)-rEAD4argsSize:])}
}

func (m READ4args) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m.m[0:16])}
}

func (m READ4args) Offset() uint64 {
	return binary.BigEndian.Uint64(m.m[16:24])
}

func (m READ4args) Count() uint32 {
	return binary.BigEndian.Uint32(m.m[24:28])
}

func (m READ4args) SetOffset(v uint64) {
	binary.BigEndian.PutUint64(m.m[16:24], v)
}

func (m READ4args) SetCount(v uint32) {
	binary.BigEndian.PutUint32(m.m[24:28], v)
}

// -------------------------------------------------------
// READ4resok — variable: eof(4) + data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type READ4resok []byte

func readREAD4resok(b *[]byte) (READ4resok, bool) {
	if len(*b) < 8 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[4:8]))
	padded := (n + 3) &^ 3
	total := 8 + padded
	if len(*b) < total {
		return nil, false
	}
	result := READ4resok((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadREAD4resok(b []byte) (READ4resok, bool) {
	if len(b) < 8 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[4:8]))
	padded := (count + 3) &^ 3
	total := 8 + padded
	if len(b) < total {
		return nil, false
	}
	return READ4resok(b[:total]), true
}

func (m READ4resok) Eof() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m READ4resok) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[4:8])
}

func (m READ4resok) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[4:8]))
	return m[8 : 8+n]
}

// READ4resokWriter writes a READ4resok:
//
//	eof + data_len + u8 data[data_len] + align(4)
type READ4resokWriter struct {
	buf     []byte
	off     int
	dataLen uint32
}

func StartREAD4resok(buf []byte) READ4resokWriter {
	off := len(buf)
	buf = append(buf, make([]byte, 8)...) // eof(4) + data_len(4)
	return READ4resokWriter{buf: buf, off: off}
}

func (w READ4resokWriter) SetEof(v uint32) READ4resokWriter {
	binary.BigEndian.PutUint32((*[8]byte)(w.buf[w.off:])[0:4], v)
	return w
}

func (w READ4resokWriter) SetData(data []byte) READ4resokWriter {
	n := len(data)
	padded := (n + 3) &^ 3
	w.buf = append(w.buf, make([]byte, padded)...)
	copy(w.buf[len(w.buf)-padded:], data)
	w.dataLen = uint32(n)
	return w
}

func (w READ4resokWriter) Finish() []byte {
	binary.BigEndian.PutUint32((*[8]byte)(w.buf[w.off:])[4:8], w.dataLen)
	return w.buf
}

// -------------------------------------------------------
// READ4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type READ4res struct {
	b    []byte
	disc uint32
}

func readREAD4res(b *[]byte, nfsstat4 uint32) (READ4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readREAD4resok(b)
		if !ok {
			return READ4res{}, false
		}
		return READ4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return READ4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m READ4res) AsREAD4resok() READ4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return READ4resok(m.b)
}

// -------------------------------------------------------
// READDIR4args — variable:
//   cookie(8) + cookieverf(8) + dircount(4) + maxcount(4) + attr_request
// -------------------------------------------------------

type READDIR4args []byte

func readREADDIR4args(b *[]byte) (READDIR4args, bool) {
	if len(*b) < 24 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[24:]
	if _, ok := readBitmap4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return READDIR4args(start[:total]), true
}

func ReadREADDIR4args(b []byte) (READDIR4args, bool) {
	return readREADDIR4args(&b)
}

func (m READDIR4args) Cookie() uint64 {
	return binary.BigEndian.Uint64(m[0:8])
}

func (m READDIR4args) Cookieverf() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m[8 : 8+verifier4Size])}
}

func (m READDIR4args) Dircount() uint32 {
	return binary.BigEndian.Uint32(m[16:20])
}

func (m READDIR4args) Maxcount() uint32 {
	return binary.BigEndian.Uint32(m[20:24])
}

func (m READDIR4args) AttrRequest() Bitmap4 {
	return Bitmap4(m[24:])
}

// READDIR4argsWriter writes a READDIR4args:
//
//	cookie + cookieverf + dircount + maxcount + attr_request
type READDIR4argsWriter struct {
	buf    []byte
	header *[24]byte
}

func StartREADDIR4args(buf []byte) READDIR4argsWriter {
	buf = append(buf, make([]byte, 24)...) // cookie(8) + cookieverf(8) + dircount(4) + maxcount(4)
	return READDIR4argsWriter{buf: buf, header: (*[24]byte)(buf[len(buf)-24:])}
}

func (w *READDIR4argsWriter) SetCookie(v uint64) {
	binary.BigEndian.PutUint64(w.header[0:8], v)
}

func (w *READDIR4argsWriter) Cookieverf() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(w.header[8:])}
}

func (w *READDIR4argsWriter) SetDircount(v uint32) {
	binary.BigEndian.PutUint32(w.header[16:20], v)
}

func (w *READDIR4argsWriter) SetMaxcount(v uint32) {
	binary.BigEndian.PutUint32(w.header[20:24], v)
}

func (w *READDIR4argsWriter) StartAttrRequest() Bitmap4Writer {
	child := StartBitmap4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *READDIR4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *READDIR4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// Entry4Opt — union on xdr_bool (external discriminant)
// -------------------------------------------------------

type Entry4Opt struct {
	b    []byte
	disc uint32
}

func readEntry4Opt(b *[]byte, xdrBool uint32) (Entry4Opt, bool) {
	switch xdrBool {
	case TRUE:
		r, ok := readEntry4(b)
		if !ok {
			return Entry4Opt{}, false
		}
		return Entry4Opt{b: r.data, disc: xdrBool}, true
	default:
		return Entry4Opt{b: (*b)[:0], disc: xdrBool}, true
	}
}

func (m Entry4Opt) AsEntry4() Entry4 {
	if m.disc != TRUE {
		panic("wrong union discriminant")
	}
	v, _ := ReadEntry4(m.b)
	return v
}

// -------------------------------------------------------
// Entry4 — variable:
//   cookie(8) + name + attrs + nextentry_present(4) + entry4_opt nextentry
// -------------------------------------------------------

type Entry4 struct {
	data []byte
	off1 int // byte offset within data where attrs starts
	off2 int // byte offset within data where nextentry_present starts
}

func readEntry4(b *[]byte) (Entry4, bool) {
	if len(*b) < 8 {
		return Entry4{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[8:]
	if _, ok := readComponent4(b); !ok {
		return Entry4{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readFattr4(b); !ok {
		return Entry4{}, false
	}
	off2 := startLen - len(*b)
	if len(*b) < 4 {
		return Entry4{}, false
	}
	c_nextentry_present := binary.BigEndian.Uint32((*b)[:4])
	*b = (*b)[4:]
	if _, ok := readEntry4Opt(b, c_nextentry_present); !ok {
		return Entry4{}, false
	}
	total := startLen - len(*b)
	return Entry4{data: start[:total], off1: off1, off2: off2}, true
}

func ReadEntry4(b []byte) (Entry4, bool) {
	return readEntry4(&b)
}

func (m Entry4) Cookie() uint64 {
	return binary.BigEndian.Uint64(m.data[0:8])
}

func (m Entry4) Name() Component4 {
	return Component4(m.data[8:m.off1])
}

func (m Entry4) Attrs() Fattr4 {
	v, _ := ReadFattr4(m.data[m.off1:m.off2])
	return v
}

func (m Entry4) NextentryPresent() uint32 {
	o := m.off2
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m Entry4) Nextentry() Entry4Opt {
	return Entry4Opt{b: m.data[m.off2+4:], disc: binary.BigEndian.Uint32(m.data[m.off2 : m.off2+4])}
}

// Entry4Writer writes a entry4:
//
//	cookie + name + attrs + nextentry_present + nextentry(nextentry_present)
type Entry4Writer struct {
	buf    []byte
	header *[8]byte
	phase  uint8
}

func StartEntry4(buf []byte) Entry4Writer {
	buf = append(buf, make([]byte, 8)...) // cookie(8)
	return Entry4Writer{buf: buf, header: (*[8]byte)(buf[len(buf)-8:])}
}

func (w *Entry4Writer) SetCookie(v uint64) {
	binary.BigEndian.PutUint64(w.header[0:8], v)
}

func (w *Entry4Writer) SetNextentry_True() Entry4Writer {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = append(w.buf, make([]byte, 4+8)...)
	off := len(w.buf) - 4 - 8
	binary.BigEndian.PutUint32((*[4 + 8]byte)(w.buf[off:])[:4], TRUE)
	buf := w.buf
	w.buf = nil
	return Entry4Writer{buf: buf, header: (*[8]byte)(buf[off+4:])}
}

func (w *Entry4Writer) SetNextentry_Default(xdrBool uint32) {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = binary.BigEndian.AppendUint32(w.buf, xdrBool)
}

func (w *Entry4Writer) StartName() Component4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartComponent4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *Entry4Writer) StartAttrs() Fattr4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartFattr4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *Entry4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *Entry4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// Dirlist4 — variable: entries_present(4) + entry4_opt entries + eof(4)
// -------------------------------------------------------

type Dirlist4 struct {
	data []byte
	off1 int // byte offset within data where eof starts
}

func readDirlist4(b *[]byte) (Dirlist4, bool) {
	if len(*b) < 4 {
		return Dirlist4{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readEntry4Opt(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return Dirlist4{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return Dirlist4{}, false
	}
	*b = (*b)[4:]
	total := startLen - len(*b)
	return Dirlist4{data: start[:total], off1: off1}, true
}

func ReadDirlist4(b []byte) (Dirlist4, bool) {
	return readDirlist4(&b)
}

func (m Dirlist4) EntriesPresent() uint32 {
	return binary.BigEndian.Uint32(m.data[0:4])
}

func (m Dirlist4) Entries() Entry4Opt {
	return Entry4Opt{b: m.data[4:m.off1], disc: binary.BigEndian.Uint32(m.data[0:4])}
}

func (m Dirlist4) Eof() uint32 {
	o := m.off1
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

// Dirlist4Writer writes a dirlist4:
//
//	entries_present + entries(entries_present) + eof
type Dirlist4Writer struct {
	buf    []byte
	header *[4]byte
}

func StartDirlist4(buf []byte) Dirlist4Writer {
	buf = append(buf, make([]byte, 4)...) // entries_present(4)
	return Dirlist4Writer{buf: buf, header: (*[4]byte)(buf[len(buf)-4:])}
}

func (w *Dirlist4Writer) SetEntriesPresent(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *Dirlist4Writer) SetEof(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *Dirlist4Writer) SetEntries_True() Entry4Writer {
	binary.BigEndian.PutUint32(w.header[0:4], TRUE)
	w.buf = append(w.buf, make([]byte, 8)...)
	buf := w.buf
	w.buf = nil
	return Entry4Writer{buf: buf, header: (*[8]byte)(buf[len(buf)-8:])}
}

func (w *Dirlist4Writer) SetEntries_Default(xdrBool uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], xdrBool)
}

func (w *Dirlist4Writer) Resume(buf []byte) {
	w.buf = buf
}

func (w *Dirlist4Writer) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// READDIR4resok — variable: cookieverf(8) + reply
// -------------------------------------------------------

type READDIR4resok []byte

func readREADDIR4resok(b *[]byte) (READDIR4resok, bool) {
	if len(*b) < 8 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[8:]
	if _, ok := readDirlist4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return READDIR4resok(start[:total]), true
}

func ReadREADDIR4resok(b []byte) (READDIR4resok, bool) {
	return readREADDIR4resok(&b)
}

func (m READDIR4resok) Cookieverf() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m[0 : 0+verifier4Size])}
}

func (m READDIR4resok) Reply() Dirlist4 {
	v, _ := ReadDirlist4(m[8:])
	return v
}

// READDIR4resokWriter writes a READDIR4resok:
//
//	cookieverf + reply
type READDIR4resokWriter struct {
	buf    []byte
	header *[8]byte
}

func StartREADDIR4resok(buf []byte) READDIR4resokWriter {
	buf = append(buf, make([]byte, 8)...) // cookieverf(8)
	return READDIR4resokWriter{buf: buf, header: (*[8]byte)(buf[len(buf)-8:])}
}

func (w *READDIR4resokWriter) Cookieverf() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(w.header[0:])}
}

func (w *READDIR4resokWriter) StartReply() Dirlist4Writer {
	child := StartDirlist4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *READDIR4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *READDIR4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// READDIR4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type READDIR4res struct {
	b    []byte
	disc uint32
}

func readREADDIR4res(b *[]byte, nfsstat4 uint32) (READDIR4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readREADDIR4resok(b)
		if !ok {
			return READDIR4res{}, false
		}
		return READDIR4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return READDIR4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m READDIR4res) AsREADDIR4resok() READDIR4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return READDIR4resok(m.b)
}

// -------------------------------------------------------
// READLINK4resok — variable: link
// -------------------------------------------------------

type READLINK4resok []byte

func readREADLINK4resok(b *[]byte) (READLINK4resok, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readLinktext4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return READLINK4resok(start[:total]), true
}

func ReadREADLINK4resok(b []byte) (READLINK4resok, bool) {
	return readREADLINK4resok(&b)
}

func (m READLINK4resok) Link() Linktext4 {
	return Linktext4(m[0:])
}

// READLINK4resokWriter writes a READLINK4resok:
//
//	link
type READLINK4resokWriter struct {
	buf []byte
	off int
}

func StartREADLINK4resok(buf []byte) READLINK4resokWriter {
	off := len(buf)
	return READLINK4resokWriter{buf: buf, off: off}
}

func (w *READLINK4resokWriter) StartLink() Linktext4Writer {
	child := StartLinktext4(w.buf)
	w.buf = nil
	return child
}

func (w *READLINK4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *READLINK4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// READLINK4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type READLINK4res struct {
	b    []byte
	disc uint32
}

func readREADLINK4res(b *[]byte, nfsstat4 uint32) (READLINK4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readREADLINK4resok(b)
		if !ok {
			return READLINK4res{}, false
		}
		return READLINK4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return READLINK4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m READLINK4res) AsREADLINK4resok() READLINK4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return READLINK4resok(m.b)
}

// -------------------------------------------------------
// REMOVE4args — variable: target
// -------------------------------------------------------

type REMOVE4args []byte

func readREMOVE4args(b *[]byte) (REMOVE4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readComponent4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return REMOVE4args(start[:total]), true
}

func ReadREMOVE4args(b []byte) (REMOVE4args, bool) {
	return readREMOVE4args(&b)
}

func (m REMOVE4args) Target() Component4 {
	return Component4(m[0:])
}

// REMOVE4argsWriter writes a REMOVE4args:
//
//	target
type REMOVE4argsWriter struct {
	buf []byte
	off int
}

func StartREMOVE4args(buf []byte) REMOVE4argsWriter {
	off := len(buf)
	return REMOVE4argsWriter{buf: buf, off: off}
}

func (w *REMOVE4argsWriter) StartTarget() Component4Writer {
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *REMOVE4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *REMOVE4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// REMOVE4resok — fixed 20 bytes: change_info4(20)
// -------------------------------------------------------

type REMOVE4resok struct {
	m *[rEMOVE4resokSize]byte
}

const rEMOVE4resokSize = 20

func readREMOVE4resok(b *[]byte) (REMOVE4resok, bool) {
	if len(*b) < rEMOVE4resokSize {
		return REMOVE4resok{}, false
	}
	result := REMOVE4resok{m: (*[rEMOVE4resokSize]byte)(*b)}
	*b = (*b)[rEMOVE4resokSize:]
	return result, true
}

func ReadREMOVE4resok(b []byte) (REMOVE4resok, bool) {
	return readREMOVE4resok(&b)
}

func StartREMOVE4resok(buf []byte) ([]byte, REMOVE4resok) {
	buf = append(buf, make([]byte, rEMOVE4resokSize)...)
	return buf, REMOVE4resok{m: (*[rEMOVE4resokSize]byte)(buf[len(buf)-rEMOVE4resokSize:])}
}

func (m REMOVE4resok) Cinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(m.m[0:20])}
}

// -------------------------------------------------------
// REMOVE4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type REMOVE4res struct {
	b    []byte
	disc uint32
}

func readREMOVE4res(b *[]byte, nfsstat4 uint32) (REMOVE4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readREMOVE4resok(b)
		if !ok {
			return REMOVE4res{}, false
		}
		return REMOVE4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return REMOVE4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m REMOVE4res) AsREMOVE4resok() REMOVE4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return REMOVE4resok{m: (*[rEMOVE4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// RENAME4args — variable: oldname + newname
// -------------------------------------------------------

type RENAME4args struct {
	data []byte
	off1 int // byte offset within data where newname starts
}

func readRENAME4args(b *[]byte) (RENAME4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readComponent4(b); !ok {
		return RENAME4args{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readComponent4(b); !ok {
		return RENAME4args{}, false
	}
	total := startLen - len(*b)
	return RENAME4args{data: start[:total], off1: off1}, true
}

func ReadRENAME4args(b []byte) (RENAME4args, bool) {
	return readRENAME4args(&b)
}

func (m RENAME4args) Oldname() Component4 {
	return Component4(m.data[0:m.off1])
}

func (m RENAME4args) Newname() Component4 {
	return Component4(m.data[m.off1:])
}

// RENAME4argsWriter writes a RENAME4args:
//
//	oldname + newname
type RENAME4argsWriter struct {
	buf   []byte
	off   int
	phase uint8
}

func StartRENAME4args(buf []byte) RENAME4argsWriter {
	off := len(buf)
	return RENAME4argsWriter{buf: buf, off: off}
}

func (w *RENAME4argsWriter) StartOldname() Component4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *RENAME4argsWriter) StartNewname() Component4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *RENAME4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *RENAME4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// RENAME4resok — fixed 40 bytes: change_info4(20) + change_info4(20)
// -------------------------------------------------------

type RENAME4resok struct {
	m *[rENAME4resokSize]byte
}

const rENAME4resokSize = 40

func readRENAME4resok(b *[]byte) (RENAME4resok, bool) {
	if len(*b) < rENAME4resokSize {
		return RENAME4resok{}, false
	}
	result := RENAME4resok{m: (*[rENAME4resokSize]byte)(*b)}
	*b = (*b)[rENAME4resokSize:]
	return result, true
}

func ReadRENAME4resok(b []byte) (RENAME4resok, bool) {
	return readRENAME4resok(&b)
}

func StartRENAME4resok(buf []byte) ([]byte, RENAME4resok) {
	buf = append(buf, make([]byte, rENAME4resokSize)...)
	return buf, RENAME4resok{m: (*[rENAME4resokSize]byte)(buf[len(buf)-rENAME4resokSize:])}
}

func (m RENAME4resok) SourceCinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(m.m[0:20])}
}

func (m RENAME4resok) TargetCinfo() ChangeInfo4 {
	return ChangeInfo4{m: (*[changeInfo4Size]byte)(m.m[20:40])}
}

// -------------------------------------------------------
// RENAME4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type RENAME4res struct {
	b    []byte
	disc uint32
}

func readRENAME4res(b *[]byte, nfsstat4 uint32) (RENAME4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readRENAME4resok(b)
		if !ok {
			return RENAME4res{}, false
		}
		return RENAME4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return RENAME4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m RENAME4res) AsRENAME4resok() RENAME4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return RENAME4resok{m: (*[rENAME4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// RENEW4args — fixed 8 bytes: clientid(8, beu64)
// -------------------------------------------------------

type RENEW4args struct {
	m *[rENEW4argsSize]byte
}

const rENEW4argsSize = 8

func readRENEW4args(b *[]byte) (RENEW4args, bool) {
	if len(*b) < rENEW4argsSize {
		return RENEW4args{}, false
	}
	result := RENEW4args{m: (*[rENEW4argsSize]byte)(*b)}
	*b = (*b)[rENEW4argsSize:]
	return result, true
}

func ReadRENEW4args(b []byte) (RENEW4args, bool) {
	return readRENEW4args(&b)
}

func StartRENEW4args(buf []byte) ([]byte, RENEW4args) {
	buf = append(buf, make([]byte, rENEW4argsSize)...)
	return buf, RENEW4args{m: (*[rENEW4argsSize]byte)(buf[len(buf)-rENEW4argsSize:])}
}

func (m RENEW4args) Clientid() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m RENEW4args) SetClientid(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

// -------------------------------------------------------
// RENEW4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type RENEW4res struct {
	m *[rENEW4resSize]byte
}

const rENEW4resSize = 4

func readRENEW4res(b *[]byte) (RENEW4res, bool) {
	if len(*b) < rENEW4resSize {
		return RENEW4res{}, false
	}
	result := RENEW4res{m: (*[rENEW4resSize]byte)(*b)}
	*b = (*b)[rENEW4resSize:]
	return result, true
}

func ReadRENEW4res(b []byte) (RENEW4res, bool) {
	return readRENEW4res(&b)
}

func StartRENEW4res(buf []byte) ([]byte, RENEW4res) {
	buf = append(buf, make([]byte, rENEW4resSize)...)
	return buf, RENEW4res{m: (*[rENEW4resSize]byte)(buf[len(buf)-rENEW4resSize:])}
}

func (m RENEW4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m RENEW4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// RESTOREFH4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type RESTOREFH4res struct {
	m *[rESTOREFH4resSize]byte
}

const rESTOREFH4resSize = 4

func readRESTOREFH4res(b *[]byte) (RESTOREFH4res, bool) {
	if len(*b) < rESTOREFH4resSize {
		return RESTOREFH4res{}, false
	}
	result := RESTOREFH4res{m: (*[rESTOREFH4resSize]byte)(*b)}
	*b = (*b)[rESTOREFH4resSize:]
	return result, true
}

func ReadRESTOREFH4res(b []byte) (RESTOREFH4res, bool) {
	return readRESTOREFH4res(&b)
}

func StartRESTOREFH4res(buf []byte) ([]byte, RESTOREFH4res) {
	buf = append(buf, make([]byte, rESTOREFH4resSize)...)
	return buf, RESTOREFH4res{m: (*[rESTOREFH4resSize]byte)(buf[len(buf)-rESTOREFH4resSize:])}
}

func (m RESTOREFH4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m RESTOREFH4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// SAVEFH4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type SAVEFH4res struct {
	m *[sAVEFH4resSize]byte
}

const sAVEFH4resSize = 4

func readSAVEFH4res(b *[]byte) (SAVEFH4res, bool) {
	if len(*b) < sAVEFH4resSize {
		return SAVEFH4res{}, false
	}
	result := SAVEFH4res{m: (*[sAVEFH4resSize]byte)(*b)}
	*b = (*b)[sAVEFH4resSize:]
	return result, true
}

func ReadSAVEFH4res(b []byte) (SAVEFH4res, bool) {
	return readSAVEFH4res(&b)
}

func StartSAVEFH4res(buf []byte) ([]byte, SAVEFH4res) {
	buf = append(buf, make([]byte, sAVEFH4resSize)...)
	return buf, SAVEFH4res{m: (*[sAVEFH4resSize]byte)(buf[len(buf)-sAVEFH4resSize:])}
}

func (m SAVEFH4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m SAVEFH4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// SECINFO4args — variable: name
// -------------------------------------------------------

type SECINFO4args []byte

func readSECINFO4args(b *[]byte) (SECINFO4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readComponent4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return SECINFO4args(start[:total]), true
}

func ReadSECINFO4args(b []byte) (SECINFO4args, bool) {
	return readSECINFO4args(&b)
}

func (m SECINFO4args) Name() Component4 {
	return Component4(m[0:])
}

// SECINFO4argsWriter writes a SECINFO4args:
//
//	name
type SECINFO4argsWriter struct {
	buf []byte
	off int
}

func StartSECINFO4args(buf []byte) SECINFO4argsWriter {
	off := len(buf)
	return SECINFO4argsWriter{buf: buf, off: off}
}

func (w *SECINFO4argsWriter) StartName() Component4Writer {
	child := StartComponent4(w.buf)
	w.buf = nil
	return child
}

func (w *SECINFO4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SECINFO4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// RpcsecGssInfo — variable: oid + qop(4) + service(4)
// -------------------------------------------------------

type RpcsecGssInfo struct {
	data []byte
	off1 int // byte offset within data where qop starts
}

func readRpcsecGssInfo(b *[]byte) (RpcsecGssInfo, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readSecOid4(b); !ok {
		return RpcsecGssInfo{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return RpcsecGssInfo{}, false
	}
	*b = (*b)[4:]
	if len(*b) < 4 {
		return RpcsecGssInfo{}, false
	}
	*b = (*b)[4:]
	total := startLen - len(*b)
	return RpcsecGssInfo{data: start[:total], off1: off1}, true
}

func ReadRpcsecGssInfo(b []byte) (RpcsecGssInfo, bool) {
	return readRpcsecGssInfo(&b)
}

func (m RpcsecGssInfo) Oid() SecOid4 {
	return SecOid4(m.data[0:m.off1])
}

func (m RpcsecGssInfo) Qop() uint32 {
	o := m.off1
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m RpcsecGssInfo) Service() uint32 {
	o := m.off1 + 4
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

// RpcsecGssInfoWriter writes a rpcsec_gss_info:
//
//	oid + qop + service
type RpcsecGssInfoWriter struct {
	buf []byte
	off int
}

func StartRpcsecGssInfo(buf []byte) RpcsecGssInfoWriter {
	off := len(buf)
	return RpcsecGssInfoWriter{buf: buf, off: off}
}

func (w *RpcsecGssInfoWriter) SetQop(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *RpcsecGssInfoWriter) SetService(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *RpcsecGssInfoWriter) StartOid() SecOid4Writer {
	child := StartSecOid4(w.buf)
	w.buf = nil
	return child
}

func (w *RpcsecGssInfoWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *RpcsecGssInfoWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// Secinfo4 — union on disc_secinfo4 (external discriminant)
// -------------------------------------------------------

type Secinfo4 struct {
	b    []byte
	disc uint32
}

func readSecinfo4(b *[]byte, discSecinfo4 uint32) (Secinfo4, bool) {
	switch discSecinfo4 {
	case RPCSEC_GSS:
		r, ok := readRpcsecGssInfo(b)
		if !ok {
			return Secinfo4{}, false
		}
		return Secinfo4{b: r.data, disc: discSecinfo4}, true
	default:
		return Secinfo4{b: (*b)[:0], disc: discSecinfo4}, true
	}
}

func (m Secinfo4) AsRpcsecGssInfo() RpcsecGssInfo {
	if m.disc != RPCSEC_GSS {
		panic("wrong union discriminant")
	}
	v, _ := ReadRpcsecGssInfo(m.b)
	return v
}

// -------------------------------------------------------
// Secinfo4Entry — variable: disc(4) + secinfo4 value
// -------------------------------------------------------

type Secinfo4Entry []byte

func readSecinfo4Entry(b *[]byte) (Secinfo4Entry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readSecinfo4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return Secinfo4Entry(start[:total]), true
}

func ReadSecinfo4Entry(b []byte) (Secinfo4Entry, bool) {
	return readSecinfo4Entry(&b)
}

func (m Secinfo4Entry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m Secinfo4Entry) Value() Secinfo4 {
	return Secinfo4{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// Secinfo4EntryWriter writes a secinfo4_entry:
//
//	disc + value(disc)
type Secinfo4EntryWriter struct {
	buf []byte
	off int
}

func StartSecinfo4Entry(buf []byte) Secinfo4EntryWriter {
	return Secinfo4EntryWriter{buf: buf, off: len(buf)}
}

func (w *Secinfo4EntryWriter) SetValue_Gss() RpcsecGssInfoWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, RPCSEC_GSS)
	child := StartRpcsecGssInfo(w.buf)
	w.buf = nil
	return child
}

func (w *Secinfo4EntryWriter) SetValue_Default(discSecinfo4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, discSecinfo4)
}

func (w *Secinfo4EntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *Secinfo4EntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// SECINFO4resok — variable: count(4) + secinfo4_entry data[count]
// -------------------------------------------------------

type SECINFO4resok []byte

func readSECINFO4resok(b *[]byte) (SECINFO4resok, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	c_count := int(binary.BigEndian.Uint32((*b)[0:4]))
	*b = (*b)[4:]
	for i := 0; i < c_count; i++ {
		if _, ok := readSecinfo4Entry(b); !ok {
			return nil, false
		}
	}
	total := startLen - len(*b)
	return SECINFO4resok(start[:total]), true
}

func ReadSECINFO4resok(b []byte) (SECINFO4resok, bool) {
	if len(b) < 4 {
		return nil, false
	}
	return SECINFO4resok(b), true
}

func (m SECINFO4resok) Count() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m SECINFO4resok) Data() Secinfo4EntryIter {
	count := int(binary.BigEndian.Uint32(m[0:4]))
	return Secinfo4EntryIter{
		b:     []byte(m[4:]),
		count: count,
	}
}

// Secinfo4EntryIter iterates over variable-size Secinfo4Entry entries.
type Secinfo4EntryIter struct {
	b     []byte
	count int
	i     int
	cur   Secinfo4Entry
}

func (it *Secinfo4EntryIter) Next() bool {
	if it.i >= it.count {
		return false
	}
	var ok bool
	it.cur, ok = readSecinfo4Entry(&it.b)
	if !ok {
		return false
	}
	it.i++
	return true
}

func (it *Secinfo4EntryIter) Data() Secinfo4Entry {
	return it.cur
}

// SECINFO4resokWriter writes a SECINFO4resok:
//
//	count + secinfo4_entry data[count]
type SECINFO4resokWriter struct {
	buf   []byte
	off   int
	count uint32
}

func StartSECINFO4resok(buf []byte) SECINFO4resokWriter {
	off := len(buf)
	buf = binary.BigEndian.AppendUint32(buf, 0) // count placeholder
	return SECINFO4resokWriter{buf: buf, off: off}
}

func (w *SECINFO4resokWriter) AppendData_Gss() RpcsecGssInfoWriter {
	_ = w.buf[:1]
	w.buf = binary.BigEndian.AppendUint32(w.buf, RPCSEC_GSS)
	w.count++
	child := StartRpcsecGssInfo(w.buf)
	w.buf = nil
	return child
}

func (w *SECINFO4resokWriter) AppendData_Default(discSecinfo4 uint32) {
	_ = w.buf[:1]
	w.buf = binary.BigEndian.AppendUint32(w.buf, discSecinfo4)
	w.count++
}

func (w *SECINFO4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SECINFO4resokWriter) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], w.count)
	return w.buf
}

// -------------------------------------------------------
// SECINFO4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type SECINFO4res struct {
	b    []byte
	disc uint32
}

func readSECINFO4res(b *[]byte, nfsstat4 uint32) (SECINFO4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readSECINFO4resok(b)
		if !ok {
			return SECINFO4res{}, false
		}
		return SECINFO4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return SECINFO4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m SECINFO4res) AsSECINFO4resok() SECINFO4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return SECINFO4resok(m.b)
}

// -------------------------------------------------------
// SETATTR4args — variable: stateid(16) + obj_attributes
// -------------------------------------------------------

type SETATTR4args []byte

func readSETATTR4args(b *[]byte) (SETATTR4args, bool) {
	if len(*b) < 16 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[16:]
	if _, ok := readFattr4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return SETATTR4args(start[:total]), true
}

func ReadSETATTR4args(b []byte) (SETATTR4args, bool) {
	return readSETATTR4args(&b)
}

func (m SETATTR4args) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m[0 : 0+stateid4Size])}
}

func (m SETATTR4args) ObjAttributes() Fattr4 {
	v, _ := ReadFattr4(m[16:])
	return v
}

// SETATTR4argsWriter writes a SETATTR4args:
//
//	stateid + obj_attributes
type SETATTR4argsWriter struct {
	buf    []byte
	header *[16]byte
}

func StartSETATTR4args(buf []byte) SETATTR4argsWriter {
	buf = append(buf, make([]byte, 16)...) // stateid(16)
	return SETATTR4argsWriter{buf: buf, header: (*[16]byte)(buf[len(buf)-16:])}
}

func (w *SETATTR4argsWriter) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[0:])}
}

func (w *SETATTR4argsWriter) StartObjAttributes() Fattr4Writer {
	child := StartFattr4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *SETATTR4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SETATTR4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// SETATTR4res — variable: status(4) + attrsset
// -------------------------------------------------------

type SETATTR4res []byte

func readSETATTR4res(b *[]byte) (SETATTR4res, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readBitmap4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return SETATTR4res(start[:total]), true
}

func ReadSETATTR4res(b []byte) (SETATTR4res, bool) {
	return readSETATTR4res(&b)
}

func (m SETATTR4res) Status() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m SETATTR4res) Attrsset() Bitmap4 {
	return Bitmap4(m[4:])
}

// SETATTR4resWriter writes a SETATTR4res:
//
//	status + attrsset
type SETATTR4resWriter struct {
	buf    []byte
	header *[4]byte
}

func StartSETATTR4res(buf []byte) SETATTR4resWriter {
	buf = append(buf, make([]byte, 4)...) // status(4)
	return SETATTR4resWriter{buf: buf, header: (*[4]byte)(buf[len(buf)-4:])}
}

func (w *SETATTR4resWriter) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(w.header[0:4], v)
}

func (w *SETATTR4resWriter) StartAttrsset() Bitmap4Writer {
	child := StartBitmap4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *SETATTR4resWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SETATTR4resWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// SETCLIENTID4args — variable: client + callback + callback_ident(4)
// -------------------------------------------------------

type SETCLIENTID4args struct {
	data []byte
	off1 int // byte offset within data where callback starts
	off2 int // byte offset within data where callback_ident starts
}

func readSETCLIENTID4args(b *[]byte) (SETCLIENTID4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readNfsClientId4(b); !ok {
		return SETCLIENTID4args{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readCbClient4(b); !ok {
		return SETCLIENTID4args{}, false
	}
	off2 := startLen - len(*b)
	if len(*b) < 4 {
		return SETCLIENTID4args{}, false
	}
	*b = (*b)[4:]
	total := startLen - len(*b)
	return SETCLIENTID4args{data: start[:total], off1: off1, off2: off2}, true
}

func ReadSETCLIENTID4args(b []byte) (SETCLIENTID4args, bool) {
	return readSETCLIENTID4args(&b)
}

func (m SETCLIENTID4args) Client() NfsClientId4 {
	return NfsClientId4(m.data[0:m.off1])
}

func (m SETCLIENTID4args) Callback() CbClient4 {
	return CbClient4(m.data[m.off1:m.off2])
}

func (m SETCLIENTID4args) CallbackIdent() uint32 {
	o := m.off2
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

// SETCLIENTID4argsWriter writes a SETCLIENTID4args:
//
//	client + callback + callback_ident
type SETCLIENTID4argsWriter struct {
	buf   []byte
	off   int
	phase uint8
}

func StartSETCLIENTID4args(buf []byte) SETCLIENTID4argsWriter {
	off := len(buf)
	return SETCLIENTID4argsWriter{buf: buf, off: off}
}

func (w *SETCLIENTID4argsWriter) SetCallbackIdent(v uint32) {
	if w.phase > 2 {
		panic("writer fields called out of order")
	}
	w.phase = 2
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *SETCLIENTID4argsWriter) StartClient() NfsClientId4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartNfsClientId4(w.buf)
	w.buf = nil
	return child
}

func (w *SETCLIENTID4argsWriter) StartCallback() CbClient4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartCbClient4(w.buf)
	w.buf = nil
	return child
}

func (w *SETCLIENTID4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SETCLIENTID4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// SETCLIENTID4resok — fixed 16 bytes: clientid(8, beu64) + verifier4(8)
// -------------------------------------------------------

type SETCLIENTID4resok struct {
	m *[sETCLIENTID4resokSize]byte
}

const sETCLIENTID4resokSize = 16

func readSETCLIENTID4resok(b *[]byte) (SETCLIENTID4resok, bool) {
	if len(*b) < sETCLIENTID4resokSize {
		return SETCLIENTID4resok{}, false
	}
	result := SETCLIENTID4resok{m: (*[sETCLIENTID4resokSize]byte)(*b)}
	*b = (*b)[sETCLIENTID4resokSize:]
	return result, true
}

func ReadSETCLIENTID4resok(b []byte) (SETCLIENTID4resok, bool) {
	return readSETCLIENTID4resok(&b)
}

func StartSETCLIENTID4resok(buf []byte) ([]byte, SETCLIENTID4resok) {
	buf = append(buf, make([]byte, sETCLIENTID4resokSize)...)
	return buf, SETCLIENTID4resok{m: (*[sETCLIENTID4resokSize]byte)(buf[len(buf)-sETCLIENTID4resokSize:])}
}

func (m SETCLIENTID4resok) Clientid() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m SETCLIENTID4resok) SetclientidConfirm() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m.m[8:16])}
}

func (m SETCLIENTID4resok) SetClientid(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

// -------------------------------------------------------
// SETCLIENTID4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type SETCLIENTID4res struct {
	b    []byte
	disc uint32
}

func readSETCLIENTID4res(b *[]byte, nfsstat4 uint32) (SETCLIENTID4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readSETCLIENTID4resok(b)
		if !ok {
			return SETCLIENTID4res{}, false
		}
		return SETCLIENTID4res{b: r.m[:], disc: nfsstat4}, true
	case NFS4ERR_CLID_INUSE:
		r, ok := readClientaddr4(b)
		if !ok {
			return SETCLIENTID4res{}, false
		}
		return SETCLIENTID4res{b: r.data, disc: nfsstat4}, true
	default:
		return SETCLIENTID4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m SETCLIENTID4res) AsSETCLIENTID4resok() SETCLIENTID4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return SETCLIENTID4resok{m: (*[sETCLIENTID4resokSize]byte)(m.b)}
}

func (m SETCLIENTID4res) AsClientaddr4() Clientaddr4 {
	if m.disc != NFS4ERR_CLID_INUSE {
		panic("wrong union discriminant")
	}
	v, _ := ReadClientaddr4(m.b)
	return v
}

// -------------------------------------------------------
// SETCLIENTIDCONFIRM4args — fixed 16 bytes: clientid(8, beu64) + verifier4(8)
// -------------------------------------------------------

type SETCLIENTIDCONFIRM4args struct {
	m *[sETCLIENTIDCONFIRM4argsSize]byte
}

const sETCLIENTIDCONFIRM4argsSize = 16

func readSETCLIENTIDCONFIRM4args(b *[]byte) (SETCLIENTIDCONFIRM4args, bool) {
	if len(*b) < sETCLIENTIDCONFIRM4argsSize {
		return SETCLIENTIDCONFIRM4args{}, false
	}
	result := SETCLIENTIDCONFIRM4args{m: (*[sETCLIENTIDCONFIRM4argsSize]byte)(*b)}
	*b = (*b)[sETCLIENTIDCONFIRM4argsSize:]
	return result, true
}

func ReadSETCLIENTIDCONFIRM4args(b []byte) (SETCLIENTIDCONFIRM4args, bool) {
	return readSETCLIENTIDCONFIRM4args(&b)
}

func StartSETCLIENTIDCONFIRM4args(buf []byte) ([]byte, SETCLIENTIDCONFIRM4args) {
	buf = append(buf, make([]byte, sETCLIENTIDCONFIRM4argsSize)...)
	return buf, SETCLIENTIDCONFIRM4args{m: (*[sETCLIENTIDCONFIRM4argsSize]byte)(buf[len(buf)-sETCLIENTIDCONFIRM4argsSize:])}
}

func (m SETCLIENTIDCONFIRM4args) Clientid() uint64 {
	return binary.BigEndian.Uint64(m.m[0:8])
}

func (m SETCLIENTIDCONFIRM4args) SetclientidConfirm() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m.m[8:16])}
}

func (m SETCLIENTIDCONFIRM4args) SetClientid(v uint64) {
	binary.BigEndian.PutUint64(m.m[0:8], v)
}

// -------------------------------------------------------
// SETCLIENTIDCONFIRM4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type SETCLIENTIDCONFIRM4res struct {
	m *[sETCLIENTIDCONFIRM4resSize]byte
}

const sETCLIENTIDCONFIRM4resSize = 4

func readSETCLIENTIDCONFIRM4res(b *[]byte) (SETCLIENTIDCONFIRM4res, bool) {
	if len(*b) < sETCLIENTIDCONFIRM4resSize {
		return SETCLIENTIDCONFIRM4res{}, false
	}
	result := SETCLIENTIDCONFIRM4res{m: (*[sETCLIENTIDCONFIRM4resSize]byte)(*b)}
	*b = (*b)[sETCLIENTIDCONFIRM4resSize:]
	return result, true
}

func ReadSETCLIENTIDCONFIRM4res(b []byte) (SETCLIENTIDCONFIRM4res, bool) {
	return readSETCLIENTIDCONFIRM4res(&b)
}

func StartSETCLIENTIDCONFIRM4res(buf []byte) ([]byte, SETCLIENTIDCONFIRM4res) {
	buf = append(buf, make([]byte, sETCLIENTIDCONFIRM4resSize)...)
	return buf, SETCLIENTIDCONFIRM4res{m: (*[sETCLIENTIDCONFIRM4resSize]byte)(buf[len(buf)-sETCLIENTIDCONFIRM4resSize:])}
}

func (m SETCLIENTIDCONFIRM4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m SETCLIENTIDCONFIRM4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// VERIFY4args — variable: obj_attributes
// -------------------------------------------------------

type VERIFY4args []byte

func readVERIFY4args(b *[]byte) (VERIFY4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readFattr4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return VERIFY4args(start[:total]), true
}

func ReadVERIFY4args(b []byte) (VERIFY4args, bool) {
	return readVERIFY4args(&b)
}

func (m VERIFY4args) ObjAttributes() Fattr4 {
	v, _ := ReadFattr4(m[0:])
	return v
}

// VERIFY4argsWriter writes a VERIFY4args:
//
//	obj_attributes
type VERIFY4argsWriter struct {
	buf []byte
	off int
}

func StartVERIFY4args(buf []byte) VERIFY4argsWriter {
	off := len(buf)
	return VERIFY4argsWriter{buf: buf, off: off}
}

func (w *VERIFY4argsWriter) StartObjAttributes() Fattr4Writer {
	child := StartFattr4(w.buf)
	w.buf = nil
	return child
}

func (w *VERIFY4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *VERIFY4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// VERIFY4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type VERIFY4res struct {
	m *[vERIFY4resSize]byte
}

const vERIFY4resSize = 4

func readVERIFY4res(b *[]byte) (VERIFY4res, bool) {
	if len(*b) < vERIFY4resSize {
		return VERIFY4res{}, false
	}
	result := VERIFY4res{m: (*[vERIFY4resSize]byte)(*b)}
	*b = (*b)[vERIFY4resSize:]
	return result, true
}

func ReadVERIFY4res(b []byte) (VERIFY4res, bool) {
	return readVERIFY4res(&b)
}

func StartVERIFY4res(buf []byte) ([]byte, VERIFY4res) {
	buf = append(buf, make([]byte, vERIFY4resSize)...)
	return buf, VERIFY4res{m: (*[vERIFY4resSize]byte)(buf[len(buf)-vERIFY4resSize:])}
}

func (m VERIFY4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m VERIFY4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// WRITE4args — variable:
//   stateid(16) + offset(8) + stable(4) + data_len(4) + u8 data[data_len] + align(4)
// -------------------------------------------------------

type WRITE4args []byte

func readWRITE4args(b *[]byte) (WRITE4args, bool) {
	if len(*b) < 32 {
		return nil, false
	}
	n := int(binary.BigEndian.Uint32((*b)[28:32]))
	padded := (n + 3) &^ 3
	total := 32 + padded
	if len(*b) < total {
		return nil, false
	}
	result := WRITE4args((*b)[:total])
	*b = (*b)[total:]
	return result, true
}

func ReadWRITE4args(b []byte) (WRITE4args, bool) {
	if len(b) < 32 {
		return nil, false
	}
	count := int(binary.BigEndian.Uint32(b[28:32]))
	padded := (count + 3) &^ 3
	total := 32 + padded
	if len(b) < total {
		return nil, false
	}
	return WRITE4args(b[:total]), true
}

func (m WRITE4args) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m[0 : 0+stateid4Size])}
}

func (m WRITE4args) Offset() uint64 {
	return binary.BigEndian.Uint64(m[16:24])
}

func (m WRITE4args) Stable() uint32 {
	return binary.BigEndian.Uint32(m[24:28])
}

func (m WRITE4args) DataLen() uint32 {
	return binary.BigEndian.Uint32(m[28:32])
}

func (m WRITE4args) Data() []byte {
	n := int(binary.BigEndian.Uint32(m[28:32]))
	return m[32 : 32+n]
}

// WRITE4argsWriter writes a WRITE4args:
//
//	stateid + offset + stable + data_len + u8 data[data_len] + align(4)
type WRITE4argsWriter struct {
	buf     []byte
	off     int
	dataLen uint32
}

func StartWRITE4args(buf []byte) WRITE4argsWriter {
	off := len(buf)
	buf = append(buf, make([]byte, 32)...) // stateid(16) + offset(8) + stable(4) + data_len(4)
	return WRITE4argsWriter{buf: buf, off: off}
}

func (w *WRITE4argsWriter) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.buf[w.off:])}
}

func (w WRITE4argsWriter) SetOffset(v uint64) WRITE4argsWriter {
	binary.BigEndian.PutUint64((*[32]byte)(w.buf[w.off:])[16:24], v)
	return w
}

func (w WRITE4argsWriter) SetStable(v uint32) WRITE4argsWriter {
	binary.BigEndian.PutUint32((*[32]byte)(w.buf[w.off:])[24:28], v)
	return w
}

func (w WRITE4argsWriter) SetData(data []byte) WRITE4argsWriter {
	n := len(data)
	padded := (n + 3) &^ 3
	w.buf = append(w.buf, make([]byte, padded)...)
	copy(w.buf[len(w.buf)-padded:], data)
	w.dataLen = uint32(n)
	return w
}

func (w WRITE4argsWriter) Finish() []byte {
	binary.BigEndian.PutUint32((*[32]byte)(w.buf[w.off:])[28:32], w.dataLen)
	return w.buf
}

// -------------------------------------------------------
// WRITE4resok — fixed 16 bytes:
//   count(4, beu32) + committed(4, beu32) + verifier4(8)
// -------------------------------------------------------

type WRITE4resok struct {
	m *[wRITE4resokSize]byte
}

const wRITE4resokSize = 16

func readWRITE4resok(b *[]byte) (WRITE4resok, bool) {
	if len(*b) < wRITE4resokSize {
		return WRITE4resok{}, false
	}
	result := WRITE4resok{m: (*[wRITE4resokSize]byte)(*b)}
	*b = (*b)[wRITE4resokSize:]
	return result, true
}

func ReadWRITE4resok(b []byte) (WRITE4resok, bool) {
	return readWRITE4resok(&b)
}

func StartWRITE4resok(buf []byte) ([]byte, WRITE4resok) {
	buf = append(buf, make([]byte, wRITE4resokSize)...)
	return buf, WRITE4resok{m: (*[wRITE4resokSize]byte)(buf[len(buf)-wRITE4resokSize:])}
}

func (m WRITE4resok) Count() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m WRITE4resok) Committed() uint32 {
	return binary.BigEndian.Uint32(m.m[4:8])
}

func (m WRITE4resok) Writeverf() Verifier4 {
	return Verifier4{m: (*[verifier4Size]byte)(m.m[8:16])}
}

func (m WRITE4resok) SetCount(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

func (m WRITE4resok) SetCommitted(v uint32) {
	binary.BigEndian.PutUint32(m.m[4:8], v)
}

// -------------------------------------------------------
// WRITE4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type WRITE4res struct {
	b    []byte
	disc uint32
}

func readWRITE4res(b *[]byte, nfsstat4 uint32) (WRITE4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readWRITE4resok(b)
		if !ok {
			return WRITE4res{}, false
		}
		return WRITE4res{b: r.m[:], disc: nfsstat4}, true
	default:
		return WRITE4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m WRITE4res) AsWRITE4resok() WRITE4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return WRITE4resok{m: (*[wRITE4resokSize]byte)(m.b)}
}

// -------------------------------------------------------
// RELEASELOCKOWNER4args — variable: lock_owner
// -------------------------------------------------------

type RELEASELOCKOWNER4args []byte

func readRELEASELOCKOWNER4args(b *[]byte) (RELEASELOCKOWNER4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readLockOwner4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return RELEASELOCKOWNER4args(start[:total]), true
}

func ReadRELEASELOCKOWNER4args(b []byte) (RELEASELOCKOWNER4args, bool) {
	return readRELEASELOCKOWNER4args(&b)
}

func (m RELEASELOCKOWNER4args) LockOwner() LockOwner4 {
	return LockOwner4(m[0:])
}

// RELEASELOCKOWNER4argsWriter writes a RELEASE_LOCKOWNER4args:
//
//	lock_owner
type RELEASELOCKOWNER4argsWriter struct {
	buf []byte
	off int
}

func StartRELEASELOCKOWNER4args(buf []byte) RELEASELOCKOWNER4argsWriter {
	off := len(buf)
	return RELEASELOCKOWNER4argsWriter{buf: buf, off: off}
}

func (w *RELEASELOCKOWNER4argsWriter) StartLockOwner() LockOwner4Writer {
	child := StartLockOwner4(w.buf)
	w.buf = nil
	return child
}

func (w *RELEASELOCKOWNER4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *RELEASELOCKOWNER4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// RELEASELOCKOWNER4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type RELEASELOCKOWNER4res struct {
	m *[rELEASELOCKOWNER4resSize]byte
}

const rELEASELOCKOWNER4resSize = 4

func readRELEASELOCKOWNER4res(b *[]byte) (RELEASELOCKOWNER4res, bool) {
	if len(*b) < rELEASELOCKOWNER4resSize {
		return RELEASELOCKOWNER4res{}, false
	}
	result := RELEASELOCKOWNER4res{m: (*[rELEASELOCKOWNER4resSize]byte)(*b)}
	*b = (*b)[rELEASELOCKOWNER4resSize:]
	return result, true
}

func ReadRELEASELOCKOWNER4res(b []byte) (RELEASELOCKOWNER4res, bool) {
	return readRELEASELOCKOWNER4res(&b)
}

func StartRELEASELOCKOWNER4res(buf []byte) ([]byte, RELEASELOCKOWNER4res) {
	buf = append(buf, make([]byte, rELEASELOCKOWNER4resSize)...)
	return buf, RELEASELOCKOWNER4res{m: (*[rELEASELOCKOWNER4resSize]byte)(buf[len(buf)-rELEASELOCKOWNER4resSize:])}
}

func (m RELEASELOCKOWNER4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m RELEASELOCKOWNER4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// ILLEGAL4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type ILLEGAL4res struct {
	m *[iLLEGAL4resSize]byte
}

const iLLEGAL4resSize = 4

func readILLEGAL4res(b *[]byte) (ILLEGAL4res, bool) {
	if len(*b) < iLLEGAL4resSize {
		return ILLEGAL4res{}, false
	}
	result := ILLEGAL4res{m: (*[iLLEGAL4resSize]byte)(*b)}
	*b = (*b)[iLLEGAL4resSize:]
	return result, true
}

func ReadILLEGAL4res(b []byte) (ILLEGAL4res, bool) {
	return readILLEGAL4res(&b)
}

func StartILLEGAL4res(buf []byte) ([]byte, ILLEGAL4res) {
	buf = append(buf, make([]byte, iLLEGAL4resSize)...)
	return buf, ILLEGAL4res{m: (*[iLLEGAL4resSize]byte)(buf[len(buf)-iLLEGAL4resSize:])}
}

func (m ILLEGAL4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m ILLEGAL4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// NfsArgop4 — union on nfs_opnum4 (external discriminant)
// -------------------------------------------------------

type NfsArgop4 struct {
	b    []byte
	disc uint32
}

func readNfsArgop4(b *[]byte, nfsOpnum4 uint32) (NfsArgop4, bool) {
	switch nfsOpnum4 {
	case OP_ACCESS:
		r, ok := readACCESS4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_CLOSE:
		r, ok := readCLOSE4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_COMMIT:
		r, ok := readCOMMIT4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_CREATE:
		r, ok := readCREATE4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.data, disc: nfsOpnum4}, true
	case OP_DELEGPURGE:
		r, ok := readDELEGPURGE4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_DELEGRETURN:
		r, ok := readDELEGRETURN4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_GETATTR:
		r, ok := readGETATTR4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_GETFH:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_LINK:
		r, ok := readLINK4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOCK:
		r, ok := readLOCK4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOCKT:
		r, ok := readLOCKT4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOCKU:
		r, ok := readLOCKU4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_LOOKUP:
		r, ok := readLOOKUP4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOOKUPP:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_NVERIFY:
		r, ok := readNVERIFY4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_OPEN:
		r, ok := readOPEN4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.data, disc: nfsOpnum4}, true
	case OP_OPENATTR:
		r, ok := readOPENATTR4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_OPEN_CONFIRM:
		r, ok := readOPENCONFIRM4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_OPEN_DOWNGRADE:
		r, ok := readOPENDOWNGRADE4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_PUTFH:
		r, ok := readPUTFH4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_PUTPUBFH:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_PUTROOTFH:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_READ:
		r, ok := readREAD4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_READDIR:
		r, ok := readREADDIR4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_READLINK:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_REMOVE:
		r, ok := readREMOVE4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_RENAME:
		r, ok := readRENAME4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.data, disc: nfsOpnum4}, true
	case OP_RENEW:
		r, ok := readRENEW4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_RESTOREFH:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_SAVEFH:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	case OP_SECINFO:
		r, ok := readSECINFO4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_SETATTR:
		r, ok := readSETATTR4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_SETCLIENTID:
		r, ok := readSETCLIENTID4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.data, disc: nfsOpnum4}, true
	case OP_SETCLIENTID_CONFIRM:
		r, ok := readSETCLIENTIDCONFIRM4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_VERIFY:
		r, ok := readVERIFY4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_WRITE:
		r, ok := readWRITE4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_RELEASE_LOCKOWNER:
		r, ok := readRELEASELOCKOWNER4args(b)
		if !ok {
			return NfsArgop4{}, false
		}
		return NfsArgop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_ILLEGAL:
		return NfsArgop4{b: (*b)[:0], disc: nfsOpnum4}, true
	default:
		return NfsArgop4{}, false
	}
}

func (m NfsArgop4) AsACCESS4args() ACCESS4args {
	if m.disc != OP_ACCESS {
		panic("wrong union discriminant")
	}
	return ACCESS4args{m: (*[aCCESS4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsCLOSE4args() CLOSE4args {
	if m.disc != OP_CLOSE {
		panic("wrong union discriminant")
	}
	return CLOSE4args{m: (*[cLOSE4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsCOMMIT4args() COMMIT4args {
	if m.disc != OP_COMMIT {
		panic("wrong union discriminant")
	}
	return COMMIT4args{m: (*[cOMMIT4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsCREATE4args() CREATE4args {
	if m.disc != OP_CREATE {
		panic("wrong union discriminant")
	}
	v, _ := ReadCREATE4args(m.b)
	return v
}

func (m NfsArgop4) AsDELEGPURGE4args() DELEGPURGE4args {
	if m.disc != OP_DELEGPURGE {
		panic("wrong union discriminant")
	}
	return DELEGPURGE4args{m: (*[dELEGPURGE4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsDELEGRETURN4args() DELEGRETURN4args {
	if m.disc != OP_DELEGRETURN {
		panic("wrong union discriminant")
	}
	return DELEGRETURN4args{m: (*[dELEGRETURN4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsGETATTR4args() GETATTR4args {
	if m.disc != OP_GETATTR {
		panic("wrong union discriminant")
	}
	return GETATTR4args(m.b)
}

func (m NfsArgop4) AsLINK4args() LINK4args {
	if m.disc != OP_LINK {
		panic("wrong union discriminant")
	}
	return LINK4args(m.b)
}

func (m NfsArgop4) AsLOCK4args() LOCK4args {
	if m.disc != OP_LOCK {
		panic("wrong union discriminant")
	}
	return LOCK4args(m.b)
}

func (m NfsArgop4) AsLOCKT4args() LOCKT4args {
	if m.disc != OP_LOCKT {
		panic("wrong union discriminant")
	}
	return LOCKT4args(m.b)
}

func (m NfsArgop4) AsLOCKU4args() LOCKU4args {
	if m.disc != OP_LOCKU {
		panic("wrong union discriminant")
	}
	return LOCKU4args{m: (*[lOCKU4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsLOOKUP4args() LOOKUP4args {
	if m.disc != OP_LOOKUP {
		panic("wrong union discriminant")
	}
	return LOOKUP4args(m.b)
}

func (m NfsArgop4) AsNVERIFY4args() NVERIFY4args {
	if m.disc != OP_NVERIFY {
		panic("wrong union discriminant")
	}
	return NVERIFY4args(m.b)
}

func (m NfsArgop4) AsOPEN4args() OPEN4args {
	if m.disc != OP_OPEN {
		panic("wrong union discriminant")
	}
	v, _ := ReadOPEN4args(m.b)
	return v
}

func (m NfsArgop4) AsOPENATTR4args() OPENATTR4args {
	if m.disc != OP_OPENATTR {
		panic("wrong union discriminant")
	}
	return OPENATTR4args{m: (*[oPENATTR4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsOPENCONFIRM4args() OPENCONFIRM4args {
	if m.disc != OP_OPEN_CONFIRM {
		panic("wrong union discriminant")
	}
	return OPENCONFIRM4args{m: (*[oPENCONFIRM4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsOPENDOWNGRADE4args() OPENDOWNGRADE4args {
	if m.disc != OP_OPEN_DOWNGRADE {
		panic("wrong union discriminant")
	}
	return OPENDOWNGRADE4args{m: (*[oPENDOWNGRADE4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsPUTFH4args() PUTFH4args {
	if m.disc != OP_PUTFH {
		panic("wrong union discriminant")
	}
	return PUTFH4args(m.b)
}

func (m NfsArgop4) AsREAD4args() READ4args {
	if m.disc != OP_READ {
		panic("wrong union discriminant")
	}
	return READ4args{m: (*[rEAD4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsREADDIR4args() READDIR4args {
	if m.disc != OP_READDIR {
		panic("wrong union discriminant")
	}
	return READDIR4args(m.b)
}

func (m NfsArgop4) AsREMOVE4args() REMOVE4args {
	if m.disc != OP_REMOVE {
		panic("wrong union discriminant")
	}
	return REMOVE4args(m.b)
}

func (m NfsArgop4) AsRENAME4args() RENAME4args {
	if m.disc != OP_RENAME {
		panic("wrong union discriminant")
	}
	v, _ := ReadRENAME4args(m.b)
	return v
}

func (m NfsArgop4) AsRENEW4args() RENEW4args {
	if m.disc != OP_RENEW {
		panic("wrong union discriminant")
	}
	return RENEW4args{m: (*[rENEW4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsSECINFO4args() SECINFO4args {
	if m.disc != OP_SECINFO {
		panic("wrong union discriminant")
	}
	return SECINFO4args(m.b)
}

func (m NfsArgop4) AsSETATTR4args() SETATTR4args {
	if m.disc != OP_SETATTR {
		panic("wrong union discriminant")
	}
	return SETATTR4args(m.b)
}

func (m NfsArgop4) AsSETCLIENTID4args() SETCLIENTID4args {
	if m.disc != OP_SETCLIENTID {
		panic("wrong union discriminant")
	}
	v, _ := ReadSETCLIENTID4args(m.b)
	return v
}

func (m NfsArgop4) AsSETCLIENTIDCONFIRM4args() SETCLIENTIDCONFIRM4args {
	if m.disc != OP_SETCLIENTID_CONFIRM {
		panic("wrong union discriminant")
	}
	return SETCLIENTIDCONFIRM4args{m: (*[sETCLIENTIDCONFIRM4argsSize]byte)(m.b)}
}

func (m NfsArgop4) AsVERIFY4args() VERIFY4args {
	if m.disc != OP_VERIFY {
		panic("wrong union discriminant")
	}
	return VERIFY4args(m.b)
}

func (m NfsArgop4) AsWRITE4args() WRITE4args {
	if m.disc != OP_WRITE {
		panic("wrong union discriminant")
	}
	return WRITE4args(m.b)
}

func (m NfsArgop4) AsRELEASELOCKOWNER4args() RELEASELOCKOWNER4args {
	if m.disc != OP_RELEASE_LOCKOWNER {
		panic("wrong union discriminant")
	}
	return RELEASELOCKOWNER4args(m.b)
}

// -------------------------------------------------------
// ACCESS4resEntry — variable: disc(4) + ACCESS4res value
// -------------------------------------------------------

type ACCESS4resEntry []byte

func readACCESS4resEntry(b *[]byte) (ACCESS4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readACCESS4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return ACCESS4resEntry(start[:total]), true
}

func ReadACCESS4resEntry(b []byte) (ACCESS4resEntry, bool) {
	return readACCESS4resEntry(&b)
}

func (m ACCESS4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m ACCESS4resEntry) Value() ACCESS4res {
	return ACCESS4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// ACCESS4resEntryWriter writes a ACCESS4res_entry:
//
//	disc + value(disc)
type ACCESS4resEntryWriter struct {
	buf []byte
	off int
}

func StartACCESS4resEntry(buf []byte) ACCESS4resEntryWriter {
	return ACCESS4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *ACCESS4resEntryWriter) SetValue_Nfs4Ok() ACCESS4resok {
	w.buf = append(w.buf, make([]byte, 4+aCCESS4resokSize)...)
	p := (*[4 + aCCESS4resokSize]byte)(w.buf[len(w.buf)-4-aCCESS4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return ACCESS4resok{m: (*[aCCESS4resokSize]byte)(p[4:])}
}

func (w *ACCESS4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *ACCESS4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// CLOSE4resEntry — variable: disc(4) + CLOSE4res value
// -------------------------------------------------------

type CLOSE4resEntry []byte

func readCLOSE4resEntry(b *[]byte) (CLOSE4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readCLOSE4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CLOSE4resEntry(start[:total]), true
}

func ReadCLOSE4resEntry(b []byte) (CLOSE4resEntry, bool) {
	return readCLOSE4resEntry(&b)
}

func (m CLOSE4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m CLOSE4resEntry) Value() CLOSE4res {
	return CLOSE4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// CLOSE4resEntryWriter writes a CLOSE4res_entry:
//
//	disc + value(disc)
type CLOSE4resEntryWriter struct {
	buf []byte
	off int
}

func StartCLOSE4resEntry(buf []byte) CLOSE4resEntryWriter {
	return CLOSE4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *CLOSE4resEntryWriter) SetValue_Nfs4Ok() Stateid4 {
	w.buf = append(w.buf, make([]byte, 4+stateid4Size)...)
	p := (*[4 + stateid4Size]byte)(w.buf[len(w.buf)-4-stateid4Size:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return Stateid4{m: (*[stateid4Size]byte)(p[4:])}
}

func (w *CLOSE4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *CLOSE4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// COMMIT4resEntry — variable: disc(4) + COMMIT4res value
// -------------------------------------------------------

type COMMIT4resEntry []byte

func readCOMMIT4resEntry(b *[]byte) (COMMIT4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readCOMMIT4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return COMMIT4resEntry(start[:total]), true
}

func ReadCOMMIT4resEntry(b []byte) (COMMIT4resEntry, bool) {
	return readCOMMIT4resEntry(&b)
}

func (m COMMIT4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m COMMIT4resEntry) Value() COMMIT4res {
	return COMMIT4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// COMMIT4resEntryWriter writes a COMMIT4res_entry:
//
//	disc + value(disc)
type COMMIT4resEntryWriter struct {
	buf []byte
	off int
}

func StartCOMMIT4resEntry(buf []byte) COMMIT4resEntryWriter {
	return COMMIT4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *COMMIT4resEntryWriter) SetValue_Nfs4Ok() COMMIT4resok {
	w.buf = append(w.buf, make([]byte, 4+cOMMIT4resokSize)...)
	p := (*[4 + cOMMIT4resokSize]byte)(w.buf[len(w.buf)-4-cOMMIT4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return COMMIT4resok{m: (*[cOMMIT4resokSize]byte)(p[4:])}
}

func (w *COMMIT4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *COMMIT4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// CREATE4resEntry — variable: disc(4) + CREATE4res value
// -------------------------------------------------------

type CREATE4resEntry []byte

func readCREATE4resEntry(b *[]byte) (CREATE4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readCREATE4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CREATE4resEntry(start[:total]), true
}

func ReadCREATE4resEntry(b []byte) (CREATE4resEntry, bool) {
	return readCREATE4resEntry(&b)
}

func (m CREATE4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m CREATE4resEntry) Value() CREATE4res {
	return CREATE4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// CREATE4resEntryWriter writes a CREATE4res_entry:
//
//	disc + value(disc)
type CREATE4resEntryWriter struct {
	buf []byte
	off int
}

func StartCREATE4resEntry(buf []byte) CREATE4resEntryWriter {
	return CREATE4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *CREATE4resEntryWriter) SetValue_Nfs4Ok() CREATE4resokWriter {
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], NFS4_OK)
	buf := w.buf
	w.buf = nil
	return CREATE4resokWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *CREATE4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *CREATE4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CREATE4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// GETATTR4resEntry — variable: disc(4) + GETATTR4res value
// -------------------------------------------------------

type GETATTR4resEntry []byte

func readGETATTR4resEntry(b *[]byte) (GETATTR4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readGETATTR4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return GETATTR4resEntry(start[:total]), true
}

func ReadGETATTR4resEntry(b []byte) (GETATTR4resEntry, bool) {
	return readGETATTR4resEntry(&b)
}

func (m GETATTR4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m GETATTR4resEntry) Value() GETATTR4res {
	return GETATTR4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// GETATTR4resEntryWriter writes a GETATTR4res_entry:
//
//	disc + value(disc)
type GETATTR4resEntryWriter struct {
	buf []byte
	off int
}

func StartGETATTR4resEntry(buf []byte) GETATTR4resEntryWriter {
	return GETATTR4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *GETATTR4resEntryWriter) SetValue_Nfs4Ok() GETATTR4resokWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, NFS4_OK)
	child := StartGETATTR4resok(w.buf)
	w.buf = nil
	return child
}

func (w *GETATTR4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *GETATTR4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *GETATTR4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// GETFH4resEntry — variable: disc(4) + GETFH4res value
// -------------------------------------------------------

type GETFH4resEntry []byte

func readGETFH4resEntry(b *[]byte) (GETFH4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readGETFH4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return GETFH4resEntry(start[:total]), true
}

func ReadGETFH4resEntry(b []byte) (GETFH4resEntry, bool) {
	return readGETFH4resEntry(&b)
}

func (m GETFH4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m GETFH4resEntry) Value() GETFH4res {
	return GETFH4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// GETFH4resEntryWriter writes a GETFH4res_entry:
//
//	disc + value(disc)
type GETFH4resEntryWriter struct {
	buf []byte
	off int
}

func StartGETFH4resEntry(buf []byte) GETFH4resEntryWriter {
	return GETFH4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *GETFH4resEntryWriter) SetValue_Nfs4Ok() GETFH4resokWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, NFS4_OK)
	child := StartGETFH4resok(w.buf)
	w.buf = nil
	return child
}

func (w *GETFH4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *GETFH4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *GETFH4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LINK4resEntry — variable: disc(4) + LINK4res value
// -------------------------------------------------------

type LINK4resEntry []byte

func readLINK4resEntry(b *[]byte) (LINK4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readLINK4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LINK4resEntry(start[:total]), true
}

func ReadLINK4resEntry(b []byte) (LINK4resEntry, bool) {
	return readLINK4resEntry(&b)
}

func (m LINK4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m LINK4resEntry) Value() LINK4res {
	return LINK4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// LINK4resEntryWriter writes a LINK4res_entry:
//
//	disc + value(disc)
type LINK4resEntryWriter struct {
	buf []byte
	off int
}

func StartLINK4resEntry(buf []byte) LINK4resEntryWriter {
	return LINK4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *LINK4resEntryWriter) SetValue_Nfs4Ok() LINK4resok {
	w.buf = append(w.buf, make([]byte, 4+lINK4resokSize)...)
	p := (*[4 + lINK4resokSize]byte)(w.buf[len(w.buf)-4-lINK4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return LINK4resok{m: (*[lINK4resokSize]byte)(p[4:])}
}

func (w *LINK4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *LINK4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// LOCK4resEntry — variable: disc(4) + LOCK4res value
// -------------------------------------------------------

type LOCK4resEntry []byte

func readLOCK4resEntry(b *[]byte) (LOCK4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readLOCK4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOCK4resEntry(start[:total]), true
}

func ReadLOCK4resEntry(b []byte) (LOCK4resEntry, bool) {
	return readLOCK4resEntry(&b)
}

func (m LOCK4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m LOCK4resEntry) Value() LOCK4res {
	return LOCK4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// LOCK4resEntryWriter writes a LOCK4res_entry:
//
//	disc + value(disc)
type LOCK4resEntryWriter struct {
	buf []byte
	off int
}

func StartLOCK4resEntry(buf []byte) LOCK4resEntryWriter {
	return LOCK4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *LOCK4resEntryWriter) SetValue_Nfs4Ok() LOCK4resok {
	w.buf = append(w.buf, make([]byte, 4+lOCK4resokSize)...)
	p := (*[4 + lOCK4resokSize]byte)(w.buf[len(w.buf)-4-lOCK4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return LOCK4resok{m: (*[lOCK4resokSize]byte)(p[4:])}
}

func (w *LOCK4resEntryWriter) SetValue_Nfs4errDenied() LOCK4deniedWriter {
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], NFS4ERR_DENIED)
	buf := w.buf
	w.buf = nil
	return LOCK4deniedWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *LOCK4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *LOCK4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LOCK4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LOCKT4resEntry — variable: disc(4) + LOCKT4res value
// -------------------------------------------------------

type LOCKT4resEntry []byte

func readLOCKT4resEntry(b *[]byte) (LOCKT4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readLOCKT4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOCKT4resEntry(start[:total]), true
}

func ReadLOCKT4resEntry(b []byte) (LOCKT4resEntry, bool) {
	return readLOCKT4resEntry(&b)
}

func (m LOCKT4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m LOCKT4resEntry) Value() LOCKT4res {
	return LOCKT4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// LOCKT4resEntryWriter writes a LOCKT4res_entry:
//
//	disc + value(disc)
type LOCKT4resEntryWriter struct {
	buf []byte
	off int
}

func StartLOCKT4resEntry(buf []byte) LOCKT4resEntryWriter {
	return LOCKT4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *LOCKT4resEntryWriter) SetValue_Nfs4errDenied() LOCK4deniedWriter {
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], NFS4ERR_DENIED)
	buf := w.buf
	w.buf = nil
	return LOCK4deniedWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *LOCKT4resEntryWriter) SetValue_Nfs4Ok() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, NFS4_OK)
}

func (w *LOCKT4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *LOCKT4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *LOCKT4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// LOCKU4resEntry — variable: disc(4) + LOCKU4res value
// -------------------------------------------------------

type LOCKU4resEntry []byte

func readLOCKU4resEntry(b *[]byte) (LOCKU4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readLOCKU4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return LOCKU4resEntry(start[:total]), true
}

func ReadLOCKU4resEntry(b []byte) (LOCKU4resEntry, bool) {
	return readLOCKU4resEntry(&b)
}

func (m LOCKU4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m LOCKU4resEntry) Value() LOCKU4res {
	return LOCKU4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// LOCKU4resEntryWriter writes a LOCKU4res_entry:
//
//	disc + value(disc)
type LOCKU4resEntryWriter struct {
	buf []byte
	off int
}

func StartLOCKU4resEntry(buf []byte) LOCKU4resEntryWriter {
	return LOCKU4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *LOCKU4resEntryWriter) SetValue_Nfs4Ok() Stateid4 {
	w.buf = append(w.buf, make([]byte, 4+stateid4Size)...)
	p := (*[4 + stateid4Size]byte)(w.buf[len(w.buf)-4-stateid4Size:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return Stateid4{m: (*[stateid4Size]byte)(p[4:])}
}

func (w *LOCKU4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *LOCKU4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// OPEN4resEntry — variable: disc(4) + OPEN4res value
// -------------------------------------------------------

type OPEN4resEntry []byte

func readOPEN4resEntry(b *[]byte) (OPEN4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readOPEN4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return OPEN4resEntry(start[:total]), true
}

func ReadOPEN4resEntry(b []byte) (OPEN4resEntry, bool) {
	return readOPEN4resEntry(&b)
}

func (m OPEN4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m OPEN4resEntry) Value() OPEN4res {
	return OPEN4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// OPEN4resEntryWriter writes a OPEN4res_entry:
//
//	disc + value(disc)
type OPEN4resEntryWriter struct {
	buf []byte
	off int
}

func StartOPEN4resEntry(buf []byte) OPEN4resEntryWriter {
	return OPEN4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *OPEN4resEntryWriter) SetValue_Nfs4Ok() OPEN4resokWriter {
	w.buf = append(w.buf, make([]byte, 4+40)...)
	off := len(w.buf) - 4 - 40
	binary.BigEndian.PutUint32((*[4 + 40]byte)(w.buf[off:])[:4], NFS4_OK)
	buf := w.buf
	w.buf = nil
	return OPEN4resokWriter{buf: buf, header: (*[40]byte)(buf[off+4:])}
}

func (w *OPEN4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *OPEN4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *OPEN4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// OPENCONFIRM4resEntry — variable: disc(4) + OPEN_CONFIRM4res value
// -------------------------------------------------------

type OPENCONFIRM4resEntry []byte

func readOPENCONFIRM4resEntry(b *[]byte) (OPENCONFIRM4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readOPENCONFIRM4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return OPENCONFIRM4resEntry(start[:total]), true
}

func ReadOPENCONFIRM4resEntry(b []byte) (OPENCONFIRM4resEntry, bool) {
	return readOPENCONFIRM4resEntry(&b)
}

func (m OPENCONFIRM4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m OPENCONFIRM4resEntry) Value() OPENCONFIRM4res {
	return OPENCONFIRM4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// OPENCONFIRM4resEntryWriter writes a OPEN_CONFIRM4res_entry:
//
//	disc + value(disc)
type OPENCONFIRM4resEntryWriter struct {
	buf []byte
	off int
}

func StartOPENCONFIRM4resEntry(buf []byte) OPENCONFIRM4resEntryWriter {
	return OPENCONFIRM4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *OPENCONFIRM4resEntryWriter) SetValue_Nfs4Ok() OPENCONFIRM4resok {
	w.buf = append(w.buf, make([]byte, 4+oPENCONFIRM4resokSize)...)
	p := (*[4 + oPENCONFIRM4resokSize]byte)(w.buf[len(w.buf)-4-oPENCONFIRM4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return OPENCONFIRM4resok{m: (*[oPENCONFIRM4resokSize]byte)(p[4:])}
}

func (w *OPENCONFIRM4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *OPENCONFIRM4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// OPENDOWNGRADE4resEntry — variable: disc(4) + OPEN_DOWNGRADE4res value
// -------------------------------------------------------

type OPENDOWNGRADE4resEntry []byte

func readOPENDOWNGRADE4resEntry(b *[]byte) (OPENDOWNGRADE4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readOPENDOWNGRADE4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return OPENDOWNGRADE4resEntry(start[:total]), true
}

func ReadOPENDOWNGRADE4resEntry(b []byte) (OPENDOWNGRADE4resEntry, bool) {
	return readOPENDOWNGRADE4resEntry(&b)
}

func (m OPENDOWNGRADE4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m OPENDOWNGRADE4resEntry) Value() OPENDOWNGRADE4res {
	return OPENDOWNGRADE4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// OPENDOWNGRADE4resEntryWriter writes a OPEN_DOWNGRADE4res_entry:
//
//	disc + value(disc)
type OPENDOWNGRADE4resEntryWriter struct {
	buf []byte
	off int
}

func StartOPENDOWNGRADE4resEntry(buf []byte) OPENDOWNGRADE4resEntryWriter {
	return OPENDOWNGRADE4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *OPENDOWNGRADE4resEntryWriter) SetValue_Nfs4Ok() OPENDOWNGRADE4resok {
	w.buf = append(w.buf, make([]byte, 4+oPENDOWNGRADE4resokSize)...)
	p := (*[4 + oPENDOWNGRADE4resokSize]byte)(w.buf[len(w.buf)-4-oPENDOWNGRADE4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return OPENDOWNGRADE4resok{m: (*[oPENDOWNGRADE4resokSize]byte)(p[4:])}
}

func (w *OPENDOWNGRADE4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *OPENDOWNGRADE4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// READ4resEntry — variable: disc(4) + READ4res value
// -------------------------------------------------------

type READ4resEntry []byte

func readREAD4resEntry(b *[]byte) (READ4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readREAD4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return READ4resEntry(start[:total]), true
}

func ReadREAD4resEntry(b []byte) (READ4resEntry, bool) {
	return readREAD4resEntry(&b)
}

func (m READ4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m READ4resEntry) Value() READ4res {
	return READ4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// READ4resEntryWriter writes a READ4res_entry:
//
//	disc + value(disc)
type READ4resEntryWriter struct {
	buf []byte
	off int
}

func StartREAD4resEntry(buf []byte) READ4resEntryWriter {
	return READ4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *READ4resEntryWriter) SetValue_Nfs4Ok() READ4resokWriter {
	w.buf = append(w.buf, make([]byte, 4+8)...)
	off := len(w.buf) - 4 - 8
	binary.BigEndian.PutUint32((*[4 + 8]byte)(w.buf[off:])[:4], NFS4_OK)
	buf := w.buf
	w.buf = nil
	return READ4resokWriter{buf: buf, off: off + 4}
}

func (w *READ4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *READ4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *READ4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// READDIR4resEntry — variable: disc(4) + READDIR4res value
// -------------------------------------------------------

type READDIR4resEntry []byte

func readREADDIR4resEntry(b *[]byte) (READDIR4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readREADDIR4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return READDIR4resEntry(start[:total]), true
}

func ReadREADDIR4resEntry(b []byte) (READDIR4resEntry, bool) {
	return readREADDIR4resEntry(&b)
}

func (m READDIR4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m READDIR4resEntry) Value() READDIR4res {
	return READDIR4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// READDIR4resEntryWriter writes a READDIR4res_entry:
//
//	disc + value(disc)
type READDIR4resEntryWriter struct {
	buf []byte
	off int
}

func StartREADDIR4resEntry(buf []byte) READDIR4resEntryWriter {
	return READDIR4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *READDIR4resEntryWriter) SetValue_Nfs4Ok() READDIR4resokWriter {
	w.buf = append(w.buf, make([]byte, 4+8)...)
	off := len(w.buf) - 4 - 8
	binary.BigEndian.PutUint32((*[4 + 8]byte)(w.buf[off:])[:4], NFS4_OK)
	buf := w.buf
	w.buf = nil
	return READDIR4resokWriter{buf: buf, header: (*[8]byte)(buf[off+4:])}
}

func (w *READDIR4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *READDIR4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *READDIR4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// READLINK4resEntry — variable: disc(4) + READLINK4res value
// -------------------------------------------------------

type READLINK4resEntry []byte

func readREADLINK4resEntry(b *[]byte) (READLINK4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readREADLINK4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return READLINK4resEntry(start[:total]), true
}

func ReadREADLINK4resEntry(b []byte) (READLINK4resEntry, bool) {
	return readREADLINK4resEntry(&b)
}

func (m READLINK4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m READLINK4resEntry) Value() READLINK4res {
	return READLINK4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// READLINK4resEntryWriter writes a READLINK4res_entry:
//
//	disc + value(disc)
type READLINK4resEntryWriter struct {
	buf []byte
	off int
}

func StartREADLINK4resEntry(buf []byte) READLINK4resEntryWriter {
	return READLINK4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *READLINK4resEntryWriter) SetValue_Nfs4Ok() READLINK4resokWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, NFS4_OK)
	child := StartREADLINK4resok(w.buf)
	w.buf = nil
	return child
}

func (w *READLINK4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *READLINK4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *READLINK4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// REMOVE4resEntry — variable: disc(4) + REMOVE4res value
// -------------------------------------------------------

type REMOVE4resEntry []byte

func readREMOVE4resEntry(b *[]byte) (REMOVE4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readREMOVE4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return REMOVE4resEntry(start[:total]), true
}

func ReadREMOVE4resEntry(b []byte) (REMOVE4resEntry, bool) {
	return readREMOVE4resEntry(&b)
}

func (m REMOVE4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m REMOVE4resEntry) Value() REMOVE4res {
	return REMOVE4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// REMOVE4resEntryWriter writes a REMOVE4res_entry:
//
//	disc + value(disc)
type REMOVE4resEntryWriter struct {
	buf []byte
	off int
}

func StartREMOVE4resEntry(buf []byte) REMOVE4resEntryWriter {
	return REMOVE4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *REMOVE4resEntryWriter) SetValue_Nfs4Ok() REMOVE4resok {
	w.buf = append(w.buf, make([]byte, 4+rEMOVE4resokSize)...)
	p := (*[4 + rEMOVE4resokSize]byte)(w.buf[len(w.buf)-4-rEMOVE4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return REMOVE4resok{m: (*[rEMOVE4resokSize]byte)(p[4:])}
}

func (w *REMOVE4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *REMOVE4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// RENAME4resEntry — variable: disc(4) + RENAME4res value
// -------------------------------------------------------

type RENAME4resEntry []byte

func readRENAME4resEntry(b *[]byte) (RENAME4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readRENAME4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return RENAME4resEntry(start[:total]), true
}

func ReadRENAME4resEntry(b []byte) (RENAME4resEntry, bool) {
	return readRENAME4resEntry(&b)
}

func (m RENAME4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m RENAME4resEntry) Value() RENAME4res {
	return RENAME4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// RENAME4resEntryWriter writes a RENAME4res_entry:
//
//	disc + value(disc)
type RENAME4resEntryWriter struct {
	buf []byte
	off int
}

func StartRENAME4resEntry(buf []byte) RENAME4resEntryWriter {
	return RENAME4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *RENAME4resEntryWriter) SetValue_Nfs4Ok() RENAME4resok {
	w.buf = append(w.buf, make([]byte, 4+rENAME4resokSize)...)
	p := (*[4 + rENAME4resokSize]byte)(w.buf[len(w.buf)-4-rENAME4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return RENAME4resok{m: (*[rENAME4resokSize]byte)(p[4:])}
}

func (w *RENAME4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *RENAME4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// SECINFO4resEntry — variable: disc(4) + SECINFO4res value
// -------------------------------------------------------

type SECINFO4resEntry []byte

func readSECINFO4resEntry(b *[]byte) (SECINFO4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readSECINFO4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return SECINFO4resEntry(start[:total]), true
}

func ReadSECINFO4resEntry(b []byte) (SECINFO4resEntry, bool) {
	return readSECINFO4resEntry(&b)
}

func (m SECINFO4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m SECINFO4resEntry) Value() SECINFO4res {
	return SECINFO4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// SECINFO4resEntryWriter writes a SECINFO4res_entry:
//
//	disc + value(disc)
type SECINFO4resEntryWriter struct {
	buf []byte
	off int
}

func StartSECINFO4resEntry(buf []byte) SECINFO4resEntryWriter {
	return SECINFO4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *SECINFO4resEntryWriter) SetValue_Nfs4Ok() SECINFO4resokWriter {
	w.buf = append(w.buf, make([]byte, 4+4)...)
	off := len(w.buf) - 4 - 4
	binary.BigEndian.PutUint32((*[4 + 4]byte)(w.buf[off:])[:4], NFS4_OK)
	buf := w.buf
	w.buf = nil
	return SECINFO4resokWriter{buf: buf, off: off + 4}
}

func (w *SECINFO4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *SECINFO4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SECINFO4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// SETCLIENTID4resEntry — variable: disc(4) + SETCLIENTID4res value
// -------------------------------------------------------

type SETCLIENTID4resEntry []byte

func readSETCLIENTID4resEntry(b *[]byte) (SETCLIENTID4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readSETCLIENTID4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return SETCLIENTID4resEntry(start[:total]), true
}

func ReadSETCLIENTID4resEntry(b []byte) (SETCLIENTID4resEntry, bool) {
	return readSETCLIENTID4resEntry(&b)
}

func (m SETCLIENTID4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m SETCLIENTID4resEntry) Value() SETCLIENTID4res {
	return SETCLIENTID4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// SETCLIENTID4resEntryWriter writes a SETCLIENTID4res_entry:
//
//	disc + value(disc)
type SETCLIENTID4resEntryWriter struct {
	buf []byte
	off int
}

func StartSETCLIENTID4resEntry(buf []byte) SETCLIENTID4resEntryWriter {
	return SETCLIENTID4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *SETCLIENTID4resEntryWriter) SetValue_Nfs4Ok() SETCLIENTID4resok {
	w.buf = append(w.buf, make([]byte, 4+sETCLIENTID4resokSize)...)
	p := (*[4 + sETCLIENTID4resokSize]byte)(w.buf[len(w.buf)-4-sETCLIENTID4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return SETCLIENTID4resok{m: (*[sETCLIENTID4resokSize]byte)(p[4:])}
}

func (w *SETCLIENTID4resEntryWriter) SetValue_Nfs4errClidInuse() Clientaddr4Writer {
	w.buf = binary.BigEndian.AppendUint32(w.buf, NFS4ERR_CLID_INUSE)
	child := StartClientaddr4(w.buf)
	w.buf = nil
	return child
}

func (w *SETCLIENTID4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *SETCLIENTID4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *SETCLIENTID4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// WRITE4resEntry — variable: disc(4) + WRITE4res value
// -------------------------------------------------------

type WRITE4resEntry []byte

func readWRITE4resEntry(b *[]byte) (WRITE4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readWRITE4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return WRITE4resEntry(start[:total]), true
}

func ReadWRITE4resEntry(b []byte) (WRITE4resEntry, bool) {
	return readWRITE4resEntry(&b)
}

func (m WRITE4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m WRITE4resEntry) Value() WRITE4res {
	return WRITE4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// WRITE4resEntryWriter writes a WRITE4res_entry:
//
//	disc + value(disc)
type WRITE4resEntryWriter struct {
	buf []byte
	off int
}

func StartWRITE4resEntry(buf []byte) WRITE4resEntryWriter {
	return WRITE4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *WRITE4resEntryWriter) SetValue_Nfs4Ok() WRITE4resok {
	w.buf = append(w.buf, make([]byte, 4+wRITE4resokSize)...)
	p := (*[4 + wRITE4resokSize]byte)(w.buf[len(w.buf)-4-wRITE4resokSize:])
	binary.BigEndian.PutUint32(p[:4], NFS4_OK)
	return WRITE4resok{m: (*[wRITE4resokSize]byte)(p[4:])}
}

func (w *WRITE4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *WRITE4resEntryWriter) Finish() []byte {
	return w.buf
}

// -------------------------------------------------------
// NfsResop4 — union on nfs_opnum4 (external discriminant)
// -------------------------------------------------------

type NfsResop4 struct {
	b    []byte
	disc uint32
}

func readNfsResop4(b *[]byte, nfsOpnum4 uint32) (NfsResop4, bool) {
	switch nfsOpnum4 {
	case OP_ACCESS:
		r, ok := readACCESS4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_CLOSE:
		r, ok := readCLOSE4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_COMMIT:
		r, ok := readCOMMIT4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_CREATE:
		r, ok := readCREATE4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_DELEGPURGE:
		r, ok := readDELEGPURGE4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_DELEGRETURN:
		r, ok := readDELEGRETURN4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_GETATTR:
		r, ok := readGETATTR4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_GETFH:
		r, ok := readGETFH4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LINK:
		r, ok := readLINK4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOCK:
		r, ok := readLOCK4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOCKT:
		r, ok := readLOCKT4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOCKU:
		r, ok := readLOCKU4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_LOOKUP:
		r, ok := readLOOKUP4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_LOOKUPP:
		r, ok := readLOOKUPP4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_NVERIFY:
		r, ok := readNVERIFY4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_OPEN:
		r, ok := readOPEN4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_OPENATTR:
		r, ok := readOPENATTR4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_OPEN_CONFIRM:
		r, ok := readOPENCONFIRM4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_OPEN_DOWNGRADE:
		r, ok := readOPENDOWNGRADE4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_PUTFH:
		r, ok := readPUTFH4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_PUTPUBFH:
		r, ok := readPUTPUBFH4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_PUTROOTFH:
		r, ok := readPUTROOTFH4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_READ:
		r, ok := readREAD4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_READDIR:
		r, ok := readREADDIR4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_READLINK:
		r, ok := readREADLINK4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_REMOVE:
		r, ok := readREMOVE4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_RENAME:
		r, ok := readRENAME4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_RENEW:
		r, ok := readRENEW4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_RESTOREFH:
		r, ok := readRESTOREFH4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_SAVEFH:
		r, ok := readSAVEFH4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_SECINFO:
		r, ok := readSECINFO4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_SETATTR:
		r, ok := readSETATTR4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_SETCLIENTID:
		r, ok := readSETCLIENTID4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_SETCLIENTID_CONFIRM:
		r, ok := readSETCLIENTIDCONFIRM4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_VERIFY:
		r, ok := readVERIFY4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_WRITE:
		r, ok := readWRITE4resEntry(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: []byte(r), disc: nfsOpnum4}, true
	case OP_RELEASE_LOCKOWNER:
		r, ok := readRELEASELOCKOWNER4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	case OP_ILLEGAL:
		r, ok := readILLEGAL4res(b)
		if !ok {
			return NfsResop4{}, false
		}
		return NfsResop4{b: r.m[:], disc: nfsOpnum4}, true
	default:
		return NfsResop4{}, false
	}
}

func (m NfsResop4) AsACCESS4resEntry() ACCESS4resEntry {
	if m.disc != OP_ACCESS {
		panic("wrong union discriminant")
	}
	return ACCESS4resEntry(m.b)
}

func (m NfsResop4) AsCLOSE4resEntry() CLOSE4resEntry {
	if m.disc != OP_CLOSE {
		panic("wrong union discriminant")
	}
	return CLOSE4resEntry(m.b)
}

func (m NfsResop4) AsCOMMIT4resEntry() COMMIT4resEntry {
	if m.disc != OP_COMMIT {
		panic("wrong union discriminant")
	}
	return COMMIT4resEntry(m.b)
}

func (m NfsResop4) AsCREATE4resEntry() CREATE4resEntry {
	if m.disc != OP_CREATE {
		panic("wrong union discriminant")
	}
	return CREATE4resEntry(m.b)
}

func (m NfsResop4) AsDELEGPURGE4res() DELEGPURGE4res {
	if m.disc != OP_DELEGPURGE {
		panic("wrong union discriminant")
	}
	return DELEGPURGE4res{m: (*[dELEGPURGE4resSize]byte)(m.b)}
}

func (m NfsResop4) AsDELEGRETURN4res() DELEGRETURN4res {
	if m.disc != OP_DELEGRETURN {
		panic("wrong union discriminant")
	}
	return DELEGRETURN4res{m: (*[dELEGRETURN4resSize]byte)(m.b)}
}

func (m NfsResop4) AsGETATTR4resEntry() GETATTR4resEntry {
	if m.disc != OP_GETATTR {
		panic("wrong union discriminant")
	}
	return GETATTR4resEntry(m.b)
}

func (m NfsResop4) AsGETFH4resEntry() GETFH4resEntry {
	if m.disc != OP_GETFH {
		panic("wrong union discriminant")
	}
	return GETFH4resEntry(m.b)
}

func (m NfsResop4) AsLINK4resEntry() LINK4resEntry {
	if m.disc != OP_LINK {
		panic("wrong union discriminant")
	}
	return LINK4resEntry(m.b)
}

func (m NfsResop4) AsLOCK4resEntry() LOCK4resEntry {
	if m.disc != OP_LOCK {
		panic("wrong union discriminant")
	}
	return LOCK4resEntry(m.b)
}

func (m NfsResop4) AsLOCKT4resEntry() LOCKT4resEntry {
	if m.disc != OP_LOCKT {
		panic("wrong union discriminant")
	}
	return LOCKT4resEntry(m.b)
}

func (m NfsResop4) AsLOCKU4resEntry() LOCKU4resEntry {
	if m.disc != OP_LOCKU {
		panic("wrong union discriminant")
	}
	return LOCKU4resEntry(m.b)
}

func (m NfsResop4) AsLOOKUP4res() LOOKUP4res {
	if m.disc != OP_LOOKUP {
		panic("wrong union discriminant")
	}
	return LOOKUP4res{m: (*[lOOKUP4resSize]byte)(m.b)}
}

func (m NfsResop4) AsLOOKUPP4res() LOOKUPP4res {
	if m.disc != OP_LOOKUPP {
		panic("wrong union discriminant")
	}
	return LOOKUPP4res{m: (*[lOOKUPP4resSize]byte)(m.b)}
}

func (m NfsResop4) AsNVERIFY4res() NVERIFY4res {
	if m.disc != OP_NVERIFY {
		panic("wrong union discriminant")
	}
	return NVERIFY4res{m: (*[nVERIFY4resSize]byte)(m.b)}
}

func (m NfsResop4) AsOPEN4resEntry() OPEN4resEntry {
	if m.disc != OP_OPEN {
		panic("wrong union discriminant")
	}
	return OPEN4resEntry(m.b)
}

func (m NfsResop4) AsOPENATTR4res() OPENATTR4res {
	if m.disc != OP_OPENATTR {
		panic("wrong union discriminant")
	}
	return OPENATTR4res{m: (*[oPENATTR4resSize]byte)(m.b)}
}

func (m NfsResop4) AsOPENCONFIRM4resEntry() OPENCONFIRM4resEntry {
	if m.disc != OP_OPEN_CONFIRM {
		panic("wrong union discriminant")
	}
	return OPENCONFIRM4resEntry(m.b)
}

func (m NfsResop4) AsOPENDOWNGRADE4resEntry() OPENDOWNGRADE4resEntry {
	if m.disc != OP_OPEN_DOWNGRADE {
		panic("wrong union discriminant")
	}
	return OPENDOWNGRADE4resEntry(m.b)
}

func (m NfsResop4) AsPUTFH4res() PUTFH4res {
	if m.disc != OP_PUTFH {
		panic("wrong union discriminant")
	}
	return PUTFH4res{m: (*[pUTFH4resSize]byte)(m.b)}
}

func (m NfsResop4) AsPUTPUBFH4res() PUTPUBFH4res {
	if m.disc != OP_PUTPUBFH {
		panic("wrong union discriminant")
	}
	return PUTPUBFH4res{m: (*[pUTPUBFH4resSize]byte)(m.b)}
}

func (m NfsResop4) AsPUTROOTFH4res() PUTROOTFH4res {
	if m.disc != OP_PUTROOTFH {
		panic("wrong union discriminant")
	}
	return PUTROOTFH4res{m: (*[pUTROOTFH4resSize]byte)(m.b)}
}

func (m NfsResop4) AsREAD4resEntry() READ4resEntry {
	if m.disc != OP_READ {
		panic("wrong union discriminant")
	}
	return READ4resEntry(m.b)
}

func (m NfsResop4) AsREADDIR4resEntry() READDIR4resEntry {
	if m.disc != OP_READDIR {
		panic("wrong union discriminant")
	}
	return READDIR4resEntry(m.b)
}

func (m NfsResop4) AsREADLINK4resEntry() READLINK4resEntry {
	if m.disc != OP_READLINK {
		panic("wrong union discriminant")
	}
	return READLINK4resEntry(m.b)
}

func (m NfsResop4) AsREMOVE4resEntry() REMOVE4resEntry {
	if m.disc != OP_REMOVE {
		panic("wrong union discriminant")
	}
	return REMOVE4resEntry(m.b)
}

func (m NfsResop4) AsRENAME4resEntry() RENAME4resEntry {
	if m.disc != OP_RENAME {
		panic("wrong union discriminant")
	}
	return RENAME4resEntry(m.b)
}

func (m NfsResop4) AsRENEW4res() RENEW4res {
	if m.disc != OP_RENEW {
		panic("wrong union discriminant")
	}
	return RENEW4res{m: (*[rENEW4resSize]byte)(m.b)}
}

func (m NfsResop4) AsRESTOREFH4res() RESTOREFH4res {
	if m.disc != OP_RESTOREFH {
		panic("wrong union discriminant")
	}
	return RESTOREFH4res{m: (*[rESTOREFH4resSize]byte)(m.b)}
}

func (m NfsResop4) AsSAVEFH4res() SAVEFH4res {
	if m.disc != OP_SAVEFH {
		panic("wrong union discriminant")
	}
	return SAVEFH4res{m: (*[sAVEFH4resSize]byte)(m.b)}
}

func (m NfsResop4) AsSECINFO4resEntry() SECINFO4resEntry {
	if m.disc != OP_SECINFO {
		panic("wrong union discriminant")
	}
	return SECINFO4resEntry(m.b)
}

func (m NfsResop4) AsSETATTR4res() SETATTR4res {
	if m.disc != OP_SETATTR {
		panic("wrong union discriminant")
	}
	return SETATTR4res(m.b)
}

func (m NfsResop4) AsSETCLIENTID4resEntry() SETCLIENTID4resEntry {
	if m.disc != OP_SETCLIENTID {
		panic("wrong union discriminant")
	}
	return SETCLIENTID4resEntry(m.b)
}

func (m NfsResop4) AsSETCLIENTIDCONFIRM4res() SETCLIENTIDCONFIRM4res {
	if m.disc != OP_SETCLIENTID_CONFIRM {
		panic("wrong union discriminant")
	}
	return SETCLIENTIDCONFIRM4res{m: (*[sETCLIENTIDCONFIRM4resSize]byte)(m.b)}
}

func (m NfsResop4) AsVERIFY4res() VERIFY4res {
	if m.disc != OP_VERIFY {
		panic("wrong union discriminant")
	}
	return VERIFY4res{m: (*[vERIFY4resSize]byte)(m.b)}
}

func (m NfsResop4) AsWRITE4resEntry() WRITE4resEntry {
	if m.disc != OP_WRITE {
		panic("wrong union discriminant")
	}
	return WRITE4resEntry(m.b)
}

func (m NfsResop4) AsRELEASELOCKOWNER4res() RELEASELOCKOWNER4res {
	if m.disc != OP_RELEASE_LOCKOWNER {
		panic("wrong union discriminant")
	}
	return RELEASELOCKOWNER4res{m: (*[rELEASELOCKOWNER4resSize]byte)(m.b)}
}

func (m NfsResop4) AsILLEGAL4res() ILLEGAL4res {
	if m.disc != OP_ILLEGAL {
		panic("wrong union discriminant")
	}
	return ILLEGAL4res{m: (*[iLLEGAL4resSize]byte)(m.b)}
}

// -------------------------------------------------------
// NfsArgop4Entry — variable: disc(4) + nfs_argop4 value
// -------------------------------------------------------

type NfsArgop4Entry []byte

func readNfsArgop4Entry(b *[]byte) (NfsArgop4Entry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readNfsArgop4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return NfsArgop4Entry(start[:total]), true
}

func ReadNfsArgop4Entry(b []byte) (NfsArgop4Entry, bool) {
	return readNfsArgop4Entry(&b)
}

func (m NfsArgop4Entry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m NfsArgop4Entry) Value() NfsArgop4 {
	return NfsArgop4{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// NfsArgop4EntryWriter writes a nfs_argop4_entry:
//
//	disc + value(disc)
type NfsArgop4EntryWriter struct {
	buf []byte
	off int
}

func StartNfsArgop4Entry(buf []byte) NfsArgop4EntryWriter {
	return NfsArgop4EntryWriter{buf: buf, off: len(buf)}
}

func (w *NfsArgop4EntryWriter) SetValue_Access() ACCESS4args {
	w.buf = append(w.buf, make([]byte, 4+aCCESS4argsSize)...)
	p := (*[4 + aCCESS4argsSize]byte)(w.buf[len(w.buf)-4-aCCESS4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_ACCESS)
	return ACCESS4args{m: (*[aCCESS4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Close() CLOSE4args {
	w.buf = append(w.buf, make([]byte, 4+cLOSE4argsSize)...)
	p := (*[4 + cLOSE4argsSize]byte)(w.buf[len(w.buf)-4-cLOSE4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_CLOSE)
	return CLOSE4args{m: (*[cLOSE4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Commit() COMMIT4args {
	w.buf = append(w.buf, make([]byte, 4+cOMMIT4argsSize)...)
	p := (*[4 + cOMMIT4argsSize]byte)(w.buf[len(w.buf)-4-cOMMIT4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_COMMIT)
	return COMMIT4args{m: (*[cOMMIT4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Create() CREATE4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+4)...)
	off := len(w.buf) - 4 - 4
	binary.BigEndian.PutUint32((*[4 + 4]byte)(w.buf[off:])[:4], OP_CREATE)
	buf := w.buf
	w.buf = nil
	return CREATE4argsWriter{buf: buf, header: (*[4]byte)(buf[off+4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Delegpurge() DELEGPURGE4args {
	w.buf = append(w.buf, make([]byte, 4+dELEGPURGE4argsSize)...)
	p := (*[4 + dELEGPURGE4argsSize]byte)(w.buf[len(w.buf)-4-dELEGPURGE4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGPURGE)
	return DELEGPURGE4args{m: (*[dELEGPURGE4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Delegreturn() DELEGRETURN4args {
	w.buf = append(w.buf, make([]byte, 4+dELEGRETURN4argsSize)...)
	p := (*[4 + dELEGRETURN4argsSize]byte)(w.buf[len(w.buf)-4-dELEGRETURN4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGRETURN)
	return DELEGRETURN4args{m: (*[dELEGRETURN4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Getattr() GETATTR4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETATTR)
	child := StartGETATTR4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Getfh() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETFH)
}

func (w *NfsArgop4EntryWriter) SetValue_Link() LINK4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LINK)
	child := StartLINK4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Lock() LOCK4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+28)...)
	off := len(w.buf) - 4 - 28
	binary.BigEndian.PutUint32((*[4 + 28]byte)(w.buf[off:])[:4], OP_LOCK)
	buf := w.buf
	w.buf = nil
	return LOCK4argsWriter{buf: buf, header: (*[28]byte)(buf[off+4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Lockt() LOCKT4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], OP_LOCKT)
	buf := w.buf
	w.buf = nil
	return LOCKT4argsWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Locku() LOCKU4args {
	w.buf = append(w.buf, make([]byte, 4+lOCKU4argsSize)...)
	p := (*[4 + lOCKU4argsSize]byte)(w.buf[len(w.buf)-4-lOCKU4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_LOCKU)
	return LOCKU4args{m: (*[lOCKU4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Lookup() LOOKUP4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOOKUP)
	child := StartLOOKUP4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Lookupp() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOOKUPP)
}

func (w *NfsArgop4EntryWriter) SetValue_Nverify() NVERIFY4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_NVERIFY)
	child := StartNVERIFY4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Open() OPEN4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+12)...)
	off := len(w.buf) - 4 - 12
	binary.BigEndian.PutUint32((*[4 + 12]byte)(w.buf[off:])[:4], OP_OPEN)
	buf := w.buf
	w.buf = nil
	return OPEN4argsWriter{buf: buf, header: (*[12]byte)(buf[off+4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Openattr() OPENATTR4args {
	w.buf = append(w.buf, make([]byte, 4+oPENATTR4argsSize)...)
	p := (*[4 + oPENATTR4argsSize]byte)(w.buf[len(w.buf)-4-oPENATTR4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPENATTR)
	return OPENATTR4args{m: (*[oPENATTR4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_OpenConfirm() OPENCONFIRM4args {
	w.buf = append(w.buf, make([]byte, 4+oPENCONFIRM4argsSize)...)
	p := (*[4 + oPENCONFIRM4argsSize]byte)(w.buf[len(w.buf)-4-oPENCONFIRM4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPEN_CONFIRM)
	return OPENCONFIRM4args{m: (*[oPENCONFIRM4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_OpenDowngrade() OPENDOWNGRADE4args {
	w.buf = append(w.buf, make([]byte, 4+oPENDOWNGRADE4argsSize)...)
	p := (*[4 + oPENDOWNGRADE4argsSize]byte)(w.buf[len(w.buf)-4-oPENDOWNGRADE4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPEN_DOWNGRADE)
	return OPENDOWNGRADE4args{m: (*[oPENDOWNGRADE4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Putfh() PUTFH4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_PUTFH)
	child := StartPUTFH4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Putpubfh() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_PUTPUBFH)
}

func (w *NfsArgop4EntryWriter) SetValue_Putrootfh() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_PUTROOTFH)
}

func (w *NfsArgop4EntryWriter) SetValue_Read() READ4args {
	w.buf = append(w.buf, make([]byte, 4+rEAD4argsSize)...)
	p := (*[4 + rEAD4argsSize]byte)(w.buf[len(w.buf)-4-rEAD4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_READ)
	return READ4args{m: (*[rEAD4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Readdir() READDIR4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+24)...)
	off := len(w.buf) - 4 - 24
	binary.BigEndian.PutUint32((*[4 + 24]byte)(w.buf[off:])[:4], OP_READDIR)
	buf := w.buf
	w.buf = nil
	return READDIR4argsWriter{buf: buf, header: (*[24]byte)(buf[off+4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Readlink() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READLINK)
}

func (w *NfsArgop4EntryWriter) SetValue_Remove() REMOVE4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_REMOVE)
	child := StartREMOVE4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Rename() RENAME4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RENAME)
	child := StartRENAME4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Renew() RENEW4args {
	w.buf = append(w.buf, make([]byte, 4+rENEW4argsSize)...)
	p := (*[4 + rENEW4argsSize]byte)(w.buf[len(w.buf)-4-rENEW4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RENEW)
	return RENEW4args{m: (*[rENEW4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Restorefh() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RESTOREFH)
}

func (w *NfsArgop4EntryWriter) SetValue_Savefh() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SAVEFH)
}

func (w *NfsArgop4EntryWriter) SetValue_Secinfo() SECINFO4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SECINFO)
	child := StartSECINFO4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Setattr() SETATTR4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+16)...)
	off := len(w.buf) - 4 - 16
	binary.BigEndian.PutUint32((*[4 + 16]byte)(w.buf[off:])[:4], OP_SETATTR)
	buf := w.buf
	w.buf = nil
	return SETATTR4argsWriter{buf: buf, header: (*[16]byte)(buf[off+4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Setclientid() SETCLIENTID4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SETCLIENTID)
	child := StartSETCLIENTID4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_SetclientidConfirm() SETCLIENTIDCONFIRM4args {
	w.buf = append(w.buf, make([]byte, 4+sETCLIENTIDCONFIRM4argsSize)...)
	p := (*[4 + sETCLIENTIDCONFIRM4argsSize]byte)(w.buf[len(w.buf)-4-sETCLIENTIDCONFIRM4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_SETCLIENTID_CONFIRM)
	return SETCLIENTIDCONFIRM4args{m: (*[sETCLIENTIDCONFIRM4argsSize]byte)(p[4:])}
}

func (w *NfsArgop4EntryWriter) SetValue_Verify() VERIFY4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_VERIFY)
	child := StartVERIFY4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Write() WRITE4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+32)...)
	off := len(w.buf) - 4 - 32
	binary.BigEndian.PutUint32((*[4 + 32]byte)(w.buf[off:])[:4], OP_WRITE)
	buf := w.buf
	w.buf = nil
	return WRITE4argsWriter{buf: buf, off: off + 4}
}

func (w *NfsArgop4EntryWriter) SetValue_ReleaseLockowner() RELEASELOCKOWNER4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RELEASE_LOCKOWNER)
	child := StartRELEASELOCKOWNER4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsArgop4EntryWriter) SetValue_Illegal() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_ILLEGAL)
}

func (w *NfsArgop4EntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *NfsArgop4EntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// COMPOUND4args — variable:
//   tag + minorversion(4) + argarray_count(4) + nfs_argop4_entry argarray[argarray_count]
// -------------------------------------------------------

type COMPOUND4args struct {
	data []byte
	off1 int // byte offset within data where minorversion starts
}

func readCOMPOUND4args(b *[]byte) (COMPOUND4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readUtf8strCs(b); !ok {
		return COMPOUND4args{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return COMPOUND4args{}, false
	}
	*b = (*b)[4:]
	if len(*b) < 4 {
		return COMPOUND4args{}, false
	}
	c_argarray_count := int(binary.BigEndian.Uint32((*b)[:4]))
	*b = (*b)[4:]
	for i := 0; i < c_argarray_count; i++ {
		if _, ok := readNfsArgop4Entry(b); !ok {
			return COMPOUND4args{}, false
		}
	}
	total := startLen - len(*b)
	return COMPOUND4args{data: start[:total], off1: off1}, true
}

func ReadCOMPOUND4args(b []byte) (COMPOUND4args, bool) {
	return readCOMPOUND4args(&b)
}

func (m COMPOUND4args) Tag() Utf8strCs {
	return Utf8strCs(m.data[0:m.off1])
}

func (m COMPOUND4args) Minorversion() uint32 {
	o := m.off1
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m COMPOUND4args) Argarray() NfsArgop4EntryIter {
	argarray_count := int(binary.BigEndian.Uint32(m.data[m.off1+4 : m.off1+8]))
	return NfsArgop4EntryIter{b: m.data[m.off1+8:], count: argarray_count}
}

func (m COMPOUND4args) ArgarrayCount() uint32 {
	return binary.BigEndian.Uint32(m.data[m.off1+4 : m.off1+8])
}

// NfsArgop4EntryIter iterates over variable-size NfsArgop4Entry entries.
type NfsArgop4EntryIter struct {
	b     []byte
	count int
	i     int
	cur   NfsArgop4Entry
}

func (it *NfsArgop4EntryIter) Next() bool {
	if it.i >= it.count {
		return false
	}
	var ok bool
	it.cur, ok = readNfsArgop4Entry(&it.b)
	if !ok {
		return false
	}
	it.i++
	return true
}

func (it *NfsArgop4EntryIter) Argarray() NfsArgop4Entry {
	return it.cur
}

// COMPOUND4argsWriter writes a COMPOUND4args:
//
//	tag + minorversion + argarray_count + nfs_argop4_entry argarray[argarray_count]
type COMPOUND4argsWriter struct {
	buf              []byte
	off              int
	argarrayCount    uint32
	argarrayCountOff int
	phase            uint8
}

func StartCOMPOUND4args(buf []byte) COMPOUND4argsWriter {
	off := len(buf)
	return COMPOUND4argsWriter{buf: buf, off: off}
}

func (w *COMPOUND4argsWriter) SetMinorversion(v uint32) {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *COMPOUND4argsWriter) AppendArgarray_Access() ACCESS4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+aCCESS4argsSize)...)
	p := (*[4 + aCCESS4argsSize]byte)(w.buf[len(w.buf)-4-aCCESS4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_ACCESS)
	w.argarrayCount++
	return ACCESS4args{m: (*[aCCESS4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Close() CLOSE4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+cLOSE4argsSize)...)
	p := (*[4 + cLOSE4argsSize]byte)(w.buf[len(w.buf)-4-cLOSE4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_CLOSE)
	w.argarrayCount++
	return CLOSE4args{m: (*[cLOSE4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Commit() COMMIT4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+cOMMIT4argsSize)...)
	p := (*[4 + cOMMIT4argsSize]byte)(w.buf[len(w.buf)-4-cOMMIT4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_COMMIT)
	w.argarrayCount++
	return COMMIT4args{m: (*[cOMMIT4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Create() CREATE4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+4)...)
	off := len(w.buf) - 4 - 4
	binary.BigEndian.PutUint32((*[4 + 4]byte)(w.buf[off:])[:4], OP_CREATE)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return CREATE4argsWriter{buf: buf, header: (*[4]byte)(buf[off+4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Delegpurge() DELEGPURGE4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+dELEGPURGE4argsSize)...)
	p := (*[4 + dELEGPURGE4argsSize]byte)(w.buf[len(w.buf)-4-dELEGPURGE4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGPURGE)
	w.argarrayCount++
	return DELEGPURGE4args{m: (*[dELEGPURGE4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Delegreturn() DELEGRETURN4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+dELEGRETURN4argsSize)...)
	p := (*[4 + dELEGRETURN4argsSize]byte)(w.buf[len(w.buf)-4-dELEGRETURN4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGRETURN)
	w.argarrayCount++
	return DELEGRETURN4args{m: (*[dELEGRETURN4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Getattr() GETATTR4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETATTR)
	w.argarrayCount++
	child := StartGETATTR4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Getfh() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETFH)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Link() LINK4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LINK)
	w.argarrayCount++
	child := StartLINK4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Lock() LOCK4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+28)...)
	off := len(w.buf) - 4 - 28
	binary.BigEndian.PutUint32((*[4 + 28]byte)(w.buf[off:])[:4], OP_LOCK)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return LOCK4argsWriter{buf: buf, header: (*[28]byte)(buf[off+4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Lockt() LOCKT4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], OP_LOCKT)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return LOCKT4argsWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Locku() LOCKU4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+lOCKU4argsSize)...)
	p := (*[4 + lOCKU4argsSize]byte)(w.buf[len(w.buf)-4-lOCKU4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_LOCKU)
	w.argarrayCount++
	return LOCKU4args{m: (*[lOCKU4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Lookup() LOOKUP4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOOKUP)
	w.argarrayCount++
	child := StartLOOKUP4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Lookupp() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOOKUPP)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Nverify() NVERIFY4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_NVERIFY)
	w.argarrayCount++
	child := StartNVERIFY4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Open() OPEN4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+12)...)
	off := len(w.buf) - 4 - 12
	binary.BigEndian.PutUint32((*[4 + 12]byte)(w.buf[off:])[:4], OP_OPEN)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return OPEN4argsWriter{buf: buf, header: (*[12]byte)(buf[off+4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Openattr() OPENATTR4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+oPENATTR4argsSize)...)
	p := (*[4 + oPENATTR4argsSize]byte)(w.buf[len(w.buf)-4-oPENATTR4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPENATTR)
	w.argarrayCount++
	return OPENATTR4args{m: (*[oPENATTR4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_OpenConfirm() OPENCONFIRM4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+oPENCONFIRM4argsSize)...)
	p := (*[4 + oPENCONFIRM4argsSize]byte)(w.buf[len(w.buf)-4-oPENCONFIRM4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPEN_CONFIRM)
	w.argarrayCount++
	return OPENCONFIRM4args{m: (*[oPENCONFIRM4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_OpenDowngrade() OPENDOWNGRADE4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+oPENDOWNGRADE4argsSize)...)
	p := (*[4 + oPENDOWNGRADE4argsSize]byte)(w.buf[len(w.buf)-4-oPENDOWNGRADE4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPEN_DOWNGRADE)
	w.argarrayCount++
	return OPENDOWNGRADE4args{m: (*[oPENDOWNGRADE4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Putfh() PUTFH4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_PUTFH)
	w.argarrayCount++
	child := StartPUTFH4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Putpubfh() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_PUTPUBFH)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Putrootfh() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_PUTROOTFH)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Read() READ4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+rEAD4argsSize)...)
	p := (*[4 + rEAD4argsSize]byte)(w.buf[len(w.buf)-4-rEAD4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_READ)
	w.argarrayCount++
	return READ4args{m: (*[rEAD4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Readdir() READDIR4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+24)...)
	off := len(w.buf) - 4 - 24
	binary.BigEndian.PutUint32((*[4 + 24]byte)(w.buf[off:])[:4], OP_READDIR)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return READDIR4argsWriter{buf: buf, header: (*[24]byte)(buf[off+4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Readlink() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READLINK)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Remove() REMOVE4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_REMOVE)
	w.argarrayCount++
	child := StartREMOVE4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Rename() RENAME4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RENAME)
	w.argarrayCount++
	child := StartRENAME4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Renew() RENEW4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+rENEW4argsSize)...)
	p := (*[4 + rENEW4argsSize]byte)(w.buf[len(w.buf)-4-rENEW4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RENEW)
	w.argarrayCount++
	return RENEW4args{m: (*[rENEW4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Restorefh() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RESTOREFH)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Savefh() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SAVEFH)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) AppendArgarray_Secinfo() SECINFO4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SECINFO)
	w.argarrayCount++
	child := StartSECINFO4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Setattr() SETATTR4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+16)...)
	off := len(w.buf) - 4 - 16
	binary.BigEndian.PutUint32((*[4 + 16]byte)(w.buf[off:])[:4], OP_SETATTR)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return SETATTR4argsWriter{buf: buf, header: (*[16]byte)(buf[off+4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Setclientid() SETCLIENTID4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SETCLIENTID)
	w.argarrayCount++
	child := StartSETCLIENTID4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_SetclientidConfirm() SETCLIENTIDCONFIRM4args {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+sETCLIENTIDCONFIRM4argsSize)...)
	p := (*[4 + sETCLIENTIDCONFIRM4argsSize]byte)(w.buf[len(w.buf)-4-sETCLIENTIDCONFIRM4argsSize:])
	binary.BigEndian.PutUint32(p[:4], OP_SETCLIENTID_CONFIRM)
	w.argarrayCount++
	return SETCLIENTIDCONFIRM4args{m: (*[sETCLIENTIDCONFIRM4argsSize]byte)(p[4:])}
}

func (w *COMPOUND4argsWriter) AppendArgarray_Verify() VERIFY4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_VERIFY)
	w.argarrayCount++
	child := StartVERIFY4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Write() WRITE4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+32)...)
	off := len(w.buf) - 4 - 32
	binary.BigEndian.PutUint32((*[4 + 32]byte)(w.buf[off:])[:4], OP_WRITE)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return WRITE4argsWriter{buf: buf, off: off + 4}
}

func (w *COMPOUND4argsWriter) AppendArgarray_ReleaseLockowner() RELEASELOCKOWNER4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RELEASE_LOCKOWNER)
	w.argarrayCount++
	child := StartRELEASELOCKOWNER4args(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) AppendArgarray_Illegal() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_ILLEGAL)
	w.argarrayCount++
}

func (w *COMPOUND4argsWriter) StartTag() Utf8strCsWriter {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartUtf8strCs(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *COMPOUND4argsWriter) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.argarrayCountOff:w.argarrayCountOff+4], w.argarrayCount)
	return w.buf
}

// -------------------------------------------------------
// NfsResop4Entry — variable: disc(4) + nfs_resop4 value
// -------------------------------------------------------

type NfsResop4Entry []byte

func readNfsResop4Entry(b *[]byte) (NfsResop4Entry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readNfsResop4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return NfsResop4Entry(start[:total]), true
}

func ReadNfsResop4Entry(b []byte) (NfsResop4Entry, bool) {
	return readNfsResop4Entry(&b)
}

func (m NfsResop4Entry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m NfsResop4Entry) Value() NfsResop4 {
	return NfsResop4{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// NfsResop4EntryWriter writes a nfs_resop4_entry:
//
//	disc + value(disc)
type NfsResop4EntryWriter struct {
	buf []byte
	off int
}

func StartNfsResop4Entry(buf []byte) NfsResop4EntryWriter {
	return NfsResop4EntryWriter{buf: buf, off: len(buf)}
}

func (w *NfsResop4EntryWriter) SetValue_Access() ACCESS4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_ACCESS)
	child := StartACCESS4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Close() CLOSE4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CLOSE)
	child := StartCLOSE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Commit() COMMIT4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_COMMIT)
	child := StartCOMMIT4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Create() CREATE4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CREATE)
	child := StartCREATE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Delegpurge() DELEGPURGE4res {
	w.buf = append(w.buf, make([]byte, 4+dELEGPURGE4resSize)...)
	p := (*[4 + dELEGPURGE4resSize]byte)(w.buf[len(w.buf)-4-dELEGPURGE4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGPURGE)
	return DELEGPURGE4res{m: (*[dELEGPURGE4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Delegreturn() DELEGRETURN4res {
	w.buf = append(w.buf, make([]byte, 4+dELEGRETURN4resSize)...)
	p := (*[4 + dELEGRETURN4resSize]byte)(w.buf[len(w.buf)-4-dELEGRETURN4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGRETURN)
	return DELEGRETURN4res{m: (*[dELEGRETURN4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Getattr() GETATTR4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETATTR)
	child := StartGETATTR4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Getfh() GETFH4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETFH)
	child := StartGETFH4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Link() LINK4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LINK)
	child := StartLINK4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Lock() LOCK4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOCK)
	child := StartLOCK4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Lockt() LOCKT4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOCKT)
	child := StartLOCKT4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Locku() LOCKU4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOCKU)
	child := StartLOCKU4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Lookup() LOOKUP4res {
	w.buf = append(w.buf, make([]byte, 4+lOOKUP4resSize)...)
	p := (*[4 + lOOKUP4resSize]byte)(w.buf[len(w.buf)-4-lOOKUP4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_LOOKUP)
	return LOOKUP4res{m: (*[lOOKUP4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Lookupp() LOOKUPP4res {
	w.buf = append(w.buf, make([]byte, 4+lOOKUPP4resSize)...)
	p := (*[4 + lOOKUPP4resSize]byte)(w.buf[len(w.buf)-4-lOOKUPP4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_LOOKUPP)
	return LOOKUPP4res{m: (*[lOOKUPP4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Nverify() NVERIFY4res {
	w.buf = append(w.buf, make([]byte, 4+nVERIFY4resSize)...)
	p := (*[4 + nVERIFY4resSize]byte)(w.buf[len(w.buf)-4-nVERIFY4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_NVERIFY)
	return NVERIFY4res{m: (*[nVERIFY4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Open() OPEN4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_OPEN)
	child := StartOPEN4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Openattr() OPENATTR4res {
	w.buf = append(w.buf, make([]byte, 4+oPENATTR4resSize)...)
	p := (*[4 + oPENATTR4resSize]byte)(w.buf[len(w.buf)-4-oPENATTR4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPENATTR)
	return OPENATTR4res{m: (*[oPENATTR4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_OpenConfirm() OPENCONFIRM4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_OPEN_CONFIRM)
	child := StartOPENCONFIRM4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_OpenDowngrade() OPENDOWNGRADE4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_OPEN_DOWNGRADE)
	child := StartOPENDOWNGRADE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Putfh() PUTFH4res {
	w.buf = append(w.buf, make([]byte, 4+pUTFH4resSize)...)
	p := (*[4 + pUTFH4resSize]byte)(w.buf[len(w.buf)-4-pUTFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_PUTFH)
	return PUTFH4res{m: (*[pUTFH4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Putpubfh() PUTPUBFH4res {
	w.buf = append(w.buf, make([]byte, 4+pUTPUBFH4resSize)...)
	p := (*[4 + pUTPUBFH4resSize]byte)(w.buf[len(w.buf)-4-pUTPUBFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_PUTPUBFH)
	return PUTPUBFH4res{m: (*[pUTPUBFH4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Putrootfh() PUTROOTFH4res {
	w.buf = append(w.buf, make([]byte, 4+pUTROOTFH4resSize)...)
	p := (*[4 + pUTROOTFH4resSize]byte)(w.buf[len(w.buf)-4-pUTROOTFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_PUTROOTFH)
	return PUTROOTFH4res{m: (*[pUTROOTFH4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Read() READ4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READ)
	child := StartREAD4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Readdir() READDIR4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READDIR)
	child := StartREADDIR4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Readlink() READLINK4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READLINK)
	child := StartREADLINK4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Remove() REMOVE4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_REMOVE)
	child := StartREMOVE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Rename() RENAME4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RENAME)
	child := StartRENAME4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Renew() RENEW4res {
	w.buf = append(w.buf, make([]byte, 4+rENEW4resSize)...)
	p := (*[4 + rENEW4resSize]byte)(w.buf[len(w.buf)-4-rENEW4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RENEW)
	return RENEW4res{m: (*[rENEW4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Restorefh() RESTOREFH4res {
	w.buf = append(w.buf, make([]byte, 4+rESTOREFH4resSize)...)
	p := (*[4 + rESTOREFH4resSize]byte)(w.buf[len(w.buf)-4-rESTOREFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RESTOREFH)
	return RESTOREFH4res{m: (*[rESTOREFH4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Savefh() SAVEFH4res {
	w.buf = append(w.buf, make([]byte, 4+sAVEFH4resSize)...)
	p := (*[4 + sAVEFH4resSize]byte)(w.buf[len(w.buf)-4-sAVEFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_SAVEFH)
	return SAVEFH4res{m: (*[sAVEFH4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Secinfo() SECINFO4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SECINFO)
	child := StartSECINFO4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_Setattr() SETATTR4resWriter {
	w.buf = append(w.buf, make([]byte, 4+4)...)
	off := len(w.buf) - 4 - 4
	binary.BigEndian.PutUint32((*[4 + 4]byte)(w.buf[off:])[:4], OP_SETATTR)
	buf := w.buf
	w.buf = nil
	return SETATTR4resWriter{buf: buf, header: (*[4]byte)(buf[off+4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Setclientid() SETCLIENTID4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SETCLIENTID)
	child := StartSETCLIENTID4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_SetclientidConfirm() SETCLIENTIDCONFIRM4res {
	w.buf = append(w.buf, make([]byte, 4+sETCLIENTIDCONFIRM4resSize)...)
	p := (*[4 + sETCLIENTIDCONFIRM4resSize]byte)(w.buf[len(w.buf)-4-sETCLIENTIDCONFIRM4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_SETCLIENTID_CONFIRM)
	return SETCLIENTIDCONFIRM4res{m: (*[sETCLIENTIDCONFIRM4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Verify() VERIFY4res {
	w.buf = append(w.buf, make([]byte, 4+vERIFY4resSize)...)
	p := (*[4 + vERIFY4resSize]byte)(w.buf[len(w.buf)-4-vERIFY4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_VERIFY)
	return VERIFY4res{m: (*[vERIFY4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Write() WRITE4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_WRITE)
	child := StartWRITE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsResop4EntryWriter) SetValue_ReleaseLockowner() RELEASELOCKOWNER4res {
	w.buf = append(w.buf, make([]byte, 4+rELEASELOCKOWNER4resSize)...)
	p := (*[4 + rELEASELOCKOWNER4resSize]byte)(w.buf[len(w.buf)-4-rELEASELOCKOWNER4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RELEASE_LOCKOWNER)
	return RELEASELOCKOWNER4res{m: (*[rELEASELOCKOWNER4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) SetValue_Illegal() ILLEGAL4res {
	w.buf = append(w.buf, make([]byte, 4+iLLEGAL4resSize)...)
	p := (*[4 + iLLEGAL4resSize]byte)(w.buf[len(w.buf)-4-iLLEGAL4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_ILLEGAL)
	return ILLEGAL4res{m: (*[iLLEGAL4resSize]byte)(p[4:])}
}

func (w *NfsResop4EntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *NfsResop4EntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// COMPOUND4res — variable:
//   status(4) + tag + resarray_count(4) + nfs_resop4_entry resarray[resarray_count]
// -------------------------------------------------------

type COMPOUND4res struct {
	data []byte
	off1 int // byte offset within data where resarray_count starts
}

func readCOMPOUND4res(b *[]byte) (COMPOUND4res, bool) {
	if len(*b) < 4 {
		return COMPOUND4res{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readUtf8strCs(b); !ok {
		return COMPOUND4res{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return COMPOUND4res{}, false
	}
	c_resarray_count := int(binary.BigEndian.Uint32((*b)[:4]))
	*b = (*b)[4:]
	for i := 0; i < c_resarray_count; i++ {
		if _, ok := readNfsResop4Entry(b); !ok {
			return COMPOUND4res{}, false
		}
	}
	total := startLen - len(*b)
	return COMPOUND4res{data: start[:total], off1: off1}, true
}

func ReadCOMPOUND4res(b []byte) (COMPOUND4res, bool) {
	return readCOMPOUND4res(&b)
}

func (m COMPOUND4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.data[0:4])
}

func (m COMPOUND4res) Tag() Utf8strCs {
	return Utf8strCs(m.data[4:m.off1])
}

func (m COMPOUND4res) Resarray() NfsResop4EntryIter {
	resarray_count := int(binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4]))
	return NfsResop4EntryIter{b: m.data[m.off1+4:], count: resarray_count}
}

func (m COMPOUND4res) ResarrayCount() uint32 {
	return binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4])
}

// NfsResop4EntryIter iterates over variable-size NfsResop4Entry entries.
type NfsResop4EntryIter struct {
	b     []byte
	count int
	i     int
	cur   NfsResop4Entry
}

func (it *NfsResop4EntryIter) Next() bool {
	if it.i >= it.count {
		return false
	}
	var ok bool
	it.cur, ok = readNfsResop4Entry(&it.b)
	if !ok {
		return false
	}
	it.i++
	return true
}

func (it *NfsResop4EntryIter) Resarray() NfsResop4Entry {
	return it.cur
}

// COMPOUND4resWriter writes a COMPOUND4res:
//
//	status + tag + resarray_count + nfs_resop4_entry resarray[resarray_count]
type COMPOUND4resWriter struct {
	buf              []byte
	off              int
	resarrayCount    uint32
	resarrayCountOff int
	phase            uint8
}

func StartCOMPOUND4res(buf []byte) COMPOUND4resWriter {
	off := len(buf)
	buf = append(buf, make([]byte, 4)...) // status(4)
	return COMPOUND4resWriter{buf: buf, off: off}
}

func (w *COMPOUND4resWriter) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], v)
}

func (w *COMPOUND4resWriter) AppendResarray_Access() ACCESS4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_ACCESS)
	w.resarrayCount++
	child := StartACCESS4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Close() CLOSE4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CLOSE)
	w.resarrayCount++
	child := StartCLOSE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Commit() COMMIT4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_COMMIT)
	w.resarrayCount++
	child := StartCOMMIT4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Create() CREATE4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CREATE)
	w.resarrayCount++
	child := StartCREATE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Delegpurge() DELEGPURGE4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+dELEGPURGE4resSize)...)
	p := (*[4 + dELEGPURGE4resSize]byte)(w.buf[len(w.buf)-4-dELEGPURGE4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGPURGE)
	w.resarrayCount++
	return DELEGPURGE4res{m: (*[dELEGPURGE4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Delegreturn() DELEGRETURN4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+dELEGRETURN4resSize)...)
	p := (*[4 + dELEGRETURN4resSize]byte)(w.buf[len(w.buf)-4-dELEGRETURN4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_DELEGRETURN)
	w.resarrayCount++
	return DELEGRETURN4res{m: (*[dELEGRETURN4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Getattr() GETATTR4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETATTR)
	w.resarrayCount++
	child := StartGETATTR4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Getfh() GETFH4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_GETFH)
	w.resarrayCount++
	child := StartGETFH4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Link() LINK4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LINK)
	w.resarrayCount++
	child := StartLINK4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Lock() LOCK4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOCK)
	w.resarrayCount++
	child := StartLOCK4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Lockt() LOCKT4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOCKT)
	w.resarrayCount++
	child := StartLOCKT4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Locku() LOCKU4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_LOCKU)
	w.resarrayCount++
	child := StartLOCKU4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Lookup() LOOKUP4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+lOOKUP4resSize)...)
	p := (*[4 + lOOKUP4resSize]byte)(w.buf[len(w.buf)-4-lOOKUP4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_LOOKUP)
	w.resarrayCount++
	return LOOKUP4res{m: (*[lOOKUP4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Lookupp() LOOKUPP4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+lOOKUPP4resSize)...)
	p := (*[4 + lOOKUPP4resSize]byte)(w.buf[len(w.buf)-4-lOOKUPP4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_LOOKUPP)
	w.resarrayCount++
	return LOOKUPP4res{m: (*[lOOKUPP4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Nverify() NVERIFY4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+nVERIFY4resSize)...)
	p := (*[4 + nVERIFY4resSize]byte)(w.buf[len(w.buf)-4-nVERIFY4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_NVERIFY)
	w.resarrayCount++
	return NVERIFY4res{m: (*[nVERIFY4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Open() OPEN4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_OPEN)
	w.resarrayCount++
	child := StartOPEN4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Openattr() OPENATTR4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+oPENATTR4resSize)...)
	p := (*[4 + oPENATTR4resSize]byte)(w.buf[len(w.buf)-4-oPENATTR4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_OPENATTR)
	w.resarrayCount++
	return OPENATTR4res{m: (*[oPENATTR4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_OpenConfirm() OPENCONFIRM4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_OPEN_CONFIRM)
	w.resarrayCount++
	child := StartOPENCONFIRM4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_OpenDowngrade() OPENDOWNGRADE4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_OPEN_DOWNGRADE)
	w.resarrayCount++
	child := StartOPENDOWNGRADE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Putfh() PUTFH4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+pUTFH4resSize)...)
	p := (*[4 + pUTFH4resSize]byte)(w.buf[len(w.buf)-4-pUTFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_PUTFH)
	w.resarrayCount++
	return PUTFH4res{m: (*[pUTFH4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Putpubfh() PUTPUBFH4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+pUTPUBFH4resSize)...)
	p := (*[4 + pUTPUBFH4resSize]byte)(w.buf[len(w.buf)-4-pUTPUBFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_PUTPUBFH)
	w.resarrayCount++
	return PUTPUBFH4res{m: (*[pUTPUBFH4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Putrootfh() PUTROOTFH4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+pUTROOTFH4resSize)...)
	p := (*[4 + pUTROOTFH4resSize]byte)(w.buf[len(w.buf)-4-pUTROOTFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_PUTROOTFH)
	w.resarrayCount++
	return PUTROOTFH4res{m: (*[pUTROOTFH4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Read() READ4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READ)
	w.resarrayCount++
	child := StartREAD4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Readdir() READDIR4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READDIR)
	w.resarrayCount++
	child := StartREADDIR4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Readlink() READLINK4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_READLINK)
	w.resarrayCount++
	child := StartREADLINK4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Remove() REMOVE4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_REMOVE)
	w.resarrayCount++
	child := StartREMOVE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Rename() RENAME4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_RENAME)
	w.resarrayCount++
	child := StartRENAME4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Renew() RENEW4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+rENEW4resSize)...)
	p := (*[4 + rENEW4resSize]byte)(w.buf[len(w.buf)-4-rENEW4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RENEW)
	w.resarrayCount++
	return RENEW4res{m: (*[rENEW4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Restorefh() RESTOREFH4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+rESTOREFH4resSize)...)
	p := (*[4 + rESTOREFH4resSize]byte)(w.buf[len(w.buf)-4-rESTOREFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RESTOREFH)
	w.resarrayCount++
	return RESTOREFH4res{m: (*[rESTOREFH4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Savefh() SAVEFH4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+sAVEFH4resSize)...)
	p := (*[4 + sAVEFH4resSize]byte)(w.buf[len(w.buf)-4-sAVEFH4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_SAVEFH)
	w.resarrayCount++
	return SAVEFH4res{m: (*[sAVEFH4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Secinfo() SECINFO4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SECINFO)
	w.resarrayCount++
	child := StartSECINFO4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_Setattr() SETATTR4resWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+4)...)
	off := len(w.buf) - 4 - 4
	binary.BigEndian.PutUint32((*[4 + 4]byte)(w.buf[off:])[:4], OP_SETATTR)
	w.resarrayCount++
	buf := w.buf
	w.buf = nil
	return SETATTR4resWriter{buf: buf, header: (*[4]byte)(buf[off+4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Setclientid() SETCLIENTID4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_SETCLIENTID)
	w.resarrayCount++
	child := StartSETCLIENTID4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_SetclientidConfirm() SETCLIENTIDCONFIRM4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+sETCLIENTIDCONFIRM4resSize)...)
	p := (*[4 + sETCLIENTIDCONFIRM4resSize]byte)(w.buf[len(w.buf)-4-sETCLIENTIDCONFIRM4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_SETCLIENTID_CONFIRM)
	w.resarrayCount++
	return SETCLIENTIDCONFIRM4res{m: (*[sETCLIENTIDCONFIRM4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Verify() VERIFY4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+vERIFY4resSize)...)
	p := (*[4 + vERIFY4resSize]byte)(w.buf[len(w.buf)-4-vERIFY4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_VERIFY)
	w.resarrayCount++
	return VERIFY4res{m: (*[vERIFY4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Write() WRITE4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_WRITE)
	w.resarrayCount++
	child := StartWRITE4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) AppendResarray_ReleaseLockowner() RELEASELOCKOWNER4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+rELEASELOCKOWNER4resSize)...)
	p := (*[4 + rELEASELOCKOWNER4resSize]byte)(w.buf[len(w.buf)-4-rELEASELOCKOWNER4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_RELEASE_LOCKOWNER)
	w.resarrayCount++
	return RELEASELOCKOWNER4res{m: (*[rELEASELOCKOWNER4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) AppendResarray_Illegal() ILLEGAL4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+iLLEGAL4resSize)...)
	p := (*[4 + iLLEGAL4resSize]byte)(w.buf[len(w.buf)-4-iLLEGAL4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_ILLEGAL)
	w.resarrayCount++
	return ILLEGAL4res{m: (*[iLLEGAL4resSize]byte)(p[4:])}
}

func (w *COMPOUND4resWriter) StartTag() Utf8strCsWriter {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartUtf8strCs(w.buf)
	w.buf = nil
	return child
}

func (w *COMPOUND4resWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *COMPOUND4resWriter) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.resarrayCountOff:w.resarrayCountOff+4], w.resarrayCount)
	return w.buf
}

// -------------------------------------------------------
// CBGETATTR4args — variable: fh + attr_request
// -------------------------------------------------------

type CBGETATTR4args struct {
	data []byte
	off1 int // byte offset within data where attr_request starts
}

func readCBGETATTR4args(b *[]byte) (CBGETATTR4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readNfsFh4(b); !ok {
		return CBGETATTR4args{}, false
	}
	off1 := startLen - len(*b)
	if _, ok := readBitmap4(b); !ok {
		return CBGETATTR4args{}, false
	}
	total := startLen - len(*b)
	return CBGETATTR4args{data: start[:total], off1: off1}, true
}

func ReadCBGETATTR4args(b []byte) (CBGETATTR4args, bool) {
	return readCBGETATTR4args(&b)
}

func (m CBGETATTR4args) Fh() NfsFh4 {
	return NfsFh4(m.data[0:m.off1])
}

func (m CBGETATTR4args) AttrRequest() Bitmap4 {
	return Bitmap4(m.data[m.off1:])
}

// CBGETATTR4argsWriter writes a CB_GETATTR4args:
//
//	fh + attr_request
type CBGETATTR4argsWriter struct {
	buf   []byte
	off   int
	phase uint8
}

func StartCBGETATTR4args(buf []byte) CBGETATTR4argsWriter {
	off := len(buf)
	return CBGETATTR4argsWriter{buf: buf, off: off}
}

func (w *CBGETATTR4argsWriter) StartFh() NfsFh4Writer {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartNfsFh4(w.buf)
	w.buf = nil
	return child
}

func (w *CBGETATTR4argsWriter) StartAttrRequest() Bitmap4Writer {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	child := StartBitmap4(w.buf)
	w.buf = nil
	return child
}

func (w *CBGETATTR4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CBGETATTR4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CBGETATTR4resok — variable: obj_attributes
// -------------------------------------------------------

type CBGETATTR4resok []byte

func readCBGETATTR4resok(b *[]byte) (CBGETATTR4resok, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readFattr4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CBGETATTR4resok(start[:total]), true
}

func ReadCBGETATTR4resok(b []byte) (CBGETATTR4resok, bool) {
	return readCBGETATTR4resok(&b)
}

func (m CBGETATTR4resok) ObjAttributes() Fattr4 {
	v, _ := ReadFattr4(m[0:])
	return v
}

// CBGETATTR4resokWriter writes a CB_GETATTR4resok:
//
//	obj_attributes
type CBGETATTR4resokWriter struct {
	buf []byte
	off int
}

func StartCBGETATTR4resok(buf []byte) CBGETATTR4resokWriter {
	off := len(buf)
	return CBGETATTR4resokWriter{buf: buf, off: off}
}

func (w *CBGETATTR4resokWriter) StartObjAttributes() Fattr4Writer {
	child := StartFattr4(w.buf)
	w.buf = nil
	return child
}

func (w *CBGETATTR4resokWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CBGETATTR4resokWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CBGETATTR4res — union on nfsstat4 (external discriminant)
// -------------------------------------------------------

type CBGETATTR4res struct {
	b    []byte
	disc uint32
}

func readCBGETATTR4res(b *[]byte, nfsstat4 uint32) (CBGETATTR4res, bool) {
	switch nfsstat4 {
	case NFS4_OK:
		r, ok := readCBGETATTR4resok(b)
		if !ok {
			return CBGETATTR4res{}, false
		}
		return CBGETATTR4res{b: []byte(r), disc: nfsstat4}, true
	default:
		return CBGETATTR4res{b: (*b)[:0], disc: nfsstat4}, true
	}
}

func (m CBGETATTR4res) AsCBGETATTR4resok() CBGETATTR4resok {
	if m.disc != NFS4_OK {
		panic("wrong union discriminant")
	}
	return CBGETATTR4resok(m.b)
}

// -------------------------------------------------------
// CBRECALL4args — variable: stateid(16) + truncate(4) + fh
// -------------------------------------------------------

type CBRECALL4args []byte

func readCBRECALL4args(b *[]byte) (CBRECALL4args, bool) {
	if len(*b) < 20 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[20:]
	if _, ok := readNfsFh4(b); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CBRECALL4args(start[:total]), true
}

func ReadCBRECALL4args(b []byte) (CBRECALL4args, bool) {
	return readCBRECALL4args(&b)
}

func (m CBRECALL4args) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(m[0 : 0+stateid4Size])}
}

func (m CBRECALL4args) Truncate() uint32 {
	return binary.BigEndian.Uint32(m[16:20])
}

func (m CBRECALL4args) Fh() NfsFh4 {
	return NfsFh4(m[20:])
}

// CBRECALL4argsWriter writes a CB_RECALL4args:
//
//	stateid + truncate + fh
type CBRECALL4argsWriter struct {
	buf    []byte
	header *[20]byte
}

func StartCBRECALL4args(buf []byte) CBRECALL4argsWriter {
	buf = append(buf, make([]byte, 20)...) // stateid(16) + truncate(4)
	return CBRECALL4argsWriter{buf: buf, header: (*[20]byte)(buf[len(buf)-20:])}
}

func (w *CBRECALL4argsWriter) Stateid() Stateid4 {
	return Stateid4{m: (*[stateid4Size]byte)(w.header[0:])}
}

func (w *CBRECALL4argsWriter) SetTruncate(v uint32) {
	binary.BigEndian.PutUint32(w.header[16:20], v)
}

func (w *CBRECALL4argsWriter) StartFh() NfsFh4Writer {
	child := StartNfsFh4(w.buf)
	w.buf = nil
	w.header = nil
	return child
}

func (w *CBRECALL4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CBRECALL4argsWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CBRECALL4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type CBRECALL4res struct {
	m *[cBRECALL4resSize]byte
}

const cBRECALL4resSize = 4

func readCBRECALL4res(b *[]byte) (CBRECALL4res, bool) {
	if len(*b) < cBRECALL4resSize {
		return CBRECALL4res{}, false
	}
	result := CBRECALL4res{m: (*[cBRECALL4resSize]byte)(*b)}
	*b = (*b)[cBRECALL4resSize:]
	return result, true
}

func ReadCBRECALL4res(b []byte) (CBRECALL4res, bool) {
	return readCBRECALL4res(&b)
}

func StartCBRECALL4res(buf []byte) ([]byte, CBRECALL4res) {
	buf = append(buf, make([]byte, cBRECALL4resSize)...)
	return buf, CBRECALL4res{m: (*[cBRECALL4resSize]byte)(buf[len(buf)-cBRECALL4resSize:])}
}

func (m CBRECALL4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m CBRECALL4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// CBILLEGAL4res — fixed 4 bytes: status(4, beu32)
// -------------------------------------------------------

type CBILLEGAL4res struct {
	m *[cBILLEGAL4resSize]byte
}

const cBILLEGAL4resSize = 4

func readCBILLEGAL4res(b *[]byte) (CBILLEGAL4res, bool) {
	if len(*b) < cBILLEGAL4resSize {
		return CBILLEGAL4res{}, false
	}
	result := CBILLEGAL4res{m: (*[cBILLEGAL4resSize]byte)(*b)}
	*b = (*b)[cBILLEGAL4resSize:]
	return result, true
}

func ReadCBILLEGAL4res(b []byte) (CBILLEGAL4res, bool) {
	return readCBILLEGAL4res(&b)
}

func StartCBILLEGAL4res(buf []byte) ([]byte, CBILLEGAL4res) {
	buf = append(buf, make([]byte, cBILLEGAL4resSize)...)
	return buf, CBILLEGAL4res{m: (*[cBILLEGAL4resSize]byte)(buf[len(buf)-cBILLEGAL4resSize:])}
}

func (m CBILLEGAL4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.m[0:4])
}

func (m CBILLEGAL4res) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(m.m[0:4], v)
}

// -------------------------------------------------------
// NfsCbArgop4 — union on nfs_cb_opnum4 (external discriminant)
// -------------------------------------------------------

type NfsCbArgop4 struct {
	b    []byte
	disc uint32
}

func readNfsCbArgop4(b *[]byte, nfsCbOpnum4 uint32) (NfsCbArgop4, bool) {
	switch nfsCbOpnum4 {
	case OP_CB_GETATTR:
		r, ok := readCBGETATTR4args(b)
		if !ok {
			return NfsCbArgop4{}, false
		}
		return NfsCbArgop4{b: r.data, disc: nfsCbOpnum4}, true
	case OP_CB_RECALL:
		r, ok := readCBRECALL4args(b)
		if !ok {
			return NfsCbArgop4{}, false
		}
		return NfsCbArgop4{b: []byte(r), disc: nfsCbOpnum4}, true
	case OP_CB_ILLEGAL:
		return NfsCbArgop4{b: (*b)[:0], disc: nfsCbOpnum4}, true
	default:
		return NfsCbArgop4{}, false
	}
}

func (m NfsCbArgop4) AsCBGETATTR4args() CBGETATTR4args {
	if m.disc != OP_CB_GETATTR {
		panic("wrong union discriminant")
	}
	v, _ := ReadCBGETATTR4args(m.b)
	return v
}

func (m NfsCbArgop4) AsCBRECALL4args() CBRECALL4args {
	if m.disc != OP_CB_RECALL {
		panic("wrong union discriminant")
	}
	return CBRECALL4args(m.b)
}

// -------------------------------------------------------
// CBGETATTR4resEntry — variable: disc(4) + CB_GETATTR4res value
// -------------------------------------------------------

type CBGETATTR4resEntry []byte

func readCBGETATTR4resEntry(b *[]byte) (CBGETATTR4resEntry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readCBGETATTR4res(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return CBGETATTR4resEntry(start[:total]), true
}

func ReadCBGETATTR4resEntry(b []byte) (CBGETATTR4resEntry, bool) {
	return readCBGETATTR4resEntry(&b)
}

func (m CBGETATTR4resEntry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m CBGETATTR4resEntry) Value() CBGETATTR4res {
	return CBGETATTR4res{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// CBGETATTR4resEntryWriter writes a CB_GETATTR4res_entry:
//
//	disc + value(disc)
type CBGETATTR4resEntryWriter struct {
	buf []byte
	off int
}

func StartCBGETATTR4resEntry(buf []byte) CBGETATTR4resEntryWriter {
	return CBGETATTR4resEntryWriter{buf: buf, off: len(buf)}
}

func (w *CBGETATTR4resEntryWriter) SetValue_Nfs4Ok() CBGETATTR4resokWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, NFS4_OK)
	child := StartCBGETATTR4resok(w.buf)
	w.buf = nil
	return child
}

func (w *CBGETATTR4resEntryWriter) SetValue_Default(nfsstat4 uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, nfsstat4)
}

func (w *CBGETATTR4resEntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CBGETATTR4resEntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// NfsCbResop4 — union on nfs_cb_opnum4 (external discriminant)
// -------------------------------------------------------

type NfsCbResop4 struct {
	b    []byte
	disc uint32
}

func readNfsCbResop4(b *[]byte, nfsCbOpnum4 uint32) (NfsCbResop4, bool) {
	switch nfsCbOpnum4 {
	case OP_CB_GETATTR:
		r, ok := readCBGETATTR4resEntry(b)
		if !ok {
			return NfsCbResop4{}, false
		}
		return NfsCbResop4{b: []byte(r), disc: nfsCbOpnum4}, true
	case OP_CB_RECALL:
		r, ok := readCBRECALL4res(b)
		if !ok {
			return NfsCbResop4{}, false
		}
		return NfsCbResop4{b: r.m[:], disc: nfsCbOpnum4}, true
	case OP_CB_ILLEGAL:
		r, ok := readCBILLEGAL4res(b)
		if !ok {
			return NfsCbResop4{}, false
		}
		return NfsCbResop4{b: r.m[:], disc: nfsCbOpnum4}, true
	default:
		return NfsCbResop4{}, false
	}
}

func (m NfsCbResop4) AsCBGETATTR4resEntry() CBGETATTR4resEntry {
	if m.disc != OP_CB_GETATTR {
		panic("wrong union discriminant")
	}
	return CBGETATTR4resEntry(m.b)
}

func (m NfsCbResop4) AsCBRECALL4res() CBRECALL4res {
	if m.disc != OP_CB_RECALL {
		panic("wrong union discriminant")
	}
	return CBRECALL4res{m: (*[cBRECALL4resSize]byte)(m.b)}
}

func (m NfsCbResop4) AsCBILLEGAL4res() CBILLEGAL4res {
	if m.disc != OP_CB_ILLEGAL {
		panic("wrong union discriminant")
	}
	return CBILLEGAL4res{m: (*[cBILLEGAL4resSize]byte)(m.b)}
}

// -------------------------------------------------------
// NfsCbArgop4Entry — variable: disc(4) + nfs_cb_argop4 value
// -------------------------------------------------------

type NfsCbArgop4Entry []byte

func readNfsCbArgop4Entry(b *[]byte) (NfsCbArgop4Entry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readNfsCbArgop4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return NfsCbArgop4Entry(start[:total]), true
}

func ReadNfsCbArgop4Entry(b []byte) (NfsCbArgop4Entry, bool) {
	return readNfsCbArgop4Entry(&b)
}

func (m NfsCbArgop4Entry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m NfsCbArgop4Entry) Value() NfsCbArgop4 {
	return NfsCbArgop4{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// NfsCbArgop4EntryWriter writes a nfs_cb_argop4_entry:
//
//	disc + value(disc)
type NfsCbArgop4EntryWriter struct {
	buf []byte
	off int
}

func StartNfsCbArgop4Entry(buf []byte) NfsCbArgop4EntryWriter {
	return NfsCbArgop4EntryWriter{buf: buf, off: len(buf)}
}

func (w *NfsCbArgop4EntryWriter) SetValue_Getattr() CBGETATTR4argsWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CB_GETATTR)
	child := StartCBGETATTR4args(w.buf)
	w.buf = nil
	return child
}

func (w *NfsCbArgop4EntryWriter) SetValue_Recall() CBRECALL4argsWriter {
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], OP_CB_RECALL)
	buf := w.buf
	w.buf = nil
	return CBRECALL4argsWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *NfsCbArgop4EntryWriter) SetValue_Illegal() {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CB_ILLEGAL)
}

func (w *NfsCbArgop4EntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *NfsCbArgop4EntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CBCOMPOUND4args — variable:
//   tag + minorversion(4) + callback_ident(4) + argarray_count(4) + nfs_cb_argop4_entry argarray[argarray_count]
// -------------------------------------------------------

type CBCOMPOUND4args struct {
	data []byte
	off1 int // byte offset within data where minorversion starts
}

func readCBCOMPOUND4args(b *[]byte) (CBCOMPOUND4args, bool) {
	start := *b
	startLen := len(start)
	if _, ok := readUtf8strCs(b); !ok {
		return CBCOMPOUND4args{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return CBCOMPOUND4args{}, false
	}
	*b = (*b)[4:]
	if len(*b) < 4 {
		return CBCOMPOUND4args{}, false
	}
	*b = (*b)[4:]
	if len(*b) < 4 {
		return CBCOMPOUND4args{}, false
	}
	c_argarray_count := int(binary.BigEndian.Uint32((*b)[:4]))
	*b = (*b)[4:]
	for i := 0; i < c_argarray_count; i++ {
		if _, ok := readNfsCbArgop4Entry(b); !ok {
			return CBCOMPOUND4args{}, false
		}
	}
	total := startLen - len(*b)
	return CBCOMPOUND4args{data: start[:total], off1: off1}, true
}

func ReadCBCOMPOUND4args(b []byte) (CBCOMPOUND4args, bool) {
	return readCBCOMPOUND4args(&b)
}

func (m CBCOMPOUND4args) Tag() Utf8strCs {
	return Utf8strCs(m.data[0:m.off1])
}

func (m CBCOMPOUND4args) Minorversion() uint32 {
	o := m.off1
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m CBCOMPOUND4args) CallbackIdent() uint32 {
	o := m.off1 + 4
	return binary.BigEndian.Uint32(m.data[o : o+4])
}

func (m CBCOMPOUND4args) Argarray() NfsCbArgop4EntryIter {
	argarray_count := int(binary.BigEndian.Uint32(m.data[m.off1+8 : m.off1+12]))
	return NfsCbArgop4EntryIter{b: m.data[m.off1+12:], count: argarray_count}
}

func (m CBCOMPOUND4args) ArgarrayCount() uint32 {
	return binary.BigEndian.Uint32(m.data[m.off1+8 : m.off1+12])
}

// NfsCbArgop4EntryIter iterates over variable-size NfsCbArgop4Entry entries.
type NfsCbArgop4EntryIter struct {
	b     []byte
	count int
	i     int
	cur   NfsCbArgop4Entry
}

func (it *NfsCbArgop4EntryIter) Next() bool {
	if it.i >= it.count {
		return false
	}
	var ok bool
	it.cur, ok = readNfsCbArgop4Entry(&it.b)
	if !ok {
		return false
	}
	it.i++
	return true
}

func (it *NfsCbArgop4EntryIter) Argarray() NfsCbArgop4Entry {
	return it.cur
}

// CBCOMPOUND4argsWriter writes a CB_COMPOUND4args:
//
//	tag + minorversion + callback_ident + argarray_count + nfs_cb_argop4_entry argarray[argarray_count]
type CBCOMPOUND4argsWriter struct {
	buf              []byte
	off              int
	argarrayCount    uint32
	argarrayCountOff int
	phase            uint8
}

func StartCBCOMPOUND4args(buf []byte) CBCOMPOUND4argsWriter {
	off := len(buf)
	return CBCOMPOUND4argsWriter{buf: buf, off: off}
}

func (w *CBCOMPOUND4argsWriter) SetMinorversion(v uint32) {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *CBCOMPOUND4argsWriter) SetCallbackIdent(v uint32) {
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

func (w *CBCOMPOUND4argsWriter) AppendArgarray_Getattr() CBGETATTR4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CB_GETATTR)
	w.argarrayCount++
	child := StartCBGETATTR4args(w.buf)
	w.buf = nil
	return child
}

func (w *CBCOMPOUND4argsWriter) AppendArgarray_Recall() CBRECALL4argsWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+20)...)
	off := len(w.buf) - 4 - 20
	binary.BigEndian.PutUint32((*[4 + 20]byte)(w.buf[off:])[:4], OP_CB_RECALL)
	w.argarrayCount++
	buf := w.buf
	w.buf = nil
	return CBRECALL4argsWriter{buf: buf, header: (*[20]byte)(buf[off+4:])}
}

func (w *CBCOMPOUND4argsWriter) AppendArgarray_Illegal() {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.argarrayCountOff == 0 {
		w.argarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CB_ILLEGAL)
	w.argarrayCount++
}

func (w *CBCOMPOUND4argsWriter) StartTag() Utf8strCsWriter {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartUtf8strCs(w.buf)
	w.buf = nil
	return child
}

func (w *CBCOMPOUND4argsWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CBCOMPOUND4argsWriter) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.argarrayCountOff:w.argarrayCountOff+4], w.argarrayCount)
	return w.buf
}

// -------------------------------------------------------
// NfsCbResop4Entry — variable: disc(4) + nfs_cb_resop4 value
// -------------------------------------------------------

type NfsCbResop4Entry []byte

func readNfsCbResop4Entry(b *[]byte) (NfsCbResop4Entry, bool) {
	if len(*b) < 4 {
		return nil, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readNfsCbResop4(b, binary.BigEndian.Uint32(start[0:4])); !ok {
		return nil, false
	}
	total := startLen - len(*b)
	return NfsCbResop4Entry(start[:total]), true
}

func ReadNfsCbResop4Entry(b []byte) (NfsCbResop4Entry, bool) {
	return readNfsCbResop4Entry(&b)
}

func (m NfsCbResop4Entry) Disc() uint32 {
	return binary.BigEndian.Uint32(m[0:4])
}

func (m NfsCbResop4Entry) Value() NfsCbResop4 {
	return NfsCbResop4{b: m[4:], disc: binary.BigEndian.Uint32(m[0:4])}
}

// NfsCbResop4EntryWriter writes a nfs_cb_resop4_entry:
//
//	disc + value(disc)
type NfsCbResop4EntryWriter struct {
	buf []byte
	off int
}

func StartNfsCbResop4Entry(buf []byte) NfsCbResop4EntryWriter {
	return NfsCbResop4EntryWriter{buf: buf, off: len(buf)}
}

func (w *NfsCbResop4EntryWriter) SetValue_Getattr() CBGETATTR4resEntryWriter {
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CB_GETATTR)
	child := StartCBGETATTR4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *NfsCbResop4EntryWriter) SetValue_Recall() CBRECALL4res {
	w.buf = append(w.buf, make([]byte, 4+cBRECALL4resSize)...)
	p := (*[4 + cBRECALL4resSize]byte)(w.buf[len(w.buf)-4-cBRECALL4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_CB_RECALL)
	return CBRECALL4res{m: (*[cBRECALL4resSize]byte)(p[4:])}
}

func (w *NfsCbResop4EntryWriter) SetValue_Illegal() CBILLEGAL4res {
	w.buf = append(w.buf, make([]byte, 4+cBILLEGAL4resSize)...)
	p := (*[4 + cBILLEGAL4resSize]byte)(w.buf[len(w.buf)-4-cBILLEGAL4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_CB_ILLEGAL)
	return CBILLEGAL4res{m: (*[cBILLEGAL4resSize]byte)(p[4:])}
}

func (w *NfsCbResop4EntryWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *NfsCbResop4EntryWriter) Finish() []byte {
	_ = w.buf[:1]
	return w.buf
}

// -------------------------------------------------------
// CBCOMPOUND4res — variable:
//   status(4) + tag + resarray_count(4) + nfs_cb_resop4_entry resarray[resarray_count]
// -------------------------------------------------------

type CBCOMPOUND4res struct {
	data []byte
	off1 int // byte offset within data where resarray_count starts
}

func readCBCOMPOUND4res(b *[]byte) (CBCOMPOUND4res, bool) {
	if len(*b) < 4 {
		return CBCOMPOUND4res{}, false
	}
	start := *b
	startLen := len(start)
	*b = (*b)[4:]
	if _, ok := readUtf8strCs(b); !ok {
		return CBCOMPOUND4res{}, false
	}
	off1 := startLen - len(*b)
	if len(*b) < 4 {
		return CBCOMPOUND4res{}, false
	}
	c_resarray_count := int(binary.BigEndian.Uint32((*b)[:4]))
	*b = (*b)[4:]
	for i := 0; i < c_resarray_count; i++ {
		if _, ok := readNfsCbResop4Entry(b); !ok {
			return CBCOMPOUND4res{}, false
		}
	}
	total := startLen - len(*b)
	return CBCOMPOUND4res{data: start[:total], off1: off1}, true
}

func ReadCBCOMPOUND4res(b []byte) (CBCOMPOUND4res, bool) {
	return readCBCOMPOUND4res(&b)
}

func (m CBCOMPOUND4res) Status() uint32 {
	return binary.BigEndian.Uint32(m.data[0:4])
}

func (m CBCOMPOUND4res) Tag() Utf8strCs {
	return Utf8strCs(m.data[4:m.off1])
}

func (m CBCOMPOUND4res) Resarray() NfsCbResop4EntryIter {
	resarray_count := int(binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4]))
	return NfsCbResop4EntryIter{b: m.data[m.off1+4:], count: resarray_count}
}

func (m CBCOMPOUND4res) ResarrayCount() uint32 {
	return binary.BigEndian.Uint32(m.data[m.off1 : m.off1+4])
}

// NfsCbResop4EntryIter iterates over variable-size NfsCbResop4Entry entries.
type NfsCbResop4EntryIter struct {
	b     []byte
	count int
	i     int
	cur   NfsCbResop4Entry
}

func (it *NfsCbResop4EntryIter) Next() bool {
	if it.i >= it.count {
		return false
	}
	var ok bool
	it.cur, ok = readNfsCbResop4Entry(&it.b)
	if !ok {
		return false
	}
	it.i++
	return true
}

func (it *NfsCbResop4EntryIter) Resarray() NfsCbResop4Entry {
	return it.cur
}

// CBCOMPOUND4resWriter writes a CB_COMPOUND4res:
//
//	status + tag + resarray_count + nfs_cb_resop4_entry resarray[resarray_count]
type CBCOMPOUND4resWriter struct {
	buf              []byte
	off              int
	resarrayCount    uint32
	resarrayCountOff int
	phase            uint8
}

func StartCBCOMPOUND4res(buf []byte) CBCOMPOUND4resWriter {
	off := len(buf)
	buf = append(buf, make([]byte, 4)...) // status(4)
	return CBCOMPOUND4resWriter{buf: buf, off: off}
}

func (w *CBCOMPOUND4resWriter) SetStatus(v uint32) {
	binary.BigEndian.PutUint32(w.buf[w.off:w.off+4], v)
}

func (w *CBCOMPOUND4resWriter) AppendResarray_Getattr() CBGETATTR4resEntryWriter {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = binary.BigEndian.AppendUint32(w.buf, OP_CB_GETATTR)
	w.resarrayCount++
	child := StartCBGETATTR4resEntry(w.buf)
	w.buf = nil
	return child
}

func (w *CBCOMPOUND4resWriter) AppendResarray_Recall() CBRECALL4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+cBRECALL4resSize)...)
	p := (*[4 + cBRECALL4resSize]byte)(w.buf[len(w.buf)-4-cBRECALL4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_CB_RECALL)
	w.resarrayCount++
	return CBRECALL4res{m: (*[cBRECALL4resSize]byte)(p[4:])}
}

func (w *CBCOMPOUND4resWriter) AppendResarray_Illegal() CBILLEGAL4res {
	_ = w.buf[:1]
	if w.phase > 1 {
		panic("writer fields called out of order")
	}
	w.phase = 1
	if w.resarrayCountOff == 0 {
		w.resarrayCountOff = len(w.buf)
		w.buf = binary.BigEndian.AppendUint32(w.buf, 0)
	}
	w.buf = append(w.buf, make([]byte, 4+cBILLEGAL4resSize)...)
	p := (*[4 + cBILLEGAL4resSize]byte)(w.buf[len(w.buf)-4-cBILLEGAL4resSize:])
	binary.BigEndian.PutUint32(p[:4], OP_CB_ILLEGAL)
	w.resarrayCount++
	return CBILLEGAL4res{m: (*[cBILLEGAL4resSize]byte)(p[4:])}
}

func (w *CBCOMPOUND4resWriter) StartTag() Utf8strCsWriter {
	if w.phase > 0 {
		panic("writer fields called out of order")
	}
	child := StartUtf8strCs(w.buf)
	w.buf = nil
	return child
}

func (w *CBCOMPOUND4resWriter) Resume(buf []byte) {
	w.buf = buf
}

func (w *CBCOMPOUND4resWriter) Finish() []byte {
	binary.BigEndian.PutUint32(w.buf[w.resarrayCountOff:w.resarrayCountOff+4], w.resarrayCount)
	return w.buf
}

// -------------------------------------------------------
// Pretty-printing
// -------------------------------------------------------

func NfsFtype4Name(v uint32) string {
	switch v {
	case NF4REG:
		return "NF4REG"
	case NF4DIR:
		return "NF4DIR"
	case NF4BLK:
		return "NF4BLK"
	case NF4CHR:
		return "NF4CHR"
	case NF4LNK:
		return "NF4LNK"
	case NF4SOCK:
		return "NF4SOCK"
	case NF4FIFO:
		return "NF4FIFO"
	case NF4ATTRDIR:
		return "NF4ATTRDIR"
	case NF4NAMEDATTR:
		return "NF4NAMEDATTR"
	default:
		return fmt.Sprintf("nfs_ftype4(%v)", v)
	}
}

func Nfsstat4Name(v uint32) string {
	switch v {
	case NFS4_OK:
		return "NFS4_OK"
	case NFS4ERR_PERM:
		return "NFS4ERR_PERM"
	case NFS4ERR_NOENT:
		return "NFS4ERR_NOENT"
	case NFS4ERR_IO:
		return "NFS4ERR_IO"
	case NFS4ERR_NXIO:
		return "NFS4ERR_NXIO"
	case NFS4ERR_ACCESS:
		return "NFS4ERR_ACCESS"
	case NFS4ERR_EXIST:
		return "NFS4ERR_EXIST"
	case NFS4ERR_XDEV:
		return "NFS4ERR_XDEV"
	case NFS4ERR_NOTDIR:
		return "NFS4ERR_NOTDIR"
	case NFS4ERR_ISDIR:
		return "NFS4ERR_ISDIR"
	case NFS4ERR_INVAL:
		return "NFS4ERR_INVAL"
	case NFS4ERR_FBIG:
		return "NFS4ERR_FBIG"
	case NFS4ERR_NOSPC:
		return "NFS4ERR_NOSPC"
	case NFS4ERR_ROFS:
		return "NFS4ERR_ROFS"
	case NFS4ERR_MLINK:
		return "NFS4ERR_MLINK"
	case NFS4ERR_NAMETOOLONG:
		return "NFS4ERR_NAMETOOLONG"
	case NFS4ERR_NOTEMPTY:
		return "NFS4ERR_NOTEMPTY"
	case NFS4ERR_DQUOT:
		return "NFS4ERR_DQUOT"
	case NFS4ERR_STALE:
		return "NFS4ERR_STALE"
	case NFS4ERR_BADHANDLE:
		return "NFS4ERR_BADHANDLE"
	case NFS4ERR_BAD_COOKIE:
		return "NFS4ERR_BAD_COOKIE"
	case NFS4ERR_NOTSUPP:
		return "NFS4ERR_NOTSUPP"
	case NFS4ERR_TOOSMALL:
		return "NFS4ERR_TOOSMALL"
	case NFS4ERR_SERVERFAULT:
		return "NFS4ERR_SERVERFAULT"
	case NFS4ERR_BADTYPE:
		return "NFS4ERR_BADTYPE"
	case NFS4ERR_DELAY:
		return "NFS4ERR_DELAY"
	case NFS4ERR_SAME:
		return "NFS4ERR_SAME"
	case NFS4ERR_DENIED:
		return "NFS4ERR_DENIED"
	case NFS4ERR_EXPIRED:
		return "NFS4ERR_EXPIRED"
	case NFS4ERR_LOCKED:
		return "NFS4ERR_LOCKED"
	case NFS4ERR_GRACE:
		return "NFS4ERR_GRACE"
	case NFS4ERR_FHEXPIRED:
		return "NFS4ERR_FHEXPIRED"
	case NFS4ERR_SHARE_DENIED:
		return "NFS4ERR_SHARE_DENIED"
	case NFS4ERR_WRONGSEC:
		return "NFS4ERR_WRONGSEC"
	case NFS4ERR_CLID_INUSE:
		return "NFS4ERR_CLID_INUSE"
	case NFS4ERR_RESOURCE:
		return "NFS4ERR_RESOURCE"
	case NFS4ERR_MOVED:
		return "NFS4ERR_MOVED"
	case NFS4ERR_NOFILEHANDLE:
		return "NFS4ERR_NOFILEHANDLE"
	case NFS4ERR_MINOR_VERS_MISMATCH:
		return "NFS4ERR_MINOR_VERS_MISMATCH"
	case NFS4ERR_STALE_CLIENTID:
		return "NFS4ERR_STALE_CLIENTID"
	case NFS4ERR_STALE_STATEID:
		return "NFS4ERR_STALE_STATEID"
	case NFS4ERR_OLD_STATEID:
		return "NFS4ERR_OLD_STATEID"
	case NFS4ERR_BAD_STATEID:
		return "NFS4ERR_BAD_STATEID"
	case NFS4ERR_BAD_SEQID:
		return "NFS4ERR_BAD_SEQID"
	case NFS4ERR_NOT_SAME:
		return "NFS4ERR_NOT_SAME"
	case NFS4ERR_LOCK_RANGE:
		return "NFS4ERR_LOCK_RANGE"
	case NFS4ERR_SYMLINK:
		return "NFS4ERR_SYMLINK"
	case NFS4ERR_RESTOREFH:
		return "NFS4ERR_RESTOREFH"
	case NFS4ERR_LEASE_MOVED:
		return "NFS4ERR_LEASE_MOVED"
	case NFS4ERR_ATTRNOTSUPP:
		return "NFS4ERR_ATTRNOTSUPP"
	case NFS4ERR_NO_GRACE:
		return "NFS4ERR_NO_GRACE"
	case NFS4ERR_RECLAIM_BAD:
		return "NFS4ERR_RECLAIM_BAD"
	case NFS4ERR_RECLAIM_CONFLICT:
		return "NFS4ERR_RECLAIM_CONFLICT"
	case NFS4ERR_BADXDR:
		return "NFS4ERR_BADXDR"
	case NFS4ERR_LOCKS_HELD:
		return "NFS4ERR_LOCKS_HELD"
	case NFS4ERR_OPENMODE:
		return "NFS4ERR_OPENMODE"
	case NFS4ERR_BADOWNER:
		return "NFS4ERR_BADOWNER"
	case NFS4ERR_BADCHAR:
		return "NFS4ERR_BADCHAR"
	case NFS4ERR_BADNAME:
		return "NFS4ERR_BADNAME"
	case NFS4ERR_BAD_RANGE:
		return "NFS4ERR_BAD_RANGE"
	case NFS4ERR_LOCK_NOTSUPP:
		return "NFS4ERR_LOCK_NOTSUPP"
	case NFS4ERR_OP_ILLEGAL:
		return "NFS4ERR_OP_ILLEGAL"
	case NFS4ERR_DEADLOCK:
		return "NFS4ERR_DEADLOCK"
	case NFS4ERR_FILE_OPEN:
		return "NFS4ERR_FILE_OPEN"
	case NFS4ERR_ADMIN_REVOKED:
		return "NFS4ERR_ADMIN_REVOKED"
	case NFS4ERR_CB_PATH_DOWN:
		return "NFS4ERR_CB_PATH_DOWN"
	default:
		return fmt.Sprintf("nfsstat4(%v)", v)
	}
}

func TimeHow4Name(v uint32) string {
	switch v {
	case SET_TO_SERVER_TIME4:
		return "SET_TO_SERVER_TIME4"
	case SET_TO_CLIENT_TIME4:
		return "SET_TO_CLIENT_TIME4"
	default:
		return fmt.Sprintf("time_how4(%v)", v)
	}
}

func NfsLockType4Name(v uint32) string {
	switch v {
	case READ_LT:
		return "READ_LT"
	case WRITE_LT:
		return "WRITE_LT"
	case READW_LT:
		return "READW_LT"
	case WRITEW_LT:
		return "WRITEW_LT"
	default:
		return fmt.Sprintf("nfs_lock_type4(%v)", v)
	}
}

func XdrBoolName(v uint32) string {
	switch v {
	case FALSE:
		return "FALSE"
	case TRUE:
		return "TRUE"
	default:
		return fmt.Sprintf("xdr_bool(%v)", v)
	}
}

func Createmode4Name(v uint32) string {
	switch v {
	case UNCHECKED4:
		return "UNCHECKED4"
	case GUARDED4:
		return "GUARDED4"
	case EXCLUSIVE4:
		return "EXCLUSIVE4"
	default:
		return fmt.Sprintf("createmode4(%v)", v)
	}
}

func Opentype4Name(v uint32) string {
	switch v {
	case OPEN4_NOCREATE:
		return "OPEN4_NOCREATE"
	case OPEN4_CREATE:
		return "OPEN4_CREATE"
	default:
		return fmt.Sprintf("opentype4(%v)", v)
	}
}

func LimitBy4Name(v uint32) string {
	switch v {
	case NFS_LIMIT_SIZE:
		return "NFS_LIMIT_SIZE"
	case NFS_LIMIT_BLOCKS:
		return "NFS_LIMIT_BLOCKS"
	default:
		return fmt.Sprintf("limit_by4(%v)", v)
	}
}

func OpenDelegationType4Name(v uint32) string {
	switch v {
	case OPEN_DELEGATE_NONE:
		return "OPEN_DELEGATE_NONE"
	case OPEN_DELEGATE_READ:
		return "OPEN_DELEGATE_READ"
	case OPEN_DELEGATE_WRITE:
		return "OPEN_DELEGATE_WRITE"
	default:
		return fmt.Sprintf("open_delegation_type4(%v)", v)
	}
}

func OpenClaimType4Name(v uint32) string {
	switch v {
	case CLAIM_NULL:
		return "CLAIM_NULL"
	case CLAIM_PREVIOUS:
		return "CLAIM_PREVIOUS"
	case CLAIM_DELEGATE_CUR:
		return "CLAIM_DELEGATE_CUR"
	case CLAIM_DELEGATE_PREV:
		return "CLAIM_DELEGATE_PREV"
	default:
		return fmt.Sprintf("open_claim_type4(%v)", v)
	}
}

func RpcGssSvcTName(v uint32) string {
	switch v {
	case RPC_GSS_SVC_NONE:
		return "RPC_GSS_SVC_NONE"
	case RPC_GSS_SVC_INTEGRITY:
		return "RPC_GSS_SVC_INTEGRITY"
	case RPC_GSS_SVC_PRIVACY:
		return "RPC_GSS_SVC_PRIVACY"
	default:
		return fmt.Sprintf("rpc_gss_svc_t(%v)", v)
	}
}

func DiscSecinfo4Name(v uint32) string {
	switch v {
	case RPCSEC_GSS:
		return "RPCSEC_GSS"
	default:
		return fmt.Sprintf("disc_secinfo4(%v)", v)
	}
}

func StableHow4Name(v uint32) string {
	switch v {
	case UNSTABLE4:
		return "UNSTABLE4"
	case DATA_SYNC4:
		return "DATA_SYNC4"
	case FILE_SYNC4:
		return "FILE_SYNC4"
	default:
		return fmt.Sprintf("stable_how4(%v)", v)
	}
}

func NfsOpnum4Name(v uint32) string {
	switch v {
	case OP_ACCESS:
		return "OP_ACCESS"
	case OP_CLOSE:
		return "OP_CLOSE"
	case OP_COMMIT:
		return "OP_COMMIT"
	case OP_CREATE:
		return "OP_CREATE"
	case OP_DELEGPURGE:
		return "OP_DELEGPURGE"
	case OP_DELEGRETURN:
		return "OP_DELEGRETURN"
	case OP_GETATTR:
		return "OP_GETATTR"
	case OP_GETFH:
		return "OP_GETFH"
	case OP_LINK:
		return "OP_LINK"
	case OP_LOCK:
		return "OP_LOCK"
	case OP_LOCKT:
		return "OP_LOCKT"
	case OP_LOCKU:
		return "OP_LOCKU"
	case OP_LOOKUP:
		return "OP_LOOKUP"
	case OP_LOOKUPP:
		return "OP_LOOKUPP"
	case OP_NVERIFY:
		return "OP_NVERIFY"
	case OP_OPEN:
		return "OP_OPEN"
	case OP_OPENATTR:
		return "OP_OPENATTR"
	case OP_OPEN_CONFIRM:
		return "OP_OPEN_CONFIRM"
	case OP_OPEN_DOWNGRADE:
		return "OP_OPEN_DOWNGRADE"
	case OP_PUTFH:
		return "OP_PUTFH"
	case OP_PUTPUBFH:
		return "OP_PUTPUBFH"
	case OP_PUTROOTFH:
		return "OP_PUTROOTFH"
	case OP_READ:
		return "OP_READ"
	case OP_READDIR:
		return "OP_READDIR"
	case OP_READLINK:
		return "OP_READLINK"
	case OP_REMOVE:
		return "OP_REMOVE"
	case OP_RENAME:
		return "OP_RENAME"
	case OP_RENEW:
		return "OP_RENEW"
	case OP_RESTOREFH:
		return "OP_RESTOREFH"
	case OP_SAVEFH:
		return "OP_SAVEFH"
	case OP_SECINFO:
		return "OP_SECINFO"
	case OP_SETATTR:
		return "OP_SETATTR"
	case OP_SETCLIENTID:
		return "OP_SETCLIENTID"
	case OP_SETCLIENTID_CONFIRM:
		return "OP_SETCLIENTID_CONFIRM"
	case OP_VERIFY:
		return "OP_VERIFY"
	case OP_WRITE:
		return "OP_WRITE"
	case OP_RELEASE_LOCKOWNER:
		return "OP_RELEASE_LOCKOWNER"
	case OP_ILLEGAL:
		return "OP_ILLEGAL"
	default:
		return fmt.Sprintf("nfs_opnum4(%v)", v)
	}
}

func NfsCbOpnum4Name(v uint32) string {
	switch v {
	case OP_CB_GETATTR:
		return "OP_CB_GETATTR"
	case OP_CB_RECALL:
		return "OP_CB_RECALL"
	case OP_CB_ILLEGAL:
		return "OP_CB_ILLEGAL"
	default:
		return fmt.Sprintf("nfs_cb_opnum4(%v)", v)
	}
}

func (m Attrlist4) String() string {
	var b strings.Builder
	b.WriteString("Attrlist4{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Bitmap4) String() string {
	var b strings.Builder
	b.WriteString("Bitmap4{")
	{
		n := int(m.Count())
		fmt.Fprintf(&b, "data: [")
		for i := 0; i < n && i < 64; i++ {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%d", m.Data(i))
		}
		if n > 64 {
			fmt.Fprintf(&b, ", ...(%d total)", n)
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m NfsFh4) String() string {
	var b strings.Builder
	b.WriteString("NfsFh4{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m SecOid4) String() string {
	var b strings.Builder
	b.WriteString("SecOid4{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Utf8strCis) String() string {
	var b strings.Builder
	b.WriteString("Utf8strCis{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Utf8strCs) String() string {
	var b strings.Builder
	b.WriteString("Utf8strCs{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Utf8strMixed) String() string {
	var b strings.Builder
	b.WriteString("Utf8strMixed{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Component4) String() string {
	var b strings.Builder
	b.WriteString("Component4{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Linktext4) String() string {
	var b strings.Builder
	b.WriteString("Linktext4{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Pathname4) String() string {
	var b strings.Builder
	b.WriteString("Pathname4{")
	{
		iter := m.Data()
		fmt.Fprintf(&b, "data: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Data())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m Verifier4) String() string {
	var b strings.Builder
	b.WriteString("Verifier4{")
	fmt.Fprintf(&b, "data: [")
	for i := 0; i < 8; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%d", m.Data(i))
	}
	b.WriteString("]")
	b.WriteString("}")
	return b.String()
}

func (m Nfstime4) String() string {
	var b strings.Builder
	b.WriteString("Nfstime4{")
	fmt.Fprintf(&b, "seconds: %d", m.Seconds())
	fmt.Fprintf(&b, ", nseconds: %d", m.Nseconds())
	b.WriteString("}")
	return b.String()
}

func (m Settime4) String() string {
	switch m.disc {
	case SET_TO_CLIENT_TIME4:
		return fmt.Sprintf("CLIENT_TIME4:%v", m.AsNfstime4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m Fsid4) String() string {
	var b strings.Builder
	b.WriteString("Fsid4{")
	fmt.Fprintf(&b, "major: %d", m.Major())
	fmt.Fprintf(&b, ", minor: %d", m.Minor())
	b.WriteString("}")
	return b.String()
}

func (m FsLocation4) String() string {
	var b strings.Builder
	b.WriteString("FsLocation4{")
	{
		iter := m.Server()
		fmt.Fprintf(&b, "server: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Server())
			i++
		}
		b.WriteString("]")
	}
	fmt.Fprintf(&b, ", rootpath: %v", m.Rootpath())
	b.WriteString("}")
	return b.String()
}

func (m FsLocations4) String() string {
	var b strings.Builder
	b.WriteString("FsLocations4{")
	fmt.Fprintf(&b, "fs_root: %v", m.FsRoot())
	{
		iter := m.Locations()
		fmt.Fprintf(&b, ", locations: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Location())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m Nfsace4) String() string {
	var b strings.Builder
	b.WriteString("Nfsace4{")
	fmt.Fprintf(&b, "type_val: %d", m.TypeVal())
	fmt.Fprintf(&b, ", flag: %d", m.Flag())
	fmt.Fprintf(&b, ", access_mask: %d", m.AccessMask())
	fmt.Fprintf(&b, ", who: %v", m.Who())
	b.WriteString("}")
	return b.String()
}

func (m Specdata4) String() string {
	var b strings.Builder
	b.WriteString("Specdata4{")
	fmt.Fprintf(&b, "specdata1: %d", m.Specdata1())
	fmt.Fprintf(&b, ", specdata2: %d", m.Specdata2())
	b.WriteString("}")
	return b.String()
}

func (m Fattr4) String() string {
	var b strings.Builder
	b.WriteString("Fattr4{")
	fmt.Fprintf(&b, "attrmask: %v", m.Attrmask())
	fmt.Fprintf(&b, ", attr_vals: %v", m.AttrVals())
	b.WriteString("}")
	return b.String()
}

func (m ChangeInfo4) String() string {
	var b strings.Builder
	b.WriteString("ChangeInfo4{")
	fmt.Fprintf(&b, "atomic: %d", m.Atomic())
	fmt.Fprintf(&b, ", before: %d", m.Before())
	fmt.Fprintf(&b, ", after: %d", m.After())
	b.WriteString("}")
	return b.String()
}

func (m XdrOpaque) String() string {
	var b strings.Builder
	b.WriteString("XdrOpaque{")
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, "data: %x", d)
		} else {
			fmt.Fprintf(&b, "data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m Clientaddr4) String() string {
	var b strings.Builder
	b.WriteString("Clientaddr4{")
	fmt.Fprintf(&b, "r_netid: %v", m.RNetid())
	fmt.Fprintf(&b, ", r_addr: %v", m.RAddr())
	b.WriteString("}")
	return b.String()
}

func (m CbClient4) String() string {
	var b strings.Builder
	b.WriteString("CbClient4{")
	fmt.Fprintf(&b, "cb_program: %d", m.CbProgram())
	fmt.Fprintf(&b, ", cb_location: %v", m.CbLocation())
	b.WriteString("}")
	return b.String()
}

func (m Stateid4) String() string {
	var b strings.Builder
	b.WriteString("Stateid4{")
	fmt.Fprintf(&b, "seqid: %d", m.Seqid())
	fmt.Fprintf(&b, ", other: [")
	for i := 0; i < 12; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%d", m.Other(i))
	}
	b.WriteString("]")
	b.WriteString("}")
	return b.String()
}

func (m NfsClientId4) String() string {
	var b strings.Builder
	b.WriteString("NfsClientId4{")
	fmt.Fprintf(&b, "verifier: %v", m.Verifier())
	{
		d := m.Id()
		if len(d) <= 32 {
			fmt.Fprintf(&b, ", id: %x", d)
		} else {
			fmt.Fprintf(&b, ", id: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m OpenOwner4) String() string {
	var b strings.Builder
	b.WriteString("OpenOwner4{")
	fmt.Fprintf(&b, "clientid: %d", m.Clientid())
	{
		d := m.Owner()
		if len(d) <= 32 {
			fmt.Fprintf(&b, ", owner: %x", d)
		} else {
			fmt.Fprintf(&b, ", owner: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m LockOwner4) String() string {
	var b strings.Builder
	b.WriteString("LockOwner4{")
	fmt.Fprintf(&b, "clientid: %d", m.Clientid())
	{
		d := m.Owner()
		if len(d) <= 32 {
			fmt.Fprintf(&b, ", owner: %x", d)
		} else {
			fmt.Fprintf(&b, ", owner: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m ACCESS4args) String() string {
	var b strings.Builder
	b.WriteString("ACCESS4args{")
	fmt.Fprintf(&b, "access: %d", m.Access())
	b.WriteString("}")
	return b.String()
}

func (m ACCESS4resok) String() string {
	var b strings.Builder
	b.WriteString("ACCESS4resok{")
	fmt.Fprintf(&b, "supported: %d", m.Supported())
	fmt.Fprintf(&b, ", access: %d", m.Access())
	b.WriteString("}")
	return b.String()
}

func (m ACCESS4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsACCESS4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m CLOSE4args) String() string {
	var b strings.Builder
	b.WriteString("CLOSE4args{")
	fmt.Fprintf(&b, "seqid: %d", m.Seqid())
	fmt.Fprintf(&b, ", open_stateid: %v", m.OpenStateid())
	b.WriteString("}")
	return b.String()
}

func (m CLOSE4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsStateid4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m COMMIT4args) String() string {
	var b strings.Builder
	b.WriteString("COMMIT4args{")
	fmt.Fprintf(&b, "offset: %d", m.Offset())
	fmt.Fprintf(&b, ", count: %d", m.Count())
	b.WriteString("}")
	return b.String()
}

func (m COMMIT4resok) String() string {
	var b strings.Builder
	b.WriteString("COMMIT4resok{")
	fmt.Fprintf(&b, "writeverf: %v", m.Writeverf())
	b.WriteString("}")
	return b.String()
}

func (m COMMIT4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsCOMMIT4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m Createtype4) String() string {
	switch m.disc {
	case NF4LNK:
		return fmt.Sprintf("NF4LNK:%v", m.AsLinktext4())
	case NF4BLK:
		return fmt.Sprintf("NF4BLK:%v", m.AsNf4blk())
	case NF4CHR:
		return fmt.Sprintf("NF4CHR:%v", m.AsNf4chr())
	case NF4SOCK:
		return "NF4SOCK"
	case NF4FIFO:
		return "NF4FIFO"
	case NF4DIR:
		return "NF4DIR"
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m CREATE4args) String() string {
	var b strings.Builder
	b.WriteString("CREATE4args{")
	fmt.Fprintf(&b, "objtype: %v", m.Objtype())
	fmt.Fprintf(&b, ", objname: %v", m.Objname())
	fmt.Fprintf(&b, ", createattrs: %v", m.Createattrs())
	b.WriteString("}")
	return b.String()
}

func (m CREATE4resok) String() string {
	var b strings.Builder
	b.WriteString("CREATE4resok{")
	fmt.Fprintf(&b, "cinfo: %v", m.Cinfo())
	fmt.Fprintf(&b, ", attrset: %v", m.Attrset())
	b.WriteString("}")
	return b.String()
}

func (m CREATE4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsCREATE4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m DELEGPURGE4args) String() string {
	var b strings.Builder
	b.WriteString("DELEGPURGE4args{")
	fmt.Fprintf(&b, "clientid: %d", m.Clientid())
	b.WriteString("}")
	return b.String()
}

func (m DELEGPURGE4res) String() string {
	var b strings.Builder
	b.WriteString("DELEGPURGE4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m DELEGRETURN4args) String() string {
	var b strings.Builder
	b.WriteString("DELEGRETURN4args{")
	fmt.Fprintf(&b, "deleg_stateid: %v", m.DelegStateid())
	b.WriteString("}")
	return b.String()
}

func (m DELEGRETURN4res) String() string {
	var b strings.Builder
	b.WriteString("DELEGRETURN4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m GETATTR4args) String() string {
	var b strings.Builder
	b.WriteString("GETATTR4args{")
	fmt.Fprintf(&b, "attr_request: %v", m.AttrRequest())
	b.WriteString("}")
	return b.String()
}

func (m GETATTR4resok) String() string {
	var b strings.Builder
	b.WriteString("GETATTR4resok{")
	fmt.Fprintf(&b, "obj_attributes: %v", m.ObjAttributes())
	b.WriteString("}")
	return b.String()
}

func (m GETATTR4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsGETATTR4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m GETFH4resok) String() string {
	var b strings.Builder
	b.WriteString("GETFH4resok{")
	fmt.Fprintf(&b, "object: %v", m.Object())
	b.WriteString("}")
	return b.String()
}

func (m GETFH4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsGETFH4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m LINK4args) String() string {
	var b strings.Builder
	b.WriteString("LINK4args{")
	fmt.Fprintf(&b, "newname: %v", m.Newname())
	b.WriteString("}")
	return b.String()
}

func (m LINK4resok) String() string {
	var b strings.Builder
	b.WriteString("LINK4resok{")
	fmt.Fprintf(&b, "cinfo: %v", m.Cinfo())
	b.WriteString("}")
	return b.String()
}

func (m LINK4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsLINK4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m OpenToLockOwner4) String() string {
	var b strings.Builder
	b.WriteString("OpenToLockOwner4{")
	fmt.Fprintf(&b, "open_seqid: %d", m.OpenSeqid())
	fmt.Fprintf(&b, ", open_stateid: %v", m.OpenStateid())
	fmt.Fprintf(&b, ", lock_seqid: %d", m.LockSeqid())
	fmt.Fprintf(&b, ", lock_owner: %v", m.LockOwner())
	b.WriteString("}")
	return b.String()
}

func (m ExistLockOwner4) String() string {
	var b strings.Builder
	b.WriteString("ExistLockOwner4{")
	fmt.Fprintf(&b, "lock_stateid: %v", m.LockStateid())
	fmt.Fprintf(&b, ", lock_seqid: %d", m.LockSeqid())
	b.WriteString("}")
	return b.String()
}

func (m Locker4) String() string {
	switch m.disc {
	case TRUE:
		return fmt.Sprintf("TRUE:%v", m.AsOpenToLockOwner4())
	case FALSE:
		return fmt.Sprintf("FALSE:%v", m.AsExistLockOwner4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m LOCK4args) String() string {
	var b strings.Builder
	b.WriteString("LOCK4args{")
	fmt.Fprintf(&b, "locktype: %s", NfsLockType4Name(m.Locktype()))
	fmt.Fprintf(&b, ", reclaim: %d", m.Reclaim())
	fmt.Fprintf(&b, ", offset: %d", m.Offset())
	fmt.Fprintf(&b, ", length: %d", m.Length())
	fmt.Fprintf(&b, ", locker: %v", m.Locker())
	b.WriteString("}")
	return b.String()
}

func (m LOCK4denied) String() string {
	var b strings.Builder
	b.WriteString("LOCK4denied{")
	fmt.Fprintf(&b, "offset: %d", m.Offset())
	fmt.Fprintf(&b, ", length: %d", m.Length())
	fmt.Fprintf(&b, ", locktype: %s", NfsLockType4Name(m.Locktype()))
	fmt.Fprintf(&b, ", owner: %v", m.Owner())
	b.WriteString("}")
	return b.String()
}

func (m LOCK4resok) String() string {
	var b strings.Builder
	b.WriteString("LOCK4resok{")
	fmt.Fprintf(&b, "lock_stateid: %v", m.LockStateid())
	b.WriteString("}")
	return b.String()
}

func (m LOCK4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsLOCK4resok())
	case NFS4ERR_DENIED:
		return fmt.Sprintf("NFS4ERR_DENIED:%v", m.AsLOCK4denied())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m LOCKT4args) String() string {
	var b strings.Builder
	b.WriteString("LOCKT4args{")
	fmt.Fprintf(&b, "locktype: %s", NfsLockType4Name(m.Locktype()))
	fmt.Fprintf(&b, ", offset: %d", m.Offset())
	fmt.Fprintf(&b, ", length: %d", m.Length())
	fmt.Fprintf(&b, ", owner: %v", m.Owner())
	b.WriteString("}")
	return b.String()
}

func (m LOCKT4res) String() string {
	switch m.disc {
	case NFS4ERR_DENIED:
		return fmt.Sprintf("NFS4ERR_DENIED:%v", m.AsLOCK4denied())
	case NFS4_OK:
		return "NFS4_OK"
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m LOCKU4args) String() string {
	var b strings.Builder
	b.WriteString("LOCKU4args{")
	fmt.Fprintf(&b, "locktype: %s", NfsLockType4Name(m.Locktype()))
	fmt.Fprintf(&b, ", seqid: %d", m.Seqid())
	fmt.Fprintf(&b, ", lock_stateid: %v", m.LockStateid())
	fmt.Fprintf(&b, ", offset: %d", m.Offset())
	fmt.Fprintf(&b, ", length: %d", m.Length())
	b.WriteString("}")
	return b.String()
}

func (m LOCKU4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsStateid4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m LOOKUP4args) String() string {
	var b strings.Builder
	b.WriteString("LOOKUP4args{")
	fmt.Fprintf(&b, "objname: %v", m.Objname())
	b.WriteString("}")
	return b.String()
}

func (m LOOKUP4res) String() string {
	var b strings.Builder
	b.WriteString("LOOKUP4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m LOOKUPP4res) String() string {
	var b strings.Builder
	b.WriteString("LOOKUPP4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m NVERIFY4args) String() string {
	var b strings.Builder
	b.WriteString("NVERIFY4args{")
	fmt.Fprintf(&b, "obj_attributes: %v", m.ObjAttributes())
	b.WriteString("}")
	return b.String()
}

func (m NVERIFY4res) String() string {
	var b strings.Builder
	b.WriteString("NVERIFY4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m Createhow4) String() string {
	switch m.disc {
	case UNCHECKED4:
		return fmt.Sprintf("UNCHECKED4:%v", m.AsUnchecked4())
	case GUARDED4:
		return fmt.Sprintf("GUARDED4:%v", m.AsGuarded4())
	case EXCLUSIVE4:
		return fmt.Sprintf("EXCLUSIVE4:%v", m.AsVerifier4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m Createhow4Entry) String() string {
	var b strings.Builder
	b.WriteString("Createhow4Entry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m Openflag4) String() string {
	switch m.disc {
	case OPEN4_CREATE:
		return fmt.Sprintf("CREATE:%v", m.AsCreatehow4Entry())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m NfsModifiedLimit4) String() string {
	var b strings.Builder
	b.WriteString("NfsModifiedLimit4{")
	fmt.Fprintf(&b, "num_blocks: %d", m.NumBlocks())
	fmt.Fprintf(&b, ", bytes_per_block: %d", m.BytesPerBlock())
	b.WriteString("}")
	return b.String()
}

func (m NfsSpaceLimit4NFSLIMITSIZE) String() string {
	var b strings.Builder
	b.WriteString("NfsSpaceLimit4NFSLIMITSIZE{")
	fmt.Fprintf(&b, "value: %d", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m NfsSpaceLimit4) String() string {
	switch m.disc {
	case NFS_LIMIT_SIZE:
		return fmt.Sprintf("SIZE:%v", m.AsNfsSpaceLimit4NFSLIMITSIZE())
	case NFS_LIMIT_BLOCKS:
		return fmt.Sprintf("BLOCKS:%v", m.AsNfsModifiedLimit4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m OpenClaimDelegateCur4) String() string {
	var b strings.Builder
	b.WriteString("OpenClaimDelegateCur4{")
	fmt.Fprintf(&b, "delegate_stateid: %v", m.DelegateStateid())
	fmt.Fprintf(&b, ", file: %v", m.File())
	b.WriteString("}")
	return b.String()
}

func (m OpenClaim4CLAIMPREVIOUS) String() string {
	var b strings.Builder
	b.WriteString("OpenClaim4CLAIMPREVIOUS{")
	fmt.Fprintf(&b, "value: %s", OpenDelegationType4Name(m.Value()))
	b.WriteString("}")
	return b.String()
}

func (m OpenClaim4) String() string {
	switch m.disc {
	case CLAIM_NULL:
		return fmt.Sprintf("NULL:%v", m.AsNull())
	case CLAIM_PREVIOUS:
		return fmt.Sprintf("PREVIOUS:%v", m.AsOpenClaim4CLAIMPREVIOUS())
	case CLAIM_DELEGATE_CUR:
		return fmt.Sprintf("DELEGATE_CUR:%v", m.AsOpenClaimDelegateCur4())
	case CLAIM_DELEGATE_PREV:
		return fmt.Sprintf("DELEGATE_PREV:%v", m.AsDelegatePrev())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m OPEN4args) String() string {
	var b strings.Builder
	b.WriteString("OPEN4args{")
	fmt.Fprintf(&b, "seqid: %d", m.Seqid())
	fmt.Fprintf(&b, ", share_access: %d", m.ShareAccess())
	fmt.Fprintf(&b, ", share_deny: %d", m.ShareDeny())
	fmt.Fprintf(&b, ", owner: %v", m.Owner())
	fmt.Fprintf(&b, ", openhow: %v", m.Openhow())
	fmt.Fprintf(&b, ", claim: %v", m.Claim())
	b.WriteString("}")
	return b.String()
}

func (m OpenReadDelegation4) String() string {
	var b strings.Builder
	b.WriteString("OpenReadDelegation4{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", recall: %d", m.Recall())
	fmt.Fprintf(&b, ", permissions: %v", m.Permissions())
	b.WriteString("}")
	return b.String()
}

func (m OpenWriteDelegation4) String() string {
	var b strings.Builder
	b.WriteString("OpenWriteDelegation4{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", recall: %d", m.Recall())
	fmt.Fprintf(&b, ", space_limit: %v", m.SpaceLimit())
	fmt.Fprintf(&b, ", permissions: %v", m.Permissions())
	b.WriteString("}")
	return b.String()
}

func (m OpenDelegation4) String() string {
	switch m.disc {
	case OPEN_DELEGATE_NONE:
		return "NONE"
	case OPEN_DELEGATE_READ:
		return fmt.Sprintf("READ:%v", m.AsOpenReadDelegation4())
	case OPEN_DELEGATE_WRITE:
		return fmt.Sprintf("WRITE:%v", m.AsOpenWriteDelegation4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m OPEN4resok) String() string {
	var b strings.Builder
	b.WriteString("OPEN4resok{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", cinfo: %v", m.Cinfo())
	fmt.Fprintf(&b, ", rflags: %d", m.Rflags())
	fmt.Fprintf(&b, ", attrset: %v", m.Attrset())
	fmt.Fprintf(&b, ", delegation: %v", m.Delegation())
	b.WriteString("}")
	return b.String()
}

func (m OPEN4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsOPEN4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m OPENATTR4args) String() string {
	var b strings.Builder
	b.WriteString("OPENATTR4args{")
	fmt.Fprintf(&b, "createdir: %d", m.Createdir())
	b.WriteString("}")
	return b.String()
}

func (m OPENATTR4res) String() string {
	var b strings.Builder
	b.WriteString("OPENATTR4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m OPENCONFIRM4args) String() string {
	var b strings.Builder
	b.WriteString("OPENCONFIRM4args{")
	fmt.Fprintf(&b, "open_stateid: %v", m.OpenStateid())
	fmt.Fprintf(&b, ", seqid: %d", m.Seqid())
	b.WriteString("}")
	return b.String()
}

func (m OPENCONFIRM4resok) String() string {
	var b strings.Builder
	b.WriteString("OPENCONFIRM4resok{")
	fmt.Fprintf(&b, "open_stateid: %v", m.OpenStateid())
	b.WriteString("}")
	return b.String()
}

func (m OPENCONFIRM4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsOPENCONFIRM4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m OPENDOWNGRADE4args) String() string {
	var b strings.Builder
	b.WriteString("OPENDOWNGRADE4args{")
	fmt.Fprintf(&b, "open_stateid: %v", m.OpenStateid())
	fmt.Fprintf(&b, ", seqid: %d", m.Seqid())
	fmt.Fprintf(&b, ", share_access: %d", m.ShareAccess())
	fmt.Fprintf(&b, ", share_deny: %d", m.ShareDeny())
	b.WriteString("}")
	return b.String()
}

func (m OPENDOWNGRADE4resok) String() string {
	var b strings.Builder
	b.WriteString("OPENDOWNGRADE4resok{")
	fmt.Fprintf(&b, "open_stateid: %v", m.OpenStateid())
	b.WriteString("}")
	return b.String()
}

func (m OPENDOWNGRADE4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsOPENDOWNGRADE4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m PUTFH4args) String() string {
	var b strings.Builder
	b.WriteString("PUTFH4args{")
	fmt.Fprintf(&b, "object: %v", m.Object())
	b.WriteString("}")
	return b.String()
}

func (m PUTFH4res) String() string {
	var b strings.Builder
	b.WriteString("PUTFH4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m PUTPUBFH4res) String() string {
	var b strings.Builder
	b.WriteString("PUTPUBFH4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m PUTROOTFH4res) String() string {
	var b strings.Builder
	b.WriteString("PUTROOTFH4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m READ4args) String() string {
	var b strings.Builder
	b.WriteString("READ4args{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", offset: %d", m.Offset())
	fmt.Fprintf(&b, ", count: %d", m.Count())
	b.WriteString("}")
	return b.String()
}

func (m READ4resok) String() string {
	var b strings.Builder
	b.WriteString("READ4resok{")
	fmt.Fprintf(&b, "eof: %d", m.Eof())
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, ", data: %x", d)
		} else {
			fmt.Fprintf(&b, ", data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m READ4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsREAD4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m READDIR4args) String() string {
	var b strings.Builder
	b.WriteString("READDIR4args{")
	fmt.Fprintf(&b, "cookie: %d", m.Cookie())
	fmt.Fprintf(&b, ", cookieverf: %v", m.Cookieverf())
	fmt.Fprintf(&b, ", dircount: %d", m.Dircount())
	fmt.Fprintf(&b, ", maxcount: %d", m.Maxcount())
	fmt.Fprintf(&b, ", attr_request: %v", m.AttrRequest())
	b.WriteString("}")
	return b.String()
}

func (m Entry4Opt) String() string {
	switch m.disc {
	case TRUE:
		return fmt.Sprintf("TRUE:%v", m.AsEntry4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m Entry4) String() string {
	var b strings.Builder
	b.WriteString("Entry4{")
	fmt.Fprintf(&b, "cookie: %d", m.Cookie())
	fmt.Fprintf(&b, ", name: %v", m.Name())
	fmt.Fprintf(&b, ", attrs: %v", m.Attrs())
	fmt.Fprintf(&b, ", nextentry: %v", m.Nextentry())
	b.WriteString("}")
	return b.String()
}

func (m Dirlist4) String() string {
	var b strings.Builder
	b.WriteString("Dirlist4{")
	fmt.Fprintf(&b, "entries: %v", m.Entries())
	fmt.Fprintf(&b, ", eof: %d", m.Eof())
	b.WriteString("}")
	return b.String()
}

func (m READDIR4resok) String() string {
	var b strings.Builder
	b.WriteString("READDIR4resok{")
	fmt.Fprintf(&b, "cookieverf: %v", m.Cookieverf())
	fmt.Fprintf(&b, ", reply: %v", m.Reply())
	b.WriteString("}")
	return b.String()
}

func (m READDIR4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsREADDIR4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m READLINK4resok) String() string {
	var b strings.Builder
	b.WriteString("READLINK4resok{")
	fmt.Fprintf(&b, "link: %v", m.Link())
	b.WriteString("}")
	return b.String()
}

func (m READLINK4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsREADLINK4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m REMOVE4args) String() string {
	var b strings.Builder
	b.WriteString("REMOVE4args{")
	fmt.Fprintf(&b, "target: %v", m.Target())
	b.WriteString("}")
	return b.String()
}

func (m REMOVE4resok) String() string {
	var b strings.Builder
	b.WriteString("REMOVE4resok{")
	fmt.Fprintf(&b, "cinfo: %v", m.Cinfo())
	b.WriteString("}")
	return b.String()
}

func (m REMOVE4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsREMOVE4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m RENAME4args) String() string {
	var b strings.Builder
	b.WriteString("RENAME4args{")
	fmt.Fprintf(&b, "oldname: %v", m.Oldname())
	fmt.Fprintf(&b, ", newname: %v", m.Newname())
	b.WriteString("}")
	return b.String()
}

func (m RENAME4resok) String() string {
	var b strings.Builder
	b.WriteString("RENAME4resok{")
	fmt.Fprintf(&b, "source_cinfo: %v", m.SourceCinfo())
	fmt.Fprintf(&b, ", target_cinfo: %v", m.TargetCinfo())
	b.WriteString("}")
	return b.String()
}

func (m RENAME4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsRENAME4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m RENEW4args) String() string {
	var b strings.Builder
	b.WriteString("RENEW4args{")
	fmt.Fprintf(&b, "clientid: %d", m.Clientid())
	b.WriteString("}")
	return b.String()
}

func (m RENEW4res) String() string {
	var b strings.Builder
	b.WriteString("RENEW4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m RESTOREFH4res) String() string {
	var b strings.Builder
	b.WriteString("RESTOREFH4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m SAVEFH4res) String() string {
	var b strings.Builder
	b.WriteString("SAVEFH4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m SECINFO4args) String() string {
	var b strings.Builder
	b.WriteString("SECINFO4args{")
	fmt.Fprintf(&b, "name: %v", m.Name())
	b.WriteString("}")
	return b.String()
}

func (m RpcsecGssInfo) String() string {
	var b strings.Builder
	b.WriteString("RpcsecGssInfo{")
	fmt.Fprintf(&b, "oid: %v", m.Oid())
	fmt.Fprintf(&b, ", qop: %d", m.Qop())
	fmt.Fprintf(&b, ", service: %s", RpcGssSvcTName(m.Service()))
	b.WriteString("}")
	return b.String()
}

func (m Secinfo4) String() string {
	switch m.disc {
	case RPCSEC_GSS:
		return fmt.Sprintf("GSS:%v", m.AsRpcsecGssInfo())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m Secinfo4Entry) String() string {
	var b strings.Builder
	b.WriteString("Secinfo4Entry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m SECINFO4resok) String() string {
	var b strings.Builder
	b.WriteString("SECINFO4resok{")
	{
		iter := m.Data()
		fmt.Fprintf(&b, "data: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Data())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m SECINFO4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsSECINFO4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m SETATTR4args) String() string {
	var b strings.Builder
	b.WriteString("SETATTR4args{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", obj_attributes: %v", m.ObjAttributes())
	b.WriteString("}")
	return b.String()
}

func (m SETATTR4res) String() string {
	var b strings.Builder
	b.WriteString("SETATTR4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	fmt.Fprintf(&b, ", attrsset: %v", m.Attrsset())
	b.WriteString("}")
	return b.String()
}

func (m SETCLIENTID4args) String() string {
	var b strings.Builder
	b.WriteString("SETCLIENTID4args{")
	fmt.Fprintf(&b, "client: %v", m.Client())
	fmt.Fprintf(&b, ", callback: %v", m.Callback())
	fmt.Fprintf(&b, ", callback_ident: %d", m.CallbackIdent())
	b.WriteString("}")
	return b.String()
}

func (m SETCLIENTID4resok) String() string {
	var b strings.Builder
	b.WriteString("SETCLIENTID4resok{")
	fmt.Fprintf(&b, "clientid: %d", m.Clientid())
	fmt.Fprintf(&b, ", setclientid_confirm: %v", m.SetclientidConfirm())
	b.WriteString("}")
	return b.String()
}

func (m SETCLIENTID4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsSETCLIENTID4resok())
	case NFS4ERR_CLID_INUSE:
		return fmt.Sprintf("NFS4ERR_CLID_INUSE:%v", m.AsClientaddr4())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m SETCLIENTIDCONFIRM4args) String() string {
	var b strings.Builder
	b.WriteString("SETCLIENTIDCONFIRM4args{")
	fmt.Fprintf(&b, "clientid: %d", m.Clientid())
	fmt.Fprintf(&b, ", setclientid_confirm: %v", m.SetclientidConfirm())
	b.WriteString("}")
	return b.String()
}

func (m SETCLIENTIDCONFIRM4res) String() string {
	var b strings.Builder
	b.WriteString("SETCLIENTIDCONFIRM4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m VERIFY4args) String() string {
	var b strings.Builder
	b.WriteString("VERIFY4args{")
	fmt.Fprintf(&b, "obj_attributes: %v", m.ObjAttributes())
	b.WriteString("}")
	return b.String()
}

func (m VERIFY4res) String() string {
	var b strings.Builder
	b.WriteString("VERIFY4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m WRITE4args) String() string {
	var b strings.Builder
	b.WriteString("WRITE4args{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", offset: %d", m.Offset())
	fmt.Fprintf(&b, ", stable: %s", StableHow4Name(m.Stable()))
	{
		d := m.Data()
		if len(d) <= 32 {
			fmt.Fprintf(&b, ", data: %x", d)
		} else {
			fmt.Fprintf(&b, ", data: %x...(%d bytes)", d[:32], len(d))
		}
	}
	b.WriteString("}")
	return b.String()
}

func (m WRITE4resok) String() string {
	var b strings.Builder
	b.WriteString("WRITE4resok{")
	fmt.Fprintf(&b, "count: %d", m.Count())
	fmt.Fprintf(&b, ", committed: %s", StableHow4Name(m.Committed()))
	fmt.Fprintf(&b, ", writeverf: %v", m.Writeverf())
	b.WriteString("}")
	return b.String()
}

func (m WRITE4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsWRITE4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m RELEASELOCKOWNER4args) String() string {
	var b strings.Builder
	b.WriteString("RELEASELOCKOWNER4args{")
	fmt.Fprintf(&b, "lock_owner: %v", m.LockOwner())
	b.WriteString("}")
	return b.String()
}

func (m RELEASELOCKOWNER4res) String() string {
	var b strings.Builder
	b.WriteString("RELEASELOCKOWNER4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m ILLEGAL4res) String() string {
	var b strings.Builder
	b.WriteString("ILLEGAL4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m NfsArgop4) String() string {
	switch m.disc {
	case OP_ACCESS:
		return fmt.Sprintf("ACCESS:%v", m.AsACCESS4args())
	case OP_CLOSE:
		return fmt.Sprintf("CLOSE:%v", m.AsCLOSE4args())
	case OP_COMMIT:
		return fmt.Sprintf("COMMIT:%v", m.AsCOMMIT4args())
	case OP_CREATE:
		return fmt.Sprintf("CREATE:%v", m.AsCREATE4args())
	case OP_DELEGPURGE:
		return fmt.Sprintf("DELEGPURGE:%v", m.AsDELEGPURGE4args())
	case OP_DELEGRETURN:
		return fmt.Sprintf("DELEGRETURN:%v", m.AsDELEGRETURN4args())
	case OP_GETATTR:
		return fmt.Sprintf("GETATTR:%v", m.AsGETATTR4args())
	case OP_GETFH:
		return "GETFH"
	case OP_LINK:
		return fmt.Sprintf("LINK:%v", m.AsLINK4args())
	case OP_LOCK:
		return fmt.Sprintf("LOCK:%v", m.AsLOCK4args())
	case OP_LOCKT:
		return fmt.Sprintf("LOCKT:%v", m.AsLOCKT4args())
	case OP_LOCKU:
		return fmt.Sprintf("LOCKU:%v", m.AsLOCKU4args())
	case OP_LOOKUP:
		return fmt.Sprintf("LOOKUP:%v", m.AsLOOKUP4args())
	case OP_LOOKUPP:
		return "LOOKUPP"
	case OP_NVERIFY:
		return fmt.Sprintf("NVERIFY:%v", m.AsNVERIFY4args())
	case OP_OPEN:
		return fmt.Sprintf("OPEN:%v", m.AsOPEN4args())
	case OP_OPENATTR:
		return fmt.Sprintf("OPENATTR:%v", m.AsOPENATTR4args())
	case OP_OPEN_CONFIRM:
		return fmt.Sprintf("OPEN_CONFIRM:%v", m.AsOPENCONFIRM4args())
	case OP_OPEN_DOWNGRADE:
		return fmt.Sprintf("OPEN_DOWNGRADE:%v", m.AsOPENDOWNGRADE4args())
	case OP_PUTFH:
		return fmt.Sprintf("PUTFH:%v", m.AsPUTFH4args())
	case OP_PUTPUBFH:
		return "PUTPUBFH"
	case OP_PUTROOTFH:
		return "PUTROOTFH"
	case OP_READ:
		return fmt.Sprintf("READ:%v", m.AsREAD4args())
	case OP_READDIR:
		return fmt.Sprintf("READDIR:%v", m.AsREADDIR4args())
	case OP_READLINK:
		return "READLINK"
	case OP_REMOVE:
		return fmt.Sprintf("REMOVE:%v", m.AsREMOVE4args())
	case OP_RENAME:
		return fmt.Sprintf("RENAME:%v", m.AsRENAME4args())
	case OP_RENEW:
		return fmt.Sprintf("RENEW:%v", m.AsRENEW4args())
	case OP_RESTOREFH:
		return "RESTOREFH"
	case OP_SAVEFH:
		return "SAVEFH"
	case OP_SECINFO:
		return fmt.Sprintf("SECINFO:%v", m.AsSECINFO4args())
	case OP_SETATTR:
		return fmt.Sprintf("SETATTR:%v", m.AsSETATTR4args())
	case OP_SETCLIENTID:
		return fmt.Sprintf("SETCLIENTID:%v", m.AsSETCLIENTID4args())
	case OP_SETCLIENTID_CONFIRM:
		return fmt.Sprintf("SETCLIENTID_CONFIRM:%v", m.AsSETCLIENTIDCONFIRM4args())
	case OP_VERIFY:
		return fmt.Sprintf("VERIFY:%v", m.AsVERIFY4args())
	case OP_WRITE:
		return fmt.Sprintf("WRITE:%v", m.AsWRITE4args())
	case OP_RELEASE_LOCKOWNER:
		return fmt.Sprintf("RELEASE_LOCKOWNER:%v", m.AsRELEASELOCKOWNER4args())
	case OP_ILLEGAL:
		return "ILLEGAL"
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m ACCESS4resEntry) String() string {
	var b strings.Builder
	b.WriteString("ACCESS4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m CLOSE4resEntry) String() string {
	var b strings.Builder
	b.WriteString("CLOSE4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m COMMIT4resEntry) String() string {
	var b strings.Builder
	b.WriteString("COMMIT4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m CREATE4resEntry) String() string {
	var b strings.Builder
	b.WriteString("CREATE4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m GETATTR4resEntry) String() string {
	var b strings.Builder
	b.WriteString("GETATTR4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m GETFH4resEntry) String() string {
	var b strings.Builder
	b.WriteString("GETFH4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m LINK4resEntry) String() string {
	var b strings.Builder
	b.WriteString("LINK4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m LOCK4resEntry) String() string {
	var b strings.Builder
	b.WriteString("LOCK4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m LOCKT4resEntry) String() string {
	var b strings.Builder
	b.WriteString("LOCKT4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m LOCKU4resEntry) String() string {
	var b strings.Builder
	b.WriteString("LOCKU4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m OPEN4resEntry) String() string {
	var b strings.Builder
	b.WriteString("OPEN4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m OPENCONFIRM4resEntry) String() string {
	var b strings.Builder
	b.WriteString("OPENCONFIRM4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m OPENDOWNGRADE4resEntry) String() string {
	var b strings.Builder
	b.WriteString("OPENDOWNGRADE4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m READ4resEntry) String() string {
	var b strings.Builder
	b.WriteString("READ4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m READDIR4resEntry) String() string {
	var b strings.Builder
	b.WriteString("READDIR4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m READLINK4resEntry) String() string {
	var b strings.Builder
	b.WriteString("READLINK4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m REMOVE4resEntry) String() string {
	var b strings.Builder
	b.WriteString("REMOVE4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m RENAME4resEntry) String() string {
	var b strings.Builder
	b.WriteString("RENAME4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m SECINFO4resEntry) String() string {
	var b strings.Builder
	b.WriteString("SECINFO4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m SETCLIENTID4resEntry) String() string {
	var b strings.Builder
	b.WriteString("SETCLIENTID4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m WRITE4resEntry) String() string {
	var b strings.Builder
	b.WriteString("WRITE4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m NfsResop4) String() string {
	switch m.disc {
	case OP_ACCESS:
		return fmt.Sprintf("ACCESS:%v", m.AsACCESS4resEntry())
	case OP_CLOSE:
		return fmt.Sprintf("CLOSE:%v", m.AsCLOSE4resEntry())
	case OP_COMMIT:
		return fmt.Sprintf("COMMIT:%v", m.AsCOMMIT4resEntry())
	case OP_CREATE:
		return fmt.Sprintf("CREATE:%v", m.AsCREATE4resEntry())
	case OP_DELEGPURGE:
		return fmt.Sprintf("DELEGPURGE:%v", m.AsDELEGPURGE4res())
	case OP_DELEGRETURN:
		return fmt.Sprintf("DELEGRETURN:%v", m.AsDELEGRETURN4res())
	case OP_GETATTR:
		return fmt.Sprintf("GETATTR:%v", m.AsGETATTR4resEntry())
	case OP_GETFH:
		return fmt.Sprintf("GETFH:%v", m.AsGETFH4resEntry())
	case OP_LINK:
		return fmt.Sprintf("LINK:%v", m.AsLINK4resEntry())
	case OP_LOCK:
		return fmt.Sprintf("LOCK:%v", m.AsLOCK4resEntry())
	case OP_LOCKT:
		return fmt.Sprintf("LOCKT:%v", m.AsLOCKT4resEntry())
	case OP_LOCKU:
		return fmt.Sprintf("LOCKU:%v", m.AsLOCKU4resEntry())
	case OP_LOOKUP:
		return fmt.Sprintf("LOOKUP:%v", m.AsLOOKUP4res())
	case OP_LOOKUPP:
		return fmt.Sprintf("LOOKUPP:%v", m.AsLOOKUPP4res())
	case OP_NVERIFY:
		return fmt.Sprintf("NVERIFY:%v", m.AsNVERIFY4res())
	case OP_OPEN:
		return fmt.Sprintf("OPEN:%v", m.AsOPEN4resEntry())
	case OP_OPENATTR:
		return fmt.Sprintf("OPENATTR:%v", m.AsOPENATTR4res())
	case OP_OPEN_CONFIRM:
		return fmt.Sprintf("OPEN_CONFIRM:%v", m.AsOPENCONFIRM4resEntry())
	case OP_OPEN_DOWNGRADE:
		return fmt.Sprintf("OPEN_DOWNGRADE:%v", m.AsOPENDOWNGRADE4resEntry())
	case OP_PUTFH:
		return fmt.Sprintf("PUTFH:%v", m.AsPUTFH4res())
	case OP_PUTPUBFH:
		return fmt.Sprintf("PUTPUBFH:%v", m.AsPUTPUBFH4res())
	case OP_PUTROOTFH:
		return fmt.Sprintf("PUTROOTFH:%v", m.AsPUTROOTFH4res())
	case OP_READ:
		return fmt.Sprintf("READ:%v", m.AsREAD4resEntry())
	case OP_READDIR:
		return fmt.Sprintf("READDIR:%v", m.AsREADDIR4resEntry())
	case OP_READLINK:
		return fmt.Sprintf("READLINK:%v", m.AsREADLINK4resEntry())
	case OP_REMOVE:
		return fmt.Sprintf("REMOVE:%v", m.AsREMOVE4resEntry())
	case OP_RENAME:
		return fmt.Sprintf("RENAME:%v", m.AsRENAME4resEntry())
	case OP_RENEW:
		return fmt.Sprintf("RENEW:%v", m.AsRENEW4res())
	case OP_RESTOREFH:
		return fmt.Sprintf("RESTOREFH:%v", m.AsRESTOREFH4res())
	case OP_SAVEFH:
		return fmt.Sprintf("SAVEFH:%v", m.AsSAVEFH4res())
	case OP_SECINFO:
		return fmt.Sprintf("SECINFO:%v", m.AsSECINFO4resEntry())
	case OP_SETATTR:
		return fmt.Sprintf("SETATTR:%v", m.AsSETATTR4res())
	case OP_SETCLIENTID:
		return fmt.Sprintf("SETCLIENTID:%v", m.AsSETCLIENTID4resEntry())
	case OP_SETCLIENTID_CONFIRM:
		return fmt.Sprintf("SETCLIENTID_CONFIRM:%v", m.AsSETCLIENTIDCONFIRM4res())
	case OP_VERIFY:
		return fmt.Sprintf("VERIFY:%v", m.AsVERIFY4res())
	case OP_WRITE:
		return fmt.Sprintf("WRITE:%v", m.AsWRITE4resEntry())
	case OP_RELEASE_LOCKOWNER:
		return fmt.Sprintf("RELEASE_LOCKOWNER:%v", m.AsRELEASELOCKOWNER4res())
	case OP_ILLEGAL:
		return fmt.Sprintf("ILLEGAL:%v", m.AsILLEGAL4res())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m NfsArgop4Entry) String() string {
	var b strings.Builder
	b.WriteString("NfsArgop4Entry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m COMPOUND4args) String() string {
	var b strings.Builder
	b.WriteString("COMPOUND4args{")
	fmt.Fprintf(&b, "tag: %v", m.Tag())
	fmt.Fprintf(&b, ", minorversion: %d", m.Minorversion())
	{
		iter := m.Argarray()
		fmt.Fprintf(&b, ", argarray: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Argarray())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m NfsResop4Entry) String() string {
	var b strings.Builder
	b.WriteString("NfsResop4Entry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m COMPOUND4res) String() string {
	var b strings.Builder
	b.WriteString("COMPOUND4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	fmt.Fprintf(&b, ", tag: %v", m.Tag())
	{
		iter := m.Resarray()
		fmt.Fprintf(&b, ", resarray: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Resarray())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m CBGETATTR4args) String() string {
	var b strings.Builder
	b.WriteString("CBGETATTR4args{")
	fmt.Fprintf(&b, "fh: %v", m.Fh())
	fmt.Fprintf(&b, ", attr_request: %v", m.AttrRequest())
	b.WriteString("}")
	return b.String()
}

func (m CBGETATTR4resok) String() string {
	var b strings.Builder
	b.WriteString("CBGETATTR4resok{")
	fmt.Fprintf(&b, "obj_attributes: %v", m.ObjAttributes())
	b.WriteString("}")
	return b.String()
}

func (m CBGETATTR4res) String() string {
	switch m.disc {
	case NFS4_OK:
		return fmt.Sprintf("NFS4_OK:%v", m.AsCBGETATTR4resok())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m CBRECALL4args) String() string {
	var b strings.Builder
	b.WriteString("CBRECALL4args{")
	fmt.Fprintf(&b, "stateid: %v", m.Stateid())
	fmt.Fprintf(&b, ", truncate: %d", m.Truncate())
	fmt.Fprintf(&b, ", fh: %v", m.Fh())
	b.WriteString("}")
	return b.String()
}

func (m CBRECALL4res) String() string {
	var b strings.Builder
	b.WriteString("CBRECALL4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m CBILLEGAL4res) String() string {
	var b strings.Builder
	b.WriteString("CBILLEGAL4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	b.WriteString("}")
	return b.String()
}

func (m NfsCbArgop4) String() string {
	switch m.disc {
	case OP_CB_GETATTR:
		return fmt.Sprintf("GETATTR:%v", m.AsCBGETATTR4args())
	case OP_CB_RECALL:
		return fmt.Sprintf("RECALL:%v", m.AsCBRECALL4args())
	case OP_CB_ILLEGAL:
		return "ILLEGAL"
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m CBGETATTR4resEntry) String() string {
	var b strings.Builder
	b.WriteString("CBGETATTR4resEntry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m NfsCbResop4) String() string {
	switch m.disc {
	case OP_CB_GETATTR:
		return fmt.Sprintf("GETATTR:%v", m.AsCBGETATTR4resEntry())
	case OP_CB_RECALL:
		return fmt.Sprintf("RECALL:%v", m.AsCBRECALL4res())
	case OP_CB_ILLEGAL:
		return fmt.Sprintf("ILLEGAL:%v", m.AsCBILLEGAL4res())
	default:
		return fmt.Sprintf("unknown(%v)", m.disc)
	}
}

func (m NfsCbArgop4Entry) String() string {
	var b strings.Builder
	b.WriteString("NfsCbArgop4Entry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m CBCOMPOUND4args) String() string {
	var b strings.Builder
	b.WriteString("CBCOMPOUND4args{")
	fmt.Fprintf(&b, "tag: %v", m.Tag())
	fmt.Fprintf(&b, ", minorversion: %d", m.Minorversion())
	fmt.Fprintf(&b, ", callback_ident: %d", m.CallbackIdent())
	{
		iter := m.Argarray()
		fmt.Fprintf(&b, ", argarray: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Argarray())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

func (m NfsCbResop4Entry) String() string {
	var b strings.Builder
	b.WriteString("NfsCbResop4Entry{")
	fmt.Fprintf(&b, "value: %v", m.Value())
	b.WriteString("}")
	return b.String()
}

func (m CBCOMPOUND4res) String() string {
	var b strings.Builder
	b.WriteString("CBCOMPOUND4res{")
	fmt.Fprintf(&b, "status: %s", Nfsstat4Name(m.Status()))
	fmt.Fprintf(&b, ", tag: %v", m.Tag())
	{
		iter := m.Resarray()
		fmt.Fprintf(&b, ", resarray: [")
		i := 0
		for iter.Next() {
			if i > 0 {
				b.WriteString(", ")
			}
			if i >= 64 {
				b.WriteString("...")
				break
			}
			fmt.Fprintf(&b, "%v", iter.Resarray())
			i++
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}
