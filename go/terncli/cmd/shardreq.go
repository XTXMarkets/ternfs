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

func NewShardReq() Command {
	shardReqCmd := flag.NewFlagSet("shard-req", flag.ExitOnError)
	shardReqShard := shardReqCmd.Uint("shard", 0, "Shard to send the request to (0..255)")
	shardReqReplica := shardReqCmd.Int("replica", -1, "Replica to send the request to (0..4, default: current leader)")
	shardReqKind := shardReqCmd.String("kind", "", "")
	shardReqReq := shardReqCmd.String("req", "", "Request body, in JSON")
	shardReqYes := shardReqCmd.Bool("yes", false, "Do not ask for confirmation")
	shardReqRun := func(runtime *Runtime) {
		l := runtime.Log
		if *shardReqShard > 255 {
			panic(fmt.Errorf("shard must be in 0..255"))
		}
		if *shardReqReplica < -1 || *shardReqReplica > 4 {
			panic(fmt.Errorf("replica must be in 0..4, or -1 for the current leader"))
		}
		req, resp, err := msgs.MkShardMessage(*shardReqKind)
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal([]byte(*shardReqReq), &req); err != nil {
			panic(fmt.Errorf("could not decode shard req: %w", err))
		}
		shard := msgs.ShardId(*shardReqShard)
		if *shardReqReplica < 0 {
			fmt.Printf("Will send this request to shard %v: %T %+v\n", shard, req, req)
		} else {
			fmt.Printf("Will send this request to shard %v replica %v: %T %+v\n", shard, *shardReqReplica, req, req)
		}
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
		if *shardReqReplica < 0 {
			err = runtime.getClient().ShardRequest(l, shard, req, resp)
		} else {
			err = runtime.getClient().ShardReplicaRequest(l, msgs.MakeShardReplicaId(shard, msgs.ReplicaId(*shardReqReplica)), req, resp)
		}
		if err != nil {
			panic(err)
		}
		out, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			panic(fmt.Errorf("could not encode response %+v to json: %w", resp, err))
		}
		os.Stdout.Write(out)
		fmt.Println()
	}
	return Command{
		Flags: shardReqCmd,
		Run:   shardReqRun,
	}
}
