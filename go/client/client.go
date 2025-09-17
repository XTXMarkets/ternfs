// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// The client package provides a interface to the ternfs cluster that should be
// sufficient for most user level operations (modifying metadata, reading and
// writing blocks, etc).
//
// To use this library you still require an understanding of the way ternfs
// operations, and you will still need to construct the request and reply
// messages (defined in the [msgs] package), but all service discovery and
// network communication is handled for you.
package client

import (
	"bytes"
	"container/heap"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
	"xtx/ternfs/core/bincode"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/msgs"
)

type ReqCounters struct {
	Timings  timing.Timings
	Attempts uint64
}

func MergeReqCounters(cs []ReqCounters) *ReqCounters {
	counters := cs[0] // important to copy
	for i := 1; i < len(cs); i++ {
		counters.Attempts += cs[i].Attempts
		counters.Timings.Merge(&cs[i].Timings)
	}
	return &counters
}

type ClientCounters struct {
	Shard map[uint8]*[256]ReqCounters
	CDC   map[uint8]*ReqCounters
}

func NewClientCounters() *ClientCounters {
	counters := ClientCounters{
		Shard: make(map[uint8]*[256]ReqCounters),
		CDC:   make(map[uint8]*ReqCounters),
	}
	for _, k := range msgs.AllShardMessageKind {
		// max = ~1min
		var shards [256]ReqCounters
		counters.Shard[uint8(k)] = &shards
		for i := 0; i < 256; i++ {
			shards[i].Timings = *timing.NewTimings(40, time.Microsecond*10, 1.5)
		}
	}
	for _, k := range msgs.AllCDCMessageKind {
		// max = ~2min
		counters.CDC[uint8(k)] = &ReqCounters{
			Timings: *timing.NewTimings(35, time.Millisecond, 1.5),
		}
	}
	return &counters
}

func (counters *ClientCounters) Log(log *log.Logger) {
	formatCounters := func(c *ReqCounters) {
		totalCount := uint64(0)
		for _, bin := range c.Timings.Histogram() {
			totalCount += bin.Count
		}
		log.Info("    count: %v", totalCount)
		if totalCount == 0 {
			log.Info("    attempts: %v", c.Attempts)
		} else {
			log.Info("    attempts: %v (%v)", c.Attempts, float64(c.Attempts)/float64(totalCount))
		}
		log.Info("    total time: %v", c.Timings.TotalTime())
		log.Info("    avg time: %v", c.Timings.Mean())
		log.Info("    median time: %v", c.Timings.Median())
		hist := bytes.NewBuffer([]byte{})
		first := true
		countSoFar := uint64(0)
		lowerBound := time.Duration(0)
		for _, bin := range c.Timings.Histogram() {
			if bin.Count == 0 {
				continue
			}
			countSoFar += bin.Count
			if first {
				fmt.Fprintf(hist, "%v < ", lowerBound)
			} else {
				fmt.Fprintf(hist, ", ")
			}
			first = false
			fmt.Fprintf(hist, "%v (%0.2f%%) < %v", bin.Count, float64(countSoFar*100)/float64(totalCount), bin.UpperBound)
		}
		log.Info("    hist: %v", hist.String())
	}
	var shardTime time.Duration
	for _, k := range msgs.AllShardMessageKind {
		for i := 0; i < 256; i++ {
			shardTime += counters.Shard[uint8(k)][i].Timings.TotalTime()
		}
	}
	log.Info("Shard stats (total shard time %v):", shardTime)
	for _, k := range msgs.AllShardMessageKind {
		c := MergeReqCounters(counters.Shard[uint8(k)][:])
		if c.Attempts == 0 {
			continue
		}
		log.Info("  %v", k)
		formatCounters(c)
	}
	var cdcTime time.Duration
	for _, k := range msgs.AllCDCMessageKind {
		cdcTime += counters.CDC[uint8(k)].Timings.TotalTime()
	}
	log.Info("CDC stats (total CDC time %v):", cdcTime)
	for _, k := range msgs.AllCDCMessageKind {
		c := counters.CDC[uint8(k)]
		if c.Attempts == 0 {
			continue
		}
		log.Info("  %v", k)
		formatCounters(c)
	}

}

var DefaultShardTimeout = timing.ReqTimeouts{
	Initial: 100 * time.Millisecond,
	Max:     2 * time.Second,
	Overall: 10 * time.Second,
	Growth:  1.5,
	Jitter:  0.1,
}

var DefaultCDCTimeout = timing.ReqTimeouts{
	Initial: time.Second,
	Max:     10 * time.Second,
	Overall: time.Minute,
	Growth:  1.5,
	Jitter:  0.1,
}

var DefaultBlockTimeout = timing.ReqTimeouts{
	Initial: time.Second,
	Max:     10 * time.Second,
	Overall: 5 * time.Minute,
	Growth:  1.5,
	Jitter:  0.1,
}

type metadataProcessorRequest struct {
	requestId uint64
	timeout   time.Duration
	shard     int16 // -1 = cdc
	req       bincode.Packable
	resp      bincode.Unpackable
	extra     any
	respCh    chan *metadataProcessorResponse
	// filled in by request processor
	deadline time.Time
	index    int // index in the heap
}

type metadataProcessorResponse struct {
	requestId uint64
	resp      any
	extra     any
	err       error
}

type metadataRequestsPQ []*metadataProcessorRequest

func (pq metadataRequestsPQ) Len() int { return len(pq) }

func (pq metadataRequestsPQ) Less(i, j int) bool {
	return pq[i].deadline.UnixNano() < pq[j].deadline.UnixNano()
}

func (pq metadataRequestsPQ) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *metadataRequestsPQ) Push(x any) {
	n := len(*pq)
	item := x.(*metadataProcessorRequest)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *metadataRequestsPQ) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

type rawMetadataResponse struct {
	receivedAt time.Time
	protocol   uint32
	requestId  uint64
	kind       uint8
	respLen    int
	buf        *[]byte // the buf contains the header
}

type clientMetadata struct {
	client *Client
	sock   *net.UDPConn

	requestsById                 map[uint64]*metadataProcessorRequest // requests we've sent, by req id
	requestsByTimeout            metadataRequestsPQ                   // requests we've sent, by timeout (earlier first)
	earlyRequests                map[uint64]rawMetadataResponse       // requests we've received a response for, but that we haven't seen that we've sent yet. should be uncommon.
	lastCleanedUpEarlyRequestsAt time.Time

	quitResponseProcessor chan struct{}                  // channel to quit the response processor, which in turn closes the socket
	incoming              chan *metadataProcessorRequest // channel where user requests come in
	inFlight              chan *metadataProcessorRequest // channel going from request processor to response processor
	rawResponses          chan rawMetadataResponse       // channel going from the socket drainer to the response processor
	responsesBufs         chan *[]byte                   // channel to store a cache of buffers to read into
}

var whichMetadatataAddr int

func (cm *clientMetadata) init(log *log.Logger, client *Client) error {
	log.Debug("initiating clientMetadata")
	defer log.Debug("finished initializing clientMetadata")
	cm.client = client
	sock, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return err
	}
	cm.sock = sock.(*net.UDPConn)
	// 10MiB/100byte ~ 100k requests in the pipe. 100byte is
	// kinda conservative.
	if err := cm.sock.SetReadBuffer(1 << 20); err != nil {
		cm.sock.Close()
		return err
	}

	cm.requestsById = make(map[uint64]*metadataProcessorRequest)
	cm.requestsByTimeout = make(metadataRequestsPQ, 0)
	cm.earlyRequests = make(map[uint64]rawMetadataResponse)

	cm.quitResponseProcessor = make(chan struct{})
	cm.incoming = make(chan *metadataProcessorRequest, 10_000)
	cm.inFlight = make(chan *metadataProcessorRequest, 10_000)
	cm.rawResponses = make(chan rawMetadataResponse, 10_000)
	cm.responsesBufs = make(chan *[]byte, 128)
	for i := 0; i < cap(cm.responsesBufs); i++ {
		buf := make([]byte, clientMtu)
		cm.responsesBufs <- &buf
	}

	go cm.processRequests(log)
	go cm.processResponses(log)
	go cm.drainSocket(log)

	return nil
}

