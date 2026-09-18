// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux

package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Process struct {
	cmd      *exec.Cmd
	done     chan struct{}
	err      error // read after done closes
	expected bool
	Log      string
	Grace    time.Duration
}

func StartProcess(cmd *exec.Cmd, logPath string) (*Process, error) {
	output, err := os.Create(logPath)
	if err != nil {
		return nil, err
	}
	cmd.Stderr = output
	if cmd.Stdout == nil {
		cmd.Stdout = output
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	if err := cmd.Start(); err != nil {
		output.Close()
		return nil, err
	}
	p := &Process{cmd: cmd, done: make(chan struct{}), Log: logPath, Grace: 5 * time.Second}
	go func() {
		p.err = cmd.Wait()
		output.Close()
		close(p.done)
	}()
	return p, nil
}

func (p *Process) Stop(sig syscall.Signal) error {
	select {
	case <-p.done:
		if !p.expected {
			return fmt.Errorf("%s exited unexpectedly: %v (log: %s)", p.cmd.Path, p.err, p.Log)
		}
		return nil
	default:
	}
	p.expected = true
	if err := syscall.Kill(-p.cmd.Process.Pid, sig); err != nil && err != syscall.ESRCH {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(p.Grace):
		_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		select {
		case <-p.done:
			return nil
		case <-time.After(5 * time.Second):
			return fmt.Errorf("process %d did not exit (log: %s)", p.cmd.Process.Pid, p.Log)
		}
	}
}

func (p *Process) Check() error {
	select {
	case <-p.done:
		data, _ := os.ReadFile(p.Log)
		return fmt.Errorf("%s exited: %v\n%s", p.cmd.Path, p.err, data)
	default:
		return nil
	}
}

// Wait is for commands expected to finish, such as the pynfs Python runner.
func (p *Process) Wait(ctx context.Context) error {
	p.expected = true
	select {
	case <-p.done:
		if p.err != nil {
			return fmt.Errorf("%s: %w (log: %s)", p.cmd.Path, p.err, p.Log)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func WaitFor(ctx context.Context, what string, timeout time.Duration, ready func() (bool, error)) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	if timeout >= time.Minute {
		tick.Reset(time.Second)
	}
	defer tick.Stop()
	for {
		ok, err := ready()
		if err != nil || ok {
			return err
		}
		select {
		case <-tick.C:
		case <-timer.C:
			return fmt.Errorf("timed out waiting for %s after %s", what, timeout)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
