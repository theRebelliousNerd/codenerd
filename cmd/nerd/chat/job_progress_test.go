package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// While a background job runs, the main agent's context carries what its
// agents have done so far, read from the transcripts the harness writes, so
// "how is it going?" is answered from the job's progress. Only the main agent
// is told.
func TestJobProgress_ReachesTheMainAgentsContext(t *testing.T) {
	sessionlog.Install(sessionlog.NewStore(sessionlog.Options{Workspace: t.TempDir(), Session: "session-jp", Enabled: func() bool { return false }}))
	t.Cleanup(func() { sessionlog.Install(nil) })

	m := admissionModel(t, config.WorkModePropose, nil)
	m.sessionID = "session-jp"
	job := backgroundJob{ID: "bg-progress", Shard: "coder", Task: "fix the loader", Session: "session-jp", Started: time.Now()}
	m.background.add(job, func() {})
	m.assertBackgroundJob(job, "/running")

	reached, release := make(chan struct{}), make(chan struct{})
	go func() {
		ctx := sessionlog.WithTask(usage.WithSessionID(context.Background(), "session-jp"), job.ID)
		_, _ = sessionlog.RunAgent(ctx, "coder", "9", job.Task, func(ctx context.Context) (string, error) {
			tr := sessionlog.TranscriptFrom(ctx)
			tr.ToolResult(ctx, "read_file", "c1", "package prompt", false)
			tr.CallStarted(ctx, "claude-opus-5-5")
			close(reached)
			<-release
			return "", nil
		})
	}()
	<-reached
	defer close(release)

	m.refreshJobProgress(time.Now())
	rows, err := m.kernel.Query("injectable_context")
	if err != nil {
		t.Fatal(err)
	}
	var line string
	for _, r := range rows {
		text := types.ExtractString(r.Args[1])
		if strings.Contains(text, "Progress of the background coder job") {
			if target := types.ExtractString(r.Args[0]); target != chatShardID {
				t.Fatalf("job progress went to %q, want only %q", target, chatShardID)
			}
			line = text
		}
	}
	for _, want := range []string{"coder-9", "1 tool result(s)", "last tool read_file", "waiting on claude-opus-5-5"} {
		if !strings.Contains(line, want) {
			t.Errorf("progress line %q lacks %q", line, want)
		}
	}
}
