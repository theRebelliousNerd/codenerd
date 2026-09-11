package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
	prior        []types.Message
	observations map[string]string
}

// workingSectionCeiling bounds the observations section of a working request
// at what the transcript it replaces was allowed to cost. The working context
// exists so the provider transcript stops growing with every tool result; a
// section allowed to grow to the whole input window would put that growth
// back, at 1M-window prices, on every round of a long turn. Within the ceiling
// the policy chooses what is shown; what it leaves out stays recallable.
const workingSectionCeiling = maxToolLoopHistoryBytes

// workingReplyReserve is the part of the input window kept free of working
// context so the request is never sent at exactly the budget.
const workingReplyReserve = 256

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
	loop := &workingLoop{set: set, focus: focus, anchor: input, prior: e.priorTurnMessages(), observations: make(map[string]string)}
	ctx = context.WithValue(ctx, workingLoopKey{}, loop)
	ctx = tools.WithContextRecall(ctx, set)
	return ctx, func() { _ = set.Close() }, nil
}

// normalizeWorkingEntity turns a target into the workspace entity observations
// are recorded under. A target that is not a workspace path — an intent target
// is often a phrase describing the change, not a file — resolves to the
// workspace root rather than to a path that exists nowhere, so the first
// round's observations are not filed under a sentence.
func normalizeWorkingEntity(target, root string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return "."
	}
	abs, err := tools.ResolveWorkspacePath(context.Background(), root, target)
	if err != nil {
		return "."
	}
	if _, err := os.Stat(abs); err != nil {
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

// prepareWorkingRequest builds the provider request for one round of a working
// loop: the prior turns, the anchor, the current native call/result pair, and a
// system prompt carrying the observations the working policy selected.
//
// The current pair is sent whole. Observed 2026-09-11: a 14 KB read of the
// file the brief named was swapped for an "archived, recall it" pointer on a
// fixed 8000-character rule, and the model spent 24 rounds reading the file,
// recalling a 2000-character page of it and reading it again, without ever
// reaching the line it was asked to change. A result the model just asked for
// is only archived when the request cannot otherwise fit the input window,
// largest first, and the pointer then says how large the body is.
//
// The catalog of tool definitions is part of the request, so its cost comes
// off the window like the system prompt and the transcript do. It used to be
// subtracted from a fixed 16 KB section budget instead: a catalog of 26 tools
// is larger than that, so the section budget was zero, no observation was ever
// selected, and the model started every round with nothing but the anchor.
func (e *Executor) prepareWorkingRequest(ctx context.Context, system string, history []types.Message, definitions []types.ToolDefinition) (string, []types.Message, error) {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return system, history, nil
	}
	// Keep the last rounds of native call/result pairs intact, as many as the
	// policy says. Older observations live in the selected state rather than
	// an ever-growing provider transcript. Only the current pair was kept
	// before, and a model that saw no turn of its own before this one started
	// every round from scratch (see working_transcript_rounds in the policy).
	rounds, err := loop.set.TranscriptRounds(ctx)
	if err != nil {
		return "", nil, err
	}
	start := len(history)
	for i, kept := len(history)-1, 0; i >= 0; i-- {
		if history[i].Role == "assistant" && len(history[i].ToolCalls) > 0 {
			start = i
			if kept++; kept >= rounds {
				break
			}
		}
	}
	messages := append([]types.Message(nil), loop.prior...)
	messages = append(messages, types.Message{Role: "user", Text: loop.anchor})
	var shown []string
	if start < len(history) {
		for _, message := range history[start:] {
			copyMessage := message
			copyMessage.ToolResults = append([]types.ToolResult(nil), message.ToolResults...)
			messages = append(messages, copyMessage)
			for _, call := range message.ToolCalls {
				if id := loop.observations[call.ID]; id != "" {
					shown = append(shown, id)
				}
			}
		}
	}
	window := e.configSnapshot().TokenBudget
	if window <= 0 {
		window = DefaultTokenBudget()
	}
	catalog := 0
	if len(definitions) > 0 {
		encoded, err := json.Marshal(definitions)
		if err != nil {
			return "", nil, err
		}
		catalog = prompt.EstimateTokens(string(encoded))
	}
	remaining, err := workingWindowRemaining(window, system, messages, catalog)
	if err != nil {
		return "", nil, err
	}
	for remaining < workingReplyReserve+512 {
		i, j, size := largestToolResult(messages)
		if size == 0 {
			return "", nil, fmt.Errorf("working request exceeds configured input budget; required instructions cannot be discarded")
		}
		result := &messages[i].ToolResults[j]
		id := loop.observations[result.ToolUseID]
		if id == "" {
			return "", nil, fmt.Errorf("oversize tool result has no durable observation")
		}
		result.Content = fmt.Sprintf("%s this %d-character result does not fit the request; recall_context id=%q returns it from offset 0, or in offset/limit pages. Historical evidence requires a current revision check.", archivedResultPrefix, size, id)
		if remaining, err = workingWindowRemaining(window, system, messages, catalog); err != nil {
			return "", nil, err
		}
	}
	budget := min((remaining-workingReplyReserve)*4, workingSectionCeiling)
	selected, err := loop.set.Select(ctx, loop.focus, loop.recent, shown, budget)
	if err != nil {
		return "", nil, err
	}
	// Stable JIT instructions lead. The active state is regenerated each call;
	// current code views and observations stay adjacent to the current request.
	section := selected.Text
	view := e.withFileContext(ctx, "", loop.focus)
	if len(section)+len(view) <= budget {
		section = view + "\n" + section
	}
	if section != "" {
		system += "\n\n" + section
	}
	return system, messages, nil
}

// archivedResultPrefix opens the pointer that replaces a tool result the
// request could not carry. It is also how largestToolResult recognises a
// result it has already archived.
const archivedResultPrefix = "Observation archived:"

// workingWindowRemaining is the input window left after the system prompt, the
// transcript and the tool catalog, in tokens.
func workingWindowRemaining(window int, system string, messages []types.Message, catalog int) (int, error) {
	encoded, err := json.Marshal(messages)
	if err != nil {
		return 0, err
	}
	return window - prompt.EstimateTokens(system) - prompt.EstimateTokens(string(encoded)) - catalog, nil
}

// largestToolResult locates the largest tool result in the transcript that has
// not already been archived. Zero size means there is nothing left to archive.
func largestToolResult(messages []types.Message) (mi, ri, size int) {
	for i := range messages {
		for j := range messages[i].ToolResults {
			content := messages[i].ToolResults[j].Content
			if n := len(content); n > size && !strings.HasPrefix(content, archivedResultPrefix) {
				mi, ri, size = i, j, n
			}
		}
	}
	return mi, ri, size
}

func (e *Executor) completeWithWorkingContext(ctx context.Context, provider types.ToolResultsProvider, system string, history []types.Message, definitions []types.ToolDefinition) (*types.LLMToolResponse, error) {
	system, history, err := e.prepareWorkingRequest(ctx, system, history, definitions)
	if err != nil {
		return nil, fmt.Errorf("compile working context: %w", err)
	}
	return provider.CompleteWithToolResults(ctx, system, history, definitions)
}