func (cm *clientMetadata) close() {
	cm.quitResponseProcessor <- struct{}{}
	close(cm.incoming)
}

// terminates when cm.incoming gets nil
func (cm *clientMetadata) processRequests(log *log.Logger) {
	buf := bytes.NewBuffer([]byte{})
	for req := range cm.incoming {
		dontWait := req.resp == nil
		log.Debug("sending request %T %+v req id %v to shard %v", req.req, req.req, req.requestId, req.shard)
		buf.Reset()
		var addrs *[2]net.UDPAddr
		var kind uint8
		var protocol uint32
		if req.shard >= 0 { // shard
			addrs = cm.client.shardAddrs(msgs.ShardId(req.shard))
			kind = uint8(req.req.(msgs.ShardRequest).ShardRequestKind())
			protocol = msgs.SHARD_REQ_PROTOCOL_VERSION
		} else { // CDC
			addrs = cm.client.cdcAddrs()
			kind = uint8(req.req.(msgs.CDCRequest).CDCRequestKind())
			protocol = msgs.CDC_REQ_PROTOCOL_VERSION
		}
		binary.Write(buf, binary.LittleEndian, protocol)
		binary.Write(buf, binary.LittleEndian, req.requestId)
		binary.Write(buf, binary.LittleEndian, kind)
		if err := req.req.Pack(buf); err != nil {
			log.RaiseAlert("could not pack request %v to shard %v: %v", req.req, req.shard, err)
			if !dontWait {
				req.respCh <- &metadataProcessorResponse{
					requestId: req.requestId,
					err:       err,
					extra:     req.extra,
					resp:      nil,
				}
			}
			// keep running even if the socket is totally broken to process all the requests
			continue
		}
		addr := &addrs[whichMetadatataAddr%2]
		if addr.Port == 0 {
			addr = &addrs[0]
		}
		whichMetadatataAddr++
		var written int
		var err error
		epermAlert := log.NewNCAlert(0)
		for attempts := 0; ; attempts++ {
			written, err = cm.sock.WriteToUDP(buf.Bytes(), addr)
			var opError *net.OpError
			// We get EPERM when nf drops packets, at least on fsf1/fsf2.
			// This is rare.
			if errors.As(err, &opError) && os.IsPermission(opError.Err) {
				log.RaiseNC(epermAlert, "could not send metadata packet because of EPERM (attempt %v), will retry in 100ms: %v", attempts, err)
				time.Sleep(100 * time.Millisecond)
			} else {
				log.ClearNC(epermAlert)
				break
			}
		}
		if err != nil {
			log.RaiseAlert("could not send request %v to shard %v addr %v: %v", req.req, req.shard, addr, err)
			if !dontWait {
				req.respCh <- &metadataProcessorResponse{
					requestId: req.requestId,
					err:       err,
					extra:     req.extra,
					resp:      nil,
				}
			}
			// keep running even if the socket is totally broken to process all the requests
			continue
		}
		if written != len(buf.Bytes()) {
			panic(fmt.Errorf("%v != %v", written, len(buf.Bytes())))
		}
		if !dontWait {
			req.deadline = time.Now().Add(req.timeout)
			cm.inFlight <- req
		}
	}
	log.Debug("got close request in request processor, winding down")
}

func (cm *clientMetadata) parseResponse(log *log.Logger, req *metadataProcessorRequest, rawResp *rawMetadataResponse, dischargeBuf bool) {
	defer func() {
		if !dischargeBuf {
			return
		}
		select {
		case cm.responsesBufs <- rawResp.buf:
		default:
			panic(fmt.Errorf("impossible: could not put back response buffer which we got from socket drainer"))
		}
	}()
	// check protocol
	if req.shard < 0 { // CDC
		if rawResp.protocol != msgs.CDC_RESP_PROTOCOL_VERSION {
			log.RaiseAlert("got bad cdc protocol %v for request id %v, ignoring", rawResp.protocol, req.requestId)
			return
		}
	} else {
		if rawResp.protocol != msgs.SHARD_RESP_PROTOCOL_VERSION {
			log.RaiseAlert("got bad shard protocol %v for request id %v, shard %v, ignoring", rawResp.protocol, req.shard, req.requestId)
			return
		}
	}
	// remove everywhere
	delete(cm.earlyRequests, req.requestId)
	if _, found := cm.requestsById[req.requestId]; found {
		delete(cm.requestsById, req.requestId)
		heap.Remove(&cm.requestsByTimeout, req.index)
	}
	if rawResp.kind == msgs.ERROR {
		var err error
		if rawResp.respLen != 4+8+1+2 {
			log.RaiseAlert("bad error response length %v, expected %v", rawResp.respLen, 4+8+1+2)
			err = msgs.MALFORMED_RESPONSE
		} else {
			err = msgs.TernError(binary.LittleEndian.Uint16((*rawResp.buf)[4+8+1:]))
		}
		req.respCh <- &metadataProcessorResponse{
			requestId: req.requestId,
			err:       err,
			extra:     req.extra,
			resp:      nil,
		}
	} else {
		// check kind
		if req.shard < 0 { // CDC
			expectedKind := req.req.(msgs.CDCRequest).CDCRequestKind()
			if uint8(expectedKind) != rawResp.kind {
				log.RaiseAlert("got bad cdc kind %v for request id %v, expected %v", msgs.CDCMessageKind(rawResp.kind), req.requestId, expectedKind)
				req.respCh <- &metadataProcessorResponse{
					requestId: req.requestId,
					err:       msgs.MALFORMED_RESPONSE,
					extra:     req.extra,
					resp:      nil,
				}
				return
			}
		} else {
			expectedKind := req.req.(msgs.ShardRequest).ShardRequestKind()
			if uint8(expectedKind) != rawResp.kind {
				log.RaiseAlert("got bad shard kind %v for request id %v, shard %v, expected %v", msgs.ShardMessageKind(rawResp.kind), req.requestId, req.shard, expectedKind)
				req.respCh <- &metadataProcessorResponse{
					requestId: req.requestId,
					err:       msgs.MALFORMED_RESPONSE,
					extra:     req.extra,
					resp:      nil,
				}
				return
			}
		}
		// unpack
		if err := bincode.Unpack((*rawResp.buf)[4+8+1:rawResp.respLen], req.resp); err != nil {
			log.RaiseAlert("could not unpack resp %T for request id %v, shard %v: %v", req.resp, req.requestId, req.shard, err)
			req.respCh <- &metadataProcessorResponse{
				requestId: req.requestId,
				err:       err,
				extra:     req.extra,
				resp:      nil,
			}
			return
		}
		log.Debug("received resp %v req id %v from shard %v", req.resp, req.requestId, req.shard)
		// done
		req.respCh <- &metadataProcessorResponse{
			requestId: req.requestId,
			err:       nil,
			extra:     req.extra,
			resp:      req.resp,
		}
	}
}

