package autopoiesis

import (
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

// TestSanitizeThunderdomeCategory validates the atom sanitiser.
// Spec: lowercase, replace outside [a-z0-9_] with underscore, fallback to unknown when empty.
func TestSanitizeThunderdomeCategory(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, got string)
	}{
		{
			name:  "nil pointer with space and bang sanitises to valid atom",
			input: "nil pointer!",
			check: func(t *testing.T, got string) {
				if !strings.HasPrefix(got, "/") {
					t.Fatalf("expected leading slash, got %q", got)
				}
				if strings.Contains(got, " ") || strings.Contains(got, "!") {
					t.Fatalf("sanitised atom still contains invalid char: %q", got)
				}
				// Must be all lower and only allowed chars after slash
				trim := strings.TrimPrefix(got, "/")
				for _, r := range trim {
					if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
						t.Fatalf("invalid char %q in sanitised %q", r, got)
					}
				}
				// lowercasing check
				if got != strings.ToLower(got) {
					t.Fatalf("expected lowercased, got %q", got)
				}
				// Specific expected from spec logic: "nil pointer!" -> "nil_pointer_"
				want := "/nil_pointer_"
				if got != want {
					t.Fatalf("got %q want %q", got, want)
				}
			},
		},
		{
			name:  "empty category falls back to unknown atom not empty",
			input: "",
			check: func(t *testing.T, got string) {
				if got == "" || got == "/" {
					t.Fatalf("empty category must not produce empty atom, got %q", got)
				}
				if got != "/unknown" {
					t.Fatalf("empty fallback expected /unknown got %q", got)
				}
			},
		},
		{
			name:  "uppercase is lowercased",
			input: "Boundary",
			check: func(t *testing.T, got string) {
				if got != "/boundary" {
					t.Fatalf("got %q want /boundary", got)
				}
			},
		},
		{
			name:  "already valid category preserved",
			input: "nil_pointer",
			check: func(t *testing.T, got string) {
				if got != "/nil_pointer" {
					t.Fatalf("got %q want /nil_pointer", got)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeThunderdomeCategory(tc.input)
			tc.check(t, got)
		})
	}
}

func TestThunderdomeOutcomeAtom(t *testing.T) {
	if got := thunderdomeOutcomeAtom(true); got != "/survived" {
		t.Fatalf("survived expected /survived got %q", got)
	}
	if got := thunderdomeOutcomeAtom(false); got != "/failed" {
		t.Fatalf("failed expected /failed got %q", got)
	}
	// Ensure they are atoms (leading slash, no quotes)
	for _, v := range []string{thunderdomeOutcomeAtom(true), thunderdomeOutcomeAtom(false)} {
		if !strings.HasPrefix(v, "/") {
			t.Fatalf("outcome atom must start with slash, got %q", v)
		}
		if strings.Contains(v, "\"") {
			t.Fatalf("outcome atom must not contain quotes, got %q", v)
		}
	}
}

// TestBuildThunderdomeResultFacts asserts fact construction for two attacks.
func TestBuildThunderdomeResultFacts(t *testing.T) {
	br := &BattleResult{
		ToolName: "my_tool",
		Results: []AttackResult{
			{Vector: AttackVector{Name: "a1", Category: "nil_pointer"}, Survived: true},
			{Vector: AttackVector{Name: "a2", Category: "boundary"}, Survived: false},
		},
	}
	facts := buildThunderdomeResultFacts(br)
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(facts))
	}
	for i, f := range facts {
		if f.Predicate != "thunderdome_result" {
			t.Errorf("fact %d predicate = %q, want thunderdome_result", i, f.Predicate)
		}
		if len(f.Args) != 3 {
			t.Fatalf("fact %d args len = %d, want 3", i, len(f.Args))
		}
		if f.Args[0] != "my_tool" {
			t.Errorf("fact %d arg1 = %v, want my_tool", i, f.Args[0])
		}
		// arg2 and arg3 must be atoms: string with leading slash, no quotes
		for j, arg := range f.Args[1:] {
			s, ok := arg.(string)
			if !ok {
				t.Fatalf("fact %d arg%d not string: %T %v", i, j+1, arg, arg)
			}
			if !strings.HasPrefix(s, "/") {
				t.Errorf("fact %d arg%d = %q, want leading slash (atom)", i, j+1, s)
			}
			if strings.Contains(s, "\"") {
				t.Errorf("fact %d arg%d contains quotes: %q", i, j+1, s)
			}
		}
		// Concrete values
		if facts[0].Args[1] != "/nil_pointer" {
			t.Errorf("fact 0 attack type = %q, want /nil_pointer", facts[0].Args[1])
		}
		if facts[0].Args[2] != "/survived" {
			t.Errorf("fact 0 outcome = %q, want /survived", facts[0].Args[2])
		}
		if facts[1].Args[1] != "/boundary" {
			t.Errorf("fact 1 attack type = %q, want /boundary", facts[1].Args[1])
		}
		if facts[1].Args[2] != "/failed" {
			t.Errorf("fact 1 outcome = %q, want /failed", facts[1].Args[2])
		}
		// Check Fact.String() representation: atoms are not quoted, tool name is quoted
		str := facts[i].String()
		// tool name should be quoted
		if !strings.Contains(str, "\"my_tool\"") {
			t.Errorf("fact %d String() %q should contain quoted tool name", i, str)
		}
		// atoms should appear without surrounding quotes: e.g., ", /nil_pointer," not ", \"/nil_pointer\","
		if strings.Contains(str, "\"/") {
			t.Errorf("fact %d String() %q contains quoted atom (should be bare)", i, str)
		}
	}
}

