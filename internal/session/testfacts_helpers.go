package session

import (
	"bytes"
	"sort"
	"strings"

	"codenerd/internal/testfacts"
)

// Structured test verdicts for the gates.
//
// The gates used to run plain `go test` and scan its combined text: "--- FAIL:
// TestX" for names, "FAIL <pkg> [build failed]" for compile failures. One
// run's blob repeated a single log line 101,144 times (46.5 MB) and blew a
// repair request past its token budget (measured 2026-09-26; see
// internal/testfacts/doc.go). The gates now run `go test -json` and read the
// verdicts from the parsed Result: failing names, build failures, and the
// Summary shown to models and logs. Names are not read back out of that
// Summary. The one reader of published text is summaryShowsBuildFailure,
// because the repair prompt is handed a string (see its comment).

// parseTestJSON parses one `go test -json` stdout. dir is the directory the
// go command ran in: Parse canonicalises failure files against it, so a
// basename, a ./ diagnostic, and an absolute frame for one file become one
// workspace-relative path. Parse's only error is a read failure, which a
// byte slice cannot produce; the result is never nil, so gates use it
// without a fallback path.
func parseTestJSON(dir string, out []byte) *testfacts.Result {
	res, _ := testfacts.Parse(dir, bytes.NewReader(out))
	if res == nil {
		return &testfacts.Result{Status: testfacts.StatusUnknown}
	}
	return res
}

// verificationOutput renders what a gate shows for a run: the Result's
// Summary, plus any raw lines the stream carried that had no JSON structure
// (a timeout's trailing fragment, a runner that died before test2json
// started). Raw lines are testfacts' own fallback record, not the raw
// stream: a complete -json run has none, and nothing is cut to fit.
func verificationOutput(res *testfacts.Result) string {
	if res == nil {
		return ""
	}
	out := res.Summary()
	if len(res.Raw) > 0 {
		out += strings.Join(res.Raw, "\n") + "\n"
	}
	return strings.TrimSpace(out)
}

// failedTestNames names every failed test, deduplicated and sorted, in the
// sanitized spelling testfacts reports. A subtest keeps its suffix:
// t.Run("case one") arrives as "TestX/case_one", never "case one". Baseline
// pre-existing checks and pin failure reports compare on these full names,
// so a new subtest is not hidden behind a parent that already failed.
func failedTestNames(res *testfacts.Result) []string {
	if res == nil {
		return nil
	}
	set := make(map[string]bool)
	for _, p := range res.Packages {
		for _, t := range p.Tests {
			if t.Status != testfacts.StatusFail || t.Name == "" {
				continue
			}
			set[t.Name] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// failedTopLevels names the failed tests' top-level parents, deduplicated
// and sorted. -run is matched against the parent (Go splits the regexp on
// '/'), and turnTests records function names, so those comparisons use the
// parent of a sanitized subtest name.
func failedTopLevels(res *testfacts.Result) []string {
	set := make(map[string]bool)
	for _, n := range failedTestNames(res) {
		set[topLevelTestName(n)] = true
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// topLevelTestName cuts a subtest suffix: "TestX/case_one" is "TestX".
func topLevelTestName(name string) string {
	if i := strings.Index(name, "/"); i >= 0 {
		return name[:i]
	}
	return name
}

// summaryShowsBuildFailure reports whether a published Summary carries
// compiler diagnostics: the "build-failed <pkg> ..." lines Summary renders
// first.
//
// testRepairPrompt is the caller, and it only has text. repairSpec.promptFor
// (repair_loop.go) threads the seed string, not the Result: a `go test`
// compile failure's seed is this Summary, and a later `go build` recheck's
// seed is the compiler's own text, which has no Result. Gates that hold the
// Result use testBuildFailed.
func summaryShowsBuildFailure(summary string) bool {
	for _, line := range strings.Split(summary, "\n") {
		if strings.HasPrefix(line, "build-failed ") {
			return true
		}
	}
	return false
}
