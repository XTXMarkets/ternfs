// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"strings"
)

func (g *Generator) emitMessageWriter(ti *TypeInfo) {
	m := ti.Decl.Message
	name := g.goName(ti)
	writerName := name + "Writer"

	g.p("\n// %s writes a %s", writerName, m.Name)
	g.emitWriterComment(m, ti)
	g.p("\n")

	if ti.HasExtent && ti.IsExtentStruct {
		g.emitExtentWriter(ti)
		return
	}

	if ti.IsMultiArray {
		g.emitMultiArrayWriter(ti)
		return
	}

	// Standard Pattern A writer.
	g.emitPatternAWriterFull(ti)
}

func (g *Generator) emitWriterComment(m *MessageDecl, ti *TypeInfo) {
	// Simple comment like ": count(4) + leu32 samples[count]."
	var parts []string
	for _, f := range m.Fields {
		switch f.Kind {
		case FieldPad:
			parts = append(parts, fmt.Sprintf("pad(%d)", f.Size))
		case FieldAlign:
			parts = append(parts, fmt.Sprintf("align(%d)", f.Size))
		case FieldExtent:
			parts = append(parts, fmt.Sprintf("extent(%s)", f.Ref))
		case FieldNormal:
			if f.ArrayLen != "" {
				parts = append(parts, fmt.Sprintf("%s %s[%s]", f.TypeName, f.Name, f.ArrayLen))
			} else if f.DiscRef != "" {
				parts = append(parts, fmt.Sprintf("%s(%s)", f.Name, f.DiscRef))
			} else {
				parts = append(parts, f.Name)
			}
		}
	}
	if len(parts) > 0 {
		g.p(":\n//\n//\t%s", strings.Join(parts, " + "))
	}
}

func (g *Generator) emitExtentWriter(ti *TypeInfo) {
	m := ti.Decl.Message
	name := g.goName(ti)
	writerName := name + "Writer"
	totalSize := ti.ExtentTotalSize

	g.p("type %s struct {\n\tbuf []byte\n\toff int\n}\n", writerName)

	// Start: pre-allocate all known fields.
	g.p("\nfunc Start%s(buf []byte) %s {\n", name, writerName)
	g.p("\toff := len(buf)\n")
	g.p("\tbuf = append(buf, make([]byte, %d)...)", totalSize)
	g.emitExtentStartComment(m, ti)
	g.p("\n\treturn %s{buf: buf, off: off}\n}\n", writerName)

	// Set methods for each field (value-chaining).
	for i, f := range m.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		fieldGoName := pascalCase(f.Name)
		off := ti.FieldOffsets[i]

		// Skip the extent length field — it's patched by Finish.
		if f.Name == ti.ExtentFieldName {
			continue
		}

		if p, ok := g.fieldPrimInfoOk(f); ok {
			g.p("\nfunc (w %s) Set%s(v %s) %s {\n", writerName, fieldGoName, goType(p), writerName)
			g.p("\t%s\n", g.writePrim(p,
				fmt.Sprintf("(*[%d]byte)(w.buf[w.off:])[%d:%d]", totalSize, off, off+p.Size), "v"))
			g.p("\treturn w\n}\n")
		}
	}

	// AppendExtra for forward compatibility.
	g.p("\nfunc (w %s) AppendExtra(extra []byte) %s {\n", writerName, writerName)
	g.p("\tw.buf = append(w.buf, extra...)\n")
	g.p("\treturn w\n}\n")

	// Finish: patch extent field.
	g.p("\nfunc (w %s) Finish() []byte {\n", writerName)
	g.p("\tbodyLen := len(w.buf) - w.off - %d\n", ti.TotalFixedSize)
	extPrim := g.extentPrimInfo(ti)
	g.p("\t%s\n", g.writePrim(extPrim,
		fmt.Sprintf("(*[%d]byte)(w.buf[w.off:])[%d:%d]", totalSize, ti.ExtentFieldOff, ti.ExtentFieldOff+extPrim.Size),
		fmt.Sprintf("%s(bodyLen)", goType(extPrim))))
	g.p("\treturn w.buf\n}\n")
}

func (g *Generator) emitExtentStartComment(m *MessageDecl, ti *TypeInfo) {
	// Comment showing what the pre-allocated bytes are for.
	var parts []string
	for _, f := range m.Fields {
		if f.Kind == FieldNormal {
			sz := g.a.typeSize(f.TypeName)
			if sz > 0 {
				parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
			}
		}
	}
	if len(parts) > 0 {
		g.p(" // %s", strings.Join(parts, " + "))
	}
}

type writerCountInfo struct {
	goName   string // camelCase Go name
	goType   string // Go type (uint32, uint16, etc.)
	deferred bool   // true if count field comes after variable content
	inline   bool   // true if SetData handles the count inline (no struct field needed)
}

