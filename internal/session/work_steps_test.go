package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/sessionlog"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// stepScriptProvider answers the planning call with a fixed plan and plays
// each step's pass from a script keyed by the step's file: the anchor names
// the current step, the provider looks the file up and answers with that
// step's calls in order, then a final sentence. It records every anchor it
// was given and every catalog it was offered.
type stepScriptProvider struct {
	*MockLLMClient
	plan     string
	script   map[string][]types.ToolCall // per step file, the calls to make
	played   map[string]int
	anchors  []string
	catalogs [][]string
	finals   map[string]string
}

func newStepScriptProvider(plan string, script map[string][]types.ToolCall) *stepScriptProvider {
	p := &stepScriptProvider{MockLLMClient: &MockLLMClient{}, plan: plan, script: script, played: map[string]int{}}
	p.MockLLMClient.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) { return p.plan, nil }
	return p
}

// currentStepFile reads the file the executive named for this pass out of
// the anchor, the only place the provider can learn it from.
func currentStepFile(history []types.Message) string {
	for _, m := range history {
		if m.Role != "user" || m.Text == "" {
			continue
		}
		for _, line := range strings.Split(m.Text, "\n") {
			if strings.HasPrefix(line, "Step ") && strings.Contains(line, " of ") && strings.Contains(line, " :: ") {
				after := line[strings.Index(line, ": ")+2:]
				file, _, _ := strings.Cut(after, " :: ")
				return strings.TrimSpace(file)
			}
		}
	}
	return ""
}

func (p *stepScriptProvider) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	p.catalogs = append(p.catalogs, names)
	for _, m := range history {
		if m.Role == "user" && m.Text != "" {
			p.anchors = append(p.anchors, m.Text)
		}
	}
	file := currentStepFile(history)
	calls := p.script[file]
	if i := p.played[file]; i < len(calls) {
		call := calls[i]
		// A call the catalog no longer offers is answered with prose: that is
		// what a provider does when a tool it wanted has left the catalog.
		offered := false
		for _, n := range names {
			if n == call.Name {
				offered = true
				break
			}
		}
		p.played[file] = i + 1
		if offered {
			return &types.LLMToolResponse{ToolCalls: []types.ToolCall{call}}, nil
		}
	}
	if final, ok := p.finals[file]; ok {
		return &types.LLMToolResponse{Text: final}, nil
	}
	return &types.LLMToolResponse{Text: "finished " + file}, nil
}

// newPlannedStepsExecutor runs on the real policy corpus: whether a turn is
// planned is turn_needs_step_plan's decision (turn_steps.mg), from the edit
// sites the brief names. The files the step tests name exist in the
// workspace, so a brief that names them names them as sites.
func newPlannedStepsExecutor(t *testing.T, client types.LLMClient) *Executor {
	t.Helper()
	e := newWorkingLoopExecutor(t, client)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	e.kernel = kernel
	for _, name := range []string{"a.txt", "b.txt", "a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(e.config.WorkspaceRoot, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// An effectful tool needs the executive gate; without one the executor
	// refuses every write.
	e.virtualStore = &testExecutiveStore{}
	return e
}

func registerStepTools(t *testing.T) (readTool, writeTool string) {
	t.Helper()
	readTool, writeTool = "planned_steps_read_probe", "create_file"
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: readTool, Category: tools.CategoryGeneral,
		Execute: func(context.Context, map[string]any) (string, error) { return "observed", nil },
	})
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectWrite, Name: writeTool, Category: tools.CategoryGeneral,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			path, err := tools.ResolveWorkspacePath(ctx, "", args["path"].(string))
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return "", err
			}
			content, _ := args["content"].(string)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return "", err
			}
			return "written " + args["path"].(string), nil
		},
	})
	return readTool, writeTool
}

func TestParseWorkSteps(t *testing.T) {
	plan := "Here is the plan:\n" +
		"STEP internal/a.go :: add the field after line 10\n" +
		"- STEP `internal/b.go` :: call it at line 20\n" +
		"2. STEP internal/a.go :: add the field after line 10\n" +
		"STEP :: no file\n" +
		"STEP internal/c.go\n" +
		"STEP internal/prompt/ :: Verify with: go test ./internal/prompt/ -run TestEmbeddedCorpus\n" +
		"STEP ./internal/prompt/... :: run go test on the package\n" +
		"STEP internal/d.go :: Run the tests named in the task\n" +
		"not a step line\n"
	got := parseWorkSteps("", plan, defaultSessionPolicy.StepPlanMaxSteps)
	if len(got) != 2 {
		t.Fatalf("steps = %+v, want the two well-formed distinct edit steps (a verification is not a step)", got)
	}
	if got[0].File != "internal/a.go" || got[0].Change != "add the field after line 10" {
		t.Errorf("step 1 = %+v", got[0])
	}
	if got[1].File != "internal/b.go" || got[1].Change != "call it at line 20" {
		t.Errorf("step 2 = %+v (the bullet and the backticks must be tolerated)", got[1])
	}
	var many strings.Builder
	for i := 0; i < defaultSessionPolicy.StepPlanMaxSteps+5; i++ {
		many.WriteString("STEP f" + strings.Repeat("x", i) + ".go :: change\n")
	}
	all := parseWorkSteps("", many.String(), defaultSessionPolicy.StepPlanMaxSteps)
	run, beyond := splitPlan(all)
	if len(run) != defaultSessionPolicy.StepPlanMaxSteps || len(beyond) != 5 {
		t.Errorf("unbounded plan: %d run, %d beyond; want the cap %d run and the 5 past it kept as remaining", len(run), len(beyond), defaultSessionPolicy.StepPlanMaxSteps)
	}
}