func (cm *clientMetadata) processRawResponse(log *log.Logger, rawResp *rawMetadataResponse) {
	if rawResp.buf != nil {
		if req, found := cm.requestsById[rawResp.requestId]; found {
			// Common case, the request is already there
			cm.parseResponse(log, req, rawResp, true)
		} else {
			// Uncommon case, the request is missing. In this (rare) case
			// we still discharge the buffer immediately so that it's already
			// available for use
			buf := make([]byte, clientMtu)
			select {
			case cm.responsesBufs <- &buf:
			default:
				panic(fmt.Errorf("impossible: could not return buffer"))
			}
			cm.earlyRequests[rawResp.requestId] = *rawResp
		}
	}
	now := time.Now()
	// expire requests
	for len(cm.requestsById) > 0 {
		first := cm.requestsByTimeout[0]
		if now.After(first.deadline) {
			log.Debug("request %v %T to shard %v has timed out", first.requestId, first.req, first.shard)
			heap.Pop(&cm.requestsByTimeout)
			delete(cm.requestsById, first.requestId)
			// consumer might very plausibly be gone by now,
			// don't risk it
			go func() {
				first.respCh <- &metadataProcessorResponse{
					requestId: first.requestId,
					err:       msgs.TIMEOUT,
					extra:     first.extra,
					resp:      nil,
				}
			}()
		} else {
			log.Debug("first request %v %T has not passed deadline %v", first.requestId, first.req, first.deadline)
			break
		}
	}
	// expire request we got past timeouts
	if now.Sub(cm.lastCleanedUpEarlyRequestsAt) > time.Minute {
		cm.lastCleanedUpEarlyRequestsAt = now
		for reqId, rawReq := range cm.earlyRequests {
			if now.Sub(rawReq.receivedAt) > 10*time.Minute {
				delete(cm.earlyRequests, reqId)
			}
		}
	}
}

// terminates when `cm.quitResponseProcessor` gets a message
func (cm *clientMetadata) processResponses(log *log.Logger) {
	for {
		// prioritize responses to requests, we want to get
		// them soon to not get spurious timeouts
		select {
		case rawResp := <-cm.rawResponses:
			cm.processRawResponse(log, &rawResp)
			continue
		case <-cm.quitResponseProcessor:
			log.Info("winding down response processor")
			cm.sock.Close()
			return
		default:
		}
		select {
		case req := <-cm.inFlight:
			if rawResp, found := cm.earlyRequests[req.requestId]; found {
				// uncommon case: we have a response for this already.
				cm.parseResponse(log, req, &rawResp, false)
			} else {
				// common case: we don't have the response yet, put it in the data structures and wait.
				// if the request was there before, we remove it from the heap so that we don't have
				// dupes and the deadline is right
				if _, found := cm.requestsById[req.requestId]; found {
					heap.Remove(&cm.requestsByTimeout, req.index)
				}
				cm.requestsById[req.requestId] = req
				heap.Push(&cm.requestsByTimeout, req)
			}
		case rawResp := <-cm.rawResponses:
			cm.processRawResponse(log, &rawResp)
		case <-cm.quitResponseProcessor:
			log.Info("winding down response processor")
			cm.sock.Close()
			return
		}
	}
}

// terminates when the socket is closed
func (cm *clientMetadata) drainSocket(log *log.Logger) {
	for {
		buf := <-cm.responsesBufs
		cm.sock.SetReadDeadline(time.Now().Add(DefaultShardTimeout.Initial / 2))
		read, _, err := cm.sock.ReadFromUDP(*buf)
		if os.IsTimeout(err) {
			cm.responsesBufs <- buf
			cm.rawResponses <- rawMetadataResponse{}
			continue
		}
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Info("socket is closed, winding down")
				cm.responsesBufs <- buf
				return
			} else {
				log.RaiseAlert("got error when reading socket: %v", err)
			}
		}
		if read < 4+8+1 {
			log.RaiseAlert("got runt metadata message, expected at least %v bytes, got %v", 4+8+1, read)
			cm.responsesBufs <- buf
			continue
		}
		rawResp := rawMetadataResponse{
			receivedAt: time.Now(),
			respLen:    read,
			buf:        buf,
			protocol:   binary.LittleEndian.Uint32(*buf),
			requestId:  binary.LittleEndian.Uint64((*buf)[4:]),
			kind:       (*buf)[4+8],
		}
		cm.rawResponses <- rawResp
	}
}

type blockCompletion struct {
	Resp  msgs.BlocksResponse
	Extra any
	Error error
}

type clientBlockResponse struct {
	req   msgs.BlocksRequest
	resp  msgs.BlocksResponse
	extra any
	// when fetching block, this gets written into
	additionalBodyWriter io.ReaderFrom
	// stores the error, if any
	err error
	// called when we're done
	completionChan chan *blockCompletion
}

func (resp *clientBlockResponse) done(log *log.Logger, addr1 *net.TCPAddr, addr2 *net.TCPAddr, extra any, err error) {
	if resp.err == nil && err != nil {
		log.InfoStack(1, "failing request %T %+v addr1=%+v addr2=%+v extra=%+v: %v", resp.req, resp.req, addr1, addr2, extra, err)
		resp.err = err
	}
	completion := &blockCompletion{
		Resp:  resp.resp,
		Error: resp.err,
		Extra: resp.extra,
	}
	resp.completionChan <- completion
}

type clientBlockRequest struct {
	blockService         msgs.BlockServiceId
	req                  msgs.BlocksRequest
	additionalBodyReader io.Reader // when writing block, this will be written after the request
	resp                 *clientBlockResponse
}

type blocksProcessorConn struct {
	conn       *net.TCPConn
	generation uint64
}

type clientBlockResponseWithGeneration struct {
	generation uint64
	resp       *clientBlockResponse
}

type blocksProcessor struct {
	reqChan         chan *clientBlockRequest
	inFlightReqChan chan clientBlockResponseWithGeneration
	addr1           net.TCPAddr
	addr2           net.TCPAddr
	sourceAddr1     *net.TCPAddr
	sourceAddr2     *net.TCPAddr
	what            string
	timeout         **timing.ReqTimeouts
	_conn           *blocksProcessorConn // this must be loaded through loadConn
}

func (proc *blocksProcessor) loadConn() *blocksProcessorConn {
	return (*blocksProcessorConn)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&proc._conn))))
}

func (proc *blocksProcessor) storeConn(conn *net.TCPConn) *blocksProcessorConn {
	gen := proc.loadConn().generation
	newConn := &blocksProcessorConn{
		conn:       conn,
		generation: gen + 1,
	}
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&proc._conn)), unsafe.Pointer(newConn))
	return newConn
}

var whichBlockIp uint64
var whichSourceIp uint64

func (proc *blocksProcessor) connect(log *log.Logger) (*net.TCPConn, error) {
	var err error
	sourceIpSelector := atomic.AddUint64(&whichSourceIp, 1)
	var sourceAddr *net.TCPAddr
	if sourceIpSelector&1 == 0 {
		sourceAddr = proc.sourceAddr1
	} else {
		sourceAddr = proc.sourceAddr2
	}

	blockIpSelector := atomic.AddUint64(&whichBlockIp, 1)
	for i := blockIpSelector; i < blockIpSelector+2; i++ {
		var addr *net.TCPAddr
		if i&1 == 0 {
			addr = &proc.addr1
		} else {
			addr = &proc.addr2
		}
		if addr.Port == 0 {
			continue
		}
		log.Debug("trying to connect to block service %v", addr)
		dialer := net.Dialer{LocalAddr: sourceAddr, Timeout: (*proc.timeout).Max}
		var conn net.Conn
		conn, err = dialer.Dial("tcp4", addr.String())
		if err == nil {
			sock := conn.(*net.TCPConn)
			log.Debug("connected to block service at %v", addr)
			return sock, nil
		}
		log.Info("could not connect to block service %v, might try next connection: %v", addr, err)
	}
	if err == nil {
		panic(fmt.Errorf("impossible: got out without errors"))
	}
	return nil, err
}

