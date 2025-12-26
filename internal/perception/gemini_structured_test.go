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
