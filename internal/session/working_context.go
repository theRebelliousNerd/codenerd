package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	working "codenerd/internal/context"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/types"
)

type workingLoopKey struct{}
type workingLoop struct {
	set          *working.WorkingSet
	focus        string
	anchor       string
	prior        []types.Message
	observations map[string]string // tool call ID -> the observation it produced or recalled
	regime       string            // the policy's working_regime for the next round
	// searchOpen is the policy's working_search_open. False at the start of a
	// loop: the raw search tools are withheld until the policy derives it.
	searchOpen bool

	// The context ledger's state, carried from request to request so what a
	// request already carried is sent again byte for byte.
	//
	// evicted names the calls whose results a compaction moved out
	// (working_evict); from then on each renders as its recall handle.
	evicted map[string]bool
	// appended is the harness's text on a message of the request -- the focus
	// file's view, stale-observation notices -- keyed by the message it rides
	// (ledgerKey), written once and resent unchanged.
	appended map[string]string
	// restated maps an observation to the revision it was last restated at
	// (working_restated).
	restated map[string]string
	// viewed is the focus view the ledger carries: focus and its revision.
	viewed string
	// callIDs are the tool-call ids this loop has handed out on the
	// Piggyback channel (claimCallIDs), so no two calls share one.
	callIDs map[string]bool
	// result is the turn this loop runs for, where a round's safety notice
	// is kept once that round's surface is superseded (piggybackChannel).
	result *ExecutionResult
}

// commitRegime is the working_regime under which exploration is closed.
const commitRegime = "commit"

// repairRegime is the regime a repair attempt's rounds run under: open for
// its diagnosis, until the policy closes reading (working_set.mg).
const repairRegime = "repair"

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

// workingInputReserve is the part of the input window a working request
// leaves free: never sent at exactly the budget, and room held back for the
// reply. One rule for the working request and the single-shot request.
const workingInputReserve = 256 + 512

