package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	internalbuild "codenerd/internal/build"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The pinning gate (external audit N22, 2026-09-19) asks of every function a
// turn changed whether a test the turn wrote would notice the change gone.
// Coverage asks whether a test executes the changed lines; a unit test of a
// new helper executes all of them while the behaviour the helper exists for
// stays unpinned. Ladder run R1-10: the fix added a helper and called it from
// two places, its five new tests exercised the helper, every line was
// covered, and all five still passed with both call sites put back as they
// were -- the reviewer's fail-before check, done by hand, was the only thing
// that noticed.
//
// The check is mechanical because the executor holds each written file's
// preimage. One change at a time -- a changed function put back as it was, an
// added one taken out -- is overlaid on the workspace with go test -overlay,
// nothing on disk touched, and the turn's own tests are run against it. A
// change is pinned when they fail or no longer compile; it is unpinned when
// they all pass. Anything that stops the measurement short -- a timeout, a
// cancel, an overlay that cannot be written -- is not a verdict either way.

// pinUnit is one change the gate can take out on its own.
type pinUnit struct {
	path string // the written path, workspace-relative
	name string // the function ("Recv.Name" for a method), or "" for the file
	// added is a function the turn created: taking it out removes it.
	added bool
	// absent takes the whole file out: a created file reverted.
	absent bool
	// forced is a condition on a changed line held at true or at false
	// (condition_units.go) rather than a declaration taken out.
	forced bool
	// content is the file with this one change taken out.
	content string
}

func (u pinUnit) label() string {
	switch {
	case u.name == "":
		return u.path + ": its declarations outside functions"
	case u.added:
		return u.path + ": " + u.name + " (added by this turn)"
	default:
		return u.path + ": " + u.name
	}
}

// funcDecl is one top-level function as its source declares it.
type funcDecl struct {
	// tokens is what the function is: its token stream with comments and
	// line breaks dropped, so a reformatted or re-commented function is the
	// same function.
	tokens     string
	start, end int // byte offsets of the declaration, doc comment excluded
}

// funcDecls indexes the top-level functions of src by name, methods by
// "Recv.Name". init functions are left out: a file may declare several, and
// no name tells them apart. ok is false when src does not parse.
func funcDecls(src string) (map[string]funcDecl, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	out := make(map[string]funcDecl)
	for _, d := range f.Decls {
		fn, isFunc := d.(*ast.FuncDecl)
		if !isFunc || fn.Name.Name == "init" || fn.Name.Name == "_" {
			continue
		}
		key := funcKey(fn)
		if _, dup := out[key]; dup {
			continue
		}
		start, end := fset.Position(fn.Pos()).Offset, fset.Position(fn.End()).Offset
		out[key] = funcDecl{tokens: tokenText(src[start:end]), start: start, end: end}
	}
	return out, true
}

// funcKey names a function the way the gate reports it: "Name", or
// "Recv.Name" for a method, the receiver's pointer and type parameters
// dropped.
func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	recv := fn.Recv.List[0].Type
	for {
		switch t := recv.(type) {
		case *ast.StarExpr:
			recv = t.X
			continue
		case *ast.IndexExpr:
			recv = t.X
			continue
		case *ast.IndexListExpr:
			recv = t.X
			continue
		case *ast.ParenExpr:
			recv = t.X
			continue
		}
		break
	}
	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

