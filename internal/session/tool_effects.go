package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/observation"
	"codenerd/internal/projectdoc"
	"codenerd/internal/types"
)

// Tool effect receipts (Docs/audits/tool-effect-receipts-and-validation-
// completeness.md). Four facts used to be compressed into one success/error
// at the modular-tool boundary: permission to attempt, what changed, whether
// the result was validated, and whether the task is complete. A validator can
// reject a result after its bytes landed, and the accounting path returned on
// the error before it counted the write, registered its path, invalidated the
// test witness or recorded what the turn left -- so a rejected write was on
// disk and nowhere in the turn's write set. The model was told only that
// validation failed, with the tool's output thrown away, and a blind retry
// stacked a second effect on the first.
//
// A receipt is host-observed and never authority: it grants nothing, and
// every new call still passes every admission gate.

// EffectState is what one call did to one resource, as the host read it back.
type EffectState string

const (
	// EffectChanged: the resource's identity after the call differs from
	// before it.
	EffectChanged EffectState = "changed"
	// EffectUnchanged: the same identity before and after.
	EffectUnchanged EffectState = "unchanged"
	// EffectUnknown: an identity could not be read on one side. Uncertainty
	// is never a fabricated revision: it stays unknown.
	EffectUnknown EffectState = "unknown"
)

// identityAbsent is the identity of a path that does not exist.
const identityAbsent = "absent"

// ResourceEffect is one resource's identity across one call: "sha256:<hex>",
// identityAbsent, or empty when it could not be read.
type ResourceEffect struct {
	Path   string
	Before string
	After  string
	State  EffectState
}

// ToolEffectReceipt is the host's record of one write call that left an
// observed change: what it changed, at which point on the turn's write clock,
// what its validation established, and the recovery the policy derived for it
// (tool_effect_recovery, policy/tool_effects.mg).
type ToolEffectReceipt struct {
	CallID string
	Tool   string
	// Seq is the call's place on the turn's write clock (turn_write_seq).
	Seq int
	// HandlerError is the handler's own failure, when it failed after
	// changing a file. A post-action rejection is in Validation instead.
	HandlerError string
	Effects      []ResourceEffect
	Validation   types.ToolValidation
	// Revalidated is set when the host measured a change no validator had
	// (the policy's /revalidate move).
	Revalidated bool
	// Recovery is the policy's move for the call when it returned: /repair,
	// /inspect, /reconcile, or empty when the change was measured.
	Recovery string
	// Closure is what the call still owed when the turn closed
	// (recordEffectClosure); nil before the closure.
	Closure *observation.EffectClosure
}

// Changed reports whether the call left any resource changed or unknown.
func (r ToolEffectReceipt) Changed() bool {
	for _, eff := range r.Effects {
		if eff.State != EffectUnchanged {
			return true
		}
	}
	return false
}

// ChangedPaths is every resource the call left changed or of unknown state.
func (r ToolEffectReceipt) ChangedPaths() []string {
	var out []string
	for _, eff := range r.Effects {
		if eff.State != EffectUnchanged {
			out = append(out, eff.Path)
		}
	}
	return out
}

// recoveryNote is the receipt as the model reads it beside the call's
// output: what changed and what it owes, in the policy's terms. Empty when
// the policy derived no recovery.
func (r ToolEffectReceipt) recoveryNote() string {
	paths := strings.Join(r.ChangedPaths(), ", ")
	why := strings.TrimSpace(r.Validation.Reason)
	if why == "" {
		why = string(r.Validation.State)
	}
	switch r.Recovery {
	case "/repair":
		return fmt.Sprintf("Host receipt: this call changed %s, and post-action validation rejected the result (%s). The change is on disk: read and repair the changed file. Repeating the same call will not undo it.", paths, why)
	case "/inspect", "/revalidate":
		return fmt.Sprintf("Host receipt: this call changed %s, but no completed validation measured the change (%s). Read the file to confirm what it holds before relying on it.", paths, why)
	case "/reconcile":
		return fmt.Sprintf("Host receipt: what %s holds after this call could not be read back. Establish the file's state before writing it again.", paths)
	}
	return ""
}

