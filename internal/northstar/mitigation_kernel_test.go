package northstar_test

// Kernel-level coverage for blueprint section 9 / NS-VISION-03: mitigation
// prose is not mitigation, evidence is revision-bound, non-passing evidence
// cannot verify, and a waiver stays waived (never mitigated).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/types"
)

const (
	headSHA  = "1111111111111111111111111111111111111111"
	otherSHA = "2222222222222222222222222222222222222222"
)

func highRisk(id, mitigation string, status northstar.MitigationStatus, evidence []northstar.MitigationEvidence, waiver *northstar.RiskWaiver) northstar.WizardRisk {
	return northstar.WizardRisk{
		ID: id, Description: "risk " + id, Likelihood: "high", Impact: "high",
		Mitigation: mitigation, MitigationStatus: status, MitigationEvidence: evidence, Waiver: waiver,
	}
}

func passed(rev string) northstar.MitigationEvidence {
	return northstar.MitigationEvidence{Kind: "test", Ref: "go test ./internal/x", Outcome: northstar.EvidencePassed, Revision: rev}
}

func derivedRiskIDs(t *testing.T, kernel *core.RealKernel, predicate string) map[string]bool {
	t.Helper()
	facts, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	out := map[string]bool{}
	for _, f := range facts {
		if len(f.Args) > 0 {
			out[types.ExtractString(f.Args[0])] = true
		}
	}
	return out
}

func setClock(t *testing.T, kernel *core.RealKernel, head string, now time.Time) {
	t.Helper()
	for _, p := range []string{"git_state", "current_time"} {
		if err := kernel.Retract(p); err != nil {
			t.Fatalf("retract %s: %v", p, err)
		}
	}
	if err := kernel.Assert(types.Fact{Predicate: "git_state", Args: []any{"/head", head}}); err != nil {
		t.Fatalf("assert head: %v", err)
	}
	if err := kernel.Assert(types.Fact{Predicate: "current_time", Args: []any{now.Unix()}}); err != nil {
		t.Fatalf("assert clock: %v", err)
	}
}

func TestMitigationState_KernelDistinguishesProseImplementedVerifiedAndWaived(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)

	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	now := time.Now()
	future := &northstar.RiskWaiver{Authority: "lead", Scope: "beta", Rationale: "accepted for beta", Expires: now.Add(24 * time.Hour)}
	past := &northstar.RiskWaiver{Authority: "lead", Scope: "beta", Rationale: "accepted for beta", Expires: now.Add(-24 * time.Hour)}
	skipped := northstar.MitigationEvidence{Kind: "test", Ref: "go test ./internal/y", Outcome: northstar.EvidenceSkipped, Revision: headSHA}

	doc := northstar.WizardDocument{
		Mission: "Trustworthy delivery",
		Risks: []northstar.WizardRisk{
			highRisk("risk_prose", "write tests", "", nil, nil),
			highRisk("risk_impl", "added a guard", northstar.MitigationImplemented, nil, nil),
			highRisk("risk_ver", "added a guard", northstar.MitigationVerified, []northstar.MitigationEvidence{passed(headSHA)}, nil),
			highRisk("risk_stale", "added a guard", northstar.MitigationVerified, []northstar.MitigationEvidence{passed(otherSHA)}, nil),
			highRisk("risk_incomplete", "added a guard", northstar.MitigationVerified, []northstar.MitigationEvidence{passed(headSHA), skipped}, nil),
			highRisk("risk_waived", "", northstar.MitigationWaived, nil, future),
			highRisk("risk_expired", "", northstar.MitigationWaived, nil, past),
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, northstar.VisionJSONFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}

	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Skipf("kernel unavailable in this environment: %v", err)
	}
	guardian, err := northstar.AcquireGuardian(nerdDir, northstar.DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("AcquireGuardian: %v", err)
	}
	defer func() { _ = northstar.ReleaseGuardian(guardian) }()
	guardian.SetParentKernel(kernel)
	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !guardian.HasVision() {
		t.Fatal("vision with mitigation records was not imported")
	}
	setClock(t, kernel, headSHA, now)

	unmitigated := derivedRiskIDs(t, kernel, "unmitigated_risk")
	mitigated := derivedRiskIDs(t, kernel, "has_mitigation")
	for id, want := range map[string]bool{
		"risk_prose":      true,  // prose is a proposal, not a mitigation
		"risk_impl":       true,  // implemented but unverified
		"risk_ver":        false, // verified with passing evidence at HEAD
		"risk_stale":      true,  // evidence from another revision
		"risk_incomplete": true,  // a skipped check at HEAD defeats verification
		"risk_waived":     false, // active waiver suppresses the warning...
		"risk_expired":    true,  // ...until it expires
	} {
		if unmitigated[id] != want {
			t.Errorf("unmitigated_risk(%s) = %v, want %v", id, unmitigated[id], want)
		}
	}
	if !mitigated["risk_ver"] {
		t.Error("has_mitigation(risk_ver) should derive from fresh passing evidence")
	}
	for _, id := range []string{"risk_prose", "risk_impl", "risk_stale", "risk_incomplete", "risk_waived", "risk_expired"} {
		if mitigated[id] {
			t.Errorf("has_mitigation(%s) derived; only verified fresh evidence may (a waiver is waived, not passed)", id)
		}
	}

	// A new commit invalidates the verification until it is re-recorded.
	setClock(t, kernel, otherSHA, now)
	if !derivedRiskIDs(t, kernel, "unmitigated_risk")["risk_ver"] {
		t.Error("risk_ver stayed mitigated after HEAD moved; revision-bound evidence went stale and must not count")
	}

	// The unmitigated risks reach prompts through injectable_context.
	ctx, err := kernel.Query("injectable_context")
	if err != nil {
		t.Fatalf("query injectable_context: %v", err)
	}
	found := false
	for _, f := range ctx {
		if len(f.Args) > 1 && strings.Contains(types.ExtractString(f.Args[1]), "risk risk_prose") {
			found = true
		}
	}
	if !found {
		t.Error("an unmitigated high risk with only mitigation prose was not injected as context")
	}
}
