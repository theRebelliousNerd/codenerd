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

func cohortFixture() []types.Fact {
	facts := []types.Fact{spanFact(0, 86400*500, 120, "/no")}
	for i := 0; i < 120; i++ {
		p, dir, day := fmt.Sprintf("other/d%d/p.md", i), fmt.Sprintf("other/d%d", i), int64(100+i*4)
		if i < 12 {
			p, dir, day = fmt.Sprintf("packet/p%02d.md", i), "packet", 10
		}
		facts = append(facts, docFact(p, dir), histFact(p, day*86400, day*86400, 1, 1),
			F("doc_subtree", p, dir), F("repo_file_day", p, day), tieFact(p, i))
		if i < 11 {
			facts = append(facts, F("doc_link", p, "packet/p11.md"))
		}
	}
	return facts
}

func TestPolicy_CohortIsSmallCohesiveAndScoredOnce(t *testing.T) {
	t.Parallel()
	e := evalFacts(t, nil, cohortFixture())
	if predCount(t, e, "doc_cohort") != 12 || predCount(t, e, "doc_burst") != 12 {
		t.Fatal("sparse one-day document act did not become a burst cohort")
	}
	mustRow(t, e, "cohort_rep", "packet/p00.md", "packet/p11.md")
	mustRow(t, e, "orient_read_candidate", "packet/p11.md", "/burst")
	if predCount(t, e, "orient_read_kept") != 1 || predCount(t, e, "orient_read_member") != 11 {
		t.Fatal("cohort consumed more than its one representative slot")
	}
	rep, err := buildReport(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.CohortGroups) != 1 || len(rep.CohortGroups[0].Members) != 12 || !rep.CohortGroups[0].Burst {
		t.Fatalf("cohort unit was not reported: %+v", rep.CohortGroups)
	}
	mustRow(t, e, "read_contribution", "packet/p00.md", "/burst", "49")
	mustRow(t, e, "read_contribution", "packet/p11.md", "/burst", "49")
}

