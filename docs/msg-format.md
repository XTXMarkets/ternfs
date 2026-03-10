<!--
Copyright 2026 XTX Markets Technologies Limited

SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception
-->

# .msg File Format

The `.msg` format describes binary message layouts for code generation. It is
designed for wire protocols where messages are read and written as flat byte
buffers with no deserialization into native structs. The format supports
enumerations, fixed-size structures, variable-length messages, and
discriminated unions.

## Comments

Line comments start with `//` and extend to the end of the line.

```
// This is a comment
```

## Primitive types

### Numeric types

| Type    | Size    | Description                            |
|---------|---------|----------------------------------------|
| `u8`    | 1 byte  | Unsigned 8-bit integer                 |
| `i8`    | 1 byte  | Signed 8-bit integer                   |
| `leu16` | 2 bytes | Little-endian unsigned 16-bit integer  |
| `beu16` | 2 bytes | Big-endian unsigned 16-bit integer     |
| `lei16` | 2 bytes | Little-endian signed 16-bit integer    |
| `bei16` | 2 bytes | Big-endian signed 16-bit integer       |
| `leu32` | 4 bytes | Little-endian unsigned 32-bit integer  |
| `beu32` | 4 bytes | Big-endian unsigned 32-bit integer     |
| `lei32` | 4 bytes | Little-endian signed 32-bit integer    |
| `bei32` | 4 bytes | Big-endian signed 32-bit integer       |
| `leu64` | 8 bytes | Little-endian unsigned 64-bit integer  |
| `beu64` | 8 bytes | Big-endian unsigned 64-bit integer     |
| `lei64` | 8 bytes | Little-endian signed 64-bit integer    |
| `bei64` | 8 bytes | Big-endian signed 64-bit integer       |

### Floating-point types

| Type       | Size    | Description                                    |
|------------|---------|------------------------------------------------|
| `lefloat`  | 4 bytes | Little-endian IEEE 754 single-precision float  |
| `befloat`  | 4 bytes | Big-endian IEEE 754 single-precision float     |
| `ledouble` | 8 bytes | Little-endian IEEE 754 double-precision float  |
| `bedouble` | 8 bytes | Big-endian IEEE 754 double-precision float     |

In Go, `lefloat` and `befloat` map to `float32`, while `ledouble` and `bedouble`
map to `float64`. The generated code uses `math.Float32frombits` /
`math.Float32bits` and the `Float64` equivalents to convert between the wire
representation and Go float types.

### Character and string types

| Type     | Size   | Description                                             |
|----------|--------|---------------------------------------------------------|
| `char`   | 1 byte | Character byte, distinct from `u8`                      |
| `npchar` | 1 byte | Null-padded character (for fixed-length string fields)   |
| `spchar` | 1 byte | Space-padded character (for fixed-length string fields)  |

`char` is semantically a text character rather than a raw byte. In Go it
produces `[]byte` (like `u8`), but in C++ it maps to `char*` rather than
`uint8_t*`.

`npchar` and `spchar` are used for fixed-length string fields. When reading,
trailing padding is stripped (0x00 for `npchar`, 0x20 for `spchar`). When
writing, values shorter than the field are padded to the full field length;
values longer are truncated.

```
struct volume_label {
    spchar    name[11];     // space-padded, getter strips trailing 0x20
    npchar    serial[16];   // null-padded, getter strips trailing 0x00
}
```

## Constants

Named constants can be defined with `const`:

```
const NFS4_FHSIZE = 128;
const NFS4_OPAQUE_LIMIT = 1024;
const NFS4_INT64_MAX = 0x7fffffffffffffff;
```

Constants are untyped integer values (decimal or hex). They are emitted as
untyped Go constants (`const NAME = value`). Multiple consecutive `const`
declarations are grouped into a single `const ( ... )` block.

Constants exist only for documentation and convenience — they do not define
types, cannot be used as array lengths or discriminants, and have no effect on
wire format parsing.

