package campaign

import "testing"

// Observed live 2026-08-08 on campaign fc6472c2: phase 1 came back with four
// tasks that were two tasks each emitted twice — "Modify
// internal/session/gate_names.go to add a doc comment" and "Run go test
// ./internal/session", duplicated. The IDs were distinct, so nothing downstream
// saw a duplicate; the orchestrator simply ran the same work again, and the two
// test tasks were in_progress concurrently.
func TestNormalizedTaskKey(t *testing.T) {
	cases := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{
			name:  "the observed duplicate pair",
			a:     "Run go test ./internal/session to verify the comment-only change compiles",
			b:     "Run go test ./internal/session to verify the comment-only change compiles",
			equal: true,
		},
		{
			name:  "case and trailing punctuation do not matter",
			a:     "Modify internal/session/gate_names.go to add a doc comment.",
			b:     "modify internal/session/gate_names.go to add a doc comment",
			equal: true,
		},
		{
			name:  "collapsed whitespace",
			a:     "Run   go test    ./internal/session",
			b:     "Run go test ./internal/session",
			equal: true,
		},
		// Paths must stay distinguishing, or two different files collapse into
		// one task and real work is deleted.
		{
			name:  "different file paths are different tasks",
			a:     "Modify internal/session/gate_names.go to add a doc comment",
			b:     "Modify internal/session/critic.go to add a doc comment",
			equal: false,
		},
		{
			name:  "different verbs are different tasks",
			a:     "Run go test ./internal/session",
			b:     "Read go test output for ./internal/session",
			equal: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ka, kb := normalizedTaskKey(tc.a), normalizedTaskKey(tc.b)
			if (ka == kb) != tc.equal {
				t.Errorf("normalizedTaskKey(%q)=%q vs (%q)=%q; equal=%v want %v",
					tc.a, ka, tc.b, kb, ka == kb, tc.equal)
			}
		})
	}
}

// An empty or punctuation-only description yields an empty key, which the
// caller treats as "cannot compare" and lets through rather than suppressing.
// Dropping a task because its description was unparseable would be worse than
// running it twice.
func TestNormalizedTaskKey_EmptyIsNotAMatch(t *testing.T) {
	for _, in := range []string{"", "   ", "...", "!!!"} {
		if got := normalizedTaskKey(in); got != "" {
			t.Errorf("normalizedTaskKey(%q) = %q; want empty so the caller does not suppress it", in, got)
		}
	}
}
