package broker

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// The passthrough forwarders are the quietest part of the decorator and the
// easiest to get wrong. Each has two paths -- the underlying implements the
// method, or it does not -- and the whole design rests on both producing
// exactly what the unwrapped client would have produced. A forwarder that
// returned its own value instead of the underlying's, or that answered a
// plausible default where the underlying would have answered differently,
// changes behaviour with nothing failing anywhere.
//
// So every one is checked in both directions.

// richClient implements every optional accessor with distinctive values, so a
// forwarder that fabricates a default instead of forwarding is visible.
type richClient struct {
	*fakeClient

	model             string
	semaphoreDisabled bool
	searchEnabled     bool
	urlContextEnabled bool
	urls              []string
}

func newRichClient() *richClient {
	return &richClient{fakeClient: newFakeClient(), model: "rich-model", searchEnabled: true, urlContextEnabled: true}
}

func (r *richClient) GetModel() string                { return r.model }
func (r *richClient) SetModel(m string)               { r.model = m }
func (r *richClient) DisableSemaphore()               { r.semaphoreDisabled = true }
func (r *richClient) GetLastThinkingTokens() int      { return 4242 }
func (r *richClient) GetThinkingLevel() string        { return "deep" }
func (r *richClient) GetLastThoughtSummary() string   { return "considered the alternatives" }
func (r *richClient) GetLastThoughtSignature() string { return "sig-opaque-abc123" }
func (r *richClient) GetLastGroundingSources() []string {
	return []string{"https://example.test/a", "https://example.test/b"}
}
func (r *richClient) IsGoogleSearchEnabled() bool  { return r.searchEnabled }
func (r *richClient) IsURLContextEnabled() bool    { return r.urlContextEnabled }
func (r *richClient) SetEnableGoogleSearch(v bool) { r.searchEnabled = v }
func (r *richClient) SetEnableURLContext(v bool)   { r.urlContextEnabled = v }
func (r *richClient) SetURLContextURLs(u []string) { r.urls = u }
func (r *richClient) SetCachedContent(h string)    { r.model = "cached:" + h }
func (r *richClient) SchemaCapable() bool          { return false }

func wrapCore(t *testing.T, underlying types.LLMClient) *core {
	t.Helper()
	meter := testMeter(200000, 8000)
	cfg := meter.ConfigFor(ProviderCreds{Provider: "fake", Model: "fake-model"})
	return &core{underlying: underlying, cfg: cfg, model: "fake-model"}
}

func TestPassthroughForwardsWhenTheUnderlyingImplements(t *testing.T) {
	rich := newRichClient()
	c := wrapCore(t, rich)

	if got := c.GetLastThinkingTokens(); got != 4242 {
		t.Errorf("GetLastThinkingTokens = %d, want 4242", got)
	}
	if got := c.GetThinkingLevel(); got != "deep" {
		t.Errorf("GetThinkingLevel = %q, want deep", got)
	}
	if got := c.GetLastThoughtSummary(); got != "considered the alternatives" {
		t.Errorf("GetLastThoughtSummary = %q", got)
	}
	// Provider-bound continuation state: forwarded verbatim, never synthesized.
	if got := c.GetLastThoughtSignature(); got != "sig-opaque-abc123" {
		t.Errorf("GetLastThoughtSignature = %q", got)
	}
	if got := c.GetLastGroundingSources(); len(got) != 2 || got[0] != "https://example.test/a" {
		t.Errorf("GetLastGroundingSources = %v", got)
	}
	if !c.IsGoogleSearchEnabled() || !c.IsURLContextEnabled() {
		t.Error("grounding probes did not forward")
	}
	if c.SchemaCapable() {
		t.Error("SchemaCapable answered for itself; a client that switched structured output off must stay off")
	}
}

func TestPassthroughReturnsTheUnwrappedZeroWhenNotImplemented(t *testing.T) {
	// fakeClient implements exactly types.LLMClient and nothing else, so a
	// caller probing the unwrapped client would skip and leave its field at
	// the zero value. Forwarding must produce the same thing.
	c := wrapCore(t, newFakeClient())

	if got := c.GetLastThinkingTokens(); got != 0 {
		t.Errorf("GetLastThinkingTokens = %d, want 0", got)
	}
	if got := c.GetThinkingLevel(); got != "" {
		t.Errorf("GetThinkingLevel = %q, want empty", got)
	}
	if got := c.GetLastThoughtSummary(); got != "" {
		t.Errorf("GetLastThoughtSummary = %q, want empty", got)
	}
	if got := c.GetLastThoughtSignature(); got != "" {
		t.Errorf("GetLastThoughtSignature = %q, want empty", got)
	}
	if got := c.GetLastGroundingSources(); got != nil {
		t.Errorf("GetLastGroundingSources = %v, want nil", got)
	}
	if c.IsGoogleSearchEnabled() || c.IsURLContextEnabled() {
		t.Error("grounding probes reported enabled on a client that has no grounding at all")
	}
	// SchemaCapable is the exception: the default is true, because a client
	// that never opted out is capable, and answering false would switch
	// structured output off for every plain client.
	if !c.SchemaCapable() {
		t.Error("SchemaCapable = false for a client with no opinion; structured output would be disabled everywhere")
	}
}

