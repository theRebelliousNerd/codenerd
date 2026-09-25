package session

import (
	"errors"
	"fmt"
	"io/fs"
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

// The repair prompt's diff budget. A turn that rewrote a generated file, or a
// whole large file, produced a diff the size of the file, and the repair round
// carried all of it: the diff is evidence for the model, not the change itself,
// so a bounded section with a marker saying what was cut serves the round
// better than a window filled with one file's hunks. The file on disk is the
// full truth and is one read_file away; the marker says so.
//
// The saved attempt patch (leaveBuildableTree) is not a prompt and is written
// unbounded: it is the record a person restores from.
const (
	// turnDiffFileBudget bounds one file's rendered section, fences included.
	turnDiffFileBudget = 8 << 10
	// turnDiffTurnBudget bounds the whole section, header included.
	turnDiffTurnBudget = 24 << 10
	// turnDiffOmissionReserve is the room turnDiffTurnBudget keeps back for
	// the line naming files that did not fit, so that line never pushes the
	// section past its budget.
	turnDiffOmissionReserve = 512
	// turnDiffMinFileSection is the smallest file section worth rendering
	// when the turn budget is nearly spent; below it the file is named in the
	// omission line instead.
	turnDiffMinFileSection = 256
	// truncationTailRoom covers truncateSection's marker line and closing
	// fence.
	truncationTailRoom = 128
)

// diffTruncatedMarker opens every truncation line, so a reader (or a test) can
// tell a cut diff from a complete one.
const diffTruncatedMarker = "[diff truncated"

const turnDiffHeader = "\nWhat this turn changed, as a diff of your own edits:\n"

// diffBudget bounds a rendered turn diff; a zero field is unbounded.
type diffBudget struct {
	perFile int
	perTurn int
}

var promptDiffBudget = diffBudget{perFile: turnDiffFileBudget, perTurn: turnDiffTurnBudget}

// fileChange is one written path as the turn left it.
type fileChange struct {
	path    string
	before  string
	after   string
	created bool // the path held no file before the turn
	deleted bool // the path holds no file now
}

// fileSection is one file's rendered diff, whole.
type fileSection struct {
	path   string
	text   string
	fenced bool // text is a ```diff block that truncateSection can cut
}

// turnDiffSection renders the turn's edits as a diff for a repair prompt, file
// by file, oldest state on the left, within promptDiffBudget. It returns ""
// when nothing was written or no preimage was recorded: the prompt then says
// what it said before.
func turnDiffSection(workspace string, written []string, preWrite map[string]PreImage) string {
	return renderTurnDiff(workspace, written, preWrite, promptDiffBudget)
}

// turnDiffPatch is turnDiffSection with no budget: the whole attempt, for the
// patch saved when a turn's files are put back.
func turnDiffPatch(workspace string, written []string, preWrite map[string]PreImage) string {
	return renderTurnDiff(workspace, written, preWrite, diffBudget{})
}

func renderTurnDiff(workspace string, written []string, preWrite map[string]PreImage, budget diffBudget) string {
	paths := append([]string(nil), written...)
	sort.Strings(paths)
	var sections []fileSection
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
		change := fileChange{path: p, before: pre.Content, created: !pre.Existed}
		data, err := os.ReadFile(diskPath(workspace, p))
		switch {
		case err == nil:
			change.after = string(data)
		case errors.Is(err, fs.ErrNotExist) && pre.Existed:
			// The turn deleted it: the whole preimage is the diff. Skipping
			// it, as this did before, hid a delete from the round repairing it.
			change.deleted = true
		default:
			continue
		}
		if text, fenced := renderFileDiff(change); text != "" {
			sections = append(sections, fileSection{path: p, text: text, fenced: fenced})
		}
	}
	if len(sections) == 0 {
		return ""
	}
	return assembleTurnDiff(sections, budget)
}

