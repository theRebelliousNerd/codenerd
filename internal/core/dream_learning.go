// Package core implements Dream State learning integration.
// This file implements intelligent extraction, filtering, and persistence of
// learnable knowledge from Dream State multi-agent consultations.
//
// Architecture:
//
//	Dream Consultation → Extract Candidates → Score Usefulness → Stage for Confirmation → Route to Store
//
// Learning Categories:
//   - Procedural:  "How to do X" → LearningStore (per-shard)
//   - ToolNeeds:   "I'd need Y" → Ouroboros queue
//   - RiskPatterns: "Watch for Z" → Kernel facts + Cold Storage
//   - Preferences: "User wants W" → Cold Storage
package core

import (
	"codenerd/internal/logging"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DreamLearningType categorizes extracted knowledge.
type DreamLearningType string

const (
	// LearningTypeProcedural represents workflow/approach knowledge
	LearningTypeProcedural DreamLearningType = "procedural"
	// LearningTypeToolNeed represents missing capability identified
	LearningTypeToolNeed DreamLearningType = "tool_need"
	// LearningTypeRiskPattern represents safety/risk awareness
	LearningTypeRiskPattern DreamLearningType = "risk_pattern"
	// LearningTypePreference represents user/project preference
	LearningTypePreference DreamLearningType = "preference"
)

// DreamLearning represents a single learnable insight extracted from Dream State.
type DreamLearning struct {
	ID              string            `json:"id"`
	Type            DreamLearningType `json:"type"`
	SourceShard     string            `json:"source_shard"`
	SourceShardType string            `json:"source_shard_type"` // ephemeral, persistent, system
	Hypothetical    string            `json:"hypothetical"`      // Original dream query
	Content         string            `json:"content"`           // The actual learning
	Confidence      float64           `json:"confidence"`        // 0.0-1.0, starts at 0.5 before confirmation
	Novelty         float64           `json:"novelty"`           // 0.0-1.0, how new/specific is this?
	ExtractedAt     time.Time         `json:"extracted_at"`
	Confirmed       bool              `json:"confirmed"`        // User said "correct!" or similar
	ConfirmedAt     *time.Time        `json:"confirmed_at"`     // When confirmed
	Persisted       bool              `json:"persisted"`        // Routed to permanent store
	PersistedTo     string            `json:"persisted_to"`     // Which store received it
	Metadata        map[string]string `json:"metadata"`         // Extra context (tool_name, risk_type, etc.)
}

// DreamLearningCollector manages staged learnings from Dream State consultations.
// Learnings are staged here until user confirmation, then routed to appropriate stores.
type DreamLearningCollector struct {
	mu            sync.RWMutex
	staged        map[string]*DreamLearning // ID -> learning (awaiting confirmation)
	confirmed     []*DreamLearning          // Confirmed but not yet persisted
	knownPatterns map[string]bool           // Deduplication: already learned patterns
	lastDreamID   string                    // ID of the most recent dream for confirmation matching
}

// NewDreamLearningCollector creates a new collector.
func NewDreamLearningCollector() *DreamLearningCollector {
	logging.Dream("Creating DreamLearningCollector")
	return &DreamLearningCollector{
		staged:        make(map[string]*DreamLearning),
		confirmed:     make([]*DreamLearning, 0),
		knownPatterns: make(map[string]bool),
	}
}

// ExtractLearnings parses Dream State consultations and extracts learnable insights.
// Returns only novel, useful learnings (filters out generic/known patterns).
func (c *DreamLearningCollector) ExtractLearnings(hypothetical string, consultations []DreamConsultation) []*DreamLearning {
	c.mu.Lock()
	defer c.mu.Unlock()

	dreamID := generateDreamID(hypothetical)
	c.lastDreamID = dreamID

	var learnings []*DreamLearning
	extractedCount := 0
	filteredCount := 0

	logging.Dream("ExtractLearnings: processing %d consultations for hypothetical: %s", len(consultations), truncate(hypothetical, 50))

	for _, consultation := range consultations {
		if consultation.Error != nil {
			continue
		}

		// Extract each category of learning
		procedural := c.extractProcedural(dreamID, hypothetical, consultation)
		toolNeeds := c.extractToolNeeds(dreamID, hypothetical, consultation)
		risks := c.extractRiskPatterns(dreamID, hypothetical, consultation)

		// Filter for novelty and usefulness
		for _, l := range append(append(procedural, toolNeeds...), risks...) {
			extractedCount++
			if c.isUseful(l) {
				l.ID = generateLearningID(dreamID, l.Type, len(learnings))
				learnings = append(learnings, l)
				c.staged[l.ID] = l
				logging.DreamDebug("ExtractLearnings: staged learning %s (type=%s, novelty=%.2f)", l.ID, l.Type, l.Novelty)
			} else {
				filteredCount++
				logging.DreamDebug("ExtractLearnings: filtered out learning (type=%s, content=%s)", l.Type, truncate(l.Content, 40))
			}
		}
	}

	logging.Dream("ExtractLearnings: extracted %d learnings, filtered %d (novelty/usefulness check)", len(learnings), filteredCount)
	return learnings
}

// extractProcedural finds workflow/approach knowledge from shard responses.
func (c *DreamLearningCollector) extractProcedural(dreamID, hypothetical string, consultation DreamConsultation) []*DreamLearning {
	var learnings []*DreamLearning
	text := consultation.Perspective

	// Look for numbered steps, approach sections
	stepPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:steps?|approach|process|workflow|procedure)[\s:]+\n?((?:\s*(?:\d+[\.\)]\s*|\*\s*|-\s*).+\n?)+)`),
		regexp.MustCompile(`(?i)I would:?\n?((?:\s*(?:\d+[\.\)]\s*|\*\s*|-\s*).+\n?)+)`),
	}

	for _, pattern := range stepPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) > 1 {
				content := strings.TrimSpace(match[1])
				if len(content) > 20 && len(content) < 2000 {
					learnings = append(learnings, &DreamLearning{
						Type:            LearningTypeProcedural,
						SourceShard:     consultation.ShardName,
						SourceShardType: consultation.ShardType,
						Hypothetical:    hypothetical,
						Content:         content,
						Confidence:      0.5, // Pre-confirmation confidence
						Novelty:         c.scoreNovelty(content),
						ExtractedAt:     time.Now(),
						Metadata: map[string]string{
							"dream_id":     dreamID,
							"extract_type": "steps",
						},
					})
				}
			}
		}
	}

	return learnings
}

// extractToolNeeds identifies missing capabilities mentioned by shards.
func (c *DreamLearningCollector) extractToolNeeds(dreamID, hypothetical string, consultation DreamConsultation) []*DreamLearning {
	var learnings []*DreamLearning
	text := consultation.Perspective

	// Patterns that indicate tool/capability needs
	needPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:need|require|want|would use|missing)[^.]*(?:tool|command|utility|script|function|capability)[^.]*\.`),
		regexp.MustCompile(`(?i)(?:tool|command|utility)[^.]*(?:doesn't exist|not available|would need to create)[^.]*\.`),
		regexp.MustCompile(`(?i)(?:create|build|implement|write)[^.]*(?:tool|script|utility)[^.]*(?:for|to)[^.]*\.`),
	}

	for _, pattern := range needPatterns {
		matches := pattern.FindAllString(text, -1)
		for _, match := range matches {
			content := strings.TrimSpace(match)
			if len(content) > 15 && len(content) < 500 {
				// Extract potential tool name
				toolName := extractToolName(content)

				learnings = append(learnings, &DreamLearning{
					Type:            LearningTypeToolNeed,
					SourceShard:     consultation.ShardName,
					SourceShardType: consultation.ShardType,
					Hypothetical:    hypothetical,
					Content:         content,
					Confidence:      0.5,
					Novelty:         c.scoreNovelty(content),
					ExtractedAt:     time.Now(),
					Metadata: map[string]string{
						"dream_id":  dreamID,
						"tool_name": toolName,
					},
				})
			}
		}
	}

	return learnings
}

