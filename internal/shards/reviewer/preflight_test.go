package reviewer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestFilterGoFiles(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "filters only go files",
			input: []string{"main.go", "README.md", "config.json", "util.go"},
			want:  []string{"main.go", "util.go"},
		},
		{
			name:  "excludes test files",
			input: []string{"main.go", "main_test.go", "util.go", "util_test.go"},
			want:  []string{"main.go", "util.go"},
		},
		{
			name:  "empty input",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "no go files",
			input: []string{"README.md", "Makefile", "config.yaml"},
			want:  []string{},
		},
		{
			name:  "mixed paths with directories",
			input: []string{"internal/core/kernel.go", "internal/core/kernel_test.go", "docs/README.md"},
			want:  []string{"internal/core/kernel.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterGoFiles(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("filterGoFiles() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetPackagesFromFiles(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		wantContain []string
	}{
		{
			name:        "single file in current dir",
			input:       []string{"main.go"},
			wantContain: []string{"."},
		},
		{
			name:        "files in subdirectory",
			input:       []string{"internal/core/kernel.go", "internal/core/store.go"},
			wantContain: []string{"./internal/core"},
		},
		{
			name:        "files in multiple packages",
			input:       []string{"internal/core/kernel.go", "internal/shards/coder.go"},
			wantContain: []string{"./internal/core", "./internal/shards"},
		},
		{
			name:        "empty input",
			input:       []string{},
			wantContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPackagesFromFiles(tt.input)

			// Check that all expected packages are present
			for _, want := range tt.wantContain {
				found := false
				for _, pkg := range got {
					if pkg == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("getPackagesFromFiles() missing package %q, got %v", want, got)
				}
			}
		})
	}
}

func TestParseBuildOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantLen int
		check   func([]Diagnostic) error
	}{
		{
			name: "standard go build error",
			output: `internal/foo/bar.go:10:5: undefined: SomeType
internal/foo/bar.go:15: cannot use x (type int) as type string`,
			wantLen: 2,
			check: func(diags []Diagnostic) error {
				if diags[0].File != "internal/foo/bar.go" {
					return errorf("expected file 'internal/foo/bar.go', got %q", diags[0].File)
				}
				if diags[0].Line != 10 {
					return errorf("expected line 10, got %d", diags[0].Line)
				}
				if !strings.Contains(diags[0].Message, "undefined") {
					return errorf("expected message containing 'undefined', got %q", diags[0].Message)
				}
				return nil
			},
		},
		{
			name:    "empty output",
			output:  "",
			wantLen: 0,
		},
		{
			name: "multiple errors with severity detection",
			output: `main.go:5:2: undefined: foo
main.go:10:8: cannot convert x`,
			wantLen: 2,
			check: func(diags []Diagnostic) error {
				// "undefined" should be CRITICAL
				if diags[0].Severity != "CRITICAL" {
					return errorf("expected CRITICAL for undefined, got %q", diags[0].Severity)
				}
				// "cannot" should also be CRITICAL
				if diags[1].Severity != "CRITICAL" {
					return errorf("expected CRITICAL for cannot, got %q", diags[1].Severity)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBuildOutput(tt.output)

			if len(got) != tt.wantLen {
				t.Errorf("parseBuildOutput() returned %d diagnostics, want %d", len(got), tt.wantLen)
				return
			}

			if tt.check != nil {
				if err := tt.check(got); err != nil {
					t.Errorf("parseBuildOutput() check failed: %v", err)
				}
			}
		})
	}
}

func TestParseVetOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		wantLen int
		check   func([]Diagnostic) error
	}{
		{
			name: "standard go vet output",
			output: `# codenerd/internal/core
internal/core/kernel.go:45:6: unreachable code
internal/core/store.go:20:3: possible nil dereference`,
			wantLen: 2,
			check: func(diags []Diagnostic) error {
				if diags[0].File != "internal/core/kernel.go" {
					return errorf("expected file 'internal/core/kernel.go', got %q", diags[0].File)
				}
				if diags[0].Line != 45 {
					return errorf("expected line 45, got %d", diags[0].Line)
				}
				if diags[0].Severity != "WARNING" {
					return errorf("expected WARNING severity, got %q", diags[0].Severity)
				}
				return nil
			},
		},
		{
			name:    "empty output",
			output:  "",
			wantLen: 0,
		},
		{
			name: "package header only",
			output: `# codenerd/internal/core
`,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVetOutput(tt.output)

			if len(got) != tt.wantLen {
				t.Errorf("parseVetOutput() returned %d diagnostics, want %d", len(got), tt.wantLen)
				return
			}

			if tt.check != nil {
				if err := tt.check(got); err != nil {
					t.Errorf("parseVetOutput() check failed: %v", err)
				}
			}
		})
	}
}

func TestCategorizeVetMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "printf format",
			message: "Printf format %d has arg of wrong type",
			want:    "format",
		},
		{
			name:    "unreachable code",
			message: "unreachable code",
			want:    "dead_code",
		},
		{
			name:    "shadow variable",
			message: "declaration of 'err' shadows declaration",
			want:    "shadowing",
		},
		{
			name:    "lock issue",
			message: "sync.Mutex is not safe to copy",
			want:    "concurrency",
		},
		{
			name:    "nil check",
			message: "nil pointer dereference",
			want:    "nil_safety",
		},
		{
			name:    "loop variable capture",
			message: "loop variable v captured by func literal",
			want:    "loop_capture",
		},
		{
			name:    "race detection",
			message: "potential race on shared variable",
			want:    "concurrency",
		},
		{
			name:    "atomic usage",
			message: "atomic access to 64-bit value",
			want:    "concurrency",
		},
		{
			name:    "generic message",
			message: "some other issue",
			want:    "general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := categorizeVetMessage(tt.message)
			if got != tt.want {
				t.Errorf("categorizeVetMessage(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

// =============================================================================
// FORMATTING TESTS
// =============================================================================

func TestFormatPreFlightFailure(t *testing.T) {
	r := NewReviewerShard()

	diagnostics := []Diagnostic{
		{
			Severity: "CRITICAL",
			Message:  "undefined: SomeType",
			File:     "main.go",
			Line:     10,
		},
		{
			Severity: "ERROR",
			Message:  "cannot convert x",
			File:     "util.go",
			Line:     25,
		},
		{
			Severity: "WARNING",
			Message:  "unreachable code",
			File:     "handler.go",
			Line:     50,
		},
	}

	output := r.formatPreFlightFailure(diagnostics)

	// Verify structure
	if !strings.Contains(output, "# Pre-Flight Check Failed") {
		t.Error("missing header")
	}
	if !strings.Contains(output, "Critical Issues") {
		t.Error("missing critical section")
	}
	if !strings.Contains(output, "Errors") {
		t.Error("missing errors section")
	}
	if !strings.Contains(output, "Warnings") {
		t.Error("missing warnings section")
	}
	if !strings.Contains(output, "main.go:10") {
		t.Error("missing file:line reference for critical")
	}
	if !strings.Contains(output, "undefined: SomeType") {
		t.Error("missing critical message")
	}
}

// =============================================================================
// INTEGRATION TESTS (require ReviewerShard)
// =============================================================================

func TestPreFlightCheckNoGoFiles(t *testing.T) {
	r := NewReviewerShard()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Non-Go files should pass pre-flight immediately
	diagnostics, proceed := r.PreFlightCheck(ctx, []string{"README.md", "Makefile"})

	if !proceed {
		t.Error("expected proceed=true for non-Go files")
	}
	if len(diagnostics) != 0 {
		t.Errorf("expected no diagnostics for non-Go files, got %d", len(diagnostics))
	}
}

func TestPreFlightCheckEmptyFiles(t *testing.T) {
	r := NewReviewerShard()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	diagnostics, proceed := r.PreFlightCheck(ctx, []string{})

	if !proceed {
		t.Error("expected proceed=true for empty file list")
	}
	if len(diagnostics) != 0 {
		t.Errorf("expected no diagnostics for empty list, got %d", len(diagnostics))
	}
}

func TestPreFlightCheckContextCancellation(t *testing.T) {
	r := NewReviewerShard()

	// Create an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	diagnostics, proceed := r.PreFlightCheck(ctx, []string{"main.go"})

	if proceed {
		t.Error("expected proceed=false for cancelled context")
	}
	if len(diagnostics) == 0 {
		t.Error("expected diagnostics for cancelled context")
	}
}

func TestRunPreFlightAndProceedNoFiles(t *testing.T) {
	r := NewReviewerShard()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, proceed := r.RunPreFlightAndProceed(ctx, []string{})

	if !proceed {
		t.Error("expected proceed=true for empty file list")
	}
	if msg != "" {
		t.Errorf("expected empty message, got %q", msg)
	}
}

// =============================================================================
// HELPERS
// =============================================================================

func errorf(format string, args ...interface{}) error {
	return &testError{msg: fmt.Sprintf(format, args...)}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
