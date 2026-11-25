// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/staging"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

func TestNamespaceUnstagedErrorOutcomes(t *testing.T) {
	for _, operation := range []string{"remove", "rename"} {
		for _, tc := range []struct {
			name   string
			err    error
			status uint32
		}{
			{"nonempty directory", msgs.DIRECTORY_NOT_EMPTY, nfs4.NFS4ERR_NOTEMPTY},
			{"permission denied", msgs.NOT_AUTHORISED, nfs4.NFS4ERR_ACCESS},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				if err := os.Mkdir(filepath.Join(root, "source"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "source", "child"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				base := vfs.NewLocalTernVFS(root)
				store, err := staging.NewLocalStore(t.TempDir(), nil)
				if err != nil {
					t.Fatal(err)
				}
				server, err := NewServer(base, store, nil)
				if err != nil {
					t.Fatal(err)
				}
				// The fault applies to namespace operations only, as in the
				// other namespace fixtures.
				server.fs = &namespaceFaultVFS{TernVFS: base, err: tc.err, remote: true}
				addr, stop := serveTestServer(t, server)
				defer stop()
				conn := dial(t, addr)
				defer conn.Close()
				res := sendCompound(t, conn, 1, func(w *nfs4.COMPOUND4argsWriter) {
					w.AppendArgarray_Putrootfh()
					if operation == "remove" {
						rw := w.AppendArgarray_Remove()
						rw.Resume(rw.StartTarget().SetData([]byte("source")).Finish())
						w.Resume(rw.Finish())
					} else {
						w.AppendArgarray_Savefh()
						rw := w.AppendArgarray_Rename()
						rw.Resume(rw.StartOldname().SetData([]byte("source")).Finish())
						rw.Resume(rw.StartNewname().SetData([]byte("target")).Finish())
						w.Resume(rw.Finish())
					}
				})
				if got := res.Status(); got != tc.status {
					t.Fatalf("%s = %s, want %s", operation, nfs4.Nfsstat4Name(got), nfs4.Nfsstat4Name(tc.status))
				}
				if _, err := os.Stat(filepath.Join(root, "source", "child")); err != nil {
					t.Fatalf("rejected mutation changed the source: %v", err)
				}
			})
		}
	}
}