// extractRiskPatterns identifies safety/risk awareness from shard responses.
func (c *DreamLearningCollector) extractRiskPatterns(dreamID, hypothetical string, consultation DreamConsultation) []*DreamLearning {
	var learnings []*DreamLearning
	text := consultation.Perspective

	// Patterns indicating risk awareness
	riskPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:risk|danger|careful|warning|caution)[^.]*(?:could|might|may|would)[^.]*\.`),
		regexp.MustCompile(`(?i)(?:could fail|might break|may cause|would corrupt)[^.]*\.`),
		regexp.MustCompile(`(?i)(?:never|always|must not|should not)[^.]*(?:because|since|as)[^.]*\.`),
	}

	for _, pattern := range riskPatterns {
		matches := pattern.FindAllString(text, -1)
		for _, match := range matches {
			content := strings.TrimSpace(match)
			if len(content) > 20 && len(content) < 500 {
				// Classify risk type
				riskType := classifyRiskType(content)

				learnings = append(learnings, &DreamLearning{
					Type:            LearningTypeRiskPattern,
					SourceShard:     consultation.ShardName,
					SourceShardType: consultation.ShardType,
					Hypothetical:    hypothetical,
					Content:         content,
					Confidence:      0.5,
					Novelty:         c.scoreNovelty(content),
					ExtractedAt:     time.Now(),
					Metadata: map[string]string{
						"dream_id":  dreamID,
						"risk_type": riskType,
					},
				})
			}
		}
	}

	return learnings
}

// isUseful determines if a learning is worth staging for confirmation.
// Filters out generic, already-known, or low-value insights.
func (c *DreamLearningCollector) isUseful(l *DreamLearning) bool {
	// Must have minimum novelty
	if l.Novelty < 0.3 {
		return false
	}

	// Check if already known
	patternKey := l.Type.String() + ":" + normalizeForDedup(l.Content)
	if c.knownPatterns[patternKey] {
		return false
	}

	// Filter out generic advice
	genericPhrases := []string{
		"best practices",
		"should always",
		"it depends",
		"consult documentation",
		"follow guidelines",
		"standard approach",
	}
	lower := strings.ToLower(l.Content)
	for _, phrase := range genericPhrases {
		if strings.Contains(lower, phrase) && l.Novelty < 0.6 {
			return false
		}
	}

	// Tool needs from specialists are more valuable
	if l.Type == LearningTypeToolNeed && strings.Contains(l.SourceShardType, "knowledge") {
		l.Novelty += 0.2 // Boost novelty for specialist insights
	}

	return true
}

// scoreNovelty estimates how novel/specific a learning is.
// Higher = more specific to this project/context.
func (c *DreamLearningCollector) scoreNovelty(content string) float64 {
	score := 0.5 // Base score

	// Specific paths/files boost novelty
	if strings.Contains(content, "/") || strings.Contains(content, "\\") || strings.Contains(content, ".go") {
		score += 0.2
	}

	// Project-specific terms boost novelty
	projectTerms := []string{"mangle", "shard", "kernel", "transducer", "ouroboros", "campaign", "nerd"}
	lower := strings.ToLower(content)
	for _, term := range projectTerms {
		if strings.Contains(lower, term) {
			score += 0.1
		}
	}

	// Concrete numbers/specifics boost novelty
	if regexp.MustCompile(`\d+`).MatchString(content) {
		score += 0.1
	}

	// Generic terms reduce novelty
	genericTerms := []string{"usually", "typically", "generally", "common", "standard"}
	for _, term := range genericTerms {
		if strings.Contains(lower, term) {
			score -= 0.15
		}
	}

	// Clamp to [0.0, 1.0]
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// ConfirmLearnings marks learnings as confirmed by user feedback.
// Call this when user says "correct!", "learn this", "remember that", etc.
func (c *DreamLearningCollector) ConfirmLearnings(userFeedback string) []*DreamLearning {
	c.mu.Lock()
	defer c.mu.Unlock()

	var confirmed []*DreamLearning
	now := time.Now()

	// Confirmation boosts confidence
	confidenceBoost := 0.3
	if isStrongConfirmation(userFeedback) {
		confidenceBoost = 0.5
	}

	for _, l := range c.staged {
		if !l.Confirmed {
			l.Confirmed = true
			l.ConfirmedAt = &now
			l.Confidence += confidenceBoost
			if l.Confidence > 1.0 {
				l.Confidence = 1.0
			}
			confirmed = append(confirmed, l)
			c.confirmed = append(c.confirmed, l)

			// Mark pattern as known for deduplication
			patternKey := l.Type.String() + ":" + normalizeForDedup(l.Content)
			c.knownPatterns[patternKey] = true

			logging.Dream("ConfirmLearnings: confirmed %s (type=%s, confidence=%.2f)", l.ID, l.Type, l.Confidence)
		}
	}

	// Clear staging for confirmed items
	for _, l := range confirmed {
		delete(c.staged, l.ID)
	}

	logging.Dream("ConfirmLearnings: confirmed %d learnings", len(confirmed))
	return confirmed
}

// LearnCorrection handles user corrections ("no, actually...", "wrong, we use...").
// Creates a new learning with the corrected information at higher confidence.
func (c *DreamLearningCollector) LearnCorrection(correction string, learningType DreamLearningType) *DreamLearning {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	learning := &DreamLearning{
		ID:          generateLearningID(c.lastDreamID, learningType, len(c.confirmed)),
		Type:        learningType,
		SourceShard: "user_correction",
		Hypothetical: func() string {
			for _, l := range c.staged {
				return l.Hypothetical
			}
			return ""
		}(),
		Content:     correction,
		Confidence:  0.9, // Corrections start at high confidence
		Novelty:     1.0, // User corrections are always novel
		ExtractedAt: now,
		Confirmed:   true,
		ConfirmedAt: &now,
		Metadata: map[string]string{
			"source": "user_correction",
		},
	}

	c.confirmed = append(c.confirmed, learning)
	patternKey := learning.Type.String() + ":" + normalizeForDedup(learning.Content)
	c.knownPatterns[patternKey] = true

	logging.Dream("LearnCorrection: learned user correction (type=%s, content=%s)", learningType, truncate(correction, 50))
	return learning
}

// GetPendingLearnings returns staged learnings awaiting confirmation.
func (c *DreamLearningCollector) GetPendingLearnings() []*DreamLearning {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pending := make([]*DreamLearning, 0, len(c.staged))
	for _, l := range c.staged {
		pending = append(pending, l)
	}
	return pending
}

// GetConfirmedLearnings returns confirmed learnings ready for persistence.
func (c *DreamLearningCollector) GetConfirmedLearnings() []*DreamLearning {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return append([]*DreamLearning{}, c.confirmed...)
}

// MarkPersisted records that a learning has been routed to permanent storage.
func (c *DreamLearningCollector) MarkPersisted(learningID, storeName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, l := range c.confirmed {
		if l.ID == learningID {
			c.confirmed[i].Persisted = true
			c.confirmed[i].PersistedTo = storeName
			logging.DreamDebug("MarkPersisted: %s → %s", learningID, storeName)
			break
		}
	}
}

// ClearStaged removes non-confirmed learnings (e.g., when user ignores dream output).
func (c *DreamLearningCollector) ClearStaged() {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.staged)
	c.staged = make(map[string]*DreamLearning)
	logging.DreamDebug("ClearStaged: cleared %d staged learnings", count)
}

// GetStats returns statistics about dream learning activity.
func (c *DreamLearningCollector) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := map[string]interface{}{
		"staged_count":        len(c.staged),
		"confirmed_count":     len(c.confirmed),
		"known_patterns":      len(c.knownPatterns),
		"last_dream_id":       c.lastDreamID,
		"confirmed_by_type":   make(map[string]int),
		"avg_confidence":      0.0,
		"pending_persistence": 0,
	}

	var totalConfidence float64
	for _, l := range c.confirmed {
		stats["confirmed_by_type"].(map[string]int)[string(l.Type)]++
		totalConfidence += l.Confidence
		if !l.Persisted {
			stats["pending_persistence"] = stats["pending_persistence"].(int) + 1
		}
	}

	if len(c.confirmed) > 0 {
		stats["avg_confidence"] = totalConfidence / float64(len(c.confirmed))
	}

	return stats
}

// --- Helper Functions ---

func generateDreamID(hypothetical string) string {
	// Simple hash-like ID from hypothetical + timestamp
	h := 0
	for _, c := range hypothetical {
		h = 31*h + int(c)
	}
	return strings.ToLower(strings.ReplaceAll(
		strings.ReplaceAll(truncate(hypothetical, 20), " ", "_"),
		".", "")) + "_" + time.Now().Format("150405")
}

func generateLearningID(dreamID string, learningType DreamLearningType, index int) string {
	return strings.ToLower(string(learningType)[:4]) + "_" + dreamID + "_" + time.Now().Format("150405.000")[7:]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func normalizeForDedup(content string) string {
	// Normalize whitespace, lowercase, remove punctuation for dedup comparison
	content = strings.ToLower(content)
	content = regexp.MustCompile(`[^a-z0-9\s]`).ReplaceAllString(content, "")
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	return strings.TrimSpace(content)
}

func extractToolName(content string) string {
	// Try to extract tool name from patterns like "need a X tool" or "create X"
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:tool|script|utility|command)\s+(?:called\s+)?["']?(\w+)["']?`),
		regexp.MustCompile(`(?i)(?:create|build|implement)\s+(?:a\s+)?["']?(\w+)["']?\s+(?:tool|script|utility)`),
		regexp.MustCompile(`(?i)["'](\w+)["']\s+(?:tool|script|utility)`),
	}

	for _, pattern := range patterns {
		if match := pattern.FindStringSubmatch(content); len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func classifyRiskType(content string) string {
	lower := strings.ToLower(content)

	switch {
	case strings.Contains(lower, "security") || strings.Contains(lower, "credential") || strings.Contains(lower, "secret"):
		return "security"
	case strings.Contains(lower, "data") || strings.Contains(lower, "loss") || strings.Contains(lower, "corrupt"):
		return "data_integrity"
	case strings.Contains(lower, "performance") || strings.Contains(lower, "slow") || strings.Contains(lower, "memory"):
		return "performance"
	case strings.Contains(lower, "break") || strings.Contains(lower, "fail") || strings.Contains(lower, "crash"):
		return "stability"
	case strings.Contains(lower, "deploy") || strings.Contains(lower, "production") || strings.Contains(lower, "release"):
		return "deployment"
	default:
		return "general"
	}
}

func isStrongConfirmation(input string) bool {
	lower := strings.ToLower(input)
	strongPhrases := []string{
		"correct!",
		"exactly!",
		"yes!",
		"perfect",
		"learn this",
		"remember this",
		"always do",
		"that's right",
	}
	for _, phrase := range strongPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}

// String returns the string representation of a DreamLearningType.
func (t DreamLearningType) String() string {
	return string(t)
}

// DreamConsultation is imported from process.go - defined here for package reference.
// Actual struct is in cmd/nerd/chat/process.go
type DreamConsultation struct {
	ShardName   string
	ShardType   string
	Perspective string
	Tools       []string
	Concerns    []string
	Error       error
}