// TestBuildThunderdomeResultFacts_SanitisesCategory ensures sanitisation is applied in fact building.
func TestBuildThunderdomeResultFacts_SanitisesCategory(t *testing.T) {
	br := &BattleResult{
		ToolName: "tool_x",
		Results: []AttackResult{
			{Vector: AttackVector{Category: "nil pointer!"}, Survived: true},
		},
	}
	facts := buildThunderdomeResultFacts(br)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	got := facts[0].Args[1].(string)
	want := "/nil_pointer_"
	if got != want {
		t.Fatalf("sanitised category = %q want %q", got, want)
	}
}

// TestBuildThunderdomeResultFacts_EmptyCategoryFallback ensures empty category yields /unknown not empty atom.
func TestBuildThunderdomeResultFacts_EmptyCategoryFallback(t *testing.T) {
	br := &BattleResult{
		ToolName: "tool_y",
		Results: []AttackResult{
			{Vector: AttackVector{Category: ""}, Survived: false},
		},
	}
	facts := buildThunderdomeResultFacts(br)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	got := facts[0].Args[1].(string)
	if got == "" || got == "/" {
		t.Fatalf("empty category produced empty atom %q", got)
	}
	if got != "/unknown" {
		t.Fatalf("empty category fallback = %q want /unknown", got)
	}
}

// TestBuildThunderdomeResultFacts_AtomValidity checks that Fact.String places atoms unquoted and tool name quoted.
func TestBuildThunderdomeResultFacts_AtomValidity(t *testing.T) {
	br := &BattleResult{
		ToolName: "demo",
		Results: []AttackResult{
			{Vector: AttackVector{Category: "resource"}, Survived: true},
		},
	}
	facts := buildThunderdomeResultFacts(br)
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact")
	}
	f := facts[0]
	// Use mangle Fact.String logic: string with "/" is atom, else quoted
	s := f.String()
	// Should be thunderdome_result("demo", /resource, /survived).
	if !strings.Contains(s, "thunderdome_result") {
		t.Fatalf("String missing predicate: %q", s)
	}
	if s != `thunderdome_result("demo", /resource, /survived).` {
		t.Fatalf("unexpected String: got %q want %q", s, `thunderdome_result("demo", /resource, /survived).`)
	}
}

// TestBuildThunderdomeResultFacts_NilAndEmpty ensures nil/empty results produce no facts.
func TestBuildThunderdomeResultFacts_NilAndEmpty(t *testing.T) {
	if facts := buildThunderdomeResultFacts(nil); facts != nil {
		t.Fatalf("nil BattleResult should produce nil, got %v", facts)
	}
	br := &BattleResult{ToolName: "x", Results: []AttackResult{}}
	if facts := buildThunderdomeResultFacts(br); len(facts) != 0 {
		t.Fatalf("empty Results should produce nil/empty, got %d", len(facts))
	}
}

// Ensure the fact predicate matches schema Decl exactly.
func TestThunderdomeResultDeclMatches(t *testing.T) {
	// Load schema and verify thunderdome_result can be asserted without error.
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = false
	eng, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	schema := `Decl thunderdome_result(ToolName, AttackType, Outcome) bound [/string, /name, /name].`
	if err := eng.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchema: %v", err)
	}
	br := &BattleResult{
		ToolName: "tool_z",
		Results: []AttackResult{
			{Vector: AttackVector{Category: "format"}, Survived: false},
		},
	}
	facts := buildThunderdomeResultFacts(br)
	if err := eng.AddFacts(facts); err != nil {
		t.Fatalf("AddFacts failed (likely malformed atom): %v", err)
	}
	// Query back via GetFacts helper if available; otherwise just ensure no error.
}
