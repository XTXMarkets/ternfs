// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"xtx/ternfs/client"
	"xtx/ternfs/core/bufpool"
	"xtx/ternfs/core/log"
	"xtx/ternfs/msgs"

	"golang.org/x/sync/errgroup"
)

// S3Error represents an error response compliant with the S3 API.
type S3Error struct {
	XMLName    xml.Name `xml:"Error"`
	Code       string   `xml:"Code"`
	Message    string   `xml:"Message"`
	Resource   string   `xml:"Resource"`
	BucketName string   `xml:"BucketName,omitempty"`
	StatusCode int      `xml:"-"` // The HTTP status code to return.
}

func (e *S3Error) Error() string {
	return e.Message
}

// --- S3 XML Response Structures ---

type CommonPrefix struct {
	Prefix string `xml:"Prefix"`
}
type ListBucketResultV2 struct {
	XMLName               xml.Name       `xml:"ListBucketResult"`
	Name                  string         `xml:"Name"`
	Prefix                string         `xml:"Prefix"`
	MaxKeys               int            `xml:"MaxKeys"`
	Delimiter             string         `xml:"Delimiter,omitempty"`
	Contents              []Object       `xml:"Contents"`
	CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
	IsTruncated           bool           `xml:"IsTruncated"`
	KeyCount              int            `xml:"KeyCount"`
	ContinuationToken     string         `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string         `xml:"NextContinuationToken,omitempty"`
}
type Object struct {
	Key          string    `xml:"Key"`
	LastModified time.Time `xml:"LastModified"`
	ETag         string    `xml:"ETag"`
	Size         int64     `xml:"Size"`
}
type CreateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}
type CompleteMultipartUploadResult struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}
type DeleteResult struct {
	XMLName xml.Name        `xml:"DeleteResult"`
	Deleted []DeletedObject `xml:"Deleted,omitempty"`
	Error   []ErrorObject   `xml:"Error,omitempty"`
}
type DeletedObject struct {
	Key string `xml:"Key"`
}
type ErrorObject struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// --- S3 XML Request Structures ---
type CompleteMultipartUpload struct {
	XMLName xml.Name     `xml:"CompleteMultipartUpload"`
	Parts   []UploadPart `xml:"Part"`
}
type UploadPart struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`
}
type Delete struct {
	XMLName xml.Name         `xml:"Delete"`
	Objects []ObjectToDelete `xml:"Object"`
	Quiet   bool             `xml:"Quiet"`
}
type ObjectToDelete struct {
	Key string `xml:"Key"`
}

type objectPath struct {
	segments    []string
	isDirectory bool
}

func (p *objectPath) String() string {
	s := "/" + strings.Join(p.segments, "/")
	if p.isDirectory && len(p.segments) > 0 {
		s += "/"
	}
	return s
}

func (p *objectPath) basename() string {
	if len(p.segments) == 0 {
		return ""
	}
	return p.segments[len(p.segments)-1]
}

func (p *objectPath) dir() *objectPath {
	d := &objectPath{
		segments:    []string{},
		isDirectory: true,
	}
	if len(p.segments) > 1 {
		d.segments = p.segments[:len(p.segments)-1]
	}
	return d
}

func parseObjectPath(str string) *objectPath {
	p := &objectPath{
		segments:    []string{},
		isDirectory: len(str) > 0 && str[len(str)-1] == '/',
	}
	// Trim leading/trailing slashes for splitting
	for _, segment := range strings.Split(str, "/") {
		if len(segment) > 0 {
			p.segments = append(p.segments, segment)
		}
	}
	return p
}

// --- Handler Implementation ---
type S3Server struct {
	log          *log.Logger
	c            *client.Client
	bufPool      *bufpool.BufPool
	dirInfoCache *client.DirInfoCache
	bucketPaths  map[string]string
	virtualHost  string
}

func NewS3Server(
	log *log.Logger,
	client *client.Client,
	bufPool *bufpool.BufPool,
	dirInfoCache *client.DirInfoCache,
	buckets map[string]string, // mapping from bucket to paths
	virtualHost string, // if not present, path-style bucket resolution will be used
) *S3Server {
	return &S3Server{
		log:          log,
		c:            client,
		bufPool:      bufPool,
		dirInfoCache: dirInfoCache,
		bucketPaths:  buckets,
		virtualHost:  virtualHost,
	}
}

