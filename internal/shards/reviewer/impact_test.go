package reviewer

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseModifiedFunctionsFromDiff(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want []ModifiedFunction
	}{
		{
			name: "single file single function",
			diff: `diff --git a/internal/core/kernel.go b/internal/core/kernel.go
index abc123..def456 100644
--- a/internal/core/kernel.go
+++ b/internal/core/kernel.go
@@ -100,6 +100,7 @@ func (k *RealKernel) Query(predicate string) ([]Fact, error) {
 	// existing code
+	// new line
 }`,
			want: []ModifiedFunction{
				{
					Name:      "Query",
					File:      "internal/core/kernel.go",
					StartLine: 100,
				},
			},
		},
		{
			name: "multiple functions same file",
			diff: `diff --git a/internal/shards/reviewer.go b/internal/shards/reviewer.go
index abc123..def456 100644
--- a/internal/shards/reviewer.go
+++ b/internal/shards/reviewer.go
@@ -50,6 +50,7 @@ func (r *ReviewerShard) Execute(ctx context.Context, task string) (string, error) {
+	// change 1
 }
@@ -120,6 +121,7 @@ func (r *ReviewerShard) analyzeFile(ctx context.Context, path string) error {
+	// change 2
 }`,
			want: []ModifiedFunction{
				{
					Name:      "Execute",
					File:      "internal/shards/reviewer.go",
					StartLine: 50,
				},
				{
					Name:      "analyzeFile",
					File:      "internal/shards/reviewer.go",
					StartLine: 121,
				},
			},
		},
		{
			name: "multiple files",
			diff: `diff --git a/internal/core/kernel.go b/internal/core/kernel.go
@@ -10,6 +10,7 @@ func NewRealKernel() *RealKernel {
+	// change
 }
diff --git a/internal/store/local.go b/internal/store/local.go
@@ -25,6 +25,7 @@ func (s *LocalStore) Save(key string, value interface{}) error {
+	// change
 }`,
			want: []ModifiedFunction{
				{
					Name:      "NewRealKernel",
					File:      "internal/core/kernel.go",
					StartLine: 10,
				},
				{
					Name:      "Save",
					File:      "internal/store/local.go",
					StartLine: 25,
				},
			},
		},
		{
			name: "no function context in hunk header",
			diff: `diff --git a/README.md b/README.md
@@ -1,5 +1,6 @@
 # Title
+New line
 Old content`,
			want: []ModifiedFunction{},
		},
		{
			name: "empty diff",
			diff: "",
			want: []ModifiedFunction{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseModifiedFunctionsFromDiff(tt.diff)
			if len(got) != len(tt.want) {
				t.Errorf("ParseModifiedFunctionsFromDiff() got %d functions, want %d", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Name != tt.want[i].Name {
					t.Errorf("function[%d].Name = %q, want %q", i, got[i].Name, tt.want[i].Name)
				}
				if got[i].File != tt.want[i].File {
					t.Errorf("function[%d].File = %q, want %q", i, got[i].File, tt.want[i].File)
				}
				if got[i].StartLine != tt.want[i].StartLine {
					t.Errorf("function[%d].StartLine = %d, want %d", i, got[i].StartLine, tt.want[i].StartLine)
				}
			}
		})
	}
}

func TestExtractLineRange(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		startLine int
		endLine   int
		maxLines  int
		want      string
		wantErr   bool
	}{
		{
			name:      "normal range",
			content:   "line1\nline2\nline3\nline4\nline5",
			startLine: 2,
			endLine:   4,
			maxLines:  10,
			want:      "line2\nline3\nline4",
			wantErr:   false,
		},
		{
			name:      "truncated",
			content:   "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
			startLine: 1,
			endLine:   10,
			maxLines:  3,
			want:      "line1\nline2\nline3\n// ... (truncated)",
			wantErr:   false,
		},
		{
			name:      "invalid range",
			content:   "line1\nline2",
			startLine: 5,
			endLine:   3,
			maxLines:  10,
			want:      "",
			wantErr:   true,
		},
		{
			name:      "single line",
			content:   "line1\nline2\nline3",
			startLine: 2,
			endLine:   2,
			maxLines:  10,
			want:      "",
			wantErr:   true, // startIdx >= endIdx
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractLineRange(tt.content, tt.startLine, tt.endLine, tt.maxLines)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractLineRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("extractLineRange() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPriorityAtomToInt(t *testing.T) {
	tests := []struct {
		atom string
		want int
	}{
		{"/high", 100},
		{"/critical", 100},
		{"/medium", 50},
		{"/normal", 50},
		{"/low", 25},
		{"high", 100},
		{"unknown", 50},
		{"", 50},
	}

	for _, tt := range tests {
		t.Run(tt.atom, func(t *testing.T) {
			got := priorityAtomToInt(tt.atom)
			if got != tt.want {
				t.Errorf("priorityAtomToInt(%q) = %d, want %d", tt.atom, got, tt.want)
			}
		})
	}
}

func TestParseCallerRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantName string
		wantFile string
	}{
		{"pkg.MyFunc", "MyFunc", ""},
		{"internal/core/kernel.go:Query", "Query", "internal/core/kernel.go"},
		{"SimpleFunc", "SimpleFunc", ""},
		{"nested.pkg.Func", "Func", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			gotName, gotFile := parseCallerRef(tt.ref)
			if gotName != tt.wantName {
				t.Errorf("parseCallerRef(%q) name = %q, want %q", tt.ref, gotName, tt.wantName)
			}
			if gotFile != tt.wantFile {
				t.Errorf("parseCallerRef(%q) file = %q, want %q", tt.ref, gotFile, tt.wantFile)
			}
		})
	}
}

