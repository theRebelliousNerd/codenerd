package sessionlog

import (
	"codenerd/internal/usage"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// An agent's progress comes from the events the harness records: a call that
// went out reads as waiting, answers and tool results are counted, and the
// task id it was started under finds it.
func TestProgressForTask_FollowsTheAgentsTranscript(t *testing.T) {
	s, _ := installStore(t, "session-p", false)
	ctx := WithTask(context.Background(), "bg-42")

	reached := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = RunAgent(ctx, "coder", "7", "fix the loader", func(ctx context.Context) (string, error) {
			tr := TranscriptFrom(ctx)
			tr.CallStarted(ctx, "opus")
			tr.Assistant(ctx, "opus", &Response{Text: []string{"reading"}})
			tr.ToolResult(ctx, "read_file", "c1", "package prompt", false)
			tr.ToolResult(ctx, "edit_element", "c2", "old does not occur", true)
			tr.CallFailed(ctx, "opus", errors.New("429"))
			tr.CallStarted(ctx, "opus")
			close(reached)
			<-release
			return "done", nil
		})
	}()
	<-reached

	got := s.ProgressForTask("session-p", "bg-42")
	if len(got) != 1 {
		t.Fatalf("progress for the task = %+v, want one agent", got)
	}
	p := got[0]
	if p.ModelCalls != 2 || p.CallErrors != 1 || p.ToolResults != 2 || p.ToolErrors != 1 || p.LastTool != "edit_element" || p.WaitingOn != "opus" || p.Finished {
		t.Fatalf("progress = %+v", p)
	}
	text := p.Text(time.Now())
	for _, want := range []string{"coder-7", "2 model call(s)", "1 failed", "1 tool error(s)", "last tool edit_element", "waiting on opus"} {
		if !strings.Contains(text, want) {
			t.Errorf("progress text %q lacks %q", text, want)
		}
	}
	if other := s.ProgressForTask("session-p", "bg-other"); len(other) != 0 {
		t.Fatalf("another task's id found this agent: %+v", other)
	}

	close(release)
	<-done
	if p := s.ProgressForTask("session-p", "bg-42")[0]; !p.Finished || p.WaitingOn != "" {
		t.Fatalf("after the run ended: %+v", p)
	}
}

