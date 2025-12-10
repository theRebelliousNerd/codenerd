// Package reviewer provides code review functionality with multi-shard orchestration.
// This file contains the two-pass creative enhancement pipeline.
package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// ExecuteCreativeEnhancement runs the full enhancement pipeline (Steps 8-12).
// This is triggered when --andEnhance flag is passed to /review.
//
// Pipeline:
//   Step 8: First Pass - Initial creative analysis
//   Step 9: Vector Search - Historical inspiration
//   Step 10: Self-Interrogation - Requirements clarification
//   Step 11: Second Pass - Enhanced creative synthesis
//   Step 12: Persist & Format
func (r *ReviewerShard) ExecuteCreativeEnhancement(
	ctx context.Context,
	fileContents map[string]string,
	holoCtx *HolographicContext,
	findings []ReviewFinding,
) (*EnhancementResult, error) {
	logging.Reviewer("Step 8: Starting creative enhancement pipeline")

	result := &EnhancementResult{}

	// ==========================================================================
	// STEP 8: FIRST PASS - Initial Creative Analysis
	// ==========================================================================
	logging.Reviewer("Step 8: First pass creative analysis")
	firstPass, err := r.firstPassCreative(ctx, fileContents, holoCtx, findings)
	if err != nil {
		return nil, fmt.Errorf("first pass failed: %w", err)
	}
	result.FirstPassCount = firstPass.TotalSuggestions()
	logging.ReviewerDebug("First pass generated %d suggestions", result.FirstPassCount)

	// ==========================================================================
	// STEP 9: VECTOR SEARCH - Historical Inspiration
	// ==========================================================================
	logging.Reviewer("Step 9: Searching for historical inspiration")
	inspiration, err := r.searchPastSuggestions(ctx, firstPass)
	if err != nil {
		logging.ReviewerDebug("Vector search failed, continuing without: %v", err)
	} else {
		result.VectorInspiration = inspiration
		logging.ReviewerDebug("Found %d past suggestions for inspiration", len(inspiration))
	}

	// ==========================================================================
	// STEP 10: SELF-INTERROGATION - Requirements Clarification
	// ==========================================================================
	logging.Reviewer("Step 10: Self-interrogation for refinement")
	selfQA, err := r.selfInterrogate(ctx, firstPass, inspiration, fileContents)
	if err != nil {
		logging.ReviewerDebug("Self-interrogation failed, continuing without: %v", err)
	} else {
		result.SelfQA = selfQA
		logging.ReviewerDebug("Generated %d self-Q&A pairs", len(selfQA))
	}

	// ==========================================================================
	// STEP 11: SECOND PASS - Enhanced Creative Synthesis
	// ==========================================================================
	logging.Reviewer("Step 11: Second pass with enhanced context")
	secondPass, err := r.secondPassCreative(ctx, fileContents, holoCtx, firstPass, inspiration, selfQA, findings)
	if err != nil {
		// Fall back to first pass results
		logging.ReviewerDebug("Second pass failed, using first pass: %v", err)
		return firstPass.ToResult(), nil
	}

	// Merge second pass into result
	result.FileSuggestions = secondPass.FileSuggestions
	result.ModuleSuggestions = secondPass.ModuleSuggestions
	result.SystemInsights = secondPass.SystemInsights
	result.FeatureIdeas = secondPass.FeatureIdeas
	result.SecondPassCount = secondPass.TotalSuggestions()

	if result.FirstPassCount > 0 {
		result.EnhancementRatio = float64(result.SecondPassCount) / float64(result.FirstPassCount)
	} else {
		result.EnhancementRatio = 1.0
	}

	logging.Reviewer("Step 12: Enhancement complete - %d total suggestions (%.1fx enhancement)",
		result.SecondPassCount, result.EnhancementRatio)

	return result, nil
}

// firstPassCreative performs initial creative analysis of the code.
func (r *ReviewerShard) firstPassCreative(
	ctx context.Context,
	fileContents map[string]string,
	holoCtx *HolographicContext,
	findings []ReviewFinding,
) (*CreativeFirstPass, error) {
	if r.llmClient == nil {
		return &CreativeFirstPass{}, nil
	}

	prompt := r.buildFirstPassPrompt(fileContents, holoCtx, findings)

	response, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	return r.parseFirstPassResponse(response)
}

