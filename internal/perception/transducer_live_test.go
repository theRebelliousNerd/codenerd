package perception

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPiggybackProtocol_Live(t *testing.T) {
	// API Key provided by user
	apiKey := os.Getenv("ZAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping live test: ZAI_API_KEY not set")
	}

	client := NewZAIClient(apiKey)
	// Use a model that supports JSON/structured output well if possible,
	// but the default in client.go is "claude-sonnet-4-20250514" which is good.
	// client.SetModel("gpt-4o") // Optional: switch if needed

	transducer := NewDualPayloadTransducer(client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test Case 1: Simple Mutation
	input := "Refactor the auth module to use JWTs"
	fmt.Printf("Testing Input: %s\n", input)

	output, err := transducer.Parse(ctx, input, []string{"auth.go", "jwt.go"})
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify Intent
	fmt.Printf("Surface Response: %s\n", output.Intent.Response)
	fmt.Printf("Intent Category: %s\n", output.Intent.Category)
	fmt.Printf("Intent Verb: %s\n", output.Intent.Verb)

	if output.Intent.Category != "/mutation" {
		t.Errorf("Expected category /mutation, got %s", output.Intent.Category)
	}
	if output.Intent.Verb != "/refactor" {
		t.Errorf("Expected verb /refactor, got %s", output.Intent.Verb)
	}
	if output.Intent.Target == "" {
		t.Error("Expected non-empty target")
	}

	// Verify Mangle Atoms
	fmt.Println("Mangle Atoms:")
	for _, atom := range output.MangleAtoms {
		fmt.Printf("- %v\n", atom)
	}

	if len(output.MangleAtoms) == 0 {
		t.Error("Expected Mangle atoms to be generated")
	}

	// Test Case 2: Impossible Request (Safety/Ambiguity)
	input2 := "Delete all files in the system"
	fmt.Printf("\nTesting Input: %s\n", input2)

	output2, err := transducer.Parse(ctx, input2, nil)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	fmt.Printf("Surface Response: %s\n", output2.Intent.Response)
	fmt.Printf("Intent Category: %s\n", output2.Intent.Category)

	// We expect the system to catch this as a mutation, possibly with a warning in response
	// The spec says: "If the user asks for something impossible... Surface Self says 'I can't do that,' while your Inner Self emits ambiguity_flag(/impossible_request)."
	// Note: Our current ParseIntent implementation maps IntentClassification to Intent.
	// We didn't strictly implement the "ambiguity_flag" extraction in the Go struct yet (it's in the JSON schema but not fully mapped to a specific field other than generic MangleUpdates or Ambiguity slice).
	// Let's see what the LLM does.
}
