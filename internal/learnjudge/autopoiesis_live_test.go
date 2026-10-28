//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"sort"
	"testing"
)

// labeled is a probe case: the judgments it is labeled with (true, false), the
// rest unlabeled and only reported.
type labeled struct {
	name   string
	text   string
	target string
	want   map[string]bool
}

// scoreLine sorts one judgment into right, wrong or unsettled at 0.70 / 0.30.
func scoreLine(tally map[string][3]int, misses *[]string, caseName, judgment, text string, p float64, want bool) {
	t := tally[judgment]
	switch {
	case p >= 0.7 == want && (p >= 0.7 || p <= 0.3):
		t[0]++
	case p > 0.3 && p < 0.7:
		t[2]++
		*misses = append(*misses, fmt.Sprintf("  ~ %-24s %-26s p=%.2f want=%v  %q", caseName, judgment, p, want, text))
	default:
		t[1]++
		*misses = append(*misses, fmt.Sprintf("  x %-24s %-26s p=%.2f want=%v  %q", caseName, judgment, p, want, text))
	}
	tally[judgment] = t
}

func tallyLines(tally map[string][3]int) []string {
	names := make([]string, 0, len(tally))
	for n := range tally {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []string
	var r, all int
	for _, n := range names {
		t := tally[n]
		out = append(out, fmt.Sprintf("%-26s right %3d  wrong %2d  unsettled %2d", n, t[0], t[1], t[2]))
		r += t[0]
		all += t[0] + t[1] + t[2]
	}
	return append(out, fmt.Sprintf("all right %d of %d", r, all))
}

var requestCases = []labeled{
	{"oauth-full", "add OAuth login: a token store, the login and callback endpoints, middleware on the API routes, tests and docs", "",
		map[string]bool{ScopeSeveralDeliverables: true, ScopeManyFiles: true, ScopeContinuesAfter: false}},
	{"migrate-staged", "migrate the session store from bbolt to SQLite: write the schema, copy the existing data, then switch the readers over and delete the old code", "",
		map[string]bool{ScopeSeveralDeliverables: true, ScopeOrderedStages: true, ScopeContinuesAfter: false}},
	{"fix-one-nil", "fix the nil pointer in internal/session/executor.go", "internal/session/executor.go",
		map[string]bool{ScopeSeveralDeliverables: false, ScopeOrderedStages: false, ScopeManyFiles: false, ScopeContinuesAfter: false}},
	{"rename-everywhere", "rename Executor to Runner across the whole codebase", "",
		map[string]bool{ScopeSeveralDeliverables: false, ScopeManyFiles: true, ScopeContinuesAfter: false}},
	{"epic-keyword", "implement a complete feature for billing", "", map[string]bool{ScopeContinuesAfter: false}},
	{"explain-two", "explain how the JIT compiler selects atoms and how the kernel ranks them", "",
		map[string]bool{ScopeSeveralDeliverables: false, ScopeManyFiles: false, ScopeContinuesAfter: false}},
	{"two-small", "add a test for Max with an empty slice and fix the typo in the README", "",
		map[string]bool{ScopeSeveralDeliverables: true, ScopeOrderedStages: false, ScopeManyFiles: false, ScopeContinuesAfter: false}},
	{"watch-tests", "watch internal/core for changes and tell me when a test starts failing", "internal/core",
		map[string]bool{ScopeContinuesAfter: true}},
	{"lint-every-commit", "run the linter on every commit", "", map[string]bool{ScopeContinuesAfter: true}},
	{"nightly-rebuild", "set up a nightly job that rebuilds the predicate corpus", "", map[string]bool{ScopeContinuesAfter: true}},
	{"monitor-question", "what does the monitor package do?", "monitor",
		map[string]bool{ScopeSeveralDeliverables: false, ScopeManyFiles: false, ScopeContinuesAfter: false}},
	{"noisy-alert-fix", "the alert when the build fails is too noisy, make it quieter", "",
		map[string]bool{ScopeSeveralDeliverables: false, ScopeContinuesAfter: false}},
	{"always-tabs", "from now on, always use tabs in Go files", "", map[string]bool{ScopeSeveralDeliverables: false}},
	{"keep-checking-flaky", "keep checking the flaky TestMangleWatcher tests in the background while I work", "",
		map[string]bool{ScopeContinuesAfter: true}},
}

// TestLiveAutopoiesis_Requests probes the four request judgments.
func TestLiveAutopoiesis_Requests(t *testing.T) {
	c, _ := liveClient(t, "autopoiesis_checks")
	tally := map[string][3]int{}
	var misses, rows []string
	for _, tc := range requestCases {
		got, err := JudgeRequestScope(probeLabel(context.Background(), "request-"+tc.name), c, tc.text, tc.target)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		rows = append(rows, fmt.Sprintf("  %-20s deliverables=%.2f stages=%.2f many_files=%.2f continues=%.2f  %q", tc.name,
			got[ScopeSeveralDeliverables], got[ScopeOrderedStages], got[ScopeManyFiles], got[ScopeContinuesAfter], tc.text))
		for judgment, want := range tc.want {
			scoreLine(tally, &misses, tc.name, judgment, tc.text, got[judgment], want)
		}
	}
	lines := append([]string{"request judgments, thresholds yes>=0.70 no<=0.30"}, tallyLines(tally)...)
	lines = append(lines, "", "every case:")
	lines = append(lines, rows...)
	lines = append(lines, "", "misses (x wrong, ~ unsettled):")
	probeSummary(t, "autopoiesis_checks-requests", append(lines, misses...))
}

var outputCases = []struct {
	name   string
	tool   string
	output string
	want   map[string]bool
}{
	{"grep-hits-with-error-word", "log_grep", "found 3 lines:\n12: ERROR failed to connect to db\n40: error budget ok\n77: retrying after timeout", map[string]bool{OutputReportsError: false, OutputReportsRateLimit: false}},
	{"connection-refused", "http_fetch", "error: dial tcp 127.0.0.1:443: connect: connection refused", map[string]bool{OutputReportsError: true, OutputReportsMorePages: false}},
	{"has-more", "issue_list", `{"items":[{"id":1},{"id":2}],"has_more":true,"next_cursor":"abc"}`, map[string]bool{OutputReportsMorePages: true, OutputReportsError: false}},
	{"last-page", "issue_list", `{"items":[{"id":9}],"has_more":false,"page":3,"total_pages":3}`, map[string]bool{OutputReportsMorePages: false, OutputReportsError: false}},
	{"rate-limited", "gh_search", "HTTP 429 Too Many Requests: secondary rate limit, retry after 60s", map[string]bool{OutputReportsRateLimit: true, OutputReportsError: true}},
	{"capped", "symbol_search", "Showing the first 100 matches; the result limit was reached.", map[string]bool{OutputReportsPartial: true, OutputReportsError: false}},
	{"complete-docs", "context7_docs", "## Usage\nCall Open(path) to open a store. Close releases it.", map[string]bool{OutputReportsPartial: false, OutputReportsError: false, OutputReportsRateLimit: false, OutputReportsMorePages: false}},
}

// TestLiveAutopoiesis_ToolOutput probes the four output judgments.
func TestLiveAutopoiesis_ToolOutput(t *testing.T) {
	c, _ := liveClient(t, "autopoiesis_checks")
	tally := map[string][3]int{}
	var misses []string
	for _, tc := range outputCases {
		got, err := JudgeToolOutput(probeLabel(context.Background(), "output-"+tc.name), c, tc.tool, "", tc.output)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for judgment, want := range tc.want {
			scoreLine(tally, &misses, tc.name, judgment, tc.output, got[judgment], want)
		}
	}
	lines := append([]string{"tool-output judgments, thresholds yes>=0.70 no<=0.30"}, tallyLines(tally)...)
	lines = append(lines, "", "misses (x wrong, ~ unsettled):")
	probeSummary(t, "autopoiesis_checks-output", append(lines, misses...))
}
