// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

type diskIOClass int

const (
	diskRead diskIOClass = iota
	diskWrite
	diskErase
	diskIOClassCount
)

var diskIOClassNames = [diskIOClassCount]string{"read", "write", "erase"}
var errDiskIOBusy = errors.New("disk I/O admission limit reached")

type diskIOLimitOptions struct {
	max               [diskIOClassCount]int
	perService        [diskIOClassCount]int
	maxWaiting        int
	perServiceWaiting int
	waitTimeout       time.Duration
}

func defaultDiskIOLimitOptions() diskIOLimitOptions {
	return diskIOLimitOptions{
		max:               [diskIOClassCount]int{4096, 2048, 1024},
		perService:        [diskIOClassCount]int{128, 64, 1},
		maxWaiting:        4096,
		perServiceWaiting: 128,
		waitTimeout:       time.Second,
	}
}

func (o *diskIOLimitOptions) registerFlags(fs *flag.FlagSet) {
	for class, name := range diskIOClassNames {
		fs.IntVar(&o.max[class], "max-"+name+"s", o.max[class], "Maximum concurrent "+name+" requests across this process (reads include CRC checks).")
		fs.IntVar(&o.perService[class], "max-"+name+"s-per-block-service", o.perService[class], "Maximum concurrent "+name+" requests per block service (reads include CRC checks).")
	}
	fs.IntVar(&o.maxWaiting, "max-io-waiters", o.maxWaiting, "Maximum requests waiting for disk I/O admission across this process; zero rejects immediately.")
	fs.IntVar(&o.perServiceWaiting, "max-io-waiters-per-block-service", o.perServiceWaiting, "Maximum requests waiting for disk I/O admission per block service; zero rejects immediately.")
	fs.DurationVar(&o.waitTimeout, "io-wait-timeout", o.waitTimeout, "Maximum wait for disk I/O admission; zero rejects immediately.")
}

func (o diskIOLimitOptions) validate() error {
	for class, name := range diskIOClassNames {
		if o.max[class] <= 0 || o.perService[class] <= 0 {
			return fmt.Errorf("max-%ss and max-%ss-per-block-service must be positive", name, name)
		}
	}
	if o.maxWaiting < 0 || o.perServiceWaiting < 0 || o.waitTimeout < 0 {
		return errors.New("I/O waiter limits and io-wait-timeout must not be negative")
	}
	return nil
}

type diskIOCounters struct {
	active   [diskIOClassCount]int
	waiting  [diskIOClassCount]int
	rejected [diskIOClassCount]uint64
	queued   int
}

func (c diskIOCounters) totalWaiting() int {
	return c.queued
}

type diskIOStats struct {
	active   atomic.Int64
	waiting  atomic.Int64
	rejected atomic.Uint64
}

type diskIOService struct {
	slots   [diskIOClassCount]chan struct{}
	waiting chan struct{}
	stats   [diskIOClassCount]diskIOStats
}

type diskIOResult struct {
	keepConnection bool
	err            error
	panicValue     any
}

type diskIOJob struct {
	service  *diskIOService
	deadline time.Time
	work     func() bool
}

func (j *diskIOJob) execute() (result diskIOResult) {
	// Deliver panics to the submitting connection goroutine, whose existing
	// recovery handler logs them and terminates the process.
	defer func() {
		if value := recover(); value != nil {
			result.panicValue = fmt.Errorf("disk worker panic: %v\n%s", value, debug.Stack())
		}
	}()
	if !j.deadline.IsZero() && !time.Now().Before(j.deadline) {
		result.err = errDiskIOBusy
		return
	}
	result.keepConnection = j.work()
	return
}

type diskIOWorker struct {
	jobs    chan diskIOJob
	results chan diskIOResult
}

type diskIOPool struct {
	idle  chan *diskIOWorker
	stats diskIOStats
}

