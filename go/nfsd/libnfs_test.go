// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

//go:build libnfs

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func connectLibnfs(t *testing.T, addr string) *libnfsClient {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	c, err := libnfsConnect(host, port)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLibnfs_StatRoot(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	st, err := c.Stat("/")
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode&0040000 == 0 {
		t.Fatalf("root is not a directory: mode=%#o", st.Mode)
	}
}

func TestLibnfs_ReadFile(t *testing.T) {
	dir := t.TempDir()
	content := []byte("Hello from libnfs test!")
	os.WriteFile(filepath.Join(dir, "test.txt"), content, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	data, err := c.ReadFile("/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Fatalf("read = %q, want %q", data, content)
	}
}

func TestLibnfs_ReadLargeFile(t *testing.T) {
	dir := t.TempDir()
	content := make([]byte, 128*1024)
	for i := range content {
		content[i] = byte(i % 251)
	}
	os.WriteFile(filepath.Join(dir, "large.bin"), content, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	data, err := c.ReadFile("/large.bin")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != len(content) {
		t.Fatalf("read %d bytes, want %d", len(data), len(content))
	}
	for i := range data {
		if data[i] != content[i] {
			t.Fatalf("mismatch at byte %d: got %d, want %d", i, data[i], content[i])
		}
	}
}

func TestLibnfs_ReadDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "aaa.txt"), []byte("A"), 0644)
	os.WriteFile(filepath.Join(dir, "bbb.txt"), []byte("BB"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	names, err := c.ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	expected := []string{"aaa.txt", "bbb.txt", "subdir"}
	if len(names) != len(expected) {
		t.Fatalf("readdir = %v, want %v", names, expected)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("readdir[%d] = %q, want %q", i, names[i], expected[i])
		}
	}
}

func TestLibnfs_SubdirNavigation(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "a", "b"), 0755)
	os.WriteFile(filepath.Join(dir, "a", "b", "deep.txt"), []byte("deep"), 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	data, err := c.ReadFile("/a/b/deep.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "deep" {
		t.Fatalf("read = %q, want %q", data, "deep")
	}

	names, err := c.ReadDir("/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "b" {
		t.Fatalf("readdir(/a) = %v, want [b]", names)
	}
}

func TestLibnfs_StatFile(t *testing.T) {
	dir := t.TempDir()
	content := []byte("stat me")
	os.WriteFile(filepath.Join(dir, "info.txt"), content, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	st, err := c.Stat("/info.txt")
	if err != nil {
		t.Fatal(err)
	}
	if st.Size != uint64(len(content)) {
		t.Fatalf("size = %d, want %d", st.Size, len(content))
	}
	if st.Mode&0100000 == 0 {
		t.Fatalf("not a regular file: mode=%#o", st.Mode)
	}
}

func TestLibnfs_Readlink(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "target.txt"), []byte("target"), 0644)
	os.Symlink("target.txt", filepath.Join(dir, "link.txt"))

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	target, err := c.Readlink("/link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if target != "target.txt" {
		t.Fatalf("readlink = %q, want %q", target, "target.txt")
	}

	// Reading through the symlink should also work.
	data, err := c.ReadFile("/link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "target" {
		t.Fatalf("read through symlink = %q, want %q", data, "target")
	}
}

func TestLibnfs_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "empty.txt"), nil, 0644)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	data, err := c.ReadFile("/empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(data))
	}

	st, err := c.Stat("/empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	if st.Size != 0 {
		t.Fatalf("size = %d, want 0", st.Size)
	}
}

func TestLibnfs_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "empty"), 0755)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	names, err := c.ReadDir("/empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty dir, got %v", names)
	}
}

func TestLibnfs_NonExistent(t *testing.T) {
	dir := t.TempDir()
	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	_, err := c.Stat("/no-such-file.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLibnfs_ManyFiles(t *testing.T) {
	dir := t.TempDir()
	var expected []string
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("file_%03d.txt", i)
		os.WriteFile(filepath.Join(dir, name), []byte(name), 0644)
		expected = append(expected, name)
	}
	sort.Strings(expected)

	addr, cleanup := startTestServer(t, dir)
	defer cleanup()

	c := connectLibnfs(t, addr)
	defer c.Close()

	names, err := c.ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	if len(names) != len(expected) {
		t.Fatalf("readdir returned %d entries, want %d", len(names), len(expected))
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Fatalf("readdir[%d] = %q, want %q", i, names[i], expected[i])
		}
	}
}
