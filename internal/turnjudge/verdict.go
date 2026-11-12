package turnjudge

import (
	"context"
	"fmt"

	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// The self-diagnosis verdict: once the read-only diagnosis of the
// self-diagnosis chain returns, Jev reads its report and judges, for each
// target the job diagnosed, whether the report names a defect in codeNERD's
// own source that a code change would fix (the repair lane), or a change to
// codeNERD's code that would make the operation faster (the optimize lane).
// The answers reach the kernel as self_diagnosis_verdict
// (policy/self_diagnosis.mg), which derives whether the chain's fix turn is
// due (self_diagnosis_fix_due) and which targets it covers. Jev judges; the
// kernel decides; a failed call is an error and no facts, never a "no".

// DiagnosisLane is what the diagnosed job was looking for.
type DiagnosisLane int

const (
	// DiagnosisDefect: a defect in codeNERD's own source (the user's chain
	// and the self-repair repair lane).
	DiagnosisDefect DiagnosisLane = iota
	// DiagnosisSpeed: a change that makes one operation faster (the
	// self-repair optimize lane).
	DiagnosisSpeed
)

// DiagnosisTarget is one thing a diagnosis covered: a hot problem (its
// signature, and its level, category and whole first message when the chat
// still has them) or, on the optimize lane, one timed operation. A target
// with no message is judged by its key alone: a job resumed after a restart
// knows only its signatures.
type DiagnosisTarget struct {
	Key      string
	Level    string
	Category string
	Message  string
}

// DiagnosisJobKey is the target key of a verdict on the whole report, asked
// when the job names no targets.
const DiagnosisJobKey = "*"

// OpDiagnosisVerdict is the usage tracker's operation for the verdict.
const OpDiagnosisVerdict = "jev_diagnosis_verdict"

// JudgeDiagnosis asks Jev, in one request (batchSize questions to a request,
// batches in parallel), one question per target over the shared report. No
// targets asks one question about the whole report, under DiagnosisJobKey.
// The probabilities come back in the targets' order.
func JudgeDiagnosis(ctx context.Context, a Asker, lane DiagnosisLane, report string, targets []DiagnosisTarget, batchSize int) ([]float64, error) {
	if len(targets) == 0 {
		targets = []DiagnosisTarget{{Key: DiagnosisJobKey}}
	}
	state := diagnosisState(lane, report)
	parts, err := inBatches(targets, batchSize, func(batch []DiagnosisTarget) ([]float64, error) {
		questions := make(map[string]typesafe.Question, len(batch))
		for i, t := range batch {
			questions[fmt.Sprintf("target_%d", i)] = diagnosisQuestion(lane, t)
		}
		resp, err := a.Ask(typesafe.WithOperation(ctx, OpDiagnosisVerdict), state, questions)
		if err != nil {
			return nil, err
		}
		out := make([]float64, len(batch))
		for i := range batch {
			if out[i], err = resp.Noul(fmt.Sprintf("target_%d", i)); err != nil {
				return nil, err
			}
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	var out []float64
	for _, p := range parts {
		out = append(out, p...)
	}
	return out, nil
}

// diagnosisState is the report, whole, under a name the questions cite.
func diagnosisState(lane DiagnosisLane, report string) map[string]any {
	if lane == DiagnosisSpeed {
		return map[string]any{
			"context":  "codeNERD is a coding agent. A read-only agent studied why one recurring operation of codeNERD takes the time it does, and wrote `analysis`.",
			"analysis": report,
		}
	}
	return map[string]any{
		"context":   "codeNERD is a coding agent. A read-only agent traced problems from codeNERD's own log through codeNERD's source and wrote `diagnosis`.",
		"diagnosis": report,
	}
}

// diagnosisQuestion is one target's question. The criteria name the case
// that caused the live defect: a report that concludes codeNERD handled the
// problem correctly names no defect, however much code it cites.
func diagnosisQuestion(lane DiagnosisLane, t DiagnosisTarget) typesafe.Question {
	if lane == DiagnosisSpeed {
		return typesafe.Noul{
			Instructions: map[string]any{
				"question":  "Does `analysis` name a change to codeNERD's own code that would make `operation` take less time?",
				"operation": t.Key,
			},
			Criteria: &typesafe.NoulCriteria{
				True:  "The analysis points to codeNERD code that wastes time in this operation and says what to change there.",
				False: "The analysis concludes the time is spent waiting on something outside codeNERD's code, such as a model, the network or the disk, or it names no change that would cut the time.",
			},
		}
	}
	criteria := &typesafe.NoulCriteria{
		True:  "The diagnosis points to code of codeNERD's that is wrong for this problem and says what to change there.",
		False: "The diagnosis concludes codeNERD's code handled this problem correctly, or that the cause lies outside codeNERD's code: the network, a model provider, the user's input or configuration, the build, or the machine. Nothing in codeNERD's source should change for it.",
	}
	if t.Key == DiagnosisJobKey {
		return typesafe.Noul{
			Instructions: map[string]any{"question": "Does `diagnosis` conclude that codeNERD's own source code has a defect that a code change would fix?"},
			Criteria:     criteria,
		}
	}
	problem := map[string]any{"signature": t.Key}
	if t.Message != "" {
		problem = map[string]any{"level": t.Level, "category": t.Category, "message": t.Message}
	}
	return typesafe.Noul{
		Instructions: map[string]any{
			"question": "Does `diagnosis` conclude that `problem` comes from a defect in codeNERD's own source code that a code change would fix?",
			"problem":  problem,
		},
		Criteria: criteria,
	}
}

// Verdict facts (policy/self_diagnosis.mg): self_diagnosis_verdict(Job,
// Target, Percent) is Jev's probability that the job's report names a fix in
// codeNERD's own source for the target.
const (
	VerdictPredicate   = "self_diagnosis_verdict"
	FixDuePredicate    = "self_diagnosis_fix_due"
	FixTargetPredicate = "self_diagnosis_fix_target"
	NoDefectPredicate  = "self_diagnosis_no_defect"
	FinishedPredicate  = "self_diagnosis_finished"
)

// VerdictFacts projects one verdict per target, keyed by the job.
func VerdictFacts(job string, targets []DiagnosisTarget, probs []float64) []types.Fact {
	if len(targets) == 0 {
		targets = []DiagnosisTarget{{Key: DiagnosisJobKey}}
	}
	out := make([]types.Fact, 0, len(targets))
	for i, t := range targets {
		if i >= len(probs) {
			break
		}
		out = append(out, types.Fact{Predicate: VerdictPredicate, Args: []any{
			types.MangleString(job), types.MangleString(t.Key), types.PercentFromRatio(probs[i]),
		}})
	}
	return out
}

// FixDecision is what the kernel derived for one finished diagnosis job.
type FixDecision struct {
	// Due: the chain's fix turn runs.
	Due bool
	// Fix: the targets the fix covers (judged a defect, or unsettled); empty
	// with Due when the job was not judged (Jev off or failed).
	Fix []string
	// NoDefect: the targets the report names nothing of codeNERD's to fix.
	NoDefect []string
}

// DecideFix reads the kernel's decision for job.
func DecideFix(k Querier, job string) (FixDecision, error) {
	var d FixDecision
	due, err := k.Query(FixDuePredicate)
	if err != nil {
		return d, err
	}
	for _, r := range due {
		if len(r.Args) == 1 && types.ExtractString(r.Args[0]) == job {
			d.Due = true
		}
	}
	fix, err := k.Query(FixTargetPredicate)
	if err != nil {
		return d, err
	}
	none, err := k.Query(NoDefectPredicate)
	if err != nil {
		return d, err
	}
	d.Fix, d.NoDefect = jobTargets(fix, job), jobTargets(none, job)
	return d, nil
}

// jobTargets is the second column of the rows whose first is job.
func jobTargets(rows []types.Fact, job string) []string {
	var out []string
	for _, r := range rows {
		if len(r.Args) == 2 && types.ExtractString(r.Args[0]) == job {
			out = append(out, types.ExtractString(r.Args[1]))
		}
	}
	return out
}
