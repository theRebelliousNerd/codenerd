package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// journey_conversation_test.go drives the conversation-first main agent the
// way the TUI does: a typed message becomes processInput's command, its
// message goes back through Update, and the next command runs. Perception is
// scripted, so each step is the user's turn as perception would read it; the
// kernel, the embedded policy, the brief, work admission, the background
// runner and the result handling are all real.

// scriptedTransducer returns one scripted intent per perception call, in order.
type scriptedTransducer struct {
	intents []perception.Intent
	calls   int
}

func (s *scriptedTransducer) next() perception.Intent {
	if s.calls >= len(s.intents) {
		return s.intents[len(s.intents)-1]
	}
	in := s.intents[s.calls]
	s.calls++
	return in
}

func (s *scriptedTransducer) ParseIntentWithContext(context.Context, string, []perception.ConversationTurn) (perception.Intent, error) {
	return s.next(), nil
}

func (s *scriptedTransducer) ParseIntent(context.Context, string) (perception.Intent, error) {
	return s.next(), nil
}

func (s *scriptedTransducer) ParseIntentWithGCD(context.Context, string, []perception.ConversationTurn, int) (perception.Intent, []string, error) {
	return s.next(), nil, nil
}

func (s *scriptedTransducer) ResolveFocus(context.Context, string, []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}

func (s *scriptedTransducer) SetPromptAssembler(perception.PromptAssembler) {}

func (s *scriptedTransducer) SetStrategicContext(string) {}

// journeyModel is a chat on a real kernel over a live workspace, in mode,
// with scripted perception and a recording task executor.
func journeyModel(t *testing.T, mode config.WorkMode, intents ...perception.Intent) (Model, *processControlTaskExecutor) {
	t.Helper()
	workspace := SetupLiveWorkspace(t)
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("create kernel: %v", err)
	}
	m := NewTestModel(WithSize(120, 50), WithWorkspace(workspace))
	t.Cleanup(m.shutdownCancel)
	m.kernel = kernel
	m.client = NewMockLLMClient()
	m.transducer = &scriptedTransducer{intents: intents}
	m.virtualStore = core.NewVirtualStore(nil)
	exec := &processControlTaskExecutor{response: "Found two places that build the retry policy."}
	m.taskExecutor = exec
	m.Config.WorkMode = mode
	m.syncWorkModeFacts()
	return m, exec
}

// say runs one user turn to its first message and applies it, returning the
// model and the command Update asked for next.
func say(t *testing.T, m Model, input string) (Model, tea.Msg, tea.Cmd) {
	t.Helper()
	msg := awaitProcessStream(t, m.processInput(input)())
	next, cmd := m.Update(msg)
	return next.(Model), msg, cmd
}

func lastAssistant(m Model) string {
	for i := len(m.history) - 1; i >= 0; i-- {
		if m.history[i].Role == "assistant" {
			return m.history[i].Content
		}
	}
	return ""
}

// Propose mode: the user describes work, the brief builds, the kernel holds
// the work as a proposal, the conversation continues, and "go" starts exactly
// the held work under its own intent.
func TestJourney_ProposeHoldsUntilGoThenRunsTheHeldWork(t *testing.T) {
	fix := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.95,
		Response: "I can fix the typo in README.md.",
		Brief:    &perception.Brief{Goal: "fix the typo in README.md", Ready: true},
	}
	chat := perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9,
		Response: "It is the second line.",
		Brief:    &perception.Brief{Goal: "fix the typo in README.md", Decisions: []string{"the typo is on the second line"}, Ready: true},
	}
	goAhead := perception.Intent{Category: "/query", Verb: "/approve", Target: "none", Confidence: 0.95}
	m, exec := journeyModel(t, config.WorkModePropose, fix, chat, goAhead)

	m, msg, _ := say(t, m, "can you fix the typo in README.md")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("propose mode answered %T, want the work held", msg)
	}
	if m.pendingWork == nil || !strings.Contains(lastAssistant(m), "**Proposed:**") {
		t.Fatalf("no proposal held/shown: pending=%v\n%s", m.pendingWork, lastAssistant(m))
	}
	if exec.callCount() != 0 {
		t.Fatal("work ran before go")
	}
	if got := m.currentBrief(); got == nil || got.Goal != "fix the typo in README.md" {
		t.Fatalf("the brief did not record the goal: %+v", got)
	}

	// A turn of conversation keeps the proposal and updates the brief.
	m, _, _ = say(t, m, "it's on the second line")
	if m.pendingWork == nil {
		t.Fatal("conversation dropped the held work")
	}
	if got := m.currentBrief(); len(got.Decisions) != 1 {
		t.Fatalf("the decision did not reach the brief: %+v", got)
	}

	// "go": the kernel's approve lane, then the held work runs as the fix.
	m, msg, cmd := say(t, m, "go")
	if _, ok := msg.(approveWorkMsg); !ok {
		t.Fatalf("go answered %T, want the approve lane", msg)
	}
	if cmd == nil || m.pendingWork != nil {
		t.Fatalf("go did not start the held work (cmd=%v pending=%v)", cmd != nil, m.pendingWork)
	}
	result := awaitProcessStream(t, cmd())
	am, ok := result.(assistantMsg)
	if !ok {
		t.Fatalf("the held work answered %T, want the delegation's result", result)
	}
	// No verifier in this rig: a mutation is refused fail-closed, which
	// proves it reached the delegate lane with its own intent.
	if !strings.Contains(am.Surface, "no task verifier is configured") {
		t.Fatalf("the held fix did not reach delegation: %q", am.Surface)
	}
}

