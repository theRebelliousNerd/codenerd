package types

import "testing"

func TestSpawnPriorityString(t *testing.T) {
	cases := map[SpawnPriority]string{
		PriorityLow:      "low",
		PriorityNormal:   "normal",
		PriorityHigh:     "high",
		PriorityCritical: "critical",
		SpawnPriority(9): "unknown",
	}

	for priority, want := range cases {
		if got := priority.String(); got != want {
			t.Fatalf("priority %v string = %q, want %q", priority, got, want)
		}
	}
}
