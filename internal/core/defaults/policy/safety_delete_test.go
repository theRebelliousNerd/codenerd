package policy

import (
	"os"
	"testing"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

// The delete gate, at the level that decides it.
//
// requires_permission(/delete_file) makes it a dangerous_action, and
// dangerous_action has no safe_action rule, so before this the constitution
// could derive permitted for a delete only through signed_approval plus
// admin_override -- i.e. never, in an unattended run. That is why a dogfood
// run asked to remove 189 forwarding stubs got no further than the first file.
//
// The narrowing: the executor asserts file_recoverable(Target) for a target
// git tracks with nothing staged and nothing modified, and only then. These
// cases pin both halves -- that the fact permits the delete, and that its
// absence still denies it, which is the part that keeps the gate a gate.
func TestSafety_DeleteFile_RecoverableOnly(t *testing.T) {
	defer goleak.VerifyNone(t)

	files := []string{
		"../schemas_safety.mg",
		"../schemas_execution.mg",
		"../schemas_shards.mg",
		"constitution.mg",
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("Required file not found: %s", f)
		}
	}

	const target = "Docs/architecture/context/01-DOMAIN-MODEL.md"

	tests := []struct {
		name         string
		recoverable  bool
		shouldPermit bool
		why          string
	}{
		{
			name:         "git can restore it",
			recoverable:  true,
			shouldPermit: true,
			why:          "the file is tracked and clean, so the deletion is undoable and needs no human",
		},
		{
			name:         "nothing says it can be restored",
			recoverable:  false,
			shouldPermit: false,
			why:          "untracked, modified, outside the workspace or unknown all assert no fact, and must stay denied",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
			if err != nil {
				t.Fatalf("Failed to create engine: %v", err)
			}
			defer eng.Close()

			for _, f := range files {
				content, err := os.ReadFile(f)
				if err != nil {
					t.Fatalf("Failed to read %s: %v", f, err)
				}
				if err := eng.LoadSchemaString(string(content)); err != nil {
					t.Fatalf("Failed to load %s: %v", f, err)
				}
			}

			// pending_action(ActionID, ActionType, Target, Payload, Timestamp)
			if err := eng.AddFact("pending_action", "req-del-1", "/delete_file", target, "{}", 123); err != nil {
				t.Fatalf("AddFact pending_action failed: %v", err)
			}
			if tc.recoverable {
				if err := eng.AddFact("file_recoverable", target); err != nil {
					t.Fatalf("AddFact file_recoverable failed: %v", err)
				}
			}

			facts, err := eng.GetFacts("permitted")
			if err != nil {
				t.Fatalf("GetFacts(permitted): %v", err)
			}

			permitted := false
			for _, f := range facts {
				if len(f.Args) < 3 {
					continue
				}
				action, ok1 := f.Args[0].(string)
				tgt, ok2 := f.Args[1].(string)
				if ok1 && ok2 && (action == "/delete_file" || action == "delete_file") && tgt == target {
					permitted = true
					break
				}
			}

			if tc.shouldPermit && !permitted {
				t.Errorf("delete of %s should be permitted: %s", target, tc.why)
			}
			if !tc.shouldPermit && permitted {
				t.Errorf("delete of %s must NOT be permitted: %s", target, tc.why)
			}
		})
	}
}

// The fact grants a delete and nothing else. If file_recoverable ever widened
// another action, this is where it would show.
func TestSafety_FileRecoverable_GrantsNothingElse(t *testing.T) {
	defer goleak.VerifyNone(t)

	files := []string{
		"../schemas_safety.mg",
		"../schemas_execution.mg",
		"../schemas_shards.mg",
		"constitution.mg",
	}

	const target = "some/path.md"

	for _, action := range []string{"/git_push", "/git_force", "/run_arbitrary_command", "/system_modify"} {
		t.Run(action, func(t *testing.T) {
			eng, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
			if err != nil {
				t.Fatalf("Failed to create engine: %v", err)
			}
			defer eng.Close()

			for _, f := range files {
				content, err := os.ReadFile(f)
				if err != nil {
					t.Fatalf("Failed to read %s: %v", f, err)
				}
				if err := eng.LoadSchemaString(string(content)); err != nil {
					t.Fatalf("Failed to load %s: %v", f, err)
				}
			}

			if err := eng.AddFact("pending_action", "req-1", action, target, "{}", 123); err != nil {
				t.Fatalf("AddFact pending_action failed: %v", err)
			}
			// The fact is present, as it would be if the same target were also
			// the subject of a delete in the same turn.
			if err := eng.AddFact("file_recoverable", target); err != nil {
				t.Fatalf("AddFact file_recoverable failed: %v", err)
			}

			facts, err := eng.GetFacts("permitted")
			if err != nil {
				t.Fatalf("GetFacts(permitted): %v", err)
			}
			for _, f := range facts {
				if len(f.Args) < 3 {
					continue
				}
				if a, ok := f.Args[0].(string); ok && (a == action || a == action[1:]) {
					t.Errorf("%s was permitted by the presence of file_recoverable; the rule must be scoped to /delete_file", action)
				}
			}
		})
	}
}
