package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/config"
	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/types"
)

const generatedSystemFixtureName = "generated_system_fixture"

func init() {
	if !strings.HasPrefix(filepath.Base(os.Args[0]), generatedSystemFixtureName) {
		return
	}
	var envelope struct {
		Input string `json:"input"`
	}
	if json.NewDecoder(os.Stdin).Decode(&envelope) != nil {
		os.Exit(9)
	}
	var args struct{ Path, Content, CountPath, Mode, Admission string }
	if json.Unmarshal([]byte(envelope.Input), &args) != nil {
		os.Exit(8)
	}
	content := args.Content
	if args.Mode == "bad-content" {
		content = "different bytes"
	}
	if err := os.WriteFile(args.Path, []byte(content), 0600); err != nil {
		fmt.Printf(`{"error":%q}`, err.Error())
		os.Exit(2)
	}
	if args.CountPath != "" {
		file, err := os.OpenFile(args.CountPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(7)
		}
		_, err = file.WriteString("effect\n")
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			os.Exit(6)
		}
	}
	if args.Mode == "fail" {
		fmt.Print(`{"output":"partial","error":"started fixture failed"}`)
		os.Exit(3)
	}
	if args.Mode == "block" {
		fmt.Print("partial live output")
		connection, err := net.Dial("tcp", args.Admission)
		if err != nil {
			os.Exit(5)
		}
		_, _ = connection.Write([]byte{1})
		var release [1]byte
		_, _ = connection.Read(release[:])
		_ = connection.Close()
	}
	fmt.Print(`{"output":"literal effect"}`)
	os.Exit(0)
}

