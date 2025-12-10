// Package core implements Dream State learning persistence routing.
// This file routes confirmed learnings to the appropriate storage tier:
//   - Procedural → LearningStore (per-shard SQLite)
//   - ToolNeeds  → Ouroboros queue + Kernel fact
//   - RiskPatterns → Kernel fact + Cold Storage
//   - Preferences → Cold Storage
package core

import (
	"codenerd/internal/logging"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DreamRouter routes confirmed learnings to appropriate persistence stores.
type DreamRouter struct {
	mu            sync.RWMutex
	kernel        Kernel
	learningStore LearningStoreSaver // Interface to avoid import cycle
	coldStore     ColdStoreSaver     // Interface to avoid import cycle
	ouroborosQ    chan<- ToolNeed    // Channel to Ouroboros tool generation queue
}

// LearningStoreSaver is the interface for persisting per-shard learnings.
type LearningStoreSaver interface {
	Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error
}

// ColdStoreSaver is the interface for persisting long-term facts.
type ColdStoreSaver interface {
	StoreFact(predicate string, args []interface{}, factType string, importance int) error
}

// ToolNeed represents a capability gap identified by Dream State.
// Sent to Ouroboros for potential tool generation.
type ToolNeed struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Priority    float64   `json:"priority"`
	SourceDream string    `json:"source_dream"`
	SourceShard string    `json:"source_shard"`
	IdentifiedAt time.Time `json:"identified_at"`
}

// NewDreamRouter creates a router with the given backend stores.
func NewDreamRouter(kernel Kernel, learningStore LearningStoreSaver, coldStore ColdStoreSaver) *DreamRouter {
	logging.Dream("Creating DreamRouter")
	return &DreamRouter{
		kernel:        kernel,
		learningStore: learningStore,
		coldStore:     coldStore,
	}
}

// SetOuroborosQueue connects the router to the Ouroboros tool generation pipeline.
func (r *DreamRouter) SetOuroborosQueue(queue chan<- ToolNeed) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ouroborosQ = queue
	logging.DreamDebug("DreamRouter: Ouroboros queue connected")
}

// RouteResult tracks what happened when routing a learning.
type RouteResult struct {
	LearningID   string
	Success      bool
	Destination  string
	ErrorMessage string
}

// RouteLearnings persists confirmed learnings to appropriate stores.
// Returns results for each learning indicating where it was routed.
func (r *DreamRouter) RouteLearnings(learnings []*DreamLearning) []RouteResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]RouteResult, 0, len(learnings))

	for _, l := range learnings {
		if !l.Confirmed {
			logging.DreamDebug("RouteLearnings: skipping unconfirmed learning %s", l.ID)
			continue
		}

		if l.Persisted {
			logging.DreamDebug("RouteLearnings: skipping already-persisted learning %s", l.ID)
			continue
		}

		var result RouteResult
		result.LearningID = l.ID

		switch l.Type {
		case LearningTypeProcedural:
			result = r.routeProcedural(l)
		case LearningTypeToolNeed:
			result = r.routeToolNeed(l)
		case LearningTypeRiskPattern:
			result = r.routeRiskPattern(l)
		case LearningTypePreference:
			result = r.routePreference(l)
		default:
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("unknown learning type: %s", l.Type)
		}

		results = append(results, result)

		if result.Success {
			l.Persisted = true
			l.PersistedTo = result.Destination
			logging.Dream("RouteLearnings: %s → %s", l.ID, result.Destination)
		} else {
			logging.Get(logging.CategoryDream).Error("RouteLearnings: failed to route %s: %s", l.ID, result.ErrorMessage)
		}
	}

	return results
}

// routeProcedural sends workflow/approach knowledge to the shard's learning store.
func (r *DreamRouter) routeProcedural(l *DreamLearning) RouteResult {
	result := RouteResult{LearningID: l.ID}

	if r.learningStore == nil {
		result.ErrorMessage = "learning store not configured"
		return result
	}

	// Determine target shard from source
	shardType := l.SourceShard
	if shardType == "" {
		shardType = "general"
	}

	// Build predicate based on content analysis
	predicate := "approach_learned"
	args := []any{
		l.Hypothetical,
		l.Content,
		l.Confidence,
	}

	if err := r.learningStore.Save(shardType, predicate, args, "dream_state"); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	result.Success = true
	result.Destination = fmt.Sprintf("LearningStore:%s", shardType)
	return result
}