func TestSettersForwardAndAreInertWithoutSupport(t *testing.T) {
	rich := newRichClient()
	c := wrapCore(t, rich)

	c.DisableSemaphore()
	if !rich.semaphoreDisabled {
		t.Error("DisableSemaphore did not reach the underlying client")
	}
	c.SetEnableGoogleSearch(false)
	c.SetEnableURLContext(false)
	if rich.searchEnabled || rich.urlContextEnabled {
		t.Error("grounding setters did not reach the underlying client")
	}
	c.SetURLContextURLs([]string{"https://example.test/c"})
	if len(rich.urls) != 1 || rich.urls[0] != "https://example.test/c" {
		t.Errorf("SetURLContextURLs did not forward: %v", rich.urls)
	}

	// A probe that succeeds and then calls a method forwarding to nothing is
	// indistinguishable from a probe that failed and skipped the call, which
	// is the whole argument for making these unconditional.
	plain := wrapCore(t, newFakeClient())
	plain.DisableSemaphore()
	plain.SetEnableGoogleSearch(true)
	plain.SetEnableURLContext(true)
	plain.SetURLContextURLs([]string{"ignored"})
	plain.SetCachedContent("ignored")
}

func TestGetModelPrefersTheUnderlyingButFallsBackToTheBroker(t *testing.T) {
	rich := newRichClient()
	c := wrapCore(t, rich)
	if got := c.GetModel(); got != "rich-model" {
		t.Errorf("GetModel = %q, want the underlying's answer", got)
	}

	// A client that reports no model leaves the broker authoritative: it is
	// the thing that tracked the model for counting, and receipts have to name
	// something.
	rich.model = ""
	if got := c.GetModel(); got != "fake-model" {
		t.Errorf("GetModel = %q, want the broker's tracked model", got)
	}
}

func TestSetModelKeepsCountingAndTheClientInStep(t *testing.T) {
	rich := newRichClient()
	c := wrapCore(t, rich)

	c.SetModel("switched-model")

	// Both halves matter. The underlying must actually switch, and the broker
	// must follow, or receipts and token counts after a mid-session switch are
	// attributed to a model that is no longer serving the conversation.
	if rich.model != "switched-model" {
		t.Errorf("underlying model = %q, want switched-model", rich.model)
	}
	if got := c.currentModel(); got != "switched-model" {
		t.Errorf("broker's model = %q, want switched-model", got)
	}
}

func TestSetModelIgnoresAnEmptyName(t *testing.T) {
	c := wrapCore(t, newFakeClient())
	c.SetModel("")
	// Clearing the broker's model on an empty set would leave receipts naming
	// the provider instead of a model.
	if got := c.currentModel(); got != "fake-model" {
		t.Errorf("model after SetModel(\"\") = %q, want it unchanged", got)
	}
}

func TestSetCachedContentForwards(t *testing.T) {
	rich := newRichClient()
	c := wrapCore(t, rich)
	c.SetCachedContent("handle-42")
	if rich.model != "cached:handle-42" {
		t.Errorf("SetCachedContent did not forward: %q", rich.model)
	}
}

func TestUnwrapReachesTheUnderlying(t *testing.T) {
	fake := newFakeClient()
	c := wrapCore(t, fake)
	if c.Unwrap() != types.LLMClient(fake) {
		t.Error("Unwrap did not return the wrapped client; call sites that assert on a " +
			"concrete type to pick a behaviour would take the wrong branch")
	}
}

func TestModelSwitchIsReflectedInReceipts(t *testing.T) {
	// The end-to-end reason SetModel tracks state at all.
	meter := testMeter(200000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newRichClient(), meter, sink)

	setter, ok := client.(interface{ SetModel(string) })
	if !ok {
		t.Fatal("the wrapped client lost SetModel")
	}
	setter.SetModel("mid-session-model")

	if _, err := client.CompleteWithSystem(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	r, ok := sink.last()
	if !ok {
		t.Fatal("no receipt emitted")
	}
	if r.Model != "mid-session-model" {
		t.Errorf("receipt model = %q, want mid-session-model — spend after a switch would be "+
			"attributed to the model that is no longer serving the conversation", r.Model)
	}
}
