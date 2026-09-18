// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

func smallDiskIOLimits() diskIOLimitOptions {
	return diskIOLimitOptions{
		max:               [diskIOClassCount]int{2, 2, 2},
		perService:        [diskIOClassCount]int{1, 1, 1},
		maxWaiting:        2,
		perServiceWaiting: 1,
		waitTimeout:       0,
	}
}

func testDiskIOLimiter(t *testing.T, opts diskIOLimitOptions) *diskIOLimiter {
	t.Helper()
	l := newDiskIOLimiter(opts, []msgs.BlockServiceId{1, 2, 3, 4, 5, 6, 7, 8})
	t.Cleanup(l.close)
	return l
}

func mustAcquireIO(t *testing.T, l *diskIOLimiter, id msgs.BlockServiceId, kind msgs.BlocksMessageKind) func() {
	return holdDiskIO(t, l, id, kind, time.Time{})
}

func holdDiskIO(t *testing.T, l *diskIOLimiter, id msgs.BlockServiceId, kind msgs.BlocksMessageKind, deadline time.Time) func() {
	t.Helper()
	started, unblock, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var runErr error
	go func() {
		_, runErr = l.run(id, kind, deadline, func() bool {
			close(started)
			<-unblock
			return true
		})
		close(done)
	}()
	var once sync.Once
	release := func() {
		once.Do(func() {
			close(unblock)
			select {
			case <-done:
				if runErr != nil {
					t.Errorf("held I/O failed: %v", runErr)
				}
			case <-time.After(5 * time.Second):
				t.Error("held I/O did not finish")
			}
		})
	}
	t.Cleanup(release)
	select {
	case <-started:
	case <-done:
		t.Fatalf("could not acquire %v for %v: %v", kind, id, runErr)
	case <-time.After(5 * time.Second):
		t.Fatalf("could not acquire %v for %v", kind, id)
	}
	return release
}

func expectIOBusy(t *testing.T, l *diskIOLimiter, id msgs.BlockServiceId, kind msgs.BlocksMessageKind) {
	t.Helper()
	_, err := l.run(id, kind, time.Time{}, func() bool { return true })
	if !errors.Is(err, errDiskIOBusy) {
		t.Fatalf("expected busy %v for %v, got %v", kind, id, err)
	}
}

func waitForIOWaiters(t *testing.T, l *diskIOLimiter, count int) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		global, _ := l.snapshot()
		if global.totalWaiting() == count {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("waiting for %d queued requests, got %+v", count, global)
		case <-ticker.C:
		}
	}
}

func TestDiskIOLimitsAcrossServicesAndClasses(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []msgs.BlocksMessageKind
	}{
		{"reads and checks", []msgs.BlocksMessageKind{msgs.FETCH_BLOCK, msgs.FETCH_BLOCK_WITH_CRC, msgs.CHECK_BLOCK}},
		{"writes and test writes", []msgs.BlocksMessageKind{msgs.WRITE_BLOCK, msgs.TEST_WRITE}},
		{"erases", []msgs.BlocksMessageKind{msgs.ERASE_BLOCK}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := testDiskIOLimiter(t, smallDiskIOLimits())
			release1 := mustAcquireIO(t, l, 1, tc.kinds[0])
			for _, kind := range tc.kinds {
				expectIOBusy(t, l, 1, kind)
			}
			release2 := mustAcquireIO(t, l, 2, tc.kinds[len(tc.kinds)-1])
			expectIOBusy(t, l, 3, tc.kinds[0])
			// Saturation of this class leaves the other classes usable.
			class := diskIORequestClass(tc.kinds[0])
			for _, kind := range []msgs.BlocksMessageKind{msgs.FETCH_BLOCK, msgs.WRITE_BLOCK, msgs.ERASE_BLOCK} {
				if diskIORequestClass(kind) != class {
					mustAcquireIO(t, l, 1, kind)()
				}
			}
			release1()
			mustAcquireIO(t, l, 3, tc.kinds[0])()
			release2()
			global, services := l.snapshot()
			if global.active != ([diskIOClassCount]int{}) || global.totalWaiting() != 0 {
				t.Fatalf("leaked admissions: %+v", global)
			}
			if global.rejected[class] != uint64(len(tc.kinds)+1) || services[1].rejected[class] != uint64(len(tc.kinds)) {
				t.Fatalf("incorrect rejection counters: %+v, %+v", global, services)
			}
		})
	}
}

func TestDiskIOQueueDoesNotReserveProcessSlots(t *testing.T) {
	opts := smallDiskIOLimits()
	opts.waitTimeout = 5 * time.Second
	l := testDiskIOLimiter(t, opts)
	release := mustAcquireIO(t, l, 1, msgs.WRITE_BLOCK)
	done := make(chan error, 1)
	go func() {
		_, err := l.run(1, msgs.TEST_WRITE, time.Time{}, func() bool { return true })
		done <- err
	}()
	waitForIOWaiters(t, l, 1)
	// One stalled service has an active write and a queued write; a
	// healthy service can still take the remaining process write slot.
	mustAcquireIO(t, l, 2, msgs.WRITE_BLOCK)()
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued write was not woken by release")
	}
	waitForIOWaiters(t, l, 0)
}

