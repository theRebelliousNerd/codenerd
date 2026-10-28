// Package verification runs a chat delegation to its end: it spawns the
// persona's turn, reads the turn's kernel verdict, has an LLM judge look at an
// attempt the kernel called done -- over a host-built evidence packet, under a
// reply contract -- and asks the kernel what happens next (delegation_move,
// policy/delegation.mg): accept, retry carrying the grounded defect, escalate
// at the persona's cap, ask the judge again with the evidence it requested, or
// fail closed when the attempt cannot be verified. The kernel's verdict
// decides; the judge can only withhold (sweep finding F5), and a judge that
// fails is never repaired by re-running the worker.
package verification

import (
	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
)

// ErrMaxRetriesExceeded is returned when verification fails after max retries.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded - escalating to user")

// ErrVerificationUnavailable is returned when verification itself could not
// run or could not settle the attempt (LLM failure, a reply that broke the
// judge contract, missing client, evidence the judge could not get). It is a
// fail-closed signal: the shard result is NOT verified and must not be treated
// as a success. Callers should use errors.Is to detect it.
var ErrVerificationUnavailable = errors.New("verification unavailable")

// QualityViolation represents a type of corner-cutting detected in code.
type QualityViolation string

const (
	MockCode        QualityViolation = "mock_code"        // func Mock..., // mock implementation
	PlaceholderCode QualityViolation = "placeholder"      // TODO, FIXME, placeholder, stub
	HallucinatedAPI QualityViolation = "hallucinated_api" // APIs that don't exist
	IncompleteImpl  QualityViolation = "incomplete"       // panic("not implemented")
	HardcodedValues QualityViolation = "hardcoded"        // Magic strings instead of real logic
	EmptyFunction   QualityViolation = "empty_function"   // func Foo() {} with no body
	MissingErrors   QualityViolation = "missing_errors"   // No error handling
	FakeTests       QualityViolation = "fake_tests"       // Tests that don't test anything
)

// CorrectiveType defines the type of corrective action to take.
type CorrectiveType string

// The kinds of next step a judge may suggest. They reach the retry as words
// (retryTask); nothing here runs them.
const (
	CorrectiveResearch  CorrectiveType = "research"  // look something up
	CorrectiveDocs      CorrectiveType = "docs"      // read an API's documentation
	CorrectiveTool      CorrectiveType = "tool"      // a tool the task lacks
	CorrectiveDecompose CorrectiveType = "decompose" // break the task into smaller steps
)

// CorrectiveAction is the judge's suggested next step for a failed attempt.
// It names no shard: who retries is the persona's, not the judge's (F5).
type CorrectiveAction struct {
	Type   CorrectiveType `json:"type"`
	Query  string         `json:"query"`
	Reason string         `json:"reason"`
}

// VerificationResult is how a delegation ended: the kernel's disposition of
// its last attempt and, separately, the judge's own judgment of it.
//
// Until 2026-10-27 one struct was both: the verifier rewrote the judge's
// reason and set Success over a below-threshold objection, so a reader of the
// result could not tell the judge's word from the kernel's.
type VerificationResult struct {
	// Accepted is the kernel's disposition (delegation_move /accept).
	Accepted bool
	// Reason is why, in the disposition's terms.
	Reason string
	// Judgment is the judge's host-validated, grounded judgment of the
	// attempt, never edited by the disposition. Nil when no judge was due or
	// none could be had.
	Judgment *Judgment
	// Evidence is the packet the judge was shown; a retry cites its items.
	Evidence *EvidencePacket
}

// Kernel is what a delegation needs of the kernel: to assert its facts, ask
// what they derive, and retract them when it ends.
type Kernel interface {
	Query(predicate string) ([]types.Fact, error)
	Assert(fact types.Fact) error
	RetractFact(fact types.Fact) error
}

