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

type FetchedStripe struct {
	Buf   *bufpool.Buf
	Start uint64
	owned bool
}

func (fs *FetchedStripe) Put(bufPool *bufpool.BufPool) {
	if !fs.owned {
		return
	}
	fs.owned = false
	bufPool.Put(fs.Buf)
}

func (c *Client) fetchCell(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	blockServices []msgs.BlockService,
	body *msgs.FetchedBlocksSpan,
	blockIx uint8,
	cell uint8,
) (buf *bufpool.Buf, err error) {
	buf = bufPool.Get(int(body.CellSize))
	defer func() {
		if err != nil {
			bufPool.Put(buf)
		}
	}()
	block := &body.Blocks[blockIx]
	blockService := &blockServices[block.BlockServiceIx]
	var data *bytes.Buffer
	// fail immediately to other block services
	data, err = c.FetchBlock(log, &timing.NoTimeouts, blockService, block.BlockId, uint32(cell)*body.CellSize, body.CellSize, block.Crc)
	if err != nil {
		log.Info("could not fetch block from block service %+v: %+v", blockService, err)
		return nil, err
	}
	defer c.PutFetchedBlock(data)
	if copy(buf.Bytes(), data.Bytes()) != int(body.CellSize) {
		panic(fmt.Errorf("runt block cell"))
	}
	return buf, nil
}

func (c *Client) fetchMirroredStripe(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	span *SpanWithBlockServices,
	body *msgs.FetchedBlocksSpan,
	offset uint64,
) (start uint64, buf *bufpool.Buf, err error) {
	spanOffset := uint32(offset - span.Span.Header.ByteOffset)
	cell := spanOffset / body.CellSize
	B := body.Parity.Blocks()
	start = span.Span.Header.ByteOffset + uint64(cell*body.CellSize)

	log.Debug("getting cell %v -> %v", start, span.Span.Header.ByteOffset+uint64((cell+1)*body.CellSize))

	found := false
	for i := 0; i < B && !found; i++ {
		block := &body.Blocks[i]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			continue
		}
		buf, err = c.fetchCell(log, bufPool, span.BlockServices, body, uint8(i), uint8(cell))
		if err != nil {
			continue
		}
		crc := msgs.Crc(crc32c.Sum(0, buf.Bytes()))
		if crc != body.StripesCrc[cell] {
			log.RaiseAlert("expected crc %v, got %v, for block %v in block service %v", body.StripesCrc[cell], crc, block.BlockId, blockService.Id)
			continue
		}
		found = true
	}

	if !found {
		return 0, nil, fmt.Errorf("could not find any suitable blocks")
	}

	return start, buf, nil
}

