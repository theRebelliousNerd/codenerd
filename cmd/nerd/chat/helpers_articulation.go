package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// STREAMING HELPERS
// =============================================================================

// waitForStream is the surface-text streaming consumer used by the TUI.
// Surface stream closing terminates the streaming UI (streamEndMsg);
// thoughts are watched in parallel via waitForThoughts and a closed
// thoughts channel is silently ignored (it's optional/auxiliary data).
func waitForStream(sub chan string) tea.Cmd {
	return func() tea.Msg {
		chunk, ok := <-sub
		if !ok {
			return streamEndMsg{}
		}
		return streamChunkMsg{chunk: chunk, sub: sub, kind: streamKindSurface}
	}
}

// waitForThoughts is the parallel consumer for the model's thinking
// trace. We deliberately do NOT emit streamEndMsg when the thoughts
// channel closes — the surface stream owns end-of-life. A nil channel
// (provider doesn't expose thoughts) yields a no-op tea.Cmd so the
// caller can wire it unconditionally.
func waitForThoughts(sub chan string) tea.Cmd {
	if sub == nil {
		return nil
	}
	return func() tea.Msg {
		chunk, ok := <-sub
		if !ok {
			return nil // thoughts stream ended; surface stream drives the UI lifecycle
		}
		return streamChunkMsg{chunk: chunk, sub: sub, kind: streamKindThought}
	}
}

func waitForResult(res chan tea.Msg, errChan chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-res:
			return msg
		case err := <-errChan:
			return errorMsg(err)
		}
	}
}

// =============================================================================
// ARTICULATION HELPERS
// =============================================================================

func formatResponse(intent perception.Intent, payload articulation.PiggybackEnvelope) string {
	// Keep logic artifacts internal; return only the conversational surface text.
	// Log intent for debugging if needed
	_ = intent.Verb // Mark as used
	return strings.TrimSpace(payload.Surface)
}

func payloadForArticulation(intent perception.Intent, mangleUpdates []string) articulation.PiggybackEnvelope {
	return articulation.PiggybackEnvelope{
		Surface: "",
		Control: articulation.ControlPacket{
			IntentClassification: articulation.IntentClassification{
				Category:   intent.Category,
				Verb:       intent.Verb,
				Target:     intent.Target,
				Constraint: intent.Constraint,
				Confidence: intent.Confidence,
			},
			MangleUpdates: mangleUpdates,
		},
	}
}

// ArticulationOutput contains the full output from the articulation layer.
// This is used to pass control data back to the compressor.
type ArticulationOutput struct {
	Surface           string
	Envelope          articulation.PiggybackEnvelope
	MemoryOperations  []articulation.MemoryOperation
	MangleUpdates     []string
	SelfCorrection    *articulation.SelfCorrection
	KnowledgeRequests []articulation.KnowledgeRequest // LLM-initiated knowledge gathering
	ContextFeedback   *articulation.ContextFeedback   // LLM feedback on context usefulness
	ParseMethod       string
	Warnings          []string
	GroundingSources  []string // URLs used to ground the response (from Google Search/URL Context)
}

// articulateWithContext performs the articulation phase and returns the surface response.
// This is the original signature for backward compatibility.
func articulateWithContext(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string) (string, error) {
	output, err := articulateWithContextFull(ctx, client, intent, payload, contextFacts, warnings, systemPrompt)
	if err != nil {
		return "", err
	}
	return output.Surface, nil
}

// ConversationContext holds recent conversation history for LLM context injection.
// This enables fluid conversation by providing the LLM with recent turns.
// Implements the Blackboard Pattern for cross-shard and cross-turn context propagation.
type ConversationContext struct {
	RecentTurns     []Message      // Last N conversation turns
	LastShardResult *ShardResult   // Most recent shard output for follow-ups
	TurnNumber      int            // Current turn number
	ShardHistory    []*ShardResult // Sliding window of past shard results (blackboard)
	CompressedCtx   string         // Semantically compressed session context from compressor
}

// articulateWithContextFull performs articulation and returns full structured output.
// This enhanced version properly extracts all control packet data for the compressor.
func articulateWithContextFull(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string) (*ArticulationOutput, error) {
	// Use the new version with nil conversation context for backward compatibility
	return articulateWithConversation(ctx, client, intent, payload, contextFacts, warnings, systemPrompt, nil, nil, nil)
}

