package campaign

import (
	"context"
	"testing"
)

// fixtureDAG is the hand-written table recurse used before the DAG was
// derived from the workspace: a fixed, known graph for planning tests.
func fixtureDAG() []SubsystemNode {
	return []SubsystemNode{
		{ID: "mangle", Title: "Mangle kernel", Paths: []string{"internal/mangle"}},
		{ID: "kernel", Title: "Core kernel and policy", Paths: []string{"internal/core"}, DependsOn: []string{"mangle"}},
		{ID: "store", Title: "Store and persistence", Paths: []string{"internal/store", "internal/persist"}, DependsOn: []string{"kernel"}},
		{ID: "context", Title: "Working context", Paths: []string{"internal/context"}, DependsOn: []string{"kernel", "store"}},
		{ID: "perception", Title: "Perception", Paths: []string{"internal/perception"}, DependsOn: []string{"kernel"}},
		{ID: "prompt", Title: "Prompt compiler and articulation", Paths: []string{"internal/prompt", "internal/jit", "internal/articulation"}, DependsOn: []string{"kernel", "store"}},
		{ID: "tools", Title: "Tools and dispatch", Paths: []string{"internal/tools", "internal/tactile", "internal/mcp"}, DependsOn: []string{"kernel", "store"}},
		{ID: "retrieval", Title: "Retrieval and embeddings", Paths: []string{"internal/retrieval", "internal/embedding"}, DependsOn: []string{"store"}},
		{ID: "world", Title: "World model and scanner", Paths: []string{"internal/world"}, DependsOn: []string{"store"}},
		{ID: "session", Title: "Session executor", Paths: []string{"internal/session"}, DependsOn: []string{"kernel", "prompt", "tools", "perception", "context"}},
		{ID: "shards", Title: "Shard lifecycle", Paths: []string{"internal/shards"}, DependsOn: []string{"session"}},
		{ID: "broker", Title: "Broker streaming", Paths: []string{"internal/broker"}, DependsOn: []string{"session"}},
		{ID: "campaign", Title: "Campaign orchestrator", Paths: []string{"internal/campaign"}, DependsOn: []string{"session", "shards"}},
		{ID: "cli", Title: "CLI and system boot", Paths: []string{"cmd", "internal/system"}, DependsOn: []string{"session", "campaign"}},
		{ID: "wiring", Title: "Cross-subsystem wiring", DependsOn: []string{"cli", "broker", "world", "retrieval"}, CrossCutting: true},
		{ID: "review", Title: "Architectural review", DependsOn: []string{"wiring"}, CrossCutting: true},
		{ID: "bench", Title: "Benchmarks and test creation", DependsOn: []string{"review"}, CrossCutting: true},
	}
}

// useFixtureDAG makes recurse planning read fixtureDAG instead of scanning the
// test's workspace, for tests that assert on known node IDs.
func useFixtureDAG(t *testing.T) {
	t.Helper()
	prev := deriveRecurseDAG
	deriveRecurseDAG = func(context.Context, string) ([]SubsystemNode, error) { return fixtureDAG(), nil }
	t.Cleanup(func() { deriveRecurseDAG = prev })
}
