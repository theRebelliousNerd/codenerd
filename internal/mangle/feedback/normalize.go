package feedback

import (
	"strconv"
	"strings"
)

// NormalizeRuleInput prepares a rule for parsing by fixing common escape issues
// in LLM output. It fixes:
// 1. Prolog-style negation (\+) to Mangle negation (!) outside strings
// 2. Backslashes inside quoted strings (unknown escapes get doubled so they
// parse as literal text instead of failing the lexer)
//
// The pass is quote-aware: \+ inside a string literal is data, not negation,
// and a quote preceded by an escaped backslash (\\") still toggles the
// string state.
func NormalizeRuleInput(rule string) string {
	rule = strings.TrimSpace(rule)

	// If the entire rule is a JSON-quoted string, unquote it first.
	if strings.HasPrefix(rule, "\"") && strings.HasSuffix(rule, "\"") {
		if unquoted, err := strconv.Unquote(rule); err == nil {
			rule = strings.TrimSpace(unquoted)
		}
	}

	if !strings.Contains(rule, "\\") {
		return rule
	}

	var sb strings.Builder
	sb.Grow(len(rule))

	inQuote := false
	for i := 0; i < len(rule); i++ {
		c := rule[i]
		switch {
		case c == '"':
			// Toggle unless escaped by an odd run of backslashes: \\" is an
			// escaped backslash followed by a real string boundary.
			slashes := 0
			for j := i - 1; j >= 0 && rule[j] == '\\'; j-- {
				slashes++
			}
			if slashes%2 == 0 {
				inQuote = !inQuote
			}
			sb.WriteByte(c)
		case c == '\\' && !inQuote && i+1 < len(rule) && rule[i+1] == '+':
			// Prolog negation outside strings only: \+ -> !
			sb.WriteByte('!')
			i++
			for i+1 < len(rule) && isASCIISpace(rule[i+1]) {
				i++
			}
		case c == '\\' && inQuote:
			i = copyStringEscape(rule, i, &sb)
		default:
			sb.WriteByte(c)
		}
	}

	return sb.String()
}

// copyStringEscape copies one backslash escape starting at rule[i] == '\\'
// inside a string literal and returns the index of the last byte consumed.
// Well-formed Mangle escapes pass through untouched; anything else gets its
// backslash doubled so it parses as literal text instead of failing the lex.
func copyStringEscape(rule string, i int, sb *strings.Builder) int {
	if i+1 >= len(rule) {
		sb.WriteString("\\\\")
		return i
	}
	next := rule[i+1]
	switch {
	case next == 'x' && i+3 < len(rule) && isLowerHexDigit(rule[i+2]) && isLowerHexDigit(rule[i+3]) &&
		hexValue(rule[i+2]) < 8:
		// \xHH with lowercase hex and a value in [0x00..0x7F]: the only
		// byte escapes the lexer accepts that Unescape will also decode.
		sb.WriteString(rule[i : i+4])
		return i + 3
	case next == 'u':
		if end, ok := unicodeEscapeEnd(rule, i); ok {
			sb.WriteString(rule[i:end])
			return end - 1
		}
	case isKnownEscape(next):
		sb.WriteByte('\\')
		sb.WriteByte(next)
		return i + 1
	}
	sb.WriteString("\\\\")
	return i
}

// unicodeEscapeEnd returns the end offset of a well-formed \u{h...} escape
// starting at rule[i] == '\\', or ok=false when malformed. The grammar
// demands 4-6 lowercase hex digits; anything else fails the lex.
func unicodeEscapeEnd(rule string, i int) (end int, ok bool) {
	if i+3 >= len(rule) || rule[i+1] != 'u' || rule[i+2] != '{' {
		return 0, false
	}
	j := i + 3
	digits := 0
	for j < len(rule) && isLowerHexDigit(rule[j]) && digits < 6 {
		j++
		digits++
	}
	if digits < 4 || j >= len(rule) || rule[j] != '}' {
		return 0, false
	}
	return j + 1, true
}

// isLowerHexDigit matches the grammar's HEXDIGIT fragment, which is
// lowercase-only: \xE9 fails the lex where \xe9 lexes (and then fails
// Unescape for being out of range — hence the hexValue check above).
func isLowerHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f'
}

func hexValue(b byte) byte {
	if b <= '9' {
		return b - '0'
	}
	return b - 'a' + 10
}

func isASCIISpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}

// isKnownEscape reports whether a backslash escape can be copied blindly into
// a Mangle double-quoted string. The set is exactly the lexer's
// STRING_ESCAPE_SEQ alternatives (parse/gen/Mangle.g4): \\ " n t ' plus the
// backslash-newline continuation. Notably \r \b \f \0 and \` are NOT valid --
// preserving them fails the parse with "token recognition error", so they
// fall through to backslash-doubling and parse as literal text. \x and \u
// need shape validation and are handled by copyStringEscape, not here.
//
// Do NOT add the backtick: a backtick inside a "..." string breaks the lex
// with or without a preceding backslash (proven by probe), and normalize
// does not rewrite quote styles. Single-quoted and long-string literals are
// not tracked either; only double-quoted spans are normalized.
func isKnownEscape(b byte) bool {
	switch b {
	case '\\', '"', 'n', 't', '\'', '\n':
		return true
	default:
		return false
	}
}
