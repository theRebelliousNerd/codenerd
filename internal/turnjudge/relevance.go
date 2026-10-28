package turnjudge

import (
	"context"
	"fmt"

	"codenerd/internal/config"
	"codenerd/internal/retrieval"
	"codenerd/internal/typesafe"
)

// RelevanceJudge asks Jev, for each candidate file, whether reading it would
// help with the request: one request, one question per file (TypeSafe's
// re-ranking pattern). It implements retrieval.RelevanceJudge.
type RelevanceJudge struct {
	Asker Asker
	// BatchSize bounds the files in one request (typesafe.batch_size); each
	// file carries its head, and batches run in parallel.
	BatchSize int
}

var _ retrieval.RelevanceJudge = RelevanceJudge{}

// RelevanceUse is the retrieval relevance use resolved from the typesafe
// section, for every retrieval pass that runs: the chat's seed and the
// session executor's per-task brief. Judge is nil when Jev or the use is off.
type RelevanceUse struct {
	Judge      retrieval.RelevanceJudge
	Drop       float64
	PoolFactor int
	HeadBytes  int
}

// RelevanceFor resolves the use; a nil asker is Jev off.
func RelevanceFor(a Asker, sec *config.TypeSafeConfig) RelevanceUse {
	uses := sec.Uses()
	if a == nil || !uses.RetrievalRelevance {
		return RelevanceUse{}
	}
	return RelevanceUse{
		Judge:      RelevanceJudge{Asker: a, BatchSize: uses.BatchSize},
		Drop:       sec.NoProbability(),
		PoolFactor: uses.RelevancePoolFactor,
		HeadBytes:  uses.RelevanceHeadBytes,
	}
}

// Relevance returns each file's probability of helping, keyed by path. The
// files are judged in parallel batches; any failed batch fails the whole
// judgment, so a partial verdict never reorders the context.
func (j RelevanceJudge) Relevance(ctx context.Context, request string, files []retrieval.RelevanceCandidate) (map[string]float64, error) {
	type part struct {
		probs map[string]float64
		err   error
	}
	var parts []chan part
	size := max(j.BatchSize, 1)
	for start := 0; start < len(files); start += size {
		batch := files[start:min(start+size, len(files))]
		ch := make(chan part, 1)
		parts = append(parts, ch)
		go func() {
			p, err := j.judgeBatch(ctx, request, batch)
			ch <- part{p, err}
		}()
	}
	out := make(map[string]float64, len(files))
	var firstErr error
	for _, ch := range parts {
		p := <-ch
		if p.err != nil && firstErr == nil {
			firstErr = p.err
		}
		for k, v := range p.probs {
			out[k] = v
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func (j RelevanceJudge) judgeBatch(ctx context.Context, request string, files []retrieval.RelevanceCandidate) (map[string]float64, error) {
	state := map[string]any{"request": request}
	questions := make(map[string]typesafe.Question, len(files))
	names := make(map[string]string, len(files))
	for i, f := range files {
		name := fmt.Sprintf("file_%d", i)
		names[name] = f.Path
		file := map[string]any{"path": f.Path}
		if len(f.Keywords) > 0 {
			file["matched_keywords"] = f.Keywords
		}
		if f.Excerpt != "" {
			file["head"] = f.Excerpt
			if f.ExcerptOf > len(f.Excerpt) {
				file["head_note"] = fmt.Sprintf("the first %d of %d bytes", len(f.Excerpt), f.ExcerptOf)
			}
		}
		questions[name] = typesafe.Noul{
			Instructions: map[string]any{
				"question": "Would reading this file help answer or carry out `request`?",
				"file":     file,
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "The file holds code, tests or docs the request is about, or code the request would have to change or call.",
				False: "The file only shares a word with the request, such as a common name, and is about something else.",
			},
		}
	}
	resp, err := j.Asker.Ask(typesafe.WithOperation(ctx, OpRetrievalRelevance), state, questions)
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(files))
	for name, path := range names {
		p, err := resp.Noul(name)
		if err != nil {
			return nil, fmt.Errorf("relevance of %s: %w", path, err)
		}
		out[path] = p
	}
	return out, nil
}
