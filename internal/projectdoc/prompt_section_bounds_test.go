package projectdoc

import (
	"strings"
	"testing"
)

// nerd.md is appended to the system prompt AFTER the JIT compiler has fitted
// and reported its budget, and nothing downstream re-measures it. The file is
// read with an unbounded os.ReadFile and the body is stored verbatim, so a
// generated or pasted-into nerd.md went to the provider whole, on every turn,
// while the compiler's "42% of budget used" line said otherwise.
func TestPromptSection_Bounds(t *testing.T) {
	rules := func(n int) []ForbidRule {
		out := make([]ForbidRule, n)
		for i := range out {
			out[i] = ForbidRule{Match: ".nerd/config.json", Reason: "kernel-protected"}
		}
		return out
	}
	lines := func(n int, s string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = s
		}
		return out
	}

	tests := []struct {
		name       string
		doc        *Document
		wantMarker bool
		mustKeep   []string
	}{
		{
			name: "a normal nerd.md is emitted verbatim",
			doc: &Document{
				Path: "nerd.md",
				Spec: Spec{
					Project:  "codenerd",
					Commands: Commands{Build: "go build ./...", Test: "go test ./..."},
					Forbid:   []ForbidRule{{Match: ".nerd/config.json", Reason: "kernel-protected"}},
				},
				Body: "Prose that is advisory only.",
			},
			mustKeep: []string{"codenerd", "go build ./...", ".nerd/config.json", "Prose that is advisory only."},
		},
		{
			name: "a multi-megabyte body is clamped head and tail",
			doc: &Document{
				Path: "nerd.md",
				Spec: Spec{Project: "big"},
				Body: "HEADRULE\n" + strings.Repeat("x", 4_000_000) + "\nTAILRULE",
			},
			wantMarker: true,
			// The tail of a project doc is where "one more thing, never touch
			// X" lives; head-only truncation loses exactly that.
			mustKeep: []string{"HEADRULE", "TAILRULE", "Project Instructions"},
		},
		{
			name: "thousands of forbid rules are capped",
			doc: &Document{
				Path: "nerd.md",
				Spec: Spec{Forbid: rules(10_000)},
			},
			wantMarker: true,
			mustKeep:   []string{"ENFORCED"},
		},
		{
			name: "thousands of requirements and conventions are capped",
			doc: &Document{
				Path: "nerd.md",
				Spec: Spec{
					Require:     lines(5_000, "run go test ./... before handoff"),
					Conventions: convs(5_000),
				},
			},
			wantMarker: true,
			mustKeep:   []string{"Required steps", "Conventions"},
		},
		{
			name: "nil document renders nothing",
			doc:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.doc.PromptSection()

			if len(got) > maxPromptSectionChars+512 {
				t.Errorf("section is %d chars, cap is %d", len(got), maxPromptSectionChars)
			}
			marked := strings.Contains(got, "[codenerd: truncated")
			if marked != tt.wantMarker {
				t.Errorf("truncation marker present = %v, want %v", marked, tt.wantMarker)
			}
			for _, want := range tt.mustKeep {
				if !strings.Contains(got, want) {
					t.Errorf("section lost %q", want)
				}
			}
		})
	}
}

// A truncated forbid list is a documentation loss, never a protection loss:
// the frontmatter is enforced by the kernel regardless of what is rendered.
// The marker has to say so, or the model will read a short list as a complete
// one and reason that an unlisted path is fair game.
func TestPromptSection_ForbidTruncationSaysEnforcementIsUnaffected(t *testing.T) {
	rules := make([]ForbidRule, 500)
	for i := range rules {
		rules[i] = ForbidRule{Match: "secret", Reason: "no"}
	}
	got := (&Document{Path: "nerd.md", Spec: Spec{Forbid: rules}}).PromptSection()

	if !strings.Contains(got, "ENFORCED by the kernel") {
		t.Errorf("forbid-list truncation marker must restate that enforcement is unaffected:\n%s",
			got[max(0, len(got)-400):])
	}
}

func convs(n int) []Convention {
	out := make([]Convention, n)
	for i := range out {
		out[i] = Convention{ID: "conventional-commits", Rule: "use conventional commits"}
	}
	return out
}
