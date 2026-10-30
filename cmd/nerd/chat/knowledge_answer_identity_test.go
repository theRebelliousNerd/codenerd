package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/store"
	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

const knowledgeIdentityQuestion = "what does the first line of app.log say?"
const knowledgeIdentityInterim = "INTERIM-581: Let me consult the specialist."
const knowledgeIdentityFinal = "FINAL-ANSWER-742: The first line is a log header."

// The production client boundary, with deterministic failure modes for the
// terminal call only. Initial articulation still uses the normal scripted stream.
type knowledgeIdentityClient struct {
	*MockLLMClient
	mode        string // protected by MockLLMClient.mu
	systemCalls int
}

func (c *knowledgeIdentityClient) CompleteWithSystem(ctx context.Context, system, prompt string) (string, error) {
	c.mu.Lock()
	mode := c.mode
	c.systemCalls++
	c.mu.Unlock()
	switch mode {
	case "panic":
		panic("SYNTHESIS-PANIC-619")
	case "canceled":
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("fixture expected canceled synthesis context")
	default:
		return c.MockLLMClient.CompleteWithSystem(ctx, system, prompt)
	}
}

// This fixture uses admission, real routing, workspace acquisition,
// articulation and consultation before returning Update's synthesis command.
// No new message constructor or synthesis signature is needed by these tests:
// the same witness can be applied to the pre-fix production implementation.
func beginKnowledgeIdentityTurn(t *testing.T, withDB bool) (Model, tea.Cmd, *knowledgeIdentityClient, *processControlTaskExecutor) {
	t.Helper()
	intent := evidenceIntent("app.log", 0.3)
	nextIntent := perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, IsQuestion: true, Response: "SECOND-ANSWER-315"}
	m, worker := journeyModel(t, config.WorkModePropose, intent, nextIntent)
	m.sessionID = "knowledge-answer-identity"
	if err := os.WriteFile(filepath.Join(m.workspace, "app.log"), []byte("HEAD-SENTINEL-8812 log header\n"+strings.Repeat("filler line\n", 20000)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.workspace, "main.go"), []byte("package main\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m.scanner = world.NewScanner()
	kernel := &processControlRecordingKernel{RealKernel: m.kernel.(*core.RealKernel)}
	m.kernel = kernel
	client := &knowledgeIdentityClient{MockLLMClient: NewMockLLMClient()}
	client.SetDefaultResponse(articulationResponseForTest(t, knowledgeIdentityInterim, intent,
		[]string{`observation("knowledge_identity_probe", "applied once").`}, nil, false,
		[]map[string]string{{"specialist": "researcher", "query": "explain this log header", "purpose": "answer the first-line question", "priority": "required"}}))
	m.client = client
	worker.response = "SPECIALIST-926: This is a generic log header."
	if withDB {
		db, err := store.NewLocalStore(":memory:")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		m.localDB = db
		m.virtualStore.SetLocalDB(db)
	}
	m, cmd := typeAndEnter(t, m, knowledgeIdentityQuestion)
	request := m.lastRequest()
	if request == 0 {
		t.Fatal("normal submission has no request identity")
	}
	gathered := turnEnd(t, cmd)
	owned, ok := gathered.(answerMsg)
	if !ok || owned.Request != request {
		t.Fatalf("consultation result lost request %d: %#v", request, gathered)
	}
	if _, ok := owned.Msg.(knowledgeGatheredMsg); !ok {
		t.Fatalf("normal request returned %T, want consultation handoff", owned.Msg)
	}
	next, cmd := m.Update(gathered)
	m = next.(Model)
	if !m.isLoading || !m.awaitingKnowledge || m.turnCount != 0 || m.answering != 0 {
		t.Fatalf("interim settled the request or leaked attribution: loading=%v awaiting=%v turns=%d answering=%d", m.isLoading, m.awaitingKnowledge, m.turnCount, m.answering)
	}
	if facts, err := m.kernel.Query("file_topology"); err != nil || len(facts) == 0 {
		t.Fatalf("normal workspace context was not acquired: %v, %v", facts, err)
	}
	client.SetDefaultResponse(knowledgeIdentityFinal)
	return m, cmd, client, worker
}

// All persistence commands go through the same snapshot writer as Bubble Tea.
// Ordered execution isolates the identity defect from concurrent save ordering.
func persistKnowledgeIdentityCommands(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("terminal result scheduled no persistence")
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, child := range msg {
			if child != nil {
				persistKnowledgeIdentityCommands(t, child)
			}
		}
	case sessionStatePersistedMsg:
		if msg.err != nil {
			t.Fatalf("persist history: %v", msg.err)
		}
	default:
		t.Fatalf("expected persistence only, got %T", msg)
	}
}

