// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "FormatTuple.hpp"

namespace ft_detail {

const char* write_up_to_next_percent_s(std::ostream &os, const char *fmt) {
    const char* head = fmt;
    for (; *fmt; ++fmt) {
        if (*fmt == '%') {
            ++fmt;
            if (*fmt == '\0') {
                break;
            } else if (*fmt == 's') {
                os.write(head, fmt - head - 1);
                return fmt + 1;
            } else if (*fmt == '%') {
                os.write(head, fmt - head - 1);
                head = fmt;
            }
        }
    }
    os.write(head, fmt - head);
    os << " <missing %s> ";
    return fmt;
}

void write_fmt_tail(std::ostream &os, const char *fmt) {
    const char* head = fmt;
    for (; *fmt; ++fmt) {
        if (*fmt == '%') {
            ++fmt;
            if (*fmt == '\0') {
                break;
            } else if (*fmt == 's') {
                os.write(head, fmt - head - 1);
                os << "<too many %s>";
                head = fmt + 1;
            } else if (*fmt == '%') {
                os.write(head, fmt - head - 1);
                head = fmt;
            }
        }
    }
    os.write(head, fmt - head);
}

}
