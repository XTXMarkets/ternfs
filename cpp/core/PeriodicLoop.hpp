#pragma once

#include "Exception.hpp"
#include "Time.hpp"
#include "Env.hpp"
#include "Loop.hpp"
#include "wyhash.h"

struct PeriodicLoopConfig {
    Duration failureInterval; // waiting between failures
    double failureIntervalJitter = 1.0;
    Duration successInterval; // waiting between successes
    double successIntervalJitter = 1.0;

    PeriodicLoopConfig(Duration failureInterval_, Duration successInterval_) : failureInterval(failureInterval_), successInterval(successInterval_) {}
    PeriodicLoopConfig(Duration failureInterval_, double failureIntervalJitter_, Duration successInterval_, double successIntervalJitter_) :
        failureInterval(failureInterval_), failureIntervalJitter(failureIntervalJitter_), successInterval(successInterval_), successIntervalJitter(successIntervalJitter_)
    {}
};

struct PeriodicLoop : Loop {
private:
    PeriodicLoopConfig _config;
    uint64_t _rand;
    bool _lastSucceded;

public:
    PeriodicLoop(Logger& logger, std::shared_ptr<XmonAgent>& xmon, const std::string& name, const PeriodicLoopConfig& config) :
        Loop(logger, xmon, name),
        _config(config),
        _rand(eggsNow().ns),
        _lastSucceded(false)
    {}

    // true = success, false = failure
    virtual bool periodicStep() = 0;

    // We sleep first to immediately introduce a jitter.
    virtual void step() override {
        auto t = eggsNow();
        Duration pause;
        if (_lastSucceded) {
            pause = _config.successInterval + Duration((double)_config.successInterval.ns * (_config.successIntervalJitter * wyhash64_double(&_rand)));
            LOG_DEBUG(_env, "periodic step succeeded, next step at %s", t + pause);
        } else {
            pause = _config.failureInterval + Duration((double)_config.failureInterval.ns * (_config.failureIntervalJitter * wyhash64_double(&_rand)));
            LOG_DEBUG(_env, "periodic step failed, next step at %s", t + pause);
        }
        if (Loop::sleep(pause) < 0) {
            if (errno == EINTR) { return; }
            throw SYSCALL_EXCEPTION("sleep");
        }
        _lastSucceded = periodicStep();
    }
};