package chat

import "testing"

// A clause's verb is the one it leads with, or failing that the first corpus
// synonym it contains as whole words -- never a synonym found inside another
// word. The corpus loop took the first entry with a synonym anywhere in the
// clause as a substring: "hi" inside "this" made "update this file" a /greet
// step with the target "s file", a step no shard runs, and the kernel then
// delegated the whole request instead of decomposing it
// (multi_step_plan_unrouted).
func TestParseVerbAndTarget_TheLeadingVerbWinsAndWordsAreWhole(t *testing.T) {
	cases := []struct{ clause, verb, target string }{
		{"review this file", "/review", "this file"},
		{"fix the prefix handling", "/fix", "the prefix handling"},
		{"then fix this", "/fix", "this"},
		{"then write docs for this", "/document", "for this"},
		{"fixed the bug", "/fix", "the bug"},
	}
	for _, tc := range cases {
		verb, target := parseVerbAndTarget(tc.clause)
		if verb != tc.verb || target != tc.target {
			t.Errorf("parseVerbAndTarget(%q) = %q, %q; want %q, %q", tc.clause, verb, target, tc.verb, tc.target)
		}
	}
	for _, clause := range []string{"update this file", "make this faster", "move this to utils", "change this behaviour"} {
		if verb, target := parseVerbAndTarget(clause); verb == "/greet" {
			t.Errorf("parseVerbAndTarget(%q) = /greet, %q: a synonym inside another word", clause, target)
		}
	}
	// A first word that is no verb is not made one.
	if verb, _ := parseVerbAndTarget("the logging module"); verb == "/the" {
		t.Errorf(`"the logging module" parsed as the verb /the`)
	}
}

func TestWholeWordIndex(t *testing.T) {
	for _, tc := range []struct {
		text, word string
		want       int
	}{
		{"update this file", "hi", -1},
		{"say hi to them", "hi", 4},
		{"hi", "hi", 0},
		{"this and hi", "hi", 9},
		{"write docs for this", "write docs", 0},
		{"rewrite docs", "write docs", -1},
	} {
		if got := wholeWordIndex(tc.text, tc.word); got != tc.want {
			t.Errorf("wholeWordIndex(%q, %q) = %d, want %d", tc.text, tc.word, got, tc.want)
		}
	}
}
