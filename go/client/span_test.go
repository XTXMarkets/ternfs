package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/parity"
	"xtx/ternfs/core/rs"
	"xtx/ternfs/core/wyhash"
	"xtx/ternfs/msgs"
)

type errorWithStack struct {
	err   error
	stack []byte
	info  string
}

func handleRecover(
	ch chan *errorWithStack,
	err any,
	info string,
) {
	if err != nil {
		ch <- &errorWithStack{
			info:  info,
			err:   err.(error),
			stack: debug.Stack(),
		}
	}
}

type blockFetcher struct {
	errorsChan   chan *errorWithStack
	spanOffset   uint64
	contents     []byte
	dataBlocks   [][]byte
	parityBlocks [][]byte
	bad          uint32
	info         string
}

func (bf *blockFetcher) StartFetchBlock(log *log.Logger, blockService *msgs.BlockService, blockId msgs.BlockId, offset uint32, count uint32, w io.ReaderFrom, extra any, completion chan *blockCompletion) error {
	blockIx := int(blockId)
	if offset%msgs.TERN_PAGE_SIZE != 0 {
		panic(fmt.Errorf("bad offset"))
	}
	if count%msgs.TERN_PAGE_SIZE != 0 {
		panic(fmt.Errorf("bad page size"))
	}
	blockLength := uint32(len(bf.dataBlocks[0]))
	if offset+count > blockLength {
		panic(fmt.Errorf("out of bounds read %v+%v > %v", offset, count, blockLength))
	}
	// TODO randomize completion order
	go func() {
		defer func() {
			handleRecover(bf.errorsChan, recover(), fmt.Sprintf("%s fetch block blockId=%v offset=%v count=%v", bf.info, blockId, offset, count))
		}()
		if bf.bad&(uint32(1)<<blockIx) != 0 {
			completion <- &blockCompletion{
				Extra: extra,
				Error: fmt.Errorf("blockFetcher error"),
			}
			return
		}
		var block []byte
		if blockIx < len(bf.dataBlocks) {
			block = bf.dataBlocks[blockIx]
		} else {
			block = bf.parityBlocks[blockIx-len(bf.dataBlocks)]
		}
		actualOffset := (offset / msgs.TERN_PAGE_SIZE) * msgs.TERN_PAGE_WITH_CRC_SIZE
		actualCount := (count / msgs.TERN_PAGE_SIZE) * msgs.TERN_PAGE_WITH_CRC_SIZE
		if n, err := w.ReadFrom(bytes.NewReader(block[actualOffset : actualOffset+actualCount])); err != nil || n != int64(actualCount) {
			panic(fmt.Errorf("bad read"))
		}
		completion <- &blockCompletion{Extra: extra}
	}()
	return nil
}

func (bf *blockFetcher) ReadCache(offset uint64, dest []byte) (count int) {
	if offset < bf.spanOffset {
		panic(fmt.Errorf("bad offset=%v < bf.spanOffset=%v", offset, bf.spanOffset))
	}
	if offset+uint64(len(dest)) > bf.spanOffset+uint64(len(bf.contents)) {
		panic(fmt.Errorf("bad offset=%v + len(dest)=%v > bf.spanOffset=%v + len(bf.contents)=%v", offset, len(dest), bf.spanOffset, len(bf.contents)))
	}
	return copy(dest, bf.contents[offset-bf.spanOffset:])
}

func (bf *blockFetcher) WriteCache(offset uint64, data []byte) {
	defer func() {
		handleRecover(bf.errorsChan, recover(), fmt.Sprintf("%s write cache offset=%v count=%v", bf.info, offset, len(data)))
	}()
	// Check that we don't write outside file boundaries, and that the
	// contents match.
	if offset < bf.spanOffset {
		panic(fmt.Errorf("offset=%v < bf.spanOffset=%v", offset, bf.spanOffset))
	}
	if offset+uint64(len(data)) > bf.spanOffset+uint64(len(bf.contents)) {
		panic(fmt.Errorf("offset=%v + len(data)=%v > bf.spanOffset=%v + len(bf.contents)=%v", offset, len(data), bf.spanOffset, len(bf.contents)))
	}
	if !bytes.Equal(bf.contents[offset-bf.spanOffset:(offset-bf.spanOffset)+uint64(len(data))], data) {
		panic(fmt.Errorf("mismatching page cache write"))
	}
}

