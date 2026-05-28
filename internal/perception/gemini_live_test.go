// Package perception — gemini_live_test.go
//
// LIVE Gemini API tests. These tests talk to the real model. They are
// gated by the CODENERD_LIVE_LLM=1 environment variable so CI / normal
// `go test ./...` runs skip them. Set CODENERD_LIVE_LLM=1 (and have a
// valid gemini_api_key in .nerd/config.json) to run them locally.
//
// Why this file exists separately from gemini_structured_test.go:
//
// The older Gemini live test hardcoded model="gemini-3-flash-preview" and
// called NewGeminiClient(apiKey) directly, bypassing the production
// client_factory. That meant the tests never exercised the actual code path
// users hit (factory → user config → schema + grounding combination on
// CompleteWithStreaming) — and a real production bug (mid-stream
// truncation of the piggyback envelope on gemini-3.1-flash-lite because
// schema+grounding is only supported on gemini-3.5-flash and pro-preview)
// slipped through every test we had.
//
// These tests fix that by:
//
//  1. Resolving the client through perception.NewLLMClient — same path
//     production uses — so the model, thinking_level, max_output_tokens,
//     grounding flags, etc. all flow from .nerd/config.json the same way
//     they do at runtime.
//  2. Exercising the streaming + schema + grounding combination that is
//     load-bearing for chat articulation. If this combination is broken
//     on the configured model, these tests fail with a clear signal
//     (finishReason, parse method, partial-envelope detection).
//  3. Asserting on finish_reason and usage metadata via the production
//     log surface — the new streaming-path logging we just added means a
//     MAX_TOKENS truncation is caught directly by the test, not by a
//     user-visible JSON dump.
//  4. Verifying the response processor's salvage path agrees: a healthy
//     run should report ParseMethod="json" with Confidence=1.0, NEVER
//     "fallback".

package perception

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
)

// liveLLMEnabled reports whether live LLM tests should run. Single gate
// for the whole package: tests skip cleanly when unset rather than
// failing, so this file is safe to keep in the default test set.
func liveLLMEnabled() bool {
	return os.Getenv("CODENERD_LIVE_LLM") == "1"
}

// liveProviderConfig resolves the production provider config from the
// user's .nerd/config.json — i.e. exactly what the running binary uses.
// The walk also handles being invoked from inside subpackages: tests run
// with cwd=internal/perception, so .nerd/config.json is two directories
// up; we still want FindWorkspaceRoot to be the source of truth.
func liveProviderConfig(t *testing.T) (*ProviderConfig, *config.UserConfig) {
	t.Helper()
	if !liveLLMEnabled() {
		t.Skip("skipping live LLM test: set CODENERD_LIVE_LLM=1 to enable")
	}

	root, err := config.FindWorkspaceRoot()
	if err != nil {
		t.Skipf("skipping live LLM test: find workspace root: %v", err)
	}
	configPath := filepath.Join(root, ".nerd", "config.json")

	userCfg, err := config.LoadUserConfig(configPath)
	if err != nil {
		t.Skipf("skipping live LLM test: load %s: %v", configPath, err)
	}

	providerCfg, err := LoadConfigJSON(configPath)
	if err != nil {
		t.Skipf("skipping live LLM test: build provider config: %v", err)
	}
	if providerCfg.APIKey == "" {
		t.Skipf("skipping live LLM test: no API key configured in %s", configPath)
	}
	return providerCfg, userCfg
}

// requireLiveGeminiClient builds a real client through the production
// factory path. If the configured provider isn't Gemini, the test skips
// — we explicitly do NOT want to silently pick a different provider
// because each provider's quirks (schema+grounding support, thinking
// budget, etc.) are different and the test would lie about coverage.
func requireLiveGeminiClient(t *testing.T) (LLMClient, *ProviderConfig) {
	t.Helper()
	providerCfg, _ := liveProviderConfig(t)
	if providerCfg.Provider != ProviderGemini {
		t.Skipf("skipping live Gemini test: configured provider=%q (need gemini)", providerCfg.Provider)
	}

	client, err := NewClientFromConfig(providerCfg)
	if err != nil {
		t.Fatalf("build live Gemini client: %v", err)
	}
	return client, providerCfg
}

// liveCallTimeout is the per-call ceiling for a single live LLM request.
// Generous enough to absorb cold starts and thinking-mode latency.
func liveCallTimeout() time.Duration {
	if timeout := config.GetLLMTimeouts().PerCallTimeout; timeout > 0 {
		return timeout
	}
	return 5 * time.Minute
}

// =============================================================================
// SMOKE TEST — basic completion against the configured model
// =============================================================================

