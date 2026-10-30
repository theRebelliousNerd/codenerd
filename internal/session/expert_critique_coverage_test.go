package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/experts"
)

// taskRecordingConsultant keeps each agent's critique task and finds nothing.
type taskRecordingConsultant struct {
	mu    sync.Mutex
	tasks map[string]string
}

func (c *taskRecordingConsultant) SpawnConsultation(_ context.Context, agent, task string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tasks[agent] = task
	return "NO FINDINGS", nil
}

// Every critiquing expert got the same packet: the first six written files in
// first-seen order. An expert whose domain file was the seventh write was
// never shown it, nor told it was missing. Each expert's packet now leads
// with its own domain files, fills newest-written first, and names every
// written file it was not shown.
func TestExpertCritique_EachExpertIsShownItsDomainAndToldWhatItMissed(t *testing.T) {
	e := newObligationExec(t)
	ws := e.config.WorkspaceRoot
	var written []string
	for i := 1; i <= 7; i++ {
		name := fmt.Sprintf("a%d.go", i)
		if err := os.WriteFile(filepath.Join(ws, name), []byte(fmt.Sprintf("package p\n\nfunc F%d() int { return %d }\n", i, i)), 0o644); err != nil {
			t.Fatal(err)
		}
		written = append(written, name)
	}
	result := &ExecutionResult{WriteToolEffects: len(written), WrittenPaths: written}
	e.recordExpertCritics(result, experts.Selection{Needs: []experts.Need{
		{Agent: "firstexpert", Critique: true, Files: []string{"a1.go"}},
		{Agent: "lastexpert", Critique: true, Files: []string{"a7.go"}},
	}})
	consultant := &taskRecordingConsultant{tasks: map[string]string{}}
	e.SetExpertConsultant(consultant)

	if _, err := e.reviewWithShardExperts(context.Background(), nil, "", nil, nil, nil, result); err != nil {
		t.Fatalf("reviewWithShardExperts: %v", err)
	}
	last := consultant.tasks["lastexpert"]
	if !strings.Contains(last, "func F7") {
		t.Fatalf("the expert for a7.go was not shown it:\n%s", last)
	}
	first := consultant.tasks["firstexpert"]
	if !strings.Contains(first, "func F1") {
		t.Fatalf("the expert for a1.go was not shown it:\n%s", first)
	}
	// a1.go first, then newest-written: a7..a3. a2.go is past the bound.
	if !strings.Contains(first, "a2.go (past the review's 6-file bound)") || strings.Contains(first, "func F2") {
		t.Fatalf("the expert was not told which written file it was not shown:\n%s", first)
	}
	if !strings.Contains(first, "Your domain covers these files of the task: a1.go") {
		t.Fatalf("the expert was not told its domain's files:\n%s", first)
	}
}
