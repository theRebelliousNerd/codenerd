package perception

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// failingGeminiClient fails every completion, optionally as a transient outage.
type failingGeminiClient struct {
	baseMockLLMClient
	err   error
	calls int
}

func (c *failingGeminiClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.calls++
	return "", c.err
}

// textGeminiClient returns fixed free-form text (never schema-capable).
type textGeminiClient struct {
	baseMockLLMClient
	response string
	calls    int
}

func (c *textGeminiClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.calls++
	return c.response, nil
}

// schemaGeminiStub exercises the structured-output path.
type schemaGeminiStub struct {
	baseMockLLMClient
	response    string
	schemaCalls int
	freeCalls   int
}

func (c *schemaGeminiStub) SchemaCapable() bool { return true }

func (c *schemaGeminiStub) CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error) {
	c.schemaCalls++
	return c.response, nil
}

func (c *schemaGeminiStub) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	c.freeCalls++
	return c.response, nil
}

func geminiTestBase(t *testing.T, client LLMClient, kernel *core.RealKernel) *GeminiThinkingTransducer {
	t.Helper()
	base, ok := NewUnderstandingTransducer(client).(*UnderstandingTransducer)
	if !ok {
		t.Fatalf("NewUnderstandingTransducer returned %T, want *UnderstandingTransducer", base)
	}
	if kernel != nil {
		base.SetKernel(kernel)
	}
	return NewGeminiThinkingTransducer(base)
}

// TestGeminiTransducer_DegradesOnLLMError pins the shared no-error contract:
// a failed classification degrades to /explain with a nil error, and a
// transient outage marks TransientFailure — exactly like the base path.
func TestGeminiTransducer_DegradesOnLLMError(t *testing.T) {
	ctx := context.Background()

	tc := geminiTestBase(t, &failingGeminiClient{err: errors.New("boom")}, nil)
	intent, err := tc.ParseIntentWithContext(ctx, "do the thing", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext returned error %v, want degraded intent", err)
	}
	if intent.Verb != "/explain" || intent.Category != "/query" {
		t.Errorf("degraded intent=%s %s, want /query /explain", intent.Category, intent.Verb)
	}
	if intent.TransientFailure {
		t.Error("plain error marked TransientFailure, want false")
	}

	tc = geminiTestBase(t, &failingGeminiClient{err: ErrLLMUnavailable}, nil)
	intent, err = tc.ParseIntentWithContext(ctx, "do the thing", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext returned error %v, want degraded intent", err)
	}
	if !intent.TransientFailure {
		t.Error("transient outage did not mark TransientFailure")
	}
	if !strings.Contains(intent.Response, "briefly overloaded") {
		t.Errorf("transient Response=%q, want outage wording", intent.Response)
	}
}

// TestGeminiTransducer_DegradesOnUnparseable pins that thinking output with
// no JSON degrades instead of erroring the turn.
func TestGeminiTransducer_DegradesOnUnparseable(t *testing.T) {
	tc := geminiTestBase(t, &textGeminiClient{response: "I pondered deeply but wrote no JSON at all"}, nil)
	intent, err := tc.ParseIntentWithContext(context.Background(), "do the thing", nil)
	if err != nil {
		t.Fatalf("ParseIntentWithContext returned error %v, want degraded intent", err)
	}
	if intent.Verb != "/explain" {
		t.Errorf("degraded verb=%s, want /explain", intent.Verb)
	}
}

