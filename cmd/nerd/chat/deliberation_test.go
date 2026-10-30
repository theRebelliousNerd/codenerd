package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/deliberation"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/sessionlog"
	"codenerd/internal/shards"
	"codenerd/internal/system"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
	"codenerd/internal/verification"
	tea "github.com/charmbracelet/bubbletea"
)

// Only the model boundary and the downstream worker are scripted. Routing,
// sharded evaluation, JIT role selection, registered shards, probes, persistence,
// clarification and verification all use their production implementations.
type deliberationModelClient struct {
	types.LLMClient
	kind   string
	frames []deliberation.Frame
}

func (c *deliberationModelClient) CompleteWithSystem(context.Context, string, string) (string, error) {
	return `{"contract":"judge/v2","status":"pass","confidence":0.95,"explanation":"the delegated attempt completed"}`, nil
}

func (c *deliberationModelClient) CompleteWithTools(_ context.Context, systemPrompt, input string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	var payload struct {
		Frame deliberation.Frame `json:"frame"`
	}
	if err := json.Unmarshal([]byte(input), &payload); err != nil {
		return nil, err
	}
	frame := payload.Frame
	c.frames = append(c.frames, frame)
	var answer any
	if len(frame.Candidates) == 0 {
		if !strings.Contains(systemPrompt, "read-only proposer") {
			return nil, fmt.Errorf("proposer JIT identity missing")
		}
		answer = deliberation.Proposal{Candidates: []deliberation.Candidate{
			{ID: "c1", Brief: "Preserve the request and resume it after the answer", Target: "request.go", SuccessTest: "the saved request resumes", Assumptions: []string{"retain the existing API"}, Evidence: []string{"request.go:1"}},
			{ID: "c2", Brief: "Add an explicit resume entry point", Target: "request.go", SuccessTest: "the saved request resumes", Assumptions: []string{"a new entry point is acceptable"}},
		}}
	} else {
		if !strings.Contains(systemPrompt, "codeNERD's challenger") {
			return nil, fmt.Errorf("challenger JIT identity missing")
		}
		challenge := deliberation.Challenge{Recommendation: "c1", Question: "Should resuming use the existing API or a new entry point?"}
		for _, candidate := range frame.Candidates {
			a := deliberation.Assessment{CandidateID: candidate.ID, Success: true}
			switch c.kind {
			case "values":
				a.Items = []deliberation.Item{{Key: "api-preference", Severity: "critical", Kind: "values", Detail: "API preference"}}
			case "empirical":
				if frame.Previous == nil {
					a.Success = false
					a.Items = []deliberation.Item{{Key: "inspect-request", Severity: "critical", Kind: "data", Detail: "read current behavior",
						Probe: &types.ToolCall{ID: "read-request", Name: "read_file", Input: map[string]any{"path": "request.go"}}}}
				} else if len(frame.Previous.Probes) == 0 || !strings.Contains(frame.Previous.Probes[0].Output, "package request") {
					return nil, fmt.Errorf("measured read result was not relayed")
				}
			case "stalled":
				a.Success = false
				a.Items = []deliberation.Item{{Key: "unresolved-design", Severity: "critical", Kind: "design", Detail: "not yet settled"}}
			}
			challenge.Assessments = append(challenge.Assessments, a)
		}
		answer = challenge
	}
	raw, err := json.Marshal(answer)
	return &types.LLMToolResponse{Text: string(raw)}, err
}

