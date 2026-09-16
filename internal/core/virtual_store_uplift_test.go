package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tactile"
)

// nilExecutorStore builds the hazardous combination the constructor permits:
// no injected executor, modern path disabled. Every shell handler must fail
// closed on this store instead of panicking on the nil interface.
func nilExecutorStore() *VirtualStore {
	vs := NewVirtualStore(nil)
	vs.DisableModernExecutor()
	return vs
}

// TestVirtualStore_NilExecutorFailsClosed pins the root fix for a panic the
// suite used to dodge by hand-injecting stub executors (see the "Prevent nil
// pointer" comment in virtual_store_gaps_test.go). Each shell handler must
// report the missing executor as a failed result, never panic.
func TestVirtualStore_NilExecutorFailsClosed(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name string
		req  ActionRequest
	}{
		{"run_tests", ActionRequest{ActionID: "nil-1", Type: ActionRunTests, Target: "go test ./..."}},
		{"build_project", ActionRequest{ActionID: "nil-2", Type: ActionBuildProject, Target: "go build ./..."}},
		{"git_operation", ActionRequest{ActionID: "nil-3", Type: ActionGitOperation, Target: "status"}},
		{"exec_cmd", ActionRequest{ActionID: "nil-4", Type: ActionExecCmd, Target: "echo hello"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vs := nilExecutorStore()
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("executeAction(%s) panicked on nil executor: %v", tc.name, r)
				}
			}()
			result, err := vs.executeAction(ctx, tc.req)
			if err != nil {
				t.Fatalf("executeAction(%s) error = %v, want failed result with nil error", tc.name, err)
			}
			if result.Success {
				t.Fatalf("executeAction(%s) Success = true with no executor", tc.name)
			}
			if !strings.Contains(result.Error, "no executor") {
				t.Fatalf("executeAction(%s) Error = %q, want it to name the missing executor", tc.name, result.Error)
			}
		})
	}
}

// The public Exec entry point shares the guard: session code calling Exec on
// an executor-less store gets an error, not a panic.
func TestVirtualStore_ExecNilExecutorFailsClosed(t *testing.T) {
	vs := nilExecutorStore()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Exec panicked on nil executor: %v", r)
		}
	}()
	_, _, err := vs.Exec(context.Background(), "echo hello", nil)
	if err == nil {
		t.Fatal("Exec with nil executor = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "no executor") {
		t.Fatalf("Exec error = %q, want it to name the missing executor", err)
	}
}

// TestVirtualStore_AllActionsDispatched pins dispatch parity: every ActionType
// constant must route to a handler through executeAction. Handlers may fail
// for missing dependencies on this minimally-wired store, but they must
// never report "unknown action type" and never panic. When adding a new
// ActionType, extend allActions below (audit_actions.py independently
// verifies const-to-dispatch parity).
func TestVirtualStore_AllActionsDispatched(t *testing.T) {
	k := setupMockKernel(t)
	vs := NewVirtualStore(&stubExecutor{})
	vs.SetKernel(k)
	vs.DisableBootGuard()
	ctx := context.Background()
	allActions := []ActionType{
		ActionExecCmd, ActionReadFile, ActionWriteFile, ActionEditFile, ActionDeleteFile, ActionListFiles,
		ActionGlob, ActionGrep, ActionSearchCode, ActionSearchFiles, ActionAnalyzeCode, ActionRunTests,
		ActionRunCommand, ActionBash, ActionRunBuild, ActionBuildProject, ActionGitOperation, ActionAnalyzeImpact,
		ActionBrowse, ActionResearch, ActionAskUser, ActionEscalate, ActionDelegate, ActionDelegateReviewer,
		ActionDelegateCoder, ActionDelegateTester, ActionDelegateResearcher, ActionDelegateToolGenerator, ActionShowDiff, ActionExecTool,
		ActionOpenFile, ActionGetElements, ActionGetElement, ActionEditElement, ActionRefreshScope, ActionCloseScope,
		ActionEditLines, ActionInsertLines, ActionDeleteLines, ActionReadErrorLog, ActionAnalyzeRootCause, ActionGeneratePatch,
		ActionEscalateToUser, ActionComplete, ActionInterrogative, ActionResumeTask, ActionRefreshShardCtx, ActionFSRead,
		ActionFSWrite, ActionGenerateTool, ActionOuroborosDetect, ActionOuroborosGen, ActionOuroborosCompile, ActionOuroborosReg,
		ActionRefineTool, ActionCampaignClarify, ActionCampaignCreateFile, ActionCampaignModifyFile, ActionCampaignWriteTest, ActionCampaignRunTest,
		ActionCampaignResearch, ActionCampaignVerify, ActionCampaignDocument, ActionCampaignRefactor, ActionCampaignIntegrate, ActionCampaignComplete,
		ActionCampaignFinalVerify, ActionCampaignCleanup, ActionArchiveCampaign, ActionShowCampaignStatus, ActionShowCampaignProg, ActionAskCampaignInt,
		ActionRunPhaseCheckpoint, ActionPauseAndReplan, ActionCompressContext, ActionEmergencyCompress, ActionCreateCheckpoint, ActionInvestigateAnomaly,
		ActionInvestigateSystemic, ActionUpdateWorldModel, ActionCorrectiveResearch, ActionCorrectiveDocs, ActionCorrectiveDecompose, ActionQueryElements,
		ActionPythonEnvSetup, ActionPythonEnvExec, ActionPythonRunPytest, ActionPythonApplyPatch, ActionPythonSnapshot, ActionPythonRestore,
		ActionPythonTeardown, ActionSWEBenchSetup, ActionSWEBenchApplyPatch, ActionSWEBenchRunTests, ActionSWEBenchSnapshot, ActionSWEBenchRestore,
		ActionSWEBenchEvaluate, ActionSWEBenchTeardown, ActionContext7Fetch, ActionWebSearch, ActionWebFetch, ActionBrowserNavigate,
		ActionBrowserExtract, ActionBrowserScreenshot, ActionBrowserClick, ActionBrowserType, ActionBrowserClose, ActionResearchCacheGet,
		ActionResearchCacheSet,
	}

	for _, typ := range allActions {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("PANIC dispatch %s: %v", typ, r)
				}
			}()
			req := ActionRequest{ActionID: "sweep", Type: typ, Target: "probe-target", Payload: map[string]any{}}
			_, err := vs.executeAction(ctx, req)
			if err != nil && strings.Contains(err.Error(), "unknown action type") {
				t.Errorf("NO DISPATCH for %s", typ)
			}
		}()
	}
}

