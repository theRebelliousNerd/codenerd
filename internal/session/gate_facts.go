package session

import (
	"context"
	"path"
	"path/filepath"
	"sort"
	"strings"

	internalbuild "codenerd/internal/build"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The /test, /vet, /check, /test_run, /test_retention and /build gates are
// derived (coder_safety.mg), and so is the critic's triage (turn_needs_uplift).
// These helpers assert the measurements and read the derived verdicts back.
// They do not choose them. /pinned stays asserted in recordBuildState: one
// source per gate.

// retractTurnPredicates removes this turn's rows of each predicate and drops
// them from the cleanup list. RetractFact matches the predicate and the first
// argument, which is the turn for every fact these gates assert.
func (e *Executor) retractTurnPredicates(turn types.MangleAtom, predicates ...string) {
	if e == nil || e.kernel == nil || turn == "" || len(predicates) == 0 {
		return
	}
	drop := make(map[string]struct{}, len(predicates))
	for _, predicate := range predicates {
		drop[predicate] = struct{}{}
		if err := e.kernel.RetractFact(types.Fact{Predicate: predicate, Args: []any{turn}}); err != nil {
			logging.Get(logging.CategorySession).Warn("gate facts: retract %s(%s): %v", predicate, turn, err)
		}
	}
	e.mu.Lock()
	kept := make([]types.Fact, 0, len(e.turnFacts))
	for _, fact := range e.turnFacts {
		if _, ok := drop[fact.Predicate]; ok && len(fact.Args) > 0 && types.ExtractString(fact.Args[0]) == string(turn) {
			continue
		}
		kept = append(kept, fact)
	}
	e.turnFacts = kept
	e.mu.Unlock()
}

// syncTestGateFacts replaces this turn's /test measurements with the head
// run as it stands now. A repair re-runs the suite; the previous rows would
// otherwise stay, and RetractFact would not tell two messages for one test
// apart. test_state is not touched: a suite that fails only on tests that
// already failed is still a failing suite, and the turn is not charged for it.
func (e *Executor) syncTestGateFacts(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn,
		"turn_test_measured", "turn_test_failed_before",
		"turn_test_case", "turn_test_failure_at", "turn_test_build_failure",
		"turn_test_output_repeat", "turn_failing_test")
	switch result.TestCheck.Verdict() {
	case VerifyPassed:
		e.assertTurnFact(types.Fact{
			Predicate: "turn_test_measured",
			Args:      []any{turn, types.MangleAtom("/passing")},
		})
	case VerifyFailed:
		e.assertTurnFact(types.Fact{
			Predicate: "turn_test_measured",
			Args:      []any{turn, types.MangleAtom("/failing")},
		})
	}
	if result.TestCheck.BaselineRan {
		for _, name := range result.TestCheck.BaselineFailures {
			if name == "" {
				continue
			}
			e.assertTurnFact(types.Fact{
				Predicate: "turn_test_failed_before",
				Args:      []any{turn, types.MangleString(name)},
			})
		}
	}
	if result.TestCheck.Ran && result.TestCheck.Result != nil {
		for _, fact := range turnScopedTestFacts(turn, result.TestCheck.Result) {
			e.assertTurnFact(fact)
		}
	}
	// The importer run is the other measurement of this same gate.
	e.syncImporterGateFacts(turn, result)
}

// syncVetGateFacts replaces this turn's /vet measurements. A conclusive run
// asserts turn_vet_ran and the counts verifyVet stored. A hand-built failure
// whose output is a positioned diagnostic is the same measurement, with no
// baseline, so every finding it names is charged. A failure that names
// nothing asserts nothing.
func (e *Executor) syncVetGateFacts(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn, "turn_vet_ran", "turn_vet_finding", "turn_vet_before")
	v := result.VetCheck
	now, before := v.VetNow, v.VetBefore
	beforeKnown := v.VetBeforeKnown
	if !v.VetMeasured {
		if v.Verdict() != VerifyFailed {
			return
		}
		now = vetDiagnostics(v.Output, func(p string) string { return filepath.ToSlash(p) })
		if len(now) == 0 {
			return
		}
		before, beforeKnown = nil, false
	}
	e.assertTurnFact(types.Fact{Predicate: "turn_vet_ran", Args: []any{turn}})
	e.assertVetCounts(turn, "turn_vet_finding", now)
	if beforeKnown {
		e.assertVetCounts(turn, "turn_vet_before", before)
	}
}

