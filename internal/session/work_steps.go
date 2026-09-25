package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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
// the turn fails rather than claiming the task. A step may instead conclude,
// with evidence, that its change is not needed, and that is reported, not failed.

// ErrStepsIncomplete marks a planned task some of whose steps made no edit.
// Wrapped so errors.Is can tell it from a provider failure.
var ErrStepsIncomplete = errors.New("planned steps incomplete")

const workStepPlanSystem = `You divide one code-change task into the edit steps an executive will run one at a time, each as its own turn with the file named.

Output one line per step, in the order the task gives them, in exactly this form:
STEP <workspace-relative file path> :: <the change to make in that file, with its location>

Rules: one step per file region the task says to change; keep the task's own numbering, names, line numbers and wording; a test the task asks for is its own step; an import a step needs is part of that step, never a step of its own; never add a file the task does not name or clearly imply; never add a step the task does not ask for; the task's verification command is not a step, the executive runs it. A task with one change is one STEP line. If the task names no file and clearly implies none, reply with exactly one line: NO STEPS. Output only STEP lines, or NO STEPS, and nothing else.`

// workStep is one edit site of a planned task and what became of it.
// CoveredBy names the earlier step (1-based) that edited the same file when
// this one made no edit: a planner that splits an import out of the change
// that needs it produces a step the first pass has already done, and that
// is not a missed edit. Zero when the step edited or nothing covers it.
type workStep struct {
	File      string
	Change    string
	Edited    bool
	CoveredBy int
	Calls     int
	Note      string // the model's closing sentence, or the pass error
	// NoChange is the evidence the model gave, on the step's last pass, that
	// the step's change is already in place or its condition does not hold;
	// empty unless the step made no edit.
	NoChange string
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
// path and a change is not a step, and neither is a verification (a path
// that is a directory on disk when workspace is known, or a change that is
// the task's verify command: the executive runs verification itself).
// Paths that do not exist yet (new files) stay. Duplicates collapse; the
// count is bounded by maxSteps (session.step_plan_max_steps): a task that
// divides into more is not one turn's work, and the executive runs the first
// steps and reports.
func parseWorkSteps(workspace, text string, maxSteps int) []workStep {
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
		if file == "" || change == "" || !isEditStep(file, change) {
			continue
		}
		if workspace != "" && isWorkspaceDir(workspace, file) {
			continue
		}
		key := file + "\x00" + change
		if seen[key] {
			continue
		}
		seen[key] = true
		steps = append(steps, workStep{File: file, Change: change})
		if len(steps) == maxSteps {
			break
		}
	}
	return steps
}

// isWorkspaceDir reports whether file names an existing directory on disk,
// resolved against workspace unless absolute.
func isWorkspaceDir(workspace, file string) bool {
	p := file
	if !filepath.IsAbs(p) {
		p = filepath.Join(workspace, filepath.FromSlash(p))
	}
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// workStepPlanUser is the planning request body: the task and the turn's
// intent verb. The bare task was sent before, and on the chat path the task
// is a one-line summary built from the intent ("fix issue in <target>"), so
// the planner had a 48-character request to divide into steps.
func workStepPlanUser(task, verb string) string {
	if verb == "" {
		verb = "unknown"
	}
	return "Task: " + task + "\nIntent verb: " + verb
}

// emptyCompletionError reports whether err describes a planner answer that
// arrived with no text. Retrying that answer only replays the same request,
// so the caller treats it as a fast single pass instead of a retry.
func emptyCompletionError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "empty completion")
}

