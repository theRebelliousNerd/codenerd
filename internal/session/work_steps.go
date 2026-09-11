package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// A change task with several edit sites is run as planned steps.
//
// Observed 2026-09-11, across every multi-site brief of the day: the model
// makes one edit, announces the next one every round, and reads or recalls
// instead of making it until the working policy stops the turn. A brief that
// names one file and one line lands in a handful of tool calls. So the
// executive divides the task once, up front, and runs each step as its own
// narrow pass with the file named, the way a repair round already runs: the
// model plans, the harness sequences. A step that makes no edit gets one more
// pass with reading closed; a step that still makes none is reported, and
// the turn fails rather than claiming the task.

// ErrStepsIncomplete marks a planned task some of whose steps made no edit.
// Wrapped so errors.Is can tell it from a provider failure.
var ErrStepsIncomplete = errors.New("planned steps incomplete")

// maxPlannedSteps bounds a plan. A task that divides into more than this is
// not one turn's work; the executive runs the first steps and reports.
const maxPlannedSteps = 12

// planStepsTimeout bounds the planning call. Planning is one short answer to
// one short prompt; a plan that has not come back in this time is abandoned
// and the task runs as a single pass.
const planStepsTimeout = 2 * time.Minute

// workStepPlanSystem is the planning prompt. It asks for edit sites, not for
// an approach: the executive needs to know where the model will write, not
// how it will think.
const workStepPlanSystem = `You divide one code-change task into the edit steps an executive will run one at a time, each as its own turn with the file named.

Output one line per step, in the order the task gives them, in exactly this form:
STEP <workspace-relative file path> :: <the change to make in that file, with its location>

Rules: one step per file region the task says to change; keep the task's own numbering, names, line numbers and wording; a test the task asks for is its own step; never add a file the task does not name or clearly imply; never add a step the task does not ask for. A task with one change is one STEP line. Output only STEP lines, nothing else.`

// workStep is one edit site of a planned task and what became of it.
type workStep struct {
	File   string
	Change string
	Edited bool
	Calls  int
	Note   string // the model's closing sentence, or the pass error
}

// toolLoopPass is how one pass of the tool loop is run. verify runs the
// post-edit gate (build, tests, review) at the pass's terminal path; a
// planned task verifies once after its last step instead. regime, when set,
// starts the pass's working loop under that regime.
type toolLoopPass struct {
	verify bool
	regime string
}

// parseWorkSteps reads STEP lines out of a plan. Anything else on a line, a
// bullet or a number in front of STEP, is tolerated; a line without both a
// path and a change is not a step. Duplicates collapse; the count is bounded.
func parseWorkSteps(text string) []workStep {
	var steps []workStep
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, "STEP ")
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len("STEP "):])
		file, change, ok := strings.Cut(rest, "::")
		if !ok {
			continue
		}
		file = strings.Trim(strings.TrimSpace(file), "`'\"")
		change = strings.TrimSpace(change)
		if file == "" || change == "" {
			continue
		}
		key := file + "\x00" + change
		if seen[key] {
			continue
		}
		seen[key] = true
		steps = append(steps, workStep{File: file, Change: change})
		if len(steps) == maxPlannedSteps {
			break
		}
	}
	return steps
}

// planTurnSteps decides whether this turn is a planned task and, if so, what
// its steps are. Only a write-oriented turn on the native tool path inside a
// working loop is planned; every other turn keeps the single pass. A plan
// with fewer than two steps is a single pass too: the planning call then
// cost one short answer and changed nothing.
func (e *Executor) planTurnSteps(ctx context.Context, client types.LLMClient, task string, cfg *config.EffectiveAgentRuntimeConfig, result *ExecutionResult) []workStep {
	e.mu.RLock()
	hasWorld := e.workingWorld != nil
	e.mu.RUnlock()
	if !hasWorld || !e.configSnapshot().ProgressDrivenTools || result == nil {
		return nil
	}
	if !e.writeOrientedIntent(result.Intent.Verb) {
		return nil
	}
	if _, native := client.(types.ToolResultsProvider); !native {
		return nil
	}
	if ptp, ok := client.(types.PiggybackToolProvider); ok && ptp.ShouldUsePiggybackTools() {
		return nil
	}
	if cfg == nil || !hasWriteTool(cfg.AllowedTools) {
		return nil
	}
	planCtx, cancel := context.WithTimeout(ctx, planStepsTimeout)
	defer cancel()
	text, err := client.CompleteWithSystem(planCtx, workStepPlanSystem, task)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("Step planning failed (%v); the task runs as one pass", err)
		return nil
	}
	steps := parseWorkSteps(text)
	if len(steps) < 2 {
		logging.SessionDebug("Step planning found %d step(s); the task runs as one pass", len(steps))
		return nil
	}
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.File)
	}
	logging.Session("Executive runs the task as %d planned step(s): %s", len(steps), strings.Join(names, ", "))
	return steps
}

func hasWriteTool(names []string) bool {
	for _, name := range names {
		if isWriteMutationTool(name) {
			return true
		}
	}
	return false
}

// workStepAnchor is the user turn one step's pass runs against: the whole
// task, so nothing the brief said is lost, then the executive's note of
// where this pass sits in the plan and what it must do. retry is the second
// pass of a step whose first made no edit.
func workStepAnchor(task string, steps []workStep, current int, retry bool) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(task))
	b.WriteString("\n\n[executive] This task runs as ")
	fmt.Fprintf(&b, "%d steps, one at a time; this turn is step %d of %d.\n", len(steps), current+1, len(steps))
	for i := 0; i < current; i++ {
		status := "made no edit"
		if steps[i].Edited {
			status = "edited"
		}
		fmt.Fprintf(&b, "Done: [%d] %s :: %s (%s)\n", i+1, steps[i].File, steps[i].Change, status)
	}
	fmt.Fprintf(&b, "Step %d of %d: %s :: %s\n", current+1, len(steps), steps[current].File, steps[current].Change)
	if retry {
		b.WriteString("This step's first pass made no edit. Reading is closed: the observations already gathered are in the working section, and recall_context recovers any of them whole. Make this step's edit now with the edit tools, or say in one sentence what is missing.")
	} else {
		b.WriteString("Make exactly this step's change now with the edit tools; the remaining steps run after it, so do not make them here. When the edit is made, reply with one sentence saying what changed.")
	}
	return b.String()
}

