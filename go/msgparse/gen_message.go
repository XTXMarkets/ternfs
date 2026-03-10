// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"strconv"
	"strings"
)

func (g *Generator) emitMessage(ti *TypeInfo) {
	m := ti.Decl.Message
	name := g.goName(ti)

	g.p("\n" + sectionSep)
	g.p("// %s — variable", name)
	g.emitMessageSizeComment(m, ti)
	g.p("\n" + sectionSep)

	// Type declaration.
	if ti.IsExtentStruct {
		g.p("\ntype %s struct {\n\tm *[%d]byte\n}\n", name, ti.ExtentTotalSize)
	} else if ti.IsMultiArray {
		g.p("\ntype %s struct {\n\tdata []byte\n\toffB int // byte offset within data where ", name)
		g.emitMultiArrayOffComment(m, ti)
		g.p("\n}\n")
	} else if len(ti.VarSectionBoundaries) > 0 {
		g.p("\ntype %s struct {\n\tdata []byte\n", name)
		for bi, b := range ti.VarSectionBoundaries {
			// Find the next normal field after the boundary to describe the offset.
			nextName := ""
			for j := b.FieldIdx + 1; j < len(m.Fields); j++ {
				if m.Fields[j].Kind == FieldNormal {
					nextName = m.Fields[j].Name
					break
				}
			}
			g.p("\t%s int // byte offset within data where %s starts\n", offName(bi), nextName)
		}
		g.p("}\n")
	} else {
		g.p("\ntype %s []byte\n", name)
	}

	// Read functions.
	g.emitMessageReads(ti)

	// Getters.
	g.emitMessageGetters(ti)

	// Iterator types (emitted after the first message that uses them).
	g.emitIteratorsForMessage(ti)

	// Writer.
	g.emitMessageWriter(ti)
}

func (g *Generator) emitMessageSizeComment(m *MessageDecl, ti *TypeInfo) {
	var parts []string
	for _, f := range m.Fields {
		switch f.Kind {
		case FieldPad:
			parts = append(parts, fmt.Sprintf("pad(%d)", f.Size))
		case FieldAlign:
			parts = append(parts, fmt.Sprintf("align(%d)", f.Size))
		case FieldExtent:
			// Don't show extent as a separate field; fields after it are "inside" it.
		case FieldNormal:
			if isVarLenArray(f) {
				parts = append(parts, fmt.Sprintf("%s %s[%s]", f.TypeName, f.Name, f.ArrayLen))
			} else if f.ArrayLen != "" {
				sz := g.a.fieldSize(f)
				parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
			} else if f.DiscRef != "" {
				parts = append(parts, fmt.Sprintf("%s %s", f.TypeName, f.Name))
			} else {
				sz := g.a.typeSize(f.TypeName)
				if sz > 0 {
					parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
				} else {
					parts = append(parts, f.Name)
				}
			}
		}
	}
	if len(parts) > 0 {
		joined := strings.Join(parts, " + ")
		prefix := fmt.Sprintf("// %s — variable: ", g.goName(ti))
		if len(prefix)+len(joined) <= 80 {
			g.p(": %s", joined)
		} else {
			g.p(":\n//   %s", joined)
		}
	}
}

func (g *Generator) emitMultiArrayOffComment(m *MessageDecl, ti *TypeInfo) {
	// Find the second array field to describe what offB points to.
	count := 0
	for _, f := range m.Fields {
		if f.Kind == FieldNormal && isVarLenArray(f) {
			count++
			if count == 2 {
				if ti.MultiArrayKind == 1 {
					g.p("%s starts", f.Name)
				} else {
					g.p("%s starts", f.ArrayLen) // count field for second array
				}
				return
			}
		}
	}
}

func (g *Generator) emitMessageReads(ti *TypeInfo) {
	name := g.goName(ti)

	if ti.NeedsCursorRead {
		g.emitCursorRead(ti)
	}

	// Public Read.
	switch ti.ReadPattern {
	case "compute":
		g.emitComputeRead(ti)
	case "wrap":
		g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)
		g.p("\treturn read%s(&b)\n}\n", name)
	case "lazy":
		g.emitLazyRead(ti)
	case "extent":
		g.emitExtentRead(ti)
	case "multiarray":
		g.emitMultiArrayRead(ti)
	}
}

