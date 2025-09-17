// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include "Common.hpp"
#include "FormatTuple.hpp"
#include "Assert.hpp"

#include <iostream>

namespace detail {

class BinaryLogDataBase {
public:
    virtual void execute(std::ostream&) const = 0;
    virtual size_t size() const = 0;
    virtual void move(char * buf) = 0;
    virtual ~BinaryLogDataBase() = default;
};

template<typename ... Brgs>
class BinaryLogData : public BinaryLogDataBase {
public:
    template<typename ...Crgs>
    explicit constexpr BinaryLogData(Crgs && ... crgs) :
        _tup(std::forward<Crgs>(crgs)...) {
    }

    size_t size() const override {
        return sizeof(BinaryLogData);
    }

    void execute(std::ostream & os) const override {
        format_tuple<1>(os, std::get<0>(this->_tup), this->_tup);
    }

    void move(char * buf) override {
        std::apply([buf](auto&&... x) { new (buf) BinaryLogData(std::move(x)...); }, std::move(_tup));
    }

private:
    std::tuple<std::decay_t<Brgs>...> _tup;
};

template<typename T>
inline auto binaryFormatPrepareHelper(T && rhs, int) -> decltype(binaryFormatPrepare(std::forward<T>(rhs))) {
    return binaryFormatPrepare(std::forward<T>(rhs));
}

template<typename T>
inline auto binaryFormatPrepareHelper(T && rhs, long) -> decltype(std::cout << std::declval<T>(), std::declval<T>()) && {
    return std::forward<T>(rhs);
}


}


template<typename ...Args>
constexpr size_t binaryFormatSize(Args && ...args) {
    //TODO: const char * thing
    typedef detail::BinaryLogData<const char *, std::decay_t<decltype(detail::binaryFormatPrepareHelper(std::forward<Args>(args), 0))>...> T;
    return (sizeof(T) + 7) & -8;
}


template<typename ...Args>
size_t binaryFormatWrite(char *buf, Args && ...args) {
    ASSERT(reinterpret_cast<uintptr_t>(buf) % alignof(size_t) == 0);
    typedef detail::BinaryLogData<std::decay_t<decltype(detail::binaryFormatPrepareHelper(std::forward<Args>(args), 0))>...> T;
    new (buf) T(detail::binaryFormatPrepareHelper(std::forward<Args>(args), 0)...);
    return (sizeof(T) + 7) & -8;
}

