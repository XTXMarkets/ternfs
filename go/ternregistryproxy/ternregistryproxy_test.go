// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestLocalShardReplicas(t *testing.T) {
	for _, location := range []msgs.Location{0, 7} {
		t.Run(fmt.Sprint(location), func(t *testing.T) {
			logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
			listen := func() *net.TCPListener {
				listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { listener.Close() })
				listener.SetDeadline(time.Now().Add(5 * time.Second))
				return listener
			}
			upstream, proxy := listen(), listen()
			local := []msgs.FullShardInfo{
				{Id: msgs.MakeShardReplicaId(2, 0), LocationId: location},
				{Id: msgs.MakeShardReplicaId(2, 1), LocationId: location, IsLeader: true},
			}
			all := append([]msgs.FullShardInfo{
				{Id: local[0].Id, LocationId: location + 1, IsLeader: true},
			}, local...)
			upstreamDone := make(chan error, 1)
			go func() {
				conn, err := upstream.AcceptTCP()
				if err != nil {
					upstreamDone <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				req, err := readRegistryRequest(logger, conn)
				if err != nil {
					upstreamDone <- err
					return
				}
				if _, ok := req.(*msgs.AllShardsReq); !ok {
					upstreamDone <- fmt.Errorf("upstream request = %T, want AllShardsReq", req)
					return
				}
				upstreamDone <- writeRegistryResponse(logger, conn, &msgs.AllShardsResp{Shards: all})
			}()
			st := newState(logger, &registryProxyConfig{
				location: location, registryAddress: upstream.Addr().String(), numHandlers: 1,
			}, nil)
			defer st.registryConn.Close()
			proxyDone := make(chan error, 1)
			go func() {
				conn, err := proxy.AcceptTCP()
				if err != nil {
					proxyDone <- err
					return
				}
				handleRequest(logger, st, conn, time.Second)
				proxyDone <- nil
			}()
			timeouts := client.DefaultRegistryTimeout
			timeouts.RequestTimeout = 2 * time.Second
			resp, err := client.RegistryRequest(logger, &timeouts, proxy.Addr().String(), &msgs.LocalShardReplicasReq{})
			if err != nil {
				t.Fatal(err)
			}
			if got := resp.(*msgs.LocalShardReplicasResp).Shards; !reflect.DeepEqual(got, local) {
				t.Fatalf("local replicas = %+v, want %+v", got, local)
			}
			for _, done := range []chan error{upstreamDone, proxyDone} {
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("registry handler did not finish")
				}
			}
		})
	}
}
