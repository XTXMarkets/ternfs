// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"

	"flag"
	"fmt"
	"io"
	mrand "math/rand"
	"net"
	"os"
	"os/signal"
	"path"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/flags"
	"xtx/ternfs/core/log"
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"
	"xtx/ternfs/ternblocks/blockservice"

	"golang.org/x/net/ipv4"
)

type blockServiceEntry struct {
	*blockservice.BlockService
	// ternblocks-specific error counters for metrics
	badBlockCrc         uint64
	blockTooOldForWrite uint64
	badCertificate      uint64
	// request counting for IO error rate alerts
	requests     uint64
	lastIoErrors uint64
	lastRequests uint64
	// alert state
	ioErrorsAlert  log.XmonNCAlert
	decommissioned bool
}

type deadBlockService struct{}

type env struct {
	bufPool        *bufpool.BufPool
	counters       map[msgs.BlocksMessageKind]*timing.Timings
	registryConn   *client.RegistryConn
	failureDomain  string
	pathPrefix     string
	ioAlertPercent uint8
}

func writeBlocksResponse(log *log.Logger, w io.Writer, resp msgs.BlocksResponse) error {
	log.Trace("writing response %T %+v", resp, resp)
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.LittleEndian, msgs.BLOCKS_RESP_PROTOCOL_VERSION); err != nil {
		return err
	}
	if _, err := buf.Write([]byte{uint8(resp.BlocksResponseKind())}); err != nil {
		return err
	}
	if err := resp.Pack(buf); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