func TestParseWorkSteps_DropsDirectoryTargets(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "internal", "mangle"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "mangle", "engine.go"), []byte("package mangle\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	plan := "STEP internal/mangle :: A\n" +
		"STEP internal/mangle :: B\n" +
		"STEP internal/mangle/engine.go :: C\n" +
		"STEP internal/mangle/new_file.go :: D\n"
	got := parseWorkSteps(dir, plan, defaultSessionPolicy.StepPlanMaxSteps)
	if len(got) != 2 {
		t.Fatalf("steps = %+v, want only the engine.go and new_file.go steps", got)
	}
	if got[0].File != "internal/mangle/engine.go" || got[0].Change != "C" {
		t.Errorf("step 1 = %+v, want the engine.go step", got[0])
	}
	if got[1].File != "internal/mangle/new_file.go" || got[1].Change != "D" {
		t.Errorf("step 2 = %+v, want the new_file.go step (paths that do not exist yet stay)", got[1])
	}
}

// Two edit sites: the executive runs two passes, each anchored on its own
// step with the earlier step's outcome stated, both files are written, and
// the turn's report lists both as edited.
func TestRunToolLoop_PlannedSteps_RunsEachStepAsItsOwnPass(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it with the greeting\nSTEP b.txt :: create it with the farewell\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	var live []sessionlog.LiveEvent
	ctx := sessionlog.WithAgent(sessionlog.WithLiveProgress(context.Background(), func(_ context.Context, ev sessionlog.LiveEvent) {
		live = append(live, ev)
	}), "coder", "planned")
	resp, toolErrs, err := e.runToolLoop(ctx, "system", "create a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q)", err, toolErrs)
	}
	if resp == nil || resp.Text != "finished b.txt" {
		t.Fatalf("response = %+v, want the last step's closing sentence", resp)
	}
	if len(live) < 3 || live[0].Kind != "plan" || live[1].Kind != "plan" || live[2].Kind != "step" {
		t.Fatalf("plan and current step must precede tool execution: %+v", live)
	}
	var startedSteps []string
	for _, ev := range live {
		if ev.Kind == "step" {
			startedSteps = append(startedSteps, ev.Text)
		}
	}
	if len(startedSteps) != 2 || !strings.Contains(startedSteps[0], "Step 1/2: a.txt") || !strings.Contains(startedSteps[1], "Step 2/2: b.txt") {
		t.Fatalf("live step transitions = %q", startedSteps)
	}
	if result.WriteToolEffects != 2 {
		t.Fatalf("writes = %d, want one per step", result.WriteToolEffects)
	}
	if !strings.Contains(result.StepReport, "Planned steps: 2, edited: 2.") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if !anyContains(client.anchors, "this turn is step 1 of 2") || !anyContains(client.anchors, "Step 1 of 2: a.txt :: create it with the greeting") {
		t.Fatalf("the first pass was not anchored on step 1; anchors: %q", client.anchors)
	}
	if !anyContains(client.anchors, "Done: [1] a.txt :: create it with the greeting (edited)") || !anyContains(client.anchors, "Step 2 of 2: b.txt") {
		t.Fatalf("the second pass was not told step 1 had edited; anchors: %q", client.anchors)
	}
	if !anyContains(client.anchors, "create a.txt and b.txt") {
		t.Fatalf("the task itself must be in every anchor; anchors: %q", client.anchors)
	}
	if n := commitRetryAnchors(client.anchors); n != 0 {
		t.Fatalf("commit-regime retries = %d, want none when every step wrote; anchors: %q", n, client.anchors)
	}
}

