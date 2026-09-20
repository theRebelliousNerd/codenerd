package session

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

type stubCodeElements struct{}

func (stubCodeElements) FileFacts(string) ([]types.Fact, error) { return nil, nil }

// The CodeDOM fact layer shipped half-wired (e78e7241) and the very next run
// proved it: R1-19 made 24 tool calls and logged not one scope line. The source
// was attached to the session executor, but `nerd fix` delegates to a coder
// shard, and the Spawner builds that shard's executor itself -- forwarding the
// parent's state one call at a time. A provider missing from that list reaches
// the session executor and never the shard that does the work.
//
// The capability had four tests and none of them could see this: they all
// exercised codedomScope directly. A capability test is not a wiring test, and
// this repo's own guidance warns that it "frequently has partially wired
// features and dormant integration points."
func TestSpawner_Spawn_InheritsCodeElementSource(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)
	src := stubCodeElements{}
	spawner.SetCodeElementSource(src)

	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name:       "coder",
		Task:       "fix the thing",
		Type:       SubAgentTypeEphemeral,
		IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent == nil || agent.executor == nil {
		t.Fatal("expected non-nil agent and executor")
	}
	if agent.executor.codeElements == nil {
		t.Fatal("a spawned shard cannot reach the CodeDOM fact layer: the file it works on will never have its elements in the kernel, which is what code_element answering nothing for 1,175 queries a run looked like")
	}
}

// With nothing set, a spawned shard simply has no source and scopeFocusFile is
// a no-op -- the layer stays empty and nothing else changes.
func TestSpawner_Spawn_NoCodeElementSourceIsQuiet(t *testing.T) {
	spawner := NewSpawner(
		&MockKernel{},
		&MockVirtualStore{},
		&MockLLMClient{},
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
		DefaultSpawnerConfig(),
	)
	agent, err := spawner.Spawn(context.Background(), SpawnRequest{
		Name: "coder", Task: "t", Type: SubAgentTypeEphemeral, IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("Spawn failed: %v", err)
	}
	if agent.executor.codeElements != nil {
		t.Fatal("a source appeared from nowhere")
	}
	agent.executor.scopeFocusFile("anything.go") // must not panic
}

// CloneForTask is the executor a `nerd fix` turn actually runs on:
// JITExecutor.executeObserved calls it (task_executor.go). The CodeDOM fact
// layer was wired onto the session executor and the Spawner, and measured dark
// on the very next run -- 17 tool calls, not one scope line -- because neither
// is what does the work. Three construction paths, and a provider has to be on
// all three.
func TestCloneForTask_InheritsCodeElementSource(t *testing.T) {
	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})
	e.SetCodeElementSource(stubCodeElements{})
	if e.CloneForTask().codeElements == nil {
		t.Fatal("a delegated task cannot reach the CodeDOM fact layer; every nerd fix turn runs on this clone")
	}
}
