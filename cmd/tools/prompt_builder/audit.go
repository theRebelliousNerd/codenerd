package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"codenerd/internal/atomjudge"
	"codenerd/internal/config"
	"codenerd/internal/embedding"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// Atomicity audit (-audit-atomicity): Jev reads every embedded atom (how much
// of it a capable engineer already knows, how many separable instructions it
// holds, how much of it gives no instruction) and, for each atom, whether its
// nearest neighbours by embedding already give every instruction it gives.
// The report is evidence for corpus edits; it changes nothing.

const (
	// auditParallel bounds concurrent Jev requests.
	auditParallel = 8
	// auditNeighbours is how many embedding neighbours each atom is compared
	// with, among those at or above auditSimilarity.
	auditNeighbours = 3
	// auditSimilarity is the cosine at which a neighbour is worth asking about.
	auditSimilarity = 0.88
	// auditEmbedBatch bounds texts per embedding request.
	auditEmbedBatch = 32
)

type auditReport struct {
	Model   string                      `json:"model"`
	Created time.Time                   `json:"created"`
	Atoms   []atomjudge.AtomicityResult `json:"atoms"`
	// Neighbours lists, per atom, the embedding neighbours it was compared
	// with and their cosine.
	Neighbours map[string]map[string]float64 `json:"neighbours"`
}

func runAuditAtomicity(ctx context.Context, configPath, outPath string) error {
	cfg, err := config.LoadUserConfig(configPath)
	if err != nil {
		return fmt.Errorf("load %s: %w", configPath, err)
	}
	client, err := typesafe.FromConfig(cfg.TypeSafe)
	if err != nil {
		return err
	}
	if client == nil {
		return errors.New("no typesafe section in " + configPath + "; the audit needs Jev")
	}
	engine, err := embedding.NewEngine(cfg.GetEmbeddingConfig().EngineConfig())
	if err != nil {
		return fmt.Errorf("embedding engine: %w", err)
	}
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return err
	}
	atoms := corpus.All()
	sort.Slice(atoms, func(i, j int) bool { return atoms[i].ID < atoms[j].ID })

	fmt.Printf("[..] embedding %d atoms with %s\n", len(atoms), engine.Name())
	vecs := make([][]float32, 0, len(atoms))
	for i := 0; i < len(atoms); i += auditEmbedBatch {
		end := min(i+auditEmbedBatch, len(atoms))
		texts := make([]string, 0, end-i)
		for _, a := range atoms[i:end] {
			texts = append(texts, a.Content)
		}
		batch, err := engine.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("embed atoms %d-%d: %w", i, end, err)
		}
		vecs = append(vecs, batch...)
	}
	neighbours := nearestByEmbedding(atoms, vecs)
	byID := make(map[string]*prompt.PromptAtom, len(atoms))
	for _, a := range atoms {
		byID[a.ID] = a
	}
	compare := func(a *prompt.PromptAtom) []*prompt.PromptAtom {
		ids := make([]string, 0, len(neighbours[a.ID]))
		for id := range neighbours[a.ID] {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return neighbours[a.ID][ids[i]] > neighbours[a.ID][ids[j]] })
		out := make([]*prompt.PromptAtom, 0, len(ids))
		for _, id := range ids {
			out = append(out, byID[id])
		}
		return out
	}
	fmt.Printf("[..] asking Jev (%s) about %d atoms\n", client.Model(), len(atoms))
	start := time.Now()
	results := atomjudge.JudgeCorpusAtomicity(ctx, client, atoms, compare, auditParallel)
	report := auditReport{Created: time.Now().UTC(), Atoms: results, Neighbours: neighbours}
	failed := 0
	for _, r := range results {
		if r.Err != "" {
			failed++
		} else if report.Model == "" {
			report.Model = r.Atomicity.Model
		}
	}
	data, err := json.MarshalIndent(report, "", " ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("[OK] %d atoms read in %s, %d without a reading; report %s\n", len(results)-failed, time.Since(start).Round(time.Second), failed, outPath)
	if failed > 0 {
		return fmt.Errorf("%d atoms have no reading; see the report", failed)
	}
	return nil
}

// nearestByEmbedding returns, per atom, up to auditNeighbours other atoms at
// or above auditSimilarity, excluding its own sections and head: a section is
// meant to continue its head, not to stand apart from it.
func nearestByEmbedding(atoms []*prompt.PromptAtom, vecs [][]float32) map[string]map[string]float64 {
	norm := make([][]float64, len(vecs))
	for i, v := range vecs {
		var n float64
		for _, x := range v {
			n += float64(x) * float64(x)
		}
		n = math.Sqrt(n)
		out := make([]float64, len(v))
		for j, x := range v {
			if n > 0 {
				out[j] = float64(x) / n
			}
		}
		norm[i] = out
	}
	related := func(a, b *prompt.PromptAtom) bool {
		return a.SectionOf == b.ID || b.SectionOf == a.ID || (a.SectionOf != "" && a.SectionOf == b.SectionOf)
	}
	out := make(map[string]map[string]float64, len(atoms))
	for i, a := range atoms {
		type cand struct {
			id  string
			sim float64
		}
		var cands []cand
		for j, b := range atoms {
			if i == j || related(a, b) {
				continue
			}
			var dot float64
			for k := range norm[i] {
				dot += norm[i][k] * norm[j][k]
			}
			if dot >= auditSimilarity {
				cands = append(cands, cand{b.ID, dot})
			}
		}
		sort.Slice(cands, func(x, y int) bool { return cands[x].sim > cands[y].sim })
		if len(cands) == 0 {
			continue
		}
		m := map[string]float64{}
		for _, c := range cands[:min(auditNeighbours, len(cands))] {
			m[c.id] = math.Round(c.sim*1000) / 1000
		}
		out[a.ID] = m
	}
	return out
}
