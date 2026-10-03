//go:build integration

package e2e_test

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/config"
	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/session"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

// These cases retain the ten original obligations without removing the imported
// tests. Root execution compiles deterministic tool source through the real
// ToolCompiler, then Boot restores its pinned binary. Only model responses and
// generated source are deterministic; no executor or feedback sink is replaced.
const productionPipelineChild = "e2e_generated_pipeline_child"

const productionPipelineSource = `package tools

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
)

func PipelineEffect(ctx context.Context, input string) (string, error) {
    var args map[string]string
    if err := json.Unmarshal([]byte(input), &args); err != nil { return "", err }
    workspace, err := os.Getwd()
    if err != nil { return "", err }
    for _, key := range []string{"path", "starts", "effects"} {
        rel, err := filepath.Rel(workspace, args[key])
        if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
            return "", fmt.Errorf("fixture path escapes workspace: %q", args[key])
        }
    }
    entry := map[string]string{"call_id": args["call_id"], "path": args["path"], "content": args["content"]}
    if err := appendJournal(args["starts"], entry); err != nil { return "", err }
    if args["gate"] != "" {
        request, err := http.NewRequestWithContext(ctx, http.MethodPost, args["gate"], strings.NewReader(args["call_id"]))
        if err != nil { return "", err }
        response, err := http.DefaultClient.Do(request)
        if err != nil { return "", err }
        _, readErr := io.Copy(io.Discard, response.Body)
        closeErr := response.Body.Close()
        if err := errors.Join(readErr, closeErr); err != nil { return "", err }
        if response.StatusCode != http.StatusOK { return "", fmt.Errorf("fixture admission status %d", response.StatusCode) }
    }
    if args["mode"] == "fail" { return "", errors.New("injected generated process failure") }
    if args["mode"] != "disconnect-effect" {
        if err := os.WriteFile(args["path"], []byte(args["content"]), 0600); err != nil { return "", err }
        if err := appendJournal(args["effects"], entry); err != nil { return "", err }
    }
    if args["mode"] == "partial-fail" {
        // Exit after a real effect with an actual partial protocol response.
        // Returning an error would let the wrapper discard that partial output.
        _ = json.NewEncoder(os.Stdout).Encode(map[string]any{"output": "partial fixture output", "error": "injected failure after literal file effect"})
        os.Exit(7)
    }
    return "literal effect: "+args["path"], nil
}

func appendJournal(path string, entry map[string]string) error {
    encoded, err := json.Marshal(entry)
    if err != nil { return err }
    file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
    if err != nil { return err }
    n, writeErr := file.Write(append(encoded, '\n'))
    if writeErr == nil && n != len(encoded)+1 { writeErr = io.ErrShortWrite }
    return errors.Join(writeErr, file.Close())
}
`

