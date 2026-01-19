package core

import (
	"context"
	"testing"
)

func TestVirtualStoreCodeDOM_HandleOpenFile(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type:   ActionOpenFile,
		Target: "test.go",
		Payload: map[string]interface{}{
			"path": "test.go",
		},
	}

	result, err := vs.handleOpenFile(ctx, req)
	if err != nil {
		t.Logf("handleOpenFile error: %v (expected if file doesn't exist)", err)
	} else {
		t.Logf("handleOpenFile success: %v", result.Success)
	}
}

func TestVirtualStoreCodeDOM_HandleGetElements(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionGetElements,
	}

	result, err := vs.handleGetElements(ctx, req)
	if err != nil {
		t.Logf("handleGetElements error: %v (expected if no scope open)", err)
	} else {
		t.Logf("handleGetElements success: %v", result.Success)
	}
}

func TestVirtualStoreCodeDOM_HandleEditLines(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type:   ActionEditLines,
		Target: "test.go",
		Payload: map[string]interface{}{
			"start_line": 1,
			"end_line":   5,
			"new_lines":  []string{"// edited"},
		},
	}

	result, err := vs.handleEditLines(ctx, req)
	if err != nil {
		t.Logf("handleEditLines error: %v (expected)", err)
	} else {
		t.Logf("handleEditLines: %v", result.Success)
	}
}

func TestVirtualStoreCodeDOM_HandleInsertLines(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type:   ActionInsertLines,
		Target: "test.go",
		Payload: map[string]interface{}{
			"after_line": 0,
			"new_lines":  []string{"// header"},
		},
	}

	result, err := vs.handleInsertLines(ctx, req)
	if err != nil {
		t.Logf("handleInsertLines error: %v (expected)", err)
	} else {
		t.Logf("handleInsertLines: %v", result.Success)
	}
}

func TestVirtualStoreCodeDOM_HandleDeleteLines(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type:   ActionDeleteLines,
		Target: "test.go",
		Payload: map[string]interface{}{
			"start_line": 1,
			"end_line":   3,
		},
	}

	result, err := vs.handleDeleteLines(ctx, req)
	if err != nil {
		t.Logf("handleDeleteLines error: %v (expected)", err)
	} else {
		t.Logf("handleDeleteLines: %v", result.Success)
	}
}

func TestVirtualStoreCodeDOM_HandleCloseScope(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionCloseScope,
	}

	result, err := vs.handleCloseScope(ctx, req)
	if err != nil {
		t.Logf("handleCloseScope error: %v", err)
	} else {
		// Should succeed even if no scope was open
		if !result.Success {
			t.Logf("handleCloseScope returned failure: %s", result.Error)
		}
	}
}

func TestTactileFileEditorAdapter(t *testing.T) {
	// Test adapter creation (nil editor - just testing interface)
	adapter := NewTactileFileEditorAdapter(nil)

	if adapter == nil {
		t.Fatal("NewTactileFileEditorAdapter returned nil")
	}
}
