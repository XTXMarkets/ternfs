// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sync"
	"time"
)

// The proxy disconnects the NFS mount without interrupting nfsd's lease sweeper.
// It is used only by the lease-recovery workload.
func nfsOutageProxy(upstream string) (string, func(time.Duration), func(), error) {
	// Do not accept a mount's RPC connection until nfsd has finished creating
	// its persistent state. Accepting and then resetting it can fail a soft
	// NFS mount immediately, instead of letting it retry connection refusal.
	deadline := time.Now().Add(time.Minute)
	for {
		conn, err := net.DialTimeout("tcp", upstream, time.Second)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			return "", nil, nil, fmt.Errorf("waiting for nfsd: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, nil, err
	}
	type connection struct{ client, server net.Conn }
	var mu sync.Mutex
	connections := make(map[*connection]struct{})
	paused, stopped := false, false
	closeConnections := func() {
		for c := range connections {
			c.client.Close()
			c.server.Close()
		}
	}
	go func() {
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer client.Close()
				mu.Lock()
				reject := paused || stopped
				mu.Unlock()
				if reject {
					return
				}
				server, err := net.DialTimeout("tcp", upstream, time.Second)
				if err != nil {
					return
				}
				defer server.Close()
				c := &connection{client, server}
				mu.Lock()
				if paused || stopped {
					mu.Unlock()
					return
				}
				connections[c] = struct{}{}
				mu.Unlock()
				defer func() {
					mu.Lock()
					delete(connections, c)
					mu.Unlock()
				}()
				go func() {
					_, _ = io.Copy(server, client)
					client.Close()
					server.Close()
				}()
				_, _ = io.Copy(client, server)
			}()
		}
	}()
	outage := func(duration time.Duration) {
		mu.Lock()
		paused = true
		closeConnections()
		mu.Unlock()
		time.Sleep(duration)
		mu.Lock()
		paused = false
		mu.Unlock()
	}
	stop := func() {
		mu.Lock()
		stopped = true
		listener.Close()
		closeConnections()
		mu.Unlock()
	}
	return listener.Addr().String(), outage, stop, nil
}

func nfsLeaseRecoveryTest(mountPoint string, outage func(time.Duration)) {
	if outage == nil {
		panic("NFS lease recovery requires an outage proxy")
	}
	// Keep both a new creator and an existing-file writer open across one
	// outage. Neither file may fall back to its published contents.
	existingPath := path.Join(mountPoint, "lease-existing")
	if err := os.WriteFile(existingPath, []byte("old"), 0644); err != nil {
		panic(err)
	}
	files := make([]*os.File, 0, 2)
	for _, name := range []string{"lease-new", "lease-existing"} {
		f, err := os.OpenFile(path.Join(mountPoint, name), os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if _, err := f.Write([]byte("acknowledged")); err != nil {
			panic(err)
		}
		if err := f.Sync(); err != nil {
			panic(err)
		}
		files = append(files, f)
	}
	// The NFS lease is 90s. Stay below the test cluster's separate 120s
	// transient-file deadline, so this tests NFS recovery rather than backend
	// transient expiry. A WRITE checks the lease even between sweep ticks.
	fmt.Println("  nfs lease recovery: disconnecting for 100 seconds")
	outage(100 * time.Second)
	fmt.Println("  nfs lease recovery: reconnecting, appending, and publishing")
	for _, f := range files {
		if _, err := f.Write([]byte("-after")); err != nil {
			panic(fmt.Errorf("write after lease expiry: %w", err))
		}
		if err := f.Sync(); err != nil {
			panic(fmt.Errorf("sync after lease expiry: %w", err))
		}
		if err := f.Close(); err != nil {
			panic(fmt.Errorf("close after lease expiry: %w", err))
		}
		got, err := os.ReadFile(f.Name())
		if err != nil || string(got) != "acknowledged-after" {
			panic(fmt.Errorf("lease recovery data=%q err=%v", got, err))
		}
	}
	fmt.Println("  nfs lease recovery: acknowledged contents preserved")
}
