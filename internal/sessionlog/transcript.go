package sessionlog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

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
// The file is a JSON document that stays valid after every append: a fixed
// closing tail is overwritten by the next event and the tail rewritten after
// it.
type Transcript struct {
	store  *Store
	path   string
	header transcriptHeader

	mu     sync.Mutex
	seq    int
	opened bool
	broken bool
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
	t.prog.observe(ev)
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
	if data, err := os.ReadFile(t.path); err == nil && len(data) > 0 {
		var existing struct {
			Events []struct {
				Seq int `json:"seq"`
			} `json:"events"`
		}
		if err := json.Unmarshal(data, &existing); err != nil {
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
	if err := os.WriteFile(t.path, doc, 0o644); err != nil {
		return err
	}
	t.opened = true
	return nil
}

// write appends one event body before the tail.
func (t *Transcript) write(body []byte) (err error) {
	f, err := os.OpenFile(t.path, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	tail := int64(len(transcriptTail))
	if size < tail {
		return fmt.Errorf("transcript %s is shorter than its tail", t.path)
	}
	// An empty array ends "[" + tail; a non-empty one ends "}" + tail.
	probe := make([]byte, 1)
	if _, err := f.ReadAt(probe, size-tail-1); err != nil {
		return err
	}
	sep := "\n"
	if probe[0] == '}' {
		sep = ",\n"
	}
	if _, err := f.Seek(size-tail, io.SeekStart); err != nil {
		return err
	}
	indented := indentEvent(body)
	_, err = f.WriteString(sep + indented + transcriptTail)
	return err
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
