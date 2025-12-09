// Package reviewer provides code review functionality with multi-shard orchestration.
// This file contains knowledge base query helpers for specialist integration.
package reviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// =============================================================================
// KNOWLEDGE BASE QUERY HELPERS
// =============================================================================
// Functions for loading and querying specialist knowledge bases during reviews.

// RetrievedKnowledge represents knowledge retrieved from a specialist's KB
type RetrievedKnowledge struct {
	Content    string                 // The knowledge content
	Concept    string                 // Category (pattern, anti-pattern, best_practice, etc.)
	Source     string                 // Where this knowledge came from
	Confidence float64                // Relevance score (0.0-1.0)
	Metadata   map[string]interface{} // Additional metadata
}

// LoadAndQueryKnowledgeBase loads a specialist's KB and retrieves relevant atoms.
// It queries both vector store (semantic) and knowledge graph (relational).
func LoadAndQueryKnowledgeBase(ctx context.Context, kbPath string, files []string) ([]RetrievedKnowledge, error) {
	if kbPath == "" {
		return nil, fmt.Errorf("knowledge base path is empty")
	}

	// Check if KB exists
	if _, err := os.Stat(kbPath); os.IsNotExist(err) {
		logging.Store("Knowledge base not found at: %s", kbPath)
		return nil, nil // Not an error, just no KB available
	}

	db, err := store.NewLocalStore(kbPath)
	if err != nil {
		logging.Get(logging.CategoryStore).Error("Failed to open knowledge base %s: %v", kbPath, err)
		return nil, fmt.Errorf("failed to open knowledge base: %w", err)
	}
	defer db.Close()

	// Build semantic query from file contents and patterns
	query := buildSemanticQuery(files)
	if query == "" {
		return nil, nil
	}

	var results []RetrievedKnowledge

	// Query vector store for semantic matches
	vectors, err := db.VectorRecall(query, 15)
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Vector recall failed: %v", err)
	} else {
		for _, v := range vectors {
			concept := ""
			if v.Metadata != nil {
				if c, ok := v.Metadata["concept"].(string); ok {
					concept = c
				}
			}
			results = append(results, RetrievedKnowledge{
				Content:    v.Content,
				Concept:    concept,
				Source:     "vector_store",
				Confidence: 0.8, // Default confidence for vector matches
				Metadata:   v.Metadata,
			})
		}
	}

	// Query knowledge atoms by common concepts
	concepts := []string{"pattern", "anti_pattern", "best_practice", "code_example", "overview"}
	for _, concept := range concepts {
		atoms, err := db.GetKnowledgeAtoms(concept)
		if err != nil {
			continue
		}
		for _, atom := range atoms {
			results = append(results, RetrievedKnowledge{
				Content:    atom.Content,
				Concept:    atom.Concept,
				Source:     "knowledge_atoms",
				Confidence: atom.Confidence,
			})
		}
	}

	// Query knowledge graph for related patterns
	// Look for links related to detected technologies
	for _, file := range files {
		ext := filepath.Ext(file)
		tech := extToTechnology(ext)
		if tech == "" {
			continue
		}

		links, err := db.QueryLinks(tech, "outgoing")
		if err != nil {
			continue
		}

		for _, link := range links {
			if link.Relation == "has_pattern" || link.Relation == "has_anti_pattern" || link.Relation == "best_practice" {
				results = append(results, RetrievedKnowledge{
					Content:    link.EntityB,
					Concept:    link.Relation,
					Source:     "knowledge_graph",
					Confidence: link.Weight,
					Metadata:   link.Metadata,
				})
			}
		}
	}

	// Deduplicate by content
	seen := make(map[string]bool)
	var deduped []RetrievedKnowledge
	for _, r := range results {
		key := r.Content[:min(100, len(r.Content))]
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, r)
		}
	}

	// Limit to top 20 most relevant
	if len(deduped) > 20 {
		deduped = deduped[:20]
	}

	logging.Store("Retrieved %d knowledge atoms from %s", len(deduped), kbPath)
	return deduped, nil
}

