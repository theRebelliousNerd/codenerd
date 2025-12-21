package context_harness

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
)

// PromptInspector intercepts and logs prompts sent to the LLM.
type PromptInspector struct {
	writer      io.Writer
	turnNumber  int
	verbose     bool

	// Tracking
	totalPromptsInspected int
	totalTokensSent       int
	totalTokensPruned     int
}

// NewPromptInspector creates a new prompt inspector.
func NewPromptInspector(writer io.Writer, verbose bool) *PromptInspector {
	return &PromptInspector{
		writer:  writer,
		verbose: verbose,
	}
}

// PromptSnapshot captures the state of a prompt before it's sent to the LLM.
type PromptSnapshot struct {
	TurnNumber      int
	Timestamp       time.Time
	Model           string
	TokenCount      int
	TokenBudget     int
	BudgetUtilized  float64 // Percentage

	// JIT Compilation
	TotalAtomsAvailable   int
	SelectedAtoms         []PromptAtom

	// Spreading Activation
	TotalFactsAvailable   int
	SelectedFacts         []ActivatedFact
	PrunedFacts           []ActivatedFact

	// Prompt Components
	SystemPrompt   string
	UserMessage    string
	FullPrompt     string
}

// PromptAtom represents a JIT prompt atom that was selected.
type PromptAtom struct {
	ID       string
	Category string
	Source   string // File path
	Tokens   int
	Reason   string // Why it was selected
}

// ActivatedFact represents a fact with its activation score.
type ActivatedFact struct {
	Fact   core.Fact
	Score  float64
	Reason string // Why it scored this way
	Tokens int
}