// s3Handler defines the signature for all S3 operation handlers.
type s3Handler func(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error

var stacktraceLock sync.Mutex

// recoverPanics is a middleware that recovers from panics in a handler,
// logs the stack trace, and returns an error.
func (s *S3Server) recoverPanics(handler s3Handler) s3Handler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) (err error) {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("internal server panic: %v", rec)
				s.log.RaiseAlert(fmt.Sprintf("caught stray error: %v", rec))
				stacktraceLock.Lock()
				fmt.Fprintf(os.Stderr, "PANIC %v. Stacktrace:\n", rec)
				for _, line := range strings.Split(string(debug.Stack()), "\n") {
					fmt.Fprintln(os.Stderr, line)
				}
				stacktraceLock.Unlock()
			}
		}()
		err = handler(ctx, w, r, bucket, rootId, path)
		return
	}
}

// FileEtag generates a consistent ETag for a given file ID.
func FileEtag(id msgs.InodeId) string {
	return fmt.Sprintf(`"%x"`, uint64(id))
}

// parseRange parses the HTTP Range header.
func parseRange(rangeHeader string, totalSize int64) (start, length int64, err error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, errors.New("invalid range header format")
	}
	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid range specification")
	}
	startStr, endStr := parts[0], parts[1]
	if startStr == "" && endStr == "" {
		return 0, 0, errors.New("invalid range: both start and end are empty")
	}
	if startStr != "" {
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 {
			return 0, 0, errors.New("invalid start range")
		}
	}
	var end int64
	if endStr != "" {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < 0 {
			return 0, 0, errors.New("invalid end range")
		}
	} else {
		end = totalSize - 1
	}
	if startStr == "" {
		// This is a suffix range, e.g., "bytes=-500"
		suffixLength, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid suffix range")
		}
		start = totalSize - suffixLength
		length = suffixLength
	} else {
		length = end - start + 1
	}
	if start > end || start >= totalSize || length <= 0 {
		return 0, 0, errors.New("range not satisfiable")
	}
	return start, length, nil
}

// extractBucketAndPath determines the bucket and object path from the request.
// It supports both virtual-host and path-style addressing.
func (s *S3Server) extractBucketAndPath(r *http.Request) (bucketName string, path *objectPath, err error) {
	fullPath := parseObjectPath(r.URL.Path)

	// Virtual host-style routing
	if s.virtualHost != "" {
		host := r.Host
		suffix := "." + s.virtualHost
		if strings.HasSuffix(host, suffix) {
			bucketName = strings.TrimSuffix(host, suffix)
			path = fullPath
			return bucketName, path, nil
		}
		// If virtual host is configured, we only accept virtual-host style requests.
		return "", nil, &S3Error{
			Code:       "InvalidRequest",
			Message:    "Request does not comply with virtual host setup. When virtual host routing is configured, all requests must use virtual host-style addressing.",
			StatusCode: http.StatusBadRequest,
		}
	}

	// Path-style routing
	if len(fullPath.segments) == 0 {
		// For bucket/object operations, a bucket is always required in the path.
		// This handles the case of a request to the root '/' which might be for
		// service-level operations not implemented here.
		return "", nil, &S3Error{
			Code:       "InvalidRequest",
			Message:    "Request URI does not contain a bucket name.",
			StatusCode: http.StatusBadRequest,
		}
	}

	bucketName = fullPath.segments[0]
	path = &objectPath{
		segments:    fullPath.segments[1:],
		isDirectory: fullPath.isDirectory,
	}
	return bucketName, path, nil
}

