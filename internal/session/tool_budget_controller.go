package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

// toolBudgetController turns the fixed iteration ceiling into a bounded,
// observable control loop. It never authorizes tools and never weakens the
// call/time/safety limits. Its only authority is to grant a small number of
// extra LLM -> tool rounds when the executed trace proves forward progress.
type toolBudgetController struct {
	baseLimit       int
	iterationLimit  int
	hardLimit       int
	maxToolCalls    int
	extensionSize   int
	maxExtensions   int
	extensions      int
	repeatThreshold int
	adaptive        bool

	trace                  []toolBudgetObservation
	seenEvents             map[string]struct{}
	progressSinceExtension int
	writesSinceExtension   int
	verifiesSinceExtension int
	hasSuccessfulWrite     bool
}

type toolBudgetObservation struct {
	signature string
	successes int
	errors    int
	novel     int
	writes    int
}
type toolBudgetExtensionDecision struct {
	Granted       bool
	AddedRounds   int
	NewLimit      int
	LoopDetected  bool
	ReadOnlyStall bool
	Reason        string
}

func newToolBudgetController(cfg ExecutorConfig) *toolBudgetController {
	base := cfg.MaxToolIterations
	if base <= 0 {
		base = defaultMaxToolIterations
	}
	maxCalls := effectiveMaxToolCalls(cfg.MaxToolCalls)
	extensionSize := cfg.ToolIterationExtensionSize
	if extensionSize <= 0 {
		extensionSize = defaultToolIterationExtensionSize
	}
	maxExtensions := cfg.MaxToolIterationExtensions
	if maxExtensions <= 0 {
		maxExtensions = defaultMaxToolIterationExtensions
	}
	repeatThreshold := cfg.ToolLoopRepeatThreshold
	if repeatThreshold < 2 {
		repeatThreshold = defaultToolLoopRepeatThreshold
	}

	hardLimit := base
	if cfg.AdaptiveToolBudget {
		hardLimit += extensionSize * maxExtensions
	}
	// Every iteration enters with at least one pending tool call. A round limit
	// above the call limit cannot execute useful work and only buys empty model
	// turns, so the absolute call budget is also the adaptive hard ceiling.
	if hardLimit > maxCalls {
		hardLimit = maxCalls
	}
	if hardLimit < base {
		hardLimit = base
	}

	return &toolBudgetController{
		baseLimit:       base,
		iterationLimit:  base,
		hardLimit:       hardLimit,
		maxToolCalls:    maxCalls,
		extensionSize:   extensionSize,
		maxExtensions:   maxExtensions,
		repeatThreshold: repeatThreshold,
		adaptive:        cfg.AdaptiveToolBudget,
		seenEvents:      make(map[string]struct{}),
	}
}

func (c *toolBudgetController) observe(calls []types.ToolCall, results []types.ToolResult) {
	if c == nil {
		return
	}
	byID := make(map[string]types.ToolResult, len(results))
	for _, result := range results {
		byID[result.ToolUseID] = result
	}

	parts := make([]string, 0, len(calls))
	observation := toolBudgetObservation{}
	for _, call := range calls {
		result, paired := byID[call.ID]
		event := toolBudgetEventSignature(call, result, paired)
		parts = append(parts, event)
		if !paired || result.IsError {
			observation.errors++
			continue
		}
		observation.successes++
		if isWriteMutationTool(call.Name) {
			observation.writes++
			c.writesSinceExtension++
			c.hasSuccessfulWrite = true
		} else if c.hasSuccessfulWrite && isFocusedVerificationCall(call) {
			c.verifiesSinceExtension++
		}
		if _, seen := c.seenEvents[event]; !seen {
			c.seenEvents[event] = struct{}{}
			observation.novel++
			c.progressSinceExtension++
		}
	}
	observation.signature = digestStrings(parts)
	c.trace = append(c.trace, observation)
	const maxTraceHistory = 24
	if len(c.trace) > maxTraceHistory {
		c.trace = append([]toolBudgetObservation(nil), c.trace[len(c.trace)-maxTraceHistory:]...)
	}
}

