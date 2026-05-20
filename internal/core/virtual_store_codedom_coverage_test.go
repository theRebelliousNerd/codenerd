package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"codenerd/internal/tactile"
)

// Mock implementation of CodeScope interface
type mockCodeScope struct {
	openFunc                  func(path string) error
	refreshFunc               func() error
	closeFunc                 func()
	getCoreElementFunc        func(ref string) *CodeElement
	getElementBodyFunc        func(ref string) string
	getCoreElementsByFileFunc func(path string) []CodeElement
	isInScopeFunc             func(path string) bool
	scopeFactsFunc            func() []Fact
	getActiveFileFunc         func() string
	getInScopeFilesFunc       func() []string
	verifyFileHashFunc        func(path string) (bool, error)
	refreshWithRetryFunc      func(maxRetries int) error
}

func (m *mockCodeScope) Open(path string) error {
	if m.openFunc != nil {
		return m.openFunc(path)
	}
	return nil
}

func (m *mockCodeScope) Refresh() error {
	if m.refreshFunc != nil {
		return m.refreshFunc()
	}
	return nil
}

func (m *mockCodeScope) Close() {
	if m.closeFunc != nil {
		m.closeFunc()
	}
}

func (m *mockCodeScope) GetCoreElement(ref string) *CodeElement {
	if m.getCoreElementFunc != nil {
		return m.getCoreElementFunc(ref)
	}
	return nil
}

func (m *mockCodeScope) GetElementBody(ref string) string {
	if m.getElementBodyFunc != nil {
		return m.getElementBodyFunc(ref)
	}
	return ""
}

func (m *mockCodeScope) GetCoreElementsByFile(path string) []CodeElement {
	if m.getCoreElementsByFileFunc != nil {
		return m.getCoreElementsByFileFunc(path)
	}
	return nil
}

func (m *mockCodeScope) IsInScope(path string) bool {
	if m.isInScopeFunc != nil {
		return m.isInScopeFunc(path)
	}
	return false
}

func (m *mockCodeScope) ScopeFacts() []Fact {
	if m.scopeFactsFunc != nil {
		return m.scopeFactsFunc()
	}
	return nil
}

func (m *mockCodeScope) GetActiveFile() string {
	if m.getActiveFileFunc != nil {
		return m.getActiveFileFunc()
	}
	return ""
}

func (m *mockCodeScope) GetInScopeFiles() []string {
	if m.getInScopeFilesFunc != nil {
		return m.getInScopeFilesFunc()
	}
	return nil
}

func (m *mockCodeScope) VerifyFileHash(path string) (bool, error) {
	if m.verifyFileHashFunc != nil {
		return m.verifyFileHashFunc(path)
	}
	return true, nil
}

func (m *mockCodeScope) RefreshWithRetry(maxRetries int) error {
	if m.refreshWithRetryFunc != nil {
		return m.refreshWithRetryFunc(maxRetries)
	}
	return nil
}

// Mock implementation of FileEditor interface
type mockFileEditor struct {
	readFileFunc       func(path string) ([]string, error)
	readLinesFunc      func(path string, startLine, endLine int) ([]string, error)
	writeFileFunc      func(path string, lines []string) (*FileEditResult, error)
	editLinesFunc      func(path string, startLine, endLine int, newLines []string) (*FileEditResult, error)
	insertLinesFunc    func(path string, afterLine int, newLines []string) (*FileEditResult, error)
	deleteLinesFunc    func(path string, startLine, endLine int) (*FileEditResult, error)
	replaceElementFunc func(path string, startLine, endLine int, newContent string) (*FileEditResult, error)
}

func (m *mockFileEditor) ReadFile(path string) ([]string, error) {
	if m.readFileFunc != nil {
		return m.readFileFunc(path)
	}
	return nil, nil
}

