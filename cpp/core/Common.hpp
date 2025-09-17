// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <iosfwd>

////////////////////////////////////////////
// Compiler hints
////////////////////////////////////////////

// BUG NOTICE: http://gcc.gnu.org/bugzilla/show_bug.cgi?id=47466
// --> this kicks in for statements such as unlikely( a && b );
// --> see assembler generated for t_compiler.cpp
// --> the unlikely case is favoured by the compiler

#define likely(x)       __builtin_expect(!!(x), 1)
#define unlikely(x)     __builtin_expect(!!(x), 0)

#define PACKED          __attribute__((packed))
#define ALIGNED(n)      __attribute__((aligned(n)))
#define ALWAYS_INLINE   __attribute__((always_inline))
#define NO_INLINE       __attribute__((noinline))
#define FORMAT(n)       __attribute__((format(printf,n+1,n+2)))
#define NORETURN        __attribute__((noreturn))
#define CONST           __attribute__((const))
#define PURE            __attribute__((pure))
#define HOT             __attribute__((hot))
#define COLD            __attribute__((cold))
#define UNUSED          __attribute__((unused))
#define DEPRECATED      __attribute__((deprecated))
#define CHECK_RETURN    __attribute__((warn_unused_result))

///////////////////////////////////
// Debugging features
///////////////////////////////////

#define SHORT_FILE (strrchr(__FILE__, '/') ? strrchr(__FILE__, '/') + 1 : __FILE__)

std::ostream& operator<<(std::ostream& out, const struct sockaddr_in& addr);
std::ostream& operator<<(std::ostream& out, const struct in_addr& addr);

std::ostream& goLangBytesFmt(std::ostream& out, const char* str, size_t len);

struct GoLangBytesFmt {
    const char* str;
    size_t len;

    GoLangBytesFmt(const char* str_, size_t len_) : str(str_), len(len_) {}
};

std::ostream& operator<<(std::ostream& out, const GoLangBytesFmt& bytes);

std::ostream& goLangQuotedStringFmt(std::ostream& out, const char* str, size_t len);

struct GoLangQuotedStringFmt {
    const char* str;
    size_t len;

    GoLangQuotedStringFmt(const char* str_, size_t len_) : str(str_), len(len_) {}
};

std::ostream& operator<<(std::ostream& out, const GoLangQuotedStringFmt& bytes);
