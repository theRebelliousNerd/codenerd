package prompt

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Text bounding primitives shared by every stage that can put text into an
// outbound prompt.
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
)

// ClampText bounds text to maxChars, keeping the head and the tail and
// leaving a marker naming how much was removed. maxChars <= 0 returns "".
//
// The returned string can exceed maxChars by the length of the marker; the
// marker is the point, and callers that need a hard ceiling should subtract
// clampMarkerBudget from their limit first.
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
	head := trimUTF8Suffix(text[:headChars])
	tail := trimUTF8Prefix(text[len(text)-tailChars:])
	return head + marker + tail
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
