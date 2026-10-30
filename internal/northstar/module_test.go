package northstar

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

type fakeQuerier struct {
	facts map[string][]types.Fact
}

func (f *fakeQuerier) Query(predicate string) ([]types.Fact, error) {
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

func TestEffectiveModulePurpose_ReturnsPurposeForDeclared(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session purpose"}},
			{Predicate: "effective_module_purpose", Args: []any{"internal", "Internal purpose"}},
		},
	}}
	got, err := EffectiveModulePurpose(q, "internal/session")
	if err != nil {
		t.Fatalf("EffectiveModulePurpose error: %v", err)
	}
	if got != "Session purpose" {
		t.Fatalf("got %q want %q", got, "Session purpose")
	}
}

func TestEffectiveModulePurpose_ReturnsEmptyForUndeclared(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session purpose"}},
		},
	}}
	got, err := EffectiveModulePurpose(q, "internal/other")
	if err != nil {
		t.Fatalf("EffectiveModulePurpose error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty for undeclared module, got %q", got)
	}
}

func TestModuleForPath_PicksLongestMatchingModule(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"internal", "Internal purpose"}},
			{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session purpose"}},
		},
	}}
	got, err := ModuleForPath(q, "internal/session/executor.go")
	if err != nil {
		t.Fatalf("ModuleForPath error: %v", err)
	}
	if got != "internal/session" {
		t.Fatalf("got %q want %q", got, "internal/session")
	}
}

func TestModuleForPath_DoesNotMatchSegmentPrefix(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session purpose"}},
		},
	}}
	got, err := ModuleForPath(q, "internal/sessionx/foo.go")
	if err != nil {
		t.Fatalf("ModuleForPath error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected no match for sessionx, got %q", got)
	}
}

func TestModuleForPath_ReturnsEmptyWhenNothingGoverns(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session purpose"}},
		},
	}}
	got, err := ModuleForPath(q, "cmd/nerd/main.go")
	if err != nil {
		t.Fatalf("ModuleForPath error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty when nothing governs, got %q", got)
	}
}

func TestBuildAlignmentSystemPrompt_IncludesModulePurposeWhenApplies(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())
	vision := &Vision{
		Mission:    "Project mission",
		Problem:    "Project problem",
		VisionStmt: "Project vision",
	}
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session module purpose"}},
		},
		"module_requirement": {
			{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "Requirement one", types.MangleAtom("/blocker")}},
		},
	}}
	g.SetQuerier(q)

	withoutSystem, without := alignmentPrompts(t, g, vision)
	withSystem, with := alignmentPrompts(t, g, vision, "internal/session/executor.go")
	refinement := "the module\nREFINES the project vision"
	if strings.Contains(withoutSystem, refinement) || !strings.Contains(withSystem, refinement) {
		t.Fatalf("the module-refinement atom must be compiled exactly when a module governs the subject")
	}

	if strings.Contains(without, "Session module purpose") {
		t.Fatalf("without module should not contain module purpose")
	}
	if !strings.Contains(with, "Session module purpose") {
		t.Fatalf("with module should contain module purpose, got:\n%s", with)
	}
	if !strings.Contains(with, "Project mission") {
		t.Fatalf("module prompt must still contain project vision")
	}
	if !strings.Contains(with, "REQ-1") || !strings.Contains(with, "Requirement one") {
		t.Fatalf("module requirements should be included, got:\n%s", with)
	}
}

func TestBuildAlignmentSystemPrompt_NoDeclaredModuleKeepsProjectVision(t *testing.T) {
	t.Parallel()
	g := NewGuardian(newTestStore(t), DefaultGuardianConfig())
	g.SetQuerier(&fakeQuerier{})
	vision := &Vision{Mission: "Project mission", Problem: "Project problem", VisionStmt: "Project vision"}
	system, user := alignmentPrompts(t, g, vision, "internal/session/executor.go")
	if !strings.Contains(user, "Project mission") || !strings.Contains(user, "No declared module: internal/session/executor.go") {
		t.Fatalf("missing project or unmatched-path context: %s", user)
	}
	if strings.Contains(system, "the module\nREFINES the project vision") || strings.Contains(user, "## Module Northstar") {
		t.Fatal("an unmatched path must not invent module context")
	}
}

func TestModuleRequirementsFor_FiltersByModule(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{facts: map[string][]types.Fact{
		"module_requirement": {
			{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "Statement 1", types.MangleAtom("/blocker")}},
			{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-2", "Statement 2", types.MangleAtom("/major")}},
			{Predicate: "module_requirement", Args: []any{"internal/other", "REQ-3", "Other", types.MangleAtom("/minor")}},
		},
	}}
	reqs, err := ModuleRequirementsFor(q, "internal/session")
	if err != nil {
		t.Fatalf("ModuleRequirementsFor error: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requirements for internal/session, got %d", len(reqs))
	}
	if reqs[0].ID != "REQ-1" || reqs[0].Severity != "blocker" {
		t.Fatalf("unexpected req[0]: %+v", reqs[0])
	}
}

func (q *fakeQuerier) QueryAll() (map[string][]types.Fact, error) {
	out := map[string][]types.Fact{}
	for _, pred := range []string{"module_northstar", "effective_module_purpose", "module_requirement"} {
		facts, err := q.Query(pred)
		if err != nil {
			return nil, err
		}
		out[pred] = facts
	}
	return out, nil
}
