package codemodel

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
)

// Change replaces the byte span [Start, End) of a file's source with Text.
type Change struct {
	Start, End int
	Text       string
}

// Outcome is a validated edit, ready to be written.
type Outcome struct {
	// File is the model of the new source; File.Source is what to write.
	File *File
	// Touched are the elements of the new file the edit created or changed,
	// in source order. The header is among them only when the edit targeted
	// it or changed its imports.
	Touched []*Element
	// Removed are the keys of targeted elements the new file no longer has.
	Removed []string
	Imports ImportReport
}

// ImportReport says what import derivation did.
type ImportReport struct {
	Added   []string
	Removed []string
	// Notes are qualifiers it could not resolve to one import path, and the
	// alternatives it passed over when it chose.
	Notes []string
}

// Changed reports whether the imports moved.
func (r ImportReport) Changed() bool { return len(r.Added)+len(r.Removed) > 0 }

// Resolver answers the workspace questions import derivation asks.
type Resolver interface {
	// ResolveQualifier returns the import path a package qualifier names
	// from this file's directory, and the candidates it chose among. path is
	// "" when it cannot choose.
	ResolveQualifier(qualifier string) (path string, candidates []string)
	// DeclaredInPackage reports whether name is declared at package level in
	// the file's own package, where X.Sel is a field or method, not an import.
	DeclaredInPackage(name string) bool
	// ModulePath is the module's import path, to group imports as gofmt'd
	// code in this module does: standard library first, everything else after.
	ModulePath() string
}

// Apply validates and applies a change to a file. targeted names the keys of
// the elements the change is meant to alter; every other element must come
// out byte-identical (a group realigned by gofmt may differ in whitespace
// only). The result must parse, unless the file did not parse before, in
// which case it must not parse worse: a broken file stays repairable one
// region at a time.
//
// For Go, every declaration the change touched is gofmt'd on its own, and
// imports are derived: a qualifier the edit newly uses is imported when res
// resolves it, and an import the edit made unused is dropped. res may be nil,
// which leaves imports alone.
func Apply(old *File, ch Change, targeted []string, res Resolver) (*Outcome, error) {
	if ch.Start < 0 || ch.End > len(old.Source) || ch.Start > ch.End {
		return nil, fmt.Errorf("change span %d-%d is outside the file (%d bytes)", ch.Start, ch.End, len(old.Source))
	}
	src := old.Source[:ch.Start] + Normalize(ch.Text) + old.Source[ch.End:]
	target := make(map[string]bool, len(targeted))
	for _, k := range targeted {
		target[k] = true
	}

	parse := func(s string) *File {
		nf, _ := Parse(old.Path, s)
		return nf
	}
	// The lines the change wrote, in the new source: a refusal shows only
	// these, never a neighbouring element's text.
	written := Normalize(ch.Text)
	span := lineSpan{lo: strings.Count(src[:ch.Start], "\n") + 1}
	if written != "" {
		span.hi = span.lo + strings.Count(strings.TrimSuffix(written, "\n"), "\n")
	}
	nf := parse(src)
	if err := parseVerdict(old, nf, span); err != nil {
		return nil, err
	}

	var report ImportReport
	if old.Language == LangGo && nf.Parsed {
		lo, hi := ch.Start, ch.Start+len(Normalize(ch.Text))
		formatted, err := formatUnits(nf, lo, hi)
		if err != nil {
			return nil, err
		}
		nf = parse(formatted)
		if err := parseVerdict(old, nf, span); err != nil {
			return nil, err
		}
		if res != nil {
			derived, rep, err := DeriveImports(old, nf, res)
			if err != nil {
				return nil, err
			}
			report = rep
			if rep.Changed() {
				nf = parse(derived)
				if err := parseVerdict(old, nf, span); err != nil {
					return nil, fmt.Errorf("deriving imports broke the file: %w", err)
				}
				target[HeaderKey] = true
			}
		}
	}

	out := &Outcome{File: nf, Imports: report}
	oldByKey := make(map[string]*Element, len(old.Elements))
	for i := range old.Elements {
		oldByKey[old.Elements[i].Key] = &old.Elements[i]
	}
	newByKey := make(map[string]*Element, len(nf.Elements))
	for i := range nf.Elements {
		newByKey[nf.Elements[i].Key] = &nf.Elements[i]
	}

	var violations []string
	for i := range old.Elements {
		oe := &old.Elements[i]
		if target[oe.Key] {
			if newByKey[oe.Key] == nil {
				out.Removed = append(out.Removed, oe.Key)
			}
			continue
		}
		if oe.Kind == KindSyntaxError {
			// A broken region has no identity to preserve: repairing the
			// declaration next to it can rightly reshape it.
			continue
		}
		ne := newByKey[oe.Key]
		switch {
		case ne == nil:
			violations = append(violations, fmt.Sprintf("%s (lines %d-%d) would disappear", oe.Key, oe.StartLine, oe.EndLine))
		case ne.Revision != oe.Revision && squash(nf.Text(ne)) != squash(old.Text(oe)):
			violations = append(violations, fmt.Sprintf("%s (lines %d-%d) would change", oe.Key, oe.StartLine, oe.EndLine))
		}
	}
	if len(violations) > 0 {
		return nil, fmt.Errorf("refusing the edit: it changes elements it does not target -- %s. "+
			"The replacement text probably opens or closes a declaration it should not; send only the targeted element's own text",
			strings.Join(violations, "; "))
	}

	for i := range nf.Elements {
		ne := &nf.Elements[i]
		oe := oldByKey[ne.Key]
		if oe != nil && !target[ne.Key] {
			continue
		}
		if oe != nil && oe.Revision == ne.Revision && ne.Kind != KindHeader {
			continue
		}
		if ne.Kind == KindHeader && !target[HeaderKey] {
			continue
		}
		out.Touched = append(out.Touched, ne)
	}
	return out, nil
}

