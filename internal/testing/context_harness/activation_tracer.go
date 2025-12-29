package context_harness

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
)

// ActivationTracer traces spreading activation decisions.
type ActivationTracer struct {
	writer  io.Writer
	verbose bool
}

// NewActivationTracer creates a new activation tracer.
func NewActivationTracer(writer io.Writer, verbose bool) *ActivationTracer {
	return &ActivationTracer{
		writer:  writer,
		verbose: verbose,
	}
}

// ActivationSnapshot captures spreading activation state.
type ActivationSnapshot struct {
	Timestamp    time.Time
	TurnNumber   int
	Query        string // User's current query/intent
	TokenBudget  int

	// Activation State
	TotalFacts      int
	ActivatedFacts  []FactActivation
	PrunedFacts     []FactActivation

	// Activation Context
	CampaignContext *CampaignContext
	IssueContext    *IssueContext
	SessionContext  *SessionContext

	// Dependency Graph
	DependencyEdges []DependencyEdge

	// Performance
	ActivationLatency time.Duration
	GraphSize         int // Number of nodes in dependency graph
}

// FactActivation represents a fact with its activation score breakdown.
type FactActivation struct {
	Fact core.Fact

	// Final score and selection
	Score    float64
	Selected bool

	// Score Breakdown (8 components that sum to final score)
	RecencyScore     float64
	RelevanceScore   float64
	DependencyScore  float64
	CampaignScore    float64
	IssueScore       float64
	SessionScore     float64
	FeedbackScore    float64 // NEW: Learned predicate usefulness (-20 to +20)

	// Explanation
	Reason string // Human-readable explanation of score
}

// CampaignContext for campaign-aware activation.
type CampaignContext struct {
	CampaignID   string
	CurrentPhase string
	PhaseGoals   []string
	RelevantFiles []string
}

// IssueContext for issue-driven activation.
type IssueContext struct {
	IssueID        string
	Keywords       map[string]float64
	MentionedFiles []string
	ErrorTypes     []string
}

// SessionContext for session-aware activation.
type SessionContext struct {
	SessionID      string
	StartTime      time.Time
	TurnsSoFar     int
	RecentTopics   []string
}

// DependencyEdge represents a dependency relationship.
type DependencyEdge struct {
	From       string // Fact ID
	To         string // Fact ID
	Type       string // "imports", "calls", "references", etc.
	Weight     float64
	Bidirectional bool
}

