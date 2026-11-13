package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The working-request guard (fitWorkingRequest) sheds, in this order, until a
// request fits its input window, and refuses only when nothing is left to shed:
//
//  1. tool results, largest first, each left as a recall pointer;
//  2. superseded view snapshots: a focus-view snapshot with a later snapshot of
//     the same path in the request becomes a one-line pointer to that later one;
//  3. replayed reasoning, oldest assistant turn first, never the latest turn's.
//
// Tool results can be recalled; snapshots are regenerated every time the view
// changes; reasoning is the provider's encrypted state and cannot come back,
// which is why it goes last and why its marker says so. Every shed piece leaves
// a visible harness marker in the request and is counted in an INFO log line.
//
// Steps 2 and 3 are sticky: the loop records them, so the next request sends
// what this one sent byte for byte (the provider's prefix cache breaks once, at
// the shed, instead of every round) and the next assemble cannot resurrect a
// collapsed snapshot or a dropped reasoning block. Step 1 is not, as it never
// was: a result is archived only on the rounds the request cannot hold it.

// viewSnapshot is one focus-view snapshot the ledger appended to a message of
// the request. text is what loop.appended[key] holds for it: the snapshot, or
// its pointer once collapsed.
type viewSnapshot struct {
	key       string
	path      string
	text      string
	collapsed bool
}

// shedReasoning is the replayed reasoning the guard dropped from one message of
// the request.
type shedReasoning struct {
	blocks int
	tokens int
}

// priorLedgerKey keys the harness's state on prior turn i of the request.
func priorLedgerKey(i int) string { return fmt.Sprintf("p%d", i) }

// supersededSnapshotPointer replaces a view snapshot a later one of the same
// path supersedes. The marker is the pipeline's, so an audit of the assembled
// request finds it with every other cut.
func supersededSnapshotPointer(path string, size int) string {
	return fmt.Sprintf("[harness: the view snapshot of %s that stood here is superseded by a later snapshot of %s in this request.] %s",
		path, path,
		types.DroppedNotice(size, size, "chars of this view snapshot; the later snapshot of the same path replaces it", ""))
}

// recordSnapshot notes a view snapshot just appended to the message at key.
func (loop *workingLoop) recordSnapshot(key, path, text string) {
	loop.snapshots = append(loop.snapshots, viewSnapshot{key: key, path: path, text: text})
}

// collapseSupersededSnapshots turns every snapshot that a later snapshot of the
// same path supersedes into its pointer, in loop.appended where it rides, and
// returns how many it collapsed and the characters that freed. The latest
// snapshot of each path is never touched.
func (loop *workingLoop) collapseSupersededSnapshots() (collapsed, freed int) {
	return loop.collapseSuperseded(func(string) bool { return true })
}

// collapseSupersededSnapshotsAt collapses, the same way, only the superseded
// snapshots riding the message at key: a retired repair payload's turn.
func (loop *workingLoop) collapseSupersededSnapshotsAt(key string) (collapsed, freed int) {
	return loop.collapseSuperseded(func(k string) bool { return k == key })
}

func (loop *workingLoop) collapseSuperseded(at func(key string) bool) (collapsed, freed int) {
	latest := make(map[string]int, len(loop.snapshots))
	for i, s := range loop.snapshots {
		latest[s.path] = i
	}
	for i := range loop.snapshots {
		s := &loop.snapshots[i]
		if s.collapsed || latest[s.path] == i || !at(s.key) {
			continue
		}
		riding := loop.appended[s.key]
		if !strings.Contains(riding, s.text) {
			continue
		}
		pointer := supersededSnapshotPointer(s.path, len(s.text))
		loop.appended[s.key] = strings.Replace(riding, s.text, pointer, 1)
		freed += len(s.text) - len(pointer)
		s.text, s.collapsed = pointer, true
		collapsed++
	}
	return collapsed, freed
}

// withoutShedReasoning applies the loop's shed reasoning to an assembled
// request: the messages the guard dropped reasoning from lose their thinking
// blocks, and the message right after the newest of them carries the marker.
// Shedding runs oldest first, so the shed turns are a prefix of the request and
// the marker's place is stable until the guard sheds more.
func (loop *workingLoop) withoutShedReasoning(messages []types.Message, keys []string) []types.Message {
	if len(loop.reasoningShed) == 0 {
		return messages
	}
	last := -1
	blocks, tokens := 0, 0
	for i, key := range keys {
		rec, ok := loop.reasoningShed[key]
		if !ok {
			continue
		}
		stripped, _ := messages[i].WithoutThinking()
		messages[i] = stripped
		blocks += rec.blocks
		tokens += rec.tokens
		last = i
	}
	if last < 0 || last+1 >= len(messages) {
		return messages
	}
	kept := 0
	for _, m := range messages {
		for _, b := range m.Content() {
			if b.Kind == types.BlockThinking {
				kept++
			}
		}
	}
	notice := fmt.Sprintf("[harness: the replayed reasoning of the %d earlier assistant turn(s) above, about %d tokens, was dropped to fit the input window. It was the provider's encrypted reasoning and cannot be recalled; those turns' text, tool calls and results are unchanged.] %s",
		len(loop.reasoningShed), tokens,
		types.DroppedNotice(blocks, blocks+kept, "reasoning blocks of earlier assistant turns; the latest turn's reasoning is kept", ""))
	messages[last+1] = messages[last+1].WithTrailingText(notice)
	return messages
}

