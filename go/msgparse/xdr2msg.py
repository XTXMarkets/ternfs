#!/usr/bin/env python3
# Copyright 2026 XTX Markets Technologies Limited
#
# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

"""
Translate an XDR (.x) file to .msg format.

Usage: python3 xdr2msg.py nfs.x > nfs_full.msg

Key mappings:
  - XDR enum           → .msg enum : beu32
  - XDR struct (fixed) → .msg struct
  - XDR struct (var)   → .msg message
  - XDR union switch   → .msg union + wrapper message
  - XDR typedef        → inline, or promoted to struct/message
  - XDR const          → .msg const (untyped)
  - XDR program        → skipped
  - XDR bool           → beu32 (4 bytes)
  - XDR int32/uint32   → bei32/beu32
  - XDR int64/uint64   → bei64/beu64
  - XDR opaque[N]      → u8[N]
  - XDR opaque<>       → beu32 data_len; u8 data[data_len]; align(4);
  - XDR T<>            → beu32 count; T arr[count];
"""

import copy
import re
import sys
from dataclasses import dataclass
from typing import Optional


# ---------------------------------------------------------------------------
# Lexer
# ---------------------------------------------------------------------------

TOKEN_RE = re.compile(r"""
    (/\*.*?\*/)           |  # block comment
    (//[^\n]*)            |  # line comment
    (0[xX][\da-fA-F]+)   |  # hex literal
    (\d+)                 |  # decimal literal
    ([a-zA-Z_]\w*)        |  # identifier
    ("[^"]*")             |  # string literal
    ([{}\[\]<>()=;:,*])  |  # punctuation
    (\s+)                    # whitespace
""", re.VERBOSE | re.DOTALL)


def tokenize(src):
    for m in TOKEN_RE.finditer(src):
        if m.group(1) or m.group(2) or m.group(8):
            continue
        if m.group(3):
            yield ("NUM", m.group(3))
        elif m.group(4):
            yield ("NUM", m.group(4))
        elif m.group(5):
            yield ("ID", m.group(5))
        elif m.group(6):
            yield ("STR", m.group(6))
        elif m.group(7):
            yield ("P", m.group(7))


# ---------------------------------------------------------------------------
# AST
# ---------------------------------------------------------------------------

@dataclass
class Const:
    name: str; value: str

@dataclass
class EnumVal:
    name: str; value: str

@dataclass
class Enum:
    name: str; values: list

@dataclass
class Field:
    type_name: str; name: str
    fixed_len: Optional[str] = None
    var_len: Optional[str] = None  # "" = unbounded
    is_optional: bool = False

@dataclass
class Struct:
    name: str; fields: list

@dataclass
class UnionArm:
    labels: list; type_name: Optional[str]; field_name: Optional[str]

@dataclass
class Union:
    name: str; disc_type: str; disc_name: str
    arms: list; default_arm: Optional[UnionArm] = None

@dataclass
class Typedef:
    name: str; base_type: str
    fixed_len: Optional[str] = None
    var_len: Optional[str] = None


# ---------------------------------------------------------------------------
# Parser
# ---------------------------------------------------------------------------

