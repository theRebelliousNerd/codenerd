package deliberation

import (
	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
	"context"
	"fmt"
)

func (s *Service) collectRound(ctx context.Context, t *Thread, n int) (Round, error) {
	frame := Frame{Brief: t.Brief, Request: t.Request, Intent: t.Intent, Phase: "divergent", Answer: t.Answer, CandidateCap: s.Config.WithDefaults().DeliberationCandidateCap}
	if len(t.Rounds) > 0 {
		frame.Phase = "convergent"
		frame.Previous = &t.Rounds[len(t.Rounds)-1]
	}
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "step", Text: fmt.Sprintf("Round %d: %s proposals", n, frame.Phase)})
	rows, err := s.Kernel.Query("deliberation_probe")
	if err != nil {
		return Round{}, err
	}
	for _, fact := range rows {
		if len(fact.Args) == 5 && types.ExtractString(fact.Args[0]) == t.ID {
			frame.ReceiptFacts = append(frame.ReceiptFacts, fact.String())
		}
	}
	r := Round{Number: n}
	raw, e := s.RunShard(ctx, Proposer, frame)
	if e != nil {
		return Round{}, e
	}
	if e = Decode(raw, &r.Proposal); e != nil {
		return Round{}, e
	}
	// Bound the proposal before sending it to the challenger.
	if len(r.Proposal.Candidates) == 0 || len(r.Proposal.Candidates) > frame.CandidateCap || (frame.Phase == "divergent" && len(r.Proposal.Candidates) == 1) {
		return Round{}, fmt.Errorf("proposal exceeds configured candidate bound")
	}
	frame.Candidates = r.Proposal.Candidates
	if len(r.Proposal.Consultations) > frame.CandidateCap {
		return Round{}, fmt.Errorf("consultations exceed candidate bound")
	}
	for i, consultation := range r.Proposal.Consultations {
		if s.Consult == nil {
			return Round{}, fmt.Errorf("expert consultation unavailable")
		}
		allowed, consultErr := AllowCall(s.Kernel, t.ID+"-consult", i)
		if consultErr != nil {
			return Round{}, consultErr
		}
		if !allowed {
			break
		}
		advice, consultErr := s.Consult(ctx, consultation, t.Request)
		observation := ProbeResult{Key: consultation.Specialist, Tool: "consult", Output: advice}
		if consultErr != nil {
			observation.Error = consultErr.Error()
		}
		r.Probes = append(r.Probes, observation)
	}
	frame.Evidence = r.Probes
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "step", Text: fmt.Sprintf("Round %d: challenge %d candidates", n, len(frame.Candidates))})
	raw, e = s.RunShard(ctx, Challenger, frame)
	if e != nil {
		return Round{}, e
	}
	if e = Decode(raw, &r.Challenge); e != nil {
		return Round{}, e
	}
	if e = validateRound(r, frame.CandidateCap); e != nil {
		return Round{}, e
	}
	return r, nil
}