## Enums

An enum declares a set of named constants with an underlying type.

### Numeric enums

The underlying type can be any numeric primitive:

```
enum open_claim_type4 : beu32 {
    CLAIM_NULL          = 0;
    CLAIM_PREVIOUS      = 1;
    CLAIM_DELEGATE_CUR  = 2;
    CLAIM_DELEGATE_PREV = 3;
}
```

Values can be decimal literals, hex literals (`0x` prefix), or character
literals (single-quoted), which resolve to the Unicode code point:

```
enum reply_code : u8 {
    ACK = 'A';       // 0x41
    NAK = 'N';       // 0x4E
    ERR = 'E';       // 0x45
}
```

### String enums

The underlying type can also be `char[N]`, `npchar[N]`, or `spchar[N]` where N
is 2, 4, or 8. Values are double-quoted strings. These are lowered to `leu16`,
`leu32`, or `leu64` respectively in the backend, with the string bytes packed
into the integer in memory order (first character at the lowest address).

```
enum file_type : char[4] {
    RIFF = "RIFF";
    PNG  = "\x89PNG";
}
```

When the underlying type is `npchar[N]` or `spchar[N]`, string values shorter
than N are padded accordingly (null or space):

```
enum tag : spchar[4] {
    HDR = "HDR";     // stored as "HDR "
    DAT = "DAT";     // stored as "DAT "
    END = "END";     // stored as "END "
}
```

### Usage

Enum names are used as discriminant types in unions and as field types in
structs/messages, where they occupy the same number of bytes as their underlying
type.

Enum constant names must be unique across all enums in the file. This is
because code generators emit constants into a flat namespace (e.g. Go package-
level `const` declarations), so two constants with the same name would collide.

## Structs

A `struct` is a **fixed-size** type. Every field must have a size that is known
at compile time: either a primitive, an enum, another struct, or a fixed-size
array with a literal length.

```
struct stateid4 {
    beu32    seqid;
    u8       other[12];
}
```

The total size of a struct is the sum of its field sizes (including any
`pad` directives). There is no implicit padding between fields. Structs can be
embedded in other types and their size is always statically known.

### Fields

Scalar fields specify a type and a name:

```
beu32    seqid;
```

Fixed-size array fields append a literal integer in brackets:

```
u8    other[12];     // 12 bytes
u8    data[8];       // 8 bytes
```

The bracket value is the **element count**. For `u8` arrays this equals the byte
length. For wider types, the byte length is `count * element_size`.

### Padding

The `pad(N)` directive inserts exactly N zero bytes. It can appear in both
structs and messages:

```
struct padded_header {
    beu32    magic;
    pad(4);               // 4 bytes of padding
    beu64    timestamp;
}
```

Padding bytes are not exposed as a field.

### Alignment

The `align(N)` directive inserts 0 to N-1 padding bytes so that the next byte
offset (relative to the start of the type) is a multiple of N:

```
struct aligned_record {
    u8       flags;
    align(4);                 // 3 bytes of padding
    beu32    value;
}
```

In messages, `align` typically appears after a variable-length `u8` array to
restore alignment:

```
message component4 {
    beu32    name_len;
    u8       name[name_len];
    align(4);
}
```

The padding bytes are always zero and are not exposed as a field. `align` is
only needed after fields that may not be a multiple of the alignment — arrays
of wider types (`beu32`, `beu64`) and fixed-size structs are inherently aligned
by their element size.

### Embedding structs

A struct can appear as a field type in another struct or message. It is laid out
inline (not length-prefixed):

```
struct CLOSE4args {
    beu32      seqid;
    stateid4   open_stateid;     // 16 bytes inline
}
```

### Enum-typed fields

When an enum type is used as a field, it occupies the same bytes as the enum's
underlying type:

```
struct claim_previous4 {
    open_delegation_type4    delegate_type;    // 4 bytes (beu32)
}
```

## Messages

