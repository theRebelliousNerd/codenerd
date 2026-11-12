package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/problemlog"
	"codenerd/internal/turnjudge"
	"codenerd/internal/types"
)

// The chain's verdict: whether a finished self-diagnosis found anything of
// codeNERD's to fix, and so whether its fix turn runs (policy/self_diagnosis.mg).
//
// Jev reads the diagnosis and judges, per target the job covered, whether it
// names a fix in codeNERD's own source (turnjudge.JudgeDiagnosis); the call
// runs in the job's own goroutine, after its work returns, so the Update
// thread never waits on it. finishBackground hands the judgments to the
// kernel, which derives self_diagnosis_fix_due and which targets are fix
// targets or settled as nothing to fix. Go only asks. Jev off, or its call
// failed: no verdict, and the kernel runs the fix turn as before.

// diagnosisVerdict is Jev's verdict on a finished diagnosis, carried on its
// done message. Err is a failed call: no evidence, never a "no".
type diagnosisVerdict struct {
	Targets []turnjudge.DiagnosisTarget
	Probs   []float64
	Err     error
}

// diagnosisLane is what the job was looking for.
func diagnosisLane(job backgroundJob) turnjudge.DiagnosisLane {
	if job.SelfRepair == problemlog.LaneOptimize {
		return turnjudge.DiagnosisSpeed
	}
	return turnjudge.DiagnosisDefect
}

// diagnosisTargets is what a job diagnosed: the problems it was built from,
// or, for a self-repair job resumed after a restart (which keeps only its
// id), its targets in the ledger, judged by signature. None means the verdict
// is on the whole report.
func (m Model) diagnosisTargets(job backgroundJob) []turnjudge.DiagnosisTarget {
	if len(job.diagnosed) > 0 {
		return job.diagnosed
	}
	if job.SelfRepair == "" || m.selfRepair == nil {
		return nil
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		return nil
	}
	var out []turnjudge.DiagnosisTarget
	for _, a := range ledger.JobTargets(job.ID) {
		out = append(out, turnjudge.DiagnosisTarget{Key: a.Target})
	}
	return out
}

// judgeDiagnosis asks Jev for the verdict on a diagnosis that returned, when
// the use is on; nil when it is off (nothing was asked).
func (m Model) judgeDiagnosis(ctx context.Context, job backgroundJob, result string) *diagnosisVerdict {
	if job.FollowUp == "" || m.jev == nil || m.Config.TypeSafe == nil || !m.Config.TypeSafe.Uses().DiagnosisVerdict {
		return nil
	}
	v := &diagnosisVerdict{Targets: m.diagnosisTargets(job)}
	v.Probs, v.Err = turnjudge.JudgeDiagnosis(m.jevContext(ctx), m.jev, diagnosisLane(job), result, v.Targets, m.Config.TypeSafe.Uses().BatchSize)
	return v
}

// decideDiagnosisFix asks the kernel whether the chain's fix turn is due for
// the finished job: its verdict facts go in, the decision comes out, and the
// job's facts are retracted again. Without a kernel, or when the kernel
// cannot be asked, the fix turn runs as before the verdict.
func (m Model) decideDiagnosisFix(job backgroundJob, v *diagnosisVerdict) turnjudge.FixDecision {
	runs := turnjudge.FixDecision{Due: true}
	if m.kernel == nil {
		return runs
	}
	log := logging.Get(logging.CategoryKernel)
	facts := []types.Fact{{Predicate: turnjudge.FinishedPredicate, Args: []any{types.MangleString(job.ID)}}}
	switch {
	case v == nil:
	case v.Err != nil:
		log.Warn("%s jev verdict on %s failed, its fix turn runs as before: %v", selfDebugMarker, job.ID, v.Err)
	default:
		if err := config.EnsureParams(m.kernel, m.Config.TypeSafe.Params()); err != nil {
			log.Warn("%s jev thresholds not asserted, the fix turn of %s runs as before: %v", selfDebugMarker, job.ID, err)
			break
		}
		facts = append(facts, turnjudge.VerdictFacts(job.ID, v.Targets, v.Probs)...)
	}
	defer func() {
		for _, pred := range []string{turnjudge.FinishedPredicate, turnjudge.VerdictPredicate} {
			if err := m.kernel.RetractFact(core.Fact{Predicate: pred, Args: []any{types.MangleString(job.ID)}}); err != nil {
				log.Debug("%s retract %s for %s: %v", selfDebugMarker, pred, job.ID, err)
			}
		}
	}()
	if err := m.kernel.AssertBatch(facts); err != nil {
		log.Error("%s assert the verdict on %s: %v; its fix turn runs as before", selfDebugMarker, job.ID, err)
		return runs
	}
	d, err := turnjudge.DecideFix(m.kernel, job.ID)
	if err != nil {
		log.Error("%s ask whether the fix of %s is due: %v; it runs as before", selfDebugMarker, job.ID, err)
		return runs
	}
	sort.Strings(d.Fix)
	sort.Strings(d.NoDefect)
	return d
}

