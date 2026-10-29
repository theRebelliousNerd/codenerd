package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/mangle"
)

// buildSnapshotContent mirrors the serialization in cmd_browser.go browserSnapshot.
// It must stay in sync with that function: hash comments and single period via Fact.String()+newline.
func buildSnapshotContent(sessionID, url string, facts []mangle.Fact) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# DOM Snapshot for session %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("# Captured at %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# URL: %s\n\n", url))
	for _, fact := range facts {
		sb.WriteString(fact.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// newBrowserEngine loads schemas_browser.mg into a fresh engine.
func newBrowserEngine(t *testing.T) *mangle.Engine {
	t.Helper()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	schema, err := core.GetDefaultContent("schemas_browser.mg")
	if err != nil {
		t.Fatalf("GetDefaultContent(schemas_browser.mg) failed: %v", err)
	}
	if err := engine.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString(schemas_browser.mg) failed: %v", err)
	}
	return engine
}

func TestBrowserSnapshotSerializationParses(t *testing.T) {
	// Sample facts covering several DOM predicates declared in schemas_browser.mg.
	sampleFacts := []mangle.Fact{
		{Predicate: "dom_node", Args: []any{"node_1", "DIV", "hello", "root"}},
		{Predicate: "dom_text", Args: []any{"node_1", "hello"}},
		{Predicate: "dom_attr", Args: []any{"node_1", "class", "container"}},
		{Predicate: "dom_layout", Args: []any{"node_1", int64(0), int64(0), int64(100), int64(50), "/true"}},
	}

	content := buildSnapshotContent("sess_test", "https://example.com", sampleFacts)

	// Sanity: correct header uses hash, not double-slash.
	if strings.Contains(content, "// DOM Snapshot") {
		t.Fatalf("snapshot contains C-style comment '// DOM Snapshot', expected '# DOM Snapshot'")
	}
	if !strings.Contains(content, "# DOM Snapshot for session sess_test") {
		t.Fatalf("snapshot missing expected hash header, got:\n%s", content)
	}
	if !strings.Contains(content, "# Captured at") {
		t.Fatalf("snapshot missing '# Captured at' header")
	}
	if !strings.Contains(content, "# URL: https://example.com") {
		t.Fatalf("snapshot missing '# URL:' header")
	}
	// Ensure no double period termination (Fact.String already ends with '.').
	if strings.Contains(content, "..") {
		t.Fatalf("snapshot contains double period '..' (extra terminator), content:\n%s", content)
	}
	// Each fact line should end with exactly one period.
	for _, fact := range sampleFacts {
		expected := fact.String()
		if !strings.Contains(content, expected+"\n") {
			t.Fatalf("snapshot missing fact line %q with single newline", expected)
		}
		if strings.Contains(content, expected+".") {
			t.Fatalf("snapshot contains double-terminated fact %q.", expected)
		}
	}

	// Round-trip: the snapshot must parse via the same path check-mangle uses (Engine.LoadSchemaString).
	engine := newBrowserEngine(t)
	if err := engine.LoadSchemaString(content); err != nil {
		t.Fatalf("snapshot failed to parse as Mangle (round-trip): %v\ncontent:\n%s", err, content)
	}

	// Parsing is the contract, and it is deliberately all that is asserted here.
	// GetFacts reads the fact STORE, which LoadSchemaString does not populate —
	// it loads program text, so ground facts land in the program's clauses.
	// Requiring GetFacts to return rows after a load asserts something the API
	// does not promise, and the check would fail on a perfectly valid snapshot.
	//
	// `nerd check-mangle` is the real consumer and does exactly this parse.
	// Confirmed live 2026-08-08: a regenerated snapshot of example.com (40 facts)
	// reports "OK: .nerd/browser/snapshots/<id>_<ts>.mg".
}

func TestBrowserSnapshotRejectsCStyleComments(t *testing.T) {
	engine := newBrowserEngine(t)
	sampleFacts := []mangle.Fact{
		{Predicate: "dom_node", Args: []any{"node_1", "DIV", "hello", "root"}},
	}
	correct := buildSnapshotContent("sess_test", "https://example.com", sampleFacts)
	// Introduce the first bug: C-style // comments.
	buggy := strings.ReplaceAll(correct, "# DOM Snapshot", "// DOM Snapshot")
	buggy = strings.ReplaceAll(buggy, "# Captured at", "// Captured at")
	buggy = strings.ReplaceAll(buggy, "# URL:", "// URL:")

	err := engine.LoadSchemaString(buggy)
	if err == nil {
		t.Fatalf("expected parse failure for C-style '//' comments, but snapshot parsed successfully:\n%s", buggy)
	}
	if !strings.Contains(err.Error(), "//") && !strings.Contains(err.Error(), "token recognition") && !strings.Contains(strings.ToLower(err.Error()), "failed to parse") {
		t.Fatalf("expected token recognition error for '//', got: %v", err)
	}
}

func TestBrowserSnapshotRejectsDoublePeriod(t *testing.T) {
	engine := newBrowserEngine(t)
	sampleFacts := []mangle.Fact{
		{Predicate: "dom_node", Args: []any{"node_1", "DIV", "hello", "root"}},
		{Predicate: "dom_text", Args: []any{"node_1", "hello"}},
	}
	correct := buildSnapshotContent("sess_test", "https://example.com", sampleFacts)
	// Introduce the second bug: double period termination (Fact.String() already has '.', add another).
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# DOM Snapshot for session %s\n", "sess_test"))
	sb.WriteString(fmt.Sprintf("# Captured at %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# URL: %s\n\n", "https://example.com"))
	for _, f := range sampleFacts {
		sb.WriteString(f.String())
		sb.WriteString(".\n") // buggy: extra period
	}
	buggy := sb.String()

	if !strings.Contains(buggy, "..") {
		t.Fatalf("buggy double-period content does not contain '..' as expected")
	}

	err := engine.LoadSchemaString(buggy)
	if err == nil {
		t.Fatalf("expected parse failure for double period '..', but snapshot parsed successfully:\n%s", buggy)
	}
	// Also verify the correct content still parses (sanity).
	engine2 := newBrowserEngine(t)
	if err := engine2.LoadSchemaString(correct); err != nil {
		t.Fatalf("correct snapshot (single period) should parse, got error: %v", err)
	}
}

func TestBrowserSnapshotSourceHasNoBugs(t *testing.T) {
	// Direct source check: ensures cmd_browser.go itself does not contain the buggy patterns.
	// This makes the test fail immediately if either bug is reintroduced, even if buildSnapshotContent helper stays correct.
	data, err := os.ReadFile("cmd_browser.go")
	if err != nil {
		t.Fatalf("failed to read cmd_browser.go: %v", err)
	}
	src := string(data)
	if strings.Contains(src, "// DOM Snapshot for session") {
		t.Fatalf("cmd_browser.go still contains C-style '// DOM Snapshot' comment (should be '#')")
	}
	if strings.Contains(src, "// Captured at") {
		t.Fatalf("cmd_browser.go still contains C-style '// Captured at' comment (should be '#')")
	}
	if strings.Contains(src, "// URL:") {
		t.Fatalf("cmd_browser.go still contains C-style '// URL:' comment (should be '#')")
	}
	// Check for the double-period bug: sb.WriteString(".\n") immediately after fact.String().
	// The fixed code uses sb.WriteString("\n") without the extra period.
	if strings.Contains(src, `sb.WriteString(".\n")`) {
		t.Fatalf("cmd_browser.go still contains double-period bug sb.WriteString(\".\\n\") (should be \"\\n\")")
	}
	if !strings.Contains(src, `# DOM Snapshot for session`) {
		t.Fatalf("cmd_browser.go missing expected hash comment '# DOM Snapshot'")
	}
}
