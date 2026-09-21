package campaign

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// Acceptance is a campaign's deterministic witness: a command the user named,
// run once every phase is done. Exit 0 is the only pass.
//
// It exists because a campaign made of model-judged tasks can report success
// over failing checks. Observed 2026-09-21: "Campaign completed successfully"
// with 17 structural problems in the corpus the campaign had just written and
// every /verify task green. Nothing in the campaign could tell, because nothing
// in it was judged by anything but a model.
type Acceptance struct {
	// Command is the argv, run in the workspace. It is never joined into a
	// shell string: the executor's binary allowlist sees Command[0].
	Command []string `json:"command"`
	// Rounds is every run, in order. Its length is the round count the policy
	// compares with campaign_acceptance_limit.
	Rounds []AcceptanceRound `json:"rounds,omitempty"`
}

// AcceptanceRound is one run of the acceptance command.
type AcceptanceRound struct {
	Round    int       `json:"round"`
	Passed   bool      `json:"passed"`
	ExitCode int       `json:"exit_code"`
	At       time.Time `json:"at"`
	// OutputPath holds the command's whole output, workspace-relative. The
	// remediation task quotes it; the campaign file does not carry it.
	OutputPath string `json:"output_path,omitempty"`
}

// ToFacts projects the witness and its rounds. A nil Acceptance is no facts:
// campaign_acceptance_unmet cannot derive, and completion is the phases'.
func (a *Acceptance) ToFacts(campaignID string) []core.Fact {
	if a == nil || len(a.Command) == 0 {
		return nil
	}
	facts := []core.Fact{{
		Predicate: "campaign_acceptance",
		Args:      []any{campaignID, strings.Join(a.Command, " ")},
	}}
	for _, r := range a.Rounds {
		verdict := types.MangleAtom("/fail")
		if r.Passed {
			verdict = types.MangleAtom("/pass")
		}
		facts = append(facts, core.Fact{
			Predicate: "campaign_acceptance_result",
			Args:      []any{campaignID, int64(r.Round), verdict},
		})
	}
	return facts
}

// acceptanceOutcome is what the execution loop does after settleAcceptance.
type acceptanceOutcome int

const (
	// acceptanceSatisfied: no witness declared, or it has passed. Complete.
	acceptanceSatisfied acceptanceOutcome = iota
	// acceptanceRemediating: the witness failed and a remediation phase was
	// appended. The loop continues; the witness runs again when it is done.
	acceptanceRemediating
	// acceptanceBlocked: the policy derived campaign_blocked /acceptance_failed.
	acceptanceBlocked
)

// settleAcceptance runs when every phase is done. Whether the witness is due,
// and whether the campaign is blocked on it, are the kernel's derivations
// (campaign_acceptance_due, campaign_blocked); this function runs the command,
// records the round as a fact, and acts on what is derived next.
func (o *Orchestrator) settleAcceptance(ctx context.Context) (acceptanceOutcome, error) {
	o.mu.RLock()
	declared := o.campaign != nil && o.campaign.Acceptance != nil && len(o.campaign.Acceptance.Command) > 0
	o.mu.RUnlock()
	if !declared {
		return acceptanceSatisfied, nil
	}

	ran := false
	if o.acceptanceDerived("campaign_acceptance_due") {
		if err := o.runAcceptanceRound(ctx); err != nil {
			return acceptanceBlocked, err
		}
		ran = true
	}
	if o.acceptanceDerived("campaign_accepted") {
		return acceptanceSatisfied, nil
	}
	if o.getCampaignBlockReason() != "" {
		return acceptanceBlocked, nil
	}
	if !ran {
		// Unmet, not exhausted, and not due: the kernel still sees a phase the
		// campaign calls done. Remediating here would answer a round that never
		// ran, so the disagreement is reported instead.
		return acceptanceBlocked, fmt.Errorf("acceptance is declared and neither due, passed nor exhausted: the kernel's phase facts disagree with the campaign's")
	}
	if err := o.appendAcceptanceRemediation(); err != nil {
		return acceptanceBlocked, err
	}
	return acceptanceRemediating, nil
}

// acceptanceDerived reports whether predicate holds for the current campaign.
func (o *Orchestrator) acceptanceDerived(predicate string) bool {
	facts, err := o.kernel.Query(predicate)
	if err != nil {
		logging.CampaignWarn("acceptance: query %s: %v", predicate, err)
		return false
	}
	o.mu.RLock()
	id := o.campaign.ID
	o.mu.RUnlock()
	for _, f := range facts {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == id {
			return true
		}
	}
	return false
}

