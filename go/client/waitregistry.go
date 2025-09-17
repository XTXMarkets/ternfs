// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"fmt"
	"time"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

func WaitForBlockServices(ll *log.Logger, registryAddress string, expectedBlockServices int, waitCurrentServicesCalcuation bool, timeout time.Duration) []msgs.BlockServiceDeprecatedInfo {
	var err error
	for {
		var resp msgs.RegistryResponse
		var bss []msgs.BlockServiceDeprecatedInfo
		resp, err = RegistryRequest(ll, nil, registryAddress, &msgs.AllBlockServicesDeprecatedReq{})
		if err != nil {
			ll.Debug("got error while getting block services from registry, will keep waiting: %v", err)
			goto KeepChecking
		}
		bss = resp.(*msgs.AllBlockServicesDeprecatedResp).BlockServices
		if len(bss) < expectedBlockServices {
			err = fmt.Errorf("not all block services are up yet, will keep waiting")
			ll.Debug("%v", err)
			goto KeepChecking
		}

		if waitCurrentServicesCalcuation {
			resp, err = RegistryRequest(ll, nil, registryAddress, &msgs.ShardBlockServicesDEPRECATEDReq{0})
			if err != nil || len(resp.(*msgs.ShardBlockServicesDEPRECATEDResp).BlockServices) == 0 {
				ll.Debug("current block services not calculated, will keep waiting")
				goto KeepChecking
			}
		}
		return bss
	KeepChecking:
		time.Sleep(10 * time.Millisecond)
	}
}

func WaitForRegistry(ll *log.Logger, registryAddress string, timeout time.Duration) error {
	t0 := time.Now()
	for {
		_, err := RegistryRequest(ll, nil, registryAddress, &msgs.InfoReq{})
		if err == nil {
			return nil
		}
		if time.Since(t0) > timeout {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// getting a client implies having all shards and cdc.
func WaitForClient(log *log.Logger, registryAddress string, timeout time.Duration) {
	t0 := time.Now()
	var err error
	var client *Client
	for {
		t := time.Now()
		if t.Sub(t0) > timeout {
			panic(fmt.Errorf("giving up waiting for client, last error: %w", err))
		}
		client, err = NewClient(log, nil, registryAddress, msgs.AddrsInfo{})
		if err != nil {
			log.Info("getting registry client failed, waiting: %v", err)
			time.Sleep(time.Second)
			continue
		}
		client.Close()
		break
	}
}
