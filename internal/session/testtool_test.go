package session

import (
	"testing"

	"codenerd/internal/tools"
)

// registerTestTool registers a tool into the process-wide registry for the
// duration of one test.
//
// It exists because the plain call it replaces was wrong in two ways that both
// produce a passing test rather than a failing one.
//
// Register returns an error and every call site discarded it. A tool rejected
// for any reason -- a name already taken, a missing effect, a validation
// failure -- simply is not there, and the test then exercises an executor with
// an empty toolbox while asserting on side effects that never happen. The
// symptom is an assertion about a counter that stayed zero, which reads as a
// bug in the code under test.
//
// And Register rejects duplicate names, so the second run of any such test in
// one process kept the first run's Execute closure. The test still passed on
// `go test`, which runs each test once, and failed under `-count=2` with a
// counter nothing was incrementing -- the first run's closure was incrementing
// the first run's variable. That is the shape of a test that only works because
// nobody ran it twice.
//
// Cleanup unregisters, so tests are isolated from each other and repeatable
// within a process.
func registerTestTool(t *testing.T, tool *tools.Tool) {
	t.Helper()

	if tool == nil {
		t.Fatal("registerTestTool: nil tool")
	}
	// The test owns the name for its duration: a tool already registered
	// under it (another test's production registration, a boot path) is set
	// aside and restored at cleanup, so the result does not depend on which
	// test ran first in the process.
	name := tool.Name
	original := tools.Global().Get(name)
	if original != nil {
		tools.Global().Unregister(name)
	}
	if err := tools.Global().Register(tool); err != nil {
		t.Fatalf("register test tool %q: %v", tool.Name, err)
	}
	t.Cleanup(func() {
		tools.Global().Unregister(name)
		if original != nil {
			_ = tools.Global().Register(original)
		}
	})
}
