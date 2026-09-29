package gates

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Finding is one thing a gate run reported wrong.
type Finding struct {
	// ID is stable across runs: the same failure keeps its identity while the
	// code around it moves. It hashes the gate, the target, and (for a
	// diagnostic) the normalised message -- not line numbers, which every
	// edit above the failure changes.
	ID string
	// Gate is the gate's ID.
	Gate string
	// Kind is the gate's kind.
	Kind Kind
	// Node is the node the run was for; "" for a workspace-scoped run.
	Node string
	// Target is what failed: a file, a package directory, or either plus
	// "::" and a test name. UnattributedTarget when the output named no
	// file and no package — that is not the module root (".").
	Target string
	// Message is the failure as reported, first line.
	Message string
	// Signature is Message normalised: numbers, addresses and the workspace
	// path collapsed. Two attempts that end on the same Signature failed the
	// same way.
	Signature string
}

// GoDiagnostic is one file:line the Go toolchain named. Findings collapses
// these into a capped target list; attribution keeps every one, including
// its line, column and the "# pkg" header that was current when it was
// printed. A package-only "FAIL\tpkg [build failed]" row is not a
// diagnostic: it names no file.
type GoDiagnostic struct {
	File    string
	Line    int
	Col     int
	Message string
	Package string
}

// UnattributedTarget is the target of a failure whose output named no file
// and no package. It is not a directory, so a node whose path is "." (the
// module root) must not claim it, and neither must any real package.
const UnattributedTarget = "<unattributed>"

// MaxFindingsPerRun bounds the findings one run yields. A tree that stops
// compiling reports the same break hundreds of times; the first ones are the
// ones worth a turn, and the rest reappear if they survive the fix.
var MaxFindingsPerRun = 50

var (
	// Go test: "--- FAIL: TestName (0.00s)"; subtests nest by indentation.
	goTestFail = regexp.MustCompile(`^\s*--- FAIL: (\S+)`)
	// Go, gcc, mypy, ruff, flake8, eslint -f unix:
	// "path/file.ext:line[:col]: message". The path may open with a Windows
	// drive ("C:\ws\a.py:3:1: ..."), whose colon the path class excludes.
	fileLineMsg = regexp.MustCompile(`^(?:\./)?((?:[A-Za-z]:)?[^\s:()]+\.[A-Za-z0-9]+):(\d+)(?::(\d+))?:?\s+(.+)$`)
	// tsc: "src/a.ts(12,5): error TS2345: message".
	tscError = regexp.MustCompile(`^(\S+?\.[A-Za-z]+)\((\d+),(\d+)\): (?:error|warning) (TS\d+: .+)$`)
	// pytest short summary: "FAILED tests/test_x.py::test_y - AssertionError".
	pytestFailed = regexp.MustCompile(`^(?:FAILED|ERROR) (\S+?)(?: - (.+))?$`)
	// cargo test: "test module::name ... FAILED".
	cargoTestFailed = regexp.MustCompile(`^test (\S+) \.\.\. FAILED$`)
	// go vet prefixes a compile error with the tool name on Windows
	// ("vet.exe: pkg\file.go:3:28: undefined: Missing", go 1.26.4). The
	// same diagnostic without the prefix is a normal file:line.
	vetPrefix = regexp.MustCompile(`^vet(?:\.exe)?:\s+`)
	// "FAIL\texample.com/mod/pkg\t0.284s" or
	// "FAIL\texample.com/mod/pkg [build failed]". The bracket is how a
	// package that never ran a test reports itself. A bare "FAIL" is the
	// run's summary and carries no package.
	goPkgStatus = regexp.MustCompile(`^FAIL\t(\S+)(?:\t(\S+))?(?: \[([^\]]+)\])?\s*$`)
	// Python traceback frame and error line (compileall, import errors).
	pyFrame = regexp.MustCompile(`^\s*File "([^"]+)", line (\d+)`)
	pyError = regexp.MustCompile(`^(\w+(?:Error|Exception)): (.+)$`)
	// rustc: "error[E0425]: cannot find value `x`" then "  --> src/main.rs:3:5".
	rustError = regexp.MustCompile(`^error(?:\[(E\d+)\])?: (.+)$`)
	rustArrow = regexp.MustCompile(`^\s*-->\s*(\S+?):\d+:\d+\s*$`)

	numbers   = regexp.MustCompile(`\d+`)
	hexAddr   = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	spaceRuns = regexp.MustCompile(`\s+`)
)