// PostconditionError is a tool call whose handler reported a result that its
// post-action validation rejected. The output travels with it: a failed
// postcondition is not proof of no mutation, and the model should not have to
// re-run the effect to see what it did.
type PostconditionError struct {
	Validation types.ToolValidation
}

func (e *PostconditionError) Error() string {
	return "post-action validation failed: " + e.Validation.Summary()
}

// contentIdentity is a path's identity now: its content digest, absent, or
// unknown (ok false) when it cannot be read.
func contentIdentity(path string) (string, bool) {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		sum := sha256.Sum256(data)
		return "sha256:" + hex.EncodeToString(sum[:]), true
	case errors.Is(err, fs.ErrNotExist):
		return identityAbsent, true
	default:
		return "", false
	}
}

// snapshotEffectTargets reads each target's identity before the call. It
// never fails the call: an unreadable target is an unknown before-state.
func snapshotEffectTargets(args map[string]any, workspace string) []ResourceEffect {
	paths, err := projectdoc.TargetPaths(args)
	if err != nil {
		return nil
	}
	out := make([]ResourceEffect, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		normalized := canonicalizeWrittenPath(p, workspace)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		before, _ := contentIdentity(turnFilePath(workspace, normalized))
		out = append(out, ResourceEffect{Path: normalized, Before: before})
	}
	return out
}

// observeEffects reads each target's identity after the call and classifies
// it against the snapshot.
func observeEffects(targets []ResourceEffect, workspace string) []ResourceEffect {
	out := make([]ResourceEffect, len(targets))
	for i, t := range targets {
		after, _ := contentIdentity(turnFilePath(workspace, t.Path))
		t.After = after
		switch {
		case t.Before == "" || t.After == "":
			t.State = EffectUnknown
		case t.Before == t.After:
			t.State = EffectUnchanged
		default:
			t.State = EffectChanged
		}
		out[i] = t
	}
	return out
}

type toolValidationLogKey struct{}

type toolValidationLog struct {
	validation types.ToolValidation
	recorded   bool
}

// withToolValidationLog gives one call a place for executeToolCall to record
// what its post-action validation established, the way WithTestRunLog does
// for the test processes it starts.
func withToolValidationLog(ctx context.Context) (context.Context, func() (types.ToolValidation, bool)) {
	log := &toolValidationLog{}
	return context.WithValue(ctx, toolValidationLogKey{}, log), func() (types.ToolValidation, bool) {
		return log.validation, log.recorded
	}
}

// recordToolValidation records a call's validation on its log, when it has one.
func recordToolValidation(ctx context.Context, v types.ToolValidation) {
	if log, ok := ctx.Value(toolValidationLogKey{}).(*toolValidationLog); ok && log != nil {
		log.validation, log.recorded = v, true
	}
}

