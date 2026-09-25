package config

import (
	"os"
	"strings"
	"testing"
)

// internal/config/agents.md told maintainers to preserve
// adaptive_tool_budget and bound the tool-iteration extensions for months
// after those keys were removed and refused at load. The guide is the first
// thing a maintainer of this package reads, so a removed key named there is a
// removed knob about to come back.
func TestAgentsGuide_TeachesNoRemovedKey(t *testing.T) {
	data, err := os.ReadFile("agents.md")
	if err != nil {
		t.Fatalf("read agents.md: %v", err)
	}
	guide := string(data)
	for _, removed := range []map[string]string{
		removedFeatureKeys, removedCoreLimitKeys, removedLLMTimeoutKeys,
		removedWorkingKeys, removedShardProfileKeys,
	} {
		for key := range removed {
			if strings.Contains(guide, key) {
				t.Errorf("agents.md names the removed key %q", key)
			}
		}
	}
}
