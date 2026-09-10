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
