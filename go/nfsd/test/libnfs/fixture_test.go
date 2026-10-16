// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux && libnfs

package libnfs

import (
	"flag"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/test/internal/harness"
)

var options harness.Options

func init() { options.Flags(flag.CommandLine) }

type nfsTestSuite struct{ *harness.Suite }

func newNFSTestSuite(t *testing.T) *nfsTestSuite {
	t.Helper()
	suite, err := harness.New(t.Context(), options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := suite.Close(!t.Failed()); err != nil {
			t.Error(err)
		}
	})
	return &nfsTestSuite{suite}
}

type nfsTestProcess struct{ *harness.Process }

func startNFSTestProcess(t *testing.T, cmd *exec.Cmd, logPath string) *nfsTestProcess {
	t.Helper()
	process, err := harness.StartProcess(cmd, logPath)
	if err != nil {
		t.Fatal(err)
	}
	p := &nfsTestProcess{process}
	t.Cleanup(func() { p.stop(t, syscall.SIGTERM) })
	return p
}

func (p *nfsTestProcess) stop(t *testing.T, sig syscall.Signal) {
	t.Helper()
	if err := p.Stop(sig); err != nil {
		t.Error(err)
	}
}

func (p *nfsTestProcess) alive(t *testing.T) {
	t.Helper()
	if err := p.Check(); err != nil {
		t.Fatal(err)
	}
}

type nfsTestServer struct {
	*harness.Server
	process     *nfsTestProcess
	clients     int
	inspections int
}

func (s *nfsTestSuite) server(t *testing.T) *nfsTestServer {
	t.Helper()
	server, err := s.Suite.Server(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(!t.Failed()); err != nil {
			t.Error(err)
		}
	})
	return &nfsTestServer{Server: server, process: &nfsTestProcess{server.Process}}
}

func (s *nfsTestServer) seed(t *testing.T, dir string) {
	t.Helper()
	if err := s.Seed(dir); err != nil {
		t.Fatal(err)
	}
}

func (s *nfsTestServer) start(t *testing.T) {
	t.Helper()
	if err := s.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.process = &nfsTestProcess{s.Process}
}

func (s *nfsTestServer) restart(t *testing.T, sig syscall.Signal) {
	t.Helper()
	s.process.alive(t)
	s.process.stop(t, sig)
	s.start(t)
}

func waitForNFSTest(t *testing.T, what string, timeout time.Duration, ready func() bool) {
	t.Helper()
	if err := harness.WaitFor(t.Context(), what, timeout, func() (bool, error) {
		return ready(), nil
	}); err != nil {
		t.Fatal(err)
	}
}
