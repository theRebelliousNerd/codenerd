package session

import (
	"testing"

	"codenerd/internal/core"
)

// isWriteMutationTool gates two independent things: whether a write-oriented
// turn is credited with having written (checkHollowSuccess) and whether a
// nerd.md-protected path is defended (projectForbidsWrite). A name missing from
// the list therefore both fails a successful edit and silently opens the safety
// gate for that tool.
//
// The list was originally written from generic LLM tool vocabulary rather than
// from internal/core's ActionType registry, so it missed the three line-editing
// tools entirely. This test pins it to the registry so the next tool added
// there cannot quietly bypass either consumer.
func TestIsWriteMutationTool_CoversEveryDurableWriteAction(t *testing.T) {
	durableWrites := []core.ActionType{
		core.ActionWriteFile,
		core.ActionEditFile,
		core.ActionDeleteFile,
		core.ActionEditLines,
		core.ActionInsertLines,
		core.ActionDeleteLines,
		core.ActionEditElement,
		core.ActionFSWrite,
	}

	for _, action := range durableWrites {
		if !isWriteMutationTool(string(action)) {
			t.Errorf("isWriteMutationTool(%q) = false, but that action lands durable changes; "+
				"a successful edit with it is reported as hollow success AND nerd.md write "+
				"protection does not fire for it", action)
		}
	}
}

// Over-broad is its own failure: crediting a read as a write would let a
// write-oriented turn that only looked at files claim success.
func TestIsWriteMutationTool_RejectsNonWriteActions(t *testing.T) {
	nonWrites := []core.ActionType{
		core.ActionReadFile,
		core.ActionListFiles,
		core.ActionGlob,
		core.ActionGrep,
		core.ActionSearchCode,
		core.ActionRunTests,
		core.ActionRunCommand,
		core.ActionGetElements,
		core.ActionOpenFile,
		core.ActionFSRead,
	}

	for _, action := range nonWrites {
		if isWriteMutationTool(string(action)) {
			t.Errorf("isWriteMutationTool(%q) = true, but that action writes nothing", action)
		}
	}
}

func TestIsWriteMutationTool_NormalizesCaseAndSpace(t *testing.T) {
	for _, name := range []string{"  insert_lines  ", "INSERT_LINES", "Edit_Lines"} {
		if !isWriteMutationTool(name) {
			t.Errorf("isWriteMutationTool(%q) = false; the name arrives from model output "+
				"and is not guaranteed to be normalized", name)
		}
	}
}

func TestRecordWrittenPaths_RecordsEveryCanonicalNestedTarget(t *testing.T) {
	result := &ExecutionResult{WrittenPaths: []string{"existing.go"}}
	err := recordWrittenPaths(result, map[string]any{
		"edits": []any{
			map[string]any{"path": "internal/a.go"},
			map[string]any{"path": "./internal/a.go"},
			map[string]any{"file_path": "internal/b.go"},
		},
	}, "")
	if err != nil {
		t.Fatalf("recordWrittenPaths: %v", err)
	}
	want := []string{"existing.go", "internal/a.go", "internal/b.go"}
	if len(result.WrittenPaths) != len(want) {
		t.Fatalf("WrittenPaths=%v want %v", result.WrittenPaths, want)
	}
	for i := range want {
		if result.WrittenPaths[i] != want[i] {
			t.Fatalf("WrittenPaths[%d]=%q want %q", i, result.WrittenPaths[i], want[i])
		}
	}
}