func insertPageCrcs(data []byte) []byte {
	pages := len(data) / int(msgs.TERN_PAGE_SIZE)
	data = append(data, make([]byte, pages*4)...) // add space for CRCs
	for page := pages - 1; page >= 0; page-- {
		pageBytes := data[page*int(msgs.TERN_PAGE_SIZE) : (page+1)*int(msgs.TERN_PAGE_SIZE)]
		pageCrc := crc32c.Sum(0, pageBytes)
		copy(data[page*int(msgs.TERN_PAGE_WITH_CRC_SIZE):(page+1)*int(msgs.TERN_PAGE_WITH_CRC_SIZE)], pageBytes)
		binary.LittleEndian.PutUint32(data[page*int(msgs.TERN_PAGE_WITH_CRC_SIZE)+int(msgs.TERN_PAGE_SIZE):], pageCrc)
	}
	return data
}

const (
	NO_PAGE_CACHE int = iota
	PAGE_CACHE_ALWAYS_SUCCEEDS
	PAGE_CACHE_SOMETIMES_SUCCEEDS
)

func setupSpan(
	terminateChan chan *errorWithStack,
	parity parity.Parity,
	stripes uint8,
	spanOffset uint64,
	contents []byte,
) (*SpanWithBlockServices, *blockFetcher) {
	if parity.DataBlocks() == 0 {
		panic(fmt.Errorf("not enough data blocks"))
	}
	if parity.ParityBlocks() == 0 {
		panic(fmt.Errorf("not enough parity blocks"))
	}
	if stripes == 0 {
		panic(fmt.Errorf("no stripes"))
	}
	rs := rs.Get(parity)
	cellSizef := float64(len(contents)) / float64(parity.DataBlocks()) / float64(stripes)
	cellSize := ((max(1, int(cellSizef)) + int(msgs.TERN_PAGE_SIZE) - 1) / int(msgs.TERN_PAGE_SIZE)) * int(msgs.TERN_PAGE_SIZE)
	stripeSize := cellSize * parity.DataBlocks()
	blockMetadata := make([]msgs.FetchedBlock, parity.Blocks())
	dataBlocks := make([][]byte, parity.DataBlocks())
	for blockIx := range parity.DataBlocks() {
		dataBlocks[blockIx] = make([]byte, cellSize*int(stripes))
	}
	for stripeIx := range int(stripes) {
		for blockIx := range int(parity.DataBlocks()) {
			buf := dataBlocks[msgs.BlockId(blockIx)]
			contentsFrom, contentsTo := stripeIx*stripeSize+blockIx*cellSize, (stripeIx+1)*stripeSize+blockIx*cellSize
			contentsFrom, contentsTo = min(contentsFrom, len(contents)), min(contentsTo, len(contents))
			copy(buf[stripeIx*cellSize:(stripeIx+1)*cellSize], contents[contentsFrom:contentsTo])
		}
	}
	parityBlocks := make([][]byte, parity.ParityBlocks())
	for blockIx := range parity.ParityBlocks() {
		parityBlocks[blockIx] = make([]byte, cellSize*int(stripes))
	}
	rs.ComputeParityInto(dataBlocks, parityBlocks)
	for blockIx := range parity.Blocks() {
		blockMetadata[blockIx] = msgs.FetchedBlock{
			BlockId: msgs.BlockId(blockIx),
		}
		if blockIx < parity.DataBlocks() {
			blockMetadata[blockIx].Crc = msgs.Crc(crc32c.Sum(0, dataBlocks[blockIx]))
			dataBlocks[blockIx] = insertPageCrcs(dataBlocks[blockIx])
		} else {
			blockMetadata[blockIx].Crc = msgs.Crc(crc32c.Sum(0, parityBlocks[blockIx-parity.DataBlocks()]))
			parityBlocks[blockIx-parity.DataBlocks()] = insertPageCrcs(parityBlocks[blockIx-parity.DataBlocks()])
		}
	}
	span := &msgs.FetchedSpan{
		Header: msgs.FetchedSpanHeader{
			Size:       uint32(len(contents)),
			Crc:        msgs.Crc(crc32c.Sum(0, contents)),
			ByteOffset: spanOffset,
		},
		Body: &msgs.FetchedBlocksSpan{
			Parity:   parity,
			Stripes:  stripes,
			CellSize: uint32(cellSize),
			Blocks:   blockMetadata,
		},
	}
	fetcher := &blockFetcher{
		contents:     contents,
		dataBlocks:   dataBlocks,
		parityBlocks: parityBlocks,
		spanOffset:   spanOffset,
		errorsChan:   terminateChan,
		info:         fmt.Sprintf("block fetcher for parity=%v stripes=%v length=%v", parity, stripes, len(contents)),
	}
	return &SpanWithBlockServices{BlockServices: []msgs.BlockService{{}}, Span: span}, fetcher
}

