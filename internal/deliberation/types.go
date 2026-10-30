// Package deliberation relays evidence between read-only proposer and challenger
// shards. The execution kernel owns candidate selection and stopping decisions.
package deliberation

import (
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

const Proposer = "deliberation_proposer"
const Challenger = "requirements_interrogator"

type Candidate struct {
	ID          string   `json:"id"`
	Brief       string   `json:"brief"`
	Target      string   `json:"target"`
	SuccessTest string   `json:"success_test"`
	Assumptions []string `json:"assumptions"`
	Evidence    []string `json:"evidence"`
}

type Item struct {
	Key      string          `json:"key"`
	Severity string          `json:"severity"` // critical | important
	Kind     string          `json:"kind"`     // data | design | values
	Detail   string          `json:"detail"`
	Probe    *types.ToolCall `json:"probe,omitempty"`
}

type Assessment struct {
	CandidateID string `json:"candidate_id"`
	Success     bool   `json:"success"`
	Items       []Item `json:"items"`
}

type Consultation struct {
	Specialist string `json:"specialist"`
	Question   string `json:"question"`
}
type Proposal struct {
	Candidates    []Candidate    `json:"candidates"`
	Consultations []Consultation `json:"consultations,omitempty"`
}
type Challenge struct {
	Assessments    []Assessment `json:"assessments"`
	Recommendation string       `json:"recommendation"`
	Question       string       `json:"question,omitempty"`
}

type ProbeResult struct {
	Key    string `json:"key"`
	Tool   string `json:"tool"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type Round struct {
	Number    int             `json:"number"`
	Proposal  Proposal        `json:"proposal"`
	Challenge Challenge       `json:"challenge"`
	Verdict   string          `json:"verdict"`
	Chosen    string          `json:"chosen,omitempty"`
	Probes    []ProbeResult   `json:"probes,omitempty"`
	Proof     json.RawMessage `json:"proof,omitempty"`
}

// Thread is the durable continuation, including the original intent. An answer
// is never reclassified as a fresh /approve request.
type Thread struct {
	Brief       *perception.Brief `json:"brief,omitempty"`
	ID          string            `json:"id"`
	Session     string            `json:"session"`
	Request     string            `json:"request"`
	Intent      perception.Intent `json:"intent"`
	Mode        string            `json:"mode"`
	DesignOnly  bool              `json:"design_only"`
	Rounds      []Round           `json:"rounds"`
	Unchanged   int               `json:"unchanged"`
	Fingerprint string            `json:"fingerprint"`
	Asked       bool              `json:"asked"`
	Question    string            `json:"question,omitempty"`
	Options     []string          `json:"options,omitempty"`
	Answer      string            `json:"answer,omitempty"`
	Status      string            `json:"status"`
}

// Frame is data, never an embedded system prompt. The role's JIT atoms define
// how to interpret it. Only the last completed round is replayed.
type Frame struct {
	Brief        *perception.Brief `json:"brief,omitempty"`
	ReceiptFacts []string          `json:"receipt_facts,omitempty"`
	Evidence     []ProbeResult     `json:"evidence,omitempty"`
	Request      string            `json:"request"`
	Intent       perception.Intent `json:"intent"`
	Phase        string            `json:"phase"` // divergent first, convergent thereafter
	Previous     *Round            `json:"previous,omitempty"`
	Candidates   []Candidate       `json:"candidates,omitempty"`
	Answer       string            `json:"answer,omitempty"`
	CandidateCap int               `json:"candidate_cap"`
}

type Outcome struct {
	Thread    *Thread
	Action    string
	Candidate *Candidate
	Question  string
	Options   []string
}

func Decode(raw string, out any) error {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		_, raw, _ = strings.Cut(raw, "\n")
		raw = strings.TrimSpace(strings.TrimSuffix(raw, "```"))
	}
	var envelope struct {
		Surface string `json:"surface_response"`
	}
	if json.Unmarshal([]byte(raw), &envelope) == nil && envelope.Surface != "" {
		raw = envelope.Surface
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("deliberation output: %w", err)
	}
	return nil
}

func validateRound(r Round, cap int) error {
	if len(r.Proposal.Candidates) == 0 || len(r.Proposal.Candidates) > cap {
		return fmt.Errorf("candidate count outside configured bound")
	}
	ids := map[string]bool{}
	for _, c := range r.Proposal.Candidates {
		if c.ID == "" || ids[c.ID] || strings.TrimSpace(c.Brief) == "" || strings.TrimSpace(c.SuccessTest) == "" {
			return fmt.Errorf("candidate requires unique id, brief and success test")
		}
		ids[c.ID] = true
	}
	seen := map[string]bool{}
	for _, a := range r.Challenge.Assessments {
		if !ids[a.CandidateID] || seen[a.CandidateID] {
			return fmt.Errorf("assessment must identify one proposed candidate exactly once")
		}
		seen[a.CandidateID] = true
		keys := map[string]bool{}
		for _, item := range a.Items {
			if item.Key == "" || keys[item.Key] {
				return fmt.Errorf("open items need stable unique keys")
			}
			keys[item.Key] = true
			if item.Severity != "critical" && item.Severity != "important" {
				return fmt.Errorf("invalid severity %q", item.Severity)
			}
			if item.Kind != "data" && item.Kind != "design" && item.Kind != "values" {
				return fmt.Errorf("invalid item kind %q", item.Kind)
			}
			if item.Severity == "critical" && item.Kind == "data" && item.Probe == nil {
				return fmt.Errorf("critical data item requires a read-only probe")
			}
		}
	}
	if len(seen) != len(ids) || !ids[r.Challenge.Recommendation] {
		return fmt.Errorf("every candidate needs an assessment and a valid recommendation")
	}
	return nil
}

func candidate(r *Round, id string) *Candidate {
	for i := range r.Proposal.Candidates {
		if r.Proposal.Candidates[i].ID == id {
			return &r.Proposal.Candidates[i]
		}
	}
	return nil
}