func (g *Generator) emitCursorRead(ti *TypeInfo) {
	m := ti.Decl.Message
	name := g.goName(ti)

	// Determine return type: []byte for extent-struct messages, TypeName otherwise.
	retType := name
	nilVal := "nil"
	if ti.IsExtentStruct {
		retType = "[]byte"
	} else if ti.IsMultiArray || len(ti.VarSectionBoundaries) > 0 {
		retType = name
		nilVal = name + "{}"
	}

	g.p("\nfunc read%s(b *[]byte) (%s, bool) {\n", name, retType)

	if ti.HasExtent && ti.IsExtentStruct {
		// Extent-based struct: validate header + extent, return []byte.
		g.p("\tif len(*b) < %d {\n\t\treturn nil, false\n\t}\n", ti.TotalFixedSize)
		extPrim := g.extentPrimInfo(ti)
		g.p("\t%s := %s\n",
			camelCase(ti.ExtentFieldName), g.readCountExpr(extPrim, fmt.Sprintf("(*b)[%d:%d]", ti.ExtentFieldOff, ti.ExtentFieldOff+extPrim.Size)))
		g.p("\tif %s < %d { // extent must hold at least %s\n\t\treturn nil, false\n\t}\n",
			camelCase(ti.ExtentFieldName), ti.KnownBodySize, g.extentMinComment(ti))
		g.p("\ttotal := %d + %s\n", ti.TotalFixedSize, camelCase(ti.ExtentFieldName))
		g.p("\tif len(*b) < total {\n\t\treturn nil, false\n\t}\n")
		g.p("\tresult := (*b)[:total]\n")
		g.p("\t*b = (*b)[total:]\n")
		g.p("\treturn result, true\n}\n")
		return
	}

	if ti.IsMultiArray {
		g.emitMultiArrayCursorRead(ti)
		return
	}

	// Optimized cursor read for byte-slice messages (count + u8 data + optional align).
	// Computes total size directly from the count, doing one bounds check.
	if g.isByteSliceMessage(ti) {
		va := ti.VarArrays[0]
		alignSize := ti.AlignSize
		lenPrim := g.fieldPrimInfo(va.CountField)
		headerSize := ti.TotalFixedSize

		g.p("\tif len(*b) < %d {\n\t\treturn nil, false\n\t}\n", headerSize)
		g.p("\tn := %s\n", g.readCountExpr(lenPrim, fmt.Sprintf("(*b)[%d:%d]", va.CountOff, va.CountOff+lenPrim.Size)))

		if alignSize > 0 {
			g.p("\tpadded := (n + %d) &^ %d\n", alignSize-1, alignSize-1)
			g.p("\ttotal := %d + padded\n", headerSize)
		} else {
			g.p("\ttotal := %d + n\n", headerSize)
		}

		g.p("\tif len(*b) < total {\n\t\treturn nil, false\n\t}\n")
		g.p("\tresult := %s((*b)[:total])\n", name)
		g.p("\t*b = (*b)[total:]\n")
		g.p("\treturn result, true\n}\n")
		return
	}

	realHeader := ti.FixedPrefixSize

	// General cursor-advancing read.
	if realHeader > 0 {
		g.p("\tif len(*b) < %d {\n\t\treturn %s, false\n\t}\n", realHeader, nilVal)
	}

	// Check if there's any variable content at all.
	hasVarContent := false
	for _, f := range m.Fields {
		if g.a.isVarContentField(f) {
			hasVarContent = true
			break
		}
	}

	if !hasVarContent && !ti.HasExtent {
		// Fixed-size message (shouldn't happen, but handle gracefully).
		g.p("\tresult := %s((*b)[:%d])\n", name, realHeader)
		g.p("\t*b = (*b)[%d:]\n", realHeader)
		g.p("\treturn result, true\n}\n")
		return
	}

	g.p("\tstart := *b\n")
	g.p("\tstartLen := len(start)\n")

	// Pre-read count fields that are in the contiguous header.
	// Prefix local variable names with c_ to avoid shadowing Go builtins.
	for _, f := range m.Fields {
		if f.Kind == FieldNormal && ti.CountFields[f.Name] {
			idx := g.a.findFieldIdx(m, f.Name)
			off := ti.FieldOffsets[idx]
			if off < realHeader {
				p := g.fieldPrimInfo(f)
				g.p("\tc_%s := %s\n", f.Name,
					g.readCountExpr(p, fmt.Sprintf("(*b)[%d:%d]", off, off+p.Size)))
			}
		}
	}

	// Advance past the contiguous header.
	if realHeader > 0 {
		g.p("\t*b = (*b)[%d:]\n", realHeader)
	}

	// Build boundary map for var-section offset recording.
	boundaryAfter := map[int]string{} // fieldIdx → offset variable name
	for bi, b := range ti.VarSectionBoundaries {
		boundaryAfter[b.FieldIdx] = offName(bi)
	}

	// Process fields past the contiguous header in wire format order.
	for fieldIdx, f := range m.Fields {
		if f.Kind == FieldAlign {
			// Alignment padding after variable data.
			g.p("\t{\n\t\toff := startLen - len(*b)\n")
			g.p("\t\taligned := (off + %d) &^ %d\n", f.Size-1, f.Size-1)
			g.p("\t\tif aligned > startLen {\n\t\t\treturn %s, false\n\t\t}\n", nilVal)
			g.p("\t\t*b = start[aligned:]\n\t}\n")
			continue
		}
		if f.Kind != FieldNormal {
			continue
		}
		// Skip fields in the fixed prefix (already advanced past).
		if !g.a.isVarContentField(f) && ti.FieldOffsets[fieldIdx] < realHeader {
			continue
		}

		// Handle fields after the header, in order.
		if isVarLenArray(f) {
			elemSize := g.a.typeSize(f.TypeName)
			if elemSize < 0 {
				elemReadName := "read" + pascalCase(f.TypeName)
				g.p("\tfor i := 0; i < c_%s; i++ {\n", f.ArrayLen)
				g.p("\t\tif _, ok := %s(b); !ok {\n", elemReadName)
				g.p("\t\t\treturn %s, false\n\t\t}\n\t}\n", nilVal)
			} else {
				g.p("\tif len(*b) < c_%s*%d {\n\t\treturn %s, false\n\t}\n", f.ArrayLen, elemSize, nilVal)
				g.p("\t*b = (*b)[c_%s*%d:]\n", f.ArrayLen, elemSize)
			}
			if offName, ok := boundaryAfter[fieldIdx]; ok {
				g.p("\t%s := startLen - len(*b)\n", offName)
			}
			continue
		}
		if f.ArrayLen != "" {
			// Fixed-size array after variable content — advance past it.
			sz := g.a.fieldSize(f)
			g.p("\tif len(*b) < %d {\n\t\treturn %s, false\n\t}\n", sz, nilVal)
			g.p("\t*b = (*b)[%d:]\n", sz)
			if offName, ok := boundaryAfter[fieldIdx]; ok {
				g.p("\t%s := startLen - len(*b)\n", offName)
			}
			continue
		}

		if f.DiscRef != "" {
			// Union field — get discriminant value.
			discIdx := g.a.findFieldIdx(m, f.DiscRef)
			discOff := ti.FieldOffsets[discIdx]
			discPrim := g.fieldPrimInfo(m.Fields[discIdx])
			readFn := "read" + pascalCase(f.TypeName)
			var discExpr string
			if discOff < realHeader {
				// Disc is in the contiguous header — read from start.
				discExpr = g.readPrimFromSlice(discPrim,
					fmt.Sprintf("start[%d:%d]", discOff, discOff+discPrim.Size))
			} else {
				// Disc was read inline as a local variable.
				discExpr = "c_" + f.DiscRef
			}
			g.p("\tif _, ok := %s(b, %s); !ok {\n", readFn, discExpr)
			g.p("\t\treturn %s, false\n\t}\n", nilVal)
			if offName, ok := boundaryAfter[fieldIdx]; ok {
				g.p("\t%s := startLen - len(*b)\n", offName)
			}
			continue
		}

		if g.a.isVarSizeType(f.TypeName) {
			// Embedded variable-size message.
			readFn := "read" + pascalCase(f.TypeName)
			g.p("\tif _, ok := %s(b); !ok {\n", readFn)
			g.p("\t\treturn %s, false\n\t}\n", nilVal)
			if offName, ok := boundaryAfter[fieldIdx]; ok {
				g.p("\t%s := startLen - len(*b)\n", offName)
			}
			continue
		}

		// Fixed-size field after variable content — read inline and advance.
		sz := g.a.typeSize(f.TypeName)
		if sz <= 0 {
			continue
		}
		g.p("\tif len(*b) < %d {\n\t\treturn %s, false\n\t}\n", sz, nilVal)
		if ti.CountFields[f.Name] {
			// Count field — store in local variable.
			p := g.fieldPrimInfo(f)
			g.p("\tc_%s := %s\n", f.Name,
				g.readCountExpr(p, fmt.Sprintf("(*b)[:%d]", p.Size)))
		}
		if ti.DiscFields[f.Name] {
			p := g.fieldPrimInfo(f)
			g.p("\tc_%s := %s\n", f.Name,
				g.readPrimFromSlice(p, fmt.Sprintf("(*b)[:%d]", p.Size)))
		}
		g.p("\t*b = (*b)[%d:]\n", sz)
	}

	g.p("\ttotal := startLen - len(*b)\n")
	if len(ti.VarSectionBoundaries) > 0 {
		// Construct struct with stored offsets.
		g.p("\treturn %s{data: start[:total]", name)
		for bi := range ti.VarSectionBoundaries {
			on := offName(bi)
			g.p(", %s: %s", on, on)
		}
		g.p("}, true\n}\n")
	} else {
		g.p("\treturn %s(start[:total]), true\n}\n", name)
	}
}

