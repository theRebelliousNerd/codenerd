package defaults

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pinned Mangle fork implements <, <=, >, >= over int64 ONLY:
// builtin.go routes every comparison through getNumberValues -> getNumberValue,
// which rejects anything whose Type != ast.NumberType. getFloatValue exists but
// has no caller, so there is no float comparison in the language at all.
//
// The failure mode is not "the rule returns false". EvalStratifiedProgram
// propagates the error, so RealKernel.evaluate() bails at kernel_eval.go:299
// before `k.store = baseStore` — the WHOLE kernel derives nothing, and the log
// line names only the offending value ("value 110 (4) is not a number"), never
// the predicate or rule. Observed live at ~4 aborts per 2s during a dogfood run.
//
// Therefore a float literal on either side of a comparison is a latent
// whole-kernel outage, armed the moment a matching fact is asserted.
var floatComparisonRE = regexp.MustCompile(`(?:[<>]=?\s*-?\d+\.\d+)|(?:-?\d+\.\d+\s*[<>]=?)`)

// stripMangleComment removes a trailing `#` comment so prose like
// "# confidence > 0.5 boundary" is not mistaken for a rule.
func stripMangleComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i]
	}
	return line
}

func collectMangleFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".mg") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(out) == 0 {
		t.Fatalf("no .mg files found under %s — test is not actually checking anything", root)
	}
	return out
}

func TestCorpus_NoFloatLiteralInComparison(t *testing.T) {
	files := collectMangleFiles(t, ".")

	type offence struct {
		file string
		line int
		text string
	}
	var found []offence

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for i, raw := range strings.Split(string(data), "\n") {
			line := stripMangleComment(raw)
			if !floatComparisonRE.MatchString(line) {
				continue
			}
			found = append(found, offence{file: path, line: i + 1, text: strings.TrimSpace(raw)})
		}
	}

	if len(found) > 0 {
		t.Errorf("%d comparison(s) use a float literal; each one aborts the entire "+
			"fixpoint (not just its own rule) as soon as a matching fact exists.\n"+
			"Rescale to integer percent to match the /number Decls:", len(found))
		for _, o := range found {
			t.Errorf("  %s:%d  %s", o.file, o.line, o.text)
		}
	}
}

// Guard the guard: the regex must catch both operand positions and must not
// fire on arithmetic or on float literals that never touch a comparison.
func TestFloatComparisonRegex(t *testing.T) {
	shouldMatch := []string{
		"    AvgQuality < 50.0.",
		"    Confidence > 0.7.",
		"    Conf1 >= 0.5,",
		"    0.5 < Score,",
		"    Coverage<0.3.",
	}
	for _, s := range shouldMatch {
		if !floatComparisonRE.MatchString(stripMangleComment(s)) {
			t.Errorf("expected a match for %q", s)
		}
	}

	shouldNotMatch := []string{
		"    Score >= 80.",                              // integer comparison — fine
		"    |> let Avg = fn:float:sum(Xs).",            // float arithmetic — supported
		"# Replaces the hardcoded Confidence >= 0.5 check", // prose in a comment
		"layer_priority(/scaffold, 10).",                // a plain fact
		"    Dist > 30.",                                // integer threshold
	}
	for _, s := range shouldNotMatch {
		if floatComparisonRE.MatchString(stripMangleComment(s)) {
			t.Errorf("false positive on %q", s)
		}
	}
}
