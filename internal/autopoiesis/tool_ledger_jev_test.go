package autopoiesis

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// tool_ledger_jev_test.go: Jev's readings persist in the tool ledger, weigh
// hits against misses in policy/tool_value.mg, and survive a restart.

func classOf(t *testing.T, s ToolLedgerSnapshot, tool string) ToolValueRow {
	t.Helper()
	report, err := EvaluateToolValue(testKernel(t), s, config.DefaultToolValueConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range report.Rows {
		if r.Tool == tool {
			return r
		}
	}
	t.Fatalf("%s is not in the report", tool)
	return ToolValueRow{}
}

// A generated tool with calls but no reported scan and no token figures to
// measure: only Jev's reading can say anything about it.
func jevSnapshot(origin types.ToolOrigin, capability tools.Capability, calls int64, jev ToolJevEvidence) ToolLedgerSnapshot {
	s := ToolLedgerSnapshot{
		Ledgers: map[string]ToolLedger{"t": {Origin: origin, Calls: calls, CallSessions: 3, Offers: 5, OfferSessions: 3, Jev: jev}},
	}
	if origin == types.ToolOriginGenerated {
		s.Generated = []string{"t"}
	} else {
		s.Catalog = []CatalogTool{{Name: "t", Capability: capability}}
	}
	return s
}

func TestToolValue_HitsMarkEarnsWithoutAScan(t *testing.T) {
	row := classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 20, ToolJevEvidence{Landed: 18, Missed: 2}), "t")
	if row.Class != ToolEarning {
		t.Fatalf("a tool whose judged calls landed 90%% of the time classed %q, want earning (%+v)", row.Class, row)
	}
	if row.CondensationPercent != nil || row.LeveragePercent != nil {
		t.Fatalf("the figures were never measured and must stay unknown: %+v", row)
	}
	if !reflect.DeepEqual(row.Evidence, []string{"hits_mark"}) {
		t.Fatalf("evidence %v, want hits_mark", row.Evidence)
	}
}

func TestToolValue_HitsBelowTheBarsDoNotEarn(t *testing.T) {
	for name, jev := range map[string]ToolJevEvidence{
		"too few judged calls": {Landed: 5},
		"hit rate under bar":   {Landed: 15, Missed: 5}, // 75% < 80%
	} {
		row := classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 20, jev), "t")
		if row.Class != ToolMarginal {
			t.Errorf("%s: classed %q, want marginal (Jev read it, it did not clear the bars)", name, row.Class)
		}
	}
	// Judged more than the tool was called is not evidence: the calls are the
	// real thing.
	row := classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 4, ToolJevEvidence{Landed: 12}), "t")
	if row.Class == ToolEarning {
		t.Errorf("12 judged hits over 4 recorded calls earned: %+v", row)
	}
}

func TestToolValue_MissesOutweighingHitsBlockEarning(t *testing.T) {
	// More misses than hits: dead for an overlapping tool, however it measures.
	row := classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 20, ToolJevEvidence{Landed: 8, Missed: 12}), "t")
	if row.Class != ToolDead || !reflect.DeepEqual(row.DeadReasons, []string{"misses_results"}) {
		t.Errorf("misses outweighing hits classed %q %v, want dead misses_results", row.Class, row.DeadReasons)
	}
	// A unique tool is not killed by misses but is no longer earning.
	row = classOf(t, jevSnapshot(types.ToolOriginBuiltin, tools.CapabilityUnique, 20, ToolJevEvidence{Landed: 8, Missed: 12}), "t")
	if row.Class != ToolMarginal {
		t.Errorf("a unique tool with misses outweighing hits classed %q, want marginal", row.Class)
	}
	// Too few judged calls to be dead; still not earning.
	row = classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 20, ToolJevEvidence{Missed: 3}), "t")
	if row.Class != ToolMarginal {
		t.Errorf("3 misses classed %q, want marginal: under min_judged_calls is not dead", row.Class)
	}
	// Condensation that would earn is blocked by the misses.
	s := jevSnapshot(types.ToolOriginBuiltin, tools.CapabilityOverlapping, 50,
		ToolJevEvidence{Landed: 4, Missed: 6})
	l := s.Ledgers["t"]
	l.KnownCalls, l.ConsumedBytes, l.ReturnedBytes, l.KnownCallTokens, l.CatalogTokens = 50, 4_000_000, 40_000, 7500, 600
	s.Ledgers["t"] = l
	if row := classOf(t, s, "t"); row.Class == ToolEarning {
		t.Errorf("a condensing tool whose judged calls mostly missed still earns: %+v", row)
	}
	// Equal hits and misses name neither side.
	row = classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 20, ToolJevEvidence{Landed: 10, Missed: 10}), "t")
	if len(row.Evidence) != 0 {
		t.Errorf("a tie gave evidence %v", row.Evidence)
	}
}

func TestToolValue_HitsOutweighOldMissesAcrossSessions(t *testing.T) {
	// One bad week does not brand a tool forever: the whole history is weighed.
	row := classOf(t, jevSnapshot(types.ToolOriginGenerated, "", 60, ToolJevEvidence{Landed: 50, Missed: 5}), "t")
	if row.Class != ToolEarning {
		t.Errorf("50 hits to 5 misses classed %q, want earning", row.Class)
	}
}