func (g *Generator) emitComputeRead(ti *TypeInfo) {
	name := g.goName(ti)
	va := ti.VarArrays[0]
	countPrim := g.fieldPrimInfo(va.CountField)
	alignSize := ti.AlignSize

	elemSize := g.a.typeSize(va.ArrayField.TypeName)
	elemSizeStr := strconv.Itoa(elemSize)
	// Use size constant if it's a struct.
	if eti, ok := g.a.Types[va.ArrayField.TypeName]; ok && eti.Decl.Kind == DeclStruct {
		elemSizeStr = sizeConstName(va.ArrayField.TypeName)
	}

	g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)
	g.p("\tif len(b) < %d {\n\t\treturn nil, false\n\t}\n", ti.TotalFixedSize)
	g.p("\tcount := %s\n", g.readCountExpr(countPrim, fmt.Sprintf("b[%d:%d]", va.CountOff, va.CountOff+countPrim.Size)))
	if alignSize > 0 && elemSize == 1 {
		g.p("\tpadded := (count + %d) &^ %d\n", alignSize-1, alignSize-1)
		g.p("\ttotal := %d + padded\n", ti.TotalFixedSize)
	} else {
		g.p("\ttotal := %d + count*%s\n", ti.TotalFixedSize, elemSizeStr)
	}
	g.p("\tif len(b) < total {\n\t\treturn nil, false\n\t}\n")
	g.p("\treturn %s(b[:total]), true\n}\n", name)
}

func (g *Generator) emitLazyRead(ti *TypeInfo) {
	name := g.goName(ti)
	g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)
	g.p("\tif len(b) < %d {\n\t\treturn nil, false\n\t}\n", ti.TotalFixedSize)
	g.p("\treturn %s(b), true\n}\n", name)
}

func (g *Generator) emitExtentRead(ti *TypeInfo) {
	name := g.goName(ti)
	g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)
	g.p("\tif len(b) < %d {\n\t\treturn %s{}, false\n\t}\n", ti.TotalFixedSize, name)
	extPrim := g.extentPrimInfo(ti)
	g.p("\t%s := %s\n",
		camelCase(ti.ExtentFieldName), g.readCountExpr(extPrim, fmt.Sprintf("b[%d:%d]", ti.ExtentFieldOff, ti.ExtentFieldOff+extPrim.Size)))
	g.p("\tif %s < %d {\n\t\treturn %s{}, false\n\t}\n",
		camelCase(ti.ExtentFieldName), ti.KnownBodySize, name)
	g.p("\ttotal := %d + %s\n", ti.TotalFixedSize, camelCase(ti.ExtentFieldName))
	g.p("\tif len(b) < total {\n\t\treturn %s{}, false\n\t}\n", name)
	g.p("\treturn %s{m: (*[%d]byte)(b)}, true\n}\n", name, ti.ExtentTotalSize)
}

