package prompt

import (
	"time"
)

// PromptManifest provides detailed observability into the prompt compilation process.
// It acts as a "Flight Recorder", explaining why atoms were selected or rejected.
type PromptManifest struct {
	// Compilation Metadata
	Timestamp   time.Time `json:"timestamp"`
	ContextHash string    `json:"context_hash"`
	TokenUsage  int       `json:"token_usage"`
	BudgetLimit int       `json:"budget_limit"`

	// Selection Breakdown
	Selected []AtomManifestEntry `json:"selected"`
	Dropped  []DroppedAtomEntry  `json:"dropped"`
}

// AtomManifestEntry details a single selected atom.
type AtomManifestEntry struct {
	ID          string  `json:"id"`
	Category    string  `json:"category"`
	Source      string  `json:"source"`
	Priority    int     `json:"priority"`
	LogicScore  float64 `json:"logic_score"`
	VectorScore float64 `json:"vector_score"`
	RenderMode  string  `json:"render_mode"` // "standard", "concise", "min"
}

// DroppedAtomEntry details an atom that was considered but rejected.
type DroppedAtomEntry struct {
	ID     string `json:"id"`
	Reason string `json:"reason"` // e.g., "Mangle: prohibited", "Budget: exceeded"
}