// lineSpan is a 1-based inclusive range of lines. hi < lo is empty; the zero
// value is unbounded.
type lineSpan struct{ lo, hi int }

// parseVerdict refuses a result that does not parse, or for a file that was
// already broken, one that parses worse. The refusal quotes only lines inside
// span: an element verb that handed back its neighbour's source to explain a
// parse error was a raw read of that neighbour (the R8 ratchet caught
// replace_element doing it).
func parseVerdict(old, nf *File, span lineSpan) error {
	if nf.Parsed {
		return nil
	}
	if old.Parsed || len(nf.Errors) > len(old.Errors) {
		return fmt.Errorf("the result does not parse, so nothing was written: %s", describeErrorsWithin(nf, span))
	}
	return nil
}

// describeErrors names a file's parse errors and shows the lines around the
// first one.
func describeErrors(f *File) string {
	return describeErrorsWithin(f, lineSpan{})
}

// describeErrorsWithin is describeErrors with the excerpt clipped to span. An
// excerpt that would fall wholly outside it is left out: the error's line and
// column still say where.
func describeErrorsWithin(f *File, span lineSpan) string {
	if len(f.Errors) == 0 {
		if f.Err != nil {
			return f.Err.Error()
		}
		return "unknown parse error"
	}
	var parts []string
	for i, e := range f.Errors {
		if i == 3 {
			parts = append(parts, fmt.Sprintf("(%d more)", len(f.Errors)-3))
			break
		}
		parts = append(parts, fmt.Sprintf("line %d:%d: %s", e.Line, e.Column, e.Msg))
	}
	first := f.Errors[0].Line
	lo, hi := max(1, first-3), min(f.LineCount(), first+3)
	if span != (lineSpan{}) {
		lo, hi = max(lo, span.lo), min(hi, span.hi)
	}
	if hi < lo {
		return strings.Join(parts, "; ")
	}
	return strings.Join(parts, "; ") + "\n" + f.NumberedLines(lo, hi)
}

// squash collapses whitespace, so a group gofmt realigned compares equal to
// what it was.
func squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// formatUnits gofmts, each on its own, the top-level declarations (and the
// header) that overlap [lo, hi]. Declarations outside the change are left
// byte-for-byte as they were, even in a file that was never gofmt'd.
func formatUnits(nf *File, lo, hi int) (string, error) {
	type unit struct {
		start, end int
		header     bool
	}
	seen := make(map[[2]int]bool)
	var units []unit
	for _, e := range nf.Elements {
		us, ue := e.UnitStart, e.UnitEnd
		if e.Kind == KindHeader {
			us, ue = e.Start, e.End
		}
		if us > hi || lo > ue {
			continue
		}
		k := [2]int{us, ue}
		if seen[k] || ue <= us {
			continue
		}
		seen[k] = true
		units = append(units, unit{start: us, end: ue, header: e.Kind == KindHeader})
	}
	sort.Slice(units, func(i, j int) bool { return units[i].start > units[j].start })
	src := nf.Source
	for _, u := range units {
		formatted, err := FormatUnit(src[u.start:u.end], u.header)
		if err != nil {
			return "", err
		}
		src = src[:u.start] + formatted + src[u.end:]
	}
	return src, nil
}

// FormatUnit gofmts one top-level declaration, or a file header, in
// isolation.
func FormatUnit(text string, header bool) (string, error) {
	if header {
		out, err := format.Source([]byte(text))
		if err != nil {
			return "", fmt.Errorf("gofmt of the file header failed: %w", err)
		}
		return strings.TrimSuffix(string(out), "\n"), nil
	}
	const prefix = "package p\n\n"
	out, err := format.Source([]byte(prefix + text))
	if err != nil {
		return "", fmt.Errorf("gofmt of the edited declaration failed: %w", err)
	}
	s, ok := strings.CutPrefix(string(out), prefix)
	if !ok {
		// gofmt moved something ahead of the package clause; keep the
		// declaration as written rather than guess at the cut.
		return text, nil
	}
	return strings.TrimSuffix(s, "\n"), nil
}
