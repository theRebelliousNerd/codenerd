package learnjudge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/typesafe"
)

// One verdict request asks the fixed questions plus one per changed file; the
// state carries only what the record has, and bounds a long output.
func TestTaskVerdictRequest_Shape(t *testing.T) {
	state, qs := TaskVerdictRequest(TaskRecord{Request: "fix Max", ChangedFiles: []string{"a.go", "b.go"}, Output: strings.Repeat("x", taskOutputLimit+10)})
	for _, name := range []string{DidWhatWasAsked, StoppedEarly, FailureKind, FailureKindFits, "file_needed_0", "file_needed_1"} {
		if _, ok := qs[name]; !ok {
			t.Fatalf("question %s missing: %v", name, qs)
		}
	}
	if c, ok := qs[FailureKind].(typesafe.Choice); !ok || len(c.Criteria) != len(FailureKinds) {
		t.Fatalf("failure kind is not a Choice over every kind: %#v", qs[FailureKind])
	}
	result := state["result"].(map[string]any)
	if _, ok := result["build_errors"]; ok {
		t.Fatal("an empty build error list went into the state")
	}
	if n, _ := result["final_output_omitted_bytes"].(int); n != 10 {
		t.Fatalf("output omission = %v, want 10", result["final_output_omitted_bytes"])
	}
}

func TestProblemTypeAndSharedCause_FailLoudly(t *testing.T) {
	failing := &autopoiesisAsker{err: errors.New("503")}
	if _, err := JudgeProblemType(context.Background(), failing, "fix it", ""); err == nil {
		t.Fatal("a failed problem-type call must be an error")
	}
	if _, err := JudgeProblemType(context.Background(), failing, "  ", ""); err == nil {
		t.Fatal("an empty task must be refused before a call")
	}
	screenState, screenQuestions := SharedCauseRequest([]GroupFailure{{Request: "x"}})
	if _, err := JudgeSharedCause(context.Background(), failing, screenState, screenQuestions); err == nil {
		t.Fatal("a failed shared-cause call must be an error")
	}
	state, _ := SharedCauseRequest(make([]GroupFailure, groupMaxFailures+3))
	if state["more_failures_omitted"] != 3 || len(state["failures"].([]map[string]any)) != groupMaxFailures {
		t.Fatalf("group bound: %v", state["more_failures_omitted"])
	}
	_, qs := ProblemTypeRequest("fix the race", "/coder")
	if c, ok := qs[ProblemKind].(typesafe.Choice); !ok || len(c.Criteria) != len(ProblemKinds) {
		t.Fatal("problem kind is not a Choice over every kind")
	}
}

// One question per promoted atom, at most atomMaxComparisons, and the state
// says how many were left out.
func TestAtomCheckRequest_ShapeAndBound(t *testing.T) {
	existing := make([]AtomText, atomMaxComparisons+2)
	for i := range existing {
		existing[i] = AtomText{ID: "a", Content: "x"}
	}
	state, qs := AtomCheckRequest(AtomText{ID: "c", Content: "do y"}, existing)
	if len(qs) != atomMaxComparisons+1 || state["existing_atoms_omitted"] != 2 {
		t.Fatalf("questions %d omitted %v", len(qs), state["existing_atoms_omitted"])
	}
	if _, err := CheckAtom(context.Background(), &autopoiesisAsker{err: errors.New("503")}, AtomText{ID: "c", Content: "do y"}, nil); err == nil {
		t.Fatal("a failed atom check must be an error")
	}
}

// TestBounded_PinHeadTailOmission pins the head/tail bounding contract:
// short text goes whole, long text keeps head and tail with a marker and
// reports the omitted byte count, and a non-positive limit keeps everything.
func TestBounded_PinHeadTailOmission(t *testing.T) {
	if got, omitted := bounded("short", 100); got != "short" || omitted != 0 {
		t.Fatalf("short text must go whole: %q omitted %d", got, omitted)
	}
	if got, omitted := bounded("exact", 5); got != "exact" || omitted != 0 {
		t.Fatalf("text at the limit must go whole: %q omitted %d", got, omitted)
	}
	if got, omitted := bounded("keep me", 0); got != "keep me" || omitted != 0 {
		t.Fatalf("non-positive limit keeps everything: %q omitted %d", got, omitted)
	}
	long := strings.Repeat("x", 90)
	kept, omitted := bounded(long, 60)
	if omitted <= 0 {
		t.Fatalf("long text must report omission: omitted %d", omitted)
	}
	if !strings.Contains(kept, "\n[...]\n") {
		t.Fatalf("long text must carry the omission marker: %q", kept)
	}
	if len(kept) >= len(long) {
		t.Fatalf("bounded text must be shorter: kept %d from %d", len(kept), len(long))
	}
	// Multibyte runes must not split: the kept text stays valid UTF-8.
	uni := strings.Repeat("é", 60)
	keptUni, omittedUni := bounded(uni, 30)
	if omittedUni <= 0 {
		t.Fatalf("multibyte text must report omission: omitted %d", omittedUni)
	}
	if !utf8.ValidString(keptUni) {
		t.Fatalf("kept text must stay valid UTF-8: %q", keptUni)
	}
	if !strings.Contains(keptUni, "\n[...]\n") {
		t.Fatalf("multibyte text must carry the omission marker: %q", keptUni)
	}
}

