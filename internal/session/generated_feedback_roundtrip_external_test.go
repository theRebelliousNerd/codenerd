package session_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	userconfig "codenerd/internal/config"
	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

const sessionGeneratedFixtureName = "session_generated_fixture"

// The copied test executable is a real generated-tool binary. It accepts the
// generated stdin protocol, mutates its target once, and returns a JSON output
// envelope that the Process-level LLM must receive in its paired tool result.
func init() {
	if filepath.Base(os.Args[0]) != sessionGeneratedFixtureName+".exe" {
		return
	}
	var envelope struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&envelope); err != nil {
		os.Exit(9)
	}
	var args struct {
		Path      string
		Content   string
		CountPath string
	}
	if err := json.Unmarshal([]byte(envelope.Input), &args); err != nil {
		os.Exit(8)
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0600); err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		os.Exit(2)
	}
	count, err := os.OpenFile(args.CountPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(7)
	}
	if _, err := count.WriteString("effect\n"); err != nil {
		_ = count.Close()
		os.Exit(6)
	}
	if err := count.Close(); err != nil {
		os.Exit(5)
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"output": "fixture output: wrote " + args.Content})
	os.Exit(0)
}

type generatedRoundTripModel struct {
	mu       sync.Mutex
	callID   string
	toolName string
	args     map[string]any
	seen     []types.ToolResult
}

func (m *generatedRoundTripModel) setCall(callID string, args map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callID = callID
	m.args = args
}

func (m *generatedRoundTripModel) currentCall() (string, string, map[string]any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	args := make(map[string]any, len(m.args))
	for key, value := range m.args {
		args[key] = value
	}
	return m.callID, m.toolName, args
}

func (m *generatedRoundTripModel) resultFor(callID string) (types.ToolResult, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, result := range m.seen {
		if result.ToolUseID == callID {
			return result, true
		}
	}
	return types.ToolResult{}, false
}

func (m *generatedRoundTripModel) Complete(context.Context, string) (string, error) {
	return "unexpected text completion", nil
}

func (m *generatedRoundTripModel) CompleteWithSystem(context.Context, string, string) (string, error) {
	return "unexpected system completion", nil
}

func (m *generatedRoundTripModel) CompleteWithStreaming(ctx context.Context, _, _ string, _ bool) (<-chan string, <-chan types.StreamResult) {
	w, out, done := types.NewStream()
	w.Finish(types.StreamResult{})
	return out, done
}

func (m *generatedRoundTripModel) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	callID, toolName, args := m.currentCall()
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: callID, Name: toolName, Input: args}}}, nil
}

func (m *generatedRoundTripModel) CompleteWithToolResults(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	callID, toolName, args := m.currentCall()
	for _, message := range history {
		for _, result := range message.ToolResults {
			if result.ToolUseID != callID {
				continue
			}
			m.mu.Lock()
			m.seen = append(m.seen, result)
			m.mu.Unlock()
			return &types.LLMToolResponse{Text: "I received the generated tool result."}, nil
		}
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: callID, Name: toolName, Input: args}}}, nil
}

