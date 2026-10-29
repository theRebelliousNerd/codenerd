package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// The verdict loop: what a caller does with a delegated task whose turn did
// not end /done (policy/task_verdict.mg).
//
// A turn that wrote and could not show a green gate ends /unverified, and one
// whose own report admits unfinished work ends the same way with
// /self_reported_incomplete. The executor says so in the observed return
// (Outcome, Missing); callers that kept the prose and dropped the verdict
// reported the task done anyway -- a background job "finished", a
// consultation answered, a CLI task printed. Nothing made the agent keep
// going. RunUntilVerified is the caller-side half of the /finish round (which
// does the same inside one turn): it sends the task out again with what is
// missing, and the kernel decides when to stop.
//
// A verified turn returns at once and never touches the kernel. For any other
// result each dispatch is recorded as task_dispatch / task_dispatch_gap /
// task_dispatch_blocked rows under a loop atom, the kernel answers
// task_loop_verdict, and the rows are retracted when the loop ends. The loop
// stops when the work is verified, the report names an obstacle, a dispatch
// makes no progress (it wrote nothing and cleared no gap), the verdict names
// nothing to finish, or the caller's context ends. There is no count.

// ErrNotVerified marks a task the loop stopped re-dispatching before it was
// verified. The *NotVerifiedError carries which stop and the last return.
var ErrNotVerified = errors.New("the task did not reach a verified end")

// Verdicts the kernel gives the last dispatch (task_loop_verdict).
const (
	verdictRedispatch = "/redispatch"
	verdictStalled    = "/stalled"
	verdictBlocked    = "/blocked"
	verdictUnjudged   = "/unjudged"
	verdictVerified   = "/verified"
)

// Dispatch runs a task once and returns what the executor measured about it.
type Dispatch func(ctx context.Context, req TaskRequest) (observation.Return, error)

// NotVerifiedError is the end of a loop that stopped without a verified turn.
type NotVerifiedError struct {
	// Verdict is the stop: /blocked, /unjudged or /stalled.
	Verdict string
	// Return is the loop's merged return: the last dispatch's verdict and
	// output, the writes of every dispatch.
	Return observation.Return
	// Dispatches is how many times the task went out.
	Dispatches int
}

func (e *NotVerifiedError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%v: the turn ended %s", ErrNotVerified, outcomeWord(e.Return))
	if why := DescribeMissingEvidence(e.Return.Missing); why != "" {
		b.WriteString(": " + why)
	}
	b.WriteString("; " + e.Reason())
	return b.String()
}

// Reason says why the task was not sent again.
func (e *NotVerifiedError) Reason() string {
	switch e.Verdict {
	case verdictBlocked:
		return "its report names an obstacle codeNERD cannot clear, so it was not sent again"
	case verdictStalled:
		return fmt.Sprintf("dispatch %d wrote nothing and cleared nothing the one before it left, so it was not sent again", e.Dispatches)
	default:
		return "the verdict names nothing left to finish, so there was nothing to send back"
	}
}

// Is makes errors.Is(err, ErrNotVerified) true for every stop.
func (e *NotVerifiedError) Is(target error) bool { return target == ErrNotVerified }

func outcomeWord(ret observation.Return) string {
	if ret.Outcome == "" {
		return "with no verdict"
	}
	return ret.Outcome
}

var loopSeq atomic.Int64

// RunUntilVerified dispatches req and, while the turn's verdict is not /done
// and the kernel's task_loop_verdict says the work is moving, dispatches it
// again with what is missing. A stop returns the merged return and a
// *NotVerifiedError; a dispatch error returns as it came, with the writes so
// far. The kernel is asked only about a turn that did not end /done.
func RunUntilVerified(ctx context.Context, k types.Kernel, dispatch Dispatch, req TaskRequest) (observation.Return, error) {
	if dispatch == nil {
		return observation.Return{}, errors.New("verdict loop: no dispatch")
	}
	var (
		merged observation.Return
		facts  []types.Fact
		loop   types.MangleAtom
	)
	defer func() {
		if k != nil && len(facts) > 0 {
			if err := k.RetractExactFactsBatch(facts); err != nil {
				logging.Get(logging.CategorySession).Warn("verdict loop %s: retract its rows: %v", loop, err)
			}
		}
	}()
	current := req
	for n := int64(1); ; n++ {
		ret, err := dispatch(ctx, current)
		if n == 1 {
			merged = ret
		} else {
			merged = mergeReturns(merged, ret)
		}
		if err != nil {
			return merged, err
		}
		if ret.Done() {
			return merged, nil
		}
		if k == nil {
			return merged, &NotVerifiedError{Verdict: verdictUnjudged, Return: merged, Dispatches: int(n)}
		}
		if loop == "" {
			loop = types.MangleAtom(fmt.Sprintf("/task_loop_%d", loopSeq.Add(1)))
		}
		rows := dispatchFacts(loop, n, ret)
		if err := k.AssertBatch(rows); err != nil {
			return merged, fmt.Errorf("verdict loop: record dispatch %d: %w", n, err)
		}
		facts = append(facts, rows...)
		verdict, err := loopVerdict(k, loop)
		if err != nil {
			return merged, err
		}
		logging.Get(logging.CategorySession).Info("verdict loop %s: dispatch %d ended %s, missing %v: %s", loop, n, outcomeWord(ret), ret.Missing, verdict)
		switch verdict {
		case verdictRedispatch:
		case verdictBlocked, verdictUnjudged, verdictStalled:
			return merged, &NotVerifiedError{Verdict: verdict, Return: merged, Dispatches: int(n)}
		default:
			return merged, fmt.Errorf("verdict loop: the kernel answered %q for dispatch %d", verdict, n)
		}
		if err := ctx.Err(); err != nil {
			return merged, err
		}
		current = req
		current.Task = strings.TrimSpace(req.Task) + redispatchBrief(ret)
	}
}

