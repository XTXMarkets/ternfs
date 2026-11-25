// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"bufio"
	"bytes"
	"container/heap"
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/XTXMarkets/ternfs/go/cleanup/scratch"
	"github.com/XTXMarkets/ternfs/go/client"
	"github.com/XTXMarkets/ternfs/go/core/bincode"
	"github.com/XTXMarkets/ternfs/go/core/bufpool"
	"github.com/XTXMarkets/ternfs/go/core/crc32c"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/parity"
	"github.com/XTXMarkets/ternfs/go/core/rs"
	"github.com/XTXMarkets/ternfs/go/msgs"
)

const replaceTestChildEnv = "TERNFS_REPLACE_TEST_CHILD"

// The fixture's client can't be closed, so, like ternfuse's TestCDCRequestReturnsIO,
// each test runs in a child process and exiting cleans the client up. In the
// parent this runs the test in the child and returns true. In the child it
// returns false and the test goes ahead.
func runInChildProcess(t *testing.T) bool {
	if os.Getenv(replaceTestChildEnv) != "" {
		return false
	}
	run := "^" + t.Name() + "$"
	// keep any subtest selection
	if _, subtests, ok := strings.Cut(flag.Lookup("test.run").Value.String(), "/"); ok {
		run += "/" + subtests
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run="+run, "-test.count=1", "-test.timeout=50s", "-test.v")
	cmd.Env = append(os.Environ(), replaceTestChildEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil || !bytes.Contains(output, []byte("--- PASS: "+t.Name())) {
		t.Fatalf("%v failed in child process: %v\n%s", t.Name(), err, output)
	}
	if testing.Verbose() {
		t.Logf("child process:\n%s", output)
	}
	return true
}

// How one location stores the span, and which of its blocks can't be read.
type replaceLayout struct {
	parity   parity.Parity
	stripes  uint8
	cellSize uint32
	noRead   []int
}

// A fake registry, shard and block service, with just enough of each for
// replaceDecommissionedBlocks. In every span, the block to replace is the last
// block of location 0, on a decommissioned block service.
type replaceFixture struct {
	t                 *testing.T
	log               *log.Logger
	logs              *os.File
	client            *client.Client
	shardAddrs        msgs.AddrsInfo
	blockServiceAddrs msgs.AddrsInfo

	mu            sync.Mutex
	files         map[msgs.InodeId]*msgs.FileSpansResp
	blockServices []msgs.FullBlockServiceInfo
	blocks        map[msgs.BlockId][]byte
	blockService  map[msgs.BlockId]msgs.BlockServiceId
	// what STAT_FILE and STAT_TRANSIENT_FILE return, files are live by default
	statFileErr, statTransientFileErr msgs.TernError
	// what REPLACE_BLOCKS returns
	replaceErr func(req *msgs.ReplaceBlocksReq) msgs.TernError
	// how many REPLACE_BLOCKS replies to drop after handling the request
	dropReplaceReplies int
	scratchFiles       []msgs.InodeId
	scrapped           []msgs.InodeId
	nextBlockId        msgs.BlockId
	fetched            []msgs.BlockId
	// written block bytes by new block id
	written map[msgs.BlockId][]byte
	// the REPLACE_BLOCKS requests as they arrived, and the ones that succeeded
	replaceRequests []msgs.ReplaceBlocksReq
	replaced        []msgs.ReplaceBlocksReq
}

// Splits the span's data into blocks the way the client writes them: each
// stripe is D data cells followed by P parity cells, and block i is cell i of
// every stripe.
func layoutBlocks(data []byte, layout replaceLayout) [][]byte {
	D := layout.parity.DataBlocks()
	blocks := make([][]byte, layout.parity.Blocks())
	cellSize := int(layout.cellSize)
	for s := 0; s < int(layout.stripes); s++ {
		cells := make([][]byte, 0, layout.parity.Blocks())
		for d := 0; d < D; d++ {
			cells = append(cells, data[(s*D+d)*cellSize:(s*D+d+1)*cellSize])
		}
		if D == 1 {
			for p := 0; p < layout.parity.ParityBlocks(); p++ {
				cells = append(cells, cells[0])
			}
		} else {
			cells = append(cells, rs.Get(layout.parity).ComputeParity(cells)...)
		}
		for i, cell := range cells {
			blocks[i] = append(blocks[i], cell...)
		}
	}
	return blocks
}

func newReplaceFixture(t *testing.T) *replaceFixture {
	f := &replaceFixture{
		t:            t,
		files:        map[msgs.InodeId]*msgs.FileSpansResp{},
		blocks:       map[msgs.BlockId][]byte{},
		blockService: map[msgs.BlockId]msgs.BlockServiceId{},
		written:      map[msgs.BlockId][]byte{},
		nextBlockId:  1 << 40,
	}
	logs, err := os.Create(path.Join(t.TempDir(), "log"))
	if err != nil {
		t.Fatal(err)
	}
	f.logs = logs
	f.log = log.NewLogger(logs, &log.LoggerOptions{Level: log.INFO})
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("logs:\n%s", f.logged())
		}
		logs.Close()
	})
	f.blockServiceAddrs = f.startBlockService()
	f.shardAddrs = f.startShard()
	return f
}