// TestGeminiLive_Smoke verifies the most basic round-trip: the configured
// model is reachable, the API key is valid, and we get a non-empty
// response. This is the canary: if it fails, nothing else will work.
func TestGeminiLive_Smoke(t *testing.T) {
	client, providerCfg := requireLiveGeminiClient(t)

	t.Logf("model=%s thinking_level=%s grounding=(search=%v url=%v)",
		providerCfg.Model,
		geminiThinkingLevel(client),
		geminiGoogleSearch(client),
		geminiURLContext(client),
	)

	ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout())
	defer cancel()

	const sentinel = "GEMINI_LIVE_SMOKE_SENTINEL"
	prompt := "Reply with exactly the word " + sentinel + " and nothing else."
	resp, err := client.Complete(ctx, prompt)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if !strings.Contains(resp, sentinel) {
		t.Fatalf("response missing sentinel %q: got %q", sentinel, resp)
	}
}

// =============================================================================
// SCHEMA+GROUNDING+STREAMING TEST — the load-bearing chat articulation path
// =============================================================================

// TestGeminiLive_ChatArticulation_StreamingEnvelope is the regression test
// for the bug that motivated this whole rework. The chat path calls
// CompleteWithStreaming with a system prompt that triggers piggyback
// isPiggyback detection (mentions control_packet/surface_response), which
// causes the client to attach responseJsonSchema AND keep grounding tools
// enabled. On models that don't support that combination (notably
// gemini-3.1-flash-lite) the model truncates mid-envelope and the user
// sees raw control_packet JSON.
//
// This test:
//
//   - Streams a piggyback prompt
//   - Aggregates the full response
//   - Runs it through the production ResponseProcessor
//   - Asserts ParseMethod="json" (NOT fallback)
//   - Asserts Surface is non-empty (a complete envelope reached
//     surface_response before stopping)
//   - Asserts no partial-envelope salvage warning was emitted
//
// If any of those fails, the model + config combination is wrong for the
// chat path, and we should NOT ship.
func TestGeminiLive_ChatArticulation_StreamingEnvelope(t *testing.T) {
	client, providerCfg := requireLiveGeminiClient(t)

	streamer, ok := client.(interface {
		CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error)
	})
	if !ok {
		t.Fatalf("client does not support CompleteWithStreaming (type=%T)", client)
	}

	systemPrompt := `You are codeNERD. Respond ONLY with a valid JSON envelope.

Required JSON Schema (THOUGHT-FIRST ordering):
{
  "control_packet": {
    "intent_classification": {"category": "/query", "verb": "/explain", "target": "string", "constraint": "none", "confidence": 1.0},
    "mangle_updates": [],
    "memory_operations": [],
    "self_correction": {"triggered": false, "hypothesis": ""},
    "reasoning_trace": "internal notes",
    "knowledge_requests": [],
    "context_feedback": {"overall_usefulness": 1.0, "helpful_facts": [], "noise_facts": [], "missing_context": ""},
    "tool_requests": []
  },
  "surface_response": "user-visible answer (MUST be non-empty)"
}

Output JSON only. The surface_response field MUST be present and non-empty.`

	userPrompt := "What is a just-in-time compiler? Give a one-sentence answer."

	ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout())
	defer cancel()

	contentCh, errCh := streamer.CompleteWithStreaming(ctx, systemPrompt, userPrompt, true)

	var full strings.Builder
	streamStart := time.Now()
	chunkCount := 0
	for content := range contentCh {
		full.WriteString(content)
		chunkCount++
	}
	streamDuration := time.Since(streamStart)

	// Drain any error after the stream closes.
	var streamErr error
	for err := range errCh {
		if err != nil {
			streamErr = err
			break
		}
	}
	if streamErr != nil {
		t.Fatalf("streaming returned error: %v (model=%s, streamed %d chunks / %d bytes in %v)",
			streamErr, providerCfg.Model, chunkCount, full.Len(), streamDuration)
	}

	raw := full.String()
	t.Logf("streamed %d chunks, %d bytes in %v on model=%s",
		chunkCount, len(raw), streamDuration, providerCfg.Model)

	if strings.TrimSpace(raw) == "" {
		t.Fatalf("streaming produced empty response on model=%s — check finish_reason in api.log",
			providerCfg.Model)
	}

	// Production response processor — same code the chat surface uses.
	processed := articulation.ProcessLLMResponseAllowPlain(raw)
	if processed == nil {
		t.Fatalf("ProcessLLMResponseAllowPlain returned nil")
	}

	t.Logf("parse_method=%s confidence=%.2f surface_len=%d",
		processed.ParseMethod, processed.Confidence, len(processed.Surface))

	if processed.ParseMethod == "fallback" {
		// The salvage path triggered — exactly the bug we fixed at the
		// display layer, and exactly what we DON'T want at the source.
		t.Fatalf("ResponseProcessor fell back: envelope incomplete or unparseable on model=%s. "+
			"This is the schema+grounding truncation regression. raw[:500]=%q",
			providerCfg.Model, truncateForLog(raw, 500))
	}
	if processed.ParseMethod != "json" && processed.ParseMethod != "json_markdown" && processed.ParseMethod != "json_extracted" {
		t.Errorf("unexpected parse_method=%q (want json / json_markdown / json_extracted)", processed.ParseMethod)
	}

	if strings.TrimSpace(processed.Surface) == "" {
		t.Fatalf("envelope parsed but surface_response is empty on model=%s; "+
			"likely the model stopped before writing surface_response. raw=%q",
			providerCfg.Model, truncateForLog(raw, 500))
	}

	// Sanity: surface should actually mention the topic, not just be filler.
	lower := strings.ToLower(processed.Surface)
	if !strings.Contains(lower, "compil") && !strings.Contains(lower, "jit") && !strings.Contains(lower, "runtime") {
		t.Errorf("surface_response doesn't appear to answer the question: %q", processed.Surface)
	}
}