// articulateWithConversation performs articulation with full conversation context.
// This is the new entry point that enables fluid conversational follow-ups.
//
// If thoughtsChan is non-nil AND the underlying client implements
// core.LLMStreamingWithThoughts, the model's thinking trace is streamed
// to thoughtsChan in arrival order; otherwise thoughtsChan is left
// untouched (and the function silently falls back to 2-channel streaming).
// Callers own closing thoughtsChan, just like streamChan.
func articulateWithConversation(ctx context.Context, client perception.LLMClient, intent perception.Intent, payload articulation.PiggybackEnvelope, contextFacts []core.Fact, warnings []string, systemPrompt string, convCtx *ConversationContext, streamChan chan<- string, thoughtsChan chan<- string) (*ArticulationOutput, error) {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString("System Instructions:\n")
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n")
	}

	// =========================================================================
	// CONVERSATION HISTORY INJECTION (Critical for fluid chat)
	// =========================================================================
	// Include recent conversation turns so the LLM understands context.
	// This enables follow-up questions like "what are the other suggestions?"
	if convCtx != nil && len(convCtx.RecentTurns) > 0 {
		sb.WriteString("## Recent Conversation History\n")
		sb.WriteString("(Use this context to understand follow-up questions)\n\n")
		for _, turn := range convCtx.RecentTurns {
			if turn.Role == "user" {
				// Cap replayed user content: a single giant paste must not be
				// re-sent verbatim in every subsequent articulation prompt.
				content := turn.Content
				if len(content) > 2000 {
					content = content[:2000] + "\n... (truncated)"
				}
				sb.WriteString(fmt.Sprintf("**User**: %s\n", content))
			} else {
				// Truncate long assistant responses
				content := turn.Content
				if len(content) > 500 {
					content = content[:500] + "\n... (truncated)"
				}
				sb.WriteString(fmt.Sprintf("**Assistant**: %s\n", content))
			}
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// LAST SHARD RESULT INJECTION (Critical for follow-up queries)
	// =========================================================================
	// If there's a recent shard result, include it so follow-ups work.
	// This enables "what are the other warnings?" after a review.
	if convCtx != nil && convCtx.LastShardResult != nil {
		sr := convCtx.LastShardResult
		sb.WriteString("## Last Shard Execution Result\n")
		sb.WriteString(fmt.Sprintf("**Type**: %s\n", sr.ShardType))
		sb.WriteString(fmt.Sprintf("**Task**: %s\n", sr.Task))
		sb.WriteString(fmt.Sprintf("**Turn**: %d\n\n", sr.TurnNumber))

		// Include structured findings if available (for reviewer).
		// Capped: a large review can emit hundreds of findings; replaying all
		// of them in every articulation prompt blows the token budget for
		// diminishing returns.
		if len(sr.Findings) > 0 {
			const maxFindingsInPrompt = 40
			sb.WriteString("### All Findings (use for follow-up queries)\n")
			for i, finding := range sr.Findings {
				if i >= maxFindingsInPrompt {
					sb.WriteString(fmt.Sprintf("... and %d more findings (ask to filter by severity or file)\n", len(sr.Findings)-maxFindingsInPrompt))
					break
				}
				sb.WriteString(fmt.Sprintf("%d. ", i+1))
				for k, v := range finding {
					sb.WriteString(fmt.Sprintf("%s=%v ", k, v))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}

		// Include metrics if available
		if len(sr.Metrics) > 0 {
			sb.WriteString("### Metrics\n")
			for k, v := range sr.Metrics {
				sb.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
			}
			sb.WriteString("\n")
		}
	}

	// =========================================================================
	// SHARD HISTORY INJECTION (Blackboard Pattern)
	// =========================================================================
	// Include summarized history of recent shard executions for cross-shard context.
	// This enables flows like: reviewer→coder, tester→debugger, coder→tester.
	if convCtx != nil && len(convCtx.ShardHistory) > 1 {
		sb.WriteString("## Shard Execution History (Blackboard)\n")
		sb.WriteString("(Previous shard results for cross-shard context)\n\n")
		// Skip the last one since it's already shown above as LastShardResult
		for i := 0; i < len(convCtx.ShardHistory)-1; i++ {
			sr := convCtx.ShardHistory[i]
			sb.WriteString(fmt.Sprintf("- **Turn %d [%s]**: %s", sr.TurnNumber, sr.ShardType, truncateForContext(sr.Task, 50)))
			if len(sr.Findings) > 0 {
				sb.WriteString(fmt.Sprintf(" → %d findings", len(sr.Findings)))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// =========================================================================
	// COMPRESSED SESSION CONTEXT INJECTION (Infinite Context)
	// =========================================================================
	// Inject the semantically compressed context from the compressor.
	// This provides >100:1 compressed session history as Mangle atoms.
	if convCtx != nil && convCtx.CompressedCtx != "" {
		sb.WriteString("## Compressed Session Context\n")
		sb.WriteString("(Semantic compression of prior turns - use for long-range context)\n\n")
		sb.WriteString(convCtx.CompressedCtx)
		sb.WriteString("\n\n")
	}

	if len(contextFacts) > 0 {
		// Capped: spreading activation can derive hundreds of context_to_inject
		// facts on a large workspace; an unbounded dump starves the model's
		// output budget and buries the relevant facts.
		const (
			maxContextFactsInPrompt = 80
			maxContextFactChars     = 12 * 1024
		)
		sb.WriteString("Context Facts:\n")
		written := 0
		chars := 0
		for _, f := range contextFacts {
			if written >= maxContextFactsInPrompt || chars >= maxContextFactChars {
				sb.WriteString(fmt.Sprintf("- ... (%d more context facts omitted)\n", len(contextFacts)-written))
				break
			}
			line := "- " + f.String() + "\n"
			sb.WriteString(line)
			written++
			chars += len(line)
		}
		sb.WriteString("\n")
	}

	if len(warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range warnings {
			sb.WriteString("- " + w + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Intent: %s -> %s\n\n", intent.Verb, intent.Target))
	sb.WriteString("CRITICAL: You MUST output JSON in this EXACT order to prevent lies!\n")
	sb.WriteString("If generation fails mid-stream, control_packet must be written FIRST.\n\n")
	sb.WriteString("Required JSON Schema (THOUGHT-FIRST ordering):\n")
	sb.WriteString("{\n")
	sb.WriteString(`  "control_packet": {` + "\n")
	sb.WriteString(`    "intent_classification": { "category": "mutation|query|instruction", "verb": "...", "target": "...", "confidence": 0.0 },` + "\n")
	sb.WriteString(`    "reasoning_trace": "optional internal notes",` + "\n")
	sb.WriteString(`    "mangle_updates": [ "predicate(arg1, arg2)." ],` + "\n")
	sb.WriteString(`    "memory_operations": [ { "op": "promote_to_long_term|forget|note|store_vector", "key": "preference_name", "value": "value" } ],` + "\n")
	sb.WriteString(`    "self_correction": { "triggered": false, "hypothesis": "" }` + "\n")
	sb.WriteString(`  },` + "\n")
	sb.WriteString(`  "surface_response": "text visible to user ONLY after control_packet is written"` + "\n")
	sb.WriteString("}\n\n")
	sb.WriteString("DO NOT speak to the user until AFTER you have written the complete control_packet!\n\n")
	sb.WriteString("MEMORY OPERATIONS:\n")
	sb.WriteString("- promote_to_long_term: Store user preferences or learned patterns\n")
	sb.WriteString("- forget: Remove outdated facts\n")
	sb.WriteString("- note: Add temporary session notes\n")
	sb.WriteString("- store_vector: Store for semantic search\n\n")
	sb.WriteString("Use only the context facts above. Do not invent filesystem access or knowledge not present in the facts. Output JSON only.")

	// Log the full prompt package for transparency
	logging.Get(logging.CategorySession).Debug("================ FULL PROMPT PACKAGE ================\nSystem:\n%s\n\nUser:\n%s\n==================================================", systemPrompt, sb.String())

	var raw string
	var err error

	if streamChan != nil {
		// Use streaming completion. If the client (or its wrapper) supports
		// 3-channel thoughts streaming AND the caller provided a thoughtsChan,
		// route the model's thinking trace to it for live TUI rendering.
		var chunkChan <-chan string
		var tChan <-chan string
		var errChan <-chan error
		enableThinking := false
		if thoughtsChan != nil {
			if ts, ok := client.(core.LLMStreamingWithThoughts); ok {
				enableThinking = true
				chunkChan, tChan, errChan = ts.CompleteWithStreamingAndThoughts(ctx, systemPrompt, sb.String(), enableThinking)
			}
		}
		if chunkChan == nil {
			chunkChan, errChan = client.CompleteWithStreaming(ctx, systemPrompt, sb.String(), enableThinking)
		}

		parser := articulation.NewStreamParser()
		var fullBuilder strings.Builder

		for {
			select {
			case chunk, ok := <-chunkChan:
				if !ok {
					chunkChan = nil
				} else {
					fullBuilder.WriteString(chunk)
					surfaceChunk := parser.ProcessChunk(chunk)
					if surfaceChunk != "" {
						streamChan <- surfaceChunk
					}
				}
			case thought, ok := <-tChan:
				if !ok {
					tChan = nil
				} else if thoughtsChan != nil && thought != "" {
					// Non-blocking forward — if the TUI hasn't drained yet,
					// drop rather than stall the LLM stream.
					select {
					case thoughtsChan <- thought:
					default:
					}
				}
			case streamErr, ok := <-errChan:
				if !ok {
					errChan = nil
				} else if streamErr != nil {
					err = streamErr
					break
				}
			case <-ctx.Done():
				err = ctx.Err()
				break
			}

			if chunkChan == nil && tChan == nil && errChan == nil {
				break // stream finished successfully
			}
			if err != nil {
				break
			}
		}
		raw = fullBuilder.String()
	} else {
		raw, err = client.CompleteWithSystem(ctx, systemPrompt, sb.String())
	}

	if err != nil {
		return nil, fmt.Errorf("articulation failed: %w", err)
	}

	// Use the enhanced ResponseProcessor from articulation package
	processor := articulation.NewResponseProcessor()
	result, err := processor.Process(raw)
	if err != nil {
		// Truncate the raw response in the error: it surfaces in the TUI error
		// display and logs, and a malformed response can be tens of KB.
		preview := raw
		if len(preview) > 600 {
			preview = preview[:600] + "... (truncated)"
		}
		return nil, fmt.Errorf("piggyback JSON invalid: %w (raw=%s)", err, preview)
	}

	// Build output
	output := &ArticulationOutput{
		Surface:           result.Surface,
		MemoryOperations:  result.Control.MemoryOperations,
		MangleUpdates:     result.Control.MangleUpdates,
		KnowledgeRequests: result.Control.KnowledgeRequests,
		ContextFeedback:   result.Control.ContextFeedback, // NEW: Extract context usefulness feedback
		ParseMethod:       result.ParseMethod,
		Warnings:          result.Warnings,
	}

	// Check for self-correction
	if result.Control.SelfCorrection != nil && result.Control.SelfCorrection.Triggered {
		output.SelfCorrection = result.Control.SelfCorrection
		output.Warnings = append(output.Warnings,
			fmt.Sprintf("Self-correction: %s", result.Control.SelfCorrection.Hypothesis))
	}

	// Build the envelope with merged data
	output.Envelope = articulation.PiggybackEnvelope{
		Surface: result.Surface,
		Control: result.Control,
	}

	// Override with any pre-existing payload data if LLM didn't provide it
	if result.Control.IntentClassification.Category == "" {
		output.Envelope.Control.IntentClassification = payload.Control.IntentClassification
	}
	if len(result.Control.MangleUpdates) == 0 && len(payload.Control.MangleUpdates) > 0 {
		output.Envelope.Control.MangleUpdates = payload.Control.MangleUpdates
		output.MangleUpdates = payload.Control.MangleUpdates
	}

	// Extract grounding sources if client supports grounding (e.g., Gemini with Google Search)
	if gp, ok := client.(types.GroundingProvider); ok {
		output.GroundingSources = gp.GetLastGroundingSources()
	}

	return output, nil
}
