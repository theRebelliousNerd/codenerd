package campaign

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// -----------------------------------------------------------------------------
// Marathon 19: Task Handler Gaps (Gaps 1-14)
// -----------------------------------------------------------------------------

func TestExtractCodeBlock_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		lang     string
		expected string
	}{
		{
			name:     "standard markdown",
			input:    "Here is the code:\n```go\npackage main\nfunc main() {}\n```",
			lang:     "go",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "no fences",
			input:    "package main\nfunc main() {}",
			lang:     "go",
			expected: "package main\nfunc main() {}",
		},
		{
			name:     "multiple blocks",
			input:    "Block 1:\n```go\nfunc one() {}\n```\nBlock 2:\n```go\nfunc two() {}\n```",
			lang:     "go",
			expected: "func one() {}",
		},
		{
			name:     "nested backticks inside code",
			input:    "```go\nfmt.Println(\"`backticks`\")\n```",
			lang:     "go",
			expected: "fmt.Println(\"`backticks`\")",
		},
		{
			name:     "language mismatch (should return original)",
			input:    "```python\nprint('hello')\n```",
			lang:     "go",
			expected: "```python\nprint('hello')\n```",
		},
		{
			name:     "bare fences",
			input:    "```\nfunc main() {}\n```",
			lang:     "go",
			expected: "func main() {}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCodeBlock(tc.input, tc.lang)
			if got != tc.expected {
				t.Errorf("extractCodeBlock(%q, %q) = %q; want %q", tc.input, tc.lang, got, tc.expected)
			}
		})
	}
}

func TestExtractPathFromDescription_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		desc     string
		expected string
	}{
		{
			name:     "create pattern",
			desc:     "Create internal/domain/foo.go",
			expected: "internal/domain/foo.go",
		},
		{
			name:     "file pattern",
			desc:     "file: path/to/file.go",
			expected: "path/to/file.go",
		},
		{
			name:     "bare path",
			desc:     "cmd/nerd/main.go",
			expected: "cmd/nerd/main.go",
		},
		{
			name:     "no path",
			desc:     "Just a description without path",
			expected: "",
		},
		{
			name:     "multiple paths (first match)",
			desc:     "Update internal/a.go and internal/b.go",
			expected: "internal/a.go",
		},
		{
			name:     "internal path",
			desc:     "internal/pkg/utils.go needs update",
			expected: "internal/pkg/utils.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPathFromDescription(tc.desc)
			if got != tc.expected {
				t.Errorf("extractPathFromDescription(%q) = %q; want %q", tc.desc, got, tc.expected)
			}
		})
	}
}

type MockTaskExecutor struct {
	ExecuteFunc            func(ctx context.Context, req session.TaskRequest) (string, error)
	ExecuteWithContextFunc func(ctx context.Context, req session.TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error)
	ExecuteAsyncFunc       func(ctx context.Context, req session.TaskRequest) (string, error)
	GetResultFunc          func(taskID string) (string, bool, error)
	WaitForResultFunc      func(ctx context.Context, taskID string) (string, error)
}

func (m *MockTaskExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, req)
	}
	return "", errors.New("not implemented")
}

func (m *MockTaskExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	if m.ExecuteWithContextFunc != nil {
		return m.ExecuteWithContextFunc(ctx, req, sessionCtx, priority)
	}
	return "", errors.New("not implemented")
}

func (m *MockTaskExecutor) ExecuteAsync(ctx context.Context, req session.TaskRequest) (string, error) {
	if m.ExecuteAsyncFunc != nil {
		return m.ExecuteAsyncFunc(ctx, req)
	}
	return "", errors.New("not implemented")
}

func (m *MockTaskExecutor) GetResult(taskID string) (string, bool, error) {
	if m.GetResultFunc != nil {
		return m.GetResultFunc(taskID)
	}
	return "", false, errors.New("not implemented")
}