// ErrInputBudgetExceeded is a working request that does not fit the input
// budget once every tool result in it has been archived: what is left -- the
// task, the instructions, the tool catalog -- is never cut, so the request is
// refused before any model call rather than sent truncated.
var ErrInputBudgetExceeded = errors.New("working request exceeds the input budget")

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
	sessionID := e.sessionID
	if e.workingScope == "" {
		e.workingScope = fmt.Sprintf("%x", rand.Text())
	}
	scopeID := e.workingScope
	e.mu.Unlock()
	root := e.workingLoopWorkspace()
	if root == "" {
		// No declared workspace: nothing to build a working set on.
		// runToolLoopPass refuses rather than running a tool loop with no
		// policy over it.
		return ctx, func() {}, nil
	}
	// A turn can reach the loop with no compilation context (the Piggyback
	// and forced-final paths build one later, or not at all). Its shard and
	// intent target only name the scope and the focus, so their absence costs
	// a narrower working set — never the policy.
	shardID, intentTarget := "", ""
	if cc != nil {
		shardID, intentTarget = cc.ShardID, cc.IntentTarget
	}
	scope := sessionID + "/" + shardID + "/" + scopeID
	set, err := working.NewWorkingSet(root, scope, e.configSnapshot().Working)
	if err != nil {
		return ctx, func() {}, err
	}
	focus := normalizeWorkingEntity(intentTarget, root)
	// The loop's window names its eviction handle, and the loop serves what
	// the window evicted behind recall_context for as long as it runs.
	prior, evictedHistory := e.priorTurnWindow(true)
	loop := &workingLoop{
		set: set, focus: focus, anchor: input, prior: prior,
		observations: make(map[string]string),
		evicted:      make(map[string]bool),
		appended:     make(map[string]string),
		restated:     make(map[string]string),
	}
	ctx = context.WithValue(ctx, workingLoopKey{}, loop)
	ctx = tools.WithContextRecall(ctx, historyRecall{
		working: set, handle: historyEvictionHandle(evictedHistory), evicted: evictedHistory,
	})
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
	// The call maps to the recalled observation, under its own file and
	// revision: the ledger carries the page as that observation's, so a
	// compaction and a restatement treat it as the evidence it is.
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
				loop.focus = working.EntityFile(entity)
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
	// An element the call names is what it observed, and its own bytes date
	// the observation (R8-3): editing one function leaves what was read of
	// another in the same file current. The ref is resolved after the call,
	// so an edit's observation is the element as the edit left it; a ref
	// that no longer resolves -- a deleted or renamed element, a failed
	// call -- leaves the observation on its file.
	observed := ""
	if ref, _ := call.Input["ref"].(string); strings.TrimSpace(ref) != "" && toolErr == nil {
		root := e.workspaceForVerification()
		if file, key, err := codedom.ElementAt(tools.WithWorkspaceRoot(ctx, root), ref, entity); err == nil {
			entity = file
			observed = working.ElementEntity(normalizeWorkingEntity(file, root), key)
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
	if observed == "" {
		observed = loop.focus
	}
	revision := loop.set.Revision(observed)
	step := time.Now().UnixNano()
	idSum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d", call.ID, kind, body, revision, step)))
	id := hex.EncodeToString(idSum[:])
	start, end := observedSpan(call)
	record := working.WorkingRecord{ID: id, Entity: observed, Revision: revision, Kind: kind, Step: step, Body: body, Start: start, End: end, Failed: toolErr != nil}
	if err := loop.set.Save(ctx, record); err != nil {
		return fmt.Errorf("persist working observation: %w", err)
	}
	loop.remember(call.ID, id)
	return nil
}

// remember maps a tool call to the observation it produced or recalled.
func (loop *workingLoop) remember(callID, id string) {
	loop.observations[callID] = id
}

// wholeFileSpanEnd stands for "to the end of the file" in an observation's
// span: a content read with no end line covers everything after its start.
const wholeFileSpanEnd = 1_000_000_000

// observedSpan is the line span a content read covered, so a later read that
// covers it supersedes it in the ledger (working_superseded). Only read_file records a
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
// loop: the prior turns, the anchor, and the context ledger -- every native
// call/result round since the loop began, each result whole or, once a
// compaction moved it out, as its recall handle.
//
// The ledger is append-only between compactions. A provider prefix cache covers
// a request up to its first changed byte, and everything a request carried is
// carried again byte for byte: the harness's text -- the focus file's view, a
// stale-observation notice -- is appended once to the round where it arose and
// resent unchanged, never regenerated. Until 2026-09-23 a request carried a
// sliding window of rounds and, at its tail, a section of selected
// observations regenerated every round; measured on 481 rounds (2026-09-22),
// 81% of each section was the round before's observations, each observation
// was sent 5.6 times, and none of the section was ever served from cache (22.3k
// of 55.4k input tokens a round uncached).
//
// What leaves the ledger is the policy's: past its ceiling (working_compact)
// one compaction moves out every result older than the kept rounds and every
// stale or superseded one (working_evict), so the cache breaks once an epoch
// instead of once a round. An observation the ledger still carries whose file
// changed is restated, not rewritten (working_restate).
//
// The current result is sent whole. Observed 2026-09-11: a 14 KB read of the
// file the brief named was swapped for an "archived, recall it" pointer on a
// fixed 8000-character rule, and the model spent 24 rounds reading the file,
// recalling a 2000-character page of it and reading it again, without ever
// reaching the line it was asked to change. A result is only archived out of
// the ledger by the budget guard below when the request cannot otherwise fit
// the input window, largest first, and the pointer then says how large the
// body is.
//
// The catalog of tool definitions is part of the request, so its cost comes
// off the window like the system prompt and the transcript do. The compiled
// system prompt reaches the provider as compiled: file contents have no
// business under system authority, and a section written into it moved the
// cacheable prefix on every round (767 of 854 follow-up calls, 2026-09-21).
func (e *Executor) prepareWorkingRequest(ctx context.Context, system string, history []types.Message, definitions []types.ToolDefinition) ([]types.Message, error) {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return history, nil
	}
	start := ledgerStart(history)
	if err := e.updateLedger(ctx, loop, history, start); err != nil {
		return nil, err
	}

	messages := append([]types.Message(nil), loop.prior...)
	messages = append(messages, types.Message{Role: "user", Text: loop.anchor}.WithTrailingText(loop.appended[anchorLedgerKey]))
	for i := start; i < len(history); i++ {
		// WithToolResults, not a field assignment: a block-built turn sends
		// its blocks, so a copy made on the flat field alone would share (and
		// let the archive below miss) the payload.
		results := append([]types.ToolResult(nil), history[i].ToolResults...)
		for j := range results {
			if loop.evicted[results[j].ToolUseID] {
				results[j].Content = compactedResultHandle(results[j].Content, loop.observations[results[j].ToolUseID])
			}
		}
		messages = append(messages, history[i].WithToolResults(results).WithTrailingText(loop.appended[ledgerKey(i)]))
	}

	window, catalog, err := e.workingWindow(definitions)
	if err != nil {
		return nil, err
	}
	remaining, err := workingWindowRemaining(window, system, messages, catalog)
	if err != nil {
		return nil, err
	}
	for remaining < workingInputReserve {
		i, j, size := largestToolResult(messages)
		if size == 0 {
			return nil, inputBudgetExceeded(window, remaining)
		}
		results := append([]types.ToolResult(nil), messages[i].ToolResults...)
		result := &results[j]
		id := loop.observations[result.ToolUseID]
		if id == "" {
			return nil, fmt.Errorf("oversize tool result has no durable observation")
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
			return nil, err
		}
	}
	return messages, nil
}

