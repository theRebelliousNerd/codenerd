package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/shards/system"
)

func TestLoadExemptionsAndMatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exempt.txt")
	data := []byte("# comment\n/foo*\n /bar\n\n")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write exemptions: %v", err)
	}

	exempt, err := loadExemptions(path)
	if err != nil {
		t.Fatalf("load exemptions: %v", err)
	}
	if len(exempt.Patterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(exempt.Patterns))
	}
	if !exempt.isExempt("/foo123") {
		t.Fatalf("expected foo* exemption to match")
	}
	if !exempt.isExempt("bar") {
		t.Fatalf("expected bar exemption to match")
	}
	if exempt.isExempt("/baz") {
		t.Fatalf("did not expect baz to be exempt")
	}
}

func TestBestRouteForAction(t *testing.T) {
	routes := []system.ToolRoute{
		{ActionPattern: "/foo", ToolName: "tool-foo"},
		{ActionPattern: "/foo_bar", ToolName: "tool-foobar"},
		{ActionPattern: "/bar", ToolName: "tool-bar"},
	}

	route, ok := bestRouteForAction("/foo_bar", routes)
	if !ok {
		t.Fatalf("expected route for /foo_bar")
	}
	if route.ToolName != "tool-foobar" {
		t.Fatalf("expected exact match tool, got %s", route.ToolName)
	}

	route, ok = bestRouteForAction("/foo_bar_baz", routes)
	if !ok {
		t.Fatalf("expected route for /foo_bar_baz")
	}
	if route.ToolName != "tool-foobar" {
		t.Fatalf("expected longest prefix match, got %s", route.ToolName)
	}
}

func TestLintDetectsRoutingMismatches(t *testing.T) {
	policyActions := map[string]actionSources{
		"foo": {
			Action:  "/foo",
			Sources: []string{"policy.mg:next_action"},
		},
		"bar": {
			Action:  "/bar",
			Sources: []string{"policy.mg:next_action"},
		},
	}

	routes := []system.ToolRoute{
		{ActionPattern: "/foo", ToolName: "kernel_internal"},
		{ActionPattern: "/bar", ToolName: "external_tool"},
	}

	virtualActions := map[string]struct{}{
		"foo": {},
	}

	issues := lint(policyActions, routes, virtualActions, false, exemptions{}, nil, nil, nil)

	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d: %+v", len(issues), issues)
	}
}

