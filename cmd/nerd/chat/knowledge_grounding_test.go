package chat

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
)

// knowledge_grounding_test.go: the request that writes the final answer
// after specialist consultations carries what the turn's answering request
// was grounded in, and every consultation's outcome.

// A turn reads a file partly, asks a specialist, and the synthesis request
// still holds the file's text and its PARTIAL coverage.
func TestKnowledgeSynthesis_KeepsTheTurnsEvidence(t *testing.T) {
	t.Parallel()
	ws := SetupLiveWorkspace(t)
	kernel, err := core.NewRealKernelWithWorkspace(ws)
	if err != nil {
		t.Fatal(err)
	}
	body := "HEAD-SENTINEL-8812 first line\n" + strings.Repeat("filler line\n", 20000)
	if err := os.WriteFile(filepath.Join(ws, "app.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	intent := evidenceIntent("app.log", 0.3)
	client := NewMockLLMClient()
	client.SetDefaultResponse(articulationResponseForTest(t, "Let me ask a specialist.", intent, nil, nil, false,
		[]map[string]string{{"specialist": "researcher", "query": "what does this log format mean", "purpose": "explain the first line", "priority": "required"}}))
	m := NewTestModel(WithSize(120, 50), WithWorkspace(ws))
	t.Cleanup(m.shutdownCancel)
	m.kernel = kernel
	m.client = client
	m.transducer = &scriptedTransducer{intents: []perception.Intent{intent}}
	m.virtualStore = core.NewVirtualStore(nil)
	m.taskExecutor = &processControlTaskExecutor{response: "That format is a generic log header."}

	msg := awaitProcessStream(t, m.processInput("what does the first line of app.log say", 0)())
	gathered, ok := msg.(knowledgeGatheredMsg)
	if !ok {
		t.Fatalf("turn returned %T, want knowledge gathering", msg)
	}
	client.SetDefaultResponse("Final answer.")
	if result, ok := m.synthesizeWithKnowledge(0, gathered.OriginalInput, gathered.Parent, gathered.Results)().(knowledgeSynthesizedMsg); !ok || result.Err != nil {
		t.Fatal("synthesis did not answer")
	}
	prompt := client.GetLastPrompt()
	if !strings.Contains(prompt, "HEAD-SENTINEL-8812") || !strings.Contains(prompt, "PARTIAL") || !strings.Contains(prompt, "the rest was NOT read") {
		t.Fatalf("the synthesis request lost the turn's evidence or its coverage:\n%.3000s", prompt)
	}
	if !strings.Contains(prompt, "That format is a generic log header.") {
		t.Fatal("the synthesis request lost the specialist's answer")
	}
}

// One consultation answers, one fails, one returns nothing: the answer is
// kept, and the two that did not answer are named with what they were for.
func TestKnowledgeSynthesis_NamesTheConsultationsThatDidNotAnswer(t *testing.T) {
	got := knowledgeSynthesisPrompt("is the migration safe?", knowledgeParent{RecentTurns: []Message{{Role: "user", Content: "we must keep v1 clients working"}}}, []KnowledgeResult{
		{Specialist: "mangleexpert", Query: "migration syntax", Purpose: "write the migration", Response: "Use ALTER TABLE ..."},
		{Specialist: "researcher", Query: "does it keep v1 compatible", Purpose: "the compatibility requirement", Error: errors.New("provider timeout")},
		{Specialist: "tester", Query: "which tests cover it", Purpose: "verification", Response: "  "},
	})
	for _, want := range []string{
		"we must keep v1 clients working",
		"### From mangleexpert (query: migration syntax; purpose: write the migration)",
		"researcher (query: does it keep v1 compatible; purpose: the compatibility requirement): provider timeout",
		"tester (query: which tests cover it; purpose: verification): returned nothing",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("synthesis request lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "All consultations failed") {
		t.Fatal("one consultation answered")
	}
}

// Requests past the per-turn cap are reported as not run, not dropped.
func TestKnowledgeRequests_PastTheCapAreReportedNotRun(t *testing.T) {
	t.Parallel()
	m, _ := journeyModel(t, config.WorkModePropose)
	reqs := []articulation.KnowledgeRequest{
		{Specialist: "researcher", Query: "q1", Purpose: "p1"},
		{Specialist: "researcher", Query: "q2", Purpose: "p2"},
		{Specialist: "researcher", Query: "q3", Purpose: "p3"},
	}
	msg := m.handleKnowledgeRequests(m.shutdownCtx, reqs, "question", "interim", knowledgeParent{})
	if len(msg.Results) != 3 {
		t.Fatalf("results = %d, want 3 (two run, one not run)", len(msg.Results))
	}
	var notRun []KnowledgeResult
	for _, r := range msg.Results {
		if errors.Is(r.Error, errKnowledgeNotRun) {
			notRun = append(notRun, r)
		}
	}
	if len(notRun) != 1 || notRun[0].Query != "q3" || notRun[0].Purpose != "p3" {
		t.Fatalf("not-run results = %+v", notRun)
	}
}
