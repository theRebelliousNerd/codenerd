package types

import "strings"

// Atom builds a valid Mangle name constant from an arbitrary identifier.
//
// It exists to make the correct thing the short thing. The repo-wide sweep in
// fact_conventions_guard_test.go found eleven assert sites passing a plain Go
// string into an argument the corpus declares `/name` — "hands_free",
// "execution_error", "branch", "pattern_not_found". Every one of those becomes a
// StringType constant, and every rule written to match the declared name simply
// never fires: no error, no log, just a policy that does nothing. The fix at each
// site is one call, and it should not require the author to remember the naming
// rules:
//
//	Args: []any{path, types.Atom("pattern_not_found")}  // -> /pattern_not_found
//
// Callers that already hold a well-formed constant should keep writing
// MangleAtom("/literal") — it is checked by ToAtom and reads as the constant it
// is. Atom is for values assembled at runtime from enums, error categories or
// user-facing labels, where the alternative today is a bare string.
//
// The result is always a valid name: everything outside [a-z0-9_-] becomes "_",
// upper case folds down, and an input that reduces to nothing yields /unknown
// rather than the bare "/" that ast.Name rejects.
func Atom(s string) MangleAtom {
	tail := strings.TrimLeft(strings.TrimSpace(s), "/")
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		default:
			return '_'
		}
	}, tail)
	// Collapse runs of underscores so "not found: pattern" does not become
	// /not_found__pattern; the atom is a policy-visible identifier and the
	// difference between one and two underscores is invisible to whoever
	// writes the matching rule.
	for strings.Contains(mapped, "__") {
		mapped = strings.ReplaceAll(mapped, "__", "_")
	}
	mapped = strings.Trim(mapped, "_")
	if mapped == "" {
		return MangleAtom("/unknown")
	}
	return MangleAtom("/" + mapped)
}
