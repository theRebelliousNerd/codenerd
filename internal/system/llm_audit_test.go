package system

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// bindAuditToTempWorkspace points the process's logging, with the audit log
// on, at a fresh workspace for the rest of the test.
func bindAuditToTempWorkspace(t *testing.T) {
	t.Helper()
	// The workspace first, the close second: cleanups run last-registered
	// first, so the logs are closed before TempDir removes the directory. The
	// other order left the boot log open during RemoveAll, which Windows
	// refuses ("being used by another process") and fails the test.
	ws := t.TempDir()
	logging.ApplyConfig(logging.Config{DebugMode: true, Level: "debug"})
	t.Cleanup(func() {
		logging.CloseAll()
		logging.ClearInjectedConfig()
	})
	if err := logging.Initialize(ws); err != nil {
		t.Fatalf("logging.Initialize: %v", err)
	}
}

// auditEvents closes the audit log and reads back the events of the given kinds.
func auditEvents(t *testing.T, kinds ...logging.AuditEventType) []logging.AuditEvent {
	t.Helper()
	logging.CloseAudit()
	path, err := logging.LatestAuditLogPath()
	if err != nil {
		t.Fatalf("LatestAuditLogPath: %v", err)
	}
	events, err := logging.ReadRecentAuditEvents(path, kinds, 20)
	if err != nil {
		t.Fatalf("ReadRecentAuditEvents: %v", err)
	}
	return events
}

// tokenReportingLLMClient answers tool calls with a usage block, and fails on
// demand, so the audit record's token count and success flag can be checked.
type tokenReportingLLMClient struct {
	fakeMeteredLLMClient
	fail bool
}

func (f *tokenReportingLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if f.fail {
		return nil, errors.New("provider refused")
	}
	return &types.LLMToolResponse{Usage: types.UsageMetadata{InputTokens: 120, OutputTokens: 30}}, nil
}

// Every model call a session makes goes through sessionLLMAdapter, and each
// now lands in the audit trail as an llm_response event. The trail carried
// turns, intents, tools, files and safety checks but no model call, so a
// forensic replay could see what the agent did and not what it asked.
// Not parallel: it binds the process's logging to a temp workspace.
func TestSessionLLMAdapter_EveryCall_ShouldReachTheAuditTrail(t *testing.T) {
	bindAuditToTempWorkspace(t)

	ok := &sessionLLMAdapter{client: &tokenReportingLLMClient{}}
	if _, err := ok.CompleteWithTools(context.Background(), "sys", "user", nil); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	failing := &sessionLLMAdapter{client: &tokenReportingLLMClient{fail: true}}
	if _, err := failing.CompleteWithTools(context.Background(), "sys", "user", nil); err == nil {
		t.Fatal("expected the provider error")
	}
	if _, err := ok.Complete(context.Background(), "prompt"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	events := auditEvents(t, logging.AuditLLMResponse)
	if len(events) != 3 {
		t.Fatalf("got %d llm_response events, want 3: %+v", len(events), events)
	}
	var sawTokens, sawFailure bool
	for _, e := range events {
		if tokens, _ := e.Fields["tokens"].(float64); tokens == 150 && e.Success {
			sawTokens = true
		}
		if !e.Success && e.Error == "provider refused" {
			sawFailure = true
		}
	}
	if !sawTokens || !sawFailure {
		t.Fatalf("events lack the token count or the failure: %+v", events)
	}
}

// An unattended Ouroboros run -- the Cortex writing a tool for itself because
// the Dreamer reported one missing -- is a self-modification, and it now lands
// in the audit trail whether it succeeded or not.
func TestRecordToolGeneration_ShouldReachTheAuditTrail(t *testing.T) {
	bindAuditToTempWorkspace(t)

	recordToolGeneration("csv_parser", &autopoiesis.LoopResult{Success: false})
	recordToolGeneration("json_diff", &autopoiesis.LoopResult{Success: true, ToolName: "json_diff_v2"})
	recordToolGeneration("ignored", nil)

	events := auditEvents(t, logging.AuditToolGenerated)
	if len(events) != 2 {
		t.Fatalf("got %d tool_generated events, want 2: %+v", len(events), events)
	}
	targets := map[string]bool{}
	for _, e := range events {
		targets[e.Target] = e.Success
	}
	if ok, seen := targets["csv_parser"]; !seen || ok {
		t.Errorf("the failed generation is missing or marked a success: %+v", events)
	}
	if ok, seen := targets["json_diff_v2"]; !seen || !ok {
		t.Errorf("the generated tool is not recorded under its name: %+v", events)
	}
}