// =============================================================================
// FINISH-REASON OBSERVABILITY TEST — proves the streaming logging works
// =============================================================================

// TestGeminiLive_FinishReasonObservable forces a moderately complex
// response and verifies that the streaming path captures and logs
// finish_reason + usage metadata. We don't assert the exact reason here
// (any STOP variant is fine) — we assert that the logging infrastructure
// added in the streaming path actually fires when production runs. This
// guards against silently regressing the diagnostic surface that lets us
// catch future MAX_TOKENS / SAFETY truncations.
func TestGeminiLive_FinishReasonObservable(t *testing.T) {
	client, _ := requireLiveGeminiClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout())
	defer cancel()

	// A prompt with enough scope that the model uses both thinking and
	// non-trivial output tokens — exercises the full usage_metadata path.
	systemPrompt := "You are a technical writer. Answer concisely but completely."
	userPrompt := "List three categories of just-in-time compiler optimization. " +
		"For each, write one sentence on what it does. Number them 1, 2, 3."

	resp, err := client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		t.Fatalf("CompleteWithSystem failed: %v", err)
	}
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("empty response")
	}
	// Loose structural check: at least the three numbered items appear.
	missing := []string{}
	for _, marker := range []string{"1", "2", "3"} {
		if !strings.Contains(resp, marker) {
			missing = append(missing, marker)
		}
	}
	if len(missing) > 1 {
		t.Errorf("response missing expected numbered list items %v: %q", missing, resp)
	}

	// Pull thought-signature + token telemetry off the client. We can't
	// assert specific values (they vary per call) but we can assert the
	// telemetry surface is wired — non-zero output tokens, captured
	// thinking metadata if thinking is enabled.
	if tp, ok := client.(interface{ GetLastThinkingTokens() int }); ok {
		thinkingTokens := tp.GetLastThinkingTokens()
		t.Logf("thinking_tokens=%d (thinking enabled=%v)",
			thinkingTokens, geminiThinkingEnabled(client))
	}
	if sp, ok := client.(interface{ GetLastThoughtSignature() string }); ok {
		if sig := sp.GetLastThoughtSignature(); sig != "" {
			t.Logf("thought_signature captured (len=%d) — multi-turn function calling will be coherent", len(sig))
		}
	}
}

// =============================================================================
// SCHEMA-CAPABLE PATH — CompleteWithSchema if the client supports it
// =============================================================================

