package broker

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// TestWrapPreservesCapabilityMatrix is the load-bearing test of this package.
//
// Several call sites choose a control flow by probing the client. If the
// wrapper advertises a capability the underlying client lacks, the request is
// routed into a path that cannot serve it — and the failure surfaces far from
// here, as tool calls that silently go nowhere on Gemini. If the wrapper drops
// a capability the client has, the system quietly degrades to a worse path.
//
// Both directions are checked for all eight combinations.
func TestWrapPreservesCapabilityMatrix(t *testing.T) {
	meter := testMeter(200000, 8000)

	type probe struct {
		name string
		has  func(types.LLMClient) bool
	}
	probes := []probe{
		{"ToolResultsProvider", func(c types.LLMClient) bool {
			_, ok := c.(types.ToolResultsProvider)
			return ok
		}},
		{"SchemaCapable", func(c types.LLMClient) bool {
			_, ok := c.(interface {
				CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
			})
			return ok
		}},
		{"StreamingWithThoughts", func(c types.LLMClient) bool {
			_, ok := c.(interface {
				CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error)
			})
			return ok
		}},
	}

	cases := []struct {
		name  string
		build func() types.LLMClient
		want  [3]bool // ToolResults, Schema, Thoughts
	}{
		{"bare", func() types.LLMClient { return newFakeClient() }, [3]bool{false, false, false}},
		{"toolResults", func() types.LLMClient { return &fakeToolResults{newFakeClient()} }, [3]bool{true, false, false}},
		{"schema", func() types.LLMClient { return &fakeSchema{newFakeClient()} }, [3]bool{false, true, false}},
		{"thoughts", func() types.LLMClient { return &fakeThoughts{newFakeClient()} }, [3]bool{false, false, true}},
		{"all", func() types.LLMClient { return newFakeAll() }, [3]bool{true, true, true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			underlying := tc.build()
			wrapped, err := Wrap(underlying, meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"}))
			if err != nil {
				t.Fatalf("Wrap: %v", err)
			}

			for i, p := range probes {
				gotUnderlying := p.has(underlying)
				gotWrapped := p.has(wrapped)

				if gotUnderlying != tc.want[i] {
					t.Fatalf("test fixture wrong: underlying %s = %v, want %v", p.name, gotUnderlying, tc.want[i])
				}
				if gotWrapped != tc.want[i] {
					t.Errorf("%s: wrapped client reports %s = %v, underlying = %v.\n"+
						"A wrapper that gains a capability routes requests into a path the provider cannot serve; "+
						"one that loses a capability silently downgrades the provider.",
						tc.name, p.name, gotWrapped, tc.want[i])
				}
			}
		})
	}
}