func (m *mockFileEditor) ReadLines(path string, startLine, endLine int) ([]string, error) {
	if m.readLinesFunc != nil {
		return m.readLinesFunc(path, startLine, endLine)
	}
	return nil, nil
}

func (m *mockFileEditor) WriteFile(path string, lines []string) (*FileEditResult, error) {
	if m.writeFileFunc != nil {
		return m.writeFileFunc(path, lines)
	}
	return &FileEditResult{Success: true}, nil
}

func (m *mockFileEditor) EditLines(path string, startLine, endLine int, newLines []string) (*FileEditResult, error) {
	if m.editLinesFunc != nil {
		return m.editLinesFunc(path, startLine, endLine, newLines)
	}
	return &FileEditResult{Success: true}, nil
}

func (m *mockFileEditor) InsertLines(path string, afterLine int, newLines []string) (*FileEditResult, error) {
	if m.insertLinesFunc != nil {
		return m.insertLinesFunc(path, afterLine, newLines)
	}
	return &FileEditResult{Success: true}, nil
}

func (m *mockFileEditor) DeleteLines(path string, startLine, endLine int) (*FileEditResult, error) {
	if m.deleteLinesFunc != nil {
		return m.deleteLinesFunc(path, startLine, endLine)
	}
	return &FileEditResult{Success: true}, nil
}

func (m *mockFileEditor) ReplaceElement(path string, startLine, endLine int, newContent string) (*FileEditResult, error) {
	if m.replaceElementFunc != nil {
		return m.replaceElementFunc(path, startLine, endLine, newContent)
	}
	return &FileEditResult{Success: true}, nil
}

// Mock implementation of ToolExecutor interface
type mockToolExecutor struct {
	executeToolFunc func(ctx context.Context, toolName string, input string) (string, error)
	listToolsFunc   func() []ToolInfo
	getToolFunc     func(name string) (*ToolInfo, bool)
}

func (m *mockToolExecutor) ExecuteTool(ctx context.Context, toolName string, input string) (string, error) {
	if m.executeToolFunc != nil {
		return m.executeToolFunc(ctx, toolName, input)
	}
	return "", nil
}

func (m *mockToolExecutor) ListTools() []ToolInfo {
	if m.listToolsFunc != nil {
		return m.listToolsFunc()
	}
	return nil
}

func (m *mockToolExecutor) GetTool(name string) (*ToolInfo, bool) {
	if m.getToolFunc != nil {
		return m.getToolFunc(name)
	}
	return nil, false
}

// Mock implementation of ToolGenerator interface
type mockToolGenerator struct {
	// Dummy
}

