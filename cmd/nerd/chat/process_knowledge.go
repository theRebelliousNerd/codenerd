// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains LLM-First Knowledge Discovery and request handling.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// KNOWLEDGE REQUEST HANDLING (LLM-First Knowledge Discovery)
// =============================================================================

// maxKnowledgeRequestsPerTurn caps how many specialist consultations a single
// turn may spawn. Each consultation is a full subagent execution (JIT compile
// + tool loop); honoring an unbounded list multiplies turn latency without
// bound on slow providers.
const maxKnowledgeRequestsPerTurn = 2

// errKnowledgeNotRun marks a requested consultation past the per-turn cap:
// it was asked for and not run, which synthesis says rather than leaving it
// indistinguishable from never asked.
var errKnowledgeNotRun = fmt.Errorf("not run: a turn runs at most %d specialist consultations", maxKnowledgeRequestsPerTurn)

// knowledgeParent is the grounding of the answering request that asked for
// knowledge: its recent conversation, the workspace evidence the host read
// for the turn (rendered with its coverage) and the compressed history. The
// final answer is written from it and the consultations, not from the bare
// input.
type knowledgeParent struct {
	RecentTurns   []Message
	Evidence      string
	CompressedCtx string
}

// knowledgeParentOf freezes the answering request's context.
func knowledgeParentOf(convCtx *ConversationContext) knowledgeParent {
	if convCtx == nil {
		return knowledgeParent{}
	}
	return knowledgeParent{
		RecentTurns:   append([]Message(nil), convCtx.RecentTurns...),
		Evidence:      renderEvidence(convCtx.Evidence),
		CompressedCtx: convCtx.CompressedCtx,
	}
}

// handleKnowledgeRequests spawns specialists in parallel to gather knowledge
// requested by the LLM. Returns a knowledgeGatheredMsg when all specialists
// have responded; the Update handler then synthesizes a final answer with ONE
// follow-up LLM call (synthesizeWithKnowledge) — it does NOT re-enter the
// processInput pipeline.
func (m *Model) handleKnowledgeRequests(
	ctx context.Context,
	requests []articulation.KnowledgeRequest,
	originalInput string,
	interimResponse string,
	parent knowledgeParent,
) knowledgeGatheredMsg {
	// NOTE: do not set m.awaitingKnowledge here — m is a copy (processInput has a
	// value receiver), so the write would be discarded. The flag is carried on
	// the returned message and applied on the Update thread.

	// Resolve each request's specialist, then fold the requests that ask the
	// same specialist the same thing: a repeated ask is one consultation, and
	// must not spend a slot of the per-turn cap that a different question
	// needed.
	requests = m.dedupeKnowledgeRequests(requests)

	var notRun []KnowledgeResult
	if len(requests) > maxKnowledgeRequestsPerTurn {
		logging.Get(logging.CategoryContext).Info(
			"Knowledge requests capped: %d requested, honoring first %d",
			len(requests), maxKnowledgeRequestsPerTurn,
		)
		for _, r := range requests[maxKnowledgeRequestsPerTurn:] {
			notRun = append(notRun, KnowledgeResult{Specialist: r.Specialist, Query: r.Query, Purpose: r.Purpose, Timestamp: time.Now(), Error: errKnowledgeNotRun})
		}
		requests = requests[:maxKnowledgeRequestsPerTurn]
	}

	// Show status to user
	m.ReportStatus(fmt.Sprintf("Gathering knowledge from %d specialist(s)...", len(requests)))

	// Create a channel to collect results
	resultsChan := make(chan KnowledgeResult, len(requests))
	var wg sync.WaitGroup

	for _, req := range requests {
		r := req
		wg.Go(func() {
			shardType := r.Specialist // resolved by dedupeKnowledgeRequests

			// Build task prompt for the specialist
			task := fmt.Sprintf(`Knowledge Query: %s

Purpose: %s

Please provide a comprehensive answer to this query. Focus on practical, actionable information.
If you need to search documentation or the web, do so to provide accurate information.`, r.Query, r.Purpose)

			// Build session context for the specialist
			sessionCtx := m.buildSessionContext(ctx)

			// Log the consultation
			logging.Get(logging.CategoryContext).Info(
				"Consulting specialist '%s' for: %s",
				shardType, oneLine(r.Query),
			)

			// Spawn the specialist with high priority (knowledge is blocking)
			ret, err := m.spawnTaskWithContext(ctx, shardTypeToTaskRequest(shardType, task), sessionCtx, types.PriorityHigh)
			if err != nil {
				err = withPartialState(err, ret)
			}

			resultsChan <- KnowledgeResult{
				Specialist: shardType,
				Query:      r.Query,
				Purpose:    r.Purpose,
				Response:   ret.Output,
				Timestamp:  time.Now(),
				Error:      err,
			}
		})
	}

	// Wait for all specialists to complete
	if m.goroutineWg != nil {
		m.goroutineWg.Add(1)
	}
	go func() {
		if m.goroutineWg != nil {
			defer m.goroutineWg.Done()
		}
		wg.Wait()
		close(resultsChan)
	}()

	// Collect all results
	var results []KnowledgeResult
	for kr := range resultsChan {
		results = append(results, kr)
		if kr.Error != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Knowledge request to '%s' failed: %v",
				kr.Specialist, kr.Error,
			)
		} else {
			logging.Get(logging.CategoryContext).Info(
				"Knowledge received from '%s': %d chars",
				kr.Specialist, len(kr.Response),
			)
		}
	}

	return knowledgeGatheredMsg{
		Results:           append(results, notRun...),
		OriginalInput:     originalInput,
		InterimResponse:   interimResponse,
		Parent:            parent,
		AwaitingKnowledge: true,
	}
}