// Findings extracts what r reported wrong. A pass reports nothing. A failure
// the parsers cannot read still reports one finding, with the output's last
// line as its message: a gate that failed is never silently a finding-free
// run. With no node and no location that target is UnattributedTarget, not
// the module root. An unverified run reports nothing either -- it is not
// evidence of anything, and the caller records it as unverified.
func Findings(root string, r Result) []Finding {
	if r.Passed || r.Unverified() {
		return nil
	}
	var out []Finding
	seen := map[string]bool{}
	add := func(target, msg string, keyByMessage bool) {
		if len(out) >= MaxFindingsPerRun {
			return
		}
		target = relTarget(root, target)
		msg = strings.TrimSpace(msg)
		sig := signature(root, msg)
		key := r.Gate.ID + "\x00" + target
		if keyByMessage {
			key += "\x00" + sig
		}
		id := shortHash(key)
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, Finding{
			ID: id, Gate: r.Gate.ID, Kind: r.Gate.Kind, Node: r.Node,
			Target: target, Message: msg, Signature: sig,
		})
	}

	lines := strings.Split(strings.ReplaceAll(r.Output, "\r\n", "\n"), "\n")
	// Go names a package by import path ("FAIL\texample.com/mod/pkg") and,
	// since Go 1.26, emits build failures inside `go test -json` as
	// build-output / build-fail events. The file:line, when there is one,
	// is the finding; a package with no file is the package's directory.
	gs := newGoStream(root, modulePath(root), r.Node, add)
	var pyFile, rustMsg string
	for _, line := range lines {
		if gs.consume(line) {
			continue
		}
		switch {
		case cargoTestFailed.MatchString(line):
			m := cargoTestFailed.FindStringSubmatch(line)
			add(nodeOr(r.Node)+"::"+m[1], "test failed: "+m[1], false)
		case pytestFailed.MatchString(line):
			m := pytestFailed.FindStringSubmatch(line)
			msg := "test failed: " + m[1]
			if m[2] != "" {
				msg += " - " + m[2]
			}
			add(m[1], msg, false)
		case tscError.MatchString(line):
			m := tscError.FindStringSubmatch(line)
			add(m[1], m[4], true)
		case rustError.MatchString(line):
			m := rustError.FindStringSubmatch(line)
			rustMsg = strings.TrimSpace(m[1] + " " + m[2])
		case rustArrow.MatchString(line) && rustMsg != "":
			add(rustArrow.FindStringSubmatch(line)[1], rustMsg, true)
			rustMsg = ""
		case pyFrame.MatchString(line):
			pyFile = pyFrame.FindStringSubmatch(line)[1]
		case pyError.MatchString(line) && pyFile != "":
			m := pyError.FindStringSubmatch(line)
			add(pyFile, m[1]+": "+m[2], true)
			pyFile = ""
		}
	}
	gs.finish()
	if len(out) == 0 {
		add(nodeOr(r.Node), lastLine(lines, r.ExitCode), false)
	}
	return out
}

