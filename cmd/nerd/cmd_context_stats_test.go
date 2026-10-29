package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalcontext "codenerd/internal/context"
)

func TestContextStatsCmd_WhenNoDatabase_ShouldReportMissingStore(t *testing.T) {
	ws := t.TempDir()
	contextStatsWorkspace = ws
	contextStatsJSON = false
	t.Cleanup(func() { contextStatsWorkspace = "" })

	var out bytes.Buffer
	contextStatsCmd.SetOut(&out)
	err := contextStatsCmd.RunE(contextStatsCmd, nil)
	if err == nil {
		t.Fatal("expected an error when the feedback database does not exist")
	}
	if !strings.Contains(err.Error(), "context_feedback.db") {
		t.Errorf("error should name the missing database, got: %v", err)
	}
}

func TestContextStatsCmd_WhenStoreIsEmpty_ShouldReportSampleFloor(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(ws, ".nerd", "context_feedback.db")
	store, err := internalcontext.NewContextFeedbackStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close()

	contextStatsWorkspace = ws
	contextStatsJSON = false
	contextStatsTop = 5
	t.Cleanup(func() { contextStatsWorkspace = "" })

	var out bytes.Buffer
	contextStatsCmd.SetOut(&out)
	if err := contextStatsCmd.RunE(contextStatsCmd, nil); err != nil {
		// An empty feedback table used to fail here: AVG() over no rows is NULL
		// and would not scan into float64.
		t.Fatalf("empty store must report, not error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"turns rated", "min samples/pred", "HELPFUL", "NOISE"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestContextStatsCmd_WhenJSONRequested_ShouldEmitParsableStats(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	store, err := internalcontext.NewContextFeedbackStore(filepath.Join(ws, ".nerd", "context_feedback.db"))
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	store.Close()

	contextStatsWorkspace = ws
	contextStatsJSON = true
	t.Cleanup(func() {
		contextStatsWorkspace = ""
		contextStatsJSON = false
	})

	var out bytes.Buffer
	contextStatsCmd.SetOut(&out)
	if err := contextStatsCmd.RunE(contextStatsCmd, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	var stats internalcontext.FeedbackStats
	if err := json.Unmarshal(out.Bytes(), &stats); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if !stats.Available {
		t.Error("stats from a real store must be marked Available")
	}
	if stats.MinSamples <= 0 {
		t.Errorf("MinSamples = %d, want the store's trust floor", stats.MinSamples)
	}
}
