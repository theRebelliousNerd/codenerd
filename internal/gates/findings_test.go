package gates

import (
	"errors"
	"strings"
	"testing"
)

func failed(g Gate, node, output string) Result {
	return Result{Gate: g, Node: node, ExitCode: 1, Output: output}
}

func targets(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Target
	}
	return out
}

func TestFindings_ReadsEachToolchainsFailures(t *testing.T) {
	cases := []struct {
		name   string
		gate   Gate
		node   string
		output string
		want   []string
	}{
		{
			name: "go test failures, log lines folded into the test",
			gate: Gate{ID: "go:test", Kind: Test}, node: "store",
			output: "--- FAIL: TestGet (0.00s)\n    store_test.go:12: got 3, want 4\n--- FAIL: TestPut (0.01s)\n    --- FAIL: TestPut/empty (0.00s)\nFAIL\nFAIL\texample.com/m/store\t0.02s\n",
			want:   []string{"store::TestGet", "store::TestPut", "store::TestPut/empty"},
		},
		{
			name: "go vet / build diagnostics",
			gate: Gate{ID: "go:vet", Kind: Lint}, node: "store",
			output: "# example.com/m/store\nstore/store.go:10:2: fmt.Printf format %d has arg s of wrong type string\nstore/other.go:3:1: unreachable code\n",
			want:   []string{"store/store.go", "store/other.go"},
		},
		{
			name:   "tsc",
			gate:   Gate{ID: "js:typecheck", Kind: Build},
			output: "src/app/main.ts(12,5): error TS2345: Argument of type 'string' is not assignable to parameter of type 'number'.\n",
			want:   []string{"src/app/main.ts"},
		},
		{
			name: "pytest short summary",
			gate: Gate{ID: "python:pytest", Kind: Test}, node: "shop",
			output: "=== short test summary info ===\nFAILED shop/tests/test_cart.py::test_total - AssertionError: assert 3 == 4\nERROR shop/tests/test_db.py\n",
			want:   []string{"shop/tests/test_cart.py::test_total", "shop/tests/test_db.py"},
		},
		{
			name: "python traceback",
			gate: Gate{ID: "python:compile", Kind: Build}, node: "shop",
			output: "*** Error compiling 'shop/cart.py'...\n  File \"shop/cart.py\", line 4\n    def total(:\n              ^\nSyntaxError: invalid syntax\n",
			want:   []string{"shop/cart.py"},
		},
		{
			name:   "cargo test",
			gate:   Gate{ID: "rust:test", Kind: Test},
			output: "running 2 tests\ntest parse::handles_empty ... ok\ntest parse::rejects_bad ... FAILED\n",
			want:   []string{".::parse::rejects_bad"},
		},
		{
			name:   "rustc error with its location",
			gate:   Gate{ID: "rust:build", Kind: Build},
			output: "error[E0425]: cannot find value `x` in this scope\n  --> src/main.rs:3:13\n   |\n",
			want:   []string{"src/main.rs"},
		},
		{
			name:   "unreadable failure still reports one finding",
			gate:   Gate{ID: "nerd.md:deadcode", Kind: Audit},
			output: "counting...\nNew unreachable functions (1): pkg.F\n\n",
			want:   []string{"."},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := Findings("/ws", failed(tc.gate, tc.node, tc.output))
			got := targets(fs)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("targets = %v, want %v\nfindings: %+v", got, tc.want, fs)
			}
			for _, f := range fs {
				if f.ID == "" || f.Gate != tc.gate.ID || f.Kind != tc.gate.Kind || f.Message == "" {
					t.Fatalf("incomplete finding: %+v", f)
				}
			}
		})
	}
}

// The same failure keeps its identity when the code above it moves; a test
// keeps its identity when a partial fix changes its message.
func TestFindings_IdentitySurvivesLineMovesAndMessageChanges(t *testing.T) {
	vet := Gate{ID: "go:vet", Kind: Lint}
	a := Findings("/ws", failed(vet, "x", "x/x.go:10:2: unreachable code\n"))
	b := Findings("/ws", failed(vet, "x", "x/x.go:57:9: unreachable code\n"))
	if a[0].ID != b[0].ID || a[0].Signature != b[0].Signature {
		t.Fatalf("a moved diagnostic changed identity: %+v vs %+v", a[0], b[0])
	}
	c := Findings("/ws", failed(vet, "x", "x/x.go:10:2: self-assignment of y\n"))
	if c[0].ID == a[0].ID {
		t.Fatal("a different diagnostic in the same file must be a different finding")
	}

	test := Gate{ID: "go:test", Kind: Test}
	d := Findings("/ws", failed(test, "x", "--- FAIL: TestX (0.00s)\n    x_test.go:5: got 1\n"))
	e := Findings("/ws", failed(test, "x", "--- FAIL: TestX (0.30s)\n    x_test.go:5: got 2\n"))
	if d[0].ID != e[0].ID {
		t.Fatal("a failing test is one finding however its message changes")
	}
}

func TestFindings_AbsolutePathsBecomeWorkspaceRelative(t *testing.T) {
	fs := Findings("/ws", failed(Gate{ID: "lint", Kind: Lint}, "", "/ws/pkg/a.py:3:1: E999 SyntaxError at /ws/pkg/a.py\n"))
	if len(fs) != 1 || fs[0].Target != "pkg/a.py" || strings.Contains(fs[0].Signature, "/ws") {
		t.Fatalf("finding = %+v", fs)
	}
}

func TestFindings_PassAndUnverifiedReportNothing(t *testing.T) {
	g := Gate{ID: "go:test", Kind: Test}
	if fs := Findings("/ws", Result{Gate: g, Passed: true, Output: "--- FAIL: TestNope\n"}); fs != nil {
		t.Fatalf("a pass has no findings: %+v", fs)
	}
	if fs := Findings("/ws", Result{Gate: g, ExitCode: -1, Err: errors.New("stopped")}); fs != nil {
		t.Fatalf("an unverified run is evidence of nothing: %+v", fs)
	}
}

func TestFindings_BoundedPerRun(t *testing.T) {
	var b strings.Builder
	for i := range MaxFindingsPerRun * 3 {
		b.WriteString("--- FAIL: TestN")
		b.WriteString(strings.Repeat("x", i+1))
		b.WriteString(" (0.00s)\n")
	}
	if fs := Findings("/ws", failed(Gate{ID: "go:test", Kind: Test}, "n", b.String())); len(fs) != MaxFindingsPerRun {
		t.Fatalf("findings = %d, want the bound %d", len(fs), MaxFindingsPerRun)
	}
}
