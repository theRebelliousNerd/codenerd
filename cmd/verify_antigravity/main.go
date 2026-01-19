package main

import (
	"codenerd/internal/config"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"log"
)

func main() {
	fmt.Println("Starting Antigravity Verification...")

	// 1. Setup Config
	// tailored to force Antigravity provider
	cfg := &config.AntigravityProviderConfig{
		EnableThinking: true,
		ThinkingLevel:  "high",
	}

	client, err := perception.NewAntigravityClient(cfg, "gemini-3.0-pro-exp") // Use a known Antigravity model
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	fmt.Println("Client created successfully.")
	fmt.Println("Testing connectivity and auth (this may open a browser)...")

	// 2. Test Completion
	ctx := context.Background()
	prompt := "Explain how you are authenticating right now."

	fmt.Printf("Sending prompt: %q\n", prompt)

	resp, err := client.Complete(ctx, prompt)
	if err != nil {
		log.Fatalf("Completion failed: %v", err)
	}

	fmt.Println("\n--- Response ---")
	fmt.Println(resp)
	fmt.Println("----------------")

	fmt.Println("\nVerification Context:")
	fmt.Printf("Thinking Enabled: %v\n", client.IsThinkingEnabled())
}
