package core

import (
	"testing"

	"codenerd/internal/tools"
)

// A model edit tells the kernel exactly what the Path-B handler does: the
// element (in the world model's ref form, which code_element and the
// test-dependency builder join on), the modified file, and for a function or
// method the modified_function fact impact.mg starts the caller walk from.
// Before 2026-09-22 no model edit produced any of them, so run_impacted_tests
// answered "No edited refs specified" on the live path.
func TestEditedElementFacts_MatchWhatThePathBHandlerAsserts(t *testing.T) {
	facts := editedElementFacts("sess", 42, []tools.EditedElement{
		{File: "internal/demo/demo.go", Language: "go", Package: "demo", Kind: "method", Name: "M", Receiver: "T"},
		{File: "internal/demo/demo.go", Language: "go", Package: "demo", Kind: "struct", Name: "T"},
		{File: "internal/demo/demo.go", Language: "go", Package: "demo", Kind: "header", Name: "header"},
		{File: "policy/x.mg", Language: "mangle", Kind: "rule", Name: "p"},
	})
	want := map[string]bool{
		"modified|internal/demo/demo.go":                   false,
		"element_modified|fn:demo.T.M":                     false,
		"modified_function|demo.T.M|internal/demo/demo.go": false,
		"element_modified|struct:demo.T":                   false,
		"modified|policy/x.mg":                             false,
	}
	for _, f := range facts {
		key := f.Predicate
		for i, a := range f.Args {
			if f.Predicate == "element_modified" && i > 0 {
				break
			}
			key += "|" + a.(string)
		}
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected fact %s %v", f.Predicate, f.Args)
			continue
		}
		want[key] = true
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("missing fact %s", k)
		}
	}
}

func TestInteractiveGate_ElementVerbsAreDestructiveAndMultiFileEditsAreGatedPerFile(t *testing.T) {
	for tool, want := range map[string]ActionType{
		"edit_element": ActionEditElement, "replace_element": ActionEditElement,
		"insert_element": ActionEditElement, "delete_element": ActionEditElement,
		"create_file": ActionWriteFile, "repoint": ActionEditFile,
	} {
		at, ok := actionTypeForToolName(tool)
		if !ok || at != want {
			t.Errorf("%s maps to %q, want %q", tool, at, want)
		}
		if !isDestructiveAction(at) {
			t.Errorf("%s must take the Dreamer preflight", tool)
		}
	}
	if !isMultiFileTool("repoint", map[string]any{"paths": []any{"a.go", "b.go"}}) {
		t.Error("repoint is gated per declared file")
	}
	if !isMultiFileTool("delete_element", map[string]any{"path": "a.go", "paths": []any{"b.go"}}) {
		t.Error("a delete that repoints uses is gated per declared file")
	}
	if isMultiFileTool("edit_element", map[string]any{"path": "a.go"}) {
		t.Error("a single-file edit is gated once")
	}
}