// emitPatternAWriterFull generates a Pattern A (pointer-receiver) writer
// for a standard message (not blob, not extent, not multi-array).
func (g *Generator) emitPatternAWriterFull(ti *TypeInfo) {
	name := g.goName(ti)
	writerName := name + "Writer"

	isUnionWrapper := ti.IsUnionWrapper

	// Build count field info from pre-computed VarArrays.
	skipHeader := g.allHeaderIsByteSliceCounts(ti) && !isUnionWrapper
	var counts []writerCountInfo
	for _, va := range ti.VarArrays {
		p := g.fieldPrimInfo(va.CountField)
		isInline := skipHeader && isByteSliceType(va.ArrayField.TypeName) &&
			ti.FieldOffsets[va.CountIdx] < ti.FixedPrefixSize
		counts = append(counts, writerCountInfo{
			goName:   camelCase(va.CountField.Name),
			goType:   goType(p),
			deferred: ti.FieldOffsets[va.CountIdx] >= ti.FixedPrefixSize,
			inline:   isInline,
		})
	}

	// Needs Resume if any variable-size section requires child writers.
	needsResume := ti.WriterPhases > 0

	needsPhase := ti.WriterPhases > 1
	useHeaderPtr := g.writerUsesHeaderPtr(ti)
	headerSize := ti.FixedPrefixSize
	if skipHeader {
		headerSize = 0
	}

	// Value receivers are safe when all variable content is byte-slice arrays
	// and no Resume is needed. Counts may be stored in struct fields (non-inline),
	// but since SetData returns the modified writer by value, callers get the
	// updated copy.
	useValueRecv := !needsResume && allVarArraysByteSlice(ti)

	// Emit type declaration.
	maxNL := maxFieldNameLen(counts)
	// Check if any deferred counts need offset tracking fields.
	for _, c := range counts {
		offName := c.goName + "Off"
		if len(offName) > maxNL {
			maxNL = len(offName)
		}
	}
	if needsPhase && len("phase") > maxNL {
		maxNL = len("phase")
	}
	if useHeaderPtr && len("header") > maxNL {
		maxNL = len("header")
	}
	g.p("type %s struct {\n", writerName)
	g.p("\tbuf%s []byte\n", strings.Repeat(" ", maxNL-3+1))
	if useHeaderPtr {
		g.p("\theader%s *[%d]byte\n", strings.Repeat(" ", maxNL-6+1), headerSize)
	} else {
		g.p("\toff%s int\n", strings.Repeat(" ", maxNL-3+1))
	}
	for _, c := range counts {
		if c.inline {
			continue // count is written inline by SetData
		}
		g.p("\t%s%s %s\n", c.goName, strings.Repeat(" ", maxNL-len(c.goName)+1), c.goType)
		if c.deferred {
			offName := c.goName + "Off"
			g.p("\t%s%s int\n", offName, strings.Repeat(" ", maxNL-len(offName)+1))
		}
	}
	if needsPhase {
		g.p("\t%s%s uint8\n", "phase", strings.Repeat(" ", maxNL-len("phase")+1))
	}
	g.p("}\n")

	// Emit Start function.
	g.p("\nfunc Start%s(buf []byte) %s {\n", name, writerName)
	if isUnionWrapper {
		// Defer discriminant allocation.
		g.p("\treturn %s{buf: buf, off: len(buf)}\n", writerName)
	} else if useHeaderPtr {
		g.p("\tbuf = append(buf, make([]byte, %d)...)", headerSize)
		g.emitStartComment(ti)
		g.p("\n")
		g.p("\treturn %s{buf: buf, header: (*[%d]byte)(buf[len(buf)-%d:])}\n",
			writerName, headerSize, headerSize)
	} else {
		g.p("\toff := len(buf)\n")
		if headerSize > 0 {
			if g.headerUsesAppendHelper(ti) {
				g.emitHeaderAlloc(ti)
			} else {
				g.p("\tbuf = append(buf, make([]byte, %d)...)", headerSize)
				g.emitStartComment(ti)
				g.p("\n")
			}
		}
		g.p("\treturn %s{buf: buf, off: off}\n", writerName)
	}
	g.p("}\n")

	// Emit setter methods for fixed header fields.
	g.emitWriterHeaderSetters(ti, writerName, useValueRecv)

	// Emit array append methods.
	g.emitWriterArrayMethods(ti, writerName, counts)

	// Emit union set/append methods.
	g.emitWriterUnionMethods(ti, writerName)

	// Emit embedded struct/message accessors.
	g.emitWriterEmbeddedAccessors(ti, writerName)

	// Resume method.
	if needsResume {
		g.p("\nfunc (w *%s) Resume(buf []byte) {\n\tw.buf = buf\n}\n", writerName)
	}

	// Finish method.
	g.emitWriterFinish(ti, writerName, counts, needsResume, useValueRecv)
}

// writerUsesHeaderPtr returns true if a message writer should use a stored
// *[N]byte header pointer instead of off int. This applies to Pattern A writers
// that have a pre-allocated header but no deferred count patching in Finish.
func (g *Generator) writerUsesHeaderPtr(ti *TypeInfo) bool {
	if ti == nil || ti.Decl.Kind != DeclMessage {
		return false
	}
	if ti.IsExtentStruct || ti.IsMultiArray {
		return false
	}
	if ti.IsUnionWrapper || ti.FixedPrefixSize == 0 {
		return false
	}
	return len(ti.VarArrays) == 0
}

// emitChildWriterReturn emits a return statement that constructs a child writer,
// using header pointer or off as appropriate for the child type. offExpr is a Go
// expression giving the child's start offset within buf.
func (g *Generator) emitChildWriterReturn(childTi *TypeInfo, childWriter, bufExpr, offExpr string) {
	if g.writerUsesHeaderPtr(childTi) {
		hs := childTi.FixedPrefixSize
		g.p("\treturn %s{buf: %s, header: (*[%d]byte)(%s[%s:])}\n",
			childWriter, bufExpr, hs, bufExpr, offExpr)
	} else {
		g.p("\treturn %s{buf: %s, off: %s}\n",
			childWriter, bufExpr, offExpr)
	}
}

func maxFieldNameLen(counts []writerCountInfo) int {
	maxLen := 3 // "buf" and "off" are 3 and 2
	for _, c := range counts {
		if len(c.goName) > maxLen {
			maxLen = len(c.goName)
		}
	}
	return maxLen
}

// allVarArraysByteSlice returns true if the message's only variable content
// is byte-slice arrays (u8/char/npchar/spchar). Such messages don't need
// Resume (no child writers), so all setters can use value receivers.
func allVarArraysByteSlice(ti *TypeInfo) bool {
	if len(ti.VarArrays) == 0 || ti.HasVarSizeArrays || ti.HasExtent {
		return false
	}
	for _, va := range ti.VarArrays {
		if !isByteSliceType(va.ArrayField.TypeName) {
			return false
		}
	}
	return true
}

// allHeaderIsByteSliceCounts returns true if every field in the fixed prefix
// is a count field for a byte-slice array whose data immediately follows the
// count. When true, the header allocation can be skipped — each SetData will
// do a combined count+data alloc.
//
// This currently only matches single-array messages where count is at offset 0
// and data follows immediately (e.g., data_blob). Multi-array messages have
// their counts grouped in a header separate from data, so combined allocation
// isn't possible.
func (g *Generator) allHeaderIsByteSliceCounts(ti *TypeInfo) bool {
	if len(ti.VarArrays) != 1 || ti.HasVarSizeArrays || ti.HasExtent {
		return false
	}
	va := ti.VarArrays[0]
	if !isByteSliceType(va.ArrayField.TypeName) {
		return false
	}
	// The count must be the only thing in the fixed prefix.
	countSize := g.a.typeSize(va.CountField.TypeName)
	return va.CountOff == 0 && ti.FixedPrefixSize == countSize
}

