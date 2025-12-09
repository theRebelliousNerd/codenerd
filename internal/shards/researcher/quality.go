// Package researcher - Research quality metrics and scoring.
// This file implements quality calculation for research results per C3.
package researcher

import (
	"fmt"
	"strings"
)

// QualityMetrics holds quality assessment data for a research result.
type QualityMetrics struct {
	AtomCount        int     // Number of atoms extracted
	SourceDiversity  int     // Number of unique sources
	CodeSnippetCount int     // Number of atoms with code examples
	TopicsCovered    int     // Number of requested topics with results
	TopicsRequested  int     // Total topics requested
	Score            float64 // Overall quality score 0-100
	Rating           string  // "Excellent", "Good", "Adequate", "Needs improvement"
}

// CalculateQualityMetrics computes quality metrics for a research result.
// Quality factors:
// - Atom count (more is better, diminishing returns after 50)
// - Source diversity (multiple sources = higher quality)
// - Code snippet coverage (atoms with code examples score higher)
// - Topic coverage (% of requested topics with atoms)
func CalculateQualityMetrics(atoms []KnowledgeAtom, requestedTopics []string) QualityMetrics {
	metrics := QualityMetrics{
		AtomCount:       len(atoms),
		TopicsRequested: len(requestedTopics),
	}

	// Count unique sources and code snippets
	sources := make(map[string]bool)
	codeCount := 0
	topicsWithAtoms := make(map[string]bool)

	for _, atom := range atoms {
		// Track source diversity
		if atom.SourceURL != "" {
			// Normalize to domain for diversity counting
			domain := extractDomain(atom.SourceURL)
			sources[domain] = true
		}

		// Count code snippets
		if atom.CodePattern != "" && len(atom.CodePattern) > 10 {
			codeCount++
		}

		// Track topic coverage via concept matching
		for _, topic := range requestedTopics {
			topicLower := strings.ToLower(topic)
			titleLower := strings.ToLower(atom.Title)
			contentLower := strings.ToLower(atom.Content)
			if strings.Contains(titleLower, topicLower) || strings.Contains(contentLower, topicLower) {
				topicsWithAtoms[topic] = true
			}
		}
	}

	metrics.SourceDiversity = len(sources)
	metrics.CodeSnippetCount = codeCount
	metrics.TopicsCovered = len(topicsWithAtoms)

	// Calculate overall score (0-100)
	metrics.Score = calculateQualityScore(metrics)
	metrics.Rating = qualityRating(metrics.Score)

	return metrics
}

// calculateQualityScore computes the 0-100 quality score.
// Weights:
// - Atom count: 30% (diminishing returns after 50)
// - Source diversity: 25%
// - Code snippet coverage: 25%
// - Topic coverage: 20%
func calculateQualityScore(m QualityMetrics) float64 {
	var score float64

	// Atom count score (30 points max)
	// Diminishing returns: first 20 atoms = 1.5 points each (30), after that 0.3 each
	atomScore := 0.0
	if m.AtomCount <= 20 {
		atomScore = float64(m.AtomCount) * 1.5
	} else if m.AtomCount <= 50 {
		atomScore = 30.0 // First 20 atoms
		atomScore += float64(m.AtomCount-20) * 0.3
	} else {
		atomScore = 30.0 // First 20 atoms
		atomScore += 9.0 // Next 30 atoms
		// Cap at 39 for atom count
	}
	if atomScore > 30 {
		atomScore = 30
	}
	score += atomScore

	// Source diversity score (25 points max)
	// 1 source = 5 points, 2 = 10, 3 = 15, 4 = 20, 5+ = 25
	diversityScore := float64(m.SourceDiversity) * 5.0
	if diversityScore > 25 {
		diversityScore = 25
	}
	score += diversityScore

	// Code snippet coverage (25 points max)
	// Ratio of code snippets to total atoms, scaled
	codeScore := 0.0
	if m.AtomCount > 0 {
		codeRatio := float64(m.CodeSnippetCount) / float64(m.AtomCount)
		codeScore = codeRatio * 25.0
		// Bonus for having code at all
		if m.CodeSnippetCount > 0 {
			codeScore += 5.0
		}
		if codeScore > 25 {
			codeScore = 25
		}
	}
	score += codeScore

	// Topic coverage score (20 points max)
	topicScore := 0.0
	if m.TopicsRequested > 0 {
		topicRatio := float64(m.TopicsCovered) / float64(m.TopicsRequested)
		topicScore = topicRatio * 20.0
	} else {
		// No specific topics requested, give full marks
		topicScore = 20.0
	}
	score += topicScore

	// Ensure score is in valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// qualityRating returns a human-readable rating for a score.
func qualityRating(score float64) string {
	switch {
	case score >= 90:
		return "Excellent"
	case score >= 75:
		return "Good"
	case score >= 50:
		return "Adequate"
	default:
		return "Needs improvement"
	}
}

// extractDomain extracts the domain from a URL for diversity counting.
func extractDomain(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "file://")

	// Get domain (before first /)
	if idx := strings.Index(url, "/"); idx > 0 {
		url = url[:idx]
	}

	return url
}

// FormatQualityDisplay returns a formatted string for display.
// Format: "AgentName: X atoms (Quality: Y% - Rating)"
func (m QualityMetrics) FormatQualityDisplay(agentName string) string {
	return fmt.Sprintf("%s: %d atoms (Quality: %.0f%% - %s)",
		agentName, m.AtomCount, m.Score, m.Rating)
}

// CalculateAndSetQuality calculates quality for a research result.
func (r *ResearchResult) CalculateAndSetQuality(requestedTopics []string) QualityMetrics {
	return CalculateQualityMetrics(r.Atoms, requestedTopics)
}