// Starts the client, once the fixture has all its block services
func (f *replaceFixture) start() {
	// Not closed: Close waits for an address refresher that only exits on Close.
	// The test runs in its own process instead, see runInChildProcess.
	c, err := client.NewClient(f.log, nil, f.startRegistry(), msgs.AddrsInfo{})
	if err != nil {
		f.t.Fatal(err)
	}
	c.SetFetchBlockServices()
	f.client = c
}

func (f *replaceFixture) addBlockService(flags msgs.BlockServiceFlags) msgs.BlockServiceId {
	id := msgs.BlockServiceId(len(f.blockServices) + 1)
	bs := msgs.FullBlockServiceInfo{Id: id, Addrs: f.blockServiceAddrs, StorageClass: msgs.HDD_STORAGE, Flags: flags}
	binary.LittleEndian.PutUint64(bs.FailureDomain.Name[:], uint64(id))
	f.blockServices = append(f.blockServices, bs)
	return id
}

// Appends a span stored in the given layouts, location i in layouts[i], to the
// file, and returns the block to replace.
func (f *replaceFixture) addSpan(fileId msgs.InodeId, layouts ...replaceLayout) *msgs.FetchedBlock {
	t := f.t
	f.mu.Lock()
	defer f.mu.Unlock()
	spans := f.files[fileId]
	if spans == nil {
		spans = &msgs.FileSpansResp{}
		f.files[fileId] = spans
	}
	spanIx := len(spans.Spans)
	offset := uint64(0)
	for _, span := range spans.Spans {
		offset += uint64(span.Header.Size)
	}
	spanSize := int(layouts[0].parity.DataBlocks()) * int(layouts[0].stripes) * int(layouts[0].cellSize)
	data := make([]byte, spanSize)
	for i := range data {
		data[i] = byte(i*7 + i/4096 + spanIx + int(fileId))
	}
	locations := &msgs.FetchedLocations{}
	for locIx, layout := range layouts {
		if size := layout.parity.DataBlocks() * int(layout.stripes) * int(layout.cellSize); size != spanSize {
			t.Fatalf("location %v holds %v bytes, location 0 holds %v", locIx, size, spanSize)
		}
		location := msgs.FetchedBlockServices{
			LocationId:   msgs.Location(locIx),
			StorageClass: msgs.HDD_STORAGE,
			Parity:       layout.parity,
			Stripes:      layout.stripes,
			CellSize:     layout.cellSize,
		}
		for blockIx, blockData := range layoutBlocks(data, layout) {
			flags := msgs.BlockServiceFlags(0)
			for _, ix := range layout.noRead {
				if ix == blockIx {
					flags = msgs.TERNFS_BLOCK_SERVICE_NO_READ
				}
			}
			if locIx == 0 && blockIx == layout.parity.Blocks()-1 {
				flags = msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED
			}
			id := f.addBlockService(flags)
			blockId := msgs.BlockId(10000*int(fileId) + 1000*spanIx + 100*(locIx+1) + blockIx)
			f.blocks[blockId] = blockData
			f.blockService[blockId] = id
			location.Blocks = append(location.Blocks, msgs.FetchedBlock{
				BlockServiceIx: uint8(len(spans.BlockServices)),
				BlockId:        blockId,
				Crc:            msgs.Crc(crc32c.Sum(0, blockData)),
			})
			spans.BlockServices = append(spans.BlockServices, msgs.BlockService{Addrs: f.blockServiceAddrs, Id: id, Flags: flags})
		}
		locations.Locations = append(locations.Locations, location)
	}
	spans.Spans = append(spans.Spans, msgs.FetchedFullSpan{
		Header: msgs.FetchedSpanHeaderFull{ByteOffset: offset, Size: uint32(spanSize), Crc: msgs.Crc(crc32c.Sum(0, data))},
		Body:   locations,
	})
	return &locations.Locations[0].Blocks[layouts[0].parity.Blocks()-1]
}

