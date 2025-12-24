package chat

import (
	"context"
	"path/filepath"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/store"

	tea "github.com/charmbracelet/bubbletea"
)

// reembedCompleteMsg is sent when re-embedding finishes.
type reembedCompleteMsg struct {
	dbCount     int
	vectorsDone int
	atomsDone   int
	skipped     []string
	duration    time.Duration
	err         error
}

// runReembedAllDBs force re-embeds vectors and prompt atoms across all .db files
// in .nerd/ and internal/.
func (m Model) runReembedAllDBs() tea.Cmd {
	return func() tea.Msg {
		m.ReportStatus("Re-embedding all databases...")

		// Build embedding engine from current config (so no restart required).
		cfg, _ := config.GlobalConfig()
		embCfg := embedding.DefaultConfig()
		if cfg != nil && cfg.Embedding != nil {
			embCfg = embedding.Config{
				Provider:       cfg.Embedding.Provider,
				OllamaEndpoint: cfg.Embedding.OllamaEndpoint,
				OllamaModel:    cfg.Embedding.OllamaModel,
				GenAIAPIKey:    cfg.Embedding.GenAIAPIKey,
				GenAIModel:     cfg.Embedding.GenAIModel,
				TaskType:       cfg.Embedding.TaskType,
			}
		}

		engine, err := embedding.NewEngine(embCfg)
		if err != nil {
			return reembedCompleteMsg{err: err}
		}

		searchRoots := []string{
			filepath.Join(m.workspace, ".nerd"),
			filepath.Join(m.workspace, "internal"),
		}

		res, err := store.ReembedAllDBsForce(
			context.Background(),
			searchRoots,
			engine,
			func(msg string) { m.ReportStatus(msg) },
		)
		if err != nil {
			return reembedCompleteMsg{err: err}
		}

		m.ReportStatus("Re-embedding complete")

		return reembedCompleteMsg{
			dbCount:     res.DBCount,
			vectorsDone: res.VectorsDone,
			atomsDone:   res.AtomsDone,
			skipped:     res.Skipped,
			duration:    res.Duration,
		}
	}
}
