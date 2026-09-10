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
	if err := tools.Global().Register(tool); err != nil {
		t.Fatalf("register test tool %q: %v", tool.Name, err)
	}
	name := tool.Name
	t.Cleanup(func() { tools.Global().Unregister(name) })
}
