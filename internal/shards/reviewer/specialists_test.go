package reviewer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchSpecialistsForReview_NoRegistry(t *testing.T) {
	files := []string{"test.go"}
	matches := MatchSpecialistsForReview(context.Background(), files, nil)
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches with nil registry, got %d", len(matches))
	}
}

func TestMatchSpecialistsForReview_EmptyRegistry(t *testing.T) {
	files := []string{"test.go"}
	registry := &AgentRegistry{Agents: []RegisteredAgent{}}
	matches := MatchSpecialistsForReview(context.Background(), files, registry)
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches with empty registry, got %d", len(matches))
	}
}

func TestMatchSpecialistsForReview_GoFiles(t *testing.T) {
	// Create temp Go file
	tmpDir, err := os.MkdirTemp("", "specialist_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goFile := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{
				Name:          "GoExpert",
				Type:          "persistent",
				Status:        "ready",
				KnowledgePath: "/fake/path/go_knowledge.db",
			},
			{
				Name:          "MangleExpert",
				Type:          "persistent",
				Status:        "ready",
				KnowledgePath: "/fake/path/mangle_knowledge.db",
			},
		},
	}

	matches := MatchSpecialistsForReview(context.Background(), []string{goFile}, registry)

	// Should match GoExpert for .go file
	found := false
	for _, m := range matches {
		if m.AgentName == "GoExpert" {
			found = true
			if m.Score < 0.3 {
				t.Errorf("Expected GoExpert score >= 0.3, got %f", m.Score)
			}
			if len(m.Files) != 1 {
				t.Errorf("Expected 1 file, got %d", len(m.Files))
			}
		}
	}
	if !found {
		t.Errorf("Expected GoExpert to match .go file, but it didn't")
	}
}

func TestMatchSpecialistsForReview_MangleFiles(t *testing.T) {
	// Create temp Mangle file
	tmpDir, err := os.MkdirTemp("", "specialist_test_mangle")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	mgFile := filepath.Join(tmpDir, "policy.mg")
	err = os.WriteFile(mgFile, []byte(`Decl test_fact(X).
test_rule(X) :- test_fact(X).
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{
				Name:          "MangleExpert",
				Type:          "persistent",
				Status:        "ready",
				KnowledgePath: "/fake/path/mangle_knowledge.db",
			},
		},
	}

	matches := MatchSpecialistsForReview(context.Background(), []string{mgFile}, registry)

	// Should match MangleExpert for .mg file
	found := false
	for _, m := range matches {
		if m.AgentName == "MangleExpert" {
			found = true
			if m.Score < 0.3 {
				t.Errorf("Expected MangleExpert score >= 0.3, got %f", m.Score)
			}
		}
	}
	if !found {
		t.Errorf("Expected MangleExpert to match .mg file, but it didn't")
	}
}

func TestMatchSpecialistsForReview_BubbleTeaImports(t *testing.T) {
	// Create temp file with Bubbletea imports
	tmpDir, err := os.MkdirTemp("", "specialist_test_tui")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tuiFile := filepath.Join(tmpDir, "model.go")
	err = os.WriteFile(tuiFile, []byte(`package chat

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct{}

func (m Model) Init() tea.Cmd { return nil }
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m Model) View() string { return "" }
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{
				Name:          "BubbleTeaExpert",
				Type:          "persistent",
				Status:        "ready",
				KnowledgePath: "/fake/path/bubbletea_knowledge.db",
			},
			{
				Name:          "GoExpert",
				Type:          "persistent",
				Status:        "ready",
				KnowledgePath: "/fake/path/go_knowledge.db",
			},
		},
	}

	matches := MatchSpecialistsForReview(context.Background(), []string{tuiFile}, registry)

	// Should match BubbleTeaExpert due to imports
	foundBT := false
	foundGo := false
	for _, m := range matches {
		if m.AgentName == "BubbleTeaExpert" {
			foundBT = true
		}
		if m.AgentName == "GoExpert" {
			foundGo = true
		}
	}
	if !foundBT {
		t.Errorf("Expected BubbleTeaExpert to match Bubbletea imports, but it didn't")
	}
	if !foundGo {
		t.Errorf("Expected GoExpert to match .go file, but it didn't")
	}
}

func TestMatchSpecialistsForReview_SkipsNonPersistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "specialist_test_skip")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goFile := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goFile, []byte(`package main`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{
				Name:          "GoExpert",
				Type:          "ephemeral", // Not persistent
				Status:        "ready",
				KnowledgePath: "/fake/path/go_knowledge.db",
			},
		},
	}

	matches := MatchSpecialistsForReview(context.Background(), []string{goFile}, registry)

	// Should not match because agent is not persistent
	if len(matches) > 0 {
		t.Errorf("Expected 0 matches for non-persistent agent, got %d", len(matches))
	}
}