// TaskVerifier runs chat delegations to their end.
type TaskVerifier struct {
	mu           sync.RWMutex
	client       perception.LLMClient
	localDB      *store.LocalStore
	taskExecutor session.TaskExecutor // Runs each attempt (ExecuteObserved)
	kernel       Kernel               // Decides each delegation's attempts

	// The judge's system prompt is compiled from its atoms
	// (internal/prompt/atoms/eval/delegation_judge.yaml) under the configured
	// prompt budget (the jit section of config.json).
	compiler *prompt.JITPromptCompiler
	jit      config.JITConfig

	// Session context for persistence
	sessionID string
	turnCount int
}

// SetTaskExecutor sets the unified task executor for shard spawning.
func (v *TaskVerifier) SetTaskExecutor(te session.TaskExecutor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.taskExecutor = te
}

// SetKernel sets the kernel that decides each delegation's attempts.
func (v *TaskVerifier) SetKernel(k Kernel) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.kernel = k
}

// NewTaskVerifier creates a verifier: the judge's client, the store its
// judgments are recorded in, and the compiler and budget its system prompt is
// compiled with. The executor and the kernel are set after boot.
func NewTaskVerifier(client perception.LLMClient, localDB *store.LocalStore, compiler *prompt.JITPromptCompiler, jit config.JITConfig) *TaskVerifier {
	return &TaskVerifier{client: client, localDB: localDB, compiler: compiler, jit: jit}
}

// judgeSystemPrompt compiles the judge's system prompt from its atoms. The
// compile's shard type is the rubric the kernel derived
// (delegation_judge_rubric: /review or /implementation, as /judge_review or
// /judge_implementation -- regime values, so only the judge's atoms answer to
// them), and its intent is /judge, which no worker intent atom claims. While
// live, the judge's atom supersedes the Piggyback output protocol and the tool
// and methodology atoms: its reply is one bare JSON object and it has no
// tools. Until 2026-09-23 this was two Go string literals. Without a compiler
// the judge cannot be asked, and the verdict fails closed.
func (v *TaskVerifier) judgeSystemPrompt(ctx context.Context, rubric string) (string, error) {
	if v.compiler == nil {
		return "", fmt.Errorf("%w: no prompt compiler for the judge", ErrVerificationUnavailable)
	}
	cc := prompt.NewCompilationContext()
	cc.ShardType = "/judge_" + strings.TrimPrefix(rubric, "/")
	cc.ShardName = "Delegation Judge"
	cc.OperationalMode = "/active"
	cc.IntentVerb = "/judge"
	cc.TokenBudget = v.jit.TokenBudget
	cc.ReservedTokens = v.jit.ReservedTokens
	cc.ReservedTokensFallbackRatio = v.jit.ReservedTokensFallbackRatio
	compiled, err := v.compiler.Compile(ctx, cc)
	if err != nil {
		return "", fmt.Errorf("compile the judge's prompt (rubric %s): %w", rubric, err)
	}
	if compiled == nil || strings.TrimSpace(compiled.Prompt) == "" {
		return "", fmt.Errorf("%w: the judge's prompt compiled empty (rubric %s)", ErrVerificationUnavailable, rubric)
	}
	return compiled.Prompt, nil
}

// SetSessionContext sets the current session for persistence.
func (v *TaskVerifier) SetSessionContext(sessionID string, turnCount int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.sessionID = sessionID
	v.turnCount = turnCount
}

// Delegation is one chat request handed to a persona.
type Delegation struct {
	// Task is what the persona is asked to do.
	Task string
	// Persona is the shard persona ("coder", "reviewer", a specialist's
	// name) or an intent verb.
	Persona string
	// MaxAttempts is the persona's attempt cap,
	// shard_profiles.<persona>.max_retries.
	MaxAttempts int
	// Config is the delegation section: the thresholds the kernel decides
	// with (Params) and the bounds on the judge's evidence and reply.
	Config config.DelegationConfig
	// SessionContext is what every attempt's turn is handed: the chat's
	// blackboard for this delegation (prior shard results, the turn's
	// context), the same one an unverified delegation runs with. Nil runs
	// each attempt without one.
	SessionContext *types.SessionContext
	// Priority is each attempt's spawn priority, as an unverified delegation
	// of the same request would have it.
	Priority types.SpawnPriority
}