// searchPastSuggestions queries knowledge DB for semantically similar suggestions.
func (r *ReviewerShard) searchPastSuggestions(
	ctx context.Context,
	firstPass *CreativeFirstPass,
) ([]PastSuggestion, error) {
	if r.virtualStore == nil {
		return nil, nil
	}

	db := r.virtualStore.GetLocalDB()
	if db == nil {
		return nil, nil
	}

	var results []PastSuggestion

	// Build query from first pass suggestions
	queryText := firstPass.BuildSearchQuery()
	if queryText == "" {
		return nil, nil
	}

	// Vector search with enhancement type filter
	vectors, err := db.VectorRecall(queryText+" enhancement_suggestion", 10)
	if err != nil {
		return nil, err
	}

	for _, v := range vectors {
		if v.Metadata == nil {
			continue
		}

		// Filter for enhancement suggestions only
		typeVal, ok := v.Metadata["type"].(string)
		if !ok || typeVal != "enhancement_suggestion" {
			continue
		}

		ps := PastSuggestion{
			Summary:    v.Content,
			Similarity: v.Similarity,
		}

		if id, ok := v.Metadata["suggestion_id"].(string); ok {
			ps.ID = id
		}
		if impl, ok := v.Metadata["implemented"].(bool); ok {
			ps.WasImplemented = impl
		}
		if rid, ok := v.Metadata["review_id"].(string); ok {
			ps.ReviewID = rid
		}

		results = append(results, ps)
	}

	return results, nil
}

// selfInterrogate uses Requirements Interrogator pattern for self-Q&A.
func (r *ReviewerShard) selfInterrogate(
	ctx context.Context,
	firstPass *CreativeFirstPass,
	inspiration []PastSuggestion,
	fileContents map[string]string,
) ([]SelfQuestion, error) {
	if r.llmClient == nil {
		return nil, nil
	}

	// Build prompt for generating clarifying questions
	questionPrompt := r.buildSelfQuestionPrompt(firstPass, inspiration)

	// Get questions from LLM
	questionsJSON, err := r.llmClient.Complete(ctx, questionPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate questions: %w", err)
	}

	questions := r.parseSelfQuestions(questionsJSON)
	if len(questions) == 0 {
		return nil, nil
	}

	// Now answer each question using code context
	var answered []SelfQuestion
	for _, q := range questions {
		answerPrompt := r.buildSelfAnswerPrompt(q.Question, fileContents, firstPass)
		answer, err := r.llmClient.Complete(ctx, answerPrompt)
		if err != nil {
			logging.ReviewerDebug("Failed to answer question: %v", err)
			continue
		}

		answered = append(answered, SelfQuestion{
			Question: q.Question,
			Answer:   strings.TrimSpace(answer),
			Insight:  extractInsight(q.Question, answer),
		})
	}

	return answered, nil
}

// secondPassCreative performs enhanced analysis with all gathered context.
func (r *ReviewerShard) secondPassCreative(
	ctx context.Context,
	fileContents map[string]string,
	holoCtx *HolographicContext,
	firstPass *CreativeFirstPass,
	inspiration []PastSuggestion,
	selfQA []SelfQuestion,
	findings []ReviewFinding,
) (*CreativeFirstPass, error) {
	if r.llmClient == nil {
		return firstPass, nil
	}

	prompt := r.buildSecondPassPrompt(fileContents, holoCtx, firstPass, inspiration, selfQA, findings)

	response, err := r.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	return r.parseFirstPassResponse(response) // Same format as first pass
}

// parseFirstPassResponse parses the LLM response into CreativeFirstPass.
func (r *ReviewerShard) parseFirstPassResponse(response string) (*CreativeFirstPass, error) {
	result := &CreativeFirstPass{}

	// Try to parse as JSON first
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")

	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		if err := json.Unmarshal([]byte(jsonStr), result); err == nil {
			return result, nil
		}
	}

	// Fall back to structured text parsing
	result = r.parseStructuredResponse(response)
	return result, nil
}

