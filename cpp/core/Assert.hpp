// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "Common.hpp"
#include "Exception.hpp"

// Assert macros take an optional FormatTuple-style message, e.g.:
// ASSERT(result != 0, "expected nonzero result, got %s", result);
// 
// Plain ASSERT(result != 0) also works.

// Assert which fires in all builds including release, essentially shorthand for
// if (!expr) then throw
#define ALWAYS_ASSERT(expr, ...) do {  \
    if (unlikely(!(expr)))  \
        throw AssertionException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), #expr, ## __VA_ARGS__);  \
    } while (false)

#if defined(TERN_DEBUG)

    #define ASSERT(expr, ...) ALWAYS_ASSERT(expr, ## __VA_ARGS__)

    #define CHECK_UNREACHABLE() do {  \
        throw AssertionException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), "unreachable code"); \
    } while (false)

    #define IF_DEBUG(expr) do {  \
        expr;  \
    } while (false)

#else

    #define ASSERT(expr, ...) do {  \
        (void)sizeof(expr);  \
    } while (false)

    #define CHECK_UNREACHABLE() __builtin_unreachable()

    #define IF_DEBUG(expr) do { } while (false)

#endif