// One span, in one location, whose last block is to be replaced
func (f *replaceFixture) addFile(fileId msgs.InodeId) *msgs.FetchedBlock {
	return f.addSpan(fileId, replaceLayout{parity: parity.MkParity(2, 1), stripes: 1, cellSize: replaceTestBlockSize})
}

func addrsInfo(addr net.Addr) msgs.AddrsInfo {
	var ip [4]byte
	var port int
	switch a := addr.(type) {
	case *net.UDPAddr:
		copy(ip[:], a.IP.To4())
		port = a.Port
	case *net.TCPAddr:
		copy(ip[:], a.IP.To4())
		port = a.Port
	}
	return msgs.AddrsInfo{Addr1: msgs.IpPort{Addrs: ip, Port: uint16(port)}}
}

func (f *replaceFixture) startRegistry() string {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				for f.registryRequest(r, conn) == nil {
				}
			}()
		}
	}()
	return listener.Addr().String()
}

func (f *replaceFixture) registryRequest(r io.Reader, w io.Writer) error {
	var header [4 + 4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	body := make([]byte, binary.LittleEndian.Uint32(header[4:]))
	if _, err := io.ReadFull(r, body); err != nil {
		return err
	}
	var resp msgs.RegistryResponse
	switch kind := msgs.RegistryMessageKind(body[0]); kind {
	case msgs.LOCAL_SHARDS:
		shards := &msgs.LocalShardsResp{Shards: make([]msgs.ShardInfo, 256)}
		for i := range shards.Shards {
			shards.Shards[i].Addrs = f.shardAddrs
		}
		resp = shards
	case msgs.LOCAL_CDC:
		resp = &msgs.LocalCdcResp{Addrs: f.shardAddrs}
	case msgs.CHANGED_BLOCK_SERVICES:
		f.mu.Lock()
		resp = &msgs.ChangedBlockServicesResp{LastChange: 1, BlockServices: append([]msgs.FullBlockServiceInfo{}, f.blockServices...)}
		f.mu.Unlock()
	default:
		return fmt.Errorf("unexpected registry request %v", kind)
	}
	packed, err := bincode.Pack(resp)
	if err != nil {
		return err
	}
	packet := binary.LittleEndian.AppendUint32(nil, msgs.REGISTRY_RESP_PROTOCOL_VERSION)
	packet = binary.LittleEndian.AppendUint32(packet, uint32(1+len(packed)))
	packet = append(packet, uint8(resp.RegistryResponseKind()))
	_, err = w.Write(append(packet, packed...))
	return err
}

func (f *replaceFixture) startShard() msgs.AddrsInfo {
	sock, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { sock.Close() })
	go func() {
		buf := make([]byte, 1<<16)
		for {
			n, from, err := sock.ReadFromUDP(buf)
			if err != nil {
				return
			}
			kind := msgs.ShardMessageKind(buf[12])
			resp, terr := f.shardRequest(kind, buf[13:n])
			if f.dropReply(kind) {
				continue
			}
			packet := binary.LittleEndian.AppendUint32(nil, msgs.SHARD_RESP_PROTOCOL_VERSION)
			packet = append(packet, buf[4:13]...)
			if terr != 0 {
				packet[12] = msgs.ERROR
				packet = binary.LittleEndian.AppendUint16(packet, uint16(terr))
			} else {
				body, err := bincode.Pack(resp)
				if err != nil {
					panic(err)
				}
				packet = append(packet, body...)
			}
			sock.WriteToUDP(packet, from)
		}
	}()
	return addrsInfo(sock.LocalAddr())
}