func TestDiskIOQueueBounds(t *testing.T) {
	opts := smallDiskIOLimits()
	opts.max[diskWrite] = 1
	opts.waitTimeout = 5 * time.Second
	l := testDiskIOLimiter(t, opts)
	release := mustAcquireIO(t, l, 1, msgs.WRITE_BLOCK)
	done := make(chan error, 2)
	queue := func(id msgs.BlockServiceId) {
		go func() {
			_, err := l.run(id, msgs.WRITE_BLOCK, time.Time{}, func() bool { return true })
			done <- err
		}()
	}
	queue(1)
	waitForIOWaiters(t, l, 1)
	expectIOBusy(t, l, 1, msgs.TEST_WRITE) // per-service queue is full
	queue(2)
	waitForIOWaiters(t, l, 2)
	expectIOBusy(t, l, 3, msgs.WRITE_BLOCK) // process queue is full
	// An immediately available erase does not need a queue position.
	mustAcquireIO(t, l, 1, msgs.ERASE_BLOCK)()
	release()
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("queue did not drain")
		}
	}
	waitForIOWaiters(t, l, 0)
}

func TestDiskIOAdmissionDeadlines(t *testing.T) {
	for _, requestDeadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("request deadline %v", requestDeadline), func(t *testing.T) {
			opts := smallDiskIOLimits()
			opts.waitTimeout = 30 * time.Millisecond
			var deadline time.Time
			if requestDeadline {
				opts.waitTimeout = 5 * time.Second
				deadline = time.Now().Add(30 * time.Millisecond)
			}
			l := testDiskIOLimiter(t, opts)
			release := mustAcquireIO(t, l, 1, msgs.FETCH_BLOCK)
			defer release()
			start := time.Now()
			if _, err := l.run(1, msgs.CHECK_BLOCK, deadline, func() bool { return true }); !errors.Is(err, errDiskIOBusy) {
				t.Fatalf("expected admission timeout, got %v", err)
			}
			if time.Since(start) > time.Second {
				t.Fatal("admission ignored the shorter deadline")
			}
			global, _ := l.snapshot()
			if global.active[diskRead] != 1 || global.totalWaiting() != 0 {
				t.Fatalf("timeout freed an active operation or leaked waiter: %+v", global)
			}
		})
	}
	l := testDiskIOLimiter(t, smallDiskIOLimits())
	if _, err := l.run(1, msgs.FETCH_BLOCK, time.Now().Add(-time.Second), func() bool {
		t.Error("expired work executed")
		return true
	}); !errors.Is(err, errDiskIOBusy) {
		t.Fatalf("expired request admitted: %v", err)
	}
}

func TestActiveDiskIOOutlivesRequestDeadline(t *testing.T) {
	for _, kind := range []msgs.BlocksMessageKind{msgs.FETCH_BLOCK, msgs.WRITE_BLOCK, msgs.ERASE_BLOCK} {
		t.Run(kind.String(), func(t *testing.T) {
			l := testDiskIOLimiter(t, smallDiskIOLimits())
			// Model a disk operation that cannot return until the device
			// recovers. Expiry must not let a second operation replace it.
			deadline := time.Now().Add(20 * time.Millisecond)
			release := holdDiskIO(t, l, 1, kind, deadline)
			<-time.After(time.Until(deadline) + time.Millisecond)
			expectIOBusy(t, l, 1, kind)
			global, _ := l.snapshot()
			if global.active[diskIORequestClass(kind)] != 1 {
				t.Fatal("expired request released its unfinished disk operation")
			}
			release()
			mustAcquireIO(t, l, 1, kind)()
		})
	}
}

func TestWorkerWaitTimeoutReturnsServiceSlot(t *testing.T) {
	for _, kind := range []msgs.BlocksMessageKind{msgs.FETCH_BLOCK, msgs.WRITE_BLOCK, msgs.ERASE_BLOCK} {
		t.Run(kind.String(), func(t *testing.T) {
			opts := smallDiskIOLimits()
			opts.max = [diskIOClassCount]int{1, 1, 1}
			opts.waitTimeout = 20 * time.Millisecond
			l := testDiskIOLimiter(t, opts)
			release := mustAcquireIO(t, l, 1, kind)
			// Service 2 has a free service slot, but no worker is idle.
			expectIOBusy(t, l, 2, kind)
			if slots := len(l.services[2].slots[diskIORequestClass(kind)]); slots != 0 {
				t.Fatalf("worker wait leaked %d service slots", slots)
			}
			global, services := l.snapshot()
			if global.totalWaiting() != 0 || services[2].totalWaiting() != 0 {
				t.Fatal("worker wait leaked queue capacity")
			}
			release()
			mustAcquireIO(t, l, 2, kind)()
		})
	}
}

