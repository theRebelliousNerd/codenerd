package core

import (
	"context"
	"strings"
	"testing"
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
