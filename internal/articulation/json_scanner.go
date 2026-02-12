package articulation

// findJSONCandidates scans the input string for top-level JSON object candidates.
// It returns a slice of strings, each representing a potential JSON object.
// It handles nested braces and string escaping to correctly identify boundaries.
//
// This function uses a byte-level state machine to efficiently skip over
// strings and non-JSON content, providing significantly better performance
// than regex-based extraction for large inputs.
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
		} else if b == '}' {
			if depth > 0 {
				depth--
				if depth == 0 && start != -1 {
					// Found a complete top-level object
					candidates = append(candidates, s[start:i+1])
					start = -1
				}
			}
		}
	}

	return candidates
}
