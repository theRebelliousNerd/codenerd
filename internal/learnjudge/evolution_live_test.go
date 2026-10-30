//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

const hollow = "Hollow success: the turn reported the work as done, but the kernel found no evidence for the claim (no successful write, test run, or build for an intent that requires one)."

type verdictCase struct {
	name string
	rec  TaskRecord
	// want: did_what_was_asked, stopped_early, failure_kind_fits (nil = unlabeled), kind ("" = unlabeled), and per changed file whether it was asked for.
	did, early, fits *bool
	kind             string
	files            []bool
}

func b(v bool) *bool { return &v }

var verdictCases = []verdictCase{
	{name: "hollow-test-claim", rec: TaskRecord{Request: "add a test for Max with an empty slice", Shard: "/coder", Output: "I added TestMaxEmpty to internal/stats/stats_test.go and it passes.", BuildErrors: []string{hollow}},
		fits: b(true)},
	{name: "provider-timeout", rec: TaskRecord{Request: "fix the nil pointer in internal/session/executor.go", Shard: "/coder", BuildErrors: []string{"LLM call failed: context deadline exceeded (per_call_timeout 120s)"}},
		did: b(false), fits: b(false)},
	{name: "rate-limit", rec: TaskRecord{Request: "refactor the retry policy into its own file", Shard: "/coder", BuildErrors: []string{"provider returned 429 Too Many Requests: rate limit exceeded, retry after 60s"}},
		did: b(false), fits: b(false)},
	{name: "syntax", rec: TaskRecord{Request: "make Max return 0 for an empty slice", Shard: "/coder", Output: "Edited internal/stats/stats.go.", BuildErrors: []string{"internal/stats/stats.go:14:2: syntax error: unexpected }, expected expression"}},
		did: b(false), fits: b(true), kind: "syntax_error"},
	{name: "wrong-function", rec: TaskRecord{Request: "make Max return 0 for an empty slice", Shard: "/coder", Output: "Changed Min in internal/stats/stats.go to return 0 for an empty slice.", TestsPassed: 4, TestsFailed: 1, BuildErrors: []string{"--- FAIL: TestMaxEmpty: Max([]) panicked: index out of range [0] with length 0"}},
		did: b(false), fits: b(true), kind: "instruction_miss"},
	{name: "edge-case", rec: TaskRecord{Request: "add a Median function to internal/stats", Shard: "/coder", Output: "Added Median to internal/stats/stats.go with tests.", TestsPassed: 6, TestsFailed: 1, BuildErrors: []string{"--- FAIL: TestMedianEmpty: Median([]) panicked: index out of range [0] with length 0"}},
		fits: b(true), kind: "edge_case"},
	{name: "clean-pass", rec: TaskRecord{Request: "fix the typo 'recieve' in README.md", Shard: "/coder", Succeeded: true, Output: "Fixed 'recieve' to 'receive' in README.md.", ChangedFiles: []string{"README.md"},
		Actions: []TaskAction{{Type: "read", Description: "read README.md", Target: "README.md"}, {Type: "edit", Description: "replace recieve with receive", Target: "README.md"}}},
		did: b(true), early: b(false), files: []bool{true}},
	{name: "stray-file", rec: TaskRecord{Request: "fix the typo 'recieve' in README.md", Shard: "/coder", Succeeded: true, Output: "Fixed the typo in README.md and tidied kernel.go while I was there.", ChangedFiles: []string{"README.md", "internal/core/kernel.go"},
		Actions: []TaskAction{{Type: "edit", Description: "replace recieve with receive", Target: "README.md"}, {Type: "edit", Description: "reformat and rename helpers", Target: "internal/core/kernel.go"}}},
		did: b(true), files: []bool{true, false}},
	{name: "stopped-early", rec: TaskRecord{Request: "move the session store from bbolt to SQLite and delete the bbolt code", Shard: "/coder", Succeeded: true, Output: "I wrote the SQLite schema in internal/store/schema.sql. Next I will copy the data and switch the readers over.", ChangedFiles: []string{"internal/store/schema.sql"}},
		did: b(false), early: b(true), files: []bool{true}},
	{name: "validator-rejected", rec: TaskRecord{Request: "fix all known bugs in internal/verification", Shard: "/coder", Output: "Wrote 2 file(s): internal/verification/verifier.go, internal/verification/verifier_test.go", BuildErrors: []string{"the turn ended /unverified: a file this turn changed was rejected by its post-action validator and not repaired"}},
		fits: b(true)},
}

