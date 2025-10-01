// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/crc32c"
	"xtx/ternfs/core/log"
	"xtx/ternfs/core/parity"
	"xtx/ternfs/core/rs"
	"xtx/ternfs/core/timing"
	"xtx/ternfs/msgs"
)

type blockReader struct {
	cells    int64
	cellSize int64
	stride   int64
	data     []byte
	cursor   int64
}

func (r *blockReader) Read(p []byte) (int, error) {
	if r.cursor < 0 || r.cursor >= r.cells*r.cellSize {
		return 0, io.EOF
	}
	cell := r.cursor / r.cellSize
	cellCursor := r.cursor % r.cellSize
	read := copy(p, r.data[cell*r.stride+cellCursor:cell*r.stride+r.cellSize])
	r.cursor += int64(read)
	return read, nil
}

func (r *blockReader) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = int64(r.cursor) + offset
	case io.SeekEnd:
		abs = int64(r.cells*r.cellSize) + offset
	default:
		return 0, errors.New("blockReader.Seek: invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("blockReader.Seek: negative position")
	}
	r.cursor = abs
	return abs, nil
}

func (c *Client) createInlineSpan(
	log *log.Logger,
	id msgs.InodeId,
	cookie [8]byte,
	offset uint64,
	sizeWithZeros uint32,
	data []byte,
) error {
	if int(sizeWithZeros) < len(data) {
		panic(fmt.Errorf("sizeWithZeros=%v < len(data)=%v", sizeWithZeros, len(data)))
	}
	crc := crc32c.Sum(0, data)
	crc = crc32c.ZeroExtend(crc, int(sizeWithZeros)-len(data))
	req := msgs.AddInlineSpanReq{
		FileId:       id,
		Cookie:       cookie,
		StorageClass: msgs.INLINE_STORAGE,
		ByteOffset:   offset,
		Size:         sizeWithZeros,
		Body:         data,
		Crc:          msgs.Crc(crc),
	}
	if err := c.ShardRequest(log, id.Shard(), &req, &msgs.AddInlineSpanResp{}); err != nil {
		return err
	}
	return nil
}

func ensureLen(buf *[]byte, l int) {
	lenBefore := len(*buf)
	if l <= cap(*buf) {
		*buf = (*buf)[:l]
	} else {
		*buf = (*buf)[:cap(*buf)]
		*buf = append(*buf, make([]byte, l-len(*buf))...)
	}
	clear((*buf)[lenBefore:])
}

type SpanParameters struct {
	Parity       parity.Parity
	StorageClass msgs.StorageClass
	Stripes      uint8
	CellSize     uint32
}

func ComputeSpanParameters(
	spanPolicies *msgs.SpanPolicy,
	blockPolicies *msgs.BlockPolicy,
	stripePolicy *msgs.StripePolicy,
	spanSize uint32,
) *SpanParameters {
	// Compute all the size parameters.
	S := int(stripePolicy.Stripes(spanSize))
	spanPolicy := spanPolicies.Pick(spanSize)
	D := spanPolicy.Parity.DataBlocks()
	blockSize := (int(spanSize) + D - 1) / D
	cellSize := (blockSize + S - 1) / S
	// Round up cell to page size
	cellSize = int(msgs.TERN_PAGE_SIZE) * ((cellSize + int(msgs.TERN_PAGE_SIZE) - 1) / int(msgs.TERN_PAGE_SIZE))
	blockSize = cellSize * S
	storageClass := blockPolicies.Pick(uint32(blockSize)).StorageClass
	return &SpanParameters{
		Parity:       spanPolicy.Parity,
		Stripes:      uint8(S),
		StorageClass: storageClass,
		CellSize:     uint32(cellSize),
	}
}

func prepareSpanInitiateReq(
	blacklist []msgs.BlacklistEntry,
	spanPolicies *msgs.SpanPolicy,
	blockPolicies *msgs.BlockPolicy,
	stripePolicy *msgs.StripePolicy,
	id msgs.InodeId,
	cookie [8]byte,
	offset uint64,
	sizeWithZeros uint32,
	data *[]byte,
) *msgs.AddSpanInitiateReq {
	if int(sizeWithZeros) < len(*data) {
		panic(fmt.Errorf("sizeWithZeros=%v < len(data)=%v", sizeWithZeros, len(*data)))
	}

	crc := crc32c.Sum(0, *data)
	crc = crc32c.ZeroExtend(crc, int(sizeWithZeros)-len(*data))

	spanParameters := ComputeSpanParameters(spanPolicies, blockPolicies, stripePolicy, uint32(len(*data)))
	S := int(spanParameters.Stripes)
	D := spanParameters.Parity.DataBlocks()
	P := spanParameters.Parity.ParityBlocks()
	B := spanParameters.Parity.Blocks()
	cellSize := int(spanParameters.CellSize)
	blockSize := cellSize * S

	// Pad the data with zeros
	ensureLen(data, S*D*cellSize)

	initiateReq := msgs.AddSpanInitiateReq{
		FileId:       id,
		Cookie:       cookie,
		ByteOffset:   offset,
		Size:         sizeWithZeros,
		Crc:          msgs.Crc(crc),
		StorageClass: spanParameters.StorageClass,
		Blacklist:    blacklist,
		Parity:       spanParameters.Parity,
		Stripes:      uint8(S),
		CellSize:     uint32(cellSize),
		Crcs:         make([]msgs.Crc, B*S),
	}

	if D == 1 { // mirroring
		for s := 0; s < S; s++ {
			crc := msgs.Crc(crc32c.Sum(0, (*data)[s*cellSize:(s+1)*cellSize]))
			for b := 0; b < B; b++ {
				initiateReq.Crcs[B*s+b] = crc
			}
		}
	} else { // RS
		// Make space for the parity blocks after the data blocks
		ensureLen(data, blockSize*B)
		rs := rs.Get(spanParameters.Parity)
		dataSrcs := make([][]byte, D)
		parityDests := make([][]byte, P)
		for s := 0; s < S; s++ {
			// Compute CRCs for data blocks, and store their offsets
			for d := 0; d < D; d++ {
				dataStart := D*cellSize*s + cellSize*d
				dataEnd := D*cellSize*s + cellSize*(d+1)
				dataSrcs[d] = (*data)[dataStart:dataEnd]
				initiateReq.Crcs[B*s+d] = msgs.Crc(crc32c.Sum(0, dataSrcs[d]))
			}
			// Generate parity
			for p := 0; p < P; p++ {
				dataStart := S*D*cellSize + P*cellSize*s + cellSize*p
				dataEnd := S*D*cellSize + P*cellSize*s + cellSize*(p+1)
				parityDests[p] = (*data)[dataStart:dataEnd]
			}
			rs.ComputeParityInto(dataSrcs, parityDests)
			// Compute parity CRC
			for p := 0; p < P; p++ {
				initiateReq.Crcs[B*s+(D+p)] = msgs.Crc(crc32c.Sum(0, parityDests[p]))
			}
		}
	}

	return &initiateReq
}

