package autopoiesis

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	internalconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/typesafe"
)

// fakeToolJev answers the tool checks: each noul by name, the choice with
// choice. A non-nil err fails every call.
type fakeToolJev struct {
	mu     sync.Mutex
	nouls  map[string]float64
	choice string
	err    error
	calls  []map[string]typesafe.Question
}

func (f *fakeToolJev) Ask(_ context.Context, _ any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	f.mu.Lock()
	f.calls = append(f.calls, questions)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	answers := map[string]any{}
	for name, q := range questions {
		switch q.(type) {
		case typesafe.Noul:
			answers[name] = map[string]any{"type": "noul", "noul": f.nouls[name]}
		case typesafe.Choice:
			answers[name] = map[string]any{"type": "choice", "choice": f.choice, "confidence": 0.9, "probabilities": map[string]float64{f.choice: 0.9}}
		}
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]any{"input_tokens": 10, "output_tokens": 1}})
	var resp typesafe.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *fakeToolJev) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// countingLLM returns code that passes the safety checker and counts calls.
type countingLLM struct {
	mu      sync.Mutex
	calls   int
	prompts []string
}

const safeToolCode = "package tools\n\nimport \"context\"\n\n// TodoFinder lists TODO comments.\nfunc TodoFinder(ctx context.Context, input string) (string, error) {\n\treturn input, nil\n}\n"

func (c *countingLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

func (c *countingLLM) CompleteWithSystem(_ context.Context, _, user string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.prompts = append(c.prompts, user)
	return "```go\n" + safeToolCode + "```", nil
}

func (c *countingLLM) n() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// jevLoop is a loop over a temp workspace with one registered tool,
// json_validator, and the fake Jev installed under cfg.
func jevLoop(t *testing.T, llm LLMClient, jev *fakeToolJev, cfg *internalconfig.TypeSafeConfig) *OuroborosLoop {
	t.Helper()
	oc := DefaultOuroborosConfig(t.TempDir())
	oc.EnableThunderdome = false
	loop := NewOuroborosLoop(llm, oc)
	loop.SetPromptAssembler(stagePromptAssembler{})
	loop.SetRolePrompts(fixedRolePrompts{})
	loop.registry.mu.Lock()
	loop.registry.tools["json_validator"] = &RuntimeTool{Name: "json_validator", Description: "Reports whether a JSON file is well-formed.", RegisteredAt: time.Now()}
	loop.registry.mu.Unlock()
	if jev != nil {
		loop.SetJev(jev, cfg)
	}
	return loop
}

func onCfg() *internalconfig.TypeSafeConfig { return &internalconfig.TypeSafeConfig{} }

var fastRetry = ExecuteConfig{Retry: RetryConfig{MaxRetries: 2, RetryDelay: time.Millisecond}, MaxIters: 10}

var jsonNeed = func() *ToolNeed {
	return &ToolNeed{Name: "json_checker", Purpose: "Check that a JSON file is valid.", InputType: "path", OutputType: "verdict"}
}

func TestOuroborosJev_DuplicateFoundReusesTheToolAndGeneratesNothing(t *testing.T) {
	llm := &countingLLM{}
	jev := &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.93}, choice: "json_validator"}
	loop := jevLoop(t, llm, jev, onCfg())

	res := loop.ExecuteWithConfig(context.Background(), jsonNeed(), fastRetry)
	if !res.Success || res.ReusedTool != "json_validator" || res.ToolHandle == nil || res.ToolHandle.Name != "json_validator" {
		t.Fatalf("result = %+v, want the existing json_validator reused", res)
	}
	if llm.n() != 0 {
		t.Fatalf("the LLM was called %d times; a reused tool generates nothing", llm.n())
	}
}

// Not found, unsettled, failed and switched off all generate as before.
func TestOuroborosJev_NoDuplicateUnsettledFailedOrOffGenerates(t *testing.T) {
	off := false
	for _, tc := range []struct {
		name     string
		jev      *fakeToolJev
		cfg      *internalconfig.TypeSafeConfig
		wantAsks bool
	}{
		{"none exists", &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.05, "code_does_what_was_asked": 0.9, "code_does_unrequested_things": 0.1}, choice: "json_validator"}, onCfg(), true},
		{"unsettled", &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.5, "code_does_what_was_asked": 0.9, "code_does_unrequested_things": 0.1}, choice: "json_validator"}, onCfg(), true},
		{"jev fails", &fakeToolJev{err: errors.New("503 from typesafe")}, onCfg(), true},
		{"switch off", &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.99}, choice: "json_validator"}, &internalconfig.TypeSafeConfig{ToolChecks: &off}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			llm := &countingLLM{}
			loop := jevLoop(t, llm, tc.jev, tc.cfg)
			res := loop.ExecuteWithConfig(context.Background(), jsonNeed(), fastRetry)
			if res.ReusedTool != "" {
				t.Fatalf("reused %q; this row generates", res.ReusedTool)
			}
			if llm.n() == 0 {
				t.Fatalf("nothing was generated (result %+v)", res)
			}
			if asked := tc.jev.count() > 0; asked != tc.wantAsks {
				t.Fatalf("Jev asked = %v, want %v", asked, tc.wantAsks)
			}
			if res.Stage == StageSafetyCheck && strings.Contains(res.Error, "did not match its request") {
				t.Fatalf("a matching or unjudged tool was refused: %s", res.Error)
			}
		})
	}
}

