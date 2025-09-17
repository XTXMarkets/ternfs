// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include "Common.hpp"

#include <arpa/inet.h>
#include <netinet/in.h>
#include <string.h>
#include <unistd.h>
#include <iomanip>

std::ostream& operator<<(std::ostream& out, const struct sockaddr_in& addr) {
    char buf[INET_ADDRSTRLEN];
    inet_ntop(AF_INET, &addr.sin_addr, buf, sizeof(buf));
    out << buf << ":" << ntohs(addr.sin_port);
    return out;
}

std::ostream& operator<<(std::ostream& out, const struct in_addr& addr) {
    char buf[INET_ADDRSTRLEN];
    inet_ntop(AF_INET, &addr, buf, sizeof(buf));
    out << buf;
    return out;
}

std::ostream& goLangBytesFmt(std::ostream& out, const char* str, size_t len) {
    out << "[";
    const uint8_t* data = (const uint8_t*)str;
    for (int i = 0; i < len; i++) {
        if (i > 0) {
            out << " ";
        }
        out << (int)data[i];
    }
    out << "]";
    return out;
}

std::ostream& operator<<(std::ostream& out, const GoLangBytesFmt& bytes) {
    return goLangBytesFmt(out, bytes.str, bytes.len);
}

std::ostream& goLangQuotedStringFmt(std::ostream& out, const char* data, size_t len) {
    out << "\"";
    for (int i = 0; i < len; i++) {
        uint8_t ch = static_cast<uint8_t>(data[i]);
        if (isprint(ch)) {
            out << ch;
        } else if (ch == 0) {
            out << "\\0";
        } else {
            const char cfill = out.fill();
            out << std::hex << std::setfill('0');
            out << "\\x" << std::setw(2) << (int)ch;
            out << std::setfill(cfill) << std::dec;
        }
    }
    out << "\"";
    return out;
}

std::ostream& operator<<(std::ostream& out, const GoLangQuotedStringFmt& bytes) {
    return goLangQuotedStringFmt(out, bytes.str, bytes.len);
}