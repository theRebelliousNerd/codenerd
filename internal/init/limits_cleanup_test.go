// limits_cleanup_test.go pins the LIMITS CLEANUP behavior changes in package
// init: document content and stored atoms reach the LLM whole instead of
// being cut at hardcoded char bounds.
package init

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/store"
)

// The relevance judge must see the whole document: a 500-char preview
// misclassified documents whose architecture signal sits past byte 500.
func TestAnalyzeDocBatch_DocumentContentReachesJudgementWhole(t *testing.T) {
	workspace := t.TempDir()
	marker := "ARCHITECTURE-SIGNAL-PAST-BYTE-500"
	content := strings.Repeat("filler prose with no signal. ", 40) + marker
	if len(content) <= 500 {
		t.Fatalf("fixture too short to exercise the old 500-char preview: %d", len(content))
	}
	if strings.Index(content, marker) <= 500 {
		t.Fatalf("marker sits at byte %d, must sit past byte 500", strings.Index(content, marker))
	}
	llm := &scriptedLLM{
		relevance: `[{"index":0,"relevant":true,"reason":"vision doc"}]`,
	}
	ini := &Initializer{config: InitConfig{Workspace: workspace, LLMClient: llm}}
	docs := []DocumentInfo{{
		Path: "DESIGN.md", Title: "Design", Content: content, Size: len(content), Priority: 1,
	}}
	got := ini.analyzeDocBatch(context.Background(), docs)
	if len(got) != 1 || !got[0].IsRelevant {
		t.Fatalf("analyzeDocBatch = %+v, want the one relevant doc", got)
	}
	llm.mu.Lock()
	defer llm.mu.Unlock()
	if len(llm.prompts) != 1 {
		t.Fatalf("got %d prompts, want 1", len(llm.prompts))
	}
	if !strings.Contains(llm.prompts[0], content) {
		t.Error("relevance prompt dropped document content past byte 500")
	}
	if !strings.Contains(llm.prompts[0], marker) {
		t.Error("relevance prompt lost the architecture signal past byte 500")
	}
}

// Stored atoms feed the synthesis model whole: a 200-char cut truncated the
// context for every long insight.
func TestBuildAtomsSummary_LongAtomContentReturnedWhole(t *testing.T) {
	long := strings.Repeat("insight detail ", 30) + "TAIL-PAST-200"
	if len(long) <= 200 {
		t.Fatalf("fixture too short to exercise the old 200-char cut: %d", len(long))
	}
	atoms := []store.KnowledgeAtom{
		{Concept: "doc/DESIGN.md/architecture/layering", Content: long, Confidence: 0.9},
		{Concept: "doc/README.md/philosophy/simplicity", Content: "short", Confidence: 0.8},
	}
	summary, categories := buildAtomsSummary(atoms)
	if !strings.Contains(summary, long) {
		t.Error("summary cut atom content at 200 chars")
	}
	if !strings.Contains(summary, "TAIL-PAST-200") {
		t.Error("summary lost the tail past char 200")
	}
	if len(categories) != 2 {
		t.Errorf("got %d categories, want 2", len(categories))
	}
}
