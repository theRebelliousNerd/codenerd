package embedding

import "testing"

// =============================================================================
// normalizeTaskType Tests
// =============================================================================

func TestNormalizeTaskType_WhenLowercase_ShouldUppercase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"lowercase", "semantic_similarity", "SEMANTIC_SIMILARITY"},
		{"mixed case", "Retrieval_Query", "RETRIEVAL_QUERY"},
		{"already uppercase", "CLASSIFICATION", "CLASSIFICATION"},
		{"empty string", "", ""},
		{"with leading whitespace", "  retrieval_document", "RETRIEVAL_DOCUMENT"},
		{"with trailing whitespace", "clustering  ", "CLUSTERING"},
		{"with both whitespace", "  fact_verification  ", "FACT_VERIFICATION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTaskType(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeTaskType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// SelectTaskType – Exhaustive Branch Coverage
// =============================================================================

func TestSelectTaskType_WhenAllContentTypes_ShouldReturnExpected(t *testing.T) {
	tests := []struct {
		name        string
		contentType ContentType
		isQuery     bool
		expected    string
	}{
		// Code
		{"code query", ContentTypeCode, true, "CODE_RETRIEVAL_QUERY"},
		{"code document", ContentTypeCode, false, "RETRIEVAL_DOCUMENT"},

		// Query
		{"query true", ContentTypeQuery, true, "RETRIEVAL_QUERY"},
		{"query false", ContentTypeQuery, false, "RETRIEVAL_QUERY"},

		// Question
		{"question true", ContentTypeQuestion, true, "QUESTION_ANSWERING"},
		{"question false", ContentTypeQuestion, false, "QUESTION_ANSWERING"},

		// Answer
		{"answer true", ContentTypeAnswer, true, "RETRIEVAL_DOCUMENT"},
		{"answer false", ContentTypeAnswer, false, "RETRIEVAL_DOCUMENT"},

		// Documentation
		{"documentation true", ContentTypeDocumentation, true, "RETRIEVAL_DOCUMENT"},
		{"documentation false", ContentTypeDocumentation, false, "RETRIEVAL_DOCUMENT"},

		// Fact
		{"fact true", ContentTypeFact, true, "FACT_VERIFICATION"},
		{"fact false", ContentTypeFact, false, "FACT_VERIFICATION"},

		// Classification
		{"classification true", ContentTypeClassification, true, "CLASSIFICATION"},
		{"classification false", ContentTypeClassification, false, "CLASSIFICATION"},

		// Clustering
		{"clustering true", ContentTypeClustering, true, "CLUSTERING"},
		{"clustering false", ContentTypeClustering, false, "CLUSTERING"},

		// KnowledgeAtom
		{"knowledge_atom true", ContentTypeKnowledgeAtom, true, "RETRIEVAL_DOCUMENT"},
		{"knowledge_atom false", ContentTypeKnowledgeAtom, false, "RETRIEVAL_DOCUMENT"},

		// PromptAtom
		{"prompt_atom true", ContentTypePromptAtom, true, "RETRIEVAL_DOCUMENT"},
		{"prompt_atom false", ContentTypePromptAtom, false, "RETRIEVAL_DOCUMENT"},

		// Conversation
		{"conversation true", ContentTypeConversation, true, "SEMANTIC_SIMILARITY"},
		{"conversation false", ContentTypeConversation, false, "SEMANTIC_SIMILARITY"},

		// Unknown / default
		{"unknown type", ContentType("unknown"), false, "SEMANTIC_SIMILARITY"},
		{"empty type", ContentType(""), false, "SEMANTIC_SIMILARITY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectTaskType(tt.contentType, tt.isQuery)
			if got != tt.expected {
				t.Errorf("SelectTaskType(%q, %v) = %q, want %q",
					tt.contentType, tt.isQuery, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// DetectContentType – Exhaustive Metadata Branch Coverage
// =============================================================================

func TestDetectContentType_WhenMetadataTypeField_ShouldReturnCorrectType(t *testing.T) {
	tests := []struct {
		name     string
		metaType string
		expected ContentType
	}{
		{"user_input", "user_input", ContentTypeQuery},
		{"query", "query", ContentTypeQuery},
		{"code", "code", ContentTypeCode},
		{"source_code", "source_code", ContentTypeCode},
		{"documentation", "documentation", ContentTypeDocumentation},
		{"docs", "docs", ContentTypeDocumentation},
		{"knowledge_atom", "knowledge_atom", ContentTypeKnowledgeAtom},
		{"fact", "fact", ContentTypeKnowledgeAtom},
		{"prompt_atom", "prompt_atom", ContentTypePromptAtom},
		{"conversation", "conversation", ContentTypeConversation},
		{"chat", "chat", ContentTypeConversation},
		{"question", "question", ContentTypeQuestion},
		{"answer", "answer", ContentTypeAnswer},
		{"classification", "classification", ContentTypeClassification},
		{"clustering", "clustering", ContentTypeClustering},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := map[string]any{"type": tt.metaType}
			got := DetectContentType("some text", meta)
			if got != tt.expected {
				t.Errorf("DetectContentType(type=%q) = %q, want %q",
					tt.metaType, got, tt.expected)
			}
		})
	}
}

func TestDetectContentType_WhenMetadataTypeUnrecognized_ShouldFallThrough(t *testing.T) {
	meta := map[string]any{"type": "unknown_type_xyz"}
	got := DetectContentType("hello world", meta)
	// Should fall through to heuristic detection, not crash
	if got == "" {
		t.Error("DetectContentType should not return empty string")
	}
}

func TestDetectContentType_WhenNilMetadata_ShouldNotPanic(t *testing.T) {
	// nil metadata shouldn't crash
	got := DetectContentType("hello world", nil)
	if got == "" {
		t.Error("DetectContentType with nil metadata should return a content type")
	}
}

func TestDetectContentType_WhenEmptyMetadata_ShouldUseHeuristics(t *testing.T) {
	meta := map[string]any{}
	got := DetectContentType("what is the meaning of life?", meta)
	if got != ContentTypeQuestion {
		t.Errorf("DetectContentType(question text) = %q, want %q", got, ContentTypeQuestion)
	}
}

func TestDetectContentType_WhenContentTypeMetadata_ShouldOverrideHeuristics(t *testing.T) {
	// Even if the text looks like code, content_type metadata wins
	meta := map[string]any{"content_type": "conversation"}
	got := DetectContentType("func main() { package main }", meta)
	if got != ContentTypeConversation {
		t.Errorf("DetectContentType(content_type override) = %q, want %q", got, ContentTypeConversation)
	}
}

// =============================================================================
// DetectContentType – Heuristic Tests
// =============================================================================

func TestDetectContentType_WhenQuestionPrefixes_ShouldDetectQuestion(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"what prefix", "what is Go used for?"},
		{"how prefix", "how do I write a test"},
		{"why prefix", "why is this slow"},
		{"when prefix", "when should I use channels"},
		{"where prefix", "where is the config file"},
		{"question mark suffix", "is this correct?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectContentType(tt.text, map[string]any{})
			if got != ContentTypeQuestion {
				t.Errorf("DetectContentType(%q) = %q, want %q", tt.text, got, ContentTypeQuestion)
			}
		})
	}
}

func TestDetectContentType_WhenConversationMarkers_ShouldDetectConversation(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"please", "please help me"},
		{"can you", "can you do this"},
		{"i want", "i want to fix it"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectContentType(tt.text, map[string]any{})
			if got != ContentTypeConversation {
				t.Errorf("DetectContentType(%q) = %q, want %q", tt.text, got, ContentTypeConversation)
			}
		})
	}
}

