package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

func missionPolicyFixture(t *testing.T) (*RecurseMission, context.Context) {
	t.Helper()
	root := t.TempDir()
	k, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	m := &RecurseMission{Text: "Preserve unrelated user edits", Limits: config.DefaultMissionConfig(),
		Deliberator: &deliberation.Service{Workspace: root, Session: "mission-policy", Kernel: k}}
	ctx, cancel, err := m.begin(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	return m, ctx
}

func TestMissionPolicyBoundsAndUnknownSpend(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		change       func(*RecurseMission)
	}{
		{"cycles", "/cycles", func(m *RecurseMission) { m.State.Cycles = m.Limits.MaxCycles }},
		{"time", "/time", func(m *RecurseMission) { m.started = time.Now().Add(-time.Duration(m.Limits.MaxSeconds) * time.Second) }},
		{"spend", "/spend", func(m *RecurseMission) { m.meter.spend = m.Limits.MaxSpendMicroUSD }},
		{"unknown", "/unknown_spend", func(m *RecurseMission) { m.meter.phaseEnded(0) }},
		{"stalls", "/no_progress", func(m *RecurseMission) { m.State.Stalls = m.Limits.NoProgressCycles }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := missionPolicyFixture(t)
			tc.change(m)
			if _, err := m.admit(); err != errMissionStopped {
				t.Fatalf("admission=%v", err)
			}
			if m.State.Status != tc.reason || len(m.State.StopFacts) != 1 {
				t.Fatalf("state=%+v", m.State)
			}
			b, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			var saved MissionState
			if err := json.Unmarshal(b, &saved); err != nil || saved.Status != tc.reason {
				t.Fatalf("durable stop=%s %v", b, err)
			}
		})
	}
}

func TestMissionScopeUsesLiveKernelAdmission(t *testing.T) {
	m, ctx := missionPolicyFixture(t)
	k := m.Deliberator.Kernel
	if err := k.Assert(types.Fact{Predicate: "mission_design", Args: []any{m.State.ID, types.MangleAtom("/ready")}}); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(types.Fact{Predicate: "mission_source", Args: []any{m.State.ID, "source", "source", "vision", "vision"}}); err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{workspace: m.Deliberator.Workspace, kernel: k,
		campaign: &Campaign{Mission: &MissionReceipt{MissionID: m.State.ID, Scope: []string{"pkg"}, ProtectedPaths: []string{"pkg/oracle.go"}, VerificationInputs: map[string]string{}}}}
	for _, tc := range []struct {
		path    string
		allowed bool
	}{
		{"pkg/code.go", true}, {"pkg/new.go", true}, {"pkg/oracle.go", false}, {"pkg2/code.go", false}, {"../escape.go", false},
	} {
		err := o.checkMissionWrite(ctx, []string{tc.path})
		if (err == nil) != tc.allowed {
			t.Errorf("%s allowed=%v: %v", tc.path, tc.allowed, err)
		}
	}
	m.State.Cycles = m.Limits.MaxCycles
	_, _ = m.admit()
	if err := o.checkMissionWrite(ctx, []string{"pkg/code.go"}); err == nil {
		t.Fatal("spent envelope still authorizes writes")
	}
}

func TestMissionEvidenceCannotTurnMetricsIntoRequirementSupport(t *testing.T) {
	for _, tc := range []struct{ outcome, witness, before, after, want string }{
		{outcomeKept, "/resolved", "old", "new", "/supported_gate_deficit"},
		{outcomeKept, "/unknown", "old", "new", "/unknown"},
		{outcomeReverted, "/resolved", "old", "new", "/unknown"},
		{outcomeKept, "/resolved", "same", "same", "/unknown"},
	} {
		m, ctx := missionPolicyFixture(t)
		m.State.Pending = &MissionReceipt{MissionID: m.State.ID, Cycle: 1, Before: tc.before, Evidence: "unknown"}
		if err := m.finish(ctx, recurseRecord{Cycle: 1, Outcome: tc.outcome}, tc.after, tc.witness); err != nil {
			t.Fatal(err)
		}
		if got := m.State.Receipts[0].Evidence; got != tc.want {
			t.Fatalf("evidence=%s want=%s", got, tc.want)
		}
	}
}

func TestMissionInterruptedIntentCannotBeReplayed(t *testing.T) {
	m, ctx := missionPolicyFixture(t)
	m.State.Pending = &MissionReceipt{Cycle: 9, Outcome: "executing"}
	if err := m.end(ctx, errors.New("lost completion")); err == nil {
		t.Fatal("error disappeared")
	}
	other := &RecurseMission{Text: m.Text, Deliberator: m.Deliberator}
	_, _, err := other.begin(context.Background(), m.Deliberator.Workspace)
	if err == nil || !strings.Contains(err.Error(), "unsettled cycle 9") {
		t.Fatalf("replay admitted: %v", err)
	}
	if err := writeInFlight(m.Deliberator.Workspace, inFlight{Cycle: 9, MissionState: m.path}); err != nil {
		t.Fatal(err)
	}
	run := &recurseRun{root: m.Deliberator.Workspace}
	if err := run.settleInFlight(ctx, nil); err == nil || !strings.Contains(err.Error(), "reconcile owned effects") {
		t.Fatalf("ordinary recurse bypassed mission recovery: %v", err)
	}
}