func TestExtractPolicyActionsAndVirtualStoreTypes(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.mg")
	policy := []byte("next_action(/foo) :- ok.\naction_mapping(/review, /bar).\nrepair_next_action(/baz) :- ok.\n")
	if err := os.WriteFile(policyPath, policy, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	actions, err := extractPolicyActions(dir)
	if err != nil {
		t.Fatalf("extract policy actions: %v", err)
	}
	if _, ok := actions["foo"]; !ok {
		t.Fatalf("expected action foo")
	}
	if _, ok := actions["bar"]; !ok {
		t.Fatalf("expected action bar from action_mapping")
	}
	if _, ok := actions["baz"]; !ok {
		t.Fatalf("expected action baz")
	}

	vsPath := filepath.Join(dir, "virtual_store.go")
	vsData := []byte("const (\n\tActionFoo ActionType = \"foo\"\n\tActionBar ActionType = \"bar_baz\"\n)\n")
	if err := os.WriteFile(vsPath, vsData, 0644); err != nil {
		t.Fatalf("write virtual store file: %v", err)
	}

	types, err := extractVirtualStoreActionTypes(vsPath)
	if err != nil {
		t.Fatalf("extract virtual store actions: %v", err)
	}
	if _, ok := types["foo"]; !ok {
		t.Fatalf("expected foo action type")
	}
	if _, ok := types["bar_baz"]; !ok {
		t.Fatalf("expected bar_baz action type")
	}
}

func TestLintDetectsMissingSafeAction(t *testing.T) {
	policyActions := map[string]actionSources{}
	routes := []system.ToolRoute{}
	virtualActions := map[string]struct{}{}
	registered := map[string]struct{}{"missing_tool": {}}
	safe := map[string]struct{}{}
	issues := lint(policyActions, routes, virtualActions, false, exemptions{}, registered, safe, nil)
	found := false
	for _, it := range issues {
		if it.Action == "/missing_tool" && it.Severity == severityError {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected error for registered tool with no safe_action, got %+v", issues)
	}
}

func TestLintNoErrorWhenSafeActionPresent(t *testing.T) {
	policyActions := map[string]actionSources{}
	routes := []system.ToolRoute{}
	virtualActions := map[string]struct{}{}
	registered := map[string]struct{}{"present_tool": {}}
	safe := map[string]struct{}{"present_tool": {}}
	issues := lint(policyActions, routes, virtualActions, false, exemptions{}, registered, safe, nil)
	for _, it := range issues {
		if it.Action == "/present_tool" {
			t.Fatalf("did not expect issue for tool with safe_action, got %+v", issues)
		}
	}
}

func TestLintExemptedMissingSafeAction(t *testing.T) {
	policyActions := map[string]actionSources{}
	routes := []system.ToolRoute{}
	virtualActions := map[string]struct{}{}
	registered := map[string]struct{}{"exempted_tool": {}}
	safe := map[string]struct{}{}
	exempt, err := loadExemptions("")
	if err != nil {
		t.Fatalf("load exemptions: %v", err)
	}
	// Use direct exemptions struct with pattern matching the tool.
	exempt = exemptions{Patterns: []string{"exempted_tool"}}
	issues := lint(policyActions, routes, virtualActions, false, exempt, registered, safe, nil)
	for _, it := range issues {
		if it.Action == "/exempted_tool" {
			t.Fatalf("exempted tool should not produce error, got %+v", issues)
		}
	}
}

func TestExtractSafeActions(t *testing.T) {
	dir := t.TempDir()
	content := []byte("safe_action(/foo).\nsafe_action(/bar).\n# comment\nsafe_action(/foo). // duplicate\n")
	path := filepath.Join(dir, "policy.mg")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	actions, err := extractSafeActions(dir)
	if err != nil {
		t.Fatalf("extract safe actions: %v", err)
	}
	if _, ok := actions["foo"]; !ok {
		t.Fatalf("expected foo safe action")
	}
	if _, ok := actions["bar"]; !ok {
		t.Fatalf("expected bar safe action")
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 safe actions, got %d: %+v", len(actions), actions)
	}
}

func TestGetRegisteredToolNames(t *testing.T) {
	tools, err := getRegisteredToolNames()
	if err != nil {
		t.Fatalf("get registered tools: %v", err)
	}
	if len(tools) == 0 {
		t.Fatalf("expected at least one registered tool")
	}
	// Spot-check a few known tools.
	for _, name := range []string{"read_file", "write_file", "glob", "get_elements", "run_command"} {
		if _, ok := tools[name]; !ok {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
}

func TestLintNoErrorWhenRequiresPermissionPresent(t *testing.T) {
	policyActions := map[string]actionSources{}
	routes := []system.ToolRoute{}
	virtualActions := map[string]struct{}{}
	registered := map[string]struct{}{"dangerous_tool": {}}
	safe := map[string]struct{}{}
	requires := map[string]struct{}{"dangerous_tool": {}}
	issues := lint(policyActions, routes, virtualActions, false, exemptions{}, registered, safe, requires)
	for _, it := range issues {
		if it.Action == "/dangerous_tool" {
			t.Fatalf("did not expect issue for tool with requires_permission, got %+v", issues)
		}
	}
}

func TestExtractRequiresPermission(t *testing.T) {
	dir := t.TempDir()
	content := []byte("requires_permission(/delete_file).\nrequires_permission(/git_push).\n# comment\nrequires_permission(/delete_file). // duplicate\n")
	path := filepath.Join(dir, "policy.mg")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	actions, err := extractRequiresPermission(dir)
	if err != nil {
		t.Fatalf("extract requires permission: %v", err)
	}
	if _, ok := actions["delete_file"]; !ok {
		t.Fatalf("expected delete_file requires permission")
	}
	if _, ok := actions["git_push"]; !ok {
		t.Fatalf("expected git_push requires permission")
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 requires_permission actions, got %d: %+v", len(actions), actions)
	}
}

func TestLintSkipsWarningForRegisteredModularTool(t *testing.T) {
	// A registered modular tool that policy never emits should NOT produce
	// the unused-executor warning because it is reachable via tools.Global()
	// bypassing RouteAction (internal/session/executor.go:55, :1052).
	policyActions := map[string]actionSources{}
	routes := []system.ToolRoute{}
	virtualActions := map[string]struct{}{"bash": {}}
	registered := map[string]struct{}{"bash": {}}
	safe := map[string]struct{}{"bash": {}}
	issues := lint(policyActions, routes, virtualActions, true, exemptions{}, registered, safe, nil)
	for _, it := range issues {
		if it.Action == "/bash" && it.Severity == severityWarning {
			t.Fatalf("registered modular tool /bash should not produce unused-executor warning, got %+v", issues)
		}
	}
}

func TestLintStillWarnsForNonRegisteredUnusedExecutor(t *testing.T) {
	// An action type that is NOT a registered tool and is never emitted must
	// still produce the unused-executor warning.
	policyActions := map[string]actionSources{}
	routes := []system.ToolRoute{}
	virtualActions := map[string]struct{}{"research": {}}
	registered := map[string]struct{}{"bash": {}}
	safe := map[string]struct{}{"bash": {}}
	issues := lint(policyActions, routes, virtualActions, true, exemptions{}, registered, safe, nil)
	found := false
	for _, it := range issues {
		if it.Action == "/research" && it.Severity == severityWarning {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning for non-registered unused executor /research, got %+v", issues)
	}
}

func TestExtractSafeActionsIgnoresCommentedLine(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# safe_action(/commented_tool).\nsafe_action(/real_tool).\n   # safe_action(/also_commented)\n safe_action(/real_tool). # inline comment should still count real_tool\n")
	path := filepath.Join(dir, "policy.mg")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	actions, err := extractSafeActions(dir)
	if err != nil {
		t.Fatalf("extract safe actions: %v", err)
	}
	if _, ok := actions["commented_tool"]; ok {
		t.Fatalf("commented-out safe_action should not count, got %+v", actions)
	}
	if _, ok := actions["also_commented"]; ok {
		t.Fatalf("commented-out safe_action with leading spaces should not count, got %+v", actions)
	}
	if _, ok := actions["real_tool"]; !ok {
		t.Fatalf("expected real_tool safe action, got %+v", actions)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 safe action (real_tool), got %d: %+v", len(actions), actions)
	}
}

func TestExtractRequiresPermissionIgnoresCommentedLine(t *testing.T) {
	dir := t.TempDir()
	content := []byte("# requires_permission(/commented_tool).\nrequires_permission(/real_tool).\n")
	path := filepath.Join(dir, "policy.mg")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	actions, err := extractRequiresPermission(dir)
	if err != nil {
		t.Fatalf("extract requires permission: %v", err)
	}
	if _, ok := actions["commented_tool"]; ok {
		t.Fatalf("commented-out requires_permission should not count, got %+v", actions)
	}
	if _, ok := actions["real_tool"]; !ok {
		t.Fatalf("expected real_tool requires_permission, got %+v", actions)
	}
}


// TestRealCorpusProducesExpectedCounts validates the live policy corpus without
// locking in the current defect count.
//
// The ten tool names in knownUncovered are a shrinking allowlist of a known
// open defect: ten registered tools currently have no safe_action entry in
// internal/core/defaults/policy/constitution.mg. Removing a name from this
// list as its safe_action fact is added is correct maintenance. This test's
// job is to catch an ELEVENTH uncovered tool appearing, not to preserve the
// ten. Zero errors must pass because zero is the goal state once the corpus
// is fixed. Warning assertions (exactly one warning for /research) are stable
// and are kept exact.
func TestRealCorpusProducesExpectedCounts(t *testing.T) {
	// Shrinking allowlist of the currently-known uncovered tools.
	// Remove entries as safe_action facts are added; do not add unless a
	// tool is legitimately uncovered. An error for a tool outside this set
	// is a new regression.
	knownUncovered := map[string]struct{}{
		"apply_edits":          {},
		"context7_fetch":       {},
		"get_impacted_tests":   {},
		"research_cache_clear": {},
		"research_cache_get":   {},
		"research_cache_set":   {},
		"research_cache_stats": {},
		"run_impacted_tests":   {},
		"web_fetch":            {},
		"web_search":           {},
	}

	// Find mg-root (repo root relative).
	candidatesMg := []string{
		"internal/core/defaults",
		"../../internal/core/defaults",
		"../../../internal/core/defaults",
		"C:/CodeProjects/codeNERD/internal/core/defaults",
	}
	var mgRoot string
	for _, c := range candidatesMg {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			mgRoot = c
			break
		}
	}
	if mgRoot == "" {
		t.Skip("mg-root not found from test cwd")
	}
	candidatesVs := []string{
		"internal/core/virtual_store_types.go",
		"../../internal/core/virtual_store_types.go",
		"../../../internal/core/virtual_store_types.go",
		"C:/CodeProjects/codeNERD/internal/core/virtual_store_types.go",
	}
	var vsPath string
	for _, c := range candidatesVs {
		if _, err := os.Stat(c); err == nil {
			vsPath = c
			break
		}
	}
	if vsPath == "" {
		t.Skip("virtual_store file not found")
	}

	policyActions, err := extractPolicyActions(mgRoot)
	if err != nil {
		t.Fatalf("extractPolicyActions: %v", err)
	}
	virtualActions, err := extractVirtualStoreActionTypes(vsPath)
	if err != nil {
		t.Fatalf("extractVirtualStoreActionTypes: %v", err)
	}
	registeredTools, err := getRegisteredToolNames()
	if err != nil {
		t.Fatalf("getRegisteredToolNames: %v", err)
	}
	safeActions, err := extractSafeActions(mgRoot)
	if err != nil {
		t.Fatalf("extractSafeActions: %v", err)
	}
	requiresPermission, err := extractRequiresPermission(mgRoot)
	if err != nil {
		t.Fatalf("extractRequiresPermission: %v", err)
	}
	routes := system.DefaultRouterConfig().DefaultRoutes

	issues := lint(policyActions, routes, virtualActions, true, exemptions{}, registeredTools, safeActions, requiresPermission)
	var warnCount int
	var warnActions []string
	var unexpected []string
	for _, it := range issues {
		if it.Severity == severityError {
			// Registry-coverage errors are those for a registered tool with
			// no safe_action entry; their Action is "/<tool>". Assert that
			// every such error names a tool in the shrinking allowlist.
			// Zero errors must pass.
			name := strings.TrimPrefix(it.Action, "/")
			if _, ok := knownUncovered[name]; !ok {
				unexpected = append(unexpected, it.Action)
			}
		} else if it.Severity == severityWarning {
			warnCount++
			warnActions = append(warnActions, it.Action)
		}
	}
	if len(unexpected) > 0 {
		t.Fatalf("unexpected registry-coverage error(s) outside shrinking allowlist %v: got %v (all issues: %+v)", knownUncovered, unexpected, issues)
	}
	// No assertion on minimum error count; zero is the goal state.
	if warnCount != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", warnCount, issues)
	}
	if len(warnActions) != 1 || warnActions[0] != "/research" {
		t.Fatalf("expected single warning for /research, got %v", warnActions)
	}
}