// From <https://stackoverflow.com/a/58664631>, checks if a connection
// is still alive.
func connCheck(conn *net.TCPConn) error {
	var sysErr error = nil
	rc, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	err = rc.Read(func(fd uintptr) bool {
		var buf []byte = []byte{0}
		n, _, err := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch {
		case n == 0 && err == nil:
			sysErr = io.EOF
		case err == syscall.EAGAIN || err == syscall.EWOULDBLOCK:
			sysErr = nil
		default:
			sysErr = err
		}
		return true
	})
	if err != nil {
		return err
	}
	return sysErr
}

func (proc *blocksProcessor) processRequests(log *log.Logger) {
	log.Debug("%v: starting request processor for addr1=%v addr2=%v", proc.what, proc.addr1, proc.addr2)
	// one iteration = one request
	for {
		conn := proc.loadConn()
		req, ok := <-proc.reqChan
		if !ok {
			log.Debug("%v: got nil request, tearing down", proc.what)
			if conn.conn != nil {
				conn.conn.Close()
			}
			close(proc.inFlightReqChan) // this tears down the response processor
			return
		}
		// empty queue, check that conn is alive, otherwise we might still succeed sending,
		// but inevitably fail when reading.
		if conn.conn != nil && len(proc.inFlightReqChan) == 0 {
			if err := connCheck(conn.conn); err != nil {
				log.Debug("connection for addr1=%+v addr2=%+v is dead: %v", proc.addr1, proc.addr2, err)
				conn.conn.Close()
				conn.conn = nil
			}
		}
		// clear stale connections otherwise we might succeed writing but fail reading on the other side
		if conn.conn == nil {
			tcpConn, err := proc.connect(log)
			if err != nil { // we couldn't connect, not much to do
				req.resp.done(log, &proc.addr1, &proc.addr2, req.resp.extra, err)
				continue
			}
			// we did connect
			conn = proc.storeConn(tcpConn)
		}
		log.Debug("writing block request %T %+v for addr1=%v addr2=%v", req.req, req.req, proc.addr1, proc.addr2)
		if err := writeBlocksRequest(log, conn.conn, req.blockService, req.req); err != nil {
			log.Info("got error when writing block request of kind %v in %v->%v: %v", req.req.BlocksRequestKind(), conn.conn.LocalAddr(), conn.conn.RemoteAddr(), err)
			req.resp.done(log, &proc.addr1, &proc.addr2, req.resp.extra, err)
			conn.conn.Close()
			conn.conn = nil
			continue
		}
		if req.req.BlocksRequestKind() == msgs.WRITE_BLOCK {
			writeReq := req.req.(*msgs.WriteBlockReq)
			lr := &io.LimitedReader{
				R: req.additionalBodyReader,
				N: int64(writeReq.Size),
			}
			log.Debug("writing block body to %v->%v", conn.conn.LocalAddr(), conn.conn.RemoteAddr())
			writtenBytes, err := conn.conn.ReadFrom(lr)
			if err != nil || writtenBytes < int64(writeReq.Size) {
				if err == nil {
					err = io.EOF
				}
				log.Info("got error when writing block body: %v", err)
				req.resp.done(log, &proc.addr1, &proc.addr2, req.resp.extra, err)
				conn.conn.Close()
				conn.conn = nil
				continue
			}
		}
		// we wrote it fine, proceed
		proc.inFlightReqChan <- clientBlockResponseWithGeneration{
			generation: conn.generation,
			resp:       req.resp,
		}
	}
}

func (proc *blocksProcessor) processResponses(log *log.Logger) {
	log.Debug("%v: starting response processor for addr1=%v addr2=%v", proc.what, proc.addr1, proc.addr2)
	// one iteration = one request
	for resp := range proc.inFlightReqChan {
		conn := proc.loadConn()
		connr := conn.conn
		if connr == nil {
			log.Info("%v: resp %T %+v has no conn, skipping", proc.what, resp.resp.resp, resp.resp.resp)
			resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, io.EOF)
			continue
		}
		if conn.generation != resp.generation {
			log.Info("%v: resp %T %+v has bad generation %v vs %v, skipping", proc.what, resp.resp.resp, resp.resp.resp, resp.generation, conn.generation)
			resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, io.EOF)
			continue
		}
		log.Debug("reading block response %T for req %+v from %v->%v", resp.resp.resp, resp.resp.req, connr.LocalAddr(), connr.RemoteAddr())
		// the responsibility for cleaning up the connection is always in the request processor
		if err := readBlocksResponse(log, connr, resp.resp.resp); err != nil {
			resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, err)
			continue
		}
		if resp.resp.resp.BlocksResponseKind() == msgs.FETCH_BLOCK_WITH_CRC {
			req := resp.resp.req.(*msgs.FetchBlockWithCrcReq)
			pageCount := (req.Count / msgs.TERN_PAGE_SIZE)
			log.Debug("reading block body from %v->%v", connr.LocalAddr(), connr.RemoteAddr())
			var page [msgs.TERN_PAGE_WITH_CRC_SIZE]byte
			pageFailed := false
			for i := uint32(0); i < pageCount; i++ {
				bytesRead := uint32(0)
				for bytesRead < msgs.TERN_PAGE_WITH_CRC_SIZE {
					read, err := connr.Read(page[bytesRead:])
					if err != nil {
						resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, err)
						pageFailed = true
						break
					}
					bytesRead += uint32(read)
				}
				if pageFailed {
					break
				}
				crc := binary.LittleEndian.Uint32(page[msgs.TERN_PAGE_SIZE:])
				actualCrc := crc32c.Sum(0, page[:msgs.TERN_PAGE_SIZE])
				if crc != actualCrc {
					resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, msgs.BAD_BLOCK_CRC)
					pageFailed = true
					break
				}

				readBytes, err := resp.resp.additionalBodyWriter.ReadFrom(bytes.NewReader(page[:msgs.TERN_PAGE_SIZE]))
				if err != nil || uint32(readBytes) < msgs.TERN_PAGE_SIZE {
					if err == nil {
						err = io.EOF
					}
					resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, err)
					pageFailed = true
					break
				}
			}
			if pageFailed {
				continue
			}
		}
		log.Debug("read block response %T %+v for req %v from %v->%v", resp.resp.resp, resp.resp.resp, resp.resp.req, connr.LocalAddr(), connr.RemoteAddr())
		resp.resp.done(log, &proc.addr1, &proc.addr2, resp.resp.extra, nil)
	}
}

type blocksProcessorKey struct {
	blockServiceKey uint64
	addrs           msgs.AddrsInfo
}

type blocksProcessors struct {
	what     string
	timeouts **timing.ReqTimeouts
	// how many bits of the block service id to use for blockServiceKey
	blockServiceBits uint8
	// blocksProcessorKey -> *blocksProcessor
	processors  sync.Map
	sourceAddr1 net.TCPAddr
	sourceAddr2 net.TCPAddr
}

