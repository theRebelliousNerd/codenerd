package context

import (
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
	StoreFact(predicate string, args []any, factType string, priority int) error
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
// "forget" drops the session note under the key, and nothing else. It used to
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
func ApplyMemoryOperation(kernel MemoryKernel, store MemoryStore, op articulation.MemoryOperation) {
	switch op.Op {
	case "promote_to_long_term":
		logging.ContextDebug("Memory op: promote_to_long_term key=%s", op.Key)
		if store == nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op promote_to_long_term FAILED, no long-term store on this path: key=%s", op.Key)
			return
		}
		if err := store.StoreFact(op.Key, []any{op.Value}, "preference", 10); err != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op promote_to_long_term FAILED, the preference is not persisted: key=%s: %v", op.Key, err)
		}
	case "forget":
		logging.ContextDebug("Memory op: forget key=%s", op.Key)
		if kernel == nil {
			logging.Get(logging.CategoryContext).Warn("Memory op forget FAILED, no kernel attached: key=%s", op.Key)
			return
		}
		if err := kernel.RetractFact(types.Fact{Predicate: sessionNotePredicate, Args: []any{op.Key}}); err != nil {
			logging.Get(logging.CategoryContext).Warn("Memory op forget could not drop the session note: key=%s: %v", op.Key, err)
		}
	case "store_vector":
		logging.ContextDebug("Memory op: store_vector key=%s", op.Key)
		if store == nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op store_vector FAILED, no vector store on this path: key=%s", op.Key)
			return
		}
		if err := store.StoreVector(op.Value, map[string]any{"key": op.Key}); err != nil {
			logging.Get(logging.CategoryContext).Warn(
				"Memory op store_vector FAILED, the content is not recallable: key=%s: %v", op.Key, err)
		}
	case "note":
		recordSessionNote(kernel, op.Key, op.Value)
	default:
		logging.Get(logging.CategoryContext).Warn(
			"Unhandled memory op %q, dropping: key=%s -- the protocol schema admits an operation this switch does not", op.Op, op.Key)
	}
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
func recordSessionNote(kernel MemoryKernel, key, value string) {
	key = strings.TrimSpace(key)
	if key == "" {
		logging.Get(logging.CategoryContext).Warn("Memory op note has no key; dropping: value=%q", truncateForNote(value))
		return
	}
	if kernel == nil {
		logging.Get(logging.CategoryContext).Warn("Memory op note FAILED, no kernel attached: key=%s", key)
		return
	}
	// RetractFact matches on the first argument: the key.
	if err := kernel.RetractFact(types.Fact{Predicate: sessionNotePredicate, Args: []any{key}}); err != nil {
		logging.Get(logging.CategoryContext).Warn("Memory op note could not replace the earlier note: key=%s: %v", key, err)
	}
	if err := kernel.Assert(types.Fact{Predicate: sessionNotePredicate, Args: []any{key, value}}); err != nil {
		logging.Get(logging.CategoryContext).Warn(
			"Memory op note FAILED, the observation is not in session context: key=%s: %v", key, err)
		return
	}
	logging.ContextDebug("Memory op: note key=%s (%d chars)", key, len(value))
}

func truncateForNote(s string) string {
	if r := []rune(s); len(r) > maxNoteLogRunes {
		return string(r[:maxNoteLogRunes]) + "..."
	}
	return s
}

// maxNoteLogRunes bounds a dropped note's value in the log line.
const maxNoteLogRunes = 80
