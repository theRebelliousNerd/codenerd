package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedLearnedKernel builds a kernel whose schema validator knows two test
// predicates, with learned-rule persistence pointed at a temp dir.
func seedLearnedKernel(t *testing.T) (*RealKernel, string) {
	t.Helper()
	k := setupMockKernel(t)
	k.SetSchemas("Decl base_val(Name).\nDecl derived_val(Name).")
	k.SetPolicy("")
	dir := t.TempDir()
	k.manglePath = dir
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return k, dir
}

const upliftRule = `derived_val(X) :- base_val(X).`

// Empty candidates are rejected before the interceptor runs, and rejection
// must leave neither memory nor disk touched: a nil error used to persist
// a junk comment block to learned.mg.
func TestHotLoadLearnedRule_RejectsEmpty(t *testing.T) {
	k, dir := seedLearnedKernel(t)
	baseline := k.GetLearned()
	for _, rule := range []string{"", "   ", "\n\t\n"} {
		if err := k.HotLoadLearnedRule(rule); err == nil {
			t.Fatalf("HotLoadLearnedRule(%q) = nil, want error", rule)
		} else if !strings.Contains(err.Error(), "empty") {
			t.Fatalf("HotLoadLearnedRule(%q) error = %v, want it to name emptiness", rule, err)
		}
	}
	if got := k.GetLearned(); got != baseline {
		t.Fatalf("learned text mutated by rejections:\nbaseline %q\ngot %q", baseline, got)
	}
	if _, err := os.Stat(filepath.Join(dir, "learned.mg")); !os.IsNotExist(err) {
		t.Fatalf("learned.mg must not be created by rejected rules (stat err = %v)", err)
	}
}

// A valid learned rule is installed AND persisted: after Evaluate, the
// derivation is queryable, and learned.mg contains the rule for reboot.
func TestHotLoadLearnedRule_PersistsAndDerives(t *testing.T) {
	k, dir := seedLearnedKernel(t)
	if err := k.HotLoadLearnedRule(upliftRule); err != nil {
		t.Fatalf("HotLoadLearnedRule: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "learned.mg"))
	if err != nil {
		t.Fatalf("learned.mg not persisted: %v", err)
	}
	if !strings.Contains(string(raw), upliftRule) {
		t.Fatalf("learned.mg does not contain the rule:\n%s", raw)
	}
	if err := k.Assert(Fact{Predicate: "base_val", Args: []any{"a"}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	rows, err := k.Query("derived_val")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("derived rows = %d, want 1 (rule must be installed, not just validated)", len(rows))
	}
}

// HotLoadRule validates without installing: this pins the documented
// contract so a future "fix" cannot silently turn validation into a load
// (or vice versa) without breaking this test.
func TestHotLoadRule_ValidatesWithoutInstalling(t *testing.T) {
	k, _ := seedLearnedKernel(t)
	if err := k.HotLoadRule(upliftRule); err != nil {
		t.Fatalf("HotLoadRule: %v", err)
	}
	if got := k.GetLearned(); strings.Contains(got, upliftRule) {
		t.Fatalf("HotLoadRule installed the rule into learned text:\n%s", got)
	}
	if err := k.Assert(Fact{Predicate: "base_val", Args: []any{"a"}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	rows, err := k.Query("derived_val")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("derived rows = %d after HotLoadRule, want 0 (validate-only)", len(rows))
	}

	// The same rule through AppendPolicy DOES install: the control case
	// proving the rule itself derives fine.
	k.AppendPolicy("\n" + upliftRule + "\n")
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate after AppendPolicy: %v", err)
	}
	rows, err = k.Query("derived_val")
	if err != nil {
		t.Fatalf("Query after AppendPolicy: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("derived rows = %d after AppendPolicy, want 1", len(rows))
	}
}
