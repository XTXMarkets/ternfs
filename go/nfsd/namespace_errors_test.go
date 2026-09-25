// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func TestNamespaceUnstagedErrorOutcomes(t *testing.T) {
	for _, operation := range []string{"remove", "rename"} {
		for _, tc := range []struct {
			name   string
			err    error
			status uint32
		}{
			{"nonempty directory", msgs.DIRECTORY_NOT_EMPTY, NFS4ERR_NOTEMPTY},
			{"permission denied", msgs.NOT_AUTHORISED, NFS4ERR_ACCESS},
		} {
			t.Run(operation+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				if err := os.Mkdir(filepath.Join(root, "source"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "source", "child"), nil, 0600); err != nil {
					t.Fatal(err)
				}
				fs := &namespaceFaultVFS{
					TernVFS: NewLocalTernVFS(root), err: tc.err, remote: true,
				}
				store, err := NewLocalStagingStore(t.TempDir(), nil)
				if err != nil {
					t.Fatal(err)
				}
				server, err := NewServer(fs, store, nil)
				if err != nil {
					t.Fatal(err)
				}
				addr, stop := serveTestServer(t, server)
				defer stop()
				conn := dial(t, addr)
				defer conn.Close()
				res := sendCompound(t, conn, 1, func(w *COMPOUND4argsWriter) {
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
					t.Fatalf("%s = %s, want %s", operation, Nfsstat4Name(got), Nfsstat4Name(tc.status))
				}
				if _, err := os.Stat(filepath.Join(root, "source", "child")); err != nil {
					t.Fatalf("rejected mutation changed the source: %v", err)
				}
			})
		}
	}
}

func TestRemoveLocalEdgeGone(t *testing.T) {
	f := newNamespaceFixture(t)
	f.server.fs = &namespaceFaultVFS{TernVFS: f.fs, err: os.ErrNotExist, apply: true}
	if got := removeNamespaceFile(t, f, "source"); got != NFS4ERR_NOENT {
		t.Fatalf("REMOVE = %s, want NOENT", Nfsstat4Name(got))
	}
	meta, ok := f.store.GetMeta(f.id)
	if !ok || !meta.Unlinked || meta.Guarded || f.store.Failed(f.id) {
		t.Fatalf("edge-gone writer was not detached: %+v", meta)
	}
	writeFileAt(t, f.conn, &f.xid, f.fh, f.state, 0, []byte("detached bytes"))
	closeFile(t, f.conn, &f.xid, f.fh, f.state)
	if _, err := os.Stat(filepath.Join(f.root, "source")); !os.IsNotExist(err) {
		t.Fatalf("detached writer published: %v", err)
	}
}
