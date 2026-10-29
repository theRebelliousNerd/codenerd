package projectdoc

import (
	"strings"
	"testing"
)

func TestNorthstar_FullBlockParsesAndRoundTrips(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: Build a logic-first agent
  requirements:
    - id: REQ-1
      statement: The kernel must derive next_action
      severity: blocker
    - id: REQ-2
      statement: Every action must be permitted
      severity: major
---

Body is advisory.
`
	parsed, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.Spec.Northstar == nil {
		t.Fatal("Northstar is nil, want non-nil block")
	}
	ns := parsed.Spec.Northstar
	if ns.Purpose != "Build a logic-first agent" {
		t.Errorf("Purpose = %q, want %q", ns.Purpose, "Build a logic-first agent")
	}
	if len(ns.Requirements) != 2 {
		t.Fatalf("Requirements len = %d, want 2", len(ns.Requirements))
	}
	if ns.Requirements[0].ID != "REQ-1" {
		t.Errorf("Requirements[0].ID = %q, want %q", ns.Requirements[0].ID, "REQ-1")
	}
	if ns.Requirements[0].Statement != "The kernel must derive next_action" {
		t.Errorf("Requirements[0].Statement = %q, want %q", ns.Requirements[0].Statement, "The kernel must derive next_action")
	}
	if ns.Requirements[0].Severity != "blocker" {
		t.Errorf("Requirements[0].Severity = %q, want %q", ns.Requirements[0].Severity, "blocker")
	}
	if ns.Requirements[1].ID != "REQ-2" {
		t.Errorf("Requirements[1].ID = %q, want %q", ns.Requirements[1].ID, "REQ-2")
	}
	if ns.Requirements[1].Statement != "Every action must be permitted" {
		t.Errorf("Requirements[1].Statement = %q, want %q", ns.Requirements[1].Statement, "Every action must be permitted")
	}
	if ns.Requirements[1].Severity != "major" {
		t.Errorf("Requirements[1].Severity = %q, want %q", ns.Requirements[1].Severity, "major")
	}
}

func TestNorthstar_AbsentIsNil(t *testing.T) {
	doc := `---
schema: nerd/v1
project: codeNERD
---

Just prose.
`
	parsed, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if parsed.Spec.Northstar != nil {
		t.Errorf("Northstar = %+v, want nil when no northstar key present", parsed.Spec.Northstar)
	}
}

func TestNorthstar_EmptyIDRejected(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: test purpose
  requirements:
    - id: ""
      statement: something is required
      severity: blocker
---

Body.
`
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("Parse should reject requirement with empty id")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "id") {
		t.Errorf("error must mention id, got: %v", err)
	}
}

func TestNorthstar_EmptyStatementRejected(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: test purpose
  requirements:
    - id: REQ-1
      statement: ""
      severity: blocker
---

Body.
`
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("Parse should reject requirement with empty statement")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "statement") {
		t.Errorf("error must mention statement, got: %v", err)
	}
}

func TestNorthstar_DuplicateIDRejected(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: test purpose
  requirements:
    - id: REQ-1
      statement: first
      severity: blocker
    - id: REQ-1
      statement: second
      severity: major
---

Body.
`
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("Parse should reject duplicate requirement ids")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		t.Errorf("error must mention duplicate, got: %v", err)
	}
}

func TestNorthstar_InvalidSeverityRejected(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: test purpose
  requirements:
    - id: REQ-1
      statement: something is required
      severity: critical
---

Body.
`
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("Parse should reject severity critical")
	}
	msg := strings.ToLower(err.Error())
	// validation message must name the allowed values
	for _, want := range []string{"blocker", "major", "minor"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error must name allowed values (blocker, major, minor), missing %q in: %v", want, err)
		}
	}
	if !strings.Contains(msg, "severity") {
		t.Errorf("error must mention severity, got: %v", err)
	}
}

func TestNorthstar_SeverityOmittedAccepted(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: test purpose
  requirements:
    - id: REQ-1
      statement: something is required
---

Body.
`
	parsed, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse with severity omitted should be accepted, got error: %v", err)
	}
	if parsed.Spec.Northstar == nil {
		t.Fatal("Northstar is nil, want non-nil")
	}
	if len(parsed.Spec.Northstar.Requirements) != 1 {
		t.Fatalf("Requirements len = %d, want 1", len(parsed.Spec.Northstar.Requirements))
	}
	if parsed.Spec.Northstar.Requirements[0].Severity != "" {
		t.Errorf("Severity = %q, want empty when omitted", parsed.Spec.Northstar.Requirements[0].Severity)
	}
}

func TestNorthstar_UnknownKeyRejected(t *testing.T) {
	doc := `---
schema: nerd/v1
northstar:
  purpose: test purpose
  unknown_key: should fail
  requirements:
    - id: REQ-1
      statement: something is required
      severity: blocker
---

Body.
`
	_, err := Parse([]byte(doc))
	if err == nil {
		t.Fatal("Parse should reject unknown key inside northstar block")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unknown_key") {
		t.Errorf("error must name the offending unknown key, got: %v", err)
	}
}
