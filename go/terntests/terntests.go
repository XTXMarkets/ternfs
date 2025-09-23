// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/managedprocess"
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"

	"golang.org/x/sys/unix"
)

func formatNanos(nanos uint64) string {
	var amount float64
	var unit string
	if nanos < 1e3 {
		amount = float64(nanos)
		unit = "ns"
	} else if nanos < 1e6 {
		amount = float64(nanos) / 1e3
		unit = "µs"
	} else if nanos < 1e9 {
		amount = float64(nanos) / 1e6
		unit = "ms"
	} else if nanos < 1e12 {
		amount = float64(nanos) / 1e9
		unit = "s "
	} else if nanos < 1e12*60 {
		amount = float64(nanos) / (1e9 * 60.0)
		unit = "m "
	} else {
		amount = float64(nanos) / (1e9 * 60.0 * 60.0)
		unit = "h "
	}
	return fmt.Sprintf("%7.2f%s", amount, unit)
}

func formatCounters[K ~uint8](what string, counters map[uint8]*client.ReqCounters) {
	fmt.Printf("    %s reqs count/attempts/avg/median/total:\n", what)
	for k, count := range counters {
		if count.Attempts == 0 {
			continue
		}
		fmt.Printf("      %-30v %10v %6.2f %7s %7s %7s\n", K(k), count.Timings.Count(), float64(count.Attempts)/float64(count.Timings.Count()), formatNanos(uint64(count.Timings.Mean().Nanoseconds())), formatNanos(uint64(count.Timings.Median().Nanoseconds())), formatNanos(uint64(count.Timings.TotalTime())))
	}
}

func totalRequests[K comparable](cs map[K]*client.ReqCounters) uint64 {
	total := uint64(0)
	for _, c := range cs {
		total += c.Timings.Count()
	}
	return total
}

type RunTests struct {
	overrides               *cfgOverrides
	registryIp              string
	registryPort            uint16
	mountPoint              string
	fuseMountPoint          string
	kmod                    bool
	short                   bool
	filter                  *regexp.Regexp
	pauseBlockServiceKiller *sync.Mutex
	// If configured, the test start and end markers will also be printed to kmsg.
	// It is done to make it easier to tie kernel logs to the tests that caused them.
	kmsgFd *os.File
}

func (r *RunTests) registryAddress() string {
	return fmt.Sprintf("%s:%d", r.registryIp, r.registryPort)
}

func (r *RunTests) print(format string, a ...any) error {
	fmt.Printf(format, a...)
	if r.kmsgFd != nil {
		s := fmt.Sprintf(format, a...)
		n, err := r.kmsgFd.Write([]byte(s))
		if err != nil {
			return fmt.Errorf("failed writing kmsg: %w", err)
		}
		if n != len(s) {
			return fmt.Errorf("only written %d out of %d to kmsg", n, len(s))
		}
	}
	return nil
}

func (r *RunTests) test(
	log *log.Logger,
	name string,
	extra string,
	run func(counters *client.ClientCounters),
) {
	if !r.filter.Match([]byte(name)) {
		fmt.Printf("skipping test %s\n", name)
		return
	}

	counters := client.NewClientCounters()

	if extra != "" {
		extra = ", " + extra
	}

	r.print("running %s test%s\n", name, extra)
	log.Info("running %s test%s\n", name, extra) // also in log to track progress in CI more easily
	t0 := time.Now()
	run(counters)
	elapsed := time.Since(t0)

	cumShardCounters := make(map[uint8]*client.ReqCounters)
	for shid, c := range counters.Shard {
		cumShardCounters[shid] = client.MergeReqCounters(c[:])
	}
	totalShardRequests := totalRequests(cumShardCounters)
	totalCDCRequests := totalRequests(counters.CDC)
	r.print("  ran test in %v, %v shard requests performed, %v CDC requests performed\n", elapsed, totalShardRequests, totalCDCRequests)
	if totalShardRequests > 0 {
		formatCounters[msgs.ShardMessageKind]("shard", cumShardCounters)
	}
	if totalCDCRequests > 0 {
		formatCounters[msgs.CDCMessageKind]("CDC", counters.CDC)
	}

	counters = client.NewClientCounters()
	t0 = time.Now()
	cleanupAfterTest(log, r.registryAddress(), counters, r.pauseBlockServiceKiller)
	elapsed = time.Since(t0)
	cumShardCounters = make(map[uint8]*client.ReqCounters)
	for shid, c := range counters.Shard {
		cumShardCounters[shid] = client.MergeReqCounters(c[:])
	}
	totalShardRequests = totalRequests(cumShardCounters)
	totalCDCRequests = totalRequests(counters.CDC)
	r.print("  cleanup took %v, %v shard requests performed, %v CDC requests performed\n", elapsed, totalShardRequests, totalCDCRequests)
	if totalShardRequests > 0 {
		formatCounters[msgs.ShardMessageKind]("shard", cumShardCounters)
	}
	if totalCDCRequests > 0 {
		formatCounters[msgs.CDCMessageKind]("CDC", counters.CDC)
	}
}

func getKmodDirRefreshTime() uint64 {
	bs, err := os.ReadFile("/proc/sys/fs/eggsfs/dir_refresh_time_ms")
	if err != nil {
		panic(err)
	}
	ms, err := strconv.ParseUint(strings.TrimSpace(string(bs)), 10, 64)
	if err != nil {
		panic(err)
	}
	return ms
}

func setKmodDirRefreshTime(ms uint64) {
	out, err := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo %v > /proc/sys/fs/eggsfs/dir_refresh_time_ms", ms)).CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("could not set dir_refresh_time (%w): %s", err, out))
	}
}

func mountKmod(registryAddr string, mountPoint string) {
	dev := registryAddr
	out, err := exec.Command("sudo", "mount", "-t", "eggsfs", dev, mountPoint).CombinedOutput()
	if err != nil {
		panic(fmt.Errorf("could not mount filesystem (%w): %s", err, out))
	}
}

type cfgOverrides map[string]string