// InspectPrompt logs a complete snapshot of a prompt before sending to LLM.
func (i *PromptInspector) InspectPrompt(snapshot *PromptSnapshot) {
	i.turnNumber = snapshot.TurnNumber
	i.totalPromptsInspected++
	i.totalTokensSent += snapshot.TokenCount

	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("TURN %d - PROMPT TO LLM\n", snapshot.TurnNumber))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Model: %s\n", snapshot.Model))
	sb.WriteString(fmt.Sprintf("Tokens: %s / %s (budget: %.1f%% utilized)\n",
		formatNumber(snapshot.TokenCount),
		formatNumber(snapshot.TokenBudget),
		snapshot.BudgetUtilized*100))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", snapshot.Timestamp.Format("2006-01-02 15:04:05")))

	// JIT Prompt Atoms
	sb.WriteString("SYSTEM PROMPT (JIT Compiled):\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("[Selected Atoms: %d / %s available]\n\n",
		len(snapshot.SelectedAtoms),
		formatNumber(snapshot.TotalAtomsAvailable)))

	if i.verbose {
		// Group atoms by category
		atomsByCategory := make(map[string][]PromptAtom)
		for _, atom := range snapshot.SelectedAtoms {
			atomsByCategory[atom.Category] = append(atomsByCategory[atom.Category], atom)
		}

		for category, atoms := range atomsByCategory {
			sb.WriteString(fmt.Sprintf("## %s (%d)\n", category, len(atoms)))
			for _, atom := range atoms {
				sb.WriteString(fmt.Sprintf("  - %s (%d tokens)\n", atom.ID, atom.Tokens))
				if atom.Reason != "" {
					sb.WriteString(fmt.Sprintf("    Reason: %s\n", atom.Reason))
				}
			}
			sb.WriteString("\n")
		}
	} else {
		// Just show counts by category
		atomsByCategory := make(map[string]int)
		for _, atom := range snapshot.SelectedAtoms {
			atomsByCategory[atom.Category]++
		}
		for category, count := range atomsByCategory {
			sb.WriteString(fmt.Sprintf("  %s: %d atoms\n", category, count))
		}
		sb.WriteString("\n")
	}

	// Spreading Activation Results
	sb.WriteString("COMPRESSED CONTEXT (Spreading Activation):\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("[Facts: %d selected from %s available]\n\n",
		len(snapshot.SelectedFacts),
		formatNumber(snapshot.TotalFactsAvailable)))

	// Show top 10 activation scores
	if len(snapshot.SelectedFacts) > 0 {
		sb.WriteString("Top 10 Activation Scores:\n")

		// Sort by score descending
		sortedFacts := make([]ActivatedFact, len(snapshot.SelectedFacts))
		copy(sortedFacts, snapshot.SelectedFacts)
		sort.Slice(sortedFacts, func(i, j int) bool {
			return sortedFacts[i].Score > sortedFacts[j].Score
		})

		topN := 10
		if len(sortedFacts) < topN {
			topN = len(sortedFacts)
		}

		for idx := 0; idx < topN; idx++ {
			fact := sortedFacts[idx]
			sb.WriteString(fmt.Sprintf("%d. %s - Score: %.2f\n",
				idx+1, fact.Fact.String(), fact.Score))
			if fact.Reason != "" {
				sb.WriteString(fmt.Sprintf("   Reason: %s\n", fact.Reason))
			}
			sb.WriteString("\n")
		}
	}

	// Pruned Facts (if verbose)
	if i.verbose && len(snapshot.PrunedFacts) > 0 {
		sb.WriteString("PRUNED FACTS (not included):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")
		sb.WriteString(fmt.Sprintf("[Pruned: %s facts]\n\n", formatNumber(len(snapshot.PrunedFacts))))

		// Show sample of lowest-scoring pruned facts
		sortedPruned := make([]ActivatedFact, len(snapshot.PrunedFacts))
		copy(sortedPruned, snapshot.PrunedFacts)
		sort.Slice(sortedPruned, func(i, j int) bool {
			return sortedPruned[i].Score < sortedPruned[j].Score
		})

		sampleSize := 5
		if len(sortedPruned) < sampleSize {
			sampleSize = len(sortedPruned)
		}

		sb.WriteString("Sample Pruned (lowest scores):\n")
		for idx := 0; idx < sampleSize; idx++ {
			fact := sortedPruned[idx]
			sb.WriteString(fmt.Sprintf("- %s - Score: %.2f\n",
				truncate(fact.Fact.String(), 80), fact.Score))
			if fact.Reason != "" {
				sb.WriteString(fmt.Sprintf("  Reason: %s\n", fact.Reason))
			}
		}
		sb.WriteString("\n")

		prunedTokens := 0
		for _, f := range snapshot.PrunedFacts {
			prunedTokens += f.Tokens
		}
		i.totalTokensPruned += prunedTokens

		sb.WriteString(fmt.Sprintf("Total Pruned Tokens: %s (saved %.1f%% of available context)\n\n",
			formatNumber(prunedTokens),
			float64(prunedTokens)/float64(prunedTokens+snapshot.TokenCount)*100))
	}

	// User Message
	sb.WriteString("USER MESSAGE:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(snapshot.UserMessage)
	sb.WriteString("\n\n")

	// Full Prompt (if verbose)
	if i.verbose {
		sb.WriteString("FULL PROMPT (sent to API):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")
		sb.WriteString(truncate(snapshot.FullPrompt, 2000))
		if len(snapshot.FullPrompt) > 2000 {
			sb.WriteString("\n... (truncated, full prompt is ")
			sb.WriteString(formatNumber(len(snapshot.FullPrompt)))
			sb.WriteString(" chars)")
		}
		sb.WriteString("\n\n")
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	i.writer.Write([]byte(sb.String()))
}

// ResponseSnapshot captures an LLM response with Piggyback protocol parsing.
type ResponseSnapshot struct {
	TurnNumber      int
	Timestamp       time.Time
	ResponseTokens  int
	ResponseLatency time.Duration

	// Piggyback Protocol
	SurfaceText    string
	ControlPacket  *ControlPacket

	// Kernel State Changes
	StateBefore    []core.Fact
	StateAfter     []core.Fact
	AddedFacts     []core.Fact
	RemovedFacts   []core.Fact
}

// ControlPacket represents the hidden control channel in Piggyback protocol.
type ControlPacket struct {
	IntentClassification IntentClassification
	MangleUpdates        []string
	NextPhase            string
	ToolCalls            []string
	Metadata             map[string]interface{}
}

// IntentClassification from the LLM's understanding.
type IntentClassification struct {
	Category   string
	Verb       string
	Target     string
	Constraint string
	Confidence float64
}

// InspectResponse logs a complete snapshot of an LLM response.
func (i *PromptInspector) InspectResponse(snapshot *ResponseSnapshot) {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("TURN %d - LLM RESPONSE (Piggyback Protocol)\n", snapshot.TurnNumber))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Tokens: %s | Latency: %v\n",
		formatNumber(snapshot.ResponseTokens),
		snapshot.ResponseLatency.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", snapshot.Timestamp.Format("2006-01-02 15:04:05")))

	// Surface Text (visible to user)
	sb.WriteString("SURFACE (visible to user):\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(truncate(snapshot.SurfaceText, 500))
	if len(snapshot.SurfaceText) > 500 {
		sb.WriteString("\n... (truncated)")
	}
	sb.WriteString("\n\n")

	// Control Packet (hidden from user)
	if snapshot.ControlPacket != nil {
		sb.WriteString("CONTROL PACKET (hidden from user):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		cp := snapshot.ControlPacket

		sb.WriteString("Intent Classification:\n")
		sb.WriteString(fmt.Sprintf("  Category:   %s\n", cp.IntentClassification.Category))
		sb.WriteString(fmt.Sprintf("  Verb:       %s\n", cp.IntentClassification.Verb))
		sb.WriteString(fmt.Sprintf("  Target:     %s\n", cp.IntentClassification.Target))
		sb.WriteString(fmt.Sprintf("  Confidence: %.2f\n\n", cp.IntentClassification.Confidence))

		if len(cp.MangleUpdates) > 0 {
			sb.WriteString("Mangle Updates:\n")
			for _, update := range cp.MangleUpdates {
				sb.WriteString(fmt.Sprintf("  - %s\n", update))
			}
			sb.WriteString("\n")
		}

		if cp.NextPhase != "" {
			sb.WriteString(fmt.Sprintf("Next Phase: %s\n\n", cp.NextPhase))
		}

		if len(cp.ToolCalls) > 0 {
			sb.WriteString("Tool Calls:\n")
			for _, tool := range cp.ToolCalls {
				sb.WriteString(fmt.Sprintf("  - %s\n", tool))
			}
			sb.WriteString("\n")
		}
	}

	// Kernel State Changes
	if len(snapshot.AddedFacts) > 0 || len(snapshot.RemovedFacts) > 0 {
		sb.WriteString("KERNEL STATE CHANGES:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		if i.verbose && len(snapshot.StateBefore) > 0 {
			sb.WriteString("Before:\n")
			for _, fact := range snapshot.StateBefore {
				sb.WriteString(fmt.Sprintf("  %s\n", fact.String()))
			}
			sb.WriteString("\n")
		}

		if len(snapshot.AddedFacts) > 0 {
			sb.WriteString(fmt.Sprintf("Added Facts (%d):\n", len(snapshot.AddedFacts)))
			for _, fact := range snapshot.AddedFacts {
				sb.WriteString(fmt.Sprintf("  + %s\n", fact.String()))
			}
			sb.WriteString("\n")
		}

		if len(snapshot.RemovedFacts) > 0 {
			sb.WriteString(fmt.Sprintf("Removed Facts (%d):\n", len(snapshot.RemovedFacts)))
			for _, fact := range snapshot.RemovedFacts {
				sb.WriteString(fmt.Sprintf("  - %s\n", fact.String()))
			}
			sb.WriteString("\n")
		}

		if i.verbose && len(snapshot.StateAfter) > 0 {
			sb.WriteString("After:\n")
			for _, fact := range snapshot.StateAfter {
				sb.WriteString(fmt.Sprintf("  %s\n", fact.String()))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	i.writer.Write([]byte(sb.String()))
}

// Summary prints a summary of the inspection session.
func (i *PromptInspector) Summary() {
	var sb strings.Builder

	sb.WriteString("\n═══════════════════════════════════════════════════════════════\n")
	sb.WriteString("PROMPT INSPECTION SUMMARY\n")
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Total Prompts Inspected: %d\n", i.totalPromptsInspected))
	sb.WriteString(fmt.Sprintf("Total Tokens Sent:       %s\n", formatNumber(i.totalTokensSent)))
	sb.WriteString(fmt.Sprintf("Total Tokens Pruned:     %s\n", formatNumber(i.totalTokensPruned)))

	if i.totalTokensSent+i.totalTokensPruned > 0 {
		compressionRatio := float64(i.totalTokensSent+i.totalTokensPruned) / float64(i.totalTokensSent)
		sb.WriteString(fmt.Sprintf("Compression Ratio:       %.2fx\n", compressionRatio))
		sb.WriteString(fmt.Sprintf("Tokens Saved:            %.1f%%\n",
			float64(i.totalTokensPruned)/float64(i.totalTokensSent+i.totalTokensPruned)*100))
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n")

	i.writer.Write([]byte(sb.String()))
}

// Helper functions

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
