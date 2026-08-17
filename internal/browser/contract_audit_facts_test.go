package browser

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	browsersecurity "codenerd/internal/browser/security"
)

func TestAuditDiscoveryFacts_ShouldEmitOneFindingFactPerFinding(t *testing.T) {
	disc := ContractAuditDiscovery{
		Findings: []AuditFinding{
			{Kind: AuditObservation, Subject: "alpha", Detail: "detail a"},
			{Kind: AuditInference, Subject: "beta", Detail: "detail b"},
			{Kind: AuditSkipped, Subject: "gamma", Detail: "detail c"},
		},
	}
	facts := AuditDiscoveryFacts("sess-1", disc, 1234567890)
	count := 0
	for _, f := range facts {
		if f.Predicate == "audit_finding" {
			count++
		}
	}
	if count != len(disc.Findings) {
		t.Fatalf("got %d audit_finding facts, want %d", count, len(disc.Findings))
	}
}

func TestAuditDiscoveryFacts_ShouldScopeEveryFactToSession(t *testing.T) {
	sessionID := "sess-isolated-123"
	disc := ContractAuditDiscovery{
		Needles: []string{"needle1", "needle2"},
		Findings: []AuditFinding{
			{Kind: AuditObservation, Subject: "needle1", Detail: "detail1", Sources: []string{"a/b.go"}},
			{Kind: AuditInference, Subject: "needle2", Detail: "detail2", Sources: []string{"c/d.go"}},
		},
		Matches: []RepoMatch{
			{Path: "a/b.go", Line: 10, Needle: "needle1"},
			{Path: "c/d.go", Line: 20, Needle: "needle2"},
		},
	}
	facts := AuditDiscoveryFacts(sessionID, disc, 999)
	if len(facts) == 0 {
		t.Fatal("expected facts")
	}
	for _, f := range facts {
		if len(f.Args) == 0 {
			t.Fatalf("fact %s has no args", f.Predicate)
		}
		if got, ok := f.Args[0].(string); !ok || got != sessionID {
			t.Errorf("fact %s scoped %q, want %q", f.Predicate, got, sessionID)
		}
	}
}

func TestAuditDiscoveryFacts_ShouldNeverEmitHazard(t *testing.T) {
	disc := ContractAuditDiscovery{
		Findings: []AuditFinding{
			{Kind: AuditApprovalRequired, Subject: "control1", Detail: "needs approval"},
			{Kind: AuditObservation, Subject: "obs1", Detail: "observed"},
		},
		Needles: []string{"control1", "obs1"},
	}
	facts := AuditDiscoveryFacts("s1", disc, 1000)
	for _, f := range facts {
		if f.Predicate == "audit_hazard" {
			t.Fatalf("must not emit audit_hazard, found %v", f)
		}
	}
}

func TestAuditDiscoveryFacts_WhenSubjectEmpty_ShouldSkip(t *testing.T) {
	disc := ContractAuditDiscovery{
		Findings: []AuditFinding{
			{Kind: AuditObservation, Subject: "", Detail: "empty should skip", Sources: []string{"a.go"}},
			{Kind: AuditObservation, Subject: "valid", Detail: "valid detail", Sources: []string{"b.go"}},
			{Kind: AuditObservation, Subject: "   ", Detail: "whitespace skip", Sources: []string{"c.go"}},
		},
		Matches: []RepoMatch{
			{Path: "a.go", Line: 1, Needle: ""},
			{Path: "b.go", Line: 2, Needle: "valid"},
		},
	}
	facts := AuditDiscoveryFacts("s1", disc, 1000)
	count := 0
	for _, f := range facts {
		if f.Predicate == "audit_finding" {
			count++
			if f.Args[2] == "" || strings.TrimSpace(fmt.Sprint(f.Args[2])) == "" {
				t.Error("emitted finding with empty subject")
			}
		}
		if f.Predicate == "audit_source" {
			if strings.TrimSpace(fmt.Sprint(f.Args[1])) == "" {
				t.Error("emitted source with empty subject")
			}
		}
	}
	if count != 1 {
		t.Fatalf("got %d finding facts, want 1", count)
	}
}

