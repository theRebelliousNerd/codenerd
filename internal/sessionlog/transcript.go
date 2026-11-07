package sessionlog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/usage"
)

// TranscriptSchema names the transcript file format.
const TranscriptSchema = "codenerd.transcript/1"

// Transcript event kinds.
const (
	EventInput      = "input"        // the task or user input the agent was given
	EventAssistant  = "assistant"    // one model answer, with the tool calls it asked for
	EventToolResult = "tool_result"  // what a tool call returned to the agent
	EventFinal      = "final"        // the agent's final answer or the error that ended it
	EventCallStart  = "call_started" // a model call went out; its answer or failure follows
	EventCallFailed = "call_failed"  // a model call ended in an error

	// LiveCallDone is a live-progress kind only, never a transcript entry:
	// the model answered, so whatever follows (tools, the turn's gates) is no
	// longer "waiting for model".
	LiveCallDone = "call_done"
)

// TranscriptEvent is one entry of a transcript.
type TranscriptEvent struct {
	Seq        int        `json:"seq"`
	Time       time.Time  `json:"time"`
	Turn       int        `json:"turn,omitempty"`
	Kind       string     `json:"kind"`
	Model      string     `json:"model,omitempty"`
	Text       []string   `json:"text,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	Tool       string     `json:"tool,omitempty"`
	CallID     string     `json:"call_id,omitempty"`
	IsError    bool       `json:"is_error,omitempty"`
	StopReason string     `json:"stop_reason,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// Transcript is one agent's conversation record. The harness appends to it at
// the points every agent already passes through, and each event is on disk
// when the call returns, so an agent that crashes or is cancelled leaves a
// transcript complete up to that point. All methods are nil-safe.
//
// Each append atomically replaces a complete JSON snapshot. A reader sees the
// previous committed history until its replacement is ready. Writers in one
// Store are serialized by mu; separate processes must not share an agent path.
type Transcript struct {
	store  *Store
	path   string
	header transcriptHeader

	mu     sync.Mutex
	seq    int
	opened bool
	broken bool
	// progMu guards task and prog so live progress queries (Store.ProgressForTask)
	// never block behind whole-history JSON decoding and atomicfile synchronization.
	progMu sync.Mutex
	// task is the task id the agent works on (WithTask), so a caller holding
	// the id can find this agent's progress (Store.ProgressForTask).
	task string
	prog progressState
}

type transcriptHeader struct {
	Schema       string    `json:"schema"`
	TextEncoding string    `json:"text_encoding"`
	Session      string    `json:"session"`
	Agent        string    `json:"agent"`
	Role         string    `json:"role,omitempty"`
	AgentID      string    `json:"agent_id,omitempty"`
	Started      time.Time `json:"started"`
}

// transcriptTail closes the events array and the document.
const transcriptTail = "\n  ]\n}\n"

// openTranscript returns the transcript for an agent folder, creating it on
// first use.
func (s *Store) openTranscript(session string, scope *Scope) *Transcript {
	key := session + "\x00" + scope.Folder
	s.mu.Lock()
	t, ok := s.transcripts[key]
	s.mu.Unlock()
	if ok {
		return t
	}
	dir, err := s.folderDir(session, scope.Folder)
	if err != nil {
		s.fail(err)
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.transcripts[key]; ok {
		return t
	}
	t = &Transcript{store: s, path: filepath.Join(dir, TranscriptFile), header: transcriptHeader{
		Schema: TranscriptSchema, TextEncoding: TextEncoding, Session: session,
		Agent: scope.Folder, Role: scope.Role, AgentID: scope.ID, Started: time.Now(),
	}}
	s.transcripts[key] = t
	return t
}

// TranscriptFrom returns the transcript of the agent ctx is scoped to, or nil
// when ctx carries no agent that keeps one or no store is installed. Unlike
// exchange files it does not depend on debug mode.
func TranscriptFrom(ctx context.Context) *Transcript {
	s := Current()
	scope := ScopeFrom(ctx)
	if s == nil || scope == nil || !scope.Transcript {
		return nil
	}
	return s.openTranscript(s.resolve(usage.SessionIDFromContext(ctx)), scope)
}

// Input records what the agent was asked to do.
func (t *Transcript) Input(ctx context.Context, text string) {
	EmitLiveProgress(ctx, LiveEvent{Kind: EventInput, Text: "starting delegated work"})
	t.append(TranscriptEvent{Kind: EventInput, Turn: TurnFrom(ctx), Text: Lines(text)})
}

// Assistant records one model answer.
func (t *Transcript) Assistant(ctx context.Context, model string, r *Response) {
	if r == nil {
		return
	}
	answered := "model answered: final"
	if n := len(r.ToolCalls); n > 0 {
		answered = fmt.Sprintf("model answered: %d tool call(s)", n)
	}
	EmitLiveProgress(ctx, LiveEvent{Kind: LiveCallDone, Text: answered})
	liveAssistant(ctx, Join(r.Text))
	if t == nil {
		return
	}
	ev := TranscriptEvent{Kind: EventAssistant, Turn: TurnFrom(ctx), Model: model, StopReason: r.StopReason, ToolCalls: r.ToolCalls}
	if len(r.Text) > 0 {
		ev.Text = r.Text
	}
	t.append(ev)
}

// ToolResult records what a tool call returned to the agent.
func (t *Transcript) ToolResult(ctx context.Context, tool, callID, content string, isError bool) {
	t.append(TranscriptEvent{Kind: EventToolResult, Turn: TurnFrom(ctx), Tool: tool, CallID: callID, IsError: isError, Text: Lines(content)})
}

// CallStarted records that a model call went out, so a long call reads as
// "waiting on the model" while it runs.
func (t *Transcript) CallStarted(ctx context.Context, model string) {
	EmitLiveProgress(ctx, LiveEvent{Kind: EventCallStart, Text: "waiting for model"})
	t.append(TranscriptEvent{Kind: EventCallStart, Turn: TurnFrom(ctx), Model: model})
}

// CallFailed records a model call that ended in an error.
func (t *Transcript) CallFailed(ctx context.Context, model string, err error) {
	EmitLiveProgress(ctx, LiveEvent{Kind: EventCallFailed, Text: "model call failed", Failed: true})
	ev := TranscriptEvent{Kind: EventCallFailed, Turn: TurnFrom(ctx), Model: model}
	if err != nil {
		ev.Error = err.Error()
	}
	t.append(ev)
}

// Final records the agent's last word for a turn.
func (t *Transcript) Final(ctx context.Context, text string, err error) {
	EmitLiveProgress(ctx, LiveEvent{Kind: EventFinal, Text: "delegated run ended", Failed: err != nil})
	ev := TranscriptEvent{Kind: EventFinal, Turn: TurnFrom(ctx), Text: Lines(text)}
	if err != nil {
		ev.Error = err.Error()
	}
	t.append(ev)
}

func (t *Transcript) append(ev TranscriptEvent) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.broken {
		return
	}
	if err := t.open(); err != nil {
		t.broken = true
		t.store.fail(err)
		return
	}
	t.seq++
	ev.Seq = t.seq
	ev.Time = time.Now()
	t.progMu.Lock()
	t.prog.observe(ev)
	t.progMu.Unlock()
	body, err := marshalEvent(ev)
	if err != nil {
		t.seq--
		t.store.fail(err)
		return
	}
	if err := t.write(body); err != nil {
		t.seq--
		t.store.fail(err)
	}
}

func marshalEvent(ev TranscriptEvent) ([]byte, error) {
	data, err := MarshalFile(ev)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// open reads an existing file's event count or writes a new header, once.
func (t *Transcript) open() error {
	if t.opened {
		return nil
	}
	data, err := readTranscriptFile(t.path)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return err
	}
	if len(data) > 0 {
		recovered, err := recoverTranscriptTail(data, t.header)
		if err != nil {
			return fmt.Errorf("transcript %s cannot be recovered: %w", t.path, err)
		}
		if !bytes.Equal(data, recovered) {
			if err := atomicfile.WriteFilePreservingMode(t.path, recovered, 0o644); err != nil {
				return err
			}
		}
		var existing struct {
			Events []struct {
				Seq int `json:"seq"`
			} `json:"events"`
		}
		if err := json.Unmarshal(recovered, &existing); err != nil {
			return fmt.Errorf("transcript %s exists and is not valid JSON: %w", t.path, err)
		}
		for _, e := range existing.Events {
			if e.Seq > t.seq {
				t.seq = e.Seq
			}
		}
		t.opened = true
		return nil
	}
	head, err := MarshalFile(t.header)
	if err != nil {
		return err
	}
	// head ends "...\n}\n": reopen the object to add the events array.
	head = head[:len(head)-len("\n}\n")]
	doc := append(head, []byte(",\n  \"events\": ["+transcriptTail)...)
	if missing {
		// Let the OS apply the inherited umask, as the old first write did.
		// Only an empty new file is exposed before atomic publication; no
		// completed event or header is ever mutated in place.
		f, err := os.OpenFile(t.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	if err := atomicfile.WriteFilePreservingMode(t.path, doc, 0o644); err != nil {
		return err
	}
	t.opened = true
	return nil
}

// write publishes an extended snapshot without modifying the committed inode.
// Preserve ordered/unknown fields and exact json.Number values, not only fields
// known to this version. This costs a read and rewrite proportional to history.
func (t *Transcript) write(body []byte) error {
	data, err := readTranscriptFile(t.path)
	if err != nil {
		return err
	}
	tree, err := DecodeOrdered(data)
	if err != nil {
		return err
	}
	doc, ok := tree.(*Object)
	if !ok {
		return fmt.Errorf("transcript %s is not an object", t.path)
	}
	events, ok := doc.Values["events"].([]any)
	if !ok {
		return fmt.Errorf("transcript %s has no events array", t.path)
	}
	event, err := DecodeOrdered(body)
	if err != nil {
		return err
	}
	doc.Values["events"] = append(events, event)
	snapshot, err := MarshalFile(doc)
	if err != nil {
		return err
	}
	return atomicfile.WriteFilePreservingMode(t.path, snapshot, 0o644)
}

// readTranscriptFile allows atomic replacement while a Windows reader is open.
func readTranscriptFile(path string) ([]byte, error) {
	f, err := atomicfile.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// recoverTranscriptTail repairs only a recognized legacy append interruption.
// Complete prior event bytes and header extensions are retained verbatim. Bad
// earlier syntax or an unknown identity remains an error, never a new document.
func recoverTranscriptTail(data []byte, expected transcriptHeader) ([]byte, error) {
	if json.Valid(data) {
		return data, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, fmt.Errorf("invalid transcript header")
	}
	header := make(map[string]json.RawMessage)
	for dec.More() {
		tok, err := dec.Token()
		key, ok := tok.(string)
		if err != nil || !ok {
			return nil, fmt.Errorf("invalid transcript header field")
		}
		if _, duplicate := header[key]; duplicate {
			return nil, fmt.Errorf("duplicate transcript header field")
		}
		if key != "events" {
			var value json.RawMessage
			if err := dec.Decode(&value); err != nil {
				return nil, err
			}
			header[key] = value
			continue
		}
		for name, want := range map[string]string{"schema": TranscriptSchema, "text_encoding": TextEncoding, "session": expected.Session, "agent": expected.Agent} {
			var got string
			if err := json.Unmarshal(header[name], &got); err != nil || got != want {
				return nil, fmt.Errorf("unrecognized transcript identity")
			}
		}
		if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
			return nil, fmt.Errorf("invalid transcript events array")
		}
		last := dec.InputOffset()
		partial := false
		for dec.More() {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !interruptedTranscriptTail(data[last:]) {
					return nil, err
				}
				partial = true
				break
			}
			var ev TranscriptEvent
			if err := json.Unmarshal(raw, &ev); err != nil || ev.Seq <= 0 || ev.Kind == "" {
				return nil, fmt.Errorf("invalid completed transcript event")
			}
			last = dec.InputOffset()
		}
		// More also returns false on a closing delimiter or EOF. Refuse any
		// unrelated bytes after a completed array instead of erasing them.
		if !partial && !interruptedTranscriptTail(data[last:]) {
			return nil, fmt.Errorf("invalid transcript closing tail")
		}
		recovered := append(append([]byte(nil), data[:last]...), transcriptTail...)
		if !json.Valid(recovered) {
			return nil, fmt.Errorf("invalid recovered transcript")
		}
		return recovered, nil
	}
	return nil, fmt.Errorf("transcript events array not found")
}

// Very early writes can leave closing bytes of the old fixed tail after a
// comma/whitespace prefix. Later partial events must end at EOF instead.
func interruptedTranscriptTail(data []byte) bool {
	compact := bytes.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, data)
	compact = bytes.TrimPrefix(compact, []byte(","))
	switch string(compact) {
	case "", "]", "}", "]}":
		return true
	default:
		return false
	}
}

// indentEvent indents an event body (rendered at column 0) to sit inside the
// events array.
func indentEvent(body []byte) string {
	out := make([]byte, 0, len(body)+64)
	lineStart := true
	for _, b := range body {
		if lineStart && b != '\n' {
			out = append(out, "    "...)
		}
		out = append(out, b)
		lineStart = b == '\n'
	}
	for len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return string(out)
}