// A plan with more steps than session.step_plan_max_steps runs the first
// ones and keeps the rest owed: every step that ran edited, and the task is
// still incomplete, with the step it did not run named.
func TestRunToolLoop_PlannedSteps_StepsBeyondTheBoundStayOwed(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it with the greeting\nSTEP b.txt :: create it with the farewell\nSTEP a.go :: add the package clause\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
		"a.go":  {{ID: "w-g", Name: writeTool, Input: map[string]any{"path": "a.go", "content": "package a"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	e.config.StepPlanMaxSteps = 2
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "create a.txt, b.txt and a.go",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if !errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("err = %v, want ErrStepsIncomplete: the bounded prefix is not the whole plan; report:\n%s", err, result.StepReport)
	}
	if !strings.Contains(err.Error(), "[3] a.go (not run: beyond the step bound)") {
		t.Fatalf("err = %v, want the remaining step named", err)
	}
	if result.WriteToolEffects != 2 || client.played["a.go"] != 0 {
		t.Fatalf("writes = %d, a.go passes = %d: the step past the bound ran", result.WriteToolEffects, client.played["a.go"])
	}
	for _, want := range []string{"Planned steps: 3, edited: 2. Run by this task: 2; remaining beyond the step bound: 1.", "[3] a.go :: add the package clause — not run: beyond the step bound, remaining"} {
		if !strings.Contains(result.StepReport, want) {
			t.Fatalf("report lacks %q:\n%s", want, result.StepReport)
		}
	}
	if !anyContains(client.anchors, "this turn is step 1 of 2") || !anyContains(client.anchors, "1 more step(s) than one task runs") {
		t.Fatalf("anchors = %q", client.anchors)
	}
}

// A step whose first pass only read gets one more pass with reading closed:
// the read tool leaves the catalog, and the edit lands.
func TestRunToolLoop_PlannedSteps_RetriesAStepThatMadeNoEditWithReadingClosed(t *testing.T) {
	readTool, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		// First pass: a read, then prose. Second pass: the read is refused by
		// the catalog, so the script's next call, the write, is what runs.
		"a.txt": {
			{ID: "r-a", Name: readTool, Input: map[string]any{"path": "a.txt"}},
			{ID: "r-a2", Name: readTool, Input: map[string]any{"path": "a.txt"}},
			{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}},
		},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, toolErrs, err := e.runToolLoop(context.Background(), "system", "create a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q)", err, toolErrs)
	}
	if result.WriteToolEffects != 2 {
		t.Fatalf("writes = %d, want both steps edited after the retry; report:\n%s", result.WriteToolEffects, result.StepReport)
	}
	if !anyContains(client.anchors, "This step's first pass made no edit. Reading is closed") {
		t.Fatalf("no retry anchor seen; anchors: %q", client.anchors)
	}
	// Reading closes at dispatch; every pass is offered the same catalog.
	requireStableCatalog(t, client.catalogs)
	if n := commitRetryAnchors(client.anchors); n != 1 {
		t.Fatalf("commit-regime retries = %d, want exactly one; anchors: %q", n, client.anchors)
	}
}

// A step that makes no edit in either pass is reported and fails the turn,
// after the other steps ran.
func TestRunToolLoop_PlannedSteps_ReportsAStepThatNeverEdited(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "create a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if !errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("err = %v, want ErrStepsIncomplete", err)
	}
	if !strings.Contains(err.Error(), "[1] a.txt") {
		t.Fatalf("the error must name the step that made no edit: %v", err)
	}
	if result.WriteToolEffects != 1 {
		t.Fatalf("writes = %d, want the second step's edit despite the first step's failure", result.WriteToolEffects)
	}
	if !strings.Contains(result.StepReport, "[1] a.txt :: create it — no edit") || !strings.Contains(result.StepReport, "[2] b.txt :: create it — edited") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if wrapped := wrapToolLoopError(err); !errors.Is(wrapped, ErrStepsIncomplete) || strings.HasPrefix(wrapped.Error(), "LLM generation failed") {
		t.Fatalf("the turn must surface the incomplete plan as itself, got %v", wrapped)
	}
	if n := commitRetryAnchors(client.anchors); n != 1 {
		t.Fatalf("commit-regime retries = %d, want exactly one for the step that never wrote; anchors: %q", n, client.anchors)
	}
}

// A step whose condition does not hold is reported, not failed: the model
// closes its second pass with NO CHANGE NEEDED evidence and the turn succeeds,
// after the other steps ran.
func TestRunToolLoop_PlannedSteps_AStepThatNeedsNoChangeIsReportedNotFailed(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: move tests only if needed\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	client.finals = map[string]string{"a.txt": "NO CHANGE NEEDED: a.txt has no test calling the unexported name"}
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, toolErrs, err := e.runToolLoop(context.Background(), "system", "create a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q); report:\n%s", err, toolErrs, result.StepReport)
	}
	if result.WriteToolEffects != 1 {
		t.Fatalf("writes = %d, want the second step's edit", result.WriteToolEffects)
	}
	if !strings.Contains(result.StepReport, "[1] a.txt :: move tests only if needed — no change needed") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if !strings.Contains(result.StepReport, "[2] b.txt :: create it — edited") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if n := commitRetryAnchors(client.anchors); n != 1 {
		t.Fatalf("commit-regime retries = %d, want exactly one before the no-change evidence; anchors: %q", n, client.anchors)
	}
}

