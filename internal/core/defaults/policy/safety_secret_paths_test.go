package policy

import (
	"fmt"
	"os"
	"testing"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

// The secret-path gate, at the level that decides it.
//
// read_file is a safe_action, so before this the constitution derived
// permitted(/read_file, ".env", ...) as readily as for any source file, and
// the file's contents went to the model's provider. The executor now asserts
// touches_secret_path(Target) for a target matching execution.secret_paths;
// these cases pin that the fact denies every action that reaches permitted --
// the safe ones and the recoverable delete alike -- and that its absence
// changes nothing.
func TestSafety_SecretPath_DeniesEveryActionOnIt(t *testing.T) {
	defer goleak.VerifyNone(t)

	files := []string{
		"../schemas_safety.mg",
		"../schemas_execution.mg",
		"../schemas_shards.mg",
		"constitution.mg",
	}

	const target = ".env"

	cases := []struct {
		name         string
		action       string
		secret       bool
		recoverable  bool
		shouldPermit bool
	}{
		{"read of an ordinary file", "/read_file", false, false, true},
		{"read of a secret", "/read_file", true, false, false},
		{"write to a secret", "/write_file", true, false, false},
		{"grep aimed at a secret", "/grep", true, false, false},
		{"shell command naming a secret", "/run_command", true, false, false},
		// A recoverable delete is otherwise permitted; a secret is not deleted
		// on the model's say-so either.
		{"recoverable delete of a secret", "/delete_file", true, true, false},
		{"recoverable delete of an ordinary file", "/delete_file", false, true, true},
	}

	for _, tc := range cases {
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

			// The measured fact goes in before pending_action. mangle.Engine's
			// AddFacts evaluates on top of what it already derived, so a
			// permitted derived before touches_secret_path arrived would
			// survive it (E13, 2026-09-22). The production kernel re-derives
			// the negative dependents and denies in either order; that order is
			// pinned against it in internal/session.
			if tc.secret {
				if err := eng.AddFact("touches_secret_path", target); err != nil {
					t.Fatalf("AddFact touches_secret_path failed: %v", err)
				}
			}
			if err := eng.AddFact("pending_action", "req-1", tc.action, target, "{}", 123); err != nil {
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
				a, ok1 := f.Args[0].(string)
				tgt, ok2 := f.Args[1].(string)
				if ok1 && ok2 && (a == tc.action || a == tc.action[1:]) && tgt == target {
					permitted = true
				}
			}
			if permitted != tc.shouldPermit {
				dangerous, _ := eng.GetFacts("dangerous_content")
				var seen []string
				for _, f := range dangerous {
					seen = append(seen, fmt.Sprintf("%v(%T)", f.Args, f.Args[0]))
				}
				t.Errorf("%s on %s: permitted=%v, want %v; dangerous_content derived: %v", tc.action, target, permitted, tc.shouldPermit, seen)
			}
		})
	}
}
