package prompt_evolution

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/prompt"

	"gopkg.in/yaml.v3"
)

// writeStoredAtom plants an atom file as an earlier version would have left it.
func writeStoredAtom(t *testing.T, nerdDir, state string, w evolvedAtomWrapper) string {
	t.Helper()
	dir := filepath.Join(nerdDir, "prompts", "evolved", state)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, atomFileName(w.Atom.ID))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func legacyWrapper(id string, providers, models, roles []string) evolvedAtomWrapper {
	return evolvedAtomWrapper{
		Atom: &prompt.PromptAtom{
			ID: id, Category: prompt.CategoryMethodology, Content: "guidance " + id,
			Providers: providers, Models: models, ShardTypes: roles,
		},
		Source: "failure_analysis", CreatedAt: time.Now(),
	}
}

func newMigrationEvolver(t *testing.T, dir string, tweak func(*EvolverConfig)) *PromptEvolver {
	return newLifecycleEvolver(t, dir, &countingLLM{}, tweak)
}

// Rule 4: a stored family-pinned, role-widened atom with a recorded serving
// pin is re-pinned to the exact model and the source role, in memory and on
// disk (where the JIT loader reads it).
func TestStoredAtomsAreRepinnedFromTheirRecord(t *testing.T) {
	dir := t.TempDir()
	w := legacyWrapper("evolved/repin", []string{"anthropic"}, []string{"claude_opus_4"}, []string{"coder", "tester"})
	w.ShardType, w.Provider, w.Model, w.PinScope = "/coder", "anthropic", "claude-opus-4-20260501", PinScopeModelFamily
	path := writeStoredAtom(t, dir, "promoted", w)

	pe := newMigrationEvolver(t, dir, nil)
	defer pe.Close()

	ga := pe.evolvedAtoms["evolved/repin"]
	if ga == nil {
		t.Fatal("re-pinnable atom was dropped")
	}
	if !equalStrings(ga.Atom.Models, []string{"claude_opus_4_20260501"}) ||
		!equalStrings(ga.Atom.ShardTypes, []string{"coder"}) ||
		!equalStrings(ga.Atom.Providers, []string{"anthropic"}) {
		t.Errorf("selectors = models %v roles %v providers %v", ga.Atom.Models, ga.Atom.ShardTypes, ga.Atom.Providers)
	}
	if ga.PinScope != PinScopeModel || ga.PromotedAt.IsZero() {
		t.Errorf("PinScope=%q promoted=%v; a re-pin must keep the lifecycle state", ga.PinScope, !ga.PromotedAt.IsZero())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("promoted file gone: %v", err)
	}
	var onDisk evolvedAtomWrapper
	if err := yaml.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if !equalStrings(onDisk.Atom.Models, []string{"claude_opus_4_20260501"}) || !equalStrings(onDisk.Atom.ShardTypes, []string{"coder"}) {
		t.Errorf("file not rewritten: models %v roles %v", onDisk.Atom.Models, onDisk.Atom.ShardTypes)
	}
}

