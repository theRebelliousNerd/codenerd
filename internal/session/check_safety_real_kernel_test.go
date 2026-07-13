package session

import (
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
