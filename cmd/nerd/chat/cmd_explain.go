// Package chat - /explain slash command.
//
// /explain <goal-fact> renders a proof tree for a derived fact using the
// Codeberg mangle-go fork's DerivationRecorder (the "full provenance"
// mode). Unlike /why (which uses a heuristic post-hoc tracer), /explain
// captures derivation events DURING evaluation, so let-transforms and
// aggregations are explained correctly.
//
// Usage:
//
//	/explain next_action(/generate_tool)
//	/explain "permitted(/edit, \"main.go\")"
//
// Side effect: enables kernel provenance recording if it's off, and
// forces a re-evaluation so the recorder captures the current pass.
// Subsequent evaluations also pay the recorder cost until provenance
// is explicitly disabled with /explain-off.
package chat

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/TauCeti/mangle-go/provenance"
)

// renderExplainProofs renders a slice of provenance.ProofNode as a
// markdown-friendly indented tree. Kept compact so the result fits in
// the chat scrollback without overflowing.
func renderExplainProofs(goal string, proofs []*provenance.ProofNode) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## /explain %s\n\n", goal))
	if len(proofs) == 0 {
		sb.WriteString("_No proofs found — either the fact was never derived, or provenance was off when it was derived. Try `/explain` again after the next eval cycle._\n")
		return sb.String()
	}
	for i, p := range proofs {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("### Proof %d\n\n", i+1))
		renderProofNode(&sb, p, 0)
	}
	return sb.String()
}

// renderProofNode walks a proof DAG depth-first. Cycles are not
// possible (the provenance builder breaks them with Partial nodes),
// so we don't track visited IDs.
func renderProofNode(sb *strings.Builder, p *provenance.ProofNode, depth int) {
	if p == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	kindLabel := kindToLabel(p.Kind)
	sb.WriteString(fmt.Sprintf("%s- **%s** `%s`", indent, kindLabel, p.Fact.String()))
	if p.Partial {
		sb.WriteString(" _(partial — max depth or transform skipped)_")
	}
	sb.WriteString("\n")
	if p.Rule != nil && p.Kind != provenance.KindEDB {
		sb.WriteString(fmt.Sprintf("%s  rule: `%s`\n", indent, p.Rule.String()))
	}
	if len(p.Bindings) > 0 {
		sb.WriteString(fmt.Sprintf("%s  bindings:", indent))
		for j, b := range p.Bindings {
			if j > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf(" `%s = %s`", b.Var.Symbol, b.Value.String()))
		}
		sb.WriteString("\n")
	}
	for _, sub := range p.Premises {
		renderProofNode(sb, sub, depth+1)
	}
}

func kindToLabel(k provenance.Kind) string {
	switch k {
	case provenance.KindEDB:
		return "fact"
	case provenance.KindDerived:
		return "derived"
	case provenance.KindAbsence:
		return "absence (negation)"
	case provenance.KindLetRow:
		return "let-row"
	case provenance.KindDoAggregate:
		return "do-aggregate"
	default:
		return "node"
	}
}

// explainCommandReply is the markdown blob returned to the chat
// viewport when the user runs `/explain <goal>`. Pure rendering — the
// caller handles the kernel evaluation and recorder lifecycle.
func explainCommandReply(goal string, proofs []*provenance.ProofNode, err error) Message {
	var content string
	if err != nil {
		content = fmt.Sprintf("## /explain %s\n\n_error: %v_\n", goal, err)
	} else {
		content = renderExplainProofs(goal, proofs)
	}
	return Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	}
}
