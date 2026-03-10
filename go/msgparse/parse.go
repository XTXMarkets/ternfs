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

// isReservedName returns true if name is a Go keyword or predeclared
// identifier that would cause problems if used as a generated variable name.
func isReservedName(name string) bool {
	switch name {
	// Go keywords
	case "break", "case", "chan", "const", "continue",
		"default", "defer", "else", "fallthrough", "for",
		"func", "go", "goto", "if", "import",
		"interface", "map", "package", "range", "return",
		"select", "struct", "switch", "type", "var":
		return true
	// Predeclared identifiers (builtins)
	case "append", "cap", "close", "complex", "copy",
		"delete", "imag", "len", "make", "max", "min",
		"new", "panic", "print", "println", "real", "recover":
		return true
	// Predeclared types
	case "bool", "byte", "comparable", "error",
		"float32", "float64", "complex64", "complex128",
		"int", "int8", "int16", "int32", "int64",
		"rune", "string", "uint", "uint8", "uint16", "uint32", "uint64",
		"uintptr", "any":
		return true
	// Predeclared constants
	case "true", "false", "iota", "nil":
		return true
	}
	return false
}

// Token types for the lexer.
type TokenKind int

const (
	tokEOF TokenKind = iota
	tokIdent
	tokNumber // decimal or hex
	tokString // "..."
	tokChar   // '.'
	tokLBrace // {
	tokRBrace // }
	tokLParen // (
	tokRParen // )
	tokLBrack // [
	tokRBrack // ]
	tokSemi   // ;
	tokColon  // :
	tokEquals // =
)

type Token struct {
	Kind TokenKind
	Text string
	Line int
}

// Lexer tokenizes .msg input.
type Lexer struct {
	src  []byte
	pos  int
	line int
}

func NewLexer(src []byte) *Lexer {
	return &Lexer{src: src, line: 1}
}

func (l *Lexer) Next() Token {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		// Skip whitespace
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.pos++
			continue
		}
		if ch == '\n' {
			l.pos++
			l.line++
			continue
		}

		// Skip line comments
		if ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
			continue
		}

		line := l.line

		// Single-character tokens
		switch ch {
		case '{':
			l.pos++
			return Token{tokLBrace, "{", line}
		case '}':
			l.pos++
			return Token{tokRBrace, "}", line}
		case '(':
			l.pos++
			return Token{tokLParen, "(", line}
		case ')':
			l.pos++
			return Token{tokRParen, ")", line}
		case '[':
			l.pos++
			return Token{tokLBrack, "[", line}
		case ']':
			l.pos++
			return Token{tokRBrack, "]", line}
		case ';':
			l.pos++
			return Token{tokSemi, ";", line}
		case ':':
			l.pos++
			return Token{tokColon, ":", line}
		case '=':
			l.pos++
			return Token{tokEquals, "=", line}
		}

		// String literal
		if ch == '"' {
			return l.lexString(line)
		}

		// Char literal
		if ch == '\'' {
			return l.lexCharLit(line)
		}

		// Number (decimal or hex)
		if ch >= '0' && ch <= '9' {
			return l.lexNumber(line)
		}

		// Identifier
		if ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			return l.lexIdent(line)
		}

		fmt.Fprintf(os.Stderr, "line %d: unexpected character %q\n", line, ch)
		l.pos++
	}
	return Token{tokEOF, "", l.line}
}

func (l *Lexer) lexString(line int) Token {
	l.pos++ // skip opening "
	var buf strings.Builder
	buf.WriteByte('"')
	for l.pos < len(l.src) && l.src[l.pos] != '"' {
		if l.src[l.pos] == '\\' && l.pos+1 < len(l.src) {
			buf.WriteByte('\\')
			l.pos++
			buf.WriteByte(l.src[l.pos])
			l.pos++
		} else {
			buf.WriteByte(l.src[l.pos])
			l.pos++
		}
	}
	if l.pos < len(l.src) {
		l.pos++ // skip closing "
	}
	buf.WriteByte('"')
	return Token{tokString, buf.String(), line}
}

func (l *Lexer) lexCharLit(line int) Token {
	start := l.pos
	l.pos++ // skip '
	for l.pos < len(l.src) && l.src[l.pos] != '\'' {
		if l.src[l.pos] == '\\' {
			l.pos++
		}
		l.pos++
	}
	if l.pos < len(l.src) {
		l.pos++ // skip closing '
	}
	return Token{tokChar, string(l.src[start:l.pos]), line}
}

