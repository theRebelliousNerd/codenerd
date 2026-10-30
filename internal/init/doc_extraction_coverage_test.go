package init

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// A document extracted in two chunks, one of which failed, was recorded
// Stored at its hash, and the next refresh skipped the unchanged document:
// the failed chunk was never retried, even with the provider healthy again
// (audit document-extraction-coverage-resumption). A partial extraction is
// now its own status, which earns no unchanged-document reuse.
func TestDocumentRefresh_APartialExtractionIsRetriedNextRefresh(t *testing.T) {
	root := orientationFixture(t)
	var long strings.Builder
	long.WriteString("# Project\nMake repository decisions from evidence.\n")
	for long.Len() < 12000 {
		long.WriteString("Review repository changes against the recorded evidence before deciding.\n")
	}
	writeInitFile(t, root, "README.md", long.String())
	orientationFixtureGit(t, root, "add", "README.md")
	orientationFixtureGit(t, root, "commit", "-m", "long readme")

	setup := DefaultInitConfig(root)
	setup.Output, setup.Interactive, setup.SkipResearch, setup.SkipAgentCreate = nil, false, true, true
	setup.EmbeddingEngine = fixedInitEngine{}
	setup.LLMClient = &orientationClient{}
	first, err := NewInitializer(setup)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := first.Initialize(context.Background()); err != nil || !result.Success {
		t.Fatalf("Initialize: %v %v", err, result.Failures)
	}
	_ = first.Close()

	failSecondChunk := true
	reply := `[{"concept":"architecture/evidence","content":"Decisions come from repository evidence.","confidence":0.9}]`
	main := &identifiedClient{provider: "meta", model: "worker-model", reply: func(system, user string) (string, error) {
		if !strings.Contains(system, "Extract key knowledge atoms") {
			return "", nil
		}
		if failSecondChunk && strings.Contains(user, "README.md (chunk 2/") {
			return "", errors.New("provider unavailable")
		}
		return reply, nil
	}}
	readmeRequests := func() int {
		n := 0
		for _, c := range main.requests() {
			if strings.Contains(c, "Document: README.md (chunk") {
				n++
			}
		}
		return n
	}
	db := strategicStore(t)
	refresh := func() *DocIngestionState {
		t.Helper()
		ini, err := NewInitializer(InitConfig{Workspace: root, LLMClient: main, EmbeddingEngine: fixedInitEngine{}})
		if err != nil {
			t.Fatal(err)
		}
		defer ini.Close()
		docs, _, err := ini.OrientationDocuments(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		state, err := ini.ProcessDocumentsWithTracking(context.Background(), docs, db, nil, false)
		if err != nil {
			t.Fatalf("ProcessDocumentsWithTracking: %v", err)
		}
		return state
	}

	state := refresh()
	entry := state.Documents["README.md"]
	if entry == nil {
		t.Skip("orientation did not admit README.md in this fixture")
	}
	if got := readmeRequests(); got < 2 {
		t.Fatalf("precondition: README.md was extracted in %d chunk request(s), want at least 2", got)
	}
	if entry.Status != DocStatusPartial || state.TotalPartial != 1 {
		t.Fatalf("README.md after a failed chunk = %s (partial count %d), want %s", entry.Status, state.TotalPartial, DocStatusPartial)
	}

	failSecondChunk = false
	before := readmeRequests()
	state = refresh()
	if readmeRequests() == before {
		t.Fatal("the unchanged, partially extracted README.md was skipped: its failed chunk was never retried")
	}
	if got := state.Documents["README.md"].Status; got != DocStatusStored {
		t.Fatalf("README.md after a healthy retry = %s, want %s", got, DocStatusStored)
	}

	// Complete coverage earns reuse as before.
	before = readmeRequests()
	refresh()
	if readmeRequests() != before {
		t.Fatal("a completely extracted, unchanged README.md was extracted again")
	}
}