// ServeHTTP is the main entry point for all S3 requests. It routes requests
// to the appropriate handler and centralizes error handling.
func (s *S3Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var handler s3Handler
	var handlerErr error
	ctx := r.Context()

	bucketName, path, err := s.extractBucketAndPath(r)
	if err != nil {
		s.log.Info("Failed to extract bucket and path: %v", err)
		var s3Err *S3Error
		if errors.As(err, &s3Err) {
			if s3Err.Resource == "" {
				s3Err.Resource = r.URL.Path
			}
			s.writeXML(w, s3Err.StatusCode, s3Err)
		} else {
			s.writeXML(w, http.StatusInternalServerError, &S3Error{Code: "InternalError", Message: err.Error(), Resource: r.URL.Path})
		}
		return
	}

	s.log.Debug("Received request: Method=%q Host=%q Path=%q Query=%v -> Bucket=%q Key=%q", r.Method, r.Host, r.URL.Path, r.URL.Query(), bucketName, path.String())

	// Look up the bucket's root path string.
	rootPath, ok := s.bucketPaths[bucketName]
	if !ok {
		s.writeXML(w, http.StatusNotFound, &S3Error{Code: "NoSuchBucket", Message: "The specified bucket does not exist", Resource: r.URL.Path, BucketName: bucketName, StatusCode: http.StatusNotFound})
		return
	}

	// Resolve the path to an InodeId on every request.
	rootId, err := lookupPath(ctx, s.log, s.c, parseObjectPath(rootPath))
	if err != nil {
		s.log.RaiseAlert("Could not resolve root path %q for configured bucket %q: %v", rootPath, bucketName, err)
		s.writeXML(w, http.StatusInternalServerError, &S3Error{
			Code:       "InternalError",
			Message:    "The bucket's underlying storage is unavailable.",
			Resource:   r.URL.Path,
			BucketName: bucketName,
		})
		return
	}

	query := r.URL.Query()

	// Route the request to the correct handler.
	switch {
	case query.Has("uploads"):
		handler = s.handleCreateMultipartUpload
	case query.Has("uploadId") && query.Has("partNumber"):
		handler = s.handleUploadPart
	case query.Has("uploadId"):
		handler = s.handleCompleteMultipartUpload
	case query.Has("attributes"):
		handler = s.handleGetObjectAttributes
	case query.Has("delete"):
		handler = s.handleDeleteObjects
	case query.Has("list-type") && query.Get("list-type") == "2":
		handler = s.handleListObjectsV2
	default:
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			handler = s.handleGetObject
		case http.MethodPut:
			handler = s.handlePutObject
		case http.MethodDelete:
			handler = s.handleDeleteObject
		}
	}

	if handler != nil {
		// Wrap the handler to recover from panics and call it.
		handlerErr = s.recoverPanics(handler)(ctx, w, r, bucketName, rootId, path)
	} else {
		handlerErr = &S3Error{Code: "MethodNotAllowed", Message: "The specified method is not allowed against this resource.", StatusCode: http.StatusMethodNotAllowed}
	}

	// Centralized error handling.
	if handlerErr != nil {
		s.log.Debug("Request failed: %v", handlerErr)
		var s3Err *S3Error
		if errors.As(handlerErr, &s3Err) {
			if s3Err.Resource == "" {
				s3Err.Resource = r.URL.Path
			}
			s3Err.BucketName = bucketName
			s.writeXML(w, s3Err.StatusCode, s3Err)
		} else {
			// Generic internal error for any other unhandled error type.
			s.writeXML(w, http.StatusInternalServerError, &S3Error{
				Code:       "InternalError",
				Message:    handlerErr.Error(),
				Resource:   r.URL.Path,
				BucketName: bucketName,
			})
		}
	}
}

// writeXML marshals data to XML and writes it to the response.
func (s *S3Server) writeXML(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := xml.NewEncoder(w).Encode(data); err != nil {
		s.log.Info("Error writing XML response: %v", err)
	}
}

