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

func NewShardReq() SpecWithClient {
	shardReqCmd := flag.NewFlagSet("shard-req", flag.ExitOnError)
	shardReqShard := shardReqCmd.Uint("shard", 0, "Shard to send the req too")
	shardReqKind := shardReqCmd.String("kind", "", "")
	shardReqReq := shardReqCmd.String("req", "", "Request body, in JSON")
	shardReqYes := shardReqCmd.Bool("yes", false, "Do not ask for confirmation")
	shardReqRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		req, resp, err := msgs.MkShardMessage(*shardReqKind)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal([]byte(*shardReqReq), &req); err != nil {
			panic(fmt.Errorf("could not decode shard req: %w", err))
		}
		shard := msgs.ShardId(*shardReqShard)
		fmt.Printf("Will send this request to shard %v: %T %+v\n", shard, req, req)
		if !*shardReqYes {
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
		if err := runtime.Client.ShardRequest(l, shard, req, resp); err != nil {
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
		Flags: shardReqCmd,
		Run:   shardReqRun,
	}
}
