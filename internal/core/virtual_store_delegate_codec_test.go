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

// TestHandleDelegateMintsSubagentHandle proves the delegate action is a live
// mint site for the subagent-return codec: a transcript above the retention
// threshold comes back as a projection carrying a redeemable handle in
// Metadata, and the handle reopens the exact transcript through the shared
// codec — the same store the subagent_expand verb reads.
func TestHandleDelegateMintsSubagentHandle(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript := "finding alpha in main.go:10\n" + strings.Repeat("audit line with no structure\n", 60)
	vs.taskDelegator = &mockActionsTaskDelegator{
		executeFunc: func(context.Context, string, string) (string, error) { return transcript, nil },
	}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		ActionID: "del_codec",
		Target:   "reviewer",
		Payload:  map[string]any{"task": "review the diff"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	handle, _ := res.Metadata["subagent_handle"].(string)
	if handle == "" {
		t.Fatal("large delegation must mint a subagent handle in Metadata")
	}
	if !strings.Contains(res.Output, handle) {
		t.Fatal("projection must name its own handle so the parent can redeem it")
	}
	if !strings.Contains(res.Output, "subagent_expand") {
		t.Fatal("projection must name the redemption verb")
	}
	if strings.Contains(res.Output, transcript) {
		t.Fatal("projection must elide the retained transcript, not paste it whole")
	}

	hydrated, err := observation.SharedSubagents().HydrateReturn(handle, observation.ReturnWindow{})
	if err != nil {
		t.Fatalf("minted handle must redeem: %v", err)
	}
	joined := strings.Join(hydrated.Lines, "\n")
	if !strings.Contains(joined, "finding alpha in main.go:10") {
		t.Fatalf("redeemed transcript lost its first line: %q", joined[:100])
	}
}

// TestHandleDelegateShortResultCarriedWhole pins what the codec wiring must
// not break for small returns: nothing is elided and no handle is minted,
// because there is nothing to elide. The return is carried whole under the
// projection header, the same shape every other delegation result has.
func TestHandleDelegateShortResultCarriedWhole(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	vs.taskDelegator = &mockActionsTaskDelegator{
		executeFunc: func(context.Context, string, string) (string, error) { return "all good", nil },
	}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		ActionID: "del_short",
		Target:   "coder",
		Payload:  map[string]any{"task": "check"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	if !res.Success || !strings.Contains(res.Output, "all good") {
		t.Fatalf("short result must be carried whole, got %+v", res)
	}
	if len(res.Metadata) != 0 {
		t.Fatalf("an unretained result must carry no handle metadata, got %v", res.Metadata)
	}
	if strings.Contains(res.Output, toolscore.SubagentExpandToolName) {
		t.Fatalf("an unretained result must not name a redemption verb:\n%s", res.Output)
	}
}

// TestHandleDelegate_FactReachesRawTranscriptThroughHandle: the kernel fact
// carries the projection (see ShouldNotPutTheTranscriptInTheKernel), so the
// raw transcript must still be reachable from it — the handle the fact names
// redeems to the exact bytes the subagent returned.
func TestHandleDelegate_FactReachesRawTranscriptThroughHandle(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript := strings.Repeat("0123456789abcdef\n", 60)
	vs.taskDelegator = &mockActionsTaskDelegator{
		executeFunc: func(context.Context, string, string) (string, error) { return transcript, nil },
	}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		ActionID: "del_fact",
		Target:   "tester",
		Payload:  map[string]any{"task": "run tests"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	handle, _ := res.Metadata["subagent_handle"].(string)
	if handle == "" {
		t.Fatal("large delegation must mint a subagent handle in Metadata")
	}
	var factText string
	for _, f := range res.FactsToAdd {
		if f.Predicate == "delegation_result" && len(f.Args) == 2 {
			factText, _ = f.Args[1].(string)
		}
	}
	if !strings.Contains(factText, handle) {
		t.Fatalf("delegation_result fact must name the handle that redeems the transcript, got %q", factText)
	}
	hydrated, err := observation.SharedSubagents().HydrateReturn(handle, observation.ReturnWindow{Limit: 200})
	if err != nil {
		t.Fatalf("handle must redeem: %v", err)
	}
	if got := strings.Join(hydrated.Lines, "\n"); got != transcript {
		t.Fatalf("redeemed transcript differs from the raw return (%d vs %d bytes)", len(got), len(transcript))
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
