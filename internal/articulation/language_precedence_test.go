package articulation

import (
	"testing"

	"codenerd/internal/types"
)

// Two sources can name a language for a turn, and which one wins is not a
// detail: prompt atoms fail closed on language, so this decides whether the
// model is handed Go advice or Python advice.
//
// The general fact is the workspace's dominant language, which the world scan
// derives and the session context now carries. The specific fact is the file
// the turn is actually about, inferred from the intent target's extension.
// Specific wins -- editing a .py script inside a Go repository is a Python
// problem, whatever the rest of the tree is written in.
//
// This is pinned in both directions because it has already inverted once. The
// target inference was guarded on "language not already set", which was
// equivalent to "always" for as long as nothing populated a language earlier.
// The moment the session context started carrying the project language, that
// guard silently turned the more precise source off, and nothing failed --
// the prompt just quietly became the wrong one.
func TestLanguagePrecedence(t *testing.T) {
	pa := &PromptAssembler{}

	tests := []struct {
		name        string
		projectLang string
		target      string
		want        string
	}{
		{
			name:        "target file beats the project language",
			projectLang: "/go",
			target:      "scripts/analyze.py",
			want:        "/python",
		},
		{
			name:        "project language stands when the turn names no file",
			projectLang: "/go",
			target:      "the context compressor",
			want:        "/go",
		},
		{
			name:        "an unrecognised extension must not erase the project language",
			projectLang: "/go",
			target:      "README.md",
			want:        "/go",
		},
		{
			name:        "target file alone still works with no project language",
			projectLang: "",
			target:      "internal/broker/wrap.go",
			want:        "/go",
		},
		{
			name:        "neither source means no language, not a guess",
			projectLang: "",
			target:      "make it faster",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionCtx := &types.SessionContext{ExtraContext: map[string]string{}}
			if tt.projectLang != "" {
				sessionCtx.ExtraContext["language"] = tt.projectLang
			}

			cc := pa.toCompilationContext(&PromptContext{
				ShardID:    "coder",
				ShardType:  "coder",
				SessionCtx: sessionCtx,
				UserIntent: &types.StructuredIntent{Verb: "/fix", Target: tt.target},
			})
			if cc == nil {
				t.Fatal("toCompilationContext returned nil")
			}
			if cc.Language != tt.want {
				t.Errorf("Language = %q, want %q (project=%q target=%q)",
					cc.Language, tt.want, tt.projectLang, tt.target)
			}
		})
	}
}

// A failing suite has to arrive as two separate signals, and the pair is what
// the TDD atoms need: TestState puts the compile into /tdd_repair, and the
// count of failing tests arms the failing_tests world state. The three entries
// in methodology/tdd.yaml are gated on BOTH, so either one missing is the same
// as neither.
//
// This is the compile half. The producer half -- that a tester shard's output
// reaches these two fields at all -- is asserted in cmd/nerd/chat. Both halves
// were broken, and separately: nothing in the repository wrote either field,
// and the code that meant to read the test results queried a predicate nothing
// asserts, at the wrong argument index, through an assertion that could not
// have matched anyway.
func TestFailingTestsProduceRepairModeAndWorldState(t *testing.T) {
	pa := &PromptAssembler{}

	cc := pa.toCompilationContext(&PromptContext{
		ShardID:   "coder",
		ShardType: "coder",
		SessionCtx: &types.SessionContext{
			ExtraContext: map[string]string{},
			TestState:    "/failing",
			FailingTests: []string{"TestBeta", "TestGamma/empty_input"},
		},
	})
	if cc == nil {
		t.Fatal("toCompilationContext returned nil")
	}

	if cc.OperationalMode != "/tdd_repair" {
		t.Errorf("OperationalMode = %q, want \"/tdd_repair\"", cc.OperationalMode)
	}
	if cc.FailingTestCount != 2 {
		t.Errorf("FailingTestCount = %d, want 2", cc.FailingTestCount)
	}

	var sawFailingTests bool
	for _, state := range cc.WorldStates() {
		if state == "failing_tests" {
			sawFailingTests = true
		}
	}
	if !sawFailingTests {
		t.Errorf("world states %v omit failing_tests; the TDD atoms are gated on it as well as "+
			"on the operational mode, so they still would not load", cc.WorldStates())
	}
}

// And a session with no test verdict must stay in /active. /tdd_repair
// displaces the default mode rather than adding to it, so a false positive
// changes which operational mode the entire compile selects for.
func TestNoTestVerdictLeavesTheModeActive(t *testing.T) {
	pa := &PromptAssembler{}

	cc := pa.toCompilationContext(&PromptContext{
		ShardID:    "coder",
		ShardType:  "coder",
		SessionCtx: &types.SessionContext{ExtraContext: map[string]string{}},
	})
	if cc.OperationalMode != "/active" {
		t.Errorf("OperationalMode = %q, want \"/active\"", cc.OperationalMode)
	}
	if cc.FailingTestCount != 0 {
		t.Errorf("FailingTestCount = %d, want 0", cc.FailingTestCount)
	}
}
