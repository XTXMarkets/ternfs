" Copyright 2026 XTX Markets Technologies Limited
"
" SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

" Vim syntax file for .msg (msgparse binary message format)
" Language: msg
" Latest Revision: 2026-03-11

if exists("b:current_syntax")
    finish
endif

" Keywords: declaration types
syntax keyword msgKeyword enum struct message union const

" Keywords: special identifiers
syntax keyword msgKeyword void default

" Directives: pad(N), align(N), extent(field)
syntax keyword msgDirective pad align extent

" Primitive types: integers
syntax keyword msgType u8 i8
syntax keyword msgType leu16 beu16 lei16 bei16
syntax keyword msgType leu32 beu32 lei32 bei32
syntax keyword msgType leu64 beu64 lei64 bei64

" Primitive types: floats
syntax keyword msgType lefloat befloat ledouble bedouble

" Primitive types: character/string
syntax keyword msgType char spchar npchar

" Comments (// only, no # or /* */)
syntax keyword msgTodo contained TODO FIXME XXX NOTE HACK
syntax match msgComment "//.*$" contains=msgTodo

" String literals: "..." with backslash escapes
syntax match msgStringEscape contained "\\."
syntax region msgString start=+"+ skip=+\\.+ end=+"+ contains=msgStringEscape

" Character literals: '.' with backslash escapes
syntax match msgCharEscape contained "\\."
syntax region msgChar start=+'+ skip=+\\.+ end=+'+ contains=msgCharEscape

" Number literals: hex (0x...) and decimal
syntax match msgNumber "\<0[xX][0-9a-fA-F]\+\>"
syntax match msgNumber "\<[0-9]\+\>"

" Braces, brackets, parens, semicolons for balanced highlighting
syntax match msgDelimiter "[{}()\[\];]"
syntax match msgOperator "[=:]"

" Highlight links
highlight default link msgKeyword   Keyword
highlight default link msgDirective PreProc
highlight default link msgType      Type
highlight default link msgTodo      Todo
highlight default link msgComment   Comment
highlight default link msgString    String
highlight default link msgChar      Character
highlight default link msgStringEscape Special
highlight default link msgCharEscape  Special
highlight default link msgNumber    Number
highlight default link msgDelimiter Delimiter
highlight default link msgOperator  Operator

let b:current_syntax = "msg"
