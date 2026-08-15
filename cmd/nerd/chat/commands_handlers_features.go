package chat

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/features"

	tea "github.com/charmbracelet/bubbletea"
)

// renderFeaturesReport formats the resolved feature-flag registry for chat.
//
// Kept separate from the handler so the rendering is testable without a
// bubbletea Model, and so `/features` and `nerd features` cannot disagree about
// what "resolved" means: both read features.Resolved().
func renderFeaturesReport() string {
	flags := features.Resolved()

	var sb strings.Builder
	sb.WriteString("## Feature Flags\n\n")
	sb.WriteString("Precedence: env → legacy-env → config → default.\n\n")
	sb.WriteString("| Flag | Value | Source | Default | Env var |\n")
	sb.WriteString("|------|-------|--------|---------|---------|\n")
	for _, f := range flags {
		envVar := f.EnvVar
		if f.LegacyEnvVar != "" {
			envVar += " (legacy: " + f.LegacyEnvVar + ")"
		}
		sb.WriteString(fmt.Sprintf("| %s | %t | %s | %t | `%s` |\n",
			f.Name, f.Value, f.Source, f.Default, envVar))
	}

	sb.WriteString(fmt.Sprintf("\n- `fast_scan_workers` %d (0 = call site default)\n",
		features.FastScanWorkers()))
	sb.WriteString(fmt.Sprintf("- `fast_ast_max_bytes` %d (0 = call site default)\n",
		features.FastASTMaxBytes()))

	if deprecations := features.Deprecations(); len(deprecations) > 0 {
		sb.WriteString("\n**Deprecated environment variables**\n\n")
		for _, msg := range deprecations {
			sb.WriteString("- " + msg + "\n")
		}
	}

	// The one-line form is what Boot logs, so showing it here lets an operator
	// match what they see in chat against what they see in session.log.
	sb.WriteString("\n```\n" + features.Summary() + "\n```\n")
	return sb.String()
}

// handleCmdFeatures handles the corresponding chat slash-command.
func (m Model) handleCmdFeatures(input string, parts []string) (tea.Model, tea.Cmd) {
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: renderFeaturesReport(),
		Time:    time.Now(),
	})
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	m.textarea.Reset()
	return m, nil
}
