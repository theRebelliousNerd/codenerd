package session

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

func TestCheckSafety_WriteFileLargeContent(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(DefaultExecutorConfig())
	content := "package todo\n\n" + strings.Repeat("// line\n", 200) + "type Task struct{ ID int; Title string }\n"
	ok := e.checkSafety(ToolCall{
		ID:   "w1",
		Name: "write_file",
		Args: map[string]any{"path": "internal/todo/store.go", "content": content},
	})
	if !ok {
		t.Fatal("expected write_file with multi-line content to be permitted")
	}
}
