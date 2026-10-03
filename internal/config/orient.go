package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OrientConfig is the `orient` section of .nerd/config.json: the thresholds
// the orientation policy (internal/orient) decides a repository's eras,
// document lineage and read set with. The engine asserts every field as a
// config_param(/orient_<key>, Value) row. A zero is absent (WithDefaults);
// Check refuses a value that would make a rule draw nothing or accept
// everything.
//
// UserConfig and check.go are owned by the configuration integration lane.
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
	TopicOverlapMin    int `json:"topic_overlap_min,omitempty"`
	SkillClusterMin    int `json:"skill_cluster_min,omitempty"`

	// Embedding request bounds and inverse-prevalence signal weights.
	EmbeddingHubTopKMultiple   int `json:"embedding_hub_top_k_multiple,omitempty"`
	EmbeddingBatchSize         int `json:"embedding_batch_size,omitempty"`
	EmbeddingConcurrency       int `json:"embedding_concurrency,omitempty"`
	EmbeddingRetryAttempts     int `json:"embedding_retry_attempts,omitempty"`
	ReasonRarityFloorPermille  int `json:"reason_rarity_floor_permille,omitempty"`
	NearDuplicatePermille      int `json:"near_duplicate_permille,omitempty"`
	CohortWindowDays           int `json:"cohort_window_days,omitempty"`
	CohortShareCeilingPermille int `json:"cohort_share_ceiling_permille,omitempty"`
	EraLullMedianPermille      int `json:"era_lull_median_permille,omitempty"`
	ReadWeightInstructions     int `json:"read_weight_instructions,omitempty"`
	ReadWeightChainLatest      int `json:"read_weight_chain_latest,omitempty"`
	ReadWeightOrigin           int `json:"read_weight_origin,omitempty"`
	ReadWeightClusterRep       int `json:"read_weight_cluster_rep,omitempty"`
	ReadWeightLiveHub          int `json:"read_weight_live_hub,omitempty"`
	ReadWeightLinkHub          int `json:"read_weight_link_hub,omitempty"`
	ReadWeightBurst            int `json:"read_weight_burst,omitempty"`
	ReadWeightCohort           int `json:"read_weight_cohort,omitempty"`
	VisionWeightConfidence     int `json:"vision_weight_confidence,omitempty"`
	VisionWeightRole           int `json:"vision_weight_role,omitempty"`
	VisionWeightRecent         int `json:"vision_weight_recent,omitempty"`
	VisionWeightOrigin         int `json:"vision_weight_origin,omitempty"`
	VisionWeightEarly          int `json:"vision_weight_early,omitempty"`
	VisionWeightMiddle         int `json:"vision_weight_middle,omitempty"`
	VisionWeightBurst          int `json:"vision_weight_burst,omitempty"`
	VisionWeightCohort         int `json:"vision_weight_cohort,omitempty"`
	VisionWeightCentral        int `json:"vision_weight_central,omitempty"`
	VisionWeightLive           int `json:"vision_weight_live,omitempty"`

	// DeriveRequestBytes bounds one north-star derivation request: framing,
	// carried-forward summary and document bytes together. A larger document
	// is paged, never cut. It bounds a request, not a derivation run. No rule
	// reads it, so it is not a Params row (orientRequestBounds).
	DeriveRequestBytes int `json:"derive_request_bytes,omitempty"`
	// DeriveAttempts is how many times one derivation exchange is sent when
	// the reply cannot be parsed. A reasoning model occasionally emits
	// malformed JSON for a request it answers cleanly on the next try; one
	// such reply used to abort the whole classification. After the last
	// attempt a classification batch is recorded unread and the rest go on.
	DeriveAttempts int `json:"derive_attempts,omitempty"`
}

