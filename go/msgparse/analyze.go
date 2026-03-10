// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PrimInfo describes a primitive type's wire properties.
type PrimInfo struct {
	Size     int // 1, 2, 4, 8
	Signed   bool
	IsBE     bool   // big-endian
	IsChar   bool   // char, npchar, spchar
	CharKind string // "", "np", "sp"
	IsFloat  bool   // IEEE 754 float/double
}

var primitives = map[string]PrimInfo{
	"u8":       {Size: 1},
	"i8":       {Size: 1, Signed: true},
	"leu16":    {Size: 2},
	"lei16":    {Size: 2, Signed: true},
	"beu16":    {Size: 2, IsBE: true},
	"bei16":    {Size: 2, Signed: true, IsBE: true},
	"leu32":    {Size: 4},
	"lei32":    {Size: 4, Signed: true},
	"beu32":    {Size: 4, IsBE: true},
	"bei32":    {Size: 4, Signed: true, IsBE: true},
	"leu64":    {Size: 8},
	"lei64":    {Size: 8, Signed: true},
	"beu64":    {Size: 8, IsBE: true},
	"bei64":    {Size: 8, Signed: true, IsBE: true},
	"lefloat":  {Size: 4, IsFloat: true},
	"befloat":  {Size: 4, IsBE: true, IsFloat: true},
	"ledouble": {Size: 8, IsFloat: true},
	"bedouble": {Size: 8, IsBE: true, IsFloat: true},
	"char":     {Size: 1, IsChar: true},
	"npchar":   {Size: 1, IsChar: true, CharKind: "np"},
	"spchar":   {Size: 1, IsChar: true, CharKind: "sp"},
}

// TypeInfo holds computed information about a declared type.
type TypeInfo struct {
	Decl Decl
	Size int // total byte size (for fixed-size types; -1 for variable)

	// For structs: computed field offsets.
	FieldOffsets []int // parallel to Decl.Struct.Fields or Decl.Message.Fields

	// Flags computed during analysis.
	NeedsCursorRead bool // needs lowercase readXxx function
	IsFixedSize     bool

	// For messages:
	TotalFixedSize       int // total bytes of fixed-size scalar fields (may span across variable content)
	HasExtent            bool
	ExtentFieldOff       int    // offset of the extent length field
	ExtentFieldName      string // name of the extent length field
	KnownBodySize        int    // size of known fields after extent
	IsExtentStruct       bool   // reader type is struct{m *[N]byte}
	ExtentTotalSize      int    // headerSize + knownBodySize (for struct reader)
	IsMultiArray         bool
	MultiArrayKind       int                  // 1 = both counts in header, 2 = interleaved
	IsUnionWrapper       bool                 // disc + union, nothing else (deferred alloc)
	AlignSize            int                  // align directive size, 0 if none
	HasVarSizeArrays     bool                 // has variable-length arrays with var-size elements
	VarArrays            []VarArrayInfo       // variable-length arrays with their count info
	VarSectionBoundaries []VarSectionBoundary // stored offsets after variable content
	ReadPattern          string               // "compute", "wrap", "lazy", "extent", "multiarray"

	FixedPrefixSize int // contiguous fixed bytes at start (stops at first variable content)

	// Precomputed field role sets (for messages):
	CountFields map[string]bool // field names referenced as array length (e.g., "count" in data[count])
	DiscFields  map[string]bool // field names referenced as union discriminants

	// Writer phase tracking (for messages with multiple variable-size sections):
	WriterPhases int         // total phases (0 or 1 = no checking needed)
	FieldPhase   map[int]int // field index → phase number
}

// VarArrayInfo describes a variable-length array field and its count field.
type VarArrayInfo struct {
	ArrayIdx   int // index of array field in message fields
	ArrayField Field
	CountIdx   int // index of count field in message fields
	CountField Field
	CountOff   int // byte offset of count field
}

