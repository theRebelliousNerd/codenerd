package prompt

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestGroundedWebSearch_ConfigVisibility verifies JIT visibility of grounded_web_search:
// - /research and /verify include it
// - /test and /fix do NOT
// - no duplicates and deterministic ordering
func TestGroundedWebSearch_ConfigVisibility(t *testing.T) {
	provider := NewDefaultConfigAtomProvider()

	cases := []struct {
		intent string
		want   bool
	}{
		{"/research", true},
		{"/explore", true},
		{"/verify", true},
		{"/validate", true},
		{"/test", false},
		{"/benchmark", false},
		{"/profile", false},
		{"/fix", false},
		{"/refactor", false},
	}

	k := testTurnKernel(t)
	for _, tc := range cases {
		if _, ok := provider.GetAtom(tc.intent); !ok {
			t.Fatalf("missing config atom for %s", tc.intent)
		}
		tools, err := DeriveTurnTools(k, tc.intent)
		if err != nil {
			t.Fatalf("DeriveTurnTools(%s): %v", tc.intent, err)
		}
		has := slices.Contains(tools, "grounded_web_search")
		if has != tc.want {
			t.Errorf("intent %s grounded_web_search present=%v want %v tools=%v", tc.intent, has, tc.want, tools)
		}
		seen := make(map[string]int)
		for _, tool := range tools {
			seen[tool]++
			if seen[tool] > 1 {
				t.Errorf("intent %s has duplicate tool %q", tc.intent, tool)
			}
		}
		again, err := DeriveTurnTools(k, tc.intent)
		if err != nil {
			t.Fatalf("DeriveTurnTools(%s) again: %v", tc.intent, err)
		}
		if !slices.Equal(tools, again) {
			t.Errorf("intent %s not deterministic: %v vs %v", tc.intent, tools, again)
		}
	}

	// The default atoms carry policies, not a second tool catalog.
	factory := NewConfigFactory(provider)
	cfg, err := factory.Generate(context.Background(), &CompilationResult{Prompt: "identity"}, "/verify")
	if err != nil {
		t.Fatalf("Generate /verify: %v", err)
	}
	if len(cfg.AllowedTools) != 0 {
		t.Errorf("Generate granted tools %v; the catalog is turn_tool_allowed", cfg.AllowedTools)
	}

	// /verify is the tester envelope plus grounded_web_search, as a set.
	// The consumer sorts, so the old prefix-order assertion does not apply.
	tester, err := DeriveTurnTools(k, "/test")
	if err != nil {
		t.Fatal(err)
	}
	verify, err := DeriveTurnTools(k, "/verify")
	if err != nil {
		t.Fatal(err)
	}
	want := append(slices.Clone(tester), "grounded_web_search")
	slices.Sort(want)
	if !slices.Equal(verify, want) {
		t.Errorf("verify = %v, want tester plus grounded_web_search %v", verify, want)
	}
}

// TestGroundedWebSearch_EmbeddedAndMatchesContext verifies the atom is embedded and selector-gated.
func TestGroundedWebSearch_EmbeddedAndMatchesContext(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus failed: %v", err)
	}
	atom, ok := corpus.Get("capability/grounded_web_search")
	if !ok {
		t.Fatalf("embedded atom capability/grounded_web_search not found")
	}
	if atom.IsMandatory {
		t.Errorf("grounded atom should be optional")
	}
	// Check selectors gated to research/explore/verify/validate and researcher/tester
	wantVerbs := []string{"research", "explore", "verify", "validate"}
	for _, v := range wantVerbs {
		if !slices.Contains(atom.IntentVerbs, v) {
			t.Errorf("atom IntentVerbs missing %q got %v", v, atom.IntentVerbs)
		}
	}
	wantShards := []string{"researcher", "tester"}
	for _, s := range wantShards {
		if !slices.Contains(atom.ShardTypes, s) {
			t.Errorf("atom ShardTypes missing %q got %v", s, atom.ShardTypes)
		}
	}
	if slices.Contains(atom.IntentVerbs, "test") {
		t.Errorf("atom should not be gated to /test, got %v", atom.IntentVerbs)
	}
	if slices.Contains(atom.IntentVerbs, "fix") {
		t.Errorf("atom should not be gated to /fix, got %v", atom.IntentVerbs)
	}

	// Content sanity: must mention catalog, single precise query, citations/usage, never hidden reasoning, fallback
	contentLower := strings.ToLower(atom.Content)
	for _, needle := range []string{
		"grounded_web_search",
		"precise",
		"citations",
		"cite sources",
		"never request or expose hidden reasoning",
		"fall back",
	} {
		if !strings.Contains(contentLower, needle) {
			t.Errorf("atom content missing %q", needle)
		}
	}

	// MatchesContext: should match research/researcher and verify/tester, not test/tester or fix/coder
	tests := []struct {
		name      string
		cc        *CompilationContext
		wantMatch bool
	}{
		{
			name:      "research researcher matches",
			cc:        NewCompilationContext().WithIntent("/research", "").WithShard("/researcher", "", ""),
			wantMatch: true,
		},
		{
			name:      "explore researcher matches",
			cc:        NewCompilationContext().WithIntent("/explore", "").WithShard("/researcher", "", ""),
			wantMatch: true,
		},
		{
			name:      "verify tester matches",
			cc:        NewCompilationContext().WithIntent("/verify", "").WithShard("/tester", "", ""),
			wantMatch: true,
		},
		{
			name:      "validate tester matches",
			cc:        NewCompilationContext().WithIntent("/validate", "").WithShard("/tester", "", ""),
			wantMatch: true,
		},
		{
			name:      "test tester does not match",
			cc:        NewCompilationContext().WithIntent("/test", "").WithShard("/tester", "", ""),
			wantMatch: false,
		},
		{
			name:      "fix coder does not match",
			cc:        NewCompilationContext().WithIntent("/fix", "").WithShard("/coder", "", ""),
			wantMatch: false,
		},
		{
			name:      "research coder does not match shard gating",
			cc:        NewCompilationContext().WithIntent("/research", "").WithShard("/coder", "", ""),
			wantMatch: false,
		},
	}
	for _, tt := range tests {
		got := atom.MatchesContext(tt.cc)
		if got != tt.wantMatch {
			t.Errorf("MatchesContext %s got %v want %v (verb=%s shard=%s)", tt.name, got, tt.wantMatch, tt.cc.IntentVerb, tt.cc.ShardType)
		}
	}
}
