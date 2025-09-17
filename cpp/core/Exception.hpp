// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <string.h>
#include <string>
#include <sstream>

#include "FormatTuple.hpp"
#include "strerror.h"

#define TERN_EXCEPTION(...) TernException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), VALIDATE_FORMAT(__VA_ARGS__))
#define SYSCALL_EXCEPTION(...) SyscallException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), errno, VALIDATE_FORMAT(__VA_ARGS__))
#define EXPLICIT_SYSCALL_EXCEPTION(rc, ...) SyscallException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), rc, VALIDATE_FORMAT(__VA_ARGS__))
#define FATAL_EXCEPTION(...) FatalException(__LINE__, SHORT_FILE, removeTemplates(__PRETTY_FUNCTION__).c_str(), VALIDATE_FORMAT(__VA_ARGS__))

std::string removeTemplates(const std::string & s);
const char *translateErrno(int _errno);

class AbstractException : public std::exception {
public:
    virtual const char *what() const throw() override = 0;
};


class TernException : public AbstractException {
public:
    template <typename TFmt, typename ... Args>
    TernException(int line, const char *file, const char *function, TFmt fmt, Args ... args);
    virtual const char *what() const throw() override;

private:
    std::string _msg;
};


class SyscallException : public AbstractException {
public:
    template <typename ... Args>
    SyscallException(int line, const char *file, const char *function, int capturedErrno, const char *format, Args ... args);
    virtual const char *what() const throw() override;

private:
    int _errno;
    std::string _msg;
};

class FatalException : public AbstractException {
public:
    template <typename ... Args>
    FatalException(int line, const char *file, const char *function, const char *format, Args ... args);
    virtual const char *what() const throw() override;

private:
    std::string _msg;
};



class AssertionException : public AbstractException {
public:
    template <typename ... Args>
    AssertionException(int line, const char *file, const char *function, const char *expr, const char* fmt, Args ... args);
    AssertionException(int line, const char *file, const char *function, const char *expr) : AssertionException(line, file, function, expr, nullptr) {}
    virtual const char *what() const throw() override { return _msg.c_str(); }

private:
    std::string _msg;
};

template <typename TFmt, typename ... Args>
TernException::TernException(int line, const char *file, const char *function, TFmt fmt, Args ... args) {

    std::stringstream ss;
    ss << "TernException(" << file << "@" << line << " in " << function << "):\n";
    format_pack(ss, fmt, args...);

    _msg = ss.str();
}


template <typename ... Args>
SyscallException::SyscallException(int line, const char *file, const char *function, int capturedErrno, const char *fmt, Args ... args) :
    _errno(capturedErrno)
{

    const char* errmsg = safe_strerror(_errno);

    std::stringstream ss;
    ss << "SyscallException(" << file << "@" << line << ", " << _errno << "/" << translateErrno(_errno) << "=" << errmsg << " in " << function << "): ";
    format_pack(ss, fmt, args...);

    _msg = ss.str();
}

template <typename ... Args>
FatalException::FatalException(int line, const char *file, const char *function, const char *fmt, Args ... args) {

    std::stringstream ss;
    ss << "FatalException(" << file << "@" << line << " in " << function << "): ";
    format_pack(ss, fmt, args...);

    _msg = ss.str();
}


template <typename ... Args>
AssertionException::AssertionException(int line, const char *file, const char *function, const char *expr, const char* fmt, Args ... args) {
    std::stringstream ss;
    ss << "AssertionException(" << file << "@" << line << " in " << function << "):\n";
    ss << "Expected: " << expr;
    if (fmt != nullptr && fmt[0] != '\0') {
        ss << "\nMessage: ";
        format_pack(ss, fmt, args...);
    }
    _msg = ss.str();
}
