package articulation

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// surfaceKeyName is the surface_response object key the streaming parser
// hunts for, matched by its decoded spelling and case-insensitively (as the
// final parser matches it), only at envelope depth in key position.
const surfaceKeyName = "surface_response"

// maxStreamKeyBytes is the longest spelling of the surface key worth holding
// a chunk for: every character of the name written as a \uXXXX escape (6 bytes
// each) plus the two quotes. A key string longer than this is not the surface
// key however it decodes.
const maxStreamKeyBytes = len(surfaceKeyName)*6 + 2

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
	// Structural tracking for the pre-surface scan: depth counts object braces
	// outside strings, arrayDepth excludes array values, and topLevelExpectKey
	// accepts surface_response only in a top-level object key position.
	depth             int
	arrayDepth        int
	topLevelExpectKey bool
	inString          bool
	strEsc            bool
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
			// A key candidate was followed by another string without a colon.
			// Treat the new string as ordinary input, even if it repeats the
			// surface key, because the envelope is malformed at this point.
			if p.markerFound {
				p.markerFound = false
				p.colonFound = false
				p.inString = true
				continue
			}
			// A provider can split anywhere, including inside this key. A key
			// is its decoded spelling (the final parser's rule), so hold at
			// the opening quote until the closing quote has arrived, then
			// decode and compare. A string that runs past any possible
			// spelling of the name is some other key.
			if p.depth == 1 && p.arrayDepth == 0 && p.topLevelExpectKey {
				name, closed, complete := scanStreamKey(data[i:])
				if !complete {
					end = i
					break loop
				}
				p.topLevelExpectKey = false
				if closed > 0 && strings.EqualFold(name, surfaceKeyName) {
					p.markerFound = true
					i += closed - 1
					continue
				}
			}
			// This is another complete string key/value. Skip it as ordinary
			// JSON.
			p.inString = true
			continue
		}
		// Track the bytes after a complete surface key across chunk calls.
		// This keeps arbitrarily split whitespace linear: the key prefix is
		// held only while incomplete, then each byte is consumed once.
		if p.markerFound {
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
				continue
			}
			if !p.colonFound && c == ':' {
				p.colonFound = true
				continue
			}
			// The key is not followed by a string value, or the would-be
			// surface value is a non-string JSON value.
			if p.colonFound {
				p.markerFound = false
				p.colonFound = false
			} else if c != ':' {
				p.markerFound = false
			}
		}
		switch c {
		case '{':
			if p.depth == 0 && p.arrayDepth == 0 {
				p.topLevelExpectKey = true
			}
			p.depth++
		case '}':
			if p.depth > 0 {
				p.depth--
			}
			if p.depth == 0 {
				p.topLevelExpectKey = false
			}
		case '[':
			p.arrayDepth++
		case ']':
			if p.arrayDepth > 0 {
				p.arrayDepth--
			}
		case ',':
			if p.depth == 1 && p.arrayDepth == 0 {
				p.topLevelExpectKey = true
			}
		}
	}

	p.lastDetectionIndex = end
	return out.String()
}

// scanStreamKey reads the string literal that opens data (data[0] is its
// opening quote). It reports the decoded value and the literal's byte length
// when the closing quote is present; complete is false only when more bytes
// could still finish a literal short enough to matter, so the caller waits. A
// literal that is too long to be the surface key, or does not decode, returns
// complete with closed == 0 and the caller skips it as an ordinary string.
func scanStreamKey(data string) (name string, closed int, complete bool) {
	limit := min(len(data), maxStreamKeyBytes)
	escaped := false
	for j := 1; j < limit; j++ {
		c := data[j]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			if err := json.Unmarshal([]byte(data[:j+1]), &name); err != nil {
				return "", 0, true
			}
			return name, j + 1, true
		}
	}
	if len(data) < maxStreamKeyBytes {
		return "", 0, false
	}
	return "", 0, true
}