func (g *Generator) emitMultiArrayRead(ti *TypeInfo) {
	name := g.goName(ti)
	arrays := ti.VarArrays

	if len(arrays) < 2 {
		return
	}

	countAPrim := g.fieldPrimInfo(arrays[0].CountField)
	headerSize := ti.TotalFixedSize

	g.p("\nfunc Read%s(b []byte) (%s, bool) {\n", name, name)

	if ti.MultiArrayKind == 1 {
		// Both counts in header.
		g.p("\tif len(b) < %d {\n\t\treturn %s{}, false\n\t}\n", headerSize, name)
		g.p("\t%s := %s\n", arrays[0].CountField.Name,
			g.readCountExpr(countAPrim, fmt.Sprintf("b[%d:%d]", arrays[0].CountOff, arrays[0].CountOff+countAPrim.Size)))
		g.p("\trest := b[%d:]\n", headerSize)
		elemRead := "read" + pascalCase(arrays[0].ArrayField.TypeName)
		g.p("\tfor i := 0; i < %s; i++ {\n", arrays[0].CountField.Name)
		g.p("\t\tif _, ok := %s(&rest); !ok {\n", elemRead)
		g.p("\t\t\treturn %s{}, false\n\t\t}\n\t}\n", name)
		g.p("\toffB := len(b) - len(rest)\n")
		g.p("\treturn %s{data: b, offB: offB}, true\n}\n", name)
	} else {
		// Interleaved counts.
		g.p("\tif len(b) < %d {\n\t\treturn %s{}, false\n\t}\n", countAPrim.Size, name)
		g.p("\t%s := %s\n", arrays[0].CountField.Name,
			g.readCountExpr(countAPrim, fmt.Sprintf("b[%d:%d]", arrays[0].CountOff, arrays[0].CountOff+countAPrim.Size)))
		dataStart := arrays[0].CountOff + countAPrim.Size
		g.p("\trest := b[%d:]\n", dataStart)
		elemRead := "read" + pascalCase(arrays[0].ArrayField.TypeName)
		g.p("\tfor i := 0; i < %s; i++ {\n", arrays[0].CountField.Name)
		g.p("\t\tif _, ok := %s(&rest); !ok {\n", elemRead)
		g.p("\t\t\treturn %s{}, false\n\t\t}\n\t}\n", name)
		g.p("\toffB := len(b) - len(rest)\n")
		countBPrim := g.fieldPrimInfo(arrays[1].CountField)
		g.p("\tif len(rest) < %d {\n\t\treturn %s{}, false\n\t}\n", countBPrim.Size, name)
		g.p("\treturn %s{data: b, offB: offB}, true\n}\n", name)
	}
}

func (g *Generator) emitMultiArrayCursorRead(ti *TypeInfo) {
	name := g.goName(ti)
	arrays := ti.VarArrays
	headerSize := ti.TotalFixedSize

	if ti.MultiArrayKind == 1 {
		// Both counts in header.
		g.p("\tif len(*b) < %d {\n\t\treturn %s{}, false\n\t}\n", headerSize, name)

		for _, arr := range arrays {
			cp := g.fieldPrimInfo(arr.CountField)
			g.p("\t%s := %s\n", arr.CountField.Name,
				g.readCountExpr(cp, fmt.Sprintf("(*b)[%d:%d]", arr.CountOff, arr.CountOff+cp.Size)))
		}

		g.p("\tstart := *b\n")
		g.p("\t*b = (*b)[%d:]\n", headerSize)

		elemRead := "read" + pascalCase(arrays[0].ArrayField.TypeName)
		g.p("\tfor i := 0; i < %s; i++ {\n", arrays[0].CountField.Name)
		g.p("\t\tif _, ok := %s(b); !ok {\n", elemRead)
		g.p("\t\t\treturn %s{}, false\n\t\t}\n\t}\n", name)

		g.p("\toffB := len(start) - len(*b)\n")

		elemRead2 := "read" + pascalCase(arrays[1].ArrayField.TypeName)
		g.p("\tfor i := 0; i < %s; i++ {\n", arrays[1].CountField.Name)
		g.p("\t\tif _, ok := %s(b); !ok {\n", elemRead2)
		g.p("\t\t\treturn %s{}, false\n\t\t}\n\t}\n", name)

		g.p("\ttotal := len(start) - len(*b)\n")
		g.p("\treturn %s{data: start[:total], offB: offB}, true\n}\n", name)
	} else {
		// Interleaved counts.
		countAPrim := g.fieldPrimInfo(arrays[0].CountField)
		g.p("\tif len(*b) < %d {\n\t\treturn %s{}, false\n\t}\n", countAPrim.Size, name)
		g.p("\t%s := %s\n", arrays[0].CountField.Name,
			g.readCountExpr(countAPrim, fmt.Sprintf("(*b)[%d:%d]", arrays[0].CountOff, arrays[0].CountOff+countAPrim.Size)))
		g.p("\tstart := *b\n")
		dataStart := arrays[0].CountOff + countAPrim.Size
		g.p("\t*b = (*b)[%d:]\n", dataStart)

		elemRead := "read" + pascalCase(arrays[0].ArrayField.TypeName)
		g.p("\tfor i := 0; i < %s; i++ {\n", arrays[0].CountField.Name)
		g.p("\t\tif _, ok := %s(b); !ok {\n", elemRead)
		g.p("\t\t\treturn %s{}, false\n\t\t}\n\t}\n", name)

		g.p("\toffB := len(start) - len(*b)\n")

		countBPrim := g.fieldPrimInfo(arrays[1].CountField)
		g.p("\tif len(*b) < %d {\n\t\treturn %s{}, false\n\t}\n", countBPrim.Size, name)
		g.p("\t%s := %s\n", arrays[1].CountField.Name,
			g.readCountExpr(countBPrim, fmt.Sprintf("(*b)[0:%d]", countBPrim.Size)))
		g.p("\t*b = (*b)[%d:]\n", countBPrim.Size)

		elemRead2 := "read" + pascalCase(arrays[1].ArrayField.TypeName)
		g.p("\tfor i := 0; i < %s; i++ {\n", arrays[1].CountField.Name)
		g.p("\t\tif _, ok := %s(b); !ok {\n", elemRead2)
		g.p("\t\t\treturn %s{}, false\n\t\t}\n\t}\n", name)

		g.p("\ttotal := len(start) - len(*b)\n")
		g.p("\treturn %s{data: start[:total], offB: offB}, true\n}\n", name)
	}
}