// workingWindow is the input budget a working request is measured against,
// in tokens, and the tool catalog's share of it.
func (e *Executor) workingWindow(definitions []types.ToolDefinition) (window, catalog int, err error) {
	window = e.configSnapshot().TokenBudget
	if window <= 0 {
		window = DefaultTokenBudget()
	}
	if len(definitions) > 0 {
		encoded, err := json.Marshal(definitions)
		if err != nil {
			return 0, 0, err
		}
		catalog = prompt.EstimateTokens(string(encoded))
	}
	return window, catalog, nil
}

// inputBudgetExceeded refuses a request that does not fit its window with
// nothing left to archive.
func inputBudgetExceeded(window, remaining int) error {
	return fmt.Errorf("%w: the request needs about %d tokens and the budget is %d (half of context_window.max_tokens after its output reserves), with %d held back for the reply; no tool result is left to archive, and the task, the instructions and the tool catalog are sent whole or not at all",
		ErrInputBudgetExceeded, window-remaining, window, workingInputReserve)
}

// singleShotRequest is the one call a client with no message channel gets
// inside a working loop: the prior turns and the task, with the focus file's
// view riding the user input -- not the system prompt, where file contents
// have no business. It has no tool result to archive, so a request that does
// not fit the window is refused whole, as a working request is.
func (e *Executor) singleShotRequest(ctx context.Context, system, userInput string, definitions []types.ToolDefinition) (string, error) {
	loop := activeWorkingLoop(ctx)
	if loop == nil {
		return userInput, nil
	}
	if view := e.workingFocusView(ctx, loop); view != "" {
		userInput += "\n\n" + view
	}
	window, catalog, err := e.workingWindow(definitions)
	if err != nil {
		return "", err
	}
	messages := append(append([]types.Message(nil), loop.prior...), types.Message{Role: "user", Text: userInput})
	remaining, err := workingWindowRemaining(window, system, messages, catalog)
	if err != nil {
		return "", err
	}
	if remaining < workingInputReserve {
		return "", inputBudgetExceeded(window, remaining)
	}
	return userInput, nil
}

// anchorLedgerKey keys the harness's text on the anchor: what the first
// request, before any tool round, carries.
const anchorLedgerKey = "anchor"

// ledgerKey keys the harness's text on the message at history index i. The
// loop's history only grows, so an index names the same message on every
// request of the loop.
func ledgerKey(i int) string { return fmt.Sprintf("m%d", i) }

// ledgerStart is the index in history of the first tool round. What precedes
// it -- the prior turns and the user's input -- the request carries as
// loop.prior and the anchor.
func ledgerStart(history []types.Message) int {
	for i, message := range history {
		if message.Role == "assistant" && len(message.ToolCalls) > 0 {
			return i
		}
	}
	return len(history)
}

