package autopoiesis

import (
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

func TestToolLedger_RecordsOffersCallsAndVerification(t *testing.T) {
	ps := NewProfileStore(t.TempDir())
	ps.NoteToolOffers([]types.ToolOffer{
		{Tool: "grep", Origin: types.ToolOriginBuiltin, SessionID: "s1", CatalogTokens: 100, AtomTokens: 20},
		{Tool: "grep", Origin: types.ToolOriginBuiltin, SessionID: "s1", CatalogTokens: 100},
		{Tool: "grep", Origin: types.ToolOriginBuiltin, SessionID: "s2", CatalogTokens: 100},
	})
	ps.NoteToolCall(types.ToolCallMeasure{
		Tool: "grep", Channel: types.ToolChannelNative, SessionID: "s1", Success: true,
		CallTokens: 12, ResultTokens: 40, ResultBytes: 160,
		Consumed: types.ToolConsumption{Known: true, Bytes: 40000, Items: 9},
	})
	ps.NoteToolCall(types.ToolCallMeasure{
		Tool: "grep", Channel: types.ToolChannelPiggyback, SessionID: "s1",
		CallTokens: 10, ResultTokens: 5, ResultBytes: 20,
	})
	ps.NoteToolVerification(types.ToolVerification{Tool: "edit_file", Verifier: "run_build", Tokens: 70})

	l := ps.ToolLedgers()["grep"]
	if l.Offers != 3 || l.OfferSessions != 2 {
		t.Errorf("offers %d in %d sessions, want 3 in 2 (a session counts once)", l.Offers, l.OfferSessions)
	}
	if l.CatalogTokens != 300 || l.AtomTokens != 20 {
		t.Errorf("prompt tokens catalog=%d atoms=%d, want 300 and 20", l.CatalogTokens, l.AtomTokens)
	}
	if l.Calls != 2 || l.CallSessions != 1 || l.Successes != 1 {
		t.Errorf("calls=%d sessions=%d successes=%d, want 2, 1, 1", l.Calls, l.CallSessions, l.Successes)
	}
	// Only the call that reported a scan is condensation evidence; the other is
	// counted as unknown, not as a scan of zero.
	if l.KnownCalls != 1 || l.UnknownCalls != 1 || l.ConsumedBytes != 40000 || l.ReturnedBytes != 160 {
		t.Errorf("known=%d unknown=%d consumed=%d returned=%d", l.KnownCalls, l.UnknownCalls, l.ConsumedBytes, l.ReturnedBytes)
	}
	if l.KnownCallTokens != 52 {
		t.Errorf("known call tokens %d, want 52 (call+result of the measured call only)", l.KnownCallTokens)
	}
	if l.Channels["native"] != 1 || l.Channels["piggyback"] != 1 {
		t.Errorf("channels %v", l.Channels)
	}
	if v := ps.ToolLedgers()["edit_file"]; v.VerifyCalls != 1 || v.VerifyTokens != 70 {
		t.Errorf("verification not charged to the write: %+v", v)
	}
}

func TestToolLedger_SessionlessMeasurementsNeverInflateSessions(t *testing.T) {
	ps := NewProfileStore(t.TempDir())
	ps.NoteToolOffers([]types.ToolOffer{{Tool: "t"}, {Tool: "t"}, {Tool: "t"}})
	if l := ps.ToolLedgers()["t"]; l.Offers != 3 || l.OfferSessions != 0 {
		t.Errorf("offers=%d sessions=%d: offers without a session id are offers, not sessions", l.Offers, l.OfferSessions)
	}
}

func TestToolLedger_PersistsAcrossStores(t *testing.T) {
	dir := t.TempDir()
	NewProfileStore(dir).NoteToolCall(types.ToolCallMeasure{Tool: "glob", SessionID: "s", ResultTokens: 7})
	if l := NewProfileStore(dir).ToolLedgers()["glob"]; l.Calls != 1 || l.ResultTokens != 7 {
		t.Errorf("a new store did not read the ledger back: %+v", l)
	}
}

func TestToolLedger_TwoProcessesDoNotLoseEachOthersCalls(t *testing.T) {
	dir := t.TempDir()
	a, b := NewProfileStore(dir), NewProfileStore(dir)
	a.NoteToolCall(types.ToolCallMeasure{Tool: "glob", SessionID: "a"})
	b.NoteToolCall(types.ToolCallMeasure{Tool: "glob", SessionID: "b"})
	a.NoteToolCall(types.ToolCallMeasure{Tool: "glob", SessionID: "a"})
	if l := NewProfileStore(dir).ToolLedgers()["glob"]; l.Calls != 3 {
		t.Errorf("calls %d, want 3: the second store overwrote the first's write", l.Calls)
	}
}

func TestToolLedger_LedgerOnlyEntryIsNotAProfile(t *testing.T) {
	ps := NewProfileStore(t.TempDir())
	ps.NoteToolCall(types.ToolCallMeasure{Tool: "grep", SessionID: "s"})
	if p := ps.GetProfile("grep"); p != nil {
		t.Fatalf("a built-in's ledger came back as quality expectations: %+v", p)
	}
	// Setting expectations later keeps what the tool already cost.
	ps.SetProfile(GetDefaultProfile("grep", ToolTypeQuickCalculation))
	if ps.GetProfile("grep") == nil {
		t.Fatal("the profile just set is missing")
	}
	if l := ps.ToolLedgers()["grep"]; l.Calls != 1 {
		t.Errorf("SetProfile erased the ledger: %+v", l)
	}
}

func TestToolLedger_OfferAfterRetirementStartsOver(t *testing.T) {
	ps := NewProfileStore(t.TempDir())
	ps.NoteToolCall(types.ToolCallMeasure{Tool: "gen", Origin: types.ToolOriginGenerated, SessionID: "s"})
	ps.MarkToolRetired("gen")
	snap := ToolLedgerSnapshot{Ledgers: ps.ToolLedgers(), Generated: []string{"gen"}}
	for _, f := range ToolLedgerFacts(snap) {
		if f.Predicate == "tool_use_count" {
			t.Fatalf("a retired tool still has facts: %v", f)
		}
	}
	ps.NoteToolOffers([]types.ToolOffer{{Tool: "gen", Origin: types.ToolOriginGenerated, SessionID: "s2"}})
	l := ps.ToolLedgers()["gen"]
	if !l.RetiredAt.IsZero() || l.Calls != 0 || l.Offers != 1 {
		t.Errorf("a regenerated tool inherited its predecessor's record: %+v", l)
	}
}

func TestToolLedgerFacts_NoCondensationFactWithoutAReportedScan(t *testing.T) {
	snap := ToolLedgerSnapshot{
		Ledgers: map[string]ToolLedger{"bash": {Calls: 40, UnknownCalls: 40}},
		Catalog: []CatalogTool{{Name: "bash", Capability: tools.CapabilityOverlapping}},
	}
	for _, f := range ToolLedgerFacts(snap) {
		if f.Predicate == "tool_condensation" {
			t.Fatalf("a tool that never reported a scan has a condensation fact: %v", f)
		}
	}
}

func testKernel(t *testing.T) types.Kernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	return k
}

