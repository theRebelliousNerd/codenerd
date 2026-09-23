package articulation

import "testing"

// Inside a string literal is the kernel gate's question: whether a string may
// carry punctuation is a property of its predicate. The closing checkpoint of
// campaign aab9612b failed three times with a well-formed /fail verdict, and
// each was dropped here for the semicolons in its reason.
func TestShellCheckedText_StringsAreLeftToTheKernelGate(t *testing.T) {
	for _, update := range []string{
		`checkpoint_verdict("Phase 6 - Verification Gate", /fail, "5 files lack front-matter; verified-against not a commit & 2 files > stale", 90).`,
		`observation("adr_slot_absent", "ADR-001 missing; TODO-01a open").`,
		`pending_action("a1", /exec_cmd, "echo ok; rm -rf /", "", 0).`,
	} {
		if got := shellCheckedText(update); containsShellMeta(got) {
			t.Fatalf("string contents reached the emitter's check: %q (checked text %q)", update, got)
		}
	}
}

// Outside its string literals every atom is held to the rule: there, none of
// the characters belongs in a fact.
func TestShellCheckedText_MetacharactersAroundTheStringsAreSeen(t *testing.T) {
	for _, hostile := range []string{
		`checkpoint_verdict("p", /pass, "ok", 90); exec_cmd("rm").`,
		`checkpoint_verdict("p", /pass, "ok", $HOME).`,
		`checkpoint_verdict("p", /pass, "unterminated ; reason, 90).`,
		`checkpoint_verdict("p", /pass, "escaped \" quote", 90) | cat.`,
		`inject($PATH).`,
	} {
		if got := shellCheckedText(hostile); !containsShellMeta(got) {
			t.Fatalf("metacharacters outside a string literal went unseen in %q (checked text %q)", hostile, got)
		}
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
