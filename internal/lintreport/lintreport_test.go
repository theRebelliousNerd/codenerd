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
	emptyVet, _, err := VetFindings("", "")
	if err != nil {
		t.Fatalf("VetFindings empty err = %v", err)
	}
	Sort(emptyVet)
	if len(emptyVet) != 0 {
		t.Errorf("Sort(empty) = %v, want empty", emptyVet)
	}
}

func TestCompileFindings_HandlesCRLFWhitespaceVetPrefixAndSorts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	text := "  vet: ./b/b.go:4:2:   undefined: x   \r\n\r\nnot a finding\r\nvet.exe: ./a/a.go:2:1: undefined: a\r\n"
	want := []Finding{
		{File: "a/a.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: a"},
		{File: "b/b.go", Line: 4, Column: 2, Check: CompileCheck, Message: "undefined: x"},
	}
	if diff := cmp.Diff(want, CompileFindings(text, dir)); diff != "" {
		t.Errorf("CompileFindings mismatch (-want +got):\n%s", diff)
	}
	if got := CompileFindings("", dir); len(got) != 0 {
		t.Errorf("CompileFindings empty = %+v, want empty", got)
	}
}

func TestVetFindings_StripsBOMAndLeadingWhitespace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bom := "\xef\xbb\xbf"
	input := bom + "  \n" + `{"p":{"printf":[{"posn":"a.go:1:2","message":"first"}]}}` + "\nvet: ./b.go:2:1: undefined: y\n"
	want := []Finding{
		{File: "a.go", Line: 1, Column: 2, Check: "printf", Message: "first"},
		{File: "b.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: y"},
	}
	got, rest, err := VetFindings(input, dir)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("VetFindings mismatch (-want +got):\n%s", diff)
	}
	if rest != "" {
		t.Errorf("rest = %q, want %q", rest, "")
	}
}

func TestStaticcheckFindings_StripsBOMAndSurroundingWhitespace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bom := "\xef\xbb\xbf"
	input := bom + "\n  " + `{"code":"SA1000","location":{"file":"a.go","line":1,"column":2},"message":"msg"}` + "  \n\n"
	want := []Finding{{File: "a.go", Line: 1, Column: 2, Check: "SA1000", Message: "msg"}}
	got, err := StaticcheckFindings(input, dir)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("StaticcheckFindings mismatch (-want +got):\n%s", diff)
	}
}

func TestGolangciFindings_StripsBOMAndTrimsWhitespace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bom := "\xef\xbb\xbf"
	input := bom + "  \n" + `{"Issues":[]}` + "  \n  trailing summary\n"
	got, rest, err := GolangciFindings(input, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("GolangciFindings = %+v, want empty", got)
	}
	if rest != "trailing summary" {
		t.Errorf("rest = %q, want %q", rest, "trailing summary")
	}
}

func TestPath_EmptyDirPreservesSlash(t *testing.T) {
	t.Parallel()
	if got := Path(filepath.Join("a", "b.go"), ""); got != "a/b.go" {
		t.Errorf("Path with empty dir = %q, want %q", got, "a/b.go")
	}
	if got := Path("", filepath.Join("x", "y")); got != "" {
		t.Errorf("Path empty path = %q, want %q", got, "")
	}
}