// TestVirtualStore_UnknownActionFailsClosed pins the dispatch default: an
// action type with no handler is an explicit error, never silent success.
func TestVirtualStore_UnknownActionFailsClosed(t *testing.T) {
	vs := NewVirtualStore(&stubExecutor{})
	vs.DisableBootGuard()
	_, err := vs.executeAction(context.Background(), ActionRequest{
		ActionID: "unknown-1", Type: ActionType("no_such_action"), Target: "x",
	})
	if err == nil {
		t.Fatal("executeAction(unknown type) = nil error, want failure")
	}
	if !strings.Contains(err.Error(), "unknown action type") {
		t.Fatalf("error = %q, want unknown-action-type complaint", err)
	}
}

// TestHandleSearchCode_SkipsLargeFiles pins the documented walk guard: files
// over maxSearchFileSize are not read whole into memory, so a match hiding
// in a bundle does not appear in search_result facts.
func TestHandleSearchCode_SkipsLargeFiles(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	ctx := context.Background()
	const needle = "upliftNeedle1984"
	if err := os.WriteFile(filepath.Join(dir, "small.go"), []byte("package p\n// "+needle+"\n"), 0644); err != nil {
		t.Fatalf("write small: %v", err)
	}
	big := []byte("// " + needle + "\n" + strings.Repeat("x", maxSearchFileSize+1024))
	if err := os.WriteFile(filepath.Join(dir, "big.go"), big, 0644); err != nil {
		t.Fatalf("write big: %v", err)
	}
	res, err := vs.handleSearchCode(ctx, ActionRequest{ActionID: "s-big", Target: needle})
	if err != nil {
		t.Fatalf("handleSearchCode: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	for _, f := range res.FactsToAdd {
		if len(f.Args) > 0 {
			if s, _ := f.Args[0].(string); strings.Contains(s, "big.go") {
				t.Fatalf("large file was searched, want skipped: %+v", f)
			}
		}
	}
	foundSmall := false
	for _, f := range res.FactsToAdd {
		if len(f.Args) > 0 {
			if s, _ := f.Args[0].(string); strings.Contains(s, "small.go") {
				foundSmall = true
			}
		}
	}
	if !foundSmall {
		t.Fatalf("small-file match missing from facts: %+v", res.FactsToAdd)
	}
}

// TestHandleReadFile_TruncatesLargeFile pins the 100KB read bound: the read
// succeeds, the file_truncated fact records the bound, and the output does
// not carry the whole file.
func TestHandleReadFile_TruncatesLargeFile(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	ctx := context.Background()
	content := strings.Repeat("0123456789abcdef\n", 8000) // ~136KB
	path := filepath.Join(dir, "huge.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := vs.handleReadFile(ctx, ActionRequest{ActionID: "r-big", Target: "huge.txt"})
	if err != nil {
		t.Fatalf("handleReadFile: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	if trunc, _ := res.Metadata["truncated"].(bool); !trunc {
		t.Errorf("truncated metadata = %v, want true", res.Metadata["truncated"])
	}
	found := false
	for _, f := range res.FactsToAdd {
		if f.Predicate == "file_truncated" {
			found = true
		}
	}
	if !found {
		t.Errorf("file_truncated fact missing: %+v", res.FactsToAdd)
	}
	if len(res.Output) >= len(content) {
		t.Errorf("output len %d, want it bounded below file len %d", len(res.Output), len(content))
	}
}

// lineEditStore wires a real tactile file editor over a temp dir: line edits
// in these tests land on real files, not mocks.
func lineEditStore(t *testing.T) (*VirtualStore, string) {
	t.Helper()
	dir := t.TempDir()
	vs := NewVirtualStore(nil)
	vs.workingDir = dir
	vs.SetFileEditor(NewTactileFileEditorAdapter(tactile.NewFileEditor()))
	return vs, dir
}

func writeLinesFile(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return name
}

func readLinesFile(t *testing.T, dir, name string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

// TestHandleLineEdits_AcceptsIntPayloads pins payloadInt unification: int,
// int64, float64 and string line numbers must all edit the addressed lines.
// The old float64-only assertion silently read int payloads as 0, which
// errored for edit/delete and mis-inserted at the top for insert.
func TestHandleLineEdits_AcceptsIntPayloads(t *testing.T) {
	ctx := context.Background()
	base := []string{"one", "two", "three", "four", "five"}
	kinds := map[string]any{
		"int":     2,
		"int64":   int64(2),
		"float64": float64(2),
		"string":  "2",
	}
	for kind, val := range kinds {
		t.Run("edit/"+kind, func(t *testing.T) {
			vs, dir := lineEditStore(t)
			name := writeLinesFile(t, dir, "e.txt", base)
			end := val
			switch v := val.(type) {
			case int:
				end = v + 1
			case int64:
				end = v + 1
			case float64:
				end = v + 1
			case string:
				end = "3"
			}
			res, err := vs.handleEditLines(ctx, ActionRequest{
				ActionID: "e1", Type: ActionEditLines, Target: name,
				Payload: map[string]any{"start_line": val, "end_line": end, "content": "TWO\nTHREE"},
			})
			if err != nil {
				t.Fatalf("handleEditLines: %v", err)
			}
			if !res.Success {
				t.Fatalf("expected success, got: %+v", res)
			}
			if got := readLinesFile(t, dir, name); strings.Join(got, "|") != "one|TWO|THREE|four|five" {
				t.Fatalf("wrong lines edited: %q", got)
			}
		})
		t.Run("insert/"+kind, func(t *testing.T) {
			vs, dir := lineEditStore(t)
			name := writeLinesFile(t, dir, "i.txt", base)
			res, err := vs.handleInsertLines(ctx, ActionRequest{
				ActionID: "i1", Type: ActionInsertLines, Target: name,
				Payload: map[string]any{"after_line": val, "content": "INSERTED"},
			})
			if err != nil {
				t.Fatalf("handleInsertLines: %v", err)
			}
			if !res.Success {
				t.Fatalf("expected success, got: %+v", res)
			}
			if got := readLinesFile(t, dir, name); strings.Join(got, "|") != "one|two|INSERTED|three|four|five" {
				t.Fatalf("insert landed in the wrong place: %q", got)
			}
		})
		t.Run("delete/"+kind, func(t *testing.T) {
			vs, dir := lineEditStore(t)
			name := writeLinesFile(t, dir, "d.txt", base)
			end := val
			switch v := val.(type) {
			case int:
				end = v + 1
			case int64:
				end = v + 1
			case float64:
				end = v + 1
			case string:
				end = "3"
			}
			res, err := vs.handleDeleteLines(ctx, ActionRequest{
				ActionID: "d1", Type: ActionDeleteLines, Target: name,
				Payload: map[string]any{"start_line": val, "end_line": end},
			})
			if err != nil {
				t.Fatalf("handleDeleteLines: %v", err)
			}
			if !res.Success {
				t.Fatalf("expected success, got: %+v", res)
			}
			if got := readLinesFile(t, dir, name); strings.Join(got, "|") != "one|four|five" {
				t.Fatalf("wrong lines deleted: %q", got)
			}
		})
	}
}

// TestHandleLineEdits_RejectsGarbage pins fail-closed parsing: a present but
// non-numeric line number errors instead of editing line 0.
func TestHandleLineEdits_RejectsGarbage(t *testing.T) {
	ctx := context.Background()
	vs, dir := lineEditStore(t)
	name := writeLinesFile(t, dir, "g.txt", []string{"one", "two"})
	for _, tc := range []struct {
		handler string
		call    func() (ActionResult, error)
	}{
		{"edit", func() (ActionResult, error) {
			return vs.handleEditLines(ctx, ActionRequest{ActionID: "g1", Type: ActionEditLines, Target: name,
				Payload: map[string]any{"start_line": "abc", "end_line": 2, "content": "X"}})
		}},
		{"delete", func() (ActionResult, error) {
			return vs.handleDeleteLines(ctx, ActionRequest{ActionID: "g2", Type: ActionDeleteLines, Target: name,
				Payload: map[string]any{"start_line": 1, "end_line": "zz"}})
		}},
		{"insert", func() (ActionResult, error) {
			return vs.handleInsertLines(ctx, ActionRequest{ActionID: "g3", Type: ActionInsertLines, Target: name,
				Payload: map[string]any{"after_line": "nope", "content": "X"}})
		}},
	} {
		res, err := tc.call()
		if err != nil {
			t.Fatalf("%s: unexpected error return: %v", tc.handler, err)
		}
		if res.Success {
			t.Fatalf("%s: garbage line number succeeded, want rejection", tc.handler)
		}
	}
	if got := readLinesFile(t, dir, name); strings.Join(got, "|") != "one|two" {
		t.Fatalf("file mutated by rejected edits: %q", got)
	}
}

// TestSWEBenchSetup_ShortCommitNoPanic pins the base-commit clamp: the commit
// is optional in the payload, and slicing it unconditionally panicked on
// short or missing values instead of failing closed.
func TestSWEBenchSetup_ShortCommitNoPanic(t *testing.T) {
	ctx := context.Background()
	vs := NewVirtualStore(nil)
	for _, commit := range []string{"", "abc", "abcdef1234567890"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("handleSWEBenchSetup(base_commit=%q) panicked: %v", commit, r)
				}
			}()
			res, err := vs.handleSWEBenchSetup(ctx, ActionRequest{
				ActionID: "sw-1", Type: ActionSWEBenchSetup, Target: "x",
				Payload: map[string]any{"instance_id": "inst-1", "repo": "r", "base_commit": commit},
			})
			if err != nil {
				t.Fatalf("handleSWEBenchSetup: %v", err)
			}
			if !res.Success {
				t.Fatalf("expected success, got: %+v", res)
			}
		}()
	}
}

// TestCampaignCreateFile_RespectsNerdMd pins the gate extension end to end:
// a campaign write to a protected path is blocked with no file created,
// while the same write to an unprotected path lands.
func TestCampaignCreateFile_RespectsNerdMd(t *testing.T) {
	ctx := context.Background()

	blocked := NewVirtualStore(nil)
	blockedDir := t.TempDir()
	blocked.workingDir = blockedDir
	blocked.SetKernel(&forbidKernel{match: "secrets.env", reason: "live secret"})
	res, err := blocked.executeAction(ctx, ActionRequest{
		ActionID: "camp-1", Type: ActionCampaignCreateFile, Target: "secrets.env",
		Payload: map[string]any{"content": "KEY=x"},
	})
	if err != nil {
		t.Fatalf("executeAction: %v", err)
	}
	if res.Success {
		t.Fatalf("campaign write to protected path succeeded: %+v", res)
	}
	if _, statErr := os.Stat(filepath.Join(blockedDir, "secrets.env")); !os.IsNotExist(statErr) {
		t.Fatalf("protected file was created despite the block (stat=%v)", statErr)
	}

	allowed := NewVirtualStore(nil)
	allowedDir := t.TempDir()
	allowed.workingDir = allowedDir
	allowed.SetKernel(setupMockKernel(t))
	res, err = allowed.executeAction(ctx, ActionRequest{
		ActionID: "camp-2", Type: ActionCampaignCreateFile, Target: "notes.txt",
		Payload: map[string]any{"content": "hello"},
	})
	if err != nil {
		t.Fatalf("executeAction: %v", err)
	}
	if !res.Success {
		t.Fatalf("campaign write to unprotected path failed: %+v", res)
	}
	raw, readErr := os.ReadFile(filepath.Join(allowedDir, "notes.txt"))
	if readErr != nil || string(raw) != "hello" {
		t.Fatalf("unprotected write did not land (content=%q, err=%v)", raw, readErr)
	}
}