// runAcceptanceRound runs the command once and records the round: on the
// campaign, on disk (the whole output) and in the kernel.
func (o *Orchestrator) runAcceptanceRound(ctx context.Context) error {
	if o.executor == nil {
		return fmt.Errorf("campaign declares an acceptance command and has no executor to run it")
	}
	o.mu.RLock()
	argv := append([]string(nil), o.campaign.Acceptance.Command...)
	round := len(o.campaign.Acceptance.Rounds) + 1
	campaignID := o.campaign.ID
	o.mu.RUnlock()

	logging.Campaign("Acceptance round %d: %s", round, strings.Join(argv, " "))
	// No Limits: the executor applies the user's execution.default_timeout.
	res, execErr := o.executor.Execute(ctx, tactile.Command{
		Binary:           argv[0],
		Arguments:        argv[1:],
		WorkingDirectory: o.workspace,
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	output, exitCode := "", -1
	if res != nil {
		output, exitCode = res.Output(), res.ExitCode
	}
	if execErr != nil {
		// A command that could not run is a failed round, stated as one: a
		// witness nobody can run must not read as a pass, and must not loop.
		output = fmt.Sprintf("the acceptance command could not be run: %v\n%s", execErr, output)
	}
	passed := execErr == nil && exitCode == 0

	rel := path.Join(".nerd", "campaigns", strings.TrimPrefix(campaignID, "/"), "acceptance", fmt.Sprintf("round_%d.txt", round))
	abs := filepath.Join(o.workspace, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("acceptance round %d: %w", round, err)
	}
	if err := os.WriteFile(abs, []byte(output), 0o644); err != nil {
		return fmt.Errorf("acceptance round %d: %w", round, err)
	}

	o.mu.Lock()
	o.campaign.Acceptance.Rounds = append(o.campaign.Acceptance.Rounds, AcceptanceRound{
		Round: round, Passed: passed, ExitCode: exitCode, At: time.Now(), OutputPath: rel,
	})
	facts := o.campaign.Acceptance.ToFacts(campaignID)
	o.persistCampaign(fmt.Sprintf("acceptance round %d", round))
	o.mu.Unlock()

	if err := o.kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("acceptance round %d: record result: %w", round, err)
	}
	if passed {
		logging.Campaign("Acceptance round %d passed", round)
		return nil
	}
	logging.Get(logging.CategoryCampaign).Warn("Acceptance round %d FAILED (exit %d); output in %s", round, exitCode, rel)
	o.emitEvent(EventCheckpointFailed, "", "", fmt.Sprintf("acceptance round %d failed (exit %d); output in %s", round, exitCode, rel), nil)
	return nil
}

// appendAcceptanceRemediation adds one phase whose one task is the failed
// round's output. The task is told what the campaign wrote and nothing about
// the cause: the witness's output is the symptom, and it is the whole brief.
func (o *Orchestrator) appendAcceptanceRemediation() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	c := o.campaign
	if len(c.Acceptance.Rounds) == 0 {
		return fmt.Errorf("acceptance remediation with no failed round")
	}
	last := c.Acceptance.Rounds[len(c.Acceptance.Rounds)-1]
	output, err := os.ReadFile(filepath.Join(o.workspace, filepath.FromSlash(last.OutputPath)))
	if err != nil {
		return fmt.Errorf("acceptance remediation: %w", err)
	}

	previous, err := cloneCampaign(c)
	if err != nil {
		return fmt.Errorf("acceptance remediation: %w", err)
	}
	suffix := strings.TrimPrefix(c.ID, "/campaign_")
	phaseID := fmt.Sprintf("/phase_%s_accept_%d", suffix, last.Round)
	writeSet := campaignWriteSet(c)
	scope := "the files this campaign wrote"
	if len(writeSet) > 0 {
		scope = strings.Join(writeSet, ", ")
	}
	profile := ""
	if n := len(c.Phases); n > 0 {
		profile = c.Phases[n-1].ContextProfile
	}
	description := fmt.Sprintf(
		"Every phase of this campaign is done and its acceptance check still fails (round %d, exit %d).\n\n"+
			"Acceptance command, run in the workspace root:\n    %s\n\nIts output:\n\n%s\n\n"+
			"Bring the check to a pass by changing %s. The check is the judge: exit 0 is the only pass, and it runs again when this task is done. "+
			"Do not delete content or weaken a true statement to get there, and do not touch the checker.",
		last.Round, last.ExitCode, strings.Join(c.Acceptance.Command, " "), indentBlock(string(output)), scope)

	c.Phases = append(c.Phases, Phase{
		ID:             phaseID,
		CampaignID:     c.ID,
		Name:           fmt.Sprintf("Acceptance remediation %d", last.Round),
		Order:          len(c.Phases),
		Status:         PhasePending,
		ContextProfile: profile,
		Objectives: []PhaseObjective{{
			Type:        ObjectiveModify,
			Description: fmt.Sprintf("the acceptance command exits 0: %s", strings.Join(c.Acceptance.Command, " ")),
			// The witness itself verifies this phase, the moment it is done.
			VerificationMethod: VerifyNone,
		}},
		EstimatedTasks:      1,
		EstimatedComplexity: "/medium",
		Tasks: []Task{{
			ID:          fmt.Sprintf("/task_%s_accept_%d_0", suffix, last.Round),
			PhaseID:     phaseID,
			Description: description,
			Status:      TaskPending,
			Type:        TaskTypeFileModify,
			PlannedType: TaskTypeFileModify,
			Priority:    PriorityHigh,
			WriteSet:    writeSet,
		}},
	})
	c.TotalPhases = len(c.Phases)
	c.TotalTasks++

	if err := syncCampaignFacts(o.kernel, previous, c, fmt.Sprintf("acceptance round %d failed: remediation phase appended", last.Round)); err != nil {
		return fmt.Errorf("acceptance remediation: %w", err)
	}
	o.persistCampaign(fmt.Sprintf("acceptance remediation %d", last.Round))
	logging.Campaign("Acceptance remediation phase appended: %s (scope: %s)", phaseID, scope)
	return nil
}

// campaignWriteSet is every path the campaign's tasks declared they write,
// deduplicated and sorted: the remediation's scope.
func campaignWriteSet(c *Campaign) []string {
	seen := map[string]bool{}
	var out []string
	for i := range c.Phases {
		for j := range c.Phases[i].Tasks {
			for _, p := range c.Phases[i].Tasks[j].WriteSet {
				if p = strings.TrimSpace(p); p != "" && !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func indentBlock(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\r\n"), "\n")
	for i, line := range lines {
		lines[i] = "    " + strings.TrimRight(line, "\r")
	}
	return strings.Join(lines, "\n")
}