// tokenText is src's token stream without comments or semicolons, one token
// per line: two sources with the same tokenText are the same code however
// they are laid out.
func tokenText(src string) string {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	s.Init(file, []byte(src), nil, 0)
	var b strings.Builder
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.SEMICOLON {
			continue
		}
		b.WriteString(tok.String())
		if lit != "" {
			b.WriteByte(' ')
			b.WriteString(lit)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// otherDecls is every top-level declaration of src that is not a function or
// an import, as sorted token texts: moving a type is not a change.
func otherDecls(src string) ([]string, bool) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	var out []string
	for _, d := range f.Decls {
		g, isGen := d.(*ast.GenDecl)
		if !isGen || g.Tok == token.IMPORT {
			continue
		}
		out = append(out, tokenText(src[fset.Position(g.Pos()).Offset:fset.Position(g.End()).Offset]))
	}
	sort.Strings(out)
	return out, true
}

// pinUnits lists the changes the gate takes out one at a time: in each
// production Go file the turn wrote, every function it changed or added. A
// file whose only changes are outside its functions -- a constant, a type --
// is one unit, the whole file put back. Functions the turn deleted have
// nothing to pin, and comment or layout changes are not changes.
func pinUnits(workspace string, written []string, preWrite map[string]PreImage) []pinUnit {
	var units []pinUnit
	seen := make(map[string]bool, len(written))
	paths := append([]string(nil), written...)
	sort.Strings(paths)
	for _, p := range paths {
		if seen[p] || !strings.HasSuffix(strings.ToLower(p), ".go") || isTestPath(p) || ignoredByGoTool(p) {
			continue
		}
		seen[p] = true
		pre, ok := preImageFor(workspace, p, preWrite)
		if !ok || !pre.Known() {
			continue
		}
		disk := diskPath(workspace, p)
		if included, err := build.Default.MatchFile(filepath.Dir(disk), filepath.Base(disk)); err != nil || !included {
			continue
		}
		data, err := os.ReadFile(disk)
		if err != nil {
			continue
		}
		cur := string(data)
		units = append(units, fileUnits(p, cur, pre)...)
	}
	return units
}

// ignoredByGoTool reports whether the go tool never builds p: a file under a
// testdata directory, or under one whose name starts with "_" or ".". A
// fixture there has no test that could fail without a change to it.
func ignoredByGoTool(p string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(p)), "/") {
		if seg == "testdata" || (seg != "." && seg != ".." && (strings.HasPrefix(seg, "_") || strings.HasPrefix(seg, "."))) {
			return true
		}
	}
	return false
}

// fileUnits is pinUnits for one file.
func fileUnits(path, cur string, pre PreImage) []pinUnit {
	curFns, ok := funcDecls(cur)
	if !ok {
		return nil
	}
	preFns := map[string]funcDecl{}
	if pre.Existed {
		if preFns, ok = funcDecls(pre.Content); !ok {
			return nil
		}
	}
	keys := make([]string, 0, len(curFns))
	for k := range curFns {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var units []pinUnit
	for _, k := range keys {
		c := curFns[k]
		o, existed := preFns[k]
		switch {
		case existed && o.tokens == c.tokens:
			continue
		case existed:
			spliced := cur[:c.start] + pre.Content[o.start:o.end] + cur[c.end:]
			units = append(units, pinUnit{path: path, name: k, content: reconcileImports(spliced, pre.Content)})
		default:
			spliced := cur[:c.start] + cur[c.end:]
			units = append(units, pinUnit{path: path, name: k, added: true, content: reconcileImports(spliced, pre.Content)})
		}
	}
	if len(units) > 0 {
		return units
	}
	curOther, okCur := otherDecls(cur)
	preOther, okPre := []string(nil), true
	if pre.Existed {
		preOther, okPre = otherDecls(pre.Content)
	}
	if !okCur || !okPre || strings.Join(curOther, "\x00") == strings.Join(preOther, "\x00") {
		return nil
	}
	return []pinUnit{{path: path, absent: !pre.Existed, content: pre.Content}}
}

// preImageFor finds a written path's preimage under the key the executor
// recorded it by.
func preImageFor(workspace, p string, preWrite map[string]PreImage) (PreImage, bool) {
	if pre, ok := preWrite[p]; ok {
		return pre, true
	}
	if key := canonicalizeWrittenPath(p, workspace); key != "" {
		pre, ok := preWrite[key]
		return pre, ok
	}
	return PreImage{}, false
}

// reconcileImports keeps a spliced file compiling where the splice alone
// would not: an import only the taken-out code used is dropped, and one the
// restored code needs is taken back from the preimage. A package's name is
// read from its import path; where that guess is wrong the file does not
// compile, which the gate reads as pinned -- a miss, never a false charge.
func reconcileImports(src, pre string) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		return src
	}
	full, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if err != nil {
		return src
	}
	used := selectorRoots(full)
	type cut struct{ start, end int }
	var cuts []cut
	have := make(map[string]bool)
	for _, d := range f.Decls {
		g, isGen := d.(*ast.GenDecl)
		if !isGen || g.Tok != token.IMPORT {
			continue
		}
		var dropped int
		for _, s := range g.Specs {
			spec := s.(*ast.ImportSpec)
			name := importLocalName(spec)
			have[name] = true
			if name == "_" || name == "." || name == "C" || used[name] {
				continue
			}
			cuts = append(cuts, cut{fset.Position(spec.Pos()).Offset, fset.Position(spec.End()).Offset})
			dropped++
		}
		if dropped == len(g.Specs) && !g.Lparen.IsValid() {
			// A lone `import "x"`: the keyword goes with its only spec.
			cuts[len(cuts)-1] = cut{fset.Position(g.Pos()).Offset, fset.Position(g.End()).Offset}
		}
	}
	var add []string
	if pf, err := parser.ParseFile(token.NewFileSet(), "", pre, parser.ImportsOnly|parser.SkipObjectResolution); err == nil {
		for _, spec := range pf.Imports {
			name := importLocalName(spec)
			if name == "_" || name == "." || have[name] || !used[name] {
				continue
			}
			have[name] = true
			line := spec.Path.Value
			if spec.Name != nil {
				line = spec.Name.Name + " " + line
			}
			add = append(add, "import "+line)
		}
	}
	out := src
	for i := len(cuts) - 1; i >= 0; i-- {
		out = out[:cuts[i].start] + out[cuts[i].end:]
	}
	if len(add) > 0 {
		at := fset.Position(f.Name.End()).Offset
		out = out[:at] + "\n\n" + strings.Join(add, "\n") + "\n" + out[at:]
	}
	return out
}

