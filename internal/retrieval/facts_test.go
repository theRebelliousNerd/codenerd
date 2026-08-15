package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// newSeedKernel builds a real kernel with the embedded default corpus, which is
// the only way to test what this package asserts: the Decl bounds live in
// internal/core/defaults/*.mg and a fact that violates one is dropped silently,
// so a fake sink would happily "pass" on facts the kernel would reject.
func newSeedKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	return k
}

func queryFacts(t *testing.T, k *core.RealKernel, pred string) []core.Fact {
	t.Helper()
	facts, err := k.Query(pred)
	if err != nil {
		t.Fatalf("Query(%s): %v", pred, err)
	}
	return facts
}

func seedWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", []byte("module seedmod\n\ngo 1.26\n"))
	writeFile(t, dir, "internal/alpha/alpha.go", []byte(
		"package alpha\n\nimport \"seedmod/internal/beta\"\n\n// WidgetError is raised by build.\ntype WidgetError struct{}\n\nfunc build_widget() error { return beta.Help() }\n"))
	writeFile(t, dir, "internal/beta/beta.go", []byte("package beta\n\nfunc Help() error { return nil }\n"))
	return dir
}

// =============================================================================
// P0: the wire
// =============================================================================

// TestSeedIssueFacts_WhenIssueNamesAFile_ShouldAssertRetrievalFactsIntoKernel is
// the regression test for the central defect: retrieval produced results that
// never became facts, so no Mangle rule could reason about what was retrieved.
func TestSeedIssueFacts_WhenIssueNamesAFile_ShouldAssertRetrievalFactsIntoKernel(t *testing.T) {
	dir := seedWorkspace(t)
	k := newSeedKernel(t)

	report, err := SeedIssueFacts(context.Background(), k, SeedRequest{
		IssueID:   "/issue_seed_test",
		IssueText: "WidgetError raised from internal/alpha/alpha.go when calling build_widget()",
		WorkDir:   dir,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		t.Fatalf("SeedIssueFacts: %v", err)
	}
	if report == nil {
		t.Fatal("nil report from a non-empty issue")
	}
	if report.Facts == 0 {
		t.Fatal("no facts built")
	}

	// Every predicate the corpus declares for this flow must actually be in the
	// store. Querying is the only proof: a Decl-nonconformant fact is rejected
	// at insert and would leave the predicate empty here.
	for _, pred := range []string{
		"issue_text", "issue_keyword", "keyword_weight",
		"file_mentioned", "candidate_file", "keyword_hit",
		"context_tier", "tiered_context_file", "issue_context",
	} {
		if got := queryFacts(t, k, pred); len(got) == 0 {
			t.Errorf("%s has no facts in the EDB after a seed pass", pred)
		}
	}

	// The mention must be asserted as a resolved, workspace-relative path.
	var mentions []string
	for _, f := range queryFacts(t, k, "file_mentioned") {
		mentions = append(mentions, types.ArgString(f, 0))
	}
	if !containsString(mentions, "internal/alpha/alpha.go") {
		t.Errorf("file_mentioned paths = %v, want the resolved workspace path", mentions)
	}

	// issue_context must summarize the pass, keyed on the same issue ID.
	summaries := queryFacts(t, k, "issue_context")
	if len(summaries) != 1 {
		t.Fatalf("issue_context count = %d, want 1", len(summaries))
	}
	if got := types.ArgString(summaries[0], 0); got != "/issue_seed_test" {
		t.Errorf("issue_context issue id = %q, want /issue_seed_test", got)
	}
	if n, ok := types.ArgInt64(summaries[0], 1); !ok || n == 0 {
		t.Errorf("issue_context total files = %v (ok=%v), want a positive count", n, ok)
	}
}

// TestSeedIssueFacts_ShouldReachMoreThanOneTier proves the tiers beyond the
// hand-named files are populated; before the wire only tier 1 ever existed.
func TestSeedIssueFacts_ShouldReachMoreThanOneTier(t *testing.T) {
	dir := seedWorkspace(t)
	k := newSeedKernel(t)

	if _, err := SeedIssueFacts(context.Background(), k, SeedRequest{
		IssueID:   "/issue_tiers",
		IssueText: "WidgetError raised from internal/alpha/alpha.go when calling build_widget()",
		WorkDir:   dir,
		Timeout:   30 * time.Second,
	}); err != nil {
		t.Fatalf("SeedIssueFacts: %v", err)
	}

	tiers := map[string]bool{}
	for _, f := range queryFacts(t, k, "tiered_context_file") {
		tiers[types.ArgName(f, 2)] = true
	}
	if len(tiers) < 2 {
		t.Errorf("tiered_context_file covered tiers %v, want more than one tier", tiers)
	}
	if !tiers["/tier1"] {
		t.Errorf("tier 1 missing from %v", tiers)
	}
}

// TestSeedIssueFacts_WhenBudgetExpires_ShouldStillAssertKeywordFacts: a slow
// filesystem must cost the disk-ranked half of the seed, not all of it.
func TestSeedIssueFacts_WhenBudgetExpires_ShouldStillAssertKeywordFacts(t *testing.T) {
	dir := seedWorkspace(t)
	k := newSeedKernel(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := SeedIssueFacts(ctx, k, SeedRequest{
		IssueID:   "/issue_expired",
		IssueText: "WidgetError raised from internal/alpha/alpha.go",
		WorkDir:   dir,
		Timeout:   time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("SeedIssueFacts on an expired budget must degrade, not fail: %v", err)
	}
	if !report.TimedOut {
		t.Error("report did not record the expired budget")
	}
	if got := queryFacts(t, k, "issue_keyword"); len(got) == 0 {
		t.Error("keyword facts were lost along with the search")
	}
}

// TestSeedIssueFacts_WhenIssueTextEmpty_ShouldDoNothing.
func TestSeedIssueFacts_WhenIssueTextEmpty_ShouldDoNothing(t *testing.T) {
	k := newSeedKernel(t)
	report, err := SeedIssueFacts(context.Background(), k, SeedRequest{IssueText: "   \n"})
	if err != nil || report != nil {
		t.Fatalf("empty issue: report=%v err=%v, want nil/nil", report, err)
	}
	if got := queryFacts(t, k, "issue_text"); len(got) != 0 {
		t.Errorf("empty issue asserted %d issue_text facts", len(got))
	}
}

// =============================================================================
// Decl conformance
// =============================================================================

// TestIssueKeywordFacts_WhenWeightIsFractional_ShouldSurviveTheKernel is the
// regression test for the silently-dropped-fact bug. issue_keyword's Weight is
// declared /number; the old seed path passed the raw 0..1 ratio as a float64 and
// RealKernel.coerceAtomToDeclLocked rejected every non-integral one, so only the
// weight-1.0 facts on mentioned files ever reached the store.
func TestIssueKeywordFacts_WhenWeightIsFractional_ShouldSurviveTheKernel(t *testing.T) {
	k := newSeedKernel(t)

	keywords := &IssueKeywords{
		Primary: []string{"WidgetError"},
		Weights: map[string]float64{
			"WidgetError":  0.9, // was rejected: fractional float in a /number slot
			"build_widget": 0.7, // likewise
			"alpha.go":     1.0, // the only weight that used to survive
		},
	}
	facts := IssueKeywordFacts("/issue_weights", "irrelevant", keywords)
	if err := k.LoadFacts(facts); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}

	got := map[string]int64{}
	for _, f := range queryFacts(t, k, "issue_keyword") {
		w, ok := types.ArgInt64(f, 2)
		if !ok {
			t.Fatalf("issue_keyword weight is not an int64: %#v", f.Args[2])
		}
		got[types.ArgString(f, 1)] = w
	}

	// Negative control: the shape the old seed path used must still be rejected,
	// otherwise this test would pass for the wrong reason if the kernel ever
	// started accepting fractional floats in a /number slot.
	before := len(queryFacts(t, k, "issue_keyword"))
	_ = k.LoadFacts([]core.Fact{{
		Predicate: "issue_keyword",
		Args:      []any{"/issue_weights", "legacy_float", 0.42},
	}})
	if after := len(queryFacts(t, k, "issue_keyword")); after != before {
		t.Errorf("a fractional float in a /number slot was accepted (%d -> %d); "+
			"PercentFromRatio's rationale needs revisiting", before, after)
	}

	want := map[string]int64{"WidgetError": 90, "build_widget": 70, "alpha.go": 100}
	for kw, w := range want {
		if got[kw] != w {
			t.Errorf("issue_keyword weight for %q = %d, want %d (all %d keywords: %v)",
				kw, got[kw], w, len(got), got)
		}
	}
}

// TestSeedFacts_ShouldMatchSchemaDeclArity is the cross-package arity/bounds
// check the corpus asked for: the Go side and schemas_knowledge.mg must not
// drift apart, and the failure mode of drift is a silently empty predicate
// rather than an error.
func TestSeedFacts_ShouldMatchSchemaDeclArity(t *testing.T) {
	decls := loadKnowledgeDecls(t)

	tc := &TieredContext{
		Keywords: &IssueKeywords{MentionedFiles: []string{"a.go"}},
		Files: []ContextFile{
			{FilePath: "a.go", Tier: 1, RelevanceScore: 1.0},
			{FilePath: "b.go", Tier: 2, RelevanceScore: 0.4},
		},
		Candidates: []CandidateFile{{
			FilePath:       "b.go",
			RelevanceScore: 0.4,
			Keywords:       []string{"kw"},
			Hits:           []KeywordHit{{FilePath: "b.go", Keyword: "kw"}},
		}},
	}

	facts := append(
		IssueKeywordFacts("/issue_arity", "text", &IssueKeywords{
			Primary: []string{"Kw"},
			Weights: map[string]float64{"Kw": 0.5},
		}),
		TieredContextFacts("/issue_arity", tc, "")...,
	)
	if len(facts) == 0 {
		t.Fatal("no facts to check")
	}

	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Predicate] = true
		bounds, ok := decls[f.Predicate]
		if !ok {
			t.Errorf("%s is asserted but not Declared in schemas_knowledge.mg", f.Predicate)
			continue
		}
		if len(bounds) != len(f.Args) {
			t.Errorf("%s asserted with arity %d, Declared arity %d", f.Predicate, len(f.Args), len(bounds))
			continue
		}
		for i, bound := range bounds {
			if got := mangleKindOf(f.Args[i]); got != bound {
				t.Errorf("%s arg %d is %s but Declared %s (value %#v)", f.Predicate, i, got, bound, f.Args[i])
			}
		}
	}

	// Every predicate section 52 declares for this flow must be produced, or the
	// wire has quietly lost a limb again.
	for _, pred := range []string{
		"issue_text", "issue_keyword", "keyword_weight", "file_mentioned",
		"candidate_file", "keyword_hit", "context_tier", "tiered_context_file",
		"issue_context",
	} {
		if !seen[pred] {
			t.Errorf("no %s fact produced by the transducer", pred)
		}
	}
}

