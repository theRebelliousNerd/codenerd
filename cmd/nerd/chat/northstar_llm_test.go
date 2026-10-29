package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/perception"
)

type northstarTUIScriptClient struct {
	perception.LLMClient
	complete func(context.Context, string, string) (string, error)
}

func (c *northstarTUIScriptClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return c.complete(ctx, system, user)
}

func northstarTUIFrame(t *testing.T, text string) (string, bool) {
	t.Helper()
	var fields [6]string
	for i := range fields {
		var ok bool
		fields[i], text, ok = strings.Cut(text, "\n")
		if !ok {
			t.Fatal("TUI did not use library frames")
		}
	}
	bodyLen, err := strconv.Atoi(strings.TrimPrefix(fields[3], "byte_length: "))
	if err != nil {
		t.Fatal(err)
	}
	summaryLen, err := strconv.Atoi(strings.TrimPrefix(fields[5], "running_summary_length: "))
	if err != nil {
		t.Fatal(err)
	}
	if summaryLen+bodyLen > len(text) {
		t.Fatal("incomplete frame")
	}
	return text[summaryLen : summaryLen+bodyLen], fields[4] == "final_page: true"
}

func TestNorthstarTUI_DocumentsUseLosslessLibrary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "research.md")
	body := strings.Repeat("full text\n", 10000) + "tail evidence\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var received strings.Builder
	calls := 0
	client := &northstarTUIScriptClient{complete: func(_ context.Context, system, user string) (string, error) {
		if !strings.Contains(system, "You are reading project documents") {
			t.Fatal("classification JIT atom absent")
		}
		if len(user) > config.DefaultOrientConfig().DeriveRequestBytes {
			t.Fatal("request over config bound")
		}
		page, final := northstarTUIFrame(t, user)
		received.WriteString(page)
		calls++
		if !final {
			return `{"summary":"carry full document findings"}`, nil
		}
		payload, err := json.Marshal(map[string]any{"documents": []any{map[string]any{"path": path, "roles": []any{map[string]any{"role": "reference", "confidence_pct": 99, "evidence": "tail evidence"}}, "themes": []string{"full text"}}}})
		return string(payload), err
	}}
	m := Model{client: client}
	msg, ok := m.analyzeNorthstarDocs([]string{path})().(northstarDocsAnalyzedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("message=%+v", msg)
	}
	if calls < 2 || received.String() != body || len(msg.facts) != 2 || !strings.Contains(msg.facts[0], "tail evidence") {
		t.Fatal("TUI failed lossless library conversion")
	}
}

func TestNorthstarTUI_RequirementsUseSharedJSONParser(t *testing.T) {
	var reqs []map[string]any
	for i := 0; i < 20; i++ {
		reqs = append(reqs, map[string]any{"id": "", "type": "functional", "description": fmt.Sprintf("requirement %d", i), "priority": "must-have", "source": "Mission"})
	}
	response, err := json.Marshal(map[string]any{"requirements": reqs})
	if err != nil {
		t.Fatal(err)
	}
	client := &northstarTUIScriptClient{complete: func(_ context.Context, system, user string) (string, error) {
		if !strings.Contains(system, "You are turning a northstar wizard state") {
			t.Fatal("requirements JIT atom absent")
		}
		body, final := northstarTUIFrame(t, user)
		if !final {
			t.Fatal("small wizard unexpectedly paged")
		}
		if !strings.Contains(body, `"PainPoints":["pain"]`) && !strings.Contains(body, `"pain_points":["pain"]`) {
			t.Fatal("persona pain points lost")
		}
		if !strings.Contains(body, "REQ-001") {
			t.Fatal("existing IDs lost")
		}
		return string(response), nil
	}}
	w := &NorthstarWizardState{Mission: "Ground decisions", Personas: []UserPersona{{Name: "Engineer", PainPoints: []string{"pain"}}}, Requirements: []NorthstarRequirement{{ID: "REQ-001"}}}
	m := Model{client: client, northstarWizard: w}
	msg, ok := m.generateRequirementsWithLLM()().(requirementsGeneratedMsg)
	if !ok || msg.err != nil {
		t.Fatalf("message=%+v", msg)
	}
	if len(msg.requirements) != 20 || msg.requirements[0].ID != "REQ-002" || msg.requirements[19].ID != "REQ-021" {
		t.Fatal("TUI lost objects or reused existing IDs")
	}
	if len(w.Requirements) != 1 {
		t.Fatal("generation mutated existing wizard requirements")
	}
}

