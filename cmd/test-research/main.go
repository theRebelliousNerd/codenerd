// Quick test of the research toolkit and new integrations
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/shards/researcher"
)

func main() {
	fmt.Println("=== Testing Research Toolkit Live ===")

	// Setup environment
	var context7Key string
	
	// 1. Try Env Var
	if os.Getenv("CONTEXT7_API_KEY") != "" {
		context7Key = os.Getenv("CONTEXT7_API_KEY")
	} else {
		// 2. Try Config File
		cfg, err := perception.LoadConfigJSON(".nerd/config.json")
		if err == nil && cfg.Context7APIKey != "" {
			context7Key = cfg.Context7APIKey
			fmt.Println("[Setup] Loaded Context7 key from .nerd/config.json")
		}
	}

	if context7Key == "" {
		fmt.Println("NOTE: CONTEXT7_API_KEY not set. Context7 tests will be skipped or fail gracefully.")
	}

	// Create researcher with toolkit
	researcherShard := researcher.NewResearcherShard()
	// Inject a real kernel
	kernel, err := core.NewRealKernel()
	if err != nil {
		fmt.Printf("FATAL: Failed to create kernel: %v\n", err)
		os.Exit(1)
	}
	researcherShard.SetParentKernel(kernel)
	
	if context7Key != "" {
		researcherShard.SetContext7APIKey(context7Key)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Test 1: Explicit URL Scraping (New Feature)
	// We use a GitHub URL which should trigger the enhanced fetchGitHubDocs path
	fmt.Println("\n1. Testing Explicit URL Scraping (GitHub)வுகளை")
	task := "Research this repo: https://github.com/charmbracelet/bubbletea"
	
	resultStr, err := researcherShard.Execute(ctx, task)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Success! Result Summary:\n%s\n", truncate(resultStr, 200))
		
		// Verify facts were loaded into kernel
		facts, _ := kernel.Query("knowledge_atom")
		fmt.Printf("   Kernel Facts: %d knowledge atoms loaded\n", len(facts))
	}

	// Test 2: Context7 Integration (via ResearchTopic)
	fmt.Println("\n2. Testing Context7 Integration...")
	// We manually invoke the tool to see if it triggers
	toolkit := researcherShard.GetToolkit()
	if toolkit.Context7() != nil {
		// We try a search which uses Context7 if configured
		atoms, err := toolkit.Context7().ResearchTopic(ctx, "next.js", []string{"react", "framework"})
		if err != nil {
			fmt.Printf("   Context7 Result: %v (Expected if no API key)\n", err)
		} else {
			fmt.Printf("   Context7 Success: fetched %d atoms\n", len(atoms))
		}
	}

	fmt.Println("\n=== Live Test Complete ===")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	lines := 0
	for i, char := range s {
		if char == '\n' {
			lines++
		}
		if i >= maxLen || lines >= 5 {
			return s[:i] + "\n... (truncated)"
		}
	}
	return s
}