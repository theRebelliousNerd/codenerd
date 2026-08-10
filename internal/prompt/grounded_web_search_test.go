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

	for _, tc := range cases {
		atom, ok := provider.GetAtom(tc.intent)
		if !ok {
			t.Fatalf("missing config atom for %s", tc.intent)
		}
		has := slices.Contains(atom.Tools, "grounded_web_search")
		if has != tc.want {
			t.Errorf("intent %s grounded_web_search present=%v want %v tools=%v", tc.intent, has, tc.want, atom.Tools)
		}
		// no duplicates in this intent's tool list
		seen := make(map[string]int)
		for _, tool := range atom.Tools {
			seen[tool]++
			if seen[tool] > 1 {
				t.Errorf("intent %s has duplicate tool %q", tc.intent, tool)
			}
		}
	}

	// Researcher via SimpleRegistry also includes grounded_web_search
	registry := NewSimpleRegistry()
	RegisterDefaultConfigAtoms(registry)
	if atom, ok := registry.GetAtom("/researcher"); ok {
		if !slices.Contains(atom.Tools, "grounded_web_search") {
			t.Errorf("/researcher SimpleRegistry missing grounded_web_search tools=%v", atom.Tools)
		}
		if !slices.Contains(atom.Tools, "web_search") {
			t.Errorf("/researcher SimpleRegistry missing web_search")
		}
		// verify grounded after web_search for deterministic ordering
		idxGrounded := slices.Index(atom.Tools, "grounded_web_search")
		idxWebSearch := slices.Index(atom.Tools, "web_search")
		if idxGrounded < idxWebSearch {
			t.Errorf("grounded_web_search should appear after web_search for deterministic ordering, got %d vs %d", idxGrounded, idxWebSearch)
		}
	} else {
		t.Fatalf("SimpleRegistry missing /researcher")
	}
	if atom, ok := registry.GetAtom("/research"); ok {
		if !slices.Contains(atom.Tools, "grounded_web_search") {
			t.Errorf("/research SimpleRegistry missing grounded_web_search")
		}
	}

	// Factory Generate also respects same visibility
	factory := NewConfigFactory(provider)
	ctx := context.Background()
	res := &CompilationResult{Prompt: "identity"}
	for _, tc := range cases {
		cfg, err := factory.Generate(ctx, res, tc.intent)
		if err != nil {
			t.Fatalf("Generate %s failed: %v", tc.intent, err)
		}
		has := slices.Contains(cfg.AllowedTools, "grounded_web_search")
		if has != tc.want {
			t.Errorf("Generate %s grounded present=%v want %v tools=%v", tc.intent, has, tc.want, cfg.AllowedTools)
		}
		// deterministic: second call same order
		cfg2, _ := factory.Generate(ctx, res, tc.intent)
		if !slices.Equal(cfg.AllowedTools, cfg2.AllowedTools) {
			t.Errorf("Generate %s not deterministic: %v vs %v", tc.intent, cfg.AllowedTools, cfg2.AllowedTools)
		}
		// no duplicates after Generate
		seen := map[string]bool{}
		for _, tool := range cfg.AllowedTools {
			if seen[tool] {
				t.Errorf("Generate %s duplicate tool %q", tc.intent, tool)
			}
			seen[tool] = true
		}
	}

	// verificationTools == testerTools + grounded_web_search preserving tester prefix
	testerAtom, _ := provider.GetAtom("/test")
	verifyAtom, _ := provider.GetAtom("/verify")
	if len(verifyAtom.Tools) != len(testerAtom.Tools)+1 {
		t.Errorf("verificationTools length=%d want tester len+1=%d", len(verifyAtom.Tools), len(testerAtom.Tools)+1)
	}
	for i, tool := range testerAtom.Tools {
		if verifyAtom.Tools[i] != tool {
			t.Errorf("verificationTools order mismatch at %d: got %q want %q (must preserve tester prefix)", i, verifyAtom.Tools[i], tool)
			break
		}
	}
	if verifyAtom.Tools[len(verifyAtom.Tools)-1] != "grounded_web_search" {
		t.Errorf("verificationTools last tool should be grounded_web_search, got %q", verifyAtom.Tools[len(verifyAtom.Tools)-1])
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
