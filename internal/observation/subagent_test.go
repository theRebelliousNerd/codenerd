package observation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/retain"
)

// reviewerReturn is a reviewer's return in the shape the reviewer atom
// specifies: prose, then findings on the "[SEVERITY] file:line: message" line
// format, then a summary. Long enough to be past minRetainBytes, because that
// is the case the codec exists for.
func reviewerReturn() Return {
	var sb strings.Builder
	sb.WriteString("I reviewed internal/widget/widget.go and internal/widget/store.go.\n")
	sb.WriteString("Starting with the write path, then the error handling, then the locking.\n")
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&sb, "Reading internal/widget/widget.go around line %d to check the guard clause.\n", 100+i*7)
	}
	sb.WriteString("- [CRITICAL] internal/widget/widget.go:142: Store is written without holding mu\n")
	sb.WriteString("- [HIGH] internal/widget/store.go:88: the error from Flush is discarded\n")
	sb.WriteString("- [LOW] internal/widget/widget.go:12: exported type has no doc comment\n")
	sb.WriteString("I could not verify whether the caller at internal/widget/api.go:30 holds the lock.\n")
	sb.WriteString("Summary: one data race, one swallowed error, one style nit.\n")
	return Return{
		Agent:    "reviewer",
		Task:     "review file:internal/widget/widget.go",
		Output:   sb.String(),
		Duration: 4 * time.Second,
	}
}

func TestProjectReturn_ShouldCarryFindingsRatherThanTheTranscript(t *testing.T) {
	t.Parallel()

	r := ProjectReturn(reviewerReturn(), ReturnLimits{})

	if r.Status != StatusCompleted {
		t.Errorf("status = %q, want %q", r.Status, StatusCompleted)
	}
	if r.Verbatim != "" {
		t.Fatalf("a return this long must be elided, not carried whole")
	}
	if len(r.Findings) != 3 {
		t.Fatalf("got %d findings, want the 3 the reviewer reported: %+v", len(r.Findings), r.Findings)
	}
	// Worst first: the fixer reads the top of this list, so an ordering that
	// puts a doc-comment nit above a data race has handed it the wrong job.
	if got := r.Findings[0].Severity; got != "critical" {
		t.Errorf("first finding severity = %q, want critical: findings must be ordered worst-first", got)
	}
	if r.Findings[0].File != "internal/widget/widget.go" || r.Findings[0].Line != 142 {
		t.Errorf("first finding = %s:%d, want internal/widget/widget.go:142",
			r.Findings[0].File, r.Findings[0].Line)
	}

	text := r.Text("subagent_expand")
	if strings.Contains(text, "Reading internal/widget/widget.go around line") {
		t.Error("the projection carried the subagent's tool narration; that is the transcript, which is what it exists not to carry")
	}
	if !strings.Contains(text, "Store is written without holding mu") {
		t.Error("the projection dropped the critical finding, which is the one thing the parent had to be told")
	}
}

// TestProjectReturn_ShouldNotRepeatAFindingSiteAsEvidence guards the defect
// that first made the code-search codec larger than the grep output it
// replaced: the same fact printed twice, once per section.
func TestProjectReturn_ShouldNotRepeatAFindingSiteAsEvidence(t *testing.T) {
	t.Parallel()

	r := ProjectReturn(reviewerReturn(), ReturnLimits{})
	for _, ref := range r.Evidence {
		if ref == "internal/widget/widget.go:142" || ref == "internal/widget/store.go:88" {
			t.Errorf("%s is cited by a finding and again as evidence; the projection is paying for the same fact twice", ref)
		}
	}
	if !containsString(r.Evidence, "internal/widget/api.go:30") {
		t.Errorf("evidence = %v, want the citation that no finding names", r.Evidence)
	}
}

