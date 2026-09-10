package chat

import (
	"strings"
	"testing"
)

// The reviewer-to-fixer delegation chain was inert.
//
// extractFindings set only "raw" and "severity"; delegation.go read f["file"],
// f["line"] and f["message"]. So formatFindingsForTask produced an empty string
// and extractFileFromFindings returned "" on every review -- the fixer was
// handed no file and no findings, and nothing failed anywhere.
//
// The extractor and the reviewer atom also disagreed on the vocabulary itself:
// the atom says CRITICAL/HIGH/MEDIUM/LOW and the extractor looked for
// CRIT/ERR/WARN/INFO, so a finding written exactly as instructed matched
// nothing at all.

const reviewerOutput = `I reviewed the authentication path.

Findings:

- [CRITICAL] internal/auth/session.go:142: token comparison is not constant time
- [HIGH] internal/auth/session.go:203: the refresh error is swallowed
- [MEDIUM] internal/auth/login.go:88: this allocation is inside the request loop
- [LOW] internal/auth/login.go: naming is inconsistent with the package

Overall the design is sound.`

func TestExtractFindingsProducesWhatTheConsumersRead(t *testing.T) {
	findings := extractFindings(reviewerOutput)

	if len(findings) != 4 {
		t.Fatalf("findings = %d, want 4: %+v", len(findings), findings)
	}

	first := findings[0]
	if got := first["severity"]; got != "critical" {
		t.Errorf("severity = %v, want critical", got)
	}
	if got := first["file"]; got != "internal/auth/session.go" {
		t.Errorf("file = %v, want internal/auth/session.go", got)
	}
	if got := findingLineNumber(first); got != 142 {
		t.Errorf("line = %d, want 142", got)
	}
	if got, _ := first["message"].(string); !strings.Contains(got, "constant time") {
		t.Errorf("message = %q, want the finding text", got)
	}

	// A finding with no line is still a finding; the format allows omitting it
	// when the issue is not about one line.
	last := findings[3]
	if got := last["file"]; got != "internal/auth/login.go" {
		t.Errorf("file = %v, want the path even with no line", got)
	}
	if got := findingLineNumber(last); got != 0 {
		t.Errorf("line = %d, want 0 for a finding with no line", got)
	}
	if got := last["severity"]; got != "low" {
		t.Errorf("severity = %v, want low", got)
	}
}

func TestAtomSeverityVocabularyIsUnderstood(t *testing.T) {
	// The reviewer atom instructs CRITICAL/HIGH/MEDIUM/LOW. Every one of those
	// must land somewhere, or a compliant reviewer is ignored.
	out := `- [CRITICAL] a.go:1: c
- [HIGH] a.go:2: h
- [MEDIUM] a.go:3: m
- [LOW] a.go:4: l`

	want := []string{"critical", "high", "medium", "low"}
	findings := extractFindings(out)
	if len(findings) != len(want) {
		t.Fatalf("findings = %d, want %d", len(findings), len(want))
	}
	for i, w := range want {
		if got := findings[i]["severity"]; got != w {
			t.Errorf("finding %d severity = %v, want %v", i, got, w)
		}
	}
}

func TestLegacyMarkersStillMapOnto(t *testing.T) {
	// Older output, and other shards, use ERROR/WARN/INFO. Those still resolve
	// rather than being dropped for using a different word for the same thing.
	findings := extractFindings(`- [ERROR] a.go:1: boom
- [WARN] a.go:2: careful
- [INFO] a.go:3: note`)
	if len(findings) != 3 {
		t.Fatalf("findings = %d, want 3", len(findings))
	}
	for i, want := range []string{"high", "medium", "low"} {
		if got := findings[i]["severity"]; got != want {
			t.Errorf("finding %d severity = %v, want %v", i, got, want)
		}
	}
}

func TestMalformedFindingIsKeptNotDropped(t *testing.T) {
	// A severity marker with no parseable location is still evidence. Dropping
	// it loses a real finding to a formatting slip.
	findings := extractFindings("The reviewer said [CRITICAL] something is very wrong here")
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	if got := findings[0]["severity"]; got != "critical" {
		t.Errorf("severity = %v, want critical", got)
	}
	if _, hasFile := findings[0]["file"]; hasFile {
		t.Error("a finding with no parseable location claimed a file")
	}
}

func TestProseWithoutFindingsProducesNone(t *testing.T) {
	// A clean review must not manufacture findings out of ordinary sentences.
	findings := extractFindings("I read the whole package and it looks correct to me.\nNo issues.")
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
}

func TestDelegationGetsAFileAndFindingsText(t *testing.T) {
	// The end of the chain: this is what the fixer is actually handed, and what
	// used to be "" and "".
	findings := extractFindings(reviewerOutput)

	// session.go and login.go both carry two findings, so this is a TIE, and
	// the old assertion's stated reason ("the most-cited file") was simply
	// false — it passed about half the time on Go's randomised map order.
	// session.go wins because it holds the CRITICAL.
	file := extractFileFromFindings(findings)
	if file != "internal/auth/session.go" {
		t.Errorf("delegated file = %q, want the file holding the worst finding "+
			"among those cited equally often", file)
	}

	text := formatFindingsForTask(findings, "session.go")
	if text == "" {
		t.Fatal("the fixer would be handed no findings at all")
	}
	if !strings.Contains(text, "L142") {
		t.Errorf("formatted findings lost the line citation: %q", text)
	}
	if !strings.Contains(text, "critical") {
		t.Errorf("formatted findings lost the severity: %q", text)
	}
	// Filtering to the target file must exclude the other file's findings.
	if strings.Contains(text, "allocation") {
		t.Errorf("findings from login.go leaked into a session.go task: %q", text)
	}
}