// noDefectLine is the one line the user sees when a diagnosis found nothing
// of codeNERD's to fix for some or all of its targets.
func noDefectLine(job backgroundJob, d turnjudge.FixDecision) string {
	what := "the problems it covered"
	if n := len(d.NoDefect); n > 0 && d.NoDefect[0] != turnjudge.DiagnosisJobKey {
		labels := make([]string, 0, n)
		for _, t := range d.NoDefect {
			labels = append(labels, selfRepairTargetLabel(t))
		}
		what = strings.Join(labels, ", ")
	}
	reason := "the diagnosis above concludes that " + what + " need no change in codeNERD's own source"
	if diagnosisLane(job) == turnjudge.DiagnosisSpeed {
		reason = "the analysis above names no change to codeNERD's own code that would make " + what + " faster"
	}
	if d.Due {
		return fmt.Sprintf("_Self-diagnosis found nothing of codeNERD's to fix for part of it: %s. Its fix turn covers the rest._", reason)
	}
	return fmt.Sprintf("_Self-diagnosis found nothing of codeNERD's to fix: %s. No fix turn was started._", reason)
}

// settleNoDefect records, for a self-repair job, the targets its diagnosis
// found nothing to fix for, each with the evidence it was judged on (the
// kernel's self_repair_evidence_count), as /no_defect. A job-level verdict
// covers every target of the job. When no fix turn follows, a target the
// verdict did not reach is stopped, not failed: nothing was tried on it.
func (m Model) settleNoDefect(job backgroundJob, d turnjudge.FixDecision) {
	if job.SelfRepair == "" || m.selfRepair == nil || len(d.NoDefect) == 0 {
		return
	}
	ledger, err := m.selfRepair.ledgerFor(m.workspace)
	if err != nil {
		return
	}
	now := m.selfRepair.clock()
	counts := m.evidenceCounts()
	evidence := map[string]int{}
	for _, t := range d.NoDefect {
		if t == turnjudge.DiagnosisJobKey {
			for _, a := range ledger.JobTargets(job.ID) {
				evidence[a.Target] = counts[a.Target]
			}
			continue
		}
		evidence[t] = counts[t]
	}
	log := logging.Get(logging.CategoryKernel)
	if _, err := ledger.NoDefect(job.ID, evidence, "its diagnosis found nothing of codeNERD's to fix", now); err != nil {
		log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
	}
	if !d.Due {
		if _, err := ledger.EndJob(job.ID, problemlog.RepairStopped, "its diagnosis was not judged for this target, and no fix turn followed", now); err != nil {
			log.Error("%s self-repair ledger: %v", selfDebugMarker, err)
		}
	}
	if m.kernel != nil {
		if err := problemlog.ReplaceRepair(m.kernel, ledger, now); err != nil {
			log.Warn("%s self-repair facts: %v", selfDebugMarker, err)
		}
	}
}

