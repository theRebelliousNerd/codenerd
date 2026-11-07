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
// upper case folds down, runs of "_" collapse to one, leading/trailing "_"
// and "-" trim away, a leading digit gains an "n_" prefix so the atom stays
// letter-leading, and an input that reduces to nothing yields /unknown
// rather than the bare "/" that ast.Name rejects.
func Atom(s string) MangleAtom {
	tail := strings.TrimLeft(strings.TrimSpace(s), "/")
	var b strings.Builder
	b.Grow(len(tail))
	prevUnderscore := false
	for _, r := range tail {
		var out rune
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			out = r
		case r >= 'A' && r <= 'Z':
			out = r + ('a' - 'A')
		default:
			out = '_'
		}
		if out == '_' {
			if prevUnderscore {
				continue
			}
			prevUnderscore = true
		} else {
			prevUnderscore = false
		}
		b.WriteRune(out)
	}
	// Collapse runs of underscores so "not found: pattern" does not become
	// /not_found__pattern; the atom is a policy-visible identifier and the
	// difference between one and two underscores is invisible to whoever
	// writes the matching rule. Single pass above already collapses; trimming
	// both "_" and "-" keeps "/-foo-" and "/---" from escaping as invalid names.
	mapped := strings.Trim(b.String(), "_-")
	if mapped == "" {
		return MangleAtom("/unknown")
	}
	if mapped[0] >= '0' && mapped[0] <= '9' {
		return MangleAtom("/n_" + mapped)
	}
	return MangleAtom("/" + mapped)
}