func assertKnowledgeIdentityStored(t *testing.T, m Model, request uint64, terminal string) {
	t.Helper()
	// Both the live transcript and serialized raw history keep explicit ownership.
	matched := 0
	for _, h := range m.history {
		if h.Role == "assistant" && strings.Contains(h.Content, terminal) {
			matched++
			if h.Answers != request {
				t.Errorf("terminal %q answers %d, want %d", terminal, h.Answers, request)
			}
		}
	}
	if matched != 1 {
		t.Fatalf("terminal appears %d times in transcript", matched)
	}
	raw, err := session.LoadSessionHistory(m.workspace, m.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	matched = 0
	for _, h := range raw.Messages {
		if h.Role == "assistant" && strings.Contains(h.Content, terminal) {
			matched++
			if h.Answers != request {
				t.Errorf("serialized terminal answers %d, want %d", h.Answers, request)
			}
		}
	}
	if matched != 1 {
		t.Fatalf("terminal appears %d times in serialized history", matched)
	}
	if m.localDB == nil {
		return
	}
	facts, err := m.virtualStore.QuerySession(m.sessionID, 20)
	if err != nil {
		t.Fatal(err)
	}
	matched = 0
	for _, f := range facts {
		response := fmt.Sprint(f.Args[3])
		if fmt.Sprint(f.Args[2]) == knowledgeIdentityQuestion {
			matched++
			if !strings.Contains(response, knowledgeIdentityInterim) || !strings.Contains(response, terminal) || strings.Contains(response, "UNRELATED-104") || strings.Contains(response, "Queued") || strings.Contains(response, "SECOND-ANSWER-315") {
				t.Errorf("originating request's queryable answer = %q", response)
			}
			if strings.Count(response, terminal) != 1 {
				t.Errorf("terminal missing or duplicated in saved exchange: %q", response)
			}
		} else if strings.Contains(response, terminal) {
			t.Errorf("terminal assigned to another request: %+v", f)
		}
	}
	if matched != 1 {
		t.Fatalf("originating request has %d queryable exchanges", matched)
	}
}

func TestKnowledgeAnswerIdentity_SubmissionThroughQueryableHistory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		db     bool
		queued bool
	}{{name: "one request", db: true}, {name: "queued request and unrelated output", db: true, queued: true}, {name: "no local database"}} {
		t.Run(tc.name, func(t *testing.T) {
			m, synthesis, client, worker := beginKnowledgeIdentityTurn(t, tc.db)
			request := m.lastRequest()
			if tc.queued {
				var cmd tea.Cmd
				m, cmd = typeAndEnter(t, m, "where are its tests?")
				if cmd != nil || m.turn.queuedCount() != 1 || m.lastRequest() == request {
					t.Fatal("second request did not queue with a distinct identity")
				}
			}
			if tc.queued {
				next, _ := m.Update(sideTurnMsg{Reply: "UNRELATED-104", Action: "/converse"})
				m = next.(Model)
			}
			before := client.GetCallCount()
			kernel := m.kernel.(*processControlRecordingKernel)
			asserts, loads, retracts := kernel.counts("observation")
			if asserts != 1 {
				t.Fatalf("fixture did not apply one control observation: %d", asserts)
			}
			next, after := m.Update(turnEnd(t, synthesis))
			m = next.(Model)
			if got := client.GetCallCount() - before; got != 1 || worker.callCount() != 1 {
				t.Fatalf("synthesis calls=%d consultations=%d, want one of each", got, worker.callCount())
			}
			prompt := client.GetLastPrompt()
			for _, want := range []string{"HEAD-SENTINEL-8812", "PARTIAL", "SPECIALIST-926"} {
				if !strings.Contains(prompt, want) {
					t.Errorf("terminal answering request lost %q", want)
				}
			}
			if a, l, r := kernel.counts("observation"); a != asserts || l != loads || r != retracts {
				t.Fatal("synthesis replayed control effects")
			}
			if m.awaitingKnowledge || m.turnCount != 1 || m.answering != 0 {
				t.Fatalf("terminal lifecycle incorrect: awaiting=%v turns=%d answering=%d", m.awaitingKnowledge, m.turnCount, m.answering)
			}
			if tc.queued {
				// Update's batch saves the first answer and starts the second turn.
				// The second uses the ordinary direct-answer path, with no consultation.
				next, save := m.Update(turnEnd(t, after))
				m = next.(Model)
				persistKnowledgeIdentityCommands(t, save)
				rows, err := m.localDB.GetSessionHistory(m.sessionID, 20)
				if err != nil || len(rows) != 2 {
					t.Fatalf("two requests: rows=%v err=%v", rows, err)
				}
				for _, row := range rows {
					if row["user_input"] == "where are its tests?" {
						response := fmt.Sprint(row["response"])
						if !strings.Contains(response, "SECOND-ANSWER-315") || strings.Contains(response, knowledgeIdentityFinal) || strings.Contains(response, "Queued") {
							t.Errorf("second request's response = %q", response)
						}
					}
				}
			} else {
				persistKnowledgeIdentityCommands(t, after)
			}
			assertKnowledgeIdentityStored(t, m, request, knowledgeIdentityFinal)
			// Another complete save must retain the terminal content exactly once.
			persistKnowledgeIdentityCommands(t, m.saveSessionStateCmd())
			assertKnowledgeIdentityStored(t, m, request, knowledgeIdentityFinal)
		})
	}
}