// dedupeKnowledgeRequests resolves "_any_specialist" to a concrete specialist
// and drops a request that repeats an earlier one exactly -- the same resolved
// specialist, query and purpose, byte for byte -- keeping first-occurrence
// order. Identity is literal: case and whitespace inside a technical question
// carry meaning (the regexp classes \s and \S, the string literals "a b" and
// "a  b"), and folding them dropped a distinct question from execution and
// from the results synthesis reports on. A near-duplicate in other words
// costs a slot of the cap, and past the cap it is reported not run, never
// silently merged.
func (m *Model) dedupeKnowledgeRequests(requests []articulation.KnowledgeRequest) []articulation.KnowledgeRequest {
	seen := make(map[string]bool, len(requests))
	out := make([]articulation.KnowledgeRequest, 0, len(requests))
	for _, r := range requests {
		if r.Specialist == "_any_specialist" {
			r.Specialist = m.matchSpecialistForQuery(r.Query)
		}
		key := knowledgeRequestKey(r.Specialist, r.Query, r.Purpose)
		if seen[key] {
			logging.Get(logging.CategoryContext).Info("Knowledge request to '%s' repeats an earlier one; consulted once", r.Specialist)
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	return out
}

// knowledgeRequestKey is an unambiguous encoding of a request's identity:
// each field length-prefixed, so no field's bytes can imitate a separator.
func knowledgeRequestKey(fields ...string) string {
	var b strings.Builder
	for _, f := range fields {
		fmt.Fprintf(&b, "%d:%s", len(f), f)
	}
	return b.String()
}

// synthesizeWithKnowledge produces the final answer after specialist
// consultations with ONE LLM call. This replaces the old behavior of
// re-running the entire processInput pipeline (perception again, the full
// DECIDE waterfall again, articulation again) with the gathered knowledge
// appended to the input — which doubled or tripled turn cost and re-rolled
// the routing dice mid-turn.
//
// The request carries the answering request's grounding (parent: recent
// conversation, workspace evidence with its coverage, compressed history)
// and every consultation's outcome: a failed, empty or not-run consultation
// is listed with its query and purpose, so a partial answer is not written
// as if every question had been answered.
func (m *Model) synthesizeWithKnowledge(request uint64, originalInput string, parent knowledgeParent, results []KnowledgeResult) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				logging.API("PANIC in synthesizeWithKnowledge (recovered): %v", r)
				msg = knowledgeSynthesizedMsg{Err: fmt.Errorf("internal error (recovered panic): %v", r)}
			}
			// The command runs after applyAnswer has restored m.answering.
			// Wrap every exit with the request handed to this command, never
			// with whichever request is newest when its result is delivered.
			msg = answerTo(request, msg)
		}()

		baseCtx := m.shutdownCtx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(baseCtx, config.GetLLMTimeouts().ArticulationTimeout)
		defer cancel()

		sb := knowledgeSynthesisPrompt(originalInput, parent, results)
		gathered := 0
		for _, kr := range results {
			if kr.Error == nil && strings.TrimSpace(kr.Response) != "" {
				gathered++
			}
		}

		// Same seam, same position as the main chat turn: the persona leads,
		// the role framing for this particular call follows it. See
		// cmd/nerd/chat/persona.go for why this is a constant and not an atom.
		systemPrompt := withArchitectPersona("You are codeNERD answering a user's question after consulting internal specialists.")
		recordArchitectPersonaDelivery("knowledge synthesis", systemPrompt)

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, sb)
		if err != nil {
			return knowledgeSynthesizedMsg{Err: fmt.Errorf("knowledge synthesis failed: %w", err)}
		}
		logging.Get(logging.CategoryContext).Info(
			"Knowledge synthesis complete: %d specialist responses, %d chars answer",
			gathered, len(response),
		)
		return knowledgeSynthesizedMsg{Response: response}
	}
}

