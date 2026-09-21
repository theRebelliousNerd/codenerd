package system

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// structureProvider adapts world.StructureIndex to the structural query tools.
// It lives here because internal/world imports internal/tools/codedom, so the
// tools cannot name the index type themselves.
type structureProvider struct {
	index *world.StructureIndex
}

func indexLine(stats world.StructureStats) string {
	return fmt.Sprintf("%d files, %d declarations, %d call sites; %d reparsed for this answer (%s)",
		stats.Files, stats.Symbols, stats.Calls, stats.Reparsed, stats.RefreshTookS)
}

func toStructureSymbols(in []world.StructSymbol) []codedom.StructureSymbol {
	out := make([]codedom.StructureSymbol, 0, len(in))
	for _, s := range in {
		out = append(out, codedom.StructureSymbol{
			ID: s.ID, Kind: s.Kind, File: s.File, Signature: s.Signature, Doc: s.Doc,
			StartLine: s.StartLine, EndLine: s.EndLine, Exported: s.Exported,
		})
	}
	return out
}

func (p *structureProvider) FindSymbol(ctx context.Context, query, kind string) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.FindSymbol(ctx, query, kind)
	return toStructureSymbols(symbols), indexLine(stats), err
}

func (p *structureProvider) Outline(ctx context.Context, path string) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.Outline(ctx, path)
	return toStructureSymbols(symbols), indexLine(stats), err
}

func (p *structureProvider) Callers(ctx context.Context, query string) ([]codedom.StructureSymbol, []codedom.StructureCaller, string, error) {
	targets, sites, stats, err := p.index.Callers(ctx, query)
	callers := make([]codedom.StructureCaller, 0, len(sites))
	for _, s := range sites {
		callers = append(callers, codedom.StructureCaller{Caller: s.Caller, File: s.File, Line: s.Line, Match: s.Match})
	}
	return toStructureSymbols(targets), callers, indexLine(stats), err
}

func (p *structureProvider) Callees(ctx context.Context, query string) ([]codedom.StructureSymbol, []codedom.StructureCallee, string, error) {
	targets, calls, stats, err := p.index.Callees(ctx, query)
	callees := make([]codedom.StructureCallee, 0, len(calls))
	for _, c := range calls {
		callees = append(callees, codedom.StructureCallee{Call: c.Call, Line: c.Line, Candidates: c.Candidates})
	}
	return toStructureSymbols(targets), callees, indexLine(stats), err
}

func (p *structureProvider) Unreferenced(ctx context.Context, path string) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.Unreferenced(ctx, path)
	return toStructureSymbols(symbols), indexLine(stats), err
}

// wireStructureProvider registers the structure index for this Cortex. The
// index is empty until the first structural query, so boot pays nothing.
func wireStructureProvider(workspace string) {
	if workspace == "" {
		codedom.RegisterStructureProvider(nil)
		logging.Get(logging.CategoryBoot).Debug("structure provider not registered: no workspace")
		return
	}
	codedom.RegisterStructureProvider(&structureProvider{index: world.NewStructureIndex(workspace)})
	logging.Get(logging.CategoryBoot).Debug("structure provider registered for %s", workspace)
}
