package atomjudge

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// RelevanceJudge asks, per atom, "Does `guidance` apply to the work in
// `request`?": one Noul per atom, the request in the state, each atom in its
// own question (TypeSafe's re-ranking pattern). It implements
// prompt.AtomRelevanceJudge.
type RelevanceJudge struct {
	Asker Asker
	// Limits apply to the combined uncached pool passed by the selector, not
	// separately to vector slots and carry. Zero uses the config defaults.
	MaxAtoms, MaxInputTokens, MaxConcurrentBatches int
}

var _ prompt.AtomRelevanceJudge = RelevanceJudge{}

// JudgeAtomRelevance preflights the whole uncached pool before sending any
// batch. Workers bound concurrent Ask calls per judgment; the caller's deadline
// covers queueing and retries. Any budget refusal, failed batch, missing answer
// or oversized atom fails the whole judgment, leaving the vector channel alone.
func (j RelevanceJudge) JudgeAtomRelevance(ctx context.Context, req prompt.AtomRelevanceRequest, atoms []*prompt.PromptAtom) (map[string]float64, error) {
	plan, err := j.planRelevance(ctx, req, atoms)
	if err != nil {
		return nil, err
	}
	logging.Get(logging.CategoryJIT).Info("Jev atom relevance plan: %d uncached atoms, %d batches, %d estimated input tokens, max %d concurrent batches per judgment (retries may resend input)",
		len(atoms), len(plan.batches), plan.inputTokens, plan.parallel)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = typesafe.WithOperation(ctx, OpAtomRelevance)
	type part struct {
		out map[string]float64
		err error
	}
	parts := make([]part, len(plan.batches))
	jobs := make(chan int, len(plan.batches))
	for i := range plan.batches {
		jobs <- i
	}
	close(jobs)
	var wg sync.WaitGroup
	for range min(plan.parallel, len(plan.batches)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for gi := range jobs {
				if ctx.Err() != nil {
					return
				}
				batch := plan.batches[gi]
				resp, err := j.Asker.Ask(ctx, plan.state, batch.questions)
				if err != nil {
					parts[gi] = part{err: err}
					cancel()
					return
				}
				out := make(map[string]float64, len(batch.atoms))
				for i, a := range batch.atoms {
					p, err := resp.Noul("atom_" + strconv.Itoa(i))
					if err != nil {
						parts[gi] = part{err: fmt.Errorf("relevance of %s: %w", a.ID, err)}
						cancel()
						return
					}
					out[a.ID] = p
				}
				parts[gi] = part{out: out}
			}
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
	if err := ctx.Err(); err != nil {
		return nil, err
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
		Judge: RelevanceJudge{Asker: a,
			MaxAtoms: uses.AtomRelevanceMaxAtoms, MaxInputTokens: uses.AtomRelevanceMaxInputTokens,
			MaxConcurrentBatches: uses.AtomRelevanceMaxConcurrentBatches},
		Pool:       uses.AtomRelevancePool,
		Timeout:    uses.AtomRelevanceTimeout,
		YesPercent: yes,
		NoPercent:  no,
	}
}
