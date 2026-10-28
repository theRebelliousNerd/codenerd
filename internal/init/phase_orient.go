package init

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/northstar"
	"codenerd/internal/orient"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// plannerCompleter sends northstar derivation through withJITPrompt on the
// planner route. Northstar compiles its own system prompt, for the same
// client, from its derive corpus; phase labels the attempts in the metrics.
type plannerCompleter struct {
	owner *Initializer
	phase string
}

func (r plannerCompleter) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return r.owner.withJITPrompt(ctx, initCall{slot: slotPlanner, phase: r.phase, task: user, system: system},
		func(ctx context.Context, client perception.LLMClient, system string) (string, error) {
			return client.CompleteWithSystem(ctx, system, user)
		})
}

// priorOrientation is what the last orientation of this workspace derived and
// measured, for the parts of this run that only redo what changed.
type priorOrientation struct {
	exists  bool
	digests map[string]string       // oriented_document(Path, Digest)
	claims  map[string][]types.Fact // doc_role_claim and doc_theme rows by path
}

// loadPriorOrientation reads the last orientation snapshot. A workspace never
// oriented has none; an unreadable snapshot is reported and reused for
// nothing, so every document is classified again.
func loadPriorOrientation(workspace string) (priorOrientation, error) {
	prior := priorOrientation{digests: map[string]string{}, claims: map[string][]types.Fact{}}
	snapshot, err := orient.LoadSnapshot(workspace)
	if errors.Is(err, fs.ErrNotExist) {
		return prior, nil
	}
	if err != nil {
		return prior, fmt.Errorf("previous orientation unreadable; every document is classified again: %w", err)
	}
	prior.exists = true
	for _, fact := range snapshot.Facts {
		if len(fact.Args) < 2 {
			continue
		}
		path := types.ExtractString(fact.Args[0])
		switch fact.Predicate {
		case "oriented_document":
			prior.digests[path] = types.ExtractString(fact.Args[1])
		case "doc_role_claim", "doc_theme":
			prior.claims[path] = append(prior.claims[path], fact)
		}
	}
	return prior, nil
}

// documentDigests are the oriented_document rows of a fresh measurement.
func documentDigests(snapshot *orient.Snapshot) map[string]string {
	digests := map[string]string{}
	for _, fact := range snapshot.Facts {
		if fact.Predicate == "oriented_document" && len(fact.Args) >= 2 {
			digests[types.ExtractString(fact.Args[0])] = types.ExtractString(fact.Args[1])
		}
	}
	return digests
}

// reusableClaims splits the read candidates into those whose body is
// byte-identical to the last orientation and already carries a role claim,
// whose claims are reused, and those that must be classified.
func (p priorOrientation) reusableClaims(current map[string]string, documents []northstar.SourceDocument) (reused []types.Fact, reusedPaths []string, pending []northstar.SourceDocument) {
	for _, doc := range documents {
		rows := p.claims[doc.Path]
		digest, measured := p.digests[doc.Path]
		hasRole := false
		for _, row := range rows {
			if row.Predicate == "doc_role_claim" {
				hasRole = true
				break
			}
		}
		if !p.exists || !measured || digest == "" || digest != current[doc.Path] || !hasRole {
			pending = append(pending, doc)
			continue
		}
		reused = append(reused, rows...)
		reusedPaths = append(reusedPaths, doc.Path)
	}
	return reused, reusedPaths, pending
}

// documentsUnchanged reports whether the last orientation measured exactly
// the documents measured now, with identical bodies.
func (p priorOrientation) documentsUnchanged(current map[string]string) bool {
	if !p.exists || len(p.digests) != len(current) {
		return false
	}
	for path, digest := range current {
		if p.digests[path] != digest {
			return false
		}
	}
	return true
}

type orientationRun struct {
	Snapshot *orient.Snapshot
	Class    *northstar.Classification
	Brief    northstar.OrientationBrief
	Draft    *northstar.Draft
	Install  northstar.InstallResult
	Omitted  []types.Fact
	Notes    []string
}

