package browser

import (
	"context"
	"strings"
	"testing"
)

func TestExecuteActions_StopOnErrorAndBoundOperations(t *testing.T) {
	manager := NewSessionManager(DefaultConfig(), nil)
	execution, err := manager.ExecuteActions(context.Background(), "", []ActionOperation{
		{Type: "not-a-real-operation"},
		{Type: "sleep", DurationMS: 0},
	}, true)
	if err != nil {
		t.Fatalf("ExecuteActions returned transport error: %v", err)
	}
	if execution.Success || len(execution.Results) != 1 || execution.Counts["failed"] != 1 {
		t.Fatalf("stop-on-error contract failed: %+v", execution)
	}

	tooMany := make([]ActionOperation, maxActionOperations+1)
	for i := range tooMany {
		tooMany[i] = ActionOperation{Type: "sleep"}
	}
	if _, err := manager.ExecuteActions(context.Background(), "", tooMany, true); err == nil || !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected operation bound error, got %v", err)
	}
}

func TestNormalizeObserveOptions_DefaultsAndBounds(t *testing.T) {
	mode, view, maxItems, filter, err := normalizeObserveOptions(ObserveOptions{MaxItems: maxObservationItems + 50})
	if err != nil {
		t.Fatalf("normalizeObserveOptions failed: %v", err)
	}
	if mode != "composite" || view != "compact" || maxItems != maxObservationItems || filter != "all" {
		t.Fatalf("unexpected normalized options: mode=%q view=%q max=%d filter=%q", mode, view, maxItems, filter)
	}
	if _, _, _, _, err := normalizeObserveOptions(ObserveOptions{Mode: "javascript"}); err == nil {
		t.Fatal("arbitrary observation mode should fail closed")
	}
}

func TestFirstNonEmpty_SkipsBlanksAndTrims(t *testing.T) {
	cases := map[string]struct {
		in   []string
		want string
	}{
		"empty returns empty":          {in: nil, want: ""},
		"all blank returns empty":      {in: []string{"", "   ", "\t"}, want: ""},
		"first non-empty wins":         {in: []string{"", "  keep  ", "other"}, want: "keep"},
		"trims surrounding whitespace": {in: []string{"  padded  "}, want: "padded"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := firstNonEmpty(tc.in...); got != tc.want {
				t.Fatalf("firstNonEmpty(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRequireSession_RejectsBlank(t *testing.T) {
	if err := requireSession("   "); err == nil || !strings.Contains(err.Error(), "session_id is required") {
		t.Fatalf("expected session_id required error, got %v", err)
	}
	if err := requireSession("sess-1"); err != nil {
		t.Fatalf("expected nil for valid session, got %v", err)
	}
}

func TestNamedKey_MapsAliasesAndSingleRune(t *testing.T) {
	cases := map[string]struct {
		in      string
		wantOK  bool
		wantKey string
	}{
		"enter alias":        {in: "Enter", wantOK: true},
		"return alias":       {in: "RETURN", wantOK: true},
		"escape alias":       {in: "esc", wantOK: true},
		"arrow alias":        {in: "left", wantOK: true},
		"single rune":        {in: "a", wantOK: true, wantKey: "a"},
		"empty rejects":      {in: "", wantOK: false},
		"unknown rejects":    {in: "frobnicator", wantOK: false},
		"multi-rune rejects": {in: "ab", wantOK: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			key, ok := namedKey(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("namedKey(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			if tc.wantKey != "" && string(key) != tc.wantKey {
				t.Fatalf("namedKey(%q) = %q, want %q", tc.in, string(key), tc.wantKey)
			}
		})
	}
}

func TestModifierKey_MapsKnownModifiers(t *testing.T) {
	for _, raw := range []string{"control", "CTRL", "shift", "alt", "option", "meta", "command", "cmd"} {
		t.Run(raw, func(t *testing.T) {
			if _, ok := modifierKey(raw); !ok {
				t.Fatalf("modifierKey(%q) should be accepted", raw)
			}
		})
	}
	if _, ok := modifierKey("super"); ok {
		t.Fatal("modifierKey(super) should be rejected")
	}
	if _, ok := modifierKey(""); ok {
		t.Fatal("modifierKey(empty) should be rejected")
	}
}

func TestMatcherEnvironmentName_NormalizesIdentity(t *testing.T) {
	got := matcherEnvironmentName(ElementMatcher{DataTestID: "user-email!"})
	if got != "CODENERD_BROWSER_TEST_USER_EMAIL" {
		t.Fatalf("unexpected env name: %q", got)
	}
	got = matcherEnvironmentName(ElementMatcher{})
	if got != "CODENERD_BROWSER_TEST_SECRET" {
		t.Fatalf("empty matcher should fall back to SECRET, got %q", got)
	}
	got = matcherEnvironmentName(ElementMatcher{ID: "  a/b c "})
	if got != "CODENERD_BROWSER_TEST_A_B_C" {
		t.Fatalf("unexpected normalized env name: %q", got)
	}
}
