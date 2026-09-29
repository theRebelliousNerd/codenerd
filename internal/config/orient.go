package config

import "fmt"

// OrientConfig is the `orient` section of .nerd/config.json: the thresholds
// the orientation policy (internal/orient) decides a repository's eras,
// document lineage and read set with. The engine asserts every field as a
// config_param(/orient_<key>, Value) row. A zero is absent (WithDefaults);
// Check refuses a value that would make a rule draw nothing or accept
// everything.
//
// The section is not on UserConfig yet. Another lane owns that struct and
// check.go. NewEngine takes an OrientConfig directly; the accessor and the
// check.go call are in the lane report until that merge.
type OrientConfig struct {
	// SimilarityFloorPermille is the cosine similarity, times 1000, at or
	// above which two documents are evidence for lineage
	// (/orient_similarity_floor_permille). Pairs below it are not asserted.
	SimilarityFloorPermille int `json:"similarity_floor_permille,omitempty"`
	// SimilarTopK is how many neighbours above the floor each document
	// contributes (/orient_similar_top_k). The scan is exhaustive; only the
	// top K are facts.
	SimilarTopK int `json:"similar_top_k,omitempty"`
	// ReadCandidateBudget is how many documents the policy keeps for a full
	// read (/orient_read_candidate_budget). The rest stay derivable as
	// orient_read_omitted, with the same reasons.
	ReadCandidateBudget int `json:"read_candidate_budget,omitempty"`
	// EraLullCommits is the monthly commit count below which a month is a
	// lull (/orient_era_lull_commits). A month with no commits is a lull at
	// the default of 1, so a gap in the calendar splits eras.
	EraLullCommits int `json:"era_lull_commits,omitempty"`
	// BurstMaxDays is the most distinct commit days a document may span and
	// still be a burst (/orient_burst_max_days).
	BurstMaxDays int `json:"burst_max_days,omitempty"`
	// BurstMinCommits is how many commits, inside that day span, make the
	// burst (/orient_burst_min_commits). One commit is not a burst.
	BurstMinCommits int `json:"burst_min_commits,omitempty"`
	// EmbeddingChunkBytes is the size of one embedding request
	// (/orient_embedding_chunk_bytes). A document is the centroid of its
	// chunks. This bounds one request, not a run.
	EmbeddingChunkBytes int `json:"embedding_chunk_bytes,omitempty"`
	// CohortMinDocs is how many documents must share a birth day before that
	// day counts as a deliberate landing (/orient_cohort_min_docs).
	CohortMinDocs int `json:"cohort_min_docs,omitempty"`
	// CentralityMinLinks is the inbound document-link count at which a
	// document is central (/orient_centrality_min_links).
	CentralityMinLinks int `json:"centrality_min_links,omitempty"`
	// LiveHubMinLinks is how many live documents must link to a document
	// before it is a live hub (/orient_live_hub_min_links).
	LiveHubMinLinks int `json:"live_hub_min_links,omitempty"`
	// OriginSpanPermille, EarlySpanPermille and MiddleSpanPermille divide a
	// single-era repository by where a document's first commit sits in
	// repo_span (/orient_origin_span_permille and the two beside it). A
	// repository with more than one era uses the eras instead.
	OriginSpanPermille int `json:"origin_span_permille,omitempty"`
	EarlySpanPermille  int `json:"early_span_permille,omitempty"`
	MiddleSpanPermille int `json:"middle_span_permille,omitempty"`
}

// DefaultOrientConfig is the orient section with every field written down.
// The similarity floor is 700/1000: neighbours a reader would call the same
// subject, not a shared word. Top-K is 5 so a document's evidence stays a
// handful of peers. The read budget is 32 full documents. A lull is a month
// with no commits. A burst is at least 5 commits on at most 2 days. Chunks
// are 2000 bytes, under a small embedding context. A cohort is 8 documents
// born the same day. Three inbound links make a centre. The single-era
// generations break at 10%, 35% and 70% of the span.
func DefaultOrientConfig() OrientConfig {
	return OrientConfig{
		SimilarityFloorPermille: 700,
		SimilarTopK:             5,
		ReadCandidateBudget:     32,
		EraLullCommits:          1,
		BurstMaxDays:            2,
		BurstMinCommits:         5,
		EmbeddingChunkBytes:     2000,
		CohortMinDocs:           8,
		CentralityMinLinks:      3,
		LiveHubMinLinks:         3,
		OriginSpanPermille:      100,
		EarlySpanPermille:       350,
		MiddleSpanPermille:      700,
	}
}

// WithDefaults fills every absent field from DefaultOrientConfig.
func (c OrientConfig) WithDefaults() OrientConfig {
	d := DefaultOrientConfig()
	if c.SimilarityFloorPermille == 0 {
		c.SimilarityFloorPermille = d.SimilarityFloorPermille
	}
	if c.SimilarTopK == 0 {
		c.SimilarTopK = d.SimilarTopK
	}
	if c.ReadCandidateBudget == 0 {
		c.ReadCandidateBudget = d.ReadCandidateBudget
	}
	if c.EraLullCommits == 0 {
		c.EraLullCommits = d.EraLullCommits
	}
	if c.BurstMaxDays == 0 {
		c.BurstMaxDays = d.BurstMaxDays
	}
	if c.BurstMinCommits == 0 {
		c.BurstMinCommits = d.BurstMinCommits
	}
	if c.EmbeddingChunkBytes == 0 {
		c.EmbeddingChunkBytes = d.EmbeddingChunkBytes
	}
	if c.CohortMinDocs == 0 {
		c.CohortMinDocs = d.CohortMinDocs
	}
	if c.CentralityMinLinks == 0 {
		c.CentralityMinLinks = d.CentralityMinLinks
	}
	if c.LiveHubMinLinks == 0 {
		c.LiveHubMinLinks = d.LiveHubMinLinks
	}
	if c.OriginSpanPermille == 0 {
		c.OriginSpanPermille = d.OriginSpanPermille
	}
	if c.EarlySpanPermille == 0 {
		c.EarlySpanPermille = d.EarlySpanPermille
	}
	if c.MiddleSpanPermille == 0 {
		c.MiddleSpanPermille = d.MiddleSpanPermille
	}
	return c
}