func (i *cfgOverrides) Set(value string) error {
	parts := strings.Split(value, "=")
	var k, v string
	if len(parts) == 1 {
		k = value
		v = ""
	} else if len(parts) == 2 {
		k = parts[0]
		v = parts[1]
	} else {
		return fmt.Errorf("invalid cfg override %q", value)
	}
	(*i)[k] = v
	return nil
}

func (i *cfgOverrides) String() string {
	pairs := []string{}
	for k, v := range *i {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, ",")
}

func (i *cfgOverrides) int(k string, def int) int {
	s, present := (*i)[k]
	if !present {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func (i *cfgOverrides) float64(k string, def float64) float64 {
	s, present := (*i)[k]
	if !present {
		return def
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(err)
	}
	return n
}

func (i *cfgOverrides) flag(k string) bool {
	s, present := (*i)[k]
	if !present {
		return false
	}
	if s != "" {
		panic(fmt.Errorf("unexpected bool value %q", s))
	}
	return true
}

func (r *RunTests) run(
	terminateChan chan any,
	log *log.Logger,
) {
	defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
	{
		c := newTestClient(log, r.registryAddress(), nil)
		// We want to immediately clean up everything when we run the GC manually
		snapshotPolicy := &msgs.SnapshotPolicy{
			DeleteAfterVersions: msgs.ActiveDeleteAfterVersions(0),
		}
		if err := c.MergeDirectoryInfo(log, msgs.ROOT_DIR_INODE_ID, snapshotPolicy); err != nil {
			panic(err)
		}
		c.Close()
	}

	fileHistoryOpts := fileHistoryTestOpts{
		steps:           r.overrides.int("fileHistory.steps", 10*1000), // perform 10k actions
		checkpointEvery: 100,                                           // get times every 100 actions
		targetFiles:     1000,                                          // how many files we want
		lowFiles:        500,
		threads:         r.overrides.int("fileHistory.threads", 5),
	}
	if r.short {
		fileHistoryOpts.targetFiles = 100
		fileHistoryOpts.lowFiles = 50
		fileHistoryOpts.steps = r.overrides.int("fileHistory.steps", 1000)
	}
	r.test(
		log,
		"file history",
		fmt.Sprintf("%v threads, %v steps", fileHistoryOpts.threads, fileHistoryOpts.steps),
		func(counters *client.ClientCounters) {
			fileHistoryTest(log, r.registryAddress(), &fileHistoryOpts, counters)
		},
	)

	fsTestOpts := fsTestOpts{
		depth:        4,
		maxFileSize:  r.overrides.int("fsTest.maxFileSize", 10<<20), // 10MiB
		spanSize:     1 << 20,                                       // 1MiB
		checkThreads: r.overrides.int("fsTest.checkThreads", 5),
	}

	// 0.03 * 20*1000 * 5MiB = ~3GiB of file data
	if r.short {
		fsTestOpts.numDirs = r.overrides.int("fsTest.numDirs", 10)
		fsTestOpts.numFiles = r.overrides.int("fsTest.numFiles", 500)
		fsTestOpts.emptyFileProb = 0.1
		fsTestOpts.inlineFileProb = 0.3
	} else {
		fsTestOpts.numDirs = r.overrides.int("fsTest.numDirs", 400)
		fsTestOpts.numFiles = r.overrides.int("fsTest.numFiles", 10*400)
		fsTestOpts.emptyFileProb = 0.1
		fsTestOpts.inlineFileProb = 0.3
	}
	fsTestOpts.corruptFileProb = r.overrides.float64("fsTest.corruptFileProb", 0.1)
	fsTestOpts.migrate = !r.overrides.flag("fsTest.dontMigrate")
	fsTestOpts.defrag = !r.overrides.flag("fsTest.dontDefrag")
	fsTestOpts.readWithMmap = r.overrides.flag("fsTest.readWithMmap")

	r.test(
		log,
		"direct fs",
		fmt.Sprintf("%v dirs, %v files, %v depth", fsTestOpts.numDirs, fsTestOpts.numFiles, fsTestOpts.depth),
		func(counters *client.ClientCounters) {
			fsTest(log, r.registryAddress(), &fsTestOpts, counters, apiHarness{})
		},
	)

	r.test(
		log,
		"mounted fs",
		fmt.Sprintf("%v dirs, %v files, %v depth", fsTestOpts.numDirs, fsTestOpts.numFiles, fsTestOpts.depth),
		func(counters *client.ClientCounters) {
			fsTest(log, r.registryAddress(), &fsTestOpts, counters, posixHarness{r.mountPoint})
		},
	)

	r.test(
		log,
		"s3 fs",
		fmt.Sprintf("%v dirs, %v files, %v depth", fsTestOpts.numDirs, fsTestOpts.numFiles, fsTestOpts.depth),
		func(counters *client.ClientCounters) {
			fsTest(log, r.registryAddress(), &fsTestOpts, counters, s3Harness{})
		},
	)

	parallelDirsOpts := &parallelDirsOpts{
		numRootDirs:      10,
		numThreads:       100,
		actionsPerThread: 100,
	}
	if r.short {
		parallelDirsOpts.actionsPerThread = 10
	}
	r.test(
		log,
		"parallel dirs",
		fmt.Sprintf("%v root dirs, %v threads, %v actions per thread", parallelDirsOpts.numRootDirs, parallelDirsOpts.numThreads, parallelDirsOpts.actionsPerThread),
		func(counters *client.ClientCounters) {
			parallelDirsTest(log, r.registryAddress(), counters, parallelDirsOpts)
		},
	)

	largeFileOpts := largeFileTestOpts{
		fileSize: 1 << 30, // 1GiB
	}
	r.test(
		log,
		"large file",
		fmt.Sprintf("%vGB", float64(largeFileOpts.fileSize)/1e9),
		func(counters *client.ClientCounters) {
			largeFileTest(log, &largeFileOpts, r.mountPoint)
		},
	)

	rsyncOpts := rsyncTestOpts{
		maxFileSize: 200 << 20, // 200MiB
		numFiles:    100,       // 20GiB
		numDirs:     10,
	}
	if r.short {
		rsyncOpts.numFiles = 10
		rsyncOpts.numDirs = 1
	}
	r.test(
		log,
		"rsync large",
		fmt.Sprintf("%v files, %v dirs, %vMB file size", rsyncOpts.numFiles, rsyncOpts.numDirs, float64(rsyncOpts.maxFileSize)/1e6),
		func(counters *client.ClientCounters) {
			rsyncTest(log, &rsyncOpts, r.mountPoint)
		},
	)

	rsyncOpts = rsyncTestOpts{
		maxFileSize: 1 << 20, // 1Mib
		numFiles:    10000,   // 10GiB
		numDirs:     1000,
	}
	if r.short {
		rsyncOpts.numFiles /= 10
		rsyncOpts.numDirs /= 10
	}
	r.test(
		log,
		"rsync small",
		fmt.Sprintf("%v files, %v dirs, %vMB file size", rsyncOpts.numFiles, rsyncOpts.numDirs, float64(rsyncOpts.maxFileSize)/1e6),
		func(counters *client.ClientCounters) {
			rsyncTest(log, &rsyncOpts, r.mountPoint)
		},
	)

	r.test(
		log,
		"cp",
		"",
		func(counters *client.ClientCounters) {
			from := path.Join(r.mountPoint, "test-1")
			to := path.Join(r.mountPoint, "test-2")
			contents := []byte("foo")
			if err := ioutil.WriteFile(from, contents, 0644); err != nil {
				panic(err)
			}
			if err := exec.Command("cp", from, to).Run(); err != nil {
				panic(err)
			}
			contents1, err := os.ReadFile(from)
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(contents, contents1) {
				panic(fmt.Errorf("expected %v, got %v", contents, contents1))
			}
			contents2, err := os.ReadFile(from)
			if err != nil {
				panic(err)
			}
			if !bytes.Equal(contents, contents2) {
				panic(fmt.Errorf("expected %v, got %v", contents, contents2))
			}
			/*
				// now try shell redirection
				f := path.Join(r.mountPoint, "test-3")
				if err := exec.Command("sh", "-c", fmt.Sprintf("echo foo > %s", f)); err != nil {
					panic(err)
				}
				contents3, err := os.ReadFile(from)
				if err != nil {
					panic(err)
				}
				if !bytes.Equal(contents, contents3) {
					panic(fmt.Errorf("expected %v, got %v", contents, contents3))
				}
			*/
		},
	)

	r.test(
		log,
		"utime",
		"",
		func(counters *client.ClientCounters) {
			fn := path.Join(r.mountPoint, "test")
			if err := os.WriteFile(fn, []byte{}, 0644); err != nil {
				panic(err)
			}
			time1 := time.Now()
			time2 := time.Now()
			if time.Time.Equal(time1, time2) {
				panic(fmt.Errorf("same times"))
			}
			if err := syscall.UtimesNano(fn, []syscall.Timespec{syscall.NsecToTimespec(time1.UnixNano()), syscall.NsecToTimespec(time2.UnixNano())}); err != nil {
				panic(err)
			}
			info, err := os.Stat(fn)
			if err != nil {
				panic(err)
			}
			stat_t := info.Sys().(*syscall.Stat_t)
			atime := time.Unix(int64(stat_t.Atim.Sec), int64(stat_t.Atim.Nsec))
			if atime.UnixNano() != time1.UnixNano() {
				panic(fmt.Errorf("expected atime %v, got %v", time1, atime))
			}
			mtime := time.Unix(int64(stat_t.Mtim.Sec), int64(stat_t.Mtim.Nsec))
			if mtime.UnixNano() != time2.UnixNano() {
				panic(fmt.Errorf("expected mtime %v, got %v", time2, mtime))
			}
			// atime is updated when opened
			now := time.Now()
			time.Sleep(time.Millisecond)
			if _, err := os.ReadFile(fn); err != nil {
				panic(err)
			}
			time.Sleep(time.Second) // try to flush out OS caches, esp in FUSE
			info, err = os.Stat(fn)
			if err != nil {
				panic(err)
			}
			stat_t = info.Sys().(*syscall.Stat_t)
			atime = time.Unix(int64(stat_t.Atim.Sec), int64(stat_t.Atim.Nsec))
			if now.UnixNano() > atime.UnixNano() {
				panic(fmt.Errorf("atime didn't update, %v > %v", now, atime))
			}
			// make sure O_NOATIME is respected
			file, err := os.OpenFile(fn, os.O_RDONLY|syscall.O_NOATIME, 0)
			if err != nil {
				panic(err)
			}
			time.Sleep(time.Millisecond)
			info, err = os.Stat(fn)
			file.Close()
			if err != nil {
				panic(err)
			}
			stat_t = info.Sys().(*syscall.Stat_t)
			newAtime := time.Unix(int64(stat_t.Atim.Sec), int64(stat_t.Atim.Nsec))
			if atime != newAtime {
				panic(fmt.Errorf("expected atime to be still %v, but got %v", atime, newAtime))
			}
		},
	)

	r.test(
		log,
		"seek to extend",
		"",
		func(counters *client.ClientCounters) {
			rand := wyhash.New(42)
			for i := 0; i < 100; i++ {
				fn := path.Join(r.mountPoint, fmt.Sprintf("test%v", i))
				f, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					panic(err)
				}
				// Write ~50MB, alternating between real content and holes
				// everything random so we try to cheaply stimulate all possible start/end
				// configurations.
				data := []byte{}
				fileSize := int64(0)
				offset := int64(0)
				for i := 0; i < 10; i++ {
					// the lowest value where we have multiple pages
					// testing multiple spans is probably not worth it, the code
					// is identical
					sz := int64(rand.Uint64()%10000 + 1) // ]0, 10000]
					if rand.Uint32()%2 == 0 {            // hole
						var whence int
						var seekOffset int64
						expectedOffset := offset + sz
						switch rand.Uint32() % 3 {
						case 0:
							whence = io.SeekStart
							seekOffset = expectedOffset
						case 1:
							whence = io.SeekCurrent
							seekOffset = expectedOffset - offset
						case 2:
							whence = io.SeekEnd
							seekOffset = expectedOffset - fileSize
						}
						log.Debug("extending %v with %v zeros using seek (offset=%v fileSize=%v seekOffset=%v whence=%v)", fn, sz, offset, fileSize, seekOffset, whence)
						retOffset, err := f.Seek(seekOffset, whence)
						if err != nil {
							panic(err)
						}
						if retOffset != expectedOffset {
							panic(fmt.Errorf("unexpected offset %v, expected %v (whence=%v, %v + %v)", retOffset, expectedOffset, whence, len(data), sz))
						}
						offset = expectedOffset
					} else { // read data
						log.Debug("extending %v with %v random bytes using write", fn, sz)
						chunk := make([]byte, sz)
						if _, err := rand.Read(chunk); err != nil {
							panic(err)
						}
						if _, err := f.Write(chunk); err != nil {
							panic(err)
						}
						// append zeros
						data = append(data, make([]byte, int(offset)-len(data))...)
						// append chunk
						data = append(data, chunk...)
						fileSize = int64(len(data))
						offset = fileSize
					}
				}
				if err := f.Close(); err != nil {
					panic(err)
				}
				// read back
				readData, err := os.ReadFile(fn)
				if err != nil {
					panic(err)
				}
				if !bytes.Equal(data, readData) {
					panic(fmt.Errorf("mismatching data"))
				}
			}
		},
	)

	dirSeekPaths := 10_000
	if r.short {
		dirSeekPaths = 1_000
	}
	r.test(
		log,
		"dir seek",
		"",
		func(counters *client.ClientCounters) {
			dirSeekTest(log, r.registryAddress(), r.mountPoint, dirSeekPaths)
		},
	)

	parallelWriteThreads := 10000
	if r.short {
		parallelWriteThreads = 1000
	}
	r.test(
		log,
		"parallel write",
		fmt.Sprintf("%v threads", parallelWriteThreads),
		func(counters *client.ClientCounters) {
			bufPool := bufpool.NewBufPool()
			dirInfoCache := client.NewDirInfoCache()
			var wg sync.WaitGroup
			wg.Add(parallelWriteThreads)
			c := newTestClient(log, r.registryAddress(), counters)
			defer c.Close()
			for i := 0; i < parallelWriteThreads; i++ {
				ti := i
				go func() {
					defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
					if _, err := c.CreateFile(log, bufPool, dirInfoCache, fmt.Sprintf("/%d", ti), bytes.NewReader([]byte{})); err != nil {
						panic(err)
					}
					wg.Done()
				}()
			}
			wg.Wait()
		},
	)

	r.test(
		log,
		"ftruncate",
		"",
		func(counters *client.ClientCounters) {
			rand := wyhash.New(42)
			for i := 0; i < 100; i++ {
				fn := path.Join(r.mountPoint, fmt.Sprintf("test%v", i))
				f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					panic(err)
				}
				sz := int64(rand.Uint64()%100000 + 1)

				data := make([]byte, sz)
				if _, err := rand.Read(data); err != nil {
					panic(err)
				}
				if _, err := f.Write(data); err != nil {
					panic(err)
				}

				var expectedErr bool
				var bytesAdded int64
				var expectedSize int64
				switch rand.Uint32() % 3 {
				case 0:
					bytesAdded = 0
					expectedErr = false
					expectedSize = sz
				case 1:
					bytesAdded = int64(rand.Uint64()%uint64(sz) + 1)
					expectedErr = false
					expectedSize = sz + bytesAdded
					data = append(data, make([]byte, int(bytesAdded))...)
				case 2:
					bytesAdded = -1 * int64(rand.Uint64()%uint64(sz)+1)
					expectedErr = true
					expectedSize = sz
				}

				log.Debug("extending %v of size %v with %v zeros using ftruncate", fn, sz, bytesAdded)
				err = f.Truncate(sz + bytesAdded)
				if (err != nil) != expectedErr {
					panic(err)
				}

				retOffset, err := f.Seek(0, io.SeekCurrent)
				if err != nil {
					panic(err)
				}
				if retOffset != sz {
					panic(fmt.Errorf("position changed after ftruncate(): %v %v, expected %v", fn, retOffset, sz))
				}

				if err := f.Close(); err != nil {
					panic(err)
				}
				// read back
				readData, err := os.ReadFile(fn)
				if err != nil {
					panic(err)
				}
				if !bytes.Equal(data, readData) {
					panic(fmt.Errorf("mismatching data"))
				}
				st, err := os.Stat(fn)
				if err != nil {
					panic(err)
				}
				if st.Size() != expectedSize {
					panic(fmt.Errorf("expected size after ftruncate: %v, got %v", expectedSize, st.Size()))
				}
			}
		},
	)

	terminateChan <- nil
}

