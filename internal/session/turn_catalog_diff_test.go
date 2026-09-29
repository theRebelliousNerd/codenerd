package session

import (
	"errors"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// The golden envelopes are the sets the config factory used to hand out, now
// written here because that Go source is gone. turn_tool_allowed derives
// them. A verb that gains or loses a tool fails this test. Counts are the
// factory's: coder 41, tester 46, verify 47, reviewer 35, researcher 40,
// nemesis 36, tool_generator 16, general 13.
//
// bash, run_command and run_check are absent from every set. The policy
// before this projection granted them; the factory never offered them.

func goldenCore() []string {
	return []string{
		"glob", "grep", "list_files", "mcp_call", "mcp_context", "mcp_expand",
		"mcp_map", "mcp_probe", "read_file", "recall_context", "search_code",
		"search_expand", "subagent_expand",
	}
}

func goldenCodeDom() []string {
	return []string{
		"apply_edits", "callees_of", "callers_of", "create_file", "delete_element",
		"delete_lines", "edit_element", "edit_lines", "find_symbol", "find_text",
		"get_element", "get_elements", "importers_of", "insert_element", "insert_lines",
		"package_outline", "predicate_outline", "replace_element", "repoint",
		"unreferenced_symbols",
	}
}

func goldenCodeDomRead() []string {
	return []string{
		"callees_of", "callers_of", "find_symbol", "find_text", "get_element",
		"get_elements", "importers_of", "package_outline", "predicate_outline",
		"unreferenced_symbols",
	}
}

func goldenImpact() []string {
	return []string{"get_impacted_tests", "run_impacted_tests"}
}

func goldenBrowserSession() []string {
	return []string{
		"browser_act", "browser_evidence", "browser_mangle", "browser_observe",
		"browser_reason", "browser_specs", "browser_test", "browser_wait",
	}
}

func goldenEnvelope(persona string) []string {
	switch persona {
	case "general":
		return goldenCore()
	case "coder":
		return unionSorted(goldenCore(), goldenCodeDom(), goldenImpact(), []string{
			"delete_file", "edit_file", "git_operation", "run_build", "run_tests", "write_file",
		})
	case "tester":
		return unionSorted(goldenCore(), goldenCodeDom(), goldenImpact(), goldenBrowserSession(), []string{
			"edit_file", "run_tests", "write_file",
		})
	case "verify":
		return unionSorted(goldenEnvelope("tester"), []string{"grounded_web_search"})
	case "reviewer":
		return unionSorted(goldenCore(), goldenCodeDom(), []string{"git_diff", "git_log"})
	case "researcher":
		return unionSorted(goldenCore(), goldenCodeDomRead(), goldenBrowserSession(), []string{
			"browser_extract", "browser_navigate", "context7_fetch", "grounded_web_search",
			"research_cache_get", "research_cache_set", "web_fetch", "web_search", "write_file",
		})
	case "nemesis":
		return unionSorted(goldenCore(), goldenCodeDom(), []string{"run_build", "run_tests", "write_file"})
	case "tool_generator":
		return unionSorted(goldenCore(), []string{"run_build", "run_tests", "write_file"})
	default:
		return nil
	}
}

func unionSorted(parts ...[]string) []string {
	seen := make(map[string]struct{})
	var all []string
	for _, part := range parts {
		for _, tool := range part {
			if _, ok := seen[tool]; ok {
				continue
			}
			seen[tool] = struct{}{}
			all = append(all, tool)
		}
	}
	slices.Sort(all)
	return all
}

// goldenVerbPersona is every intent the default provider registers, plus the
// edge spellings the consumer normalizes. The value is the envelope key.
func goldenVerbPersona() map[string]string {
	m := map[string]string{}
	add := func(persona string, verbs ...string) {
		for _, v := range verbs {
			m[v] = persona
		}
	}
	add("coder",
		"/fix", "/refactor", "/create", "/write", "/delete", "/debug",
		"/campaign", "/git", "/migrate", "/optimize", "/document",
		"/scaffold", "/format", "/deploy",
		"/implement", "/modify", "/add", "/update")
	add("tester", "/test", "/benchmark", "/profile", "/cover")
	add("verify", "/verify", "/validate")
	add("reviewer",
		"/review", "/review_enhance", "/security", "/analyze", "/audit", "/lint",
		"/check", "/inspect")
	add("researcher",
		"/explore", "/search", "/research", "/init",
		"/learn", "/understand", "/find")
	add("nemesis", "/attack", "/break", "/exploit", "/fuzz", "/pentest", "/nemesis")
	add("tool_generator",
		"/generate_tool", "/generate", "/generate-tool", "/tool_generator", "/create_tool")
	add("general",
		"/general",
		"/explain", "/read", "/stats", "/knowledge", "/help", "/greet",
		"/configure", "/dream", "/shadow", "/assault",
		"/converse", "/forget", "/remember",
		"/requirements_interrogator", "/consult/requirements_interrogator")
	return m
}

// TestTurnCatalog_KernelEnvelope is the golden gate that replaced the
// factory parity check. The expected sets are the factory envelopes; the
// kernel is the only source left.
func TestTurnCatalog_KernelEnvelope(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	verbs := goldenVerbPersona()

	for _, registered := range prompt.NewDefaultConfigAtomProvider().RegisteredIntents() {
		if _, ok := verbs[registered]; !ok {
			t.Errorf("registered intent %s has no golden envelope", registered)
		}
	}
	for verb, persona := range verbs {
		want := goldenEnvelope(persona)
		if want == nil {
			t.Fatalf("persona %s has no envelope", persona)
		}
		got, derr := e.turnDerivedTools(nil, verb)
		if derr != nil {
			t.Errorf("%s: %v", verb, derr)
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s (%s) derived %v, want %v", verb, persona, got, want)
		}
	}

	// The shell and the acceptance check stay out of every persona. Granting
	// one of them is the widening this projection exists to prevent.
	withheld := []string{"bash", "run_command", "run_check"}
	for _, persona := range []string{"general", "coder", "tester", "verify", "reviewer", "researcher", "nemesis", "tool_generator"} {
		got := goldenEnvelope(persona)
		for _, tool := range withheld {
			if slices.Contains(got, tool) {
				t.Errorf("golden %s contains %s", persona, tool)
			}
		}
	}
	// The reviewer inspects with CodeDOM edits and does not receive the
	// whole-file writers. The mandatory-atom gate depends on that.
	reviewer := goldenEnvelope("reviewer")
	if slices.Contains(reviewer, "edit_file") || slices.Contains(reviewer, "write_file") {
		t.Errorf("reviewer envelope widened to whole-file writes: %v", reviewer)
	}
	if !slices.Contains(reviewer, "edit_element") {
		t.Error("reviewer envelope lost edit_element")
	}
	tester := goldenEnvelope("tester")
	verify := goldenEnvelope("verify")
	if slices.Contains(tester, "grounded_web_search") {
		t.Error("tester envelope gained grounded_web_search")
	}
	if !slices.Contains(verify, "grounded_web_search") {
		t.Error("verify envelope lost grounded_web_search")
	}
	if len(verify) != len(tester)+1 {
		t.Errorf("verify len %d, want tester+1 %d", len(verify), len(tester)+1)
	}

	// Normalization the factory used to do inside GetAtom / ResolveAllowedTools.
	general, gerr := e.turnDerivedTools(nil, "/general")
	if gerr != nil {
		t.Fatal(gerr)
	}
	for _, verb := range []string{"", "/FIX", "not an atom", "/consult/", "/verb_invented_next_quarter"} {
		got, derr := e.turnDerivedTools(nil, verb)
		if derr != nil {
			t.Errorf("%q: %v", verb, derr)
			continue
		}
		if !slices.Equal(got, general) {
			t.Errorf("%q = %v, want the /general floor", verb, got)
		}
	}
	if !slices.Contains(general, "read_file") || slices.Contains(general, "write_file") {
		t.Errorf("/general floor = %v, want read_file and not write_file", general)
	}
}

// TestTurnDerivedTools_NilKernelFailsClosed pins the degraded-kernel answer: no
// kernel, no envelope, loudly. The caller fail-closes to an empty catalog.
func TestTurnDerivedTools_NilKernelFailsClosed(t *testing.T) {
	e := NewExecutor(nil, nil, nil, nil, nil, nil)
	if _, err := e.turnDerivedTools(nil, "/fix"); err == nil {
		t.Fatal("nil kernel derived tools; want an error (caller fail-closes)")
	}
}

// TestTurnDerivedTools_QueryErrorFailsClosed pins a kernel that answers the
// catalog query with an error. The empty slice is not a fallback to every
// tool, and it is not the /general floor either: the query did not succeed.
func TestTurnDerivedTools_QueryErrorFailsClosed(t *testing.T) {
	e := &Executor{kernel: &MockKernel{QueryError: errors.New("kernel unavailable")}}
	tools, err := e.turnDerivedTools(nil, "/fix")
	if err == nil {
		t.Fatalf("query error derived %v; want an error", tools)
	}
	if len(tools) != 0 {
		t.Fatalf("query error returned tools %v", tools)
	}
}