func (g *Generator) headerUsesAppendHelper(ti *TypeInfo) bool {
	// Use appendLeUXX when the header is just a single count placeholder (4 bytes).
	if ti.FixedPrefixSize != 4 || len(ti.VarArrays) != 1 {
		return false
	}
	va := ti.VarArrays[0]
	return va.CountOff == 0 && g.fieldPrimInfo(va.CountField).Size == 4
}

func (g *Generator) emitHeaderAlloc(ti *TypeInfo) {
	f := ti.VarArrays[0].CountField
	g.p("\tbuf = %s(buf, 0) // %s placeholder\n", g.appendHelper(g.fieldPrimInfo(f)), f.Name)
}

func (g *Generator) emitStartComment(ti *TypeInfo) {
	m := ti.Decl.Message
	var parts []string
	off := 0
	for _, f := range m.Fields {
		if f.Kind == FieldPad {
			parts = append(parts, fmt.Sprintf("pad(%d)", f.Size))
			off += f.Size
		} else if f.Kind == FieldAlign {
			pad := (f.Size - (off % f.Size)) % f.Size
			off += pad
		} else if f.Kind == FieldNormal {
			if isVarLenArray(f) {
				break
			}
			if f.DiscRef != "" {
				break
			}
			sz := g.a.typeSize(f.TypeName)
			if sz < 0 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
			off += sz
		}
	}
	if len(parts) > 0 {
		g.p(" // %s", strings.Join(parts, " + "))
	}
}

func (g *Generator) emitWriterHeaderSetters(ti *TypeInfo, writerName string, useValueRecv bool) {
	m := ti.Decl.Message
	headerSize := ti.FixedPrefixSize
	needsPhase := ti.WriterPhases > 1
	useHeaderPtr := g.writerUsesHeaderPtr(ti)

	for i, f := range m.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		// Skip count fields, array fields, union fields, and embedded var-size.
		if f.ArrayLen != "" || f.DiscRef != "" {
			continue
		}
		// Skip if it's a count field (will be set by Finish).
		if ti.CountFields[f.Name] {
			continue
		}

		// Skip if embedded variable-size message.
		if g.a.isVarSizeType(f.TypeName) {
			continue
		}

		off := ti.FieldOffsets[i]
		fieldGoName := pascalCase(f.Name)

		if ti.IsUnionWrapper {
			// In union wrappers, the discriminant is deferred.
			continue
		}

		// Fields beyond the pre-allocated header: check if they're
		// discriminants handled by union methods, otherwise emit append-style setters.
		if off >= headerSize {
			if ti.DiscFields[f.Name] {
				continue // handled by union Set/Append methods
			}
			fieldPhase := ti.FieldPhase[i]
			// Emit append-style setter.
			if p, ok := g.fieldPrimInfoOk(f); ok {
				g.p("\nfunc (w *%s) Set%s(v %s) {\n", writerName, fieldGoName, goType(p))
				g.emitPhaseCheck(needsPhase, fieldPhase)
				g.p("\tw.buf = %s(w.buf, v)\n", g.appendHelper(p))
				g.p("}\n")
			}
			continue
		}

		if p, ok := g.fieldPrimInfoOk(f); ok {
			if useValueRecv {
				g.p("\nfunc (w %s) Set%s(v %s) %s {\n", writerName, fieldGoName, goType(p), writerName)
				g.p("\t%s\n", g.writerSetPrimVal(p, headerSize, off, "v", useHeaderPtr))
				g.p("\treturn w\n}\n")
			} else {
				g.p("\nfunc (w *%s) Set%s(v %s) {\n", writerName, fieldGoName, goType(p))
				g.p("\t%s\n", g.writerSetPrimVal(p, headerSize, off, "v", useHeaderPtr))
				g.p("}\n")
			}
			continue
		}

		// Embedded fixed struct in header — return mutable view.
		if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
			sizeConst := sizeConstName(f.TypeName)
			g.p("\nfunc (w *%s) %s() %s {\n", writerName, fieldGoName, g.goName(eti))
			if useHeaderPtr {
				g.p("\treturn %s{m: (*[%s]byte)(w.header[%d:])}\n}\n",
					g.goName(eti), sizeConst, off)
			} else {
				g.p("\treturn %s{m: (*[%s]byte)(w.buf[%s:])}\n}\n",
					g.goName(eti), sizeConst, offExpr("w.off", off))
			}
		}
	}
}

