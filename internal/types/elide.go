package types

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Text bounding primitives shared by every stage that can put text into an
// outbound prompt, a tool result, or a message the model reads.
//
// They live in types, not in the prompt compiler that first needed them,
// because the rule they implement is the pipeline's and not one package's.
// The shell backend that caps a 40 MB build log, the compressor that folds a
// history segment, the fact serializer and the prompt compiler all cut
// model-bound content, and a marker only means anything if every one of them
// leaves the same one. types is the lowest package all of them already import.
//
// This is the second half of a two-part rule. The first half lives next door
// in truncation.go: when the MODEL's own output is cut at the provider's
// ceiling, the broker sends it back to be restated whole within the limit
// (OutputTruncated, internal/broker/compression.go) rather than keeping the
// partial. Content is restated when it can be, and elided with a marker when
// it cannot. It is never quietly shortened.
//
// The governing rule is that truncation is never silent. A model that is
// handed the first 4 KB of a 40 MB build log and no marker will reason about
// it as if it were the whole log, and will confidently report that the build
// succeeded because the failure it needed to see was in the part we dropped.
// Every helper here leaves a marker that names how much was removed, so the
// model can ask for the rest instead of hallucinating it.
//
// Truncation is head+tail rather than head-only wherever the tail can carry
// the conclusion: a stack trace ends with the panic, `go test` output ends
// with FAIL and the failing package list, a diff ends with the last hunk.
// Head-only truncation on those inputs removes exactly the line the turn
// exists to act on.
//
// Cuts also snap to line boundaries where doing so is cheap. Keeping the right
// end of the text is not enough on its own: a cut at a byte offset lands
// mid-line, and a half-line reads to the model as a whole record.

const (
	// clampMarkerPrefix is the visible truncation marker's stable prefix.
	// Tests and log scrapers match on it; do not reword it casually.
	clampMarkerPrefix = "[codenerd: truncated"

	// clampTailDivisor splits a clamped budget between head and tail. The
	// tail gets 1/clampTailDivisor of the budget and the head gets the rest:
	// framing (what this text is, which file, which command) is at the top
	// and is needed to interpret anything, but the verdict is at the bottom.
	// A 2:1 head:tail split keeps enough of both.
	clampTailDivisor = 3

	// minClampChars is the smallest budget for which a head+tail split is
	// worth doing. Below this the marker itself dominates, so we degrade to
	// head-only.
	minClampChars = 200

	// clampSnapDivisor bounds how far a cut may move to land on a line
	// boundary: at most budget/clampSnapDivisor characters.
	//
	// Tool results are overwhelmingly line-oriented — `go test` output,
	// compiler diagnostics, shell output, file reads — and a byte-position cut
	// lands mid-line most of the time. The fragment that produces is worse
	// than a shorter honest excerpt, because the model reads a half-line as a
	// whole record: "FAIL github.com/x/y" cut after "FAIL github.com/x" is a
	// package that does not exist, and "0 tests failed" cut from
	// "10 tests failed" inverts the result.
	//
	// The bound matters as much as the snapping. A minified bundle or a
	// base64 blob can be one line of megabytes, and snapping unconditionally
	// would discard the entire excerpt to reach a newline that never comes.
	// Past this distance the raw cut is the better answer.
	clampSnapDivisor = 8
)

// ClampText bounds text to maxChars, keeping the head and the tail and
// leaving a marker naming how much was removed. maxChars <= 0 returns "".
//
// The returned string can exceed maxChars by the length of the marker; the
// marker is the point, and callers that need a hard ceiling should subtract
// a marker allowance from their limit first.
func ClampText(text string, maxChars int, label string) string {
	if maxChars <= 0 {
		return ""
	}
	if len(text) <= maxChars {
		return text
	}
	dropped := len(text) - maxChars
	marker := fmt.Sprintf("\n\n%s %d of %d chars from %s] …\n\n",
		clampMarkerPrefix, dropped, len(text), label)

	if maxChars < minClampChars {
		return trimUTF8Suffix(text[:maxChars]) + marker
	}

	tailChars := maxChars / clampTailDivisor
	headChars := maxChars - tailChars
	head := trimUTF8Suffix(snapHeadToLine(text[:headChars], headChars/clampSnapDivisor))
	tail := trimUTF8Prefix(snapTailToLine(text[len(text)-tailChars:], tailChars/clampSnapDivisor))
	return head + marker + tail
}

// snapHeadToLine trims a trailing partial line from a head excerpt, provided
// the trim costs no more than budget characters.
func snapHeadToLine(head string, budget int) string {
	if budget <= 0 {
		return head
	}
	idx := strings.LastIndexByte(head, '\n')
	if idx < 0 || len(head)-idx > budget {
		return head
	}
	return head[:idx]
}

// snapTailToLine drops a leading partial line from a tail excerpt, provided
// the drop costs no more than budget characters.
func snapTailToLine(tail string, budget int) string {
	if budget <= 0 {
		return tail
	}
	idx := strings.IndexByte(tail, '\n')
	if idx < 0 || idx+1 > budget {
		return tail
	}
	return tail[idx+1:]
}