func TestPolicy_CohortRejectsBroadImportsAndDispersedBirths(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"broad", "dispersed", "unrelated", "share ceiling", "spread touches"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			facts := cohortFixture()
			if mode == "share ceiling" {
				cfg.CohortShareCeilingPermille = 99
			}
			for i := range facts {
				f := &facts[i]
				p := types.ExtractString(f.Args[0])
				if mode == "broad" {
					switch f.Predicate {
					case "repo_file_history":
						f.Args[1], f.Args[2] = int64(86400*10), int64(86400*10)
					case "doc_subtree":
						f.Args[1] = "all"
					}
				}
				if mode == "dispersed" && strings.HasPrefix(p, "packet/") {
					if f.Predicate == "repo_file_history" {
						day := int64(10 + i*4)
						f.Args[1], f.Args[2] = day*86400, day*86400
					}
				}
				if mode == "unrelated" {
					if f.Predicate == "doc_subtree" {
						f.Args[1] = p
					}
					// Existing links all point to the representative and are
					// not reciprocal, so they cannot establish cohesion.
				}
				if mode == "spread touches" && strings.HasPrefix(p, "packet/") && f.Predicate == "repo_file_day" {
					f.Args[1] = int64(10 + i*3)
				}
			}
			e := evalFacts(t, &cfg, facts)
			if mode == "spread touches" {
				if predCount(t, e, "doc_cohort") != 12 || predCount(t, e, "doc_burst") != 0 {
					t.Fatal("an unconcentrated cohort was called a burst")
				}
			} else if predCount(t, e, "doc_cohort") != 0 {
				t.Fatal("weak cohort evidence was accepted")
			}
		})
	}
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
		mustRow(t, e, "doc_generation", "old.md", "/early")
		refuseRow(t, e, "doc_generation", "old.md", "/origin")
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
		mustRow(t, e, "doc_generation", "o.md", "/early")
		mustRow(t, e, "doc_generation", "m.md", "/middle")
		mustRow(t, e, "doc_generation", "r.md", "/recent")
	})
	t.Run("zero width peripheral document is early", func(t *testing.T) {
		facts := []types.Fact{
			spanFact(5, 5, 1, "/no"),
			monthFact(0, "m", 1),
			mspan(0, 0, 100),
			histFact("z.md", 5, 5, 1, 1),
			docFact("z.md", ""),
		}
		e := evalFacts(t, nil, facts)
		mustRow(t, e, "doc_generation", "z.md", "/early")
		refuseRow(t, e, "origin_source", "z.md", "/zero_span")
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
	weights := map[string]int64{}
	rows, err := e.Query("vision_source")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		weights[types.ExtractString(row.Args[0])], _ = types.ExtractInt64(row.Args[1])
	}
	if !(weights["burst.md"] > weights["draft.md"] && weights["draft.md"] > weights["old.md"]) {
		t.Fatalf("vision ordering %+v", weights)
	}
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
			F("doc_link", "x.md", "o.md"),
			F("doc_link", "y.md", "o.md"),
			F("doc_link", "z.md", "o.md"),
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
	mustRow(t, e, "doc_generation", "zzz-notes.md", "/early")
	refuseRow(t, e, "origin_source", "zzz-notes.md", "/span_position")
	refuseRow(t, e, "orient_read_candidate", "docs/vision/north-star.md", "/origin")
	refuseRow(t, e, "orient_read_candidate", "zzz-notes.md", "/origin")
	if predCount(t, e, "orient_read_candidate") != 0 {
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

func TestPolicy_RareReasonsOutrankNinetyPercentSharedReasons(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultOrientConfig()
	cfg.ReadCandidateBudget = 1
	facts := []types.Fact{spanFact(0, 1000, 100, "/no")}
	for i := 0; i < 100; i++ {
		p := fmt.Sprintf("d%03d.md", i)
		commits := int64(1)
		if i < 90 {
			commits = 6
		}
		facts = append(facts, docFact(p, ""), histFact(p, 500, 500, commits, 1), tieFact(p, i))
	}
	facts = append(facts, F("agent_source", "rare", N("/codex"), N("/instructions"), "root", "d099.md", N("/yes")))
	e := evalFacts(t, &cfg, facts)
	mustRow(t, e, "reason_document_count", "/burst", "90")
	mustRow(t, e, "reason_rarity", "/burst", "100")
	mustRow(t, e, "reason_rarity", "/instructions", "990")
	mustRow(t, e, "orient_read_candidate", "d099.md", "/instructions")
	refuseRow(t, e, "orient_read_candidate", "d000.md", "/burst")
	cfg.ReasonRarityFloorPermille = 250
	withFloor := evalFacts(t, &cfg, facts)
	mustRow(t, withFloor, "reason_rarity", "/burst", "250")
}

func TestPolicy_EqualWeightReasonsBothContribute(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultOrientConfig()
	cfg.ReadWeightInstructions, cfg.ReadWeightLinkHub = 60, 60
	facts := []types.Fact{
		docFact("root.md", ""), docFact("other.md", ""),
		F("agent_source", "i", N("/codex"), N("/instructions"), "root", "root.md", N("/yes")),
		F("doc_link", "a.md", "root.md"), F("doc_link", "b.md", "root.md"), F("doc_link", "c.md", "root.md"),
	}
	e := evalFacts(t, &cfg, facts)
	mustRow(t, e, "read_score", "root.md", "60")
}

func TestPolicy_DuplicateComponentsShareOneSlot(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"digest", "similarity", "transitive similarity", "below threshold"} {
		t.Run(mode, func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			facts := []types.Fact{spanFact(0, 1000, 20, "/no")}
			for i, p := range []string{"a.md", "b.md", "c.md", "different.md"} {
				facts = append(facts, docFact(p, ""), histFact(p, 500, 500, 6, 1), tieFact(p, i))
			}
			switch mode {
			case "digest":
				for _, p := range []string{"a.md", "b.md", "c.md"} {
					facts = append(facts, F("doc_body_digest", p, "same-body"))
				}
			case "similarity", "transitive similarity":
				facts = append(facts, F("doc_similar", "a.md", "b.md", int64(975)), F("doc_similar", "b.md", "c.md", int64(975)))
				if mode == "similarity" {
					facts = append(facts, F("doc_similar", "a.md", "c.md", int64(975)))
				}
			case "below threshold":
				facts = append(facts, F("doc_similar", "a.md", "b.md", int64(949)))
			}
			e := evalFacts(t, &cfg, facts)
			want := 2
			if mode == "below threshold" {
				want = 4
			}
			if predCount(t, e, "orient_read_kept") != want {
				t.Fatalf("read units = %d, want %d", predCount(t, e, "orient_read_kept"), want)
			}
			if mode != "below threshold" {
				if predCount(t, e, "orient_read_member") != 2 {
					t.Fatal("duplicate members not recorded")
				}
			}
		})
	}
}

func TestPolicy_OriginsRequireStructuralWitnesses(t *testing.T) {
	t.Parallel()
	for _, witness := range []string{"none", "descendant", "links", "embedding hub", "root instruction", "subtree instruction"} {
		t.Run(witness, func(t *testing.T) {
			p, dir := "old.md", ""
			if witness == "subtree instruction" {
				p, dir = "sub/root.md", "sub"
			}
			facts := append(threeMonths("/no"), docFact(p, dir), histFact(p, 10, 250, 2, 2))
			switch witness {
			case "descendant":
				facts = append(facts, docFact("later.md", ""), histFact("later.md", 260, 280, 1, 1), F("doc_similar", "later.md", p, int64(900)))
			case "links":
				for _, from := range []string{"a", "b", "c"} {
					facts = append(facts, F("doc_link", from, p))
				}
			case "embedding hub":
				for i := 0; i < 15; i++ {
					other := fmt.Sprintf("neighbor-%02d.md", i)
					facts = append(facts, F("doc_similar", other, p, int64(900)))
				}
			case "root instruction", "subtree instruction":
				facts = append(facts, F("agent_source", "i", N("/codex"), N("/instructions"), "root", p, N("/yes")), F("agent_source_scope", "i", dir))
			}
			e := evalFacts(t, nil, facts)
			if witness == "none" {
				mustRow(t, e, "doc_generation", p, "/early")
				refuseRow(t, e, "doc_generation", p, "/origin")
			} else {
				mustRow(t, e, "doc_generation", p, "/origin")
				mustRow(t, e, "origin_source", p, "/birth_era")
			}
		})
	}
}

