package core

import (
	"context"
	"strings"
	"testing"
)

func TestVirtualStorePython_EnvSetup(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name and git_url
	req := ActionRequest{
		Type:    ActionPythonEnvSetup,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonEnvSetup(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name or git_url required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Project name present
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
	}
	res, err = vs.handlePythonEnvSetup(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["project_name"] != "my-project" {
		t.Errorf("expected success, got: %+v", res)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "python_environment" {
		t.Errorf("expected python_environment fact, got: %v", res.FactsToAdd)
	}

	// 3. Project name and git_url present
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
		"git_url":      "https://github.com/foo/bar",
		"commit":       "abcdef",
		"branch":       "main",
	}
	res, err = vs.handlePythonEnvSetup(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["git_url"] != "https://github.com/foo/bar" {
		t.Errorf("expected success with git_url, got: %+v", res)
	}
	if len(res.FactsToAdd) != 2 || res.FactsToAdd[1].Predicate != "python_project_source" {
		t.Errorf("expected python_project_source fact, got: %v", res.FactsToAdd)
	}

	// 4. Context canceled
	res, err = vs.handlePythonEnvSetup(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_EnvExec(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name or command
	req := ActionRequest{
		Type:    ActionPythonEnvExec,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonEnvExec(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name and command required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
		"command":      "python -m pip install pytest",
	}
	res, err = vs.handlePythonEnvExec(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["command"] != "python -m pip install pytest" {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handlePythonEnvExec(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_RunPytest(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name
	req := ActionRequest{
		Type:    ActionPythonRunPytest,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonRunPytest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path with test_args
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
		"test_args":    []interface{}{"test_foo.py", "test_bar.py", 123}, // 123 should be ignored as non-string
	}
	res, err = vs.handlePythonRunPytest(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got: %+v", res)
	}
	args, _ := res.Metadata["test_args"].([]string)
	if len(args) != 2 || args[0] != "test_foo.py" || args[1] != "test_bar.py" {
		t.Errorf("expected test_args to contain only strings, got: %v", res.Metadata["test_args"])
	}

	// 3. Context canceled
	res, err = vs.handlePythonRunPytest(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_ApplyPatch(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name or patch
	req := ActionRequest{
		Type:    ActionPythonApplyPatch,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonApplyPatch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name and patch required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
		"patch":        "diff --git a/file.py b/file.py...",
	}
	res, err = vs.handlePythonApplyPatch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["patch_size"] != len("diff --git a/file.py b/file.py...") {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handlePythonApplyPatch(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_Snapshot(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name
	req := ActionRequest{
		Type:    ActionPythonSnapshot,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path with snapshot_name empty
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
	}
	res, err = vs.handlePythonSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !strings.Contains(res.Metadata["snapshot_name"].(string), "my-project-snapshot-") {
		t.Errorf("expected generated snapshot name, got: %+v", res)
	}

	// 3. Success path with snapshot_name specified
	req.Payload = map[string]interface{}{
		"project_name":  "my-project",
		"snapshot_name": "my-snap",
	}
	res, err = vs.handlePythonSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["snapshot_name"] != "my-snap" {
		t.Errorf("expected snapshot_name 'my-snap', got: %+v", res)
	}

	// 4. Context canceled
	res, err = vs.handlePythonSnapshot(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_Restore(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name or snapshot_name
	req := ActionRequest{
		Type:    ActionPythonRestore,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonRestore(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name and snapshot_name required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"project_name":  "my-project",
		"snapshot_name": "my-snap",
	}
	res, err = vs.handlePythonRestore(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["snapshot_name"] != "my-snap" {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handlePythonRestore(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_Teardown(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing project_name
	req := ActionRequest{
		Type:    ActionPythonTeardown,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handlePythonTeardown(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "project_name required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"project_name": "my-project",
	}
	res, err = vs.handlePythonTeardown(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["project_name"] != "my-project" {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handlePythonTeardown(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchSetup(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id
	req := ActionRequest{
		Type:    ActionSWEBenchSetup,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchSetup(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path with test expectations
	req.Payload = map[string]interface{}{
		"instance_id":       "inst-1",
		"repo":              "django/django",
		"base_commit":       "abcdef1234567890",
		"problem_statement": "fix model field validation",
		"fail_to_pass":      []interface{}{"test_fail_1", 123},
		"pass_to_pass":      []interface{}{"test_pass_1"},
	}
	res, err = vs.handleSWEBenchSetup(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["instance_id"] != "inst-1" {
		t.Errorf("expected success, got: %+v", res)
	}
	// Verify facts count: 2 env facts + 1 FTP + 1 PTP = 4 facts
	if len(res.FactsToAdd) != 4 {
		t.Errorf("expected 4 facts, got: %v", res.FactsToAdd)
	}

	// 3. Context canceled
	res, err = vs.handleSWEBenchSetup(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchApplyPatch(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id or patch
	req := ActionRequest{
		Type:    ActionSWEBenchApplyPatch,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchApplyPatch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id and patch required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"instance_id": "inst-1",
		"patch":        "diff --git a/file.py...",
	}
	res, err = vs.handleSWEBenchApplyPatch(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["patch_size"] != len("diff --git a/file.py...") {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handleSWEBenchApplyPatch(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchRunTests(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id
	req := ActionRequest{
		Type:    ActionSWEBenchRunTests,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchRunTests(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path with test_names
	req.Payload = map[string]interface{}{
		"instance_id": "inst-1",
		"test_names":  []interface{}{"test_one", 123},
	}
	res, err = vs.handleSWEBenchRunTests(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["test_count"] != 1 {
		t.Errorf("expected test count 1, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handleSWEBenchRunTests(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchSnapshot(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id
	req := ActionRequest{
		Type:    ActionSWEBenchSnapshot,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path with snapshot_name empty
	req.Payload = map[string]interface{}{
		"instance_id": "inst-1",
	}
	res, err = vs.handleSWEBenchSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || !strings.Contains(res.Metadata["snapshot_name"].(string), "inst-1-snapshot-") {
		t.Errorf("expected generated snapshot name, got: %+v", res)
	}

	// 3. Success path with snapshot_name specified
	req.Payload = map[string]interface{}{
		"instance_id":   "inst-1",
		"snapshot_name": "my-snap",
	}
	res, err = vs.handleSWEBenchSnapshot(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["snapshot_name"] != "my-snap" {
		t.Errorf("expected snapshot name 'my-snap', got: %+v", res)
	}

	// 4. Context canceled
	res, err = vs.handleSWEBenchSnapshot(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchRestore(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id or snapshot_name
	req := ActionRequest{
		Type:    ActionSWEBenchRestore,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchRestore(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id and snapshot_name required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"instance_id":   "inst-1",
		"snapshot_name": "my-snap",
	}
	res, err = vs.handleSWEBenchRestore(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["snapshot_name"] != "my-snap" {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handleSWEBenchRestore(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchEvaluate(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id
	req := ActionRequest{
		Type:    ActionSWEBenchEvaluate,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchEvaluate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path with model_name empty (defaults to codenerd)
	req.Payload = map[string]interface{}{
		"instance_id": "inst-1",
		"patch":        "diff...",
	}
	res, err = vs.handleSWEBenchEvaluate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["model_name"] != "codenerd" {
		t.Errorf("expected default model name codenerd, got: %+v", res)
	}

	// 3. Success path with model_name specified
	req.Payload = map[string]interface{}{
		"instance_id": "inst-1",
		"patch":        "diff...",
		"model_name":  "custom-model",
	}
	res, err = vs.handleSWEBenchEvaluate(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["model_name"] != "custom-model" {
		t.Errorf("expected model name custom-model, got: %+v", res)
	}

	// 4. Context canceled
	res, err = vs.handleSWEBenchEvaluate(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}

func TestVirtualStorePython_SWEBenchTeardown(t *testing.T) {
	k := setupMockKernel(t)
	vs := &VirtualStore{kernel: k}
	ctx := context.Background()
	cCtx, cancel := context.WithCancel(ctx)
	cancel()

	// 1. Missing instance_id
	req := ActionRequest{
		Type:    ActionSWEBenchTeardown,
		Payload: map[string]interface{}{},
	}
	res, err := vs.handleSWEBenchTeardown(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != "instance_id required in payload" {
		t.Errorf("expected validation failure, got: %+v", res)
	}

	// 2. Success path
	req.Payload = map[string]interface{}{
		"instance_id": "inst-1",
	}
	res, err = vs.handleSWEBenchTeardown(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success || res.Metadata["instance_id"] != "inst-1" {
		t.Errorf("expected success, got: %+v", res)
	}

	// 3. Context canceled
	res, err = vs.handleSWEBenchTeardown(cCtx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Success || res.Error != context.Canceled.Error() {
		t.Errorf("expected canceled error, got: %+v", res)
	}
}
