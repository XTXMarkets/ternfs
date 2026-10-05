// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"net"
	"testing"
)

type cbSecOffer uint8

const (
	offerNone cbSecOffer = iota
	offerSys
	offerGSS
)

// backCreateSessionBody is a CREATE_SESSION with flags and the given
// callback credentials, in order.
func backCreateSessionBody(clientID uint64, seq, flags uint32, offers ...cbSecOffer) []byte {
	return backCreateSessionBodyLimits(clientID, seq, flags, maxRPCMessage, 16, offers...)
}

// backCreateSessionBodyLimits is backCreateSessionBody with the back
// channel's request size and slot count given.
func backCreateSessionBodyLimits(clientID uint64, seq, flags, backReq, backReqs uint32, offers ...cbSecOffer) []byte {
	return buildCompoundBody(nil, func(w *COMPOUND4argsWriter) {
		cw := w.AppendArgarray_CreateSession()
		cw.SetCsaClientid(clientID)
		cw.SetCsaSequence(seq)
		cw.SetCsaFlags(flags)
		attrs := func(a ChannelAttrs4Writer, req, reqs uint32) []byte {
			a.SetCaMaxrequestsize(req)
			a.SetCaMaxresponsesize(maxRPCMessage)
			a.SetCaMaxresponsesizeCached(maxSessionReplyCached)
			a.SetCaMaxoperations(maxCompoundOperations)
			a.SetCaMaxrequests(reqs)
			return a.Finish()
		}
		cw.Resume(attrs(cw.StartCsaForeChanAttrs(), maxRPCMessage, 16))
		cw.Resume(attrs(cw.StartCsaBackChanAttrs(), backReq, backReqs))
		cw.SetCsaCbProgram(0x40000001)
		for _, offer := range offers {
			switch offer {
			case offerNone:
				cw.AppendCsaSecParms_None()
			case offerSys:
				aw := cw.AppendCsaSecParms_Sys()
				aw.SetStamp(7)
				aw.SetMachinename([]byte("cb-host"))
				aw.SetUid(1000)
				aw.SetGid(100)
				gw := aw.StartGids()
				aw.Resume(gw.Finish())
				cw.Resume(aw.Finish())
			case offerGSS:
				gw := cw.AppendCsaSecParms_RpcsecGss()
				gw.SetGcbpService(1)
				hw := gw.StartGcbpHandleFromServer()
				gw.Resume(hw.SetData([]byte("s")).Finish())
				hw = gw.StartGcbpHandleFromClient()
				gw.Resume(hw.SetData([]byte("c")).Finish())
				cw.Resume(gw.Finish())
			}
		}
		w.Resume(cw.Finish())
	})
}

func createWith(t *testing.T, conn net.Conn, xid *uint32, body []byte) (uint32, CREATESESSION4resok) {
	t.Helper()
	reply, err := sendRawRPC(conn, *xid, procCompound, body)
	*xid++
	if err != nil {
		t.Fatal(err)
	}
	res, err := parseCompoundReply(reply)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status() != NFS4_OK {
		return res.Status(), CREATESESSION4resok{}
	}
	iter := res.Resarray()
	iter.Next()
	return NFS4_OK, iter.Resarray().Value().AsCREATESESSION4resEntry().Value().AsCREATESESSION4resok()
}