func (m *MockTaskExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	if m.WaitForResultFunc != nil {
		return m.WaitForResultFunc(ctx, taskID)
	}
	return "", errors.New("not implemented")
}

func TestSpawnTask_InputValidation(t *testing.T) {
	t.Run("nil taskExecutor", func(t *testing.T) {
		o := &Orchestrator{}
		_, err := o.spawnTask(context.Background(), "coder", "do something")
		if err == nil || err.Error() != "taskExecutor not initialized" {
			t.Errorf("expected error 'taskExecutor not initialized', got %v", err)
		}
	})

	t.Run("empty shard type", func(t *testing.T) {
		o := &Orchestrator{
			taskExecutor: &MockTaskExecutor{
				ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
					// With the removal of LegacyShardNameToIntent, empty intent is passed as-is
					if req.IntentVerb != "" {
						return "", fmt.Errorf("expected empty intent to be passed directly, got %s", req.IntentVerb)
					}
					return "success", nil
				},
			},
		}
		res, err := o.spawnTask(context.Background(), "", "do something")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res != "success" {
			t.Errorf("expected 'success', got %v", res)
		}
	})

	t.Run("huge task payload", func(t *testing.T) {
		// Create a massive task payload (e.g. 10MB)
		hugePayload := make([]byte, 10*1024*1024)
		for i := range hugePayload {
			hugePayload[i] = 'A'
		}
		taskStr := string(hugePayload)

		o := &Orchestrator{
			taskExecutor: &MockTaskExecutor{
				ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
					if len(req.Task) != len(taskStr) {
						return "", errors.New("task payload size mismatch")
					}
					return "success", nil
				},
			},
		}
		res, err := o.spawnTask(context.Background(), "/fix", taskStr)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res != "success" {
			t.Errorf("expected 'success', got %v", res)
		}
	})

	t.Run("valid taskExecutor", func(t *testing.T) {
		o := &Orchestrator{
			taskExecutor: &MockTaskExecutor{
				ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
					return "success", nil
				},
			},
		}
		res, err := o.spawnTask(context.Background(), "/fix", "do something")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if res != "success" {
			t.Errorf("expected 'success', got %v", res)
		}
	})
}

func TestExecuteCampaignRefTask_MissingSubCampaignID(t *testing.T) {
	o := &Orchestrator{
		kernel: &MockKernel{},
	}
	task := &Task{
		ID:   "/task_campaign_ref",
		Type: TaskTypeCampaignRef,
	}

	res, err := o.executeCampaignRefTask(context.Background(), task)
	if err == nil {
		t.Fatal("expected error for missing sub_campaign_id")
	}
	if res != nil {
		t.Fatalf("expected nil result on error, got %#v", res)
	}
}

func TestExecuteCampaignRefTask_DefaultPolicyAndTypedEnvelope(t *testing.T) {
	eventCh := make(chan OrchestratorEvent, 1)
	o := &Orchestrator{
		kernel:    &MockKernel{},
		campaign:  &Campaign{ID: "/parent_campaign"},
		eventChan: eventCh,
	}
	task := &Task{
		ID:            "/task_campaign_ref",
		Type:          TaskTypeCampaignRef,
		SubCampaignID: "/child_campaign",
	}

	res, err := o.executeCampaignRefTask(context.Background(), task)
	if err != nil {
		t.Fatalf("executeCampaignRefTask() error = %v", err)
	}

	envelope, ok := res.(CampaignRefResult)
	if !ok {
		t.Fatalf("expected CampaignRefResult, got %T", res)
	}
	if envelope.Version != 1 {
		t.Fatalf("expected version 1, got %d", envelope.Version)
	}
	if envelope.SubCampaignID != "/child_campaign" {
		t.Fatalf("unexpected sub_campaign_id: %s", envelope.SubCampaignID)
	}
	if envelope.Status != CampaignRefLifecycleLinked {
		t.Fatalf("expected linked status, got %s", envelope.Status)
	}
	if envelope.FailurePolicy != CampaignRefPolicyPropagate {
		t.Fatalf("expected default propagate policy, got %s", envelope.FailurePolicy)
	}
	if envelope.Inheritance.FactsScope != "campaign_namespace_readonly" ||
		envelope.Inheritance.FSScope != "child_snapshot_rw" ||
		envelope.Inheritance.MemoryScope != "scoped_vector_campaign_namespace" ||
		envelope.Inheritance.ToolScope != "parent_tool_allowlist" {
		t.Fatalf("unexpected default inheritance: %#v", envelope.Inheritance)
	}

	select {
	case evt := <-eventCh:
		if evt.Type != "sub_campaign_referenced" {
			t.Fatalf("unexpected event type: %s", evt.Type)
		}
		data, ok := evt.Data.(map[string]any)
		if !ok {
			t.Fatalf("expected map data in event, got %T", evt.Data)
		}
		if data["failure_policy"] != string(CampaignRefPolicyPropagate) {
			t.Fatalf("expected event failure_policy %s, got %#v", CampaignRefPolicyPropagate, data["failure_policy"])
		}
	default:
		t.Fatal("expected sub_campaign_referenced event")
	}
}