// settleToolEffects accounts a write call's effect on every terminal path and
// returns the output the model reads. A write that completed is the turn's, as
// it always was. A write that failed -- in its handler or its post-action
// validation -- after observably changing a file is the turn's too: its paths
// join the write set, it advances the write clock and so invalidates every
// earlier test witness, and what it left is recorded for the restore. A
// failed write that changed nothing is not an effect and owes nothing.
//
// A call that left a change gets a receipt, asserted for the policy
// (turn_tool_effect, turn_tool_validation); the policy derives what it owes
// (tool_effect_recovery), the host carries out the one move that is its own
// (/revalidate: measure a change no validator measured), and the rest is said
// to the model beside the output.
func (e *Executor) settleToolEffects(ctx context.Context, call types.ToolCall, targets []ResourceEffect, out string, callErr error, validationOf func() (types.ToolValidation, bool), result *ExecutionResult) string {
	if result == nil || !isWriteMutationTool(call.Name) {
		return out
	}
	workspace := e.workspaceForVerification()
	receipt := ToolEffectReceipt{
		CallID:  fmt.Sprintf("%s#%d", call.ID, result.ToolCallsExecuted),
		Tool:    call.Name,
		Effects: observeEffects(targets, workspace),
	}
	if v, ok := validationOf(); ok {
		receipt.Validation = v
	} else {
		receipt.Validation = types.ToolValidation{State: types.ToolValidationNotRun, Reason: "no validation ran on the result"}
	}
	var rejected *PostconditionError
	if callErr != nil && !errors.As(callErr, &rejected) {
		receipt.HandlerError = callErr.Error()
	}
	changed := receipt.Changed()
	if callErr != nil && !changed {
		return out
	}

	result.WriteToolEffects++
	// A run before this write says nothing about what the write left. The
	// receipt facts stay; the write's seq is what puts them before it.
	result.TestRunSinceLastWrite = nil
	result.FinalTestCheck = nil
	e.noteWrite(result)
	if callErr == nil {
		if err := recordWrittenPaths(result, call.Input, workspace); err != nil {
			logging.Get(logging.CategorySession).Warn(
				"successful write %s returned invalid target metadata: %v", call.Name, err)
		}
		recordLastWrittenContents(result, call.Input, workspace)
	} else {
		for _, p := range receipt.ChangedPaths() {
			addWrittenPath(result, p)
			recordLastWrittenPath(result, p, workspace)
		}
	}
	if !changed {
		// A completed write that left the bytes as they were: in the write
		// set as before, and nothing for a receipt to say.
		return out
	}

	receipt.Seq = result.gateSeq
	e.assertToolReceipt(result, receipt)
	if e.toolEffectRecovery(result, receipt.CallID) == "/revalidate" {
		if gate, ok := e.interactiveGate(); ok {
			receipt.Validation = gate.ValidateInteractiveToolResult(ctx, receipt.CallID+":revalidate", call.Name, call.Input, out, true)
			receipt.Revalidated = true
			e.assertToolValidation(result, receipt.CallID, 1, receipt.Validation.State)
		}
	}
	receipt.Recovery = e.toolEffectRecovery(result, receipt.CallID)
	result.ToolReceipts = append(result.ToolReceipts, receipt)
	if note := receipt.recoveryNote(); note != "" {
		if strings.TrimSpace(out) == "" {
			return note
		}
		return out + "\n\n" + note
	}
	return out
}

// addWrittenPath adds one canonical path to the turn's write set.
func addWrittenPath(result *ExecutionResult, normalized string) {
	for _, p := range result.WrittenPaths {
		if p == normalized {
			return
		}
	}
	result.WrittenPaths = append(result.WrittenPaths, normalized)
}

// assertToolReceipt asserts one receipt's effects and its first validation
// round for the turn.
func (e *Executor) assertToolReceipt(result *ExecutionResult, r ToolEffectReceipt) {
	if e == nil || e.kernel == nil {
		return
	}
	turn := result.turnAtom()
	for _, eff := range r.Effects {
		if eff.State == EffectUnchanged {
			continue
		}
		e.assertTurnFact(types.Fact{Predicate: "turn_tool_effect", Args: []any{
			turn, types.MangleString(r.CallID), types.MangleString(eff.Path), int64(r.Seq), types.MangleAtom("/" + string(eff.State)),
		}})
	}
	e.assertToolValidation(result, r.CallID, 0, r.Validation.State)
}

// assertToolValidation appends one validation round for a call. Rounds are
// never rewritten: a revalidation is the next round.
func (e *Executor) assertToolValidation(result *ExecutionResult, callID string, round int, state types.ToolValidationState) {
	if e == nil || e.kernel == nil {
		return
	}
	e.assertTurnFact(types.Fact{Predicate: "turn_tool_validation", Args: []any{
		result.turnAtom(), types.MangleString(callID), int64(round), state.Atom(),
	}})
}

// toolEffectRecoveryOrder is which move answers first when the policy derives
// several for one call: measure before judging, and establish an unreadable
// state before repairing it.
var toolEffectRecoveryOrder = []string{"/revalidate", "/reconcile", "/repair", "/inspect"}