func (c *Client) fetchRsStripe(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	fileId msgs.InodeId,
	span *SpanWithBlockServices,
	body *msgs.FetchedBlocksSpan,
	offset uint64,
) (start uint64, buf *bufpool.Buf, err error) {
	D := body.Parity.DataBlocks()
	B := body.Parity.Blocks()
	spanOffset := uint32(offset - span.Span.Header.ByteOffset)
	blocks := make([]*bufpool.Buf, B)
	defer func() {
		for i := range blocks {
			bufPool.Put(blocks[i])
		}
	}()
	blocksFound := 0
	stripe := spanOffset / (uint32(D) * body.CellSize)
	if stripe >= uint32(body.Stripes) {
		panic(fmt.Errorf("impossible: stripe %v >= stripes %v, file=%v spanOffset=%v, spanSize=%v, cellSize=%v, D=%v", stripe, body.Stripes, fileId, spanOffset, span.Span.Header.Size, body.CellSize, D))
	}
	log.Debug("fetching stripe %v, cell size %v", stripe, body.CellSize)
	for i := 0; i < B; i++ {
		block := &body.Blocks[i]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			continue
		}
		buf, err = c.fetchCell(log, bufPool, span.BlockServices, body, uint8(i), uint8(stripe))
		if err != nil {
			continue
		}
		// we managed to get the block, store it
		blocks[i] = buf
		blocksFound++
		if blocksFound >= D {
			break
		}
	}
	if blocksFound != D {
		return 0, nil, fmt.Errorf("couldn't get enough block connections (need at least %v, got %v)", D, blocksFound)
	}

	stripeBuf := bufPool.Get(D * int(body.CellSize))

	// check if we're missing data blocks, and recover them if needed
	stripeCrc := uint32(0)
	for i := 0; i < D; i++ {
		if blocks[i] == nil {
			haveBlocksRecover := [][]byte{}
			haveBlocksRecoverIxs := []uint8{}
			for j := 0; j < B; j++ {
				if blocks[j] != nil {
					haveBlocksRecover = append(haveBlocksRecover, blocks[j].Bytes())
					haveBlocksRecoverIxs = append(haveBlocksRecoverIxs, uint8(j))
					if len(haveBlocksRecover) >= D {
						break
					}
				}
			}
			blocks[i] = bufPool.Get(int(body.CellSize))
			rs.Get(body.Parity).RecoverInto(haveBlocksRecoverIxs, haveBlocksRecover, uint8(i), blocks[i].Bytes())
		}
		stripeCrc = crc32c.Sum(stripeCrc, blocks[i].Bytes())
		copy(stripeBuf.Bytes()[i*int(body.CellSize):(i+1)*int(body.CellSize)], blocks[i].Bytes())
	}

	// check if our crc is scuppered
	if stripeCrc != uint32(body.StripesCrc[stripe]) {
		// TODO implement corrupted stripe repair
		return 0, nil, fmt.Errorf("bad crc, expected %v, got %v", msgs.Crc(stripeCrc), body.StripesCrc[stripe])
	}

	return span.Span.Header.ByteOffset + uint64(stripe)*uint64(D)*uint64(body.CellSize), stripeBuf, nil
}

func (c *Client) FetchStripeFromSpan(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	fileId msgs.InodeId,
	span *SpanWithBlockServices,
	offset uint64,
) (*FetchedStripe, error) {
	if offset < span.Span.Header.ByteOffset || offset >= span.Span.Header.ByteOffset+uint64(span.Span.Header.Size) {
		panic(fmt.Errorf("out of bounds offset %v for span going from %v to %v", offset, span.Span.Header.ByteOffset, span.Span.Header.ByteOffset+uint64(span.Span.Header.Size)))
	}
	log.DebugStack(1, "will fetch span %v -> %v", span.Span.Header.ByteOffset, span.Span.Header.ByteOffset+uint64(span.Span.Header.Size))

	// if inline, it's very easy
	if span.Span.Header.StorageClass == msgs.INLINE_STORAGE {
		data := span.Span.Body.(*msgs.FetchedInlineSpan).Body
		dataCrc := msgs.Crc(crc32c.Sum(0, data))
		if dataCrc != span.Span.Header.Crc {
			panic(fmt.Errorf("header CRC for inline span is %v, but data is %v", span.Span.Header.Crc, dataCrc))
		}
		stripe := &FetchedStripe{
			Buf:   bufpool.NewBuf(&span.Span.Body.(*msgs.FetchedInlineSpan).Body),
			Start: span.Span.Header.ByteOffset,
		}
		log.Debug("fetched inline span")
		return stripe, nil
	}

	// otherwise we need to fetch
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	D := body.Parity.DataBlocks()

	// if we're in trailing zeros, just return the trailing zeros part
	// (this is a "synthetic" stripe of sorts, but it's helpful to callers)
	spanDataEnd := span.Span.Header.ByteOffset + uint64(body.Stripes)*uint64(D*int(body.CellSize))
	if offset >= spanDataEnd {
		buf := bufPool.Get(int(uint64(span.Span.Header.Size) - spanDataEnd))
		bufSlice := buf.Bytes()
		for i := range bufSlice {
			bufSlice[i] = 0
		}
		stripe := &FetchedStripe{
			Buf:   buf,
			Start: spanDataEnd,
			owned: true,
		}
		return stripe, nil
	}

	// otherwise just fetch
	var start uint64
	var buf *bufpool.Buf
	var err error
	if D == 1 {
		start, buf, err = c.fetchMirroredStripe(log, bufPool, span, body, offset)
		if err != nil {
			return nil, err
		}
	} else {
		start, buf, err = c.fetchRsStripe(log, bufPool, fileId, span, body, offset)
		if err != nil {
			return nil, err
		}
	}

	// chop off trailing zeros in the span
	stripeEnd := start + uint64(len(buf.Bytes()))
	spanEnd := span.Span.Header.ByteOffset + uint64(span.Span.Header.Size)
	if stripeEnd > spanEnd {
		*buf.BytesPtr() = (*buf.BytesPtr())[:uint64(len(*buf.BytesPtr()))-(stripeEnd-spanEnd)]
	}

	// we're done
	stripe := &FetchedStripe{
		Buf:   buf,
		owned: true,
		Start: start,
	}
	return stripe, nil
}

