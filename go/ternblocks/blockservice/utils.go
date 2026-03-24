// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package blockservice

import (
	"os"
	"path/filepath"
)

func moveFileAndSyncDir(src, dst string) error {
	err := os.Rename(src, dst)
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(dst))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func eraseFileIfExistsAndSyncDir(path string) error {
	err := os.Remove(path)
	if err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
