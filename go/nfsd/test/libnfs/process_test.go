// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux && libnfs

package libnfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

type libnfsRequest struct {
	Op     string
	Path   string
	Handle int
	Flags  int
	Data   []byte
	Offset uint64
}

type libnfsReply struct {
	Handle int
	Data   []byte
	Stat   libnfsStat
	Names  []string
	Target string
	Size   uint64
	Error  string
	Errno  int
}

func (r libnfsReply) err() error {
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}

// All libnfs calls run here, retaining the context and handles between commands.
func TestLibnfsClientProcess(t *testing.T) {
	addr := os.Getenv("TERNFS_LIBNFS_CLIENT")
	if addr == "" {
		t.Skip("subprocess helper")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	c, err := libnfsConnectAt(host, portNumber, os.Getenv("TERNFS_LIBNFS_ROOT"),
		os.Getenv("TERNFS_LIBNFS_IDENTITY"))
	encoder := json.NewEncoder(os.Stdout)
	reply := func(r libnfsReply, err error) {
		if err != nil {
			r.Error = err.Error()
			var errno syscall.Errno
			if errors.As(err, &errno) {
				r.Errno = int(errno)
			}
		}
		fmt.Fprintf(os.Stderr, "%s reply handle=%d bytes=%d errno=%d error=%q\n",
			time.Now().UTC().Format(time.RFC3339Nano), r.Handle, len(r.Data), r.Errno, r.Error)
		if err := encoder.Encode(r); err != nil {
			os.Exit(2)
		}
	}
	reply(libnfsReply{}, err)
	if err != nil {
		os.Exit(1)
	}
	decoder := json.NewDecoder(os.Stdin)
	files := make(map[int]*libnfsFile)
	nextHandle := 0
	for {
		var req libnfsRequest
		if err := decoder.Decode(&req); err != nil {
			if err != io.EOF {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			c.Close()
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "%s %s handle=%d path=%q offset=%d bytes=%d\n",
			time.Now().UTC().Format(time.RFC3339Nano), req.Op, req.Handle, req.Path, req.Offset, len(req.Data))
		var result libnfsReply
		var err error
		switch req.Op {
		case "stat":
			result.Stat, err = c.Stat(req.Path)
		case "readfile":
			result.Data, err = c.ReadFile(req.Path)
		case "readdir":
			result.Names, err = c.ReadDir(req.Path)
		case "readlink":
			result.Target, err = c.Readlink(req.Path)
		case "open":
			var f *libnfsFile
			f, err = c.OpenFile(req.Path, req.Flags)
			if err == nil {
				nextHandle++
				files[nextHandle] = f
				result.Handle = nextHandle
			}
		case "read", "write", "sync", "syncsize", "close":
			f := files[req.Handle]
			if f == nil {
				err = fmt.Errorf("unknown handle %d", req.Handle)
				break
			}
			switch req.Op {
			case "read":
				result.Data, err = f.Read()
			case "write":
				err = f.WriteAt(req.Data, req.Offset)
			case "sync":
				err = f.Sync()
			case "syncsize":
				result.Size, err = f.SyncSize()
			case "close":
				err = f.Close()
				delete(files, req.Handle)
			}
		default:
			err = fmt.Errorf("unknown operation %q", req.Op)
		}
		reply(result, err)
	}
}

type libnfsProcessClient struct {
	t        *testing.T
	process  *nfsTestProcess
	input    *json.Encoder
	stdin    io.Closer
	output   *json.Decoder
	identity string
}

func (s *nfsTestServer) client(t *testing.T) *libnfsProcessClient {
	t.Helper()
	return s.clientAt(t, "/")
}

func (s *nfsTestServer) clientAt(t *testing.T, root string) *libnfsProcessClient {
	t.Helper()
	s.clients++
	identity := fmt.Sprintf("%s-client-%d", s.Name, s.clients)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestLibnfsClientProcess$")
	cmd.Env = append(os.Environ(), "TERNFS_LIBNFS_CLIENT="+s.Addr,
		"TERNFS_LIBNFS_IDENTITY="+identity, "TERNFS_LIBNFS_ROOT="+root)
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	c := &libnfsProcessClient{
		t: t, input: json.NewEncoder(input), stdin: input,
		output: json.NewDecoder(output), identity: identity,
	}
	c.process = startNFSTestProcess(t, cmd, filepath.Join(s.Dir, identity+".log"))
	t.Cleanup(c.Close)
	if r := c.result(t); r.Error != "" {
		t.Fatalf("libnfs mount: %s (log: %s)", r.Error, c.process.Log)
	}
	return c
}

func (c *libnfsProcessClient) Close() {
	c.process.stop(c.t, syscall.SIGTERM)
	c.stdin.Close()
}

func (c *libnfsProcessClient) result(t *testing.T) libnfsReply {
	t.Helper()
	type decoded struct {
		reply libnfsReply
		err   error
	}
	done := make(chan decoded, 1)
	go func() {
		var r libnfsReply
		err := c.output.Decode(&r)
		done <- decoded{r, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("client response: %v (log: %s)", r.err, c.process.Log)
		}
		return r.reply
	case <-time.After(20 * time.Second):
		c.process.stop(t, syscall.SIGKILL)
		t.Fatalf("libnfs call timed out (log: %s)", c.process.Log)
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	return libnfsReply{}
}

func (c *libnfsProcessClient) call(t *testing.T, req libnfsRequest) libnfsReply {
	t.Helper()
	c.process.alive(t)
	if err := c.input.Encode(req); err != nil {
		t.Fatal(err)
	}
	return c.result(t)
}

func (c *libnfsProcessClient) must(t *testing.T, req libnfsRequest) libnfsReply {
	t.Helper()
	r := c.call(t, req)
	if r.Error != "" {
		t.Fatalf("%s: %s (errno %d; log: %s)", req.Op, r.Error, r.Errno, c.process.Log)
	}
	return r
}

func (c *libnfsProcessClient) open(t *testing.T, s *nfsTestServer, name string, flags int) int {
	t.Helper()
	return c.must(t, libnfsRequest{Op: "open", Path: "/" + s.Name + "/" + name, Flags: flags}).Handle
}

func (c *libnfsProcessClient) read(t *testing.T, handle int, want string) {
	t.Helper()
	r := c.must(t, libnfsRequest{Op: "read", Handle: handle})
	if string(r.Data) != want {
		t.Fatalf("read %q, want %q", r.Data, want)
	}
}

func (c *libnfsProcessClient) Stat(path string) (libnfsStat, error) {
	r := c.call(c.t, libnfsRequest{Op: "stat", Path: path})
	return r.Stat, r.err()
}

func (c *libnfsProcessClient) ReadFile(path string) ([]byte, error) {
	r := c.call(c.t, libnfsRequest{Op: "readfile", Path: path})
	return r.Data, r.err()
}

func (c *libnfsProcessClient) ReadDir(path string) ([]string, error) {
	r := c.call(c.t, libnfsRequest{Op: "readdir", Path: path})
	return r.Names, r.err()
}

func (c *libnfsProcessClient) Readlink(path string) (string, error) {
	r := c.call(c.t, libnfsRequest{Op: "readlink", Path: path})
	return r.Target, r.err()
}

type libnfsProcessFile struct {
	client *libnfsProcessClient
	handle int
}

func (c *libnfsProcessClient) OpenFile(path string, flags int) (*libnfsProcessFile, error) {
	r := c.call(c.t, libnfsRequest{Op: "open", Path: path, Flags: flags})
	if err := r.err(); err != nil {
		return nil, err
	}
	return &libnfsProcessFile{client: c, handle: r.Handle}, nil
}

func (f *libnfsProcessFile) Close() error {
	if f.handle == 0 {
		return nil
	}
	r := f.client.call(f.client.t, libnfsRequest{Op: "close", Handle: f.handle})
	f.handle = 0
	return r.err()
}

func (f *libnfsProcessFile) Read() ([]byte, error) {
	r := f.client.call(f.client.t, libnfsRequest{Op: "read", Handle: f.handle})
	return r.Data, r.err()
}

func (f *libnfsProcessFile) WriteAt(data []byte, offset uint64) error {
	r := f.client.call(f.client.t, libnfsRequest{Op: "write", Handle: f.handle, Data: data, Offset: offset})
	return r.err()
}

func (f *libnfsProcessFile) SyncSize() (uint64, error) {
	r := f.client.call(f.client.t, libnfsRequest{Op: "syncsize", Handle: f.handle})
	return r.Size, r.err()
}
