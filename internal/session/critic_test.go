package session

import (
	"context"
	"os"
	"path/filepath"

	"codenerd/internal/types"
	"strings"
	"testing"
	"time"
)

// F-CRITIC: the build and test gates prove edits compile and were executed;
// neither asks whether the edits are right. An adversarial reviewer closes
// that gap, but a reviewer rewarded for activity invents defects in sound
// code — which is worse than missing one. These tests pin the pure half of
// that reviewer: prompt construction, response parsing, and triage.

func TestBuildCriticPrompt(t *testing.T) {
	cases := []struct {
		name             string
		writtenFiles     map[string]string
		uncoveredSummary string
		wantContains     []string
		wantNotContains  []string
	}{
		{
			name:             "empty files and empty summary",
			writtenFiles:     nil,
			uncoveredSummary: "",
			wantContains: []string{
				"FINDING file.go:123 severity: claim text",
				"NO FINDINGS",
				"Inventing a finding to appear useful is worse than finding nothing",
			},
		},
		{
			name:             "empty map no file blocks",
			writtenFiles:     map[string]string{},
			uncoveredSummary: "",
			wantContains: []string{
				"NO FINDINGS",
			},
			wantNotContains: []string{
				"File:",
			},
		},
		{
			name: "single file embedded in fenced block",
			writtenFiles: map[string]string{
				"internal/foo/bar.go": "package foo\nfunc Foo() {}\n",
			},
			uncoveredSummary: "",
			wantContains: []string{
				"File: internal/foo/bar.go",
				"package foo",
				"func Foo()",
				"```",
				"FINDING file.go:123 severity: claim text",
				"NO FINDINGS",
				"Inventing a finding to appear useful is worse than finding nothing",
			},
		},
		{
			name: "multiple files sorted deterministically",
			writtenFiles: map[string]string{
				"z/last.go":  "package last\n",
				"a/first.go": "package first\n",
				"m/mid.go":   "package mid\n",
			},
			uncoveredSummary: "",
			wantContains: []string{
				"File: a/first.go",
				"File: m/mid.go",
				"File: z/last.go",
			},
		},
		{
			name: "content without trailing newline still fenced",
			writtenFiles: map[string]string{
				"foo.go": "package foo",
			},
			uncoveredSummary: "",
			wantContains: []string{
				"File: foo.go",
				"package foo",
				"```",
			},
		},
		{
			name: "uncovered summary included when non-empty",
			writtenFiles: map[string]string{
				"foo.go": "package foo\n",
			},
			uncoveredSummary: "uncovered: foo.go:10",
			wantContains: []string{
				"uncovered: foo.go:10",
				"Uncovered code summary",
			},
		},
		{
			name: "uncovered summary whitespace only not included",
			writtenFiles: map[string]string{
				"foo.go": "package foo\n",
			},
			uncoveredSummary: "   \n\t  ",
			wantNotContains: []string{
				"Uncovered code summary",
			},
		},
		{
			name: "prompt instructs severity values",
			writtenFiles: map[string]string{
				"foo.go": "package foo\n",
			},
			uncoveredSummary: "",
			wantContains: []string{
				"high, medium, or low",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildCriticPrompt(tc.writtenFiles, tc.uncoveredSummary)
			for _, want := range tc.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("buildCriticPrompt() missing %q\nprompt:\n%s", want, got)
				}
			}
			for _, notWant := range tc.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("buildCriticPrompt() should not contain %q\nprompt:\n%s", notWant, got)
				}
			}
			// Multiple files must appear in sorted order so the prompt is
			// deterministic and cacheable. Check ordering when applicable.
			if tc.name == "multiple files sorted deterministically" {
				idxFirst := strings.Index(got, "File: a/first.go")
				idxMid := strings.Index(got, "File: m/mid.go")
				idxLast := strings.Index(got, "File: z/last.go")
				if !(idxFirst < idxMid && idxMid < idxLast) {
					t.Errorf("files not in sorted order: first=%d mid=%d last=%d", idxFirst, idxMid, idxLast)
				}
			}
		})
	}
}

