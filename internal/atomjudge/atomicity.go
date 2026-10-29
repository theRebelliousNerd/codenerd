package atomjudge

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// Atomicity: what an atom costs on every call it is selected for, read by
// Jev. Three Scores about one atom's text (how much of it a capable engineer
// already knows, how many separable instructions it holds, how much of it
// gives no instruction) and one Noul about a pair (does one atom already give
// every instruction the other gives). The corpus audit
// (cmd/tools/prompt_builder -audit-atomicity) asks them of every embedded
// atom; every generator that writes an atom asks them before storing it
// (admission.go). Nothing here decides.
//
// Wording probed live on 2026-10-28 against twelve atoms with known answers
// (generic debugging and review checklists, CodeDOM tool guidance, the
// Piggyback protocol, a Mangle reference): the known Score put generic
// checklists at 3.5-3.9 and tool or protocol guidance at 0.8-1.3, where a
// yes/no wording ("would a strong coding model already follow it") left the
// generic checklists unsettled between 0.4 and 0.6.

// OpAtomAtomicity is the usage-tracker operation for these questions.
const OpAtomAtomicity = "jev_atom_atomicity"

// Question names.
const (
	QuestionKnown    = "known"
	QuestionConcerns = "concerns"
	QuestionPadding  = "padding"
	questionCovers   = "covers_"
)

// KnownLevels are the known Score's levels, lowest first.
var KnownLevels = []string{
	"None of it: every instruction is about this agent, its tools or formats, or a fact that is not common knowledge.",
	"A little of it: most instructions are specific to this agent or not common knowledge.",
	"About half of it.",
	"Most of it: only one or two instructions are specific to this agent or not common knowledge.",
	"All of it: every instruction is ordinary engineering practice found in any style guide or review checklist.",
}

// ConcernLevels are the concerns Score's levels, lowest first.
var ConcernLevels = []string{
	"One instruction; everything else explains or illustrates it.",
	"Two separate instructions.",
	"Three to five separate instructions.",
	"Six or more separate instructions.",
}

// PaddingLevels are the padding Score's levels, lowest first.
var PaddingLevels = []string{
	"None: every sentence states a rule, its condition, or an example of it.",
	"A sentence or two.",
	"About a quarter of the text.",
	"Half or more of the text.",
}

func levelsOf(levels []string) []any {
	out := make([]any, len(levels))
	for i, l := range levels {
		out[i] = l
	}
	return out
}

// atomicityQuestions ask about `guidance`, the one atom in the state.
func atomicityQuestions() map[string]typesafe.Question {
	return map[string]typesafe.Question{
		QuestionKnown: typesafe.Score{
			Instructions: "How much of `guidance` would an experienced software engineer already know and follow without being told?",
			Criteria:     levelsOf(KnownLevels),
		},
		QuestionConcerns: typesafe.Score{
			Instructions: "How many separate instructions does `guidance` give that could each be needed on a task where the others are not?",
			Criteria:     levelsOf(ConcernLevels),
		},
		QuestionPadding: typesafe.Score{
			Instructions: "How much of `guidance` is text that gives no instruction, such as an introduction, motivation, a slogan, a restatement or hedging?",
			Criteria:     levelsOf(PaddingLevels),
		},
	}
}

// coversQuestion asks whether the named existing atom already gives every
// instruction in `guidance`. Scope (agent, language) is left out on purpose:
// the question is about the instructions; whether the two atoms reach the
// same calls is a selector comparison code makes.
func coversQuestion(existingKey string) typesafe.Noul {
	return typesafe.Noul{
		Instructions: "Does `" + existingKey + "` already give every instruction that `guidance` gives?",
		Criteria: &typesafe.NoulCriteria{
			True:  "An agent following `" + existingKey + "` would already do everything `guidance` tells it, even if the wording or examples differ.",
			False: "`guidance` gives at least one instruction `" + existingKey + "` does not: another rule, condition, tool, format or fact.",
		},
	}
}

// atomicityAgent is the one fact a person needs to read an atom as prompt text.
const atomicityAgent = "codeNERD, a coding agent. Each guidance block is text placed in the system prompt of its language model when it applies to the work."

