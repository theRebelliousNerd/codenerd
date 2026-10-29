// Package turnjudge asks Jev (TypeSafe's System One model) a set of narrow,
// typed judgments about each chat turn and hands the answers to the kernel as
// evidence. It enhances perception, never replaces it: perception still reads
// the turn and the LLM still writes every reply; the kernel's rules
// (policy/jev_evidence.mg, conversation_acts.mg) only ever use these judgments
// to ask first, to recognise a go-ahead, to close an answered brief question,
// to start the self-diagnosis chain, to stop background work, to add a
// restriction, or to answer in the conversation a turn whose act asks for no
// work (the conversation act, a Choice). A turn with no judgments routes
// exactly as before.
//
// The package also holds Jev's other uses, each with its own question and its
// own kernel fact: retrieval relevance (relevance.go), review findings checked
// against their code (findings.go), and self-diagnosis triage (triage.go).
// Docs/architecture/jev-turn-judgments.md lists every question, its switch
// and its consumer.
package turnjudge

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// Turn is one prior turn of the conversation, whole.
type Turn struct {
	From string `json:"from"`
	Text string `json:"text"`
}

// HeldWork is work held for the user's go-ahead, as named fields.
type HeldWork struct {
	Action      string `json:"action,omitempty"`       // the verb, such as fix or refactor
	Target      string `json:"target,omitempty"`       // what it acts on
	Shard       string `json:"handled_by,omitempty"`   // the agent that would do it
	RequestedAs string `json:"requested_as,omitempty"` // the user's own words
}

func (w *HeldWork) empty() bool {
	return w == nil || *w == HeldWork{}
}

// Input is what a person would look at to judge the turn.
type Input struct {
	Message string
	Recent  []Turn
	// Brief is the brief as it stood before this turn.
	Brief *perception.Brief
	// HeldWork is the work held for a go-ahead; nil when none is.
	HeldWork *HeldWork
	// RunningWork describes each piece of work running now, as a person
	// would read it; empty when none runs. The stop judgment is asked only
	// then, for there is nothing to stop otherwise.
	RunningWork []string
	// RecentTurns bounds Recent (typesafe.recent_turns): the judgments are
	// about the latest message, the turns go whole, older ones are left out.
	// Zero keeps them all.
	RecentTurns int
}

// The usage tracker's operation for each use (typesafe.WithOperation), so
// each use's cost is its own row.
const (
	OpTurnJudgments      = "jev_turn_judgments"
	OpRetrievalRelevance = "jev_retrieval_relevance"
	OpReviewChecks       = "jev_review_checks"
	OpDiagnosisTriage    = "jev_diagnosis_triage"
)

// Asker is the Jev call (typesafe.Client.Ask).
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error)
}

// Result is one turn's judgments: the probability that each Noul is true, each
// Choice's whole distribution (option -> probability), what an answered_N
// judgment is about, and the model and input tokens that paid for it.
type Result struct {
	Judgments   map[string]float64
	Choices     map[string]map[string]float64
	About       map[string]string
	Model       string
	InputTokens int
}

// Request builds the state and the questions for a turn. Only fields with
// content go in the state: an empty list reads to the model as a signal.
func Request(in Input) (state map[string]any, questions map[string]typesafe.Question, about map[string]string) {
	state = map[string]any{
		"assistant": "codeNERD, a coding agent working in the user's repository",
		"message":   in.Message,
	}
	recent := in.Recent
	if in.RecentTurns > 0 && len(recent) > in.RecentTurns {
		// Older turns are left out to fit Jev's context, and the state says so.
		state["earlier_turns_omitted"] = len(recent) - in.RecentTurns
		recent = recent[len(recent)-in.RecentTurns:]
	}
	if len(recent) > 0 {
		state["recent_turns"] = recent
	}
	questions = fixedQuestions()
	about = map[string]string{}
	if !in.Brief.Empty() {
		b := map[string]any{"goal": in.Brief.Goal}
		if len(in.Brief.Constraints) > 0 {
			b["constraints"] = in.Brief.Constraints
		}
		if len(in.Brief.Decisions) > 0 {
			b["decisions"] = in.Brief.Decisions
		}
		if len(in.Brief.OpenQuestions) > 0 {
			b["open_questions"] = in.Brief.OpenQuestions
		}
		state["brief"] = b
		for i, q := range in.Brief.OpenQuestions {
			if strings.TrimSpace(q) == "" {
				continue
			}
			name := answeredName(i)
			questions[name] = openQuestion(q)
			about[name] = q
		}
	}
	if running := nonBlank(in.RunningWork); len(running) > 0 {
		state["running_work"] = running
		questions[WantsToStop] = stopQuestion()
	}
	if !in.HeldWork.empty() {
		state["held_work"] = in.HeldWork
		questions[ApprovesHeldWork] = heldWorkQuestion()
		questions[AddsToHeldWork] = addsToHeldWorkQuestion()
	}
	return state, questions, about
}