func mkBlockReader(
	req *msgs.AddSpanInitiateReq,
	data []byte,
	block int,
) (msgs.Crc, io.ReadSeeker) {
	D := req.Parity.DataBlocks()
	P := req.Parity.ParityBlocks()
	B := req.Parity.Blocks()
	S := int(req.Stripes)
	cellSize := int(req.CellSize)
	blockCrc := uint32(0)
	for s := 0; s < int(req.Stripes); s++ {
		blockCrc = crc32c.Append(blockCrc, uint32(req.Crcs[B*s+block]), cellSize)
	}
	if D == 1 {
		// mirroring, we only have one block
		return msgs.Crc(blockCrc), bytes.NewReader(data)
	} else if block < D {
		// data block, first section of the blob
		r := &blockReader{
			cells:    int64(S),
			cellSize: int64(cellSize),
			stride:   int64(cellSize * D),
			data:     data[block*cellSize:],
			cursor:   0,
		}
		return msgs.Crc(blockCrc), r
	} else {
		// parity block, second section of the blob
		r := &blockReader{
			cells:    int64(S),
			cellSize: int64(cellSize),
			stride:   int64(cellSize * P),
			data:     data[D*S*cellSize+(block-D)*cellSize:],
			cursor:   0,
		}
		return msgs.Crc(blockCrc), r
	}
}

// Appends a new span to a given file.
// Note: the buffer underlying data might be modified by adding padding zeros
// for the purpose of splitting things into blocks/stripes. The (possibly modified)
// buffer is returned, regardless of whether the error is nil or not.
//
// Return which block ids were created for the span, this is needed in defragmentation
// so we return it immediately here
func (c *Client) CreateSpan(
	log *log.Logger,
	blacklist []msgs.BlacklistEntry,
	spanPolicies *msgs.SpanPolicy,
	blockPolicies *msgs.BlockPolicy,
	stripePolicy *msgs.StripePolicy,
	id msgs.InodeId,
	reference msgs.InodeId,
	cookie [8]byte,
	offset uint64,
	// The span size might be greater than `len(*data)`, in which case we have trailing
	// zeros (this allows us to cheaply stored zero sections).
	spanSize uint32,
	// The contents of this pointer might be modified by this function (we might have to extend the
	// buffer), the intention is that if you're using a buf pool to get this you can put it back after.
	data *[]byte,
) ([]msgs.BlockId, error) {
	if reference == msgs.NULL_INODE_ID {
		reference = id
	}
	log.Debug("writing span spanSize=%v len=%v", spanSize, len(*data))

	if len(*data) < 256 {
		if err := c.createInlineSpan(log, id, cookie, offset, spanSize, *data); err != nil {
			return nil, err
		}
		return []msgs.BlockId{}, nil
	}

	// initiate span add
	bareInitiateReq := prepareSpanInitiateReq(append([]msgs.BlacklistEntry{}, blacklist...), spanPolicies, blockPolicies, stripePolicy, id, cookie, offset, spanSize, data)
	{
		expectedSize := float64(spanSize) * float64(bareInitiateReq.Parity.Blocks()) / float64(bareInitiateReq.Parity.DataBlocks())
		actualSize := bareInitiateReq.CellSize * uint32(bareInitiateReq.Stripes) * uint32(bareInitiateReq.Parity.Blocks())
		log.Debug("span logical size: %v, span physical size: %v, waste: %v%%", spanSize, actualSize, 100.0*(float64(actualSize)-expectedSize)/float64(actualSize))
	}
	initiateReq := &msgs.AddSpanInitiateWithReferenceReq{
		Req:       *bareInitiateReq,
		Reference: reference,
	}

	maxAttempts := 5 // 4 = number of block services that can be down at once in the tests right now
	var err error
	var blocks []msgs.BlockId
	for attempt := 0; ; attempt++ {
		log.Debug("span writing attempt %v", attempt+1)
		initiateResp := msgs.AddSpanInitiateWithReferenceResp{}
		if err = c.ShardRequest(log, id.Shard(), initiateReq, &initiateResp); err != nil {
			return nil, err
		}
		blocks = []msgs.BlockId{}
		for _, block := range initiateResp.Resp.Blocks {
			blocks = append(blocks, block.BlockId)
		}
		// write blocks
		certifyReq := msgs.AddSpanCertifyReq{
			FileId:     id,
			Cookie:     cookie,
			ByteOffset: offset,
			Proofs:     make([]msgs.BlockProof, len(initiateResp.Resp.Blocks)),
		}
		for i, block := range initiateResp.Resp.Blocks {
			var proof [8]byte
			blockCrc, blockReader := mkBlockReader(&initiateReq.Req, *data, i)
			// fail immediately to other block services
			proof, err = c.WriteBlock(log, &timing.NoTimeouts, &block, blockReader, initiateReq.Req.CellSize*uint32(initiateReq.Req.Stripes), blockCrc)
			if err != nil {
				initiateReq.Req.Blacklist = append(initiateReq.Req.Blacklist, msgs.BlacklistEntry{FailureDomain: block.BlockServiceFailureDomain})
				log.Info("failed to write block to %+v: %v, might retry without failure domain %q", block, err, string(block.BlockServiceFailureDomain.Name[:]))
				goto FailedAttempt
			}
			certifyReq.Proofs[i].BlockId = block.BlockId
			certifyReq.Proofs[i].Proof = proof
		}
		if err = c.ShardRequest(log, id.Shard(), &certifyReq, &msgs.AddSpanCertifyResp{}); err != nil {
			return nil, err
		}
		// we've managed
		break

	FailedAttempt:
		if attempt >= maxAttempts { // too many failures
			break
		}
		err = nil
		// create temp file, move the bad span there, then we can restart
		constructResp := &msgs.ConstructFileResp{}
		if err := c.ShardRequest(log, id.Shard(), &msgs.ConstructFileReq{Type: msgs.FILE, Note: "bad_add_span_attempt"}, constructResp); err != nil {
			return nil, err
		}
		moveSpanReq := &msgs.MoveSpanReq{
			FileId1:     id,
			ByteOffset1: offset,
			Cookie1:     cookie,
			FileId2:     constructResp.Id,
			ByteOffset2: 0,
			Cookie2:     constructResp.Cookie,
			SpanSize:    spanSize,
		}
		if err := c.ShardRequest(log, id.Shard(), moveSpanReq, &msgs.MoveSpanResp{}); err != nil {
			return nil, err
		}
	}

	return blocks, err
}

