package codedom

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"codenerd/internal/world/codemodel"
)

// =============================================================================
// USE EVIDENCE FOR DESTRUCTIVE EDITS
// =============================================================================
// delete_element and repoint act on a symbol's use set. Three things make
// that set safe to act on (Docs/audits/structure-coverage-and-refactor-
// preconditions.md, S2 and S3):
//
//   - It was observed completely. A use file the index could not read or
//     parse is an unresolved obligation, never an empty list.
//   - Each use's offsets are applied only to the bytes they were measured
//     on: the file's digest and the span's text are checked before a rewrite.
//   - The set is still the set at commit. A file the edit does not write can
//     gain a use between discovery and the write; the set is re-observed
//     under commitMu and must be unchanged.

// sourceDigest is the content identity the structure index records for a
// use: the hex SHA-256 of the file's raw bytes.
func sourceDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// unresolvedUses refuses an edit whose use discovery was incomplete, naming
// what could not be observed and what to do about it.
func unresolvedUses(verb, ref string, found int, obs Observation) error {
	return fmt.Errorf("refusing to %s %s: use discovery is incomplete, so no use outside it is established (%d found in what could be observed). %s. Restore read access or repair those files, then call again",
		verb, ref, found, obs.DeficitSummary())
}

// verifyUseSource checks that every use in one file was measured on the
// bytes the plan loaded: the same content digest, and the expected text at
// each span. Bounds and a clean parse do not show this; a same-length
// identifier substituted at a use's offset passes both.
func verifyUseSource(rel string, data []byte, src string, uses []StructureUse) error {
	digest := sourceDigest(data)
	for _, u := range uses {
		if u.Digest == "" {
			return fmt.Errorf("%s: the index recorded no revision for the use at line %d; nothing was written, call again", rel, u.Line)
		}
		if u.Digest != digest {
			return fmt.Errorf("%s changed after its uses were found (revision %.12s, now %.12s); nothing was written, call again to plan against the current source", rel, u.Digest, digest)
		}
		if u.Start < 0 || u.End > len(src) || u.Start > u.End || src[u.Start:u.End] != u.Text {
			return fmt.Errorf("%s: the use at line %d no longer reads %q; nothing was written, call again", rel, u.Line, u.Text)
		}
	}
	return nil
}

// usesUnchanged is the read dependency of an edit planned from a use set:
// re-observed at commit, the set must be complete and identical, file by
// file and span by span.
func usesUnchanged(ref string, want []StructureUse, discover func(context.Context) ([]StructureUse, Observation, error)) readDependency {
	want = sortedUses(want)
	return func(ctx context.Context) error {
		got, obs, err := discover(ctx)
		if err != nil {
			return fmt.Errorf("optimistic conflict: the uses of %s could not be observed again before writing: %w", ref, err)
		}
		if !obs.Complete() {
			return fmt.Errorf("optimistic conflict: %w", unresolvedUses("write an edit planned from the uses of", ref, len(got), obs))
		}
		if !sameUses(want, sortedUses(got)) {
			return fmt.Errorf("optimistic conflict: the uses of %s changed since they were found (%d then, %d now: %s); nothing was written, call again to plan against the current source",
				ref, len(want), len(got), firstUseDifference(want, sortedUses(got)))
		}
		return nil
	}
}

// declarationStands is the read dependency on the declaration a repoint
// points uses at: still one element, at the same ref, in the same file.
func declarationStands(provider StructureProvider, want StructureSymbol) readDependency {
	return func(ctx context.Context) error {
		syms, _, err := provider.Resolve(ctx, want.Ref)
		if err != nil {
			return fmt.Errorf("optimistic conflict: %s could not be resolved again before writing: %w", want.Ref, err)
		}
		if len(syms) != 1 || syms[0].Ref != want.Ref || syms[0].File != want.File || syms[0].Kind != want.Kind {
			return fmt.Errorf("optimistic conflict: %s, the declaration the uses would point at, changed or moved since the plan; nothing was written, call again", want.Ref)
		}
		return nil
	}
}

// scriptSitesUnchanged is the read dependency of a script rename: the sites
// scope resolution finds at commit are the sites the plan rewrote.
func scriptSitesUnchanged(from StructureSymbol, want []codemodel.RenameSite, discover func(context.Context) ([]codemodel.RenameSite, error)) readDependency {
	key := func(s codemodel.RenameSite) string { return fmt.Sprintf("%s:%d-%d", slashPath(s.Path), s.Start, s.End) }
	return func(ctx context.Context) error {
		got, err := discover(ctx)
		if err != nil {
			return fmt.Errorf("optimistic conflict: the references to %s could not be observed again before writing: %w", from.Ref, err)
		}
		a := make([]string, 0, len(want))
		for _, s := range want {
			a = append(a, key(s))
		}
		b := make([]string, 0, len(got))
		for _, s := range got {
			b = append(b, key(s))
		}
		sort.Strings(a)
		sort.Strings(b)
		if fmt.Sprint(a) != fmt.Sprint(b) {
			return fmt.Errorf("optimistic conflict: the references to %s changed since they were found (%d then, %d now); nothing was written, call again", from.Ref, len(a), len(b))
		}
		return nil
	}
}

func sortedUses(uses []StructureUse) []StructureUse {
	out := append([]StructureUse(nil), uses...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		if out[i].Start != out[j].Start {
			return out[i].Start < out[j].Start
		}
		return out[i].Line < out[j].Line
	})
	return out
}

// useIdentity is what makes two observations of a use the same use.
func useIdentity(u StructureUse) string {
	return fmt.Sprintf("%s:%d:%d-%d@%s", u.File, u.Line, u.Start, u.End, u.Digest)
}

func sameUses(a, b []StructureUse) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if useIdentity(a[i]) != useIdentity(b[i]) {
			return false
		}
	}
	return true
}

func firstUseDifference(want, got []StructureUse) string {
	for i := range max(len(want), len(got)) {
		switch {
		case i >= len(want):
			return fmt.Sprintf("new use at %s:%d", got[i].File, got[i].Line)
		case i >= len(got):
			return fmt.Sprintf("use at %s:%d is gone", want[i].File, want[i].Line)
		case useIdentity(want[i]) != useIdentity(got[i]):
			return fmt.Sprintf("%s:%d differs", got[i].File, got[i].Line)
		}
	}
	return "no difference"
}