// planTurnSteps decides whether this turn is a planned task and, if so, what
// its steps are. The transport has to be able to run steps (a tool loop inside
// a working loop, with a write tool); whether this brief is worth
// a planning call is the policy's: turn_needs_step_plan derives from the edit
// sites the brief names (briefSites), and a brief that names fewer than
// session.step_plan_min_sites of them runs as one pass without asking.
// Measured 2026-09-21: a campaign run made 13 planning calls, every one on a
// one-site brief, and every one came back "one step". A plan with fewer than
// two steps is a single pass too.
func (e *Executor) planTurnSteps(ctx context.Context, client types.LLMClient, task string, cfg *config.EffectiveAgentRuntimeConfig, result *ExecutionResult) []workStep {
	// This runs before beginWorkingLoop installs the loop, so it asks whether
	// one will exist rather than reading it off the context.
	if !e.workingLoopAvailable() || result == nil {
		return nil
	}
	if !e.writeOrientedIntent(result.Intent.Verb) {
		return nil
	}
	// Each step is a pass of the tool loop, so the client needs a
	// continuation channel: native, or the Piggyback envelope channel.
	if _, ok := e.toolResultsChannel(client, cfg); !ok {
		return nil
	}
	if cfg == nil || !hasWriteTool(cfg.AllowedTools) {
		return nil
	}
	if !e.briefNeedsStepPlan(task, result) {
		return nil
	}
	settings := e.configSnapshot()
	timeout := settings.StepPlanTimeout
	if timeout <= 0 {
		timeout = defaultSessionPolicy.StepPlanTimeout
	}
	maxSteps := settings.StepPlanMaxSteps
	if maxSteps <= 0 {
		maxSteps = defaultSessionPolicy.StepPlanMaxSteps
	}
	user := workStepPlanUser(task, result.Intent.Verb)
	var text string
	var err error
	for attempt := 1; attempt <= 2; attempt++ {
		planCtx, cancel := context.WithTimeout(ctx, timeout)
		text, err = client.CompleteWithSystem(planCtx, workStepPlanSystem, user)
		cancel()
		if err == nil {
			break
		}
		// An empty answer is not something an identical second request fixes:
		// measured 2026-09-17 21:20, both attempts came back empty (finish_reason
		// "stop", no content) and cost about 40 s before the task ran as one pass.
		if emptyCompletionError(err) {
			logging.Get(logging.CategorySession).Warn("Step planning got an empty model answer; the task runs as one pass")
			return nil
		}
		if attempt == 1 && ctx.Err() == nil {
			logging.Get(logging.CategorySession).Warn("Step planning attempt 1 failed (%v); retrying once", err)
			continue
		}
		logging.Get(logging.CategorySession).Warn("Step planning failed (%v); the task runs as one pass", err)
		return nil
	}
	steps := parseWorkSteps(e.workspaceForVerification(), text, maxSteps)
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

// isEditStep tells an edit site from a verification the planner wrote down
// as a step. Observed 2026-09-11: "STEP internal/prompt/ :: Verify with: go
// test ./internal/prompt/ ..." ran as a fifth step, made no edit, and
// failed the turn.
func isEditStep(file, change string) bool {
	if strings.HasSuffix(file, "/") || strings.HasSuffix(file, "\\") || strings.HasSuffix(file, "/...") {
		return false
	}
	lower := strings.ToLower(change)
	for _, prefix := range []string{"verify with", "verify:", "verification:", "run go test", "run the tests", "run tests"} {
		if strings.HasPrefix(lower, prefix) {
			return false
		}
	}
	return true
}

func hasWriteTool(names []string) bool {
	for _, name := range names {
		if isWriteMutationTool(name) {
			return true
		}
	}
	return false
}

// noChangeEvidence reads the evidence for a step whose change is not needed
// out of the model's closing note: the first line starting
// "NO CHANGE NEEDED:" yields the trimmed text after it, and anything else
// yields nothing.
func noChangeEvidence(note string) string {
	for _, line := range strings.Split(note, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "NO CHANGE NEEDED:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "NO CHANGE NEEDED:"))
	}
	return ""
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
		b.WriteString("This step's first pass made no edit. Reading is closed: the observations already gathered are in the working section, and recall_context recovers any of them whole. Make this step's edit now with the edit tools. If the change is already in place, or the task's own condition for it does not hold, make no edit and reply with one line starting \"NO CHANGE NEEDED:\" followed by the evidence (the file and line you checked and what is there).")
	} else {
		b.WriteString("Make exactly this step's change now with the edit tools; the remaining steps run after it, so do not make them here. When the edit is made, reply with one sentence saying what changed.")
	}
	return b.String()
}

// markCoveredSteps records, for every step that made no edit, the earliest
// step that edited the same file. The task's guarantee is per file: every
// file the plan names was edited, whichever of its steps did it.
func markCoveredSteps(steps []workStep) {
	for i := range steps {
		if steps[i].Edited {
			continue
		}
		for j := range steps {
			if j != i && steps[j].Edited && steps[j].File == steps[i].File {
				steps[i].CoveredBy = j + 1
				break
			}
		}
	}
}

// unfinishedSteps are the steps whose file no step edited.
func unfinishedSteps(steps []workStep) []string {
	var missing []string
	for i, s := range steps {
		if s.NoChange != "" {
			continue
		}
		if !s.Edited && s.CoveredBy == 0 {
			missing = append(missing, fmt.Sprintf("[%d] %s", i+1, s.File))
		}
	}
	return missing
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
		switch {
		case s.Edited:
			status = "edited"
		case s.NoChange != "":
			status = "no change needed"
		case s.CoveredBy > 0:
			status = fmt.Sprintf("no edit (file edited in step %d)", s.CoveredBy)
		}
		fmt.Fprintf(&b, "\n[%d] %s :: %s — %s (%d tool call(s))", i+1, s.File, s.Change, status, s.Calls)
		if note := strings.TrimSpace(s.Note); note != "" {
			fmt.Fprintf(&b, ": %s", firstLine(note))
		}
	}
	return b.String()
}

