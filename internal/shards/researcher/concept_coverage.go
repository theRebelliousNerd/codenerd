// Package researcher - Concept coverage analysis for intelligent research optimization.
// This file provides tools to analyze existing knowledge atoms and determine
// what concepts are already covered, enabling the researcher to avoid redundant
// Context7 API queries.
package researcher

import (
	"strings"

	"codenerd/internal/logging"
)

// ConceptCoverage represents the coverage analysis for a topic.
type ConceptCoverage struct {
	Topic            string            // The topic being analyzed
	TotalAtoms       int               // Total existing atoms for this topic
	CoveredConcepts  map[string]int    // Concept -> count of atoms
	UniqueKeywords   map[string]bool   // Unique keywords found in content
	QualityScore     float64           // Overall quality (0-1) based on coverage
	ShouldSkipAPI    bool              // True if coverage is sufficient to skip Context7
	GapsIdentified   []string          // Concepts that might need more research
	RecommendedQuery string            // If not skipping, what to query for
}

// ExistingKnowledge holds atoms passed to the researcher for coverage analysis.
type ExistingKnowledge struct {
	Atoms []KnowledgeAtom // Existing atoms from the knowledge store
}

// NewExistingKnowledge creates an ExistingKnowledge from a slice of atoms.
func NewExistingKnowledge(atoms []KnowledgeAtom) *ExistingKnowledge {
	return &ExistingKnowledge{Atoms: atoms}
}

// AnalyzeTopicCoverage analyzes existing atoms to determine coverage for a topic.
// Returns a ConceptCoverage struct indicating whether Context7 should be queried.
func AnalyzeTopicCoverage(topic string, existing *ExistingKnowledge) *ConceptCoverage {
	coverage := &ConceptCoverage{
		Topic:           topic,
		CoveredConcepts: make(map[string]int),
		UniqueKeywords:  make(map[string]bool),
		GapsIdentified:  make([]string, 0),
	}

	if existing == nil || len(existing.Atoms) == 0 {
		coverage.ShouldSkipAPI = false
		coverage.RecommendedQuery = topic
		return coverage
	}

	// Normalize topic for matching
	topicLower := strings.ToLower(topic)
	topicKeywords := extractKeywords(topicLower)

	// Analyze each atom for relevance to this topic
	relevantAtoms := make([]KnowledgeAtom, 0)
	for _, atom := range existing.Atoms {
		if isAtomRelevantToTopic(atom, topicLower, topicKeywords) {
			relevantAtoms = append(relevantAtoms, atom)

			// Track concepts
			concept := strings.ToLower(atom.Concept)
			coverage.CoveredConcepts[concept]++

			// Track keywords from content
			contentKeywords := extractKeywords(strings.ToLower(atom.Content))
			for kw := range contentKeywords {
				coverage.UniqueKeywords[kw] = true
			}
		}
	}

	coverage.TotalAtoms = len(relevantAtoms)

	// Calculate quality score based on coverage metrics
	coverage.QualityScore = calculateCoverageQuality(coverage, topicKeywords)

	// Determine if we should skip the API
	coverage.ShouldSkipAPI = shouldSkipContext7Query(coverage)

	// If not skipping, identify gaps and recommend a targeted query
	if !coverage.ShouldSkipAPI {
		coverage.GapsIdentified = identifyKnowledgeGaps(topicKeywords, coverage.UniqueKeywords)
		coverage.RecommendedQuery = buildTargetedQuery(topic, coverage.GapsIdentified)
	}

	logging.Researcher("[ConceptCoverage] Topic '%s': %d relevant atoms, quality=%.2f, skip=%v",
		topic, coverage.TotalAtoms, coverage.QualityScore, coverage.ShouldSkipAPI)

	return coverage
}

// isAtomRelevantToTopic checks if an atom is relevant to the given topic.
func isAtomRelevantToTopic(atom KnowledgeAtom, topicLower string, topicKeywords map[string]bool) bool {
	// Check concept match
	conceptLower := strings.ToLower(atom.Concept)
	if strings.Contains(topicLower, conceptLower) || strings.Contains(conceptLower, topicLower) {
		return true
	}

	// Check title match
	titleLower := strings.ToLower(atom.Title)
	if strings.Contains(titleLower, topicLower) {
		return true
	}

	// Check keyword overlap in content
	contentLower := strings.ToLower(atom.Content)
	matchCount := 0
	for kw := range topicKeywords {
		if len(kw) >= 3 && strings.Contains(contentLower, kw) {
			matchCount++
		}
	}

	// Consider relevant if at least half the topic keywords are in content
	if len(topicKeywords) > 0 && matchCount >= (len(topicKeywords)+1)/2 {
		return true
	}

	return false
}

// extractKeywords extracts meaningful keywords from text (3+ chars, no stopwords).
func extractKeywords(text string) map[string]bool {
	keywords := make(map[string]bool)
	stopwords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true, "this": true,
		"that": true, "from": true, "are": true, "was": true, "were": true,
		"been": true, "have": true, "has": true, "had": true, "will": true,
		"would": true, "could": true, "should": true, "may": true, "might": true,
		"can": true, "not": true, "but": true, "all": true, "any": true,
		"how": true, "when": true, "where": true, "what": true, "which": true,
		"who": true, "whom": true, "why": true, "use": true, "using": true,
		"used": true, "get": true, "set": true, "new": true, "make": true,
	}

	// Split on non-alphanumeric characters
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

	for _, word := range words {
		if len(word) >= 3 && !stopwords[word] {
			keywords[word] = true
		}
	}

	return keywords
}