// knowledgeSynthesisPrompt is the synthesis request body: the parent's
// grounding, the question, each consultation that returned knowledge, and
// each that did not, with why.
func knowledgeSynthesisPrompt(originalInput string, parent knowledgeParent, results []KnowledgeResult) string {
	var sb strings.Builder
	if len(parent.RecentTurns) > 0 {
		sb.WriteString("## Recent Conversation History\n")
		for _, turn := range parent.RecentTurns {
			if turn.Role == "user" {
				fmt.Fprintf(&sb, "**User**: %s\n", turn.Content)
			} else {
				fmt.Fprintf(&sb, "**Assistant**: %s\n", turn.Content)
			}
		}
		sb.WriteString("\n")
	}
	if parent.CompressedCtx != "" {
		sb.WriteString("## Compressed Session Context\n")
		sb.WriteString(parent.CompressedCtx)
		sb.WriteString("\n\n")
	}
	if parent.Evidence != "" {
		sb.WriteString(parent.Evidence)
		sb.WriteString("\n")
	}
	sb.WriteString("The user asked:\n\n")
	sb.WriteString(originalInput)
	sb.WriteString("\n\n---\nSpecialist consultations returned the following knowledge:\n\n")
	gathered := 0
	var missing []string
	for _, kr := range results {
		label := fmt.Sprintf("%s (query: %s", kr.Specialist, kr.Query)
		if p := strings.TrimSpace(kr.Purpose); p != "" {
			label += "; purpose: " + p
		}
		label += ")"
		switch {
		case kr.Error != nil:
			missing = append(missing, fmt.Sprintf("- %s: %v", label, kr.Error))
		case strings.TrimSpace(kr.Response) == "":
			missing = append(missing, fmt.Sprintf("- %s: returned nothing", label))
		default:
			fmt.Fprintf(&sb, "### From %s\n%s\n\n", label, kr.Response)
			gathered++
		}
	}
	if gathered == 0 {
		sb.WriteString("(All consultations failed — answer from your own knowledge and say what could not be verified.)\n")
	}
	if len(missing) > 0 {
		sb.WriteString("\nConsultations that returned no knowledge; what they were asked is not answered by the knowledge above:\n")
		sb.WriteString(strings.Join(missing, "\n"))
		sb.WriteString("\n")
	}
	sb.WriteString("---\n\nAnswer the user's question directly and completely using this knowledge. Cite which specialist informed which part when relevant. Do not describe the consultation process.")
	return sb.String()
}