// VarSectionBoundary marks the end of a variable-size section in a message.
// The stored offset points to the byte position immediately after the
// section's data, which is the start of the next field.
type VarSectionBoundary struct {
	FieldIdx int // index of the var-size field that ends at this boundary
}

// IterInfo tracks iterator types for variable-size array elements.
type IterInfo struct {
	ElemMsgName string // .msg type name of element
	FieldName   string // first field name using this type (for accessor naming)
	Created     bool
}

// Analysis holds the result of analyzing a .msg file.
type Analysis struct {
	File      *MsgFile
	Types     map[string]*TypeInfo // keyed by .msg name
	Order     []string             // topologically sorted type names
	Iterators map[string]*IterInfo // keyed by element .msg type name
}

func Analyze(file *MsgFile) *Analysis {
	a := &Analysis{
		File:      file,
		Types:     make(map[string]*TypeInfo),
		Iterators: make(map[string]*IterInfo),
	}

	// Phase 1: Register all types (skip const groups — they are anonymous).
	for _, d := range file.Decls {
		if d.Kind == DeclConstGroup {
			continue
		}
		name := d.Name()
		a.Types[name] = &TypeInfo{
			Decl: d,
		}
	}

	// Phase 2: Compute sizes and field offsets for fixed-size types.
	for _, d := range file.Decls {
		if d.Kind == DeclStruct {
			a.computeStructSize(d.Struct)
		}
		if d.Kind == DeclEnum {
			ti := a.Types[d.Name()]
			ti.IsFixedSize = true
			ti.Size = a.enumSize(d.Enum)
		}
	}

	// Phase 3: Analyze messages.
	for _, d := range file.Decls {
		if d.Kind == DeclMessage {
			a.analyzeMessage(d.Message)
		}
	}

	// Phase 4: Determine which types need cursor-advancing reads.
	a.computeCursorReads()

	// Phase 5: Compute output order.
	a.computeOrder()

	// Phase 6: Determine iterators.
	a.computeIterators()

	return a
}

func (a *Analysis) computeStructSize(s *StructDecl) {
	ti := a.Types[s.Name]
	if ti.Size > 0 {
		return // already computed
	}
	offset := 0
	offsets := make([]int, len(s.Fields))
	for i, f := range s.Fields {
		switch f.Kind {
		case FieldPad:
			offsets[i] = offset
			offset += f.Size
		case FieldAlign:
			pad := (f.Size - (offset % f.Size)) % f.Size
			offsets[i] = offset
			offset += pad
		case FieldNormal:
			offsets[i] = offset
			sz := a.fieldSize(f)
			offset += sz
		}
	}
	ti.Size = offset
	ti.IsFixedSize = true
	ti.FieldOffsets = offsets
}

func (a *Analysis) fieldSize(f Field) int {
	baseSize := a.typeSize(f.TypeName)
	if f.ArrayLen != "" {
		n, err := strconv.Atoi(f.ArrayLen)
		if err != nil {
			// Variable-length array — not fixed size
			return -1
		}
		return baseSize * n
	}
	return baseSize
}

func (a *Analysis) typeSize(name string) int {
	if p, ok := primitives[name]; ok {
		return p.Size
	}
	ti, ok := a.Types[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown type: %s\n", name)
		os.Exit(1)
	}
	if ti.Decl.Kind == DeclStruct {
		if ti.Size == 0 {
			a.computeStructSize(ti.Decl.Struct)
		}
		return ti.Size
	}
	if ti.Decl.Kind == DeclEnum {
		return a.enumSize(ti.Decl.Enum)
	}
	return -1 // variable-size
}

func (a *Analysis) enumSize(e *EnumDecl) int {
	return enumPrimInfo(e).Size
}

