package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/mangle"
	"codenerd/internal/types"
)

// A learned rule may narrow what the constitution permits, never widen it.
// These drive the production entry, RealKernel.HotLoadLearnedRule, which every
// learning caller uses (constitution, executive autopoiesis, legislator), and
// assert on what the constitution then derives.

func learnedRuleKernel(t *testing.T) (*RealKernel, string) {
	t.Helper()
	ws := t.TempDir()
	k, err := NewRealKernelWithWorkspace(ws)
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	return k, ws
}

// pendingDelete asserts a delete the constitution does not permit on its own:
// /delete_file requires permission, and the target is not recoverable.
func pendingDelete(t *testing.T, k *RealKernel) {
	t.Helper()
	if err := k.Assert(types.Fact{Predicate: "pending_action", Args: []any{
		"act_del", types.MangleAtom("/delete_file"), "notes.txt", "{}", int64(1),
	}}); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}
}

func deletePermitted(t *testing.T, k *RealKernel) bool {
	t.Helper()
	rows, err := k.Query("permitted")
	if err != nil {
		t.Fatalf("query permitted: %v", err)
	}
	for _, r := range rows {
		if len(r.Args) > 0 && strings.Contains(types.ExtractString(r.Args[0]), "delete_file") {
			return true
		}
	}
	return false
}

func learnedFileText(t *testing.T, ws string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, ".nerd", "mangle", "learned.mg"))
	if err != nil {
		return ""
	}
	return string(data)
}

// The appeal path derives permitted from has_active_override, which is derived
// from appeal_granted. None of the three was a protected head, so this one
// learned fact used to be validated, persisted, and grant every pending delete.
func TestHotLoadLearnedRule_AnAppealFactCannotGrantPermission(t *testing.T) {
	for _, tc := range []struct{ name, rule, head string }{
		{"appeal fact", `appeal_granted("a9", /delete_file, "learned", 1).`, "appeal_granted"},
		{"override rule", `has_active_override(/delete_file) :- pending_action(_, /delete_file, _, _, _).`, "has_active_override"},
		{"recoverable fact", `file_recoverable("notes.txt").`, "file_recoverable"},
		{"temporary override fact", `temporary_override(/delete_file, 99999999999).`, "temporary_override"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, ws := learnedRuleKernel(t)
			pendingDelete(t, k)
			if deletePermitted(t, k) {
				t.Fatal("baseline: the delete is permitted before anything was learned")
			}

			err := k.HotLoadLearnedRule(tc.rule)
			if err == nil || !strings.Contains(err.Error(), tc.head) {
				t.Errorf("HotLoadLearnedRule(%s) = %v, want it refused naming %s", tc.rule, err, tc.head)
			}
			if deletePermitted(t, k) {
				t.Errorf("after learning %s the constitution permits the delete", tc.rule)
			}
			if strings.Contains(learnedFileText(t, ws), tc.head) {
				t.Errorf("the refused rule was persisted to learned.mg:\n%s", learnedFileText(t, ws))
			}
		})
	}
}

// A second statement is judged too: the head regex saw only the first, so a
// harmless-looking rule carried a grant through validation and into learned.mg.
func TestHotLoadLearnedRule_EveryStatementsHeadIsJudged(t *testing.T) {
	k, ws := learnedRuleKernel(t)
	pendingDelete(t, k)
	rule := `block_action(/learned_block) :- pending_action(_, /delete_file, _, _, _).
appeal_granted("a9", /delete_file, "learned", 1).`
	if err := k.HotLoadLearnedRule(rule); err == nil {
		t.Error("a learned text whose second statement is an appeal was admitted")
	}
	if deletePermitted(t, k) {
		t.Error("the second statement's appeal granted the delete")
	}
	if strings.Contains(learnedFileText(t, ws), "appeal_granted") {
		t.Errorf("persisted:\n%s", learnedFileText(t, ws))
	}
}

// Narrowing stays learnable: the constitution's autopoiesis learns
// dangerous_action rules (its prompt atom teaches the pattern), and
// dangerous_action reaches permitted only through the rule that also requires a
// signed approval and an admin override.
func TestHotLoadLearnedRule_ANarrowingRuleIsStillLearned(t *testing.T) {
	k, ws := learnedRuleKernel(t)
	for _, rule := range []string{
		`dangerous_action(/delete_file) :- pending_action(_, /delete_file, _, _, _).`,
		`block_action(/learned_block) :- pending_action(_, /delete_file, _, _, _).`,
	} {
		if err := k.HotLoadLearnedRule(rule); err != nil {
			t.Errorf("HotLoadLearnedRule(%s) = %v, want it learned", rule, err)
		}
	}
	if !strings.Contains(learnedFileText(t, ws), "dangerous_action(/delete_file)") {
		t.Errorf("the narrowing rule was not persisted:\n%s", learnedFileText(t, ws))
	}
}

