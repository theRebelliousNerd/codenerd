package embedding

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// INTELLIGENT TASK TYPE SELECTION
// =============================================================================

// ContentType represents the type of content being embedded.
type ContentType string

const (
	ContentTypeCode           ContentType = "code"           // Source code
	ContentTypeDocumentation  ContentType = "documentation"  // Technical docs
	ContentTypeConversation   ContentType = "conversation"   // Chat messages
	ContentTypeKnowledgeAtom  ContentType = "knowledge_atom" // Extracted knowledge
	ContentTypePromptAtom     ContentType = "prompt_atom"    // Prompt atoms for JIT compilation
	ContentTypeQuery          ContentType = "query"          // User queries
	ContentTypeFact           ContentType = "fact"           // Logical facts
	ContentTypeQuestion       ContentType = "question"       // Questions
	ContentTypeAnswer         ContentType = "answer"         // Answers
	ContentTypeClassification ContentType = "classification" // For classification
	ContentTypeClustering     ContentType = "clustering"     // For grouping
)

func normalizeTaskType(taskType string) string {
	return strings.ToUpper(strings.TrimSpace(taskType))
}

// knownTaskTypes are the Gemini embedding task types (the set
// config.EmbeddingConfig.TaskType documents, plus the API's unspecified value).
var knownTaskTypes = map[string]bool{
	"SEMANTIC_SIMILARITY":   true,
	"CLASSIFICATION":        true,
	"CLUSTERING":            true,
	"RETRIEVAL_DOCUMENT":    true,
	"RETRIEVAL_QUERY":       true,
	"CODE_RETRIEVAL_QUERY":  true,
	"QUESTION_ANSWERING":    true,
	"FACT_VERIFICATION":     true,
	"TASK_TYPE_UNSPECIFIED": true,
}

// checkTaskType refuses a task type the API does not define. An unknown one
// used to reach the API untouched and fail server-side, on every call, as a
// 400 that did not say which knob was wrong.
func checkTaskType(taskType string) error {
	if taskType == "" || knownTaskTypes[taskType] {
		return nil
	}
	known := make([]string, 0, len(knownTaskTypes))
	for k := range knownTaskTypes {
		known = append(known, k)
	}
	sort.Strings(known)
	return fmt.Errorf("unknown embedding task type %q (embedding.task_type, or a caller's task type); known: %s",
		taskType, strings.Join(known, ", "))
}

// knownContentTypes are the ContentType values SelectTaskType understands.
var knownContentTypes = map[ContentType]bool{
	ContentTypeCode: true, ContentTypeDocumentation: true, ContentTypeConversation: true,
	ContentTypeKnowledgeAtom: true, ContentTypePromptAtom: true, ContentTypeQuery: true,
	ContentTypeFact: true, ContentTypeQuestion: true, ContentTypeAnswer: true,
	ContentTypeClassification: true, ContentTypeClustering: true,
}

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

	case ContentTypeKnowledgeAtom, ContentTypePromptAtom:
		taskType = "RETRIEVAL_DOCUMENT" // Documents to be retrieved via semantic search

	case ContentTypeConversation:
		taskType = "SEMANTIC_SIMILARITY" // General semantic matching

	default:
		taskType = "SEMANTIC_SIMILARITY" // Safe default
		logging.EmbeddingDebug("SelectTaskType: unknown content_type=%s, defaulting to SEMANTIC_SIMILARITY", contentType)
	}

	taskType = normalizeTaskType(taskType)
	logging.EmbeddingDebug("SelectTaskType: selected task_type=%s", taskType)
	return taskType
}

// DetectContentType attempts to auto-detect content type from text and metadata.
func DetectContentType(text string, metadata map[string]any) ContentType {
	logging.EmbeddingDebug("DetectContentType: analyzing text (length=%d chars), metadata_keys=%d", len(text), len(metadata))

	originalText := text
	text = strings.ToLower(text)

	// Check metadata first (most reliable) -- when it names a content type.
	// It was returned verbatim, so a misspelled or differently-cased kind
	// ("Code", "docs") became an unknown ContentType and silently selected
	// SEMANTIC_SIMILARITY; the text was never looked at.
	if meta, ok := metadata["content_type"].(string); ok {
		logging.EmbeddingDebug("DetectContentType: found explicit content_type in metadata: %s", meta)
		if ct := ContentType(strings.ToLower(strings.TrimSpace(meta))); knownContentTypes[ct] {
			return ct
		}
		logging.Get(logging.CategoryEmbedding).Warn(
			"DetectContentType: metadata content_type %q is not a known content type; detecting from the type field and the text instead", meta)
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
		case "prompt_atom":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypePromptAtom")
			return ContentTypePromptAtom
		case "conversation", "chat":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeConversation")
			return ContentTypeConversation
		case "question":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeQuestion")
			return ContentTypeQuestion
		case "answer":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeAnswer")
			return ContentTypeAnswer
		case "classification":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeClassification")
			return ContentTypeClassification
		case "clustering":
			logging.EmbeddingDebug("DetectContentType: metadata type matched -> ContentTypeClustering")
			return ContentTypeClustering
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
func GetOptimalTaskType(text string, metadata map[string]any, isQuery bool) string {
	logging.EmbeddingDebug("GetOptimalTaskType: starting auto-detection for text (length=%d), is_query=%v", len(text), isQuery)

	contentType := DetectContentType(text, metadata)
	if isQuery {
		switch contentType {
		case ContentTypeCode, ContentTypeClassification, ContentTypeClustering:
		default:
			contentType = ContentTypeQuery
		}
	}
	taskType := SelectTaskType(contentType, isQuery)

	logging.Embedding("GetOptimalTaskType: detected content_type=%s -> task_type=%s", contentType, taskType)
	return taskType
}