// A NO CHANGE NEEDED line without evidence still fails: the step made no
// edit and gave no reason, so the turn is incomplete after the other steps ran.
func TestRunToolLoop_PlannedSteps_NoChangeWithoutEvidenceStillFails(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: move tests only if needed\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	client.finals = map[string]string{"a.txt": "NO CHANGE NEEDED:"}
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, _, err := e.runToolLoop(context.Background(), "system", "create a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if !errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("err = %v, want ErrStepsIncomplete", err)
	}
	if n := commitRetryAnchors(client.anchors); n != 1 {
		t.Fatalf("commit-regime retries = %d, want exactly one; anchors: %q", n, client.anchors)
	}
}

func TestNoChangeEvidence(t *testing.T) {
	tests := []struct {
		name string
		note string
		want string
	}{
		{"simple", "NO CHANGE NEEDED: x", "x"},
		{"second line indented", "done.\n  NO CHANGE NEEDED: already at line 4", "already at line 4"},
		{"lowercase prefix is not evidence", "no change needed: x", ""},
		{"empty evidence", "NO CHANGE NEEDED:   ", ""},
		{"empty note", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := noChangeEvidence(tt.note); got != tt.want {
				t.Errorf("noChangeEvidence(%q) = %q, want %q", tt.note, got, tt.want)
			}
		})
	}
}

func TestWorkStepAnchor_NoChangeOfferedOnlyOnRetry(t *testing.T) {
	task := "fix it"
	steps := []workStep{{File: "a.txt", Change: "move tests only if needed"}}
	if got := workStepAnchor(task, steps, nil, 0, true); !strings.Contains(got, "NO CHANGE NEEDED:") {
		t.Fatalf("retry anchor = %q, want it to offer NO CHANGE NEEDED", got)
	}
	if got := workStepAnchor(task, steps, nil, 0, false); strings.Contains(got, "NO CHANGE NEEDED:") {
		t.Fatalf("first-pass anchor = %q, want no NO CHANGE NEEDED offer", got)
	}
}

// A planner that splits an import out of the change that needs it produces
// a step the earlier step has already done. Such a step is reported, not
// failed: the guarantee is that every file the plan names was edited.
func TestRunToolLoop_PlannedSteps_AStepOnAFileAlreadyEditedIsNotAFailure(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: create it with the greeting\nSTEP a.txt :: add the import it needs\nSTEP b.txt :: create it\n"
	client := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "hello"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "bye"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}

	_, toolErrs, err := e.runToolLoop(context.Background(), "system", "create a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q); report:\n%s", err, toolErrs, result.StepReport)
	}
	if result.WriteToolEffects != 2 {
		t.Fatalf("writes = %d, want one per file", result.WriteToolEffects)
	}
	if !strings.Contains(result.StepReport, "[2] a.txt :: add the import it needs — no edit (file edited in step 1)") {
		t.Fatalf("report = %q", result.StepReport)
	}
	if n := commitRetryAnchors(client.anchors); n != 1 {
		t.Fatalf("commit-regime retries = %d, want exactly one for the step whose file was already edited; anchors: %q", n, client.anchors)
	}
}

// A turn that reaches planned steps with no kernel cannot decide the retry
// or the verdict in Go. Production does not: planTurnSteps asks
// briefNeedsStepPlan, which is false without a kernel, so runToolLoop never
// calls this.
func TestRunPlannedSteps_NoKernelDoesNotDecide(t *testing.T) {
	e := &Executor{}
	_, _, err := e.runPlannedSteps(context.Background(), "system", "task",
		[]workStep{{File: "a.txt", Change: "one"}, {File: "b.txt", Change: "two"}},
		&config.EffectiveAgentRuntimeConfig{},
		&prompt.CompilationContext{},
		&ExecutionResult{})
	if err == nil || errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("err = %v, want a refusal to decide, not a Go completeness verdict", err)
	}
}

// stepRepairRestoreKernel models a repair made during the post-edit gate. It
// restores the exact initial bytes as the verifier begins, after the planned
// write has landed and before runPlannedSteps measures its final state.
type stepRepairRestoreKernel struct {
	types.Kernel
	t        *testing.T
	path     string
	initial  []byte
	restored bool
}

func (k *stepRepairRestoreKernel) Query(predicate string) ([]types.Fact, error) {
	if predicate == "turn_next_round" && !k.restored {
		k.restored = true
		current, err := os.ReadFile(k.path)
		if err != nil {
			k.t.Errorf("read step output before simulated repair: %v", err)
		} else if string(current) != "temporary" {
			k.t.Errorf("step output before simulated repair = %q, want temporary", current)
		}
		if err := os.WriteFile(k.path, k.initial, 0o644); err != nil {
			k.t.Errorf("simulated repair restore: %v", err)
		}
	}
	return k.Kernel.Query(predicate)
}

