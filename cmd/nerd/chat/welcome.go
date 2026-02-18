// Package chat — welcome.go implements the post-boot LLM health check.
//
// After the workspace scan completes, codeNERD fires a single LLM call
// to verify the configured provider is reachable and to generate a
// project-aware welcome message. This serves dual purposes:
//
//  1. Health check — confirms the API key, network, and model are working
//  2. Warm greeting — gives the user a contextual welcome with project stats
//
// The call is non-blocking (runs as a tea.Cmd goroutine) and has a short
// timeout. If it fails, the user sees a clear warning but can still use
// slash commands and other non-LLM features.
package chat

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
	tea "github.com/charmbracelet/bubbletea"
)

// performWelcomeHealthCheck fires a single LLM call after boot to verify
// the provider is working and generate a project-aware welcome message.
//
// Design decisions:
//   - 10s timeout: generous enough for cold starts, short enough to not block UX
//   - Uses m.shutdownCtx as parent: cancels cleanly on quit
//   - Captures scan stats by value (msg is a copy) so the goroutine is safe
//   - Returns welcomeHealthCheckMsg which the Update() switch handles
func (m Model) performWelcomeHealthCheck(scanStats scanCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		// Guard: if client wasn't wired during boot, fail gracefully
		if m.client == nil {
			return welcomeHealthCheckMsg{
				err: fmt.Errorf("LLM client not initialized — check your config"),
			}
		}

		// Short timeout — this is a smoke test, not a conversation
		ctx, cancel := context.WithTimeout(m.shutdownCtx, 10*time.Second)
		defer cancel()

		projectName := filepath.Base(m.workspace)
		provider := m.Config.Provider
		model := m.Config.Model

		// Build a concise system prompt that constrains output length
		systemPrompt := `You are codeNERD, a neuro-symbolic coding agent powered by a Mangle logic kernel. Generate a brief welcome message for the user who just opened a coding session.

Rules:
- 2-3 sentences maximum
- Mention the project name naturally
- Reference one interesting stat (file count, fact count, or scan speed)
- Be warm but professional — no gushing, no emojis
- Do NOT use markdown headers (no # or ##)
- Do NOT repeat the stats table — the user already saw it
- Keep it under 280 characters if possible`

		userPrompt := fmt.Sprintf(`Project: %s
Provider: %s | Model: %s
Files indexed: %d | Directories: %d
Facts generated: %d | Scan duration: %.2fs`,
			projectName, provider, model,
			scanStats.fileCount, scanStats.directoryCount,
			scanStats.factCount, scanStats.duration.Seconds(),
		)

		logging.BootDebug("LLM health check: provider=%s model=%s project=%s", provider, model, projectName)

		response, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			logging.API("LLM health check failed: provider=%s model=%s err=%v", provider, model, err)
			return welcomeHealthCheckMsg{
				err:      err,
				provider: provider,
				model:    model,
			}
		}

		// Trim any accidental whitespace from the LLM response
		response = strings.TrimSpace(response)

		logging.BootDebug("LLM health check succeeded: provider=%s model=%s response_len=%d", provider, model, len(response))

		return welcomeHealthCheckMsg{
			welcome:  response,
			provider: provider,
			model:    model,
		}
	}
}
