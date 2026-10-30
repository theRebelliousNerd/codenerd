package system

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/consultation"
	"codenerd/internal/core"
)

// The proposal prompt is hashed by FeedbackLoop.CanRetryPrompt for per-prompt
// retry accounting: map-ordered context and pattern lists would mint a
// fresh-looking prompt on every call and defeat the retry budget.
func TestBuildPolicyProposalPrompt_Stable(t *testing.T) {
	e := NewExecutivePolicyShard()
	e.patternSuccess["zebra"] = 5
	e.patternSuccess["alpha"] = 5
	e.patternFailure["delta"] = 3
	e.patternFailure["beta"] = 3
	cases := []UnhandledCase{{
		Query:   "next_action(/x)",
		Context: map[string]string{"zeta": "1", "aardvark": "2"},
	}}
	first := e.buildPolicyProposalPrompt(cases)
	for i := 0; i < 10; i++ {
		if got := e.buildPolicyProposalPrompt(cases); got != first {
			t.Fatalf("prompt differs between builds:\n%s\n---\n%s", first, got)
		}
	}
	if !strings.Contains(first, "alpha\n- zebra") {
		t.Errorf("success patterns not in sorted order:\n%s", first)
	}
	if !strings.Contains(first, "aardvark: 2\n   zeta: 1") {
		t.Errorf("case context not in sorted order:\n%s", first)
	}
}

// Proposed-rule confidence documents 0.0-1.0 and gates HotLoad: 1.5 must
// not auto-apply, nor -0.5 linger.
func TestParseProposedRule_ConfidenceClamped(t *testing.T) {
	e := NewExecutivePolicyShard()
	if got := e.parseProposedRule("RULE: x :- y.\nCONFIDENCE: 1.5\n", nil).Confidence; got != 1 {
		t.Errorf("1.5 -> %v, want 1", got)
	}
	if got := e.parseProposedRule("RULE: x :- y.\nCONFIDENCE: -0.5\n", nil).Confidence; got != 0 {
		t.Errorf("-0.5 -> %v, want 0", got)
	}
	if got := e.parseProposedRule("RULE: x :- y.\nCONFIDENCE: 0.7\n", nil).Confidence; got != 0.7 {
		t.Errorf("0.7 -> %v, want 0.7", got)
	}
}

// slowFirstSpawner answers the first target last, so completion order is the
// reverse of requested order. The batch contract is requested order: both
// consumers render responses in slice order into reports.
type slowFirstSpawner struct{}