func TestSplitPosition_TrimsWhitespaceAndRejectsNegative(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, input, file string
		line, col         int
	}{
		{name: "surrounding spaces", input: "  a.go:12:7  ", file: "a.go", line: 12, col: 7},
		{name: "spaces around numbers", input: "a.go: 12 : 7", file: "a.go", line: 12, col: 7},
		{name: "negative column kept whole", input: "a.go:12:-1", file: "a.go:12:-1"},
		{name: "negative line keeps file", input: "a.go:-2:7", file: "a.go:-2", col: 7},
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

func TestSort_SingletonUnchanged(t *testing.T) {
	t.Parallel()
	want := []Finding{{File: "a.go", Line: 1, Column: 1, Check: "a", Message: "only"}}
	got := []Finding{{File: "a.go", Line: 1, Column: 1, Check: "a", Message: "only"}}
	Sort(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Sort singleton mismatch (-want +got):\n%s", diff)
	}
}

func TestPath_PinsRelativizationAgainstRevert(t *testing.T) {
	if got := Path("", "/work"); got != "" {
		t.Fatalf("Path empty path = %q, want empty", got)
	}
	if got := Path("a/b.go", ""); got != "a/b.go" {
		t.Fatalf("Path empty dir = %q, want %q", got, "a/b.go")
	}
	if got := Path("/work/sub/a.go", "/work"); got != "sub/a.go" {
		t.Fatalf("Path abs inside = %q, want %q", got, "sub/a.go")
	}
	if got := Path("/other/a.go", "/work"); got != "/other/a.go" {
		t.Fatalf("Path abs outside = %q, want preserved", got)
	}
	if got := Path("./a.go", "/work"); got != "a.go" {
		t.Fatalf("Path dot-relative inside = %q, want %q", got, "a.go")
	}
	if got := Path("/work/a.go", "/work"); got != "a.go" {
		t.Fatalf("Path abs file directly inside = %q, want %q", got, "a.go")
	}
}

func TestSort_PinsFullOrderingAgainstRevert(t *testing.T) {
	in := []Finding{
		{File: "b.go", Line: 10, Column: 1, Check: "a", Message: "m"},
		{File: "a.go", Line: 20, Column: 1, Check: "a", Message: "m"},
		{File: "a.go", Line: 10, Column: 5, Check: "b", Message: "m"},
		{File: "a.go", Line: 10, Column: 5, Check: "a", Message: "m"},
		{File: "a.go", Line: 10, Column: 1, Check: "z", Message: "m"},
	}
	Sort(in)
	want := []Finding{
		{File: "a.go", Line: 10, Column: 1, Check: "z", Message: "m"},
		{File: "a.go", Line: 10, Column: 5, Check: "a", Message: "m"},
		{File: "a.go", Line: 10, Column: 5, Check: "b", Message: "m"},
		{File: "a.go", Line: 20, Column: 1, Check: "a", Message: "m"},
		{File: "b.go", Line: 10, Column: 1, Check: "a", Message: "m"},
	}
	if len(in) != len(want) {
		t.Fatalf("Sort len = %d, want %d", len(in), len(want))
	}
	for i := range want {
		if in[i] != want[i] {
			t.Fatalf("Sort[%d] = %+v, want %+v (full slice %+v)", i, in[i], want[i], in)
		}
	}
}

func TestVetFindings_SkipsBlankLinesWhenCompilePresent(t *testing.T) {
	output := "vet: ./a.go:3:5: undefined: x\nsome note\n"
	findings, rest, err := VetFindings(output, "")
	if err != nil {
		t.Fatalf("VetFindings err = %v", err)
	}
	if len(findings) != 1 || findings[0].Check != CompileCheck {
		t.Fatalf("findings = %+v, want one compile finding", findings)
	}
	if rest != "some note" {
		t.Fatalf("rest = %q, want %q", rest, "some note")
	}
}

func TestStaticcheckFindings_ScanErrorBranchDefensive(t *testing.T) {
	hugeMessage := strings.Repeat("x", 128*1024)
	line := `{"code":"SA1000","location":{"file":"a.go","line":1,"column":1},"message":"` + hugeMessage + `"}`
	findings, err := StaticcheckFindings(line+"\n", "/tmp")
	if err != nil {
		t.Fatalf("StaticcheckFindings with oversized token returned error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for oversized token, got %d", len(findings))
	}
	if findings[0].Check != "SA1000" {
		t.Fatalf("expected check SA1000, got %q", findings[0].Check)
	}
}

func TestVetFindings_KeepsNonCompileLinesWhenCompileFindingsExist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got, rest, err := VetFindings("tool could not load package\nvet: ./a.go:3:2: undefined: x\n", dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{{File: "a.go", Line: 3, Column: 2, Check: CompileCheck, Message: "undefined: x"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("VetFindings mismatch (-want +got):\n%s", diff)
	}
	if rest != "tool could not load package" {
		t.Errorf("rest = %q, want %q", rest, "tool could not load package")
	}

	mixed := `{"p":{"printf":[{"posn":"a.go:1:2","message":"first"}]}}` + "\ntool could not load package\nvet: ./b.go:2:1: undefined: y\n"
	got, rest, err = VetFindings(mixed, dir)
	if err != nil {
		t.Fatal(err)
	}
	want = []Finding{
		{File: "a.go", Line: 1, Column: 2, Check: "printf", Message: "first"},
		{File: "b.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: y"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("VetFindings mixed mismatch (-want +got):\n%s", diff)
	}
	if rest != "tool could not load package" {
		t.Errorf("rest = %q, want %q", rest, "tool could not load package")
	}
}

func TestCompileFindings_UsesPathAndSort(t *testing.T) {
	text := "/work/b.go:10:1: second\n/work/a.go:5:2: first\n"
	got := CompileFindings(text, "/work")
	if len(got) != 2 {
		t.Fatalf("CompileFindings len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].File != "a.go" || got[1].File != "b.go" {
		t.Fatalf("CompileFindings files = %q,%q want a.go,b.go (%+v)", got[0].File, got[1].File, got)
	}
}

func TestVetFindings_UsesPathAndSort(t *testing.T) {
	output := `{"example.com/pkg": {"printf": [{"posn": "/work/b.go:10:1", "message": "second"}, {"posn": "/work/a.go:5:2", "message": "first"}]}}`
	got, _, err := VetFindings(output, "/work")
	if err != nil {
		t.Fatalf("VetFindings err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("VetFindings len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].File != "a.go" || got[1].File != "b.go" {
		t.Fatalf("VetFindings files = %q,%q want a.go,b.go (%+v)", got[0].File, got[1].File, got)
	}
}

func TestStaticcheckFindings_UsesPathAndSort(t *testing.T) {
	stdout := "{\"code\":\"SA5009\",\"location\":{\"file\":\"/work/b.go\",\"line\":10,\"column\":1},\"message\":\"second\"}\n{\"code\":\"SA5009\",\"location\":{\"file\":\"/work/a.go\",\"line\":5,\"column\":2},\"message\":\"first\"}\n"
	got, err := StaticcheckFindings(stdout, "/work")
	if err != nil {
		t.Fatalf("StaticcheckFindings err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("StaticcheckFindings len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].File != "a.go" || got[1].File != "b.go" {
		t.Fatalf("StaticcheckFindings files = %q,%q want a.go,b.go (%+v)", got[0].File, got[1].File, got)
	}
}

func TestGolangciFindings_UsesPathAndSort(t *testing.T) {
	stdout := `{"Issues": [{"FromLinter": "errcheck", "Text": "second", "Pos": {"Filename": "/work/b.go", "Line": 10, "Column": 1}}, {"FromLinter": "errcheck", "Text": "first", "Pos": {"Filename": "/work/a.go", "Line": 5, "Column": 2}}]}`
	got, _, err := GolangciFindings(stdout, "/work")
	if err != nil {
		t.Fatalf("GolangciFindings err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GolangciFindings len = %d, want 2 (%+v)", len(got), got)
	}
	if got[0].File != "a.go" || got[1].File != "b.go" {
		t.Fatalf("GolangciFindings files = %q,%q want a.go,b.go (%+v)", got[0].File, got[1].File, got)
	}
}

func TestPath_PreservesParentWhenRelIsDotDot(t *testing.T) {
	if got := Path("/a", "/a/b"); got != "/a" {
		t.Fatalf("Path parent = %q, want %q", got, "/a")
	}
	if got := Path("", ""); got != "" {
		t.Fatalf("Path empty,empty = %q, want empty", got)
	}
}

func TestSort_IgnoresMessageAndKeepsStableOrder(t *testing.T) {
	in := []Finding{
		{File: "a.go", Line: 1, Column: 1, Check: "c", Message: "z"},
		{File: "a.go", Line: 1, Column: 1, Check: "c", Message: "a"},
		{File: "a.go", Line: 1, Column: 1, Check: "c", Message: "m"},
	}
	Sort(in)
	wantMsgs := []string{"z", "a", "m"}
	for i, w := range wantMsgs {
		if in[i].Message != w {
			t.Fatalf("Sort stable[%d] msg = %q, want %q (full %+v)", i, in[i].Message, w, in)
		}
	}
}

func TestStabilize_PinsCompileFindingsVetPrefixAndWhitespace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	text := "  vet: ./b/b.go:4:2:   undefined: x   \r\nnot a finding\r\nvet.exe: ./a/a.go:2:1: undefined: a\r\n"
	want := []Finding{
		{File: filepath.ToSlash(filepath.Join("a", "a.go")), Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: a"},
		{File: filepath.ToSlash(filepath.Join("b", "b.go")), Line: 4, Column: 2, Check: CompileCheck, Message: "undefined: x"},
	}
	got := CompileFindings(text, dir)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("CompileFindings stabilize mismatch (-want +got):\n%s", diff)
	}
	if got := CompileFindings("not a finding\n   \r\n", dir); len(got) != 0 {
		t.Fatalf("CompileFindings ignored lines = %+v, want empty", got)
	}
}

func TestStabilize_PinsVetBOMRemainderAndCRLF(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bom := "\xef\xbb\xbf"
	input := bom + "  \r\n" + `{"p":{"printf":[{"posn":"a.go:1:2","message":"first"}]}}` + "\r\n# header line\r\nvet: ./b.go:2:1: undefined: y\r\ntrailing note\r\n"
	want := []Finding{
		{File: "a.go", Line: 1, Column: 2, Check: "printf", Message: "first"},
		{File: "b.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: y"},
	}
	got, rest, err := VetFindings(input, dir)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("VetFindings stabilize mismatch (-want +got):\n%s", diff)
	}
	if rest != "trailing note" {
		t.Fatalf("VetFindings rest = %q, want %q", rest, "trailing note")
	}
}

func TestStabilize_PinsPathSplitPositionEdges(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if got := Path("", dir); got != "" {
		t.Fatalf("Path empty = %q, want empty", got)
	}
	if got := Path(filepath.Join("a", "b.go"), ""); got != "a/b.go" {
		t.Fatalf("Path empty dir = %q, want %q", got, "a/b.go")
	}
	if got := Path(dir, dir); got != "." {
		t.Fatalf("Path self = %q, want %q", got, ".")
	}
	inside := filepath.Join(dir, "sub", "a.go")
	if got := Path(inside, dir); got != filepath.ToSlash(filepath.Join("sub", "a.go")) {
		t.Fatalf("Path inside = %q, want relativized", got)
	}
	if got := Path(filepath.Join(".", "a.go"), dir); got != "a.go" {
		t.Fatalf("Path dot-relative = %q, want %q", got, "a.go")
	}
	for _, tc := range []struct {
		in, file  string
		line, col int
	}{
		{"  a.go:12:7  ", "a.go", 12, 7},
		{"a.go: 12 : 7", "a.go", 12, 7},
		{"a.go:12:-1", "a.go:12:-1", 0, 0},
		{"a.go:-2:7", "a.go:-2", 0, 7},
		{"a.go:12:bad", "a.go:12:bad", 0, 0},
		{"", "", 0, 0},
	} {
		file, line, col := splitPosition(tc.in)
		if file != tc.file || line != tc.line || col != tc.col {
			t.Fatalf("splitPosition(%q) = (%q,%d,%d), want (%q,%d,%d)", tc.in, file, line, col, tc.file, tc.line, tc.col)
		}
	}
}

func TestStabilize_PinsStaticcheckGolangciSortAndScanner(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bom := "\xef\xbb\xbf"
	sc := bom + `{"code":"SA5009","location":{"file":"b.go","line":10,"column":1},"message":"second"}` + "\n" + `{"code":"SA5009","location":{"file":"a.go","line":5,"column":2},"message":"first"}` + "\n"
	got, err := StaticcheckFindings(sc, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].File != "a.go" || got[1].File != "b.go" {
		t.Fatalf("StaticcheckFindings sort = %+v, want a.go then b.go", got)
	}
	longMessage := strings.Repeat("x", 70*1024)
	got, err = StaticcheckFindings(`{"code":"SA1000","location":{"file":"a.go","line":1,"column":2},"message":"`+longMessage+`"}`+"\n", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Message != longMessage {
		t.Fatalf("StaticcheckFindings long record len = %d, want 1 with full message", len(got))
	}
	gl := `{"Issues":[{"FromLinter":"typecheck","Text":"# p\n./z.go:9:4: undefined: z\n./a.go:2:1: undefined: a","Pos":{"Filename":"ignored.go","Line":99}}]}` + "\nsummary line\n"
	got2, rest, err := GolangciFindings(gl, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{
		{File: "a.go", Line: 2, Column: 1, Check: CompileCheck, Message: "undefined: a"},
		{File: "z.go", Line: 9, Column: 4, Check: CompileCheck, Message: "undefined: z"},
	}
	if diff := cmp.Diff(want, got2); diff != "" {
		t.Fatalf("GolangciFindings stabilize mismatch (-want +got):\n%s", diff)
	}
	if rest != "summary line" {
		t.Fatalf("GolangciFindings rest = %q, want %q", rest, "summary line")
	}
	in := []Finding{
		{File: "b.go", Line: 10, Column: 1, Check: "a"},
		{File: "a.go", Line: 20, Column: 1, Check: "a"},
		{File: "a.go", Line: 10, Column: 5, Check: "b"},
		{File: "a.go", Line: 10, Column: 5, Check: "a"},
	}
	Sort(in)
	if in[0].File != "a.go" || in[0].Line != 10 || in[0].Column != 5 || in[0].Check != "a" {
		t.Fatalf("Sort stabilize first = %+v, want a.go:10:5:a (%+v)", in[0], in)
	}
	if in[len(in)-1].File != "b.go" {
		t.Fatalf("Sort stabilize last = %+v, want b.go (%+v)", in[len(in)-1], in)
	}
}

func TestVetFindings_PinsBOMHeadersCRLFRemainderAndSort(t *testing.T) {
	dir := "/work"
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	output := bom + "   \n# example.com/pkg\n{\"example.com/pkg\":{\"printf\":[{\"posn\":\"/work/z.go:10:2\",\"message\":\"bad verb\"}]}}\nvet: ./a.go:3:23: undefined: x\nnote: something else\n"
	findings, rest, err := VetFindings(output, dir)
	if err != nil {
		t.Fatalf("VetFindings returned error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("VetFindings returned %d findings, want 2: %#v", len(findings), findings)
	}
	if findings[0].File != "a.go" || findings[0].Line != 3 || findings[0].Column != 23 || findings[0].Check != CompileCheck {
		t.Fatalf("first finding not sorted compile a.go:3:23: %#v", findings[0])
	}
	if findings[0].Message != "undefined: x" {
		t.Fatalf("compile message = %q, want %q", findings[0].Message, "undefined: x")
	}
	if findings[1].File != "z.go" || findings[1].Line != 10 || findings[1].Column != 2 || findings[1].Check != "printf" {
		t.Fatalf("second finding not printf z.go:10:2: %#v", findings[1])
	}
	if findings[1].Message != "bad verb" {
		t.Fatalf("printf message = %q, want %q", findings[1].Message, "bad verb")
	}
	if rest != "note: something else" {
		t.Fatalf("rest = %q, want %q", rest, "note: something else")
	}
}

func TestVetFindings_PinsErrorRestPreservesOutput(t *testing.T) {
	bad := "{ not json"
	findings, rest, err := VetFindings(bad, "")
	if err == nil {
		t.Fatalf("VetFindings(%q) returned nil error", bad)
	}
	if len(findings) != 0 {
		t.Fatalf("VetFindings error findings = %#v, want empty", findings)
	}
	if rest != bad {
		t.Fatalf("VetFindings error rest = %q, want original %q", rest, bad)
	}
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	withBOM := bom + bad
	findings, rest, err = VetFindings(withBOM, "")
	if err == nil {
		t.Fatalf("VetFindings BOM input returned nil error")
	}
	if rest != bad {
		t.Fatalf("VetFindings BOM error rest = %q, want stripped %q", rest, bad)
	}
	valid := "{\"example.com/pkg\":{\"printf\":[{\"posn\":\"a.go:1:2\",\"message\":\"m\"}]}}"
	mixed := valid + "\n{ broken"
	findings, rest, err = VetFindings(mixed, "")
	if err == nil {
		t.Fatalf("VetFindings mixed valid+broken returned nil error")
	}
	if rest != mixed {
		t.Fatalf("VetFindings mixed error rest = %q, want full output %q", rest, mixed)
	}
}

func TestVetFindings_PinsSameLineBOMSpaceAndBraceBoundary(t *testing.T) {
	bom := string([]byte{0xEF, 0xBB, 0xBF})
	json1 := "{\"example.com/a\":{\"printf\":[{\"posn\":\"z.go:10:2\",\"message\":\"brace } inside\"}]}}"
	json2 := "{\"example.com/b\":{\"vet\":[{\"posn\":\"a.go:3:4\",\"message\":\"second\"}]}}"
	output := bom + "   " + json1 + "\n# example.com/b\n\n" + json2 + "\nvet: ./c.go:5:6: undefined: y\nnote: keep\n"
	findings, rest, err := VetFindings(output, "")
	if err != nil {
		t.Fatalf("VetFindings returned error: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("VetFindings returned %d findings, want 3: %#v", len(findings), findings)
	}
	if findings[0].File != "a.go" || findings[0].Check != "vet" {
		t.Fatalf("first finding want a.go vet, got %#v", findings[0])
	}
	if findings[1].File != "c.go" || findings[1].Check != CompileCheck {
		t.Fatalf("second finding want c.go compile, got %#v", findings[1])
	}
	if findings[1].Message != "undefined: y" {
		t.Fatalf("compile message = %q", findings[1].Message)
	}
	if findings[2].File != "z.go" || findings[2].Message != "brace } inside" {
		t.Fatalf("third finding want z.go brace message, got %#v", findings[2])
	}
	if rest != "note: keep" {
		t.Fatalf("rest = %q, want %q", rest, "note: keep")
	}
}

func TestVetFindings_PinsRemainderCRLFWithoutBOM(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bom := "\xef\xbb\xbf"
	jsonLine := `{"p":{"printf":[{"posn":"z.go:10:2","message":"brace } inside"}]}}`
	input := bom + "   " + jsonLine + "\r\n# header dropped\r\nvet: ./a.go:3:4: undefined: x\r\nnote one\r\nnote two\r\n"
	want := []Finding{
		{File: "a.go", Line: 3, Column: 4, Check: CompileCheck, Message: "undefined: x"},
		{File: "z.go", Line: 10, Column: 2, Check: "printf", Message: "brace } inside"},
	}
	got, rest, err := VetFindings(input, dir)
	if err != nil {
		t.Fatalf("VetFindings err = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("VetFindings mismatch (-want +got):\n%s", diff)
	}
	if rest != "note one\nnote two" {
		t.Fatalf("rest = %q, want %q", rest, "note one\nnote two")
	}
	if strings.Contains(rest, "\r") {
		t.Fatalf("rest contains CR: %q", rest)
	}
	if strings.Contains(rest, "\ufeff") {
		t.Fatalf("rest contains BOM: %q", rest)
	}
	if strings.Contains(rest, "# header") {
		t.Fatalf("rest contains header: %q", rest)
	}
}

func TestSplitPosition_PinsAgainstRevert(t *testing.T) {
	cases := []struct {
		in   string
		file string
		line int
		col  int
	}{
		{in: "a.go:1:2", file: "a.go", line: 1, col: 2},
		{in: "  a.go:1:2  ", file: "a.go", line: 1, col: 2},
		{in: "a.go: 10 : 20 ", file: "a.go", line: 10, col: 20},
		{in: "C:/x/y.go:3:4", file: "C:/x/y.go", line: 3, col: 4},
		{in: "a.go:1:-3", file: "a.go:1:-3", line: 0, col: 0},
		{in: "a.go:-1:2", file: "a.go:-1", line: 0, col: 2},
		{in: "a.go:foo:bar", file: "a.go:foo:bar", line: 0, col: 0},
		{in: "a.go", file: "a.go", line: 0, col: 0},
		{in: "", file: "", line: 0, col: 0},
		{in: "a.go:5", file: "a.go", line: 0, col: 5},
	}
	for _, tc := range cases {
		file, line, col := splitPosition(tc.in)
		if file != tc.file || line != tc.line || col != tc.col {
			t.Errorf("splitPosition(%q) = (%q,%d,%d), want (%q,%d,%d)", tc.in, file, line, col, tc.file, tc.line, tc.col)
		}
	}
}

func TestVetFindings_PinsSplitPositionThroughCaller(t *testing.T) {
	output := "{\"pkg\": {\"printf\": [{\"posn\": \"  a.go: 7 : 8 \", \"message\": \"m\"}, {\"posn\": \"C:/x/y.go:3:4\", \"message\": \"n\"}, {\"posn\": \"a.go:foo:bar\", \"message\": \"o\"}]}}"
	findings, _, err := VetFindings(output, "")
	if err != nil {
		t.Fatalf("VetFindings returned error: %v", err)
	}
	byMsg := map[string]Finding{}
	for _, f := range findings {
		byMsg[f.Message] = f
	}
	if f, ok := byMsg["m"]; !ok || f.File != "a.go" || f.Line != 7 || f.Column != 8 {
		t.Errorf("whitespace posn through VetFindings = %+v, want a.go:7:8", f)
	}
	if f, ok := byMsg["n"]; !ok || f.File != "C:/x/y.go" || f.Line != 3 || f.Column != 4 {
		t.Errorf("drive-colon posn through VetFindings = %+v, want C:/x/y.go:3:4", f)
	}
	if f, ok := byMsg["o"]; !ok || f.File != "a.go:foo:bar" || f.Line != 0 || f.Column != 0 {
		t.Errorf("invalid posn through VetFindings = %+v, want file kept with 0,0", f)
	}
}

func TestSplitPosition_PinsZeroLeadingColonAndExtraSegments(t *testing.T) {
	cases := []struct {
		in   string
		file string
		line int
		col  int
	}{
		{in: "a.go:0:0", file: "a.go", line: 0, col: 0},
		{in: "a.go:0:5", file: "a.go", line: 0, col: 5},
		{in: ":1:2", file: ":1", line: 0, col: 2},
		{in: "a.go:1:2:3", file: "a.go:1", line: 2, col: 3},
		{in: "a:b.go:1:2", file: "a:b.go", line: 1, col: 2},
		{in: "a.go::2", file: "a.go:", line: 0, col: 2},
		{in: "a.go : 1 : 2", file: "a.go", line: 1, col: 2},
		{in: "a.go:1:", file: "a.go:1:", line: 0, col: 0},
	}
	for _, tc := range cases {
		file, line, col := splitPosition(tc.in)
		if file != tc.file || line != tc.line || col != tc.col {
			t.Errorf("splitPosition(%q) = (%q,%d,%d), want (%q,%d,%d)", tc.in, file, line, col, tc.file, tc.line, tc.col)
		}
	}
}
