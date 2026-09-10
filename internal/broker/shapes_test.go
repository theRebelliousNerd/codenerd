package broker

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/types"
)

// wrap_test.go proves each shape *advertises* the right capability surface.
// This file proves each advertised method actually works: reaches the
// underlying client, and emits a receipt.
//
// Those are different failures. A shape whose CompleteWithToolResults delegated
// to the wrong core method, or whose embedded baseClient shadowed it, would
// pass the capability matrix and fail at runtime -- and the runtime failure
// would look like a provider problem, because the call would go somewhere
// plausible and return something plausible.

// mixedFake composes exactly the requested capabilities onto one base client,
// so every shape can be exercised against a client that genuinely serves it.
type mixedFake struct {
	*fakeClient
	tr *fakeToolResults
	sc *fakeSchema
	th *fakeThoughts
}

func newMixedFake(withTR, withSC, withTH bool) *mixedFake {
	base := newFakeClient()
	m := &mixedFake{fakeClient: base}
	if withTR {
		m.tr = &fakeToolResults{base}
	}
	if withSC {
		m.sc = &fakeSchema{base}
	}
	if withTH {
		m.th = &fakeThoughts{base}
	}
	return m
}

// asClient returns the fake narrowed to exactly the requested capability set.
// Go decides interface satisfaction from the method set, so the narrowing has
// to be done with distinct concrete types rather than a flag.
func (m *mixedFake) asClient() types.LLMClient {
	switch {
	case m.tr != nil && m.sc != nil && m.th != nil:
		return &fakeAll{fakeClient: m.fakeClient, tr: m.tr, sc: m.sc, th: m.th}
	case m.tr != nil && m.sc != nil:
		return &trscFake{fakeClient: m.fakeClient, tr: m.tr, sc: m.sc}
	case m.tr != nil && m.th != nil:
		return &trthFake{fakeClient: m.fakeClient, tr: m.tr, th: m.th}
	case m.sc != nil && m.th != nil:
		return &scthFake{fakeClient: m.fakeClient, sc: m.sc, th: m.th}
	case m.tr != nil:
		return m.tr
	case m.sc != nil:
		return m.sc
	case m.th != nil:
		return m.th
	default:
		return m.fakeClient
	}
}

type trscFake struct {
	*fakeClient
	tr *fakeToolResults
	sc *fakeSchema
}

func (f *trscFake) CompleteWithToolResults(ctx context.Context, s string, h []types.Message, t []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return f.tr.CompleteWithToolResults(ctx, s, h, t)
}
func (f *trscFake) CompleteWithSchema(ctx context.Context, s, u, j string) (string, error) {
	return f.sc.CompleteWithSchema(ctx, s, u, j)
}

type trthFake struct {
	*fakeClient
	tr *fakeToolResults
	th *fakeThoughts
}

func (f *trthFake) CompleteWithToolResults(ctx context.Context, s string, h []types.Message, t []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return f.tr.CompleteWithToolResults(ctx, s, h, t)
}
func (f *trthFake) CompleteWithStreamingAndThoughts(ctx context.Context, s, u string, think bool) (<-chan string, <-chan string, <-chan error) {
	return f.th.CompleteWithStreamingAndThoughts(ctx, s, u, think)
}

type scthFake struct {
	*fakeClient
	sc *fakeSchema
	th *fakeThoughts
}

func (f *scthFake) CompleteWithSchema(ctx context.Context, s, u, j string) (string, error) {
	return f.sc.CompleteWithSchema(ctx, s, u, j)
}
func (f *scthFake) CompleteWithStreamingAndThoughts(ctx context.Context, s, u string, think bool) (<-chan string, <-chan string, <-chan error) {
	return f.th.CompleteWithStreamingAndThoughts(ctx, s, u, think)
}

// drainThoughts consumes a thoughts-and-content stream to completion, which is
// what makes a streamed turn settle and emit its receipt.
func drainThoughts(t *testing.T, content, thoughts <-chan string, errs <-chan error) (string, string) {
	t.Helper()

	var gotContent, gotThoughts string
	for content != nil || thoughts != nil || errs != nil {
		select {
		case s, ok := <-content:
			if !ok {
				content = nil
				continue
			}
			gotContent += s
		case s, ok := <-thoughts:
			if !ok {
				thoughts = nil
				continue
			}
			gotThoughts += s
		case _, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
		case <-timeAfter():
			t.Fatal("stream did not close; a leaked receipt never settles")
		}
	}
	return gotContent, gotThoughts
}

