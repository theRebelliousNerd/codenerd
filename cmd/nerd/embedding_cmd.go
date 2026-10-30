package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

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
	RunE: parentGroupRunE,
}

var embeddingSetCmd = &cobra.Command{
	Use:   "set <ollama|genai> [api-key]",
	Short: "Set embedding provider and model: `set ollama <model>` or `set genai <api-key> <model>`",
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
			if len(args) >= 2 {
				cfg.Embedding.OllamaModel = args[1]
			}
		case "genai":
			if len(args) >= 2 {
				cfg.Embedding.GenAIAPIKey = args[1]
			}
			if len(args) >= 3 {
				cfg.Embedding.GenAIModel = args[2]
			}
			if cfg.Embedding.TaskType == "" {
				cfg.Embedding.TaskType = "SEMANTIC_SIMILARITY"
			}
		default:
			return fmt.Errorf("unsupported provider %q (use ollama or genai)", provider)
		}
		// The model is the user's to name: nothing is written to config.json
		// that they did not type or already have there.
		if missing := cfg.Embedding.MissingModel(); missing != "" {
			return fmt.Errorf("%s", missing)
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
		return runEmbeddingStats(cmd.OutOrStdout(), ws)
	},
}

// runEmbeddingStats describes the vectors in the workspace's knowledge.db:
// how many there are, which embedding space each was written in, and how
// many the semantic index holds. It observes; it attaches no engine.
//
// It used to attach the configured engine to name it. Attaching drops and
// rebuilds vec_index and runs the provider's readiness check (an Ollama
// model list, and a pull when the model is missing), and the command then
// closed the store, possibly before the rebuild finished: asking for
// statistics could leave the index emptier than it found it (audit
// embedding-stats-observation-boundary). The configured engine is reported
// as configured; its provider-resolved name (an Ollama tag) is what a live
// engine stamps, which is why rows can show under a fuller name.
func runEmbeddingStats(w io.Writer, ws string) error {
	dbPath := filepath.Join(ws, ".nerd", "knowledge.db")
	ls, err := store.NewLocalStore(dbPath)
	if err != nil {
		return err
	}
	defer ls.Close()

	stats, err := ls.GetVectorStats()
	if err != nil {
		return err
	}
	spaces, err := ls.VectorSpaceCounts()
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Embedding Statistics:\n  Total Vectors: %v\n  With Embeddings: %v\n  Without Embeddings: %v\n",
		stats["total_vectors"], stats["with_embeddings"], stats["without_embeddings"])

	cfg, _ := config.LoadUserConfig(filepath.Join(ws, ".nerd", "config.json"))
	if cfg == nil {
		cfg = config.DefaultUserConfig()
	}
	emb := cfg.GetEmbeddingConfig()
	model := emb.OllamaModel
	if emb.Provider == "genai" {
		model = emb.GenAIModel
	}
	if model == "" {
		model = "(no model configured)"
	}
	fmt.Fprintf(w, "  Configured engine: %s %s\n", emb.Provider, model)

	fmt.Fprintln(w, "  Embedded vectors by embedding space:")
	names := make([]string, 0, len(spaces))
	for name := range spaces {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		label := name
		if label == "" {
			label = "(written before the embedding space was recorded)"
		}
		fmt.Fprintf(w, "    %s: %d\n", label, spaces[name])
	}
	if rows, ok := ls.VecIndexRows(); ok {
		fmt.Fprintf(w, "  Semantic index rows: %d\n", rows)
	} else {
		fmt.Fprintln(w, "  Semantic index: none (sqlite-vec unavailable, or no engine has built it)")
	}
	fmt.Fprintln(w, "  Rows in a space other than the live engine's do not rank; `nerd embedding reembed` brings them into it.")
	return nil
}

var embeddingReembedCmd = &cobra.Command{
	Use:   "reembed",
	Short: "Force re-embed every database under .nerd",
	RunE:  runEmbeddingReembed,
}

type embeddingReembedOps struct {
	newEngine func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error)
	reembed   func(context.Context, []string, embedding.EmbeddingEngine, store.ReembedProgressFn) (store.ReembedResult, error)
}

func runEmbeddingReembed(cmd *cobra.Command, args []string) error {
	return runEmbeddingReembedWithOps(cmd, args, embeddingReembedOps{
		newEngine: embedding.NewEngineWithContext,
		reembed:   store.ReembedAllDBsForce,
	})
}

func runEmbeddingReembedWithOps(cmd *cobra.Command, _ []string, ops embeddingReembedOps) (outcome error) {
	ctx, cancel := commandContext(cmd)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("embedding reembed: %w", err)
	}

	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}

	// Always read model/provider from .nerd/config.json via GetEmbeddingConfig.
	cfgPath := filepath.Join(ws, ".nerd", "config.json")
	cfg, _ := config.LoadUserConfig(cfgPath)
	if cfg == nil {
		cfg = config.DefaultUserConfig()
	}
	ucEmb := cfg.GetEmbeddingConfig()
	embCfg := ucEmb.EngineConfig()

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("embedding reembed: %w", err)
	}
	engine, err := ops.newEngine(ctx, embCfg)
	if err != nil {
		return err
	}
	if engine == nil {
		return errors.New("embedding reembed: engine factory returned nil engine")
	}
	if closer, ok := engine.(interface{ Close() error }); ok {
		defer func() {
			if closeErr := closer.Close(); closeErr != nil {
				outcome = errors.Join(outcome, fmt.Errorf("close embedding engine: %w", closeErr))
			}
		}()
	}

	roots := store.ReembedSearchRoots(ws)

	fmt.Println("Re-embedding all databases (vectors + prompt atoms + traces + learnings)...")
	res, err := ops.reembed(ctx, roots, engine, func(msg string) {
		fmt.Println(msg)
	})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("embedding reembed: %w", err)
	}

	verdict := "Re-embedding complete."
	if !res.Complete {
		verdict = "Re-embedding INCOMPLETE: some rows are still outside the target embedding space."
	}
	fmt.Printf("\n%s\n  DBs processed: %d\n  Vectors re-embedded: %d\n  Prompt atoms re-embedded: %d\n  Traces re-embedded: %d\n  Learnings re-embedded: %d\n  Duration: %.2fs\n",
		verdict, res.DBCount, res.VectorsDone, res.AtomsDone, res.TracesDone, res.LearningsDone, res.Duration.Seconds())
	if res.Complete {
		return nil
	}
	fmt.Println("\nNot migrated (re-run `nerd embedding reembed` to resume vectors where they stopped):")
	for _, m := range res.Stores {
		if m.Complete {
			continue
		}
		fmt.Printf("  - %s [%s]: %s\n", m.Path, m.Table, m.Detail)
	}
	return errors.New("embedding reembed incomplete: see the list above")
}

func init() {
	embeddingCmd.AddCommand(embeddingSetCmd)
	embeddingCmd.AddCommand(embeddingStatsCmd)
	embeddingCmd.AddCommand(embeddingReembedCmd)
}
