package init

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/northstar"
	"codenerd/internal/orient"
	"codenerd/internal/types"
)

// recordingCompleter keeps derivation calls on init's provider metrics.
type recordingCompleter struct {
	owner *Initializer
	next  northstar.Completer
}

func (r recordingCompleter) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return r.owner.withJITPrompt(ctx, "orientation", user, nil, func(ctx context.Context, prompt string) (string, error) {
		return r.next.CompleteWithSystem(ctx, prompt, user)
	}, system)
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
	client := recordingCompleter{owner: i, next: i.config.LLMClient}
	if len(documents) > 0 && i.config.LLMClient != nil && budgetErr == nil {
		run.Class, err = northstar.ClassifyDocuments(ctx, client, northstar.CompilePhasePrompt, budget, documents)
		if err != nil {
			run.Notes = append(run.Notes, "Role transduction failed: "+err.Error())
		}
		if run.Class != nil {
			claims := run.Class.Facts()
			if err := eng.Assert(claims); err != nil {
				return err
			}
			snapshot.Facts = append(snapshot.Facts, claims...)
			if err := eng.Evaluate(ctx); err != nil {
				return err
			}
		}
	} else if len(documents) > 0 {
		run.Notes = append(run.Notes, "Role transduction unavailable; documents remain unclassified.")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	run.Brief, err = orientationBrief(eng, snapshot, run.Class)
	if err != nil {
		return err
	}
	if len(run.Brief.Vision) > 0 && i.config.LLMClient != nil && budgetErr == nil {
		run.Draft, err = northstar.DraftVision(ctx, client, northstar.CompilePhasePrompt, budget, run.Brief)
		if err != nil {
			run.Notes = append(run.Notes, "North star drafting failed: "+err.Error())
		}
		if run.Draft != nil {
			store, err := northstar.NewStore(nerdDir)
			if err != nil {
				return err
			}
			run.Install, err = northstar.InstallDerivedVision(store, nerdDir, run.Draft)
			if err != nil {
				_ = store.Close()
				return err
			}
			result.VisionDerived = run.Install.Installed
			i.orientedVision, err = store.LoadVision()
			_ = store.Close()
			if err != nil {
				return err
			}
		}
	} else {
		run.Notes = append(run.Notes, "No vision_source rows were available for drafting.")
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

func orientationBrief(eng *orient.Engine, snapshot *orient.Snapshot, class *northstar.Classification) (northstar.OrientationBrief, error) {
	brief := northstar.OrientationBrief{}
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
			if class != nil {
				for _, claim := range class.Claims {
					if claim.Path == p {
						src.Roles = append(src.Roles, claim.Role)
					}
				}
			}
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

func (i *Initializer) persistOrientationKnowledge(ctx context.Context, profile ProjectProfile, result *InitResult) {
	if i.localDB == nil {
		result.Warnings = append(result.Warnings, "Orientation knowledge has no database")
		return
	}
	knowledge := strategicKnowledgeFromVision(i.orientedVision, profile)
	if _, err := i.PersistStrategicKnowledge(ctx, knowledge, i.localDB); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}
	if i.orientation == nil || i.orientationSnapshot == nil {
		return
	}
	rows, err := i.orientation.Query("orient_read_candidate")
	if err != nil {
		result.Warnings = append(result.Warnings, err.Error())
		return
	}
	seen := map[string]bool{}
	for _, row := range rows {
		p := types.ExtractString(row.Args[0])
		if seen[p] {
			continue
		}
		seen[p] = true
		body, ok := i.orientationSnapshot.Bodies[p]
		if !ok {
			continue
		}
		if err := i.localDB.StoreKnowledgeAtomWithEmbedding(ctx, "strategic/source/"+p, body, 1); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("orientation source %s: %v", p, err))
		}
	}
}
