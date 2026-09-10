package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/jsonl"
	"codenerd/internal/prompt"

	"github.com/spf13/cobra"
)

// meterFixture writes a receipt log and a selection log into a temp workspace
// and points the command's flags at it.
//
// The logs are written through the real writers rather than by hand, so the
// test exercises the same serialization the agent uses. A fixture that hand-
// rolls the on-disk shape passes happily while the two halves drift apart.
func meterFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	prev := meterWorkspace
	prevJSON := meterJSON
	meterWorkspace = root
	t.Cleanup(func() { meterWorkspace = prev; meterJSON = prevJSON })

	sink, err := broker.NewFileSink(filepath.Join(root, ".nerd", broker.DefaultReceiptLogName))
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	mk := func(purpose broker.Purpose, scope, prefix string, at time.Time, est int, in, out, cached int64) broker.Receipt {
		r := broker.Receipt{
			Purpose:  purpose,
			Provider: "anthropic",
			Model:    "claude-opus-5",
			Method:   "CompleteWithTools",
			Started:  at,
			Duration: 900 * time.Millisecond,
			Scope:    scope,
			Prefix:   prefix,
			Estimated: broker.Count{
				Tokens:     est,
				Confidence: broker.ConfidenceExact,
				Segments:   broker.Segments{System: est / 2, Tools: est / 4, History: est / 8, User: est / 8},
			},
			Actual:   broker.Spend{InputTokens: in, OutputTokens: out, CachedTokens: cached, Calls: 1},
			Decision: broker.Decision{Allowed: true, Code: broker.DecisionAdmitted, Window: 200000},
		}
		if in > 0 {
			r.EstimateErrorPct = float64(est-int(in)) / float64(in) * 100
		}
		return r
	}

	// One session: four calls on a stable prefix, then a prefix change.
	for i := 0; i < 4; i++ {
		sink.Record(mk(broker.PurposeSession, "sess-a", "prefixA",
			base.Add(time.Duration(i)*time.Second), 1000, 1010, 300, 400))
	}
	sink.Record(mk(broker.PurposeSession, "sess-a", "prefixB", base.Add(10*time.Second), 1000, 990, 300, 0))

	// A second session, and other purposes, so the by-purpose table has rows.
	sink.Record(mk(broker.PurposeCompression, "sess-b", "prefixC", base.Add(20*time.Second), 500, 505, 100, 0))
	sink.Record(mk(broker.PurposeSubagent, "sess-b", "prefixD", base.Add(30*time.Second), 800, 760, 250, 0))
	sink.Record(mk(broker.PurposeUnattributed, "", "prefixE", base.Add(40*time.Second), 300, 300, 50, 0))

	// A refusal: no tokens, but the reason must reach the readout.
	refused := mk(broker.PurposeSession, "sess-a", "prefixA", base.Add(50*time.Second), 900000, 0, 0, 0)
	refused.Decision = broker.Decision{
		Allowed: false, Code: broker.DecisionWindowExceeded,
		Reason: "counted 900000 tokens against a 200000 window", Window: 200000,
	}
	refused.Actual = broker.Spend{}
	sink.Record(refused)

	if err := sink.Close(); err != nil {
		t.Fatalf("close receipt log: %v", err)
	}

	// Selection log: two atoms always together, two others always together.
	selLog, err := jsonl.Open(filepath.Join(root, ".nerd", prompt.DefaultSelectionLogName))
	if err != nil {
		t.Fatalf("open selection log: %v", err)
	}
	for i := 0; i < 30; i++ {
		atoms := []prompt.RecordedAtom{
			{ID: "identity/core", Category: "identity"},
			{ID: "arch/layering", Category: "architecture"},
			{ID: "arch/boundaries", Category: "architecture"},
		}
		if i%2 == 1 {
			atoms = []prompt.RecordedAtom{
				{ID: "identity/core", Category: "identity"},
				{ID: "impl/errors", Category: "knowledge"},
				{ID: "verify/tests", Category: "verification"},
			}
		}
		selLog.Append(prompt.SelectionRecord{At: base.Add(time.Duration(i) * time.Minute), Outcome: prompt.OutcomeSuccess, Atoms: atoms})
	}
	if err := selLog.Close(); err != nil {
		t.Fatalf("close selection log: %v", err)
	}

	return root
}