// selectorRoots is every identifier used as the left side of a selector:
// the names a file can be using an import by.
func selectorRoots(f *ast.File) map[string]bool {
	used := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	return used
}

var majorVersion = regexp.MustCompile(`^v[0-9]+$`)

// importLocalName is the name a file refers to an import by: its alias, or
// the package name guessed from the path -- the last element, a /vN suffix
// skipped, a go- prefix and any .suffix or -suffix dropped.
func importLocalName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return ""
	}
	parts := strings.Split(path, "/")
	last := parts[len(parts)-1]
	if len(parts) > 1 && majorVersion.MatchString(last) {
		last = parts[len(parts)-2]
	}
	last = strings.TrimPrefix(last, "go-")
	if i := strings.IndexAny(last, ".-"); i > 0 {
		last = last[:i]
	}
	return last
}

// turnTests are the tests this turn wrote: in each _test.go file it wrote
// that the default build includes and that sits in a package the test gate
// runs, every Test, Example or Fuzz function that is new or whose code
// changed. A test that already failed before the turn pins nothing, and a
// benchmark does not run under -run. gated counts the turn's tests left out
// because their package or file is behind a build tag.
func turnTests(workspace string, written []string, preWrite map[string]PreImage, preExisting []string) (names, pkgs []string, gated int) {
	failing := make(map[string]bool, len(preExisting))
	for _, n := range preExisting {
		failing[n] = true
	}
	var files []string
	for _, p := range written {
		if isTestPath(p) {
			files = append(files, p)
		}
	}
	sort.Strings(files)
	runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(files))
	isRunnable := make(map[string]bool, len(runnable))
	for _, pkg := range runnable {
		isRunnable[pkg] = true
	}
	nameSet, pkgSet := map[string]bool{}, map[string]bool{}
	for _, p := range files {
		pre, ok := preImageFor(workspace, p, preWrite)
		if !ok || !pre.Known() {
			continue
		}
		data, err := os.ReadFile(diskPath(workspace, p))
		if err != nil {
			continue
		}
		curFns, ok := funcDecls(string(data))
		if !ok {
			continue
		}
		preFns := map[string]funcDecl{}
		if pre.Existed {
			if preFns, ok = funcDecls(pre.Content); !ok {
				continue
			}
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "", data, parser.SkipObjectResolution)
		if err != nil {
			continue
		}
		disk := diskPath(workspace, p)
		included, _ := build.Default.MatchFile(filepath.Dir(disk), filepath.Base(disk))
		pkg := packagesForPaths([]string{p})
		for _, d := range f.Decls {
			fn, isFunc := d.(*ast.FuncDecl)
			if !isFunc || fn.Recv != nil || !isTestFuncDecl(fn) || strings.HasPrefix(fn.Name.Name, "Benchmark") || failing[fn.Name.Name] {
				continue
			}
			if o, existed := preFns[fn.Name.Name]; existed && o.tokens == curFns[fn.Name.Name].tokens {
				continue
			}
			if !included || len(pkg) == 0 || !isRunnable[pkg[0]] {
				gated++
				continue
			}
			nameSet[fn.Name.Name] = true
			pkgSet[pkg[0]] = true
		}
	}
	for n := range nameSet {
		names = append(names, n)
	}
	for p := range pkgSet {
		pkgs = append(pkgs, p)
	}
	sort.Strings(names)
	sort.Strings(pkgs)
	return names, pkgs, gated
}