func noRunawayArgs() {
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Unexpected extra arguments %v\n", flag.Args())
		os.Exit(2)
	}
}

type blockServiceVictim struct {
	failureDomain  string
	path           string
	storageClasses []msgs.StorageClass
}

func (bsv *blockServiceVictim) start(
	log *log.Logger,
	blocksExe string,
	registryPort uint16,
	port1 uint16,
	port2 uint16,
	profile bool,
	procs *managedprocess.ManagedProcesses,
) managedprocess.ManagedProcessId {
	return procs.StartBlockService(log, &managedprocess.BlockServiceOpts{
		Exe:              blocksExe,
		Path:             bsv.path,
		StorageClasses:   bsv.storageClasses,
		FailureDomain:    bsv.failureDomain,
		LogLevel:         log.Level(),
		RegistryAddress:  fmt.Sprintf("127.0.0.1:%d", registryPort),
		FutureCutoff:     &testBlockFutureCutoff,
		Addr1:            fmt.Sprintf("127.0.0.1:%d", port1),
		Addr2:            fmt.Sprintf("127.0.0.1:%d", port2),
		Profile:          profile,
		ReserverdStorage: 10 << 30, // 10 GiB
	})
}

func killBlockServices(
	log *log.Logger,
	terminateChan chan any,
	stopChan chan struct{},
	pause *sync.Mutex,
	blocksExe string,
	registryPort uint16,
	profile bool,
	procs *managedprocess.ManagedProcesses,
	bsProcs map[managedprocess.ManagedProcessId]blockServiceVictim,
	bsPorts map[msgs.FailureDomain]struct {
		_1 uint16
		_2 uint16
	},
) {
	// right now the kmod does not really digest multiple dead block services
	// at once when writing, because it only remembers the failures for the current
	// request. we should make it so that older failures are remembered too.
	// in any case, this means that for now we only kill one block service at a time.
	killDuration := time.Second * 10
	gracePeriod := time.Second * 10
	log.Info("will kill block service for %v", killDuration)
	rand := wyhash.New(uint64(time.Now().UnixNano()))
	go func() {
		defer func() { lrecover.HandleRecoverChan(log, terminateChan, recover()) }()
		lc := net.ListenConfig{
			Control: func(network, address string, c syscall.RawConn) error {
				var operr error
				err := c.Control(func(fd uintptr) {
					operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				})
				if err != nil {
					return err
				}
				return operr
			},
		}
		var reservedPorts [2]net.Listener
		for {
			// pick and kill the victim
			pause.Lock()
			var victim blockServiceVictim
			{
				ix := int(rand.Uint64()) % len(bsProcs)
				j := 0
				var procId managedprocess.ManagedProcessId
				for procId = range bsProcs {
					if j >= ix {
						victim = bsProcs[procId]
						delete(bsProcs, procId)
						var failureDomain msgs.FailureDomain
						copy(failureDomain.Name[:], victim.failureDomain)
						ports := bsPorts[failureDomain]
						log.Info("killing %v, ports %v %v", victim.path, ports._1, ports._2)
						procs.Kill(procId, syscall.SIGKILL)
						if ports._1 != 0 {
							if port1Listener, err := lc.Listen(context.Background(), "tcp4", fmt.Sprintf("127.0.0.1:%d", ports._1)); err == nil {
								reservedPorts[0] = port1Listener
							}
						}
						if ports._2 != 0 {
							if port2Listener, err := lc.Listen(context.Background(), "tcp4", fmt.Sprintf("127.0.0.1:%d", ports._2)); err == nil {
								reservedPorts[1] = port2Listener
							}
						}
						break
					}
					j++
				}
			}
			pause.Unlock()
			// wait
			sleepChan := make(chan struct{}, 1)
			go func() {
				time.Sleep(killDuration)
				sleepChan <- struct{}{}
			}()
			select {
			case <-stopChan:
				for i, ls := range reservedPorts {
					if ls != nil {
						ls.Close()
						reservedPorts[i] = nil
					}
				}
				stopChan <- struct{}{} // reply
				return
			case <-sleepChan:
			}
			// revive the victims -- important to preserve ports
			// so that shard blockservice cache data won't be
			// invalidated (we don't want to have to wait for it
			// to be refreshed for the tests to work)
			{
				log.Info("reviving %v", victim.path)
				var failureDomain msgs.FailureDomain
				copy(failureDomain.Name[:], victim.failureDomain)
				ports := bsPorts[failureDomain]
				for i, ls := range reservedPorts {
					if ls != nil {
						ls.Close()
						reservedPorts[i] = nil
					}
				}
				procId := victim.start(
					log,
					blocksExe,
					registryPort,
					ports._1,
					ports._2,
					profile,
					procs,
				)
				bsProcs[procId] = victim
			}
			// wait
			go func() {
				time.Sleep(gracePeriod)
				sleepChan <- struct{}{}
			}()
			select {
			case <-stopChan:
				stopChan <- struct{}{} // reply
				return
			case <-sleepChan:
			}
		}
	}()
}

