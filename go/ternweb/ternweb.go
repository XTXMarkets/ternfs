// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"mime"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bincode"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	lrecover "xtx/ternfs/core/recover"
	"xtx/ternfs/msgs"
	"xtx/ternfs/msgs/public"
)

type namedTemplate struct {
	name string
	body string
}

func parseTemplates(ts ...namedTemplate) (tmpl *template.Template) {
	for _, t := range ts {
		if tmpl == nil {
			tmpl = template.New(t.name)
			if _, err := tmpl.Parse(t.body); err != nil {
				panic(err)
			}
		} else {
			other := tmpl.New(t.name)
			if _, err := other.Parse(t.body); err != nil {
				panic(err)
			}
		}
	}
	return tmpl
}

type cdcState struct {
	addrs    msgs.AddrsInfo
	lastSeen msgs.TernTime
}

type locStorageClass struct {
	locationId   msgs.Location
	storageClass msgs.StorageClass
}

type state struct {
	config *registryConfig
	client *client.Client
}

type BlockServiceInfoWithLocation struct {
	msgs.BlockServiceDeprecatedInfo
	LocationId msgs.Location
}

type registryConfig struct {
	registryAddress string
	addrs           msgs.AddrsInfo
}

func newState(
	l *log.Logger,
	conf *registryConfig,
) (*state, error) {
	st := &state{
		config: conf,
	}

	var err error
	st.client, err = client.NewClient(l, nil, conf.registryAddress, conf.addrs)
	if err != nil {
		return nil, err
	}
	// Take a long time to fail requests so that we don't get spurious failures
	// on restarts
	{
		blockTimeout := client.DefaultBlockTimeout
		blockTimeout.Overall = 5 * time.Minute
		st.client.SetBlockTimeout(&blockTimeout)
		shardTimeout := client.DefaultShardTimeout
		shardTimeout.Overall = 5 * time.Minute
		st.client.SetShardTimeouts(&shardTimeout)
		cdcTimeout := client.DefaultCDCTimeout
		cdcTimeout.Overall = 5 * time.Minute
		st.client.SetCDCTimeouts(&cdcTimeout)
	}
	return st, err
}

func isBenignConnTermination(err error) bool {
	// we don't currently use timeouts here, but can't hurt
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			if sysErr.Err == syscall.EPIPE {
				return true
			}
			if sysErr.Err == syscall.ECONNRESET {
				return true
			}
		}
	}

	return false
}

func noRunawayArgs() {
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Unexpected extra arguments %v\n", flag.Args())
		os.Exit(2)
	}
}

//go:embed ternfs.png
var registryFacePngStr []byte

//go:embed bootstrap.5.0.2.min.css
var bootstrapCssStr []byte

type pageData struct {
	Title string
	Body  any
}

//go:embed base.html
var baseTemplateStr string

func renderPage(
	tmpl *template.Template,
	data *pageData,
) []byte {
	content := bytes.NewBuffer([]byte{})
	if err := tmpl.Execute(content, &data); err != nil {
		panic(err)
	}
	return content.Bytes()
}

func sendPage(
	tmpl *template.Template,
	data *pageData,
	status int,
) (io.ReadCloser, int64, int) {
	content := renderPage(tmpl, data)
	return io.NopCloser(bytes.NewReader(content)), int64(len(content)), status
}

func handleWithRecover(
	log *log.Logger,
	w http.ResponseWriter,
	r *http.Request,
	methodAllowed *string,
	// if the ReadCloser == nil, it means that
	// `handle` does not want us to handle the request.
	handle func(log *log.Logger, query url.Values) (io.ReadCloser, int64, int),
) {
	if methodAllowed == nil {
		str := http.MethodGet
		methodAllowed = &str
	}
	statusPtr := new(int)
	var content io.ReadCloser
	defer func() {
		if content != nil {
			content.Close()
		}
	}()
	sizePtr := new(int64)
	func() {
		var status int
		var size int64
		defer func() {
			if r := recover(); r != nil {
				content, size, status = sendPage(errorPage(http.StatusInternalServerError, fmt.Sprintf("PANIC: %v\n%v", r, string(debug.Stack()))))
				*statusPtr = status
				*sizePtr = size
			}
		}()
		if r.Method != *methodAllowed {
			content, size, status = sendPage(errorPage(http.StatusMethodNotAllowed, fmt.Sprintf("Only %v allowed", *methodAllowed)))
		} else {
			query, err := url.ParseQuery(r.URL.RawQuery)
			if err != nil {
				content, size, status = sendPage(errorPage(http.StatusBadRequest, "could not parse query"))
			} else {
				content, size, status = handle(log, query)
			}
		}
		*statusPtr = status
		*sizePtr = size
	}()
	if content != nil {
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(*statusPtr)
		if written, err := io.CopyN(w, content, *sizePtr); err != nil {
			if !isBenignConnTermination(err) {
				log.RaiseAlert("could not send full response of size %v, %v written: %w", *sizePtr, written, err)
			}
		}
	}
}