// buildSemanticQuery constructs a semantic search query from files
func buildSemanticQuery(files []string) string {
	var parts []string

	for _, file := range files {
		// Add file name patterns
		base := filepath.Base(file)
		ext := filepath.Ext(file)
		parts = append(parts, base)

		// Add technology keywords based on extension
		tech := extToTechnology(ext)
		if tech != "" {
			parts = append(parts, tech)
		}

		// Read first 500 chars of file for context keywords
		if content, err := os.ReadFile(file); err == nil {
			sample := string(content)
			if len(sample) > 500 {
				sample = sample[:500]
			}
			// Extract key identifiers (simplistic approach)
			keywords := extractKeywords(sample)
			parts = append(parts, keywords...)
		}
	}

	// Deduplicate and join
	seen := make(map[string]bool)
	var unique []string
	for _, p := range parts {
		lower := strings.ToLower(p)
		if !seen[lower] && len(p) > 2 {
			seen[lower] = true
			unique = append(unique, p)
		}
	}

	// Limit query length
	if len(unique) > 20 {
		unique = unique[:20]
	}

	return strings.Join(unique, " ")
}

// extToTechnology maps file extensions to technology names
func extToTechnology(ext string) string {
	mapping := map[string]string{
		".go":   "go golang",
		".mg":   "mangle datalog logic",
		".py":   "python",
		".js":   "javascript",
		".ts":   "typescript",
		".tsx":  "react typescript",
		".jsx":  "react javascript",
		".rs":   "rust",
		".java": "java",
		".sql":  "sql database",
		".html": "html web",
		".css":  "css styling",
	}
	return mapping[ext]
}

// extractKeywords extracts meaningful keywords from code content
func extractKeywords(content string) []string {
	var keywords []string

	// Look for common patterns
	patterns := []string{
		"func ", "type ", "struct ", "interface ", // Go
		"Decl ", ":-", // Mangle
		"def ", "class ", // Python
		"function ", "const ", "import ", // JS/TS
	}

	for _, pattern := range patterns {
		if strings.Contains(content, pattern) {
			keywords = append(keywords, strings.TrimSpace(pattern))
		}
	}

	// Extract import paths (Go specific)
	if strings.Contains(content, "import") {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, `"`) && strings.Contains(line, "/") {
				// Extract package name from import
				start := strings.LastIndex(line, "/")
				end := strings.LastIndex(line, `"`)
				if start > 0 && end > start {
					pkg := line[start+1 : end]
					if len(pkg) > 0 && len(pkg) < 30 {
						keywords = append(keywords, pkg)
					}
				}
			}
		}
	}

	return keywords
}

// FormatKnowledgeContext formats retrieved knowledge for prompt injection
func FormatKnowledgeContext(knowledge []RetrievedKnowledge) string {
	if len(knowledge) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Domain Knowledge\n\n")

	// Group by concept
	byCategory := make(map[string][]RetrievedKnowledge)
	for _, k := range knowledge {
		cat := k.Concept
		if cat == "" {
			cat = "general"
		}
		byCategory[cat] = append(byCategory[cat], k)
	}

	// Format each category
	categoryOrder := []string{"best_practice", "pattern", "anti_pattern", "code_example", "overview", "general"}
	for _, cat := range categoryOrder {
		items, ok := byCategory[cat]
		if !ok || len(items) == 0 {
			continue
		}

		// Pretty category name
		prettyName := strings.ReplaceAll(cat, "_", " ")
		prettyName = strings.ToUpper(prettyName[:1]) + prettyName[1:]
		sb.WriteString(fmt.Sprintf("### %s\n\n", prettyName))

		for _, item := range items {
			// Truncate long content
			content := item.Content
			if len(content) > 500 {
				content = content[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("- %s\n", content))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// min returns the minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