// LoadOrientConfig reads only the target workspace's orientation section.
// The caller receives invalid explicit values as errors, never defaults.
func LoadOrientConfig(workspace string) (OrientConfig, error) {
	cfg := DefaultOrientConfig()
	if strings.TrimSpace(workspace) == "" {
		return cfg, nil
	}
	body, err := os.ReadFile(filepath.Join(workspace, ".nerd", "config.json"))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	var section struct {
		Orient *OrientConfig `json:"orient"`
	}
	if err := json.Unmarshal(body, &section); err != nil {
		return cfg, fmt.Errorf("orientation configuration: %w", err)
	}
	if section.Orient != nil {
		cfg = section.Orient.WithDefaults()
	}
	if problems := cfg.Check("orient"); len(problems) > 0 {
		return cfg, fmt.Errorf("%s: %s", problems[0].Path, problems[0].Message)
	}
	return cfg, nil
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
		TopicOverlapMin:         2,
		SkillClusterMin:         2,

		// Embedding and signal defaults.
		EmbeddingHubTopKMultiple:   3,
		EmbeddingBatchSize:         32,
		EmbeddingConcurrency:       4,
		EmbeddingRetryAttempts:     3,
		ReasonRarityFloorPermille:  50,
		NearDuplicatePermille:      950,
		CohortWindowDays:           2,
		CohortShareCeilingPermille: 100,
		EraLullMedianPermille:      100,
		ReadWeightInstructions:     100,
		ReadWeightChainLatest:      90,
		ReadWeightOrigin:           80,
		ReadWeightClusterRep:       70,
		ReadWeightLiveHub:          65,
		ReadWeightLinkHub:          60,
		ReadWeightBurst:            55,
		ReadWeightCohort:           50,
		VisionWeightConfidence:     40,
		VisionWeightRole:           10,
		VisionWeightRecent:         20,
		VisionWeightOrigin:         10,
		VisionWeightEarly:          8,
		VisionWeightMiddle:         5,
		VisionWeightBurst:          20,
		VisionWeightCohort:         15,
		VisionWeightCentral:        10,
		VisionWeightLive:           15,
		DeriveRequestBytes:         32 << 10,
		DeriveAttempts:             3,
	}
}