func unpack[T any, P interface {
	*T
	bincode.Unpackable
}](body []byte) *T {
	req := P(new(T))
	if err := bincode.Unpack(body, req); err != nil {
		panic(err)
	}
	return req
}

func (f *replaceFixture) shardRequest(kind msgs.ShardMessageKind, body []byte) (bincode.Packable, msgs.TernError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch kind {
	case msgs.FILE_SPANS:
		req := unpack[msgs.FileSpansReq](body)
		spans, ok := f.files[req.FileId]
		if !ok {
			return nil, msgs.FILE_NOT_FOUND
		}
		return spans, 0
	case msgs.STAT_FILE:
		return &msgs.StatFileResp{}, f.statFileErr
	case msgs.STAT_TRANSIENT_FILE:
		return &msgs.StatTransientFileResp{}, f.statTransientFileErr
	case msgs.CONSTRUCT_FILE:
		id := msgs.MakeInodeId(msgs.FILE, 0, uint64(1000+len(f.scratchFiles)))
		f.scratchFiles = append(f.scratchFiles, id)
		return &msgs.ConstructFileResp{Id: id}, 0
	case msgs.ADD_SPAN_AT_LOCATION_INITIATE:
		f.nextBlockId++
		return &msgs.AddSpanAtLocationInitiateResp{Resp: msgs.AddSpanInitiateResp{Blocks: []msgs.AddSpanInitiateBlockInfo{{
			BlockServiceAddrs: f.blockServiceAddrs,
			BlockServiceId:    f.blockServices[len(f.blockServices)-1].Id,
			BlockId:           f.nextBlockId,
		}}}}, 0
	case msgs.ADD_SPAN_CERTIFY:
		return &msgs.AddSpanCertifyResp{}, 0
	case msgs.SCRAP_TRANSIENT_FILE:
		f.scrapped = append(f.scrapped, unpack[msgs.ScrapTransientFileReq](body).Id)
		return &msgs.ScrapTransientFileResp{}, 0
	case msgs.REPLACE_BLOCKS:
		req := unpack[msgs.ReplaceBlocksReq](body)
		f.replaceRequests = append(f.replaceRequests, *req)
		if f.replaceErr != nil {
			if err := f.replaceErr(req); err != 0 {
				return nil, err
			}
		}
		f.replaced = append(f.replaced, *req)
		return &msgs.ReplaceBlocksResp{}, 0
	}
	f.t.Errorf("unexpected shard request %v", kind)
	return nil, msgs.UNRECOGNIZED_REQUEST
}

func (f *replaceFixture) dropReply(kind msgs.ShardMessageKind) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if kind == msgs.REPLACE_BLOCKS && f.dropReplaceReplies > 0 {
		f.dropReplaceReplies--
		return true
	}
	return false
}

func (f *replaceFixture) startBlockService() msgs.AddrsInfo {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				r := bufio.NewReader(conn)
				for f.blocksRequest(r, conn) == nil {
				}
			}()
		}
	}()
	return addrsInfo(listener.Addr())
}

func blocksResponse(w io.Writer, kind msgs.BlocksMessageKind, resp bincode.Packable, terr msgs.TernError) error {
	packet := binary.LittleEndian.AppendUint32(nil, msgs.BLOCKS_RESP_PROTOCOL_VERSION)
	if terr != 0 {
		packet = append(packet, msgs.ERROR)
		packet = binary.LittleEndian.AppendUint16(packet, uint16(terr))
	} else {
		packet = append(packet, uint8(kind))
		body, err := bincode.Pack(resp)
		if err != nil {
			return err
		}
		packet = append(packet, body...)
	}
	_, err := w.Write(packet)
	return err
}

