package deliberation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
)

func (s *Service) finish(ctx context.Context, t *Thread, r *Round) (Outcome, error) {
	rows, err := s.Kernel.Query("deliberation_disposition")
	if err != nil {
		return Outcome{}, err
	}
	f, err := oneRound(rows, t.ID, r.Number)
	if err != nil {
		return Outcome{}, err
	}
	action, id := strings.TrimPrefix(types.ExtractString(f.Args[2]), "/"), types.ExtractString(f.Args[3])
	c := candidate(r, id)
	if c == nil {
		return Outcome{}, fmt.Errorf("policy selected unknown candidate %q", id)
	}
	if err = s.prove(f, r); err != nil {
		return Outcome{}, err
	}
	out := Outcome{Thread: t, Action: action, Candidate: c}
	switch action {
	case "proceed", "proceed_assuming":
		t.Status = "ready"
		r.Chosen = id
	case "ask_once":
		question := strings.TrimSpace(r.Challenge.Question)
		if question == "" || strings.Contains(question, "\n") || strings.Count(question, "?") > 1 {
			return Outcome{}, fmt.Errorf("values fork needs one concrete question")
		}
		eligible, e := s.Kernel.Query("deliberation_eligible")
		if e != nil {
			return Outcome{}, e
		}
		permitted := map[string]bool{}
		for _, row := range eligible {
			if len(row.Args) == 5 && types.ExtractString(row.Args[0]) == t.ID && number(row.Args[1]) == r.Number && types.ExtractString(row.Args[2]) == "/values_fork" {
				permitted[types.ExtractString(row.Args[3])] = true
			}
		}
		out.Options = append(out.Options, option(*c)+" (Recommended)")
		for _, other := range r.Proposal.Candidates {
			if other.ID != c.ID && permitted[other.ID] {
				out.Options = append(out.Options, option(other))
			}
		}
		if len(out.Options) < 2 {
			return Outcome{}, fmt.Errorf("values fork requires at least two viable options")
		}
		t.Status, t.Asked = "awaiting_values", true
		out.Question = question
		t.Question, t.Options = question, out.Options
	case "stop_values":
		t.Status = "unresolved_values"
	default:
		return Outcome{}, fmt.Errorf("unknown deliberation disposition %q", action)
	}
	if err = s.Save(t); err != nil {
		return Outcome{}, err
	}
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventAssistant, Text: out.Note()})
	return out, nil
}

func option(c Candidate) string { return c.ID + ": " + c.Brief }
func choice(r Round, answer string) *Candidate {
	answer = strings.TrimSpace(strings.TrimSuffix(answer, " (Recommended)"))
	for i := range r.Proposal.Candidates {
		c := &r.Proposal.Candidates[i]
		if answer == c.ID || answer == option(*c) {
			return c
		}
	}
	return nil
}

func (out Outcome) Note() string {
	if out.Thread == nil {
		return ""
	}
	if out.Thread.Status != "ready" {
		return "Deliberation: " + out.Thread.Status + "."
	}
	c := out.Candidate
	text := "Decided: " + c.Brief
	if len(c.Assumptions) > 0 {
		text += "\nAssumed: " + strings.Join(c.Assumptions, "; ")
	}
	if out.Action == "proceed_assuming" {
		text += "\nAssumed the recommended preference in yolo mode."
	}
	text += "\nSuccess test: " + c.SuccessTest
	if len(out.Thread.Rounds) > 0 {
		var proof struct {
			Goal   string
			Proofs []struct{ ID string }
		}
		last := out.Thread.Rounds[len(out.Thread.Rounds)-1]
		if json.Unmarshal(last.Proof, &proof) == nil && len(proof.Proofs) > 0 {
			text += "\nPolicy proof: " + proof.Proofs[0].ID
		}
	}
	return text
}

func progressFingerprint(r Round) string {
	// Use stable open-item identities and actual probe observations. Rewording
	// a brief or rationale does not count as progress.
	var parts []string
	for _, a := range r.Challenge.Assessments {
		for _, item := range a.Items {
			if item.Severity == "critical" {
				parts = append(parts, a.CandidateID+":"+item.Key+":"+item.Kind)
			}
		}
	}
	for _, p := range r.Probes {
		b, _ := json.Marshal(p)
		parts = append(parts, string(b))
	}
	sort.Strings(parts)
	hash := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(hash[:])
}

func (s *Service) runProbes(ctx context.Context, thread *Thread, r *Round, add func(types.Fact) error) error {
	id := thread.ID
	if s.Probe == nil {
		return fmt.Errorf("empirical verdict requires probe executor")
	}
	actor := fmt.Sprintf("%s-probe-%d", sessionlog.ScopeFrom(ctx).Folder, r.Number)
	calls := 0
	for _, a := range r.Challenge.Assessments {
		if a.CandidateID != r.Chosen {
			continue
		}
		for _, item := range a.Items {
			if item.Severity != "critical" || item.Kind != "data" {
				continue
			}
			allowed, err := AllowCall(s.Kernel, actor, calls)
			if err != nil {
				return err
			}
			if !allowed {
				return nil
			}
			calls++
			sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "step", Text: "Probe: " + item.Key})
			result, e := s.Probe(ctx, *item.Probe)
			p := ProbeResult{Key: item.Key, Tool: item.Probe.Name, Output: result}
			if e != nil {
				p.Error = e.Error()
			}
			r.Probes = append(r.Probes, p)
			if err := s.Save(thread); err != nil {
				return err
			}
			receipt, err := json.Marshal(p)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(receipt)
			if err := add(types.Fact{Predicate: "deliberation_probe", Args: []any{id, int64(r.Number), types.MangleString(p.Key), types.MangleString(p.Tool), hex.EncodeToString(digest[:])}}); err != nil {
				return err
			}
		}
	}
	return nil
}

// AllowCall measures usage; policy owns the limit. An exact temporary fact
// prevents concurrent roles from erasing each other's budgets.
func AllowCall(k types.Kernel, id string, count int) (bool, error) {
	tick := types.Fact{Predicate: "deliberation_agent_tick", Args: []any{id, int64(count)}}
	if err := k.Assert(tick); err != nil {
		return false, err
	}
	defer k.RetractFact(tick)
	rows, err := k.Query("deliberation_agent_continue")
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if len(row.Args) == 1 && types.ExtractString(row.Args[0]) == id {
			return true, nil
		}
	}
	return false, nil
}
