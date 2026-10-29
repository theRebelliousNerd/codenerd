package perception

import "testing"

// The OpenAI-compatible vendor table is the one list of such vendors. The
// factory used to repeat it in two switch statements and isAPIProvider, so a
// vendor added to the table (endpoint and all) still failed with "unknown
// provider" until someone found the other three lists. Registering a vendor in
// the table alone must make it buildable as a main and a classification
// client.
func TestFactoryDispatchesEveryTableVendorToTheCompatClient(t *testing.T) {
	const vendor Provider = "compat_dispatch_probe"
	openAICompatVendorDefaults[vendor] = vendorDefault{baseURL: "https://compat.example.invalid/v1"}
	t.Cleanup(func() { delete(openAICompatVendorDefaults, vendor) })

	if !isAPIProvider(vendor) {
		t.Fatalf("isAPIProvider(%s) = false for a vendor in the compat table", vendor)
	}

	main, err := NewClientFromConfig(&ProviderConfig{Provider: vendor, APIKey: "k", Model: "probe-model"})
	if err != nil {
		t.Fatalf("NewClientFromConfig(%s): %v", vendor, err)
	}
	if _, ok := unwrapForCompatTest(main).(*OpenAICompatClient); !ok {
		t.Fatalf("main client for %s is %T, want *OpenAICompatClient", vendor, unwrapForCompatTest(main))
	}

	classification, err := newRawClassificationClientFromConfig(&ProviderConfig{Provider: vendor, APIKey: "k", Model: "probe-model"})
	if err != nil {
		t.Fatalf("classification client for %s: %v", vendor, err)
	}
	if _, ok := classification.(*OpenAICompatClient); !ok {
		t.Fatalf("classification client for %s is %T, want *OpenAICompatClient", vendor, classification)
	}

	// A failed compat construction is a nil interface, not a typed nil.
	if client, err := newCompatClient(&ProviderConfig{Provider: vendor, Model: "probe-model"}); err == nil || client != nil {
		t.Fatalf("keyless compat client = (%v, %v), want (nil, error)", client, err)
	}
}

// unwrapForCompatTest peels the wrappers NewClientFromConfig may add.
func unwrapForCompatTest(c LLMClient) LLMClient {
	for i := 0; i < 8; i++ {
		u, ok := c.(interface{ Unwrap() LLMClient })
		if !ok {
			return c
		}
		c = u.Unwrap()
	}
	return c
}
