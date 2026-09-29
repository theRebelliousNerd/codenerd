package orient

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func mustEngine(t *testing.T, cfg *config.OrientConfig) *Engine {
	t.Helper()
	if cfg == nil {
		d := config.DefaultOrientConfig()
		cfg = &d
	}
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func evalFacts(t *testing.T, cfg *config.OrientConfig, facts []types.Fact) *Engine {
	t.Helper()
	e := mustEngine(t, cfg)
	if err := e.Assert(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	if err := e.Evaluate(context.Background()); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	return e
}

func F(pred string, args ...any) types.Fact {
	return types.Fact{Predicate: pred, Args: args}
}

func N(s string) types.MangleAtom {
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return types.MangleAtom(s)
}

func spanFact(first, last, commits int64, shallow string) types.Fact {
	return F("repo_span", first, last, commits, N(shallow))
}

func monthFact(i int, label string, commits int) types.Fact {
	return F("repo_month", int64(i), label, int64(commits), int64(0), int64(0))
}

func mspan(i int, start, end int64) types.Fact {
	return F("repo_month_span", int64(i), start, end)
}

func histFact(path string, first, last, commits, days int64) types.Fact {
	return F("repo_file_history", path, first, last, commits, days)
}

func docFact(path, dir string) types.Fact {
	return F("doc_file", path, dir, int64(10), int64(1))
}

func tieFact(path string, ord int) types.Fact {
	return F("doc_tie", path, int64(ord))
}

func rowMatch(row types.Fact, args []string) bool {
	if len(row.Args) != len(args) {
		return false
	}
	for i, want := range args {
		if types.ExtractString(row.Args[i]) != want {
			return false
		}
	}
	return true
}

func hasRow(t *testing.T, e *Engine, pred string, args ...string) bool {
	t.Helper()
	rows, err := e.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	for _, row := range rows {
		if rowMatch(row, args) {
			return true
		}
	}
	return false
}

func mustRow(t *testing.T, e *Engine, pred string, args ...string) {
	t.Helper()
	if hasRow(t, e, pred, args...) {
		return
	}
	rows, _ := e.Query(pred)
	t.Fatalf("%s missing (%s)\nhave %s", pred, strings.Join(args, ", "), dumpRows(rows))
}

func refuseRow(t *testing.T, e *Engine, pred string, args ...string) {
	t.Helper()
	if hasRow(t, e, pred, args...) {
		t.Fatalf("%s unexpectedly has (%s)", pred, strings.Join(args, ", "))
	}
}

func predCount(t *testing.T, e *Engine, pred string) int {
	t.Helper()
	rows, err := e.Query(pred)
	if err != nil {
		t.Fatalf("query %s: %v", pred, err)
	}
	return len(rows)
}

func dumpRows(rows []types.Fact) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		args := make([]string, len(row.Args))
		for i, a := range row.Args {
			args[i] = types.ExtractString(a)
		}
		parts = append(parts, strings.Join(args, ","))
	}
	return strings.Join(parts, " | ")
}

func threeMonths(shallow string) []types.Fact {
	return []types.Fact{
		spanFact(0, 299, 22, shallow),
		monthFact(0, "2020-01", 10),
		monthFact(1, "2020-02", 0),
		monthFact(2, "2020-03", 12),
		mspan(0, 0, 99),
		mspan(1, 100, 199),
		mspan(2, 200, 299),
	}
}

