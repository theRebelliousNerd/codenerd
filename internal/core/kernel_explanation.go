package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

// =============================================================================
// EXPLANATION ENVELOPE: PROOF DATA WITHOUT OVERSTATING PROOF
// =============================================================================
//
// Explain returns proof trees; provenance.EmitFacts turns them into
// proves/premise/edb_leaf/absence_leaf/... facts a second program can read.
// Two things are lost on the way, and both matter to a consumer:
//
//   - which evaluation the proof is about. The recorder is reset every pass,
//     so a "why?" answered after the next evaluation silently describes a
//     different world. The envelope stamps the program digest, the source
//     revision and the recorder generation, and says whether the current
//     epoch was recorded at all (EnableProvenance arms an empty recorder that
//     no pass has filled yet).
//   - ProofNode.Partial. EmitFacts does not export it, though the printer
//     shows it. A partial node is not "the conclusion is wrong"; it is an
//     unexpanded step. The envelope keeps the flag per node, with the reason
//     when the host can tell it (depth limit, a partial premise, a step the
//     explainer does not represent, or the host's own truncation), and emits
//     it as proof_partial(ProofID, Reason) beside EmitFacts' schema.
//
// Extraction is bounded by nodes, premise edges and rendered bytes of the
// emitted proof facts (config.ExplainConfig). Bounding extraction is not
// bounding recording, and the envelope says which happened: Truncated and
// TruncationReasons describe what was cut, never what was not recorded.
//
// The proof facts live in their own read-only store. They are evidence about
// the execution kernel, never facts in it: nothing here asserts into the
// kernel, so no explanation can grant itself a permission.

// Partial reasons, as name constants in proof_partial facts.
const (
	PartialDepthLimit      = "/depth_limit"
	PartialPremise         = "/partial_premise"
	PartialUnrepresented   = "/unrepresented_step"
	PartialHostTruncated   = "/truncated"
	partialIDPrefix        = "/proof/partial/"
	explanationMaxRetries  = 8
	explanationMetaFactKey = "explanation"
)

// ProofNodeStatus is the completeness of one node of the bounded proofs.
type ProofNodeStatus struct {
	Partial bool
	Reason  string // one of the Partial* constants; "" when complete
}

// ExplanationEnvelope is a bounded, epoch-stamped answer to "why goal?".
type ExplanationEnvelope struct {
	Goal               string
	SourceRevision     string // vcs.revision of this binary, "" when unknown
	ProgramDigest      string // sha256 of the analyzed program text
	RecorderGeneration uint64 // which evaluation's recorder answered
	RecordedThisEpoch  bool   // false: provenance armed but no pass recorded yet
	Limits             ExplainOptions
	Truncated          bool
	TruncationReasons  []string
	Proofs             []*provenance.ProofNode // bounded copies; nil premises were cut
	Nodes              map[string]ProofNodeStatus
	ProofFacts         factstore.ReadOnlyFactStore
	ProofFactCount     int
	ProofFactBytes     int
}

