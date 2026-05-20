package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tactile"

	"github.com/google/mangle/analysis"
)

// Mock implementation of types.Kernel interface for precise testing
type mockWorkflowKernel struct {
	loadFactsFunc                 func([]Fact) error
	queryFunc                     func(predicate string) ([]Fact, error)
	queryAllFunc                  func() (map[string][]Fact, error)
	assertFunc                    func(Fact) error
	assertBatchFunc               func([]Fact) error
	retractFunc                   func(string) error
	retractFactFunc               func(Fact) error
	updateSystemFactsFunc         func() error
	getProgramInfoFunc            func() *analysis.ProgramInfo
	resetFunc                     func()
	appendPolicyFunc              func(string)
	retractExactFactsBatchFunc    func([]Fact) error
	removeFactsByPredicateSetFunc func(map[string]struct{}) error
}

func (m *mockWorkflowKernel) LoadFacts(facts []Fact) error {
	if m.loadFactsFunc != nil {
		return m.loadFactsFunc(facts)
	}
	return nil
}

func (m *mockWorkflowKernel) Query(predicate string) ([]Fact, error) {
	if m.queryFunc != nil {
		return m.queryFunc(predicate)
	}
	return nil, nil
}

func (m *mockWorkflowKernel) QueryAll() (map[string][]Fact, error) {
	if m.queryAllFunc != nil {
		return m.queryAllFunc()
	}
	return nil, nil
}

func (m *mockWorkflowKernel) Assert(fact Fact) error {
	if m.assertFunc != nil {
		return m.assertFunc(fact)
	}
	return nil
}

func (m *mockWorkflowKernel) AssertBatch(facts []Fact) error {
	if m.assertBatchFunc != nil {
		return m.assertBatchFunc(facts)
	}
	return nil
}

func (m *mockWorkflowKernel) Retract(predicate string) error {
	if m.retractFunc != nil {
		return m.retractFunc(predicate)
	}
	return nil
}

func (m *mockWorkflowKernel) RetractFact(fact Fact) error {
	if m.retractFactFunc != nil {
		return m.retractFactFunc(fact)
	}
	return nil
}

func (m *mockWorkflowKernel) UpdateSystemFacts() error {
	if m.updateSystemFactsFunc != nil {
		return m.updateSystemFactsFunc()
	}
	return nil
}

func (m *mockWorkflowKernel) GetProgramInfo() *analysis.ProgramInfo {
	if m.getProgramInfoFunc != nil {
		return m.getProgramInfoFunc()
	}
	return nil
}

func (m *mockWorkflowKernel) Reset() {
	if m.resetFunc != nil {
		m.resetFunc()
	}
}

func (m *mockWorkflowKernel) AppendPolicy(policy string) {
	if m.appendPolicyFunc != nil {
		m.appendPolicyFunc(policy)
	}
}

func (m *mockWorkflowKernel) RetractExactFactsBatch(facts []Fact) error {
	if m.retractExactFactsBatchFunc != nil {
		return m.retractExactFactsBatchFunc(facts)
	}
	return nil
}

func (m *mockWorkflowKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	if m.removeFactsByPredicateSetFunc != nil {
		return m.removeFactsByPredicateSetFunc(predicates)
	}
	return nil
}

// Mock implementation of tactile.Executor for testing commands
type mockWorkflowExecutor struct {
	executeFunc func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error)
}

func (m *mockWorkflowExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, cmd)
	}
	return &tactile.ExecutionResult{
		Success:  true,
		ExitCode: 0,
		Stdout:   "mock stdout",
		Stderr:   "",
	}, nil
}