// TestProjectReturn_WhenTheProducerHasStructure_ShouldPreferItOverProse is the
// half the task turns on. internal/session measures the write set and the build
// and test verdicts on every turn; a projection that re-derived them from the
// agent's own description of its work would be guessing at an answer it was
// handed.
func TestProjectReturn_WhenTheProducerHasStructure_ShouldPreferItOverProse(t *testing.T) {
	t.Parallel()

	in := reviewerReturn()
	in.Findings = []Finding{{File: "a.go", Line: 1, Severity: "high", Message: "structured finding"}}
	in.Changed = []string{"internal/widget/widget.go"}
	in.Build = &Verification{Ran: true, OK: true}
	in.Output += "\n--- FAIL: TestSomething\nFAIL\n"

	r := ProjectReturn(in, ReturnLimits{})
	if len(r.Findings) != 1 || r.Findings[0].Message != "structured finding" {
		t.Fatalf("findings = %+v, want only the producer's own", r.Findings)
	}
	if len(r.Changed) != 1 || r.Changed[0] != "internal/widget/widget.go" {
		t.Fatalf("changed = %v, want the producer's write set", r.Changed)
	}
	if len(r.Verification) != 1 || r.Verification[0].Source != SourceObserved {
		t.Fatalf("verification = %+v, want one observed build check", r.Verification)
	}
}

// TestProjectReturn_ShouldNotInventChangedArtifacts is the one guess this codec
// refuses to make. The parent ACTS on the changed list — it re-reads those
// files, it reports them as done — so a path read out of prose sends it to edit
// something the subagent never touched.
func TestProjectReturn_ShouldNotInventChangedArtifacts(t *testing.T) {
	t.Parallel()

	in := reviewerReturn()
	in.Output += "\nI modified internal/widget/widget.go and created internal/widget/widget_test.go.\n"

	r := ProjectReturn(in, ReturnLimits{})
	if len(r.Changed) != 0 {
		t.Errorf("changed = %v, but the producer supplied no write set; a projection that reads one out of prose reports edits nobody made", r.Changed)
	}
}

// TestProjectReturn_WhenVerificationDidNotRun_ShouldSaySoRatherThanStaySilent.
// A skipped check renders as an absence, and an absence reads as "fine" — which
// is exactly how a parent skips the verification it needed.
func TestProjectReturn_WhenVerificationDidNotRun_ShouldSaySoRatherThanStaySilent(t *testing.T) {
	t.Parallel()

	in := reviewerReturn()
	in.Build = &Verification{Ran: false}

	r := ProjectReturn(in, ReturnLimits{})
	if !containsSubstring(r.Uncertainty, "build verification did not run") {
		t.Fatalf("uncertainty = %v, want the skipped build named", r.Uncertainty)
	}
	text := r.Text("")
	if !strings.Contains(text, "build did not run") {
		t.Errorf("rendered text does not say the build did not run:\n%s", text)
	}
	if strings.Contains(text, "build passed") {
		t.Error("a skipped build rendered as a pass")
	}
}

// TestProjectReturn_ShouldNotReadAVerificationVerdictOutOfProse is the defect
// the first version of this codec shipped with, caught by this fixture. The
// reviewer return says "the error from Flush is discarded" and "I could not
// verify..."; internal/testoutput's generic heuristics match "error" and
// "failed" as whole words so they can read a non-Go runner, and running them
// over that prose scored three test failures for a shard that never ran a test.
// A fabricated verdict is not a summary — the parent acts on it.
func TestProjectReturn_ShouldNotReadAVerificationVerdictOutOfProse(t *testing.T) {
	t.Parallel()

	in := reviewerReturn()
	in.Output += "\n--- FAIL: TestWidgetFlush\nFAIL\nok  \tcodenerd/internal/widget\t0.4s\n"

	r := ProjectReturn(in, ReturnLimits{})
	if len(r.Verification) != 0 {
		t.Fatalf("verification = %+v, but the producer supplied none; a codec reading one out of text reports runs that never happened", r.Verification)
	}
}