func (c *Client) fetchInlineSpan(
	inlineSpan *msgs.FetchedInlineSpan,
	crc msgs.Crc,
) (*bufpool.Buf, error) {
	dataCrc := msgs.Crc(crc32c.Sum(0, inlineSpan.Body))
	if dataCrc != crc {
		return nil, fmt.Errorf("header CRC for inline span is %v, but data is %v", crc, dataCrc)
	}
	return bufpool.NewBuf(&inlineSpan.Body), nil
}

func (c *Client) fetchMirroredSpan(log *log.Logger,
	bufPool *bufpool.BufPool,
	span *SpanWithBlockServices,
) (*bufpool.Buf, error) {
	body := span.Span.Body.(*msgs.FetchedBlocksSpan)
	blockSize := uint32(body.Stripes) * uint32(body.CellSize)

	buf := bufPool.Get(int(span.Span.Header.Size))
	for i := blockSize; i < span.Span.Header.Size; i++ {
		buf.Bytes()[i] = 0
	}

	for _, block := range body.Blocks {
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			continue
		}
		data, err := c.FetchBlock(log, nil, blockService, block.BlockId, 0, blockSize, block.Crc)
		if err == nil {
			defer c.PutFetchedBlock(data)
			if copy(buf.Bytes(), data.Bytes()) != int(span.Span.Header.Size) {
				panic(fmt.Errorf("runt block"))
			}
			return buf, nil
		}
	}
	return nil, fmt.Errorf("could not find any suitable blocks")
}