func TestRunPlannedSteps_FinalRepairRestoreIsIncomplete(t *testing.T) {
	_, writeTool := registerStepTools(t)
	client := newStepScriptProvider("", map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "temporary"}}},
	})
	e := newPlannedStepsExecutor(t, client)
	path := filepath.Join(e.config.WorkspaceRoot, "a.txt")
	initial := []byte("initial bytes")
	if err := os.WriteFile(path, initial, 0o644); err != nil {
		t.Fatal(err)
	}
	restoring := &stepRepairRestoreKernel{Kernel: e.kernel, t: t, path: path, initial: initial}
	e.kernel = restoring

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	_, toolErrs, err := e.runPlannedSteps(context.Background(), "system", "edit a.txt",
		[]workStep{{File: "a.txt", Change: "replace its content"}},
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if !errors.Is(err, ErrStepsIncomplete) {
		t.Fatalf("runPlannedSteps: %v (tool errors: %q), want ErrStepsIncomplete; report:\n%s", err, toolErrs, result.StepReport)
	}
	if !restoring.restored {
		t.Fatal("post-edit verifier did not trigger the simulated repair restore")
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != string(initial) {
		t.Fatalf("final a.txt = %q, err=%v; want exact initial bytes %q", got, readErr, initial)
	}
	if !strings.Contains(result.StepReport, "[1] a.txt :: replace its content — edit reverted") {
		t.Fatalf("report = %q, want the Mangle-derived reverted outcome", result.StepReport)
	}

	turn := result.turnAtom()
	netRows, err := e.kernel.Query("step_file_net")
	if err != nil {
		t.Fatal(err)
	}
	if !hasStepNetFact(netRows, turn, 1, "a.txt", "/unchanged") {
		t.Fatalf("step_file_net rows = %+v, want /unchanged after final repair", netRows)
	}
	revertedRows, err := e.kernel.Query("step_reverted")
	if err != nil {
		t.Fatal(err)
	}
	if !hasStepRevertedFact(revertedRows, turn, 1, "a.txt") {
		t.Fatalf("step_reverted rows = %+v, want the planned step", revertedRows)
	}
	verdicts, err := e.kernel.Query("turn_steps_verdict")
	if err != nil {
		t.Fatal(err)
	}
	if !hasTurnVerdict(verdicts, turn, "/incomplete") || hasTurnVerdict(verdicts, turn, "/complete") {
		t.Fatalf("turn_steps_verdict rows = %+v, want only /incomplete", verdicts)
	}
}

// A pass can edit the right package without writing its planner-named target.
// Closure must measure that target as unchanged and reach the policy verdict,
// preserving both the real writes and the target's existence and bytes.
func TestRunPlannedSteps_UnwrittenTargetReachesVerdict(t *testing.T) {
	for _, tt := range []struct {
		name    string
		initial *string
	}{
		{name: "absent target"},
		{name: "empty target", initial: ptrString("")},
		{name: "existing target", initial: ptrString("keep these bytes\n")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, writeTool := registerStepTools(t)
			const target = "planned.txt"
			client := newStepScriptProvider("", map[string][]types.ToolCall{
				target: {
					{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "production edit"}},
					{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "test edit"}},
				},
			})
			e := newPlannedStepsExecutor(t, client)
			path := filepath.Join(e.config.WorkspaceRoot, target)
			if tt.initial != nil {
				if err := os.WriteFile(path, []byte(*tt.initial), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := readFileState(path)
			result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
			_, toolErrs, err := e.runPlannedSteps(context.Background(), "system", "edit the package",
				[]workStep{{File: target, Change: "fix the package"}},
				&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
				&prompt.CompilationContext{ShardID: "probe"}, result)
			// The unchanged named target is still incomplete under the current
			// policy. It must be judged, not abort with a measurement error.
			if !errors.Is(err, ErrStepsIncomplete) {
				t.Fatalf("runPlannedSteps: %v (tool errors: %q), want the policy's ErrStepsIncomplete; report:\n%s", err, toolErrs, result.StepReport)
			}
			if len(toolErrs) != 0 || result.WriteToolEffects != 2 {
				t.Fatalf("tool errors = %q, writes = %d; want two real writes without tool errors", toolErrs, result.WriteToolEffects)
			}
			if after := readFileState(path); !before.Known || after != before {
				t.Fatalf("target state = %+v, want original %+v", after, before)
			}
			if _, ok := result.PreWriteContents[target]; ok {
				t.Fatal("unwritten target acquired a first-write preimage")
			}
			if len(result.WrittenPaths) != 2 || !slices.Contains(result.WrittenPaths, "a.txt") || !slices.Contains(result.WrittenPaths, "b.txt") {
				t.Fatalf("written paths = %q, want only a.txt and b.txt", result.WrittenPaths)
			}
			for file, want := range map[string]string{"a.txt": "production edit", "b.txt": "test edit"} {
				got, readErr := os.ReadFile(filepath.Join(e.config.WorkspaceRoot, file))
				if readErr != nil || string(got) != want {
					t.Fatalf("%s = %q, err=%v; want %q", file, got, readErr, want)
				}
			}
			turn := result.turnAtom()
			netRows, queryErr := e.kernel.Query("step_file_net")
			if queryErr != nil {
				t.Fatal(queryErr)
			}
			if !hasStepNetFact(netRows, turn, 1, target, "/unchanged") {
				t.Fatalf("step_file_net rows = %+v, want /unchanged for the unwritten target", netRows)
			}
			verdicts, queryErr := e.kernel.Query("turn_steps_verdict")
			if queryErr != nil {
				t.Fatal(queryErr)
			}
			if !hasTurnVerdict(verdicts, turn, "/incomplete") || hasTurnVerdict(verdicts, turn, "/complete") {
				t.Fatalf("turn_steps_verdict rows = %+v, want only /incomplete", verdicts)
			}
		})
	}
}

