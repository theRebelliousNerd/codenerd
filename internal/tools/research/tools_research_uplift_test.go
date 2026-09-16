package research

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestContext7_StringMaxDocsHonoredEndToEnd pins the canonical-coercion
// behavior change through the full executor: a model emitting max_docs as the
// string "1" gets one document, not the default-powered full set. The old
// research-local argInt rejected strings, so the caller's bound was silently
// dropped and all three docs came back.
func TestContext7_StringMaxDocsHonoredEndToEnd(t *testing.T) {
	const docBody = "# Doc\nThis body is deliberately longer than fifty characters so the parser keeps it."
	mock := NewMockTransport()
	mock.RegisterResponder("https://raw.githubusercontent.com/owner/repo/main/llms.txt",
		"- docs/one.md: One\n- docs/two.md: Two\n- docs/three.md: Three", 200)
	for _, name := range []string{"one", "two", "three"} {
		mock.RegisterResponder("https://raw.githubusercontent.com/owner/repo/main/docs/"+name+".md",
			docBody, 200)
	}
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = mock
	defer func() { http.DefaultClient.Transport = oldTransport }()

	tool := Context7Tool()
	ctx := context.Background()

	one, err := tool.Execute(ctx, map[string]any{"topic": "t", "repo": "owner/repo", "max_docs": "1"})
	if err != nil {
		t.Fatalf("string max_docs execute failed: %v", err)
	}
	if got := strings.Count(one, "## Source:"); got != 1 {
		t.Errorf("max_docs \"1\" returned %d docs; want exactly 1", got)
	}

	three, err := tool.Execute(ctx, map[string]any{"topic": "t", "repo": "owner/repo", "max_docs": 3})
	if err != nil {
		t.Fatalf("int max_docs execute failed: %v", err)
	}
	if got := strings.Count(three, "## Source:"); got != 3 {
		t.Errorf("max_docs 3 returned %d docs; want exactly 3", got)
	}
}

// TestBrowserSpecRange_StrictAddresses pins that from/to/line are exact line
// addresses: fractional values are refused, integral shapes (including whole
// floats and decimal strings) are honored, and line stays shorthand for a
// single-line range.
func TestBrowserSpecRange_StrictAddresses(t *testing.T) {
	t.Run("fractional refused", func(t *testing.T) {
		for _, args := range []map[string]any{
			{"line": 1.5},
			{"from": 1.5, "to": 5},
			{"from": 1, "to": 5.5},
			{"from": "abc"},
			{"line": "6.5"},
		} {
			if _, _, err := browserSpecRange(args); err == nil {
				t.Errorf("browserSpecRange(%v) must fail, got success", args)
			}
		}
	})
	t.Run("integral honored", func(t *testing.T) {
		from, to, err := browserSpecRange(map[string]any{"from": 2.0, "to": 4.0})
		if err != nil || from != 2 || to != 4 {
			t.Errorf("whole floats: got (%d,%d,%v); want (2,4,nil)", from, to, err)
		}
		from, to, err = browserSpecRange(map[string]any{"from": "3", "to": "5"})
		if err != nil || from != 3 || to != 5 {
			t.Errorf("decimal strings: got (%d,%d,%v); want (3,5,nil)", from, to, err)
		}
		from, to, err = browserSpecRange(map[string]any{"line": 7})
		if err != nil || from != 7 || to != 7 {
			t.Errorf("line shorthand: got (%d,%d,%v); want (7,7,nil)", from, to, err)
		}
		from, to, err = browserSpecRange(map[string]any{})
		if err != nil || from != 0 || to != 0 {
			t.Errorf("absent: got (%d,%d,%v); want (0,0,nil)", from, to, err)
		}
	})
	t.Run("line yields to explicit range", func(t *testing.T) {
		from, to, err := browserSpecRange(map[string]any{"from": 2, "to": 9, "line": 7})
		if err != nil || from != 2 || to != 9 {
			t.Errorf("got (%d,%d,%v); want (2,9,nil)", from, to, err)
		}
	})
}

// TestBrowserSpecInput_FractionalLineRefused pins the refusal through the real
// input builder: a fractional line never becomes a truncated range.
func TestBrowserSpecInput_FractionalLineRefused(t *testing.T) {
	if _, err := browserSpecInput(nil, map[string]any{"route": "/x", "line": 2.5}); err == nil {
		t.Error("line 2.5 must be refused, got success")
	}
	input, err := browserSpecInput(nil, map[string]any{"route": "/x", "line": 2})
	if err != nil {
		t.Fatalf("line 2 must work: %v", err)
	}
	if input.From != 2 || input.To != 2 {
		t.Errorf("got range (%d,%d); want (2,2)", input.From, input.To)
	}
}

// TestInt64Arg_TimestampShapes pins the canonical 64-bit delegation: since_ms
// reads the same instant from int64, float64, and decimal strings, and falls
// back only on garbage or absence.
func TestInt64Arg_TimestampShapes(t *testing.T) {
	const epochMS = int64(1789544000000)
	cases := []struct {
		name string
		val  any
		want int64
	}{
		{"int64", epochMS, epochMS},
		{"float64 exact", float64(epochMS), epochMS},
		{"fraction truncates", 1500.9, 1500},
		{"decimal string honored", "1789544000000", epochMS},
		{"garbage falls back", "abc", 0},
		{"nil falls back", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.val != nil {
				args["since_ms"] = tc.val
			}
			if got := int64Arg(args, "since_ms", 0); got != tc.want {
				t.Errorf("int64Arg(%#v) = %d; want %d", tc.val, got, tc.want)
			}
		})
	}
}

// TestIntArg_SoftLimitShapes pins the canonical delegation: soft limits honor
// every numeric shape and fall back only on garbage or absence.
func TestIntArg_SoftLimitShapes(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want int
	}{
		{"int", 12, 12},
		{"whole float64", 12.0, 12},
		{"fraction truncates", 12.7, 12},
		{"decimal string honored", "8", 8},
		{"garbage falls back", "abc", 20},
		{"bool falls back", true, 20},
		{"nil falls back", nil, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]any{}
			if tc.val != nil {
				args["max_items"] = tc.val
			}
			if got := intArg(args, "max_items", 20); got != tc.want {
				t.Errorf("intArg(%#v) = %d; want %d", tc.val, got, tc.want)
			}
		})
	}
}
