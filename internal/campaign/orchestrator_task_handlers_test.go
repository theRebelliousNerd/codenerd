package campaign

import (
	"testing"
)

// TODO: TEST_GAP: TestExecuteTask_Dispatch
// Verify that executeTask correctly routes tasks based on TaskType.
// Edge cases:
// - Unknown TaskType (should route to generic or error)
// - Empty TaskType
// - Explicit Shard override (task.Shard != "")

// TODO: TEST_GAP: TestExecuteFileTask_ShardFailure_Fallback
// Verify that if spawnTask fails (returns error), the system falls back to executeFileTaskFallback.
// Verify that the fallback mechanism correctly invokes the LLM and writes the file.
// Edge cases:
// - LLM returns error
// - LLM returns invalid content (no code block)
// - File write fails (permissions, disk full)

// TODO: TEST_GAP: TestExecuteFileTask_VerificationFailure
// Verify that if the shard returns success but the file is not created, the fallback is triggered.
// Edge cases:
// - Shard claims success but file missing
// - Shard claims success but file is empty

func TestExtractCodeBlock_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		lang     string
		expected string
	}{
		{
			name:     "standard markdown",
			input:    "Here is the code:\n```go\npackage main\nfunc main() {}\n```",
			lang:     "go",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "no fences",
			input:    "package main\nfunc main() {}",
			lang:     "go",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "multiple blocks",
			input:    "Block 1:\n```go\nfunc one() {}\n```\nBlock 2:\n```go\nfunc two() {}\n```",
			lang:     "go",
			expected: "func one() {}",
		},
		{
			name:     "nested backticks inside code",
			input:    "```go\nfmt.Println(\"`backticks`\")\n```",
			lang:     "go",
			expected: "fmt.Println(\"`backticks`\")",
		},
		{
			name:     "language mismatch (should return original)",
			input:    "```python\nprint('hello')\n```",
			lang:     "go",
			expected: "```python\nprint('hello')\n```",
		},
		{
			name:     "bare fences",
			input:    "```\nfunc main() {}\n```",
			lang:     "go",
			expected: "func main() {}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCodeBlock(tc.input, tc.lang)
			if got != tc.expected {
				t.Errorf("extractCodeBlock(%q, %q) = %q; want %q", tc.input, tc.lang, got, tc.expected)
			}
		})
	}
}

func TestExtractPathFromDescription_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		expected string
	}{
		{
			name:     "create pattern",
			desc:     "Create internal/domain/foo.go",
			expected: "internal/domain/foo.go",
		},
		{
			name:     "file pattern",
			desc:     "file: path/to/file.go",
			expected: "path/to/file.go",
		},
		{
			name:     "bare path",
			desc:     "cmd/nerd/main.go",
			expected: "cmd/nerd/main.go",
		},
		{
			name:     "no path",
			desc:     "Just a description without path",
			expected: "",
		},
		{
			name:     "multiple paths (first match)",
			desc:     "Update internal/a.go and internal/b.go",
			expected: "internal/a.go",
		},
		{
			name:     "internal path",
			desc:     "internal/pkg/utils.go needs update",
			expected: "internal/pkg/utils.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPathFromDescription(tc.desc)
			if got != tc.expected {
				t.Errorf("extractPathFromDescription(%q) = %q; want %q", tc.desc, got, tc.expected)
			}
		})
	}
}

// TODO: TEST_GAP: TestExecuteTestRunTask_Timeout
// Verify that test execution enforces timeouts.
// Edge cases:
// - Test hangs indefinitely
// - Test output exceeds buffer limits

// TODO: TEST_GAP: TestExecuteToolCreateTask_Autopoiesis
// Verify the interaction with the kernel for tool creation.
// Edge cases:
// - Kernel assertion failure
// - Timeout waiting for tool_registered fact
// - Context cancellation during wait

// TODO: TEST_GAP: TestSpawnTask_InputValidation
// Verify spawnTask handles invalid inputs.
// Edge cases:
// - Nil taskExecutor
// - Empty shard type
// - Huge task payload

func BenchmarkExtractPathFromDescription(b *testing.B) {
	descriptions := []string{
		"Create internal/domain/foo.go",
		"file: path/to/file.go",
		"cmd/nerd/main.go",
		"Just a description without path",
		"internal/pkg/utils.go needs update",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, desc := range descriptions {
			extractPathFromDescription(desc)
		}
	}
}