func TestNorthstarTUI_ReadFailuresAreVisible(t *testing.T) {
	client := &northstarTUIScriptClient{complete: func(context.Context, string, string) (string, error) {
		t.Fatal("called model after failed read")
		return "", nil
	}}
	m := Model{client: client}
	msg := m.analyzeNorthstarDocs([]string{filepath.Join(t.TempDir(), "missing.md")})().(northstarDocsAnalyzedMsg)
	if msg.err == nil {
		t.Fatal("unreadable document silently omitted")
	}
}

func TestNorthstarTUI_RequirementsCarryLinksToTheCanonicalIDs(t *testing.T) {
	client := &northstarTUIScriptClient{complete: func(_ context.Context, _, user string) (string, error) {
		body, _ := northstarTUIFrame(t, user)
		// The model is shown the wizard's stable IDs, not positional aliases.
		if !strings.Contains(body, `"ID":"cap_custom"`) && !strings.Contains(body, `"id":"cap_custom"`) {
			t.Fatalf("model was not shown the capability's real ID: %s", body)
		}
		return `{"requirements":[{"type":"functional","description":"Ground decisions","priority":"must-have","source":"Capabilities[0]","supports":["cap_custom"],"addresses":["risk_1"]}]}`, nil
	}}
	w := &NorthstarWizardState{Mission: "Ground decisions",
		Capabilities: []Capability{{ID: "cap_custom", Description: "Evidence", Timeline: "now", Priority: "critical"}},
		Risks:        []Risk{{Description: "Drift", Likelihood: "high", Impact: "high"}}}
	m := Model{client: client, northstarWizard: w}
	msg := m.generateRequirementsWithLLM()().(requirementsGeneratedMsg)
	if msg.err != nil || len(msg.requirements) != 1 {
		t.Fatalf("message = %+v", msg)
	}
	req := msg.requirements[0]
	if !reflect.DeepEqual(req.Supports, []string{"cap_custom"}) || !reflect.DeepEqual(req.Addresses, []string{"risk_1"}) {
		t.Fatalf("links did not survive the TUI adapter: %+v", req)
	}
	if w.Risks[0].ID != "risk_1" {
		t.Fatal("a risk created in the wizard was not given its stable ID before generation")
	}
}

func TestNorthstarTUI_RequirementsRejectLinksToUnknownEntities(t *testing.T) {
	calls := 0
	client := &northstarTUIScriptClient{complete: func(context.Context, string, string) (string, error) {
		calls++
		return `{"requirements":[{"type":"functional","description":"Ground decisions","priority":"must-have","source":"Capabilities[0]","supports":["cap_ghost"]}]}`, nil
	}}
	m := Model{client: client, northstarWizard: &NorthstarWizardState{Mission: "Ground decisions", Capabilities: []Capability{{Description: "Evidence", Timeline: "now", Priority: "critical"}}}}
	msg := m.generateRequirementsWithLLM()().(requirementsGeneratedMsg)
	if msg.err == nil || len(msg.requirements) != 0 || !strings.Contains(msg.err.Error(), "unknown capability") {
		t.Fatalf("a dangling link was accepted into the wizard: %+v", msg)
	}
	if calls < 2 {
		t.Fatalf("a refused derivation was not retried (calls=%d)", calls)
	}
}