func TestExecuteCampaignRefTask_FailurePolicyMapping(t *testing.T) {
	testCases := []struct {
		name             string
		policy           CampaignRefFailurePolicy
		expectError      bool
		expectedStatus   string
		expectedFactHint string
	}{
		{
			name:           "propagate",
			policy:         CampaignRefPolicyPropagate,
			expectError:    true,
			expectedStatus: CampaignRefLifecycleFailed,
		},
		{
			name:             "absorb",
			policy:           CampaignRefPolicyAbsorb,
			expectError:      false,
			expectedStatus:   CampaignRefLifecycleCompleted,
			expectedFactHint: "/campaign_ref_failure_absorbed",
		},
		{
			name:             "transform",
			policy:           CampaignRefPolicyTransform,
			expectError:      false,
			expectedStatus:   CampaignRefLifecycleCompleted,
			expectedFactHint: "/campaign_ref_failure_transformed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kernel := &MockKernel{}
			_ = kernel.Assert(core.Fact{
				Predicate: "campaign",
				Args:      []any{"/child_campaign", string(CampaignTypeFeature), "Child", "", string(StatusFailed)},
			})

			o := &Orchestrator{
				kernel: kernel,
			}
			task := &Task{
				ID:                       "/task_campaign_ref",
				Type:                     TaskTypeCampaignRef,
				SubCampaignID:            "/child_campaign",
				CampaignRefFailurePolicy: tc.policy,
				CampaignRefInheritance: &CampaignRefInheritance{
					ToolScope: "/isolate",
				},
			}

			res, err := o.executeCampaignRefTask(context.Background(), task)
			if tc.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), "failed state") {
					t.Fatalf("expected failed state error, got %v", err)
				}
				if res != nil {
					t.Fatalf("expected nil result when policy propagates, got %#v", res)
				}
				return
			}

			if err != nil {
				t.Fatalf("executeCampaignRefTask() error = %v", err)
			}
			envelope, ok := res.(CampaignRefResult)
			if !ok {
				t.Fatalf("expected CampaignRefResult, got %T", res)
			}
			if envelope.Status != tc.expectedStatus {
				t.Fatalf("expected status %s, got %s", tc.expectedStatus, envelope.Status)
			}
			if envelope.FailurePolicy != tc.policy {
				t.Fatalf("expected policy %s, got %s", tc.policy, envelope.FailurePolicy)
			}
			if envelope.FailureSummary == "" {
				t.Fatal("expected failure summary for failed child campaign")
			}
			if envelope.Inheritance.ToolScope != "/isolate" {
				t.Fatalf("expected tool scope override, got %#v", envelope.Inheritance)
			}
			if tc.expectedFactHint != "" {
				found := slices.Contains(envelope.LearnedFacts, tc.expectedFactHint)
				if !found {
					t.Fatalf("expected learned fact %s, got %#v", tc.expectedFactHint, envelope.LearnedFacts)
				}
			}
		})
	}
}

