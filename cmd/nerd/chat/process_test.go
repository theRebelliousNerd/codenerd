package chat

import "testing"

func TestIsShardRelevantToTopic(t *testing.T) {
	tests := []struct {
		shardName string
		topic     string
		expected  bool
	}{
		// Generic shards always relevant
		{"coder", "anything", true},
		{"tester", "random topic", true},
		{"reviewer", "browser automation", true},
		{"researcher", "go programming", true},
		{"SecurityAuditor", "sql injection", true},

		// Go expert - relevant for Go topics
		{"GoExpert", "add a gin endpoint", true},
		{"GoExpert", "bubbletea tui", true},
		{"GoExpert", "golang error handling", true},
		{"GoExpert", "react component", false}, // Not Go related
		{"GoExpert", "python django", false},   // Not Go related

		// Rod expert - relevant for browser automation
		{"RodExpert", "browser automation", true},
		{"RodExpert", "scrape website", true},
		{"RodExpert", "headless chrome", true},
		{"RodExpert", "database query", false}, // Not browser related
		{"RodExpert", "api endpoint", false},   // Not browser related

		// Mangle expert - relevant for logic programming
		{"MangleExpert", "mangle rules", true},
		{"MangleExpert", "datalog query", true},
		{"MangleExpert", "predicate logic", true},
		{"MangleExpert", "frontend react", false}, // Not logic related

		// BubbleTea expert - relevant for TUI
		{"BubbleTeaExpert", "terminal ui", true},
		{"BubbleTeaExpert", "cli interface", true},
		{"BubbleTeaExpert", "interactive tui", true},
		{"BubbleTeaExpert", "web server", false}, // Not TUI related

		// Cobra expert - relevant for CLI
		{"CobraExpert", "cli command", true},
		{"CobraExpert", "flag parsing", true},
		{"CobraExpert", "subcommand", true},
		{"CobraExpert", "web api", false}, // Not CLI related
	}

	for _, tt := range tests {
		t.Run(tt.shardName+"_"+tt.topic, func(t *testing.T) {
			result := isShardRelevantToTopic(tt.shardName, tt.topic)
			if result != tt.expected {
				t.Errorf("isShardRelevantToTopic(%q, %q) = %v, want %v",
					tt.shardName, tt.topic, result, tt.expected)
			}
		})
	}
}