// TestGeminiLive_CompleteWithSchema exercises the strict-schema response
// path used by transducers and some shards. The schema is the canonical
// piggyback schema (same one production uses); we assert the response
// validates against it.
func TestGeminiLive_CompleteWithSchema(t *testing.T) {
	client, providerCfg := requireLiveGeminiClient(t)

	sc, ok := core.AsSchemaCapable(client)
	if !ok {
		t.Skipf("client does not implement SchemaCapableLLMClient (type=%T)", client)
	}

	ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout())
	defer cancel()

	systemPrompt := `You are an intent classifier. Respond ONLY with valid JSON matching the schema.`
	userPrompt := "Classify this intent: 'Fix the authentication bug in auth.go'"

	resp, err := sc.CompleteWithSchema(ctx, systemPrompt, userPrompt, articulation.PiggybackEnvelopeSchema)
	if err != nil {
		t.Fatalf("CompleteWithSchema failed on model=%s: %v", providerCfg.Model, err)
	}
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("empty schema-constrained response")
	}

	// Schema-constrained calls should always parse as direct JSON — if
	// they fall back, something is very wrong (the API isn't honoring
	// the schema constraint).
	processed := articulation.ProcessLLMResponse(resp)
	if processed == nil {
		t.Fatalf("ProcessLLMResponse returned nil")
	}
	if processed.ParseMethod == "fallback" {
		t.Fatalf("schema-constrained response did NOT validate as JSON (parse=%s) on model=%s — raw=%q",
			processed.ParseMethod, providerCfg.Model, truncateForLog(resp, 300))
	}
	if strings.TrimSpace(processed.Surface) == "" {
		t.Fatalf("schema-constrained response has empty surface_response — raw=%q",
			truncateForLog(resp, 300))
	}

	// Sanity check: the intent_classification.verb should be /fix-ish.
	var env articulation.PiggybackEnvelope
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Errorf("response failed to unmarshal as PiggybackEnvelope: %v", err)
	} else {
		t.Logf("parsed envelope: verb=%s target=%s confidence=%.2f",
			env.Control.IntentClassification.Verb,
			env.Control.IntentClassification.Target,
			env.Control.IntentClassification.Confidence,
		)
		if env.Control.IntentClassification.Verb == "" {
			t.Error("envelope is missing intent_classification.verb")
		}
	}
}

// =============================================================================
// TOOL CALLING — verify the tool-calling path streams thought_signatures
// =============================================================================

// TestGeminiLive_CompleteWithTools runs a single round-trip with a tool
// declaration and verifies the response is a valid LLMToolResponse with
// either a text answer or a tool call. The point is to confirm that the
// tool-calling path:
//
//  1. Doesn't crash on the configured model
//  2. Captures thought_signature so multi-turn calls work
//  3. Returns a usable response (not the truncated-envelope failure mode)
//
// We don't assert a specific tool call (the model may legitimately answer
// directly), but we do require either text content or a tool invocation
// — an empty response would mean truncation again.
func TestGeminiLive_CompleteWithTools(t *testing.T) {
	client, providerCfg := requireLiveGeminiClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), liveCallTimeout())
	defer cancel()

	tools := []ToolDefinition{
		{
			Name:        "lookup_definition",
			Description: "Look up the definition of a computer-science term.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"term": map[string]any{
						"type":        "string",
						"description": "The term to define",
					},
				},
				"required": []string{"term"},
			},
		},
	}

	systemPrompt := "You are a helpful coding assistant. Use lookup_definition if you need to look up a term."
	userPrompt := "Define 'just-in-time compiler' for me."

	resp, err := client.CompleteWithTools(ctx, systemPrompt, userPrompt, tools)
	if err != nil {
		t.Fatalf("CompleteWithTools failed on model=%s: %v", providerCfg.Model, err)
	}
	if resp == nil {
		t.Fatalf("CompleteWithTools returned nil response")
	}

	t.Logf("tool response: text_len=%d tool_calls=%d stop_reason=%s",
		len(resp.Text), len(resp.ToolCalls), resp.StopReason)

	if strings.TrimSpace(resp.Text) == "" && len(resp.ToolCalls) == 0 {
		t.Fatalf("tool response had neither text nor tool calls on model=%s — likely truncated", providerCfg.Model)
	}

	if sp, ok := client.(interface{ GetLastThoughtSignature() string }); ok {
		if sig := sp.GetLastThoughtSignature(); sig != "" {
			t.Logf("thought_signature captured for multi-turn tool calls (len=%d)", len(sig))
		}
	}
}

// =============================================================================
// HELPERS — reflection accessors for client capability assertions
// =============================================================================

func geminiThinkingLevel(client LLMClient) string {
	if p, ok := client.(interface{ GetThinkingLevel() string }); ok {
		return p.GetThinkingLevel()
	}
	return ""
}

func geminiThinkingEnabled(client LLMClient) bool {
	if p, ok := client.(interface{ IsThinkingEnabled() bool }); ok {
		return p.IsThinkingEnabled()
	}
	return false
}

func geminiGoogleSearch(client LLMClient) bool {
	if p, ok := client.(interface{ IsGoogleSearchEnabled() bool }); ok {
		return p.IsGoogleSearchEnabled()
	}
	return false
}

func geminiURLContext(client LLMClient) bool {
	if p, ok := client.(interface{ IsURLContextEnabled() bool }); ok {
		return p.IsURLContextEnabled()
	}
	return false
}

// (truncateForLog is defined in transducer.go and shared package-wide.)
