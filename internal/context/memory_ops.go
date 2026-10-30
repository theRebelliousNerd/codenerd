package context

import (
	"fmt"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// MemoryKernel is what a memory operation needs of the kernel: to record a
// session note and to drop one.
type MemoryKernel interface {
	Assert(fact types.Fact) error
	RetractFact(fact types.Fact) error
}

// MemoryStore is the durable half: cold storage for a promoted preference,
// vector memory for recallable content.
type MemoryStore interface {
	// RememberUserPreference saves or reaffirms a long-term preference; a
	// reaffirmation renews it against archival
	// (store.LocalStore.RememberUserPreference).
	RememberUserPreference(key, value string) error
	StoreVector(content string, metadata map[string]any) error
}

// ApplyMemoryOperation lands one memory operation from a Piggyback control
// packet. It is the one implementation both readers of control packets use:
// the chat compressor (Compressor.processMemoryOperation) and the session
// executor, which is how `nerd run`, delegated tasks and campaigns see the
// model's envelope. Until 2026-09-25 the executor asserted each operation as
// memory_operation(Op, Key, Value), a predicate no .mg file declares, so every
// operation the model sent on those paths was stored where no rule and no
// query could read it.
//
// This is where "remember that I prefer tabs" actually lands, which is why
// both writes report failure rather than dropping it: the user was told the
// thing was remembered, and a write that failed with nobody informed means the
// next session simply does not know.
//
// "forget" drops the session note under the key, and nothing else, unless the
// host marked the operation as the person's own instruction
// (Domain == MemoryDomainUserMemory, which only the perception adapter sets and
// no model envelope can): then it removes the one saved preference the words
// resolve to (memory_forget.go). Either way the disposition's Receipt names the
// domain addressed. It used to
// call Retract(key), which removes every fact of the predicate the key names:
// model-authored control data deleting whatever executive state it chose to
// name -- a user_intent, a security_violation, a pending_action -- through a
// path that never met the mangle_updates filter. A memory operation can only
// take back what a memory operation put there.
//
// The default branch matters for a different reason. The protocol enum in
// internal/articulation/schema.go once admitted four operations while the
// switch handled three, so a "note" -- specified in protocol/piggyback/memory_ops
// as a short-term session observation, and validated as legal on the way in --
// fell through to nothing at all. An operation the model was told to emit must
// not vanish without a word, and the next one added to that enum must not
// either (TestEveryProtocolMemoryOpIsHandled).
//
// The returned disposition says what the operation did: applied, failed (with
// the store's reason), partially applied (a note's two writes), or unsupported.
// It used to return nothing, so a caller that told the user "I saved your
// preference" could not know the store had refused it.
func ApplyMemoryOperation(kernel MemoryKernel, store MemoryStore, op articulation.MemoryOperation) articulation.ControlDisposition {
	d := articulation.ControlDisposition{
		Kind:   articulation.ControlMemoryOperation,
		Detail: articulation.DescribeMemoryOperation(op),
		Stage:  articulation.StageApplied,
	}
	fail := func(stage articulation.ControlStage, reason string) articulation.ControlDisposition {
		d.Stage, d.Reason = stage, reason
		return d
	}
	switch op.Op {
	case "promote_to_long_term":
		logging.ContextDebug("Memory op: promote_to_long_term key=%s", op.Key)
		if store == nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op promote_to_long_term FAILED, no long-term store on this path: key=%s", op.Key)
			return fail(articulation.StageFailed, "there is no long-term store on this path")
		}
		if err := store.RememberUserPreference(op.Key, op.Value); err != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op promote_to_long_term FAILED, the preference is not persisted: key=%s: %v", op.Key, err)
			return fail(articulation.StageFailed, "the preference was not persisted: "+err.Error())
		}
	case "forget":
		logging.ContextDebug("Memory op: forget key=%s domain=%q", op.Key, op.Domain)
		if op.Domain == articulation.MemoryDomainUserMemory {
			return forgetUserMemory(kernel, store, op, d)
		}
		return forgetSessionNote(kernel, op, d)
	case "store_vector":
		logging.ContextDebug("Memory op: store_vector key=%s", op.Key)
		if store == nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op store_vector FAILED, no vector store on this path: key=%s", op.Key)
			return fail(articulation.StageFailed, "there is no vector store on this path")
		}
		if err := store.StoreVector(op.Value, map[string]any{"key": op.Key}); err != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op store_vector FAILED, the content is not recallable: key=%s: %v", op.Key, err)
			return fail(articulation.StageFailed, "the content was not stored: "+err.Error())
		}
	case "note":
		return recordSessionNote(kernel, op.Key, op.Value, d)
	default:
		logging.Get(logging.CategoryContext).Warn(
			"Unhandled memory op %q, dropping: key=%s -- the protocol schema admits an operation this switch does not", op.Op, op.Key)
		return fail(articulation.StageUnsupported, fmt.Sprintf("%q is not a memory operation this path can apply", op.Op))
	}
	return d
}

