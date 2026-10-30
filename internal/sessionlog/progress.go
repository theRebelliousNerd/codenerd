package sessionlog

import (
	"codenerd/internal/usage"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// progressState is an agent's live progress, kept from the events its
// transcript records: the harness writes them, so progress needs nothing from
// the agent or the model.
type progressState struct {
	modelCalls  int
	callErrors  int
	toolResults int
	toolErrors  int
	lastTool    string
	last        time.Time
	waitingOn   string
	waitSince   time.Time
	finished    bool
	finalError  string
}

func (p *progressState) observe(ev TranscriptEvent) {
	p.last = ev.Time
	switch ev.Kind {
	case EventCallStart:
		p.waitingOn, p.waitSince = ev.Model, ev.Time
	case EventAssistant:
		p.modelCalls++
		p.waitingOn = ""
	case EventCallFailed:
		p.modelCalls++
		p.callErrors++
		p.waitingOn = ""
	case EventToolResult:
		p.toolResults++
		if ev.IsError {
			p.toolErrors++
		}
		p.lastTool = ev.Tool
	case EventFinal:
		p.finished = true
		p.waitingOn = ""
		p.finalError = ev.Error
	}
}

// Progress is one agent's progress at a moment.
type Progress struct {
	Agent       string
	Started     time.Time
	ModelCalls  int
	CallErrors  int
	ToolResults int
	ToolErrors  int
	LastTool    string
	LastEvent   time.Time
	WaitingOn   string
	WaitSince   time.Time
	Finished    bool
	FinalError  string
}

// Text is the progress in one line, measured against now.
func (p Progress) Text(now time.Time) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s, running %s", p.Agent, roundDur(now.Sub(p.Started))))
	if p.Finished {
		parts[0] = fmt.Sprintf("%s, finished after %s", p.Agent, roundDur(p.LastEvent.Sub(p.Started)))
	}
	parts = append(parts, fmt.Sprintf("%d model call(s)", p.ModelCalls))
	if p.CallErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", p.CallErrors))
	}
	parts = append(parts, fmt.Sprintf("%d tool result(s)", p.ToolResults))
	if p.ToolErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d tool error(s)", p.ToolErrors))
	}
	if p.LastTool != "" {
		parts = append(parts, "last tool "+p.LastTool)
	}
	switch {
	case p.WaitingOn != "":
		parts = append(parts, fmt.Sprintf("waiting on %s for %s", p.WaitingOn, roundDur(now.Sub(p.WaitSince))))
	case !p.Finished && !p.LastEvent.IsZero():
		parts = append(parts, fmt.Sprintf("last activity %s ago", roundDur(now.Sub(p.LastEvent))))
	}
	if p.FinalError != "" {
		parts = append(parts, "ended with error: "+p.FinalError)
	}
	return strings.Join(parts, "; ")
}

func roundDur(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d.Round(time.Second)
}

// ProgressForTask returns the progress of every agent in the session working
// on the task id (WithTask), oldest first.
func (s *Store) ProgressForTask(session, task string) []Progress {
	if s == nil || task == "" {
		return nil
	}
	session = s.resolve(session)
	s.mu.Lock()
	var ts []*Transcript
	for _, t := range s.transcripts {
		if t.header.Session == session {
			ts = append(ts, t)
		}
	}
	s.mu.Unlock()
	var out []Progress
	for _, t := range ts {
		t.mu.Lock()
		if t.task == task {
			p := t.prog
			out = append(out, Progress{
				Agent: t.header.Agent, Started: t.header.Started,
				ModelCalls: p.modelCalls, CallErrors: p.callErrors,
				ToolResults: p.toolResults, ToolErrors: p.toolErrors, LastTool: p.lastTool,
				LastEvent: p.last, WaitingOn: p.waitingOn, WaitSince: p.waitSince,
				Finished: p.finished, FinalError: p.finalError,
			})
		}
		t.mu.Unlock()
	}
	sortProgress(out)
	return out
}

