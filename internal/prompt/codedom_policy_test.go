package prompt

import (
	"strings"
	"testing"
)

// TestCodeDOMEditingPolicy loads the production embedded prompt corpus and
// enforces the CodeDOM editing contract distributed across five atoms.
// Policy is split across identity/coder/codedom_premier, identity/coder/tool_usage,
// capability/codedom_first, capability/codedom_selection, capability/codedom_tools.
// Checks use resilient case-insensitive substring matching so minor wording
// changes do not break the suite while policy drift still fails loudly.
func TestCodeDOMEditingPolicy(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus failed (production embedded corpus must load): %v", err)
	}
	if corpus.Count() == 0 {
		t.Fatalf("embedded corpus is empty")
	}

	requiredIDs := []string{
		"identity/coder/codedom_premier",
		"identity/coder/tool_usage",
		"capability/codedom_first",
		"capability/codedom_selection",
		"capability/codedom_tools",
	}

	contents := make(map[string]string, len(requiredIDs))
	for _, id := range requiredIDs {
		atom, found := corpus.Get(id)
		if !found {
			t.Errorf("missing required CodeDOM atom %q — expected in embedded corpus under internal/prompt/atoms; check id, category, and go:embed", id)
			continue
		}
		if atom.ID != id {
			t.Errorf("atom id mismatch: got %q want %q", atom.ID, id)
		}
		if strings.TrimSpace(atom.Content) == "" {
			t.Errorf("atom %q has empty content", id)
		}
		contents[id] = atom.Content
	}

	if len(contents) != len(requiredIDs) {
		t.Fatalf("aborting combined-contract checks: %d/%d required atoms present", len(contents), len(requiredIDs))
	}

	var combined string
	for _, id := range requiredIDs {
		combined += "\n" + contents[id]
	}
	lower := strings.ToLower(combined)

	mustContain := func(substr, why string) {
		t.Helper()
		if !strings.Contains(lower, strings.ToLower(substr)) {
			t.Errorf("CodeDOM contract violation (%s): combined corpus missing %q\nHint: search internal/prompt/atoms/capability/codedom_*.yaml and internal/prompt/atoms/identity/coder.yaml", why, substr)
		}
	}
	mustContainAny := func(substrings []string, why string) {
		t.Helper()
		for _, s := range substrings {
			if strings.Contains(lower, strings.ToLower(s)) {
				return
			}
		}
		t.Errorf("CodeDOM contract violation (%s): combined corpus missing any of %q", why, substrings)
	}

	// 1. Existing Markdown uses edit_lines / insert_lines / delete_lines with path + 1-indexed ranges.
	mustContain("existing markdown", "existing Markdown must be documented as using line-range tools")
	mustContain("edit_lines", "existing Markdown must use edit_lines")
	mustContain("insert_lines", "existing Markdown must use insert_lines")
	mustContain("delete_lines", "existing Markdown must use delete_lines")
	mustContain("path", "line-range tools require path param")
	mustContain("1-indexed", "line-range tools must be documented as 1-indexed file ranges")
	mustContainAny([]string{
		"preferred over `edit_file`",
		"preferred over edit_file",
	}, "existing Markdown must be documented as preferred over edit_file")

	// 2. Markdown has no semantic get_elements parser.
	mustContain("get_elements", "Markdown no-parser rule must mention get_elements")
	mustContain("does not parse markdown", "Markdown must state get_elements does NOT parse Markdown")
	mustContainAny([]string{
		"no semantic",
		"has no semantic",
	}, "Markdown must state it has no semantic CodeElement parser")

	// 3. Existing code uses get_elements or get_element then CodeDOM line tools.
	mustContain("existing code", "existing code workflow must be documented")
	mustContainAny([]string{"get_elements", "get_element"}, "existing code must use get_elements or get_element for discovery")
	mustContain("edit_lines", "existing code must then use edit_lines/insert_lines/delete_lines")
	if !strings.Contains(lower, "get_elements") && !strings.Contains(lower, "get_element") {
		t.Errorf("CodeDOM contract violation (existing code discovery): missing get_elements/get_element")
	}

	// 4. edit_file is only a bounded fallback for existing code or Markdown.
	mustContain("edit_file", "fallback rule must mention edit_file")
	mustContain("bounded fallback", "edit_file must be documented as only a bounded fallback")
	mustContainAny([]string{
		"cannot represent",
		"concrete failure",
	}, "bounded fallback must be conditioned on line-range tools cannot represent operation or concrete failure")
	mustContainAny([]string{
		"config/data",
		".json",
	}, "edit_file correct default for config/data must still be documented alongside fallback")

	// 5. write_file is the early-out for a supplied new file.
	mustContain("write_file", "new-file early-out must mention write_file")
	mustContain("early-out", "new-file must be documented as early-out")
	mustContainAny([]string{
		"new file",
		"supplied full content",
	}, "write_file early-out must be for supplied/new file with full content")
	mustContainAny([]string{
		"no get_elements",
		"do not",
		"skip",
	}, "new-file early-out must instruct to skip discovery (get_elements/read_file/glob)")

	// Sanity: three line tools collectively present.
	for _, tool := range []string{"edit_lines", "insert_lines", "delete_lines"} {
		if !strings.Contains(lower, tool) {
			t.Errorf("combined corpus missing line-range tool %q", tool)
		}
	}
}

func TestCodeDOMEditingPolicy_AtomsAreMandatoryWhereExpected(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus failed: %v", err)
	}
	mandatoryIDs := []string{
		"identity/coder/codedom_premier",
		"identity/coder/tool_usage",
		"capability/codedom_first",
	}
	for _, id := range mandatoryIDs {
		atom, found := corpus.Get(id)
		if !found {
			t.Errorf("expected mandatory atom %q not found", id)
			continue
		}
		if !atom.IsMandatory {
			t.Errorf("atom %q should be is_mandatory=true per YAML, got false", id)
		}
	}
}
