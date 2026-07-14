package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
)

// Mock implementation of types.Kernel
type mockActionsKernel struct {
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

func (m *mockActionsKernel) LoadFacts(facts []Fact) error {
	if m.loadFactsFunc != nil {
		return m.loadFactsFunc(facts)
	}
	return nil
}

func (m *mockActionsKernel) Query(predicate string) ([]Fact, error) {
	if m.queryFunc != nil {
		return m.queryFunc(predicate)
	}
	return nil, nil
}

func (m *mockActionsKernel) QueryAll() (map[string][]Fact, error) {
	if m.queryAllFunc != nil {
		return m.queryAllFunc()
	}
	return nil, nil
}

func (m *mockActionsKernel) Assert(fact Fact) error {
	if m.assertFunc != nil {
		return m.assertFunc(fact)
	}
	return nil
}

func (m *mockActionsKernel) AssertBatch(facts []Fact) error {
	if m.assertBatchFunc != nil {
		return m.assertBatchFunc(facts)
	}
	return nil
}

func (m *mockActionsKernel) Retract(predicate string) error {
	if m.retractFunc != nil {
		return m.retractFunc(predicate)
	}
	return nil
}

func (m *mockActionsKernel) RetractFact(fact Fact) error {
	if m.retractFactFunc != nil {
		return m.retractFactFunc(fact)
	}
	return nil
}

func (m *mockActionsKernel) UpdateSystemFacts() error {
	if m.updateSystemFactsFunc != nil {
		return m.updateSystemFactsFunc()
	}
	return nil
}

func (m *mockActionsKernel) GetProgramInfo() *analysis.ProgramInfo {
	if m.getProgramInfoFunc != nil {
		return m.getProgramInfoFunc()
	}
	return nil
}

func (m *mockActionsKernel) Reset() {
	if m.resetFunc != nil {
		m.resetFunc()
	}
}

func (m *mockActionsKernel) AppendPolicy(policy string) {
	if m.appendPolicyFunc != nil {
		m.appendPolicyFunc(policy)
	}
}

func (m *mockActionsKernel) RetractExactFactsBatch(facts []Fact) error {
	if m.retractExactFactsBatchFunc != nil {
		return m.retractExactFactsBatchFunc(facts)
	}
	return nil
}

func (m *mockActionsKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	if m.removeFactsByPredicateSetFunc != nil {
		return m.removeFactsByPredicateSetFunc(predicates)
	}
	return nil
}

// Mock tactile.Executor
type mockActionsExecutor struct {
	executeFunc func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error)
}

func (m *mockActionsExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.executeFunc != nil {
		return m.executeFunc(ctx, cmd)
	}
	return &tactile.ExecutionResult{
		Success:  true,
		ExitCode: 0,
		Stdout:   "mock stdout",
		Stderr:   "mock stderr",
	}, nil
}

func (m *mockActionsExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{
		Name: "mock-executor",
	}
}

func (m *mockActionsExecutor) Validate(cmd tactile.Command) error {
	return nil
}

// Mock TaskDelegator
type mockActionsTaskDelegator struct {
	executeFunc func(ctx context.Context, intent string, task string) (string, error)
}

func (m *mockActionsTaskDelegator) Execute(ctx context.Context, intent string, task string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if m.executeFunc != nil {
		return m.executeFunc(ctx, intent, task)
	}
	return "mock delegation success", nil
}

// Mock LLMClient conforming to types.LLMClient
type mockActionsLLMClient struct {
	completeFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *mockActionsLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, prompt)
	}
	return "mock llm response", nil
}

func (m *mockActionsLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "mock llm response", nil
}

func (m *mockActionsLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	return nil, nil
}

func (m *mockActionsLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}

// Test Helper for basic VirtualStore
func createActionsTestVS(t *testing.T) (*VirtualStore, string) {
	tmpDir := t.TempDir()
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.workingDir = tmpDir
	mExec := &mockActionsExecutor{}
	vs.executor = mExec
	vs.modernExecutor = mExec
	vs.useModernExecutor = false
	vs.DisableBootGuard()
	return vs, tmpDir
}

