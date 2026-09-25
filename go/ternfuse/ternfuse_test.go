// Copyright 2026 XTX Markets Technologies Limited
// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestCDCRequestReturnsIO(t *testing.T) {
	const childEnv = "TERNFS_TEST_CDC_REQUEST_ERROR"
	failure := os.Getenv(childEnv)
	if failure == "" {
		// An unhandled error in a FUSE callback must not terminate the process.
		for _, failure := range []string{"pack", "unpack"} {
			t.Run(failure, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCDCRequestReturnsIO$", "-test.timeout=5s")
				cmd.Env = append(os.Environ(), childEnv+"="+failure)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("CDC %s failure: %v\n%s", failure, err, output)
				}
			})
		}
		return
	}

	logger = log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	var err error
	c, err = client.NewClientDirectNoAddrs(logger, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	c.SetCDCTimeouts(timing.NewReqTimeouts(100*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0))
	name := "dir"
	switch failure {
	case "pack":
		name = strings.Repeat("x", 256)
	case "unpack":
		sock, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
		if err != nil {
			t.Fatal(err)
		}
		defer sock.Close()
		c.SetAddrs(msgs.AddrsInfo{Addr1: msgs.IpPort{
			Addrs: [4]byte{127, 0, 0, 1}, Port: uint16(sock.LocalAddr().(*net.UDPAddr).Port),
		}}, &[256]msgs.AddrsInfo{})
		sock.SetReadDeadline(time.Now().Add(2 * time.Second))
		replied := make(chan error, 1)
		go func() {
			request := make([]byte, 1024)
			n, addr, err := sock.ReadFromUDP(request)
			if err != nil {
				replied <- err
				return
			}
			if n < 13 {
				replied <- syscall.EINVAL
				return
			}
			// Echo the request ID and kind, with a truncated response body.
			response := make([]byte, 14)
			binary.LittleEndian.PutUint32(response, msgs.CDC_RESP_PROTOCOL_VERSION)
			copy(response[4:13], request[4:13])
			_, err = sock.WriteToUDP(response, addr)
			replied <- err
		}()
		defer func() {
			if err := <-replied; err != nil {
				t.Errorf("CDC test server: %v", err)
			}
		}()
	default:
		t.Fatalf("unknown CDC failure %q", failure)
	}

	errno := cdcRequest(&msgs.MakeDirectoryReq{
		OwnerId: msgs.ROOT_DIR_INODE_ID, Name: name,
	}, &msgs.MakeDirectoryResp{})
	if errno != syscall.EIO {
		t.Fatalf("cdcRequest = %v, want EIO", errno)
	}
}