// enumPrimInfo returns the PrimInfo for an enum's underlying wire type.
// String enums (char[N], npchar[N], spchar[N]) are lowered to LE integers.
func enumPrimInfo(e *EnumDecl) PrimInfo {
	u := e.Underlying
	if strings.Contains(u, "[") {
		n := extractBracketNum(u)
		switch n {
		case 2:
			return primitives["leu16"]
		case 4:
			return primitives["leu32"]
		case 8:
			return primitives["leu64"]
		}
	}
	if p, ok := primitives[u]; ok {
		return p
	}
	fmt.Fprintf(os.Stderr, "unknown enum underlying type: %s\n", u)
	os.Exit(1)
	return PrimInfo{}
}

func (a *Analysis) analyzeMessage(m *MessageDecl) {
	ti := a.Types[m.Name]
	ti.IsFixedSize = false
	ti.Size = -1

	// Precompute field role sets.
	ti.CountFields = map[string]bool{}
	ti.DiscFields = map[string]bool{}
	for _, f := range m.Fields {
		if f.Kind == FieldNormal {
			if isVarLenArray(f) {
				ti.CountFields[f.ArrayLen] = true
			}
			if f.DiscRef != "" {
				ti.DiscFields[f.DiscRef] = true
			}
		}
	}

	// Compute header size and field offsets.
	offset := 0
	offsets := make([]int, len(m.Fields))
	hasOtherVarContent := false // var content besides fixed-element arrays
	var extentIdx = -1
	pastExtent := false

	for i, f := range m.Fields {
		offsets[i] = offset
		switch f.Kind {
		case FieldPad:
			if !pastExtent {
				offset += f.Size
			}
		case FieldAlign:
			// Alignment after variable data — tracked via AlignSize, not
			// treated as "other variable content" since compute-read can handle it.
		case FieldExtent:
			ti.HasExtent = true
			extentIdx = i
			pastExtent = true
			ti.TotalFixedSize = offset // save header size before extent body
			// Find the extent field's offset
			ti.ExtentFieldName = f.Ref
			for j, f2 := range m.Fields {
				if f2.Kind == FieldNormal && f2.Name == f.Ref {
					ti.ExtentFieldOff = offsets[j]
					break
				}
			}
		case FieldNormal:
			if pastExtent {
				// Fields after extent are inside the extent body.
				// Track their offsets for getter generation, but don't affect header size.
				offsets[i] = offset
				sz := a.typeSize(f.TypeName)
				if sz > 0 {
					offset += sz
				}
			} else if isVarLenArray(f) {
				countIdx := a.findFieldIdx(m, f.ArrayLen)
				ti.VarArrays = append(ti.VarArrays, VarArrayInfo{
					ArrayIdx:   i,
					ArrayField: f,
					CountIdx:   countIdx,
					CountField: m.Fields[countIdx],
					CountOff:   offsets[countIdx],
				})
				elemSize := a.typeSize(f.TypeName)
				if elemSize < 0 {
					ti.HasVarSizeArrays = true
				}
			} else if f.ArrayLen != "" {
				// Fixed-size array
				offset += a.fieldSize(f)
			} else if f.DiscRef != "" {
				// Union field — variable-size
				hasOtherVarContent = true
			} else {
				sz := a.typeSize(f.TypeName)
				if sz < 0 {
					// Embedded variable-size message
					hasOtherVarContent = true
				} else {
					offset += sz
				}
			}
		}
	}

	if !pastExtent {
		ti.TotalFixedSize = offset // bytes of fixed header before variable content
	}
	ti.FieldOffsets = offsets

	// Compute real contiguous header size (stops at first variable content).
	rh := 0
	for _, f := range m.Fields {
		if f.Kind == FieldPad {
			rh += f.Size
			continue
		}
		if f.Kind == FieldAlign {
			pad := (f.Size - (rh % f.Size)) % f.Size
			rh += pad
			continue
		}
		if f.Kind != FieldNormal {
			continue
		}
		if a.isVarContentField(f) {
			break
		}
		rh += a.fieldSize(f)
	}
	ti.FixedPrefixSize = rh

	// Detect multi-array messages.
	if len(ti.VarArrays) >= 2 {
		ti.IsMultiArray = true
		// Determine pattern: check if all count fields come before all array fields.
		allCountsBefore := true
		firstArrayIdx := ti.VarArrays[0].ArrayIdx
		for _, va := range ti.VarArrays {
			if va.CountIdx > firstArrayIdx {
				allCountsBefore = false
				break
			}
		}
		if allCountsBefore {
			ti.MultiArrayKind = 1 // both counts in header
		} else {
			ti.MultiArrayKind = 2 // interleaved
		}
	}

	// Compute boundaries after variable-content fields. A boundary is needed
	// after each variable-content field that has subsequent normal fields,
	// so getters can locate those subsequent fields via stored offsets.
	if !ti.IsMultiArray {
		var varFieldIdxs []int
		for i, f := range m.Fields {
			if a.isVarContentField(f) {
				varFieldIdxs = append(varFieldIdxs, i)
			}
		}
		for _, vfi := range varFieldIdxs {
			hasSubsequent := false
			for k := vfi + 1; k < len(m.Fields); k++ {
				if m.Fields[k].Kind == FieldNormal {
					hasSubsequent = true
					break
				}
			}
			if hasSubsequent {
				ti.VarSectionBoundaries = append(ti.VarSectionBoundaries, VarSectionBoundary{
					FieldIdx: vfi,
				})
			}
		}
	}

	// Compute align size (first align directive found).
	for _, f := range m.Fields {
		if f.Kind == FieldAlign {
			ti.AlignSize = f.Size
			break
		}
	}

	// Detect extent-based struct reader type: extent with all known fixed-offset fields.
	if ti.HasExtent && !ti.IsMultiArray {
		// Compute known body size (fields after extent)
		bodySize := 0
		allFixed := true
		if extentIdx >= 0 {
			for j := extentIdx + 1; j < len(m.Fields); j++ {
				f := m.Fields[j]
				if f.Kind == FieldNormal {
					sz := a.typeSize(f.TypeName)
					if sz < 0 || f.ArrayLen != "" || f.DiscRef != "" {
						allFixed = false
						break
					}
					bodySize += sz
				}
			}
		}
		if allFixed && bodySize > 0 {
			ti.IsExtentStruct = true
			ti.KnownBodySize = bodySize
			ti.ExtentTotalSize = ti.TotalFixedSize + bodySize
		}
	}

	// Determine read pattern.
	if ti.IsMultiArray {
		ti.ReadPattern = "multiarray"
	} else if len(ti.VarSectionBoundaries) > 0 {
		// Multiple var-size sections: must use cursor read to compute offsets.
		ti.ReadPattern = "wrap"
	} else if ti.IsExtentStruct {
		ti.ReadPattern = "extent"
	} else if len(ti.VarArrays) == 1 && !ti.HasVarSizeArrays && !hasOtherVarContent {
		// Single array of fixed-size elements, no other variable content
		// → validate length and return slice.
		ti.ReadPattern = "compute"
	} else if ti.HasVarSizeArrays || len(ti.VarArrays) >= 1 {
		// Has var-size array elements → lazy read for public Read
		ti.ReadPattern = "lazy"
	} else {
		// Has var-size children (embedded message, union) but no var-size arrays
		ti.ReadPattern = "wrap"
	}

	// Detect union wrapper: exactly a discriminant scalar + a union field.
	normalCount := 0
	hasUnion := false
	for _, f := range m.Fields {
		if f.Kind == FieldNormal {
			normalCount++
			if f.DiscRef != "" {
				hasUnion = true
			}
		}
	}
	ti.IsUnionWrapper = normalCount == 2 && hasUnion

	// Compute writer phases. Walk fields left to right; each variable-size
	// section (embedded var message, var-size array, or union) that requires
	// Resume increments the phase. Fields in the fixed prefix (phase 0) are
	// always safe. Fields after each variable section get the next phase number.
	a.computeWriterPhases(ti)
}