func TestPolicy_ErasFollowTheLullParam(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		lull    int
		shallow string
		want    [][4]string
	}{
		{
			name: "empty month is a lull between waves", lull: 1, shallow: "/no",
			want: [][4]string{{"0", "0", "0", "/wave"}, {"1", "1", "1", "/lull"}, {"2", "2", "2", "/wave"}},
		},
		{
			name: "threshold is read, not a constant", lull: 11, shallow: "/no",
			want: [][4]string{{"0", "0", "1", "/lull"}, {"1", "2", "2", "/wave"}},
		},
		{
			name: "shallow history draws no eras", lull: 1, shallow: "/yes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			cfg.EraLullCommits = tc.lull
			facts := threeMonths(tc.shallow)
			facts = append(facts, histFact("a.md", 10, 20, 6, 1), docFact("a.md", ""))
			e := evalFacts(t, &cfg, facts)
			if got := predCount(t, e, "repo_era"); got != len(tc.want) {
				rows, _ := e.Query("repo_era")
				t.Fatalf("eras %d, want %d (%s)", got, len(tc.want), dumpRows(rows))
			}
			for _, w := range tc.want {
				mustRow(t, e, "repo_era", w[0], w[1], w[2], w[3])
			}
			if tc.shallow == "/yes" {
				if predCount(t, e, "doc_generation") != 0 || predCount(t, e, "doc_burst") != 0 || predCount(t, e, "origin_source") != 0 {
					t.Fatal("shallow history still drew a timeline judgment")
				}
			}
		})
	}
}

func TestPolicy_Burst(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		commits, days int64
		want          bool
	}{
		{name: "six commits one day", commits: 6, days: 1, want: true},
		{name: "six commits two days", commits: 6, days: 2, want: true},
		{name: "six commits three days", commits: 6, days: 3, want: false},
		{name: "five commits one day", commits: 5, days: 1, want: true},
		{name: "four commits one day", commits: 4, days: 1, want: false},
	}
	facts := []types.Fact{spanFact(0, 100, 5, "/no")}
	for i, tc := range cases {
		p := fmt.Sprintf("b%d.md", i)
		facts = append(facts, histFact(p, 10, 10, tc.commits, tc.days), docFact(p, ""))
	}
	e := evalFacts(t, nil, facts)
	for i, tc := range cases {
		p := fmt.Sprintf("b%d.md", i)
		if hasRow(t, e, "doc_burst", p) != tc.want {
			t.Errorf("%s: burst = %v, want %v", tc.name, !tc.want, tc.want)
		}
	}
}

func TestPolicy_CohortIsASharedBirthDay(t *testing.T) {
	t.Parallel()
	facts := []types.Fact{spanFact(0, 86400*30, 9, "/no")}
	for i := 0; i < 8; i++ {
		p := fmt.Sprintf("c%d.md", i)
		facts = append(facts, histFact(p, 86400*10, 86400*10, 1, 1), docFact(p, ""))
	}
	facts = append(facts, histFact("solo.md", 86400*20, 86400*20, 1, 1), docFact("solo.md", ""))
	e := evalFacts(t, nil, facts)
	if got := predCount(t, e, "doc_cohort"); got != 8 {
		rows, _ := e.Query("doc_cohort")
		t.Fatalf("cohort %d, want 8 (%s)", got, dumpRows(rows))
	}
	refuseRow(t, e, "doc_cohort", "solo.md")
}

func TestPolicy_Generation(t *testing.T) {
	t.Parallel()
	t.Run("multi era by birth era", func(t *testing.T) {
		facts := threeMonths("/no")
		facts = append(facts,
			histFact("old.md", 10, 10, 1, 1),
			histFact("mid.md", 150, 150, 1, 1),
			histFact("new.md", 250, 250, 1, 1),
		)
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_generation", "old.md", "/origin")
		mustRow(t, e, "doc_generation", "mid.md", "/early")
		mustRow(t, e, "doc_generation", "new.md", "/recent")
	})
	t.Run("single era by span position", func(t *testing.T) {
		facts := []types.Fact{
			spanFact(0, 1000, 3, "/no"),
			monthFact(0, "m", 5),
			mspan(0, 0, 1000),
			histFact("o.md", 0, 0, 1, 1),
			histFact("m.md", 500, 500, 1, 1),
			histFact("r.md", 950, 950, 1, 1),
		}
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_generation", "o.md", "/origin")
		mustRow(t, e, "doc_generation", "m.md", "/middle")
		mustRow(t, e, "doc_generation", "r.md", "/recent")
	})
	t.Run("zero width span is origin", func(t *testing.T) {
		facts := []types.Fact{
			spanFact(5, 5, 1, "/no"),
			monthFact(0, "m", 1),
			mspan(0, 0, 100),
			histFact("z.md", 5, 5, 1, 1),
			docFact("z.md", ""),
		}
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_generation", "z.md", "/origin")
		mustRow(t, e, "origin_source", "z.md", "/zero_span")
	})
}