// calculateCoverageQuality calculates a quality score (0-1) based on coverage metrics.
func calculateCoverageQuality(coverage *ConceptCoverage, topicKeywords map[string]bool) float64 {
	if coverage.TotalAtoms == 0 {
		return 0.0
	}

	score := 0.0

	// Factor 1: Atom count (0-0.4)
	// More atoms = better coverage, diminishing returns after 20
	atomScore := float64(coverage.TotalAtoms) / 20.0
	if atomScore > 0.4 {
		atomScore = 0.4
	}
	score += atomScore

	// Factor 2: Concept diversity (0-0.3)
	// More unique concepts = broader coverage
	conceptScore := float64(len(coverage.CoveredConcepts)) / 10.0
	if conceptScore > 0.3 {
		conceptScore = 0.3
	}
	score += conceptScore

	// Factor 3: Keyword coverage (0-0.3)
	// How many topic keywords are covered by existing content
	if len(topicKeywords) > 0 {
		coveredKeywords := 0
		for kw := range topicKeywords {
			if coverage.UniqueKeywords[kw] {
				coveredKeywords++
			}
		}
		keywordScore := float64(coveredKeywords) / float64(len(topicKeywords)) * 0.3
		score += keywordScore
	}

	return score
}

// shouldSkipContext7Query determines if existing coverage is sufficient.
func shouldSkipContext7Query(coverage *ConceptCoverage) bool {
	// Skip if we have high quality coverage
	if coverage.QualityScore >= 0.7 {
		return true
	}

	// Skip if we have many atoms (20+) even with lower quality
	if coverage.TotalAtoms >= 20 && coverage.QualityScore >= 0.5 {
		return true
	}

	// Skip if we have good concept diversity (5+ unique concepts)
	if len(coverage.CoveredConcepts) >= 5 && coverage.QualityScore >= 0.5 {
		return true
	}

	return false
}

// identifyKnowledgeGaps finds concepts/keywords that might need more research.
func identifyKnowledgeGaps(topicKeywords map[string]bool, coveredKeywords map[string]bool) []string {
	gaps := make([]string, 0)

	for kw := range topicKeywords {
		if !coveredKeywords[kw] {
			gaps = append(gaps, kw)
		}
	}

	return gaps
}

// buildTargetedQuery builds a more targeted query based on identified gaps.
func buildTargetedQuery(originalTopic string, gaps []string) string {
	if len(gaps) == 0 {
		return originalTopic
	}

	// If we have gaps, create a more targeted query
	// For example: "bubbletea" with gaps ["keyboard", "events"]
	// becomes "bubbletea keyboard events"
	if len(gaps) <= 3 {
		return originalTopic + " " + strings.Join(gaps, " ")
	}

	// If many gaps, just use the top 3
	return originalTopic + " " + strings.Join(gaps[:3], " ")
}

// FilterTopicsWithCoverage filters a list of topics based on existing knowledge.
// Returns: (topicsNeedingResearch, skippedTopics, coverageReport)
func FilterTopicsWithCoverage(topics []string, existing *ExistingKnowledge) ([]string, []string, map[string]*ConceptCoverage) {
	needResearch := make([]string, 0, len(topics))
	skipped := make([]string, 0)
	report := make(map[string]*ConceptCoverage)

	for _, topic := range topics {
		coverage := AnalyzeTopicCoverage(topic, existing)
		report[topic] = coverage

		if coverage.ShouldSkipAPI {
			skipped = append(skipped, topic)
			logging.Researcher("[ConceptCoverage] Skipping '%s' - sufficient coverage (atoms=%d, quality=%.2f)",
				topic, coverage.TotalAtoms, coverage.QualityScore)
		} else {
			// Use the recommended (targeted) query instead of original
			needResearch = append(needResearch, coverage.RecommendedQuery)
		}
	}

	logging.Researcher("[ConceptCoverage] Topics: %d total, %d need research, %d skipped",
		len(topics), len(needResearch), len(skipped))

	return needResearch, skipped, report
}

// ConvertStoreAtomsToResearcherAtoms converts store-format atoms to researcher atoms.
// This is needed because the knowledge store and researcher use different atom types.
func ConvertStoreAtomsToResearcherAtoms(storeAtoms []map[string]interface{}) []KnowledgeAtom {
	atoms := make([]KnowledgeAtom, 0, len(storeAtoms))

	for _, sa := range storeAtoms {
		atom := KnowledgeAtom{}

		if concept, ok := sa["concept"].(string); ok {
			atom.Concept = concept
		}
		if content, ok := sa["content"].(string); ok {
			atom.Content = content
		}
		if title, ok := sa["title"].(string); ok {
			atom.Title = title
		}
		if conf, ok := sa["confidence"].(float64); ok {
			atom.Confidence = conf
		}
		if source, ok := sa["source_url"].(string); ok {
			atom.SourceURL = source
		}

		atoms = append(atoms, atom)
	}

	return atoms
}
