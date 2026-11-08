package campaign

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRecurseStatusJSON_ConflictsMapping(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	longCommit := "abcdef1234567890xyz"
	st := RecurseStatus{
		Running: true,
		Pass:    1,
		Base: RecurseBaseStatus{
			Branch:     "main",
			TipAtStart: "tipcommit1234567890",
			Conflicts: []RecurseConflict{
				{Time: now, BaseCommit: longCommit, Detail: "merge conflicted"},
				{Time: now, BaseCommit: "", Detail: "empty base commit stays empty"},
			},
		},
	}
	got := st.ToRecurseStatusJSON()
	if len(got.Base.Conflicts) != len(st.Base.Conflicts) {
		t.Fatalf("conflicts len = %d, want %d", len(got.Base.Conflicts), len(st.Base.Conflicts))
	}
	for i, want := range st.Base.Conflicts {
		if got.Base.Conflicts[i].BaseCommit != want.BaseCommit {
			t.Errorf("conflict %d BaseCommit = %q, want %q", i, got.Base.Conflicts[i].BaseCommit, want.BaseCommit)
		}
		if want.BaseCommit == "" {
			if got.Base.Conflicts[i].ShortCommit != "" {
				t.Errorf("conflict %d ShortCommit = %q, want empty for empty commit", i, got.Base.Conflicts[i].ShortCommit)
			}
		} else if got.Base.Conflicts[i].ShortCommit != shortCommit(want.BaseCommit) {
			t.Errorf("conflict %d ShortCommit = %q, want %q", i, got.Base.Conflicts[i].ShortCommit, shortCommit(want.BaseCommit))
		}
		if got.Base.Conflicts[i].Detail != want.Detail {
			t.Errorf("conflict %d Detail = %q, want %q", i, got.Base.Conflicts[i].Detail, want.Detail)
		}
		if !got.Base.Conflicts[i].Time.Equal(want.Time) {
			t.Errorf("conflict %d Time = %v, want %v", i, got.Base.Conflicts[i].Time, want.Time)
		}
	}
	if got.Base.Conflicts[0].ShortCommit == longCommit {
		t.Errorf("ShortCommit was not shortened: %q", got.Base.Conflicts[0].ShortCommit)
	}
	if got.Base.Tip != st.Base.TipAtStart || got.Base.TipShort != shortCommit(st.Base.TipAtStart) {
		t.Errorf("tip = %q/%q, want %q/%q", got.Base.Tip, got.Base.TipShort, st.Base.TipAtStart, shortCommit(st.Base.TipAtStart))
	}
}

func TestRecurseStatusJSON_GenerationsMapping(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Second)
	st := RecurseStatus{
		Pass: 1,
		Base: RecurseBaseStatus{
			Branch: "main",
			Generations: []RecurseGeneration{
				{Time: now, Commit: "feedface1234567890", Binary: "/tmp/nerd-new"},
				{Time: now, Commit: "deadbeef1234567890", Failure: "build failed: vet red"},
				{Time: now, Commit: "", Binary: ""},
			},
		},
	}
	got := st.ToRecurseStatusJSON()
	if len(got.Base.Generations) != len(st.Base.Generations) {
		t.Fatalf("generations len = %d, want %d", len(got.Base.Generations), len(st.Base.Generations))
	}
	for i, want := range st.Base.Generations {
		if got.Base.Generations[i].Commit != want.Commit {
			t.Errorf("generation %d Commit = %q, want %q", i, got.Base.Generations[i].Commit, want.Commit)
		}
		if got.Base.Generations[i].ShortCommit != shortCommit(want.Commit) {
			t.Errorf("generation %d ShortCommit = %q, want %q", i, got.Base.Generations[i].ShortCommit, shortCommit(want.Commit))
		}
		if got.Base.Generations[i].Binary != want.Binary {
			t.Errorf("generation %d Binary = %q, want %q", i, got.Base.Generations[i].Binary, want.Binary)
		}
		if got.Base.Generations[i].Failure != want.Failure {
			t.Errorf("generation %d Failure = %q, want %q", i, got.Base.Generations[i].Failure, want.Failure)
		}
		if !got.Base.Generations[i].Time.Equal(want.Time) {
			t.Errorf("generation %d Time = %v, want %v", i, got.Base.Generations[i].Time, want.Time)
		}
	}
	if got.Base.Generations[0].Binary == "" {
		t.Errorf("success generation lost its Binary path")
	}
	if got.Base.Generations[1].Failure == "" {
		t.Errorf("failed generation lost its Failure text")
	}
	if got.Base.Generations[0].ShortCommit == st.Base.Generations[0].Commit {
		t.Errorf("generation ShortCommit was not shortened: %q", got.Base.Generations[0].ShortCommit)
	}

	raw, err := st.MarshalRecurseStatusJSON()
	if err != nil {
		t.Fatalf("MarshalRecurseStatusJSON: %v", err)
	}
	if len(raw) == 0 {
		t.Fatalf("MarshalRecurseStatusJSON returned empty bytes")
	}
	var decoded RecurseStatusJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal status JSON: %v", err)
	}
	if len(decoded.Base.Generations) != len(st.Base.Generations) {
		t.Fatalf("decoded generations len = %d, want %d", len(decoded.Base.Generations), len(st.Base.Generations))
	}
	if decoded.Base.Generations[1].Failure != "build failed: vet red" {
		t.Errorf("decoded failure = %q, want failure text preserved", decoded.Base.Generations[1].Failure)
	}
}
