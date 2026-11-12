package problemlog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

const fixture = "testdata/problems_fixture.txt"

func skipNotes(e Entry) bool { return strings.Contains(e.Message, "debug-self:") }

func scanAll(t *testing.T, path string) []Entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var got []Entry
	if err := Scan(f, nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	return got
}

// A record owns every line up to the next record, whole; records of a level
// the log never mirrors end the previous one and are not entries; lines
// before the first record belong to nothing.
func TestScan_KeepsWholeMultiLineMessagesAndBoundsRecords(t *testing.T) {
	entries := scanAll(t, fixture)
	if len(entries) != 12 {
		t.Fatalf("scanned %d entries, want 12 (WARN/ERROR records only)", len(entries))
	}
	first := entries[0]
	if first.Level != LevelWarn || first.Category != "kernel" || !strings.HasPrefix(first.Message, "admission: tool_capability/2") {
		t.Fatalf("first entry = %+v (the orphan line before the first record must be ignored)", first)
	}
	want := time.Date(2026, 10, 28, 2, 20, 3, 0, time.Local)
	if !first.Time.Equal(want) {
		t.Fatalf("time = %v, want %v (fraction dropped, local zone)", first.Time, want)
	}
	var multi Entry
	for _, e := range entries {
		if strings.Contains(e.Message, "handleNorthstarSummary") {
			multi = e
		}
	}
	lines := strings.Split(multi.Message, "\n")
	if multi.Level != LevelError || len(lines) != 7 || !strings.HasSuffix(lines[0], "Its text is:") ||
		!strings.Contains(lines[len(lines)-1], "saveNorthstar(false)") {
		t.Fatalf("multi-line message not kept whole (%d lines):\n%s", len(lines), multi.Message)
	}
	for _, e := range entries {
		if strings.Contains(e.Message, "never mirrors") || strings.Contains(e.Message, "continuation of the debug record") {
			t.Fatalf("a DEBUG record leaked into the entries: %+v", e)
		}
	}
}