// isNoise drops file:line lines that are locations, not diagnostics: a
// frame's offset ("+0x1d"), a parenthesised location, a bare position.
func isNoise(msg string) bool {
	m := strings.TrimSpace(msg)
	return m == "" || strings.HasPrefix(m, "+0x") || strings.HasPrefix(m, "(") ||
		!strings.ContainsAny(m, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
}

// nodeOr is the target prefix for a tool that names a test but not a file.
// An empty node is UnattributedTarget, not ".": "." is the module root, and
// a cargo failure with no location is not a file there.
func nodeOr(node string) string {
	if node == "" {
		return UnattributedTarget
	}
	return node
}

// relTarget reports target relative to root with forward slashes, so the same
// file has one spelling across platforms and across absolute and relative
// tool output.
func relTarget(root, target string) string {
	t := strings.TrimSpace(target)
	// The Windows compiler prints "pkg\file.go". Node directories are
	// slash-separated, and a captured log has to name the same directory
	// wherever it is read.
	t = strings.ReplaceAll(t, `\`, `/`)
	if filepath.IsAbs(t) && root != "" {
		if rel, err := filepath.Rel(root, t); err == nil && !strings.HasPrefix(rel, "..") {
			t = rel
		}
	}
	t = filepath.ToSlash(t)
	return strings.TrimPrefix(t, "./")
}

func signature(root, msg string) string {
	s := msg
	if root != "" {
		s = strings.ReplaceAll(s, root, "<root>")
		s = strings.ReplaceAll(s, filepath.ToSlash(root), "<root>")
	}
	s = hexAddr.ReplaceAllString(s, "0xN")
	s = numbers.ReplaceAllString(s, "N")
	return strings.TrimSpace(spaceRuns.ReplaceAllString(s, " "))
}

func lastLine(lines []string, exitCode int) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return "failed with exit code " + strconv.Itoa(exitCode) + " and no output"
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// goStream reads one go test / go test -json / go build / go vet log.
//
// A file:line is the finding. "FAIL\tpkg [build failed]" is a package
// finding only when no diagnostic already named a file in that package:
// the captured vet-off run prints the compiler line and, later, the FAIL
// line, and they are one break. "--- FAIL:" lines, including indented
// subtests, wait for the next "FAIL\tpkg\t0.284s" that is not bracketed;
// that line names the package, so two packages can both fail TestSame.
// A column-0 "panic:" after "--- FAIL: TestPanics" belongs to that test.
// A column-0 "panic:" with no "--- FAIL" ("panic: initboom") belongs to
// the following "FAIL\tpkg" line. An import path the module line cannot
// place is kept as the tool printed it.
type goStream struct {
	root   string
	module string
	node   string
	add    func(target, msg string, keyByMessage bool)

	currentImport string
	pending       []pendingTest
	pkgPanic      string
	// diags is every file:line, uncapped. Findings' add callback still
	// stops at MaxFindingsPerRun; attribution must not.
	diags []GoDiagnostic

	covered       map[string]bool
	tested        map[string]bool
	jsonTested    map[string]bool
	jsonPkgDone   map[string]bool
	jsonPkgPanic  map[string]string
	jsonTestPanic map[string]string
}

type pendingTest struct {
	name  string
	panic string
}

// goEvent is one test2json line. Go 1.26 emits build failures as
// build-output / build-fail with ImportPath, then a fail event whose
// FailedBuild repeats the package (the test binary's path carries a
// " [pkg.test]" suffix).
type goEvent struct {
	Action      string `json:"Action"`
	Package     string `json:"Package"`
	Test        string `json:"Test"`
	ImportPath  string `json:"ImportPath"`
	Output      string `json:"Output"`
	FailedBuild string `json:"FailedBuild"`
}

func knownGoAction(action string) bool {
	switch action {
	case "start", "run", "pause", "cont", "pass", "fail", "skip", "output", "bench", "build-output", "build-fail":
		return true
	default:
		return false
	}
}

func newGoStream(root, module, node string, add func(target, msg string, keyByMessage bool)) *goStream {
	return &goStream{
		root: root, module: module, node: node, add: add,
		covered:       map[string]bool{},
		tested:        map[string]bool{},
		jsonTested:    map[string]bool{},
		jsonPkgDone:   map[string]bool{},
		jsonPkgPanic:  map[string]string{},
		jsonTestPanic: map[string]string{},
	}
}

// modulePath is the module line of root/go.mod. A missing or unreadable
// file yields "" so an import path stays as printed instead of being
// guessed onto a directory.
func modulePath(root string) string {
	if root == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], `"`)
		}
	}
	return ""
}