func TestAssertPlannedStepFileNetComparesExistenceAndBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		before PreImage
		final  *string
		want   string
	}{
		{name: "absent to empty is changed", before: PreImage{}, final: ptrString(""), want: "/changed"},
		{name: "empty to absent is changed", before: existed(""), final: nil, want: "/changed"},
		{name: "same bytes are unchanged", before: existed("same"), final: ptrString("same"), want: "/unchanged"},
		{name: "different bytes are changed", before: existed("before"), final: ptrString("after"), want: "/changed"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := filepath.Join(workspace, "a.txt")
			if tt.final != nil {
				if err := os.WriteFile(path, []byte(*tt.final), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			kernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			e := &Executor{kernel: kernel, config: ExecutorConfig{WorkspaceRoot: workspace}}
			turn := types.MangleAtom(fmt.Sprintf("/step_net_case_%d", i))
			result := &ExecutionResult{PreWriteContents: map[string]PreImage{"a.txt": tt.before}}
			steps := []workStep{{File: "a.txt", Edited: true}}
			if err := e.assertPlannedStepFileNet(turn, steps, result); err != nil {
				t.Fatalf("assertPlannedStepFileNet: %v", err)
			}
			rows, err := kernel.Query("step_file_net")
			if err != nil {
				t.Fatal(err)
			}
			if !hasStepNetFact(rows, turn, 1, "a.txt", tt.want) {
				t.Fatalf("step_file_net rows = %+v, want %s", rows, tt.want)
			}
		})
	}
}

func TestAssertPlannedStepFileNetFailsClosedWithoutACompleteMeasurement(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		before  map[string]PreImage
		written []string
		final   func(string) error
		wantErr string
	}{
		{
			name:    "written target with missing preimage",
			before:  map[string]PreImage{},
			written: []string{"a.txt"},
			final:   func(path string) error { return os.WriteFile(path, []byte("after"), 0o644) },
			wantErr: "no first-write preimage",
		},
		{
			name:    "written absent target with nil preimage map",
			written: []string{"a.txt"},
			wantErr: "no first-write preimage",
		},
		{
			name:    "written target with absolute path and missing preimage",
			written: []string{"absolute"},
			final:   func(path string) error { return os.WriteFile(path, []byte("after"), 0o644) },
			wantErr: "no first-write preimage",
		},
		{
			name:    "written target with unknown preimage",
			before:  map[string]PreImage{"a.txt": {Unknown: "permission denied"}},
			written: []string{"a.txt"},
			final:   func(path string) error { return os.WriteFile(path, []byte("after"), 0o644) },
			wantErr: "preimage for \"a.txt\" is unknown",
		},
		{
			name:    "attempted write with unknown preimage",
			before:  map[string]PreImage{"a.txt": {Unknown: "permission denied"}},
			final:   func(path string) error { return os.WriteFile(path, []byte("after"), 0o644) },
			wantErr: "preimage for \"a.txt\" is unknown",
		},
		{
			name:    "unreadable final state",
			before:  map[string]PreImage{"a.txt": existed("before")},
			written: []string{"a.txt"},
			final:   func(path string) error { return os.Mkdir(path, 0o755) },
			wantErr: "final state of \"a.txt\" is unreadable",
		},
		{
			name:    "unreadable unwritten target",
			final:   func(path string) error { return os.Mkdir(path, 0o755) },
			wantErr: "final state of \"a.txt\" is unreadable",
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := t.TempDir()
			if tt.final != nil {
				if err := tt.final(filepath.Join(workspace, "a.txt")); err != nil {
					t.Fatal(err)
				}
			}
			written := tt.written
			if slices.Contains(written, "absolute") {
				written = []string{filepath.Join(workspace, "a.txt")}
			}
			kernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			e := &Executor{kernel: kernel, config: ExecutorConfig{WorkspaceRoot: workspace}}
			turn := types.MangleAtom(fmt.Sprintf("/step_net_failure_%d", i))
			steps := []workStep{{File: "unwritten.txt", Edited: true}, {File: "a.txt", Edited: true}}
			err = e.assertPlannedStepFileNet(turn, steps, &ExecutionResult{PreWriteContents: tt.before, WrittenPaths: written})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("assertPlannedStepFileNet error = %v, want text %q", err, tt.wantErr)
			}
			for _, step := range steps {
				if step.MeasurementError == "" {
					t.Fatal("the report did not retain the measurement failure")
				}
			}
			rows, queryErr := kernel.Query("step_file_net")
			if queryErr != nil {
				t.Fatal(queryErr)
			}
			for _, row := range rows {
				if factTurn(row) == string(turn) {
					t.Fatalf("failed measurement asserted a partial net-state fact: %+v", rows)
				}
			}
		})
	}
}

