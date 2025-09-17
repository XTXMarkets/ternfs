// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <atomic>
#include <signal.h>

#include "Env.hpp"
#include "Assert.hpp"
#include "Exception.hpp"

std::ostream& operator<<(std::ostream& out, LogLevel ll) {
    switch (ll) {
    case LogLevel::LOG_TRACE:
        out << "TRACE";
        break;
    case LogLevel::LOG_DEBUG:
        out << "DEBUG";
        break;
    case LogLevel::LOG_INFO:
        out << "INFO";
        break;
    case LogLevel::LOG_ERROR:
        out << "ERROR";
        break;
    default:
        throw TERN_EXCEPTION("bad log level %s", (uint32_t)ll);
    }
    return out;
}

void Logger::toggleDebug() {
    if (_logLevel == LogLevel::LOG_DEBUG) {
        _logLevel = _savedLogLevel; 
    } else {
        _logLevel = LogLevel::LOG_DEBUG;
    }
}

static std::atomic<Logger*> debuggableLogger(nullptr);

extern "C" {
static void loggerSignalHandler(int signal_number) {
    (*debuggableLogger).toggleDebug();
}
}

void installLoggerSignalHandler(void* logger) {
    Logger* old = nullptr;
    if (!debuggableLogger.compare_exchange_strong(old, (Logger*)logger)) {
        throw TERN_EXCEPTION("Could not install logger signal handler, some other logger is already here.");
    }

    struct sigaction act;
    act.sa_handler = loggerSignalHandler;
    if (sigemptyset(&act.sa_mask) != 0) {
        SYSCALL_EXCEPTION("sigemptyset");
    }
    act.sa_flags = 0;

    // USR1 is used by undertaker
    if (sigaction(SIGUSR2, &act, NULL) < 0) {
        throw SYSCALL_EXCEPTION("sigaction");
    }
}

void tearDownLoggerSignalHandler(void* logger) {
    Logger* old = (Logger*)logger;
    if (!debuggableLogger.compare_exchange_strong(old, nullptr)) {
        throw TERN_EXCEPTION("Could not tear down logger signal handler, bad preexisting logger");
    }

    struct sigaction act;
    act.sa_handler = SIG_DFL;
    if (sigemptyset(&act.sa_mask) != 0) {
        SYSCALL_EXCEPTION("sigemptyset");
    }
    act.sa_flags = 0;

    if (sigaction(SIGUSR2, &act, NULL) < 0) {
        throw SYSCALL_EXCEPTION("sigaction");
    }
}