func TestLiveProgress_ReportsBeforeAgentReturns(t *testing.T) {
	for _, withStore := range []bool{false, true} {
		t.Run(fmt.Sprintf("store=%v", withStore), func(t *testing.T) {
			if withStore {
				installStore(t, "live-session", false)
			}
			var events []LiveEvent
			ctx := WithLiveProgress(usage.WithSessionID(context.Background(), "live-session"), func(_ context.Context, ev LiveEvent) { events = append(events, ev) })
			_, err := RunAgent(ctx, "coder", "live", "fix loader", func(ctx context.Context) (string, error) {
				tr := TranscriptFrom(ctx)
				tr.CallStarted(ctx, "model")
				tr.Assistant(ctx, "model", &Response{Text: []string{`{"control_packet":{"private":"do not display"},"surface_response":"Plan: inspect loader\nThen fix it"}`}})
				if len(events) != 4 {
					t.Fatalf("before return: got %d events, want start, waiting and both plan lines", len(events))
				}
				tr.CallFailed(ctx, "model", errors.New("provider failed"))
				return "partial", context.Canceled
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("run error = %v", err)
			}
			if len(events) != 6 || events[5].Kind != EventFinal || !events[5].Failed {
				t.Fatalf("events = %+v", events)
			}
			for _, ev := range events {
				if ev.Session != "live-session" || ev.Agent != "coder-live" || ev.At.IsZero() {
					t.Errorf("missing ownership/time: %+v", ev)
				}
				if strings.Contains(ev.Text, "do not display") || strings.Contains(ev.Text, "control_packet") {
					t.Errorf("control packet leaked: %s", ev.Text)
				}
			}
		})
	}
}

func TestLiveToolSubject_UsesIdentifiersNotPayloads(t *testing.T) {
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"read_file", map[string]any{"path": "internal/a.go"}, "internal/a.go"},
		{"edit_element", map[string]any{"path": "internal/a.go", "ref": "a.F", "new": "private source"}, "internal/a.go, a.F"},
		{"run_tests", map[string]any{"packages": []any{"./internal/session", "./cmd/nerd/chat"}}, "./internal/session, ./cmd/nerd/chat"},
		{"run_build", nil, "./..."},
		{"run_tests", nil, "changed packages"},
		{"run_lint", map[string]any{"packages": []string{"./internal/session"}}, "./internal/session"},
		{"apply_edits", map[string]any{"edits": []any{map[string]any{"path": "a.go"}, map[string]any{"path": "b.go"}}}, "a.go, b.go"},
		{"other", map[string]any{"content": "private source", "api_key": "not a subject"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.want, func(t *testing.T) {
			if got := LiveToolSubject(tc.name, tc.args); got != tc.want {
				t.Errorf("subject = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLiveAssistant_SplitsUnicodeWithoutLosingProse(t *testing.T) {
	var events []LiveEvent
	ctx := WithAgent(WithLiveProgress(context.Background(), func(_ context.Context, ev LiveEvent) { events = append(events, ev) }), "coder", "1")
	text := strings.Repeat("界", 601)
	liveAssistant(ctx, text)
	var joined string
	for _, ev := range events {
		if len([]rune(ev.Text)) > 240 || !utf8.ValidString(ev.Text) {
			t.Fatalf("invalid chunk: %q", ev.Text)
		}
		joined += ev.Text
	}
	if joined != text {
		t.Fatalf("prose was lost: %d runes", len([]rune(joined)))
	}
	before := len(events)
	liveAssistant(ctx, `{"control_packet":`)
	EmitLiveProgress(WithChat(ctx), LiveEvent{Text: "not delegated"})
	if len(events) != before {
		t.Fatal("malformed envelope or chat-only event reached delegated feed")
	}
}

func TestLiveAssistant_UsesOnlyPublicDecisionSummaryWhenSurfaceIsEmpty(t *testing.T) {
	var events []LiveEvent
	ctx := WithAgent(WithLiveProgress(context.Background(), func(_ context.Context, ev LiveEvent) { events = append(events, ev) }), "coder", "summary")
	liveAssistant(ctx, `{"control_packet":{"decision_summary":"Inspect parser, then run tests.","private":"never display"},"surface_response":""}`)
	if len(events) != 1 || events[0].Text != "Inspect parser, then run tests." {
		t.Fatalf("public summary = %+v", events)
	}
}

func TestLiveProgress_ContextBoundaries(t *testing.T) {
	cases := []struct {
		name        string
		makeContext func(func(context.Context, LiveEvent)) context.Context
		want        bool
	}{
		{"nil context", func(_ func(context.Context, LiveEvent)) context.Context { return nil }, false},
		{"unscoped without sink", func(_ func(context.Context, LiveEvent)) context.Context { return context.Background() }, false},
		{"agent without sink", func(_ func(context.Context, LiveEvent)) context.Context {
			return WithAgent(context.Background(), "coder", "boundary")
		}, false},
		{"sink without scope", func(sink func(context.Context, LiveEvent)) context.Context {
			return WithLiveProgress(context.Background(), sink)
		}, false},
		{"chat is not delegated", func(sink func(context.Context, LiveEvent)) context.Context {
			return WithChat(WithLiveProgress(context.Background(), sink))
		}, false},
		{"delegated consumer", func(sink func(context.Context, LiveEvent)) context.Context {
			return WithAgent(WithLiveProgress(usage.WithSessionID(context.Background(), "boundary-session"), sink), "coder", "boundary")
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var events []LiveEvent
			ctx := tc.makeContext(func(_ context.Context, ev LiveEvent) { events = append(events, ev) })
			if got := HasLiveProgress(ctx); got != tc.want {
				t.Fatalf("HasLiveProgress = %v, want %v", got, tc.want)
			}
			EmitLiveProgress(ctx, LiveEvent{Kind: "step", Text: "inspect parser"})
			if !tc.want {
				if len(events) != 0 {
					t.Fatalf("non-delegated context published events: %+v", events)
				}
				return
			}
			if len(events) != 1 || events[0].Session != "boundary-session" || events[0].Agent != "coder-boundary" || events[0].Text != "inspect parser" || events[0].At.IsZero() {
				t.Fatalf("delegated observation lost ownership or content: %+v", events)
			}
		})
	}
}

func TestLiveToolSubject_IgnoresEmptyAndMalformedIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"empty optional path uses build default", map[string]any{"path": ""}, "./..."},
		{"mixed identifiers", map[string]any{"paths": []any{nil, 42, "a.go", false, "b.go"}}, "a.go, b.go"},
		{"wrong edits shape", map[string]any{"edits": "not an edit list"}, "./..."},
		{"mixed edit entries", map[string]any{"edits": []any{nil, 42, "a.go", map[string]any{"path": "b.go"}}}, "b.go"},
		{"missing or nonstring edit paths", map[string]any{"edits": []any{map[string]any{"content": "not a subject"}, map[string]any{"path": 42}, map[string]any{"path": "c.go"}}}, "c.go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := LiveToolSubject("run_build", tc.args); got != tc.want {
				t.Fatalf("subject=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestLiveAssistant_RejectsPartiallyDecodedMalformedEnvelope(t *testing.T) {
	var events []LiveEvent
	ctx := WithAgent(WithLiveProgress(context.Background(), func(_ context.Context, ev LiveEvent) { events = append(events, ev) }), "coder", "malformed")
	// Unmarshal fills Surface before reporting the Control type error. That
	// partially decoded prose must not be promoted into a valid observation.
	liveAssistant(ctx, `{"surface_response":"must not appear","control_packet":42}`)
	if len(events) != 0 {
		t.Fatalf("malformed envelope emitted partially decoded prose: %+v", events)
	}
}

func TestResidentServiceKeepsTranscriptOutsideTaskProgress(t *testing.T) {
	s, _ := installStore(t, "service-session", true)
	for _, role := range []string{"tactile_router", "session_planner"} {
		var live []LiveEvent
		ctx := WithLiveProgress(WithTask(context.Background(), "user-task"), func(_ context.Context, ev LiveEvent) { live = append(live, ev) })
		_, err := RunService(ctx, role, "service", "on_demand_activation", func(ctx context.Context) (string, error) {
			if TaskFrom(ctx) != "" {
				t.Fatal("service inherited user task")
			}
			if ScopeFrom(ctx) == nil || !ScopeFrom(ctx).Service {
				t.Fatal("missing service scope")
			}
			if tr := TranscriptFrom(ctx); tr == nil {
				t.Fatal("service lost diagnostic transcript")
			}
			if progress := s.ProgressForTask("service-session", "user-task"); len(progress) != 0 {
				t.Fatalf("service appears as running agent: %+v", progress)
			}
			EmitLiveProgress(ctx, LiveEvent{Kind: EventCallStart, Text: "waiting for model"})
			return "stopped", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(live) != 0 {
			t.Fatalf("service entered delegated live feed: %+v", live)
		}
	}
}
