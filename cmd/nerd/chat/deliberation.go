package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/consultation"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/perception"
	"codenerd/internal/sessionlog"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) deliberator() *deliberation.Service {
	cfg := m.Config.GetRoutingConfig()
	explain := m.Config.GetExplainConfig()
	reads := deliberation.ReadTools{Kernel: m.kernel, ContextBytes: cfg.DeliberationContextBytes}
	if m.virtualStore != nil {
		reads.Registry = m.virtualStore.GetModularTools()
	}
	return &deliberation.Service{Kernel: m.kernel, Workspace: m.workspace, Session: m.sessionID, Config: cfg,
		Explain: core.ExplainOptions{MaxProofs: explain.MaxProofs, MaxDepth: explain.MaxDepth,
			MaxNodes: explain.MaxNodes, MaxEdges: explain.MaxEdges, MaxBytes: explain.MaxBytes},
		RunShard: func(ctx context.Context, role string, frame deliberation.Frame) (string, error) {
			if m.shardMgr == nil {
				return "", fmt.Errorf("deliberation shard manager unavailable")
			}
			body, err := json.Marshal(frame)
			if err != nil {
				return "", err
			}
			return m.shardMgr.Spawn(ctx, role, string(body))
		}, Probe: reads.Execute,
		Consult: func(ctx context.Context, req deliberation.Consultation, request string) (string, error) {
			if m.consultationMgr == nil {
				return "", fmt.Errorf("consultation manager unavailable")
			}
			restricted := perception.RestrictionOf("deliberation reads only", perception.RestrictReadOnly)
			ctx = perception.WithRestriction(ctx, perception.RestrictionFromContext(ctx).Merge(restricted))
			answer, err := m.consultationMgr.Consult(ctx, consultation.Request{Requester: deliberation.Proposer,
				Specialist: req.Specialist, Question: req.Question, Context: request})
			if err != nil {
				return "", err
			}
			return answer.Advice, nil
		}}
}

func (m Model) deliberationQuestion(out deliberation.Outcome) clarificationMsg {
	return clarificationMsg{Question: out.Question, Options: out.Options, DefaultOption: out.Options[0],
		Context: out.Thread.Request, PendingIntent: &out.Thread.Intent, DeliberationID: out.Thread.ID}
}

// adoptDeliberation preserves the verb and permission restrictions of the
// original request. The selected reading supplies a brief, not new authority.
func (m *Model) adoptDeliberation(out deliberation.Outcome) (string, perception.Intent) {
	intent := out.Thread.Intent
	intent.Target = out.Candidate.Target
	intent.Constraint = strings.TrimSpace(intent.Constraint + "\nChosen brief: " + out.Candidate.Brief +
		"\nSuccess test: " + out.Candidate.SuccessTest + "\nAssumptions: " + strings.Join(out.Candidate.Assumptions, "; "))
	intent.Response = ""
	m.resumeIntent, m.deliberationResolved, m.resumeNote = &intent, true, out.Note()
	return out.Thread.Request + "\n\n" + intent.Constraint, intent
}

func (m Model) resumeDeliberation(response, id string, request uint64) tea.Cmd {
	return m.foregroundOperation("deliberation", func(ctx context.Context) tea.Msg {
		service := m.deliberator()
		thread, err := service.Load(id)
		if err != nil {
			return errorMsg(err)
		}
		if thread.Status != "awaiting_values" {
			return errorMsg(fmt.Errorf("deliberation is not awaiting an answer"))
		}
		thread.Answer = response
		out, err := service.Run(ctx, thread)
		if err != nil {
			return errorMsg(err)
		}
		if thread.Status != "ready" {
			return assistantMsg{Surface: out.Note()}
		}
		if thread.DesignOnly {
			return assistantMsg{Surface: out.Note()}
		}
		input, _ := m.adoptDeliberation(out)
		return m.processInput(input, request)()
	})
}

func (m Model) handleCmdDeliberate(input string, parts []string) (tea.Model, tea.Cmd) {
	if len(parts) < 2 {
		m = m.pushAssistantMsg("Usage: /deliberate <sub-problem> (or /innovate). Returns a converged design brief.")
		m.viewport.SetContent(m.renderHistory())
		m.viewport.GotoBottom()
		return m, nil
	}
	task := strings.TrimSpace(strings.TrimPrefix(input, parts[0]))
	m.textarea.Reset()
	m.isLoading = true
	return m, tea.Batch(m.spinner.Tick, m.foregroundOperation("deliberation", func(ctx context.Context) tea.Msg {
		if m.delegatedEvents != nil {
			ctx = sessionlog.WithLiveProgress(ctx, m.sendDelegatedProgress)
		}
		thread := &deliberation.Thread{Brief: m.currentBrief(), Request: task, Intent: perception.Intent{Category: "/query", Verb: "/research", Target: task}, Mode: string(m.workMode()), DesignOnly: true}
		out, err := m.deliberator().Run(ctx, thread)
		if err != nil {
			return errorMsg(err)
		}
		if out.Action == "ask_once" {
			return m.deliberationQuestion(out)
		}
		return assistantMsg{Surface: out.Note()}
	}))
}

func withDeliberationNote(msg tea.Msg, note string) tea.Msg {
	if note == "" {
		return msg
	}
	switch v := msg.(type) {
	case responseMsg:
		return responseMsg(note + "\n\n" + string(v))
	case assistantMsg:
		v.Surface = note + "\n\n" + v.Surface
		return v
	case backgroundStartMsg:
		v.Surface = note + "\n\n" + v.Surface
		return v
	case continuationInitMsg:
		v.completedSurface = note + "\n\n" + v.completedSurface
		return v
	case holdMsg:
		v.Surface = note + "\n\n" + v.Surface
		return v
	}
	return msg
}
