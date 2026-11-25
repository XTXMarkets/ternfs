// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

// parwalkTestDirs serves ReadDir from memory, standing in for the shards.
type parwalkTestDirs map[msgs.InodeId][]msgs.CurrentEdge

func (dirs parwalkTestDirs) ShardRequest(_ *log.Logger, _ msgs.ShardId, req msgs.ShardRequest, resp msgs.ShardResponse) error {
	*resp.(*msgs.ReadDirResp) = msgs.ReadDirResp{Results: dirs[req.(*msgs.ReadDirReq).DirId]}
	return nil
}

func (dirs parwalkTestDirs) fullReadDir(*log.Logger, msgs.InodeId, msgs.FullReadDirFlags, string, msgs.TernTime) (*msgs.FullReadDirResp, error) {
	panic("snapshot walks are not served")
}

func (dirs parwalkTestDirs) add(parent msgs.InodeId, name string, typ msgs.InodeType, shard msgs.ShardId) msgs.InodeId {
	id := msgs.MakeInodeId(typ, shard, uint64(len(dirs)+1))
	if typ == msgs.DIRECTORY {
		dirs[id] = nil
	}
	if parent != msgs.NULL_INODE_ID {
		dirs[parent] = append(dirs[parent], msgs.CurrentEdge{TargetId: id, Name: name})
	}
	return id
}

// parwalkTestTree has directories reached each way the walk can descend:
// inline on the parent's shard, and through another shard's queue.
//
//	/root           shard 1
//	/root/f0
//	/root/same      shard 1
//	/root/same/deep shard 1
//	/root/same/deep/f1
//	/root/other     shard 2
//	/root/other/f2
//	/root/other/back shard 1
//	/root/other/back/f3
func parwalkTestTree() (parwalkTestDirs, msgs.InodeId) {
	dirs := parwalkTestDirs{}
	root := dirs.add(msgs.NULL_INODE_ID, "root", msgs.DIRECTORY, 1)
	dirs.add(root, "f0", msgs.FILE, 1)
	same := dirs.add(root, "same", msgs.DIRECTORY, 1)
	deep := dirs.add(same, "deep", msgs.DIRECTORY, 1)
	dirs.add(deep, "f1", msgs.FILE, 1)
	other := dirs.add(root, "other", msgs.DIRECTORY, 2)
	dirs.add(other, "f2", msgs.FILE, 2)
	back := dirs.add(other, "back", msgs.DIRECTORY, 1)
	dirs.add(back, "f3", msgs.FILE, 1)
	return dirs, root
}

// parwalkTestWalk walks /root, and checks that the walk leaves no goroutines
// behind.
func parwalkTestWalk(
	t *testing.T,
	ctx context.Context,
	dirs parwalkTestDirs,
	options *ParwalkOptions,
	root msgs.InodeId,
	callback func(fullPath string) error,
) error {
	t.Helper()
	before := runtime.NumGoroutine()
	err := parwalkRunWithSeeds(
		ctx, nil, dirs, options,
		func(_ context.Context, _ msgs.InodeId, parentPath string, name string, _ msgs.TernTime, _ msgs.InodeId, _ bool, _ bool) error {
			return callback(path.Join(parentPath, name))
		},
		[]parwalkSeed{{parentId: msgs.NULL_INODE_ID, parentPath: "/", name: "root", rootId: root}},
	)
	// the workers have finished their loops, give them a moment to exit
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Errorf("walk left %d goroutines behind", runtime.NumGoroutine()-before)
			break
		}
		time.Sleep(time.Millisecond)
	}
	return err
}

