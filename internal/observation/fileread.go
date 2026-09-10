package observation

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"codenerd/internal/observation/precondition"
	"codenerd/internal/tools/codedom"
)

// OutlineEntry is one code element the projection did not print in full.
//
// Name, kind and extent are all of it. Printing the body is what the codec
// exists not to do; printing the extent is what lets the next read ask for the
// right lines instead of guessing at them.
type OutlineEntry struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// FileReadResult is the projection of a file read handed to the reasoning.
type FileReadResult struct {
	Path  string `json:"path"`
	Lines int    `json:"lines"`
	Bytes int    `json:"bytes"`

	// Start and End bound the source actually shown, after the requested range
	// was snapped out to whole code elements and padded.
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Region string `json:"region"`

	// Elided counts lines of source not shown. A projection that hides its own
	// omissions reads as a whole file and gets edited as one.
	Elided int `json:"elided,omitempty"`
	// RegionCut counts bytes dropped from the middle of the shown text — which
	// only happens on a file with no line breaks to trim at.
	RegionCut      int            `json:"region_cut,omitempty"`
	Outline        []OutlineEntry `json:"outline,omitempty"`
	OutlineOmitted int            `json:"outline_omitted,omitempty"`
	Truncated      bool           `json:"truncated,omitempty"`

	// Handle checks a later edit against this read. Empty means nothing was
	// retained, so no edit can be proved to rest on it.
	Handle string `json:"handle,omitempty"`
}

// ReadLimits bound the projection. A read of a ten-thousand-line file must cost
// what a read of four hundred lines costs, or the codec has changed the shape
// of the problem without changing its size.
type ReadLimits struct {
	MaxRegionLines int
	PadLines       int
	MaxOutline     int
}

// DefaultReadLimits sizes a projection for making one edit.
//
// 400 lines is above the 75th percentile of Go files in this repository, so a
// typical whole-file read still arrives whole and the elision engages on the
// files where an unelided read was already being cut to pieces further down. 8
// lines of padding is what a region gets on top of the code element it was
// snapped out to — enough to see the declaration on either side, and the only
// context at all for a hit in a config file or a top-level comment, where there
// is no element to snap to.
func DefaultReadLimits() ReadLimits {
	return ReadLimits{MaxRegionLines: 400, PadLines: 8, MaxOutline: 60}
}

func (l ReadLimits) resolved() ReadLimits {
	def := DefaultReadLimits()
	if l.MaxRegionLines <= 0 {
		l.MaxRegionLines = def.MaxRegionLines
	}
	if l.PadLines < 0 {
		l.PadLines = def.PadLines
	}
	if l.MaxOutline <= 0 {
		l.MaxOutline = def.MaxOutline
	}
	return l
}

// maxRegionBytes bounds the shown region regardless of its line count.
//
// A line ceiling alone is not a ceiling. Four hundred lines of a minified
// bundle, a base64 asset or a generated lookup table can be megabytes, and a
// codec whose whole purpose is a bounded cost per observation must not have a
// shape of input that walks straight through it.
const maxRegionBytes = 24 << 10

// EncodeRead projects a file read and retains it under a precondition handle.
//
// The retention and the check live in internal/observation/precondition rather
// than here, because the verbs that must check a precondition are in
// internal/tools/codedom and this package imports that one to find code
// elements. That package documents the split; the short version is that a
// precondition edit_lines cannot reach protects nothing.
func EncodeRead(r precondition.Read, limits ReadLimits) FileReadResult {
	result := ProjectRead(r, limits)
	result.Handle = precondition.Shared().Mint(r)
	return result
}