// lookupDir traverses a path to find the final directory's InodeId.
func (s *S3Server) lookupDir(ctx context.Context, rootId msgs.InodeId, path *objectPath) (msgs.InodeId, error) {
	s.log.Debug("lookupDir=%q in root %d", path.String(), rootId)
	current := rootId

	for _, segment := range path.segments {
		lookupResp := &msgs.LookupResp{}
		req := &msgs.LookupReq{DirId: current, Name: segment}
		if err := s.c.ShardRequest(s.log, current.Shard(), req, lookupResp); err != nil {
			if errors.Is(err, msgs.NAME_NOT_FOUND) {
				return 0, &S3Error{Code: "NoSuchKey", Message: "The specified key does not exist.", StatusCode: http.StatusNotFound}
			}
			return 0, fmt.Errorf("lookup failed for segment '%s': %w", segment, err)
		}
		if lookupResp.TargetId.Type() != msgs.DIRECTORY {
			return 0, &S3Error{Code: "InvalidRequest", Message: fmt.Sprintf("A component of the path is not a directory: %s", segment), StatusCode: http.StatusBadRequest}
		}
		current = lookupResp.TargetId
	}
	return current, nil
}

func (s *S3Server) lookupDirEntry(ctx context.Context, rootId msgs.InodeId, path *objectPath) (dirId msgs.InodeId, lookupResp *msgs.LookupResp, err error) {
	if len(path.segments) == 0 {
		return msgs.NULL_INODE_ID, &msgs.LookupResp{TargetId: rootId}, nil
	}

	dirId, err = s.lookupDir(ctx, rootId, path.dir())
	if err != nil {
		return 0, nil, err // Pass through S3Error from lookupDir
	}

	lookupResp = &msgs.LookupResp{}
	req := &msgs.LookupReq{DirId: dirId, Name: path.basename()}
	if err := s.c.ShardRequest(s.log, dirId.Shard(), req, lookupResp); err != nil {
		if errors.Is(err, msgs.NAME_NOT_FOUND) {
			return 0, nil, &S3Error{Code: "NoSuchKey", Message: "The specified key does not exist.", StatusCode: http.StatusNotFound}
		}
		return 0, nil, fmt.Errorf("dir entry lookup for %q failed: %w", path.String(), err)
	}
	return dirId, lookupResp, nil
}

func (s *S3Server) handleGetObject(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	s.log.Debug("GetObject: bucket=%s path=%s", bucket, path)
	_, dentry, err := s.lookupDirEntry(ctx, rootId, path)
	if err != nil {
		return err // Returns S3Error or internal error
	}

	if dentry.TargetId.Type() == msgs.DIRECTORY && !path.isDirectory {
		return &S3Error{Code: "NoSuchKey", Message: "The specified key does not exist.", StatusCode: http.StatusNotFound}
	}

	var lastModified msgs.TernTime
	var size uint64
	var bodyReader io.ReadSeeker

	if dentry.TargetId.Type() == msgs.DIRECTORY {
		statResp := &msgs.StatDirectoryResp{}
		if err := s.c.ShardRequest(s.log, dentry.TargetId.Shard(), &msgs.StatDirectoryReq{Id: dentry.TargetId}, statResp); err != nil {
			return fmt.Errorf("stat failed for directory %q: %w", path.String(), err)
		}
		lastModified = statResp.Mtime
		bodyReader = bytes.NewReader([]byte{})
	} else {
		statResp := &msgs.StatFileResp{}
		if err := s.c.ShardRequest(s.log, dentry.TargetId.Shard(), &msgs.StatFileReq{Id: dentry.TargetId}, statResp); err != nil {
			return fmt.Errorf("stat failed for file %q: %w", path.String(), err)
		}
		fileReader, err := s.c.ReadFile(s.log, s.bufPool, dentry.TargetId)
		if err != nil {
			return fmt.Errorf("failed to read contents of file id=%v: %w", dentry.TargetId, err)
		}
		defer fileReader.Close()
		bodyReader = fileReader
		size = statResp.Size
		lastModified = statResp.Mtime
	}

	rangeHeader := r.Header.Get("Range")
	w.Header().Set("ETag", FileEtag(dentry.TargetId))
	w.Header().Set("Last-Modified", lastModified.Time().Format(http.TimeFormat))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", "application/octet-stream")

	if rangeHeader == "" {
		w.Header().Set("Content-Length", strconv.FormatInt(int64(size), 10))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			if _, err := io.Copy(w, bodyReader); err != nil {
				s.log.Info("GetObject: Error writing full response body for object %s: %v", path, err)
			}
		}
		return nil
	}

	start, length, err := parseRange(rangeHeader, int64(size))
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return &S3Error{Code: "InvalidRange", Message: "The requested range is not satisfiable", StatusCode: http.StatusRequestedRangeNotSatisfiable}
	}
	if _, err := bodyReader.Seek(start, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek object: %w", err)
	}

	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, start+length-1, size))
	w.WriteHeader(http.StatusPartialContent)
	if r.Method != http.MethodHead {
		if _, err := io.CopyN(w, bodyReader, length); err != nil {
			s.log.Info("GetObject: Error writing partial response body for object %s: %v", path, err)
		}
	}
	return nil
}

