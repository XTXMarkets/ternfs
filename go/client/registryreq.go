// Copyright 2025 XTX Markets Technologies Limited
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
	"time"
	"xtx/ternfs/core/bincode"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/msgs"
)

func writeRegistryRequest(log *log.Logger, w io.Writer, req msgs.RegistryRequest) error {
	log.Debug("sending request %v to registry", req.RegistryRequestKind())
	// serialize
	bytes := bincode.Pack(req)
	// write out
	if err := binary.Write(w, binary.LittleEndian, msgs.REGISTRY_REQ_PROTOCOL_VERSION); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(1+len(bytes))); err != nil {
		return err
	}
	if _, err := w.Write([]byte{uint8(req.RegistryRequestKind())}); err != nil {
		return err
	}
	if _, err := w.Write(bytes); err != nil {
		return err
	}
	return nil
}

func readRegistryResponse(
	log *log.Logger,
	r io.Reader,
) (msgs.RegistryResponse, error) {
	log.Debug("reading response from registry")
	var protocol uint32
	if err := binary.Read(r, binary.LittleEndian, &protocol); err != nil {
		return nil, fmt.Errorf("could not read protocol: %w", err)
	}
	if protocol != msgs.REGISTRY_RESP_PROTOCOL_VERSION {
		return nil, fmt.Errorf("bad registry protocol, expected %v, got %v", msgs.REGISTRY_RESP_PROTOCOL_VERSION, protocol)
	}
	var len uint32
	if err := binary.Read(r, binary.LittleEndian, &len); err != nil {
		return nil, fmt.Errorf("could not read len: %w", err)
	}
	data := make([]byte, len)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	if data[0] == msgs.ERROR {
		var err uint16
		if err := binary.Read(r, binary.LittleEndian, &err); err != nil {
			return nil, fmt.Errorf("could not read error: %w", err)
		}
		return nil, msgs.TernError(err)
	}
	kind := msgs.RegistryMessageKind(data[0])
	var resp msgs.RegistryResponse
	switch kind {
	case msgs.LOCAL_SHARDS:
		resp = &msgs.LocalShardsResp{}
	case msgs.REGISTER_SHARD:
		resp = &msgs.RegisterShardResp{}
	case msgs.ALL_BLOCK_SERVICES_DEPRECATED:
		resp = &msgs.AllBlockServicesDeprecatedResp{}
	case msgs.SET_BLOCK_SERVICE_FLAGS:
		resp = &msgs.SetBlockServiceFlagsResp{}
	case msgs.DECOMMISSION_BLOCK_SERVICE:
		resp = &msgs.DecommissionBlockServiceResp{}
	case msgs.REGISTER_CDC:
		resp = &msgs.RegisterCdcResp{}
	case msgs.LOCAL_CDC:
		resp = &msgs.LocalCdcResp{}
	case msgs.CDC_REPLICAS_DE_PR_EC_AT_ED:
		resp = &msgs.CdcReplicasDEPRECATEDResp{}
	case msgs.INFO:
		resp = &msgs.InfoResp{}
	case msgs.SHARD_BLOCK_SERVICES_DE_PR_EC_AT_ED:
		resp = &msgs.ShardBlockServicesDEPRECATEDResp{}
	case msgs.ERASE_DECOMMISSIONED_BLOCK:
		resp = &msgs.EraseDecommissionedBlockResp{}
	case msgs.ALL_CDC:
		resp = &msgs.AllCdcResp{}
	case msgs.CLEAR_CDC_INFO:
		resp = &msgs.ClearCdcInfoResp{}
	case msgs.CLEAR_SHARD_INFO:
		resp = &msgs.ClearShardInfoResp{}
	case msgs.CREATE_LOCATION:
		resp = &msgs.CreateLocationResp{}
	case msgs.LOCAL_CHANGED_BLOCK_SERVICES:
		resp = &msgs.LocalChangedBlockServicesResp{}
	case msgs.LOCATIONS:
		resp = &msgs.LocationsResp{}
	case msgs.MOVE_CDC_LEADER:
		resp = &msgs.MoveCdcLeaderResp{}
	case msgs.MOVE_SHARD_LEADER:
		resp = &msgs.MoveShardLeaderResp{}
	case msgs.RENAME_LOCATION:
		resp = &msgs.RenameLocationResp{}
	case msgs.ALL_SHARDS:
		resp = &msgs.AllShardsResp{}
	case msgs.CDC_AT_LOCATION:
		resp = &msgs.CdcAtLocationResp{}
	case msgs.CHANGED_BLOCK_SERVICES_AT_LOCATION:
		resp = &msgs.ChangedBlockServicesAtLocationResp{}
	case msgs.REGISTER_BLOCK_SERVICES:
		resp = &msgs.RegisterBlockServicesResp{}
	case msgs.SHARDS_AT_LOCATION:
		resp = &msgs.ShardsAtLocationResp{}
	case msgs.SHARD_BLOCK_SERVICES:
		resp = &msgs.ShardBlockServicesResp{}
	case msgs.UPDATE_BLOCK_SERVICE_PATH:
		resp = &msgs.UpdateBlockServicePathResp{}
	default:
		return nil, fmt.Errorf("bad registry response kind %v", kind)
	}
	log.Debug("read response %v from registry", kind)
	if err := bincode.Unpack(data[1:], resp); err != nil {
		return nil, err
	}
	return resp, nil
}

type RegistryTimeouts struct {
	ReconnectTimeout timing.ReqTimeouts
	RequestTimeout   time.Duration
}

