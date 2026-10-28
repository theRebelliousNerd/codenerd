package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// recordingMeter is the ledger as the executor sees it.
type recordingMeter struct {
	mu            sync.Mutex
	offers        []types.ToolOffer
	calls         []types.ToolCallMeasure
	verifications []types.ToolVerification
}

func (m *recordingMeter) NoteToolOffers(o []types.ToolOffer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.offers = append(m.offers, o...)
}
func (m *recordingMeter) NoteToolCall(c types.ToolCallMeasure) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, c)
}
func (m *recordingMeter) NoteToolVerification(v types.ToolVerification) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verifications = append(m.verifications, v)
}

func meteredExecutor(m types.ToolMeter) *Executor {
	e := &Executor{config: DefaultExecutorConfig(), sessionID: "session-under-test"}
	e.config.EnableSafetyGate = false
	e.SetToolMeter(m)
	return e
}

func scanningTool(t *testing.T, scanBytes, scanItems int64, report bool, out string, fail error) string {
	t.Helper()
	name := fmt.Sprintf("meter_probe_%d", capabilityTestToolCounter.Add(1))
	registerTestTool(t, &tools.Tool{
		Effect: tools.EffectRead, Name: name, Description: "a probe", Capability: tools.CapabilityOverlapping,
		Execute: func(ctx context.Context, _ map[string]any) (string, error) {
			if report {
				tools.ReportConsumed(ctx, scanBytes, scanItems)
			}
			return out, fail
		},
	})
	return name
}

func TestToolMeter_CallRecordsWhatTheToolReportedScanning(t *testing.T) {
	name := scanningTool(t, 90_000, 12, true, strings.Repeat("x", 400), nil)
	meter := &recordingMeter{}
	e := meteredExecutor(meter)
	if _, err := e.executeAndRecordToolCall(context.Background(),
		types.ToolCall{ID: "1", Name: name, Input: map[string]any{"pattern": "needle"}},
		validCapabilityTestConfig(name), &ExecutionResult{}); err != nil {
		t.Fatal(err)
	}
	if len(meter.calls) != 1 {
		t.Fatalf("meter saw %d calls, want 1", len(meter.calls))
	}
	c := meter.calls[0]
	if !c.Consumed.Known || c.Consumed.Bytes != 90_000 || c.Consumed.Items != 12 {
		t.Errorf("consumption %+v, want the tool's reported 90000 bytes over 12 items", c.Consumed)
	}
	if c.ResultBytes != 400 || c.ResultTokens <= 0 || c.CallTokens <= 0 {
		t.Errorf("result bytes=%d tokens=%d call tokens=%d", c.ResultBytes, c.ResultTokens, c.CallTokens)
	}
	if c.Channel != types.ToolChannelNative || c.Origin != types.ToolOriginBuiltin || !c.Success || c.SessionID != "session-under-test" {
		t.Errorf("measure %+v", c)
	}
}

func TestToolMeter_AToolThatReportsNothingIsUnknownNotZero(t *testing.T) {
	name := scanningTool(t, 0, 0, false, "result", nil)
	meter := &recordingMeter{}
	e := meteredExecutor(meter)
	if _, err := e.executeAndRecordToolCall(context.Background(), types.ToolCall{ID: "1", Name: name},
		validCapabilityTestConfig(name), &ExecutionResult{}); err != nil {
		t.Fatal(err)
	}
	if got := meter.calls[0].Consumed; got.Known {
		t.Errorf("a tool that reported no scan was recorded as %+v", got)
	}
}

func TestToolMeter_AFailedCallCostsTheErrorTextAndIsNotASuccess(t *testing.T) {
	name := scanningTool(t, 0, 0, false, "partial output", errors.New("exit status 1"))
	meter := &recordingMeter{}
	e := meteredExecutor(meter)
	_, _ = e.executeAndRecordToolCall(context.Background(), types.ToolCall{ID: "1", Name: name},
		validCapabilityTestConfig(name), &ExecutionResult{})
	c := meter.calls[0]
	if c.Success {
		t.Error("a failed call was recorded as a success")
	}
	if c.ResultBytes < int64(len("partial output")+len("exit status 1")) {
		t.Errorf("result bytes %d: the model is handed the error text with the output, and pays for both", c.ResultBytes)
	}
}

func TestToolMeter_NoMeterRecordsNothingAndFailsNothing(t *testing.T) {
	name := scanningTool(t, 10, 1, true, "ok", nil)
	e := &Executor{config: DefaultExecutorConfig()}
	e.config.EnableSafetyGate = false
	if _, err := e.executeAndRecordToolCall(context.Background(), types.ToolCall{ID: "1", Name: name},
		validCapabilityTestConfig(name), &ExecutionResult{}); err != nil {
		t.Fatalf("an unmetered executor failed the call: %v", err)
	}
}

type piggybackOnlyClient struct{ types.LLMClient }