// meterRunE resolves the RunE body for a meter subcommand.
//
// The bodies are invoked directly rather than through Execute because cobra's
// Execute always dispatches from the root, which drags in the root command's
// TTY-dependent hooks. Registration and flag wiring are covered separately by
// TestMeterCommandIsRegistered, so nothing goes unchecked.
func meterRunE(t *testing.T, args []string) func(*cobra.Command, []string) error {
	t.Helper()
	if len(args) == 0 {
		return meterCmd.RunE
	}
	switch args[0] {
	case "epochs":
		return meterEpochsCmd.RunE
	case "atoms":
		return meterAtomsCmd.RunE
	}
	t.Fatalf("unknown meter subcommand %q", args[0])
	return nil
}

func runMeter(t *testing.T, args ...string) string {
	t.Helper()

	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetErr(&buf)

	if err := meterRunE(t, args)(c, nil); err != nil {
		t.Fatalf("nerd meter %v: %v\noutput:\n%s", args, err, buf.String())
	}
	return buf.String()
}

func TestMeterCommandIsRegistered(t *testing.T) {
	var found *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "meter" {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("meter is not registered on the root command; the readout is unreachable")
	}

	subs := map[string]bool{}
	for _, c := range found.Commands() {
		subs[c.Name()] = true
	}
	for _, want := range []string{"epochs", "atoms"} {
		if !subs[want] {
			t.Errorf("meter is missing the %q subcommand", want)
		}
	}
	for _, want := range []string{"workspace", "json"} {
		if found.PersistentFlags().Lookup(want) == nil {
			t.Errorf("meter is missing the --%s flag", want)
		}
	}
}

func TestMeterSummaryReadsTheLog(t *testing.T) {
	meterFixture(t)
	meterJSON = false

	out := runMeter(t)

	for _, want := range []string{"session", "compression", "subagent", "unattributed", "window_exceeded"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}
	// A refusal costs no tokens. If it were counted as spend the readout would
	// inflate the very number the meter exists to make trustworthy.
	if strings.Contains(out, "900.0k") {
		t.Errorf("the refused request's counted size was reported as spend:\n%s", out)
	}
}

func TestMeterSummaryJSONAccounting(t *testing.T) {
	meterFixture(t)
	meterJSON = true

	var s MeterSummary
	if err := json.Unmarshal([]byte(runMeter(t)), &s); err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	if s.Receipts != 9 {
		t.Fatalf("receipts = %d, want 9", s.Receipts)
	}
	if s.Admitted != 8 || s.Refused != 1 {
		t.Fatalf("admitted/refused = %d/%d, want 8/1", s.Admitted, s.Refused)
	}
	// 4*1010 + 990 + 505 + 760 + 300
	const wantInput = 4*1010 + 990 + 505 + 760 + 300
	if s.InputTokens != wantInput {
		t.Fatalf("input tokens = %d, want %d", s.InputTokens, wantInput)
	}
	if s.CachedTokens != 4*400 {
		t.Fatalf("cached tokens = %d, want %d", s.CachedTokens, 4*400)
	}
	if s.RefusalsByCode["window_exceeded"] != 1 {
		t.Fatalf("refusals by code = %+v", s.RefusalsByCode)
	}
	if s.UnattributedTokens != 350 {
		t.Fatalf("unattributed = %d, want 350 — untagged spend must be reported, not hidden", s.UnattributedTokens)
	}

	shares := 0.0
	for _, p := range s.ByPurpose {
		shares += p.Share
	}
	if shares < 99.9 || shares > 100.1 {
		t.Fatalf("purpose shares sum to %.2f%%, want 100%%", shares)
	}
}

