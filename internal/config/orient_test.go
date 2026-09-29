package config

import "testing"

func TestOrientConfig_DefaultsPassCheck(t *testing.T) {
	t.Parallel()
	c := DefaultOrientConfig()
	if probs := c.Check("orient"); len(probs) != 0 {
		t.Fatalf("defaults: %v", probs)
	}
	filled := OrientConfig{}.WithDefaults()
	if filled != c {
		t.Fatalf("zero config defaults to %+v, want %+v", filled, c)
	}
	if got, want := len(c.Params()), 13; got != want {
		t.Fatalf("params: %d, want %d", got, want)
	}
	for _, p := range c.Params() {
		if p.Value == 0 || p.Key == "" || p.Key[0] != '/' {
			t.Fatalf("param %+v", p)
		}
	}
}

func TestOrientConfig_CheckRejectsContradictions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		cfg   OrientConfig
		path  string
		wantN int
	}{
		{
			name: "floor above permille",
			cfg:  OrientConfig{SimilarityFloorPermille: 1001},
			path: "orient.similarity_floor_permille",
		},
		{
			name: "negative budget survives defaults",
			cfg:  OrientConfig{ReadCandidateBudget: -1},
			path: "orient.read_candidate_budget",
		},
		{
			name: "one commit is not a burst",
			cfg:  OrientConfig{BurstMinCommits: 1},
			path: "orient.burst_min_commits",
		},
		{
			name: "chunk is one request",
			cfg:  OrientConfig{EmbeddingChunkBytes: 8},
			path: "orient.embedding_chunk_bytes",
		},
		{
			name: "generation cuts must rise",
			cfg: OrientConfig{
				OriginSpanPermille: 400,
				EarlySpanPermille:  200,
				MiddleSpanPermille: 700,
			},
			path: "orient.origin_span_permille",
		},
		{
			name: "cohort of one",
			cfg:  OrientConfig{CohortMinDocs: 1},
			path: "orient.cohort_min_docs",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			probs := tc.cfg.Check("orient")
			if len(probs) == 0 {
				t.Fatal("expected a problem")
			}
			found := false
			for _, p := range probs {
				if p.Path == tc.path && p.Severity == SeverityError {
					found = true
				}
			}
			if !found {
				t.Fatalf("problems %v, want path %s", probs, tc.path)
			}
		})
	}
}

func TestOrientConfig_ExplicitValueWinsOverDefault(t *testing.T) {
	t.Parallel()
	c := OrientConfig{ReadCandidateBudget: 4, SimilarityFloorPermille: 800}.WithDefaults()
	if c.ReadCandidateBudget != 4 || c.SimilarityFloorPermille != 800 {
		t.Fatalf("explicit values lost: %+v", c)
	}
	if c.EraLullCommits != DefaultOrientConfig().EraLullCommits {
		t.Fatalf("absent lull was not defaulted: %+v", c)
	}
	if probs := c.Check("orient"); len(probs) != 0 {
		t.Fatalf("mixed config: %v", probs)
	}
}
