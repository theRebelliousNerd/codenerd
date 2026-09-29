package config

import "testing"

func TestOrientEcosystemThresholds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   OrientConfig
		valid bool
	}{
		{"absent", OrientConfig{}, true},
		{"custom", OrientConfig{TopicOverlapMin: 3, SkillClusterMin: 4}, true},
		{"negative overlap", OrientConfig{TopicOverlapMin: -1}, false},
		{"singleton cluster", OrientConfig{SkillClusterMin: 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(tc.cfg.Check("orient")) == 0; got != tc.valid {
				t.Fatalf("Check valid=%v want=%v", got, tc.valid)
			}
			applied := tc.cfg.WithDefaults()
			values := map[string]int64{}
			for _, param := range applied.Params() {
				values[param.Key] = param.Value
			}
			if values["/orient_topic_overlap_min"] != int64(applied.TopicOverlapMin) || values["/orient_skill_cluster_min"] != int64(applied.SkillClusterMin) {
				t.Fatal("configured thresholds did not reach policy Params")
			}
		})
	}
}