func (f *replaceFixture) blocksRequest(r *bufio.Reader, w io.Writer) error {
	var header [4 + 8 + 1]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return err
	}
	switch kind := msgs.BlocksMessageKind(header[12]); kind {
	case msgs.FETCH_BLOCK_WITH_CRC:
		req := msgs.FetchBlockWithCrcReq{}
		if err := req.Unpack(r); err != nil {
			return err
		}
		f.mu.Lock()
		data, ok := f.blocks[req.BlockId]
		f.fetched = append(f.fetched, req.BlockId)
		f.mu.Unlock()
		if !ok || req.Offset+req.Count > uint32(len(data)) {
			return blocksResponse(w, kind, nil, msgs.BLOCK_NOT_FOUND)
		}
		if err := blocksResponse(w, kind, &msgs.FetchBlockWithCrcResp{}, 0); err != nil {
			return err
		}
		for page := req.Offset; page < req.Offset+req.Count; page += msgs.TERN_PAGE_SIZE {
			pageData := data[page : page+msgs.TERN_PAGE_SIZE]
			if _, err := w.Write(binary.LittleEndian.AppendUint32(append([]byte{}, pageData...), crc32c.Sum(0, pageData))); err != nil {
				return err
			}
		}
		return nil
	case msgs.WRITE_BLOCK:
		req := msgs.WriteBlockReq{}
		if err := req.Unpack(r); err != nil {
			return err
		}
		data := make([]byte, req.Size)
		if _, err := io.ReadFull(r, data); err != nil {
			return err
		}
		if msgs.Crc(crc32c.Sum(0, data)) != req.Crc {
			return blocksResponse(w, kind, nil, msgs.BAD_BLOCK_CRC)
		}
		f.mu.Lock()
		f.written[req.BlockId] = data
		f.mu.Unlock()
		return blocksResponse(w, kind, &msgs.WriteBlockResp{Proof: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}}, 0)
	default:
		return fmt.Errorf("unexpected blocks request %v", kind)
	}
}

func (f *replaceFixture) logged() string {
	logged, err := os.ReadFile(f.logs.Name())
	if err != nil {
		f.t.Fatal(err)
	}
	return string(logged)
}

func (f *replaceFixture) replace(scratchFile scratch.ScratchFile, files ...msgs.InodeId) MigrateStats {
	f.t.Helper()
	stats := MigrateStats{}
	all := func(*msgs.BlockService, uint32, *msgs.FetchedBlock) (bool, error) { return true, nil }
	for i, err := range replaceDecommissionedBlocks(f.log, f.client, bufpool.NewBufPool(), &stats, scratchFile, files, 0, all) {
		if err != nil {
			f.t.Fatalf("replacing blocks in %v failed: %v", files[i], err)
		}
	}
	return stats
}

func (f *replaceFixture) replaceOne(file msgs.InodeId) MigrateStats {
	scratchFile := scratch.NewScratchFile(f.log, f.client, 0, "test")
	defer scratchFile.Close()
	return f.replace(scratchFile, file)
}

// The check functions read what the fakes recorded, so the caller must hold f.mu.
func (f *replaceFixture) checkFetched(expected ...msgs.BlockId) {
	f.t.Helper()
	if fmt.Sprint(f.fetched) != fmt.Sprint(expected) {
		f.t.Errorf("fetched blocks %v, expected %v", f.fetched, expected)
	}
}

func (f *replaceFixture) checkStats(stats MigrateStats, blocks, files uint64) {
	f.t.Helper()
	if stats.MigratedBlocks != blocks || stats.MigratedBytes != blocks*uint64(replaceTestBlockSize) || stats.MigratedFiles != files {
		f.t.Errorf("stats %+v, expected %v blocks of %v bytes in %v files", stats, blocks, replaceTestBlockSize, files)
	}
}