// routeToolNeed queues capability gaps for Ouroboros and records as kernel fact.
func (r *DreamRouter) routeToolNeed(l *DreamLearning) RouteResult {
	result := RouteResult{LearningID: l.ID}

	// Extract tool name from metadata or content
	toolName := l.Metadata["tool_name"]
	if toolName == "" {
		toolName = inferToolName(l.Content)
	}

	// Record as kernel fact for query
	if r.kernel != nil {
		fact := Fact{
			Predicate: "dream_tool_need",
			Args: []interface{}{
				toolName,
				l.Content,
				l.Confidence,
				l.Hypothetical,
			},
		}
		if err := r.kernel.Assert(fact); err != nil {
			logging.DreamDebug("routeToolNeed: failed to assert kernel fact: %v", err)
		}
	}

	// Queue for Ouroboros if channel available
	if r.ouroborosQ != nil {
		need := ToolNeed{
			Name:        toolName,
			Description: l.Content,
			Priority:    l.Confidence * l.Novelty, // Combined score
			SourceDream: l.Hypothetical,
			SourceShard: l.SourceShard,
			IdentifiedAt: l.ExtractedAt,
		}

		select {
		case r.ouroborosQ <- need:
			logging.Dream("routeToolNeed: queued tool need '%s' for Ouroboros", toolName)
			result.Success = true
			result.Destination = "OuroborosQueue"
		default:
			// Queue full, just persist to kernel
			result.Success = true
			result.Destination = "Kernel:dream_tool_need"
			logging.DreamDebug("routeToolNeed: Ouroboros queue full, persisted to kernel only")
		}
	} else {
		result.Success = true
		result.Destination = "Kernel:dream_tool_need"
	}

	return result
}

// routeRiskPattern persists safety/risk awareness to kernel and cold storage.
func (r *DreamRouter) routeRiskPattern(l *DreamLearning) RouteResult {
	result := RouteResult{LearningID: l.ID}

	riskType := l.Metadata["risk_type"]
	if riskType == "" {
		riskType = "general"
	}

	// Assert to kernel for immediate availability
	if r.kernel != nil {
		fact := Fact{
			Predicate: "dream_risk_pattern",
			Args: []interface{}{
				riskType,
				l.Content,
				l.Confidence,
			},
		}
		if err := r.kernel.Assert(fact); err != nil {
			logging.DreamDebug("routeRiskPattern: failed to assert kernel fact: %v", err)
		}
	}

	// High-confidence risks go to cold storage for persistence
	if r.coldStore != nil && l.Confidence >= 0.7 {
		importance := int(l.Confidence * 10) // 7-10 based on confidence
		if err := r.coldStore.StoreFact(
			"learned_risk",
			[]interface{}{riskType, l.Content, l.Hypothetical},
			"risk_pattern",
			importance,
		); err != nil {
			result.ErrorMessage = err.Error()
			return result
		}
		result.Success = true
		result.Destination = "ColdStorage:learned_risk"
	} else {
		result.Success = true
		result.Destination = "Kernel:dream_risk_pattern"
	}

	return result
}

// routePreference persists user/project preferences to cold storage.
func (r *DreamRouter) routePreference(l *DreamLearning) RouteResult {
	result := RouteResult{LearningID: l.ID}

	if r.coldStore == nil {
		// Fallback to kernel
		if r.kernel != nil {
			fact := Fact{
				Predicate: "dream_preference",
				Args: []interface{}{
					l.Content,
					l.Confidence,
				},
			}
			r.kernel.Assert(fact)
			result.Success = true
			result.Destination = "Kernel:dream_preference"
		} else {
			result.ErrorMessage = "no storage backend available"
		}
		return result
	}

	// Preferences are high importance and long-lived
	importance := 8
	if l.Confidence >= 0.9 {
		importance = 10
	}

	if err := r.coldStore.StoreFact(
		"user_preference",
		[]interface{}{l.Content, l.SourceShard},
		"preference",
		importance,
	); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	result.Success = true
	result.Destination = "ColdStorage:user_preference"
	return result
}

// inferToolName attempts to extract a tool name from description text.
func inferToolName(content string) string {
	// Simple heuristic: find a word before "tool", "utility", "script", etc.
	words := strings.Fields(strings.ToLower(content))
	for i, w := range words {
		if w == "tool" || w == "utility" || w == "script" || w == "command" {
			if i > 0 {
				candidate := words[i-1]
				// Filter out articles and adjectives
				if candidate != "a" && candidate != "an" && candidate != "the" &&
					candidate != "new" && candidate != "custom" && candidate != "simple" {
					return candidate
				}
				// Try word before that
				if i > 1 {
					return words[i-2]
				}
			}
		}
	}

	// Fallback: first noun-like word
	for _, w := range words {
		if len(w) > 3 && !isCommonWord(w) {
			return w
		}
	}

	return "unknown_tool"
}

func isCommonWord(w string) bool {
	common := map[string]bool{
		"need": true, "want": true, "would": true, "could": true,
		"should": true, "must": true, "that": true, "this": true,
		"with": true, "from": true, "have": true, "will": true,
	}
	return common[w]
}
