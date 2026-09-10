// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"os"
)

func NewRegistryReq() SpecWithClient {
	registryReqCmd := flag.NewFlagSet("registry-req", flag.ExitOnError)
	registryReqKind := registryReqCmd.String("kind", "", "")
	registryReqReq := registryReqCmd.String("req", "", "Request body, in JSON")
	registryReqYes := registryReqCmd.Bool("yes", false, "Do not ask for confirmation")
	registryReqRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		req, resp, err := msgs.MkRegistryMessage(*registryReqKind)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal([]byte(*registryReqReq), &req); err != nil {
			panic(fmt.Errorf("could not decode registry req: %w", err))
		}
		fmt.Printf("Will send this registry request: %T %+v\n", req, req)
		if !*registryReqYes {
			for {
				var action string
				fmt.Printf("Proceed? y/n ")
				fmt.Scanln(&action)
				if action == "y" {
					break
				}
				if action == "n" {
					fmt.Printf("BYE\n")
					os.Exit(0)
				}
			}
		}
		if resp, err = runtime.Client.RegistryRequest(l, req); err != nil {
			panic(err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			panic(fmt.Errorf("could not encode response %+v to json: %w", resp, err))
		}
		os.Stdout.Write(out)
		fmt.Println()
	}
	return SpecWithClient{
		Flags: registryReqCmd,
		Run:   registryReqRun,
	}
}