func handlePage(
	l *log.Logger,
	w http.ResponseWriter,
	r *http.Request,
	// third result is status code
	page func(query url.Values) (*template.Template, *pageData, int),
) {
	handleWithRecover(
		l, w, r, nil,
		func(log *log.Logger, query url.Values) (io.ReadCloser, int64, int) {
			return sendPage(page(query))
		},
	)
}

//go:embed error.html
var errorTemplateStr string

var errorTemplate *template.Template

func errorPage(status int, body string) (*template.Template, *pageData, int) {
	return errorTemplate, &pageData{Title: "Error!", Body: body}, status
}

type indexData struct {
	NumBlockServices    int
	NumFailureDomains   int
	TotalCapacity       string
	TotalUsed           string
	TotalUsedPercentage string
	Blocks              uint64
}

//go:embed index.html
var indexTemplateStr string

func formatSize(bytes uint64) string {
	bytesf := float64(bytes)
	if bytes == 0 {
		return "0"
	}
	if bytes < (uint64(1) << 20) {
		return fmt.Sprintf("%.2fKiB", bytesf/float64(uint64(1)<<10))
	}
	if bytes < (uint64(1) << 30) {
		return fmt.Sprintf("%.2fMiB", bytesf/float64(uint64(1)<<20))
	}
	if bytes < (uint64(1) << 40) {
		return fmt.Sprintf("%.2fGiB", bytesf/float64(uint64(1)<<30))
	}
	if bytes < (uint64(1) << 50) {
		return fmt.Sprintf("%.2fTiB", bytesf/float64(uint64(1)<<40))
	}
	return fmt.Sprintf("%.2fPiB", bytesf/float64(uint64(1)<<50))
}

func formatPreciseSize(bytes uint64) string {
	if bytes == 0 {
		return "0"
	}
	if bytes%(1<<30) == 0 {
		return fmt.Sprintf("%vGiB", bytes>>30)
	}
	if bytes%(1<<20) == 0 {
		return fmt.Sprintf("%vMiB", bytes>>20)
	}
	if bytes%(1<<10) == 0 {
		return fmt.Sprintf("%vKiB", bytes>>10)
	}
	return fmt.Sprintf("%vB", bytes)
}

var indexTemplate *template.Template

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
		unit = "m"
	} else {
		amount = float64(nanos) / (1e9 * 60.0 * 60.0)
		unit = "h"
	}
	return fmt.Sprintf("%7.2f%s", amount, unit)
}

func handleIndex(ll *log.Logger, state *state, w http.ResponseWriter, r *http.Request) {
	handlePage(
		ll, w, r,
		func(_ url.Values) (*template.Template, *pageData, int) {
			if r.URL.Path != "/" {
				return errorPage(http.StatusNotFound, "not found")
			}

			blockServicesResp, err := state.client.RegistryRequest(ll, &msgs.AllBlockServicesDeprecatedReq{})
			if err != nil {
				ll.RaiseAlert("error reading block services: %s", err)
				return errorPage(http.StatusInternalServerError, fmt.Sprintf("error reading block services: %s", err))
			}

			data := indexData{
				NumBlockServices: len(blockServicesResp.(*msgs.AllBlockServicesDeprecatedResp).BlockServices),
			}
			totalCapacityBytes := uint64(0)
			totalAvailableBytes := uint64(0)
			failureDomainsBytes := make(map[string]struct{})

			data.TotalUsed = formatSize(totalCapacityBytes - totalAvailableBytes)
			data.TotalCapacity = formatSize(totalAvailableBytes)
			data.NumFailureDomains = len(failureDomainsBytes)
			if totalAvailableBytes == 0 {
				data.TotalUsedPercentage = "0%"
			} else {
				data.TotalUsedPercentage = fmt.Sprintf("%0.2f%%", 100.0*(1.0-float64(totalAvailableBytes)/float64(totalCapacityBytes)))
			}
			return indexTemplate, &pageData{Title: "Registry", Body: &data}, http.StatusOK
		},
	)
}

//go:embed file.html
var fileTemplateStr string

type fileBlock struct {
	Id           string
	Crc          string
	BlockService string
	Link         string
	Hosts        string
}

type fileSpan struct {
	Offset       string
	Size         string
	Crc          string
	StorageClass string
	CellSize     string
	BodyBlocks   []fileBlock
	BodyBytes    string
	ParityBlocks int
	DataBlocks   int
	Stripes      uint8
}