func (c *Client) WriteFile(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	dirInfoCache *DirInfoCache,
	dirId msgs.InodeId, // to get policies
	fileId msgs.InodeId,
	cookie [8]byte,
	r io.Reader,
) error {
	spanPolicies := msgs.SpanPolicy{}
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, &spanPolicies); err != nil {
		return err
	}
	blockPolicies := msgs.BlockPolicy{}
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, &blockPolicies); err != nil {
		return err
	}
	stripePolicy := msgs.StripePolicy{}
	if _, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, dirId, &stripePolicy); err != nil {
		return err
	}
	maxSpanSize := spanPolicies.Entries[len(spanPolicies.Entries)-1].MaxSize
	spanBuf := bufPool.Get(int(maxSpanSize))
	defer bufPool.Put(spanBuf)
	spanBufPtr := spanBuf.BytesPtr()
	offset := uint64(0)
	for {
		*spanBufPtr = (*spanBufPtr)[:maxSpanSize]
		read, err := io.ReadFull(r, *spanBufPtr)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return err
		}
		if err == io.EOF {
			break
		}
		*spanBufPtr = (*spanBufPtr)[:read]
		_, err = c.CreateSpan(
			log, []msgs.BlacklistEntry{}, &spanPolicies, &blockPolicies, &stripePolicy, fileId, msgs.NULL_INODE_ID, cookie, offset, uint32(read), spanBufPtr,
		)
		if err != nil {
			return err
		}
		offset += uint64(read)
		if read < int(maxSpanSize) {
			break
		}
	}
	return nil
}

func (c *Client) CreateFile(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	dirInfoCache *DirInfoCache,
	path string, // must be absolute
	r io.Reader,
) (msgs.InodeId, error) {
	if path[0] != '/' {
		return 0, fmt.Errorf("non-absolute file path %v", path)
	}
	dirPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	if fileName == dirPath {
		return 0, fmt.Errorf("bad file path %v", path)
	}
	dirId, err := c.ResolvePath(log, dirPath)
	if err != nil {
		return 0, err
	}
	fileResp := msgs.ConstructFileResp{}
	if err := c.ShardRequest(log, dirId.Shard(), &msgs.ConstructFileReq{Type: msgs.FILE}, &fileResp); err != nil {
		return 0, err
	}
	fileId := fileResp.Id
	cookie := fileResp.Cookie
	if err := c.WriteFile(log, bufPool, dirInfoCache, dirId, fileId, cookie, r); err != nil {
		return 0, err
	}
	if err := c.ShardRequest(log, dirId.Shard(), &msgs.LinkFileReq{FileId: fileId, Cookie: cookie, OwnerId: dirId, Name: fileName}, &msgs.LinkFileResp{}); err != nil {
		return 0, err
	}
	return fileId, nil
}

func (c *Client) fetchInlineSpan(
	inlineSpan *msgs.FetchedInlineSpan,
	crc msgs.Crc,
) (*bufpool.Buf, error) {
	dataCrc := msgs.Crc(crc32c.Sum(0, inlineSpan.Body))
	if dataCrc != crc {
		return nil, fmt.Errorf("header CRC for inline span is %v, but data is %v", crc, dataCrc)
	}
	buf := append([]byte{}, inlineSpan.Body...)
	return bufpool.NewBuf(&buf), nil
}

func (c *Client) fetchMirroredSpan(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	span *SpanWithBlockServices,
) (*bufpool.Buf, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	blockSize := uint32(body.Stripes) * uint32(body.CellSize)

	for _, block := range body.Blocks {
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			continue
		}
		var crc CrcAccumulator
		var br BlockReader
		br.New(bufPool, 0, blockSize, &crc)
		if err := c.FetchBlock(log, nil, blockService, block.BlockId, 0, blockSize, &br); err == nil {
			buf := br.AcquireBuf()
			zeros := int(span.Span.Header.Size) - len(buf.Bytes())
			if zeros >= 0 {
				*buf.BytesPtr() = append(buf.Bytes(), make([]byte, zeros)...)
			} else {
				*buf.BytesPtr() = buf.Bytes()[:span.Span.Header.Size]
			}
			crc := crc32c.ZeroExtend(uint32(crc.Crc), zeros)
			if msgs.Crc(crc) != span.Span.Header.Crc {
				// panic since we CRC every page, this must be a bug in the go logic
				panic(fmt.Errorf("bad CRC, expected %v, got %v", span.Span.Header.Crc, msgs.Crc(crc)))
			}
			return br.AcquireBuf(), nil
		}
		bufPool.Put(br.AcquireBuf())
	}
	return nil, fmt.Errorf("could not find any suitable blocks")
}

func fetchRsSpan(
	log *log.Logger,
	startFetch startFetch,
	bufPool *bufpool.BufPool,
	span *SpanWithBlockServices,
) (*bufpool.Buf, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	blockSize := uint32(body.Stripes) * uint32(body.CellSize)

	dataSize := blockSize * uint32(body.Parity.DataBlocks())

	ch := make(chan *blockCompletion)

	var succeedBlocks int
	var inFlightBlocks int
	var blockIdx int
	dataBlocks := body.Parity.DataBlocks()
	blockBufs := make([]*bufpool.Buf, body.Parity.Blocks())
	blockCrcs := make([]*[15]msgs.Crc, body.Parity.Blocks())
	defer func() {
		for i := range blockBufs {
			bufPool.Put(blockBufs[i])
		}
	}()

	type blockReaderWithIx struct {
		ix        int
		br        BlockReader
		cellsCrcs CellsCrcs
	}

scheduleMoreBlocks:
	for succeedBlocks+inFlightBlocks < dataBlocks && blockIdx < body.Parity.Blocks() {
		block := body.Blocks[blockIdx]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			blockIdx++
			continue
		}
		readerWithIx := blockReaderWithIx{ix: blockIdx}
		readerWithIx.br.New(bufPool, 0, blockSize, &readerWithIx.cellsCrcs)
		readerWithIx.cellsCrcs.New(body.CellSize)
		if err := startFetch.StartFetchBlock(log, blockService, block.BlockId, 0, blockSize, &readerWithIx.br, &readerWithIx, ch); err == nil {
			inFlightBlocks++
		}
		blockIdx++
	}

	for inFlightBlocks > 0 {
		result := <-ch
		inFlightBlocks--
		readerWithIx := result.Extra.(*blockReaderWithIx)
		buf := readerWithIx.br.AcquireBuf()
		if result.Error != nil {
			bufPool.Put(buf)
			goto scheduleMoreBlocks
		}
		blockBufs[readerWithIx.ix] = buf
		blockCrcs[readerWithIx.ix] = readerWithIx.cellsCrcs.Crcs()
		succeedBlocks++
	}

	if succeedBlocks < dataBlocks {
		return nil, fmt.Errorf("could not find any suitable blocks")
	}

	haveBlocksRecover := [][]byte{}
	var haveBlocksRecoverIxs uint32
	// collect what we have in case we need to recover
	for i := range blockBufs {
		if blockBufs[i] != nil {
			haveBlocksRecover = append(haveBlocksRecover, blockBufs[i].Bytes())
			haveBlocksRecoverIxs |= uint32(1) << i
		}
	}
	// recover what is missing
	for i := range dataBlocks {
		if blockBufs[i] != nil {
			continue
		}
		blockBufs[i] = bufPool.Get(int(blockSize))
		rs.Get(body.Parity).RecoverInto(haveBlocksRecoverIxs, haveBlocksRecover, uint8(i), blockBufs[i].Bytes())
		blockCrcs[i] = &[15]msgs.Crc{}
		for j := 0; j < int(body.Stripes); j++ {
			blockCrcs[i][j] = msgs.Crc(crc32c.Sum(0, blockBufs[i].Bytes()[j*int(body.CellSize):(j+1)*int(body.CellSize)]))
		}
	}

	buf := bufPool.Get(int(span.Span.Header.Size))

	spanOffset := 0
	blockOffset := 0
	crc := uint32(0)
	for i := range body.Stripes {
		for j := range dataBlocks {
			if spanOffset >= len(buf.Bytes()) {
				break
			}
			copy(buf.Bytes()[spanOffset:], blockBufs[j].Bytes()[blockOffset:blockOffset+int(body.CellSize)])
			spanOffset += int(body.CellSize)
			crc = crc32c.Append(crc, uint32(blockCrcs[j][i]), int(body.CellSize))
		}
		blockOffset += int(body.CellSize)
	}

	clear(buf.Bytes()[min(span.Span.Header.Size, dataSize):]) // zero pad
	crc = crc32c.ZeroExtend(crc, int(span.Span.Header.Size)-int(spanOffset))

	if msgs.Crc(crc) != span.Span.Header.Crc {
		panic(fmt.Errorf("expected CRC %v, got %v", span.Span.Header.Crc, msgs.Crc(crc)))
	}

	return buf, nil
}

