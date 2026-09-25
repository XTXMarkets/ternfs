// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

type metadataSendTestSocket struct {
	*net.UDPConn
	send func([]byte, *net.UDPAddr) (int, error)
}

func (s *metadataSendTestSocket) WriteToUDP(packet []byte, addr *net.UDPAddr) (int, error) {
	return s.send(packet, addr)
}

func (*metadataSendTestSocket) Close() error { return nil }

// Use the real send and response processors, with injected socket writes.
// Empty receive events drive the same expiry checks as socket read timeouts.
func metadataRetryClient(t *testing.T, timeouts timing.ReqTimeouts, send func(*clientMetadata, []byte) (int, error)) (*Client, *log.Logger) {
	t.Helper()
	shardTimeout, cdcTimeout := timeouts, timeouts
	c := &Client{shardTimeout: &shardTimeout, cdcTimeout: &cdcTimeout}
	cm := &c.clientMetadata
	*cm = clientMetadata{
		client: c, incoming: make(chan *metadataProcessorRequest, 16),
		inFlight:              make(chan *metadataProcessorRequest, 16),
		rawResponses:          make(chan rawMetadataResponse, 16),
		responsesBufs:         make(chan *[]byte, 128),
		requestsById:          make(map[uint64]*metadataProcessorRequest),
		earlyRequests:         make(map[uint64]rawMetadataResponse),
		quitResponseProcessor: make(chan struct{}),
	}
	cm.sock = &metadataSendTestSocket{send: func(packet []byte, _ *net.UDPAddr) (int, error) {
		return send(cm, packet)
	}}
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	sendDone, responsesDone, timerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	stopTimer := make(chan struct{})
	go func() {
		defer close(sendDone)
		cm.processRequests(logger)
	}()
	go func() {
		defer close(responsesDone)
		cm.processResponses(logger)
	}()
	go func() {
		defer close(timerDone)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case cm.rawResponses <- rawMetadataResponse{}:
				case <-stopTimer:
					return
				}
			case <-stopTimer:
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(cm.incoming)
		<-sendDone
		close(stopTimer)
		<-timerDone
		close(cm.quitResponseProcessor)
		<-responsesDone
	})
	return c, logger
}

func metadataRetryReply(cm *clientMetadata, request []byte, result msgs.TernError) {
	protocol := msgs.SHARD_RESP_PROTOCOL_VERSION
	var body bincode.Packable = &msgs.LookupResp{TargetId: msgs.ROOT_DIR_INODE_ID}
	if binary.LittleEndian.Uint32(request) == msgs.CDC_REQ_PROTOCOL_VERSION {
		protocol = msgs.CDC_RESP_PROTOCOL_VERSION
		body = &msgs.MakeDirectoryResp{Id: msgs.ROOT_DIR_INODE_ID}
	}
	packet := make([]byte, 13)
	binary.LittleEndian.PutUint32(packet, protocol)
	copy(packet[4:13], request[4:13])
	if result != 0 {
		packet[12] = msgs.ERROR
		packet = binary.LittleEndian.AppendUint16(packet, uint16(result))
	} else {
		data, err := bincode.Pack(body)
		if err != nil {
			panic(err)
		}
		packet = append(packet, data...)
	}
	cm.rawResponses <- rawMetadataResponse{
		receivedAt: time.Now(), protocol: protocol,
		requestId: binary.LittleEndian.Uint64(packet[4:]),
		kind:      packet[12], respLen: len(packet), buf: &packet,
	}
}

func TestMetadataTransientSendRetries(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		for _, errno := range []syscall.Errno{
			syscall.EINTR, syscall.EAGAIN, syscall.ENOBUFS, syscall.ENOMEM,
			syscall.ENETDOWN, syscall.ENETUNREACH, syscall.EHOSTUNREACH,
			syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ETIMEDOUT, syscall.EPERM,
		} {
			t.Run(api+"/"+errno.Error(), func(t *testing.T) {
				timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
				var mu sync.Mutex
				var ids []uint64
				var sentAt []time.Time
				c, logger := metadataRetryClient(t, timeouts, func(cm *clientMetadata, packet []byte) (int, error) {
					mu.Lock()
					defer mu.Unlock()
					ids = append(ids, binary.LittleEndian.Uint64(packet[4:]))
					sentAt = append(sentAt, time.Now())
					if len(ids) == 1 {
						return 0, &net.OpError{Op: "write", Net: "udp", Err: &os.SyscallError{Syscall: "sendmsg", Err: errno}}
					}
					metadataRetryReply(cm, packet, 0)
					return len(packet), nil
				})
				if err := testMetadataRequest(c, logger, api, "file"); err != nil {
					t.Fatal(err)
				}
				mu.Lock()
				defer mu.Unlock()
				if len(ids) != 2 || ids[0] != ids[1] {
					t.Fatalf("request IDs = %v, want two attempts with the same ID", ids)
				}
				if sentAt[1].Sub(sentAt[0]) < timeouts.Initial/2 {
					t.Fatalf("send retried without waiting: %v", sentAt[1].Sub(sentAt[0]))
				}
			})
		}
	}
}

func TestMetadataPermanentSendErrors(t *testing.T) {
	for _, cause := range []error{net.ErrClosed, syscall.EBADF, syscall.EINVAL, syscall.EMSGSIZE, syscall.EACCES} {
		t.Run(cause.Error(), func(t *testing.T) {
			want := &net.OpError{Op: "write", Net: "udp", Err: cause}
			c, logger := metadataRetryClient(t, DefaultShardTimeout, func(_ *clientMetadata, _ []byte) (int, error) {
				return 0, want
			})
			if err := testMetadataRequest(c, logger, "shard", "file"); err != want {
				t.Fatalf("error = %v, want original error %v", err, want)
			}
		})
	}
}