// ExplainEnvelope answers "why goal?" for the current evaluation, bounded by
// opts. With provenance armed but no pass recorded yet, it returns an
// envelope with RecordedThisEpoch false and no proofs rather than an error:
// "not recorded" is not "not derivable".
func (k *RealKernel) ExplainEnvelope(goal string, opts ExplainOptions) (*ExplanationEnvelope, error) {
	if err := k.ensureEvaluated(); err != nil {
		return nil, fmt.Errorf("ExplainEnvelope: evaluation failed: %w", err)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.proofRecorder == nil {
		return nil, ErrProvenanceDisabled
	}
	env := &ExplanationEnvelope{
		Goal:           goal,
		SourceRevision: buildRevision(),
		ProgramDigest:  k.programDigestLocked(),
		Limits:         opts,
	}
	if k.proofRecorder != k.recorderEpoch.seen {
		k.recorderEpoch.seen = k.proofRecorder
		k.recorderEpoch.generation++
	}
	env.RecorderGeneration = k.recorderEpoch.generation
	env.RecordedThisEpoch = k.proofRecorder != k.recorderEpoch.armed
	if !env.RecordedThisEpoch {
		return env, nil
	}
	proofs, err := k.explainLocked(goal, opts)
	if errors.Is(err, provenance.ErrNoProof) {
		return env, nil
	}
	if err != nil {
		return nil, err
	}
	if err := env.bound(proofs); err != nil {
		return nil, err
	}
	return env, nil
}

// bound copies proofs within the node/edge limits, emits their facts, and
// halves the node budget until the emitted facts also fit MaxBytes.
func (env *ExplanationEnvelope) bound(proofs []*provenance.ProofNode) error {
	nodeBudget := env.Limits.MaxNodes
	for attempt := 0; ; attempt++ {
		b := proofBounder{maxNodes: nodeBudget, maxEdges: env.Limits.MaxEdges, copies: map[string]*provenance.ProofNode{}, status: map[string]ProofNodeStatus{}}
		bounded := make([]*provenance.ProofNode, 0, len(proofs))
		for _, p := range proofs {
			if c := b.copy(p); c != nil {
				bounded = append(bounded, c)
			}
		}
		store, count, size, err := emitProofFacts(env, bounded, b.status)
		if err != nil {
			return err
		}
		if env.Limits.MaxBytes > 0 && size > env.Limits.MaxBytes && len(b.copies) > 1 && attempt < explanationMaxRetries {
			nodeBudget = len(b.copies) / 2
			continue
		}
		env.Proofs, env.Nodes = bounded, b.status
		env.ProofFacts, env.ProofFactCount, env.ProofFactBytes = store, count, size
		env.Truncated = b.cut
		if b.cutNodes {
			env.TruncationReasons = append(env.TruncationReasons, fmt.Sprintf("node budget %d reached", nodeBudget))
		}
		if b.cutEdges {
			env.TruncationReasons = append(env.TruncationReasons, fmt.Sprintf("edge budget %d reached", env.Limits.MaxEdges))
		}
		if nodeBudget != env.Limits.MaxNodes {
			env.TruncationReasons = append(env.TruncationReasons, fmt.Sprintf("byte budget %d: nodes reduced to %d", env.Limits.MaxBytes, nodeBudget))
		}
		if env.Limits.MaxBytes > 0 && size > env.Limits.MaxBytes {
			env.Truncated = true
			env.TruncationReasons = append(env.TruncationReasons, fmt.Sprintf("%d proof-fact bytes still exceed the byte budget %d", size, env.Limits.MaxBytes))
		}
		return nil
	}
}

// proofBounder copies a proof DAG (shared sub-proofs stay shared) until a
// budget is reached. A premise it cannot afford is left nil and its parent is
// marked partial with PartialHostTruncated.
type proofBounder struct {
	maxNodes, maxEdges int
	edges              int
	copies             map[string]*provenance.ProofNode
	status             map[string]ProofNodeStatus
	cut                bool
	cutNodes, cutEdges bool
}

func (b *proofBounder) copy(n *provenance.ProofNode) *provenance.ProofNode {
	if n == nil {
		return nil
	}
	if c, ok := b.copies[n.ID]; ok {
		return c
	}
	if b.maxNodes > 0 && len(b.copies) >= b.maxNodes {
		b.cut, b.cutNodes = true, true
		return nil
	}
	c := *n
	c.Premises = make([]*provenance.ProofNode, len(n.Premises))
	b.copies[n.ID] = &c
	b.status[n.ID] = ProofNodeStatus{Partial: n.Partial, Reason: partialReason(n)}
	for i, sub := range n.Premises {
		if sub == nil {
			continue
		}
		if b.maxEdges > 0 && b.edges >= b.maxEdges {
			b.cut, b.cutEdges = true, true
			b.markTruncated(&c)
			break
		}
		b.edges++
		if c.Premises[i] = b.copy(sub); c.Premises[i] == nil {
			b.edges--
			b.markTruncated(&c)
		}
	}
	return &c
}

func (b *proofBounder) markTruncated(c *provenance.ProofNode) {
	c.Partial = true
	b.status[c.ID] = ProofNodeStatus{Partial: true, Reason: PartialHostTruncated}
}

// partialReason says why the explainer left n partial, when it can tell.
func partialReason(n *provenance.ProofNode) string {
	if !n.Partial {
		return ""
	}
	if strings.HasPrefix(n.ID, partialIDPrefix) {
		return PartialDepthLimit
	}
	for _, sub := range n.Premises {
		if sub != nil && sub.Partial {
			return PartialPremise
		}
	}
	return PartialUnrepresented
}

// emitProofFacts writes the bounded proofs into a fresh read-only store:
// EmitFacts' schema, plus proof_partial(ProofID, Reason) for every partial
// node and one explanation(Goal, ProgramDigest, Generation) row naming the
// epoch. It returns the store, its fact count and rendered size.
func emitProofFacts(env *ExplanationEnvelope, proofs []*provenance.ProofNode, status map[string]ProofNodeStatus) (factstore.ReadOnlyFactStore, int, int, error) {
	store := factstore.NewSimpleInMemoryStore()
	if err := provenance.EmitFacts(proofs, store); err != nil {
		return nil, 0, 0, fmt.Errorf("emit proof facts: %w", err)
	}
	for id, st := range status {
		if !st.Partial {
			continue
		}
		pid, err := ast.Name(id)
		if err != nil {
			return nil, 0, 0, err
		}
		reason, err := ast.Name(st.Reason)
		if err != nil {
			return nil, 0, 0, err
		}
		store.Add(ast.NewAtom("proof_partial", pid, reason))
	}
	store.Add(ast.NewAtom(explanationMetaFactKey, ast.String(env.Goal), ast.String(env.ProgramDigest),
		ast.Number(int64(env.RecorderGeneration))))
	count, size := 0, 0
	for _, sym := range store.ListPredicates() {
		_ = store.GetFacts(ast.NewQuery(sym), func(a ast.Atom) error {
			count++
			size += len(a.String())
			return nil
		})
	}
	return store, count, size, nil
}

// programDigestLocked is the sha256 of the program text programInfo was
// analyzed from, computed once per analyzed program. Call holding k.mu.
func (k *RealKernel) programDigestLocked() string {
	if k.programInfo == nil {
		return ""
	}
	if k.digest.info == k.programInfo && k.digest.hex != "" {
		return k.digest.hex
	}
	var sb strings.Builder
	k.writeProgramLocked(&sb)
	sum := sha256.Sum256([]byte(sb.String()))
	k.digest.info = k.programInfo
	k.digest.hex = hex.EncodeToString(sum[:])
	return k.digest.hex
}

// ExplainEnvelope asks the shards a Query of goal's predicate reads, and
// returns the first that recorded a proof; otherwise the first shard's
// envelope, which says why there is none.
func (c *CortexKernel) ExplainEnvelope(goal string, opts ExplainOptions) (*ExplanationEnvelope, error) {
	c.adoptionMu.RLock()
	defer c.adoptionMu.RUnlock()
	shards, _, err := c.readShards(goal)
	if err != nil {
		return nil, err
	}
	var first *ExplanationEnvelope
	for _, s := range shards {
		env, err := s.kernel.ExplainEnvelope(goal, opts)
		if err != nil {
			return nil, err
		}
		if len(env.Proofs) > 0 {
			return env, nil
		}
		if first == nil {
			first = env
		}
	}
	if first == nil {
		return nil, provenance.ErrNoProof
	}
	return first, nil
}
