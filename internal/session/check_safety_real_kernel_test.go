package session

import (
	"codenerd/internal/types"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func TestCheckSafety_GlobAndWriteFile_RealKernel(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(DefaultExecutorConfig())

	cases := []ToolCall{
		{ID: "c1", Name: "glob", Args: map[string]any{"pattern": "**/*.go"}},
		{ID: "c2", Name: "write_file", Args: map[string]any{"path": "cmd/todo/main.go", "content": "package main\n"}},
		{ID: "c3", Name: "read_file", Args: map[string]any{"path": "cmd/todo/main.go"}},
		{ID: "c4", Name: "list_files", Args: map[string]any{"path": "."}},
	}
	for _, tc := range cases {
		ok := e.checkSafety(tc)
		if !ok {
			t.Errorf("checkSafety denied %s args=%v", tc.Name, tc.Args)
		}
	}
}

func TestCheckSafety_SecurityViolationEmitted(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(DefaultExecutorConfig())

	// A completely unsupported action should fail and emit a security_violation fact.
	tc := ToolCall{
		ID:   "v1",
		Name: "unknown_malicious_tool",
		Args: map[string]any{"target": "sensitive.txt"},
	}

	ok := e.checkSafety(tc)
	if ok {
		t.Fatalf("expected checkSafety to deny %s", tc.Name)
	}

	// Verify that the security_violation fact was asserted.
	facts, err := k.Query("security_violation")
	if err != nil {
		t.Fatalf("failed to query security_violation: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("expected at least one security_violation fact to be asserted")
	}

	found := false
	for _, f := range facts {
		if len(f.Args) == 3 {
			// In Mangle, the first argument will be an Atom
			actionType := types.ExtractString(f.Args[0])
			reason := types.ExtractString(f.Args[1])
			if actionType == "/unknown_malicious_tool" && strings.Contains(reason, "action not permitted") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("did not find expected security_violation fact for unknown_malicious_tool")
	}
}
