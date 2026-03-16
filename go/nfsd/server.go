// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"net"
	"time"
)

// peekCompoundHeader extracts the tag and minor version from COMPOUND4args
// without fully parsing the argarray. The wire format is: tag (XDR opaque:
// u32 len + data padded to 4 bytes), then u32 minorversion.
func peekCompoundHeader(body []byte) (tag []byte, minor uint32, ok bool) {
	if len(body) < 4 {
		return nil, 0, false
	}
	tagLen := binary.BigEndian.Uint32(body[0:4])
	padded := (tagLen + 3) &^ 3 // XDR pad to 4-byte boundary
	off := 4 + padded
	if uint64(off)+4 > uint64(len(body)) {
		return nil, 0, false
	}
	return body[4 : 4+tagLen], binary.BigEndian.Uint32(body[off : off+4]), true
}

// Server is the NFSv4 server.
type Server struct {
	fs            TernVFS
	clients       *ClientStore
	stagingStore  StagingStore
	writeVerifier [8]byte       // random per server instance, changes on restart
	idleTimeout   time.Duration // connection idle timeout
	log           *slog.Logger
}

func NewServer(fs TernVFS, stagingStore StagingStore, logger *slog.Logger) (*Server, error) {
	clients, err := NewClientStore(fs)
	if err != nil {
		return nil, err
	}
	var verf [8]byte
	rand.Read(verf[:])
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		fs:            fs,
		clients:       clients,
		stagingStore:  stagingStore,
		writeVerifier: verf,
		idleTimeout:   5 * time.Minute,
		log:           logger,
	}
	return s, nil
}

func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.log.Error("accept failed", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	remote := conn.RemoteAddr().String()
	s.log.Info("client connected", "remote", remote)
	for {
		if s.idleTimeout > 0 {
			conn.SetDeadline(time.Now().Add(s.idleTimeout))
		}
		frame, err := readFrame(conn)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				s.log.Info("client idle timeout", "remote", remote)
			} else {
				s.log.Info("client disconnected", "remote", remote, "err", err)
			}
			return
		}
		req, err := parseRPCCall(frame)
		if err != nil {
			s.log.Warn("RPC parse error", "remote", remote, "err", err)
			continue
		}
		s.log.Debug("RPC call", "remote", remote, "xid", req.xid,
			"prog", req.prog, "vers", req.vers, "proc", req.proc)
		var reply []byte
		if req.prog != nfsProg || req.vers != nfsVersion {
			s.log.Debug("prog/vers mismatch", "prog", req.prog, "vers", req.vers)
			reply = buildRPCReply(req.xid, acceptProgMismatch)
			reply = binary.BigEndian.AppendUint32(reply, nfsVersion)
			reply = binary.BigEndian.AppendUint32(reply, nfsVersion)
		} else {
			switch req.proc {
			case procNull:
				s.log.Debug("NULL")
				reply = buildRPCReply(req.xid, acceptSuccess)
			case procCompound:
				reply = s.handleCompound(req)
			default:
				s.log.Debug("unknown proc", "proc", req.proc)
				reply = buildRPCReply(req.xid, acceptProcUnavail)
			}
		}
		if err := writeFrame(conn, reply); err != nil {
			return
		}
	}
}

// compoundState tracks the current/saved filehandle during COMPOUND execution.
type compoundState struct {
	currentID    InodeID
	currentIDSet bool
	savedID      InodeID
	savedIDSet   bool
}