class Parser:
    def __init__(self, src):
        self.toks = list(tokenize(src))
        self.pos = 0

    def peek(self, off=0):
        i = self.pos + off
        return self.toks[i] if i < len(self.toks) else ("EOF", "")

    def adv(self):
        t = self.toks[self.pos]; self.pos += 1; return t

    def exp(self, kind, text=None):
        t = self.adv()
        if t[0] != kind or (text is not None and t[1] != text):
            raise SyntaxError(f"Expected ({kind},{text!r}), got {t} @{self.pos-1}")
        return t

    def match(self, kind, text=None):
        t = self.peek()
        if t[0] == kind and (text is None or t[1] == text):
            return self.adv()
        return None

    def parse(self):
        decls = []
        while self.peek()[0] != "EOF":
            d = self._top()
            if d:
                decls.append(d)
        return decls

    def _top(self):
        k, t = self.peek()
        if k != "ID":
            self.adv(); return None
        if t == "const":    return self._const()
        if t == "enum":     return self._enum()
        if t == "struct":   return self._struct()
        if t == "union":    return self._union()
        if t == "typedef":  return self._typedef()
        if t == "program":  self._skip_prog(); return None
        self.adv(); return None

    def _const(self):
        self.exp("ID", "const"); n = self.exp("ID")[1]
        self.exp("P", "="); v = self.adv()[1]; self.exp("P", ";")
        return Const(n, v)

    def _enum(self):
        self.exp("ID", "enum"); n = self.exp("ID")[1]; self.exp("P", "{")
        vals = []
        while not self.match("P", "}"):
            vn = self.exp("ID")[1]
            vv = self.adv()[1] if self.match("P", "=") else str(len(vals))
            vals.append(EnumVal(vn, vv)); self.match("P", ",")
        self.exp("P", ";")
        return Enum(n, vals)

    def _type_spec(self):
        if self.match("ID", "unsigned"):
            if self.match("ID", "hyper"): return "uint64_t"
            self.match("ID", "int"); return "uint32_t"
        if self.match("ID", "hyper"): return "int64_t"
        if self.peek()[1] == "int": self.adv(); return "int32_t"
        return self.exp("ID")[1]

    def _field(self):
        tn = self._type_spec()
        opt = bool(self.match("P", "*"))
        nm = self.exp("ID")[1]
        fl = vl = None
        if self.match("P", "["):
            fl = self.adv()[1]; self.exp("P", "]")
        elif self.match("P", "<"):
            vl = self.adv()[1] if self.peek()[1] != ">" else ""
            self.exp("P", ">")
        self.exp("P", ";")
        return Field(tn, nm, fl, vl, opt)

    def _struct(self):
        self.exp("ID", "struct"); n = self.exp("ID")[1]; self.exp("P", "{")
        fs = []
        while not self.match("P", "}"):
            fs.append(self._field())
        self.exp("P", ";")
        return Struct(n, fs)

    def _union(self):
        self.exp("ID", "union"); n = self.exp("ID")[1]
        self.exp("ID", "switch"); self.exp("P", "(")
        dt = self._type_spec(); dn = self.exp("ID")[1]
        self.exp("P", ")"); self.exp("P", "{")
        arms = []; default = None; pending = []
        while not self.match("P", "}"):
            if self.match("ID", "case"):
                lbl = self.adv()[1]; self.exp("P", ":")
                if self.peek()[1] in ("case", "default"):
                    pending.append(lbl); continue
                pending.append(lbl)
                arms.append(self._arm_payload(pending)); pending = []
            elif self.match("ID", "default"):
                self.exp("P", ":")
                default = self._arm_payload(pending + ["default"]); pending = []
            else:
                self.adv()
        self.exp("P", ";")
        return Union(n, dt, dn, arms, default)

    def _arm_payload(self, labels):
        if self.match("ID", "void"):
            self.exp("P", ";"); return UnionArm(labels, None, None)
        tn = self._type_spec(); fn = self.exp("ID")[1]; self.exp("P", ";")
        return UnionArm(labels, tn, fn)

    def _typedef(self):
        self.exp("ID", "typedef"); tn = self._type_spec(); nm = self.exp("ID")[1]
        fl = vl = None
        if self.match("P", "["):
            fl = self.adv()[1]; self.exp("P", "]")
        elif self.match("P", "<"):
            vl = self.adv()[1] if self.peek()[1] != ">" else ""
            self.exp("P", ">")
        self.exp("P", ";")
        return Typedef(nm, tn, fl, vl)

    def _skip_prog(self):
        self.exp("ID", "program"); self.exp("ID")
        d = 0
        while True:
            t = self.adv()
            if t[1] == "{": d += 1
            elif t[1] == "}":
                d -= 1
                if d == 0:
                    self.exp("P", "="); self.adv(); self.exp("P", ";"); return


# ---------------------------------------------------------------------------
# .msg Generator
# ---------------------------------------------------------------------------