func TestPolicy_EvolutionAndSupersession(t *testing.T) {
	t.Parallel()
	base := func() []types.Fact {
		return append(threeMonths("/no"),
			histFact("a.md", 10, 20, 1, 1),
			histFact("b.md", 250, 280, 1, 1),
			docFact("a.md", ""),
			docFact("b.md", ""),
			F("doc_similar", "a.md", "b.md", int64(900)),
		)
	}
	t.Run("no themes evolves and supersedes a quiet origin", func(t *testing.T) {
		e := evalFacts(t, nil, base())
		mustRow(t, e, "doc_evolved_into", "a.md", "b.md")
		mustRow(t, e, "doc_superseded", "a.md", "b.md")
		refuseRow(t, e, "doc_live", "a.md")
		mustRow(t, e, "doc_live", "b.md")
		mustRow(t, e, "chain_latest", "b.md")
		refuseRow(t, e, "origin_source", "a.md", "/birth_era")
		refuseRow(t, e, "read_reason", "a.md", "/origin")
		mustRow(t, e, "read_reason", "b.md", "/chain_latest")
	})
	t.Run("lex order is not time order", func(t *testing.T) {
		facts := append(threeMonths("/no"),
			histFact("m.md", 10, 20, 1, 1),
			histFact("c.md", 250, 280, 1, 1),
			docFact("m.md", ""),
			docFact("c.md", ""),
			F("doc_similar", "c.md", "m.md", int64(900)),
		)
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_evolved_into", "m.md", "c.md")
	})
	t.Run("differing themes block", func(t *testing.T) {
		facts := append(base(), F("doc_theme", "a.md", "red"), F("doc_theme", "b.md", "blue"))
		e := evalFacts(t, nil, facts)
		if predCount(t, e, "doc_evolved_into") != 0 {
			t.Fatal("evolved across different themes")
		}
	})
	t.Run("shared theme allows", func(t *testing.T) {
		facts := append(base(), F("doc_theme", "a.md", "same"), F("doc_theme", "b.md", "same"))
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_evolved_into", "a.md", "b.md")
	})
	t.Run("one side without a theme allows", func(t *testing.T) {
		facts := append(base(), F("doc_theme", "a.md", "same"))
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_evolved_into", "a.md", "b.md")
	})
	t.Run("equal birth does not evolve", func(t *testing.T) {
		facts := append(threeMonths("/no"),
			histFact("a.md", 10, 20, 1, 1),
			histFact("b.md", 10, 280, 1, 1),
			docFact("a.md", ""),
			docFact("b.md", ""),
			F("doc_similar", "a.md", "b.md", int64(900)),
		)
		e := evalFacts(t, nil, facts)
		if predCount(t, e, "doc_evolved_into") != 0 {
			t.Fatal("evolved at equal birth")
		}
	})
	t.Run("below the floor does not evolve", func(t *testing.T) {
		facts := append(threeMonths("/no"),
			histFact("a.md", 10, 20, 1, 1),
			histFact("b.md", 250, 280, 1, 1),
			docFact("a.md", ""),
			docFact("b.md", ""),
			F("doc_similar", "a.md", "b.md", int64(699)),
		)
		e := evalFacts(t, nil, facts)
		if predCount(t, e, "doc_evolved_into") != 0 {
			t.Fatal("evolved below the floor")
		}
	})
	t.Run("a live origin is not superseded", func(t *testing.T) {
		facts := append(threeMonths("/no"),
			histFact("a.md", 10, 250, 2, 2),
			histFact("b.md", 260, 280, 1, 1),
			docFact("a.md", ""),
			docFact("b.md", ""),
			F("doc_similar", "a.md", "b.md", int64(900)),
		)
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_evolved_into", "a.md", "b.md")
		if predCount(t, e, "doc_superseded") != 0 {
			t.Fatal("superseded a document that was still touched in the recent era")
		}
		mustRow(t, e, "origin_source", "a.md", "/birth_era")
		mustRow(t, e, "doc_live", "a.md")
	})
	t.Run("centrality keeps an otherwise quiet origin", func(t *testing.T) {
		facts := append(base(),
			F("doc_link", "x.md", "a.md"),
			F("doc_link", "y.md", "a.md"),
			F("doc_link", "z.md", "a.md"),
		)
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_evolved_into", "a.md", "b.md")
		if predCount(t, e, "doc_superseded") != 0 {
			t.Fatal("superseded a central document")
		}
		mustRow(t, e, "doc_live", "a.md")
		mustRow(t, e, "origin_source", "a.md", "/birth_era")
	})
}