type pipelineJournalEntry struct {
	CallID  string `json:"call_id"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type pipelinePlan struct {
	Marker string
	Call   types.ToolCall
	Panic  bool
}

// Only model responses are replaced. The factory, transducer, JIT compiler,
// kernel, executive validators, process backend and learning store stay real.
type pipelineModel struct {
	mu        sync.Mutex
	plans     map[string]pipelinePlan
	issued    map[string]int
	observed  map[string]int
	piggyback bool
	envelopes int
	native    int
}

var _ types.LLMClient = (*pipelineModel)(nil)
var _ types.ToolResultsProvider = (*pipelineModel)(nil)
var _ types.PiggybackToolProvider = (*pipelineModel)(nil)

func (m *pipelineModel) ShouldUsePiggybackTools() bool { return m.piggyback }

func (m *pipelineModel) planFor(text string) (pipelinePlan, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	latest := -1
	var selected pipelinePlan
	for _, plan := range m.plans {
		if index := strings.LastIndex(text, plan.Marker); index > latest {
			latest, selected = index, plan
		}
	}
	return selected, latest >= 0
}

func (m *pipelineModel) Complete(ctx context.Context, prompt string) (string, error) {
	return m.CompleteWithSystem(ctx, "", prompt)
}

func (m *pipelineModel) CompleteWithSystem(ctx context.Context, systemPrompt, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	plan, _ := m.planFor(input)
	if m.piggyback && strings.Contains(systemPrompt, "## Available Tools") {
		m.mu.Lock()
		m.envelopes++
		m.mu.Unlock()
		// Actual malformed articulation envelope, containing a real effect request.
		args, err := json.Marshal(plan.Call.Input)
		if err != nil {
			return "", err
		}
		return `{"surface_response":"incomplete","control_packet":{"tool_requests":[{"id":"` + plan.Call.ID + `","tool_name":"` + productionPipelineChild + `","tool_args":` + string(args), nil
	}
	path, _ := plan.Call.Input["path"].(string)
	understanding := map[string]any{
		"primary_intent": "implement", "semantic_type": "mechanism", "action_type": "implement", "domain": "testing",
		"confidence": 1, "scope": map[string]any{"level": "file", "target": path, "file": path},
		"signals": map[string]any{"is_multi_step": false}, "suggested_approach": map[string]any{"mode": "normal"},
	}
	encoded, err := json.Marshal(map[string]any{"understanding": understanding, "surface_response": "Use the approved generated tool for the literal effect."})
	return string(encoded), err
}

func (m *pipelineModel) CompleteWithStreaming(ctx context.Context, systemPrompt, input string, _ bool) (<-chan string, <-chan error) {
	chunks, failures := make(chan string, 1), make(chan error, 1)
	text, err := m.CompleteWithSystem(ctx, systemPrompt, input)
	if err != nil {
		failures <- err
	} else {
		chunks <- text
	}
	close(chunks)
	close(failures)
	return chunks, failures
}

func (m *pipelineModel) CompleteWithTools(ctx context.Context, systemPrompt string, input string, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(systemPrompt), "You are an AI assistant helping with software development.") {
		return nil, errors.New("production JIT compilation fell back to the executor's baseline prompt")
	}
	plan, ok := m.planFor(input)
	if !ok {
		return nil, errors.New("production fixture received an uncorrelated model request")
	}
	found := false
	for _, definition := range definitions {
		if definition.Name == productionPipelineChild {
			found = true
		}
	}
	if !found {
		return nil, errors.New("production generated tool absent from compiled turn catalog")
	}
	m.mu.Lock()
	m.native++
	if m.issued[plan.Call.ID] != 0 {
		m.mu.Unlock()
		return &types.LLMToolResponse{Text: "The generated tool result was returned.", StopReason: "end_turn"}, nil
	}
	m.issued[plan.Call.ID]++
	m.mu.Unlock()
	if plan.Panic {
		panic("injected articulation model completion panic")
	}
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{plan.Call}, StopReason: "tool_use"}, nil
}

func (m *pipelineModel) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var input string
	for index := len(history) - 1; index >= 0; index-- {
		if history[index].Role == "user" && len(history[index].ToolResults) == 0 {
			if _, ok := m.planFor(history[index].Text); ok {
				input = history[index].Text
				break
			}
		}
	}
	plan, ok := m.planFor(input)
	if !ok {
		return nil, errors.New("production fixture history lost the actual turn identity")
	}
	for _, message := range history {
		for _, result := range message.ToolResults {
			if result.ToolUseID == plan.Call.ID {
				m.mu.Lock()
				m.observed[plan.Call.ID]++
				m.mu.Unlock()
				text := "Generated tool returned: " + result.Content
				if result.IsError {
					text = "Task incomplete; generated tool failed: " + result.Content
				}
				return &types.LLMToolResponse{Text: text, StopReason: "end_turn"}, nil
			}
		}
	}
	return m.CompleteWithTools(ctx, systemPrompt, input, definitions)
}

type pipelineFixture struct {
	t        *testing.T
	root     string
	cortex   *system.Cortex
	model    *pipelineModel
	identity types.GeneratedToolIdentity
	baseline []types.Fact
}

func newPipelineUserConfig(test *testing.T) *config.UserConfig {
	test.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, "fixture embedding unavailable", http.StatusServiceUnavailable)
	}))
	test.Cleanup(server.Close)
	configuration := config.DefaultUserConfig()
	configuration.Embedding = &config.EmbeddingConfig{Provider: "ollama", OllamaEndpoint: server.URL, OllamaModel: "fixture", Dimensions: 2}
	return configuration
}

func newPipelineFixture(t *testing.T, piggyback bool) *pipelineFixture {
	t.Helper()
	root := t.TempDir()
	compiled := filepath.Join(root, ".nerd", "tools", ".compiled")
	if err := os.MkdirAll(compiled, 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, ".nerd", "tools", productionPipelineChild+".go")
	if err := os.WriteFile(source, []byte(productionPipelineSource), 0600); err != nil {
		t.Fatal(err)
	}
	compileConfig := autopoiesis.DefaultOuroborosConfig(root)
	compileConfig.TargetOS, compileConfig.TargetArch = runtime.GOOS, runtime.GOARCH
	compileConfig.CompileTimeout = 90 * time.Second
	compileCtx, compileCancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer compileCancel()
	compiledTool, err := autopoiesis.NewToolCompiler(compileConfig).Compile(compileCtx, &autopoiesis.GeneratedTool{
		Name: productionPipelineChild, Description: "Deterministic literal effect and protocol fault fixture", Code: productionPipelineSource,
	})
	if err != nil || compiledTool == nil || !compiledTool.Success {
		t.Fatalf("production generated ToolCompiler: %+v, %v", compiledTool, err)
	}
	binary := compiledTool.OutputPath
	identity := types.GeneratedToolIdentity{Name: productionPipelineChild, BinaryPath: binary,
		BinaryHash: compiledTool.Hash, Protocol: types.GeneratedStdinV1, Workspace: root}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary+".identity.json", encoded, 0600); err != nil {
		t.Fatal(err)
	}
	model := &pipelineModel{plans: make(map[string]pipelinePlan), issued: make(map[string]int), observed: make(map[string]int), piggyback: piggyback}
	userConfig := newPipelineUserConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	cortex, err := system.BootCortexWithConfig(ctx, system.BootConfig{Workspace: root,
		SessionID: "production-generated-pipeline", UserConfigOverride: userConfig, LLMClientOverride: model})
	if err != nil {
		cancel()
		t.Fatalf("real production boot: %v", err)
	}
	t.Cleanup(func() {
		if err := cortex.Close(); err != nil {
			t.Errorf("production Cortex.Close: %v", err)
		}
		cancel()
	})
	if cortex.Kernel == nil || cortex.VirtualStore == nil || cortex.SessionExecutor == nil || cortex.Orchestrator == nil || cortex.JITCompiler == nil {
		t.Fatal("production boot omitted a mandatory pipeline component")
	}
	registry := cortex.VirtualStore.GetToolRegistry()
	if registry == nil {
		t.Fatal("production boot omitted its generated catalog")
	}
	restored, err := registry.GeneratedToolIdentity(productionPipelineChild)
	if err != nil || restored != identity {
		t.Fatalf("boot must restore immutable binary/protocol identity: got %+v, error %v", restored, err)
	}
	backendIdentity, err := cortex.Orchestrator.GeneratedToolIdentity(productionPipelineChild)
	if err != nil || backendIdentity.BinaryPath != identity.BinaryPath || backendIdentity.BinaryHash != identity.BinaryHash || backendIdentity.Protocol != identity.Protocol {
		t.Fatalf("factory/runtime restored identity mismatch: %+v, %v", backendIdentity, err)
	}
	f := &pipelineFixture{t: t, root: root, cortex: cortex, model: model, identity: identity}
	f.configure(cortex.SessionExecutor)
	baselinePath := filepath.Join(root, "baseline-only.go")
	baselineContents := "package fixture\n// immutable baseline\n"
	if err := os.WriteFile(baselinePath, []byte(baselineContents), 0600); err != nil {
		t.Fatal(err)
	}
	baselineHash := sha256.Sum256([]byte(baselineContents))
	f.baseline = []types.Fact{
		{Predicate: "critical_file", Args: []any{baselinePath}},
		{Predicate: "file_topology", Args: []any{baselinePath, hex.EncodeToString(baselineHash[:]), types.MangleAtom("/go"), int64(1), types.MangleAtom("/false")}},
	}
	for _, fact := range f.baseline {
		if err := cortex.Kernel.Assert(fact); err != nil {
			t.Fatalf("declared baseline %s: %v", fact.Predicate, err)
		}
	}
	f.assertBaseline()
	return f
}

func (f *pipelineFixture) configure(executor *session.Executor) {
	f.t.Helper()
	policies, err := core.DefaultPolicyFiles()
	if err != nil {
		f.t.Fatal(err)
	}
	cfg := &jitconfig.EffectiveAgentRuntimeConfig{IdentityPrompt: "Exercise the single approved generated tool and report its actual result.",
		IntentVerb: "/implement", Persona: "coder", AllowedTools: []string{productionPipelineChild}, Policies: policies}
	if err := cfg.Validate(); err != nil {
		f.t.Fatal(err)
	}
	// Host capability scope only. This does not grant Mangle permission or
	// replace the real JIT prompt compilation performed by Process.
	executor.SetAgentConfig(cfg)
}

func (f *pipelineFixture) plan(id, content, mode, gate string) pipelinePlan {
	f.t.Helper()
	plan := pipelinePlan{Marker: "[production-pipeline:" + id + "]", Call: types.ToolCall{ID: id, Name: productionPipelineChild,
		Input: map[string]any{"path": filepath.Join(f.root, id+".txt"), "content": content, "call_id": id,
			"starts": filepath.Join(f.root, "starts.jsonl"), "effects": filepath.Join(f.root, "effects.jsonl"), "mode": mode, "gate": gate}}}
	f.model.mu.Lock()
	f.model.plans[id] = plan
	f.model.mu.Unlock()
	return plan
}

func (f *pipelineFixture) request(plan pipelinePlan) types.GeneratedToolRequest {
	f.t.Helper()
	canonical, err := types.CanonicalGeneratedArgs(plan.Call.Input)
	if err != nil {
		f.t.Fatal(err)
	}
	return types.GeneratedToolRequest{ScopeID: "pipeline/" + plan.Call.ID, CallID: plan.Call.ID, AuthorizationID: "exec-" + plan.Call.ID,
		Action: types.MangleAtom("/" + productionPipelineChild), Target: plan.Call.Input["path"].(string), CanonicalArgs: canonical, Tool: f.identity}
}

func (f *pipelineFixture) approve(plan pipelinePlan) {
	f.t.Helper()
	request := f.request(plan)
	// Exact host approval through the existing constitution. No safe_action,
	// permitted, schema append, pending_action or wildcard policy is injected.
	for _, fact := range []types.Fact{
		{Predicate: "permitted_action", Args: []any{request.AuthorizationID, request.Action, request.Target, request.CanonicalArgs, time.Now().Unix()}},
		{Predicate: "permission_check_result", Args: []any{request.AuthorizationID, types.MangleAtom("/permit"), "explicit fixture host approval", time.Now().Unix()}},
	} {
		if err := f.cortex.Kernel.Assert(fact); err != nil {
			f.t.Fatalf("exact approval assertion: %v", err)
		}
	}
}

type pipelineOutcome struct {
	result  *session.ExecutionResult
	err     error
	escaped any
}

func (f *pipelineFixture) start(ctx context.Context, executor *session.Executor, plan pipelinePlan) <-chan pipelineOutcome {
	request := f.request(plan)
	ctx = types.WithGeneratedCallScope(ctx, request.ScopeID)
	done := make(chan pipelineOutcome, 1)
	go func() {
		var outcome pipelineOutcome
		// Capture only to report a product panic as a failed obligation; recovery
		// here never makes the positive witness pass or supplies product feedback.
		defer func() { outcome.escaped = recover(); done <- outcome }()
		outcome.result, outcome.err = executor.Process(ctx, plan.Marker+" Write the literal requested content to "+plan.Call.Input["path"].(string)+" using the approved generated tool.")
	}()
	return done
}

func awaitPipeline[T any](t *testing.T, channel <-chan T, label string) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(90 * time.Second):
		t.Fatalf("no channel acknowledgment for %s", label)
	}
	var zero T
	return zero
}

func (f *pipelineFixture) run(plan pipelinePlan) pipelineOutcome {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	return awaitPipeline(f.t, f.start(ctx, f.cortex.SessionExecutor, plan), "Process completion")
}

func (f *pipelineFixture) journal(path string) []pipelineJournalEntry {
	f.t.Helper()
	file, err := os.Open(filepath.Join(f.root, path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	defer file.Close()
	var entries []pipelineJournalEntry
	scanner := bufio.NewScanner(file)
	// Journal content includes the real near-limit input, not a duplicate fact flood.
	scanner.Buffer(make([]byte, 4096), 2*session.MaxActionPayloadBytes)
	for scanner.Scan() {
		var entry pipelineJournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			f.t.Fatalf("corrupt effect/admission journal: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		f.t.Fatal(err)
	}
	return entries
}

func (f *pipelineFixture) countJournal(path, callID string) int {
	count := 0
	for _, entry := range f.journal(path) {
		if entry.CallID == callID {
			count++
		}
	}
	return count
}

func (f *pipelineFixture) learning() *autopoiesis.LearningStore {
	return autopoiesis.NewLearningStore(filepath.Join(f.root, ".nerd", "tools", ".learnings"))
}

func (f *pipelineFixture) assertProductionRebootLearning(executions int) {
	f.t.Helper()
	if err := f.cortex.Close(); err != nil {
		f.t.Fatalf("production close before durable reopen: %v", err)
	}
	userConfig := newPipelineUserConfig(f.t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	reopened, err := system.BootCortexWithConfig(ctx, system.BootConfig{Workspace: f.root,
		SessionID: "production-generated-reopened", UserConfigOverride: userConfig, LLMClientOverride: f.model})
	if err != nil {
		cancel()
		f.t.Fatalf("production reboot: %v", err)
	}
	f.t.Cleanup(func() {
		if err := reopened.Close(); err != nil {
			f.t.Errorf("reopened production Cortex.Close: %v", err)
		}
		cancel()
	})
	if reopened.Orchestrator == nil || reopened.VirtualStore == nil || reopened.VirtualStore.GetToolRegistry() == nil {
		f.t.Fatal("production reboot omitted generated execution or automatic learning restoration")
	}
	learning := reopened.Orchestrator.GetToolLearning(productionPipelineChild)
	if learning == nil || learning.TotalExecutions != executions || learning.SuccessRate != 1 {
		f.t.Fatalf("second production boot failed to automatically reopen successful learning: %+v", learning)
	}
	identity, err := reopened.VirtualStore.GetToolRegistry().GeneratedToolIdentity(productionPipelineChild)
	if err != nil || identity != f.identity {
		f.t.Fatalf("second boot lost immutable generated identity: %+v, %v", identity, err)
	}
}

// One witness is shared by positives and fault controls. In particular, model
// prose, process output, an in-memory learning count, or a manually recorded
// feedback row cannot satisfy it.
func (f *pipelineFixture) positiveWitness(plan pipelinePlan, outcome pipelineOutcome) error {
	var problems []error
	if outcome.escaped != nil {
		problems = append(problems, fmt.Errorf("escaped production panic: %v", outcome.escaped))
	}
	if outcome.err != nil {
		problems = append(problems, outcome.err)
	}
	if outcome.result == nil {
		problems = append(problems, errors.New("missing Process result"))
	} else {
		if outcome.result.Error != nil {
			problems = append(problems, outcome.result.Error)
		}
		if outcome.result.ToolCallsExecuted != 1 || outcome.result.SuccessfulToolCalls != 1 {
			problems = append(problems, fmt.Errorf("expected exactly one successful actual call, attempted=%d successful=%d", outcome.result.ToolCallsExecuted, outcome.result.SuccessfulToolCalls))
		}
	}
	contents, err := os.ReadFile(plan.Call.Input["path"].(string))
	if err != nil || string(contents) != plan.Call.Input["content"].(string) {
		problems = append(problems, fmt.Errorf("literal effect missing or differs: %v", err))
	}
	if count := f.countJournal("starts.jsonl", plan.Call.ID); count != 1 {
		problems = append(problems, fmt.Errorf("process admission count %d", count))
	}
	if count := f.countJournal("effects.jsonl", plan.Call.ID); count != 1 {
		problems = append(problems, fmt.Errorf("literal effect count %d", count))
	}
	request := f.request(plan)
	store := f.learning()
	ack, acknowledged := store.ExecutionAcknowledgment(request.Key())
	if !acknowledged || !ack.Durable || ack.ExecutionID != request.Key() || ack.Fingerprint != request.Fingerprint() || ack.CommittedAt.IsZero() || ack.Path != filepath.Join(f.root, ".nerd", "tools", ".learnings", "tool_learnings.json") {
		problems = append(problems, errors.New("correlated durable acknowledgment missing after LearningStore reopen"))
	}
	feedback, recorded := store.ExecutionFeedback(request.Key())
	if !recorded || !feedback.Success || feedback.ToolName != productionPipelineChild || feedback.Input != request.CanonicalArgs || feedback.RequestFingerprint != request.Fingerprint() || feedback.Output != "literal effect: "+request.Target {
		problems = append(problems, errors.New("successful persisted execution feedback missing or differs"))
	}
	for key, want := range map[string]string{"call_id": request.CallID, "authorization_id": request.AuthorizationID, "action": string(request.Action), "target": request.Target, "binary_hash": request.Tool.BinaryHash, "protocol": string(request.Tool.Protocol)} {
		if feedback.TaskContext[key] != want {
			problems = append(problems, fmt.Errorf("persisted %s correlation differs", key))
		}
	}
	validation, err := f.cortex.Kernel.Query("action_verified")
	if err != nil {
		problems = append(problems, err)
	}
	validated := false
	for _, fact := range validation {
		if len(fact.Args) == 5 && types.ExtractString(fact.Args[0]) == request.AuthorizationID && types.ExtractString(fact.Args[1]) == string(request.Action) && types.ExtractString(fact.Args[2]) == "/hash" {
			validated = true
		}
	}
	if !validated {
		problems = append(problems, errors.New("real literal hash validation fact missing"))
	}
	f.model.mu.Lock()
	issued, observed := f.model.issued[plan.Call.ID], f.model.observed[plan.Call.ID]
	f.model.mu.Unlock()
	if issued != 1 || observed == 0 {
		problems = append(problems, fmt.Errorf("model/tool-result identity disconnected: issued=%d observed=%d", issued, observed))
	}
	return errors.Join(problems...)
}

func (f *pipelineFixture) assertBaseline() {
	f.t.Helper()
	for _, expected := range f.baseline {
		facts, err := f.cortex.Kernel.Query(expected.Predicate)
		if err != nil {
			f.t.Fatal(err)
		}
		// RealKernel.Query exposes ast.NameType as its exact string symbol.
		// Keep typed assertions above, and compare the full documented query
		// representation rather than requiring an unreturned Go named type.
		expectedArgs := append([]any(nil), expected.Args...)
		for index, arg := range expectedArgs {
			if atom, ok := arg.(types.MangleAtom); ok {
				expectedArgs[index] = string(atom)
			}
		}
		var actual []types.Fact
		for _, fact := range facts {
			if len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == types.ExtractString(expected.Args[0]) {
				actual = append(actual, fact)
			}
		}
		if len(actual) != 1 || !reflect.DeepEqual(actual[0].Args, expectedArgs) {
			f.t.Fatalf("declared baseline tuple changed: want %s%+v, got %+v", expected.Predicate, expectedArgs, actual)
		}
	}
	bytes, err := os.ReadFile(f.baseline[0].Args[0].(string))
	if err != nil || string(bytes) != "package fixture\n// immutable baseline\n" {
		f.t.Fatalf("baseline literal contents changed: %v", err)
	}
}

func (f *pipelineFixture) assertReleased(plan pipelinePlan) {
	f.t.Helper()
	facts, err := f.cortex.Kernel.Query("pending_action")
	if err != nil {
		f.t.Fatal(err)
	}
	for _, fact := range facts {
		if len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == "exec-"+plan.Call.ID {
			f.t.Fatalf("authorization still pending after completion: %+v", fact)
		}
	}
}

func (f *pipelineFixture) assertTotals(starts, effects, acknowledgments int) {
	f.t.Helper()
	if actual := len(f.journal("starts.jsonl")); actual != starts {
		f.t.Errorf("process starts=%d, want %d", actual, starts)
	}
	if actual := len(f.journal("effects.jsonl")); actual != effects {
		f.t.Errorf("literal effects=%d, want %d", actual, effects)
	}
	store := f.learning()
	learning := store.GetLearning(productionPipelineChild)
	if acknowledgments == 0 {
		if learning != nil && learning.TotalExecutions != 0 {
			f.t.Errorf("preflight refusal or failed publication fabricated learning: %+v", learning)
		}
	} else if learning == nil || learning.TotalExecutions != acknowledgments {
		f.t.Errorf("reopened learning count=%+v, want %d", learning, acknowledgments)
	}
	path := filepath.Join(f.root, ".nerd", "tools", ".learnings", "tool_learnings.json")
	data, err := os.ReadFile(path)
	if acknowledgments == 0 && errors.Is(err, os.ErrNotExist) {
		return
	}
	// A deliberate directory-as-file publication fault may report ENOTDIR.
	if acknowledgments == 0 && err != nil {
		if info, statErr := os.Stat(filepath.Dir(path)); statErr == nil && !info.IsDir() {
			return
		}
	}
	if err != nil {
		f.t.Fatalf("durable snapshot unreadable: %v", err)
	}
	var snapshot struct {
		Executions map[string]types.GeneratedLearningAck    `json:"executions"`
		Feedback   map[string]autopoiesis.ExecutionFeedback `json:"feedback"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		f.t.Fatal(err)
	}
	if len(snapshot.Executions) != acknowledgments || len(snapshot.Feedback) != acknowledgments {
		f.t.Errorf("durable acknowledgments=%d feedback=%d, want exactly %d", len(snapshot.Executions), len(snapshot.Feedback), acknowledgments)
	}
}

func (f *pipelineFixture) assertNoEffect(plan pipelinePlan) {
	f.t.Helper()
	if _, err := os.Stat(plan.Call.Input["path"].(string)); !errors.Is(err, os.ErrNotExist) {
		f.t.Fatalf("refused operation produced a file or unexpected stat error: %v", err)
	}
	if f.countJournal("effects.jsonl", plan.Call.ID) != 0 {
		f.t.Fatal("refused operation produced an effect journal entry")
	}
}

func assertPipelineFailed(t *testing.T, outcome pipelineOutcome) {
	t.Helper()
	if outcome.escaped != nil {
		t.Fatalf("production panic escaped instead of a visible failure: %v", outcome.escaped)
	}
	if outcome.err == nil && (outcome.result == nil || outcome.result.Error == nil) {
		t.Fatalf("production failure was not exposed: %+v", outcome.result)
	}
}

func pipelineFailureText(outcome pipelineOutcome) string {
	var resultError error
	var response string
	if outcome.result != nil {
		resultError, response = outcome.result.Error, outcome.result.Response
	}
	return fmt.Sprintf("%v %v %s", outcome.err, resultError, response)
}

func (f *pipelineFixture) assertFailedLearning(plan pipelinePlan) {
	f.t.Helper()
	request := f.request(plan)
	store := f.learning()
	ack, ok := store.ExecutionAcknowledgment(request.Key())
	if !ok || !ack.Durable || ack.Fingerprint != request.Fingerprint() || ack.ExecutionID != request.Key() || ack.CommittedAt.IsZero() || ack.Path != filepath.Join(f.root, ".nerd", "tools", ".learnings", "tool_learnings.json") {
		f.t.Fatalf("started failure lacks correlated durable acknowledgment: %+v", ack)
	}
	feedback, ok := store.ExecutionFeedback(request.Key())
	if !ok || feedback.Success || feedback.ErrorMsg == "" || feedback.Input != request.CanonicalArgs || feedback.RequestFingerprint != request.Fingerprint() || feedback.TaskContext["call_id"] != plan.Call.ID || feedback.TaskContext["authorization_id"] != request.AuthorizationID {
		f.t.Fatalf("started failure was lost or persisted as success: %+v", feedback)
	}
}

func TestE2E_ProductionGeneratedPipeline_Smoke(t *testing.T) {
	f := newPipelineFixture(t, false)
	plan := f.plan("smoke", "exact smoke effect\n", "", "")
	f.approve(plan)
	if err := f.positiveWitness(plan, f.run(plan)); err != nil {
		t.Fatal(err)
	}
	f.assertReleased(plan)
	f.assertBaseline()
	f.assertTotals(1, 1, 1)
	f.assertProductionRebootLearning(1)
	f.assertTotals(1, 1, 1)
}

func TestE2E_ProductionGeneratedPipeline_MalformedPiggyback_ContractViolation(t *testing.T) {
	f := newPipelineFixture(t, true)
	plan := f.plan("malformed", "must never land\n", "", "")
	f.approve(plan)
	outcome := f.run(plan)
	if outcome.escaped != nil {
		t.Fatalf("malformed articulation escaped: %v", outcome.escaped)
	}
	f.model.mu.Lock()
	envelopes := f.model.envelopes
	f.model.mu.Unlock()
	if envelopes == 0 {
		t.Fatal("malformed control never reached the actual articulation input")
	}
	f.assertNoEffect(plan)
	f.assertReleased(plan)
	f.assertBaseline()
	f.assertTotals(0, 0, 0)
	if _, ok := f.learning().ExecutionAcknowledgment(f.request(plan).Key()); ok {
		t.Fatal("malformed control fabricated an acknowledgment")
	}
}

type pipelineGate struct {
	server   *httptest.Server
	admitted chan string
	drained  chan string
	release  chan struct{}
	once     sync.Once
}

func newPipelineGate(t *testing.T) *pipelineGate {
	gate := &pipelineGate{admitted: make(chan string, 8), drained: make(chan string, 8), release: make(chan struct{})}
	gate.server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		bytes, err := io.ReadAll(io.LimitReader(request.Body, 4096))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		id := string(bytes)
		gate.admitted <- id
		defer func() { gate.drained <- id }()
		select {
		case <-gate.release:
			writer.WriteHeader(http.StatusOK)
		case <-request.Context().Done():
			return
		}
	}))
	t.Cleanup(func() { gate.open(); gate.server.Close() })
	return gate
}

