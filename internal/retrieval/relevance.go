package retrieval

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"
)

// RelevanceJudge rates, for each candidate file, the probability that reading
// it helps with the request (turnjudge.RelevanceJudge asks Jev). It enhances
// Tier 2's keyword ranking: the judge sees a wider pool than the tier keeps,
// the most relevant files are kept, and a file it judges irrelevant is left
// out. A failed judgment is no evidence: the keyword ranking stands.
type RelevanceJudge interface {
	Relevance(ctx context.Context, request string, files []RelevanceCandidate) (map[string]float64, error)
}

// RelevanceCandidate is one file as the judge sees it.
type RelevanceCandidate struct {
	Path     string
	Keywords []string
	// Excerpt is the file's head, for judging only; the file itself is never
	// cut. ExcerptOf says how much of the file it is.
	Excerpt   string
	ExcerptOf int
}

// relevancePoolFactor widens the keyword pool the judge sees beyond the
// files Tier 2 keeps, so a relevant file ranked low by keywords can rise:
// keyword ranking puts a doc that names every term above the source file
// that implements them.
const relevancePoolFactor = 6

// relevanceExcerptBytes bounds each file's head in the judgment request: Jev
// reads every question against a 64k-token request context.
const relevanceExcerptBytes = 3000

// JudgedFile is one Tier 2 candidate's relevance judgment.
type JudgedFile struct {
	FilePath    string
	Probability float64
	Kept        bool
}

// judgeTier2 reorders ranked by the judge's probability (keyword score breaks
// ties) and drops candidates at or below drop. It returns the reordered
// candidates and every judgment; on a failed judgment, ranked unchanged.
func (b *TieredContextBuilder) judgeTier2(ctx context.Context, request string, ranked []CandidateFile, addedFiles map[string]bool) ([]CandidateFile, []JudgedFile) {
	var pool []CandidateFile
	var asks []RelevanceCandidate
	for _, c := range ranked {
		if addedFiles[c.FilePath] {
			continue
		}
		pool = append(pool, c)
		asks = append(asks, b.relevanceCandidate(c))
	}
	if len(pool) == 0 {
		return ranked, nil
	}
	probs, err := b.relevance.Relevance(ctx, request, asks)
	if err != nil {
		logging.Context("TieredContextBuilder: relevance judgment failed, keyword ranking stands: %v", err)
		return ranked, nil
	}
	sort.SliceStable(pool, func(i, k int) bool { return probs[pool[i].FilePath] > probs[pool[k].FilePath] })
	var kept []CandidateFile
	judged := make([]JudgedFile, 0, len(pool))
	for _, c := range pool {
		p, ok := probs[c.FilePath]
		keep := !ok || p > b.relevanceDrop
		judged = append(judged, JudgedFile{FilePath: c.FilePath, Probability: p, Kept: keep})
		if keep {
			kept = append(kept, c)
		}
	}
	logging.Context("TieredContextBuilder: relevance judged %d Tier 2 candidates, %d kept", len(pool), len(kept))
	return kept, judged
}

func (b *TieredContextBuilder) relevanceCandidate(c CandidateFile) RelevanceCandidate {
	rc := RelevanceCandidate{Path: c.FilePath, Keywords: c.Keywords}
	path := c.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.workDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rc
	}
	rc.ExcerptOf = len(data)
	head := string(data)
	if len(head) > relevanceExcerptBytes {
		head = strings.ToValidUTF8(head[:relevanceExcerptBytes], "")
	}
	rc.Excerpt = head
	return rc
}