func (g *Generator) emitMessageGetters(ti *TypeInfo) {
	m := ti.Decl.Message
	name := g.goName(ti)

	receiver := "m"
	dataExpr := receiver
	if ti.IsMultiArray || len(ti.VarSectionBoundaries) > 0 {
		dataExpr = "m.data"
	} else if ti.IsExtentStruct {
		dataExpr = "m.m"
	}

	// Build set of count fields emitted by boundary array handlers (avoid duplicates).
	boundaryCountFields := map[string]bool{}
	if len(ti.VarSectionBoundaries) > 0 {
		for i, f := range m.Fields {
			if f.Kind == FieldNormal && isVarLenArray(f) && ti.FieldOffsets[i] >= ti.FixedPrefixSize {
				boundaryCountFields[f.ArrayLen] = true
			}
		}
	}

	for i, f := range m.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		// Skip count fields whose getter is emitted by the boundary array handler.
		if boundaryCountFields[f.Name] {
			continue
		}
		off := ti.FieldOffsets[i]
		fieldGoName := pascalCase(f.Name)

		// For messages with boundaries, fields past the prefix use stored offsets.
		if len(ti.VarSectionBoundaries) > 0 && off >= ti.FixedPrefixSize {
			g.emitBoundaryGetter(ti, i, f)
			continue
		}

		if isVarLenArray(f) {
			g.emitVarArrayGetter(ti, f, dataExpr)
			continue
		}
		if f.ArrayLen != "" {
			continue // fixed-size array in message header
		}

		if f.DiscRef != "" {
			// Union field — return union type from remaining bytes, with disc for safety.
			unionGoType := pascalCase(f.TypeName)
			discIdx := g.a.findFieldIdx(m, f.DiscRef)
			discOff := ti.FieldOffsets[discIdx]
			discPrim := g.fieldPrimInfo(m.Fields[discIdx])
			discExpr := g.readPrimFromSlice(discPrim, fmt.Sprintf("%s[%d:%d]", dataExpr, discOff, discOff+discPrim.Size))
			g.p("\nfunc (%s %s) %s() %s {\n", receiver, name, fieldGoName, unionGoType)
			g.p("\treturn %s{b: %s[%d:], disc: %s}\n}\n", unionGoType, dataExpr, off, discExpr)
			continue
		}

		// Scalar field (primitive or enum).
		if p, ok := g.fieldPrimInfoOk(f); ok {
			// For interleaved multi-array, the second array's count is at m.offB.
			if ti.IsMultiArray && ti.MultiArrayKind == 2 && len(ti.VarArrays) >= 2 &&
				ti.VarArrays[1].CountField.Name == f.Name {
				g.p("\nfunc (%s %s) %s() %s {\n", receiver, name, fieldGoName, goType(p))
				g.p("\treturn %s\n}\n", g.readPrimFromSlice(p, fmt.Sprintf("%s[m.offB:m.offB+%d]", dataExpr, p.Size)))
			} else {
				g.p("\nfunc (%s %s) %s() %s {\n", receiver, name, fieldGoName, goType(p))
				g.p("\treturn %s\n}\n", g.readPrimFromSlice(p, fmt.Sprintf("%s[%d:%d]", dataExpr, off, off+p.Size)))
			}
			continue
		}

		// Embedded struct.
		if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
			embSize := sizeConstName(f.TypeName)
			g.p("\nfunc (%s %s) %s() %s {\n", receiver, name, fieldGoName, g.goName(eti))
			g.p("\treturn %s{m: (*[%s]byte)(%s[%d:%s])}\n}\n",
				g.goName(eti), embSize, dataExpr, off, fmt.Sprintf("%d+%s", off, embSize))
			continue
		}

		// Embedded variable-size message.
		if eti, ok := g.a.Types[f.TypeName]; ok && (eti.Decl.Kind == DeclMessage || eti.Decl.Kind == DeclUnion) {
			g.p("\nfunc (%s %s) %s() %s {\n", receiver, name, fieldGoName, g.goName(eti))
			if g.a.isStructReader(f.TypeName) {
				g.p("\tv, _ := Read%s(%s[%d:])\n\treturn v\n}\n", g.goName(eti), dataExpr, off)
			} else {
				g.p("\treturn %s(%s[%d:])\n}\n", g.goName(eti), dataExpr, off)
			}
			continue
		}
	}
}

// boundaryInfo computes the position of a field past the fixed prefix in a
// message with VarSectionBoundaries. Returns:
//   - baseExpr: Go expression for the boundary base offset (e.g., "m.off1" or "4")
//   - relOff: additional static offset from the base
//   - endExpr: Go expression for the end of this section ("" if open-ended)
func (g *Generator) boundaryInfo(ti *TypeInfo, fieldIdx int) (baseExpr string, relOff int, endExpr string) {
	m := ti.Decl.Message
	bounds := ti.VarSectionBoundaries

	// Find which boundary this field is after.
	bIdx := -1
	for i, b := range bounds {
		if b.FieldIdx < fieldIdx {
			bIdx = i
		}
	}

	// Compute relative offset from section start (sum of fixed-size bytes).
	startFieldIdx := 0
	baseExpr = fmt.Sprintf("%d", ti.FixedPrefixSize)
	if bIdx >= 0 {
		startFieldIdx = bounds[bIdx].FieldIdx + 1
		baseExpr = fmt.Sprintf("m.%s", offName(bIdx))
	}
	for j := startFieldIdx; j < fieldIdx; j++ {
		fj := m.Fields[j]
		if fj.Kind != FieldNormal || ti.FieldOffsets[j] < ti.FixedPrefixSize {
			continue
		}
		sz := g.a.typeSize(fj.TypeName)
		if sz > 0 && fj.ArrayLen == "" && fj.DiscRef == "" {
			relOff += sz
		}
	}

	// Find end expression (next boundary or open-ended).
	for bi, b := range bounds {
		if b.FieldIdx >= fieldIdx {
			endExpr = fmt.Sprintf("m.%s", offName(bi))
			break
		}
	}
	return
}