func (e *Executor) assertVetCounts(turn types.MangleAtom, predicate string, findings []vetDiagnostic) {
	type key struct{ file, message string }
	counts := make(map[key]int)
	var order []key
	for _, finding := range findings {
		k := key{finding.file, finding.message}
		if counts[k] == 0 {
			order = append(order, k)
		}
		counts[k]++
	}
	for _, k := range order {
		e.assertTurnFact(types.Fact{
			Predicate: predicate,
			Args:      []any{turn, types.MangleString(k.file), types.MangleString(k.message), int64(counts[k])},
		})
	}
}

// syncBuildGateFacts replaces this turn's /build measurements. A repair
// re-runs the build; the previous rows would otherwise stay. build_state is
// not touched: it is the workspace's raw exit, and a build that fails only
// outside this turn is still a failing build.
//
// The import graph is one `go list -e` of direct .Imports (the same family
// as importer_packages.go), asserted only for packages that list returned.
// Stdlib and third-party imports are dropped: the closure the policy
// computes is the module's own, and a graph that included the standard
// library is tens of thousands of edges. The world model's import facts are
// not this measurement. They can lag a write this turn just made. The list
// runs only for a located failure. No file:line is already attributed, and
// a passing build has nothing to close over.
func (e *Executor) syncBuildGateFacts(ctx context.Context, turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn,
		"turn_build_measured", "turn_build_diagnostic", "turn_pkg_imports",
		"turn_written_package", "turn_build_graph_unknown")
	// The file rule joins turn_written. Repair may have created a file
	// since the round that first asserted the write set.
	e.assertTurnWrites(turn, result)
	var outcome types.MangleAtom
	switch result.BuildCheck.Verdict() {
	case VerifyPassed:
		outcome = "/passing"
	case VerifyFailed:
		outcome = "/failing"
	default:
		return
	}
	e.assertTurnFact(types.Fact{
		Predicate: "turn_build_measured",
		Args:      []any{turn, outcome},
	})
	if outcome == "/passing" {
		return
	}
	workspace := e.workspaceForVerification()
	asserted := 0
	for _, diag := range result.BuildCheck.locatedDiagnostics(workspace) {
		if diag.File == "" {
			continue
		}
		// The join is string equality. The compiler's spelling and the
		// write set's spelling are the same path; on Windows they can
		// differ by case. Asserting the write set's spelling is the
		// measurement, not the attribution.
		file := spellLikeWriteSet(diag.File, result.WrittenPaths)
		e.assertTurnFact(types.Fact{
			Predicate: "turn_build_diagnostic",
			Args:      []any{turn, types.MangleString(file), types.MangleString(diag.Package)},
		})
		asserted++
	}
	if asserted == 0 {
		// Unlocatable. The policy attributes it; there is no graph to ask for.
		return
	}
	edges, dirs, unknown := e.moduleImportGraph(ctx, workspace)
	if unknown {
		// The list failed. Asserting no edges would call an importer break
		// foreign. The policy attributes a missing graph (fail closed).
		e.assertTurnFact(types.Fact{Predicate: "turn_build_graph_unknown", Args: []any{turn}})
		return
	}
	for _, pkg := range packagesForWrittenGo(result.WrittenPaths, dirs) {
		e.assertTurnFact(types.Fact{
			Predicate: "turn_written_package",
			Args:      []any{turn, types.MangleString(pkg)},
		})
	}
	for _, edge := range edges {
		e.assertTurnFact(types.Fact{
			Predicate: "turn_pkg_imports",
			Args:      []any{turn, types.MangleString(edge[0]), types.MangleString(edge[1])},
		})
	}
}