func nonBlank(in []string) []string {
	var out []string
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Judge asks every question in one request. A failed call is an error, never
// an answer: the caller then has no evidence, which routes as before.
func Judge(ctx context.Context, a Asker, in Input) (*Result, error) {
	state, questions, about := Request(in)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpTurnJudgments), state, questions)
	if err != nil {
		return nil, err
	}
	res := &Result{Judgments: make(map[string]float64, len(questions)), Choices: map[string]map[string]float64{}, About: about, Model: resp.Model, InputTokens: resp.Usage.InputTokens}
	for name, q := range questions {
		if _, choice := q.(typesafe.Choice); choice {
			a, err := resp.Choice(name)
			if err != nil {
				return nil, fmt.Errorf("jev judgment %s: %w", name, err)
			}
			res.Choices[name] = a.Probabilities
			continue
		}
		p, err := resp.Noul(name)
		if err != nil {
			return nil, fmt.Errorf("jev judgment %s: %w", name, err)
		}
		res.Judgments[name] = p
	}
	return res, nil
}

// Predicates are the facts Facts writes, for retract-and-replace each turn.
var Predicates = []string{"jev_judgment", "jev_judgment_about", "jev_choice"}

// Facts projects the judgments as policy/jev_evidence.mg's base facts:
// jev_judgment(Name, Percent) with the probability as a whole percent,
// jev_judgment_about(Name, Subject) for each answered_N, and
// jev_choice(Question, Option, Percent) for every option of every Choice. The
// whole distribution goes in, not the top option: which option counts as
// chosen, and when none does, is the kernel's reading (jev_chosen).
func (r *Result) Facts() []types.Fact {
	if r == nil {
		return nil
	}
	out := make([]types.Fact, 0, len(r.Judgments)+len(r.About))
	for _, n := range sortedKeys(r.Judgments) {
		out = append(out, types.Fact{Predicate: "jev_judgment", Args: []any{types.MangleString(n), percent(r.Judgments[n])}})
		if subject, ok := r.About[n]; ok {
			out = append(out, types.Fact{Predicate: "jev_judgment_about", Args: []any{types.MangleString(n), types.MangleString(subject)}})
		}
	}
	for _, q := range sortedKeys(r.Choices) {
		for _, o := range sortedKeys(r.Choices[q]) {
			out = append(out, types.Fact{Predicate: "jev_choice", Args: []any{types.MangleString(q), types.MangleString(o), percent(r.Choices[q][o])}})
		}
	}
	return out
}

// percent is a probability as the whole percent the rules compare.
func percent(p float64) int64 { return int64(math.Round(p * 100)) }

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Summary is one line for logs and the Glass Box: each judgment and its
// probability, sorted by name, then each Choice's top option.
func (r *Result) Summary() string {
	if r == nil {
		return "no judgments"
	}
	names := sortedKeys(r.Judgments)
	parts := make([]string, 0, len(names)+len(r.Choices))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%.2f", n, r.Judgments[n]))
	}
	// A Choice shows its top option; the kernel decides whether it counts.
	for _, q := range sortedKeys(r.Choices) {
		best, bp := "", -1.0
		for _, o := range sortedKeys(r.Choices[q]) {
			if p := r.Choices[q][o]; p > bp {
				best, bp = o, p
			}
		}
		parts = append(parts, fmt.Sprintf("%s=%s:%.2f", q, best, bp))
	}
	return fmt.Sprintf("%s (%s, %d input tokens)", strings.Join(parts, " "), r.Model, r.InputTokens)
}