// TestPutBounded_PinOmissionKey pins that the omission key appears only when
// bytes were left out.
func TestPutBounded_PinOmissionKey(t *testing.T) {
	state := map[string]any{}
	putBounded(state, "task", "short", 100)
	if state["task"] != "short" {
		t.Fatalf("short value must go whole: %v", state)
	}
	if _, ok := state["task_omitted_bytes"]; ok {
		t.Fatalf("whole value must not set an omission key: %v", state)
	}
	longState := map[string]any{}
	putBounded(longState, "task", strings.Repeat("y", 90), 60)
	if n, _ := longState["task_omitted_bytes"].(int); n <= 0 {
		t.Fatalf("cut value must say how much was left out: %v", longState)
	}
}

// TestSharedCauseRequest_PinShape pins kind lowercasing, empty-field omission,
// per-field bounding, and keeping the most recent failures when over the cap.
func TestSharedCauseRequest_PinShape(t *testing.T) {
	state, qs := SharedCauseRequest([]GroupFailure{{
		Request: "fix the timeout", Explanation: "timed out", Category: "Timeout",
		TaskID: "t1", VerdictVersion: "v1", SuggestedRule: "retry",
	}})
	if _, ok := qs[SharesOneCause]; !ok {
		t.Fatal("shares_one_cause question is missing")
	}
	items := state["failures"].([]map[string]any)
	if items[0]["kind"] != "timeout" {
		t.Fatalf("kind must be lowercased: %v", items[0]["kind"])
	}
	if items[0]["task_id"] != "t1" || items[0]["verdict_version"] != "v1" {
		t.Fatalf("ids must ride along: %v", items[0])
	}
	bare, _ := SharedCauseRequest([]GroupFailure{{Request: "x", Explanation: "y"}})
	bareItem := bare["failures"].([]map[string]any)[0]
	if _, ok := bareItem["kind"]; ok {
		t.Fatalf("empty category must leave kind out: %v", bareItem)
	}
	if _, ok := bareItem["task_id"]; ok {
		t.Fatalf("empty task id must stay out: %v", bareItem)
	}
	many := make([]GroupFailure, groupMaxFailures+3)
	for i := range many {
		many[i] = GroupFailure{Request: "r", Explanation: "e"}
	}
	capped, _ := SharedCauseRequest(many)
	if capped["more_failures_omitted"] != 3 {
		t.Fatalf("omission = %v, want 3", capped["more_failures_omitted"])
	}
	if len(capped["failures"].([]map[string]any)) != groupMaxFailures {
		t.Fatalf("kept %d, want %d", len(capped["failures"].([]map[string]any)), groupMaxFailures)
	}
}

// TestJudgeSharedCause_PinValidation pins that a bad projection never costs a
// call: nil client, empty group, and a missing question are errors.
func TestJudgeSharedCause_PinValidation(t *testing.T) {
	goodState, goodQs := SharedCauseRequest([]GroupFailure{{Request: "x", Explanation: "y"}})
	if _, err := JudgeSharedCause(context.Background(), nil, goodState, goodQs); err == nil {
		t.Fatal("nil client must be an error")
	}
	if _, err := JudgeSharedCause(context.Background(), &autopoiesisAsker{}, map[string]any{}, goodQs); err == nil {
		t.Fatal("empty failure group must be an error")
	}
	if _, err := JudgeSharedCause(context.Background(), &autopoiesisAsker{}, goodState, map[string]typesafe.Question{}); err == nil {
		t.Fatal("missing question must be an error")
	}
}