// spellLikeWriteSet returns path spelled the way the write set spells it
// when the two name the same file, and path otherwise.
func spellLikeWriteSet(file string, written []string) string {
	slash := filepath.ToSlash(file)
	for _, w := range written {
		ws := filepath.ToSlash(w)
		if strings.EqualFold(ws, slash) {
			return ws
		}
	}
	return slash
}

// moduleImportGraph is the current module's direct import edges. unknown
// is true when the list did not run or named no package: the caller then
// fail-closes instead of treating a missing graph as "nothing depends on
// the turn".
func (e *Executor) moduleImportGraph(ctx context.Context, workspace string) (edges [][2]string, dirs map[string]string, unknown bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	const format = "{{.ImportPath}}\t{{.Dir}}\t{{join .Imports \" \"}}"
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout,
		"go", []string{"list", "-e", "-f", format, "./..."}, verifyBuildRunner)
	if outcome != VerifyPassed {
		logging.Get(logging.CategorySession).Warn("build attribution: import graph unavailable (%s%s)", outcome, suffixed(reason))
		return nil, nil, true
	}
	root := goWorkspace(workspace)
	type row struct {
		dir     string
		imports []string
	}
	parsed := map[string]row{}
	for _, line := range strings.Split(string(out), "\n") {
		// TrimSpace would delete the trailing tab that marks an empty
		// import list. A package with no imports would then vanish, and
		// either the graph looks empty (fail closed) or an importer of
		// that package is not recorded as depending on it.
		line = strings.TrimRight(strings.TrimLeft(line, " "), "\r ")
		imp, rest, ok := strings.Cut(line, "\t")
		if !ok || imp == "" {
			continue
		}
		dir, imports, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		parsed[imp] = row{dir: moduleRelDir(root, dir), imports: strings.Fields(imports)}
	}
	if len(parsed) == 0 {
		logging.Get(logging.CategorySession).Warn("build attribution: go list returned no packages; the failure is charged to the turn")
		return nil, nil, true
	}
	dirs = make(map[string]string, len(parsed))
	for imp, r := range parsed {
		dirs[imp] = r.dir
	}
	// An import is kept only when both ends were listed. A broken
	// third-party dependency is foreign: this turn did not write it, and
	// nothing it wrote is that package's source.
	for imp, r := range parsed {
		for _, imported := range r.imports {
			if imported == imp {
				continue
			}
			if _, listed := parsed[imported]; !listed {
				continue
			}
			edges = append(edges, [2]string{imp, imported})
		}
	}
	return edges, dirs, false
}

func moduleRelDir(root, dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == "." {
		return "."
	}
	if root != "" {
		if rel, err := filepath.Rel(root, dir); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
			rel = filepath.ToSlash(rel)
			if rel == "." || rel == "" {
				return "."
			}
			return rel
		}
	}
	return filepath.ToSlash(dir)
}

