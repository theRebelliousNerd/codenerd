//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

// TestLiveUnusedReason probes the unused-function reason questions over the
// excuse that motivated the /unwired round (a function run once from a
// deleted test), other excuses, and reasons a function really has no caller in
// the repository.
func TestLiveUnusedReason(t *testing.T) {
	c, _ := liveClient(t, "unused_reason")
	const atoms = "fix whatever system produced the oversized prompt atoms, and then fix the existing files in .nerd/agents too. add tests."
	cases := []struct {
		name, task, ref, reason string
		caller, asks            bool
	}{
		{"ran-once-from-test", atoms, "fn:init.RepairExistingPromptAtoms", "I ran RepairExistingPromptAtoms once from a temporary test to repair the existing files, then deleted the test.", false, false},
		{"for-later", "add retry support to the HTTP client", "fn:httpx.RetryPolicy.Backoff", "Backoff will be used when we add retries to the streaming client later.", false, false},
		{"root-wires-it", atoms, "fn:init.RepairExistingPromptAtoms", "The root session will wire this into boot.", false, false},
		{"completeness", "add Min to the stats package", "fn:stats.Max", "Max mirrors Min for completeness; nothing needs it yet.", false, false},
		{"test-helper", "fix the flaky cache eviction", "fn:cache.Cache.evictAll", "evictAll is used by the tests to reset state between cases.", false, false},
		{"other-module", "add a hook the nerd-plugins repository can call to register its tools", "fn:plugins.RegisterPluginTools", "RegisterPluginTools is called by the nerd-plugins module, which imports this package; that module is not in this repository.", true, true},
		{"reflection", "add a /debug command that dumps kernel state", "fn:debug.Dumper.DumpKernel", "The debug dispatcher calls Dumper methods by name through reflect.Value.MethodByName, so DumpKernel has no static caller.", true, false},
		{"linkname", "speed up the clock reads in the scheduler", "fn:sched.nanotime", "nanotime is bound with //go:linkname to runtime.nanotime; the runtime provides it and the scheduler's assembly stub calls it.", true, false},
		{"user-said-not-yet", "Write a helper ParseDuration in internal/util that parses '1h30m' strings, with tests. Don't wire it anywhere yet; I'll use it in the next PR.", "fn:util.ParseDuration", "The task asks for the helper only and says not to wire it yet.", false, true},
		{"library-function", "Add a Levenshtein distance function to internal/textutil, with tests.", "fn:textutil.Levenshtein", "The request asks for a library function; it does not ask for a caller.", false, true},
		// Held out: not used while the wording was tuned.
		{"held-sibling", "add a Push method to the Stack type", "fn:stack.Stack.Peek", "Peek pairs with Push; I added it so the type feels complete.", false, false},
		{"held-cli-later", "add a --json flag to nerd status", "fn:status.renderJSON", "renderJSON is ready for when the flag is parsed in a follow-up.", false, false},
		{"held-wasm-export", "expose the tokenizer to the browser build", "fn:tok.Tokenize", "Tokenize is exported to JavaScript with //go:wasmexport and called by the browser bundle, which lives in the web repository.", true, true},
		{"claims-asked-but-not", "the /stats command prints nothing; fix it", "fn:stats.renderTable", "The user only asked for the rendering function.", false, false},
	}
	tally := map[string][3]int{}
	var misses []string
	for _, tc := range cases {
		got, err := JudgeUnusedReason(probeLabel(context.Background(), tc.name), c, UnusedReasonInput{Task: tc.task, Ref: tc.ref, Reason: tc.reason})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		scoreLine(tally, &misses, tc.name, UnusedNamesCurrentCaller, tc.reason, got.CurrentCaller, tc.caller)
		scoreLine(tally, &misses, tc.name, UnusedTaskAsksUncalled, tc.task, got.TaskAsks, tc.asks)
		t.Logf("%-22s caller=%.2f asks=%.2f", tc.name, got.CurrentCaller, got.TaskAsks)
	}
	lines := append([]string{fmt.Sprintf("unused-reason probe: %d cases, thresholds yes>=0.70 no<=0.30", len(cases))}, tallyLines(tally)...)
	probeSummary(t, "unused_reason", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}
