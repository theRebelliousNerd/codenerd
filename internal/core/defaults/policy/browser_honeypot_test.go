package policy_test

import (
	"context"
	"os"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

// TestHoneypotLogic verifies the honeypot detection rules in browser_honeypot.mg
// using the application's Mangle engine wrapper.
func TestHoneypotLogic(t *testing.T) {
	// 1. Read Rules from Source of Truth
	ruleBytes, err := os.ReadFile("browser_honeypot.mg")
	if err != nil {
		t.Fatalf("Failed to read browser_honeypot.mg: %v", err)
	}
	rules := string(ruleBytes)

	// 2. Prepare Schema and Mode Declarations
	// engine.Query requires mode declarations. Mode "-" means output (list all results).
	// browser_honeypot.mg contains only rules - Decl statements are in schemas_browser.mg
	// We declare all predicates here with modes for isolated testing
	schema := `
	Decl element(ID, Tag, Parent).
	Decl css_property(Elem, Prop, Value).
	Decl computed_style(ID, Prop, Val).
	Decl position(Elem, X, Y, Width, Height).
	Decl attribute(Elem, Name, Value).
	Decl link(Elem, Href).
	Decl honeypot_suspicious_url(Elem).
	Decl honeypot_css_hidden(Elem).
	Decl honeypot_css_invisible(Elem).
	Decl honeypot_opacity_hidden(Elem).
	Decl honeypot_offscreen(Elem).
	Decl honeypot_zero_size(Elem).
	Decl honeypot_aria_hidden(Elem).
	Decl honeypot_no_keyboard(Elem).
	Decl honeypot_pointer_events_none(Elem).
	Decl is_honeypot(Elem) descr [mode("-")].
	Decl high_confidence_honeypot(Elem) descr [mode("-")].
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
				// Explicitly using /atoms to match rule expectations and demonstrate Type Canary compliance.
				// The engine auto-promotes "display" -> /display, but explicit is better for "Testudo".
				{Predicate: "css_property", Args: []interface{}{"/e1", "/display", "/none"}},
			},
			query:    "is_honeypot(X)", // Variable X will bind to /e1
			expected: 1,
		},
		{
			name: "Offscreen X",
			facts: []mangle.Fact{
				{Predicate: "position", Args: []interface{}{"/e2", -2000, 0, 10, 10}},
			},
			query:    "is_honeypot(X)",
			expected: 1,
		},
		{
			name: "Zero Size",
			facts: []mangle.Fact{
				{Predicate: "position", Args: []interface{}{"/e3", 0, 0, 1, 1}},
			},
			query:    "is_honeypot(X)",
			expected: 1,
		},
		{
			name: "High Confidence (Hidden + Zero Size)",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []interface{}{"/e5", "/display", "/none"}},
				{Predicate: "position", Args: []interface{}{"/e5", 0, 0, 1, 1}},
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
