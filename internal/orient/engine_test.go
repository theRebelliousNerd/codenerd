package orient

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

func TestEngine_ParamsMatchRequired(t *testing.T) {
	t.Parallel()
	e := mustEngine(t, nil)
	rows, err := e.Query("config_param_required")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range rows {
		if len(f.Args) != 2 || types.ExtractString(f.Args[0]) != "/orient" {
			t.Fatalf("required row %v", f.Args)
		}
		got[types.ExtractString(f.Args[1])] = true
	}
	for _, p := range config.DefaultOrientConfig().Params() {
		if !got[p.Key] {
			t.Errorf("param %s is not config_param_required", p.Key)
		}
		delete(got, p.Key)
	}
	if len(got) != 0 {
		t.Fatalf("required without a param: %v", got)
	}
	held, err := e.Query("config_param")
	if err != nil {
		t.Fatal(err)
	}
	if len(held) != 13 {
		t.Fatalf("config_param rows %d, want 13", len(held))
	}
	missing, err := e.Query("config_param_missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing %s", dumpRows(missing))
	}
	weights, err := e.Query("reason_weight")
	if err != nil {
		t.Fatal(err)
	}
	if len(weights) != 8 {
		t.Fatalf("reason weights %d, want 8 (%s)", len(weights), dumpRows(weights))
	}
}

func TestEngine_RefusesMissingParam(t *testing.T) {
	t.Parallel()
	params := config.DefaultOrientConfig().Params()
	params = params[:len(params)-1]
	_, err := newEngine(params)
	if err == nil || !strings.Contains(err.Error(), "/orient_middle_span_permille") {
		t.Fatalf("err %v", err)
	}
}

func TestEngine_RefusesBadConfig(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultOrientConfig()
	cfg.ReadCandidateBudget = -1
	if _, err := NewEngine(&cfg); err == nil {
		t.Fatal("accepted a negative budget")
	}
	if _, err := NewEngine(nil); err == nil {
		t.Fatal("accepted a nil config")
	}
}

func TestEngine_AssertEvaluateQuery(t *testing.T) {
	t.Parallel()
	facts := []types.Fact{
		spanFact(0, 100, 1, "/no"),
		histFact("a.md", 10, 10, 6, 1),
		docFact("a.md", ""),
	}
	one := evalFacts(t, nil, facts)
	mustRow(t, one, "doc_burst", "a.md")

	split := mustEngine(t, nil)
	if err := split.Assert(facts[:1]); err != nil {
		t.Fatal(err)
	}
	if n := predCount(t, split, "doc_burst"); n != 0 {
		t.Fatalf("derived before evaluate: %d", n)
	}
	// The extensional row is stored as soon as it is asserted.
	if n := predCount(t, split, "repo_span"); n != 1 {
		t.Fatalf("base facts before evaluate: %d", n)
	}
	if err := split.Assert(facts[1:]); err != nil {
		t.Fatal(err)
	}
	if err := split.Evaluate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := split.Evaluate(context.Background()); err != nil {
		t.Fatal(err)
	}
	mustRow(t, split, "doc_burst", "a.md")
	mustRow(t, split, "repo_span", "0", "100", "1", "/no")

	if _, err := split.Query(""); err == nil {
		t.Fatal("empty predicate")
	}
	if _, err := split.Query("not_a_predicate"); err == nil {
		t.Fatal("undeclared predicate")
	}
	if err := split.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := split.Query("doc_burst"); err == nil {
		t.Fatal("query after close")
	}
}
