package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// fakeRolePrompt is the system prompt fakeRolePrompts compiles for role, so a
// test's model can tell one role's call from another's and from the turn's.
func fakeRolePrompt(role string) string { return "test role prompt: " + role }

// fakeCriticPrompt is the build critic's fake system prompt.
var fakeCriticPrompt = fakeRolePrompt(buildCriticRole)

// fakeRolePrompts is a types.RolePromptCompiler that answers each role with
// fakeRolePrompt(role), or err, and records what it was asked.
type fakeRolePrompts struct {
	err error

	mu   sync.Mutex
	reqs []types.RolePromptRequest
}

func (f *fakeRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	return fakeRolePrompt(req.Role), nil
}

// The critic asks for its own role, under the judge intent, and the system
// prompt it sends is the compiled one.
func TestCriticSystemPrompt_CompilesTheBuildCriticRole(t *testing.T) {
	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	fake := &fakeRolePrompts{}
	e.SetRolePrompts(fake)
	got, err := e.rolePrompt(context.Background(), buildCriticRole)
	if err != nil || got != fakeCriticPrompt {
		t.Fatalf("criticSystemPrompt = %q, %v; want the compiled prompt", got, err)
	}
	if len(fake.reqs) != 1 || fake.reqs[0].Role != "build_critic" || fake.reqs[0].Intent != "judge" {
		t.Errorf("compiled %+v, want role build_critic intent judge", fake.reqs)
	}
}

// No compiler, or a compile that fails, is ErrRolePromptUnavailable: there is
// no Go text to review with instead.
func TestCriticSystemPrompt_UnavailableIsAnError(t *testing.T) {
	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	if _, err := e.rolePrompt(context.Background(), buildCriticRole); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("no compiler: err = %v, want ErrRolePromptUnavailable", err)
	}
	e.SetRolePrompts(&fakeRolePrompts{err: types.ErrRolePromptUnavailable})
	if _, err := e.rolePrompt(context.Background(), buildCriticRole); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("failed compile: err = %v, want ErrRolePromptUnavailable", err)
	}
}

// A clone and a spawned subagent review with the same role compiler as the
// executor they came from.
func TestRolePrompts_ReachClonesAndSubagents(t *testing.T) {
	fake := &fakeRolePrompts{}
	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.SetRolePrompts(fake)
	if e.CloneForTask().currentRolePrompts() != fake {
		t.Error("CloneForTask dropped the role prompt compiler")
	}
	s := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{}, DefaultSpawnerConfig())
	s.SetRolePrompts(fake)
	agent, err := s.Spawn(context.Background(), SpawnRequest{Name: "critic-probe", Task: "probe", Type: SubAgentTypeEphemeral})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if agent.executor.currentRolePrompts() != fake {
		t.Error("a spawned subagent's executor has no role prompt compiler")
	}
}

// The critic's user message is the change and nothing else: its instructions
// and reply contract are the role prompt, never re-sent as Go text.
func TestBuildCriticPrompt_CarriesOnlyTheChange(t *testing.T) {
	p := buildCriticPrompt(
		map[string]string{"a.go": "    1| package p\n"},
		map[string]string{"a.go": "    2| // gone\n"},
		"a.go:10-12", "make Add handle overflow")
	for _, want := range []string{"make Add handle overflow", "File: a.go", "// gone", "a.go:10-12"} {
		if !strings.Contains(p, want) {
			t.Errorf("the message lacks %q:\n%s", want, p)
		}
	}
	for _, instruction := range []string{
		"You are", "Instructions:", "FINDING file.go:123", "NO FINDINGS",
		"Check the comments against the code", "tool output, not opinion", "Every one must be accounted for",
	} {
		if strings.Contains(p, instruction) {
			t.Errorf("the message carries the instruction %q; it belongs to the role prompt", instruction)
		}
	}
}

// planCapturingClient answers a planning call with plan and records the
// system prompt and message it was sent.
type planCapturingClient struct {
	*MockLLMClient
	plan         string
	calls        int
	system, user string
}

func (c *planCapturingClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	c.calls++
	c.system, c.user = system, user
	return c.plan, nil
}

func (c *planCapturingClient) CompleteWithToolResults(context.Context, string, []types.Message, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{}, nil
}

