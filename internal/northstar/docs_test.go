package northstar

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// INGESTED DOCS
// =============================================================================

func TestIngestDoc_WhenReIngestingSamePath_ShouldReplaceNotDuplicate(t *testing.T) {
	store, _ := newBridgeStore(t)

	if err := store.IngestDoc(&IngestedDoc{Path: "docs/vision.md", Content: "first", Relevance: 0.4}); err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if err := store.IngestDoc(&IngestedDoc{Path: "docs/vision.md", Content: "second", Relevance: 0.9}); err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	docs, err := store.ListIngestedDocs(10)
	if err != nil {
		t.Fatalf("ListIngestedDocs: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("doc count = %d, want 1 (re-ingest must replace, not accumulate)", len(docs))
	}
	if docs[0].Content != "second" || docs[0].Relevance != 0.9 {
		t.Errorf("doc = %+v, want the second ingest to win", docs[0])
	}
}

func TestIngestDoc_WhenPathEmpty_ShouldReject(t *testing.T) {
	store, _ := newBridgeStore(t)
	if err := store.IngestDoc(&IngestedDoc{Content: "orphan"}); err == nil {
		t.Error("accepted a document with no path; its identity would be a fresh row on every ingest")
	}
}

func TestListIngestedDocs_ShouldOrderByRelevance(t *testing.T) {
	store, _ := newBridgeStore(t)
	for _, d := range []*IngestedDoc{
		{Path: "a.md", Content: "a", Relevance: 0.2},
		{Path: "b.md", Content: "b", Relevance: 0.8},
		{Path: "c.md", Content: "c", Relevance: 0.5},
	} {
		if err := store.IngestDoc(d); err != nil {
			t.Fatalf("ingest %s: %v", d.Path, err)
		}
	}
	docs, err := store.ListIngestedDocs(10)
	if err != nil {
		t.Fatalf("ListIngestedDocs: %v", err)
	}
	if len(docs) != 3 || docs[0].Path != "b.md" || docs[2].Path != "a.md" {
		t.Errorf("order = %v, want b, c, a", []string{docs[0].Path, docs[1].Path, docs[2].Path})
	}
}

func TestDeleteIngestedDoc_ShouldRemoveByPath(t *testing.T) {
	store, _ := newBridgeStore(t)
	if err := store.IngestDoc(&IngestedDoc{Path: "gone.md", Content: "x"}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if err := store.DeleteIngestedDoc("gone.md"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	docs, _ := store.ListIngestedDocs(10)
	if len(docs) != 0 {
		t.Errorf("doc survived deletion: %+v", docs)
	}
}

// =============================================================================
// EMBEDDING RELEVANCE
// =============================================================================

// keywordEmbedder is a deterministic stand-in for a real embedding backend:
// each of a fixed vocabulary of terms gets its own dimension.
type keywordEmbedder struct{ vocab []string }

func (e *keywordEmbedder) Embed(text string) ([]float32, error) {
	lower := strings.ToLower(text)
	vec := make([]float32, len(e.vocab))
	for i, term := range e.vocab {
		if strings.Contains(lower, term) {
			vec[i] = 1
		}
	}
	return vec, nil
}

func TestDocumentRelevance_WhenEmbedderSet_ShouldUseCosineSimilarity(t *testing.T) {
	store, _ := newBridgeStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())
	g.SetEmbedder(&keywordEmbedder{vocab: []string{"kernel", "mangle", "invoice", "billing"}})

	if _, err := g.IngestDocument("docs/kernel.md", "Kernel", "the mangle kernel decides"); err != nil {
		t.Fatalf("IngestDocument: %v", err)
	}

	near, ok := g.DocumentRelevance("changes to the mangle kernel evaluator")
	if !ok {
		t.Fatal("DocumentRelevance reported no corpus after a successful ingest")
	}
	far, ok := g.DocumentRelevance("invoice billing rounding")
	if !ok {
		t.Fatal("DocumentRelevance reported no corpus on the second call")
	}
	if near <= far {
		t.Errorf("cosine relevance did not separate on-topic from off-topic text: near=%.3f far=%.3f", near, far)
	}
}

func TestDocumentRelevance_WhenNoCorpus_ShouldReportUnavailable(t *testing.T) {
	store, _ := newBridgeStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())
	if score, ok := g.DocumentRelevance("anything"); ok {
		t.Errorf("reported a relevance of %.2f with no ingested docs; callers must fall through to their own default", score)
	}
}

func TestDocumentRelevance_WhenNoEmbedder_ShouldFallBackToTermOverlap(t *testing.T) {
	store, _ := newBridgeStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())

	if err := store.IngestDoc(&IngestedDoc{Path: "d.md", Content: "kernel projection reconciliation authority"}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	score, ok := g.DocumentRelevance("the kernel projection is the authority for reconciliation")
	if !ok {
		t.Fatal("no corpus reported despite an ingested doc")
	}
	if score <= 0 {
		t.Errorf("term-overlap relevance = %.2f, want > 0 for text sharing most of the document's terms", score)
	}
}

func TestCalculateRelevance_WhenDocCorpusIsStronger_ShouldPreferIt(t *testing.T) {
	store, _ := newBridgeStore(t)
	if err := store.SaveVision(&Vision{
		Mission:    "unrelated words entirely",
		Problem:    "different vocabulary altogether",
		VisionStmt: "nothing overlapping here",
	}); err != nil {
		t.Fatalf("SaveVision: %v", err)
	}
	g := NewGuardian(store, DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	visionOnly := g.calculateRelevance("guardian reconciliation authority projection")

	if err := store.IngestDoc(&IngestedDoc{Path: "d.md", Content: "guardian reconciliation authority projection"}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	withDocs := g.calculateRelevance("guardian reconciliation authority projection")

	if withDocs <= visionOnly {
		t.Errorf("ingested corpus did not raise relevance: vision-only=%.2f with-docs=%.2f", visionOnly, withDocs)
	}
}

func TestCosineSimilarity_ShouldHandleDegenerateInput(t *testing.T) {
	if got := CosineSimilarity(nil, nil); got != 0 {
		t.Errorf("CosineSimilarity(nil, nil) = %v, want 0", got)
	}
	if got := CosineSimilarity([]float32{1, 0}, []float32{1, 0, 0}); got != 0 {
		t.Errorf("mismatched dimensions returned %v, want 0", got)
	}
	if got := CosineSimilarity([]float32{0, 0}, []float32{1, 1}); got != 0 {
		t.Errorf("zero vector returned %v, want 0", got)
	}
	if got := CosineSimilarity([]float32{1, 1}, []float32{1, 1}); got < 0.999 {
		t.Errorf("identical vectors returned %v, want ~1", got)
	}
}

func TestEmbedding_ShouldSurviveAStoreRoundTrip(t *testing.T) {
	store, _ := newBridgeStore(t)
	want := []float32{0.5, -1.25, 3}
	if err := store.IngestDoc(&IngestedDoc{Path: "v.md", Content: "x", Embedding: want}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	docs, err := store.ListIngestedDocs(1)
	if err != nil || len(docs) != 1 {
		t.Fatalf("ListIngestedDocs: %v (%d docs)", err, len(docs))
	}
	got := docs[0].Embedding
	if len(got) != len(want) {
		t.Fatalf("embedding length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("embedding[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// =============================================================================
// METRICS
// =============================================================================

func TestGetMetrics_WhenChecksRecorded_ShouldReportBlockedRateAndMeanScore(t *testing.T) {
	store, _ := newBridgeStore(t)

	checks := []struct {
		result AlignmentResult
		score  float64
	}{
		{AlignmentPassed, 0.9},
		{AlignmentPassed, 0.8},
		{AlignmentBlocked, 0.1},
		{AlignmentFailed, 0.4},
	}
	for i, c := range checks {
		if err := store.RecordAlignmentCheck(&AlignmentCheck{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Trigger:   TriggerManual,
			Subject:   "s",
			Result:    c.result,
			Score:     c.score,
		}); err != nil {
			t.Fatalf("record check %d: %v", i, err)
		}
	}

	m, err := store.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if m.TotalChecks != 4 {
		t.Errorf("TotalChecks = %d, want 4", m.TotalChecks)
	}
	if want := 0.25; m.BlockedRate < want-1e-9 || m.BlockedRate > want+1e-9 {
		t.Errorf("BlockedRate = %.4f, want %.2f", m.BlockedRate, want)
	}
	if want := 0.25; m.FailedRate < want-1e-9 || m.FailedRate > want+1e-9 {
		t.Errorf("FailedRate = %.4f, want %.2f", m.FailedRate, want)
	}
	if want := 0.55; m.MeanScore < want-1e-6 || m.MeanScore > want+1e-6 {
		t.Errorf("MeanScore = %.4f, want %.2f", m.MeanScore, want)
	}
	if m.ChecksByResult[AlignmentPassed] != 2 {
		t.Errorf("passed count = %d, want 2", m.ChecksByResult[AlignmentPassed])
	}
	if m.FirstCheck.IsZero() || m.LastCheck.IsZero() || !m.LastCheck.After(m.FirstCheck) {
		t.Errorf("check window is wrong: first=%v last=%v", m.FirstCheck, m.LastCheck)
	}
}

func TestGetMetrics_WhenNoChecks_ShouldReportZeroesNotError(t *testing.T) {
	store, _ := newBridgeStore(t)
	m, err := store.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics on an empty store: %v", err)
	}
	if m.TotalChecks != 0 || m.BlockedRate != 0 || m.MeanScore != 0 {
		t.Errorf("empty store metrics = %+v, want zeroes", m)
	}
	if m.OverallAlignment != 1.0 {
		t.Errorf("OverallAlignment = %.2f, want the 1.0 default", m.OverallAlignment)
	}
}

func TestGetMetrics_ShouldCountResolvedAndActiveDriftSeparately(t *testing.T) {
	store, _ := newBridgeStore(t)

	open := &DriftEvent{Severity: DriftMajor, Category: "alignment", Description: "open"}
	closed := &DriftEvent{Severity: DriftMinor, Category: "alignment", Description: "closed"}
	if err := store.RecordDriftEvent(open); err != nil {
		t.Fatalf("record open drift: %v", err)
	}
	if err := store.RecordDriftEvent(closed); err != nil {
		t.Fatalf("record closed drift: %v", err)
	}
	if err := store.ResolveDriftEvent(closed.ID, "fixed"); err != nil {
		t.Fatalf("resolve drift: %v", err)
	}

	m, err := store.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if m.ActiveDrift != 1 || m.ResolvedDrift != 1 {
		t.Errorf("drift counts = active %d / resolved %d, want 1 / 1", m.ActiveDrift, m.ResolvedDrift)
	}
}

func TestGetDriftHistory_ShouldIncludeResolvedEventsWithResolution(t *testing.T) {
	store, _ := newBridgeStore(t)
	drift := &DriftEvent{Severity: DriftMajor, Category: "alignment", Description: "d", Evidence: []string{"file.go"}}
	if err := store.RecordDriftEvent(drift); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := store.ResolveDriftEvent(drift.ID, "reverted the change"); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	active, err := store.GetActiveDriftEvents()
	if err != nil {
		t.Fatalf("GetActiveDriftEvents: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("resolved drift still reported as active: %+v", active)
	}

	history, err := store.GetDriftHistory(10)
	if err != nil {
		t.Fatalf("GetDriftHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if !history[0].Resolved || history[0].Resolution != "reverted the change" {
		t.Errorf("history entry = %+v, want resolved with its resolution text", history[0])
	}
	if history[0].ResolvedAt == nil {
		t.Error("resolved drift has no resolved_at timestamp")
	}
	if len(history[0].Evidence) != 1 {
		t.Errorf("evidence = %v, want one entry", history[0].Evidence)
	}
}
