// Package learnjudge holds the Jev (TypeSafe System One) questions that the
// learning and self-modification loops ask: whether the user is correcting
// codeNERD's last turn, how a past task went for prompt evolution, what kind
// of problem a task is, whether a requested tool already exists and whether a
// generated tool does what was asked, whether a request needs a campaign or a
// persistent agent, and whether an evolved prompt atom may be promoted.
//
// Every question's wording, options and criteria live in this package, one
// file per use. Each answer is a probability; the decision that reads it is a
// truth table in the consuming package or a kernel rule
// (policy/jev_learning.mg), never a threshold in here. A failed, timed-out or
// switched-off call is an error or no answer, never a "no": each consumer
// names its fallback and logs it (Docs/architecture/jev-turn-judgments.md,
// "Learning and autopoiesis").
package learnjudge

import (
	"context"

	"codenerd/internal/typesafe"
)

// Asker is the Jev call (typesafe.Client.Ask).
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error)
}

// The usage tracker's operation for each use (typesafe.WithOperation), so each
// use's cost is its own row.
const (
	OpCorrection        = "jev_correction"
	OpTaskVerdict       = "jev_task_verdict"
	OpProblemType       = "jev_problem_type"
	OpToolCheck         = "jev_tool_check"
	OpAutopoiesisChecks = "jev_autopoiesis_checks"
	OpAtomCheck         = "jev_atom_check"
	OpCoverage          = "jev_instruction_coverage"
	OpVerbChoice        = "jev_verb_choice"
	OpFinalReport       = "jev_final_report"
)
