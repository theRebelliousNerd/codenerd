package deliberation

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
	"encoding/json"
	"fmt"
)

func (s *Service) assertRound(id string, r *Round, add func(types.Fact) error) error {
	for rank, c := range r.Proposal.Candidates {
		if err := add(types.Fact{Predicate: "deliberation_brief", Args: []any{id, int64(r.Number), types.MangleString(c.ID), types.MangleString(c.Brief), types.MangleString(c.SuccessTest)}}); err != nil {
			return err
		}
		for _, citation := range c.Evidence {
			if err := add(types.Fact{Predicate: "deliberation_citation", Args: []any{id, int64(r.Number), types.MangleString(c.ID), types.MangleString(citation)}}); err != nil {
				return err
			}
		}
		if err := add(types.Fact{Predicate: "deliberation_candidate", Args: []any{id, int64(r.Number), types.MangleString(c.ID), int64(rank)}}); err != nil {
			return err
		}
	}
	for _, a := range r.Challenge.Assessments {
		if a.Success {
			if err := add(types.Fact{Predicate: "deliberation_success", Args: []any{id, int64(r.Number), types.MangleString(a.CandidateID)}}); err != nil {
				return err
			}
		}
		for _, item := range a.Items {
			if err := add(types.Fact{Predicate: "deliberation_item", Args: []any{id, int64(r.Number), types.MangleString(a.CandidateID), types.MangleString(item.Key), types.MangleAtom("/" + item.Severity), types.MangleAtom("/" + item.Kind)}}); err != nil {
				return err
			}
		}
	}
	if err := add(types.Fact{Predicate: "deliberation_recommended", Args: []any{id, int64(r.Number), types.MangleString(r.Challenge.Recommendation)}}); err != nil {
		return err
	}
	return add(types.Fact{Predicate: "deliberation_round", Args: []any{id, int64(r.Number)}})
}

func (s *Service) prove(f types.Fact, r *Round) error {
	p, ok := s.Kernel.(interface {
		ExplainEnvelope(string, core.ExplainOptions) (*core.ExplanationEnvelope, error)
	})
	if !ok {
		return fmt.Errorf("deliberation requires kernel proof support")
	}
	goal, err := f.ToAtom()
	if err != nil {
		return err
	}
	env, err := p.ExplainEnvelope(goal.String(), s.Explain)
	if err != nil {
		return err
	}
	if !env.RecordedThisEpoch || len(env.Proofs) == 0 {
		return fmt.Errorf("deliberation decision has no recorded proof")
	}
	// Persist the bounded proof tree, including epoch and partial flags. Never
	// serialize the live store or feed a proof back as execution authority.
	b, err := json.Marshal(struct {
		Goal, ProgramDigest string
		Generation          uint64
		Recorded, Truncated bool
		Proofs              any
	}{env.Goal, env.ProgramDigest, env.RecorderGeneration, env.RecordedThisEpoch, env.Truncated, env.Proofs})
	if err != nil {
		return err
	}
	r.Proof = b
	return nil
}