func TestAuditDiscoveryFacts_ShouldCapTotalFacts(t *testing.T) {
	var findings []AuditFinding
	var needles []string
	for i := 0; i < 600; i++ {
		subj := fmt.Sprintf("subject-%d", i)
		findings = append(findings, AuditFinding{Kind: AuditObservation, Subject: subj, Detail: "detail"})
		needles = append(needles, subj)
	}
	disc := ContractAuditDiscovery{Findings: findings, Needles: needles}
	facts := AuditDiscoveryFacts("s1", disc, 1000)
	if len(facts) > maxAuditFacts {
		t.Fatalf("facts %d exceeds cap %d", len(facts), maxAuditFacts)
	}
	if len(facts) != maxAuditFacts {
		t.Fatalf("expected capped at %d, got %d", maxAuditFacts, len(facts))
	}
}

func TestAuditDiscoveryFacts_ShouldEmitRelativePathsOnly(t *testing.T) {
	disc := ContractAuditDiscovery{
		Findings: []AuditFinding{
			{Kind: AuditObservation, Subject: "needle1", Detail: "d", Sources: []string{"/absolute/path/file.go", "relative/path/file.go"}},
			{Kind: AuditInference, Subject: "needle2", Detail: "d2", Sources: []string{"/tmp/abs2.go"}},
		},
		Matches: []RepoMatch{
			{Path: "/absolute/path/file.go", Line: 10, Needle: "needle1"},
			{Path: "relative/path/file.go", Line: 20, Needle: "needle1"},
			{Path: "/tmp/abs2.go", Line: 30, Needle: "needle2"},
		},
	}
	facts := AuditDiscoveryFacts("s1", disc, 1000)
	for _, f := range facts {
		if f.Predicate != "audit_source" {
			continue
		}
		pathArg := fmt.Sprint(f.Args[2])
		if filepath.IsAbs(pathArg) {
			t.Errorf("audit_source Path is absolute %q", pathArg)
		}
	}
}

func TestAssertAuditDiscovery_ShouldRedactThroughAddFacts(t *testing.T) {
	secret := "superSecret123"
	detail := fmt.Sprintf("found password=%s in file", secret)
	redactor := browsersecurity.NewRedactor(nil)
	sanitizedCheck := redactor.SanitizeString(detail)
	if strings.Contains(sanitizedCheck, secret) {
		t.Fatalf("redactor does not catch secret %q; choose a different pattern, got %q", secret, sanitizedCheck)
	}
	if !strings.Contains(sanitizedCheck, browsersecurity.Redacted) {
		t.Fatalf("expected redacted marker in %q", sanitizedCheck)
	}
	disc := ContractAuditDiscovery{
		Findings: []AuditFinding{
			{Kind: AuditObservation, Subject: "test-subject", Detail: detail},
		},
		Needles: []string{"test-subject"},
	}
	sink := &mockEngineSink{}
	mgr := NewSessionManagerWithSink(DefaultConfig(), sink)
	if err := mgr.AssertAuditDiscovery("sess-redact", disc); err != nil {
		t.Fatalf("AssertAuditDiscovery error: %v", err)
	}
	got := sink.getFacts()
	if len(got) == 0 {
		t.Fatal("no facts asserted")
	}
	for _, f := range got {
		for _, arg := range f.Args {
			if s, ok := arg.(string); ok && strings.Contains(s, secret) {
				t.Fatalf("secret %q survived redaction in %s args %v", secret, f.Predicate, f.Args)
			}
		}
	}
	foundRedacted := false
	for _, f := range got {
		if f.Predicate == "audit_finding" {
			for _, arg := range f.Args {
				if s, ok := arg.(string); ok && strings.Contains(s, browsersecurity.Redacted) {
					foundRedacted = true
				}
			}
		}
	}
	if !foundRedacted {
		t.Error("expected redacted marker in asserted facts")
	}
}
