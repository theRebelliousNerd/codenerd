// Package turnjudge asks Jev (TypeSafe's System One model) a set of narrow,
// typed judgments about each chat turn and hands the answers to the kernel as
// evidence. It enhances perception, never replaces it: perception still reads
// the turn and the LLM still writes every reply; the kernel's rules
// (policy/jev_evidence.mg) only ever use these judgments to ask first, to
// recognise a go-ahead, to close an answered brief question, or to start the
// self-diagnosis chain. A turn with no judgments routes exactly as before.
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

// Input is what a person would look at to judge the turn.
type Input struct {
	Message string
	Recent  []Turn
	// Brief is the brief as it stood before this turn.
	Brief *perception.Brief
	// HeldWork describes work held for a go-ahead; empty when none is.
	HeldWork string
}

// Asker is the Jev call (typesafe.Client.Ask).
type Asker interface {
	Ask(ctx context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error)
}

// Result is one turn's judgments: the probability that each is true, what an
// answered_N judgment is about, and the model and input tokens that paid for it.
type Result struct {
	Judgments   map[string]float64
	About       map[string]string
	Model       string
	InputTokens int
}

// RecentWindow bounds the prior turns a judgment sees. Jev's context is 64k
// tokens per request (docs.typesafe.ai/models), and the judgments are about
// the latest message; the turns go whole, and older ones are left out.
const RecentWindow = 12

// Request builds the state and the questions for a turn. Only fields with
// content go in the state: an empty list reads to the model as a signal.
func Request(in Input) (state map[string]any, questions map[string]typesafe.Question, about map[string]string) {
	state = map[string]any{
		"assistant": "codeNERD, a coding agent working in the user's repository",
		"message":   in.Message,
	}
	recent := in.Recent
	if len(recent) > RecentWindow {
		recent = recent[len(recent)-RecentWindow:]
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
	if strings.TrimSpace(in.HeldWork) != "" {
		state["held_work"] = in.HeldWork
		questions[ApprovesHeldWork] = heldWorkQuestion()
		questions[AddsToHeldWork] = addsToHeldWorkQuestion()
	}
	return state, questions, about
}

// Judge asks every question in one request. A failed call is an error, never
// an answer: the caller then has no evidence, which routes as before.
func Judge(ctx context.Context, a Asker, in Input) (*Result, error) {
	state, questions, about := Request(in)
	resp, err := a.Ask(ctx, state, questions)
	if err != nil {
		return nil, err
	}
	res := &Result{Judgments: make(map[string]float64, len(questions)), About: about, Model: resp.Model, InputTokens: resp.Usage.InputTokens}
	for name := range questions {
		p, err := resp.Noul(name)
		if err != nil {
			return nil, fmt.Errorf("jev judgment %s: %w", name, err)
		}
		res.Judgments[name] = p
	}
	return res, nil
}

// Predicates are the facts Facts writes, for retract-and-replace each turn.
var Predicates = []string{"jev_judgment", "jev_judgment_about"}

// Facts projects the judgments as policy/jev_evidence.mg's base facts:
// jev_judgment(Name, Percent) with the probability as a whole percent, and
// jev_judgment_about(Name, Subject) for each answered_N.
func (r *Result) Facts() []types.Fact {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.Judgments))
	for n := range r.Judgments {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]types.Fact, 0, len(names)+len(r.About))
	for _, n := range names {
		pct := int64(math.Round(r.Judgments[n] * 100))
		out = append(out, types.Fact{Predicate: "jev_judgment", Args: []any{types.MangleString(n), pct}})
		if subject, ok := r.About[n]; ok {
			out = append(out, types.Fact{Predicate: "jev_judgment_about", Args: []any{types.MangleString(n), types.MangleString(subject)}})
		}
	}
	return out
}

// Summary is one line for logs and the Glass Box: each judgment and its
// probability, sorted by name.
func (r *Result) Summary() string {
	if r == nil {
		return "no judgments"
	}
	names := make([]string, 0, len(r.Judgments))
	for n := range r.Judgments {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%.2f", n, r.Judgments[n]))
	}
	return fmt.Sprintf("%s (%s, %d input tokens)", strings.Join(parts, " "), r.Model, r.InputTokens)
}