// ProjectRead turns an observed file into the region around the edit plus an
// outline of everything it did not print.
//
// Exported separately from EncodeRead so the projection can be exercised, and
// reasoned about, without retention in the picture: the two halves fail
// differently, and a test that could not separate them would be testing both at
// once.
func ProjectRead(r precondition.Read, limits ReadLimits) FileReadResult {
	limits = limits.resolved()

	lines := precondition.SplitLines(r.Content)
	total := len(lines)

	result := FileReadResult{
		Path:      r.Name(),
		Lines:     total,
		Bytes:     len(r.Content),
		Truncated: r.Truncated,
	}
	if total == 0 {
		return result
	}

	anchorStart, anchorEnd := precondition.ClampRegion(r.Start, r.End, total)

	// Snap the region out to whole code elements. An edit inside a function
	// needs that function's signature and its closing brace to be sound: this
	// repo already refuses an edit_lines call whose replacement drops a
	// delimiter the replaced range was holding, and a read that showed the
	// middle of a function without either end is how the model composes such a
	// call in the first place.
	elements := sortedElements(codedom.ElementsFromSource(r.Path, r.Content))
	lo, hi := anchorStart, anchorEnd
	if e, ok := innermost(elements, anchorStart); ok && e.StartLine < lo {
		lo = e.StartLine
	}
	if e, ok := innermost(elements, anchorEnd); ok && e.EndLine > hi {
		hi = e.EndLine
	}

	lo = max(1, lo-limits.PadLines)
	hi = min(total, hi+limits.PadLines)
	lo, hi = budgetRegion(lines, lo, hi, anchorStart, anchorEnd, limits.MaxRegionLines)

	region := precondition.Region(lines, lo, hi)
	result.Start, result.End = lo, hi
	result.Elided = (lo - 1) + (total - hi)

	// The line budget has nothing to trim on a file with no line breaks in it,
	// and a minified bundle or a base64 asset is exactly that: one line of
	// megabytes. budgetRegion stops at a single line by construction, so
	// without this a shape of input walks straight through both ceilings and
	// the whole file lands in context. The cut is at the tail of that one line
	// and is announced, because a silently halved line reads as a whole record.
	if len(region) > maxRegionBytes {
		cut := trimToRune(region[:maxRegionBytes])
		result.RegionCut = len(region) - len(cut)
		region = cut
	}
	result.Region = region

	// Only elements the region does not already show in full. One printed above
	// and then listed again is the projection paying for the same fact twice,
	// which is how the code-search codec next door first came out larger than
	// the grep output it replaced.
	outline := make([]OutlineEntry, 0, len(elements))
	for _, e := range elements {
		if e.StartLine >= lo && e.EndLine <= hi {
			continue
		}
		outline = append(outline, OutlineEntry{
			Name: e.Name, Kind: e.Type, StartLine: e.StartLine, EndLine: e.EndLine,
		})
	}
	sort.SliceStable(outline, func(i, j int) bool {
		if outline[i].StartLine != outline[j].StartLine {
			return outline[i].StartLine < outline[j].StartLine
		}
		if outline[i].EndLine != outline[j].EndLine {
			return outline[i].EndLine < outline[j].EndLine
		}
		return outline[i].Name < outline[j].Name
	})
	if len(outline) > limits.MaxOutline {
		result.OutlineOmitted = len(outline) - limits.MaxOutline
		outline = outline[:limits.MaxOutline]
	}
	result.Outline = outline
	return result
}

// budgetRegion trims context until the region fits, and gives up the caller's
// own range only when that alone is over budget.
//
// Context goes first, on purpose. The lines the caller asked about are the
// reason the read happened; surrendering those to keep the padding would answer
// a question nobody asked.
func budgetRegion(lines []string, lo, hi, anchorStart, anchorEnd, maxLines int) (int, int) {
	if anchorEnd-anchorStart+1 > maxLines {
		anchorEnd = anchorStart + maxLines - 1
	}
	if hi-lo+1 > maxLines {
		slack := maxLines - (anchorEnd - anchorStart + 1)
		after := min(hi-anchorEnd, slack/2)
		before := min(anchorStart-lo, slack-after)
		after = min(hi-anchorEnd, slack-before)
		lo, hi = anchorStart-before, anchorEnd+after
	}

	// The byte ceiling is applied after the line ceiling because it is the rare
	// one: it only bites on files whose lines are not line-shaped. The running
	// total is carried rather than recomputed each step — re-measuring a
	// four-hundred-line region once per trimmed line is quadratic on exactly
	// the oversized input this branch exists to handle.
	size := regionBytes(lines, lo, hi)
	for hi > lo && size > maxRegionBytes {
		switch {
		case hi > anchorEnd:
			size -= lineCost(lines, hi)
			hi--
		case lo < anchorStart:
			size -= lineCost(lines, lo)
			lo++
		default:
			size -= lineCost(lines, hi)
			hi--
			anchorEnd = hi
		}
	}
	return lo, hi
}

// regionBytes totals what lines lo..hi will cost, newlines included.
func regionBytes(lines []string, lo, hi int) int {
	total := 0
	for i := lo - 1; i < hi && i < len(lines); i++ {
		total += len(lines[i]) + 1
	}
	return total
}

// lineCost is what one 1-indexed line contributes to a region.
func lineCost(lines []string, n int) int {
	if n < 1 || n > len(lines) {
		return 0
	}
	return len(lines[n-1]) + 1
}