func TestOuroborosJev_MismatchRegeneratesWithJevFeedback(t *testing.T) {
	llm := &countingLLM{}
	jev := &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.02, "code_does_what_was_asked": 0.04, "code_does_unrequested_things": 0.9}, choice: "json_validator"}
	loop := jevLoop(t, llm, jev, onCfg())

	res := loop.ExecuteWithConfig(context.Background(), jsonNeed(), fastRetry)
	if res.Success || !strings.Contains(res.Error, "did not match its request after 2 attempts") {
		t.Fatalf("result = %+v, want the tool refused after the retry budget", res)
	}
	// One regeneration carried Jev's reasons as its feedback.
	regenerations := 0
	for _, p := range llm.prompts {
		if strings.Contains(p, "does not do what the request asks") && strings.Contains(p, "something the request did not ask for") {
			regenerations++
		}
	}
	if regenerations != 1 {
		t.Fatalf("%d prompts carried Jev's reasons, want the one regeneration; prompts:\n%s", regenerations, strings.Join(llm.prompts, "\n----\n"))
	}
}

func TestOuroborosJev_GenerateFromCodeRefusesADuplicateAndAMismatch(t *testing.T) {
	dup := &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.95}, choice: "json_validator"}
	loop := jevLoop(t, &countingLLM{}, dup, onCfg())
	ok, _, _, msg := loop.GenerateToolFromCode(context.Background(), "json_checker", "Check a JSON file is valid.", safeToolCode, 0.8, 5, false)
	if ok || !strings.Contains(msg, "json_validator already does") {
		t.Fatalf("duplicate: ok=%v msg=%q, want refused naming json_validator", ok, msg)
	}

	bad := &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.05, "code_does_what_was_asked": 0.1, "code_does_unrequested_things": 0.2}, choice: "json_validator"}
	loop = jevLoop(t, &countingLLM{}, bad, onCfg())
	ok, _, _, msg = loop.GenerateToolFromCode(context.Background(), "todo_finder", "List TODO comments with file and line.", safeToolCode, 0.8, 5, false)
	if ok || !strings.Contains(msg, "does not match its purpose") {
		t.Fatalf("mismatch: ok=%v msg=%q, want refused with Jev's reason", ok, msg)
	}
}

// The tool being regenerated under its own name is not its own duplicate.
func TestOuroborosJev_SameNameToolIsNotACandidate(t *testing.T) {
	jev := &fakeToolJev{nouls: map[string]float64{"any_tool_does_this": 0.99}, choice: "json_validator"}
	loop := jevLoop(t, &countingLLM{}, jev, onCfg())
	_, tools, _ := loop.duplicateCandidates("json_validator")
	if len(tools) != 0 {
		t.Fatalf("candidates = %v, want none besides the tool itself", tools)
	}
}

func TestDecideToolChecks_TruthTables(t *testing.T) {
	for _, tc := range []struct {
		anyExists float64
		want      string
	}{{0.7, "json_validator"}, {0.95, "json_validator"}, {0.69, ""}, {0.31, ""}, {0.3, ""}} {
		if got, _ := decideDuplicate(learnjudgeDup(tc.anyExists), 0.7, 0.3); got != tc.want {
			t.Errorf("any=%.2f: reuse %q, want %q", tc.anyExists, got, tc.want)
		}
	}
	for _, tc := range []struct {
		asked, extra float64
		mismatch     bool
	}{{0.9, 0.1, false}, {0.5, 0.5, false}, {0.3, 0.1, true}, {0.9, 0.7, true}, {0.1, 0.9, true}} {
		if got := decideCode(learnjudgeCode(tc.asked, tc.extra), 0.7, 0.3) != ""; got != tc.mismatch {
			t.Errorf("asked=%.2f extra=%.2f: mismatch %v, want %v", tc.asked, tc.extra, got, tc.mismatch)
		}
	}
}

func learnjudgeDup(p float64) learnjudge.DuplicateCheck {
	return learnjudge.DuplicateCheck{AnyExists: p, Tool: "json_validator", ToolProbability: 0.9}
}

func learnjudgeCode(asked, extra float64) learnjudge.CodeCheck {
	return learnjudge.CodeCheck{DoesAsked: asked, DoesExtra: extra}
}

// A written tool keeps its description across a restart: Restore reads the
// header WriteTool puts above the package clause.
func TestWriteTool_DescriptionSurvivesRestore(t *testing.T) {
	dir := t.TempDir()
	compiled := dir + "/.compiled"
	tg := NewToolGenerator(&countingLLM{}, dir)
	tool := &GeneratedTool{Name: "todo_finder", Description: "Lists TODO comments\nwith file and line.", Code: safeToolCode, FilePath: dir + "/todo_finder.go"}
	if err := tg.WriteTool(tool); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(compiled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(compiled+"/todo_finder", []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := NewRuntimeRegistry(dir, compiled)
	reg.Restore(dir, compiled)
	got, ok := reg.Get("todo_finder")
	if !ok || got.Description != "Lists TODO comments with file and line." {
		t.Fatalf("restored = %+v, want the written description", got)
	}
	if again := withDescriptionHeader("// Description: kept\npackage tools\n", "other"); again != "// Description: kept\npackage tools\n" {
		t.Fatalf("an existing header must stay: %q", again)
	}
}
