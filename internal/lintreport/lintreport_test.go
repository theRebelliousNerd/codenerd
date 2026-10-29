package lintreport

import (
	"path/filepath"
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
