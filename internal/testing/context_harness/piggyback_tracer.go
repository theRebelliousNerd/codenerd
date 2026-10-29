package context_harness

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// PiggybackTracer traces Piggyback protocol parsing for assistant responses.
type PiggybackTracer struct {
	writer  io.Writer
	verbose bool
}

// NewPiggybackTracer creates a new piggyback protocol tracer.
func NewPiggybackTracer(writer io.Writer, verbose bool) *PiggybackTracer {
	return &PiggybackTracer{
		writer:  writer,
		verbose: verbose,
	}
}

// PiggybackEvent captures a piggyback protocol parsing event.
type PiggybackEvent struct {
	Timestamp       time.Time
	TurnNumber      int
	Speaker         string
	SurfaceText     string
	ControlPacket   *ControlPacket
	ResponseTokens  int
	ResponseLatency time.Duration

	// Mangle state changes derived from control packet
	AddedFacts   []string
	RemovedFacts []string
	ToolCalls    []string
}

// TracePiggyback logs a piggyback protocol event.
func (t *PiggybackTracer) TracePiggyback(event *PiggybackEvent) {
	var sb strings.Builder

	// Header
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("PIGGYBACK PROTOCOL - TURN %d (%s)\n", event.TurnNumber, event.Speaker))
	sb.WriteString("═══════════════════════════════════════════════════════════════\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", event.Timestamp.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Response Tokens: %d | Latency: %v\n\n", event.ResponseTokens, event.ResponseLatency.Round(time.Millisecond)))

	// Surface Text (what user sees)
	sb.WriteString("SURFACE (visible to user):\n")
	sb.WriteString("───────────────────────────────────────────────────────────────\n")
	surfacePreview := event.SurfaceText
	if len(surfacePreview) > 300 {
		surfacePreview = surfacePreview[:300] + "..."
	}
	sb.WriteString(fmt.Sprintf("  %s\n\n", strings.ReplaceAll(surfacePreview, "\n", "\n  ")))

	// Control Packet (hidden from user)
	if event.ControlPacket != nil {
		sb.WriteString("CONTROL PACKET (hidden from user):\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		cp := event.ControlPacket

		// Intent Classification
		sb.WriteString("Intent Classification:\n")
		sb.WriteString(fmt.Sprintf("  Category:   %s\n", cp.IntentClassification.Category))
		sb.WriteString(fmt.Sprintf("  Verb:       %s\n", cp.IntentClassification.Verb))
		if cp.IntentClassification.Target != "" {
			sb.WriteString(fmt.Sprintf("  Target:     %s\n", cp.IntentClassification.Target))
		}
		if cp.IntentClassification.Constraint != "" {
			sb.WriteString(fmt.Sprintf("  Constraint: %s\n", cp.IntentClassification.Constraint))
		}
		sb.WriteString(fmt.Sprintf("  Confidence: %.2f\n\n", cp.IntentClassification.Confidence))

		// Mangle Updates
		if len(cp.MangleUpdates) > 0 {
			sb.WriteString("Mangle Updates:\n")
			for _, update := range cp.MangleUpdates {
				sb.WriteString(fmt.Sprintf("  %s\n", update))
			}
			sb.WriteString("\n")
		}

		// Next Phase
		if cp.NextPhase != "" {
			sb.WriteString(fmt.Sprintf("Next Phase: %s\n\n", cp.NextPhase))
		}

		// Tool Calls
		if len(cp.ToolCalls) > 0 {
			sb.WriteString("Tool Calls:\n")
			for _, tool := range cp.ToolCalls {
				sb.WriteString(fmt.Sprintf("  - %s\n", tool))
			}
			sb.WriteString("\n")
		}

		// Metadata
		if t.verbose && len(cp.Metadata) > 0 {
			sb.WriteString("Metadata:\n")
			for key, value := range cp.Metadata {
				sb.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
			}
			sb.WriteString("\n")
		}

		// Context Feedback (LLM-driven context learning)
		if cp.ContextFeedback != nil {
			sb.WriteString("Context Feedback:\n")
			sb.WriteString(fmt.Sprintf("  Overall Usefulness: %.2f\n", cp.ContextFeedback.OverallUsefulness))

			if len(cp.ContextFeedback.HelpfulFacts) > 0 {
				sb.WriteString(fmt.Sprintf("  Helpful Predicates (%d):\n", len(cp.ContextFeedback.HelpfulFacts)))
				for _, p := range cp.ContextFeedback.HelpfulFacts {
					sb.WriteString(fmt.Sprintf("    + %s\n", p))
				}
			}

			if len(cp.ContextFeedback.NoiseFacts) > 0 {
				sb.WriteString(fmt.Sprintf("  Noise Predicates (%d):\n", len(cp.ContextFeedback.NoiseFacts)))
				for _, p := range cp.ContextFeedback.NoiseFacts {
					sb.WriteString(fmt.Sprintf("    - %s\n", p))
				}
			}

			if cp.ContextFeedback.MissingContext != "" {
				sb.WriteString(fmt.Sprintf("  Missing Context: %s\n", cp.ContextFeedback.MissingContext))
			}
			sb.WriteString("\n")
		}
	}

	// State Changes
	if len(event.AddedFacts) > 0 || len(event.RemovedFacts) > 0 {
		sb.WriteString("KERNEL STATE CHANGES:\n")
		sb.WriteString("───────────────────────────────────────────────────────────────\n")

		if len(event.AddedFacts) > 0 {
			sb.WriteString(fmt.Sprintf("Added Facts (%d):\n", len(event.AddedFacts)))
			for _, fact := range event.AddedFacts {
				sb.WriteString(fmt.Sprintf("  + %s\n", fact))
			}
			sb.WriteString("\n")
		}

		if len(event.RemovedFacts) > 0 {
			sb.WriteString(fmt.Sprintf("Removed Facts (%d):\n", len(event.RemovedFacts)))
			for _, fact := range event.RemovedFacts {
				sb.WriteString(fmt.Sprintf("  - %s\n", fact))
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("═══════════════════════════════════════════════════════════════\n\n")

	t.writer.Write([]byte(sb.String()))
}