func deliberationFixture(t *testing.T, kind string, mode config.WorkMode) (Model, *deliberationModelClient, *verdictExecutor) {
	t.Helper()
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "request.go"), []byte("package request\n"), 0600); err != nil {
		t.Fatal(err)
	}
	kernel, err := system.NewDomainCortex(workspace)
	if err != nil {
		t.Fatal(err)
	}
	atoms, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := prompt.NewJITPromptCompiler(prompt.WithKernel(system.NewKernelAdapter(kernel)), prompt.WithEmbeddedCorpus(atoms))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	roles, err := prompt.NewRoleCompiler(compiler, config.DefaultJITConfig())
	if err != nil {
		t.Fatal(err)
	}
	client := &deliberationModelClient{LLMClient: NewMockLLMClient(), kind: kind}
	virtual := core.NewVirtualStore(nil)
	if virtual.GetModularTools().Get("read_file") == nil {
		if err := virtual.RegisterModularTool(toolscore.ReadFileTool()); err != nil {
			t.Fatal(err)
		}
	}
	virtual.GetModularTools().SetWorkspaceRoot(workspace)
	manager := coreshards.NewShardManager()
	manager.SetParentKernel(kernel)
	manager.SetLLMClient(client)
	shards.RegisterAllShardFactories(manager, shards.RegistryContext{Kernel: kernel, LLMClient: client,
		VirtualStore: virtual, Workspace: workspace, JITCompiler: compiler, RolePrompts: roles})
	t.Cleanup(manager.StopAll)
	worker := &verdictExecutor{returns: []observation.Return{{Output: "Request continuation preserved.", Outcome: "/done", Workspace: workspace}}}
	m := NewTestModel()
	t.Cleanup(m.shutdownCancel)
	m.workspace, m.sessionID = workspace, "deliberation-test"
	m.kernel, m.client, m.shardMgr, m.virtualStore = kernel, client, manager, virtual
	m.Config.WorkMode = mode
	m.syncWorkModeFacts()
	m.taskExecutor = worker
	m.verifier = verification.NewTaskVerifier(client, nil, compiler, config.DefaultJITConfig())
	m.verifier.SetKernel(kernel)
	m.verifier.SetTaskExecutor(worker)
	m.transducer = &clarifyStubTransducer{intent: perception.Intent{Category: "/mutation", Verb: "/fix", Target: "none", Confidence: 0.4}}
	m.delegatedEvents = make(chan sessionlog.LiveEvent, 512)
	return m, client, worker
}