// VerifyWithRetry runs a delegation to its end. Each attempt spawns the
// persona's turn through the observed executor, so the turn's kernel verdict
// and its evidence come back typed; that verdict, and the judge's grounded
// judgment of an attempt the kernel called done, are asserted, and the kernel
// derives the move (delegation_move): accept; retry carrying the grounded
// defect; escalate at the persona's cap (ErrMaxRetriesExceeded); or, when the
// judge cannot settle the attempt, fail closed (ErrVerificationUnavailable).
// The judge can only withhold, and a judge that cannot run or broke its reply
// contract fails the delegation closed: retrying the worker cannot fix it.
//
// Until 2026-09-23 the attempts went through the string-only executor, which
// dropped the verdict, so a turn the kernel ended /unverified completed the
// request on the judge's word; an LLM advisor picked who retried, and a retry
// without a corrective action re-ran the task with no word of why. Until
// 2026-10-27 the judge read the task and the prose alone, and any JSON that
// decoded was a judgment.
func (v *TaskVerifier) VerifyWithRetry(ctx context.Context, d Delegation) (string, *VerificationResult, error) {
	v.mu.RLock()
	te, k := v.taskExecutor, v.kernel
	v.mu.RUnlock()
	observed, ok := te.(session.ObservedTaskExecutor)
	if !ok {
		return "", nil, fmt.Errorf("task executor %T returns no verdict; a delegation reads each attempt's", te)
	}
	if k == nil {
		return "", nil, errors.New("no kernel to decide the delegation's attempts")
	}
	if d.MaxAttempts < 1 {
		return "", nil, fmt.Errorf("no attempt cap for %s: shard_profiles.%s.max_retries must be at least 1", d.Persona, d.Persona)
	}
	d.Config = d.Config.WithDefaults()
	if err := config.EnsureParams(k, d.Config.Params()); err != nil {
		return "", nil, fmt.Errorf("assert the delegation thresholds: %w", err)
	}

	// Verification retries multiply spend: a task verified three times costs
	// four inferences, not one. Attributing them here is what makes that
	// multiplier visible instead of hiding inside the caller's budget.
	ctx = broker.WithPurpose(ctx, broker.PurposeVerification)

	root := fmt.Sprintf("delegation-%d", time.Now().UnixNano())
	defer forgetDelegation(k, root)
	if err := k.Assert(types.Fact{Predicate: "delegation_request", Args: []any{root, personaAtom(d.Persona), int64(d.MaxAttempts)}}); err != nil {
		return "", nil, fmt.Errorf("assert the delegation: %w", err)
	}

	intent := strings.TrimSpace(d.Persona)
	task := d.Task
	for attempt := int64(1); ; attempt++ {
		// With the delegation's session context, as the chat hands an
		// unverified delegation. Until 2026-09-25 every attempt went through
		// ExecuteObserved and ran without the blackboard: the delegations the
		// kernel routes through verification -- mutations -- were the ones
		// that never saw the prior shard results.
		ret, err := observed.ExecuteObservedWithContext(ctx, session.TaskRequest{IntentVerb: intent, Task: task}, d.SessionContext, d.Priority)
		if err != nil {
			return ret.Output, nil, fmt.Errorf("shard execution failed: %w", err)
		}
		if err := k.Assert(types.Fact{Predicate: "delegation_attempt", Args: []any{root, attempt, outcomeAtom(ret.Outcome)}}); err != nil {
			return ret.Output, nil, fmt.Errorf("assert attempt %d: %w", attempt, err)
		}

		verdict := &VerificationResult{Reason: turnReason(ret.Outcome, ret.Missing)}
		due, err := derivedFor(k, "delegation_judge_due", root, attempt)
		if err != nil {
			return ret.Output, verdict, err
		}
		if len(due) > 0 {
			rubric, err := derivedFor(k, "delegation_judge_rubric", root, -1)
			if err != nil || len(rubric) != 1 {
				return ret.Output, verdict, fmt.Errorf("the kernel derived %d judge rubrics for %s (%v)", len(rubric), root, err)
			}
			judgment, packet, jerr := v.judge(ctx, k, root, attempt, d, ret, rubric[0])
			verdict.Evidence = packet
			if jerr != nil {
				verdict.Reason = fmt.Sprintf("verification unavailable: %v", jerr)
				v.storeVerification(d, verdict, attempt, jerr)
				return ret.Output, verdict, fmt.Errorf("shard result could not be verified (%v): %w", jerr, ErrVerificationUnavailable)
			}
			verdict.Judgment = judgment
		}

		moves, err := derivedFor(k, "delegation_move", root, attempt)
		if err != nil {
			return ret.Output, verdict, err
		}
		if len(moves) != 1 {
			return ret.Output, verdict, fmt.Errorf("the kernel derived %d moves for attempt %d of %s (%v); the policy must decide one", len(moves), attempt, root, moves)
		}
		switch moves[0] {
		case "/accept":
			verdict.Accepted = true
			verdict.Reason = acceptReason(verdict)
			v.storeVerification(d, verdict, attempt, nil)
			return ret.Output, verdict, nil
		case "/escalate":
			verdict.Reason = rejectReason(verdict)
			v.storeVerification(d, verdict, attempt, nil)
			return ret.Output, verdict, ErrMaxRetriesExceeded
		case "/retry":
			verdict.Reason = rejectReason(verdict)
			v.storeVerification(d, verdict, attempt, nil)
			task = retryTask(d.Task, verdict)
		case "/unverifiable":
			verdict.Reason = unverifiableReason(verdict)
			v.storeVerification(d, verdict, attempt, nil)
			return ret.Output, verdict, fmt.Errorf("%s: %w", verdict.Reason, ErrVerificationUnavailable)
		default:
			return ret.Output, verdict, fmt.Errorf("the kernel derived an unknown move %s for %s", moves[0], root)
		}
	}
}

