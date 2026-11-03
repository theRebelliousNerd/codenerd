package specs

import (
	"bufio"
	"errors"
	"strings"
	"testing"
)

const sampleSpec = `---
name: Login form
source: src/components/LoginForm.tsx
bindings:
  - { kind: component, target: LoginForm }
  - { kind: route, target: /login }
invariants:
  - name: no-visible-errors
    query: "user_visible_error(S, _, _, _)"
    expect: absent
---

# Login form

The login form reaches the dashboard without exposing credentials.

<!-- codenerd:invariant name=submit-gated from:42 to:80 expect:present -->
Submit stays disabled until fields validate.
` + "```query" + `
browser_page_state(S, _, false, false, _)
` + "```" + `
<!-- codenerd:end -->

<!-- browsernerd:invariant name=validation-no-error in:src/utils/validate.ts from:5 to:40 expect:absent -->
Validation does not log an error.
` + "```query" + `
console_event(S, "error", _, _)
` + "```" + `
<!-- browsernerd:end -->
`

func TestParseNativeAndCompatibleInvariants(t *testing.T) {
	doc, err := Parse("docs/specs/login.md", []byte(sampleSpec))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "Login form" || doc.Title != "Login form" || len(doc.Bindings) != 2 || len(doc.Invariants) != 3 {
		t.Fatalf("parsed spec = %+v", doc)
	}
	if doc.Invariants[0].Expect != "absent" || doc.Invariants[0].File != doc.Source {
		t.Fatalf("frontmatter invariant = %+v", doc.Invariants[0])
	}
	if !doc.Invariants[1].Covers("LoginForm.tsx", 60, 90) {
		t.Fatalf("native inline invariant does not cover range: %+v", doc.Invariants[1])
	}
	if doc.Invariants[2].File != "src/utils/validate.ts" || doc.Invariants[2].Query == "" {
		t.Fatalf("compatible inline invariant = %+v", doc.Invariants[2])
	}
}

func TestParseRejectsUnterminatedInvariant(t *testing.T) {
	content := "# Broken\n<!-- codenerd:invariant name=broken -->\nnever closed\n"
	if _, err := Parse("broken.md", []byte(content)); err == nil {
		t.Fatal("expected unterminated invariant error")
	}
}

func TestParseRejectsYAMLAliasesAndBoundsQueries(t *testing.T) {
	aliased := "---\ntitle: &title Login\nsummary: *title\n---\n# Login\n"
	if _, err := Parse("alias.md", []byte(aliased)); err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatalf("expected YAML alias rejection, got %v", err)
	}
	long := "---\ninvariants:\n  - name: long\n    query: \"" + strings.Repeat("x", 1000) + "\"\n    expect: maybe\n---\n# Long\n"
	doc, err := Parse("long.md", []byte(long))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Invariants) != 1 || len(doc.Invariants[0].Query) > 513 || len(doc.Invariants[0].Query) <= 512 {
		t.Fatalf("bounded invalid query = %d bytes", len(doc.Invariants[0].Query))
	}
	if doc.Invariants[0].Expect != "maybe" {
		t.Fatalf("invalid expect was silently normalized: %+v", doc.Invariants[0])
	}
}

