package atomjudge

import (
	"context"
	"encoding/json"
	"fmt"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// RelevanceBudgetError refuses the entire uncached pool before any Ask call.
// It is an unavailable judgment, never a relevance answer about an atom.
type RelevanceBudgetError struct {
	Resource        string
	Required, Limit int
}

func (e *RelevanceBudgetError) Error() string {
	return fmt.Sprintf("atom relevance budget exceeded: %s %d > %d; no judgment", e.Resource, e.Required, e.Limit)
}

type relevanceBatch struct {
	atoms     []*prompt.PromptAtom
	questions map[string]typesafe.Question
}

type relevancePlan struct {
	state       map[string]any
	batches     []relevanceBatch
	inputTokens int
	parallel    int
}

// planRelevance accounts for all uncached atoms together, including slots and
// carry. Input is EstimateTokens over each batch's serialized state/questions:
// full guidance, metadata, criteria, question names, and repeated request state.
// This estimate excludes provider-internal framing and transport retries; it is
// a scheduling bound, not measured usage or a monetary cap. No text is cut and
// no partial pool is judged when either budget is exceeded.
func (j RelevanceJudge) planRelevance(ctx context.Context, req prompt.AtomRelevanceRequest, atoms []*prompt.PromptAtom) (relevancePlan, error) {
	var plan relevancePlan
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	defaults := config.DefaultTypeSafeUses()
	limits := []struct {
		name  string
		value *int
		def   int
	}{
		{"atoms", &j.MaxAtoms, defaults.AtomRelevanceMaxAtoms},
		{"estimated input tokens", &j.MaxInputTokens, defaults.AtomRelevanceMaxInputTokens},
		{"concurrent batches", &j.MaxConcurrentBatches, defaults.AtomRelevanceMaxConcurrentBatches},
	}
	for _, l := range limits {
		if *l.value < 0 {
			return plan, fmt.Errorf("atom relevance %s limit must be positive", l.name)
		}
		if *l.value == 0 {
			*l.value = l.def
		}
	}
	if len(atoms) > j.MaxAtoms {
		return plan, &RelevanceBudgetError{Resource: "atoms", Required: len(atoms), Limit: j.MaxAtoms}
	}
	groups, tooLarge := batches(atoms)
	if len(tooLarge) > 0 {
		return plan, tooLarge[0]
	}
	plan.state = requestState(req)
	plan.parallel = j.MaxConcurrentBatches
	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return relevancePlan{}, err
		}
		questions := make(map[string]typesafe.Question, len(group))
		for i, a := range group {
			questions[fmt.Sprintf("atom_%d", i)] = relevanceQuestion(guidanceOf(a))
		}
		payload, err := json.Marshal(struct {
			State     any                          `json:"state"`
			Questions map[string]typesafe.Question `json:"questions"`
		}{plan.state, questions})
		if err != nil {
			return relevancePlan{}, fmt.Errorf("estimate atom relevance input: %w", err)
		}
		plan.inputTokens += prompt.EstimateTokens(string(payload))
		plan.batches = append(plan.batches, relevanceBatch{atoms: group, questions: questions})
	}
	if plan.inputTokens > j.MaxInputTokens {
		return relevancePlan{}, &RelevanceBudgetError{Resource: "estimated input tokens", Required: plan.inputTokens, Limit: j.MaxInputTokens}
	}
	return plan, nil
}
