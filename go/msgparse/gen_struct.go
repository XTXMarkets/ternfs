// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"strconv"
	"strings"
)

func (g *Generator) emitStruct(ti *TypeInfo) {
	s := ti.Decl.Struct
	name := g.goName(ti)
	sizeConst := sizeConstName(s.Name)

	g.p("\n" + sectionSep)
	g.p("// %s — fixed %d bytes", name, ti.Size)
	g.emitStructSizeComment(s, ti)
	g.p("\n" + sectionSep)

	// Type declaration.
	g.p("\ntype %s struct {\n\tm *[%s]byte\n}\n", name, sizeConst)
	g.p("\nconst %s = %d\n", sizeConst, ti.Size)

	// Read function.
	if ti.NeedsCursorRead {
		// Cursor-advancing read (lowercase).
		g.p("\nfunc read%s(b *[]byte) (%s, bool) {\n", name, name)
		g.p("\tif len(*b) < %s {\n\t\treturn %s{}, false\n\t}\n", sizeConst, name)
		g.p("\tresult := %s{m: (*[%s]byte)(*b)}\n", name, sizeConst)
		g.p("\t*b = (*b)[%s:]\n", sizeConst)
		g.p("\treturn result, true\n}\n")

		// Public Read wraps cursor-advancing form.
		g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)
		g.p("\treturn read%s(&b)\n}\n", name)
	} else {
		// Simple public Read.
		g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)
		g.p("\tif len(b) < %s {\n\t\treturn %s{}, false\n\t}\n", sizeConst, name)
		g.p("\treturn %s{m: (*[%s]byte)(b)}, true\n}\n", name, sizeConst)
	}

	// Start function (writer).
	g.p("\nfunc Start%s(buf []byte) ([]byte, %s) {\n", name, name)
	g.p("\tbuf = append(buf, make([]byte, %s)...)\n", sizeConst)
	g.p("\treturn buf, %s{m: (*[%s]byte)(buf[len(buf)-%s:])}\n}\n", name, sizeConst, sizeConst)

	// All getters first.
	for i, f := range s.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		off := ti.FieldOffsets[i]
		g.emitStructFieldGetter(name, f, off, ti)
	}
	// Then all setters.
	for i, f := range s.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		off := ti.FieldOffsets[i]
		g.emitStructFieldSetter(name, f, off, ti)
	}
}

func (g *Generator) emitStructSizeComment(s *StructDecl, ti *TypeInfo) {
	// Build a comment showing field layout.
	var parts []string
	for i, f := range s.Fields {
		switch f.Kind {
		case FieldPad:
			parts = append(parts, fmt.Sprintf("pad(%d)", f.Size))
		case FieldAlign:
			alignPad := ti.FieldOffsets[i]
			if i > 0 {
				alignPad = ti.FieldOffsets[i] - ti.FieldOffsets[i-1]
				if s.Fields[i-1].Kind == FieldNormal {
					alignPad = ti.FieldOffsets[i] + ((f.Size - (ti.FieldOffsets[i] % f.Size)) % f.Size) - ti.FieldOffsets[i]
				}
			}
			if alignPad > 0 {
				parts = append(parts, fmt.Sprintf("align(%d)=%d", f.Size, alignPad))
			}
		case FieldNormal:
			sz := g.a.fieldSize(f)
			if f.ArrayLen != "" {
				// Char array: show as "name(type[N])".
				parts = append(parts, fmt.Sprintf("%s(%s[%s])", f.Name, f.TypeName, f.ArrayLen))
			} else if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclEnum {
				// Enum field: show as "name(size, underlying)" for non-trivial underlying.
				u := eti.Decl.Enum.Underlying
				if strings.Contains(u, "[") || u == "u8" || u == "i8" {
					parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
				} else {
					parts = append(parts, fmt.Sprintf("%s(%d, %s)", f.Name, sz, u))
				}
			} else if _, ok := g.a.Types[f.TypeName]; ok {
				// Embedded struct: show type name in comment.
				parts = append(parts, fmt.Sprintf("%s(%d)", f.TypeName, sz))
			} else if p, ok := primitives[f.TypeName]; ok && p.IsBE {
				// Big-endian: annotate with type.
				parts = append(parts, fmt.Sprintf("%s(%d, %s)", f.Name, sz, f.TypeName))
			} else {
				parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
			}
		}
	}
	if len(parts) > 0 {
		joined := strings.Join(parts, " + ")
		// Check if it fits on one line with the prefix.
		prefix := fmt.Sprintf("// %s — fixed %d bytes: ", g.goName(ti), ti.Size)
		if len(prefix)+len(joined) <= 80 {
			g.p(": %s", joined)
		} else {
			g.p(":\n//   %s", joined)
		}
	}
}

func (g *Generator) emitStructFieldGetter(typeName string, f Field, off int, ti *TypeInfo) {
	fieldGoName := pascalCase(f.Name)

	if f.ArrayLen != "" {
		n, err := strconv.Atoi(f.ArrayLen)
		if err != nil {
			return
		}
		p, isPrim := primitives[f.TypeName]
		if isPrim && p.IsChar {
			g.emitCharArrayGetter(typeName, f, off, n, p)
			return
		}
		if isPrim {
			g.emitPrimArrayGetter(typeName, fieldGoName, off, p)
			return
		}
		return
	}

	p, isPrim := primitives[f.TypeName]
	if isPrim {
		g.emitPrimScalarGetter(typeName, fieldGoName, off, p)
		return
	}

	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclEnum {
		ep := enumPrimInfo(eti.Decl.Enum)
		g.emitPrimScalarGetter(typeName, fieldGoName, off, ep)
		return
	}

	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
		embSize := sizeConstName(f.TypeName)
		g.p("\nfunc (m %s) %s() %s {\n", typeName, fieldGoName, g.goName(eti))
		g.p("\treturn %s{m: (*[%s]byte)(m.m[%d:%d])}\n}\n",
			g.goName(eti), embSize, off, off+eti.Size)
		return
	}
}

