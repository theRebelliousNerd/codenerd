package embedding

import "testing"

func TestSelectTaskType(t *testing.T) {
	if got := SelectTaskType(ContentTypeCode, true); got != "CODE_RETRIEVAL_QUERY" {
		t.Fatalf("SelectTaskType(code, query)=%q, want CODE_RETRIEVAL_QUERY", got)
	}
	if got := SelectTaskType(ContentTypeCode, false); got != "RETRIEVAL_DOCUMENT" {
		t.Fatalf("SelectTaskType(code, doc)=%q, want RETRIEVAL_DOCUMENT", got)
	}
	if got := SelectTaskType(ContentTypeQuestion, true); got != "QUESTION_ANSWERING" {
		t.Fatalf("SelectTaskType(question)=%q, want QUESTION_ANSWERING", got)
	}
	if got := SelectTaskType(ContentTypeFact, false); got != "FACT_VERIFICATION" {
		t.Fatalf("SelectTaskType(fact)=%q, want FACT_VERIFICATION", got)
	}
}

func TestDetectContentType_MetadataWins(t *testing.T) {
	meta := map[string]interface{}{"content_type": "prompt_atom"}
	if got := DetectContentType("func main() {}", meta); got != ContentTypePromptAtom {
		t.Fatalf("DetectContentType(metadata content_type)=%q, want %q", got, ContentTypePromptAtom)
	}

	meta = map[string]interface{}{"type": "query"}
	if got := DetectContentType("how do I do x", meta); got != ContentTypeQuery {
		t.Fatalf("DetectContentType(metadata type=query)=%q, want %q", got, ContentTypeQuery)
	}
}

func TestDetectContentType_Heuristics(t *testing.T) {
	// Code score >= 3
	code := "package main\n\nfunc main() { /* hi */ }\n"
	if got := DetectContentType(code, map[string]interface{}{}); got != ContentTypeCode {
		t.Fatalf("DetectContentType(code)=%q, want %q", got, ContentTypeCode)
	}

	q := "how do I write a scanner?"
	if got := DetectContentType(q, map[string]interface{}{}); got != ContentTypeQuestion {
		t.Fatalf("DetectContentType(question)=%q, want %q", got, ContentTypeQuestion)
	}

	conv := "please help"
	if got := DetectContentType(conv, map[string]interface{}{}); got != ContentTypeConversation {
		t.Fatalf("DetectContentType(conversation)=%q, want %q", got, ContentTypeConversation)
	}

	doc := "## Title\n\nThis is documentation."
	if got := DetectContentType(doc, map[string]interface{}{}); got != ContentTypeDocumentation {
		t.Fatalf("DetectContentType(documentation)=%q, want %q", got, ContentTypeDocumentation)
	}
}

func TestGetOptimalTaskType(t *testing.T) {
	got := GetOptimalTaskType("package main\nfunc main() {}", map[string]interface{}{}, true)
	if got != "CODE_RETRIEVAL_QUERY" {
		t.Fatalf("GetOptimalTaskType(code query)=%q, want CODE_RETRIEVAL_QUERY", got)
	}
}