func TestKnowledgeAnswerIdentity_TerminalFailureKeepsItsOwner(t *testing.T) {
	for _, tc := range []struct{ mode, terminal string }{
		{"error", "PROVIDER-FAILURE-428: clarification endpoint unavailable"}, {"canceled", "context canceled"}, {"panic", "SYNTHESIS-PANIC-619"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			m, synthesis, client, worker := beginKnowledgeIdentityTurn(t, true)
			request := m.lastRequest()
			client.mu.Lock()
			client.mode = tc.mode
			before := client.systemCalls
			client.mu.Unlock()
			if tc.mode == "error" {
				client.SetError(tc.terminal)
			}
			if tc.mode == "canceled" {
				m.shutdownCancel()
			}
			next, save := m.Update(turnEnd(t, synthesis))
			m = next.(Model)
			if m.isLoading || m.awaitingKnowledge || m.turnCount != 0 || !m.showError || m.answering != 0 {
				t.Fatalf("failure was left pending or counted successful: loading=%v awaiting=%v turns=%d showError=%v answering=%d", m.isLoading, m.awaitingKnowledge, m.turnCount, m.showError, m.answering)
			}
			client.mu.Lock()
			calls := client.systemCalls - before
			client.mu.Unlock()
			if calls != 1 || worker.callCount() != 1 {
				t.Fatalf("failure reran synthesis/consultation: calls=%d consultations=%d", calls, worker.callCount())
			}
			persistKnowledgeIdentityCommands(t, save)
			assertKnowledgeIdentityStored(t, m, request, tc.terminal)
		})
	}
}

func TestKnowledgeAnswerIdentity_SaveRetryDoesNotRerunSynthesis(t *testing.T) {
	m, synthesis, client, worker := beginKnowledgeIdentityTurn(t, true)
	request := m.lastRequest()
	next, save := m.Update(turnEnd(t, synthesis))
	m = next.(Model)
	calls := client.GetCallCount()
	// Make only session-state persistence fail. The history writer and SQLite
	// may still succeed; retry must preserve the same owned terminal result.
	statePath := filepath.Join(m.workspace, ".nerd", "session.json")
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(statePath, 0o700); err != nil {
		t.Fatal(err)
	}
	if save == nil {
		t.Fatal("no terminal persistence command")
	}
	if done, ok := save().(sessionStatePersistedMsg); !ok || done.err == nil {
		t.Fatal("fixture did not report the session-state save failure")
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	// Retry the very same immutable snapshot, without executing any turn work.
	persistKnowledgeIdentityCommands(t, save)
	assertKnowledgeIdentityStored(t, m, request, knowledgeIdentityFinal)
	if client.GetCallCount() != calls || worker.callCount() != 1 {
		t.Fatal("persistence retry reran synthesis or consultation")
	}
}

// Supplement the admitted, nonempty-interim witness with an empty handoff and
// legacy request zero. Neither case may infer an owner from the live history.
func TestKnowledgeAnswerIdentity_EmptyInterimAndUnknownRequest(t *testing.T) {
	for _, request := range []uint64{7, 0} {
		t.Run(fmt.Sprint(request), func(t *testing.T) {
			m := NewTestModel(WithWorkspace(t.TempDir()))
			t.Cleanup(m.shutdownCancel)
			m.sessionID = "empty-interim"
			m.history = []Message{{Role: "user", Content: "question", Request: request}}
			m.client = NewMockLLMClient()
			m.client.(*MockLLMClient).SetDefaultResponse(knowledgeIdentityFinal)
			m.isLoading = true
			next, synthesis := m.Update(answerTo(request, knowledgeGatheredMsg{OriginalInput: "question", AwaitingKnowledge: true}))
			m = next.(Model)
			next, save := m.Update(turnEnd(t, synthesis))
			m = next.(Model)
			persistKnowledgeIdentityCommands(t, save)
			if got := m.history[len(m.history)-1]; got.Content != knowledgeIdentityFinal || got.Answers != request {
				t.Fatalf("terminal with no interim = %+v, want owner %d", got, request)
			}
			turns := sessionTurns(m.history)
			want := knowledgeIdentityFinal
			if request == 0 {
				want = ""
			}
			if len(turns) != 1 || turns[0].assistant != want {
				t.Fatalf("projected legacy/owned answer = %+v, want %q", turns, want)
			}
		})
	}
}