func installGeneratedSystemFixture(t *testing.T, workspace string) types.GeneratedToolIdentity {
	t.Helper()
	toolsDir := filepath.Join(workspace, ".nerd", "tools")
	compiled := filepath.Join(toolsDir, ".compiled")
	if err := os.MkdirAll(compiled, 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(compiled, generatedSystemFixtureName+".exe")
	if err := os.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, generatedSystemFixtureName+".go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	identity := types.GeneratedToolIdentity{Name: generatedSystemFixtureName, BinaryPath: binary, BinaryHash: hex.EncodeToString(digest[:]), Protocol: types.GeneratedStdinV1}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary+".identity.json", encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return identity
}

func generatedSystemRequest(t *testing.T, workspace string, identity types.GeneratedToolIdentity, mode string) types.GeneratedToolRequest {
	t.Helper()
	identity.Workspace = workspace
	target := filepath.Join(workspace, "effect.txt")
	canonical, err := types.CanonicalGeneratedArgs(map[string]any{"Path": target, "Content": "literal bytes", "CountPath": filepath.Join(workspace, "effect-count.txt"), "Mode": mode})
	if err != nil {
		t.Fatal(err)
	}
	return types.GeneratedToolRequest{ScopeID: "scope", CallID: "effect-call", AuthorizationID: "exec-effect-call", Action: types.MangleAtom("/" + identity.Name), Target: target, CanonicalArgs: canonical, Tool: identity}
}

func generatedSystemAdapter(t *testing.T, workspace string) *orchestratorToolExecutor {
	t.Helper()
	orchestrator := autopoiesis.NewOrchestrator(&MockLLMClient{}, autopoiesis.DefaultConfig(workspace))
	adapter := newOrchestratorToolExecutor(orchestrator, context.Background())
	t.Cleanup(func() {
		if err := adapter.Close(); err != nil {
			t.Error(err)
		}
	})
	return adapter
}

func TestGeneratedFactoryDuplicateFailureAndFeedbackRetry(t *testing.T) {
	workspace := t.TempDir()
	identity := installGeneratedSystemFixture(t, workspace)
	adapter := generatedSystemAdapter(t, workspace)
	request := generatedSystemRequest(t, workspace, identity, "fail")
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var validations atomic.Int64
	validator := func(context.Context, types.GeneratedToolRequest, types.GeneratedToolReceipt) error {
		validations.Add(1)
		once.Do(func() { close(entered) })
		<-release
		return fmt.Errorf("failed process validation")
	}
	results := make(chan types.GeneratedToolReceipt, 2)
	var workers sync.WaitGroup
	defer func() {
		onceRelease := false
		select {
		case <-release:
			onceRelease = true
		default:
		}
		if !onceRelease {
			close(release)
		}
		workers.Wait()
	}()
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			receipt, _ := adapter.ExecuteGenerated(context.Background(), request, validator)
			results <- receipt
		}()
	}
	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		t.Fatal("execution did not enter validation")
	}
	conflicting := request
	conflicting.CanonicalArgs = `{"Path":"different"}`
	if receipt, err := adapter.ExecuteGenerated(context.Background(), conflicting, validator); err == nil || receipt.ProcessStarted {
		t.Fatalf("conflicting call admitted: %+v %v", receipt, err)
	}
	close(release)
	workers.Wait()
	for index := 0; index < 2; index++ {
		receipt := <-results
		if !receipt.ProcessStarted || receipt.BackendError == nil || receipt.Output != "partial" || !receipt.Feedback.Durable {
			t.Fatalf("failed live receipt: %+v", receipt)
		}
	}
	if validations.Load() != 1 {
		t.Fatalf("duplicate validation count=%d", validations.Load())
	}
	count, err := os.ReadFile(filepath.Join(workspace, "effect-count.txt"))
	if err != nil || string(count) != "effect\n" {
		t.Fatalf("effects=%q err=%v", count, err)
	}
	reopened := autopoiesis.NewLearningStore(filepath.Join(workspace, ".nerd", "tools", ".learnings"))
	if learning := reopened.GetLearning(identity.Name); learning == nil || learning.TotalExecutions != 1 || learning.SuccessRate != 0 {
		t.Fatalf("failed learning=%+v", learning)
	}

	// A new live call can execute, but failed publication cannot repeat it.
	request.CallID, request.AuthorizationID = "save-failure", "exec-save-failure"
	learnings := filepath.Join(workspace, ".nerd", "tools", ".learnings")
	backup := learnings + "-saved"
	if err := os.Rename(learnings, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(learnings, []byte("block publication"), 0600); err != nil {
		t.Fatal(err)
	}
	receipt, err := adapter.ExecuteGenerated(context.Background(), request, func(context.Context, types.GeneratedToolRequest, types.GeneratedToolReceipt) error { return nil })
	if err == nil || !receipt.ProcessStarted || receipt.Feedback.Durable || receipt.FeedbackError == nil {
		t.Fatalf("publication failure receipt=%+v err=%v", receipt, err)
	}
	if err := os.Remove(learnings); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backup, learnings); err != nil {
		t.Fatal(err)
	}
	receipt, _ = adapter.ExecuteGenerated(context.Background(), request, validator)
	if !receipt.Feedback.Durable || receipt.FeedbackError != nil {
		t.Fatalf("feedback retry=%+v", receipt)
	}
	count, err = os.ReadFile(filepath.Join(workspace, "effect-count.txt"))
	if err != nil || string(count) != "effect\neffect\n" {
		t.Fatalf("feedback retry repeated process: %q %v", count, err)
	}
	if learning := autopoiesis.NewLearningStore(learnings).GetLearning(identity.Name); learning == nil || learning.TotalExecutions != 2 {
		t.Fatalf("reopened count=%+v", learning)
	}
	if receipt, err := adapter.ExecuteGenerated(context.Background(), request, validator); err == nil || !receipt.Replayed || !receipt.Feedback.Durable {
		t.Fatalf("reconnect did not retain failed backend result: %+v %v", receipt, err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatal(err)
	}
	restarted := generatedSystemAdapter(t, workspace)
	if receipt, err := restarted.ExecuteGenerated(context.Background(), request, validator); err == nil || receipt.ProcessStarted {
		t.Fatalf("restart replay repeated persisted effect: %+v %v", receipt, err)
	}
	count, err = os.ReadFile(filepath.Join(workspace, "effect-count.txt"))
	if err != nil || string(count) != "effect\neffect\n" {
		t.Fatalf("reconnect/restart repeated effect: %q %v", count, err)
	}
}

