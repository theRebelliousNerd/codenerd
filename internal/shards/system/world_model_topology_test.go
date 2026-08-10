package system

import "testing"

func TestNormalizeTopologyPath(t *testing.T) {
	workspaceRoot := `C:\CodeProjects\codeNERD`
	absoluteWindows := `C:\CodeProjects\codeNERD\internal\session\gate_names_test.go`
	relativePOSIX := "internal/session/gate_names_test.go"
	backslashRelative := `internal\session\gate_names_test.go`

	cases := []struct {
		name  string
		input string
	}{
		{"absolute Windows", absoluteWindows},
		{"relative POSIX", relativePOSIX},
		{"backslash relative", backslashRelative},
	}

	var want string
	for i, tc := range cases {
		got := normalizeTopologyPath(workspaceRoot, tc.input)
		if i == 0 {
			want = got
			// Ensure the canonical form is the expected POSIX relative path.
			expected := "internal/session/gate_names_test.go"
			if got != expected {
				t.Fatalf("normalizeTopologyPath(%q, %q) = %q, want %q", workspaceRoot, tc.input, got, expected)
			}
		} else {
			if got != want {
				t.Errorf("normalizeTopologyPath(%q, %q) = %q, want %q (same as absolute Windows case %q)", workspaceRoot, tc.input, got, want, absoluteWindows)
			}
		}
	}

	// Additional coverage: workspace root with forward slashes and absolute path with forward slashes.
	t.Run("forward slash root and absolute", func(t *testing.T) {
		wsForward := "C:/CodeProjects/codeNERD"
		absForward := "C:/CodeProjects/codeNERD/internal/session/gate_names_test.go"
		gotAbs := normalizeTopologyPath(wsForward, absForward)
		gotRel := normalizeTopologyPath(wsForward, relativePOSIX)
		gotBack := normalizeTopologyPath(wsForward, backslashRelative)
		if gotAbs != gotRel || gotAbs != gotBack {
			t.Errorf("forward-slash variants diverge: abs=%q rel=%q back=%q", gotAbs, gotRel, gotBack)
		}
		if gotAbs != "internal/session/gate_names_test.go" {
			t.Errorf("forward-slash absolute got %q, want %q", gotAbs, "internal/session/gate_names_test.go")
		}
	})

	t.Run("already relative POSIX unchanged", func(t *testing.T) {
		got := normalizeTopologyPath(workspaceRoot, relativePOSIX)
		if got != relativePOSIX {
			t.Errorf("relative POSIX should be unchanged, got %q want %q", got, relativePOSIX)
		}
	})

	t.Run("backslash converts to slash", func(t *testing.T) {
		got := normalizeTopologyPath(workspaceRoot, backslashRelative)
		if got != relativePOSIX {
			t.Errorf("backslash path got %q, want %q", got, relativePOSIX)
		}
	})
}
