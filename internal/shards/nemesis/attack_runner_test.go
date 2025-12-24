package nemesis

import (
	"strings"
	"testing"
)

func TestWrapTestCodeUsesExistingTest(t *testing.T) {
	runner := &AttackRunner{}
	code := "package nemesis_attack\n\nfunc TestExisting(t *testing.T) {}\n"
	script := &AttackScript{
		TestCode: code,
	}

	got := runner.wrapTestCode(script)
	if got != code {
		t.Fatalf("expected test code to be returned unchanged")
	}
}

func TestWrapTestCodeBuildsWrapper(t *testing.T) {
	runner := &AttackRunner{}
	script := &AttackScript{
		Name:       "attack example",
		Hypothesis: "attack hypothesis",
		Category:   "logic",
		TestCode:   "panic(\"boom\")",
	}

	got := runner.wrapTestCode(script)
	if !strings.Contains(got, "package nemesis_attack") {
		t.Fatalf("expected package declaration")
	}
	if !strings.Contains(got, "func Test") {
		t.Fatalf("expected Test wrapper")
	}
	if !strings.Contains(got, "TestAttackExample") {
		t.Fatalf("expected sanitized test name, got %q", got)
	}
	if !strings.Contains(got, script.Hypothesis) {
		t.Fatalf("expected hypothesis to be included")
	}
}