// TestReportedTests_ShouldMarkTheVerdictAsAClaimNotEvidence. A subagent pasting
// a green log is a claim about a run nobody watched; the parent decides whether
// to re-verify on exactly that difference.
func TestReportedTests_ShouldMarkTheVerdictAsAClaimNotEvidence(t *testing.T) {
	t.Parallel()

	v := ReportedTests("--- FAIL: TestWidgetFlush\nFAIL\nok  \tcodenerd/internal/widget\t0.4s\n")
	if v == nil {
		t.Fatal("a real test log produced no verdict")
	}
	if v.Source != SourceReported {
		t.Errorf("source = %q, want %q: nothing watched this run", v.Source, SourceReported)
	}
	if v.Failed != 1 || !containsString(v.FailedNames, "TestWidgetFlush") {
		t.Errorf("verdict = %+v, want the named failure", *v)
	}

	in := reviewerReturn()
	in.Tests = v
	if !strings.Contains(ProjectReturn(in, ReturnLimits{}).Text(""), "["+SourceReported+"]") {
		t.Error("rendered text does not mark the verdict as reported rather than observed")
	}
}

// TestReportedTests_WhenThereIsNoRunnerOutput_ShouldReturnNothing keeps the
// producer-side helper from becoming the same defect one layer out.
func TestReportedTests_WhenThereIsNoRunnerOutput_ShouldReturnNothing(t *testing.T) {
	t.Parallel()

	if v := ReportedTests("I reviewed the file and found nothing of note.\n"); v != nil {
		t.Errorf("prose with no runner output produced %+v, want no verdict", *v)
	}
}

func TestProjectReturn_WhenItFailed_ShouldSayWhatFailedAndStillCarryTheOutput(t *testing.T) {
	t.Parallel()

	in := reviewerReturn()
	in.Failure = "context deadline exceeded"

	r := ProjectReturn(in, ReturnLimits{})
	if r.Status != StatusFailed {
		t.Errorf("status = %q, want %q", r.Status, StatusFailed)
	}
	if !strings.Contains(r.Text(""), "context deadline exceeded") {
		t.Error("a failed return did not name its failure")
	}
	if len(r.Findings) == 0 {
		t.Error("a failed return dropped the findings it produced before failing; those are usually the reason")
	}
}

func TestProjectReturn_WhenTheSubagentReturnedNothing_ShouldNotReportCompletion(t *testing.T) {
	t.Parallel()

	r := ProjectReturn(Return{Agent: "coder", Task: "fix file:a.go"}, ReturnLimits{})
	if r.Status != StatusEmpty {
		t.Errorf("status = %q, want %q: a subagent that returned nothing failed at the only thing it was for", r.Status, StatusEmpty)
	}
}

func TestProjectReturn_WhenFindingsExceedTheLimit_ShouldCapAndSayHowMany(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "- [MEDIUM] pkg/file%d.go:%d: issue number %d\n", i, i+1, i)
	}
	r := ProjectReturn(Return{Agent: "reviewer", Output: sb.String()}, ReturnLimits{MaxFindings: 5})

	if len(r.Findings) != 5 {
		t.Fatalf("got %d findings, want the cap of 5", len(r.Findings))
	}
	if r.FindingsOmitted != 35 {
		t.Errorf("omitted = %d, want 35 reported so the omission is visible rather than silent", r.FindingsOmitted)
	}
	if !strings.Contains(r.Text(""), "35 more finding(s) not shown") {
		t.Error("the rendered projection hid its own omission")
	}
}

func TestProjectReturn_ShouldProduceTheSameResultForTheSameObservation(t *testing.T) {
	t.Parallel()

	first := ProjectReturn(reviewerReturn(), ReturnLimits{})
	for i := 0; i < 25; i++ {
		if got := ProjectReturn(reviewerReturn(), ReturnLimits{}); !reflect.DeepEqual(got, first) {
			t.Fatalf("projection %d differs from the first for one unchanged return", i)
		}
	}
}