// ClampHead bounds text to maxChars keeping only the head. Use it when the
// tail provably cannot matter — a sorted registry listing, a fact dump, a
// description field — and ClampText everywhere else.
func ClampHead(text string, maxChars int, label string) string {
	if maxChars <= 0 {
		return ""
	}
	if len(text) <= maxChars {
		return text
	}
	dropped := len(text) - maxChars
	return trimUTF8Suffix(text[:maxChars]) +
		fmt.Sprintf("\n%s %d of %d chars from %s] …\n", clampMarkerPrefix, dropped, len(text), label)
}

// ClampInline bounds text to maxChars for a position where a newline cannot
// go: a Mangle fact argument, a status line, one rendered row. It keeps the
// head and appends the marker on the same line.
//
// The variant exists because the alternative was every inline caller inventing
// its own "..." — which says that something was cut but not how much, and
// reads to a model like the author's own ellipsis rather than the pipeline's.
func ClampInline(text string, maxChars int, label string) string {
	if maxChars <= 0 {
		return ""
	}
	if len(text) <= maxChars {
		return text
	}
	dropped := len(text) - maxChars
	return trimUTF8Suffix(text[:maxChars]) +
		fmt.Sprintf(" %s %d of %d chars from %s] …", clampMarkerPrefix, dropped, len(text), label)
}

// TruncationMarker renders the bare marker around a caller-supplied detail.
// Use it only where the count-and-kind shape the other helpers produce does
// not fit — a cut whose size the source genuinely did not report. Everything
// that knows how much it dropped says how much it dropped.
func TruncationMarker(detail string) string {
	return fmt.Sprintf("%s %s] …", clampMarkerPrefix, detail)
}

// DroppedNotice renders the marker for content that left a model-facing
// message whole rather than being shortened in place — an evicted history
// turn, an archived tool result, the remainder of a capped list. handle, when
// non-empty, is what the model calls to get the content back; it is the
// difference between "some of this is gone" and "some of this is over there".
//
// A handle nobody can redeem is worse than none — it sends the reader off to
// a verb that answers "not found", which is indistinguishable from expiry —
// so callers pass one only when a verb in this process actually resolves it.
func DroppedNotice(dropped, total int, unit, handle string) string {
	if dropped <= 0 {
		return ""
	}
	notice := TruncationMarker(fmt.Sprintf("%d of %d %s", dropped, total, unit))
	if handle != "" {
		notice += " " + handle
	}
	return notice
}

// ClampLines bounds text to maxLines, keeping the head and the tail. Use it
// for line-oriented output (compiler diagnostics, test output) where cutting
// mid-line produces a fragment the model may misread as a complete record.
func ClampLines(text string, maxLines int, label string) string {
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}
	tailLines := maxLines / clampTailDivisor
	headLines := maxLines - tailLines
	dropped := len(lines) - maxLines
	var b strings.Builder
	b.WriteString(strings.Join(lines[:headLines], "\n"))
	fmt.Fprintf(&b, "\n%s %d of %d lines from %s] …\n", clampMarkerPrefix, dropped, len(lines), label)
	b.WriteString(strings.Join(lines[len(lines)-tailLines:], "\n"))
	return b.String()
}

// TruncationNotice renders the standard "and N more" line a caller emits after
// breaking out of a capped list. Centralized so every list cap in the prompt
// path reads the same way to the model.
func TruncationNotice(shown, total int, unit string) string {
	if total <= shown {
		return ""
	}
	return fmt.Sprintf("%s %d of %d %s] …", clampMarkerPrefix, total-shown, total, unit)
}

// CapReachedNotice renders the marker for a list that stopped AT a cap without
// counting what was left, which TruncationNotice cannot express because it
// needs a total.
//
// A search that breaks out of its walk at the limit does not know how many
// more there were, and saying "100 matches" is then indistinguishable from
// having found exactly 100. Measured 2026-09-20: grep returned exactly its cap
// for a class with 189 members and said nothing, so the model -- which had
// already raised max_results once -- re-ran the identical search, the working
// policy saw two identical rounds and derived working_stop(/repeated_cycle),
// and the task ended after three tool calls. A silent cap does not lose
// information politely; it produces a confident wrong count and a loop.
//
// remedy names the way out, so the notice is actionable rather than merely
// honest.
func CapReachedNotice(shown int, unit, remedy string) string {
	if shown <= 0 {
		return ""
	}
	notice := fmt.Sprintf("%s at the cap of %d %s; there may be more] …",
		clampMarkerPrefix, shown, unit)
	if strings.TrimSpace(remedy) != "" {
		notice += " " + remedy
	}
	return notice
}

// IsClamped reports whether text carries a truncation marker. Tests assert on
// this rather than on the marker's exact wording.
func IsClamped(text string) bool {
	return strings.Contains(text, clampMarkerPrefix)
}

// trimUTF8Suffix drops a trailing partial rune from a byte-sliced prefix.
func trimUTF8Suffix(s string) string {
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == utf8.RuneError && size <= 1 {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

// trimUTF8Prefix drops a leading partial rune from a byte-sliced suffix.
func trimUTF8Prefix(s string) string {
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size <= 1 {
			s = s[1:]
			continue
		}
		break
	}
	return s
}
