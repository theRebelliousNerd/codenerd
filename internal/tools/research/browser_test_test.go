package research

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/browser"
	"codenerd/internal/types"
)

func TestBrowserTestCreateAndInspectPortableYAML(t *testing.T) {
	manager := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserManager(manager)
	defer ClearBrowserManager(manager)

	fixture := map[string]any{
		"name": "save profile",
		"actions": []any{map[string]any{
			"type": "interact", "action": "click", "target": map[string]any{"data_testid": "save-profile"},
		}},
		"assertions": []any{map[string]any{
			"name": "no errors", "query": "user_visible_error(S, Kind, Message, Timestamp)", "expect": "absent",
		}},
	}
	created, err := BrowserTestTool().Execute(context.Background(), map[string]any{
		"operation": "create", "test": fixture, "view": "full",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(created, `"status":"valid"`) || !strings.Contains(created, "data_testid: save-profile") || strings.Contains(created, "ref:") {
		t.Fatalf("unexpected create result: %s", created)
	}
	inspected, err := BrowserTestTool().Execute(context.Background(), map[string]any{
		"operation": "inspect", "test_yaml": extractBrowserTestYAML(t, created), "view": "summary",
	})
	if err != nil || !strings.Contains(inspected, `"action_count":1`) || strings.Contains(inspected, "test_yaml") {
		t.Fatalf("unexpected inspect result: %v, %s", err, inspected)
	}
}

func TestBrowserTestGenerateReadsPortableActionIntent(t *testing.T) {
	workspace := t.TempDir()
	cfg := browser.DefaultConfig()
	cfg.WorkspaceRoot = workspace
	cfg.WritableRoots = []string{workspace}
	manager := browser.NewSessionManagerWithSink(cfg, nil)
	SetBrowserManager(manager)
	defer ClearBrowserManager(manager)

	_, err := manager.RecordEvidence("session-a", "action_intent", map[string]any{"operation": browser.ActionOperation{
		Type: "interact", Action: "type", Target: &browser.ElementMatcher{ID: "password", InputType: "password"},
		ValueEnv: "CODENERD_BROWSER_TEST_PASSWORD",
	}})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := BrowserTestTool().Execute(context.Background(), map[string]any{
		"operation": "generate", "session_id": "session-a", "name": "login", "view": "full",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(generated, `"status":"draft"`) || !strings.Contains(generated, "CODENERD_BROWSER_TEST_PASSWORD") ||
		strings.Contains(generated, "selector") || strings.Contains(generated, "unit-secret") {
		t.Fatalf("unexpected generated fixture: %s", generated)
	}
}

func TestBrowserTestGenerateRefusesOversizedActionHistory(t *testing.T) {
	workspace := t.TempDir()
	cfg := browser.DefaultConfig()
	cfg.WorkspaceRoot = workspace
	cfg.WritableRoots = []string{workspace}
	manager := browser.NewSessionManagerWithSink(cfg, nil)
	SetBrowserManager(manager)
	defer ClearBrowserManager(manager)
	for index := 0; index < maxGeneratedBrowserActions+1; index++ {
		if _, err := manager.RecordEvidence("session-a", "action_intent", map[string]any{"operation": browser.ActionOperation{
			Type: "key", Key: "Tab",
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := BrowserTestTool().Execute(context.Background(), map[string]any{
		"operation": "generate", "session_id": "session-a",
	}); err == nil || !strings.Contains(err.Error(), "exceed fixture limit") {
		t.Fatalf("expected action history bound, got %v", err)
	}
}

func TestSubtractBrowserFactsUsesPerAssertionBaseline(t *testing.T) {
	old := types.Fact{Predicate: "console_event", Args: []any{"session-a", "error", "old", int64(1)}}
	newFact := types.Fact{Predicate: "console_event", Args: []any{"session-a", "error", "new", int64(2)}}
	fresh := subtractBrowserFacts([]types.Fact{old, newFact}, []types.Fact{old})
	if len(fresh) != 1 || fresh[0].Args[2] != "new" {
		t.Fatalf("unexpected fresh facts: %+v", fresh)
	}
}

func TestBrowserTestInvalidRedactsErrors(t *testing.T) {
	raw, err := browserTestInvalid("run", fmt.Errorf("password=secret-value"))
	if err != nil {
		t.Fatalf("browserTestInvalid: %v", err)
	}
	if strings.Contains(raw, "secret-value") || !strings.Contains(raw, "[REDACTED]") {
		t.Fatalf("invalid fixture error was not redacted: %s", raw)
	}
}

func extractBrowserTestYAML(t *testing.T, raw string) string {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	value, _ := decoded["test_yaml"].(string)
	if value == "" {
		t.Fatalf("missing test_yaml: %s", raw)
	}
	return value
}