func (s *S3Server) handleGetObjectAttributes(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	if r.Method != http.MethodHead && r.Method != http.MethodGet {
		return &S3Error{Code: "MethodNotAllowed", Message: "This resource only supports GET or HEAD.", StatusCode: http.StatusMethodNotAllowed}
	}
	s.log.Debug("GetObjectAttributes: path=%s", path)

	_, dentry, err := s.lookupDirEntry(ctx, rootId, path)
	if err != nil {
		return err
	}

	inode := dentry.TargetId
	var size uint64
	var lastModified msgs.TernTime

	if inode.Type() == msgs.DIRECTORY {
		statResp := &msgs.StatDirectoryResp{}
		if err := s.c.ShardRequest(s.log, dentry.TargetId.Shard(), &msgs.StatDirectoryReq{Id: dentry.TargetId}, statResp); err != nil {
			return fmt.Errorf("stat failed for directory %q: %w", path.String(), err)
		}
		lastModified = statResp.Mtime
	} else {
		statResp := &msgs.StatFileResp{}
		if err := s.c.ShardRequest(s.log, dentry.TargetId.Shard(), &msgs.StatFileReq{Id: dentry.TargetId}, statResp); err != nil {
			return fmt.Errorf("stat failed for file %q: %w", path.String(), err)
		}
		size = statResp.Size
		lastModified = statResp.Mtime
	}

	w.Header().Set("x-amz-object-attributes-etag", FileEtag(dentry.TargetId))
	w.Header().Set("x-amz-object-attributes-last-modified", lastModified.Time().UTC().Format(http.TimeFormat))
	w.Header().Set("x-amz-object-attributes-object-size", strconv.FormatInt(int64(size), 10))
	w.Header().Set("x-amz-storage-class", "STANDARD")
	w.WriteHeader(http.StatusOK)
	return nil
}

