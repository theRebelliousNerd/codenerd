package northstar_test

// Integration coverage for the northstar -> kernel projection.
//
// This is the only file in the package that imports internal/core. It is kept
// in the external test package (northstar_test) so the production package stays
// a leaf: internal/northstar is depended on by the campaign risk gate and the
// CLI, and a compile-time dependency on the kernel there would make the vision
// guardian un-buildable whenever core is mid-edit.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/types"
)

func writeVisionJSON(t *testing.T, nerdDir string) {
	t.Helper()
	doc := northstar.WizardDocument{
		Mission: "Make the logic kernel the executive",
		Problem: "Improvised control flow is unauditable",
		Vision:  "Facts decide, the model creates",
		Personas: []northstar.WizardPersona{
			{Name: "Operator", Needs: []string{"auditable decisions"}},
		},
		Capabilities: []northstar.WizardCapability{
			{Description: "Project the vision into the kernel", Timeline: "now", Priority: "critical", Serves: []string{"Operator"}},
		},
		Risks: []northstar.WizardRisk{
			{Description: "Vision and code diverge", Likelihood: "high", Impact: "high", Mitigation: "reconcile on every boot"},
		},
		Requirements: []northstar.WizardRequirement{
			{ID: "REQ-A", Type: "functional", Description: "One authority", Priority: "must-have",
				Supports: []string{"cap_1"}, Addresses: []string{"risk_1"}},
		},
		Constraints: []string{"no network at boot"},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, northstar.VisionJSONFileName), data, 0o644); err != nil {
		t.Fatalf("write vision json: %v", err)
	}
}

func hasFact(facts []types.Fact, predicate string) bool {
	for _, f := range facts {
		if f.Predicate == predicate {
			return true
		}
	}
	return false
}

// TestGuardianBoot_WithVision_ShouldMakeNorthstarDefinedQueryTrue is the
// end-to-end the corpus asked for: a workspace whose only vision artifact is
// the wizard's JSON, booted the way session_boot.go boots, must leave the
// kernel answering northstar_defined().
func TestGuardianBoot_WithVision_ShouldMakeNorthstarDefinedQueryTrue(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)

	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	writeVisionJSON(t, nerdDir)

	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Skipf("kernel unavailable in this environment: %v", err)
	}

	guardian, err := northstar.AcquireGuardian(nerdDir, northstar.DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("AcquireGuardian: %v", err)
	}
	defer func() { _ = northstar.ReleaseGuardian(guardian) }()

	// Exactly the boot sequence both chat boot paths use.
	guardian.SetParentKernel(kernel)
	if err := guardian.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !guardian.HasVision() {
		t.Fatal("guardian did not pick up the wizard's vision")
	}

	defined, err := kernel.Query("northstar_defined")
	if err != nil {
		t.Fatalf("query northstar_defined: %v", err)
	}
	if len(defined) == 0 {
		t.Fatal("northstar_defined() is not true in the kernel after boot with a vision")
	}

	for _, predicate := range []string{
		"northstar_mission", "northstar_persona", "northstar_capability",
		"northstar_serves", "northstar_risk", "northstar_mitigation",
		"northstar_mitigation_text", "northstar_requirement",
		"northstar_supports", "northstar_addresses", "northstar_constraint",
	} {
		facts, err := kernel.Query(predicate)
		if err != nil {
			t.Errorf("query %s: %v", predicate, err)
			continue
		}
		if !hasFact(facts, predicate) {
			t.Errorf("kernel holds no %s facts after projection", predicate)
		}
	}
}

// TestGuardianBoot_WithoutVision_ShouldNotDefineNorthstar guards the other
// direction: an empty workspace must not assert northstar_defined(), or every
// injectable_context(/northstar_*) rule fires against an empty vision.
func TestGuardianBoot_WithoutVision_ShouldNotDefineNorthstar(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)

	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")

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

	defined, err := kernel.Query("northstar_defined")
	if err != nil {
		t.Fatalf("query northstar_defined: %v", err)
	}
	if len(defined) != 0 {
		t.Fatalf("northstar_defined() is true with no vision configured (%d facts)", len(defined))
	}
}

// TestGuardianUpdateVision_ShouldRetractStaleFacts proves the retract set in
// guardian.go covers the link predicates: a capability removed from the vision
// must not survive in the kernel.
func TestGuardianUpdateVision_ShouldRetractStaleFacts(t *testing.T) {
	t.Cleanup(northstar.ResetGuardianRegistry)

	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	writeVisionJSON(t, nerdDir)

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
	if facts, _ := kernel.Query("northstar_serves"); len(facts) == 0 {
		t.Fatal("precondition failed: no northstar_serves facts to retract")
	}

	stripped := guardian.GetVision()
	stripped.Capabilities = nil
	stripped.Requirements = nil
	if err := guardian.UpdateVision(stripped); err != nil {
		t.Fatalf("UpdateVision: %v", err)
	}

	for _, predicate := range []string{"northstar_serves", "northstar_supports", "northstar_addresses", "northstar_capability"} {
		facts, err := kernel.Query(predicate)
		if err != nil {
			t.Errorf("query %s: %v", predicate, err)
			continue
		}
		if hasFact(facts, predicate) {
			t.Errorf("%s facts survived a vision that no longer contains them", predicate)
		}
	}
}
