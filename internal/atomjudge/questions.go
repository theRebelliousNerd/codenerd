// Package atomjudge asks Jev (TypeSafe's System One model) the two questions
// JIT prompt compilation needs about prompt atoms:
//
//   - at corpus build time, a Score per atom: what happens if the model
//     ignores this guidance (importance.go); the answer is stored in the
//     embedded importance ledger and costs nothing at runtime;
//   - per turn, a Noul per pooled flesh atom: does this guidance apply to
//     this request (relevance.go); the answers become atom_relevance facts.
//
// Every question's wording lives in this file. Decisions are Mangle rules
// (jit_compiler.mg, jit_selection.mg) and Go orderings over the answers;
// nothing here decides.
package atomjudge

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// Usage-tracker operations (typesafe.WithOperation).
const (
	OpAtomImportance = "jev_atom_importance"
	OpAtomRelevance  = "jev_atom_relevance"
)

// importanceState is shared by every importance question: the one fact a
// person needs to read a guidance block as prompt text.
var importanceState = map[string]any{
	"agent": "codeNERD, a coding agent. Each guidance block is text placed in the system prompt of the agent's language model when it applies to the work.",
}

// importanceInstruction is the Score's question. Probed side by side with
// "How much harm would ignoring `guidance` cause?" on 2026-10-28
// (Docs/architecture/jev-probes/2026-10-28-atom-jit/atom-importance-wording.json); the two agreed
// within 0.15 on every atom, and this one names the situation the levels
// describe.
const importanceInstruction = "If the coding agent ignored `guidance`, what would most likely happen?"

// importanceQuestion is the Score for one atom. The levels are
// prompt.ImportanceMeanings, the same text the level constants document.
func importanceQuestion(g guidance) typesafe.Score {
	levels := prompt.ImportanceMeanings()
	criteria := make([]any, len(levels))
	for i, l := range levels {
		criteria[i] = l
	}
	return typesafe.Score{
		Instructions: map[string]any{"question": importanceInstruction, "guidance": g},
		Criteria:     criteria,
	}
}

// ImportanceScaleFingerprint identifies the importance question and its
// levels. The ledger records it; a ledger with another fingerprint answered a
// different question, and every score in it is stale.
func ImportanceScaleFingerprint() string {
	h := sha256.New()
	h.Write([]byte(importanceInstruction))
	for _, l := range prompt.ImportanceMeanings() {
		h.Write([]byte{0})
		h.Write([]byte(l))
	}
	h.Write([]byte{0})
	h.Write([]byte(importanceState["agent"].(string)))
	return "importance/" + hex.EncodeToString(h.Sum(nil))[:16]
}

// relevanceQuestion is the per-turn Noul for one atom. Probed against "Would
// following `guidance` help the agent carry out `request`?" on 2026-10-28
// (Docs/architecture/jev-probes/2026-10-28-atom-jit/atom-relevance-wording.json): this wording
// put clear matches above 0.85 and clear mismatches below 0.1, where the
// other compressed matches toward 0.6.
func relevanceQuestion(g guidance) typesafe.Noul {
	return typesafe.Noul{
		Instructions: map[string]any{
			"question": "Does `guidance` apply to the work in `request`?",
			"guidance": g,
		},
		Criteria: &typesafe.NoulCriteria{
			True:  "The guidance tells the agent how to do part of this request: it is about the request's task, language, tools or the kind of code it changes.",
			False: "The guidance is about a different task, language or tool than the request, even if it shares words with it.",
		},
	}
}

// guidance is one atom as Jev reads it: its name, what it is scoped to, and
// its full text. Selector values drop their slash.
type guidance map[string]any

func guidanceOf(a *prompt.PromptAtom) guidance {
	g := guidance{"id": a.ID, "category": string(a.Category)}
	if a.Description != "" {
		g["description"] = a.Description
	}
	add := func(key string, values []string) {
		if len(values) == 0 {
			return
		}
		out := make([]string, len(values))
		for i, v := range values {
			out[i] = strings.TrimPrefix(v, "/")
		}
		g[key] = out
	}
	add("agents", a.ShardTypes)
	add("languages", a.Languages)
	add("frameworks", a.Frameworks)
	add("intents", a.IntentVerbs)
	g["text"] = a.Content
	return g
}

// requestState is the per-turn state: the request as a person would read it.
// Empty fields are left out so the model never reads an empty value as a
// signal.
func requestState(r prompt.AtomRelevanceRequest) map[string]any {
	req := map[string]any{"text": r.Request}
	if r.Intent != "" {
		req["intent"] = r.Intent
	}
	if r.Target != "" {
		req["target"] = r.Target
	}
	if r.Language != "" {
		req["language"] = r.Language
	}
	if len(r.Frameworks) > 0 {
		req["frameworks"] = r.Frameworks
	}
	if r.Agent != "" {
		req["agent"] = r.Agent
	}
	return map[string]any{"request": req}
}