// emitBoundaryGetter emits a getter for a field past the fixed prefix in a
// message with VarSectionBoundaries. Uses stored boundary offsets (m.off1, etc.)
// to locate fields after variable content.
func (g *Generator) emitBoundaryGetter(ti *TypeInfo, fieldIdx int, f Field) {
	m := ti.Decl.Message
	name := g.goName(ti)
	fieldGoName := pascalCase(f.Name)
	dataExpr := "m.data"

	baseExpr, relOff, endExpr := g.boundaryInfo(ti, fieldIdx)
	offExprStr := baseExpr
	if relOff > 0 {
		offExprStr = fmt.Sprintf("%s+%d", baseExpr, relOff)
	}

	// --- Union field ---
	if f.DiscRef != "" {
		unionGoType := pascalCase(f.TypeName)
		discIdx := g.a.findFieldIdx(m, f.DiscRef)
		discPrim := g.fieldPrimInfo(m.Fields[discIdx])
		discExpr := g.boundaryFieldSlice(ti, discIdx, discPrim.Size)
		g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, unionGoType)
		g.p("\treturn %s{b: %s, disc: %s}\n}\n",
			unionGoType, sliceStr(dataExpr, offExprStr, endExpr),
			g.readPrimFromSlice(discPrim, discExpr))
		return
	}

	// --- Variable-length array ---
	if isVarLenArray(f) {
		countIdx := g.a.findFieldIdx(m, f.ArrayLen)
		countPrim := g.fieldPrimInfo(m.Fields[countIdx])
		countSlice := g.boundaryFieldSlice(ti, countIdx, countPrim.Size)
		elemSize := g.a.typeSize(f.TypeName)

		if elemSize >= 0 {
			if isByteSliceType(f.TypeName) {
				g.emitBoundaryByteSliceGetter(ti, f, offExprStr, countPrim, countSlice)
			} else if p, isPrim := primitives[f.TypeName]; isPrim {
				g.p("\nfunc (m %s) %s(i int) %s {\n", name, fieldGoName, goType(p))
				g.p("\toff := %s + i*%d\n", offExprStr, elemSize)
				g.p("\treturn %s\n}\n", g.readPrimFromSlice(p, fmt.Sprintf("%s[off:off+%d]", dataExpr, elemSize)))
			} else if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
				sizeConst := sizeConstName(f.TypeName)
				g.p("\nfunc (m %s) %s(i int) %s {\n", name, fieldGoName, g.goName(eti))
				g.p("\toff := %s + i*%s\n", offExprStr, sizeConst)
				g.p("\treturn %s{m: (*[%s]byte)(%s[off:off+%s])}\n}\n",
					g.goName(eti), sizeConst, dataExpr, sizeConst)
			}
		} else {
			iterInfo := g.a.Iterators[f.TypeName]
			if iterInfo != nil {
				iterType := pascalCase(iterInfo.ElemMsgName) + "Iter"
				g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, iterType)
				g.p("\t%s := %s\n", f.ArrayLen,
					g.readCountExpr(countPrim, countSlice))
				g.p("\treturn %s{b: %s[%s:], count: %s}\n}\n",
					iterType, dataExpr, offExprStr, f.ArrayLen)
			}
		}

		// Count getter.
		g.p("\nfunc (m %s) %s() %s {\n", name, pascalCase(f.ArrayLen), goType(countPrim))
		g.p("\treturn %s\n}\n",
			g.readPrimFromSlice(countPrim, countSlice))
		return
	}
	if f.ArrayLen != "" {
		return
	}

	// --- Scalar (primitive or enum) ---
	if p, ok := g.fieldPrimInfoOk(f); ok {
		g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, goType(p))
		g.p("\to := %s\n", offExprStr)
		g.p("\treturn %s\n}\n", g.readPrimFromSlice(p, fmt.Sprintf("%s[o:o+%d]", dataExpr, p.Size)))
		return
	}

	// --- Embedded fixed struct ---
	if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
		embSize := sizeConstName(f.TypeName)
		g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, g.goName(eti))
		g.p("\to := %s\n", offExprStr)
		g.p("\treturn %s{m: (*[%s]byte)(%s[o:o+%s])}\n}\n",
			g.goName(eti), embSize, dataExpr, embSize)
		return
	}

	// --- Embedded variable-size message ---
	if eti, ok := g.a.Types[f.TypeName]; ok {
		sl := sliceStr(dataExpr, offExprStr, endExpr)
		g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, g.goName(eti))
		if g.a.isStructReader(f.TypeName) {
			g.p("\tv, _ := Read%s(%s)\n\treturn v\n}\n", g.goName(eti), sl)
		} else {
			g.p("\treturn %s(%s)\n}\n", g.goName(eti), sl)
		}
		return
	}
}

// boundaryFieldSlice returns a Go slice expression for reading a field at
// fieldIdx within a message with stored boundary offsets.
func (g *Generator) boundaryFieldSlice(ti *TypeInfo, fieldIdx, size int) string {
	dataExpr := "m.data"

	// If in fixed prefix, use static offset.
	if ti.FieldOffsets[fieldIdx] < ti.FixedPrefixSize {
		off := ti.FieldOffsets[fieldIdx]
		return fmt.Sprintf("%s[%d:%d]", dataExpr, off, off+size)
	}

	base, relOff, _ := g.boundaryInfo(ti, fieldIdx)
	if relOff > 0 {
		return fmt.Sprintf("%s[%s+%d:%s+%d]", dataExpr, base, relOff, base, relOff+size)
	}
	return fmt.Sprintf("%s[%s:%s+%d]", dataExpr, base, base, size)
}

// emitBoundaryByteSliceGetter emits a byte-slice getter for a field past
// the fixed prefix, using boundary offsets.
func (g *Generator) emitBoundaryByteSliceGetter(ti *TypeInfo, f Field, offExprStr string, countPrim PrimInfo, countSlice string) {
	name := g.goName(ti)
	fieldGoName := pascalCase(f.Name)
	dataExpr := "m.data"

	countExpr := g.readCountExpr(countPrim, countSlice)

	switch f.TypeName {
	case "npchar":
		g.p("\nfunc (m %s) %s() string {\n", name, fieldGoName)
		g.p("\tn := %s\n", countExpr)
		g.p("\to := %s\n", offExprStr)
		g.p("\treturn strings.TrimRight(string(%s[o:o+n]), \"\\x00\")\n}\n", dataExpr)
	case "spchar":
		g.p("\nfunc (m %s) %s() string {\n", name, fieldGoName)
		g.p("\tn := %s\n", countExpr)
		g.p("\to := %s\n", offExprStr)
		g.p("\treturn strings.TrimRight(string(%s[o:o+n]), \" \")\n}\n", dataExpr)
	default:
		g.p("\nfunc (m %s) %s() []byte {\n", name, fieldGoName)
		g.p("\tn := %s\n", countExpr)
		g.p("\to := %s\n", offExprStr)
		g.p("\treturn %s[o:o+n]\n}\n", dataExpr)
	}
}

