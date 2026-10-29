package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"codenerd/internal/atomjudge"
	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// importanceParallel bounds the concurrent Jev requests while scoring.
const importanceParallel = 4

// importanceOf is an atom's build-time level as the corpus column stores it:
// NULL when the ledger holds no score for exactly this content.
func importanceOf(ledger *prompt.ImportanceLedger, atom ProcessedAtom) any {
	level, ok := ledger.Lookup(atom.ID, atom.ContentHash)
	if !ok {
		return nil
	}
	return int64(level)
}

// runScoreImportance asks Jev the importance Score for every embedded atom the
// ledger does not cover (all of them with rescoreAll, or when the question's
// fingerprint changed) and rewrites the ledger. Atoms whose request failed
// stay out of the ledger and are listed; the run then fails.
func runScoreImportance(ctx context.Context, configPath, ledgerPath string, rescoreAll bool) error {
	cfg, err := config.LoadUserConfig(configPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", configPath, err)
	}
	client, err := typesafe.FromConfig(cfg.TypeSafe)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("no typesafe section in " + configPath + "; importance needs Jev")
	}
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return err
	}
	ledger := &prompt.ImportanceLedger{Atoms: map[string]prompt.ImportanceRecord{}}
	if data, err := os.ReadFile(ledgerPath); err == nil {
		if ledger, err = prompt.ParseImportanceLedger(data); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	scale := atomjudge.ImportanceScaleFingerprint()
	atoms := corpus.All()
	todo := atomjudge.StaleAtoms(ledger, atoms)
	if rescoreAll {
		todo = atoms
	}
	if ledger.Scale != scale {
		ledger.Atoms = map[string]prompt.ImportanceRecord{}
	}
	// Records for atoms no longer in the corpus are dropped.
	live := make(map[string]bool, len(atoms))
	for _, a := range atoms {
		live[a.ID] = true
	}
	for id := range ledger.Atoms {
		if !live[id] {
			delete(ledger.Atoms, id)
		}
	}
	fmt.Printf("[..] scoring %d of %d atoms with %s\n", len(todo), len(atoms), client.Model())
	start := time.Now()
	results := atomjudge.ScoreImportance(ctx, client, todo, importanceParallel)
	var failed []string
	levels := map[prompt.ImportanceLevel]int{}
	for _, r := range results {
		if r.Err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", r.AtomID, r.Err))
			delete(ledger.Atoms, r.AtomID)
			continue
		}
		ledger.Atoms[r.AtomID] = r.Record
		levels[r.Record.Level]++
		ledger.Model = r.Model
	}
	ledger.Scale = scale
	out, err := ledger.MarshalIndent()
	if err != nil {
		return err
	}
	if err := os.WriteFile(ledgerPath, out, 0o644); err != nil {
		return err
	}
	var dist []string
	for l := prompt.ImportanceStyle; l <= prompt.ImportanceCritical; l++ {
		dist = append(dist, fmt.Sprintf("%s=%d", l, levels[l]))
	}
	fmt.Printf("[OK] scored %d in %s (%s); ledger holds %d of %d atoms\n",
		len(results)-len(failed), time.Since(start).Round(time.Millisecond), strings.Join(dist, " "), len(ledger.Atoms), len(atoms))
	if len(failed) > 0 {
		sort.Strings(failed)
		return fmt.Errorf("%d atoms not scored:\n  %s", len(failed), strings.Join(failed, "\n  "))
	}
	return nil
}
