// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"bufio"
	"bytes"
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
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run="+run, "-test.count=1", "-test.timeout=20s", "-test.v")
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

// A fake shard and block service, with just enough of each for
// replaceDecommissionedBlocks: FILE_SPANS and REPLACE_BLOCK_* on the shard,
// FETCH_BLOCK_WITH_CRC and WRITE_BLOCK on the block service. The block to
// replace is the last block of location 0, on a decommissioned block service.
type replaceFixture struct {
	t      *testing.T
	log    *log.Logger
	logs   *os.File
	client *client.Client
	fileId msgs.InodeId
	spans  msgs.FileSpansResp
	blocks map[msgs.BlockId][]byte
	target *msgs.FetchedBlock
	// what the replacement block will be
	newBlock msgs.AddSpanInitiateBlockInfo

	mu        sync.Mutex
	fetched   []msgs.BlockId
	written   [][]byte
	certified []msgs.ReplaceBlockCertifyReq
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

func newReplaceFixture(t *testing.T, layouts ...replaceLayout) *replaceFixture {
	f := &replaceFixture{
		t:      t,
		fileId: msgs.MakeInodeId(msgs.FILE, 0, 1),
		blocks: map[msgs.BlockId][]byte{},
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
	blockServiceAddrs := f.startBlockService()

	spanSize := int(layouts[0].parity.DataBlocks()) * int(layouts[0].stripes) * int(layouts[0].cellSize)
	data := make([]byte, spanSize)
	for i := range data {
		data[i] = byte(i*7 + i/4096)
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
			blockService := msgs.BlockService{
				Addrs: blockServiceAddrs,
				Id:    msgs.BlockServiceId(10*(locIx+1) + blockIx),
			}
			for _, ix := range layout.noRead {
				if ix == blockIx {
					blockService.Flags = msgs.TERNFS_BLOCK_SERVICE_NO_READ
				}
			}
			if locIx == 0 && blockIx == layout.parity.Blocks()-1 {
				blockService.Flags = msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED
			}
			blockId := msgs.BlockId(100*(locIx+1) + blockIx)
			f.blocks[blockId] = blockData
			location.Blocks = append(location.Blocks, msgs.FetchedBlock{
				BlockServiceIx: uint8(len(f.spans.BlockServices)),
				BlockId:        blockId,
				Crc:            msgs.Crc(crc32c.Sum(0, blockData)),
			})
			f.spans.BlockServices = append(f.spans.BlockServices, blockService)
		}
		locations.Locations = append(locations.Locations, location)
	}
	f.target = &locations.Locations[0].Blocks[layouts[0].parity.Blocks()-1]
	f.spans.Spans = []msgs.FetchedFullSpan{{
		Header: msgs.FetchedSpanHeaderFull{ByteOffset: 0, Size: uint32(spanSize), Crc: msgs.Crc(crc32c.Sum(0, data))},
		Body:   locations,
	}}
	f.newBlock = msgs.AddSpanInitiateBlockInfo{
		BlockServiceAddrs: blockServiceAddrs,
		BlockServiceId:    999,
		BlockId:           9999,
	}

	c, err := client.NewClientDirectNoAddrs(f.log, msgs.AddrsInfo{})
	if err != nil {
		t.Fatal(err)
	}
	// Not closed: Close waits for an address refresher this client doesn't have.
	// The test runs in its own process instead, see runInChildProcess.
	shards := [256]msgs.AddrsInfo{}
	shards[0] = f.startShard()
	c.SetAddrs(msgs.AddrsInfo{}, &shards)
	f.client = c
	return f
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

func (f *replaceFixture) shardRequest(kind msgs.ShardMessageKind, body []byte) (bincode.Packable, msgs.TernError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch kind {
	case msgs.FILE_SPANS:
		return &f.spans, 0
	case msgs.REPLACE_BLOCK_INITIATE:
		return &msgs.ReplaceBlockInitiateResp{HolderId: msgs.MakeInodeId(msgs.FILE, 0, 2), NewBlock: f.newBlock}, 0
	case msgs.REPLACE_BLOCK_CERTIFY:
		req := msgs.ReplaceBlockCertifyReq{}
		if err := bincode.Unpack(body, &req); err != nil {
			panic(err)
		}
		f.certified = append(f.certified, req)
		return &msgs.ReplaceBlockCertifyResp{Replaced: true}, 0
	}
	f.t.Errorf("unexpected shard request %v", kind)
	return nil, msgs.UNRECOGNIZED_REQUEST
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
		f.written = append(f.written, data)
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

func (f *replaceFixture) replace() (MigrateStats, error) {
	stats := MigrateStats{}
	all := func(*msgs.BlockService, uint32, *msgs.FetchedBlock) (bool, error) { return true, nil }
	err := replaceDecommissionedBlocks(f.log, f.client, bufpool.NewBufPool(), &stats, f.fileId, 0, all)
	return stats, err
}

func (f *replaceFixture) checkFetched(expected ...msgs.BlockId) {
	f.t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if fmt.Sprint(f.fetched) != fmt.Sprint(expected) {
		f.t.Errorf("fetched blocks %v, expected %v", f.fetched, expected)
	}
}

// Checks that the target was replaced exactly once, with its own bytes.
func (f *replaceFixture) checkReplaced(stats MigrateStats, err error) {
	f.t.Helper()
	if err != nil {
		f.t.Fatalf("replace failed: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.written) != 1 || !bytes.Equal(f.written[0], f.blocks[f.target.BlockId]) {
		f.t.Errorf("wrote %v blocks, expected the target's bytes once", len(f.written))
	}
	if len(f.certified) != 1 {
		f.t.Fatalf("certified %v times, expected once", len(f.certified))
	}
	certify := f.certified[0]
	if certify.FileId != f.fileId || certify.ByteOffset != 0 || certify.OldBlockId != f.target.BlockId ||
		certify.NewBlock.BlockId != f.newBlock.BlockId || certify.NewBlock.Proof != [8]byte{1, 2, 3, 4, 5, 6, 7, 8} {
		f.t.Errorf("unexpected certify %+v", certify)
	}
	if stats.MigratedBlocks != 1 || stats.MigratedBytes != uint64(replaceTestBlockSize) || stats.MigratedFiles != 1 {
		f.t.Errorf("stats %+v, expected 1 block of %v bytes in 1 file", stats, replaceTestBlockSize)
	}
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
			f := newReplaceFixture(t, target, tc.incompatible, compatible)
			if tc.incompatible.parity.Blocks() > 2 && bytes.Equal(f.blocks[202][:replaceTestBlockSize], f.blocks[f.target.BlockId]) {
				t.Fatal("the incompatible location's block 2 has the target's bytes")
			}
			f.checkReplaced(f.replace())
			f.checkFetched(302)
		})
	}
	t.Run("rebuild in a compatible location", func(t *testing.T) {
		rebuild := compatible
		rebuild.noRead = []int{2}
		f := newReplaceFixture(t, target, replaceLayout{parity: parity.MkParity(2, 1), stripes: 2, cellSize: replaceTestBlockSize / 2}, rebuild)
		f.checkReplaced(f.replace())
		f.checkFetched(300, 301)
	})
}

func TestReplaceFewerBlocksDoesNotPanic(t *testing.T) {
	if runInChildProcess(t) {
		return
	}
	f := newReplaceFixture(t,
		replaceLayout{parity: parity.MkParity(2, 1), stripes: 1, cellSize: replaceTestBlockSize, noRead: []int{0, 1}},
		replaceLayout{parity: parity.MkParity(1, 1), stripes: 1, cellSize: 2 * replaceTestBlockSize},
	)
	stats, err := f.replace()
	if err != nil {
		t.Fatal(err)
	}
	f.checkFetched()
	if len(f.written) != 0 || len(f.certified) != 0 || stats.MigratedBlocks != 0 {
		t.Errorf("wrote %v blocks and certified %v, expected nothing", len(f.written), len(f.certified))
	}
}