func TestLookupCampaignStatus_UsesLatestFact(t *testing.T) {
	kernel := &MockKernel{}
	_ = kernel.Assert(core.Fact{
		Predicate: "campaign",
		Args:      []any{"/child_campaign", string(CampaignTypeFeature), "Child", "", string(StatusActive)},
	})
	_ = kernel.Assert(core.Fact{
		Predicate: "campaign",
		Args:      []any{"/child_campaign", string(CampaignTypeFeature), "Child", "", string(StatusPaused)},
	})

	o := &Orchestrator{kernel: kernel}
	status, ok := o.lookupCampaignStatus("/child_campaign")
	if !ok {
		t.Fatal("expected campaign status to be found")
	}
	if status != StatusPaused {
		t.Fatalf("expected latest status %s, got %s", StatusPaused, status)
	}
}

func BenchmarkExtractPathFromDescription(b *testing.B) {
	descriptions := []string{
		"Create internal/domain/foo.go",
		"file: path/to/file.go",
		"cmd/nerd/main.go",
		"Just a description without path",
		"internal/pkg/utils.go needs update",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, desc := range descriptions {
			extractPathFromDescription(desc)
		}
	}
}

// -----------------------------------------------------------------------------
// Gap Implementations
// -----------------------------------------------------------------------------

func TestExecuteTask_NilTask(t *testing.T) {
	o := &Orchestrator{}
	_, err := o.executeTask(context.Background(), nil)
	if err == nil || err.Error() != "task cannot be nil" {
		t.Errorf("Expected nil task error, got %v", err)
	}
}

func TestExecuteTask_Dispatch(t *testing.T) {
	called := ""
	o := &Orchestrator{
		kernel: &MockKernel{},
		campaign: &Campaign{
			Phases: []Phase{
				{Tasks: []Task{{ID: "t1"}}},
			},
		},
		taskExecutor: &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				called = req.IntentVerb
				// Return a substantive (non-trivial) result so the F-HOLLOW-2
				// empty-result retry does not fire and re-route to /research;
				// this test asserts the primary explicit-shard routing only.
				return "This is a substantive analysis result describing real findings about the code under review.", nil
			},
		},
	}

	// Explicit shard override
	_, _ = o.executeTask(context.Background(), &Task{ID: "t1", Shard: "/custom", Description: "d"})
	if called != "/custom" {
		t.Errorf("Expected /custom shard routing, got %s", called)
	}
}

// TODO: Test scenarios where Task struct itself is nil, Task.ID is empty, or Description contains null bytes/non-printable chars.
func TestNullEmptyInputs(t *testing.T) {
	ctx := context.Background()
	o := &Orchestrator{
		taskExecutor: &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) { return "success", nil },
		},
	}

	// executeGenericTask empty description
	_, err := o.executeGenericTask(ctx, &Task{ID: "1", Description: ""})
	if err == nil || !strings.Contains(err.Error(), "description cannot be empty") {
		t.Errorf("Expected description error, got %v", err)
	}

	// executeFileTask missing paths (fallback triggers, then fails)
	_, err = o.executeFileTask(ctx, &Task{ID: "2", Description: "no path here"})
	if err == nil || !strings.Contains(err.Error(), "no target path") {
		t.Errorf("Expected no target path error, got %v", err)
	}
}