func (g *Generator) emitVarArrayGetter(ti *TypeInfo, f Field, dataExpr string) {
	m := ti.Decl.Message
	name := g.goName(ti)
	fieldGoName := pascalCase(f.Name)

	elemSize := g.a.typeSize(f.TypeName)

	countIdx := g.a.findFieldIdx(m, f.ArrayLen)
	countField := m.Fields[countIdx]
	countOff := ti.FieldOffsets[countIdx]
	countPrim := g.fieldPrimInfo(countField)

	if elemSize >= 0 {
		// Fixed-size elements.
		if isByteSliceType(f.TypeName) {
			// Byte-slice type: return []byte slice.
			g.emitByteSliceGetter(ti, f, countOff, countPrim)
			return
		}
		// Index-based access.
		if _, isPrim := primitives[f.TypeName]; isPrim {
			// Primitive array.
			p := primitives[f.TypeName]
			g.p("\nfunc (m %s) %s(i int) %s {\n", name, fieldGoName, goType(p))
			g.p("\toff := %d + i*%d\n", ti.TotalFixedSize, elemSize)
			g.p("\treturn %s\n}\n", g.readPrimFromSlice(p, fmt.Sprintf("m[off : off+%d]", elemSize)))
		} else if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
			// Struct array.
			sizeConst := sizeConstName(f.TypeName)
			g.p("\nfunc (m %s) %s(i int) %s {\n", name, fieldGoName, g.goName(eti))
			g.p("\toff := %d + i*%s\n", ti.TotalFixedSize, sizeConst)
			g.p("\treturn %s{m: (*[%s]byte)(m[off : off+%s])}\n}\n",
				g.goName(eti), sizeConst, sizeConst)
		}
		return
	}

	// Variable-size elements: iterator.
	iterInfo := g.a.Iterators[f.TypeName]
	if iterInfo == nil {
		return
	}
	iterType := pascalCase(iterInfo.ElemMsgName) + "Iter"

	// Determine the data start offset for this array.
	var dataStart int
	if ti.IsMultiArray {
		// For multi-array, iterators use offB boundaries.
		g.emitMultiArrayIterGetter(ti, f, countField, countOff, countPrim, iterType, dataExpr)
		return
	}

	dataStart = ti.TotalFixedSize

	g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, iterType)
	g.p("\t%s := %s\n", countField.Name,
		g.readCountExpr(countPrim, fmt.Sprintf("m[%d:%d]", countOff, countOff+countPrim.Size)))
	g.p("\treturn %s{\n\t\tb:     []byte(m[%d:]),\n\t\tcount: %s,\n\t}\n}\n",
		iterType, dataStart, countField.Name)
}

// emitByteSliceGetter emits a getter that returns []byte (or string for
// npchar/spchar) for a variable-length array of byte-sized elements.
func (g *Generator) emitByteSliceGetter(ti *TypeInfo, f Field, countOff int, countPrim PrimInfo) {
	name := g.goName(ti)
	fieldGoName := pascalCase(f.Name)
	dataStart := ti.TotalFixedSize

	countExpr := g.readCountExpr(countPrim, fmt.Sprintf("m[%d:%d]", countOff, countOff+countPrim.Size))

	switch f.TypeName {
	case "npchar":
		g.p("\nfunc (m %s) %s() string {\n", name, fieldGoName)
		g.p("\tn := %s\n", countExpr)
		g.p("\treturn strings.TrimRight(string(m[%d:%d+n]), \"\\x00\")\n}\n", dataStart, dataStart)
	case "spchar":
		g.p("\nfunc (m %s) %s() string {\n", name, fieldGoName)
		g.p("\tn := %s\n", countExpr)
		g.p("\treturn strings.TrimRight(string(m[%d:%d+n]), \" \")\n}\n", dataStart, dataStart)
	default:
		// u8, char → []byte
		g.p("\nfunc (m %s) %s() []byte {\n", name, fieldGoName)
		g.p("\tn := %s\n", countExpr)
		g.p("\treturn m[%d : %d+n]\n}\n", dataStart, dataStart)
	}
}

func (g *Generator) emitMultiArrayIterGetter(ti *TypeInfo, f, countField Field, countOff int, countPrim PrimInfo, iterType, dataExpr string) {
	name := g.goName(ti)
	fieldGoName := pascalCase(f.Name)

	// Determine which array this is (first or second).
	arrayIdx := 0
	for i, va := range ti.VarArrays {
		if va.ArrayField.Name == f.Name {
			arrayIdx = i
			break
		}
	}

	g.p("\nfunc (m %s) %s() %s {\n", name, fieldGoName, iterType)
	if arrayIdx > 0 && ti.MultiArrayKind == 2 {
		// For interleaved multi-array, the second count is at m.offB (not a fixed offset).
		g.p("\t%s := %s\n", countField.Name,
			g.readCountExpr(countPrim, fmt.Sprintf("%s[m.offB:m.offB+%d]", dataExpr, countPrim.Size)))
	} else {
		g.p("\t%s := %s\n", countField.Name,
			g.readCountExpr(countPrim, fmt.Sprintf("%s[%d:%d]", dataExpr, countOff, countOff+countPrim.Size)))
	}

	if arrayIdx == 0 {
		if ti.MultiArrayKind == 1 {
			g.p("\treturn %s{b: %s[%d:m.offB], count: %s}\n}\n",
				iterType, dataExpr, ti.TotalFixedSize, countField.Name)
		} else {
			g.p("\treturn %s{b: %s[%d:m.offB], count: %s}\n}\n",
				iterType, dataExpr, countOff+countPrim.Size, countField.Name)
		}
	} else {
		if ti.MultiArrayKind == 1 {
			g.p("\treturn %s{b: %s[m.offB:], count: %s}\n}\n",
				iterType, dataExpr, countField.Name)
		} else {
			g.p("\treturn %s{b: %s[m.offB+%d:], count: %s}\n}\n",
				iterType, dataExpr, countPrim.Size, countField.Name)
		}
	}
}

