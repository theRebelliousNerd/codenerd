package core

import (
	"strings"
	"testing"

	"codenerd/internal/mangle"
	"codenerd/internal/mangle/transpiler"
)

// A bound list is not documentation. Two live consumers read it and act on it,
// and both were making the wrong call for the intent-identity slots while they
// were declared /string:
//
//   - mangle.AtomValidator (grammar.go UpdateFromProgramInfo -> boundToArgType,
//     compatibleTypes) type-checks every LLM-emitted atom against it. It warned
//     "argument 1 (ID): expected quoted string, got name constant" about the
//     exact shape all five Go producers of user_intent/5 assert.
//   - transpiler.Sanitizer (used by the Ouroboros/Legislator loop in
//     internal/autopoiesis) interns a string into a name for any slot the Decl
//     types /name. With the slot declared /string it left the LLM's
//     user_intent("current_intent", ...) alone, and a newly legislated rule
//     that cannot match a single fact in the store is worse than no rule: it
//     analyzes, it derives nothing, and nothing reports it.
//
// So the Decl decides whether self-written policy can ever fire. That earns a
// test rather than a comment.
func TestIntentDeclDrivesAtomInterningAndValidation(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	info := k.GetProgramInfo()
	if info == nil {
		t.Fatal("GetProgramInfo() returned nil — nothing below can be measured")
	}

	v := mangle.NewAtomValidator()
	v.UpdateFromProgramInfo(info)

	// No type complaint on what production actually asserts. Unknown-name-constant
	// warnings are a separate axis (the validator keeps a hand-written vocabulary)
	// and are not what this test is about.
	for _, atom := range []string{
		`user_intent(/current_intent, /query, /explain, "auth.go", "")`,
		`executive_processed_intent(/current_intent)`,
		`processed_intent(/current_intent)`,
		`no_action_reason(/current_intent, /no_route)`,
		`context_token("security")`,
	} {
		for _, e := range v.ValidateAtom(atom).Errors {
			if strings.Contains(e.Message, "expected") {
				t.Errorf("ValidateAtom(%s): %s — the Decl disagrees with what Go asserts", atom, e.Message)
			}
		}
	}

	// The interning pass, on the rule shape an LLM writes when it has only seen
	// quoted values. Every /name slot must come out with a leading slash.
	s := transpiler.NewSanitizer()
	s.UpdateFromProgramInfo(info)
	out, err := s.SanitizeAtoms(
		`next_action(/probe) :- user_intent("current_intent", "query", "explain", "auth.go", ""), ` +
			`!executive_processed_intent("current_intent").`)
	if err != nil {
		t.Fatalf("SanitizeAtoms error = %v", err)
	}
	for _, want := range []string{
		"user_intent(/current_intent,",
		"!executive_processed_intent(/current_intent)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sanitized rule missing %q — the intent id slot is not being interned:\n%s", want, out)
		}
	}
	// Target stays a string: it holds paths and noun phrases, and interning it
	// would turn "auth.go" into the name /auth.go.
	if !strings.Contains(out, `"auth.go"`) {
		t.Errorf("sanitized rule lost the quoted Target — Target must stay /string:\n%s", out)
	}
}