func (procs *blocksProcessors) init(what string, timeouts **timing.ReqTimeouts, localAddresses msgs.AddrsInfo) {
	procs.what = what
	procs.timeouts = timeouts
	if localAddresses.Addr1.Addrs[0] != 0 {
		procs.sourceAddr1 = net.TCPAddr{IP: net.IP(localAddresses.Addr1.Addrs[:]), Port: int(localAddresses.Addr1.Port)}
	}
	if localAddresses.Addr2.Addrs[0] != 0 {
		procs.sourceAddr2 = net.TCPAddr{IP: net.IP(localAddresses.Addr2.Addrs[:]), Port: int(localAddresses.Addr2.Port)}
	}
}

type sendArgs struct {
	blockService       msgs.BlockServiceId
	addrs              msgs.AddrsInfo
	req                msgs.BlocksRequest
	reqAdditionalBody  io.ReadSeeker // this will _only_ be used to seek to start on retries
	resp               msgs.BlocksResponse
	respAdditionalBody io.ReaderFrom
	extra              any
}

// This currently never fails (everything network related happens in
// the processor loops), keeping error since it might fail in the future
func (procs *blocksProcessors) send(
	log *log.Logger,
	args *sendArgs,
	completionChan chan *blockCompletion,
) error {
	if args.addrs.Addr1.Port == 0 && args.addrs.Addr2.Port == 0 {
		panic(fmt.Errorf("got zero ports for both addresses for block service %v: %v:%v %v:%v", args.blockService, args.addrs.Addr1.Addrs, args.addrs.Addr1.Port, args.addrs.Addr2.Addrs, args.addrs.Addr2.Port))
	}
	resp := &clientBlockResponse{
		req:                  args.req,
		resp:                 args.resp,
		additionalBodyWriter: args.respAdditionalBody,
		completionChan:       completionChan,
		extra:                args.extra,
	}
	req := &clientBlockRequest{
		blockService:         args.blockService,
		req:                  args.req,
		additionalBodyReader: args.reqAdditionalBody,
		resp:                 resp,
	}
	key := blocksProcessorKey{
		blockServiceKey: uint64(args.blockService) & ((1 << uint64(procs.blockServiceBits)) - 1),
		addrs:           args.addrs,
	}

	// likely case, we already have something. we could
	// LoadOrStore directly but this saves us allocating new
	// chans
	if procAny, found := procs.processors.Load(key); found {
		proc := procAny.(*blocksProcessor)
		proc.reqChan <- req
		return nil
	}
	// unlikely case, create new one
	procAny, loaded := procs.processors.LoadOrStore(key, &blocksProcessor{
		reqChan:         make(chan *clientBlockRequest, 128),
		inFlightReqChan: make(chan clientBlockResponseWithGeneration, 128),
		addr1:           net.TCPAddr{IP: net.IP(args.addrs.Addr1.Addrs[:]), Port: int(args.addrs.Addr1.Port)},
		addr2:           net.TCPAddr{IP: net.IP(args.addrs.Addr2.Addrs[:]), Port: int(args.addrs.Addr2.Port)},
		sourceAddr1:     &procs.sourceAddr1,
		sourceAddr2:     &procs.sourceAddr2,
		what:            procs.what,
		timeout:         procs.timeouts,
		_conn:           &blocksProcessorConn{},
	})
	proc := procAny.(*blocksProcessor)
	if !loaded {
		// we're the first ones here, start the routines
		go proc.processRequests(log)
		go proc.processResponses(log)
	}
	proc.reqChan <- req
	return nil
}

func (procs *blocksProcessors) close() {
	procs.processors.Range(func(key, value any) bool {
		close(value.(*blocksProcessor).reqChan)
		return true
	})
}

type Client struct {
	shardRawAddrs        [256][2]uint64
	cdcRawAddr           [2]uint64
	clientMetadata       clientMetadata
	counters             *ClientCounters
	writeBlockProcessors blocksProcessors
	fetchBlockProcessors blocksProcessors
	fetchBlockBufs       sync.Pool
	eraseBlockProcessors blocksProcessors
	checkBlockProcessors blocksProcessors
	shardTimeout         *timing.ReqTimeouts
	cdcTimeout           *timing.ReqTimeouts
	blockTimeout         *timing.ReqTimeouts
	requestIdCounter     uint64
	registryAddress       string
	addrsRefreshTicker   *time.Ticker
	addrsRefreshClose    chan (struct{})
	registryConn          *RegistryConn

	fetchBlockServices          bool
	blockServicesLock           *sync.RWMutex
	blockServiceToFailureDomain map[msgs.BlockServiceId]msgs.FailureDomain
}

func (c *Client) refreshAddrs(log *log.Logger) error {
	var shardAddrs [256]msgs.AddrsInfo
	var cdcAddrs msgs.AddrsInfo
	{
		log.Info("Getting shard/CDC info from registry at '%v'", c.registryAddress)
		resp, err := c.registryConn.Request(&msgs.LocalShardsReq{})
		if err != nil {
			return fmt.Errorf("could not request shards from registry: %w", err)
		}
		shards := resp.(*msgs.LocalShardsResp)
		for i, shard := range shards.Shards {
			if shard.Addrs.Addr1.Port == 0 {
				return fmt.Errorf("shard %v not present in registry", i)
			}
			shardAddrs[i] = shard.Addrs
		}
		resp, err = c.registryConn.Request(&msgs.LocalCdcReq{})
		if err != nil {
			return fmt.Errorf("could not request CDC from registry: %w", err)
		}
		cdc := resp.(*msgs.LocalCdcResp)
		if cdc.Addrs.Addr1.Port == 0 {
			return fmt.Errorf("CDC not present in registry")
		}
		cdcAddrs = cdc.Addrs
	}
	c.SetAddrs(cdcAddrs, &shardAddrs)

	fetchBlockServices := func() bool {
		c.blockServicesLock.RLock()
		defer c.blockServicesLock.RUnlock()
		return c.fetchBlockServices
	}()
	if !fetchBlockServices {
		return nil
	}

	blockServicesResp, err := c.registryConn.Request(&msgs.AllBlockServicesDeprecatedReq{})
	if err != nil {
		return fmt.Errorf("could not request block services from registry: %w", err)
	}
	blockServices := blockServicesResp.(*msgs.AllBlockServicesDeprecatedResp)
	var blockServicesToAdd []msgs.BlacklistEntry
	func() {

		for _, bs := range blockServices.BlockServices {
			if _, ok := c.blockServiceToFailureDomain[bs.Id]; !ok {
				blockServicesToAdd = append(blockServicesToAdd, msgs.BlacklistEntry{bs.FailureDomain, bs.Id})
			}
		}
	}()
	if len(blockServicesToAdd) > 0 {
		func() {
			c.blockServicesLock.Lock()
			defer c.blockServicesLock.Unlock()
			for _, bs := range blockServicesToAdd {
				c.blockServiceToFailureDomain[bs.BlockService] = bs.FailureDomain
			}
		}()
	}

	return nil
}

// Create a new client by acquiring the CDC and Shard connection details from Registry. It
// also refreshes the shard/cdc infos every minute.
func NewClient(
	log *log.Logger,
	registryTimeout *RegistryTimeouts,
	registryAddress string,
	localAddresses msgs.AddrsInfo,
) (*Client, error) {
	c, err := NewClientDirectNoAddrs(log, localAddresses)
	if err != nil {
		return nil, err
	}
	c.registryAddress = registryAddress
	c.registryConn = MakeRegistryConn(log, registryTimeout, registryAddress, 1)
	if err := c.refreshAddrs(log); err != nil {
		c.registryConn.Close()
		return nil, err
	}
	c.addrsRefreshTicker = time.NewTicker(time.Minute)
	c.addrsRefreshClose = make(chan struct{})
	addressRefreshAlert := log.NewNCAlert(5 * time.Minute)
	go func() {

		for {
			select {
			case <-c.addrsRefreshClose:
				return
			case <-c.addrsRefreshTicker.C:
				if err := c.refreshAddrs(log); err != nil {
					log.RaiseNC(addressRefreshAlert, "could not refresh shard & cdc addresses: %v", err)
				} else {
					log.ClearNC(addressRefreshAlert)
				}
			}
		}
	}()

	return c, nil
}

