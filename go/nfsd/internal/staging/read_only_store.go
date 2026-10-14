// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package staging

import (
	"os"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// ReadOnlyStore is used when no staging directory is configured.
type ReadOnlyStore struct{}

func (ReadOnlyStore) ReadOnly() bool { return true }

func (ReadOnlyStore) Create(vfs.InodeID, Meta) (File, error) {
	return nil, os.ErrPermission
}

func (ReadOnlyStore) Get(vfs.InodeID) File { return nil }

func (ReadOnlyStore) GetMeta(vfs.InodeID) (Meta, bool) { return Meta{}, false }

func (ReadOnlyStore) FindTargets(vfs.InodeID, string) map[vfs.InodeID]Meta { return nil }

func (ReadOnlyStore) Entries() map[vfs.InodeID]Meta { return nil }

func (ReadOnlyStore) Rebind(vfs.InodeID, uint64, nfs4.StateID) error {
	return os.ErrPermission
}

func (ReadOnlyStore) SetGuard(vfs.InodeID, bool) error { return os.ErrPermission }

func (ReadOnlyStore) Detach(vfs.InodeID) error { return os.ErrPermission }

func (ReadOnlyStore) Retarget(vfs.InodeID, vfs.InodeID, string) error { return os.ErrPermission }

func (ReadOnlyStore) Quarantine(vfs.InodeID) error { return nil }

func (ReadOnlyStore) Failed(vfs.InodeID) bool { return false }

func (ReadOnlyStore) Remove(vfs.InodeID) {}

func (ReadOnlyStore) StagedSize(vfs.InodeID) (uint64, bool) { return 0, false }
