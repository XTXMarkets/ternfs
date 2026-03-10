// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"strconv"
)

// emitStringMethods generates String() methods for all types and name
// functions for enums.
func (g *Generator) emitStringMethods() {
	g.p("\n" + sectionSep + "// Pretty-printing\n" + sectionSep)

	for _, name := range g.a.Order {
		ti := g.a.Types[name]
		switch ti.Decl.Kind {
		case DeclEnum:
			g.emitEnumNameFunc(ti)
		case DeclStruct:
			g.emitStructString(ti)
		case DeclMessage:
			g.emitMessageString(ti)
		case DeclUnion:
			g.emitUnionString(ti)
		}
	}
}

// emitEnumNameFunc generates a function that returns the string name for an
// enum value. Example:
//
//	func SensorTypeName(v uint16) string {
//	    switch v {
//	    case SENS_ACCEL: return "SENS_ACCEL"
//	    ...
//	    default: return fmt.Sprintf("sensor_type(%d)", v)
//	    }
//	}
func (g *Generator) emitEnumNameFunc(ti *TypeInfo) {
	e := ti.Decl.Enum
	goName := g.goName(ti)
	gt := enumGoType(e)
	funcName := goName + "Name"

	g.p("\nfunc %s(v %s) string {\n", funcName, gt)
	g.p("\tswitch v {\n")
	for _, c := range e.Constants {
		g.p("\tcase %s:\n\t\treturn %q\n", c.Name, c.Name)
	}
	// Default: show the raw value.
	g.p("\tdefault:\n\t\treturn fmt.Sprintf(\"%s(%%v)\", v)\n", e.Name)
	g.p("\t}\n}\n")
}

// emitStructString generates a String() method for a fixed-size struct.
func (g *Generator) emitStructString(ti *TypeInfo) {
	s := ti.Decl.Struct
	name := g.goName(ti)

	g.p("\nfunc (m %s) String() string {\n", name)
	g.p("\tvar b strings.Builder\n")
	g.p("\tb.WriteString(\"%s{\")\n", name)

	first := true
	for i, f := range s.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		_ = ti.FieldOffsets[i] // just to confirm we have offsets
		fieldGoName := pascalCase(f.Name)
		sep := ""
		if !first {
			sep = ", "
		}
		first = false

		g.emitFieldPrint(ti, f, fieldGoName, sep, false)
	}

	g.p("\tb.WriteString(\"}\")\n")
	g.p("\treturn b.String()\n}\n")
}

// emitMessageString generates a String() method for a variable-size message.
func (g *Generator) emitMessageString(ti *TypeInfo) {
	m := ti.Decl.Message
	name := g.goName(ti)

	g.p("\nfunc (m %s) String() string {\n", name)
	g.p("\tvar b strings.Builder\n")
	g.p("\tb.WriteString(\"%s{\")\n", name)

	// Skip count fields (they're implicit from the array they count).
	countFields := map[string]bool{}
	for _, f := range m.Fields {
		if f.Kind == FieldNormal && isVarLenArray(f) {
			countFields[f.ArrayLen] = true
		}
	}

	// Also skip disc fields (they're shown via the union).
	discFields := map[string]bool{}
	for _, f := range m.Fields {
		if f.Kind == FieldNormal && f.DiscRef != "" {
			discFields[f.DiscRef] = true
		}
	}

	first := true
	for _, f := range m.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		if countFields[f.Name] || discFields[f.Name] {
			continue
		}
		fieldGoName := pascalCase(f.Name)
		sep := ""
		if !first {
			sep = ", "
		}
		first = false

		g.emitFieldPrint(ti, f, fieldGoName, sep, true)
	}

	g.p("\tb.WriteString(\"}\")\n")
	g.p("\treturn b.String()\n}\n")
}

