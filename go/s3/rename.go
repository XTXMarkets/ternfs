// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package s3

import (
	"context"
	"errors"
	"net/http"

	"github.com/XTXMarkets/ternfs/go/msgs"
)

func (s *S3Server) handleRenameObject(
	ctx context.Context, w http.ResponseWriter, r *http.Request,
	bucket string, rootId msgs.InodeId, destination *objectPath,
) error {
	source, err := parseRenameRequest(r, bucket, destination)
	if err != nil {
		return err
	}
	parent, entry, err := s.lookupDirEntry(ctx, rootId, source)
	if err != nil {
		return renameError(err)
	}
	if !entry.TargetId.IsFile() {
		return &S3Error{
			Code: "InvalidRequest", Message: "RenameObject requires a regular file.",
			StatusCode: http.StatusBadRequest,
		}
	}
	if source.String() != destination.String() {
		newParent, err := s.lookupDir(ctx, rootId, destination.dir())
		if err != nil {
			return renameError(err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if parent == newParent {
			err = s.c.ShardRequest(
				s.log, parent.Shard(),
				&msgs.SameDirectoryRenameReq{
					TargetId: entry.TargetId, DirId: parent,
					OldName: source.basename(), OldCreationTime: entry.CreationTime,
					NewName: destination.basename(),
				},
				&msgs.SameDirectoryRenameResp{},
			)
		} else {
			err = s.c.CDCRequest(
				s.log,
				&msgs.RenameFileReq{
					TargetId: entry.TargetId, OldOwnerId: parent,
					OldName: source.basename(), OldCreationTime: entry.CreationTime,
					NewOwnerId: newParent, NewName: destination.basename(),
				},
				&msgs.RenameFileResp{},
			)
		}
		if err != nil {
			return renameError(err)
		}
	}
	w.Header().Set("ETag", FileEtag(entry.TargetId))
	w.WriteHeader(http.StatusOK)
	return nil
}

func renameError(err error) error {
	for _, missing := range []error{
		msgs.NAME_NOT_FOUND, msgs.FILE_NOT_FOUND, msgs.DIRECTORY_NOT_FOUND,
		msgs.OLD_DIRECTORY_NOT_FOUND, msgs.NEW_DIRECTORY_NOT_FOUND,
	} {
		if errors.Is(err, missing) {
			return &S3Error{
				Code: "NoSuchKey", Message: "A source or parent directory does not exist.",
				StatusCode: http.StatusNotFound,
			}
		}
	}
	if errors.Is(err, msgs.CANNOT_OVERRIDE_NAME) ||
		errors.Is(err, msgs.MISMATCHING_CREATION_TIME) ||
		errors.Is(err, msgs.MISMATCHING_TARGET) ||
		errors.Is(err, msgs.MISMATCHING_OWNER) ||
		errors.Is(err, msgs.EDGE_NOT_FOUND) {
		return &S3Error{
			Code: "OperationAborted", Message: "The rename conflicts with a namespace change.",
			StatusCode: http.StatusConflict,
		}
	}
	return err
}
