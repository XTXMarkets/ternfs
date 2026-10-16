// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/timing"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func receiveMetadata[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for metadata processor")
		var zero T
		return zero
	}
}

func testMetadataRequest(c *Client, logger *log.Logger, api, name string) <-chan error {
	done := make(chan error, 1)
	go func() {
		var id msgs.InodeId
		var err error
		if api == "cdc" {
			resp := msgs.MakeDirectoryResp{}
			err = c.CDCRequest(logger, &msgs.MakeDirectoryReq{OwnerId: msgs.ROOT_DIR_INODE_ID, Name: name}, &resp)
			id = resp.Id
		} else {
			resp := msgs.LookupResp{}
			err = c.ShardRequest(logger, 0, &msgs.LookupReq{DirId: msgs.ROOT_DIR_INODE_ID, Name: name}, &resp)
			id = resp.TargetId
		}
		if err == nil && id != msgs.ROOT_DIR_INODE_ID {
			err = fmt.Errorf("response inode = %v, want %v", id, msgs.ROOT_DIR_INODE_ID)
		}
		done <- err
	}()
	return done
}

type metadataSendTestSocket struct {
	*net.UDPConn
	send func([]byte, *net.UDPAddr) ([]byte, error)
}

func (s *metadataSendTestSocket) WriteToUDP(packet []byte, addr *net.UDPAddr) (int, error) {
	reply, err := s.send(packet, addr)
	if reply != nil {
		if _, replyErr := s.UDPConn.WriteToUDP(reply, s.LocalAddr().(*net.UDPAddr)); replyErr != nil {
			return 0, replyErr
		}
	}
	return len(packet), err
}

var metadataTestAddrs = msgs.AddrsInfo{Addr1: msgs.IpPort{Addrs: [4]byte{127, 0, 0, 1}, Port: 1}}

// Inject send failures while replies and timeouts use the real socket reader.
func metadataRetryClient(t *testing.T, timeouts timing.ReqTimeouts, addr msgs.AddrsInfo, send func([]byte) ([]byte, error)) (*Client, *log.Logger) {
	t.Helper()
	return metadataRoutingClient(t, timeouts, addr, func(packet []byte, _ *net.UDPAddr) ([]byte, error) {
		return send(packet)
	})
}

func metadataRoutingClient(t *testing.T, timeouts timing.ReqTimeouts, addr msgs.AddrsInfo, send func([]byte, *net.UDPAddr) ([]byte, error)) (*Client, *log.Logger) {
	t.Helper()
	sock, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	shardTimeout, cdcTimeout := timeouts, timeouts
	c := &Client{shardTimeout: &shardTimeout, cdcTimeout: &cdcTimeout}
	c.SetAddrs(addr, &[256]msgs.AddrsInfo{addr})
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.ERROR})
	c.clientMetadata.sock = &metadataSendTestSocket{UDPConn: sock, send: send}
	if err := c.clientMetadata.init(logger, c); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.clientMetadata.close)
	return c, logger
}

func metadataRetryReply(request []byte, result msgs.TernError) []byte {
	protocol := msgs.SHARD_RESP_PROTOCOL_VERSION
	var body bincode.Packable = &msgs.LookupResp{TargetId: msgs.ROOT_DIR_INODE_ID}
	if binary.LittleEndian.Uint32(request) == msgs.CDC_REQ_PROTOCOL_VERSION {
		protocol = msgs.CDC_RESP_PROTOCOL_VERSION
		body = &msgs.MakeDirectoryResp{Id: msgs.ROOT_DIR_INODE_ID}
	}
	packet := binary.LittleEndian.AppendUint32(nil, protocol)
	packet = append(packet, request[4:13]...)
	if result != 0 {
		packet[12] = msgs.ERROR
		return binary.LittleEndian.AppendUint16(packet, uint16(result))
	}
	data, err := bincode.Pack(body)
	if err != nil {
		panic(err)
	}
	return append(packet, data...)
}