// TestToolNeedRequest_PinFailureQuestion pins that the failure question only
// exists when there was a failed attempt to ask about, and that registered
// tools ride along as name/does pairs.
func TestToolNeedRequest_PinFailureQuestion(t *testing.T) {
	plain, plainQs := ToolNeedRequest(ToolNeedInput{Request: "fix the parser"})
	if _, ok := plainQs[FailedForMissingTool]; ok {
		t.Fatal("no failed attempt means no failure question")
	}
	if _, ok := plain["failed_attempt"]; ok {
		t.Fatal("no failed attempt means no failed_attempt in state")
	}
	withFail, withFailQs := ToolNeedRequest(ToolNeedInput{
		Request:       "convert the dump",
		FailedAttempt: "tried jq and it could not read it",
		Tools:         []ExistingTool{{Name: "t1", Description: "does x"}},
	})
	if _, ok := withFailQs[FailedForMissingTool]; !ok {
		t.Fatal("a failed attempt must add the failure question")
	}
	if withFail["failed_attempt"] == nil {
		t.Fatal("a failed attempt must be in the state")
	}
	tools := withFail["generated_tools"].([]map[string]string)
	if len(tools) != 1 || tools[0]["name"] != "t1" || tools[0]["does"] != "does x" {
		t.Fatalf("tools = %v", tools)
	}
	if _, err := JudgeToolNeed(context.Background(), nil, ToolNeedInput{Request: "x"}); err == nil {
		t.Fatal("nil client must be an error")
	}
	if _, err := JudgeToolNeed(context.Background(), &autopoiesisAsker{}, ToolNeedInput{}); err == nil {
		t.Fatal("empty request must be an error before a call")
	}
}

// TestHarnessCoversRequest_PinShape pins the state: harness fact plus the
// trimmed tool name and description, with one question.
func TestHarnessCoversRequest_PinShape(t *testing.T) {
	state, qs := HarnessCoversRequest(ExistingTool{Name: "grep_plus", Description: "  finds lines  "})
	if len(qs) != 1 {
		t.Fatalf("questions = %d, want 1", len(qs))
	}
	if _, ok := qs[HarnessCoversName]; !ok {
		t.Fatal("harness_covers question is missing")
	}
	tool := state["tool"].(map[string]string)
	if tool["name"] != "grep_plus" || tool["does"] != "finds lines" {
		t.Fatalf("tool = %v", tool)
	}
	if state["harness_capabilities"] == nil {
		t.Fatal("harness capabilities must be in the state")
	}
	if _, err := JudgeHarnessCovers(context.Background(), nil, ExistingTool{Name: "t", Description: "d"}); err == nil {
		t.Fatal("nil client must be an error")
	}
	if _, err := JudgeHarnessCovers(context.Background(), &autopoiesisAsker{}, ExistingTool{Name: "t"}); err == nil {
		t.Fatal("a tool with no description must be refused before a call")
	}
}

// TestUnusedReasonRequest_PinShape pins optional signature/doc handling and
// that both questions name the function under review.
func TestUnusedReasonRequest_PinShape(t *testing.T) {
	state, qs := UnusedReasonRequest(UnusedReasonInput{
		Task: "add helper", Ref: "pkg.Foo", Signature: "func Foo()", Doc: "does foo", Reason: "used by plugin",
	})
	if len(qs) != 2 {
		t.Fatalf("questions = %d, want 2", len(qs))
	}
	fn := state["function"].(map[string]any)
	if fn["ref"] != "pkg.Foo" || fn["signature"] != "func Foo()" || fn["doc"] != "does foo" {
		t.Fatalf("function = %v", fn)
	}
	bare, _ := UnusedReasonRequest(UnusedReasonInput{Task: "t", Ref: "pkg.Bar", Reason: "r"})
	bareFn := bare["function"].(map[string]any)
	if _, ok := bareFn["signature"]; ok {
		t.Fatalf("empty signature must stay out: %v", bareFn)
	}
	if _, ok := bareFn["doc"]; ok {
		t.Fatalf("empty doc must stay out: %v", bareFn)
	}
	if _, err := JudgeUnusedReason(context.Background(), nil, UnusedReasonInput{Reason: "r"}); err == nil {
		t.Fatal("nil client must be an error")
	}
	if _, err := JudgeUnusedReason(context.Background(), &autopoiesisAsker{}, UnusedReasonInput{Reason: "  "}); err == nil {
		t.Fatal("empty reason must be refused before a call")
	}
}
