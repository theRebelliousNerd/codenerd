package world

import (
	"go/scanner"
	"go/token"
	"os"
	"strings"
)

// A package-level declaration change used to re-parse every sibling in the
// package: one new helper in internal/world remapped 161 files on a live scan
// (2026-11-07). The cartographer reads a sibling's calls against the package
// symbols only through identifiers the sibling spells: T(x) is a call or a
// conversion depending on what T is, x.M() resolves through the method name
// M, an alias A changes what A names. A sibling that never mentions a name
// whose declaration changed parses to the same rows, so only the siblings
// that mention one are remapped. When the changed names cannot be told (no
// name differs although the sets do), every sibling is remapped as before.

// changedNames is every identifier whose declaration differs between two
// snapshots: a type, function or alias added, removed or redefined, a
// method added or removed (both its name and its receiver's), and an alias
// retargeted.
func (s pkgSymbols) changedNames(o pkgSymbols) map[string]struct{} {
	names := map[string]struct{}{}
	diffBool := func(a, b map[string]bool) {
		for k, v := range a {
			if w, ok := b[k]; !ok || w != v {
				names[k] = struct{}{}
			}
		}
		for k := range b {
			if _, ok := a[k]; !ok {
				names[k] = struct{}{}
			}
		}
	}
	diffBool(s.types, o.types)
	diffBool(s.funcs, o.funcs)
	diffBool(s.aliasBad, o.aliasBad)
	for k, v := range s.aliases {
		if w, ok := o.aliases[k]; !ok || w != v {
			names[k] = struct{}{}
		}
	}
	for k := range o.aliases {
		if _, ok := s.aliases[k]; !ok {
			names[k] = struct{}{}
		}
	}
	diffMethods := func(a, b map[string]map[string]bool) {
		for recv, ms := range a {
			for m, v := range ms {
				if w, ok := b[recv][m]; !ok || w != v {
					names[m] = struct{}{}
					names[recv] = struct{}{}
				}
			}
		}
	}
	diffMethods(s.methods, o.methods)
	diffMethods(o.methods, s.methods)
	return names
}

// namesFromKeyDiff is changedNames for two declaration-key sets
// (func:<pkg>.<Name>, method:<pkg>.<Recv>.<Name>, type:<pkg>.<Name>), the
// shape stored code_defines rows give when no in-memory snapshot exists.
func namesFromKeyDiff(a, b map[string]struct{}) map[string]struct{} {
	names := map[string]struct{}{}
	add := func(key string) {
		_, rest, ok := strings.Cut(key, ":")
		if !ok {
			return
		}
		parts := strings.Split(rest, ".")
		// parts[0] is the package; the rest are the receiver and the name.
		for _, p := range parts[1:] {
			if p != "" {
				names[p] = struct{}{}
			}
		}
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			add(k)
		}
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			add(k)
		}
	}
	return names
}

// siblingsMentioning keeps the candidates whose source spells at least one
// of names as a Go identifier. An empty name set, or a file that cannot be
// read, keeps the candidate: narrowing is only allowed when it is proven.
func siblingsMentioning(candidates []string, names map[string]struct{}) []string {
	if len(names) == 0 {
		return candidates
	}
	out := make([]string, 0, len(candidates))
	for _, p := range candidates {
		if mentionsIdentifier(p, names) {
			out = append(out, p)
		}
	}
	return out
}

// mentionsIdentifier scans path's tokens for an identifier in names. Text in
// comments and string literals is not an identifier and does not count. A
// read failure counts as a mention.
func mentionsIdentifier(path string, names map[string]struct{}) bool {
	src, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	fset := token.NewFileSet()
	file := fset.AddFile(path, -1, len(src))
	var sc scanner.Scanner
	// A syntax error does not stop the scan; the scanner keeps tokenizing,
	// and an unparsable sibling still names what it names.
	sc.Init(file, src, nil, 0)
	for {
		_, tok, lit := sc.Scan()
		if tok == token.EOF {
			return false
		}
		if tok == token.IDENT {
			if _, ok := names[lit]; ok {
				return true
			}
		}
	}
}
