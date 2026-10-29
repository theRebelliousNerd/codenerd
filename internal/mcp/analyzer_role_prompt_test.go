package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// testAnalyzerPromptText is the system prompt testAnalyzerPrompts compiles.
const testAnalyzerPromptText = "test mcp tool analyzer role prompt"

// testAnalyzerPrompts answers every role compile with testAnalyzerPromptText.
type testAnalyzerPrompts struct{}

func (testAnalyzerPrompts) CompileRolePrompt(context.Context, types.RolePromptRequest) (string, error) {
	return testAnalyzerPromptText, nil
}

// recordingRolePrompts records the role compile it was asked for.
type recordingRolePrompts struct {
	got types.RolePromptRequest
	err error
}

func (r *recordingRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	r.got = req
	return testAnalyzerPromptText, r.err
}

// systemRecordingLLM records the system prompt and message it was sent.
type systemRecordingLLM struct {
	system, user string
	reply        string
}

func (s *systemRecordingLLM) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	s.system, s.user = system, user
	return s.reply, nil
}

// The analyzer's system prompt is its role compile, and the message is the
// tool: name, description and schemas, with no instructions in it.
func TestAnalyzer_SendsTheRolePromptAndTheToolAsData(t *testing.T) {
	llm := &systemRecordingLLM{reply: `{"categories":["filesystem"],"capabilities":["/read"],"domain":"/go","condensed":"lists"}`}
	rp := &recordingRolePrompts{}
	a := NewToolAnalyzer(llm, rp, nil)
	schema := MCPToolSchema{
		Name:         "list_dir",
		Description:  "List directory contents",
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		OutputSchema: json.RawMessage(`{"type":"array"}`),
	}
	analysis, err := a.Analyze(context.Background(), schema)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if rp.got.Role != "mcp_tool_analyzer" || rp.got.Intent != "judge" {
		t.Errorf("compiled %+v, want role mcp_tool_analyzer intent judge", rp.got)
	}
	if llm.system != testAnalyzerPromptText {
		t.Errorf("system prompt = %q, want the compiled role prompt", llm.system)
	}
	for _, want := range []string{"Name: list_dir", "Description: List directory contents", `"type": "object"`, "Output Schema:", `"type": "array"`} {
		if !strings.Contains(llm.user, want) {
			t.Errorf("the message lacks %q:\n%s", want, llm.user)
		}
	}
	for _, instruction := range []string{"Required Output", "Respond with ONLY", "Shard Affinities", "Analyze this"} {
		if strings.Contains(llm.user, instruction) {
			t.Errorf("the message carries the instruction %q; it belongs to the role prompt", instruction)
		}
	}
	if analysis.Domain != "/go" || analysis.Condensed != "lists" {
		t.Errorf("analysis = %+v, want the model's reply parsed", analysis)
	}
}

// With a model and no role prompt the analyzer cannot ask it: the error
// reaches the caller, and the model is not called with some other prompt.
func TestAnalyzer_RolePromptUnavailableIsAnError(t *testing.T) {
	for name, rp := range map[string]types.RolePromptCompiler{
		"no compiler":    nil,
		"failed compile": &recordingRolePrompts{err: types.ErrRolePromptUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			llm := &systemRecordingLLM{reply: "{}"}
			_, err := NewToolAnalyzer(llm, rp, nil).Analyze(context.Background(), MCPToolSchema{Name: "x"})
			if !errors.Is(err, types.ErrRolePromptUnavailable) {
				t.Errorf("err = %v, want ErrRolePromptUnavailable", err)
			}
			if llm.system != "" || llm.user != "" {
				t.Error("the model was called without the role prompt")
			}
		})
	}
}
