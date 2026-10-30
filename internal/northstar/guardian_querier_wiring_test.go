package northstar

import (
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// querierKernel implements both KernelClient and FactQuerier.
type querierKernel struct {
	facts map[string][]types.Fact
}

func (q *querierKernel) Assert(fact types.Fact) error   { return nil }
func (q *querierKernel) Retract(predicate string) error { return nil }
func (q *querierKernel) Query(predicate string) ([]types.Fact, error) {
	if q.facts == nil {
		return nil, nil
	}
	if v, ok := q.facts[predicate]; ok {
		out := make([]types.Fact, len(v))
		copy(out, v)
		return out, nil
	}
	return nil, nil
}

// simpleKernel implements only KernelClient, not FactQuerier.
type simpleKernel struct{}

func (s *simpleKernel) Assert(fact types.Fact) error   { return nil }
func (s *simpleKernel) Retract(predicate string) error { return nil }

func TestBuildCampaignObserver_WiresQuerierWhenSupported(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	kern := &querierKernel{
		facts: map[string][]types.Fact{
			"effective_module_purpose": {
				{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session module purpose"}},
			},
		},
	}
	observer := BuildCampaignObserver(tmp, nil, kern)
	if observer == nil {
		t.Fatal("BuildCampaignObserver returned nil, expected observer")
	}
	if observer.guardian == nil {
		t.Fatal("guardian is nil")
	}
	// Close DB to allow TempDir cleanup on Windows (sqlite file lock).
	if observer.guardian.store != nil {
		t.Cleanup(func() { _ = observer.guardian.store.Close() })
	}
	if observer.guardian.querier == nil {
		t.Fatal("expected guardian querier to be wired when kernel implements FactQuerier")
	}
	if observer.guardian.kernel == nil {
		t.Fatal("expected guardian kernel to be set")
	}
}

func TestBuildCampaignObserver_DoesNotPanicWithoutQuerier(t *testing.T) {
	t.Parallel()

	// Nil kernel should not panic and should leave querier nil.
	t.Run("nil kernel", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildCampaignObserver panicked with nil kernel: %v", r)
			}
		}()
		observer := BuildCampaignObserver(tmp, nil, nil)
		if observer == nil {
			t.Fatal("BuildCampaignObserver returned nil for nil kernel, expected observer")
		}
		if observer.guardian.store != nil {
			t.Cleanup(func() { _ = observer.guardian.store.Close() })
		}
		if observer.guardian.querier != nil {
			t.Fatal("querier should be nil when kernel is nil")
		}
	})

	// Kernel that does not implement FactQuerier should not panic and should leave querier nil.
	t.Run("non-querier kernel", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("BuildCampaignObserver panicked with non-querier kernel: %v", r)
			}
		}()
		kern := &simpleKernel{}
		observer := BuildCampaignObserver(tmp, nil, kern)
		if observer == nil {
			t.Fatal("BuildCampaignObserver returned nil, expected observer")
		}
		if observer.guardian.store != nil {
			t.Cleanup(func() { _ = observer.guardian.store.Close() })
		}
		if observer.guardian.querier != nil {
			t.Fatal("querier should be nil when kernel does not implement FactQuerier")
		}
		if observer.guardian.kernel == nil {
			t.Fatal("kernel should still be set even when querier is not wired")
		}
	})
}

func TestGuardian_QuerierWiringMakesAlignmentPromptDifference(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	vision := &Vision{
		Mission:    "Project mission",
		Problem:    "Project problem",
		VisionStmt: "Project vision",
	}

	q := &querierKernel{
		facts: map[string][]types.Fact{
			"effective_module_purpose": {
				{Predicate: "effective_module_purpose", Args: []any{"internal/session", "Session module purpose"}},
			},
			"module_requirement": {
				{Predicate: "module_requirement", Args: []any{"internal/session", "REQ-1", "Requirement one", types.MangleAtom("/blocker")}},
			},
		},
	}

	// A scoped assessment without a querier is explicitly incomplete.
	gWithout := NewGuardian(store, DefaultGuardianConfig())
	coverage, _ := gWithout.resolveAlignmentScope(ManualAlignmentScope(filepath.Dir(filepath.Dir(store.Path())), "internal/session/executor.go"), vision)
	if coverage.Complete || len(coverage.Issues) == 0 {
		t.Fatalf("missing querier concealed: %+v", coverage)
	}

	// Same guardian shape with querier set produces prompt containing governing module's purpose.
	gWith := NewGuardian(store, DefaultGuardianConfig())
	gWith.SetQuerier(q)
	_, promptWith := alignmentPrompts(t, gWith, vision, "internal/session/executor.go")
	if !strings.Contains(promptWith, "Session module purpose") {
		t.Fatalf("with querier, prompt should contain module purpose; got:\n%s", promptWith)
	}
	if !strings.Contains(promptWith, "Project mission") {
		t.Fatalf("module prompt must still contain project vision")
	}
	if !strings.Contains(promptWith, "REQ-1") || !strings.Contains(promptWith, "Requirement one") {
		t.Fatalf("module requirements should be included when querier is wired; got:\n%s", promptWith)
	}

}

func (q *querierKernel) QueryAll() (map[string][]types.Fact, error) {
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