// Atomicity is Jev's reading of one atom. Each Score is the
// probability-weighted position on its levels (0 = the first level).
type Atomicity struct {
	Known    float64 `json:"known"`
	Concerns float64 `json:"concerns"`
	Padding  float64 `json:"padding"`
	// Covered holds, per compared atom ID, the probability that it already
	// gives every instruction the judged atom gives.
	Covered map[string]float64 `json:"covered,omitempty"`
	Model   string             `json:"model"`
}

// AtomicityRequest builds the state and questions for one atom, compared with
// the given existing atoms. The judged text is the whole atom: an atom too
// large for one question is an error, never cut to fit.
func AtomicityRequest(candidate *prompt.PromptAtom, compare []*prompt.PromptAtom) (map[string]any, map[string]typesafe.Question, error) {
	if candidate == nil || strings.TrimSpace(candidate.Content) == "" {
		return nil, nil, fmt.Errorf("atomjudge: atomicity needs an atom with content")
	}
	tokens := prompt.EstimateTokens(candidate.Content)
	for _, a := range compare {
		tokens += prompt.EstimateTokens(a.Content)
	}
	if tokens > questionTokenLimit {
		return nil, nil, ErrTooLarge{AtomID: candidate.ID, Tokens: tokens}
	}
	state := map[string]any{"agent": atomicityAgent, "guidance": textOf(candidate)}
	qs := atomicityQuestions()
	for i, a := range compare {
		key := fmt.Sprintf("existing_%d", i)
		state[key] = textOf(a)
		qs[fmt.Sprintf("%s%d", questionCovers, i)] = coversQuestion(key)
	}
	return state, qs, nil
}

// textOf is an atom as the atomicity questions read it: its text alone. The
// importance and relevance questions carry scope because their answer
// depends on it; these do not.
func textOf(a *prompt.PromptAtom) map[string]any {
	return map[string]any{"text": a.Content}
}

// JudgeAtomicity asks the atomicity questions about one atom. A failed call
// or a missing answer is an error, never a reading.
func JudgeAtomicity(ctx context.Context, asker Asker, candidate *prompt.PromptAtom, compare []*prompt.PromptAtom) (*Atomicity, error) {
	if asker == nil {
		return nil, fmt.Errorf("atomjudge: no Jev client")
	}
	state, qs, err := AtomicityRequest(candidate, compare)
	if err != nil {
		return nil, err
	}
	resp, err := asker.Ask(typesafe.WithOperation(ctx, OpAtomAtomicity), state, qs)
	if err != nil {
		return nil, err
	}
	out := &Atomicity{Model: resp.Model}
	for name, dst := range map[string]*float64{QuestionKnown: &out.Known, QuestionConcerns: &out.Concerns, QuestionPadding: &out.Padding} {
		ans, err := resp.Score(name)
		if err != nil {
			return nil, fmt.Errorf("atomicity of %s: %w", candidate.ID, err)
		}
		*dst = ans.Score
	}
	if len(compare) > 0 {
		out.Covered = make(map[string]float64, len(compare))
		for i, a := range compare {
			p, err := resp.Noul(fmt.Sprintf("%s%d", questionCovers, i))
			if err != nil {
				return nil, fmt.Errorf("atomicity of %s against %s: %w", candidate.ID, a.ID, err)
			}
			out.Covered[a.ID] = p
		}
	}
	return out, nil
}

// AtomicityResult is one atom's reading, or why it has none.
type AtomicityResult struct {
	AtomID    string     `json:"atom_id"`
	Atomicity *Atomicity `json:"atomicity,omitempty"`
	Err       string     `json:"error,omitempty"`
}

// JudgeCorpusAtomicity reads every atom, comparing each with the atoms
// compare names for it (nil compares with none), at most parallel requests at
// a time. A failed request fails only its own atom.
func JudgeCorpusAtomicity(ctx context.Context, asker Asker, atoms []*prompt.PromptAtom, compare func(*prompt.PromptAtom) []*prompt.PromptAtom, parallel int) []AtomicityResult {
	parallel = max(parallel, 1)
	results := make([]AtomicityResult, len(atoms))
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	for i, a := range atoms {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			var cmp []*prompt.PromptAtom
			if compare != nil {
				cmp = compare(a)
			}
			r := AtomicityResult{AtomID: a.ID}
			at, err := JudgeAtomicity(ctx, asker, a, cmp)
			if err != nil {
				r.Err = err.Error()
			} else {
				r.Atomicity = at
			}
			results[i] = r
		}()
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].AtomID < results[j].AtomID })
	return results
}
