package policy_test

import (
	"context"
	"os"
	"testing"
	"time"

	"codenerd/internal/mangle"

	"go.uber.org/goleak"
)

// TestHoneypotLogic verifies the honeypot detection rules in browser_honeypot.mg
// using the application's Mangle engine wrapper.
func TestHoneypotLogic(t *testing.T) {
	defer goleak.VerifyNone(t)

	// 1. Read Rules from Source of Truth
	ruleBytes, err := os.ReadFile("browser_honeypot.mg")
	if err != nil {
		t.Fatalf("Failed to read browser_honeypot.mg: %v", err)
	}
	rules := string(ruleBytes)

	// 2. Prepare Schema and Mode Declarations
	// engine.Query requires mode declarations. Mode "-" means output (list all results).
	// browser_honeypot.mg contains only rules - Decl statements are in schemas_browser.mg
	// We declare all predicates here with modes for isolated testing.
	//
	// Every bound list below must mirror schemas_browser.mg. Leaving them off is
	// not neutral: Engine.factToAtomLocked only forces ast.String when the Decl
	// says /string, so under an untyped Decl convertValueToTypedTerm's
	// auto-atomizer promotes any identifier-like string - "display", "none",
	// "hidden" - to a name constant. This test then passed against a rule shape
	// (css_property(Elem, /display, /none)) that no live page can produce, while
	// the string-form rules the browser actually feeds went unexercised.
	schema := `
	Decl element(ID, Tag, Parent) bound [/string, /string, /string].
	Decl css_property(Elem, Prop, Value) bound [/string, /string, /string].
	Decl computed_style(ID, Prop, Val) bound [/string, /string, /string].
	Decl position(Elem, X, Y, Width, Height) bound [/string, /number, /number, /number, /number].
	Decl attribute(Elem, Name, Value) bound [/string, /string, /string].
	Decl link(Elem, Href) bound [/string, /string].
	Decl honeypot_suspicious_url(Elem) bound [/string].
	Decl honeypot_css_hidden(Elem) bound [/string].
	Decl honeypot_css_invisible(Elem) bound [/string].
	Decl honeypot_opacity_hidden(Elem) bound [/string].
	Decl honeypot_offscreen(Elem) bound [/string].
	Decl honeypot_zero_size(Elem) bound [/string].
	Decl honeypot_aria_hidden(Elem) bound [/string].
	Decl honeypot_no_keyboard(Elem) bound [/string].
	Decl honeypot_pointer_events_none(Elem) bound [/string].
	Decl honeypot_clip_hidden(Elem) bound [/string].
	Decl honeypot_overflow_hidden(Elem) bound [/string].
	Decl css_clip_rect(Elem, Top, Right, Bottom, Left) bound [/string, /number, /number, /number, /number].
	Decl link_url_pattern(Elem, Pattern) bound [/string, /name].
	Decl honeypot_reason(Elem, Code) descr [mode("-", "-")] bound [/string, /name].
	Decl interactable(ID, ElemType) bound [/string, /name].
	Decl safe_interactable(ID) descr [mode("-")] bound [/string].
	Decl is_honeypot(Elem) descr [mode("-")] bound [/string].
	Decl high_confidence_honeypot(Elem) descr [mode("-")] bound [/string].
	`

	logic := schema + "\n" + rules

	// 3. Table-Driven Cases
	tests := []struct {
		name     string
		facts    []mangle.Fact
		query    string
		expected int
	}{
		{
			name: "CSS Hidden",
			facts: []mangle.Fact{
				// Element ids, CSS property names and CSS values are all /string:
				// schemas_browser.mg declares css_property bound
				// [/string, /string, /string], and internal/browser pushes the raw
				// getComputedStyle map, which is arbitrary page text. Asserting the
				// atom form here would type-check against the untyped Decl above but
				// exercise a rule shape the live browser never produces.
				{Predicate: "css_property", Args: []any{"e1", "display", "none"}},
			},
			query:    "is_honeypot(X)", // Variable X will bind to "e1"
			expected: 1,
		},
		{
			name: "Offscreen X",
			facts: []mangle.Fact{
				{Predicate: "position", Args: []any{"e2", -2000, 0, 10, 10}},
			},
			query:    "is_honeypot(X)",
			expected: 1,
		},
		{
			name: "Zero Size",
			facts: []mangle.Fact{
				{Predicate: "position", Args: []any{"e3", 0, 0, 1, 1}},
			},
			query:    "is_honeypot(X)",
			expected: 1,
		},
		{
			name: "High Confidence (Hidden + Zero Size)",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []any{"e5", "display", "none"}},
				{Predicate: "position", Args: []any{"e5", 0, 0, 1, 1}},
			},
			query:    "high_confidence_honeypot(X)",
			expected: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Isolation: New Engine for each test
			cfg := mangle.DefaultConfig()
			cfg.AutoEval = true

			// Note: NewEngine does not take a context, but we use context in Query.
			eng, err := mangle.NewEngine(cfg, nil)
			if err != nil {
				t.Fatalf("Failed to create engine: %v", err)
			}

			if err := eng.LoadSchemaString(logic); err != nil {
				t.Fatalf("Failed to load logic: %v", err)
			}

			if err := eng.AddFacts(tc.facts); err != nil {
				t.Fatalf("Failed to add facts: %v", err)
			}

			// Context Hygiene: Use context.WithTimeout for the Query
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			res, err := eng.Query(ctx, tc.query)
			if err != nil {
				t.Fatalf("Query failed: %v", err)
			}

			if len(res.Bindings) != tc.expected {
				t.Errorf("Logic Failure: Expected %d results for query %q, got %d. Bindings: %v",
					tc.expected, tc.query, len(res.Bindings), res.Bindings)
			}
		})
	}
}
