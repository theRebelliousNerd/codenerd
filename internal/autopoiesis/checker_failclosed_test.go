package autopoiesis

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// permissiveConfig is the most-allowing configuration the checker supports.
// Used to prove the fail-closed path denies even when every capability gate is
// open, i.e. that denial comes from the missing policy and not from the
// allowlist.
func permissiveConfig() OuroborosConfig {
	return OuroborosConfig{
		AllowFileSystem: true,
		AllowNetworking: true,
		AllowExec:       true,
	}
}

// What this test does and does not claim, recorded because a commit message on
// this branch overstated it.
//
// Adversarial review tried to reproduce a fail-OPEN on the base branch and
// could not: with an empty policy, every input it tried was already denied,
// including a tool importing os/exec and one calling panic. The mechanism was
// accidental rather than designed. Check calls engine.AddFacts before it
// queries ?violation(V), an empty policy declares no predicates, so the first
// fact was rejected with "predicate ast_import is not declared in schemas" and
// the error was routed through sc.fail — Safe=false, Score=0,
// SeverityBlocking. buildAllowedPackages always returns a non-empty base list,
// so the fact set was never empty and the query was unreachable.
//
// So the branch's contribution is attribution, not safety: the denial now comes
// from an explicit policyErr check before any work, names go_safety.mg as the
// location, and says the policy is unavailable rather than blaming an
// undeclared predicate the author never wrote. That is worth having — a
// blocking violation nobody can act on is nearly as bad as none — but it is not
// the difference between a gate that worked and one that did not.
//
// The assertions below describe behavior the base branch also had. Keep them:
// they were incidental there and are load-bearing here, which is exactly when a
// property needs a test.
func TestSafetyChecker_WhenPolicyFailsToLoad_ShouldDenyEverything(t *testing.T) {
	checker := newSafetyCheckerWithPolicy(permissiveConfig(), "", errors.New("embedded FS read failed"))

	if checker.PolicyLoadError() == nil {
		t.Fatal("expected checker to report a policy load error")
	}

	// The most boring, obviously-safe program there is.
	report := checker.Check(`package main

func main() {}`)

	if report.Safe {
		t.Fatal("checker reported Safe with no policy loaded: an unaudited tool would be committed and executed")
	}
	if len(report.Violations) == 0 {
		t.Fatal("expected a blocking violation explaining the denial")
	}
	v := report.Violations[0]
	if v.Type != ViolationPolicy {
		t.Errorf("violation type = %v, want ViolationPolicy", v.Type)
	}
	if v.Severity != SeverityBlocking {
		t.Errorf("severity = %v, want SeverityBlocking", v.Severity)
	}
	if !strings.Contains(v.Description, "embedded FS read failed") {
		t.Errorf("violation should name the underlying load failure, got %q", v.Description)
	}
	if report.Score != 0 {
		t.Errorf("score = %v, want 0", report.Score)
	}
}

// An empty policy string is the shape the old code produced on load failure:
// it looked like a successfully-constructed checker but permitted everything.
func TestSafetyChecker_WhenPolicyIsEmpty_ShouldTreatAsLoadFailure(t *testing.T) {
	checker := newSafetyCheckerWithPolicy(permissiveConfig(), "   \n\t ", nil)

	if checker.PolicyLoadError() == nil {
		t.Fatal("an empty policy must be classified as a load failure, not as a policy that permits everything")
	}

	// Code that the real policy rejects outright.
	report := checker.Check(`package main

import "os/exec"

func main() { _ = exec.Command("rm", "-rf", "/") }`)
	if report.Safe {
		t.Fatal("empty policy reported Safe: this is the exact hole the fail-closed change exists to remove")
	}
}

func TestSafetyChecker_WhenEmbeddedPolicyIsPresent_ShouldNotBeFailClosed(t *testing.T) {
	if goSafetyPolicyErr != nil {
		t.Fatalf("embedded go_safety.mg failed to load: %v", goSafetyPolicyErr)
	}
	checker := NewSafetyChecker(OuroborosConfig{})
	if err := checker.PolicyLoadError(); err != nil {
		t.Fatalf("production checker is fail-closed: %v", err)
	}
	if report := checker.Check("package main\n\nfunc main() {}"); !report.Safe {
		t.Fatalf("trivial safe program rejected: %+v", report.Violations)
	}
}

// =============================================================================
// GOLDEN SUITE PER ViolationType  (TODO P2 "Golden suite per ViolationType")
// =============================================================================

// violationGolden pins, for every ViolationType, either a source sample that
// provokes it or an explicit statement of why the checker cannot produce it.
//
// The point is that the enum stops being a wish list. Before this, four of the
// eleven categories (unsafe pointer, reflection, cgo, exec) existed only as
// constants — nothing in Go or in go_safety.mg ever emitted them — so the
// feedback string handed to the LLM for regeneration could never name the real
// hazard. Two remain unproducible by the checker and say so.
type violationGolden struct {
	code string // source that must trigger this violation type
	// notProducedByChecker explains why the checker cannot emit this type.
	// Mutually exclusive with code.
	notProducedByChecker string
}