// The step planner's system prompt is its role compile; its message is the
// task and the verb, with no instructions. Without the role prompt planning
// is an error and the model is not asked.
func TestPlanTurnSteps_AsksWithTheWorkStepPlannerRolePrompt(t *testing.T) {
	_, writeTool := registerStepTools(t)
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}}
	client := &planCapturingClient{MockLLMClient: &MockLLMClient{}, plan: "STEP a.go :: change one\nSTEP b.go :: change two\n"}
	e := newPlannedStepsExecutor(t, client)
	fake := &fakeRolePrompts{}
	e.SetRolePrompts(fake)
	steps, err := e.planTurnSteps(context.Background(), client, "change a.go and b.go", cfg, &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}})
	if err != nil || len(steps) != 2 {
		t.Fatalf("planTurnSteps = %+v, %v; want the two planned steps", steps, err)
	}
	if client.system != fakeRolePrompt(workStepPlannerRole) {
		t.Errorf("planning system prompt = %q, want the work_step_planner role compile", client.system)
	}
	if client.user != "Task: change a.go and b.go\nIntent verb: /fix" {
		t.Errorf("planning message = %q, want the task and verb only", client.user)
	}

	e.SetRolePrompts(nil)
	client.calls = 0
	if _, err := e.planTurnSteps(context.Background(), client, "change a.go and b.go", cfg, &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("no role prompt: err = %v, want ErrRolePromptUnavailable", err)
	}
	if client.calls != 0 {
		t.Error("the planner was asked without its role prompt")
	}
}

// The final-report audit asks with its role compile, and its message carries
// the task and report but not the question. Without the role prompt the audit
// cannot run, and the error reaches the turn's settlement rather than the
// turn passing unread.
func TestAuditFinalReport_AsksWithTheAuditorRolePrompt(t *testing.T) {
	client := &auditSystemClient{answer: "COMPLETE"}
	e := NewExecutor(nil, nil, client, nil, nil, nil)
	e.config = DefaultExecutorConfig()
	e.SetRolePrompts(&fakeRolePrompts{})
	result := &ExecutionResult{Response: "Added the line.", WriteToolEffects: 1}
	if err := e.auditFinalReport(context.Background(), "Add the line.", result); err != nil {
		t.Fatalf("auditFinalReport: %v", err)
	}
	if client.system != fakeRolePrompt(finalReportAuditorRole) {
		t.Errorf("audit system prompt = %q, want the final_report_auditor role compile", client.system)
	}
	if strings.Contains(client.user, "Answer INCOMPLETE") || strings.Contains(client.user, "Does the report") {
		t.Errorf("the audit message carries the question; it belongs to the role prompt:\n%s", client.user)
	}
	e.SetRolePrompts(nil)
	client.calls = 0
	unaudited := &ExecutionResult{Response: "Added the line.", WriteToolEffects: 1}
	if err := e.auditFinalReport(context.Background(), "Add the line.", unaudited); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("no role prompt: err = %v, want ErrRolePromptUnavailable", err)
	}
	if client.calls != 0 {
		t.Error("the auditor was asked without its role prompt")
	}
}

// auditSystemClient records the audit's system prompt and message.
type auditSystemClient struct {
	MockLLMClient
	answer       string
	calls        int
	system, user string
}

func (c *auditSystemClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	c.calls++
	c.system, c.user = system, user
	return c.answer, nil
}

// roleAtomText is the text of the named role atoms as the binary embeds them.
func roleAtomText(t *testing.T, ids ...string) string {
	t.Helper()
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	var b strings.Builder
	for _, id := range ids {
		atom, ok := corpus.Get(id)
		if !ok {
			t.Fatalf("the embedded corpus has no atom %s", id)
		}
		b.WriteString(atom.Content)
		b.WriteString("\n")
	}
	return b.String()
}

// buildCriticContract is the build critic's role atoms: what the reviewer is
// told to do with the message buildCriticPrompt builds.
func buildCriticContract(t *testing.T) string {
	t.Helper()
	return roleAtomText(t, "system/build_critic/identity", "system/build_critic/review_contract",
		"system/build_critic/comment_check", "system/build_critic/tool_evidence")
}
