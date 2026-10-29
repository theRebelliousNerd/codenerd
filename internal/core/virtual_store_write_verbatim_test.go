package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Ladder V1: the VirtualStore writes what it is given. Every write_file used
// to pass through a model-reply extractor: the content was trimmed, a Go file
// was sliced from its first "package " (its //go:build line and licence header
// gone), and the first bare fence of any file was unwrapped -- a Go file whose
// package comment showed a fenced example landed as the example alone. The
// store's own FileWriteValidator then failed any content ending in a newline.
//
// Each case goes the production way: a kernel-permitted pending_action, then
// RouteActionResult with the store's validators, and the file must hold the
// exact bytes with the write verified.
func TestRouteAction_WriteFileLandsExactlyTheGivenBytes(t *testing.T) {
	cases := []struct {
		name, file, content string
	}{
		{"go build constraint and licence header", "constrained.go",
			"// Copyright 2026 The Authors. Licensed under Apache-2.0.\n\n//go:build go1.18\n\npackage constrained\n\nfunc A() {}\n"},
		{"go package comment with a fenced example", "documented.go",
			"// Package documented shows an example:\n//\n//\t```\n//\tx := 1\n//\t```\npackage documented\n\nfunc B() int { return 1 }\n"},
		{"markdown with an untagged fence", "guide.md",
			"# Guide\n\nRun it:\n\n```\nnerd check-mangle\n```\n\nThen read the report.\n"},
		{"leading and trailing whitespace", "spaced.txt", "\n  indented first line\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs, dir := permittedWriteStore(t)
			full := filepath.Join(dir, tc.file)
			res, err := routePermittedWrite(t, vs, full, tc.content)
			if err != nil || !res.Success {
				t.Fatalf("write_file %s: success=%v error=%q err=%v", tc.file, res.Success, res.Error, err)
			}
			got, readErr := os.ReadFile(full)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != tc.content {
				t.Fatalf("%s holds\n%q\nwant the bytes it was given\n%q", tc.file, got, tc.content)
			}
		})
	}
}

// The one change the store makes is documented: overwriting a CRLF file keeps
// CRLF, and the validator checks the content after that conversion.
func TestRouteAction_WriteFileKeepsAnExistingFilesLineEnding(t *testing.T) {
	vs, dir := permittedWriteStore(t)
	full := filepath.Join(dir, "crlf.txt")
	if err := os.WriteFile(full, []byte("old\r\nfile\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := routePermittedWrite(t, vs, full, "new\nfile\n")
	if err != nil || !res.Success {
		t.Fatalf("write_file: success=%v error=%q err=%v", res.Success, res.Error, err)
	}
	if got, _ := os.ReadFile(full); string(got) != "new\r\nfile\r\n" {
		t.Fatalf("crlf.txt holds %q, want the new content in the file's CRLF", got)
	}
}

func permittedWriteStore(t *testing.T) (*VirtualStore, string) {
	t.Helper()
	dir := t.TempDir()
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	cfg := DefaultVirtualStoreConfig()
	cfg.WorkingDir = dir
	vs := NewVirtualStoreWithConfig(&mockActionsExecutor{}, cfg)
	t.Cleanup(func() { vs.Close() })
	vs.SetKernel(k)
	vs.DisableBootGuard()
	return vs, dir
}

// routePermittedWrite asserts the pending_action the kernel's permission
// matches -- the exact canonical payload, as the session executor asserts it
// -- and routes the write.
func routePermittedWrite(t *testing.T, vs *VirtualStore, full, content string) (ActionResult, error) {
	t.Helper()
	payload := map[string]any{"content": content}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	const actionID = "v1-verbatim-write"
	pending := Fact{Predicate: "pending_action", Args: []any{actionID, MangleAtom("/write_file"), full, string(payloadJSON), time.Now().Unix()}}
	if err := vs.kernel.Assert(pending); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}
	t.Cleanup(func() { _ = vs.kernel.RetractFact(pending) })
	return vs.RouteActionResult(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{actionID, "write_file", full, payload},
	})
}