// assembleTurnDiff joins the file sections under the header within budget. A
// section over its per-file budget, or over what the turn budget has left, is
// cut at a line boundary when that is still worth reading, and otherwise named
// in one closing line, so no file the turn changed goes unmentioned.
func assembleTurnDiff(sections []fileSection, budget diffBudget) string {
	var b strings.Builder
	b.WriteString(turnDiffHeader)
	var omitted []string
	for _, s := range sections {
		limit := budget.perFile
		if budget.perTurn > 0 {
			room := budget.perTurn - turnDiffOmissionReserve - b.Len()
			if room < turnDiffMinFileSection && len(s.text) > room {
				omitted = append(omitted, s.path)
				continue
			}
			if limit <= 0 || room < limit {
				limit = room
			}
		}
		switch {
		case limit <= 0 || len(s.text) <= limit:
			b.WriteString(s.text)
		case s.fenced:
			b.WriteString(truncateSection(s.text, limit))
		default:
			omitted = append(omitted, s.path)
		}
	}
	if len(omitted) > 0 {
		b.WriteString(omittedFilesLine(omitted, turnDiffOmissionReserve))
	}
	return b.String()
}

// omittedFilesLine names the files the turn budget left out, within limit
// bytes.
func omittedFilesLine(omitted []string, limit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s: %d more changed file(s) not shown; read them with read_file:", diffTruncatedMarker, len(omitted))
	listed := 0
	for _, p := range omitted {
		// Room for this name plus the " and N more]" tail.
		if b.Len()+1+len(p)+24 > limit {
			break
		}
		b.WriteString(" ")
		b.WriteString(p)
		listed++
	}
	if rest := len(omitted) - listed; rest > 0 {
		fmt.Fprintf(&b, " and %d more", rest)
	}
	b.WriteString("]\n")
	return b.String()
}

// truncateSection cuts a rendered ```diff section to limit bytes at a line
// boundary, closes its fence, and says how much of the file's diff was cut.
func truncateSection(section string, limit int) string {
	lines := strings.SplitAfter(section, "\n")
	// The section is "\n", "<path>\n", "```diff\n", the diff lines, "```\n":
	// only the diff lines are counted.
	const frame = 3
	total := max(strings.Count(section, "\n")-frame-1, 0)
	var b strings.Builder
	shown := 0
	for i, l := range lines {
		// Keep room for the marker line and the closing fence.
		if b.Len()+len(l)+truncationTailRoom > limit {
			break
		}
		b.WriteString(l)
		if i >= frame {
			shown++
		}
	}
	shown = min(shown, total)
	fmt.Fprintf(&b, "%s: %d of %d lines of this file's diff not shown; read_file for the rest]\n```\n",
		diffTruncatedMarker, total-shown, total)
	return b.String()
}

// renderFileDiff is one file's whole diff, or "" when the file is unchanged.
// fenced reports a ```diff block, as opposed to a one-line note.
func renderFileDiff(c fileChange) (text string, fenced bool) {
	// A short-circuit, not a guard: identical content produces no hunks
	// below either, and a turn that wrote a file back as it found it is
	// common enough that the diff is worth not computing.
	if c.before == c.after && !c.deleted && !c.created {
		return "", false
	}
	fd := diff.ComputeDiff(c.path, c.path, c.before, c.after)
	if fd == nil {
		return "", false
	}
	note := fileNote(c)
	if fd.IsBinary {
		// A binary edit has no hunks. Rendering "" for it, as this did before,
		// told the round the turn had not touched the file.
		return fmt.Sprintf("\n%s%s (binary: %d bytes before, %d after; diff not shown)\n",
			c.path, note, len(c.before), len(c.after)), false
	}
	if len(fd.Hunks) == 0 {
		if note == "" {
			return "", false
		}
		// Created or deleted empty: there are no lines to show, but the
		// file's existence changed and the round should know.
		return fmt.Sprintf("\n%s%s (empty)\n", c.path, note), false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s%s\n```diff\n", c.path, note)
	for _, h := range fd.Hunks {
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, l := range h.Lines {
			b.WriteString(diffMarker(l.Type) + l.Content + "\n")
		}
	}
	b.WriteString("```\n")
	return b.String(), true
}

// fileNote says how the file's existence changed, from the preimage and the
// disk rather than from diff.FileDiff's IsNew/IsDelete, which only mean one
// side is empty: a file emptied by the turn is not deleted, and a file that
// existed empty and gained content was not created.
func fileNote(c fileChange) string {
	switch {
	case c.created:
		return " (created by this turn)"
	case c.deleted:
		return " (deleted by this turn)"
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