// computeWriterPhases assigns phase numbers to message fields for writer
// ordering validation. Phase 0 covers the fixed prefix. Each variable-size
// section that requires Resume increments the phase. Fields (including
// deferred fixed fields) after a variable section get the new phase number.
//
// A "variable-size section" is one of:
//   - An embedded variable-size message
//   - A variable-length array with variable-size elements
//   - A union field (which may have variable-size arms)
//
// Fixed-size arrays (e.g., leu32 samples[count]) don't require Resume and
// don't create phase boundaries.
func (a *Analysis) computeWriterPhases(ti *TypeInfo) {
	m := ti.Decl.Message
	ti.FieldPhase = make(map[int]int)
	phase := 0
	inPrefix := true

	for i, f := range m.Fields {
		if f.Kind != FieldNormal {
			ti.FieldPhase[i] = phase
			continue
		}

		// Fields in the fixed prefix are always phase 0.
		if inPrefix && ti.FieldOffsets[i] < ti.FixedPrefixSize {
			ti.FieldPhase[i] = phase
			continue
		}

		// Check if this field is a variable-size section.
		isVarSection := false
		if isVarLenArray(f) {
			if a.typeSize(f.TypeName) < 0 {
				// Variable-size elements → needs Resume per element.
				isVarSection = true
			}
		} else if f.DiscRef != "" {
			isVarSection = a.unionHasVarSizeArm(f.TypeName)
		} else if a.isVarSizeType(f.TypeName) {
			// Embedded variable-size message.
			isVarSection = true
		}

		inPrefix = false
		ti.FieldPhase[i] = phase
		if isVarSection {
			phase++ // next field gets the new phase
		}
	}

	ti.WriterPhases = phase
}