func violationGoldens() map[ViolationType]violationGolden {
	return map[ViolationType]violationGolden{
		ViolationForbiddenImport: {code: `package main

import "plugin"

func main() { _ = plugin.Open }`},
		ViolationDangerousCall: {notProducedByChecker: "go_safety.mg derives violations from imports, goroutine spawns and panic calls only; " +
			"there is no dangerous-call rule, so os.RemoveAll under AllowFileSystem is reported as nothing at all. " +
			"The alias tracking in astFactEmitter.handleAssignment already collects the facts a future rule would need."},
		ViolationUnsafePointer: {code: `package main

import "unsafe"

func main() { var x int; _ = unsafe.Pointer(&x) }`},
		ViolationReflection: {code: `package main

import "reflect"

func main() { _ = reflect.TypeOf(0) }`},
		ViolationCGO: {code: `package main

import "C"

func main() {}`},
		ViolationExec: {code: `package main

import "os/exec"

func main() { _ = exec.Command("whoami") }`},
		ViolationPanic: {code: `package main

func main() { panic("boom") }`},
		ViolationGoroutineLeak: {code: `package main

func main() {
	go func() {
		select {}
	}()
}`},
		ViolationParseError: {code: `this is not go source at all`},
		ViolationPolicy: {notProducedByChecker: "emitted for infrastructure failures (policy load, engine init, query error) rather than for a code shape; " +
			"covered by TestSafetyChecker_WhenPolicyFailsToLoad_ShouldDenyEverything"},
		ViolationPanicMakerKill: {notProducedByChecker: "produced by the Thunderdome in OuroborosLoop.ExecuteWithConfig, not by the static checker; " +
			"covered by the Thunderdome tests"},
	}
}

func TestSafetyChecker_WhenGivenGoldenSample_ShouldReportMatchingViolationType(t *testing.T) {
	goldens := violationGoldens()

	// Enumerate the enum by walking until String() stops recognising values,
	// so a newly added ViolationType fails here until it is classified.
	var all []ViolationType
	for v := ViolationType(0); v.String() != "unknown"; v++ {
		all = append(all, v)
		if len(all) > 64 {
			t.Fatal("ViolationType enumeration did not terminate")
		}
	}
	if len(all) < 11 {
		t.Fatalf("enumerated only %d violation types; expected at least 11", len(all))
	}

	// Deny-everything config: the samples describe what a *rejected* import
	// looks like, so granting the capability would remove the violation
	// entirely rather than change its classification.
	checker := newSafetyCheckerWithPolicy(OuroborosConfig{}, goSafetyPolicy, goSafetyPolicyErr)

	for _, vt := range all {
		golden, ok := goldens[vt]
		if !ok {
			t.Errorf("ViolationType %q (%d) has no golden entry: add a provoking sample or state why the checker cannot produce it", vt, vt)
			continue
		}
		if golden.notProducedByChecker != "" {
			if golden.code != "" {
				t.Errorf("%q has both a sample and an unreachable reason", vt)
			}
			t.Logf("%-18s not produced by checker: %s", vt.String(), golden.notProducedByChecker)
			continue
		}

		t.Run(vt.String(), func(t *testing.T) {
			report := checker.Check(golden.code)
			if report.Safe {
				t.Fatalf("sample for %q passed the safety check", vt)
			}
			for _, v := range report.Violations {
				if v.Type == vt {
					if v.Severity != SeverityBlocking {
						t.Errorf("severity = %v, want SeverityBlocking", v.Severity)
					}
					return
				}
			}
			t.Fatalf("sample for %q produced %s instead", vt, formatViolationTypes(report.Violations))
		})
	}
}

func formatViolationTypes(vs []SafetyViolation) string {
	if len(vs) == 0 {
		return "no violations"
	}
	parts := make([]string, 0, len(vs))
	for _, v := range vs {
		parts = append(parts, fmt.Sprintf("%s(%s)", v.Type, v.Description))
	}
	return strings.Join(parts, ", ")
}

// =============================================================================
// AllowExec AUDIT  (TODO P0 "Audit default AllowExec: true")
// =============================================================================

func TestOuroborosDefaults_WhenNotConfigured_ShouldDenyExecToGeneratedTools(t *testing.T) {
	if cfg := DefaultOuroborosConfig(t.TempDir()); cfg.AllowExec {
		t.Error("DefaultOuroborosConfig grants os/exec by default; generated tools must not get a shell without an explicit opt-in")
	}
	if cfg := DefaultOuroborosConfig(t.TempDir()); cfg.AllowNetworking {
		t.Error("DefaultOuroborosConfig grants networking by default")
	}

	// The orchestrator builds its own OuroborosConfig; the gate has to hold
	// there too, since that is the config production actually runs with.
	orchCfg := Config{ToolsDir: t.TempDir(), AgentsDir: t.TempDir(), WorkspaceRoot: t.TempDir()}
	if orchCfg.AllowToolExec {
		t.Error("autopoiesis.Config zero value must not grant exec")
	}

	checker := NewSafetyChecker(OuroborosConfig{AllowFileSystem: true, AllowExec: orchCfg.AllowToolExec})
	report := checker.Check(`package main

import "os/exec"

func main() { _ = exec.Command("curl", "http://evil") }`)
	if report.Safe {
		t.Fatal("a tool importing os/exec passed the default safety configuration")
	}
}

func TestSafetyChecker_WhenExecExplicitlyGranted_ShouldAllowOsExec(t *testing.T) {
	checker := NewSafetyChecker(OuroborosConfig{AllowExec: true})
	report := checker.Check(`package main

import "os/exec"

func main() { _ = exec.Command("git", "status") }`)
	if !report.Safe {
		t.Fatalf("explicit exec grant did not open the gate: %+v", report.Violations)
	}
}
