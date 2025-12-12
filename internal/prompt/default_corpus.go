package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/core/defaults"
)

// MaterializeDefaultPromptCorpus writes the embedded default prompt corpus DB to dstPath
// if (and only if) dstPath does not already exist.
//
// Returns (true, nil) if the file was written, (false, nil) if no write occurred.
func MaterializeDefaultPromptCorpus(dstPath string) (bool, error) {
	if strings.TrimSpace(dstPath) == "" {
		return false, fmt.Errorf("dstPath is required")
	}

	// Never clobber an existing corpus DB.
	if _, err := os.Stat(dstPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat dstPath: %w", err)
	}

	if !defaults.PromptCorpusAvailable() {
		return false, nil
	}

	data, err := defaults.PromptCorpusDB.ReadFile("prompt_corpus.db")
	if err != nil {
		return false, fmt.Errorf("read embedded prompt corpus: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return false, fmt.Errorf("mkdir prompts dir: %w", err)
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return false, fmt.Errorf("write corpus: %w", err)
	}

	return true, nil
}

// HydrateAtomContextTags ensures the atom_context_tags table contains tag rows for the
// provided atoms that already exist in the DB.
//
// This is useful when the corpus DB was produced by older tooling (or was seeded)
// and is missing normalized tag rows.
func HydrateAtomContextTags(ctx context.Context, db *sql.DB, atoms []*PromptAtom) error {
	if db == nil || len(atoms) == 0 {
		return nil
	}

	// Snapshot which atoms exist in the DB to avoid inserting orphan tag rows.
	ids := make(map[string]struct{})
	rows, err := db.QueryContext(ctx, "SELECT atom_id FROM prompt_atoms")
	if err != nil {
		return fmt.Errorf("query prompt_atoms ids: %w", err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids[id] = struct{}{}
	}
	_ = rows.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO atom_context_tags (atom_id, dimension, tag) VALUES (?, ?, ?)")
	if err != nil {
		return fmt.Errorf("prepare tag insert: %w", err)
	}
	defer stmt.Close()

	insertTags := func(atomID, dim string, values []string) error {
		for _, v := range values {
			if strings.TrimSpace(v) == "" {
				continue
			}
			if _, err := stmt.ExecContext(ctx, atomID, dim, v); err != nil {
				return err
			}
		}
		return nil
	}

	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		if _, ok := ids[atom.ID]; !ok {
			continue
		}

		if err := insertTags(atom.ID, "mode", atom.OperationalModes); err != nil {
			return fmt.Errorf("insert tags (mode) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "phase", atom.CampaignPhases); err != nil {
			return fmt.Errorf("insert tags (phase) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "layer", atom.BuildLayers); err != nil {
			return fmt.Errorf("insert tags (layer) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "init_phase", atom.InitPhases); err != nil {
			return fmt.Errorf("insert tags (init_phase) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "northstar_phase", atom.NorthstarPhases); err != nil {
			return fmt.Errorf("insert tags (northstar_phase) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "ouroboros_stage", atom.OuroborosStages); err != nil {
			return fmt.Errorf("insert tags (ouroboros_stage) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "intent", atom.IntentVerbs); err != nil {
			return fmt.Errorf("insert tags (intent) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "shard", atom.ShardTypes); err != nil {
			return fmt.Errorf("insert tags (shard) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "lang", atom.Languages); err != nil {
			return fmt.Errorf("insert tags (lang) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "framework", atom.Frameworks); err != nil {
			return fmt.Errorf("insert tags (framework) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "state", atom.WorldStates); err != nil {
			return fmt.Errorf("insert tags (state) for %s: %w", atom.ID, err)
		}

		if err := insertTags(atom.ID, "depends_on", atom.DependsOn); err != nil {
			return fmt.Errorf("insert tags (depends_on) for %s: %w", atom.ID, err)
		}
		if err := insertTags(atom.ID, "conflicts_with", atom.ConflictsWith); err != nil {
			return fmt.Errorf("insert tags (conflicts_with) for %s: %w", atom.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
