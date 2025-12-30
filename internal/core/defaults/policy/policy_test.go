package policy_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/mangle/analysis"
	"github.com/google/mangle/ast"
	"github.com/google/mangle/engine"
	"github.com/google/mangle/factstore"
	"github.com/google/mangle/parse"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestGitSafety_Golden(t *testing.T) {
	// 1. Load Logic
	rulesFiles := []string{
		"../schemas_intent.mg",
		"../schemas_safety.mg",
		"git_safety.mg",
	}

	var sb strings.Builder
	for _, f := range rulesFiles {
		content, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("failed to read file %s: %v", f, err)
		}
		sb.Write(content)
		sb.WriteByte('\n')
	}

	// 2. Parse & Analyze
	unit, err := parse.Unit(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("Syntax Error: %v", err)
	}

	program, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		t.Fatalf("Logic Error (Unsafe/Unstratified): %v", err)
	}

	// 3. Setup Facts (EDB)
	store := factstore.NewSimpleInMemoryStore()

	// Helper to add fact via parsing
	addFact := func(factStr string) {
		atom, err := parse.Atom(factStr)
		if err != nil {
			t.Fatalf("failed to parse fact '%s': %v", factStr, err)
		}
		store.Add(atom)
	}

	// Facts
	addFact(`git_history("/file1", "hash1", "bob", 1, "msg")`)
	addFact(`current_user("alice")`)
	addFact(`user_intent(/current_intent, /mutation, /delete, "/file1", "none")`)

	addFact(`churn_rate("/file2", 10)`)
	addFact(`user_intent(/current_intent, /mutation, /refactor, "/file2", "none")`)


	// 4. Execution
	// Verify cancellation context is setup even if EvalProgram doesn't strictly take it (Testudo principle)
	// But since the API doesn't take it, we just defer cancel.
	_, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

    // Pass context if EvalProgram supports it, otherwise just run it.
    // engine.EvalProgram(program, store) seems to be the signature from grep
	if err := engine.EvalProgram(program, store); err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// 5. Verification vs Golden File
	expectedFacts, err := os.ReadFile("testdata/git_safety_golden.txt")
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(expectedFacts)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		expectedAtom, err := parse.Atom(line)
		if err != nil {
			t.Errorf("invalid golden file line '%s': %v", line, err)
			continue
		}

		found := false
		err = store.GetFacts(expectedAtom, func(c ast.Atom) error {
            if atomsMatch(c, expectedAtom) {
			    found = true
            }
			return nil
		})
        if err != nil {
            t.Fatalf("Error reading store: %v", err)
        }

		if !found {
			t.Errorf("Missing expected fact: %s", line)
		}
	}
}

// atomsMatch checks if two ground atoms are equal
func atomsMatch(a, b ast.Atom) bool {
    if a.Predicate.Symbol != b.Predicate.Symbol {
        return false
    }
    if len(a.Args) != len(b.Args) {
        return false
    }
    for i := range a.Args {
        c1, ok1 := a.Args[i].(ast.Constant)
        c2, ok2 := b.Args[i].(ast.Constant)
        if !ok1 || !ok2 {
            return false
        }
        if c1.Type != c2.Type {
            return false
        }
        if c1.Symbol != c2.Symbol {
            return false
        }
        if c1.NumValue != c2.NumValue {
            return false
        }
    }
    return true
}