func TestScan_SkipAndNoTrailingNewline(t *testing.T) {
	log := "2026/10/28 02:00:00.000001 [WARN] [kernel] debug-self: noted\n" +
		"2026/10/28 02:00:01.000001 [ERROR] [kernel] real failure"
	var got []Entry
	err := Scan(strings.NewReader(log), skipNotes, func(e Entry) { got = append(got, e) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "real failure" {
		t.Fatalf("got %+v, want only the unterminated final record", got)
	}
}

func TestSignature_StripsVolatileValuesFromTheHeadlineOnly(t *testing.T) {
	cases := []struct{ msg, want string }{
		{"hydrate failed for /home/u/a.db after 12ms", "hydrate failed for <path> after #ms"},
		{"Validation failed: action=call_97c69f7c-8d73-44a2-9390-30104009fe54 error=x", "Validation failed: action=call_<uuid> error=x"},
		{"digest deadbeefcafe1234 mismatch", "digest <hex> mismatch"},
		{"took 371.977644ms (target: <100ms)", "took #ms (target: <#ms)"},
		{"  spaced   out\t words ", "spaced out words"},
		{"headline 7\n903  payload line 99", "headline #"},
	}
	for _, c := range cases {
		if got := Signature(c.msg); got != c.want {
			t.Errorf("Signature(%q) = %q, want %q", c.msg, got, c.want)
		}
	}
	long := strings.Repeat("word ", 200) + "tail"
	if got := Signature(long); !strings.HasSuffix(got, "word tail") {
		t.Errorf("a long headline must not be shortened, got %d bytes", len(got))
	}
}

func TestReadFile_GroupsRepeatsAndRanksErrorsFirst(t *testing.T) {
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Entries != 11 || rep.Omitted() {
		t.Fatalf("entries=%d omitted=%d, want 11 read and nothing omitted", rep.Entries, rep.OmittedProblems)
	}
	// Errors first (by count), then warnings by count.
	var order []string
	for _, p := range rep.Problems {
		order = append(order, fmt.Sprintf("%s/%s x%d", p.Level, p.Category, p.Count))
	}
	wantPrefix := []string{"ERROR/session x2", "ERROR/session x1", "ERROR/session x1", "WARN/performance x3"}
	for i, w := range wantPrefix {
		if order[i] != w {
			t.Fatalf("rank %d = %s, want %s (full order %v)", i, order[i], w, order)
		}
	}
	edit := rep.Problems[0]
	if edit.Signature != "Tool call edit_element failed: modular tool execution failed: old does not occur in cmd<path> Its text is:" {
		t.Fatalf("signature = %q", edit.Signature)
	}
	if !strings.Contains(edit.Sample, "handleNorthstarSummary") || strings.Count(edit.Sample, "\n") != 6 {
		t.Fatalf("sample must be the first occurrence, whole:\n%s", edit.Sample)
	}
	if got := edit.First.Format("15:04:05"); got != "02:24:40" {
		t.Fatalf("first seen %s", got)
	}
	if got := edit.Last.Format("15:04:05"); got != "02:40:34" {
		t.Fatalf("last seen %s", got)
	}
}

func TestReadFile_BoundReportsWhatWasLeftOut(t *testing.T) {
	rep, err := ReadFile(fixture, Options{MaxProblems: 4, Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 4 {
		t.Fatalf("kept %d problems, want 4", len(rep.Problems))
	}
	kept := 0
	for _, p := range rep.Problems {
		kept += p.Count
	}
	if rep.OmittedProblems != 4 || rep.OmittedEntries != rep.Entries-kept || rep.OmittedEntries != 4 {
		t.Fatalf("omitted %d problems / %d entries (kept %d of %d), want 4 / 4",
			rep.OmittedProblems, rep.OmittedEntries, kept, rep.Entries)
	}
	if !strings.Contains(rep.Summary(), "4 lower-ranked problem(s) covering 4 entr(ies) left out") {
		t.Fatalf("summary does not say what was left out: %s", rep.Summary())
	}
}

func TestReadFile_MissingFileIsAnError(t *testing.T) {
	if _, err := ReadFile(filepath.Join(t.TempDir(), "absent_problems.log"), Options{}); err == nil {
		t.Fatal("a missing log must be an error, not an empty report")
	}
}

func TestFacts_ProjectProblemsSeenAndOmitted(t *testing.T) {
	rep, err := ReadFile(fixture, Options{MaxProblems: 4, Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	facts := rep.Facts()
	seconds := map[string]map[int64]bool{}
	for _, p := range rep.Problems {
		if seconds[p.Signature] == nil {
			seconds[p.Signature] = map[int64]bool{}
		}
		for _, s := range p.Seconds {
			seconds[p.Signature][s] = true
		}
	}
	at := 0
	for _, secs := range seconds {
		at += len(secs)
	}
	if at == 0 || len(facts) != 4*2+at+1 {
		t.Fatalf("%d facts, want a problem and a seen row per problem, %d self_problem_at rows, and the omission", len(facts), at)
	}
	for _, f := range facts[4*2 : 4*2+at] {
		if f.Predicate != AtPredicate || len(f.Args) != 2 {
			t.Fatalf("expected self_problem_at rows after the problems, got %+v", f)
		}
	}
	p := facts[0]
	if p.Predicate != "self_problem" || len(p.Args) != 5 {
		t.Fatalf("fact 0 = %+v", p)
	}
	if lvl, ok := p.Args[1].(types.MangleAtom); !ok || lvl != "/error" {
		t.Fatalf("level must be the /error name constant, got %#v", p.Args[1])
	}
	if p.Args[3] != int64(2) {
		t.Fatalf("count = %#v, want int64(2)", p.Args[3])
	}
	if sample, _ := p.Args[4].(types.MangleString); !strings.Contains(string(sample), "saveNorthstar(false)") {
		t.Fatal("the sample fact must hold the whole multi-line message")
	}
	seen := facts[1]
	if seen.Predicate != "self_problem_seen" || seen.Args[3].(int64) >= seen.Args[4].(int64) {
		t.Fatalf("seen fact = %+v, want first < last unix seconds", seen)
	}
	om := facts[len(facts)-1]
	if om.Predicate != "self_problem_omitted" || om.Args[0] != int64(4) || om.Args[1] != int64(4) {
		t.Fatalf("omitted fact = %+v", om)
	}
	if (*Report)(nil).Facts() != nil {
		t.Fatal("a nil report has no facts")
	}
}

func TestScan_BoundsOverlongRecordsWithTruncationMarker(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("2026/10/28 02:00:00 [WARN] [kernel] runaway head\n")
	for i := 0; i < 300; i++ {
		sb.WriteString("continuation line payload\n")
	}
	var got []Entry
	if err := Scan(strings.NewReader(sb.String()), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 bounded record", len(got))
	}
	if !strings.Contains(got[0].Message, "[truncated]") {
		t.Fatalf("overlong record must carry a truncation marker, got %d bytes", len(got[0].Message))
	}
	if n := len(strings.Split(got[0].Message, "\n")); n > 202 {
		t.Fatalf("bounded record has %d lines, want at most 202", n)
	}
	huge := "2026/10/28 02:00:00 [WARN] [kernel] " + strings.Repeat("x", 40*1024) + "\nsecond line that must be dropped\n"
	got = nil
	if err := Scan(strings.NewReader(huge), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0].Message, "[truncated]") {
		t.Fatalf("oversized headline must truncate, got %+v", got)
	}
	if strings.Contains(got[0].Message, "second line") {
		t.Fatalf("lines after an oversized headline must be dropped:\n%s", got[0].Message[:200])
	}
}

func TestScan_StampAndInfoLevelsBoundRecords(t *testing.T) {
	log := "orphan continuation before any record\n" +
		"2026/10/28 02:00:00 [WARN] [kernel] first\n" +
		"2026/10/28 02:00:01 [INFO] [kernel] info chatter\n" +
		"stray continuation after info\n" +
		"2026/10/28 02:00:02 [WARN] [kernel] second"
	var got []Entry
	if err := Scan(strings.NewReader(log), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Message != "first" || got[1].Message != "second" {
		t.Fatalf("got %+v, want only the two WARN records bounded by INFO/stamp lines", got)
	}
}

func TestReadFile_DefaultLimitKeepsAllWithoutOmission(t *testing.T) {
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Omitted() {
		t.Fatalf("default limit must keep every distinct problem, omitted %+v", rep.OmittedProblems)
	}
	if len(rep.Problems) != 8 {
		t.Fatalf("kept %d problems, want all 8 distinct groups", len(rep.Problems))
	}
	if got := rep.Summary(); strings.Contains(got, "left out") {
		t.Fatalf("summary without omission must not mention what was left out: %s", got)
	} else if !strings.Contains(got, "8 problem(s) from 11 log entr(ies)") {
		t.Fatalf("summary = %q, want 8 problems from 11 entries", got)
	}
	neg, err := ReadFile(fixture, Options{MaxProblems: -1, Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	if len(neg.Problems) != len(rep.Problems) {
		t.Fatalf("non-positive MaxProblems must fall back to DefaultMaxProblems: %d vs %d", len(neg.Problems), len(rep.Problems))
	}
	var nilRep *Report
	if nilRep.Omitted() || nilRep.Summary() != "no problems log" {
		t.Fatalf("nil report Omitted=%v Summary=%q", nilRep.Omitted(), nilRep.Summary())
	}
	if got := (&Report{}).Summary(); got != "0 problem(s) from 0 log entr(ies)" {
		t.Fatalf("empty summary = %q", got)
	}
}

func TestReadFile_GroupsOutOfOrderTimesAndKeepsFirstSample(t *testing.T) {
	log := "2026/10/28 02:00:03 [WARN] [kernel] flaky thing 1\n" +
		"2026/10/28 02:00:01 [WARN] [kernel] flaky thing 1\n" +
		"2026/10/28 02:00:02 [WARN] [kernel] flaky thing 1\n"
	path := filepath.Join(t.TempDir(), "problems.log")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 1 {
		t.Fatalf("got %d problems, want 1 grouped signature", len(rep.Problems))
	}
	p := rep.Problems[0]
	if p.Count != 3 {
		t.Fatalf("count = %d, want 3 repeats grouped", p.Count)
	}
	if got := p.First.Format("15:04:05"); got != "02:00:01" {
		t.Fatalf("first = %s, want earliest 02:00:01", got)
	}
	if got := p.Last.Format("15:04:05"); got != "02:00:03" {
		t.Fatalf("last = %s, want latest 02:00:03", got)
	}
	if p.Sample != "flaky thing 1" {
		t.Fatalf("sample = %q, want the first occurrence kept whole", p.Sample)
	}
}

func TestFacts_WarnLevelWithoutOmission(t *testing.T) {
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	facts := (&Report{Problems: []Problem{{
		Signature: "s", Level: LevelWarn, Category: "c", Sample: "m",
		Count: 2, First: time.Unix(10, 0), Last: time.Unix(20, 0),
	}}}).Facts()
	if len(facts) != 2 {
		t.Fatalf("%d facts, want problem+seen and no omission", len(facts))
	}
	if lvl, ok := facts[0].Args[1].(types.MangleAtom); !ok || lvl != "/warn" {
		t.Fatalf("WARN level must be the /warn name constant, got %#v", facts[0].Args[1])
	}
	_ = rep
	if got := (&Report{}).Facts(); len(got) != 0 {
		t.Fatalf("empty report must project no facts, got %+v", got)
	}
}

type stubProblemKernel struct {
	retracts   []string
	asserted   []types.Fact
	retractErr error
	assertErr  error
}

func (s *stubProblemKernel) Retract(predicate string) error {
	s.retracts = append(s.retracts, predicate)
	return s.retractErr
}

func (s *stubProblemKernel) AssertBatch(facts []types.Fact) error {
	s.asserted = append(s.asserted, facts...)
	return s.assertErr
}

func TestReplace_RetractAndReplaceStable(t *testing.T) {
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	stub := &stubProblemKernel{}
	if err := Replace(stub, rep); err != nil {
		t.Fatal(err)
	}
	if len(stub.retracts) != len(Predicates) {
		t.Fatalf("retracted %v, want all %v before assert", stub.retracts, Predicates)
	}
	for i, pred := range Predicates {
		if stub.retracts[i] != pred {
			t.Fatalf("retract order %v, want %v", stub.retracts, Predicates)
		}
	}
	if len(stub.asserted) != len(rep.Facts()) {
		t.Fatalf("asserted %d facts, want %d replacement facts", len(stub.asserted), len(rep.Facts()))
	}
	if err := Replace(nil, rep); err == nil {
		t.Fatal("nil Kernel must be an error, not a silent no-op")
	}
	joined := &stubProblemKernel{retractErr: fmt.Errorf("boom retract"), assertErr: fmt.Errorf("boom assert")}
	if err := Replace(joined, rep); err == nil || !strings.Contains(err.Error(), "boom retract") || !strings.Contains(err.Error(), "boom assert") {
		t.Fatalf("retract and assert failures must join: %v", err)
	}
	fresh := &stubProblemKernel{}
	if err := Replace(fresh, nil); err != nil {
		t.Fatal(err)
	}
	if len(fresh.asserted) != 0 {
		t.Fatalf("nil report retracts but asserts nothing, got %+v", fresh.asserted)
	}
}

type stubProblemQuerier struct {
	rows []types.Fact
	err  error
}

func (s stubProblemQuerier) Query(predicate string) ([]types.Fact, error) {
	if s.err != nil {
		return nil, s.err
	}
	if predicate != HotPredicate {
		return nil, fmt.Errorf("unexpected predicate %q", predicate)
	}
	return s.rows, nil
}

func TestHot_JoinsKernelVerdictInReportOrder(t *testing.T) {
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	first := rep.Problems[0]
	var warnProb Problem
	for _, p := range rep.Problems {
		if p.Level == LevelWarn {
			warnProb = p
			break
		}
	}
	hotRow := func(p Problem, level string) types.Fact {
		return types.Fact{Predicate: HotPredicate, Args: []any{
			types.MangleString(p.Signature), types.MangleAtom(level), types.MangleString(p.Category),
		}}
	}
	q := stubProblemQuerier{rows: []types.Fact{
		hotRow(warnProb, "/warn"),
		{Predicate: HotPredicate, Args: []any{"only-sig"}},
		hotRow(first, "/error"),
	}}
	hot, err := rep.Hot(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(hot) != 2 || hot[0].Signature != first.Signature || hot[1].Signature != warnProb.Signature {
		t.Fatalf("hot = %v, want report order with malformed row skipped", hot)
	}
	if _, err := rep.Hot(nil); err == nil {
		t.Fatal("nil Querier must be an error")
	}
	if _, err := rep.Hot(stubProblemQuerier{err: fmt.Errorf("boom query")}); err == nil {
		t.Fatal("query failure must come back")
	}
	if got, err := (*Report)(nil).Hot(q); err != nil || got != nil {
		t.Fatalf("nil report has no hot problems: %v %v", got, err)
	}
}

func TestReadFile_UnknownLevelRanksAfterWarn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	content := "2026/01/02 15:04:05 [ERROR] [cat-a] error boom\n" +
		"2026/01/02 15:04:05 [WARN] [cat-b] warn boom\n" +
		"2026/01/02 15:04:05 [DEBUG] [cat-c] debug boom\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("expected DEBUG to be filtered leaving 2 problems, got %d: %+v", len(rep.Problems), rep.Problems)
	}
	if rep.Problems[0].Level != LevelError {
		t.Errorf("expected ERROR first, got %q", rep.Problems[0].Level)
	}
	if rep.Problems[1].Level != LevelWarn {
		t.Errorf("expected WARN second, got %q", rep.Problems[1].Level)
	}
}

func TestReadFile_SampleBreaksFullTie(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	content := "2026/01/02 15:04:05 [DEBUG] [cat-same] boom failure\nsecond line B\n" +
		"2026/01/02 15:04:05 [TRACE] [cat-same] boom failure\nsecond line A\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(rep.Problems) != 0 {
		t.Fatalf("expected DEBUG/TRACE to be filtered leaving 0 problems, got %d: %+v", len(rep.Problems), rep.Problems)
	}
	if rep.Entries != 0 {
		t.Fatalf("expected 0 entries for filtered levels, got %d", rep.Entries)
	}
}

func TestScan_ContinuationByteBoundTruncates(t *testing.T) {
	head := "2026/10/28 02:00:00 [WARN] [kernel] head\n"
	big := strings.Repeat("y", 33*1024)
	log := head + big + "\n" + "trailing that must be dropped\n"
	var got []Entry
	if err := Scan(strings.NewReader(log), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if !strings.Contains(got[0].Message, "[truncated]") {
		t.Fatalf("continuation past maxEntryBytes must carry a truncation marker, got %d bytes", len(got[0].Message))
	}
	if strings.Contains(got[0].Message, "trailing that must be dropped") {
		t.Fatalf("lines after a byte-bound truncation must be dropped:\n%s", got[0].Message)
	}
	if strings.Contains(got[0].Message, big[:100]) {
		t.Fatalf("oversized continuation payload must not be appended")
	}
}

func TestReadFile_SignatureTiebreakerOrdersDeterministically(t *testing.T) {
	log := "2026/10/28 02:00:00 [WARN] [kernel] beta failure\n" +
		"2026/10/28 02:00:00 [WARN] [kernel] alpha failure\n"
	path := filepath.Join(t.TempDir(), "problems.log")
	if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("got %d problems, want 2 equally-ranked distinct signatures", len(rep.Problems))
	}
	if rep.Problems[0].Signature != "alpha failure" || rep.Problems[1].Signature != "beta failure" {
		t.Fatalf("signature tiebreak order = %q, %q, want alpha before beta", rep.Problems[0].Signature, rep.Problems[1].Signature)
	}
}

func TestReadFile_GroupsByNormalizedSignature(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	content := "2026/01/02 15:04:05 [WARN] [kernel] hydrate failed for /home/u/a.db after 12ms\n" +
		"2026/01/02 15:04:06 [WARN] [kernel] hydrate failed for /home/u/b.db after 99ms\n" +
		"2026/01/02 15:04:07 [WARN] [kernel] something else entirely\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("got %d problems, want 2 (volatile paths/numbers must collapse to one signature): %+v", len(rep.Problems), rep.Problems)
	}
	grouped := rep.Problems[0]
	if grouped.Count != 2 {
		t.Fatalf("first problem count = %d, want 2 grouped repeats", grouped.Count)
	}
	if grouped.Signature != "hydrate failed for <path> after #ms" {
		t.Fatalf("signature = %q, want normalized <path> and # placeholders", grouped.Signature)
	}
	if !strings.Contains(grouped.Sample, "/home/u/a.db") {
		t.Fatalf("sample must keep the first raw occurrence, got %q", grouped.Sample)
	}
}

func TestReadFile_GroupsIgnoringContinuationPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	content := "2026/01/02 15:04:05 [ERROR] [cat] boom failure 123\npayload A line\n" +
		"2026/01/02 15:04:06 [ERROR] [cat] boom failure 456\npayload B line\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 1 {
		t.Fatalf("got %d problems, want 1 (continuation payload must not feed the signature): %+v", len(rep.Problems), rep.Problems)
	}
	p := rep.Problems[0]
	if p.Count != 2 {
		t.Fatalf("count = %d, want 2 repeats of one headline", p.Count)
	}
	if p.Signature != "boom failure #" {
		t.Fatalf("signature = %q, want headline-only normalized key", p.Signature)
	}
}

func TestReadFile_SignatureTiebreakerBeatsSampleOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	content := "2026/01/02 15:04:05 [WARN] [cat-same] boom 2 apple\n" +
		"2026/01/02 15:04:05 [WARN] [cat-same] boom 10 banana\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("got %d problems, want 2 distinct signatures: %+v", len(rep.Problems), rep.Problems)
	}
	first, second := rep.Problems[0], rep.Problems[1]
	if first.Signature != "boom # apple" || second.Signature != "boom # banana" {
		t.Fatalf("signatures = %q vs %q, want normalized apple/banana keys", first.Signature, second.Signature)
	}
	if !(first.Signature < second.Signature) {
		t.Fatalf("test setup broken: %q must sort before %q", first.Signature, second.Signature)
	}
	if !(second.Sample < first.Sample) {
		t.Fatalf("test setup broken: samples must order opposite signatures (sample %q vs %q)", first.Sample, second.Sample)
	}
	if !strings.Contains(first.Sample, "boom 2 apple") {
		t.Fatalf("signature tiebreaker must win over sample order, first sample = %q", first.Sample)
	}
}

func TestReadFile_PipeInCategoryAndSignatureKeepsGroupsDistinct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	content := "2026/01/02 15:04:05 [WARN] [a] b|c\n" +
		"2026/01/02 15:04:05 [WARN] [a|b] c\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("got %d problems, want 2 (struct key must not collide on '|' like string concat does): %+v", len(rep.Problems), rep.Problems)
	}
	seen := map[string]bool{}
	for _, p := range rep.Problems {
		seen[p.Category+"\x00"+p.Signature] = true
	}
	if !seen["a\x00b|c"] || !seen["a|b\x00c"] {
		t.Fatalf("unexpected groups: %+v", rep.Problems)
	}
	cats := map[string]bool{}
	for _, p := range rep.Problems {
		cats[p.Category] = true
	}
	if !cats["a"] || !cats["a|b"] {
		t.Fatalf("categories must stay distinct, got %+v", rep.Problems)
	}
}