// mangleKindOf reports the Decl bound a Go argument value satisfies.
func mangleKindOf(arg any) string {
	switch arg.(type) {
	case types.MangleAtom:
		return "/name"
	case types.MangleString:
		return "/string"
	case int64, int:
		return "/number"
	case float64, float32:
		// Deliberately not "/number": ToAtom makes this an ast.Float64 and the
		// kernel rejects a fractional one outright.
		return "/float64"
	case string:
		return "/string"
	}
	return "/unknown"
}

var knowledgeDeclRe = regexp.MustCompile(`(?m)^\s*Decl\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)\s*bound\s*\[([^\]]*)\]`)

func loadKnowledgeDecls(t *testing.T) map[string][]string {
	t.Helper()
	path := filepath.Join(repoRoot(t), "internal", "core", "defaults", "schemas_knowledge.mg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	out := map[string][]string{}
	for _, m := range knowledgeDeclRe.FindAllStringSubmatch(string(data), -1) {
		bounds := make([]string, 0, 4)
		for _, b := range strings.Split(m[3], ",") {
			bounds = append(bounds, strings.TrimSpace(b))
		}
		out[m[1]] = bounds
	}
	if len(out) == 0 {
		t.Fatalf("no Decls parsed from %s; this guard is blind", path)
	}
	return out
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

// =============================================================================
// Path resolution
// =============================================================================

func TestWorkspacePath_ShouldRelativizeInsideWorkspace(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "repo", "proj")

	if got := workspacePath(root, filepath.Join(root, "internal", "x.go")); got != "internal/x.go" {
		t.Errorf("inside workspace = %q, want internal/x.go", got)
	}
	// A path outside the workspace must not be rewritten into a ../.. chain.
	outside := filepath.Join(string(filepath.Separator), "elsewhere", "y.go")
	if got := workspacePath(root, outside); !strings.HasSuffix(got, "elsewhere/y.go") {
		t.Errorf("outside workspace = %q, want the absolute path preserved", got)
	}
	if got := workspacePath("", ""); got != "" {
		t.Errorf("empty path = %q, want empty", got)
	}
}

func TestTierAtom_ShouldRejectOutOfRangeTiers(t *testing.T) {
	t.Parallel()
	for tier, want := range map[int]types.MangleAtom{1: "/tier1", 2: "/tier2", 3: "/tier3", 4: "/tier4", 0: "", 5: ""} {
		if got := tierAtom(tier); got != want {
			t.Errorf("tierAtom(%d) = %q, want %q", tier, got, want)
		}
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