func (g *pipelineGate) open() { g.once.Do(func() { close(g.release) }) }

func TestE2E_ProductionGeneratedPipeline_ContextCancellation_TemporalFailure(t *testing.T) {
	f := newPipelineFixture(t, false)
	gate := newPipelineGate(t)
	plan := f.plan("cancel-admitted", "must not land after cancellation\n", "", gate.server.URL)
	f.approve(plan)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := f.start(ctx, f.cortex.SessionExecutor, plan)
	if admitted := awaitPipeline(t, gate.admitted, "native process admission"); admitted != plan.Call.ID {
		t.Fatalf("wrong admitted call %q", admitted)
	}
	// Admission also proves the exact pending tuple survived until process start.
	request := f.request(plan)
	pending, err := f.cortex.Kernel.Query("pending_action")
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, fact := range pending {
		if len(fact.Args) == 5 && types.ExtractString(fact.Args[0]) == request.AuthorizationID && types.ExtractString(fact.Args[1]) == string(request.Action) && types.ExtractString(fact.Args[2]) == request.Target && types.ExtractString(fact.Args[3]) == request.CanonicalArgs {
			matched = true
		}
	}
	if !matched {
		t.Fatal("admitted generated process lost its exact pending authorization tuple")
	}
	cancel()
	outcome := awaitPipeline(t, done, "canceled Process and child join")
	assertPipelineFailed(t, outcome)
	if drained := awaitPipeline(t, gate.drained, "canceled child connection drain"); drained != plan.Call.ID {
		t.Fatalf("wrong drained call %q", drained)
	}
	f.assertNoEffect(plan)
	f.assertFailedLearning(plan)
	f.assertReleased(plan)
	f.assertBaseline()
	f.assertTotals(1, 0, 1)
	feedback, _ := f.learning().ExecutionFeedback(request.Key())
	if !strings.Contains(feedback.ErrorMsg, context.Canceled.Error()) {
		t.Fatalf("admitted cancellation was replaced by an unrelated failure: %+v", feedback)
	}
}

