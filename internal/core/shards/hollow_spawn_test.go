package shards

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// recordingDelegator captures what ShardManager hands to the JIT clean loop.
type recordingDelegator struct {
	mu      sync.Mutex
	intents []string
	tasks   []string
	result  string
	err     error
	delay   time.Duration
}

func (d *recordingDelegator) Execute(_ context.Context, intent, task string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.intents = append(d.intents, intent)
	d.tasks = append(d.tasks, task)
	if d.delay > 0 {
		time.Sleep(d.delay)
	}
	return d.result, d.err
}

func (d *recordingDelegator) calls() ([]string, []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.intents...), append([]string(nil), d.tasks...)
}

// TestBaseShardAgentExecuteIsNotFakeSuccess is the guard for this project's
// top-priority failure mode: reporting success without doing work.
//
// Regression guarded: BaseShardAgent.Execute returned
// ("BaseShardAgent execution", nil). ShardManager installs a BaseShardAgent for
// any shard type with no registered factory, and the JIT migration deleted the
// coder/tester/reviewer/researcher factories — so that placeholder became the
// answer for every domain persona and every user-defined agent. Campaign
// consultations recorded the string as specialist advice and parsed a
// confidence from it, the retry verifier accepted it as a completed retry, and
// `nerd spawn <anything> <task>` printed it and exited 0.
//
// If someone restores a non-error return here, every one of those silently
// starts lying again.
func TestBaseShardAgentExecuteIsNotFakeSuccess(t *testing.T) {
	agent := NewBaseShardAgent("probe-1", types.ShardConfig{Name: "probe", Type: types.ShardTypeEphemeral})

	result, err := agent.Execute(context.Background(), "do real work")
	if err == nil {
		t.Fatal("BaseShardAgent.Execute returned nil error: a shard with no implementation must fail loudly, never report success")
	}
	if result != "" {
		t.Errorf("BaseShardAgent.Execute returned result %q; a failed shard must return no output", result)
	}
	if !strings.Contains(err.Error(), "probe") {
		t.Errorf("error should name the shard so the wiring gap is diagnosable, got: %v", err)
	}
}

// TestSpawnWithoutFactoryOrDelegatorFailsLoudly guards the ShardManager side of
// the same defect.
//
// Regression guarded: SpawnAsyncWithContext used to fall back to
// `NewBaseShardAgent` whenever no factory matched, via two dead lookups of
// `sm.factories["researcher"]` (a factory that has not been registered anywhere
// since the JIT migration). A shard type nobody implements must be an error at
// spawn time, not a shard that completes with a placeholder.
func TestSpawnWithoutFactoryOrDelegatorFailsLoudly(t *testing.T) {
	sm := NewShardManager()

	_, err := sm.SpawnAsyncWithContext(context.Background(), "totally_unknown_shard", "task", nil)
	if err == nil {
		t.Fatal("spawning an unimplemented shard type succeeded; it must fail rather than produce a no-op agent")
	}
	if !strings.Contains(err.Error(), "totally_unknown_shard") {
		t.Errorf("error should name the shard type, got: %v", err)
	}
}

// TestSpawnWithoutFactoryDelegatesToCleanLoop pins the replacement behavior.
//
// Regression guarded: domain personas (coder/tester/reviewer/researcher) and
// user-defined agents have no in-process ShardManager factory — they live in
// internal/session. ShardManager must hand those to the task delegator so they
// do real work. Losing the SetTaskDelegator call at a boot site (the mistake
// this codebase keeps making with setters) turns those spawns back into hard
// errors, which the test above proves is at least loud.
func TestSpawnWithoutFactoryDelegatesToCleanLoop(t *testing.T) {
	delegator := &recordingDelegator{result: "real work performed"}
	sm := NewShardManager()
	sm.SetTaskDelegator(delegator)

	if !sm.HasTaskDelegator() {
		t.Fatal("SetTaskDelegator did not take effect")
	}

	result, err := sm.SpawnWithContext(context.Background(), "coder", "fix the bug", nil)
	if err != nil {
		t.Fatalf("delegated spawn failed: %v", err)
	}
	if result != "real work performed" {
		t.Errorf("result = %q, want the delegator's output; the shard's result must come from the clean loop", result)
	}

	intents, tasks := delegator.calls()
	if len(intents) != 1 {
		t.Fatalf("expected exactly 1 delegation, got %d", len(intents))
	}
	if intents[0] != "coder" {
		t.Errorf("delegated intent = %q, want %q (the executor normalizes shard names into verbs)", intents[0], "coder")
	}
	if tasks[0] != "fix the bug" {
		t.Errorf("delegated task = %q, want %q", tasks[0], "fix the bug")
	}
}

// TestDelegatedSpawnPropagatesFailure ensures a failing delegation surfaces as
// a shard error rather than an empty success.
func TestDelegatedSpawnPropagatesFailure(t *testing.T) {
	delegator := &recordingDelegator{err: errors.New("model refused")}
	sm := NewShardManager()
	sm.SetTaskDelegator(delegator)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := sm.SpawnWithContext(ctx, "reviewer", "review it", nil)
	if err == nil {
		t.Fatal("delegated spawn reported success despite the delegator failing")
	}
	if !strings.Contains(err.Error(), "model refused") {
		t.Errorf("underlying error should propagate, got: %v", err)
	}
}

// TestImageShardIsNeverDelegated guards the dual-LLM isolation rule (FM15).
//
// Regression guarded: image generation must stay on the Gemini Nano Banana 2
// client. The task delegator is wired to the worker LLM (often a local Ollama),
// so a missing image_generator factory must be an error, never a delegation.
func TestImageShardIsNeverDelegated(t *testing.T) {
	delegator := &recordingDelegator{result: "should never be reached"}
	sm := NewShardManager()
	sm.SetTaskDelegator(delegator)
	// Deliberately no image factory and no image LLM client registered.

	_, err := sm.SpawnAsyncWithContext(context.Background(), "image_generator", "draw a square", nil)
	if err == nil {
		t.Fatal("image shard spawn succeeded without the Nano Banana 2 client")
	}
	if intents, _ := delegator.calls(); len(intents) != 0 {
		t.Errorf("image generation was delegated to the worker LLM: %v", intents)
	}
}
