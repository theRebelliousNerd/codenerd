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

	file := extractFileFromFindings(findings)
	if file != "internal/auth/session.go" {
		t.Errorf("delegated file = %q, want the most-cited file", file)
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
