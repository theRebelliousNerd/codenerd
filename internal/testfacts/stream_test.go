package testfacts

import (
	"strings"
	"testing"
)

// Synthetic stream cases for edges real runs cannot produce on demand:
// mid-line chunk splits, testify-style continuations, benchmarks,
// unknown actions, repeated verdicts, homeless output, and missing
// verdicts. Each stream is small; inputs are built, never pasted large.
func TestSyntheticStreams(t *testing.T) {
	run := func(pkg, test string) string {
		return `{"Action":"run","Package":"` + pkg + `","Test":"` + test + `"}` + "\n"
	}
	out := func(pkg, test, text string) string {
		return `{"Action":"output","Package":"` + pkg + `","Test":"` + test +
			`","Output":` + quoteForStream(text) + "}\n"
	}
	verdict := func(action, pkg, test string) string {
		return `{"Action":"` + action + `","Package":"` + pkg + `","Test":"` + test + `"}` + "\n"
	}
	cases := []struct {
		name   string
		stream string
		check  func(t *testing.T, res *Result)
	}{
		{
			name: "chunk split joins mid-line",
			stream: run("p", "T") +
				out("p", "T", "hel") + out("p", "T", "lo\nwor") + out("p", "T", "ld\n") +
				verdict("pass", "p", "T") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if len(ct.Output) != 2 || ct.Output[0] != "hello" || ct.Output[1] != "world" {
					t.Fatalf("Output = %q", ct.Output)
				}
			},
		},
		{
			name: "indented lines continue the message",
			stream: run("p", "T") +
				out("p", "T", "    x_test.go:10: \n") +
				out("p", "T", "\tError: boom\n") +
				out("p", "T", "\tTest: T\n") +
				verdict("fail", "p", "T") + verdict("fail", "p", ""),
			check: func(t *testing.T, res *Result) {
				if len(res.Failures) != 1 {
					t.Fatalf("Failures = %+v", res.Failures)
				}
				if want := "\n\tError: boom\n\tTest: T"; res.Failures[0].Message != want {
					t.Errorf("Message = %q, want %q", res.Failures[0].Message, want)
				}
			},
		},
		{
			name: "blank line ends the message",
			stream: run("p", "T") +
				out("p", "T", "    x_test.go:10: note\n") +
				out("p", "T", "\n") +
				out("p", "T", "    stray indented\n") +
				verdict("fail", "p", "T") + verdict("fail", "p", ""),
			check: func(t *testing.T, res *Result) {
				if len(res.Failures) != 1 || res.Failures[0].Message != "note" {
					t.Fatalf("Failures = %+v", res.Failures)
				}
			},
		},
		{
			name: "bench is pass",
			stream: run("p", "BenchmarkX") +
				out("p", "BenchmarkX", "BenchmarkX 1 1 ns/op\n") +
				verdict("bench", "p", "BenchmarkX") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "BenchmarkX")
				if ct.Status != StatusPass {
					t.Fatalf("status = %q", ct.Status)
				}
			},
		},
		{
			name: "unknown action output is kept",
			stream: run("p", "T") +
				`{"Action":"frob","Package":"p","Test":"T","Output":"kept\n"}` + "\n" +
				verdict("pass", "p", "T") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if len(ct.Output) != 1 || ct.Output[0] != "kept" {
					t.Fatalf("Output = %q", ct.Output)
				}
			},
		},
		{
			name: "last verdict wins",
			stream: run("p", "T") + verdict("fail", "p", "T") +
				run("p", "T") + verdict("pass", "p", "T") + verdict("pass", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if ct.Status != StatusPass {
					t.Fatalf("status = %q, want pass", ct.Status)
				}
			},
		},
		{
			name:   "homeless output goes raw",
			stream: `{"Action":"output","Output":"orphan\n"}` + "\n",
			check: func(t *testing.T, res *Result) {
				if len(res.Raw) != 1 || res.Raw[0] != "orphan" {
					t.Fatalf("Raw = %q", res.Raw)
				}
			},
		},
		{
			name:   "run without verdict is unknown and factless",
			stream: run("p", "T") + verdict("fail", "p", ""),
			check: func(t *testing.T, res *Result) {
				ct := findTest(t, res.Packages[0], "T")
				if ct.Status != StatusUnknown {
					t.Fatalf("status = %q", ct.Status)
				}
				for _, f := range res.Facts() {
					if f.Predicate == PredTestCase {
						t.Fatalf("unexpected test_case: %v", f)
					}
				}
			},
		},
		{
			name:   "skip without marker stays skip",
			stream: `{"Action":"skip","Package":"p"}` + "\n",
			check: func(t *testing.T, res *Result) {
				if res.Packages[0].Status != StatusSkip {
					t.Fatalf("status = %q", res.Packages[0].Status)
				}
			},
		},
		{
			name:   "crlf stream parses",
			stream: "{\"Action\":\"pass\",\"Package\":\"p\"}\r\n",
			check: func(t *testing.T, res *Result) {
				if len(res.Raw) != 0 || res.Packages[0].Status != StatusPass {
					t.Fatalf("Raw = %q status = %q", res.Raw, res.Packages[0].Status)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, parseString(t, tc.stream))
		})
	}
}

// quoteForStream renders Go text as a JSON string for synthetic events.
func quoteForStream(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