func writeBlocksResponseError(log *log.Logger, w io.Writer, err msgs.TernError) error {
	log.Debug("writing blocks error %v", err)
	buf := bytes.NewBuffer([]byte{})
	if err := binary.Write(buf, binary.LittleEndian, msgs.BLOCKS_RESP_PROTOCOL_VERSION); err != nil {
		return err
	}
	if _, err := buf.Write([]byte{msgs.ERROR}); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(err)); err != nil {
		return err
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// handleRequestError decides whether to keep the connection alive after an error.
func handleRequestError(
	log *log.Logger,
	blockServices map[msgs.BlockServiceId]*blockServiceEntry,
	deadBlockServices map[msgs.BlockServiceId]deadBlockService,
	conn *net.TCPConn,
	lastError *error,
	blockServiceId msgs.BlockServiceId,
	req msgs.BlocksMessageKind,
	err error,
) bool {
	defer func() {
		*lastError = err
	}()

	if err == io.EOF {
		log.Debug("got EOF from %v, terminating", conn.RemoteAddr())
		return false
	}

	rootErr := err
	for {
		unwrapped := errors.Unwrap(rootErr)
		if unwrapped == nil {
			break
		}
		rootErr = unwrapped
	}

	if netErr, ok := rootErr.(net.Error); ok && netErr.Timeout() {
		log.Debug("got timeout from %v, terminating", conn.RemoteAddr())
		return false
	}
	if sysErr, ok := rootErr.(syscall.Errno); ok {
		if sysErr == syscall.EPIPE {
			log.Info("got broken pipe error from %v, terminating", conn.RemoteAddr())
			return false
		}
		if sysErr == syscall.ECONNRESET {
			log.Info("got connection reset error from %v, terminating", conn.RemoteAddr())
			return false
		}
	}

	if errors.Is(err, syscall.EIO) && blockServiceId != 0 {
		entry := blockServices[blockServiceId]
		if entry != nil && entry.ToDecommission() {
			err = msgs.BLOCK_IO_ERROR_DEVICE
		} else {
			err = msgs.BLOCK_IO_ERROR_FILE
		}
		log.ErrorNoAlert("got unxpected IO error %v from %v for req kind %v, block service %v, will return %v, previous error: %v", err, conn.RemoteAddr(), req, blockServiceId, err, *lastError)
		writeBlocksResponseError(log, conn, err.(msgs.TernError))
		return false
	}

	// In general, refuse to service requests for block services that we
	// don't have. In the case of checking a dead block, we don't emit
	// an alert, since it's expected when the scrubber is running.
	// Similarly, reading blocks from old block services can happen if
	// the cached span structure is used in the kmod.
	if _, isDead := deadBlockServices[blockServiceId]; isDead && (req == msgs.CHECK_BLOCK || req == msgs.FETCH_BLOCK || req == msgs.FETCH_BLOCK_WITH_CRC) {
		log.Info("got fetch/check block request for dead block service %v", blockServiceId)
		if ternErr, isTernErr := err.(msgs.TernError); isTernErr {
			if err := writeBlocksResponseError(log, conn, ternErr); err != nil {
				log.Info("could not write response error to %v, will terminate connection: %v", conn.RemoteAddr(), err)
				return false
			}
		}
		return true
	}

	// we always raise an alert since this is almost always bad news in the block service
	if !errors.Is(err, syscall.ENOSPC) && err != msgs.BLOCK_SERVICE_NOT_FOUND && err != msgs.BLOCK_NOT_FOUND &&
		err != msgs.BAD_BLOCK_CRC && err != msgs.BLOCK_TOO_OLD_FOR_WRITE && err != msgs.BAD_CERTIFICATE {
		log.RaiseAlertStack("", 1, "got unexpected error %v from %v for req kind %v, block service %v, previous error %v", err, conn.RemoteAddr(), req, blockServiceId, *lastError)
	}

	if ternErr, isTernErr := err.(msgs.TernError); isTernErr {
		if err := writeBlocksResponseError(log, conn, ternErr); err != nil {
			log.Info("could not write response error to %v, will terminate connection: %v", conn.RemoteAddr(), err)
			return false
		}
		// Keep the connection around using a whitelist of conditions where we know
		// that the stream is safe.
		safeError := false
		safeError = safeError || ((req == msgs.CHECK_BLOCK || req == msgs.FETCH_BLOCK || req == msgs.FETCH_BLOCK_WITH_CRC) && ternErr == msgs.BLOCK_NOT_FOUND)
		if safeError {
			log.Info("preserving connection from %v after err %v", conn.RemoteAddr(), err)
			return true
		} else {
			log.Info("not preserving connection from %v after err %v", conn.RemoteAddr(), err)
			return false
		}
	} else {
		// attempt to say goodbye, ignore errors
		writeBlocksResponseError(log, conn, msgs.INTERNAL_ERROR)
		log.Info("tearing down connection from %v after internal error %v", conn.RemoteAddr(), err)
		return false
	}
}

func readBlocksRequest(
	log *log.Logger,
	r io.Reader,
) (msgs.BlockServiceId, msgs.BlocksRequest, error) {
	var protocol uint32
	if err := binary.Read(r, binary.LittleEndian, &protocol); err != nil {
		return 0, nil, err
	}
	if protocol != msgs.BLOCKS_REQ_PROTOCOL_VERSION {
		log.RaiseAlert("bad blocks protocol, expected %v, got %v", msgs.BLOCKS_REQ_PROTOCOL_VERSION, protocol)
		return 0, nil, msgs.MALFORMED_REQUEST
	}
	var blockServiceId uint64
	if err := binary.Read(r, binary.LittleEndian, &blockServiceId); err != nil {
		return 0, nil, err
	}
	var kindByte [1]byte
	if _, err := io.ReadFull(r, kindByte[:]); err != nil {
		return 0, nil, err
	}
	kind := msgs.BlocksMessageKind(kindByte[0])
	var req msgs.BlocksRequest
	switch kind {
	case msgs.ERASE_BLOCK:
		req = &msgs.EraseBlockReq{}
	case msgs.FETCH_BLOCK:
		req = &msgs.FetchBlockReq{}
	case msgs.FETCH_BLOCK_WITH_CRC:
		req = &msgs.FetchBlockWithCrcReq{}
	case msgs.WRITE_BLOCK:
		req = &msgs.WriteBlockReq{}
	case msgs.TEST_WRITE:
		req = &msgs.TestWriteReq{}
	case msgs.CHECK_BLOCK:
		req = &msgs.CheckBlockReq{}
	default:
		log.RaiseAlert("bad blocks request kind %v", kind)
		return 0, nil, msgs.MALFORMED_REQUEST
	}
	if err := req.Unpack(r); err != nil {
		return 0, nil, err
	}
	return msgs.BlockServiceId(blockServiceId), req, nil
}

func classifyError(err error, entry *blockServiceEntry) {
	switch err {
	case msgs.BAD_BLOCK_CRC:
		atomic.AddUint64(&entry.badBlockCrc, 1)
	case msgs.BLOCK_TOO_OLD_FOR_WRITE:
		atomic.AddUint64(&entry.blockTooOldForWrite, 1)
	case msgs.BAD_CERTIFICATE:
		atomic.AddUint64(&entry.badCertificate, 1)
	}
}

// handleSingleRequest processes one request and returns whether to keep the connection.
func handleSingleRequest(
	log *log.Logger,
	env *env,
	_ chan any,
	lastError *error,
	blockServices map[msgs.BlockServiceId]*blockServiceEntry,
	deadBlockServices map[msgs.BlockServiceId]deadBlockService,
	conn *net.TCPConn,
	connectionTimeout time.Duration,
) bool {
	if connectionTimeout != 0 {
		conn.SetReadDeadline(time.Now().Add(connectionTimeout))
	}
	blockServiceId, req, err := readBlocksRequest(log, conn)
	if err != nil {
		return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, 0, 0, err)
	}
	kind := req.BlocksRequestKind()
	t := time.Now()
	defer func() {
		env.counters[kind].Add(time.Since(t))
	}()
	log.Debug("servicing request of type %T from %v", req, conn.RemoteAddr())
	log.Trace("req %+v", req)
	defer log.Debug("serviced request of type %T from %v", req, conn.RemoteAddr())
	if connectionTimeout != 0 {
		conn.SetDeadline(time.Now().Add(connectionTimeout))
	}
	entry, found := blockServices[blockServiceId]
	if !found {
		return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.BLOCK_SERVICE_NOT_FOUND)
	}
	atomic.AddUint64(&entry.requests, 1)
	bs := entry.BlockService

	switch whichReq := req.(type) {
	case *msgs.EraseBlockReq:
		proof, err := bs.EraseBlock(whichReq.BlockId, whichReq.Certificate)
		if err != nil {
			classifyError(err, entry)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if err := writeBlocksResponse(log, conn, &msgs.EraseBlockResp{Proof: proof}); err != nil {
			log.Info("could not send blocks response to %v: %v", conn.RemoteAddr(), err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}

	case *msgs.FetchBlockWithCrcReq:
		readAhead := bs.StorageClass == msgs.HDD_STORAGE
		f, byteCount, err := bs.GetBlockFileForFetch(whichReq.BlockId, whichReq.Offset, whichReq.Count, readAhead)
		if err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		defer func() {
			f.Close()
			bs.ReleaseBlockFile()
		}()
		if err := writeBlocksResponse(log, conn, &msgs.FetchBlockWithCrcResp{}); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		lf := io.LimitedReader{R: f, N: byteCount}
		read, err := conn.ReadFrom(&lf)
		if err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if read != byteCount {
			log.RaiseAlert("expected to read at least %v bytes, but only got %v for block %v", byteCount, read, whichReq.BlockId)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.INTERNAL_ERROR)
		}

	case *msgs.FetchBlockReq:
		readAhead := bs.StorageClass == msgs.HDD_STORAGE
		reader, err := bs.GetBlockReader(whichReq.BlockId, whichReq.Offset, whichReq.Count, readAhead, true)
		if err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		defer reader.Close()
		if err := writeBlocksResponse(log, conn, &msgs.FetchBlockResp{}); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		lf := io.LimitedReader{R: reader, N: int64(whichReq.Count)}
		read, err := conn.ReadFrom(&lf)
		if err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if read != int64(whichReq.Count) {
			log.RaiseAlert("expected to read at least %v bytes, but only got %v for block %v", whichReq.Count, read, whichReq.BlockId)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, msgs.INTERNAL_ERROR)
		}

	case *msgs.WriteBlockReq:
		proof, err := bs.WriteBlock(whichReq.BlockId, whichReq.Certificate, whichReq.Crc, whichReq.Size, conn)
		if err != nil {
			classifyError(err, entry)
			log.Info("could not write block: %v", err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if err := writeBlocksResponse(log, conn, &msgs.WriteBlockResp{Proof: proof}); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}

	case *msgs.CheckBlockReq:
		if err := bs.CheckBlock(whichReq.BlockId, whichReq.Size, whichReq.Crc); err != nil {
			log.Info("checking block failed, conn %v, err %v", conn.RemoteAddr(), err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if err := writeBlocksResponse(log, conn, &msgs.CheckBlockResp{}); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}

	case *msgs.TestWriteReq:
		if err := bs.TestWrite(uint32(whichReq.Size), conn); err != nil {
			log.Info("could not perform test write: %v", err)
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}
		if err := writeBlocksResponse(log, conn, &msgs.TestWriteResp{}); err != nil {
			return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, err)
		}

	default:
		return handleRequestError(log, blockServices, deadBlockServices, conn, lastError, blockServiceId, kind, fmt.Errorf("bad request type %T", req))
	}
	return true
}

