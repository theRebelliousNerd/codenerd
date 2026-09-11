package shards

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The shard assembler and articulation's build the same context for different
// callers, and a section present in only one is a section a shard silently does
// not get. Both render it; this is the half that keeps them in step.
func TestShardPromptCarriesDependenciesOfFilesInFocus(t *testing.T) {
	agent := NewBaseShardAgent("coder", types.ShardConfig{
		SessionContext: &types.SessionContext{
			DependencyContext: []string{
				"internal/prompt/compiler.go imports codenerd/internal/logging",
			},
		},
	})

	prompt := agent.BuildSessionContextPrompt()
	if !strings.Contains(prompt, "DEPENDENCIES OF FILES IN FOCUS") {
		t.Fatalf("no dependency section in the shard prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "imports codenerd/internal/logging") {
		t.Errorf("the dependency section lost its content:\n%s", prompt)
	}
}