// The buf we get out must be returned to the bufPool.
func (c *Client) FetchSpan(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	fileId msgs.InodeId,
	span *SpanWithBlockServices,
) (*bufpool.Buf, error) {
	log.Debug("fetching span file=%v offset=%v", fileId, span.Span.Header.ByteOffset)
	switch {
	// inline storage
	case span.Span.Header.StorageClass == msgs.INLINE_STORAGE:
		return c.fetchInlineSpan(span.Span.Body.(*msgs.FetchedInlineSpan), span.Span.Header.Crc)
	// Mirrored replication
	case span.Span.Body.(*msgs.FetchedBlocksSpan).Parity.DataBlocks() == 1:
		return c.fetchMirroredSpan(log, bufPool, span)
	// RS replication
	default:
		return fetchRsSpan(log, c, bufPool, span)
	}
}

type SpanWithBlockServices struct {
	BlockServices []msgs.BlockService
	Span          *msgs.FetchedSpan
}

func (c *Client) FetchSpans(
	log *log.Logger,
	fileId msgs.InodeId,
) (spans []SpanWithBlockServices, err error) {
	var byteOffset uint64
	for {
		req := msgs.LocalFileSpansReq{FileId: fileId, ByteOffset: byteOffset}
		resp := msgs.LocalFileSpansResp{}
		if err := c.ShardRequest(log, fileId.Shard(), &req, &resp); err != nil {
			return nil, err
		}
		for s := range resp.Spans {
			spans = append(spans, SpanWithBlockServices{BlockServices: resp.BlockServices, Span: &resp.Spans[s]})
		}
		byteOffset = resp.NextOffset
		if byteOffset == 0 {
			break
		}
	}
	log.Debug("fetched spans %+v", spans)
	return spans, err
}

// Thread safe.
type FileReader struct {
	fileId   msgs.InodeId
	fileSize uint64
	spans    []SpanWithBlockServices
	/*
		// We store the CRC of the latest stripe. If we happen to continue
		// from the previous offset (common occurrence) we carry it forward
		// and try to complete it. This is a best-effort check, although it
		// should kick in pretty often.
		crc       msgs.Crc
		crcOffset uint64
	*/
}

func (fr *FileReader) findSpan(offset uint64) *SpanWithBlockServices {
	if fr.fileSize == 0 {
		return nil
	}
	ix := sort.Search(len(fr.spans), func(i int) bool {
		return fr.spans[i].Span.Header.ByteOffset > offset
	})
	ix--
	if offset >= fr.spans[ix].Span.Header.ByteOffset+uint64(fr.spans[ix].Span.Header.Size) {
		return nil
	}
	return &fr.spans[ix]
}

func (f *FileReader) readMirrored(log *log.Logger, client *Client, bufPool *bufpool.BufPool, offset uint64, span *SpanWithBlockServices, p []byte) (int, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	// TODO implement trailing zeros, this functionality is currently unused
	spanDataSize := body.CellSize * uint32(body.Stripes) * uint32(body.Parity.DataBlocks())
	if spanDataSize < span.Span.Header.Size {
		panic("unimplemented trailing zeros fetch")
	}
	fileStart := offset / uint64(msgs.TERN_PAGE_SIZE) * uint64(msgs.TERN_PAGE_SIZE)
	fileEnd := ((offset + uint64(len(p)) + uint64(msgs.TERN_PAGE_SIZE) - 1) / uint64(msgs.TERN_PAGE_SIZE)) * uint64(msgs.TERN_PAGE_SIZE)
	blockStart := uint32(fileStart - span.Span.Header.ByteOffset)
	blockEnd := uint32(fileEnd - span.Span.Header.ByteOffset)
	var err error
	for i := range body.Blocks {
		block := &body.Blocks[i]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			continue
		}
		var br BlockReader
		br.New(bufPool, blockStart, blockEnd, nil)
		// TODO check CRC for entire stripes
		err = client.FetchBlock(log, nil, &span.BlockServices[block.BlockServiceIx], block.BlockId, blockStart, blockEnd, &br)
		if err == nil {
			buf := br.AcquireBuf()
			n := copy(p, buf.Bytes()[offset-fileStart:])
			bufPool.Put(buf)
			return n, nil
		} else {
			bufPool.Put(br.AcquireBuf())
		}
	}
	if err == nil {
		err = fmt.Errorf("could not find suitable block service for file=%v offset=%v", f.fileId, offset)
	}
	return 0, err
}

type readRsStateRecover struct {
	start uint32
	end   uint32
	// Blocks we're downloading because we're recovering stuff.
	// Lenghth is D+P.
	// Note that we might have to recover some blocks that we're
	// also downloading because we need more of them.
	blocks                 []BlockReader
	blocksCh               chan *blockCompletion
	blocksPageCacheWriteCh chan struct{}
	fetching               uint32
	succeeded              uint32
	failed                 uint32
	// This is how many page-cache writes we're doing. It's a counter, not a bitmask.
	writersToPageCache uint32
}

type readRsState struct {
	span *msgs.FetchedBlocksSpan
	// The indices of the region of the span we're interested in.
	// Note that we rely on the data to be larger than the actual
	// span size.
	spanStart uint32
	spanEnd   uint32
	// Blocks we're downloading because we need the data.
	// Lenght is D
	blocks    []BlockReader
	blocksCh  chan *blockCompletion
	fetching  uint32
	succeeded uint32
	failed    uint32
	// Recovery state
	recover *readRsStateRecover
}