func TestDetectContentType_WhenDocIndicators_ShouldDetectDocumentation(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"markdown h1", "# Title\nSome documentation content here that is long enough"},
		{"markdown h2", "## Section\nSome documentation content here that is long enough"},
		{"markdown h3", "### Subsection\nSome documentation content here that is long enough"},
		// Note: javadoc "/** ... */" triggers code indicators (/* and */) and code wins over doc
		{"readme mention", "readme file contains the full documentation for this project"},
		{"documentation mention", "this documentation explains how to use the API in detail"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectContentType(tt.text, map[string]any{})
			if got != ContentTypeDocumentation {
				t.Errorf("DetectContentType(%q) = %q, want %q", tt.text, got, ContentTypeDocumentation)
			}
		})
	}
}

func TestDetectContentType_WhenCodeScore3Plus_ShouldDetectCode(t *testing.T) {
	// Need at least 3 code indicators
	codeText := "package main\nimport \"fmt\"\nfunc main() { fmt.Println() }"
	got := DetectContentType(codeText, map[string]any{})
	if got != ContentTypeCode {
		t.Errorf("DetectContentType(code) = %q, want %q", got, ContentTypeCode)
	}
}

func TestDetectContentType_WhenCodeScoreLow_ShouldNotDetectCode(t *testing.T) {
	// Only 1 code indicator (import)
	text := "import something"
	got := DetectContentType(text, map[string]any{})
	// Should NOT be code (score < 3)
	if got == ContentTypeCode {
		t.Errorf("DetectContentType with low code score should not be code, got %q", got)
	}
}

func TestDetectContentType_WhenLongNaturalText_ShouldDefaultToConversation(t *testing.T) {
	// Long text without specific markers should default to conversation
	// Avoid words like "documentation" or markers like "# " that trigger other heuristics
	text := "this is a general statement about something with no particular markers or patterns that would identify it as anything specific really just plain text that goes on and on with no real purpose"
	got := DetectContentType(text, map[string]any{})
	if got != ContentTypeConversation {
		t.Errorf("DetectContentType(plain text) = %q, want %q", got, ContentTypeConversation)
	}
}

