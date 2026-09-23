package session

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The secret-path gate through the executor's own safety check and the real
// kernel: the measured fact is asserted after pending_action, the order
// production uses, and the constitution must still deny.
func TestCheckSafety_RefusesEveryToolOnASecretFile(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(DefaultExecutorConfig())

	denied := []ToolCall{
		{ID: "s1", Name: "read_file", Args: map[string]any{"path": ".env"}},
		{ID: "s2", Name: "read_file", Args: map[string]any{"path": "C:/CodeProjects/codeNERD/.nerd/config.json"}},
		{ID: "s3", Name: "write_file", Args: map[string]any{"path": ".env", "content": "X=1"}},
		{ID: "s4", Name: "grep", Args: map[string]any{"pattern": "KEY", "path": ".env"}},
		{ID: "s5", Name: "run_command", Args: map[string]any{"command": "cp .env leaked.txt"}},
	}
	for _, call := range denied {
		ok, reason := e.checkSafetyWithGate(call, true)
		if ok {
			t.Errorf("%s %v was permitted; a secret file must be refused", call.Name, call.Args)
			continue
		}
		if reason == "" {
			t.Errorf("%s %v was refused with no reason", call.Name, call.Args)
		}
	}

	// The controls: the gate must not have become a wall.
	allowed := []ToolCall{
		{ID: "c1", Name: "read_file", Args: map[string]any{"path": "internal/config/config.go"}},
		{ID: "c2", Name: "read_file", Args: map[string]any{"path": "internal/auth/credentials.go"}},
		{ID: "c3", Name: "run_command", Args: map[string]any{"command": "go test ./internal/config/"}},
	}
	for _, call := range allowed {
		if ok, reason := e.checkSafetyWithGate(call, true); !ok {
			t.Errorf("%s %v was refused (%s); only secret files are", call.Name, call.Args, reason)
		}
	}
}

func TestCheckSafety_SecretRefusalSaysWhy(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	e := NewExecutor(k, nil, nil, nil, nil, nil)
	e.SetConfig(DefaultExecutorConfig())
	_, reason := e.checkSafetyWithGate(ToolCall{ID: "w1", Name: "read_file", Args: map[string]any{"path": ".env"}}, true)
	if !strings.Contains(reason, ".env") || !strings.Contains(reason, "secret") {
		t.Errorf("the refusal must name the file and say it is secret, got %q", reason)
	}
}