func TestFindingLineNumberAcceptsEveryShapeItArrivesIn(t *testing.T) {
	// Findings reach the consumer two ways: parsed from text as a Go int, and
	// decoded from JSON as a float64. The consumer asserted float64 only, so an
	// extractor-produced finding silently reported line 0 -- which reads as "no
	// line known" and drops the citation the reviewer was required to give.
	for name, f := range map[string]map[string]any{
		"int":     {"line": 42},
		"int64":   {"line": int64(42)},
		"float64": {"line": float64(42)},
		"string":  {"line": "42"},
	} {
		if got := findingLineNumber(f); got != 42 {
			t.Errorf("%s: line = %d, want 42", name, got)
		}
	}
	for name, f := range map[string]map[string]any{
		"absent":  {},
		"nil":     {"line": nil},
		"garbage": {"line": "not a number"},
	} {
		if got := findingLineNumber(f); got != 0 {
			t.Errorf("%s: line = %d, want 0", name, got)
		}
	}
}

func TestTesterSummaryReadsTheRealOutput(t *testing.T) {
	// extractShardSummary asserted sr.Metrics["pass"].(int) against a map that
	// extractMetrics only ever fills with strings, so the counts were always
	// zero. And the map is never nil, so the branch always fired: a tester's
	// real output was REPLACED by "0 pass, 0 fail" in the context handed to the
	// next turn, rather than falling through to the generic summary.
	sr := &ShardResult{
		ShardType: "tester",
		RawOutput: "--- PASS: TestAlpha (0.01s)\n--- PASS: TestBeta (0.00s)\n--- FAIL: TestGamma (0.02s)\nFAIL",
		Metrics:   map[string]any{},
	}

	got := extractShardSummary(sr)
	if !strings.Contains(got, "2 pass") {
		t.Errorf("summary = %q, want the real pass count", got)
	}
	if !strings.Contains(got, "2 fail") && !strings.Contains(got, "1 fail") {
		t.Errorf("summary = %q, want the real fail count", got)
	}
	if strings.HasPrefix(got, "0 pass, 0 fail") {
		t.Errorf("summary = %q — still reporting zeroes over real output", got)
	}
}

func TestTesterSummaryFallsBackWhenOutputIsNotTestResults(t *testing.T) {
	// A summary that cannot read the output should show what the output said,
	// not invent counts. Reporting "1 pass" here -- the checkpoint runner's
	// optimistic convention -- would be a lie in a context window.
	sr := &ShardResult{
		ShardType: "tester",
		RawOutput: "I could not find a test command for this project.",
		Metrics:   map[string]any{},
	}

	got := extractShardSummary(sr)
	if strings.Contains(got, "pass") && strings.Contains(got, "fail") {
		t.Errorf("summary = %q, want the raw output rather than invented counts", got)
	}
	if !strings.Contains(got, "could not find a test command") {
		t.Errorf("summary = %q, want it to carry the real output", got)
	}
}

// The dispatch must not depend on Go's map iteration order.
//
// This is the property, not the instance: a review that cites two files
// equally must send the fixer to the same one every time, or the agent's
// behaviour changes run to run for no reason a reader could ever find.
func TestDelegatedFileIsDeterministicOnATie(t *testing.T) {
	findings := extractFindings(reviewerOutput)

	first := extractFileFromFindings(findings)
	if first == "" {
		t.Fatal("no file was selected at all")
	}
	// Enough iterations that a randomised map order would have shown itself:
	// with two tied keys, 200 draws miss a 50/50 flip with probability 2^-199.
	for i := 0; i < 200; i++ {
		if got := extractFileFromFindings(findings); got != first {
			t.Fatalf("iteration %d selected %q, first selected %q: the fixer is "+
				"dispatched by coin flip", i, got, first)
		}
	}
}

// Severity breaks a tie, because that is the rule worth having rather than
// merely a deterministic one.
func TestTiedFilesAreBrokenByWorstSeverity(t *testing.T) {
	findings := extractFindings(`
- [LOW] a/first.go:1: cosmetic
- [LOW] a/first.go:2: cosmetic
- [MEDIUM] b/second.go:3: a real problem
- [CRITICAL] b/second.go:4: a serious one`)

	if len(findings) != 4 {
		t.Fatalf("findings = %d, want 4: %+v", len(findings), findings)
	}
	// first.go is mentioned first and both files are cited twice; second.go
	// wins on the CRITICAL.
	if got := extractFileFromFindings(findings); got != "b/second.go" {
		t.Errorf("delegated file = %q, want b/second.go — equal citations, worse findings", got)
	}
}

// Count still outranks severity: a file cited five times with only LOW
// findings is where the work is, even if another file has one CRITICAL.
func TestCitationCountOutranksSeverity(t *testing.T) {
	findings := extractFindings(`
- [CRITICAL] rare/once.go:1: one bad thing
- [LOW] common/many.go:1: a
- [LOW] common/many.go:2: b
- [LOW] common/many.go:3: c`)

	if got := extractFileFromFindings(findings); got != "common/many.go" {
		t.Errorf("delegated file = %q, want common/many.go — severity breaks ties, "+
			"it does not override the count", got)
	}
}