func (gs *goStream) consume(line string) bool {
	if line == "" {
		return false
	}
	if strings.HasPrefix(line, "{") {
		var ev goEvent
		if json.Unmarshal([]byte(line), &ev) == nil && knownGoAction(ev.Action) {
			gs.onJSON(ev)
			return true
		}
	}
	if strings.HasPrefix(line, "# ") {
		gs.noteHeader(line)
		return true
	}
	if m := goTestFail.FindStringSubmatch(line); m != nil {
		gs.pending = append(gs.pending, pendingTest{name: m[1]})
		return true
	}
	if strings.HasPrefix(line, "panic:") {
		gs.noteTextPanic(strings.TrimSpace(line))
		return true
	}
	// Indented file:line lines are test logs ("    ok_test.go:7: got 1 want 2")
	// and stack frames ("\tC:/Program Files/Go/src/testing/testing.go:1872 +0x239"),
	// not diagnostics.
	if !leadingWS(line) {
		diag := vetPrefix.ReplaceAllString(line, "")
		if m := fileLineMsg.FindStringSubmatch(diag); m != nil {
			if !isNoise(m[4]) {
				gs.addFile(m[1], atoiOrZero(m[2]), atoiOrZero(m[3]), m[4])
			}
			return true
		}
	}
	if m := goPkgStatus.FindStringSubmatch(line); m != nil {
		gs.onTextStatus(m[1], m[3])
		return true
	}
	if strings.TrimSpace(line) == "FAIL" {
		return true
	}
	return false
}

func (gs *goStream) noteHeader(line string) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	if rest == "" {
		return
	}
	gs.currentImport = cleanImport(rest)
}

func (gs *goStream) noteTextPanic(msg string) {
	if n := len(gs.pending); n > 0 {
		if gs.pending[n-1].panic == "" {
			gs.pending[n-1].panic = msg
		}
		return
	}
	if gs.pkgPanic == "" {
		gs.pkgPanic = msg
	}
}

func (gs *goStream) onTextStatus(importPath, bracket string) {
	key := gs.coverKey(importPath)
	if bracket != "" {
		// A bracketed line ("FAIL\tpkg [build failed]") is the package
		// that did not run. It is not where buffered --- FAIL lines belong,
		// and a compiler line already in that directory is the same break.
		if !gs.covered[key] && !gs.tested[key] {
			gs.add(gs.pkgTarget(importPath), bracket+": "+cleanImport(importPath), true)
			gs.covered[key] = true
		}
		return
	}
	if len(gs.pending) > 0 {
		for _, pt := range gs.pending {
			msg := "test failed: " + pt.name
			if pt.panic != "" {
				msg = "test panicked: " + pt.name + ": " + pt.panic
			}
			gs.add(gs.testTarget(importPath, pt.name), msg, false)
		}
		gs.pending = nil
		gs.pkgPanic = ""
		gs.tested[key] = true
		return
	}
	if gs.covered[key] || gs.tested[key] {
		gs.pkgPanic = ""
		return
	}
	if gs.pkgPanic != "" {
		gs.add(gs.pkgTarget(importPath), gs.pkgPanic, true)
		gs.pkgPanic = ""
		gs.tested[key] = true
		return
	}
	gs.add(gs.pkgTarget(importPath), "package failed: "+cleanImport(importPath), true)
	gs.tested[key] = true
}

func (gs *goStream) onJSON(ev goEvent) {
	switch ev.Action {
	case "build-output":
		gs.onBuildOutput(ev)
	case "build-fail":
		gs.onBuildFail(ev)
	case "output":
		gs.onOutput(ev)
	case "fail":
		gs.onFail(ev)
	}
}

