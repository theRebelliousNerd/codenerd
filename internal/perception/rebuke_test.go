package perception

import "testing"

// IsRebuke is the fallback evidence for the learning-loop trigger: the kernel
// reads it only on a turn Jev did not judge (policy/jev_learning.mg).
func TestIsRebuke(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"bad bot", true},
		{"wrong", true},
		{"no that's not right", true},
		{"you didn't do what I asked", true},
		{"that's incorrect", true},
		{"you made a mistake", true},
		{"WRONG!", true},
		{"Bad bot, wrong answer", true},
		{"no that's not right, Max panics on an empty slice rather than returning zero", true},

		// A bare stop is the user stopping running work, not a rebuke.
		{"stop", false},
		{"stop doing that", false},
		{"the tests fail", false},
		{"The failsafe mechanism caught the error", false},
		{"I need a stopwatch function", false},
		// A request that mentions a failure is a task, not a complaint.
		{"Mean in stats.go is wrong - go test says TestMean fails and it returns 6 instead of 4", false},
		{"", false},
		{"hello world", false},
		{"looks good", false},
		{"correct", false},
	}
	for _, tt := range tests {
		if got := IsRebuke(tt.input); got != tt.want {
			t.Errorf("IsRebuke(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