func ptrString(value string) *string { return &value }

func hasStepNetFact(rows []types.Fact, turn types.MangleAtom, step int, file, net string) bool {
	for _, fact := range rows {
		if factTurn(fact) != string(turn) || len(fact.Args) < 4 {
			continue
		}
		n, ok := factNumber(fact, 1)
		if !ok || n != int64(step) || types.ExtractString(fact.Args[2]) != file {
			continue
		}
		if net == "" || types.ExtractString(fact.Args[3]) == net {
			return true
		}
	}
	return false
}

func hasStepRevertedFact(rows []types.Fact, turn types.MangleAtom, step int, file string) bool {
	for _, fact := range rows {
		if factTurn(fact) != string(turn) || len(fact.Args) < 3 {
			continue
		}
		n, ok := factNumber(fact, 1)
		if ok && n == int64(step) && types.ExtractString(fact.Args[2]) == file {
			return true
		}
	}
	return false
}

func hasTurnVerdict(rows []types.Fact, turn types.MangleAtom, verdict string) bool {
	for _, fact := range rows {
		if factTurn(fact) == string(turn) && len(fact.Args) > 1 && types.ExtractString(fact.Args[1]) == verdict {
			return true
		}
	}
	return false
}

// One STEP line, or none, is the single pass the loop always ran: the task
// text is the anchor, untouched, and no report is written.
func TestRunToolLoop_PlannedSteps_OneStepIsOnePass(t *testing.T) {
	_, writeTool := registerStepTools(t)
	for _, plan := range []string{"STEP a.txt :: create it\n", "", "no plan here"} {
		client := newStepScriptProvider(plan, nil)
		e := newPlannedStepsExecutor(t, client)
		result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
		_, _, err := e.runToolLoop(context.Background(), "system", "create a.txt and b.txt",
			&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
			&prompt.CompilationContext{ShardID: "probe"}, result)
		if err != nil {
			t.Fatalf("plan %q: runToolLoop: %v", plan, err)
		}
		if result.StepReport != "" {
			t.Fatalf("plan %q: a single pass wrote a step report: %q", plan, result.StepReport)
		}
		if len(client.anchors) == 0 || client.anchors[0] != "create a.txt and b.txt" {
			t.Fatalf("plan %q: the anchor must be the task itself, got %q", plan, client.anchors)
		}
	}
}

// A turn that is not write-oriented, or has no write tool, is never planned:
// the planning call is not even made.
func TestRunToolLoop_PlannedSteps_OnlyForChangeTasksWithAWriteTool(t *testing.T) {
	readTool, writeTool := registerStepTools(t)
	planned := 0
	newClient := func() *stepScriptProvider {
		c := newStepScriptProvider("STEP a.txt :: x\nSTEP b.txt :: y\n", nil)
		c.MockLLMClient.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) {
			planned++
			return c.plan, nil
		}
		return c
	}
	cases := []struct {
		name  string
		verb  string
		tools []string
	}{
		{"a read task", "/explain", []string{readTool, writeTool}},
		{"a change task with no write tool", "/fix", []string{readTool}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newClient()
			e := newPlannedStepsExecutor(t, client)
			result := &ExecutionResult{Intent: perception.Intent{Verb: tc.verb}}
			if _, _, err := e.runToolLoop(context.Background(), "system", "do a.txt and b.txt",
				&config.EffectiveAgentRuntimeConfig{AllowedTools: tc.tools},
				&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
				t.Fatalf("runToolLoop: %v", err)
			}
			if planned != 0 {
				t.Fatalf("the planning call was made %d time(s)", planned)
			}
		})
	}
}

// The report reaches the user through the response, after the model's own
// closing sentence, so a task run in steps is described by the ledger and
// not by whatever the last pass claimed.
func TestWorkStepReport_ListsEveryStep(t *testing.T) {
	// CoveredBy is the coverage join's answer, read back before the report.
	// Which step covers which is turn_steps.mg, not this formatter.
	steps := []workStep{
		{File: "a.go", Change: "add the field", Edited: true, Calls: 3, Note: "Added the field.\nmore"},
		{File: "b.go", Change: "call it", Edited: false, Calls: 9, Note: "task unresolved: read_only_stall"},
		{File: "a.go", Change: "add the import", Edited: false, Calls: 2, CoveredBy: 1},
	}
	got := workStepReport(steps, nil)
	for _, want := range []string{
		"Planned steps: 3, edited: 1.",
		"[1] a.go :: add the field — edited (3 tool call(s)): Added the field.",
		"[2] b.go :: call it — no edit (9 tool call(s)): task unresolved: read_only_stall",
		"[3] a.go :: add the import — no edit (file edited in step 1) (2 tool call(s))",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "more") {
		t.Errorf("only the first line of a note belongs in the report:\n%s", got)
	}
}

