package tools

import (
	"fmt"
	"go/build/constraint"
	"go/version"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"codenerd/internal/build"
)

// RejectAddedBuildExclusion refuses a .go write that adds a build constraint
// excluding the file from the current build. Dogfood run 5 (2026-09-29): a
// repair loop hid another agent's compile failure by adding `//go:build
// ignore` to files outside its write set, making the build pass by deleting
// code from it. Hiding a file from the build is never a fix for a failing
// one, so like RejectUnparseableGo this is a mechanical refusal at the write
// path, not a policy verdict: the bytes before and after are in hand here,
// and Mangle cannot parse constraints (Go parses, Mangle decides).
// constitution.mg should not carry it. A string match on the payload fires
// on a mention and cannot spare a file that already had the constraint; the
// before and after bytes are what make that distinction, and they are not
// facts the kernel holds.
//
// Only added lines are judged, and only when they flip the file from
// included to excluded under the current GOOS, GOARCH and effective build
// tags: a file that already carried the constraint is not affected.
// path gates on the .go extension; before nil is a new file. root is the
// workspace whose gate tags (TestTagsForWorkspace) join GOFLAGS.
func RejectAddedBuildExclusion(root, path string, before, after []byte) error {
	if !strings.EqualFold(filepath.Ext(path), ".go") {
		return nil
	}
	beforeGo, beforePlus := headerBuildConstraints(before)
	beforeSet := make(map[string]bool, len(beforeGo)+len(beforePlus))
	for _, line := range beforeGo {
		beforeSet["go:"+line] = true
	}
	for _, line := range beforePlus {
		beforeSet["plus:"+line] = true
	}
	afterGo, afterPlus := headerBuildConstraints(after)
	var added []string
	for _, line := range afterGo {
		if !beforeSet["go:"+line] {
			added = append(added, "//go:build "+line)
		}
	}
	for _, line := range afterPlus {
		if !beforeSet["plus:"+line] {
			added = append(added, "// +build "+line)
		}
	}
	if len(added) == 0 {
		return nil
	}
	tags := currentBuildTags(root)
	if buildConstraintsExclude(afterGo, afterPlus, tags) && !buildConstraintsExclude(beforeGo, beforePlus, tags) {
		return fmt.Errorf("refusing to write %s: the edit adds %s, which excludes this file from the current build (GOOS=%s GOARCH=%s tags=%s); hiding a file from the build is not a fix",
			path, strings.Join(added, ", "), runtime.GOOS, runtime.GOARCH, formatBuildTags(tags))
	}
	return nil
}

