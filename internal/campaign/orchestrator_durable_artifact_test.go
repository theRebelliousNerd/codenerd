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

func TestIsTrivialResult(t *testing.T) {
	trivial := []string{"", "   ", "ok", "done", "No output.", strings.Repeat("x", 39)}
	for _, s := range trivial {
		if !isTrivialResult(s) {
			t.Errorf("expected %q to be trivial", s)
		}
	}
	substantial := []string{
		strings.Repeat("x", 40),
		substantialFindings,
		"The scanner owns a worker pool and a FileCache that must be Closed on shutdown.",
	}
	for _, s := range substantial {
		if isTrivialResult(s) {
			t.Errorf("expected %q (len=%d) to be substantial", s, len(strings.TrimSpace(s)))
		}
	}
}

func TestIsFileProducingType(t *testing.T) {
	fileTypes := []TaskType{
		TaskTypeFileCreate, TaskTypeFileModify, TaskTypeTestWrite,
		TaskTypeTestRun, TaskTypeToolCreate, TaskTypeRefactor,
	}
	for _, tt := range fileTypes {
		if !isFileProducingType(tt) {
			t.Errorf("expected %s to be file-producing (exempt from retry/persist)", tt)
		}
	}
	analysisTypes := []TaskType{
		TaskTypeResearch, TaskTypeVerify, TaskTypeDocument, TaskTypeIntegrate,
	}
	for _, tt := range analysisTypes {
		if isFileProducingType(tt) {
			t.Errorf("expected %s to be analysis (eligible for retry/persist)", tt)
		}
	}
}

func TestLooksLikeIntentStub(t *testing.T) {
	// The actual stub captured live in run 15 phase 2 (task 2_1).
	realStub := "I'll audit `internal/world` for nil panics, bounds issues, unchecked " +
		"type assertions, missing validation, and unsafe API-boundary assumptions. " +
		"Starting with the package inventory and high-risk patterns."
	stubs := []string{
		realStub,
		"I will examine the error-handling paths next.",
		"Let me review the concurrency primitives.",
		"I'm going to audit the lifecycle of the scanner.",
		"Plan: enumerate closers, then check double-close.",
	}
	for _, s := range stubs {
		if !looksLikeIntentStub(s) {
			t.Errorf("expected intent stub: %q", s)
		}
	}

	notStubs := []string{
		// Real findings, even when terse.
		"Found a nil-deref at fs.go:212: canonicalScanPath dereferences a nil FileInfo when Stat fails.",
		"No issues found in the exported invariants; all constructors validate inputs.",
		substantialFindings,
		// A long summary that opens with a planning phrase has still done the work.
		"Let me summarize the findings. " + strings.Repeat("Concrete finding with file:line evidence. ", 20),
	}
	for _, s := range notStubs {
		if looksLikeIntentStub(s) {
			t.Errorf("expected NOT an intent stub (len=%d): %q", len(s), s)
		}
	}
}

func TestNeedsAnalysisRetry(t *testing.T) {
	retry := []string{"", "   ", "ok", "I'll audit the package for hazards, starting now."}
	for _, s := range retry {
		if !needsAnalysisRetry(s) {
			t.Errorf("expected needsAnalysisRetry for %q", s)
		}
	}
	noRetry := []string{
		substantialFindings,
		"Found a bounds bug at deep_scan.go:88 where the slice index is unchecked.",
	}
	for _, s := range noRetry {
		if needsAnalysisRetry(s) {
			t.Errorf("expected NO retry for %q", s)
		}
	}
}

func TestIsAnalyticalVerifyDescription(t *testing.T) {
	analytical := []string{
		"Inspect logic defects and invariant violations in internal/world.",
		"Assess error-handling mistakes across the scanner.",
		"Audit API contract drift between emitters and predicate sets.",
		"Review the resource ownership and lifecycle of the LSP manager.",
		"Identify race conditions in the worker pool.",
	}
	for _, d := range analytical {
		if !isAnalyticalVerifyDescription(d) {
			t.Errorf("expected analytical routing for %q", d)
		}
	}
	buildOnly := []string{
		"Verify the package compiles with go build ./...",
		"Confirm the tree builds after the change.",
		"Run go build and report the exit status.",
	}
	for _, d := range buildOnly {
		if isAnalyticalVerifyDescription(d) {
			t.Errorf("expected build path (not analytical) for %q", d)
		}
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
