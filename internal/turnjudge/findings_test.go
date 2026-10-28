package turnjudge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestCheckFindings_JudgesCitedCodeAndSkipsUncited(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\nfunc Get(m map[string]int, k string) int {\n\treturn m[k]\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c, seen := fakeJev(t, map[string]float64{"finding_0": 0.12})
	verdicts, err := CheckFindings(context.Background(), c, dir, []Finding{
		{File: "x.go", Line: 4, Message: "nil pointer dereference on m"},
		{File: "x.go", Message: "no line: not judged"},
		{File: "missing.go", Line: 3, Message: "unreadable: not judged"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(verdicts) != 1 || verdicts[0].Probability != 0.12 {
		t.Fatalf("verdicts = %+v, want the one cited finding at 0.12", verdicts)
	}
	q := (*seen)["questions"].(map[string]any)["finding_0"].(map[string]any)
	code := q["instructions"].(map[string]any)["code"].(string)
	if !strings.Contains(code, "4: \treturn m[k]") {
		t.Fatalf("the cited line is not numbered in the code sent:\n%s", code)
	}
	facts := FindingFacts("rev-1", verdicts)
	if len(facts) != 1 || types.ExtractString(facts[0].Args[0]) != "rev-1" {
		t.Fatalf("facts = %v", facts)
	}
}
