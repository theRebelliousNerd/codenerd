package perception

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// TestGeminiStructuredOutput tests that Gemini 3 Flash returns valid structured output.
// Run with: go test -v -run TestGeminiStructuredOutput ./internal/perception/
func TestGeminiStructuredOutput(t *testing.T) {
	// Load API key from config or environment
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		// Try loading from config
		key, _, err := LoadConfigForTest()
		if err == nil && key != "" {
			apiKey = key
		}
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping live test")
	}

	client := NewGeminiClient(apiKey)
	client.SetModel("gemini-3-flash-preview")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test 1: Basic completion
	t.Run("BasicCompletion", func(t *testing.T) {
		resp, err := client.Complete(ctx, "Say 'Hello from Gemini 3 Flash' and nothing else.")
		if err != nil {
			t.Fatalf("Basic completion failed: %v", err)
		}
		t.Logf("Basic response: %s", resp)
		if resp == "" {
			t.Error("Empty response from Gemini")
		}
	})

	// Test 2: Structured output with Piggyback Protocol
	t.Run("StructuredOutput", func(t *testing.T) {
		systemPrompt := `You are an intent classifier. Respond ONLY with valid JSON matching this schema:
{
  "control_packet": {
    "intent_classification": {
      "category": "string",
      "verb": "string",
      "target": "string",
      "constraint": "string",
      "confidence": number
    }
  },
  "surface_response": "string"
}`

		userPrompt := "Classify this intent: 'Fix the bug in auth.go'"

		resp, err := client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			t.Fatalf("Structured output failed: %v", err)
		}
		t.Logf("Structured response: %s", resp)

		// Verify it's valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(resp), &result); err != nil {
			t.Errorf("Response is not valid JSON: %v\nResponse was: %s", err, resp)
		}

		// Check for expected keys
		if _, ok := result["control_packet"]; !ok {
			t.Error("Missing control_packet in response")
		}
		if _, ok := result["surface_response"]; !ok {
			t.Error("Missing surface_response in response")
		}
	})

	// Test 3: Model identification
	t.Run("ModelIdentification", func(t *testing.T) {
		model := client.GetModel()
		t.Logf("Using model: %s", model)
		if model != "gemini-3-flash-preview" {
			t.Errorf("Expected model gemini-3-flash-preview, got %s", model)
		}
	})

	// Test 4: Structured output with context_feedback
	t.Run("ContextFeedback", func(t *testing.T) {
		systemPrompt := `You are a coding assistant using the Piggyback Protocol. Respond with valid JSON:
{
  "control_packet": {
    "intent_classification": {
      "category": "/mutation",
      "verb": "/fix",
      "target": "auth.go",
      "constraint": "",
      "confidence": 0.9
    },
    "mangle_updates": [],
    "context_feedback": {
      "overall_usefulness": 0.75,
      "helpful_facts": ["file_topology", "test_state"],
      "noise_facts": ["browser_state"],
      "missing_context": "call graph would help"
    }
  },
  "surface_response": "I'll help fix the auth.go file."
}

You were provided these context facts: file_topology, test_state, browser_state, symbol_graph.
Rate their usefulness in context_feedback.`

		userPrompt := "Fix the authentication bug in auth.go"

		resp, err := client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			t.Fatalf("Context feedback test failed: %v", err)
		}
		t.Logf("Context feedback response: %s", resp)

		// Parse and verify structure
		var result struct {
			ControlPacket struct {
				IntentClassification struct {
					Category   string  `json:"category"`
					Verb       string  `json:"verb"`
					Confidence float64 `json:"confidence"`
				} `json:"intent_classification"`
				MangleUpdates   []string `json:"mangle_updates"`
				ContextFeedback *struct {
					OverallUsefulness float64  `json:"overall_usefulness"`
					HelpfulFacts      []string `json:"helpful_facts"`
					NoiseFacts        []string `json:"noise_facts"`
					MissingContext    string   `json:"missing_context"`
				} `json:"context_feedback"`
			} `json:"control_packet"`
			SurfaceResponse string `json:"surface_response"`
		}

		if err := json.Unmarshal([]byte(resp), &result); err != nil {
			t.Errorf("Failed to parse response as JSON: %v\nResponse: %s", err, resp)
			return
		}

		// Verify context_feedback is present
		if result.ControlPacket.ContextFeedback == nil {
			t.Log("Note: context_feedback not present (optional field)")
		} else {
			cf := result.ControlPacket.ContextFeedback
			t.Logf("Context Feedback: usefulness=%.2f, helpful=%d, noise=%d",
				cf.OverallUsefulness, len(cf.HelpfulFacts), len(cf.NoiseFacts))

			// Validate usefulness range
			if cf.OverallUsefulness < 0.0 || cf.OverallUsefulness > 1.0 {
				t.Errorf("overall_usefulness out of range: %.2f", cf.OverallUsefulness)
			}
		}

		// Verify required fields
		if result.SurfaceResponse == "" {
			t.Error("Missing surface_response")
		}
	})
}

