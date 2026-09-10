package broker

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/types"
)

// cutThenWhole reports the output as cut at the ceiling for its first `cuts`
// calls and answers whole after that, recording every prompt it was handed so
// a test can see exactly what the model was asked the second time.
type cutThenWhole struct {
	mu      sync.Mutex
	cuts    int
	calls   int
	systems []string
	prompts []string
}

func (f *cutThenWhole) cut(method string) error {
	return &types.OutputTruncated{
		Provider: "fake", Model: "fake-model", Method: method, Reason: "length",
		LimitTokens: 4096, OutputTokens: 4096, Partial: "half an ans",
	}
}

func (f *cutThenWhole) Complete(_ context.Context, prompt string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.prompts = append(f.prompts, prompt)
	if f.calls <= f.cuts {
		return "", f.cut("Complete")
	}
	return "whole", nil
}

func (f *cutThenWhole) CompleteWithSystem(_ context.Context, system, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.systems = append(f.systems, system)
	if f.calls <= f.cuts {
		return "", f.cut("CompleteWithSystem")
	}
	return "whole", nil
}

func (f *cutThenWhole) CompleteWithTools(_ context.Context, system, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.systems = append(f.systems, system)
	if f.calls <= f.cuts {
		return nil, f.cut("CompleteWithTools")
	}
	return &types.LLMToolResponse{Text: "whole", StopReason: "end_turn"}, nil
}

func (f *cutThenWhole) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	return closedStringChan(), errChanWith(nil)
}

func (f *cutThenWhole) snapshot() (int, []string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, append([]string(nil), f.systems...), append([]string(nil), f.prompts...)
}

func wrapCutThenWhole(t *testing.T, cuts int) (types.LLMClient, *cutThenWhole, *captureSink) {
	t.Helper()
	fake := &cutThenWhole{cuts: cuts}
	sink := &captureSink{}
	cfg := testMeter(200000, 8000).ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	cfg.Sink = sink
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	return client, fake, sink
}

// An answer the provider cut at its ceiling is sent back to be restated
// within the limit, and the caller receives the whole restatement — never the
// partial. The instruction rides on the system prompt, after it, so the
// original instructions are untouched.
func TestCompressing_RestatesACutAnswerWithinTheLimit(t *testing.T) {
	client, fake, sink := wrapCutThenWhole(t, 1)

	out, err := client.CompleteWithSystem(context.Background(), "SYSTEM", "user")
	if err != nil || out != "whole" {
		t.Fatalf("out=%q err=%v, want the whole restatement and no error", out, err)
	}
	calls, systems, _ := fake.snapshot()
	if calls != 2 {
		t.Fatalf("calls=%d, want the cut call and one restatement", calls)
	}
	if systems[0] != "SYSTEM" {
		t.Fatalf("first call's system prompt was altered: %q", systems[0])
	}
	if !strings.HasPrefix(systems[1], "SYSTEM\n\n") || !strings.Contains(systems[1], "cut off") ||
		!strings.Contains(systems[1], "4096 tokens") || !strings.Contains(systems[1], "compress") {
		t.Fatalf("restatement must carry the original system prompt followed by the compression instruction naming the limit; got %q", systems[1])
	}
	receipts := sink.all()
	if len(receipts) != 2 {
		t.Fatalf("receipts=%d, want one per billed pass", len(receipts))
	}
	if !strings.Contains(receipts[0].Err, "truncated") || receipts[1].Err != "" {
		t.Fatalf("the cut pass must be receipted as such and the restatement as clean; got %q then %q", receipts[0].Err, receipts[1].Err)
	}
}

// After the bounded restatements the caller gets the typed error and no text.
// A partial answer leaving the broker is the failure this exists to end.
func TestCompressing_GivesUpAfterBoundedRestatementsWithNoPartial(t *testing.T) {
	client, fake, sink := wrapCutThenWhole(t, 100)

	out, err := client.CompleteWithSystem(context.Background(), "SYSTEM", "user")
	if _, cut := types.AsOutputTruncated(err); !cut {
		t.Fatalf("err=%v, want the truncation report to surface", err)
	}
	if out != "" {
		t.Fatalf("out=%q, want no text at all after a cut that could not be restated", out)
	}
	calls, _, _ := fake.snapshot()
	if calls != 1+maxCompressionRetries {
		t.Fatalf("calls=%d, want the cut call plus %d restatements", calls, maxCompressionRetries)
	}
	if got := len(sink.all()); got != calls {
		t.Fatalf("receipts=%d for %d billed passes", got, calls)
	}
}

// Complete has no system prompt, so the instruction rides on the prompt.
func TestCompressing_CompleteCarriesTheInstructionOnThePrompt(t *testing.T) {
	client, fake, _ := wrapCutThenWhole(t, 1)

	out, err := client.Complete(context.Background(), "PROMPT")
	if err != nil || out != "whole" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	_, _, prompts := fake.snapshot()
	if len(prompts) != 2 || prompts[0] != "PROMPT" || !strings.HasPrefix(prompts[1], "PROMPT\n\n") || !strings.Contains(prompts[1], "cut off") {
		t.Fatalf("prompts=%q, want the original then the original plus the instruction", prompts)
	}
}

// A tool response cut mid-arguments is refused as a whole and restated; the
// caller never sees a half-parsed tool call.
func TestCompressing_ToolResponseIsRestatedNotPartial(t *testing.T) {
	client, fake, _ := wrapCutThenWhole(t, 1)

	resp, err := client.CompleteWithTools(context.Background(), "SYSTEM", "user", nil)
	if err != nil || resp == nil || resp.Text != "whole" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if calls, _, _ := fake.snapshot(); calls != 2 {
		t.Fatalf("calls=%d, want 2", calls)
	}
}

// A call that fails for any other reason is not retried here: the
// restatement is for one condition, and turning it into a generic retry
// would double-bill every transient failure.
func TestCompressing_OtherErrorsAreNotRetried(t *testing.T) {
	fake := newFakeClient()
	fake.respErr = context.DeadlineExceeded
	cfg := testMeter(200000, 8000).ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	client, err := Wrap(fake, cfg)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if _, err := client.CompleteWithSystem(context.Background(), "s", "u"); err != context.DeadlineExceeded {
		t.Fatalf("err=%v, want the underlying error untouched", err)
	}
	if got := fake.callNames(); len(got) != 1 {
		t.Fatalf("calls=%v, want exactly one", got)
	}
}
