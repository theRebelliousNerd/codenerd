package types

import (
	"path/filepath"
	"testing"
)

func TestAgentKnowledgeDBPath(t *testing.T) {
	got := AgentKnowledgeDBPath(filepath.Join("ws", ".nerd"), "BubbleTeaExpert")
	want := filepath.Join("ws", ".nerd", "shards", "bubbleteaexpert_knowledge.db")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