func (s *Server) handleCompound(req *rpcRequest) []byte {
	// Pre-check minor version before full parse, since ReadCOMPOUND4args
	// eagerly walks the argarray and will fail on unknown v4.1+ opcodes.
	if tag, minor, ok := peekCompoundHeader(req.body); ok && minor != 0 {
		s.log.Debug("minor version not supported", "minor", minor)
		reply := buildRPCReply(req.xid, acceptSuccess)
		// Build COMPOUND4res: status + echoed tag + 0 ops.
		reply = binary.BigEndian.AppendUint32(reply, NFS4ERR_MINOR_VERS_MISMATCH)
		// Echo tag as XDR opaque (u32 len + data + padding).
		reply = binary.BigEndian.AppendUint32(reply, uint32(len(tag)))
		reply = append(reply, tag...)
		if pad := (4 - len(tag)%4) % 4; pad > 0 {
			reply = append(reply, make([]byte, pad)...)
		}
		reply = binary.BigEndian.AppendUint32(reply, 0) // resarray count
		return reply
	}

	args, ok := ReadCOMPOUND4args(req.body)
	if !ok {
		s.log.Debug("COMPOUND: garbage args")
		reply := buildRPCReply(req.xid, acceptGarbageArgs)
		return reply
	}

	reply := buildRPCReply(req.xid, acceptSuccess)

	w := StartCOMPOUND4res(reply)
	w.SetStatus(NFS4_OK)

	// Echo the tag.
	tagW := w.StartTag()
	tagData := args.Tag().Data()
	reply = tagW.SetData(tagData).Finish()
	w.Resume(reply)

	st := &compoundState{}
	overallStatus := NFS4_OK
	opCount := 0

	iter := args.Argarray()
	for iter.Next() {
		entry := iter.Argarray()
		opnum := entry.Disc()
		op := entry.Value()

		s.log.Debug("op", "opnum", NfsOpnum4Name(opnum))

		var opStatus uint32
		switch opnum {
		case OP_ACCESS:
			opStatus = s.opAccess(op.AsACCESS4args(), st, &w)
		case OP_CLOSE:
			opStatus = s.opClose(op.AsCLOSE4args(), st, &w)
		case OP_COMMIT:
			opStatus = s.opCommit(st, &w)
		case OP_CREATE:
			opStatus = s.opCreate(op.AsCREATE4args(), st, &w)
		case OP_DELEGPURGE:
			opStatus = s.opDelegpurge(&w)
		case OP_DELEGRETURN:
			opStatus = s.opDelegreturn(st, &w)
		case OP_GETATTR:
			opStatus = s.opGetattr(op.AsGETATTR4args(), st, &w)
		case OP_GETFH:
			opStatus = s.opGetfh(st, &w)
		case OP_LINK:
			opStatus = s.opLink(st, &w)
		case OP_LOCK:
			opStatus = s.opLock(&w)
		case OP_LOCKT:
			opStatus = s.opLockt(&w)
		case OP_LOCKU:
			opStatus = s.opLocku(&w)
		case OP_LOOKUP:
			opStatus = s.opLookup(op.AsLOOKUP4args(), st, &w)
		case OP_LOOKUPP:
			opStatus = s.opLookupp(st, &w)
		case OP_NVERIFY:
			opStatus = s.opNverify(op.AsNVERIFY4args(), st, &w)
		case OP_OPEN:
			opStatus = s.opOpen(op.AsOPEN4args(), st, &w)
		case OP_OPEN_CONFIRM:
			opStatus = s.opOpenConfirm(op.AsOPENCONFIRM4args(), st, &w)
		case OP_OPEN_DOWNGRADE:
			opStatus = s.opOpenDowngrade(st, &w)
		case OP_OPENATTR:
			opStatus = s.opOpenattr(&w)
		case OP_PUTFH:
			opStatus = s.opPutfh(op.AsPUTFH4args(), st, &w)
		case OP_PUTPUBFH:
			opStatus = s.opPutrootfh(st, &w, true)
		case OP_PUTROOTFH:
			opStatus = s.opPutrootfh(st, &w, false)
		case OP_READ:
			opStatus = s.opRead(op.AsREAD4args(), st, &w)
		case OP_READDIR:
			opStatus = s.opReaddir(op.AsREADDIR4args(), st, &w)
		case OP_READLINK:
			opStatus = s.opReadlink(st, &w)
		case OP_REMOVE:
			opStatus = s.opRemove(op.AsREMOVE4args(), st, &w)
		case OP_RENAME:
			opStatus = s.opRename(op.AsRENAME4args(), st, &w)
		case OP_RENEW:
			opStatus = s.opRenew(op.AsRENEW4args(), &w)
		case OP_RESTOREFH:
			opStatus = s.opRestorefh(st, &w)
		case OP_SAVEFH:
			opStatus = s.opSavefh(st, &w)
		case OP_SECINFO:
			opStatus = s.opSecinfo(st, &w)
		case OP_SETATTR:
			opStatus = s.opSetattr(op.AsSETATTR4args(), st, &w)
		case OP_SETCLIENTID:
			opStatus = s.opSetclientid(op.AsSETCLIENTID4args(), &w)
		case OP_SETCLIENTID_CONFIRM:
			opStatus = s.opSetclientidConfirm(op.AsSETCLIENTIDCONFIRM4args(), &w)
		case OP_VERIFY:
			opStatus = s.opVerify(op.AsVERIFY4args(), st, &w)
		case OP_WRITE:
			opStatus = s.opWrite(op.AsWRITE4args(), st, &w)
		case OP_RELEASE_LOCKOWNER:
			opStatus = s.opReleaseLockowner(&w)
		default:
			ew := w.AppendResarray_Illegal()
			ew.SetStatus(NFS4ERR_OP_ILLEGAL)
			opStatus = NFS4ERR_OP_ILLEGAL
		}

		opCount++

		if opStatus != NFS4_OK {
			s.log.Debug("op failed", "op", NfsOpnum4Name(opnum), "status", Nfsstat4Name(opStatus))
			overallStatus = opStatus
			break
		}
	}

	s.log.Debug("COMPOUND done", "status", Nfsstat4Name(overallStatus), "ops", opCount)

	w.SetStatus(overallStatus)
	return w.Finish()
}