func (g *Generator) emitWriterArrayMethods(ti *TypeInfo, writerName string, counts []writerCountInfo) {
	needsPhase := ti.WriterPhases > 1

	// counts is parallel to ti.VarArrays.
	for ci, va := range ti.VarArrays {
		f := va.ArrayField
		c := counts[ci]
		fieldGoName := pascalCase(f.Name)
		elemSize := g.a.typeSize(f.TypeName)
		fieldPhase := ti.FieldPhase[va.ArrayIdx]

		if elemSize >= 0 {
			// Fixed-size elements.
			if isByteSliceType(f.TypeName) {
				// Byte-slice type: SetXxx(data []byte)
				useValueRecv := allVarArraysByteSlice(ti) && ti.WriterPhases == 0
				g.emitByteSliceSetter(ti, va.ArrayIdx, f, writerName, c, needsPhase, fieldPhase, useValueRecv)
			} else if p, isPrim := primitives[f.TypeName]; isPrim {
				// Primitive array: AppendXxx(v Type)
				helper := g.appendHelper(p)
				g.p("\nfunc (w *%s) Append%s(v %s) {\n", writerName, fieldGoName, goType(p))
				g.emitPhaseCheck(needsPhase, fieldPhase)
				g.emitDeferredCountAlloc(c.deferred, c.goName, va.CountField)
				g.p("\tw.buf = %s(w.buf, v)\n", helper)
				g.p("\tw.%s++\n", c.goName)
				g.p("}\n")
			} else if eti, ok := g.a.Types[f.TypeName]; ok && eti.Decl.Kind == DeclStruct {
				// Struct array: AppendXxx() StructType
				sizeConst := sizeConstName(f.TypeName)
				g.p("\nfunc (w *%s) Append%s() %s {\n", writerName, fieldGoName, g.goName(eti))
				g.emitPhaseCheck(needsPhase, fieldPhase)
				g.emitDeferredCountAlloc(c.deferred, c.goName, va.CountField)
				g.p("\tw.buf = append(w.buf, make([]byte, %s)...)\n", sizeConst)
				g.p("\tw.%s++\n", c.goName)
				g.p("\treturn %s{m: (*[%s]byte)(w.buf[len(w.buf)-%s:])}\n}\n",
					g.goName(eti), sizeConst, sizeConst)
			}
		} else {
			// Variable-size elements.
			if eti, ok := g.a.Types[f.TypeName]; ok {
				// Check if element is a union wrapper — generate convenience methods.
				if eti.IsUnionWrapper {
					g.emitConvenienceAppendMethods(ti, f, eti, writerName, c.goName, c.deferred, needsPhase, fieldPhase, va.CountField)
				} else {
					// Regular variable-size message: AppendXxx() → child writer.
					childWriter := g.goName(eti) + "Writer"
					g.p("\nfunc (w *%s) Append%s() %s {\n", writerName, fieldGoName, childWriter)
					g.p("\t_ = w.buf[:1]\n")
					g.emitPhaseCheck(needsPhase, fieldPhase)
					g.emitDeferredCountAlloc(c.deferred, c.goName, va.CountField)
					g.p("\tw.%s++\n", c.goName)
					g.p("\tchild := Start%s(w.buf)\n", g.goName(eti))
					g.p("\tw.buf = nil\n")
					g.p("\treturn child\n}\n")
				}
			}
		}
	}
}

// emitDeferredCountAlloc emits lazy allocation of a count field placeholder
// for counts that come after variable content in the message.
func (g *Generator) emitDeferredCountAlloc(deferred bool, countGoName string, countField Field) {
	if !deferred {
		return
	}
	p := g.fieldPrimInfo(countField)
	offName := countGoName + "Off"
	g.p("\tif w.%s == 0 {\n", offName)
	g.p("\t\tw.%s = len(w.buf)\n", offName)
	g.p("\t\tw.buf = %s(w.buf, 0)\n", g.appendHelper(p))
	g.p("\t}\n")
}

// emitByteSliceSetter generates a SetXxx(data []byte) method for byte-slice
// array fields (u8, char, npchar, spchar). Handles alignment padding if a
// FieldAlign directive follows the array field.
//
// When c.inline is true, the count field is NOT pre-allocated in the header.
// SetData does a single combined allocation for count + padded data.
func (g *Generator) emitByteSliceSetter(ti *TypeInfo, fieldIdx int, f Field, writerName string, c writerCountInfo, needsPhase bool, fieldPhase int, useValueRecv bool) {
	m := ti.Decl.Message
	fieldGoName := pascalCase(f.Name)

	// Find the VarArrayInfo for this field.
	var va VarArrayInfo
	for _, v := range ti.VarArrays {
		if v.ArrayIdx == fieldIdx {
			va = v
			break
		}
	}

	// Find alignment directive after this field, if any.
	alignSize := 0
	for j := fieldIdx + 1; j < len(m.Fields); j++ {
		if m.Fields[j].Kind == FieldAlign {
			alignSize = m.Fields[j].Size
			break
		}
		if m.Fields[j].Kind == FieldNormal {
			break
		}
	}

	countPrim := g.fieldPrimInfo(va.CountField)

	if c.inline {
		// Combined allocation: count + padded data in one append.
		g.p("\nfunc (w %s) Set%s(data []byte) %s {\n", writerName, fieldGoName, writerName)
		g.emitPhaseCheck(needsPhase, fieldPhase)
		g.p("\tn := len(data)\n")
		if alignSize > 0 {
			g.p("\tpadded := (n + %d) &^ %d\n", alignSize-1, alignSize-1)
			g.p("\ttotal := %d + padded\n", countPrim.Size)
		} else {
			g.p("\ttotal := %d + n\n", countPrim.Size)
		}
		g.p("\tw.buf = append(w.buf, make([]byte, total)...)\n")
		g.p("\t%s\n", g.writePrim(countPrim,
			fmt.Sprintf("w.buf[w.off:w.off+%d]", countPrim.Size),
			fmt.Sprintf("%s(n)", goType(countPrim))))
		g.p("\tcopy(w.buf[w.off+%d:], data)\n", countPrim.Size)
		g.p("\treturn w\n}\n")
		return
	}

	if useValueRecv {
		g.p("\nfunc (w %s) Set%s(data []byte) %s {\n", writerName, fieldGoName, writerName)
	} else {
		g.p("\nfunc (w *%s) Set%s(data []byte) {\n", writerName, fieldGoName)
	}
	g.emitPhaseCheck(needsPhase, fieldPhase)
	g.emitDeferredCountAlloc(c.deferred, c.goName, va.CountField)
	g.p("\tn := len(data)\n")

	if alignSize > 0 {
		g.p("\tpadded := (n + %d) &^ %d\n", alignSize-1, alignSize-1)
		g.p("\tw.buf = append(w.buf, make([]byte, padded)...)\n")
	} else {
		g.p("\tw.buf = append(w.buf, make([]byte, n)...)\n")
	}

	g.p("\tcopy(w.buf[len(w.buf)-")
	if alignSize > 0 {
		g.p("padded:")
	} else {
		g.p("n:")
	}
	g.p("], data)\n")

	// Set the count field value; Finish will write it to the buffer.
	g.p("\tw.%s = %s(n)\n", c.goName, goType(countPrim))
	if useValueRecv {
		g.p("\treturn w\n")
	}
	g.p("}\n")
}