// TraceActivation logs a spreading activation event.
func (t *ActivationTracer) TraceActivation(snapshot *ActivationSnapshot) {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("SPREADING ACTIVATION - TURN %d\n", snapshot.TurnNumber))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", snapshot.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Activation Latency: %v\n\n", snapshot.ActivationLatency.Round(time.Millisecond)))

	// Query Context
	sb.WriteString("QUERY CONTEXT:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Query:        %s\n", snapshot.Query))
	sb.WriteString(fmt.Sprintf("  Token Budget: %s\n", formatNumber(snapshot.TokenBudget)))
	sb.WriteString(fmt.Sprintf("  Total Facts:  %s\n\n", formatNumber(snapshot.TotalFacts)))

	// Campaign Context
	if snapshot.CampaignContext != nil {
		cc := snapshot.CampaignContext
		sb.WriteString("Campaign Context:\n")
		sb.WriteString(fmt.Sprintf("  Campaign: %s\n", cc.CampaignID))
		sb.WriteString(fmt.Sprintf("  Phase:    %s\n", cc.CurrentPhase))
		if len(cc.PhaseGoals) > 0 {
			sb.WriteString("  Goals:\n")
			for _, goal := range cc.PhaseGoals {
				sb.WriteString(fmt.Sprintf("    - %s\n", goal))
			}
		}
		sb.WriteString("\n")
	}

	// Issue Context
	if snapshot.IssueContext != nil {
		ic := snapshot.IssueContext
		sb.WriteString("Issue Context:\n")
		sb.WriteString(fmt.Sprintf("  Issue ID: %s\n", ic.IssueID))
		if len(ic.Keywords) > 0 {
			sb.WriteString("  Keywords:\n")
			// Sort by weight
			type kw struct {
				word   string
				weight float64
			}
			keywords := make([]kw, 0, len(ic.Keywords))
			for word, weight := range ic.Keywords {
				keywords = append(keywords, kw{word, weight})
			}
			sort.Slice(keywords, func(i, j int) bool {
				return keywords[i].weight > keywords[j].weight
			})
			for i := 0; i < len(keywords) && i < 10; i++ {
				sb.WriteString(fmt.Sprintf("    - %s (%.2f)\n", keywords[i].word, keywords[i].weight))
			}
		}
		sb.WriteString("\n")
	}

	// Session Context
	if snapshot.SessionContext != nil {
		sc := snapshot.SessionContext
		sb.WriteString("Session Context:\n")
		sb.WriteString(fmt.Sprintf("  Session ID:  %s\n", sc.SessionID))
		sb.WriteString(fmt.Sprintf("  Turns So Far: %d\n", sc.TurnsSoFar))
		if len(sc.RecentTopics) > 0 {
			sb.WriteString(fmt.Sprintf("  Recent Topics: %s\n", strings.Join(sc.RecentTopics, ", ")))
		}
		sb.WriteString("\n")
	}

	// Activation Results
	selectedCount := 0
	selectedTokens := 0
	for _, fa := range snapshot.ActivatedFacts {
		if fa.Selected {
			selectedCount++
			selectedTokens += estimateFactTokens(fa.Fact)
		}
	}

	sb.WriteString("ACTIVATION RESULTS:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Selected:  %d facts (~%s tokens)\n",
		selectedCount, formatNumber(selectedTokens)))
	sb.WriteString(fmt.Sprintf("  Pruned:    %d facts\n", len(snapshot.PrunedFacts)))
	sb.WriteString(fmt.Sprintf("  Graph Size: %d nodes, %d edges\n\n",
		snapshot.GraphSize, len(snapshot.DependencyEdges)))

	// Top Activated Facts (by score)
	if len(snapshot.ActivatedFacts) > 0 {
		sb.WriteString("TOP ACTIVATED FACTS:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		// Sort by score descending
		sortedFacts := make([]FactActivation, len(snapshot.ActivatedFacts))
		copy(sortedFacts, snapshot.ActivatedFacts)
		sort.Slice(sortedFacts, func(i, j int) bool {
			return sortedFacts[i].Score > sortedFacts[j].Score
		})

		topN := 15
		if len(sortedFacts) < topN {
			topN = len(sortedFacts)
		}

		for i := 0; i < topN; i++ {
			fa := sortedFacts[i]
			status := "✓"
			if !fa.Selected {
				status = "✗"
			}

			sb.WriteString(fmt.Sprintf("%s [%.3f] %s\n",
				status, fa.Score, truncate(fa.Fact.String(), 80)))

			if t.verbose {
				// Show score breakdown (8 components)
				sb.WriteString(fmt.Sprintf("    Score Breakdown:\n"))
				if fa.RecencyScore > 0 {
					sb.WriteString(fmt.Sprintf("      Recency:     %.3f\n", fa.RecencyScore))
				}
				if fa.RelevanceScore > 0 {
					sb.WriteString(fmt.Sprintf("      Relevance:   %.3f\n", fa.RelevanceScore))
				}
				if fa.DependencyScore > 0 {
					sb.WriteString(fmt.Sprintf("      Dependency:  %.3f\n", fa.DependencyScore))
				}
				if fa.CampaignScore > 0 {
					sb.WriteString(fmt.Sprintf("      Campaign:    %.3f\n", fa.CampaignScore))
				}
				if fa.IssueScore > 0 {
					sb.WriteString(fmt.Sprintf("      Issue:       %.3f\n", fa.IssueScore))
				}
				if fa.SessionScore > 0 {
					sb.WriteString(fmt.Sprintf("      Session:     %.3f\n", fa.SessionScore))
				}
				if fa.FeedbackScore != 0 {
					indicator := "↑"
					if fa.FeedbackScore < 0 {
						indicator = "↓"
					}
					sb.WriteString(fmt.Sprintf("      Feedback:    %s%.3f (learned)\n", indicator, fa.FeedbackScore))
				}
			}

			if fa.Reason != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", fa.Reason))
			}

			sb.WriteString("\n")
		}
	}

	// Pruned Facts (lowest scores)
	if t.verbose && len(snapshot.PrunedFacts) > 0 {
		sb.WriteString("PRUNED FACTS (lowest scores):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		// Sort by score ascending
		sortedPruned := make([]FactActivation, len(snapshot.PrunedFacts))
		copy(sortedPruned, snapshot.PrunedFacts)
		sort.Slice(sortedPruned, func(i, j int) bool {
			return sortedPruned[i].Score < sortedPruned[j].Score
		})

		sampleSize := 10
		if len(sortedPruned) < sampleSize {
			sampleSize = len(sortedPruned)
		}

		for i := 0; i < sampleSize; i++ {
			fa := sortedPruned[i]
			sb.WriteString(fmt.Sprintf("✗ [%.3f] %s\n",
				fa.Score, truncate(fa.Fact.String(), 80)))
			if fa.Reason != "" {
				sb.WriteString(fmt.Sprintf("    %s\n", fa.Reason))
			}
		}
		sb.WriteString("\n")
	}

	// Dependency Graph Visualization (if verbose)
	if t.verbose && len(snapshot.DependencyEdges) > 0 && len(snapshot.DependencyEdges) < 50 {
		sb.WriteString("DEPENDENCY GRAPH:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		// Group edges by type
		byType := make(map[string][]DependencyEdge)
		for _, edge := range snapshot.DependencyEdges {
			byType[edge.Type] = append(byType[edge.Type], edge)
		}

		for edgeType, edges := range byType {
			sb.WriteString(fmt.Sprintf("\n%s (%d):\n", edgeType, len(edges)))
			for _, edge := range edges {
				arrow := "→"
				if edge.Bidirectional {
					arrow = "↔"
				}
				sb.WriteString(fmt.Sprintf("  %s %s %s (weight: %.2f)\n",
					truncate(edge.From, 30),
					arrow,
					truncate(edge.To, 30),
					edge.Weight))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	t.writer.Write([]byte(sb.String()))
}

// estimateFactTokens estimates token count for a fact.
func estimateFactTokens(fact core.Fact) int {
	// Rough estimate: predicate + args
	// Each component is ~5 tokens on average
	return 5 + len(fact.Args)*5
}
