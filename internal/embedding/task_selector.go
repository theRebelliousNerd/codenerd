package embedding

import (
	"strings"

	"codenerd/internal/logging"
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
	logging.EmbeddingDebug("SelectTaskType: content_type=%s, is_query=%v", contentType, isQuery)

	var taskType string

	switch contentType {
	case ContentTypeCode:
		if isQuery {
			taskType = "CODE_RETRIEVAL_QUERY" // Searching for code
		} else {
			taskType = "RETRIEVAL_DOCUMENT" // Indexing code
		}

	case ContentTypeQuery:
		taskType = "RETRIEVAL_QUERY" // General search queries

	case ContentTypeQuestion:
		taskType = "QUESTION_ANSWERING" // QA system queries

	case ContentTypeAnswer, ContentTypeDocumentation:
		taskType = "RETRIEVAL_DOCUMENT" // Documents to be retrieved

	case ContentTypeFact:
		taskType = "FACT_VERIFICATION" // For fact checking

	case ContentTypeClassification:
		taskType = "CLASSIFICATION" // For categorization

	case ContentTypeClustering:
		taskType = "CLUSTERING" // For grouping similar items

	case ContentTypeConversation, ContentTypeKnowledgeAtom:
		taskType = "SEMANTIC_SIMILARITY" // General semantic matching

	default:
		taskType = "SEMANTIC_SIMILARITY" // Safe default
		logging.EmbeddingDebug("SelectTaskType: unknown content_type=%s, defaulting to SEMANTIC_SIMILARITY", contentType)
	}

	logging.EmbeddingDebug("SelectTaskType: selected task_type=%s", taskType)
	return taskType
}

// DetectContentType attempts to auto-detect content type from text and metadata.
func DetectContentType(text string, metadata map[string]interface{}) ContentType {
	logging.EmbeddingDebug("DetectContentType: analyzing text (length=%d chars), metadata_keys=%d", len(text), len(metadata))

	originalText := text
	text = strings.ToLower(text)

	// Check metadata first (most reliable)
	if meta, ok := metadata["content_type"].(string); ok {
		logging.EmbeddingDebug("DetectContentType: found explicit content_type in metadata: %s", meta)
		return ContentType(meta)
	}

	// Check metadata type field
	if metaType, ok := metadata["type"].(string); ok {
		logging.EmbeddingDebug("DetectContentType: found type field in metadata: %s", metaType)
		switch metaType {
		case "user_input", "query":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeQuery")
			return ContentTypeQuery
		case "code", "source_code":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeCode")
			return ContentTypeCode
		case "documentation", "docs":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeDocumentation")
			return ContentTypeDocumentation
		case "knowledge_atom", "fact":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeKnowledgeAtom")
			return ContentTypeKnowledgeAtom
		}
	}

	logging.EmbeddingDebug("DetectContentType: no metadata match, analyzing content heuristics")

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
	logging.EmbeddingDebug("DetectContentType: code_score=%d (threshold=3)", codeScore)
	if codeScore >= 3 {
		logging.EmbeddingDebug("DetectContentType: detected as code based on indicators")
		return ContentTypeCode
	}

	// Question indicators
	if strings.HasPrefix(text, "what ") || strings.HasPrefix(text, "how ") ||
		strings.HasPrefix(text, "why ") || strings.HasPrefix(text, "when ") ||
		strings.HasPrefix(text, "where ") || strings.HasSuffix(text, "?") {
		logging.EmbeddingDebug("DetectContentType: detected as question based on prefix/suffix")
		return ContentTypeQuestion
	}

	// Conversation indicators (short, informal)
	if len(originalText) < 100 && (strings.Contains(text, "please") || strings.Contains(text, "can you") || strings.Contains(text, "i want")) {
		logging.EmbeddingDebug("DetectContentType: detected as conversation (short + informal markers)")
		return ContentTypeConversation
	}

	// Documentation indicators
	docIndicators := []string{"# ", "## ", "### ", "/**", "* @param", "* @return", "readme", "documentation"}
	for _, indicator := range docIndicators {
		if strings.Contains(text, indicator) {
			logging.EmbeddingDebug("DetectContentType: detected as documentation based on indicator: %s", indicator)
			return ContentTypeDocumentation
		}
	}

	// Default to conversation for natural language
	logging.EmbeddingDebug("DetectContentType: no specific pattern matched, defaulting to conversation")
	return ContentTypeConversation
}

// GetOptimalTaskType combines detection and selection for convenience.
func GetOptimalTaskType(text string, metadata map[string]interface{}, isQuery bool) string {
	logging.EmbeddingDebug("GetOptimalTaskType: starting auto-detection for text (length=%d), is_query=%v", len(text), isQuery)

	contentType := DetectContentType(text, metadata)
	taskType := SelectTaskType(contentType, isQuery)

	logging.Embedding("GetOptimalTaskType: detected content_type=%s -> task_type=%s", contentType, taskType)
	return taskType
}
