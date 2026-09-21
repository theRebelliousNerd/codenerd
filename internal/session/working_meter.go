package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	working "codenerd/internal/context"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// workingMeter measures the shape of a turn's executed tool trace so the
// working policy can decide what it means. It decides nothing itself: it
// counts rounds, durable writes, rounds since the last write and the last
// focused verification, and whether the tail of the trace repeats
// deterministically, and hands those numbers to working_set.mg as
// working_progress/5 and working_control/2.
//
// Until 2026-09-18 this type was a "tool budget controller": it also held an
// iteration ceiling and granted extra rounds when the trace looked productive.
// No count is a completion criterion, so the ceiling and the extension
// arithmetic are gone; the measurement they were built on is what the policy
// needed all along.
type workingMeter struct {
	// repeatThreshold is working_repeat_threshold, read from the policy by the
	// loop and handed here. Below 2 no cycle can be claimed.
	repeatThreshold int

	trace []workingObservation

	// Whole-turn counters reported to the working policy at each boundary.
	rounds            int
	writesTotal       int
	roundsSinceWrite  int
	roundsSinceVerify int

	// Structural queries that ran, and how many of them had no answer. The
	// policy reads both to decide when the raw search tools are offered.
	structuralAttempts int
	structuralMisses   int
}

type workingObservation struct {
	signature string
	writes    int
}

func newWorkingMeter(repeatThreshold int) *workingMeter {
	return &workingMeter{repeatThreshold: repeatThreshold}
}

func (c *workingMeter) observe(calls []types.ToolCall, results []types.ToolResult) {
	if c == nil {
		return
	}
	byID := make(map[string]types.ToolResult, len(results))
	for _, result := range results {
		byID[result.ToolUseID] = result
	}

	parts := make([]string, 0, len(calls))
	observation := workingObservation{}
	verifies := 0
	for _, call := range calls {
		result, paired := byID[call.ID]
		event := toolEventSignature(call, result, paired)
		parts = append(parts, event)
		if paired && tools.IsStructuralQuery(call.Name) {
			c.structuralAttempts++
			if result.IsError || tools.StructuralMissed(result.Content) {
				c.structuralMisses++
			}
		}
		if !paired || result.IsError {
			continue
		}
		if isWriteMutationTool(call.Name) {
			observation.writes++
		} else if isFocusedVerificationCall(call) {
			verifies++
		}
	}
	c.rounds++
	c.writesTotal += observation.writes
	if observation.writes > 0 {
		c.roundsSinceWrite = 0
	} else {
		c.roundsSinceWrite++
	}
	if verifies > 0 {
		c.roundsSinceVerify = 0
	} else {
		c.roundsSinceVerify++
	}
	observation.signature = digestStrings(parts)
	c.trace = append(c.trace, observation)
	const maxTraceHistory = 24
	if len(c.trace) > maxTraceHistory {
		c.trace = append([]workingObservation(nil), c.trace[len(c.trace)-maxTraceHistory:]...)
	}
}

func toolEventSignature(call types.ToolCall, result types.ToolResult, paired bool) string {
	args, err := json.Marshal(call.Input)
	if err != nil {
		args = []byte(fmt.Sprintf("%v", call.Input))
	}
	status := "missing"
	content := ""
	if paired {
		status = "ok"
		if result.IsError {
			status = "error"
		}
		content = result.Content
	}
	sum := sha256.Sum256([]byte(call.Name + "\x00" + string(args) + "\x00" + status + "\x00" + content))
	return hex.EncodeToString(sum[:])
}