func TestE2E_ProductionGeneratedPipeline_StateCorruption(t *testing.T) {
	f := newPipelineFixture(t, false)
	gate := newPipelineGate(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	plans := []pipelinePlan{f.plan("concurrent-a", "distinct A\n", "", gate.server.URL), f.plan("concurrent-b", "distinct B\n", "", gate.server.URL)}
	var completions []<-chan pipelineOutcome
	for _, plan := range plans {
		f.approve(plan)
		executor := f.cortex.SessionExecutor.CloneForTask()
		executor.SetSessionID("isolation-" + plan.Call.ID)
		f.configure(executor)
		completions = append(completions, f.start(ctx, executor, plan))
	}
	admitted := make(map[string]bool)
	for range plans {
		admitted[awaitPipeline(t, gate.admitted, "distinct concurrent native process admission")] = true
	}
	if len(admitted) != len(plans) || !admitted[plans[0].Call.ID] || !admitted[plans[1].Call.ID] {
		t.Fatalf("concurrent calls were aliased or serialized: %+v", admitted)
	}
	gate.open()
	for index, plan := range plans {
		if err := f.positiveWitness(plan, awaitPipeline(t, completions[index], "distinct concurrent Process completion")); err != nil {
			t.Errorf("%s: %v", plan.Call.ID, err)
		}
		f.assertReleased(plan)
	}
	for range plans {
		awaitPipeline(t, gate.drained, "concurrent child connection drain")
	}
	f.assertBaseline()
	f.assertTotals(2, 2, 2)
}

func TestE2E_ProductionGeneratedPipeline_ResourceExhaustion(t *testing.T) {
	f := newPipelineFixture(t, false)
	// Test both sides of the actual production action payload bound. The
	// serialized argument size, not an invented duplicate-fact count, decides it.
	near := f.plan("near-bound", "", "", "")
	base, err := types.CanonicalGeneratedArgs(near.Call.Input)
	if err != nil {
		t.Fatal(err)
	}
	near.Call.Input["content"] = strings.Repeat("x", session.MaxActionPayloadBytes-len(base)-1)
	f.model.mu.Lock()
	f.model.plans[near.Call.ID] = near
	f.model.mu.Unlock()
	if size := len(f.request(near).CanonicalArgs); size != session.MaxActionPayloadBytes-1 {
		t.Fatalf("near-bound setup size=%d", size)
	}
	f.approve(near)
	if err := f.positiveWitness(near, f.run(near)); err != nil {
		t.Fatalf("below production bound: %v", err)
	}
	over := f.plan("over-bound", strings.Repeat("x", session.MaxActionPayloadBytes), "", "")
	if len(f.request(over).CanonicalArgs) <= session.MaxActionPayloadBytes {
		t.Fatal("oversized setup does not cross actual bound")
	}
	f.approve(over)
	outcome := f.run(over)
	assertPipelineFailed(t, outcome)
	if failure := pipelineFailureText(outcome); !strings.Contains(failure, "payload exceeds kernel action bound") && !strings.Contains(failure, "payload too large") {
		t.Fatalf("oversized input was refused for a reason other than the actual production bound: %s", failure)
	}
	f.assertNoEffect(over)
	f.assertReleased(near)
	f.assertReleased(over)
	f.assertBaseline()
	f.assertTotals(1, 1, 1)
	if _, ok := f.learning().ExecutionAcknowledgment(f.request(over).Key()); ok {
		t.Fatal("pre-admission oversized refusal produced feedback")
	}
}

func TestE2E_ProductionGeneratedPipeline_CascadingFailure_PanicRecovery(t *testing.T) {
	f := newPipelineFixture(t, false)
	panicPlan := f.plan("articulation-panic", "must not land\n", "", "")
	panicPlan.Panic = true
	f.model.mu.Lock()
	f.model.plans[panicPlan.Call.ID] = panicPlan
	f.model.mu.Unlock()
	f.approve(panicPlan)
	outcome := f.run(panicPlan)
	// This is the actual model completion/articulation seam. An escaping
	// product panic fails the obligation; the test observer does not repair it.
	if outcome.escaped != nil {
		t.Errorf("unresolved production articulation panic containment: %v", outcome.escaped)
	} else {
		assertPipelineFailed(t, outcome)
	}
	f.model.mu.Lock()
	injected := f.model.issued[panicPlan.Call.ID]
	f.model.mu.Unlock()
	if injected != 1 {
		t.Fatal("actual articulation panic injection was never reached")
	}
	f.assertNoEffect(panicPlan)
	f.assertReleased(panicPlan)
	f.assertBaseline()
	f.assertTotals(0, 0, 0)
	recovery := f.plan("after-articulation-panic", "recovered pipeline effect\n", "", "")
	f.approve(recovery)
	if err := f.positiveWitness(recovery, f.run(recovery)); err != nil {
		t.Errorf("next actual turn failed to recover: %v", err)
	}
	f.assertReleased(recovery)
	f.assertBaseline()
	f.assertTotals(1, 1, 1)
}

func TestE2E_ProductionGeneratedPipeline_Recovery_AfterFailure(t *testing.T) {
	f := newPipelineFixture(t, false)
	failed := f.plan("failed-process", "must not land\n", "fail", "")
	f.approve(failed)
	assertPipelineFailed(t, f.run(failed))
	f.assertNoEffect(failed)
	f.assertFailedLearning(failed)
	f.assertReleased(failed)
	f.assertBaseline()
	f.assertTotals(1, 0, 1)
	recovered := f.plan("recovered-process", "literal recovered effect\n", "", "")
	f.approve(recovered)
	if err := f.positiveWitness(recovered, f.run(recovered)); err != nil {
		t.Fatal(err)
	}
	f.assertReleased(recovered)
	f.assertBaseline()
	f.assertTotals(2, 1, 2)
	if learning := f.learning().GetLearning(productionPipelineChild); learning == nil || learning.SuccessRate != 0.5 {
		t.Fatalf("failure/recovery feedback history differs: %+v", learning)
	}
}

func TestE2E_ProductionGeneratedPipeline_EndToEndDataIntegrity(t *testing.T) {
	f := newPipelineFixture(t, false)
	f.assertBaseline()
	plan := f.plan("integrity", "precise unrelated effect\n", "", "")
	f.approve(plan)
	if err := f.positiveWitness(plan, f.run(plan)); err != nil {
		t.Fatal(err)
	}
	f.assertBaseline()
	f.assertReleased(plan)
	f.assertTotals(1, 1, 1)
}

func TestE2E_ProductionGeneratedPipeline_MultiTurnStateAccumulation(t *testing.T) {
	f := newPipelineFixture(t, false)
	var plans []pipelinePlan
	for index := 0; index < 5; index++ {
		plan := f.plan(fmt.Sprintf("actual-turn-%d", index+1), fmt.Sprintf("distinct turn %d\n", index+1), "", "")
		f.approve(plan)
		if err := f.positiveWitness(plan, f.run(plan)); err != nil {
			t.Fatalf("actual turn %d: %v", index+1, err)
		}
		plans = append(plans, plan)
		f.assertReleased(plan)
		f.assertBaseline()
		f.assertTotals(index+1, index+1, index+1)
		for _, earlier := range plans {
			contents, err := os.ReadFile(earlier.Call.Input["path"].(string))
			if err != nil || string(contents) != earlier.Call.Input["content"].(string) {
				t.Fatalf("turn %d corrupted earlier %s: %v", index+1, earlier.Call.ID, err)
			}
			ack, ok := f.learning().ExecutionAcknowledgment(f.request(earlier).Key())
			if !ok || ack.Fingerprint != f.request(earlier).Fingerprint() {
				t.Fatalf("turn %d lost or aliased earlier durable acknowledgment", index+1)
			}
		}
	}
}

func TestE2E_ProductionGeneratedPipeline_PartialPipelineFailure(t *testing.T) {
	f := newPipelineFixture(t, false)
	plan := f.plan("partial-process", "effect landed before failure\n", "partial-fail", "")
	f.approve(plan)
	outcome := f.run(plan)
	assertPipelineFailed(t, outcome)
	contents, err := os.ReadFile(plan.Call.Input["path"].(string))
	if err != nil || string(contents) != plan.Call.Input["content"].(string) {
		t.Fatalf("partial failure lost its actual effect: %v", err)
	}
	f.assertFailedLearning(plan)
	feedback, _ := f.learning().ExecutionFeedback(f.request(plan).Key())
	if feedback.Output != "partial fixture output" || !strings.Contains(feedback.ErrorMsg, "injected failure after literal file effect") {
		t.Fatalf("partial output/backend failure was not retained: %+v", feedback)
	}
	f.assertBaseline()
	f.assertReleased(plan)
	f.assertTotals(1, 1, 1)
}

func TestE2E_ProductionGeneratedPipeline_DisconnectedWitnessControls(t *testing.T) {
	t.Run("execution_bridge_disconnected", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		plan := f.plan("disconnected-execution", "must not land\n", "", "")
		f.approve(plan)
		f.cortex.VirtualStore.SetToolExecutor(nil)
		outcome := f.run(plan)
		assertPipelineFailed(t, outcome)
		if err := f.positiveWitness(plan, outcome); err == nil || !strings.Contains(err.Error(), "literal effect") || !strings.Contains(err.Error(), "durable acknowledgment") {
			t.Fatalf("disconnected execution did not falsify the positive witness: %v", err)
		}
		f.assertNoEffect(plan)
		f.assertReleased(plan)
		f.assertBaseline()
		f.assertTotals(0, 0, 0)
	})
	t.Run("output_claim_without_file_effect", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		plan := f.plan("disconnected-effect", "must not be inferred from output\n", "disconnect-effect", "")
		f.approve(plan)
		outcome := f.run(plan)
		assertPipelineFailed(t, outcome)
		if err := f.positiveWitness(plan, outcome); err == nil || !strings.Contains(err.Error(), "literal effect") {
			t.Fatalf("missing effect did not falsify the positive witness: %v", err)
		}
		f.assertNoEffect(plan)
		f.assertFailedLearning(plan)
		f.assertReleased(plan)
		f.assertBaseline()
		f.assertTotals(1, 0, 1)
	})
	t.Run("durable_feedback_publication_disconnected", func(t *testing.T) {
		f := newPipelineFixture(t, false)
		plan := f.plan("disconnected-feedback", "effect must execute only once\n", "", "")
		f.approve(plan)
		directory := filepath.Join(f.root, ".nerd", "tools", ".learnings")
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
		retained := directory + "-retained"
		if err := os.Rename(directory, retained); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(directory, []byte("publication deliberately blocked"), 0600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Remove(directory); err != nil {
				t.Error(err)
				return
			}
			if err := os.Rename(retained, directory); err != nil {
				t.Error(err)
			}
		})
		outcome := f.run(plan)
		assertPipelineFailed(t, outcome)
		if err := f.positiveWitness(plan, outcome); err == nil || !strings.Contains(err.Error(), "durable acknowledgment") {
			t.Fatalf("missing feedback did not falsify the positive witness: %v", err)
		}
		contents, err := os.ReadFile(plan.Call.Input["path"].(string))
		if err != nil || string(contents) != plan.Call.Input["content"].(string) {
			t.Fatalf("feedback fault replaced the admitted effect with preflight refusal: %v", err)
		}
		f.assertReleased(plan)
		f.assertBaseline()
		f.assertTotals(1, 1, 0)
	})
}
