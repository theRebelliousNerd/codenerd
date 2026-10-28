package chat

import (
	"codenerd/internal/articulation"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/perception"
)

// appliedControls is what the chat turn did with a model reply's controls: the
// notices to show, the controls that took effect (the only ones history may
// record as asked-for), and the ones that did not (the parser's removals and
// every memory operation a store refused), which history records as such.
type appliedControls struct {
	Warnings        []string
	RecordedUpdates []string
	RecordedMemory  []articulation.MemoryOperation
	NotApplied      []articulation.ControlDisposition
}

// memoryOutcome is the result of applying one packet's memory operations.
type memoryOutcome struct {
	applied    []articulation.MemoryOperation
	notApplied []articulation.ControlDisposition
	warnings   []string
}

// applyMemoryOperations applies the operations through the shared scoped
// implementation and sorts them by what happened. A failure becomes a
// user-visible warning built from the recorded state ("[Control] memory
// operation ... failed: ..."), because the model's own sentence about having
// remembered something is not evidence that the store took it.
func (m Model) applyMemoryOperations(ops []articulation.MemoryOperation) memoryOutcome {
	var out memoryOutcome
	if len(ops) == 0 {
		return out
	}
	for i, d := range ctxcompress.ApplyMemoryOperations(m.memoryKernel(), m.memoryStore(), ops) {
		if d.Took() {
			out.applied = append(out.applied, ops[i])
			continue
		}
		out.notApplied = append(out.notApplied, d)
		out.warnings = append(out.warnings, d.Notice())
	}
	return out
}

// memoryKernel and memoryStore are the two halves a memory operation lands in,
// or nil interfaces: a nil pointer in an interface is not a nil interface, and
// the operation reports the missing half instead of calling through it.
func (m Model) memoryKernel() ctxcompress.MemoryKernel {
	if k, ok := m.kernel.(ctxcompress.MemoryKernel); ok && m.kernel != nil {
		return k
	}
	return nil
}

func (m Model) memoryStore() ctxcompress.MemoryStore {
	if m.localDB != nil {
		return m.localDB
	}
	return nil
}

// compressionControlPacket is the sanitized packet the compressor records for
// a completed turn. The classification carries the user's constraint (history
// used to keep category, verb and target only), and the controls are the ones
// that took effect: what did not rides beside it as Turn.ControlOutcomes.
func compressionControlPacket(intent perception.Intent, applied appliedControls) *articulation.ControlPacket {
	return &articulation.ControlPacket{
		IntentClassification: articulation.IntentClassification{
			Category:   intent.Category,
			Verb:       intent.Verb,
			Target:     intent.Target,
			Constraint: intent.Constraint,
		},
		MemoryOperations: applied.RecordedMemory,
		MangleUpdates:    applied.RecordedUpdates,
	}
}
