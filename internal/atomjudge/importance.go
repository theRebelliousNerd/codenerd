package atomjudge

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// Asker is the TypeSafe client's call (typesafe.Client satisfies it).
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error)
}

// Request budget. Jev reads at most 64k tokens per request and 32k per
// question (docs.typesafe.ai/models); batches stay well inside both so the
// state and question framing always fit.
const (
	// batchTokenBudget bounds the guidance tokens in one request.
	batchTokenBudget = 40000
	// questionTokenLimit is the largest guidance one question may carry.
	questionTokenLimit = 30000
	// batchMaxQuestions bounds the questions in one request.
	batchMaxQuestions = 40
)

// ErrTooLarge is a guidance block too large for one Jev question. It is
// reported, never cut to fit.
type ErrTooLarge struct {
	AtomID string
	Tokens int
}

func (e ErrTooLarge) Error() string {
	return fmt.Sprintf("atom %s has %d tokens of guidance; one Jev question holds %d", e.AtomID, e.Tokens, questionTokenLimit)
}

// batches splits atoms into requests within the token and question budgets.
// An atom too large for any question is returned apart.
func batches(atoms []*prompt.PromptAtom) (out [][]*prompt.PromptAtom, tooLarge []ErrTooLarge) {
	var cur []*prompt.PromptAtom
	tokens := 0
	for _, a := range atoms {
		t := prompt.EstimateTokens(a.Content)
		if t > questionTokenLimit {
			tooLarge = append(tooLarge, ErrTooLarge{AtomID: a.ID, Tokens: t})
			continue
		}
		if len(cur) > 0 && (tokens+t > batchTokenBudget || len(cur) >= batchMaxQuestions) {
			out = append(out, cur)
			cur, tokens = nil, 0
		}
		cur = append(cur, a)
		tokens += t
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out, tooLarge
}

// ImportanceResult is one atom's Score, or why it has none.
type ImportanceResult struct {
	AtomID string
	// Model is the versioned Jev ID that answered.
	Model  string
	Record prompt.ImportanceRecord
	Err    error
}

// ScoreImportance asks the importance Score for every atom, parallel requests
// of at most parallel at a time. Each atom gets a record or an error; a
// failed request fails only its own atoms, which stay unscored.
func ScoreImportance(ctx context.Context, asker Asker, atoms []*prompt.PromptAtom, parallel int) []ImportanceResult {
	parallel = max(parallel, 1)
	groups, tooLarge := batches(atoms)
	results := make([]ImportanceResult, 0, len(atoms))
	for _, e := range tooLarge {
		results = append(results, ImportanceResult{AtomID: e.AtomID, Err: e})
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, parallel)
	for _, group := range groups {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out := scoreBatch(ctx, asker, group)
			mu.Lock()
			results = append(results, out...)
			mu.Unlock()
		}()
	}
	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].AtomID < results[j].AtomID })
	return results
}

func scoreBatch(ctx context.Context, asker Asker, group []*prompt.PromptAtom) []ImportanceResult {
	questions := make(map[string]typesafe.Question, len(group))
	for i, a := range group {
		questions["atom_"+strconv.Itoa(i)] = importanceQuestion(guidanceOf(a))
	}
	resp, err := asker.Ask(typesafe.WithOperation(ctx, OpAtomImportance), importanceState, questions)
	out := make([]ImportanceResult, 0, len(group))
	for i, a := range group {
		r := ImportanceResult{AtomID: a.ID}
		if err != nil {
			r.Err = err
			out = append(out, r)
			continue
		}
		r.Model = resp.Model
		r.Record, r.Err = importanceRecord(resp, "atom_"+strconv.Itoa(i), a.ContentHash)
		out = append(out, r)
	}
	return out
}

// importanceRecord reads one Score answer as a ledger record.
func importanceRecord(resp *typesafe.Response, name, contentHash string) (prompt.ImportanceRecord, error) {
	ans, err := resp.Score(name)
	if err != nil {
		return prompt.ImportanceRecord{}, err
	}
	level, err := prompt.ImportanceFromScore(ans.Score)
	if err != nil {
		return prompt.ImportanceRecord{}, err
	}
	n := len(prompt.ImportanceMeanings())
	probs := make([]float64, n)
	for i := range n {
		p, ok := ans.Probabilities[strconv.Itoa(i)]
		if !ok {
			return prompt.ImportanceRecord{}, fmt.Errorf("score answer %s lacks level %d's probability", name, i)
		}
		probs[i] = roundTo(p, 3)
	}
	return prompt.ImportanceRecord{
		ContentHash:   contentHash,
		Level:         level,
		Score:         roundTo(ans.Score, 3),
		Probabilities: probs,
	}, nil
}

func roundTo(v float64, places int) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', places, 64), 64)
	return f
}

// StaleAtoms lists the atoms the ledger does not cover for exactly their
// current content, in ID order. A ledger for another scale covers nothing.
func StaleAtoms(ledger *prompt.ImportanceLedger, atoms []*prompt.PromptAtom) []*prompt.PromptAtom {
	var out []*prompt.PromptAtom
	for _, a := range atoms {
		if ledger == nil || ledger.Scale != ImportanceScaleFingerprint() {
			out = append(out, a)
			continue
		}
		if _, ok := ledger.Lookup(a.ID, a.ContentHash); !ok {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
