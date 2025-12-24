package shards

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestNormalizeMangleAtom(t *testing.T) {
	cases := map[string]string{
		" /foo//bar ": "/foo/bar",
		"/baz":        "/baz",
		"":            "",
		"  ":          "",
	}
	for input, want := range cases {
		if got := normalizeMangleAtom(input); got != want {
			t.Fatalf("normalizeMangleAtom(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeShardTypeName(t *testing.T) {
	if got := normalizeShardTypeName(" /coder "); got != "coder" {
		t.Fatalf("expected normalized name to be coder, got %q", got)
	}
}

func TestTrimToTokenBudget(t *testing.T) {
	sm := NewShardManager()
	tools := []types.ToolInfo{
		{Name: "aaaa", Description: "bbbb"},
		{Name: strings.Repeat("c", 40)},
	}
	trimmed := sm.trimToTokenBudget(tools, 40)
	if len(trimmed) != 1 {
		t.Fatalf("expected 1 tool in budget, got %d", len(trimmed))
	}
	if trimmed[0].Name != "aaaa" {
		t.Fatalf("expected first tool to remain in budget")
	}
}

func TestCategorizeShardType(t *testing.T) {
	sm := NewShardManager()
	if got := sm.categorizeShardType("perception_firewall", types.ShardTypeEphemeral); got != "system" {
		t.Fatalf("expected system classification, got %q", got)
	}
	if got := sm.categorizeShardType("coder", types.ShardTypeEphemeral); got != "ephemeral" {
		t.Fatalf("expected ephemeral classification, got %q", got)
	}
	if got := sm.categorizeShardType("custom", types.ShardTypePersistent); got != "specialist" {
		t.Fatalf("expected specialist classification, got %q", got)
	}
}

func TestShardManagerGetBackpressureStatus(t *testing.T) {
	sm := NewShardManager()
	if sm.GetBackpressureStatus() != nil {
		t.Fatalf("expected nil backpressure status when no queue attached")
	}

	queue := NewSpawnQueue(nil, nil, SpawnQueueConfig{MaxQueueSize: 2, MaxQueuePerPriority: 2})
	sm.SetSpawnQueue(queue)
	status := sm.GetBackpressureStatus()
	if status == nil {
		t.Fatalf("expected backpressure status with queue attached")
	}
	if status.QueueDepth != 0 || !status.Accepting {
		t.Fatalf("expected empty queue to be accepting")
	}
}
