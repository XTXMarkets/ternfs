// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cmd

import (
	"crypto/aes"
	"flag"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/certificate"
	"github.com/XTXMarkets/ternfs/go/core/crc32c"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"io/ioutil"
)

func NewWriteBlockReq() Spec {
	blockReqCmd := flag.NewFlagSet("write-block-req", flag.ExitOnError)
	blockReqBlockId := blockReqCmd.Uint64("b", 0, "Block id")
	blockReqBlockService := blockReqCmd.Uint64("bs", 0, "Block service")
	blockReqFile := blockReqCmd.String("file", "", "")
	blockReqRun := func(runtime Runtime) {
		l := runtime.Log
		registryAddress := runtime.RegistryAddress
		resp, err := client.RegistryRequest(l, nil, *registryAddress, &msgs.ChangedBlockServicesReq{})
		if err != nil {
			panic(err)
		}
		blockServices := resp.(*msgs.ChangedBlockServicesResp)
		var blockServiceInfo msgs.FullBlockServiceInfo
		for _, bsInfo := range blockServices.BlockServices {
			if bsInfo.Id == msgs.BlockServiceId(*blockReqBlockService) {
				blockServiceInfo = bsInfo
				break
			}
		}
		cipher, err := aes.NewCipher(blockServiceInfo.SecretKey[:])
		if err != nil {
			panic(err)
		}
		fileContents, err := ioutil.ReadFile(*blockReqFile)
		if err != nil {
			panic(err)
		}
		req := msgs.WriteBlockReq{
			BlockId: msgs.BlockId(*blockReqBlockId),
			Crc:     msgs.Crc(crc32c.Sum(0, fileContents)),
			Size:    uint32(len(fileContents)),
		}
		req.Certificate = certificate.BlockWriteCertificate(cipher, blockServiceInfo.Id, &req)
		l.Info("request: %+v", req)
	}
	return Spec{
		Flags: blockReqCmd,
		Run:   blockReqRun,
	}
}
