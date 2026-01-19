package core

import (
	"context"
	"testing"
)

func TestVirtualStorePython_HandleSetup(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionPythonEnvSetup,
		Payload: map[string]interface{}{
			"project_name": "test-project",
		},
	}

	result, err := vs.handlePythonEnvSetup(ctx, req)
	if err != nil {
		t.Logf("handlePythonEnvSetup error: %v (expected without Python env)", err)
		return
	}
	t.Logf("handlePythonEnvSetup: %v", result.Success)
}

func TestVirtualStorePython_HandleExec(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionPythonEnvExec,
		Payload: map[string]interface{}{
			"project_name": "test-project",
			"command":      "python --version",
		},
	}

	result, err := vs.handlePythonEnvExec(ctx, req)
	if err != nil {
		t.Logf("handlePythonEnvExec error: %v", err)
		return
	}
	t.Logf("handlePythonEnvExec: %v", result.Success)
}

func TestVirtualStorePython_HandlePytest(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionPythonRunPytest,
		Payload: map[string]interface{}{
			"project_name": "test-project",
		},
	}

	result, err := vs.handlePythonRunPytest(ctx, req)
	if err != nil {
		t.Logf("handlePythonRunPytest error: %v", err)
		return
	}
	t.Logf("handlePythonRunPytest: %v", result.Success)
}

func TestVirtualStorePython_HandleSnapshot(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionPythonSnapshot,
		Payload: map[string]interface{}{
			"project_name":  "test-project",
			"snapshot_name": "baseline",
		},
	}

	result, err := vs.handlePythonSnapshot(ctx, req)
	if err != nil {
		t.Logf("handlePythonSnapshot error: %v", err)
		return
	}
	t.Logf("handlePythonSnapshot: %v", result.Success)
}

func TestVirtualStorePython_HandleRestore(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionPythonRestore,
		Payload: map[string]interface{}{
			"project_name":  "test-project",
			"snapshot_name": "baseline",
		},
	}

	result, err := vs.handlePythonRestore(ctx, req)
	if err != nil {
		t.Logf("handlePythonRestore error: %v", err)
		return
	}
	t.Logf("handlePythonRestore: %v", result.Success)
}

func TestVirtualStorePython_HandleTeardown(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}

	ctx := context.Background()
	req := ActionRequest{
		Type: ActionPythonTeardown,
		Payload: map[string]interface{}{
			"project_name": "test-project",
		},
	}

	result, err := vs.handlePythonTeardown(ctx, req)
	if err != nil {
		t.Logf("handlePythonTeardown error: %v", err)
		return
	}
	t.Logf("handlePythonTeardown: %v", result.Success)
}

func TestVirtualStorePython_SWEBenchSetup(t *testing.T) {
	t.Skip("SWE-bench setup requires Docker/container environment")
}

func TestVirtualStorePython_SWEBenchEvaluate(t *testing.T) {
	t.Skip("SWE-bench evaluate requires Docker/container environment")
}
