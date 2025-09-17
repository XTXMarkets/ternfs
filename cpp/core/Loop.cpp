// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#include <atomic>
#include <pthread.h>
#include <signal.h>
#include <sys/epoll.h>
#include <unistd.h>

#include "Assert.hpp"
#include "Loop.hpp"
#include "Exception.hpp"

// The sigset we run with normally, with SIGINT/SIGTERM masked.
static sigset_t baseSigset;
// The sigset to run when running blocking syscalls, with SIGINT/SIGTERM unmasked.
static sigset_t blockingSigset;

__attribute__((constructor))
static void setupSigsets() {
    sigemptyset(&baseSigset);
    blockingSigset = baseSigset;
    sigaddset(&baseSigset, SIGINT);
    sigaddset(&baseSigset, SIGTERM);
}

thread_local std::atomic<bool> stopLoop;
static std::atomic<pthread_t> mainThread;

static void stopLoopHandler(int signum) {
    // make sure leader will terminate
    if (pthread_self() != mainThread.load(std::memory_order_acquire)) {
        pthread_kill(mainThread, SIGTERM);
    }
    stopLoop.store(true, std::memory_order_relaxed);
}

void LoopThread::sendStop() {
    _loop->sendStop();
    auto stopLoopPtr = _stopLoop.load(std::memory_order_relaxed);
    if (stopLoopPtr) {
        stopLoopPtr->store(true, std::memory_order_relaxed);
    }
    pthread_kill(_thread, SIGTERM);
}

Loop::Loop(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const std::string& name) : _env(logger, xmon, name), _name(name) {}

void* LoopThread::runLoop(void* arg) {
    LoopThread* loopThread = (LoopThread*) arg;
    loopThread->_stopLoop.store(&stopLoop, std::memory_order_relaxed);


    // there's a race between starting the thread and getting the
    // sigmask, but then again we haven't setup the handler at
    // that point yet, so the whole process would just go down.
    //
    // it would be good to use `pthread_attr_setsigmask_np` but it's
    // a recent glibc feature (~2020) which musl does not support
    // yet.
    {
        int ret = pthread_sigmask(SIG_SETMASK, &baseSigset, nullptr);
        if (ret != 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(ret, "pthread_sigmask");
        }
    }
    {
        int ret = pthread_setname_np(pthread_self(), loopThread->_loop->name().c_str());
        if (ret != 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(ret, "pthread_setname_np");
        }
    }
    loopThread->_loop->run();
    return nullptr;
}

std::unique_ptr<LoopThread> LoopThread::Spawn(std::unique_ptr<Loop>&& loop) {
    auto thread = std::unique_ptr<LoopThread>(new LoopThread(std::move(loop)));
    {
        int res = pthread_create(&thread->_thread, nullptr, &runLoop, thread.get());
        if (res != 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(res, "pthread_create");
        }
    }
    return thread;
}

void Loop::run() {
    while (!stopLoop.load()) { step(); }
}

int Loop::poll(struct pollfd* fds, nfds_t nfds, Duration timeout) {
    struct timespec spec = timeout.timespec();
    return ppoll(fds, nfds, timeout < 0 ? nullptr : &spec, &blockingSigset);
}

int Loop::epollWait(int epfd, struct epoll_event* events, int maxevents, Duration timeout) {
    return epoll_pwait(epfd, events, maxevents, static_cast<int>(timeout.ns / 1000000), &blockingSigset);
}

void Loop::stop() {
    stopLoop.store(true, std::memory_order_release);
}

int Loop::sleep(Duration d) {
    auto tspec = d.timespec();
    return ppoll(nullptr, 0, &tspec, &blockingSigset);
}

void LoopThread::waitUntilStopped(LoopThreads& loops) {
    ALWAYS_ASSERT(getpid() == gettid(), "You can only run this function from the main thread");

    // mask signals here too
    {
        int ret = pthread_sigmask(SIG_SETMASK, &baseSigset, nullptr);
        if (ret < 0) {
            throw EXPLICIT_SYSCALL_EXCEPTION(ret, "pthread_sigmask");
        }
    }

    // fill in mainThread -- important to do this _before_
    // setting up the signal handler
    mainThread.store(pthread_self(), std::memory_order_release);

    // setup signal handler
    {
        struct sigaction sa;
        memset(&sa, 0, sizeof(sa));
        sa.sa_handler = &stopLoopHandler;
        if (sigaction(SIGTERM, &sa, nullptr) < 0) {
            throw SYSCALL_EXCEPTION("sigaction");
        }
        if (sigaction(SIGINT, &sa, nullptr) < 0) {
            throw SYSCALL_EXCEPTION("sigaction");
        }
    }

    // start waiting
    while (!stopLoop.load()) {
        int ret = Loop::sleep(1_hours);
        if (ret < 0 && errno != EINTR) {
            throw SYSCALL_EXCEPTION("ppoll");
        }
    }

    // we've been told to stop, tear down all threads
    for (auto& loop : loops) {
        loop->sendStop();
    }
    for (const auto& loop: loops) {
        struct timespec timeout;
        if (clock_gettime(CLOCK_REALTIME, &timeout) < 0) {
            throw SYSCALL_EXCEPTION("clock_gettime");
        }
        timeout.tv_sec += 10;
        for (;;) {
            int ret = pthread_timedjoin_np(loop->_thread, nullptr, &timeout);
            if (ret == EINTR) { continue; }
            if (ret != 0) {
                if (ret == ETIMEDOUT) {
                    char name[16];
                    {
                        int ret = pthread_getname_np(loop->_thread, name, sizeof(name));
                        if (ret != 0) {
                            throw EXPLICIT_SYSCALL_EXCEPTION(ret, "pthread_getname_np");
                        }
                    }
                    throw TERN_EXCEPTION("loop %s has not terminated in time, aborting", name);
                }
                throw EXPLICIT_SYSCALL_EXCEPTION(ret, "pthread_timedjoin_np");
            }
            break;
        }
    }
}