func TestFindFunctionEnd(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		startIdx int
		want     int
	}{
		{
			name:     "simple function",
			lines:    []string{"func foo() {", "  return", "}"},
			startIdx: 0,
			want:     2,
		},
		{
			name:     "nested braces",
			lines:    []string{"func foo() {", "  if x {", "    y()", "  }", "  return", "}"},
			startIdx: 0,
			want:     5,
		},
		{
			name:     "no closing brace",
			lines:    []string{"func foo() {", "  return"},
			startIdx: 0,
			want:     maxFunctionBodyLines, // Falls back to max
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findFunctionEnd(tt.lines, tt.startIdx)
			if got != tt.want {
				t.Errorf("findFunctionEnd() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestImpactContext_FormatForPrompt(t *testing.T) {
	tests := []struct {
		name   string
		ctx    *ImpactContext
		checks []string
	}{
		{
			name: "with modified functions and callers",
			ctx: &ImpactContext{
				ModifiedFunctions: []ModifiedFunction{
					{Name: "Query", File: "kernel.go", StartLine: 100},
				},
				ImpactedCallers: []ImpactedCaller{
					{Name: "Execute", File: "shard.go", Priority: 90, Body: "func Execute() {}"},
				},
				AffectedFiles: []string{"kernel.go", "shard.go"},
			},
			checks: []string{
				"## Impact Analysis Context",
				"### Modified Functions",
				"`Query` in `kernel.go`",
				"### Impacted Callers",
				"`Execute`",
				"**Priority: HIGH**",
				"```go",
				"func Execute() {}",
				"**Impact Summary:**",
			},
		},
		{
			name: "nil context",
			ctx:  nil,
			checks: []string{
				"", // Empty output
			},
		},
		{
			name: "empty context",
			ctx: &ImpactContext{
				ModifiedFunctions: []ModifiedFunction{},
				ImpactedCallers:   []ImpactedCaller{},
			},
			checks: []string{
				"", // Empty output
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.FormatForPrompt()
			for _, check := range tt.checks {
				if check == "" {
					if got != "" {
						t.Errorf("FormatForPrompt() expected empty, got %q", got)
					}
				} else if !strings.Contains(got, check) {
					t.Errorf("FormatForPrompt() missing %q in output:\n%s", check, got)
				}
			}
		})
	}
}

func TestImpactContext_FormatCompact(t *testing.T) {
	tests := []struct {
		name string
		ctx  *ImpactContext
		want string
	}{
		{
			name: "nil context",
			ctx:  nil,
			want: "No impact context",
		},
		{
			name: "with data",
			ctx: &ImpactContext{
				ModifiedFunctions: []ModifiedFunction{{Name: "A"}, {Name: "B"}},
				ImpactedCallers:   []ImpactedCaller{{Name: "X"}, {Name: "Y"}},
				AffectedFiles:     []string{"a.go", "b.go", "c.go"},
			},
			want: "Modified: 2 funcs, Impacted: [X Y], Files: 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.FormatCompact()
			if got != tt.want {
				t.Errorf("FormatCompact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		item  string
		want  bool
	}{
		{
			name:  "found",
			slice: []string{"a", "b", "c"},
			item:  "b",
			want:  true,
		},
		{
			name:  "not found",
			slice: []string{"a", "b", "c"},
			item:  "d",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			item:  "a",
			want:  false,
		},
		{
			name:  "nil slice",
			slice: nil,
			item:  "a",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsString(tt.slice, tt.item)
			if got != tt.want {
				t.Errorf("containsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractGoFunctionBody(t *testing.T) {
	r := &ReviewerShard{}

	content := `package main

import "fmt"

// Query retrieves facts from the kernel.
func (k *Kernel) Query(predicate string) ([]Fact, error) {
	result := k.store.Get(predicate)
	if result == nil {
		return nil, fmt.Errorf("not found")
	}
	return result, nil
}

// Assert adds a fact to the kernel.
func (k *Kernel) Assert(fact Fact) error {
	return k.store.Put(fact)
}
`

	tests := []struct {
		name     string
		funcName string
		wantErr  bool
		contains []string
	}{
		{
			name:     "find Query method",
			funcName: "Query",
			wantErr:  false,
			contains: []string{"func (k *Kernel) Query", "predicate string", "return result, nil"},
		},
		{
			name:     "find Assert method",
			funcName: "Assert",
			wantErr:  false,
			contains: []string{"func (k *Kernel) Assert", "return k.store.Put"},
		},
		{
			name:     "function not found",
			funcName: "NonExistent",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.extractGoFunctionBody(content, tt.funcName)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractGoFunctionBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for _, c := range tt.contains {
					if !strings.Contains(got, c) {
						t.Errorf("extractGoFunctionBody() missing %q in:\n%s", c, got)
					}
				}
			}
		})
	}
}

// Verify cmp import is used
var _ = cmp.Diff