// verifyPinning measures the gate: every change of the turn's, taken out on
// its own, against the tests the turn wrote. It passes when each one breaks
// them (or there is no change to pin), fails naming the ones that do not --
// or naming every change when the turn wrote no test that could -- and is
// indeterminate when a measurement did not finish.
// measureUnits runs the turn's tests against each unit, a few at a time, and
// reports the units the tests did not notice and the ones that were not
// measured.
func measureUnits(ctx context.Context, workspace string, units []pinUnit, runArg string, names, pkgs []string, bound time.Duration) (unpinned, unmeasured []string) {
	type measured struct{ verdict, why string }
	results := make([]measured, len(units))
	var wg sync.WaitGroup
	slots := make(chan struct{}, pinWorkers())
	for i, u := range units {
		wg.Add(1)
		go func(i int, u pinUnit) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			verdict, why := runPinUnit(ctx, workspace, u, runArg, names, pkgs, bound)
			logging.Get(logging.CategorySession).Info("pinning gate: %s -> %s%s", u.label(), verdict, suffixed(why))
			results[i] = measured{verdict, why}
		}(i, u)
	}
	wg.Wait()
	for i, r := range results {
		switch r.verdict {
		case "unpinned":
			unpinned = append(unpinned, units[i].label())
		case "unmeasured":
			unmeasured = append(unmeasured, units[i].label()+" ("+r.why+")")
		}
	}
	return unpinned, unmeasured
}

// conditionsFor is every condition on a line the turn changed, in each
// production Go file it wrote.
func conditionsFor(workspace string, result *ExecutionResult) []pinUnit {
	var units []pinUnit
	for _, p := range result.WrittenPaths {
		if !strings.HasSuffix(strings.ToLower(p), ".go") || isTestPath(p) || ignoredByGoTool(p) {
			continue
		}
		pre, ok := preImageFor(workspace, p, result.PreWriteContents)
		if !ok || !pre.Known() {
			continue
		}
		data, err := os.ReadFile(diskPath(workspace, p))
		if err != nil {
			continue
		}
		units = append(units, conditionUnits(p, string(data), pre)...)
	}
	return units
}

