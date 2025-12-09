// Package reviewer provides code review functionality with multi-shard orchestration.
// This file contains persistence and export functionality for review results.
package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// =============================================================================
// REVIEW PERSISTENCE
// =============================================================================
// Functions for storing reviews in knowledge DB and exporting to markdown.

// PersistedReview represents a complete multi-shard review for storage
type PersistedReview struct {
	ID               string                       `json:"id"`
	Timestamp        time.Time                    `json:"timestamp"`
	Target           string                       `json:"target"`
	Files            []string                     `json:"files"`
	Participants     []string                     `json:"participants"`
	IsComplete       bool                         `json:"is_complete"`
	IncompleteReason []string                     `json:"incomplete_reason,omitempty"`
	Summary          string                       `json:"summary"`
	FindingsByShard  map[string][]ParsedFinding   `json:"findings_by_shard"`
	HolisticInsights []string                     `json:"holistic_insights"`
	TotalFindings    int                          `json:"total_findings"`
	Duration         time.Duration                `json:"duration"`
}

// PersistReview stores a review in the knowledge database for future querying.
// It stores both vector entries (for semantic search) and cold storage facts.
func PersistReview(ctx context.Context, db *store.LocalStore, review *PersistedReview) error {
	if db == nil {
		logging.Store("No database provided for review persistence")
		return nil
	}

	logging.Store("Persisting review %s to knowledge DB", review.ID)

	// Store summary as vector for semantic search on past reviews
	metadata := map[string]interface{}{
		"type":         "multi_shard_review",
		"review_id":    review.ID,
		"target":       review.Target,
		"timestamp":    review.Timestamp.Unix(),
		"complete":     review.IsComplete,
		"participants": strings.Join(review.Participants, ","),
		"total":        review.TotalFindings,
	}

	if err := db.StoreVector(review.Summary, metadata); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to store review vector: %v", err)
	}

	// Store in cold storage as structured fact
	reviewArgs := []interface{}{
		review.ID,
		review.Target,
		strings.Join(review.Participants, ","),
		review.IsComplete,
		review.TotalFindings,
		review.Timestamp.Format(time.RFC3339),
	}
	if err := db.StoreFact("multi_shard_review", reviewArgs, "review", 100); err != nil {
		logging.Get(logging.CategoryStore).Warn("Failed to store review fact: %v", err)
	}

	// Store each finding as a fact for detailed querying
	for shard, findings := range review.FindingsByShard {
		for _, f := range findings {
			findingArgs := []interface{}{
				review.ID,
				shard,
				f.File,
				f.Line,
				f.Severity,
				f.Message,
			}
			priority := severityToPriority(f.Severity)
			if err := db.StoreFact("review_finding", findingArgs, "review", priority); err != nil {
				logging.Get(logging.CategoryStore).Warn("Failed to store finding fact: %v", err)
			}
		}
	}

	// Store holistic insights
	for i, insight := range review.HolisticInsights {
		insightArgs := []interface{}{
			review.ID,
			i,
			insight,
		}
		if err := db.StoreFact("review_insight", insightArgs, "review", 50); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to store insight fact: %v", err)
		}
	}

	// Store in knowledge graph for relationship queries
	// Link review to participants
	for _, participant := range review.Participants {
		if err := db.StoreLink(review.ID, "reviewed_by", participant, 1.0, nil); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to store participant link: %v", err)
		}
	}

	// Link review to files
	for _, file := range review.Files {
		if err := db.StoreLink(review.ID, "reviewed_file", file, 1.0, nil); err != nil {
			logging.Get(logging.CategoryStore).Warn("Failed to store file link: %v", err)
		}
	}

	logging.Store("Review %s persisted with %d findings", review.ID, review.TotalFindings)
	return nil
}

// severityToPriority converts severity string to priority int
func severityToPriority(severity string) int {
	switch strings.ToLower(severity) {
	case "critical":
		return 100
	case "error":
		return 80
	case "warning":
		return 60
	case "info":
		return 40
	default:
		return 20
	}
}

// ExportReviewToMarkdown writes a review to a markdown file.
// Returns the path to the created file.
func ExportReviewToMarkdown(review *PersistedReview, outputDir string) (string, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("review_%s_%s.md",
		sanitizeFilename(review.Target),
		review.Timestamp.Format("20060102_150405"))
	outputPath := filepath.Join(outputDir, filename)

	// Build markdown content
	content := buildMarkdownContent(review)

	// Write file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write markdown file: %w", err)
	}

	logging.Store("Exported review to: %s", outputPath)
	return outputPath, nil
}

