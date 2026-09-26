package gates

import (
	"crypto/sha256"
	"encoding/hex"
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
	// Target is what failed: a file, a file::test, a test name, or the node
	// (or "." for the workspace) when the output named nothing narrower.
	Target string
	// Message is the failure as reported, first line.
	Message string
	// Signature is Message normalised: numbers, addresses and the workspace
	// path collapsed. Two attempts that end on the same Signature failed the
	// same way.
	Signature string
}

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
// the parsers cannot read still reports one finding against the node, with
// the output's last line as its message: a gate that failed is never
// silently a finding-free run. An unverified run reports nothing either --
// it is not evidence of anything, and the caller records it as unverified.
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
	var pyFile, rustMsg string
	for _, line := range lines {
		switch {
		case goTestFail.MatchString(line):
			m := goTestFail.FindStringSubmatch(line)
			// A test is its own identity: a partial fix that changes the
			// message has not turned the failure into a different one.
			add(nodeOr(r.Node)+"::"+m[1], "test failed: "+m[1], false)
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
		case fileLineMsg.MatchString(line):
			if strings.TrimLeft(line, " \t") != line {
				// Indented file:line lines are a failing Go test's log
				// output (or a stack frame); the --- FAIL line above them
				// is the finding. Diagnostics start at column zero.
				continue
			}
			m := fileLineMsg.FindStringSubmatch(line)
			msg := m[4]
			if isNoise(msg) {
				continue
			}
			add(m[1], msg, true)
		}
	}
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

func nodeOr(node string) string {
	if node == "" {
		return "."
	}
	return node
}

// relTarget reports target relative to root with forward slashes, so the same
// file has one spelling across platforms and across absolute and relative
// tool output.
func relTarget(root, target string) string {
	t := strings.TrimSpace(target)
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

// SortFindings orders findings by ID, the order every consumer sees them in.
func SortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
}