func TestGeneratedFactoryCanceledProcessPersistsTerminalLearning(t *testing.T) {
	workspace := t.TempDir()
	identity := installGeneratedSystemFixture(t, workspace)
	adapter := generatedSystemAdapter(t, workspace)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	request := generatedSystemRequest(t, workspace, identity, "block")
	args, err := request.Args()
	if err != nil {
		t.Fatal(err)
	}
	args["admission"] = listener.Addr().String()
	request.CanonicalArgs, err = types.CanonicalGeneratedArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan types.GeneratedToolReceipt, 1)
	accepted := make(chan net.Conn, 1)
	processDone, acceptDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(processDone)
		receipt, _ := adapter.ExecuteGenerated(ctx, request, func(ctx context.Context, _ types.GeneratedToolRequest, _ types.GeneratedToolReceipt) error {
			return ctx.Err()
		})
		results <- receipt
	}()
	go func() { defer close(acceptDone); connection, _ := listener.Accept(); accepted <- connection }()
	var connection net.Conn
	defer func() {
		cancel()
		listener.Close()
		<-processDone
		<-acceptDone
		if connection != nil {
			connection.Close()
		} else {
			select {
			case remaining := <-accepted:
				if remaining != nil {
					remaining.Close()
				}
			default:
			}
		}
	}()
	select {
	case connection = <-accepted:
	case <-time.After(15 * time.Second):
		t.Fatal("process admission not observed")
	}
	if connection == nil {
		t.Fatal("child did not connect")
	}
	if err := connection.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var marker [1]byte
	if _, err := io.ReadFull(connection, marker[:]); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case receipt := <-results:
		if !receipt.ProcessStarted || !errors.Is(receipt.BackendError, context.Canceled) || !receipt.PartialOutput || !receipt.Feedback.Durable || receipt.ValidationPassed {
			t.Fatalf("canceled terminal receipt: %+v", receipt)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("canceled execution not joined")
	}
	reopened := autopoiesis.NewLearningStore(filepath.Join(workspace, ".nerd", "tools", ".learnings"))
	feedback, ok := reopened.ExecutionFeedback(request.Key())
	if !ok || !feedback.ProcessStarted || feedback.Success || !feedback.PartialOutput || feedback.Stdout != "partial live output" || feedback.BackendError == "" {
		t.Fatalf("canceled persisted feedback: %+v", feedback)
	}
	if learning := reopened.GetLearning(identity.Name); learning == nil || learning.TotalExecutions != 1 || learning.SuccessRate != 0 {
		t.Fatalf("canceled learning: %+v", learning)
	}
}

func TestGeneratedFactoryCloseCancelsAndJoinsAdmission(t *testing.T) {
	workspace := t.TempDir()
	identity := installGeneratedSystemFixture(t, workspace)
	adapter := generatedSystemAdapter(t, workspace)
	request := generatedSystemRequest(t, workspace, identity, "")
	entered, canceled, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan types.GeneratedToolReceipt, 1)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	workDone := make(chan struct{})
	defer func() { adapter.cancel(); unblock(); <-workDone }()
	go func() {
		defer close(workDone)
		receipt, _ := adapter.ExecuteGenerated(context.Background(), request, func(ctx context.Context, _ types.GeneratedToolRequest, _ types.GeneratedToolReceipt) error {
			close(entered)
			<-ctx.Done()
			close(canceled)
			<-release
			return ctx.Err()
		})
		finished <- receipt
	}()
	select {
	case <-entered:
	case <-time.After(15 * time.Second):
		t.Fatal("execution not admitted")
	}
	closed := make(chan error, 1)
	closeDone := make(chan struct{})
	go func() { defer close(closeDone); closed <- adapter.Close() }()
	defer func() { unblock(); <-closeDone }()
	select {
	case <-canceled:
	case <-time.After(15 * time.Second):
		t.Fatal("Close did not cancel execution")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned before execution joined: %v", err)
	default:
	}
	if receipt, err := adapter.ExecuteGenerated(context.Background(), request, func(context.Context, types.GeneratedToolRequest, types.GeneratedToolReceipt) error { return nil }); err == nil || receipt.ProcessStarted {
		t.Fatalf("closed admission: %+v %v", receipt, err)
	}
	unblock()
	select {
	case receipt := <-finished:
		if !errors.Is(receipt.ValidationError, context.Canceled) || !receipt.Feedback.Durable {
			t.Fatalf("canceled receipt=%+v", receipt)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("execution not joined")
	}
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Close did not drain")
	}
}

