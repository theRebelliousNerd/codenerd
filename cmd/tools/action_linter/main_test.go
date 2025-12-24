package main

import (
	"os"
	"path/filepath"
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

	issues := lint(policyActions, routes, virtualActions, false, exemptions{})

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