// languageOfFile asks the kernel what language a file is (policy/coder_language.mg,
// detected_language/2), or "" when the policy has no row for its extension (the
// compile then keeps the project's language). Go measures the extension; the
// table that maps it to a language key is policy. The key is the atom corpus's
// own address for knowledge (languages: ["/mangle"], ["/go"], ["/markdown"]...).
//
// This used to be a Go switch with no row for any prose format, so a turn aimed
// at a Markdown file kept the project's /go and carried ~4.7k tokens of Go atoms
// (measured 2026-09-22 on the features docs campaign), while the policy table
// that did have the row sat dormant: nothing ever asserted file_extension.
func (e *Executor) languageOfFile(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if e.kernel == nil || ext == "" {
		return ""
	}
	measured := types.Fact{Predicate: "file_extension", Args: []interface{}{path, ext}}
	if err := e.kernel.Assert(measured); err != nil {
		logging.Get(logging.CategorySession).Warn("file_extension for %s not asserted: %v", path, err)
		return ""
	}
	defer func() {
		if err := e.kernel.RetractFact(measured); err != nil {
			logging.Get(logging.CategorySession).Warn("file_extension for %s not retracted: %v", path, err)
		}
	}()
	facts, err := e.kernel.Query(fmt.Sprintf("detected_language(%s, Lang)", strconv.Quote(path)))
	if err != nil {
		logging.Get(logging.CategorySession).Warn("detected_language query for %s failed: %v", path, err)
		return ""
	}
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		if lang := types.ExtractString(f.Args[1]); lang != "" {
			return "/" + strings.TrimPrefix(lang, "/")
		}
	}
	return ""
}

// compileNeeds is every need the kernel derives for a compile: the target's
// (targetNeeds) and those of what will read the answer (consumerNeeds for the
// channel this verb's client answers on). Both compile boundaries -- the turn
// and each planned step -- ask the same way.
func (e *Executor) compileNeeds(language, verb string) []string {
	needs := e.targetNeeds(language)
	for _, need := range e.consumerNeeds(e.servingConsumer(verb)) {
		if !slices.Contains(needs, need) {
			needs = append(needs, need)
		}
	}
	return needs
}

// servingConsumer names the path that will read this verb's answer, for the
// kernel's consumer_need (policy/jit_needs.mg): the text channel runs envelope
// tool_requests (piggybackChannel), the native channel runs native tool calls. It asks the client the turn will call (llmForVerb) the
// question generateResponse asks it, so the compile and the channel cannot
// disagree.
func (e *Executor) servingConsumer(verb string) string {
	if usesPiggybackTools(e.llmForVerb(verb)) {
		return "/executor_text_channel"
	}
	return "/executor_native_channel"
}

// consumerNeeds asks the kernel what a compile whose answer this consumer
// reads will need (consumer_need/2), with the consumer bound as a constant.
func (e *Executor) consumerNeeds(consumer string) []string {
	if e.kernel == nil || !validMangleVerb(consumer) {
		return nil
	}
	facts, err := e.kernel.Query(fmt.Sprintf("consumer_need(%s, Need)", consumer))
	return needsDerived("consumer_need", consumer, facts, err)
}

// needsDerived reads the answer to a needs query -- target_need or
// consumer_need with its first argument bound: each row's last argument,
// without the slash. The callers name the predicate in their own Query call,
// so the conclusions gate sees who reads it.
func needsDerived(predicate, bound string, facts []types.Fact, err error) []string {
	if err != nil {
		logging.Get(logging.CategorySession).Warn("needs query %s(%s, Need) failed: %v", predicate, bound, err)
		return nil
	}
	var needs []string
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		need := strings.TrimPrefix(types.ExtractString(f.Args[len(f.Args)-1]), "/")
		if need != "" && !slices.Contains(needs, need) {
			needs = append(needs, need)
		}
	}
	return needs
}