// packagesForWrittenGo names the import path of each .go file the turn
// wrote, by the directory go list reported for that package. A package is
// one directory; the match is that directory, not the package clause.
func packagesForWrittenGo(written []string, dirs map[string]string) []string {
	byDir := make(map[string]string, len(dirs))
	for imp, dir := range dirs {
		byDir[dir] = imp
	}
	var pkgs []string
	seen := map[string]bool{}
	for _, writtenPath := range written {
		if !strings.EqualFold(filepath.Ext(writtenPath), ".go") {
			continue
		}
		dir := path.Dir(filepath.ToSlash(writtenPath))
		if dir == "" || dir == "/" {
			dir = "."
		}
		imp, ok := byDir[dir]
		if !ok {
			for d, p := range byDir {
				if strings.EqualFold(d, dir) {
					imp, ok = p, true
					break
				}
			}
		}
		if ok && !seen[imp] {
			seen[imp] = true
			pkgs = append(pkgs, imp)
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

// buildGateRed is the repair loop's question: does policy charge this turn
// with a build failure? With no kernel there is nothing to ask, and the raw
// exit is what the loop has. A mock kernel derives nothing; neither a
// verdict nor a foreign atom is then present, and that too is charged
// (fail closed) so a real breakage is not skipped.
func (e *Executor) buildGateRed(ctx context.Context, turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil || result.BuildCheck.Verdict() != VerifyFailed {
		return false
	}
	if e == nil || e.kernel == nil {
		return true
	}
	e.syncBuildGateFacts(ctx, turn, result)
	_, fail := e.derivedGate(turn, "/build")
	if fail {
		return true
	}
	if e.hasBuildFailureForeign(turn) {
		result.BuildForeignPackages = e.foreignBuildPackages(turn)
		return false
	}
	return true
}

func (e *Executor) hasBuildFailureForeign(turn types.MangleAtom) bool {
	rows, err := e.turnRows("turn_build_failure_foreign", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("gate facts: query turn_build_failure_foreign: %v", err)
		return false
	}
	return len(rows) > 0
}

func (e *Executor) foreignBuildPackages(turn types.MangleAtom) []string {
	if e == nil || e.kernel == nil || turn == "" {
		return nil
	}
	rows, err := e.turnRows("turn_build_failure_foreign", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("gate facts: query turn_build_failure_foreign: %v", err)
		return nil
	}
	seen := map[string]struct{}{}
	var pkgs []string
	for _, row := range rows {
		if len(row.Args) < 2 {
			continue
		}
		pkg := types.ExtractString(row.Args[1])
		if pkg == "" {
			continue
		}
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	return pkgs
}

// derivedGate reads the turn_gate atoms policy derived (or the executor
// asserted, for /pinned) for this turn and gate.
func (e *Executor) derivedGate(turn types.MangleAtom, gate string) (pass, fail bool) {
	if e == nil || e.kernel == nil || turn == "" {
		return false, false
	}
	facts, err := e.kernel.Query("turn_gate")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("gate facts: query turn_gate: %v", err)
		return false, false
	}
	for _, fact := range facts {
		// Walk the args. An integer above 1 in a condition is counted as an
		// executive knob (defaults/executive_literals_test.go); this is an
		// arity check, so it peels one argument at a time.
		rest := fact.Args
		if len(rest) == 0 || types.ExtractString(rest[0]) != string(turn) {
			continue
		}
		rest = rest[1:]
		if len(rest) == 0 || types.ExtractString(rest[0]) != gate {
			continue
		}
		rest = rest[1:]
		if len(rest) == 0 {
			continue
		}
		switch types.ExtractString(rest[0]) {
		case "/passing":
			pass = true
		case "/failing":
			fail = true
		}
	}
	return pass, fail
}

// testGateRed is the repair loop's question: does policy charge this turn
// with a test failure? With no kernel there is nothing to ask, and the raw
// suite exit is what the loop has (suiteExit: the importer run when it
// failed or did not finish, otherwise the turn's own). That path does not
// record a verdict, and it does not attribute: an importer failure that
// predates the turn is still a failing suite until policy can say otherwise.
func (e *Executor) testGateRed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return suiteExit(result) == VerifyFailed
	}
	e.syncTestGateFacts(turn, result)
	_, fail := e.derivedGate(turn, "/test")
	return fail
}

// testGatePassed is the repair recheck's question, the other side of
// testGateRed. A failing suite whose every failure predates the turn is
// passed here and still a failing suite on the raw exit.
func (e *Executor) testGatePassed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return suiteExit(result) == VerifyPassed
	}
	e.syncTestGateFacts(turn, result)
	pass, _ := e.derivedGate(turn, "/test")
	return pass
}

func (e *Executor) vetGateRed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return result.VetCheck.Verdict() == VerifyFailed
	}
	e.syncVetGateFacts(turn, result)
	_, fail := e.derivedGate(turn, "/vet")
	return fail
}

