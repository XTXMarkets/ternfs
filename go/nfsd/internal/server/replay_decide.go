// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"errors"
	"os"

	"github.com/XTXMarkets/ternfs/go/nfsd/internal/nfs4"
	"github.com/XTXMarkets/ternfs/go/nfsd/internal/vfs"
)

// Deciding a request retransmitted after a restart whose slot holds an
// intent but no reply (README.md#restart-replay-decision). The earlier
// process may or may not have committed the effect. The compound runs
// again, and the operation the intent names is decided against TernFS
// first: applied, not applied, or NFS4ERR_DEADSESSION, the RFC's answer for
// a request a restart left half done (RFC 8881 §18.46.4).
//
// While the file was on disk each compound ran at most one mutating
// operation (mutationGate), so the intent, by its index, names the only one
// that may have changed anything. Nothing a rerun cannot prove is done
// again: a mutating operation before it, an operation at its index that is
// not the one it names or uses other names or directories (a LOOKUP before
// it can resolve differently now), and every mutating operation when the
// intent cannot be read get DEADSESSION. RENAME and REMOVE are decided on
// edge identity, since TernFS never reuses an inode ID; a CREATE has no such
// proof, so a restored CREATE intent is always DEADSESSION.

type replayVerdict uint8

const (
	// replayNone: no intent for this operation; run it as usual.
	replayNone replayVerdict = iota
	// replayApplied: the effect is there; report success.
	replayApplied
	// replayUndecidable: neither state fits; fail with the status returned.
	replayUndecidable
)

// replayState is a restored intent for a request run again after a restart.
type replayState struct {
	pending   bool // not yet taken by its operation
	malformed bool // the intent does not decode, or names no operation
	op        int  // index of the operation it protects
	intent    any
	raw       []byte // the intent as recorded
}

// newReplayState reads the intent a restored slot held, if it held one.
// An intent that does not decode, an empty one included, or that names
// SEQUENCE or an index past the largest compound, is malformed.
func newReplayState(present bool, b []byte) replayState {
	if !present {
		return replayState{}
	}
	op, in, err := decodeIntent(b)
	if err != nil || op < 1 || op >= maxCompoundOperations {
		return replayState{pending: true, malformed: true}
	}
	return replayState{pending: true, op: op, intent: in, raw: b}
}

// replayGate is the status for running opnum, at st.opIndex, in a request
// run again with a restored intent: DEADSESSION for an operation that may
// repeat an effect nothing records.
func (s *Server) replayGate(st *compoundState, opnum uint32) uint32 {
	r := &st.replay
	refuse := false
	switch {
	case !r.pending:
	case r.malformed:
		refuse = operationMutates(opnum)
	case st.opIndex < r.op:
		refuse = operationMutates(opnum)
	case st.opIndex == r.op:
		refuse = opnum != intentOperation(r.intent)
	default:
		// The operation it names went on without deciding it. Nothing does,
		// but if one did, what came after it is not known either.
		refuse = operationMutates(opnum)
	}
	if !refuse {
		return nfs4.NFS4_OK
	}
	s.log.Warn("replayed request undecidable", "session", sessionText(st.session.id), "slot", st.slotID,
		"seq", st.seq, "op", nfs4.NfsOpnum4Name(opnum), "index", st.opIndex, "malformed", r.malformed)
	s.metrics.replayUndecidable.Add(1)
	return nfs4.NFS4ERR_DEADSESSION
}

// mutationGate removes the session's file before a compound's second
// mutating operation, and every later one. The file can protect one effect
// of a request, by the intent it records first, and after a kill a
// retransmission runs the whole compound again: OPEN, WRITE and CLOSE of an
// existing file, alone or after a REMOVE, would publish once more over a
// file put at the name since. Without the file the retransmission gets
// BADSESSION, as it did before sessions were kept on disk, and the request
// goes on in memory. A first mutating operation that changed nothing still
// counts. If the file cannot be removed, the operation gets SERVERFAULT
// before its effect, and the session is retired once its slot is unlocked.
func (s *Server) mutationGate(st *compoundState, opnum uint32) uint32 {
	if st.session == nil || !operationMutates(opnum) {
		return nfs4.NFS4_OK
	}
	if !st.mutated {
		st.mutated = true
		return nfs4.NFS4_OK
	}
	if !s.dropSessionFile(st.session, "a compound's second mutating operation") {
		return nfs4.NFS4ERR_SERVERFAULT
	}
	return nfs4.NFS4_OK
}

