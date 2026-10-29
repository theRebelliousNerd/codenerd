package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/types"
)

// Graph direction: a Mangle rule can hand any string to QueryLinks, so an
// unknown direction must name itself — never surface as a SQL arg-count error.
func TestGraph_InvalidDirectionNamesItself(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreLink("a", "depends_on", "b", 1.0, nil); err != nil {
		t.Fatal(err)
	}
	_, err := s.QueryLinks("a", "sideways")
	if err == nil {
		t.Fatal("expected an error for an unknown direction, got nil")
	}
	if !strings.Contains(err.Error(), "sideways") {
		t.Errorf("error %q does not name the bad direction", err)
	}
	for _, dir := range []string{"outgoing", "incoming", "both"} {
		if _, err := s.QueryLinks("a", dir); err != nil {
			t.Errorf("direction %q: unexpected error %v", dir, err)
		}
	}
}

// TraversePath distinguishes a clean miss (ErrNoPath) from storage failure:
// a dead database must never report "no path".
func TestGraph_TraversePathMissVsFailure(t *testing.T) {
	s := openLocalTestStore(t)
	if err := s.StoreLink("a", "depends_on", "b", 1.0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TraversePath("b", "a", 5); !errors.Is(err, ErrNoPath) {
		t.Errorf("clean miss err = %v, want errors.Is ErrNoPath", err)
	}

	broken, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = broken.Close()
	if _, err := broken.TraversePath("a", "b", 5); err == nil {
		t.Fatal("expected an error traversing a closed store, got nil")
	} else if errors.Is(err, ErrNoPath) {
		t.Errorf("closed-store error %v must not match ErrNoPath", err)
	}
}

// The Mangle query_graph adapter: nil-safe, clean path misses fold to false,
// and storage failures propagate as errors rather than false.
func TestGraphAdapter_PathSemanticsAndNilSafety(t *testing.T) {
	var nilAdapter *LocalStoreGraphAdapter
	if _, err := nilAdapter.QueryGraph("links", map[string]any{"arg": "a"}); err == nil {
		t.Error("nil adapter: expected an error, got nil")
	}
	if _, err := (&LocalStoreGraphAdapter{}).QueryGraph("links", map[string]any{"arg": "a"}); err == nil {
		t.Error("adapter without a store: expected an error, got nil")
	}

	s := openLocalTestStore(t)
	if err := s.StoreLink("a", "depends_on", "b", 1.0, nil); err != nil {
		t.Fatal(err)
	}
	a := NewLocalStoreGraphAdapter(s)
	got, err := a.QueryGraph("path", map[string]any{"arg": "a->b"})
	if err != nil || got != true {
		t.Errorf("path a->b = %v, %v; want true, nil", got, err)
	}
	got, err = a.QueryGraph("path", map[string]any{"arg": "b->a"})
	if err != nil || got != false {
		t.Errorf("path b->a = %v, %v; want false, nil", got, err)
	}
	if _, err := a.QueryGraph("bogus", map[string]any{"arg": "a"}); err == nil {
		t.Error("unknown query type: expected an error, got nil")
	}

	broken, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_ = broken.Close()
	if _, err := NewLocalStoreGraphAdapter(broken).QueryGraph("path", map[string]any{"arg": "a->b"}); err == nil {
		t.Error("path over a closed store: expected an error, got nil/false")
	}
}

// Fact codec: nanosecond int64 values survive the round trip bit-for-bit —
// float64 would round them and silently break retraction matching.
func TestFactCodec_Int64NanosecondRoundTrip(t *testing.T) {
	const ts = int64(1786773933859876776)
	enc, err := encodeFactArgs([]any{ts, int32(-7), float32(1.5), "s", nil, true, types.MangleAtom("/a")})
	if err != nil {
		t.Fatal(err)
	}
	dec, err := decodeFactArgs(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec) != 7 {
		t.Fatalf("decoded %d args, want 7", len(dec))
	}
	if dec[0] != ts {
		t.Errorf("int64 = %v (%T), want %d exactly", dec[0], dec[0], ts)
	}
	if dec[1] != int64(-7) {
		t.Errorf("int32 = %v (%T), want int64(-7)", dec[1], dec[1])
	}
	if f, ok := dec[2].(float64); !ok || f != float64(float32(1.5)) {
		t.Errorf("float32 = %v (%T), want float64", dec[2], dec[2])
	}
	if _, ok := dec[6].(types.MangleAtom); !ok {
		t.Errorf("atom = %v (%T), want types.MangleAtom", dec[6], dec[6])
	}
}

// Fact codec: a huge uint64 keeps its digits as a string instead of wrapping,
// and an unknown tag never leaks json.Number into kernel facts.
func TestFactCodec_Uint64AndUnknownTag(t *testing.T) {
	enc, err := encodeFactArgs([]any{uint64(math.MaxUint64), uint64(42)})
	if err != nil {
		t.Fatal(err)
	}
	dec, err := decodeFactArgs(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec[0] != "18446744073709551615" {
		t.Errorf("huge uint64 = %v (%T), want the digit string", dec[0], dec[0])
	}
	if dec[1] != int64(42) {
		t.Errorf("small uint64 = %v (%T), want int64(42)", dec[1], dec[1])
	}

	raw := `[{"type":"future_tag","value":1234567890123456789},{"type":"future_tag","value":1.5}]`
	dec, err = decodeFactArgs(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, isNum := dec[0].(json.Number); isNum {
		t.Errorf("unknown tag leaked json.Number: %v (%T)", dec[0], dec[0])
	}
	if dec[0] != int64(1234567890123456789) {
		t.Errorf("unknown int tag = %v (%T), want int64", dec[0], dec[0])
	}
	if _, ok := dec[1].(float64); !ok {
		t.Errorf("unknown float tag = %v (%T), want float64", dec[1], dec[1])
	}
	if _, err := decodeFactArgs("not json"); err == nil {
		t.Error("garbage input: expected an error, got nil")
	}
}

// Vector parser: a truncated column is corrupt data, not a short vector.
func TestFastParseVectorJSON_UnterminatedIsCorrupt(t *testing.T) {
	for _, in := range []string{"[0.1,0.2", "[", "[1.0, "} {
		if _, err := fastParseVectorJSON([]byte(in), nil); err == nil {
			t.Errorf("input %q: expected an error, got nil", in)
		}
	}
	got, err := fastParseVectorJSON([]byte("[0.1, 0.2]"), nil)
	if err != nil || len(got) != 2 {
		t.Errorf("valid input = %v, %v; want 2 values, nil", got, err)
	}
	if got, err := fastParseVectorJSON(nil, nil); err != nil || len(got) != 0 {
		t.Errorf("empty input = %v, %v; want empty, nil", got, err)
	}
}

// Descriptor hygiene: secrets stay redacted and truncation never splits a rune.
func TestReflectionUtils_SanitizeAndClamp(t *testing.T) {
	out := sanitizeDescriptor("token=abc123 see main.go for detail")
	if strings.Contains(out, "abc123") {
		t.Errorf("secret survived sanitizing: %q", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("file hint lost: %q", out)
	}
	wide := strings.Repeat("界", defaultDescriptorMaxLen+100)
	out = sanitizeDescriptor(wide)
	if !utf8.ValidString(out) {
		t.Error("truncation produced invalid UTF-8")
	}
	if n := len([]rune(out)); n != defaultDescriptorMaxLen {
		t.Errorf("truncated to %d runes, want %d", n, defaultDescriptorMaxLen)
	}
	if got := clampScore(math.NaN()); got != 0 {
		t.Errorf("clampScore(NaN) = %v, want 0", got)
	}
	if got := clampScore(-2); got != 0 {
		t.Errorf("clampScore(-2) = %v, want 0", got)
	}
	if got := clampScore(2); got != 1 {
		t.Errorf("clampScore(2) = %v, want 1", got)
	}
}

// Learning candidates: counts accumulate per key, status transitions stick,
// and unknown ids fail instead of silently updating nothing.
func TestLearningCandidates_Lifecycle(t *testing.T) {
	s := openLocalTestStore(t)
	n, err := s.RecordLearningCandidate("fix it", "/fix", "code", "r1")
	if err != nil || n != 1 {
		t.Fatalf("first record = %d, %v; want 1, nil", n, err)
	}
	n, err = s.RecordLearningCandidate("fix it", "/fix", "code", "r1")
	if err != nil || n != 2 {
		t.Fatalf("second record = %d, %v; want 2, nil", n, err)
	}
	if _, err := s.RecordLearningCandidate("other", "/fix", "code", "r1"); err != nil {
		t.Fatal(err)
	}
	cands, err := s.ListLearningCandidates("", 0)
	if err != nil || len(cands) != 2 {
		t.Fatalf("list = %d, %v; want 2, nil", len(cands), err)
	}
	if cands[0].Count != 1 || cands[1].Count != 2 {
		// Same-second rows order by id DESC: newest first, deterministically.
		t.Errorf("counts in order = %d,%d; want 1,2", cands[0].Count, cands[1].Count)
	}
	pending, err := s.ListLearningCandidates("pending", 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending = %d, %v; want 2, nil", len(pending), err)
	}
	if err := s.ConfirmLearningCandidate(cands[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RejectLearningCandidateMatch("other", "/fix", "code", "r1"); err != nil {
		t.Fatal(err)
	}
	pending, err = s.ListLearningCandidates("pending", 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending after transitions = %d, %v; want 0, nil", len(pending), err)
	}
	if err := s.ConfirmLearningCandidate(999999); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unknown id err = %v, want sql.ErrNoRows", err)
	}
}

// Review findings: write then read back — the table is no longer write-only —
// with project filtering, limits, and newest-first order.
func TestReviewFindings_StoreAndList(t *testing.T) {
	s := openLocalTestStore(t)
	mk := func(path, sev, root string) StoredReviewFinding {
		return StoredReviewFinding{FilePath: path, Line: 3, Severity: sev,
			Category: "c", RuleID: "r", Message: "m", ProjectRoot: root}
	}
	if err := s.StoreReviewFinding(mk("a.go", "high", "p1")); err != nil {
		t.Fatal(err)
	}
	if err := s.StoreReviewFinding(mk("b.go", "low", "p2")); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListReviewFindings("", 0)
	if err != nil || len(all) != 2 {
		t.Fatalf("list all = %d, %v; want 2, nil", len(all), err)
	}
	if all[0].FilePath != "b.go" || all[1].FilePath != "a.go" {
		t.Errorf("order = %s,%s; want newest first (b.go,a.go)", all[0].FilePath, all[1].FilePath)
	}
	one, err := s.ListReviewFindings("p1", 10)
	if err != nil || len(one) != 1 || one[0].FilePath != "a.go" {
		t.Fatalf("project filter = %+v, %v; want [a.go], nil", one, err)
	}
	lim, err := s.ListReviewFindings("", 1)
	if err != nil || len(lim) != 1 {
		t.Fatalf("limit 1 = %d, %v; want 1, nil", len(lim), err)
	}
	var nilStore *LocalStore
	if err := nilStore.StoreReviewFinding(mk("x", "low", "")); err == nil {
		t.Error("nil store write: expected an error, got nil")
	}
	if _, err := nilStore.ListReviewFindings("", 0); err == nil {
		t.Error("nil store list: expected an error, got nil")
	}
}

// Prompt atoms: guards reject nil/empty writes, and a stored atom round-trips
// with its selector dimensions intact across every read path.
func TestPromptAtoms_GuardsAndRoundTrip(t *testing.T) {
	var nilStore *LocalStore
	if err := nilStore.StorePromptAtom(&PromptAtom{AtomID: "x"}); err == nil {
		t.Error("nil store: expected an error, got nil")
	}
	s := openLocalTestStore(t)
	if err := s.StorePromptAtom(nil); err == nil {
		t.Error("nil atom: expected an error, got nil")
	}
	if err := s.StorePromptAtom(&PromptAtom{}); err == nil {
		t.Error("empty atom_id: expected an error, got nil")
	}
	if _, err := nilStore.LoadPromptAtoms(); err == nil {
		t.Error("nil store load: expected an error, got nil")
	}

	in := &PromptAtom{AtomID: "identity/coder/mission", Version: 2, Content: "hello",
		TokenCount: 3, ContentHash: "h", Category: "identity", Subcategory: "sub",
		OperationalModes: []string{"/active"}, IntentVerbs: []string{"/fix"},
		Languages: []string{"/go"}, Priority: 9, IsMandatory: true,
		IsExclusive: "g", DependsOn: []string{"d"}, ConflictsWith: []string{"c"},
		EmbeddingTask: "RETRIEVAL_DOCUMENT"}
	if err := s.StorePromptAtom(in); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetPromptAtom("identity/coder/mission")
	if err != nil || got == nil {
		t.Fatalf("get = %+v, %v; want the atom, nil", got, err)
	}
	if got.Version != 2 || got.TokenCount != 3 || !got.IsMandatory || got.IsExclusive != "g" {
		t.Errorf("scalar round trip = %+v, want version 2 / 3 tokens / mandatory / group g", got)
	}
	if len(got.OperationalModes) != 1 || got.OperationalModes[0] != "/active" ||
		len(got.IntentVerbs) != 1 || len(got.Languages) != 1 || len(got.DependsOn) != 1 {
		t.Errorf("selector round trip = %+v, want the stored dimensions", got)
	}
	byCat, err := s.LoadPromptAtomsByCategory("identity")
	if err != nil || len(byCat) != 1 {
		t.Fatalf("by category = %d, %v; want 1, nil", len(byCat), err)
	}
	all, err := s.LoadPromptAtoms()
	if err != nil || len(all) != 1 {
		t.Fatalf("load all = %d, %v; want 1, nil", len(all), err)
	}
	// Upsert over the same atom_id replaces content rather than duplicating.
	in.Content = "updated"
	if err := s.StorePromptAtom(in); err != nil {
		t.Fatal(err)
	}
	all, err = s.LoadPromptAtoms()
	if err != nil || len(all) != 1 || all[0].Content != "updated" {
		t.Fatalf("after upsert = %+v, %v; want 1 updated row", all, err)
	}
	if err := s.DeletePromptAtom("identity/coder/mission"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeletePromptAtom("identity/coder/mission"); err != nil {
		t.Errorf("deleting a missing atom should be a no-op: %v", err)
	}
	if all, _ := s.LoadPromptAtoms(); len(all) != 0 {
		t.Errorf("after delete = %d atoms, want 0", len(all))
	}
}

// Re-embed-all honors cancellation: a dead context returns before touching any
// database, and reports the context error rather than empty success.
func TestReembedAll_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReembedAllDBsForce(ctx, []string{t.TempDir()}, stubEngine{}, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled run err = %v, want context.Canceled", err)
	}
	if _, err := ReembedAllDBsForce(context.Background(), []string{t.TempDir()}, nil, nil); err == nil {
		t.Error("nil engine: expected an error, got nil")
	}
}

type stubEngine struct{}

func (stubEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func (stubEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{0.1, 0.2}
	}
	return out, nil
}

func (stubEngine) Dimensions() int { return 2 }
func (stubEngine) Name() string    { return "stub" }
