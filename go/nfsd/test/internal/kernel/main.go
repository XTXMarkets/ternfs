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
	"bytes"
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

// guestMountPoint is a guest-local tmpfs path. It is deliberately outside
// /tmp since some tools (such as vim) change their behaviour there, and
// nfsd is not designed for use there.
const guestMountPoint = "/run/nfs"

func main() {
	var opts harness.Options
	opts.Flags(flag.CommandLine)
	image := flag.String("image-dir", "", "guest kernel image directory written by prepare-image.sh")
	tests := flag.String("tests", "all", "comma-separated functional case numbers, or all")
	timeout := flag.Duration("timeout", 30*time.Minute, "overall run timeout")
	caseTimeout := flag.Duration("case-timeout", 5*time.Minute, "timeout for one functional case in the guest")
	smp := flag.Int("smp", 4, "guest CPUs")
	memory := flag.String("mem", "4G", "guest memory")
	restart := flag.Bool("restart-mid-write", false,
		"instead of the functional cases, kill and restart nfsd while the guest holds a writer open, then check what it publishes")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	if err := run(ctx, opts, *image, *tests, *caseTimeout, *smp, *memory, *restart); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const restartWriterName = "restart-mid-write.dat"

// restartWriterData is what restart-writer.py writes: a prefix before the
// restart and the rest after it.
func restartWriterData() []byte {
	return append(bytes.Repeat([]byte("written before the nfsd restart\n"), 2048),
		bytes.Repeat([]byte("written after the nfsd restart\n"), 2048)...)
}

// restartMidWrite waits for the guest's writer to have synced its prefix,
// kills nfsd, starts it again on the same address, staging directory and
// export, and tells the guest to continue.
func restartMidWrite(ctx context.Context, server *harness.Server, control string, timeout time.Duration) error {
	if err := harness.WaitFor(ctx, "guest writer", timeout, func() (bool, error) {
		_, err := os.Stat(filepath.Join(control, "ready"))
		return err == nil, nil
	}); err != nil {
		return err
	}
	if err := server.Process.Stop(syscall.SIGKILL); err != nil {
		return err
	}
	fmt.Println("RESTART nfsd killed with the guest's writer open")
	if err := server.Start(ctx); err != nil {
		return err
	}
	fmt.Println("RESTART nfsd started again")
	return os.WriteFile(filepath.Join(control, "restarted"), nil, 0644)
}

func run(ctx context.Context, opts harness.Options, image, tests string,
	caseTimeout time.Duration, smp int, memory string, restart bool) (err error) {
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
	// Linux passes these parameters to init as environment variables.
	var extra []string
	control := filepath.Join(server.Dir, "restart-control")
	if restart {
		if err := os.Mkdir(control, 0755); err != nil {
			return err
		}
		extra = []string{"mode=restart-mid-write", "control=" + control,
			"restart_writer=" + filepath.Join(kernelDir, "restart-writer.py")}
	}
	cmdline, err := kernelCommandLine(append([]string{
		"console=ttyS0", "loglevel=4", "panic=-1",
		"guest_init=" + filepath.Join(kernelDir, "guest-init.sh"),
		"nfsport=" + port,
		"export=/" + server.Name,
		"host=" + server.Name,
		"tests=" + tests,
		"case_timeout=" + strconv.Itoa(int(caseTimeout/time.Second)),
		"log=" + guestLog,
		"mnt=" + guestMountPoint,
		"functional=" + functional,
	}, extra...))
	if err != nil {
		return err
	}
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
	restarted := make(chan error, 1)
	restartCtx, cancelRestart := context.WithCancel(ctx)
	defer cancelRestart()
	if restart {
		go func() { restarted <- restartMidWrite(restartCtx, server, control, caseTimeout) }()
	}

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
	// Once the guest is gone, no restart is still useful; wait for the
	// coordinator so nothing else touches the server meanwhile.
	var restartErr error
	if restart {
		cancelRestart()
		restartErr = <-restarted
	}
	time.Sleep(200 * time.Millisecond)
	stopFollow()
	<-followed

	output, readErr := os.ReadFile(guestLog)
	if readErr != nil {
		return errors.Join(waitErr, restartErr, readErr)
	}
	match := resultPattern.FindSubmatch(output)
	if match == nil {
		return errors.Join(waitErr, restartErr, fmt.Errorf("guest reported no result; see %s", guest.Log))
	}
	if status, _ := strconv.Atoi(string(match[1])); status != 0 {
		return errors.Join(restartErr, fmt.Errorf("functional cases failed (guest exit %d)", status))
	}
	if restart {
		if restartErr != nil {
			return errors.Join(waitErr, restartErr)
		}
		// Read what was published through the backend, not the guest's cache.
		data, err := server.ReadFile(restartWriterName)
		if err != nil {
			return errors.Join(waitErr, fmt.Errorf("read published %s: %w", restartWriterName, err))
		}
		if !bytes.Equal(data, restartWriterData()) {
			return errors.Join(waitErr, fmt.Errorf("published %s has %d bytes, want the %d written across the restart",
				restartWriterName, len(data), len(restartWriterData())))
		}
		fmt.Printf("RESTART published %d bytes, read back through the backend\n", len(data))
		if err := checkBackchannelLifetimes(server.Dir, 2); err != nil {
			return errors.Join(waitErr, err)
		}
	}
	return waitErr
}

// checkBackchannelLifetimes requires, separately for each nfsd lifetime of
// the one mount, a CB_NULL whose reply nfsd validated: one "backchannel path
// up" line in that lifetime's own log, and no "backchannel path
// unresponsive" line. It prints the evidence, so it outlives the run
// directory's cleanup.
func checkBackchannelLifetimes(dir string, lifetimes int) error {
	var errs []error
	for epoch := 1; epoch <= lifetimes; epoch++ {
		name := filepath.Join(dir, fmt.Sprintf("nfsd-%d.log", epoch))
		data, err := os.ReadFile(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		up, unresponsive := 0, 0
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case strings.Contains(line, `msg="backchannel path up"`):
				up++
				fmt.Printf("BACKCHANNEL epoch=%d %s\n", epoch, line)
			case strings.Contains(line, `msg="backchannel path unresponsive"`):
				unresponsive++
				fmt.Printf("BACKCHANNEL epoch=%d %s\n", epoch, line)
			}
		}
		fmt.Printf("BACKCHANNEL epoch=%d path-up=%d unresponsive=%d\n", epoch, up, unresponsive)
		if up == 0 || unresponsive != 0 {
			errs = append(errs, fmt.Errorf("nfsd lifetime %d: %d validated CB_NULL replies, %d unresponsive paths", epoch, up, unresponsive))
		}
	}
	return errors.Join(errs...)
}

// Linux accepts double-quoted values, but does not support escaping a double
// quote within them. Backslashes and shell metacharacters are literal.
func kernelCommandLine(args []string) (string, error) {
	quoted := make([]string, len(args))
	for i, arg := range args {
		if strings.ContainsAny(arg, "\"\x00") {
			return "", fmt.Errorf("kernel command line argument contains a double quote or NUL: %q", arg)
		}
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return "", fmt.Errorf("kernel command line argument has no value: %q", arg)
		}
		quoted[i] = key + `="` + value + `"`
	}
	return strings.Join(quoted, " "), nil
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
