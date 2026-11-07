package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/turntime"
	"codenerd/internal/types"
)

// A turn that writes a broken file, is sent back by the build gate, repairs
// it and passes must say where its time went: model calls through the real
// broker wrapper, the two writes as tool calls, the gate's go build runs as
// verification, and one build repair round. The phases sum to the total.
func TestTurnTiming_BuildRepairTurnReportsItsLedger(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, nil)
	meter := broker.NewMeter(broker.MeterConfig{Window: 1_000_000, OutputReserve: 8000, ReceiptBuffer: 64, PrimaryModel: "fake-model"})
	cfg := meter.ConfigFor(broker.ProviderCreds{Provider: "fake", Model: "fake-model"})
	metered, err := broker.Wrap(h.executor.llmClient, cfg)
	if err != nil {
		t.Fatalf("broker.Wrap: %v", err)
	}
	h.executor.llmClient = metered

	if err := os.WriteFile(filepath.Join(h.ws, "main.go"), []byte(repairMainGo), 0o600); err != nil {
		t.Fatal(err)
	}
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainTypeBroken),
		}}
	}
	h.onRepair = func(int, []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "fixed", ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", filepath.Join(h.ws, "main.go"), repairMainTypeFixed),
		}}
	}
	result, err := h.drive(t, "fix make the package build")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}

	s := result.Timing
	if !s.Recorded() {
		t.Fatal("the settled turn carries no time ledger")
	}
	if sum := s.Model + s.Tool + s.Other + s.VerifyTime(); sum != s.Total {
		t.Errorf("phases sum to %s, total is %s: a phase was counted twice or dropped", sum, s.Total)
	}
	if s.ModelCalls < 2 {
		t.Errorf("model calls = %d, want the opening call and the repair call through the broker", s.ModelCalls)
	}
	if s.ToolCalls < 2 {
		t.Errorf("tool calls = %d, want the write and the repair's write", s.ToolCalls)
	}
	var build turntime.Kind
	for _, k := range s.Verify {
		if k.Kind == turntime.KindBuild {
			build = k
		}
	}
	if build.Count < 2 || build.Time <= 0 {
		t.Errorf("build verification = %+v, want the failing run and the recheck", build)
	}
	// The gates after the build (pinning) may owe rounds of their own.
	var buildRounds int
	for _, k := range s.Repair {
		if k.Kind == "build" {
			buildRounds = k.Count
		}
	}
	if buildRounds != 1 {
		t.Errorf("repair = %+v, want exactly one build round", s.Repair)
	}
	if s.RepairTime() > s.Model+s.Tool {
		t.Errorf("repair time %s exceeds model+tool %s: the round's own time is a subset of them", s.RepairTime(), s.Model+s.Tool)
	}

	// The ledger reaches the parent the way every other piece of evidence does.
	if got := observedReturn("coder", "task", result).Timing; got == nil || got.Total != s.Total {
		t.Errorf("observed return timing = %+v, want the turn's split", got)
	}
	for _, want := range []string{"secs_total=", "secs_verify=", "repair_rounds=", "repair_build=", "verify_build="} {
		line := turnCost{sessionID: "s", turnNumber: 1, outcome: result.TurnOutcome, timing: s}.logLine()
		if !strings.Contains(line, want) {
			t.Errorf("turn_cost line %q lacks %q", line, want)
		}
	}
}

// A verification subprocess is verification whichever gate started it, named
// by its command or by the caller that knows what it is for.
func TestVerificationKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, tc := range []struct {
		ctx  context.Context
		name string
		args []string
		want string
	}{
		{ctx, "go", []string{"build", "./..."}, turntime.KindBuild},
		{ctx, "go", []string{"vet", "./a"}, turntime.KindVet},
		{ctx, "go", []string{"test", "-json", "./a"}, turntime.KindTests},
		{ctx, "go", []string{"test", "-json", "-covermode=set", "-coverprofile=/tmp/x", "./a"}, turntime.KindCoverage},
		{ctx, "go", []string{"list", "./..."}, turntime.KindOther},
		{turntime.WithVerifyKind(ctx, turntime.KindImporter), "go", []string{"test", "./a"}, turntime.KindImporter},
		{turntime.WithVerifyKind(ctx, turntime.KindFinal), "go", []string{"test", "./..."}, turntime.KindFinal},
		{turntime.WithVerifyKind(ctx, turntime.KindPin), "go", []string{"test", "-run", "X"}, turntime.KindPin},
	} {
		if got := verificationKind(tc.ctx, tc.name, tc.args); got != tc.want {
			t.Errorf("verificationKind(%v %v) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}

// Every verification subprocess goes through runVerificationCommand, which
// opens the ledger span: a known run time is the verification time.
func TestRunVerificationCommandIsTimedOnTheTurnLedger(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	now := time.Unix(0, 0)
	clock := func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	ledger := turntime.NewWithClock(clock)
	ctx := turntime.WithLedger(context.Background(), ledger)

	runner := func(_ context.Context, _ string, _ []string, _ string, _ []string) ([]byte, error) {
		mu.Lock()
		now = now.Add(42 * time.Second)
		mu.Unlock()
		return nil, nil
	}
	if _, outcome, _ := runVerificationCommand(turntime.WithVerifyKind(ctx, turntime.KindImporter), t.TempDir(), nil, time.Minute, "go", []string{"test", "./a"}, runner); outcome != VerifyPassed {
		t.Fatalf("outcome = %s", outcome)
	}
	s := ledger.Snapshot()
	if len(s.Verify) != 1 || s.Verify[0].Kind != turntime.KindImporter || s.Verify[0].Time != 42*time.Second || s.Verify[0].Count != 1 {
		t.Fatalf("verify = %+v, want one importer run of 42s", s.Verify)
	}
	if s.Other != 0 || s.Total != 42*time.Second {
		t.Errorf("other = %s total = %s, want 0 and 42s", s.Other, s.Total)
	}
}
