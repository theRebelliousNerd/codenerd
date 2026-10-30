package init

import (
	"sort"
	"strings"
	"unicode"
)

// A shard agent is expert in a domain: a technology key (the vocabulary in
// technology_vocabulary.go, or a language the kernel's extension table names)
// or a concept. A concept is a subject no import or file extension names --
// security, testing -- so a task needs its expert by the verb it runs as, and
// the agent's knowledge is about subjects of its own.
//
// This file is measurement: the words that name a concept, the verbs it covers
// and what to research about it. Go never decides that an agent holds a domain
// or that a task needs it: boot asserts these as facts (domain_concept_term,
// domain_concept_verb, shard_agent_name_part) and policy/shard_experts.mg
// derives shard_agent_domain and expert_needed over them.

// ConceptDomain is one concept domain's vocabulary.
type ConceptDomain struct {
	// Key is the domain key an agent's `domains` records ("security").
	Key string
	// Terms are the lower-case words that name the concept: a word of an
	// agent's name that equals one makes the agent expert in the domain.
	Terms []string
	// Verbs are the verbs (with the leading slash) whose tasks the domain's
	// experts are needed for. A concept no verb covers still gives its agents
	// something to know (Subjects), and is needed through nothing else.
	Verbs []string
	// Subjects are what the domain's knowledge is acquired about: asked of
	// web search and Context7 as research topics.
	Subjects []string
}

var conceptDomains = []ConceptDomain{
	{
		Key:   "security",
		Terms: []string{"security", "secure"},
		Verbs: []string{"/security"},
		Subjects: []string{
			"application security: OWASP Top 10 risks and their mitigations",
			"CWE Top 25 most dangerous software weaknesses",
		},
	},
	{
		Key:   "testing",
		Terms: []string{"test", "tests", "testing"},
		Verbs: []string{"/test"},
		Subjects: []string{
			"software testing strategy: unit, integration and property-based tests",
			"test design: coverage, flakiness and test isolation",
		},
	},
	{
		Key:   "observability",
		Terms: []string{"observability"},
		Subjects: []string{
			"observability: structured logs, metrics and traces",
		},
	},
	{
		Key:   "evaluation",
		Terms: []string{"evaluation"},
		Subjects: []string{
			"evaluating LLM agents: benchmarks, graders and regression suites",
		},
	},
	{
		Key:   "safety",
		Terms: []string{"safety"},
		Subjects: []string{
			"AI agent safety: permission policy, sandboxing and approval gates",
		},
	},
}

// ConceptDomains returns the concept vocabulary, sorted by key.
func ConceptDomains() []ConceptDomain {
	out := make([]ConceptDomain, len(conceptDomains))
	for i, c := range conceptDomains {
		out[i] = ConceptDomain{
			Key:      c.Key,
			Terms:    append([]string(nil), c.Terms...),
			Verbs:    append([]string(nil), c.Verbs...),
			Subjects: append([]string(nil), c.Subjects...),
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// ConceptSubjects are the research subjects of a concept domain key, or nil
// for a key that is not a concept.
func ConceptSubjects(key string) []string {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, c := range conceptDomains {
		if c.Key == key {
			return append([]string(nil), c.Subjects...)
		}
	}
	return nil
}

// AgentNameParts splits an agent's name into its lower-case words: on
// separators (hyphen, underscore, dot, space) and at a lower-to-upper case
// change, with an acronym kept whole ("MCPIntegrationExpert" -> mcp,
// integration, expert; "mangle-logic-architect" -> mangle, logic, architect).
// Boot asserts them as shard_agent_name_part; whether a word names a
// language, a technology or a concept is the kernel's judgement.
func AgentNameParts(name string) []string {
	var parts []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			parts = append(parts, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
			continue
		case unicode.IsUpper(r) && len(cur) > 0:
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return parts
}
