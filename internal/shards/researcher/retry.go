// Package researcher - Retry logic and graceful degradation.
// This file implements exponential backoff and fallback strategies per D3.
package researcher

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration (doubles each retry)
	MaxBackoff     time.Duration // Maximum backoff duration
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     8 * time.Second,
	}
}

// RetryableFunc is a function that can be retried.
type RetryableFunc func(ctx context.Context) ([]KnowledgeAtom, error)

// ErrMaxRetriesExceeded indicates all retry attempts failed.
var ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")

// WithRetry executes a function with exponential backoff retry.
// Returns the result on success, or error after all retries exhausted.
func WithRetry(ctx context.Context, config RetryConfig, operation string, fn RetryableFunc) ([]KnowledgeAtom, error) {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Execute the function
		atoms, err := fn(ctx)
		if err == nil {
			if attempt > 0 {
				logging.Researcher("Retry succeeded for %s on attempt %d", operation, attempt+1)
			}
			return atoms, nil
		}

		lastErr = err
		logging.Researcher("Attempt %d/%d for %s failed: %v", attempt+1, config.MaxRetries+1, operation, err)

		// Don't sleep after the last attempt
		if attempt < config.MaxRetries {
			backoff := calculateBackoff(config, attempt)
			logging.Researcher("Retrying %s in %v...", operation, backoff)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return nil, fmt.Errorf("%w for %s: %v", ErrMaxRetriesExceeded, operation, lastErr)
}

// calculateBackoff computes exponential backoff.
func calculateBackoff(config RetryConfig, attempt int) time.Duration {
	// Exponential backoff: initial * 2^attempt
	backoff := float64(config.InitialBackoff) * math.Pow(2, float64(attempt))

	// Cap at max backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	return time.Duration(backoff)
}

// FallbackStrategy represents a fallback approach when primary research fails.
type FallbackStrategy int

const (
	FallbackNone FallbackStrategy = iota
	FallbackFewerTopics
	FallbackSimplerNames
	FallbackCachedResults
	FallbackMinimalKnowledge
)

// GracefulResearchResult contains research results with fallback info.
type GracefulResearchResult struct {
	Atoms           []KnowledgeAtom
	FallbackUsed    FallbackStrategy
	FallbackReason  string
	OriginalTopics  []string
	EffectiveTopics []string
	AttemptsMade    int
}

// ResearchWithGracefulDegradation performs research with automatic fallback.
// Implements D3 graceful degradation strategy:
// 1. Try full research with retries
// 2. If that fails, try with fewer topics
// 3. If that fails, try with simpler topic names
// 4. If nothing works, return minimal fallback knowledge
func (r *ResearcherShard) ResearchWithGracefulDegradation(ctx context.Context, topics []string) (*GracefulResearchResult, error) {
	result := &GracefulResearchResult{
		OriginalTopics:  topics,
		EffectiveTopics: topics,
		FallbackUsed:    FallbackNone,
	}

	retryConfig := DefaultRetryConfig()

	// Strategy 1: Full research with retries
	atoms, err := WithRetry(ctx, retryConfig, "full research", func(ctx context.Context) ([]KnowledgeAtom, error) {
		res, err := r.ResearchTopicsParallel(ctx, topics)
		if err != nil {
			return nil, err
		}
		if len(res.Atoms) == 0 {
			return nil, fmt.Errorf("no atoms returned")
		}
		return res.Atoms, nil
	})

	if err == nil && len(atoms) > 0 {
		result.Atoms = atoms
		result.AttemptsMade = 1
		return result, nil
	}

	logging.Researcher("Full research failed, trying with fewer topics...")

	// Strategy 2: Fewer topics (top 3 most important)
	if len(topics) > 3 {
		reducedTopics := topics[:3]
		result.EffectiveTopics = reducedTopics
		result.FallbackUsed = FallbackFewerTopics
		result.FallbackReason = fmt.Sprintf("reduced from %d to %d topics", len(topics), len(reducedTopics))

		atoms, err = WithRetry(ctx, retryConfig, "reduced topics", func(ctx context.Context) ([]KnowledgeAtom, error) {
			res, err := r.ResearchTopicsParallel(ctx, reducedTopics)
			if err != nil {
				return nil, err
			}
			if len(res.Atoms) == 0 {
				return nil, fmt.Errorf("no atoms returned")
			}
			return res.Atoms, nil
		})

		if err == nil && len(atoms) > 0 {
			result.Atoms = atoms
			result.AttemptsMade = 2
			return result, nil
		}
	}

	logging.Researcher("Reduced topics failed, trying simpler names...")

	// Strategy 3: Simpler topic names (remove qualifiers)
	simplifiedTopics := simplifyTopicNames(topics)
	if len(simplifiedTopics) > 0 {
		result.EffectiveTopics = simplifiedTopics
		result.FallbackUsed = FallbackSimplerNames
		result.FallbackReason = "simplified topic names"

		atoms, err = WithRetry(ctx, retryConfig, "simplified topics", func(ctx context.Context) ([]KnowledgeAtom, error) {
			res, err := r.ResearchTopicsParallel(ctx, simplifiedTopics)
			if err != nil {
				return nil, err
			}
			if len(res.Atoms) == 0 {
				return nil, fmt.Errorf("no atoms returned")
			}
			return res.Atoms, nil
		})

		if err == nil && len(atoms) > 0 {
			result.Atoms = atoms
			result.AttemptsMade = 3
			return result, nil
		}
	}

	logging.Researcher("Simplified topics failed, generating minimal fallback knowledge...")

	// Strategy 4: Generate minimal fallback knowledge (never return empty)
	fallbackAtoms := generateMinimalFallbackKnowledge(topics)
	result.Atoms = fallbackAtoms
	result.FallbackUsed = FallbackMinimalKnowledge
	result.FallbackReason = "generated minimal fallback knowledge"
	result.AttemptsMade = 4

	return result, nil
}

// ResearchWithExistingKnowledgeAndGracefulDegradation combines concept-aware research
// with graceful degradation. It first analyzes existing knowledge to skip redundant
// queries, then applies graceful degradation strategies for topics that need research.
func (r *ResearcherShard) ResearchWithExistingKnowledgeAndGracefulDegradation(
	ctx context.Context,
	topics []string,
	existingAtoms []KnowledgeAtom,
) (*GracefulResearchResult, error) {

	result := &GracefulResearchResult{
		OriginalTopics:  topics,
		EffectiveTopics: topics,
		FallbackUsed:    FallbackNone,
	}

	// First, analyze coverage to filter topics
	existing := NewExistingKnowledge(existingAtoms)
	topicsNeedingResearch, skippedTopics, _ := FilterTopicsWithCoverage(topics, existing)

	logging.Researcher("[ConceptAware+Graceful] %d topics need research, %d skipped (sufficient coverage)",
		len(topicsNeedingResearch), len(skippedTopics))

	// If no topics need research, return empty result (success!)
	if len(topicsNeedingResearch) == 0 {
		result.Atoms = make([]KnowledgeAtom, 0)
		result.AttemptsMade = 0
		result.FallbackUsed = FallbackNone
		result.FallbackReason = fmt.Sprintf("all %d topics have sufficient coverage", len(topics))
		logging.Researcher("[ConceptAware+Graceful] All topics already covered - no API calls needed")
		return result, nil
	}

	// Update effective topics to only the ones needing research
	result.EffectiveTopics = topicsNeedingResearch

	// Now use graceful degradation for the topics that actually need research
	retryConfig := DefaultRetryConfig()

	// Strategy 1: Full research with retries (concept-aware already filtered)
	atoms, err := WithRetry(ctx, retryConfig, "concept-aware research", func(ctx context.Context) ([]KnowledgeAtom, error) {
		res, err := r.ResearchTopicsParallel(ctx, topicsNeedingResearch)
		if err != nil {
			return nil, err
		}
		if len(res.Atoms) == 0 {
			return nil, fmt.Errorf("no atoms returned")
		}
		return res.Atoms, nil
	})

	if err == nil && len(atoms) > 0 {
		result.Atoms = atoms
		result.AttemptsMade = 1
		return result, nil
	}

	logging.Researcher("Concept-aware research failed, trying with fewer topics...")

	// Strategy 2: Fewer topics (top 3 most important)
	if len(topicsNeedingResearch) > 3 {
		reducedTopics := topicsNeedingResearch[:3]
		result.EffectiveTopics = reducedTopics
		result.FallbackUsed = FallbackFewerTopics
		result.FallbackReason = fmt.Sprintf("reduced from %d to %d topics", len(topicsNeedingResearch), len(reducedTopics))

		atoms, err = WithRetry(ctx, retryConfig, "reduced topics", func(ctx context.Context) ([]KnowledgeAtom, error) {
			res, err := r.ResearchTopicsParallel(ctx, reducedTopics)
			if err != nil {
				return nil, err
			}
			if len(res.Atoms) == 0 {
				return nil, fmt.Errorf("no atoms returned")
			}
			return res.Atoms, nil
		})

		if err == nil && len(atoms) > 0 {
			result.Atoms = atoms
			result.AttemptsMade = 2
			return result, nil
		}
	}

	logging.Researcher("Reduced topics failed, trying simpler names...")

	// Strategy 3: Simpler topic names (remove qualifiers)
	simplifiedTopics := simplifyTopicNames(topicsNeedingResearch)
	if len(simplifiedTopics) > 0 {
		result.EffectiveTopics = simplifiedTopics
		result.FallbackUsed = FallbackSimplerNames
		result.FallbackReason = "simplified topic names"

		atoms, err = WithRetry(ctx, retryConfig, "simplified topics", func(ctx context.Context) ([]KnowledgeAtom, error) {
			res, err := r.ResearchTopicsParallel(ctx, simplifiedTopics)
			if err != nil {
				return nil, err
			}
			if len(res.Atoms) == 0 {
				return nil, fmt.Errorf("no atoms returned")
			}
			return res.Atoms, nil
		})

		if err == nil && len(atoms) > 0 {
			result.Atoms = atoms
			result.AttemptsMade = 3
			return result, nil
		}
	}

	logging.Researcher("Simplified topics failed, generating minimal fallback knowledge...")

	// Strategy 4: Generate minimal fallback knowledge (never return empty)
	fallbackAtoms := generateMinimalFallbackKnowledge(topicsNeedingResearch)
	result.Atoms = fallbackAtoms
	result.FallbackUsed = FallbackMinimalKnowledge
	result.FallbackReason = "generated minimal fallback knowledge"
	result.AttemptsMade = 4

	return result, nil
}

// simplifyTopicNames removes qualifiers and version numbers from topic names.
func simplifyTopicNames(topics []string) []string {
	simplified := make([]string, 0, len(topics))
	seen := make(map[string]bool)

	for _, topic := range topics {
		// Remove common qualifiers
		simple := topic
		qualifiers := []string{
			" expert", " advanced", " best practices", " patterns",
			" tutorial", " guide", " documentation", " docs",
			" 18", " 17", " 16", " 1.21", " 1.22", " 1.23",
			" v2", " v3", " v4",
		}

		for _, q := range qualifiers {
			simple = strings.TrimSuffix(simple, q)
			simple = strings.TrimSuffix(strings.ToLower(simple), strings.ToLower(q))
		}

		simple = strings.TrimSpace(simple)
		if simple != "" && !seen[simple] {
			seen[simple] = true
			simplified = append(simplified, simple)
		}
	}

	// Limit to 3 simplified topics
	if len(simplified) > 3 {
		simplified = simplified[:3]
	}

	return simplified
}

// generateMinimalFallbackKnowledge creates basic knowledge atoms when all else fails.
// This ensures we never create empty knowledge bases.
func generateMinimalFallbackKnowledge(topics []string) []KnowledgeAtom {
	atoms := make([]KnowledgeAtom, 0, len(topics)+1)

	// Add a meta-atom explaining the fallback
	atoms = append(atoms, KnowledgeAtom{
		SourceURL:   "internal://fallback",
		Title:       "Knowledge Base Initialization",
		Content:     fmt.Sprintf("This knowledge base was initialized with fallback content for topics: %s. External research sources were unavailable. The agent should rely on general knowledge and request clarification from the user when needed.", strings.Join(topics, ", ")),
		Concept:     "meta_fallback",
		Confidence:  0.5,
		ExtractedAt: time.Now(),
		Metadata: map[string]interface{}{
			"fallback":        true,
			"topics":          topics,
			"needs_hydration": true,
		},
	})

	// Add basic atoms for each topic
	for _, topic := range topics {
		atoms = append(atoms, KnowledgeAtom{
			SourceURL:   "internal://fallback/" + strings.ReplaceAll(topic, " ", "_"),
			Title:       topic + " (Placeholder)",
			Content:     fmt.Sprintf("Placeholder knowledge for '%s'. This agent specializes in this area but detailed documentation was unavailable during initialization. The agent should use general knowledge and best practices.", topic),
			Concept:     "placeholder",
			Confidence:  0.4,
			ExtractedAt: time.Now(),
			Metadata: map[string]interface{}{
				"fallback": true,
				"topic":    topic,
			},
		})
	}

	return atoms
}
