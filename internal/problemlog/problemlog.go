// Package problemlog reads codeNERD's own aggregated problems log
// (.nerd/logs/<run>_problems.log, written by internal/logging) into
// structured entries, groups repeats by a normalized signature, and projects
// the result into kernel facts (policy/self_problems.mg) so the agent reasons
// over its own failures instead of asking the user to paste log lines.
package problemlog

import (
	"bufio"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
)

// The levels the problems log carries: internal/logging mirrors only WARN and
// ERROR into it.
const (
	LevelError = "ERROR"
	LevelWarn  = "WARN"
)

// timeLayout is the prefix log.Ldate|log.Ltime|log.Lmicroseconds writes; the
// fraction is optional so a hand-edited or older line still parses.
const timeLayout = "2006/01/02 15:04:05"

// Entry is one problems-log record. Message is the whole message: a record
// continues over every following line that does not start a new record, so a
// message that carries a multi-line payload (a source listing, a tool's
// stderr) is kept whole.
type Entry struct {
	Time     time.Time
	Level    string
	Category string
	Message  string
}

var entryRe = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})(?:\.\d+)? \[([A-Z]+)\] \[([^\]]+)\] ?(.*)$`)

// stampRe recognizes any timestamped record line, so a record of a level this
// package does not keep (and its continuation lines) ends the previous one
// instead of being appended to it.
var stampRe = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? \[`)

// Scan calls emit for every WARN and ERROR entry in r, in file order. skip, if
// non-nil, drops an entry before it is emitted (the caller's own notes that
// the log would otherwise feed back to itself). Lines before the first record
// belong to no entry and are ignored: a chunk read from the middle of a file
// can start inside a multi-line message. A record owns every line up to the
// next record, up to maxEntryLines lines and maxEntryBytes bytes; the rest of
// an overlong record is dropped and the kept sample carries a truncation
// marker, so one runaway dump cannot grow an Entry without bound. The bounds
// are generous: the multi-line samples in the fixture stay whole.
func Scan(r io.Reader, skip func(Entry) bool, emit func(Entry)) error {
	if r == nil {
		return errors.New("problemlog: nil reader")
	}
	if emit == nil {
		return errors.New("problemlog: nil emit")
	}
	const maxEntryLines = 200
	const maxEntryBytes = 32 << 10
	const truncatedMarker = "\n…[truncated]"
	var cur *Entry
	curLines := 0
	truncated := false
	flush := func() {
		if cur == nil {
			return
		}
		cur.Message = strings.TrimRight(cur.Message, " \t\r\n")
		if skip == nil || !skip(*cur) {
			emit(*cur)
		}
		cur = nil
		curLines = 0
		truncated = false
	}
	br := bufio.NewReader(r)
	firstLine := true
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			line = strings.ToValidUTF8(strings.TrimRight(line, "\r\n"), "")
			if firstLine {
				line = strings.TrimPrefix(line, string([]byte{239, 187, 191}))
				firstLine = false
			}
			if m := entryRe.FindStringSubmatch(line); m != nil {
				flush()
				at, perr := time.ParseInLocation(timeLayout, m[1], time.Local)
				if perr == nil && (m[2] == LevelError || m[2] == LevelWarn) {
					msg := m[4]
					if len(msg) > maxEntryBytes {
						msg = strings.ToValidUTF8(msg[:maxEntryBytes], "") + " …[truncated]"
						truncated = true
					}
					cur = &Entry{Time: at, Level: m[2], Category: strings.TrimSpace(m[3]), Message: msg}
					curLines = 1
				}
			} else if stampRe.MatchString(line) {
				flush()
			} else if cur != nil {
				if truncated || curLines >= maxEntryLines || len(cur.Message) >= maxEntryBytes {
					if !truncated {
						cur.Message += truncatedMarker
						truncated = true
					}
				} else if len(cur.Message)+1+len(line) > maxEntryBytes {
					cur.Message += truncatedMarker
					truncated = true
				} else {
					cur.Message += "\n" + line
					curLines++
				}
			}
		}
		if err != nil {
			flush()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

var (
	uuidRe  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	pathRe  = regexp.MustCompile(`(?:/[A-Za-z0-9._\-]+)+`)
	hexRe   = regexp.MustCompile(`\b[0-9a-fA-F]{8,}\b`)
	numRe   = regexp.MustCompile(`\d+(?:\.\d+)?`)
	spaceRe = regexp.MustCompile(`\s+`)
)

// Signature is the grouping key of a message: its first line with ids, paths,
// hex digests and numbers replaced, so repeats of one problem collapse. A
// continuation (the payload under the headline) is volatile by nature and
// stays in the sample, not the key. The key is never shortened: two problems
// that differ late in the headline are two problems.
func Signature(msg string) string {
	head, _, _ := strings.Cut(msg, "\n")
	head = strings.ToValidUTF8(strings.TrimSuffix(head, "\r"), "")
	// Order is load-bearing: uuid before path before hex before numbers, so a
	// placeholder introduced by an earlier step is never re-mangled by a later
	// one. Only the headline feeds the key.
	s := uuidRe.ReplaceAllString(head, "<uuid>")
	s = pathRe.ReplaceAllString(s, "<path>")
	s = hexRe.ReplaceAllString(s, "<hex>")
	s = numRe.ReplaceAllString(s, "#")
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}