// TestExec verifies the shell execution helper function
func TestExec(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	// Happy path
	stdout, stderr, err := vs.Exec(ctx, "echo hello", []string{"KEY=val"})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if stdout != "mock stdout" {
		t.Errorf("expected mock stdout, got %q", stdout)
	}
	if stderr != "mock stderr" {
		t.Errorf("expected mock stderr, got %q", stderr)
	}

	// Execution error
	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return nil, errors.New("exec error")
	}
	_, _, err = vs.Exec(ctx, "echo hello", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Canceled context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = vs.Exec(cCtx, "echo hello", nil)
	if err == nil {
		t.Fatal("expected cancel error, got nil")
	}
}

// TestHandleExecCmd tests basic shell execution handler
func TestHandleExecCmd(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	// 1. Success execution
	req := ActionRequest{
		ActionID: "a1",
		Target:   "go test",
		Payload: map[string]any{
			"env":             []any{"GO111MODULE=on"},
			"timeout_seconds": 30,
		},
	}
	res, err := vs.handleExecCmd(ctx, req)
	if err != nil {
		t.Fatalf("handleExecCmd failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}
	if res.Output != "mock stdout\nmock stderr" {
		t.Errorf("expected mock stdout\\nmock stderr, got %q", res.Output)
	}

	// 2. Allowed binaries verification - Forbidden binary
	vs.allowedBinaries = []string{"go", "git"}
	req.Payload["binary"] = "rm"
	res, err = vs.handleExecCmd(ctx, req)
	if err != nil {
		t.Fatalf("handleExecCmd failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "not allowed") {
		t.Errorf("expected binary not allowed error, got: %+v", res)
	}
	vs.allowedBinaries = []string{"go", "git", "bash"}

	// 3. Executor execution failure (Exit code non-zero or error returned)
	delete(req.Payload, "binary")
	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return nil, fmt.Errorf("command execution error")
	}
	res, err = vs.handleExecCmd(ctx, req)
	if err != nil {
		t.Fatalf("handleExecCmd failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "command execution error") {
		t.Errorf("expected command execution failure, got success: %+v", res)
	}

	// 4. Custom working directory checks (legacy path uses vs.workingDir)
	req.Payload["cwd"] = "sub_dir"
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		if cmd.WorkingDirectory != vs.workingDir {
			return nil, fmt.Errorf("expected cmd.WorkingDirectory to equal vs.workingDir %q, got %q", vs.workingDir, cmd.WorkingDirectory)
		}
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "cwd matches"}, nil
	}
	res, err = vs.handleExecCmd(ctx, req)
	if err != nil {
		t.Fatalf("handleExecCmd failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got error: %s", res.Error)
	}

	// 5. Canceled context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleExecCmd(cCtx, req)
	if err != nil {
		t.Fatalf("handleExecCmd failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleExecCmd_NonZeroExitReportsFailure is the deterministic regression
// for F-VS-2. tactile.DirectExecutor returns (result{Success:true, ExitCode:N≠0},
// nil error) when a command RAN but exited non-zero. The legacy handleExecCmd
// keyed success off err==nil alone, so it reported Success:true and asserted a
// false cmd_succeeded fact into the kernel for a failed command. The fix
// requires a zero exit code, mirroring handleExecCmdModern / handleGitOperation.
func TestHandleExecCmd_NonZeroExitReportsFailure(t *testing.T) {
	vs, _ := createActionsTestVS(t) // legacy path: useModernExecutor=false
	vs.allowedBinaries = []string{"go", "git", "bash"}
	ctx := context.Background()

	mExec := vs.executor.(*mockActionsExecutor)
	// Ran to completion, exited non-zero, nil error — DirectExecutor's contract.
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{Success: true, ExitCode: 1, Stderr: "boom"}, nil
	}

	req := ActionRequest{ActionID: "a1", Target: "go test"}
	res, err := vs.handleExecCmd(ctx, req)
	if err != nil {
		t.Fatalf("handleExecCmd returned err: %v", err)
	}
	if res.Success {
		t.Errorf("expected Success=false for exit code 1, got %+v", res)
	}
	for _, f := range res.FactsToAdd {
		if f.Predicate == "cmd_succeeded" {
			t.Errorf("asserted cmd_succeeded for a command that exited non-zero: %+v", res.FactsToAdd)
		}
	}
	// Sanity: exit-0 still succeeds (no behavior change on the correct path).
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "ok"}, nil
	}
	res, err = vs.handleExecCmd(ctx, req)
	if err != nil {
		t.Fatalf("handleExecCmd (exit 0) returned err: %v", err)
	}
	if !res.Success {
		t.Errorf("expected Success=true for exit code 0, got %+v", res)
	}
}

// TestHandleExecCmdModern tests the modern wrapper implementation
func TestHandleExecCmdModern(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "a1",
		Target:   "go build",
	}
	_ = req

	// Default: should route to normal handleExecCmd
	res, err := vs.handleExecCmdModern(ctx, "go", []string{"build"}, 30, "s1", "a1")
	if err != nil {
		t.Fatalf("handleExecCmdModern failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Enable modern executor
	vs.EnableModernExecutor()
	res, err = vs.handleExecCmdModern(ctx, "go", []string{"build"}, 30, "s1", "a1")
	if err != nil {
		t.Fatalf("handleExecCmdModern failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success with modern executor, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleExecCmdModern(cCtx, "go", []string{"build"}, 30, "s1", "a1")
	if err != nil {
		t.Fatalf("handleExecCmdModern failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleReadFile tests file read action
func TestHandleReadFile(t *testing.T) {
	vs, tmpDir := createActionsTestVS(t)
	ctx := context.Background()

	// Write test file
	fileName := "test.txt"
	filePath := filepath.Join(tmpDir, fileName)
	content := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Success reading full file
	req := ActionRequest{
		ActionID: "r1",
		Target:   fileName,
	}
	res, err := vs.handleReadFile(ctx, req)
	if err != nil {
		t.Fatalf("handleReadFile failed: %v", err)
	}
	if !res.Success || res.Output != content {
		t.Errorf("expected full content, got success=%v, output=%q", res.Success, res.Output)
	}

	// 2. File is a directory (should succeed and return directory listing)
	req.Target = "."
	req.Payload = nil
	res, err = vs.handleReadFile(ctx, req)
	if err != nil {
		t.Fatalf("handleReadFile failed: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "test.txt") {
		t.Errorf("expected directory listing containing test.txt, got: %+v", res)
	}

	// 3. File does not exist
	req.Target = "does_not_exist.txt"
	res, err = vs.handleReadFile(ctx, req)
	if err != nil {
		t.Fatalf("handleReadFile failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "does_not_exist.txt") {
		t.Errorf("expected file not found error, got: %+v", res)
	}

	// 6. Test Kernel fact injection on read_file
	mKernel := &mockActionsKernel{}
	vs.kernel = mKernel
	req.Target = fileName
	res, err = vs.handleReadFile(ctx, req)
	if err != nil {
		t.Fatalf("handleReadFile failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got error: %s", res.Error)
	}
	// Verify facts to add contain file_content
	var foundFileContent bool
	for _, f := range res.FactsToAdd {
		if f.Predicate == "file_content" {
			foundFileContent = true
		}
	}
	if !foundFileContent {
		t.Errorf("expected file_content fact in output, got: %+v", res.FactsToAdd)
	}

	// 7. Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleReadFile(cCtx, req)
	if err != nil {
		t.Fatalf("handleReadFile failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleReadDirectory tests directory listing
func TestHandleReadDirectory(t *testing.T) {
	vs, tmpDir := createActionsTestVS(t)
	ctx := context.Background()

	// Set up files inside directory
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "a.txt"), []byte("aaa"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("bbbbb"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	// 1. Basic reading
	req := ActionRequest{
		ActionID: "d1",
		Target:   "sub",
	}
	res, err := vs.handleReadDirectory(ctx, vs.resolvePath(req.Target))
	if err != nil {
		t.Fatalf("handleReadDirectory failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got error: %s", res.Error)
	}
	if !strings.Contains(res.Output, "a.txt (3 bytes)") || !strings.Contains(res.Output, "b.txt (5 bytes)") {
		t.Errorf("expected files in listing output, got %q", res.Output)
	}

	// 2. Reading a file as directory (should fail)
	req.Target = filepath.Join("sub", "a.txt")
	res, err = vs.handleReadDirectory(ctx, vs.resolvePath(req.Target))
	if err != nil {
		t.Fatalf("handleReadDirectory failed: %v", err)
	}
	if res.Success {
		t.Errorf("expected not a directory error, got success: %+v", res)
	}

	// 3. Directory not found
	req.Target = "does_not_exist"
	res, err = vs.handleReadDirectory(ctx, vs.resolvePath(req.Target))
	if err != nil {
		t.Fatalf("handleReadDirectory failed: %v", err)
	}
	if res.Success {
		t.Errorf("expected not found error, got success: %+v", res)
	}

	// 4. Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleReadDirectory(cCtx, vs.resolvePath(req.Target))
	if err != nil {
		t.Fatalf("handleReadDirectory failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleWriteFile tests file creation
func TestHandleWriteFile(t *testing.T) {
	vs, tmpDir := createActionsTestVS(t)
	ctx := context.Background()

	fileName := "new_file.txt"
	req := ActionRequest{
		ActionID: "w1",
		Target:   fileName,
		Payload: map[string]any{
			"content":   "some content",
			"overwrite": true,
		},
	}

	// 1. Happy path: write new file
	res, err := vs.handleWriteFile(ctx, req)
	if err != nil {
		t.Fatalf("handleWriteFile failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Verify file content
	gotBytes, err := os.ReadFile(filepath.Join(tmpDir, fileName))
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(gotBytes) != "some content" {
		t.Errorf("expected 'some content', got %q", string(gotBytes))
	}

	// 2. Write file again (should always succeed/overwrite in production)
	req.Payload["overwrite"] = false
	res, err = vs.handleWriteFile(ctx, req)
	if err != nil {
		t.Fatalf("handleWriteFile failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success overwriting file, got: %+v", res)
	}

	// 3. Payload missing content (returns error in production)
	req.Target = "another.txt"
	req.Payload = nil
	res, err = vs.handleWriteFile(ctx, req)
	if err == nil {
		t.Error("expected error for missing content, got nil")
	}

	// 4. Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleWriteFile(cCtx, req)
	if err != nil {
		t.Fatalf("handleWriteFile failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleEditFile tests search-and-replace modification on files
func TestHandleEditFile(t *testing.T) {
	vs, tmpDir := createActionsTestVS(t)
	ctx := context.Background()

	fileName := "edit.txt"
	filePath := filepath.Join(tmpDir, fileName)
	content := "line 1\nold content\nline 3\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Success edit (single replacement)
	req := ActionRequest{
		ActionID: "e1",
		Target:   fileName,
		Payload: map[string]any{
			"old": "old content",
			"new": "new content",
		},
	}
	res, err := vs.handleEditFile(ctx, req)
	if err != nil {
		t.Fatalf("handleEditFile failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	gotBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	expected := "line 1\nnew content\nline 3\n"
	if string(gotBytes) != expected {
		t.Errorf("expected %q, got %q", expected, string(gotBytes))
	}

	// 2. Edit missing 'old' in payload (returns error in production)
	req.Payload = map[string]any{
		"new": "some content",
	}
	res, err = vs.handleEditFile(ctx, req)
	if err == nil {
		t.Error("expected error for missing 'old', got nil")
	}

	// 3. Search target not found
	req.Target = fileName
	req.Payload = map[string]any{
		"old": "non-existent",
		"new": "new stuff",
	}
	res, err = vs.handleEditFile(ctx, req)
	if err != nil {
		t.Fatalf("handleEditFile failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "not found") {
		t.Errorf("expected search pattern not found error, got: %+v", res)
	}

	// 4. File does not exist
	req.Target = "missing_file.txt"
	res, err = vs.handleEditFile(ctx, req)
	if err != nil {
		t.Fatalf("handleEditFile failed: %v", err)
	}
	if res.Success {
		t.Errorf("expected file not found error, got success: %+v", res)
	}

	// 5. Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleEditFile(cCtx, req)
	if err != nil {
		t.Fatalf("handleEditFile failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleDeleteFile tests deleting files
func TestHandleDeleteFile(t *testing.T) {
	vs, tmpDir := createActionsTestVS(t)
	ctx := context.Background()

	fileName := "delete.txt"
	filePath := filepath.Join(tmpDir, fileName)
	if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Success deletion with confirmation
	req := ActionRequest{
		ActionID: "del1",
		Target:   fileName,
		Payload: map[string]any{
			"confirmed": true,
		},
	}
	res, err := vs.handleDeleteFile(ctx, req)
	if err != nil {
		t.Fatalf("handleDeleteFile failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file still exists after deletion")
	}

	// 2. Delete non-existent file
	res, err = vs.handleDeleteFile(ctx, req)
	if err != nil {
		t.Fatalf("handleDeleteFile failed: %v", err)
	}
	if res.Success {
		t.Errorf("expected file not found error, got success: %+v", res)
	}

	// 3. Canceled context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleDeleteFile(cCtx, req)
	if err != nil {
		t.Fatalf("handleDeleteFile failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleSearchCode tests code search wrapper
func TestHandleSearchCode(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "s1",
		Target:   "search_pattern",
	}

	// 1. Write file to tmpDir with some pattern to walk/find
	fileName := "search.txt"
	filePath := filepath.Join(vs.workingDir, fileName)
	if err := os.WriteFile(filePath, []byte("here is the needle in the haystack"), 0644); err != nil {
		t.Fatalf("failed to write search file: %v", err)
	}

	req = ActionRequest{
		ActionID: "s1",
		Target:   "needle",
	}
	res, err := vs.handleSearchCode(ctx, req)
	if err != nil {
		t.Fatalf("handleSearchCode failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}
	if !strings.Contains(res.Output, "search.txt:1:here is the needle in the haystack") {
		t.Errorf("expected match output, got %q", res.Output)
	}

	// 2. Canceled context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleSearchCode(cCtx, req)
	if err != nil {
		t.Fatalf("handleSearchCode failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleRunTests tests test command runner
func TestHandleRunTests(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "t1",
		Target:   "go test ./internal/core/...",
	}

	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		cmdStr := cmd.CommandString()
		if !strings.Contains(cmdStr, "go test") {
			return nil, fmt.Errorf("expected go test command, got %q", cmdStr)
		}
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "ok"}, nil
	}

	res, err := vs.handleRunTests(ctx, req)
	if err != nil {
		t.Fatalf("handleRunTests failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleRunTests(cCtx, req)
	if err != nil {
		t.Fatalf("handleRunTests failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleBuildProject tests build command runner
func TestHandleBuildProject(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "b1",
		Target:   "go build ./cmd/nerd",
	}

	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		cmdStr := cmd.CommandString()
		if !strings.Contains(cmdStr, "go build") {
			return nil, fmt.Errorf("expected go build command, got %q", cmdStr)
		}
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "build ok"}, nil
	}

	res, err := vs.handleBuildProject(ctx, req)
	if err != nil {
		t.Fatalf("handleBuildProject failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleBuildProject(cCtx, req)
	if err != nil {
		t.Fatalf("handleBuildProject failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleGitOperation tests git wrapper commands
func TestHandleGitOperation(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "g1",
		Target:   "status",
	}

	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		cmdStr := cmd.CommandString()
		if !strings.Contains(cmdStr, "git status") {
			return nil, fmt.Errorf("expected git status command, got %q", cmdStr)
		}
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "on branch main"}, nil
	}

	res, err := vs.handleGitOperation(ctx, req)
	if err != nil {
		t.Fatalf("handleGitOperation failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleGitOperation(cCtx, req)
	if err != nil {
		t.Fatalf("handleGitOperation failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleShowDiff tests diff logic wrapper
func TestHandleShowDiff(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "diff1",
		Target:   "main.go",
	}

	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		cmdStr := cmd.CommandString()
		if !strings.Contains(cmdStr, "git diff") {
			return nil, fmt.Errorf("expected git diff command, got %q", cmdStr)
		}
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "diff output"}, nil
	}

	res, err := vs.handleShowDiff(ctx, req)
	if err != nil {
		t.Fatalf("handleShowDiff failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleShowDiff(cCtx, req)
	if err != nil {
		t.Fatalf("handleShowDiff failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleAnalyzeImpact tests impact analysis wrapper
func TestHandleAnalyzeImpact(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "i1",
		Target:   "internal/core/kernel.go",
	}

	// Should run analysis commands or query kernel
	mExec := vs.executor.(*mockActionsExecutor)
	mExec.executeFunc = func(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
		return &tactile.ExecutionResult{Success: true, ExitCode: 0, Stdout: "impact output"}, nil
	}

	res, err := vs.handleAnalyzeImpact(ctx, req)
	if err != nil {
		t.Fatalf("handleAnalyzeImpact failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleAnalyzeImpact(cCtx, req)
	if err != nil {
		t.Fatalf("handleAnalyzeImpact failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleBrowse tests browser navigation wrapper
func TestHandleBrowse(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "br1",
		Target:   "http://localhost:8080",
	}

	// Normally uses browser tool integration or mock.
	res, err := vs.handleBrowse(ctx, req)
	if err != nil {
		t.Fatalf("handleBrowse failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "must be executed via TactileRouterShard") {
		t.Errorf("expected routing error, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleBrowse(cCtx, req)
	if err != nil {
		t.Fatalf("handleBrowse failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleResearch tests context7 researcher wrapper
func TestHandleResearch(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "res1",
		Target:   "mcp server tools",
	}

	err := vs.modularTools.Register(&tools.Tool{
		Name:     "context7_fetch",
		Category: tools.CategoryResearch,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "mock documentation content", nil
		},
	})
	if err != nil {
		t.Fatalf("failed to register context7_fetch tool: %v", err)
	}

	res, err := vs.handleResearch(ctx, req)
	if err != nil {
		t.Fatalf("handleResearch failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleResearch(cCtx, req)
	if err != nil {
		t.Fatalf("handleResearch failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleModularTool tests modular tool call execution
func TestHandleModularTool(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "m1",
		Type:     ActionContext7Fetch,
		Target:   "test_topic",
	}

	// 1. Call with uninitialized registry
	vs.modularTools = nil
	res, err := vs.handleModularTool(ctx, req)
	if err != nil {
		t.Fatalf("handleModularTool failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "not initialized") {
		t.Errorf("expected not initialized error, got: %+v", res)
	}

	// 2. Call with empty registry (tool not found)
	vs.modularTools = tools.NewRegistry()
	res, err = vs.handleModularTool(ctx, req)
	if err != nil {
		t.Fatalf("handleModularTool failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "tool not found") {
		t.Errorf("expected not found error, got: %+v", res)
	}

	// 3. Register tool and call successfully
	err = vs.modularTools.Register(&tools.Tool{
		Name:     "context7_fetch",
		Category: tools.CategoryResearch,
		Execute: func(ctx context.Context, args map[string]any) (string, error) {
			return "mock tool output", nil
		},
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	res, err = vs.handleModularTool(ctx, req)
	if err != nil {
		t.Fatalf("handleModularTool failed: %v", err)
	}
	if !res.Success || res.Output != "mock tool output" {
		t.Errorf("expected success with mock tool output, got: %+v", res)
	}

	// 4. Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleModularTool(cCtx, req)
	if err != nil {
		t.Fatalf("handleModularTool failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleDelegate tests delegating tasks to shard specialists
func TestHandleDelegate(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "del_task",
		Target:   "coder",
		Payload: map[string]any{
			"task": "refactor logic",
		},
	}

	// 1. Delegator not configured
	vs.taskDelegator = nil
	vs.shardManager = nil
	res, err := vs.handleDelegate(ctx, req)
	if err != nil {
		t.Fatalf("handleDelegate failed: %v", err)
	}
	if res.Success || !strings.Contains(res.Error, "no executor configured") {
		t.Errorf("expected no executor configured error, got: %+v", res)
	}

	// 2. Setup mock delegator
	mDel := &mockActionsTaskDelegator{}
	vs.taskDelegator = mDel
	res, err = vs.handleDelegate(ctx, req)
	if err != nil {
		t.Fatalf("handleDelegate failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}
	if res.Output != "mock delegation success" {
		t.Errorf("expected mock delegation success, got %q", res.Output)
	}

	// 3. Delegate alias checks
	res, err = vs.handleDelegateAlias(ctx, req, "/coder")
	if err != nil {
		t.Fatalf("handleDelegateAlias failed: %v", err)
	}
	if !res.Success || res.Output != "mock delegation success" {
		t.Errorf("expected success for alias, got: %+v", res)
	}

	// 4. Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleDelegate(cCtx, req)
	if err != nil {
		t.Fatalf("handleDelegate failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleAskUser tests user interaction
func TestHandleAskUser(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "q1",
		Target:   "Is this acceptable?",
	}

	res, err := vs.handleAskUser(ctx, req)
	if err != nil {
		t.Fatalf("handleAskUser failed: %v", err)
	}
	if res.Success || res.Error != "USER_INPUT_REQUIRED" {
		t.Errorf("expected USER_INPUT_REQUIRED error, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleAskUser(cCtx, req)
	if err != nil {
		t.Fatalf("handleAskUser failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestHandleEscalate tests error escalation
func TestHandleEscalate(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	ctx := context.Background()

	req := ActionRequest{
		ActionID: "esc1",
		Target:   "critical failure",
	}

	res, err := vs.handleEscalate(ctx, req)
	if err != nil {
		t.Fatalf("handleEscalate failed: %v", err)
	}
	if res.Success || res.Error != "ESCALATION_REQUIRED" {
		t.Errorf("expected ESCALATION_REQUIRED error, got: %+v", res)
	}

	// Cancel context
	cCtx, cancel := context.WithCancel(ctx)
	cancel()
	res, err = vs.handleEscalate(cCtx, req)
	if err != nil {
		t.Fatalf("handleEscalate failed: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

// TestGetStrategicSummary tests the GetStrategicSummary method
func TestGetStrategicSummary(t *testing.T) {
	vs, tmpDir := createActionsTestVS(t)

	// 1. DB is nil
	vs.localDB = nil
	if summary := vs.GetStrategicSummary(); summary != "" {
		t.Errorf("expected empty strategic summary when DB is nil, got %q", summary)
	}

	// Setup local DB
	dbPath := filepath.Join(tmpDir, "knowledge.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()
	vs.localDB = db

	// 2. Empty database strategic summary
	if summary := vs.GetStrategicSummary(); summary != "" {
		t.Errorf("expected empty strategic summary for empty DB, got %q", summary)
	}

	// 3. Inject strategic and doc atoms
	atoms := []struct {
		concept    string
		content    string
		confidence float64
	}{
		{"strategic/vision", "Build the ultimate coding framework.", 1.0},
		{"strategic/component", "Kernel, perception, virtual store.", 0.8},
		{"strategic/pattern", "OODA loop.", 0.85},
		{"strategic/capability", "Self-repair, tool generation.", 0.99},
		{"strategic/constraint", "Verify safety of commands.", 1.0},
		{"strategic/full_knowledge", "Skip this blob.", 1.0},                // Should be skipped
		{"doc/internal/architecture/design", "Documentation detail.", 0.95}, // High confidence doc/ architecture
		{"doc/internal/pattern/flow", "Flow patterns.", 0.9},                // High confidence doc/ pattern
		{"doc/internal/philosophy/creed", "Datalog creed.", 0.99},           // High confidence doc/ philosophy
		{"doc/internal/capability/tools", "Tool capability.", 0.92},         // High confidence doc/ capability
		{"doc/internal/constraint/jail", "Jail cell restriction.", 0.89},    // High confidence doc/ constraint
		{"doc/internal/constraint/unsafe", "Ignore this constraint.", 0.5},  // Low confidence doc (should be ignored)
	}

	for _, a := range atoms {
		err = db.StoreKnowledgeAtom(a.concept, a.content, a.confidence)
		if err != nil {
			t.Fatalf("failed to store knowledge atom: %v", err)
		}
	}

	summary := vs.GetStrategicSummary()
	if summary == "" {
		t.Fatal("expected non-empty strategic summary")
	}

	// Verify containing concepts
	expectedSnippets := []string{
		"Build the ultimate coding framework.",
		"Kernel, perception, virtual store.",
		"OODA loop.",
		"Self-repair, tool generation.",
		"Verify safety of commands.",
		"Documentation detail.",
		"Flow patterns.",
		"Datalog creed.",
		"Tool capability.",
		"Jail cell restriction.",
	}
	for _, snip := range expectedSnippets {
		if !strings.Contains(summary, snip) {
			t.Errorf("expected summary to contain %q, summary was:\n%s", snip, summary)
		}
	}

	// Verify skipped things
	if strings.Contains(summary, "Skip this blob.") {
		t.Error("strategic/full_knowledge was not skipped")
	}
	if strings.Contains(summary, "Ignore this constraint.") {
		t.Error("low confidence doc atom was not ignored")
	}
}

// TestExtractCodeBlockForFile tests private helper extractCodeBlockForFile
func TestExtractCodeBlockForFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		path     string
		expected string
	}{
		{
			name:     "Markdown Go block",
			content:  "Here is the code:\n```go\npackage main\nfunc main() {}\n```\nHope it helps!",
			path:     "main.go",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "JSON block brace match",
			content:  "Explanation...\n{\n  \"key\": \"value\"\n}\nMore info...",
			path:     "config.json",
			expected: "{\n  \"key\": \"value\"\n}",
		},
		{
			name:     "Go package keyword fallback",
			content:  "Intro\npackage main\nfunc foo() {}",
			path:     "foo.go",
			expected: "package main\nfunc foo() {}",
		},
		{
			name:     "Direct content fallback",
			content:  "just raw string content",
			path:     "text.txt",
			expected: "just raw string content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCodeBlockForFile(tt.content, tt.path)
			if got != tt.expected {
				t.Errorf("extractCodeBlockForFile() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

// TestExtToLang checks the mapping in extToLang
func TestExtToLang(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{"go", "go"},
		{"ts", "typescript"},
		{"tsx", "typescript"},
		{"js", "javascript"},
		{"jsx", "javascript"},
		{"kt", "kotlin"},
		{"py", "python"},
		{"sql", "sql"},
		{"yaml", "yaml"},
		{"yml", "yaml"},
		{"json", "json"},
		{"md", "markdown"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := extToLang(tt.ext)
		if got != tt.want {
			t.Errorf("extToLang(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}
