package browser

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/mangle"
)

// The DOM policy rules read css_property, attribute and dom_node - all declared
// bound [/string, ...] because their values are whatever the live page contains.
// schema_contract_test.go already proves buildDOMFacts *types* conform; this proves
// the rules actually fire on that output.
//
// Both halves matter and both were broken. policy/browser.mg matched the tag and
// attributes as atoms (/input, /type, /checkbox), which can never unify with a
// /string fact; and dom_node carried el.tagName verbatim, which the DOM reports
// upper-cased, so even the corrected "input" literal would have missed. Asserting
// hand-written fixtures instead of buildDOMFacts output is what hid that for so
// long, so this test starts from the producer.
func TestDOMPolicy_WhenFedRealSnapshotFacts_ShouldDeriveHoneypotAndTargetCheckbox(t *testing.T) {
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	root := repoRoot(t)
	for _, p := range []string{
		"internal/core/defaults/schemas_browser.mg",
		"internal/core/defaults/policy/browser_honeypot.mg",
		"internal/core/defaults/policy/browser.mg",
	} {
		if err := engine.LoadSchema(filepath.Join(root, p)); err != nil {
			t.Fatalf("load %s: %v", p, err)
		}
	}

	// A checkbox with a label to its right, plus a hidden trap carrying every
	// piece of evidence the honeypot rules know how to read.
	check := domSnapshotNode{
		ID: "agree", Tag: "INPUT", Parent: "root",
		Attrs:  map[string]string{"type": "checkbox"},
		Styles: map[string]string{"display": "block"},
	}
	check.Layout.X, check.Layout.Y, check.Layout.Width, check.Layout.Height = 10, 20, 15, 15
	label := domSnapshotNode{
		ID: "terms", Tag: "BUTTON", Text: "I agree to the terms", Parent: "root",
		Attrs:  map[string]string{},
		Styles: map[string]string{"display": "block"},
	}
	label.Layout.X, label.Layout.Y, label.Layout.Width, label.Layout.Height = 200, 20, 80, 15
	trap := domSnapshotNode{
		ID: "trap", Tag: "A", Parent: "root",
		Attrs:  map[string]string{"tabindex": "-1", "aria-hidden": "true"},
		Styles: map[string]string{"display": "none", "visibility": "hidden", "opacity": "0", "pointerEvents": "none"},
	}
	trap.Layout.X, trap.Layout.Y, trap.Layout.Width, trap.Layout.Height = 0, 0, 1, 1

	manager := newSessionManager(DefaultConfig(), nil)
	facts := manager.buildDOMFacts("s1", []domSnapshotNode{check, label, trap}, time.Now())
	if err := engine.AddFacts(facts); err != nil {
		t.Fatalf("engine rejected a buildDOMFacts fact: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tc := range []struct {
		query   string
		wantMin int
	}{
		{"is_honeypot(X)", 1},
		// One row per evidence code the trap carries: css_hidden, css_invisible,
		// opacity_hidden, zero_size, aria_hidden, no_keyboard, pointer_events_none.
		{"honeypot_reason(X, R)", 7},
		{"target_checkbox(C, L)", 1},
		{"safe_interactable(X)", 1},
	} {
		res, qerr := engine.Query(ctx, tc.query)
		if qerr != nil {
			t.Fatalf("Query(%s): %v", tc.query, qerr)
		}
		if len(res.Bindings) < tc.wantMin {
			t.Errorf("Query(%s) returned %d rows, want >= %d: %v",
				tc.query, len(res.Bindings), tc.wantMin, res.Bindings)
		}
	}
}
