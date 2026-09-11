// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"flag"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
)

// Runtime contains the process-wide dependencies shared by subcommands.
type Runtime struct {
	Log             *log.Logger
	RegistryAddress string

	getClient func() *client.Client
}

func NewRuntime(
	l *log.Logger,
	registryAddress string,
	getClient func() *client.Client,
) *Runtime {
	return &Runtime{
		Log:             l,
		RegistryAddress: registryAddress,
		getClient:       getClient,
	}
}

// Client returns the process-wide TernFS client, creating it on first use.
func (runtime *Runtime) Client() *client.Client {
	return runtime.getClient()
}

// Command contains one subcommand's flags and deferred implementation.
type Command struct {
	Flags *flag.FlagSet
	Run   func(*Runtime)
}

func (command Command) FlagSet() *flag.FlagSet {
	return command.Flags
}
