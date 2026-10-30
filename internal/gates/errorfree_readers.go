package gates

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/lintreport"
)

// readGoTest reads `go test -json`: a failing test is a finding (File its
// package, Check "test", Message its name), and a package that failed with no
// failing test -- a build failure, a panic before any test, a TestMain exit --
// is one finding for the package (Check "package"). Compile errors in the
// build output become compile findings at their positions. rest is the output
// of every failing test and package, whole.
func readGoTest(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
	type event struct {
		Action, Package, Test, Output string
	}
	var findings []lintreport.Finding
	failedTests := map[string]bool{}
	output := map[string]*strings.Builder{}
	var buildOut, rest strings.Builder
	key := func(pkg, test string) string { return pkg + "\x00" + test }
	var pkgFailed []string
	sc := bufio.NewScanner(strings.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), len(stdout)+1)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(strings.TrimSpace(line), "{") {
			buildOut.WriteString(line + "\n")
			continue
		}
		var e event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, "", fmt.Errorf("go test -json line is not JSON: %w", err)
		}
		switch e.Action {
		case "output":
			k := key(e.Package, e.Test)
			if output[k] == nil {
				output[k] = &strings.Builder{}
			}
			output[k].WriteString(e.Output)
		case "build-output":
			buildOut.WriteString(e.Output)
		case "fail":
			if e.Test != "" {
				failedTests[e.Package] = true
				findings = append(findings, lintreport.Finding{File: e.Package, Check: "test", Message: e.Test})
				if b := output[key(e.Package, e.Test)]; b != nil {
					rest.WriteString(b.String())
				}
			} else if e.Package != "" {
				pkgFailed = append(pkgFailed, e.Package)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	compile := lintreport.CompileFindings(buildOut.String()+stderr, dir)
	findings = append(findings, compile...)
	for _, pkg := range pkgFailed {
		if failedTests[pkg] {
			continue
		}
		if len(compile) == 0 {
			findings = append(findings, lintreport.Finding{File: pkg, Check: "package", Message: "the package failed with no failing test"})
		}
		if b := output[key(pkg, "")]; b != nil {
			rest.WriteString(b.String())
		}
	}
	rest.WriteString(buildOut.String())
	return sortedUnique(findings), strings.TrimSpace(rest.String() + "\n" + stderr), nil
}

// readGovulncheck reads `govulncheck -json`: a stream of messages, of which
// each "finding" whose innermost trace frame names a function is a
// vulnerability the code reaches. One finding per advisory: Check its ID,
// File the call site in the module (the outermost frame with a position).
func readGovulncheck(stdout, stderr, dir string) ([]lintreport.Finding, string, error) {
	type frame struct {
		Module, Version, Package, Function, Receiver string
		Position                                     *struct {
			Filename     string
			Line, Column int
		}
	}
	type message struct {
		Finding *struct {
			OSV   string  `json:"osv"`
			Trace []frame `json:"trace"`
		} `json:"finding"`
	}
	dec := json.NewDecoder(strings.NewReader(stdout))
	byOSV := map[string]lintreport.Finding{}
	for {
		var m message
		if err := dec.Decode(&m); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, "", fmt.Errorf("govulncheck -json output is not JSON: %w", err)
		}
		if m.Finding == nil || len(m.Finding.Trace) == 0 || m.Finding.Trace[0].Function == "" {
			continue
		}
		vuln := m.Finding.Trace[0]
		f := lintreport.Finding{Check: m.Finding.OSV,
			Message: fmt.Sprintf("%s@%s: %s.%s", vuln.Module, vuln.Version, vuln.Package, vuln.Function)}
		for i := len(m.Finding.Trace) - 1; i >= 0; i-- {
			if p := m.Finding.Trace[i].Position; p != nil && p.Filename != "" {
				f.File, f.Line, f.Column = lintreport.Path(p.Filename, dir), p.Line, p.Column
				break
			}
		}
		if _, seen := byOSV[f.Check]; !seen {
			byOSV[f.Check] = f
		}
	}
	findings := make([]lintreport.Finding, 0, len(byOSV))
	for _, f := range byOSV {
		findings = append(findings, f)
	}
	return sortedUnique(findings), strings.TrimSpace(stderr), nil
}

// gofmtGate is G10: every Go file of the module formatted as go/format would
// write it -- the check TestRepository_EveryGoFileIsGofmtClean makes. It walks
// the files the go tool sees: directories starting with "." or "_", testdata
// and vendor are skipped, as `go list ./...` skips them, so a nested checkout
// under .claude/ is not measured as this module.
type gofmtGate struct{}

func (gofmtGate) Gate() string  { return "gofmt" }
func (gofmtGate) Label() string { return "G10" }

func (gofmtGate) Measure(ctx context.Context, root string) Measurement {
	m := Measurement{Reason: "go/format over the module's Go files"}
	start := time.Now()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		rel := lintreport.Path(path, root)
		formatted, ferr := format.Source(data)
		switch {
		case ferr != nil:
			m.Findings = append(m.Findings, lintreport.Finding{File: rel, Check: "gofmt", Message: "does not parse: " + ferr.Error()})
		case string(formatted) != string(data):
			m.Findings = append(m.Findings, lintreport.Finding{File: rel, Check: "gofmt", Message: "not gofmt-formatted"})
		}
		return nil
	})
	m.Duration = time.Since(start)
	switch {
	case err != nil:
		m.Status, m.Reason = MeasureUnverified, "walk the module: "+err.Error()
	case len(m.Findings) > 0:
		m.Status = MeasureFail
	default:
		m.Status = MeasurePass
	}
	lintreport.Sort(m.Findings)
	return m
}