// dispatchFacts are one dispatch's rows for the loop.
func dispatchFacts(loop types.MangleAtom, n int64, ret observation.Return) []types.Fact {
	outcome := types.MangleAtom("/no_verdict")
	if ret.Outcome != "" {
		outcome = types.MangleAtom(ret.Outcome)
	}
	// What the dispatch wrote: the paths the executor listed, or the writes it
	// recorded (the two describe one set; a return may carry only one).
	wrote := max(len(ret.Changed), len(ret.Writes))
	rows := []types.Fact{{Predicate: "task_dispatch", Args: []any{loop, n, outcome, int64(wrote)}}}
	for _, gap := range ret.Missing {
		rows = append(rows, types.Fact{Predicate: "task_dispatch_gap", Args: []any{loop, n, types.MangleAtom(gap)}})
	}
	if ret.Report != nil {
		rows = append(rows, types.Fact{Predicate: "task_dispatch_blocked", Args: []any{loop, n, ret.Report.Blocked}})
	}
	return rows
}

// loopVerdict is the kernel's one task_loop_verdict for loop.
func loopVerdict(k types.Kernel, loop types.MangleAtom) (string, error) {
	rows, err := k.Query("task_loop_verdict")
	if err != nil {
		return "", fmt.Errorf("verdict loop: ask task_loop_verdict: %w", err)
	}
	var verdicts []string
	for _, f := range rows {
		if types.ArgString(f, 0) == string(loop) {
			verdicts = append(verdicts, types.ArgString(f, 1))
		}
	}
	if len(verdicts) != 1 {
		return "", fmt.Errorf("verdict loop: the kernel derived %d task_loop_verdict rows for %s (%v), want one", len(verdicts), loop, verdicts)
	}
	return verdicts[0], nil
}

// redispatchBrief hands the task back with what the last dispatch left: the
// verdict and the unmet evidence the kernel named, the unwritten tests and
// the notes the executor measured, and the dispatch's own report, whole.
func redispatchBrief(ret observation.Return) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n\n=== THE PREVIOUS DISPATCH OF THIS TASK ENDED %s, NOT VERIFIED ===\n", strings.ToUpper(strings.TrimPrefix(outcomeWord(ret), "/")))
	if why := DescribeMissingEvidence(ret.Missing); why != "" {
		b.WriteString("Missing: " + why + "\n")
	}
	if len(ret.Untested) > 0 {
		b.WriteString("Written with no test beside it: " + strings.Join(ret.Untested, " ") + "\n")
	}
	for _, note := range ret.Notes {
		b.WriteString("Note: " + note + "\n")
	}
	if report := strings.TrimSpace(ret.Output); report != "" {
		b.WriteString("\nIts report:\n" + report + "\n")
	}
	b.WriteString("\nWhat it did is in the workspace: read what is there and close what is missing, rather than repeating it. " +
		"If an obstacle you cannot clear yourself stops you (a permission or policy denial, a missing credential or input, " +
		"a choice only the user can make), say exactly what it is and what you need, instead of doing more.")
	return b.String()
}

// mergeReturns folds the next dispatch into the loop's return. The verdict
// fields (outcome, missing, stage, gates, report) are the latest dispatch's;
// what the dispatches changed is all of theirs, and a path keeps what it held
// before the first dispatch that wrote it, so a caller undoing the loop's
// writes restores the original.
func mergeReturns(prev, next observation.Return) observation.Return {
	out := next
	out.Duration = prev.Duration + next.Duration
	out.Changed = unionStrings(prev.Changed, next.Changed)
	out.Effects = append(append([]observation.EffectReceipt(nil), prev.Effects...), next.Effects...)
	out.Writes = mergeWrites(prev.Writes, next.Writes)
	return out
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var out []string
	for _, list := range [][]string{a, b} {
		for _, s := range list {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}

func mergeWrites(prev, next []observation.FileWrite) []observation.FileWrite {
	if len(prev) == 0 {
		return next
	}
	out := append([]observation.FileWrite(nil), prev...)
	index := make(map[string]int, len(out))
	for i, w := range out {
		index[w.Path] = i
	}
	for _, w := range next {
		if i, ok := index[w.Path]; ok {
			out[i].After = w.After
			continue
		}
		index[w.Path] = len(out)
		out = append(out, w)
	}
	return out
}
