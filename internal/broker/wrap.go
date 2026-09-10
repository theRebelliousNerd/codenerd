package broker

import (
	"context"

	"codenerd/internal/types"
)

// Wrap installs metering in front of an LLM client while preserving the exact
// set of optional interfaces the client implements.
//
// Why this is not one wrapper type:
//
// Several call sites choose a control flow by probing the client. The session
// executor takes the native multi-turn tool loop when the client implements
// types.ToolResultsProvider and a single-shot path otherwise; core.AsSchemaCapable
// selects structured output; the chat articulation path streams reasoning when
// the client can. A single wrapper that always exposed those three methods would
// tell every one of those probes "yes" — and Gemini, which deliberately uses a
// synthesized Piggyback envelope instead of native tool results, would be routed
// into a path it cannot serve. The bug would appear as tool calls silently going
// nowhere, far from this file.
//
// So Wrap composes one of eight shapes from the three gating capabilities. The
// unconditional forwarders in passthrough.go carry everything else, where a
// probe that succeeds and forwards to nothing is indistinguishable from a probe
// that failed.
//
// Returns an error rather than a degraded client if the configuration cannot
// meter: a broker that cannot count enforces nothing, and returning one silently
// would recreate the exact condition this package exists to end.
func Wrap(underlying types.LLMClient, cfg Config) (types.LLMClient, error) {
	if underlying == nil {
		return nil, ErrNoUnderlying
	}
	if cfg.Counter == nil {
		return nil, ErrNoCounter
	}
	if cfg.Ledger == nil {
		cfg.Ledger = NewLedger(LedgerConfig{})
	}

	c := &core{underlying: underlying, cfg: cfg, model: cfg.Model}
	base := baseClient{core: c}

	_, hasToolResults := underlying.(types.ToolResultsProvider)
	_, hasSchema := underlying.(interface {
		CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
	})
	_, hasThoughts := underlying.(interface {
		CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error)
	})

	switch {
	case hasToolResults && hasSchema && hasThoughts:
		return clientTRSCTH{base}, nil
	case hasToolResults && hasSchema:
		return clientTRSC{base}, nil
	case hasToolResults && hasThoughts:
		return clientTRTH{base}, nil
	case hasSchema && hasThoughts:
		return clientSCTH{base}, nil
	case hasToolResults:
		return clientTR{base}, nil
	case hasSchema:
		return clientSC{base}, nil
	case hasThoughts:
		return clientTH{base}, nil
	default:
		return base, nil
	}
}

// Base reaches through any chain of broker wrappers to the innermost client.
//
// Call sites that assert on a concrete client type to make a behavioural choice
// — cmd/nerd/chat tags the Codex CLI engine so JIT can select engine-specific
// prompt atoms — must use this rather than asserting on whatever is outermost.
// It also unwraps any other decorator that exposes Unwrap, so it keeps working
// as more layers are added.
func Base(client types.LLMClient) types.LLMClient {
	var last types.LLMClient
	Walk(client, func(layer types.LLMClient) bool {
		last = layer
		return true
	})
	return last
}

// Walk calls fn on client and on each layer beneath it, outermost first,
// stopping early when fn returns false.
//
// It is the one place the chain is traversed. Before this existed the loop was
// copied into Base and IsBrokered, and cmd/nerd/chat had two more walks of its
// own that switched on two decorator types by name and did not know the broker
// wrapper existed at all -- correct only because every layer happened to forward
// the method being looked for, and silently wrong for any layer that did not.
// A traversal written four times is four things to remember to update when a
// decorator is added, which is precisely the update nobody remembers.
//
// A nil client, or a layer whose Unwrap returns nil, ends the walk: nil is not a
// layer and must never be handed to fn.
func Walk(client types.LLMClient, fn func(types.LLMClient) bool) {
	for i := 0; i < maxUnwrapDepth && client != nil; i++ {
		if !fn(client) {
			return
		}
		unwrapper, ok := client.(interface{ Unwrap() types.LLMClient })
		if !ok {
			return
		}
		client = unwrapper.Unwrap()
	}
}

// maxUnwrapDepth bounds Walk against a decorator chain that cycles. A malformed
// Unwrap that returns its own receiver would otherwise spin forever inside a
// prompt-assembly path.
const maxUnwrapDepth = 16

// IsBrokered reports whether client has metering installed anywhere in its
// decorator chain. The wiring test uses this to prove no construction path
// escapes the broker.
func IsBrokered(client types.LLMClient) bool {
	var found bool
	Walk(client, func(layer types.LLMClient) bool {
		_, found = layer.(brokered)
		return !found
	})
	return found
}

// brokered marks the wrapper shapes. It is unexported so nothing outside this
// package can claim to be metered without being metered.
type brokered interface{ brokerMarker() }

// baseClient exposes types.LLMClient plus every unconditional forwarder.
//
// It embeds *core, which is safe precisely because core's three gating methods
// are unexported: completeWithToolResults does not satisfy
// types.ToolResultsProvider, so embedding cannot accidentally advertise a
// capability the underlying client lacks.
type baseClient struct{ *core }

func (baseClient) brokerMarker() {}

type clientTR struct{ baseClient }

// CompleteWithToolResults implements types.ToolResultsProvider.
func (c clientTR) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return c.completeWithToolResults(ctx, systemPrompt, history, tools)
}

type clientSC struct{ baseClient }

// CompleteWithSchema implements core.SchemaCapableLLMClient.
func (c clientSC) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return c.completeWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
}

type clientTH struct{ baseClient }

// CompleteWithStreamingAndThoughts implements core.LLMStreamingWithThoughts.
func (c clientTH) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error) {
	return c.completeWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)
}

type clientTRSC struct{ baseClient }

func (c clientTRSC) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return c.completeWithToolResults(ctx, systemPrompt, history, tools)
}

func (c clientTRSC) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return c.completeWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
}

type clientTRTH struct{ baseClient }

func (c clientTRTH) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return c.completeWithToolResults(ctx, systemPrompt, history, tools)
}

func (c clientTRTH) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error) {
	return c.completeWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)
}

type clientSCTH struct{ baseClient }

func (c clientSCTH) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return c.completeWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
}

func (c clientSCTH) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error) {
	return c.completeWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)
}

type clientTRSCTH struct{ baseClient }

func (c clientTRSCTH) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return c.completeWithToolResults(ctx, systemPrompt, history, tools)
}

func (c clientTRSCTH) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	return c.completeWithSchema(ctx, systemPrompt, userPrompt, jsonSchema)
}

func (c clientTRSCTH) CompleteWithStreamingAndThoughts(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan string, <-chan error) {
	return c.completeWithStreamingAndThoughts(ctx, systemPrompt, userPrompt, enableThinking)
}

// Compile-time proof that every shape is an LLMClient and that the gating
// interfaces appear on exactly the shapes that should carry them.
var (
	_ types.LLMClient           = baseClient{}
	_ types.LLMClient           = clientTR{}
	_ types.ToolResultsProvider = clientTR{}
	_ types.ToolResultsProvider = clientTRSC{}
	_ types.ToolResultsProvider = clientTRTH{}
	_ types.ToolResultsProvider = clientTRSCTH{}
	_ types.GroundingController = baseClient{}
)