// Each class has a fixed worker pool. Acquire a service slot before taking a
// worker: a worker never waits on a saturated service's concurrency limit.
// The service map is built at startup and read-only once requests can arrive.
type diskIOLimiter struct {
	options  diskIOLimitOptions
	pools    [diskIOClassCount]diskIOPool
	services map[msgs.BlockServiceId]*diskIOService
	waiting  chan struct{}
	workers  sync.WaitGroup
}

func newDiskIOLimiter(options diskIOLimitOptions, serviceIDs []msgs.BlockServiceId) *diskIOLimiter {
	l := &diskIOLimiter{
		options:  options,
		services: make(map[msgs.BlockServiceId]*diskIOService, len(serviceIDs)),
		waiting:  make(chan struct{}, options.maxWaiting),
	}
	for _, id := range serviceIDs {
		service := &diskIOService{waiting: make(chan struct{}, options.perServiceWaiting)}
		for class := range diskIOClassCount {
			service.slots[class] = make(chan struct{}, options.perService[class])
		}
		l.services[id] = service
	}
	for class := range diskIOClassCount {
		pool := &l.pools[class]
		pool.idle = make(chan *diskIOWorker, options.max[class])
		for range options.max[class] {
			// Buffer one assignment so taking an idle worker does not
			// depend on when the scheduler first runs that worker.
			worker := &diskIOWorker{
				jobs:    make(chan diskIOJob, 1),
				results: make(chan diskIOResult),
			}
			pool.idle <- worker
			l.workers.Go(func() {
				for job := range worker.jobs {
					pool.stats.active.Add(1)
					job.service.stats[class].active.Add(1)
					result := job.execute()
					job.service.stats[class].active.Add(-1)
					pool.stats.active.Add(-1)
					<-job.service.slots[class]
					worker.results <- result
				}
			})
		}
	}
	return l
}

// close is for draining a pool after its submitters have stopped. It waits for
// active disk operations; it must not be used to cancel an unresponsive disk.
func (l *diskIOLimiter) close() {
	for class := range diskIOClassCount {
		pool := &l.pools[class]
		for range l.options.max[class] {
			worker := <-pool.idle
			close(worker.jobs)
		}
	}
	l.workers.Wait()
}

func diskIORequestClass(kind msgs.BlocksMessageKind) diskIOClass {
	switch kind {
	case msgs.FETCH_BLOCK, msgs.FETCH_BLOCK_WITH_CRC, msgs.CHECK_BLOCK:
		return diskRead
	case msgs.WRITE_BLOCK, msgs.TEST_WRITE:
		return diskWrite
	case msgs.ERASE_BLOCK:
		return diskErase
	default:
		panic(fmt.Sprintf("no disk I/O class for request %v", kind))
	}
}

func tryDiskIOSlot(slots chan struct{}) bool {
	select {
	case slots <- struct{}{}:
		return true
	default:
		return false
	}
}