func TestMeterAccuracyUsesMeanAbsoluteError(t *testing.T) {
	meterFixture(t)
	meterJSON = true

	var s MeterSummary
	if err := json.Unmarshal([]byte(runMeter(t)), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(s.ByModel) != 1 {
		t.Fatalf("models = %d, want 1", len(s.ByModel))
	}

	m := s.ByModel[0]
	// The fixture's errors are deliberately mixed-sign, so a report headlining
	// net bias would score a visibly imprecise estimator as near perfect.
	if m.MeanAbsErrorPct <= 0 {
		t.Fatalf("mean absolute error = %v, want a positive value", m.MeanAbsErrorPct)
	}
	if m.MeanAbsErrorPct < absFloat(m.MeanBiasPct) {
		t.Fatalf("mean abs error %v is below |mean bias| %v, which is arithmetically impossible",
			m.MeanAbsErrorPct, m.MeanBiasPct)
	}
	if m.ExactCounts != 8 {
		t.Fatalf("exact counts = %d, want 8", m.ExactCounts)
	}
}

func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func TestMeterEpochsSegmentsBySession(t *testing.T) {
	meterFixture(t)
	meterJSON = true

	var h broker.EpochHistogram
	if err := json.Unmarshal([]byte(runMeter(t, "epochs")), &h); err != nil {
		t.Fatalf("decode histogram: %v", err)
	}

	// sess-a: prefixA x4, prefixB x1, prefixA x1 (refused, excluded) = 2 epochs
	// sess-b: prefixC, prefixD = 2 epochs
	// untagged scope: prefixE = 1 epoch
	if h.Epochs != 5 {
		t.Fatalf("epochs = %d, want 5", h.Epochs)
	}
	if h.Calls != 8 {
		t.Fatalf("calls = %d, want 8 (the refusal is excluded)", h.Calls)
	}
	if h.Max != 4 {
		t.Fatalf("max = %d, want 4", h.Max)
	}
	if h.Singletons != 4 {
		t.Fatalf("singletons = %d, want 4", h.Singletons)
	}

	if len(h.ByProvider) != 1 || h.ByProvider[0].Provider != "anthropic" {
		t.Fatalf("providers = %+v", h.ByProvider)
	}
	pv := h.ByProvider[0]
	// Anthropic's break-even is (1.25-0.10)/(1-0.10) = 1.278, so only the
	// four-call epoch clears it; the four singletons cannot.
	if pv.BreakEven < 1.27 || pv.BreakEven > 1.29 {
		t.Fatalf("break-even = %v, want ~1.278", pv.BreakEven)
	}
	if pv.PayingEpochs != 1 {
		t.Fatalf("paying epochs = %d, want 1", pv.PayingEpochs)
	}
}

func TestMeterEpochsTableRenders(t *testing.T) {
	meterFixture(t)
	meterJSON = false

	out := runMeter(t, "epochs")
	for _, want := range []string{"Calls per epoch", "served exactly one call", "Amortization", "anthropic"} {
		if !strings.Contains(out, want) {
			t.Errorf("epochs table is missing %q:\n%s", want, out)
		}
	}
}

func TestMeterAtomsReportsCrossCategoryClustering(t *testing.T) {
	meterFixture(t)
	meterJSON = true

	var r prompt.CoUseReport
	if err := json.Unmarshal([]byte(runMeter(t, "atoms")), &r); err != nil {
		t.Fatalf("decode co-use report: %v", err)
	}

	if r.SuccessSelections != 30 {
		t.Fatalf("selections = %d, want 30", r.SuccessSelections)
	}
	// identity/core is in every selection, so it is skeleton, not signal.
	if len(r.Ubiquitous) != 1 || r.Ubiquitous[0] != "identity/core" {
		t.Fatalf("ubiquitous = %v, want [identity/core]", r.Ubiquitous)
	}
	for _, p := range r.Pairs {
		if p.A == "identity/core" || p.B == "identity/core" {
			t.Fatalf("a skeleton atom formed an association: %+v", p)
		}
	}

	// The fixture has one within-category cluster (two architecture atoms) and
	// one across-category cluster (knowledge + verification). Alignment must
	// therefore sit strictly between the extremes -- a readout that reported
	// 100% or 0% here would be measuring something other than what it claims.
	if r.CategoryAlignment <= 0 || r.CategoryAlignment >= 1 {
		t.Fatalf("category alignment = %v, want strictly between 0 and 1", r.CategoryAlignment)
	}
	if r.CrossCategoryPairs != 1 {
		t.Fatalf("cross-category pairs = %d, want 1", r.CrossCategoryPairs)
	}
	if len(r.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2: %+v", len(r.Clusters), r.Clusters)
	}
}

func TestMeterAtomsTableRenders(t *testing.T) {
	meterFixture(t)
	meterJSON = false

	out := runMeter(t, "atoms")
	for _, want := range []string{"Atom co-use", "CATEGORY ALIGNMENT", "Clusters", "skeleton atoms"} {
		if !strings.Contains(out, want) {
			t.Errorf("atoms table is missing %q:\n%s", want, out)
		}
	}
}

func TestMeterExplainsAnEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	prev := meterWorkspace
	meterWorkspace = root
	t.Cleanup(func() { meterWorkspace = prev })
	meterJSON = false

	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetErr(&buf)

	err := meterCmd.RunE(c, nil)
	if err == nil {
		t.Fatal("meter succeeded on a workspace with no receipt log")
	}
	// "No data" and "broken" must not read the same. The message has to say
	// which one it is and what to do about it.
	if !strings.Contains(err.Error(), "run an agent session") {
		t.Fatalf("error does not tell the operator what to do: %v", err)
	}
}

