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
	regime       string // the policy's working_regime for the next round
	// searchOpen is the policy's working_search_open. False at the start of a
	// loop: the raw search tools are withheld until the policy derives it.
	searchOpen bool
}

// commitRegime is the working_regime under which exploration is closed.
const commitRegime = "commit"

// closedForReading says whether a tool is withheld under the commit regime:
// every read-effect and external-effect tool except recall_context, which
// recovers evidence the loop already gathered rather than exploring for more.
// External tools are exploration too: observed 2026-09-11, a model under the
// regime reached for mcp_context and mcp_map instead of writing.
func closedForReading(name string) bool {
	if name == "recall_context" {
		return false
	}
	effect, err := tools.LookupEffect(name)
	return err == nil && (effect == tools.EffectRead || effect == tools.EffectExternal)
}

// enterCommitRegime puts the active working loop under the commit regime and
// returns the call that restores what it was. A no-op outside a working loop.
func (e *Executor) enterCommitRegime(ctx context.Context) func() {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return func() {}
	}
	previous := loop.regime
	loop.regime = commitRegime
	return func() { loop.regime = previous }
}

// structuralFirstDefinitions is the catalog offered while the policy has not
// derived working_search_open: the raw search tools are withheld, provided the
// catalog holds the structural query that stands in for them. A persona that
// was never given find_symbol keeps its search tools; withholding grep with
// nothing in its place is a brick, not a policy.
func structuralFirstDefinitions(definitions []types.ToolDefinition) []types.ToolDefinition {
	if !offersStructuralSearch(definitions) {
		return definitions
	}
	kept := make([]types.ToolDefinition, 0, len(definitions))
	for _, def := range definitions {
		if !tools.IsRawSearch(def.Name) {
			kept = append(kept, def)
		}
	}
	return kept
}

func offersStructuralSearch(definitions []types.ToolDefinition) bool {
	for _, def := range definitions {
		if def.Name == "find_symbol" {
			return true
		}
	}
	return false
}

// structuralFirstText answers a call to a withheld search tool.
const structuralFirstText = "Raw search (grep, glob, list_files, search_code) is not offered yet on this task. " +
	"The workspace is already parsed: find_symbol locates a declaration by name, find_text finds text in string literals, comments or identifiers and answers with the element holding each hit, " +
	"package_outline lists what a directory or file declares with line spans, importers_of lists who imports a package, " +
	"callers_of and callees_of follow the call graph, unreferenced_symbols lists what nothing uses, and get_element returns a declaration's source with its doc comment, line-numbered, and the rev the edit verbs take. " +
	"read_file still reads any file, including ones that are not Go. Raw search opens once these have been tried and cannot answer."

// searchOpenedText tells the model the policy has opened the raw search tools.
const searchOpenedText = "Raw search (grep, glob, list_files, search_code) is now offered as well. Prefer the structural queries where they answer; search text where they cannot."

// commitRegimeDefinitions is the catalog offered under the commit regime.
func commitRegimeDefinitions(definitions []types.ToolDefinition) []types.ToolDefinition {
	kept := make([]types.ToolDefinition, 0, len(definitions))
	for _, def := range definitions {
		if !closedForReading(def.Name) {
			kept = append(kept, def)
		}
	}
	return kept
}

// workingReplyReserve is the part of the input window kept free of working
// context so the request is never sent at exactly the budget.
const workingReplyReserve = 256

func activeWorkingLoop(ctx context.Context) *workingLoop {
	value, _ := ctx.Value(workingLoopKey{}).(*workingLoop)
	return value
}

// workingLoopWorkspace is the workspace a working set persists this turn's
// observations into. It is the *declared* root only, never the one
// workspaceForVerification discovers from the process's working directory:
// discovery is the right answer to "which tree do I compile" and the wrong
// one to "where do I write durable state", which must be the workspace the
// caller named. Every production boot declares one (internal/system/factory.go
// resolves it before SetConfig), so every production tool-loop path — chat and
// shard alike — has a working set and therefore a policy over it.
func (e *Executor) workingLoopWorkspace() string {
	root := strings.TrimSpace(e.configSnapshot().WorkspaceRoot)
	if root == "" {
		return ""
	}
	canonical, err := tools.CanonicalWorkspaceRoot(root)
	if err != nil {
		return ""
	}
	return canonical
}

