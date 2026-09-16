package articulation

import (
	"strings"
	"unicode/utf8"
)

// maxJSONDepth is the maximum nesting depth for JSON object extraction.
// Prevents CPU/memory exhaustion on deeply nested garbage like {"a":{"a":{"a":...}}}.
const maxJSONDepth = 200

// maxJSONCandidateSize is the maximum size (bytes) of a single extracted JSON object.
// Prevents massive substring allocations from LLM output.
const maxJSONCandidateSize = 5 * 1024 * 1024 // 5MB

// maxRichCandidates caps retained marker-bearing candidates, keeping the tail
// (the extractor tries candidates back-to-front, so the tail is what gets
// parsed). Without this a response of ten thousand tiny
// {"surface_response":...} objects — a confused or hostile model — turns into
// ten thousand full unmarshal attempts. A legitimate response holds exactly
// one envelope; 64 is already generous.
const maxRichCandidates = 64

// findJSONCandidates scans the input string for top-level JSON object candidates.
// It returns a slice of strings, each representing a potential JSON object.
// It handles nested braces and string escaping to correctly identify boundaries.
//
// This function uses a byte-level state machine to efficiently skip over
// strings and non-JSON content, providing significantly better performance
// than regex-based extraction for large inputs.
//
// Safety: depth is capped at maxJSONDepth (200) and candidate size at 5MB
// to prevent resource exhaustion from adversarial input.
//
// Note: It is safe to iterate bytes for ASCII delimiters ({, }, ", \) because
// UTF-8 encoding guarantees that ASCII bytes never appear as part of a multi-byte sequence.
func findJSONCandidates(s string) []string {
	var candidates []string
	var depth int
	var start int = -1
	var inString bool
	var escape bool

	for i := 0; i < len(s); i++ {
		b := s[i]

		// Handle escape sequences inside strings
		if escape {
			escape = false
			continue
		}

		if inString {
			if b == '\\' {
				escape = true
			} else if b == '"' {
				inString = false
			}
			continue
		}

		// Not in string
		if b == '"' {
			inString = true
			continue
		}

		if b == '{' {
			if depth == 0 {
				start = i
			}
			depth++
			// Circuit breaker: bail on absurdly deep nesting
			if depth > maxJSONDepth {
				// Reset and skip this candidate entirely
				depth = 0
				start = -1
				continue
			}
		} else if b == '}' {
			if depth > 0 {
				depth--
				if depth == 0 && start != -1 {
					// Found a complete top-level object - check size cap
					candidateLen := i + 1 - start
					if candidateLen <= maxJSONCandidateSize {
						candStr := s[start : i+1]
						if strings.Contains(candStr, `"surface_response"`) {
							candidates = append(candidates, candStr)
							// Keep the tail: drop the earliest rich candidate
							// past the cap, mirroring the noise eviction below.
							rich := 0
							for _, c := range candidates {
								if strings.Contains(c, `"surface_response"`) {
									rich++
								}
							}
							if rich > maxRichCandidates {
								for idx, c := range candidates {
									if strings.Contains(c, `"surface_response"`) {
										candidates = append(candidates[:idx], candidates[idx+1:]...)
										break
									}
								}
							}
						} else {
							const maxNoiseCandidates = 50
							noiseCount := 0
							for _, c := range candidates {
								if !strings.Contains(c, `"surface_response"`) {
									noiseCount++
								}
							}
							if noiseCount >= maxNoiseCandidates {
								for idx, c := range candidates {
									if !strings.Contains(c, `"surface_response"`) {
										candidates = append(candidates[:idx], candidates[idx+1:]...)
										break
									}
								}
							}
							candidates = append(candidates, candStr)
						}
					}
					start = -1
				}
			}
		}
	}

	return candidates
}

// findKeyOutsideStrings locates the first `"key"` occurrence that is not
// inside a JSON string literal and is followed (past optional whitespace) by
// a colon — i.e. a real object key, not a mention inside reasoning text. It
// returns the index of the opening quote, or -1. A strings.Index for the key
// would happily match a decoy the model quoted inside another string and
// salvage the wrong value.
func findKeyOutsideStrings(s, key string) int {
	needle := `"` + key + `"`
	inString := false
	escape := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if b == '\\' {
				escape = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			if strings.HasPrefix(s[i:], needle) {
				rest := s[i+len(needle):]
				trimmed := strings.TrimLeft(rest, " \t\r\n")
				if strings.HasPrefix(trimmed, ":") {
					return i
				}
			}
			inString = true
			continue
		}
	}
	return -1
}

// scanJSONString decodes the JSON string value whose opening quote is s[open].
// It returns the decoded value and the index just past the closing quote.
// ok is false when the string is unterminated or an escape is truncated.
//
// This is the single home for salvage-time string scanning: the surface
// salvager and the generic field extractor both used to carry their own copy,
// and both dropped the backslash of any escape they did not know — so a
// \u00e9 surfaced as the literal text "u00e9".
func scanJSONString(s string, open int) (val string, end int, ok bool) {
	return scanJSONStringInner(s, open, false)
}