// takeWorker bounds both stages of waiting: first for a service slot, then for
// an idle worker. It holds no process worker while waiting for a service.
func (l *diskIOLimiter) takeWorker(service *diskIOService, class diskIOClass, requestDeadline time.Time) (worker *diskIOWorker, deadline time.Time, err error) {
	pool := &l.pools[class]
	now := time.Now()
	if !requestDeadline.IsZero() && !now.Before(requestDeadline) {
		return nil, time.Time{}, errDiskIOBusy
	}
	haveServiceSlot := tryDiskIOSlot(service.slots[class])
	defer func() {
		if haveServiceSlot && worker == nil {
			<-service.slots[class]
		}
	}()
	if haveServiceSlot {
		select {
		case worker = <-pool.idle:
			return worker, requestDeadline, nil
		default:
		}
	}
	if l.options.waitTimeout == 0 || !tryDiskIOSlot(service.waiting) {
		return nil, time.Time{}, errDiskIOBusy
	}
	defer func() { <-service.waiting }()
	if !tryDiskIOSlot(l.waiting) {
		return nil, time.Time{}, errDiskIOBusy
	}
	defer func() { <-l.waiting }()
	pool.stats.waiting.Add(1)
	service.stats[class].waiting.Add(1)
	defer func() {
		service.stats[class].waiting.Add(-1)
		pool.stats.waiting.Add(-1)
	}()
	deadline = now.Add(l.options.waitTimeout)
	if !requestDeadline.IsZero() && requestDeadline.Before(deadline) {
		deadline = requestDeadline
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	if !haveServiceSlot {
		select {
		case service.slots[class] <- struct{}{}:
			haveServiceSlot = true
		case <-timer.C:
			return nil, time.Time{}, errDiskIOBusy
		}
	}
	if !time.Now().Before(deadline) {
		return nil, time.Time{}, errDiskIOBusy
	}
	select {
	case worker = <-pool.idle:
		return worker, deadline, nil
	case <-timer.C:
		return nil, time.Time{}, errDiskIOBusy
	}
}

// run dispatches filesystem work after parsing the request header. Once handed
// off, the request waits for worker completion before capacity can be reused.
func (l *diskIOLimiter) run(id msgs.BlockServiceId, kind msgs.BlocksMessageKind, requestDeadline time.Time, work func() bool) (bool, error) {
	service := l.services[id]
	if service == nil {
		return false, msgs.BLOCK_SERVICE_NOT_FOUND
	}
	class := diskIORequestClass(kind)
	worker, deadline, err := l.takeWorker(service, class, requestDeadline)
	if err != nil {
		l.pools[class].stats.rejected.Add(1)
		service.stats[class].rejected.Add(1)
		return false, err
	}
	worker.jobs <- diskIOJob{
		service: service, deadline: deadline, work: work,
	}
	// Deliberately do not select on the socket deadline here: the worker
	// might be blocked in a syscall which that deadline cannot interrupt.
	result := <-worker.results
	l.pools[class].idle <- worker
	if result.panicValue != nil {
		panic(result.panicValue)
	}
	if result.err != nil {
		l.pools[class].stats.rejected.Add(1)
		service.stats[class].rejected.Add(1)
	}
	return result.keepConnection, result.err
}

func (l *diskIOLimiter) snapshot() (diskIOCounters, map[msgs.BlockServiceId]diskIOCounters) {
	// Monitoring samples independent atomics; no pool or service is paused.
	global := diskIOCounters{queued: len(l.waiting)}
	for class := range diskIOClassCount {
		global.active[class] = int(l.pools[class].stats.active.Load())
		global.waiting[class] = int(l.pools[class].stats.waiting.Load())
		global.rejected[class] = l.pools[class].stats.rejected.Load()
	}
	services := make(map[msgs.BlockServiceId]diskIOCounters, len(l.services))
	for id, service := range l.services {
		counters := diskIOCounters{queued: len(service.waiting)}
		for class := range diskIOClassCount {
			counters.active[class] = int(service.stats[class].active.Load())
			counters.waiting[class] = int(service.stats[class].waiting.Load())
			counters.rejected[class] = service.stats[class].rejected.Load()
		}
		services[id] = counters
	}
	return global, services
}

func (l *diskIOLimiter) appendMetrics(metrics *log.MetricsBuilder, failureDomain, pathPrefix string, now time.Time) {
	global, services := l.snapshot()
	appendCounters := func(scope, id string, counters diskIOCounters, limits [diskIOClassCount]int) {
		for class, name := range diskIOClassNames {
			metrics.Measurement("eggsfs_blocks_io")
			metrics.Tag("failuredomain", failureDomain)
			metrics.Tag("pathprefix", pathPrefix)
			metrics.Tag("scope", scope)
			if id != "" {
				metrics.Tag("blockservice", id)
			}
			metrics.Tag("operation", name)
			metrics.FieldU64("active", uint64(counters.active[class]))
			metrics.FieldU64("waiting", uint64(counters.waiting[class]))
			metrics.FieldU64("rejected", counters.rejected[class])
			metrics.FieldU64("limit", uint64(limits[class]))
			metrics.Timestamp(now)
		}
	}
	appendCounters("process", "", global, l.options.max)
	for id, counters := range services {
		appendCounters("blockservice", id.String(), counters, l.options.perService)
	}
}
