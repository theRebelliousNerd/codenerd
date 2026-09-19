package tools

import (
	"context"
	"strings"
	"sync"
)

// A TestRun is one test command the tool layer actually started: the argv it
// ran and the exit code it ended with. It is the only evidence that a tool
// call executed tests. A tool's name is not -- run_impacted_tests used to
// count as a test execution when it was a dry run, when it selected nothing
// and when no edit was known to it, all three a success that ran no test
// (external audit N07, 2026-09-19).
type TestRun struct {
	Argv     []string
	ExitCode int
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

// RecordTestRun records that a test command was started, where the process is
// started -- never where tests are only selected, listed or planned. Without a
// log in the context it records nothing.
func RecordTestRun(ctx context.Context, run TestRun) {
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
