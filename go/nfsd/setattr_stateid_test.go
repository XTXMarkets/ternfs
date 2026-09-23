// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetattrTimeWithInvalidStateID(t *testing.T) {
	for _, withSize := range []bool{false, true} {
		name := "time only"
		if withSize {
			name = "time and size"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "file")
			if err := os.WriteFile(path, []byte("data"), 0600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			addr, stop := startTestServer(t, root)
			defer stop()
			conn := dial(t, addr)
			defer conn.Close()
			xid := uint32(1)
			fh := lookupFH(t, conn, &xid, "file")
			target := time.Unix(1_600_000_000, 0)
			res := sendCompound(t, conn, xid, func(w *COMPOUND4argsWriter) {
				pw := w.AppendArgarray_Putfh()
				pw.Resume(pw.StartObject().SetData(fh).Finish())
				w.Resume(pw.Finish())
				sw := w.AppendArgarray_Setattr()
				sw.Stateid().SetSeqid(1)
				for i := range 12 {
					sw.Stateid().SetOther(i, 0xab)
				}
				aw := sw.StartObjAttributes()
				bw := aw.StartAttrmask()
				var data []byte
				if withSize {
					bw.AppendData(1 << FATTR4_SIZE)
					data = binary.BigEndian.AppendUint64(data, 1)
				} else {
					bw.AppendData(0)
				}
				bw.AppendData(1 << (FATTR4_TIME_MODIFY_SET - 32))
				aw.Resume(bw.Finish())
				data = binary.BigEndian.AppendUint32(data, SET_TO_CLIENT_TIME4)
				data = binary.BigEndian.AppendUint64(data, uint64(target.Unix()))
				data = binary.BigEndian.AppendUint32(data, 0)
				aw.Resume(aw.StartAttrVals().SetData(data).Finish())
				sw.Resume(aw.Finish())
				w.Resume(sw.Finish())
			})
			wantStatus := uint32(NFS4_OK)
			wantTime := target
			if withSize {
				wantStatus = NFS4ERR_BAD_STATEID
				wantTime = before.ModTime()
			}
			if res.Status() != wantStatus {
				t.Fatalf("SETATTR = %s, want %s", Nfsstat4Name(res.Status()), Nfsstat4Name(wantStatus))
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !after.ModTime().Equal(wantTime) || after.Size() != before.Size() {
				t.Fatalf("SETATTR left mtime=%v size=%d, want %v and %d",
					after.ModTime(), after.Size(), wantTime, before.Size())
			}
		})
	}
}
