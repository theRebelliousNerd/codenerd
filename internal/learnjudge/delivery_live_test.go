//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestLiveRequestDelivery probes the request-delivery reading over chat
// requests and step reports shaped like the delegated turns that ended "the
// work is not verified": a shard that did one part and said done, one that
// did all of it over two steps, one stopped by a denial, and read-only asks.
func TestLiveRequestDelivery(t *testing.T) {
	c, sec := liveClient(t, "request_delivery")
	yes, no := sec.Thresholds()
	cases := []struct {
		name    string
		request string
		reports []string
		changed []string
		missing []string // fragments that must read missing
		blocked bool
	}{
		{
			name:    "flag-no-docs",
			request: "add a --json flag to nerd status and document it in the README",
			reports: []string{"Done. Added a --json flag to the status command in cmd/nerd/cmd_status.go; it prints the status as JSON. Build and tests pass."},
			changed: []string{"cmd/nerd/cmd_status.go"},
			missing: []string{"document it in the README"},
		},
		{
			name:    "flag-and-docs-two-steps",
			request: "add a --json flag to nerd status and document it in the README",
			reports: []string{
				"Done. Added a --json flag to the status command in cmd/nerd/cmd_status.go; it prints the status as JSON. Build and tests pass.",
				"Documented the new --json flag of nerd status in README.md under Commands, with an example of its output.",
			},
			changed: []string{"cmd/nerd/cmd_status.go", "README.md"},
		},
		{
			name:    "review-no-fix",
			request: "review internal/cache for data races and fix any you find",
			reports: []string{"Review of internal/cache: Get reads c.items without holding c.mu (cache.go:41) while Set writes it under the lock (cache.go:58). That is a data race. Evict iterates the map unlocked (cache.go:77)."},
			missing: []string{"fix any you find"},
		},
		{
			name:    "fix-denied",
			request: "fix the flaky TestWatcher test and commit the fix",
			reports: []string{"Fixed TestWatcher: it waited on a fixed sleep; it now waits for the event channel with a timeout. go test -count=20 passes. The commit was blocked: git commit was denied by the shell-effect gate. Please commit it yourself or approve the commit."},
			changed: []string{"internal/watch/watcher_test.go"},
			missing: []string{"commit the fix"},
			blocked: true,
		},
		{
			name:    "explain-full",
			request: "explain why the kernel rebuilds the program on every retraction. Then list the files involved",
			reports: []string{"Retraction rebuilds from a fresh EDB because derived facts cannot be un-derived incrementally: internal/core/kernel_eval.go rebuildProgram re-parses schemas, policy and learned rules, then re-evaluates. Files involved: internal/core/kernel_eval.go, internal/core/kernel_retract.go, internal/core/kernel_init.go."},
		},
		{
			name:    "context-and-rule",
			request: "The chat drops the last line of long replies. Do not touch the renderer. Find where the reply is cut and fix it with a test",
			reports: []string{"The reply was cut in cmd/nerd/chat/view.go wrapReply, which dropped the remainder after the last newline. Fixed it to keep the tail and added TestWrapReplyKeepsLastLine. Tests pass."},
			changed: []string{"cmd/nerd/chat/view.go", "cmd/nerd/chat/view_test.go"},
		},
		{
			name:    "said-done-half",
			request: "rename Config.Timeout to Config.CallTimeout everywhere and update the docs that mention it",
			reports: []string{"Done: renamed Config.Timeout to Config.CallTimeout in internal/config and all 14 call sites. Build passes."},
			changed: []string{"internal/config/config.go", "internal/session/executor.go"},
			missing: []string{"update the docs that mention it"},
		},
	}
	var lines []string
	wrong := 0
	for _, tc := range cases {
		frags := RequestFragments(tc.request)
		got, err := CheckDelivery(probeLabel(context.Background(), tc.name), c, DeliveryInput{Request: tc.request, Reports: tc.reports, Changed: tc.changed, Fragments: frags})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		missing, unsettled := CoverageGaps(got.Verdicts, got.ResultCut, float64(yes)/100, float64(no)/100)
		lines = append(lines, fmt.Sprintf("%s: blocked=%.2f missing=%q unsettled=%q", tc.name, got.Blocked, missing, unsettled))
		for _, v := range got.Verdicts {
			lines = append(lines, fmt.Sprintf("    asked=%.2f delivered=%.2f  %q", v.Asked, v.Delivered, v.Fragment))
		}
		if strings.Join(missing, "|") != strings.Join(tc.missing, "|") {
			wrong++
			lines = append(lines, fmt.Sprintf("    x missing: want %q", tc.missing))
		}
		if b := got.Blocked >= float64(yes)/100; b != tc.blocked {
			wrong++
			lines = append(lines, fmt.Sprintf("    x blocked: want %v", tc.blocked))
		}
	}
	header := fmt.Sprintf("request-delivery probe: %d cases, %d wrong readings, thresholds yes>=%d no<=%d", len(cases), wrong, yes, no)
	probeSummary(t, "request_delivery", append([]string{header}, lines...))
}