// emitFieldPrint emits code to print a single field into a strings.Builder.
// isMessage indicates whether the parent type is a message (vs struct).
func (g *Generator) emitFieldPrint(ti *TypeInfo, f Field, fieldGoName, sep string, isMessage bool) {
	prefix := sep + f.Name + ": "

	if f.DiscRef != "" {
		// Union field — print the union's String() method.
		g.p("\tfmt.Fprintf(&b, \"%s%%v\", m.%s())\n", prefix, fieldGoName)
		return
	}

	if isVarLenArray(f) {
		g.emitVarArrayPrint(ti, f, fieldGoName, prefix)
		return
	}

	if f.ArrayLen != "" {
		// Fixed-size array.
		n, err := strconv.Atoi(f.ArrayLen)
		if err != nil {
			return
		}
		p, isPrim := primitives[f.TypeName]
		if isPrim && p.IsChar {
			// char array — print as string/bytes.
			if p.CharKind == "" {
				g.p("\tfmt.Fprintf(&b, \"%s%%x\", m.%s())\n", prefix, fieldGoName)
			} else {
				g.p("\tfmt.Fprintf(&b, \"%s%%q\", m.%s())\n", prefix, fieldGoName)
			}
			return
		}
		if isPrim {
			// Fixed prim array — print each element.
			g.p("\tfmt.Fprintf(&b, \"%s[\")\n", prefix)
			g.p("\tfor i := 0; i < %d; i++ {\n", n)
			g.p("\t\tif i > 0 {\n\t\t\tb.WriteString(\", \")\n\t\t}\n")
			g.emitPrimFormatExpr(p, fmt.Sprintf("m.%s(i)", fieldGoName))
			g.p("\t}\n")
			g.p("\tb.WriteString(\"]\")\n")
			return
		}
		return
	}

	// Scalar: prim, enum, embedded struct, or embedded message.
	if p, ok := primitives[f.TypeName]; ok {
		g.p("\tfmt.Fprintf(&b, \"%s", prefix)
		g.p("%s\", m.%s())\n", primFmtVerb(p), fieldGoName)
		return
	}

	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclEnum {
		enumFuncName := g.goName(eti) + "Name"
		g.p("\tfmt.Fprintf(&b, \"%s%%s\", %s(m.%s()))\n", prefix, enumFuncName, fieldGoName)
		return
	}

	// Embedded struct or message — use its String() method.
	g.p("\tfmt.Fprintf(&b, \"%s%%v\", m.%s())\n", prefix, fieldGoName)
}

// emitVarArrayPrint emits code to print a variable-length array field.
func (g *Generator) emitVarArrayPrint(ti *TypeInfo, f Field, fieldGoName, prefix string) {
	elemSize := g.a.typeSize(f.TypeName)

	if isByteSliceType(f.TypeName) {
		// Blob field — show length and hex.
		p, _ := primitives[f.TypeName]
		if p.CharKind != "" {
			// npchar/spchar → show as string.
			g.p("\tfmt.Fprintf(&b, \"%s%%q\", m.%s())\n", prefix, fieldGoName)
		} else {
			// u8/char → show as hex with length.
			g.p("\t{\n")
			g.p("\t\td := m.%s()\n", fieldGoName)
			g.p("\t\tif len(d) <= 32 {\n")
			g.p("\t\t\tfmt.Fprintf(&b, \"%s%%x\", d)\n", prefix)
			g.p("\t\t} else {\n")
			g.p("\t\t\tfmt.Fprintf(&b, \"%s%%x...(%%d bytes)\", d[:32], len(d))\n", prefix)
			g.p("\t\t}\n")
			g.p("\t}\n")
		}
		return
	}

	if elemSize >= 0 {
		// Fixed-size elements with index-based access.
		if p, isPrim := primitives[f.TypeName]; isPrim {
			// Prim array with count.
			countField := f.ArrayLen
			g.p("\t{\n")
			g.p("\t\tn := int(m.%s())\n", pascalCase(countField))
			g.p("\t\tfmt.Fprintf(&b, \"%s[\")\n", prefix)
			g.p("\t\tfor i := 0; i < n && i < 64; i++ {\n")
			g.p("\t\t\tif i > 0 {\n\t\t\t\tb.WriteString(\", \")\n\t\t\t}\n")
			g.emitPrimFormatExprIndent(p, fmt.Sprintf("m.%s(i)", fieldGoName), "\t\t\t")
			g.p("\t\t}\n")
			g.p("\t\tif n > 64 {\n\t\t\tfmt.Fprintf(&b, \", ...(%%d total)\", n)\n\t\t}\n")
			g.p("\t\tb.WriteString(\"]\")\n")
			g.p("\t}\n")
			return
		}

		if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
			countField := f.ArrayLen
			g.p("\t{\n")
			g.p("\t\tn := int(m.%s())\n", pascalCase(countField))
			g.p("\t\tfmt.Fprintf(&b, \"%s[\")\n", prefix)
			g.p("\t\tfor i := 0; i < n && i < 64; i++ {\n")
			g.p("\t\t\tif i > 0 {\n\t\t\t\tb.WriteString(\", \")\n\t\t\t}\n")
			g.p("\t\t\tfmt.Fprintf(&b, \"%%v\", m.%s(i))\n", fieldGoName)
			g.p("\t\t}\n")
			g.p("\t\tif n > 64 {\n\t\t\tfmt.Fprintf(&b, \", ...(%%d total)\", n)\n\t\t}\n")
			g.p("\t\tb.WriteString(\"]\")\n")
			g.p("\t}\n")
			return
		}
		return
	}

	// Variable-size elements: use iterator.
	iterInfo := g.a.Iterators[f.TypeName]
	if iterInfo == nil {
		g.p("\tfmt.Fprintf(&b, \"%s[...]\")\n", prefix)
		return
	}
	accessorName := pascalCase(singularize(iterInfo.FieldName))

	g.p("\t{\n")
	g.p("\t\titer := m.%s()\n", fieldGoName)
	g.p("\t\tfmt.Fprintf(&b, \"%s[\")\n", prefix)
	g.p("\t\ti := 0\n")
	g.p("\t\tfor iter.Next() {\n")
	g.p("\t\t\tif i > 0 {\n\t\t\t\tb.WriteString(\", \")\n\t\t\t}\n")
	g.p("\t\t\tif i >= 64 {\n")
	g.p("\t\t\t\tb.WriteString(\"...\")\n")
	g.p("\t\t\t\tbreak\n")
	g.p("\t\t\t}\n")
	g.p("\t\t\tfmt.Fprintf(&b, \"%%v\", iter.%s())\n", accessorName)
	g.p("\t\t\ti++\n")
	g.p("\t\t}\n")
	g.p("\t\tb.WriteString(\"]\")\n")
	g.p("\t}\n")
}