func (i *Initializer) runOrientation(ctx context.Context, nerdDir string, result *InitResult) error {
	clearEcosystemSeed(i.config.Workspace)
	cfg, err := config.LoadOrientConfig(i.config.Workspace)
	if err != nil {
		return err
	}
	prior, priorErr := loadPriorOrientation(i.config.Workspace)
	snapshot, err := orient.Measure(ctx, i.config.Workspace, cfg, i.embedEngine)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, headErr := orient.Head(ctx, i.config.Workspace); headErr == nil {
			return fmt.Errorf("orientation census failed: %w", err)
		}
		result.Warnings = append(result.Warnings, "Orientation measurements unavailable: "+err.Error())
		snapshot = &orient.Snapshot{Config: cfg, Bodies: map[string]string{}, Notes: []string{err.Error()}}
		snapshot.Sources, err = orient.Discover(i.config.Workspace)
		if err != nil {
			snapshot.Notes = append(snapshot.Notes, err.Error())
		}
		snapshot.Facts = orient.Facts(snapshot.Sources)
	}
	eng, err := snapshot.Engine(ctx)
	if err != nil {
		return err
	}
	if i.orientation != nil {
		_ = i.orientation.Close()
	}
	i.orientation, i.orientationSnapshot = eng, snapshot
	run := orientationRun{Snapshot: snapshot}
	if priorErr != nil {
		run.Notes = append(run.Notes, priorErr.Error())
	}
	rows, err := eng.Query("orient_read_candidate")
	if err != nil {
		return err
	}
	run.Omitted, err = eng.Query("orient_read_omitted")
	if err != nil {
		return err
	}
	var documents []northstar.SourceDocument
	seen := map[string]bool{}
	for _, row := range rows {
		p := types.ExtractString(row.Args[0])
		if seen[p] {
			continue
		}
		seen[p] = true
		body, ok := snapshot.Bodies[p]
		if !ok {
			run.Notes = append(run.Notes, "Read candidate has no tracked document body: "+p)
			continue
		}
		documents = append(documents, northstar.SourceDocument{Path: p, Body: body})
	}
	sort.Slice(documents, func(a, b int) bool { return documents[a].Path < documents[b].Path })
	budget, budgetErr := northstar.ResolveDeriveBudget(i.config.Workspace)
	if budgetErr != nil {
		run.Notes = append(run.Notes, budgetErr.Error())
	}

	// Document roles and the north star are planner work: everything after
	// orientation reads them. Northstar compiles its derive prompts for the
	// client that will read them.
	planner := i.route(slotPlanner)
	phasePrompt := northstar.PhasePromptFor(northstar.PhaseTarget{
		Provider: planner.provider, Model: planner.model, InputWindow: planner.window,
	})
	current := documentDigests(snapshot)

	// A document whose body is unchanged since the last orientation keeps
	// the role claims it was given then; only new and changed documents are
	// classified again.
	reused, reusedPaths, pending := prior.reusableClaims(current, documents)
	var claims []types.Fact
	if len(reused) > 0 {
		claims = append(claims, reused...)
		run.Notes = append(run.Notes, fmt.Sprintf(
			"%d document(s) unchanged since the last orientation kept their role claims: %s",
			len(reusedPaths), strings.Join(reusedPaths, ", ")))
	}
	if len(pending) > 0 && planner.client != nil && budgetErr == nil {
		run.Class, err = northstar.ClassifyDocuments(ctx, plannerCompleter{owner: i, phase: "document_roles"}, phasePrompt, budget, pending)
		if err != nil {
			run.Notes = append(run.Notes, "Role transduction failed: "+err.Error())
		}
		claims = append(claims, run.Class.Facts()...)
	} else if len(pending) > 0 {
		run.Notes = append(run.Notes, "Role transduction unavailable; documents remain unclassified.")
	}
	if len(claims) > 0 {
		if err := eng.Assert(claims); err != nil {
			return err
		}
		snapshot.Facts = append(snapshot.Facts, claims...)
		if err := eng.Evaluate(ctx); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	run.Brief, err = orientationBrief(eng, snapshot)
	if err != nil {
		return err
	}
	if err := i.deriveNorthStar(ctx, nerdDir, &run, result, prior.documentsUnchanged(current), phasePrompt, budget, budgetErr); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Save(i.config.Workspace, eng); err != nil {
		return err
	}
	if err := i.writeOrientationReport(nerdDir, run, result); err != nil {
		return err
	}
	result.FilesCreated = append(result.FilesCreated, filepath.Join(nerdDir, "orientation", "orientation.mg"), filepath.Join(nerdDir, "orientation", "snapshot.json"))
	result.Warnings = append(result.Warnings, run.Notes...)
	return nil
}

