package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainPrintsFields(t *testing.T) {
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "Field:") {
		t.Fatalf("expected output to include field names")
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