// emitUnionString generates a String() method for a union type.
func (g *Generator) emitUnionString(ti *TypeInfo) {
	u := ti.Decl.Union
	name := g.goName(ti)

	// Detect payload types used by multiple arms.
	payloadCount := map[string]int{}
	for _, arm := range u.Arms {
		if arm.Payload != "void" && arm.Label != "default" {
			payloadCount[arm.Payload]++
		}
	}

	discEnum := g.a.Types[u.DiscType]

	g.p("\nfunc (m %s) String() string {\n", name)
	g.p("\tswitch m.disc {\n")

	for _, arm := range u.Arms {
		if arm.Label == "default" {
			continue
		}
		g.p("\tcase %s:\n", arm.Label)
		if arm.Payload == "void" {
			g.p("\t\treturn %q\n", stripEnumPrefix(arm.Label, discEnum.Decl.Enum.Constants))
		} else {
			payloadTi := g.a.Types[arm.Payload]
			methodName := g.goName(payloadTi)
			if payloadCount[arm.Payload] > 1 {
				methodName = armSuffix(stripEnumPrefix(arm.Label, discEnum.Decl.Enum.Constants))
			}
			g.p("\t\treturn fmt.Sprintf(\"%s:%%v\", m.As%s())\n",
				stripEnumPrefix(arm.Label, discEnum.Decl.Enum.Constants), methodName)
		}
	}

	// Default arm.
	g.p("\tdefault:\n")
	hasDefault := false
	for _, arm := range u.Arms {
		if arm.Label == "default" {
			hasDefault = true
			if arm.Payload == "void" {
				g.p("\t\treturn fmt.Sprintf(\"unknown(%%v)\", m.disc)\n")
			} else {
				payloadTi := g.a.Types[arm.Payload]
				g.p("\t\treturn fmt.Sprintf(\"%%v:%%v\", m.disc, m.As%s())\n", g.goName(payloadTi))
			}
			break
		}
	}
	if !hasDefault {
		g.p("\t\treturn fmt.Sprintf(\"unknown(%%v)\", m.disc)\n")
	}

	g.p("\t}\n}\n")
}

// primFmtVerb returns the fmt verb for a primitive type.
func primFmtVerb(p PrimInfo) string {
	if p.IsFloat {
		return "%g"
	}
	if p.IsChar {
		return "%c"
	}
	if p.Size >= 4 {
		return "%d"
	}
	return "%d"
}

// emitPrimFormatExpr emits a fmt.Fprintf call to print a primitive value.
func (g *Generator) emitPrimFormatExpr(p PrimInfo, expr string) {
	g.emitPrimFormatExprIndent(p, expr, "\t\t")
}

func (g *Generator) emitPrimFormatExprIndent(p PrimInfo, expr, indent string) {
	g.p("%sfmt.Fprintf(&b, \"%s\", %s)\n", indent, primFmtVerb(p), expr)
}