func TestParseCriticFindings(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     []CriticFinding
		wantNil  bool
	}{
		{
			name:     "empty response",
			response: "",
			wantNil:  true,
		},
		{
			name:     "whitespace only",
			response: "   \n\t\n  ",
			wantNil:  true,
		},
		{
			name:     "NO FINDINGS alone",
			response: "NO FINDINGS",
			wantNil:  true,
		},
		{
			name:     "NO FINDINGS with surrounding whitespace",
			response: "  NO FINDINGS  \n",
			wantNil:  true,
		},
		{
			name:     "NO FINDINGS among other lines still nil",
			response: "FINDING foo.go:10 high: real defect\nNO FINDINGS\nFINDING bar.go:20 low: something",
			wantNil:  true,
		},
		{
			name:     "NO FINDINGS embedded in chatter",
			response: "I reviewed the code.\nNO FINDINGS\nLooks good.",
			wantNil:  true,
		},
		{
			name:     "single high severity",
			response: "FINDING internal/foo/bar.go:42 high: nil dereference",
			want:     []CriticFinding{{File: "internal/foo/bar.go", Line: 42, Severity: "high", Claim: "nil dereference"}},
		},
		{
			name:     "single medium severity",
			response: "FINDING foo.go:10 medium: unchecked error",
			want:     []CriticFinding{{File: "foo.go", Line: 10, Severity: "medium", Claim: "unchecked error"}},
		},
		{
			name:     "single low severity",
			response: "FINDING foo.go:1 low: style nit",
			want:     []CriticFinding{{File: "foo.go", Line: 1, Severity: "low", Claim: "style nit"}},
		},
		{
			name:     "severity case insensitive high",
			response: "FINDING foo.go:5 HIGH: uppercase severity",
			want:     []CriticFinding{{File: "foo.go", Line: 5, Severity: "high", Claim: "uppercase severity"}},
		},
		{
			name:     "severity case insensitive mixed",
			response: "FINDING foo.go:5 Medium: mixed case\nFINDING bar.go:6 Low: low mixed\nFINDING baz.go:7 High: high mixed",
			want: []CriticFinding{
				{File: "foo.go", Line: 5, Severity: "medium", Claim: "mixed case"},
				{File: "bar.go", Line: 6, Severity: "low", Claim: "low mixed"},
				{File: "baz.go", Line: 7, Severity: "high", Claim: "high mixed"},
			},
		},
		{
			name:     "malformed lines ignored",
			response: "this is not a finding\nFINDING foo.go:10 high: valid\nnot FINDING bar.go:20 medium: also invalid\nalso bad",
			want:     []CriticFinding{{File: "foo.go", Line: 10, Severity: "high", Claim: "valid"}},
		},
		{
			name:     "invalid severity ignored",
			response: "FINDING foo.go:10 critical: unknown severity\nFINDING bar.go:20 high: valid one",
			want:     []CriticFinding{{File: "bar.go", Line: 20, Severity: "high", Claim: "valid one"}},
		},
		{
			name:     "missing line number ignored",
			response: "FINDING foo.go: high: missing line\nFINDING bar.go:20 high: valid",
			want:     []CriticFinding{{File: "bar.go", Line: 20, Severity: "high", Claim: "valid"}},
		},
		{
			name:     "missing claim colon ignored",
			response: "FINDING foo.go:10 high missing colon\nFINDING bar.go:20 high: valid",
			want:     []CriticFinding{{File: "bar.go", Line: 20, Severity: "high", Claim: "valid"}},
		},
		{
			name:     "malformed severity typo ignored",
			response: "FINDING foo.go:10 hihg: typo\n",
			wantNil:  true,
		},
		{
			name:     "mixed valid and malformed",
			response: "preamble text\nFINDING a.go:1 high: first\nbad line\nFINDING b.go:2 medium: second\nFINDING c.go: bad: no line\nFINDING d.go:3 low: third\ntrailing chatter",
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "high", Claim: "first"},
				{File: "b.go", Line: 2, Severity: "medium", Claim: "second"},
				{File: "d.go", Line: 3, Severity: "low", Claim: "third"},
			},
		},
		{
			name:     "claim with colons preserved",
			response: "FINDING foo.go:10 high: something: with colon: and more",
			want:     []CriticFinding{{File: "foo.go", Line: 10, Severity: "high", Claim: "something: with colon: and more"}},
		},
		{
			name:     "multiple findings all severities",
			response: "FINDING a.go:1 high: h1\nFINDING b.go:2 medium: m1\nFINDING c.go:3 low: l1\nFINDING d.go:4 HIGH: h2",
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "high", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "medium", Claim: "m1"},
				{File: "c.go", Line: 3, Severity: "low", Claim: "l1"},
				{File: "d.go", Line: 4, Severity: "high", Claim: "h2"},
			},
		},
		{
			name:     "no valid findings returns nil not empty slice",
			response: "hello world\nnothing here matches",
			wantNil:  true,
		},
		{
			name:     "severity trimmed and lowercased",
			response: "FINDING foo.go:10 HIGH: claim",
			want:     []CriticFinding{{File: "foo.go", Line: 10, Severity: "high", Claim: "claim"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCriticFindings(tc.response)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("parseCriticFindings(%q) = %v; want nil", tc.response, got)
				}
				return
			}
			if got == nil && tc.want != nil {
				t.Fatalf("parseCriticFindings(%q) = nil; want %v", tc.response, tc.want)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseCriticFindings(%q) len=%d want %d; got=%v want=%v", tc.response, len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseCriticFindings(%q)[%d] = %+v; want %+v", tc.response, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestFindingsWorthUplift(t *testing.T) {
	cases := []struct {
		name     string
		findings []CriticFinding
		want     []CriticFinding
		wantNil  bool
	}{
		{
			name:    "nil input",
			wantNil: true,
		},
		{
			name:     "empty slice",
			findings: []CriticFinding{},
			wantNil:  true,
		},
		{
			name: "only low returns nil",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "low", Claim: "style"},
				{File: "b.go", Line: 2, Severity: "low", Claim: "nit"},
			},
			wantNil: true,
		},
		{
			name: "only high",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "high", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "high", Claim: "h2"},
			},
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "high", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "high", Claim: "h2"},
			},
		},
		{
			name: "only medium",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "medium", Claim: "m1"},
			},
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "medium", Claim: "m1"},
			},
		},
		{
			name: "mixed severities filters low",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "high", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "low", Claim: "l1"},
				{File: "c.go", Line: 3, Severity: "medium", Claim: "m1"},
				{File: "d.go", Line: 4, Severity: "low", Claim: "l2"},
			},
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "high", Claim: "h1"},
				{File: "c.go", Line: 3, Severity: "medium", Claim: "m1"},
			},
		},
		{
			name: "case insensitive high medium",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "HIGH", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "Medium", Claim: "m1"},
				{File: "c.go", Line: 3, Severity: "LOW", Claim: "l1"},
				{File: "d.go", Line: 4, Severity: "Low", Claim: "l2"},
			},
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "HIGH", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "Medium", Claim: "m1"},
			},
		},
		{
			name: "unknown severity ignored",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "critical", Claim: "c1"},
				{File: "b.go", Line: 2, Severity: "high", Claim: "h1"},
			},
			want: []CriticFinding{
				{File: "b.go", Line: 2, Severity: "high", Claim: "h1"},
			},
		},
		{
			name: "preserves order and fields",
			findings: []CriticFinding{
				{File: "z.go", Line: 9, Severity: "medium", Claim: "m"},
				{File: "a.go", Line: 1, Severity: "high", Claim: "h"},
			},
			want: []CriticFinding{
				{File: "z.go", Line: 9, Severity: "medium", Claim: "m"},
				{File: "a.go", Line: 1, Severity: "high", Claim: "h"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findingsWorthUplift(tc.findings)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("findingsWorthUplift(%v) = %v; want nil", tc.findings, got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("findingsWorthUplift(%v) len=%d want %d; got=%v want=%v", tc.findings, len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("findingsWorthUplift()[%d] = %+v; want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestCriticFinding_Fields(t *testing.T) {
	// Pin the struct shape: the four fields the spec requires must exist
	// and round-trip through construction. A coverage gate checks the type
	// is exercised.
	f := CriticFinding{
		File:     "internal/foo/bar.go",
		Line:     42,
		Severity: "high",
		Claim:    "nil dereference",
	}
	if f.File != "internal/foo/bar.go" || f.Line != 42 || f.Severity != "high" || f.Claim != "nil dereference" {
		t.Fatalf("CriticFinding fields not stored correctly: %+v", f)
	}
}

func TestParseAndUpliftIntegration(t *testing.T) {
	// End-to-end: parse then filter — low findings are dropped for uplift.
	resp := "FINDING a.go:1 high: h1\nFINDING b.go:2 low: l1\nFINDING c.go:3 medium: m1"
	parsed := parseCriticFindings(resp)
	if len(parsed) != 3 {
		t.Fatalf("parseCriticFindings integration: got %d want 3: %v", len(parsed), parsed)
	}
	uplift := findingsWorthUplift(parsed)
	if len(uplift) != 2 {
		t.Fatalf("findingsWorthUplift integration: got %d want 2: %v", len(uplift), uplift)
	}
	for _, f := range uplift {
		if f.Severity != "high" && f.Severity != "medium" {
			t.Errorf("uplift should only contain high/medium, got %q", f.Severity)
		}
	}
}

func TestBuildCriticPrompt_RoundTripWithParser(t *testing.T) {
	// The prompt's documented output format must be parseable by the parser.
	// If the prompt says 'FINDING file.go:123 severity: claim text' and the
	// parser expects something else, the reviewer gate is wired to nothing.
	files := map[string]string{
		"foo.go": "package foo\n",
	}
	prompt := buildCriticPrompt(files, "")
	if !strings.Contains(prompt, "FINDING file.go:123 severity: claim text") {
		t.Fatal("prompt does not contain the documented finding format")
	}
	// Simulate a reviewer response using the exact format the prompt specifies.
	response := "FINDING foo.go:10 high: example defect"
	findings := parseCriticFindings(response)
	if len(findings) != 1 || findings[0].File != "foo.go" || findings[0].Line != 10 {
		t.Fatalf("round-trip failed: prompt format not parseable: %v", findings)
	}
}

// The coverage gate reported critic.go:153-154 as the only unexecuted block on
// the turn that created this file: the branch that rejects a FINDING line whose
// severity is not high/medium/low. `go test` was green at the same moment.
//
// The branch matters. Severity is what findingsWorthUplift triages on, so a
// reviewer that invents a severity ("critical", "warn", "P0") must not have its
// finding silently admitted with an unrecognised level and then be triaged as
// if it were low.
func TestParseCriticFindings_RejectsUnknownSeverity(t *testing.T) {
	cases := []struct {
		name     string
		response string
		want     int
	}{
		{"critical is not a severity", "FINDING a.go:1 critical: boom", 0},
		{"warn is not a severity", "FINDING a.go:1 warn: boom", 0},
		{"P0 is not a severity", "FINDING a.go:1 P0: boom", 0},
		{"empty-ish severity token does not match the format", "FINDING a.go:1 : boom", 0},
		{"valid severity still parses", "FINDING a.go:1 high: boom", 1},
		{
			"a bad severity does not poison the good lines beside it",
			"FINDING a.go:1 critical: boom\nFINDING b.go:2 medium: real one",
			1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCriticFindings(tc.response)
			if len(got) != tc.want {
				t.Fatalf("parseCriticFindings(%q) returned %d findings (%+v); want %d",
					tc.response, len(got), got, tc.want)
			}
		})
	}
}

// A line number that is not a number must be dropped, not defaulted to 0. A
// finding pointing at line 0 sends a reader to the top of the file.
func TestParseCriticFindings_RejectsNonNumericLine(t *testing.T) {
	if got := parseCriticFindings("FINDING a.go:notanumber high: boom"); len(got) != 0 {
		t.Errorf("expected a non-numeric line to be dropped, got %+v", got)
	}
}

// The critic gate must skip the turns it has no business reviewing. An LLM call
// on a markdown-only edit is a tax on every doc change.
func TestVerifyAndUpliftWithCritic_SkipsWhenNotApplicable(t *testing.T) {
	cases := []struct {
		name   string
		cfgOn  bool
		result *ExecutionResult
	}{
		{"disabled by config", false, &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"a.go"}}},
		{"nil result", true, nil},
		{"no successful writes", true, &ExecutionResult{SuccessfulWriteTools: 0, WrittenPaths: []string{"a.go"}}},
		{"wrote no Go", true, &ExecutionResult{SuccessfulWriteTools: 2, WrittenPaths: []string{"README.md"}}},
		{"only test files", true, &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"a_test.go"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultExecutorConfig()
			cfg.CriticReviewAfterEdits = tc.cfgOn
			e := &Executor{config: cfg}

			// A nil client would panic if the gate got as far as calling the
			// model, so reaching the end proves we short-circuited.
			resp, errs, err := e.verifyAndUpliftWithCritic(
				context.Background(), stubToolResults{}, "", nil, nil, nil, tc.result)
			if err != nil {
				t.Fatalf("gate should have skipped, got error: %v", err)
			}
			if resp != nil || errs != nil {
				t.Errorf("gate should have skipped, got resp=%v errs=%v", resp, errs)
			}
		})
	}
}

type stubToolResults struct{}

func (stubToolResults) CompleteWithToolResults(
	_ context.Context, _ string, _ []types.Message, _ []types.ToolDefinition,
) (*types.LLMToolResponse, error) {
	return nil, nil
}

// Test files are excluded from review on purpose: including them invites the
// critic to review the tests instead of the code, which is both lower value and
// the easiest place to generate plausible nitpicks.
func TestReadWrittenFilesForReview(t *testing.T) {
	ws := t.TempDir()
	mk := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, rel), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	mk("real.go", "package p\n\nfunc A() {}\n")
	mk("real_test.go", "package p\n")
	mk("notes.md", "# hi\n")
	mk("big.go", "package p\n"+strings.Repeat("// filler\n", 5000))

	got := readWrittenFilesForReview(ws, []string{
		"real.go", "real_test.go", "notes.md", "missing.go", "big.go", "  ",
	})

	if _, ok := got["real.go"]; !ok {
		t.Error("production Go file was not offered for review")
	}
	if _, ok := got["real_test.go"]; ok {
		t.Error("test file was offered for review; the critic should review code, not tests")
	}
	if _, ok := got["notes.md"]; ok {
		t.Error("non-Go file was offered for review")
	}
	if _, ok := got["missing.go"]; ok {
		t.Error("unreadable file was offered for review")
	}
	if body, ok := got["big.go"]; !ok {
		t.Error("large file was dropped entirely; it should be truncated instead")
	} else if len(body) > criticMaxFileBytes+64 {
		t.Errorf("large file was not truncated: %d bytes", len(body))
	}
}