func TestVirtualStoreCodeDOM_OpenFile_Detailed(t *testing.T) {
	t.Logf("Running test at %v (pid: %d)", time.Now(), os.Getpid())
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()

	// 1. Unconfigured CodeScope
	req := ActionRequest{
		Type:   ActionOpenFile,
		Target: "test.go",
	}
	res, err := vs.handleOpenFile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "code scope not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	// 2. Configured CodeScope - Fail
	mScope := &mockCodeScope{
		openFunc: func(path string) error {
			return errors.New("open failed")
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleOpenFile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "open failed" {
		t.Errorf("expected open failed error, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "scope_open_failed" {
		t.Errorf("expected scope_open_failed fact, got: %v", res.FactsToAdd)
	}

	// 3. Configured CodeScope - Success
	mScope.openFunc = func(path string) error {
		if path != "test.go" {
			return fmt.Errorf("unexpected path: %s", path)
		}
		return nil
	}
	mScope.scopeFactsFunc = func() []Fact {
		return []Fact{{Predicate: "file_in_scope", Args: []interface{}{"test.go"}}}
	}
	res, err = vs.handleOpenFile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}
	if res.Metadata["active_file"] != "test.go" {
		t.Errorf("expected active_file metadata, got: %v", res.Metadata)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "file_in_scope" {
		t.Errorf("expected scope facts to be returned, got: %v", res.FactsToAdd)
	}

	// 4. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleOpenFile(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_GetElements_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	// 1. Unconfigured
	req := ActionRequest{Type: ActionGetElements}
	res, err := vs.handleGetElements(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "code scope not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	// 2. Configured - Target Empty
	mScope := &mockCodeScope{
		getInScopeFilesFunc: func() []string {
			return []string{"file1.go", "file2.go"}
		},
		getCoreElementsByFileFunc: func(path string) []CodeElement {
			if path == "file1.go" {
				return []CodeElement{
					{Ref: "elem1", Type: "struct"},
					{Ref: "elem2", Type: "func"},
				}
			}
			return []CodeElement{
				{Ref: "elem3", Type: "func"},
			}
		},
	}
	vs.SetCodeScope(mScope)

	res, err = vs.handleGetElements(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	if res.Metadata["count"] != 3 {
		t.Errorf("expected count 3, got: %v", res.Metadata["count"])
	}

	// 3. Configured - Target Empty with Type Filter
	req.Payload = map[string]interface{}{"type": "func"}
	res, err = vs.handleGetElements(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	if res.Metadata["count"] != 2 {
		t.Errorf("expected count 2, got: %v", res.Metadata["count"])
	}

	// 4. Configured - Target Specific File
	req.Target = "file1.go"
	req.Payload = nil
	res, err = vs.handleGetElements(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	if res.Metadata["count"] != 2 {
		t.Errorf("expected count 2, got: %v", res.Metadata["count"])
	}

	// 5. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleGetElements(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_GetElement_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{Type: ActionGetElement, Target: "ref_foo"}

	// 1. Unconfigured
	res, err := vs.handleGetElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "code scope not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	// 2. Configured - Not Found
	mScope := &mockCodeScope{
		getCoreElementFunc: func(ref string) *CodeElement {
			return nil
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleGetElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "element not found: ref_foo" {
		t.Errorf("expected not found error, got: %+v", res)
	}

	// 3. Configured - Found
	elem := &CodeElement{
		Ref:       "ref_foo",
		Type:      "func",
		File:      "foo.go",
		StartLine: 10,
		EndLine:   20,
		Body:      "",
	}
	mScope.getCoreElementFunc = func(ref string) *CodeElement {
		if ref == "ref_foo" {
			return elem
		}
		return nil
	}
	mScope.getElementBodyFunc = func(ref string) string {
		if ref == "ref_foo" {
			return "func foo() {}"
		}
		return ""
	}

	// Without include_body
	res, err = vs.handleGetElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	if res.Metadata["ref"] != "ref_foo" || res.Metadata["type"] != "func" {
		t.Errorf("unexpected metadata: %v", res.Metadata)
	}

	// With include_body
	req.Payload = map[string]interface{}{"include_body": true}
	res, err = vs.handleGetElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}

	// 4. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleGetElement(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_EditElement_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{
		Type:      ActionEditElement,
		Target:    "ref_foo",
		SessionID: "sess_1",
		Payload:   map[string]interface{}{"content": "new_body"},
	}

	// 1. Unconfigured CodeScope
	res, err := vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "code scope not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	// 2. Unconfigured FileEditor
	mScope := &mockCodeScope{}
	vs.SetCodeScope(mScope)
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "file editor not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	mEditor := &mockFileEditor{}
	vs.SetFileEditor(mEditor)

	// 3. Payload missing content
	badReq := req
	badReq.Payload = nil
	res, err = vs.handleEditElement(ctx, badReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "edit_element requires 'content' in payload" {
		t.Errorf("expected payload error, got: %+v", res)
	}

	// 4. Element not found
	mScope.getCoreElementFunc = func(ref string) *CodeElement {
		return nil
	}
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "element not found: ref_foo" {
		t.Errorf("expected not found error, got: %+v", res)
	}

	// Setup valid element
	elem := &CodeElement{
		Ref:       "ref_foo",
		File:      "foo.go",
		StartLine: 1,
		EndLine:   5,
	}
	mScope.getCoreElementFunc = func(ref string) *CodeElement {
		return elem
	}

	// 5. VerifyFileHash fails
	mScope.verifyFileHashFunc = func(path string) (bool, error) {
		return false, errors.New("hash err")
	}
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "failed to verify file hash: hash err" {
		t.Errorf("expected hash error, got: %+v", res)
	}

	// 6. File modified externally, refresh fails
	mScope.verifyFileHashFunc = func(path string) (bool, error) {
		return false, nil
	}
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return errors.New("refresh err")
	}
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "file was modified externally and refresh failed" {
		t.Errorf("expected refresh err, got: %+v", res)
	}

	// 7. File modified externally, refresh succeeds but element is gone
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return nil
	}
	var getCount int
	mScope.getCoreElementFunc = func(ref string) *CodeElement {
		getCount++
		if getCount == 1 {
			return elem // First call (before refresh)
		}
		return nil // Second call (after refresh)
	}
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "element ref_foo no longer exists after refresh" {
		t.Errorf("expected stale err, got: %+v", res)
	}

	// Restore getCoreElementFunc
	mScope.getCoreElementFunc = func(ref string) *CodeElement {
		return elem
	}

	// 8. ReplaceElement fails
	mScope.verifyFileHashFunc = func(path string) (bool, error) {
		return true, nil
	}
	mEditor.replaceElementFunc = func(path string, startLine, endLine int, newContent string) (*FileEditResult, error) {
		return nil, errors.New("replace err")
	}
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "replace err" {
		t.Errorf("expected replace err, got: %+v", res)
	}

	// 9. Success path (with refresh scope success)
	mEditor.replaceElementFunc = func(path string, startLine, endLine int, newContent string) (*FileEditResult, error) {
		return &FileEditResult{
			Success:       true,
			LinesAffected: 5,
			LineCount:     10,
			Facts:         []Fact{{Predicate: "replaced_something", Args: []interface{}{}}},
		}, nil
	}
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return nil
	}
	mScope.scopeFactsFunc = func() []Fact {
		return []Fact{{Predicate: "refreshed_fact", Args: []interface{}{}}}
	}

	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}
	if res.Metadata["lines_affected"] != 5 {
		t.Errorf("expected lines_affected 5, got: %v", res.Metadata)
	}

	// 10. Success path (with refresh scope failure)
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return errors.New("post-refresh fail")
	}
	res, err = vs.handleEditElement(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got: %+v", res)
	}

	// 11. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleEditElement(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_RefreshScope_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{Type: ActionRefreshScope}

	// 1. Unconfigured
	res, err := vs.handleRefreshScope(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "code scope not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	// 2. Configured - Fail
	mScope := &mockCodeScope{
		refreshFunc: func() error {
			return errors.New("refresh failed")
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleRefreshScope(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "refresh failed" {
		t.Errorf("expected error, got: %+v", res)
	}

	// 3. Configured - Success
	mScope.refreshFunc = func() error {
		return nil
	}
	res, err = vs.handleRefreshScope(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 4. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleRefreshScope(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_CloseScope_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{Type: ActionCloseScope}

	// 1. Unconfigured
	res, err := vs.handleCloseScope(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "code scope not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	// 2. Configured
	var closeCalled bool
	mScope := &mockCodeScope{
		closeFunc: func() {
			closeCalled = true
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleCloseScope(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !closeCalled {
		t.Errorf("expected success and close to be called, got: %+v", res)
	}

	// 3. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleCloseScope(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_EditLines_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionEditLines,
		Target: "test.go",
		Payload: map[string]interface{}{
			"start_line": float64(1),
			"end_line":   float64(5),
			"content":    "line1\nline2\n",
		},
	}

	// 1. Unconfigured
	res, err := vs.handleEditLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "file editor not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	mEditor := &mockFileEditor{}
	vs.SetFileEditor(mEditor)

	// 2. Missing line numbers
	badReq := req
	badReq.Payload = nil
	res, err = vs.handleEditLines(ctx, badReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "edit_lines requires 'start_line' and 'end_line' in payload" {
		t.Errorf("expected payload error, got: %+v", res)
	}

	// 3. EditLines fails
	mEditor.editLinesFunc = func(path string, startLine, endLine int, newLines []string) (*FileEditResult, error) {
		return nil, errors.New("edit failed")
	}
	res, err = vs.handleEditLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "edit failed" {
		t.Errorf("expected error, got: %+v", res)
	}

	// Setup successful EditLines mock
	mEditor.editLinesFunc = func(path string, startLine, endLine int, newLines []string) (*FileEditResult, error) {
		return &FileEditResult{
			Success:       true,
			LinesAffected: 5,
		}, nil
	}

	// 4. Success, but no scope configured
	res, err = vs.handleEditLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 5. Success, with scope (out of scope path)
	mScope := &mockCodeScope{
		isInScopeFunc: func(path string) bool {
			return false
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleEditLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 6. Success, with scope (in scope path) - refresh succeeds
	mScope.isInScopeFunc = func(path string) bool {
		return true
	}
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return nil
	}
	res, err = vs.handleEditLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 7. Success, with scope (in scope path) - refresh fails
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return errors.New("refresh fail")
	}
	res, err = vs.handleEditLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 8. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleEditLines(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_InsertLines_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionInsertLines,
		Target: "test.go",
		Payload: map[string]interface{}{
			"after_line": float64(10),
			"content":    "inserted_line\n",
		},
	}

	// 1. Unconfigured
	res, err := vs.handleInsertLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "file editor not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	mEditor := &mockFileEditor{}
	vs.SetFileEditor(mEditor)

	// 2. Missing content
	badReq := req
	badReq.Payload = nil
	res, err = vs.handleInsertLines(ctx, badReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "insert_lines requires 'content' in payload" {
		t.Errorf("expected payload error, got: %+v", res)
	}

	// 3. InsertLines fails
	mEditor.insertLinesFunc = func(path string, afterLine int, newLines []string) (*FileEditResult, error) {
		return nil, errors.New("insert failed")
	}
	res, err = vs.handleInsertLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "insert failed" {
		t.Errorf("expected error, got: %+v", res)
	}

	// Setup successful InsertLines mock
	mEditor.insertLinesFunc = func(path string, afterLine int, newLines []string) (*FileEditResult, error) {
		return &FileEditResult{
			Success:       true,
			LinesAffected: 1,
		}, nil
	}

	// 4. Success, with scope (in scope path) - refresh succeeds
	mScope := &mockCodeScope{
		isInScopeFunc: func(path string) bool {
			return true
		},
		refreshWithRetryFunc: func(maxRetries int) error {
			return nil
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleInsertLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 5. Success, with scope (in scope path) - refresh fails
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return errors.New("refresh fail")
	}
	res, err = vs.handleInsertLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 6. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleInsertLines(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_DeleteLines_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionDeleteLines,
		Target: "test.go",
		Payload: map[string]interface{}{
			"start_line": float64(5),
			"end_line":   float64(10),
		},
	}

	// 1. Unconfigured
	res, err := vs.handleDeleteLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "file editor not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	mEditor := &mockFileEditor{}
	vs.SetFileEditor(mEditor)

	// 2. Missing line numbers
	badReq := req
	badReq.Payload = nil
	res, err = vs.handleDeleteLines(ctx, badReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "delete_lines requires 'start_line' and 'end_line' in payload" {
		t.Errorf("expected payload error, got: %+v", res)
	}

	// 3. DeleteLines fails
	mEditor.deleteLinesFunc = func(path string, startLine, endLine int) (*FileEditResult, error) {
		return nil, errors.New("delete failed")
	}
	res, err = vs.handleDeleteLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "delete failed" {
		t.Errorf("expected error, got: %+v", res)
	}

	// Setup successful DeleteLines mock
	mEditor.deleteLinesFunc = func(path string, startLine, endLine int) (*FileEditResult, error) {
		return &FileEditResult{
			Success:       true,
			LinesAffected: 6,
		}, nil
	}

	// 4. Success, with scope (in scope path) - refresh succeeds
	mScope := &mockCodeScope{
		isInScopeFunc: func(path string) bool {
			return true
		},
		refreshWithRetryFunc: func(maxRetries int) error {
			return nil
		},
	}
	vs.SetCodeScope(mScope)
	res, err = vs.handleDeleteLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 5. Success, with scope (in scope path) - refresh fails
	mScope.refreshWithRetryFunc = func(maxRetries int) error {
		return errors.New("refresh fail")
	}
	res, err = vs.handleDeleteLines(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}

	// 6. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleDeleteLines(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStoreCodeDOM_ExecTool_Detailed(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()

	req := ActionRequest{
		Type:   ActionExecTool,
		Target: "test_tool",
		Payload: map[string]interface{}{
			"input": "tool_input",
		},
	}

	// 1. Unconfigured
	res, err := vs.handleExecTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "tool executor not configured" {
		t.Errorf("expected config error, got: %+v", res)
	}

	mExec := &mockToolExecutor{}
	vs.SetToolExecutor(mExec)

	// 2. Tool not found in executor
	mExec.getToolFunc = func(name string) (*ToolInfo, bool) {
		return nil, false
	}
	res, err = vs.handleExecTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "tool not found: test_tool" {
		t.Errorf("expected not found error, got: %+v", res)
	}

	// Setup valid tool
	tInfo := &ToolInfo{
		Name:         "test_tool",
		Hash:         "hash_123",
		ExecuteCount: 10,
	}
	mExec.getToolFunc = func(name string) (*ToolInfo, bool) {
		if name == "test_tool" {
			return tInfo, true
		}
		return nil, false
	}

	// 3. Tool execution fails
	mExec.executeToolFunc = func(ctx context.Context, name string, input string) (string, error) {
		return "partial output", errors.New("exec fail")
	}
	res, err = vs.handleExecTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "exec fail" || res.Output != "partial output" {
		t.Errorf("expected execution fail, got: %+v", res)
	}

	// 4. Tool execution succeeds (with registry)
	// We can use a real ToolRegistry!
	tr := NewToolRegistry(t.TempDir())
	vs.toolRegistry = tr
	// Register the tool in the registry first
	tr.tools["test_tool"] = &Tool{
		Name:          "test_tool",
		Command:       "dummy_cmd",
		ShardAffinity: "/coder",
	}

	mExec.executeToolFunc = func(ctx context.Context, name string, input string) (string, error) {
		return "success output", nil
	}
	res, err = vs.handleExecTool(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Output != "success output" {
		t.Errorf("expected success, got: %+v", res)
	}
	if res.Metadata["tool_name"] != "test_tool" || res.Metadata["shard_affinity"] != "/coder" {
		t.Errorf("unexpected metadata: %v", res.Metadata)
	}

	// 5. Context cancelled
	cCtx, cancel := context.WithCancel(context.Background())
	cancel()
	res, err = vs.handleExecTool(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestTactileFileEditorAdapter_Detailed(t *testing.T) {
	tempDir := t.TempDir()
	testFilePath := filepath.Join(tempDir, "test.txt")

	// Create real file editor
	editor := tactile.NewFileEditor()
	editor.SetWorkingDir(tempDir)

	// Wrap in adapter
	adapter := NewTactileFileEditorAdapter(editor)

	// 1. WriteFile
	initialLines := []string{"line1", "line2", "line3"}
	res, err := adapter.WriteFile(testFilePath, initialLines)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if !res.Success || res.LineCount != 3 {
		t.Errorf("unexpected WriteFile result: %+v", res)
	}

	// 2. ReadFile
	readLines, err := adapter.ReadFile(testFilePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !reflect.DeepEqual(readLines, initialLines) {
		t.Errorf("read lines do not match: %v vs %v", readLines, initialLines)
	}

	// 3. ReadLines (inclusive, 1-indexed)
	subset, err := adapter.ReadLines(testFilePath, 2, 3)
	if err != nil {
		t.Fatalf("ReadLines failed: %v", err)
	}
	expectedSubset := []string{"line2", "line3"}
	if !reflect.DeepEqual(subset, expectedSubset) {
		t.Errorf("subset mismatch: %v", subset)
	}

	// 4. EditLines
	editRes, err := adapter.EditLines(testFilePath, 2, 2, []string{"new_line2"})
	if err != nil {
		t.Fatalf("EditLines failed: %v", err)
	}
	if !editRes.Success || editRes.LineCount != 3 {
		t.Errorf("unexpected EditLines result: %+v", editRes)
	}
	readLines, _ = adapter.ReadFile(testFilePath)
	expectedAfterEdit := []string{"line1", "new_line2", "line3"}
	if !reflect.DeepEqual(readLines, expectedAfterEdit) {
		t.Errorf("after edit lines mismatch: %v", readLines)
	}

	// 5. InsertLines
	insertRes, err := adapter.InsertLines(testFilePath, 1, []string{"inserted"})
	if err != nil {
		t.Fatalf("InsertLines failed: %v", err)
	}
	if !insertRes.Success || insertRes.LineCount != 4 {
		t.Errorf("unexpected InsertLines result: %+v", insertRes)
	}
	readLines, _ = adapter.ReadFile(testFilePath)
	expectedAfterInsert := []string{"line1", "inserted", "new_line2", "line3"}
	if !reflect.DeepEqual(readLines, expectedAfterInsert) {
		t.Errorf("after insert lines mismatch: %v", readLines)
	}

	// 6. DeleteLines
	deleteRes, err := adapter.DeleteLines(testFilePath, 2, 3)
	if err != nil {
		t.Fatalf("DeleteLines failed: %v", err)
	}
	if !deleteRes.Success || deleteRes.LineCount != 2 {
		t.Errorf("unexpected DeleteLines result: %+v", deleteRes)
	}
	readLines, _ = adapter.ReadFile(testFilePath)
	expectedAfterDelete := []string{"line1", "line3"}
	if !reflect.DeepEqual(readLines, expectedAfterDelete) {
		t.Errorf("after delete lines mismatch: %v", readLines)
	}

	// 7. ReplaceElement
	replaceRes, err := adapter.ReplaceElement(testFilePath, 1, 2, "replaced_all\n")
	if err != nil {
		t.Fatalf("ReplaceElement failed: %v", err)
	}
	if !replaceRes.Success || replaceRes.LineCount != 1 {
		t.Errorf("unexpected ReplaceElement result: %+v", replaceRes)
	}
	readLines, _ = adapter.ReadFile(testFilePath)
	expectedAfterReplace := []string{"replaced_all"}
	if !reflect.DeepEqual(readLines, expectedAfterReplace) {
		t.Errorf("after replace lines mismatch: %v", readLines)
	}

	// 8. Exec (errors as not supported)
	_, _, err = adapter.Exec(context.Background(), "echo hello", nil)
	if err == nil {
		t.Error("expected adapter.Exec to fail")
	}

	// 9. Exec with nil editor (should return error)
	nilAdapter := NewTactileFileEditorAdapter(nil)
	_, _, err = nilAdapter.Exec(context.Background(), "echo hello", nil)
	if err == nil {
		t.Error("expected nil adapter.Exec to fail")
	}
}