func TestParwalkVisitsEveryEdgeOnce(t *testing.T) {
	dirs, root := parwalkTestTree()
	var mu sync.Mutex
	visits := map[string]int{}
	err := parwalkTestWalk(t, t.Context(), dirs, &ParwalkOptions{WorkersPerShard: 2}, root, func(fullPath string) error {
		mu.Lock()
		defer mu.Unlock()
		visits[fullPath]++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"/root":               1,
		"/root/f0":            1,
		"/root/same":          1,
		"/root/same/deep":     1,
		"/root/same/deep/f1":  1,
		"/root/other":         1,
		"/root/other/f2":      1,
		"/root/other/back":    1,
		"/root/other/back/f3": 1,
	}
	if !maps.Equal(visits, want) {
		t.Fatalf("visits = %v, want %v", visits, want)
	}
}

func TestParwalkSkipSubtree(t *testing.T) {
	dirs, root := parwalkTestTree()
	var mu sync.Mutex
	var visited []string
	err := parwalkTestWalk(t, t.Context(), dirs, &ParwalkOptions{WorkersPerShard: 2}, root, func(fullPath string) error {
		mu.Lock()
		defer mu.Unlock()
		visited = append(visited, fullPath)
		if fullPath == "/root/other" || fullPath == "/root/same/deep" {
			return ErrSkipSubtree
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(visited)
	want := []string{"/root", "/root/f0", "/root/other", "/root/same", "/root/same/deep"}
	if !slices.Equal(visited, want) {
		t.Fatalf("visited = %v, want %v", visited, want)
	}
}

func TestParwalkReturnsCallbackError(t *testing.T) {
	for _, failAt := range []string{
		"/root",               // the root, visited by the caller
		"/root/same/deep/f1",  // nested directories read inline
		"/root/other/f2",      // read by another shard's worker
		"/root/other/back/f3", // and back again
	} {
		t.Run(failAt, func(t *testing.T) {
			dirs, root := parwalkTestTree()
			callbackErr := fmt.Errorf("failed at %s", failAt)
			err := parwalkTestWalk(t, t.Context(), dirs, &ParwalkOptions{WorkersPerShard: 2}, root, func(fullPath string) error {
				if fullPath == failAt {
					return callbackErr
				}
				return nil
			})
			if !errors.Is(err, callbackErr) {
				t.Fatalf("error = %v, want %v", err, callbackErr)
			}
		})
	}
}

func TestParwalkStopsOnContextCancel(t *testing.T) {
	// A chain of directories on alternating shards, so that each one is
	// read by a worker and the walk is sequential.
	dirs := parwalkTestDirs{}
	root := dirs.add(msgs.NULL_INODE_ID, "root", msgs.DIRECTORY, 1)
	parent := root
	for depth := 1; depth <= 5; depth++ {
		parent = dirs.add(parent, fmt.Sprintf("d%d", depth), msgs.DIRECTORY, msgs.ShardId(1+depth%2))
	}

	stopErr := errors.New("stop")
	ctx, cancel := context.WithCancelCause(context.Background())
	var visited []string
	err := parwalkTestWalk(t, ctx, dirs, &ParwalkOptions{WorkersPerShard: 1}, root, func(fullPath string) error {
		visited = append(visited, fullPath)
		if fullPath == "/root/d1/d2" {
			cancel(stopErr)
		}
		return nil
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("error = %v, want %v", err, stopErr)
	}
	want := []string{"/root", "/root/d1", "/root/d1/d2"}
	if !slices.Equal(visited, want) {
		t.Fatalf("visited = %v, want %v", visited, want)
	}

	// already cancelled: nothing is visited
	err = parwalkTestWalk(t, ctx, dirs, &ParwalkOptions{WorkersPerShard: 1}, root, func(fullPath string) error {
		t.Errorf("visited %s after cancellation", fullPath)
		return nil
	})
	if !errors.Is(err, stopErr) {
		t.Fatalf("error = %v, want %v", err, stopErr)
	}
}

func TestParwalkFullQueue(t *testing.T) {
	// One shard 2 worker, held in a callback while shard 1 finds more
	// shard 2 directories than the queue can take.
	const queueSize = 16
	const numDirs = queueSize + 10
	dirs := parwalkTestDirs{}
	root := dirs.add(msgs.NULL_INODE_ID, "root", msgs.DIRECTORY, 1)
	dirs.add(dirs.add(root, "holder", msgs.DIRECTORY, 2), "hold", msgs.FILE, 2)
	for i := 0; i < numDirs; i++ {
		dirs.add(dirs.add(root, fmt.Sprintf("d%04d", i), msgs.DIRECTORY, 2), "f", msgs.FILE, 2)
	}
	firstDir := "/root/d0000"
	lastDir := fmt.Sprintf("/root/d%04d", numDirs-1)

	held := make(chan struct{})
	release := make(chan struct{})
	var mu sync.Mutex
	visits := map[string]int{}
	readByFinder := 0
	err := parwalkTestWalk(t, t.Context(), dirs, &ParwalkOptions{WorkersPerShard: 1, QueueSize: queueSize}, root, func(fullPath string) error {
		switch fullPath {
		case "/root/holder/hold":
			close(held)
			<-release
		case firstDir:
			<-held
		case lastDir:
			close(release)
		}
		mu.Lock()
		defer mu.Unlock()
		visits[fullPath]++
		if path.Base(fullPath) == "f" {
			select {
			case <-release:
			default:
				// the shard 2 worker is still held
				readByFinder++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := 1 + 2 + 2*numDirs; len(visits) != want {
		t.Fatalf("visited %d paths, want %d", len(visits), want)
	}
	for fullPath, count := range visits {
		if count != 1 {
			t.Fatalf("%s visited %d times", fullPath, count)
		}
	}
	// all but the last of the directories which did not fit in the queue
	if want := numDirs - queueSize - 1; readByFinder < want {
		t.Fatalf("%d directories read by their finder, want at least %d", readByFinder, want)
	}
}

func TestParwalkFromInode(t *testing.T) {
	file := msgs.MakeInodeId(msgs.FILE, 1, 1)
	for _, test := range []struct {
		rootPath   string
		parentPath string
		name       string
	}{
		{"/some/file.txt", "/some", "file.txt"},
		{"/some/file.txt/", "/some", "file.txt"},
		{"/some//file.txt", "/some", "file.txt"},
		{"/file.txt", "/", "file.txt"},
		{"/", "/", "/"},
	} {
		var called bool
		err := ParwalkFromInode(
			t.Context(), nil, nil, &ParwalkOptions{WorkersPerShard: 1}, file, test.rootPath,
			func(_ context.Context, parent msgs.InodeId, parentPath string, name string, _ msgs.TernTime, id msgs.InodeId, current bool, owned bool) error {
				called = true
				if parent != msgs.NULL_INODE_ID || parentPath != test.parentPath || name != test.name || id != file || !current || !owned {
					t.Errorf(
						"root %q: callback got parent=%v parentPath=%q name=%q id=%v current=%v owned=%v",
						test.rootPath, parent, parentPath, name, id, current, owned,
					)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatalf("root %q: callback was not called", test.rootPath)
		}
	}
}

type parwalkFailingClient struct {
	err error
}

func (client parwalkFailingClient) ShardRequest(*log.Logger, msgs.ShardId, msgs.ShardRequest, msgs.ShardResponse) error {
	return client.err
}

func (parwalkFailingClient) fullReadDir(*log.Logger, msgs.InodeId, msgs.FullReadDirFlags, string, msgs.TernTime) (*msgs.FullReadDirResp, error) {
	panic("snapshot walks are not served")
}

func TestParwalkReadDirErrors(t *testing.T) {
	readErr := errors.New("shard unavailable")
	logger := log.NewLogger(os.Stderr, &log.LoggerOptions{Level: log.INFO})
	for _, test := range []struct {
		name    string
		readErr error
		strict  bool
		root    bool
		wantErr bool
	}{
		{name: "missing directory ignored", readErr: msgs.DIRECTORY_NOT_FOUND, strict: true},
		{name: "missing file ignored", readErr: msgs.FILE_NOT_FOUND, strict: true},
		{name: "other errors returned", readErr: readErr, strict: true, wantErr: true},
		{name: "missing root returned without strict mode", readErr: msgs.DIRECTORY_NOT_FOUND, root: true, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := msgs.MakeInodeId(msgs.DIRECTORY, 1, 1)
			var rootIds map[msgs.InodeId]struct{}
			if test.root {
				rootIds = map[msgs.InodeId]struct{}{root: {}}
			}
			env := parwalkEnv{
				ctx:                context.Background(),
				client:             parwalkFailingClient{err: test.readErr},
				failOnReadDirError: test.strict,
				rootIds:            rootIds,
			}
			err := env.process(logger, 1, root, "/root")
			if test.wantErr && !errors.Is(err, test.readErr) {
				t.Fatalf("error = %v, want %v", err, test.readErr)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
		})
	}
}