// TestProjectReturn_WhenTheReturnIsShort_ShouldCarryItWholeRatherThanElide is
// the honest worst case, and the reason minRetainBytes exists. On a one-line
// confirmation the header and the handle footer cost several times the return
// itself, so eliding it would make the projection larger than the thing it
// replaced for no gain the parent can use.
func TestProjectReturn_WhenTheReturnIsShort_ShouldCarryItWholeRatherThanElide(t *testing.T) {
	t.Parallel()

	const short = "Done. Updated internal/widget/widget.go to hold mu across the write."
	r := ProjectReturn(Return{Agent: "coder", Output: short}, ReturnLimits{})

	if r.Verbatim != short {
		t.Fatalf("verbatim = %q, want the whole short return", r.Verbatim)
	}
	if r.Handle != "" {
		t.Error("a return carried whole must not also advertise a handle: there is nothing behind it to redeem")
	}
	text := r.Text("subagent_expand")
	if !strings.Contains(text, short) {
		t.Fatalf("the short return was not carried:\n%s", text)
	}
	if strings.Contains(text, "subagent_expand") {
		t.Error("nothing was elided, so naming a redemption verb charges for a promise with no debt behind it")
	}
	// Measured: 67 bytes of header over a 68-byte return, and 67 over a 5-byte
	// one — the overhead is one header line and does not scale. Pinned because
	// this is the case the codec is worst at: it cannot help, so all it must do
	// is not hurt much. A second header line would double it.
	if over := len(text) - len(short); over > 90 {
		t.Errorf("a short return costs %d bytes over the raw return; the header must stay small on the case the codec cannot help", over)
	}
}

// TestProjectReturn_AtTheElideThreshold_ShouldCrossOverInTheRightDirection is
// the worst case pinned honestly. Just under minRetainBytes the projection is
// LARGER than the return it replaced — measured at 571 bytes against 504, a
// 13% loss — and just over it the projection is smaller. That is the whole
// justification for the threshold, and a change that moved the crossover the
// wrong way would make the codec cost more than it saves on every short return
// while every other test still passed.
func TestProjectReturn_AtTheElideThreshold_ShouldCrossOverInTheRightDirection(t *testing.T) {
	t.Parallel()

	filler := "prose that says very little at all. "
	under := strings.Repeat(filler, (minRetainBytes/len(filler))-1)
	over := strings.Repeat(filler, (minRetainBytes/len(filler))+1)
	if len(under) >= minRetainBytes || len(over) < minRetainBytes {
		t.Fatalf("fixture straddles the wrong threshold: under=%d over=%d minRetainBytes=%d", len(under), len(over), minRetainBytes)
	}

	c := NewSubagents(retain.DefaultConfig())
	underText := c.EncodeReturn(Return{Agent: "coder", Output: under}, ReturnLimits{}).Text("subagent_expand")
	overText := c.EncodeReturn(Return{Agent: "coder", Output: over}, ReturnLimits{}).Text("subagent_expand")

	if len(underText) <= len(under) {
		t.Errorf("a return under the threshold projected to %d bytes from %d; if it were already smaller the threshold would be in the wrong place", len(underText), len(under))
	}
	if cost := len(underText) - len(under); cost > 90 {
		t.Errorf("under the threshold the projection costs %d bytes over the return; that is the loss the threshold exists to bound", cost)
	}
	if len(overText) >= len(over) {
		t.Errorf("a return over the threshold projected to %d bytes from %d; past the threshold eliding must pay", len(overText), len(over))
	}
}