func (s *readRsState) recoverFetching() uint32 {
	if s.recover == nil {
		return 0
	}
	return s.recover.fetching
}

func (s *readRsState) writersToPageCache() uint32 {
	if s.recover == nil {
		return 0
	}
	return s.recover.writersToPageCache
}

func (s *readRsState) recoverBlocksCh() chan *blockCompletion {
	if s.recover == nil {
		return nil
	}
	return s.recover.blocksCh
}

func (s *readRsState) blocksPageCacheWriteCh() chan struct{} {
	if s.recover == nil {
		return nil
	}
	return s.recover.blocksPageCacheWriteCh
}

// Block start: where to download from in the block.
// blockFrom/blockTo: where to copy from this block. These offsets are
// relative to the block start.
// outFrom/outTo: where to copy into the output array.
func (rs *readRsState) cell(stripeIx uint32, blockIx uint32) (blockStart uint32, blockFrom uint32, blockTo uint32, outFrom uint32, outTo uint32) {
	stripeSize := rs.span.CellSize * uint32(rs.span.Parity.DataBlocks())
	cellIx := stripeIx*uint32(rs.span.Parity.DataBlocks()) + blockIx
	cellStart := cellIx * rs.span.CellSize
	cellEnd := (cellIx + 1) * rs.span.CellSize
	if cellStart >= rs.spanEnd || cellEnd <= rs.spanStart { // totally out of bounds
		return 0, 0, 0, 0, 0
	}
	blockAdjustment := stripeIx*stripeSize + blockIx*rs.span.CellSize - stripeIx*rs.span.CellSize
	// first one, round down to page, <= is important, we want to set fetchStart below even if spanStart is a multiple of cell size
	if cellStart <= rs.spanStart {
		cellStart = (rs.spanStart / msgs.TERN_PAGE_SIZE) * msgs.TERN_PAGE_SIZE
		blockStart = cellStart - blockAdjustment
		blockFrom = rs.spanStart - blockAdjustment
		outFrom = 0
	} else {
		blockFrom = cellStart - blockAdjustment
		blockStart = blockFrom
		outFrom = cellStart - rs.spanStart
	}
	// last one, round up to page
	if cellEnd > rs.spanEnd {
		cellEnd = ((rs.spanEnd + msgs.TERN_PAGE_SIZE - 1) / msgs.TERN_PAGE_SIZE) * msgs.TERN_PAGE_SIZE
		outTo = rs.spanEnd - rs.spanStart
	} else {
		outTo = cellEnd - rs.spanStart
	}
	blockTo = cellEnd - blockAdjustment
	return blockStart, blockFrom, blockTo, outFrom, outTo
}

// Returns blockStart == blockFrom == blockTo == 0 if we shouldn't download anything from this block.
func (rs *readRsState) blockToDownload(blockIx uint32) (blockStart uint32, blockFrom uint32, blockTo uint32) {
	for stripeIx := range uint32(rs.span.Stripes) {
		thisStart, thisFrom, thisTo, _, _ := rs.cell(stripeIx, blockIx)
		if thisFrom == thisTo {
			continue
		}
		blockStart = min(thisStart, blockStart)
		blockFrom = min(thisFrom, blockFrom)
		blockTo = max(blockTo, thisTo)
	}
	return blockStart, blockFrom, blockTo
}

func (rs *readRsState) copyToOut(blockIx uint32, blockStart uint32, buf []byte, out []byte) {
	for stripeIx := range uint32(rs.span.Stripes) {
		_, blockFrom, blockTo, outFrom, outTo := rs.cell(stripeIx, uint32(blockIx))
		if blockFrom == blockTo {
			continue
		}
		to := out[outFrom:outTo]
		from := buf[blockFrom-blockStart : blockTo-blockStart]
		copy(to, from)
	}
}

func (rs *readRsState) freeBlocks(bufPool *bufpool.BufPool) {
	if rs.blocks != nil {
		for i := range rs.blocks {
			bufPool.Put(rs.blocks[i].AcquireBuf())
		}
	}
	if rs.recover != nil {
		for i := range rs.recover.blocks {
			bufPool.Put(rs.recover.blocks[i].AcquireBuf())
		}
	}
}

func printBlockBitmap(
	numBlocks int,
	bitmap uint32,
) string {
	s := make([]byte, numBlocks)
	for i := range numBlocks {
		if bitmap&(uint32(1)<<i) != 0 {
			s[i] = '1'
		} else {
			s[i] = '0'
		}
	}
	return string(s)
}

type PageCache interface {
	// Return 0 if things could not be found in the cache. Must read multiples of
	// TERN_PAGE_SIZE, or not at all.
	ReadCache(offset uint64, dest []byte) (count int)
	WriteCache(offset uint64, data []byte)
}

type readRsRecoverExtra struct {
	blockIx           uint8
	readFromPageCache uint32 // the portion of the read that came from page cache, always page-aligned
}

func readRsStartFetchBlockWithCache(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	startFetch startFetch,
	pageCache PageCache,
	span *SpanWithBlockServices,
	state *readRsState,
	blockIx uint8,
) error {
	reader := &state.recover.blocks[blockIx]
	reader.New(bufPool, state.recover.start, state.recover.end-state.recover.start, nil)
	offset := state.recover.start
	count := state.recover.end - state.recover.start
	extra := readRsRecoverExtra{blockIx: blockIx}
	// If we have the page cache, try to retrieve data blocks from it. We read
	// from the page cache until we can, left to right, and hand off to block
	// services from when we can't onwards. We trust the page cache to not return
	// bad results (i.e. we don't, and can't, CRC its pages).
	block := &state.span.Blocks[blockIx]
	if pageCache == nil || blockIx >= uint8(state.span.Parity.DataBlocks()) {
		return startFetch.StartFetchBlock(log, &span.BlockServices[block.BlockServiceIx], block.BlockId, offset, count, reader, extra, state.recover.blocksCh)
	}
	go func() {
		// Go cell-by-cell, fetching from the page cache as much as we can
		stripeSize := uint32(state.span.Parity.DataBlocks()) * state.span.CellSize
		readerBuf := reader.PagesWithCrcBuffer()
		for stripeIx := range uint32(state.span.Stripes) {
			blockCellStart := stripeIx * state.span.CellSize
			blockCellEnd := blockCellStart + state.span.CellSize
			if blockCellEnd <= offset {
				continue
			}
			if blockCellStart >= offset+count {
				break
			}
			fileStart := span.Span.Header.ByteOffset + uint64(stripeSize*stripeIx) + uint64(uint32(blockIx)*state.span.CellSize)
			pageCacheReadCount := state.span.CellSize
			if blockCellStart < offset {
				fileStart += uint64(offset - blockCellStart)
				pageCacheReadCount -= offset - blockCellStart
			}
			if blockCellEnd > offset+count {
				pageCacheReadCount -= blockCellEnd - (offset + count)
			}
			// Spilling after end of file, round down to page. I think technically we could probably
			// round _up_ to page, but let's be safe.
			if fileStart+uint64(pageCacheReadCount) > span.Span.Header.ByteOffset+uint64(span.Span.Header.Size) {
				fileEnd := ((span.Span.Header.ByteOffset + uint64(span.Span.Header.Size)) / uint64(msgs.TERN_PAGE_SIZE)) * uint64(msgs.TERN_PAGE_SIZE)
				if fileEnd <= fileStart {
					break
				}
				pageCacheReadCount = uint32(fileEnd - fileStart)
			}
			r := pageCache.ReadCache(fileStart, readerBuf[extra.readFromPageCache:extra.readFromPageCache+pageCacheReadCount])
			// Round down to page, Linux's page cache should never return half-pages I think, but
			// let's be safe.
			r = (r / int(msgs.TERN_PAGE_SIZE)) * int(msgs.TERN_PAGE_SIZE)
			extra.readFromPageCache += uint32(r)
			if r < int(pageCacheReadCount) {
				break
			}
		}
		reader.Advance(extra.readFromPageCache)
		if extra.readFromPageCache == count { // we got everything from page cache
			_, err := reader.ReadFrom(bytes.NewReader([]byte{})) // realign
			state.recover.blocksCh <- &blockCompletion{
				Error: err,
				Extra: extra,
			}
		} else {
			if err := startFetch.StartFetchBlock(log, &span.BlockServices[block.BlockServiceIx], block.BlockId, offset+extra.readFromPageCache, count-extra.readFromPageCache, reader, extra, state.recover.blocksCh); err != nil {
				state.recover.blocksCh <- &blockCompletion{
					Error: err,
					Extra: extra,
				}
			}
		}
	}()
	return nil
}

