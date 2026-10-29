package autopoiesis

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerationAuditDoesNotRetainOrReconstructResponse(t *testing.T) {
	client := &MockLLMClient{CompleteFunc: func(context.Context, string) (string, error) {
		t.Fatal("audit collection must not ask a model to reconstruct deliberation")
		return "", nil
	}}
	dir := t.TempDir()
	collector := NewTraceCollector(dir, client)
	record := collector.StartTrace("parser", &ToolNeed{Name: "parser"}, "Parse a document")
	// Neither numbered prose nor a response without steps should trigger extraction.
	for _, response := range []string{"1. private-response-marker\n2. another step", "private-response-marker"} {
		collector.RecordResponse(context.Background(), record, response, 42, time.Second)
	}
	collector.FinalizeTrace(record, true, "package main", "")
	data, err := os.ReadFile(filepath.Join(dir, "tool_generation_records.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-response-marker", "chain_of_thought", "raw_response"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("audit retained %q", forbidden)
		}
	}
	reopened := NewTraceCollector(dir, nil).GetTrace(record.TraceID)
	if reopened == nil || !reopened.Success || reopened.TokensUsed != 42 || reopened.GenerationTime != time.Second || reopened.CodeGenerated != "package main" {
		t.Fatalf("usage and artifact outcome did not survive reopening: %+v", reopened)
	}
}

func TestGenerationAuditDoesNotImportLegacyExtractedProse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "reasoning_traces.json")
	legacy := `{"traces":{"old":{"trace_id":"old","raw_response":"legacy-private-marker","chain_of_thought":[{"description":"legacy-private-marker"}]}},"by_tool":{}}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	collector := NewTraceCollector(dir, nil)
	if got := collector.GetAllTraces(); len(got) != 0 {
		t.Fatalf("imported legacy extracted prose: %+v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != legacy {
		t.Fatalf("legacy file was changed: %q, %v", data, err)
	}
}
