package session

import (
	"testing"

	"codenerd/internal/core"
)

// TestAssertPendingEdits_RejectsMalformedPaths pins a gate that was not there.
//
// pending_edit is read by 26 policy rules and its FilePath is consumed by
// VirtualStore.resolvePath, all of which assume a repo-relative path with '/'
// separators and no traversal. assertPendingEdits wrote the fact with no shape
// check at all, so a tool argument naming an absolute path or climbing out of
// the workspace became a fact the rules then reasoned over.
//
// core.ValidatePendingEditFilePath is the check, and it had no production
// caller before this.
func TestAssertPendingEdits_RejectsMalformedPaths(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantAssert bool
	}{
		{name: "repo-relative path is accepted", path: "internal/core/foo.go", wantAssert: true},
		{name: "absolute path is refused", path: "/etc/passwd"},
		{name: "parent traversal is refused", path: "a/../../b.go"},
		{name: "backslash separators are refused", path: `internal\core\foo.go`},
		{name: "empty path is refused", path: "   "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &recordingKernel{}
			e := &Executor{kernel: k}

			facts := e.assertPendingEdits(ToolCall{
				Name: "write_file",
				Args: map[string]any{"path": tc.path, "content": "package p\n"},
			})

			if tc.wantAssert {
				if len(facts) != 1 {
					t.Fatalf("a valid path must be asserted, got %d facts", len(facts))
				}
				if len(k.asserted) != 1 {
					t.Fatalf("fact never reached the kernel: %+v", k.asserted)
				}
				if k.asserted[0].Predicate != core.PendingEditFactName {
					t.Errorf("predicate = %q, want %q", k.asserted[0].Predicate, core.PendingEditFactName)
				}
				return
			}
			if len(facts) != 0 {
				t.Fatalf("malformed path %q was asserted: %+v", tc.path, facts)
			}
			if len(k.asserted) != 0 {
				t.Fatalf("malformed path %q reached the kernel: %+v", tc.path, k.asserted)
			}
		})
	}
}

// TestValidatePendingEditFilePath_HasAProductionCaller is the regression that
// matters: the defect was not a wrong rule, it was that no rule ran. If the
// validation call is removed from assertPendingEdits, the case above turns
// green again on its own — this one does not.
func TestValidatePendingEditFilePath_HasAProductionCaller(t *testing.T) {
	k := &recordingKernel{}
	e := &Executor{kernel: k}
	facts := e.assertPendingEdits(ToolCall{
		Name: "write_file",
		Args: map[string]any{"path": "/absolute/path.go", "content": "x"},
	})
	if len(facts) != 0 || len(k.asserted) != 0 {
		t.Fatal("core.ValidatePendingEditFilePath is not being consulted by assertPendingEdits")
	}
}
