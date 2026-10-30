package session

import (
	"bufio"
	"fmt"
	"go/build"
	"go/build/constraint"
	"os"
	"slices"
	"sort"
	"strings"

	"codenerd/internal/testfacts"
)

// A written tag-gated test is verified by running it under a build
// configuration that includes its file. Enabling every positive tag the
// constraint names is not one: "(a && !b) || (!a && b)" excludes the file
// with both set, the targeted `go test` then matches no test and exits 0, and
// the gate read that as the test passing. The configuration is now a
// satisfying assignment of the file's constraint, and the run must show the
// named tests passing.

// satisfyingTags is the smallest set of the file's feature tags (its
// non-toolchain tag names) under which its //go:build constraint holds on
// this toolchain, smallest first, then lexically. It reads the constraint's
// disjunctive form: each conjunction whose toolchain literals hold here and
// whose feature literals do not contradict names the feature tags to set.
// ok is false when no conjunction can hold; why says so. A file with no
// constraint, or one the default configuration satisfies, returns no tags.
func satisfyingTags(path string) (tags []string, ok bool, why string) {
	expr, err := fileConstraint(path)
	if err != nil {
		return nil, false, err.Error()
	}
	if expr == nil {
		return nil, true, ""
	}
	var best []string
	found := false
	for _, term := range disjunctiveForm(expr, false) {
		set, consistent := termFeatureTags(term)
		if !consistent {
			continue
		}
		if !found || len(set) < len(best) || (len(set) == len(best) && strings.Join(set, ",") < strings.Join(best, ",")) {
			best, found = set, true
		}
	}
	if !found {
		return nil, false, "no set of its feature tags satisfies its build constraint on this toolchain"
	}
	enabled := map[string]bool{}
	for _, t := range best {
		enabled[t] = true
	}
	if !expr.Eval(func(tag string) bool { return enabled[tag] || toolchainTag(tag) }) {
		return nil, false, "the configuration derived for its build constraint does not satisfy it"
	}
	return best, true, ""
}

// tagLiteral is one tag, required set (pos) or unset.
type tagLiteral struct {
	tag string
	pos bool
}

// disjunctiveForm expands expr (negated when neg) into OR-of-ANDs.
func disjunctiveForm(expr constraint.Expr, neg bool) [][]tagLiteral {
	switch x := expr.(type) {
	case *constraint.TagExpr:
		return [][]tagLiteral{{{tag: x.Tag, pos: !neg}}}
	case *constraint.NotExpr:
		return disjunctiveForm(x.X, !neg)
	case *constraint.AndExpr:
		if neg {
			return append(disjunctiveForm(x.X, true), disjunctiveForm(x.Y, true)...)
		}
		return crossTerms(disjunctiveForm(x.X, false), disjunctiveForm(x.Y, false))
	case *constraint.OrExpr:
		if neg {
			return crossTerms(disjunctiveForm(x.X, true), disjunctiveForm(x.Y, true))
		}
		return append(disjunctiveForm(x.X, false), disjunctiveForm(x.Y, false)...)
	}
	return nil
}

func crossTerms(left, right [][]tagLiteral) [][]tagLiteral {
	out := make([][]tagLiteral, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			term := append(append([]tagLiteral(nil), l...), r...)
			out = append(out, term)
		}
	}
	return out
}

// termFeatureTags is the feature tags a conjunction requires set, and
// whether it can hold: its toolchain literals agree with this toolchain and
// no feature tag is both required and forbidden.
func termFeatureTags(term []tagLiteral) ([]string, bool) {
	want := map[string]bool{}
	for _, lit := range term {
		if isTagGatedToolchainTag(lit.tag) {
			if lit.pos != toolchainTag(lit.tag) {
				return nil, false
			}
			continue
		}
		if prior, seen := want[lit.tag]; seen && prior != lit.pos {
			return nil, false
		}
		want[lit.tag] = lit.pos
	}
	var set []string
	for tag, pos := range want {
		if pos {
			set = append(set, tag)
		}
	}
	sort.Strings(set)
	return set, true
}

// fileConstraint is the file's //go:build expression, nil when it has none.
func fileConstraint(path string) (constraint.Expr, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			break
		}
		if constraint.IsGoBuild(line) {
			expr, err := constraint.Parse(line)
			if err != nil {
				return nil, fmt.Errorf("its build constraint does not parse: %w", err)
			}
			return expr, nil
		}
	}
	return nil, scanner.Err()
}

// unixGOOS are the GOOS values the "unix" build tag matches.
var unixGOOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
	"hurd": true, "illumos": true, "ios": true, "linux": true, "netbsd": true,
	"openbsd": true, "solaris": true,
}

// toolchainTag reports whether this toolchain's default configuration sets
// tag: its GOOS, GOARCH, unix, cgo, gc, release tags, and the configured
// build and tool tags.
func toolchainTag(tag string) bool {
	ctx := build.Default
	switch {
	case tag == ctx.GOOS, tag == ctx.GOARCH, tag == ctx.Compiler:
		return true
	case tag == "unix":
		return unixGOOS[ctx.GOOS]
	case tag == "cgo":
		return ctx.CgoEnabled
	}
	return slices.Contains(ctx.ReleaseTags, tag) || slices.Contains(ctx.BuildTags, tag) || slices.Contains(ctx.ToolTags, tag)
}

// unranTests are the requested test names the run's parsed result shows no
// pass for: a targeted run that matched nothing exits 0 and proves nothing.
func unranTests(res *testfacts.Result, names []string) []string {
	passed := map[string]bool{}
	if res != nil {
		for _, p := range res.Packages {
			for _, t := range p.Tests {
				if t.Status == testfacts.StatusPass {
					passed[t.Name] = true
				}
			}
		}
	}
	var out []string
	for _, n := range names {
		if !passed[n] {
			out = append(out, n)
		}
	}
	return out
}