func handleRequest(
	log *log.Logger,
	env *env,
	terminateChan chan any,
	blockServices map[msgs.BlockServiceId]*blockServiceEntry,
	deadBlockServices map[msgs.BlockServiceId]deadBlockService,
	conn *net.TCPConn,
	connectionTimeout time.Duration,
) {
	defer conn.Close()

	var lastError error

	for {
		keepGoing := handleSingleRequest(log, env, terminateChan, &lastError, blockServices, deadBlockServices, conn, connectionTimeout)
		if !keepGoing {
			return
		}
	}
}

var minimumRegisterInterval time.Duration = time.Second * 60
var maximumRegisterInterval time.Duration = minimumRegisterInterval * 2
var variantRegisterInterval time.Duration = maximumRegisterInterval - minimumRegisterInterval

func registerPeriodically(
	log *log.Logger,
	blockServices map[msgs.BlockServiceId]*blockServiceEntry,
	env *env,
) {
	req := msgs.RegisterBlockServicesReq{}
	alert := log.NewNCAlert(10 * time.Second)
	failureBackoff := 100 * time.Millisecond
	const maxFailureBackoff = 60 * time.Second
	registrationCount := 0
	for {
		req.BlockServices = req.BlockServices[:0]
		for _, entry := range blockServices {
			if entry.ToDecommission() || entry.decommissioned {
				continue
			}
			req.BlockServices = append(req.BlockServices, entry.RegisterBlockServiceInfo)
		}
		log.Trace("registering with %+v", req)
		_, err := env.registryConn.Request(&req)
		if err != nil {
			log.RaiseNC(alert, "could not register block services with %+v: %v", env.registryConn.RegistryAddress(), err)
			time.Sleep(failureBackoff)
			failureBackoff = min(failureBackoff*2, maxFailureBackoff)
			continue
		}
		log.ClearNC(alert)
		failureBackoff = 100 * time.Millisecond
		registrationCount++
		var waitFor time.Duration
		if registrationCount < 3 {
			waitFor = 5*time.Second + time.Duration(mrand.Uint64()%uint64((5*time.Second).Nanoseconds()))
		} else {
			waitFor = minimumRegisterInterval + time.Duration(mrand.Uint64()%uint64(variantRegisterInterval.Nanoseconds()))
		}
		log.Info("registered with %v (%v alive), waiting %v", env.registryConn.RegistryAddress(), len(blockServices), waitFor)
		time.Sleep(waitFor)
	}
}