// buildMarkdownContent creates the markdown content for a review
func buildMarkdownContent(review *PersistedReview) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Code Review: %s\n\n", review.Target))
	sb.WriteString(fmt.Sprintf("**Date**: %s\n", review.Timestamp.Format("2006-01-02 15:04")))

	status := "Complete"
	if !review.IsComplete {
		status = "Partial"
	}
	sb.WriteString(fmt.Sprintf("**Status**: %s\n", status))
	sb.WriteString(fmt.Sprintf("**Duration**: %s\n", review.Duration.Round(time.Second)))
	sb.WriteString(fmt.Sprintf("**Participants**: %s\n", strings.Join(review.Participants, ", ")))
	sb.WriteString(fmt.Sprintf("**Total Findings**: %d\n\n", review.TotalFindings))

	// Incomplete reasons if any
	if !review.IsComplete && len(review.IncompleteReason) > 0 {
		sb.WriteString("### Incomplete Reasons\n\n")
		for _, reason := range review.IncompleteReason {
			sb.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("## Summary\n\n")
	if review.Summary != "" {
		sb.WriteString(review.Summary + "\n\n")
	} else {
		sb.WriteString("_No summary available._\n\n")
	}

	// Holistic Insights
	if len(review.HolisticInsights) > 0 {
		sb.WriteString("## Holistic Insights\n\n")
		sb.WriteString("Cross-shard analysis revealed:\n\n")
		for _, insight := range review.HolisticInsights {
			sb.WriteString(fmt.Sprintf("- %s\n", insight))
		}
		sb.WriteString("\n")
	}

	// Findings by shard
	sb.WriteString("## Findings by Specialist\n\n")

	// Sort shards: ReviewerShard first, then alphabetically
	shardOrder := []string{"reviewer", "ReviewerShard"}
	for shard := range review.FindingsByShard {
		found := false
		for _, s := range shardOrder {
			if strings.EqualFold(shard, s) {
				found = true
				break
			}
		}
		if !found {
			shardOrder = append(shardOrder, shard)
		}
	}

	for _, shard := range shardOrder {
		findings, ok := review.FindingsByShard[shard]
		if !ok {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s (%d findings)\n\n", shard, len(findings)))

		if len(findings) == 0 {
			sb.WriteString("_No issues found._\n\n")
			continue
		}

		// Group by severity
		bySeverity := map[string][]ParsedFinding{
			"critical": {},
			"error":    {},
			"warning":  {},
			"info":     {},
		}

		for _, f := range findings {
			sev := strings.ToLower(f.Severity)
			if _, ok := bySeverity[sev]; !ok {
				sev = "info"
			}
			bySeverity[sev] = append(bySeverity[sev], f)
		}

		// Output in severity order
		severityOrder := []string{"critical", "error", "warning", "info"}
		severityEmoji := map[string]string{
			"critical": "CRITICAL",
			"error":    "ERROR",
			"warning":  "WARNING",
			"info":     "INFO",
		}

		for _, sev := range severityOrder {
			items := bySeverity[sev]
			if len(items) == 0 {
				continue
			}

			for _, f := range items {
				location := f.File
				if f.Line > 0 {
					location = fmt.Sprintf("%s:%d", f.File, f.Line)
				}
				sb.WriteString(fmt.Sprintf("- **[%s]** `%s` - %s\n",
					severityEmoji[sev], location, f.Message))
				if f.Recommendation != "" {
					sb.WriteString(fmt.Sprintf("  - _Recommendation_: %s\n", f.Recommendation))
				}
			}
		}

		sb.WriteString("\n")
	}

	// Files reviewed
	sb.WriteString("## Files Reviewed\n\n")
	for _, file := range review.Files {
		sb.WriteString(fmt.Sprintf("- `%s`\n", file))
	}
	sb.WriteString("\n")

	// Footer
	sb.WriteString("---\n\n")
	sb.WriteString("_Generated by codeNERD Multi-Shard Review_\n")

	return sb.String()
}

// sanitizeFilename removes characters that aren't safe for filenames
func sanitizeFilename(s string) string {
	// Replace path separators and other unsafe chars
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := s
	for _, u := range unsafe {
		result = strings.ReplaceAll(result, u, "_")
	}
	// Truncate if too long
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// QueryPastReviews searches for past reviews matching a query.
// Uses semantic search on review summaries.
func QueryPastReviews(ctx context.Context, db *store.LocalStore, query string, limit int) ([]PersistedReview, error) {
	if db == nil {
		return nil, fmt.Errorf("no database provided")
	}

	if limit <= 0 {
		limit = 10
	}

	// Semantic search on review vectors
	vectors, err := db.VectorRecall(query+" multi_shard_review", limit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	var reviews []PersistedReview
	for _, v := range vectors {
		if v.Metadata == nil {
			continue
		}

		// Check if this is a review
		typeVal, ok := v.Metadata["type"].(string)
		if !ok || typeVal != "multi_shard_review" {
			continue
		}

		// Reconstruct review from metadata
		review := PersistedReview{
			Summary: v.Content,
		}

		if id, ok := v.Metadata["review_id"].(string); ok {
			review.ID = id
		}
		if target, ok := v.Metadata["target"].(string); ok {
			review.Target = target
		}
		if ts, ok := v.Metadata["timestamp"].(float64); ok {
			review.Timestamp = time.Unix(int64(ts), 0)
		}
		if complete, ok := v.Metadata["complete"].(bool); ok {
			review.IsComplete = complete
		}
		if participants, ok := v.Metadata["participants"].(string); ok {
			review.Participants = strings.Split(participants, ",")
		}
		if total, ok := v.Metadata["total"].(float64); ok {
			review.TotalFindings = int(total)
		}

		reviews = append(reviews, review)
	}

	return reviews, nil
}

// LoadReviewDetails loads full review details including findings from cold storage.
func LoadReviewDetails(ctx context.Context, db *store.LocalStore, reviewID string) (*PersistedReview, error) {
	if db == nil {
		return nil, fmt.Errorf("no database provided")
	}

	// Load main review fact
	facts, err := db.LoadFacts("multi_shard_review")
	if err != nil {
		return nil, fmt.Errorf("failed to load review facts: %w", err)
	}

	var review *PersistedReview
	for _, f := range facts {
		if len(f.Args) >= 1 {
			if id, ok := f.Args[0].(string); ok && id == reviewID {
				review = &PersistedReview{
					ID:              reviewID,
					FindingsByShard: make(map[string][]ParsedFinding),
				}
				if len(f.Args) >= 2 {
					review.Target, _ = f.Args[1].(string)
				}
				if len(f.Args) >= 3 {
					if p, ok := f.Args[2].(string); ok {
						review.Participants = strings.Split(p, ",")
					}
				}
				if len(f.Args) >= 4 {
					review.IsComplete, _ = f.Args[3].(bool)
				}
				if len(f.Args) >= 5 {
					if total, ok := f.Args[4].(float64); ok {
						review.TotalFindings = int(total)
					}
				}
				if len(f.Args) >= 6 {
					if ts, ok := f.Args[5].(string); ok {
						review.Timestamp, _ = time.Parse(time.RFC3339, ts)
					}
				}
				break
			}
		}
	}

	if review == nil {
		return nil, fmt.Errorf("review not found: %s", reviewID)
	}

	// Load findings
	findingFacts, err := db.LoadFacts("review_finding")
	if err == nil {
		for _, f := range findingFacts {
			if len(f.Args) >= 6 {
				id, _ := f.Args[0].(string)
				if id != reviewID {
					continue
				}
				shard, _ := f.Args[1].(string)
				finding := ParsedFinding{
					ShardSource: shard,
				}
				finding.File, _ = f.Args[2].(string)
				if line, ok := f.Args[3].(float64); ok {
					finding.Line = int(line)
				}
				finding.Severity, _ = f.Args[4].(string)
				finding.Message, _ = f.Args[5].(string)

				review.FindingsByShard[shard] = append(review.FindingsByShard[shard], finding)
			}
		}
	}

	// Load insights
	insightFacts, err := db.LoadFacts("review_insight")
	if err == nil {
		for _, f := range insightFacts {
			if len(f.Args) >= 3 {
				id, _ := f.Args[0].(string)
				if id != reviewID {
					continue
				}
				insight, _ := f.Args[2].(string)
				review.HolisticInsights = append(review.HolisticInsights, insight)
			}
		}
	}

	return review, nil
}

// ReviewToJSON exports a review to JSON format
func ReviewToJSON(review *PersistedReview) (string, error) {
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal review: %w", err)
	}
	return string(data), nil
}