func (piggybackOnlyClient) ShouldUsePiggybackTools() bool { return true }

func TestToolMeter_ChannelIsTheOneTheCallArrivedOn(t *testing.T) {
	native := &Executor{}
	if ch, _ := native.toolChannelOf("read_file", &ExecutionResult{}); ch != types.ToolChannelNative {
		t.Errorf("native client call recorded as %q", ch)
	}
	piggyback := &Executor{llmClient: piggybackOnlyClient{}}
	if ch, _ := piggyback.toolChannelOf("read_file", &ExecutionResult{}); ch != types.ToolChannelPiggyback {
		t.Errorf("piggyback client call recorded as %q", ch)
	}
	// An MCP verb is MCP whichever way the model reached it.
	if ch, _ := piggyback.toolChannelOf("mcp_call", &ExecutionResult{}); ch != types.ToolChannelMCP {
		t.Errorf("mcp_call recorded as %q", ch)
	}
}

func TestToolMeter_VerificationIsChargedToTheWriteThatNeededIt(t *testing.T) {
	meter := &recordingMeter{}
	e := meteredExecutor(meter)
	result := &ExecutionResult{}
	// A verification with no write before it is nobody's cost.
	e.noteToolCall(types.ToolCall{Name: "run_build"}, "ok", nil, tools.Consumption{}, false, time.Now(), false, result)
	if len(meter.verifications) != 0 {
		t.Fatalf("verification charged with no write before it: %v", meter.verifications)
	}
	e.noteToolCall(types.ToolCall{Name: "edit_file", Input: map[string]any{"path": "a.go"}}, "edited", nil, tools.Consumption{}, false, time.Now(), false, result)
	e.noteToolCall(types.ToolCall{Name: "run_build"}, strings.Repeat("diag ", 40), nil, tools.Consumption{}, false, time.Now(), false, result)
	e.noteToolCall(types.ToolCall{Name: "run_command", Input: map[string]any{"command": "go test ./..."}}, "FAIL", errors.New("exit 1"), tools.Consumption{}, false, time.Now(), true, result)
	if len(meter.verifications) != 2 {
		t.Fatalf("verifications %v, want the build and the test run", meter.verifications)
	}
	for _, v := range meter.verifications {
		if v.Tool != "edit_file" || v.Tokens <= 0 {
			t.Errorf("verification %+v, want it charged to edit_file", v)
		}
	}
	// A failed write made nothing that needs verifying.
	other := &ExecutionResult{}
	e.noteToolCall(types.ToolCall{Name: "write_file"}, "denied", errors.New("denied"), tools.Consumption{}, false, time.Now(), false, other)
	if other.lastWriteTool != "" {
		t.Errorf("a failed write became the tool verification is charged to: %q", other.lastWriteTool)
	}
}

func TestToolMeter_OfferChargesCatalogAndASharedAtomSplitEvenly(t *testing.T) {
	a := fmt.Sprintf("meter_offer_a_%d", capabilityTestToolCounter.Add(1))
	b := fmt.Sprintf("meter_offer_b_%d", capabilityTestToolCounter.Add(1))
	for _, n := range []string{a, b} {
		registerTestTool(t, &tools.Tool{Effect: tools.EffectRead, Name: n, Description: "does a thing for " + n,
			Execute: func(context.Context, map[string]any) (string, error) { return "", nil }})
	}
	meter := &recordingMeter{}
	e := meteredExecutor(meter)
	compiled := &prompt.CompilationResult{IncludedAtoms: []*prompt.PromptAtom{
		{ID: "teaches-both", TokenCount: 101, RequiresTools: []string{a, b, "not_offered"}},
		{ID: "teaches-a", TokenCount: 10, RequiresTools: []string{a}},
		{ID: "teaches-nothing-offered", TokenCount: 999, RequiresTools: []string{"not_offered"}},
		{ID: "teaches-all", TokenCount: 500},
	}}
	e.noteToolOffers(compiled, validCapabilityTestConfig(a, b))
	got := map[string]types.ToolOffer{}
	for _, o := range meter.offers {
		got[o.Tool] = o
	}
	if len(got) != 2 {
		t.Fatalf("offers for %v, want exactly the two offered tools", got)
	}
	if sum := got[a].AtomTokens + got[b].AtomTokens; sum != 101+10 {
		t.Errorf("atom tokens %d+%d: an atom's tokens are split among the tools it names and never exceed it (want %d)",
			got[a].AtomTokens, got[b].AtomTokens, 111)
	}
	if got[a].AtomTokens != 51+10 || got[b].AtomTokens != 50 {
		t.Errorf("a=%d b=%d, want 61 and 50 (the odd token goes to the first)", got[a].AtomTokens, got[b].AtomTokens)
	}
	for _, o := range got {
		if o.CatalogTokens <= 0 || o.SessionID != "session-under-test" || o.Origin != types.ToolOriginBuiltin {
			t.Errorf("offer %+v", o)
		}
	}
}