func (g *Generator) emitConvenienceAppendMethods(parentTi *TypeInfo, arrayField Field, elemTi *TypeInfo, writerName, countGoName string, countDeferred bool, needsPhase bool, fieldPhase int, countField Field) {
	elemMsg := elemTi.Decl.Message
	fieldGoName := pascalCase(arrayField.Name)

	// Find the discriminant field and union field in the element message.
	var unionField Field
	for _, f := range elemMsg.Fields {
		if f.Kind == FieldNormal && f.DiscRef != "" {
			unionField = f
		}
	}

	// Get the union declaration.
	unionTi := g.a.Types[unionField.TypeName]
	if unionTi == nil || unionTi.Decl.Kind != DeclUnion {
		return
	}
	unionDecl := unionTi.Decl.Union
	discEnum := g.a.Types[unionDecl.DiscType]
	discPrim := enumPrimInfo(discEnum.Decl.Enum)
	discSize := discPrim.Size

	for _, arm := range unionDecl.Arms {
		suffix := armSuffix(stripEnumPrefix(arm.Label, discEnum.Decl.Enum.Constants))
		if arm.Label == "default" {
			suffix = "Default"
		}

		methodName := fmt.Sprintf("Append%s_%s", fieldGoName, suffix)

		if arm.Payload == "void" {
			discVal := arm.Label
			if arm.Label == "default" {
				discVal = camelCase(g.goName(discEnum))
				g.p("\nfunc (w *%s) %s(%s %s) {\n", writerName, methodName, discVal, goType(discPrim))
			} else {
				g.p("\nfunc (w *%s) %s() {\n", writerName, methodName)
			}
			g.p("\t_ = w.buf[:1]\n")
			g.emitPhaseCheck(needsPhase, fieldPhase)
			g.emitDeferredCountAlloc(countDeferred, countGoName, countField)
			g.p("\tw.buf = %s(w.buf, %s)\n", g.appendHelper(discPrim), discVal)
			g.p("\tw.%s++\n", countGoName)
			g.p("}\n")
			continue
		}

		payloadTi := g.a.Types[arm.Payload]
		if payloadTi == nil {
			continue
		}

		if payloadTi.IsFixedSize {
			// Fixed-size struct payload: coalesce disc + body into single append.
			sizeConst := sizeConstName(arm.Payload)

			g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, g.goName(payloadTi))
			g.p("\t_ = w.buf[:1]\n")
			g.emitPhaseCheck(needsPhase, fieldPhase)
			g.emitDeferredCountAlloc(countDeferred, countGoName, countField)
			g.p("\tw.buf = append(w.buf, make([]byte, %d+%s)...)\n", discSize, sizeConst)
			g.p("\tp := (*[%d + %s]byte)(w.buf[len(w.buf)-%d-%s:])\n",
				discSize, sizeConst, discSize, sizeConst)
			g.p("\t%s\n", g.writePrim(discPrim, fmt.Sprintf("p[:%d]", discSize), arm.Label))
			g.p("\tw.%s++\n", countGoName)
			g.p("\treturn %s{m: (*[%s]byte)(p[%d:])}\n}\n",
				g.goName(payloadTi), sizeConst, discSize)
		} else {
			// Variable-size payload: coalesce disc + known header.
			childWriter := g.goName(payloadTi) + "Writer"
			knownSize := g.childWriterKnownSize(payloadTi)
			if knownSize > 0 {
				g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, childWriter)
				g.p("\t_ = w.buf[:1]\n")
				g.emitPhaseCheck(needsPhase, fieldPhase)
				g.emitDeferredCountAlloc(countDeferred, countGoName, countField)
				g.p("\tw.buf = append(w.buf, make([]byte, %d+%d)...)", discSize, knownSize)
				g.emitCoalesceComment(discSize, knownSize, payloadTi)
				g.p("\n")
				g.p("\toff := len(w.buf) - %d - %d\n", discSize, knownSize)
				g.p("\t%s\n", g.writePrim(discPrim,
					fmt.Sprintf("(*[%d + %d]byte)(w.buf[off:])[:%d]", discSize, knownSize, discSize), arm.Label))
				g.p("\tw.%s++\n", countGoName)
				g.p("\tbuf := w.buf\n")
				g.p("\tw.buf = nil\n")
				g.emitChildWriterReturn(payloadTi, childWriter, "buf", fmt.Sprintf("off + %d", discSize))
				g.p("}\n")
			} else {
				// Can't coalesce — just append disc, then start child.
				g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, childWriter)
				g.p("\t_ = w.buf[:1]\n")
				g.emitPhaseCheck(needsPhase, fieldPhase)
				g.emitDeferredCountAlloc(countDeferred, countGoName, countField)
				helper := g.appendHelper(discPrim)
				g.p("\tw.buf = %s(w.buf, %s)\n", helper, arm.Label)
				g.p("\tw.%s++\n", countGoName)
				g.p("\tchild := Start%s(w.buf)\n", g.goName(payloadTi))
				g.p("\tw.buf = nil\n")
				g.p("\treturn child\n}\n")
			}
		}
	}
}

func (g *Generator) emitCoalesceComment(discSize, knownSize int, payloadTi *TypeInfo) {
	// Add comment explaining what the coalesced bytes are.
	if payloadTi.HasExtent && payloadTi.IsExtentStruct {
		m := payloadTi.Decl.Message
		var parts []string
		parts = append(parts, fmt.Sprintf("discriminant(%d)", discSize))
		for _, f := range m.Fields {
			if f.Kind == FieldNormal {
				sz := g.a.typeSize(f.TypeName)
				if sz > 0 {
					parts = append(parts, fmt.Sprintf("%s(%d)", f.Name, sz))
				}
			}
		}
		g.p(" // %s", strings.Join(parts, " + "))
	}
}

