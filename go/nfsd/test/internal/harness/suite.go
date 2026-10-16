// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

type Options struct {
	Cluster  bool
	Registry string
	Binaries string
	Nfsd     string
	Output   string
}

func (o *Options) Flags(f *flag.FlagSet) {
	f.BoolVar(&o.Cluster, "cluster", false, "start a temporary ternrun cluster")
	f.StringVar(&o.Registry, "registry", "", "existing TernFS registry")
	f.StringVar(&o.Binaries, "binaries-dir", "", "prebuilt backend binaries for ternrun; omit to build them")
	f.StringVar(&o.Nfsd, "nfsd", "", "nfsd binary to test; omit to build the current source")
	f.StringVar(&o.Output, "artifacts-dir", "", "parent directory for run artifacts; failed runs are retained")
}

// SourcePath keeps Makefile arguments relative to go/nfsd/test after Go
// changes the working directory to a test package.
func SourcePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../..", name))
}

type Suite struct {
	Dir      string
	Registry string
	Binary   string
	backend  *Process
	servers  []*Server
}

func New(ctx context.Context, opts Options) (_ *Suite, err error) {
	if opts.Cluster == (opts.Registry != "") {
		return nil, fmt.Errorf("select -cluster or -registry HOST:PORT")
	}
	if opts.Registry != "" && opts.Binaries != "" {
		return nil, fmt.Errorf("-binaries-dir configures a local cluster; use -registry alone for an existing one")
	}
	if opts.Output != "" {
		opts.Output = SourcePath(opts.Output)
	}
	dir, err := os.MkdirTemp(opts.Output, "nfsd-integration-")
	if err != nil {
		return nil, err
	}
	s := &Suite{Dir: dir, Registry: opts.Registry}
	fmt.Printf("artifacts: %s\n", dir)
	defer func() {
		if err != nil {
			err = errors.Join(err, s.Close(false))
		}
	}()
	build := func(name, pkg string) (string, error) {
		binary := filepath.Join(dir, name)
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, pkg)
		cmd.Dir = SourcePath("..")
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("build %s: %w\n%s", name, err, output)
		}
		return binary, nil
	}
	if opts.Nfsd == "" {
		s.Binary, err = build("nfsd", ".")
		if err != nil {
			return nil, err
		}
	} else {
		s.Binary = SourcePath(opts.Nfsd)
	}
	if opts.Cluster {
		socket, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		s.Registry = socket.LocalAddr().String()
		_, port, _ := net.SplitHostPort(s.Registry)
		socket.Close()
		args := []string{"-data-dir", filepath.Join(dir, "backend"), "-no-fuse",
			"-leader-only", "-registry-bincode-port", port, "-registry-http-port=0"}
		timeout := 90 * time.Second
		if opts.Binaries != "" {
			args = append(args, "-binaries-dir", SourcePath(opts.Binaries))
		} else {
			timeout = 30 * time.Minute
		}
		binary, err := build("ternrun", "../ternrun")
		if err != nil {
			return nil, err
		}
		s.backend, err = StartProcess(exec.Command(binary, args...), filepath.Join(dir, "ternrun.log"))
		if err != nil {
			return nil, err
		}
		// ternrun gives its children 20 seconds to stop before killing them.
		s.backend.Grace = 30 * time.Second
		if err := WaitFor(ctx, "ternrun readiness", timeout, func() (bool, error) {
			if err := s.backend.Check(); err != nil {
				return false, err
			}
			output, err := os.ReadFile(s.backend.Log)
			return bytes.Contains(output, []byte("operational\n")), err
		}); err != nil {
			return nil, err
		}
	}
	fmt.Printf("nfsd=%s registry=%s\n", s.Binary, s.Registry)
	return s, nil
}

func (s *Suite) Close(success bool) error {
	var err error
	for i := len(s.servers) - 1; i >= 0; i-- {
		err = errors.Join(err, s.servers[i].Close(success))
	}
	if s.backend != nil {
		err = errors.Join(err, s.backend.Stop(syscall.SIGTERM))
	}
	if success && err == nil {
		err = os.RemoveAll(s.Dir)
	} else {
		fmt.Printf("preserved artifacts: %s\n", s.Dir)
	}
	return err
}

type Server struct {
	Suite   *Suite
	Dir     string
	Staging string
	Addr    string
	// StatusAddr serves the diagnostics API; see Diagnostics.
	StatusAddr string
	Name       string
	Process    *Process
	store      *store
	epoch      int
	closed     bool
}