// judge asks the LLM judge about one done attempt, over the attempt's frozen
// evidence packet, and asserts each round's grounded verdict. While the kernel
// derives /acquire_evidence -- the judge asked for packet evidence the host
// can supply, within delegation.judge_evidence_rounds -- it asks again with
// that evidence expanded, over the same packet. A judge that cannot be had (no
// client, a failed call, a reply that breaks the contract) is an error: the
// caller fails closed, and the worker is not re-run for it.
func (v *TaskVerifier) judge(ctx context.Context, k Kernel, root string, attempt int64, d Delegation, ret observation.Return, rubric string) (*Judgment, *EvidencePacket, error) {
	packet := buildEvidencePacket(attempt, rubric, ret, *d.Config.JudgeEvidenceBytes)
	if v.client == nil {
		return nil, packet, ErrVerificationUnavailable
	}
	systemPrompt, err := v.judgeSystemPrompt(ctx, rubric)
	if err != nil {
		return nil, packet, err
	}
	userPrompt := packet.render(d.Task)
	for round := int64(1); ; round++ {
		reply, err := v.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return nil, packet, fmt.Errorf("verification LLM call failed: %w", err)
		}
		judgment, err := parseJudgment(reply, *d.Config.JudgeReplyBytes)
		if err != nil {
			return nil, packet, fmt.Errorf("the judgment broke the %s contract: %w", JudgeContract, err)
		}
		judgment.ground(packet)
		if err := assertJudgment(k, root, attempt, round, judgment); err != nil {
			return nil, packet, err
		}
		moves, err := derivedFor(k, "delegation_move", root, attempt)
		if err != nil {
			return nil, packet, err
		}
		if !slices.Contains(moves, "/acquire_evidence") {
			return judgment, packet, nil
		}
		userPrompt += "\n\n" + packet.expand(judgment.Expandable)
	}
}