// ApplyMemoryOperations applies a packet's operations in order and returns one
// disposition per operation, indexed by its position in the list the model
// sent. A caller that tells the user, or its history, what was remembered
// reads these rather than assuming the list took effect.
func ApplyMemoryOperations(kernel MemoryKernel, store MemoryStore, ops []articulation.MemoryOperation) []articulation.ControlDisposition {
	out := make([]articulation.ControlDisposition, 0, len(ops))
	for i, op := range ops {
		d := ApplyMemoryOperation(kernel, store, op)
		d.Index = i
		out = append(out, d)
	}
	return out
}

// sessionNotePredicate carries a memory-op note in the kernel:
// session_note(Key, Value), declared in schemas_context.mg and made relevant
// by context_compilation.mg.
const sessionNotePredicate = "session_note"

// recordSessionNote lands a "note" memory operation: a short-term observation
// for the session (protocol/piggyback/memory_ops), such as
// {"op": "note", "key": "current_focus", "value": "refactoring auth module"}.
//
// It becomes session_note(Key, Value) in the kernel, replacing any earlier
// note under the key, and the kernel decides when it enters the window:
// context_relevant(Key, /p90) :- session_note(Key, _). Until 2026-09-25 the
// operation was admitted by the protocol schema, described to the model, and
// dropped with a warning (TODO-CTX-07A).
func recordSessionNote(kernel MemoryKernel, key, value string, d articulation.ControlDisposition) articulation.ControlDisposition {
	key = strings.TrimSpace(key)
	if key == "" {
		logging.Get(logging.CategoryContext).Warn("Memory op note has no key; dropping: value=%q", truncateForNote(value))
		d.Stage, d.Reason = articulation.StageRejectedParse, "a note needs a key"
		return d
	}
	if kernel == nil {
		logging.Get(logging.CategoryContext).Warn("Memory op note FAILED, no kernel attached: key=%s", key)
		d.Stage, d.Reason = articulation.StageFailed, "there is no kernel attached"
		return d
	}
	// A replacement is two writes. The outcome has to say which of them
	// landed: an assert that fails after the retraction succeeded has removed
	// the old note, and a retraction that fails before a successful assert
	// leaves the old note beside the new one.
	retractErr := kernel.RetractFact(types.Fact{Predicate: sessionNotePredicate, Args: []any{key}})
	if retractErr != nil {
		logging.Get(logging.CategoryContext).Warn("Memory op note could not replace the earlier note: key=%s: %v", key, retractErr)
	}
	if err := kernel.Assert(types.Fact{Predicate: sessionNotePredicate, Args: []any{key, value}}); err != nil {
		logging.Get(logging.CategoryContext).Warn(
			"Memory op note FAILED, the observation is not in session context: key=%s: %v", key, err)
		d.Stage = articulation.StageFailed
		d.Reason = "the observation is not in session context: " + err.Error()
		if retractErr == nil {
			d.Stage = articulation.StagePartiallyApplied
			d.Reason += " (any earlier note under this key was already removed)"
		}
		return d
	}
	if retractErr != nil {
		d.Stage = articulation.StagePartiallyApplied
		d.Reason = "the new note was added but the earlier note under this key could not be removed: " + retractErr.Error()
		return d
	}
	logging.ContextDebug("Memory op: note key=%s (%d chars)", key, len(value))
	return d
}

func truncateForNote(s string) string {
	if r := []rune(s); len(r) > maxNoteLogRunes {
		return string(r[:maxNoteLogRunes]) + "..."
	}
	return s
}

// maxNoteLogRunes bounds a dropped note's value in the log line.
const maxNoteLogRunes = 80
