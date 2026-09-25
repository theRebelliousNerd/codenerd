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
//
// The section is bounded (session.repair_diff_file_bytes and
// repair_diff_turn_bytes, ExecutorConfig.repairDiffBudget). A turn that
// rewrote a generated file, or a whole large file, produced a diff the size of
// the file, and the repair round carried all of it: the diff is evidence for
// the model, not the change itself, so a bounded section with a marker saying
// what was cut serves the round better than a window filled with one file's
// hunks. The file on disk is the full truth and is one read_file away; the
// marker says so. The saved attempt patch (leaveBuildableTree) is not a prompt
// and is rendered unbounded: it is the record a person restores from.

// diffTruncatedMarker opens every truncation line, so a reader (or a test) can
// tell a cut diff from a complete one.
const diffTruncatedMarker = "[diff truncated"

const turnDiffHeader = "\nWhat this turn changed, as a diff of your own edits:\n"

// diffFence closes a file's diff block.
const diffFence = "```\n"

// diffBudget bounds a rendered turn diff; a zero field is unbounded.
type diffBudget struct {
	perFile int
	perTurn int
}

// fileChange is one written path as the turn left it.
type fileChange struct {
	path    string
	before  string
	after   string
	created bool // the path held no file before the turn
	deleted bool // the path holds no file now
}

// fileSection is one file's rendered diff: head opens it ("\n<path><note>\n"
// and, for a diff block, the fence), lines are its diff lines. A section with
// no lines is a one-line note (binary, or created/deleted empty).
type fileSection struct {
	path  string
	head  string
	lines []string
}

func (s fileSection) fenced() bool { return len(s.lines) > 0 }

func (s fileSection) String() string {
	if !s.fenced() {
		return s.head
	}
	return s.head + strings.Join(s.lines, "") + diffFence
}

// turnDiffSection renders the turn's edits as a diff for a repair prompt, file
// by file, oldest state on the left, within budget. It returns "" when nothing
// was written or no preimage was recorded: the prompt then says what it said
// before.
func turnDiffSection(workspace string, written []string, preWrite map[string]PreImage, budget diffBudget) string {
	return renderTurnDiff(workspace, written, preWrite, budget)
}

// turnDiffPatch is the turn diff with no budget: the whole attempt, for the
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
		if s, ok := renderFileDiff(change); ok {
			sections = append(sections, s)
		}
	}
	if len(sections) == 0 {
		return ""
	}
	return assembleTurnDiff(sections, budget)
}

// assembleTurnDiff joins the file sections under the header within budget. A
// section over its per-file budget, or over what the turn budget has left, is
// cut at a line boundary when its head and one line still fit, and otherwise
// named in one closing line, so no file the turn changed goes unmentioned.
func assembleTurnDiff(sections []fileSection, budget diffBudget) string {
	var b strings.Builder
	b.WriteString(turnDiffHeader)
	// The closing line's room is held back from the start: its fixed part,
	// with the widest count it could carry. Names go in only as far as the
	// room left at the end allows.
	reserve := 0
	if budget.perTurn > 0 {
		reserve = len(omittedFilesLine(make([]string, len(sections)), 0))
	}
	var omitted []string
	for _, s := range sections {
		text := s.String()
		limit := budget.perFile
		if budget.perTurn > 0 {
			room := budget.perTurn - reserve - b.Len()
			if limit <= 0 || room < limit {
				limit = room
			}
		}
		switch {
		case limit <= 0 && budget.perTurn <= 0:
			b.WriteString(text)
		case len(text) <= limit:
			b.WriteString(text)
		case s.fenced() && len(s.head)+len(s.lines[0])+len(truncationTail(len(s.lines), len(s.lines))) <= limit:
			b.WriteString(truncateSection(s, limit))
		default:
			omitted = append(omitted, s.path)
		}
	}
	if len(omitted) > 0 {
		room := len(omittedFilesLine(omitted, 0))
		if budget.perTurn > 0 {
			room = max(budget.perTurn-b.Len(), room)
		}
		b.WriteString(omittedFilesLine(omitted, room))
	}
	return b.String()
}

// omittedFilesLine names the files the turn budget left out, listing names
// only while the line stays within limit bytes (limit 0: none).
func omittedFilesLine(omitted []string, limit int) string {
	head := fmt.Sprintf("\n%s: %d more changed file(s) not shown; read them with read_file:", diffTruncatedMarker, len(omitted))
	listed := 0
	var names strings.Builder
	for _, p := range omitted {
		tail := omittedTail(len(omitted) - listed - 1)
		if len(head)+names.Len()+len(" ")+len(p)+len(tail) > limit {
			break
		}
		names.WriteString(" ")
		names.WriteString(p)
		listed++
	}
	return head + names.String() + omittedTail(len(omitted)-listed)
}

func omittedTail(rest int) string {
	if rest > 0 {
		return fmt.Sprintf(" and %d more]\n", rest)
	}
	return "]\n"
}

// truncationTail is the marker line and closing fence of a cut section.
func truncationTail(cut, total int) string {
	return fmt.Sprintf("%s: %d of %d lines of this file's diff not shown; read_file for the rest]\n%s",
		diffTruncatedMarker, cut, total, diffFence)
}

// truncateSection renders s cut to limit bytes at a line boundary, with the
// marker saying how much of the file's diff was cut.
func truncateSection(s fileSection, limit int) string {
	total := len(s.lines)
	// The tail is sized for the widest counts it could print, so the cut
	// never pushes the section past limit.
	room := limit - len(s.head) - len(truncationTail(total, total))
	var b strings.Builder
	b.WriteString(s.head)
	shown := 0
	for _, l := range s.lines {
		if b.Len()-len(s.head)+len(l) > room {
			break
		}
		b.WriteString(l)
		shown++
	}
	b.WriteString(truncationTail(total-shown, total))
	return b.String()
}

// renderFileDiff is one file's whole diff; false when the file is unchanged.
func renderFileDiff(c fileChange) (fileSection, bool) {
	// A short-circuit, not a guard: identical content produces no hunks
	// below either, and a turn that wrote a file back as it found it is
	// common enough that the diff is worth not computing.
	if c.before == c.after && !c.deleted && !c.created {
		return fileSection{}, false
	}
	fd := diff.ComputeDiff(c.path, c.path, c.before, c.after)
	if fd == nil {
		return fileSection{}, false
	}
	note := fileNote(c)
	if fd.IsBinary {
		// A binary edit has no hunks. Rendering "" for it, as this did before,
		// told the round the turn had not touched the file.
		return fileSection{path: c.path, head: fmt.Sprintf("\n%s%s (binary: %d bytes before, %d after; diff not shown)\n",
			c.path, note, len(c.before), len(c.after))}, true
	}
	if len(fd.Hunks) == 0 {
		if note == "" {
			return fileSection{}, false
		}
		// Created or deleted empty: there are no lines to show, but the
		// file's existence changed and the round should know.
		return fileSection{path: c.path, head: fmt.Sprintf("\n%s%s (empty)\n", c.path, note)}, true
	}
	s := fileSection{path: c.path, head: fmt.Sprintf("\n%s%s\n```diff\n", c.path, note)}
	for _, h := range fd.Hunks {
		s.lines = append(s.lines, fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount))
		for _, l := range h.Lines {
			s.lines = append(s.lines, diffMarker(l.Type)+l.Content+"\n")
		}
	}
	return s, true
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