func TestParseInvariantCountLimits(t *testing.T) {
	tests := []struct {
		name        string
		frontmatter int
		inline      int
		wantErr     bool
	}{
		{name: "frontmatter at limit", frontmatter: 500},
		{name: "frontmatter over limit", frontmatter: 501, wantErr: true},
		{name: "inline at limit", inline: 500},
		{name: "inline over limit", inline: 501, wantErr: true},
		{name: "combined at limit", frontmatter: 250, inline: 250},
		{name: "combined over limit", frontmatter: 250, inline: 251, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := "# Quota\n"
			if tt.frontmatter > 0 {
				content = "---\ninvariants:\n" + strings.Repeat("  - name: quota\n    query: ready(S)\n", tt.frontmatter) + "---\n" + content
			}
			content += strings.Repeat("<!-- codenerd:invariant name=quota -->\n```query\nready(S)\n```\n<!-- codenerd:end -->\n", tt.inline)
			doc, err := Parse("quota.md", []byte(content))
			if tt.wantErr {
				if err == nil {
					t.Fatal("Parse() accepted too many invariants")
				}
				if doc.Path != "" || len(doc.Invariants) != 0 {
					t.Fatalf("Parse() returned a partial spec on error: %+v", doc)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() at invariant limit: %v", err)
			}
			if got, want := len(doc.Invariants), tt.frontmatter+tt.inline; got != want {
				t.Fatalf("invariant count = %d, want %d", got, want)
			}
			for i, inv := range doc.Invariants {
				if inv.Query != "ready(S)" || inv.Expect != "present" || inv.Inline != (i >= tt.frontmatter) {
					t.Errorf("invariant %d lost query, default expectation or declaration order: %+v", i, inv)
				}
			}
		})
	}
}

func TestParseRejectsInvalidFrontmatter(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCause error
	}{
		{name: "unterminated block", content: "---\ntitle: Broken\n"},
		{name: "malformed yaml", content: "---\ntitle: [\n---\n# Broken\n"},
		{name: "wrong field type", content: "---\ntags: {unexpected: mapping}\n---\n# Broken\n"},
		{name: "depth limit", content: "---\nmetadata: " + strings.Repeat("[", 101) + "0" + strings.Repeat("]", 101) + "\n---\n# Broken\n"},
		{name: "node limit", content: "---\nmetadata: [" + strings.Repeat("0,", 10001) + "]\n---\n# Broken\n"},
		{name: "scanner token limit", content: strings.Repeat("x", hardMaxFileBytes+1), wantCause: bufio.ErrTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			doc, err := Parse("broken.md", []byte(tt.content))
			if err == nil {
				t.Fatal("Parse() accepted invalid frontmatter or an oversized line")
			}
			if tt.wantCause != nil && !errors.Is(err, tt.wantCause) {
				t.Errorf("Parse() error = %v, want cause %v", err, tt.wantCause)
			}
			if doc.Path != "" || doc.Body != "" || len(doc.Invariants) != 0 {
				t.Fatalf("Parse() returned a partial spec on error: %+v", doc)
			}
		})
	}
}

func TestParseInlineInvariantDefaultsAndSourceOffsets(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		attributes string
		want       Invariant
	}{
		{
			name:   "without frontmatter",
			prefix: "# Example\n\n",
			want:   Invariant{Name: "invariant_line_3", Expect: "present", Inline: true},
		},
		{
			name:   "frontmatter source and line offset",
			prefix: "---\nsource: src/default.go\n---\n# Example\n\n",
			want:   Invariant{Name: "invariant_line_6", File: "src/default.go", Expect: "present", Inline: true},
		},
		{
			name:       "invalid ranges default to zero",
			prefix:     "# Example\n\n",
			attributes: " from:not-a-number to=-5",
			want:       Invariant{Name: "invariant_line_3", Expect: "present", Inline: true},
		},
		{
			name:       "explicit attributes override defaults",
			prefix:     "---\nsource: src/default.go\n---\n# Example\n\n",
			attributes: " name=custom in:src/override.go from:2 to=7 expect=ABSENT",
			want:       Invariant{Name: "custom", File: "src/override.go", From: 2, To: 7, Expect: "absent", Inline: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			content := tt.prefix + "<!-- codenerd:invariant" + tt.attributes + " -->\nVisible state is ready.\n```query\nready(S)\n```\n<!-- codenerd:end -->\n"
			doc, err := Parse("example.md", []byte(content))
			if err != nil {
				t.Fatalf("Parse(): %v", err)
			}
			if len(doc.Invariants) != 1 {
				t.Fatalf("invariant count = %d, want 1", len(doc.Invariants))
			}
			want := tt.want
			want.Query = "ready(S)"
			want.Prose = "Visible state is ready."
			if got := doc.Invariants[0]; got != want {
				t.Errorf("invariant = %+v, want %+v", got, want)
			}
		})
	}
}
