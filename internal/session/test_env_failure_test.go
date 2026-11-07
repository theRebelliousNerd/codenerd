package session

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/testfacts"
)

func envBuildStream(file, msg string) string {
	q, _ := json.Marshal(file + ":6:2: " + msg + "\n")
	return `{"Action":"build-output","ImportPath":"example.com/p","Output":` + string(q) + "}\n" +
		`{"Action":"build-fail","ImportPath":"example.com/p"}` + "\n" +
		`{"Action":"fail","Package":"example.com/p","FailedBuild":"example.com/p"}` + "\n"
}

func TestEnvironmentBuildFailure_OnlyLoaderErrorsOutsideTheWorkspace(t *testing.T) {
	ws := t.TempDir()
	parse := func(stream string) *testfacts.Result {
		res, err := testfacts.Parse(ws, strings.NewReader(stream))
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	goroot := "/usr/local/go/src/crypto/tls/auth.go"
	if _, ok := environmentBuildFailure(ws, parse(envBuildStream(goroot, "package crypto/ecdsa is not in std"))); !ok {
		t.Fatal("a toolchain loader failure in GOROOT was charged to the code")
	}
	if _, ok := environmentBuildFailure(ws, parse(envBuildStream("../../go/pkg/mod/x@v1/copy.go", "open /cache/abc-d: no such file or directory"))); !ok {
		t.Fatal("a missing build-cache entry was charged to the code")
	}
	if _, ok := environmentBuildFailure(ws, parse(envBuildStream(goroot, "undefined: Foo"))); ok {
		t.Fatal("a type error outside the workspace (a dependency bump) was called an environment failure")
	}
	if _, ok := environmentBuildFailure(ws, parse(envBuildStream("pkg/a.go", "package crypto/ecdsa is not in std"))); ok {
		t.Fatal("a failure located in the workspace was called an environment failure")
	}
}

// The gate re-runs once on an environment failure and keeps the second
// measurement; one that fails the same way twice is indeterminate, never a
// failure of the code.
func TestVerifyTestRuns_RetriesAnEnvironmentFailureOnce(t *testing.T) {
	ws := t.TempDir()
	env := envBuildStream("/usr/local/go/src/crypto/tls/auth.go", "package crypto/ecdsa is not in std")
	pass := `{"Action":"pass","Package":"example.com/p"}` + "\n"
	real := verifyTestRunner
	t.Cleanup(func() { verifyTestRunner = real })

	calls := 0
	verifyTestRunner = func(context.Context, string, []string, string, []string) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte(env), errors.New("exit status 1")
		}
		return []byte(pass), nil
	}
	if v := verifyTestRuns(context.Background(), ws, "", testRun{Packages: []string{"./..."}}); v.Verdict() != VerifyPassed || calls != 2 {
		t.Fatalf("verdict %s after %d run(s), want a pass on the second run", v.Verdict(), calls)
	}

	calls = 0
	verifyTestRunner = func(context.Context, string, []string, string, []string) ([]byte, error) {
		calls++
		return []byte(env), errors.New("exit status 1")
	}
	v := verifyTestRuns(context.Background(), ws, "", testRun{Packages: []string{"./..."}})
	if v.Verdict() != VerifyIndeterminate || calls != 2 || !strings.Contains(v.Reason, "not the code under test") {
		t.Fatalf("verdict %s (%s) after %d run(s), want indeterminate naming the environment", v.Verdict(), v.Reason, calls)
	}
}