type pathSegment struct {
	Segment   string
	PathSoFar string
}

type fileData struct {
	Id            string
	Path          string // might be empty
	Size          string
	Mtime         string
	Atime         string
	TransientNote string // only if transient
	DownloadLink  string
	AllInline     bool
	Spans         []fileSpan
	PathSegments  []pathSegment
	DirectoryLink string
}

//go:embed directory.html
var directoryTemplateStr string

type directoryInfoEntry struct {
	Tag           string
	Body          string
	InheritedFrom string
}

type directoryData struct {
	Id           string
	Path         string // might be empty
	PathSegments []pathSegment
	Mtime        string
	Owner        string
	Info         []directoryInfoEntry
}

func normalizePath(path string) string {
	newPath := ""
	for _, segment := range strings.Split(path, "/") {
		if segment == "" {
			continue
		}
		newPath = newPath + "/" + segment
	}
	if len(newPath) > 1 {
		newPath = newPath[1:] // strip leading /
	}
	return newPath
}

func handleStorageHostStatus(ll *log.Logger, s *state, req *public.StorageHostStatusReq) (*public.StorageHostStatusResp, error) {
	blockServicesResp, err := s.client.RegistryRequest(ll, &msgs.AllBlockServicesDeprecatedReq{})
	if err != nil {
		return nil, err
	}

	ret := public.StorageHostStatusResp{IsIdle: true, IsDrained: true}
	for _, v := range blockServicesResp.(*msgs.AllBlockServicesDeprecatedResp).BlockServices {
		if v.HasFiles {
			ret.IsDrained = false
		}
		if v.Flags&msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED != msgs.TERNFS_BLOCK_SERVICE_DECOMMISSIONED {
			ret.IsDrained = false
			// Either D or NR|NW would qualify as idled.
			// S is not considered intentional state.
			if v.Flags&msgs.TERNFS_BLOCK_SERVICE_NO_READ != msgs.TERNFS_BLOCK_SERVICE_NO_READ ||
				v.Flags&msgs.TERNFS_BLOCK_SERVICE_NO_WRITE != msgs.TERNFS_BLOCK_SERVICE_NO_WRITE {
				ret.IsIdle = false
			}
		}
	}
	return &ret, nil
}

func lookup(log *log.Logger, client *client.Client, path string) *msgs.InodeId {
	id := msgs.ROOT_DIR_INODE_ID
	if path == "" {
		return &id
	}
	segments := strings.Split(path, "/")
	for _, segment := range segments {
		req := msgs.LookupReq{
			DirId: id,
			Name:  segment,
		}
		resp := msgs.LookupResp{}
		if err := client.ShardRequest(log, id.Shard(), &req, &resp); err != nil {
			ternErr, ok := err.(msgs.TernError)
			if ok {
				if ternErr == msgs.NAME_NOT_FOUND {
					return nil
				}
			}
			panic(err)
		}
		id = resp.TargetId
	}
	return &id
}

func pathSegments(path string) []pathSegment {
	pathSegments := []pathSegment{}
	if path == "" {
		return pathSegments
	}
	segments := strings.Split(path, "/")
	pathSoFar := "/"
	for _, segment := range segments {
		if pathSoFar == "/" {
			pathSoFar = pathSoFar + segment
		} else {
			pathSoFar = pathSoFar + "/" + segment
		}
		pathSegments = append(pathSegments, pathSegment{Segment: segment, PathSoFar: pathSoFar})
	}
	return pathSegments
}

var fileTemplate *template.Template
var directoryTemplate *template.Template

