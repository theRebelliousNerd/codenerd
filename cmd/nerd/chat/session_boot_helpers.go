package chat

import (
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/session"
	"codenerd/internal/shards"

	// Domain shards removed - JIT clean loop handles these via prompt atoms:
	// "codenerd/internal/shards/coder"
	// "codenerd/internal/shards/nemesis"
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/reviewer"
	// "codenerd/internal/shards/tester"
	// "codenerd/internal/shards/tool_generator"

	nerdsystem "codenerd/internal/system"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// taskExecutorObserverSpawner adapts TaskExecutor to ObserverSpawner interface.
type taskExecutorObserverSpawner struct {
	executor session.TaskExecutor
}

func (s *taskExecutorObserverSpawner) SpawnObserver(ctx context.Context, observerName, task string) (string, error) {
	if s.executor == nil {
		return "", fmt.Errorf("task executor not available")
	}
	req := session.TaskRequest{
		IntentVerb: observerName,
		Task:       task,
	}
	return s.executor.Execute(ctx, req)
}

// taskExecutorConsultationSpawner adapts TaskExecutor to ConsultationSpawner interface.
type taskExecutorConsultationSpawner struct {
	executor session.TaskExecutor
}

func (s *taskExecutorConsultationSpawner) SpawnConsultation(ctx context.Context, specialistName, task string) (string, error) {
	if s.executor == nil {
		return "", fmt.Errorf("task executor not available")
	}
	req := session.TaskRequest{
		IntentVerb: specialistName,
		Task:       task,
	}
	return s.executor.Execute(ctx, req)
}

// northstarHandlerAdapter adapts northstar.BackgroundEventHandler to shards.NorthstarHandler interface.
type northstarHandlerAdapter struct {
	handler *northstar.BackgroundEventHandler
}

func (a *northstarHandlerAdapter) HandleEvent(ctx context.Context, event shards.ObserverEvent) (*shards.ObserverAssessment, error) {
	// Convert shards event to northstar handler call
	assessment, err := a.handler.HandleEvent(ctx, string(event.Type), event.Source, event.Target, event.Details, event.Timestamp)
	if err != nil {
		return nil, err
	}
	if assessment == nil {
		return nil, nil
	}

	// Convert northstar assessment to shards assessment
	return &shards.ObserverAssessment{
		ObserverName: assessment.ObserverName,
		EventID:      assessment.EventID,
		Score:        assessment.Score,
		Level:        shards.AssessmentLevel(assessment.Level),
		VisionMatch:  assessment.VisionMatch,
		Deviations:   assessment.Deviations,
		Suggestions:  assessment.Suggestions,
		Metadata:     assessment.Metadata,
		Timestamp:    assessment.Timestamp,
	}, nil
}

func hydrateNerdState(workspace string, kernel *core.RealKernel, shardMgr *coreshards.ShardManager, initialMessages *[]Message) (*Session, *Preferences) {
	nerdDir := filepath.Join(workspace, ".nerd")

	// Load profile facts
	profilePath := filepath.Join(nerdDir, "profile.mg")
	if info, err := os.Stat(profilePath); err == nil && !info.IsDir() {
		if err := kernel.LoadFactsFromFile(profilePath); err != nil {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to load .nerd/profile.mg: %v", err),
				Time:    time.Now(),
			})
		}
	} else if err != nil && !os.IsNotExist(err) {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Unable to access .nerd/profile.mg: %v", err),
			Time:    time.Now(),
		})
	}

	// Load preferences
	var prefs *Preferences
	prefPath := filepath.Join(nerdDir, "preferences.json")
	if data, err := os.ReadFile(prefPath); err == nil {
		var p Preferences
		if err := json.Unmarshal(data, &p); err == nil {
			prefs = &p
		} else {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to parse .nerd/preferences.json: %v", err),
				Time:    time.Now(),
			})
		}
	}

	// QUIESCENT BOOT: Always start fresh sessions.
	// Previous sessions can be resumed explicitly via /sessions command.
	// This prevents stale state from affecting new sessions.
	var session *Session
	// Generate a new session ID for this boot
	newSessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	session = &Session{
		SessionID: newSessionID,
		StartedAt: time.Now().Format(time.RFC3339),
		TurnCount: 0,
	}
	logging.Session("Starting fresh session: %s", newSessionID)

	// Check if there are previous sessions to hint about
	if sessions, err := nerdinit.ListSessionHistories(workspace); err == nil && len(sessions) > 0 {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("*Fresh session started.* Use `/sessions` to load previous sessions (%d available).", len(sessions)),
			Time:    time.Now(),
		})
	}

	// Ensure .nerd/agents.json reflects any agents present under .nerd/agents/*.
	// This keeps the registry in sync even when agents are created/edited outside of /init.
	if err := nerdsystem.SyncAgentRegistryFromDisk(workspace); err != nil {
		*initialMessages = append(*initialMessages, Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Warning: failed to sync agent registry from .nerd/agents: %v", err),
			Time:    time.Now(),
		})
	}

	// Load agents registry and hydrate shard profiles
	agentsPath := filepath.Join(nerdDir, "agents.json")
	if data, err := os.ReadFile(agentsPath); err == nil {
		var reg Registry
		if err := json.Unmarshal(data, &reg); err == nil {
			for _, agent := range reg.Agents {
				cfg := coreshards.DefaultSpecialistConfig(agent.Name, agent.KnowledgePath)
				if agent.Type != "" {
					cfg.Type = types.ShardType(agent.Type)
				}
				shardMgr.DefineProfile(agent.Name, cfg)
			}
		} else {
			*initialMessages = append(*initialMessages, Message{
				Role:    "assistant",
				Content: fmt.Sprintf("Failed to parse .nerd/agents.json: %v", err),
				Time:    time.Now(),
			})
		}
	}

	return session, prefs
}

// hydrateAllTools loads all tools into the VirtualStore's tool registry.
// Sources:
// 0. Built-in modular tools (core, shell, codedom, research)
// 1. available_tools.json - Static language/framework tools from init
// 2. .compiled/ directory - Autopoiesis-generated tools
func hydrateAllTools(virtualStore *core.VirtualStore, nerdDir string) error {
	var warnings []string

	// 0. Register built-in modular tools (core, shell, codedom, research)
	if err := virtualStore.HydrateModularTools(); err != nil {
		warnings = append(warnings, fmt.Sprintf("modular tools: %v", err))
	}

	// 1. Load static tools from available_tools.json
	if toolDefs, err := nerdinit.LoadToolsFromFile(nerdDir); err == nil && len(toolDefs) > 0 {
		// Convert init.ToolDefinition to core.StaticToolDef
		staticDefs := make([]core.StaticToolDef, len(toolDefs))
		for i, td := range toolDefs {
			staticDefs[i] = core.StaticToolDef{
				Name:          td.Name,
				Category:      td.Category,
				Description:   td.Description,
				Command:       td.Command,
				ShardAffinity: td.ShardAffinity,
			}
		}
		if err := virtualStore.HydrateStaticTools(staticDefs); err != nil {
			warnings = append(warnings, fmt.Sprintf("static tools: %v", err))
		}
	} else if err != nil {
		warnings = append(warnings, fmt.Sprintf("load available_tools.json: %v", err))
	}

	// 2. Restore compiled tools from disk and sync from Ouroboros
	if err := virtualStore.HydrateToolsFromDisk(nerdDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("compiled tools: %v", err))
	}

	if len(warnings) > 0 {
		return fmt.Errorf("%d issues: %s", len(warnings), strings.Join(warnings, "; "))
	}
	return nil
}