func sortProgress(ps []Progress) {
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j].Started.Before(ps[j-1].Started); j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}

// setTask records the task id the agent works on.
func (t *Transcript) setTask(task string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.task = task
	t.mu.Unlock()
}

// LiveEvent is a compact, session-owned execution update, not a tool payload or
// model reasoning trace. It is independent of transcript storage and debug mode.
type LiveEvent struct {
	Session string
	Agent   string
	Kind    string
	Text    string
	Tool    string
	CallID  string
	At      time.Time
	Failed  bool
	Status  string // PASS/FAIL are process outcomes; DONE alone is not test evidence.
}

type liveProgressKey struct{}

// WithLiveProgress carries a UI-owned sink through delegated and nested agents.
// The sink must be concurrency-safe and honor the supplied context on shutdown.
func WithLiveProgress(ctx context.Context, sink func(context.Context, LiveEvent)) context.Context {
	return context.WithValue(ctx, liveProgressKey{}, sink)
}

// HasLiveProgress reports whether this delegated scope has a live consumer.
// Brokers use it to capture visible prose even when disk logging is unavailable.
func HasLiveProgress(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	sink, _ := ctx.Value(liveProgressKey{}).(func(context.Context, LiveEvent))
	scope := ScopeFrom(ctx)
	return sink != nil && scope != nil && scope.Transcript
}

// EmitLiveProgress reports an observation only for a scoped delegated agent.
// No store, file write or debug subscription is required to make work visible.
func EmitLiveProgress(ctx context.Context, ev LiveEvent) {
	if ctx == nil {
		return
	}
	sink, _ := ctx.Value(liveProgressKey{}).(func(context.Context, LiveEvent))
	scope := ScopeFrom(ctx)
	if sink == nil || scope == nil || !scope.Transcript || scope.Service {
		return
	}
	ev.Session = usage.SessionIDFromContext(ctx)
	ev.Agent = scope.Folder
	ev.At = time.Now()
	sink(ctx, ev)
}

// LiveToolSubject selects identifiers only: never source, commands, credentials
// or arbitrary tool output. Multi-file edits keep each affected path visible.
func LiveToolSubject(name string, args map[string]any) string {
	var subjects []string
	for _, key := range []string{"path", "paths", "packages", "ref", "refs", "symbol", "predicate", "name", "edited_refs"} {
		switch v := args[key].(type) {
		case string:
			if v != "" {
				subjects = append(subjects, v)
			}
		case []string:
			subjects = append(subjects, v...)
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					subjects = append(subjects, s)
				}
			}
		}
	}
	if edits, ok := args["edits"].([]any); ok {
		for _, item := range edits {
			if edit, ok := item.(map[string]any); ok {
				if p, ok := edit["path"].(string); ok {
					subjects = append(subjects, p)
				}
			}
		}
	}
	if len(subjects) == 0 {
		switch name {
		case "run_build":
			return "./..."
		case "run_tests", "run_lint", "run_impacted_tests":
			return "changed packages"
		}
	}
	return strings.Join(subjects, ", ")
}

// liveAssistant reports visible prose, including plans, but never the envelope's
// control packet or a model's private reasoning. Long prose is split, not lost.
func liveAssistant(ctx context.Context, text string) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "```") {
		var envelope struct {
			Surface string `json:"surface_response"`
			Control struct {
				Summary string `json:"decision_summary"`
			} `json:"control_packet"`
		}
		body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(text, "```json"), "```"), "```"))
		if err := json.Unmarshal([]byte(body), &envelope); err != nil {
			return
		}
		text = envelope.Surface
		if strings.TrimSpace(text) == "" {
			text = envelope.Control.Summary
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		runes := []rune(line)
		for len(runes) > 0 {
			n := min(240, len(runes))
			EmitLiveProgress(ctx, LiveEvent{Kind: EventAssistant, Text: string(runes[:n])})
			runes = runes[n:]
		}
	}
}