// evidenceCounts is the kernel's self_repair_evidence_count, the largest per
// target (a signature can be logged at more than one level).
func (m Model) evidenceCounts() map[string]int {
	out := map[string]int{}
	if m.kernel == nil {
		return out
	}
	rows, err := m.kernel.Query("self_repair_evidence_count")
	if err != nil {
		return out
	}
	for _, r := range rows {
		if len(r.Args) != 2 {
			continue
		}
		if n, ok := r.Args[1].(int64); ok && int(n) > out[types.ExtractString(r.Args[0])] {
			out[types.ExtractString(r.Args[0])] = int(n)
		}
	}
	return out
}

// =============================================================================
// THE FIX STEP
// =============================================================================

// chainStep is the self-diagnosis chain's fix step: a turn whose request is
// the step's instruction and whose evidence is the diagnosis it acts on. It
// runs with the intent the chain already knows (the fix, of codeNERD's own
// source, on the files the diagnosis cites), never re-perceived: perception
// reading instruction and diagnosis together took the diagnosis's own
// sentence for the request, the decomposer split it into a plan, and a /fix
// with no target perception could name stalled on clarification.
type chainStep struct {
	Instruction string
	Evidence    string
	Verb        string // /fix, or /refactor on the optimize lane
	Target      string
	// Autonomous: codeNERD started it itself (self-repair); a message of the
	// user's goes before it in the queue.
	Autonomous bool
}

// input is the turn's text: the instruction, then the diagnosis.
func (s chainStep) input() string {
	return s.Instruction + "\n\n" + s.Evidence
}

// intent is the step's preset intent: a mutation of what the diagnosis
// cites, with the instruction and the diagnosis as its constraint.
func (s chainStep) intent() perception.Intent {
	return perception.Intent{
		Category:   "/mutation",
		Verb:       s.Verb,
		Target:     s.Target,
		Constraint: s.Instruction + "\n\nThe diagnosis this acts on:\n" + s.Evidence,
		Confidence: 1,
	}
}

// ownSourceTarget names the fix's target when the diagnosis cites no file
// of the workspace.
const ownSourceTarget = "codeNERD's own source"

// citedFile is a workspace-relative source path, with an optional :line.
var citedFile = regexp.MustCompile(`[A-Za-z0-9_./-]+\.(?:go|mg|yaml|yml|json)(?::\d+)?`)

// citedFiles is every file of the workspace the diagnosis cites, in order of
// first mention.
func citedFiles(workspace, diagnosis string) []string {
	seen := map[string]bool{}
	var out []string
	for _, ref := range citedFile.FindAllString(diagnosis, -1) {
		path, _, _ := strings.Cut(ref, ":")
		path = strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")
		if seen[path] || strings.HasPrefix(path, "../") || filepath.IsAbs(path) {
			continue
		}
		if info, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(path))); err != nil || info.IsDir() {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

// fixStep builds the chain's fix step for a finished job: its follow-up
// instruction (narrowed to the fix targets when the verdict settled some as
// nothing to fix) and its diagnosis.
func (m Model) fixStep(job backgroundJob, result string, d turnjudge.FixDecision) chainStep {
	step := chainStep{Instruction: job.FollowUp, Evidence: result, Verb: "/fix", Target: ownSourceTarget, Autonomous: job.SelfRepair != ""}
	if job.SelfRepair == problemlog.LaneOptimize {
		step.Verb = "/refactor"
	}
	if files := citedFiles(m.workspace, result); len(files) > 0 {
		step.Target = strings.Join(files, ", ")
	}
	if len(d.NoDefect) > 0 && d.NoDefect[0] != turnjudge.DiagnosisJobKey {
		labels := make([]string, 0, len(d.NoDefect))
		for _, t := range d.NoDefect {
			labels = append(labels, truncateSelfSignature(t))
		}
		step.Instruction += " The diagnosis found nothing of codeNERD's to fix for these, so leave them: " + strings.Join(labels, "; ") + "."
	}
	return step
}