func toolBudgetEventSignature(call types.ToolCall, result types.ToolResult, paired bool) string {
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

// maybeExtend is called only when the model returns another tool batch at the
// current iteration boundary. The grant is mechanical: novel successful tool
// evidence since the previous boundary, no repeated tail cycle, and remaining
// configured extension capacity. For write-oriented intents, a successful
// durable write or focused post-write verification is required; novel
// read/search evidence alone is a deterministic stall.
func (c *toolBudgetController) maybeExtend(writeOriented bool) toolBudgetExtensionDecision {
	decision := toolBudgetExtensionDecision{NewLimit: c.iterationLimit}
	if c == nil || !c.adaptive {
		decision.Reason = "adaptive tool budget disabled"
		return decision
	}
	if c.extensions >= c.maxExtensions || c.iterationLimit >= c.hardLimit {
		decision.Reason = "adaptive hard limit reached"
		return decision
	}
	if c.repeatedTailCycle() {
		decision.LoopDetected = true
		decision.Reason = "deterministic repeated tool trace detected"
		return decision
	}
	if c.progressSinceExtension == 0 {
		decision.Reason = "no novel successful tool result since the previous boundary"
		return decision
	}
	if writeOriented && c.writesSinceExtension == 0 && c.verifiesSinceExtension == 0 {
		decision.ReadOnlyStall = true
		decision.Reason = "write-oriented intent stalled: no durable write or post-write verification since the previous boundary"
		return decision
	}

	added := c.extensionSize
	if remaining := c.hardLimit - c.iterationLimit; added > remaining {
		added = remaining
	}
	if added <= 0 {
		decision.Reason = "no bounded extension capacity remains"
		return decision
	}
	c.iterationLimit += added
	c.extensions++
	c.progressSinceExtension = 0
	c.writesSinceExtension = 0
	c.verifiesSinceExtension = 0
	decision.Granted = true
	decision.AddedRounds = added
	decision.NewLimit = c.iterationLimit
	decision.Reason = "novel successful tool results without a repeated trace cycle"
	return decision
}

// isFocusedVerificationCall recognizes bounded proof steps that can earn a
// follow-up extension after a durable write. Generic shell calls qualify only
// when they contain one unchained, well-known verification command.
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
// while identical read/read or error/error churn is a loop.
func (c *toolBudgetController) repeatedTailCycle() bool {
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

func (c *toolBudgetController) nudge(iterationsCompleted, callsUsed int, writeOriented bool, batchCodeDOM bool) string {
	if c == nil {
		return ""
	}
	roundsLeft := max(c.iterationLimit-iterationsCompleted, 0)
	callsLeft := max(c.maxToolCalls-callsUsed, 0)
	prefix := fmt.Sprintf("Orchestrator budget: %d tool calls and %d rounds remain.", callsLeft, roundsLeft)

	critical := roundsLeft <= 2 || callsLeft <= 4
	tight := roundsLeft <= max(c.baseLimit/4, 3) || callsLeft <= max(c.maxToolCalls/4, 6)
	switch {
	case critical && writeOriented && batchCodeDOM:
		return prefix + " Finish now: one multi-file CodeDOM transaction, then one focused verification; do not explore."
	case critical && writeOriented:
		return prefix + " Finish now: batch independent edits in this response, then one focused verification; do not explore."
	case critical:
		return prefix + " Conclude now from gathered evidence; do not open a new exploration branch."
	case tight && writeOriented && batchCodeDOM:
		return prefix + " Converge: batch remaining reads, then one multi-file CodeDOM transaction and focused verification."
	case tight && writeOriented:
		return prefix + " Converge: batch remaining reads and independent edits; implement before more exploration."
	case tight:
		return prefix + " Converge: batch remaining searches and stop guessing paths."
	default:
		return prefix + " Batch independent reads/searches in one response; discover paths before reading them."
	}
}

func appendToolBudgetNudge(results []types.ToolResult, nudge string) []types.ToolResult {
	if len(results) == 0 || strings.TrimSpace(nudge) == "" {
		return results
	}
	last := len(results) - 1
	results[last].Content = strings.TrimSpace(results[last].Content) + "\n\n[orchestrator] " + nudge
	return results
}

func hasToolDefinition(defs []types.ToolDefinition, name string) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}
