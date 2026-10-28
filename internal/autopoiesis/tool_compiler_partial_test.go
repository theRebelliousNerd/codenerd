package autopoiesis

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
)

// Finding A of Docs/audits/generated-tool-observation-fidelity.md: the
// wrapper encoded a function's result only when it returned no error, so a
// partial result returned beside an error became "output": null and never
// reached the receipt, learning, or the model. Driven through the real
// wrapper and a real build, not a hand-written envelope.
func TestToolCompiler_WrapperKeepsAPartialResultReturnedWithAnError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go-build cross-boundary test in -short mode")
	}
	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.WorkspaceRoot = ""
	tc := NewToolCompiler(cfg)
	res, err := tc.Compile(context.Background(), &GeneratedTool{
		Name: "partial_tool",
		Code: `package tools

import (
	"context"
	"errors"
)

func Execute(ctx context.Context, input string) (string, error) {
	if input == "json" {
		return ` + "`" + `{"done":["a.go"]}` + "`" + `, errors.New("stopped at b.go")
	}
	return "scanned 3 of 5 files", errors.New("permission denied on d.go")
}
`,
	})
	if err != nil || !res.Success {
		t.Fatalf("compile: %v %v", err, res.Errors)
	}
	run := func(input string) (output json.RawMessage, errText string, exitCode int) {
		t.Helper()
		cmd := exec.Command(res.OutputPath)
		in, _ := json.Marshal(map[string]string{"input": input})
		cmd.Stdin = bytes.NewReader(in)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		_ = cmd.Run()
		var env struct {
			Output json.RawMessage `json:"output"`
			Error  string          `json:"error"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("envelope %q: %v", stdout.String(), err)
		}
		return env.Output, env.Error, cmd.ProcessState.ExitCode()
	}

	out, errText, code := run("text")
	if string(out) != `"scanned 3 of 5 files"` || errText != "permission denied on d.go" || code != 1 {
		t.Fatalf("text partial: output=%s error=%q exit=%d", out, errText, code)
	}
	out, errText, code = run("json")
	if string(bytes.Join(bytes.Fields(out), nil)) != `{"done":["a.go"]}` || errText != "stopped at b.go" || code != 1 {
		t.Fatalf("json partial: output=%s error=%q exit=%d", out, errText, code)
	}
}
