// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

#pragma once

#include <pthread.h>
#include <poll.h>

#include "Env.hpp"

// Each loop runs with SIGINT/SIGTERM blocked. It's expected that any
// non-time-bounded syscalls which step runs has SIGINT/SIGTERM unmasked
// (e.g. using ppoll).
struct Loop {
protected:
    Env _env;

private:
    std::string _name;

protected:
    // can be used to stop the thread from within `run`, as opposed to doing
    // `kill()`
    void stop();

public:
    Loop(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const std::string& name);
    virtual ~Loop() = default;

    const std::string& name() const { return _name; }

    virtual void step() = 0;

    void run();

    // Override if additional work is needed to signal stop. By default
    // all we do is send SIGTERM (see `LoopThread::sendStop`).
    virtual void sendStop() {};

    // Polls forever with SIGINT/SIGTERM unmasked. If timeout < 0, waits forever.
    // If timeout == 0, returns immediately. If timeout > 0, it'll wait.
    static int poll(struct pollfd* fds, nfds_t nfds, Duration timeout);
    static int epollWait(int epfd, struct epoll_event* events, int maxevents, Duration timeout);

    // Sleeps with SIGINT/SIGTERM unmasked.
    static int sleep(Duration d);
};

struct LoopThread;
typedef std::vector<std::unique_ptr<LoopThread>> LoopThreads;
struct LoopThread {
    // This will remember the sigmask that it's been called with, and use it
    // to unmask stuff in Loop::poll/Loop::sleep.
    static std::unique_ptr<LoopThread> Spawn(std::unique_ptr<Loop>&& loop);

    static void waitUntilStopped(LoopThreads& loops);

    virtual ~LoopThread() = default;
    LoopThread(const LoopThread&) = delete;
private:
    LoopThread(std::unique_ptr<Loop>&& loop) : _loop(std::move(loop)), _stopLoop(nullptr) {}
    static void* runLoop(void* arg);
    void sendStop();
    std::unique_ptr<Loop> _loop;
    pthread_t _thread;
    std::atomic<std::atomic<bool>*> _stopLoop;
};
