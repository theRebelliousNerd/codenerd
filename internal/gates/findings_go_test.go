package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func r7bModule(t *testing.T) string {
	t.Helper()
	return moduleAt(t, "example.com/r7b")
}

func moduleAt(t *testing.T, module string) string {
	t.Helper()
	root := t.TempDir()
	body := "module " + module + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func readTD(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func capturedLine(t *testing.T, file string, want string) string {
	t.Helper()
	text := strings.ReplaceAll(readTD(t, file), "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("%s has no line containing %q", file, want)
	return ""
}

func assertTargets(t *testing.T, fs []Finding, want []string) {
	t.Helper()
	got := targets(fs)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, f := range fs {
		if strings.Contains(f.Target, `\`) || strings.Contains(f.Target, "Program Files") || strings.Contains(f.Target, "testing.go") {
			t.Fatalf("a stack or compiler path leaked into the target: %+v", f)
		}
		if strings.HasSuffix(f.Target, "ok_test.go") {
			t.Fatalf("indented test log became a finding: %+v", f)
		}
		if strings.HasPrefix(f.Message, "{") || f.Message == "FAIL" {
			t.Fatalf("unparsed trailer became the finding: %+v", f)
		}
	}
}

func goGate(id string, kind Kind) Gate {
	return Gate{ID: id, Kind: kind}
}

// go build ./... on the throwaway module. The compiler line is
// "compileerr\broken.go:3:28: undefined: Missing" under
// "# example.com/r7b/compileerr". That file is the finding.
func TestFindings_CapturedGoBuild(t *testing.T) {
	fs := Findings(r7bModule(t), failed(goGate("go:build", Build), "", readTD(t, "go_build.txt")))
	assertTargets(t, fs, []string{"compileerr/broken.go"})
	if !strings.Contains(fs[0].Message, "undefined: Missing") {
		t.Fatalf("message = %q", fs[0].Message)
	}
}

// go vet ./... prefixes Windows diagnostics with "vet.exe: ".
// "vet.exe: compileerr\broken.go:3:28: undefined: Missing" did not match a
// file:line pattern, and the printf line in testsfail kept the run from
// falling back to one last-line finding, so the vet.exe lines were dropped.
func TestFindings_CapturedGoVet(t *testing.T) {
	fs := Findings(r7bModule(t), failed(goGate("go:vet", Lint), "", readTD(t, "go_vet.txt")))
	assertTargets(t, fs, []string{
		"compileerr/broken.go",
		"testbuild/bad_test.go",
		"testsfail/ok.go",
	})
	if !strings.Contains(fs[0].Message, "undefined: Missing") || !strings.Contains(fs[1].Message, "undefined: NotASymbol") {
		t.Fatalf("messages = %q | %q", fs[0].Message, fs[1].Message)
	}
	if !strings.Contains(fs[2].Message, "wrong type") {
		t.Fatalf("printf diagnostic = %q", fs[2].Message)
	}
}

// go test (text, -vet=off) of the throwaway module. The same TestSame fails
// in samea and sameb; TestPanics prints "panic: boom [recovered, repanicked]"
// and a stack whose frames are indented; packages that fail to build print
// "FAIL\tpkg [build failed]" after the compiler line.
func TestFindings_CapturedGoTest(t *testing.T) {
	fs := Findings(r7bModule(t), failed(goGate("go:test", Test), "", readTD(t, "go_test.txt")))
	assertTargets(t, fs, []string{
		"compileerr/broken.go",
		"testbuild/bad_test.go",
		".::TestRoot",
		"samea::TestSame",
		"sameb::TestSame",
		"testsfail::TestFails",
		"testsfail::TestPanics",
	})
	panics, ok := findingByTarget(fs, "testsfail::TestPanics")
	if !ok || !strings.Contains(panics.Message, "panic: boom") {
		t.Fatalf("TestPanics = %+v", panics)
	}
	fails, ok := findingByTarget(fs, "testsfail::TestFails")
	if !ok || fails.Message != "test failed: TestFails" {
		t.Fatalf("TestFails = %+v", fails)
	}
	a, _ := findingByTarget(fs, "samea::TestSame")
	b, _ := findingByTarget(fs, "sameb::TestSame")
	if a.ID == "" || a.ID == b.ID {
		t.Fatalf("TestSame collapsed: %+v %+v", a, b)
	}
	for _, f := range fs {
		if strings.HasPrefix(f.Message, "build failed") || f.Target == ".::TestSame" || strings.HasPrefix(f.Target, UnattributedTarget) {
			t.Fatalf("build failure double-counted or test left unlocated: %+v", f)
		}
	}
}

// go test -json. Build failures arrive as build-output / build-fail events
// ("compileerr\\broken.go:3:28: undefined: Missing" inside Output). Test
// failures are fail events; the package fail after them is the same break.
// Event order in the capture is sameb, the root package, samea, then testsfail.
func TestFindings_CapturedGoTestJSON(t *testing.T) {
	fs := Findings(r7bModule(t), failed(goGate("go:test", Test), "", readTD(t, "go_test_json.txt")))
	assertTargets(t, fs, []string{
		"compileerr/broken.go",
		"testbuild/bad_test.go",
		"sameb::TestSame",
		".::TestRoot",
		"samea::TestSame",
		"testsfail::TestFails",
		"testsfail::TestPanics",
	})
	panics, ok := findingByTarget(fs, "testsfail::TestPanics")
	if !ok || !strings.Contains(panics.Message, "panic: boom") {
		t.Fatalf("TestPanics = %+v", panics)
	}
	a, _ := findingByTarget(fs, "samea::TestSame")
	b, _ := findingByTarget(fs, "sameb::TestSame")
	if a.ID == "" || a.ID == b.ID {
		t.Fatalf("TestSame collapsed: %+v %+v", a, b)
	}
}

// "panic: initboom" then "FAIL\texample.com/r7b/initpanic\t0.177s", with no
// "--- FAIL:" line. The package is the finding; the tab-indented frame
// "C:/.../initpanic/p.go:3 +0x25" is not.
func TestFindings_CapturedInitPanic(t *testing.T) {
	for _, name := range []string{"go_test_initpanic.txt", "go_test_initpanic_json.txt"} {
		t.Run(name, func(t *testing.T) {
			fs := Findings(r7bModule(t), failed(goGate("go:test", Test), "", readTD(t, name)))
			if len(fs) != 1 || fs[0].Target != "initpanic" || !strings.Contains(fs[0].Message, "panic: initboom") {
				t.Fatalf("%+v", fs)
			}
			if strings.Contains(fs[0].Target, "p.go") {
				t.Fatalf("stack frame became the target: %+v", fs[0])
			}
		})
	}
}

// An init panic after other packages' failures used to vanish: the run
// already had findings, so the last-line fallback never ran, and nothing
// else read "panic: initboom".
func TestFindings_InitPanicSurvivesEarlierFailures(t *testing.T) {
	text := readTD(t, "go_test.txt")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	fs := Findings(r7bModule(t), failed(goGate("go:test", Test), "", text+readTD(t, "go_test_initpanic.txt")))
	assertTargets(t, fs, []string{
		"compileerr/broken.go",
		"testbuild/bad_test.go",
		".::TestRoot",
		"samea::TestSame",
		"sameb::TestSame",
		"testsfail::TestFails",
		"testsfail::TestPanics",
		"initpanic",
	})
	init, ok := findingByTarget(fs, "initpanic")
	if !ok || !strings.Contains(init.Message, "panic: initboom") {
		t.Fatalf("init = %+v", init)
	}
	panics, ok := findingByTarget(fs, "testsfail::TestPanics")
	if !ok || !strings.Contains(panics.Message, "panic: boom") {
		t.Fatalf("TestPanics lost its panic: %+v", panics)
	}

	js := readTD(t, "go_test_json.txt")
	if !strings.HasSuffix(js, "\n") {
		js += "\n"
	}
	fs = Findings(r7bModule(t), failed(goGate("go:test", Test), "", js+readTD(t, "go_test_initpanic_json.txt")))
	assertTargets(t, fs, []string{
		"compileerr/broken.go",
		"testbuild/bad_test.go",
		"sameb::TestSame",
		".::TestRoot",
		"samea::TestSame",
		"testsfail::TestFails",
		"testsfail::TestPanics",
		"initpanic",
	})
}

// "FAIL\texample.com/r7b/compileerr [build failed]" names a package and no
// file. The directory comes from the module line. A different module, or
// no module, keeps the import path rather than a guessed directory.
func TestFindings_BuildFailedLineKeepsThePackage(t *testing.T) {
	line := capturedLine(t, "go_test.txt", "example.com/r7b/compileerr [build failed]")
	if !strings.Contains(line, "\t") {
		t.Fatalf("captured line lost its tab: %q", line)
	}
	fs := Findings(r7bModule(t), failed(goGate("go:test", Test), "", line+"\n"))
	if len(fs) != 1 || fs[0].Target != "compileerr" || !strings.Contains(fs[0].Message, "build failed") {
		t.Fatalf("with the module: %+v", fs)
	}

	fs = Findings(moduleAt(t, "example.com/other"), failed(goGate("go:test", Test), "", line+"\n"))
	if len(fs) != 1 || fs[0].Target != "example.com/r7b/compileerr" {
		t.Fatalf("other module: %+v", fs)
	}
	fs = Findings(t.TempDir(), failed(goGate("go:test", Test), "", line+"\n"))
	if len(fs) != 1 || fs[0].Target != "example.com/r7b/compileerr" {
		t.Fatalf("no module: %+v", fs)
	}

	// The module prefix is exact: example.com/r7b does not own example.com/r7b-extra.
	extra := "FAIL\texample.com/r7b-extra/pkg\t0.1s\n"
	fs = Findings(r7bModule(t), failed(goGate("go:test", Test), "", extra))
	if len(fs) != 1 || fs[0].Target != "example.com/r7b-extra/pkg" {
		t.Fatalf("prefix: %+v", fs)
	}
}

func findingByTarget(fs []Finding, target string) (Finding, bool) {
	for _, f := range fs {
		if f.Target == target {
			return f, true
		}
	}
	return Finding{}, false
}