type flakyPageCache struct {
	r wyhash.Rand
	c PageCache
}

func (fpc *flakyPageCache) ReadCache(offset uint64, dest []byte) (count int) {
	x := fpc.r.Float64()
	if x < 0.3 {
		return 0
	}
	if x < 0.6 {
		dest = dest[:fpc.r.Uint64()%uint64(len(dest))]
	}
	return fpc.c.ReadCache(offset, dest)
}

func (fpc *flakyPageCache) WriteCache(offset uint64, data []byte) {
	fpc.c.WriteCache(offset, data)
}

func runRsReadTest(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	errorsChan chan *errorWithStack,
	pageCacheMode int,
	parity parity.Parity,
	stripes uint8,
	length int, // length of content
	iterations int, // iterations of each page aligned, start aligned, end aligned
) {
	var lastReadFrom uint32
	var lastReadTo uint32
	var lastBad uint32
	defer func() {
		handleRecover(errorsChan, recover(), fmt.Sprintf("test runner for parity=%v stripes=%v length=%v lastReadFrom=%v lastReadTo=%v lastBad=%s", parity, stripes, length, lastReadFrom, lastReadTo, printBlockBitmap(parity.Blocks(), lastBad)))
	}()
	seed := uint64(length) | uint64(stripes)<<32 | uint64(parity)<<(32+8)
	r := wyhash.New(seed)
	contents := make([]byte, length)
	r.Read(contents)
	span, fetch := setupSpan(errorsChan, parity, stripes, r.Uint64()%(100<<30), contents)
	var pageCache PageCache
	switch pageCacheMode {
	case NO_PAGE_CACHE:
	case PAGE_CACHE_ALWAYS_SUCCEEDS:
		pageCache = fetch
	case PAGE_CACHE_SOMETIMES_SUCCEEDS:
		pageCache = &flakyPageCache{r: *wyhash.New(seed), c: fetch}
	}
	check := func(from uint32, to uint32) {
		lastReadFrom = from
		lastReadTo = to
		{
			numBad := r.Uint32() % uint32(parity.ParityBlocks())
			fetch.bad = uint32(0)
			for bits.OnesCount32(fetch.bad) < int(numBad) {
				fetch.bad |= uint32(1) << (r.Uint32() % uint32(parity.Blocks()))
			}
		}
		lastBad = fetch.bad
		contentsOut := make([]byte, to-from)
		read, err := readRs(log, fetch, pageCache, bufPool, from, span, contentsOut)
		if err != nil {
			panic(err)
		}
		if read != int(to-from) {
			panic(fmt.Errorf("expected read=%v got read=%v", to-from, read))
		}
		if !bytes.Equal(contents[from:to], contentsOut) {
			panic(fmt.Errorf("mismatching contents"))
		}
	}
	// check the whole thing
	for range iterations { // to get many RS failures
		check(0, uint32(len(contents)))
	}
	// not page aligned
	for range iterations {
		offset := r.Uint32() % uint32(len(contents)-1)
		length := r.Uint32() % uint32(len(contents)-int(offset))
		check(offset, offset+length)
	}
	// page aligned start
	pageStarts := (len(contents) - 1) / int(msgs.TERN_PAGE_SIZE)
	if pageStarts > 0 {
		for range iterations {
			offset := (r.Uint32() % uint32(pageStarts)) * msgs.TERN_PAGE_SIZE
			length := r.Uint32() % uint32(len(contents)-int(offset))
			check(offset, offset+length)
		}
	}
	// page aligned end
	pageEnds := len(contents) / int(msgs.TERN_PAGE_SIZE)
	if pageEnds > 1 {
		for range iterations {
			end := (1 + (r.Uint32() % uint32(pageEnds-1))) * msgs.TERN_PAGE_SIZE
			offset := r.Uint32() % end
			check(offset, end)
		}
	}
}