// verifyPinning measures the gate. withConditions also asks the advisory
// question about the decisions inside the change (conditionsFor); the closure
// re-measures without it, because nothing reads the answer.
func verifyPinning(ctx context.Context, workspace string, result *ExecutionResult, withConditions bool) BuildVerification {
	start := time.Now()
	units := pinUnits(workspace, result.WrittenPaths, result.PreWriteContents)
	if len(units) == 0 {
		return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Reason: "the turn changed no function", Duration: time.Since(start)}
	}
	names, pkgs, gated := turnTests(workspace, result.WrittenPaths, result.PreWriteContents, result.TestCheck.PreExistingFailures)
	if len(names) == 0 {
		if gated > 0 {
			return BuildVerification{Outcome: VerifyIndeterminate, Reason: fmt.Sprintf("the %d test(s) this turn wrote are behind build tags the gate does not run", gated), Duration: time.Since(start)}
		}
		return BuildVerification{Ran: true, Outcome: VerifyFailed, Output: noTurnTestsListing(units),
			Reason: "the turn changed functions and wrote no test", Duration: time.Since(start)}
	}
	runArg := baselineRunRegex(names)
	bound, why := pinBaseline(ctx, workspace, runArg, names, pkgs)
	if bound <= 0 {
		return BuildVerification{Outcome: VerifyIndeterminate, Reason: "the turn's tests were not measured on their own first: " + why, Duration: time.Since(start)}
	}
	unpinned, unmeasured := measureUnits(ctx, workspace, units, runArg, names, pkgs, bound)
	if errors.Is(ctx.Err(), context.Canceled) {
		return BuildVerification{Outcome: VerifyCanceled, Reason: "canceled while measuring", Duration: time.Since(start)}
	}
	if withConditions {
		// The decisions inside the changed functions are asked about too, and
		// what survives is recorded rather than charged: measured over a
		// hand-written change (2026-09-19, N24's own commit) five of eight
		// surviving conditions were guards whose forcing changes nothing
		// observable, and a verdict cannot rest on a question whose answer is
		// sometimes unanswerable. The round hands them to the model, the log
		// and the record keep them for the review.
		conditions := conditionsFor(workspace, result)
		survived, _ := measureUnits(ctx, workspace, conditions, runArg, names, pkgs, bound)
		result.PinAdvisory = survived
		if len(survived) > 0 {
			logging.Get(logging.CategorySession).Warn(
				"%d decision(s) this turn made are not distinguished by the tests it wrote (not charged):\n%s",
				len(survived), strings.Join(survived, "\n"))
		}
	}
	command := append([]string{"go", "test", "-overlay", "<one change taken out>", "-count=1", "-v", "-run", runArg}, pkgs...)
	switch {
	case len(unpinned) > 0:
		return BuildVerification{Ran: true, Outcome: VerifyFailed, Command: command, Output: unpinnedListing(unpinned, names),
			Reason: fmt.Sprintf("%d of %d change(s) pinned by no test this turn wrote", len(unpinned), len(units)), Duration: time.Since(start)}
	case len(unmeasured) > 0:
		return BuildVerification{Ran: true, Outcome: VerifyIndeterminate, Command: command,
			Reason: "not measured: " + strings.Join(unmeasured, "; "), Duration: time.Since(start)}
	default:
		return BuildVerification{Ran: true, OK: true, Outcome: VerifyPassed, Command: command, Duration: time.Since(start)}
	}
}

// pinWorkers is how many changes are measured at once. Each measurement is a
// go test that compiles and links a test binary with every processor it can
// get, so a handful at a time keeps a turn of many changes from measuring
// them one by one without starving the machine: an eighth of the processors,
// at least one.
func pinWorkers() int {
	return max(1, runtime.GOMAXPROCS(0)/8)
}

func suffixed(why string) string {
	if why == "" {
		return ""
	}
	return ": " + why
}

