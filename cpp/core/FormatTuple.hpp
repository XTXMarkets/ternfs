// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <stddef.h>
#include <stdexcept>
#include <utility>
#include <sstream>
#include <tuple>

#define VALIDATE_FORMAT(fmt, ...) ({\
    static_assert(ft_detail::placeholders_count_is(fmt, decltype(ft_detail::argument_counter(__VA_ARGS__))::value), "Wrong number of placeholders or format arguments"); \
    fmt; }),## __VA_ARGS__

struct do_not_validate_t {};
constexpr do_not_validate_t do_not_validate = do_not_validate_t();

inline constexpr size_t constexpr_strlen(const char* s) {
    const char* end = s;
    while (*end != '\0')
        ++end;
    return end - s;
}


inline constexpr size_t count_format_placeholders(const char* s, bool onlyAllowPercentS) {
    // throwing in a constexpr function guarantees a compile time error when this function
    // is used in a constant expression (such as in static_assert)
    bool prev_sym_pct_sign = false;
    size_t len = constexpr_strlen(s);
    size_t count = 0;
    for (size_t i = 0; i != len; ++i) {
        if (prev_sym_pct_sign) {
            if (s[i] == 's')
                ++count;
            else if (s[i] != '%' && onlyAllowPercentS) {
                throw std::runtime_error("% in format string must be followed by % or s");
            }
            prev_sym_pct_sign = false;
        }
        else if (s[i] == '%') {
            prev_sym_pct_sign = true;
        }
    }
    if (prev_sym_pct_sign)
        throw std::runtime_error("format string can not end with '%'");
    return count;
}

namespace ft_detail {

const char* write_up_to_next_percent_s(std::ostream &os, const char *fmt);
void write_fmt_tail(std::ostream &os, const char *fmt);

template<size_t Skip = 0, typename Tuple, size_t... I>
void format_tuple(std::ostream &os, const char *fmt, const Tuple &t, std::integer_sequence<size_t, I...>) {
    ((fmt = write_up_to_next_percent_s(os, fmt), os << std::get<Skip + I>(t)), ...), write_fmt_tail(os, fmt);
}

inline constexpr bool placeholders_count_is(const char* s, size_t expected_count) {
    return count_format_placeholders(s, true) == expected_count;
}

inline constexpr bool placeholders_count_is(do_not_validate_t, size_t) {
    return true;
}

template <typename ...Args>
constexpr auto argument_counter(Args&&... args) -> std::integral_constant<size_t, sizeof...(Args)>;

template<size_t NPlaceholders>
class FString {
public:
    constexpr FString(const char * fmt) : _fmt(fmt) {}

    template<typename ...Args>
    std::string operator()(Args&&... args) const {
        static_assert(sizeof...(Args) == NPlaceholders, "Wrong number of placeholders or format arguments");
        std::ostringstream os;
        ft_detail::format_tuple(os, _fmt, std::forward_as_tuple(std::forward<Args>(args)...), std::make_integer_sequence<size_t, sizeof...(Args)>());
        return os.str();
    }

private:
    const char * _fmt;
};

} // namespace ft_detail

template<typename ...Args>
void format_pack(std::ostream &os, const char *fmt, Args&&... args) {
    ft_detail::format_tuple(os, fmt, std::forward_as_tuple(std::forward<Args>(args)...), std::make_integer_sequence<size_t, sizeof...(Args)>());
};

template<typename ...Args>
void format_pack(std::ostream &os, do_not_validate_t, const char *fmt, Args&&... args) {
    ft_detail::format_tuple(os, fmt, std::forward_as_tuple(std::forward<Args>(args)...), std::make_integer_sequence<size_t, sizeof...(Args)>());
};


template<size_t Skip=0, typename ...Args>
void format_tuple(std::ostream &os, const char *fmt, const std::tuple<Args...>& args) {
    ft_detail::format_tuple<Skip>(os, fmt, args, std::make_integer_sequence<size_t, sizeof...(Args) - Skip>());
};


template<typename C, C... c>
inline auto operator ""_f() {
    static constexpr const char fmt[sizeof...(c)+1] = {c..., 0};
    constexpr size_t n_placeholders = count_format_placeholders(fmt, true);
    return ft_detail::FString<n_placeholders>(fmt);
}