type generatedRefinementKernel struct{ *core.RealKernel }

func (k *generatedRefinementKernel) Query(query string) ([]core.Fact, error) {
	if strings.HasPrefix(query, "tool_needs_refinement(") {
		return []core.Fact{{Predicate: "tool_needs_refinement", Args: []any{generatedSystemFixtureName}}}, nil
	}
	return k.RealKernel.Query(query)
}

type generatedRefinementLLM struct {
	MockLLMClient
	entered, canceled, release chan struct{}
	once                       sync.Once
}

func (m *generatedRefinementLLM) Complete(ctx context.Context, _ string) (string, error) {
	m.once.Do(func() { close(m.entered) })
	<-ctx.Done()
	close(m.canceled)
	<-m.release
	return "", ctx.Err()
}
func (m *generatedRefinementLLM) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	return m.Complete(ctx, "")
}

func TestGeneratedFactoryCortexCloseJoinsRefinement(t *testing.T) {
	workspace := t.TempDir()
	model := &generatedRefinementLLM{entered: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
	orchestrator := autopoiesis.NewOrchestrator(model, autopoiesis.DefaultConfig(workspace))
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	kernel.SetWorkspace(workspace)
	orchestrator.SetKernel(&generatedRefinementKernel{RealKernel: kernel})
	adapter := newOrchestratorToolExecutor(orchestrator, context.Background())
	cortex := &Cortex{generatedExecutor: adapter}
	var released sync.Once
	unblock := func() { released.Do(func() { close(model.release) }) }
	defer func() {
		unblock()
		if err := cortex.Close(); err != nil {
			t.Error(err)
		}
	}()
	adapter.startRefinement(generatedSystemFixtureName)
	select {
	case <-model.entered:
	case <-time.After(15 * time.Second):
		t.Fatal("refinement never entered the actual model seam")
	}
	closed := make(chan error, 1)
	closeDone := make(chan struct{})
	go func() { defer close(closeDone); closed <- cortex.Close() }()
	defer func() { unblock(); <-closeDone }()
	select {
	case <-model.canceled:
	case <-time.After(15 * time.Second):
		t.Fatal("Cortex.Close did not cancel refinement")
	}
	select {
	case err := <-closed:
		t.Fatalf("Cortex.Close returned before refinement joined: %v", err)
	default:
	}
	unblock()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("refinement did not drain")
	}
}

type generatedProcessLLM struct {
	MockLLMClient
	args   map[string]any
	calls  atomic.Int64
	target string
}

func (m *generatedProcessLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return m.CompleteWithSystem(ctx, prompt, "")
}
func (m *generatedProcessLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	encoded, _ := json.Marshal(map[string]any{"understanding": map[string]any{"primary_intent": "analyze", "semantic_type": "query", "action_type": "analyze", "domain": "general", "scope": map[string]any{"level": "file", "target": m.target}, "confidence": 1.0, "suggested_approach": map[string]any{"mode": "normal", "primary_shard": "reviewer", "tools_needed": []string{generatedSystemFixtureName}}}, "surface_response": ""})
	return string(encoded), nil
}
func (m *generatedProcessLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if m.calls.Add(1) == 1 {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "effect-call", Name: generatedSystemFixtureName, Input: m.args}}}, nil
	}
	return &types.LLMToolResponse{Text: "Generated fixture completed."}, nil
}
func (m *generatedProcessLLM) CompleteWithToolResults(ctx context.Context, _ string, _ []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.CompleteWithTools(ctx, "", "", tools)
}