// WithDefaults fills every absent field from DefaultOrientConfig.
func (c OrientConfig) WithDefaults() OrientConfig {
	d := DefaultOrientConfig()
	// Embedding and signal settings share defaults with policy parameters.
	if c.EmbeddingHubTopKMultiple == 0 {
		c.EmbeddingHubTopKMultiple = d.EmbeddingHubTopKMultiple
	}
	if c.EmbeddingBatchSize == 0 {
		c.EmbeddingBatchSize = d.EmbeddingBatchSize
	}
	if c.EmbeddingConcurrency == 0 {
		c.EmbeddingConcurrency = d.EmbeddingConcurrency
	}
	if c.EmbeddingRetryAttempts == 0 {
		c.EmbeddingRetryAttempts = d.EmbeddingRetryAttempts
	}
	if c.ReasonRarityFloorPermille == 0 {
		c.ReasonRarityFloorPermille = d.ReasonRarityFloorPermille
	}
	if c.NearDuplicatePermille == 0 {
		c.NearDuplicatePermille = d.NearDuplicatePermille
	}
	if c.CohortWindowDays == 0 {
		c.CohortWindowDays = d.CohortWindowDays
	}
	if c.CohortShareCeilingPermille == 0 {
		c.CohortShareCeilingPermille = d.CohortShareCeilingPermille
	}
	if c.EraLullMedianPermille == 0 {
		c.EraLullMedianPermille = d.EraLullMedianPermille
	}
	if c.ReadWeightInstructions == 0 {
		c.ReadWeightInstructions = d.ReadWeightInstructions
	}
	if c.ReadWeightChainLatest == 0 {
		c.ReadWeightChainLatest = d.ReadWeightChainLatest
	}
	if c.ReadWeightOrigin == 0 {
		c.ReadWeightOrigin = d.ReadWeightOrigin
	}
	if c.ReadWeightClusterRep == 0 {
		c.ReadWeightClusterRep = d.ReadWeightClusterRep
	}
	if c.ReadWeightLiveHub == 0 {
		c.ReadWeightLiveHub = d.ReadWeightLiveHub
	}
	if c.ReadWeightLinkHub == 0 {
		c.ReadWeightLinkHub = d.ReadWeightLinkHub
	}
	if c.ReadWeightBurst == 0 {
		c.ReadWeightBurst = d.ReadWeightBurst
	}
	if c.ReadWeightCohort == 0 {
		c.ReadWeightCohort = d.ReadWeightCohort
	}
	if c.VisionWeightConfidence == 0 {
		c.VisionWeightConfidence = d.VisionWeightConfidence
	}
	if c.VisionWeightRole == 0 {
		c.VisionWeightRole = d.VisionWeightRole
	}
	if c.VisionWeightRecent == 0 {
		c.VisionWeightRecent = d.VisionWeightRecent
	}
	if c.VisionWeightOrigin == 0 {
		c.VisionWeightOrigin = d.VisionWeightOrigin
	}
	if c.VisionWeightEarly == 0 {
		c.VisionWeightEarly = d.VisionWeightEarly
	}
	if c.VisionWeightMiddle == 0 {
		c.VisionWeightMiddle = d.VisionWeightMiddle
	}
	if c.VisionWeightBurst == 0 {
		c.VisionWeightBurst = d.VisionWeightBurst
	}
	if c.VisionWeightCohort == 0 {
		c.VisionWeightCohort = d.VisionWeightCohort
	}
	if c.VisionWeightCentral == 0 {
		c.VisionWeightCentral = d.VisionWeightCentral
	}
	if c.VisionWeightLive == 0 {
		c.VisionWeightLive = d.VisionWeightLive
	}
	if c.TopicOverlapMin == 0 {
		c.TopicOverlapMin = d.TopicOverlapMin
	}
	if c.SkillClusterMin == 0 {
		c.SkillClusterMin = d.SkillClusterMin
	}
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
	if c.DeriveRequestBytes == 0 {
		c.DeriveRequestBytes = d.DeriveRequestBytes
	}
	if c.DeriveAttempts == 0 {
		c.DeriveAttempts = d.DeriveAttempts
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
	rangeOf("embedding_hub_top_k_multiple", c.EmbeddingHubTopKMultiple, 2, 1000, "embedding hub degree relative to the top-k allowance")
	rangeOf("embedding_batch_size", c.EmbeddingBatchSize, 1, 1024, "chunks per embedding batch")
	rangeOf("embedding_concurrency", c.EmbeddingConcurrency, 1, 64, "embedding batch workers")
	rangeOf("embedding_retry_attempts", c.EmbeddingRetryAttempts, 1, 10, "total attempts per embedding request")
	rangeOf("reason_rarity_floor_permille", c.ReasonRarityFloorPermille, 1, 1000, "minimum inverse-prevalence factor")
	rangeOf("near_duplicate_permille", c.NearDuplicatePermille, 1, 1000, "similarity for one duplicate read unit")
	rangeOf("cohort_window_days", c.CohortWindowDays, 1, 31, "UTC birth window length")
	rangeOf("cohort_share_ceiling_permille", c.CohortShareCeilingPermille, 1, 999, "maximum repository share of a cohort")
	rangeOf("era_lull_median_permille", c.EraLullMedianPermille, 1, 1000, "relative lull threshold as a share of median commits")
	rangeOf("read_weight_instructions", c.ReadWeightInstructions, 1, 1000, "instruction read weight")
	rangeOf("read_weight_chain_latest", c.ReadWeightChainLatest, 1, 1000, "lineage tip read weight")
	rangeOf("read_weight_origin", c.ReadWeightOrigin, 1, 1000, "origin read weight")
	rangeOf("read_weight_cluster_rep", c.ReadWeightClusterRep, 1, 1000, "similarity representative read weight")
	rangeOf("read_weight_live_hub", c.ReadWeightLiveHub, 1, 1000, "live hub read weight")
	rangeOf("read_weight_link_hub", c.ReadWeightLinkHub, 1, 1000, "link hub read weight")
	rangeOf("read_weight_burst", c.ReadWeightBurst, 1, 1000, "burst read weight")
	rangeOf("read_weight_cohort", c.ReadWeightCohort, 1, 1000, "cohort read weight")
	rangeOf("vision_weight_confidence", c.VisionWeightConfidence, 1, 100, "confidence contribution ceiling")
	rangeOf("vision_weight_role", c.VisionWeightRole, 1, 100, "vision role weight")
	rangeOf("vision_weight_recent", c.VisionWeightRecent, 1, 100, "recent generation vision weight")
	rangeOf("vision_weight_origin", c.VisionWeightOrigin, 1, 100, "origin generation vision weight")
	rangeOf("vision_weight_early", c.VisionWeightEarly, 1, 100, "early generation vision weight")
	rangeOf("vision_weight_middle", c.VisionWeightMiddle, 1, 100, "middle generation vision weight")
	rangeOf("vision_weight_burst", c.VisionWeightBurst, 1, 100, "burst vision weight")
	rangeOf("vision_weight_cohort", c.VisionWeightCohort, 1, 100, "cohort vision weight")
	rangeOf("vision_weight_central", c.VisionWeightCentral, 1, 100, "centrality vision weight")
	rangeOf("vision_weight_live", c.VisionWeightLive, 1, 100, "live vision weight")
	if c.NearDuplicatePermille < c.SimilarityFloorPermille {
		out = append(out, Problem{Severity: SeverityError, Path: prefix + ".near_duplicate_permille", Message: "near-duplicate threshold must be at least similarity_floor_permille", Fix: "raise near_duplicate_permille or lower similarity_floor_permille"})
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
	atLeast("topic_overlap_min", c.TopicOverlapMin, 1, "duplicate evidence requires shared topics")
	atLeast("skill_cluster_min", c.SkillClusterMin, 2, "a skill cluster requires more than one source")
	rangeOf("origin_span_permille", c.OriginSpanPermille, 1, 999, "the opening slice of a single-era span")
	rangeOf("early_span_permille", c.EarlySpanPermille, 1, 999, "the early slice of a single-era span")
	rangeOf("middle_span_permille", c.MiddleSpanPermille, 1, 999, "the middle slice of a single-era span")
	rangeOf("derive_request_bytes", c.DeriveRequestBytes, 256, 1<<20, "a request must fit framing, carry-forward context and document bytes")
	rangeOf("derive_attempts", c.DeriveAttempts, 1, 10, "each attempt is one model request for the same exchange")
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

// orientRequestBounds are the orient fields Go reads as request bounds. They
// are checked like every orient key but are not policy thresholds, so Params
// leaves them out and no config_param_required row names them.
var orientRequestBounds = map[string]bool{"DeriveRequestBytes": true, "DeriveAttempts": true}

// Params are the orient thresholds the policy reads, as config_param rows.
// The keys are declared config_param_required(/orient, Key) beside the rules.
func (c OrientConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/orient_embedding_hub_top_k_multiple", Value: int64(c.EmbeddingHubTopKMultiple)},
		{Key: "/orient_embedding_batch_size", Value: int64(c.EmbeddingBatchSize)},
		{Key: "/orient_embedding_concurrency", Value: int64(c.EmbeddingConcurrency)},
		{Key: "/orient_embedding_retry_attempts", Value: int64(c.EmbeddingRetryAttempts)},
		{Key: "/orient_reason_rarity_floor_permille", Value: int64(c.ReasonRarityFloorPermille)},
		{Key: "/orient_near_duplicate_permille", Value: int64(c.NearDuplicatePermille)},
		{Key: "/orient_cohort_window_days", Value: int64(c.CohortWindowDays)},
		{Key: "/orient_cohort_share_ceiling_permille", Value: int64(c.CohortShareCeilingPermille)},
		{Key: "/orient_era_lull_median_permille", Value: int64(c.EraLullMedianPermille)},
		{Key: "/orient_read_weight_instructions", Value: int64(c.ReadWeightInstructions)},
		{Key: "/orient_read_weight_chain_latest", Value: int64(c.ReadWeightChainLatest)},
		{Key: "/orient_read_weight_origin", Value: int64(c.ReadWeightOrigin)},
		{Key: "/orient_read_weight_cluster_rep", Value: int64(c.ReadWeightClusterRep)},
		{Key: "/orient_read_weight_live_hub", Value: int64(c.ReadWeightLiveHub)},
		{Key: "/orient_read_weight_link_hub", Value: int64(c.ReadWeightLinkHub)},
		{Key: "/orient_read_weight_burst", Value: int64(c.ReadWeightBurst)},
		{Key: "/orient_read_weight_cohort", Value: int64(c.ReadWeightCohort)},
		{Key: "/orient_vision_weight_confidence", Value: int64(c.VisionWeightConfidence)},
		{Key: "/orient_vision_weight_role", Value: int64(c.VisionWeightRole)},
		{Key: "/orient_vision_weight_recent", Value: int64(c.VisionWeightRecent)},
		{Key: "/orient_vision_weight_origin", Value: int64(c.VisionWeightOrigin)},
		{Key: "/orient_vision_weight_early", Value: int64(c.VisionWeightEarly)},
		{Key: "/orient_vision_weight_middle", Value: int64(c.VisionWeightMiddle)},
		{Key: "/orient_vision_weight_burst", Value: int64(c.VisionWeightBurst)},
		{Key: "/orient_vision_weight_cohort", Value: int64(c.VisionWeightCohort)},
		{Key: "/orient_vision_weight_central", Value: int64(c.VisionWeightCentral)},
		{Key: "/orient_vision_weight_live", Value: int64(c.VisionWeightLive)},
		{Key: "/orient_topic_overlap_min", Value: int64(c.TopicOverlapMin)},
		{Key: "/orient_skill_cluster_min", Value: int64(c.SkillClusterMin)},
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