func handleInode(
	log *log.Logger,
	state *state,
	w http.ResponseWriter,
	r *http.Request,
) {
	handlePage(
		log, w, r,
		func(query url.Values) (*template.Template, *pageData, int) {
			path := r.URL.Path[len("/browse"):]
			path = normalizePath(path)
			var id msgs.InodeId
			idStr := query.Get("id")
			if idStr != "" {
				i, err := strconv.ParseUint(idStr, 0, 63)
				if err != nil {
					return errorPage(http.StatusBadRequest, fmt.Sprintf("cannot parse id %v: %v", idStr, err))
				}
				id = msgs.InodeId(i)
			}
			nameStr := query.Get("name")
			if id != msgs.NULL_INODE_ID && id != msgs.ROOT_DIR_INODE_ID && path == "/" {
				path = "" // path not provided
			}
			if id != msgs.NULL_INODE_ID && id != msgs.ROOT_DIR_INODE_ID && path != "" {
				return errorPage(http.StatusBadRequest, "cannot specify both id and path")
			}
			if id == msgs.ROOT_DIR_INODE_ID && path != "/" && path != "" {
				return errorPage(http.StatusBadRequest, "bad root inode id")
			}
			c := state.client
			if id == msgs.NULL_INODE_ID {
				mbId := lookup(log, c, path)
				if mbId == nil {
					return errorPage(http.StatusNotFound, fmt.Sprintf("path '%v' not found", path))
				}
				id = *mbId
			}
			if id.Type() == msgs.DIRECTORY {
				data := directoryData{
					Id: fmt.Sprintf("%v", id),
				}
				if path != "" {
					data.Path = "/" + path + "/"
				}
				if id == msgs.ROOT_DIR_INODE_ID {
					data.Path = "/"
				}
				title := fmt.Sprintf("Directory %v", data.Id)
				{
					resp := msgs.StatDirectoryResp{}
					if err := c.ShardRequest(log, id.Shard(), &msgs.StatDirectoryReq{Id: id}, &resp); err != nil {
						panic(err)
					}
					data.Owner = resp.Owner.String()
					data.Mtime = resp.Mtime.String()
				}
				data.PathSegments = pathSegments(path)
				{
					data.Info = []directoryInfoEntry{}
					dirInfoCache := client.NewDirInfoCache()
					populateInfo := func(info msgs.IsDirectoryInfoEntry) {
						inheritedFrom, err := c.ResolveDirectoryInfoEntry(log, dirInfoCache, id, info)
						if err != nil {
							panic(err)
						}
						data.Info = append(data.Info, directoryInfoEntry{
							Tag:           info.Tag().String(),
							Body:          fmt.Sprintf("%+v", info),
							InheritedFrom: fmt.Sprintf("%v", inheritedFrom),
						})
					}
					populateInfo(&msgs.SnapshotPolicy{})
					populateInfo(&msgs.SpanPolicy{})
					populateInfo(&msgs.BlockPolicy{})
					populateInfo(&msgs.StripePolicy{})
				}
				return directoryTemplate, &pageData{Title: title, Body: &data}, http.StatusOK
			} else {
				data := fileData{
					Id:        fmt.Sprintf("%v", id),
					AllInline: true,
				}
				if len(pathSegments(path)) > 0 {
					data.PathSegments = pathSegments(path)
					data.DirectoryLink = fmt.Sprintf("/browse%s?Name=%s", filepath.Dir("/"+path), url.QueryEscape("^"+regexp.QuoteMeta(filepath.Base(path)))+"$")
					data.Path = "/" + path
				}
				title := fmt.Sprintf("File %v", data.Id)
				{
					resp := msgs.StatFileResp{}
					err := c.ShardRequest(log, id.Shard(), &msgs.StatFileReq{Id: id}, &resp)
					if err == msgs.FILE_NOT_FOUND {
						transResp := msgs.StatTransientFileResp{}
						transErr := c.ShardRequest(log, id.Shard(), &msgs.StatTransientFileReq{Id: id}, &transResp)
						if transErr == nil {
							data.Mtime = transResp.Mtime.String()
							data.Size = fmt.Sprintf("%v (%v bytes)", formatSize(transResp.Size), transResp.Size)
							data.TransientNote = transResp.Note
						} else {
							panic(fmt.Errorf("normal: %v, transient: %v", err, transErr))
						}
					} else if err == nil {
						data.Mtime = resp.Mtime.String()
						data.Atime = resp.Atime.String()
						data.Size = fmt.Sprintf("%v (%v bytes)", formatSize(resp.Size), resp.Size)
					} else {
						panic(err)
					}
				}
				data.DownloadLink = fmt.Sprintf("/files/%v", id)
				if nameStr != "" {
					data.DownloadLink = fmt.Sprintf("%s?name=%s", data.DownloadLink, nameStr)
				} else if len(data.PathSegments) > 0 {
					data.DownloadLink = fmt.Sprintf("%s?name=%s", data.DownloadLink, data.PathSegments[len(data.PathSegments)-1].Segment)
				}
				{
					req := msgs.LocalFileSpansReq{FileId: id}
					resp := msgs.LocalFileSpansResp{}
					blockServiceToHosts := make(map[msgs.BlockServiceId]string)
					for {
						if err := c.ShardRequest(log, id.Shard(), &req, &resp); err != nil {
							panic(err)
						}
						for _, span := range resp.Spans {
							fs := fileSpan{
								Offset:       formatPreciseSize(span.Header.ByteOffset),
								Size:         formatPreciseSize(uint64(span.Header.Size)),
								Crc:          span.Header.Crc.String(),
								StorageClass: span.Header.StorageClass.String(),
							}
							if span.Header.StorageClass == msgs.INLINE_STORAGE {
								fs.BodyBytes = fmt.Sprintf("%q", span.Body.(*msgs.FetchedInlineSpan).Body)
							} else {
								data.AllInline = false
								body := span.Body.(*msgs.FetchedBlocksSpan)
								fs.CellSize = formatPreciseSize(uint64(body.CellSize))
								fs.DataBlocks = body.Parity.DataBlocks()
								fs.ParityBlocks = body.Parity.ParityBlocks()
								fs.Stripes = body.Stripes
								blockSize := body.CellSize * uint32(body.Stripes)
								for _, block := range body.Blocks {
									blockService := resp.BlockServices[block.BlockServiceIx]
									hosts, found := blockServiceToHosts[blockService.Id]
									if !found {
										names1, _ := net.LookupAddr(net.IP(blockService.Addrs.Addr1.Addrs[:]).String())
										names2 := []string{}
										if blockService.Addrs.Addr2.Addrs != [4]byte{0, 0, 0, 0} {
											names2, _ = net.LookupAddr(net.IP(blockService.Addrs.Addr2.Addrs[:]).String())
										}
										hosts = strings.Join(append(names1, names2...), ",")
										blockServiceToHosts[blockService.Id] = hosts
									}
									fb := fileBlock{
										Id:           block.BlockId.String(),
										BlockService: blockService.Id.String(),
										Crc:          block.Crc.String(),
										Link:         fmt.Sprintf("/blocks/%v/%v?size=%v", blockService.Id, block.BlockId, blockSize),
										Hosts:        hosts,
									}
									fs.BodyBlocks = append(fs.BodyBlocks, fb)
								}
							}
							data.Spans = append(data.Spans, fs)
						}
						req.ByteOffset = resp.NextOffset
						if req.ByteOffset == 0 {
							break
						}
					}
				}
				return fileTemplate, &pageData{Title: title, Body: &data}, http.StatusOK
			}
		},
	)
}

