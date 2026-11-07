package tactile

import (
	"os"
	"strings"
)

// DetectLineEnding returns the dominant newline convention in data. CRLF must
// strictly outnumber lone LF sequences; ties use LF.
func DetectLineEnding(data []byte) string {
	lfCount := 0
	crlfCount := 0
	for i, b := range data {
		if b != '\n' {
			continue
		}
		lfCount++
		if i > 0 && data[i-1] == '\r' {
			crlfCount++
		}
	}
	if crlfCount > lfCount-crlfCount {
		return "\r\n"
	}
	return "\n"
}

// NormalizeLineEnding converts content to one newline convention without
// doubling existing CRLF sequences.
func NormalizeLineEnding(content, ending string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if ending == "\r\n" {
		return strings.ReplaceAll(content, "\n", "\r\n")
	}
	return content
}

// SpliceOntoFile returns the bytes to write to path so that edited becomes
// its content. A path that does not exist yet gets edited as given (the
// new-file convention is the caller's); an existing file is spliced with
// SpliceLineEndings so lines the edit left alone keep their own endings. A
// read failure for an existing path is returned, never papered over.
func SpliceOntoFile(path, edited string) ([]byte, error) {
	orig, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []byte(edited), nil
	}
	if err != nil {
		return nil, err
	}
	return SpliceLineEndings(orig, edited), nil
}

// spliceLine is one line of a file: its text without the terminator and the
// terminator it ended with ("\r\n", "\n", or "" for an unterminated last
// line).
type spliceLine struct {
	text string
	term string
}

// splitKeepingTerminators splits data into lines that remember their own
// terminator. An empty input has no lines; a trailing newline does not open
// an empty last line.
func splitKeepingTerminators(data string) []spliceLine {
	var lines []spliceLine
	for len(data) > 0 {
		i := strings.IndexByte(data, '\n')
		if i < 0 {
			lines = append(lines, spliceLine{text: data})
			break
		}
		line := spliceLine{text: data[:i], term: "\n"}
		if strings.HasSuffix(line.text, "\r") {
			line.text = line.text[:len(line.text)-1]
			line.term = "\r\n"
		}
		lines = append(lines, line)
		data = data[i+1:]
	}
	return lines
}

// sameLine reports whether two lines are the same edit-wise: equal text, and
// both terminated or both not. The terminator's kind is deliberately not
// compared; the edit was made in LF space and never saw it.
func sameLine(a, b spliceLine) bool {
	return a.text == b.text && (a.term != "") == (b.term != "")
}

// SpliceLineEndings returns the bytes to write when edited (the edit's result,
// in any newline convention) replaces the existing file content orig. Lines
// the edit did not change keep their original bytes, terminator included, so
// a file with mixed line endings is not rewritten outside the edited span.
// Rewriting the whole output in the file's dominant convention instead
// changed every line that did not match the majority.
//
// The unchanged lines are the longest common prefix and suffix of the two
// line lists (compared by content; whether a line is terminated counts, which
// keeps the end-of-file state the edit chose). The lines between them are the
// edit's: each takes the terminator of the original line it replaces, in
// order, and a line with no such partner takes the dominant ending of orig.
// The last edited line is left unterminated when the edit leaves it so.
// On a file that uses one convention throughout the result equals
// NormalizeLineEnding(edited, that convention).
func SpliceLineEndings(orig []byte, edited string) []byte {
	dominant := DetectLineEnding(orig)
	before := splitKeepingTerminators(string(orig))
	after := splitKeepingTerminators(NormalizeLineEnding(edited, "\n"))

	limit := min(len(before), len(after))
	prefix := 0
	for prefix < limit && sameLine(before[prefix], after[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < limit-prefix && sameLine(before[len(before)-1-suffix], after[len(after)-1-suffix]) {
		suffix++
	}

	out := make([]byte, 0, len(orig)+len(edited))
	for _, l := range before[:prefix] {
		out = append(out, l.text...)
		out = append(out, l.term...)
	}
	replaced := before[prefix : len(before)-suffix]
	for k, l := range after[prefix : len(after)-suffix] {
		out = append(out, l.text...)
		if l.term == "" {
			continue
		}
		term := dominant
		if k < len(replaced) && replaced[k].term != "" {
			term = replaced[k].term
		}
		out = append(out, term...)
	}
	for _, l := range before[len(before)-suffix:] {
		out = append(out, l.text...)
		out = append(out, l.term...)
	}
	return out
}
