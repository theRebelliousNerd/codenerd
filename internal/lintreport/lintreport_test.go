package lintreport

import (
	"encoding/json"
	"errors"
	"github.com/google/go-cmp/cmp"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintParsers_ReadEachReport(t *testing.T) {
	t.Parallel()
	dir := filepath.FromSlash("/ws")
	vet := "# lintprobe/a\n{\n\t\"lintprobe/a\": {\n\t\t\"printf\": [\n\t\t\t{\"posn\": \"" + filepath.ToSlash(filepath.Join(dir, "a", "a.go")) +
		":3:9\", \"message\": \"brace } in a message {\"}\n\t\t]\n\t}\n}\nvet: ./b/b.go:4:2: undefined: x\n"
	got, rest, err := VetFindings(vet, dir)
	if err != nil || rest != "" || len(got) != 2 ||
		got[0] != (Finding{File: "a/a.go", Line: 3, Column: 9, Check: "printf", Message: "brace } in a message {"}) ||
		got[1] != (Finding{File: "b/b.go", Line: 4, Column: 2, Check: CompileCheck, Message: "undefined: x"}) {
		t.Fatalf("VetFindings = %+v, rest %q, err %v", got, rest, err)
	}

	sc := `{"code":"U1000","severity":"error","location":{"file":"/ws/a/a.go","line":7,"column":6},"message":"func unused is unused"}` + "\n" +
		`{"code":"compile","severity":"error","location":{"file":"","line":0,"column":0},"message":"# lintprobe/b\n./b/b.go:3:23: undefined: missing"}`
	got, err = StaticcheckFindings(sc, dir)
	if err != nil || len(got) != 2 || got[0].File != "a/a.go" || got[0].Check != "U1000" ||
		got[1] != (Finding{File: "b/b.go", Line: 3, Column: 23, Check: CompileCheck, Message: "undefined: missing"}) {
		t.Fatalf("StaticcheckFindings = %+v, err %v", got, err)
	}

	gl := `{"Issues":[{"FromLinter":"errcheck","Text":"Error return value is not checked","Pos":{"Filename":"a/a.go","Line":5,"Column":2}}],"Report":{}}`
	got, _, err = GolangciFindings(gl, dir)
	if err != nil || len(got) != 1 || got[0] != (Finding{File: "a/a.go", Line: 5, Column: 2, Check: "errcheck", Message: "Error return value is not checked"}) {
		t.Fatalf("GolangciFindings = %+v, err %v", got, err)
	}
	if _, _, err := GolangciFindings("level=error msg=bad config", dir); err == nil {
		t.Fatal("a report that is not JSON was read as clean")
	}
}

func TestLintParsers_RejectMalformedReportsWithoutPartialFindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		parse    func(string, string) ([]Finding, string, error)
		input    string
		wantRest string
		wantEOF  bool
		wantType bool
	}{
		{
			name: "vet truncated after valid report", parse: VetFindings,
			input:    `{"p":{"printf":[{"posn":"a.go:1:2","message":"first"}]}}` + "\n" + `{"p":`,
			wantRest: `{"p":{"printf":[{"posn":"a.go:1:2","message":"first"}]}}` + "\n" + `{"p":`,
			wantEOF:  true,
		},
		{
			name: "vet wrong diagnostic type", parse: VetFindings,
			input: `{"p":{"printf":"bad"}}`, wantRest: `{"p":{"printf":"bad"}}`, wantType: true,
		},
		{
			name: "staticcheck truncated after valid report",
			parse: func(s, dir string) ([]Finding, string, error) {
				f, err := StaticcheckFindings(s, dir)
				return f, "", err
			},
			input: `{"code":"SA1000","message":"first"}` + "\n" + `{"code":`,
		},
		{
			name: "staticcheck wrong code type",
			parse: func(s, dir string) ([]Finding, string, error) {
				f, err := StaticcheckFindings(s, dir)
				return f, "", err
			},
			input: `{"code":42}`, wantType: true,
		},
		{name: "golangci truncated", parse: GolangciFindings, input: `{"Issues":`, wantEOF: true},
		{name: "golangci wrong issues type", parse: GolangciFindings, input: `{"Issues":"bad"}`, wantType: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, rest, err := tt.parse(tt.input, t.TempDir())
			if err == nil {
				t.Fatal("malformed report was accepted")
			}
			if tt.wantEOF {
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Errorf("error = %v, want unexpected EOF", err)
				}
			} else if tt.wantType {
				var target *json.UnmarshalTypeError
				if !errors.As(err, &target) {
					t.Errorf("error = %v, want wrapped UnmarshalTypeError", err)
				}
			} else {
				var target *json.SyntaxError
				if !errors.As(err, &target) {
					t.Errorf("error = %v, want wrapped SyntaxError", err)
				}
			}
			if len(got) != 0 || rest != tt.wantRest {
				t.Errorf("got findings %+v, rest %q; want no findings, rest %q", got, rest, tt.wantRest)
			}
		})
	}
}

