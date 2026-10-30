package deliberation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/sessionlog"
	"codenerd/internal/types"
)

// Service is also the planner-facing hook: Run returns a policy-admitted design
// without editing anything. The caller decides when to execute the returned brief
// through its ordinary work-admission and constitutional paths.
type Service struct {
	Kernel             types.Kernel
	Workspace, Session string
	Config             config.RoutingConfig
	Explain            core.ExplainOptions
	RunShard           func(context.Context, string, Frame) (string, error)
	Probe              func(context.Context, types.ToolCall) (string, error)
	Consult            func(context.Context, Consultation, string) (string, error)
}

func (s *Service) Run(ctx context.Context, t *Thread) (out Outcome, err error) {
	if s.Kernel == nil || s.RunShard == nil {
		return out, fmt.Errorf("deliberation dependencies unavailable")
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("deliberation-%d", time.Now().UnixNano())
	}
	if t.Session != "" && t.Session != s.Session {
		return out, fmt.Errorf("deliberation belongs to another session")
	}
	t.Session = s.Session
	if err = config.EnsureParams(s.Kernel, s.Config.Params()); err != nil {
		return out, err
	}
	ctx = sessionlog.WithAgent(ctx, "deliberation", t.ID)
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "plan", Text: "Frame: " + t.Request})
	defer func() {
		if err != nil {
			t.Status = "failed"
			if saveErr := s.Save(t); saveErr != nil {
				err = fmt.Errorf("%w; persist failure: %v", err, saveErr)
			}
		}
		sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventFinal, Text: "Deliberation: " + t.Status, Failed: err != nil})
	}()
	var facts []types.Fact
	defer func() {
		if e := s.Kernel.RetractExactFactsBatch(facts); e != nil && err == nil {
			err = e
		}
	}()
	add := func(f types.Fact) error {
		if err := s.Kernel.Assert(f); err != nil {
			return err
		}
		facts = append(facts, f)
		return nil
	}
	if p, ok := s.Kernel.(interface {
		EnableProvenance()
		DisableProvenance()
		IsProvenanceEnabled() bool
	}); ok && !p.IsProvenanceEnabled() {
		p.EnableProvenance()
		defer p.DisableProvenance()
	}
	if err = add(types.Fact{Predicate: "deliberation_mode", Args: []any{t.ID, types.MangleAtom("/" + strings.TrimPrefix(t.Mode, "/"))}}); err != nil {
		return out, err
	}
	if t.Asked {
		if err = add(types.Fact{Predicate: "deliberation_questioned", Args: []any{t.ID}}); err != nil {
			return out, err
		}
	}
	// A concrete option answer replays the exact challenged round. Free text
	// becomes evidence for refinement; it is never interpreted as an approval.
	if t.Status == "awaiting_values" && len(t.Rounds) > 0 {
		r := &t.Rounds[len(t.Rounds)-1]
		if c := choice(*r, t.Answer); c != nil {
			if err = s.assertRound(t.ID, r, add); err != nil {
				return out, err
			}
			if err = add(types.Fact{Predicate: "deliberation_answer", Args: []any{t.ID, int64(r.Number), types.MangleString(c.ID)}}); err != nil {
				return out, err
			}
			return s.finish(ctx, t, r)
		}
	}
	t.Status = "running"
	if err = s.Save(t); err != nil {
		return out, err
	}
	for n := len(t.Rounds) + 1; ; n++ {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		if err = add(types.Fact{Predicate: "deliberation_tick", Args: []any{t.ID, int64(n), int64(t.Unchanged)}}); err != nil {
			return out, err
		}
		stops, e := s.Kernel.Query("deliberation_stop")
		if e != nil {
			return out, e
		}
		for _, f := range stops {
			if len(f.Args) == 3 && types.ExtractString(f.Args[0]) == t.ID && number(f.Args[1]) == n {
				t.Status = strings.TrimPrefix(types.ExtractString(f.Args[2]), "/")
				if err = s.Save(t); err != nil {
					return out, err
				}
				return Outcome{Thread: t, Action: "stopped"}, nil
			}
		}
		r, e := s.collectRound(ctx, t, n)
		if e != nil {
			return out, e
		}
		if e = s.assertRound(t.ID, &r, add); e != nil {
			return out, e
		}
		decisions, e := s.Kernel.Query("deliberation_decision")
		if e != nil {
			return out, e
		}
		decision, e := oneRound(decisions, t.ID, n)
		if e != nil {
			return out, e
		}
		r.Verdict, r.Chosen = strings.TrimPrefix(types.ExtractString(decision.Args[2]), "/"), types.ExtractString(decision.Args[3])
		if e = s.prove(decision, &r); e != nil {
			return out, e
		}
		t.Rounds = append(t.Rounds, r)
		current := &t.Rounds[len(t.Rounds)-1]
		if e = s.Save(t); e != nil {
			return out, e
		}
		sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventAssistant, Text: fmt.Sprintf("Round %d: %s (%s)", n, r.Verdict, r.Chosen)})
		switch r.Verdict {
		case "ready", "values_fork":
			return s.finish(ctx, t, current)
		case "empirical":
			if e = s.runProbes(ctx, t, current, add); e != nil {
				return out, e
			}
		case "needs_work":
		default:
			return out, fmt.Errorf("unrecognized deliberation verdict %q", r.Verdict)
		}
		signature := progressFingerprint(*current)
		if signature == t.Fingerprint {
			t.Unchanged++
		} else {
			t.Unchanged = 0
		}
		t.Fingerprint = signature
		if e = s.Save(t); e != nil {
			return out, e
		}
	}
}

func number(v any) int {
	switch x := v.(type) {
	case int64:
		return int(x)
	case int:
		return x
	case float64:
		return int(x)
	}
	return -1
}

func oneRound(rows []types.Fact, id string, round int) (types.Fact, error) {
	var matched []types.Fact
	for _, f := range rows {
		if len(f.Args) == 4 && types.ExtractString(f.Args[0]) == id && number(f.Args[1]) == round {
			matched = append(matched, f)
		}
	}
	if len(matched) != 1 {
		return types.Fact{}, fmt.Errorf("deliberation requires exactly one decision, got %d", len(matched))
	}
	return matched[0], nil
}