// shedOldestReasoning drops the replayed reasoning of the oldest assistant turn
// that still carries any, except the latest assistant turn of the request:
// Anthropic requires the last tool-use turn's thinking blocks and a Responses
// replay needs the latest round's reasoning item. It records the drop in the
// loop and reports what it freed; ok is false when nothing is left to drop.
func (loop *workingLoop) shedOldestReasoning(messages []types.Message, keys []string) (blocks, tokens int, ok bool) {
	latest := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			latest = i
			break
		}
	}
	for i := 0; i < latest; i++ {
		if messages[i].Role != "assistant" {
			continue
		}
		_, n := messages[i].WithoutThinking()
		if n == 0 {
			continue
		}
		freed := 0
		for _, b := range messages[i].Content() {
			freed += prompt.ReasoningTokens(b)
		}
		if freed == 0 {
			// Unsigned reasoning is not replayed: dropping it frees nothing.
			continue
		}
		if loop.reasoningShed == nil {
			loop.reasoningShed = map[string]shedReasoning{}
		}
		loop.reasoningShed[keys[i]] = shedReasoning{blocks: n, tokens: freed}
		return n, freed, true
	}
	return 0, 0, false
}

// shedReport is what one round's guard shed, for its log line.
type shedReport struct {
	results, resultChars       int
	snapshots, snapshotChars   int
	reasoningTurns, reasoningN int
	reasoningTokens            int
}

func (r shedReport) any() bool {
	return r.results > 0 || r.snapshots > 0 || r.reasoningTurns > 0
}

// fitWorkingRequest assembles the request as the ledger stands and sheds until
// it fits the window (see the order above), or refuses with the breakdown of
// what is left.
func (e *Executor) fitWorkingRequest(ctx context.Context, loop *workingLoop, system string, history []types.Message, start int, definitions []types.ToolDefinition, window inputBudget) ([]types.Message, error) {
	messages, keys, err := loop.assemble(ctx, history, start)
	if err != nil {
		return nil, err
	}
	remaining, err := workingWindowRemaining(window.tokens, system, messages, definitions)
	if err != nil {
		return nil, err
	}
	needed := window.tokens - remaining
	var report shedReport
	archived := map[[2]int]string{}
	// rebuild reassembles after a shed that changed the loop's state, and
	// re-applies the tool-result archives of this round.
	rebuild := func() error {
		if messages, keys, err = loop.assemble(ctx, history, start); err != nil {
			return err
		}
		applyArchived(messages, archived)
		remaining, err = workingWindowRemaining(window.tokens, system, messages, definitions)
		return err
	}

	// 1. Tool results, largest first.
	for remaining < workingInputReserve {
		i, j, size := largestToolResult(messages)
		if size == 0 {
			break
		}
		results := append([]types.ToolResult(nil), messages[i].ToolResults...)
		result := &results[j]
		storageID := loop.observations[result.ToolUseID]
		if storageID == "" {
			return nil, fmt.Errorf("oversize tool result has no durable observation")
		}
		handle, err := loop.modelHandle(ctx, storageID)
		if err != nil {
			return nil, err
		}
		// The pipeline's marker rides with the pointer. This path already told
		// the model the size and the handle, which is most of what a marker is
		// for, but it said it in words of its own — so an audit that walks the
		// assembled messages looking for unannounced cuts could not tell this
		// from a result that was simply short. One prefix, every cutter.
		result.Content = fmt.Sprintf("%s %s recall_context id=%q returns it from offset 0, or in offset/limit pages. Historical evidence requires a current revision check.",
			archivedResultPrefix,
			types.DroppedNotice(size, size, "chars of this tool result; it does not fit the request", ""),
			handle)
		archived[[2]int{i, j}] = result.Content
		// Both views: assigning Content on the flat projection alone would leave
		// a block-built turn sending the payload this archive accounted as gone.
		messages[i] = messages[i].WithToolResults(results)
		report.results++
		report.resultChars += size
		if remaining, err = workingWindowRemaining(window.tokens, system, messages, definitions); err != nil {
			return nil, err
		}
	}

	// 2. View snapshots a later snapshot of the same path supersedes.
	if remaining < workingInputReserve {
		if n, freed := loop.collapseSupersededSnapshots(); n > 0 {
			report.snapshots, report.snapshotChars = n, freed
			if err := rebuild(); err != nil {
				return nil, err
			}
		}
	}

	// 3. Replayed reasoning, oldest assistant turn first; the latest turn's stays.
	for remaining < workingInputReserve {
		blocks, tokens, ok := loop.shedOldestReasoning(messages, keys)
		if !ok {
			break
		}
		report.reasoningTurns++
		report.reasoningN += blocks
		report.reasoningTokens += tokens
		if err := rebuild(); err != nil {
			return nil, err
		}
	}

	if report.any() {
		logging.Context("Working request guard: the request needed about %d tokens against a %d-token budget; shed %d tool result(s) (%d chars) behind recall handles, collapsed %d superseded view snapshot(s) (%d chars), dropped the replayed reasoning of %d earlier assistant turn(s) (%d block(s), about %d tokens, not recallable); %d token(s) free of the %d held back",
			needed, window.tokens, report.results, report.resultChars, report.snapshots, report.snapshotChars,
			report.reasoningTurns, report.reasoningN, report.reasoningTokens, remaining, workingInputReserve)
	}

	// 4. Nothing left to shed.
	if remaining < workingInputReserve {
		return nil, inputBudgetExceeded(window, remaining, loop.requestParts(system, messages, definitions))
	}
	return messages, nil
}

