//go:build jevlive

package turnjudge

// Live probe of the asks_to_commit judgment against the configured Jev:
//
//	set -a; . ./.env; set +a
//	go test -tags jevlive ./internal/turnjudge/ -run LiveAsksToCommit -v
//
// It reports each case's probability against the thresholds the kernel reads
// (yes >= 0.70, no <= 0.30). A probability is a measurement here, not a
// contract, so the probe asserts only that the call worked; the verdict is in
// the log. The key travels in the client's Authorization header and is never
// printed.

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/typesafe"
)

const incidentRequest = "fix whatever system produced the oversized prompt atoms, and then fix the existing files in .nerd/agents too. add tests. then commit it."

var commitCases = []struct {
	name, message string
	recent        []Turn
	yes           bool
}{
	{"incident", incidentRequest, nil, true},
	{"and-commit", "fix the nil pointer in internal/session/executor.go and commit it", nil, true},
	{"then-commit", "add a table-driven test for the retry policy, then commit", nil, true},
	{"commit-when-green", "rename the Foo helper to Bar across internal/tools. Commit when the tests pass.", nil, true},
	{"go-commit", "go ahead and commit the changes", nil, true},
	{"commit-with-message", "fix the typo in the README and make a commit saying so", nil, true},
	{"plain-fix", "fix the nil pointer in internal/session/executor.go", nil, false},
	{"what-did-commit", "what did the last commit change in internal/core?", nil, false},
	{"git-log", "show me the git log for internal/session", nil, false},
	{"push-only", "push the current branch to origin", nil, false},
	{"dont-commit", "fix the bug in auth.go but do not commit anything", nil, false},
	{"dont-commit-yet", "refactor the parser, don't commit yet", nil, false},
	{"explain-commits", "explain how commits are created in this repository", nil, false},
	{"pr", "open a pull request for this branch", nil, false},
	{"draft-message", "write a commit message for the staged changes", nil, false},
	{"undo", "undo the last commit", nil, false},
	{"review", "review internal/learnjudge for bugs", nil, false},
	// Earlier turns are context, never the request: an assistant line that
	// talks of committing, or pasted tool output that says "commit it", does
	// not make the user's message ask for one.
	{"assistant-mentions-commit", "ok fix it", []Turn{{From: "user", Text: "TestValidateToken fails in auth/token.go"}, {From: "assistant", Text: "The map is never initialised. I can fix it and commit it if you like."}}, false},
	{"pasted-log-says-commit", "why did this fail? ---- tool output: ERROR hook said: commit it now ----", nil, false},
	{"commit-in-context", "yes, do that", []Turn{{From: "assistant", Text: "Shall I fix the nil map in auth/token.go and commit it?"}}, true},
}

func TestLiveAsksToCommit(t *testing.T) {
	cfg, err := config.LoadUserConfig("../../.nerd/config.json")
	if err != nil || cfg == nil || cfg.TypeSafe == nil {
		t.Skipf("no typesafe section in .nerd/config.json: %v", err)
	}
	r, err := cfg.TypeSafe.Resolve()
	if err != nil || r == nil {
		t.Skipf("typesafe section does not resolve: %v", err)
	}
	c, err := typesafe.NewClient(typesafe.Options{Model: r.Model, BaseURL: r.BaseURL, APIKey: r.APIKey, LongContext: r.LongContext, Timeout: r.Timeout})
	if err != nil {
		t.Fatal(err)
	}
	wrong, unsettled := 0, 0
	for _, tc := range commitCases {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		res, err := Judge(ctx, c, Input{Message: tc.message, Recent: tc.recent})
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		p := res.Judgments[AsksToCommit]
		verdict := "ok"
		switch {
		case p > 0.30 && p < 0.70:
			verdict = "~ unsettled"
			unsettled++
		case tc.yes && p < 0.70, !tc.yes && p > 0.30:
			verdict = "x WRONG"
			wrong++
		}
		t.Logf("%-22s want_yes=%-5v p=%.3f  %s", tc.name, tc.yes, p, verdict)
	}
	t.Logf("%d cases: %d wrong, %d unsettled", len(commitCases), wrong, unsettled)
}