// deriveNorthStar drafts the north star from the orientation brief and
// installs it when the workspace has none, then loads whatever vision the
// store holds, drafted now, earlier, or written by the user, so strategic
// knowledge is fed from the vision of record. A stored vision is not drafted
// again while every oriented document is unchanged.
func (i *Initializer) deriveNorthStar(ctx context.Context, nerdDir string, run *orientationRun, result *InitResult, documentsUnchanged bool, phasePrompt northstar.PhasePrompt, budget northstar.DeriveBudget, budgetErr error) error {
	store, err := northstar.NewStore(nerdDir)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	stored, err := store.LoadVision()
	if err != nil {
		return err
	}

	planner := i.route(slotPlanner)
	switch {
	case len(run.Brief.Vision) == 0:
		run.Notes = append(run.Notes, "No vision_source rows were available for drafting.")
	case stored != nil && documentsUnchanged:
		run.Notes = append(run.Notes, "Every oriented document is unchanged since the last orientation; the stored north star was kept without drafting again.")
	case planner.client == nil || budgetErr != nil:
		run.Notes = append(run.Notes, "North star drafting unavailable without an LLM client and a derive budget.")
	default:
		run.Draft, err = northstar.DraftVision(ctx, plannerCompleter{owner: i, phase: "north_star"}, phasePrompt, budget, run.Brief)
		if err != nil {
			run.Notes = append(run.Notes, "North star drafting failed: "+err.Error())
		}
		if run.Draft != nil {
			run.Install, err = northstar.InstallDerivedVision(store, nerdDir, run.Draft)
			if err != nil {
				return err
			}
			result.VisionDerived = run.Install.Installed
		}
	}
	i.orientedVision, err = store.LoadVision()
	return err
}

// orientationBrief renders the engine's judgments as the north-star brief.
// Source roles come from the engine's doc_role_claim rows, so claims reused
// from an earlier orientation reach the brief like fresh ones.
func orientationBrief(eng *orient.Engine, snapshot *orient.Snapshot) (northstar.OrientationBrief, error) {
	brief := northstar.OrientationBrief{}
	roles := map[string][]string{}
	claimRows, err := eng.Query("doc_role_claim")
	if err != nil {
		return brief, err
	}
	for _, row := range claimRows {
		if len(row.Args) < 2 {
			continue
		}
		path := types.ExtractString(row.Args[0])
		roles[path] = append(roles[path], strings.TrimPrefix(types.ExtractString(row.Args[1]), "/"))
	}
	months, err := eng.Query("repo_month")
	if err != nil {
		return brief, err
	}
	eras, err := eng.Query("repo_era")
	if err != nil {
		return brief, err
	}
	births, err := eng.Query("doc_birth_month")
	if err != nil {
		return brief, err
	}
	sort.Slice(eras, func(i, j int) bool { return orientationNumber(eras[i].Args[0]) < orientationNumber(eras[j].Args[0]) })
	for _, row := range months {
		brief.Months = append(brief.Months, northstar.MonthView{Index: orientationNumber(row.Args[0]), Label: types.ExtractString(row.Args[1]), Commits: orientationNumber(row.Args[2]), FilesAdded: orientationNumber(row.Args[3]), DocsAdded: orientationNumber(row.Args[4])})
	}
	sort.Slice(brief.Months, func(i, j int) bool { return brief.Months[i].Index < brief.Months[j].Index })
	for _, row := range eras {
		era := northstar.EraView{Index: orientationNumber(row.Args[0]), StartMonth: orientationNumber(row.Args[1]), EndMonth: orientationNumber(row.Args[2]), Kind: strings.TrimPrefix(types.ExtractString(row.Args[3]), "/")}
		for _, month := range brief.Months {
			if month.Index >= era.StartMonth && month.Index <= era.EndMonth {
				era.Months = append(era.Months, month)
			}
		}
		for _, birth := range births {
			month := orientationNumber(birth.Args[1])
			if month >= era.StartMonth && month <= era.EndMonth {
				era.Documents = append(era.Documents, types.ExtractString(birth.Args[0]))
			}
		}
		sort.Strings(era.Documents)
		brief.Eras = append(brief.Eras, era)
	}
	if len(eras) == 0 {
		brief.NoEraNote = "No eras derived; calendar-month measurements are not eras."
	}
	for _, pred := range []string{"origin_source", "vision_source"} {
		rows, err := eng.Query(pred)
		if err != nil {
			return brief, err
		}
		for _, row := range rows {
			p := types.ExtractString(row.Args[0])
			body, ok := snapshot.Bodies[p]
			if !ok {
				continue
			}
			src := northstar.BriefSource{Path: p, Body: body}
			if pred == "vision_source" {
				src.Weight = int(orientationNumber(row.Args[1]))
				src.Why = types.ExtractString(row.Args[2])
			} else {
				src.Why = types.ExtractString(row.Args[1])
			}
			src.Roles = append(src.Roles, roles[p]...)
			sort.Strings(src.Roles)
			if pred == "origin_source" {
				brief.Origins = append(brief.Origins, src)
			} else {
				brief.Vision = append(brief.Vision, src)
			}
		}
	}
	sort.Slice(brief.Origins, func(i, j int) bool { return brief.Origins[i].Path < brief.Origins[j].Path })
	sort.Slice(brief.Vision, func(i, j int) bool {
		if brief.Vision[i].Weight != brief.Vision[j].Weight {
			return brief.Vision[i].Weight > brief.Vision[j].Weight
		}
		return brief.Vision[i].Path < brief.Vision[j].Path
	})
	for _, pred := range []string{"doc_evolved_into", "doc_superseded"} {
		rows, err := eng.Query(pred)
		if err != nil {
			return brief, err
		}
		for _, row := range rows {
			link := northstar.EvolutionLink{Old: types.ExtractString(row.Args[0]), New: types.ExtractString(row.Args[1])}
			if pred == "doc_evolved_into" {
				brief.Evolved = append(brief.Evolved, link)
			} else {
				brief.Superseded = append(brief.Superseded, link)
			}
		}
	}
	return brief, nil
}