func (l *Lexer) lexNumber(line int) Token {
	start := l.pos
	if l.src[l.pos] == '0' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == 'x' || l.src[l.pos+1] == 'X') {
		l.pos += 2
		for l.pos < len(l.src) && isHexDigit(l.src[l.pos]) {
			l.pos++
		}
	} else {
		for l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '9' {
			l.pos++
		}
	}
	return Token{tokNumber, string(l.src[start:l.pos]), line}
}

func (l *Lexer) lexIdent(line int) Token {
	start := l.pos
	for l.pos < len(l.src) && isIdentChar(l.src[l.pos]) {
		l.pos++
	}
	return Token{tokIdent, string(l.src[start:l.pos]), line}
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isIdentChar(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// Parser builds an AST from tokens.
type Parser struct {
	lex *Lexer
	cur Token
}

func NewParser(src []byte) *Parser {
	p := &Parser{lex: NewLexer(src)}
	p.advance()
	return p
}

func (p *Parser) advance() Token {
	old := p.cur
	p.cur = p.lex.Next()
	return old
}

func (p *Parser) expect(kind TokenKind) Token {
	if p.cur.Kind != kind {
		fmt.Fprintf(os.Stderr, "line %d: expected token kind %d, got %d (%q)\n",
			p.cur.Line, kind, p.cur.Kind, p.cur.Text)
		os.Exit(1)
	}
	return p.advance()
}

func (p *Parser) Parse() *MsgFile {
	file := &MsgFile{}
	for p.cur.Kind != tokEOF {
		switch p.cur.Text {
		case "enum":
			file.Decls = append(file.Decls, Decl{Kind: DeclEnum, Enum: p.parseEnum()})
		case "struct":
			file.Decls = append(file.Decls, Decl{Kind: DeclStruct, Struct: p.parseStruct()})
		case "message":
			file.Decls = append(file.Decls, Decl{Kind: DeclMessage, Message: p.parseMessage()})
		case "union":
			file.Decls = append(file.Decls, Decl{Kind: DeclUnion, Union: p.parseUnion()})
		case "const":
			file.Decls = append(file.Decls, Decl{Kind: DeclConstGroup, ConstGroup: p.parseConstGroup()})
		default:
			fmt.Fprintf(os.Stderr, "line %d: unexpected %q\n", p.cur.Line, p.cur.Text)
			os.Exit(1)
		}
	}
	return file
}

func (p *Parser) parseEnum() *EnumDecl {
	p.advance() // skip "enum"
	name := p.expect(tokIdent).Text
	p.expect(tokColon)

	// Parse underlying type: "leu32", "u8", "char[4]", "spchar[4]", etc.
	underlying := p.expect(tokIdent).Text
	if p.cur.Kind == tokLBrack {
		p.advance()
		n := p.expect(tokNumber).Text
		p.expect(tokRBrack)
		underlying += "[" + n + "]"
	}

	p.expect(tokLBrace)
	var consts []EnumConst
	for p.cur.Kind != tokRBrace {
		cname := p.expect(tokIdent).Text
		p.expect(tokEquals)
		var cval string
		switch p.cur.Kind {
		case tokNumber:
			cval = p.advance().Text
		case tokString:
			cval = p.advance().Text
		case tokChar:
			cval = p.advance().Text
		default:
			fmt.Fprintf(os.Stderr, "line %d: unexpected enum value %q\n", p.cur.Line, p.cur.Text)
			os.Exit(1)
		}
		p.expect(tokSemi)
		consts = append(consts, EnumConst{Name: cname, Value: cval})
	}
	p.expect(tokRBrace)
	return &EnumDecl{Name: name, Underlying: underlying, Constants: consts}
}

func (p *Parser) parseStruct() *StructDecl {
	p.advance() // skip "struct"
	name := p.expect(tokIdent).Text
	p.expect(tokLBrace)
	fields := p.parseFields()
	p.expect(tokRBrace)
	return &StructDecl{Name: name, Fields: fields}
}

func (p *Parser) parseMessage() *MessageDecl {
	p.advance() // skip "message"
	name := p.expect(tokIdent).Text
	p.expect(tokLBrace)
	fields := p.parseFields()
	p.expect(tokRBrace)
	return &MessageDecl{Name: name, Fields: fields}
}

func (p *Parser) parseFields() []Field {
	var fields []Field
	for p.cur.Kind != tokRBrace {
		if p.cur.Kind == tokIdent && p.cur.Text == "pad" {
			p.advance()
			p.expect(tokLParen)
			n, _ := strconv.Atoi(p.expect(tokNumber).Text)
			p.expect(tokRParen)
			p.expect(tokSemi)
			fields = append(fields, Field{Kind: FieldPad, Size: n})
		} else if p.cur.Kind == tokIdent && p.cur.Text == "align" {
			p.advance()
			p.expect(tokLParen)
			n, _ := strconv.Atoi(p.expect(tokNumber).Text)
			p.expect(tokRParen)
			p.expect(tokSemi)
			fields = append(fields, Field{Kind: FieldAlign, Size: n})
		} else if p.cur.Kind == tokIdent && p.cur.Text == "extent" {
			p.advance()
			p.expect(tokLParen)
			ref := p.expect(tokIdent).Text
			p.expect(tokRParen)
			p.expect(tokSemi)
			fields = append(fields, Field{Kind: FieldExtent, Ref: ref})
		} else {
			// Regular field: type name [arrayLen] ; or type name(disc) ;
			typeName := p.expect(tokIdent).Text

			nameTok := p.expect(tokIdent)
			name := nameTok.Text
			if isReservedName(name) {
				fmt.Fprintf(os.Stderr, "line %d: %q is a Go reserved word and cannot be used as a field name\n", nameTok.Line, name)
				os.Exit(1)
			}

			var arrayLen string
			var discRef string

			if p.cur.Kind == tokLBrack {
				p.advance()
				// Array length: number or identifier
				arrayLen = p.advance().Text
				p.expect(tokRBrack)
			}

			if p.cur.Kind == tokLParen {
				p.advance()
				discRef = p.expect(tokIdent).Text
				p.expect(tokRParen)
			}

			p.expect(tokSemi)
			fields = append(fields, Field{
				Kind:     FieldNormal,
				TypeName: typeName,
				Name:     name,
				ArrayLen: arrayLen,
				DiscRef:  discRef,
			})
		}
	}
	return fields
}

func (p *Parser) parseConstGroup() *ConstGroupDecl {
	p.advance() // skip "const"
	cg := &ConstGroupDecl{}
	// Parse one or more constants: const NAME = VALUE; [NAME = VALUE; ...]
	// First constant is required.
	for {
		name := p.expect(tokIdent).Text
		p.expect(tokEquals)
		var val string
		switch p.cur.Kind {
		case tokNumber:
			val = p.advance().Text
		case tokChar:
			val = p.advance().Text
		default:
			fmt.Fprintf(os.Stderr, "line %d: unexpected const value %q\n", p.cur.Line, p.cur.Text)
			os.Exit(1)
		}
		p.expect(tokSemi)
		cg.Constants = append(cg.Constants, ConstEntry{Name: name, Value: val})
		// Continue if next token is an identifier followed by '=' (another const in same block).
		// Stop if next token is a keyword or EOF.
		if p.cur.Kind != tokIdent {
			break
		}
		// Check if this looks like another const (NAME = ...) or a new declaration keyword.
		if p.cur.Text == "const" || p.cur.Text == "enum" || p.cur.Text == "struct" ||
			p.cur.Text == "message" || p.cur.Text == "union" {
			break
		}
	}
	return cg
}

func (p *Parser) parseUnion() *UnionDecl {
	p.advance() // skip "union"
	name := p.expect(tokIdent).Text
	p.expect(tokLParen)
	discType := p.expect(tokIdent).Text
	p.expect(tokRParen)
	p.expect(tokLBrace)

	var arms []UnionArm
	for p.cur.Kind != tokRBrace {
		var label string
		if p.cur.Kind == tokIdent && p.cur.Text == "default" {
			label = "default"
			p.advance()
		} else {
			label = p.expect(tokIdent).Text
		}
		p.expect(tokColon)
		payload := p.expect(tokIdent).Text
		p.expect(tokSemi)
		arms = append(arms, UnionArm{Label: label, Payload: payload})
	}
	p.expect(tokRBrace)
	return &UnionDecl{Name: name, DiscType: discType, Arms: arms}
}
