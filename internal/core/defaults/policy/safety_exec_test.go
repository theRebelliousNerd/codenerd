package policy

import (
	"os"
	"testing"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

func TestSafety_ExecCmd_Blocking(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfg := mangle.DefaultConfig()

	// Load Schemas and Policy
	// We need schemas_safety.mg, schemas_execution.mg, schemas_shards.mg (for pending_action), and constitution.mg
	files := []string{
		"../schemas_safety.mg",
		"../schemas_execution.mg",
		"../schemas_shards.mg",
		"constitution.mg",
	}

	// Verify files exist first
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("Required file not found: %s", f)
		}
	}

	tests := []struct {
		name         string
		payload      string
		shouldPermit bool
	}{
		{
			name:         "Safe Command",
			payload:      "ls -la",
			shouldPermit: true,
		},
		{
			name:         "Dangerous Command Force Push",
			payload:      "git push --force origin main",
			shouldPermit: false,
		},
		{
			name:         "Dangerous Command Short Flag",
			payload:      "git push -f origin main",
			shouldPermit: false,
		},
		{
			name:         "Dangerous Command With Options",
			payload:      "git push origin main --force",
			shouldPermit: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eng, err := mangle.NewEngine(cfg, nil)
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

			// Add pending_action fact
			// pending_action(ActionID, ActionType, Target, Payload, Timestamp)
			// Note: ActionType must be Atom (/exec_cmd).
			if err := eng.AddFact("pending_action", "req-1", "/exec_cmd", "", tc.payload, 123); err != nil {
				t.Fatalf("AddFact pending_action failed: %v", err)
			}

			// Add safe_action fact if it's not derived (it IS derived in constitution.mg as a base fact)
			// But wait, safe_action(/exec_cmd) is a fact in constitution.mg.
			// So we don't need to add it.

			// Query permitted
			facts, err := eng.GetFacts("permitted")
			if err != nil {
				// If not found, it's empty, so not permitted.
				// But we expect it to be declared.
				// If GetFacts returns error for missing predicate, that's different.
				// But permitted IS declared in schemas_safety.mg.
			}

			permitted := false
			for _, f := range facts {
				// permitted(ActionType, Target, Payload)
				// Check if it matches our action
				if len(f.Args) >= 3 {
					// Args[0] is ActionType (/exec_cmd)
					// Args[2] is Payload
					actionType, ok1 := f.Args[0].(string)
					payload, ok2 := f.Args[2].(string)

					// Normalize actionType (might have leading /)
					if ok1 && (actionType == "/exec_cmd" || actionType == "exec_cmd") && ok2 && payload == tc.payload {
						permitted = true
						break
					}
				}
			}

			if tc.shouldPermit && !permitted {
				t.Errorf("Expected command '%s' to be PERMITTED, but it was blocked.", tc.payload)
			}
			if !tc.shouldPermit && permitted {
				t.Errorf("Expected command '%s' to be BLOCKED, but it was permitted.", tc.payload)
			}
		})
	}
}