func (e *Executor) vetGatePassed(turn types.MangleAtom, result *ExecutionResult) bool {
	if result == nil {
		return false
	}
	if e == nil || e.kernel == nil {
		return result.VetCheck.Verdict() == VerifyPassed
	}
	e.syncVetGateFacts(turn, result)
	pass, _ := e.derivedGate(turn, "/vet")
	return pass
}

// syncRemovedTestGateFacts replaces this turn's /test_retention measurements
// with the listing as it stands now: the marker, and one row per test the
// turn removed and has not put back. A repair round restores tests; the
// previous rows would otherwise stay red after the restoration.
func (e *Executor) syncRemovedTestGateFacts(turn types.MangleAtom, removed []string) {
	if e == nil || e.kernel == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn, "turn_removed_test_ran", "turn_removed_test")
	e.assertTurnFact(types.Fact{Predicate: "turn_removed_test_ran", Args: []any{turn}})
	for _, name := range removed {
		if name == "" {
			continue
		}
		e.assertTurnFact(types.Fact{
			Predicate: "turn_removed_test",
			Args:      []any{turn, types.MangleString(name)},
		})
	}
}

// removedTestsGateRed is the /removed_tests round's question and the
// closure's: does policy charge this turn with a removed test? With no
// kernel there is nothing to ask, and a verdict that cannot be read
// discharges nothing, so both fall back to the raw measurement: names still
// missing fail, as before. That fallback is not a second verdict; it is what
// the round does when the executive is silent.
func (e *Executor) removedTestsGateRed(turn types.MangleAtom, removed []string) bool {
	if e == nil || e.kernel == nil {
		return len(removed) > 0
	}
	e.syncRemovedTestGateFacts(turn, removed)
	pass, fail := e.derivedGate(turn, "/test_retention")
	if !pass && !fail {
		return len(removed) > 0
	}
	return fail
}

// syncCriticGateFacts replaces this turn's critic triage inputs with the
// on-change findings as parsed now: one row per finding with its index and
// its severity atom. The review runs once per turn; the retract keeps a
// second run from triaging the first run's findings alongside its own.
func (e *Executor) syncCriticGateFacts(turn types.MangleAtom, findings []CriticFinding) {
	if e == nil || e.kernel == nil || turn == "" {
		return
	}
	e.retractTurnPredicates(turn, "turn_critic_finding")
	for i, finding := range findings {
		e.assertTurnFact(types.Fact{
			Predicate: "turn_critic_finding",
			Args:      []any{turn, int64(i), criticSeverityAtom(finding.Severity)},
		})
	}
}

// criticSeverityAtom is the severity word as a severity_rank atom: high,
// medium and low case-insensitively, anything else /unknown, which ranks
// lowest. The parser admits only the first three, so /unknown is a
// hand-built finding's; it must not triage as if it were low.
func criticSeverityAtom(sev string) types.MangleAtom {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "high":
		return types.MangleAtom("/high")
	case "medium":
		return types.MangleAtom("/medium")
	case "low":
		return types.MangleAtom("/low")
	default:
		return types.MangleAtom("/unknown")
	}
}

