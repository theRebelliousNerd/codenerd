package articulation

import (
	"fmt"
	"strings"

	"codenerd/internal/core"
)

// AdmissionOutcome is how the response processor classified one model reply.
// It is the host's reading of the reply, not the model's claim about itself.
type AdmissionOutcome string

const (
	// AdmissionAdmitted: exactly one envelope was selected and its controls are
	// proposals for the caller to apply.
	AdmissionAdmitted AdmissionOutcome = "admitted"
	// AdmissionPlainOnly: the reply carried no envelope. It is display text,
	// and no control may be taken from it.
	AdmissionPlainOnly AdmissionOutcome = "plain_only"
	// AdmissionMalformed: the reply began an envelope it never finished.
	// Whatever surface could be salvaged is display text only.
	AdmissionMalformed AdmissionOutcome = "malformed"
	// AdmissionAmbiguous: the reply held more than one reading of its own
	// envelope (a repeated key, or two candidate envelopes). Nothing was
	// selected and no control may be taken from it.
	AdmissionAmbiguous AdmissionOutcome = "ambiguous"
)

// ControlKind names the channel a control arrived on.
type ControlKind string

const (
	ControlMangleUpdate    ControlKind = "mangle_update"
	ControlMemoryOperation ControlKind = "memory_operation"
)

// ControlStage is what actually became of one proposed control.
type ControlStage string

const (
	// StageRejectedParse: the parser removed it before any policy saw it
	// (empty, oversized, malformed, or past a list limit).
	StageRejectedParse ControlStage = "rejected_parse"
	// StageRejectedPolicy: the kernel's observation policy refused it.
	StageRejectedPolicy ControlStage = "rejected_policy"
	// StageUnsupported: the consumer has no way to apply this control.
	StageUnsupported ControlStage = "unsupported"
	// StageFailed: it was admitted and the store or kernel refused the write.
	StageFailed ControlStage = "failed"
	// StagePartiallyApplied: a multi-step control completed some of its steps.
	StagePartiallyApplied ControlStage = "partially_applied"
	// StageUnresolved: the control named something the host could not resolve
	// to exactly one record (no match, or several), so it changed nothing.
	StageUnresolved ControlStage = "unresolved"
	// StageApplied: the write landed.
	StageApplied ControlStage = "applied"
)

// ControlDisposition is the host-owned record of one proposed control. The
// model proposes controls; only the host writes a disposition, so a model can
// not claim "applied" for its own request. Index is the control's position in
// the list the model sent, so two equal operations at different positions are
// two controls.
type ControlDisposition struct {
	Kind   ControlKind  `json:"kind"`
	Index  int          `json:"index"`
	Stage  ControlStage `json:"stage"`
	Detail string       `json:"detail,omitempty"` // what the control was, short
	Reason string       `json:"reason,omitempty"` // why it did not take effect
	// Receipt is what the host did, in the host's words and naming the memory
	// domain it addressed. A person is told it for a control they asked for
	// (a forget), applied or not, so "done" is never read as "everywhere".
	Receipt string `json:"receipt,omitempty"`
}

// Took reports whether the control took full effect.
func (d ControlDisposition) Took() bool { return d.Stage == StageApplied }

// Notice is the sentence a person reads about a control that did not take
// full effect, or "" for one that did. It is host text, written from the
// recorded state, never from what the model said happened.
func (d ControlDisposition) Notice() string {
	if d.Took() {
		return ""
	}
	what := string(d.Kind)
	switch d.Kind {
	case ControlMemoryOperation:
		what = "memory operation"
	case ControlMangleUpdate:
		what = "kernel update"
	}
	if d.Detail != "" {
		what += " " + d.Detail
	}
	state := map[ControlStage]string{
		StageRejectedParse:    "was removed before it was considered",
		StageRejectedPolicy:   "was refused by policy",
		StageUnsupported:      "is not supported here",
		StageFailed:           "failed",
		StagePartiallyApplied: "was only partly applied",
		StageUnresolved:       "changed nothing",
	}[d.Stage]
	if state == "" {
		state = "did not take effect (" + string(d.Stage) + ")"
	}
	line := fmt.Sprintf("[Control] %s #%d %s", what, d.Index+1, state)
	if d.Reason != "" {
		line += ": " + d.Reason
	}
	return line
}

// DescribeMemoryOperation is the short Detail for a memory operation: the
// operation and its key, never its value (a preference can be private).
func DescribeMemoryOperation(op MemoryOperation) string {
	key := strings.TrimSpace(op.Key)
	if key == "" {
		return op.Op
	}
	return op.Op + " " + key
}

// MangleUpdateDispositions accounts for every update a consumer filtered with
// core.FilterMangleUpdates and then tried to assert: one disposition per
// position in updates. A blocked update was refused by policy, a blank one was
// never an update, and each remaining update was applied unless assertErr says
// its write failed. assertErr is asked by the update's ordinal among the
// admitted ones (the order of the facts the filter returned); nil means every
// admitted update was written.
//
// origins maps each position in updates to its position in the list the model
// sent (ProcessedLLMResponse.MangleOrigins), so a refusal here and a parser
// removal there are numbered in one space. Nil means updates is the model's
// own list.
func MangleUpdateDispositions(updates []string, origins []int, blocked []core.MangleUpdateBlock, assertErr func(admitted int) error) []ControlDisposition {
	refused := make(map[int]string, len(blocked))
	for _, b := range blocked {
		refused[b.Index] = b.Reason
	}
	out := make([]ControlDisposition, 0, len(updates))
	admitted := 0
	for i, update := range updates {
		index := i
		if i < len(origins) {
			index = origins[i]
		}
		d := ControlDisposition{Kind: ControlMangleUpdate, Index: index, Detail: updateDetail(update), Stage: StageApplied}
		switch reason, isBlocked := refused[i]; {
		case isBlocked:
			d.Stage, d.Reason = StageRejectedPolicy, reason
		case strings.TrimSpace(update) == "":
			d.Stage, d.Reason = StageRejectedParse, "empty update"
		default:
			if assertErr != nil {
				if err := assertErr(admitted); err != nil {
					d.Stage, d.Reason = StageFailed, "the kernel refused the write: "+err.Error()
				}
			}
			admitted++
		}
		out = append(out, d)
	}
	return out
}
