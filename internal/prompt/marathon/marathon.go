package marathon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// Config configures one marathon run.
type Config struct {
	// Workspace is the project root; the overlay and checkpoint live under
	// .nerd/prompts/.
	Workspace string

	// Client serves both the research phase (it must implement
	// types.GroundedWebSearcher and types.ModelIdentifier) and the optimization
	// phase. Using one client is deliberate: the atoms are being optimized FOR
	// this model, so it is also the most appropriate judge of how to phrase
	// them for itself.
	Client types.LLMClient

	// EmbedEngine embeds emitted variants so they are reachable by vector
	// search, exactly as the shipped corpus is.
	EmbedEngine embedding.EmbeddingEngine

	// MaxAtoms bounds a single run. Zero means the whole corpus. Progress is
	// checkpointed either way, so a bounded run followed by a re-run is
	// equivalent to one long run.
	MaxAtoms int

	// Categories restricts the pass to specific atom categories. Empty means
	// all of them.
	Categories []prompt.AtomCategory

	// Resume continues from an existing checkpoint instead of starting over.
	// Default true; a fresh run is the explicit choice.
	Resume bool

	// Progress receives one update per atom when non-nil.
	Progress func(Progress)
}

// Progress is a single tick of the optimization phase.
type Progress struct {
	AtomID    string
	Index     int
	Total     int
	Optimized bool
	Err       error
}

// checkpoint records which atoms a run has already decided, so a resumed run
// does not pay for them twice.
type checkpoint struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	ModelPin string `json:"model_pin"`

	// Decided maps base atom ID -> base content hash at the time it was
	// decided. The hash is what makes resume correct rather than merely fast:
	// if a shipped atom changed since the checkpoint was written, its previous
	// decision no longer applies and it is re-optimized.
	Decided map[string]string `json:"decided"`

	UpdatedAt time.Time `json:"updated_at"`
}

// Run executes the marathon: resolve identity, research, optimize, emit.
//
// The phases are ordered so that every hard failure happens before any work is
// done and before anything is written. A run that cannot be grounded costs one
// failed search, not a partially rewritten corpus.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	start := time.Now()

	if strings.TrimSpace(cfg.Workspace) == "" {
		return nil, fmt.Errorf("marathon: workspace is required")
	}
	if cfg.Client == nil {
		return nil, fmt.Errorf("marathon: an LLM client is required")
	}

	// Phase 1 -- serving identity. Hard fail.
	provider, model, err := ServingIdentity(cfg.Client)
	if err != nil {
		return nil, err
	}
	logging.Get(logging.CategoryJIT).Info("Marathon: serving identity provider=%q model=%q", provider, model)

	// Phase 2 -- research. Hard fail on unavailable grounding or no docs.
	researcher, err := NewResearcher(cfg.Client)
	if err != nil {
		return nil, err
	}
	profile, err := researcher.Research(ctx, provider, model)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Provider:  provider,
		Model:     model,
		Citations: profile.Citations,
	}

	// Phase 3 -- load the shipped corpus. Read-only from here on.
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return nil, fmt.Errorf("marathon: load embedded corpus: %w", err)
	}
	bases := selectBases(corpus.All(), cfg.Categories)
	result.AtomsConsidered = len(bases)

	optimizer, err := NewOptimizer(cfg.Client, profile)
	if err != nil {
		return nil, err
	}

	// Phase 4 -- open the overlay and checkpoint.
	db, err := openCorpusDB(cfg.Workspace)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	cp := loadCheckpoint(cfg, profile)
	loader := prompt.NewAtomLoader(cfg.EmbedEngine)

	// Phase 5 -- optimize and emit, one atom at a time.
	budget := cfg.MaxAtoms
	for i, base := range bases {
		if err := ctx.Err(); err != nil {
			// Cancellation keeps everything decided so far; the checkpoint has
			// already been flushed per atom.
			result.Duration = time.Since(start)
			return result, err
		}

		if hash, done := cp.Decided[base.ID]; done && hash == base.ContentHash {
			result.AtomsResumed++
			continue
		}
		if budget > 0 && (result.AtomsOptimized+result.AtomsUnchanged+result.AtomsFailed) >= budget {
			break
		}

		optimized, optErr := optimizer.Optimize(ctx, base)
		logOptimization(base, optimized)

		switch {
		case optErr != nil:
			result.AtomsFailed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", base.ID, optErr))
			// Not checkpointed: a failure should be retried on the next run.
		case optimized == nil:
			result.AtomsUnchanged++
			cp.markDecided(base)
		default:
			if err := emitVariant(ctx, loader, db, optimized); err != nil {
				result.AtomsFailed++
				result.Errors = append(result.Errors, fmt.Sprintf("emit %s: %v", optimized.Atom.ID, err))
				break
			}
			result.AtomsOptimized++
			cp.markDecided(base)
		}

		if err := cp.save(cfg.Workspace); err != nil {
			logging.Get(logging.CategoryJIT).Warn("Marathon: checkpoint save failed: %v", err)
		}

		if cfg.Progress != nil {
			cfg.Progress(Progress{
				AtomID:    base.ID,
				Index:     i + 1,
				Total:     len(bases),
				Optimized: optimized != nil,
				Err:       optErr,
			})
		}
	}

	result.Duration = time.Since(start)
	logging.Get(logging.CategoryJIT).Info(
		"Marathon complete: optimized=%d unchanged=%d failed=%d resumed=%d in %v",
		result.AtomsOptimized, result.AtomsUnchanged, result.AtomsFailed,
		result.AtomsResumed, result.Duration)

	return result, nil
}

