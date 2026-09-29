package session

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/testfacts"
)

// Shared `go test -json` fixtures for the gate tests, generated at test time
// from tiny throwaway modules (the internal/testfacts approach): no pasted
// streams, so the parsers are pinned against what the toolchain emits now.

// jsonToolchainRe matches released Go versions ("go1.26.4"); anything else
// falls back to ambient toolchain selection.
var jsonToolchainRe = regexp.MustCompile(`^go\d+\.\d+`)

// jsonTestModule writes files (path -> content) into a fresh throwaway module
// and returns its directory.
func jsonTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[name]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module verifyprobe\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runJSONTest runs `go test -json -count=1` over args in dir and returns its
// stdout. A failing run still returns its stream: the exit code is the
// subject under test, not a test failure.
func runJSONTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"test", "-json", "-count=1"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = dir
	env := os.Environ()
	if v := runtime.Version(); jsonToolchainRe.MatchString(v) {
		env = append(env, "GOTOOLCHAIN="+v)
	}
	cmd.Env = append(env, "GOPROXY=off")
	var stdout strings.Builder
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("go test -json: %v", err)
		}
	}
	return stdout.String()
}

// jsonEvent is one test2json event. jsonStream emits the minimal stream a
// stubbed runner must return: production parses `go test -json`, so a
// fixture that never starts the toolchain still has to speak that protocol.
// A throwaway module (runJSONTest) is used whenever the toolchain can
// produce the stream; this is for a package name or a runner double that
// cannot.
type jsonEvent struct {
	Action      string
	Package     string
	Test        string
	Output      string
	FailedBuild string
}

func jsonStream(t *testing.T, evs ...jsonEvent) string {
	t.Helper()
	var b strings.Builder
	for _, ev := range evs {
		m := map[string]string{"Action": ev.Action}
		if ev.Package != "" {
			m["Package"] = ev.Package
		}
		if ev.Test != "" {
			m["Test"] = ev.Test
		}
		if ev.Output != "" {
			m["Output"] = ev.Output
		}
		if ev.FailedBuild != "" {
			m["FailedBuild"] = ev.FailedBuild
		}
		line, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// parseJSONTest parses a stream, failing the test on a read error.
func parseJSONTest(t *testing.T, stream string) *testfacts.Result {
	t.Helper()
	res, err := testfacts.Parse(strings.NewReader(stream))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return res
}

// failingModule is one passing test, one t.Errorf failure, and one subtest
// failure (t.Run with a space, which the stream sanitizes to an underscore).
func failingModule() map[string]string {
	return map[string]string{
		"calc.go": "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"calc_test.go": "package verifyprobe\n\nimport \"testing\"\n\n" +
			"func TestAdd(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"bad\") } }\n" +
			"func TestOops(t *testing.T) { t.Errorf(\"wrong value\") }\n" +
			"func TestSub(t *testing.T) { t.Run(\"case one\", func(t *testing.T) { t.Fatal(\"sub boom\") }) }\n",
	}
}

func TestFailedTopLevels(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	res := parseJSONTest(t, runJSONTest(t, jsonTestModule(t, failingModule()), "."))
	got := failedTopLevels(res)
	want := []string{"TestOops", "TestSub"}
	if len(got) != len(want) {
		t.Fatalf("failedTopLevels = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("failedTopLevels = %v; want %v", got, want)
		}
	}
	names := failedTestNames(res)
	hasSub := false
	for _, n := range names {
		if n == "TestSub/case_one" {
			hasSub = true
		}
		if n == "TestAdd" || n == "TestSub/case one" {
			t.Errorf("failedTestNames = %v; a passing test or an unsanitized subtest name does not belong", names)
		}
	}
	if !hasSub {
		t.Errorf("failedTestNames = %v; want the sanitized subtest TestSub/case_one", names)
	}
	if failedTopLevels(nil) != nil {
		t.Error("failedTopLevels(nil) must be nil: a run with no result names no failures")
	}
}

func TestParseTestJSON_BuildFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	dir := jsonTestModule(t, map[string]string{
		"calc.go":        "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"broken_test.go": "package verifyprobe\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { neverWritten() }\n",
	})
	res := parseTestJSON([]byte(runJSONTest(t, dir, ".")))
	if len(res.BuildFailures) == 0 {
		t.Fatalf("a test file calling an undefined function produced no build failures:\n%s", res.Summary())
	}
	bf := res.BuildFailures[0]
	if bf.File == "" || bf.Line == 0 || !strings.Contains(bf.Message, "neverWritten") {
		t.Errorf("build failure = %+v; want the file, line, and undefined symbol", bf)
	}
	for _, p := range res.Packages {
		if p.Status != testfacts.StatusBuildFailed {
			t.Errorf("package %s status = %s; want build-failed", p.Name, p.Status)
		}
	}
}

func TestVerificationOutput_KeepsFailuresCompact(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	res := parseJSONTest(t, runJSONTest(t, jsonTestModule(t, failingModule()), "."))
	out := verificationOutput(res)
	for _, want := range []string{"FAIL verifyprobe TestOops", "wrong value", "FAIL verifyprobe TestSub/case_one", "sub boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("verification output does not carry %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"Action"`) {
		t.Errorf("verification output leaks raw -json lines:\n%s", out)
	}
	if verificationOutput(nil) != "" {
		t.Error("verificationOutput(nil) must be empty")
	}
}

func TestVerificationOutput_RetainsUnparseableLines(t *testing.T) {
	res := parseTestJSON([]byte("still testing...\n"))
	if !strings.Contains(verificationOutput(res), "still testing...") {
		t.Errorf("a timeout's partial line was dropped: %q", verificationOutput(res))
	}
}
