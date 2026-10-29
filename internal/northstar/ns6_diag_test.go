package northstar

import (
	"testing"

	"codenerd/internal/types"
)

type fakeQuerier2 struct {
	facts map[string][]types.Fact
}

func (f *fakeQuerier2) Query(predicate string) ([]types.Fact, error) {
	if f.facts == nil {
		return nil, nil
	}
	if v, ok := f.facts[predicate]; ok {
		out := make([]types.Fact, len(v))
		copy(out, v)
		return out, nil
	}
	return nil, nil
}

func TestDiagNS6Relevance(t *testing.T) {
	// blocker => 0.9
	t.Run("blocker", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement":       {{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/blocker")}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{} // clear defaults to isolate module logic
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.9 {
			t.Fatalf("blocker expected 0.9 got %v", got)
		}
	})
	t.Run("major", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement":       {{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/major")}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.7 {
			t.Fatalf("major expected 0.7 got %v", got)
		}
	})
	t.Run("major mixed with minor still major", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement": {
				{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/minor")}},
				{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-2", "stmt", types.MangleAtom("/major")}},
			},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.7 {
			t.Fatalf("major mixed expected 0.7 got %v", got)
		}
	})
	t.Run("minor only", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement":       {{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/minor")}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.6 {
			t.Fatalf("minor expected 0.6 got %v", got)
		}
	})
	t.Run("unspecified", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement":       {{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/unspecified")}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.6 {
			t.Fatalf("unspecified expected 0.6 got %v", got)
		}
	})
	t.Run("empty severity", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement":       {{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", ""}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.6 {
			t.Fatalf("empty severity expected 0.6 got %v", got)
		}
	})
	t.Run("purpose no requirements", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.6 {
			t.Fatalf("purpose no req expected 0.6 got %v", got)
		}
	})
	t.Run("no module", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/other", "purpose"}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.5 {
			t.Fatalf("no module expected 0.5 got %v", got)
		}
	})
	t.Run("highimpact overrides minor", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement":       {{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/minor")}}},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{"internal/session/"}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.9 {
			t.Fatalf("highimpact should override minor to 0.9 got %v", got)
		}
	})
	t.Run("no querier returns legacy", func(t *testing.T) {
		g := NewGuardian(nil, GuardianConfig{HighImpactPaths: []string{"internal/session/"}})
		// no querier set
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.9 {
			t.Fatalf("no querier highimpact expected 0.9 got %v", got)
		}
		if got := g.calculatePathRelevance("internal/other/foo.go"); got != 0.5 {
			t.Fatalf("no querier normal expected 0.5 got %v", got)
		}
		// Even with module facts that would otherwise be 0.9, no querier must stay 0.5
		// This tests the same paths that previously gave module scores, but without querier they must be 0.5
	})
	t.Run("blocker over major", func(t *testing.T) {
		q := &fakeQuerier2{facts: map[string][]types.Fact{
			"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"internal/session", "purpose"}}},
			"module_requirement": {
				{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "stmt", types.MangleAtom("/major")}},
				{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-2", "stmt", types.MangleAtom("/blocker")}},
			},
		}}
		g := NewGuardian(nil, DefaultGuardianConfig())
		g.config.HighImpactPaths = []string{}
		g.SetQuerier(q)
		if got := g.calculatePathRelevance("internal/session/foo.go"); got != 0.9 {
			t.Fatalf("blocker over major expected 0.9 got %v", got)
		}
	})
}