// TestProjectReturn_WhenThereAreNoFindingsOrCitations_ShouldStillCarryTheShape.
// A third of the 67 real agent outputs in this repository's .quality_assurance
// directory are long structured prose with no severity marker and no file:line
// citation anywhere. Without the outline those projected to a status line and a
// handle, which is honest and nearly useless.
func TestProjectReturn_WhenThereAreNoFindingsOrCitations_ShouldStillCarryTheShape(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	sb.WriteString("# Boundary analysis\n\n")
	for i := 0; i < 6; i++ {
		fmt.Fprintf(&sb, "## Section %d\n", i)
		for j := 0; j < 6; j++ {
			sb.WriteString("Discussion of the shape of the problem, with no citations at all.\n")
		}
	}

	r := ProjectReturn(Return{Agent: "researcher", Output: sb.String()}, ReturnLimits{})
	if len(r.Findings) != 0 || len(r.Evidence) != 0 {
		t.Fatalf("fixture is not the prose-only case: findings=%v evidence=%v", r.Findings, r.Evidence)
	}
	if len(r.Outline) != 7 {
		t.Fatalf("outline = %+v, want the 7 headings", r.Outline)
	}
	if r.Outline[0].Title != "Boundary analysis" || r.Outline[0].Line != 1 {
		t.Errorf("first section = %+v, want the document title at line 1", r.Outline[0])
	}
	text := r.Text("")
	if !strings.Contains(text, "sections (line title)") || !strings.Contains(text, "Section 3") {
		t.Errorf("the rendered projection carries no navigation for a prose-only return:\n%s", text)
	}
	if strings.Contains(text, "Discussion of the shape") {
		t.Error("the outline carried body prose; it is an outline of what is in the return, not the return")
	}
}

// TestExtractOutline_ShouldNotTreatOrdinaryProseAsAHeading keeps the outline
// from filling with sentences, which would cost the same bytes as real headings
// and carry none of the navigation.
func TestExtractOutline_ShouldNotTreatOrdinaryProseAsAHeading(t *testing.T) {
	t.Parallel()

	out, _ := extractOutline("Findings:\n**Important**\n#hashtag not a heading\n# Real heading\n", 10)
	if len(out) != 1 || out[0].Title != "Real heading" {
		t.Errorf("outline = %+v, want only the ATX heading", out)
	}
}

// TestReturnText_WhenTheReturnIsLong_ShouldCostFarLessThanTheTranscript is the
// case the codec exists for.
func TestReturnText_WhenTheReturnIsLong_ShouldCostFarLessThanTheTranscript(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "Considering internal/widget/file%d.go: the guard clause looks right, moving on.\n", i)
	}
	sb.WriteString("- [HIGH] internal/widget/widget.go:142: Store is written without holding mu\n")
	in := Return{Agent: "reviewer", Task: "review all", Output: sb.String()}

	projected := len(NewSubagents(retain.DefaultConfig()).EncodeReturn(in, ReturnLimits{}).Text("subagent_expand"))
	raw := len(in.Output)
	if projected >= raw/8 {
		t.Errorf("projected return is %d bytes against %d raw; on a transcript this long the projection must be a fraction of it", projected, raw)
	}
}

// TestReturnText_ShouldStayBoundedAsTheTranscriptGrows pins that the cost does
// not scale with how talkative the subagent was, which is the property the
// truncate-at-N it replaces did not have.
func TestReturnText_ShouldStayBoundedAsTheTranscriptGrows(t *testing.T) {
	t.Parallel()

	sizeAt := func(n int) int {
		var sb strings.Builder
		sb.WriteString("- [HIGH] internal/widget/widget.go:142: Store is written without holding mu\n")
		for i := 0; i < n; i++ {
			fmt.Fprintf(&sb, "Considering internal/widget/file%d.go: nothing to report here at all.\n", i)
		}
		return len(ProjectReturn(Return{Agent: "reviewer", Output: sb.String()}, ReturnLimits{}).Text(""))
	}

	small, large := sizeAt(20), sizeAt(2000)
	// The header carries a byte and a line count, so a longer transcript costs
	// a few more digits, and the evidence section fills to its cap. Anything
	// past that means the projection scales with the input.
	if large-small > 400 {
		t.Errorf("projection grew from %d to %d bytes on a transcript 100x longer; it must be bounded by structure, not by input size", small, large)
	}
}

