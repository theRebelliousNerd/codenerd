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
) knowledgeGatheredMsg {
	// NOTE: do not set m.awaitingKnowledge here — m is a copy (processInput has a
	// value receiver), so the write would be discarded. The flag is carried on
	// the returned message and applied on the Update thread.

	if len(requests) > maxKnowledgeRequestsPerTurn {
		logging.Get(logging.CategoryContext).Info(
			"Knowledge requests capped: %d requested, honoring first %d",
			len(requests), maxKnowledgeRequestsPerTurn,
		)
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
			// Resolve specialist type
			shardType := r.Specialist
			if shardType == "_any_specialist" {
				shardType = m.matchSpecialistForQuery(r.Query)
			}

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
				shardType, truncateSummary(r.Query, 100),
			)

			// Spawn the specialist with high priority (knowledge is blocking)
			ret, err := m.spawnTaskWithContext(ctx, shardType, task, sessionCtx, types.PriorityHigh)

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
		Results:           results,
		OriginalInput:     originalInput,
		InterimResponse:   interimResponse,
		AwaitingKnowledge: true,
	}
}

// synthesizeWithKnowledge produces the final answer after specialist
// consultations with ONE LLM call. This replaces the old behavior of
// re-running the entire processInput pipeline (perception again, the full
// DECIDE waterfall again, articulation again) with the gathered knowledge
// appended to the input — which doubled or tripled turn cost and re-rolled
// the routing dice mid-turn.
func (m *Model) synthesizeWithKnowledge(originalInput string, results []KnowledgeResult) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				logging.API("PANIC in synthesizeWithKnowledge (recovered): %v", r)
				msg = errorMsg(fmt.Errorf("internal error (recovered panic): %v", r))
			}
		}()

		baseCtx := m.shutdownCtx
		if baseCtx == nil {
			baseCtx = context.Background()
		}
		ctx, cancel := context.WithTimeout(baseCtx, config.GetLLMTimeouts().ArticulationTimeout)
		defer cancel()

		var sb strings.Builder
		sb.WriteString("The user asked:\n\n")
		sb.WriteString(originalInput)
		sb.WriteString("\n\n---\nSpecialist consultations returned the following knowledge:\n\n")
		gathered := 0
		for _, kr := range results {
			if kr.Error != nil || strings.TrimSpace(kr.Response) == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("### From %s (query: %s)\n%s\n\n", kr.Specialist, kr.Query, kr.Response))
			gathered++
		}
		if gathered == 0 {
			sb.WriteString("(All consultations failed — answer from your own knowledge and say what could not be verified.)\n")
		}
		sb.WriteString("---\n\nAnswer the user's question directly and completely using this knowledge. Cite which specialist informed which part when relevant. Do not describe the consultation process.")

		// Same seam, same position as the main chat turn: the persona leads,
		// the role framing for this particular call follows it. See
		// cmd/nerd/chat/persona.go for why this is a constant and not an atom.
		systemPrompt := withArchitectPersona("You are codeNERD answering a user's question after consulting internal specialists.")
		recordArchitectPersonaDelivery("knowledge synthesis", systemPrompt)

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, sb.String())
		if err != nil {
			return errorMsg(fmt.Errorf("knowledge synthesis failed: %w", err))
		}
		logging.Get(logging.CategoryContext).Info(
			"Knowledge synthesis complete: %d specialist responses, %d chars answer",
			gathered, len(response),
		)
		return assistantMsg{Surface: response}
	}
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

	// Keyword-based specialist matching
	specialists := map[string][]string{
		"goexpert":       {"go ", "golang", "goroutine", "channel", "interface{}", "struct"},
		"mangleexpert":   {"mangle", "datalog", "predicate", "fact", "rule", "logic"},
		"uiexpert":       {"bubbletea", "tui", "terminal", "ui", "charm", "lipgloss"},
		"securityexpert": {"security", "vulnerability", "cve", "injection", "xss", "csrf"},
		"testexpert":     {"test", "testing", "coverage", "mock", "stub", "assert"},
	}

	for specialist, keywords := range specialists {
		for _, kw := range keywords {
			if strings.Contains(queryLower, kw) {
				return specialist
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
			truncateSummary(kr.Query, 50),
			kr.Specialist,
			kr.Timestamp.Unix(),
		)

		// Store with high confidence (specialist knowledge is authoritative)
		if err := m.localDB.StoreKnowledgeAtom(concept, kr.Response, 0.85); err != nil {
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