func TestPolicy_LullsUseMonthlyMedianAndAbsoluteFloor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		commits      []int
		share, floor int
		want         []string
	}{
		{"quiet five percent", []int{100, 5, 100}, 100, 1, []string{"/wave", "/lull", "/wave"}},
		{"relative config changes the verdict", []int{100, 5, 100}, 40, 1, []string{"/wave", "/wave", "/wave"}},
		{"absolute floor still wins", []int{100, 5, 100}, 40, 6, []string{"/wave", "/lull", "/wave"}},
		{"even median", []int{100, 5, 300, 100}, 100, 1, []string{"/wave", "/lull", "/wave", "/wave"}},
		{"half integer median", []int{1, 2}, 700, 1, []string{"/lull", "/wave"}},
		{"all zero", []int{0, 0, 0}, 100, 1, []string{"/lull", "/lull", "/lull"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultOrientConfig()
			cfg.EraLullMedianPermille, cfg.EraLullCommits = tc.share, tc.floor
			facts := []types.Fact{spanFact(0, 1000, 10, "/no")}
			for i, commits := range tc.commits {
				facts = append(facts, monthFact(i, fmt.Sprintf("m%d", i), commits))
			}
			e := evalFacts(t, &cfg, facts)
			for i, kind := range tc.want {
				mustRow(t, e, "month_kind", fmt.Sprint(i), kind)
			}
		})
	}
}

func TestPolicy_CohortSpansCalendarBoundaryWithoutChainingWindows(t *testing.T) {
	t.Parallel()
	facts := cohortFixture()
	for i := range facts {
		f := &facts[i]
		p := types.ExtractString(f.Args[0])
		if !strings.HasPrefix(p, "packet/") {
			continue
		}
		day := int64(1)
		if p >= "packet/p06.md" {
			day = 2
		}
		switch f.Predicate {
		case "repo_file_history":
			f.Args[1], f.Args[2] = day*86400, day*86400
		case "repo_file_day":
			f.Args[1] = day
		}
	}
	e := evalFacts(t, nil, facts)
	if predCount(t, e, "doc_cohort") != 12 || predCount(t, e, "doc_burst") != 12 || predCount(t, e, "cohort_rep") != 1 {
		t.Fatal("a two-day act was split at an arbitrary calendar boundary")
	}
	// A four-day chain never becomes one unbounded cohort, even with
	// reciprocal links across adjacent births.
	for i := range facts {
		f := &facts[i]
		p := types.ExtractString(f.Args[0])
		if !strings.HasPrefix(p, "packet/") {
			continue
		}
		var member int
		if _, err := fmt.Sscanf(p, "packet/p%d.md", &member); err != nil {
			t.Fatal(err)
		}
		day := int64(member/3 + 1)
		switch f.Predicate {
		case "repo_file_history":
			f.Args[1], f.Args[2] = day*86400, day*86400
		case "repo_file_day":
			f.Args[1] = day
		}
	}
	for i := 0; i < 11; i++ {
		a, b := fmt.Sprintf("packet/p%02d.md", i), fmt.Sprintf("packet/p%02d.md", i+1)
		facts = append(facts, F("doc_link", a, b), F("doc_link", b, a))
	}
	noLongChain := evalFacts(t, nil, facts)
	if predCount(t, noLongChain, "doc_cohort") != 0 {
		t.Fatal("short windows chained into a long cohort")
	}
}

func TestPolicy_DirectoryCohortSurvivesBroadSimilarity(t *testing.T) {
	t.Parallel()
	facts := cohortFixture()
	for i := range facts {
		f := &facts[i]
		switch f.Predicate {
		case "repo_file_history":
			f.Args[1], f.Args[2] = int64(86400*10), int64(86400*10)
		case "repo_file_day":
			f.Args[1] = int64(10)
		}
	}
	for i := 12; i < 120; i++ {
		facts = append(facts, F("doc_similar", fmt.Sprintf("other/d%d/p.md", i), "packet/p00.md", int64(900)))
	}
	e := evalFacts(t, nil, facts)
	if predCount(t, e, "doc_cohort") != 12 {
		t.Fatal("broad similarity swallowed a qualified directory act")
	}
	mustRow(t, e, "cohort_rep", "packet/p00.md", "packet/p11.md")
}
