package problemlog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

const fixture = "testdata/problems.log"

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
	if len(facts) != 4*2+1 {
		t.Fatalf("%d facts, want a problem and a seen row per problem plus the omission", len(facts))
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
