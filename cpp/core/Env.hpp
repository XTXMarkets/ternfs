// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <cstdint>
#include <mutex>
#include <sstream>
#include <string>
#include <signal.h>
#include <memory>

#include "Common.hpp"
#include "Exception.hpp"
#include "Time.hpp"
#include "XmonAgent.hpp"

enum class LogLevel : uint32_t {
    LOG_TRACE = 0,
    LOG_DEBUG = 1,
    LOG_INFO = 2,
    LOG_ERROR = 3,
};

std::ostream& operator<<(std::ostream& out, LogLevel ll);

void installLoggerSignalHandler(void* logger);
void tearDownLoggerSignalHandler(void* logger);

struct Logger {
private:
    LogLevel _logLevel;
    LogLevel _savedLogLevel;
    int _fd;
    std::mutex _mutex;
    bool _syslog;
    bool _usr2ToDebug;
public:
    Logger(LogLevel logLevel, int fd, bool syslog, bool usr2ToDebug):
        _logLevel(logLevel), _savedLogLevel(logLevel), _fd(fd), _syslog(syslog), _usr2ToDebug(usr2ToDebug)
    {
        if (usr2ToDebug) {
            installLoggerSignalHandler(this);
        }
    }

    ~Logger() {
        if (_usr2ToDebug) {
            tearDownLoggerSignalHandler(this);
        }
    }

    template<typename ...Args>
    void _log(LogLevel level, const std::string& prefix, const char* fmt, Args&&... args) {
        std::stringstream formatSs;
        std::stringstream outSs;
        format_pack(formatSs, fmt, args...);
        std::string line;
        if (likely(_syslog)) {
            int syslogLevel;
            switch (level) {
                case LogLevel::LOG_TRACE:
                case LogLevel::LOG_DEBUG:
                    syslogLevel = 7; break;
                case LogLevel::LOG_INFO:
                    syslogLevel = 6; break;
                case LogLevel::LOG_ERROR:
                    syslogLevel = 3; break;
                default:
                    syslogLevel = 3; break; // should be throw?
            }
            while (std::getline(formatSs, line)) {
                outSs << "<" << syslogLevel << ">" << prefix << ": " << line << std::endl;
            }
        } else {
            auto t = ternNow();
            while (std::getline(formatSs, line)) {
                outSs << t << " " << prefix << " [" << level << "] " << line << std::endl;
            }
        }
        {
            std::scoped_lock lock(_mutex);
            std::string_view v = outSs.view();
            int remaining = v.size();
            while (remaining) {
                int res = write(_fd, v.data(), v.size());
                if (res < 0) {
                    throw SYSCALL_EXCEPTION("write");
                }
                remaining -= res;
            }
        }
    }

    bool _shouldLog(LogLevel level) {
        return level >= _logLevel;
    }

    void toggleDebug();
};

struct Env {
private:
    Logger& _logger;
    std::shared_ptr<XmonAgent> _xmon;
    std::string _prefix;
public:
    Env(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const std::string& prefix): _logger(logger), _xmon(xmon), _prefix(prefix) {}

    Logger& logger() { return _logger; }
    std::shared_ptr<XmonAgent> xmon() { return _xmon; }

    template<typename ...Args>
    void _log(LogLevel level, const char* fmt, Args&&... args) {
        _logger._log(level, _prefix, fmt, std::forward<Args>(args)...);
    }

    template<typename ...Args>
    void raiseAlertAppType(XmonAppType appType, const char* fmt, Args&&... args) {
        std::stringstream ss;
        format_pack(ss, fmt, args...);
        std::string line;
        std::string s = ss.str();
        _log(LogLevel::LOG_ERROR, s.c_str());
        if (likely(_xmon)) {
            _xmon->raiseAlert(appType, s);
        }
    }

    template<typename ...Args>
    void raiseAlert(const char* fmt, Args&&... args) {
        raiseAlertAppType(XmonAppType::DEFAULT, fmt, args...);
    }

    template<typename ...Args>
    void updateAlert(XmonNCAlert& alert, const char* fmt, Args&&... args) {
        std::stringstream ss;
        format_pack(ss, fmt, args...);
        std::string line;
        std::string s = ss.str();
        _log(LogLevel::LOG_ERROR, s.c_str());
        if (likely(_xmon)) {
            _xmon->updateAlert(alert, s);
        }
    }

    void clearAlert(XmonNCAlert& alert) {
        if (likely(_xmon)) {
            _xmon->clearAlert(alert);
        }
    }

    bool _shouldLog(LogLevel level) {
        return _logger._shouldLog(level);
    }
};

#ifdef TERN_DEBUG
    #define LOG_TRACE(env, ...) \
        do { \
            if (unlikely((env)._shouldLog(LogLevel::LOG_TRACE))) { \
                (env)._log(LogLevel::LOG_TRACE, VALIDATE_FORMAT(__VA_ARGS__)); \
            } \
        } while (false)
#else
    #define LOG_TRACE(env, ...) do {} while (false)
#endif

#define LOG_DEBUG(env, ...) \
    do { \
        if (unlikely((env)._shouldLog(LogLevel::LOG_DEBUG))) { \
            (env)._log(LogLevel::LOG_DEBUG, VALIDATE_FORMAT(__VA_ARGS__)); \
        } \
    } while (false)

#define LOG_INFO(env, ...) \
    do { \
        if (likely((env)._shouldLog(LogLevel::LOG_INFO))) { \
            (env)._log(LogLevel::LOG_INFO, VALIDATE_FORMAT(__VA_ARGS__)); \
        } \
    } while (false)

#define LOG_ERROR(env, ...) \
    do { \
        if (likely((env)._shouldLog(LogLevel::LOG_ERROR))) { \
            (env)._log(LogLevel::LOG_ERROR, VALIDATE_FORMAT(__VA_ARGS__)); \
        } \
    } while (false)

#define RAISE_ALERT(env, ...) \
    do { \
        (env).raiseAlert(VALIDATE_FORMAT(__VA_ARGS__)); \
    } while (false)

#define RAISE_ALERT_APP_TYPE(env, appType, ...) \
    do { \
        (env).raiseAlertAppType(appType, VALIDATE_FORMAT(__VA_ARGS__)); \
    } while (false)
