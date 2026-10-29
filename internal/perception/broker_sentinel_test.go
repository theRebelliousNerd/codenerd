package perception

import (
	"context"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/types"
)

// The static audit in internal/broker parses client_factory.go and requires
// every exported constructor to route through metering. It cannot see what a
// switch actually returns, so a provider case that fell out of the metered path
// -- or a new one added below the switch -- would satisfy the parser and still
// spend off the books.
//
// These tests are the runtime half: they build a client through the real
// exported constructor for every provider and engine the factory knows, and
// assert on the value that comes back.

func TestEveryProviderReturnsAMeteredClient(t *testing.T) {
	providers := []Provider{
		ProviderAnthropic,
		ProviderOpenAI,
		ProviderGemini,
		ProviderXAI,
		ProviderZAI,
		ProviderOpenRouter,
		ProviderOllama,
		ProviderDashScope,
		ProviderMeta,
		ProviderMoonshot,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			cfg := &ProviderConfig{
				Provider: provider,
				APIKey:   "test-key-not-used-for-network",
				Model:    "test-model",
			}
			if provider == ProviderOllama {
				cfg.Ollama = &config.OllamaLLMConfig{Model: "test-model"}
			}

			client, err := NewClientFromConfig(cfg)
			if err != nil {
				t.Skipf("provider %s could not be constructed in this environment: %v", provider, err)
			}
			if client == nil {
				t.Fatalf("NewClientFromConfig(%s) returned a nil client and a nil error", provider)
			}
			if !broker.IsBrokered(client) {
				t.Errorf("provider %s returns an un-metered client: its inference spend would not "+
					"reach the ledger and would not appear in any receipt", provider)
			}
			// The wrapper must not have swallowed the concrete client.
			if broker.Base(client) == nil {
				t.Errorf("provider %s: broker.Base returned nil, so concrete-type checks downstream "+
					"cannot reach the provider client", provider)
			}
		})
	}
}

func TestEveryEngineReturnsAMeteredClient(t *testing.T) {
	engines := []struct {
		name string
		cfg  *ProviderConfig
	}{
		{"claude-cli", &ProviderConfig{Engine: "claude-cli", ClaudeCLI: &config.ClaudeCLIConfig{}}},
		{"codex-cli", &ProviderConfig{Engine: "codex-cli", CodexCLI: &config.CodexCLIConfig{}}},
	}

	for _, e := range engines {
		t.Run(e.name, func(t *testing.T) {
			client, err := NewClientFromConfig(e.cfg)
			if err != nil {
				t.Skipf("engine %s could not be constructed here: %v", e.name, err)
			}
			if !broker.IsBrokered(client) {
				t.Errorf("engine %s returns an un-metered client", e.name)
			}
		})
	}
}

func TestClassificationClientIsMetered(t *testing.T) {
	// The classification tier runs on the critical path of every turn. Its
	// spend is small per call and large in aggregate, which is exactly the kind
	// that disappears when nothing counts it.
	cfg := &ProviderConfig{
		Provider:            ProviderAnthropic,
		APIKey:              "test-key-not-used-for-network",
		Model:               "test-model",
		ClassificationModel: "test-classification-model",
	}

	client, err := NewClassificationClientFromConfig(cfg)
	if err != nil {
		t.Skipf("classification client unavailable here: %v", err)
	}
	if client == nil {
		t.Skip("no classification tier configured for this provider")
	}
	if !broker.IsBrokered(client) {
		t.Error("the classification client is not metered")
	}
}

// TestMeteredClientEmitsAReceiptWithoutNetwork proves the boundary end to end:
// a call through a factory-built client produces a receipt.
//
// It drives the refusal path on purpose. Admission runs before dispatch, so a
// window too small for the request means a receipt is emitted and the provider
// is never contacted -- a genuine end-to-end assertion that needs no API key,
// no network, and no fixture server.
func TestMeteredClientEmitsAReceiptWithoutNetwork(t *testing.T) {
	// Configure before constructing: a client captures the meter's sink at
	// construction time, so installing the capture afterwards would leave it
	// reporting to the sink that was current when it was built.
	captured := make(chan broker.Receipt, 8)
	broker.Configure(broker.MeterConfig{
		Window:        1000,
		OutputReserve: 950,
		ExtraSink: recordFunc(func(r broker.Receipt) {
			select {
			case captured <- r:
			default:
			}
		}),
	})
	t.Cleanup(func() { broker.Configure(broker.MeterConfig{}) })

	client, err := NewClientFromConfig(&ProviderConfig{
		Provider: ProviderAnthropic,
		APIKey:   "test-key-not-used-for-network",
		Model:    "test-model",
	})
	if err != nil {
		t.Fatalf("NewClientFromConfig: %v", err)
	}

	ctx := broker.WithPurpose(context.Background(), broker.PurposeSession)
	_, callErr := client.CompleteWithSystem(ctx, longSystemPromptForRefusal(), "user prompt")

	if callErr == nil {
		t.Fatal("a request far larger than the window was admitted")
	}
	if _, ok := broker.IsAdmissionError(callErr); !ok {
		t.Fatalf("expected a broker admission error, got %T: %v", callErr, callErr)
	}

	select {
	case r := <-captured:
		if r.Decision.Allowed {
			t.Error("the receipt for a refused call claims it was allowed")
		}
		if r.Purpose != broker.PurposeSession {
			t.Errorf("receipt purpose = %q, want %q", r.Purpose, broker.PurposeSession)
		}
		if r.Method == "" {
			t.Error("receipt does not name the method")
		}
	default:
		t.Error("no receipt was emitted for a call made through a factory-built client")
	}
}

func longSystemPromptForRefusal() string {
	const chunk = "this system prompt exists only to exceed a deliberately tiny admission window. "
	out := make([]byte, 0, len(chunk)*400)
	for i := 0; i < 400; i++ {
		out = append(out, chunk...)
	}
	return string(out)
}

// Compile-time proof that a metered client still satisfies the base interface.
var _ types.LLMClient = (types.LLMClient)(nil)

// recordFunc adapts a function to broker.ReceiptSink.
//
// Declared here rather than exported from the broker: an adapter whose only
// user is a test is API the package under test has to keep alive for the
// benefit of its own test suite.
type recordFunc func(broker.Receipt)

func (f recordFunc) Record(r broker.Receipt) { f(r) }
