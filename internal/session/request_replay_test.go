package session

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The request replay measures the working-request builder against a recorded
// session instead of estimating it. A coder run records every request it sent
// (.nerd/sessions/<session>/coder-*/NNNN-*.json: the wire body and the
// provider's usage). The replay rebuilds the run's history from those
// recordings -- every tool result whole, from the first request that carried
// it before any later cut -- and sends it through prepareWorkingRequest,
// request by request, as the executor does. Both sides are measured the same
// way: a request is a sequence of canonical items, the provider can serve the
// prefix it shares with the previous request from cache, and tokens and price
// are fitted to what the provider actually billed for the recorded requests.
//
// Run it on a recording:
//
//	CODENERD_REPLAY_DIR=.nerd/sessions/<session>/coder-<id> \
//	  go test -tags sqlite_vec -run TestReplayRecordedSession -v ./internal/session/
//
// The input window is the largest the recorded requests reached (the budget
// guard kept them under it); the ledger knobs come from the recording
// workspace's .nerd/config.json, with CODENERD_REPLAY_LOW_WATER overriding the
// low-water percent.

// replayItem is one recorded or rebuilt request item, reduced to what a
// provider's prefix cache compares.
type replayItem struct {
	kind    string // U user text, A assistant text, Z reasoning, C call, R result
	callID  string
	name    string
	args    map[string]any
	text    string
	isError bool
}

func (it replayItem) canonical() string {
	switch it.kind {
	case "C":
		args, _ := json.Marshal(it.args)
		return "C\x00" + it.callID + "\x00" + it.name + "\x00" + string(args)
	case "R":
		return "R\x00" + it.callID + "\x00" + it.text
	default:
		return it.kind + "\x00" + it.text
	}
}

// replayRequest is one recorded request.
type replayRequest struct {
	file      string
	system    string
	items     []replayItem
	input     int // billed input tokens
	cached    int // billed cache-read tokens
	output    int // billed output (and thinking) tokens
	usd       float64
	estimated int // the harness's own token estimate of the request
}

