// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

// AST types for parsed .msg files.

type MsgFile struct {
	Decls []Decl // all declarations in file order
}

// Decl is a tagged union of declaration types.
type Decl struct {
	Kind       DeclKind
	Enum       *EnumDecl
	Struct     *StructDecl
	Message    *MessageDecl
	Union      *UnionDecl
	ConstGroup *ConstGroupDecl
}

type DeclKind int

const (
	DeclEnum DeclKind = iota
	DeclStruct
	DeclMessage
	DeclUnion
	DeclConstGroup
)

func (d Decl) Name() string {
	switch d.Kind {
	case DeclEnum:
		return d.Enum.Name
	case DeclStruct:
		return d.Struct.Name
	case DeclMessage:
		return d.Message.Name
	case DeclUnion:
		return d.Union.Name
	case DeclConstGroup:
		return "" // const groups are anonymous
	}
	return ""
}

// EnumDecl represents an enum declaration.
type EnumDecl struct {
	Name       string // e.g. "file_magic"
	Underlying string // e.g. "leu32", "u8", "char[4]", "spchar[4]"
	Constants  []EnumConst
}

type EnumConst struct {
	Name  string // e.g. "MAGIC_SLOG"
	Value string // raw value: "0x0001", "1", "'I'", "\"SLOG\""
}

// ConstGroupDecl represents a group of untyped constants.
type ConstGroupDecl struct {
	Constants []ConstEntry
}

// ConstEntry is a single named constant.
type ConstEntry struct {
	Name  string // e.g. "NFS4_FHSIZE"
	Value string // raw value: "128", "0x7fffffff"
}

// StructDecl represents a struct declaration.
type StructDecl struct {
	Name   string
	Fields []Field
}

// MessageDecl represents a message declaration.
type MessageDecl struct {
	Name   string
	Fields []Field
}

// UnionDecl represents a union declaration.
type UnionDecl struct {
	Name     string
	DiscType string // enum type name for discriminant
	Arms     []UnionArm
}

type UnionArm struct {
	Label   string // enum constant name, or "default"
	Payload string // type name, or "void"
}

// Field represents a field or directive in a struct/message.
type Field struct {
	Kind FieldKind

	// For regular fields:
	TypeName string // type name (primitive or user-defined)
	Name     string // field name
	ArrayLen string // "" for scalar, numeric for fixed array, field name for variable array
	DiscRef  string // for union fields: discriminant field name

	// For pad/align/extent:
	Size int    // N in pad(N), align(N)
	Ref  string // field name in extent(field)
}

type FieldKind int

const (
	FieldNormal FieldKind = iota
	FieldPad
	FieldAlign
	FieldExtent
)
