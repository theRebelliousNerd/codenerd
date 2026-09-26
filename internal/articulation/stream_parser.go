package articulation

import (
	"strings"
	"unicode/utf8"
)

// surfaceKeyMarker is the surface_response object key the streaming parser
// hunts for. Matched only outside string literals at envelope depth.
const surfaceKeyMarker = `"surface_response"`

// StreamParser extracts surface_response incrementally from a streaming JSON
// envelope. Feed it chunks as they arrive; each call returns the newly
// arrived surface text (already-decoded: escapes resolved, complete runes
// only).
//
// Single-owner and stateful: one parser per stream, one goroutine. The parser
// retains the full stream in memory — the caller already holds the same bytes
// for the final authoritative parse, so the parser shares the caller's bound
// rather than inventing its own.
type StreamParser struct {
	buffer             strings.Builder
	markerFound        bool
	colonFound         bool
	inSurface          bool
	escape             bool
	completed          bool
	lastDetectionIndex int
	// Structural tracking for the pre-surface scan: depth counts braces
	// outside strings so a "surface_response" mentioned inside reasoning text
	// is not mistaken for the real top-level key.
	depth    int
	inString bool
	strEsc   bool
}

// NewStreamParser creates a new streaming parser.
func NewStreamParser() *StreamParser {
	return &StreamParser{}
}

// ProcessChunk feeds a chunk of the stream and returns newly arrived surface
// text. Returns "" when the chunk holds no new surface bytes (still inside
// the control packet, or the surface already completed).
func (p *StreamParser) ProcessChunk(chunk string) string {
	if p.completed {
		return ""
	}
	p.buffer.WriteString(chunk)
	data := p.buffer.String()
	end := len(data)

	var out strings.Builder
	i := p.lastDetectionIndex
loop:
	for ; i < len(data); i++ {
		c := data[i]

		if p.inSurface {
			if p.escape {
				// Escape sequences resolve here, including \uXXXX — which
				// may be split across chunks. When the bytes are not all
				// here yet, rewind to the backslash and wait for more.
				switch c {
				case '"', '\\', '/':
					out.WriteByte(c)
					p.escape = false
				case 'b':
					out.WriteByte('\b')
					p.escape = false
				case 'f':
					out.WriteByte('\f')
					p.escape = false
				case 'n':
					out.WriteByte('\n')
					p.escape = false
				case 'r':
					out.WriteByte('\r')
					p.escape = false
				case 't':
					out.WriteByte('\t')
					p.escape = false
				case 'u':
					// \uXXXX may be split across chunks, and a high surrogate
					// may be waiting on its pair. decodeStreamEscape decides:
					// hold (rewind to the backslash) or emit (advance past
					// the consumed bytes).
					consumed, r, hold := decodeStreamEscape(data, i-1)
					if hold {
						p.escape = false
						end = i - 1
						break loop
					}
					out.WriteRune(r)
					p.escape = false
					i += consumed - 2 // -2: loop's i++ plus the consumed 'u'
				default:
					out.WriteByte(c)
					p.escape = false
				}
				continue
			}
			if c == '\\' {
				p.escape = true
				continue
			}
			if c == '"' {
				p.inSurface = false
				p.completed = true
				i++
				break
			}
			// A multi-byte rune split across chunks must not emit half: the
			// renderer would flash invalid UTF-8 for a frame. Hold position —
			// the bytes stay buffered and the next chunk completes them. (If
			// the stream ends mid-rune the tail is dropped from the live
			// channel, but GetFullBuffer stays whole for the final parse.)
			if c >= 0x80 && !utf8.FullRuneInString(data[i:]) {
				end = i
				break loop
			}
			out.WriteByte(c)
			continue
		}

		// Pre-surface structural scan.
		if p.strEsc {
			p.strEsc = false
			continue
		}
		if p.inString {
			if c == '\\' {
				p.strEsc = true
			} else if c == '"' {
				p.inString = false
			}
			continue
		}
		if c == '"' {
			// The value's opening quote: the marker and colon already matched.
			if p.markerFound && p.colonFound {
				p.inSurface = true
				continue
			}
			// A candidate key: accept it only at envelope depth, followed
			// by a colon. Anything quoted inside reasoning text fails the
			// depth test and is skipped as a string instead.
			if p.depth == 1 && strings.HasPrefix(data[i:], surfaceKeyMarker) {
				rest := data[i+len(surfaceKeyMarker):]
				trimmed := strings.TrimLeft(rest, " \t\r\n")
				if strings.HasPrefix(trimmed, ":") {
					p.markerFound = true
					p.colonFound = true
					// Skip past the marker; the colon is consumed logically
					// by colonFound, and any whitespace/colon bytes ahead are
					// inert to the structural scan.
					i += len(surfaceKeyMarker) - 1
					continue
				}
			}
			p.inString = true
			continue
		}
		// Once the key matched, the value's opening quote must come before any
		// structural byte. A comma or brace first means the value is not a
		// string (a number, an object, a missing value) — release the match
		// rather than streaming some later field's string as the surface.
		if p.markerFound {
			switch c {
			case ',', '{', '}', '[', ']':
				p.markerFound = false
				p.colonFound = false
			}
		}
		switch c {
		case '{':
			p.depth++
		case '}':
			if p.depth > 0 {
				p.depth--
			}
		}
	}

	p.lastDetectionIndex = end
	return out.String()
}