// joinLines restores a recorded text: long text is stored as an array of its
// lines.
func joinLines(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		parts := make([]string, len(x))
		for i, p := range x {
			parts[i] = fmt.Sprint(p)
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func contentText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	blocks, _ := v.([]any)
	var parts []string
	for _, b := range blocks {
		if m, ok := b.(map[string]any); ok {
			parts = append(parts, joinLines(m["text"]))
		}
	}
	return strings.Join(parts, "\n")
}

// loadReplayRequests reads the session requests of a coder directory in order.
func loadReplayRequests(dir string) ([]replayRequest, error) {
	files, err := filepath.Glob(filepath.Join(dir, "0*-session*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var out []replayRequest
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var rec struct {
			Wire []struct {
				Request struct {
					Body struct {
						Input []map[string]any `json:"input"`
					} `json:"body"`
				} `json:"request"`
			} `json:"wire"`
			Usage struct {
				InputTokens     int `json:"input_tokens"`
				CacheReadTokens int `json:"cache_read_tokens"`
				OutputTokens    int `json:"output_tokens"`
				ThinkingTokens  int `json:"thinking_tokens"`
				Price           struct {
					USD float64 `json:"usd"`
				} `json:"price"`
			} `json:"usage"`
			Admission struct {
				Estimated struct {
					Tokens int `json:"tokens"`
				} `json:"estimated"`
			} `json:"admission"`
		}
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		if len(rec.Wire) == 0 {
			continue
		}
		req := replayRequest{
			file: filepath.Base(f), input: rec.Usage.InputTokens, cached: rec.Usage.CacheReadTokens,
			output: rec.Usage.OutputTokens + rec.Usage.ThinkingTokens, usd: rec.Usage.Price.USD,
			estimated: rec.Admission.Estimated.Tokens,
		}
		for _, it := range rec.Wire[0].Request.Body.Input {
			typ, _ := it["type"].(string)
			role, _ := it["role"].(string)
			switch {
			case role == "developer" || role == "system":
				req.system += contentText(it["content"])
			case typ == "reasoning":
				enc, _ := it["encrypted_content"].(string)
				req.items = append(req.items, replayItem{kind: "Z", text: enc})
			case typ == "function_call":
				var args map[string]any
				_ = json.Unmarshal([]byte(joinLines(it["arguments"])), &args)
				id, _ := it["call_id"].(string)
				name, _ := it["name"].(string)
				req.items = append(req.items, replayItem{kind: "C", callID: id, name: name, args: args})
			case typ == "function_call_output":
				id, _ := it["call_id"].(string)
				req.items = append(req.items, replayItem{kind: "R", callID: id, text: joinLines(it["output"])})
			case role == "assistant":
				req.items = append(req.items, replayItem{kind: "A", text: contentText(it["content"])})
			case role == "user":
				text := contentText(it["content"])
				kind := "U"
				if strings.HasPrefix(text, "[harness:") {
					// The harness's own text -- focus views, stale notices --
					// rides the request as appended text (loop.appended),
					// which a compaction drops with the epoch.
					kind = "H"
				}
				req.items = append(req.items, replayItem{kind: kind, text: text})
			}
		}
		out = append(out, req)
	}
	return out, nil
}

// sharedPrefix is how many leading items two requests have in common.
func sharedPrefix(prev, cur []string) int {
	n := 0
	for n < len(prev) && n < len(cur) && prev[n] == cur[n] {
		n++
	}
	return n
}

// itemTokens is the tokens a run of canonical items puts in a request, by the
// harness's own estimator (prompt.EstimateTokens: four characters a token),
// except encrypted reasoning, which runs about 13 characters per reasoning
// token (prompt.nativeBlockTokens) and is charged at that.
func itemTokens(items []string) int {
	total := 0
	for _, s := range items {
		if strings.HasPrefix(s, "Z\x00") {
			total += (len(s) + 12) / 13
			continue
		}
		total += (len(s) + 3) / 4
	}
	return total
}

// priceModel is USD per uncached input, cached input and output token, fitted
// by least squares to what the provider billed for the recorded requests.
type priceModel struct{ uncached, cached, output float64 }

func fitPrice(reqs []replayRequest) priceModel {
	var a [3][3]float64
	var b [3]float64
	for _, r := range reqs {
		x := [3]float64{float64(r.input - r.cached), float64(r.cached), float64(r.output)}
		for i := range 3 {
			for j := range 3 {
				a[i][j] += x[i] * x[j]
			}
			b[i] += x[i] * r.usd
		}
	}
	// Gaussian elimination with partial pivoting on the 3x3 normal equations.
	for c := range 3 {
		p := c
		for r := c + 1; r < 3; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		a[c], a[p] = a[p], a[c]
		b[c], b[p] = b[p], b[c]
		if a[c][c] == 0 {
			return priceModel{}
		}
		for r := c + 1; r < 3; r++ {
			f := a[r][c] / a[c][c]
			for k := c; k < 3; k++ {
				a[r][k] -= f * a[c][k]
			}
			b[r] -= f * b[c]
		}
	}
	var x [3]float64
	for r := 2; r >= 0; r-- {
		s := b[r]
		for k := r + 1; k < 3; k++ {
			s -= a[r][k] * x[k]
		}
		x[r] = s / a[r][r]
	}
	return priceModel{uncached: x[0], cached: x[1], output: x[2]}
}

func (p priceModel) cost(input, cached, output int) float64 {
	return p.uncached*float64(input-cached) + p.cached*float64(cached) + p.output*float64(output)
}

// replayMeasure is one side of the A/B for one request.
type replayMeasure struct {
	input, cached int
	usd           float64
	broke         bool // the cache prefix broke before the previous request's end
}

// replayOutcome is the whole A/B over a recording.
type replayOutcome struct {
	requests       []replayRequest
	before, after  []replayMeasure
	recordedUSD    float64
	beforeUSD      float64
	afterUSD       float64
	beforeBreaks   int
	afterBreaks    int
	guardedRequest int // replayed requests in which the budget guard archived a result
}

// replayRecording rebuilds the recorded run and sends it through the
// working-request builder under window tokens and the given working config.
func replayRecording(t *testing.T, reqs []replayRequest, workspace string, window int, working nerdconfig.WorkingConfig) replayOutcome {
	t.Helper()
	if len(reqs) == 0 {
		t.Fatal("no recorded session requests")
	}
	// Every tool result whole: the first request that carried a call's
	// output carried it before any later request cut it.
	full := map[string]replayItem{}
	calls := map[string]replayItem{}
	for _, r := range reqs {
		for _, it := range r.items {
			switch it.kind {
			case "R":
				if prev, ok := full[it.callID]; !ok || strings.HasPrefix(prev.text, archivedResultPrefix) {
					full[it.callID] = it
				}
			case "C":
				calls[it.callID] = it
			}
		}
	}
	anchor := ""
	for _, it := range reqs[0].items {
		if it.kind == "U" {
			anchor = it.text
			break
		}
	}

	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.WorkspaceRoot = workspace
	e.config.TokenBudget = window
	e.config.Working = working
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), nil, anchor, &prompt.CompilationContext{ShardID: "replay"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	defer closeLoop()

	price := fitPrice(reqs)
	if price.uncached <= 0 || price.cached < 0 || price.output < 0 {
		t.Fatalf("the billed usage does not fit a price per token: %+v", price)
	}
	t.Logf("fitted price per million tokens: uncached $%.4f, cached $%.4f, output $%.4f", price.uncached*1e6, price.cached*1e6, price.output*1e6)
	out := replayOutcome{requests: reqs}
	recorded := map[string]bool{}
	harnessSeen := map[string]bool{}
	loop := activeWorkingLoop(ctx)
	var prevBefore, prevAfter []string
	for _, r := range reqs {
		history, placed := replayHistory(r.items, full, anchor)
		// A harness text the run produced at this request is appended where
		// it arose, as the executor appends it; one carried from an earlier
		// request is already there, or a compaction dropped it with its
		// epoch.
		for _, p := range placed {
			if harnessSeen[p.text] {
				continue
			}
			harnessSeen[p.text] = true
			key := ledgerKey(p.index)
			if p.index == 0 {
				key = anchorLedgerKey
			}
			if prior := loop.appended[key]; prior != "" {
				loop.appended[key] = prior + "\n\n" + p.text
			} else {
				loop.appended[key] = p.text
			}
		}
		for _, it := range r.items {
			if it.kind != "R" || recorded[it.callID] {
				continue
			}
			recorded[it.callID] = true
			call := calls[it.callID]
			if err := e.recordWorkingResult(ctx, types.ToolCall{ID: call.callID, Name: call.name, Input: call.args}, full[it.callID].text, nil); err != nil {
				t.Fatalf("%s: recordWorkingResult(%s): %v", r.file, call.name, err)
			}
		}
		messages, err := e.prepareWorkingRequest(ctx, r.system, history, nil)
		if err != nil {
			t.Fatalf("%s: prepareWorkingRequest: %v", r.file, err)
		}

		before := []string{"S\x00" + r.system}
		for _, it := range r.items {
			before = append(before, it.canonical())
		}
		after := []string{"S\x00" + r.system}
		guarded := false
		for _, m := range messages {
			for _, it := range canonicalMessage(m) {
				after = append(after, it.canonical())
				guarded = guarded || (it.kind == "R" && strings.Contains(it.text, "it does not fit the request") &&
					!strings.Contains(full[it.callID].text, "it does not fit the request"))
			}
		}
		if guarded {
			out.guardedRequest++
		}

		// Tokens per byte from what the provider billed for this request.
		// Both sides are counted by one estimator; the billed figures are
		// logged beside them, so the estimator can be read against them.
		measure := func(prev, cur []string) replayMeasure {
			shared := sharedPrefix(prev, cur)
			m := replayMeasure{
				input:  itemTokens(cur),
				cached: itemTokens(cur[:shared]),
				broke:  prev != nil && shared < len(prev),
			}
			m.usd = price.cost(m.input, m.cached, r.output)
			return m
		}
		b, a := measure(prevBefore, before), measure(prevAfter, after)
		out.before = append(out.before, b)
		out.after = append(out.after, a)
		out.recordedUSD += r.usd
		out.beforeUSD += b.usd
		out.afterUSD += a.usd
		if b.broke {
			out.beforeBreaks++
		}
		if a.broke {
			out.afterBreaks++
		}
		prevBefore, prevAfter = before, after
	}
	return out
}

// replayHistory is the executor's history for one recorded request: the
// anchor, then each round as an assistant turn (reasoning, text, calls) and a
// user turn (results, and any harness text the request carried after them).
func replayHistory(items []replayItem, full map[string]replayItem, anchor string) ([]types.Message, []harnessPlacement) {
	history := []types.Message{{Role: "user", Text: anchor}}
	var placed []harnessPlacement
	var assistant, user []types.ContentBlock
	flush := func() {
		if len(assistant) > 0 {
			history = append(history, types.NewAssistantMessage(assistant...))
			assistant = nil
		}
		if len(user) > 0 {
			history = append(history, types.NewUserMessage(user...))
			user = nil
		}
	}
	seenAnchor := false
	for _, it := range items {
		switch it.kind {
		case "U":
			if !seenAnchor && it.text == anchor {
				seenAnchor = true
				continue
			}
			if len(assistant) > 0 && len(user) == 0 {
				flush()
			}
			user = append(user, types.TextBlock(it.text))
		case "Z":
			if len(user) > 0 {
				flush()
			}
			assistant = append(assistant, types.RedactedThinkingBlock(it.text))
		case "A":
			if len(user) > 0 {
				flush()
			}
			assistant = append(assistant, types.TextBlock(it.text))
		case "C":
			if len(user) > 0 {
				flush()
			}
			assistant = append(assistant, types.ToolUseBlock(it.callID, it.name, it.args))
		case "R":
			r := full[it.callID]
			user = append(user, types.ToolResultBlock(r.callID, r.text, r.isError))
		case "H":
			// Harness text trails the user turn it arose on: the pending
			// one, or the last one already in the history.
			idx := len(history) - 1
			if len(user) > 0 {
				idx = len(history)
				if len(assistant) > 0 {
					idx++
				}
			}
			placed = append(placed, harnessPlacement{index: idx, text: it.text})
		}
	}
	flush()
	return history, placed
}

// harnessPlacement is one recorded harness text and the history index of the
// user turn it was appended to.
type harnessPlacement struct {
	index int
	text  string
}

// canonicalMessage reduces a built message to the items a provider would see.
func canonicalMessage(m types.Message) []replayItem {
	var out []replayItem
	for _, b := range m.Content() {
		switch b.Kind {
		case types.BlockThinking:
			out = append(out, replayItem{kind: "Z", text: b.Signature})
		case types.BlockText:
			kind := "U"
			if m.Role == "assistant" {
				kind = "A"
			}
			out = append(out, replayItem{kind: kind, text: b.Text})
		case types.BlockToolUse:
			out = append(out, replayItem{kind: "C", callID: b.ID, name: b.Name, args: b.Input})
		case types.BlockToolResult:
			out = append(out, replayItem{kind: "R", callID: b.ToolUseID, text: b.Text})
		}
	}
	return out
}

// replayWindow is the input window the recorded requests were held under:
// the largest request the harness estimated plus the guard's reserve.
func replayWindow(reqs []replayRequest) int {
	window := 0
	for _, r := range reqs {
		window = max(window, r.estimated)
	}
	return window + workingInputReserve
}

func logReplay(t *testing.T, label string, out replayOutcome) {
	t.Helper()
	for i, r := range out.requests {
		b, a := out.before[i], out.after[i]
		t.Logf("%-28s billed %6d in %6d cached $%.4f | before %6d/%6d $%.4f break=%v | after %6d/%6d $%.4f break=%v",
			r.file, r.input, r.cached, r.usd, b.input, b.cached, b.usd, b.broke, a.input, a.cached, a.usd, a.broke)
	}
	t.Logf("%s: %d requests; billed $%.4f; modeled before $%.4f (%d prefix breaks); after $%.4f (%d prefix breaks, guard archived in %d)",
		label, len(out.requests), out.recordedUSD, out.beforeUSD, out.beforeBreaks, out.afterUSD, out.afterBreaks, out.guardedRequest)
}

// TestReplayRecordedSession is the A/B on a real recording. It runs only when
// CODENERD_REPLAY_DIR names a coder directory.
func TestReplayRecordedSession(t *testing.T) {
	dir := os.Getenv("CODENERD_REPLAY_DIR")
	if dir == "" {
		t.Skip("set CODENERD_REPLAY_DIR to a recorded .nerd/sessions/<session>/coder-<id> directory")
	}
	reqs, err := loadReplayRequests(dir)
	if err != nil {
		t.Fatal(err)
	}
	// The recording's workspace: <ws>/.nerd/sessions/<session>/coder-<id>.
	ws := filepath.Clean(filepath.Join(dir, "..", "..", "..", ".."))
	cfg, err := nerdconfig.LoadUserConfig(filepath.Join(ws, ".nerd", "config.json"))
	if err != nil {
		t.Fatalf("load the recording workspace's config: %v", err)
	}
	working := cfg.GetWorkingConfig()
	if v := os.Getenv("CODENERD_REPLAY_LOW_WATER"); v != "" {
		if _, err := fmt.Sscan(v, &working.LedgerLowWaterPercent); err != nil {
			t.Fatalf("CODENERD_REPLAY_LOW_WATER: %v", err)
		}
	}
	window := replayWindow(reqs)
	t.Logf("window %d tokens, ledger ceiling %d bytes, low water %d%%", window, working.LedgerCeilingBytes, working.LedgerLowWaterPercent)
	logReplay(t, filepath.Base(dir), replayRecording(t, reqs, ws, window, working))
}

// The replay itself, on a synthetic recording: a run whose results outgrow
// the window. The recorded side is what the builder used to send -- every
// result whole until the window, then the guard cutting the largest one in
// the middle of each request -- and the replay must rebuild the same history
// and measure both sides.
func TestReplayRecording_MeasuresBothSidesOfARecordedRun(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "sessions", "s", "coder-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var input []map[string]any
	input = append(input,
		map[string]any{"role": "developer", "content": []any{map[string]any{"type": "input_text", "text": "SYSTEM"}}},
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "fix the thing"}}})
	const rounds, size = 24, 6000
	for n := 1; n <= rounds; n++ {
		name := fmt.Sprintf("f%02d.go", n)
		if err := os.WriteFile(filepath.Join(ws, name), []byte("package f\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		id := fmt.Sprintf("call-%02d", n)
		input = append(input,
			map[string]any{"type": "function_call", "call_id": id, "name": "read_file", "arguments": fmt.Sprintf(`{"path":%q}`, name)},
			map[string]any{"type": "function_call_output", "call_id": id, "output": strings.Repeat(fmt.Sprintf("r%02d ", n), size/4)})
		// The recorded request: what the old builder sent. Past the window
		// it cut the oldest whole result -- a different one each request.
		sent := append([]map[string]any(nil), input...)
		cut := n - 16
		for i, it := range sent {
			if it["type"] == "function_call_output" && cut > 0 {
				copyIt := map[string]any{}
				for k, v := range it {
					copyIt[k] = v
				}
				copyIt["output"] = archivedResultPrefix + " [codenerd: truncated] it does not fit the request"
				sent[i] = copyIt
				cut--
			}
		}
		rec := map[string]any{
			"wire": []any{map[string]any{"request": map[string]any{"body": map[string]any{"input": sent}}}},
			"usage": map[string]any{"input_tokens": 4000 + n*1500, "cache_read_tokens": 3000 + n*1400, "output_tokens": 50 + n*n,
				"price": map[string]any{"usd": 1e-7*float64(1000+n*100) + 2e-9*float64(3000+n*1400) + 2e-7*float64(50+n*n)}},
			"admission": map[string]any{"estimated": map[string]any{"tokens": min(4000+n*1500, 28000)}},
		}
		data, _ := json.Marshal(rec)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d-session.json", n)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	reqs, err := loadReplayRequests(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != rounds {
		t.Fatalf("loaded %d requests, want %d", len(reqs), rounds)
	}
	working := nerdconfig.DefaultWorkingConfig()
	out := replayRecording(t, reqs, ws, replayWindow(reqs), working)
	logReplay(t, "synthetic", out)
	if len(out.after) != rounds || out.afterUSD <= 0 || out.beforeUSD <= 0 {
		t.Fatalf("the replay measured %d requests, after $%f, before $%f", len(out.after), out.afterUSD, out.beforeUSD)
	}
	if out.beforeBreaks == 0 {
		t.Fatal("the recorded side cut a result from the middle of 8 requests, and the replay saw no prefix break")
	}
	if out.guardedRequest != 0 {
		t.Errorf("the builder's guard archived results in %d replayed requests; the policy should have compacted first", out.guardedRequest)
	}
	if out.afterBreaks >= out.beforeBreaks {
		t.Errorf("the builder broke the prefix %d times against %d recorded", out.afterBreaks, out.beforeBreaks)
	}
}
