package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// substantialFindings is a realistic-length analysis result that an audit/research
// task would return — long enough to clear the persistence threshold.
const substantialFindings = `## Audit: internal/session

Reviewed the executor loop and lifecycle handling.

1. context cancellation is propagated to spawned shards.
2. the maintenance goroutine is bounded by an 8s close deadline.
3. no unbounded goroutine growth observed under normal operation.`

func newArtifactTestOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	return &Orchestrator{
		workspace: t.TempDir(),
		campaign:  &Campaign{ID: "/campaign_abc123"},
	}
}

// countArtifactsOfType returns how many of the task's artifacts have the given type.
func countArtifactsOfType(task *Task, typ string) int {
	n := 0
	for _, a := range task.Artifacts {
		if a.Type == typ {
			n++
		}
	}
	return n
}

func TestPersistTaskOutputArtifact_ResearchPersistsDoc(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	// Mirrors run 12: an audit task whose only artifact is the /source_file INPUT
	// (the material being audited) and no durable output.
	task := &Task{
		ID:          "/task_abc_0_1",
		PhaseID:     "/phase_abc_0",
		Type:        TaskTypeResearch,
		Description: "Audit internal/session for correctness and resource-lifecycle issues.",
		Artifacts:   []TaskArtifact{{Type: "/source_file", Path: "internal/session"}},
	}

	o.persistTaskOutputArtifact(task, substantialFindings)

	if got := countArtifactsOfType(task, "/doc"); got != 1 {
		t.Fatalf("expected exactly 1 /doc output artifact, got %d (artifacts=%+v)", got, task.Artifacts)
	}
	// The /source_file input must be preserved, not replaced.
	if got := countArtifactsOfType(task, "/source_file"); got != 1 {
		t.Fatalf("expected /source_file input artifact preserved, got %d", got)
	}

	var docPath string
	for _, a := range task.Artifacts {
		if a.Type == "/doc" {
			docPath = a.Path
		}
	}
	full := filepath.Join(o.workspace, docPath)
	body, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("durable artifact not written to disk: %v", err)
	}
	if !strings.Contains(string(body), "context cancellation is propagated") {
		t.Fatalf("durable artifact missing the findings body: %q", string(body))
	}
	// It must live under the campaign artifact dir, not the audited source tree.
	if !strings.Contains(filepath.ToSlash(docPath), ".nerd/campaigns/") {
		t.Fatalf("artifact should live under .nerd/campaigns/, got %q", docPath)
	}
}

func TestPersistTaskOutputArtifact_SkipsFileTasks(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	task := &Task{
		ID:          "/task_abc_1_1",
		Type:        TaskTypeFileCreate,
		Description: "Create internal/foo/bar.go",
		Artifacts:   []TaskArtifact{{Type: "/source_file", Path: "internal/foo/bar.go"}},
	}

	o.persistTaskOutputArtifact(task, substantialFindings)

	if got := countArtifactsOfType(task, "/doc"); got != 0 {
		t.Fatalf("file-producing task must not get a /doc artifact, got %d", got)
	}
}

func TestPersistTaskOutputArtifact_SkipsTrivialResult(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	task := &Task{
		ID:   "/task_abc_2_1",
		Type: TaskTypeResearch,
	}

	o.persistTaskOutputArtifact(task, "ok")

	if len(task.Artifacts) != 0 {
		t.Fatalf("trivial result must not persist an artifact, got %+v", task.Artifacts)
	}
}

func TestPersistTaskOutputArtifact_SkipsWhenDurableOutputExists(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	// Simulate a task that already wrote a durable /doc output.
	existingRel := filepath.ToSlash(filepath.Join(".nerd", "campaigns", "existing", "report.md"))
	existingFull := filepath.Join(o.workspace, existingRel)
	if err := os.MkdirAll(filepath.Dir(existingFull), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existingFull, []byte("already durable"), 0o644); err != nil {
		t.Fatal(err)
	}
	task := &Task{
		ID:          "/task_abc_3_1",
		Type:        TaskTypeDocument,
		Description: "Produce the ranked risk report.",
		Artifacts:   []TaskArtifact{{Type: "/doc", Path: existingRel}},
	}

	o.persistTaskOutputArtifact(task, substantialFindings)

	if got := countArtifactsOfType(task, "/doc"); got != 1 {
		t.Fatalf("must not duplicate an existing durable /doc artifact, got %d", got)
	}
}
