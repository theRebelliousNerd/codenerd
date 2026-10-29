package session

import (
	"context"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// R1-11 in miniature: the change adds two directions to one function, the
// turn's test exercises one of them, and taking the whole function out fails
// that test -- so the function-shaped question is answered while the second
// direction is pinned by nothing. Forcing the condition the test never
// distinguishes is the question that finds it.
const (
	coversBefore = "package pinprobe\n\nfunc Covers(dir, path string) bool { return dir == path }\n"
	coversAfter  = "package pinprobe\n\nimport \"strings\"\n\n" +
		"func Covers(dir, path string) bool {\n" +
		"\tif strings.HasPrefix(path, dir+\"/\") {\n" +
		"\t\treturn true\n" +
		"\t}\n" +
		"\tif strings.HasPrefix(dir, path+\"/\") {\n" +
		"\t\treturn true\n" +
		"\t}\n" +
		"\treturn dir == path\n}\n"
	coversTest = "package pinprobe\n\nimport \"testing\"\n\n" +
		"func TestCovers(t *testing.T) {\n" +
		"\tif !Covers(\"a\", \"a/b.go\") {\n\t\tt.Fatal(\"a covers a/b.go\")\n\t}\n" +
		"\tif Covers(\"a\", \"c\") {\n\t\tt.Fatal(\"a does not cover c\")\n\t}\n}\n"
	coversOtherDirectionTest = "\nfunc TestCoversTheOtherWay(t *testing.T) {\n" +
		"\tif !Covers(\"a/b\", \"a\") {\n\t\tt.Fatal(\"a/b is under a\")\n\t}\n}\n"
)

func TestVerifyPinning_ADirectionInsideAChangedFunctionIsRecordedNotCharged(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":         pinGoMod,
		"covers.go":      coversAfter,
		"covers_test.go": coversTest,
	})
	result := mutationResult()
	result.Intent.Verb = "/fix"
	result.WrittenPaths = []string{"covers.go", "covers_test.go"}
	result.PreWriteContents = map[string]PreImage{"covers.go": existed(coversBefore), "covers_test.go": {}}

	v := verifyPinning(context.Background(), ws, result, true)
	if v.Verdict() != VerifyPassed {
		t.Fatalf("verdict = %s (%s):\n%s\nwant passed: the function's own test fails without it, and a decision inside it is recorded, not charged",
			v.Verdict(), v.Reason, v.Output)
	}
	recorded := strings.Join(result.PinAdvisory, "\n")
	if !strings.Contains(recorded, "forced false") || !strings.Contains(recorded, "path+\"/\"") {
		t.Fatalf("the decision no test distinguishes was not recorded:\n%s", recorded)
	}

	writeWorkspaceFile(t, ws, "covers_test.go", coversTest+coversOtherDirectionTest)
	if v := verifyPinning(context.Background(), ws, result, true); v.Verdict() != VerifyPassed || len(result.PinAdvisory) != 0 {
		t.Fatalf("verdict = %s, recorded = %v; want passed with nothing left once a test takes the second direction", v.Verdict(), result.PinAdvisory)
	}
}

// The closure re-measures the gate without the advisory question: nothing
// reads its answer there, and it costs a test run for every decision.
func TestVerifyPinning_TheAdvisoryQuestionIsAskedOnlyWhenItIsWanted(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":         pinGoMod,
		"covers.go":      coversAfter,
		"covers_test.go": coversTest,
	})
	result := mutationResult()
	result.Intent.Verb = "/fix"
	result.WrittenPaths = []string{"covers.go", "covers_test.go"}
	result.PreWriteContents = map[string]PreImage{"covers.go": existed(coversBefore), "covers_test.go": {}}

	if v := verifyPinning(context.Background(), ws, result, false); v.Verdict() != VerifyPassed {
		t.Fatalf("verdict = %s (%s), want passed", v.Verdict(), v.Reason)
	}
	if len(result.PinAdvisory) != 0 {
		t.Errorf("the advisory question was asked anyway: %v", result.PinAdvisory)
	}
}

// Only the conditions on lines the turn changed are asked about, and a
// condition that is already a constant is not a decision.
func TestConditionUnits_AsksAboutTheChangedLinesOnly(t *testing.T) {
	before := "package p\n\nfunc F(a int) int {\n\tif a > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"
	after := "package p\n\nfunc F(a int) int {\n\tif a > 0 {\n\t\treturn 1\n\t}\n\tif a < -3 {\n\t\treturn -1\n\t}\n\tif true {\n\t\treturn 0\n\t}\n\treturn 0\n}\n"
	units := conditionUnits("p.go", after, existed(before))
	if len(units) != 2 {
		t.Fatalf("conditionUnits = %d units, want 2 (the new condition, both ways):\n%s", len(units), unitNames(units))
	}
	for _, u := range units {
		if !strings.Contains(u.name, "a < -3") {
			t.Errorf("unit %q is not about the condition the turn wrote", u.name)
		}
		if !u.forced {
			t.Errorf("unit %q is not marked as a forced condition", u.name)
		}
	}
	if got := unitNames(units); !strings.Contains(got, "forced true") || !strings.Contains(got, "forced false") {
		t.Errorf("units = %s, want the condition forced both ways", got)
	}
}

// A forced condition leaves the names its own init declared unused, which Go
// refuses to compile; the unit keeps them used so the question is about the
// condition and not about the compiler.
func TestConditionUnits_KeepsTheNamesTheInitDeclares(t *testing.T) {
	before := "package p\n\nfunc F(m map[string]int, k string) int {\n\treturn m[k]\n}\n"
	after := "package p\n\nfunc F(m map[string]int, k string) int {\n\tif v, ok := m[k]; ok && v > 0 {\n\t\treturn v\n\t}\n\treturn 0\n}\n"
	units := conditionUnits("p.go", after, existed(before))
	if len(units) != 2 {
		t.Fatalf("conditionUnits = %d units, want 2:\n%s", len(units), unitNames(units))
	}
	for _, u := range units {
		if _, err := parser.ParseFile(token.NewFileSet(), "", u.content, parser.SkipObjectResolution); err != nil {
			t.Fatalf("unit %q does not parse: %v\n%s", u.name, err, u.content)
		}
		for _, name := range []string{"_ = ok", "_ = v"} {
			if !strings.Contains(u.content, name) {
				t.Errorf("unit %q drops %q, so the mutant would not compile:\n%s", u.name, name, u.content)
			}
		}
	}
}

func unitNames(units []pinUnit) string {
	var b strings.Builder
	for _, u := range units {
		b.WriteString("  " + u.label() + "\n")
	}
	return b.String()
}
