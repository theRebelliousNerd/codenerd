package transparency

import (
	"strings"
	"testing"
)

func TestSafetyReporterViolationClassification(t *testing.T) {
	reporter := NewSafetyReporter()
	violation := reporter.ReportViolation("rm -rf /tmp", "/tmp", "policy_rule")
	if violation == nil {
		t.Fatalf("expected violation")
	}
	if violation.ViolationType != ViolationDestructiveAction {
		t.Fatalf("unexpected violation type: %v", violation.ViolationType)
	}
	if violation.ID == "" {
		t.Fatalf("expected violation id")
	}

	formatted := reporter.FormatViolation(violation)
	if !strings.Contains(formatted, "[SAFETY]") {
		t.Fatalf("expected safety header")
	}
	if !strings.Contains(formatted, "How to proceed") {
		t.Fatalf("expected remediation section")
	}
}

func TestSafetyReporterSecretExposure(t *testing.T) {
	reporter := NewSafetyReporter()
	violation := reporter.ReportViolation("cat", ".env", "policy_rule")
	if violation.ViolationType != ViolationSecretExposure {
		t.Fatalf("expected secret exposure violation, got %v", violation.ViolationType)
	}
}