// Attach an optional counters object to track requests.
// This is only safe to use during initialization.
func (c *Client) SetCounters(counters *ClientCounters) {
	c.counters = counters
}

// Override the shard timeout parameters.
// This is only safe to use during initialization.
func (c *Client) SetShardTimeouts(t *timing.ReqTimeouts) {
	c.shardTimeout = t
}

// Override the CDC timeout parameters.
// This is only safe to use during initialization.
func (c *Client) SetCDCTimeouts(t *timing.ReqTimeouts) {
	c.cdcTimeout = t
}

// Override the block timeout parameters.
// This is only safe to use during initialization.
func (c *Client) SetBlockTimeout(t *timing.ReqTimeouts) {
	c.blockTimeout = t
}

func (c *Client) IncreaseNumRegistryHandlersTo(numHandlers uint) {
	c.registryConn.IncreaseNumHandlersTo(numHandlers)
}

var clientMtu uint16 = msgs.DEFAULT_UDP_MTU

// MTU used for large responses (file spans etc)
// This is only safe to use during initialization.
func SetMTU(mtu uint64) {
	if mtu < msgs.DEFAULT_UDP_MTU {
		panic(fmt.Errorf("mtu (%v) < DEFAULT_UDP_MTU (%v)", mtu, msgs.DEFAULT_UDP_MTU))
	}
	if mtu > msgs.MAX_UDP_MTU {
		panic(fmt.Errorf("mtu (%v) > MAX_UDP_MTU (%v)", mtu, msgs.MAX_UDP_MTU))
	}
	clientMtu = uint16(mtu)
}

func NewClientDirectNoAddrs(
	log *log.Logger,
	localAddresses msgs.AddrsInfo,
) (c *Client, err error) {
	c = &Client{
		// do not catch requests from previous executions
		requestIdCounter: rand.Uint64(),
		fetchBlockBufs: sync.Pool{
			New: func() any {
				return bytes.NewBuffer([]byte{})
			},
		},
		fetchBlockServices:          false,
		blockServicesLock:           &sync.RWMutex{},
		blockServiceToFailureDomain: make(map[msgs.BlockServiceId]msgs.FailureDomain),
	}
	c.shardTimeout = &DefaultShardTimeout
	c.cdcTimeout = &DefaultCDCTimeout
	c.blockTimeout = &DefaultBlockTimeout
	if err := c.clientMetadata.init(log, c); err != nil {
		return nil, err
	}
	// Ideally, for write/fetch we'd want to have one socket
	// per block service. However we don't have flash yet, so
	// it's hard to saturate a socket because of seek time.
	//
	// Currently we have 102 disks per server, 96 servers. 5
	// bits splits the block services in 32 buckets, and the
	// block service id is uniformly distributed.
	//
	// So we'll have roughly 30 connections per server for the first
	// two, and, for a maximum of (currently) 30*96*2 + 102*96*2 =
	// 25k connections, which is within limits.
	//
	// (The exact expected number of connections per server is
	// ~30.7447, I'll let you figure out why.)
	c.writeBlockProcessors.init("write", &c.blockTimeout, localAddresses)
	c.writeBlockProcessors.blockServiceBits = 5
	c.fetchBlockProcessors.init("fetch", &c.blockTimeout, localAddresses)
	c.fetchBlockProcessors.blockServiceBits = 5
	c.eraseBlockProcessors.init("erase", &c.blockTimeout, localAddresses)
	// we're not constrained by bandwidth here, we want to have requests
	// for all block services in parallel.
	c.eraseBlockProcessors.blockServiceBits = 5
	// here we're also not constrained by bandwidth, but the requests
	// take a long time, so have a separate channel from the erase ones.
	c.checkBlockProcessors.init("check", &c.blockTimeout, localAddresses)
	c.checkBlockProcessors.blockServiceBits = 5
	return c, nil
}

func uint64ToUDPAddr(addr uint64) *net.UDPAddr {
	udpAddr := &net.UDPAddr{}
	udpAddr.IP = []byte{byte(addr >> 24), byte(addr >> 16), byte(addr >> 8), byte(addr)}
	udpAddr.Port = int(addr >> 32)
	return udpAddr
}

func (c *Client) cdcAddrs() *[2]net.UDPAddr {
	return &[2]net.UDPAddr{
		*uint64ToUDPAddr(atomic.LoadUint64(&c.cdcRawAddr[0])),
		*uint64ToUDPAddr(atomic.LoadUint64(&c.cdcRawAddr[1])),
	}
}

func (c *Client) shardAddrs(shid msgs.ShardId) *[2]net.UDPAddr {
	return &[2]net.UDPAddr{
		*uint64ToUDPAddr(atomic.LoadUint64(&c.shardRawAddrs[shid][0])),
		*uint64ToUDPAddr(atomic.LoadUint64(&c.shardRawAddrs[shid][1])),
	}
}

// Modify the CDC and Shard IP addresses and ports dynamically.
// The update is atomic, this can be done at any time from any context.
func (c *Client) SetAddrs(
	cdcAddrs msgs.AddrsInfo,
	shardAddrs *[256]msgs.AddrsInfo,
) {
	for i := 0; i < len(shardAddrs); i++ {
		atomic.StoreUint64(
			&c.shardRawAddrs[i][0],
			uint64(shardAddrs[i].Addr1.Port)<<32|uint64(shardAddrs[i].Addr1.Addrs[0])<<24|uint64(shardAddrs[i].Addr1.Addrs[1])<<16|uint64(shardAddrs[i].Addr1.Addrs[2])<<8|uint64(shardAddrs[i].Addr1.Addrs[3]),
		)
		atomic.StoreUint64(
			&c.shardRawAddrs[i][1],
			uint64(shardAddrs[i].Addr2.Port)<<32|uint64(shardAddrs[i].Addr2.Addrs[0])<<24|uint64(shardAddrs[i].Addr2.Addrs[1])<<16|uint64(shardAddrs[i].Addr2.Addrs[2])<<8|uint64(shardAddrs[i].Addr2.Addrs[3]),
		)
	}

	atomic.StoreUint64(
		&c.cdcRawAddr[0],
		uint64(cdcAddrs.Addr1.Port)<<32|uint64(cdcAddrs.Addr1.Addrs[0])<<24|uint64(cdcAddrs.Addr1.Addrs[1])<<16|uint64(cdcAddrs.Addr1.Addrs[2])<<8|uint64(cdcAddrs.Addr1.Addrs[3]),
	)
	atomic.StoreUint64(
		&c.cdcRawAddr[1],
		uint64(cdcAddrs.Addr2.Port)<<32|uint64(cdcAddrs.Addr2.Addrs[0])<<24|uint64(cdcAddrs.Addr2.Addrs[1])<<16|uint64(cdcAddrs.Addr2.Addrs[2])<<8|uint64(cdcAddrs.Addr2.Addrs[3]),
	)
}

func (c *Client) Close() {
	c.addrsRefreshClose <- struct{}{}
	c.addrsRefreshTicker.Stop()
	c.clientMetadata.close()
	c.writeBlockProcessors.close()
	c.fetchBlockProcessors.close()
	c.eraseBlockProcessors.close()
	c.checkBlockProcessors.close()
	if c.registryConn != nil {
		c.registryConn.Close()
	}
}