func handleBlock(l *log.Logger, st *state, w http.ResponseWriter, r *http.Request) {
	handleWithRecover(
		l, w, r, nil,
		func(log *log.Logger, query url.Values) (io.ReadCloser, int64, int) {
			segments := strings.Split(r.URL.Path, "/")[1:]
			if segments[0] != "blocks" {
				panic(fmt.Errorf("bad path %v", r.URL.Path))
			}
			if len(segments) != 3 {
				return sendPage(errorPage(http.StatusBadRequest, fmt.Sprintf("Expected /blocks/<blockservice>/<blockid>, got %v", r.URL.Path)))
			}
			blockServiceIdU, err := strconv.ParseUint(segments[1], 0, 64)
			if err != nil {
				return sendPage(errorPage(http.StatusBadRequest, fmt.Sprintf("Expected /blocks/<blockservice>/<blockid>, got %v", r.URL.Path)))
			}
			blockServiceId := msgs.BlockServiceId(blockServiceIdU)
			blockIdU, err := strconv.ParseUint(segments[2], 0, 64)
			if err != nil {
				return sendPage(errorPage(http.StatusBadRequest, fmt.Sprintf("Expected /blocks/<blockservice>/<blockid>, got %v", r.URL.Path)))
			}
			blockId := msgs.BlockId(blockIdU)
			size, err := strconv.ParseUint(query.Get("size"), 0, 32)
			if err != nil {
				return sendPage(errorPage(http.StatusBadRequest, fmt.Sprintf("Bad block size '%v'", query.Get("size"))))
			}
			var blockService msgs.BlockService
			var conn *net.TCPConn
			{
				var blockService msgs.BlockServiceDeprecatedInfo
				{
					blockServicesResp, err := st.client.RegistryRequest(l, &msgs.AllBlockServicesDeprecatedReq{})
					if err != nil {
						return sendPage(errorPage(http.StatusInternalServerError, fmt.Sprintf("Failed getting block services '%v'", err)))
					}
					var found = false
					for _, bs := range blockServicesResp.(*msgs.AllBlockServicesDeprecatedResp).BlockServices {
						if bs.Id == blockServiceId {
							blockService = bs
							found = true
							break
						}
					}
					if !found {
						return sendPage(errorPage(http.StatusNotFound, fmt.Sprintf("Unknown block service id %v", blockServiceId)))
					}
				}

				conn, err = client.BlockServiceConnection(log, blockService.Addrs)
				if err != nil {
					panic(err)
				}
			}
			if err := client.FetchBlock(log, conn, &blockService, blockId, 0, uint32(size)); err != nil {
				panic(err)
			}
			w.Header().Set("Content-Type", "application/x-binary")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%016x\"", uint64(blockId)))
			log.Info("serving block of size %v", size)
			return conn, int64(size), http.StatusOK
		},
	)
}

var readSpanBufPool *bufpool.BufPool