// Checks that the requests replaced exactly these blocks, each with its own
// bytes, and returns the items.
func (f *replaceFixture) checkReplaced(requests []msgs.ReplaceBlocksReq, targets ...*msgs.FetchedBlock) []msgs.ReplaceBlock {
	f.t.Helper()
	items := []msgs.ReplaceBlock{}
	for _, req := range requests {
		items = append(items, req.Blocks...)
	}
	if len(items) != len(targets) {
		f.t.Fatalf("replaced %+v, expected %v blocks", items, len(targets))
	}
	for i, item := range items {
		if item.OldBlockId != targets[i].BlockId || item.FileId != msgs.InodeId(item.OldBlockId/10000) {
			f.t.Errorf("replaced %+v, expected block %v", item, targets[i].BlockId)
		}
		if item.OldBlockServiceId != f.blockService[item.OldBlockId] {
			f.t.Errorf("replaced %+v, expected its block service to be %v", item, f.blockService[item.OldBlockId])
		}
		if !bytes.Equal(f.written[item.NewBlockId], f.blocks[item.OldBlockId]) {
			f.t.Errorf("block %v replacing %v doesn't have its bytes", item.NewBlockId, item.OldBlockId)
		}
	}
	return items
}

const replaceTestBlockSize = 2 * msgs.TERN_PAGE_SIZE

// The target's own location can't rebuild it, so it has to come from another
// location. Block 2 of another location only has the same bytes if that
// location splits the span the same way.
func TestReplaceSkipsIncompatibleLocations(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	target := replaceLayout{parity: parity.MkParity(2, 1), stripes: 1, cellSize: replaceTestBlockSize, noRead: []int{0, 1}}
	compatible := replaceLayout{parity: parity.MkParity(2, 1), stripes: 1, cellSize: replaceTestBlockSize}
	file := msgs.InodeId(1)
	// Every location holds the same 16KiB span. With the same parity and block
	// size, D*cellSize*stripes fixes the stripes too, so a different parity with
	// the same block count needs bigger blocks.
	for _, tc := range []struct {
		name         string
		incompatible replaceLayout
	}{
		{"same parity, different stripes", replaceLayout{parity: parity.MkParity(2, 1), stripes: 2, cellSize: replaceTestBlockSize / 2}},
		{"different parity, same block count", replaceLayout{parity: parity.MkParity(1, 2), stripes: 1, cellSize: 2 * replaceTestBlockSize}},
		{"fewer blocks", replaceLayout{parity: parity.MkParity(1, 1), stripes: 1, cellSize: 2 * replaceTestBlockSize}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReplaceFixture(t)
			replaced := f.addSpan(file, target, tc.incompatible, compatible)
			f.addBlockService(0)
			f.start()
			if tc.incompatible.parity.Blocks() > 2 && bytes.Equal(f.blocks[10202][:replaceTestBlockSize], f.blocks[replaced.BlockId]) {
				t.Fatal("the incompatible location's block 2 has the target's bytes")
			}
			f.checkStats(f.replaceOne(file), 1, 1)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.checkReplaced(f.replaced, replaced)
			f.checkFetched(10302)
		})
	}
	t.Run("rebuild in a compatible location", func(t *testing.T) {
		rebuild := compatible
		rebuild.noRead = []int{2}
		f := newReplaceFixture(t)
		replaced := f.addSpan(file, target, replaceLayout{parity: parity.MkParity(2, 1), stripes: 2, cellSize: replaceTestBlockSize / 2}, rebuild)
		f.addBlockService(0)
		f.start()
		f.checkStats(f.replaceOne(file), 1, 1)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.checkReplaced(f.replaced, replaced)
		f.checkFetched(10300, 10301)
	})
}

