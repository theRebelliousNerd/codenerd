// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains conversation-context helpers and agent creation.
//
// HISTORY: this file used to hold a pre-perception follow-up substring
// detector (detectFollowUpQuestion / handleFollowUpQuestion). It was removed
// when routing arbitration moved to the kernel: the detector hijacked any
// input containing "what is the"/"that"/"this" whenever a shard had run
// earlier in the session, answering about the last shard result instead of
// the user's actual question. Follow-ups now flow through the single
// perception → route_decision → articulation pipeline; articulation carries
// lastShardResult and the shard history blackboard in ConversationContext, so
// "why is that bad?" still answers from prior context — deterministically.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"

	tea "github.com/charmbracelet/bubbletea"
)

// getRecentTurns returns the last N conversation turns.
func (m Model) getRecentTurns(n int) []Message {
	if len(m.history) <= n {
		return m.history
	}
	return m.history[len(m.history)-n:]
}

func (m Model) createAgentFromPrompt(description string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), config.GetLLMTimeouts().FollowUpTimeout)
		defer cancel()

		systemPrompt := "You design specialist software agents. Respond in English. Return JSON with fields: name (CamelCase, no spaces), topic (<=80 chars), knowledge_path (path string). Keep responses compact."
		userPrompt := fmt.Sprintf("Workspace: %s\nSpecialist description: %s", m.workspace, description)

		raw, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			return errorMsg(fmt.Errorf("agent creation failed: %w", err))
		}

		var out struct {
			Name          string `json:"name"`
			Topic         string `json:"topic"`
			KnowledgePath string `json:"knowledge_path"`
		}

		if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
			return errorMsg(fmt.Errorf("agent creation: invalid JSON from LLM: %w (got: %s)", err, raw))
		}

		name := strings.TrimSpace(out.Name)
		if name == "" {
			return errorMsg(fmt.Errorf("agent creation: LLM returned empty name"))
		}
		topic := strings.TrimSpace(out.Topic)
		kp := strings.TrimSpace(out.KnowledgePath)
		if kp == "" {
			// Use workspace root for .nerd path to avoid creating in wrong directory
			kp = filepath.Join(m.workspace, ".nerd", "shards", fmt.Sprintf("%s_knowledge.db", name))
		}

		cfg := coreshards.DefaultSpecialistConfig(name, kp)
		m.shardMgr.DefineProfile(name, cfg)
		_ = persistAgentProfile(m.workspace, name, "persistent", kp, 0, "ready")

		surface := fmt.Sprintf("## Agent Created: %s\n\n**Topic**: %s\n**Knowledge Path**: %s\n\nNext: `/spawn %s <task>`", name, topic, kp, name)
		return responseMsg(surface)
	}
}