func handleFile(l *log.Logger, st *state, w http.ResponseWriter, r *http.Request) {
	handleWithRecover(
		l, w, r, nil,
		func(log *log.Logger, query url.Values) (io.ReadCloser, int64, int) {
			segments := strings.Split(r.URL.Path, "/")[1:]
			if segments[0] != "files" {
				panic(fmt.Errorf("bad path %v", r.URL.Path))
			}
			if len(segments) != 2 {
				return sendPage(errorPage(http.StatusBadRequest, fmt.Sprintf("Expected /files/<fileid>, got %v", r.URL.Path)))
			}
			fileIdU, err := strconv.ParseUint(segments[1], 0, 64)
			if err != nil {
				return sendPage(errorPage(http.StatusBadRequest, fmt.Sprintf("Expected /blocks/<fileid>, got %v", r.URL.Path)))
			}
			fileId := msgs.InodeId(fileIdU)
			fname := query.Get("name")
			mimeType := mime.TypeByExtension(path.Ext(fname))
			if fname == "" {
				fname = fileId.String()
			}
			if mimeType == "" {
				mimeType = "application/x-binary"
			}
			client := st.client
			statResp := msgs.StatFileResp{}
			err = client.ShardRequest(log, fileId.Shard(), &msgs.StatFileReq{Id: fileId}, &statResp)
			if err == msgs.FILE_NOT_FOUND {
				return sendPage(errorPage(http.StatusNotFound, fmt.Sprintf("could not find file %v", fileId)))
			}
			if err != nil {
				panic(err)
			}

			r, err := client.ReadFile(log, readSpanBufPool, fileId)
			if err != nil {
				panic(err)
			}

			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fname))

			return r, int64(statResp.Size), http.StatusOK
		},
	)
}

//go:embed transient.html
var transientTemplateStr string

var transientTemplate *template.Template

func handleTransient(log *log.Logger, st *state, w http.ResponseWriter, r *http.Request) {
	handlePage(
		log, w, r,
		func(query url.Values) (*template.Template, *pageData, int) {
			pd := pageData{
				Title: "transient",
				Body:  nil,
			}
			return transientTemplate, &pd, http.StatusOK
		},
	)
}

func sendJson(w http.ResponseWriter, out []byte) (io.ReadCloser, int64, int) {
	w.Header().Set("Content-Type", "application/json")
	return ioutil.NopCloser(bytes.NewReader(out)), int64(len(out)), http.StatusOK
}

func sendJsonErr(w http.ResponseWriter, err any, status int) (io.ReadCloser, int64, int) {
	w.Header().Set("Content-Type", "application/json")
	j := map[string]any{"err": fmt.Sprintf("%v", err)}
	ternErr, ok := err.(msgs.TernError)
	if ok {
		j["errCode"] = uint8(ternErr)
	}
	out, err := json.Marshal(j)
	w.Header().Set("Content-Type", "application/json")
	return ioutil.NopCloser(bytes.NewReader(out)), int64(len(out)), status
}

func handleApi(l *log.Logger, st *state, w http.ResponseWriter, r *http.Request) {
	methodAllowed := http.MethodPost
	handleWithRecover(
		l, w, r, &methodAllowed,
		func(log *log.Logger, query url.Values) (io.ReadCloser, int64, int) {
			segments := strings.Split(r.URL.Path, "/")[1:]
			if segments[0] != "api" {
				panic(fmt.Errorf("bad path %v", r.URL.Path))
			}
			badUrl := func(what string) (io.ReadCloser, int64, int) {
				return sendJsonErr(w, fmt.Errorf("expected /api/(registry|cdc|shard)/<req>, got %v (%v)", r.URL.Path, what), http.StatusBadRequest)
			}
			sendResponse := func(resp bincode.Packable) (io.ReadCloser, int64, int) {
				accept := r.Header.Get("Accept")
				if accept == "application/octet-stream" {
					buf := bytes.NewBuffer([]byte{})
					if err := resp.Pack(buf); err != nil {
						panic(err)
					}
					w.Header().Set("Content-Type", "application/octet-stream")
					out := buf.Bytes()
					return ioutil.NopCloser(bytes.NewReader(out)), int64(len(out)), http.StatusOK
				} else {
					out, err := json.Marshal(map[string]any{"resp": resp})
					if err != nil {
						return sendJsonErr(w, fmt.Errorf("could not marshal request: %v", err), http.StatusInternalServerError)
					}
					return sendJson(w, out)
				}
			}
			switch segments[1] {
			case "registry":
				if len(segments) != 3 {
					return badUrl("bad segments len")
				}
				req, _, err := msgs.MkRegistryMessage(segments[2])
				if err != nil {
					return badUrl("bad ")
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					return sendJsonErr(w, fmt.Errorf("could not decode request: %v", err), http.StatusBadRequest)
				}

				resp, err := st.client.RegistryRequest(l, req)
				if err != nil {
					return sendJsonErr(w, err, http.StatusInternalServerError)
				}
				return sendResponse(resp)
			case "cdc":
				if len(segments) != 3 {
					return badUrl("bad segments len")
				}
				req, resp, err := msgs.MkCDCMessage(segments[2])
				if err != nil {
					return badUrl("bad req type")
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					return sendJsonErr(w, fmt.Errorf("could not decode request: %v", err), http.StatusBadRequest)
				}
				client := st.client
				if err := client.CDCRequest(log, req, resp); err != nil {
					return sendJsonErr(w, err, http.StatusInternalServerError)
				}
				return sendResponse(resp)
			case "shard":
				if len(segments) != 4 {
					return badUrl("bad segments len")
				}
				req, resp, err := msgs.MkShardMessage(segments[3])
				if err != nil {
					return badUrl("bad req type")
				}
				shidU, err := strconv.ParseUint(segments[2], 10, 8)
				if err != nil {
					return badUrl("bad shard id")
				}
				shid := msgs.ShardId(shidU)
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					return sendJsonErr(w, fmt.Errorf("could not decode request: %v", err), http.StatusBadRequest)
				}
				client := st.client
				if err := client.ShardRequest(log, shid, req, resp); err != nil {
					return sendJsonErr(w, err, http.StatusInternalServerError)
				}
				return sendResponse(resp)
			default:
				return badUrl("bad api type")
			}
		},
	)
}