func TestWorkerPanicReturnsResourcesAndReachesCaller(t *testing.T) {
	l := testDiskIOLimiter(t, smallDiskIOLimits())
	var caught any
	func() {
		defer func() { caught = recover() }()
		l.run(1, msgs.WRITE_BLOCK, time.Time{}, func() bool { panic("disk failure") })
	}()
	if message := fmt.Sprint(caught); !strings.Contains(message, "disk failure") || !strings.Contains(message, "io_limits_test.go") {
		t.Fatalf("worker panic and original stack not delivered to caller: %v", caught)
	}
	mustAcquireIO(t, l, 1, msgs.WRITE_BLOCK)()
	global, _ := l.snapshot()
	if global.active[diskWrite] != 0 {
		t.Fatal("worker panic leaked its assignment")
	}
}

func TestUnknownServiceDoesNotChangeWorkerServiceMap(t *testing.T) {
	l := testDiskIOLimiter(t, smallDiskIOLimits())
	_, err := l.run(999, msgs.FETCH_BLOCK, time.Time{}, func() bool {
		t.Error("unknown service executed disk work")
		return true
	})
	if err != msgs.BLOCK_SERVICE_NOT_FOUND || len(l.services) != 8 {
		t.Fatalf("unknown service changed admission state: %v, %d services", err, len(l.services))
	}
}

func TestDefaultErasesAreSerializedPerService(t *testing.T) {
	opts := defaultDiskIOLimitOptions()
	opts.max = [diskIOClassCount]int{1, 1, 2}
	opts.waitTimeout = 0
	l := testDiskIOLimiter(t, opts)
	release := mustAcquireIO(t, l, 1, msgs.ERASE_BLOCK)
	defer release()
	expectIOBusy(t, l, 1, msgs.ERASE_BLOCK)
	// Erases on a different disk remain independent.
	mustAcquireIO(t, l, 2, msgs.ERASE_BLOCK)()
}

func TestDiskIOLimitsUnderContention(t *testing.T) {
	opts := smallDiskIOLimits()
	opts.max = [diskIOClassCount]int{4, 3, 2}
	opts.waitTimeout = 10 * time.Millisecond
	l := testDiskIOLimiter(t, opts)
	var wg sync.WaitGroup
	for worker := range 48 {
		wg.Go(func() {
			for range 30 {
				kind := []msgs.BlocksMessageKind{msgs.FETCH_BLOCK, msgs.CHECK_BLOCK, msgs.WRITE_BLOCK, msgs.ERASE_BLOCK}[worker%4]
				_, err := l.run(msgs.BlockServiceId(1+worker%8), kind, time.Time{}, func() bool {
					global, services := l.snapshot()
					for class := range diskIOClassCount {
						if global.active[class] > opts.max[class] || global.active[class] < 0 {
							t.Errorf("process limit violated: %+v", global)
						}
						for _, service := range services {
							if service.active[class] > opts.perService[class] || service.active[class] < 0 {
								t.Errorf("service limit violated: %+v", service)
							}
						}
					}
					if global.totalWaiting() > opts.maxWaiting {
						t.Errorf("queue limit violated: %+v", global)
					}
					return true
				})
				if errors.Is(err, errDiskIOBusy) {
					continue
				}
				if err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	global, _ := l.snapshot()
	if global.active != ([diskIOClassCount]int{}) || global.totalWaiting() != 0 {
		t.Fatalf("leaked admissions: %+v", global)
	}
}

func TestDiskIOLimitFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		ok   bool
	}{
		{nil, true},
		{[]string{"-max-reads=0"}, false},
		{[]string{"-max-writes-per-block-service=-1"}, false},
		{[]string{"-max-erases-per-block-service=0"}, false},
		{[]string{"-max-io-waiters=-1"}, false},
		{[]string{"-max-io-waiters-per-block-service=-1"}, false},
		{[]string{"-io-wait-timeout=-1s"}, false},
		{[]string{"-max-io-waiters=0", "-io-wait-timeout=0"}, true},
		{[]string{"-max-reads=12", "-max-reads-per-block-service=3", "-max-erases-per-block-service=5"}, true},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			opts := defaultDiskIOLimitOptions()
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			opts.registerFlags(fs)
			if err := fs.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			if err := opts.validate(); (err == nil) != tc.ok {
				t.Fatalf("validation error %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestDiskIOMetrics(t *testing.T) {
	l := testDiskIOLimiter(t, smallDiskIOLimits())
	release := mustAcquireIO(t, l, 1, msgs.FETCH_BLOCK)
	defer release()
	expectIOBusy(t, l, 1, msgs.CHECK_BLOCK)
	metrics := &log.MetricsBuilder{}
	l.appendMetrics(metrics, "host", "prefix", time.Unix(0, 1))
	data, err := io.ReadAll(metrics.Payload())
	if err != nil {
		t.Fatal(err)
	}
	payload := string(data)
	for _, want := range []string{
		"scope=process,operation=read active=1i,waiting=0i,rejected=1i,limit=2i",
		"scope=blockservice,blockservice=0x0000000000000001,operation=read active=1i,waiting=0i,rejected=1i,limit=1i",
		"operation=erase active=0i,waiting=0i,rejected=0i,limit=2i",
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("metrics missing %q:\n%s", want, payload)
		}
	}
}