func TestProcessInputDeliberatesAndProceedsWithAssumption(t *testing.T) {
	m, _, worker := deliberationFixture(t, "ready", config.WorkModeYolo)
	msg := m.processInput("Fix the dropped request. What should hold instead: the answer resumes the request.", 0)()
	result, ok := msg.(assistantMsg)
	if !ok {
		t.Fatalf("got %T: %v", msg, msg)
	}
	if !strings.Contains(result.Surface, "Assumed: retain the existing API") || !strings.Contains(result.Surface, "Policy proof:") {
		t.Fatal(result.Surface)
	}
	if len(worker.sent) != 1 || !strings.Contains(worker.sent[0], "Chosen brief:") || !strings.Contains(worker.sent[0], "What should hold instead:") {
		t.Fatalf("original request and chosen brief not executed: %v", worker.sent)
	}
	if len(m.delegatedEvents) == 0 {
		t.Fatal("no live deliberation events")
	}
	dir, err := sessionlog.SessionDir(m.workspace, m.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "deliberation", "*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("persisted thread missing: %v %v", files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	var thread deliberation.Thread
	if err := json.Unmarshal(raw, &thread); err != nil {
		t.Fatal(err)
	}
	if thread.Status != "ready" || len(thread.Rounds) != 1 || len(thread.Rounds[0].Proof) == 0 {
		t.Fatalf("thread not receipted: %+v", thread)
	}
}

func TestDeliberationValuesForkAsksOnceAndResumesOriginal(t *testing.T) {
	m, client, worker := deliberationFixture(t, "values", config.WorkModePropose)
	first := m.processInput("Fix the dropped request", 0)()
	question, ok := first.(clarificationMsg)
	if !ok {
		t.Fatalf("got %T: %v", first, first)
	}
	if len(question.Options) != 2 || !strings.Contains(question.Options[0], "Recommended") || strings.Count(question.Question, "?") != 1 {
		t.Fatalf("not one concrete options question: %+v", question)
	}
	saved, err := m.deliberator().Pending()
	if err != nil || saved == nil {
		t.Fatalf("pending continuation missing: %v", err)
	}
	// Simulate a restart by loading from disk and answering with no model call.
	calls := len(client.frames)
	saved.Answer = question.Options[0]
	out, err := m.deliberator().Run(context.Background(), saved)
	if err != nil {
		t.Fatal(err)
	}
	if out.Thread.Status != "ready" || len(client.frames) != calls {
		t.Fatalf("choice did not resume the saved round: %+v", out)
	}
	input, intent := m.adoptDeliberation(out)
	if intent.Verb != "/fix" || !strings.Contains(input, "Fix the dropped request") {
		t.Fatal("original request lost")
	}
	// Propose mode still holds mutations; deliberation is not work approval.
	next := m.processInput(input, 0)()
	if _, ok := next.(holdMsg); !ok {
		t.Fatalf("expected normal work admission, got %T", next)
	}
	if len(worker.sent) != 0 {
		t.Fatal("values answer authorized unapproved work")
	}
	pending, err := m.deliberator().Pending()
	if err != nil || pending != nil {
		t.Fatalf("completed question still pending: %v", err)
	}
}

func TestDeliberationValuesForkYoloUsesRecommendation(t *testing.T) {
	m, _, _ := deliberationFixture(t, "values", config.WorkModeYolo)
	msg := m.processInput("Fix the dropped request", 0)()
	answer, ok := msg.(assistantMsg)
	if !ok || !strings.Contains(answer.Surface, "Assumed the recommended preference in yolo mode") {
		t.Fatalf("got %T: %v", msg, msg)
	}
}

func TestDeliberationEmpiricalUsesRealReadToolAndContinues(t *testing.T) {
	m, _, _ := deliberationFixture(t, "empirical", config.WorkModeYolo)
	thread := &deliberation.Thread{Request: "Fix the dropped request", Mode: "yolo"}
	out, err := m.deliberator().Run(context.Background(), thread)
	if err != nil {
		t.Fatal(err)
	}
	if out.Thread.Status != "ready" || len(thread.Rounds) != 2 || thread.Rounds[0].Verdict != "empirical" || len(thread.Rounds[0].Probes) == 0 {
		t.Fatalf("probe did not advance round: %+v", thread)
	}
	if thread.Rounds[0].Probes[0].Error != "" {
		t.Fatal(thread.Rounds[0].Probes[0].Error)
	}
}

func TestDeliberationStopsAtConfiguredBounds(t *testing.T) {
	for _, tc := range []struct {
		name              string
		rounds, unchanged int
		status            string
		count             int
	}{
		{"round cap", 1, 9, "round_cap", 1},
		{"no progress", 9, 1, "no_progress", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _, _ := deliberationFixture(t, "stalled", config.WorkModeYolo)
			m.Config.Routing.DeliberationRoundCap, m.Config.Routing.DeliberationNoProgress = tc.rounds, tc.unchanged
			thread := &deliberation.Thread{Request: "Resolve the design", Mode: "yolo"}
			out, err := m.deliberator().Run(context.Background(), thread)
			if err != nil {
				t.Fatal(err)
			}
			if out.Action != "stopped" || thread.Status != tc.status || len(thread.Rounds) != tc.count {
				t.Fatalf("bounds not obeyed: %+v", thread)
			}
		})
	}
}

func TestClarificationAnswerKeepsOriginalIntentWithoutReperception(t *testing.T) {
	m, _, worker := deliberationFixture(t, "ready", config.WorkModeYolo)
	tr := &clarifyStubTransducer{intent: perception.Intent{Category: "/instruction", Verb: "/approve", Confidence: 0.9}}
	m.transducer = tr
	original := &perception.Intent{Category: "/mutation", Verb: "/fix", Target: "request.go", Confidence: 0.9}
	msg := m.processClarificationResponse("Preserve the existing API and resume the request.", original, "Fix the dropped request", 0)()
	if tr.calls != 0 {
		t.Fatalf("clarification was reclassified %d times", tr.calls)
	}
	if _, ok := msg.(assistantMsg); !ok {
		t.Fatalf("got %T: %v", msg, msg)
	}
	if len(worker.sent) != 1 || !strings.Contains(worker.sent[0], "Fix the dropped request") || !strings.Contains(worker.sent[0], "Preserve the existing API") {
		t.Fatalf("original request dropped: %v", worker.sent)
	}
	if original.Target != "request.go" {
		t.Fatal("answer replaced original target")
	}
}

