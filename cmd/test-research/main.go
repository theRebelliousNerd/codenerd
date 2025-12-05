// Quick test of the research toolkit
package main

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/shards/researcher"
)

func main() {
	fmt.Println("=== Testing Research Toolkit ===\n")

	// Create researcher with toolkit
	researcherShard := researcher.NewResearcherShard()
	toolkit := researcherShard.GetToolkit()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test 1: GitHub fetch for Rod
	fmt.Println("1. Testing GitHub fetch for go-rod/rod...")
	atoms, err := toolkit.GitHub().FetchRepository(ctx, "go-rod", "rod", []string{"browser", "automation"})
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Success: %d atoms fetched\n", len(atoms))
		for i, atom := range atoms {
			if i >= 3 {
				fmt.Printf("   ... and %d more\n", len(atoms)-3)
				break
			}
			fmt.Printf("   - [%s] %s (%.0f%%)\n", atom.Concept, truncate(atom.Title, 40), atom.Confidence*100)
		}
	}

	// Test 2: Web search
	fmt.Println("\n2. Testing web search for 'golang concurrency patterns'...")
	results, err := toolkit.Search().Search(ctx, "golang concurrency patterns", 5)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Success: %d results found\n", len(results))
		for i, r := range results {
			if i >= 3 {
				break
			}
			fmt.Printf("   - %s\n", truncate(r.Title, 60))
		}
	}

	// Test 3: Cache verification
	fmt.Println("\n3. Testing cache (fetching rod again)...")
	start := time.Now()
	atoms2, _ := toolkit.GitHub().FetchRepository(ctx, "go-rod", "rod", []string{"browser"})
	fmt.Printf("   Second fetch: %d atoms in %v (should be cached)\n", len(atoms2), time.Since(start))

	// Test 4: Parallel topic research
	fmt.Println("\n4. Testing parallel topic research...")
	topics := []string{"rod browser automation", "golang testing"}
	result, err := researcherShard.ResearchTopicsParallel(ctx, topics)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else {
		fmt.Printf("   Success: %d atoms from %d topics in %.2fs\n",
			len(result.Atoms), len(topics), result.Duration.Seconds())
	}

	fmt.Println("\n=== Research Toolkit Test Complete ===")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