func TestReplaceFewerBlocksDoesNotPanic(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	f := newReplaceFixture(t)
	f.addSpan(1,
		replaceLayout{parity: parity.MkParity(2, 1), stripes: 1, cellSize: replaceTestBlockSize, noRead: []int{0, 1}},
		replaceLayout{parity: parity.MkParity(1, 1), stripes: 1, cellSize: 2 * replaceTestBlockSize},
	)
	f.start()
	f.checkStats(f.replaceOne(1), 0, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkFetched()
	if len(f.written) != 0 || len(f.replaceRequests) != 0 {
		t.Errorf("wrote %v blocks and sent %v requests, expected nothing", len(f.written), len(f.replaceRequests))
	}
}

// When no location can produce a block of a live file, its data is at risk, so
// raise an alert. A deleted file, or one that's gone by the time we look, is
// GC's business. Either way the rest of the file is still migrated.
func TestReplaceAlertsOnlyForLiveFiles(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	for _, tc := range []struct {
		name                              string
		statFileErr, statTransientFileErr msgs.TernError
		alert                             bool
	}{
		{"live", 0, 0, true},
		{"transient", msgs.FILE_NOT_FOUND, 0, false},
		{"gone", msgs.FILE_NOT_FOUND, msgs.FILE_NOT_FOUND, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReplaceFixture(t)
			// nothing but the block to replace can be read in the first span
			lost := f.addSpan(1, replaceLayout{parity: parity.MkParity(2, 1), stripes: 1, cellSize: replaceTestBlockSize, noRead: []int{0, 1}})
			// the second can be rebuilt in place
			next := f.addFile(1)
			f.addBlockService(0)
			f.start()
			f.mu.Lock()
			f.statFileErr, f.statTransientFileErr = tc.statFileErr, tc.statTransientFileErr
			f.mu.Unlock()
			f.checkStats(f.replaceOne(1), 1, 1)
			alert := fmt.Sprintf("could not recover block %v in file %v from any location", lost.BlockId, msgs.InodeId(1))
			alerted := false
			for _, line := range strings.Split(f.logged(), "\n") {
				alerted = alerted || (strings.Contains(line, "[ERROR]") && strings.Contains(line, alert))
			}
			if alerted != tc.alert {
				t.Errorf("alerted %v, expected %v", alerted, tc.alert)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if items := f.checkReplaced(f.replaced, next); items[0].ByteOffset != 2*uint64(replaceTestBlockSize) {
				t.Errorf("replaced %+v, expected the second span", items[0])
			}
		})
	}
}

// Files rarely have more than one block on a block service, so a request
// carries blocks from many files, and the scratch file is reused once it's
// empty.
func TestReplaceBatchesAcrossFiles(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	f := newReplaceFixture(t)
	files := []msgs.InodeId{}
	targets := []*msgs.FetchedBlock{}
	for i := range msgs.MAX_REPLACE_BLOCKS + 3 {
		files = append(files, msgs.InodeId(i+1))
		targets = append(targets, f.addFile(files[i]))
	}
	f.addBlockService(0)
	f.start()
	scratchFile := scratch.NewScratchFile(f.log, f.client, 0, "test")
	defer scratchFile.Close()
	f.checkStats(f.replace(scratchFile, files[:len(files)-1]...), uint64(len(files)-1), uint64(len(files)-1))
	f.checkStats(f.replace(scratchFile, files[len(files)-1]), 1, 1)
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.replaced) != 3 || len(f.replaced[0].Blocks) != msgs.MAX_REPLACE_BLOCKS || len(f.replaced[1].Blocks) != 2 || len(f.replaced[2].Blocks) != 1 {
		t.Fatalf("sent %v requests, expected %v, 2 and 1 blocks", len(f.replaced), msgs.MAX_REPLACE_BLOCKS)
	}
	f.checkReplaced(f.replaced, targets...)
	for _, req := range f.replaced {
		if req.ScratchId != f.scratchFiles[0] {
			t.Errorf("request used scratch file %v, expected %v", req.ScratchId, f.scratchFiles[0])
		}
		for i, item := range req.Blocks {
			if item.ScratchByteOffset != uint64(i)*uint64(replaceTestBlockSize) {
				t.Errorf("item %v is at %v in the scratch file, expected %v", i, item.ScratchByteOffset, uint64(i)*uint64(replaceTestBlockSize))
			}
		}
	}
	if len(f.scratchFiles) != 1 || len(f.scrapped) != 0 {
		t.Errorf("created scratch files %v and scrapped %v, expected one and none", f.scratchFiles, f.scrapped)
	}
}

