// Package chat - /explain slash command.
//
// /explain <goal-fact> renders a proof tree for a derived fact using the
// Codeberg mangle-go fork's DerivationRecorder (the "full provenance"
// mode). Unlike /why (which uses a heuristic post-hoc tracer), /explain
// captures derivation events DURING evaluation, so let-transforms and
// aggregations are explained correctly.
//
// The answer is a core.ExplanationEnvelope: bounded by the `explain` config
// section, stamped with the program digest and recorder generation it
// describes, and carrying each partial node's reason.
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

	"codenerd/internal/config"
	"codenerd/internal/core"

	"codeberg.org/TauCeti/mangle-go/provenance"
)

// explainOptions are the configured /explain bounds.
func explainOptions(cfg *config.UserConfig) core.ExplainOptions {
	e := cfg.GetExplainConfig()
	return core.ExplainOptions{
		MaxProofs: e.MaxProofs,
		MaxDepth:  e.MaxDepth,
		MaxNodes:  e.MaxNodes,
		MaxEdges:  e.MaxEdges,
		MaxBytes:  e.MaxBytes,
	}
}

// renderExplainProofs renders a slice of provenance.ProofNode as a
// markdown-friendly indented tree. Kept compact so the result fits in
// the chat scrollback without overflowing. status carries each node's
// partial reason when the envelope knows it.
func renderExplainProofs(goal string, proofs []*provenance.ProofNode, status map[string]core.ProofNodeStatus) string {
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
		renderProofNode(&sb, p, 0, status)
	}
	return sb.String()
}

// renderProofNode walks a proof DAG depth-first. Cycles are not
// possible (the provenance builder breaks them with Partial nodes),
// so we don't track visited IDs.
func renderProofNode(sb *strings.Builder, p *provenance.ProofNode, depth int, status map[string]core.ProofNodeStatus) {
	if p == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	kindLabel := kindToLabel(p.Kind)
	sb.WriteString(fmt.Sprintf("%s- **%s** `%s`", indent, kindLabel, p.Fact.String()))
	if p.Partial {
		sb.WriteString(" _(" + partialLabel(status[p.ID].Reason) + ")_")
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
		renderProofNode(sb, sub, depth+1, status)
	}
}

// partialLabel says why a node is partial. A partial node is an unexpanded
// step, not a wrong conclusion.
func partialLabel(reason string) string {
	switch reason {
	case core.PartialDepthLimit:
		return "partial — depth limit reached"
	case core.PartialPremise:
		return "partial — a premise is partial"
	case core.PartialUnrepresented:
		return "partial — a step the explainer does not represent (temporal or builtin)"
	case core.PartialHostTruncated:
		return "partial — cut by the explain budget"
	default:
		return "partial — max depth or transform skipped"
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
func explainCommandReply(goal string, env *core.ExplanationEnvelope, err error) Message {
	var content string
	switch {
	case err != nil:
		content = fmt.Sprintf("## /explain %s\n\n_error: %v_\n", goal, err)
	case env == nil:
		content = renderExplainProofs(goal, nil, nil)
	case !env.RecordedThisEpoch:
		content = fmt.Sprintf("## /explain %s\n\n_Provenance is armed but no evaluation has been recorded yet — nothing can be said about this goal until the next evaluation._\n", goal)
	default:
		content = renderExplainProofs(goal, env.Proofs, env.Nodes) + explainEnvelopeFooter(env)
	}
	return Message{
		Role:    "assistant",
		Content: content,
		Time:    time.Now(),
	}
}

// explainEnvelopeFooter names the epoch the answer describes and what the
// budget cut, so an answer read later cannot pass for the current world.
func explainEnvelopeFooter(env *core.ExplanationEnvelope) string {
	var sb strings.Builder
	digest := env.ProgramDigest
	if len(digest) > 12 {
		digest = digest[:12]
	}
	fmt.Fprintf(&sb, "\n_epoch: program %s, recorder generation %d; %d proof facts (%d bytes)_\n",
		digest, env.RecorderGeneration, env.ProofFactCount, env.ProofFactBytes)
	if env.Truncated {
		fmt.Fprintf(&sb, "_truncated: %s_\n", strings.Join(env.TruncationReasons, "; "))
	}
	return sb.String()
}
