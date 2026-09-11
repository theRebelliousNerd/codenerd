package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	working "codenerd/internal/context"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type WorkingWorld = working.WorkingWorld

func (e *Executor) SetWorkingWorld(world WorkingWorld) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workingWorld = world
}
func (s *Spawner) SetWorkingWorld(world WorkingWorld) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workingWorld = world
}

type workingLoopKey struct{}
type workingLoop struct {
	set          *working.WorkingSet
	focus        string
	recent       []string
	anchor       string
	budget       int
	prior        []types.Message
	observations map[string]string
}

func activeWorkingLoop(ctx context.Context) *workingLoop {
	value, _ := ctx.Value(workingLoopKey{}).(*workingLoop)
	return value
}

func (e *Executor) beginWorkingLoop(ctx context.Context, input string, cc *prompt.CompilationContext) (context.Context, func(), error) {
	e.mu.Lock()
	world, sessionID := e.workingWorld, e.sessionID
	if e.workingScope == "" {
		e.workingScope = fmt.Sprintf("%x", rand.Text())
	}
	scopeID := e.workingScope
	e.mu.Unlock()
	if world == nil {
		return ctx, func() {}, nil
	}
	root := e.workspaceForVerification()
	if root == "" {
		return ctx, func() {}, fmt.Errorf("working context requires a workspace")
	}
	scope := sessionID + "/" + cc.ShardID + "/" + scopeID
	set, err := working.NewWorkingSet(world, root, scope)
	if err != nil {
		return ctx, func() {}, err
	}
	focus := normalizeWorkingEntity(cc.IntentTarget, root)
	loop := &workingLoop{set: set, focus: focus, anchor: input, budget: 16384, prior: e.priorTurnMessages(), observations: make(map[string]string)}
	ctx = context.WithValue(ctx, workingLoopKey{}, loop)
	ctx = tools.WithContextRecall(ctx, set)
	return ctx, func() { _ = set.Close() }, nil
}

func normalizeWorkingEntity(target, root string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "."
	}
	abs, err := tools.ResolveWorkspacePath(context.Background(), root, target)
	if err != nil {
		return "."
	}
	root, err = tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return "."
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "."
	}
	return filepath.ToSlash(rel)
}

// recordWorkingResult runs at the effect boundary, before any later tool in the
// same batch can change the observed source revision. Failures remain failures.
func (e *Executor) recordWorkingResult(ctx context.Context, call types.ToolCall, body string, toolErr error) error {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return nil
	}
	entity := ""
	for _, key := range []string{"path", "file_path", "file", "target", "working_dir"} {
		if value, ok := call.Input[key].(string); ok && value != "" {
			entity = value
			break
		}
	}
	if entity == "" {
		if ref, ok := call.Input["ref"].(string); ok {
			if i := strings.LastIndex(ref, ":"); i > 1 {
				entity = ref[:i]
			}
		}
	}
	if entity != "" {
		loop.focus = normalizeWorkingEntity(entity, e.workspaceForVerification())
	}
	// The discarded error is recorded in the audit_json_errors baseline, whose
	// tool doc reaches the same conclusion from the other end; this is the note
	// that says so where the reader of this line actually is. Keep the two in
	// step if either changes.
	// ToolCall.Input is only ever built by json.Unmarshal into a map[string]any
	// (client_tool_helpers.go, xaioauth/tools.go), so its values are nil, bool,
	// float64, string, []any or map[string]any -- and JSON has no NaN or Inf
	// literal, which is the only way an Unmarshal result can refuse to Marshal.
	//
	// Worth stating what a failure WOULD cost, because it is not local: args
	// feeds a hash that becomes Kind, Kind is persisted on the record and is
	// what working_set.mg selects observations by. A nil args does not error
	// here, it makes every call of one tool hash identically regardless of its
	// arguments -- distinct observations silently collapsing into one kind. If
	// Input ever acquires a producer that is not an Unmarshal, that is the
	// consequence to design against.
	args, _ := json.Marshal(call.Input)
	sum := sha256.Sum256(append([]byte(call.Name+"\x00"), args...))
	kind := call.Name + "/" + hex.EncodeToString(sum[:8])
	if toolErr != nil {
		body += "\nERROR: " + toolErr.Error()
	}
	revision := loop.set.Revision(loop.focus)
	step := time.Now().UnixNano()
	idSum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", call.ID, kind, body, revision, step)))
	id := hex.EncodeToString(idSum[:])
	record := working.WorkingRecord{ID: id, Entity: loop.focus, Revision: revision, Kind: kind, Step: step, Body: body, Failed: toolErr != nil}
	if err := loop.set.Save(ctx, record); err != nil {
		return fmt.Errorf("persist working observation: %w", err)
	}
	loop.recent = append(loop.recent, id)
	loop.observations[call.ID] = id
	if len(loop.recent) > 16 {
		loop.recent = loop.recent[len(loop.recent)-16:]
	}
	return nil
}

