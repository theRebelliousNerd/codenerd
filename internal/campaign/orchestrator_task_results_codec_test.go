package campaign

import (
	"fmt"
	"strings"
	"testing"
)

// shardTurn is a shard's whole turn as an explicit-shard handler returns it:
// narration first, findings last. That ordering is why a head-truncated copy
// reliably carried the preamble and dropped the conclusions.
func shardTurn() (transcript, narration, finding string) {
	narration = "Plan: I will begin with internal/widget/store.go and work outward."
	finding = "Flush error discarded"

	var sb strings.Builder
	sb.WriteString(narration + "\n")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&sb, "Reading internal/widget/file%d.go; the guard clause looks right, moving on.\n", i)
	}
	fmt.Fprintf(&sb, "- [CRITICAL] internal/widget/store.go:88: %s\n", finding)
	return sb.String(), narration, finding
}

// TestProjectTaskReturn_ShouldCarryFindingsIntoTheNextTaskNotTheTranscript is
// the campaign producer. What buildTaskInput pastes into a dependent task's
// prompt used to be json.Marshal of the handler's return, cut at 1000 bytes by
// completeTask -- a head-kept, tail-dropped copy of one shard's prose.
func TestProjectTaskReturn_ShouldCarryFindingsIntoTheNextTaskNotTheTranscript(t *testing.T) {
	t.Parallel()

	transcript, narration, finding := shardTurn()
	o := &Orchestrator{}
	task := &Task{
		ID:          "task-1",
		Description: "audit internal/widget for error handling",
		Shard:       "reviewer",
		Artifacts:   []TaskArtifact{{Type: "/doc", Path: "reports/widget_audit.md"}},
	}

	out := o.projectTaskReturn(task, map[string]any{
		"shard":  "reviewer",
		"result": transcript,
		"task":   task.ID,
	})

	if strings.Contains(out, narration) {
		t.Errorf("the projected task return carried the shard's narration into the next task's prompt:\n%s", out)
	}
	if !strings.Contains(out, finding) {
		t.Errorf("the projected task return dropped the finding, which is the whole reason the next task is given context:\n%s", out)
	}
	if !strings.Contains(out, "reports/widget_audit.md") {
		t.Errorf("the projection dropped the artifact the orchestrator already knew this task produced:\n%s", out)
	}
	if !strings.Contains(out, "subagent_expand handle=") {
		t.Errorf("the projection elided the transcript without naming the verb that redeems it:\n%s", out)
	}
	if len(out) >= len(transcript)/4 {
		t.Errorf("projection is %d bytes against %d transcript; it must be bounded by structure, not by input size",
			len(out), len(transcript))
	}
}

// TestProjectTaskReturn_WhenTheHandlerReturnsSomethingElse_ShouldStillSayWhat
// pins the fallback. Task handlers return several shapes; an unrecognised one
// must not become an empty context section, which a dependent task reads as
// "the upstream task found nothing".
func TestProjectTaskReturn_WhenTheHandlerReturnsSomethingElse_ShouldStillSayWhat(t *testing.T) {
	t.Parallel()

	o := &Orchestrator{}
	task := &Task{ID: "task-2", Description: "run the suite", Type: TaskTypeTestRun}

	out := o.projectTaskReturn(task, map[string]any{"passed": 12, "failed": 0})
	if strings.TrimSpace(out) == "" {
		t.Fatal("an unrecognised handler return produced an empty context section")
	}
	if !strings.Contains(out, "passed") {
		t.Errorf("the encoded fallback lost the handler's own fields:\n%s", out)
	}
}

// TestProjectTaskReturn_ShouldNotClaimArtifactsTheTaskDidNotRecord keeps the
// campaign producer honest about the one field a dependent task acts on.
func TestProjectTaskReturn_ShouldNotClaimArtifactsTheTaskDidNotRecord(t *testing.T) {
	t.Parallel()

	transcript, _, _ := shardTurn()
	o := &Orchestrator{}
	out := o.projectTaskReturn(&Task{ID: "task-3", Description: "audit"},
		map[string]any{"shard": "reviewer", "result": transcript + "\nI also created reports/extra.md.\n"})

	if strings.Contains(out, "reports/extra.md") {
		t.Errorf("the projection reported an artifact only the shard's prose claimed:\n%s", out)
	}
}
