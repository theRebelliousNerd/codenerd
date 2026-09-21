package articulation

import "testing"

// The closing checkpoint of campaign aab9612b failed three times with a
// well-formed /fail verdict, and each was dropped for the semicolons in its
// reason: the campaign reported that no verdict could be determined.
func TestShellCheckedText_ProseVerdictStringsAreNotShellText(t *testing.T) {
	verdict := `checkpoint_verdict("Phase 6 - Verification Gate", /fail, "5 files lack front-matter; verified-against not a commit & 2 files > stale", 90).`
	if got := shellCheckedText(verdict); containsShellMeta(got) {
		t.Fatalf("a prose reason's punctuation is still treated as shell text: %q", got)
	}
}

// Outside its string literals a verdict is held to the same rule as any fact.
func TestShellCheckedText_ProseVerdictIsStillCheckedOutsideItsStrings(t *testing.T) {
	for _, hostile := range []string{
		`checkpoint_verdict("p", /pass, "ok", 90); exec_cmd("rm").`,
		`checkpoint_verdict("p", /pass, "ok", $HOME).`,
		`checkpoint_verdict("p", /pass, "unterminated ; reason, 90).`,
		`checkpoint_verdict("p", /pass, "escaped \" quote", 90) | cat.`,
	} {
		if got := shellCheckedText(hostile); !containsShellMeta(got) {
			t.Fatalf("metacharacters outside a string literal went unseen in %q (checked text %q)", hostile, got)
		}
	}
}

// Every other predicate keeps the whole-atom check: a string argument there
// may be an action's input, which is what the check exists for.
func TestShellCheckedText_OtherPredicatesAreCheckedWhole(t *testing.T) {
	update := `pending_action("a1", /exec_cmd, "echo ok; rm -rf /").`
	if got := shellCheckedText(update); got != update {
		t.Fatalf("a non-verdict predicate lost part of its text to the check: %q", got)
	}
}

func containsShellMeta(s string) bool {
	for _, r := range s {
		switch r {
		case '`', '$', ';', '|', '&', '<', '>':
			return true
		}
	}
	return false
}