func readRsWriteToPageCache(
	log *log.Logger,
	pageCache PageCache,
	span *SpanWithBlockServices,
	state *readRsState,
	extra *readRsRecoverExtra,
) {
	// If we have a page cache, and this is a data block, and this didn't all come from page
	// cache, write out the other parts which are within file boundaries.
	if pageCache != nil && int(extra.blockIx) < state.span.Parity.DataBlocks() && extra.readFromPageCache < state.recover.end-state.recover.start {
		stripeSize := uint32(state.span.Parity.DataBlocks()) * state.span.CellSize
		blockStart, _, blockTo := state.blockToDownload(uint32(extra.blockIx)) // blockStart is what we want -- it's already page-aligned.
		noFetch := blockStart == blockTo
		pageCacheFrom := state.recover.start + extra.readFromPageCache
		pageCacheTo := state.recover.end
		for stripeIx := range uint32(state.span.Stripes) {
			blockCellStart := stripeIx * state.span.CellSize
			blockCellEnd := blockCellStart + state.span.CellSize
			if blockCellEnd <= pageCacheFrom { // didn't get to the interesting region yet
				continue
			}
			if blockCellStart >= pageCacheTo { // reached the end
				break
			}
			if blockCellStart < pageCacheFrom {
				blockCellStart = pageCacheFrom
			}
			if blockCellEnd > pageCacheTo {
				blockCellEnd = pageCacheTo
			}
			fileAdjustment := -stripeIx*state.span.CellSize + stripeIx*stripeSize + uint32(extra.blockIx)*state.span.CellSize
			fileStart := span.Span.Header.ByteOffset + uint64(blockCellStart) + uint64(fileAdjustment)
			fileEnd := span.Span.Header.ByteOffset + uint64(blockCellEnd) + uint64(fileAdjustment)
			clippedFileEnd := min(fileEnd, span.Span.Header.ByteOffset+uint64(span.Span.Header.Size))
			clippedLen := int64(clippedFileEnd) - int64(fileStart)
			if clippedLen < 0 { // ran over the file
				break
			}
			blockCellEnd = min(blockCellEnd, blockCellStart+uint32(clippedLen))
			if noFetch { // we weren't fetching this at all, write everything
				state.recover.writersToPageCache++
				go func() {
					defer func() { state.recover.blocksPageCacheWriteCh <- struct{}{} }()
					pageCache.WriteCache(uint64(fileStart), state.recover.blocks[extra.blockIx].Buf().Bytes()[uint32(blockCellStart)-state.recover.start:uint32(blockCellEnd)-state.recover.start])
				}()
			} else {
				if blockCellStart < blockStart { // we have stuff to write before
					state.recover.writersToPageCache++
					end := min(blockCellEnd, blockStart)
					go func() {
						defer func() { state.recover.blocksPageCacheWriteCh <- struct{}{} }()
						pageCache.WriteCache(fileStart, state.recover.blocks[extra.blockIx].Buf().Bytes()[blockCellStart-state.recover.start:end-state.recover.start])
					}()
				}
				if blockCellEnd > blockTo { // we have stuff to write after
					state.recover.writersToPageCache++
					begin := max(blockTo, blockCellStart)
					go func() {
						defer func() { state.recover.blocksPageCacheWriteCh <- struct{}{} }()
						pageCache.WriteCache(fileStart+uint64(begin-blockCellStart), state.recover.blocks[extra.blockIx].Buf().Bytes()[begin-state.recover.start:blockCellEnd-state.recover.start])
					}()
				}
			}
		}
	}
}

