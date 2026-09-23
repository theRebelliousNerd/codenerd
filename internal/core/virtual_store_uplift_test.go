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
		ActionCampaignResearch, ActionCampaignVerify, ActionCampaignDocument, ActionCampaignRefactor, ActionCampaignIntegrate,
		ActionCampaignFinalVerify, ActionCampaignCleanup, ActionArchiveCampaign, ActionShowCampaignStatus, ActionShowCampaignProg, ActionAskCampaignInt,
		ActionCompressContext, ActionEmergencyCompress, ActionCreateCheckpoint, ActionInvestigateAnomaly,
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

// TestValidatorAlias_CampaignWritesVerified pins validatorTypeAlias: campaign
// actions that delegate to file/test handlers must run those handlers'
// validators instead of taking the "skipped" branch.
func TestValidatorAlias_CampaignWritesVerified(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "camp.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := NewValidatorRegistry()
	RegisterAllValidators(r)

	req := ActionRequest{ActionID: "v-1", Type: ActionCampaignCreateFile, Target: path,
		Payload: map[string]any{"content": "hello"}}
	res := ActionResult{Success: true, Output: "written"}
	results := r.Validate(ctx, req, res)
	seenWrite := false
	// A registry-level skip is a single "no validators registered" result.
	// Per-validator skips (no parser for .txt, not a Mangle file) are
	// legitimate granular verdicts, not dispatch misses.
	if len(results) == 1 && results[0].Method == ValidationMethodSkipped {
		t.Fatalf("campaign write validation skipped: %+v", results)
	}
	for _, vr := range results {
		if vr.ActionID != "v-1" {
			t.Errorf("result not stamped with action id: %+v", vr)
		}
		// Validators leave ActionType empty; the registry stamps the real
		// (campaign) type so facts attribute the verification correctly.
		if vr.ActionType != ActionCampaignCreateFile {
			t.Errorf("ActionType = %q, want the campaign type", vr.ActionType)
		}
	}
	// file_write_validator must have run and verified the hash.
	for _, vr := range results {
		if vr.Method == ValidationMethodHash && vr.Verified {
			seenWrite = true
		}
	}
	if !seenWrite {
		t.Fatalf("no verified hash result for campaign write: %+v", results)
	}

	// Tamper: the aliased validators must catch a mismatch at high confidence.
	if err := os.WriteFile(path, []byte("tampered"), 0644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	results = r.Validate(ctx, req, res)
	if ValidateAll(results) {
		t.Fatalf("tampered campaign write verified: %+v", results)
	}
	if f := FirstFailure(results); f == nil || f.Confidence < 0.8 {
		t.Fatalf("tamper not caught at high confidence: %+v", results)
	}
}

// TestValidatorAlias_CampaignRunTestNotSkipped pins test delegation: a
// campaign test run must reach the execution/test validators.
func TestValidatorAlias_CampaignRunTestNotSkipped(t *testing.T) {
	ctx := context.Background()
	r := NewValidatorRegistry()
	RegisterAllValidators(r)
	req := ActionRequest{ActionID: "v-2", Type: ActionCampaignRunTest, Target: "go test ./..."}
	res := ActionResult{Success: true, Output: "ok  	pkg	1.2s"}
	results := r.Validate(ctx, req, res)
	if len(results) == 0 {
		t.Fatal("no validation results")
	}
	for _, vr := range results {
		if vr.Method == ValidationMethodSkipped {
			t.Fatalf("campaign test run validation skipped: %+v", results)
		}
	}
}

// TestValidatorAlias_CampaignDocumentSignalSkipped pins the deliberate
// exception: signal-mode document requests (no content, nothing written)
// take the skipped branch rather than failing existence checks.
func TestValidatorAlias_CampaignDocumentSignalSkipped(t *testing.T) {
	ctx := context.Background()
	r := NewValidatorRegistry()
	RegisterAllValidators(r)
	req := ActionRequest{ActionID: "v-3", Type: ActionCampaignDocument, Target: "missing.md"}
	res := ActionResult{Success: true, Output: "Documentation requested"}
	results := r.Validate(ctx, req, res)
	if len(results) != 1 || results[0].Method != ValidationMethodSkipped {
		t.Fatalf("signal-mode document should skip validation, got: %+v", results)
	}
}

// TestMangleSyntaxValidator_RealParser pins the heuristic-to-parser upgrade:
// malformed rules the old checks never noticed (unbalanced parens) must
// fail, while valid multi-line rules pass.
func TestMangleSyntaxValidator_RealParser(t *testing.T) {
	ctx := context.Background()
	v := NewMangleSyntaxValidator()
	cases := []struct {
		name    string
		content string
		wantOK  bool
	}{
		{"unbalanced paren", "Decl foo(Name).\nfoo(\"bar\".\n", false},
		{"missing period", "Decl foo(Name)\n", false},
		{"sql aggregation", "total(X) :- X = sum(Y).\n", false},
		{"valid multiline rule", "Decl a(X).\nDecl b(X).\nb(X) :-\n  a(X).\n", true},
		{"valid fact", "Decl foo(Name).\nfoo(\"bar\").\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "case.mg")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			vr := v.Validate(ctx, ActionRequest{Type: ActionWriteFile, Target: path}, ActionResult{Success: true})
			if vr.Verified != tc.wantOK {
				t.Fatalf("Verified=%v, want %v (error=%q)", vr.Verified, tc.wantOK, vr.Error)
			}
			if !tc.wantOK && !strings.HasPrefix(vr.Error, "syntax validation failed") {
				t.Fatalf("Error=%q, want the validation.mg vocabulary prefix", vr.Error)
			}
		})
	}
}