func TestLiveTaskVerdict(t *testing.T) {
	c, _ := liveClient(t, "task_verdicts")
	tally := map[string][3]int{}
	var misses, rows []string
	for _, tc := range verdictCases {
		j, err := JudgeTask(probeLabel(context.Background(), tc.name), c, tc.rec)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		rows = append(rows, fmt.Sprintf("%-18s did=%.2f early=%.2f kind=%s(%.2f) fits=%.2f files=%v", tc.name, j.DidWhatWasAsked, j.StoppedEarly, j.Category, j.CategoryProbs[j.Category], j.CategoryFits, j.FileNeeded))
		if tc.did != nil {
			scoreLine(tally, &misses, tc.name, DidWhatWasAsked, tc.rec.Request, j.DidWhatWasAsked, *tc.did)
		}
		if tc.early != nil {
			scoreLine(tally, &misses, tc.name, StoppedEarly, tc.rec.Request, j.StoppedEarly, *tc.early)
		}
		if tc.fits != nil {
			scoreLine(tally, &misses, tc.name, FailureKindFits, tc.rec.Request, j.CategoryFits, *tc.fits)
		}
		if tc.kind != "" {
			k := tally[FailureKind]
			if j.Category == tc.kind {
				k[0]++
			} else {
				k[1]++
				misses = append(misses, fmt.Sprintf("  x %-24s %-26s got=%s want=%s", tc.name, FailureKind, j.Category, tc.kind))
			}
			tally[FailureKind] = k
		}
		for i, want := range tc.files {
			scoreLine(tally, &misses, tc.name, fmt.Sprintf("file_needed(%s)", tc.rec.ChangedFiles[i]), tc.rec.Request, j.FileNeeded[i], want)
		}
	}
	lines := append([]string{fmt.Sprintf("task verdict probe: %d records, thresholds yes>=0.70 no<=0.30", len(verdictCases))}, rows...)
	lines = append(lines, tallyLines(tally)...)
	probeSummary(t, "task_verdicts", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}

// The five task requests in .nerd/prompts/evolution.db on 2026-10-28, beside
// the regex classifier's stored answer, plus hard and negative cases.
var problemCases = []struct {
	name, task, agent, regex, want string
	fits                           bool
}{
	{"db-fix-all-bugs", "fix file:all bugs with constraint: fix all known bugs; find and fix additional bugs along the way", "/fix", "research", "debugging", true},
	{"db-explain-pool", "Read internal/embedding/long_input.go and explain in three sentences how poolVectors combines the halves.", "/explain", "documentation", "research", true},
	{"db-northstar-wizard", "fix create northstar wizard auto mode with constraint: agentic interview behavior; ability to explore codebase; ask questions; suggest things", "/fix", "feature_creation", "feature_creation", true},
	{"db-northstar-retry", "fix create northstar wizard auto mode with constraint: agentic interview behavior; ability to explore codebase; ask questions; suggest things\n\n## Previous Attempt Failed\nFile names and a green build claim alone do not prove agentic interview behavior, codebase exploration, question-asking, or suggestions.", "/fix", "code_review", "feature_creation", true},
	{"db-codebase-query", "codebase Knowledge Query: northstar system implementation storage CLI commands\n\nPurpose: ground setup steps in actual code paths", "/codebase", "config_setup", "research", true},
	{"race", "fix the data race between the cache writer and the reader goroutine", "/coder", "", "concurrency", true},
	{"bump-dep", "upgrade mangle-go to v0.7 and fix what breaks", "/coder", "", "dependency_mgmt", true},
	{"wrap-errors", "make every error from the store package wrap its cause with %w", "/coder", "", "error_handling", true},
	{"greeting", "hey, good morning!", "", "", "", false},
	{"weather", "what's the weather in Lisbon tomorrow?", "", "", "", false},
}

func TestLiveProblemType(t *testing.T) {
	c, _ := liveClient(t, "problem_types")
	right, wrong := 0, 0
	var lines []string
	for _, tc := range problemCases {
		j, err := JudgeProblemType(probeLabel(context.Background(), tc.name), c, tc.task, tc.agent)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		ok := (tc.fits && j.Fits >= 0.7 && (tc.want == "" || j.Kind == tc.want)) || (!tc.fits && j.Fits <= 0.3)
		mark := "ok"
		if ok {
			right++
		} else {
			wrong++
			mark = "MISS"
		}
		lines = append(lines, fmt.Sprintf("%-4s %-20s jev=%s(%.2f) fits=%.2f regex=%s want=%s", mark, tc.name, j.Kind, j.Probs[j.Kind], j.Fits, tc.regex, tc.want))
	}
	lines = append([]string{fmt.Sprintf("problem type probe: %d right, %d wrong of %d (fits threshold 0.70/0.30)", right, wrong, len(problemCases))}, lines...)
	probeSummary(t, "problem_types", lines)
}

var sharedCauseCases = []struct {
	name     string
	failures []GroupFailure
	want     bool
}{
	{"same-hollow-claim", []GroupFailure{
		{Request: "add a test for Max with an empty slice", Explanation: "Failed: the run failed; Jev reads the output as complete, so the reported work is not backed by evidence.", Category: "HALLUCINATION"},
		{Request: "add a test for the parser's empty input", Explanation: "Failed: the agent said it added TestParseEmpty but no file was written and no test ran.", Category: "HALLUCINATION"},
		{Request: "write a regression test for the retry bug", Explanation: "Failed: claimed the regression test passes; the kernel saw no test run and no write.", Category: "HALLUCINATION"},
	}, true},
	{"same-edge-case", []GroupFailure{
		{Request: "add a Median function", Explanation: "Failed: TestMedianEmpty panicked: index out of range on an empty slice.", Category: "EDGE_CASE"},
		{Request: "add a Mode function", Explanation: "Failed: Mode([]) panicked: index out of range [0] with length 0.", Category: "EDGE_CASE"},
		{Request: "add a Percentile function", Explanation: "Failed: Percentile(nil, 50) panicked on an empty input.", Category: "EDGE_CASE"},
	}, true},
	{"unrelated", []GroupFailure{
		{Request: "fix the nil pointer in executor.go", Explanation: "Failed: provider call timed out after 120s.", Category: "LOGIC_ERROR"},
		{Request: "rename Executor to Runner", Explanation: "Failed: the agent edited README.md, which the request did not call for.", Category: "INSTRUCTION_MISS"},
		{Request: "make Max return 0 for empty", Explanation: "Failed: stats.go:14: syntax error: unexpected }.", Category: "SYNTAX_ERROR"},
	}, false},
	{"provider-outages", []GroupFailure{
		{Request: "fix the footer wrap", Explanation: "Failed: provider returned 429 Too Many Requests.", Category: "LOGIC_ERROR"},
		{Request: "add a test for Max", Explanation: "Failed: provider returned 503 Service Unavailable.", Category: "LOGIC_ERROR"},
		{Request: "refactor the retry policy", Explanation: "Failed: LLM call failed: context deadline exceeded.", Category: "LOGIC_ERROR"},
	}, false},
}

func TestLiveSharedCause(t *testing.T) {
	c, _ := liveClient(t, "atom_checks")
	tally := map[string][3]int{}
	var misses, rows []string
	for _, tc := range sharedCauseCases {
		state, questions := SharedCauseRequest(tc.failures)
		judgment, err := JudgeSharedCause(probeLabel(context.Background(), "shared-"+tc.name), c, state, questions)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		p := judgment.Probability
		rows = append(rows, fmt.Sprintf("%-20s shares_one_cause=%.2f want=%v", tc.name, p, tc.want))
		scoreLine(tally, &misses, tc.name, SharesOneCause, tc.name, p, tc.want)
	}
	lines := append([]string{"shared-cause probe (evolution trigger)"}, rows...)
	lines = append(lines, tallyLines(tally)...)
	probeSummary(t, "atom_checks-shared_cause", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}
