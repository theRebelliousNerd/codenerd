package prompt

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/store"
)

// Retrieved knowledge used to enter every compile at full score, and the
// budget's second pass filled whatever window was left with it. It now has a
// share of the budget the kernel spends best-first: a row that does not fit
// is left out whole and named, with a handle that returns it verbatim.
func TestRetrievedKnowledgeIsAdmittedUnderItsShare(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	project, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = project.Close() })
	// Four rows of ~5000 tokens: the 8000-token share holds one.
	for i := range 4 {
		body := fmt.Sprintf("zorblatt calibration note %d. ", i) + strings.Repeat(fmt.Sprintf("zorblatt detail %d line\n", i), 900)
		if err := project.StoreKnowledgeAtom(fmt.Sprintf("notes/zorblatt-%d", i), body, 0.9, "test"); err != nil {
			t.Fatal(err)
		}
	}
	compiler, err := NewJITPromptCompiler(WithKernel(&realKernelQuerier{k: k}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	compiler.SetLocalDB(project)

	cc := NewCompilationContextWithBudget(1048576)
	cc.ShardID = "coder"
	cc.ShardType = "/coder"
	cc.SemanticQuery = "zorblatt calibration"
	res, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	admitted, tokens := 0, 0
	var pointer *PromptAtom
	for _, a := range res.IncludedAtoms {
		if a.RetrievedContext {
			admitted++
			tokens += EstimateTokens(a.Content)
		}
		if a.ID == knowledgeWithheldAtomID {
			pointer = a
		}
	}
	if admitted != 1 || tokens > 8000 {
		t.Fatalf("admitted %d retrieved rows (%d tokens), want 1 within the 8000-token share", admitted, tokens)
	}
	if pointer == nil {
		t.Fatal("withheld rows were dropped without a pointer")
	}
	handles := regexp.MustCompile(`obs:kn:[0-9a-f]+`).FindAllString(pointer.Content, -1)
	if len(handles) != 3 || !strings.Contains(pointer.Content, "3 retrieved knowledge row(s)") {
		t.Fatalf("pointer names %d handles, want 3:\n%s", len(handles), pointer.Content)
	}
	text, err := observation.SharedKnowledge().Read(handles[0], 0, 0)
	if err != nil || !strings.Contains(text, "zorblatt calibration note") || strings.Count(text, "\n") < 900 {
		t.Fatalf("withheld row was not returned whole: err=%v len=%d", err, len(text))
	}
	if !strings.Contains(res.Prompt, "[knowledge withheld]") {
		t.Fatal("the pointer did not reach the prompt")
	}
}

// Two rows cut from one source page whose texts overlap are one row; two
// different sections of the page are two.
func TestAppendKnowledgePromptAtoms_SamePageOverlapCountsOnce(t *testing.T) {
	section := "Query answers a pattern.\nBound arguments use the index.\nFree ones scan."
	rows := []store.KnowledgeAtom{
		{Concept: "web/mangle/section_3", Content: section, Confidence: 0.8},
		{Concept: "web/mangle/section_7", Content: section + "\nA trailing note.", Confidence: 0.8},
		{Concept: "web/mangle/section_9", Content: "Aggregation uses do and let.", Confidence: 0.8},
	}
	seen := map[string]struct{}{}
	got := appendKnowledgePromptAtoms(nil, seen, rows[:1], "", "expert:a", "a")
	got = appendKnowledgePromptAtoms(got, seen, rows[1:], "", "expert:b", "b")
	if len(got) != 2 {
		var concepts []string
		for _, a := range got {
			concepts = append(concepts, a.KnowledgeConcept)
		}
		t.Fatalf("kept %v, want section_3 and section_9", concepts)
	}
}
