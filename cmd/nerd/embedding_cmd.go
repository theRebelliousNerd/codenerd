package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/store"

	"github.com/spf13/cobra"
)

// embeddingCmd mirrors the TUI /embedding commands for non-interactive use.
var embeddingCmd = &cobra.Command{
	Use:   "embedding",
	Short: "Embedding engine operations (set, stats, reembed)",
	Long: `Manage vector embedding configuration and maintenance.

This command mirrors the interactive TUI /embedding commands.`,
}

var embeddingSetCmd = &cobra.Command{
	Use:   "set <ollama|genai> [api-key]",
	Short: "Set embedding provider (and optional API key)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		cfgPath := filepath.Join(ws, ".nerd", "config.json")
		cfg, err := config.LoadUserConfig(cfgPath)
		if err != nil || cfg == nil {
			cfg = config.DefaultUserConfig()
		}
		if cfg.Embedding == nil {
			cfg.Embedding = &config.EmbeddingConfig{}
		}

		provider := args[0]
		cfg.Embedding.Provider = provider
		switch provider {
		case "ollama":
			if cfg.Embedding.OllamaEndpoint == "" {
				cfg.Embedding.OllamaEndpoint = "http://localhost:11434"
			}
			if cfg.Embedding.OllamaModel == "" {
				cfg.Embedding.OllamaModel = "embeddinggemma"
			}
		case "genai":
			if len(args) >= 2 {
				cfg.Embedding.GenAIAPIKey = args[1]
			}
			if cfg.Embedding.GenAIModel == "" {
				cfg.Embedding.GenAIModel = "gemini-embedding-001"
			}
			if cfg.Embedding.TaskType == "" {
				cfg.Embedding.TaskType = "SEMANTIC_SIMILARITY"
			}
		default:
			return fmt.Errorf("unsupported provider %q (use ollama or genai)", provider)
		}

		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Printf("✓ Embedding provider set to %s. Restart codeNERD to apply.\n", provider)
		return nil
	},
}

var embeddingStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show vector embedding statistics for knowledge.db",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		dbPath := filepath.Join(ws, ".nerd", "knowledge.db")
		ls, err := store.NewLocalStore(dbPath)
		if err != nil {
			return err
		}
		defer ls.Close()

		// Attach current embedding engine for reporting.
		cfgPath := filepath.Join(ws, ".nerd", "config.json")
		cfg, _ := config.LoadUserConfig(cfgPath)
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
		if engine, engErr := embedding.NewEngine(embCfg); engErr == nil {
			ls.SetEmbeddingEngine(engine)
		}

		stats, err := ls.GetVectorStats()
		if err != nil {
			return err
		}

		fmt.Printf("Embedding Statistics:\n  Total Vectors: %v\n  With Embeddings: %v\n  Without Embeddings: %v\n  Engine: %v\n  Dimensions: %v\n",
			stats["total_vectors"],
			stats["with_embeddings"],
			stats["without_embeddings"],
			stats["embedding_engine"],
			stats["embedding_dimensions"],
		)
		return nil
	},
}

var embeddingReembedCmd = &cobra.Command{
	Use:   "reembed",
	Short: "Force re-embed all .nerd + internal DBs",
	RunE: func(cmd *cobra.Command, args []string) error {
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		cfgPath := filepath.Join(ws, ".nerd", "config.json")
		cfg, _ := config.LoadUserConfig(cfgPath)
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
			return err
		}

		roots := []string{
			filepath.Join(ws, ".nerd"),
			filepath.Join(ws, "internal"),
		}

		fmt.Println("Re-embedding all databases (vectors + prompt atoms + traces + learnings)...")
		res, err := store.ReembedAllDBsForce(context.Background(), roots, engine, func(msg string) {
			fmt.Println(msg)
		})
		if err != nil {
			return err
		}

		fmt.Printf("\nRe-embedding complete.\n  DBs processed: %d\n  Vectors re-embedded: %d\n  Prompt atoms re-embedded: %d\n  Traces re-embedded: %d\n  Learnings re-embedded: %d\n  Duration: %.2fs\n",
			res.DBCount, res.VectorsDone, res.AtomsDone, res.TracesDone, res.LearningsDone, res.Duration.Seconds())
		if len(res.Skipped) > 0 {
			fmt.Println("\nSkipped/errored DBs:")
			for _, s := range res.Skipped {
				fmt.Printf("  - %s\n", s)
			}
		}
		return nil
	},
}

func init() {
	embeddingCmd.AddCommand(embeddingSetCmd)
	embeddingCmd.AddCommand(embeddingStatsCmd)
	embeddingCmd.AddCommand(embeddingReembedCmd)
}
