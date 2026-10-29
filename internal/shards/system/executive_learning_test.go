package system

import (
	"testing"

	"codenerd/internal/types"
)

// fakeExecutiveLearningStore is a minimal fake implementing core.LearningStore
// for executive learning tests. It only needs LoadByPredicate for these tests
// but implements the full interface to satisfy the type.
type fakeExecutiveLearningStore struct {
	learnings map[string][]types.ShardLearning
	loadErr   map[string]error
}

func (f *fakeExecutiveLearningStore) Save(shardType, factPredicate string, factArgs []any, sourceCampaign string) error {
	return nil
}

func (f *fakeExecutiveLearningStore) SaveBatch(shardType string, learnings []types.ShardLearning, sourceCampaign string) error {
	return nil
}

func (f *fakeExecutiveLearningStore) Load(shardType string) ([]types.ShardLearning, error) {
	return nil, nil
}

func (f *fakeExecutiveLearningStore) LoadByPredicate(shardType, predicate string) ([]types.ShardLearning, error) {
	if shardType != "executive" {
		// Return empty if shardType is wrong - ensures caller uses "executive".
		return nil, nil
	}
	if f.loadErr != nil {
		if err, ok := f.loadErr[predicate]; ok && err != nil {
			return nil, err
		}
	}
	if f.learnings == nil {
		return nil, nil
	}
	return f.learnings[predicate], nil
}

func (f *fakeExecutiveLearningStore) DecayConfidence(shardType string, decayFactor float64) error {
	return nil
}

func (f *fakeExecutiveLearningStore) Close() error {
	return nil
}