// What a control packet may not write, a learned rule may not derive: one set
// (hostWitnessPredicates) serves both gates.
func TestHotLoadLearnedRule_AHostWitnessIsNotLearnable(t *testing.T) {
	k, _ := learnedRuleKernel(t)
	for _, rule := range []string{
		`build_state(/passing).`,
		`turn_done(/t1) :- pending_action(_, /delete_file, _, _, _).`,
	} {
		if err := k.HotLoadLearnedRule(rule); err == nil {
			t.Errorf("HotLoadLearnedRule(%s) admitted a host witness", rule)
		}
	}
	for pred := range hostWitnessPredicates {
		if predicateAllowed(pred, MangleUpdatePolicy{}) {
			t.Errorf("predicateAllowed(%s) with no allowlist = true", pred)
		}
		if err := k.ValidateLearnedRule(pred + "(/x)."); err == nil || !strings.Contains(err.Error(), pred) {
			t.Errorf("a learned %s fact = %v, want it refused", pred, err)
		}
	}
}

// The grant path is derived from the constitution, so it covers what the
// hand-kept list names and what it missed. Every predicate on it is refused.
func TestLearnedHeads_TheDerivedGrantPathIsRefusedWhole(t *testing.T) {
	k, _ := learnedRuleKernel(t)
	path, err := mangle.GrantPathOfSource(k.GetSchemas() + "\n" + k.GetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"permitted", "safe_action", "pending_action", "admin_override", "signed_approval",
		"permitted_action", "permission_check_result", "has_active_override", "appeal_granted",
		"temporary_override", "file_recoverable"} {
		if _, ok := path.Contains(want); !ok {
			t.Errorf("grant path %v lacks %s", path.Sorted(), want)
		}
	}
	for _, narrowing := range []string{"dangerous_action", "dangerous_content", "block_action"} {
		if via, ok := path.Contains(narrowing); ok {
			t.Errorf("%s is on the grant path (through %s); it only narrows permitted", narrowing, via)
		}
	}
	for _, pred := range path.Sorted() {
		if err := k.ValidateLearnedRule(pred + "(A) :- pending_action(A, _, _, _, _)."); err == nil || !strings.Contains(err.Error(), "protected predicate \""+pred+"\"") {
			t.Errorf("a learned rule deriving %s = %v, want it refused as a protected predicate", pred, err)
		}
	}
}

// The program the grant path is derived from is the one the rule would join
// now: a grant rule appended after the validator was built is covered.
func TestLearnedHeads_AGrantAppendedAtRuntimeIsCovered(t *testing.T) {
	k, _ := learnedRuleKernel(t)
	pendingDelete(t, k)
	learned := `runtime_grant(A) :- pending_action(_, A, _, _, _).`
	if err := k.ValidateLearnedRule(learned); err != nil {
		t.Fatalf("before the grant rule exists, runtime_grant is a fresh predicate: %v", err)
	}
	k.AppendPolicy(`Decl runtime_grant(Action) bound [/name].
permitted(Action, Target, Payload) :- runtime_grant(Action), pending_action(_, Action, Target, Payload, _).`)
	if err := k.HotLoadLearnedRule(learned); err == nil || !strings.Contains(err.Error(), "runtime_grant") {
		t.Errorf("after AppendPolicy made runtime_grant a grant premise, HotLoadLearnedRule = %v, want it refused", err)
	}
	if deletePermitted(t, k) {
		t.Error("the learned runtime_grant rule granted the delete")
	}
}

// A rule learned before the grant path was derived is commented out when the
// kernel boots, instead of granting from learned.mg forever.
func TestBoot_ALearnedGrantAlreadyOnDiskIsHealed(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "mangle")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "learned.mg"),
		[]byte("appeal_granted(\"a9\", /delete_file, \"learned\", 1).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := NewRealKernelWithWorkspace(ws)
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	pendingDelete(t, k)
	if deletePermitted(t, k) {
		t.Error("the appeal persisted in learned.mg granted the delete after boot")
	}
	if text := learnedFileText(t, ws); !strings.Contains(text, "# SELF-HEALED") {
		t.Errorf("learned.mg was not healed:\n%s", text)
	}
}

// A program whose grant path cannot be derived admits no learned rule, and a
// boot-time heal does not persist that verdict over the rules on disk.
func TestLearnedHeads_WithoutAGrantPathNothingIsLearned(t *testing.T) {
	sv := mangle.NewSchemaValidator("Decl block_action(Reason) bound [/name].", "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatal(err)
	}
	_, derr := mangle.GrantPathOfSource("this is not mangle (")
	if derr == nil {
		t.Fatal("GrantPathOfSource accepted an unparsable program")
	}
	err := sv.ValidateLearnedRuleProtected(`block_action(/x).`, mangle.LearnedHeadProtection{GrantPathErr: derr})
	if !errors.Is(err, mangle.ErrGrantPathUnknown) {
		t.Fatalf("ValidateLearnedRuleProtected without a grant path = %v, want ErrGrantPathUnknown", err)
	}

	// The kernel whose policy stops parsing refuses every learned rule, and
	// its heal leaves learned.mg's rules on disk as they were.
	k, _ := learnedRuleKernel(t)
	k.AppendPolicy("this is not mangle (")
	if err := k.ValidateLearnedRule(`block_action(/x).`); !errors.Is(err, mangle.ErrGrantPathUnknown) {
		t.Errorf("ValidateLearnedRule on an unparsable program = %v, want ErrGrantPathUnknown", err)
	}
	res := k.validateLearnedRulesContent("block_action(/x).\n", "", true)
	if strings.Contains(res.healedText, "SELF-HEALED") {
		t.Errorf("an unknown grant path healed a rule it said nothing about:\n%s", res.healedText)
	}
}