func readRsRecover(
	log *log.Logger,
	startFetch startFetch,
	pageCache PageCache,
	bufPool *bufpool.BufPool,
	span *SpanWithBlockServices,
	out []byte,
	state *readRsState,
) (int, error) {
	// Sad case -- we didn't. We switch to the fetching mode whereby
	// we try to fetch from the page cache first. We also make a somewhat
	// pessimistic assumption: we assume that any recover block might
	// need to cover any other block. This saves us from cases where we
	// started covering from one block but then we actually need to cover
	// more because another block is involved, too. So the first thing
	// we compute is the from/to recover we need for any recover block we
	// will start.
	state.recover = &readRsStateRecover{
		blocks:                 make([]BlockReader, state.span.Parity.Blocks()),
		blocksCh:               make(chan *blockCompletion, state.span.Parity.DataBlocks()),
		blocksPageCacheWriteCh: make(chan struct{}),
	}
	for blockIx := range uint32(state.span.Parity.DataBlocks()) {
		blockStart, _, blockTo := state.blockToDownload(blockIx)
		if blockStart == blockTo {
			continue
		}
		state.recover.start = min(state.recover.start, blockStart)
		state.recover.end = max(blockTo, state.recover.end)
	}
	// Mark things that don't need recover fetching as complete already,
	// and those that are failed or can't be read failed also. Again we
	// cut corners a bit and if we need to recover anything at all we
	// re-request everything from the block. If we don't need to request
	// anything we just use the fetched data.
	for blockIx := range uint32(state.span.Parity.DataBlocks()) {
		if state.failed&(uint32(1)<<blockIx) != 0 || !span.BlockServices[state.span.Blocks[blockIx].BlockServiceIx].Flags.CanRead() {
			state.recover.failed |= uint32(1) << blockIx
		} else {
			if state.succeeded&(uint32(1)<<blockIx) != 0 {
				blockStart, _, blockTo := state.blockToDownload(blockIx)
				if blockStart <= state.recover.start && state.recover.end <= blockTo {
					state.recover.succeeded |= uint32(1) << blockIx
				}
			}
		}
	}
	// Mark the parity blocks we can't read as failed also.
	for pBlockIx := range uint32(state.span.Parity.ParityBlocks()) {
		blockIx := uint32(state.span.Parity.DataBlocks()) + pBlockIx
		if !span.BlockServices[state.span.Blocks[blockIx].BlockServiceIx].Flags.CanRead() {
			state.recover.failed |= uint32(1) << blockIx
		}
	}
	// OK, now we take the steps needed to download what we need to
	// reconstruct stuff, stopping when we're hopeless.
	for bits.OnesCount32(state.recover.succeeded) < state.span.Parity.DataBlocks() && bits.OnesCount32(state.recover.failed) <= state.span.Parity.ParityBlocks() {
		// Find new recover blocks to start
		for blockIx := uint32(0); blockIx < uint32(state.span.Parity.Blocks()) && bits.OnesCount32(state.fetching|state.recover.succeeded|state.recover.fetching) < state.span.Parity.DataBlocks(); blockIx++ {
			if (uint32(1)<<blockIx)&(state.fetching|state.recover.fetching|state.recover.succeeded|state.recover.failed) != 0 {
				// We've already dealt or are dealing with this one
				continue
			}
			// Start fetching, with page cache.
			if err := readRsStartFetchBlockWithCache(log, bufPool, startFetch, pageCache, span, state, uint8(blockIx)); err != nil {
				state.recover.failed |= uint32(1) << blockIx
			} else {
				state.recover.fetching |= uint32(1) << blockIx
			}
		}
		// Wait for at least one update
		select {
		case resp := <-state.blocksCh:
			blockIx := resp.Extra.(uint8)
			state.fetching &= ^(uint32(1) << blockIx)
			if resp.Error != nil {
				state.failed |= uint32(1) << blockIx
				state.recover.failed |= uint32(1) << blockIx
			} else {
				reader := &state.blocks[blockIx]
				buf := reader.Buf()
				blockStart, _, blockTo := state.blockToDownload(uint32(blockIx))
				// If the fetched subsumed the resuming, mark it as such
				if blockStart <= state.recover.start && state.recover.end <= blockTo {
					state.recover.succeeded |= uint32(1) << blockIx
				}
				state.copyToOut(uint32(blockIx), blockStart, buf.Bytes(), out)
				state.succeeded |= uint32(1) << blockIx
			}
		case resp := <-state.recover.blocksCh:
			extra := resp.Extra.(readRsRecoverExtra)
			state.recover.fetching &= ^(uint32(1) << extra.blockIx)
			if resp.Error != nil {
				state.recover.failed |= uint32(1) << extra.blockIx
			} else {
				state.recover.succeeded |= uint32(1) << extra.blockIx
				readRsWriteToPageCache(log, pageCache, span, state, &extra)
			}
		}
	}
	// We've made it, do the reconstruction.
	if bits.OnesCount32(state.recover.succeeded) == state.span.Parity.DataBlocks() {
		// Accumulate successful blocks
		haveBlocks := make([][]byte, 0, state.span.Parity.DataBlocks())
		for blockIx := range uint32(state.span.Parity.Blocks()) {
			if state.recover.succeeded&(uint32(1)<<blockIx) != 0 {
				if buf := state.recover.blocks[blockIx].Buf(); buf != nil { // it was fetched during recovery
					haveBlocks = append(haveBlocks, buf.Bytes())
				} else { // it was fetched during normal reading
					blockStart, _, _ := state.blockToDownload(blockIx)
					buf := state.blocks[blockIx].Buf()
					haveBlocks = append(haveBlocks, buf.Bytes()[state.recover.start-blockStart:state.recover.end-blockStart])
				}
			}
		}
		// Buffer to store the RS result (note that the fetching buf, if present,
		// might be smaller).
		recoverBuf := bufPool.Get(int(state.recover.end - state.recover.start))
		defer bufPool.Put(recoverBuf)
		// Now do the recovery + copy for every failed block
		rs := rs.Get(state.span.Parity)
		for blockIx := range uint32(state.span.Parity.DataBlocks()) {
			if state.failed&(uint32(1)<<blockIx) != 0 {
				rs.RecoverInto(state.recover.succeeded, haveBlocks, uint8(blockIx), recoverBuf.Bytes())
				state.copyToOut(uint32(blockIx), state.recover.start, recoverBuf.Bytes(), out)
			}
		}
		return int(state.spanEnd - state.spanStart), nil
	}
	// We very much have not made it.
	return 0, fmt.Errorf("could not recover span data succeeded=%s fetching=%s failed=%v recoverSucceeded=%v recoverFetching=%v recoverFailed=%v", printBlockBitmap(state.span.Parity.DataBlocks(), state.succeeded), printBlockBitmap(state.span.Parity.DataBlocks(), state.fetching), printBlockBitmap(state.span.Parity.DataBlocks(), state.failed), printBlockBitmap(state.span.Parity.Blocks(), state.recover.succeeded), printBlockBitmap(state.span.Parity.Blocks(), state.recover.fetching), printBlockBitmap(state.span.Parity.Blocks(), state.recover.failed))
}

type startFetch interface {
	StartFetchBlock(log *log.Logger, blockService *msgs.BlockService, blockId msgs.BlockId, offset uint32, count uint32, w io.ReaderFrom, extra any, completion chan *blockCompletion) error
}