func TestExecutivePolicyShard_loadLearnedPatterns(t *testing.T) {
	tests := []struct {
		name        string
		store       *fakeExecutiveLearningStore
		wantSuccess map[string]int
		wantFailure map[string]int
	}{
		{
			name: "success pattern int count 7 seeds 7",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"success_pattern": {
						{FactArgs: []any{"patternA", 7}},
					},
				},
			},
			wantSuccess: map[string]int{"patternA": 7},
		},
		{
			name: "failure pattern int64 count seeds int64 value",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"failure_pattern": {
						{FactArgs: []any{"patternB", "some reason", int64(9)}},
					},
				},
			},
			wantFailure: map[string]int{"patternB": 9},
		},
		{
			name: "float64 count seeds truncated int success",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"success_pattern": {
						{FactArgs: []any{"patternC", float64(8.9)}},
					},
				},
			},
			wantSuccess: map[string]int{"patternC": 8},
		},
		{
			name: "float64 count seeds truncated int failure",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"failure_pattern": {
						{FactArgs: []any{"patternD", "reason", float64(6.7)}},
					},
				},
			},
			wantFailure: map[string]int{"patternD": 6},
		},
		{
			name: "missing count fallback success to 5",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"success_pattern": {
						{FactArgs: []any{"onlyPattern"}},
					},
				},
			},
			wantSuccess: map[string]int{"onlyPattern": 5},
		},
		{
			name: "missing count fallback failure to 3",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"failure_pattern": {
						{FactArgs: []any{"failPat", "reason"}},
					},
				},
			},
			wantFailure: map[string]int{"failPat": 3},
		},
		{
			name: "missing count fallback failure short args to 3",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"failure_pattern": {
						{FactArgs: []any{"failShort"}},
					},
				},
			},
			wantFailure: map[string]int{"failShort": 3},
		},
		{
			name: "non-string FactArgs0 is skipped success",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"success_pattern": {
						{FactArgs: []any{123, 7}},
						{FactArgs: []any{"valid", 5}},
					},
				},
			},
			wantSuccess: map[string]int{"valid": 5},
		},
		{
			name: "non-string FactArgs0 is skipped failure",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"failure_pattern": {
						{FactArgs: []any{456, "reason", 3}},
						{FactArgs: []any{"validFail", "reason", 4}},
					},
				},
			},
			wantFailure: map[string]int{"validFail": 4},
		},
		{
			name: "empty string FactArgs0 is skipped",
			store: &fakeExecutiveLearningStore{
				learnings: map[string][]types.ShardLearning{
					"success_pattern": {
						{FactArgs: []any{"", 7}},
						{FactArgs: []any{"kept", 2}},
					},
					"failure_pattern": {
						{FactArgs: []any{"", "reason", 3}},
						{FactArgs: []any{"keptFail", "reason", 2}},
					},
				},
			},
			wantSuccess: map[string]int{"kept": 2},
			wantFailure: map[string]int{"keptFail": 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shard := NewExecutivePolicyShard()
			shard.SetLearningStore(tt.store)

			// Verify success patterns
			for wantPat, wantCount := range tt.wantSuccess {
				if got, ok := shard.patternSuccess[wantPat]; !ok {
					t.Errorf("expected success pattern %q to be seeded, but not found (have %v)", wantPat, shard.patternSuccess)
				} else if got != wantCount {
					t.Errorf("success pattern %q count = %d, want %d", wantPat, got, wantCount)
				}
			}
			// Also ensure non-string/empty patterns were not added
			if tt.wantSuccess != nil {
				for pat := range shard.patternSuccess {
					if _, ok := tt.wantSuccess[pat]; !ok {
						// If test expects only specific patterns, any extra is unexpected
						// But allow for previous patterns if multiple learnings were present
						// So just check that unwanted int-key patterns are absent
						if pat == "" {
							t.Errorf("empty pattern should not be seeded")
						}
					}
				}
			}
			for wantPat, wantCount := range tt.wantFailure {
				if got, ok := shard.patternFailure[wantPat]; !ok {
					t.Errorf("expected failure pattern %q to be seeded, but not found (have %v)", wantPat, shard.patternFailure)
				} else if got != wantCount {
					t.Errorf("failure pattern %q count = %d, want %d", wantPat, got, wantCount)
				}
			}
			// Verify non-string skipped cases: ensure only wanted patterns exist
			if tt.name == "non-string FactArgs0 is skipped success" {
				if _, ok := shard.patternSuccess["123"]; ok {
					t.Error("non-string pattern should be skipped")
				}
				if len(shard.patternSuccess) != 1 {
					t.Errorf("expected 1 success pattern, got %d: %v", len(shard.patternSuccess), shard.patternSuccess)
				}
			}
			if tt.name == "non-string FactArgs0 is skipped failure" {
				if len(shard.patternFailure) != 1 {
					t.Errorf("expected 1 failure pattern, got %d: %v", len(shard.patternFailure), shard.patternFailure)
				}
			}
		})
	}
}

func TestExecutivePolicyShard_SetLearningStore_NilStore_NoOp(t *testing.T) {
	shard := NewExecutivePolicyShard()
	// Pre-seed a pattern to ensure nil store does not clear it and does not panic.
	shard.patternSuccess["pre"] = 1
	shard.patternFailure["preFail"] = 2

	shard.SetLearningStore(nil)

	if shard.learningStore != nil {
		t.Error("learningStore should be nil after SetLearningStore(nil)")
	}
	// Maps should be unchanged (no load happened, no panic).
	if got := shard.patternSuccess["pre"]; got != 1 {
		t.Errorf("patternSuccess[pre] = %d, want 1 after nil store", got)
	}
	if got := shard.patternFailure["preFail"]; got != 2 {
		t.Errorf("patternFailure[preFail] = %d, want 2 after nil store", got)
	}
}

func TestExecutivePolicyShard_SetLearningStore_EmptyStore_NoOp(t *testing.T) {
	shard := NewExecutivePolicyShard()
	store := &fakeExecutiveLearningStore{
		learnings: map[string][]types.ShardLearning{},
	}
	shard.SetLearningStore(store)
	if len(shard.patternSuccess) != 0 {
		t.Errorf("expected no success patterns, got %v", shard.patternSuccess)
	}
	if len(shard.patternFailure) != 0 {
		t.Errorf("expected no failure patterns, got %v", shard.patternFailure)
	}
}