func TestPolicy_VisionWeightsARecentDraftAboveAQuietOrigin(t *testing.T) {
	t.Parallel()
	facts := append(threeMonths("/no"),
		histFact("old.md", 10, 20, 1, 1),
		docFact("old.md", ""),
		F("doc_role_claim", "old.md", N("/vision"), int64(90)),
		histFact("draft.md", 250, 280, 1, 1),
		docFact("draft.md", ""),
		F("doc_role_claim", "draft.md", N("/north_star_draft"), int64(70)),
		histFact("burst.md", 260, 270, 6, 1),
		docFact("burst.md", ""),
		F("doc_role_claim", "burst.md", N("/north_star_draft"), int64(70)),
	)
	e := evalFacts(t, nil, facts)
	mustRow(t, e, "vision_source", "old.md", "56", "/origin_vision")
	mustRow(t, e, "vision_source", "draft.md", "73", "/recent_draft")
	mustRow(t, e, "vision_source", "burst.md", "93", "/recent_burst_draft")
}

func TestPolicy_ReadBudgetAndTieBreak(t *testing.T) {
	t.Parallel()
	t.Run("budget keeps the higher reasons and lists the rest", func(t *testing.T) {
		cfg := config.DefaultOrientConfig()
		cfg.ReadCandidateBudget = 2
		facts := []types.Fact{
			spanFact(0, 1000, 3, "/no"),
			monthFact(0, "m", 5),
			mspan(0, 0, 1000),
			F("agent_source", "i", N("/claude"), N("/instructions"), "AGENTS.md", "AGENTS.md", N("/yes")),
			tieFact("AGENTS.md", 0),
			histFact("o.md", 0, 0, 1, 1),
			docFact("o.md", ""),
			tieFact("o.md", 1),
			histFact("b.md", 900, 900, 6, 1),
			docFact("b.md", ""),
			tieFact("b.md", 2),
		}
		e := evalFacts(t, &cfg, facts)
		mustRow(t, e, "orient_read_candidate", "AGENTS.md", "/instructions")
		mustRow(t, e, "orient_read_candidate", "o.md", "/origin")
		refuseRow(t, e, "orient_read_candidate", "b.md", "/burst")
		mustRow(t, e, "orient_read_omitted", "b.md", "/burst")
		if predCount(t, e, "orient_read_kept") != 2 {
			t.Fatalf("kept %d", predCount(t, e, "orient_read_kept"))
		}
	})
	t.Run("equal scores cut on the earlier path", func(t *testing.T) {
		cfg := config.DefaultOrientConfig()
		cfg.ReadCandidateBudget = 1
		facts := []types.Fact{
			spanFact(0, 100, 2, "/no"),
			histFact("p-early.md", 10, 10, 6, 1),
			histFact("p-late.md", 10, 10, 6, 1),
			docFact("p-early.md", ""),
			docFact("p-late.md", ""),
			tieFact("p-early.md", 0),
			tieFact("p-late.md", 1),
		}
		e := evalFacts(t, &cfg, facts)
		mustRow(t, e, "orient_read_candidate", "p-early.md", "/burst")
		mustRow(t, e, "orient_read_omitted", "p-late.md", "/burst")
		if predCount(t, e, "orient_read_kept") != 1 {
			t.Fatalf("kept %d, want 1", predCount(t, e, "orient_read_kept"))
		}
	})
}