func (m *mockWorkflowExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (m *mockWorkflowExecutor) Validate(cmd tactile.Command) error {
	return nil
}

// Mock implementation of ToolGenerator for Ouroboros testing
type mockWorkflowToolGenerator struct {
	generateToolFromCodeFunc func(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (bool, string, string, string)
}

func (m *mockWorkflowToolGenerator) GenerateToolFromCode(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (bool, string, string, string) {
	if m.generateToolFromCodeFunc != nil {
		return m.generateToolFromCodeFunc(ctx, name, purpose, code, confidence, priority, isDiagnostic)
	}
	return true, name, "/bin/" + name, ""
}

// Mock implementation of TaskDelegator
type mockWorkflowTaskDelegator struct {
	executeFunc func(ctx context.Context, intent string, task string) (string, error)
}

func (m *mockWorkflowTaskDelegator) Execute(ctx context.Context, intent string, task string) (string, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, intent, task)
	}
	return "mock delegation result", nil
}

// Mock implementation of IntegrationClient
type mockWorkflowIntegrationClient struct {
	callToolFunc func(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error)
}

func (m *mockWorkflowIntegrationClient) CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error) {
	if m.callToolFunc != nil {
		return m.callToolFunc(ctx, tool, args)
	}
	return "mock integration result", nil
}

