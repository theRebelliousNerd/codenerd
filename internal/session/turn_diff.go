package session

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"codenerd/internal/diff"
)

// A repair round is shown what the turn changed (external audit N26).
//
// The round's prompt carried the failure and, since N24, the failing tests --
// but never the turn's own edits, so the model re-diagnoses from scratch every
// attempt. Ladder runs R1-12 and R1-15 both spent three attempts on "why is
// ForbidsPath not extracted" while their own diff, one line of it, renamed the
// element the test looks for. The executor holds every written file's
// preimage, so the diff costs nothing to produce.

// turnDiffSection renders the turn's edits as a diff, file by file, oldest
// state on the left. It returns "" when nothing was written or no preimage was
// recorded: the prompt then says what it said before.
func turnDiffSection(workspace string, written []string, preWrite map[string]PreImage) string {
	paths := append([]string(nil), written...)
	sort.Strings(paths)
	var b strings.Builder
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		pre, ok := preImageFor(workspace, p, preWrite)
		if !ok || !pre.Known() {
			continue
		}
		data, err := os.ReadFile(diskPath(workspace, p))
		if err != nil {
			continue
		}
		rendered := renderFileDiff(p, pre.Content, string(data))
		if rendered == "" {
			continue
		}
		b.WriteString(rendered)
	}
	if b.Len() == 0 {
		return ""
	}
	return "\nWhat this turn changed, as a diff of your own edits:\n" + b.String()
}

// renderFileDiff is one file's hunks, or "" when the file is unchanged.
func renderFileDiff(path, before, after string) string {
	// A short-circuit, not a guard: identical content produces no hunks
	// below either, and a turn that wrote a file back as it found it is
	// common enough that the diff is worth not computing.
	if before == after {
		return ""
	}
	fd := diff.ComputeDiff(path, path, before, after)
	if fd == nil || len(fd.Hunks) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s%s\n```diff\n", path, newFileNote(fd))
	for _, h := range fd.Hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, l := range h.Lines {
			b.WriteString(diffMarker(l.Type) + l.Content + "\n")
		}
	}
	b.WriteString("```\n")
	return b.String()
}

func newFileNote(fd *diff.FileDiff) string {
	if fd.IsNew {
		return " (created by this turn)"
	}
	return ""
}

func diffMarker(t diff.LineType) string {
	switch t {
	case diff.LineAdded:
		return "+"
	case diff.LineRemoved:
		return "-"
	default:
		return " "
	}
}