// The uplift prompt must leave room to reject a finding. The reviewer is
// another fallible model, and forcing a "fix" for a mistaken finding makes the
// code worse than leaving it alone.
func TestFormatUpliftPrompt_AllowsRejectingAFinding(t *testing.T) {
	p := formatUpliftPrompt([]CriticFinding{
		{File: "a.go", Line: 12, Severity: "high", Claim: "nil deref on empty input"},
	})
	for _, want := range []string{"a.go", "12", "high", "nil deref on empty input"} {
		if !strings.Contains(p, want) {
			t.Errorf("uplift prompt drops %q, so the model cannot act on the finding", want)
		}
	}
	if !strings.Contains(p, "finding is wrong") {
		t.Error("uplift prompt does not let the model reject a mistaken finding")
	}
}

func TestCriticSeverityRank(t *testing.T) {
	cases := []struct {
		name string
		sev  string
		want int
	}{
		{"high", "high", 3},
		{"medium", "medium", 2},
		{"low", "low", 1},
		{"unknown", "unknown", 0},
		{"empty", "", 0},
		{"critical is not a severity", "critical", 0},
		{"high uppercase", "HIGH", 3},
		{"medium mixed case", "Medium", 2},
		{"low uppercase", "LOW", 1},
		{"high with spaces", "  high  ", 3},
		{"medium with spaces", " medium ", 2},
		{"low with newline", "\tlow\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CriticSeverityRank(tc.sev)
			if got != tc.want {
				t.Errorf("CriticSeverityRank(%q) = %d; want %d", tc.sev, got, tc.want)
			}
		})
	}
}

