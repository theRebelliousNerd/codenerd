package tools

import (
	"context"
	"os/exec"
	"strings"
	"sync"

	"codenerd/internal/processutil"
)

// A TestRun is one test command the tool layer actually started: the argv it
// ran and the exit code it ended with. It is the only evidence that a tool
// call executed tests. A tool's name is not -- run_impacted_tests used to
// count as a test execution when it was a dry run, when it selected nothing
// and when no edit was known to it, all three a success that ran no test
// (external audit N07, 2026-09-19).
//
// Dir and EnvDigest say where and under what environment it ran
// (processutil.Identity of the process). A planned check is satisfied by a
// receipt only when argv, Dir and EnvDigest all match its own invocation;
// a receipt without them -- a producer that cannot see its process -- is
// never taken for one (audit workspace-test-receipt-invocation-identity).
type TestRun struct {
	Argv      []string
	ExitCode  int
	Dir       string
	EnvDigest string
}

// TestRunOf is the receipt of cmd, which ran argv to exit code: argv as the
// tool names it, the identity from the process itself.
func TestRunOf(cmd *exec.Cmd, argv []string, code int) TestRun {
	dir, env := processutil.Identity(cmd)
	return TestRun{Argv: argv, ExitCode: code, Dir: dir, EnvDigest: env}
}

type testRunLogKey struct{}

type testRunLog struct {
	mu   sync.Mutex
	runs []TestRun
}

// WithTestRunLog returns a context in which RecordTestRun records, and a
// function that returns what was recorded.
func WithTestRunLog(ctx context.Context) (context.Context, func() []TestRun) {
	log := &testRunLog{}
	return context.WithValue(ctx, testRunLogKey{}, log), func() []TestRun {
		log.mu.Lock()
		defer log.mu.Unlock()
		return append([]TestRun(nil), log.runs...)
	}
}

// RecordTestRun records a test command the tool layer ran to an exit, where
// the process is started -- never where tests are only selected, listed or
// planned. Without a log in the context it records nothing.
//
// A negative ExitCode is a process that did not exit: killed by a signal (a
// timeout's SIGKILL, or a SIGTERM from outside codeNERD) or never reported.
// It ran no suite to a verdict, so it is not recorded. It used to be, as a
// failing run: a `go test ./...` terminated after 233 s became "`go test
// -count=1 ./...` exited -1", and the /test_run round sent the model to fix
// code for a failure no test had reported (problems log 2026-10-28).
func RecordTestRun(ctx context.Context, run TestRun) {
	if run.ExitCode < 0 {
		return
	}
	log, ok := ctx.Value(testRunLogKey{}).(*testRunLog)
	if !ok {
		return
	}
	log.mu.Lock()
	log.runs = append(log.runs, run)
	log.mu.Unlock()
}

// testCommandPrefixes are the command lines that run a test suite.
var testCommandPrefixes = []string{
	"go test",
	"gotestsum",
	"pytest",
	"python -m pytest",
	"python3 -m pytest",
	"cargo test",
	"npm test",
	"npm run test",
	"yarn test",
	"pnpm test",
	"dotnet test",
	"mvn test",
	"gradle test",
	"./gradlew test",
	"ctest",
	"bazel test",
}

// IsTestCommand reports whether a shell command line runs a test suite, for a
// shell tool to decide whether the command it just ran was a test run.
func IsTestCommand(commandLine string) bool {
	cmd := strings.ToLower(strings.TrimSpace(commandLine))
	for _, prefix := range testCommandPrefixes {
		if cmd == prefix || (strings.HasPrefix(cmd, prefix) && len(cmd) > len(prefix) && (cmd[len(prefix)] == ' ' || cmd[len(prefix)] == '\t')) {
			return true
		}
	}
	return false
}
