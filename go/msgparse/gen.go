// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Generator produces Go source code from an analyzed .msg file.
type Generator struct {
	a       *Analysis
	w       strings.Builder
	pkg     string
	nameMap map[string]string // .msg name → Go PascalCase name (cache)
}

// goName returns the Go PascalCase name for a TypeInfo.
func (g *Generator) goName(ti *TypeInfo) string {
	name := ti.Decl.Name()
	if n, ok := g.nameMap[name]; ok {
		return n
	}
	n := pascalCase(name)
	g.nameMap[name] = n
	return n
}

// offName returns the Go struct field name for a VarSectionBoundary at index idx.
func offName(idx int) string {
	return fmt.Sprintf("off%d", idx+1)
}

// goType returns the Go type string for a PrimInfo.
func goType(p PrimInfo) string {
	if p.IsFloat {
		if p.Size == 4 {
			return "float32"
		}
		return "float64"
	}
	if p.IsChar {
		return "byte"
	}
	base := "uint"
	if p.Signed {
		base = "int"
	}
	if p.Size == 1 {
		return base + "8"
	}
	return fmt.Sprintf("%s%d", base, p.Size*8)
}

// enumGoType returns the Go type for an enum's underlying wire type.
func enumGoType(e *EnumDecl) string {
	return goType(enumPrimInfo(e))
}

// Naming helpers.

func pascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func camelCase(s string) string {
	p := pascalCase(s)
	if len(p) == 0 {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

func singularize(s string) string {
	if strings.HasSuffix(s, "ies") {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(s, "ses") || strings.HasSuffix(s, "xes") ||
		strings.HasSuffix(s, "zes") || strings.HasSuffix(s, "shes") ||
		strings.HasSuffix(s, "ches") {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(s, "s") {
		return s[:len(s)-1]
	}
	return s
}

// stripEnumPrefix removes the common prefix from an enum constant name.
func stripEnumPrefix(constName string, allConstants []EnumConst) string {
	if len(allConstants) == 0 {
		return constName
	}
	prefix := allConstants[0].Name
	for _, c := range allConstants[1:] {
		for len(prefix) > 0 && !strings.HasPrefix(c.Name, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	if idx := strings.LastIndex(prefix, "_"); idx >= 0 {
		prefix = prefix[:idx+1]
	} else {
		prefix = ""
	}
	stripped := strings.TrimPrefix(constName, prefix)
	if stripped == "" {
		return constName
	}
	return stripped
}

// armSuffix converts an ALL_CAPS stripped name to PascalCase.
func armSuffix(allCaps string) string {
	if len(allCaps) == 0 {
		return allCaps
	}
	parts := strings.Split(allCaps, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = string(unicode.ToUpper(rune(p[0]))) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, "")
}

func Generate(a *Analysis, pkg string) string {
	g := &Generator{a: a, pkg: pkg, nameMap: make(map[string]string)}
	g.emitHeader()
	g.emitEnums()

	g.emitTypes()
	g.emitStringMethods()
	return g.w.String()
}

func (g *Generator) p(format string, args ...any) {
	fmt.Fprintf(&g.w, format, args...)
}

const sectionSep = "// -------------------------------------------------------\n"

func sizeConstName(typeName string) string {
	return camelCase(typeName) + "Size"
}

// isByteSliceType returns true if a primitive type should use []byte/string
// slice access instead of element-at-a-time access for variable-length arrays.
// This applies to u8, char, npchar, and spchar (all single-byte, no endian conversion).
func isByteSliceType(typeName string) bool {
	switch typeName {
	case "u8", "char", "npchar", "spchar":
		return true
	}
	return false
}

func (g *Generator) emitHeader() {
	needsMath := false
	for _, name := range g.a.Order {
		ti := g.a.Types[name]
		var fields []Field
		switch ti.Decl.Kind {
		case DeclStruct:
			fields = ti.Decl.Struct.Fields
		case DeclMessage:
			fields = ti.Decl.Message.Fields
		}
		for _, f := range fields {
			if p, ok := primitives[f.TypeName]; ok && p.IsFloat {
				needsMath = true
			}
		}
	}
	// String() methods always need "fmt" and "strings".
	g.p("package %s\n\nimport (\n\t\"encoding/binary\"\n\t\"fmt\"\n", g.pkg)
	if needsMath {
		g.p("\t\"math\"\n")
	}
	g.p("\t\"strings\"\n)\n")
}

func (g *Generator) emitEnums() {
	first := true

	// Collect consecutive const groups and emit them merged.
	i := 0
	for i < len(g.a.Order) {
		name := g.a.Order[i]
		ti := g.a.Types[name]
		if ti.Decl.Kind == DeclConstGroup {
			if first {
				g.p("\n" + sectionSep + "// Enum constants\n" + sectionSep)
				first = false
			}
			// Collect all consecutive const groups.
			var allConsts []ConstEntry
			for i < len(g.a.Order) {
				n := g.a.Order[i]
				t := g.a.Types[n]
				if t.Decl.Kind != DeclConstGroup {
					break
				}
				allConsts = append(allConsts, t.Decl.ConstGroup.Constants...)
				i++
			}
			g.emitConstGroup(&ConstGroupDecl{Constants: allConsts})
			continue
		}
		i++
		if ti.Decl.Kind != DeclEnum {
			continue
		}
		e := ti.Decl.Enum
		if first {
			g.p("\n" + sectionSep + "// Enum constants\n" + sectionSep)
			first = false
		}
		g.p("\n// %s constants (%s", e.Name, e.Underlying)
		if strings.Contains(e.Underlying, "[") {
			// String enum — show lowered type.
			n := extractBracketNum(e.Underlying)
			switch n {
			case 2:
				g.p(" -> leu16")
			case 4:
				g.p(" -> leu32")
			case 8:
				g.p(" -> leu64")
			}
		}
		g.p(")\nconst (\n")
		goType := enumGoType(e)
		maxNameLen := 0
		for _, c := range e.Constants {
			if len(c.Name) > maxNameLen {
				maxNameLen = len(c.Name)
			}
		}
		for _, c := range e.Constants {
			pad := strings.Repeat(" ", maxNameLen-len(c.Name))
			val, comment := g.enumConstValue(e, c)
			if comment != "" {
				g.p("\t%s%s %s = %s // %s\n", c.Name, pad, goType, val, comment)
			} else {
				g.p("\t%s%s %s = %s\n", c.Name, pad, goType, val)
			}
		}
		g.p(")\n")
	}
}

func (g *Generator) enumConstValue(e *EnumDecl, c EnumConst) (string, string) {
	if strings.Contains(e.Underlying, "[") {
		// String enum: pack string into integer.
		n := extractBracketNum(e.Underlying)
		raw := unquoteString(c.Value)
		charKind := ""
		if strings.HasPrefix(e.Underlying, "npchar") {
			charKind = "np"
		} else if strings.HasPrefix(e.Underlying, "spchar") {
			charKind = "sp"
		}
		padByte := byte(0x00)
		if charKind == "sp" {
			padByte = 0x20
		}
		// Pad or truncate to N bytes.
		bytes := make([]byte, n)
		for i := range bytes {
			bytes[i] = padByte
		}
		copy(bytes, raw)
		// Pack as little-endian integer.
		var val uint64
		for i := 0; i < n; i++ {
			val |= uint64(bytes[i]) << (uint(i) * 8)
		}
		var hex string
		switch n {
		case 2:
			hex = fmt.Sprintf("0x%04X", val)
		case 4:
			hex = fmt.Sprintf("0x%08X", val)
		case 8:
			hex = fmt.Sprintf("0x%016X", val)
		}
		// Format the comment showing the padded string.
		comment := formatStringComment(c.Value, charKind, n)
		return hex, comment
	}

	// Numeric enum.
	if c.Value[0] == '\'' {
		// Character literal: emit as Go rune literal with hex comment.
		ch := unquoteChar(c.Value)
		hex := fmt.Sprintf("0x%02X", ch)
		return c.Value, hex
	}

	// Decimal or hex numeric literal — pass through.
	return c.Value, ""
}

func unquoteString(s string) []byte {
	// Remove surrounding quotes.
	if len(s) < 2 {
		return nil
	}
	s = s[1 : len(s)-1]
	var result []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'x':
				if i+3 < len(s) {
					b, _ := strconv.ParseUint(s[i+2:i+4], 16, 8)
					result = append(result, byte(b))
					i += 3
				}
			case 'n':
				result = append(result, '\n')
				i++
			case '\\':
				result = append(result, '\\')
				i++
			case '"':
				result = append(result, '"')
				i++
			default:
				result = append(result, s[i+1])
				i++
			}
		} else {
			result = append(result, s[i])
		}
	}
	return result
}

func unquoteChar(s string) byte {
	// 'X' or '\xNN'
	if len(s) < 3 {
		return 0
	}
	inner := s[1 : len(s)-1]
	if inner[0] == '\\' {
		if len(inner) >= 4 && inner[1] == 'x' {
			b, _ := strconv.ParseUint(inner[2:4], 16, 8)
			return byte(b)
		}
		switch inner[1] {
		case 'n':
			return '\n'
		case '\\':
			return '\\'
		}
		return inner[1]
	}
	return inner[0]
}

func formatStringComment(value string, charKind string, n int) string {
	// Show the padded string as it appears in the packed integer.
	raw := unquoteString(value)
	padByte := byte(0x00)
	if charKind == "sp" {
		padByte = 0x20
	}
	bytes := make([]byte, n)
	for i := range bytes {
		bytes[i] = padByte
	}
	copy(bytes, raw)

	var buf strings.Builder
	buf.WriteByte('"')
	for _, b := range bytes {
		if b >= 0x20 && b <= 0x7e {
			buf.WriteByte(b)
		} else {
			buf.WriteString(fmt.Sprintf("\\x%02x", b))
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

func (g *Generator) emitConstGroup(cg *ConstGroupDecl) {
	if len(cg.Constants) == 1 {
		g.p("\nconst %s = %s\n", cg.Constants[0].Name, cg.Constants[0].Value)
		return
	}
	g.p("\nconst (\n")
	maxNameLen := 0
	for _, c := range cg.Constants {
		if len(c.Name) > maxNameLen {
			maxNameLen = len(c.Name)
		}
	}
	for _, c := range cg.Constants {
		pad := strings.Repeat(" ", maxNameLen-len(c.Name))
		g.p("\t%s%s = %s\n", c.Name, pad, c.Value)
	}
	g.p(")\n")
}

func (g *Generator) emitTypes() {
	for _, name := range g.a.Order {
		ti := g.a.Types[name]
		switch ti.Decl.Kind {
		case DeclEnum, DeclConstGroup:
			continue // already emitted
		case DeclStruct:
			g.emitStruct(ti)
		case DeclMessage:
			g.emitMessage(ti)
		case DeclUnion:
			g.emitUnion(ti)
		}
	}
}

func (g *Generator) writePrim(p PrimInfo, slice, val string) string {
	if p.Size == 1 {
		if p.Signed {
			return fmt.Sprintf("%s[0] = byte(%s)", slice, val)
		}
		return fmt.Sprintf("%s[0] = %s", slice, val)
	}
	order := "LittleEndian"
	if p.IsBE {
		order = "BigEndian"
	}
	fn := fmt.Sprintf("PutUint%d", p.Size*8)
	castVal := val
	if p.IsFloat {
		floatToBits := fmt.Sprintf("math.Float%dbits", p.Size*8)
		castVal = fmt.Sprintf("%s(%s)", floatToBits, val)
	} else if p.Signed {
		utype := strings.Replace(goType(p), "int", "uint", 1)
		castVal = fmt.Sprintf("%s(%s)", utype, val)
	}
	return fmt.Sprintf("binary.%s.%s(%s, %s)", order, fn, slice, castVal)
}

// offExpr returns "base" if off==0, "base+N" otherwise.
func offExpr(base string, off int) string {
	if off == 0 {
		return base
	}
	return fmt.Sprintf("%s+%d", base, off)
}

// sliceStr returns "base[start:end]" or "base[start:]" if end is empty.
func sliceStr(base, start, end string) string {
	if end != "" {
		return fmt.Sprintf("%s[%s:%s]", base, start, end)
	}
	return fmt.Sprintf("%s[%s:]", base, start)
}

func (g *Generator) readPrimFromSlice(p PrimInfo, slice string) string {
	if p.Size == 1 {
		if p.Signed {
			return fmt.Sprintf("int8(%s[0])", slice)
		}
		return fmt.Sprintf("%s[0]", slice)
	}
	order := "LittleEndian"
	if p.IsBE {
		order = "BigEndian"
	}
	fn := fmt.Sprintf("Uint%d", p.Size*8)
	expr := fmt.Sprintf("binary.%s.%s(%s)", order, fn, slice)
	if p.IsFloat {
		bitsToFloat := fmt.Sprintf("math.Float%dfrombits", p.Size*8)
		return fmt.Sprintf("%s(%s)", bitsToFloat, expr)
	}
	if p.Signed {
		return fmt.Sprintf("%s(%s)", goType(p), expr)
	}
	return expr
}

// readCountExpr returns a Go expression that reads a count/length field as int.
// For signed types, wraps with max(0, ...) to clamp negative values to zero.
func (g *Generator) readCountExpr(p PrimInfo, slice string) string {
	inner := g.readPrimFromSlice(p, slice)
	if p.Signed {
		return fmt.Sprintf("max(0, int(%s))", inner)
	}
	return fmt.Sprintf("int(%s)", inner)
}

func (g *Generator) fieldPrimInfo(f Field) PrimInfo {
	if p, ok := primitives[f.TypeName]; ok {
		return p
	}
	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclEnum {
		return enumPrimInfo(eti.Decl.Enum)
	}
	return primitives["leu32"]
}

// fieldPrimInfoOk returns the PrimInfo and true if the field is a primitive
// or enum type (i.e., a scalar). Returns false for struct/message/union types.
func (g *Generator) fieldPrimInfoOk(f Field) (PrimInfo, bool) {
	if p, ok := primitives[f.TypeName]; ok {
		return p, true
	}
	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclEnum {
		return enumPrimInfo(eti.Decl.Enum), true
	}
	return PrimInfo{}, false
}

func (g *Generator) extentPrimInfo(ti *TypeInfo) PrimInfo {
	m := ti.Decl.Message
	return g.fieldPrimInfo(m.Fields[g.a.findFieldIdx(m, ti.ExtentFieldName)])
}

func (g *Generator) extentMinComment(ti *TypeInfo) string {
	m := ti.Decl.Message
	var parts []string
	inExtent := false
	for _, f := range m.Fields {
		if f.Kind == FieldExtent {
			inExtent = true
			continue
		}
		if inExtent && f.Kind == FieldNormal {
			sz := g.a.typeSize(f.TypeName)
			parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
		}
	}
	return strings.Join(parts, " + ")
}
