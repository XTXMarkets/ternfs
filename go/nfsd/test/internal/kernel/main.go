// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build linux

// Command kernel runs the functional workflow tests through the Linux kernel
// NFS client in a qemu guest. The shared harness provides the backend and an
// nfsd; the guest boots the image from prepare-image.sh, shares this host's
// root filesystem over virtio-9p and reaches nfsd on the host loopback through
// qemu user networking. Nothing needs root, KVM, tap or FUSE access.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/XTXMarkets/ternfs/go/nfsd/test/internal/harness"
)

var resultPattern = regexp.MustCompile(`(?m)^suite exit=(\d+)$`)

func main() {
	var opts harness.Options
	opts.Flags(flag.CommandLine)
	image := flag.String("image-dir", "", "guest kernel image directory written by prepare-image.sh")
	tests := flag.String("tests", "all", "comma-separated functional case numbers, or all")
	timeout := flag.Duration("timeout", 30*time.Minute, "overall run timeout")
	caseTimeout := flag.Duration("case-timeout", 5*time.Minute, "timeout for one functional case in the guest")
	smp := flag.Int("smp", 4, "guest CPUs")
	memory := flag.String("mem", "4G", "guest memory")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if err := run(ctx, opts, *image, *tests, *caseTimeout, *smp, *memory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, opts harness.Options, image, tests string,
	caseTimeout time.Duration, smp int, memory string) (err error) {
	if image == "" {
		return fmt.Errorf("-image-dir is required; run prepare-image.sh or make build-kernel-image")
	}
	image = harness.SourcePath(image)
	kernel := filepath.Join(image, "vmlinuz")
	initrd := filepath.Join(image, "initrd.img")
	for _, path := range []string{kernel, initrd} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("guest image incomplete: %w", err)
		}
	}
	qemu, err := exec.LookPath("qemu-system-x86_64")
	if err != nil {
		return fmt.Errorf("qemu-system-x86_64 is not installed: %w", err)
	}
	kernelDir := harness.SourcePath("internal/kernel")
	functional := harness.SourcePath("functional")

	suite, err := harness.New(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, suite.Close(err == nil)) }()
	server, err := suite.Server(ctx)
	if err != nil {
		return err
	}
	_, port, err := net.SplitHostPort(server.Addr)
	if err != nil {
		return err
	}

	guestLog := filepath.Join(server.Dir, "guest.log")
	if err := os.WriteFile(guestLog, nil, 0644); err != nil {
		return err
	}
	// The guest resolves these host paths through its 9p root. It mounts the
	// server's own export so the harness can remove the run's data on success.
	cmdline := strings.Join([]string{
		"console=ttyS0", "loglevel=4", "panic=-1",
		"guest_init=" + filepath.Join(kernelDir, "guest-init.sh"),
		"nfsport=" + port,
		"export=/" + server.Name,
		"host=" + server.Name,
		"tests=" + tests,
		"case_timeout=" + strconv.Itoa(int(caseTimeout/time.Second)),
		"log=" + guestLog,
		"mnt=" + filepath.Join(server.Dir, "mnt"),
		"functional=" + functional,
	}, " ")
	accel := "tcg,thread=multi"
	if f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0); err == nil {
		f.Close()
		accel = "kvm"
	}
	args := []string{
		"-M", "q35", "-accel", accel, "-smp", strconv.Itoa(smp), "-m", memory,
		"-kernel", kernel, "-initrd", initrd, "-append", cmdline,
		"-nographic", "-no-reboot",
		"-netdev", "user,id=net0", "-device", "virtio-net-pci,netdev=net0",
		"-fsdev", "local,id=root,path=/,security_model=none,multidevs=remap",
		"-device", "virtio-9p-pci,fsdev=root,mount_tag=hostroot",
	}
	args = append(args, strings.Fields(os.Getenv("QEMU_ARGS"))...)
	cmd := exec.Command(qemu, args...)
	cmd.Stdin = nil
	guest, err := harness.StartProcess(cmd, filepath.Join(server.Dir, "console.log"))
	if err != nil {
		return err
	}
	fmt.Printf("qemu accel=%s image=%s guest log=%s\n", accel, image, guestLog)

	follow, stopFollow := context.WithCancel(ctx)
	followed := make(chan struct{})
	go func() {
		defer close(followed)
		followFile(follow, guestLog, os.Stdout)
	}()
	waitErr := guest.Wait(ctx)
	if waitErr != nil {
		_ = guest.Stop(syscall.SIGKILL)
	}
	time.Sleep(200 * time.Millisecond)
	stopFollow()
	<-followed

	output, readErr := os.ReadFile(guestLog)
	if readErr != nil {
		return errors.Join(waitErr, readErr)
	}
	match := resultPattern.FindSubmatch(output)
	if match == nil {
		return errors.Join(waitErr, fmt.Errorf("guest reported no result; see %s", guest.Log))
	}
	if status, _ := strconv.Atoi(string(match[1])); status != 0 {
		return fmt.Errorf("functional cases failed (guest exit %d)", status)
	}
	return waitErr
}

// followFile prints data appended to path until ctx is cancelled, then drains
// what remains.
func followFile(ctx context.Context, path string, w io.Writer) {
	var offset int64
	drain := func() {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return
		}
		n, _ := io.Copy(w, f)
		offset += n
	}
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			drain()
			return
		case <-tick.C:
			drain()
		}
	}
}
