package sessionlog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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
