package world

import (
	"context"
	"strings"

	"codenerd/internal/specs"
	"codenerd/internal/world/codemodel"
)

// SpecSnapshot is the structure index as the spec pairing layer reads it: every
// element in the specs package's own row, every file the index holds, and the
// measurement of each ref in measure.
//
// A file-level citation ("see internal/core/foo.go") needs an address, and the
// Go index lists no header element, so every file contributes one: its ref is
// "path:header", the spelling the element tools use for the top of a file.
//
// Every element carries its content fingerprint (codemodel.Fingerprint of its
// span); a file's header carries the whole file's, so a citation of the file
// follows any change in it.
//
// measure names the elements a requirement demands things of. The index
// measures only those: a caller search over every test file is not free.
//
//   - HasTest: some call site in a test file reaches the element (the same
//     matching callers_of reports), so a test that exercises the element is
//     the evidence, not a name convention.
//   - Calls: the refs of the elements it calls directly. A call whose name the
//     index cannot tie to exactly one element is not evidence that Other is
//     called, so it is left out.
func (s *StructureIndex) SpecSnapshot(ctx context.Context, measure []string) ([]specs.Element, []string, map[string]specs.Measurement, StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats, err := s.refreshLocked(ctx)
	if err != nil {
		return nil, nil, nil, stats, err
	}
	var elems []specs.Element
	files := make([]string, 0, len(s.files))
	for canonical, f := range s.files {
		files = append(files, canonical)
		headered := false
		for i := range f.symbols {
			sym := &f.symbols[i]
			if sym.Kind == string(codemodel.KindSyntaxError) {
				continue
			}
			if sym.Kind == string(codemodel.KindHeader) {
				headered = true
			}
			fingerprint := sym.Fingerprint
			if sym.Kind == string(codemodel.KindHeader) {
				fingerprint = f.contentFP
			}
			elems = append(elems, specs.Element{
				Ref: sym.Ref, Key: sym.Key, ID: sym.ID, Name: sym.name, Receiver: sym.receiver,
				Kind: sym.Kind, File: canonical, Dir: f.dir, Exported: sym.Exported,
				HasDoc: sym.Doc != "", Test: isTestFile(canonical), Fingerprint: fingerprint,
			})
		}
		if !headered {
			elems = append(elems, specs.Element{
				Ref: canonical + ":" + codemodel.HeaderKey, Key: codemodel.HeaderKey, Name: codemodel.HeaderKey,
				Kind: string(codemodel.KindHeader), File: canonical, Dir: f.dir, Exported: true, Test: isTestFile(canonical),
				Fingerprint: f.contentFP,
			})
		}
	}
	measured := make(map[string]specs.Measurement, len(measure))
	for _, ref := range measure {
		targets := s.resolveAnyLocked(strings.TrimSpace(ref))
		if len(targets) != 1 || targets[0].Ref != ref {
			continue
		}
		var m specs.Measurement
		for _, c := range s.callersOfLocked(targets) {
			if isTestFile(c.File) {
				m.HasTest = true
				break
			}
		}
		seen := map[string]bool{}
		for _, callee := range s.calleesOfLocked(targets) {
			if len(callee.Candidates) != 1 {
				continue
			}
			candidate, _, _ := strings.Cut(callee.Candidates[0], " @ ")
			if !seen[candidate] {
				seen[candidate] = true
				m.Calls = append(m.Calls, candidate)
			}
		}
		measured[ref] = m
	}
	return elems, files, measured, stats, nil
}
