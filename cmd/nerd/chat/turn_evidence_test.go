package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
)

func TestReadEvidenceFile_WholeAndPrefixAreTold(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "small.txt"), []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	whole := readEvidenceFile(ws, "small.txt", 100)
	if whole.Coverage != coverageWhole || whole.Body != "abcdef" || whole.SourceBytes != 6 {
		t.Fatalf("whole read = %+v", whole)
	}

	// 1 MiB with the answer only in the tail: the read stops at the budget.
	big := strings.Repeat("filler line\n", 90000) + "TAIL-SENTINEL\n"
	if err := os.WriteFile(filepath.Join(ws, "big.log"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	span := readEvidenceFile(ws, "big.log", 4096)
	if span.Coverage != coverageSpan {
		t.Fatalf("a prefix was labelled %q", span.Coverage)
	}
	if span.ReturnedBytes > 4096 || int64(len(span.Body)) != span.ReturnedBytes {
		t.Fatalf("read %d bytes past a 4096 budget (body %d)", span.ReturnedBytes, len(span.Body))
	}
	if span.SourceBytes != int64(len(big)) {
		t.Fatalf("source size = %d, want %d", span.SourceBytes, len(big))
	}
	if strings.Contains(span.Body, "TAIL-SENTINEL") {
		t.Fatal("the tail cannot be in a 4 KiB prefix")
	}
}

func TestReadEvidenceFile_CutNeverSplitsARune(t *testing.T) {
	ws := t.TempDir()
	body := strings.Repeat("é", 100) // two bytes each
	if err := os.WriteFile(filepath.Join(ws, "u.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	obs := readEvidenceFile(ws, "u.txt", 51)
	if obs.Coverage != coverageSpan || obs.ReturnedBytes != 50 {
		t.Fatalf("obs = %+v, want a 50 byte span ending on a rune boundary", obs)
	}
}

func TestReadEvidenceFile_UnavailableIsTyped(t *testing.T) {
	ws := t.TempDir()
	outside := filepath.Join(filepath.Dir(ws), "evidence-outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if err := os.Mkdir(filepath.Join(ws, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{outside, "missing.txt", "dir"} {
		obs := readEvidenceFile(ws, target, 100)
		if obs.Coverage != coverageUnavailable || obs.Body != "" || obs.Diagnostic == "" {
			t.Errorf("%s: obs = %+v, want unavailable with a diagnostic and no body", target, obs)
		}
	}
}

// The residual read no longer stores a prefix as the file: file_content is
// asserted only for a whole file, and the observation carries the coverage.
func TestResidualRead_PrefixIsNotFileContent(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "big.log"), []byte(strings.Repeat("x", 100000)), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(WithWorkspace(ws))
	warnings := []string{}
	facts, _, evidence := m.executeInfoGatheringActions(context.Background(), "q",
		perception.Intent{Target: "big.log"},
		[]core.Fact{{Predicate: "action", Args: []any{"/fs_read"}}}, &warnings)
	if len(facts) != 0 {
		t.Fatalf("a prefix was asserted as file_content: %d facts", len(facts))
	}
	if len(evidence) != 1 || evidence[0].Coverage != coverageSpan {
		t.Fatalf("evidence = %+v, want one span", evidence)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "of 100000 bytes") || !strings.Contains(warnings[0], "the rest was not") {
		t.Fatalf("warning does not say the read was partial: %v", warnings)
	}
}

// The body reaches the answering request: a sentinel in the file is in the
// prompt, a prefix says it is one, and an unavailable source says nothing was
// read. Kernel presence is not the oracle; the prompt is.
func TestArticulation_PromptCarriesEvidenceAndItsCoverage(t *testing.T) {
	intent := perception.Intent{Category: "/query", Verb: "/analyze", Target: "app.log"}
	prompt := func(ev ...evidenceObservation) string {
		client := NewMockLLMClient()
		_, _ = articulateWithConversation(context.Background(), client, intent,
			payloadForArticulation(intent, nil), nil, "sys",
			&ConversationContext{Evidence: ev}, nil, nil)
		client.mu.Lock()
		defer client.mu.Unlock()
		return client.lastPrompt
	}

	whole := prompt(evidenceObservation{Path: "app.log", Coverage: coverageWhole, Body: "line one\nSENTINEL-4471\n", SourceBytes: 23, ReturnedBytes: 23, ReturnedLines: 2})
	if !strings.Contains(whole, "SENTINEL-4471") || !strings.Contains(whole, "whole file, 23 bytes") {
		t.Fatalf("whole-file evidence is not in the prompt:\n%s", whole)
	}

	part := prompt(evidenceObservation{Path: "app.log", Coverage: coverageSpan, Body: "head\n", SourceBytes: 99999, ReturnedBytes: 5, ReturnedLines: 1})
	if !strings.Contains(part, "PARTIAL") || !strings.Contains(part, "5 of 99999 bytes") || !strings.Contains(part, "the rest was NOT read") {
		t.Fatalf("a prefix is not marked as one:\n%s", part)
	}

	gone := prompt(evidenceObservation{Path: "app.log", Coverage: coverageUnavailable, Diagnostic: "no such file"})
	if !strings.Contains(gone, "UNAVAILABLE: no such file") || !strings.Contains(gone, "do not claim to have read it") {
		t.Fatalf("an unavailable source is not stated:\n%s", gone)
	}

	if none := prompt(); strings.Contains(none, "Workspace Evidence") {
		t.Fatalf("an evidence section was rendered with no evidence:\n%s", none)
	}
}

func TestFencedBody_FileCannotCloseItsFence(t *testing.T) {
	out := fencedBody("a\n```\nb")
	if !strings.HasPrefix(out, "````\n") || !strings.HasSuffix(out, "\n````\n") {
		t.Fatalf("fence = %q", out)
	}
}

func evidenceIntent(file string, conf float64) perception.Intent {
	return perception.Intent{
		Category: "/query", Verb: "/analyze", Target: file, Confidence: conf, IsQuestion: true,
		ScopeLevel: "file", ScopeFile: file,
	}
}

// The policy gives an evidence request a disposition: with no shard candidate
// (or one below the gate) the old result was no lane and a tool-less answer.
func TestDecideRoute_EvidenceWithNoShardGathersEvidence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		shard string
		conf  float64
	}{
		{"no candidate shard", "", 0.9},
		{"candidate below the confidence gate", "reviewer", 0.3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newRoundtripModel(t)
			intent := evidenceIntent("app.log", tc.conf)
			assertRouteIntent(t, m, intent)
			route := m.decideRoute("how many errors are in app.log", intent, tc.shard)
			if route.Kind != RouteGatherEvidence {
				t.Fatalf("route = %s, want gather_evidence", route.Kind)
			}
			rows, err := m.kernel.Query("route_decision")
			if err != nil || len(rows) != 1 {
				t.Fatalf("route_decision rows = %v (err %v), want exactly one lane", rows, err)
			}
		})
	}
}