func TestDeliberationReadEnvelopeCannotExecuteOrWrite(t *testing.T) {
	m, _, _ := deliberationFixture(t, "ready", config.WorkModeYolo)
	reads := deliberation.ReadTools{Kernel: m.kernel, Registry: m.virtualStore.GetModularTools(), ContextBytes: m.Config.GetRoutingConfig().DeliberationContextBytes}
	for _, name := range []string{"write_file", "run_command", "mcp_call", "ask_user"} {
		if _, err := reads.Execute(context.Background(), types.ToolCall{Name: name}); err == nil {
			t.Fatalf("deliberation admitted %s", name)
		}
	}
}

func TestDeliberationPersistedRestrictionRetainsConsent(t *testing.T) {
	m, _, _ := deliberationFixture(t, "values", config.WorkModePropose)
	thread := &deliberation.Thread{Request: "Fix request.go after confirmation", Mode: "propose", Intent: perception.Intent{
		Verb: "/fix", Category: "/mutation", Restriction: perception.RestrictionOf("confirm first", perception.RestrictConfirmFirst)}}
	out, err := m.deliberator().Run(context.Background(), thread)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := m.deliberator().Load(out.Thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Intent.Restriction.Has(perception.RestrictConfirmFirst) {
		t.Fatal("persisted continuation lost confirmation obligation")
	}
}

func TestDeliberateCommandReturnsConvergedDesign(t *testing.T) {
	for _, command := range []string{"/deliberate", "/innovate"} {
		t.Run(command, func(t *testing.T) {
			m, _, worker := deliberationFixture(t, "ready", config.WorkModeYolo)
			next, cmd := m.handleCommand(command + " preserve requests across questions")
			if !next.(Model).isLoading || cmd == nil {
				t.Fatal("command did not start")
			}
			batch, ok := cmd().(tea.BatchMsg)
			if !ok {
				t.Fatal("expected command batch")
			}
			var result *assistantMsg
			for _, run := range batch {
				if run == nil {
					continue
				}
				if msg, ok := run().(assistantMsg); ok {
					copy := msg
					result = &copy
				}
			}
			if result == nil || !strings.Contains(result.Surface, "Decided:") {
				t.Fatalf("command did not return a design: %+v", result)
			}
			if len(worker.sent) != 0 {
				t.Fatal("design command executed development work")
			}
		})
	}
}

func TestDeliberationNeverAsksASecondValuesQuestion(t *testing.T) {
	m, client, _ := deliberationFixture(t, "values", config.WorkModePropose)
	thread := &deliberation.Thread{Request: "Settle the preference", Mode: "propose"}
	out, err := m.deliberator().Run(context.Background(), thread)
	if err != nil || out.Action != "ask_once" {
		t.Fatalf("first question: %+v %v", out, err)
	}
	saved, err := m.deliberator().Load(thread.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved.Answer = "I cannot choose either option yet"
	out, err = m.deliberator().Run(context.Background(), saved)
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != "stop_values" || saved.Status != "unresolved_values" {
		t.Fatalf("asked again: %+v", out)
	}
	if len(client.frames) != 4 {
		t.Fatalf("expected one refinement, got %d role calls", len(client.frames))
	}
}

func TestResolvedYoloTurnIsNotHeldByTheOlderBrief(t *testing.T) {
	m, _, worker := deliberationFixture(t, "ready", config.WorkModeYolo)
	if err := m.kernel.AssertBatch((&perception.Brief{Goal: "Fix the dropped request", OpenQuestions: []string{"Which API?"}}).Facts()); err != nil {
		t.Fatal(err)
	}
	msg := m.processInput("Fix the dropped request", 0)()
	if _, ok := msg.(holdMsg); ok {
		t.Fatal("resolved yolo request was held by stale brief questions")
	}
	if len(worker.sent) != 1 {
		t.Fatalf("resolved request did not execute: %T %v", msg, msg)
	}
}