// Check reports the contradictions in an orient section, addressed under
// prefix ("orient"). Absent fields are defaulted first, so only what the
// file says can be wrong.
func (c OrientConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	rangeOf := func(field string, v, lo, hi int, why string) {
		if v < lo || v > hi {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + field,
				Message:  fmt.Sprintf("%d is outside %d..%d: %s", v, lo, hi, why),
				Fix:      fmt.Sprintf("a value from %d to %d, or remove the key for the default", lo, hi),
			})
		}
	}
	atLeast := func(field string, v, floor int, why string) {
		if v < floor {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + field,
				Message:  fmt.Sprintf("%d is below %d: %s", v, floor, why),
				Fix:      fmt.Sprintf("a value of at least %d, or remove the key for the default", floor),
			})
		}
	}
	rangeOf("similarity_floor_permille", c.SimilarityFloorPermille, 1, 1000, "permille is cosine times 1000, and 0 would keep every pair")
	atLeast("similar_top_k", c.SimilarTopK, 1, "top-k is how many neighbours a document may contribute")
	atLeast("read_candidate_budget", c.ReadCandidateBudget, 1, "the read set has to be able to hold a document")
	atLeast("era_lull_commits", c.EraLullCommits, 1, "a lull is a month quieter than this, and a month cannot have fewer than 0 commits")
	atLeast("burst_max_days", c.BurstMaxDays, 1, "a burst is concentrated on at least one day")
	atLeast("burst_min_commits", c.BurstMinCommits, 2, "one commit is not a burst of work")
	rangeOf("embedding_chunk_bytes", c.EmbeddingChunkBytes, 256, 1<<20, "a chunk is one embedding request; below 256 bytes is a request per sentence, above 1MiB is more than one request should carry")
	atLeast("cohort_min_docs", c.CohortMinDocs, 2, "a cohort is more than one document")
	atLeast("centrality_min_links", c.CentralityMinLinks, 1, "centrality is a count of inbound links")
	atLeast("live_hub_min_links", c.LiveHubMinLinks, 1, "a live hub is a count of inbound links")
	rangeOf("origin_span_permille", c.OriginSpanPermille, 1, 999, "the opening slice of a single-era span")
	rangeOf("early_span_permille", c.EarlySpanPermille, 1, 999, "the early slice of a single-era span")
	rangeOf("middle_span_permille", c.MiddleSpanPermille, 1, 999, "the middle slice of a single-era span")
	if c.OriginSpanPermille < c.EarlySpanPermille && c.EarlySpanPermille < c.MiddleSpanPermille && c.MiddleSpanPermille < 1000 {
		return out
	}
	out = append(out, Problem{
		Severity: SeverityError,
		Path:     prefix + ".origin_span_permille",
		Message: fmt.Sprintf("generation cuts are %d, %d, %d permille: they must rise and stay below 1000",
			c.OriginSpanPermille, c.EarlySpanPermille, c.MiddleSpanPermille),
		Fix: "origin_span_permille < early_span_permille < middle_span_permille < 1000",
	})
	return out
}

// Params are the orient thresholds the policy reads, as config_param rows.
// The keys are declared config_param_required(/orient, Key) beside the rules.
func (c OrientConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/orient_similarity_floor_permille", Value: int64(c.SimilarityFloorPermille)},
		{Key: "/orient_similar_top_k", Value: int64(c.SimilarTopK)},
		{Key: "/orient_read_candidate_budget", Value: int64(c.ReadCandidateBudget)},
		{Key: "/orient_era_lull_commits", Value: int64(c.EraLullCommits)},
		{Key: "/orient_burst_max_days", Value: int64(c.BurstMaxDays)},
		{Key: "/orient_burst_min_commits", Value: int64(c.BurstMinCommits)},
		{Key: "/orient_embedding_chunk_bytes", Value: int64(c.EmbeddingChunkBytes)},
		{Key: "/orient_cohort_min_docs", Value: int64(c.CohortMinDocs)},
		{Key: "/orient_centrality_min_links", Value: int64(c.CentralityMinLinks)},
		{Key: "/orient_live_hub_min_links", Value: int64(c.LiveHubMinLinks)},
		{Key: "/orient_origin_span_permille", Value: int64(c.OriginSpanPermille)},
		{Key: "/orient_early_span_permille", Value: int64(c.EarlySpanPermille)},
		{Key: "/orient_middle_span_permille", Value: int64(c.MiddleSpanPermille)},
	}
}

// GetOrientConfig returns the orientation config with every absent key filled
// from DefaultOrientConfig.
func (c *UserConfig) GetOrientConfig() OrientConfig {
	if c == nil || c.Orient == nil {
		return DefaultOrientConfig()
	}
	return c.Orient.WithDefaults()
}