func (a *Analysis) findFieldIdx(m *MessageDecl, name string) int {
	for i, f := range m.Fields {
		if f.Kind == FieldNormal && f.Name == name {
			return i
		}
	}
	return -1
}

func (a *Analysis) computeCursorReads() {
	// Mark types that need cursor-advancing reads:
	// 1. All unions
	// 2. Structs used as union arm payloads
	// 3. Messages containing variable-size content OR used in variable contexts

	// First pass: mark unions and their arm dependencies.
	for _, d := range a.File.Decls {
		if d.Kind == DeclUnion {
			a.Types[d.Name()].NeedsCursorRead = true
			for _, arm := range d.Union.Arms {
				if arm.Payload != "void" {
					if ti, ok := a.Types[arm.Payload]; ok {
						ti.NeedsCursorRead = true
					}
				}
			}
		}
	}

	// Second pass: mark messages with variable-size content, and mark
	// variable-size types referenced from messages (they need cursor reads
	// to be embedded or used as array elements).
	for _, d := range a.File.Decls {
		if d.Kind != DeclMessage {
			continue
		}
		ti := a.Types[d.Name()]
		for _, f := range d.Message.Fields {
			if f.Kind != FieldNormal {
				continue
			}
			if f.DiscRef != "" {
				ti.NeedsCursorRead = true
			} else if isVarLenArray(f) {
				if a.typeSize(f.TypeName) < 0 {
					ti.NeedsCursorRead = true
				}
			} else if a.isVarSizeType(f.TypeName) {
				ti.NeedsCursorRead = true
			}
			// Mark any referenced var-size type as needing cursor read.
			if eti, ok := a.Types[f.TypeName]; ok && !eti.IsFixedSize {
				eti.NeedsCursorRead = true
			}
		}
	}

	// Multi-array and boundary messages always need cursor read.
	for _, ti := range a.Types {
		if ti.IsMultiArray || len(ti.VarSectionBoundaries) > 0 {
			ti.NeedsCursorRead = true
		}
	}
}