func (slowFirstSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if specialistName == "slow" {
		select {
		case <-time.After(150 * time.Millisecond):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "ADVICE: from " + specialistName + "\nCONFIDENCE: 80", nil
}

func TestBatchConsultation_RequestedOrder(t *testing.T) {
	m := consultation.New(slowFirstSpawner{})
	batch := m.RequestBatchConsultation(context.Background(), consultation.BatchRequest{
		Question:    "q",
		Specialists: []string{"slow", "fast-a", "fast-b"},
	})
	if err := batch.Err(); err != nil {
		t.Fatalf("batch failed: %v", err)
	}
	responses := batch.Succeeded()
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3", len(responses))
	}
	for i, want := range []string{"slow", "fast-a", "fast-b"} {
		if responses[i].Specialist != want {
			t.Fatalf("position %d = %s, want %s (full: %v)", i, responses[i].Specialist, want, responses)
		}
	}
}

// rejectAndTimeoutSpawner is the one-rejection-plus-one-timeout fixture: the
// coder rejects, the tester never answers inside the deadline, the others
// approve.
type rejectAndTimeoutSpawner struct{}

func (rejectAndTimeoutSpawner) SpawnConsultation(ctx context.Context, spec, _ string) (string, error) {
	switch spec {
	case "coder":
		return "VOTE: REJECT\nCONFIDENCE: 90\nREASONING:\nthe write sets collide", nil
	case "tester":
		<-ctx.Done()
		return "", ctx.Err()
	}
	return "VOTE: APPROVE\nCONFIDENCE: 80\nREASONING:\nfine", nil
}

// The supervised-campaign door used to log a failed advisor and return the
// survivors with a nil error, so a panel missing its tester looked complete.
// Through the engine and board this door now constructs, the rejection is
// kept, the timeout is a named missing review, and nothing is approved.
func TestCampaignRunnerDoor_RejectionSurvivesTimeout(t *testing.T) {
	board := campaign.NewShardAdvisoryBoard(consultation.New(rejectAndTimeoutSpawner{}))
	cfg := campaign.DefaultAdvisoryConfig()
	cfg.ConsultTimeout = 50 * time.Millisecond
	board.WithConfig(cfg)
	responses, err := board.ConsultAdvisors(context.Background(), campaign.AdvisoryRequest{CampaignID: "/c", Goal: "g"})
	if err != nil {
		t.Fatal(err)
	}
	s := board.SynthesizeVotes(responses)
	if s.Approved || !s.Incomplete {
		t.Fatalf("approved=%v incomplete=%v, want an incomplete, unapproved panel", s.Approved, s.Incomplete)
	}
	if len(s.BlockingConcerns) != 1 || s.BlockingConcerns[0].Advisor != "coder" || s.BlockingConcerns[0].Severity != "blocking" {
		t.Fatalf("blocking concerns = %+v, want the coder's rejection", s.BlockingConcerns)
	}
	if len(s.Missing) != 1 || s.Missing[0].Advisor != "tester" {
		t.Fatalf("missing = %+v, want tester", s.Missing)
	}
}

// Learned-pattern lists read map registries: they must come out sorted, not
// in iteration order, for every consumer that renders them.
func TestGetLearnedPatterns_Sorted(t *testing.T) {
	e := NewExecutivePolicyShard()
	for _, p := range []string{"zebra", "alpha", "mike"} {
		e.patternSuccess[p] = 5
		e.patternFailure[p] = 3
	}
	for i := 0; i < 5; i++ {
		got := e.GetLearnedPatterns()
		want := []string{"alpha", "mike", "zebra"}
		for j, w := range want {
			if got["successful"][j] != w || got["failed"][j] != w {
				t.Fatalf("run %d: got %v / %v, want sorted %v", i, got["successful"], got["failed"], want)
			}
		}
	}

	p := NewPerceptionFirewallShard()
	for _, q := range []string{"zebra", "alpha", "mike"} {
		p.BaseSystemShard.patternSuccess[q] = 5
		p.BaseSystemShard.corrections[q] = 3
	}
	got := p.GetLearnedPatterns()
	for j, w := range []string{"alpha", "mike", "zebra"} {
		if got["successful"][j] != w || got["corrections"][j] != w {
			t.Fatalf("perception: got %v / %v, want sorted", got["successful"], got["corrections"])
		}
	}
}

// The domain allowlist matches hosts, not substrings: evil-github.com and
// github.com.evil.com merely contain an allowed name and must be refused,
// while genuine subdomains and ports stay admitted.
func TestIsAllowedDomain_HostMatch(t *testing.T) {
	shard := NewConstitutionGateShard()
	cases := []struct {
		target string
		want   bool
	}{
		{"https://github.com/user/repo", true},
		{"https://api.github.com:8443/v1", true},
		{"github.com", true},
		{"https://evil-github.com/x", false},
		{"https://github.com.evil.com/x", false},
		{"https://githubXcom/", false},
		{"https://evil.example.com", false},
		{"", false},
	}
	for _, c := range cases {
		if got := shard.isAllowedDomain(c.target); got != c.want {
			t.Errorf("isAllowedDomain(%q) = %v, want %v", c.target, got, c.want)
		}
	}
}

// The rule-proposal prompt is hashed by CanRetryPrompt for per-prompt retry
// accounting: map-ordered context would mint a fresh-looking prompt on every
// call and defeat the retry budget.
func TestBuildRuleProposalPrompt_Stable(t *testing.T) {
	c := NewConstitutionGateShard()
	cases := []UnhandledCase{{
		Query:   "permitted(/x)",
		Context: map[string]string{"zeta": "1", "aardvark": "2", "mid": "3"},
	}}
	first := c.buildRuleProposalPrompt(cases)
	for i := 0; i < 10; i++ {
		if got := c.buildRuleProposalPrompt(cases); got != first {
			t.Fatalf("prompt differs between builds:\n%s\n---\n%s", first, got)
		}
	}
}

// classifyVerbMapping's unknown-verb branch is unreachable through Perceive
// (the transducer only emits corpus verbs), but it is the documented guard:
// a corpus-unknown verb must classify unmapped rather than slip through.
func TestClassifyVerbMapping_UnknownVerb(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	// A real kernel: with no kernel the mapping check degrades to mapped
	// (kernelless Perceive is already degraded mode everywhere else), so
	// the unknown-verb branch needs mappings to consult.
	p := NewPerceptionFirewallShard()
	p.SetParentKernel(kernel)
	mapped, reason := p.classifyVerbMapping("/teleport")
	if mapped || reason != "/unknown_verb" {
		t.Errorf("classify(/teleport) = (%v, %q), want (false, /unknown_verb)", mapped, reason)
	}
	if mapped, _ := p.classifyVerbMapping(""); mapped {
		t.Error("classify(\"\") must not map")
	}
}

// AddTask is exported and the kernel arrives later via SetParentKernel: the
// agenda write must land with or without a kernel attached.
func TestPlannerAddTask_WithoutKernel(t *testing.T) {
	p := NewSessionPlannerShard()
	id := p.AddTask("kernel-less task", 1)
	if id == "" {
		t.Fatal("AddTask returned no ID")
	}
	if agenda := p.GetAgenda(); len(agenda) != 1 || agenda[0].Description != "kernel-less task" {
		t.Fatalf("agenda = %v, want the one task", agenda)
	}
}