func TestGeneratedFactoryNormalBootProcessDurableLearning(t *testing.T) {
	workspace := t.TempDir()
	identity := installGeneratedSystemFixture(t, workspace)
	request := generatedSystemRequest(t, workspace, identity, "")
	args, err := request.Args()
	if err != nil {
		t.Fatal(err)
	}
	// Use lowercase path/content keys so the normal target and file validator
	// contracts apply; the fixture decoder is case insensitive.
	args = map[string]any{"path": request.Target, "content": "literal bytes", "countPath": filepath.Join(workspace, "effect-count.txt")}
	request.CanonicalArgs, err = types.CanonicalGeneratedArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	embeddingServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "fixture embedding unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(embeddingServer.Close)
	configuration := &config.UserConfig{Provider: "ollama", Engine: "api", Model: "fixture", Embedding: &config.EmbeddingConfig{Provider: "ollama", OllamaEndpoint: embeddingServer.URL, OllamaModel: "fixture", Dimensions: 2}}
	encoded, err := json.Marshal(configuration)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".nerd", "config.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	model := &generatedProcessLLM{args: args, target: request.Target}
	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{Workspace: workspace, LLMClientOverride: model})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cortex.Close(); err != nil {
			t.Error(err)
		}
	}()
	if cortex.generatedExecutor == nil || cortex.SessionExecutor == nil || cortex.JITCompiler == nil {
		t.Fatal("normal boot did not own generated/JIT/session route")
	}
	cortex.SessionExecutor.SetAgentConfig(&jitconfig.EffectiveAgentRuntimeConfig{IdentityPrompt: "Run the explicitly admitted generated fixture.", IntentVerb: "/analyze", Persona: "reviewer", AllowedTools: []string{generatedSystemFixtureName}, Policies: []string{"policy/constitution.mg"}})
	for _, fact := range []core.Fact{
		{Predicate: "permitted_action", Args: []any{request.AuthorizationID, request.Action, request.Target, request.CanonicalArgs, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{request.AuthorizationID, core.MangleAtom("/permit"), "exact fixture admission", time.Now().Unix()}},
		{Predicate: "critical_file", Args: []any{"baseline-only.go"}},
	} {
		if err := cortex.Kernel.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	ctx := types.WithGeneratedCallScope(context.Background(), "real-boot")
	result, err := cortex.SessionExecutor.Process(ctx, "Analyze through the explicitly admitted generated fixture.")
	if err != nil || result == nil || result.Error != nil || result.ToolCallsExecuted != 1 {
		t.Fatalf("Process result=%+v err=%v", result, err)
	}
	data, err := os.ReadFile(request.Target)
	if err != nil || string(data) != "literal bytes" {
		t.Fatalf("literal effect=%q err=%v", data, err)
	}
	count, err := os.ReadFile(filepath.Join(workspace, "effect-count.txt"))
	if err != nil || string(count) != "effect\n" {
		t.Fatalf("double route=%q err=%v", count, err)
	}
	pending, err := cortex.Kernel.Query("pending_action")
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending lease=%v err=%v", pending, err)
	}
	baseline, err := cortex.Kernel.Query(`critical_file("baseline-only.go")`)
	if err != nil || len(baseline) != 1 {
		t.Fatalf("baseline=%v err=%v", baseline, err)
	}
	if err := cortex.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := autopoiesis.NewLearningStore(filepath.Join(workspace, ".nerd", "tools", ".learnings"))
	ack, ok := reopened.ExecutionAcknowledgment("real-boot/exec-effect-call")
	feedback, recalled := reopened.ExecutionFeedback("real-boot/exec-effect-call")
	learning := reopened.GetLearning(identity.Name)
	if !ok || !ack.Durable || !recalled || !feedback.Success || feedback.Input != request.CanonicalArgs || feedback.TaskContext["authorization_id"] != request.AuthorizationID || feedback.TaskContext["call_id"] != request.CallID || learning == nil || learning.TotalExecutions != 1 {
		t.Fatalf("reopened ack=%+v feedback=%+v learning=%+v", ack, feedback, learning)
	}
}
