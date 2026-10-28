package atomjudge

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// RelevanceJudge asks, per atom, "Does `guidance` apply to the work in
// `request`?": one Noul per atom, the request in the state, each atom in its
// own question (TypeSafe's re-ranking pattern). It implements
// prompt.AtomRelevanceJudge.
type RelevanceJudge struct {
	Asker Asker
}

var _ prompt.AtomRelevanceJudge = RelevanceJudge{}

// JudgeAtomRelevance returns each atom's probability of applying. Batches run
// in parallel; any failed batch, missing answer or oversized atom fails the
// whole judgment, so a partial verdict never reorders the flesh.
func (j RelevanceJudge) JudgeAtomRelevance(ctx context.Context, req prompt.AtomRelevanceRequest, atoms []*prompt.PromptAtom) (map[string]float64, error) {
	groups, tooLarge := batches(atoms)
	if len(tooLarge) > 0 {
		return nil, tooLarge[0]
	}
	state := requestState(req)
	ctx = typesafe.WithOperation(ctx, OpAtomRelevance)
	type part struct {
		out map[string]float64
		err error
	}
	parts := make([]part, len(groups))
	var wg sync.WaitGroup
	for gi, group := range groups {
		wg.Add(1)
		go func() {
			defer wg.Done()
			questions := make(map[string]typesafe.Question, len(group))
			for i, a := range group {
				questions["atom_"+strconv.Itoa(i)] = relevanceQuestion(guidanceOf(a))
			}
			resp, err := j.Asker.Ask(ctx, state, questions)
			if err != nil {
				parts[gi] = part{err: err}
				return
			}
			out := make(map[string]float64, len(group))
			for i, a := range group {
				p, err := resp.Noul("atom_" + strconv.Itoa(i))
				if err != nil {
					parts[gi] = part{err: fmt.Errorf("relevance of %s: %w", a.ID, err)}
					return
				}
				out[a.ID] = p
			}
			parts[gi] = part{out: out}
		}()
	}
	wg.Wait()
	all := make(map[string]float64, len(atoms))
	for _, p := range parts {
		if p.err != nil {
			return nil, p.err
		}
		for k, v := range p.out {
			all[k] = v
		}
	}
	return all, nil
}

// RelevanceFor resolves the atom-relevance use from the typesafe section. A
// nil asker (Jev off) or the use switched off gives a zero AtomRelevance,
// which the selector reads as off.
func RelevanceFor(a Asker, sec *config.TypeSafeConfig) prompt.AtomRelevance {
	uses := sec.Uses()
	if a == nil || !uses.AtomRelevance {
		return prompt.AtomRelevance{}
	}
	yes, no := sec.Thresholds()
	return prompt.AtomRelevance{
		Judge:      RelevanceJudge{Asker: a},
		Pool:       uses.AtomRelevancePool,
		Timeout:    uses.AtomRelevanceTimeout,
		YesPercent: yes,
		NoPercent:  no,
	}
}