// updateLedger asks the working policy what this request does with the
// ledger, applies a compaction, and appends this round's harness text -- the
// focus view when the focus or its file changed, a notice for every carried
// observation whose file changed -- to the request's last user turn. Go
// measures (sizes, rounds, revisions) and renders; the policy decides.
func (e *Executor) updateLedger(ctx context.Context, loop *workingLoop, history []types.Message, start int) error {
	round := 0
	var entries []working.LedgerEntry
	for i := start; i < len(history); i++ {
		if history[i].Role == "assistant" && len(history[i].ToolCalls) > 0 {
			round++
		}
		extra := len(loop.appended[ledgerKey(i)])
		for _, r := range history[i].ToolResults {
			// A result with no observation has no recall handle, so it cannot
			// leave the ledger; one already moved out is not counted again.
			id := loop.observations[r.ToolUseID]
			if id == "" || loop.evicted[r.ToolUseID] {
				continue
			}
			entries = append(entries, working.LedgerEntry{Call: r.ToolUseID, ID: id, Bytes: len(r.Content) + extra, Round: round})
			extra = 0
		}
	}
	decision, err := loop.set.Ledger(ctx, entries, round, loop.restated)
	if err != nil {
		return fmt.Errorf("working ledger: %w", err)
	}
	if len(decision.Evict) > 0 {
		// A compaction starts an epoch. Every stale observation left with it,
		// so no restatement is owed; the harness text on earlier rounds goes
		// too, and the focus view is appended again to this round.
		for _, call := range decision.Evict {
			loop.evicted[call] = true
		}
		loop.appended = make(map[string]string)
		loop.restated = make(map[string]string)
		loop.viewed = ""
		logging.Context("Working ledger: compaction at round %d moved %d result(s) out behind their recall handles", round, len(decision.Evict))
	}

	// The harness's text rides the request's last user turn; a request that
	// ends on another turn carries none this round, and what was owed is
	// appended to the next.
	key := anchorLedgerKey
	if n := len(history); n > start {
		if history[n-1].Role != "user" {
			return nil
		}
		key = ledgerKey(n - 1)
	}
	var notes []string
	if revision := loop.set.Revision(loop.focus); loop.focus+"@"+revision != loop.viewed {
		if view := e.workingFocusView(ctx, loop); view != "" {
			notes = append(notes, view)
		}
		loop.viewed = loop.focus + "@" + revision
	}
	if len(decision.Restate) > 0 {
		records, err := loop.set.Observations(ctx, decision.Restate)
		if err != nil {
			return fmt.Errorf("working ledger: %w", err)
		}
		sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
		for _, r := range records {
			tool, _, _ := strings.Cut(r.Kind, "/")
			notes = append(notes, fmt.Sprintf(staleObservationNotice, r.ID, tool, r.Entity, r.Entity))
			loop.restated[r.ID] = loop.set.Revision(r.Entity)
		}
	}
	if len(notes) > 0 {
		text := strings.Join(notes, "\n\n")
		if prior := loop.appended[key]; prior != "" {
			text = prior + "\n\n" + text
		}
		loop.appended[key] = text
	}
	return nil
}

// workingFocusView is the focus file's current view (outline with line ranges,
// importers, callers), under a header that tells the model it is the
// harness's evidence and not something the user said; "" when there is none.
func (e *Executor) workingFocusView(ctx context.Context, loop *workingLoop) string {
	if loop == nil {
		return ""
	}
	view := strings.TrimSpace(e.withFileContext(ctx, "", loop.focus))
	if view == "" {
		return ""
	}
	return fmt.Sprintf(workingViewHeader, loop.focus) + view
}

// workingViewHeader opens the focus view where it rides a user turn.
const workingViewHeader = "[harness: the current view of %s, appended when the focus or the file changes. Evidence for the task above, not a new request.]\n"

// staleObservationNotice restates a carried observation whose file changed
// after it was made: observation id, the tool that made it, the file.
const staleObservationNotice = "[harness: observation %s (%s of %s) predates the current content of %s, which changed after it was made. It is history, not the file as it stands; read again what you still need from it.]"

// compactedResultHandle is what a result a compaction moved out of the ledger
// reads as on every later request: its size and its recall handle, in the
// pipeline's marker, byte-identical from one request to the next.
func compactedResultHandle(content, id string) string {
	return fmt.Sprintf("%s %s recall_context id=%q returns it from offset 0, or in offset/limit pages.",
		archivedResultPrefix,
		types.DroppedNotice(len(content), len(content), "chars of this tool result; the context ledger's compaction moved it out of the request", ""),
		id)
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