func installSessionGeneratedFixture(t *testing.T, workspace string) types.GeneratedToolIdentity {
	t.Helper()
	toolsDir := filepath.Join(workspace, ".nerd", "tools")
	compiledDir := filepath.Join(toolsDir, ".compiled")
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binaryBytes, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	binaryPath := filepath.Join(compiledDir, sessionGeneratedFixtureName+".exe")
	if err := os.WriteFile(binaryPath, binaryBytes, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolsDir, sessionGeneratedFixtureName+".go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(binaryBytes)
	identity := types.GeneratedToolIdentity{
		Name:       sessionGeneratedFixtureName,
		BinaryPath: binaryPath,
		BinaryHash: hex.EncodeToString(digest[:]),
		Protocol:   types.GeneratedStdinV1,
		Workspace:  workspace,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath+".identity.json", encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return identity
}

func generatedRoundTripArgs(workspace, name, content string) map[string]any {
	return map[string]any{
		"path":      filepath.Join(workspace, name),
		"content":   content,
		"countPath": filepath.Join(workspace, "effect-count.txt"),
	}
}

func assertGeneratedPermission(t *testing.T, kernel core.Kernel, callID string, args map[string]any) types.GeneratedToolRequest {
	t.Helper()
	canonical, err := types.CanonicalGeneratedArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	target := args["path"].(string)
	request := types.GeneratedToolRequest{
		ScopeID:         "unused",
		CallID:          callID,
		AuthorizationID: "exec-" + callID,
		Action:          types.MangleAtom("/" + sessionGeneratedFixtureName),
		Target:          target,
		CanonicalArgs:   canonical,
	}
	for _, fact := range []core.Fact{
		{Predicate: "permitted_action", Args: []any{request.AuthorizationID, request.Action, request.Target, request.CanonicalArgs, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{request.AuthorizationID, core.MangleAtom("/permit"), "exact generated fixture admission", time.Now().Unix()}},
	} {
		if err := kernel.Assert(fact); err != nil {
			t.Fatal(err)
		}
	}
	return request
}

func TestExecutor_Process_RealGeneratedBinaryFeedbackRoundTrip(t *testing.T) {
	t.Parallel()
	workspace := t.TempDir()
	identity := installSessionGeneratedFixture(t, workspace)
	model := &generatedRoundTripModel{toolName: identity.Name}
	userConfig := userconfig.DefaultUserConfig()
	userConfig.Embedding = &userconfig.EmbeddingConfig{Provider: "none"}
	cortex, err := system.BootCortexWithConfig(context.Background(), system.BootConfig{
		Workspace:          workspace,
		UserConfigOverride: userConfig,
		LLMClientOverride:  model,
	})
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	defer func() {
		if !closed {
			if err := cortex.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	if cortex.SessionExecutor == nil || cortex.JITCompiler == nil || cortex.VirtualStore == nil {
		t.Fatal("normal boot did not wire the JIT/session/generated execution route")
	}
	cortex.SessionExecutor.SetAgentConfig(&jitconfig.EffectiveAgentRuntimeConfig{
		IdentityPrompt: "Run the explicitly admitted generated fixture.",
		IntentVerb:     "/analyze",
		Persona:        "reviewer",
		AllowedTools:   []string{identity.Name},
		Policies:       []string{"policy/constitution.mg"},
	})
	if err := cortex.Kernel.Assert(core.Fact{Predicate: "critical_file", Args: []any{"baseline-only.go"}}); err != nil {
		t.Fatal(err)
	}

	callID := "roundtrip-call"
	args := generatedRoundTripArgs(workspace, "roundtrip.txt", "durable bytes")
	request := assertGeneratedPermission(t, cortex.Kernel, callID, args)
	request.ScopeID = "session-real-boot"
	model.setCall(callID, args)
	ctx := types.WithGeneratedCallScope(context.Background(), request.ScopeID)
	result, err := cortex.SessionExecutor.ProcessWithIntent(ctx, "Write through the admitted generated fixture.", &perception.Intent{Verb: "/analyze", Category: "/query"})
	if err != nil || result == nil || result.Error != nil || result.ToolCallsExecuted != 1 {
		t.Fatalf("successful Process result=%+v err=%v", result, err)
	}
	model.mu.Lock()
	seen := append([]types.ToolResult(nil), model.seen...)
	model.mu.Unlock()
	data, err := os.ReadFile(args["path"].(string))
	if err != nil || string(data) != "durable bytes" {
		t.Fatalf("real generated effect=%q err=%v result=%+v toolResults=%+v", data, err, result, seen)
	}
	countPath := filepath.Join(workspace, "effect-count.txt")
	count, err := os.ReadFile(countPath)
	if err != nil || string(count) != "effect\n" {
		t.Fatalf("generated binary invocation count=%q err=%v", count, err)
	}
	if len(seen) != 1 || seen[0].ToolUseID != callID || seen[0].IsError || !strings.Contains(seen[0].Content, "fixture output: wrote durable bytes") {
		t.Fatalf("provider did not receive the real paired generated output: %+v", seen)
	}

	// The same live Process path must deny an unapproved exact payload before
	// it reaches the binary or earns a durable execution acknowledgment.
	deniedID := "denied-call"
	deniedArgs := generatedRoundTripArgs(workspace, "denied.txt", "must not write")
	deniedRequest := types.GeneratedToolRequest{ScopeID: "session-denied", CallID: deniedID, AuthorizationID: "exec-" + deniedID}
	model.setCall(deniedID, deniedArgs)
	denied, deniedErr := cortex.SessionExecutor.ProcessWithIntent(
		types.WithGeneratedCallScope(context.Background(), deniedRequest.ScopeID),
		"Try the generated fixture without exact approval.",
		&perception.Intent{Verb: "/analyze", Category: "/query"},
	)
	deniedFeedback, deniedReturned := model.resultFor(deniedID)
	if denied == nil || !deniedReturned || !deniedFeedback.IsError || !strings.Contains(deniedFeedback.Content, "action not permitted") {
		t.Fatalf("exact authorization refusal was not returned as a tool error: result=%+v err=%v toolResult=%+v", denied, deniedErr, deniedFeedback)
	}
	if _, err := os.Stat(deniedArgs["path"].(string)); !os.IsNotExist(err) {
		t.Fatalf("unauthorized generated call produced an effect (stat err=%v)", err)
	}

	// A valid authorization still cannot cross a disconnected production
	// bridge. This control guards the Process-to-VirtualStore seam as well as
	// the kernel permission seam above.
	bridgeID := "bridge-call"
	bridgeArgs := generatedRoundTripArgs(workspace, "bridge.txt", "must not write")
	bridgeRequest := assertGeneratedPermission(t, cortex.Kernel, bridgeID, bridgeArgs)
	bridgeRequest.ScopeID = "session-bridge-missing"
	cortex.VirtualStore.SetToolExecutor(nil)
	model.setCall(bridgeID, bridgeArgs)
	bridgeResult, bridgeErr := cortex.SessionExecutor.ProcessWithIntent(
		types.WithGeneratedCallScope(context.Background(), bridgeRequest.ScopeID),
		"Attempt the fixture with the generated bridge disconnected.",
		&perception.Intent{Verb: "/analyze", Category: "/query"},
	)
	bridgeFeedback, bridgeReturned := model.resultFor(bridgeID)
	if bridgeResult == nil || !bridgeReturned || !bridgeFeedback.IsError || !strings.Contains(bridgeFeedback.Content, "mandatory generated execution bridge disconnected") {
		t.Fatalf("disconnected generated bridge refusal was not returned as a tool error: result=%+v err=%v toolResult=%+v", bridgeResult, bridgeErr, bridgeFeedback)
	}
	if _, err := os.Stat(bridgeArgs["path"].(string)); !os.IsNotExist(err) {
		t.Fatalf("disconnected generated bridge produced an effect (stat err=%v)", err)
	}
	count, err = os.ReadFile(countPath)
	if err != nil || string(count) != "effect\n" {
		t.Fatalf("refused attempts reached generated binary: count=%q err=%v", count, err)
	}

	if err := cortex.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	reopened := autopoiesis.NewLearningStore(filepath.Join(workspace, ".nerd", "tools", ".learnings"))
	ack, ackOK := reopened.ExecutionAcknowledgment(request.Key())
	feedback, feedbackOK := reopened.ExecutionFeedback(request.Key())
	learning := reopened.GetLearning(identity.Name)
	if !ackOK || !ack.Durable || !feedbackOK || !feedback.Success || feedback.Input != request.CanonicalArgs ||
		feedback.TaskContext["authorization_id"] != request.AuthorizationID || feedback.TaskContext["call_id"] != request.CallID ||
		learning == nil || learning.TotalExecutions != 1 {
		t.Fatalf("reopened real feedback ack=%+v feedback=%+v learning=%+v", ack, feedback, learning)
	}
	for _, refused := range []types.GeneratedToolRequest{deniedRequest, bridgeRequest} {
		if _, ok := reopened.ExecutionAcknowledgment(refused.Key()); ok {
			t.Errorf("refused request %s fabricated a durable execution acknowledgment", refused.Key())
		}
		if _, ok := reopened.ExecutionFeedback(refused.Key()); ok {
			t.Errorf("refused request %s fabricated durable feedback", refused.Key())
		}
	}
}

var _ perception.LLMClient = (*generatedRoundTripModel)(nil)