func runRsSpanTest(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	errorsChan chan *errorWithStack,
	parity parity.Parity,
	stripes uint8,
	length int, // length of content
	iterations int,
) {
	var lastBad uint32
	defer func() {
		handleRecover(errorsChan, recover(), fmt.Sprintf("test runner for parity=%v stripes=%v length=%v lastBad=%s", parity, stripes, length, printBlockBitmap(parity.Blocks(), lastBad)))
	}()
	r := wyhash.New(uint64(length) | uint64(stripes)<<32 | uint64(parity)<<32 + 8)
	contents := make([]byte, length)
	r.Read(contents)
	span, fetch := setupSpan(errorsChan, parity, stripes, 0, contents)
	for range iterations {
		{
			numBad := r.Uint32() % uint32(parity.ParityBlocks())
			fetch.bad = uint32(0)
			for bits.OnesCount32(fetch.bad) < int(numBad) {
				fetch.bad |= uint32(1) << (r.Uint32() % uint32(parity.Blocks()))
			}
		}
		lastBad = fetch.bad
		buf, err := fetchRsSpan(log, fetch, bufPool, span)
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(buf.Bytes(), contents) {
			panic(fmt.Errorf("mismatching contents"))
		}
		bufPool.Put(buf)
	}
}

func runRsTests(
	run func(log *log.Logger, bufPool *bufpool.BufPool, errorsChan chan *errorWithStack, parity parity.Parity, stripes uint8, length int, iterations int),
) {
	log := log.NewLogger(os.Stderr, &log.LoggerOptions{
		Level:            log.DEBUG,
		PrintQuietAlerts: true,
	})
	// Intentionally shared: we want to test whether there are
	// bugs in buf handling. This naturally hurts reproducibility.
	bufPool := bufpool.NewBufPool()
	parities := []parity.Parity{parity.MkParity(2, 1), parity.MkParity(10, 4)}
	stripes := []uint8{1, 2, 15}
	lengths := []int{
		100, 4096,
		614400,     // exactly one page per cell at stripes=15 D=10
		614400 * 2, // two pages per cell
		100 << 20,  // max span size
	}
	{ // some random sizes..
		r := wyhash.New(0)
		for range 10 {
			lengths = append(lengths, int(r.Uint32()%(10<<20)))
		}
	}
	fmt.Fprintf(os.Stderr, "Will run %v tests\n", len(parities)*len(stripes)*len(lengths))
	type test struct {
		parity  parity.Parity
		stripes uint8
		length  int
	}
	testsReq := make(chan test)
	errorsChan := make(chan *errorWithStack)
	terminateOnFirstError := true
	var wg sync.WaitGroup
	numCpus := runtime.NumCPU()
	for range numCpus {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for test := range testsReq {
				run(log, bufPool, errorsChan, test.parity, test.stripes, test.length, 100)
			}
		}()
	}
	go func() {
		for _, parity := range parities {
			for _, stripes := range stripes {
				for _, length := range lengths {
					testsReq <- test{parity: parity, stripes: stripes, length: length}
				}
			}
		}
		close(testsReq)
	}()
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		doneCh <- struct{}{}
	}()
	anyError := false
	select {
	case <-doneCh:
		break
	case err := <-errorsChan:
		fmt.Fprintf(os.Stderr, "Got error: %v\n", err.err)
		fmt.Fprintf(os.Stderr, "Info: %s, backtrace:\n", err.info)
		for _, line := range strings.Split(string(err.stack), "\n") {
			fmt.Fprintln(os.Stderr, line)
		}
		anyError = true
		if terminateOnFirstError {
			panic(fmt.Errorf("got error, exiting"))
		}
	}
	if anyError {
		panic(fmt.Errorf("got some errors"))
	}
}

func TestRsReader(t *testing.T) {
	runRsTests(
		func(log *log.Logger, bufPool *bufpool.BufPool, errorsChan chan *errorWithStack, parity parity.Parity, stripes uint8, length, iterations int) {
			runRsReadTest(log, bufPool, errorsChan, NO_PAGE_CACHE, parity, stripes, length, iterations)
		},
	)
}

func TestRsReaderWithPageCache(t *testing.T) {
	runRsTests(
		func(log *log.Logger, bufPool *bufpool.BufPool, errorsChan chan *errorWithStack, parity parity.Parity, stripes uint8, length, iterations int) {
			runRsReadTest(log, bufPool, errorsChan, PAGE_CACHE_ALWAYS_SUCCEEDS, parity, stripes, length, iterations)
		},
	)
}

func TestRsReaderWithRandomPageCache(t *testing.T) {
	runRsTests(
		func(log *log.Logger, bufPool *bufpool.BufPool, errorsChan chan *errorWithStack, parity parity.Parity, stripes uint8, length, iterations int) {
			runRsReadTest(log, bufPool, errorsChan, PAGE_CACHE_SOMETIMES_SUCCEEDS, parity, stripes, length, iterations)
		},
	)
}

func TestRsSpan(t *testing.T) {
	runRsTests(runRsSpanTest)
}
