package testfacts

import (
	"bufio"
	"io"
	"strings"
)

// Parse reads a `go test -json` stream that the go command produced with
// dir as its working directory. dir is the workspace root the command ran
// in: failure files are canonicalised against it, so a basename, a
// `./` diagnostic, and an absolute frame for one file become one
// workspace-relative path. A file outside dir keeps its absolute slash
// form. An empty dir leaves relative paths cleaned but unanchored.
//
// Output chunks are accumulated per test, package, and build target and
// split into lines only after the whole stream is read, because one
// Output field can end mid-line. The only error Parse returns is a read
// failure, with whatever was parsed so far alongside it; unparseable
// content degrades to Raw, never to an error that loses it.
func Parse(dir string, r io.Reader) (*Result, error) {
	a := newAccumulator(dir)
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			a.addLine(strings.TrimSuffix(line, "\n"))
		}
		if err != nil {
			if err == io.EOF {
				return a.result(), nil
			}
			return a.result(), err
		}
	}
}

// pkgAcc gathers one package's events; testAcc gathers one test's.
type testAcc struct {
	status  Status
	elapsed float64
	verdict bool
	out     strings.Builder
}

type pkgAcc struct {
	name      string
	status    Status
	elapsed   float64
	verdict   bool
	failedBld bool
	tests     map[string]*testAcc
	out       strings.Builder
}

type accumulator struct {
	loc         locator
	pkgs        map[string]*pkgAcc
	buildOut    map[string]*strings.Builder
	buildFailed map[string]bool
	raw         []string
}

func newAccumulator(dir string) *accumulator {
	return &accumulator{
		loc:         newLocator(dir),
		pkgs:        make(map[string]*pkgAcc),
		buildOut:    make(map[string]*strings.Builder),
		buildFailed: make(map[string]bool),
	}
}

func (a *accumulator) pkg(name string) *pkgAcc {
	p, ok := a.pkgs[name]
	if !ok {
		p = &pkgAcc{name: name, status: StatusUnknown, tests: make(map[string]*testAcc)}
		a.pkgs[name] = p
	}
	return p
}

func (a *accumulator) test(pkgName, testName string) *testAcc {
	p := a.pkg(pkgName)
	t, ok := p.tests[testName]
	if !ok {
		t = &testAcc{status: StatusUnknown}
		p.tests[testName] = t
	}
	return t
}

// addLine routes one stream line to its accumulator.
func (a *accumulator) addLine(line string) {
	ev, ok := decodeLine(line)
	if !ok {
		a.raw = append(a.raw, strings.TrimSuffix(line, "\r"))
		return
	}
	switch ev.Action {
	case actionBuildOut:
		if _, found := a.buildOut[ev.ImportPath]; !found {
			a.buildOut[ev.ImportPath] = &strings.Builder{}
		}
		a.buildOut[ev.ImportPath].WriteString(ev.Output)
	case actionBuildFail:
		a.buildFailed[ev.ImportPath] = true
	case actionOutput:
		a.addOutput(ev)
	case actionRun, actionPause, actionCont:
		if ev.Test != "" {
			a.test(ev.Package, ev.Test)
		} else if ev.Package != "" {
			a.pkg(ev.Package)
		} else {
			a.raw = append(a.raw, line)
		}
	case actionPass, actionBench, actionFail, actionSkip:
		a.addVerdict(ev)
	case actionStart:
		if ev.Package != "" {
			a.pkg(ev.Package)
		}
	default:
		// Unknown or empty action: keep any output it carries, using
		// the same routing as output so future actions degrade
		// gracefully instead of dropping bytes.
		if ev.Output != "" {
			a.addOutput(ev)
		}
	}
}

// addOutput routes an output-carrying event to its test, package, or Raw
// when it names neither (JSON output with no home is still output).
func (a *accumulator) addOutput(ev testEvent) {
	switch {
	case ev.Test != "":
		a.test(ev.Package, ev.Test).out.WriteString(ev.Output)
	case ev.Package != "":
		a.pkg(ev.Package).out.WriteString(ev.Output)
	default:
		a.raw = append(a.raw, splitLines(ev.Output)...)
	}
}

// addVerdict records a pass/bench/fail/skip event for a test or package.
// The last verdict wins: `go test -count=2` runs each test twice and both
// runs' output is kept, so the verdict follows the final run.
func (a *accumulator) addVerdict(ev testEvent) {
	if ev.Package == "" && ev.Test == "" {
		// A verdict naming nothing carries no bytes worth keeping;
		// real test2json never emits one.
		return
	}
	st := Status(ev.Action)
	if ev.Action == actionBench {
		st = StatusPass
	}
	if ev.Test != "" {
		t := a.test(ev.Package, ev.Test)
		t.status, t.elapsed, t.verdict = st, ev.Elapsed, true
		return
	}
	p := a.pkg(ev.Package)
	p.status, p.elapsed, p.verdict = st, ev.Elapsed, true
	if ev.Action == actionFail && ev.FailedBuild != "" {
		p.failedBld = true
	}
}

// stripVariant maps a build ImportPath to its package: the test binary
// builds report as "pkg [pkg.test]" (observed 2026-09-28 on Go 1.26.4),
// while plain builds report the bare path.
func stripVariant(importPath string) string {
	if i := strings.Index(importPath, " ["); i >= 0 {
		return importPath[:i]
	}
	return importPath
}
