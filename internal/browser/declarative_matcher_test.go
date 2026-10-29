package browser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestElementMatcherValidationAndSensitivity(t *testing.T) {
	if err := (ElementMatcher{}).Validate(); err == nil {
		t.Fatal("empty matcher accepted")
	}
	if err := (ElementMatcher{ID: "save", TagName: "button"}).Validate(); err != nil {
		t.Fatalf("valid matcher rejected: %v", err)
	}
	if !(ElementMatcher{ID: "password", InputType: "password"}).IsSensitive() {
		t.Fatal("password matcher was not sensitive")
	}
	if (ElementMatcher{ID: "display-name", InputType: "text"}).IsSensitive() {
		t.Fatal("ordinary matcher was sensitive")
	}
}

func TestMatcherForRefNeverExposesSelectors(t *testing.T) {
	manager := NewSessionManagerWithSink(DefaultConfig(), nil)
	manager.mu.Lock()
	manager.sessions["session-a"] = &sessionRecord{meta: Session{ID: "session-a"}, registry: NewElementRegistry()}
	manager.mu.Unlock()
	registry := manager.Registry("session-a")
	registered := registry.RegisterBatch([]ElementFingerprint{{
		Selector: "#save > span", AltSelectors: []string{"button#save"}, ID: "save", TagName: "button", TextContent: "Save",
	}})
	matcher, err := manager.MatcherForRef("session-a", registered[0].Ref)
	if err != nil {
		t.Fatal(err)
	}
	if matcher.ID != "save" || matcher.TagName != "button" || matcher.Text != "Save" {
		t.Fatalf("unexpected matcher: %+v", matcher)
	}
	if strings.Contains(strings.ToLower(strings.Join([]string{matcher.ID, matcher.Text, matcher.TagName}, " ")), "selector") {
		t.Fatalf("selector leaked through matcher: %+v", matcher)
	}
}

func TestRecordActionIntentIsBoundedRedactedEvidence(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultConfig()
	cfg.WorkspaceRoot = workspace
	cfg.WritableRoots = []string{workspace}
	manager := NewSessionManagerWithSink(cfg, nil)
	manager.recordActionIntent("session-a", ActionOperation{
		Type: "interact", Action: "type", Target: &ElementMatcher{ID: "password", InputType: "password"},
		ValueEnv: "CODENERD_BROWSER_TEST_PASSWORD",
	})
	read, err := manager.ReadEvidence("session-a", FlightReadOptions{Types: []string{"action_intent"}, MaxItems: 10})
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := json.Marshal(read.Events)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(bytes)
	if len(read.Events) != 1 || !strings.Contains(encoded, "CODENERD_BROWSER_TEST_PASSWORD") || strings.Contains(encoded, "unit-secret") {
		t.Fatalf("unexpected action intent: %s", encoded)
	}
}