func TestVirtualStoreWorkflows_TDDLoop(t *testing.T) {
	tmpDir := t.TempDir()
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.workingDir = tmpDir

	ctx := context.Background()

	// 1. handleReadErrorLog - Test log not found
	req := ActionRequest{Target: "test"}
	res, err := vs.handleReadErrorLog(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success=true even if log not found")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "error_log_empty" {
		t.Errorf("expected error_log_empty fact, got: %v", res.FactsToAdd)
	}

	// Create dummy log files
	logDir := filepath.Join(tmpDir, ".nerd", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	testLogPath := filepath.Join(logDir, "test.log")
	if err := os.WriteFile(testLogPath, []byte("test log contents"), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}
	buildLogPath := filepath.Join(logDir, "build.log")
	if err := os.WriteFile(buildLogPath, []byte("build log contents"), 0644); err != nil {
		t.Fatalf("failed to write build log: %v", err)
	}

	// Target: "test" - Found
	res, err = vs.handleReadErrorLog(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "test log contents" {
		t.Errorf("expected test log contents, got output: %q", res.Output)
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "error_log_read" || res.FactsToAdd[1].Predicate != "test_state" {
		t.Errorf("expected error_log_read and test_state facts, got: %v", res.FactsToAdd)
	}

	// Target: "build" - Found
	req.Target = "build"
	res, err = vs.handleReadErrorLog(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "build log contents" {
		t.Errorf("expected build log contents, got output: %q", res.Output)
	}

	// Target: specific path
	otherLogPath := filepath.Join(tmpDir, "custom.log")
	if err := os.WriteFile(otherLogPath, []byte("custom log contents"), 0644); err != nil {
		t.Fatalf("failed to write custom log: %v", err)
	}
	req.Target = "custom.log"
	res, err = vs.handleReadErrorLog(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "custom log contents" {
		t.Errorf("expected custom log contents, got output: %q", res.Output)
	}

	// Target: empty (defaults to "test")
	req.Target = ""
	res, err = vs.handleReadErrorLog(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "test log contents" {
		t.Errorf("expected test log contents as default, got output: %q", res.Output)
	}

	// Context canceled
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleReadErrorLog(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 2. handleAnalyzeRootCause
	req = ActionRequest{Target: "panic at line 45"}
	res, err = vs.handleAnalyzeRootCause(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success=true")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "analyzing_root_cause" || res.FactsToAdd[1].Predicate != "tdd_phase" {
		t.Errorf("expected analyzing_root_cause and tdd_phase facts, got: %v", res.FactsToAdd)
	}

	req.Target = ""
	res, err = vs.handleAnalyzeRootCause(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "unknown") {
		t.Errorf("expected default target 'unknown' to be used, got: %q", res.Output)
	}

	res, err = vs.handleAnalyzeRootCause(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 3. handleGeneratePatch
	req = ActionRequest{Target: "file.go", Payload: map[string]interface{}{"description": "fix syntax error"}}
	res, err = vs.handleGeneratePatch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success=true")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "generating_patch" || res.FactsToAdd[1].Predicate != "tdd_phase" {
		t.Errorf("expected generating_patch and tdd_phase facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleGeneratePatch(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 4. handleEscalateToUser
	req = ActionRequest{Target: "no test file found"}
	res, err = vs.handleEscalateToUser(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "USER_INTERVENTION_REQUIRED" {
		t.Errorf("expected escalation failure, got: %+v", res)
	}

	res, err = vs.handleEscalateToUser(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 5. handleComplete
	req = ActionRequest{Target: "task_123", Payload: map[string]interface{}{"summary": "all tests pass now"}}
	res, err = vs.handleComplete(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected complete to succeed")
	}
	if len(res.FactsToAdd) != 3 || res.FactsToAdd[0].Predicate != "task_completed" || res.FactsToAdd[2].Predicate != "observation" {
		t.Errorf("expected complete facts, got: %v", res.FactsToAdd)
	}

	req.Payload = nil
	res, err = vs.handleComplete(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.FactsToAdd) != 2 {
		t.Errorf("expected 2 complete facts when summary is empty, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleComplete(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 6. handleInterrogative
	req = ActionRequest{Target: "do you want dark mode?", Payload: map[string]interface{}{"options": []interface{}{"yes", "no"}}}
	res, err = vs.handleInterrogative(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "CLARIFICATION_NEEDED" {
		t.Errorf("expected interrogative failure, got: %+v", res)
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "awaiting_clarification" {
		t.Errorf("expected interrogative facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleInterrogative(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 7. handleResumeTask
	req = ActionRequest{Target: "task_123"}
	res, err = vs.handleResumeTask(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected resume success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "task_resumed" {
		t.Errorf("expected resume facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleResumeTask(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 8. handleRefreshShardContext
	// Case 8a: kernel nil
	vs.kernel = nil
	res, err = vs.handleRefreshShardContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "kernel not attached" {
		t.Errorf("expected kernel not attached error, got: %+v", res)
	}

	// Case 8b: Query error
	mKernel := &mockWorkflowKernel{
		queryFunc: func(predicate string) ([]Fact, error) {
			return nil, errors.New("query failed")
		},
	}
	vs.kernel = mKernel
	res, err = vs.handleRefreshShardContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "query failed" {
		t.Errorf("expected query failed error, got: %+v", res)
	}

	// Case 8c: Happy path with stale facts (refreshes all)
	mKernel.queryFunc = func(predicate string) ([]Fact, error) {
		if predicate == "context_stale" {
			return []Fact{
				{Predicate: "context_stale", Args: []interface{}{"coder", "file_topology"}},
				{Predicate: "context_stale", Args: []interface{}{"tester", "test_output"}},
			}, nil
		}
		return nil, nil
	}
	req.Target = ""
	res, err = vs.handleRefreshShardContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "Refreshed 2 stale shard context atoms") {
		t.Errorf("expected 2 refreshed atoms, got output: %q", res.Output)
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "shard_context_refreshed" {
		t.Errorf("expected shard_context_refreshed facts, got: %v", res.FactsToAdd)
	}

	// Case 8d: Happy path with stale facts and shard filter
	req.Target = "coder"
	res, err = vs.handleRefreshShardContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "Refreshed 1 stale shard context atoms for coder") {
		t.Errorf("expected 1 refreshed atom for coder, got output: %q", res.Output)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "shard_context_refreshed" {
		t.Errorf("expected shard_context_refreshed facts, got: %v", res.FactsToAdd)
	}

	// Case 8e: No stale facts
	mKernel.queryFunc = func(predicate string) ([]Fact, error) {
		return nil, nil
	}
	req.Target = ""
	res, err = vs.handleRefreshShardContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "No stale shard context atoms detected" {
		t.Errorf("expected no stale facts output, got output: %q", res.Output)
	}

	req.Target = "coder"
	res, err = vs.handleRefreshShardContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "No stale shard context atoms detected for coder" {
		t.Errorf("expected no stale facts for coder output, got output: %q", res.Output)
	}

	// Context canceled
	res, err = vs.handleRefreshShardContext(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreWorkflows_Ouroboros(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	ctx := context.Background()

	// 1. handleGenerateTool
	// Case 1a: generator nil
	req := ActionRequest{Target: "my_tool"}
	res, err := vs.handleGenerateTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "tool generator not configured" {
		t.Errorf("expected no generator error, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "tool_generation_failed" {
		t.Errorf("expected tool_generation_failed fact, got: %v", res.FactsToAdd)
	}

	// Setup mock tool generator
	mGen := &mockWorkflowToolGenerator{}
	vs.toolGenerator = mGen

	// Case 1b: Generator succeeds
	req.Payload = map[string]interface{}{
		"purpose":       "testing tool generation",
		"code":          "func main() {}",
		"confidence":    0.9,
		"priority":      3.0,
		"is_diagnostic": true,
	}
	res, err = vs.handleGenerateTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["tool_name"] != "my_tool" {
		t.Errorf("expected my_tool success, got res: %+v", res)
	}
	if len(res.FactsToAdd) != 3 || res.FactsToAdd[0].Predicate != "tool_generated" {
		t.Errorf("expected tool_generated facts, got: %v", res.FactsToAdd)
	}

	// Case 1c: Generator fails
	mGen.generateToolFromCodeFunc = func(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (bool, string, string, string) {
		return false, "", "", "compilation failed"
	}
	res, err = vs.handleGenerateTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "compilation failed" {
		t.Errorf("expected generator fail, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "tool_generation_failed" {
		t.Errorf("expected tool_generation_failed fact, got: %v", res.FactsToAdd)
	}

	// Context canceled
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleGenerateTool(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 2. handleOuroborosDetect
	req = ActionRequest{Target: "user intent /fix"}
	res, err = vs.handleOuroborosDetect(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "ouroboros_phase" || res.FactsToAdd[1].Predicate != "tool_detection_context" {
		t.Errorf("expected ouroboros detect facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleOuroborosDetect(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 3. handleOuroborosGenerate
	req = ActionRequest{Target: "my_tool"}
	res, err = vs.handleOuroborosGenerate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "ouroboros_phase" || res.FactsToAdd[1].Predicate != "tool_generating" {
		t.Errorf("expected ouroboros generate facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleOuroborosGenerate(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 4. handleOuroborosCompile
	// Case 4a: compile succeeds via taskDelegator delegation
	mDelegator := &mockWorkflowTaskDelegator{
		executeFunc: func(ctx context.Context, intent string, task string) (string, error) {
			if intent != "/tool_generator" {
				return "", fmt.Errorf("unexpected intent: %s", intent)
			}
			return "compile success", nil
		}	}
	vs.taskDelegator = mDelegator

	req = ActionRequest{Target: "my_tool", Payload: map[string]interface{}{"source_path": "my_tool.go"}}
	res, err = vs.handleOuroborosCompile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "compile success" {
		t.Errorf("expected compile success, got: %+v", res)
	}

	// Context canceled
	res, err = vs.handleOuroborosCompile(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 5. handleOuroborosRegister
	// Case 5a: registry nil
	vs.toolRegistry = nil
	req = ActionRequest{Target: "my_tool", Payload: map[string]interface{}{"binary_path": "/bin/my_tool", "shard_affinity": "coder"}}
	res, err = vs.handleOuroborosRegister(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "tool registry not initialized" {
		t.Errorf("expected registry nil error, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "ouroboros_register_failed" {
		t.Errorf("expected registration failure fact, got: %v", res.FactsToAdd)
	}

	// Case 5b: Registration success
	vs.toolRegistry = NewToolRegistry(t.TempDir())
	res, err = vs.handleOuroborosRegister(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected register success, got error: %s", res.Error)
	}
	if len(res.FactsToAdd) != 4 || res.FactsToAdd[0].Predicate != "ouroboros_phase" || res.FactsToAdd[1].Predicate != "tool_registered" {
		t.Errorf("expected tool registered facts, got: %v", res.FactsToAdd)
	}

	// Case 5c: Registration success with empty shard affinity (defaults to "coder")
	req.Payload = map[string]interface{}{"binary_path": "/bin/my_tool"}
	res, err = vs.handleOuroborosRegister(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected register success, got error: %s", res.Error)
	}

	// Context canceled
	res, err = vs.handleOuroborosRegister(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 6. handleRefineTool
	req = ActionRequest{Target: "my_tool", Payload: map[string]interface{}{"feedback": "fix panic"}}
	res, err = vs.handleRefineTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "tool_refining" {
		t.Errorf("expected tool refining facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleRefineTool(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreWorkflows_Campaign(t *testing.T) {
	tmpDir := t.TempDir()
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.workingDir = tmpDir

	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. handleCampaignClarify
	req := ActionRequest{Target: "what is the base branch?", Payload: map[string]interface{}{"campaign_id": "c1"}}
	res, err := vs.handleCampaignClarify(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "CAMPAIGN_CLARIFICATION_NEEDED" {
		t.Errorf("expected CAMPAIGN_CLARIFICATION_NEEDED error, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "campaign_awaiting_clarification" {
		t.Errorf("expected campaign_awaiting_clarification fact, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignClarify(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 2. handleCampaignCreateFile (delegates to handleWriteFile)
	req = ActionRequest{Target: "newfile.txt", Payload: map[string]interface{}{"content": "new contents"}}
	res, err = vs.handleCampaignCreateFile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected create file success, got: %+v", res)
	}

	// 3. handleCampaignModifyFile (delegates to handleEditFile)
	req = ActionRequest{Target: "newfile.txt", Payload: map[string]interface{}{"old": "contents", "new": "stuff"}}
	res, err = vs.handleCampaignModifyFile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected edit file success, got: %+v", res)
	}

	// 4. handleCampaignWriteTest
	req = ActionRequest{Target: "newfile_test.txt", Payload: map[string]interface{}{"content": "test contents"}}
	res, err = vs.handleCampaignWriteTest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected write test success, got: %+v", res)
	}
	// Verify test_written fact was added
	var testWritten bool
	for _, f := range res.FactsToAdd {
		if f.Predicate == "test_written" && f.Args[0] == "newfile_test.txt" {
			testWritten = true
		}
	}
	if !testWritten {
		t.Errorf("expected test_written fact to be present, got: %v", res.FactsToAdd)
	}

	// Write test failure path (write to invalid dir)
	req.Target = "invalid/dir/newfile_test.txt"
	// Create a read-only or blocked path context by writing a file first so MkdirAll fails or mock it.
	// Actually, just making MkdirAll fail by creating a file where the directory should be!
	blockedDir := filepath.Join(tmpDir, "blocked_dir")
	if err := os.WriteFile(blockedDir, []byte("file"), 0644); err != nil {
		t.Fatalf("failed to create blocked dir file: %v", err)
	}
	req.Target = filepath.Join("blocked_dir", "test.txt")
	res, err = vs.handleCampaignWriteTest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success {
		t.Errorf("expected failure due to blocked path creation")
	}

	res, err = vs.handleCampaignWriteTest(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 5. handleCampaignRunTest (delegates to handleRunTests)
	mExec := &mockWorkflowExecutor{}
	vs.executor = mExec
	req = ActionRequest{Target: "go test ./...", Payload: map[string]interface{}{"timeout": 10}}
	res, err = vs.handleCampaignRunTest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected run tests success, got: %+v", res)
	}

	// 6. handleCampaignResearch (delegates to handleResearch)
	// Case 6a: modular tools registry nil
	vs.modularTools = nil
	res, err = vs.handleCampaignResearch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "modular tools registry not initialized" {
		t.Errorf("expected modular tools registry nil error, got: %+v", res)
	}

	// 7. handleCampaignVerify
	req = ActionRequest{Target: "step_1", Payload: map[string]interface{}{"campaign_id": "c1"}}
	res, err = vs.handleCampaignVerify(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_step_verifying" {
		t.Errorf("expected campaign verify facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignVerify(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 8. handleCampaignDocument
	// Case 8a: content empty (requests documentation)
	req = ActionRequest{Target: "docs/readme.md"}
	res, err = vs.handleCampaignDocument(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_documenting" {
		t.Errorf("expected campaign documenting facts, got: %v", res.FactsToAdd)
	}

	// Case 8b: content present (writes documentation file)
	req.Payload = map[string]interface{}{"content": "document contents"}
	res, err = vs.handleCampaignDocument(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected write success")
	}

	res, err = vs.handleCampaignDocument(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 9. handleCampaignRefactor
	req = ActionRequest{Target: "auth.go", Payload: map[string]interface{}{"refactor_type": "inline"}}
	res, err = vs.handleCampaignRefactor(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_refactoring" {
		t.Errorf("expected campaign refactoring facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignRefactor(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 10. handleCampaignIntegrate
	req = ActionRequest{Target: "auth", Payload: map[string]interface{}{"campaign_id": "c1"}}
	res, err = vs.handleCampaignIntegrate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_integrating" {
		t.Errorf("expected campaign integrating facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignIntegrate(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 11. handleCampaignComplete
	req = ActionRequest{Target: "c1", Payload: map[string]interface{}{"summary": "done"}}
	res, err = vs.handleCampaignComplete(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_completed" {
		t.Errorf("expected campaign completed facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignComplete(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 12. handleCampaignFinalVerify
	req = ActionRequest{Target: "c1"}
	res, err = vs.handleCampaignFinalVerify(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_final_verifying" {
		t.Errorf("expected campaign final verify facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignFinalVerify(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 13. handleCampaignCleanup
	req = ActionRequest{Target: "c1"}
	res, err = vs.handleCampaignCleanup(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_cleaned_up" {
		t.Errorf("expected campaign cleanup facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCampaignCleanup(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 14. handleArchiveCampaign
	req = ActionRequest{Target: "c1"}
	res, err = vs.handleArchiveCampaign(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "campaign_archived" {
		t.Errorf("expected campaign archived facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleArchiveCampaign(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 15. handleShowCampaignStatus
	req = ActionRequest{Target: "c1"}
	res, err = vs.handleShowCampaignStatus(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "campaign_status_requested" {
		t.Errorf("expected campaign status facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleShowCampaignStatus(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 16. handleShowCampaignProgress
	req = ActionRequest{Target: "c1"}
	res, err = vs.handleShowCampaignProgress(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "campaign_progress_requested" {
		t.Errorf("expected campaign progress facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleShowCampaignProgress(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 17. handleAskCampaignInterrupt
	req = ActionRequest{Target: "c1", Payload: map[string]interface{}{"reason": "user requested"}}
	res, err = vs.handleAskCampaignInterrupt(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "CAMPAIGN_INTERRUPT_REQUESTED" {
		t.Errorf("expected interrupt failure, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "campaign_interrupt_requested" {
		t.Errorf("expected campaign interrupt facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleAskCampaignInterrupt(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 18. handleRunPhaseCheckpoint
	req = ActionRequest{Target: "p1", Payload: map[string]interface{}{"campaign_id": "c1"}}
	res, err = vs.handleRunPhaseCheckpoint(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "phase_checkpoint" {
		t.Errorf("expected phase_checkpoint fact, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleRunPhaseCheckpoint(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 19. handlePauseAndReplan
	req = ActionRequest{Target: "c1", Payload: map[string]interface{}{"reason": "unreachable goal"}}
	res, err = vs.handlePauseAndReplan(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "campaign_paused" {
		t.Errorf("expected campaign pause facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handlePauseAndReplan(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreWorkflows_ContextManagement(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. handleCompressContext
	// Case 1a: ratio defaults
	req := ActionRequest{Target: "token budget limit reached"}
	res, err := vs.handleCompressContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "context_compressing" {
		t.Errorf("expected context compressing facts, got: %v", res.FactsToAdd)
	}
	// Verify ratio default 0.5
	if res.FactsToAdd[0].Args[1] != 0.5 {
		t.Errorf("expected default ratio 0.5, got: %v", res.FactsToAdd[0].Args[1])
	}

	// Case 1b: ratio specified
	req.Payload = map[string]interface{}{"ratio": 0.8}
	res, err = vs.handleCompressContext(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FactsToAdd[0].Args[1] != 0.8 {
		t.Errorf("expected ratio 0.8, got: %v", res.FactsToAdd[0].Args[1])
	}

	res, err = vs.handleCompressContext(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 2. handleEmergencyCompress
	req = ActionRequest{SessionID: "sess_1"}
	res, err = vs.handleEmergencyCompress(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "context_compressing" || res.FactsToAdd[1].Predicate != "compression_requested" {
		t.Errorf("expected emergency compressing facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleEmergencyCompress(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 3. handleCreateCheckpoint
	// Case 3a: name empty (defaults to timestamp name)
	req = ActionRequest{}
	res, err = vs.handleCreateCheckpoint(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "checkpoint_created" {
		t.Errorf("expected checkpoint_created fact, got: %v", res.FactsToAdd)
	}

	// Case 3b: name specified
	req.Target = "my_checkpoint"
	res, err = vs.handleCreateCheckpoint(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["checkpoint_name"] != "my_checkpoint" {
		t.Errorf("expected my_checkpoint metadata, got: %+v", res)
	}

	res, err = vs.handleCreateCheckpoint(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreWorkflows_InvestigationAndCorrective(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. handleInvestigateAnomaly
	// Case 1a: severity empty (defaults to "medium")
	req := ActionRequest{Target: "test failure count spiking"}
	res, err := vs.handleInvestigateAnomaly(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "anomaly_investigating" {
		t.Errorf("expected anomaly facts, got: %v", res.FactsToAdd)
	}
	if res.FactsToAdd[0].Args[1] != "medium" {
		t.Errorf("expected severity 'medium', got: %v", res.FactsToAdd[0].Args[1])
	}

	// Case 1b: severity specified
	req.Payload = map[string]interface{}{"severity": "high"}
	res, err = vs.handleInvestigateAnomaly(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FactsToAdd[0].Args[1] != "high" {
		t.Errorf("expected severity 'high', got: %v", res.FactsToAdd[0].Args[1])
	}

	res, err = vs.handleInvestigateAnomaly(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 2. handleInvestigateSystemic
	req = ActionRequest{Target: "intermittent db lock timeouts"}
	res, err = vs.handleInvestigateSystemic(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "systemic_investigating" {
		t.Errorf("expected systemic facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleInvestigateSystemic(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 3. handleUpdateWorldModel
	req = ActionRequest{Target: "file_hashes", Payload: map[string]interface{}{"scope": "internal/core"}}
	res, err = vs.handleUpdateWorldModel(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "world_model_updating" {
		t.Errorf("expected world model updating facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleUpdateWorldModel(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 4. handleCorrectiveResearch
	// Case 4a: scraper not registered (falls back to not calling handleResearch, returns success=true signal)
	req = ActionRequest{Target: "how to implement FFI in mangle", Payload: map[string]interface{}{"issue_type": "build_error"}}
	res, err = vs.handleCorrectiveResearch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "corrective_researching" {
		t.Errorf("expected corrective_researching facts, got: %v", res.FactsToAdd)
	}

	// Case 4b: scraper client registered but modularTools is nil (calls handleResearch which fails on modularTools nil)
	vs.mcpClients = map[string]IntegrationClient{"scraper": &mockWorkflowIntegrationClient{}}
	vs.modularTools = nil
	res, err = vs.handleCorrectiveResearch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "modular tools registry not initialized" {
		t.Errorf("expected modular tools nil failure, got: %+v", res)
	}

	res, err = vs.handleCorrectiveResearch(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 5. handleCorrectiveDocs
	req = ActionRequest{Target: "docs/architecture.md"}
	res, err = vs.handleCorrectiveDocs(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "corrective_documenting" {
		t.Errorf("expected corrective documenting facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCorrectiveDocs(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}

	// 6. handleCorrectiveDecompose
	req = ActionRequest{Target: "unresolved symbols in build link"}
	res, err = vs.handleCorrectiveDecompose(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success")
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[0].Predicate != "corrective_decomposing" {
		t.Errorf("expected corrective decomposing facts, got: %v", res.FactsToAdd)
	}

	res, err = vs.handleCorrectiveDecompose(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}
