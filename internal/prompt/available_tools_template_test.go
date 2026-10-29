package prompt

import (
	"strings"
	"testing"
)

// A granted tool the request cannot carry is named with its reason and never
// listed as callable (Docs/audits/request-visible-tool-contract-and-
// investigation-affordances.md, T3).
func TestAvailableToolsTemplate_NamesGrantedButUnavailableTools(t *testing.T) {
	te := NewTemplateEngine()
	out := te.Process("{{available_tools}}", &CompilationContext{
		AvailableTools:   []string{"read_file"},
		UnavailableTools: []string{"grounded_web_search: no backend is registered for it on this runtime"},
	})
	if !strings.Contains(out, "- `read_file`") {
		t.Fatalf("callable tool missing:\n%s", out)
	}
	if strings.Contains(out, "`grounded_web_search`") {
		t.Fatalf("an unavailable grant was listed as callable:\n%s", out)
	}
	if !strings.Contains(out, "Granted but not callable this turn:\n- grounded_web_search: no backend") {
		t.Fatalf("unavailable grant not disclosed:\n%s", out)
	}
	clone := (&CompilationContext{UnavailableTools: []string{"x: y"}}).Clone()
	if len(clone.UnavailableTools) != 1 {
		t.Fatalf("clone dropped UnavailableTools")
	}
}