// TestSubagents_ShouldHoldNothingItCouldReRunWith pins the structural half of
// the guarantee. The comment on Subagents says hydration cannot re-run because
// the type has nothing to re-run with; a later edit adding a task executor, a
// spawner or a workspace root would quietly make that false while every other
// test still passed — and for THIS codec a re-run is not a stale read, it is a
// second execution with every side effect the first one had.
func TestSubagents_ShouldHoldNothingItCouldReRunWith(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(Subagents{})
	if typ.NumField() != 1 {
		t.Fatalf("Subagents has %d fields; it must hold only its retention, or hydration can grow a way to run a subagent again", typ.NumField())
	}
	if got := typ.Field(0).Type; got != reflect.TypeOf((*retain.Store)(nil)) {
		t.Fatalf("Subagents' only field is %s, want *retain.Store", got)
	}
}

func TestEncodeReturn_ShouldRetainTheWholeTranscriptBehindTheHandle(t *testing.T) {
	t.Parallel()

	c := NewSubagents(retain.DefaultConfig())
	in := reviewerReturn()
	encoded := c.EncodeReturn(in, ReturnLimits{})
	if encoded.Handle == "" {
		t.Fatal("a long return published no handle, so its transcript is unreachable")
	}

	hydrated, err := c.HydrateReturn(encoded.Handle, ReturnWindow{Limit: maxReturnHydrateLines})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	rejoined := strings.Join(hydrated.Lines, "\n")
	if rejoined != in.Output {
		t.Errorf("hydration returned %d bytes, want the %d retained; a handle that expands to less than the parent was given is lossy at the worst moment",
			len(rejoined), len(in.Output))
	}
}

func TestHydrateReturn_WhenWindowed_ShouldPageWithoutLosingALine(t *testing.T) {
	t.Parallel()

	c := NewSubagents(retain.DefaultConfig())
	in := reviewerReturn()
	encoded := c.EncodeReturn(in, ReturnLimits{})

	var seen []string
	offset := 0
	for i := 0; i < 50; i++ {
		page, err := c.HydrateReturn(encoded.Handle, ReturnWindow{Offset: offset, Limit: 3})
		if err != nil {
			t.Fatalf("hydrate page at offset %d: %v", offset, err)
		}
		seen = append(seen, page.Lines...)
		if page.NextOffset == 0 {
			break
		}
		offset = page.NextOffset
	}

	want := strings.Split(in.Output, "\n")
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("walking the window returned %d lines, want the %d retained exactly once", len(seen), len(want))
	}
}

func TestHydrateReturn_WhenWindowMatches_ShouldReturnOnlyTheLinesThatContainIt(t *testing.T) {
	t.Parallel()

	c := NewSubagents(retain.DefaultConfig())
	encoded := c.EncodeReturn(reviewerReturn(), ReturnLimits{})

	hydrated, err := c.HydrateReturn(encoded.Handle, ReturnWindow{Match: "CRITICAL"})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(hydrated.Lines) != 1 || !strings.Contains(hydrated.Lines[0], "holding mu") {
		t.Errorf("match window returned %v, want only the critical finding's line", hydrated.Lines)
	}
}

func TestHydrateReturn_WhenLimitIsAbsurd_ShouldStillBound(t *testing.T) {
	t.Parallel()

	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&sb, "line %d of a very long transcript\n", i)
	}
	c := NewSubagents(retain.DefaultConfig())
	encoded := c.EncodeReturn(Return{Agent: "coder", Output: sb.String()}, ReturnLimits{})

	hydrated, err := c.HydrateReturn(encoded.Handle, ReturnWindow{Limit: 1 << 20})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(hydrated.Lines) != maxReturnHydrateLines {
		t.Errorf("expansion returned %d lines; the hard cap of %d is what stops the withheld transcript arriving one turn later",
			len(hydrated.Lines), maxReturnHydrateLines)
	}
	if hydrated.NextOffset != maxReturnHydrateLines {
		t.Errorf("next offset = %d, want %d so the walk can continue", hydrated.NextOffset, maxReturnHydrateLines)
	}
}

