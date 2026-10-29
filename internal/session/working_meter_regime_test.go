package session

import (
	"strings"
	"testing"
)

// commitRegimeInput returns a regime input that produces the commit-regime
// sentence together with that sentence. It tries the spellings
// workingRegimeText accepts so the test does not hard-code the constant.
func commitRegimeInput(t *testing.T) (string, string) {
	t.Helper()
	for _, regime := range []string{"commit", "/commit", " commit ", " /commit "} {
		if text := workingRegimeText(regime); text != "" {
			return regime, text
		}
	}
	t.Fatal("workingRegimeText(commit) returned empty for all candidate spellings")
	return "", ""
}

func openRegimeInput(t *testing.T) string {
	t.Helper()
	for _, regime := range []string{"", "open", "/open"} {
		if workingRegimeText(regime) == "" {
			return regime
		}
	}
	t.Fatal("could not find an open-regime input with empty regime text")
	return ""
}

func TestWithRegimePromptCommitAppendsOnce(t *testing.T) {
	regime, want := commitRegimeInput(t)
	base := "The tests pass, but no test executes these lines of code you changed: foo.go:1-2."
	got := withRegimePrompt(base, regime)
	if got == base {
		t.Fatalf("withRegimePrompt did not append commit-regime text; got %q", got)
	}
	if got != base+"\n\n"+want {
		t.Fatalf("unexpected withRegimePrompt result:\n got %q\nwant %q", got, base+"\n\n"+want)
	}
	if n := strings.Count(got, want); n != 1 {
		t.Fatalf("regime sentence appears %d times, want exactly 1: %q", n, got)
	}
}

func TestWithRegimePromptResendStaysSingle(t *testing.T) {
	regime, want := commitRegimeInput(t)
	base := "The tests pass, but no test executes these lines of code you changed: foo.go:1-2."
	first := withRegimePrompt(base, regime)
	// Simulate the coverage repair round being re-sent after a read-only
	// round: the already-decorated prompt goes through the helper again.
	second := withRegimePrompt(first, regime)
	if second != first {
		t.Fatalf("re-sent prompt changed:\nfirst  %q\nsecond %q", first, second)
	}
	if n := strings.Count(second, want); n != 1 {
		t.Fatalf("regime sentence appears %d times after re-send, want exactly 1: %q", n, second)
	}
}

func TestWithRegimePromptAlreadyDecoratedUnchanged(t *testing.T) {
	regime, want := commitRegimeInput(t)
	decorated := "demand body" + "\n\n" + want
	if got := withRegimePrompt(decorated, regime); got != decorated {
		t.Fatalf("already-decorated prompt changed:\n got %q\nwant %q", got, decorated)
	}
}

func TestWithRegimePromptOpenRegimeUnchanged(t *testing.T) {
	regime := openRegimeInput(t)
	cases := map[string]string{
		"plain": "repair demand without regime",
		"empty": "",
	}
	for name, base := range cases {
		t.Run(name, func(t *testing.T) {
			if got := withRegimePrompt(base, regime); got != base {
				t.Fatalf("open-regime prompt changed:\n got %q\nwant %q", got, base)
			}
		})
	}
}

func TestWithRegimePromptEmptyPromptCommit(t *testing.T) {
	regime, want := commitRegimeInput(t)
	got := withRegimePrompt("", regime)
	if n := strings.Count(got, want); n != 1 {
		t.Fatalf("empty prompt produced %d regime sentences, want exactly 1: %q", n, got)
	}
}