func (s *Suite) Server(ctx context.Context) (*Server, error) {
	dir, err := os.MkdirTemp(s.Dir, "case-")
	if err != nil {
		return nil, err
	}
	server := &Server{Suite: s, Dir: dir, Staging: filepath.Join(dir, "staging"),
		Name: filepath.Base(dir) + "-" + filepath.Base(s.Dir)}
	s.servers = append(s.servers, server)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server.Addr = listener.Addr().String()
	listener.Close()
	listener, err = net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	server.StatusAddr = listener.Addr().String()
	listener.Close()
	server.store, err = newStore(s.Registry, filepath.Join(dir, "backend-client.log"))
	if err != nil {
		return nil, err
	}
	if _, err := server.store.mkdir(server.store.root(), server.Name); err != nil {
		return nil, err
	}
	if err := server.Start(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (s *Server) Start(ctx context.Context) error {
	s.epoch++
	// libnfs offers sessions only as NFSv4.2. The kernel guest and pynfs
	// ask for 4.1 explicitly, so accepting 4.2 does not change what they use.
	args := []string{"-addr", s.Addr, "-staging", s.Staging, "-registry", s.Suite.Registry,
		"-allow-minor-version-2", "-v"}
	if s.StatusAddr != "" {
		args = append(args, "-status-addr", s.StatusAddr)
	}
	var err error
	s.Process, err = StartProcess(exec.Command(s.Suite.Binary, args...),
		filepath.Join(s.Dir, fmt.Sprintf("nfsd-%d.log", s.epoch)))
	if err != nil {
		return err
	}
	return WaitFor(ctx, "nfsd readiness", 15*time.Second, func() (bool, error) {
		if err := s.Process.Check(); err != nil {
			return false, err
		}
		conn, err := net.DialTimeout("tcp", s.Addr, 100*time.Millisecond)
		if err != nil {
			return false, nil
		}
		conn.Close()
		if s.StatusAddr == "" {
			return true, nil
		}
		var status struct {
			Mode string `json:"mode"`
		}
		if err := s.Diagnostics(ctx, "/v1/status", &status); err != nil {
			return false, nil
		}
		return status.Mode == "serving", nil
	})
}

// Diagnostics fetches one resource of the server's diagnostics API into
// out, which is decoded from JSON. A non-200 reply is an error carrying the
// body, which is the API's own error object.
func (s *Server) Diagnostics(ctx context.Context, path string, out any) error {
	if s.StatusAddr == "" {
		return fmt.Errorf("diagnostics: server %s has no status address", s.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+s.StatusAddr+path, nil)
	if err != nil {
		return err
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("diagnostics %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// diagnosticsDump is what a failed run keeps of the server's in-memory
// state, fetched before the process is stopped.
var diagnosticsDump = []string{
	"/", "/v1/status", "/v1/clients", "/v1/incarnations", "/v1/sessions",
	"/v1/connections", "/v1/opens", "/v1/staging", "/v1/confirmations",
}

// DumpDiagnostics writes each resource of the running server's diagnostics
// API into the server directory, under diagnostics/, so a failed run's
// artifacts include what the process knew. Fetch errors are written in
// place of the resource; the dump never fails the run.
func (s *Server) DumpDiagnostics(ctx context.Context) {
	if s.StatusAddr == "" || s.Process == nil || s.Process.Check() != nil {
		return
	}
	dir := filepath.Join(s.Dir, "diagnostics")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("diagnostics dump: %v\n", err)
		return
	}
	for _, path := range diagnosticsDump {
		name := strings.Trim(strings.ReplaceAll(path, "/", "-"), "-")
		if name == "" {
			name = "index"
		}
		var raw json.RawMessage
		err := s.Diagnostics(ctx, path, &raw)
		if err != nil {
			raw = json.RawMessage(fmt.Sprintf("%q", err.Error()))
		}
		if err := os.WriteFile(filepath.Join(dir, name+".json"), raw, 0644); err != nil {
			fmt.Printf("diagnostics dump %s: %v\n", path, err)
		}
	}
	fmt.Printf("diagnostics of %s written to %s\n", s.Name, dir)
}

func (s *Server) Close(success bool) error {
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if !success {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		s.DumpDiagnostics(ctx)
		cancel()
	}
	if s.Process != nil {
		err = s.Process.Stop(syscall.SIGTERM)
	}
	if s.store != nil {
		if success && err == nil {
			err = s.store.remove(s.store.root(), s.Name)
		}
		s.store.close()
	}
	if !success || err != nil {
		fmt.Printf("preserved backend directory /%s\n", s.Name)
	}
	return err
}