func (g *Generator) emitStructFieldSetter(typeName string, f Field, off int, ti *TypeInfo) {
	fieldGoName := pascalCase(f.Name)

	if f.ArrayLen != "" {
		n, err := strconv.Atoi(f.ArrayLen)
		if err != nil {
			return
		}
		p, isPrim := primitives[f.TypeName]
		if isPrim && p.IsChar {
			g.emitCharArraySetter(typeName, f, off, n, p)
			return
		}
		if isPrim {
			g.emitPrimArraySetter(typeName, fieldGoName, off, p)
			return
		}
		return
	}

	p, isPrim := primitives[f.TypeName]
	if isPrim {
		g.emitPrimScalarSetter(typeName, fieldGoName, off, p)
		return
	}

	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclEnum {
		ep := enumPrimInfo(eti.Decl.Enum)
		g.emitPrimScalarSetter(typeName, fieldGoName, off, ep)
		return
	}
	// Embedded structs have no setter — they return a mutable view.
}

func (g *Generator) emitCharArrayGetter(typeName string, f Field, off, n int, p PrimInfo) {
	fieldGoName := pascalCase(f.Name)
	end := off + n

	if p.CharKind == "" {
		g.p("\n// %s returns the raw char[%d] field as a byte slice.\n", fieldGoName, n)
		g.p("func (m %s) %s() []byte {\n", typeName, fieldGoName)
		g.p("\treturn m.m[%d:%d]\n}\n", off, end)
	} else {
		padByte := "0x00"
		if p.CharKind == "sp" {
			padByte = "0x20"
		}
		g.p("\n// %s returns %schar[%d] with trailing %s stripped.\n", fieldGoName, p.CharKind, n, padByte)
		g.p("func (m %s) %s() string {\n", typeName, fieldGoName)
		g.p("\ts := m.m[%d:%d]\n", off, end)
		g.p("\ti := len(s)\n")
		g.p("\tfor i > 0 && s[i-1] == %s {\n\t\ti--\n\t}\n", padByte)
		g.p("\treturn string(s[:i])\n}\n")
	}
}

func (g *Generator) emitCharArraySetter(typeName string, f Field, off, n int, p PrimInfo) {
	fieldGoName := pascalCase(f.Name)
	end := off + n

	padByte := "0x00"
	if p.CharKind == "sp" {
		padByte = "0x20"
	}
	valType := "string"
	if p.CharKind == "" {
		valType = "[]byte"
	}
	g.p("\n// Set%s writes v into %schar[%d], padding with %s or truncating.\n", fieldGoName, p.CharKind, n, padByte)
	g.p("func (m %s) Set%s(v %s) {\n", typeName, fieldGoName, valType)
	g.p("\tb := m.m[%d:%d]\n", off, end)
	g.p("\tn := copy(b, v)\n")
	g.p("\tfor i := n; i < len(b); i++ {\n\t\tb[i] = %s\n\t}\n}\n", padByte)
}

func (g *Generator) emitPrimScalarGetter(typeName, fieldGoName string, off int, p PrimInfo) {
	g.p("\nfunc (m %s) %s() %s {\n", typeName, fieldGoName, goType(p))
	if p.Size == 1 {
		if p.Signed {
			g.p("\treturn int8(m.m[%d])\n", off)
		} else {
			g.p("\treturn m.m[%d]\n", off)
		}
	} else {
		g.p("\treturn %s\n", g.readPrimFromSlice(p, fmt.Sprintf("m.m[%d:%d]", off, off+p.Size)))
	}
	g.p("}\n")
}

func (g *Generator) emitPrimScalarSetter(typeName, fieldGoName string, off int, p PrimInfo) {
	g.p("\nfunc (m %s) Set%s(v %s) {\n", typeName, fieldGoName, goType(p))
	if p.Size == 1 {
		if p.Signed {
			g.p("\tm.m[%d] = byte(v)\n", off)
		} else {
			g.p("\tm.m[%d] = v\n", off)
		}
	} else {
		g.p("\t%s\n", g.writePrim(p, fmt.Sprintf("m.m[%d:%d]", off, off+p.Size), "v"))
	}
	g.p("}\n")
}

func (g *Generator) emitPrimArrayGetter(typeName, fieldGoName string, baseOff int, p PrimInfo) {
	g.p("\nfunc (m %s) %s(i int) %s {\n", typeName, fieldGoName, goType(p))
	g.p("\toff := %d + i*%d\n", baseOff, p.Size)
	g.p("\treturn %s\n}\n", g.readPrimFromSlice(p, fmt.Sprintf("m.m[off : off+%d]", p.Size)))
}

func (g *Generator) emitPrimArraySetter(typeName, fieldGoName string, baseOff int, p PrimInfo) {
	gt := goType(p)
	elemSize := p.Size
	g.p("\nfunc (m %s) Set%s(i int, v %s) {\n", typeName, fieldGoName, gt)
	g.p("\toff := %d + i*%d\n", baseOff, elemSize)
	g.p("\t%s\n}\n", g.writePrim(p, fmt.Sprintf("m.m[off:off+%d]", elemSize), "v"))
}
