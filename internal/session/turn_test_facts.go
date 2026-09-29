package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	working "codenerd/internal/context"
	"codenerd/internal/logging"
	"codenerd/internal/testfacts"
	"codenerd/internal/types"
)

// Per-turn test facts.
//
// testfacts.Facts() is the run: test_case, test_failure_at, test_build_failure,
// test_output_repeat, failing_test. Those predicates are not keyed by turn.
// context_compilation.mg reads failing_test as "some test is failing", and
// RetractFact removes every row whose first argument matches, so asserting
// them for this turn and retracting them at cleanup would either leak into
// the next turn or delete another producer's rows. A link fact
// (turn_ran_tests(Turn) joined to the unscoped rows) does not help: the join
// still sees every test_failure_at in the kernel, including a concurrent
// turn's. The verdict joins on Turn.
//
// The facts this file asserts are the same rows with the turn prepended
// (turn_test_case, turn_test_failure_at, ...). cleanupTurnFacts retracts them
// with the rest of the turn, and a second turn's rows survive because their
// first argument is a different turn. The source is TestVerification.Result;
// the output text is not parsed again.

// assertTurnTestFacts asserts the gate's parsed run for this turn. A gate
// that did not run asserts nothing, even if a Result was left on the struct:
// that Result is not this turn's measurement.
func (e *Executor) assertTurnTestFacts(turn types.MangleAtom, result *ExecutionResult) {
	if e == nil || e.kernel == nil || result == nil || !result.TestCheck.Ran || result.TestCheck.Result == nil {
		return
	}
	for _, fact := range turnScopedTestFacts(turn, result.TestCheck.Result) {
		e.assertTurnFact(fact)
	}
}

// turnScopedTestFacts is Result.Facts() with Turn as the first argument and
// the predicate renamed to the turn-scoped Decl. The remaining arguments are
// the ones testfacts already built, including the canonical file on a failure.
//
// Each name is a full literal in the Fact composite. The undeclared-assert
// scan takes the quoted word after Predicate, so a prefix concatenated with
// the testfacts name is recorded as that prefix, and no Decl has it.
func turnScopedTestFacts(turn types.MangleAtom, res *testfacts.Result) []types.Fact {
	if res == nil {
		return nil
	}
	src := res.Facts()
	out := make([]types.Fact, 0, len(src))
	for _, f := range src {
		args := make([]any, 1+len(f.Args))
		args[0] = turn
		copy(args[1:], f.Args)
		scoped, ok := scopeTurnTestFact(f.Predicate, args)
		if !ok {
			logging.Get(logging.CategorySession).Warn("test fact %s has no turn-scoped predicate; it was not asserted", f.Predicate)
			continue
		}
		out = append(out, scoped)
	}
	return out
}

// scopeTurnTestFact is the turn-scoped Decl for one testfacts predicate.
// An unknown name is not given a turn_ prefix: that would be a fact no Decl
// names.
func scopeTurnTestFact(predicate string, args []any) (types.Fact, bool) {
	switch predicate {
	case testfacts.PredTestCase:
		return types.Fact{Predicate: "turn_test_case", Args: args}, true
	case testfacts.PredTestFailureAt:
		return types.Fact{Predicate: "turn_test_failure_at", Args: args}, true
	case testfacts.PredTestBuildFailure:
		return types.Fact{Predicate: "turn_test_build_failure", Args: args}, true
	case testfacts.PredTestOutputRepeat:
		return types.Fact{Predicate: "turn_test_output_repeat", Args: args}, true
	case testfacts.PredFailingTest:
		return types.Fact{Predicate: "turn_failing_test", Args: args}, true
	default:
		return types.Fact{}, false
	}
}

// annotateFailingTestRecall stores each failing test's full output in the
// turn's working set and names that record on the test's summary line.
// recall_context reads the working set; there is no other archive. A turn
// with no working loop keeps the summary the Result rendered — the output
// is still on Result, and inventing a second store would not be
// recall_context.
//
// Save is called directly, not through recordWorkingResult. That hook is the
// effect boundary of a tool call: it moves the loop's focus to the file the
// call named, and a test log is not a file the turn is editing.
func annotateFailingTestRecall(ctx context.Context, v *TestVerification) {
	if v == nil || v.Result == nil {
		return
	}
	loop := activeWorkingLoop(ctx)
	if loop == nil || loop.set == nil {
		return
	}
	text := v.Output
	if strings.TrimSpace(text) == "" {
		text = verificationOutput(v.Result)
	}
	notes, orphans := archiveFailingOutputs(ctx, loop.set, v.Result)
	if len(notes) == 0 && len(orphans) == 0 {
		return
	}
	text = applyRecallNotes(text, notes)
	for _, line := range orphans {
		text = insertRecallLine(text, line)
	}
	v.Output = text
}

// recallNote is one summary line and the working-set id of that test's output.
type recallNote struct {
	line string
	id   string
}