// assertJudgment asserts one judge round: the round, the grounded verdict
// (replacing the previous round's), and whether it requested evidence the
// host can supply.
func assertJudgment(k Kernel, root string, attempt, round int64, j *Judgment) error {
	for _, p := range []string{"judge_verdict", "judge_evidence_requested"} {
		if err := k.RetractFact(types.Fact{Predicate: p, Args: []any{root}}); err != nil {
			return fmt.Errorf("retract the previous round's %s: %w", p, err)
		}
	}
	facts := []types.Fact{
		{Predicate: "judge_round", Args: []any{root, attempt, round}},
		{Predicate: "judge_verdict", Args: []any{root, attempt, types.MangleAtom(j.Status.Atom()), percent(j.Confidence)}},
	}
	if len(j.Expandable) > 0 {
		facts = append(facts, types.Fact{Predicate: "judge_evidence_requested", Args: []any{root, attempt}})
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			return fmt.Errorf("assert %s for attempt %d: %w", f.Predicate, attempt, err)
		}
	}
	return nil
}

// turnReason is what the turn's kernel verdict says, before any judge.
func turnReason(outcome string, missing []string) string {
	switch outcome {
	case "/done":
		return "the turn ended /done"
	case "":
		return "the turn returned no verdict"
	}
	reason := "the turn ended " + outcome
	if why := session.DescribeMissingEvidence(missing); why != "" {
		reason += ": " + why
	}
	return reason
}

// acceptReason says why the kernel accepted, naming an objection it stood
// over. The judgment itself is left as the judge gave it.
func acceptReason(r *VerificationResult) string {
	if j := r.Judgment; j != nil && j.Status == JudgeDefect {
		return fmt.Sprintf("accepted: the turn ended /done; the judge's grounded defect (%.0f%% confidence) is below delegation.judge_reject_confidence: %s", j.Confidence*100, j.Explanation)
	}
	return r.Reason
}

// rejectReason says why an attempt was not accepted: the judge's grounded
// defect when it withheld a done turn, else the turn's own verdict.
func rejectReason(r *VerificationResult) string {
	if j := r.Judgment; j != nil && j.Status == JudgeDefect {
		return fmt.Sprintf("the judge grounded a defect (%.0f%% confidence): %s", j.Confidence*100, j.Explanation)
	}
	return r.Reason
}

// unverifiableReason says why the judge could not settle the attempt.
func unverifiableReason(r *VerificationResult) string {
	j := r.Judgment
	if j == nil {
		return "unverifiable: the judge could not settle the attempt"
	}
	why := j.Explanation
	if j.Downgraded != "" {
		why = j.Downgraded + "; " + why
	}
	return "unverifiable: the judge had insufficient evidence: " + why
}

func outcomeAtom(outcome string) types.MangleAtom {
	if s := strings.ToLower(strings.TrimSpace(outcome)); strings.HasPrefix(s, "/") && len(s) > 1 {
		return types.MangleAtom(s)
	}
	return types.MangleAtom("/none")
}

func personaAtom(persona string) types.MangleAtom {
	s := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(persona), "/")))
	if s == "" {
		return types.MangleAtom("/none")
	}
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return types.MangleAtom("/" + s)
}

// percent is a 0-1 confidence as an integer percent: Mangle compares
// integers only. The contract has already refused one outside [0,1].
func percent(confidence float64) int64 {
	return int64(math.Round(confidence * 100))
}

// derivedFor returns the last argument of every row of predicate whose first
// argument is root and, when attempt is not negative, whose second is
// attempt.
func derivedFor(k Kernel, predicate, root string, attempt int64) ([]string, error) {
	rows, err := k.Query(predicate)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", predicate, err)
	}
	var out []string
	for _, f := range rows {
		if len(f.Args) < 2 || types.ExtractString(f.Args[0]) != root {
			continue
		}
		if attempt >= 0 {
			if n, ok := f.Args[1].(int64); !ok || n != attempt {
				continue
			}
		}
		out = append(out, types.ExtractString(f.Args[len(f.Args)-1]))
	}
	return out, nil
}

// forgetDelegation retracts a finished delegation's facts: they describe
// attempts that are over.
func forgetDelegation(k Kernel, root string) {
	for _, p := range []string{"delegation_request", "delegation_attempt", "judge_verdict", "judge_round", "judge_evidence_requested"} {
		if err := k.RetractFact(types.Fact{Predicate: p, Args: []any{root}}); err != nil {
			logging.SystemShardsWarn("retract %s for %s: %v", p, root, err)
		}
	}
}