func raiseAlerts(log *log.Logger, env *env, blockServices map[msgs.BlockServiceId]*blockServiceEntry) {
	for {
		for bsId, entry := range blockServices {
			if entry.decommissioned {
				continue
			}
			ioErrors := entry.lastIoErrors
			requests := entry.lastRequests
			entry.lastIoErrors = atomic.LoadUint64(&entry.IoErrors)
			entry.lastRequests = atomic.LoadUint64(&entry.requests)
			ioErrors = entry.lastIoErrors - ioErrors
			requests = entry.lastRequests - requests
			if requests*uint64(env.ioAlertPercent) < ioErrors*100 {
				log.Info("block service %v had %v ioErrors from %v requests in the last 5 minutes (over %d%% threshold), requesting decommission", bsId, ioErrors, requests, env.ioAlertPercent)
				_, err := env.registryConn.Request(&msgs.DecommissionBlockServiceReq{Id: entry.Id})
				if err != nil {
					log.RaiseNC(&entry.ioErrorsAlert, "block service %v had %v ioErrors from %v requests in the last 5 minutes (over %d%% threshold), decommission failed: %v", bsId, ioErrors, requests, env.ioAlertPercent, err)
				} else {
					entry.decommissioned = true
					log.ClearNC(&entry.ioErrorsAlert)
					log.Info("block service %v decommissioned successfully", entry.Id)
				}
			} else {
				log.ClearNC(&entry.ioErrorsAlert)
			}
		}
		time.Sleep(5 * time.Minute)
	}
}

type diskStats struct {
	readMs       uint64
	writeMs      uint64
	weightedIoMs uint64
}

func getDiskStats(log *log.Logger, statsPath string) (map[string]diskStats, error) {
	file, err := os.Open(statsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	ret := make(map[string]diskStats)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 11 {
			log.RaiseAlert("malformed disk entry in %v: %v", statsPath, line)
			continue
		}
		devId := fmt.Sprintf("%s:%s", fields[0], fields[1])
		readMs, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse reads ms from %s", line)
		}
		writeMs, err := strconv.ParseUint(fields[10], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse write ms from %s", line)
		}
		weightedIoMs, err := strconv.ParseUint(fields[11], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse weighted IO ms from %s", line)
		}

		ret[devId] = diskStats{
			readMs:       readMs,
			writeMs:      writeMs,
			weightedIoMs: weightedIoMs,
		}
	}
	return ret, nil
}

func sendMetrics(l *log.Logger, env *env, influxDB *log.InfluxDB, blockServices map[msgs.BlockServiceId]*blockServiceEntry, failureDomain string) {
	metrics := log.MetricsBuilder{}
	rand := wyhash.New(mrand.Uint64())
	alert := l.NewNCAlert(10 * time.Second)
	failureDomainEscaped := strings.ReplaceAll(failureDomain, " ", "-")
	for {
		diskMetrics, err := getDiskStats(l, "/proc/diskstats")
		if err != nil {
			l.RaiseAlert("failed reading diskstats: %v", err)
		}
		l.Info("sending metrics")
		metrics.Reset()
		now := time.Now()
		for bsId, entry := range blockServices {
			metrics.Measurement("eggsfs_blocks_write")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("bytes", atomic.LoadUint64(&entry.BytesWritten))
			metrics.FieldU64("blocks", atomic.LoadUint64(&entry.BlocksWritten))
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_read")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("bytes", atomic.LoadUint64(&entry.BytesFetched))
			metrics.FieldU64("blocks", atomic.LoadUint64(&entry.BlocksFetched))
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_erase")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("blocks", atomic.LoadUint64(&entry.BlocksErased))
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_check")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("blocks", atomic.LoadUint64(&entry.BlocksChecked))
			metrics.FieldU64("bytes", atomic.LoadUint64(&entry.BytesChecked))
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_errors")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.FieldU64("bad_block_crc", atomic.LoadUint64(&entry.badBlockCrc))
			metrics.FieldU64("block_too_old", atomic.LoadUint64(&entry.blockTooOldForWrite))
			metrics.FieldU64("bad_certificate", atomic.LoadUint64(&entry.badCertificate))
			metrics.Timestamp(now)

			metrics.Measurement("eggsfs_blocks_storage")
			metrics.Tag("blockservice", bsId.String())
			metrics.Tag("failuredomain", failureDomainEscaped)
			metrics.Tag("pathprefix", env.pathPrefix)
			metrics.Tag("storageclass", entry.StorageClass.String())
			metrics.FieldU64("capacity", atomic.LoadUint64(&entry.CapacityBytes))
			metrics.FieldU64("available", atomic.LoadUint64(&entry.AvailableBytes))
			metrics.FieldU64("blocks", atomic.LoadUint64(&entry.Blocks))
			metrics.FieldU64("io_errors", atomic.LoadUint64(&entry.IoErrors))
			dm, found := diskMetrics[entry.DevId]
			if found {
				metrics.FieldU64("read_ms", dm.readMs)
				metrics.FieldU64("write_ms", dm.writeMs)
				metrics.FieldU64("weighted_io_ms", dm.weightedIoMs)
			}
			metrics.Timestamp(now)
		}
		err = influxDB.SendMetrics(metrics.Payload())
		if err == nil {
			l.ClearNC(alert)
			sleepFor := time.Minute + time.Duration(rand.Uint64() & ^(uint64(1)<<63))%time.Minute
			l.Info("metrics sent, sleeping for %v", sleepFor)
			time.Sleep(sleepFor)
		} else {
			l.RaiseNC(alert, "failed to send metrics, will try again in a second: %v", err)
			time.Sleep(time.Second)
		}
	}
}