// workStepReport is the ledger the turn surfaces: every step, whether it
// edited, and the model's closing word on it.
func workStepReport(steps []workStep) string {
	var b strings.Builder
	edited := 0
	for _, s := range steps {
		if s.Edited {
			edited++
		}
	}
	fmt.Fprintf(&b, "Planned steps: %d, edited: %d.", len(steps), edited)
	for i, s := range steps {
		status := "no edit"
		if s.Edited {
			status = "edited"
		}
		fmt.Fprintf(&b, "\n[%d] %s :: %s — %s (%d tool call(s))", i+1, s.File, s.Change, status, s.Calls)
		if note := strings.TrimSpace(s.Note); note != "" {
			fmt.Fprintf(&b, ": %s", firstLine(note))
		}
	}
	return b.String()
}

// runPlannedSteps runs each step as its own pass of the tool loop, gives a
// step that made no edit one more pass with reading closed, verifies the
// whole once, and reports. A step's own failure (a policy stop, a provider
// error) ends that step, not the task; a cancelled context ends the task.
func (e *Executor) runPlannedSteps(
	ctx context.Context,
	systemPrompt, task string,
	steps []workStep,
	cfg *config.EffectiveAgentRuntimeConfig,
	compilationCtx *prompt.CompilationContext,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	var toolErrs []string
	var last *types.LLMToolResponse
	for i := range steps {
		if ctx.Err() != nil {
			result.StepReport = workStepReport(steps)
			return last, toolErrs, ctx.Err()
		}
		step := &steps[i]
		stepCtx := *compilationCtx
		stepCtx.IntentTarget = step.File
		writesBefore, callsBefore := result.SuccessfulWriteTools, result.ToolCallsExecuted

		resp, errs, err := e.runToolLoopPass(ctx, systemPrompt, workStepAnchor(task, steps, i, false), cfg, &stepCtx, result, toolLoopPass{})
		toolErrs = append(toolErrs, errs...)
		if err != nil {
			if ctx.Err() != nil {
				result.StepReport = workStepReport(steps)
				return last, toolErrs, err
			}
			step.Note = err.Error()
			logging.Get(logging.CategorySession).Warn("Step %d/%d (%s) ended in error: %v", i+1, len(steps), step.File, err)
		}
		if result.SuccessfulWriteTools == writesBefore {
			logging.Get(logging.CategorySession).Warn(
				"Step %d/%d (%s) made no edit in %d tool call(s); one more pass with reading closed",
				i+1, len(steps), step.File, result.ToolCallsExecuted-callsBefore)
			retried, retryErrs, retryErr := e.runToolLoopPass(ctx, systemPrompt, workStepAnchor(task, steps, i, true), cfg, &stepCtx, result, toolLoopPass{regime: commitRegime})
			toolErrs = append(toolErrs, retryErrs...)
			if retryErr != nil {
				if ctx.Err() != nil {
					result.StepReport = workStepReport(steps)
					return last, toolErrs, retryErr
				}
				step.Note = retryErr.Error()
				logging.Get(logging.CategorySession).Warn("Step %d/%d (%s) retry ended in error: %v", i+1, len(steps), step.File, retryErr)
			}
			if retried != nil {
				resp = retried
			}
		}
		step.Edited = result.SuccessfulWriteTools > writesBefore
		step.Calls = result.ToolCallsExecuted - callsBefore
		if resp != nil {
			if text := strings.TrimSpace(resp.Text); text != "" {
				step.Note = text
			}
			last = resp
		}
		logging.Session("Step %d/%d %s: edited=%v, %d tool call(s)", i+1, len(steps), step.File, step.Edited, step.Calls)
	}
	result.StepReport = workStepReport(steps)
	if last == nil {
		last = &types.LLMToolResponse{Text: result.StepReport}
	}

	// One gate for the whole task, inside a working loop so a repair round
	// sees what the steps gathered.
	client := e.llmForVerb(result.Intent.Verb)
	trp, _ := client.(types.ToolResultsProvider)
	gateCtx, closeWorking, workingErr := e.beginWorkingLoop(ctx, task, compilationCtx)
	if workingErr != nil {
		return last, toolErrs, workingErr
	}
	defer closeWorking()
	history := append(e.priorTurnMessages(),
		types.Message{Role: "user", Text: task},
		types.Message{Role: "assistant", Text: last.Text})
	verified, verifyErrs, verifyErr := e.verifyCompletedToolTurn(
		gateCtx, trp, systemPrompt, history, last, e.buildToolDefinitions(cfg), cfg, result)
	toolErrs = append(toolErrs, verifyErrs...)
	if verifyErr != nil {
		return verified, toolErrs, verifyErr
	}
	var missing []string
	for i, s := range steps {
		if !s.Edited {
			missing = append(missing, fmt.Sprintf("[%d] %s", i+1, s.File))
		}
	}
	if len(missing) > 0 {
		return verified, toolErrs, fmt.Errorf("%w: %d of %d step(s) made no edit (%s)\n%s",
			ErrStepsIncomplete, len(missing), len(steps), strings.Join(missing, ", "), result.StepReport)
	}
	return verified, toolErrs, nil
}
