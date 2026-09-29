package config

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

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
	if got, want := len(c.Params()), reflect.TypeOf(c).NumField()-len(orientRequestBounds); got != want {
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

func TestOrientConfig_C3ValuesRoundTripAndReachPolicy(t *testing.T) {
	t.Parallel()
	c := DefaultOrientConfig()
	value := reflect.ValueOf(&c).Elem()
	for _, name := range []string{
		"EmbeddingHubTopKMultiple", "EmbeddingBatchSize", "EmbeddingConcurrency", "EmbeddingRetryAttempts",
		"ReasonRarityFloorPermille", "NearDuplicatePermille", "CohortWindowDays",
		"CohortShareCeilingPermille", "EraLullMedianPermille",
		"ReadWeightInstructions", "ReadWeightChainLatest", "ReadWeightOrigin",
		"ReadWeightClusterRep", "ReadWeightLiveHub", "ReadWeightLinkHub",
		"ReadWeightBurst", "ReadWeightCohort", "VisionWeightConfidence",
		"VisionWeightRole", "VisionWeightRecent", "VisionWeightOrigin",
		"VisionWeightEarly", "VisionWeightMiddle", "VisionWeightBurst",
		"VisionWeightCohort", "VisionWeightCentral", "VisionWeightLive",
	} {
		field := value.FieldByName(name)
		field.SetInt(field.Int() + 1)
	}
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var restored OrientConfig
	if err := json.Unmarshal(body, &restored); err != nil {
		t.Fatal(err)
	}
	if got := restored.WithDefaults(); got != c {
		t.Fatalf("explicit fields lost: %+v", got)
	}
	if problems := restored.Check("orient"); len(problems) != 0 {
		t.Fatalf("explicit fields: %v", problems)
	}
	params := map[string]int64{}
	for _, p := range restored.Params() {
		params[p.Key] = p.Value
	}
	kind := value.Type()
	for i := 0; i < value.NumField(); i++ {
		if orientRequestBounds[kind.Field(i).Name] {
			continue
		}
		key := "/orient_" + strings.Split(kind.Field(i).Tag.Get("json"), ",")[0]
		if params[key] != value.Field(i).Int() {
			t.Errorf("%s did not reach Params", key)
		}
	}
}

func TestOrientConfig_C3InvalidValues(t *testing.T) {
	t.Parallel()
	cases := []struct{ body, path string }{
		{`{"embedding_batch_size":-1}`, "embedding_batch_size"},
		{`{"embedding_batch_size":1025}`, "embedding_batch_size"},
		{`{"embedding_concurrency":65}`, "embedding_concurrency"},
		{`{"embedding_retry_attempts":11}`, "embedding_retry_attempts"},
		{`{"reason_rarity_floor_permille":1001}`, "reason_rarity_floor_permille"},
		{`{"near_duplicate_permille":699}`, "near_duplicate_permille"},
		{`{"cohort_window_days":32}`, "cohort_window_days"},
		{`{"cohort_share_ceiling_permille":1000}`, "cohort_share_ceiling_permille"},
		{`{"era_lull_median_permille":1001}`, "era_lull_median_permille"},
		{`{"read_weight_origin":1001}`, "read_weight_origin"},
		{`{"vision_weight_confidence":101}`, "vision_weight_confidence"},
	}
	for _, tc := range cases {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			var c OrientConfig
			if err := json.Unmarshal([]byte(tc.body), &c); err != nil {
				t.Fatal(err)
			}
			for _, p := range c.Check("orient") {
				if p.Path == "orient."+tc.path && p.Severity == SeverityError {
					return
				}
			}
			t.Fatalf("accepted %s", tc.body)
		})
	}
}