// workingLoopAvailable reports whether beginWorkingLoop will install a working
// set for this turn. A declared workspace root is the whole requirement: the
// working policy reads no world, so a turn without one still gets its stops.
// Callers that must decide before beginWorkingLoop runs ask here instead of
// re-deriving the condition and drifting from it.
func (e *Executor) workingLoopAvailable() bool {
	return e.workingLoopWorkspace() != ""
}

func (e *Executor) beginWorkingLoop(ctx context.Context, input string, cc *prompt.CompilationContext) (context.Context, func(), error) {
	e.mu.Lock()
	world, sessionID := e.workingWorld, e.sessionID
	if e.workingScope == "" {
		e.workingScope = fmt.Sprintf("%x", rand.Text())
	}
	scopeID := e.workingScope
	e.mu.Unlock()
	root := e.workingLoopWorkspace()
	if root == "" {
		if world == nil {
			// No declared workspace and no world: nothing to build a working
			// set on. runToolLoopPass refuses rather than running a tool loop
			// with no policy over it.
			return ctx, func() {}, nil
		}
		// A world with no workspace is a misconfiguration, not a degraded mode.
		return ctx, func() {}, fmt.Errorf("working context requires a workspace")
	}
	// A nil world costs the dependency hops in Select and nothing else:
	// Continue — the policy call, and every stop the loop can derive — reads
	// no world at all. Refusing the working set here left those turns running
	// a tool loop with no policy over it, which is a forcing decision made by
	// a Go nil check.
	// A turn can reach the loop with no compilation context (the Piggyback
	// and forced-final paths build one later, or not at all). Its shard and
	// intent target only name the scope and the focus, so their absence costs
	// a narrower working set — never the policy.
	shardID, intentTarget := "", ""
	if cc != nil {
		shardID, intentTarget = cc.ShardID, cc.IntentTarget
	}
	scope := sessionID + "/" + shardID + "/" + scopeID
	set, err := working.NewWorkingSet(world, root, scope)
	if err != nil {
		return ctx, func() {}, err
	}
	focus := normalizeWorkingEntity(intentTarget, root)
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
	// A recall brings an archived observation back; it is not a new one.
	// Saving its result minted a copy under the focus -- whatever file was
	// touched last -- at that file's revision, so recalled evidence went stale
	// when the wrong file changed and stayed current when its own file did.
	// The recalled observation becomes recent again and the call maps to it:
	// the transcript carries the page while the round is kept, the section
	// carries the record after, under its own file and revision.
	//
	// The focus follows it to the record's file (N21): what the model recalls
	// is what it is working on, and the focus decides whose context -- the
	// file's outline and current line ranges -- every request renders. Under
	// the commit regime a recall is the only way left to look at code, and a
	// focus that moved only on reads froze on the last file read before
	// reading closed. R1-9 (2026-09-19): eleven recalls of build_verify.go,
	// repair_loop.go and their neighbours while every request rendered
	// working_meter.go, then a read-only stall with no edit made.
	if call.Name == "recall_context" && toolErr == nil {
		if id, _ := call.Input["id"].(string); id != "" {
			loop.remember(call.ID, id)
			if entity, err := loop.set.Entity(ctx, id); err == nil && entity != "" {
				loop.focus = entity
			}
			return nil
		}
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
		// The focus is what this run is looking at, so it is what the CodeDOM
		// fact layer holds: the elements of the file the turn just touched,
		// re-parsed whenever an edit changed it (codedom_scope.go).
		e.scopeFocusFile(loop.focus)
	}
	// args feeds a hash that becomes Kind, Kind is persisted on the record and
	// is what working_set.mg selects observations by. A discarded marshal error
	// would not fail here: nil args makes every call of one tool hash
	// identically regardless of its arguments, silently collapsing distinct
	// observations into one kind. ToolCall.Input is only built by
	// json.Unmarshal today (client_tool_helpers.go, xaioauth/tools.go), so this
	// should not fire from that producer -- but a marshal failure must neither
	// collide with a real input nor vanish: fold the error into the hashed
	// material instead.
	args, marshalErr := json.Marshal(call.Input)
	if marshalErr != nil {
		args = []byte("marshal-error:\x00" + marshalErr.Error())
	}
	sum := sha256.Sum256(append([]byte(call.Name+"\x00"), args...))
	kind := call.Name + "/" + hex.EncodeToString(sum[:8])
	if toolErr != nil {
		body += "\nERROR: " + toolErr.Error()
	}
	revision := loop.set.Revision(loop.focus)
	step := time.Now().UnixNano()
	idSum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", call.ID, kind, body, revision, step)))
	id := hex.EncodeToString(idSum[:])
	start, end := observedSpan(call)
	record := working.WorkingRecord{ID: id, Entity: loop.focus, Revision: revision, Kind: kind, Step: step, Body: body, Start: start, End: end, Failed: toolErr != nil}
	if err := loop.set.Save(ctx, record); err != nil {
		return fmt.Errorf("persist working observation: %w", err)
	}
	loop.remember(call.ID, id)
	return nil
}