// actionableCriticFindings is the critic round's triage: the on-change
// findings policy charges the turn with, in the reviewer's order, or nil.
// Go parses (parseCriticFindings) and locates (findingsOnChange); whether a
// finding is worth an uplift round is turn_needs_uplift's, and the findings
// it names come back as turn_critic_actionable. With no kernel there is no
// triage: the review is recorded by the caller and no round runs, which is
// what "advisory" means.
func (e *Executor) actionableCriticFindings(turn types.MangleAtom, onChange []CriticFinding) []CriticFinding {
	if e == nil || e.kernel == nil || turn == "" || len(onChange) == 0 {
		return nil
	}
	e.syncCriticGateFacts(turn, onChange)
	if !e.criticNeedsUplift(turn) {
		return nil
	}
	actionable := e.criticActionableSet(turn)
	var out []CriticFinding
	for i, finding := range onChange {
		if actionable[int64(i)] {
			out = append(out, finding)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// criticNeedsUplift is the round's question: does policy charge this turn
// with an actionable finding? An unreadable verdict charges nothing; the
// review stays recorded and the turn continues without the round.
func (e *Executor) criticNeedsUplift(turn types.MangleAtom) bool {
	facts, err := e.kernel.Query("turn_needs_uplift")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("gate facts: query turn_needs_uplift: %v", err)
		return false
	}
	for _, fact := range facts {
		rest := fact.Args
		if len(rest) == 0 || types.ExtractString(rest[0]) != string(turn) {
			continue
		}
		return true
	}
	return false
}

// criticActionableSet is the findings policy named, by index.
func (e *Executor) criticActionableSet(turn types.MangleAtom) map[int64]bool {
	out := map[int64]bool{}
	facts, err := e.kernel.Query("turn_critic_actionable")
	if err != nil {
		logging.Get(logging.CategorySession).Warn("gate facts: query turn_critic_actionable: %v", err)
		return out
	}
	for _, fact := range facts {
		rest := fact.Args
		if len(rest) == 0 || types.ExtractString(rest[0]) != string(turn) {
			continue
		}
		rest = rest[1:]
		if len(rest) == 0 {
			continue
		}
		if idx, ok := types.ExtractInt64(rest[0]); ok {
			out[idx] = true
		}
	}
	return out
}

// testRunGatePassed is the /test_run repair's question. The receipts are
// already asserted; this only reads what policy derived.
func (e *Executor) testRunGatePassed(turn types.MangleAtom) bool {
	if e == nil || e.kernel == nil {
		return false
	}
	pass, _ := e.derivedGate(turn, "/test_run")
	return pass
}

// noteWrite records one successful write on the turn's seq clock.
func (e *Executor) noteWrite(result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	result.gateSeq++
	result.writeSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_write_seq",
		Args:      []any{result.turnAtom(), int64(result.gateSeq)},
	})
}

// noteTestRun records one test process on the same clock.
func (e *Executor) noteTestRun(result *ExecutionResult, exitCode int) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	result.gateSeq++
	result.testRunSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_test_run",
		Args:      []any{result.turnAtom(), int64(result.gateSeq), int64(exitCode)},
	})
}

// noteCheckRun records one run_check on the same clock.
func (e *Executor) noteCheckRun(result *ExecutionResult, exitCode int) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	result.gateSeq++
	e.assertTurnFact(types.Fact{
		Predicate: "turn_check_run",
		Args:      []any{result.turnAtom(), int64(result.gateSeq), int64(exitCode)},
	})
}

// ensureWriteSequenced asserts one write at the start of the clock when the
// turn already wrote and nothing has been sequenced yet. Forcing rounds and
// the closure see writes that were recorded on the result rather than through
// the tool loop; those writes are one moment, before any later run. A clock
// the tool loop already advanced is left alone: that write is already a fact,
// or a run was recorded first and a write invented after it would hide the run.
func (e *Executor) ensureWriteSequenced(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || result.writeSequenced || result.SuccessfulWriteTools == 0 {
		return
	}
	if result.gateSeq > 0 {
		result.writeSequenced = true
		return
	}
	result.gateSeq++
	result.writeSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_write_seq",
		Args:      []any{turn, int64(result.gateSeq)},
	})
}

// ensurePointerTestRun asserts the test run a caller recorded on the result
// when the tool loop did not. It is after the writes already sequenced.
func (e *Executor) ensurePointerTestRun(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || result.testRunSequenced || result.TestRunSinceLastWrite == nil {
		return
	}
	result.gateSeq++
	result.testRunSequenced = true
	e.assertTurnFact(types.Fact{
		Predicate: "turn_test_run",
		Args:      []any{turn, int64(result.gateSeq), int64(result.TestRunSinceLastWrite.ExitCode)},
	})
}