func (s *S3Server) handleListObjectsV2(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	if r.Method != http.MethodGet {
		return &S3Error{Code: "MethodNotAllowed", Message: "This resource only supports GET.", StatusCode: http.StatusMethodNotAllowed}
	}

	query := r.URL.Query()
	prefix := parseObjectPath(query.Get("prefix"))
	prefixStr := prefix.String()
	if !prefix.isDirectory {
		return &S3Error{Code: "InvalidArgument", Message: "Only prefixes terminating with / (directories) are allowed", StatusCode: http.StatusBadRequest}
	}
	delimiter := query.Get("delimiter")
	if delimiter == "" {
		delimiter = "/"
	}
	if delimiter != "/" {
		return &S3Error{Code: "InvalidArgument", Message: "Delimiter must be empty or '/'", StatusCode: http.StatusBadRequest}
	}
	maxKeys, _ := strconv.Atoi(query.Get("max-keys"))
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	startHash, _ := strconv.ParseUint(query.Get("continuation-token"), 10, 64)

	s.log.Debug("ListObjectsV2: bucket=%q prefix=%q", bucket, prefix.String())

	dirId, err := s.lookupDir(ctx, rootId, prefix)
	if err != nil {
		// If the prefix itself doesn't exist, return an empty list.
		var s3Err *S3Error
		if errors.As(err, &s3Err) && s3Err.Code == "NoSuchKey" {
			response := ListBucketResultV2{
				Name:      bucket,
				Prefix:    prefix.String(),
				Delimiter: delimiter,
				MaxKeys:   maxKeys,
				KeyCount:  0,
			}
			s.writeXML(w, http.StatusOK, response)
			return nil
		}
		return err
	}

	readDirResp := &msgs.ReadDirResp{}
	req := &msgs.ReadDirReq{DirId: dirId, StartHash: msgs.NameHash(startHash)}
	if err := s.c.ShardRequest(s.log, dirId.Shard(), req, readDirResp); err != nil {
		return fmt.Errorf("could not read directory %q: %w", prefix, err)
	}

	response := ListBucketResultV2{
		Name:              bucket,
		Prefix:            prefix.String(),
		Delimiter:         delimiter,
		MaxKeys:           maxKeys,
		Contents:          []Object{},
		CommonPrefixes:    []CommonPrefix{},
		ContinuationToken: query.Get("continuation-token"),
	}

	var filesToProcess []msgs.CurrentEdge
	for i := range readDirResp.Results {
		if len(response.Contents)+len(response.CommonPrefixes) >= maxKeys {
			response.IsTruncated = true
			response.NextContinuationToken = fmt.Sprintf("%d", uint64(readDirResp.Results[i].NameHash))
			break
		}
		entry := &readDirResp.Results[i]
		if entry.TargetId.Type() == msgs.DIRECTORY {
			response.CommonPrefixes = append(response.CommonPrefixes, CommonPrefix{Prefix: prefixStr + entry.Name + "/"})
		} else {
			filesToProcess = append(filesToProcess, *entry)
		}
	}

	if len(filesToProcess) > 0 {
		g, _ := errgroup.WithContext(ctx)
		response.Contents = make([]Object, len(filesToProcess))
		for i, entry := range filesToProcess {
			i, entry := i, entry // capture loop variables
			g.Go(func() error {
				s.log.Debug("Spawning request for details of object %s", entry.Name)
				resp := &msgs.StatFileResp{}
				if err := s.c.ShardRequest(s.log, entry.TargetId.Shard(), &msgs.StatFileReq{Id: entry.TargetId}, resp); err != nil {
					return err
				}
				response.Contents[i] = Object{
					Key:          prefixStr + entry.Name,
					ETag:         FileEtag(entry.TargetId),
					Size:         int64(resp.Size),
					LastModified: resp.Mtime.Time(),
				}
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return err
		}
	}

	if !response.IsTruncated && readDirResp.NextHash != 0 {
		isTruncated := len(response.Contents)+len(response.CommonPrefixes) < len(readDirResp.Results)
		if isTruncated || (readDirResp.NextHash != 0 && len(readDirResp.Results) > 0) {
			response.IsTruncated = true
			if response.NextContinuationToken == "" {
				response.NextContinuationToken = fmt.Sprintf("%d", uint64(readDirResp.NextHash))
			}
		}
	}
	response.KeyCount = len(response.Contents) + len(response.CommonPrefixes)
	s.writeXML(w, http.StatusOK, response)
	return nil
}

func (s *S3Server) handlePutObject(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	s.log.Debug("PutObject: bucket=%q path=%q", bucket, path.String())

	if len(path.segments) == 0 {
		return &S3Error{Code: "MethodNotAllowed", Message: "PUT is not allowed on a bucket. To create a bucket, use the AWS CLI or other tools.", StatusCode: http.StatusMethodNotAllowed}
	}

	ownerId := rootId
	var err error

	// Create parent directories if they don't exist
	for _, dirName := range path.dir().segments {
		lookupResp := msgs.LookupResp{}
		err = s.c.ShardRequest(s.log, ownerId.Shard(), &msgs.LookupReq{DirId: ownerId, Name: dirName}, &lookupResp)
		if err != nil {
			if errors.Is(err, msgs.NAME_NOT_FOUND) {
				s.log.Debug("Directory %q in dir %d not found, creating it.", dirName, ownerId)
				resp := &msgs.MakeDirectoryResp{}
				if err = s.c.CDCRequest(s.log, &msgs.MakeDirectoryReq{OwnerId: ownerId, Name: dirName}, resp); err != nil {
					return fmt.Errorf("failed to construct directory %q: %w", dirName, err)
				}
				ownerId = resp.Id
			} else {
				return fmt.Errorf("failed to lookup directory %q: %w", dirName, err)
			}
		} else {
			if lookupResp.TargetId.Type() != msgs.DIRECTORY {
				return &S3Error{Code: "InvalidRequest", Message: fmt.Sprintf("A component of the path is not a directory: %q", dirName), StatusCode: http.StatusBadRequest}
			}
			ownerId = lookupResp.TargetId
		}
	}

	var inode msgs.InodeId
	basename := path.basename()
	if path.isDirectory {
		resp := &msgs.MakeDirectoryResp{}
		if err := s.c.CDCRequest(s.log, &msgs.MakeDirectoryReq{OwnerId: ownerId, Name: basename}, resp); err != nil {
			return fmt.Errorf("failed to make directory: %w", err)
		} else {
			inode = resp.Id
		}
	} else {
		fileResp := msgs.ConstructFileResp{}
		if err := s.c.ShardRequest(s.log, ownerId.Shard(), &msgs.ConstructFileReq{Type: msgs.FILE}, &fileResp); err != nil {
			return fmt.Errorf("failed to construct file: %w", err)
		}
		fileId := fileResp.Id
		cookie := fileResp.Cookie
		if err := s.c.WriteFile(s.log, s.bufPool, s.dirInfoCache, ownerId, fileId, cookie, r.Body); err != nil {
			return fmt.Errorf("failed to write file data: %w", err)
		}
		linkReq := &msgs.LinkFileReq{FileId: fileId, Cookie: cookie, OwnerId: ownerId, Name: basename}
		if err := s.c.ShardRequest(s.log, ownerId.Shard(), linkReq, &msgs.LinkFileResp{}); err != nil {
			return fmt.Errorf("failed to link file %q: %w", path.String(), err)
		}
		inode = fileId
	}

	w.Header().Set("ETag", FileEtag(inode))
	w.WriteHeader(http.StatusOK)
	return nil
}

func (s *S3Server) handleCreateMultipartUpload(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	if r.Method != http.MethodPost {
		return &S3Error{Code: "MethodNotAllowed", Message: "This resource only supports POST.", StatusCode: http.StatusMethodNotAllowed}
	}
	return &S3Error{Code: "NotImplemented", Message: "Multipart upload is not implemented.", StatusCode: http.StatusNotImplemented}
}

func (s *S3Server) handleUploadPart(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	if r.Method != http.MethodPut {
		return &S3Error{Code: "MethodNotAllowed", Message: "This resource only supports PUT.", StatusCode: http.StatusMethodNotAllowed}
	}
	return &S3Error{Code: "NotImplemented", Message: "Multipart upload is not implemented.", StatusCode: http.StatusNotImplemented}
}

func (s *S3Server) handleCompleteMultipartUpload(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	if r.Method != http.MethodPost {
		return &S3Error{Code: "MethodNotAllowed", Message: "This resource only supports POST.", StatusCode: http.StatusMethodNotAllowed}
	}
	var payload CompleteMultipartUpload
	if err := xml.NewDecoder(r.Body).Decode(&payload); err != nil {
		s.log.Info("Could not decode XML for CompleteMultipartUpload: %v", err)
		return &S3Error{Code: "MalformedXML", Message: fmt.Sprintf("The XML you provided was not well-formed or did not validate against our published schema: %v", err), StatusCode: http.StatusBadRequest}
	}
	return &S3Error{Code: "NotImplemented", Message: "Multipart upload is not implemented.", StatusCode: http.StatusNotImplemented}
}

// deleteObjectLogic contains the core logic for deleting a single object.
func (s *S3Server) deleteObjectLogic(ctx context.Context, rootId msgs.InodeId, path *objectPath) error {
	if len(path.segments) == 0 {
		return &S3Error{Code: "InvalidRequest", Message: "Cannot delete bucket root with object deletion API", StatusCode: http.StatusBadRequest}
	}

	dirId, dentry, err := s.lookupDirEntry(ctx, rootId, path)
	if err != nil {
		var s3Err *S3Error
		if errors.As(err, &s3Err) && s3Err.Code == "NoSuchKey" {
			s.log.Debug("deleteObjectLogic: Object %q not found, treating as success for deletion.", path.String())
			return nil // Success from S3 perspective
		}
		return err
	}

	if dentry.TargetId.Type() == msgs.DIRECTORY {
		req := &msgs.SoftUnlinkDirectoryReq{
			OwnerId:      dirId,
			TargetId:     dentry.TargetId,
			Name:         path.basename(),
			CreationTime: dentry.CreationTime,
		}
		if err := s.c.CDCRequest(s.log, req, &msgs.SoftUnlinkDirectoryResp{}); err != nil {
			return fmt.Errorf("could not delete directory %q: %w", path.String(), err)
		}
	} else {
		req := &msgs.SoftUnlinkFileReq{
			OwnerId:      dirId,
			FileId:       dentry.TargetId,
			Name:         path.basename(),
			CreationTime: dentry.CreationTime,
		}
		if err := s.c.ShardRequest(s.log, dirId.Shard(), req, &msgs.SoftUnlinkFileResp{}); err != nil {
			return fmt.Errorf("could not delete file %q: %w", path.String(), err)
		}
	}

	return nil
}

func (s *S3Server) handleDeleteObject(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	s.log.Debug("DeleteObject: path=%q", path.String())
	if err := s.deleteObjectLogic(ctx, rootId, path); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// deleteResult holds the outcome of a single deletion operation.
type deleteResult struct {
	Key string
	Err error
}

func (s *S3Server) handleDeleteObjects(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket string, rootId msgs.InodeId, path *objectPath) error {
	s.log.Debug("DeleteObjects in bucket %q", bucket)

	if r.Method != http.MethodPost {
		return &S3Error{Code: "MethodNotAllowed", Message: "This resource only supports POST.", StatusCode: http.StatusMethodNotAllowed}
	}
	if len(path.segments) != 0 {
		return &S3Error{Code: "InvalidRequest", Message: "DeleteObjects should be called on a bucket, not an object"}
	}
	var payload Delete
	if err := xml.NewDecoder(r.Body).Decode(&payload); err != nil {
		return &S3Error{Code: "MalformedXML", Message: fmt.Sprintf("The XML you provided was not well-formed or did not validate against our published schema: %v", err), StatusCode: http.StatusBadRequest}
	}

	results := make([]deleteResult, len(payload.Objects))
	g, gCtx := errgroup.WithContext(ctx)

	for i, obj := range payload.Objects {
		i, obj := i, obj // Capture loop variables to use in goroutine.
		g.Go(func() error {
			s.log.Debug("Batch deleting object: %q", obj.Key)
			err := s.deleteObjectLogic(gCtx, rootId, parseObjectPath(obj.Key))
			results[i] = deleteResult{Key: obj.Key, Err: err}
			// Return nil so that one failed deletion doesn't cancel others.
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err // Likely a context cancellation
	}

	var deletedObjects []DeletedObject
	var errorObjects []ErrorObject
	for _, res := range results {
		if res.Err != nil {
			var s3Err *S3Error
			code := "InternalError"
			message := res.Err.Error()
			if errors.As(res.Err, &s3Err) {
				code = s3Err.Code
				message = s3Err.Message
			}
			errorObjects = append(errorObjects, ErrorObject{Key: res.Key, Code: code, Message: message})
		} else if !payload.Quiet {
			deletedObjects = append(deletedObjects, DeletedObject{Key: res.Key})
		}
	}

	response := DeleteResult{
		Deleted: deletedObjects,
		Error:   errorObjects,
	}

	s.writeXML(w, http.StatusOK, response)
	return nil
}

// lookupPath is a helper to resolve a full path from the filesystem root to an InodeId.
func lookupPath(ctx context.Context, log *log.Logger, c *client.Client, path *objectPath) (msgs.InodeId, error) {
	current := msgs.ROOT_DIR_INODE_ID
	for _, segment := range path.segments {
		lookupResp := &msgs.LookupResp{}
		req := &msgs.LookupReq{DirId: current, Name: segment}
		if err := c.ShardRequest(log, current.Shard(), req, lookupResp); err != nil {
			return 0, fmt.Errorf("lookup failed for segment '%s': %w", segment, err)
		}
		current = lookupResp.TargetId
	}
	return current, nil
}
