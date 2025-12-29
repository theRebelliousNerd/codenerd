package context_harness

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// FeedbackTracer traces context feedback learning events.
// This supports testing the third feedback loop: LLM-driven context usefulness learning.
type FeedbackTracer struct {
	writer  io.Writer
	verbose bool
}

// NewFeedbackTracer creates a new feedback tracer.
func NewFeedbackTracer(writer io.Writer, verbose bool) *FeedbackTracer {
	return &FeedbackTracer{
		writer:  writer,
		verbose: verbose,
	}
}

// FeedbackSnapshot captures a context feedback event.
type FeedbackSnapshot struct {
	Timestamp   time.Time
	TurnNumber  int
	IntentVerb  string

	// Feedback from LLM
	OverallUsefulness float64  // 0.0-1.0
	HelpfulFacts      []string // Predicates that helped
	NoiseFacts        []string // Predicates that were noise
	MissingContext    string   // What would have helped

	// Activation context at time of feedback
	ActivePredicates []PredicateFeedbackState

	// Historical learning state
	LearnedPredicates []PredicateFeedbackState
	TotalFeedbackSamples int
}

// PredicateFeedbackState represents the learned state of a predicate.
type PredicateFeedbackState struct {
	Predicate     string
	HelpfulCount  int
	NoiseCount    int
	TotalMentions int
	UsefulnessScore float64 // -1.0 to +1.0
	ScoreComponent  float64 // Contribution to activation score (-20 to +20)
	LastUpdated   time.Time
}

// TraceFeedback logs a context feedback event.
func (t *FeedbackTracer) TraceFeedback(snapshot *FeedbackSnapshot) {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("CONTEXT FEEDBACK - TURN %d\n", snapshot.TurnNumber))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", snapshot.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Intent: %s\n\n", snapshot.IntentVerb))

	// LLM Feedback
	sb.WriteString("LLM FEEDBACK:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Overall Usefulness: %.2f\n", snapshot.OverallUsefulness))

	if len(snapshot.HelpfulFacts) > 0 {
		sb.WriteString(fmt.Sprintf("\n  Helpful Predicates (%d):\n", len(snapshot.HelpfulFacts)))
		for _, p := range snapshot.HelpfulFacts {
			sb.WriteString(fmt.Sprintf("    + %s\n", p))
		}
	}

	if len(snapshot.NoiseFacts) > 0 {
		sb.WriteString(fmt.Sprintf("\n  Noise Predicates (%d):\n", len(snapshot.NoiseFacts)))
		for _, p := range snapshot.NoiseFacts {
			sb.WriteString(fmt.Sprintf("    - %s\n", p))
		}
	}

	if snapshot.MissingContext != "" {
		sb.WriteString(fmt.Sprintf("\n  Missing Context: %s\n", snapshot.MissingContext))
	}
	sb.WriteString("\n")

	// Learning State
	sb.WriteString("LEARNING STATE:\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	sb.WriteString(fmt.Sprintf("  Total Feedback Samples: %d\n\n", snapshot.TotalFeedbackSamples))

	// Active predicates with feedback scores
	if len(snapshot.ActivePredicates) > 0 {
		sb.WriteString("Active Predicates (with feedback scores):\n")

		// Sort by score contribution descending
		sorted := make([]PredicateFeedbackState, len(snapshot.ActivePredicates))
		copy(sorted, snapshot.ActivePredicates)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ScoreComponent > sorted[j].ScoreComponent
		})

		for _, p := range sorted {
			indicator := "○" // Neutral
			if p.ScoreComponent > 5 {
				indicator = "↑" // Boosted
			} else if p.ScoreComponent < -5 {
				indicator = "↓" // Penalized
			}

			sb.WriteString(fmt.Sprintf("  %s %s: score=%.1f (helpful=%d, noise=%d, samples=%d)\n",
				indicator, p.Predicate, p.ScoreComponent,
				p.HelpfulCount, p.NoiseCount, p.TotalMentions))
		}
		sb.WriteString("\n")
	}

	// Learned predicates (historical)
	if t.verbose && len(snapshot.LearnedPredicates) > 0 {
		sb.WriteString("HISTORICAL PREDICATE LEARNING:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		// Sort by absolute impact
		sorted := make([]PredicateFeedbackState, len(snapshot.LearnedPredicates))
		copy(sorted, snapshot.LearnedPredicates)
		sort.Slice(sorted, func(i, j int) bool {
			return abs(sorted[i].UsefulnessScore) > abs(sorted[j].UsefulnessScore)
		})

		// Show top 20
		limit := 20
		if len(sorted) < limit {
			limit = len(sorted)
		}

		for i := 0; i < limit; i++ {
			p := sorted[i]
			classification := "NEUTRAL"
			if p.UsefulnessScore > 0.3 {
				classification = "HELPFUL"
			} else if p.UsefulnessScore < -0.3 {
				classification = "NOISE"
			}

			sb.WriteString(fmt.Sprintf("  [%s] %s: %.2f (helpful=%d, noise=%d)\n",
				classification, p.Predicate, p.UsefulnessScore,
				p.HelpfulCount, p.NoiseCount))
		}

		if len(sorted) > limit {
			sb.WriteString(fmt.Sprintf("  ... and %d more predicates\n", len(sorted)-limit))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	t.writer.Write([]byte(sb.String()))
}

// TraceScoreImpact logs the impact of learned feedback on activation scores.
func (t *FeedbackTracer) TraceScoreImpact(turnNumber int, impacts []PredicateScoreImpact) {
	if len(impacts) == 0 {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FEEDBACK SCORE IMPACT - TURN %d\n", turnNumber))
	sb.WriteString("───────────────────────────────────────────────────────────────\n")

	// Sort by absolute impact
	sorted := make([]PredicateScoreImpact, len(impacts))
	copy(sorted, impacts)
	sort.Slice(sorted, func(i, j int) bool {
		return abs(sorted[i].ScoreDelta) > abs(sorted[j].ScoreDelta)
	})

	for _, impact := range sorted[:min(10, len(sorted))] {
		direction := "→"
		if impact.ScoreDelta > 0 {
			direction = "↑"
		} else if impact.ScoreDelta < 0 {
			direction = "↓"
		}

		sb.WriteString(fmt.Sprintf("  %s %s: %+.1f (%.1f → %.1f)\n",
			direction, impact.Predicate,
			impact.ScoreDelta, impact.BaseScore, impact.FinalScore))
	}
	sb.WriteString("\n")

	t.writer.Write([]byte(sb.String()))
}

// PredicateScoreImpact represents how feedback affected a fact's activation score.
type PredicateScoreImpact struct {
	Predicate   string
	BaseScore   float64 // Score without feedback
	FeedbackMod float64 // Feedback contribution
	FinalScore  float64 // Score with feedback
	ScoreDelta  float64 // FinalScore - BaseScore
}

// abs returns absolute value of float64.
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