// preconditionVerbs names the edit verbs that accept a precondition. It is
// stated once, next to precondition.Arg, so a read result cannot advertise a
// verb that ignores the argument — a model told to pass something that is
// silently dropped believes it is protected and is not.
const preconditionVerbs = "edit_file, edit_lines, insert_lines or delete_lines"

// Text renders the projection for a model or an operator.
//
// The region is line-numbered because that is what read_file has always
// returned and what every citation and every line-addressed edit in this repo
// depends on; numbering a snapped region from 1 would look authoritative and be
// wrong by the offset. The outline is a table with its columns named once in a
// header rather than JSON naming them once per row — on a file with sixty
// elided elements that difference is most of the saving.
func (r FileReadResult) Text() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "%s: lines %d-%d of %d", r.Path, r.Start, r.End, r.Lines)
	if r.Elided > 0 {
		fmt.Fprintf(&sb, " (%d line(s) not shown", r.Elided)
		if len(r.Outline) == 0 {
			// With an outline the elided part is reachable by name. Without one
			// — a config file, a document, anything codedom has no patterns for
			// — the reader is left with a hole and no handle on it, so the
			// header has to say how to ask. This clause is the whole difference
			// between an elision and a loss on those files.
			sb.WriteString("; ask for them with start_line/end_line")
		}
		sb.WriteString(")")
	}
	if r.RegionCut > 0 {
		fmt.Fprintf(&sb, " (%d byte(s) cut from the end of the text below: these lines are longer than one read's budget)", r.RegionCut)
	}
	if r.Truncated {
		// End of what was read is not end of file when the read stopped at a
		// size cap, and an agent that concludes "that is the whole file" from a
		// capped read has drawn a false negative about everything below it.
		sb.WriteString(" (the read stopped at its size cap; the file continues past the last line shown)")
	}
	sb.WriteString("\n")

	if r.Lines == 0 {
		sb.WriteString("(empty file)\n")
	} else {
		sb.WriteString(numberedRegion(r.Region, r.Start))
		sb.WriteString("\n")
	}

	if len(r.Outline) > 0 {
		sb.WriteString("elsewhere in this file (kind name lines):\n")
		for _, e := range r.Outline {
			fmt.Fprintf(&sb, "  %s %s %d-%d\n", e.Kind, e.Name, e.StartLine, e.EndLine)
		}
		if r.OutlineOmitted > 0 {
			fmt.Fprintf(&sb, "  ... %d more element(s) not listed\n", r.OutlineOmitted)
		}
	}

	if r.Handle != "" {
		// A precondition nobody is told how to use is a precondition nobody
		// uses, and minting one is worth nothing unless the edit that follows
		// can be refused when this read has gone stale. Naming the argument and
		// the verbs is what makes that reachable.
		//
		// It is written as the argument itself, once, rather than as a handle
		// and then a sentence repeating it. On a short file where nothing is
		// elided this footer IS the codec's entire cost over the raw read, and
		// naming the handle twice doubled the largest part of it. The full
		// explanation lives in the schema of each verb, where it is paid for
		// once per turn instead of once per read.
		fmt.Fprintf(&sb, "%s=%s — pass to %s to have a stale edit refused\n",
			precondition.Arg, r.Handle, preconditionVerbs)
	}
	return sb.String()
}

// numberedRegion prefixes each line with its real 1-indexed number.
//
// Without numbering the model has to count lines by eye to cite anything, and
// it counts badly. Measured on the architecture docs codeNERD wrote about its
// own projectdoc package: the claims were correct but the citations drifted by
// between one and forty-two lines, and one pointed into an unrelated function.
// This repo asks every architectural claim to carry a file:line, so an
// uncountable read makes that convention unsatisfiable.
func numberedRegion(region string, startAt int) string {
	if region == "" {
		return ""
	}
	if startAt < 1 {
		startAt = 1
	}
	lines := strings.Split(region, "\n")
	var sb strings.Builder
	sb.Grow(len(region) + len(lines)*8)
	for i, line := range lines {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "%d\t%s", startAt+i, line)
	}
	return sb.String()
}

// trimToRune drops a partial rune left by a byte-position cut.
//
// A cut inside a multi-byte character leaves an invalid sequence, which is not
// merely ugly: it can make the whole excerpt unusable to a consumer that
// validates UTF-8 on the way out, and the two or three bytes it costs to fix
// are nothing against the ceiling that caused the cut.
func trimToRune(s string) string {
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