func (g *Generator) emitWriterUnionMethods(ti *TypeInfo, writerName string) {
	m := ti.Decl.Message
	headerSize := ti.FixedPrefixSize
	needsPhase := ti.WriterPhases > 1
	useHeaderPtr := g.writerUsesHeaderPtr(ti)

	for fi, f := range m.Fields {
		if f.Kind != FieldNormal || f.DiscRef == "" {
			continue
		}

		unionTi := g.a.Types[f.TypeName]
		if unionTi == nil || unionTi.Decl.Kind != DeclUnion {
			continue
		}
		unionDecl := unionTi.Decl.Union
		discEnum := g.a.Types[unionDecl.DiscType]
		discPrim := enumPrimInfo(discEnum.Decl.Enum)
		discSize := discPrim.Size
		fieldPhase := ti.FieldPhase[fi]

		// Check if the discriminant field is in the pre-allocated header.
		// Union wrappers defer header allocation, so the disc is never pre-allocated.
		discInHeader := false
		discOff := 0
		if !ti.IsUnionWrapper {
			for i2, f2 := range m.Fields {
				if f2.Kind == FieldNormal && f2.Name == f.DiscRef {
					discOff = ti.FieldOffsets[i2]
					discInHeader = discOff+discSize <= headerSize
					break
				}
			}
		}

		fieldGoName := pascalCase(f.Name)

		for _, arm := range unionDecl.Arms {
			suffix := armSuffix(stripEnumPrefix(arm.Label, discEnum.Decl.Enum.Constants))
			if arm.Label == "default" {
				suffix = "Default"
			}

			methodName := fmt.Sprintf("Set%s_%s", fieldGoName, suffix)

			if arm.Payload == "void" {
				discVal := arm.Label
				if arm.Label == "default" {
					discVal = camelCase(unionDecl.DiscType)
					g.p("\nfunc (w *%s) %s(%s %s) {\n",
						writerName, methodName, discVal, goType(discPrim))
				} else {
					g.p("\nfunc (w *%s) %s() {\n", writerName, methodName)
				}
				if discInHeader {
					g.p("\t%s\n", g.writerSetPrimVal(discPrim, headerSize, discOff, discVal, useHeaderPtr))
				} else {
					g.emitPhaseCheck(needsPhase, fieldPhase)
					g.p("\tw.buf = %s(w.buf, %s)\n", g.appendHelper(discPrim), discVal)
				}
				g.p("}\n")
				continue
			}

			payloadTi := g.a.Types[arm.Payload]
			if payloadTi == nil {
				continue
			}

			if discInHeader {
				// Discriminant is in the pre-allocated header — write it there,
				// only append payload bytes.
				g.emitUnionArmDiscInHeader(writerName, methodName, payloadTi,
					discPrim, headerSize, discOff, arm, needsPhase, fieldPhase, useHeaderPtr)
			} else if payloadTi.IsFixedSize {
				// Fixed-size struct: coalesce disc + body.
				sizeConst := sizeConstName(arm.Payload)

				g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, g.goName(payloadTi))
				g.emitPhaseCheck(needsPhase, fieldPhase)
				g.p("\tw.buf = append(w.buf, make([]byte, %d+%s)...)\n", discSize, sizeConst)
				g.p("\tp := (*[%d + %s]byte)(w.buf[len(w.buf)-%d-%s:])\n",
					discSize, sizeConst, discSize, sizeConst)
				g.p("\t%s\n", g.writePrim(discPrim,
					fmt.Sprintf("p[:%d]", discSize), arm.Label))
				g.p("\treturn %s{m: (*[%s]byte)(p[%d:])}\n}\n",
					g.goName(payloadTi), sizeConst, discSize)
			} else {
				// Variable-size payload.
				childWriter := g.goName(payloadTi) + "Writer"
				knownSize := g.childWriterKnownSize(payloadTi)
				if knownSize > 0 {
					g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, childWriter)
					g.emitPhaseCheck(needsPhase, fieldPhase)
					g.p("\tw.buf = append(w.buf, make([]byte, %d+%d)...)\n", discSize, knownSize)
					g.p("\toff := len(w.buf) - %d - %d\n", discSize, knownSize)
					g.p("\t%s\n", g.writePrim(discPrim,
						fmt.Sprintf("(*[%d + %d]byte)(w.buf[off:])[:%d]", discSize, knownSize, discSize), arm.Label))
					g.p("\tbuf := w.buf\n")
					g.p("\tw.buf = nil\n")
					g.emitChildWriterReturn(payloadTi, childWriter, "buf", fmt.Sprintf("off + %d", discSize))
					g.p("}\n")
				} else {
					// Default: append disc, start child.
					helper := g.appendHelper(discPrim)
					g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, childWriter)
					g.emitPhaseCheck(needsPhase, fieldPhase)
					g.p("\tw.buf = %s(w.buf, %s)\n", helper, arm.Label)
					g.p("\tchild := Start%s(w.buf)\n", g.goName(payloadTi))
					g.p("\tw.buf = nil\n")
					g.p("\treturn child\n}\n")
				}
			}
		}
	}
}

// emitUnionArmDiscInHeader generates a Set method for a union arm whose
// discriminant is in the pre-allocated message header.
func (g *Generator) emitUnionArmDiscInHeader(writerName, methodName string, payloadTi *TypeInfo, discPrim PrimInfo, headerSize, discOff int, arm UnionArm, needsPhase bool, fieldPhase int, useHeaderPtr bool) {
	if payloadTi.IsFixedSize {
		// Fixed-size struct payload: write disc into header, append only payload.
		sizeConst := sizeConstName(arm.Payload)
		g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, g.goName(payloadTi))
		g.emitPhaseCheck(needsPhase, fieldPhase)
		g.p("\t%s\n", g.writerSetPrimVal(discPrim, headerSize, discOff, arm.Label, useHeaderPtr))
		g.p("\tw.buf = append(w.buf, make([]byte, %s)...)\n", sizeConst)
		g.p("\treturn %s{m: (*[%s]byte)(w.buf[len(w.buf)-%s:])}\n}\n",
			g.goName(payloadTi), sizeConst, sizeConst)
	} else {
		// Variable-size payload: write disc into header, append only payload header.
		childWriter := g.goName(payloadTi) + "Writer"
		knownSize := g.childWriterKnownSize(payloadTi)
		g.p("\nfunc (w *%s) %s() %s {\n", writerName, methodName, childWriter)
		g.emitPhaseCheck(needsPhase, fieldPhase)
		g.p("\t%s\n", g.writerSetPrimVal(discPrim, headerSize, discOff, arm.Label, useHeaderPtr))
		if knownSize > 0 {
			g.p("\tw.buf = append(w.buf, make([]byte, %d)...)\n", knownSize)
			g.p("\tbuf := w.buf\n")
			g.p("\tw.buf = nil\n")
			g.emitChildWriterReturn(payloadTi, childWriter, "buf", fmt.Sprintf("len(buf) - %d", knownSize))
			g.p("}\n")
		} else {
			g.p("\tchild := Start%s(w.buf)\n", g.goName(payloadTi))
			g.p("\tw.buf = nil\n")
			if useHeaderPtr {
				g.p("\tw.header = nil\n")
			}
			g.p("\treturn child\n}\n")
		}
	}
}

