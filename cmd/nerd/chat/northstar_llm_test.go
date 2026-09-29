package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func TestNorthstarTUI_RequirementsRejectUnrepresentableLinks(t *testing.T) {
	client := &northstarTUIScriptClient{complete: func(context.Context, string, string) (string, error) {
		return `{"requirements":[{"id":"REQ-001","type":"functional","description":"Ground decisions","priority":"must-have","source":"Capabilities[0]","supports":["cap_1"]}]}`, nil
	}}
	m := Model{client: client, northstarWizard: &NorthstarWizardState{Mission: "Ground decisions", Capabilities: []Capability{{Description: "Evidence", Timeline: "now", Priority: "critical"}}}}
	msg := m.generateRequirementsWithLLM()().(requirementsGeneratedMsg)
	if msg.err == nil || len(msg.requirements) != 0 || !strings.Contains(msg.err.Error(), "cannot preserve") {
		t.Fatal("TUI silently discarded a requirement link")
	}
}