// parseStructuredResponse parses non-JSON structured text response.
func (r *ReviewerShard) parseStructuredResponse(response string) *CreativeFirstPass {
	result := &CreativeFirstPass{}

	lines := strings.Split(response, "\n")
	currentSection := ""
	var currentItem map[string]string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect section headers
		lower := strings.ToLower(line)
		if strings.Contains(lower, "file") && strings.Contains(lower, "suggestion") {
			currentSection = "file"
			continue
		} else if strings.Contains(lower, "module") && strings.Contains(lower, "suggestion") {
			currentSection = "module"
			continue
		} else if strings.Contains(lower, "system") && strings.Contains(lower, "insight") {
			currentSection = "system"
			continue
		} else if strings.Contains(lower, "feature") && strings.Contains(lower, "idea") {
			currentSection = "feature"
			continue
		}

		// Parse items
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			// Save previous item
			if currentItem != nil {
				r.saveItem(result, currentSection, currentItem)
			}
			currentItem = map[string]string{
				"title": strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "),
			}
		} else if currentItem != nil {
			// Additional details for current item
			if strings.HasPrefix(lower, "file:") {
				currentItem["file"] = strings.TrimSpace(line[5:])
			} else if strings.HasPrefix(lower, "category:") {
				currentItem["category"] = strings.TrimSpace(line[9:])
			} else if strings.HasPrefix(lower, "description:") {
				currentItem["description"] = strings.TrimSpace(line[12:])
			} else if strings.HasPrefix(lower, "effort:") {
				currentItem["effort"] = strings.TrimSpace(line[7:])
			} else if strings.HasPrefix(lower, "impact:") {
				currentItem["impact"] = strings.TrimSpace(line[7:])
			} else if strings.HasPrefix(lower, "package:") {
				currentItem["package"] = strings.TrimSpace(line[8:])
			} else if strings.HasPrefix(lower, "complexity:") {
				currentItem["complexity"] = strings.TrimSpace(line[11:])
			} else if strings.HasPrefix(lower, "rationale:") {
				currentItem["rationale"] = strings.TrimSpace(line[10:])
			} else if currentItem["description"] == "" {
				currentItem["description"] = line
			}
		}
	}

	// Save last item
	if currentItem != nil {
		r.saveItem(result, currentSection, currentItem)
	}

	return result
}

// saveItem saves a parsed item to the appropriate list.
func (r *ReviewerShard) saveItem(result *CreativeFirstPass, section string, item map[string]string) {
	switch section {
	case "file":
		result.FileSuggestions = append(result.FileSuggestions, FileSuggestion{
			File:        item["file"],
			Category:    item["category"],
			Title:       item["title"],
			Description: item["description"],
			Effort:      item["effort"],
			Confidence:  0.7,
		})
	case "module":
		result.ModuleSuggestions = append(result.ModuleSuggestions, ModuleSuggestion{
			Package:     item["package"],
			Category:    item["category"],
			Title:       item["title"],
			Description: item["description"],
			Effort:      item["effort"],
			Confidence:  0.7,
		})
	case "system":
		result.SystemInsights = append(result.SystemInsights, SystemInsight{
			Category:    item["category"],
			Title:       item["title"],
			Description: item["description"],
			Impact:      item["impact"],
			Confidence:  0.7,
		})
	case "feature":
		result.FeatureIdeas = append(result.FeatureIdeas, FeatureIdea{
			Title:       item["title"],
			Description: item["description"],
			Rationale:   item["rationale"],
			Complexity:  item["complexity"],
			Confidence:  0.7,
		})
	}
}

// parseSelfQuestions parses self-interrogation questions from LLM response.
func (r *ReviewerShard) parseSelfQuestions(response string) []SelfQuestion {
	var questions []SelfQuestion

	// Try JSON parsing first
	jsonStart := strings.Index(response, "[")
	jsonEnd := strings.LastIndex(response, "]")

	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := response[jsonStart : jsonEnd+1]
		var parsed []map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
			for _, p := range parsed {
				if q, ok := p["question"]; ok && q != "" {
					questions = append(questions, SelfQuestion{Question: q})
				}
			}
			return questions
		}
	}

	// Fall back to line-based parsing
	lines := strings.Split(response, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for question patterns
		if strings.HasSuffix(line, "?") {
			// Remove bullet points, numbers, etc.
			q := strings.TrimPrefix(line, "- ")
			q = strings.TrimPrefix(q, "* ")
			q = strings.TrimPrefix(q, "Q: ")
			if len(q) > 1 {
				// Remove number prefix like "1. " or "1) "
				for i, c := range q {
					if c == '.' || c == ')' {
						if i < 3 {
							q = strings.TrimSpace(q[i+1:])
							break
						}
					}
					if !('0' <= c && c <= '9') {
						break
					}
				}
				questions = append(questions, SelfQuestion{Question: q})
			}
		}
	}

	return questions
}