func digestStrings(parts []string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// isFocusedVerificationCall recognizes bounded proof steps: the evidence that
// a durable write was checked. Generic shell calls qualify only when they
// contain one unchained, well-known verification command.
func isFocusedVerificationCall(call types.ToolCall) bool {
	switch strings.TrimSpace(call.Name) {
	case "run_tests", "test_single", "run_impacted_tests", "coverage", "run_build", "check_syntax", "lint":
		return true
	case "run_command", "exec_cmd", "bash":
		command := ""
		for _, key := range []string{"command", "cmd"} {
			if value, ok := call.Input[key].(string); ok {
				command = value
				break
			}
		}
		command = strings.ToLower(strings.TrimSpace(command))
		if command == "" || strings.ContainsAny(command, "\r\n;|") || strings.Contains(command, "&&") {
			return false
		}
		for _, prefix := range []string{
			"go test", "go vet", "go build", "git diff --check",
			"cargo test", "cargo check", "pytest", "python -m pytest",
			"npm test", "npm run test", "pnpm test", "yarn test",
		} {
			if command == prefix || strings.HasPrefix(command, prefix+" ") {
				return true
			}
		}
	}
	return false
}

// repeatedTailCycle detects deterministic period-1, period-2, and period-3
// cycles at the end of the trace. Comparing the full event signature means a
// legitimate re-read after a write is progress when its returned bytes change,
// while identical read/read or error/error churn is a loop. The span is
// working_repeat_threshold, handed in from the policy.
func (c *workingMeter) repeatedTailCycle() bool {
	if c == nil || c.repeatThreshold < 2 {
		return false
	}
	signatures := make([]string, len(c.trace))
	for i, observation := range c.trace {
		signatures[i] = observation.signature
	}
	for period := 1; period <= 3; period++ {
		need := period * c.repeatThreshold
		if len(signatures) < need {
			continue
		}
		start := len(signatures) - need
		matches := true
		for i := start + period; i < len(signatures); i++ {
			if signatures[i] != signatures[start+(i-start)%period] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// workingProgress is the loop's report to the working policy at a round
// boundary. It only counts; working_set.mg decides what the counts mean.
func (c *workingMeter) workingProgress(writeIntent bool, failedRounds int) working.WorkingProgress {
	if c == nil {
		return working.WorkingProgress{WriteIntent: writeIntent, FailedRounds: failedRounds}
	}
	return working.WorkingProgress{
		Cycle:        c.repeatedTailCycle(),
		FailedRounds: failedRounds,
		WriteIntent:  writeIntent,
		Rounds:       c.rounds,
		Writes:       c.writesTotal,
		SinceWrite:   c.roundsSinceWrite,
		SinceVerify:  c.roundsSinceVerify,

		StructuralAttempts: c.structuralAttempts,
		StructuralMisses:   c.structuralMisses,
	}
}

// workingNudgeText renders a policy-derived steering kind for the model. The
// policy decides when; the wording lives here so the .mg stays declarative.
// It reports what the turn has done, never what it has left: a count of
// remaining calls is not a fact about the task.
func workingNudgeText(kind string, p working.WorkingProgress) string {
	switch strings.TrimPrefix(strings.TrimSpace(kind), "/") {
	case "implement":
		return fmt.Sprintf("%d rounds of reading and no file written for a change task. Make the change now with the evidence in hand; read only what the edit itself needs.", p.Rounds)
	case "verify":
		return fmt.Sprintf("A file was written %d rounds ago and nothing has verified it. Run the verification the task names (or the tests for the touched package) now, then finish.", p.SinceWrite)
	case "conclude":
		return fmt.Sprintf("%d rounds of reading. Conclude from the gathered evidence, or name exactly what is missing and read only that.", p.Rounds)
	}
	return ""
}

// describeWorkingStop renders a derived working_stop for the user. A stop is
// never "you ran out of calls": it is a rule in working_set.mg that held over
// the facts this loop asserted, so the message names the rule and the facts
// that satisfied it. Reason arrives without its leading slash (see
// working.WorkingDecision).
func describeWorkingStop(reason string, p working.WorkingProgress, executed int) string {
	name := strings.TrimPrefix(strings.TrimSpace(reason), "/")
	if name == "" {
		// Continue returned !Continue with no reason: the policy has a stop
		// rule the loop cannot name, which is a policy bug, not a ceiling.
		return fmt.Sprintf(
			"task unresolved: the working policy withheld continuation without deriving a working_stop reason, after %d executed tool call(s)",
			executed)
	}
	intent := "/read"
	if p.WriteIntent {
		intent = "/write"
	}
	evidence := ""
	switch name {
	case "repeated_cycle":
		evidence = fmt.Sprintf(
			"the tail of the tool trace repeated deterministically for working_repeat_threshold cycles (working_control(/yes, %d))",
			p.FailedRounds)
	case "tool_failures":
		evidence = fmt.Sprintf(
			"%d consecutive round(s) in which every tool call failed (working_control(_, %d))",
			p.FailedRounds, p.FailedRounds)
	case "read_only_stall":
		evidence = fmt.Sprintf(
			"a change task read for %d round(s) and wrote nothing (working_progress(/write, %d, 0, _, _) with working_stall_rounds)",
			p.Rounds, p.Rounds)
	default:
		evidence = fmt.Sprintf("working_progress(%s, %d, %d, %d, %d)",
			intent, p.Rounds, p.Writes, p.SinceWrite, p.SinceVerify)
	}
	return fmt.Sprintf(
		"task unresolved: the working policy derived working_stop(/%s) — %s; %d tool call(s) executed",
		name, evidence, executed)
}

// workingRegimeText tells the model what the policy's regime means for the
// next round. Empty for the open regime.
func workingRegimeText(regime string) string {
	if strings.TrimPrefix(strings.TrimSpace(regime), "/") == commitRegime {
		return "Reading is closed for this task: the evidence gathered so far is what there is, and the tools offered now are the ones that make and verify the change (recall_context recovers what was already read). Make the change now, verify it, or conclude with what is missing."
	}
	return ""
}

// withRegimePrompt appends the regime text to prompt exactly once. Callers
// re-send the same failing prompt across rounds of one repair attempt, and the
// prompt may already carry the regime sentence from an earlier append (repair
// attempt N under the commit regime, or a round re-sent inside repairRound):
// a blind append repeats the sentence every time the message is re-sent, so
// the model receives it twice in a row. A prompt that already contains the
// regime text is returned unchanged.
func withRegimePrompt(prompt, regime string) string {
	text := workingRegimeText(regime)
	if strings.TrimSpace(text) == "" {
		return prompt
	}
	if strings.Contains(prompt, text) {
		return prompt
	}
	return prompt + "\n\n" + text
}

// appendWorkingNudge delivers a policy-derived nudge by appending it to the
// round's last tool result.
func appendWorkingNudge(results []types.ToolResult, nudge string) []types.ToolResult {
	if len(results) == 0 || strings.TrimSpace(nudge) == "" {
		return results
	}
	last := len(results) - 1
	results[last].Content = strings.TrimSpace(results[last].Content) + "\n\n[orchestrator] " + nudge
	return results
}
