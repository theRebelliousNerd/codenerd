package embedding

import (
	"strings"
)

// =============================================================================
// INTELLIGENT TASK TYPE SELECTION
// =============================================================================

// ContentType represents the type of content being embedded.
type ContentType string

const (
	ContentTypeCode             ContentType = "code"              // Source code
	ContentTypeDocumentation    ContentType = "documentation"     // Technical docs
	ContentTypeConversation     ContentType = "conversation"      // Chat messages
	ContentTypeKnowledgeAtom    ContentType = "knowledge_atom"    // Extracted knowledge
	ContentTypeQuery            ContentType = "query"             // User queries
	ContentTypeFact             ContentType = "fact"              // Logical facts
	ContentTypeQuestion         ContentType = "question"          // Questions
	ContentTypeAnswer           ContentType = "answer"            // Answers
	ContentTypeClassification   ContentType = "classification"    // For classification
	ContentTypeClustering       ContentType = "clustering"        // For grouping
)

// SelectTaskType intelligently selects the optimal GenAI task type based on content.
// This ensures embeddings are optimized for their specific use case.
func SelectTaskType(contentType ContentType, isQuery bool) string {
	switch contentType {
	case ContentTypeCode:
		if isQuery {
			return "CODE_RETRIEVAL_QUERY" // Searching for code
		}
		return "RETRIEVAL_DOCUMENT" // Indexing code

	case ContentTypeQuery:
		return "RETRIEVAL_QUERY" // General search queries

	case ContentTypeQuestion:
		return "QUESTION_ANSWERING" // QA system queries

	case ContentTypeAnswer, ContentTypeDocumentation:
		return "RETRIEVAL_DOCUMENT" // Documents to be retrieved

	case ContentTypeFact:
		return "FACT_VERIFICATION" // For fact checking

	case ContentTypeClassification:
		return "CLASSIFICATION" // For categorization

	case ContentTypeClustering:
		return "CLUSTERING" // For grouping similar items

	case ContentTypeConversation, ContentTypeKnowledgeAtom:
		return "SEMANTIC_SIMILARITY" // General semantic matching

	default:
		return "SEMANTIC_SIMILARITY" // Safe default
	}
}

// DetectContentType attempts to auto-detect content type from text and metadata.
func DetectContentType(text string, metadata map[string]interface{}) ContentType {
	text = strings.ToLower(text)

	// Check metadata first (most reliable)
	if meta, ok := metadata["content_type"].(string); ok {
		return ContentType(meta)
	}

	// Check metadata type field
	if metaType, ok := metadata["type"].(string); ok {
		switch metaType {
		case "user_input", "query":
			return ContentTypeQuery
		case "code", "source_code":
			return ContentTypeCode
		case "documentation", "docs":
			return ContentTypeDocumentation
		case "knowledge_atom", "fact":
			return ContentTypeKnowledgeAtom
		}
	}

	// Auto-detect from content
	// Code indicators
	codeIndicators := []string{
		"func ", "function ", "class ", "def ", "import ", "package ",
		"const ", "var ", "let ", "interface ", "struct ", "type ",
		"{", "}", "=>", "->", "//", "/*", "*/", "public ", "private ",
	}
	codeScore := 0
	for _, indicator := range codeIndicators {
		if strings.Contains(text, indicator) {
			codeScore++
		}
	}
	if codeScore >= 3 {
		return ContentTypeCode
	}

	// Question indicators
	if strings.HasPrefix(text, "what ") || strings.HasPrefix(text, "how ") ||
		strings.HasPrefix(text, "why ") || strings.HasPrefix(text, "when ") ||
		strings.HasPrefix(text, "where ") || strings.HasSuffix(text, "?") {
		return ContentTypeQuestion
	}

	// Conversation indicators (short, informal)
	if len(text) < 100 && (strings.Contains(text, "please") || strings.Contains(text, "can you") || strings.Contains(text, "i want")) {
		return ContentTypeConversation
	}

	// Documentation indicators
	docIndicators := []string{"# ", "## ", "### ", "/**", "* @param", "* @return", "readme", "documentation"}
	for _, indicator := range docIndicators {
		if strings.Contains(text, indicator) {
			return ContentTypeDocumentation
		}
	}

	// Default to conversation for natural language
	return ContentTypeConversation
}

// GetOptimalTaskType combines detection and selection for convenience.
func GetOptimalTaskType(text string, metadata map[string]interface{}, isQuery bool) string {
	contentType := DetectContentType(text, metadata)
	return SelectTaskType(contentType, isQuery)
}