func TestMetadataSendRetryDeadline(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		t.Run(api, func(t *testing.T) {
			// The attempt timeout must be capped by the shorter overall budget.
			timeouts := *timing.NewReqTimeouts(2*time.Second, 2*time.Second, 40*time.Millisecond, 2, 0)
			c, logger := metadataRetryClient(t, timeouts, func(_ *clientMetadata, _ []byte) (int, error) {
				return 0, &net.OpError{Op: "write", Net: "udp", Err: syscall.ENOBUFS}
			})
			started := time.Now()
			if err := testMetadataRequest(c, logger, api, "file"); err != msgs.TIMEOUT {
				t.Fatalf("error = %v, want TIMEOUT", err)
			}
			if elapsed := time.Since(started); elapsed >= timeouts.Initial/2 {
				t.Fatalf("overall retry budget was exceeded: %v", elapsed)
			}
		})
	}
}

func TestMetadataLateReplyAfterSendFailure(t *testing.T) {
	for _, result := range []msgs.TernError{0, msgs.NOT_AUTHORISED} {
		t.Run(result.Error(), func(t *testing.T) {
			timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
			var mu sync.Mutex
			var original []byte
			attempts := 0
			c, logger := metadataRetryClient(t, timeouts, func(cm *clientMetadata, packet []byte) (int, error) {
				mu.Lock()
				defer mu.Unlock()
				attempts++
				if attempts == 1 {
					// The first send succeeds, but its reply is delayed.
					original = append([]byte(nil), packet...)
					return len(packet), nil
				}
				// The retransmission fails to send while the first reply arrives.
				metadataRetryReply(cm, original, result)
				return 0, &net.OpError{Op: "write", Net: "udp", Err: syscall.ENETUNREACH}
			})
			err := testMetadataRequest(c, logger, "shard", "file")
			if result == 0 && err != nil || result != 0 && err != result {
				t.Fatalf("error = %v, want reply status %v", err, result)
			}
			mu.Lock()
			defer mu.Unlock()
			if attempts != 2 {
				t.Fatalf("sent %d attempts, want 2", attempts)
			}
		})
	}
}

func TestMetadataDontWaitDoesNotRetrySendErrors(t *testing.T) {
	var mu sync.Mutex
	var kinds []uint8
	c, logger := metadataRetryClient(t, DefaultShardTimeout, func(cm *clientMetadata, packet []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		kinds = append(kinds, packet[12])
		if len(kinds) == 1 {
			return 0, &net.OpError{Op: "write", Net: "udp", Err: syscall.EPERM}
		}
		metadataRetryReply(cm, packet, 0)
		return len(packet), nil
	})
	if err := c.ShardRequestDontWait(logger, 0, &msgs.SetTimeReq{Id: msgs.ROOT_DIR_INODE_ID}); err != nil {
		t.Fatal(err)
	}
	if err := testMetadataRequest(c, logger, "shard", "healthy"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(kinds) != 2 || kinds[0] != uint8(msgs.SET_TIME) || kinds[1] != uint8(msgs.LOOKUP) {
		t.Fatalf("send kinds = %v, want SET_TIME followed by LOOKUP", kinds)
	}
}

func TestMetadataSendRetryDoesNotBlockOtherRequests(t *testing.T) {
	firstSend, releaseSend := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var names []string
	timeouts := *timing.NewReqTimeouts(20*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
	c, logger := metadataRetryClient(t, timeouts, func(cm *clientMetadata, packet []byte) (int, error) {
		var req msgs.LookupReq
		if err := bincode.Unpack(packet[13:], &req); err != nil {
			return 0, err
		}
		mu.Lock()
		names = append(names, req.Name)
		first := len(names) == 1
		mu.Unlock()
		if first {
			close(firstSend)
			<-releaseSend
			return 0, &net.OpError{Op: "write", Net: "udp", Err: syscall.EPERM}
		}
		metadataRetryReply(cm, packet, 0)
		return len(packet), nil
	})
	result := make(chan error, 1)
	go func() { result <- testMetadataRequest(c, logger, "shard", "retry") }()
	<-firstSend
	// Queue the healthy request before releasing the failed send, so the
	// required processing order does not depend on goroutine scheduling.
	healthy := make(chan *metadataProcessorResponse, 1)
	c.clientMetadata.incoming <- &metadataProcessorRequest{
		requestId: c.newRequestId(), timeout: time.Second, shard: 0,
		req:  &msgs.LookupReq{DirId: msgs.ROOT_DIR_INODE_ID, Name: "healthy"},
		resp: &msgs.LookupResp{}, respCh: healthy,
	}
	close(releaseSend)
	if resp := <-healthy; resp.err != nil {
		t.Fatal(resp.err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(names) != 3 || names[0] != "retry" || names[1] != "healthy" || names[2] != "retry" {
		t.Fatalf("send order = %v, want [retry healthy retry]", names)
	}
}

type metadataPackErrorRequest struct {
	msgs.LookupReq
	err error
}

func (r *metadataPackErrorRequest) Pack(io.Writer) error { return r.err }

func TestMetadataPackingErrorIsNotRetried(t *testing.T) {
	want := &net.OpError{Op: "write", Net: "udp", Err: syscall.ENOBUFS}
	c, logger := metadataRetryClient(t, DefaultShardTimeout, func(_ *clientMetadata, _ []byte) (int, error) {
		t.Error("attempted a send after packing failed")
		return 0, net.ErrClosed
	})
	err := c.ShardRequest(logger, 0, &metadataPackErrorRequest{err: want}, &msgs.LookupResp{})
	if !errors.Is(err, want) {
		t.Fatalf("packing error = %v, want %v", err, want)
	}
}