// runPinUnit runs the turn's tests with one change taken out. "pinned" when
// they fail or no longer compile, "unpinned" when they ran and passed,
// "unmeasured" with the reason otherwise.
func runPinUnit(ctx context.Context, workspace string, u pinUnit, runArg string, names, pkgs []string, bound time.Duration) (string, string) {
	tmpDir, err := os.MkdirTemp("", "pin-unit-*")
	if err != nil {
		return "unmeasured", err.Error()
	}
	defer os.RemoveAll(tmpDir)
	abs := diskPath(workspace, filepath.FromSlash(u.path))
	replace := map[string]string{abs: ""}
	if !u.absent {
		stand := filepath.Join(tmpDir, "unit.go")
		if err := os.WriteFile(stand, []byte(u.content), 0o644); err != nil {
			return "unmeasured", err.Error()
		}
		replace[abs] = stand
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": replace})
	if err != nil {
		return "unmeasured", err.Error()
	}
	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o644); err != nil {
		return "unmeasured", err.Error()
	}
	args := append([]string{"test", "-overlay", overlayPath, "-count=1", "-v", "-timeout", bound.String(), "-run", runArg}, pkgs...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), testVerifyTimeout, "go", args, verifyTestRunner)
	switch outcome {
	case VerifyFailed:
		if testBuildFailed(string(out)) {
			return "pinned", "the tests do not compile without it"
		}
		return "pinned", "failing: " + strings.Join(topLevelFailedTests(string(out)), ", ")
	case VerifyPassed:
		if !anyTestRan(string(out), names) {
			return "unmeasured", "none of the turn's tests ran"
		}
		return "unpinned", ""
	default:
		return "unmeasured", reason
	}
}

// pinBaseline runs the turn's tests as they are, before anything is taken
// out. It answers two questions with one run: whether they pass and really
// run (a gate measured against tests that do not is measuring nothing), and
// how long they take -- which bounds every mutant, so one that leaves the
// tests spinning is cut instead of holding the turn. Measured 2026-09-19 on a
// prototype: one forced condition left a lock test waiting ten minutes.
func pinBaseline(ctx context.Context, workspace, runArg string, names, pkgs []string) (time.Duration, string) {
	start := time.Now()
	args := append([]string{"test", "-count=1", "-v", "-run", runArg}, pkgs...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), testVerifyTimeout, "go", args, verifyTestRunner)
	switch {
	case outcome != VerifyPassed:
		return 0, fmt.Sprintf("they did not pass on their own (%s%s)", outcome, suffixed(reason))
	case !anyTestRan(string(out), names):
		return 0, "none of them ran"
	}
	return 2*time.Since(start) + time.Minute, ""
}

// anyTestRan reports whether go test -v output shows one of names finishing.
func anyTestRan(out string, names []string) bool {
	for _, n := range names {
		for _, verb := range []string{"--- PASS: ", "--- SKIP: "} {
			if strings.Contains(out, verb+n+" ") {
				return true
			}
		}
	}
	return false
}

func unpinnedListing(unpinned, names []string) string {
	return "Each change below can be undone -- a function put back as it was, one this turn added " +
		"removed, or a condition it wrote held at a constant -- and every test this turn wrote still " +
		"passes (" + strings.Join(names, ", ") + "):\n\n```\n" +
		strings.Join(unpinned, "\n") + "\n```"
}

func noTurnTestsListing(units []pinUnit) string {
	labels := make([]string, 0, len(units))
	for _, u := range units {
		labels = append(labels, u.label())
	}
	return "This turn changed the functions below and wrote no test, so nothing would notice if a change were lost:\n\n```\n" +
		strings.Join(labels, "\n") + "\n```"
}

func pinningRepairPrompt(seed string) string {
	return seed + "\n\n" +
		"For each one, write or extend a test that fails when the change is taken out and passes with it: drive the " +
		"path the change sits on -- the function named, or a caller that reaches it -- and assert the behaviour the " +
		"change was made for. For a condition, the test that pins it takes the branch the forced value would skip, and " +
		"asserts what that branch is for. A test of a helper does not pin the place that calls it. Put the tests in the package's " +
		"_test.go files. Do not change the production code to make a test fail, and do not take a change out to clear " +
		"this list: the change is what the tests must pin. The check is run again afterwards."
}