A `message` is a **variable-size** type. It contains at least one field whose
size depends on runtime data: a variable-length array, an alignment directive,
or an embedded message/union.

```
message PUTFH4args {
    beu32    fh_len;
    u8       fh[fh_len];
    align(4);
}
```

Messages use the same field syntax as structs, plus variable-length arrays
and extent.

### Variable-length arrays

A variable-length array uses a field name (rather than a literal) as its size:

```
beu32    bitmap_count;
beu32    bitmap[bitmap_count];
```

The referenced field must appear earlier in the same type and gives the
**element count**. The byte length on the wire is `count * element_size`.

For `u8` arrays, the element size is 1, so the count field is also the byte
length. For `beu32` arrays, each element is 4 bytes.

The array element type can also be a named type (struct, message, or union):

```
beu32               argarray_count;
nfs_argop4_entry    argarray[argarray_count];
```

When the element type is variable-size, each element must be parsed in sequence
to determine the extent of the array.

### Extent

The `extent(field)` directive declares that a field gives the byte length from
the directive to the end of the message. All subsequent fields are parsed from
within that byte region, and any bytes remaining after the last known field are
skipped.

```
message foo {
    beu32    version;
    beu32    body_len;
    extent(body_len);
    beu32    field_a;
    beu32    field_b;
}
```

The referenced field can appear anywhere in the same message (before or after
the directive). The reader parses fields after the directive normally, but the
total number of bytes consumed from that point is always exactly the value of
the extent field — any unparsed remainder is skipped.

This provides forward compatibility: future versions of the format can append
fields after `field_b` without breaking older readers, because older readers
know to skip `body_len` bytes regardless of how many fields they understand.

Wire format for `version=1, body_len=12, field_a=1, field_b=2` with one
unknown extension field:

```
[version:4] [body_len:4] [field_a:4] [field_b:4] [unknown:4]
                         |<----------- 12 bytes ---------->|
```

The reader parses `field_a` and `field_b` (8 bytes), sees it has consumed fewer
than `body_len` (12) bytes, and skips the remaining 4.

If the known fields of the message consume more bytes than the runtime extent
value, the behavior is undefined. The code generator should emit a validation
function that checks the extent is at least as large as the minimum size of the
known fields.

`extent` may appear at most once in a message and always covers from the
directive to the end of the message. If the length-delimited region is a
sub-region with fields after it, factor it into a separate message type.

### Embedding messages

A message can appear as a field type in another message. It is laid out inline
(not length-prefixed) and must be parsed to determine its extent:

```
message open_claim_delegate_cur4 {
    stateid4    delegate_stateid;    // fixed-size struct, 16 bytes
    component4  file;                // variable-size message
}
```

## Unions

A `union` is a discriminated (tagged) union. The discriminant's enum type is
given in parentheses, followed by arms that map enum values to payload types.

```
union createhow4 (createmode4) {
    UNCHECKED4:  fattr4;
    GUARDED4:    fattr4;
    EXCLUSIVE4:  verifier4;
}
```