var DefaultRegistryTimeout = RegistryTimeouts{
	ReconnectTimeout: timing.ReqTimeouts{
		Initial: 100 * time.Millisecond,
		Max:     1 * time.Second,
		Overall: 10 * time.Second,
		Growth:  1.5,
		Jitter:  0.1,
	},
	RequestTimeout: 20 * time.Second,
}

func RegistryRequest(
	log *log.Logger,
	timeout *RegistryTimeouts,
	registryAddress string,
	req msgs.RegistryRequest,
) (msgs.RegistryResponse, error) {
	conn := MakeRegistryConn(log, timeout, registryAddress, 1)
	defer conn.Close()
	return conn.Request(req)
}

type shuckResp struct {
	resp msgs.RegistryResponse
	err  error
}

type shuckReq struct {
	req       msgs.RegistryRequest
	respC     chan<- shuckResp
	startedAt time.Time
}

type RegistryConn struct {
	log            *log.Logger
	timeout        RegistryTimeouts
	registryAddress string
	reqChan        chan shuckReq
	numHandlers    uint
}

func MakeRegistryConn(
	log *log.Logger,
	timeout *RegistryTimeouts,
	registryAddress string,
	numHandlers uint,
) *RegistryConn {
	if timeout == nil {
		timeout = &DefaultRegistryTimeout
	}
	shuckConn := RegistryConn{log, *timeout, registryAddress, make(chan shuckReq), 0}
	shuckConn.IncreaseNumHandlersTo(numHandlers)
	return &shuckConn
}

func (c *RegistryConn) IncreaseNumHandlersTo(numHandlers uint) {
	for i := c.numHandlers; i < numHandlers; i++ {
		go func() {
			c.requestHandler()
		}()
	}
	c.numHandlers = numHandlers
}

func (c *RegistryConn) requestHandler() {
	var conn net.Conn
	var err error
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	for {
		req, ok := <-c.reqChan
		if !ok {
			return
		}
		reqDeadline := req.startedAt.Add(c.timeout.RequestTimeout)

		if time.Now().After(reqDeadline) {
			req.respC <- shuckResp{nil, msgs.TIMEOUT}
			continue
		}

		connectAttempted := false
	Reconnect:
		if conn == nil && !connectAttempted {

			connectAttempted = true
			if conn, err = c.connect(); err != nil {
				req.respC <- shuckResp{nil, err}
				continue
			}
		}
		if conn == nil {
			req.respC <- shuckResp{nil, msgs.INTERNAL_ERROR}
			continue
		}
		conn.SetWriteDeadline(reqDeadline)
		if err = writeRegistryRequest(c.log, conn, req.req); err != nil {
			conn.Close()
			conn = nil
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				req.respC <- shuckResp{nil, msgs.TIMEOUT}
				continue
			}
			goto Reconnect
		}
		conn.SetWriteDeadline(time.Time{})
		conn.SetReadDeadline(reqDeadline)
		resp, err := readRegistryResponse(c.log, conn)
		if err != nil {
			if _, isTernErr := err.(msgs.TernError); !isTernErr {
				conn.Close()
				conn = nil
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					err = msgs.TIMEOUT
				}
			}
		}
		if conn != nil {
			conn.SetReadDeadline(time.Time{})
		}
		req.respC <- shuckResp{resp, err}
	}
}

func (c *RegistryConn) connect() (net.Conn, error) {
	start := time.Now()

	var err error
	var conn net.Conn
	var delay time.Duration

	goto ReconnectBegin

Reconnect:
	delay = c.timeout.ReconnectTimeout.Next(start)
	if delay == 0 {
		c.log.Info("could not connect to registry and we're out of attempts: %v", err)
		return nil, err
	}
	time.Sleep(delay)

ReconnectBegin:
	conn, err = net.DialTimeout("tcp", c.registryAddress, c.timeout.ReconnectTimeout.Max)
	if err != nil {
		if opErr, ok := err.(*net.OpError); ok {
			if syscallErr, ok := opErr.Err.(*os.SyscallError); ok {
				if errno, ok := syscallErr.Err.(syscall.Errno); ok && (errno == syscall.ECONNREFUSED || errno == syscall.ETIMEDOUT) {
					goto Reconnect
				}
			}
			if opErr.Timeout() {
				goto Reconnect
			}
			if dnsErr, ok := opErr.Err.(*net.DNSError); ok && (dnsErr.IsTemporary || dnsErr.IsTimeout) {
				goto Reconnect
			}
		}
		c.log.Info("could not connect to registry: %v", err)
		return nil, err
	}
	tcpConn := conn.(*net.TCPConn)
	if err = tcpConn.SetKeepAlive(true); err != nil {
		c.log.RaiseAlert("could not set keepalive on connection to registry: %v", err)
		tcpConn.Close()
		return nil, err
	}
	keepAlivePeriod := c.timeout.ReconnectTimeout.Overall
	if keepAlivePeriod == 0 {
		keepAlivePeriod = c.timeout.RequestTimeout
	}
	if err = tcpConn.SetKeepAlivePeriod(keepAlivePeriod); err != nil {
		c.log.RaiseAlert("could not set keepalive period on connection to registry: %v", err)
		tcpConn.Close()
		return nil, err
	}
	return conn, nil
}

func (c *RegistryConn) Request(req msgs.RegistryRequest) (msgs.RegistryResponse, error) {
	respC := make(chan shuckResp)
	c.reqChan <- shuckReq{req, respC, time.Now()}
	resp := <-respC
	return resp.resp, resp.err
}

func (c *RegistryConn) RegistryAddress() string {
	return c.registryAddress
}

func (c *RegistryConn) Close() {
	close(c.reqChan)
}
