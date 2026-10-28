package prompt_evolution

import (
	"context"
	"errors"
	"testing"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/prompt"
)

// The promotion check holds an atom that describes one past task or goes
// against a promoted atom; with no Jev answer the usage threshold decides.
func TestPromotionHolds_TruthTable(t *testing.T) {
	ready := func(id, shard, content string) *GeneratedAtom {
		return &GeneratedAtom{Atom: &prompt.PromptAtom{ID: id, Content: content}, ShardType: shard, UsageCount: 5, SuccessCount: 5}
	}
	promotedAtom := func(id, shard, content string) *GeneratedAtom {
		ga := ready(id, shard, content)
		ga.PromotedAt = time.Now()
		return ga
	}
	off := false
	cases := []struct {
		name     string
		jev      *fakeJev
		sec      nerdconfig.TypeSafeConfig
		wantHeld bool
	}{
		{"general and consistent", &fakeJev{nouls: map[string]float64{learnjudge.OneTaskOnly: 0.05, "contradicts_0": 0.04}}, nerdconfig.TypeSafeConfig{}, false},
		{"one past task", &fakeJev{nouls: map[string]float64{learnjudge.OneTaskOnly: 0.9, "contradicts_0": 0.04}}, nerdconfig.TypeSafeConfig{}, true},
		{"contradicts a promoted atom", &fakeJev{nouls: map[string]float64{learnjudge.OneTaskOnly: 0.05, "contradicts_0": 0.92}}, nerdconfig.TypeSafeConfig{}, true},
		{"unsettled holds", &fakeJev{nouls: map[string]float64{learnjudge.OneTaskOnly: 0.5, "contradicts_0": 0.04}}, nerdconfig.TypeSafeConfig{}, true},
		{"jev failing: the threshold decides", &fakeJev{err: errors.New("503")}, nerdconfig.TypeSafeConfig{}, false},
		{"switch off: the threshold decides", &fakeJev{nouls: map[string]float64{learnjudge.OneTaskOnly: 0.99}}, nerdconfig.TypeSafeConfig{AtomChecks: &off}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pe := cascadeEvolver(t, &judgeCountingLLM{})
			pe.mu.Lock()
			pe.config.AutoPromote = true
			pe.evolvedAtoms["evolved/new"] = ready("evolved/new", "/coder", "Run the package's tests after every edit.")
			pe.evolvedAtoms["evolved/old"] = promotedAtom("evolved/old", "/coder", "Never run tests until the whole change is written.")
			pe.mu.Unlock()
			pe.SetJev(tc.jev, &tc.sec)
			held := pe.promotionHolds(context.Background())
			if _, got := held["evolved/new"]; got != tc.wantHeld {
				t.Fatalf("held = %v, want held %v", held, tc.wantHeld)
			}
			pe.mu.Lock()
			_, promoted := pe.autoPromoteAtoms(held)
			pe.mu.Unlock()
			if (promoted == 1) == tc.wantHeld {
				t.Fatalf("promoted %d with held %v", promoted, held)
			}
		})
	}
}

func TestPromotionCheckOn_DefersRecordTimePromotion(t *testing.T) {
	pe := cascadeEvolver(t, &judgeCountingLLM{})
	if pe.promotionCheckOn() {
		t.Fatal("no Jev configured, yet the check is on")
	}
	pe.SetJev(&fakeJev{}, &nerdconfig.TypeSafeConfig{})
	if !pe.promotionCheckOn() {
		t.Fatal("Jev configured with atom_checks on, yet record-time promotion would skip the check")
	}
}