func handlePublicRequestParsed(log *log.Logger, s *state, req any) (any, error) {
	log.Debug("handling request %T", req)
	log.Trace("request body %+v", req)
	var err error
	var resp any
	switch whichReq := req.(type) {
	case *public.StorageHostStatusReq:
		resp, err = handleStorageHostStatus(log, s, whichReq)
	default:
		err = fmt.Errorf("bad req type %T", req)
	}
	return resp, err
}

func handlePublic(l *log.Logger, st *state, w http.ResponseWriter, r *http.Request) {
	methodAllowed := http.MethodPost
	handleWithRecover(
		l, w, r, &methodAllowed,
		func(log *log.Logger, query url.Values) (io.ReadCloser, int64, int) {
			segments := strings.Split(r.URL.Path, "/")[1:]
			if segments[0] != "public" {
				panic(fmt.Errorf("bad path %v", r.URL.Path))
			}
			badUrl := func(what string) (io.ReadCloser, int64, int) {
				return sendJsonErr(w, fmt.Errorf("expected /public/<req>, got %v (%v)", r.URL.Path, what), http.StatusBadRequest)
			}
			sendResponse := func(resp any) (io.ReadCloser, int64, int) {

				out, err := json.Marshal(resp)
				if err != nil {
					return sendJsonErr(w, fmt.Errorf("could not marshal request: %v", err), http.StatusInternalServerError)
				}
				return sendJson(w, out)
			}
			switch segments[1] {
			case "registry":
				if len(segments) != 3 {
					return badUrl("bad segments len")
				}
				req, _, err := public.MkPublicMessage(segments[2])
				if err != nil {
					return badUrl("bad ")
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					return sendJsonErr(w, fmt.Errorf("could not decode request: %v", err), http.StatusBadRequest)
				}
				resp, err := handlePublicRequestParsed(log, st, req)
				if err != nil {
					return sendJsonErr(w, err, http.StatusInternalServerError)
				}
				return sendResponse(resp)
			default:
				return badUrl("bad api type")
			}
		},
	)
}

//go:embed preact-10.18.1.module.js
var preactJsStr []byte

//go:embed preact-hooks-10.18.1.module.js
var preactHooksJsStr []byte

//go:embed scripts.js
var scriptsJs []byte