func getMountsInfo(log *log.Logger, mountsPath string) (map[string]string, error) {
	file, err := os.Open(mountsPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	ret := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		mountFields := strings.Fields(line)
		if len(mountFields) < 11 {
			log.RaiseAlert("malformed mount in %v: %v", mountsPath, line)
			continue
		}
		path := mountFields[4]
		ret[path] = mountFields[2]
	}
	return ret, nil
}

func readOrCreateKey(l *log.Logger, dir string) ([16]byte, error) {
	keyPath := path.Join(dir, blockservice.SECRET_FILE_NAME)
	f, err := os.Open(keyPath)
	if os.IsNotExist(err) {
		key := blockservice.CreateSecret(l, keyPath)
		if key == nil {
			return [16]byte{}, fmt.Errorf("failed creating secret at %s", keyPath)
		}
		return *key, nil
	}
	if err != nil {
		return [16]byte{}, fmt.Errorf("could not open key file %v: %v", keyPath, err)
	}
	defer f.Close()
	var key [16]byte
	if _, err := io.ReadFull(f, key[:]); err != nil {
		return [16]byte{}, fmt.Errorf("could not read key file %v: %v", keyPath, err)
	}
	expectedCrc := crc32c.Sum(0, key[:])
	var actualCrc uint32
	if err := binary.Read(f, binary.LittleEndian, &actualCrc); err != nil {
		return [16]byte{}, fmt.Errorf("could not read crc from key file %v: %v", keyPath, err)
	}
	if expectedCrc != actualCrc {
		return [16]byte{}, fmt.Errorf("crc mismatch in key file %v: expected %v, got %v", keyPath, expectedCrc, actualCrc)
	}
	return key, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %v DIRECTORY STORAGE_CLASS...\n\n", os.Args[0])
	description := `
For each directory/storage class pair specified we'll have one block
service. The block service id for each will be automatically generated
when running for the first time. The failure domain will be the same.

The intention is that a single blockservice process will service
a storage node.

Options:`
	description = strings.TrimSpace(description)
	fmt.Fprintln(os.Stderr, description)
	flag.PrintDefaults()
}

const PAST_CUTOFF time.Duration = 22 * time.Hour
const WRITE_FUTURE_CUTOFF time.Duration = 5 * time.Minute