// The other half of TestMeterExplainsAnEmptyWorkspace, and the half that was
// wrong. A log whose FIRST record will not decode yields zero receipts, and the
// empty check ran before the truncation check, so a full log of unreadable
// records reported "run an agent session in this workspace first" -- the one
// case where that advice is certainly useless, because the sessions ran.
//
// The realistic cause is a schema change between binary versions, which is
// exactly when an operator most needs to be told it is a parse problem.
func TestMeterDistinguishesAnUnreadableLogFromAnAbsentOne(t *testing.T) {
	root := t.TempDir()
	prev := meterWorkspace
	meterWorkspace = root
	t.Cleanup(func() { meterWorkspace = prev })
	meterJSON = false

	path := filepath.Join(root, ".nerd", broker.DefaultReceiptLogName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// One field of the wrong shape on the first line, then perfectly good
	// records behind it that the stream decoder can never reach.
	body := `{"purpose":"/reasoning","estimated":{"confidence":{"source":"x"}}}` + "\n" +
		`{"purpose":"/reasoning","actual":{"input_tokens":10,"calls":1}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	var buf bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&buf)
	c.SetErr(&buf)

	err := meterCmd.RunE(c, nil)
	if err == nil {
		t.Fatal("meter succeeded on a log it could not parse a single record from")
	}
	if strings.Contains(err.Error(), "run an agent session") {
		t.Errorf("an unreadable log was reported as an absent one, so the operator "+
			"is told to do the thing they already did: %v", err)
	}
	if !strings.Contains(err.Error(), "could be parsed") {
		t.Errorf("error does not say the records are unreadable: %v", err)
	}
}

// emptyLogError's two branches turn on the truncation count, so the reader
// contract it depends on is pinned here rather than assumed: a malformed first
// record must yield zero records AND a non-zero truncation count. If the reader
// ever started skipping bad lines instead of stopping, the two cases would
// become indistinguishable again with nothing failing.
func TestReaderReportsTruncationWhenTheFirstRecordIsBad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipts.jsonl")
	if err := os.WriteFile(path, []byte("{not json at all\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	recs, truncated, err := broker.ReadReceiptLog(path)
	if err != nil {
		t.Fatalf("ReadReceiptLog: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("got %d records from a log with one malformed line", len(recs))
	}
	if truncated == 0 {
		t.Error("truncation not reported for a malformed first record; " +
			"emptyLogError can no longer tell an unreadable log from an absent one")
	}
}

func TestMeterToleratesATruncatedLog(t *testing.T) {
	root := meterFixture(t)
	meterJSON = true

	path := filepath.Join(root, ".nerd", broker.DefaultReceiptLogName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open for corruption: %v", err)
	}
	if _, err := f.WriteString(`{"purpose":"session","prov`); err != nil {
		t.Fatalf("write partial: %v", err)
	}
	_ = f.Close()

	// A crash mid-write must not make the whole history unreadable.
	var s MeterSummary
	if err := json.Unmarshal([]byte(runMeter(t)), &s); err != nil {
		t.Fatalf("decode after truncation: %v", err)
	}
	if s.Receipts != 9 {
		t.Fatalf("receipts = %d, want the 9 complete records before the cut", s.Receipts)
	}
}
