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
	RegistryAddress *string
}

// RuntimeWithClient is provided only to commands which require the full
// TernFS client.
type RuntimeWithClient struct {
	Runtime
	Client *client.Client
}

// Command is the common interface used by terncli's dispatcher.
type Command interface {
	FlagSet() *flag.FlagSet
	NeedsClient() bool
	Execute(Runtime, *client.Client)
}

// Spec describes a command which does not require a TernFS client.
type Spec struct {
	Flags *flag.FlagSet
	Run   func(Runtime)
}

func (spec Spec) FlagSet() *flag.FlagSet {
	return spec.Flags
}

func (Spec) NeedsClient() bool {
	return false
}

func (spec Spec) Execute(runtime Runtime, _ *client.Client) {
	spec.Run(runtime)
}

// SpecWithClient describes a command which requires a TernFS client.
type SpecWithClient struct {
	Flags *flag.FlagSet
	Run   func(RuntimeWithClient)
}

func (spec SpecWithClient) FlagSet() *flag.FlagSet {
	return spec.Flags
}

func (SpecWithClient) NeedsClient() bool {
	return true
}

func (spec SpecWithClient) Execute(
	runtime Runtime,
	ternClient *client.Client,
) {
	if ternClient == nil {
		panic("client command executed without a client")
	}
	spec.Run(RuntimeWithClient{
		Runtime: runtime,
		Client:  ternClient,
	})
}