func orientationNumber(value any) int64 { n, _ := factNumber(value); return n }

// persistOrientationKnowledge feeds the strategic/* atoms the intelligence
// layer and query_strategic read: the vision of record as strategic
// knowledge, and each orientation read candidate as strategic/source/<path>.
// Both are replaced rather than appended, and an unchanged document is not
// embedded again. Without a vision nothing is invented in its place.
func (i *Initializer) persistOrientationKnowledge(ctx context.Context, _ ProjectProfile, result *InitResult) {
	if i.localDB == nil {
		result.Warnings = append(result.Warnings, "Orientation knowledge has no database")
		return
	}
	if i.orientedVision != nil {
		if _, err := i.PersistStrategicKnowledge(ctx, strategicKnowledgeFromVision(i.orientedVision), i.localDB); err != nil {
			result.Warnings = append(result.Warnings, err.Error())
		}
	}
	if i.orientation == nil || i.orientationSnapshot == nil {
		return
	}
	rows, err := i.orientation.Query("orient_read_candidate")
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		return
	}
	sources := map[string]string{}
	for _, row := range rows {
		p := types.ExtractString(row.Args[0])
		if body, ok := i.orientationSnapshot.Bodies[p]; ok {
			sources["strategic/source/"+p] = body
		}
	}
	if err := syncStrategicSources(ctx, i.localDB, sources); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}
}

// syncStrategicSources makes strategic/source/* hold exactly sources (concept
// to body): atoms for documents no longer read are deleted, changed ones are
// rewritten, and unchanged ones are left as they are.
func syncStrategicSources(ctx context.Context, db *store.LocalStore, sources map[string]string) error {
	existing, err := db.GetKnowledgeAtomsByPrefix("strategic/source/")
	if err != nil {
		return fmt.Errorf("read stored orientation sources: %w", err)
	}
	stored := map[string][]string{}
	for _, atom := range existing {
		stored[atom.Concept] = append(stored[atom.Concept], atom.Content)
	}
	pending := map[string]string{}
	for concept, body := range sources {
		pending[concept] = body
	}
	var stale []string
	for concept, contents := range stored {
		body, current := sources[concept]
		if current && len(contents) == 1 && contents[0] == body {
			delete(pending, concept)
			continue
		}
		stale = append(stale, concept)
	}
	if len(stale) > 0 {
		if _, err := db.DeleteKnowledgeAtomsByConcept(stale...); err != nil {
			return fmt.Errorf("retire stale orientation sources: %w", err)
		}
	}
	concepts := make([]string, 0, len(pending))
	for concept := range pending {
		concepts = append(concepts, concept)
	}
	sort.Strings(concepts)
	var errs []error
	for _, concept := range concepts {
		if err := db.StoreKnowledgeAtomWithEmbedding(ctx, concept, pending[concept], 1); err != nil {
			errs = append(errs, fmt.Errorf("orientation source %s: %w", strings.TrimPrefix(concept, "strategic/source/"), err))
		}
	}
	return errors.Join(errs...)
}
