package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func TestJoinArgs(t *testing.T) {
	got := joinArgs([]string{"one", "two", "three"})
	if got != "one two three" {
		t.Fatalf("expected 'one two three', got '%s'", got)
	}
}

func TestQueryFactsNoFacts(t *testing.T) {
	logger = zap.NewNop()
	workspace = t.TempDir()

	output := captureOutput(t, func() {
		if err := queryFacts(&cobra.Command{}, []string{"next_action"}); err != nil {
			t.Fatalf("queryFacts returned error: %v", err)
		}
	})

	if !strings.Contains(output, "No facts found") {
		t.Fatalf("expected message about missing facts, got: %s", output)
	}
}

func TestRunWhyNotInitialized(t *testing.T) {
	logger = zap.NewNop()
	workspace = t.TempDir()

	output := captureOutput(t, func() {
		if err := runWhy(&cobra.Command{}, []string{}); err != nil {
			t.Fatalf("runWhy returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Project not initialized") {
		t.Fatalf("expected initialization notice, got: %s", output)
	}
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	origOut := os.Stdout
	origErr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_, _ = io.Copy(&buf, rErr)
		done <- buf.String()
	}()

	fn()

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = origOut
	os.Stderr = origErr
	return <-done
}