// scanJSONStringPrefix decodes as much of the string at s[open] as is
// complete: it stops before the first truncated escape or the end of input
// instead of failing the whole value. ok is false when nothing decodable
// precedes the truncation.
func scanJSONStringPrefix(s string, open int) (val string, ok bool) {
	val, _, ok = scanJSONStringInner(s, open, true)
	return val, ok
}

// scanJSONStringInner is the shared engine. With prefix=true a truncated
// escape or end of input ends the value successfully instead of failing it;
// the salvage path uses that to recover what arrived before truncation.
func scanJSONStringInner(s string, open int, prefix bool) (val string, end int, ok bool) {
	var sb strings.Builder
	i := open + 1
	for i < len(s) {
		c := s[i]
		if c == '"' {
			return sb.String(), i + 1, true
		}
		if c != '\\' {
			sb.WriteByte(c)
			i++
			continue
		}
		// Escape sequence.
		if i+1 >= len(s) {
			break
		}
		e := s[i+1]
		switch e {
		case '"', '\\', '/':
			sb.WriteByte(e)
			i += 2
		case 'b':
			sb.WriteByte('\b')
			i += 2
		case 'f':
			sb.WriteByte('\f')
			i += 2
		case 'n':
			sb.WriteByte('\n')
			i += 2
		case 'r':
			sb.WriteByte('\r')
			i += 2
		case 't':
			sb.WriteByte('\t')
			i += 2
		case 'u':
			r, size, ok := decodeUnicodeEscape(s, i)
			if !ok {
				if !prefix {
					return "", 0, false
				}
				return sb.String(), i, sb.Len() > 0
			}
			sb.WriteRune(r)
			i += size
		default:
			// Unknown escape: keep the byte, drop the backslash, matching
			// encoding/json's tolerance for invalid escapes.
			sb.WriteByte(e)
			i += 2
		}
	}
	if prefix {
		return sb.String(), i, sb.Len() > 0
	}
	return "", 0, false
}

// decodeUnicodeEscape decodes the \uXXXX (or surrogate pair) at s[i],
// where s[i] is the backslash. It returns the rune and the number of bytes
// consumed.
func decodeUnicodeEscape(s string, i int) (rune, int, bool) {
	if i+5 >= len(s) {
		return 0, 0, false
	}
	hi, ok := parseHex4(s[i+2 : i+6])
	if !ok {
		return 0, 0, false
	}
	consumed := 6
	r := rune(hi)
	// Surrogate pair: \uD83D\uDE00.
	if hi >= 0xD800 && hi <= 0xDBFF && i+11 < len(s) && s[i+6] == '\\' && s[i+7] == 'u' {
		lo, ok := parseHex4(s[i+8 : i+12])
		if ok && lo >= 0xDC00 && lo <= 0xDFFF {
			r = 0x10000 + (rune(hi)-0xD800)*0x400 + (rune(lo) - 0xDC00)
			consumed = 12
		}
	}
	return r, consumed, true
}

// decodeStreamEscape decodes the \uXXXX escape at data[bs] (the backslash)
// for the incremental stream parser. Unlike the batch decoder it must tell
// "truncated, wait for more" apart from "decisive, emit now":
//
//   - fewer than 6 bytes available, or a high surrogate whose pair region is
//     still growing: hold=true, and the caller rewinds to the backslash.
//   - anything decisive (a complete BMP escape, a complete pair, definitive
//     garbage): hold=false, consumed bytes to advance, and the rune —
//     utf8.RuneError for lone surrogates and bad hex, matching
//     encoding/json's tolerance.
func decodeStreamEscape(data string, bs int) (consumed int, r rune, hold bool) {
	if bs+6 > len(data) {
		return 0, 0, true
	}
	hi, ok := parseHex4(data[bs+2 : bs+6])
	if !ok {
		return 6, utf8.RuneError, false
	}
	if hi < 0xD800 || hi > 0xDBFF {
		return 6, rune(hi), false
	}
	// High surrogate: the pair may still be arriving.
	rest := data[bs+6:]
	if rest == "" || rest == `\` {
		return 0, 0, true
	}
	if len(rest) >= 2 && rest[0] == '\\' && rest[1] == 'u' && len(rest) < 6 {
		return 0, 0, true
	}
	if len(rest) >= 6 && rest[0] == '\\' && rest[1] == 'u' {
		if lo, ok := parseHex4(rest[2:6]); ok && lo >= 0xDC00 && lo <= 0xDFFF {
			return 12, 0x10000 + (rune(hi)-0xD800)*0x400 + (rune(lo) - 0xDC00), false
		}
	}
	return 6, utf8.RuneError, false
}

// parseHex4 parses exactly four hex digits.
func parseHex4(s string) (uint16, bool) {
	if len(s) != 4 {
		return 0, false
	}
	var v uint16
	for i := 0; i < 4; i++ {
		c := s[i]
		var d uint16
		switch {
		case c >= '0' && c <= '9':
			d = uint16(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint16(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint16(c-'A') + 10
		default:
			return 0, false
		}
		v = v*16 + d
	}
	return v, true
}