func (c *Client) fetchRsSpan(log *log.Logger,
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
	blockBufs := make([]*bytes.Buffer, body.Parity.Blocks())
	defer func() {
		for _, buf := range blockBufs {
			if buf != nil {
				c.fetchBlockBufs.Put(buf)
			}
		}
	}()

scheduleMoreBlocks:
	for succeedBlocks+inFlightBlocks < dataBlocks && blockIdx < body.Parity.Blocks() {
		block := body.Blocks[blockIdx]
		blockService := &span.BlockServices[block.BlockServiceIx]
		if !blockService.Flags.CanRead() {
			blockIdx++
			continue
		}
		blockBufs[blockIdx] = c.fetchBlockBufs.Get().(*bytes.Buffer)
		blockBufs[blockIdx].Reset()
		extra := blockIdx
		if err := c.StartFetchBlock(log, blockService, block.BlockId, 0, blockSize, blockBufs[blockIdx], &extra, ch, block.Crc); err == nil {
			inFlightBlocks++
		}
		blockIdx++
	}

	for inFlightBlocks > 0 {
		result := <-ch
		inFlightBlocks--
		idx := *result.Extra.(*int)
		if result.Error != nil {
			c.PutFetchedBlock(blockBufs[idx])
			blockBufs[idx] = nil
			goto scheduleMoreBlocks
		}
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
		blockBufs[i] = c.fetchBlockBufs.Get().(*bytes.Buffer)
		blockBufs[i].Reset()
		blockBufs[i].Write(make([]byte, blockSize))
		rs.Get(body.Parity).RecoverInto(haveBlocksRecoverIxs, haveBlocksRecover, uint8(i), blockBufs[i].Bytes())
	}

	buf := bufPool.Get(int(span.Span.Header.Size))
	for i := dataSize; i < span.Span.Header.Size; i++ {
		buf.Bytes()[i] = 0
	}
	spanOffset := 0
	blockOffset := 0
	for range body.Stripes {
		for i := range dataBlocks {
			if spanOffset >= len(buf.Bytes()) {
				break
			}
			copy(buf.Bytes()[spanOffset:], blockBufs[i].Bytes()[blockOffset:blockOffset+int(body.CellSize)])
			spanOffset += int(body.CellSize)
		}
		blockOffset += int(body.CellSize)
	}

	dataCrc := msgs.Crc(crc32c.Sum(0, buf.Bytes()))
	if dataCrc != span.Span.Header.Crc {
		return nil, fmt.Errorf("header CRC for span is %v, but data is %v", span.Span.Header.Crc, dataCrc)
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

// Returns nil, nil if span or stripe cannot be found.
// Stripe might not be found because
func (c *Client) FetchStripe(
	log *log.Logger,
	bufPool *bufpool.BufPool,
	fileId msgs.InodeId,
	spans []SpanWithBlockServices,
	offset uint64,
) (*FetchedStripe, error) {
	// find span
	spanIx := sort.Search(len(spans), func(i int) bool {
		return offset < spans[i].Span.Header.ByteOffset+uint64(spans[i].Span.Header.Size)
	})
	if spanIx >= len(spans) {
		log.Debug("empty file offset=%v spanIx=%v len(spans)=%v", offset, spanIx, len(spans))
		return nil, nil // out of spans
	}
	span := &spans[spanIx]
	if offset >= (span.Span.Header.ByteOffset + uint64(span.Span.Header.Size)) {
		log.Debug("could not find span")
		return nil, nil // out of spans
	}

	return c.FetchStripeFromSpan(log, bufPool, fileId, span, offset)
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

type fileReader struct {
	client        *Client
	log           *log.Logger
	bufPool       *bufpool.BufPool
	fileId        msgs.InodeId
	fileSize      int64 // if -1, we haven't initialized this yet
	spans         []SpanWithBlockServices
	currentStripe *FetchedStripe
	cursor        uint64
}

func (f *fileReader) Close() error {
	if f.currentStripe != nil {
		f.currentStripe.Put(f.bufPool)
	}
	return nil
}

func (f *fileReader) Read(p []byte) (int, error) {
	if f.currentStripe == nil || f.cursor >= (f.currentStripe.Start+uint64(len(f.currentStripe.Buf.Bytes()))) {
		var err error
		f.currentStripe, err = f.client.FetchStripe(f.log, f.bufPool, f.fileId, f.spans, f.cursor)
		if err != nil {
			return 0, err
		}
		if f.currentStripe == nil {
			return 0, io.EOF
		}
		return f.Read(p)
	}
	r := copy(p, f.currentStripe.Buf.Bytes()[f.cursor-f.currentStripe.Start:])
	f.cursor += uint64(r)
	return r, nil
}

func (f *fileReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.cursor = uint64(offset)
	case io.SeekCurrent:
		f.cursor = uint64(int64(f.cursor) + offset)
	case io.SeekEnd:
		if f.fileSize < 0 {
			resp := msgs.StatFileResp{}
			if err := f.client.ShardRequest(f.log, f.fileId.Shard(), &msgs.StatFileReq{Id: f.fileId}, &resp); err != nil {
				return 0, err
			}
			f.fileSize = int64(resp.Size)
		}
		f.cursor = uint64(f.fileSize + offset)
	default:
		return 0, fmt.Errorf("bad whence %v", whence)
	}
	return int64(f.cursor), nil
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
		client:   c,
		log:      log,
		bufPool:  bufPool,
		fileId:   id,
		fileSize: -1,
		spans:    spans,
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
			if b != nil {
				bufPool.Put(b)
			}
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
