package codedom

import "testing"

// Synthetic fixtures can agree with a buggy implementation. This runs the real
// extractor over a real repo file whose extents are known by hand:
// ForbidsPath begins at nerdmd.go:281 and its body runs to the closing brace.
//
// The bug this guards: EndLine was set equal to StartLine with the comment
// "Would need block tracking for accurate end", so every element reported as
// one line long — which silently truncates any edit_lines driven by a
// get_elements range.
func TestExtractCodeElements_RealFileExtents(t *testing.T) {
	elements, err := extractCodeElements("../../projectdoc/nerdmd.go")
	if err != nil {
		t.Fatalf("extractCodeElements: %v", err)
	}
	if len(elements) == 0 {
		t.Fatal("no elements extracted from nerdmd.go")
	}

	var found bool
	for _, el := range elements {
		if el.Name != "ForbidsPath" {
			continue
		}
		found = true
		if el.StartLine != 281 {
			t.Errorf("ForbidsPath StartLine = %d, want 281", el.StartLine)
		}
		if el.EndLine <= el.StartLine {
			t.Errorf("ForbidsPath EndLine = %d, StartLine = %d — extent tracking is not working; "+
				"an edit_lines driven by this range would replace only the signature line",
				el.EndLine, el.StartLine)
		}
	}
	if !found {
		t.Error("ForbidsPath was not extracted from nerdmd.go")
	}

	// No element may be a single line unless it genuinely is one; a whole file
	// of one-line elements is the signature of the old bug.
	multiline := 0
	for _, el := range elements {
		if el.EndLine > el.StartLine {
			multiline++
		}
	}
	if multiline == 0 {
		t.Error("every element reported as one line long — extent tracking is inert")
	}
}