func TestVetFindings_ReadsMultiplePackagesAndPreservesUnparsedOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, rest string
		want              []Finding
	}{
		{name: "empty"},
		{name: "headers only", input: " \r\n# first\r\n# second\n"},
		{name: "unpositioned failure", input: "# p\r\n  tool could not load package\r\n", rest: "tool could not load package"},
		{
			name:  "multiple objects and analyzers",
			input: "# z\r\n" + `{"z":{"printf":[{"posn":"z.go:9:2","message":"last"}]}}` + "\n# a\n" + `{"a":{"printf":[{"posn":"a.go:1:3","message":"format"}],"copylocks":[{"posn":"a.go:1:3","message":"lock"}]}}` + "\ntrailing output\n",
			rest:  "trailing output",
			want: []Finding{
				{File: "a.go", Line: 1, Column: 3, Check: "copylocks", Message: "lock"},
				{File: "a.go", Line: 1, Column: 3, Check: "printf", Message: "format"},
				{File: "z.go", Line: 9, Column: 2, Check: "printf", Message: "last"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, rest, err := VetFindings(tt.input, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("VetFindings mismatch (-want +got):\n%s", diff)
			}
			if rest != tt.rest {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

func TestStaticcheckFindings_HandlesBlankLinesAndCompileFallback(t *testing.T) {
	t.Parallel()
	longMessage := strings.Repeat("x", 70*1024)
	tests := []struct {
		name, input string
		want        []Finding
	}{
		{name: "blank lines", input: " \r\n\t\n"},
		{
			name:  "compile without positioned text",
			input: `{"code":"compile","location":{"file":"a.go","line":4,"column":2},"message":"could not import dependency"}`,
			want:  []Finding{{File: "a.go", Line: 4, Column: 2, Check: CompileCheck, Message: "could not import dependency"}},
		},
		{
			name:  "multiple compile positions sorted",
			input: `{"code":"compile","message":"# p\n./z.go:9:4: undefined: z\nvet.exe: ./a.go:2:1: undefined: a"}`,
			want: []Finding{
				{File: "a.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: a"},
				{File: "z.go", Line: 9, Column: 4, Check: CompileCheck, Message: "undefined: z"},
			},
		},
		{
			name:  "record exceeds default scanner limit",
			input: `{"code":"SA1000","location":{"file":"a.go","line":1,"column":2},"message":"` + longMessage + `"}` + "\n",
			want:  []Finding{{File: "a.go", Line: 1, Column: 2, Check: "SA1000", Message: longMessage}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := StaticcheckFindings(tt.input, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("StaticcheckFindings mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGolangciFindings_ExpandsTypecheckAndRetainsSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, rest string
		want              []Finding
	}{
		{name: "clean report", input: `{"Issues":[]}`},
		{
			name:  "positioned typecheck with summary",
			input: `{"Issues":[{"FromLinter":"typecheck","Text":"# p\n./z.go:9:4: undefined: z\n./a.go:2:1: undefined: a","Pos":{"Filename":"ignored.go","Line":99}}]}` + "\n  2 issues\nsummary\n",
			rest:  "2 issues\nsummary",
			want: []Finding{
				{File: "a.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: a"},
				{File: "z.go", Line: 9, Column: 4, Check: CompileCheck, Message: "undefined: z"},
			},
		},
		{
			name:  "unpositioned typecheck preserves issue location",
			input: `{"Issues":[{"FromLinter":"typecheck","Text":"could not import dependency","Pos":{"Filename":"a.go","Line":3,"Column":4}}]}`,
			want:  []Finding{{File: "a.go", Line: 3, Column: 4, Check: "typecheck", Message: "could not import dependency"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, rest, err := GolangciFindings(tt.input, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GolangciFindings mismatch (-want +got):\n%s", diff)
			}
			if rest != tt.rest {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

func TestPath_KeepsOutsidePathsAndNormalizesInsidePaths(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "ws")
	outside := filepath.Join(filepath.Dir(dir), "ws-other", "a.go")
	tests := []struct{ name, path, want string }{
		{name: "empty"},
		{name: "relative", path: filepath.Join("a", "a.go"), want: "a/a.go"},
		{name: "absolute inside", path: filepath.Join(dir, "a", "a.go"), want: "a/a.go"},
		{name: "clean inside", path: "./a/../b.go", want: "b.go"},
		{name: "parent itself", path: "..", want: ".."},
		{name: "relative outside", path: filepath.Join("..", "a.go"), want: "../a.go"},
		{name: "sibling with shared prefix", path: outside, want: filepath.ToSlash(outside)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Path(tt.path, dir); got != tt.want {
				t.Errorf("Path(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitPosition_PreservesDriveColonAndInvalidSuffix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, file string
		line, col         int
	}{
		{name: "empty"},
		{name: "no position", input: "a.go", file: "a.go"},
		{name: "position", input: "a.go:12:7", file: "a.go", line: 12, col: 7},
		{name: "windows drive", input: `C:\src\a.go:12:7`, file: `C:\src\a.go`, line: 12, col: 7},
		{name: "invalid column", input: "a.go:12:bad", file: "a.go:12:bad"},
		{name: "invalid line", input: "a.go:bad:7", file: "a.go:bad", col: 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, line, col := splitPosition(tt.input)
			if file != tt.file || line != tt.line || col != tt.col {
				t.Errorf("splitPosition(%q) = (%q, %d, %d), want (%q, %d, %d)", tt.input, file, line, col, tt.file, tt.line, tt.col)
			}
		})
	}
}

func TestSort_OrdersLocationAndCheckWhilePreservingTies(t *testing.T) {
	t.Parallel()
	want := []Finding{
		{File: "a.go", Line: 1, Column: 1, Check: "a", Message: "first tie"},
		{File: "a.go", Line: 1, Column: 1, Check: "a", Message: "second tie"},
		{File: "a.go", Line: 1, Column: 1, Check: "z"},
		{File: "a.go", Line: 1, Column: 2, Check: "a"},
		{File: "a.go", Line: 2, Column: 1, Check: "a"},
		{File: "b.go", Line: 1, Column: 1, Check: "a"},
	}
	got := []Finding{want[5], want[4], want[3], want[2], want[0], want[1]}
	Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Sort mismatch (-want +got):\n%s", diff)
	}
	Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("second Sort mismatch (-want +got):\n%s", diff)
	}
	var empty []Finding
	Sort(empty)
	if empty != nil {
		t.Errorf("Sort(nil) = %v, want nil", empty)
	}
}