// TestWrapAlwaysExposesUnconditionalForwarders pins the other half of the
// decision recorded in passthrough.go: accessors and setters are forwarded
// unconditionally because a probe that succeeds and forwards to nothing is
// indistinguishable from a probe that failed.
func TestWrapAlwaysExposesUnconditionalForwarders(t *testing.T) {
	meter := testMeter(200000, 8000)
	wrapped, err := Wrap(newFakeClient(), meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"}))
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	if _, ok := wrapped.(types.GroundingController); !ok {
		t.Error("wrapper must expose GroundingController; autopoiesis probes it and would stop configuring grounding")
	}
	if _, ok := wrapped.(interface{ SetModel(string) }); !ok {
		t.Error("wrapper must expose SetModel; the chat /model command probes it")
	}
	if _, ok := wrapped.(interface{ GetModel() string }); !ok {
		t.Error("wrapper must expose GetModel; the initializer probes it")
	}
	if _, ok := wrapped.(interface{ GetLastThinkingTokens() int }); !ok {
		t.Error("wrapper must expose GetLastThinkingTokens; the tracing client probes it")
	}
	if _, ok := wrapped.(interface{ Unwrap() types.LLMClient }); !ok {
		t.Error("wrapper must expose Unwrap; concrete-type assertions reach through it")
	}
}

// TestGroundingForwardsToUnderlying proves the unconditional forwarders are
// genuine forwards and not stubs that swallow the call.
func TestGroundingForwardsToUnderlying(t *testing.T) {
	meter := testMeter(200000, 8000)
	grounding := &fakeGrounding{fakeClient: newFakeClient()}

	wrapped, err := Wrap(grounding, meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"}))
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	gc, ok := wrapped.(types.GroundingController)
	if !ok {
		t.Fatal("wrapper does not implement GroundingController")
	}

	gc.SetEnableGoogleSearch(true)
	if !grounding.searchEnabled {
		t.Error("SetEnableGoogleSearch did not reach the underlying client")
	}
	if !gc.IsGoogleSearchEnabled() {
		t.Error("IsGoogleSearchEnabled did not read through to the underlying client")
	}

	gc.SetURLContextURLs([]string{"https://example.com"})
	if len(grounding.urls) != 1 {
		t.Errorf("SetURLContextURLs did not reach the underlying client: %v", grounding.urls)
	}
}

type fakeGrounding struct {
	*fakeClient
	searchEnabled bool
	urlContext    bool
	urls          []string
}

func (f *fakeGrounding) SetEnableGoogleSearch(v bool)      { f.searchEnabled = v }
func (f *fakeGrounding) SetEnableURLContext(v bool)        { f.urlContext = v }
func (f *fakeGrounding) SetURLContextURLs(u []string)      { f.urls = u }
func (f *fakeGrounding) IsGoogleSearchEnabled() bool       { return f.searchEnabled }
func (f *fakeGrounding) IsURLContextEnabled() bool         { return f.urlContext }
func (f *fakeGrounding) GetLastGroundingSources() []string { return f.urls }

// TestBaseReachesThroughDecorators covers the helper that concrete-type
// assertions depend on.
func TestBaseReachesThroughDecorators(t *testing.T) {
	meter := testMeter(200000, 8000)
	inner := newFakeClient()

	once, err := Wrap(inner, meter.ConfigFor(ProviderCreds{Provider: "fake"}))
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	if Base(once) != types.LLMClient(inner) {
		t.Error("Base did not reach the innermost client through one wrapper")
	}
	if Base(inner) != types.LLMClient(inner) {
		t.Error("Base must be identity on an unwrapped client")
	}
	if Base(nil) != nil {
		t.Error("Base(nil) must be nil")
	}
}

// TestBaseTerminatesOnCyclicUnwrap guards the bound. A decorator whose Unwrap
// returns itself would otherwise spin forever inside prompt assembly.
func TestBaseTerminatesOnCyclicUnwrap(t *testing.T) {
	cyclic := &selfUnwrapping{fakeClient: newFakeClient()}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = Base(cyclic)
		_ = IsBrokered(cyclic)
	}()

	select {
	case <-done:
	case <-timeAfter():
		t.Fatal("Base/IsBrokered did not terminate on a cyclic Unwrap chain")
	}
}

type selfUnwrapping struct{ *fakeClient }

func (s *selfUnwrapping) Unwrap() types.LLMClient { return s }

// TestIsBrokeredDetectsMeteringThroughChain backs the wiring audit.
func TestIsBrokeredDetectsMeteringThroughChain(t *testing.T) {
	meter := testMeter(200000, 8000)
	inner := newFakeClient()

	if IsBrokered(inner) {
		t.Error("a bare client must not report as brokered")
	}

	wrapped, err := Wrap(inner, meter.ConfigFor(ProviderCreds{Provider: "fake"}))
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	if !IsBrokered(wrapped) {
		t.Error("a wrapped client must report as brokered")
	}

	// A further decorator layered on top must not hide the metering beneath it,
	// which is exactly the shape production uses: tracing wraps the broker.
	outer := &passthroughDecorator{inner: wrapped}
	if !IsBrokered(outer) {
		t.Error("metering must remain detectable through an outer decorator")
	}
}

type passthroughDecorator struct{ inner types.LLMClient }

func (p *passthroughDecorator) Complete(ctx context.Context, prompt string) (string, error) {
	return p.inner.Complete(ctx, prompt)
}
func (p *passthroughDecorator) CompleteWithSystem(ctx context.Context, s, u string) (string, error) {
	return p.inner.CompleteWithSystem(ctx, s, u)
}
func (p *passthroughDecorator) CompleteWithStreaming(ctx context.Context, s, u string, t bool) (<-chan string, <-chan error) {
	return p.inner.CompleteWithStreaming(ctx, s, u, t)
}
func (p *passthroughDecorator) CompleteWithTools(ctx context.Context, s, u string, t []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return p.inner.CompleteWithTools(ctx, s, u, t)
}
func (p *passthroughDecorator) Unwrap() types.LLMClient { return p.inner }

// TestWrapRejectsUnmeterableConfig proves construction fails closed.
func TestWrapRejectsUnmeterableConfig(t *testing.T) {
	if _, err := Wrap(nil, Config{Counter: fixedCounter{tokens: 1}}); err == nil {
		t.Error("Wrap(nil client) must fail rather than return an unusable client")
	}
	if _, err := Wrap(newFakeClient(), Config{}); err == nil {
		t.Error("Wrap without a counter must fail: a broker that cannot count enforces nothing")
	}
}
