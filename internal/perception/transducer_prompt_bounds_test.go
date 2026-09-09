package perception

import (
	"strings"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The classification prompt runs on EVERY turn before any work happens, and
// every input it interpolates is attacker- or accident-controlled: an editor
// "select all", a campaign-injected strategic blob, exemplars recalled from
// whatever was typed, and five prior turns. ParseIntentWithContext caps the
// raw input at 50 KB and nothing else was capped at all, so a chat that had
// pasted a large log replayed it into every subsequent classification call.
func TestBuildPrompt_Bounds(t *testing.T) {
	huge := strings.Repeat("Q", 400_000)

	tests := []struct {
		name       string
		build      func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string)
		wantMarker bool
		mustKeep   []string
	}{
		{
			name: "an editor select-all is clamped",
			build: func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string) {
				return "fix it", nil, nil, &types.SessionContext{
					Ambient: &types.AmbientContext{
						ActiveFile:   "main.go",
						SelectedText: "HEADMARK" + huge + "TAILMARK",
					},
				}, ""
			},
			wantMarker: true,
			mustKeep:   []string{"main.go", "HEADMARK", "TAILMARK", "fix it"},
		},
		{
			name: "diagnostics are capped in count and in line length",
			build: func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string) {
				diags := make([]string, 500)
				for i := range diags {
					diags[i] = strings.Repeat("d", 5000)
				}
				return "fix it", nil, nil, &types.SessionContext{
					Ambient: &types.AmbientContext{Diagnostics: diags},
				}, ""
			},
			wantMarker: true,
			mustKeep:   []string{"fix it"},
		},
		{
			name: "strategic context is clamped",
			build: func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string) {
				return "fix it", nil, nil, nil, huge
			},
			wantMarker: true,
			mustKeep:   []string{"Strategic Context", "fix it"},
		},
		{
			name: "recalled exemplars are capped in count and length",
			build: func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string) {
				matches := make([]SemanticMatch, 200)
				for i := range matches {
					matches[i] = SemanticMatch{
						TextContent: strings.Repeat("m", 20_000),
						Verb:        "/fix",
						Similarity:  0.99,
					}
				}
				return "fix it", nil, matches, nil, ""
			},
			wantMarker: true,
			mustKeep:   []string{"Learned Semantic Matches", "fix it"},
		},
		{
			name: "an oversized prior turn is clamped head and tail",
			build: func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string) {
				return "fix it", []ConversationTurn{
					{Role: "user", Content: "HEADMARK" + huge + "TAILMARK", ThoughtSummary: huge},
				}, nil, nil, ""
			},
			wantMarker: true,
			mustKeep:   []string{"HEADMARK", "TAILMARK", "fix it"},
		},
		{
			name: "an ordinary turn is untouched",
			build: func() (string, []ConversationTurn, []SemanticMatch, *types.SessionContext, string) {
				return "rename the handler", []ConversationTurn{
					{Role: "user", Content: "what does this do?"},
					{Role: "assistant", Content: "it routes requests"},
				}, nil, nil, ""
			},
			mustKeep: []string{"what does this do?", "it routes requests", "rename the handler"},
		},
	}

	tr := NewLLMTransducer(nil, nil, "system")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, history, matches, sctx, strategic := tt.build()
			got := tr.BuildPrompt(input, history, matches, sctx, strategic)

			// The prompt is the sum of five capped sections plus the (already
			// 50 KB-capped) input. This ceiling is what makes classification
			// cost predictable regardless of what the workspace throws at it.
			const ceiling = 128 * 1024
			if len(got) > ceiling {
				t.Errorf("classification prompt is %d chars, want <= %d", len(got), ceiling)
			}
			if prompt.IsClamped(got) != tt.wantMarker {
				t.Errorf("IsClamped = %v, want %v", prompt.IsClamped(got), tt.wantMarker)
			}
			for _, want := range tt.mustKeep {
				if !strings.Contains(got, want) {
					t.Errorf("prompt lost %q", want)
				}
			}
		})
	}
}

// Exemplars below the similarity gate must not consume the exemplar cap:
// counting scanned positions rather than written entries would let a run of
// weak hits suppress the strong ones behind them.
func TestBuildPrompt_ExemplarCapCountsWrittenNotScanned(t *testing.T) {
	matches := make([]SemanticMatch, 0, maxSemanticExemplars+20)
	for i := 0; i < maxSemanticExemplars+20; i++ {
		matches = append(matches, SemanticMatch{TextContent: "weak", Similarity: 0.1})
	}
	matches = append(matches, SemanticMatch{TextContent: "STRONGHIT", Verb: "/fix", Similarity: 0.95})

	got := NewLLMTransducer(nil, nil, "system").BuildPrompt("go", nil, matches, nil, "")
	if !strings.Contains(got, "STRONGHIT") {
		t.Error("a high-similarity exemplar was suppressed by low-similarity ones ahead of it")
	}
}