// selectBases filters the corpus to the atoms this run will consider, in a
// stable order so a resumed run walks the same sequence.
//
// Atoms that are already pinned are skipped. They are either someone else's
// vendor-specific atom or a variant from a previous marathon; re-optimizing a
// variant would produce a variant of a variant, pinned to the same model,
// superseding an atom that already supersedes the original.
func selectBases(all []*prompt.PromptAtom, categories []prompt.AtomCategory) []*prompt.PromptAtom {
	allowed := map[prompt.AtomCategory]struct{}{}
	for _, c := range categories {
		allowed[c] = struct{}{}
	}

	var out []*prompt.PromptAtom
	for _, a := range all {
		if a == nil || strings.TrimSpace(a.Content) == "" {
			continue
		}
		if len(a.Providers) > 0 || len(a.Models) > 0 {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[a.Category]; !ok {
				continue
			}
		}
		out = append(out, a)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// emitVariant writes one optimized atom into the workspace overlay.
func emitVariant(ctx context.Context, loader *prompt.AtomLoader, db *sql.DB, optimized *OptimizedAtom) error {
	return loader.StoreAtom(ctx, db, optimized.Atom)
}

// openCorpusDB opens the workspace's JIT prompt corpus, which is where the
// overlay lives. This is workspace state that `nerd init` regenerates from the
// shipped YAML, which is what makes the whole overlay revertible: delete the
// overlay rows, or re-run init, and the corpus is back to shipped.
func openCorpusDB(workspace string) (*sql.DB, error) {
	path := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("marathon: create prompts dir: %w", err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("marathon: open corpus db: %w", err)
	}
	return db, nil
}

func checkpointPath(workspace string) string {
	return filepath.Join(workspace, ".nerd", "prompts", "marathon_checkpoint.json")
}

// loadCheckpoint reads the existing checkpoint when resuming, and starts a fresh
// one otherwise.
//
// A checkpoint for a DIFFERENT model is discarded rather than resumed. Its
// decisions were made against another model's documentation and mean nothing
// here; resuming it would silently skip most of the corpus and report a
// near-instant success.
func loadCheckpoint(cfg Config, profile *ModelDocProfile) *checkpoint {
	fresh := &checkpoint{
		Provider: profile.Provider,
		Model:    profile.Model,
		ModelPin: profile.ModelPin,
		Decided:  map[string]string{},
	}
	if !cfg.Resume {
		return fresh
	}

	data, err := os.ReadFile(checkpointPath(cfg.Workspace))
	if err != nil {
		return fresh
	}

	var existing checkpoint
	if err := json.Unmarshal(data, &existing); err != nil {
		logging.Get(logging.CategoryJIT).Warn("Marathon: unreadable checkpoint, starting fresh: %v", err)
		return fresh
	}
	if existing.ModelPin != profile.ModelPin {
		logging.Get(logging.CategoryJIT).Info(
			"Marathon: checkpoint is for model %q, this run is %q; starting fresh",
			existing.ModelPin, profile.ModelPin)
		return fresh
	}
	if existing.Decided == nil {
		existing.Decided = map[string]string{}
	}
	logging.Get(logging.CategoryJIT).Info("Marathon: resuming, %d atoms already decided", len(existing.Decided))
	return &existing
}

func (c *checkpoint) markDecided(base *prompt.PromptAtom) {
	if c.Decided == nil {
		c.Decided = map[string]string{}
	}
	c.Decided[base.ID] = base.ContentHash
}

func (c *checkpoint) save(workspace string) error {
	c.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	// Atomic: a torn checkpoint is unreadable, and an unreadable checkpoint
	// restarts a run that may be hundreds of LLM calls deep.
	return atomicfile.WriteFile(checkpointPath(workspace), data, 0644)
}
