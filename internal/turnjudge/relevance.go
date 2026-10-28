package turnjudge

import (
	"context"
	"fmt"

	"codenerd/internal/retrieval"
	"codenerd/internal/typesafe"
)

// RelevanceJudge asks Jev, for each candidate file, whether reading it would
// help with the request: one request, one question per file (TypeSafe's
// re-ranking pattern). It implements retrieval.RelevanceJudge.
type RelevanceJudge struct {
	Asker Asker
}

var _ retrieval.RelevanceJudge = RelevanceJudge{}

// relevanceBatch bounds the files in one request: each carries its head, and
// a request is 64k tokens. Batches run in parallel.
const relevanceBatch = 20

// Relevance returns each file's probability of helping, keyed by path. The
// files are judged in parallel batches; any failed batch fails the whole
// judgment, so a partial verdict never reorders the context.
func (j RelevanceJudge) Relevance(ctx context.Context, request string, files []retrieval.RelevanceCandidate) (map[string]float64, error) {
	type part struct {
		probs map[string]float64
		err   error
	}
	var parts []chan part
	for start := 0; start < len(files); start += relevanceBatch {
		batch := files[start:min(start+relevanceBatch, len(files))]
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
	resp, err := j.Asker.Ask(ctx, state, questions)
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
