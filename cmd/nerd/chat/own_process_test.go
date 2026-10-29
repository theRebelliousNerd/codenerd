package chat

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// ownProcessEnv names, in a child test process, the one test it runs.
const ownProcessEnv = "CODENERD_CHAT_OWN_PROCESS"

// inOwnProcess runs the calling test alone in a child test process and
// reports true in the parent, which then returns: the child's verdict is the
// test's.
//
// It is for tests that read back what the process-wide logging wrote -- the
// problems log, the warnings log. Every goroutine in a process writes to the
// one workspace logging is bound to, so in the package's process a goroutine
// another test left running could add a line to the log the test counts
// (TestDebugSelf_ProjectsProblemsLogIntoKernel failed once that way under a
// loaded run, 2026-10-29). In its own process the log is the test's alone.
func inOwnProcess(t *testing.T) bool {
	t.Helper()
	if os.Getenv(ownProcessEnv) == t.Name() {
		return false
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), ownProcessEnv+"="+t.Name())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s in its own process: %v\n%s", t.Name(), err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+t.Name()) {
		t.Fatalf("%s did not run in its own process:\n%s", t.Name(), out)
	}
	return true
}
