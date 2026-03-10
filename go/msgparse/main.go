// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

// msgc compiles .msg message definition files into Go source code
// that performs zero-copy serialization/deserialization over byte buffers.
//
// Usage:
//
//	msgc [-pkg name] [-o output] input.msg
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	pkg := flag.String("pkg", "", "Go package name (default: derived from directory)")
	output := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: msgc [-pkg name] [-o output] input.msg")
		os.Exit(1)
	}

	inputFile := flag.Arg(0)
	src, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", inputFile, err)
		os.Exit(1)
	}

	// Determine package name.
	pkgName := *pkg
	if pkgName == "" {
		dir, _ := filepath.Abs(filepath.Dir(inputFile))
		pkgName = filepath.Base(dir)
		// Sanitize.
		pkgName = strings.ReplaceAll(pkgName, "-", "")
	}

	// Parse.
	parser := NewParser(src)
	file := parser.Parse()

	// Analyze.
	analysis := Analyze(file)

	// Generate.
	code := Generate(analysis, pkgName)

	if *output != "" {
		if err := os.WriteFile(*output, []byte(code), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", *output, err)
			os.Exit(1)
		}
		// Run gofmt on the output.
		cmd := exec.Command("gofmt", "-w", *output)
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else {
		// Write to stdout, pipe through gofmt if available.
		cmd := exec.Command("gofmt")
		cmd.Stdin = strings.NewReader(code)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// Fallback: write raw.
			fmt.Print(code)
		}
	}
}