// matchSpecialistForQuery attempts to find the best specialist for a given query
// by checking the agents registry and matching keywords.
func (m *Model) matchSpecialistForQuery(query string) string {
	queryLower := strings.ToLower(query)

	// Try to load agents from registry
	if m.workspace != "" {
		registryPath := filepath.Join(m.workspace, ".nerd", "agents.json")
		if data, err := os.ReadFile(registryPath); err == nil {
			var reg Registry
			if err := json.Unmarshal(data, &reg); err == nil {
				for _, agent := range reg.Agents {
					for _, kw := range agent.Keywords {
						if strings.Contains(queryLower, strings.ToLower(kw)) {
							return agent.Name
						}
					}
				}
			}
		}
	}

	// Keyword-based specialist matching, in a fixed order: the first
	// specialist listed that matches wins. (This was a map, so a query that
	// matched two specialists was routed by Go's randomized iteration order.)
	for _, sp := range builtinSpecialistKeywords {
		for _, kw := range sp.keywords {
			if containsKeyword(queryLower, kw) {
				return sp.name
			}
		}
	}

	// Default to researcher for general knowledge gathering
	return "researcher"
}

// persistKnowledgeResults stores gathered knowledge in the local knowledge database.
// This enables future retrieval via semantic search and JIT prompt compilation.
func (m *Model) persistKnowledgeResults(results []KnowledgeResult) {
	if m.localDB == nil {
		return
	}

	for _, kr := range results {
		if kr.Error != nil {
			continue
		}

		// Create a concept identifier based on session and specialist
		concept := fmt.Sprintf("session/%s/%s/%d",
			oneLine(kr.Query),
			kr.Specialist,
			kr.Timestamp.Unix(),
		)

		// Store with high confidence (specialist knowledge is authoritative)
		if err := m.localDB.StoreKnowledgeAtom(concept, kr.Response, 0.85, "chat/knowledge_request"); err != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Failed to persist knowledge from %s: %v",
				kr.Specialist, err,
			)
		} else {
			logging.Get(logging.CategoryContext).Debug(
				"Persisted knowledge from %s: %d chars",
				kr.Specialist, len(kr.Response),
			)
		}
	}
}

// builtinSpecialistKeywords is the fallback routing table, ordered.
var builtinSpecialistKeywords = []struct {
	name     string
	keywords []string
}{
	{"goexpert", []string{"go", "golang", "goroutine", "channel", "interface{}", "struct"}},
	{"mangleexpert", []string{"mangle", "datalog", "predicate", "fact", "rule", "logic"}},
	{"uiexpert", []string{"bubbletea", "tui", "terminal", "ui", "charm", "lipgloss"}},
	{"securityexpert", []string{"security", "vulnerability", "cve", "injection", "xss", "csrf"}},
	{"testexpert", []string{"test", "testing", "coverage", "mock", "stub", "assert"}},
}

// containsKeyword reports whether kw occurs in text starting at a word
// boundary; a keyword of three letters or fewer must also end at one. A bare
// substring match sent "build" to the UI specialist ("ui") and "category" to Go.
func containsKeyword(text, kw string) bool {
	isWord := func(b byte) bool {
		return b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '_'
	}
	for from := 0; ; {
		i := strings.Index(text[from:], kw)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(kw)
		startOK := start == 0 || !isWord(text[start-1])
		endOK := len(kw) > 3 || end == len(text) || !isWord(text[end])
		if startOK && endOK {
			return true
		}
		from = start + 1
	}
}