func TestRetryableMetadataSendError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{errMetadataAddressUnknown, true},
		{syscall.EINTR, true}, {syscall.EAGAIN, true}, {syscall.ENOBUFS, true},
		{syscall.ENOMEM, true}, {syscall.ENETDOWN, true}, {syscall.ENETUNREACH, true},
		{syscall.EHOSTUNREACH, true}, {syscall.ECONNREFUSED, true}, {syscall.ECONNRESET, true},
		{syscall.ETIMEDOUT, true}, {syscall.EPERM, true},
		{syscall.EBADF, false}, {syscall.EINVAL, false}, {syscall.EMSGSIZE, false},
		{syscall.EACCES, false}, {net.ErrClosed, false}, {io.ErrUnexpectedEOF, false}, {nil, false},
	} {
		for _, err := range []error{tc.err, &net.OpError{Op: "write", Net: "udp", Err: &os.SyscallError{Syscall: "sendmsg", Err: tc.err}}} {
			if got := retryableMetadataSendError(err); got != tc.want {
				t.Errorf("retryableMetadataSendError(%v) = %v, want %v", err, got, tc.want)
			}
		}
	}
}

func TestMetadataRequestWaitsForAddresses(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		t.Run(api, func(t *testing.T) {
			c, logger := metadataRetryClient(t, DefaultShardTimeout, msgs.AddrsInfo{}, func(packet []byte) ([]byte, error) {
				return metadataRetryReply(packet, 0), nil
			})
			done := testMetadataRequest(c, logger, api, "file")
			select {
			case err := <-done:
				t.Fatalf("request finished before an address was known: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			c.SetAddrs(metadataTestAddrs, &[256]msgs.AddrsInfo{metadataTestAddrs})
			if err := receiveMetadata(t, done); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMetadataTransientSendRetries(t *testing.T) {
	for _, api := range []string{"shard", "cdc"} {
		t.Run(api, func(t *testing.T) {
			timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
			type attempt struct {
				id uint64
				at time.Time
			}
			sends := make(chan attempt, 4)
			first := true
			c, logger := metadataRetryClient(t, timeouts, metadataTestAddrs, func(packet []byte) ([]byte, error) {
				sends <- attempt{binary.LittleEndian.Uint64(packet[4:]), time.Now()}
				if first {
					first = false
					return nil, &net.OpError{Op: "write", Net: "udp", Err: syscall.EPERM}
				}
				return metadataRetryReply(packet, 0), nil
			})
			if err := receiveMetadata(t, testMetadataRequest(c, logger, api, "file")); err != nil {
				t.Fatal(err)
			}
			a, b := receiveMetadata(t, sends), receiveMetadata(t, sends)
			if a.id != b.id || b.at.Sub(a.at) < timeouts.Initial {
				t.Fatalf("retry changed request ID or skipped the timeout: %v, %v", a, b)
			}
		})
	}
}

func TestMetadataSendRetryExhaustion(t *testing.T) {
	timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, 100*time.Millisecond, 2, 0)
	c, logger := metadataRetryClient(t, timeouts, metadataTestAddrs, func([]byte) ([]byte, error) { return nil, syscall.ENOBUFS })
	if err := receiveMetadata(t, testMetadataRequest(c, logger, "shard", "file")); err != msgs.TIMEOUT {
		t.Fatalf("error = %v, want TIMEOUT", err)
	}
}

func TestMetadataLateReplyAfterSendFailure(t *testing.T) {
	for _, result := range []msgs.TernError{0, msgs.NOT_AUTHORISED} {
		t.Run(result.Error(), func(t *testing.T) {
			timeouts := *timing.NewReqTimeouts(10*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
			var original []byte
			c, logger := metadataRetryClient(t, timeouts, metadataTestAddrs, func(packet []byte) ([]byte, error) {
				if original == nil {
					original = append([]byte(nil), packet...)
					return nil, nil
				}
				// Deliver the first transmission's reply while the retry fails to send.
				return metadataRetryReply(original, result), syscall.ENETUNREACH
			})
			err := receiveMetadata(t, testMetadataRequest(c, logger, "shard", "file"))
			if result == 0 && err != nil || result != 0 && err != result {
				t.Fatalf("error = %v, want reply status %v", err, result)
			}
		})
	}
}

func TestMetadataDontWaitDoesNotRetrySendErrors(t *testing.T) {
	for _, sendErr := range []error{syscall.EPERM, syscall.EBADF} {
		t.Run(sendErr.Error(), func(t *testing.T) {
			kinds := make(chan uint8, 4)
			c, logger := metadataRetryClient(t, DefaultShardTimeout, metadataTestAddrs, func(packet []byte) ([]byte, error) {
				kinds <- packet[12]
				if packet[12] == uint8(msgs.SET_TIME) {
					return nil, sendErr
				}
				return metadataRetryReply(packet, 0), nil
			})
			if err := c.ShardRequestDontWait(logger, 0, &msgs.SetTimeReq{Id: msgs.ROOT_DIR_INODE_ID}); err != nil {
				t.Fatal(err)
			}
			if err := receiveMetadata(t, testMetadataRequest(c, logger, "shard", "healthy")); err != nil {
				t.Fatal(err)
			}
			if a, b := receiveMetadata(t, kinds), receiveMetadata(t, kinds); a != uint8(msgs.SET_TIME) || b != uint8(msgs.LOOKUP) {
				t.Fatalf("send kinds = %v, %v, want SET_TIME followed by LOOKUP", a, b)
			}
		})
	}
}

func TestMetadataSendRetryDoesNotBlockOtherRequests(t *testing.T) {
	firstSend, releaseSend := make(chan struct{}), make(chan struct{})
	timeouts := *timing.NewReqTimeouts(20*time.Millisecond, 100*time.Millisecond, time.Second, 2, 0)
	names := make(chan string, 4)
	first := true
	c, logger := metadataRetryClient(t, timeouts, metadataTestAddrs, func(packet []byte) ([]byte, error) {
		var req msgs.LookupReq
		if err := bincode.Unpack(packet[13:], &req); err != nil {
			return nil, err
		}
		names <- req.Name
		if first {
			first = false
			close(firstSend)
			receiveMetadata(t, releaseSend)
			return nil, syscall.EPERM
		}
		return metadataRetryReply(packet, 0), nil
	})
	retrying := testMetadataRequest(c, logger, "shard", "retry")
	receiveMetadata(t, firstSend)
	// Queue the healthy request before releasing the failed send.
	healthy := make(chan *metadataProcessorResponse, 1)
	c.clientMetadata.incoming <- &metadataProcessorRequest{
		requestId: c.newRequestId(), timeout: time.Second, shard: 0,
		req: &msgs.LookupReq{DirId: msgs.ROOT_DIR_INODE_ID, Name: "healthy"}, resp: &msgs.LookupResp{}, respCh: healthy,
	}
	close(releaseSend)
	if resp := receiveMetadata(t, healthy); resp.err != nil {
		t.Fatal(resp.err)
	}
	if err := receiveMetadata(t, retrying); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"retry", "healthy", "retry"} {
		if name := receiveMetadata(t, names); name != want {
			t.Fatalf("send name = %q, want %q", name, want)
		}
	}
}

type metadataPackErrorRequest struct {
	msgs.LookupReq
	err error
}

func (r *metadataPackErrorRequest) Pack(io.Writer) error { return r.err }

func TestMetadataPackingErrorIsNotRetried(t *testing.T) {
	want := &net.OpError{Op: "write", Net: "udp", Err: syscall.ENOBUFS}
	c, logger := metadataRetryClient(t, DefaultShardTimeout, metadataTestAddrs, func([]byte) ([]byte, error) {
		t.Error("attempted a send after packing failed")
		return nil, net.ErrClosed
	})
	done := make(chan error, 1)
	go func() { done <- c.ShardRequest(logger, 0, &metadataPackErrorRequest{err: want}, &msgs.LookupResp{}) }()
	if err := receiveMetadata(t, done); err != want {
		t.Fatalf("packing error = %v, want %v", err, want)
	}
}