// The critic must not be able to hold a turn open.
//
// Observed live on this gate's first run: the review call started
// (prompt_len=11407) and had not returned twenty minutes later. The turn's work
// was done and both hard gates had passed; an advisory review was holding it.
// An advisory gate that cannot fail a turn but CAN hang one is worse than no
// gate, because the failure is invisible and unbounded.
func TestCriticTimeouts_AreBounded(t *testing.T) {
	if criticTimeout <= 0 || criticTimeout > 5*time.Minute {
		t.Errorf("criticTimeout = %v; an advisory review must be bounded and short", criticTimeout)
	}
	if criticUpliftTimeout <= 0 || criticUpliftTimeout > 10*time.Minute {
		t.Errorf("criticUpliftTimeout = %v; the uplift round must be bounded", criticUpliftTimeout)
	}
	// The review must be the shorter of the two: it produces an opinion, while
	// the uplift round makes real edits.
	if criticTimeout > criticUpliftTimeout {
		t.Errorf("criticTimeout (%v) exceeds criticUpliftTimeout (%v); the cheap advisory call should be bounded tighter than the one that edits",
			criticTimeout, criticUpliftTimeout)
	}
}

// A stalled reviewer must yield the turn, not block it. This drives the gate
// with a client that never returns and asserts it gives up.
func TestVerifyAndUpliftWithCritic_AbandonsAStalledReview(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = ws
	e := &Executor{config: cfg, llmClient: hangingLLM{}}

	// A context the caller cancels is the mechanism that must win. Without the
	// bound inside the gate, this call would never return.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		e.verifyAndUpliftWithCritic(ctx, stubToolResults{}, "", nil, nil, nil,
			&ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"a.go"}})
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("critic gate did not return after its context expired; it can hang a turn")
	}
}

