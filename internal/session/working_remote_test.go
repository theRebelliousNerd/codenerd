package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Finding one of Docs/audits/research-observation-identity-and-admission.md:
// a web result was filed under the focus file at that file's digest, so
// editing the file staled the fetched page. It is now a non-file
// observation whose revision the edit does not touch.
func TestRecordWorkingResult_ARemoteResultDoesNotTakeTheFocusFilesRevision(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	client := filepath.Join(e.config.WorkspaceRoot, "client.go")
	if err := os.WriteFile(client, []byte("package client\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), nil, "fix client.go", &prompt.CompilationContext{ShardID: "probe", IntentTarget: "client.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)
	loop := activeWorkingLoop(ctx)

	read := types.ToolCall{ID: "r", Name: "read_file", Input: map[string]any{"path": "client.go"}}
	if err := e.recordWorkingResult(ctx, read, "package client", nil); err != nil {
		t.Fatal(err)
	}
	fetch := types.ToolCall{ID: "f", Name: "web_fetch", Input: map[string]any{"url": "https://api.example.test/docs"}}
	if err := e.recordWorkingResult(ctx, fetch, "# API docs", nil); err != nil {
		t.Fatal(err)
	}
	records, err := loop.set.Observations(ctx, []string{loop.observations["r"], loop.observations["f"]})
	if err != nil || len(records) != 2 {
		t.Fatalf("observations: %v %v", records, err)
	}
	byID := map[string]int{records[0].ID: 0, records[1].ID: 1}
	readRec, fetchRec := records[byID[loop.observations["r"]]], records[byID[loop.observations["f"]]]
	if readRec.Entity != "client.go" {
		t.Fatalf("the file read is filed under %q, want client.go", readRec.Entity)
	}
	if fetchRec.Entity != "" || fetchRec.Revision == readRec.Revision {
		t.Fatalf("the fetched page took the focus file's identity: entity %q revision %q", fetchRec.Entity, fetchRec.Revision)
	}
	if err := os.WriteFile(client, []byte("package client // edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if now := loop.set.Revision(fetchRec.Entity); now != fetchRec.Revision {
		t.Fatalf("editing client.go moved the fetched page's revision %q -> %q", fetchRec.Revision, now)
	}
	if loop.focus != "client.go" {
		t.Fatalf("a remote fetch moved the focus to %q", loop.focus)
	}
}
