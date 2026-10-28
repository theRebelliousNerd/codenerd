package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/problemlog"
	"codenerd/internal/turnjudge"
)

// "Debug yourself" is a chain the kernel lays out, not one verb
// (policy/self_diagnosis.mg):
//
//  1. query: the problems log is projected into the kernel (self_problem,
//     policy/self_problems.mg), whether or not /debug-self is on;
//  2. analyze and root-cause: a read-only shard works through every hot
//     problem in codeNERD's own source, in the background;
//  3. fix: its findings come back as a turn of their own, and the work mode
//     decides whether the fix runs, is proposed, or waits for go.

// selfDiagnosisFollowUp opens the chain's fix step; the diagnosis is appended.
const selfDiagnosisFollowUp = "Fix the root causes this self-diagnosis of codeNERD found, in codeNERD's own source. Change only what each finding names, with tests. If it names no defect in codeNERD's own source (the failures came from the user's input or from outside), change nothing and say so in a sentence."

// latestProblemsLog is the log debug logging is writing now, else the newest
// one under .nerd/logs: a session without debug logging still diagnoses the
// last run that had it.
func (m Model) latestProblemsLog() string {
	if path, _, _ := logging.ProblemsSnapshot(); path != "" && strings.HasPrefix(path, filepath.Join(m.workspace, ".nerd")+string(filepath.Separator)) {
		return path
	}
	matches, err := filepath.Glob(filepath.Join(m.workspace, ".nerd", "logs", "*_problems.log"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, k int) bool {
		a, errA := os.Stat(matches[i])
		b, errB := os.Stat(matches[k])
		if errA != nil || errB != nil {
			return matches[i] > matches[k]
		}
		return a.ModTime().After(b.ModTime())
	})
	return matches[0]
}

// selfDiagnosisJob is the chain's analysis step: the problems log projected
// into the kernel, then one task naming every problem the kernel calls hot,
// each with its whole first message. ok is false, with the reason, when there
// is nothing to diagnose.
func (m Model) selfDiagnosisJob(ctx context.Context, shard string) (job backgroundJob, ok bool, reason string) {
	if m.kernel == nil {
		return job, false, "no kernel to hold the evidence"
	}
	path := m.latestProblemsLog()
	if path == "" {
		return job, false, "no problems log exists yet: turn on `/debug-self` (or logging.debug_mode) so codeNERD logs its own warnings and errors, reproduce the problem, then ask again"
	}
	rep, err := problemlog.ReadFile(path, problemlog.Options{Skip: isSelfNote})
	if err != nil {
		return job, false, fmt.Sprintf("reading %s: %v", path, err)
	}
	if err := config.EnsureParams(m.kernel, m.Config.Logging.Params()); err != nil {
		return job, false, fmt.Sprintf("logging thresholds: %v", err)
	}
	if err := problemlog.Replace(m.kernel, rep); err != nil {
		return job, false, fmt.Sprintf("projecting the problems log: %v", err)
	}
	hot, err := rep.Hot(m.kernel)
	if err != nil {
		return job, false, fmt.Sprintf("asking the kernel for hot problems: %v", err)
	}
	if len(hot) == 0 {
		return job, false, fmt.Sprintf("%s holds nothing hot (%s): no errors, and no warning repeated often enough (logging.self_problem_warn_repeats)", path, rep.Summary())
	}

	defects, around := m.triageProblems(ctx, hot)

	var b strings.Builder
	fmt.Fprintf(&b, "Diagnose codeNERD itself from its own problems log (%s; %s). Read-only: change nothing.\n", path, rep.Summary())
	b.WriteString("For each problem below: find where codeNERD's source raises it, trace the root cause, and give file:line, the evidence, and the fix you would make. Say when two problems share a cause.\n")
	n := writeDiagnosisProblems(&b, defects, 0)
	if len(around) > 0 {
		b.WriteString("\nThese look like failures around codeNERD (network, providers, configuration, the machine) rather than defects in it. Confirm that briefly; fix codeNERD only where it handles them wrongly:\n")
		writeDiagnosisProblems(&b, around, n)
	}
	return backgroundJob{
		ID:       fmt.Sprintf("self-diagnosis-%d", time.Now().UnixNano()),
		Shard:    shard,
		Task:     b.String(),
		Started:  time.Now(),
		FollowUp: selfDiagnosisFollowUp,
	}, true, ""
}

// triageProblems is the chain's Jev triage, when on: Jev judges each hot
// problem, the judgments reach the kernel (self_problem_triage), and the
// kernel's self_problem_around lists a failure of codeNERD's surroundings
// (network, providers, config, machine) apart from the defects worked first.
// Off, or failed, every problem is a defect in the log's order.
func (m Model) triageProblems(ctx context.Context, hot []problemlog.Problem) (defects, around []problemlog.Problem) {
	if m.jev == nil || !m.Config.TypeSafe.Uses().DiagnosisTriage {
		return hot, nil
	}
	log := logging.Get(logging.CategoryKernel)
	// Last chain's judgments go first: a failed triage must not leave them
	// splitting this one.
	if err := m.kernel.Retract(turnjudge.TriagePredicate); err != nil {
		log.Warn("%s jev triage not run, earlier judgments not retracted: %v", selfDebugMarker, err)
		return hot, nil
	}
	probs, err := turnjudge.TriageProblems(m.jevContext(ctx), m.jev, hot, m.Config.TypeSafe.Uses().BatchSize)
	if err != nil {
		log.Warn("%s jev triage failed, the log's order stands: %v", selfDebugMarker, err)
		return hot, nil
	}
	if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
		log.Warn("%s jev thresholds not asserted, the log's order stands: %v", selfDebugMarker, err)
		return hot, nil
	}
	if err := m.kernel.AssertBatch(turnjudge.TriageFacts(hot, probs)); err != nil {
		log.Error("%s assert jev triage: %v", selfDebugMarker, err)
		return hot, nil
	}
	defects, around, err = turnjudge.SplitDefects(m.kernel, hot)
	if err != nil {
		log.Warn("%s jev triage unread, the log's order stands: %v", selfDebugMarker, err)
		return hot, nil
	}
	return defects, around
}

// writeProblems numbers problems from after, each with its whole first message.
func writeDiagnosisProblems(b *strings.Builder, problems []problemlog.Problem, after int) int {
	for i, p := range problems {
		fmt.Fprintf(b, "\n%d. %s [%s] x%d, %s to %s\n%s\n", after+i+1, strings.ToUpper(p.Level), p.Category, p.Count,
			p.First.Format(time.TimeOnly), p.Last.Format(time.TimeOnly), p.Sample)
	}
	return after + len(problems)
}
