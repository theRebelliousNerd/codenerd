package main

import (
	"os"
	"testing"
)

// providerKeyEnv are the ambient provider credentials a developer's shell may
// carry. Booting a real Cortex with a scripted client must never see one.
var providerKeyEnv = []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "ZAI_API_KEY", "OPENROUTER_API_KEY"}

// TestMain clears the ambient provider keys once for the whole package, so a
// test that boots Cortex offline can run in parallel (t.Setenv forbids it).
// Tests that need a key set it themselves with t.Setenv, which restores the
// cleared value afterwards.
func TestMain(m *testing.M) {
	for _, key := range providerKeyEnv {
		if err := os.Setenv(key, ""); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