func valueSnapshot() ToolLedgerSnapshot {
	day := time.Now()
	return ToolLedgerSnapshot{
		Catalog: []CatalogTool{
			{Name: "grep", Capability: tools.CapabilityOverlapping},
			{Name: "write_file", Capability: tools.CapabilityUnique},
			{Name: "bash", Capability: tools.CapabilityOverlapping},
			{Name: "find_text", Capability: tools.CapabilityOverlapping},
			{Name: "glob", Capability: tools.CapabilityOverlapping},
		},
		Generated: []string{"gen_slow", "gen_new", "gen_gone"},
		Ledgers: map[string]ToolLedger{
			// Condenses 100x and levers far past its cost: earns.
			"grep": {Offers: 60, OfferSessions: 6, CatalogTokens: 600, Calls: 50, CallSessions: 5,
				CallTokens: 2500, ResultTokens: 5000, KnownCalls: 50, ConsumedBytes: 4_000_000,
				ReturnedBytes: 40_000, KnownCallTokens: 7500},
			// Unique and used: earns on capability, with no scan to measure.
			"write_file": {Offers: 60, OfferSessions: 6, CatalogTokens: 500, Calls: 3, CallSessions: 2,
				CallTokens: 100, ResultTokens: 100, VerifyCalls: 3, VerifyTokens: 50},
			// Overlapping and offered in 12 sessions without a call: dead.
			"bash": {Offers: 100, OfferSessions: 12, CatalogTokens: 3000},
			// Measured over enough calls but between the bars: marginal.
			"find_text": {Offers: 40, OfferSessions: 4, CatalogTokens: 400, Calls: 25, CallSessions: 4,
				CallTokens: 500, ResultTokens: 4000, KnownCalls: 25, ConsumedBytes: 100_000,
				ReturnedBytes: 20_000, KnownCallTokens: 4500},
			// Never offered or called: unknown, not dead.
			"glob": {},
			// Generated, 30 measured calls that cost more than the work: dead, retire.
			"gen_slow": {Origin: types.ToolOriginGenerated, Offers: 40, OfferSessions: 4, CatalogTokens: 4000,
				Calls: 30, CallSessions: 4, CallTokens: 900, ResultTokens: 3000, KnownCalls: 30,
				ConsumedBytes: 12_000, ReturnedBytes: 11_000, KnownCallTokens: 3900},
			// Generated, two calls: too early to say.
			"gen_new":  {Origin: types.ToolOriginGenerated, Offers: 3, OfferSessions: 1, CatalogTokens: 100, Calls: 2, CallSessions: 1},
			"gen_gone": {Origin: types.ToolOriginGenerated, Calls: 99, RetiredAt: day},
		},
	}
}

