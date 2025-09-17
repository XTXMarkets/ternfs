// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

#pragma once


#include <memory>


#include "Env.hpp"
#include "Loop.hpp"

#include "RegistryCommon.hpp"

class Registry {
public:
    explicit Registry(Logger& logger, std::shared_ptr<XmonAgent> xmon);
    ~Registry();
    void start(const RegistryOptions& options, LoopThreads& threads);
    void close();
private:
    Env _env;
    std::unique_ptr<RegistryState> _state;
};