// One block the shard won't take fails its whole batch, and the scratch file
// is scrapped. Each file in the batch is then rebuilt and sent again on its
// own, so only the file with that block is left behind.
func TestReplaceRetriesFilesOnTheirOwn(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	f := newReplaceFixture(t)
	files := []msgs.InodeId{1, 2, 3}
	targets := []*msgs.FetchedBlock{}
	for _, file := range files {
		targets = append(targets, f.addFile(file))
	}
	next := f.addFile(4)
	f.addBlockService(0)
	f.start()
	f.mu.Lock()
	f.replaceErr = func(req *msgs.ReplaceBlocksReq) msgs.TernError {
		for _, item := range req.Blocks {
			if item.FileId == 2 {
				return msgs.BLOCK_NOT_FOUND
			}
		}
		return 0
	}
	f.mu.Unlock()
	scratchFile := scratch.NewScratchFile(f.log, f.client, 0, "test")
	defer scratchFile.Close()
	f.checkStats(f.replace(scratchFile, files...), 2, 3)
	f.checkStats(f.replace(scratchFile, 4), 1, 1)
	f.mu.Lock()
	defer f.mu.Unlock()

	sent := []string{}
	for _, req := range f.replaceRequests {
		ids := []msgs.InodeId{}
		for _, item := range req.Blocks {
			ids = append(ids, item.FileId)
		}
		sent = append(sent, fmt.Sprint(ids))
		if req.Blocks[0].ScratchByteOffset != 0 {
			t.Errorf("request for %v starts at %v in the scratch file, expected 0", ids, req.Blocks[0].ScratchByteOffset)
		}
	}
	expected := []string{fmt.Sprint(files), fmt.Sprint(files[:1]), fmt.Sprint(files[1:2]), fmt.Sprint(files[2:]), fmt.Sprint([]msgs.InodeId{4})}
	if fmt.Sprint(sent) != fmt.Sprint(expected) {
		t.Fatalf("sent requests for %v, expected %v", sent, expected)
	}
	f.checkReplaced(f.replaced, targets[0], targets[2], next)
	// the batch's scratch file, and the one file 2 failed in on its own
	if len(f.scratchFiles) != 3 || fmt.Sprint(f.scrapped) != fmt.Sprint(f.scratchFiles[:2]) {
		t.Errorf("scrapped %v of %v, expected the first two of three", f.scrapped, f.scratchFiles)
	}
}

// The shard applies the request but its reply is lost, so the client sends the
// same request again. Repeating it is a no-op on the shard (its own tests check
// that), and the blocks are counted once.
func TestReplaceCountsOnceWhenReplyDropped(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	f := newReplaceFixture(t)
	target := f.addFile(1)
	f.addBlockService(0)
	f.start()
	f.mu.Lock()
	f.dropReplaceReplies = 1
	f.mu.Unlock()
	f.checkStats(f.replaceOne(1), 1, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.replaceRequests) != 2 || fmt.Sprint(f.replaceRequests[0]) != fmt.Sprint(f.replaceRequests[1]) {
		t.Errorf("sent %+v, expected the same request twice", f.replaceRequests)
	}
	f.checkReplaced(f.replaced[:1], target)
}

// The aggregator hands out files in one shard together, so that they can share
// requests, and the queue stays in order for the files left behind.
func TestReplaceGroupsQueuedFilesByShard(t *testing.T) {
	queue := filePQ{inodeToIdx: make(map[msgs.InodeId]int)}
	heap.Init(&queue)
	for i := uint64(1); i <= 10; i++ {
		heap.Push(&queue, fileInfo{msgs.MakeInodeId(msgs.FILE, msgs.ShardId(i%2), i), int(i % 3)})
	}
	first := heap.Pop(&queue).(fileInfo).id
	group := queue.popShard([]msgs.InodeId{first}, 4)
	if len(group) != 4 {
		t.Fatalf("grouped %v, expected 4 files", group)
	}
	for _, file := range group {
		if file.Shard() != first.Shard() {
			t.Errorf("grouped %v with %v, which is in another shard", file, first)
		}
	}
	left := []fileInfo{}
	for queue.Len() > 0 {
		left = append(left, heap.Pop(&queue).(fileInfo))
	}
	if len(left) != 6 || len(queue.inodeToIdx) != 0 || len(queue.shardFiles[0])+len(queue.shardFiles[1]) != 0 {
		t.Fatalf("left %v in the queue, expected 6 files and empty indexes", left)
	}
	for i := 1; i < len(left); i++ {
		if left[i].errorCount > left[i-1].errorCount {
			t.Errorf("popped %v after %v", left[i], left[i-1])
		}
	}
}
