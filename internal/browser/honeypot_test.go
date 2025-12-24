package browser

import (
	"path/filepath"
	"runtime"
	"testing"

	"codenerd/internal/mangle"
)

// getProjectRoot returns the absolute path to the project root.
func getProjectRoot(t *testing.T) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	// filename is .../internal/browser/honeypot_test.go
	// root is .../
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func TestHoneypotDetection(t *testing.T) {
	// Setup engine with honeypot rules
	cfg := mangle.DefaultConfig()
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Load schemas and rules from files
	root := getProjectRoot(t)
	schemaPath := filepath.Join(root, "internal/core/defaults/schemas_browser.mg")
	policyPath := filepath.Join(root, "internal/core/defaults/policy/browser_honeypot.mg")

	if err := engine.LoadSchema(schemaPath); err != nil {
		t.Fatalf("Failed to load schema from %s: %v", schemaPath, err)
	}
	if err := engine.LoadSchema(policyPath); err != nil {
		t.Fatalf("Failed to load policy from %s: %v", policyPath, err)
	}

	detector := NewHoneypotDetector(engine)

	tests := []struct {
		name     string
		facts    []mangle.Fact
		elemID   string
		expected bool
		reasons  []string
	}{
		{
			name: "Display None",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []interface{}{"elem1", "display", "none"}},
			},
			elemID:   "elem1",
			expected: true,
			reasons:  []string{"Hidden via display:none"},
		},
		{
			name: "Visibility Hidden",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []interface{}{"elem2", "visibility", "hidden"}},
			},
			elemID:   "elem2",
			expected: true,
			reasons:  []string{"Hidden via visibility:hidden"},
		},
		{
			name: "Offscreen",
			facts: []mangle.Fact{
				{Predicate: "position", Args: []interface{}{"elem3", int64(-9999), int64(0), int64(100), int64(100)}},
			},
			elemID:   "elem3",
			expected: true,
			reasons:  []string{"Positioned off-screen"},
		},
		{
			name: "Zero Size",
			facts: []mangle.Fact{
				{Predicate: "position", Args: []interface{}{"elem4", int64(0), int64(0), int64(0), int64(0)}},
			},
			elemID:   "elem4",
			expected: true,
			reasons:  []string{"Zero or near-zero size"},
		},
		{
			name: "Suspicious URL",
			facts: []mangle.Fact{
				{Predicate: "honeypot_suspicious_url", Args: []interface{}{"elem5"}},
			},
			elemID:   "elem5",
			expected: true,
			reasons:  []string{"Suspicious URL pattern"},
		},
		{
			name: "Normal Element",
			facts: []mangle.Fact{
				{Predicate: "css_property", Args: []interface{}{"elem6", "display", "block"}},
				{Predicate: "position", Args: []interface{}{"elem6", int64(100), int64(100), int64(50), int64(20)}},
			},
			elemID:   "elem6",
			expected: false,
			reasons:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear facts for this test
			// Note: In a real scenario we might use separate engines or scoped facts
			// For this test we just add facts and rely on unique IDs
			if err := engine.AddFacts(tc.facts); err != nil {
				t.Fatalf("Failed to add facts: %v", err)
			}

			// Force recompute
			if err := engine.RecomputeRules(); err != nil {
				t.Fatalf("Failed to recompute rules: %v", err)
			}

			// Check results
			reasons := detector.getHoneypotReasons(tc.elemID)
			isHoneypot := len(reasons) > 0

			if isHoneypot != tc.expected {
				t.Errorf("Expected isHoneypot=%v, got %v", tc.expected, isHoneypot)
			}

			if tc.expected {
				if len(reasons) != len(tc.reasons) {
					t.Errorf("Expected %d reasons, got %d", len(tc.reasons), len(reasons))
				}
				// Simple check for presence of expected reason string
				for _, expectedReason := range tc.reasons {
					found := false
					for _, r := range reasons {
						if r == expectedReason {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected reason %q not found in %v", expectedReason, reasons)
					}
				}
			}
		})
	}
}