// TestSyntaxValidator_CoversDeleteLines pins delete coverage: removing lines
// can break syntax (a deleted closing brace), so deletes take the parsers.
func TestSyntaxValidator_CoversDeleteLines(t *testing.T) {
	v := NewSyntaxValidator()
	if !v.CanValidate(ActionDeleteLines) {
		t.Fatal("SyntaxValidator skips delete_lines")
	}
	mv := NewMangleSyntaxValidator()
	for _, at := range []ActionType{ActionEditLines, ActionInsertLines, ActionDeleteLines} {
		if !mv.CanValidate(at) {
			t.Fatalf("MangleSyntaxValidator skips %s", at)
		}
	}
}

// TestCodeDOMValidator_PreEditCaptureWired proves the capture hook end to
// end: CapturePreEditState had no callers, so the "hash unchanged" check
// never fired. Part 1 pins the check given a capture; part 2 pins that
// executeAction performs the capture before a real line edit.
func TestCodeDOMValidator_PreEditCaptureWired(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "w.go")
	if err := os.WriteFile(path, []byte("package w\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Part 1: captured pre-state + identical bytes = applied-nothing failure.
	cv := NewCodeDOMValidator()
	if err := cv.CapturePreEditState(path); err != nil {
		t.Fatalf("capture: %v", err)
	}
	vr := cv.Validate(ctx, ActionRequest{Type: ActionEditLines, Target: path}, ActionResult{Success: true})
	if vr.Verified {
		t.Fatalf("unchanged file verified after capture: %+v", vr)
	}
	if !strings.Contains(vr.Error, "hash unchanged") {
		t.Fatalf("Error=%q, want the unchanged-hash verdict", vr.Error)
	}

	// Part 2: executeAction captures before running the edit.
	vs, editDir := lineEditStore(t)
	vs.SetKernel(setupMockKernel(t)) // nerd.md gate fails closed without one
	name := writeLinesFile(t, editDir, "e.go", []string{"package e", "", "func F() {}"})
	abs := filepath.Join(editDir, name)
	_, err := vs.executeAction(ctx, ActionRequest{
		ActionID: "cap-1", Type: ActionEditLines, Target: name,
		Payload: map[string]any{"start_line": 1, "end_line": 1, "content": "package edited"},
	})
	if err != nil {
		t.Fatalf("executeAction: %v", err)
	}
	found := false
	for _, validator := range vs.validators.Validators() {
		if c, ok := validator.(*CodeDOMValidator); ok {
			found = true
			if h := c.preEditHashes[abs]; h == "" {
				t.Fatalf("no pre-edit snapshot for %s after executeAction", abs)
			}
		}
	}
	if !found {
		t.Fatal("no CodeDOMValidator in the store registry")
	}
}

// TestEnhancedEditValidator_CRLFAndExpansion pins LF-space comparison: a
// CRLF working copy with LF payload must verify, and an edit that expands
// text (old inside new) must not trip the old-absence checks. A genuine
// mismatch must still fail.
func TestEnhancedEditValidator_CRLFAndExpansion(t *testing.T) {
	ctx := context.Background()
	v := NewEnhancedEditValidator()

	t.Run("crlf", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "c.txt")
		// File on disk is CRLF; payload carries LF text.
		if err := os.WriteFile(path, []byte("alpha\r\nBETA\r\ngamma\r\n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		vr := v.Validate(ctx, ActionRequest{Type: ActionEditFile, Target: path,
			Payload: map[string]any{"old": "beta", "new": "BETA"}}, ActionResult{Success: true})
		if !vr.Verified {
			t.Fatalf("CRLF edit failed validation: %q (%+v)", vr.Error, vr.Details)
		}
	})

	t.Run("expansion", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "e.txt")
		if err := os.WriteFile(path, []byte("type UserService struct{}\n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		vr := v.Validate(ctx, ActionRequest{Type: ActionEditFile, Target: path,
			Payload: map[string]any{"old": "User", "new": "UserService"}}, ActionResult{Success: true})
		if !vr.Verified {
			t.Fatalf("expanding edit failed validation: %q (%+v)", vr.Error, vr.Details)
		}
	})

	t.Run("genuine mismatch still fails", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "m.txt")
		if err := os.WriteFile(path, []byte("nothing relevant\n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		vr := v.Validate(ctx, ActionRequest{Type: ActionEditFile, Target: path,
			Payload: map[string]any{"old": "aaa", "new": "bbb"}}, ActionResult{Success: true})
		if vr.Verified {
			t.Fatalf("mismatched edit verified: %+v", vr)
		}
	})
}