func main() {
	flag.Usage = usage
	failureDomainStr := flag.String("failure-domain", "", "Failure domain")
	hostname := flag.String("hostname", "", "Hostname (for hardware event reporting)")
	pathPrefixStr := flag.String("path-prefix", "", "We filter our block service not only by failure domain but also by path prefix")

	var addresses flags.StringArrayFlags
	flag.Var(&addresses, "addr", "Addresses (up to two) to bind to, and that will be advertised to registry.")
	verbose := flag.Bool("verbose", false, "")
	xmon := flag.String("xmon", "", "Xmon address (empty for no xmon)")
	trace := flag.Bool("trace", false, "")
	logFile := flag.String("log-file", "", "If empty, stdout")
	registryAddress := flag.String("registry", "", "Registry address (host:port).")
	hardwareEventAddress := flag.String("hardwareevent", "", "Server address (host:port) to send hardware events to OR empty for no event logging")
	profileFile := flag.String("profile-file", "", "")
	syslog := flag.Bool("syslog", false, "")
	connectionTimeout := flag.Duration("connection-idle-timeout", 10*time.Minute, "Close connections idle for this long. Keepalive probes are sent at half this interval.")
	reservedStorage := flag.Uint64("reserved-storage", 100<<30, "How many bytes to reserve and under-report capacity")
	influxDBOrigin := flag.String("influx-db-origin", "", "Base URL to InfluxDB endpoint")
	influxDBOrg := flag.String("influx-db-org", "", "InfluxDB org")
	influxDBBucket := flag.String("influx-db-bucket", "", "InfluxDB bucket")
	locationId := flag.Uint("location", 10000, "Location ID")
	ioAlertPercent := flag.Uint("io-alert-percent", 10, "Threshold percent of I/O errors over which we alert")
	registryConnectionTimeout := flag.Duration("registry-connection-timeout", 10*time.Second, "")
	eraseCutoff := flag.Duration("erase-cutoff", time.Minute, "How old a block must be before it can be erased")
	dscp := flag.Uint("dscp", 0, "DSCP value to set on connections")
	maxConcurrentWrites := flag.Int("max-concurrent-writes", 0, "Max concurrent writes per block service (0=unlimited)")
	maxConcurrentReads := flag.Int("max-concurrent-reads", 0, "Max concurrent reads per block service (0=unlimited)")

	flag.Parse()
	flagErrors := false
	if flag.NArg()%2 != 0 {
		fmt.Fprintf(os.Stderr, "Malformed directory/storage class pairs.\n\n")
		flagErrors = true
	}
	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Expected at least one block service.\n\n")
		flagErrors = true
	}

	if *registryAddress == "" {
		fmt.Fprintf(os.Stderr, "You need to specify -registry.\n")
		flagErrors = true
	}

	if len(addresses) == 0 || len(addresses) > 2 {
		fmt.Fprintf(os.Stderr, "at least one -addr and no more than two needs to be provided\n")
		flagErrors = true
	}

	if *locationId > 255 {
		fmt.Fprintf(os.Stderr, "Provide valid location id\n")
		flagErrors = true
	}
	if *ioAlertPercent > 100 {
		fmt.Fprintf(os.Stderr, "io-alert-percent should not be above 100\n")
		flagErrors = true
	}

	if *failureDomainStr == "" {
		fmt.Fprintf(os.Stderr, "failure-domain can not be empty\n")
		flagErrors = true
	}

	if *pathPrefixStr == "" {
		*pathPrefixStr = *failureDomainStr
	}

	if *hardwareEventAddress != "" && *hostname == "" {
		fmt.Fprintf(os.Stderr, "-hostname must be provided if you need hardware event reporting\n")
		flagErrors = true
	}

	if *dscp > 63 {
		fmt.Fprintf(os.Stderr, "DSCP value must be between 0 and 63\n")
		flagErrors = true
	}

	var influxDB *log.InfluxDB
	if *influxDBOrigin == "" {
		if *influxDBOrg != "" || *influxDBBucket != "" {
			fmt.Fprintf(os.Stderr, "Either all or none of the -influx-db flags must be passed\n")
			flagErrors = true
		}
	} else {
		if *influxDBOrg == "" || *influxDBBucket == "" {
			fmt.Fprintf(os.Stderr, "Either all or none of the -influx-db flags must be passed\n")
			flagErrors = true
		}
		influxDB = &log.InfluxDB{
			Origin: *influxDBOrigin,
			Org:    *influxDBOrg,
			Bucket: *influxDBBucket,
		}
	}

	if flagErrors {
		usage()
		os.Exit(2)
	}

	ownIp1, port1, err := flags.ParseIPV4Addr(addresses[0])
	if err != nil {
		panic(err)
	}
	var ownIp2 [4]byte
	var port2 uint16
	if len(addresses) == 2 {
		ownIp2, port2, err = flags.ParseIPV4Addr(addresses[1])
		if err != nil {
			panic(err)
		}
	}

	var failureDomain [16]byte
	if copy(failureDomain[:], []byte(*failureDomainStr)) != len(*failureDomainStr) {
		fmt.Fprintf(os.Stderr, "Failure domain too long -- must be at most 16 characters: %v\n\n", *failureDomainStr)
		usage()
		os.Exit(2)
	}

	// create all directories first, we might need them for the log output
	for i := 0; i < flag.NArg(); i += 2 {
		dir := flag.Args()[i]
		if err := os.Mkdir(dir, 0777); err != nil && !os.IsExist(err) {
			panic(fmt.Errorf("could not create data dir %v", dir))
		}
	}

	logOut := os.Stdout
	if *logFile != "" {
		var err error
		logOut, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open log file %v: %v\n", *logFile, err)
			os.Exit(1)
		}
		defer logOut.Close()
	}
	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}
	l := log.NewLogger(logOut, &log.LoggerOptions{
		Level:                  level,
		Syslog:                 *syslog,
		XmonAddr:               *xmon,
		HardwareEventServerURL: *hardwareEventAddress,
		AppInstance:            "eggsblocks",
		AppType:                "restech_eggsfs.daytime",
		PrintQuietAlerts:       true,
	})

	if *profileFile != "" {
		f, err := os.Create(*profileFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not open profile file %v", *profileFile)
			os.Exit(1)
		}
		pprof.StartCPUProfile(f)
		stopCpuProfile := func() {
			l.Info("stopping cpu profile")
			pprof.StopCPUProfile()
		}
		defer stopCpuProfile()
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP, syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)
		go func() {
			sig := <-signalChan
			signal.Stop(signalChan)
			stopCpuProfile()
			syscall.Kill(syscall.Getpid(), sig.(syscall.Signal))
		}()
	}

	l.Info("Running block service with options:")
	l.Info("  locationId = %v", *locationId)
	l.Info("  failureDomain = %v", *failureDomainStr)
	l.Info("  pathPrefix = %v", *pathPrefixStr)
	l.Info("  addr = '%v'", addresses)
	l.Info("  logLevel = %v", level)
	l.Info("  logFile = '%v'", *logFile)
	l.Info("  registryAddress = '%v'", *registryAddress)
	l.Info("  connectionTimeout = %v", *connectionTimeout)
	l.Info("  reservedStorage = %v", *reservedStorage)
	l.Info("  registryConnectionTimeout = %v", *registryConnectionTimeout)

	bufPool := bufpool.NewBufPool()
	env := &env{
		bufPool:        bufPool,
		failureDomain:  *failureDomainStr,
		pathPrefix:     *pathPrefixStr,
		ioAlertPercent: uint8(*ioAlertPercent),
		registryConn:   client.MakeRegistryConn(l, nil, *registryAddress, 1),
	}

	// Start TCP listeners first to get actual ports
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
		},
	}

	listener1, err := lc.Listen(context.Background(), "tcp4", fmt.Sprintf("%v:%v", net.IP(ownIp1[:]), port1))
	if err != nil {
		panic(err)
	}
	defer listener1.Close()

	l.Info("running 1 on %v", listener1.Addr())
	actualPort1 := uint16(listener1.Addr().(*net.TCPAddr).Port)

	var listener2 net.Listener
	var actualPort2 uint16
	if len(addresses) == 2 {
		listener2, err = lc.Listen(context.Background(), "tcp4", fmt.Sprintf("%v:%v", net.IP(ownIp2[:]), port2))
		if err != nil {
			panic(err)
		}
		defer listener2.Close()

		l.Info("running 2 on %v", listener2.Addr())
		actualPort2 = uint16(listener2.Addr().(*net.TCPAddr).Port)
	}

	addrs := msgs.AddrsInfo{
		Addr1: msgs.IpPort{Addrs: ownIp1, Port: actualPort1},
		Addr2: msgs.IpPort{Addrs: ownIp2, Port: actualPort2},
	}

	mountsInfo, err := getMountsInfo(l, "/proc/self/mountinfo")
	if err != nil {
		l.RaiseAlert("Disk stats for mounted paths will not be collected due to failure collecting mount info: %v", err)
	}

	bsOptions := &blockservice.BlockServiceOptions{
		BufferSize:            1 << 20,
		MaxConcurrentWrites:   *maxConcurrentWrites,
		MaxConcurrentReads:    *maxConcurrentReads,
		FutureCutoff:          WRITE_FUTURE_CUTOFF,
		PastCutoff:            PAST_CUTOFF,
		EraseCutoff:           *eraseCutoff,
		ReservedCapacityBytes: *reservedStorage,
	}

	// Create block services
	blockServices := make(map[msgs.BlockServiceId]*blockServiceEntry)
	var failedBlockServiceCount int
	for i := 0; i < flag.NArg(); i += 2 {
		dir := flag.Args()[i]
		storageClass := msgs.StorageClassFromString(flag.Args()[i+1])
		if storageClass == msgs.EMPTY_STORAGE || storageClass == msgs.INLINE_STORAGE {
			fmt.Fprintf(os.Stderr, "Storage class cannot be EMPTY/INLINE")
			os.Exit(2)
		}
		key, err := readOrCreateKey(l, dir)
		if err != nil {
			l.Info("%v", err)
			failedBlockServiceCount++
			continue
		}
		id := blockservice.BlockServiceIdFromKey(key)
		devId, found := mountsInfo[dir]
		if !found {
			devId = ""
		}
		bsInfo := msgs.RegisterBlockServiceInfo{
			Id:           id,
			LocationId:   msgs.Location(*locationId),
			Addrs:        addrs,
			StorageClass: storageClass,
			SecretKey:    key,
			Path:         dir,
		}
		if len(env.pathPrefix) > 0 {
			bsInfo.Path = fmt.Sprintf("%s:%s", env.pathPrefix, dir)
		}
		bsInfo.FailureDomain.Name = failureDomain

		bs := blockservice.OpenBlockService(l, bsOptions, bufPool, bsInfo, devId)
		blockServices[id] = &blockServiceEntry{
			BlockService:  bs,
			ioErrorsAlert: *l.NewNCAlert(time.Second),
		}
	}
	for id, entry := range blockServices {
		l.Info("block service %v at %v, storage class %v", id, entry.LocalPath(), entry.StorageClass)
	}

	if len(blockServices)+failedBlockServiceCount != flag.NArg()/2 {
		panic(fmt.Errorf("duplicate block services"))
	}

	// Wait for all block services to become active
	for id, entry := range blockServices {
		deadline := time.Now().Add(2 * time.Minute)
		for !entry.Active() {
			if time.Now().After(deadline) {
				panic(fmt.Errorf("timed out waiting for block service %v to become active", id))
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	l.Info("all block services active")

	// Now ask registry for block services we _had_ before. We need to know this to honor
	// erase block requests for old block services safely.
	deadBlockServices := make(map[msgs.BlockServiceId]deadBlockService)
	{
		var registryBlockServices []msgs.FullBlockServiceInfo
		{
			alert := l.NewNCAlert(time.Minute)
			l.RaiseNC(alert, "fetching block services")
			for {
				resp, err := env.registryConn.Request(&msgs.ChangedBlockServicesReq{})
				if err != nil {
					l.RaiseNC(alert, "could not request block services from registry: %v", err)
					time.Sleep(time.Second)
					continue
				}
				registryBlockServices = resp.(*msgs.ChangedBlockServicesResp).BlockServices
				break
			}
			l.ClearNC(alert)
		}
		for i := range registryBlockServices {
			bs := &registryBlockServices[i]
			_, weHaveBs := blockServices[bs.Id]
			sameFailureDomain := bs.FailureDomain.Name == failureDomain
			if len(env.pathPrefix) > 0 {
				pathParts := strings.Split(bs.Path, ":")
				if len(pathParts) == 2 {
					sameFailureDomain = pathParts[0] == env.pathPrefix
				}
			}
			isDecommissioned := (bs.Flags & msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED) != 0
			if weHaveBs && !sameFailureDomain {
				panic(fmt.Errorf("we have block service %v, and we're failure domain %v, but registry thinks it should be failure domain %v. If you've moved this block service, change the failure domain on registry", bs.Id, failureDomain, bs.FailureDomain))
			}
			if !weHaveBs && sameFailureDomain {
				if !isDecommissioned {
					panic(fmt.Errorf("registry has block service %v for our failure domain %v, but we don't have this block service, and it is not decommissioned. If the block service is dead, mark it as decommissioned", bs.Id, failureDomain))
				}
				deadBlockServices[bs.Id] = deadBlockService{}
			}
			if weHaveBs && isDecommissioned {
				l.ErrorNoAlert("We have block service %v, which is decommissioned according to registry. We will treat it as if it doesn't exist.", bs.Id)
				entry := blockServices[bs.Id]
				entry.Close()
				delete(blockServices, bs.Id)
				deadBlockServices[bs.Id] = deadBlockService{}
			}
		}
	}

	terminateChan := make(chan any)

	env.counters = make(map[msgs.BlocksMessageKind]*timing.Timings)
	for _, k := range msgs.AllBlocksMessageKind {
		env.counters[k] = timing.NewTimings(40, 100*time.Microsecond, 1.5)
	}

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		registerPeriodically(l, blockServices, env)
	}()

	if influxDB != nil {
		go func() {
			defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
			sendMetrics(l, env, influxDB, blockServices, *failureDomainStr)
		}()
	}

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		raiseAlerts(l, env, blockServices)
	}()

	setupConn := func(conn net.Conn) {
		tcpConn := conn.(*net.TCPConn)
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(*connectionTimeout / 2)
	}

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		for {
			conn, err := listener1.Accept()
			l.Trace("new conn %+v", conn)
			if err != nil {
				terminateChan <- err
				return
			}

			if *dscp != 0 {
				ipv4Conn := ipv4.NewConn(conn)
				currentDSCP, err := ipv4Conn.TOS()
				if err != nil {
					terminateChan <- err
					return
				}
				if currentDSCP>>2 == 0 {
					err = ipv4Conn.SetTOS(int(*dscp) << 2)
					if err != nil {
						terminateChan <- err
						return
					}
				}
			}
			setupConn(conn)
			go func() {
				defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
				handleRequest(l, env, terminateChan, blockServices, deadBlockServices, conn.(*net.TCPConn), *connectionTimeout)
			}()
		}
	}()
	if listener2 != nil {
		go func() {
			defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
			for {
				conn, err := listener2.Accept()
				l.Trace("new conn %+v", conn)
				if err != nil {
					terminateChan <- err
					return
				}
				setupConn(conn)
				go func() {
					defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
					handleRequest(l, env, terminateChan, blockServices, deadBlockServices, conn.(*net.TCPConn), *connectionTimeout)
				}()
			}
		}()
	}

	{
		err := <-terminateChan
		if err != nil {
			panic(err)
		}
	}
}