// TODO: Test scenarios where JSON payloads contain incorrect types (e.g., array instead of string) and maps are uninitialized (nil).
func TestTypeCoercion(t *testing.T) {
	ctx := context.Background()
	o := &Orchestrator{
		workspace: t.TempDir(),
	}

	// Path traversal
	_, err := o.executeFileTaskFallback(ctx, &Task{ID: "1", Description: "desc"}, "../../etc/passwd")
	if err == nil || !strings.Contains(err.Error(), "path traversal attempt") {
		t.Errorf("Expected path traversal error, got %v", err)
	}

	// extractCodeBlock pathological
	input := "````go\ncode\n```\n``` "
	extracted := extractCodeBlock(input, "go")
	if !strings.Contains(extracted, "code") {
		t.Errorf("extractCodeBlock failed on nested fences: %s", extracted)
	}
}

// TODO: Test extreme file sizes (10GB+), 1 million lines of base64 data, completely fabricated programming languages, and massive un-parseable JSON.
func TestUserRequestExtremes(t *testing.T) {
	// Massive string for extractCodeBlock
	massive := strings.Repeat("a", 50*1024*1024)
	start := time.Now()
	extractCodeBlock(massive, "go")
	if time.Since(start) > 2*time.Second {
		t.Errorf("extractCodeBlock too slow on massive string")
	}
}

// TODO: Test concurrent state conflicts (two tasks writing to the same file simultaneously), symlink loop path traversals, and mid-execution directory deletion.
func TestExecuteFileTask_ShardFailure_Fallback(t *testing.T) {
	ctx := context.Background()
	o := &Orchestrator{
		workspace: t.TempDir(),
		taskExecutor: &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				return "", errors.New("shard failure")
			},
		},
		llmClient: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
				return "```go\npackage main\n```", nil
			},
		},
	}

	// Shard fails -> fallback invoked
	res, err := o.executeFileTask(ctx, &Task{ID: "1", Description: "do it", Artifacts: []TaskArtifact{{Path: "test.go"}}})
	if err != nil {
		t.Errorf("Expected fallback success, got %v", err)
	}

	if resMap, ok := res.(map[string]any); ok {
		if _, exists := resMap["size"]; !exists {
			t.Errorf("Expected fallback result (size), got %v", resMap)
		}
	}
}

func TestExecuteFileTask_VerificationFailure(t *testing.T) {
	ctx := context.Background()
	o := &Orchestrator{
		workspace: t.TempDir(),
		taskExecutor: &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
				// Success but file is NOT created!
				return "I did it", nil
			},
		},
		llmClient: &MockLLMClient{
			CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
				return "```go\npackage main\n```", nil
			},
		},
	}

	res, err := o.executeFileTask(ctx, &Task{ID: "1", Description: "do it", Artifacts: []TaskArtifact{{Path: "test.go"}}})
	if err != nil {
		t.Errorf("Expected fallback success, got %v", err)
	}
	if resMap, ok := res.(map[string]any); ok {
		if _, exists := resMap["size"]; !exists {
			t.Errorf("Expected fallback result due to verification failure, got %v", resMap)
		}
	}
}

func TestExecuteToolCreateTask_Autopoiesis(t *testing.T) {
	ctx := t.Context()
	kernel := &MockKernel{
		Facts: []core.Fact{
			{Predicate: "has_capability", Args: []any{"git_read"}},
		},
	}
	o := &Orchestrator{
		kernel:   kernel,
		campaign: &Campaign{ID: "camp1", Goal: "goal"},
	}

	resChan := make(chan error)
	go func() {
		_, err := o.executeToolCreateTask(ctx, &Task{ID: "1", Description: "git_read"})
		resChan <- err
	}()

	err := <-resChan
	if err != nil {
		t.Errorf("executeToolCreateTask failed: %v", err)
	}

	// Context cancellation
	ctx2, cancel2 := context.WithCancel(context.Background())
	o2 := &Orchestrator{
		kernel:   &MockKernel{},
		campaign: &Campaign{ID: "camp1"},
	}

	cancel2() // cancel immediately
	_, err = o2.executeToolCreateTask(ctx2, &Task{ID: "1", Description: "git_read"})
	if err == nil || err != context.Canceled {
		t.Errorf("Expected context canceled, got %v", err)
	}
}