// archiveFailingOutputs saves every failing test body that has text. notes
// are the Result's own FAIL lines (first failure per test). orphans are
// failing tests that printed nothing the failure parser kept — a parent
// that failed only because a child did — so they have no FAIL line to extend.
func archiveFailingOutputs(ctx context.Context, set *working.WorkingSet, res *testfacts.Result) (notes []recallNote, orphans []string) {
	if set == nil || res == nil {
		return nil, nil
	}
	seen := make(map[[2]string]bool)
	for _, f := range res.Failures {
		key := [2]string{f.Package, f.Test}
		if seen[key] {
			continue
		}
		seen[key] = true
		body := res.Output(f.Package, f.Test)
		if body == "" {
			continue
		}
		id, err := saveFailingOutput(ctx, set, f.Package, f.Test, f.File, body)
		if err != nil {
			logging.Get(logging.CategorySession).Warn("failing test output for %s %s was not archived: %v", f.Package, f.Test, err)
			continue
		}
		notes = append(notes, recallNote{line: failSummaryLine(f), id: id})
	}
	for _, p := range res.Packages {
		for _, test := range p.Tests {
			if test == nil || test.Status != testfacts.StatusFail || test.Name == "" {
				continue
			}
			key := [2]string{p.Name, test.Name}
			if seen[key] {
				continue
			}
			seen[key] = true
			body := res.Output(p.Name, test.Name)
			if body == "" {
				continue
			}
			id, err := saveFailingOutput(ctx, set, p.Name, test.Name, "", body)
			if err != nil {
				logging.Get(logging.CategorySession).Warn("failing test output for %s %s was not archived: %v", p.Name, test.Name, err)
				continue
			}
			orphans = append(orphans, fmt.Sprintf("FAIL %s %s %s", p.Name, test.Name, failingOutputPhrase(id)))
		}
	}
	return notes, orphans
}

// saveFailingOutput writes one test's full output and returns the id
// recall_context takes. The id is the body's hash, so a second save of the
// same output keeps the record (the store ignores a conflicting id) and the
// summary names a row whose body is this output.
func saveFailingOutput(ctx context.Context, set *working.WorkingSet, pkg, test, file, body string) (string, error) {
	sum := sha256.Sum256([]byte(pkg + "\x00" + test + "\x00" + body))
	id := hex.EncodeToString(sum[:])
	entity := file
	if entity == "" {
		entity = pkg
	}
	if entity == "" {
		entity = "."
	}
	kindSum := sha256.Sum256([]byte(pkg + "\x00" + test))
	rec := working.WorkingRecord{
		ID:       id,
		Entity:   entity,
		Revision: set.Revision(entity),
		Kind:     "go_test/" + hex.EncodeToString(kindSum[:8]),
		Step:     time.Now().UnixNano(),
		Body:     body,
		Failed:   true,
	}
	if err := set.Save(ctx, rec); err != nil {
		return "", err
	}
	// The model is handed the short per-task ordinal, not the storage id:
	// a 64-hex id was mis-copied on the first live run (the ledger
	// replay in internal/context), and recall now accepts only the ordinal.
	return set.Handle(ctx, id)
}

// failingOutputPhrase is the clause appended to a FAIL line. The id form
// matches the other recall pointers the harness hands the model
// (recall_context id="...").
func failingOutputPhrase(id string) string {
	return fmt.Sprintf("recall_context id=%q returns this test's full output", id)
}

// failSummaryLine is the FAIL line testfacts.Summary renders for one first
// failure. The handle is appended to that line; a different spelling would
// leave the handle off the line the model reads.
func failSummaryLine(f testfacts.Failure) string {
	switch {
	case f.Test == "" && f.File == "":
		return fmt.Sprintf("FAIL %s: %s", f.Package, f.Message)
	case f.Test == "":
		return fmt.Sprintf("FAIL %s %s:%d: %s", f.Package, f.File, f.Line, f.Message)
	case f.File == "":
		return fmt.Sprintf("FAIL %s %s: %s", f.Package, f.Test, f.Message)
	default:
		return fmt.Sprintf("FAIL %s %s %s:%d: %s", f.Package, f.Test, f.File, f.Line, f.Message)
	}
}

// applyRecallNotes appends each saved id to its FAIL line. Summary prints the
// failure message raw, and a message can hold newlines (a panic stack, a
// continuation line the failure parser kept), so the entry is often more
// than one physical line. The handle goes on the first of those: that is the
// line that names the test. A line that is not in the summary is logged and
// left: the body is stored, and the model was not told the id.
func applyRecallNotes(text string, notes []recallNote) string {
	if len(notes) == 0 {
		return text
	}
	parts := strings.Split(text, "\n")
	for i, part := range parts {
		for n := range notes {
			if notes[n].id == "" || !failLineMatches(part, notes[n].line) {
				continue
			}
			parts[i] = part + " " + failingOutputPhrase(notes[n].id)
			notes[n].id = ""
			break
		}
	}
	for _, n := range notes {
		if n.id == "" {
			continue
		}
		logging.Get(logging.CategorySession).Warn("failing-test summary line was not found; recall id %s is not on it: %s", n.id, n.line)
	}
	return strings.Join(parts, "\n")
}

// failLineMatches reports whether a physical summary line is the start of
// the FAIL entry Summary rendered for one failure. A single-line message
// matches the whole entry. A message that contains newlines matches only
// its first physical line, which still begins with the FAIL prefix.
func failLineMatches(part, entry string) bool {
	if part == entry {
		return true
	}
	first, rest, ok := strings.Cut(entry, "\n")
	return ok && rest != "" && part == first
}

// insertRecallLine puts a line for a failing test that had no FAIL line of
// its own just before the tally, which Summary writes last.
func insertRecallLine(text, line string) string {
	if text == "" {
		return line
	}
	i := strings.LastIndex(text, "\n")
	if i < 0 {
		return line + "\n" + text
	}
	return text[:i+1] + line + "\n" + text[i+1:]
}
