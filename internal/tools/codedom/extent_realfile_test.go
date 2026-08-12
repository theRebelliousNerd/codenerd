package codedom

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// Synthetic fixtures can agree with a buggy implementation. This runs the real
// extractor over a real repo file whose extents are known by hand:
// ForbidsPath's body runs to the closing brace.
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

	// Expectation is derived by reading the fixture file rather than hardcoding
	// a literal line number: a literal couples this test to unrelated edits in
	// a file it does not own and will break again whenever that file grows
	// above the target function.
	wantStart := 0
	f, err := os.Open("../../projectdoc/nerdmd.go")
	if err != nil {
		t.Fatalf("open nerdmd.go to derive expected StartLine: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	lnum := 0
	for scanner.Scan() {
		lnum++
		trimmed := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(trimmed, "func ") && strings.Contains(trimmed, "ForbidsPath(") {
			wantStart = lnum
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan nerdmd.go: %v", err)
	}
	if wantStart == 0 {
		t.Fatalf("ForbidsPath declaration not found in ../../projectdoc/nerdmd.go — fixture moved or renamed")
	}

	var found bool
	for _, el := range elements {
		if el.Name != "ForbidsPath" {
			continue
		}
		found = true
		if el.StartLine != wantStart {
			t.Errorf("ForbidsPath StartLine = %d, want %d", el.StartLine, wantStart)
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

