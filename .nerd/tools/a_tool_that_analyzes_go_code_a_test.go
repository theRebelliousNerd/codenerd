package tools

import (
	"context"
	"strings"
	"testing"
)

func TestAToolThatAnalyzesGoCodeA(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantReport  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Happy Path - Simple Function",
			input:      "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}",
			wantReport: "Cyclomatic Complexity Report:\n=============================\nFunction: add - Complexity: 1\n",
			wantErr:    false,
		},
		{
			name: "Happy Path - Function with if statement",
			input: `package main

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nFunction: max - Complexity: 2\n",
			wantErr:    false,
		},
		{
			name: "Happy Path - Function with for loop and logical operators",
			input: `package main

func process(items []int) int {
	count := 0
	for _, item := range items {
		if item > 0 && item < 100 {
			count++
		}
	}
	return count
}`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nFunction: process - Complexity: 4\n",
			wantErr:    false,
		},
		{
			name: "Happy Path - Multiple Functions",
			input: `package main

func add(a, b int) int {
	return a + b
}

func subtract(a, b int) int {
	return a - b
}`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nFunction: add - Complexity: 1\nFunction: subtract - Complexity: 1\n",
			wantErr:    false,
		},
		{
			name: "Happy Path - Function with select and defer",
			input: `package main

func worker(ch chan int) {
	defer close(ch)
	select {
	case ch <- 1:
	default:
	}
}`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nFunction: worker - Complexity: 3\n",
			wantErr:    false,
		},
		{
			name: "Happy Path - Function with switch/case",
			input: `package main

func check(val int) string {
	switch val {
	case 1:
		return "one"
	case 2:
		return "two"
	default:
		return "other"
	}
}`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nFunction: check - Complexity: 3\n",
			wantErr:    false,
		},
		{
			name:       "Edge Case - No Functions",
			input:      "package main\n\nconst Pi = 3.14",
			wantReport: "Cyclomatic Complexity Report:\n=============================\nNo functions found in the provided code.\n",
			wantErr:    false,
		},
		{
			name:       "Edge Case - Empty Input",
			input:      "",
			wantErr:    true,
			errContains: "input Go code cannot be empty",
		},
		{
			name: "Edge Case - Function with no body",
			input: `package main

func noBody()`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nNo functions found in the provided code.\n",
			wantErr:    false,
		},
		{
			name: "Edge Case - Unbalanced Braces",
			input: `package main

func unbalanced() {
	if true {
		// missing closing brace
}`,
			wantReport: "Cyclomatic Complexity Report:\n=============================\nNo functions found in the provided code.\n",
			wantErr:    false,
		},
		{
			name: "Error Case - Context Cancellation",
			input:      "package main\n\nfunc add(a, b int) int { return a + b }",
			wantErr:    true,
			errContains: "context canceled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.name == "Error Case - Context Cancellation" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel() // Cancel the context immediately
			}

			got, err := aToolThatAnalyzesGoCodeA(ctx, tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("aToolThatAnalyzesGoCodeA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("aToolThatAnalyzesGoCodeA() error = %v, expected to contain %s", err, tt.errContains)
				}
				return
			}

			if got != tt.wantReport {
				t.Errorf("aToolThatAnalyzesGoCodeA() =\n%v,\nwant\n%v", got, tt.wantReport)
			}
		})
	}
}

func TestExtractFunctions(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		want     []Function
		wantErr  bool
	}{
		{
			name:   "Simple Function",
			source: "func add(a, b int) int { return a + b }",
			want:   []Function{{name: "add", body: " return a + b "}},
			wantErr: false,
		},
		{
			name: "Multiple Functions",
			source: `func add(a, b int) int { return a + b }
func subtract(a, b int) int { return a - b }`,
			want: []Function{
				{name: "add", body: " return a + b "},
				{name: "subtract", body: " return a - b "},
			},
			wantErr: false,
		},
		{
			name:   "No Functions",
			source: "package main\n\nconst Pi = 3.14",
			want:   nil,
			wantErr: false,
		},
		{
			name:   "Function with Receiver",
			source: "func (s *MyStruct) Method() { /* do something */ }",
			want:   []Function{{name: "Method", body: " /* do something */ "}},
			wantErr: false,
		},
		{
			name:   "Function with No Return Type",
			source: "func doSomething() { println(\"hello\") }",
			want:   []Function{{name: "doSomething", body: " println(\"hello\") "}},
			wantErr: false,
		},
		{
			name:   "Function with Multiple Parameters and Return Values",
			source: "func divide(a, b int) (int, error) { if b == 0 { return 0, fmt.Errorf(\"divide by zero\") } return a / b, nil }",
			want:   []Function{{name: "divide", body: " if b == 0 { return 0, fmt.Errorf(\"divide by zero\") } return a / b, nil "}},
			wantErr: false,
		},
		{
			name:   "Unbalanced Braces",
			source: "func unbalanced() { if true {",
			want:   nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractFunctions(tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractFunctions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("extractFunctions() got %d functions, want %d", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i].name != tt.want[i].name || got[i].body != tt.want[i].body {
						t.Errorf("extractFunctions() = %v, want %v", got, tt.want)
					}
				}
			}
		})
	}
}

func TestCalculateComplexity(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected int
	}{
		{
			name:     "Empty Body",
			body:     "",
			expected: 1,
		},
		{
			name:     "Simple Body",
			body:     "return a + b",
			expected: 1,
		},
		{
			name:     "Single If",
			body:     "if a > b { return a }",
			expected: 2,
		},
		{
			name:     "Single For",
			body:     "for i := 0; i < 10; i++ { count++ }",
			expected: 2,
		},
		{
			name:     "Single Case",
			body:     "switch val { case 1: return \"one\" }",
			expected: 2,
		},
		{
			name:     "Single Select",
			body:     "select { case <-ch: return }",
			expected: 2,
		},
		{
			name:     "Single Go",
			body:     "go doWork()",
			expected: 2,
		},
		{
			name:     "Single Defer",
			body:     "defer close(ch)",
			expected: 2,
		},
		{
			name:     "Logical AND",
			body:     "if a > 0 && b > 0 { return }",
			expected: 3,
		},
		{
			name:     "Logical OR",
			body:     "if a < 0 || b < 0 { return }",
			expected: 3,
		},
		{
			name:     "Complex Function",
			body:     "for i := 0; i < 10; i++ { if i%2 == 0 && i > 0 { go process(i) } } defer cleanup()",
			expected: 7, // 1 (base) + 1 (for) + 1 (if) + 1 (&&) + 1 (go) + 1 (defer) = 6. Wait, let's re-calculate. Base 1 + for 1 + if 1 + && 1 + go 1 + defer 1 = 6. The original code had 7. Let's re-verify. Ah, the regex for `&&` and `||` is not word-boundary aware. `i%2 == 0 && i > 0` will match `&&`. `i < 0 || b < 0` will match `||`. The calculation is correct. Let's re-calculate the expected value. Base 1 + for 1 + if 1 + && 1 + go 1 + defer 1 = 6. The test expectation was wrong. Let's fix it.
			expected: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateComplexity(tt.body)
			if got != tt.expected {
				t.Errorf("calculateComplexity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRegisterAToolThatAnalyzesGoCodeA(t *testing.T) {
	registry := make(map[string]interface{})
	RegisterAToolThatAnalyzesGoCodeA(registry)

	if _, ok := registry["a_tool_that_analyzes_go_code_a"]; !ok {
		t.Errorf("RegisterAToolThatAnalyzesGoCodeA() did not register the tool")
	}

	// Check if the registered value is a function
	if _, ok := registry["a_tool_that_analyzes_go_code_a"].(func(context.Context, string) (string, error)); !ok {
		t.Errorf("Registered item is not of the correct function type")
	}
}