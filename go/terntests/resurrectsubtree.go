// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/XTXMarkets/ternfs/go/core/log"
	"github.com/XTXMarkets/ternfs/go/core/wyhash"
	"github.com/XTXMarkets/ternfs/go/msgs"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

type resurrectSubtreeTestOpts struct {
	numDirs      int
	numFiles     int
	maxDepth     int
	maxFileSize  int
	overwriteMin int
}

func md5File(p string) string {
	f, err := os.Open(p)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		panic(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func md5Bytes(b []byte) string {
	h := md5.Sum(b)
	return hex.EncodeToString(h[:])
}

// buildResurrectSubtree creates numDirs directories (depth <= maxDepth) rooted
// at `root`, then drops numFiles files across them with random content. A
// subset of files is overwritten (atomic rename over the original) several
// times so that only the newest version should be resurrected. Returns a map
// of path -> md5 of the expected final content.
func buildResurrectSubtree(root string, rand *wyhash.Rand, opts *resurrectSubtreeTestOpts) map[string]string {
	if err := os.MkdirAll(root, 0777); err != nil {
		panic(err)
	}
	dirs := []string{root}
	for len(dirs) < opts.numDirs {
		parent := dirs[int(rand.Uint32())%len(dirs)]
		rel, err := filepath.Rel(root, parent)
		if err != nil {
			panic(err)
		}
		depth := 0
		if rel != "." {
			for _, c := range rel {
				if c == filepath.Separator {
					depth++
				}
			}
			depth++
		}
		if depth >= opts.maxDepth {
			continue
		}
		name := fmt.Sprintf("d%03d", len(dirs))
		d := path.Join(parent, name)
		if err := os.Mkdir(d, 0777); err != nil {
			panic(err)
		}
		dirs = append(dirs, d)
	}

	expected := map[string]string{}
	buf := []byte{}
	for i := 0; i < opts.numFiles; i++ {
		d := dirs[int(rand.Uint32())%len(dirs)]
		f := path.Join(d, fmt.Sprintf("f%04d.dat", i))
		size := int(rand.Uint32()%uint32(opts.maxFileSize-64)) + 64
		buf = ensureLen(buf, size)
		rand.Read(buf)
		if err := os.WriteFile(f, buf, 0666); err != nil {
			panic(err)
		}
		rel, err := filepath.Rel(root, f)
		if err != nil {
			panic(err)
		}
		expected[rel] = md5Bytes(buf)
	}

	// Overwrite a subset several times. TernFS files are immutable; write a
	// fresh file with a temporary name and rename it over the original. Only
	// the latest write should survive and be what resurrect-subtree returns.
	paths := make([]string, 0, len(expected))
	for p := range expected {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	// Fisher-Yates shuffle with our deterministic RNG.
	for i := len(paths) - 1; i > 0; i-- {
		j := int(rand.Uint32()) % (i + 1)
		paths[i], paths[j] = paths[j], paths[i]
	}
	toOverwrite := opts.overwriteMin
	if toOverwrite > len(paths) {
		toOverwrite = len(paths)
	}
	for i := 0; i < toOverwrite; i++ {
		rel := paths[i]
		full := path.Join(root, rel)
		var final []byte
		rounds := int(rand.Uint32()%3) + 2
		for k := 0; k < rounds; k++ {
			size := int(rand.Uint32()%uint32(opts.maxFileSize-64)) + 64
			buf = ensureLen(buf, size)
			rand.Read(buf)
			final = append(final[:0], buf...)
			tmp := full + fmt.Sprintf(".tmp%d", k)
			if err := os.WriteFile(tmp, final, 0666); err != nil {
				panic(err)
			}
			if err := os.Rename(tmp, full); err != nil {
				panic(err)
			}
		}
		expected[rel] = md5Bytes(final)
	}
	return expected
}

func walkMd5s(root string) map[string]string {
	out := map[string]string{}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[rel] = md5File(p)
		return nil
	})
	if err != nil {
		panic(err)
	}
	return out
}

func resurrectSubtreeTest(
	l *log.Logger,
	terncliExe string,
	registryAddress string,
	mountPoint string,
	opts *resurrectSubtreeTestOpts,
) {
	root := path.Join(mountPoint, "resurrect-subtree-test")
	src := path.Join(root, "test_delete")
	kept := path.Join(root, "test_kept")
	dst := path.Join(root, "test_resurrected")
	if err := os.MkdirAll(kept, 0777); err != nil {
		panic(err)
	}

	// The terntests harness applies DeleteAfterVersions=0 to ROOT, which the
	// fuse Unlink path turns into an immediate HardUnlinkFile after every
	// SoftUnlinkFile — wiping the owned snapshot edges that resurrect-subtree
	// needs. Override with infinite retention on this test's root so the
	// snapshot tree survives rm -rf.
	{
		var rootStat syscall.Stat_t
		if err := syscall.Stat(root, &rootStat); err != nil {
			panic(err)
		}
		c := newTestClient(l, registryAddress, nil)
		if err := c.MergeDirectoryInfo(l, msgs.InodeId(rootStat.Ino), &msgs.SnapshotPolicy{}); err != nil {
			c.Close()
			panic(err)
		}
		c.Close()
	}

	l.Info("building subtree at %v (%v dirs, %v files)", src, opts.numDirs, opts.numFiles)
	rand := wyhash.New(1)
	expected := buildResurrectSubtree(src, rand, opts)
	l.Info("wrote %v files", len(expected))

	// Grab the src directory inode id; this is what resurrect-subtree operates
	// on since the live path will be gone after the rm -rf below.
	var srcStat syscall.Stat_t
	if err := syscall.Stat(src, &srcStat); err != nil {
		panic(fmt.Errorf("could not stat %v: %w", src, err))
	}
	srcId := msgs.InodeId(srcStat.Ino)
	l.Info("source inode id: %v", srcId)

	// Pick a directory (one that has survived the creation loop, not the root)
	// and a file to move out of the subtree BEFORE deletion. Neither should be
	// resurrected at the destination — the directory because it's no longer
	// orphaned (owner != NULL), the file because the deleted dir no longer
	// owns the edge that pointed at it.
	movedDirRel := ""
	for rel := range expected {
		// pick the parent directory of the first file whose parent isn't src
		// itself.
		parent := path.Dir(rel)
		if parent != "." {
			// first segment of the relative path
			seg := parent
			if i := strings.IndexByte(parent, '/'); i >= 0 {
				seg = parent[:i]
			}
			movedDirRel = seg
			break
		}
	}
	if movedDirRel == "" {
		panic(fmt.Errorf("test subtree has no nested directories to move"))
	}
	movedDirSrc := path.Join(src, movedDirRel)
	movedDirDst := path.Join(kept, movedDirRel)
	// Collect the paths (and md5s) we expect to survive the move — they should
	// NOT appear in the resurrected tree.
	movedPaths := map[string]string{}
	for rel, sum := range expected {
		if rel == movedDirRel || startsWithDir(rel, movedDirRel) {
			movedPaths[rel] = sum
		}
	}
	if len(movedPaths) == 0 {
		panic(fmt.Errorf("moved-out directory %q has no descendants", movedDirRel))
	}
	l.Info("moving directory %v (%v entries) out of subtree", movedDirRel, len(movedPaths))
	if err := os.Rename(movedDirSrc, movedDirDst); err != nil {
		panic(err)
	}
	for rel := range movedPaths {
		delete(expected, rel)
	}

	// Pick a single file (not inside the already-moved dir) to rename out too.
	movedFileRel := ""
	for rel := range expected {
		movedFileRel = rel
		break
	}
	if movedFileRel == "" {
		panic(fmt.Errorf("no file left to move out after directory move"))
	}
	movedFileSrc := path.Join(src, movedFileRel)
	movedFileDst := path.Join(kept, "moved_"+path.Base(movedFileRel))
	l.Info("moving file %v out of subtree", movedFileRel)
	if err := os.Rename(movedFileSrc, movedFileDst); err != nil {
		panic(err)
	}
	movedFileMd5 := expected[movedFileRel]
	delete(expected, movedFileRel)

	// Now delete the subtree. After this, srcId refers to a snapshot directory
	// whose edges we want resurrect-subtree to reconstruct.
	l.Info("removing %v", src)
	if err := os.RemoveAll(src); err != nil {
		panic(err)
	}

	cmd := exec.Command(
		terncliExe,
		"-registry", registryAddress,
		"-mtu", "max",
		"resurrect-subtree",
		"-src-id", fmt.Sprintf("%v", uint64(srcId)),
		"-dst", "/"+path.Join("resurrect-subtree-test", "test_resurrected"),
	)
	cmd.Stdout = l.Sink(log.INFO)
	cmd.Stderr = l.Sink(log.INFO)
	l.Info("running %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("terncli resurrect-subtree failed: %w", err))
	}

	l.Info("verifying %v", dst)
	got := walkMd5s(dst)
	for rel, want := range expected {
		have, ok := got[rel]
		if !ok {
			panic(fmt.Errorf("resurrected tree missing %q", rel))
		}
		if have != want {
			panic(fmt.Errorf("resurrected %q md5 mismatch: want %v got %v", rel, want, have))
		}
	}
	for rel := range got {
		if _, ok := expected[rel]; !ok {
			panic(fmt.Errorf("resurrected tree has unexpected entry %q", rel))
		}
	}
	// The moved-out directory descendants and the moved-out file must not
	// reappear anywhere under the resurrected tree.
	for rel := range movedPaths {
		if _, ok := got[rel]; ok {
			panic(fmt.Errorf("moved-out dir entry %q was resurrected", rel))
		}
	}
	if _, ok := got[movedFileRel]; ok {
		panic(fmt.Errorf("moved-out file %q was resurrected", movedFileRel))
	}

	// Sanity: the files we moved out should still be intact at their new
	// locations.
	keptGot := walkMd5s(kept)
	for rel, want := range movedPaths {
		keptRel := rel // same relative layout, just under `kept/`
		have, ok := keptGot[keptRel]
		if !ok {
			panic(fmt.Errorf("moved-out dir entry %q disappeared from kept tree", rel))
		}
		if have != want {
			panic(fmt.Errorf("moved-out dir entry %q md5 mismatch at kept: want %v got %v", rel, want, have))
		}
	}
	keptFileRel := "moved_" + path.Base(movedFileRel)
	if have, ok := keptGot[keptFileRel]; !ok {
		panic(fmt.Errorf("moved-out file disappeared from kept tree"))
	} else if have != movedFileMd5 {
		panic(fmt.Errorf("moved-out file md5 mismatch at kept: want %v got %v", movedFileMd5, have))
	}

	l.Info("resurrect-subtree OK: %v files verified, %v moved-out entries excluded",
		len(expected), len(movedPaths)+1)
}

// startsWithDir reports whether rel is inside the directory named dir (i.e.
// has dir as its first path segment, followed by '/').
func startsWithDir(rel, dir string) bool {
	return strings.HasPrefix(rel, dir+"/")
}
