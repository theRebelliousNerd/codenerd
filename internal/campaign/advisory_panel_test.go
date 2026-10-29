package campaign

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/consultation"
)

func TestParseAdvisoryVote_ExplicitFieldOnly(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want VoteType
	}{
		{"approve prose mentioning a rejected alternative",
			"VOTE: APPROVE\nCONFIDENCE: 85\nREASONING:\nWe rejected the alternative of a global lock; this plan avoids it.", VoteApprove},
		{"markdown bold and bullet", "- **VOTE:** request_changes", VoteRequestChanges},
		{"spaced enum", "Vote: approve with notes", VoteApproveWithNotes},
		{"no vote field", "I would reject this plan outright.", VoteUnknown},
		{"invalid value", "VOTE: MAYBE", VoteUnknown},
		{"echoed template", "VOTE: APPROVE | APPROVE_WITH_NOTES | REQUEST_CHANGES | REJECT | ABSTAIN", VoteUnknown},
		{"contradictory duplicates", "VOTE: APPROVE\n...\nVOTE: REJECT", VoteUnknown},
		{"agreeing duplicates", "VOTE: REJECT\nVOTE: reject", VoteReject},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseAdvisoryVote(tc.raw); got != tc.want {
				t.Fatalf("parseAdvisoryVote = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseAdvisoryResponse_SectionsFromRaw(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)
	resp := board.parseAdvisoryResponse(consultation.Response{
		Specialist: "reviewer",
		Confidence: 0.8,
		Raw: "VOTE: APPROVE_WITH_NOTES\nCONFIDENCE: 80\nREASONING: sound overall\n" +
			"CONCERNS:\n1. ordering of t2\nSUGGESTIONS:\n- add a test\nCAVEATS:\n* flaky CI",
	})
	if resp.Vote != VoteApproveWithNotes || resp.Reasoning != "sound overall" {
		t.Fatalf("vote/reasoning = %s/%q", resp.Vote, resp.Reasoning)
	}
	if len(resp.Concerns) != 1 || len(resp.Suggestions) != 1 || len(resp.Caveats) != 1 {
		t.Fatalf("sections = %+v", resp)
	}
}

func TestSynthesizeVotes_UnknownVoteIsMissingNotApproval(t *testing.T) {
	board := NewShardAdvisoryBoard(nil)
	s := board.SynthesizeVotes([]AdvisoryResponse{
		{AdvisorName: "coder", Vote: VoteApprove, Confidence: 0.9},
		{AdvisorName: "tester", Vote: VoteUnknown, Confidence: 0.95},
	})
	if s.Approved || !s.Incomplete || len(s.Missing) != 1 || s.Missing[0].Advisor != "tester" {
		t.Fatalf("synthesis = %+v, want an incomplete unapproved panel missing tester", s)
	}
	if !strings.Contains(s.Summary, "Missing Reviews") {
		t.Fatalf("summary does not name the missing review: %s", s.Summary)
	}
}

// rejectAndTimeoutSpawner: coder rejects, tester never answers, others approve.
type rejectAndTimeoutSpawner struct{}

func (rejectAndTimeoutSpawner) SpawnConsultation(ctx context.Context, spec, task string) (string, error) {
	switch spec {
	case "coder":
		return "VOTE: REJECT\nCONFIDENCE: 90\nREASONING:\nthe write sets of t1 and t2 collide", nil
	case "tester":
		<-ctx.Done()
		return "", ctx.Err()
	}
	return "VOTE: APPROVE\nCONFIDENCE: 80\nREASONING:\nfine", nil
}

func manifestCampaign() *Campaign {
	return &Campaign{
		ID: "/campaign_panel", Goal: "refactor the store", TotalTasks: 3,
		Phases: []Phase{{
			ID: "/phase_a", Name: "A",
			Objectives: []PhaseObjective{{Type: ObjectiveModify, Description: "split the store"}},
			Tasks: []Task{
				{ID: "/t1", Type: TaskTypeFileModify, Description: "edit a", WriteSet: []string{"a.go"}},
				{ID: "/t2", Type: TaskTypeFileModify, Description: "edit b", DependsOn: []string{"/t1"}, WriteSet: []string{"a.go"}},
				{ID: "/t3", Type: TaskTypeTestRun, Description: "test"},
			},
		}},
	}
}

// Through the production gate: the real consultation engine, the board and
// the risk gate's grading. One advisor's timeout must not reduce the coder's
// rejection to a generic missing-review concern.
func TestRunAdvisoryRiskGate_RejectionSurvivesTimeoutAndReportsCoverage(t *testing.T) {
	t.Parallel()
	board := NewShardAdvisoryBoard(consultation.New(rejectAndTimeoutSpawner{}))
	cfg := DefaultAdvisoryConfig()
	cfg.ConsultTimeout = 50 * time.Millisecond
	board.WithConfig(cfg)
	o := &Orchestrator{
		campaign:      manifestCampaign(),
		advisoryBoard: board,
		policy:        config.CampaignPolicy{AdvisoryManifestMaxTasks: 2},
	}

	result, concerns := o.runAdvisoryRiskGate(context.Background(), nil, nil)
	if result.Outcome != RiskGateOutcomeBlocked {
		t.Fatalf("outcome = %s, want blocked", result.Outcome)
	}
	if len(concerns) != 1 || concerns[0].severity != riskConcernBlocking || !strings.Contains(concerns[0].detail, "collide") {
		t.Fatalf("concerns = %+v, want the coder's specific blocking rejection", concerns)
	}
	if got := result.Data["missing_reviews"]; got != 1 {
		t.Fatalf("missing_reviews = %v, want 1", got)
	}
	if result.Data["manifest_tasks_examined"] != 2 || result.Data["manifest_tasks_unavailable"] != 1 {
		t.Fatalf("coverage data = %v, want 2 examined and 1 unavailable", result.Data)
	}
}

func TestBuildPlanManifest_BoundAndRender(t *testing.T) {
	m := buildPlanManifest(manifestCampaign(), 2)
	cov := m.Coverage()
	if len(cov.Examined) != 2 || len(cov.Unavailable) != 1 || cov.Unavailable[0] != "/t3" || len(cov.NotAssigned) != 0 {
		t.Fatalf("coverage = %+v", cov)
	}
	var sb strings.Builder
	m.render(&sb)
	out := sb.String()
	for _, want := range []string{"/t2", "depends_on: /t1", "writes: a.go", "split the store", "NOT SHOWN: 1 task"} {
		if !strings.Contains(out, want) {
			t.Fatalf("manifest rendering lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "/t3 [") {
		t.Fatalf("a task beyond the bound was rendered:\n%s", out)
	}
}

// The board's question no longer embeds a second answer format: the VOTE
// contract travels as the consultation's response contract.
func TestConsultAdvisors_SendsVoteContract(t *testing.T) {
	var prompts []string
	spawner := consultationSpawnerFunc(func(_ context.Context, _ string, task string) (string, error) {
		prompts = append(prompts, task)
		return "VOTE: APPROVE\nCONFIDENCE: 90", nil
	})
	board := NewShardAdvisoryBoard(consultation.New(spawner))
	cfg := DefaultAdvisoryConfig()
	cfg.EnabledAdvisors = []string{"coder"}
	board.WithConfig(cfg)
	responses, err := board.ConsultAdvisors(context.Background(), AdvisoryRequest{CampaignID: "/c", Goal: "g"})
	if err != nil || len(responses) != 1 || responses[0].Vote != VoteApprove {
		t.Fatalf("responses = %+v err = %v", responses, err)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "advisory-vote/v1") || strings.Contains(prompts[0], "ADVICE:") {
		t.Fatalf("prompt carries conflicting formats:\n%s", prompts[0])
	}
}

type consultationSpawnerFunc func(ctx context.Context, spec, task string) (string, error)

func (f consultationSpawnerFunc) SpawnConsultation(ctx context.Context, spec, task string) (string, error) {
	return f(ctx, spec, task)
}