// hangingLLM never answers, which is what the live stall looked like.
type hangingLLM struct{}

func (hangingLLM) Complete(ctx context.Context, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
func (h hangingLLM) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}
func (hangingLLM) CompleteWithStreaming(ctx context.Context, _, _ string, _ bool) (<-chan string, <-chan error) {
	return nil, nil
}
func (hangingLLM) CompleteWithTools(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// Evidence in the prompt is worthless if nothing tells the reviewer to use it.
// Without an instruction naming the uncovered and static-analysis sections, a
// reviewer reads them as background and reports on the source alone — wasting
// the one signal in the prompt produced by a tool rather than a model.
func TestBuildCriticPrompt_InstructsOnUncoveredEvidence(t *testing.T) {
	files := map[string]string{"a.go": "package p\n"}

	withEvidence := buildCriticPrompt(files, "a.go:10-12")
	if !strings.Contains(withEvidence, "a.go:10-12") {
		t.Error("uncovered summary is not embedded in the prompt")
	}
	if !strings.Contains(withEvidence, "tool output, not opinion") {
		t.Error("prompt does not tell the reviewer the evidence is tool output")
	}
	if !strings.Contains(withEvidence, "so a test gets written for it") {
		t.Error("prompt does not ask for uncovered logic to be reported, so coverage never improves")
	}
	// It must also permit declining: demanding a test for every uncovered block
	// would manufacture tests for unreachable defensive branches.
	if !strings.Contains(withEvidence, "not worth a test") {
		t.Error("prompt gives no way to skip a block that genuinely does not need a test")
	}

	// With no evidence, those instructions must not appear — an empty coverage
	// section followed by "account for the evidence above" is incoherent.
	without := buildCriticPrompt(files, "")
	if strings.Contains(without, "tool output, not opinion") {
		t.Error("evidence instructions appear when there is no evidence")
	}
}