func TestHydrateReturn_WhenHandleIsUnknownOrExpired_ShouldSaySoDistinctly(t *testing.T) {
	t.Parallel()

	c := NewSubagents(retain.DefaultConfig())
	if _, err := c.HydrateReturn("obs:sa:nope", ReturnWindow{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound: the caller must re-delegate only when it means to, not retry the expansion", err)
	}
	if _, err := c.HydrateReturn("  ", ReturnWindow{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error for an empty handle = %v, want ErrNotFound", err)
	}
}

// TestHydrateReturn_WhenHandleBelongsToAnotherCodec_ShouldRefuse. Both codecs
// mint content-addressed ids into their own store; a caller that pasted a
// search handle here must be told it is the wrong kind rather than handed a
// decode failure it will read as corruption.
func TestHydrateReturn_WhenHandleBelongsToAnotherCodec_ShouldRefuse(t *testing.T) {
	t.Parallel()

	store := retain.New(retain.Config{Prefix: subagentHandlePrefix})
	c := &Subagents{store: store}
	foreign := store.Mint("code_search", []byte(`{"query":"x"}`))

	if _, err := c.HydrateReturn(foreign, ReturnWindow{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound for a handle minted under another kind", err)
	}
}

func TestEncodeReturn_WhenTheSameReturnRepeats_ShouldReuseOneHandle(t *testing.T) {
	t.Parallel()

	c := NewSubagents(retain.DefaultConfig())
	first := c.EncodeReturn(reviewerReturn(), ReturnLimits{})
	second := c.EncodeReturn(reviewerReturn(), ReturnLimits{})

	if first.Handle == "" || first.Handle != second.Handle {
		t.Fatalf("handles %q and %q differ for one identical return; a handle quoted back from an earlier turn must still resolve",
			first.Handle, second.Handle)
	}
	if got := c.store.Stats().Entries; got != 1 {
		t.Errorf("store holds %d entries for one repeated return, want 1", got)
	}
}

func TestReturnText_ShouldNameTheHandleAndTheVerbThatRedeemsIt(t *testing.T) {
	t.Parallel()

	c := NewSubagents(retain.DefaultConfig())
	text := c.EncodeReturn(reviewerReturn(), ReturnLimits{}).Text("subagent_expand")

	if !strings.Contains(text, subagentHandlePrefix) {
		t.Fatalf("rendered return names no handle:\n%s", text)
	}
	if !strings.Contains(text, "subagent_expand handle=") {
		t.Errorf("rendered return does not name the verb that redeems its handle:\n%s", text)
	}
	if !strings.Contains(text, "without re-running the subagent") {
		t.Errorf("rendered return does not say the expansion is free of a second execution, which is what makes relying on it safe:\n%s", text)
	}
}

// TestReturnText_WhenNothingWasRetained_ShouldNotLetTheElisionReadAsAnAbsence.
// A transcript that was elided and then not retained is the one case where the
// parent must not conclude "that was all it said".
func TestReturnText_WhenNothingWasRetained_ShouldNotLetTheElisionReadAsAnAbsence(t *testing.T) {
	t.Parallel()

	text := ProjectReturn(reviewerReturn(), ReturnLimits{}).Text("subagent_expand")
	if !strings.Contains(text, "cannot be reopened") {
		t.Errorf("an unretained elision did not say so:\n%s", text)
	}
}

func TestSharedSubagents_ShouldBeOneStoreForEveryProducer(t *testing.T) {
	t.Parallel()

	if SharedSubagents() != SharedSubagents() {
		t.Fatal("SharedSubagents returned two stores; a handle minted by one producer and redeemed by another would never resolve")
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