// takeIntent consumes the compound's restored intent for the operation
// being run, the one it was written for. replayGate has checked that the
// operation is the kind it names. From here the intent on disk is this
// operation's, which persistIntent reuses if it runs and would write the
// same intent.
func (st *compoundState) takeIntent() (any, bool) {
	r := &st.replay
	if !r.pending || r.malformed || st.opIndex != r.op {
		return nil, false
	}
	r.pending = false
	st.intentOp = st.opIndex + 1
	return r.intent, true
}

// undecidableReplay logs and counts a replay that cannot be decided.
func (s *Server) undecidableReplay(what string, args ...any) (replayVerdict, uint32) {
	s.log.Warn("replayed "+what+" undecidable", args...)
	s.metrics.replayUndecidable.Add(1)
	return replayUndecidable, nfs4.NFS4ERR_DEADSESSION
}

// decideRenameReplay says whether the RENAME the intent describes happened.
func (s *Server) decideRenameReplay(st *compoundState, srcDir vfs.InodeID, oldName string, dstDir vfs.InodeID, newName string) (replayVerdict, uint32) {
	v, ok := st.takeIntent()
	if !ok {
		return replayNone, nfs4.NFS4_OK
	}
	logArgs := []any{"src_dir", srcDir, "old_name", oldName, "dst_dir", dstDir, "new_name", newName}
	in, ok := v.(renameIntent)
	if !ok || in.srcDir != srcDir || in.srcName != oldName || in.dstDir != dstDir || in.dstName != newName {
		return s.undecidableReplay("RENAME on other names", logArgs...)
	}
	edge, err := s.fs.LookupEdge(srcDir, oldName)
	if err == nil && edge == in.src {
		return replayNone, nfs4.NFS4_OK // The source is still there: not applied.
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return replayUndecidable, s.errToNFS(err)
	}
	cur, err := s.fs.Lookup(dstDir, newName)
	if err == nil && cur == in.src.ID {
		s.metrics.replayDecidedApplied.Add(1)
		return replayApplied, nfs4.NFS4_OK
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return replayUndecidable, s.errToNFS(err)
	}
	return s.undecidableReplay("RENAME", logArgs...)
}

// decideRemoveReplay says whether the REMOVE the intent describes happened.
// The edge being gone, or replaced, is taken as removed: the client asked
// for the name to be gone.
func (s *Server) decideRemoveReplay(st *compoundState, dir vfs.InodeID, name string) (replayVerdict, uint32) {
	v, ok := st.takeIntent()
	if !ok {
		return replayNone, nfs4.NFS4_OK
	}
	in, ok := v.(removeIntent)
	if !ok || in.dir != dir || in.name != name {
		return s.undecidableReplay("REMOVE on another name", "dir", dir, "name", name)
	}
	edge, err := s.fs.LookupEdge(dir, name)
	switch {
	case err == nil && edge == in.edge:
		return replayNone, nfs4.NFS4_OK
	case err == nil || errors.Is(err, os.ErrNotExist):
		s.metrics.replayDecidedApplied.Add(1)
		return replayApplied, nfs4.NFS4_OK
	}
	return replayUndecidable, s.errToNFS(err)
}

// decideCreateReplay fails a CREATE run again with its intent restored:
// nothing proves whether it made what is, or is not, at the name.
func (s *Server) decideCreateReplay(st *compoundState, dir vfs.InodeID, name string) (replayVerdict, uint32) {
	if _, ok := st.takeIntent(); !ok {
		return replayNone, nfs4.NFS4_OK
	}
	return s.undecidableReplay("CREATE", "dir", dir, "name", name)
}