func (g *Generator) emitWriterEmbeddedAccessors(ti *TypeInfo, writerName string) {
	m := ti.Decl.Message
	needsPhase := ti.WriterPhases > 1
	useHeaderPtr := g.writerUsesHeaderPtr(ti)

	for fi, f := range m.Fields {
		if f.Kind != FieldNormal {
			continue
		}
		if f.ArrayLen != "" || f.DiscRef != "" {
			continue
		}
		// Only emit for embedded structs and messages.
		eti, ok := g.a.Types[f.TypeName]
		if !ok {
			continue
		}

		fieldGoName := pascalCase(f.Name)

		if eti.Decl.Kind == DeclStruct {
			// Already handled in header setters as embedded struct getter.
			continue
		}

		if eti.Decl.Kind == DeclMessage && !eti.IsFixedSize {
			// Embedded variable-size message: StartFieldName() → child writer.
			childWriter := g.goName(eti) + "Writer"
			fieldPhase := ti.FieldPhase[fi]
			g.p("\nfunc (w *%s) Start%s() %s {\n", writerName, fieldGoName, childWriter)
			g.emitPhaseCheck(needsPhase, fieldPhase)
			g.p("\tchild := Start%s(w.buf)\n", g.goName(eti))
			g.p("\tw.buf = nil\n")
			if useHeaderPtr {
				g.p("\tw.header = nil\n")
			}
			g.p("\treturn child\n}\n")
		}
	}
}

func (g *Generator) emitWriterFinish(ti *TypeInfo, writerName string, counts []writerCountInfo, needsResume bool, useValueRecv bool) {
	headerSize := ti.FixedPrefixSize

	if useValueRecv {
		g.p("\nfunc (w %s) Finish() []byte {\n", writerName)
	} else {
		g.p("\nfunc (w *%s) Finish() []byte {\n", writerName)
	}
	if needsResume && len(counts) == 0 {
		g.p("\t_ = w.buf[:1]\n")
	}

	// Patch deferred count fields. counts is parallel to ti.VarArrays.
	for ci, c := range counts {
		if c.inline {
			continue // count was written inline by SetData
		}
		va := ti.VarArrays[ci]
		p := g.fieldPrimInfo(va.CountField)
		if c.deferred {
			offName := c.goName + "Off"
			g.p("\t%s\n", g.writePrim(p,
				fmt.Sprintf("w.buf[w.%s:w.%s+%d]", offName, offName, p.Size),
				fmt.Sprintf("w.%s", c.goName)))
		} else {
			off := va.CountOff
			if headerSize >= 8 {
				g.p("\t%s\n", g.writePrim(p,
					fmt.Sprintf("(*[%d]byte)(w.buf[w.off:])[%d:%d]", headerSize, off, off+p.Size),
					fmt.Sprintf("w.%s", c.goName)))
			} else {
				start := offExpr("w.off", off)
				end := offExpr("w.off", off+p.Size)
				g.p("\t%s\n", g.writePrim(p,
					fmt.Sprintf("w.buf[%s:%s]", start, end),
					fmt.Sprintf("w.%s", c.goName)))
			}
		}
	}

	g.p("\treturn w.buf\n}\n")
}