// remember maps a tool call to the observation it produced or recalled and
// makes that observation the most recent of the last sixteen.
func (loop *workingLoop) remember(callID, id string) {
	loop.recent = append(loop.recent, id)
	loop.observations[callID] = id
	if len(loop.recent) > 16 {
		loop.recent = loop.recent[len(loop.recent)-16:]
	}
}

// wholeFileSpanEnd stands for "to the end of the file" in an observation's
// span: a content read with no end line covers everything after its start.
const wholeFileSpanEnd = 1_000_000_000

// observedSpan is the line span a content read covered, so a later read that
// covers it can replace it in the working section. Only read_file records a
// span: an outline or a search is not a view of the lines, and must not stand
// in for one.
func observedSpan(call types.ToolCall) (int64, int64) {
	if !strings.EqualFold(strings.TrimSpace(call.Name), "read_file") {
		return 0, 0
	}
	start, end := int64(1), int64(wholeFileSpanEnd)
	if v, ok := tools.CoerceInt(call.Input["start_line"]); ok && v > 0 {
		start = int64(v)
	}
	if v, ok := tools.CoerceInt(call.Input["end_line"]); ok && v > 0 {
		end = int64(v)
	}
	if end < start {
		end = start
	}
	return start, end
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
//
// The request is ordered stable to volatile, because a provider prefix cache
// covers it only up to its first changed byte: tool catalog, the compiled
// system prompt, the prior turns, the anchor, a transcript that is append-only
// between cuts (working_transcript_slack), and last the section, which follows
// the model's attention and so changes on most rounds. Until 2026-09-21 the
// section was appended to the system prompt, ahead of every message: measured
// that day, 767 of 854 follow-up calls changed the cacheable prefix and the
// anchor, the prior turns and the transcript were billed uncached on each. It
// also put file contents under system authority, where text read from the
// workspace has no business being.
func (e *Executor) prepareWorkingRequest(ctx context.Context, system string, history []types.Message, definitions []types.ToolDefinition) ([]types.Message, error) {
	messages, section, err := e.workingRequestParts(ctx, system, history, definitions)
	if err != nil {
		return nil, err
	}
	return withWorkingSection(messages, section), nil
}

// workingSectionHeader opens the section where it rides on a user turn, so the
// model reads it as the harness's evidence and not as something the user said.
const workingSectionHeader = "[working context -- selected by the harness for this round and regenerated every round: current code views and earlier observations. Evidence for the task above, not a new request.]\n"

// withWorkingSection puts the section at the end of the request's last user
// turn, after that turn's tool results.
func withWorkingSection(messages []types.Message, section string) []types.Message {
	if section == "" {
		return messages
	}
	text := workingSectionHeader + section
	if n := len(messages); n > 0 && messages[n-1].Role == "user" {
		out := append([]types.Message(nil), messages...)
		out[n-1] = out[n-1].WithTrailingText(text)
		return out
	}
	return append(append([]types.Message(nil), messages...), types.Message{Role: "user", Text: text})
}

// workingRequestParts computes the two halves of a working request: the
// messages, and the section the working policy selected for this round.
func (e *Executor) workingRequestParts(ctx context.Context, system string, history []types.Message, definitions []types.ToolDefinition) ([]types.Message, string, error) {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return history, "", nil
	}
	// Keep the last rounds of native call/result pairs intact, as many as the
	// policy says. Older observations live in the selected state rather than
	// an ever-growing provider transcript. Only the current pair was kept
	// before, and a model that saw no turn of its own before this one started
	// every round from scratch (see working_transcript_rounds in the policy).
	rounds, err := loop.set.TranscriptRounds(ctx)
	if err != nil {
		return nil, "", err
	}
	slack, err := loop.set.TranscriptSlack(ctx)
	if err != nil {
		return nil, "", err
	}
	start := transcriptStart(history, rounds, slack)
	messages := append([]types.Message(nil), loop.prior...)
	messages = append(messages, types.Message{Role: "user", Text: loop.anchor})
	var shown []string
	if start < len(history) {
		for _, message := range history[start:] {
			// WithToolResults, not a field assignment: a block-built turn
			// sends its blocks, so a copy made on the flat field alone would
			// share (and let the archive below miss) the payload.
			copyMessage := message.WithToolResults(append([]types.ToolResult(nil), message.ToolResults...))
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
			return nil, "", err
		}
		catalog = prompt.EstimateTokens(string(encoded))
	}
	remaining, err := workingWindowRemaining(window, system, messages, catalog)
	if err != nil {
		return nil, "", err
	}
	for remaining < workingReplyReserve+512 {
		i, j, size := largestToolResult(messages)
		if size == 0 {
			return nil, "", fmt.Errorf("working request exceeds configured input budget; required instructions cannot be discarded")
		}
		results := append([]types.ToolResult(nil), messages[i].ToolResults...)
		result := &results[j]
		id := loop.observations[result.ToolUseID]
		if id == "" {
			return nil, "", fmt.Errorf("oversize tool result has no durable observation")
		}
		// The pipeline's marker rides with the pointer. This path already told
		// the model the size and the handle, which is most of what a marker is
		// for, but it said it in words of its own — so an audit that walks the
		// assembled messages looking for unannounced cuts could not tell this
		// from a result that was simply short. One prefix, every cutter.
		result.Content = fmt.Sprintf("%s %s recall_context id=%q returns it from offset 0, or in offset/limit pages. Historical evidence requires a current revision check.",
			archivedResultPrefix,
			types.DroppedNotice(size, size, "chars of this tool result; it does not fit the request", ""),
			id)
		// Both views: assigning Content on the flat projection alone would leave
		// a block-built turn sending the payload this archive accounted as gone.
		messages[i] = messages[i].WithToolResults(results)
		if remaining, err = workingWindowRemaining(window, system, messages, catalog); err != nil {
			return nil, "", err
		}
	}
	// The section's ceiling is the working policy's (working_section_ceiling):
	// the working context exists so the request stops growing with every tool
	// result, and a section allowed to fill whatever the window leaves would put
	// that growth back on every round. Within it the policy chooses what is
	// shown; what it leaves out stays recallable.
	ceiling, err := loop.set.SectionCeiling(ctx)
	if err != nil {
		return nil, "", err
	}
	budget := min((remaining-workingReplyReserve)*4, ceiling)
	// The focused file's context is rendered first and its room reserved: it is
	// the one copy the request carries (compile-time prompts leave it out when a
	// working loop runs, withCompiledFileContext), and it is regenerated here
	// every call so its outline's line ranges follow the edits. Observations
	// fill what is left; before, the view was added only if it still fit after
	// them, so a full section could drop the one thing every round needs.
	view := e.withFileContext(ctx, "", loop.focus)
	if len(view) > budget {
		view = ""
	}
	selected, err := loop.set.Select(ctx, loop.focus, loop.recent, shown, budget-len(view))
	if err != nil {
		return nil, "", err
	}
	// The active state is regenerated each call; current code views and
	// observations stay adjacent to the current request.
	section := selected.Text
	if view != "" {
		section = view + "\n" + section
	}
	return messages, section, nil
}