// Yolo mode: the agent's readiness judgment gates the start. An open question
// holds the work; once the answer settles it, the same request runs.
func TestJourney_YoloWaitsOnAnOpenQuestionThenRuns(t *testing.T) {
	asks := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.95,
		Response: "Which typo do you mean?",
		Brief:    &perception.Brief{Goal: "fix a README typo", OpenQuestions: []string{"which typo?"}},
	}
	answered := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "README.md", Confidence: 0.95,
		Response: "Fixing 'teh' on line 2.",
		Brief:    &perception.Brief{Goal: "fix a README typo", Decisions: []string{"the typo is 'teh' on line 2"}, Ready: true},
	}
	m, _ := journeyModel(t, config.WorkModeYolo, asks, answered)

	m, msg, _ := say(t, m, "fix the readme typo")
	if _, ok := msg.(holdMsg); !ok {
		t.Fatalf("yolo with an open question answered %T, want a hold", msg)
	}
	if !strings.Contains(lastAssistant(m), "which typo?") {
		t.Fatalf("the hold does not name what it waits on:\n%s", lastAssistant(m))
	}

	_, msg, _ = say(t, m, "the 'teh' on line 2")
	am, ok := msg.(assistantMsg)
	if !ok || !strings.Contains(am.Surface, "no task verifier is configured") {
		t.Fatalf("a ready brief in yolo did not run the fix: %T %+v", msg, msg)
	}
}

// Read-only work runs in the background in every mode: the chat answers at
// once, the shard's result arrives as its own message, and the kernel names
// the job while it runs.
func TestJourney_ReviewRunsInTheBackgroundAndReportsBack(t *testing.T) {
	review := perception.Intent{
		Category: "/query", Verb: "/research", Target: "retry policy", Confidence: 0.9,
		Response: "I'll have the researcher look.",
	}
	m, exec := journeyModel(t, config.WorkModeExplicit, review)

	m, msg, cmd := say(t, m, "where do we build the retry policy?")
	start, ok := msg.(backgroundStartMsg)
	if !ok {
		t.Fatalf("read-only work answered %T, want a background start", msg)
	}
	if m.isLoading {
		t.Fatal("the chat is blocked while background work runs")
	}
	if !strings.Contains(m.renderFooter(), "bg: "+start.Job.Shard) {
		t.Fatalf("the footer does not show the running job:\n%s", m.renderFooter())
	}
	if !strings.Contains(injectedLines(t, m), "already working on") {
		t.Fatal("the kernel does not name the running job to the model")
	}

	done := runBatchFor[backgroundDoneMsg](t, cmd)
	next, _ := m.Update(done)
	m = next.(Model)
	if exec.callCount() != 1 {
		t.Fatalf("the shard ran %d times, want once", exec.callCount())
	}
	if !strings.Contains(lastAssistant(m), "Found two places") || m.backgroundFooter() != "" {
		t.Fatalf("the result did not come back, or the job still shows:\n%s", lastAssistant(m))
	}
}

// runBatchFor runs a command (unwrapping tea.Batch) until it yields a T.
func runBatchFor[T tea.Msg](t *testing.T, cmd tea.Cmd) T {
	t.Helper()
	var zero T
	if cmd == nil {
		t.Fatalf("no command to run for %T", zero)
	}
	msg := cmd()
	if got, ok := msg.(T); ok {
		return got
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			if got, ok := c().(T); ok {
				return got
			}
		}
	}
	t.Fatalf("command yielded %T, want %T", msg, zero)
	return zero
}

// "Debug yourself" end to end: the problems log becomes evidence, a reviewer
// root-causes the hot problems in the background, and its findings come back
// as a fix that the work mode holds as a proposal.
func TestJourney_DebugYourselfChainEndsInAProposedFix(t *testing.T) {
	diagnose := perception.Intent{
		Category: "/query", Verb: "/self_diagnose", Target: "codenerd", Confidence: 0.92,
		Response: "Let me look at my own logs.",
	}
	fix := perception.Intent{
		Category: "/mutation", Verb: "/fix", Target: "internal/session/executor.go", Confidence: 0.9,
		Response: "I can fix the nil check the diagnosis found.",
		Brief:    &perception.Brief{Goal: "fix the root cause the self-diagnosis found", Ready: true},
	}
	m, exec := journeyModel(t, config.WorkModePropose, diagnose, fix)
	exec.response = "Root cause: executor.go:120 dereferences a nil session. Fix: guard it."
	writeProblemsLog(t, m.workspace,
		"2026/10/28 06:19:54.100 [ERROR] [session] nil session in executor dispatch: panic recovered\n")

	m, msg, cmd := say(t, m, "debug yourself")
	start, ok := msg.(backgroundStartMsg)
	if !ok {
		t.Fatalf("debug yourself answered %T, want the background diagnosis", msg)
	}
	if !strings.Contains(start.Job.Task, "nil session in executor dispatch") || start.Job.FollowUp == "" {
		t.Fatalf("the diagnosis task lacks the evidence or the fix step:\n%s", start.Job.Task)
	}

	done := runBatchFor[backgroundDoneMsg](t, cmd)
	next, followUp := m.Update(done)
	m = next.(Model)
	if !strings.Contains(lastAssistant(m), "executor.go:120") {
		t.Fatalf("the diagnosis did not come back:\n%s", lastAssistant(m))
	}
	if followUp == nil {
		t.Fatal("the chain stopped before its fix step")
	}
	held := awaitProcessStream(t, followUp())
	if _, ok := held.(holdMsg); !ok {
		t.Fatalf("the fix step answered %T, want it held as a proposal in propose mode", held)
	}
}

func writeProblemsLog(t *testing.T, workspace, content string) {
	t.Helper()
	dir := filepath.Join(workspace, ".nerd", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20261028_061954_problems.log"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
