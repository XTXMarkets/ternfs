// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"
)

// A low-level utility for directly communication with block services.
//
// Currently this is not used by the main [Client] library at all.
func BlockServiceConnection(log *log.Logger, addrs msgs.AddrsInfo) (*net.TCPConn, error) {
	if addrs.Addr1.Port == 0 {
		panic(fmt.Errorf("ip1/port1 must be provided"))
	}
	var ips [2][4]byte
	ips[0] = addrs.Addr1.Addrs
	ips[1] = addrs.Addr2.Addrs
	var ports [2]uint16
	ports[0] = addrs.Addr1.Port
	ports[1] = addrs.Addr2.Port
	var errs [2]error
	var sock *net.TCPConn
	start := int(rand.Uint32())
	for i := start; i < start+2; i++ {
		ip := net.IP(ips[i%2][:])
		port := int(ports[i%2])
		if port == 0 {
			continue
		}
		sock, errs[i%2] = net.DialTCP("tcp4", nil, &net.TCPAddr{IP: ip, Port: port})
		if errs[i%2] == nil {
			return sock, nil
		}
		log.RaiseAlert("could not connect to block service %v:%v: %v, might try other ip/port", ip, port, errs[i%2])
	}
	// return one of the two errors, we don't want to mess with them too much and they are alerts
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	panic("impossible")
}

func writeBlocksRequest(log *log.Logger, w io.Writer, blockServiceId msgs.BlockServiceId, req msgs.BlocksRequest) error {
	// log.Debug("writing blocks request %v for block service id %v: %+v", req.BlocksRequestKind(), blockServiceId, req)
	if err := binary.Write(w, binary.LittleEndian, msgs.BLOCKS_REQ_PROTOCOL_VERSION); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, blockServiceId); err != nil {
		return err
	}
	if _, err := w.Write([]byte{uint8(req.BlocksRequestKind())}); err != nil {
		return err
	}
	if err := req.Pack(w); err != nil {
		return err
	}
	return nil
}

func readBlocksResponse(
	log *log.Logger,
	r io.Reader,
	resp msgs.BlocksResponse,
) error {
	var protocol uint32
	if err := binary.Read(r, binary.LittleEndian, &protocol); err != nil {
		log.Info("could not read protocol: %v", err)
		return err
	}
	if protocol != msgs.BLOCKS_RESP_PROTOCOL_VERSION {
		return fmt.Errorf("bad blocks protocol, expected %v, got %v", msgs.BLOCKS_RESP_PROTOCOL_VERSION, protocol)
	}
	var kindByte [1]byte
	if _, err := io.ReadFull(r, kindByte[:]); err != nil {
		log.Info("could not read kind: %v", err)
		return err
	}
	if kindByte[0] == msgs.ERROR {
		var err uint16
		if err := binary.Read(r, binary.LittleEndian, &err); err != nil {
			log.Info("could not read error: %v", err)
			return err
		}
		return msgs.TernError(err)
	}
	kind := msgs.BlocksMessageKind(kindByte[0])
	if kind != resp.BlocksResponseKind() {
		return fmt.Errorf("bad blocks response kind %v, expected %v", kind, resp.BlocksResponseKind())
	}
	if err := resp.Unpack(r); err != nil {
		log.Info("could not unpack response: %v", err)
		return err
	}
	return nil
}

func WriteBlock(
	logger *log.Logger,
	conn interface {
		io.ReaderFrom
		io.Reader
		io.Writer
	},
	block *msgs.AddSpanInitiateBlockInfo,
	r io.Reader, // only `size` bytes will be read
	size uint32,
	crc msgs.Crc,
) ([8]byte, error) {
	logger.Debug("writing block %+v, size %v, CRC %v", block, size, crc)
	var proof [8]byte
	err := writeBlocksRequest(logger, conn, block.BlockServiceId, &msgs.WriteBlockReq{
		BlockId:     block.BlockId,
		Crc:         crc,
		Size:        size,
		Certificate: block.Certificate,
	})
	if err != nil {
		return proof, err
	}
	logger.Debug("sent block request, starting to write data")
	lr := io.LimitedReader{
		R: r,
		N: int64(size),
	}
	if _, err := conn.ReadFrom(&lr); err != nil {
		logger.Info("could not write block data to: %v", err)
		return proof, err
	}
	logger.Debug("data written, getting proof")
	resp := msgs.WriteBlockResp{}
	if err := readBlocksResponse(logger, conn, &resp); err != nil {
		return proof, err
	}
	logger.Debug("proof received")
	proof = resp.Proof
	return proof, nil
}

// Won't actually fetch the block -- it'll be readable from `conn` as this function terminates.
// Note that this function will _not_ check the CRC of the block! You should probably do that
// before using the block in any meaningful way.
func FetchBlock(
	logger *log.Logger,
	conn interface {
		io.Reader
		io.Writer
	},
	blockService *msgs.BlockService,
	blockId msgs.BlockId,
	offset uint32,
	count uint32,
) error {
	logger.Debug("fetching block %v from block service %+v, offset=%v, count=%v", blockId, blockService, offset, count)
	req := msgs.FetchBlockReq{
		BlockId: blockId,
		Offset:  offset,
		Count:   count,
	}
	if err := writeBlocksRequest(logger, conn, blockService.Id, &req); err != nil {
		return err
	}
	resp := msgs.FetchBlockResp{}
	if err := readBlocksResponse(logger, conn, &resp); err != nil {
		return err
	}
	return nil
}

func EraseBlock(
	logger *log.Logger,
	conn interface {
		io.Writer
		io.Reader
	},
	block *msgs.RemoveSpanInitiateBlockInfo,
) ([8]byte, error) {
	logger.Debug("erasing block %+v", block)
	var proof [8]byte
	req := msgs.EraseBlockReq{
		BlockId:     block.BlockId,
		Certificate: block.Certificate,
	}
	if err := writeBlocksRequest(logger, conn, block.BlockServiceId, &req); err != nil {
		return proof, err
	}
	resp := msgs.EraseBlockResp{}
	if err := readBlocksResponse(logger, conn, &resp); err != nil {
		return proof, err
	}
	proof = resp.Proof
	return proof, nil
}

func TestWrite(
	logger *log.Logger,
	conn interface {
		io.ReaderFrom
		io.Reader
		io.Writer
	},
	bs msgs.BlockServiceId,
	r io.Reader, // only `size` bytes will be read
	size uint64,
) error {
	logger.Debug("writing request")
	err := writeBlocksRequest(logger, conn, bs, &msgs.TestWriteReq{Size: size})
	if err != nil {
		return err
	}
	logger.Debug("starting to write test data")
	lr := io.LimitedReader{
		R: r,
		N: int64(size),
	}
	if _, err := conn.ReadFrom(&lr); err != nil {
		logger.Info("could not write test data to: %v", err)
		return err
	}
	logger.Debug("data written, getting response")
	resp := msgs.TestWriteResp{}
	if err := readBlocksResponse(logger, conn, &resp); err != nil {
		return err
	}
	logger.Debug("response received")
	return nil
}