func TestMissionMeterKeepsUnknownAndVersionedSpend(t *testing.T) {
	const model = "mission-budget-fixture"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	var meter missionMeter
	meter.Observed(usage.Report{Model: model, Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 20, Known: types.UsageTokens}})
	spend, unknown, reports := meter.snapshot()
	if spend < 30 || unknown || reports != 1 || len(meter.prices()) != 1 {
		t.Fatalf("meter=%d %v %d", spend, unknown, reports)
	}
	meter.Observed(usage.Report{Model: model, Usage: types.UsageMetadata{InputTokens: 10}})
	_, unknown, _ = meter.snapshot()
	if !unknown {
		t.Fatal("missing usage metadata treated as free")
	}
}

func TestMissionDeliberationStopsBeforeEffectsForValuesUnknownSpendAndDrift(t *testing.T) {
	const model = "mission-design-script"
	usage.RegisterPrice(model, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	for _, kind := range []string{"values", "unknown_spend", "source_changed"} {
		t.Run(kind, func(t *testing.T) {
			m, ctx := missionPolicyFixture(t)
			path := filepath.Join(m.Deliberator.Workspace, "note.txt")
			if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
				t.Fatal(err)
			}
			e := config.DefaultExplainConfig()
			m.Deliberator.Explain = core.ExplainOptions{MaxProofs: e.MaxProofs, MaxDepth: e.MaxDepth, MaxNodes: e.MaxNodes, MaxEdges: e.MaxEdges, MaxBytes: e.MaxBytes}
			m.Deliberator.Config = config.DefaultRoutingConfig()
			m.Deliberator.RunShard = func(ctx context.Context, role string, frame deliberation.Frame) (string, error) {
				if kind != "unknown_spend" {
					usage.RecordFromContext(ctx, usage.Report{Model: model, Usage: types.UsageMetadata{InputTokens: 1, Known: types.UsageTokens}})
				}
				var answer any
				if role == deliberation.Proposer {
					if kind == "source_changed" {
						if err := os.WriteFile(path, []byte("external edit"), 0600); err != nil {
							return "", err
						}
					}
					answer = deliberation.Proposal{Candidates: []deliberation.Candidate{
						{ID: "a", Brief: "Preserve existing behavior", SuccessTest: "The behavior is exercised", Evidence: []string{"note.txt"}},
						{ID: "b", Brief: "Add an explicit capability", SuccessTest: "The capability is exercised", Evidence: []string{"note.txt"}},
					}}
				} else {
					challenge := deliberation.Challenge{Recommendation: "a", Question: "Keep the existing API or add a new one?"}
					for _, c := range frame.Candidates {
						a := deliberation.Assessment{CandidateID: c.ID, Success: true}
						if kind == "values" {
							a.Items = []deliberation.Item{{Key: "preference", Severity: "critical", Kind: "values"}}
						}
						challenge.Assessments = append(challenge.Assessments, a)
					}
					answer = challenge
				}
				b, err := json.Marshal(answer)
				return string(b), err
			}
			a := RecurseAttempt{Cycle: 1, Node: SubsystemNode{ID: ".", Paths: []string{"."}}}
			err := m.prepare(ctx, &a)
			if err != errMissionStopped || m.State.Status != "/"+kind || a.Mission != nil {
				t.Fatalf("unexpected admission: %v %+v", err, m.State)
			}
			if err := m.end(ctx, err); err != nil {
				t.Fatal(err)
			}
			if m.State.Pending != nil || len(m.State.Receipts) != 1 {
				t.Fatalf("unexecuted design not receipted: %+v", m.State)
			}
			if kind == "values" && (m.State.Question == "" || len(m.State.Options) != 2) {
				t.Fatalf("values question lost: %+v", m.State)
			}
		})
	}
}

func TestMissionJudgmentsCannotBeWrittenByTheModel(t *testing.T) {
	policy := core.MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	for _, update := range []string{
		`mission_tick("m", 0, 0, 0, /no, 0).`,
		`mission_design("m", /ready).`,
		`mission_write_allowed("m", "policy.go").`,
		`mission_evidence("m", 1, /supported_gate_deficit).`,
	} {
		kept, _ := core.FilterMangleUpdates(nil, []string{update}, policy)
		if len(kept) != 0 {
			t.Fatalf("model forged a mission judgment: %v", kept)
		}
	}
}
