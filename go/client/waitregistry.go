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

func WaitForBlockServices(ll *log.Logger, registryAddress string, expectedBlockServices int, timeout time.Duration) ([]msgs.BlockServiceDeprecatedInfo, error) {
	var err error
	t0 := time.Now()
	for {
		if time.Since(t0) > timeout {
			return nil, fmt.Errorf("giving up waiting for block services, last error: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
		var resp msgs.RegistryResponse
		var bss []msgs.BlockServiceDeprecatedInfo
		resp, err = RegistryRequest(ll, nil, registryAddress, &msgs.AllBlockServicesDeprecatedReq{})
		if err != nil {
			ll.Debug("got error while getting block services from registry, will keep waiting: %v", err)
			continue
		}
		bss = resp.(*msgs.AllBlockServicesDeprecatedResp).BlockServices
		if len(bss) >= expectedBlockServices {
			return bss, nil
		}
		err = fmt.Errorf("not all block services are up yet, will keep waiting")
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

func WaitForRegistryReplicas(ll *log.Logger, registryAddress string, minReplicas uint8, location msgs.Location, timeout time.Duration) ([]msgs.FullRegistryInfo, error) {
	registryTimeout := &RegistryTimeouts{
		ReconnectTimeout: DefaultRegistryTimeout.ReconnectTimeout,
		RequestTimeout:   timeout,
	}
	resp, err := RegistryRequest(ll, registryTimeout, registryAddress, &msgs.AllRegistryReplicasReq{
		MinKnownReplicas: minReplicas,
		Location:         location,
	})
	if err != nil {
		return nil, fmt.Errorf("failed waiting for registry replicas: %w", err)
	}
	return resp.(*msgs.AllRegistryReplicasResp).Replicas, nil
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
		if client == nil {
			client, err = NewClient(log, nil, registryAddress, msgs.AddrsInfo{})
			if err != nil {
				log.Info("getting registry client failed, waiting: %v", err)
				time.Sleep(time.Second)
				continue
			}
		}
		if !client.IsReady() {
			log.Info("client not ready, waiting")
			time.Sleep(time.Second)
			continue
		}
		client.Close()
		break
	}
}
