// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package client

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	"xtx/ternfs/divide32"
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
	// memset? what's that?
	for i := lenBefore; i < len(*buf); i++ {
		(*buf)[i] = 0
	}
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
		br := NewBlockReader(bufPool, 0, blockSize, blockSize)
		if err := c.FetchBlock(log, nil, blockService, block.BlockId, 0, blockSize, br); err == nil {
			buf := br.AcquireBuf()
			zeros := int(span.Span.Header.Size) - len(buf.Bytes())
			if zeros >= 0 {
				*buf.BytesPtr() = append(buf.Bytes(), make([]byte, zeros)...)
			} else {
				*buf.BytesPtr() = buf.Bytes()[:span.Span.Header.Size]
			}
			crc := crc32c.ZeroExtend(uint32(br.Crcs()[0]), zeros)
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

func (c *Client) fetchRsSpan(
	log *log.Logger,
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
	blockCrcs := make([][]msgs.Crc, body.Parity.Blocks())
	defer func() {
		for i := range blockBufs {
			bufPool.Put(blockBufs[i])
		}
	}()

	type blockReaderWithIx struct {
		ix int
		br *BlockReader
	}

scheduleMoreBlocks:
	for succeedBlocks+inFlightBlocks < dataBlocks && blockIdx < body.Parity.Blocks() {
		block := body.Blocks[blockIdx]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			blockIdx++
			continue
		}
		readerWithIx := blockReaderWithIx{
			ix: blockIdx,
			br: NewBlockReader(bufPool, 0, blockSize, body.CellSize),
		}
		if err := c.StartFetchBlock(log, blockService, block.BlockId, 0, blockSize, readerWithIx.br, &readerWithIx, ch); err == nil {
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
		blockCrcs[readerWithIx.ix] = readerWithIx.br.Crcs()
		succeedBlocks++
	}

	if succeedBlocks < dataBlocks {
		return nil, fmt.Errorf("could not find any suitable blocks")
	}

	haveBlocksRecover := [][]byte{}
	haveBlocksRecoverIxs := []uint8{}
	// collect what we have in case we need to recover
	for i := range blockBufs {
		if blockBufs[i] != nil {
			haveBlocksRecover = append(haveBlocksRecover, blockBufs[i].Bytes())
			haveBlocksRecoverIxs = append(haveBlocksRecoverIxs, uint8(i))
		}
	}
	// recover what is missing
	for i := range dataBlocks {
		if blockBufs[i] != nil {
			continue
		}
		blockBufs[i] = bufPool.Get(int(blockSize))
		rs.Get(body.Parity).RecoverInto(haveBlocksRecoverIxs, haveBlocksRecover, uint8(i), blockBufs[i].Bytes())
		blockCrcs[i] = make([]msgs.Crc, body.Stripes)
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
		return c.fetchRsSpan(log, bufPool, span)
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

type FileReader struct {
}

type fileReader struct {
	client   *Client
	log      *log.Logger
	bufPool  *bufpool.BufPool
	fileId   msgs.InodeId
	mu       sync.Mutex
	fileSize uint64
	spans    []SpanWithBlockServices
	offset   uint64
}

func (f *fileReader) Close() error {
	return nil
}

func (fr *fileReader) findSpan(offset uint64) *SpanWithBlockServices {
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

func (f *fileReader) readFallback(span *SpanWithBlockServices, p []byte) (int, error) {
	buf, err := f.client.FetchSpan(f.log, f.bufPool, f.fileId, span)
	defer f.bufPool.Put(buf)
	if err != nil {
		return 0, err
	}
	r := copy(p, buf.Bytes()[f.offset-span.Span.Header.ByteOffset:])
	return r, nil
}

func (f *fileReader) readMirrored(span *SpanWithBlockServices, p []byte) (int, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	// TODO implement trailing zeros, this functionality is currently unused
	spanDataSize := body.CellSize * uint32(body.Stripes) * uint32(body.Parity.DataBlocks())
	if spanDataSize < span.Span.Header.Size {
		panic("unimplemented trailing zeros fetch")
	}
	fileStart := f.offset / uint64(msgs.TERN_PAGE_SIZE) * uint64(msgs.TERN_PAGE_SIZE)
	fileEnd := ((f.offset + uint64(len(p)) + uint64(msgs.TERN_PAGE_SIZE) - 1) / uint64(msgs.TERN_PAGE_SIZE)) * uint64(msgs.TERN_PAGE_SIZE)
	blockStart := uint32(fileStart - span.Span.Header.ByteOffset)
	blockEnd := uint32(fileEnd - span.Span.Header.ByteOffset)
	var err error
	for i := range body.Blocks {
		block := &body.Blocks[i]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			continue
		}
		br := NewBlockReader(f.bufPool, blockStart, blockEnd, blockEnd-blockStart)
		defer f.bufPool.Put(br.AcquireBuf())
		// TODO check CRC for entire stripes
		err = f.client.FetchBlock(f.log, nil, &span.BlockServices[block.BlockServiceIx], block.BlockId, blockStart, blockEnd, br)
		if err == nil {
			buf := br.AcquireBuf()
			n := copy(p, buf.Bytes()[f.offset-fileStart:])
			f.bufPool.Put(buf)
			return n, nil
		}
		f.bufPool.Put(br.AcquireBuf())
	}
	if err == nil {
		err = fmt.Errorf("could not find suitable block service for file=%v offset=%v", f.fileId, f.offset)
	}
	return 0, err
}

// So that we can do copy(out[outStart:outEnd], fetchedBlock[blockStart-segments.fetchStart:blockEnd-segments.fetchStart])
type readRsBlockSegment struct {
	outStart   uint32
	outEnd     uint32
	blockStart uint32
	blockEnd   uint32
}

type readRsBlock struct {
	segments    [15]readRsBlockSegment // max 15 stripes
	numSegments int
	fetchStart  uint32 // where to start fetching this block from
	reader      *BlockReader
}

func (b *readRsBlock) blockCount() uint32 {
	return b.segments[b.numSegments-1].blockEnd - b.fetchStart
}

func (f *fileReader) readRs(span *SpanWithBlockServices, p []byte) (int, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	// TODO implement trailing zeros, this functionality is currently unused
	spanDataSize := body.CellSize * uint32(body.Stripes) * uint32(body.Parity.DataBlocks())
	if spanDataSize < span.Span.Header.Size {
		panic("unimplemented trailing zeros fetch")
	}
	// fill in segments
	var blocks [16]readRsBlock
	// The indices of the region inside the span that we're interested in.
	// Note that we rely on the data to be larger than the actual size (see above)
	spanStart := uint32(f.offset - span.Span.Header.ByteOffset)
	spanEnd := spanStart + uint32(min(uint64(len(p)), uint64(span.Span.Header.Size-spanStart)))
	{
		// Go cell-by-cell. We start from the cell that contains spanStart,
		// and end up in the cell that contains spanEnd-1.
		stripeSize := body.CellSize*uint32(body.Parity.DataBlocks())
		dataDiv := divide32.NewDivisor(uint32(body.Parity.DataBlocks()))
		for cellIx := spanStart / body.CellSize; cellIx < (spanEnd+body.CellSize-1)/body.CellSize; cellIx++ {
			stripeIx := dataDiv.Div(cellIx)
			blockIx := dataDiv.Mod(cellIx)
			segments := &blocks[blockIx]
			segment := &segments.segments[segments.numSegments]
			cellStart := cellIx * body.CellSize
			blockAdjustment := stripeIx*stripeSize + blockIx*body.CellSize - stripeIx*body.CellSize
			if cellStart <= spanStart { // first one, round down to page, <= is important, we want to set fetchStart below even if spanStart is a multiple of cell size
				cellStart = (spanStart / msgs.TERN_PAGE_SIZE) * msgs.TERN_PAGE_SIZE
				segments.fetchStart = cellStart - blockAdjustment
				segment.blockStart = spanStart - blockAdjustment
				segment.outStart = 0
			} else {
				segment.blockStart = cellStart - blockAdjustment
				segment.outStart = cellStart - spanStart
			}
			cellEnd := (cellIx + 1) * body.CellSize
			if cellEnd > spanEnd { // last one, round up to page
				cellEnd = ((spanEnd + msgs.TERN_PAGE_SIZE - 1) / msgs.TERN_PAGE_SIZE) * msgs.TERN_PAGE_SIZE
				segment.outEnd = spanEnd - spanStart
			} else {
				segment.outEnd = cellEnd - spanStart
			}
			segment.blockEnd = cellEnd - blockAdjustment
			segments.numSegments++
		}
	}
	// start segments
	blocksFetching := 0
	var err error
	ch := make(chan *blockCompletion, body.Parity.DataBlocks())
	for i := range body.Parity.DataBlocks() {
		segments := &blocks[i]
		if segments.numSegments == 0 {
			continue
		}
		block := &body.Blocks[i]
		segments.reader = NewBlockReader(f.bufPool, segments.fetchStart, segments.blockCount(), segments.blockCount())
		if err = f.client.StartFetchBlock(f.log, &span.BlockServices[block.BlockServiceIx], block.BlockId, segments.fetchStart, segments.blockCount(), segments.reader, segments, ch); err != nil {
			break
		}
		blocksFetching++
	}
	copied := 0
	if err != nil {
		goto Err
	}
	// fetch results
	for blocksFetching > 0 {
		resp := <-ch
		blocksFetching--
		segments := resp.Extra.(*readRsBlock)
		if resp.Error != nil {
			f.bufPool.Put(segments.reader.AcquireBuf())
			err = resp.Error
			goto Err
		} else {
			buf := segments.reader.AcquireBuf()
			for i := 0; i < segments.numSegments; i++ {
				segment := &segments.segments[i]
				to := p[segment.outStart:segment.outEnd]
				from := buf.Bytes()[segment.blockStart-segments.fetchStart:segment.blockEnd-segments.fetchStart]
				copied += copy(to, from)
			}
			f.bufPool.Put(buf)
		}
	}
	if copied != int(spanEnd)-int(spanStart) {
		panic("impossible")
	}
	return copied, nil
Err:
	if err == nil {
		panic("impossible")
	}
	f.log.Info("Could not read file=%v offset=%v len=%v through stripes, falling back to entire span: %v", f.fileId, f.offset, len(p), err)
	for range blocksFetching {
		resp := <-ch
		segments := resp.Extra.(*readRsBlock)
		f.bufPool.Put(segments.reader.AcquireBuf())
	}
	return 0, err
}

func (f *fileReader) readInternal(p []byte) (int, error) {
	span := f.findSpan(f.offset)
	if span == nil {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}
	if span.Span.Header.StorageClass == msgs.INLINE_STORAGE {
		body := span.Span.Body.(*msgs.FetchedInlineSpan)
		// TODO easy to implement I just don't want to commit effectively untested code
		if span.Span.Header.Size != uint32(len(body.Body)) {
			panic("unimplemented trailing zeros")
		}
		return copy(p, body.Body[f.offset-span.Span.Header.ByteOffset:]), nil
	}
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	if body.Parity.DataBlocks() == 1 { // mirrored
		return f.readMirrored(span, p)
	} else { // RS
		// if this doesn't work, we fall back to reading the whole span,
		// we should be smarter and only reconstruct what's needed instead
		n, err := f.readRs(span, p)
		if err != nil {
			return f.readFallback(span, p)
		}
		return n, err
	}
}

func (f *fileReader) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := f.readInternal(p)
	f.log.Debug("read file=%v fileSize=%v offset=%v len=%v read=%v", f.fileId, f.fileSize, f.offset, len(p), n)
	f.offset += uint64(n)
	return n, err
}

func (f *fileReader) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset = int64(f.offset) + offset
	case io.SeekEnd:
		offset = int64(f.fileSize) + offset
	default:
		return 0, syscall.EINVAL
	}
	if offset < 0 {
		return 0, syscall.EINVAL
	}
	f.offset = uint64(offset)
	return int64(f.offset), nil
}

func (c *Client) ReadFile(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	id msgs.InodeId,
) (io.ReadSeekCloser, error) {
	spans, err := c.FetchSpans(log, id)
	if err != nil {
		return nil, err
	}
	r := &fileReader{
		client:  c,
		log:     log,
		bufPool: bufPool,
		fileId:  id,
		spans:   spans,
	}
	for i := range r.spans {
		r.fileSize += uint64(r.spans[i].Span.Header.Size)
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