// applyArchived puts this round's archived tool-result pointers, by position,
// into a freshly assembled request.
func applyArchived(messages []types.Message, archived map[[2]int]string) {
	byMessage := map[int][]int{}
	for pos := range archived {
		byMessage[pos[0]] = append(byMessage[pos[0]], pos[1])
	}
	for i, indexes := range byMessage {
		results := append([]types.ToolResult(nil), messages[i].ToolResults...)
		for _, j := range indexes {
			if j < len(results) {
				results[j].Content = archived[[2]int{i, j}]
			}
		}
		messages[i] = messages[i].WithToolResults(results)
	}
}

// requestParts is what a request that does not fit is made of, in the tokens
// EstimateRequestTokens counts, so a refusal can say what is left.
type requestParts struct {
	system, task, catalog, prior int
	reasoning, snapshots         int
	assistant, results           int
}

// String names every part the request has.
func (p requestParts) String() string {
	parts := []string{
		fmt.Sprintf("system prompt %d", p.system),
		fmt.Sprintf("task and anchor %d", p.task),
		fmt.Sprintf("tool catalog %d", p.catalog),
		fmt.Sprintf("prior turns %d", p.prior),
	}
	for _, extra := range []struct {
		name string
		n    int
	}{
		{"reasoning kept", p.reasoning},
		{"view snapshots kept", p.snapshots},
		{"tool calls and assistant text", p.assistant},
		{"tool results and harness notes", p.results},
	} {
		if extra.n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", extra.name, extra.n))
		}
	}
	return strings.Join(parts, ", ")
}

// catalogTokens is the tokens the tool catalog puts in a request.
func catalogTokens(definitions []types.ToolDefinition) int {
	if len(definitions) == 0 {
		return 0
	}
	encoded, err := json.Marshal(definitions)
	if err != nil {
		return 0
	}
	return prompt.EstimateTokens(string(encoded))
}

// messageTokens is the tokens one message puts in a request, replayed
// reasoning included.
func messageTokens(m types.Message) int {
	n, err := prompt.EstimateRequestTokens("", []types.Message{m}, nil)
	if err != nil {
		return 0
	}
	return n
}

// requestParts measures an assembled working request by part. The messages are
// loop.prior, the anchor, then the ledger's turns.
func (loop *workingLoop) requestParts(system string, messages []types.Message, definitions []types.ToolDefinition) requestParts {
	parts := requestParts{system: prompt.EstimateTokens(system), catalog: catalogTokens(definitions)}
	estimate := messageTokens
	anchor := len(loop.prior)
	for i, m := range messages {
		switch {
		case i < anchor:
			parts.prior += estimate(m)
		case i == anchor:
			parts.task += estimate(m)
		case m.Role == "assistant":
			for _, b := range m.Content() {
				parts.reasoning += prompt.ReasoningTokens(b)
			}
			stripped, _ := m.WithoutThinking()
			parts.assistant += estimate(stripped)
		default:
			parts.results += estimate(m)
		}
	}
	for _, s := range loop.snapshots {
		if s.collapsed {
			continue
		}
		n := prompt.EstimateTokens(s.text)
		parts.snapshots += n
		if s.key == anchorLedgerKey {
			parts.task = max(parts.task-n, 0)
		} else {
			parts.results = max(parts.results-n, 0)
		}
	}
	return parts
}