func TestToolLedger_JevEvidenceSurvivesAReloadAndRederivesTheClass(t *testing.T) {
	dir := t.TempDir()
	a := NewProfileStore(dir)
	for i := 0; i < 20; i++ {
		a.NoteToolCall(types.ToolCallMeasure{Tool: "t", Origin: types.ToolOriginGenerated, SessionID: "s"})
	}
	for i := 0; i < 11; i++ {
		a.NoteJevCall("t", JevCallLanded)
	}
	a.NoteJevCall("t", JevCallMissed)
	a.NoteJevCall("t", JevCallUnsettled)
	a.NoteJevVerdicts([]JevVerdictWrite{{Tool: "t", Kind: JevVerdictHarnessCovers,
		ToolJevVerdict: ToolJevVerdict{Yes: false, Percent: 12}}})
	before := a.ToolLedgers()["t"]

	reloaded := NewProfileStore(dir).ToolLedgers()["t"]
	if !reflect.DeepEqual(before.Jev, reloaded.Jev) {
		t.Fatalf("Jev evidence changed across a reload:\n before %+v\n after  %+v", before.Jev, reloaded.Jev)
	}
	j := reloaded.Jev
	if j.Landed != 11 || j.Missed != 1 || j.Unsettled != 1 || j.LastLanded.IsZero() || j.LastMissed.IsZero() || !j.Harness.Answered() {
		t.Fatalf("tallies or recency lost: %+v", j)
	}

	snap := func(l map[string]ToolLedger) ToolLedgerSnapshot {
		return ToolLedgerSnapshot{Ledgers: l, Generated: []string{"t"}}
	}
	first := classOf(t, snap(a.ToolLedgers()), "t")
	second := classOf(t, snap(NewProfileStore(dir).ToolLedgers()), "t")
	if first.Class != ToolEarning || first.Class != second.Class || !reflect.DeepEqual(first.Evidence, second.Evidence) {
		t.Fatalf("class before %q %v, after reload %q %v; want the same, earning", first.Class, first.Evidence, second.Class, second.Evidence)
	}
}

func TestToolLedger_JevVerdictIsReplacedByTheLaterReading(t *testing.T) {
	dir := t.TempDir()
	a, b := NewProfileStore(dir), NewProfileStore(dir)
	a.NoteJevVerdicts([]JevVerdictWrite{{Tool: "t", Kind: JevVerdictDuplicate,
		ToolJevVerdict: ToolJevVerdict{Yes: true, Percent: 90, Other: "u", At: time.Now().Add(-time.Hour)}}})
	b.NoteJevVerdicts([]JevVerdictWrite{{Tool: "t", Kind: JevVerdictDuplicate,
		ToolJevVerdict: ToolJevVerdict{Yes: false, Percent: 10, Other: "u"}}})
	v := NewProfileStore(dir).ToolLedgers()["t"].Jev.Duplicate
	if v.Yes || v.Percent != 10 {
		t.Fatalf("an older verdict outlived the later reading: %+v", v)
	}
	facts := JevFacts(NewProfileStore(dir).ToolLedgers())
	if len(facts) != 0 {
		t.Fatalf("a withdrawn verdict is still replayed: %v", facts)
	}
}

func TestToolLedger_ConcurrentWritersMergeJevEvidence(t *testing.T) {
	dir := t.TempDir()
	stores := []*ProfileStore{NewProfileStore(dir), NewProfileStore(dir), NewProfileStore(dir)}
	const perStore = 15
	var wg sync.WaitGroup
	for _, ps := range stores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perStore; i++ {
				ps.NoteJevCall("t", JevCallLanded)
				ps.NoteJevCall("t", JevCallMissed)
				ps.NoteToolCall(types.ToolCallMeasure{Tool: "t"})
			}
		}()
	}
	wg.Wait()
	l := NewProfileStore(dir).ToolLedgers()["t"]
	want := int64(len(stores) * perStore)
	if l.Jev.Landed != want || l.Jev.Missed != want || l.Calls != want {
		t.Fatalf("landed %d missed %d calls %d, want %d each: a writer's counts were lost", l.Jev.Landed, l.Jev.Missed, l.Calls, want)
	}
}

// A restart: the orchestrator attaches a kernel and the evidence earlier
// processes persisted is derived again, with no Jev call.
func TestOrchestrator_SetKernelReloadsPersistedJevEvidence(t *testing.T) {
	dir := t.TempDir()
	ps := NewProfileStore(dir)
	for i := 0; i < 3; i++ {
		ps.NoteJevCall("good", JevCallLanded)
	}
	ps.NoteJevCall("bad", JevCallMissed)
	ps.NoteJevVerdicts([]JevVerdictWrite{{Tool: "dup", Kind: JevVerdictDuplicate,
		ToolJevVerdict: ToolJevVerdict{Yes: true, Percent: 91, Other: "good"}}})

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{profiles: NewProfileStore(dir), toolGen: NewToolGenerator(nil, t.TempDir())}
	o.SetKernel(k)
	got, err := readToolValueEvidence(k)
	if err != nil {
		t.Fatal(err)
	}
	want := []ToolValueEvidence{
		{Tool: "bad", Evidence: []string{"/misses_mark"}},
		{Tool: "dup", Evidence: []string{"/duplicate"}},
		{Tool: "good", Evidence: []string{"/hits_mark"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("evidence after boot = %+v, want %+v", got, want)
	}
}