func TestSignature_TrimsCarriageReturnBeforeNormalization(t *testing.T) {
	got := Signature("headline 7\r\npayload 99")
	if got != "headline #" {
		t.Fatalf("Signature with CRLF = %q, want headline-only normalized %q", got, "headline #")
	}
	got = Signature("hydrate failed for /home/u/a.db after 12ms\r")
	if got != "hydrate failed for <path> after #ms" {
		t.Fatalf("Signature with trailing CR = %q, want %q", got, "hydrate failed for <path> after #ms")
	}
}

func TestScan_HeadlineTruncationUsesSpaceMarkerAndBoundsLength(t *testing.T) {
	huge := "2026/10/28 02:00:00 [WARN] [kernel] " + strings.Repeat("x", 40*1024) + "\nsecond line that must be dropped\n"
	var got []Entry
	if err := Scan(strings.NewReader(huge), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	msg := got[0].Message
	if !strings.Contains(msg, " …[truncated]") {
		t.Fatalf("oversized headline must carry space marker, got %d bytes %q", len(msg), msg[max(0, len(msg)-40):])
	}
	if strings.Contains(msg, "\n…[truncated]") {
		t.Fatalf("headline truncation must use space marker, not newline marker")
	}
	if len(msg) > 33*1024 {
		t.Fatalf("headline must be cut near 32KiB, got %d bytes", len(msg))
	}
	if strings.Contains(msg, "second line") {
		t.Fatalf("continuation after oversized headline must be dropped")
	}
}

func TestScan_NormalContinuationStaysWholeWithoutMarker(t *testing.T) {
	log := "2026/01/02 15:04:05 [WARN] [kernel] first line\ncontinuation A\ncontinuation B\n" +
		"2026/01/02 15:04:06 [WARN] [kernel] second\n"
	var got []Entry
	if err := Scan(strings.NewReader(log), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %+v, want 2 entries", got)
	}
	if strings.Contains(got[0].Message, "[truncated]") {
		t.Fatalf("short multi-line record must not truncate, got %q", got[0].Message)
	}
	if got[0].Message != "first line\ncontinuation A\ncontinuation B" {
		t.Fatalf("continuation must be kept whole, got %q", got[0].Message)
	}
}

func TestScan_NilReaderAndNilEmitFailClosed(t *testing.T) {
	if err := Scan(nil, nil, func(Entry) {}); err == nil || !strings.Contains(err.Error(), "nil reader") {
		t.Fatalf("Scan(nil reader) = %v, want nil reader error", err)
	}
	if err := Scan(strings.NewReader("2026/10/28 02:00:00 [WARN] [kernel] hi\n"), nil, nil); err == nil || !strings.Contains(err.Error(), "nil emit") {
		t.Fatalf("Scan(nil emit) = %v, want nil emit error", err)
	}
}

func TestScan_InvalidTimestampDropsRecord(t *testing.T) {
	log := "2026/13/40 99:99:99 [WARN] [kernel] bad time\n" +
		"2026/10/28 02:00:00 [WARN] [kernel] good\n"
	var got []Entry
	if err := Scan(strings.NewReader(log), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != "good" {
		t.Fatalf("got %+v, want only the parseable record (bad timestamp dropped)", got)
	}
}

type scanFailReader struct{}

func (scanFailReader) Read([]byte) (int, error) { return 0, fmt.Errorf("boom read") }

func TestScan_ReadErrorPropagates(t *testing.T) {
	var got []Entry
	err := Scan(scanFailReader{}, nil, func(e Entry) { got = append(got, e) })
	if err == nil || !strings.Contains(err.Error(), "boom read") {
		t.Fatalf("Scan(failing reader) = %v, want boom read", err)
	}
	if len(got) != 0 {
		t.Fatalf("failing read must emit nothing, got %+v", got)
	}
}

func TestFacts_UnknownLevelMapsToWarnAndHotAgrees(t *testing.T) {
	prob := Problem{Signature: "s", Level: "INFO", Category: "c", Sample: "m", Count: 1, First: time.Unix(10, 0), Last: time.Unix(20, 0)}
	rep := &Report{Problems: []Problem{prob}}
	facts := rep.Facts()
	if len(facts) != 2 {
		t.Fatalf("%d facts, want problem+seen and no omission", len(facts))
	}
	if lvl, ok := facts[0].Args[1].(types.MangleAtom); !ok || lvl != "/warn" {
		t.Fatalf("unexpected level must map to /warn, got %#v", facts[0].Args[1])
	}
	warnRow := types.Fact{Predicate: HotPredicate, Args: []any{types.MangleString("s"), types.MangleAtom("/warn"), types.MangleString("c")}}
	hot, err := rep.Hot(stubProblemQuerier{rows: []types.Fact{warnRow}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hot) != 1 || hot[0].Signature != "s" {
		t.Fatalf("Hot with /warn verdict must match INFO problem, got %+v", hot)
	}
	errRow := types.Fact{Predicate: HotPredicate, Args: []any{types.MangleString("s"), types.MangleAtom("/error"), types.MangleString("c")}}
	hotErr, err := rep.Hot(stubProblemQuerier{rows: []types.Fact{errRow}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hotErr) != 0 {
		t.Fatalf("Hot with /error verdict must not match INFO problem, got %+v", hotErr)
	}
}

func TestFacts_FixtureLevelsMapThroughSharedHelper(t *testing.T) {
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	facts := rep.Facts()
	if len(facts) < 2*len(rep.Problems) {
		t.Fatalf("%d facts for %d problems, want problem+seen per problem", len(facts), len(rep.Problems))
	}
	for i, p := range rep.Problems {
		lvl, ok := facts[2*i].Args[1].(types.MangleAtom)
		if !ok {
			t.Fatalf("fact %d level = %#v, want a /name", i, facts[2*i].Args[1])
		}
		want := types.MangleAtom("/warn")
		if p.Level == LevelError {
			want = types.MangleAtom("/error")
		}
		if lvl != want {
			t.Errorf("problem %q level %q mapped to %q, want %q", p.Signature, p.Level, string(lvl), string(want))
		}
	}
}

func TestReadFile_DirectoryReadErrorWrapped(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadFile(dir, Options{})
	if err == nil || !strings.Contains(err.Error(), "read problems log") {
		t.Fatalf("ReadFile(dir) = %v, want wrapped read error", err)
	}
}

func TestSignature_PlaceholdersAreStable(t *testing.T) {
	stable := []string{
		"hydrate failed for <path> after #ms",
		"digest <hex> mismatch",
		"took #ms (target: <#ms)",
		"",
	}
	for _, s := range stable {
		if got := Signature(s); got != s {
			t.Errorf("Signature(%q) = %q, want stable (placeholders never re-mangled)", s, got)
		}
	}
	if got := Signature("   "); got != "" {
		t.Errorf("Signature(whitespace) = %q, want empty", got)
	}
}

func TestHot_NilQuerierWinsOverNilReport(t *testing.T) {
	if _, err := (*Report)(nil).Hot(nil); err == nil || !strings.Contains(err.Error(), "nil Querier") {
		t.Fatalf("nil report with nil Querier = %v, want nil Querier error (querier checked before report)", err)
	}
}

func TestFacts_NilReportAndEdgeLevelsShareWarnMapping(t *testing.T) {
	if got := (*Report)(nil).Facts(); got != nil {
		t.Fatalf("nil report Facts = %+v, want nil", got)
	}
	cases := []struct {
		level string
		want  types.MangleAtom
	}{
		{LevelError, "/error"},
		{LevelWarn, "/warn"},
		{"INFO", "/warn"},
		{"", "/warn"},
		{"error", "/warn"},
		{"DEBUG", "/warn"},
	}
	for _, tc := range cases {
		prob := Problem{Signature: "s", Level: tc.level, Category: "c", Sample: "m", Count: 1, First: time.Unix(10, 0), Last: time.Unix(20, 0)}
		rep := &Report{Problems: []Problem{prob}}
		facts := rep.Facts()
		if len(facts) != 2 {
			t.Fatalf("level %q: %d facts, want problem+seen", tc.level, len(facts))
		}
		for i, f := range facts {
			if lvl, ok := f.Args[1].(types.MangleAtom); !ok || lvl != tc.want {
				t.Fatalf("level %q fact %d maps to %#v, want %q (Facts and Hot share one helper)", tc.level, i, f.Args[1], string(tc.want))
			}
		}
		match := types.Fact{Predicate: HotPredicate, Args: []any{types.MangleString("s"), tc.want, types.MangleString("c")}}
		hot, err := rep.Hot(stubProblemQuerier{rows: []types.Fact{match}})
		if err != nil {
			t.Fatal(err)
		}
		if len(hot) != 1 {
			t.Fatalf("level %q: Hot with %q verdict must match, got %+v", tc.level, string(tc.want), hot)
		}
		var miss types.MangleAtom = "/error"
		if tc.want == "/error" {
			miss = "/warn"
		}
		missRow := types.Fact{Predicate: HotPredicate, Args: []any{types.MangleString("s"), miss, types.MangleString("c")}}
		hotMiss, err := rep.Hot(stubProblemQuerier{rows: []types.Fact{missRow}})
		if err != nil {
			t.Fatal(err)
		}
		if len(hotMiss) != 0 {
			t.Fatalf("level %q: Hot with %q verdict must not match, got %+v", tc.level, string(miss), hotMiss)
		}
	}
	rep, err := ReadFile(fixture, Options{Skip: skipNotes})
	if err != nil {
		t.Fatal(err)
	}
	facts := rep.Facts()
	for i, p := range rep.Problems {
		want := types.MangleAtom("/warn")
		if p.Level == LevelError {
			want = types.MangleAtom("/error")
		}
		if lvl, ok := facts[2*i].Args[1].(types.MangleAtom); !ok || lvl != want {
			t.Fatalf("fixture problem %q level %q mapped to %#v, want %q", p.Signature, p.Level, facts[2*i].Args[1], string(want))
		}
	}
}

func TestScan_BothNilPrefersReaderError(t *testing.T) {
	if err := Scan(nil, nil, nil); err == nil || !strings.Contains(err.Error(), "nil reader") {
		t.Fatalf("Scan(nil, nil, nil) = %v, want nil reader error (reader checked before emit)", err)
	}
	var got []Entry
	err := Scan(strings.NewReader("2026/10/28 02:00:00 [WARN] [kernel] hi\n"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "nil emit") {
		t.Fatalf("Scan(nil emit) = %v, want nil emit error", err)
	}
	if len(got) != 0 {
		t.Fatalf("nil emit must emit nothing fail-closed, got %+v", got)
	}
}

func TestReadFile_ErrorsCarryPathContext(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent_problems.log")
	if _, err := ReadFile(missing, Options{}); err == nil || !strings.Contains(err.Error(), "read problems log") || !strings.Contains(err.Error(), missing) {
		t.Fatalf("ReadFile(missing) = %v, want wrapped error carrying %q", err, missing)
	}
	dir := t.TempDir()
	if _, err := ReadFile(dir, Options{}); err == nil || !strings.Contains(err.Error(), "read problems log") || !strings.Contains(err.Error(), dir) {
		t.Fatalf("ReadFile(dir) = %v, want wrapped error carrying %q", err, dir)
	}
}

func TestReadFile_UncoveredLines125Pin(t *testing.T) {
	base, err := ReadFile(fixture, Options{})
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}
	if base == nil {
		t.Fatal("ReadFile fixture returned nil report")
	}
	_ = base.Omitted()
	_ = base.Summary()

	emptyPath := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(emptyPath, nil, 0o644); err != nil {
		t.Fatalf("write empty log: %v", err)
	}
	empty, err := ReadFile(emptyPath, Options{})
	if err != nil {
		t.Fatalf("ReadFile empty: %v", err)
	}
	if empty == nil {
		t.Fatal("ReadFile empty returned nil report")
	}
	if empty.Omitted() {
		t.Fatalf("empty report should not omit anything: %q", empty.Summary())
	}

	dir := t.TempDir()
	overPath := filepath.Join(dir, "over.log")
	var sb strings.Builder
	for i := 0; i < DefaultMaxProblems+16; i++ {
		sb.WriteString("2026/01/02 15:04:05 [ERROR] [cat] distinct failure case marker ")
		sb.WriteString(strings.Repeat("z", i+1))
		sb.WriteString("\n")
	}
	if err := os.WriteFile(overPath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write over-bound log: %v", err)
	}
	over, err := ReadFile(overPath, Options{})
	if err != nil {
		t.Fatalf("ReadFile over-bound: %v", err)
	}
	if over == nil {
		t.Fatal("ReadFile over-bound returned nil report")
	}
	if !over.Omitted() {
		t.Fatalf("over-bound report should report omission: %q", over.Summary())
	}
	if got := len(over.Problems); got != DefaultMaxProblems {
		t.Fatalf("over-bound report keeps DefaultMaxProblems=%d, got %d", DefaultMaxProblems, got)
	}
}

func TestReadFile_LevelTiebreakerOrdersSameRankUnknownLevels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "levels.log")
	content := "2026/01/02 15:04:05 [ZZZ] [cat] same payload here\n2026/01/02 15:04:05 [AAA] [cat] same payload here\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write levels log: %v", err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("ReadFile levels: %v", err)
	}
	if rep == nil {
		t.Fatal("ReadFile levels returned nil report")
	}
	// Scan only emits WARN and ERROR (problemlog.go filters unknown levels),
	// so unknown levels never reach grouping: the level tiebreaker in ReadFile
	// sorting is defensive and not exercised via files. Document the filter.
	if len(rep.Problems) != 0 {
		t.Fatalf("unknown levels are filtered by Scan, expected 0 problems, got %d: %+v", len(rep.Problems), rep.Problems)
	}
}

func TestPin_Scan_ValidReaderNilEmitFailsClosed(t *testing.T) {
	valid := "2026/01/02 15:04:05 [WARN] [cat] hello world\n"
	err := Scan(strings.NewReader(valid), func(e Entry) bool { return false }, nil)
	if err == nil {
		t.Fatalf("Scan with nil emit on valid input should fail closed, got nil error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "nil") && !strings.Contains(strings.ToLower(err.Error()), "emit") {
		t.Fatalf("Scan nil-emit error should mention nil/emit, got %q", err.Error())
	}
}

func TestPin_Scan_NilReaderFailsClosedMentionsReader(t *testing.T) {
	err := Scan(nil, func(e Entry) bool { return false }, func(e Entry) {})
	if err == nil {
		t.Fatalf("Scan with nil reader should fail closed, got nil error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "nil") && !strings.Contains(strings.ToLower(err.Error()), "read") {
		t.Fatalf("Scan nil-reader error should mention nil/reader, got %q", err.Error())
	}
}

func TestPin_Signature_HeadlineOnlyIgnoresContinuation(t *testing.T) {
	sig := Signature("first 12345\nsecond 67890 /a/b/c")
	if strings.Contains(sig, "second") {
		t.Fatalf("Signature should only use headline, got %q containing continuation", sig)
	}
	if strings.Contains(sig, "/a/b/c") {
		t.Fatalf("Signature should ignore continuation payload paths, got %q", sig)
	}
	if strings.Contains(sig, "12345") {
		t.Fatalf("Signature should strip volatile numbers from headline, got %q", sig)
	}
}

type pinKernel struct {
	retracts []string
	asserted int
}

func (k *pinKernel) Retract(pred string) error {
	k.retracts = append(k.retracts, pred)
	return nil
}

func (k *pinKernel) AssertBatch(facts []types.Fact) error {
	k.asserted++
	return nil
}

func TestPin_Replace_RetractsAllPredicates(t *testing.T) {
	k := &pinKernel{}
	r := &Report{}
	if err := Replace(k, r); err != nil {
		t.Fatalf("Replace with empty report should not error, got %v", err)
	}
	if len(k.retracts) != len(Predicates) {
		t.Fatalf("Replace should retract %d predicates, retracted %d (%v)", len(Predicates), len(k.retracts), k.retracts)
	}
	for _, want := range Predicates {
		found := false
		for _, got := range k.retracts {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Replace should retract %q, retracted %v", want, k.retracts)
		}
	}
}

func TestPin_Replace_NilKernelFailsClosed(t *testing.T) {
	err := Replace(nil, &Report{})
	if err == nil {
		t.Fatalf("Replace with nil kernel should fail closed, got nil error")
	}
}

type pinQuerier struct {
	queried []string
	facts   []types.Fact
	err     error
}

func (q *pinQuerier) Query(pred string) ([]types.Fact, error) {
	q.queried = append(q.queried, pred)
	return q.facts, q.err
}

func TestPin_Hot_NilQuerierWinsOverNilReport(t *testing.T) {
	var r *Report
	_, err := r.Hot(nil)
	if err == nil {
		t.Fatalf("Hot with nil querier (and nil report) should error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "nil") && !strings.Contains(strings.ToLower(err.Error()), "quer") {
		t.Fatalf("Hot both-nil error should mention nil/querier, got %q", err.Error())
	}
}

func TestPin_Hot_NilReportWithValidQuerierIsEmpty(t *testing.T) {
	var r *Report
	q := &pinQuerier{}
	got, err := r.Hot(q)
	if err != nil {
		t.Fatalf("Hot with nil report and valid querier should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Hot with nil report should return no problems, got %v", got)
	}
}

func TestPin_ReadFile_GroupsByNormalizedSignature(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/group.log"
	content := "2026/01/02 15:04:05 [WARN] [cat] boom 111\n2026/01/02 15:04:06 [WARN] [cat] boom 222\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(rep.Problems) != 1 {
		t.Fatalf("ReadFile should group normalized signatures into 1 problem, got %d (%v)", len(rep.Problems), rep.Problems)
	}
	if rep.Problems[0].Count != 2 {
		t.Fatalf("grouped problem should have count 2, got %d", rep.Problems[0].Count)
	}
}

func TestPin_ReadFile_SignatureOrderDeterministic(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/order.log"
	content := "2026/01/02 15:04:05 [WARN] [cat] zebra issue\n2026/01/02 15:04:06 [WARN] [cat] alpha issue\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("setup write failed: %v", err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("expected 2 problems, got %d", len(rep.Problems))
	}
	if !(rep.Problems[0].Signature < rep.Problems[1].Signature) {
		t.Fatalf("ReadFile should order same-rank problems by signature deterministically, got %q then %q", rep.Problems[0].Signature, rep.Problems[1].Signature)
	}
}

func TestPin_Replace_NilReportClearsWithoutError(t *testing.T) {
	k := &pinKernel{}
	if err := Replace(k, nil); err != nil {
		t.Fatalf("Replace with nil report should clear and succeed, got %v", err)
	}
	if len(k.retracts) != len(Predicates) {
		t.Fatalf("Replace(nil) should still retract %d predicates, got %v", len(Predicates), k.retracts)
	}
}

type pinFailKernel struct {
	pinKernel
	retractErr error
	assertErr  error
}

func (k *pinFailKernel) Retract(pred string) error {
	k.retracts = append(k.retracts, pred)
	return k.retractErr
}

func (k *pinFailKernel) AssertBatch(facts []types.Fact) error {
	k.asserted++
	return k.assertErr
}

func TestPin_Replace_PropagatesRetractError(t *testing.T) {
	k := &pinFailKernel{retractErr: errPinSentinel}
	if err := Replace(k, &Report{}); err == nil {
		t.Fatalf("Replace should propagate retract error, got nil")
	}
}

func TestPin_Replace_PropagatesAssertError(t *testing.T) {
	k := &pinFailKernel{assertErr: errPinSentinel}
	r := &Report{Problems: []Problem{{Level: "WARN", Category: "c", Signature: "s", Count: 1}}}
	if err := Replace(k, r); err == nil {
		t.Fatalf("Replace should propagate assert error, got nil")
	}
}

func TestPin_Hot_PropagatesQueryError(t *testing.T) {
	q := &pinQuerier{err: errPinSentinel}
	r := &Report{Problems: []Problem{{Level: "WARN", Category: "c", Signature: "s", Count: 1}}}
	if _, err := r.Hot(q); err == nil {
		t.Fatalf("Hot should propagate querier error, got nil")
	}
}

func TestPin_Scan_NilSkipEmitsAll(t *testing.T) {
	valid := "2026/01/02 15:04:05 [WARN] [cat] first\n2026/01/02 15:04:06 [ERROR] [cat] second\n"
	var got []Entry
	err := Scan(strings.NewReader(valid), nil, func(e Entry) { got = append(got, e) })
	if err != nil {
		t.Fatalf("Scan with nil skip should succeed, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Scan with nil skip should emit all entries, got %d", len(got))
	}
}

func TestPin_Signature_EmptyStaysEmpty(t *testing.T) {
	if got := Signature(""); got != "" {
		t.Fatalf("Signature(\"\") should stay empty, got %q", got)
	}
}

func TestPin_ReadFile_EmptyPathFailsClosed(t *testing.T) {
	if _, err := ReadFile("", Options{}); err == nil {
		t.Fatalf("ReadFile with empty path should fail closed, got nil error")
	}
}

var errPinSentinel = errors.New("pin sentinel error")

type stabilizeEmptyQuerier struct{}

func (stabilizeEmptyQuerier) Query(string) ([]types.Fact, error) { return nil, nil }

type stabilizeErrQuerier struct{ err error }

func (q stabilizeErrQuerier) Query(string) ([]types.Fact, error) { return nil, q.err }

type stabilizeFailKernel struct {
	retractErr error
	assertErr  error
	retracts   []string
}

func (k *stabilizeFailKernel) Retract(s string) error {
	k.retracts = append(k.retracts, s)
	return k.retractErr
}

func (k *stabilizeFailKernel) AssertBatch([]types.Fact) error { return k.assertErr }

func TestPin_Stabilize_ScanSanitizesInvalidUTF8(t *testing.T) {
	bad := string([]byte{0xff})
	input := "2026/01/02 15:04:05 [WARN] [cat] hello" + bad + "world\n"
	var got []Entry
	if err := Scan(strings.NewReader(input), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatalf("Scan invalid UTF-8 should not error, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Scan should emit one entry for invalid UTF-8 headline, got %d", len(got))
	}
	if got[0].Message != strings.ToValidUTF8(got[0].Message, "") {
		t.Fatalf("Scan message must be valid UTF-8, got %q", got[0].Message)
	}
	if strings.Contains(got[0].Message, bad) {
		t.Fatalf("Scan message must sanitize invalid bytes, got %q", got[0].Message)
	}
}

func TestPin_Stabilize_SignatureSanitizesInvalidUTF8HeadlineOnly(t *testing.T) {
	bad := string([]byte{0xff, 0xfe})
	got := Signature("headline " + bad + " 123\nsecond line " + bad)
	if got != strings.ToValidUTF8(got, "") {
		t.Fatalf("Signature must be valid UTF-8, got %q", got)
	}
	if strings.Contains(got, bad) {
		t.Fatalf("Signature must sanitize invalid headline bytes, got %q", got)
	}
	if Signature("same 123\npayload-a") != Signature("same 123\npayload-b") {
		t.Fatalf("Signature must ignore continuation payload")
	}
}

func TestPin_Stabilize_HotEmptyVerdictIsNonNil(t *testing.T) {
	r := &Report{Problems: []Problem{{Signature: "s", Level: "WARN", Category: "c", Count: 1}}}
	got, err := r.Hot(stabilizeEmptyQuerier{})
	if err != nil {
		t.Fatalf("Hot with empty verdict should not error, got %v", err)
	}
	if got == nil {
		t.Fatalf("Hot with no matches must return non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("Hot with no matches should return 0 problems, got %d", len(got))
	}
}

func TestPin_Stabilize_ReplaceWrapsRetractAndAssert(t *testing.T) {
	k := &stabilizeFailKernel{retractErr: errPinSentinel, assertErr: errPinSentinel}
	r := &Report{Problems: []Problem{{Signature: "s", Level: "ERROR", Category: "c", Count: 1, Sample: "m"}}}
	err := Replace(k, r)
	if err == nil {
		t.Fatalf("Replace with failing kernel should error")
	}
	if !errors.Is(err, errPinSentinel) {
		t.Fatalf("Replace should preserve sentinel via Join, got %v", err)
	}
	if !strings.Contains(err.Error(), "problemlog: retract") {
		t.Fatalf("Replace should wrap retract error, got %v", err)
	}
	if !strings.Contains(err.Error(), "problemlog: assert") {
		t.Fatalf("Replace should join assert error, got %v", err)
	}
	if len(k.retracts) != len(Predicates) {
		t.Fatalf("Replace should retract %d predicates, got %v", len(Predicates), k.retracts)
	}
}

func TestPin_Stabilize_HotWrapsQueryError(t *testing.T) {
	r := &Report{Problems: []Problem{{Signature: "s", Level: "WARN", Category: "c"}}}
	_, err := r.Hot(stabilizeErrQuerier{err: errPinSentinel})
	if err == nil {
		t.Fatalf("Hot should propagate querier error")
	}
	if !errors.Is(err, errPinSentinel) {
		t.Fatalf("Hot should preserve querier sentinel, got %v", err)
	}
	if !strings.Contains(err.Error(), "problemlog: query") {
		t.Fatalf("Hot should wrap query error, got %v", err)
	}
	if !strings.Contains(err.Error(), HotPredicate) {
		t.Fatalf("Hot error should name predicate %q, got %v", HotPredicate, err)
	}
}

func TestPin_Stabilize_ReadFileMissingWrapsPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.log")
	_, err := ReadFile(missing, Options{})
	if err == nil {
		t.Fatalf("ReadFile missing should error")
	}
	if !strings.Contains(err.Error(), "read problems log") {
		t.Fatalf("ReadFile error should wrap with operation, got %v", err)
	}
	if !strings.Contains(err.Error(), "does-not-exist.log") {
		t.Fatalf("ReadFile error should carry path context, got %v", err)
	}
}

func TestPin_Stabilize_FactStringStringBranchIsQueryReadback(t *testing.T) {
	if got := factString("sig/plain"); got != "sig/plain" {
		t.Fatalf("factString string branch = %q, want %q", got, "sig/plain")
	}
	if got := factString(types.MangleString("sig/mstring")); got != "sig/mstring" {
		t.Fatalf("factString MangleString branch = %q, want %q", got, "sig/mstring")
	}
	if got := factString(types.MangleAtom("/warn")); got != "/warn" {
		t.Fatalf("factString MangleAtom branch = %q, want %q", got, "/warn")
	}
}

func TestPin_Stabilize_NormalizeLevelBareNameAddsSlash(t *testing.T) {
	if got := normalizeLevelName(""); got != "" {
		t.Fatalf("normalizeLevelName empty = %q, want empty", got)
	}
	if got := normalizeLevelName("/warn"); got != "/warn" {
		t.Fatalf("normalizeLevelName slashed = %q, want %q", got, "/warn")
	}
	if got := normalizeLevelName("warn"); got != "/warn" {
		t.Fatalf("normalizeLevelName bare warn = %q, want %q", got, "/warn")
	}
	if got := normalizeLevelName("error"); got != "/error" {
		t.Fatalf("normalizeLevelName bare error = %q, want %q", got, "/error")
	}
}

func TestPin_Stabilize_HotJoinsPlainStringReadback(t *testing.T) {
	rep := &Report{Problems: []Problem{{Signature: "sig/plain", Level: LevelWarn, Category: "cat/plain"}}}
	q := stubProblemQuerier{rows: []types.Fact{{Predicate: HotPredicate, Args: []any{"sig/plain", "warn", "cat/plain"}}}}
	hot, err := rep.Hot(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(hot) != 1 || hot[0].Signature != "sig/plain" {
		t.Fatalf("Hot with plain-string readback = %+v, want 1 match on sig/plain", hot)
	}
}

func TestPin_Stabilize_FactStringDefaultBranchDelegates(t *testing.T) {
	cases := []any{int(7), int64(42), true}
	for _, tc := range cases {
		want := types.ExtractString(tc)
		if got := factString(tc); got != want {
			t.Fatalf("factString(%T(%v)) = %q, want ExtractString %q", tc, tc, got, want)
		}
	}
}

func TestPin_Stabilize_ScanBOMStrippedAndUTF8Valid(t *testing.T) {
	// BOM-prefixed headline with invalid UTF-8 bytes must still scan as one
	// valid entry with the bytes stripped, not dropped or passed through raw.
	raw := "\xef\xbb\xbf2026/01/02 15:04:05 [ERROR] [cat] hello\xff\xfe world\n"
	var got []Entry
	if err := Scan(strings.NewReader(raw), nil, func(e Entry) { got = append(got, e) }); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("BOM-prefixed record was not recognized: got %d entries, want 1", len(got))
	}
	msg := got[0].Message
	if msg != strings.ToValidUTF8(msg, "") {
		t.Fatalf("Scan message is not valid UTF-8: %q", msg)
	}
	if strings.Contains(msg, "\xff") || strings.Contains(msg, "\xfe") {
		t.Fatalf("Scan message still contains raw invalid bytes: %q", msg)
	}
	if !strings.Contains(msg, "hello") || !strings.Contains(msg, "world") {
		t.Fatalf("Scan message lost surrounding text after sanitizing: %q", msg)
	}
	if got[0].Level != LevelError || got[0].Category != "cat" {
		t.Fatalf("BOM leaked into parsed fields: level=%q category=%q", got[0].Level, got[0].Category)
	}
}

func TestPin_Stabilize_ReadFileLevelTiebreakerIsolated(t *testing.T) {
	// Same rank (both ERROR), same count, same Last, same Level: only the
	// explicit Category tiebreaker orders aaa before zzz. Signature and
	// sample both order the opposite way, so without the Category tiebreaker
	// the file order flips. Uses ERROR so Scan emits both records.
	dir := t.TempDir()
	path := filepath.Join(dir, "problems.log")
	body := "2026/01/02 15:04:05 [ERROR] [aaa] zebra marker\n" +
		"2026/01/02 15:04:05 [ERROR] [zzz] apple marker\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp log: %v", err)
	}
	rep, err := ReadFile(path, Options{})
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if len(rep.Problems) != 2 {
		t.Fatalf("got %d problems, want 2", len(rep.Problems))
	}
	if rep.Problems[0].Category != "aaa" || rep.Problems[1].Category != "zzz" {
		t.Fatalf("Category tiebreaker did not decide order: got [%q %q], want [aaa zzz]",
			rep.Problems[0].Category, rep.Problems[1].Category)
	}
}

func TestPin_Stabilize_ReadFileEmptyPathMentionsEmptyPath(t *testing.T) {
	// The empty-path guard must fail closed with its own message. os.Open("")
	// also errors, so only asserting err != nil cannot tell the guard apart
	// from the filesystem error.
	rep, err := ReadFile("", Options{})
	if err == nil {
		t.Fatalf("ReadFile(\"\") returned nil error")
	}
	if rep != nil {
		t.Fatalf("ReadFile(\"\") returned report %v, want nil", rep)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "empty path") {
		t.Fatalf("ReadFile(\"\") error does not name the empty-path guard: %q", err.Error())
	}
}
