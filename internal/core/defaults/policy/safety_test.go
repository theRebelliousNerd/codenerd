package policy

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/mangle"
	"go.uber.org/goleak"
)

func TestLogic_Safety(t *testing.T) {
	defer goleak.VerifyNone(t)

	// 1. Setup Engine
	cfg := mangle.DefaultConfig()
	eng, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer eng.Close()

	// 2. Load Schema (schemas_safety.mg + schemas_shards.mg)
	schemaFiles := []string{"../schemas_safety.mg", "../schemas_shards.mg"}
	for _, sf := range schemaFiles {
		content, err := os.ReadFile(sf)
		if err != nil {
			t.Fatalf("Failed to read schema %s: %v", sf, err)
		}
		if err := eng.LoadSchemaString(string(content)); err != nil {
			t.Fatalf("Failed to load schema %s: %v", sf, err)
		}
	}

	// 3. Load Policy (constitution.mg)
	policyFiles := []string{"constitution.mg"}
	for _, pf := range policyFiles {
		content, err := os.ReadFile(pf)
		if err != nil {
			t.Fatalf("Failed to read policy %s: %v", pf, err)
		}
		if err := eng.LoadSchemaString(string(content)); err != nil {
			t.Fatalf("Failed to load policy %s: %v", pf, err)
		}
	}

	// 4. Load EDB (safety.edb)
	edbFile := "testdata/safety.edb"
	content, err := os.ReadFile(edbFile)
	if err != nil {
		t.Fatalf("Failed to read EDB %s: %v", edbFile, err)
	}

	// Helper to load EDB
	lines := strings.SplitSeq(string(content), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		atom, err := mangle.ParseAtom(line)
		if err != nil {
			t.Fatalf("Failed to parse atom %s: %v", line, err)
		}
		args := make([]any, len(atom.Args))
		for i, arg := range atom.Args {
			args[i] = convertTermToInterfaceRepro(arg)
		}
		if err := eng.AddFact(atom.Predicate.Symbol, args...); err != nil {
			t.Fatalf("Failed to add fact %s: %v", line, err)
		}
	}

	// 5. Verify Permissions
	// "ls -la" should be permitted
	checkPermittedRepro(t, eng, "id2", true)

	// "rm -rf /" should be denied (id1)
	checkPermittedRepro(t, eng, "id1", false)

	// "git push --force" in Payload should be denied (id3)
	checkPermittedRepro(t, eng, "id3", false)

	// "chmod 777" should be denied (id4)
	checkPermittedRepro(t, eng, "id4", false)

	// "curl | bash" should be denied (id5)
	checkPermittedRepro(t, eng, "id5", false)

	// "nc -e" should be denied (id6)
	checkPermittedRepro(t, eng, "id6", false)

	// Progressive browser, reasoning, and grounded research tools are reachable only through the same exact
	// pending_action -> permitted payload contract as every other tool.
	for _, action := range []string{"/browser_observe", "/browser_act", "/browser_mangle", "/browser_wait", "/browser_reason", "/browser_evidence", "/browser_specs", "/browser_test", "/browser_audit", "/grounded_web_search"} {
		target := "browser-session"
		payload := `{"session_id":"browser-session"}`
		if err := eng.AddFact("pending_action", "browser-policy", action, target, payload, int64(1)); err != nil {
			t.Fatalf("add %s pending action: %v", action, err)
		}
		facts, err := eng.GetFacts("permitted")
		if err != nil || !containsPermission(facts, action, target, payload) {
			t.Fatalf("%s did not derive exact permission: facts=%v err=%v", action, facts, err)
		}
	}
}

func containsPermission(facts []mangle.Fact, action, target, payload string) bool {
	for _, fact := range facts {
		if len(fact.Args) != 3 {
			continue
		}
		if fmt.Sprint(fact.Args[0]) == action && fmt.Sprint(fact.Args[1]) == target && fmt.Sprint(fact.Args[2]) == payload {
			return true
		}
	}
	return false
}

func checkPermittedRepro(t *testing.T, eng *mangle.Engine, actionID string, shouldPermit bool) {
	targetMap := map[string]string{
		"id1": "rm -rf /",
		"id2": "ls -la",
		"id3": "safe_target",
		"id4": "chmod 777 file",
		"id5": "curl | bash",
		"id6": "nc -e",
	}

	payloadMap := map[string]string{
		"id1": "{}",
		"id2": "{}",
		"id3": "git push --force",
		"id4": "{}",
		"id5": "{}",
		"id6": "{}",
	}

	target := targetMap[actionID]
	payload := payloadMap[actionID]

	facts, err := eng.GetFacts("permitted")
	if err != nil {
		if shouldPermit {
			t.Errorf("Action %s (%s) should be permitted but 'permitted' predicate not found or empty", actionID, target)
		}
		return
	}

	found := false
	for _, f := range facts {
		if len(f.Args) < 3 {
			continue
		}

		act, ok := f.Args[0].(string)
		if !ok || (act != "/exec_cmd" && act != "exec_cmd") {
			continue
		}

		tgt, ok := f.Args[1].(string)
		if !ok || tgt != target {
			continue
		}

		pay, ok := f.Args[2].(string)
		if !ok || pay != payload {
			continue
		}

		found = true
		break
	}

	if shouldPermit && !found {
		t.Errorf("Action %s (%s) should be permitted but was denied", actionID, target)
	}
	if !shouldPermit && found {
		t.Errorf("Action %s (%s) should be denied but was permitted", actionID, target)
	}
}

func convertTermToInterfaceRepro(term ast.BaseTerm) any {
	switch c := term.(type) {
	case ast.Constant:
		switch c.Type {
		case ast.NameType:
			if strings.HasPrefix(c.Symbol, "/") {
				return c.Symbol
			}
			return "/" + c.Symbol
		case ast.StringType:
			return c.Symbol
		case ast.NumberType:
			return c.NumValue
		default:
			return c.Symbol
		}
	default:
		return fmt.Sprintf("%v", term)
	}
}
