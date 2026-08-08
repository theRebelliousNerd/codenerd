package session

import (
	"context"
	"testing"

	"codenerd/internal/prompt"
)

// This file guards the executor half of user-defined-agent wiring: turning an
// intent verb into the shard dimensions that select a custom agent's prompt
// atoms. The other half — that those atoms actually reach a compiled prompt —
// is covered with a real Mangle kernel and a real JIT compiler in
// internal/system/user_agent_prompt_test.go.

// TestUserAgentFromIntentVerb pins the verb -> agent-name extraction.
//
// Regression guarded: this is the only thing that tells the compilation context
// which per-agent atom DB to open. It must accept both shapes a user agent is
// reachable by — "/consult/<name>" from chat delegation
// (cmd/nerd/chat/delegation_routing.go personaToIntent) and "/<name>" from
// `nerd spawn <name>` via normalizeTaskIntentVerb — and must reject structured
// verbs so a multi-segment verb is never mistaken for an agent.
func TestUserAgentFromIntentVerb(t *testing.T) {
	cases := []struct {
		verb string
		want string
	}{
		{"/consult/RustExpert", "rustexpert"},
		{"/consult/goexpert", "goexpert"},
		{"/rustexpert", "rustexpert"},
		{"/GoExpert", "goexpert"},
		// Taxonomy verbs are single segments too; the CALLER checks
		// perception.GetShardTypeForVerb first (see buildCompilationContext),
		// which is what keeps "/fix" on the coder persona.
		{"/fix", "fix"},
		{"", ""},
		{"rustexpert", ""},  // not a verb: no leading slash
		{"/a/b/c", ""},      // structured verb, not an agent name
		{"/two words", ""},  // whitespace is never part of an agent name
		{"/", ""},
	}

	for _, tc := range cases {
		if got := UserAgentFromIntentVerb(tc.verb); got != tc.want {
			t.Errorf("UserAgentFromIntentVerb(%q) = %q, want %q", tc.verb, got, tc.want)
		}
	}
}

// TestBuiltinVerbsKeepTheirPersona guards the branch ORDER in
// buildCompilationContext: taxonomy verbs must resolve their persona through
// perception.GetShardTypeForVerb, never through the user-agent fallback.
//
// Regression guarded: if the user-agent branch were checked first, "/review"
// would set ShardID="review" instead of "reviewer", and the reviewer persona
// atoms would silently stop being selected — turning a core verb generic.
func TestBuiltinVerbsKeepTheirPersona(t *testing.T) {
	// Query-category verbs only: mutation verbs trip the hollow-success gate
	// when the mock LLM returns text without calling a tool, which is correct
	// behavior and unrelated to what this test asserts.
	cases := map[string]string{
		"/review":   "reviewer",
		"/research": "researcher",
		"/explore":  "researcher",
	}

	for verb, wantShard := range cases {
		var seen *prompt.CompilationContext
		compiler := &MockJITCompiler{
			CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				seen = cc
				return &prompt.CompilationResult{Prompt: "p"}, nil
			},
		}
		exec := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{},
			compiler, &MockConfigFactory{}, &MockTransducer{})

		if _, err := exec.ProcessWithIntent(context.Background(), "do it",
			presetIntentForTask(verb, "do it", "")); err != nil {
			t.Fatalf("ProcessWithIntent(%s): %v", verb, err)
		}
		if seen == nil {
			t.Fatalf("%s: compiler never called", verb)
		}
		if seen.ShardID != wantShard {
			t.Errorf("%s: ShardID = %q, want %q (built-in taxonomy must win over the user-agent fallback)",
				verb, seen.ShardID, wantShard)
		}
	}
}

// TestUserAgentSetsShardContext asserts a custom verb populates BOTH shard
// dimensions of the compilation context.
//
// Regression guarded: ShardID selects the agent's per-agent atom DB
// (internal/prompt/compiler.go collectAtomsWithStats); ShardType drives
// jit_compiler.mg's blocked_by_context. Before this fix neither was set for a
// custom agent, which caused two independent failures at once: the agent's own
// prompts.yaml atoms were never read, AND with the shard dimension absent every
// shard-gated atom in the corpus was admitted, handing the agent 25+
// contradictory built-in identities. Dropping either assignment reintroduces
// one of them.
func TestUserAgentSetsShardContext(t *testing.T) {
	for _, verb := range []string{"/consult/BubbleTeaExpert", "/bubbleteaexpert"} {
		var seen *prompt.CompilationContext
		compiler := &MockJITCompiler{
			CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				seen = cc
				return &prompt.CompilationResult{Prompt: "p"}, nil
			},
		}
		exec := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{},
			compiler, &MockConfigFactory{}, &MockTransducer{})

		if _, err := exec.ProcessWithIntent(context.Background(), "advise",
			presetIntentForTask(verb, "advise", "")); err != nil {
			t.Fatalf("ProcessWithIntent(%s): %v", verb, err)
		}
		if seen == nil {
			t.Fatalf("%s: compiler never called", verb)
		}
		if seen.ShardID != "bubbleteaexpert" {
			t.Errorf("%s: ShardID = %q, want %q (selects .nerd/shards/bubbleteaexpert_knowledge.db)",
				verb, seen.ShardID, "bubbleteaexpert")
		}
		if seen.ShardType != "/bubbleteaexpert" {
			t.Errorf("%s: ShardType = %q, want %q (gates out other personas' atoms)",
				verb, seen.ShardType, "/bubbleteaexpert")
		}
	}
}
