package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F-VERIFY-3: the test gate shows green even when new code is not exercised.
// Coverage closes that gap. These tests pin the profile parser that narrows
// `go test -coverprofile` to the files a turn actually wrote.

// TestParseCoverProfile verifies the coverage profile parser against real
// profile text, including mode-line handling, covered/uncovered filtering,
// written-file suffix matching, and malformed input.
func TestParseCoverProfile(t *testing.T) {
	cases := []struct {
		name         string
		profile      string
		writtenFiles []string
		want         []UncoveredBlock
		wantErr      bool
	}{
		{
			name:         "uncovered block in written file is returned",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 3 0\n",
			writtenFiles: []string{"internal/session/foo.go"},
			want: []UncoveredBlock{
				{File: "codenerd/internal/session/foo.go", StartLine: 10, EndLine: 20, NumStmts: 3},
			},
		},
		{
			name:         "covered block with count 1 is filtered",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 3 1\n",
			writtenFiles: []string{"internal/session/foo.go"},
			want:         nil,
		},
		{
			name:         "uncovered block in file not in writtenFiles is filtered",
			profile:      "mode: set\ncodenerd/internal/other/bar.go:5.1,8.2 2 0\n",
			writtenFiles: []string{"internal/session/foo.go"},
			want:         nil,
		},
		{
			name: "mixed covered uncovered and foreign file",
			profile: strings.Join([]string{
				"mode: set",
				"codenerd/internal/session/foo.go:1.1,5.1 2 1",
				"codenerd/internal/session/foo.go:10.14,20.1 3 0",
				"codenerd/internal/other/bar.go:5.1,8.2 2 0",
				"codenerd/internal/session/foo.go:30.1,35.1 1 0",
			}, "\n") + "\n",
			writtenFiles: []string{"internal/session/foo.go"},
			want: []UncoveredBlock{
				{File: "codenerd/internal/session/foo.go", StartLine: 10, EndLine: 20, NumStmts: 3},
				{File: "codenerd/internal/session/foo.go", StartLine: 30, EndLine: 35, NumStmts: 1},
			},
		},
		{
			name: "suffix matching with dot-slash prefix in writtenFiles",
			profile: strings.Join([]string{
				"mode: set",
				"codenerd/internal/session/foo.go:10.14,20.1 3 0",
			}, "\n") + "\n",
			writtenFiles: []string{"./internal/session/foo.go"},
			want: []UncoveredBlock{
				{File: "codenerd/internal/session/foo.go", StartLine: 10, EndLine: 20, NumStmts: 3},
			},
		},
		{
			name:         "malformed missing mode line",
			profile:      "codenerd/internal/session/foo.go:10.14,20.1 3 0\n",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "malformed empty profile",
			profile:      "",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "malformed coverage line missing fields",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 3\n",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "malformed coverage line missing colon",
			profile:      "mode: set\ncodenerd/internal/session/foo.go10.14,20.1 3 0\n",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "malformed coverage line missing comma in range",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14 3 0\n",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "malformed coverage line invalid numStmts",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 bad 0\n",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "malformed coverage line invalid count",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 3 bad\n",
			writtenFiles: []string{"internal/session/foo.go"},
			wantErr:      true,
		},
		{
			name:         "empty lines are skipped",
			profile:      "mode: set\n\ncodenerd/internal/session/foo.go:10.14,20.1 3 0\n\n",
			writtenFiles: []string{"internal/session/foo.go"},
			want: []UncoveredBlock{
				{File: "codenerd/internal/session/foo.go", StartLine: 10, EndLine: 20, NumStmts: 3},
			},
		},
		{
			name:         "no written files means no blocks returned",
			profile:      "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 3 0\n",
			writtenFiles: nil,
			want:         nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCoverProfile(strings.NewReader(tc.profile), tc.writtenFiles)
			if (err != nil) != tc.wantErr {
				t.Fatalf("parseCoverProfile() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseCoverProfile() = %v; want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("parseCoverProfile()[%d] = %+v; want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// uncoveredWrittenCode had no test at all when it was written, while its file
// header claimed it was "exercised through the integration path". This is that
// integration path: a real throwaway module, a real `go test -coverprofile`,
// and an assertion that the function reached but never called is the one
// reported.
func TestUncoveredWrittenCode_ReportsOnlyUnexercisedWrittenCode(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(ws, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module coverprobe\n\ngo 1.21\n")
	// Exercised is called by the test; Unexercised is not. That is the whole
	// point: `go test` goes green either way, and only the profile can tell
	// them apart.
	write("calc.go", `package coverprobe

func Exercised(a, b int) int { return a + b }

func Unexercised(a, b int) int {
	if a > b {
		return a
	}
	return b
}
`)
	write("calc_test.go", `package coverprobe

import "testing"

func TestExercised(t *testing.T) {
	if Exercised(2, 3) != 5 {
		t.Fatal("bad")
	}
}
`)

	blocks, err := uncoveredWrittenCode(
		context.Background(), ws, []string{"."}, []string{"calc.go"})
	if err != nil {
		t.Fatalf("uncoveredWrittenCode: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("no uncovered blocks reported, but Unexercised is never called — `go test` alone cannot catch this, which is why this gate exists")
	}
	for _, b := range blocks {
		if !strings.HasSuffix(filepath.ToSlash(b.File), "calc.go") {
			t.Errorf("reported a block outside the written file: %+v", b)
		}
		if b.NumStmts <= 0 {
			t.Errorf("block reports no statements: %+v", b)
		}
	}

	// A file the turn did not write must never be reported, even though it is
	// in the same package and equally uncovered.
	none, err := uncoveredWrittenCode(
		context.Background(), ws, []string{"."}, []string{"not_written.go"})
	if err != nil {
		t.Fatalf("uncoveredWrittenCode (unwritten filter): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("reported blocks for a file the turn never wrote: %+v", none)
	}
}

// Every not-run path must return (nil, nil) — "no signal", never "nothing
// uncovered". Reading a skipped verification as a clean one is the exact
// failure the Ran/OK split exists to prevent in the sibling gates.
func TestUncoveredWrittenCode_NotRunPathsReturnNoSignal(t *testing.T) {
	cases := []struct {
		name      string
		workspace string
		packages  []string
	}{
		{"empty workspace", "  ", []string{"."}},
		{"no packages", t.TempDir(), nil},
		{"whitespace packages", t.TempDir(), []string{"  ", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks, err := uncoveredWrittenCode(
				context.Background(), tc.workspace, tc.packages, []string{"a.go"})
			if err != nil {
				t.Errorf("not-run path returned an error: %v", err)
			}
			if blocks != nil {
				t.Errorf("not-run path returned blocks: %+v", blocks)
			}
		})
	}
}

// One invocation must produce both signals. Composing verifyTests and
// uncoveredWrittenCode would double the test time of every green write turn --
// ~9s becoming ~18s on internal/session, paid on every edit -- and buy nothing,
// since `go test -coverprofile` already reports pass/fail.
func TestVerifyTestsWithCoverage_OneRunGivesBothSignals(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles and tests a throwaway package")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module coverboth\n\ngo 1.21\n")
	write("calc.go", "package coverboth\n\nfunc Used() int { return 1 }\n\nfunc Unused() int { return 2 }\n")
	write("calc_test.go", "package coverboth\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) { if Used() != 1 { t.Fatal(\"bad\") } }\n")

	v, blocks := verifyTestsWithCoverage(context.Background(), ws, []string{"."}, []string{"calc.go"})
	if !v.Ran || !v.OK {
		t.Fatalf("passing package reported Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}
	if len(blocks) == 0 {
		t.Error("tests passed but Unused is never called; coverage must still report it — that is the whole point of this gate")
	}

	// Failing tests must still yield a verdict. Coverage is secondary and must
	// never be what decides a turn.
	write("calc_test.go", "package coverboth\n\nimport \"testing\"\n\nfunc TestUsed(t *testing.T) { if Used() != 999 { t.Fatal(\"intentional failure\") } }\n")
	v2, _ := verifyTestsWithCoverage(context.Background(), ws, []string{"."}, []string{"calc.go"})
	if !v2.Ran {
		t.Fatal("verification did not run against a failing package")
	}
	if v2.OK {
		t.Fatal("verification passed a package whose tests fail")
	}
}

func TestSummarizeUncovered_CapsTheList(t *testing.T) {
	var many []UncoveredBlock
	for i := 0; i < 20; i++ {
		many = append(many, UncoveredBlock{File: "pkg/a.go", StartLine: i, EndLine: i + 1, NumStmts: 1})
	}
	s := summarizeUncovered(many)
	if !strings.Contains(s, "and 12 more") {
		t.Errorf("long lists must be capped so the warning stays readable, got %q", s)
	}
	if s := summarizeUncovered(nil); s != "" {
		t.Errorf("summarizeUncovered(nil) = %q; want empty", s)
	}
}

// The coverage gate reported this function uncovered on the turn that created
// it (2026-08-08 11:26, "Turn wrote 21 block(s) of Go that no test executes"),
// with `go test` green at the same moment. This test is the response to that
// signal — which is the loop the gate exists to close.
func TestNormalizeCoverPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"internal/session/foo.go", "internal/session/foo.go"},
		{`internal\session\foo.go`, "internal/session/foo.go"},
		{"./internal/session/foo.go", "internal/session/foo.go"},
		{`.\internal\session\foo.go`, "internal/session/foo.go"},
		{"", ""},
		{"./", ""},
		// Exactly one leading "./" is stripped, not all of them.
		{"././foo.go", "./foo.go"},
		// A parent-relative path does not start with "./" and is left alone.
		{"../foo.go", "../foo.go"},
	}
	for _, tc := range cases {
		if got := NormalizeCoverPath(tc.in); got != tc.want {
			t.Errorf("NormalizeCoverPath(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

// Windows path separators in written paths must still match a profile, which
// always uses forward slashes. Without normalisation every comparison on
// Windows fails silently and the gate reports nothing forever.
func TestParseCoverProfile_MatchesWindowsStyleWrittenPaths(t *testing.T) {
	profile := "mode: set\ncodenerd/internal/session/foo.go:10.14,20.1 3 0\n"
	got, err := parseCoverProfile(strings.NewReader(profile), []string{`internal\session\foo.go`})
	if err != nil {
		t.Fatalf("parseCoverProfile: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("backslash-spelled written path did not match the profile: got %v", got)
	}
}