A union's wire format is the **arm payload only** — the discriminant is never
part of the union encoding. The discriminant is always provided by a separate
field in the enclosing message (see [Usage in messages](#usage-in-messages)).

Each arm label must be a constant from the discriminant's enum type. The payload
type can be any defined type (struct, message, or another union).

### void arms

An arm with type `void` has no payload — when the discriminant matches, the
union contributes zero bytes:

```
union openflag4 (opentype4) {
    OPEN4_CREATE: openflag4_create;
    default:      void;
}
```

### default arm

A `default` arm matches any discriminant value not explicitly listed. It is
typically used with `void` for cases where no payload is needed:

```
union nfs_argop4 (nfs_opnum4) {
    OP_ACCESS:    ACCESS4args;
    OP_CLOSE:     CLOSE4args;
    // ...
    default:      void;
}
```

### Usage in messages

A union field must always be annotated with the name of the field that provides
its discriminant value, in parentheses:

```
message OPEN4args {
    beu32              seqid;
    beu32              share_access;
    beu32              share_deny;
    open_owner4        owner;
    opentype4          openhow_type;
    openflag4          openhow(openhow_type);
    open_claim_type4   claim_type;
    open_claim4        claim(claim_type);
}
```

The discriminant field must appear earlier in the same message. The code
generator uses its value to determine which arm to parse. On the wire, the
discriminant field and the union payload are separate — the discriminant appears
at its declared position, and the union payload follows at its declared
position.

When a union contains another union as an arm payload, the inner union needs
its own discriminant. Wrap it in a message that pairs the discriminant with the
payload:

```
// createhow4 needs a createmode4 discriminant
message openflag4_create {
    createmode4    mode;
    createhow4     how(mode);
}

union openflag4 (opentype4) {
    OPEN4_CREATE: openflag4_create;
    default:      void;
}
```

Similarly, arrays of unions require the array element type to be a message that
includes the discriminant:

```
message nfs_argop4_entry {
    nfs_opnum4    opnum;
    nfs_argop4    arg(opnum);
}

message COMPOUND4args {
    // ...
    beu32               argarray_count;
    nfs_argop4_entry    argarray[argarray_count];
}
```

## Type categories summary

| Keyword   | Size     | Can contain                                      |
|-----------|----------|--------------------------------------------------|
| `const`   | —        | Untyped integer constants                        |
| `enum`    | Fixed    | Named constants with a numeric or string underlying type |
| `struct`  | Fixed    | Primitives, enums, structs, fixed-size arrays, `pad`, `align` |
| `message` | Variable | Everything structs can, plus variable-length arrays, `extent`, embedded messages/unions |
| `union`   | Variable | Per-arm payloads (any type or `void`), discriminated by an external enum field |

## Declaration order

Types may be declared in any order. Forward references are supported — a type
can reference another type that is defined later in the file.

## Complete example

```
enum createmode4 : beu32 {
    UNCHECKED4 = 0;
    GUARDED4   = 1;
    EXCLUSIVE4 = 2;
}

struct verifier4 {
    u8    data[8];
}

message fattr4 {
    beu32    bitmap_count;
    beu32    bitmap[bitmap_count];
    beu32    attrlist_len;
    u8       attrlist[attrlist_len];
    align(4);
}

union createhow4 (createmode4) {
    UNCHECKED4:  fattr4;
    GUARDED4:    fattr4;
    EXCLUSIVE4:  verifier4;
}

message openflag4_create {
    createmode4    mode;
    createhow4     how(mode);
}
```

This defines a union `createhow4` discriminated on `createmode4`. When the
discriminant is `UNCHECKED4` or `GUARDED4`, the payload is a variable-length
`fattr4`. When `EXCLUSIVE4`, the payload is a fixed 8-byte `verifier4`. The
`openflag4_create` message pairs the discriminant with the union payload.

## Example: extent and external discriminant

```
enum msg_type : beu32 {
    MSG_REQUEST  = 1;
    MSG_RESPONSE = 2;
}

message request_data {
    beu32    request_id;
    beu32    flags;
}

message response_data {
    beu32    request_id;
    beu32    status;
}

union msg_body (msg_type) {
    MSG_REQUEST:  request_data;
    MSG_RESPONSE: response_data;
}

message envelope {
    msg_type    type;
    beu32       body_len;
    extent(body_len);
    msg_body    body(type);
}
```

`envelope` uses both features together. The `type` field determines which arm of
`msg_body` to parse, and `body(type)` names it as the discriminant. The
`extent(body_len)` directive says the remaining bytes are exactly `body_len`, so
future versions can append fields after `body` and older readers will skip them.

Wire format for a request with `body_len=12` and one unknown extension field:

```
[type:4 = 1] [body_len:4 = 12] [request_id:4] [flags:4] [unknown:4]
                               |<------------ 12 bytes ---------->|
```
