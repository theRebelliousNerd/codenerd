//go:build ignore

package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainOutput(t *testing.T) {
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "Hello, World!") {
		t.Fatalf("expected hello output")
	}
}

func TestUnusedFunctionOutput(t *testing.T) {
	output := captureStdout(func() {
		UnusedFunction()
	})
	if !strings.Contains(output, "I am lonely.") {
		t.Fatalf("expected unused function output")
	}
}

func captureStdout(fn func()) string {
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}