// Not atomic between the read/write
func (c *Client) MergeDirectoryInfo(log *log.Logger, id msgs.InodeId, entry msgs.IsDirectoryInfoEntry) error {
	packedEntry := msgs.DirectoryInfoEntry{
		Body: bincode.Pack(entry),
		Tag:  entry.Tag(),
	}
	statResp := msgs.StatDirectoryResp{}
	if err := c.ShardRequest(log, id.Shard(), &msgs.StatDirectoryReq{Id: id}, &statResp); err != nil {
		return err
	}
	info := statResp.Info
	found := false
	for i := 0; i < len(info.Entries); i++ {
		if info.Entries[i].Tag == packedEntry.Tag {
			info.Entries[i] = packedEntry
			found = true
			break
		}
	}
	if !found {
		info.Entries = append(info.Entries, packedEntry)
	}
	if err := c.ShardRequest(log, id.Shard(), &msgs.SetDirectoryInfoReq{Id: id, Info: info}, &msgs.SetDirectoryInfoResp{}); err != nil {
		return err
	}
	return nil
}

// Not atomic between the read/write
func (c *Client) RemoveDirectoryInfoEntry(log *log.Logger, id msgs.InodeId, tag msgs.DirectoryInfoTag) error {
	statResp := msgs.StatDirectoryResp{}
	if err := c.ShardRequest(log, id.Shard(), &msgs.StatDirectoryReq{Id: id}, &statResp); err != nil {
		return err
	}
	info := statResp.Info
	for i := 0; i < len(info.Entries); i++ {
		if info.Entries[i].Tag == tag {
			info.Entries = append(info.Entries[:i], info.Entries[i+1:]...)
			break
		}
	}
	if err := c.ShardRequest(log, id.Shard(), &msgs.SetDirectoryInfoReq{Id: id, Info: info}, &msgs.SetDirectoryInfoResp{}); err != nil {
		return err
	}
	return nil
}

func (c *Client) ResolveDirectoryInfoEntry(
	log *log.Logger,
	dirInfoCache *DirInfoCache,
	dirId msgs.InodeId,
	entry msgs.IsDirectoryInfoEntry, // output will be stored in here
) (inheritedFrom msgs.InodeId, err error) {
	statReq := msgs.StatDirectoryReq{
		Id: dirId,
	}
	statResp := msgs.StatDirectoryResp{}
	visited := []msgs.InodeId{}
TraverseDirectories:
	for {
		inheritedFrom = dirInfoCache.LookupCachedDirInfoEntry(statReq.Id, entry)
		if inheritedFrom != msgs.NULL_INODE_ID {
			break
		}
		visited = append(visited, statReq.Id)
		if err := c.ShardRequest(log, statReq.Id.Shard(), &statReq, &statResp); err != nil {
			return msgs.NULL_INODE_ID, err
		}
		for i := len(statResp.Info.Entries) - 1; i >= 0; i-- {
			respEntry := &statResp.Info.Entries[i]
			if entry.Tag() == respEntry.Tag {
				inheritedFrom = statReq.Id
				if err := bincode.Unpack(respEntry.Body, entry); err != nil {
					return msgs.NULL_INODE_ID, fmt.Errorf("could not decode dir info entry for dir %v, inherited from %v, with tag %v, body %v: %v", dirId, statReq.Id, entry.Tag(), respEntry.Body, err)
				}
				break TraverseDirectories
			}
		}
		if statReq.Id == msgs.ROOT_DIR_INODE_ID {
			return msgs.NULL_INODE_ID, fmt.Errorf("could not find directory info entry with tag %v %+v", entry.Tag(), statResp)
		}
		statReq.Id = statResp.Owner
	}
	// since we're traversing upwards, and we want to insert the policies first,
	// update the cache from root to leaf
	for i := len(visited) - 1; i >= 0; i-- {
		id := visited[i]
		if id == inheritedFrom {
			dirInfoCache.UpdatePolicy(id, entry)
		}
		dirInfoCache.UpdateInheritedFrom(id, entry.Tag(), inheritedFrom)
	}
	return inheritedFrom, nil
}

// High-level helper function to take a string path and return the inode and parent inode
func (c *Client) ResolvePathWithParent(log *log.Logger, path string) (id msgs.InodeId, creationTime msgs.TernTime, parent msgs.InodeId, err error) {
	if !filepath.IsAbs(path) {
		return msgs.NULL_INODE_ID, 0, msgs.NULL_INODE_ID, fmt.Errorf("expected absolute path, got '%v'", path)
	}
	parent = msgs.NULL_INODE_ID
	id = msgs.ROOT_DIR_INODE_ID
	for _, segment := range strings.Split(filepath.Clean(path), "/")[1:] {
		if segment == "" {
			continue
		}
		resp := msgs.LookupResp{}
		if err := c.ShardRequest(log, id.Shard(), &msgs.LookupReq{DirId: id, Name: segment}, &resp); err != nil {
			return msgs.NULL_INODE_ID, 0, msgs.NULL_INODE_ID, err
		}
		parent = id
		id = resp.TargetId
		creationTime = resp.CreationTime
	}
	return id, creationTime, parent, nil
}

// High-level helper function to take a string path and return the inode
func (c *Client) ResolvePath(log *log.Logger, path string) (msgs.InodeId, error) {
	id, _, _, err := c.ResolvePathWithParent(log, path)
	return id, err
}

func writeBlockSendArgs(block *msgs.AddSpanInitiateBlockInfo, r io.ReadSeeker, size uint32, crc msgs.Crc, extra any) *sendArgs {
	return &sendArgs{
		block.BlockServiceId,
		block.BlockServiceAddrs,
		&msgs.WriteBlockReq{
			BlockId:     block.BlockId,
			Crc:         crc,
			Size:        size,
			Certificate: block.Certificate,
		},
		r,
		&msgs.WriteBlockResp{},
		nil,
		extra,
	}
}

// An asynchronous version of [StartBlock] that is currently unused.
func (c *Client) StartWriteBlock(log *log.Logger, block *msgs.AddSpanInitiateBlockInfo, r io.ReadSeeker, size uint32, crc msgs.Crc, extra any, completion chan *blockCompletion) error {
	return c.writeBlockProcessors.send(log, writeBlockSendArgs(block, r, size, crc, extra), completion)
}

func retriableBlockError(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}

func (c *Client) singleBlockReq(log *log.Logger, timeouts *timing.ReqTimeouts, processor *blocksProcessors, args *sendArgs) (msgs.BlocksResponse, error) {
	if timeouts == nil {
		timeouts = c.blockTimeout
	}
	startedAt := time.Now()
	for attempt := 0; ; attempt++ {
		ch := make(chan *blockCompletion, 1)
		err := processor.send(log, args, ch)
		if err != nil {
			log.Debug("failed to send block request to %v:%v %v:%v: %v", net.IP(args.addrs.Addr1.Addrs[:]), args.addrs.Addr1.Port, net.IP(args.addrs.Addr2.Addrs[:]), args.addrs.Addr2.Port, err)
			return nil, err
		}
		resp := <-ch
		err = resp.Error
		if err == nil {
			return resp.Resp, nil
		}
		if retriableBlockError(err) {
			next := timeouts.Next(startedAt)
			if next == 0 {
				log.ErrorNoAlert("block request to %v %v:%v %v:%v failed with retriable error (attempt %v), will not retry since time is up: %v", args.blockService, net.IP(args.addrs.Addr1.Addrs[:]), args.addrs.Addr1.Port, net.IP(args.addrs.Addr2.Addrs[:]), args.addrs.Addr2.Port, attempt, err)
				return nil, err
			}
			log.ErrorNoAlert("block request to %v %v:%v %v:%v failed with retriable error (attempt %v), will retry in %v: %v", args.blockService, net.IP(args.addrs.Addr1.Addrs[:]), args.addrs.Addr1.Port, net.IP(args.addrs.Addr2.Addrs[:]), args.addrs.Addr2.Port, attempt, next, err)
			if args.reqAdditionalBody != nil {
				_, err := args.reqAdditionalBody.Seek(0, io.SeekStart)
				if err != nil {
					log.RaiseAlertStack("", 2, "could not seek req additional body, will fail request: %v", err)
					return nil, err
				}
			}
			time.Sleep(next)
		} else {
			// No alert here because there are many circumstances where errors are
			// expected (e.g. scrubbing), and we want the caller to handle this.
			return nil, err
		}
	}
}