// commitRetryAnchors counts entries into the commit regime. One entry is
// several anchors: the tool loop re-sends the same step anchor on every
// model round of that pass (the write, then the closing sentence).
func commitRetryAnchors(anchors []string) int {
	const marker = "This step's first pass made no edit. Reading is closed"
	n := 0
	inRetry := false
	for _, a := range anchors {
		hit := strings.Contains(a, marker)
		if hit && !inRetry {
			n++
		}
		inRetry = hit
	}
	return n
}

// planRetryClient fails the first failFirst planning calls with a timeout
// before answering with plan, so the retry in planTurnSteps is exercised.
type planRetryClient struct {
	*MockLLMClient
	calls     int
	failFirst int
	plan      string
}

func (c *planRetryClient) CompleteWithSystem(_ context.Context, _, _ string) (string, error) {
	c.calls++
	if c.calls <= c.failFirst {
		return "", context.DeadlineExceeded
	}
	return c.plan, nil
}

func (c *planRetryClient) CompleteWithToolResults(_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: ""}, nil
}

func TestPlanTurnSteps_RetriesOnceThenFallsBack(t *testing.T) {
	_, writeTool := registerStepTools(t)
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}}
	plan := "STEP a.go :: change one\nSTEP b.go :: change two\n"

	retrying := &planRetryClient{MockLLMClient: &MockLLMClient{}, failFirst: 1, plan: plan}
	e := newPlannedStepsExecutor(t, retrying)
	steps, err := e.planTurnSteps(context.Background(), retrying, "change a.go and b.go", cfg, &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}})
	if err != nil {
		t.Fatalf("planTurnSteps: %v", err)
	}
	if len(steps) != 2 || steps[0].File != "a.go" || steps[1].File != "b.go" {
		t.Fatalf("steps = %+v, want the two planned steps after one retry", steps)
	}
	if retrying.calls != 2 {
		t.Fatalf("planning calls = %d, want exactly 2 (one failure plus one retry)", retrying.calls)
	}

	failing := &planRetryClient{MockLLMClient: &MockLLMClient{}, failFirst: 2, plan: plan}
	e2 := newPlannedStepsExecutor(t, failing)
	steps2, err := e2.planTurnSteps(context.Background(), failing, "change a.go and b.go", cfg, &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}})
	if err != nil {
		t.Fatalf("a planner that fails to answer is a single pass, not an error: %v", err)
	}
	if steps2 != nil {
		t.Fatalf("steps = %+v, want nil after two planning failures", steps2)
	}
	if failing.calls != 2 {
		t.Fatalf("planning calls = %d, want exactly 2 (no third attempt)", failing.calls)
	}
}

// An empty planner answer is not something an identical second request
// fixes: measured 2026-09-17 21:20, both attempts came back empty
// (finish_reason "stop", no content) and cost about 40 s before the task ran
// as one pass. One attempt, then one pass.
func TestPlanTurnSteps_EmptyAnswerIsOnePassNotARetry(t *testing.T) {
	readTool, writeTool := registerStepTools(t)
	client := newStepScriptProvider("", nil)
	planned := 0
	client.MockLLMClient.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) {
		planned++
		return "", errors.New("meta returned an empty completion (model=muse-spark-1.3-contributor finish_reason=stop reasoning_chars=0 output_tokens=2177)")
	}
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	if _, _, err := e.runToolLoop(context.Background(), "system", "fix a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	if planned != 1 {
		t.Fatalf("planning was attempted %d time(s) after an empty answer; want exactly 1", planned)
	}
}

// The planner is told what kind of turn it is dividing, not only the task
// text: on the chat path the task is a one-line summary of the intent, so
// the verb is most of what the planner has.
func TestPlanTurnSteps_PlannerIsToldTheIntentVerb(t *testing.T) {
	readTool, writeTool := registerStepTools(t)
	client := newStepScriptProvider("STEP a.txt :: x", nil)
	asked := ""
	client.MockLLMClient.CompleteWithSystemFunc = func(_ context.Context, _ string, user string) (string, error) {
		asked = user
		return client.plan, nil
	}
	e := newPlannedStepsExecutor(t, client)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	_, _, _ = e.runToolLoop(context.Background(), "system", "fix a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{readTool, writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result)
	if !strings.Contains(asked, "fix a.txt and b.txt") || !strings.Contains(asked, "/fix") {
		t.Fatalf("planner request lacks the task or the verb: %q", asked)
	}
}