// toolEffectRecovery is the policy's move for one call (tool_effect_recovery).
// With no kernel there is no policy, and no move.
func (e *Executor) toolEffectRecovery(result *ExecutionResult, callID string) string {
	if e == nil || e.kernel == nil {
		return ""
	}
	rows, err := e.kernel.Query("tool_effect_recovery")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("query tool_effect_recovery: %v", err)
		return ""
	}
	turn := string(result.turnAtom())
	moves := map[string]bool{}
	for _, f := range rows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != turn || types.ExtractString(f.Args[1]) != callID {
			continue
		}
		moves[types.ExtractString(f.Args[2])] = true
	}
	for _, m := range toolEffectRecoveryOrder {
		if moves[m] {
			return m
		}
	}
	return ""
}

// recordEffectClosure reads, at the closure and while the turn's facts still
// hold, what each receipted call still owes (tool_effect_recovery) and on
// which paths it is still the current effect (turn_effect_current), and
// records it on the receipt. The call-time Recovery stays as history: a
// rejected write a later verified write repaired keeps its rejection, but no
// longer owes /repair, and the parent is not told it does
// (Docs/audits/effect-recovery-closure.md). One read of each relation for
// the turn, not one per receipt. A failed read is recorded as unknown on
// every receipt, never as settled. With no kernel there is no policy, so no
// closure is recorded.
func (e *Executor) recordEffectClosure(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || len(result.ToolReceipts) == 0 {
		return
	}
	recRows, recErr := e.kernel.Query("tool_effect_recovery")
	curRows, curErr := e.kernel.Query("turn_effect_current")
	if recErr != nil || curErr != nil {
		logging.Get(logging.CategorySession).Warn("effect closure: tool_effect_recovery: %v; turn_effect_current: %v", recErr, curErr)
		for i := range result.ToolReceipts {
			result.ToolReceipts[i].Closure = &observation.EffectClosure{State: observation.ClosureUnknown}
		}
		return
	}
	moves := map[string]map[string]bool{}
	for _, f := range recRows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != string(turn) {
			continue
		}
		call := types.ExtractString(f.Args[1])
		if moves[call] == nil {
			moves[call] = map[string]bool{}
		}
		moves[call][types.ExtractString(f.Args[2])] = true
	}
	current := map[string][]string{}
	for _, f := range curRows {
		if len(f.Args) < 3 || types.ExtractString(f.Args[0]) != string(turn) {
			continue
		}
		call := types.ExtractString(f.Args[2])
		current[call] = append(current[call], types.ExtractString(f.Args[1]))
	}
	for i := range result.ToolReceipts {
		r := &result.ToolReceipts[i]
		c := &observation.EffectClosure{Current: current[r.CallID]}
		sort.Strings(c.Current)
		for _, m := range toolEffectRecoveryOrder {
			if moves[r.CallID][m] {
				c.Recovery = m
				break
			}
		}
		switch {
		case len(c.Current) == 0:
			c.State, c.Recovery = observation.ClosureSuperseded, ""
		case c.Recovery != "":
			c.State = observation.ClosureOwed
		default:
			c.State = observation.ClosureSettled
		}
		r.Closure = c
	}
}

// assertSupersededEffects marks, at the closure, every receipted change whose
// path no longer holds what the call left: a writer the receipts did not
// observe replaced it, so the call's validation describes bytes that are gone
// (turn_tool_effect_superseded). An unreadable path is not superseded: what
// it holds is unknown, and the receipt's own state stands.
func (e *Executor) assertSupersededEffects(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	workspace := e.workspaceForVerification()
	for _, r := range result.ToolReceipts {
		for _, eff := range r.Effects {
			if eff.State != EffectChanged {
				continue
			}
			now, ok := contentIdentity(turnFilePath(workspace, eff.Path))
			if !ok || now == eff.After {
				continue
			}
			e.assertTurnFact(types.Fact{Predicate: "turn_tool_effect_superseded", Args: []any{
				turn, types.MangleString(r.CallID), types.MangleString(eff.Path),
			}})
		}
	}
}
