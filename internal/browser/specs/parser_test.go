package specs

import (
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