func TestEvaluateToolValue_ClassifiesFromMeasurements(t *testing.T) {
	report, err := EvaluateToolValue(testKernel(t), valueSnapshot(), config.DefaultToolValueConfig())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]ToolValueClass{
		"grep": ToolEarning, "write_file": ToolEarning, "bash": ToolDead, "find_text": ToolMarginal,
		"glob": ToolUnknown, "gen_slow": ToolDead, "gen_new": ToolUnknown,
	}
	got := map[string]ToolValueClass{}
	for _, r := range report.Rows {
		got[r.Tool] = r.Class
	}
	for tool, class := range want {
		if got[tool] != class {
			t.Errorf("%s classed %q, want %q", tool, got[tool], class)
		}
	}
	if _, ok := got["gen_gone"]; ok {
		t.Error("a retired tool is still in the report")
	}
}

func TestEvaluateToolValue_BuiltInsAreFlaggedGeneratedAreProposed(t *testing.T) {
	report, err := EvaluateToolValue(testKernel(t), valueSnapshot(), config.DefaultToolValueConfig())
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, p := range report.Proposals {
		kinds[p.Tool] = p.Kind + ":" + p.Reason
	}
	if kinds["bash"] != "flag:never_called" {
		t.Errorf("bash proposal %q, want a flag: a built-in is never proposed for removal", kinds["bash"])
	}
	if kinds["gen_slow"] != "retire:cost_exceeds_value" {
		t.Errorf("gen_slow proposal %q, want retire:cost_exceeds_value", kinds["gen_slow"])
	}
	for _, p := range report.Proposals {
		if p.Kind == "retire" && p.Tool == "bash" {
			t.Error("a built-in was proposed for removal")
		}
	}
	if len(report.Proposals) != 2 {
		t.Errorf("proposals %v, want exactly bash and gen_slow", report.Proposals)
	}
}

func TestEvaluateToolValue_ThresholdsComeFromConfig(t *testing.T) {
	cfg := config.DefaultToolValueConfig()
	cfg.MinCalls = 1000 // grep's 50 measured calls are no longer evidence
	cfg.MinSessions = 100
	report, err := EvaluateToolValue(testKernel(t), valueSnapshot(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range report.Rows {
		switch r.Tool {
		case "grep":
			if r.Class != ToolUnknown {
				t.Errorf("grep classed %q with min_calls 1000, want unknown", r.Class)
			}
		case "bash":
			if r.Class != ToolUnknown {
				t.Errorf("bash classed %q with min_sessions 100, want unknown", r.Class)
			}
		}
	}
	if len(report.Proposals) != 0 {
		t.Errorf("proposals %v with thresholds nothing meets", report.Proposals)
	}
}

func TestEvaluateToolValue_JevEvidenceIsReadBesideTheLedger(t *testing.T) {
	k := testKernel(t)
	if err := k.Assert(types.Fact{Predicate: "tool_value_evidence", Args: []any{"gen_new", types.MangleAtom("/duplicate")}}); err != nil {
		t.Fatal(err)
	}
	report, err := EvaluateToolValue(k, valueSnapshot(), config.DefaultToolValueConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range report.Rows {
		if r.Tool == "gen_new" && (r.Class != ToolDead || len(r.DeadReasons) != 1 || r.DeadReasons[0] != "redundant") {
			t.Errorf("gen_new %+v: Jev's duplicate judgment should make it dead as redundant", r)
		}
	}
	found := false
	for _, p := range report.Proposals {
		found = found || (p.Tool == "gen_new" && p.Kind == "retire" && p.Reason == "redundant")
	}
	if !found {
		t.Errorf("no retire proposal for the duplicate: %v", report.Proposals)
	}
}