func TestPolicy_NamesDoNotSelect(t *testing.T) {
	t.Parallel()
	facts := []types.Fact{
		spanFact(0, 1000, 2, "/no"),
		monthFact(0, "m", 5),
		mspan(0, 0, 1000),
		histFact("docs/vision/north-star.md", 500, 500, 1, 1),
		docFact("docs/vision/north-star.md", "docs/vision"),
		tieFact("docs/vision/north-star.md", 0),
		histFact("zzz-notes.md", 0, 0, 1, 1),
		docFact("zzz-notes.md", ""),
		tieFact("zzz-notes.md", 1),
	}
	e := evalFacts(t, nil, facts)
	mustRow(t, e, "doc_generation", "docs/vision/north-star.md", "/middle")
	mustRow(t, e, "origin_source", "zzz-notes.md", "/span_position")
	refuseRow(t, e, "orient_read_candidate", "docs/vision/north-star.md", "/origin")
	mustRow(t, e, "orient_read_candidate", "zzz-notes.md", "/origin")
	if predCount(t, e, "orient_read_candidate") != 1 {
		rows, _ := e.Query("orient_read_candidate")
		t.Fatalf("candidates %s", dumpRows(rows))
	}
}

func TestPolicy_InstructionRoots(t *testing.T) {
	t.Parallel()
	facts := []types.Fact{
		F("agent_source", "i1", N("/claude"), N("/instructions"), "AGENTS.md", "AGENTS.md", N("/yes")),
		F("agent_source", "i2", N("/claude"), N("/instructions"), "RULES.md", "sub/RULES.md", N("/yes")),
		F("agent_source_scope", "i2", "other"),
		docFact("sub/RULES.md", "sub"),
		F("agent_source", "i3", N("/cursor"), N("/instructions"), "RULES.md", "nest/RULES.md", N("/yes")),
		F("agent_source_scope", "i3", "nest"),
		docFact("nest/RULES.md", "nest"),
	}
	e := evalFacts(t, nil, facts)
	mustRow(t, e, "orient_read_candidate", "AGENTS.md", "/instructions")
	mustRow(t, e, "orient_read_candidate", "nest/RULES.md", "/instructions")
	refuseRow(t, e, "read_reason", "sub/RULES.md", "/instructions")
}

func TestPolicy_ClusterRepresentative(t *testing.T) {
	t.Parallel()
	t.Run("higher centrality wins", func(t *testing.T) {
		facts := []types.Fact{
			F("doc_cluster", "a.md", "a.md"),
			F("doc_cluster", "b.md", "a.md"),
			tieFact("a.md", 0),
			tieFact("b.md", 1),
			F("doc_link", "x.md", "b.md"),
			F("doc_link", "y.md", "b.md"),
			F("doc_link", "z.md", "b.md"),
		}
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "cluster_rep", "b.md")
		refuseRow(t, e, "cluster_rep", "a.md")
	})
	t.Run("equal centrality keeps the earlier path", func(t *testing.T) {
		facts := []types.Fact{
			F("doc_cluster", "a.md", "a.md"),
			F("doc_cluster", "b.md", "a.md"),
			tieFact("a.md", 0),
			tieFact("b.md", 1),
		}
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "cluster_rep", "a.md")
		refuseRow(t, e, "cluster_rep", "b.md")
	})
}

func TestPolicy_ShallowStillReadsInstructions(t *testing.T) {
	t.Parallel()
	facts := append(threeMonths("/yes"),
		histFact("a.md", 10, 20, 6, 1),
		docFact("a.md", ""),
		F("agent_source", "i", N("/claude"), N("/instructions"), "AGENTS.md", "AGENTS.md", N("/yes")),
	)
	e := evalFacts(t, nil, facts)
	mustRow(t, e, "orient_read_candidate", "AGENTS.md", "/instructions")
	if predCount(t, e, "repo_era") != 0 || predCount(t, e, "doc_burst") != 0 {
		t.Fatal("shallow clone drew eras or a burst")
	}
}