// TestContextFeedbackSchema validates the schema includes context_feedback.
func TestContextFeedbackSchema(t *testing.T) {
	schema := piggybackEnvelopeRawSchema()

	// Navigate to control_packet.properties
	controlPacket, ok := schema["properties"].(map[string]interface{})["control_packet"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing control_packet in schema")
	}

	props, ok := controlPacket["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing properties in control_packet")
	}

	// Verify context_feedback exists
	contextFeedback, ok := props["context_feedback"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing context_feedback in control_packet properties")
	}

	// Verify context_feedback properties
	cfProps, ok := contextFeedback["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing properties in context_feedback")
	}

	expectedFields := []string{"overall_usefulness", "helpful_facts", "noise_facts", "missing_context"}
	for _, field := range expectedFields {
		if _, ok := cfProps[field]; !ok {
			t.Errorf("Missing field %s in context_feedback schema", field)
		}
	}

	// Verify context_feedback is NOT in required fields (it's optional)
	required, ok := controlPacket["required"].([]string)
	if !ok {
		t.Fatal("Missing required array in control_packet")
	}

	for _, req := range required {
		if req == "context_feedback" {
			t.Error("context_feedback should NOT be in required fields (it's optional)")
		}
	}

	// Verify only intent_classification and mangle_updates are required
	if len(required) != 2 {
		t.Errorf("Expected 2 required fields, got %d: %v", len(required), required)
	}

	t.Logf("Schema validated: context_feedback present with fields: %v", expectedFields)
}

// TestContextFeedbackParsing tests parsing of context_feedback from JSON response.
func TestContextFeedbackParsing(t *testing.T) {
	// Simulate a response with context_feedback
	response := `{
		"control_packet": {
			"intent_classification": {
				"category": "/mutation",
				"verb": "/fix",
				"target": "main.go",
				"constraint": "",
				"confidence": 0.95
			},
			"mangle_updates": ["task_status(/fix, /in_progress)"],
			"context_feedback": {
				"overall_usefulness": 0.85,
				"helpful_facts": ["file_topology", "test_state", "symbol_graph"],
				"noise_facts": ["browser_state", "dom_node"],
				"missing_context": "dependency graph would have helped"
			}
		},
		"surface_response": "I'll fix the bug in main.go."
	}`

	var result struct {
		ControlPacket struct {
			IntentClassification struct {
				Category   string  `json:"category"`
				Verb       string  `json:"verb"`
				Target     string  `json:"target"`
				Confidence float64 `json:"confidence"`
			} `json:"intent_classification"`
			MangleUpdates   []string `json:"mangle_updates"`
			ContextFeedback *struct {
				OverallUsefulness float64  `json:"overall_usefulness"`
				HelpfulFacts      []string `json:"helpful_facts"`
				NoiseFacts        []string `json:"noise_facts"`
				MissingContext    string   `json:"missing_context"`
			} `json:"context_feedback"`
		} `json:"control_packet"`
		SurfaceResponse string `json:"surface_response"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Validate context_feedback
	if result.ControlPacket.ContextFeedback == nil {
		t.Fatal("context_feedback is nil")
	}

	cf := result.ControlPacket.ContextFeedback

	if cf.OverallUsefulness != 0.85 {
		t.Errorf("Expected usefulness 0.85, got %.2f", cf.OverallUsefulness)
	}

	if len(cf.HelpfulFacts) != 3 {
		t.Errorf("Expected 3 helpful facts, got %d", len(cf.HelpfulFacts))
	}

	if len(cf.NoiseFacts) != 2 {
		t.Errorf("Expected 2 noise facts, got %d", len(cf.NoiseFacts))
	}

	if cf.MissingContext != "dependency graph would have helped" {
		t.Errorf("Unexpected missing_context: %s", cf.MissingContext)
	}

	t.Logf("Parsed context_feedback: usefulness=%.2f, helpful=%v, noise=%v",
		cf.OverallUsefulness, cf.HelpfulFacts, cf.NoiseFacts)
}

// TestContextFeedbackOptional verifies context_feedback can be omitted.
func TestContextFeedbackOptional(t *testing.T) {
	// Response without context_feedback (valid because it's optional)
	response := `{
		"control_packet": {
			"intent_classification": {
				"category": "/query",
				"verb": "/search",
				"target": "files",
				"constraint": "*.go",
				"confidence": 0.9
			},
			"mangle_updates": []
		},
		"surface_response": "Searching for Go files..."
	}`

	var result struct {
		ControlPacket struct {
			ContextFeedback *struct {
				OverallUsefulness float64 `json:"overall_usefulness"`
			} `json:"context_feedback"`
		} `json:"control_packet"`
		SurfaceResponse string `json:"surface_response"`
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.ControlPacket.ContextFeedback != nil {
		t.Error("context_feedback should be nil when not provided")
	}

	if result.SurfaceResponse == "" {
		t.Error("surface_response should be present")
	}

	t.Log("Verified context_feedback is properly optional")
}

// LoadConfigForTest loads the config from .nerd/config.json
func LoadConfigForTest() (string, string, error) {
	paths := []string{
		".nerd/config.json",
		"../../.nerd/config.json",
		"C:/CodeProjects/codeNERD/.nerd/config.json",
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", "", err
	}

	var cfg struct {
		GeminiAPIKey string `json:"gemini_api_key"`
		Model        string `json:"model"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", "", err
	}

	return cfg.GeminiAPIKey, cfg.Model, nil
}