// targetNeeds asks the kernel what a compile aimed at a file of this language
// will need (policy/jit_needs.mg, target_need/2), with the language bound as a
// constant in the query. A compile whose language nothing measured asks as
// /undetected: not knowing is a measurement too, and the policy says what such
// a compile needs. The kernel owns the answer; this only carries it into the
// compilation context, where the atoms gated on those needs are served.
func (e *Executor) targetNeeds(language string) []string {
	if strings.TrimSpace(language) == "" {
		language = "/undetected"
	}
	if e.kernel == nil || !validMangleVerb(language) {
		return nil
	}
	facts, err := e.kernel.Query(fmt.Sprintf("target_need(%s, Need)", language))
	return needsDerived("target_need", language, facts, err)
}

// stepSystemPrompt compiles the system prompt for one planned step: the turn's
// compilation context re-aimed at the step's file and that file's language, so
// the selector serves the knowledge this step needs at the moment it starts
// (the /mangle corpus for a policy file, the Go corpus for a Go file) rather
// than whatever fit the task's first sentence. Observed 2026-09-18: a three-step
// plan whose second step edited internal/context/working_set.mg ran all three
// steps on the prompt compiled for step one, with zero of the corpus's 119
// /mangle atoms in it. The assembly after the compile is the turn's own
// (project instructions, then the target's file context), as the no-tool retry
// does it. A failed compile keeps the turn's prompt: a step never runs on less
// than the turn did.
func (e *Executor) stepSystemPrompt(
	ctx context.Context,
	turnPrompt string,
	stepCtx *prompt.CompilationContext,
	cfg *config.EffectiveAgentRuntimeConfig,
) string {
	if e.jitCompiler == nil || stepCtx == nil {
		return turnPrompt
	}
	if lang := e.languageOfFile(stepCtx.IntentTarget); lang != "" {
		stepCtx.Language = lang
	}
	stepCtx.DerivedNeeds = e.compileNeeds(stepCtx.Language, stepCtx.IntentVerb)
	if cfg != nil && len(cfg.AllowedTools) > 0 && len(stepCtx.AvailableTools) == 0 {
		stepCtx.AvailableTools = slices.Clone(cfg.AllowedTools)
	}
	compiled, err := e.jitCompiler.Compile(ctx, stepCtx)
	if err != nil || compiled == nil || strings.TrimSpace(compiled.Prompt) == "" {
		logging.Get(logging.CategorySession).Warn(
			"step prompt for %s did not compile (%v); the step runs on the turn's prompt", stepCtx.IntentTarget, err)
		return turnPrompt
	}
	logging.Session("Step prompt compiled for %s (language=%s, %d chars)", stepCtx.IntentTarget, stepCtx.Language, len(compiled.Prompt))
	return e.withCompiledFileContext(ctx, e.withProjectInstructions(compiled.Prompt), stepCtx.IntentTarget)
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

		// The window is compiled for THIS step, when the step starts: the
		// turn's prompt was selected for the task as a whole and the project's
		// language, so a step that edits a policy file ran on a Go prompt with
		// none of the /mangle corpus in it. Both passes of the step share it.
		stepPrompt := e.stepSystemPrompt(ctx, systemPrompt, &stepCtx, cfg)

		resp, errs, err := e.runToolLoopPass(ctx, stepPrompt, workStepAnchor(task, steps, i, false), cfg, &stepCtx, result, toolLoopPass{})
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
			retried, retryErrs, retryErr := e.runToolLoopPass(ctx, stepPrompt, workStepAnchor(task, steps, i, true), cfg, &stepCtx, result, toolLoopPass{regime: commitRegime})
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
			// The closing word is the envelope's surface text, not its first
			// brace: a model that answers in the Piggyback envelope closes
			// with a JSON object.
			if text := strings.TrimSpace(e.processPiggybackControlPacket(resp.Text)); text != "" {
				step.Note = text
			}
			last = resp
		}
		if !step.Edited {
			step.NoChange = noChangeEvidence(step.Note)
		}
		logging.Session("Step %d/%d %s: edited=%v, %d tool call(s)", i+1, len(steps), step.File, step.Edited, step.Calls)
	}
	markCoveredSteps(steps)
	result.StepReport = workStepReport(steps)
	if last == nil {
		last = &types.LLMToolResponse{Text: result.StepReport}
	}

	// One gate for the whole task, inside a working loop so a repair round
	// sees what the steps gathered.
	client := e.llmForVerb(result.Intent.Verb)
	trp, _ := e.toolResultsChannel(client, cfg)
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
	if missing := unfinishedSteps(steps); len(missing) > 0 {
		return verified, toolErrs, fmt.Errorf("%w: %d of %d step(s) made no edit and nothing else edited the file (%s)\n%s",
			ErrStepsIncomplete, len(missing), len(steps), strings.Join(missing, ", "), result.StepReport)
	}
	return verified, toolErrs, nil
}
