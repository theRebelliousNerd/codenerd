package perception

import (
	"encoding/json"
	"testing"
)

// findObjectsMissingAdditionalProperties walks a JSON-schema node and reports
// the paths of object nodes that declare properties but omit
// "additionalProperties": false.
func findObjectsMissingAdditionalProperties(node any, path string, out *[]string) {
	switch typed := node.(type) {
	case map[string]any:
		if typed["type"] == "object" {
			if _, hasProps := typed["properties"]; hasProps {
				if ap, ok := typed["additionalProperties"]; !ok || ap != false {
					*out = append(*out, path)
				}
			}
		}
		for k, v := range typed {
			findObjectsMissingAdditionalProperties(v, path+"/"+k, out)
		}
	case []any:
		for i, item := range typed {
			findObjectsMissingAdditionalProperties(item, path+"/"+string(rune('0'+i)), out)
		}
	}
}

// Strict structured output requires additionalProperties:false on every object
// that declares a property set. Meta enforces it hard -- without it the request
// is rejected with 400 "'additionalProperties' is required to be supplied and
// to be false." Observed live on a cold start.
func TestPiggybackSchema_IsStrictForEveryObject(t *testing.T) {
	for name, build := range map[string]func() *ZAIResponseFormat{
		"openai":     BuildOpenAIPiggybackEnvelopeSchema,
		"openrouter": BuildOpenRouterPiggybackEnvelopeSchema,
	} {
		t.Run(name, func(t *testing.T) {
			rf := build()
			if rf.JSONSchema == nil || rf.JSONSchema.Schema == nil {
				t.Fatalf("%s builder produced no schema", name)
			}
			var missing []string
			findObjectsMissingAdditionalProperties(rf.JSONSchema.Schema, "", &missing)
			if len(missing) > 0 {
				t.Errorf("%d object node(s) lack additionalProperties:false: %v", len(missing), missing)
			}
			// It must still be serializable as a request body.
			if _, err := json.Marshal(rf); err != nil {
				t.Errorf("schema does not marshal: %v", err)
			}
		})
	}
}

// The shared raw schema is cached behind a sync.Once and handed to concurrent
// callers, so the strict builders must copy rather than stamp in place.
func TestStrictObjectSchema_DoesNotMutateSharedRaw(t *testing.T) {
	raw := piggybackEnvelopeRawSchema()
	before, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal raw: %v", err)
	}

	_ = strictObjectSchema(raw)
	_ = BuildOpenAIPiggybackEnvelopeSchema()
	_ = BuildOpenRouterPiggybackEnvelopeSchema()

	after, err := json.Marshal(piggybackEnvelopeRawSchema())
	if err != nil {
		t.Fatalf("marshal raw after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("shared raw schema was mutated:\nbefore: %s\nafter:  %s", before, after)
	}
}

// Gemini takes the raw schema, not the strict one -- its responseJsonSchema
// does not want OpenAI's strict-mode keyword. Pinning this keeps a future
// "make everything strict" edit from silently changing the Gemini request.
func TestGeminiSchema_StaysRaw(t *testing.T) {
	gemini, err := json.Marshal(BuildGeminiPiggybackEnvelopeSchema())
	if err != nil {
		t.Fatalf("marshal gemini schema: %v", err)
	}
	raw, err := json.Marshal(piggybackEnvelopeRawSchema())
	if err != nil {
		t.Fatalf("marshal raw schema: %v", err)
	}
	if string(gemini) != string(raw) {
		t.Errorf("Gemini schema diverged from the raw schema:\ngemini: %s\nraw:    %s", gemini, raw)
	}
}

// A 400 that blames the schema must be recognised whatever the vendor calls the
// field, so the client can retry without response_format instead of failing.
func TestIsSchemaRejection(t *testing.T) {
	rejections := map[string]string{
		"openai response_format": `{"error":{"message":"Invalid response_format","param":"response_format"}}`,
		"json_schema named":      `{"error":{"message":"json_schema unsupported for this model"}}`,
		"meta additionalProps":   `{"error":{"code":null,"message":"'additionalProperties' is required to be supplied and to be false.","param":"schema","type":"invalid_request_error"}}`,
		"meta param schema":      `{"error":{"message":"bad schema","param":"schema"}}`,
	}
	for name, body := range rejections {
		if !isSchemaRejection(body) {
			t.Errorf("%s: expected a schema rejection, got false for %s", name, body)
		}
	}

	unrelated := map[string]string{
		"context length": `{"error":{"message":"maximum context length exceeded","param":"messages"}}`,
		"bad model":      `{"error":{"message":"model not found","param":"model"}}`,
	}
	for name, body := range unrelated {
		if isSchemaRejection(body) {
			t.Errorf("%s: must NOT be treated as a schema rejection (would silently drop structured output): %s", name, body)
		}
	}
}

// A transient 5xx is the vendor failing, not the request being wrong. Observed
// live: one Meta 500 killed a turn that would have succeeded on retry.
func TestIsRetryableServerStatus(t *testing.T) {
	for _, code := range []int{500, 502, 503, 504, 529} {
		if !isRetryableServerStatus(code) {
			t.Errorf("status %d should be retried", code)
		}
	}
	// Client errors are the caller's fault and 501 will never start working.
	for _, code := range []int{200, 400, 401, 403, 404, 429, 501} {
		if isRetryableServerStatus(code) {
			t.Errorf("status %d must NOT be retried as a server error", code)
		}
	}
}

// Meta answered 404 model_not_found for a model that had already served ~200
// calls that day, killing a shard delegation. A spurious 404 costs a whole
// turn; a genuinely wrong model name costs one wasted retry.
func TestIsTransientModelNotFound(t *testing.T) {
	body := `{"error":{"code":"model_not_found","message":"The requested model was not found.","type":"invalid_request_error"}}`
	if !isTransientModelNotFound(404, body) {
		t.Error("404 model_not_found should be retried")
	}
	// A 404 for anything else is a real routing error -- do not mask it.
	if isTransientModelNotFound(404, `{"error":{"message":"unknown route /v1/nope"}}`) {
		t.Error("a non-model 404 must not be retried")
	}
	// The marker alone must not make other statuses retryable here.
	if isTransientModelNotFound(400, body) {
		t.Error("only 404 qualifies")
	}
}
