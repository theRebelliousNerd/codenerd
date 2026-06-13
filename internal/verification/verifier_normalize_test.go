package verification

import "testing"

func TestNormalizeIntentVerb(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "/general"},                 // empty -> general
		{"   ", "/general"},              // whitespace-only -> general
		{"/fix", "/fix"},                 // already a verb, passes through
		{"/custom/path", "/custom/path"}, // any slash-prefixed string is preserved
		{"coder", "/fix"},
		{"tester", "/test"},
		{"reviewer", "/review"},
		{"researcher", "/research"},
		{"nemesis", "/attack"},
		{"librarian", "/learn"},
		{"planner", "/plan"},
		{"legislator", "/legislate"},
		{"constitution", "/audit"},
		{"CODER", "/fix"},                       // case-insensitive role mapping
		{"unknownrole", "/consult/unknownrole"}, // default -> consult/<role>
	}
	for _, c := range cases {
		if got := normalizeIntentVerb(c.in); got != c.want {
			t.Errorf("normalizeIntentVerb(%q)=%q, want %q", c.in, got, c.want)
		}
	}
}

func TestSetTaskExecutor(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, nil)
	// Setting a nil executor must not panic and should store the value.
	v.SetTaskExecutor(nil)
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.taskExecutor != nil {
		t.Error("expected taskExecutor to remain nil after SetTaskExecutor(nil)")
	}
}
