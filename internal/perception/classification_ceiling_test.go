package perception

import "testing"

// A classification call runs on EVERY interactive turn and produces a label.
// Its completion ceiling was applied to one branch of seven.
//
// The rule was already written down, on the OpenAI-compatible branch: "A
// classification reply is a short label, so a small ceiling is right — and a
// ceiling is a cap, not a spend, so keeping it tight costs nothing either
// way." Anthropic, Gemini, OpenAI, Z.AI, xAI and OpenRouter each built their
// classification client from a default config and kept the general-purpose
// ceiling: 4096 for most, 8192 for Anthropic, and 65536 for Gemini — a 64K
// output ceiling on a per-turn labelling call.
//
// This is the gate for the seventh branch. A provider added to
// newRawClassificationClientFromConfig without a ceiling fails here rather than
// quietly costing tokens on every turn of every session that uses it.
func TestEveryClassificationClientCarriesTheLabelCeiling(t *testing.T) {
	providers := []Provider{
		ProviderAnthropic,
		ProviderGemini,
		ProviderOpenAI,
		ProviderZAI,
		ProviderXAI,
		ProviderOpenRouter,
		ProviderDashScope,
		ProviderMeta,
		ProviderMoonshot,
	}

	for _, provider := range providers {
		t.Run(string(provider), func(t *testing.T) {
			// A ClassificationModel is required by the branches that have no
			// assumable fast tier; supplying one everywhere keeps the table
			// uniform and exercises the branch rather than its early return.
			client, err := newRawClassificationClientFromConfig(&ProviderConfig{
				Provider:            provider,
				APIKey:              "test-key",
				ClassificationModel: "test-model",
			})
			if err != nil {
				t.Fatalf("newRawClassificationClientFromConfig: %v", err)
			}
			if client == nil {
				t.Fatalf("no classification client for %s, so the ceiling cannot be checked", provider)
			}

			want := classificationCeiling(provider)
			got, ok := clientCompletionCeiling(client)
			if !ok {
				t.Fatalf("%T carries no completion ceiling this test can read; add it to "+
					"clientCompletionCeiling rather than deleting the case", client)
			}
			if got != want {
				t.Errorf("%s classification ceiling is %d, want %d — this client is built "+
					"from a default config and kept the general-purpose ceiling",
					provider, got, want)
			}
		})
	}
}

// The floor is the other direction and is not optional: below a reasoning
// vendor's minimum, a model burns the whole budget thinking and returns an
// empty body with finish_reason "stop".
func TestTheLabelCeilingNeverDropsBelowAVendorFloor(t *testing.T) {
	if got := classificationCeiling(ProviderMeta); got < minCompletionTokensFor(ProviderMeta) {
		t.Errorf("Meta classification ceiling %d is under its %d floor; a reasoning model "+
			"there returns an empty body rather than a label",
			got, minCompletionTokensFor(ProviderMeta))
	}
	if got := classificationCeiling(ProviderAnthropic); got != classificationMaxOutputTokens {
		t.Errorf("a vendor with no floor got %d, want the plain label budget %d",
			got, classificationMaxOutputTokens)
	}
}

// clientCompletionCeiling reads the ceiling a concrete client will send.
//
// A type switch rather than an interface, because the ceiling is deliberately
// not part of the LLMClient contract: it is a request detail each adapter owns,
// and widening the interface to let one test read it would put it in front of
// every implementation and every test double.
func clientCompletionCeiling(c LLMClient) (int, bool) {
	switch x := c.(type) {
	case *AnthropicClient:
		return x.maxOutputTokens, true
	case *GeminiClient:
		return x.maxOutputTokens, true
	case *OpenAIClient:
		return x.maxOutputTokens, true
	case *ZAIClient:
		return x.maxOutputTokens, true
	case *XAIClient:
		return x.maxOutputTokens, true
	case *OpenRouterClient:
		return x.maxOutputTokens, true
	case *OpenAICompatClient:
		return x.maxOutputTokens, true
	}
	return 0, false
}
