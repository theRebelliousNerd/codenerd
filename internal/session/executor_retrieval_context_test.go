package session

import (
	"context"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
)

func TestDelegatedTaskTextReachesRetrievalAndCacheIdentity(t *testing.T) {
	var queries, hashes []string
	e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{},
		&MockJITCompiler{CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			queries = append(queries, cc.SemanticQuery)
			hashes = append(hashes, cc.Hash())
			return &prompt.CompilationResult{Prompt: "test"}, nil
		}}, &MockConfigFactory{}, &MockTransducer{})
	for _, task := range []string{"kernel indexing measured observation", "Z12 action utility calibration"} {
		_, err := e.ProcessWithIntent(t.Context(), task, &perception.Intent{Verb: "/symbolicefficiencyexpert", Category: "/query"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(queries) != 2 || queries[0] != "kernel indexing measured observation" || queries[1] != "Z12 action utility calibration" {
		t.Fatalf("delegation lost task query: %v", queries)
	}
	if hashes[0] == hashes[1] {
		t.Fatal("different tasks reused one retrieval cache identity")
	}
}