func (g *Generator) emitMultiArrayWriter(ti *TypeInfo) {
	name := g.goName(ti)
	writerName := name + "Writer"

	type arrWriter struct {
		VarArrayInfo
		goName string
		goType string
	}
	var arrays []arrWriter
	for _, va := range ti.VarArrays {
		p := g.fieldPrimInfo(va.CountField)
		arrays = append(arrays, arrWriter{va, camelCase(va.CountField.Name), goType(p)})
	}

	needsPhase := ti.WriterPhases > 1

	if ti.MultiArrayKind == 1 {
		// Pattern 1: both counts in header.
		headerSize := ti.TotalFixedSize
		g.p("type %s struct {\n", writerName)
		g.p("\tbuf       []byte\n")
		g.p("\toff       int\n")
		for _, arr := range arrays {
			g.p("\t%s    %s\n", arr.goName, arr.goType)
		}
		g.p("\titemsBOff int // absolute buffer offset where %s starts\n", arrays[1].ArrayField.Name)
		if needsPhase {
			g.p("\tphase     uint8\n")
		}
		g.p("}\n")

		g.p("\nfunc Start%s(buf []byte) %s {\n", name, writerName)
		g.p("\toff := len(buf)\n")
		g.p("\tbuf = append(buf, make([]byte, %d)...)", headerSize)
		g.emitStartComment(ti)
		g.p("\n\treturn %s{buf: buf, off: off}\n}\n", writerName)

		// Append methods for each array.
		for idx, arr := range arrays {
			elemTi := g.a.Types[arr.ArrayField.TypeName]
			childWriter := g.goName(elemTi) + "Writer"
			fieldGoName := pascalCase(arr.ArrayField.Name)
			arrPhase := ti.FieldPhase[arr.ArrayIdx]

			g.p("\nfunc (w *%s) Append%s() %s {\n", writerName, fieldGoName, childWriter)
			g.p("\t_ = w.buf[:1]\n")
			g.emitPhaseCheck(needsPhase, arrPhase)
			if idx == 1 {
				// Second array: record offB on first call.
				g.p("\tif w.itemsBOff == 0 {\n")
				g.p("\t\tw.itemsBOff = len(w.buf)\n")
				g.p("\t}\n")
			}
			g.p("\tw.%s++\n", arr.goName)
			g.p("\tchild := Start%s(w.buf)\n", g.goName(elemTi))
			g.p("\tw.buf = nil\n")
			g.p("\treturn child\n}\n")
		}

		g.p("\nfunc (w *%s) Resume(buf []byte) {\n\tw.buf = buf\n}\n", writerName)

		// Finish.
		g.p("\nfunc (w *%s) Finish() []byte {\n", writerName)
		for _, arr := range arrays {
			p := g.fieldPrimInfo(arr.CountField)
			g.p("\t%s\n", g.writePrim(p,
				fmt.Sprintf("(*[%d]byte)(w.buf[w.off:])[%d:%d]", headerSize, arr.CountOff, arr.CountOff+p.Size),
				fmt.Sprintf("w.%s", arr.goName)))
		}
		g.p("\treturn w.buf\n}\n")
	} else {
		// Pattern 2: interleaved counts.
		g.p("type %s struct {\n", writerName)
		g.p("\tbuf       []byte\n")
		g.p("\toff       int\n")
		for _, arr := range arrays {
			g.p("\t%s    %s\n", arr.goName, arr.goType)
		}
		g.p("\t%sOff int // absolute buffer offset of %s field\n",
			arrays[1].goName, arrays[1].CountField.Name)
		if needsPhase {
			g.p("\tphase     uint8\n")
		}
		g.p("}\n")

		g.p("\nfunc Start%s(buf []byte) %s {\n", name, writerName)
		g.p("\toff := len(buf)\n")
		countAPrim := g.fieldPrimInfo(arrays[0].CountField)
		helper := g.appendHelper(countAPrim)
		g.p("\tbuf = %s(buf, 0) // %s placeholder\n", helper, arrays[0].CountField.Name)
		g.p("\treturn %s{buf: buf, off: off}\n}\n", writerName)

		// Append methods.
		for idx, arr := range arrays {
			elemTi := g.a.Types[arr.ArrayField.TypeName]
			childWriter := g.goName(elemTi) + "Writer"
			fieldGoName := pascalCase(arr.ArrayField.Name)
			arrPhase := ti.FieldPhase[arr.ArrayIdx]

			g.p("\nfunc (w *%s) Append%s() %s {\n", writerName, fieldGoName, childWriter)
			g.p("\t_ = w.buf[:1]\n")
			g.emitPhaseCheck(needsPhase, arrPhase)
			if idx == 1 {
				// Second array: emit count_b placeholder on first call.
				g.p("\tif w.%sOff == 0 {\n", arrays[1].goName)
				g.p("\t\tw.%sOff = len(w.buf)\n", arrays[1].goName)
				countBPrim := g.fieldPrimInfo(arrays[1].CountField)
				helper2 := g.appendHelper(countBPrim)
				g.p("\t\tw.buf = %s(w.buf, 0) // %s placeholder\n", helper2, arrays[1].CountField.Name)
				g.p("\t}\n")
			}
			g.p("\tw.%s++\n", arr.goName)
			g.p("\tchild := Start%s(w.buf)\n", g.goName(elemTi))
			g.p("\tw.buf = nil\n")
			g.p("\treturn child\n}\n")
		}

		g.p("\nfunc (w *%s) Resume(buf []byte) {\n\tw.buf = buf\n}\n", writerName)

		// Finish.
		g.p("\nfunc (w *%s) Finish() []byte {\n", writerName)
		countAPrim = g.fieldPrimInfo(arrays[0].CountField)
		g.p("\t%s\n", g.writePrim(countAPrim,
			fmt.Sprintf("w.buf[w.off:w.off+%d]", countAPrim.Size),
			fmt.Sprintf("w.%s", arrays[0].goName)))
		g.p("\tif w.%sOff == 0 {\n", arrays[1].goName)
		g.p("\t\tw.%sOff = len(w.buf)\n", arrays[1].goName)
		countBPrim := g.fieldPrimInfo(arrays[1].CountField)
		helper2 := g.appendHelper(countBPrim)
		g.p("\t\tw.buf = %s(w.buf, 0) // %s = 0\n", helper2, arrays[1].CountField.Name)
		g.p("\t} else {\n")
		g.p("\t\t%s\n", g.writePrim(countBPrim,
			fmt.Sprintf("w.buf[w.%sOff:w.%sOff+%d]", arrays[1].goName, arrays[1].goName, countBPrim.Size),
			fmt.Sprintf("w.%s", arrays[1].goName)))
		g.p("\t}\n")
		g.p("\treturn w.buf\n}\n")
	}
}

// writerSetPrimVal generates a write statement for a given value into the header.
func (g *Generator) writerSetPrimVal(p PrimInfo, headerSize, off int, val string, useHeaderPtr bool) string {
	if useHeaderPtr {
		return g.writePrim(p, fmt.Sprintf("w.header[%d:%d]", off, off+p.Size), val)
	}
	if headerSize >= 8 {
		return g.writePrim(p,
			fmt.Sprintf("(*[%d]byte)(w.buf[w.off:])[%d:%d]", headerSize, off, off+p.Size), val)
	}
	start := offExpr("w.off", off)
	end := offExpr("w.off", off+p.Size)
	return g.writePrim(p, fmt.Sprintf("w.buf[%s:%s]", start, end), val)
}

// emitPhaseCheck generates a runtime check that the writer is in the correct
// phase. Advancing forward (phase < required) is allowed and sets the phase.
// Going backward (phase > required) panics.
func (g *Generator) emitPhaseCheck(needsPhase bool, requiredPhase int) {
	if !needsPhase {
		return
	}
	g.p("\tif w.phase > %d {\n\t\tpanic(\"writer fields called out of order\")\n\t}\n", requiredPhase)
	if requiredPhase > 0 {
		g.p("\tw.phase = %d\n", requiredPhase)
	}
}

// childWriterKnownSize returns the number of bytes to pre-allocate for a
// child message writer's header. Returns 0 if the child handles its own
// allocation (blob writers, union wrappers).
func (g *Generator) childWriterKnownSize(ti *TypeInfo) int {
	if ti.IsUnionWrapper {
		return 0
	}
	if ti.HasExtent && ti.IsExtentStruct {
		return ti.ExtentTotalSize
	}
	// When the header is skipped (all counts handled inline by SetData),
	// Start allocates 0 bytes.
	if g.allHeaderIsByteSliceCounts(ti) {
		return 0
	}
	return ti.FixedPrefixSize
}

func (g *Generator) appendHelper(p PrimInfo) string {
	order := "LittleEndian"
	if p.IsBE {
		order = "BigEndian"
	}
	return fmt.Sprintf("binary.%s.AppendUint%d", order, p.Size*8)
}