func setupRouting(l *log.Logger, st *state, scriptsJsFile string) {
	errorTemplate = parseTemplates(
		namedTemplate{name: "base", body: baseTemplateStr},
		namedTemplate{name: "error", body: errorTemplateStr},
	)

	setupPage := func(path string, handle func(l *log.Logger, state *state, w http.ResponseWriter, r *http.Request)) {
		http.HandleFunc(
			path,
			func(w http.ResponseWriter, r *http.Request) { handle(l, st, w, r) },
		)
	}

	// Static assets
	http.HandleFunc(
		"/static/bootstrap.5.0.2.min.css",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			w.Header().Set("Cache-Control", "max-age=31536000")
			w.Write(bootstrapCssStr)
		},
	)
	http.HandleFunc(
		"/static/registry-face.png",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "max-age=31536000")
			w.Write(registryFacePngStr)
		},
	)
	http.HandleFunc(
		"/static/preact-10.18.1.module.js",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Header().Set("Cache-Control", "max-age=31536000")
			w.Write(preactJsStr)
		},
	)
	http.HandleFunc(
		"/static/preact-hooks-10.18.1.module.js",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Header().Set("Cache-Control", "max-age=31536000")
			w.Write(preactHooksJsStr)
		},
	)

	http.HandleFunc(
		"/static/scripts.js",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/javascript")
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			if scriptsJsFile == "" {
				w.Write(scriptsJs)
			} else {
				file, err := os.Open(scriptsJsFile)
				if err != nil {
					panic(err)
				}
				defer file.Close()
				if _, err := io.Copy(w, file); err != nil {
					panic(err)
				}
			}
		},
	)

	// blocks serving
	http.HandleFunc(
		"/blocks/",
		func(w http.ResponseWriter, r *http.Request) { handleBlock(l, st, w, r) },
	)

	// file serving
	http.HandleFunc(
		"/files/",
		func(w http.ResponseWriter, r *http.Request) { handleFile(l, st, w, r) },
	)

	http.HandleFunc(
		"/api/",
		func(w http.ResponseWriter, r *http.Request) { handleApi(l, st, w, r) },
	)

	http.HandleFunc(
		"/public/",
		func(w http.ResponseWriter, r *http.Request) { handlePublic(l, st, w, r) },
	)

	// pages
	indexTemplate = parseTemplates(
		namedTemplate{name: "base", body: baseTemplateStr},
		namedTemplate{name: "registry", body: indexTemplateStr},
	)
	setupPage("/", handleIndex)

	fileTemplate = parseTemplates(
		namedTemplate{name: "base", body: baseTemplateStr},
		namedTemplate{name: "file", body: fileTemplateStr},
	)
	directoryTemplate = parseTemplates(
		namedTemplate{name: "base", body: baseTemplateStr},
		namedTemplate{name: "directory", body: directoryTemplateStr},
	)
	setupPage("/browse/", handleInode)

	transientTemplate = parseTemplates(
		namedTemplate{name: "base", body: baseTemplateStr},
		namedTemplate{name: "transient", body: transientTemplateStr},
	)
	setupPage("/transient", handleTransient)
}

func main() {
	httpPort := flag.Uint("http-port", 10000, "Port on which to run the HTTP server")
	logFile := flag.String("log-file", "", "File in which to write logs (or stdout)")
	verbose := flag.Bool("verbose", false, "")
	trace := flag.Bool("trace", false, "")
	xmon := flag.String("xmon", "", "Xmon address (empty for no xmon)")
	syslog := flag.Bool("syslog", false, "")
	mtu := flag.Uint64("mtu", 0, "")
	registry := flag.String("registry", "", "registry address host:port")
	scriptsJs := flag.String("scripts-js", "", "")

	flag.Parse()
	noRunawayArgs()

	if *registry == "" {
		fmt.Fprintf(os.Stderr, "-registry needs to be provided\n")
		os.Exit(2)
	}

	logOut := os.Stdout
	if *logFile != "" {
		var err error
		logOut, err = os.OpenFile(*logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Errorf("could not open log file %v: %v\n", *logFile, err)
			os.Exit(2)
		}
	}

	level := log.INFO
	if *verbose {
		level = log.DEBUG
	}
	if *trace {
		level = log.TRACE
	}
	l := log.NewLogger(logOut, &log.LoggerOptions{Level: level, Syslog: *syslog, XmonAddr: *xmon, AppInstance: "ternweb", AppType: "restech_eggsfs.daytime"})

	l.Info("Running registry with options:")
	l.Info("  httpPort = %v", *httpPort)
	l.Info("  logFile = '%v'", *logFile)
	l.Info("  logLevel = %v", level)
	l.Info("  mtu = %v", *mtu)

	if *mtu != 0 {
		client.SetMTU(*mtu)
	}

	readSpanBufPool = bufpool.NewBufPool()

	httpListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", *httpPort))
	if err != nil {
		panic(err)
	}
	defer httpListener.Close()

	config := &registryConfig{
		addrs:           msgs.AddrsInfo{},
		registryAddress: *registry,
	}
	state, err := newState(l, config)
	if err != nil {
		panic(err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP, syscall.SIGABRT, syscall.SIGSTKFLT, syscall.SIGSYS)
	go func() {
		sig := <-signalChan
		signal.Stop(signalChan)
		syscall.Kill(syscall.Getpid(), sig.(syscall.Signal))
	}()

	setupRouting(l, state, *scriptsJs)

	terminateChan := make(chan any)

	go func() {
		defer func() { lrecover.HandleRecoverChan(l, terminateChan, recover()) }()
		terminateChan <- http.Serve(httpListener, nil)
	}()

	panic(<-terminateChan)
}
