package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrientTreatmentDefaultsPassCheckAndReachParams(t *testing.T) {
	t.Parallel()
	if probs := DefaultOrientTreatmentConfig().Check("orient.treatment"); len(probs) != 0 {
		t.Fatalf("defaults: %v", probs)
	}
	if got := (OrientConfig{}).TreatmentPolicy(); got.CensusMaxDepth != DefaultOrientTreatmentConfig().CensusMaxDepth {
		t.Fatalf("absent section is not the defaults: %+v", got)
	}
	explicit := OrientConfig{Treatment: &OrientTreatmentConfig{SourceSharePermille: 900}}
	params := map[string]int64{}
	for _, p := range explicit.TreatmentPolicy().Params() {
		params[p.Key] = p.Value
	}
	if params["/orient_treatment_source_share_permille"] != 900 || params["/orient_treatment_data_min_bytes"] != DefaultOrientTreatmentConfig().DataMinBytes {
		t.Fatalf("params %v", params)
	}
}

func TestOrientTreatmentCheckRefusesContradictions(t *testing.T) {
	t.Parallel()
	op := TreatmentBasisOperational
	cases := []struct {
		name string
		cfg  OrientTreatmentConfig
		path string
	}{
		{"depth", OrientTreatmentConfig{CensusMaxDepth: 33}, "orient.treatment.census_max_depth"},
		{"budget", OrientTreatmentConfig{HeaderReadBudget: -1}, "orient.treatment.header_read_budget"},
		{"min samples above per-dir", OrientTreatmentConfig{HeaderSamplesPerDir: 2, GeneratedMinSamples: 3}, "orient.treatment.generated_min_samples"},
		{"empty path", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "", Treatment: TreatmentExclude, Basis: op}}}, "orient.treatment.overrides[0].path"},
		{"absolute path", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "/data", Treatment: TreatmentExclude, Basis: op}}}, "orient.treatment.overrides[0].path"},
		{"trailing slash", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "data/", Treatment: TreatmentExclude, Basis: op}}}, "orient.treatment.overrides[0].path"},
		{"escape", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "a/../b", Treatment: TreatmentExclude, Basis: op}}}, "orient.treatment.overrides[0].path"},
		{"duplicate", OrientTreatmentConfig{Overrides: []TreatmentOverride{
			{Path: "data", Treatment: TreatmentExclude, Basis: op},
			{Path: "data", Treatment: TreatmentIndexNamesOnly, Basis: op},
		}}, "orient.treatment.overrides[1].path"},
		{"unknown treatment", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "data", Treatment: "hide", Basis: op}}}, "orient.treatment.overrides[0].treatment"},
		{"missing basis", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "data", Treatment: TreatmentExclude}}}, "orient.treatment.overrides[0].basis"},
		{"restriction cannot grant", OrientTreatmentConfig{Overrides: []TreatmentOverride{{Path: "data", Treatment: TreatmentIndexAndParse, Basis: TreatmentBasisAccessRestriction}}}, "orient.treatment.overrides[0].basis"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Through the orient section, the way NewEngine and LoadOrientConfig see it.
			probs := OrientConfig{Treatment: &tc.cfg}.Check("orient")
			for _, p := range probs {
				if p.Path == tc.path && p.Severity == SeverityError {
					return
				}
			}
			t.Fatalf("problems %v, want %s", probs, tc.path)
		})
	}
}

func TestLoadOrientConfigEnforcesTreatment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"orient":{"treatment":{"overrides":[{"path":"seeds","treatment":"index_names_only","basis":"operational"}]}}}`)
	cfg, err := LoadOrientConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TreatmentPolicy().Overrides; len(got) != 1 || got[0].Path != "seeds" {
		t.Fatalf("overrides %+v", got)
	}
	write(`{"orient":{"treatment":{"overrides":[{"path":"seeds","treatment":"exclude"}]}}}`)
	if _, err := LoadOrientConfig(root); err == nil || !strings.Contains(err.Error(), "basis") {
		t.Fatalf("an override without a basis loaded: %v", err)
	}
}
