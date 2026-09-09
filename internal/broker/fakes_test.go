package broker

import (
	"context"
	"sync"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// fakeClient implements exactly types.LLMClient and nothing else. Shapes with
// optional capabilities embed it and add methods, which is how the capability
// matrix in wrap_test.go is built.
type fakeClient struct {
	mu sync.Mutex

	// reportInput/reportOutput are the usage the client reports through the
	// usage plumbing, exactly as every real provider client does.
	reportInput  int
	reportOutput int
	// reportTimes lets a fake emit several usage reports for one logical call,
	// which is what a retrying or streaming client does.
	reportTimes int

	respText string
	respErr  error

	// calls records what reached the underlying client, so a test can prove the
	// broker forwarded rather than fabricated.
	calls []string

	// streamChunks are delivered on CompleteWithStreaming.
	streamChunks []string

	usageOnResponse *types.UsageMetadata
}

func newFakeClient() *fakeClient {
	return &fakeClient{reportInput: 100, reportOutput: 50, reportTimes: 1, respText: "ok"}
}

func (f *fakeClient) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
}

func (f *fakeClient) callNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeClient) report(ctx context.Context) {
	for i := 0; i < f.reportTimes; i++ {
		usage.TrackFromContext(ctx, "fake-model", "fake", f.reportInput, f.reportOutput, "test")
	}
}

func (f *fakeClient) Complete(ctx context.Context, _ string) (string, error) {
	f.record("Complete")
	f.report(ctx)
	return f.respText, f.respErr
}

func (f *fakeClient) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	f.record("CompleteWithSystem")
	f.report(ctx)
	return f.respText, f.respErr
}

func (f *fakeClient) CompleteWithTools(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	f.record("CompleteWithTools")
	f.report(ctx)
	if f.respErr != nil {
		return nil, f.respErr
	}
	resp := &types.LLMToolResponse{Text: f.respText}
	if f.usageOnResponse != nil {
		resp.Usage = *f.usageOnResponse
	}
	return resp, nil
}

func (f *fakeClient) CompleteWithStreaming(ctx context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	f.record("CompleteWithStreaming")

	content := make(chan string, len(f.streamChunks)+1)
	errs := make(chan error, 1)

	go func() {
		defer close(content)
		defer close(errs)
		for _, chunk := range f.streamChunks {
			content <- chunk
		}
		// Providers report usage as the stream finishes, not when it starts.
		f.report(ctx)
		if f.respErr != nil {
			errs <- f.respErr
		}
	}()

	return content, errs
}

var _ types.LLMClient = (*fakeClient)(nil)

// --- optional-capability shapes -------------------------------------------

type fakeToolResults struct{ *fakeClient }

func (f *fakeToolResults) CompleteWithToolResults(ctx context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	f.record("CompleteWithToolResults")
	f.report(ctx)
	if f.respErr != nil {
		return nil, f.respErr
	}
	resp := &types.LLMToolResponse{Text: f.respText}
	if f.usageOnResponse != nil {
		resp.Usage = *f.usageOnResponse
	}
	return resp, nil
}

type fakeSchema struct{ *fakeClient }

func (f *fakeSchema) CompleteWithSchema(ctx context.Context, _, _, _ string) (string, error) {
	f.record("CompleteWithSchema")
	f.report(ctx)
	return f.respText, f.respErr
}

type fakeThoughts struct{ *fakeClient }

func (f *fakeThoughts) CompleteWithStreamingAndThoughts(ctx context.Context, _, _ string, _ bool) (<-chan string, <-chan string, <-chan error) {
	f.record("CompleteWithStreamingAndThoughts")

	content := make(chan string, 1)
	thoughts := make(chan string, 1)
	errs := make(chan error, 1)

	go func() {
		defer close(content)
		defer close(thoughts)
		defer close(errs)
		thoughts <- "thinking"
		content <- f.respText
		f.report(ctx)
	}()

	return content, thoughts, errs
}

type fakeAll struct {
	*fakeClient
	tr *fakeToolResults
	sc *fakeSchema
	th *fakeThoughts
}

func newFakeAll() *fakeAll {
	base := newFakeClient()
	return &fakeAll{
		fakeClient: base,
		tr:         &fakeToolResults{base},
		sc:         &fakeSchema{base},
		th:         &fakeThoughts{base},
	}
}

func (f *fakeAll) CompleteWithToolResults(ctx context.Context, s string, h []types.Message, t []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return f.tr.CompleteWithToolResults(ctx, s, h, t)
}

func (f *fakeAll) CompleteWithSchema(ctx context.Context, s, u, j string) (string, error) {
	return f.sc.CompleteWithSchema(ctx, s, u, j)
}

func (f *fakeAll) CompleteWithStreamingAndThoughts(ctx context.Context, s, u string, think bool) (<-chan string, <-chan string, <-chan error) {
	return f.th.CompleteWithStreamingAndThoughts(ctx, s, u, think)
}

// --- test scaffolding ------------------------------------------------------

// testMeter builds an isolated meter so a test never shares the process ledger
// or calibrator with another test.
func testMeter(window, reserve int) *Meter {
	return NewMeter(MeterConfig{
		Window:        window,
		OutputReserve: reserve,
		ReceiptBuffer: 64,
		PrimaryModel:  "fake-model",
	})
}

// captureSink collects receipts for assertions.
type captureSink struct {
	mu       sync.Mutex
	receipts []Receipt
}

func (c *captureSink) Record(r Receipt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.receipts = append(c.receipts, r)
}

func (c *captureSink) all() []Receipt {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Receipt, len(c.receipts))
	copy(out, c.receipts)
	return out
}

func (c *captureSink) last() (Receipt, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.receipts) == 0 {
		return Receipt{}, false
	}
	return c.receipts[len(c.receipts)-1], true
}

// failingCounter always errors, to exercise the fail-closed path.
type failingCounter struct{ err error }

func (f failingCounter) Count(context.Context, *Request) (Count, error) { return Count{}, f.err }

// fixedCounter returns a preset count, for admission tests that must not depend
// on estimator behaviour.
type fixedCounter struct {
	tokens     int
	confidence Confidence
}

func (f fixedCounter) Count(_ context.Context, req *Request) (Count, error) {
	return Count{
		Tokens:     f.tokens,
		Confidence: f.confidence,
		Source:     "fixed",
		Model:      req.Model,
		Segments:   splitProportional(f.tokens, measure(req)),
	}, nil
}