func (gs *goStream) onBuildOutput(ev goEvent) {
	imp := ev.ImportPath
	if imp == "" {
		imp = ev.Package
	}
	if c := cleanImport(imp); c != "" {
		gs.currentImport = c
	}
	for _, line := range outputLines(ev.Output) {
		if strings.HasPrefix(line, "# ") {
			gs.noteHeader(line)
			continue
		}
		if leadingWS(line) {
			continue
		}
		diag := vetPrefix.ReplaceAllString(line, "")
		if m := fileLineMsg.FindStringSubmatch(diag); m != nil && !isNoise(m[4]) {
			gs.addFile(m[1], atoiOrZero(m[2]), atoiOrZero(m[3]), m[4])
		}
	}
}

func (gs *goStream) onBuildFail(ev goEvent) {
	imp := ev.ImportPath
	if imp == "" {
		imp = ev.Package
	}
	if cleanImport(imp) == "" {
		return
	}
	gs.notePackageFail(imp, "build failed: "+cleanImport(imp))
}

func (gs *goStream) onOutput(ev goEvent) {
	pkg := cleanImport(ev.Package)
	for _, line := range outputLines(ev.Output) {
		// "panic: boom [recovered, repanicked]", not the stack's "panic({0x...})".
		if !strings.HasPrefix(line, "panic:") {
			continue
		}
		msg := strings.TrimSpace(line)
		if ev.Test != "" {
			k := pkg + "\x00" + ev.Test
			if gs.jsonTestPanic[k] == "" {
				gs.jsonTestPanic[k] = msg
			}
			continue
		}
		if pkg != "" && gs.jsonPkgPanic[pkg] == "" {
			gs.jsonPkgPanic[pkg] = msg
		}
	}
}

func (gs *goStream) onFail(ev goEvent) {
	pkg := cleanImport(ev.Package)
	if ev.Test != "" {
		msg := "test failed: " + ev.Test
		if p := gs.jsonTestPanic[pkg+"\x00"+ev.Test]; p != "" {
			msg = "test panicked: " + ev.Test + ": " + p
		}
		gs.add(gs.testTarget(ev.Package, ev.Test), msg, false)
		gs.jsonTested[pkg] = true
		gs.tested[gs.coverKey(ev.Package)] = true
		return
	}
	if pkg == "" && ev.FailedBuild == "" {
		return
	}
	if gs.jsonTested[pkg] || gs.jsonPkgDone[pkg] {
		gs.jsonPkgDone[pkg] = true
		return
	}
	imp := ev.Package
	msg := "package failed: " + pkg
	if p := gs.jsonPkgPanic[pkg]; p != "" {
		msg = p
	} else if ev.FailedBuild != "" {
		imp = ev.FailedBuild
		msg = "build failed: " + cleanImport(ev.FailedBuild)
	}
	gs.notePackageFail(imp, msg)
}

func (gs *goStream) notePackageFail(importPath, msg string) {
	key := gs.coverKey(importPath)
	pkg := cleanImport(importPath)
	if gs.covered[key] || gs.tested[key] || gs.jsonTested[pkg] || gs.jsonPkgDone[pkg] {
		gs.jsonPkgDone[pkg] = true
		return
	}
	gs.add(gs.pkgTarget(importPath), msg, true)
	gs.covered[key] = true
	gs.jsonPkgDone[pkg] = true
}

func (gs *goStream) finish() {
	if len(gs.pending) > 0 {
		for _, pt := range gs.pending {
			msg := "test failed: " + pt.name
			if pt.panic != "" {
				msg = "test panicked: " + pt.name + ": " + pt.panic
			}
			gs.add(gs.testTarget("", pt.name), msg, false)
		}
		gs.pending = nil
		gs.pkgPanic = ""
	}
	if gs.pkgPanic != "" {
		if gs.currentImport != "" {
			gs.notePackageFail(gs.currentImport, gs.pkgPanic)
		} else {
			target := UnattributedTarget
			if gs.node != "" {
				target = gs.node
			}
			gs.add(target, gs.pkgPanic, true)
		}
		gs.pkgPanic = ""
	}
	pkgs := make([]string, 0, len(gs.jsonPkgPanic))
	for pkg := range gs.jsonPkgPanic {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		if gs.jsonPkgDone[pkg] || gs.jsonTested[pkg] {
			continue
		}
		gs.notePackageFail(pkg, gs.jsonPkgPanic[pkg])
	}
}