// extractInsight generates an insight from a Q&A pair.
func extractInsight(question, answer string) string {
	// Simple heuristic: extract key conclusion from answer
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return ""
	}

	// Look for conclusion indicators
	indicators := []string{
		"therefore",
		"this means",
		"in conclusion",
		"the key insight is",
		"importantly",
		"this suggests",
	}

	lower := strings.ToLower(answer)
	for _, ind := range indicators {
		if idx := strings.Index(lower, ind); idx >= 0 {
			// Extract sentence containing the indicator
			end := strings.IndexAny(answer[idx:], ".!?")
			if end > 0 {
				return strings.TrimSpace(answer[idx : idx+end+1])
			}
			return strings.TrimSpace(answer[idx:])
		}
	}

	// Fall back to first sentence if short enough
	if end := strings.IndexAny(answer, ".!?"); end > 0 && end < 200 {
		return strings.TrimSpace(answer[:end+1])
	}

	// Truncate if too long
	if len(answer) > 150 {
		return answer[:147] + "..."
	}

	return answer
}

// PersistEnhancements stores suggestions for future vector search.
func (r *ReviewerShard) PersistEnhancements(
	ctx context.Context,
	result *EnhancementResult,
	reviewID string,
) error {
	if r.virtualStore == nil {
		return nil
	}

	db := r.virtualStore.GetLocalDB()
	if db == nil {
		return nil
	}

	timestamp := time.Now().Unix()

	// Store file suggestions
	for i, fs := range result.FileSuggestions {
		content := fmt.Sprintf("[%s] %s: %s", fs.Category, fs.Title, fs.Description)
		metadata := map[string]interface{}{
			"type":          "enhancement_suggestion",
			"suggestion_id": fmt.Sprintf("%s-file-%d", reviewID, i),
			"level":         "file",
			"category":      fs.Category,
			"file":          fs.File,
			"effort":        fs.Effort,
			"implemented":   false,
			"review_id":     reviewID,
			"timestamp":     timestamp,
		}

		if err := db.StoreVector(content, metadata); err != nil {
			logging.ReviewerDebug("Failed to store file suggestion: %v", err)
		}
	}

	// Store module suggestions
	for i, ms := range result.ModuleSuggestions {
		content := fmt.Sprintf("[%s] %s: %s", ms.Category, ms.Title, ms.Description)
		metadata := map[string]interface{}{
			"type":          "enhancement_suggestion",
			"suggestion_id": fmt.Sprintf("%s-module-%d", reviewID, i),
			"level":         "module",
			"category":      ms.Category,
			"package":       ms.Package,
			"effort":        ms.Effort,
			"implemented":   false,
			"review_id":     reviewID,
			"timestamp":     timestamp,
		}

		if err := db.StoreVector(content, metadata); err != nil {
			logging.ReviewerDebug("Failed to store module suggestion: %v", err)
		}
	}

	// Store system insights
	for i, si := range result.SystemInsights {
		content := fmt.Sprintf("[%s] %s: %s", si.Category, si.Title, si.Description)
		metadata := map[string]interface{}{
			"type":          "enhancement_suggestion",
			"suggestion_id": fmt.Sprintf("%s-system-%d", reviewID, i),
			"level":         "system",
			"category":      si.Category,
			"impact":        si.Impact,
			"implemented":   false,
			"review_id":     reviewID,
			"timestamp":     timestamp,
		}

		if err := db.StoreVector(content, metadata); err != nil {
			logging.ReviewerDebug("Failed to store system insight: %v", err)
		}
	}

	// Store feature ideas
	for i, fi := range result.FeatureIdeas {
		content := fmt.Sprintf("[feature] %s: %s", fi.Title, fi.Description)
		metadata := map[string]interface{}{
			"type":          "enhancement_suggestion",
			"suggestion_id": fmt.Sprintf("%s-feature-%d", reviewID, i),
			"level":         "feature",
			"complexity":    fi.Complexity,
			"implemented":   false,
			"review_id":     reviewID,
			"timestamp":     timestamp,
		}

		if err := db.StoreVector(content, metadata); err != nil {
			logging.ReviewerDebug("Failed to store feature idea: %v", err)
		}
	}

	// Store enhancement metadata as cold storage fact
	if db != nil {
		factArgs := []interface{}{
			reviewID,
			result.TotalSuggestions(),
			len(result.FileSuggestions),
			len(result.ModuleSuggestions),
			len(result.SystemInsights),
			len(result.FeatureIdeas),
			result.EnhancementRatio,
			timestamp,
		}
		if err := db.StoreFact("enhancement_result", factArgs, "review", 50); err != nil {
			logging.ReviewerDebug("Failed to store enhancement fact: %v", err)
		}
	}

	logging.ReviewerDebug("Persisted %d enhancement suggestions to knowledge DB", result.TotalSuggestions())
	return nil
}
