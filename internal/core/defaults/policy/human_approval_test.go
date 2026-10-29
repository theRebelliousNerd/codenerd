package policy

import (
	"os"
	"testing"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

// Request-bound human approval, at the level that decides it.
//
// The host asserts human_approval_granted(ActionID, Action, Target, Payload,
// Digest) for one exact pending_action after a person approved that request
// and the execution claim succeeded. These cases pin that the fact admits
// exactly that tuple and nothing next to it, that consent does not override a
// hard prohibition, and which denials may open a request at all.

type approvalFact struct {
	pred string
	args []any
}

func approvalEngine(t *testing.T, facts ...approvalFact) *mangle.Engine {
	t.Helper()
	eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	for _, f := range []string{"../schemas_safety.mg", "../schemas_execution.mg", "../schemas_shards.mg", "constitution.mg"} {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if err := eng.LoadSchemaString(string(content)); err != nil {
			t.Fatalf("load %s: %v", f, err)
		}
	}
	for _, f := range facts {
		if err := eng.AddFact(f.pred, f.args...); err != nil {
			t.Fatalf("AddFact %s: %v", f.pred, err)
		}
	}
	return eng
}

func permittedTuples(t *testing.T, eng *mangle.Engine) map[[3]string]bool {
	t.Helper()
	facts, err := eng.GetFacts("permitted")
	if err != nil {
		t.Fatal(err)
	}
	out := map[[3]string]bool{}
	for _, f := range facts {
		if len(f.Args) != 3 {
			continue
		}
		a, _ := f.Args[0].(string)
		tg, _ := f.Args[1].(string)
		p, _ := f.Args[2].(string)
		out[[3]string{a, tg, p}] = true
	}
	return out
}

func requiredIDs(t *testing.T, eng *mangle.Engine) map[string]bool {
	t.Helper()
	facts, err := eng.GetFacts("human_approval_required")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, f := range facts {
		if id, ok := f.Args[0].(string); ok {
			out[id] = true
		}
	}
	return out
}

func pending(id, action, target, payload string) approvalFact {
	return approvalFact{"pending_action", []any{id, action, target, payload, 1}}
}

func granted(id, action, target, payload string) approvalFact {
	return approvalFact{"human_approval_granted", []any{id, action, target, payload, "sha256:reviewed"}}
}

func TestHumanApproval_AdmitsExactlyTheApprovedTuple(t *testing.T) {
	defer goleak.VerifyNone(t)
	eng := approvalEngine(t,
		pending("exec-1", "/delete_file", "a.md", `{"path":"a.md"}`),
		pending("exec-2", "/delete_file", "b.md", `{"path":"b.md"}`),
		granted("exec-1", "/delete_file", "a.md", `{"path":"a.md"}`),
	)
	got := permittedTuples(t, eng)
	if !got[[3]string{"/delete_file", "a.md", `{"path":"a.md"}`}] {
		t.Fatal("the approved request was not admitted")
	}
	if got[[3]string{"/delete_file", "b.md", `{"path":"b.md"}`}] {
		t.Fatal("a sibling request of the same action type borrowed the approval")
	}
}

func TestHumanApproval_MismatchedFactAdmitsNothing(t *testing.T) {
	defer goleak.VerifyNone(t)
	for name, g := range map[string]approvalFact{
		"other action id": granted("exec-9", "/delete_file", "a.md", `{"path":"a.md"}`),
		"other target":    granted("exec-1", "/delete_file", "b.md", `{"path":"a.md"}`),
		"other payload":   granted("exec-1", "/delete_file", "a.md", `{"path":"b.md"}`),
	} {
		t.Run(name, func(t *testing.T) {
			eng := approvalEngine(t, pending("exec-1", "/delete_file", "a.md", `{"path":"a.md"}`), g)
			if len(permittedTuples(t, eng)) != 0 {
				t.Fatal("an approval that does not name the pending tuple admitted it")
			}
		})
	}
}

func TestHumanApproval_IsNoOverrideForUnapprovableOrDangerous(t *testing.T) {
	defer goleak.VerifyNone(t)
	t.Run("action type not human_approvable", func(t *testing.T) {
		eng := approvalEngine(t, pending("exec-1", "/git_push", "origin", "{}"), granted("exec-1", "/git_push", "origin", "{}"))
		if len(permittedTuples(t, eng)) != 0 {
			t.Fatal("a person's click admitted an action with no consent arm")
		}
		if len(requiredIDs(t, eng)) != 0 {
			t.Fatal("an unapprovable denial opened an approval request")
		}
	})
	t.Run("secret path", func(t *testing.T) {
		eng := approvalEngine(t,
			pending("exec-1", "/delete_file", ".env", "{}"),
			granted("exec-1", "/delete_file", ".env", "{}"),
			approvalFact{"touches_secret_path", []any{".env"}},
		)
		if len(permittedTuples(t, eng)) != 0 {
			t.Fatal("consent overrode the secret-path prohibition")
		}
		if len(requiredIDs(t, eng)) != 0 {
			t.Fatal("a hard prohibition opened an approval request")
		}
	})
}

func TestHumanApproval_RequiredOnlyForApprovableDenials(t *testing.T) {
	defer goleak.VerifyNone(t)
	eng := approvalEngine(t,
		pending("exec-del", "/delete_file", "untracked.md", "{}"),
		pending("exec-clean", "/delete_file", "tracked.md", "{}"),
		approvalFact{"file_recoverable", []any{"tracked.md"}},
		pending("exec-read", "/read_file", "x.go", "{}"),
	)
	got := requiredIDs(t, eng)
	if !got["exec-del"] {
		t.Error("an approvable denial did not ask for approval")
	}
	if got["exec-clean"] {
		t.Error("a recoverable delete the policy already permits asked for approval")
	}
	if got["exec-read"] {
		t.Error("a permitted read asked for approval")
	}
}