// Rule 4: where the record cannot say which model or role, the atom is moved
// aside -- unselected, kept on disk, listed -- not deleted and not served.
func TestStoredAtomsWithoutRecordedPinAreQuarantined(t *testing.T) {
	dir := t.TempDir()
	noModel := legacyWrapper("evolved/no-model", []string{"anthropic"}, nil, []string{"coder"})
	noModel.ShardType, noModel.Provider = "/coder", "anthropic"
	noRole := legacyWrapper("evolved/no-role", nil, []string{"claude_opus_4"}, nil)
	noRole.Provider, noRole.Model = "anthropic", "claude-opus-4"
	bare := legacyWrapper("evolved/bare", nil, nil, nil)
	promotedPath := writeStoredAtom(t, dir, "promoted", noModel)
	pendingPath := writeStoredAtom(t, dir, "pending", noRole)
	writeStoredAtom(t, dir, "pending", bare)

	pe := newMigrationEvolver(t, dir, nil)
	defer pe.Close()

	if got := len(pe.GetEvolvedAtoms()); got != 0 {
		t.Errorf("%d unpinnable atoms still selectable", got)
	}
	listed := map[string]bool{}
	for _, u := range pe.UnpinnedAtoms() {
		listed[u.ID] = true
		if _, err := os.Stat(u.Path); err != nil {
			t.Errorf("%s not kept on disk: %v", u.ID, err)
		}
		if u.Reason == "" {
			t.Errorf("%s listed without a reason", u.ID)
		}
	}
	for _, id := range []string{"evolved/no-model", "evolved/no-role", "evolved/bare"} {
		if !listed[id] {
			t.Errorf("%s not listed as unpinned", id)
		}
	}
	for _, p := range []string{promotedPath, pendingPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s still in a directory the JIT loader reads", p)
		}
	}
	if got := pe.GetStats().AtomsUnpinned; got != 3 {
		t.Errorf("AtomsUnpinned = %d, want 3", got)
	}

	// The listing survives a restart even though the atoms are no longer loaded.
	pe.Close()
	again := newMigrationEvolver(t, dir, nil)
	defer again.Close()
	if got := len(again.UnpinnedAtoms()); got != 3 {
		t.Errorf("after restart %d unpinned listed, want 3", got)
	}
	if got := len(again.GetEvolvedAtoms()); got != 0 {
		t.Errorf("after restart %d atoms selectable, want 0", got)
	}
}

// An atom already pinned as the rules require is left alone (no rewrite).
func TestStoredAtomsAlreadyPinnedAreUntouched(t *testing.T) {
	dir := t.TempDir()
	w := legacyWrapper("evolved/ok", []string{"anthropic"}, []string{"claude_opus_4_20260501"}, []string{"coder"})
	w.ShardType, w.Provider, w.Model, w.PinScope = "/coder", "anthropic", "claude-opus-4-20260501", PinScopeModel
	path := writeStoredAtom(t, dir, "pending", w)
	before, _ := os.ReadFile(path)

	pe := newMigrationEvolver(t, dir, nil)
	defer pe.Close()

	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("a correctly pinned atom was rewritten")
	}
	if len(pe.UnpinnedAtoms()) != 0 || len(pe.GetEvolvedAtoms()) != 1 {
		t.Errorf("unpinned=%d atoms=%d", len(pe.UnpinnedAtoms()), len(pe.GetEvolvedAtoms()))
	}
}

// Rule 3 in the cycle: a failure group with no recorded provider/model buys no
// generation, produces no atom, is counted, and its evidence is consumed so
// it is not refused again every cycle.
func TestCycleRefusesUnpinnedGroupAndCountsIt(t *testing.T) {
	llm := &countingLLM{}
	pe := newMigrationEvolver(t, t.TempDir(), nil)
	defer pe.Close()
	pe.atomGenerator.llmClient = llm

	rec := failRecord("unpinned-1", "debugging", time.Unix(1700000000, 0))
	rec.Provider, rec.Model = "", ""
	if err := pe.RecordExecution(rec); err != nil {
		t.Fatal(err)
	}

	result, err := pe.RunEvolutionCycle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.GroupsWithoutPin != 1 || result.AtomsGenerated != 0 {
		t.Errorf("GroupsWithoutPin=%d AtomsGenerated=%d, want 1 and 0", result.GroupsWithoutPin, result.AtomsGenerated)
	}
	if llm.generations.Load() != 0 || result.GenerationCalls != 0 {
		t.Errorf("generation bought for unpinnable evidence: calls=%d", result.GenerationCalls)
	}
	if result.EvidenceConsumed != 1 {
		t.Errorf("EvidenceConsumed = %d, want 1", result.EvidenceConsumed)
	}
	again, _ := pe.RunEvolutionCycle(context.Background())
	if again.GroupsWithoutPin != 0 {
		t.Errorf("refused evidence came back next cycle: %d", again.GroupsWithoutPin)
	}
}
