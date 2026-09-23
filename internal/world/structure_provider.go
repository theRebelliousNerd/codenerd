package world

import (
	"context"

	"codenerd/internal/tools/codedom"
	"codenerd/internal/world/codemodel"
)

// structureProvider adapts the structure index to the CodeDOM tools. It lives
// here, beside the index, because the world already imports the tools'
// package; the tools reach it only through codedom.StructureProvider.
type structureProvider struct {
	index *StructureIndex
}

// Provider returns the index as the CodeDOM tools' structure provider.
func (s *StructureIndex) Provider() codedom.StructureProvider {
	return &structureProvider{index: s}
}

var _ codedom.StructureProvider = (*structureProvider)(nil)

func toSymbol(s StructSymbol) codedom.StructureSymbol {
	return codedom.StructureSymbol{
		Ref: s.Ref, Key: s.Key, ID: s.ID, Kind: s.Kind, File: s.File,
		Signature: s.Signature, Doc: s.Doc, Revision: s.Revision,
		StartLine: s.StartLine, EndLine: s.EndLine, Exported: s.Exported,
	}
}

func toSymbols(in []StructSymbol) []codedom.StructureSymbol {
	out := make([]codedom.StructureSymbol, 0, len(in))
	for _, s := range in {
		out = append(out, toSymbol(s))
	}
	return out
}

func (p *structureProvider) FindSymbol(ctx context.Context, q codedom.SymbolQuery) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.FindSymbol(ctx, SymbolFilter{Names: q.Names, Pattern: q.Pattern, Kind: q.Kind, Path: q.Path})
	return toSymbols(symbols), stats.Line(), err
}

func (p *structureProvider) Outline(ctx context.Context, path string) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.Outline(ctx, path)
	return toSymbols(symbols), stats.Line(), err
}

func (p *structureProvider) Callers(ctx context.Context, query string) ([]codedom.StructureSymbol, []codedom.StructureCaller, string, error) {
	targets, sites, stats, err := p.index.Callers(ctx, query)
	callers := make([]codedom.StructureCaller, 0, len(sites))
	for _, s := range sites {
		callers = append(callers, codedom.StructureCaller{Caller: s.Caller, File: s.File, Line: s.Line, Match: s.Match})
	}
	return toSymbols(targets), callers, stats.Line(), err
}

func (p *structureProvider) Callees(ctx context.Context, query string) ([]codedom.StructureSymbol, []codedom.StructureCallee, string, error) {
	targets, calls, stats, err := p.index.Callees(ctx, query)
	callees := make([]codedom.StructureCallee, 0, len(calls))
	for _, c := range calls {
		callees = append(callees, codedom.StructureCallee{Call: c.Call, Line: c.Line, Candidates: c.Candidates})
	}
	return toSymbols(targets), callees, stats.Line(), err
}

func (p *structureProvider) Unreferenced(ctx context.Context, path string) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.Unreferenced(ctx, path)
	return toSymbols(symbols), stats.Line(), err
}

func (p *structureProvider) Resolve(ctx context.Context, ref string) ([]codedom.StructureSymbol, string, error) {
	symbols, stats, err := p.index.Resolve(ctx, ref)
	return toSymbols(symbols), stats.Line(), err
}

func (p *structureProvider) Importers(ctx context.Context, pkg string) (string, []codedom.StructureImporter, string, error) {
	path, rows, stats, err := p.index.Importers(ctx, pkg)
	out := make([]codedom.StructureImporter, 0, len(rows))
	for _, r := range rows {
		out = append(out, codedom.StructureImporter{File: r.File, Name: r.Name, Line: r.Line})
	}
	return path, out, stats.Line(), err
}

func (p *structureProvider) FindText(ctx context.Context, text, in, path string) ([]codedom.StructureTextHit, string, error) {
	hits, stats, err := p.index.FindText(ctx, text, in, path)
	out := make([]codedom.StructureTextHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, codedom.StructureTextHit{File: h.File, Line: h.Line, In: h.In, Text: h.Text, Ref: h.Ref})
	}
	return out, stats.Line(), err
}

func (p *structureProvider) Uses(ctx context.Context, ref string) ([]codedom.StructureSymbol, []codedom.StructureUse, string, error) {
	target, uses, stats, err := p.index.Uses(ctx, ref)
	if err != nil {
		return nil, nil, stats.Line(), err
	}
	out := make([]codedom.StructureUse, 0, len(uses))
	for _, u := range uses {
		out = append(out, codedom.StructureUse{
			File: u.File, Line: u.Line, Column: u.Column, Start: u.Start, End: u.End,
			Qualifier: u.Qualifier, Ref: u.Ref, Match: u.Match,
		})
	}
	return []codedom.StructureSymbol{toSymbol(*target)}, out, stats.Line(), nil
}

func (p *structureProvider) FileStatus(ctx context.Context, path string) (codedom.FileStatus, error) {
	st, err := p.index.FileStatus(ctx, path)
	return codedom.FileStatus{Known: st.Known, Parsed: st.Parsed, Errors: st.Errors, LastGood: toSymbols(st.LastGood)}, err
}

func (p *structureProvider) CanonicalRefs(ctx context.Context, path string) (map[string]string, error) {
	return p.index.CanonicalRefs(ctx, path)
}

func (p *structureProvider) ImportResolver(ctx context.Context, path string) (codemodel.Resolver, error) {
	return p.index.ImportResolver(ctx, path)
}

func (p *structureProvider) PredicateOutline(ctx context.Context, pred string) ([]codedom.PredicateRow, string, error) {
	rows, stats, err := p.index.PredicateOutline(ctx, pred)
	out := make([]codedom.PredicateRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, codedom.PredicateRow{Role: r.Role, Symbol: toSymbol(r.Symbol)})
	}
	return out, stats.Line(), err
}
