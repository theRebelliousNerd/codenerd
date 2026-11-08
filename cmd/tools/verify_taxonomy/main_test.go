package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"codenerd/internal/features"
)

func TestMainVerificationOutput(t *testing.T) {
	t.Setenv("CODENERD_TAXONOMY_FAST", "1")
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "VERIFICATION COMPLETE") {
		t.Fatalf("expected verification completion output")
	}
}

func TestMainFastPathSkipsScenarios(t *testing.T) {
	features.SetActive(nil)
	t.Setenv("CODENERD_TAXONOMY_FAST", "1")
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "MANGLE TAXONOMY VERIFICATION PROTOCOL") {
		t.Fatalf("fast path should print protocol header, got:\n%s", output)
	}
	if !strings.Contains(output, "VERIFICATION COMPLETE") {
		t.Fatalf("fast path should print completion, got:\n%s", output)
	}
	if strings.Contains(output, ">>> Input:") {
		t.Fatalf("fast path should skip scenario sweep, got scenario output:\n%s", output)
	}
}

func TestMainSlowPathRunsScenarioSweep(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	t.Setenv("CODENERD_TAXONOMY_FAST", "0")
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "MANGLE TAXONOMY VERIFICATION PROTOCOL") {
		t.Fatalf("slow path should print protocol header, got:\n%s", output)
	}
	for _, want := range []string{">>> Input:", "Goal:", "Result:", "Status:", "VERIFICATION COMPLETE"} {
		if !strings.Contains(output, want) {
			t.Fatalf("slow path output should contain %q, got:\n%s", want, output)
		}
	}
	if got := strings.Count(output, ">>> Input:"); got != 5 {
		t.Fatalf("slow path should run 5 scenarios, got %d:\n%s", got, output)
	}
}

func TestMainFastAcceptsTrueSpelling(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	t.Setenv("CODENERD_TAXONOMY_FAST", "true")
	output := captureStdout(func() {
		main()
	})
	if !strings.Contains(output, "VERIFICATION COMPLETE") {
		t.Fatalf("expected verification completion output, got:\n%s", output)
	}
	if strings.Contains(output, ">>> Input:") {
		t.Fatalf("fast path (true) should skip scenario sweep, got:\n%s", output)
	}
}

func TestCaptureStdoutRestoresStdout(t *testing.T) {
	features.SetActive(nil)
	t.Setenv("CODENERD_TAXONOMY_FAST", "1")
	inner := captureStdout(func() {
		main()
	})
	if !strings.Contains(inner, "VERIFICATION COMPLETE") {
		t.Fatalf("captured output should contain completion, got:\n%s", inner)
	}
	// Stdout must be usable after capture: a second capture still works.
	again := captureStdout(func() {
		main()
	})
	if !strings.Contains(again, "VERIFICATION COMPLETE") {
		t.Fatalf("second capture after restore should contain completion, got:\n%s", again)
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