func (gs *goStream) addFile(file string, line, col int, msg string) {
	norm := relTarget(gs.root, file)
	gs.diags = append(gs.diags, GoDiagnostic{
		File: norm, Line: line, Col: col,
		Message: strings.TrimSpace(msg), Package: gs.currentImport,
	})
	gs.add(file, msg, true)
	dir := path.Dir(norm)
	if dir == "" || dir == "/" {
		dir = "."
	}
	gs.covered[dir] = true
	if gs.currentImport != "" {
		if d, ok := gs.locate(gs.currentImport); ok {
			gs.covered[d] = true
		}
	}
}

// locate maps an import path to a workspace-relative directory using the
// module path exactly. "example.com/r7b-extra" does not belong to module
// "example.com/r7b", and a path that does not match is not guessed from
// its suffix.
func (gs *goStream) locate(importPath string) (string, bool) {
	imp := cleanImport(importPath)
	if imp == "" {
		return "", false
	}
	if imp == "." || strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, `.\`) {
		return relTarget(gs.root, imp), true
	}
	if gs.module == "" {
		return "", false
	}
	if imp == gs.module {
		return ".", true
	}
	rest, ok := strings.CutPrefix(imp, gs.module+"/")
	if !ok || rest == "" || strings.ContainsAny(rest, " \t") {
		return "", false
	}
	return rest, true
}

func (gs *goStream) coverKey(importPath string) string {
	if dir, ok := gs.locate(importPath); ok {
		return dir
	}
	if imp := cleanImport(importPath); imp != "" {
		return imp
	}
	return "."
}

// pkgTarget is the directory of a package failure. An import the module
// line cannot place stays the import path the tool printed; it is not
// replaced with the node the gate ran for.
func (gs *goStream) pkgTarget(importPath string) string {
	if dir, ok := gs.locate(importPath); ok {
		return dir
	}
	if imp := cleanImport(importPath); imp != "" {
		return imp
	}
	if gs.node != "" {
		return gs.node
	}
	return UnattributedTarget
}

// testTarget names a failed test. The directory wins when the module line
// places the import. Otherwise a node-scoped run (the store findings
// fixture has no go.mod under its root) uses the node the gate ran for,
// and a workspace run keeps the import path so two packages that both
// fail TestSame stay distinct.
func (gs *goStream) testTarget(importPath, test string) string {
	if dir, ok := gs.locate(importPath); ok {
		return dir + "::" + test
	}
	if gs.node != "" {
		return gs.node + "::" + test
	}
	if imp := cleanImport(importPath); imp != "" {
		return imp + "::" + test
	}
	return UnattributedTarget + "::" + test
}

// cleanImport strips the " [pkg.test]" suffix go prints on a test-binary
// build and the brackets of a "# [pkg]" vet header.
func cleanImport(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " ["); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(strings.Trim(s, "[]"))
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ParseGoDiagnostics reads file:line diagnostics out of go build / go test /
// go vet output. It walks the same stream Findings uses (the "# pkg" header,
// the vet.exe prefix, a Windows path, a test2json build-output line) and
// keeps every file:line. Findings stops at MaxFindingsPerRun; a turn is
// charged from the whole log, so this does not. A package-only failure
// ("FAIL\tpkg [build failed]") yields nothing: no file was named.
func ParseGoDiagnostics(root, output string) []GoDiagnostic {
	gs := newGoStream(root, modulePath(root), "", func(string, string, bool) {})
	for _, line := range outputLines(output) {
		gs.consume(line)
	}
	gs.finish()
	return gs.diags
}

func outputLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func leadingWS(s string) bool {
	return strings.HasPrefix(s, " ") || strings.HasPrefix(s, "\t")
}

// SortFindings orders findings by ID, the order every consumer sees them in.
func SortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
}