func (c *Client) WriteBlock(log *log.Logger, timeouts *timing.ReqTimeouts, block *msgs.AddSpanInitiateBlockInfo, r io.ReadSeeker, size uint32, crc msgs.Crc) (proof [8]byte, err error) {
	resp, err := c.singleBlockReq(log, timeouts, &c.writeBlockProcessors, writeBlockSendArgs(block, r, size, crc, nil))
	if err != nil {
		return proof, err
	}
	return resp.(*msgs.WriteBlockResp).Proof, nil
}

func fetchBlockSendArgs(blockService *msgs.BlockService, blockId msgs.BlockId, offset uint32, count uint32, w io.ReaderFrom, extra any, crc msgs.Crc) *sendArgs {
	return &sendArgs{
		blockService.Id,
		blockService.Addrs,
		&msgs.FetchBlockWithCrcReq{
			BlockId:  blockId,
			BlockCrc: crc,
			Offset:   offset,
			Count:    count,
		},
		nil,
		&msgs.FetchBlockWithCrcResp{},
		w,
		extra,
	}
}

// An asynchronous version of [FetchBlock] that is currently unused.
func (c *Client) StartFetchBlock(log *log.Logger, blockService *msgs.BlockService, blockId msgs.BlockId, offset uint32, count uint32, w io.ReaderFrom, extra any, completion chan *blockCompletion, crc msgs.Crc) error {
	return c.fetchBlockProcessors.send(log, fetchBlockSendArgs(blockService, blockId, offset, count, w, extra, crc), completion)
}

// Return a buffer that was provided by [FetchBlock] to the internal pool.
func (c *Client) PutFetchedBlock(body *bytes.Buffer) {
	c.fetchBlockBufs.Put(body)
}

// Retrieve a single block from the block server.
//
// The block is identified by the BlockService (which holds the id, status flags and ips/ports) and the BlockId.
// Offset and count control the amount of data that is read and the position within the block.
// The default timeout is controlled at the client level if timeouts is nil, otherwise a custom timeout can be specified.
//
// The function returns a buffer that is allocated from an internal pool. Once you have finished with this buffer it must
// be returned via the [PutFetchedBlock] function.
func (c *Client) FetchBlock(log *log.Logger, timeouts *timing.ReqTimeouts, blockService *msgs.BlockService, blockId msgs.BlockId, offset uint32, count uint32, crc msgs.Crc) (body *bytes.Buffer, err error) {
	buf := c.fetchBlockBufs.Get().(*bytes.Buffer)
	buf.Reset()

	_, err = c.singleBlockReq(log, timeouts, &c.fetchBlockProcessors, fetchBlockSendArgs(blockService, blockId, offset, count, buf, nil, crc))
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func eraseBlockSendArgs(block *msgs.RemoveSpanInitiateBlockInfo, extra any) *sendArgs {
	return &sendArgs{
		block.BlockServiceId,
		block.BlockServiceAddrs,
		&msgs.EraseBlockReq{
			BlockId:     block.BlockId,
			Certificate: block.Certificate,
		},
		nil,
		&msgs.EraseBlockResp{},
		nil,
		extra,
	}
}

// An asynchronous version of [EraseBlock] that is currently unused.
func (c *Client) StartEraseBlock(log *log.Logger, block *msgs.RemoveSpanInitiateBlockInfo, extra any, completion chan *blockCompletion) error {
	return c.eraseBlockProcessors.send(log, eraseBlockSendArgs(block, extra), completion)
}

func (c *Client) EraseBlock(log *log.Logger, block *msgs.RemoveSpanInitiateBlockInfo) (proof [8]byte, err error) {
	resp, err := c.singleBlockReq(log, nil, &c.eraseBlockProcessors, eraseBlockSendArgs(block, nil))
	if err != nil {
		return proof, err
	}
	return resp.(*msgs.EraseBlockResp).Proof, nil
}

func (c *Client) RegistryAddress() string {
	return c.registryAddress
}

func (c *Client) EraseDecommissionedBlock(block *msgs.RemoveSpanInitiateBlockInfo) (proof [8]byte, err error) {
	resp, err := c.registryConn.Request(&msgs.EraseDecommissionedBlockReq{block.BlockServiceId, block.BlockId, block.Certificate})
	if err != nil {
		return [8]byte{}, err
	}
	return resp.(*msgs.EraseDecommissionedBlockResp).Proof, nil
}

func checkBlockSendArgs(blockService *msgs.BlockService, blockId msgs.BlockId, size uint32, crc msgs.Crc, extra any) *sendArgs {
	return &sendArgs{
		blockService.Id,
		blockService.Addrs,
		&msgs.CheckBlockReq{
			BlockId: blockId,
			Size:    size,
			Crc:     crc,
		},
		nil,
		&msgs.CheckBlockResp{},
		nil,
		extra,
	}
}

// An asynchronous version of [CheckBlock] that is currently unused.
func (c *Client) StartCheckBlock(log *log.Logger, blockService *msgs.BlockService, blockId msgs.BlockId, size uint32, crc msgs.Crc, extra any, completion chan *blockCompletion) error {
	return c.checkBlockProcessors.send(log, checkBlockSendArgs(blockService, blockId, size, crc, extra), completion)
}

func (c *Client) CheckBlock(log *log.Logger, blockService *msgs.BlockService, blockId msgs.BlockId, size uint32, crc msgs.Crc) error {
	_, err := c.singleBlockReq(log, nil, &c.checkBlockProcessors, checkBlockSendArgs(blockService, blockId, size, crc, nil))
	return err
}

func (c *Client) SetFetchBlockServices() {
	var needToInitFetch = false
	func() {
		c.blockServicesLock.Lock()
		defer c.blockServicesLock.Unlock()
		needToInitFetch = !c.fetchBlockServices
		c.fetchBlockServices = true
	}()
	if needToInitFetch {
		c.refreshAddrs(c.registryConn.log)
	}
}

func (c *Client) GetFailureDomainForBlockService(blockServiceId msgs.BlockServiceId) (msgs.FailureDomain, bool) {
	c.blockServicesLock.RLock()
	defer c.blockServicesLock.RUnlock()
	if !c.fetchBlockServices {
		panic("GetFailureDomainForBlockService called and flag to keep block services information not set")
	}
	failureDomain, ok := c.blockServiceToFailureDomain[blockServiceId]
	return failureDomain, ok
}

func (c *Client) RegistryRequest(logger *log.Logger, reqBody msgs.RegistryRequest) (msgs.RegistryResponse, error) {	
	return c.registryConn.Request(reqBody)
}