func (g *Generator) emitIteratorsForMessage(ti *TypeInfo) {
	m := ti.Decl.Message
	for _, f := range m.Fields {
		if f.Kind != FieldNormal || !isVarLenArray(f) {
			continue
		}
		elemSize := g.a.typeSize(f.TypeName)
		if elemSize >= 0 {
			continue
		}
		// This field needs an iterator.
		iterInfo := g.a.Iterators[f.TypeName]
		if iterInfo == nil || iterInfo.Created {
			continue
		}
		iterInfo.Created = true

		elemGoName := pascalCase(iterInfo.ElemMsgName)
		iterType := elemGoName + "Iter"
		accessorName := pascalCase(singularize(iterInfo.FieldName))

		elemTi := g.a.Types[f.TypeName]
		if elemTi != nil && g.isByteSliceMessage(elemTi) {
			g.emitBlobIterator(elemTi, iterType, elemGoName, accessorName)
		} else {
			g.emitGenericIterator(iterType, elemGoName, accessorName)
		}
	}
}

// emitBlobIterator generates an optimized iterator for blob-pattern elements.
// The struct stores curLen instead of cur, and the accessor returns the
// iterator's b directly (no sub-slice). Next() inlines the size computation.
func (g *Generator) emitBlobIterator(elemTi *TypeInfo, iterType, elemGoName, accessorName string) {
	va := elemTi.VarArrays[0]
	lenPrim := g.fieldPrimInfo(va.CountField)
	headerSize := elemTi.TotalFixedSize

	alignSize := elemTi.AlignSize

	g.p("\n// %s iterates over variable-size %s entries.\n", iterType, elemGoName)
	g.p("type %s struct {\n", iterType)
	g.p("\tb      []byte\n")
	g.p("\tcount  int\n")
	g.p("\ti      int\n")
	g.p("\tcurLen int\n")
	g.p("}\n")

	g.p("\nfunc (it *%s) Next() bool {\n", iterType)
	// Advance past previous element with explicit bounds guard.
	g.p("\tb := it.b\n")
	g.p("\tif it.curLen > 0 {\n")
	g.p("\t\tif len(b) < it.curLen {\n\t\t\treturn false\n\t\t}\n")
	g.p("\t\tb = b[it.curLen:]\n")
	g.p("\t\tit.curLen = 0\n")
	g.p("\t}\n")
	g.p("\tif it.i >= it.count {\n\t\tit.b = b\n\t\treturn false\n\t}\n")
	// Inline size computation for blob element.
	g.p("\tif len(b) < %d {\n\t\treturn false\n\t}\n", headerSize)
	g.p("\tn := %s\n", g.readCountExpr(lenPrim, fmt.Sprintf("b[%d:%d]", va.CountOff, va.CountOff+lenPrim.Size)))
	if alignSize > 0 {
		g.p("\tpadded := (n + %d) &^ %d\n", alignSize-1, alignSize-1)
		g.p("\ttotal := %d + padded\n", headerSize)
	} else {
		g.p("\ttotal := %d + n\n", headerSize)
	}
	g.p("\tif len(b) < total {\n\t\treturn false\n\t}\n")
	g.p("\tit.b = b\n")
	g.p("\tit.curLen = total\n")
	g.p("\tit.i++\n")
	g.p("\treturn true\n}\n")

	// Accessor returns the element type aliasing it.b directly (no sub-slice).
	g.p("\nfunc (it *%s) %s() %s {\n", iterType, accessorName, elemGoName)
	g.p("\treturn %s(it.b)\n", elemGoName)
	g.p("}\n")
}

// emitGenericIterator generates a standard iterator using readElem + cur.
func (g *Generator) emitGenericIterator(iterType, elemGoName, accessorName string) {
	readFn := "read" + elemGoName

	g.p("\n// %s iterates over variable-size %s entries.\n", iterType, elemGoName)
	g.p("type %s struct {\n", iterType)
	g.p("\tb     []byte\n")
	g.p("\tcount int\n")
	g.p("\ti     int\n")
	g.p("\tcur   %s\n", elemGoName)
	g.p("}\n")

	g.p("\nfunc (it *%s) Next() bool {\n", iterType)
	g.p("\tif it.i >= it.count {\n\t\treturn false\n\t}\n")
	g.p("\tvar ok bool\n")
	g.p("\tit.cur, ok = %s(&it.b)\n", readFn)
	g.p("\tif !ok {\n\t\treturn false\n\t}\n")
	g.p("\tit.i++\n")
	g.p("\treturn true\n}\n")

	g.p("\nfunc (it *%s) %s() %s {\n", iterType, accessorName, elemGoName)
	g.p("\treturn it.cur\n}\n")
}

// isByteSliceMessage returns true if a message type consists of a single
// byte-slice variable-length array plus optional alignment and fixed fields.
// Used to select the optimized blob iterator for these elements.
func (g *Generator) isByteSliceMessage(ti *TypeInfo) bool {
	if ti.Decl.Kind != DeclMessage {
		return false
	}
	if len(ti.VarArrays) != 1 || ti.HasVarSizeArrays || ti.HasExtent {
		return false
	}
	if !isByteSliceType(ti.VarArrays[0].ArrayField.TypeName) {
		return false
	}
	m := ti.Decl.Message
	for j := ti.VarArrays[0].ArrayIdx + 1; j < len(m.Fields); j++ {
		if m.Fields[j].Kind != FieldAlign {
			return false
		}
	}
	return true
}