// Suggested tools with no file still derive the lane: the answer must be told
// it has no evidence, not left to answer from prose.
func TestDecideRoute_ToolsOnlyEvidenceGathers(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	intent := perception.Intent{Category: "/query", Verb: "/explain", Target: "retry policy", Confidence: 0.9, IsQuestion: true, ToolsNeeded: []string{"grep"}}
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("where is the retry policy built", intent, ""); route.Kind != RouteGatherEvidence {
		t.Fatalf("route = %s, want gather_evidence", route.Kind)
	}
}

// The lane never takes a turn another lane owns: a confident shard still
// delegates, a mutation never gathers, a plain question still answers.
func TestDecideRoute_GatherEvidenceDoesNotTakeOtherLanes(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	intent := evidenceIntent("app.log", 0.96)
	assertRouteIntent(t, m, intent)
	if route := m.decideRoute("how many errors are in app.log", intent, "reviewer"); route.Kind != RouteDelegate {
		t.Errorf("a confident reviewer: route = %s, want delegate", route.Kind)
	}

	m = newRoundtripModel(t)
	mut := perception.Intent{Category: "/mutation", Verb: "/fix", Target: "app.go", Confidence: 0.95, ScopeLevel: "file", ScopeFile: "app.go"}
	assertRouteIntent(t, m, mut)
	if route := m.decideRoute("fix app.go", mut, "coder"); route.Kind == RouteGatherEvidence {
		t.Errorf("a mutation gathered evidence")
	}

	m = newRoundtripModel(t)
	q := perception.Intent{Category: "/query", Verb: "/explain", Target: "jit", Confidence: 0.9, IsQuestion: true}
	assertRouteIntent(t, m, q)
	if route := m.decideRoute("what is the jit system", q, ""); route.Kind != RouteRespondDirectly {
		t.Errorf("a prose question: route = %s, want respond_directly", route.Kind)
	}
}