// TestGeminiTransducer_EmptyInputShortCircuits pins that blank input never
// reaches the model on the Gemini path either.
func TestGeminiTransducer_EmptyInputShortCircuits(t *testing.T) {
	client := &textGeminiClient{response: "should never be called"}
	tc := geminiTestBase(t, client, nil)
	intent, err := tc.ParseIntentWithContext(context.Background(), "   ", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Response != "Input is empty" {
		t.Errorf("Response=%q, want empty-input fallback", intent.Response)
	}
	if client.calls != 0 {
		t.Errorf("LLM called %d times for blank input, want 0", client.calls)
	}
}

const geminiSchemaEnvelope = `{
  "understanding": {
    "primary_intent": "document",
    "semantic_type": "definition",
    "action_type": "document",
    "domain": "general",
    "scope": {"level": "file", "target": "auth.go"},
    "confidence": 0.9,
    "signals": {"is_question": false},
    "suggested_approach": {"mode": "normal", "primary_shard": "coder"}
  },
  "surface_response": "Documenting."
}`

// TestGeminiTransducer_SchemaPathDerivesRouting pins that the structured path
// runs the same Mangle routing as the base: the canned envelope suggests
// coder, but the document affinity must derive researcher.
func TestGeminiTransducer_SchemaPathDerivesRouting(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	client := &schemaGeminiStub{response: geminiSchemaEnvelope}
	tc := geminiTestBase(t, client, kernel)
	intent, err := tc.ParseIntentWithContext(context.Background(), "write docs", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.schemaCalls != 1 || client.freeCalls != 0 {
		t.Errorf("schema=%d free=%d calls, want schema path only", client.schemaCalls, client.freeCalls)
	}
	if intent.Verb != "/document" || intent.Category != "/mutation" {
		t.Errorf("intent=%s %s, want /mutation /document", intent.Category, intent.Verb)
	}
	last := tc.GetLastUnderstanding()
	if last == nil || last.Routing == nil {
		t.Fatal("last understanding or its routing is nil after schema path")
	}
	if last.Routing.PrimaryShard != "researcher" {
		t.Errorf("PrimaryShard=%q, want %q (Mangle-derived, not suggested coder)",
			last.Routing.PrimaryShard, "researcher")
	}
}

// TestGeminiTransducer_FreeFormParsesThinkingPreamble pins the
// "[Thoughts...] { JSON }" shape: the last JSON object wins, and routing
// still derives on the fallback path.
func TestGeminiTransducer_FreeFormParsesThinkingPreamble(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	raw := "Let me think carefully about this request.\n" +
		"The user wants documentation written.\n" + geminiSchemaEnvelope
	tc := geminiTestBase(t, &textGeminiClient{response: raw}, kernel)
	intent, err := tc.ParseIntentWithContext(context.Background(), "write docs", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Verb != "/document" {
		t.Errorf("verb=%s, want /document (last-JSON parse)", intent.Verb)
	}
	if last := tc.GetLastUnderstanding(); last == nil || last.Routing == nil {
		t.Fatal("routing missing after free-form path")
	} else if last.Routing.PrimaryShard != "researcher" {
		t.Errorf("PrimaryShard=%q, want researcher", last.Routing.PrimaryShard)
	}
}

// TestTruncateClassificationInput_RuneSafe pins that truncation never splits
// a multi-byte rune and reports whether it cut.
func TestTruncateClassificationInput_RuneSafe(t *testing.T) {
	short := "hello"
	if got, cut := truncateClassificationInput(short); cut || got != short {
		t.Errorf("short input cut or changed: %q %v", got, cut)
	}
	multi := strings.Repeat("héllo ", 10000) // 60000 runes, multibyte
	got, cut := truncateClassificationInput(multi)
	if !cut {
		t.Fatal("oversized input not cut")
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Error("truncation produced U+FFFD replacement chars")
	}
	body := strings.TrimSuffix(got, "... [Input truncated due to length]")
	if n := len([]rune(body)); n != maxClassificationInputChars {
		t.Errorf("truncated to %d runes, want %d", n, maxClassificationInputChars)
	}
}

// TestUnderstandingSchema_Structure pins the Gemini structured-output schema:
// valid JSON, understanding required, and depth within the Gemini 3 limit.
func TestUnderstandingSchema_Structure(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(understandingSchema), &schema); err != nil {
		t.Fatalf("understandingSchema is not valid JSON: %v", err)
	}
	var walk func(v any, depth int) int
	walk = func(v any, d int) int {
		max := d
		if m, ok := v.(map[string]any); ok {
			for _, child := range m {
				if got := walk(child, d+1); got > max {
					max = got
				}
			}
		}
		return max
	}
	if depth := walk(schema, 0); depth > 12 {
		t.Errorf("schema depth=%d, exceeds comfortable Gemini 3 margin", depth)
	}
	props, _ := schema["properties"].(map[string]any)
	u, _ := props["understanding"].(map[string]any)
	uprops, _ := u["properties"].(map[string]any)
	for _, field := range []string{"primary_intent", "semantic_type", "action_type", "domain", "signals", "suggested_approach"} {
		if _, ok := uprops[field]; !ok {
			t.Errorf("schema understanding.properties lacks %q", field)
		}
	}
}