func (a *Analysis) computeOrder() {
	// Emit in declaration order: enums and const groups first, then everything else.
	// Const groups are anonymous — use a synthetic key "#const_N" where N is the decl index.
	constIdx := 0
	for _, d := range a.File.Decls {
		if d.Kind == DeclEnum {
			a.Order = append(a.Order, d.Name())
		} else if d.Kind == DeclConstGroup {
			key := fmt.Sprintf("#const_%d", constIdx)
			constIdx++
			a.Types[key] = &TypeInfo{
				Decl: d,
			}
			a.Order = append(a.Order, key)
		}
	}
	for _, d := range a.File.Decls {
		if d.Kind != DeclEnum && d.Kind != DeclConstGroup {
			a.Order = append(a.Order, d.Name())
		}
	}
}

func (a *Analysis) computeIterators() {
	// For each variable-size element type used in a variable-length array,
	// create an iterator. The accessor name comes from the first field name
	// that uses this element type.
	for _, name := range a.Order {
		ti := a.Types[name]
		if ti.Decl.Kind != DeclMessage {
			continue
		}
		for _, f := range ti.Decl.Message.Fields {
			if f.Kind != FieldNormal || !isVarLenArray(f) {
				continue
			}
			elemSize := a.typeSize(f.TypeName)
			if elemSize >= 0 {
				continue // fixed-size elements — no iterator needed
			}
			// Variable-size element → needs iterator.
			if _, exists := a.Iterators[f.TypeName]; !exists {
				a.Iterators[f.TypeName] = &IterInfo{
					ElemMsgName: f.TypeName,
					FieldName:   f.Name,
				}
			}
		}
	}
}

// isVarSizeType returns true if the type is variable-size.
func (a *Analysis) isVarSizeType(name string) bool {
	if _, ok := primitives[name]; ok {
		return false
	}
	ti, ok := a.Types[name]
	if !ok {
		return false
	}
	return !ti.IsFixedSize
}

// isStructReader returns true if a type's reader is a struct (not a []byte alias).
// This includes extent structs, multi-array messages, and var-section messages.
func (a *Analysis) isStructReader(name string) bool {
	ti, ok := a.Types[name]
	if !ok {
		return false
	}
	return ti.IsExtentStruct || ti.IsMultiArray || len(ti.VarSectionBoundaries) > 0
}

// unionHasVarSizeArm returns true if a union type has at least one non-void arm
// with a variable-size payload.
func (a *Analysis) unionHasVarSizeArm(typeName string) bool {
	uti, ok := a.Types[typeName]
	if !ok || uti.Decl.Kind != DeclUnion {
		return false
	}
	for _, arm := range uti.Decl.Union.Arms {
		if arm.Payload != "void" {
			if pti, ok := a.Types[arm.Payload]; ok && !pti.IsFixedSize {
				return true
			}
		}
	}
	return false
}

// isVarLenArray returns true if a field is a variable-length array (count reference, not literal size).
func isVarLenArray(f Field) bool {
	if f.ArrayLen == "" {
		return false
	}
	_, err := strconv.Atoi(f.ArrayLen)
	return err != nil
}

// isVarContentField returns true if a normal field introduces variable-length content.
func (a *Analysis) isVarContentField(f Field) bool {
	if f.Kind != FieldNormal {
		return false
	}
	if f.DiscRef != "" {
		return true
	}
	if isVarLenArray(f) {
		return true
	}
	if f.ArrayLen != "" {
		return false
	}
	return a.isVarSizeType(f.TypeName)
}

func extractBracketNum(s string) int {
	idx := strings.Index(s, "[")
	if idx < 0 {
		return 0
	}
	end := strings.Index(s, "]")
	if end < 0 {
		return 0
	}
	n, _ := strconv.Atoi(s[idx+1 : end])
	return n
}