func TestEveryShapeActuallyServesWhatItAdvertises(t *testing.T) {
	for _, tc := range []struct{ tr, sc, th bool }{
		{false, false, false},
		{true, false, false},
		{false, true, false},
		{false, false, true},
		{true, true, false},
		{true, false, true},
		{false, true, true},
		{true, true, true},
	} {
		name := shapeName(tc.tr, tc.sc, tc.th)
		t.Run(name, func(t *testing.T) {
			fake := newMixedFake(tc.tr, tc.sc, tc.th)
			meter := testMeter(200000, 8000)
			sink := &captureSink{}
			client := meteredClient(t, fake.asClient(), meter, sink)

			ctx := context.Background()

			if tc.tr {
				p, ok := client.(types.ToolResultsProvider)
				if !ok {
					t.Fatal("shape lost ToolResultsProvider")
				}
				resp, err := p.CompleteWithToolResults(ctx, "sys",
					[]types.Message{{Role: "user", Text: "hi"}}, nil)
				if err != nil {
					t.Fatalf("CompleteWithToolResults: %v", err)
				}
				if resp == nil || resp.Text != "ok" {
					t.Fatalf("response = %#v; the call did not reach the underlying client", resp)
				}
			}

			if tc.sc {
				p, ok := client.(interface {
					CompleteWithSchema(context.Context, string, string, string) (string, error)
				})
				if !ok {
					t.Fatal("shape lost CompleteWithSchema")
				}
				out, err := p.CompleteWithSchema(ctx, "sys", "user", `{"type":"object"}`)
				if err != nil {
					t.Fatalf("CompleteWithSchema: %v", err)
				}
				if out != "ok" {
					t.Fatalf("schema output = %q; the call did not reach the underlying client", out)
				}
			}

			if tc.th {
				p, ok := client.(interface {
					CompleteWithStreamingAndThoughts(context.Context, string, string, bool) (<-chan string, <-chan string, <-chan error)
				})
				if !ok {
					t.Fatal("shape lost CompleteWithStreamingAndThoughts")
				}
				content, thoughts, errs := p.CompleteWithStreamingAndThoughts(ctx, "sys", "user", true)
				gotContent, gotThoughts := drainThoughts(t, content, thoughts, errs)
				if gotContent != "ok" {
					t.Fatalf("streamed content = %q", gotContent)
				}
				// The reasoning channel is a separate stream and must not be
				// swallowed or merged into the content one.
				if gotThoughts != "thinking" {
					t.Fatalf("streamed thoughts = %q", gotThoughts)
				}
			}

			// Every capability call must have produced a receipt. A shape whose
			// method reached the provider without metering is spend off the
			// books, which is the one thing this package exists to prevent.
			want := 0
			for _, on := range []bool{tc.tr, tc.sc, tc.th} {
				if on {
					want++
				}
			}
			if want == 0 {
				return
			}
			// A streamed turn settles after its channels close, so a receipt is
			// not present the instant the consumer finishes draining. See the
			// ordering note on proxyStream: the wait is the contract, not a
			// flake workaround.
			waitForReceipts(t, sink, want)
			for _, r := range sink.all() {
				if r.Method == "" {
					t.Error("a receipt carries no method name")
				}
				if !r.Decision.Allowed {
					t.Errorf("a capability call was refused: %+v", r.Decision)
				}
			}
		})
	}
}

func shapeName(tr, sc, th bool) string {
	name := ""
	if tr {
		name += "TR"
	}
	if sc {
		name += "SC"
	}
	if th {
		name += "TH"
	}
	if name == "" {
		return "base"
	}
	return name
}

func TestThoughtStreamingDegradesWhenTheUnderlyingCannotServeIt(t *testing.T) {
	// core.completeWithStreamingAndThoughts is reachable on a client that does
	// not implement it -- Wrap will not advertise it, but the core method still
	// has to answer safely rather than panic, because a future shape could.
	c := wrapCore(t, newFakeClient())

	content, thoughts, errs := c.completeWithStreamingAndThoughts(
		context.Background(), "sys", "user", true)

	gotContent, gotThoughts := drainThoughts(t, content, thoughts, errs)
	if gotContent != "" || gotThoughts != "" {
		t.Errorf("a client with no thought streaming produced content %q / thoughts %q",
			gotContent, gotThoughts)
	}
}

// waitForReceipts blocks until sink holds at least n receipts, or fails.
func waitForReceipts(t *testing.T, sink *captureSink, n int) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		got := len(sink.all())
		if got >= n {
			if got > n {
				t.Fatalf("receipts = %d, want %d — a call was metered more than once", got, n)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("receipts = %d after 3s, want %d — a capability call went unmetered", got, n)
		}
		time.Sleep(time.Millisecond)
	}
}
