package main

import (
	"codenerd/internal/perception"
	"fmt"
)

func main() {
	fmt.Println("==================================================")
	fmt.Println("   MANGLE TAXONOMY VERIFICATION PROTOCOL")
	fmt.Println("==================================================")

	scenarios := []struct {
		Input    string
		Expected string
		Why      string
	}{
		{
			Input:    "review this code",
			Expected: "/review",
			Why:      "Direct keyword match (Regex)",
		},
		{
			Input:    "check for vulnerabilities",
			Expected: "/security",
			Why:      "Keyword 'vulnerabilities' maps to /security",
		},
		{
			Input:    "fix the security bug in auth.go",
			Expected: "/security", 
			Why:      "Hybrid: 'fix' (Coder) vs 'security' (Reviewer). Mangle inference should boost /security context.",
		},
		{
			Input:    "verify the test coverage",
			Expected: "/test",
			Why:      "Mangle Inference: 'verify' + 'coverage' boosts /test.",
		},
		{
			Input:    "debug this panic in the stacktrace",
			Expected: "/debug",
			Why:      "Mangle Inference: 'stacktrace' boosts /debug over generic /fix.",
		},
	}

	for _, s := range scenarios {
		fmt.Printf("\n>>> Input: %q\n", s.Input)
		fmt.Printf("    Goal:  %s (%s)\n", s.Expected, s.Why)

		verb, cat, conf, shard := perception.DebugTaxonomy(s.Input)

		status := "❌ FAIL"
		if verb == s.Expected {
			status = "✅ PASS"
		}

		// Fallback check: logic might prioritize /fix for "fix security bug" depending on exact weights.
		// Let's see what happens. If it fails, we tune inference.mg.
		
		fmt.Printf("    Result: %s [%s] (Conf: %.2f) -> Shard: %s\n", verb, cat, conf, shard)
		fmt.Printf("    Status: %s\n", status)
	}
	
	fmt.Println("\n==================================================")
	fmt.Println("   VERIFICATION COMPLETE")
	fmt.Println("==================================================")
}
