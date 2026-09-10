package core

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/observation"
	toolscore "codenerd/internal/tools/core"
)

// delegateFixtureSeq keeps each fixture's transcript unique across runs, so a
// content-addressed handle from an earlier pass of the same test in the same
// process cannot stand in for one minted by the code under test.
var delegateFixtureSeq atomic.Int64

// longSubagentReturn is a delegated agent's whole turn: narration first,
// findings last, which is the ordering that makes a head-truncated copy carry
// the preamble and drop the conclusions.
func longSubagentReturn() (transcript, narration, finding string) {
	n := delegateFixtureSeq.Add(1)
	narration = fmt.Sprintf("Plan: I will start with internal/widget/store_%d.go and work outward.", n)
	finding = fmt.Sprintf("Flush error discarded in store_%d", n)

	var sb strings.Builder
	sb.WriteString(narration + "\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&sb, "Reading internal/widget/file%d_%d.go; the guard clause looks right.\n", i, n)
	}
	fmt.Fprintf(&sb, "- [CRITICAL] internal/widget/store_%d.go:88: %s\n", n, finding)
	return sb.String(), narration, finding
}

// observingDelegator reports the structured return the JIT executor produces,
// standing in for session.JITExecutor through core.ObservedTaskDelegator.
type observingDelegator struct {
	ret observation.Return
}

func (d *observingDelegator) Execute(ctx context.Context, intent, task string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return d.ret.Output, nil
}

func (d *observingDelegator) ExecuteObserved(ctx context.Context, intent, task string) (observation.Return, error) {
	if err := ctx.Err(); err != nil {
		return observation.Return{}, err
	}
	return d.ret, nil
}

// TestHandleDelegate_ShouldProjectTheReturnRatherThanForwardTheTranscript is
// the producer wiring this codec exists for. A delegation hands back another
// agent's entire turn, and it lands in the parent's tool loop where the parent
// re-reads it on every subsequent round of that same loop.
func TestHandleDelegate_ShouldProjectTheReturnRatherThanForwardTheTranscript(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript, narration, finding := longSubagentReturn()

	vs.taskDelegator = &observingDelegator{ret: observation.Return{
		Output:  transcript,
		Changed: []string{"internal/widget/store.go"},
		Build:   &observation.Verification{Ran: true, OK: true},
	}}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		Target:  "reviewer",
		Payload: map[string]any{"task": "review internal/widget"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	if !res.Success {
		t.Fatalf("delegation failed: %+v", res)
	}

	if strings.Contains(res.Output, narration) {
		t.Errorf("the delegation forwarded the subagent's narration; that is the transcript, which belongs behind the handle:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, finding) {
		t.Errorf("the projection dropped the critical finding:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, "internal/widget/store.go") {
		t.Errorf("the projection dropped the write set the executor measured:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, "build passed [observed]") {
		t.Errorf("the projection dropped the observed build verdict:\n%s", res.Output)
	}
	if len(res.Output) >= len(transcript)/2 {
		t.Errorf("projection is %d bytes against %d transcript; it must be a fraction of it, not a rounding",
			len(res.Output), len(transcript))
	}
}

// TestHandleDelegate_ShouldPublishAHandleTheExpandVerbRedeems is the end-to-end
// half. A handle the model cannot redeem is a promise it is structurally unable
// to keep, so the elision above is only defensible if this passes.
func TestHandleDelegate_ShouldPublishAHandleTheExpandVerbRedeems(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript, narration, _ := longSubagentReturn()
	vs.taskDelegator = &observingDelegator{ret: observation.Return{Output: transcript}}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		Target:  "reviewer",
		Payload: map[string]any{"task": "review internal/widget"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}

	if !strings.Contains(res.Output, toolscore.SubagentExpandToolName+" handle=") {
		t.Fatalf("the delegation result names no redemption verb:\n%s", res.Output)
	}
	handle := delegateHandleFrom(t, res.Output)

	expanded, err := toolscore.SubagentExpandTool().Execute(context.Background(),
		map[string]any{"handle": handle})
	if err != nil {
		t.Fatalf("%s on the published handle: %v", toolscore.SubagentExpandToolName, err)
	}
	if !strings.Contains(expanded, narration) {
		t.Errorf("the published handle did not redeem to the elided transcript:\n%s", expanded)
	}
}

// TestHandleDelegate_WhenDelegatorReportsNoStructure_ShouldStillSucceed pins
// the fallback. A delegator that cannot answer ExecuteObserved is not broken,
// and turning "this executor keeps no write set" into "the delegation failed"
// would be a much worse answer for the parent to act on.
func TestHandleDelegate_WhenDelegatorReportsNoStructure_ShouldStillSucceed(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	vs.taskDelegator = &mockActionsTaskDelegator{}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		Target:  "coder",
		Payload: map[string]any{"task": "refactor logic"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "mock delegation success") {
		t.Errorf("a plain delegator's return did not survive projection: %+v", res)
	}
	if strings.Contains(res.Output, "changed:") {
		t.Errorf("the projection invented a write set for a delegator that reported none:\n%s", res.Output)
	}
}

// TestHandleDelegate_ShouldNotPutTheTranscriptInTheKernel. delegation_result is
// reachable by every injectable-context query for the rest of the session, so a
// fact holding a whole transcript is that transcript in the prompt repeatedly.
func TestHandleDelegate_ShouldNotPutTheTranscriptInTheKernel(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript, narration, _ := longSubagentReturn()
	vs.taskDelegator = &observingDelegator{ret: observation.Return{Output: transcript}}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		Target:  "reviewer",
		Payload: map[string]any{"task": "review internal/widget"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}

	var found bool
	for _, f := range res.FactsToAdd {
		if f.Predicate != "delegation_result" {
			continue
		}
		found = true
		for _, arg := range f.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, narration) {
				t.Errorf("delegation_result carries the transcript into the kernel:\n%s", s)
			}
		}
	}
	if !found {
		t.Error("no delegation_result fact was asserted, so the kernel cannot see the delegation happened")
	}
}

// delegateHandleFrom pulls the handle out of a rendered delegation result.
func delegateHandleFrom(t *testing.T, out string) string {
	t.Helper()
	const marker = "retained as "
	i := strings.Index(out, marker)
	if i < 0 {
		t.Fatalf("no handle in delegation result:\n%s", out)
	}
	rest := out[i+len(marker):]
	if j := strings.IndexAny(rest, " \n"); j >= 0 {
		rest = rest[:j]
	}
	if rest == "" {
		t.Fatalf("empty handle in delegation result:\n%s", out)
	}
	return rest
}
