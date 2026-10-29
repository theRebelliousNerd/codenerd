package campaign

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Ladder V1 at the campaign's boundary: the document writer's reply is the
// document. Only a reply that is wholly one fenced block is unwrapped.
func TestUnwrapReplyFence(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, reply, want string }{
		{"plain document", "# Title\n\nBody.\n", "# Title\n\nBody.\n"},
		{"document with an untagged fence inside",
			"# Guide\n\n```\nnerd check-mangle\n```\n\nThen read it.\n",
			"# Guide\n\n```\nnerd check-mangle\n```\n\nThen read it.\n"},
		{"wholly fenced with an info string", "```markdown\n# Title\n\nBody.\n```\n", "# Title\n\nBody.\n"},
		{"wholly fenced, bare, surrounding blank lines", "\n```\nBody.\n```\n\n", "Body.\n"},
		{"wholly fenced with fences inside", "````md\n# T\n\n```go\nx := 1\n```\n````", "# T\n\n```go\nx := 1\n```\n"},
		{"wholly fenced, CRLF", "```markdown\r\n# T\r\n```\r\n", "# T\r\n"},
		{"prose before the fence is kept", "Here it is:\n```\nBody.\n```\n", "Here it is:\n```\nBody.\n```\n"},
		{"closing fence of another length", "````\nBody.\n```\n", "````\nBody.\n```\n"},
		{"an empty fenced block", "```\n```", ""},
		{"no newline at all", "just text", "just text"},
		{"a lone fence line", "```\n", "```\n"},
	}
	for _, tc := range cases {
		if got := unwrapReplyFence(tc.reply); got != tc.want {
			t.Errorf("%s: unwrapReplyFence(%q) = %q, want %q", tc.name, tc.reply, got, tc.want)
		}
	}
}

// Through the production fallback and a real VirtualStore: a generated
// document with a fenced example lands whole and byte for byte -- its trailing
// newline too -- and a reply the model fenced as a whole lands as its inside.
// The campaign's old extractor wrote the first fence's inside, trimmed; the
// store's old extractor did it again, and its validator then failed the
// write over the trimmed newline.
func TestFileTask_FallbackWritesTheDocumentAsGenerated(t *testing.T) {
	t.Parallel()
	const document = "# Report\n\nRun the check:\n\n```\nnerd check-mangle\n```\n\nIt reports every finding.\n"
	cases := []struct{ name, reply, file string }{
		{"as generated", document, "docs/report.md"},
		{"fenced as a whole", "```markdown\n" + document + "```\n", "docs/fenced.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, root, generated := fallbackGateFixture(t, errors.New("coder explored without writing"))
			o.llmClient = &MockLLMClient{CompleteFunc: func(context.Context, string) (string, error) {
				*generated++
				return tc.reply, nil
			}}
			task := &Task{ID: "/task_doc_verbatim", Type: TaskTypeFileCreate, Description: "Write the report", WriteSet: []string{tc.file}}
			if _, err := o.executeFileTask(context.Background(), task); err != nil {
				t.Fatalf("the document fallback failed: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.file)))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != document {
				t.Fatalf("%s holds\n%q\nwant the generated document\n%q", tc.file, got, document)
			}
		})
	}
}