func TestDetectContentType_WhenConversationMarkersButLongText_ShouldFallThroughToDefault(t *testing.T) {
	// "please" in text > 100 chars should NOT match via the short-text conversation check,
	// but will still hit the default conversation fallback at the end.
	longText := "this is a very long text that contains the word please somewhere in it but because it is more than one hundred characters long it should not match the short conversation pattern check"
	got := DetectContentType(longText, map[string]any{})
	// Falls through conversation check (len > 100) but hits default conversation at end
	if got != ContentTypeConversation {
		t.Errorf("Long text should default to conversation fallback, got %q", got)
	}
}

// =============================================================================
// GetOptimalTaskType – Integration Tests
// =============================================================================

func TestGetOptimalTaskType_WhenIsQuery_ShouldOverrideContentType(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		isQuery  bool
		expected string
	}{
		{
			"plain text as query",
			"hello world just some text",
			true,
			"RETRIEVAL_QUERY",
		},
		{
			"plain text not query",
			"hello world just some text",
			false,
			"SEMANTIC_SIMILARITY",
		},
		{
			"code text as query",
			"package main\nfunc main() { var x int }",
			true,
			"CODE_RETRIEVAL_QUERY",
		},
		{
			"classification isQuery stays classification",
			"classify this",
			true,
			"RETRIEVAL_QUERY", // conversation overridden to query
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetOptimalTaskType(tt.text, map[string]any{}, tt.isQuery)
			if got != tt.expected {
				t.Errorf("GetOptimalTaskType(%q, isQuery=%v) = %q, want %q",
					tt.text, tt.isQuery, got, tt.expected)
			}
		})
	}
}

func TestGetOptimalTaskType_WhenMetadataProvided_ShouldUseMetadata(t *testing.T) {
	meta := map[string]any{"content_type": "fact"}
	got := GetOptimalTaskType("some text", meta, false)
	if got != "FACT_VERIFICATION" {
		t.Errorf("GetOptimalTaskType with fact metadata = %q, want FACT_VERIFICATION", got)
	}
}

func TestGetOptimalTaskType_WhenClassificationIsQuery_ShouldKeepClassification(t *testing.T) {
	meta := map[string]any{"content_type": "classification"}
	got := GetOptimalTaskType("categorize this item", meta, true)
	if got != "CLASSIFICATION" {
		t.Errorf("GetOptimalTaskType(classification, isQuery=true) = %q, want CLASSIFICATION", got)
	}
}

func TestGetOptimalTaskType_WhenClusteringIsQuery_ShouldKeepClustering(t *testing.T) {
	meta := map[string]any{"content_type": "clustering"}
	got := GetOptimalTaskType("group these items", meta, true)
	if got != "CLUSTERING" {
		t.Errorf("GetOptimalTaskType(clustering, isQuery=true) = %q, want CLUSTERING", got)
	}
}

func TestGetOptimalTaskType_WhenCodeIsQuery_ShouldKeepCodeRetrieval(t *testing.T) {
	meta := map[string]any{"content_type": "code"}
	got := GetOptimalTaskType("find this function", meta, true)
	if got != "CODE_RETRIEVAL_QUERY" {
		t.Errorf("GetOptimalTaskType(code, isQuery=true) = %q, want CODE_RETRIEVAL_QUERY", got)
	}
}

func TestGetOptimalTaskType_WhenNilMetadata_ShouldNotPanic(t *testing.T) {
	got := GetOptimalTaskType("test text", nil, false)
	if got == "" {
		t.Error("GetOptimalTaskType with nil metadata should return a task type")
	}
}

// =============================================================================
// ContentType Constants Tests
// =============================================================================

func TestContentType_Constants_ShouldBeDistinct(t *testing.T) {
	allTypes := []ContentType{
		ContentTypeCode,
		ContentTypeDocumentation,
		ContentTypeConversation,
		ContentTypeKnowledgeAtom,
		ContentTypePromptAtom,
		ContentTypeQuery,
		ContentTypeFact,
		ContentTypeQuestion,
		ContentTypeAnswer,
		ContentTypeClassification,
		ContentTypeClustering,
	}

	seen := make(map[ContentType]bool)
	for _, ct := range allTypes {
		if seen[ct] {
			t.Errorf("Duplicate ContentType: %q", ct)
		}
		seen[ct] = true

		if string(ct) == "" {
			t.Errorf("ContentType should not be empty string")
		}
	}

	if len(seen) != 11 {
		t.Errorf("Expected 11 distinct ContentTypes, got %d", len(seen))
	}
}