// transcriptStart is the index in history of the oldest round the transcript
// keeps. The window holds between rounds and rounds+slack rounds: it grows by
// appending until it is slack over, then is cut back to rounds in one step, so
// its first message -- where a prefix cache would break -- moves once in every
// slack+1 rounds. The cut depends only on how many rounds there are, so the
// same history always yields the same window.
func transcriptStart(history []types.Message, rounds, slack int) int {
	var starts []int
	for i, message := range history {
		if message.Role == "assistant" && len(message.ToolCalls) > 0 {
			starts = append(starts, i)
		}
	}
	total := len(starts)
	if total == 0 {
		return len(history)
	}
	if total <= rounds {
		return starts[0]
	}
	keep := rounds + (total-rounds)%(slack+1)
	return starts[total-keep]
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
	if loop := activeWorkingLoop(ctx); loop != nil {
		if loop.regime == commitRegime {
			definitions = commitRegimeDefinitions(definitions)
		}
		if !loop.searchOpen {
			definitions = structuralFirstDefinitions(definitions)
		}
	}
	history, err := e.prepareWorkingRequest(ctx, system, history, definitions)
	if err != nil {
		return nil, fmt.Errorf("compile working context: %w", err)
	}
	return provider.CompleteWithToolResults(ctx, system, history, definitions)
}