XDR_PRIM = {
    "uint32_t": "beu32", "int32_t": "bei32",
    "uint64_t": "beu64", "int64_t": "bei64",
    "bool": "beu32", "opaque": "u8", "string": "u8",
}


class Gen:
    def __init__(self, decls):
        self.decls = decls
        self.consts = {}
        self.enums = {}
        self.enum_vals = {}
        self.structs = {}
        self.unions = {}
        self.tdmap = {}
        self.emitted = set()
        self.wrappers = {}
        self.out = []

        # External constants not defined in the .x file
        self.consts["RPCSEC_GSS"] = "6"  # RFC 2203

        for d in decls:
            if isinstance(d, Const):   self.consts[d.name] = d.value
            elif isinstance(d, Enum):
                self.enums[d.name] = d
                for v in d.values: self.enum_vals[v.name] = d.name
            elif isinstance(d, Struct):  self.structs[d.name] = d
            elif isinstance(d, Union):   self.unions[d.name] = d
            elif isinstance(d, Typedef): self.tdmap[d.name] = d

    def rc(self, val):
        """Resolve constant."""
        return self.consts.get(val, val) if val else val

    def resolve_td(self, name, depth=0):
        """Resolve typedef chain → (base, fixed_len, var_len).
        Stops at named types (structs, enums, unions) even if they're typedef'd."""
        if depth > 20 or name not in self.tdmap:
            return name, None, None
        td = self.tdmap[name]
        # If this typedef adds array dimensions and the base is a known named type,
        # stop here — don't resolve further
        base_is_named = (td.base_type in self.structs or
                         td.base_type in self.enums or
                         td.base_type in self.unions or
                         self._is_promoted(td.base_type))
        if base_is_named:
            fl = self.rc(td.fixed_len)
            vl = td.var_len
            if vl and vl != "":
                vl = self.rc(vl)
            return td.base_type, fl, vl

        base, bfl, bvl = self.resolve_td(td.base_type, depth + 1)
        fl = self.rc(td.fixed_len) if td.fixed_len is not None else bfl
        vl = td.var_len if td.var_len is not None else bvl
        if vl and vl != "":
            vl = self.rc(vl)
        return base, fl, vl

    def _is_promoted(self, name):
        """Check if a typedef name should become its own .msg type."""
        if name not in self.tdmap:
            return False
        base, fl, vl = self._raw_resolve(name)
        if vl is not None:
            return True
        if fl is not None and base in ("opaque", "u8", "string"):
            return True
        return False

    def _raw_resolve(self, name, depth=0):
        """Raw resolve without stopping at named types."""
        if depth > 20 or name not in self.tdmap:
            return name, None, None
        td = self.tdmap[name]
        base, bfl, bvl = self._raw_resolve(td.base_type, depth + 1)
        fl = self.rc(td.fixed_len) if td.fixed_len is not None else bfl
        vl = td.var_len if td.var_len is not None else bvl
        if vl and vl != "":
            vl = self.rc(vl)
        return base, fl, vl

    def msg_type(self, xdr_type):
        if xdr_type in XDR_PRIM:
            return XDR_PRIM[xdr_type]
        base, fl, vl = self.resolve_td(xdr_type)
        if base in XDR_PRIM:
            return XDR_PRIM[base]
        return xdr_type

    def is_enum(self, name):
        if name in self.enums: return True
        base, _, _ = self.resolve_td(name)
        return base in self.enums

    def enum_name(self, name):
        if name in self.enums: return name
        base, _, _ = self.resolve_td(name)
        return base if base in self.enums else name

    def is_fixed(self, xdr_type, visited=None):
        if visited is None: visited = set()
        if xdr_type in visited: return True  # break cycles
        visited.add(xdr_type)
        if xdr_type in XDR_PRIM: return True
        if xdr_type in self.enums: return True
        # Promoted typedefs that become messages are variable-size
        if self._is_promoted(xdr_type):
            _, fl, vl = self._raw_resolve(xdr_type)
            if vl is not None: return False
            # Fixed promoted (struct) is fixed
            return True
        base, fl, vl = self.resolve_td(xdr_type)
        if vl is not None: return False
        if base in XDR_PRIM: return fl is not None or base not in ("opaque", "string")
        if base in self.enums: return True
        if base in self.structs:
            return all(self._field_fixed(f, visited) for f in self.structs[base].fields)
        if base in self.unions: return False
        # Check if base is promoted
        if self._is_promoted(base): return self.is_fixed(base, visited)
        return True

    def _field_fixed(self, f, visited=None):
        if f.var_len is not None or f.is_optional: return False
        base, fl, vl = self.resolve_td(f.type_name)
        if vl is not None: return False
        return self.is_fixed(f.type_name, visited)

    # -- Emit --

    def _find_used_types(self):
        """Find all promoted typedef names that are actually referenced
        as field types in structs, messages, or union arms."""
        used = set()
        for d in self.decls:
            if isinstance(d, Struct):
                for f in d.fields:
                    self._mark_used(f.type_name, used)
            elif isinstance(d, Union):
                for arm in d.arms:
                    if arm.type_name:
                        self._mark_used(arm.type_name, used)
                if d.default_arm and d.default_arm.type_name:
                    self._mark_used(d.default_arm.type_name, used)
        return used

    def _mark_used(self, type_name, used):
        """Mark a type as used if it's a promoted typedef."""
        if type_name in used:
            return
        if self._is_promoted(type_name):
            used.add(type_name)
            return
        # Check if it resolves to a promoted typedef
        base, _, _ = self.resolve_td(type_name)
        if base != type_name and self._is_promoted(base):
            # The field uses the alias name, which is itself promoted.
            # Don't mark the base — it's inlined into the alias.
            used.add(type_name)

    def emit(self):
        self.out = []

        # Emit constants first.
        for d in self.decls:
            if isinstance(d, Const):
                self.out.append(f"const {d.name} = {d.value};")

        # Add blank line separator if we emitted any constants.
        if self.out:
            self.out.append("")

        used_promoted = self._find_used_types()

        for d in self.decls:
            if isinstance(d, Enum):
                self._emit_enum(d)
            elif isinstance(d, Typedef) and self._is_promoted(d.name):
                if d.name in used_promoted:
                    self._emit_promoted_td(d)
            elif isinstance(d, Struct):
                self._emit_struct(d)
            elif isinstance(d, Union):
                self._emit_union(d)

        return "\n".join(self.out) + "\n"

    def _start_block(self):
        """Mark the start of a block whose body lines will be aligned."""
        return len(self.out)

    def _end_block(self, start):
        """Align body lines emitted since _start_block."""
        self.out[start:] = self._align_block(self.out[start:])

    @staticmethod
    def _align_block(lines):
        """Align the second column within a block of indented lines.

        For field lines like '    type    name;', aligns name to a consistent
        column. For enum lines like '    NAME = value;', aligns '='. For union
        arm lines like '    LABEL: type;', aligns the type after ':'.
        Non-matching lines (align directives, braces) are left unchanged.
        """
        # Detect the pattern from the first indented two-column line.
        field_re = re.compile(r'^(    \S+)\s+(\S.*)$')
        max_col1 = 0
        for line in lines:
            m = field_re.match(line)
            if m:
                max_col1 = max(max_col1, len(m.group(1)))
        if max_col1 == 0:
            return lines
        result = []
        for line in lines:
            m = field_re.match(line)
            if m:
                result.append(f"{m.group(1):<{max_col1}} {m.group(2)}")
            else:
                result.append(line)
        return result

    def _emit_enum(self, e):
        if e.name in self.emitted: return
        self.emitted.add(e.name)
        self.out.append(f"enum {e.name} : beu32 {{")
        start = self._start_block()
        for v in e.values:
            self.out.append(f"    {v.name} = {v.value};")
        self._end_block(start)
        self.out.append("}")
        self.out.append("")

    def _emit_promoted_td(self, td):
        if td.name in self.emitted: return
        self.emitted.add(td.name)
        # Use raw resolution (all the way to primitives) to determine structure
        base, fl, vl = self._raw_resolve(td.name)
        msg_base = XDR_PRIM.get(base, base)

        # But also check resolve_td for the immediate base — if it's a named
        # type (struct/message/union), we reference it directly
        imm_base, imm_fl, imm_vl = self.resolve_td(td.name)
        if imm_base in self.unions:
            # Typedef of a union — if array, use wrapper entry type
            if imm_vl is not None:
                wrapper = self._ensure_union_wrapper_emitted(imm_base)
                self.out.append(f"message {td.name} {{")
                start = self._start_block()
                self.out.append(f"    beu32 count;")
                self.out.append(f"    {wrapper} data[count];")
                self._end_block(start)
                self.out.append("}")
                self.out.append("")
                return
            # Simple alias of union — skip, shouldn't happen
            return
        if imm_base in self.structs or self._is_promoted(imm_base):
            # This typedef aliases a named type. Check if it adds array dims.
            if imm_fl is not None:
                self.out.append(f"struct {td.name} {{")
                self.out.append(f"    {imm_base} data[{imm_fl}];")
                self.out.append("}")
                self.out.append("")
                return
            if imm_vl is not None:
                self.out.append(f"message {td.name} {{")
                start = self._start_block()
                self.out.append(f"    beu32 count;")
                self.out.append(f"    {imm_base} data[count];")
                self._end_block(start)
                self.out.append("}")
                self.out.append("")
                return
            # Simple alias — emit same structure as the base
            # Fall through to raw resolution

        if fl is not None and vl is None:
            # Fixed array → struct
            self.out.append(f"struct {td.name} {{")
            self.out.append(f"    {msg_base} data[{fl}];")
            self.out.append("}")
        elif vl is not None:
            # Variable-length → message
            self.out.append(f"message {td.name} {{")
            start = self._start_block()
            if msg_base == "u8":
                self.out.append(f"    beu32 data_len;")
                self.out.append(f"    u8 data[data_len];")
                self.out.append(f"    align(4);")
            else:
                self.out.append(f"    beu32 count;")
                self.out.append(f"    {msg_base} data[count];")
            self._end_block(start)
            self.out.append("}")
        self.out.append("")

    def _emit_struct(self, s):
        if s.name in self.emitted: return
        self.emitted.add(s.name)

        has_union = any(self._field_is_union(f) for f in s.fields)
        is_fixed = (not has_union) and all(self._field_fixed(f) for f in s.fields)

        # Pre-emit any wrappers needed by fields
        for f in s.fields:
            self._pre_emit_wrappers(f)

        # Count how many variable-length u8 arrays would be inlined.
        # If >=2, we need to use message references instead (multi-array
        # with u8 elements isn't supported by the compiler).
        inline_var_u8_count = 0
        for f in s.fields:
            if self._would_inline_var_u8(f):
                inline_var_u8_count += 1
        use_opaque_ref = inline_var_u8_count >= 2
        if use_opaque_ref:
            self._ensure_xdr_opaque()

        kw = "struct" if is_fixed else "message"
        self.out.append(f"{kw} {s.name} {{")
        start = self._start_block()
        for f in s.fields:
            self._emit_field(f, use_opaque_ref=use_opaque_ref)
        self._end_block(start)
        self.out.append("}")
        self.out.append("")

    def _would_inline_var_u8(self, f):
        """Check if a field would be inlined as beu32 len; u8 data[len]; align(4);"""
        if f.is_optional or f.fixed_len is not None:
            return False
        base, fl, vl = self.resolve_td(f.type_name)
        if base in self.unions or base in self.structs:
            return False
        if self._is_promoted(f.type_name) or (base in self.tdmap and self._is_promoted(base)):
            return False
        if f.type_name in self.structs or f.type_name in self.enums:
            return False
        msg_t = self.msg_type(f.type_name)
        eff_vl = f.var_len
        if eff_vl is None:
            _, _, eff_vl = self.resolve_td(f.type_name)
        return eff_vl is not None and msg_t == "u8"

    def _pre_emit_wrappers(self, f):
        """Pre-emit any wrapper types needed by this field."""
        if f.is_optional:
            self._ensure_optional_union(f.type_name)
            return
        base, fl, vl = self.resolve_td(f.type_name)
        if base in self.unions:
            if f.var_len is not None or (vl is not None and f.fixed_len is None and f.var_len is None):
                self._ensure_union_wrapper_emitted(base)

    def _resolves_to(self, tn, target):
        base, _, _ = self.resolve_td(tn)
        return base == target or tn == target

    def _field_is_union(self, f):
        base, _, _ = self.resolve_td(f.type_name)
        return base in self.unions

    def _ensure_xdr_opaque(self):
        """Ensure the xdr_opaque message type exists."""
        if "xdr_opaque" not in self.emitted:
            self.emitted.add("xdr_opaque")
            self.out.append("message xdr_opaque {")
            start = self._start_block()
            self.out.append("    beu32 data_len;")
            self.out.append("    u8 data[data_len];")
            self.out.append("    align(4);")
            self._end_block(start)
            self.out.append("}")
            self.out.append("")

    def _emit_field(self, f, use_opaque_ref=False):
        # XDR "type" clashes with Go keyword.
        if f.name == "type":
            f = copy.copy(f)
            f.name = "type_val"

        if f.is_optional:
            opt_union = self._ensure_optional_union(f.type_name)
            disc_field = f"{f.name}_present"
            self._ensure_xdr_bool()
            self.out.append(f"    xdr_bool {disc_field};")
            self.out.append(f"    {opt_union} {f.name}({disc_field});")
            return

        base, fl, vl = self.resolve_td(f.type_name)

        # Union-typed field
        if base in self.unions:
            u = self.unions[base]
            if f.var_len is not None or (vl is not None and f.fixed_len is None and f.var_len is None):
                # Array of unions → each element needs disc+union wrapper
                # (wrapper already emitted by _pre_emit_wrappers)
                key = f"uwrap_{base}"
                wrapper = self.wrappers.get(key, f"{base}_entry")
                cn = f"{f.name}_count"
                self.out.append(f"    beu32 {cn};")
                self.out.append(f"    {wrapper} {f.name}[{cn}];")
                return
            if f.fixed_len is None and f.var_len is None and vl is None:
                # Scalar union field → split into disc + union(disc)
                disc_enum = self._ensure_disc_enum(u)
                disc_field = f"{f.name}_type"
                self.out.append(f"    {disc_enum} {disc_field};")
                self.out.append(f"    {u.name} {f.name}({disc_field});")
                return

        # Promoted typedef → use its name directly as a type reference
        if self._is_promoted(f.type_name):
            if f.fixed_len is not None:
                self.out.append(f"    {f.type_name} {f.name}[{self.rc(f.fixed_len)}];")
            elif f.var_len is not None:
                cn = f"{f.name}_count"
                self.out.append(f"    beu32 {cn};")
                self.out.append(f"    {f.type_name} {f.name}[{cn}];")
            else:
                self.out.append(f"    {f.type_name} {f.name};")
            return

        # Check if the type resolves to a promoted typedef (e.g., field type
        # is "component4" which is a promoted typedef)
        if base in self.tdmap and self._is_promoted(base):
            # The base itself is promoted — field type resolves to it
            if f.fixed_len is not None:
                self.out.append(f"    {base} {f.name}[{self.rc(f.fixed_len)}];")
            elif f.var_len is not None:
                cn = f"{f.name}_count"
                self.out.append(f"    beu32 {cn};")
                self.out.append(f"    {base} {f.name}[{cn}];")
            else:
                self.out.append(f"    {base} {f.name};")
            return

        # Named struct/enum that isn't a typedef
        if f.type_name in self.structs or f.type_name in self.enums:
            if f.fixed_len is not None:
                self.out.append(f"    {f.type_name} {f.name}[{self.rc(f.fixed_len)}];")
            elif f.var_len is not None:
                cn = f"{f.name}_count"
                self.out.append(f"    beu32 {cn};")
                self.out.append(f"    {f.type_name} {f.name}[{cn}];")
            else:
                self.out.append(f"    {f.type_name} {f.name};")
            return

        # Resolve the full type
        msg_t = self.msg_type(f.type_name)

        # Field-level array specs
        eff_fl = self.rc(f.fixed_len)
        eff_vl = f.var_len
        if eff_vl and eff_vl != "":
            eff_vl = self.rc(eff_vl)

        # If no field-level spec, use typedef's
        if eff_fl is None and eff_vl is None:
            eff_fl = fl
            eff_vl = vl

        if eff_vl is not None:
            if msg_t == "u8" and use_opaque_ref:
                # Multi-array: use xdr_opaque message type instead of inline
                self._ensure_xdr_opaque()
                self.out.append(f"    xdr_opaque {f.name};")
            elif msg_t == "u8":
                self.out.append(f"    beu32 {f.name}_len;")
                self.out.append(f"    u8 {f.name}[{f.name}_len];")
                self.out.append(f"    align(4);")
            else:
                cn = f"{f.name}_count"
                self.out.append(f"    beu32 {cn};")
                self.out.append(f"    {msg_t} {f.name}[{cn}];")
        elif eff_fl is not None:
            self.out.append(f"    {msg_t} {f.name}[{eff_fl}];")
        elif f.type_name == "bool":
            self.out.append(f"    beu32 {f.name};")
        else:
            self.out.append(f"    {msg_t} {f.name};")

    def _ensure_xdr_bool(self):
        """Ensure the xdr_bool enum exists."""
        if "xdr_bool" not in self.emitted:
            self.emitted.add("xdr_bool")
            self.out.append("enum xdr_bool : beu32 {")
            self.out.append("    FALSE = 0;")
            self.out.append("    TRUE = 1;")
            self.out.append("}")
            self.out.append("")

    def _ensure_optional_union(self, type_name):
        """Ensure an optional union for a type exists. Returns the union name."""
        base = self.msg_type(type_name)
        uname = f"{base}_opt"
        if uname not in self.emitted:
            self.emitted.add(uname)
            self._ensure_xdr_bool()
            arm_lines = [f"    TRUE: {base};", f"    default: void;"]
            self.out.append(f"union {uname} (xdr_bool) {{")
            self.out.extend(self._align_block(arm_lines))
            self.out.append("}")
            self.out.append("")
        return uname

    def _ensure_disc_enum(self, u):
        dt = u.disc_type
        if dt in self.enums: return dt
        base, _, _ = self.resolve_td(dt)
        if base in self.enums: return self.enum_name(dt)
        if dt == "bool": return self._make_bool_enum(u.name)
        return self._make_synth_enum(u)

    def _make_bool_enum(self, uname):
        self._ensure_xdr_bool()
        return "xdr_bool"

    def _resolve_label_value(self, lbl):
        """Resolve a union case label to a numeric value."""
        # Check consts first
        if lbl in self.consts:
            return self.consts[lbl]
        # Check enum values
        if lbl in self.enum_vals:
            enum_name = self.enum_vals[lbl]
            for v in self.enums[enum_name].values:
                if v.name == lbl:
                    return v.value
        return lbl

    def _make_synth_enum(self, u):
        # First check: if all labels come from a single existing enum, just use it
        label_enums = set()
        for arm in u.arms:
            for lbl in arm.labels:
                if lbl in self.enum_vals:
                    label_enums.add(self.enum_vals[lbl])
        if u.default_arm:
            for lbl in u.default_arm.labels:
                if lbl != "default" and lbl in self.enum_vals:
                    label_enums.add(self.enum_vals[lbl])
        if len(label_enums) == 1:
            return label_enums.pop()

        key = f"disc_{u.name}"
        if key in self.emitted: return key
        self.emitted.add(key)
        self.out.append(f"enum {key} : beu32 {{")
        start = self._start_block()
        for arm in u.arms:
            for lbl in arm.labels:
                val = self._resolve_label_value(lbl)
                self.out.append(f"    {lbl} = {val};")
        if u.default_arm:
            for lbl in u.default_arm.labels:
                if lbl != "default":
                    val = self._resolve_label_value(lbl)
                    self.out.append(f"    {lbl} = {val};")
        self._end_block(start)
        self.out.append("}")
        self.out.append("")
        return key

    def _emit_union(self, u):
        if u.name in self.emitted: return
        self.emitted.add(u.name)

        disc_enum = self._ensure_disc_enum(u)

        # Collect arm lines + any wrapper types needed
        arm_lines = []
        wrapper_lines = []

        for arm in u.arms:
            for label in arm.labels:
                if arm.type_name is None:
                    arm_lines.append(f"    {label}: void;")
                else:
                    payload, wlines = self._resolve_arm(arm.type_name, label, u.name)
                    wrapper_lines.extend(wlines)
                    arm_lines.append(f"    {label}: {payload};")

        if u.default_arm:
            if u.default_arm.type_name is None:
                arm_lines.append(f"    default: void;")
            else:
                payload, wlines = self._resolve_arm(u.default_arm.type_name, "default", u.name)
                wrapper_lines.extend(wlines)
                arm_lines.append(f"    default: {payload};")

        # Emit wrappers first, then the union
        self.out.extend(wrapper_lines)
        self.out.append(f"union {u.name} ({disc_enum}) {{")
        self.out.extend(self._align_block(arm_lines))
        self.out.append("}")
        self.out.append("")

    def _resolve_arm(self, type_name, label, union_name):
        """Resolve a union arm's payload type. Returns (type_name, wrapper_lines)."""
        base, fl, vl = self.resolve_td(type_name)
        wlines = []

        # Named struct → use directly
        if base in self.structs:
            return type_name, wlines
        # Promoted typedef → use directly
        if self._is_promoted(type_name):
            return type_name, wlines
        if self._is_promoted(base):
            return base, wlines
        # Named union → needs disc+union wrapper
        if base in self.unions:
            name, wl = self._make_union_wrapper(base)
            return name, wl
        # Bare primitive or enum → scalar wrapper struct
        if base in XDR_PRIM or base in self.enums:
            name, wl = self._make_scalar_wrapper(type_name, label, union_name)
            return name, wl
        # Unknown — use as-is
        return type_name, wlines

    def _make_scalar_wrapper(self, type_name, label, union_name):
        key = f"scalar_{union_name}_{label}"
        if key in self.wrappers:
            return self.wrappers[key], []
        wname = f"{union_name}_{label}"
        self.wrappers[key] = wname
        msg_t = self.msg_type(type_name)
        lines = [
            f"struct {wname} {{",
            f"    {msg_t} value;",
            "}",
            "",
        ]
        return wname, lines

    def _ensure_union_wrapper_emitted(self, union_name):
        """Create and emit a wrapper message inline for a union type."""
        key = f"uwrap_{union_name}"
        if key in self.wrappers:
            return self.wrappers[key]
        u = self.unions[union_name]
        disc_enum = self._ensure_disc_enum(u)
        wname = f"{union_name}_entry"
        self.wrappers[key] = wname
        self.out.append(f"message {wname} {{")
        start = self._start_block()
        self.out.append(f"    {disc_enum} disc;")
        self.out.append(f"    {union_name} value(disc);")
        self._end_block(start)
        self.out.append("}")
        self.out.append("")
        return wname

    def _make_union_wrapper(self, union_name):
        key = f"uwrap_{union_name}"
        if key in self.wrappers:
            return self.wrappers[key], []
        u = self.unions[union_name]
        disc_enum = self._ensure_disc_enum(u)
        wname = f"{union_name}_entry"
        self.wrappers[key] = wname
        body = self._align_block([
            f"    {disc_enum} disc;",
            f"    {union_name} value(disc);",
        ])
        lines = [f"message {wname} {{"] + body + ["}", ""]
        return wname, lines


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 xdr2msg.py <file.x>", file=sys.stderr)
        sys.exit(1)

    with open(sys.argv[1]) as f:
        src = f.read()

    decls = Parser(src).parse()
    output = Gen(decls).emit()
    print(output, end="")


if __name__ == "__main__":
    main()