func TestGatherWorkspaceEvidence_NamesNoFileAsUnavailable(t *testing.T) {
	obs := gatherWorkspaceEvidence(t.TempDir(), perception.Intent{Target: "retry policy", ToolsNeeded: []string{"grep"}}, nil)
	if len(obs) != 1 || obs[0].Coverage != coverageUnavailable || !strings.Contains(obs[0].Diagnostic, "names no file") || !strings.Contains(obs[0].Diagnostic, "grep") {
		t.Fatalf("obs = %+v", obs)
	}
}

func TestGatherWorkspaceEvidence_DoesNotRereadAnObservedFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.log"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	have := []evidenceObservation{{Path: "a.log", Coverage: coverageWhole}}
	if got := gatherWorkspaceEvidence(ws, perception.Intent{ScopeFile: "a.log"}, have); len(got) != 0 {
		t.Fatalf("re-read an observed file: %+v", got)
	}
}

// Through the real turn: an evidence question with no shard reaches
// articulation carrying the file's text. The file is larger than the prompt
// budget and the answer is in its head, so the sentinel is deliverable and the
// partial coverage is stated.
func TestProcessInput_EvidenceTurnDeliversTheFileToTheAnsweringCall(t *testing.T) {
	t.Parallel()
	ws := SetupLiveWorkspace(t)
	kernel, err := core.NewRealKernelWithWorkspace(ws)
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	body := "HEAD-SENTINEL-8812 first line\n" + strings.Repeat("filler line\n", 20000)
	if err := os.WriteFile(filepath.Join(ws, "app.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	client := NewMockLLMClient()
	client.SetDefaultResponse("Answered from the evidence.")
	m := NewTestModel(WithSize(120, 50), WithWorkspace(ws))
	t.Cleanup(m.shutdownCancel)
	m.kernel = kernel
	m.client = client
	m.transducer = &scriptedTransducer{intents: []perception.Intent{evidenceIntent("app.log", 0.3)}}
	m.virtualStore = core.NewVirtualStore(nil)

	msg := awaitProcessStream(t, m.processInput("what does the first line of app.log say", 0)())
	if _, ok := msg.(assistantMsg); !ok {
		t.Fatalf("turn returned %T, want an articulated answer", msg)
	}
	client.mu.Lock()
	prompt := client.lastPrompt
	client.mu.Unlock()
	if !strings.Contains(prompt, "HEAD-SENTINEL-8812") {
		t.Fatalf("the file's text never reached the answering request:\n%.2000s", prompt)
	}
	if !strings.Contains(prompt, "PARTIAL") || !strings.Contains(prompt, "the rest was NOT read") {
		t.Fatalf("the prompt does not say the file was only partly read")
	}
}