var shardTimeouts timing.ReqTimeouts
var cdcTimeouts timing.ReqTimeouts
var blockTimeouts timing.ReqTimeouts

func newTestClient(log *log.Logger, registryAddress string, counters *client.ClientCounters) *client.Client {
	c, err := client.NewClient(log, nil, registryAddress, msgs.AddrsInfo{})
	if err != nil {
		panic(err)
	}
	c.SetFetchBlockServices()
	c.SetCounters(counters)
	c.SetShardTimeouts(&shardTimeouts)
	c.SetCDCTimeouts(&cdcTimeouts)
	c.SetBlockTimeouts(&blockTimeouts)
	return c
}

// 0 interval won't do, because otherwise transient files will immediately be
// expired and not picked.
var testTransientDeadlineInterval = 30 * time.Second
var testBlockFutureCutoff = testTransientDeadlineInterval / 2

func main() {
	overrides := make(cfgOverrides)

	buildType := flag.String("build-type", "release", "C++ build type")
	verbose := flag.Bool("verbose", false, "")
	trace := flag.Bool("trace", false, "")
	kmsg := flag.Bool("kmsg", false, "")
	dataDir := flag.String("data-dir", "", "Directory where to store the TernFS data. If not present a temporary directory will be used.")
	preserveDbDir := flag.Bool("preserve-data-dir", false, "Whether to preserve the temp data dir (if we're using a temp data dir).")
	filter := flag.String("filter", "", "Regex to match against test names -- only matching ones will be ran.")
	profileTest := flag.Bool("profile-test", false, "Run the test driver with profiling. Implies -preserve-data-dir")
	profile := flag.Bool("profile", false, "Run with profiling (this includes the C++ and Go binaries and the test driver). Implies -preserve-data-dir")
	outgoingPacketDrop := flag.Float64("outgoing-packet-drop", 0.0, "Simulate packet drop in shard (the argument is the probability that any packet will be dropped). This one will process the requests, but drop the responses.")
	short := flag.Bool("short", false, "Run a shorter version of the tests")
	repoDir := flag.String("repo-dir", "", "Used to build C++/Go binaries. If not provided, the path will be derived form the filename at build time (so will only work locally).")
	binariesDir := flag.String("binaries-dir", "", "If provided, nothing will be built, instead it'll be assumed that the binaries will be in the specified directory.")
	kmod := flag.Bool("kmod", false, "Whether to mount with the kernel module, rather than FUSE. Note that the tests will not attempt to run the kernel module and load it, they'll just mount with 'mount -t eggsfs'.")
	dropFetchBlockSocketsEvery := flag.Duration("drop-fetch-block-sockets-every", 0, "")
	dirRefreshTime := flag.Duration("dir-refresh-time", 0, "If set, it will set the kmod /proc/sys/fs/eggsfs/dir_refresh_time_ms")
	mtu := flag.Uint64("mtu", 0, "If set, we'll use the given MTU for big requests.")
	tmpDir := flag.String("tmp-dir", "", "")
	registryPortArg := flag.Uint("registry-port", 55555, "")
	blockServiceKiller := flag.Bool("block-service-killer", false, "Go around killing block services to stimulate paths recovering from that.")
	race := flag.Bool("race", false, "Go race detector")
	leaderOnly := flag.Bool("leader-only", false, "Run only LogsDB leader with LEADER_NO_FOLLOWERS")
	closeTrackerObject := flag.String("close-tracker-object", "", "Compiled BPF object to track explicitly closed files")
	flag.Var(&overrides, "cfg", "Config overrides")
	flag.Parse()
	noRunawayArgs()

	{
		shardTimeouts = client.DefaultShardTimeout
		cdcTimeouts = client.DefaultCDCTimeout
		blockTimeouts = client.DefaultBlockTimeout

		// In tests where we intentionally drop packets this makes things _much_
		// faster
		shardTimeouts.Initial = 5 * time.Millisecond
		cdcTimeouts.Initial = 10 * time.Millisecond
		// Tests fail frequently hitting various timeouts. Higher Max and Overall
		// timeouts makes tests much more stable
		shardTimeouts.Max = 20 * time.Second
		cdcTimeouts.Max = 20 * time.Second
		shardTimeouts.Overall = 60 * time.Second
		cdcTimeouts.Overall = 60 * time.Second
		// Retry block stuff quickly to avoid being starved by the block service
		// killer (and also to go faster)
		blockTimeouts.Max = time.Second
		blockTimeouts.Overall = 10 * time.Minute // for block service killer tests
	}

	filterRe := regexp.MustCompile(*filter)

	cleanupDbDir := false
	tmpDataDir := *dataDir == ""
	if tmpDataDir {
		dir, err := os.MkdirTemp(*tmpDir, "tern-integrationtest.")
		if err != nil {
			panic(fmt.Errorf("could not create tmp data dir: %w", err))
		}
		*dataDir = dir
		fmt.Printf("running with temp data dir %v\n", *dataDir)
	}
	defer func() {
		if cleanupDbDir {
			fmt.Printf("cleaning up temp data dir %v\n", *dataDir)
			os.RemoveAll(*dataDir)
		} else if tmpDataDir {
			fmt.Printf("preserved temp data dir %v\n", *dataDir)
		}
	}()
	if *dirRefreshTime != 0 {
		fmt.Printf("setting dir refresh time to %v\n", *dirRefreshTime)
		before := getKmodDirRefreshTime()
		setKmodDirRefreshTime(uint64(dirRefreshTime.Milliseconds()))
		defer setKmodDirRefreshTime(before)
	}

	if *profileTest {
		f, err := os.Create(path.Join(*dataDir, "test-profile"))
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *repoDir == "" {
		_, filename, _, ok := runtime.Caller(0)
		if !ok {
			panic("no caller information")
		}
		*repoDir = path.Dir(path.Dir(path.Dir(filename)))
	}

	logFile := path.Join(*dataDir, "test-log")
	var logOut *os.File
	{
		var err error
		logOut, err = os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not open file %v: %v\n", logFile, err)
			os.Exit(1)
		}
		defer logOut.Close()
	}
	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}
	l := log.NewLogger(logOut, &log.LoggerOptions{Level: level, Syslog: false, PrintQuietAlerts: true})

	if *mtu > 0 {
		l.Info("Setting MTU to %v", *mtu)
		client.SetMTU(*mtu)
	}

	var cppExes *managedprocess.CppExes
	var goExes *managedprocess.GoExes
	if *binariesDir != "" {
		cppExes = &managedprocess.CppExes{
			RegistryExe: path.Join(*binariesDir, "ternregistry"),
			ShardExe:    path.Join(*binariesDir, "ternshard"),
			CDCExe:      path.Join(*binariesDir, "terncdc"),
			DBToolsExe:  path.Join(*binariesDir, "terndbtools"),
		}
		goExes = &managedprocess.GoExes{
			BlocksExe: path.Join(*binariesDir, "ternblocks"),
			FuseExe:   path.Join(*binariesDir, "ternfuse"),
		}
	} else {
		fmt.Printf("building shard/cdc/blockservice/registry\n")
		cppExes = managedprocess.BuildCppExes(l, *repoDir, *buildType)
		goExes = managedprocess.BuildGoExes(l, *repoDir, *race)
	}

	terminateChan := make(chan any, 1)

	procs := managedprocess.New(terminateChan)
	procsClosed := false
	defer func() {
		if !procsClosed {
			procsClosed = true
			procs.Close()
		}
	}()

	replicaCount := uint8(5)
	if *leaderOnly {
		replicaCount = 1
	}

	// If we're running with kmod, set the timeout to be very high
	// (in qemu things can be delayed massively). TODO it would be
	// nice to understand the circumstances where the timeout
	// is so high, even inside a very stressed QEMU.
	if *kmod {
		cmd := exec.Command("sudo", "dmidecode", "-s", "system-manufacturer")
		dmiOut, err := cmd.Output()
		if err != nil {
			panic(err)
		}
		sysctl := func(what string, val string) {
			cmd = exec.Command("sudo", "/usr/sbin/sysctl", fmt.Sprintf("%s=%s", what, val))
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if err := cmd.Run(); err != nil {
				panic(err)
			}
		}
		isQEMU := strings.TrimSpace(string(dmiOut)) == "QEMU"
		if isQEMU {
			l.Info("increasing metadata timeout since we're in QEMU")
			sysctl("fs.eggsfs.overall_shard_timeout_ms", "60000")
			sysctl("fs.eggsfs.overall_cdc_timeout_ms", "60000")
		}
		// low initial timeout for fast packet drop stuff
		sysctl("fs.eggsfs.initial_shard_timeout_ms", fmt.Sprintf("%v", 5))
		sysctl("fs.eggsfs.initial_cdc_timeout_ms", fmt.Sprintf("%v", 5))
		// low getattr expiry which we rely on in some tests (most notably the utime ones)
		sysctl("fs.eggsfs.file_getattr_refresh_time_ms", "100")
	}

	// Start fetch block socket dropper
	if *dropFetchBlockSocketsEvery != time.Duration(0) {
		if !*kmod {
			panic(fmt.Errorf("you provided -drop-fetch-block-sockets-every but you did't provide -kmod"))
		}
		fmt.Printf("will drop fetch block sockets every %v\n", *dropFetchBlockSocketsEvery)
		go func() {
			for {
				cmd := exec.Command("sudo", "/usr/sbin/sysctl", "fs.eggsfs.drop_fetch_block_sockets=1")
				cmd.Stdout = io.Discard
				cmd.Stderr = io.Discard
				if err := cmd.Run(); err != nil {
					terminateChan <- err
					return
				}
				time.Sleep(*dropFetchBlockSocketsEvery)
			}
		}()
	}

	registryPort := uint16(*registryPortArg)
	registryAddress := fmt.Sprintf("127.0.0.1:%v", registryPort)

	{
		for r := uint8(0); r < replicaCount; r++ {
			dir := path.Join(*dataDir, fmt.Sprintf("registry_%d", r))
			if r == 0 {
				dir = path.Join(*dataDir, "registry")
			}
			opts := managedprocess.RegistryOpts{
				Exe:               cppExes.RegistryExe,
				LogLevel:          level,
				Dir:               dir,
				RegistryAddress:   registryAddress,
				Replica:           msgs.ReplicaId(r),
				UsingDynamicPorts: true,
			}
			if r == 0 {
				if *leaderOnly {
					opts.LogsDBFlags = []string{"-logsdb-leader", "-logsdb-no-replication"}
				} else {
					opts.LogsDBFlags = []string{"-logsdb-leader"}
				}
			}
			if r == 0 {
				opts.Addr1 = fmt.Sprintf("127.0.0.1:%v", registryPort)
			} else {
				opts.Addr1 = "127.0.0.1:0"
			}

			if *blockServiceKiller {
				opts.Stale = time.Hour * 1000 // never, so that we stimulate the clients ability to fallback
			}
			procs.StartRegistry(l, &opts)
		}
	}

	err := client.WaitForRegistry(l, registryAddress, 10*time.Second)
	if err != nil {
		panic(fmt.Errorf("failed to connect to registry %v", err))
	}

	failureDomains := 14 + 4 // so that any 4 can fail and we can still do everything.
	hddBlockServices := 10
	flashBlockServices := 10
	blockServicesProcs := make(map[managedprocess.ManagedProcessId]blockServiceVictim)
	{
		storageClasses := make([]msgs.StorageClass, hddBlockServices+flashBlockServices)
		for i := 0; i < hddBlockServices+flashBlockServices; i++ {
			storageClasses[i] = msgs.HDD_STORAGE
			if i >= hddBlockServices {
				storageClasses[i] = msgs.FLASH_STORAGE
			}
		}
		for i := 0; i < failureDomains; i++ {
			bsv := blockServiceVictim{
				failureDomain:  fmt.Sprintf("%d", i),
				path:           path.Join(*dataDir, fmt.Sprintf("bs_%d", i)),
				storageClasses: storageClasses,
			}
			procId := bsv.start(
				l,
				goExes.BlocksExe,
				registryPort,
				0, 0,
				*profile,
				procs,
			)
			blockServicesProcs[procId] = bsv
		}
	}

	waitRegistryFor := 60 * time.Second
	if *buildType == "valgrind" || *profile || *kmod { // kmod often runs in qemu, which is slower
		waitRegistryFor = 60 * time.Second
	}

	// wait for block services first, so we know that shards will immediately have all of them
	fmt.Printf("waiting for block services for %v...\n", waitRegistryFor)
	blockServices := client.WaitForBlockServices(l, fmt.Sprintf("127.0.0.1:%v", registryPort), failureDomains*(hddBlockServices+flashBlockServices), true, waitRegistryFor)
	blockServicesPorts := make(map[msgs.FailureDomain]struct {
		_1 uint16
		_2 uint16
	})

	for _, bs := range blockServices {
		blockServicesPorts[bs.FailureDomain] = struct {
			_1 uint16
			_2 uint16
		}{bs.Addrs.Addr1.Port, bs.Addrs.Addr2.Port}
	}

	if *outgoingPacketDrop > 0 {
		fmt.Printf("will drop %0.2f%% of packets after executing requests\n", *outgoingPacketDrop*100.0)
	}

	// Start CDC
	for r := uint8(0); r < replicaCount; r++ {
		cdcOpts := &managedprocess.CDCOpts{
			ReplicaId:       msgs.ReplicaId(r),
			Exe:             cppExes.CDCExe,
			Dir:             path.Join(*dataDir, fmt.Sprintf("cdc_%d", r)),
			LogLevel:        level,
			Valgrind:        *buildType == "valgrind",
			Perf:            *profile,
			RegistryAddress: registryAddress,
			Addr1:           "127.0.0.1:0",
			Addr2:           "127.0.0.1:0",
		}
		if r == 0 {
			if *leaderOnly {
				cdcOpts.LogsDBFlags = []string{"-logsdb-leader", "-logsdb-no-replication"}
			} else {
				cdcOpts.LogsDBFlags = []string{"-logsdb-leader"}
			}
		}
		if *buildType == "valgrind" {
			// apparently 100ms is too little when running with valgrind
			cdcOpts.ShardTimeout = time.Millisecond * 500
		}
		procs.StartCDC(l, *repoDir, cdcOpts)
	}

	// Start shards
	numShards := 256
	for i := 0; i < numShards; i++ {
		for r := uint8(0); r < replicaCount; r++ {
			shrid := msgs.MakeShardReplicaId(msgs.ShardId(i), msgs.ReplicaId(r))
			shopts := managedprocess.ShardOpts{
				Exe:                       cppExes.ShardExe,
				Dir:                       path.Join(*dataDir, fmt.Sprintf("shard_%03d_%d", i, r)),
				LogLevel:                  level,
				Shrid:                     shrid,
				Valgrind:                  *buildType == "valgrind",
				Perf:                      *profile,
				OutgoingPacketDrop:        *outgoingPacketDrop,
				RegistryAddress:           registryAddress,
				Addr1:                     "127.0.0.1:0",
				Addr2:                     "127.0.0.1:0",
				TransientDeadlineInterval: &testTransientDeadlineInterval,
				LogsDBFlags:               nil,
			}
			if r == 0 {
				if *leaderOnly {
					shopts.LogsDBFlags = []string{"-logsdb-leader", "-logsdb-no-replication"}
				} else {
					shopts.LogsDBFlags = []string{"-logsdb-leader"}
				}
			}
			procs.StartShard(l, *repoDir, &shopts)
		}
	}

	// now wait for shards/cdc
	fmt.Printf("waiting for shards/cdc for %v...\n", waitRegistryFor)
	client.WaitForClient(l, fmt.Sprintf("127.0.0.1:%v", registryPort), waitRegistryFor)

	var stopBlockServiceKiller chan struct{}
	var pauseBlockServiceKiller sync.Mutex
	if *blockServiceKiller {
		fmt.Printf("will kill block services\n")
		stopBlockServiceKiller = make(chan struct{}, 1)
		killBlockServices(l, terminateChan, stopBlockServiceKiller, &pauseBlockServiceKiller, goExes.BlocksExe, registryPort, *profile, procs, blockServicesProcs, blockServicesPorts)
		// stop before trying to clean up data dir etc.
		defer func() {
			stopBlockServiceKiller <- struct{}{}
			<-stopBlockServiceKiller
		}()
	}

	fuseMountPoint := procs.StartFuse(l, &managedprocess.FuseOpts{
		Exe:                goExes.FuseExe,
		Path:               path.Join(*dataDir, "fuse"),
		LogLevel:           level,
		Wait:               true,
		RegistryAddress:    registryAddress,
		Profile:            *profile,
		ShardTimeouts:      shardTimeouts,
		CDCTimeouts:        cdcTimeouts,
		BlockTimeouts:      blockTimeouts,
		CloseTrackerObject: *closeTrackerObject,
		SetUid:             true,
	})

	var mountPoint string
	if *kmod {
		mountPoint = path.Join(*dataDir, "kmod", "mnt")
		err := os.MkdirAll(mountPoint, 0777)
		if err != nil {
			panic(err)
		}
		mountKmod(registryAddress, mountPoint)
		defer func() {
			l.Info("about to unmount kmod mount")
			out, err := exec.Command("sudo", "umount", mountPoint).CombinedOutput()
			l.Info("done unmounting")
			if err != nil {
				fmt.Printf("could not umount fs (%v): %s", err, out)
			}
		}()
	} else {
		mountPoint = fuseMountPoint
	}

	fmt.Printf("operational 🤖\n")

	// Start block policy changer -- this is useful to test kmod/policy.c machinery
	/*
		if *changeBlockPolicyEvery != time.Duration(0) {
			fmt.Printf("will change block policy every %v\n", *dropFetchBlockSocketsEvery)
			c, err := client.NewClient(log, nil, registryAddress)
			if err != nil {
				panic(err)
			}
			defer c.Close()
			updateTimeouts(c)
			blockPolicy := &msgs.BlockPolicy{}
			if _, err := c.ResolveDirectoryInfoEntry(log, client.NewDirInfoCache(), msgs.ROOT_DIR_INODE_ID, blockPolicy); err != nil {
				panic(err)
			}
			if len(blockPolicy.Entries) != 2 || blockPolicy.Entries[0].MinSize != 0 {
				panic(fmt.Errorf("bad block policy %+v", blockPolicy))
			}
			go func() {
				i := 0
				// ./eggs/eggstests -kmod -binaries-dir eggs -filter 'direct' -short -change-block-policy-every 1s
				for {
					d := 1
					if (i & 1) != 0 {
						d = -1
					}
					i++
					blockPolicy.Entries[1].MinSize = uint32(int(blockPolicy.Entries[1].MinSize) + d) // flip flop between + and -
					if err := c.MergeDirectoryInfo(log, msgs.ROOT_DIR_INODE_ID, blockPolicy); err != nil {
						terminateChan <- err
						return
					}
					time.Sleep(*dropFetchBlockSocketsEvery)
				}
			}()
		}
	*/

	// start tests
	var kfd *os.File = nil
	var kerr error
	if *kmsg {
		kfd, kerr = os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0o666)
		if kerr != nil {
			panic(fmt.Errorf("failed to open /dev/kmsg: %w", kerr))
		}
		defer kfd.Close()
	}
	go func() {
		r := RunTests{
			overrides:               &overrides,
			registryIp:              "127.0.0.1",
			registryPort:            registryPort,
			mountPoint:              mountPoint,
			fuseMountPoint:          fuseMountPoint,
			kmod:                    *kmod,
			short:                   *short,
			filter:                  filterRe,
			pauseBlockServiceKiller: &pauseBlockServiceKiller,
			kmsgFd:                  kfd,
		}
		r.run(terminateChan, l)
	}()

	// wait for things to finish
	errT := <-terminateChan
	if errT != nil {
		panic(errT)
	}

	// fsck everything
	l.Info("stopping cluster and fscking it")
	procsClosed = true
	procs.Close()
	{
		subDataDirs, err := os.ReadDir(*dataDir)
		if err != nil {
			panic(err)
		}
		for _, subDir := range subDataDirs {
			if !strings.Contains(subDir.Name(), "shard") {
				continue
			}
			l.Info("fscking %q", path.Join(*dataDir, subDir.Name(), "db"))
			cmd := exec.Command(cppExes.DBToolsExe, "fsck", path.Join(*dataDir, subDir.Name(), "db"))
			if err := cmd.Run(); err != nil {
				panic(err)
			}
		}
	}

	// we haven't panicked, allow to cleanup the db dir if appropriate
	cleanupDbDir = tmpDataDir && !*preserveDbDir && !*profile && !*profileTest
}
