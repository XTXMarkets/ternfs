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

func NewSetDirInfo() SpecWithClient {
	setDirInfoCmd := flag.NewFlagSet("set-dir-info", flag.ExitOnError)
	setDirInfoIdU64 := setDirInfoCmd.Uint64("id", 0, "InodeId for the directory to set the policy of.")
	setDirInfoIdTag := setDirInfoCmd.String("tag", "", "One of SNAPSHOT|SPAN|BLOCK|STRIPE")
	setDirInfoPolicy := setDirInfoCmd.String("body", "", "Policy, in JSON")
	setDirInfoRun := func(runtime RuntimeWithClient) {
		l := runtime.Log
		entry := msgs.TagToDirInfoEntry(msgs.DirInfoTagFromName(*setDirInfoIdTag))
		if err := json.Unmarshal([]byte(*setDirInfoPolicy), entry); err != nil {
			panic(fmt.Errorf("could not decode directory info: %w", err))
		}
		id := msgs.InodeId(*setDirInfoIdU64)
		fmt.Printf("Will set dir info %v to directory %v:\n", entry.Tag(), id)
		fmt.Printf("%+v\n", entry)
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
		if err := runtime.Client.MergeDirectoryInfo(l, id, entry); err != nil {
			panic(err)
		}
	}
	return SpecWithClient{
		Flags: setDirInfoCmd,
		Run:   setDirInfoRun,
	}
}