// retryTask is the task a retry runs: the original, and why the attempt
// before it was not accepted -- the turn's verdict, or the judge's grounded
// defect with the packet evidence it cites and its suggested next step, as
// words. The judge's corrective action is advice to the next attempt, not an
// action: until 2026-09-23 the verifier ran it, and the judge can only
// withhold (F5). The persona's own compile carries the quality rules.
func retryTask(originalTask string, r *VerificationResult) string {
	var b strings.Builder
	b.WriteString(originalTask)
	if r == nil {
		return b.String()
	}
	if r.Reason != "" {
		b.WriteString("\n\n## Previous Attempt Not Accepted\n")
		b.WriteString(r.Reason)
	}
	j := r.Judgment
	if j == nil || j.Status != JudgeDefect {
		return b.String()
	}
	if len(j.Violations) > 0 {
		b.WriteString("\n\n## Quality Issues to Fix\n")
		for _, v := range j.Violations {
			fmt.Fprintf(&b, "- %s\n", v)
		}
	}
	if cited := r.CitedEvidence(); len(cited) > 0 {
		b.WriteString("\n## Evidence the Review Cites\n")
		for _, c := range cited {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}
	if a := j.CorrectiveAction; a != nil && strings.TrimSpace(a.Query) != "" {
		b.WriteString("\n## Suggested Next Step (from the review)\n")
		fmt.Fprintf(&b, "%s: %s", a.Type, a.Query)
		if a.Reason != "" {
			b.WriteString(" -- " + a.Reason)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// CitedEvidence is each grounded citation as "[ID] label": the packet
// evidence the judge's defect stands on.
func (r *VerificationResult) CitedEvidence() []string {
	if r == nil || r.Judgment == nil {
		return nil
	}
	var out []string
	for _, id := range r.Judgment.Grounded {
		if it, ok := r.Evidence.item(id); ok {
			out = append(out, fmt.Sprintf("[%s] %s", id, it.Label))
		}
	}
	return out
}

// storeVerification persists an attempt's disposition and, beside it, the
// judge's own judgment -- its contract, its claimed and grounded status, its
// confidence and explanation -- so a calibration reader never reconstructs the
// judge's word from a disposition's reason. unavailable is why no judgment
// could be had, when none could.
func (v *TaskVerifier) storeVerification(d Delegation, r *VerificationResult, attempt int64, unavailable error) {
	if v.localDB == nil || r == nil {
		return
	}
	v.mu.RLock()
	sessionID, turnCount := v.sessionID, v.turnCount
	v.mu.RUnlock()

	taskHash := sha256.Sum256([]byte(d.Task))
	rec := store.VerificationRecord{
		SessionID:     sessionID,
		TurnNumber:    turnCount,
		Task:          d.Task,
		ShardType:     d.Persona,
		AttemptNumber: int(attempt - 1),
		Success:       r.Accepted,
		Reason:        r.Reason,
		ResultHash:    hex.EncodeToString(taskHash[:]),
	}
	marshal := func(what string, x any) string {
		data, err := json.Marshal(x)
		if err != nil {
			logging.SystemShardsWarn("failed to marshal %s: %v", what, err)
		}
		return string(data)
	}
	if j := r.Judgment; j != nil {
		rec.Confidence = j.Confidence
		rec.ViolationsJSON = marshal("quality violations", j.Violations)
		rec.CorrectiveJSON = marshal("corrective action", j.CorrectiveAction)
		rec.EvidenceJSON = marshal("cited evidence", r.CitedEvidence())
		rec.JudgeContract = j.Contract
		rec.JudgeClaimed = string(j.Claimed)
		rec.JudgeStatus = string(j.Status)
		rec.JudgeExplanation = j.Explanation
	} else {
		rec.ViolationsJSON, rec.CorrectiveJSON, rec.EvidenceJSON = "null", "null", "null"
		if unavailable != nil {
			rec.JudgeStatus = "unavailable"
			rec.JudgeExplanation = unavailable.Error()
		}
	}
	if err := v.localDB.StoreVerification(rec); err != nil {
		logging.StoreError("failed to store verification result: %v", err)
	}
}