// advisorySection adds the decisions the tests do not distinguish to a round
// that is already running. They are not what the round must clear -- nothing
// fails for them -- but the round is the one moment the model is already
// writing tests for this change.
func advisorySection(advisory []string) string {
	if len(advisory) == 0 {
		return ""
	}
	return "\n\nWorth pinning while you are here: these decisions your change makes are not distinguished by " +
		"any test you wrote -- held at a constant, the tests still pass. Where one of them is a real choice " +
		"(and not a guard whose branch changes nothing observable), a test that takes the other side is worth " +
		"writing; the turn does not fail for them.\n\n```\n" + strings.Join(advisory, "\n") + "\n```"
}

func pinningBrokeTestsPrompt(testOutput string) string {
	return "The tests fail after your last edit:\n\n```\n" + testOutput + "\n```\n\n" +
		"A test that pins a change passes with the change in place. Fix the test so it passes against the code as it " +
		"is and fails when the change is taken out; do not change the production code to make it pass."
}

// verifyAndRepairPinning is the forcing round for the /pinned gate. The
// policy decides whether the turn owes it (coder_safety.mg: a behaviour
// change -- /fix, /create, /implement -- that wrote Go); the executor measures
// it. The model gets rounds to write the tests that pin what is unpinned; a
// round that does not converge leaves /change_not_pinned to the verdict, and
// one that gives up with the suite red is undone.
func (e *Executor) verifyAndRepairPinning(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) (*types.LLMToolResponse, []string, error) {
	if result == nil || result.SuccessfulWriteTools == 0 || !touchedGoFiles(result.WrittenPaths) || e.kernel == nil {
		return nil, nil, nil
	}
	if e.sessionContext != nil && e.sessionContext.DreamMode {
		return nil, nil, nil
	}
	if !e.configSnapshot().VerifyTestsAfterEdits || result.TestCheck.Verdict() != VerifyPassed {
		return nil, nil, nil
	}
	if !e.turnOwesGate(result, "/pinned") {
		return nil, nil, nil
	}
	workspace := e.workspaceForVerification()
	result.PinCheck = verifyPinning(ctx, workspace, result, true)
	if result.PinCheck.Verdict() != VerifyFailed || trp == nil {
		return nil, nil, nil
	}
	logging.Get(logging.CategorySession).Warn("the tests this turn wrote do not pin its change; giving the model rounds to write ones that do:\n%s", result.PinCheck.Output)
	snap, green, greenPin := snapshotTurnFiles(workspace, result), result.TestCheck, result.PinCheck
	testsBroke := false
	spec := repairSpec{
		kind:         "pinning",
		brokenPhrase: "the tests this turn wrote pass without a change it made",
		promptFor: func(seed string) string {
			if testsBroke {
				return pinningBrokeTestsPrompt(seed)
			}
			return pinningRepairPrompt(seed) + advisorySection(result.PinAdvisory)
		},
		recheck: func(epCtx context.Context) (bool, string, VerifyOutcome) {
			testsBroke = false
			v, _ := gateOwnTests(epCtx, workspace, result, false)
			if v.Verdict() == VerifyPassed || v.Verdict() == VerifyFailed {
				v.Repair = result.TestCheck.Repair
				result.TestCheck = v
			}
			if v.Verdict() != VerifyPassed {
				testsBroke = v.Verdict() == VerifyFailed
				return false, v.Output, v.Verdict()
			}
			p := verifyPinning(epCtx, workspace, result, true)
			result.PinCheck = p
			snap, green, greenPin = snapshotTurnFiles(workspace, result), result.TestCheck, p
			if p.Verdict() == VerifyPassed {
				return true, "", VerifyPassed
			}
			return false, p.Output, p.Verdict()
		},
		followups: func() []string {
			runnable, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
			return repairFollowups(workspace, runnable, result, "tests")
		},
	}
	repaired, repairErrs, rec, err := e.repairLoop(ctx, trp, systemPrompt, &history, toolDefs, cfg, result, result.PinCheck.Output, spec)
	if undoRedRound("pinning", workspace, result, snap, green, err) {
		result.PinCheck = greenPin
		repaired = nil
	}
	result.PinCheck.Repair = rec
	return repaired, repairErrs, settleForcingRepair(err)
}