// The goal here is to read as little as possible (i.e. only the pages
// involved), which is a common use case for flash storage. We also assume
// that while the currently requested region is _not_ in the page cache,
// other regions might, so we try fetching there first if we need to
// reconstruct.
//
// `startFetch` is stubbed out so that we can mock it and test this
// pretty tricky function in isolation.
func readRs(
	log *log.Logger,
	startFetch startFetch,
	pageCache PageCache,
	bufPool *bufpool.BufPool,
	offset uint32, // offset in the _span_.
	span *SpanWithBlockServices,
	out []byte,
) (int, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	// TODO implement trailing zeros, this functionality is currently unused
	spanDataSize := body.CellSize * uint32(body.Stripes) * uint32(body.Parity.DataBlocks())
	if spanDataSize < span.Span.Header.Size {
		panic(fmt.Errorf("unimplemented trailing zeros fetch spanSize=%v spanDataSize=%v", span.Span.Header.Size, spanDataSize))
	}
	state := readRsState{
		span:     body,
		blocksCh: make(chan *blockCompletion, body.Parity.DataBlocks()),
		blocks:   make([]BlockReader, body.Parity.DataBlocks()),
	}
	state.spanStart = offset
	state.spanEnd = state.spanStart + uint32(min(uint64(len(out)), uint64(span.Span.Header.Size-state.spanStart)))
	// Cleanup all buffers when done
	defer func() {
		fetching := bits.OnesCount32(state.fetching)
		recoverFetching := bits.OnesCount32(state.recoverFetching())
		if fetching > 0 || recoverFetching > 0 || state.writersToPageCache() > 0 {
			go func() {
				// Need to make sure that all requests to be finished
				// before we can free the buffers. No need to block
				// the exit though.
				for fetching > 0 || recoverFetching > 0 || state.writersToPageCache() > 0 {
					select {
					case <-state.blocksCh:
						fetching--
					case <-state.recoverBlocksCh():
						recoverFetching--
					case <-state.blocksPageCacheWriteCh():
						state.recover.writersToPageCache--
					}

				}
				state.freeBlocks(bufPool)
			}()
		} else {
			state.freeBlocks(bufPool)
		}
	}()
	for blockIx := range uint32(body.Parity.DataBlocks()) {
		start, from, to := state.blockToDownload(blockIx)
		if from == to {
			state.succeeded |= uint32(1) << blockIx
			continue
		}
		count := to - start
		state.blocks[blockIx].New(bufPool, start, count, nil)
		block := state.span.Blocks[blockIx]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			state.failed |= uint32(1) << blockIx
		} else if err := startFetch.StartFetchBlock(log, &span.BlockServices[block.BlockServiceIx], block.BlockId, start, count, &state.blocks[blockIx], uint8(blockIx), state.blocksCh); err != nil {
			state.failed |= uint32(1) << blockIx
		} else {
			state.fetching |= uint32(1) << blockIx
		}
	}
	// Proceed as long as everyone's happy and as long as we're not done.
	// As soon as something fails, immediately bail so that we'll start
	// downloading more stuff.
	for bits.OnesCount32(state.succeeded) < body.Parity.DataBlocks() && bits.OnesCount32(state.failed) == 0 {
		resp := <-state.blocksCh
		blockIx := resp.Extra.(uint8)
		state.fetching &= ^(uint32(1) << blockIx)
		if resp.Error != nil {
			state.failed |= uint32(1) << blockIx
		} else {
			reader := &state.blocks[blockIx]
			buf := reader.Buf()
			blockStart, _, _ := state.blockToDownload(uint32(blockIx))
			state.copyToOut(uint32(blockIx), blockStart, buf.Bytes(), out)
			state.succeeded |= uint32(1) << blockIx
		}
	}
	// Happy case: we got them all on the first try
	if bits.OnesCount32(state.succeeded) == body.Parity.DataBlocks() {
		return int(state.spanEnd - state.spanStart), nil
	}
	// Go into recovery
	return readRsRecover(log, startFetch, pageCache, bufPool, span, out, &state)
}

// Thread safe (but concurrent access might lead to duplicated fetches).
func (f *FileReader) Read(
	log *log.Logger,
	client *Client,
	pageCache PageCache,
	bufPool *bufpool.BufPool,
	offset uint64,
	dest []byte,
) (int, error) {
	span := f.findSpan(offset)
	if span == nil {
		return 0, io.EOF
	}
	if len(dest) == 0 {
		return 0, nil
	}
	if span.Span.Header.StorageClass == msgs.INLINE_STORAGE {
		body := span.Span.Body.(*msgs.FetchedInlineSpan)
		// TODO easy to implement I just don't want to commit effectively untested code
		if span.Span.Header.Size != uint32(len(body.Body)) {
			panic("unimplemented trailing zeros")
		}
		return copy(dest, body.Body[offset-span.Span.Header.ByteOffset:]), nil
	}
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	if body.Parity.DataBlocks() == 1 { // mirrored
		return f.readMirrored(log, client, bufPool, offset, span, dest)
	} else { // RS
		return readRs(log, client, pageCache, bufPool, uint32(offset-span.Span.Header.ByteOffset), span, dest)
	}
}

func (client *Client) NewFileReader(log *log.Logger, id msgs.InodeId) (*FileReader, error) {
	spans, err := client.FetchSpans(log, id)
	if err != nil {
		return nil, err
	}
	r := &FileReader{
		fileId: id,
		spans:  spans,
	}
	for i := range r.spans {
		r.fileSize += uint64(r.spans[i].Span.Header.Size)
	}
	return r, nil
}

type fileReader struct {
	client  *Client
	log     *log.Logger
	bufPool *bufpool.BufPool
	offset  uint64
	reader  *FileReader
}

func (f *fileReader) Read(p []byte) (int, error) {
	n, err := f.reader.Read(f.log, f.client, nil, f.bufPool, f.offset, p)
	f.Seek(int64(n), io.SeekCurrent)
	return n, err
}

func (f *fileReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset = int64(f.offset) + offset
	case io.SeekEnd:
		offset = int64(f.reader.fileSize) + offset
	default:
		return 0, syscall.EINVAL
	}
	if offset < 0 {
		return 0, syscall.EINVAL
	}
	f.offset = uint64(offset)
	return int64(f.offset), nil
}

func (f *fileReader) Close() error {
	return nil
}

func (c *Client) ReadFile(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	id msgs.InodeId,
) (io.ReadSeekCloser, error) {
	fr, err := c.NewFileReader(log, id)
	if err != nil {
		return nil, err
	}
	r := &fileReader{
		client:  c,
		log:     log,
		bufPool: bufPool,
		reader:  fr,
	}
	return r, nil
}

func (c *Client) FetchFile(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	id msgs.InodeId,
) (*bufpool.Buf, error) {
	spans, err := c.FetchSpans(log, id)
	if err != nil {
		return nil, err
	}
	bufs := make([]*bufpool.Buf, len(spans))
	defer func() {
		for _, b := range bufs {
			bufPool.Put(b)
		}
	}()
	wg := sync.WaitGroup{}
	totalSize := 0
	for i := range len(spans) {
		totalSize += int(spans[i].Span.Header.Size)
		wg.Add(1)
		go func(idx int) {
			buf, err := c.FetchSpan(log, bufPool, id, &spans[idx])
			if err == nil {
				bufs[idx] = buf
			} else {
				log.ErrorNoAlert("failed to fetch span %v", err)
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	buf := bufPool.Get(totalSize)
	for i := range len(spans) {
		if bufs[i] == nil {
			bufPool.Put(buf)
			return nil, fmt.Errorf("failed to fetch file %v", id)
		}
		if len(bufs[i].Bytes()) != int(spans[i].Span.Header.Size) {
			bufPool.Put(buf)
			return nil, fmt.Errorf("failed to fetch file %v, span size mismatch", id)
		}
		copy(buf.Bytes()[spans[i].Span.Header.ByteOffset:], bufs[i].Bytes())
	}

	return buf, nil
}
