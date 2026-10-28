package session

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/types"
)

// refusingPersister is the session's long-term store when its backend refuses
// every write.
type refusingPersister struct{ memoryPersister }

func (*refusingPersister) StoreFact(string, []any, string, int) error {
	return errors.New("the knowledge store is read-only")
}

// A display string that happens to contain a well-formed envelope. The outer
// envelope is what the model sent; the inner one is an example it is showing.
const exampleEnvelope = `{"control_packet":{"memory_operations":[{"op":"note","key":"example_only","value":"must never be applied"}]},"surface_response":"inner"}`

func envelopeShowingExample() string {
	return `{"control_packet":{"tool_requests":[{"id":"r1","tool_name":"not_offered","tool_args":{}}]},` +
		`"surface_response":` + quoteJSON("Here is what an envelope looks like: "+exampleEnvelope) + `}`
}

func quoteJSON(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func notesOf(t *testing.T, e *Executor) []types.Fact {
	t.Helper()
	notes, err := e.kernel.Query("session_note")
	if err != nil {
		t.Fatal(err)
	}
	return notes
}

// The witness from the audit: promotion reads the outer envelope, rewrites the
// text to its surface, and the call is then refused and cleared, as forced
// finalization does. The task tail used to parse that surface and apply the
// example envelope inside it. The mark on the response is what stops it.
func TestAdmitResponseControls_SurfaceTextIsNeverReadAsControlInput(t *testing.T) {
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	result := &ExecutionResult{}

	resp := &types.LLMToolResponse{Text: envelopeShowingExample()}
	if !executor.promotePiggybackToolRequests(resp, result) {
		t.Fatal("the outer envelope's tool request was not promoted")
	}
	resp.ClearToolCalls() // the unoffered call is refused, as in forced finalization
	if !resp.ControlsApplied {
		t.Fatal("promotion must mark the response; the mark survives Rewrite and ClearToolCalls")
	}

	surface := executor.admitResponseControls(resp, result)
	if !strings.Contains(surface, "example_only") {
		t.Fatalf("the displayed example was altered: %q", surface)
	}
	if notes := notesOf(t, executor); len(notes) != 0 {
		t.Fatalf("the example envelope inside the surface was applied as control input: %v", notes)
	}
}

// The same guarantee without promotion: a response read once is not read again.
func TestAdmitResponseControls_AppliesAnEnvelopeExactlyOnce(t *testing.T) {
	persister := &memoryPersister{}
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	executor.SetSessionPersister(persister)

	envelope := `{"control_packet":{"memory_operations":[{"op":"promote_to_long_term","key":"indent","value":"tabs"}]},"surface_response":"noted"}`
	resp := &types.LLMToolResponse{Text: envelope}
	if got := executor.admitResponseControls(resp, nil); got != "noted" {
		t.Fatalf("surface = %q", got)
	}
	if got := executor.admitResponseControls(resp, nil); got != "noted" {
		t.Fatalf("second read surface = %q, want the already-admitted text", got)
	}
	if len(persister.facts) != 1 {
		t.Fatalf("the promotion reached the store %d times, want exactly once", len(persister.facts))
	}
}

// "I saved your preference" while the store refused it: the user learns that
// from a host-written notice, not from a log line.
func TestAdmitResponseControls_FailedMemoryWriteReachesTheUser(t *testing.T) {
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	executor.SetSessionPersister(&refusingPersister{})
	result := &ExecutionResult{}

	envelope := `{"control_packet":{"memory_operations":[` +
		`{"op":"note","key":"focus","value":"auth"},` +
		`{"op":"promote_to_long_term","key":"indent","value":"tabs"}]},` +
		`"surface_response":"I saved your preference."}`
	surface := executor.admitResponseControls(&types.LLMToolResponse{Text: envelope}, result)
	response := withSafetyNotices(surface, result.safetyNotices)

	if !strings.Contains(response, "I saved your preference.") {
		t.Fatalf("the model's sentence is gone: %q", response)
	}
	want := "[Control] memory operation promote_to_long_term indent #2 failed"
	if !strings.Contains(response, want) || !strings.Contains(response, "read-only") {
		t.Fatalf("response carries no notice for the failed write:\n%s", response)
	}
	if strings.Contains(response, "#1") {
		t.Fatalf("the note that landed must not be reported as failed:\n%s", response)
	}
	if len(notesOf(t, executor)) != 1 {
		t.Fatal("the note that could be applied was not applied")
	}
}

// Controls the parser removed keep their place in the answer too.
func TestAdmitResponseControls_ParserRemovalsReachTheUser(t *testing.T) {
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	result := &ExecutionResult{}
	envelope := `{"control_packet":{"mangle_updates":["observation(\"a\", \"b\").","not an atom"]},"surface_response":"done"}`
	surface := executor.admitResponseControls(&types.LLMToolResponse{Text: envelope}, result)
	response := withSafetyNotices(surface, result.safetyNotices)
	if !strings.Contains(response, "kernel update") || !strings.Contains(response, "#2") {
		t.Fatalf("the removed update #2 is not in the response:\n%s", response)
	}
}

// A reply with two readings of its envelope selects neither: no controls, and
// the text is the host's notice.
func TestAdmitResponseControls_AmbiguousReplyAppliesNothing(t *testing.T) {
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	a := `{"control_packet":{"memory_operations":[{"op":"note","key":"a","value":"1"}]},"surface_response":"A"}`
	b := `{"control_packet":{"memory_operations":[{"op":"note","key":"b","value":"2"}]},"surface_response":"B"}`
	resp := &types.LLMToolResponse{Text: "Example: " + a + "\nReal: " + b}
	surface := executor.admitResponseControls(resp, nil)
	if !strings.Contains(surface, "could not be accepted") {
		t.Fatalf("surface = %q, want the host's notice", surface)
	}
	if resp.Text != surface {
		t.Fatal("the response must hold the notice, not the raw envelopes")
	}
	if notes := notesOf(t, executor); len(notes) != 0 {
		t.Fatalf("an ambiguous reply applied controls: %v", notes)
	}
}

// Kernel assertions that fail are dispositions, not log lines.
func TestProcessMangleUpdates_RecordsPolicyRefusalAndAppliedByPosition(t *testing.T) {
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	env := &articulation.PiggybackEnvelope{Control: articulation.ControlPacket{MangleUpdates: []string{
		`observation("a", "b").`,
		`test_state(/passing).`, // a host witness the model may not write
	}}}
	ds := executor.processMangleUpdatesFromEnvelope(env, nil)
	if len(ds) != 2 {
		t.Fatalf("dispositions = %+v, want one per update", ds)
	}
	if !ds[0].Took() || ds[1].Stage != articulation.StageRejectedPolicy || ds[1].Index != 1 {
		t.Fatalf("dispositions = %+v, want #1 applied and #2 refused by policy", ds)
	}
}

func TestProcessMangleUpdates_NoKernelIsUnsupportedNotSilent(t *testing.T) {
	executor := &Executor{config: DefaultExecutorConfig()}
	env := &articulation.PiggybackEnvelope{Control: articulation.ControlPacket{MangleUpdates: []string{`observation("a", "b").`}}}
	ds := executor.processMangleUpdatesFromEnvelope(env, nil)
	if len(ds) != 1 || ds[0].Stage != articulation.StageUnsupported {
		t.Fatalf("dispositions = %+v, want one unsupported", ds)
	}
}

// A parser removal and a policy refusal are numbered in one space: the
// position in the list the model sent, not in the list left after the parser.
func TestApplyPiggybackControls_PositionsAreTheModelsOwn(t *testing.T) {
	executor := &Executor{kernel: realKernel(t), config: DefaultExecutorConfig()}
	raw := `{"control_packet":{"mangle_updates":["no atom here","observation(\"a\", \"b\").","test_state(/passing)."]},"surface_response":"x"}`
	_, ds := executor.applyPiggybackControls(articulation.ProcessLLMResponseAllowPlain(raw))
	stages := map[int]articulation.ControlStage{}
	for _, d := range ds {
		stages[d.Index] = d.Stage
	}
	want := map[int]articulation.ControlStage{0: articulation.StageRejectedParse, 1: articulation.StageApplied, 2: articulation.StageRejectedPolicy}
	if len(stages) != 3 || stages[0] != want[0] || stages[1] != want[1] || stages[2] != want[2] {
		t.Fatalf("dispositions by position = %v (%+v), want %v", stages, ds, want)
	}
}