// moduleRootForTags walks from path to the nearest go.mod so a caller that
// only has a file path still evaluates constraints against that module's
// gate tags. RejectGoSyntaxRegression is that caller: lines.go passes the
// resolved path and no workspace. A sqlite_vec file in this repo is part of
// the build, so an added //go:build sqlite_vec must not be refused here. No
// go.mod means no gate tags; ignore is still never satisfied.
func moduleRootForTags(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	dir := abs
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// currentBuildTags is the tag set the current build evaluates constraints
// against: the workspace's gate tags (TestTagsForWorkspace: this repo's
// gates build with sqlite_vec, so a sqlite_vec-gated file is not hidden
// here) plus whatever GOFLAGS carries in this process.
func currentBuildTags(root string) map[string]bool {
	tags := map[string]bool{}
	addTags := func(value string) {
		for _, field := range strings.FieldsFunc(value, func(r rune) bool { return r == ' ' || r == '\t' || r == ',' }) {
			if field = strings.Trim(field, `"'`); field != "" {
				tags[field] = true
			}
		}
	}
	flags := build.TestTagsForWorkspace(root)
	for i := 0; i < len(flags); i++ {
		if flags[i] == "-tags" && i+1 < len(flags) {
			i++
			addTags(flags[i])
		} else if rest, ok := strings.CutPrefix(flags[i], "-tags="); ok {
			addTags(rest)
		}
	}
	goflags := strings.Fields(os.Getenv("GOFLAGS"))
	for i := 0; i < len(goflags); i++ {
		if goflags[i] == "-tags" && i+1 < len(goflags) {
			i++
			addTags(goflags[i])
		} else if rest, ok := strings.CutPrefix(goflags[i], "-tags="); ok {
			addTags(rest)
		}
	}
	return tags
}

func formatBuildTags(tags map[string]bool) string {
	if len(tags) == 0 {
		return "[]"
	}
	names := make([]string, 0, len(tags))
	for name := range tags {
		names = append(names, name)
	}
	sort.Strings(names)
	return "[" + strings.Join(names, " ") + "]"
}

// headerBuildConstraints returns the //go:build and // +build expressions a
// .go file carries in its header: line comments before the package clause,
// matched the way the toolchain honors them. A bare mention inside other
// text (a string literal, a prose comment, a constraint after the package
// clause) is not a constraint, and a file with no package clause builds
// nowhere, so nothing in it can hide it. The toolchain also requires a blank
// line after the constraint paragraph; without one the lines are inert, and
// refusing an edit for inert lines would be a false positive.
func headerBuildConstraints(content []byte) (goLines, plusLines []string) {
	lines := strings.Split(string(content), "\n")
	packageIdx := -1
	for i, line := range lines {
		fields := strings.Fields(strings.TrimSuffix(line, "\r"))
		if len(fields) > 0 && fields[0] == "package" {
			packageIdx = i
			break
		}
	}
	if packageIdx < 0 {
		return nil, nil
	}
	type candidate struct {
		idx        int
		goBuild    bool
		expression string
	}
	var found []candidate
	for i := 0; i < packageIdx; i++ {
		trimmed := strings.TrimSuffix(lines[i], "\r")
		trimmed = strings.TrimSpace(trimmed)
		rest, ok := strings.CutPrefix(trimmed, "//")
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		if expr, ok := strings.CutPrefix(rest, "go:build"); ok {
			found = append(found, candidate{idx: i, goBuild: true, expression: strings.TrimSpace(expr)})
		} else if expr, ok := strings.CutPrefix(rest, "+build"); ok {
			found = append(found, candidate{idx: i, expression: strings.TrimSpace(expr)})
		}
	}
	if len(found) == 0 {
		return nil, nil
	}
	last := found[len(found)-1].idx
	blanked := false
	for i := last + 1; i < packageIdx; i++ {
		if strings.TrimSpace(strings.TrimSuffix(lines[i], "\r")) == "" {
			blanked = true
			break
		}
	}
	if !blanked {
		return nil, nil
	}
	for _, c := range found {
		if c.goBuild {
			goLines = append(goLines, c.expression)
		} else {
			plusLines = append(plusLines, c.expression)
		}
	}
	return goLines, plusLines
}

// buildConstraintsExclude reports whether a file carrying these header
// expressions is excluded from the current build: any unsatisfied line
// excludes (the toolchain ANDs lines), and a file with no lines is
// included. A line the toolchain cannot parse fails closed: a constraint
// addition that is not even well-formed can never be a correct edit.
func buildConstraintsExclude(goLines, plusLines []string, tags map[string]bool) bool {
	ok := func(tag string) bool { return buildTagSatisfied(tag, tags) }
	for _, expr := range goLines {
		// Parse takes the whole comment line, not the bare expression.
		parsed, err := constraint.Parse("//go:build " + expr)
		if err != nil {
			return true
		}
		if !parsed.Eval(ok) {
			return true
		}
	}
	for _, expr := range plusLines {
		if !plusBuildSatisfied(expr, ok) {
			return true
		}
	}
	return false
}

// plusBuildSatisfied evaluates one // +build line: space-separated
// alternatives of comma-separated conjunctions, each term optionally
// negated. A structurally empty alternative satisfies nothing.
func plusBuildSatisfied(expr string, ok func(string) bool) bool {
	alts := strings.Fields(expr)
	if len(alts) == 0 {
		return false
	}
	for _, alt := range alts {
		terms := strings.Split(alt, ",")
		satisfied := true
		for _, term := range terms {
			negated := false
			for strings.HasPrefix(term, "!") {
				negated = !negated
				term = term[1:]
			}
			if term == "" || ok(term) == negated {
				satisfied = false
				break
			}
		}
		if satisfied {
			return true
		}
	}
	return false
}

// buildTagSatisfied reports whether one constraint tag holds for the current
// build: the platform (GOOS, GOARCH, compiler), the toolchain version for
// go1.x tags, cgo unless explicitly disabled, and the effective build tags.
// ignore is never satisfied: by convention no build passes -tags=ignore.
func buildTagSatisfied(tag string, tags map[string]bool) bool {
	switch {
	case tag == "ignore":
		return false
	case tag == runtime.GOOS || tag == runtime.GOARCH || tag == runtime.Compiler:
		return true
	case tag == "cgo":
		return os.Getenv("CGO_ENABLED") != "0"
	case len(tag) > 2 && strings.HasPrefix(tag, "go") && tag[2] >= '0' && tag[2] <= '9':
		if version.IsValid(tag) && version.IsValid(runtime.Version()) {
			return version.Compare(runtime.Version(), tag) >= 0
		}
		return tags[tag]
	default:
		return tags[tag]
	}
}