func (e *Executor) prepareWorkingRequest(ctx context.Context, system string, history []types.Message) (string, []types.Message, error) {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return system, history, nil
	}
	// Keep the current native call/result pair intact. Earlier observations live
	// in the selected state, rather than an ever-growing provider transcript.
	start := len(history)
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" && len(history[i].ToolCalls) > 0 {
			start = i
			break
		}
	}
	messages := append([]types.Message(nil), loop.prior...)
	messages = append(messages, types.Message{Role: "user", Text: loop.anchor})
	if start < len(history) {
		for _, message := range history[start:] {
			copyMessage := message
			copyMessage.ToolResults = append([]types.ToolResult(nil), message.ToolResults...)
			for i := range copyMessage.ToolResults {
				if len(copyMessage.ToolResults[i].Content) > 8000 {
					id := loop.observations[copyMessage.ToolResults[i].ToolUseID]
					if id == "" {
						return "", nil, fmt.Errorf("oversize tool result has no durable observation")
					}
					copyMessage.ToolResults[i].Content = fmt.Sprintf("Observation archived: recall_context id=%q; retrieve its paginated body. Historical evidence requires a current revision check.", id)
				}
			}
			messages = append(messages, copyMessage)
		}
	}
	window := e.configSnapshot().TokenBudget
	if window <= 0 {
		window = DefaultTokenBudget()
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		return "", nil, err
	}
	remaining := window - prompt.EstimateTokens(system) - prompt.EstimateTokens(string(encoded))
	if remaining < 512 {
		return "", nil, fmt.Errorf("working request exceeds configured input budget; required instructions cannot be discarded")
	}
	selected, err := loop.set.Select(ctx, loop.focus, loop.recent, min(loop.budget, (remaining-256)*4))
	if err != nil {
		return "", nil, err
	}
	// Stable JIT instructions lead. The active state is regenerated each call;
	// current code views and observations stay adjacent to the current request.
	section := selected.Text
	view := e.withFileContext(ctx, "", loop.focus)
	if len(section)+len(view) <= min(loop.budget, (remaining-256)*4) {
		section = view + "\n" + section
	}
	if section != "" {
		system += "\n\n" + section
	}
	return system, messages, nil
}

func (e *Executor) completeWithWorkingContext(ctx context.Context, provider types.ToolResultsProvider, system string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if activeWorkingLoop(ctx) != nil {
		encoded, err := json.Marshal(definitions)
		if err != nil {
			return nil, err
		}
		// Reserve the actual catalog cost before selecting optional observations.
		loop := activeWorkingLoop(ctx)
		original := loop.budget
		loop.budget = max(0, original-len(encoded))
		defer func() { loop.budget = original }()
	}
	system, history, err := e.prepareWorkingRequest(ctx, system, history)
	if err != nil {
		return nil, fmt.Errorf("compile working context: %w", err)
	}
	return provider.CompleteWithToolResults(ctx, system, history, definitions)
}