func TestMatchSpecialistsForReview_SkipsNotReady(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "specialist_test_status")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goFile := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goFile, []byte(`package main`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	registry := &AgentRegistry{
		Agents: []RegisteredAgent{
			{
				Name:          "GoExpert",
				Type:          "persistent",
				Status:        "training", // Not ready
				KnowledgePath: "/fake/path/go_knowledge.db",
			},
		},
	}

	matches := MatchSpecialistsForReview(context.Background(), []string{goFile}, registry)

	// Should not match because agent is not ready
	if len(matches) > 0 {
		t.Errorf("Expected 0 matches for not-ready agent, got %d", len(matches))
	}
}

func TestParseShardOutput_BasicFinding(t *testing.T) {
	output := `### [SEVERITY: warning] Missing error handling

- **File**: main.go:42
- **Issue**: Error from function call is ignored
- **Recommendation**: Add proper error handling with if err != nil check
`
	findings := ParseShardOutput(output, "GoExpert")

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Severity != "warning" {
		t.Errorf("Expected severity 'warning', got '%s'", f.Severity)
	}
	if f.File != "main.go" {
		t.Errorf("Expected file 'main.go', got '%s'", f.File)
	}
	if f.Line != 42 {
		t.Errorf("Expected line 42, got %d", f.Line)
	}
	if f.ShardSource != "GoExpert" {
		t.Errorf("Expected ShardSource 'GoExpert', got '%s'", f.ShardSource)
	}
}

func TestParseShardOutput_MultipleFindingsWithDifferentSeverities(t *testing.T) {
	output := `### [SEVERITY: critical] SQL Injection vulnerability
- **File**: handler.go:15
- **Issue**: User input directly concatenated into SQL query
- **Recommendation**: Use parameterized queries

### [SEVERITY: error] Unvalidated input
- **File**: handler.go:20
- **Issue**: No validation on user input
- **Recommendation**: Add input validation

### [SEVERITY: info] Consider adding logging
- **File**: handler.go:30
- **Issue**: No logging for debugging
`
	findings := ParseShardOutput(output, "SecurityAuditor")

	if len(findings) != 3 {
		t.Fatalf("Expected 3 findings, got %d", len(findings))
	}

	severities := map[string]bool{"critical": false, "error": false, "info": false}
	for _, f := range findings {
		severities[f.Severity] = true
	}

	for sev, found := range severities {
		if !found {
			t.Errorf("Expected to find severity '%s' in findings", sev)
		}
	}
}

func TestParseShardOutput_EmptyOutput(t *testing.T) {
	findings := ParseShardOutput("", "TestShard")
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for empty output, got %d", len(findings))
	}
}

func TestParseShardOutput_TableFormat(t *testing.T) {
	output := `## Detailed Findings

| Severity | Category | File:Line | Message |
|---|---|---|---|
| 🔴 critical | security | ` + "`cmd/main.go:42`" + ` | SQL injection vulnerability |
| ❌ error | logic | ` + "`internal/core.go:100`" + ` | Nil pointer dereference |
| ⚠️ warning | style | ` + "`utils.go:15`" + ` | Unused variable |
`
	findings := ParseShardOutput(output, "Reviewer")

	if len(findings) != 3 {
		t.Fatalf("Expected 3 findings from table format, got %d", len(findings))
	}

	// Check critical finding
	critical := findings[0]
	if critical.Severity != "critical" {
		t.Errorf("Expected severity 'critical', got '%s'", critical.Severity)
	}
	if critical.Category != "security" {
		t.Errorf("Expected category 'security', got '%s'", critical.Category)
	}
	if critical.File != "cmd/main.go" {
		t.Errorf("Expected file 'cmd/main.go', got '%s'", critical.File)
	}
	if critical.Line != 42 {
		t.Errorf("Expected line 42, got %d", critical.Line)
	}
	if critical.Message != "SQL injection vulnerability" {
		t.Errorf("Expected message 'SQL injection vulnerability', got '%s'", critical.Message)
	}

	// Check error finding
	err := findings[1]
	if err.Severity != "error" {
		t.Errorf("Expected severity 'error', got '%s'", err.Severity)
	}

	// Check warning finding
	warn := findings[2]
	if warn.Severity != "warning" {
		t.Errorf("Expected severity 'warning', got '%s'", warn.Severity)
	}
}

func TestParseShardOutput_NoFindings(t *testing.T) {
	output := `# Review Complete

No issues found. The code looks good!

## Summary
All files passed review.
`
	findings := ParseShardOutput(output, "Reviewer")
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for clean output, got %d", len(findings))
	}
}

func TestGetAllPatterns(t *testing.T) {
	patterns := GetAllPatterns()
	if len(patterns) == 0 {
		t.Error("Expected at least some patterns, got 0")
	}

	// Verify key patterns exist
	foundPatterns := map[string]bool{
		"golang":    false,
		"mangle":    false,
		"bubbletea": false,
	}

	for _, p := range patterns {
		if _, ok := foundPatterns[p.ShardName]; ok {
			foundPatterns[p.ShardName] = true
		}
	}

	for name, found := range foundPatterns {
		if !found {
			t.Errorf("Expected to find pattern '%s' in all patterns", name)
		}
	}
}
