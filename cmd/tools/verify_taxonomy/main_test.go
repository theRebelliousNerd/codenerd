package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestMainVerificationOutput(t *testing.T) {
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "VERIFICATION COMPLETE") {
		t.Fatalf("expected verification completion output")
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
