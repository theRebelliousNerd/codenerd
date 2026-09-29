package testfacts

import (
	"fmt"
	"strconv"
	"strings"
)

// Summary renders a compact deterministic brief for a model: build
// failures first, then each failing test with its first failure's
// file:line and message, then repeats with their counts, then a one-line
// tally. A failing test with no failure lines of its own (a parent that
// failed only through a child) has no FAIL line but still counts in the
// tally. Its size follows the content -- no caps -- and it never inlines
// a whole test body; Output recalls any one test's full output.
func (r *Result) Summary() string {
	if r == nil {
		return "empty result: no events parsed\n"
	}
	var b strings.Builder
	for _, bf := range r.BuildFailures {
		fmt.Fprintf(&b, "build-failed %s %s:%d:%d: %s\n",
			bf.Package, bf.File, bf.Line, bf.Column, bf.Message)
	}
	for _, f := range firstFailures(r.Failures) {
		switch {
		case f.Test == "" && f.File == "":
			fmt.Fprintf(&b, "FAIL %s: %s\n", f.Package, f.Message)
		case f.Test == "":
			fmt.Fprintf(&b, "FAIL %s %s:%d: %s\n", f.Package, f.File, f.Line, f.Message)
		case f.File == "":
			fmt.Fprintf(&b, "FAIL %s %s: %s\n", f.Package, f.Test, f.Message)
		default:
			fmt.Fprintf(&b, "FAIL %s %s %s:%d: %s\n",
				f.Package, f.Test, f.File, f.Line, f.Message)
		}
	}
	for _, rp := range r.Repeats {
		fmt.Fprintf(&b, "repeated %dx: %s\n", rp.Count, rp.Line)
	}
	b.WriteString(r.tally() + "\n")
	return b.String()
}

// tally folds the run into one line: package and test verdict counts
// (nonzero categories only, fixed order), plus the raw-line count when a
// runner died before test2json started or stderr mixed in.
func (r *Result) tally() string {
	var parts []string
	if len(r.Packages) > 0 {
		counts := make(map[Status]int)
		tests, tcounts := 0, make(map[Status]int)
		for _, p := range r.Packages {
			counts[p.Status]++
			for _, t := range p.Tests {
				tests++
				tcounts[t.Status]++
			}
		}
		parts = append(parts, "packages: "+strconv.Itoa(len(r.Packages))+
			" ("+joinCounts(counts)+")")
		parts = append(parts, "tests: "+strconv.Itoa(tests)+
			" ("+joinCounts(tcounts)+")")
	}
	if len(r.Raw) > 0 {
		parts = append(parts, "raw lines: "+strconv.Itoa(len(r.Raw)))
	}
	if len(parts) == 0 {
		return "empty result: no events parsed"
	}
	return strings.Join(parts, "; ")
}

// tallyOrder fixes the verdict order inside the tally.
var tallyOrder = []Status{
	StatusPass, StatusFail, StatusSkip,
	StatusBuildFailed, StatusNoTestFiles, StatusUnknown,
}

// joinCounts renders nonzero verdict counts in tally order.
func joinCounts(counts map[Status]int) string {
	var parts []string
	for _, s := range tallyOrder {
		if counts[s] > 0 {
			parts = append(parts, string(s)+" "+strconv.Itoa(counts[s]))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// Output returns one test's full output for recall, lines joined as they
// arrived. An empty test name recalls the package's own output instead
// (package-level floods need recall too); unknown names yield "".
func (r *Result) Output(pkg, test string) string {
	if r == nil {
		return ""
	}
	for _, p := range r.Packages {
		if p.Name != pkg {
			continue
		}
		if test == "" {
			return joinOutput(p.Output)
		}
		for _, t := range p.Tests {
			if t.Name == test {
				return joinOutput(t.Output)
			}
		}
		return ""
	}
	return ""
}

// joinOutput rejoins lines into the byte stream they came from.
func joinOutput(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
