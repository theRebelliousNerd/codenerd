package articulation

import (
	"strings"
)

// StreamParser progressively parses an LLM JSON stream containing a PiggybackEnvelope.
// It searches for the start of "surface_response": " and streams the unescaped content.
type StreamParser struct {
	buffer           strings.Builder
	inSurface        bool
	completed        bool
	surfaceBuffer    strings.Builder
	escapeNext       bool
	lastEmittedIndex int
}

// NewStreamParser creates a new StreamParser.
func NewStreamParser() *StreamParser {
	return &StreamParser{}
}

// ProcessChunk takes a new chunk of string from the LLM, adds it to the internal buffer,
// and returns any new unescaped text that belongs to the surface response.
func (p *StreamParser) ProcessChunk(chunk string) string {
	if len(chunk) == 0 {
		return ""
	}

	p.buffer.WriteString(chunk)

	// Once the closing quote of surface_response has been consumed, the parser
	// is terminal. Keep buffering (GetFullBuffer must stay complete) but never
	// re-detect the key or re-emit — otherwise a trailing chunk (e.g. the
	// closing '}' arriving split from the quote) re-enters the detection block
	// below, re-finds "surface_response", resets lastEmittedIndex back to the
	// opening quote, and streams the entire surface a second time.
	if p.completed {
		return ""
	}

	if !p.inSurface {
		// Look for the start of the surface response
		bufStr := p.buffer.String()
		// Try to find the exact marker (with optional spaces)
		// Usually it's `"surface_response": "`
		idx := strings.Index(bufStr, `"surface_response"`)
		if idx != -1 {
			// Find the first quote after the colon
			colonIdx := strings.IndexByte(bufStr[idx:], ':')
			if colonIdx != -1 {
				colonIdx += idx
				quoteIdx := strings.IndexByte(bufStr[colonIdx:], '"')
				if quoteIdx != -1 {
					quoteIdx += colonIdx
					p.inSurface = true
					p.lastEmittedIndex = quoteIdx + 1
				}
			}
		}
	}

	if p.inSurface {
		bufStr := p.buffer.String()
		if p.lastEmittedIndex >= len(bufStr) {
			return ""
		}

		// Process new characters
		var newText strings.Builder
		for i := p.lastEmittedIndex; i < len(bufStr); i++ {
			c := bufStr[i]

			if p.escapeNext {
				switch c {
				case 'n':
					newText.WriteByte('\n')
				case 'r':
					newText.WriteByte('\r')
				case 't':
					newText.WriteByte('\t')
				case '"':
					newText.WriteByte('"')
				case '\\':
					newText.WriteByte('\\')
				default:
					newText.WriteByte(c) // Unrecognized escape
				}
				p.escapeNext = false
				p.lastEmittedIndex = i + 1
				continue
			}

			if c == '\\' {
				p.escapeNext = true
				p.lastEmittedIndex = i + 1
				continue
			}

			// End of string reached
			if c == '"' {
				p.inSurface = false
				p.completed = true
				// We don't advance lastEmittedIndex past the quote because
				// we don't care about the rest of the JSON padding
				break
			}

			newText.WriteByte(c)
			p.lastEmittedIndex = i + 1
		}
		return newText.String()
	}

	return ""
}

// GetFullBuffer returns the complete buffered JSON string.
func (p *StreamParser) GetFullBuffer() string {
	return p.buffer.String()
}
